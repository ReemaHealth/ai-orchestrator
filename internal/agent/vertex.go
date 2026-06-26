package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"ai-orchestration/internal/config"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// VertexClient calls Vertex AI Reasoning Engine :streamQuery using Application Default Credentials.
type VertexClient struct {
	httpClient    *http.Client
	resourceName  string
	streamURL     string
	classMethod   string
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

	return &VertexClient{
		httpClient: oauthHTTPClient(ctx, creds),
		resourceName: resourceName,
		streamURL:    streamURL,
		classMethod:  cfg.AgentClassMethod,
	}, nil
}

// NewVertexClientWithHTTP allows injecting HTTP client and URL for tests.
func NewVertexClientWithHTTP(httpClient *http.Client, streamURL, classMethod string) *VertexClient {
	return &VertexClient{
		httpClient:  httpClient,
		streamURL:   streamURL,
		classMethod: classMethod,
	}
}

// StreamQuery invokes Reasoning Engine streamQuery and emits extracted text chunks.
func (c *VertexClient) StreamQuery(ctx context.Context, in StreamQueryInput, emit func(chunk string) error) error {
	if strings.TrimSpace(in.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(in.UserID) == "" {
		return fmt.Errorf("user id is required")
	}

	input := map[string]any{
		"message":       in.Prompt,
		"user_id":       in.UserID,
		"reema_user_id": in.ReemaUserID.String(),
	}
	if strings.TrimSpace(in.SessionID) != "" {
		input["session_id"] = in.SessionID
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

	return readStreamChunks(resp.Body, emit)
}

func readStreamChunks(r io.Reader, emit func(chunk string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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

		chunks, err := extractChunks(line)
		if err != nil {
			return err
		}
		for _, chunk := range chunks {
			if chunk == "" {
				continue
			}
			if err := emit(chunk); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	return nil
}

func extractChunks(line string) ([]string, error) {
	var raw any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return []string{line}, nil
	}

	texts := collectTextValues(raw)
	if len(texts) == 0 {
		return nil, nil
	}
	return texts, nil
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
		var out []string
		for _, key := range []string{"output", "text", "chunk", "content", "message", "response"} {
			if val, ok := t[key]; ok {
				out = append(out, collectTextValues(val)...)
			}
		}
		if len(out) > 0 {
			return out
		}
		for _, val := range t {
			out = append(out, collectTextValues(val)...)
		}
		return out
	default:
		return nil
	}
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
