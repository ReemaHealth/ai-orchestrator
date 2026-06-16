package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"ai-orchestration/internal/auth"
)

const promptChunkDelay = 500 * time.Millisecond

type promptRequest struct {
	Prompt string `json:"prompt"`
}

func Prompt(w http.ResponseWriter, r *http.Request) {
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

	streamPrompt(w, principal.ReemaUserID, req.Prompt, promptChunkDelay)
}

// StreamPromptForTest exposes streamPrompt for tests with a configurable chunk delay.
func StreamPromptForTest(w http.ResponseWriter, reemaUserID uuid.UUID, prompt string, chunkDelay time.Duration) {
	streamPrompt(w, reemaUserID, prompt, chunkDelay)
}

func streamPrompt(w http.ResponseWriter, reemaUserID uuid.UUID, prompt string, chunkDelay time.Duration) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	meta, _ := json.Marshal(map[string]string{
		"reemaUserId": reemaUserID.String(),
	})
	_, _ = fmt.Fprintf(w, "event: meta\ndata: %s\n\n", meta)
	flusher.Flush()

	_ = prompt // reserved for ADK agent invocation

	for i := 1; i <= 5; i++ {
		_, _ = fmt.Fprintf(w, "data: Skeleton token chunk %d\n\n", i)
		flusher.Flush()
		if chunkDelay > 0 {
			time.Sleep(chunkDelay)
		}
	}
}
