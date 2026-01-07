// Package persist provides TrueLayer token storage.
//
// Phase 31.3b: Real TrueLayer Sync Integration
// Reference: docs/ADR/ADR-0066-phase31-3b-truelayer-real-sync.md
//
// CRITICAL INVARIANTS:
//   - In-memory only - tokens are NOT persisted to disk
//   - Bounded retention - tokens expire and are cleaned up
//   - No goroutines. No time.Now() - clock injection only.
//   - Never log token values (SENSITIVE)
package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// TrueLayerTokenStore stores TrueLayer OAuth tokens in memory.
// Thread-safe, bounded retention, no persistence to disk.
//
// SECURITY: Tokens are stored only in memory and never logged.
// This is acceptable for demo/sandbox but production should use
// encrypted storage with secure key management.
type TrueLayerTokenStore struct {
	mu sync.RWMutex

	// tokens stores access tokens by circleID
	// SENSITIVE: Never log these values
	tokens map[string]*TrueLayerTokenEntry

	// clock for time operations
	clock func() time.Time
}

// TrueLayerTokenEntry holds OAuth tokens for a circle.
// CRITICAL: Never log or expose these values.
type TrueLayerTokenEntry struct {
	// CircleID identifies the circle
	CircleID string

	// AccessToken is the OAuth access token
	// SENSITIVE: Never log this value
	AccessToken string

	// RefreshToken is the OAuth refresh token
	// SENSITIVE: Never log this value
	RefreshToken string

	// ExpiresAt is when the access token expires
	ExpiresAt time.Time

	// CreatedAt is when this entry was created
	CreatedAt time.Time

	// TokenHash is a hash of the token for audit/correlation
	// (Safe to log - does not reveal the actual token)
	TokenHash string
}

// NewTrueLayerTokenStore creates a new token store.
func NewTrueLayerTokenStore(clock func() time.Time) *TrueLayerTokenStore {
	return &TrueLayerTokenStore{
		tokens: make(map[string]*TrueLayerTokenEntry),
		clock:  clock,
	}
}

// StoreToken stores OAuth tokens for a circle.
// CRITICAL: Never log the actual token values.
func (s *TrueLayerTokenStore) StoreToken(circleID, accessToken, refreshToken string, expiresIn int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)

	// Compute token hash for audit (never log the actual token)
	tokenHash := computeTokenHash(circleID, accessToken)

	s.tokens[circleID] = &TrueLayerTokenEntry{
		CircleID:     circleID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
		TokenHash:    tokenHash,
	}
}

// GetToken retrieves the access token for a circle.
// Returns empty string if no token exists or token is expired.
// CRITICAL: Never log the returned value.
func (s *TrueLayerTokenStore) GetToken(circleID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry := s.tokens[circleID]
	if entry == nil {
		return ""
	}

	// Check expiration
	now := s.clock()
	if now.After(entry.ExpiresAt) {
		// Token expired - return empty
		// Caller should handle refresh or re-auth
		return ""
	}

	return entry.AccessToken
}

// GetRefreshToken retrieves the refresh token for a circle.
// Returns empty string if no token exists.
// CRITICAL: Never log the returned value.
func (s *TrueLayerTokenStore) GetRefreshToken(circleID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry := s.tokens[circleID]
	if entry == nil {
		return ""
	}

	return entry.RefreshToken
}

// GetTokenHash retrieves the token hash for audit purposes.
// Safe to log - does not reveal actual token.
func (s *TrueLayerTokenStore) GetTokenHash(circleID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry := s.tokens[circleID]
	if entry == nil {
		return ""
	}

	return entry.TokenHash
}

// HasValidToken checks if a circle has a non-expired access token.
func (s *TrueLayerTokenStore) HasValidToken(circleID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry := s.tokens[circleID]
	if entry == nil {
		return false
	}

	now := s.clock()
	return now.Before(entry.ExpiresAt)
}

// RemoveToken removes tokens for a circle.
func (s *TrueLayerTokenStore) RemoveToken(circleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, circleID)
}

// UpdateToken updates the access token after a refresh.
// CRITICAL: Never log the actual token values.
func (s *TrueLayerTokenStore) UpdateToken(circleID, accessToken string, expiresIn int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.tokens[circleID]
	if entry == nil {
		return // No existing entry to update
	}

	now := s.clock()
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)

	entry.AccessToken = accessToken
	entry.ExpiresAt = expiresAt
	entry.TokenHash = computeTokenHash(circleID, accessToken)
}

// ExpireOldTokens removes expired tokens.
// Call periodically for cleanup.
func (s *TrueLayerTokenStore) ExpireOldTokens() {
	now := s.clock()

	s.mu.Lock()
	defer s.mu.Unlock()

	var toRemove []string
	for circleID, entry := range s.tokens {
		if now.After(entry.ExpiresAt) {
			toRemove = append(toRemove, circleID)
		}
	}

	for _, circleID := range toRemove {
		delete(s.tokens, circleID)
	}
}

// Count returns the number of stored tokens.
func (s *TrueLayerTokenStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tokens)
}

// computeTokenHash computes a hash of the token for audit.
// The hash is safe to log and helps correlate actions without exposing the token.
func computeTokenHash(circleID, token string) string {
	input := circleID + "|" + token
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16]) // First 16 bytes for brevity
}
