package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"ai-orchestration/internal/agent"
	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/handlers"
)

type stubAgent struct {
	chunks []string
}

func (s stubAgent) StreamQuery(_ context.Context, _ agent.StreamQueryInput, emit func(chunk string) error) error {
	for _, chunk := range s.chunks {
		if err := emit(chunk); err != nil {
			return err
		}
	}
	return nil
}

func TestPromptStreamsSSE(t *testing.T) {
	reemaUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	principal := auth.Principal{
		ReemaUserID: reemaUserID,
		Email:       "user@example.com",
	}
	err := handlers.StreamPromptForTest(rec, principal, "hello", stubAgent{chunks: []string{"chunk one", "chunk two"}})
	if err != nil {
		t.Fatalf("stream prompt: %v", err)
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
	if !strings.Contains(body, "data: chunk two") {
		t.Fatalf("expected agent chunk, got: %s", body)
	}
}

func TestPromptRequiresPrincipal(t *testing.T) {
	handler := handlers.NewPromptHandler(agent.NewSkeletonClient())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", bytes.NewReader([]byte(`{"prompt":"hello"}`)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestPromptRequiresPromptBody(t *testing.T) {
	handler := handlers.NewPromptHandler(agent.NewSkeletonClient())
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

func TestStreamPromptSkeletonAgent(t *testing.T) {
	rec := httptest.NewRecorder()
	principal := auth.Principal{ReemaUserID: uuid.New(), Email: "user@example.com"}
	err := handlers.StreamPromptForTest(rec, principal, "hi", agent.NewSkeletonClient())
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
