package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"ai-orchestration/internal/agent"
	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/useroauth"
)

// PromptHandler handles POST /api/v1/prompt with SSE streaming from an agent Client.
type PromptHandler struct {
	Agent         agent.Client
	TokenProvider useroauth.TokenProvider
}

// GCPAccessTokenHeader is the HTTP header clients may send with a GE OAuth token (Drive scopes).
// When set, it overrides server-side token resolution.
const GCPAccessTokenHeader = "X-GCP-Access-Token"

// NewPromptHandler returns a handler that streams agent output for authenticated prompts.
func NewPromptHandler(client agent.Client, tokens useroauth.TokenProvider) *PromptHandler {
	if tokens == nil {
		tokens = &useroauthNoopAdapter{}
	}
	return &PromptHandler{Agent: client, TokenProvider: tokens}
}

// useroauthNoopAdapter satisfies TokenProvider when nil is passed (tests).
type useroauthNoopAdapter struct{}

func (useroauthNoopAdapter) AccessToken(_ context.Context, _ auth.Principal, clientOverride string) (string, error) {
	return strings.TrimSpace(clientOverride), nil
}

func (useroauthNoopAdapter) ConsentRequired() bool { return false }

func (useroauthNoopAdapter) AuthorizePath() string { return "" }

// ServeHTTP streams SSE output after Firebase auth (Principal on context).
// Auth: Firebase JWT required (Authorization header). User-scoped GCP access for workspace
// datastores is resolved server-side from stored refresh tokens when OAuth is configured,
// or from optional X-GCP-Access-Token / JSON gcpAccessToken override.
// Request: JSON {"prompt":"..."} (prompt required).
// Response: 200 text/event-stream (meta event + agent chunks), 400 missing prompt,
// 401 no principal, 428 google oauth consent required, 502 agent unavailable before stream starts.
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

	clientOverride := strings.TrimSpace(r.Header.Get(GCPAccessTokenHeader))
	if clientOverride == "" {
		clientOverride = strings.TrimSpace(req.GCPAccessToken)
	}

	oauthToken, err := h.TokenProvider.AccessToken(r.Context(), principal, clientOverride)
	if err != nil {
		if errors.Is(err, useroauth.ErrConsentRequired) {
			writeConsentRequired(w, h.TokenProvider.AuthorizePath())
			return
		}
		http.Error(w, "user oauth unavailable", http.StatusBadGateway)
		return
	}

	if err := streamPrompt(r.Context(), w, h.Agent, principal, req.Prompt, oauthToken); err != nil {
		log.Printf("prompt agent error (user=%s prompt=%q): %v", principal.Email, req.Prompt, err)
		if headersSent(w) {
			writeSSEError(w, sanitizeAgentError(err))
			return
		}
		http.Error(w, "agent unavailable", http.StatusBadGateway)
	}
}

type promptRequest struct {
	Prompt         string `json:"prompt"`
	GCPAccessToken string `json:"gcpAccessToken,omitempty"`
}

// StreamPromptForTest streams SSE via a mock agent for handler tests.
func StreamPromptForTest(w http.ResponseWriter, principal auth.Principal, prompt, oauthToken string, client agent.Client) error {
	return streamPrompt(context.Background(), w, client, principal, prompt, oauthToken)
}

func streamPrompt(ctx context.Context, w http.ResponseWriter, client agent.Client, principal auth.Principal, prompt, oauthToken string) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	meta, _ := json.Marshal(map[string]string{
		"reemaUserId": principal.ReemaUserID.String(),
		"email":       principal.Email,
	})
	_, _ = fmt.Fprintf(w, "event: meta\ndata: %s\n\n", meta)
	flusher.Flush()

	log.Printf("prompt stream: user_id=%s reema_user_id=%s oauth_token_present=%t", principal.Email, principal.ReemaUserID, oauthToken != "")

	err := client.StreamQuery(ctx, agent.StreamQueryInput{
		Prompt:         prompt,
		UserID:         principal.Email,
		ReemaUserID:    principal.ReemaUserID,
		UserOAuthToken: oauthToken,
	}, func(chunk string) error {
		if strings.TrimSpace(chunk) == "" {
			return nil
		}
		writeSSEData(w, chunk)
		flusher.Flush()
		return nil
	})
	if err != nil {
		return err
	}

	writeSSEDone(w)
	flusher.Flush()
	return nil
}

// writeSSEDone emits a terminal SSE event so clients (e.g. Postman) can detect stream completion.
func writeSSEDone(w http.ResponseWriter) {
	_, _ = fmt.Fprintf(w, "event: done\ndata: {}\n\n")
}

// writeSSEData writes one SSE message, prefixing each line with "data:" per the SSE spec.
// Without this, embedded newlines break clients (e.g. Postman shows "(empty)" for each line).
func writeSSEData(w http.ResponseWriter, chunk string) {
	chunk = strings.TrimSpace(strings.ReplaceAll(chunk, "\r\n", "\n"))
	if chunk == "" {
		return
	}
	for _, line := range strings.Split(chunk, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}

func writeConsentRequired(w http.ResponseWriter, authorizePath string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPreconditionRequired)
	payload, _ := json.Marshal(map[string]string{
		"error":         "google_oauth_required",
		"authorizePath": authorizePath,
		"message":       "Complete Google OAuth consent before using workspace datastore features",
	})
	_, _ = w.Write(payload)
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

// sanitizeAgentError returns a client-safe error string for SSE (truncated, no secrets).
func sanitizeAgentError(err error) string {
	if err == nil {
		return "agent error"
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	return msg
}
