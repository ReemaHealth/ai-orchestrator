package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-orchestration/internal/agent"
	"ai-orchestration/internal/auth"
)

// PromptHandler handles POST /api/v1/prompt with SSE streaming from an agent Client.
type PromptHandler struct {
	Agent agent.Client
}

// NewPromptHandler returns a handler that streams agent output for authenticated prompts.
func NewPromptHandler(client agent.Client) *PromptHandler {
	return &PromptHandler{Agent: client}
}

// ServeHTTP streams SSE output after Firebase auth (Principal on context).
func (h *PromptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req promptRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	if err := streamPrompt(r.Context(), w, h.Agent, principal, req.Prompt); err != nil {
		if headersSent(w) {
			writeSSEError(w, "agent error")
			return
		}
		http.Error(w, "agent unavailable", http.StatusBadGateway)
	}
}

type promptRequest struct {
	Prompt string `json:"prompt"`
}

// StreamPromptForTest streams SSE via a mock agent for handler tests.
func StreamPromptForTest(w http.ResponseWriter, principal auth.Principal, prompt string, client agent.Client) error {
	return streamPrompt(context.Background(), w, client, principal, prompt)
}

func streamPrompt(ctx context.Context, w http.ResponseWriter, client agent.Client, principal auth.Principal, prompt string) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	meta, _ := json.Marshal(map[string]string{
		"reemaUserId": principal.ReemaUserID.String(),
	})
	_, _ = fmt.Fprintf(w, "event: meta\ndata: %s\n\n", meta)
	flusher.Flush()

	return client.StreamQuery(ctx, agent.StreamQueryInput{
		Prompt:      prompt,
		UserID:      principal.Email,
		ReemaUserID: principal.ReemaUserID,
	}, func(chunk string) error {
		_, writeErr := fmt.Fprintf(w, "data: %s\n\n", chunk)
		if writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	})
}

func writeSSEError(w http.ResponseWriter, message string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	payload, _ := json.Marshal(map[string]string{"error": message})
	_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", payload)
	flusher.Flush()
}

func headersSent(w http.ResponseWriter) bool {
	return w.Header().Get("Content-Type") == "text/event-stream"
}
