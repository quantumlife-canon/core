// Package impl_inmem provides an in-memory implementation of the memory interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation is NOT for production use.
// Production requires persistent storage with proper versioning.
package impl_inmem

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/memory"
	"quantumlife/pkg/primitives"
)

// Store implements the memory Store and LoopMemoryUpdater interfaces.
type Store struct {
	mu        sync.RWMutex
	entries   map[string]memory.MemoryEntry // key: ownerID:key
	versions  map[string][]memory.MemoryEntry
	idCounter int
}

// NewStore creates a new in-memory memory store.
func NewStore() *Store {
	return &Store{
		entries:  make(map[string]memory.MemoryEntry),
		versions: make(map[string][]memory.MemoryEntry),
	}
}

// makeKey creates a composite key for storage.
func makeKey(ownerID, key string) string {
	return ownerID + ":" + key
}

// WriteCircleMemory writes to a circle's memory.
func (s *Store) WriteCircleMemory(ctx context.Context, circleID string, entry memory.MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	compositeKey := makeKey(circleID, entry.Key)

	// Get current version
	currentVersion := 0
	if existing, ok := s.entries[compositeKey]; ok {
		currentVersion = existing.Version
		// Store in version history
		s.versions[compositeKey] = append(s.versions[compositeKey], existing)
	}

	s.idCounter++
	entry.ID = fmt.Sprintf("mem-%d", s.idCounter)
	entry.OwnerID = circleID
	entry.OwnerType = "circle"
	entry.Version = currentVersion + 1
	entry.UpdatedAt = time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = entry.UpdatedAt
	}

	s.entries[compositeKey] = entry
	return nil
}

// ReadCircleMemory reads from a circle's memory.
func (s *Store) ReadCircleMemory(ctx context.Context, circleID string, key string) (*memory.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compositeKey := makeKey(circleID, key)
	if entry, ok := s.entries[compositeKey]; ok {
		return &entry, nil
	}
	return nil, fmt.Errorf("memory entry not found: %s/%s", circleID, key)
}

// ListCircleMemory lists entries in a circle's memory.
func (s *Store) ListCircleMemory(ctx context.Context, circleID string, filter memory.Filter) ([]memory.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []memory.MemoryEntry
	prefix := circleID + ":"
	if filter.Prefix != "" {
		prefix = circleID + ":" + filter.Prefix
	}

	for key, entry := range s.entries {
		if strings.HasPrefix(key, prefix) {
			if s.matchesFilter(entry, filter) {
				results = append(results, entry)
			}
		}
	}

	// Apply offset and limit
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// DeleteCircleMemory deletes from a circle's memory.
func (s *Store) DeleteCircleMemory(ctx context.Context, circleID string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	compositeKey := makeKey(circleID, key)
	if _, ok := s.entries[compositeKey]; !ok {
		return fmt.Errorf("memory entry not found: %s/%s", circleID, key)
	}
	delete(s.entries, compositeKey)
	return nil
}

// WriteIntersectionMemory writes to an intersection's shared memory.
func (s *Store) WriteIntersectionMemory(ctx context.Context, intersectionID string, entry memory.MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	compositeKey := makeKey(intersectionID, entry.Key)

	// Get current version
	currentVersion := 0
	if existing, ok := s.entries[compositeKey]; ok {
		currentVersion = existing.Version
		s.versions[compositeKey] = append(s.versions[compositeKey], existing)
	}

	s.idCounter++
	entry.ID = fmt.Sprintf("mem-%d", s.idCounter)
	entry.OwnerID = intersectionID
	entry.OwnerType = "intersection"
	entry.Version = currentVersion + 1
	entry.UpdatedAt = time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = entry.UpdatedAt
	}

	s.entries[compositeKey] = entry
	return nil
}

// ReadIntersectionMemory reads from an intersection's shared memory.
func (s *Store) ReadIntersectionMemory(ctx context.Context, intersectionID string, key string) (*memory.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compositeKey := makeKey(intersectionID, key)
	if entry, ok := s.entries[compositeKey]; ok {
		return &entry, nil
	}
	return nil, fmt.Errorf("memory entry not found: %s/%s", intersectionID, key)
}

// GetVersion returns the current version of a memory entry.
func (s *Store) GetVersion(ctx context.Context, ownerID string, key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compositeKey := makeKey(ownerID, key)
	if entry, ok := s.entries[compositeKey]; ok {
		return entry.Version, nil
	}
	return 0, nil
}

// GetHistory returns version history for a memory entry.
func (s *Store) GetHistory(ctx context.Context, ownerID string, key string) ([]memory.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compositeKey := makeKey(ownerID, key)
	history := s.versions[compositeKey]

	// Include current entry
	if current, ok := s.entries[compositeKey]; ok {
		history = append(history, current)
	}

	return history, nil
}

// RecordLoopOutcome records the outcome of a complete loop traversal.
func (s *Store) RecordLoopOutcome(ctx context.Context, loopCtx primitives.LoopContext, outcome memory.LoopOutcome) (*memory.MemoryRecord, error) {
	entry := memory.MemoryEntry{
		Key:   fmt.Sprintf("loop_outcome:%s", outcome.TraceID),
		Value: []byte(fmt.Sprintf("%+v", outcome)),
	}

	if err := s.WriteCircleMemory(ctx, loopCtx.IssuerCircleID, entry); err != nil {
		return nil, err
	}

	return &memory.MemoryRecord{
		RecordID: entry.ID,
		TraceID:  outcome.TraceID,
		StoredAt: time.Now().Format(time.RFC3339),
	}, nil
}

// matchesFilter checks if an entry matches the given filter.
func (s *Store) matchesFilter(entry memory.MemoryEntry, filter memory.Filter) bool {
	if !filter.After.IsZero() && entry.UpdatedAt.Before(filter.After) {
		return false
	}
	if !filter.Before.IsZero() && entry.UpdatedAt.After(filter.Before) {
		return false
	}
	return true
}

// Verify interface compliance at compile time.
var (
	_ memory.Store             = (*Store)(nil)
	_ memory.LoopMemoryUpdater = (*Store)(nil)
)
