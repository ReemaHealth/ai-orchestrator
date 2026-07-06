package agent_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"ai-orchestration/internal/agent"
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

	client := agent.NewVertexClientWithHTTP(server.Client(), "", server.URL, "async_stream_query", "")

	var chunks []string
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
		SessionID:   "existing-session",
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

func TestVertexClientStreamQueryADKParts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"parts\":[{\"text\":\"Hello \"}],\"role\":\"model\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"parts\":[{\"text\":\"world\"}],\"role\":\"model\"}\n\n")
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), "", server.URL, "async_stream_query", "")

	var chunks []string
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
		SessionID:   "session-1",
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}
	if strings.Join(chunks, "") != "Hello world" {
		t.Fatalf("unexpected chunks: %q", chunks)
	}
}

func TestVertexClientCreatesSession(t *testing.T) {
	var gotCreateSession bool
	streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"parts\":[{\"text\":\"ok\"}],\"role\":\"model\"}\n\n")
	}))
	t.Cleanup(streamServer.Close)

	queryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		gotCreateSession = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"output":{"id":"4857885913439920384"}}`)
	}))
	t.Cleanup(queryServer.Close)

	client := agent.NewVertexClientWithHTTP(streamServer.Client(), queryServer.URL, streamServer.URL, "async_stream_query", "")

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
	if !gotCreateSession {
		t.Fatal("expected async_create_session call")
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Fatalf("unexpected chunks: %q", chunks)
	}
}

func TestVertexClientStreamQueryContentWrappedParts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"content\":{\"parts\":[{\"text\":\"wrapped\"}],\"role\":\"model\"}}\n\n")
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), "", server.URL, "async_stream_query", "")

	var chunks []string
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
		SessionID:   "session-1",
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}
	if strings.Join(chunks, "") != "wrapped" {
		t.Fatalf("unexpected chunks: %q", chunks)
	}
}

func TestVertexClientStreamQueryADKToolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"content":{"parts":[{"function_call":{"name":"search_documents"}}],"role":"model"}}`+"\n\n")
		_, _ = io.WriteString(w, `data: {"error_code":"RuntimeError","error_message":"Discovery Engine search failed (403)"}`+"\n\n")
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), "", server.URL, "async_stream_query", "")

	var chunks []string
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "kudos?",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
		SessionID:   "session-1",
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}
	if len(chunks) != 1 || !strings.Contains(chunks[0], "403") {
		t.Fatalf("expected tool error in output, got %q", chunks)
	}
}

func TestVertexClientCreatesSessionWithOAuthState(t *testing.T) {
	var createBody map[string]any
	streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"parts\":[{\"text\":\"ok\"}],\"role\":\"model\"}\n\n")
	}))
	t.Cleanup(streamServer.Close)

	queryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&createBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"output":{"id":"4857885913439920384"}}`)
	}))
	t.Cleanup(queryServer.Close)

	client := agent.NewVertexClientWithHTTP(streamServer.Client(), queryServer.URL, streamServer.URL, "async_stream_query", "fetch-agent-auth")

	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:         "hello",
		UserID:         "user@example.com",
		ReemaUserID:    uuid.New(),
		UserOAuthToken: "user-oauth-token",
	}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}

	input, _ := createBody["input"].(map[string]any)
	state, _ := input["state"].(map[string]any)
	if state["fetch-agent-auth"] != "user-oauth-token" {
		t.Fatalf("expected oauth token in session state, got %#v", state)
	}
}

func TestVertexClientStreamQueryErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), "", server.URL, "async_stream_query", "")
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
		SessionID:   "session-1",
	}, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestVertexClientStreamQueryNoOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"role\":\"model\"}\n\n")
	}))
	t.Cleanup(server.Close)

	client := agent.NewVertexClientWithHTTP(server.Client(), "", server.URL, "async_stream_query", "")
	err := client.StreamQuery(context.Background(), agent.StreamQueryInput{
		Prompt:      "hi",
		UserID:      "user@example.com",
		ReemaUserID: uuid.New(),
		SessionID:   "session-1",
	}, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "no output") {
		t.Fatalf("expected no output error, got %v", err)
	}
}
