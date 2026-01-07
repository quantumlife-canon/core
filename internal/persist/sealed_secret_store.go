// Package persist provides the sealed secret store for Phase 35b.
//
// This store is a FORMALLY DOCUMENTED EXCEPTION to hash-only storage.
// It stores AES-GCM encrypted APNs device tokens.
//
// CRITICAL INVARIANTS:
//   - Indexed ONLY by token_hash (SHA256 of raw token).
//   - No retrieval by circle, user, or device metadata.
//   - File permissions: 0600 (owner read/write only).
//   - Encrypted blob never leaves this store (except to apns.go).
//   - No logging of raw tokens or encrypted blobs.
//   - No goroutines. No time.Now().
//
// Reference: docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md
package persist

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SealedSecretStore stores AES-GCM encrypted APNs device tokens.
// CRITICAL: This is the ONLY place encrypted tokens may be stored.
type SealedSecretStore struct {
	mu sync.RWMutex

	// encryptionKey is the 32-byte AES-256 key.
	encryptionKey []byte

	// dataDir is the directory for sealed files.
	dataDir string

	// gcm is the AES-GCM cipher for encryption/decryption.
	gcm cipher.AEAD
}

// SealedSecretStoreConfig configures the sealed secret store.
type SealedSecretStoreConfig struct {
	// EncryptionKeyBase64 is the base64-encoded 32-byte encryption key.
	// Should come from QL_SEALED_SECRET_KEY environment variable.
	EncryptionKeyBase64 string

	// DataDir is the directory for sealed files.
	// Defaults to $QL_DATA_DIR/sealed/ or ./data/sealed/
	DataDir string
}

// DefaultSealedSecretStoreConfig returns default configuration.
func DefaultSealedSecretStoreConfig() SealedSecretStoreConfig {
	dataDir := os.Getenv("QL_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	return SealedSecretStoreConfig{
		EncryptionKeyBase64: os.Getenv("QL_SEALED_SECRET_KEY"),
		DataDir:             filepath.Join(dataDir, "sealed"),
	}
}

// NewSealedSecretStore creates a new sealed secret store.
// Returns error if encryption key is missing or invalid.
func NewSealedSecretStore(cfg SealedSecretStoreConfig) (*SealedSecretStore, error) {
	if cfg.EncryptionKeyBase64 == "" {
		return nil, fmt.Errorf("QL_SEALED_SECRET_KEY is required")
	}

	// Decode base64 key
	key, err := base64.StdEncoding.DecodeString(cfg.EncryptionKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encryption key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Ensure data directory exists with proper permissions
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	return &SealedSecretStore{
		encryptionKey: key,
		dataDir:       cfg.DataDir,
		gcm:           gcm,
	}, nil
}

// filePath returns the path for a token hash.
func (s *SealedSecretStore) filePath(tokenHash string) string {
	// Sanitize token hash to prevent path traversal
	safe := sanitizeTokenHash(tokenHash)
	return filepath.Join(s.dataDir, safe+".sealed")
}

// sanitizeTokenHash ensures the token hash is safe for file naming.
func sanitizeTokenHash(tokenHash string) string {
	// Only allow hex characters
	var sb strings.Builder
	for _, c := range tokenHash {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// Encrypt encrypts plaintext using AES-GCM.
// Returns: nonce || ciphertext (includes GCM tag).
func (s *SealedSecretStore) Encrypt(plaintext []byte) ([]byte, error) {
	// Generate random nonce
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt (appends ciphertext to nonce)
	ciphertext := s.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-GCM.
// Expects: nonce || ciphertext (includes GCM tag).
func (s *SealedSecretStore) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < s.gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce
	nonce := ciphertext[:s.gcm.NonceSize()]
	ciphertext = ciphertext[s.gcm.NonceSize():]

	// Decrypt
	plaintext, err := s.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// StoreEncrypted encrypts and stores a secret indexed by token hash.
// CRITICAL: The plaintext is the raw device token. It is encrypted before storage.
func (s *SealedSecretStore) StoreEncrypted(tokenHash string, plaintext []byte) error {
	if tokenHash == "" {
		return fmt.Errorf("token_hash is required")
	}
	if len(plaintext) == 0 {
		return fmt.Errorf("plaintext is required")
	}

	// Encrypt
	encrypted, err := s.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Write to file with restricted permissions
	path := s.filePath(tokenHash)
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		return fmt.Errorf("write sealed file: %w", err)
	}

	return nil
}

// LoadEncrypted loads and decrypts a secret by token hash.
// CRITICAL: Returns the raw device token. Caller MUST NOT log or persist it.
func (s *SealedSecretStore) LoadEncrypted(tokenHash string) ([]byte, error) {
	if tokenHash == "" {
		return nil, fmt.Errorf("token_hash is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Read file
	path := s.filePath(tokenHash)
	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("sealed secret not found: %s", tokenHash)
		}
		return nil, fmt.Errorf("read sealed file: %w", err)
	}

	// Decrypt
	plaintext, err := s.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// DeleteEncrypted removes a sealed secret by token hash.
func (s *SealedSecretStore) DeleteEncrypted(tokenHash string) error {
	if tokenHash == "" {
		return fmt.Errorf("token_hash is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(tokenHash)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("delete sealed file: %w", err)
	}

	return nil
}

// Exists checks if a sealed secret exists for the token hash.
func (s *SealedSecretStore) Exists(tokenHash string) bool {
	if tokenHash == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.filePath(tokenHash)
	_, err := os.Stat(path)
	return err == nil
}

// Count returns the number of sealed secrets stored.
// Used for testing/debugging only.
func (s *SealedSecretStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sealed") {
			count++
		}
	}
	return count, nil
}

// GenerateKey generates a new 32-byte encryption key.
// Returns base64-encoded key suitable for QL_SEALED_SECRET_KEY.
// This is a helper for key generation, not used at runtime.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
