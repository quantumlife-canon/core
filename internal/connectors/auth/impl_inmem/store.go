// Package impl_inmem provides in-memory implementation of the token broker.
// This file implements encrypted token storage.
//
// CRITICAL: This is for demo/testing only. Production requires persistent storage.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package impl_inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/auth"
)

// TokenStore stores encrypted refresh tokens in memory.
// Tokens are encrypted at rest using the TokenEncryptor.
type TokenStore struct {
	mu        sync.RWMutex
	encryptor *TokenEncryptor
	tokens    map[string]*StoredToken // key: circleID:provider
	idCounter int
}

// StoredToken represents an encrypted token in storage.
type StoredToken struct {
	// ID is the unique identifier for this token.
	ID string

	// CircleID is the circle that owns this token.
	CircleID string

	// Provider is the OAuth provider.
	Provider auth.ProviderID

	// EncryptedRefreshToken is the encrypted refresh token.
	// CRITICAL: Never log or expose this value.
	EncryptedRefreshToken string

	// Scopes are the granted scopes (QuantumLife format).
	Scopes []string

	// CreatedAt is when the token was stored.
	CreatedAt time.Time

	// ExpiresAt is when the refresh token expires (if known).
	ExpiresAt time.Time
}

// NewTokenStore creates a new token store.
func NewTokenStore(encryptionKey string) *TokenStore {
	return &TokenStore{
		encryptor: NewTokenEncryptor(encryptionKey),
		tokens:    make(map[string]*StoredToken),
	}
}

// tokenKey generates the storage key for a circle/provider combination.
func tokenKey(circleID string, provider auth.ProviderID) string {
	return fmt.Sprintf("%s:%s", circleID, provider)
}

// Store stores an encrypted refresh token.
func (s *TokenStore) Store(ctx context.Context, circleID string, provider auth.ProviderID, refreshToken string, scopes []string, expiresAt time.Time) (auth.TokenHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Encrypt the refresh token
	encryptedToken, err := s.encryptor.EncryptString(refreshToken)
	if err != nil {
		return auth.TokenHandle{}, fmt.Errorf("failed to encrypt token: %w", err)
	}

	s.idCounter++
	tokenID := fmt.Sprintf("token-%d", s.idCounter)
	now := time.Now()

	token := &StoredToken{
		ID:                    tokenID,
		CircleID:              circleID,
		Provider:              provider,
		EncryptedRefreshToken: encryptedToken,
		Scopes:                scopes,
		CreatedAt:             now,
		ExpiresAt:             expiresAt,
	}

	key := tokenKey(circleID, provider)
	s.tokens[key] = token

	return auth.TokenHandle{
		ID:        tokenID,
		CircleID:  circleID,
		Provider:  provider,
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// Get retrieves and decrypts a refresh token.
func (s *TokenStore) Get(ctx context.Context, circleID string, provider auth.ProviderID) (string, *StoredToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := tokenKey(circleID, provider)
	token, ok := s.tokens[key]
	if !ok {
		return "", nil, auth.ErrNoToken
	}

	// Check expiry
	if !token.ExpiresAt.IsZero() && time.Now().After(token.ExpiresAt) {
		return "", nil, auth.ErrTokenExpired
	}

	// Decrypt the token
	refreshToken, err := s.encryptor.DecryptString(token.EncryptedRefreshToken)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	return refreshToken, token, nil
}

// Delete removes a stored token.
func (s *TokenStore) Delete(ctx context.Context, circleID string, provider auth.ProviderID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tokenKey(circleID, provider)
	if _, ok := s.tokens[key]; !ok {
		return auth.ErrNoToken
	}

	delete(s.tokens, key)
	return nil
}

// HasToken checks if a token exists for the circle/provider.
func (s *TokenStore) HasToken(ctx context.Context, circleID string, provider auth.ProviderID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := tokenKey(circleID, provider)
	token, ok := s.tokens[key]
	if !ok {
		return false
	}

	// Check expiry
	if !token.ExpiresAt.IsZero() && time.Now().After(token.ExpiresAt) {
		return false
	}

	return true
}

// ListTokens returns all token handles for a circle.
func (s *TokenStore) ListTokens(ctx context.Context, circleID string) []auth.TokenHandle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var handles []auth.TokenHandle
	for _, token := range s.tokens {
		if token.CircleID == circleID {
			handles = append(handles, auth.TokenHandle{
				ID:        token.ID,
				CircleID:  token.CircleID,
				Provider:  token.Provider,
				Scopes:    token.Scopes,
				CreatedAt: token.CreatedAt,
				ExpiresAt: token.ExpiresAt,
			})
		}
	}

	return handles
}

// UpdateToken updates the encrypted refresh token (e.g., after token refresh).
func (s *TokenStore) UpdateToken(ctx context.Context, circleID string, provider auth.ProviderID, newRefreshToken string, newExpiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tokenKey(circleID, provider)
	token, ok := s.tokens[key]
	if !ok {
		return auth.ErrNoToken
	}

	// Encrypt the new token
	encryptedToken, err := s.encryptor.EncryptString(newRefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	token.EncryptedRefreshToken = encryptedToken
	token.ExpiresAt = newExpiresAt

	return nil
}
