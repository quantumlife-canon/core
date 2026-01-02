package persist

import (
	"sync"

	"quantumlife/pkg/domain/connection"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// ConnectionStore implements persistent storage for connection intents.
//
// Phase 18.6: First Connect (Consent-first Onboarding)
//
// CRITICAL: No goroutines. No time.Now(). stdlib-only.
// CRITICAL: Append-only intent store backed by storelog.
// CRITICAL: On startup, replay intents from storelog → in-memory list.
//
// Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
type ConnectionStore struct {
	mu      sync.RWMutex
	log     storelog.AppendOnlyLog
	intents connection.IntentList
	byHash  map[string]*connection.ConnectionIntent

	// configPresent tracks which kinds have real configuration.
	// In Phase 18.6, this is always empty (all real → NeedsConfig).
	configPresent map[connection.ConnectionKind]bool
}

// NewConnectionStore creates a new file-backed connection store.
func NewConnectionStore(log storelog.AppendOnlyLog) (*ConnectionStore, error) {
	store := &ConnectionStore{
		log:           log,
		intents:       make(connection.IntentList, 0),
		byHash:        make(map[string]*connection.ConnectionIntent),
		configPresent: make(map[connection.ConnectionKind]bool),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads connection intents from the log.
func (s *ConnectionStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeConnectionIntent)
	if err != nil {
		return err
	}

	for _, record := range records {
		intent, err := connection.ParseCanonicalIntent(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		s.intents = append(s.intents, intent)
		s.byHash[intent.ID] = intent
	}

	// Sort intents deterministically
	s.intents.Sort()
	return nil
}

// AppendIntent adds a new connection intent.
func (s *ConnectionStore) AppendIntent(intent *connection.ConnectionIntent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create log record
	logRecord := storelog.NewRecord(
		storelog.RecordTypeConnectionIntent,
		intent.At,
		identity.EntityID(""), // No circle ID for connection intents
		intent.CanonicalString(),
	)

	// Append to log
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	// Add to in-memory list
	s.intents = append(s.intents, intent)
	s.byHash[intent.ID] = intent

	// Re-sort
	s.intents.Sort()
	return nil
}

// ListIntents returns all intents sorted deterministically.
func (s *ConnectionStore) ListIntents() connection.IntentList {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(connection.IntentList, len(s.intents))
	copy(result, s.intents)
	return result
}

// GetIntent returns an intent by hash.
func (s *ConnectionStore) GetIntent(hash string) *connection.ConnectionIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byHash[hash]
}

// State returns the computed connection state from all intents.
func (s *ConnectionStore) State() *connection.ConnectionStateSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return connection.ComputeState(s.intents, s.configPresent)
}

// SetConfigPresent marks a kind as having real configuration.
// In Phase 18.6, this is typically not called (all real → NeedsConfig).
func (s *ConnectionStore) SetConfigPresent(kind connection.ConnectionKind, present bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configPresent[kind] = present
}

// StateHash returns the hash of the current computed state.
func (s *ConnectionStore) StateHash() string {
	return s.State().Hash
}

// IntentCount returns the number of recorded intents.
func (s *ConnectionStore) IntentCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.intents)
}

// Flush ensures all records are persisted.
func (s *ConnectionStore) Flush() error {
	return s.log.Flush()
}

// InMemoryConnectionStore is a simple in-memory connection store for testing.
type InMemoryConnectionStore struct {
	mu            sync.RWMutex
	intents       connection.IntentList
	byHash        map[string]*connection.ConnectionIntent
	configPresent map[connection.ConnectionKind]bool
}

// NewInMemoryConnectionStore creates a new in-memory connection store.
func NewInMemoryConnectionStore() *InMemoryConnectionStore {
	return &InMemoryConnectionStore{
		intents:       make(connection.IntentList, 0),
		byHash:        make(map[string]*connection.ConnectionIntent),
		configPresent: make(map[connection.ConnectionKind]bool),
	}
}

// AppendIntent adds a new connection intent.
func (s *InMemoryConnectionStore) AppendIntent(intent *connection.ConnectionIntent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.intents = append(s.intents, intent)
	s.byHash[intent.ID] = intent
	s.intents.Sort()
	return nil
}

// ListIntents returns all intents sorted deterministically.
func (s *InMemoryConnectionStore) ListIntents() connection.IntentList {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(connection.IntentList, len(s.intents))
	copy(result, s.intents)
	return result
}

// GetIntent returns an intent by hash.
func (s *InMemoryConnectionStore) GetIntent(hash string) *connection.ConnectionIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byHash[hash]
}

// State returns the computed connection state from all intents.
func (s *InMemoryConnectionStore) State() *connection.ConnectionStateSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return connection.ComputeState(s.intents, s.configPresent)
}

// SetConfigPresent marks a kind as having real configuration.
func (s *InMemoryConnectionStore) SetConfigPresent(kind connection.ConnectionKind, present bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configPresent[kind] = present
}

// StateHash returns the hash of the current computed state.
func (s *InMemoryConnectionStore) StateHash() string {
	return s.State().Hash
}

// IntentCount returns the number of recorded intents.
func (s *InMemoryConnectionStore) IntentCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.intents)
}
