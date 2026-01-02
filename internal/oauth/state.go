// Package oauth provides OAuth state management with CSRF protection.
//
// Phase 18.8: Real OAuth (Gmail Read-Only)
// Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
//
// CRITICAL: State tokens are signed and time-bucketed to prevent CSRF.
// CRITICAL: No goroutines. All operations synchronous.
// CRITICAL: Read-only scopes only.
package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// State represents an OAuth state parameter with CSRF protection.
// The state binds the OAuth flow to a specific circle and time window.
type State struct {
	CircleID       string // Circle this OAuth flow is for
	Nonce          string // Random nonce for uniqueness (16 bytes hex)
	IssuedAtBucket int64  // Unix timestamp floored to 5-minute bucket
	Signature      string // HMAC-SHA256 of canonical string
}

// StateBucketDuration is the time window for state validity.
// States are valid for 2 buckets (10 minutes) to allow for clock skew.
const StateBucketDuration = 5 * time.Minute

// MaxStateAge is the maximum age of a valid state.
const MaxStateAge = 10 * time.Minute

// ErrInvalidState indicates the state parameter is malformed or invalid.
var ErrInvalidState = errors.New("invalid oauth state")

// ErrExpiredState indicates the state has expired.
var ErrExpiredState = errors.New("expired oauth state")

// ErrInvalidSignature indicates the state signature does not match.
var ErrInvalidSignature = errors.New("invalid state signature")

// StateManager creates and validates OAuth states.
type StateManager struct {
	secretKey []byte
	clock     func() time.Time
}

// NewStateManager creates a new StateManager with the given secret key.
// The secret key should be at least 32 bytes for security.
func NewStateManager(secretKey []byte, clock func() time.Time) *StateManager {
	return &StateManager{
		secretKey: secretKey,
		clock:     clock,
	}
}

// GenerateState creates a new OAuth state for the given circle.
func (m *StateManager) GenerateState(circleID string) (*State, error) {
	// Generate random nonce
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// Compute time bucket
	now := m.clock()
	bucket := now.Unix() / int64(StateBucketDuration.Seconds())
	bucketTime := bucket * int64(StateBucketDuration.Seconds())

	state := &State{
		CircleID:       circleID,
		Nonce:          nonce,
		IssuedAtBucket: bucketTime,
	}

	// Sign the state
	state.Signature = m.sign(state)

	return state, nil
}

// sign computes the HMAC-SHA256 signature of the state.
func (m *StateManager) sign(s *State) string {
	canonical := s.CanonicalStringWithoutSignature()
	mac := hmac.New(sha256.New, m.secretKey)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// Encode encodes the state as a URL-safe base64 string.
func (s *State) Encode() string {
	canonical := s.CanonicalString()
	return base64.URLEncoding.EncodeToString([]byte(canonical))
}

// DecodeState decodes a state from a URL-safe base64 string.
func DecodeState(encoded string) (*State, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidState
	}

	canonical := string(data)
	return ParseState(canonical)
}

// ParseState parses a state from its canonical string representation.
func ParseState(canonical string) (*State, error) {
	// Format: OAUTH_STATE|v1|{circle_id}|{nonce}|{bucket}|{signature}
	parts := strings.Split(canonical, "|")
	if len(parts) != 6 {
		return nil, ErrInvalidState
	}

	if parts[0] != "OAUTH_STATE" || parts[1] != "v1" {
		return nil, ErrInvalidState
	}

	var bucket int64
	if _, err := fmt.Sscanf(parts[4], "%d", &bucket); err != nil {
		return nil, ErrInvalidState
	}

	return &State{
		CircleID:       parts[2],
		Nonce:          parts[3],
		IssuedAtBucket: bucket,
		Signature:      parts[5],
	}, nil
}

// CanonicalString returns the canonical string representation of the state.
func (s *State) CanonicalString() string {
	return fmt.Sprintf("OAUTH_STATE|v1|%s|%s|%d|%s",
		s.CircleID, s.Nonce, s.IssuedAtBucket, s.Signature)
}

// CanonicalStringWithoutSignature returns the canonical string without signature.
func (s *State) CanonicalStringWithoutSignature() string {
	return fmt.Sprintf("OAUTH_STATE|v1|%s|%s|%d",
		s.CircleID, s.Nonce, s.IssuedAtBucket)
}

// Hash returns the SHA256 hash of the state.
func (s *State) Hash() string {
	data := []byte(s.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ValidateState validates a state string and returns the parsed state.
// It checks the signature and expiration.
func (m *StateManager) ValidateState(encoded string) (*State, error) {
	state, err := DecodeState(encoded)
	if err != nil {
		return nil, err
	}

	// Verify signature
	expectedSig := m.sign(state)
	if !hmac.Equal([]byte(state.Signature), []byte(expectedSig)) {
		return nil, ErrInvalidSignature
	}

	// Check expiration
	now := m.clock()
	stateTime := time.Unix(state.IssuedAtBucket, 0)
	if now.Sub(stateTime) > MaxStateAge {
		return nil, ErrExpiredState
	}

	return state, nil
}
