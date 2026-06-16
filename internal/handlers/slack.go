package handlers

import "net/http"

func SlackEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// SlackEventsFuture acknowledges Slack immediately and processes events in the background.
// See docs/endpoints.md for the intended production behavior.
func SlackEventsFuture(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
	// TODO: Spin out a background worker context here to call the ADK agent asynchronously.
}
