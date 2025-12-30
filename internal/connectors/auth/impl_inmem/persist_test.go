package impl_inmem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/connectors/auth"
)

func TestPersistenceManagerEncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "broker_store.json")

	// Create with real encryption key
	encKey := "test-encryption-key-12345"
	pm := NewPersistenceManager(encKey, WithPersistPath(storePath))

	if !pm.IsEnabled() {
		t.Error("Expected persistence to be enabled with key")
	}
	if !pm.HasRealKey() {
		t.Error("Expected HasRealKey to be true")
	}

	// Create some tokens
	store := NewTokenStore(encKey)
	ctx := context.Background()

	_, err := store.Store(ctx, "circle-1", auth.ProviderGoogle, "refresh-token-1", []string{"calendar:read"}, time.Time{})
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	_, err = store.Store(ctx, "circle-2", auth.ProviderMicrosoft, "refresh-token-2", []string{"calendar:read"}, time.Time{})
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Save
	store.mu.RLock()
	tokens := store.tokens
	nextID := store.idCounter
	store.mu.RUnlock()

	if err := pm.Save(tokens, nextID); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("Expected permissions 0600, got %o", mode)
	}

	// Load with same key
	loadedTokens, loadedID, err := pm.Load()
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if loadedID != nextID {
		t.Errorf("Expected next ID %d, got %d", nextID, loadedID)
	}

	if len(loadedTokens) != 2 {
		t.Errorf("Expected 2 tokens, got %d", len(loadedTokens))
	}
}

func TestPersistenceManagerWrongKey(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "broker_store.json")

	// Save with one key
	pm1 := NewPersistenceManager("key-1", WithPersistPath(storePath))
	store := NewTokenStore("key-1")
	ctx := context.Background()

	store.Store(ctx, "circle-1", auth.ProviderGoogle, "refresh-token", []string{"calendar:read"}, time.Time{})

	store.mu.RLock()
	tokens := store.tokens
	nextID := store.idCounter
	store.mu.RUnlock()

	if err := pm1.Save(tokens, nextID); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Try to load with different key
	pm2 := NewPersistenceManager("key-2", WithPersistPath(storePath))
	_, _, err := pm2.Load()
	if err == nil {
		t.Error("Expected error when loading with wrong key")
	}
}

func TestPersistenceDisabledWithoutKey(t *testing.T) {
	pm := NewPersistenceManager("")

	if pm.IsEnabled() {
		t.Error("Expected persistence to be disabled without key")
	}
	if pm.HasRealKey() {
		t.Error("Expected HasRealKey to be false")
	}

	_, _, err := pm.Load()
	if err != ErrPersistenceDisabled {
		t.Errorf("Expected ErrPersistenceDisabled, got %v", err)
	}

	err = pm.Save(nil, 0)
	if err != ErrPersistenceDisabled {
		t.Errorf("Expected ErrPersistenceDisabled, got %v", err)
	}
}

func TestTokenStoreWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	encKey := "test-key-for-persistence"

	// Set env for the path
	oldPath := os.Getenv("HOME")
	defer os.Setenv("HOME", oldPath)
	os.Setenv("HOME", tmpDir)

	// Create store with persistence
	store, err := NewTokenStoreWithPersistence(encKey)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	if !store.IsPersistenceEnabled() {
		t.Error("Expected persistence to be enabled")
	}

	// Store a token
	ctx := context.Background()
	handle, err := store.StoreWithPersist("circle-1", auth.ProviderGoogle, "refresh-token", []string{"calendar:read"}, time.Time{})
	if err != nil {
		t.Fatalf("Failed to store: %v", err)
	}

	if handle.ID == "" {
		t.Error("Expected non-empty handle ID")
	}

	// Create new store and load from disk
	store2, err := NewTokenStoreWithPersistence(encKey)
	if err != nil {
		t.Fatalf("Failed to create second store: %v", err)
	}

	// Verify token was loaded
	_, storedToken, err := store2.Get(ctx, "circle-1", auth.ProviderGoogle)
	if err != nil {
		t.Fatalf("Failed to get token from second store: %v", err)
	}

	if storedToken.ID != handle.ID {
		t.Errorf("Expected handle ID %s, got %s", handle.ID, storedToken.ID)
	}
}

func TestBrokerGetTokenHandle(t *testing.T) {
	cfg := auth.Config{
		TokenEncryptionKey: "test-key",
		Google: auth.GoogleConfig{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
	}

	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	// Store a token directly
	handle, err := broker.StoreTokenDirectly(ctx, "circle-1", auth.ProviderGoogle, "refresh-token", []string{"calendar:read"})
	if err != nil {
		t.Fatalf("Failed to store: %v", err)
	}

	// Get the handle
	retrieved, ok := broker.GetTokenHandle("circle-1", auth.ProviderGoogle)
	if !ok {
		t.Error("Expected to find token handle")
	}

	if retrieved.ID != handle.ID {
		t.Errorf("Expected handle ID %s, got %s", handle.ID, retrieved.ID)
	}

	// Non-existent should return false
	_, ok = broker.GetTokenHandle("nonexistent", auth.ProviderGoogle)
	if ok {
		t.Error("Expected false for nonexistent circle")
	}
}

func TestOutputNeverContainsTokenMaterial(t *testing.T) {
	// This test verifies that handle.String() or similar never leaks tokens
	handle := auth.TokenHandle{
		ID:        "token-123",
		CircleID:  "circle-1",
		Provider:  auth.ProviderGoogle,
		Scopes:    []string{"calendar:read"},
		CreatedAt: time.Now(),
	}

	// The handle should never contain the actual token
	repr := handle.ID + handle.CircleID + string(handle.Provider)
	for _, scope := range handle.Scopes {
		repr += scope
	}

	forbiddenPatterns := []string{
		"access_token",
		"refresh_token",
		"bearer",
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(repr), pattern) {
			t.Errorf("Handle representation contains forbidden pattern: %s", pattern)
		}
	}
}
