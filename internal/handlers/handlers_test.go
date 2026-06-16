package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/handlers"
)

func TestPromptStreamsSSE(t *testing.T) {
	reemaUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	handlers.StreamPromptForTest(rec, reemaUserID, "hello", 0)

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
	if !strings.Contains(body, "data: Skeleton token chunk 5") {
		t.Fatalf("expected final chunk, got: %s", body)
	}
}

func TestPromptRequiresPrincipal(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompt", nil)
	rec := httptest.NewRecorder()

	handlers.Prompt(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStreamPromptNoDelay(t *testing.T) {
	rec := httptest.NewRecorder()
	handlers.StreamPromptForTest(rec, uuid.New(), "hi", 0)

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
