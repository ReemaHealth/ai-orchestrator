package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
)

func TestFirebaseVerifierVerifyToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		modulus := make([]byte, privateKey.Size())
		privateKey.N.FillBytes(modulus)
		n := base64.RawURLEncoding.EncodeToString(modulus)
		e := base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"kid": "test-kid",
					"n":   n,
					"e":   e,
				},
			},
		})
	}))
	t.Cleanup(jwksServer.Close)

	ctx := t.Context()
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{jwksServer.URL})
	if err != nil {
		t.Fatalf("jwks: %v", err)
	}

	projectID := "reema-application-test"
	reemaUserID := uuid.New()
	cfg := config.Config{
		FirebaseProjectID:      projectID,
		FirebaseSignInProvider: "google.com",
	}

	verifier := auth.NewFirebaseVerifierWithJWKS(jwks, cfg)
	token := signTestFirebaseToken(t, privateKey, projectID, reemaUserID)

	principal, err := verifier.VerifyToken(ctx, token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if principal.ReemaUserID != reemaUserID {
		t.Fatalf("expected reema user id %s, got %s", reemaUserID, principal.ReemaUserID)
	}
	if principal.Email != "user@example.com" {
		t.Fatalf("expected email user@example.com, got %s", principal.Email)
	}
}

func TestExtractBearerToken(t *testing.T) {
	token, err := auth.ExtractBearerToken("Bearer abc.def.ghi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc.def.ghi" {
		t.Fatalf("unexpected token: %q", token)
	}

	if _, err := auth.ExtractBearerToken(""); err != auth.ErrNoCredentials {
		t.Fatalf("expected ErrNoCredentials for empty header, got %v", err)
	}

	if _, err := auth.ExtractBearerToken("Token abc"); err != auth.ErrNoCredentials {
		t.Fatalf("expected ErrNoCredentials for non-bearer header, got %v", err)
	}
}

func signTestFirebaseToken(t *testing.T, privateKey *rsa.PrivateKey, projectID string, reemaUserID uuid.UUID) string {
	t.Helper()

	claims := jwt.MapClaims{
		"iss": "https://securetoken.google.com/" + projectID,
		"aud": projectID,
		"exp": time.Now().Add(time.Hour).Unix(),
		"email": "user@example.com",
		"firebase": map[string]any{
			"sign_in_provider": "google.com",
		},
		"reemaUserId": reemaUserID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid"

	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
