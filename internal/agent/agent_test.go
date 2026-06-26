package agent_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-orchestration/internal/agent"

	"github.com/google/uuid"
)

func TestSkeletonClientStreamQuery(t *testing.T) {
	client := agent.NewSkeletonClient()
	var chunks []string

	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hello",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}
	if len(chunks) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(chunks))
	}
}

func TestVertexClientStreamQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"output\":\"hello \"}\n\n")
		_, _ = io.WriteString(w, "data: {\"output\":\"world\"}\n\n")
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), server.URL, "stream_query")

	var chunks []string
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}
	if strings.Join(chunks, "") != "hello world" {
		t.Fatalf("unexpected chunks: %q", chunks)
	}
}

func TestVertexClientStreamQueryErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), server.URL, "stream_query")
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
	}, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}
