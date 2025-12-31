// Package state provides local CLI state storage for developer convenience.
// This is NOT authoritative state - the TokenBroker and Circle memory layer
// remain the source of truth.
//
// CRITICAL: This store holds only opaque handle IDs, never raw tokens.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SchemaVersion is the current state file schema version.
const SchemaVersion = 1

// Default file location.
const (
	DefaultDirName  = ".quantumlife"
	DefaultFileName = "state.json"
)

// State represents the CLI local state.
type State struct {
	mu sync.RWMutex

	// SchemaVersion for forward compatibility.
	SchemaVersion int `json:"schema_version"`

	// UpdatedAt is the last modification timestamp.
	UpdatedAt time.Time `json:"updated_at"`

	// Circles maps circleID to circle state.
	Circles map[string]*CircleState `json:"circles"`

	// filePath is the path to the state file.
	filePath string
}

// CircleState holds state for a single circle.
type CircleState struct {
	// Providers maps provider ID to provider state.
	Providers map[string]*ProviderState `json:"providers"`
}

// ProviderState holds state for a single provider within a circle.
type ProviderState struct {
	// HandleID is the opaque token handle ID from the TokenBroker.
	HandleID string `json:"handle_id"`

	// LinkedAt is when this provider was linked.
	LinkedAt time.Time `json:"linked_at"`
}

// Errors.
var (
	ErrNoState          = errors.New("no state file found")
	ErrInvalidSchema    = errors.New("invalid or unsupported schema version")
	ErrNoTokenHandle    = errors.New("no token handle found for circle/provider")
	ErrPermissionDenied = errors.New("state file has insecure permissions")
)

// DefaultPath returns the default state file path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DefaultDirName, DefaultFileName), nil
}

// Load loads state from the default location.
func Load() (*State, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom loads state from a specific path.
func LoadFrom(path string) (*State, error) {
	s := &State{
		SchemaVersion: SchemaVersion,
		Circles:       make(map[string]*CircleState),
		filePath:      path,
	}

	// Check if file exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Return empty state, not an error
		return s, nil
	}
	if err != nil {
		return nil, err
	}

	// Verify file permissions (must be 0600)
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return nil, fmt.Errorf("%w: %s has mode %o, expected 0600", ErrPermissionDenied, path, mode)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Validate schema version
	if s.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("%w: file has version %d, this CLI supports up to %d",
			ErrInvalidSchema, s.SchemaVersion, SchemaVersion)
	}

	// Ensure maps are initialized
	if s.Circles == nil {
		s.Circles = make(map[string]*CircleState)
	}

	s.filePath = path
	return s, nil
}

// Save persists the state to disk.
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveLocked()
}

func (s *State) saveLocked() error {
	s.UpdatedAt = time.Now()

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Marshal JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first for atomic operation
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	// Rename for atomic write
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return err
	}

	return nil
}

// SetTokenHandle stores a token handle ID for a circle+provider.
func (s *State) SetTokenHandle(circleID, provider, handleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Circles == nil {
		s.Circles = make(map[string]*CircleState)
	}

	circleState, ok := s.Circles[circleID]
	if !ok {
		circleState = &CircleState{
			Providers: make(map[string]*ProviderState),
		}
		s.Circles[circleID] = circleState
	}

	if circleState.Providers == nil {
		circleState.Providers = make(map[string]*ProviderState)
	}

	circleState.Providers[provider] = &ProviderState{
		HandleID: handleID,
		LinkedAt: time.Now(),
	}
}

// GetTokenHandle retrieves the token handle ID for a circle+provider.
func (s *State) GetTokenHandle(circleID, provider string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	circleState, ok := s.Circles[circleID]
	if !ok {
		return "", false
	}

	providerState, ok := circleState.Providers[provider]
	if !ok {
		return "", false
	}

	return providerState.HandleID, true
}

// RemoveTokenHandle removes a token handle for a circle+provider.
func (s *State) RemoveTokenHandle(circleID, provider string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	circleState, ok := s.Circles[circleID]
	if !ok {
		return false
	}

	if _, ok := circleState.Providers[provider]; !ok {
		return false
	}

	delete(circleState.Providers, provider)

	// Clean up empty circle state
	if len(circleState.Providers) == 0 {
		delete(s.Circles, circleID)
	}

	return true
}

// ListCircles returns all circle IDs with stored state.
func (s *State) ListCircles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	circles := make([]string, 0, len(s.Circles))
	for id := range s.Circles {
		circles = append(circles, id)
	}
	return circles
}

// ListProviders returns all provider IDs for a circle.
func (s *State) ListProviders(circleID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	circleState, ok := s.Circles[circleID]
	if !ok {
		return nil
	}

	providers := make([]string, 0, len(circleState.Providers))
	for id := range circleState.Providers {
		providers = append(providers, id)
	}
	return providers
}

// GetFilePath returns the state file path.
func (s *State) GetFilePath() string {
	return s.filePath
}

// GetAllTokenHandles returns all token handles as a map of "circleID:provider" -> handleID.
func (s *State) GetAllTokenHandles() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for circleID, circleState := range s.Circles {
		for provider, providerState := range circleState.Providers {
			key := circleID + ":" + provider
			result[key] = providerState.HandleID
		}
	}
	return result
}

// Clear removes all state.
func (s *State) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Circles = make(map[string]*CircleState)
}
