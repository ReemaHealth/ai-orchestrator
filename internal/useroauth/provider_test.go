package useroauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/useroauth"
	"ai-orchestration/internal/useroauth/store"
)

func TestProviderUsesClientOverride(t *testing.T) {
	cfg := oauthTestConfig()
	tokenStore := store.NewMemoryStore()
	provider, err := useroauth.NewProvider(cfg, tokenStore)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	token, err := provider.AccessToken(context.Background(), testPrincipal(), "override-token")
	if err != nil {
		t.Fatalf("access token: %v", err)
	}
	if token != "override-token" {
		t.Fatalf("token = %q, want override-token", token)
	}
}

func TestProviderReturnsConsentRequiredWithoutGrant(t *testing.T) {
	cfg := oauthTestConfig()
	tokenStore := store.NewMemoryStore()
	provider, err := useroauth.NewProvider(cfg, tokenStore)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = provider.AccessToken(context.Background(), testPrincipal(), "")
	if err == nil {
		t.Fatal("expected ErrConsentRequired")
	}
	if err != useroauth.ErrConsentRequired {
		t.Fatalf("err = %v, want ErrConsentRequired", err)
	}
}

func TestProviderUsesStoredRefreshToken(t *testing.T) {
	t.Skip("requires live Google token exchange; covered by handler integration tests")

	cfg := oauthTestConfig()
	tokenStore := store.NewMemoryStore()
	principal := testPrincipal()

	_ = tokenStore.Save(context.Background(), principal.ReemaUserID, principal.Email, &oauth2.Token{
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(time.Hour),
	})

	provider, err := useroauth.NewProvider(cfg, tokenStore)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = provider.AccessToken(context.Background(), principal, "")
	if err == nil {
		t.Fatal("expected refresh error without valid refresh token")
	}
}

func TestStateCodecRoundTrip(t *testing.T) {
	codec, err := useroauth.NewStateCodec("test-secret")
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	state, err := codec.Encode(userID, "user@example.com")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	gotID, email, err := codec.Decode(state)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if gotID != userID {
		t.Fatalf("user id = %s, want %s", gotID, userID)
	}
	if email != "user@example.com" {
		t.Fatalf("email = %q", email)
	}
}

func TestMemoryStoreSaveLoadDelete(t *testing.T) {
	mem := store.NewMemoryStore()
	userID := uuid.New()
	token := &oauth2.Token{RefreshToken: "refresh", AccessToken: "access"}

	if err := mem.Save(context.Background(), userID, "user@example.com", token); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := mem.Load(context.Background(), userID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.RefreshToken != "refresh" {
		t.Fatalf("refresh = %q", loaded.RefreshToken)
	}

	if err := mem.Delete(context.Background(), userID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := mem.Load(context.Background(), userID); err == nil {
		t.Fatal("expected load error after delete")
	}
}

func oauthTestConfig() config.Config {
	return config.Config{
		AgentEnabled:               true,
		UserOAuthMode:              "auto",
		GoogleOAuthClientID:        "client-id",
		GoogleOAuthClientSecret:    "client-secret",
		GoogleOAuthRedirectURI:     "http://localhost:8080/api/v1/oauth/google/callback",
		GoogleOAuthStateSecret:     "state-secret",
		GoogleOAuthScopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		GoogleOAuthAllowedDomains:  []string{"example.com"},
	}
}

func testPrincipal() auth.Principal {
	return auth.Principal{
		ReemaUserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Email:       "user@example.com",
	}
}
