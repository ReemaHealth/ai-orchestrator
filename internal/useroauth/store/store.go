package store

import (
	"context"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// RefreshTokenStore persists Google OAuth refresh grants keyed by Reema user id.
type RefreshTokenStore interface {
	// Save stores or replaces the refresh token for userID.
	Save(ctx context.Context, userID uuid.UUID, email string, token *oauth2.Token) error
	// Load returns the stored token for userID.
	Load(ctx context.Context, userID uuid.UUID) (*oauth2.Token, error)
	// Delete removes the stored token for userID.
	Delete(ctx context.Context, userID uuid.UUID) error
}
