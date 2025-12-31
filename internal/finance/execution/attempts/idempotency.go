// Package attempts provides idempotency and replay defense primitives for v9.6 financial execution.
package attempts

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// IdempotencyKeyInput contains the inputs for deriving an idempotency key.
type IdempotencyKeyInput struct {
	// EnvelopeID is the sealed envelope identifier.
	EnvelopeID string

	// ActionHash is the deterministic hash of the action specification.
	ActionHash string

	// AttemptID uniquely identifies this execution attempt.
	AttemptID string

	// SealHash is the envelope's seal hash for integrity.
	SealHash string
}

// DeriveIdempotencyKey computes a deterministic idempotency key from the inputs.
//
// The key is derived using HMAC-SHA256 with a fixed domain separator.
// This ensures:
// - Determinism: same inputs always produce the same key
// - Uniqueness: different inputs produce different keys
// - Collision resistance: SHA256 provides cryptographic guarantees
//
// CRITICAL: The derived key is safe for logging (prefix only recommended).
// Full key should only be sent to providers.
func DeriveIdempotencyKey(input IdempotencyKeyInput) string {
	// Use a fixed domain separator for HMAC key
	// This is not a secret - it's a domain separator to avoid collisions
	domainSeparator := []byte("quantumlife-v96-idempotency")

	h := hmac.New(sha256.New, domainSeparator)

	// Write inputs in deterministic order with separators
	h.Write([]byte("envelope_id:"))
	h.Write([]byte(input.EnvelopeID))
	h.Write([]byte("|action_hash:"))
	h.Write([]byte(input.ActionHash))
	h.Write([]byte("|attempt_id:"))
	h.Write([]byte(input.AttemptID))
	h.Write([]byte("|seal_hash:"))
	h.Write([]byte(input.SealHash))

	return hex.EncodeToString(h.Sum(nil))
}

// IdempotencyKeyPrefix returns a safe-to-log prefix of the idempotency key.
// Uses first 16 characters (64 bits) which is sufficient for identification
// while avoiding full key exposure in logs.
func IdempotencyKeyPrefix(key string) string {
	if len(key) < 16 {
		return key
	}
	return key[:16] + "..."
}

// ValidateIdempotencyKey checks if the key format is valid (64-char hex).
func ValidateIdempotencyKey(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("invalid idempotency key length: expected 64, got %d", len(key))
	}
	_, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid idempotency key format: %w", err)
	}
	return nil
}

// DeriveAttemptID creates a unique attempt ID from envelope and attempt number.
// This ensures deterministic attempt IDs that can be verified.
func DeriveAttemptID(envelopeID string, attemptNumber int) string {
	return fmt.Sprintf("%s-attempt-%d", envelopeID, attemptNumber)
}

// HashForProvider creates a shorter provider-compatible idempotency key.
// Some providers have length limits on idempotency keys.
// This returns a 32-character key derived from the full key.
func HashForProvider(fullKey string) string {
	if len(fullKey) >= 32 {
		return fullKey[:32]
	}
	return fullKey
}
