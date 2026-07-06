package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"ai-orchestration/internal/config"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// VertexClient calls Vertex AI Reasoning Engine :streamQuery using Application Default Credentials.
type VertexClient struct {
	httpClient     *http.Client
	resourceName   string
	queryURL       string
	streamURL      string
	classMethod    string
	geAuthStateKey string
}

// NewVertexClient builds a Vertex Agent Engine client from configuration.
func NewVertexClient(ctx context.Context, cfg config.Config) (*VertexClient, error) {
	creds, err := google.FindDefaultCredentials(ctx, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("adc credentials: %w", err)
	}

	resourceName := fmt.Sprintf(
		"projects/%s/locations/%s/reasoningEngines/%s",
		cfg.GCPProject,
		cfg.GCPLocation,
		cfg.ReasoningEngineID,
	)
	streamURL := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/%s:streamQuery?alt=sse",
		cfg.GCPLocation,
		resourceName,
	)
	queryURL := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/%s:query",
		cfg.GCPLocation,
		resourceName,
	)

	return &VertexClient{
		httpClient:     oauthHTTPClient(ctx, creds),
		resourceName:   resourceName,
		queryURL:       queryURL,
		streamURL:      streamURL,
		classMethod:    cfg.AgentClassMethod,
		geAuthStateKey: cfg.GEAuthStateKey,
	}, nil
}

// NewVertexClientWithHTTP allows injecting HTTP client and URLs for tests.
// Pass empty queryURL to skip session creation (tests supply SessionID directly).
func NewVertexClientWithHTTP(httpClient *http.Client, queryURL, streamURL, classMethod, geAuthStateKey string) *VertexClient {
	return &VertexClient{
		httpClient:     httpClient,
		queryURL:       queryURL,
		streamURL:      streamURL,
		classMethod:    classMethod,
		geAuthStateKey: geAuthStateKey,
	}
}

// StreamQuery invokes Reasoning Engine streamQuery and emits extracted text chunks.
// ADK agents require a session: when SessionID is empty, async_create_session is called first.
func (c *VertexClient) StreamQuery(ctx context.Context, in StreamQueryInput, emit func(chunk string) error) error {
	if strings.TrimSpace(in.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(in.UserID) == "" {
		return fmt.Errorf("user id is required")
	}

	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" && c.queryURL != "" {
		var err error
		sessionID, err = c.createSession(ctx, in.UserID, in.UserOAuthToken)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
	}

	// ADK async_stream_query accepts user_id, session_id, message, and optional state_delta.
	input := map[string]any{
		"message": in.Prompt,
		"user_id": in.UserID,
	}
	if sessionID != "" {
		input["session_id"] = sessionID
	}
	if stateDelta := c.authStateMap(in.UserOAuthToken); stateDelta != nil {
		input["state_delta"] = stateDelta
	}

	body, err := json.Marshal(map[string]any{
		"class_method": c.classMethod,
		"input":        input,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.streamURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream query request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("stream query status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	emitted, err := readStreamChunks(resp.Body, emit)
	if err != nil {
		return err
	}
	if emitted == 0 {
		return fmt.Errorf("agent returned no output (check server logs for raw stream events)")
	}
	return nil
}

// createSession calls async_create_session and returns the new session id.
// When oauthToken is set, it is stored in session state under geAuthStateKey (fetch-agent-auth).
func (c *VertexClient) createSession(ctx context.Context, userID, oauthToken string) (string, error) {
	sessionInput := map[string]any{
		"user_id": userID,
	}
	if state := c.authStateMap(oauthToken); state != nil {
		sessionInput["state"] = state
	}

	body, err := json.Marshal(map[string]any{
		"class_method": "async_create_session",
		"input":        sessionInput,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.queryURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return parseSessionID(respBody)
}

func parseSessionID(body []byte) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	// ADK returns {"output": {"id": "..."}}.
	if output, ok := raw["output"].(map[string]any); ok {
		if id, ok := output["id"].(string); ok && id != "" {
			return id, nil
		}
	}
	if id, ok := raw["id"].(string); ok && id != "" {
		return id, nil
	}
	return "", fmt.Errorf("session id not found in response: %s", strings.TrimSpace(string(body)))
}

func (c *VertexClient) authStateMap(oauthToken string) map[string]any {
	key := strings.TrimSpace(c.geAuthStateKey)
	token := strings.TrimSpace(oauthToken)
	if key == "" || token == "" {
		return nil
	}
	return map[string]any{key: token}
}

func readStreamChunks(r io.Reader, emit func(chunk string) error) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	emitted := 0
	var rawLines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" || line == "[DONE]" {
			continue
		}

		rawLines = append(rawLines, line)

		chunks, err := extractChunks(line)
		if err != nil {
			return emitted, err
		}
		for _, chunk := range chunks {
			if chunk == "" {
				continue
			}
			if err := emit(chunk); err != nil {
				return emitted, err
			}
			emitted++
		}
	}

	if err := scanner.Err(); err != nil {
		return emitted, fmt.Errorf("read stream: %w", err)
	}
	if emitted == 0 && len(rawLines) > 0 {
		const maxLog = 2048
		sample := strings.Join(rawLines, " | ")
		if len(sample) > maxLog {
			sample = sample[:maxLog] + "..."
		}
		log.Printf("vertex stream had %d events but no text; sample: %s", len(rawLines), sample)
	}
	return emitted, nil
}

func extractChunks(line string) ([]string, error) {
	var raw any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return []string{line}, nil
	}

	if errMsg := streamErrorMessage(raw); errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	texts := collectTextValues(raw)
	if len(texts) > 0 {
		return texts, nil
	}

	// Surface ADK tool errors and status when no text is present.
	if msg := adkStatusMessage(raw); msg != "" {
		return []string{msg}, nil
	}

	return nil, nil
}

// adkStatusMessage extracts human-readable status from non-text ADK stream events.
func adkStatusMessage(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if msg, ok := m["error_message"].(string); ok && strings.TrimSpace(msg) != "" {
		return strings.TrimSpace(msg)
	}
	if code, ok := m["error_code"].(string); ok && strings.TrimSpace(code) != "" {
		if msg, ok := m["error_message"].(string); ok {
			return strings.TrimSpace(code + ": " + msg)
		}
		return strings.TrimSpace(code)
	}
	return ""
}

func streamErrorMessage(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"error", "message"} {
		if raw, ok := m[key]; ok {
			switch t := raw.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			case map[string]any:
				if msg, ok := t["message"].(string); ok && strings.TrimSpace(msg) != "" {
					return msg
				}
			}
		}
	}
	return ""
}

func collectTextValues(v any) []string {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			out = append(out, collectTextValues(item)...)
		}
		return out
	case map[string]any:
		// ADK stream events: {"parts":[{"text":"..."}],"role":"model"}
		// or wrapped: {"content":{"parts":[...],"role":"model"}}
		if parts, ok := t["parts"].([]any); ok {
			return collectPartsTexts(parts)
		}
		if content, ok := t["content"].(map[string]any); ok {
			if parts, ok := content["parts"].([]any); ok {
				return collectPartsTexts(parts)
			}
			return collectTextValues(content)
		}
		if output, ok := t["output"]; ok {
			return collectTextValues(output)
		}
		var out []string
		for _, key := range []string{"text", "chunk", "message", "response", "content"} {
			if val, ok := t[key]; ok {
				out = append(out, collectTextValues(val)...)
			}
		}
		return out
	default:
		return nil
	}
}

func collectPartsTexts(parts []any) []string {
	var out []string
	for _, part := range parts {
		m, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
			out = append(out, text)
		}
	}
	return out
}

func oauthHTTPClient(ctx context.Context, creds *google.Credentials) *http.Client {
	return &http.Client{
		Transport: &oauth2Transport{
			base:  http.DefaultTransport,
			token: creds.TokenSource,
		},
	}
}

type oauth2Transport struct {
	base  http.RoundTripper
	token oauth2.TokenSource
}

func (t *oauth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.token.Token()
	if err != nil {
		return nil, err
	}
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}
