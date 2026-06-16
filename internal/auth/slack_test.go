package auth_test

import (
	"strconv"
	"testing"
	"time"

	"ai-orchestration/internal/auth"
)

func TestVerifySlackRequest(t *testing.T) {
	secret := "test-signing-secret"
	body := []byte(`{"type":"event_callback"}`)
	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	signature := auth.SignSlackRequest(timestamp, body, secret)

	if err := auth.VerifySlackRequestWithNow(timestamp, signature, body, secret, time.Unix(ts, 0)); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestVerifySlackRequestMissingHeaders(t *testing.T) {
	err := auth.VerifySlackRequest("", "v0=abc", []byte("{}"), "secret")
	if err != auth.ErrNoCredentials {
		t.Fatalf("expected ErrNoCredentials, got %v", err)
	}
}

func TestVerifySlackRequestBadSignature(t *testing.T) {
	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	body := []byte(`{"type":"event_callback"}`)

	err := auth.VerifySlackRequestWithNow(timestamp, "v0=deadbeef", body, "secret", time.Unix(ts, 0))
	if err != auth.ErrVerificationFailed {
		t.Fatalf("expected ErrVerificationFailed, got %v", err)
	}
}

func TestVerifySlackRequestStaleTimestamp(t *testing.T) {
	secret := "test-signing-secret"
	body := []byte(`{"type":"event_callback"}`)
	ts := time.Now().Add(-10 * time.Minute).Unix()
	timestamp := strconv.FormatInt(ts, 10)
	signature := auth.SignSlackRequest(timestamp, body, secret)

	err := auth.VerifySlackRequestWithNow(timestamp, signature, body, secret, time.Now())
	if err != auth.ErrVerificationFailed {
		t.Fatalf("expected ErrVerificationFailed, got %v", err)
	}
}
