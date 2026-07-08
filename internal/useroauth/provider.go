package useroauth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/useroauth/store"
)

// Mode controls how user OAuth access tokens are resolved for agent calls.
type Mode string

const (
	// ModeAuto uses stored refresh tokens when OAuth is configured; client override still wins.
	ModeAuto Mode = "auto"
	// ModeRequired requires a token from client override or stored refresh grant.
	ModeRequired Mode = "required"
	// ModeClientOnly requires X-GCP-Access-Token or body gcpAccessToken (legacy).
	ModeClientOnly Mode = "client_only"
	// ModeDisabled skips user OAuth resolution (skeleton / no datastore ACL).
	ModeDisabled Mode = "disabled"
)

// TokenProvider resolves a Google access token for an authenticated Reema user.
type TokenProvider interface {
	// AccessToken returns a user-scoped Google access token. clientOverride is a
	// caller-supplied token (header/body); when non-empty it is returned as-is.
	AccessToken(ctx context.Context, principal auth.Principal, clientOverride string) (string, error)
	// ConsentRequired reports whether missing tokens should trigger consent flow.
	ConsentRequired() bool
	// AuthorizePath returns the relative path to start Google OAuth consent.
	AuthorizePath() string
	// RevokeGrant removes stored grants and invalidates cached access tokens.
	RevokeGrant(ctx context.Context, principal auth.Principal) error
}

// Provider resolves tokens via stored refresh grants with optional client override.
type Provider struct {
	mode       Mode
	oauth      *oauth2.Config
	store      store.RefreshTokenStore
	allowedDom []string
	cache      tokenCache
}

type tokenCache struct {
	mu     sync.Mutex
	byUser map[string]cachedAccessToken
}

type cachedAccessToken struct {
	token     string
	expiresAt time.Time
}

// NewProvider builds a TokenProvider from configuration and a refresh token store.
// When OAuth client settings are incomplete, returns a client-only or disabled provider.
func NewProvider(cfg config.Config, tokenStore store.RefreshTokenStore) (TokenProvider, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(cfg.UserOAuthMode)))
	if mode == "" {
		mode = ModeAuto
	}

	if !cfg.AgentEnabled || mode == ModeDisabled {
		return &noopProvider{}, nil
	}

	if mode == ModeClientOnly || !cfg.GoogleOAuthConfigured() {
		return &clientOnlyProvider{mode: mode}, nil
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		RedirectURL:  cfg.GoogleOAuthRedirectURI,
		Scopes:       cfg.GoogleOAuthScopes,
		Endpoint:     google.Endpoint,
	}

	return &Provider{
		mode:       mode,
		oauth:      oauthCfg,
		store:      tokenStore,
		allowedDom: cfg.GoogleOAuthAllowedDomains,
		cache: tokenCache{
			byUser: make(map[string]cachedAccessToken),
		},
	}, nil
}

// OAuthConfig returns the oauth2.Config for consent and callback handlers.
func OAuthConfig(cfg config.Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		RedirectURL:  cfg.GoogleOAuthRedirectURI,
		Scopes:       cfg.GoogleOAuthScopes,
		Endpoint:     google.Endpoint,
	}
}

// AccessToken implements TokenProvider.
func (p *Provider) AccessToken(ctx context.Context, principal auth.Principal, clientOverride string) (string, error) {
	if token := strings.TrimSpace(clientOverride); token != "" {
		return token, nil
	}

	if err := p.validateEmail(principal.Email); err != nil {
		return "", err
	}

	if cached, ok := p.cache.get(principal.ReemaUserID); ok {
		return cached, nil
	}

	stored, err := p.store.Load(ctx, principal.ReemaUserID)
	if err != nil {
		return "", ErrConsentRequired
	}

	src := p.oauth.TokenSource(ctx, stored)
	tok, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("refresh access token: %w", err)
	}

	if tok.RefreshToken != "" && tok.RefreshToken != stored.RefreshToken {
		_ = p.store.Save(ctx, principal.ReemaUserID, principal.Email, tok)
	}

	if strings.TrimSpace(tok.AccessToken) == "" {
		return "", ErrConsentRequired
	}

	p.cache.set(principal.ReemaUserID, tok.AccessToken, tok.Expiry)
	return tok.AccessToken, nil
}

// RevokeGrant implements TokenProvider.
func (p *Provider) RevokeGrant(ctx context.Context, principal auth.Principal) error {
	p.cache.delete(principal.ReemaUserID)

	stored, err := p.store.Load(ctx, principal.ReemaUserID)
	if err != nil {
		return nil
	}

	if token := strings.TrimSpace(stored.RefreshToken); token != "" {
		if err := revokeGoogleOAuthToken(ctx, token); err != nil {
			return fmt.Errorf("revoke google oauth token: %w", err)
		}
	} else if token := strings.TrimSpace(stored.AccessToken); token != "" {
		_ = revokeGoogleOAuthToken(ctx, token)
	}

	return p.store.Delete(ctx, principal.ReemaUserID)
}

// ConsentRequired implements TokenProvider.
func (p *Provider) ConsentRequired() bool {
	return p.mode == ModeAuto || p.mode == ModeRequired
}

// AuthorizePath implements TokenProvider.
func (p *Provider) AuthorizePath() string {
	return "/api/v1/oauth/google/start"
}

func (p *Provider) validateEmail(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return fmt.Errorf("verified email is required for user oauth")
	}
	if len(p.allowedDom) == 0 {
		return nil
	}
	for _, domain := range p.allowedDom {
		domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "@")
		if domain != "" && strings.HasSuffix(email, "@"+domain) {
			return nil
		}
	}
	return fmt.Errorf("email domain not allowed for user oauth")
}

func (c *tokenCache) get(userID interface{ String() string }) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.byUser[userID.String()]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt.Add(-2 * time.Minute)) {
		delete(c.byUser, userID.String())
		return "", false
	}
	return entry.token, true
}

func (c *tokenCache) set(userID interface{ String() string }, token string, expiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if expiry.IsZero() {
		expiry = time.Now().Add(50 * time.Minute)
	}
	c.byUser[userID.String()] = cachedAccessToken{
		token:     token,
		expiresAt: expiry,
	}
}

func (c *tokenCache) delete(userID interface{ String() string }) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.byUser, userID.String())
}

type noopProvider struct{}

func (p *noopProvider) AccessToken(_ context.Context, _ auth.Principal, clientOverride string) (string, error) {
	return strings.TrimSpace(clientOverride), nil
}

func (p *noopProvider) ConsentRequired() bool { return false }

func (p *noopProvider) AuthorizePath() string { return "" }

func (p *noopProvider) RevokeGrant(_ context.Context, _ auth.Principal) error {
	return nil
}

type clientOnlyProvider struct {
	mode Mode
}

func (p *clientOnlyProvider) AccessToken(_ context.Context, _ auth.Principal, clientOverride string) (string, error) {
	token := strings.TrimSpace(clientOverride)
	if token == "" && (p.mode == ModeRequired || p.mode == ModeClientOnly || p.mode == ModeAuto) {
		return "", ErrConsentRequired
	}
	return token, nil
}

func (p *clientOnlyProvider) ConsentRequired() bool { return true }

func (p *clientOnlyProvider) AuthorizePath() string { return "/api/v1/oauth/google/start" }

func (p *clientOnlyProvider) RevokeGrant(_ context.Context, _ auth.Principal) error {
	return nil
}
