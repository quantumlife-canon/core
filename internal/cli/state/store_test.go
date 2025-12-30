package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateFilePermissions(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create and save state
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
		filePath:      statePath,
	}

	s.SetTokenHandle("circle-1", "google", "token-123")
	if err := s.Save(); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("Failed to stat state file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", mode)
	}
}

func TestStateSchemaVersioning(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Save with current version
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
		filePath:      statePath,
	}
	s.SetTokenHandle("circle-1", "google", "token-123")
	if err := s.Save(); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Load and verify version
	loaded, err := LoadFrom(statePath)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	if loaded.SchemaVersion != SchemaVersion {
		t.Errorf("Expected schema version %d, got %d", SchemaVersion, loaded.SchemaVersion)
	}
}

func TestStateSetGetTokenHandle(t *testing.T) {
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
	}

	// Initially no handle
	_, ok := s.GetTokenHandle("circle-1", "google")
	if ok {
		t.Error("Expected no handle initially")
	}

	// Set handle
	s.SetTokenHandle("circle-1", "google", "token-123")

	// Get handle
	handleID, ok := s.GetTokenHandle("circle-1", "google")
	if !ok {
		t.Error("Expected handle to exist")
	}
	if handleID != "token-123" {
		t.Errorf("Expected handle ID 'token-123', got '%s'", handleID)
	}

	// Different provider should not exist
	_, ok = s.GetTokenHandle("circle-1", "microsoft")
	if ok {
		t.Error("Expected no handle for microsoft")
	}

	// Different circle should not exist
	_, ok = s.GetTokenHandle("circle-2", "google")
	if ok {
		t.Error("Expected no handle for circle-2")
	}
}

func TestStateRemoveTokenHandle(t *testing.T) {
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
	}

	// Set and then remove
	s.SetTokenHandle("circle-1", "google", "token-123")
	removed := s.RemoveTokenHandle("circle-1", "google")
	if !removed {
		t.Error("Expected remove to return true")
	}

	// Verify removed
	_, ok := s.GetTokenHandle("circle-1", "google")
	if ok {
		t.Error("Expected handle to be removed")
	}

	// Remove non-existent
	removed = s.RemoveTokenHandle("circle-1", "google")
	if removed {
		t.Error("Expected remove of non-existent to return false")
	}
}

func TestStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create and save
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
		filePath:      statePath,
	}
	s.SetTokenHandle("circle-1", "google", "token-123")
	s.SetTokenHandle("circle-1", "microsoft", "token-456")
	s.SetTokenHandle("circle-2", "google", "token-789")

	if err := s.Save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Load and verify
	loaded, err := LoadFrom(statePath)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	tests := []struct {
		circleID string
		provider string
		expected string
	}{
		{"circle-1", "google", "token-123"},
		{"circle-1", "microsoft", "token-456"},
		{"circle-2", "google", "token-789"},
	}

	for _, tt := range tests {
		handleID, ok := loaded.GetTokenHandle(tt.circleID, tt.provider)
		if !ok {
			t.Errorf("Expected handle for %s/%s to exist", tt.circleID, tt.provider)
			continue
		}
		if handleID != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, handleID)
		}
	}
}

func TestStateEmptyFileLoad(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "nonexistent.json")

	// Load non-existent file should return empty state
	s, err := LoadFrom(statePath)
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if s == nil {
		t.Fatal("Expected non-nil state")
	}

	if len(s.Circles) != 0 {
		t.Error("Expected empty circles map")
	}
}

func TestStateInsecurePermissionsRejected(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "insecure.json")

	// Create file with insecure permissions
	if err := os.WriteFile(statePath, []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Attempt to load should fail
	_, err := LoadFrom(statePath)
	if err == nil {
		t.Error("Expected error for insecure permissions")
	}
}

func TestListCirclesAndProviders(t *testing.T) {
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
	}

	s.SetTokenHandle("circle-1", "google", "token-1")
	s.SetTokenHandle("circle-1", "microsoft", "token-2")
	s.SetTokenHandle("circle-2", "google", "token-3")

	circles := s.ListCircles()
	if len(circles) != 2 {
		t.Errorf("Expected 2 circles, got %d", len(circles))
	}

	providers := s.ListProviders("circle-1")
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers for circle-1, got %d", len(providers))
	}

	providers = s.ListProviders("circle-2")
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider for circle-2, got %d", len(providers))
	}

	providers = s.ListProviders("nonexistent")
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers for nonexistent, got %d", len(providers))
	}
}
