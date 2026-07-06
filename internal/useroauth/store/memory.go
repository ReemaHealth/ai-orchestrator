package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// MemoryStore is an in-process RefreshTokenStore for tests and local development.
type MemoryStore struct {
	mu     sync.RWMutex
	tokens map[uuid.UUID]*oauth2.Token
}

// NewMemoryStore returns an empty in-memory refresh token store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tokens: make(map[uuid.UUID]*oauth2.Token)}
}

// Save implements RefreshTokenStore.
func (s *MemoryStore) Save(_ context.Context, userID uuid.UUID, _ string, token *oauth2.Token) error {
	if token == nil || token.RefreshToken == "" {
		return fmt.Errorf("refresh token is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *token
	s.tokens[userID] = &clone
	return nil
}

// Load implements RefreshTokenStore.
func (s *MemoryStore) Load(_ context.Context, userID uuid.UUID) (*oauth2.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.tokens[userID]
	if !ok {
		return nil, fmt.Errorf("refresh token not found")
	}
	clone := *token
	return &clone, nil
}

// Delete implements RefreshTokenStore.
func (s *MemoryStore) Delete(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, userID)
	return nil
}
