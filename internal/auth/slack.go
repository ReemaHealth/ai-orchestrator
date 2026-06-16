package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const slackMaxRequestAge = 5 * time.Minute

// VerifySlackRequest validates a Slack Events API request per Slack's signing protocol.
func VerifySlackRequest(timestamp, signature string, body []byte, signingSecret string) error {
	if strings.TrimSpace(timestamp) == "" || strings.TrimSpace(signature) == "" {
		return ErrNoCredentials
	}
	if signingSecret == "" {
		return ErrVerificationFailed
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrVerificationFailed
	}

	requestTime := time.Unix(ts, 0)
	age := time.Since(requestTime)
	if age < 0 {
		age = -age
	}
	if age > slackMaxRequestAge {
		return ErrVerificationFailed
	}

	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(fmt.Sprintf("v0:%s:", timestamp)))
	_, _ = mac.Write(body)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) != 1 {
		return ErrVerificationFailed
	}

	return nil
}

// VerifySlackRequestWithNow is like VerifySlackRequest but accepts a clock for testing.
func VerifySlackRequestWithNow(timestamp, signature string, body []byte, signingSecret string, now time.Time) error {
	if strings.TrimSpace(timestamp) == "" || strings.TrimSpace(signature) == "" {
		return ErrNoCredentials
	}
	if signingSecret == "" {
		return ErrVerificationFailed
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrVerificationFailed
	}

	requestTime := time.Unix(ts, 0)
	age := now.Sub(requestTime)
	if age < 0 {
		age = -age
	}
	if age > slackMaxRequestAge {
		return ErrVerificationFailed
	}

	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(fmt.Sprintf("v0:%s:", timestamp)))
	_, _ = mac.Write(body)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) != 1 {
		return ErrVerificationFailed
	}

	return nil
}

// SignSlackRequest builds a valid Slack signature for tests.
func SignSlackRequest(timestamp string, body []byte, signingSecret string) string {
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(fmt.Sprintf("v0:%s:", timestamp)))
	_, _ = mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}
