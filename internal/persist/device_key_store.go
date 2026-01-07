// Package persist provides persistence for device keys.
//
// Phase 30A: Device Key Store
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only (not needed for key storage)
// - No goroutines
// - Private key stored with 0600 permissions
// - NEVER log or print private keys
// - NEVER include private key in any error messages
//
// Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
package persist

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"quantumlife/pkg/domain/deviceidentity"
)

// DeviceKeyStore manages Ed25519 device keypairs.
// Private key is stored on disk with restricted permissions.
// Public key and fingerprint are derived on demand.
type DeviceKeyStore struct {
	mu         sync.RWMutex
	keyPath    string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	loaded     bool
}

// NewDeviceKeyStore creates a new device key store.
// keyPath is the path to the private key file.
func NewDeviceKeyStore(keyPath string) *DeviceKeyStore {
	return &DeviceKeyStore{
		keyPath: keyPath,
	}
}

// EnsureKeypair ensures a keypair exists, creating one if necessary.
// Creates with 0600 permissions. Reads existing if present.
// Returns the public key and fingerprint.
//
// CRITICAL: Never logs or returns the private key.
func (s *DeviceKeyStore) EnsureKeypair() (deviceidentity.DevicePublicKey, deviceidentity.Fingerprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If already loaded, return cached values
	if s.loaded {
		pubKeyHex := deviceidentity.DevicePublicKey(hex.EncodeToString(s.publicKey))
		return pubKeyHex, pubKeyHex.Fingerprint(), nil
	}

	// Check if key file exists
	if _, err := os.Stat(s.keyPath); os.IsNotExist(err) {
		// Create new keypair
		if err := s.createKeypair(); err != nil {
			return "", "", fmt.Errorf("failed to create keypair: %w", err)
		}
	} else if err != nil {
		return "", "", fmt.Errorf("failed to check key file: %w", err)
	} else {
		// Load existing keypair
		if err := s.loadKeypair(); err != nil {
			return "", "", fmt.Errorf("failed to load keypair: %w", err)
		}
	}

	s.loaded = true
	pubKeyHex := deviceidentity.DevicePublicKey(hex.EncodeToString(s.publicKey))
	return pubKeyHex, pubKeyHex.Fingerprint(), nil
}

// createKeypair generates a new Ed25519 keypair and saves it.
func (s *DeviceKeyStore) createKeypair() error {
	// Ensure directory exists
	dir := filepath.Dir(s.keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Generate keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Write private key with restricted permissions
	privHex := hex.EncodeToString(priv)
	if err := os.WriteFile(s.keyPath, []byte(privHex), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	s.privateKey = priv
	s.publicKey = pub
	return nil
}

// loadKeypair loads an existing keypair from disk.
func (s *DeviceKeyStore) loadKeypair() error {
	// Read private key
	data, err := os.ReadFile(s.keyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	// Decode hex
	privBytes, err := hex.DecodeString(string(data))
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(privBytes) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key length: got %d, want %d", len(privBytes), ed25519.PrivateKeySize)
	}

	s.privateKey = ed25519.PrivateKey(privBytes)
	s.publicKey = s.privateKey.Public().(ed25519.PublicKey)
	return nil
}

// GetPublicKey returns the public key. Must call EnsureKeypair first.
func (s *DeviceKeyStore) GetPublicKey() (deviceidentity.DevicePublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.loaded {
		return "", errors.New("keypair not loaded - call EnsureKeypair first")
	}

	return deviceidentity.DevicePublicKey(hex.EncodeToString(s.publicKey)), nil
}

// GetFingerprint returns the fingerprint. Must call EnsureKeypair first.
func (s *DeviceKeyStore) GetFingerprint() (deviceidentity.Fingerprint, error) {
	pubKey, err := s.GetPublicKey()
	if err != nil {
		return "", err
	}
	return pubKey.Fingerprint(), nil
}

// Sign signs a message with the private key.
// Returns the signature as hex.
//
// CRITICAL: Never logs the message content or signature for sensitive operations.
func (s *DeviceKeyStore) Sign(message []byte) (deviceidentity.Signature, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.loaded {
		return "", errors.New("keypair not loaded - call EnsureKeypair first")
	}

	sig := ed25519.Sign(s.privateKey, message)
	return deviceidentity.Signature(hex.EncodeToString(sig)), nil
}

// Verify verifies a signature against a message and public key.
// This is a static method that doesn't require the store's private key.
func Verify(message []byte, signature deviceidentity.Signature, publicKey deviceidentity.DevicePublicKey) (bool, error) {
	// Decode signature
	sigBytes, err := signature.ToBytes()
	if err != nil {
		return false, fmt.Errorf("invalid signature: %w", err)
	}

	// Decode public key
	pubBytes, err := publicKey.ToBytes()
	if err != nil {
		return false, fmt.Errorf("invalid public key: %w", err)
	}

	// Verify
	return ed25519.Verify(pubBytes, message, sigBytes), nil
}

// SignRequest signs a signed request using the device's private key.
// Returns the request with the signature field populated.
func (s *DeviceKeyStore) SignRequest(req *deviceidentity.SignedRequest) error {
	message := []byte(req.CanonicalString())
	sig, err := s.Sign(message)
	if err != nil {
		return err
	}
	req.Signature = sig
	return nil
}

// VerifyRequest verifies a signed request.
func VerifyRequest(req *deviceidentity.SignedRequest) (bool, error) {
	if err := req.Validate(); err != nil {
		return false, err
	}

	message := []byte(req.CanonicalString())
	return Verify(message, req.Signature, req.PublicKey)
}

// IsLoaded returns whether a keypair has been loaded.
func (s *DeviceKeyStore) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

// KeyPath returns the path to the key file.
func (s *DeviceKeyStore) KeyPath() string {
	return s.keyPath
}
