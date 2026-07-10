package useroauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// revokeGoogleOAuthToken invalidates a refresh or access token at Google.
func revokeGoogleOAuthToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	body := url.Values{"token": {token}}.Encode()
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://oauth2.googleapis.com/revoke",
		strings.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if len(respBody) == 0 {
		return fmt.Errorf("revoke status %d", resp.StatusCode)
	}
	return fmt.Errorf("revoke status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
}
