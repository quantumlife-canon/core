// Package impl_inmem provides in-memory cryptographic implementations.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation uses HMAC-SHA256 as a PLACEHOLDER.
// Production MUST use proper asymmetric cryptography with algorithm agility.
//
// Reference: docs/TECHNOLOGY_SELECTION_V1.md §6 Identity & Crypto Posture
package impl_inmem

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/crypto"
)

// PlaceholderAlgorithm is used to clearly mark this as non-production crypto.
const PlaceholderAlgorithm = "HMAC-SHA256-PLACEHOLDER"

// Key represents an in-memory cryptographic key.
// PLACEHOLDER: Uses symmetric HMAC for demo. Production needs asymmetric keys.
type Key struct {
	ID        string
	Secret    []byte // PLACEHOLDER: Would be private/public key pair in production
	Algorithm string
	Version   int
	CreatedAt time.Time
	ExpiresAt time.Time
	IsActive  bool
}

// Signer implements crypto.Signer using HMAC-SHA256 (placeholder).
type Signer struct {
	key *Key
}

// NewSigner creates a new signer for the given key.
func NewSigner(key *Key) *Signer {
	return &Signer{key: key}
}

// Sign creates an HMAC-SHA256 signature (placeholder implementation).
func (s *Signer) Sign(ctx context.Context, data []byte) ([]byte, error) {
	if s.key == nil {
		return nil, fmt.Errorf("no key available")
	}
	if !s.key.IsActive {
		return nil, fmt.Errorf("key %s is not active", s.key.ID)
	}
	if !s.key.ExpiresAt.IsZero() && time.Now().After(s.key.ExpiresAt) {
		return nil, fmt.Errorf("key %s has expired", s.key.ID)
	}

	mac := hmac.New(sha256.New, s.key.Secret)
	mac.Write(data)
	return mac.Sum(nil), nil
}

// KeyID returns the key identifier.
func (s *Signer) KeyID() string {
	if s.key == nil {
		return ""
	}
	return s.key.ID
}

// Algorithm returns the algorithm identifier.
func (s *Signer) Algorithm() string {
	return PlaceholderAlgorithm
}

// Verifier implements crypto.Verifier using HMAC-SHA256 (placeholder).
type Verifier struct {
	key *Key
}

// NewVerifier creates a new verifier for the given key.
func NewVerifier(key *Key) *Verifier {
	return &Verifier{key: key}
}

// Verify checks an HMAC-SHA256 signature (placeholder implementation).
func (v *Verifier) Verify(ctx context.Context, data []byte, signature []byte) error {
	if v.key == nil {
		return fmt.Errorf("no key available")
	}

	mac := hmac.New(sha256.New, v.key.Secret)
	mac.Write(data)
	expected := mac.Sum(nil)

	if !hmac.Equal(signature, expected) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// KeyID returns the key identifier.
func (v *Verifier) KeyID() string {
	if v.key == nil {
		return ""
	}
	return v.key.ID
}

// Algorithm returns the algorithm identifier.
func (v *Verifier) Algorithm() string {
	return PlaceholderAlgorithm
}

// KeyManager implements crypto.KeyManager with in-memory storage.
type KeyManager struct {
	mu      sync.RWMutex
	keys    map[string]*Key
	counter int
}

// NewKeyManager creates a new in-memory key manager.
func NewKeyManager() *KeyManager {
	return &KeyManager{
		keys: make(map[string]*Key),
	}
}

// CreateKey creates a new key with the given ID.
// Uses deterministic secret generation for testing (NOT SECURE).
func (km *KeyManager) CreateKey(ctx context.Context, keyID string, expiresIn time.Duration) (*Key, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if _, exists := km.keys[keyID]; exists {
		return nil, fmt.Errorf("key %s already exists", keyID)
	}

	km.counter++

	// PLACEHOLDER: Deterministic "secret" for testing. NOT SECURE.
	secret := sha256.Sum256([]byte(fmt.Sprintf("placeholder-secret-%s-%d", keyID, km.counter)))

	now := time.Now()
	var expiresAt time.Time
	if expiresIn > 0 {
		expiresAt = now.Add(expiresIn)
	}

	key := &Key{
		ID:        keyID,
		Secret:    secret[:],
		Algorithm: PlaceholderAlgorithm,
		Version:   1,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		IsActive:  true,
	}

	km.keys[keyID] = key
	return key, nil
}

// GetSigner returns a signer for the specified key.
func (km *KeyManager) GetSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, exists := km.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	return NewSigner(key), nil
}

// GetVerifier returns a verifier for the specified key.
func (km *KeyManager) GetVerifier(ctx context.Context, keyID string) (crypto.Verifier, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, exists := km.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	return NewVerifier(key), nil
}

// RotateKey creates a new version of the specified key.
func (km *KeyManager) RotateKey(ctx context.Context, keyID string) (crypto.KeyMetadata, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	key, exists := km.keys[keyID]
	if !exists {
		return crypto.KeyMetadata{}, fmt.Errorf("key not found: %s", keyID)
	}

	km.counter++

	// Generate new secret
	newSecret := sha256.Sum256([]byte(fmt.Sprintf("placeholder-secret-%s-%d-v%d", keyID, km.counter, key.Version+1)))

	key.Secret = newSecret[:]
	key.Version++
	key.CreatedAt = time.Now()

	return crypto.KeyMetadata{
		KeyID:            key.ID,
		Algorithm:        key.Algorithm,
		AlgorithmVersion: key.Version,
		CreatedAt:        key.CreatedAt,
		ExpiresAt:        key.ExpiresAt,
		IsActive:         key.IsActive,
		PQExtension:      false,
	}, nil
}

// ListKeys returns metadata for all keys.
func (km *KeyManager) ListKeys(ctx context.Context) ([]crypto.KeyMetadata, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	var result []crypto.KeyMetadata
	for _, key := range km.keys {
		result = append(result, crypto.KeyMetadata{
			KeyID:            key.ID,
			Algorithm:        key.Algorithm,
			AlgorithmVersion: key.Version,
			CreatedAt:        key.CreatedAt,
			ExpiresAt:        key.ExpiresAt,
			IsActive:         key.IsActive,
			PQExtension:      false,
		})
	}
	return result, nil
}

// GetKey returns a key by ID (for testing).
func (km *KeyManager) GetKey(keyID string) (*Key, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	key, exists := km.keys[keyID]
	return key, exists
}

// RedactedSignature returns a redacted version of a signature for display.
func RedactedSignature(sig []byte) string {
	if len(sig) == 0 {
		return "<empty>"
	}
	full := hex.EncodeToString(sig)
	if len(full) > 16 {
		return full[:8] + "..." + full[len(full)-8:]
	}
	return full
}

// Verify interface compliance at compile time.
var (
	_ crypto.Signer     = (*Signer)(nil)
	_ crypto.Verifier   = (*Verifier)(nil)
	_ crypto.KeyManager = (*KeyManager)(nil)
)
