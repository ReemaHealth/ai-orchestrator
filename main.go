package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	// Health check endpoint
	http.HandleFunc("/healthz", healthzHandler)

	// Web App Endpoint (Streams responses via SSE)
	http.HandleFunc("/api/v1/prompt", sseHandler)

	// Slack webhook endpoint (Async event handler)
	http.HandleFunc("/api/v1/slack/events", slackHandler)

	// Start the server
	fmt.Println("Starting server on port 8080...")
	http.ListenAndServe(":8080", nil)
}

// healthzHandler responds quickly to let GCP know the container is alive and well
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests for the health check
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for i := 1; i <= 5; i++ {
		fmt.Fprintf(w, "data: Skeleton token chunk %d\n\n", i)
		flusher.Flush() 
		time.Sleep(500 * time.Millisecond)
	}
}

func slackHandler(w http.ResponseWriter, r *http.Request) {
	// Acknowledge Slack immediately within 3s window
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "accepted"}`))
	// TODO: Spin out a background worker context here to call your ADK agent asynchronously
}