package handlers

import (
	"encoding/json"
	"net/http"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/useroauth"
)

// OAuthStatusHandler reports whether the caller has completed Google OAuth consent.
type OAuthStatusHandler struct {
	tokens useroauth.TokenProvider
}

// NewOAuthStatusHandler returns a handler for GET /api/v1/oauth/google/status.
func NewOAuthStatusHandler(tokens useroauth.TokenProvider) *OAuthStatusHandler {
	return &OAuthStatusHandler{tokens: tokens}
}

// ServeHTTP returns {"connected":true} or {"connected":false} (428 when consent required).
func (h *OAuthStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	connected := true
	if h.tokens != nil {
		if !h.tokens.ConsentRequired() {
			connected = true
		} else {
			connected = h.tokens.HasStoredGrant(r.Context(), principal)
		}
	}

	if !connected {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPreconditionRequired)
		payload, _ := json.Marshal(map[string]bool{"connected": false})
		_, _ = w.Write(payload)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	payload, _ := json.Marshal(map[string]bool{"connected": true})
	_, _ = w.Write(payload)
}
