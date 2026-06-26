package middleware_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/middleware"
)

type stubFirebaseVerifier struct {
	err       error
	principal auth.Principal
}

func (s stubFirebaseVerifier) VerifyToken(_ *http.Request, _ string) (auth.Principal, error) {
	if s.err != nil {
		return auth.Principal{}, s.err
	}
	return s.principal, nil
}

func TestFirebaseAuthStatuses(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		header     string
		verifier   stubFirebaseVerifier
		wantStatus int
	}{
		{
			name:       "missing authorization",
			header:     "",
			verifier:   stubFirebaseVerifier{err: auth.ErrNoCredentials},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			header:     "Bearer bad.token",
			verifier:   stubFirebaseVerifier{err: auth.ErrVerificationFailed},
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "valid token",
			header: "Bearer good.token",
			verifier: stubFirebaseVerifier{
				principal: auth.Principal{
					ReemaUserID: uuid.New(),
					Email:       "user@example.com",
				},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware.FirebaseAuth(tt.verifier)(okHandler)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestSlackAuthStatuses(t *testing.T) {
	secret := "test-signing-secret"
	body := []byte(`{"type":"event_callback"}`)
	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	signature := auth.SignSlackRequest(timestamp, body, secret)

	handler := middleware.SlackAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("missing headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/slack/events", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("bad signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/slack/events", bytes.NewReader(body))
		req.Header.Set("X-Slack-Request-Timestamp", timestamp)
		req.Header.Set("X-Slack-Signature", "v0=deadbeef")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("valid signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/slack/events", bytes.NewReader(body))
		req.Header.Set("X-Slack-Request-Timestamp", timestamp)
		req.Header.Set("X-Slack-Signature", signature)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestWriteAuthError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"no credentials", auth.ErrNoCredentials, http.StatusUnauthorized},
		{"verification failed", auth.ErrVerificationFailed, http.StatusForbidden},
		{"unknown", errors.New("other"), http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			middleware.WriteAuthError(rec, tt.err)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
