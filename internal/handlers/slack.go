package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"

	"ai-orchestration/internal/auth"
)

const (
	slackTypeURLVerification = "url_verification"
	slackTypeEventCallback   = "event_callback"
)

type slackPayload struct {
	Type      string          `json:"type"`
	Challenge string          `json:"challenge"`
	TeamID    string          `json:"team_id"`
	Event     json.RawMessage `json:"event"`
}

// SlackEvents handles POST /api/v1/slack/events. Requires verified body on context (set by SlackAuth).
// Dispatches by payload type: url_verification challenge echo, event_callback async ack.
func SlackEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, ok := auth.SlackBodyFromContext(r.Context())
	if !ok {
		http.Error(w, "missing payload", http.StatusInternalServerError)
		return
	}

	var payload slackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case slackTypeURLVerification:
		writeJSON(w, http.StatusOK, map[string]string{"challenge": payload.Challenge})
		return
	case slackTypeEventCallback:
		writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
		go processSlackEvent(bytes.Clone(body))
		return
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
	}
}

func processSlackEvent(body []byte) {
	var payload slackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("slack event: parse payload: %v", err)
		return
	}

	// TODO: map event user + team_id to reemaUserId, then invoke ADK agent.
	ctx := context.Background()
	_ = ctx
	log.Printf("slack event: team_id=%s type=%s (agent dispatch not wired)", payload.TeamID, payload.Type)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
