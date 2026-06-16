package middleware

// Package middleware provides HTTP middleware that wraps internal/auth verification
// and maps auth errors to 401/403 responses.
import (
	"errors"
	"io"
	"net/http"

	"ai-orchestration/internal/auth"
)

func WriteAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrNoCredentials):
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	case errors.Is(err, auth.ErrVerificationFailed):
		http.Error(w, "Forbidden", http.StatusForbidden)
	default:
		http.Error(w, "Forbidden", http.StatusForbidden)
	}
}

type FirebaseVerifier interface {
	VerifyToken(r *http.Request, token string) (auth.Principal, error)
}

type firebaseVerifierAdapter struct {
	verifier *auth.FirebaseVerifier
}

func NewFirebaseVerifierAdapter(verifier *auth.FirebaseVerifier) FirebaseVerifier {
	return &firebaseVerifierAdapter{verifier: verifier}
}

func (a *firebaseVerifierAdapter) VerifyToken(r *http.Request, token string) (auth.Principal, error) {
	return a.verifier.VerifyToken(r.Context(), token)
}

func FirebaseAuth(verifier FirebaseVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := auth.ExtractBearerToken(r.Header.Get("Authorization"))
			if err != nil {
				WriteAuthError(w, err)
				return
			}

			principal, err := verifier.VerifyToken(r, token)
			if err != nil {
				WriteAuthError(w, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
		})
	}
}

func SlackAuth(signingSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				WriteAuthError(w, auth.ErrVerificationFailed)
				return
			}
			_ = r.Body.Close()

			err = auth.VerifySlackRequest(
				r.Header.Get("X-Slack-Request-Timestamp"),
				r.Header.Get("X-Slack-Signature"),
				body,
				signingSecret,
			)
			if err != nil {
				WriteAuthError(w, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithSlackBody(r.Context(), body)))
		})
	}
}
