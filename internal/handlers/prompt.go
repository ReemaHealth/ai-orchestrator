package handlers

import (
	"net/http"
	"strconv"
	"time"
)

func Prompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// PromptSSEFuture streams agent output via Server-Sent Events.
// See docs/endpoints.md for the intended production behavior.
func PromptSSEFuture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	for i := 1; i <= 5; i++ {
		_, _ = w.Write([]byte("data: Skeleton token chunk " + strconv.Itoa(i) + "\n\n"))
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
	}
}
