package agent

// Package agent invokes downstream AI agents. Production uses Vertex AI Reasoning Engine;
// a skeleton implementation supports local development when AGENT_ENABLED=false.
import (
	"context"

	"github.com/google/uuid"
)

// StreamQueryInput carries a prompt and verified caller identity for Agent Engine sessions.
type StreamQueryInput struct {
	Prompt      string
	UserID      string // verified email → Agent Engine user_id for enterprise ACL context
	ReemaUserID uuid.UUID
	SessionID   string // optional; empty generates a new session per request
}

// Client streams agent output chunks to the emit callback.
type Client interface {
	StreamQuery(ctx context.Context, in StreamQueryInput, emit func(chunk string) error) error
}
