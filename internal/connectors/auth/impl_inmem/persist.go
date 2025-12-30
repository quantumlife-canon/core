// Package impl_inmem provides in-memory implementation of the token broker.
// This file implements optional file persistence for the token store.
//
// CRITICAL: This is for CLI convenience only. Production requires proper
// persistent storage (Postgres + Key Vault).
//
// The broker store is encrypted on disk using TOKEN_ENC_KEY.
// If TOKEN_ENC_KEY is not set, persistence is disabled with a warning.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package impl_inmem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"quantumlife/internal/connectors/auth"
)

// Default persistence file location.
const (
	DefaultPersistDirName  = ".quantumlife"
	DefaultPersistFileName = "broker_store.json"
)

// Persistence errors.
var (
	ErrPersistenceDisabled = errors.New("persistence disabled: TOKEN_ENC_KEY not set")
	ErrPersistCorrupted    = errors.New("persisted store is corrupted or key mismatch")
)

// persistedStore is the on-disk format for the token store.
type persistedStore struct {
	// SchemaVersion for forward compatibility.
	SchemaVersion int `json:"schema_version"`

	// UpdatedAt is the last modification timestamp.
	UpdatedAt time.Time `json:"updated_at"`

	// EncryptedTokens maps storage key to encrypted token data.
	// Key format: "circleID:provider"
	// Value is base64-encoded encrypted JSON of StoredToken.
	EncryptedTokens map[string]string `json:"encrypted_tokens"`

	// NextID is the next token ID counter.
	NextID int `json:"next_id"`
}

// PersistenceManager handles loading and saving the token store.
type PersistenceManager struct {
	encryptor  *TokenEncryptor
	filePath   string
	enabled    bool
	hasRealKey bool
}

// PersistenceOption configures the persistence manager.
type PersistenceOption func(*PersistenceManager)

// WithPersistPath sets a custom persistence file path.
func WithPersistPath(path string) PersistenceOption {
	return func(pm *PersistenceManager) {
		pm.filePath = path
	}
}

// NewPersistenceManager creates a new persistence manager.
// If encryptionKey is empty, persistence is disabled.
func NewPersistenceManager(encryptionKey string, opts ...PersistenceOption) *PersistenceManager {
	pm := &PersistenceManager{
		enabled:    encryptionKey != "",
		hasRealKey: encryptionKey != "",
	}

	// Set default path
	home, err := os.UserHomeDir()
	if err == nil {
		pm.filePath = filepath.Join(home, DefaultPersistDirName, DefaultPersistFileName)
	}

	// Create encryptor if key is provided
	if encryptionKey != "" {
		pm.encryptor = NewTokenEncryptor(encryptionKey)
	}

	// Apply options
	for _, opt := range opts {
		opt(pm)
	}

	return pm
}

// IsEnabled returns true if persistence is enabled.
func (pm *PersistenceManager) IsEnabled() bool {
	return pm.enabled
}

// HasRealKey returns true if a real encryption key was provided.
func (pm *PersistenceManager) HasRealKey() bool {
	return pm.hasRealKey
}

// GetFilePath returns the persistence file path.
func (pm *PersistenceManager) GetFilePath() string {
	return pm.filePath
}

// Load loads the token store from disk.
// Returns an empty store if the file doesn't exist.
func (pm *PersistenceManager) Load() (map[string]*StoredToken, int, error) {
	if !pm.enabled {
		return nil, 0, ErrPersistenceDisabled
	}

	// Check if file exists
	info, err := os.Stat(pm.filePath)
	if os.IsNotExist(err) {
		// Return empty store
		return make(map[string]*StoredToken), 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	// Verify file permissions (must be 0600)
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return nil, 0, fmt.Errorf("broker store has insecure permissions: %o", mode)
	}

	// Read file
	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		return nil, 0, err
	}

	// Parse persisted store
	var ps persistedStore
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, 0, fmt.Errorf("failed to parse broker store: %w", err)
	}

	// Decrypt and deserialize tokens
	tokens := make(map[string]*StoredToken)
	for key, encryptedData := range ps.EncryptedTokens {
		tokenJSON, err := pm.encryptor.DecryptString(encryptedData)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: failed to decrypt token %s", ErrPersistCorrupted, key)
		}

		var token StoredToken
		if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
			return nil, 0, fmt.Errorf("%w: failed to parse token %s", ErrPersistCorrupted, key)
		}

		tokens[key] = &token
	}

	return tokens, ps.NextID, nil
}

// Save persists the token store to disk.
func (pm *PersistenceManager) Save(tokens map[string]*StoredToken, nextID int) error {
	if !pm.enabled {
		return ErrPersistenceDisabled
	}

	// Encrypt and serialize tokens
	encryptedTokens := make(map[string]string)
	for key, token := range tokens {
		tokenJSON, err := json.Marshal(token)
		if err != nil {
			return fmt.Errorf("failed to serialize token %s: %w", key, err)
		}

		encrypted, err := pm.encryptor.EncryptString(string(tokenJSON))
		if err != nil {
			return fmt.Errorf("failed to encrypt token %s: %w", key, err)
		}

		encryptedTokens[key] = encrypted
	}

	ps := persistedStore{
		SchemaVersion:   1,
		UpdatedAt:       time.Now(),
		EncryptedTokens: encryptedTokens,
		NextID:          nextID,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(pm.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create broker store directory: %w", err)
	}

	// Write to temp file for atomic operation
	tmpPath := pm.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	// Rename for atomic write
	if err := os.Rename(tmpPath, pm.filePath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// Delete removes the persistence file.
func (pm *PersistenceManager) Delete() error {
	if err := os.Remove(pm.filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// TokenStoreWithPersistence wraps TokenStore with persistence support.
type TokenStoreWithPersistence struct {
	*TokenStore
	persist *PersistenceManager
}

// NewTokenStoreWithPersistence creates a token store with optional persistence.
func NewTokenStoreWithPersistence(encryptionKey string, opts ...PersistenceOption) (*TokenStoreWithPersistence, error) {
	store := NewTokenStore(encryptionKey)
	persist := NewPersistenceManager(encryptionKey, opts...)

	ts := &TokenStoreWithPersistence{
		TokenStore: store,
		persist:    persist,
	}

	// Load existing tokens if persistence is enabled
	if persist.IsEnabled() {
		tokens, nextID, err := persist.Load()
		if err != nil && !errors.Is(err, ErrPersistenceDisabled) {
			// Log warning but don't fail - start with empty store
			fmt.Fprintf(os.Stderr, "Warning: failed to load broker store: %v\n", err)
		} else if tokens != nil {
			store.mu.Lock()
			store.tokens = tokens
			store.idCounter = nextID
			store.mu.Unlock()
		}
	}

	return ts, nil
}

// StoreWithPersist stores a token and persists to disk.
func (ts *TokenStoreWithPersistence) StoreWithPersist(circleID string, provider auth.ProviderID, refreshToken string, scopes []string, expiresAt time.Time) (auth.TokenHandle, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Encrypt the refresh token
	encryptedToken, err := ts.encryptor.EncryptString(refreshToken)
	if err != nil {
		return auth.TokenHandle{}, fmt.Errorf("failed to encrypt token: %w", err)
	}

	ts.idCounter++
	tokenID := fmt.Sprintf("token-%d", ts.idCounter)
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
	ts.tokens[key] = token

	// Persist if enabled
	if ts.persist.IsEnabled() {
		if err := ts.persist.Save(ts.tokens, ts.idCounter); err != nil {
			// Log warning but don't fail the operation
			fmt.Fprintf(os.Stderr, "Warning: failed to persist broker store: %v\n", err)
		}
	}

	return auth.TokenHandle{
		ID:        tokenID,
		CircleID:  circleID,
		Provider:  provider,
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// IsPersistenceEnabled returns true if persistence is enabled.
func (ts *TokenStoreWithPersistence) IsPersistenceEnabled() bool {
	return ts.persist.IsEnabled()
}

// GetPersistPath returns the persistence file path.
func (ts *TokenStoreWithPersistence) GetPersistPath() string {
	return ts.persist.GetFilePath()
}

// Sync forces a save to disk.
func (ts *TokenStoreWithPersistence) Sync() error {
	if !ts.persist.IsEnabled() {
		return ErrPersistenceDisabled
	}

	ts.mu.RLock()
	tokens := ts.tokens
	nextID := ts.idCounter
	ts.mu.RUnlock()

	return ts.persist.Save(tokens, nextID)
}
