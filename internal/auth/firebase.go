package auth

// Package auth provides cryptographic identity verification for two caller types:
// Firebase JWT (web app) and Slack request signing (Events API). Verified identity
// is attached to context.Context for handlers. See docs/architecture.md.
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"ai-orchestration/internal/config"
)

const firebaseJWKSURL = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

type FirebaseVerifier struct {
	jwks           keyfunc.Keyfunc
	projectID      string
	signInProvider string
}

func NewFirebaseVerifier(ctx context.Context, cfg config.Config) (*FirebaseVerifier, error) {
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{firebaseJWKSURL})
	if err != nil {
		return nil, fmt.Errorf("init firebase jwks: %w", err)
	}

	return &FirebaseVerifier{
		jwks:           jwks,
		projectID:      cfg.FirebaseProjectID,
		signInProvider: cfg.FirebaseSignInProvider,
	}, nil
}

func NewFirebaseVerifierWithJWKS(jwks keyfunc.Keyfunc, cfg config.Config) *FirebaseVerifier {
	return &FirebaseVerifier{
		jwks:           jwks,
		projectID:      cfg.FirebaseProjectID,
		signInProvider: cfg.FirebaseSignInProvider,
	}
}

func ExtractBearerToken(authHeader string) (string, error) {
	if strings.TrimSpace(authHeader) == "" {
		return "", ErrNoCredentials
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", ErrNoCredentials
	}

	return strings.TrimSpace(parts[1]), nil
}

func (v *FirebaseVerifier) VerifyToken(_ context.Context, tokenString string) (Principal, error) {
	token, err := jwt.Parse(tokenString, v.jwks.Keyfunc)
	if err != nil || !token.Valid {
		return Principal{}, ErrVerificationFailed
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, ErrVerificationFailed
	}

	expectedIss := "https://securetoken.google.com/" + v.projectID
	iss, _ := claims.GetIssuer()
	if iss != expectedIss {
		return Principal{}, ErrVerificationFailed
	}

	audiences, _ := claims.GetAudience()
	if !audienceContains(audiences, v.projectID) {
		return Principal{}, ErrVerificationFailed
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil || exp.Before(time.Now()) {
		return Principal{}, ErrVerificationFailed
	}

	if !signInProviderMatches(claims, v.signInProvider) {
		return Principal{}, ErrVerificationFailed
	}

	reemaUserID, err := reemaUserIDFromClaims(claims)
	if err != nil {
		return Principal{}, ErrVerificationFailed
	}

	email, err := emailFromClaims(claims, v.signInProvider)
	if err != nil {
		return Principal{}, ErrVerificationFailed
	}

	return Principal{ReemaUserID: reemaUserID, Email: email}, nil
}

func audienceContains(audiences jwt.ClaimStrings, expected string) bool {
	for _, aud := range audiences {
		if aud == expected {
			return true
		}
	}
	return false
}

func signInProviderMatches(claims jwt.MapClaims, expected string) bool {
	firebaseClaim, ok := claims["firebase"].(map[string]any)
	if !ok {
		return false
	}
	provider, ok := firebaseClaim["sign_in_provider"].(string)
	return ok && provider == expected
}

func emailFromClaims(claims jwt.MapClaims, signInProvider string) (string, error) {
	if signInProvider == "google.com" {
		raw, ok := claims["email"]
		if !ok {
			return "", ErrVerificationFailed
		}
		email, ok := raw.(string)
		if !ok || strings.TrimSpace(email) == "" {
			return "", ErrVerificationFailed
		}
		return strings.TrimSpace(email), nil
	}

	return "", nil
}

func reemaUserIDFromClaims(claims jwt.MapClaims) (uuid.UUID, error) {
	raw, ok := claims["reemaUserId"]
	if !ok {
		return uuid.UUID{}, ErrVerificationFailed
	}

	switch v := raw.(type) {
	case string:
		return uuid.Parse(v)
	default:
		return uuid.UUID{}, ErrVerificationFailed
	}
}
