package auth

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

type Principal struct {
	ReemaUserID uuid.UUID
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok
}
