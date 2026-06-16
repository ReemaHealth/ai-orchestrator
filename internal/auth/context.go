package auth

import (
	"context"

	"github.com/google/uuid"
)

type principalKey struct{}
type slackBodyKey struct{}

type Principal struct {
	ReemaUserID uuid.UUID
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func WithSlackBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, slackBodyKey{}, body)
}

func SlackBodyFromContext(ctx context.Context) ([]byte, bool) {
	body, ok := ctx.Value(slackBodyKey{}).([]byte)
	return body, ok
}
