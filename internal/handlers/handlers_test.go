package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"ai-orchestration/internal/agent"
	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/handlers"
	"ai-orchestration/internal/useroauth"
)

type stubAgent struct {
	chunks []string
	lastIn agent.StreamQueryInput
}

func (s *stubAgent) StreamQuery(_ context.Context, in agent.StreamQueryInput, emit func(chunk string) error) error {
	s.lastIn = in
	for _, chunk := range s.chunks {
		if err := emit(chunk); err != nil {
			return err
		}
	}
	return nil
}

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) AccessToken(_ context.Context, _ auth.Principal, clientOverride string) (string, error) {
	if strings.TrimSpace(clientOverride) != "" {
		return clientOverride, nil
	}
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func (stubTokenProvider) ConsentRequired() bool { return true }

func (stubTokenProvider) AuthorizePath() string { return "/api/v1/oauth/google/start" }

func TestPromptStreamsSSE(t *testing.T) {
	reemaUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	principal := auth.Principal{
		ReemaUserID: reemaUserID,
		Email:       "user@example.com",
	}
	stub := &stubAgent{chunks: []string{"chunk one", "chunk two"}}
	err := handlers.StreamPromptForTest(rec, principal, "hello", "oauth-token", stub)
	if err != nil {
		t.Fatalf("stream prompt: %v", err)
	}

	if stub.lastIn.UserOAuthToken != "oauth-token" {
		t.Fatalf("expected oauth token on agent input, got %q", stub.lastIn.UserOAuthToken)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `event: meta`) {
		t.Fatalf("expected meta event, got: %s", body)
	}
	if !strings.Contains(body, reemaUserID.String()) {
		t.Fatalf("expected reemaUserId in stream, got: %s", body)
	}
	if !strings.Contains(body, "user@example.com") {
		t.Fatalf("expected email in meta event, got: %s", body)
	}
	if !strings.Contains(body, "data: chunk two") {
		t.Fatalf("expected agent chunk, got: %s", body)
	}
}

func TestPromptRequiresPrincipal(t *testing.T) {
	handler := handlers.NewPromptHandler(agent.NewSkeletonClient(), stubTokenProvider{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader([]byte(`{"prompt":"hello"}`)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestPromptRequiresPromptBody(t *testing.T) {
	handler := handlers.NewPromptHandler(agent.NewSkeletonClient(), stubTokenProvider{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{
		ReemaUserID: uuid.New(),
		Email:       "user@example.com",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPromptRequiresOAuthWhenMissing(t *testing.T) {
	handler := handlers.NewPromptHandler(agent.NewSkeletonClient(), stubTokenProvider{err: useroauth.ErrConsentRequired})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader([]byte(`{"prompt":"hello"}`)))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{
		ReemaUserID: uuid.New(),
		Email:       "user@example.com",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusPreconditionRequired, rec.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["error"] != "google_oauth_required" {
		t.Fatalf("error = %q", payload["error"])
	}
}

func TestPromptAcceptsOAuthHeader(t *testing.T) {
	stub := &stubAgent{chunks: []string{"ok"}}
	handler := handlers.NewPromptHandler(stub, stubTokenProvider{err: useroauth.ErrConsentRequired})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader([]byte(`{"prompt":"hello"}`)))
	req.Header.Set(handlers.GCPAccessTokenHeader, "user-oauth-token")
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{
		ReemaUserID: uuid.New(),
		Email:       "user@example.com",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if stub.lastIn.UserOAuthToken != "user-oauth-token" {
		t.Fatalf("expected oauth token, got %q", stub.lastIn.UserOAuthToken)
	}
}

func TestPromptAcceptsOAuthInBody(t *testing.T) {
	stub := &stubAgent{chunks: []string{"ok"}}
	handler := handlers.NewPromptHandler(stub, stubTokenProvider{err: useroauth.ErrConsentRequired})
	body, _ := json.Marshal(map[string]string{
		"prompt":         "hello",
		"gcpAccessToken": "body-oauth-token",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader(body))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{
		ReemaUserID: uuid.New(),
		Email:       "user@example.com",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if stub.lastIn.UserOAuthToken != "body-oauth-token" {
		t.Fatalf("expected oauth token from body, got %q", stub.lastIn.UserOAuthToken)
	}
}

func TestPromptUsesProviderToken(t *testing.T) {
	stub := &stubAgent{chunks: []string{"ok"}}
	handler := handlers.NewPromptHandler(stub, stubTokenProvider{token: "provider-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader([]byte(`{"prompt":"hello"}`)))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{
		ReemaUserID: uuid.New(),
		Email:       "user@example.com",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if stub.lastIn.UserOAuthToken != "provider-token" {
		t.Fatalf("expected provider token, got %q", stub.lastIn.UserOAuthToken)
	}
}

func TestStreamPromptSkeletonAgent(t *testing.T) {
	rec := httptest.NewRecorder()
	principal := auth.Principal{ReemaUserID: uuid.New(), Email: "user@example.com"}
	err := handlers.StreamPromptForTest(rec, principal, "hi", "", agent.NewSkeletonClient())
	if err != nil {
		t.Fatalf("stream prompt: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "data: Skeleton token chunk 1") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestSlackEventsURLVerification(t *testing.T) {
	body := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slack/events", nil)
	req = req.WithContext(auth.WithSlackBody(req.Context(), body))

	rec := httptest.NewRecorder()
	handlers.SlackEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"challenge":"abc123"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestSlackEventsAckAndProcessesAsync(t *testing.T) {
	body := []byte(`{"type":"event_callback","team_id":"T123","event":{"type":"message","user":"U123"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slack/events", nil)
	req = req.WithContext(auth.WithSlackBody(req.Context(), body))

	rec := httptest.NewRecorder()
	handlers.SlackEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"status":"accepted"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	time.Sleep(50 * time.Millisecond)
}
