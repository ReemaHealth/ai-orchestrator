package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/useroauth"
	"ai-orchestration/internal/useroauth/store"
)

// GoogleOAuthHandler serves Google OAuth consent endpoints for workspace datastore access.
type GoogleOAuthHandler struct {
	cfg        config.Config
	oauth      *oauth2.Config
	store      store.RefreshTokenStore
	tokens     useroauth.TokenProvider
	stateCodec *useroauth.StateCodec
}

// NewGoogleOAuthHandler returns handlers for Google OAuth start, callback, and revoke.
// Returns nil when Google OAuth client settings are not configured.
func NewGoogleOAuthHandler(
	cfg config.Config,
	tokenStore store.RefreshTokenStore,
	tokens useroauth.TokenProvider,
) (*GoogleOAuthHandler, error) {
	if !cfg.GoogleOAuthConfigured() {
		return nil, nil
	}

	stateCodec, err := useroauth.NewStateCodec(cfg.GoogleOAuthStateSecret)
	if err != nil {
		return nil, fmt.Errorf("oauth state codec: %w", err)
	}

	return &GoogleOAuthHandler{
		cfg:        cfg,
		oauth:      useroauth.OAuthConfig(cfg),
		store:      tokenStore,
		tokens:     tokens,
		stateCodec: stateCodec,
	}, nil
}

// Start handles GET /api/v1/oauth/google/start.
// Auth: Firebase JWT required. Redirects the browser to Google OAuth consent.
func (h *GoogleOAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	state, err := h.stateCodec.Encode(principal.ReemaUserID, principal.Email)
	if err != nil {
		log.Printf("oauth start: encode state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authURL := h.oauth.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles GET /api/v1/oauth/google/callback.
// Auth: none (state is HMAC-signed). Exchanges code, stores refresh token, redirects to success URL.
func (h *GoogleOAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if errMsg := strings.TrimSpace(r.URL.Query().Get("error")); errMsg != "" {
		http.Error(w, "oauth denied: "+errMsg, http.StatusBadRequest)
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	userID, email, err := h.stateCodec.Decode(state)
	if err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("oauth callback: exchange code: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}

	if token.RefreshToken == "" {
		http.Error(w, "no refresh token returned; revoke prior grant in Google account settings and retry", http.StatusBadRequest)
		return
	}

	if err := h.store.Save(r.Context(), userID, email, token); err != nil {
		log.Printf("oauth callback: save token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	redirectURL := strings.TrimSpace(h.cfg.GoogleOAuthSuccessRedirect)
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Revoke handles DELETE /api/v1/oauth/google/revoke.
// Auth: Firebase JWT required. Deletes the stored refresh token for the caller.
func (h *GoogleOAuthHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.tokens.RevokeGrant(r.Context(), principal); err != nil {
		log.Printf("oauth revoke: %v", err)
		http.Error(w, "oauth revoke failed", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
