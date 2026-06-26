package auth

import (
	"context"

	"github.com/google/uuid"
)

// Principal holds verified caller identity after Firebase JWT authentication.
type Principal struct {
	ReemaUserID uuid.UUID
	Email       string // verified JWT email claim; used as Agent Engine user_id for enterprise ACLs
}

type principalKey struct{}
type slackBodyKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext returns the verified Firebase principal, if present.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// WithSlackBody attaches the verified raw Slack request body to context.
func WithSlackBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, slackBodyKey{}, body)
}

// SlackBodyFromContext returns the verified Slack payload bytes, if present.
func SlackBodyFromContext(ctx context.Context) ([]byte, bool) {
	body, ok := ctx.Value(slackBodyKey{}).([]byte)
	return body, ok
}
