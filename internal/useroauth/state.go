package useroauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const stateTTL = 10 * time.Minute

type oauthStatePayload struct {
	UserID    string `json:"uid"`
	Email     string `json:"email"`
	Nonce     string `json:"nonce"`
	ExpiresAt int64  `json:"exp"`
}

// StateCodec signs and verifies OAuth state parameters binding consent to a Reema user.
type StateCodec struct {
	secret []byte
}

// NewStateCodec returns a codec that HMAC-signs OAuth state with secret.
func NewStateCodec(secret string) (*StateCodec, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("oauth state secret is required")
	}
	return &StateCodec{secret: []byte(secret)}, nil
}

// Encode builds a signed state string for the given user.
func (c *StateCodec) Encode(userID uuid.UUID, email string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	payload := oauthStatePayload{
		UserID:    userID.String(),
		Email:     strings.TrimSpace(email),
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
		ExpiresAt: time.Now().Add(stateTTL).Unix(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal state: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sig := c.sign(encoded)
	return encoded + "." + sig, nil
}

// Decode verifies and parses a signed state string.
func (c *StateCodec) Decode(state string) (uuid.UUID, string, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return uuid.UUID{}, "", fmt.Errorf("invalid state format")
	}

	expectedSig := c.sign(parts[0])
	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return uuid.UUID{}, "", fmt.Errorf("invalid state signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.UUID{}, "", fmt.Errorf("decode state: %w", err)
	}

	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return uuid.UUID{}, "", fmt.Errorf("parse state: %w", err)
	}

	if time.Now().Unix() > payload.ExpiresAt {
		return uuid.UUID{}, "", fmt.Errorf("state expired")
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return uuid.UUID{}, "", fmt.Errorf("invalid user id in state: %w", err)
	}

	return userID, payload.Email, nil
}

func (c *StateCodec) sign(payload string) string {
	mac := hmac.New(sha256.New, c.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
