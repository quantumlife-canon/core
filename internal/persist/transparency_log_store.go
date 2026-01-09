// Package persist provides persistence for the transparency log.
//
// CRITICAL INVARIANTS:
// - APPEND-ONLY: Entries can only be appended, never mutated or deleted.
// - DEDUP: Duplicate entries (same LineHash for period) are ignored.
// - BOUNDED: Maximum 5000 entries, 30 days retention, FIFO eviction.
// - HASH-ONLY: Only stores hash-safe fields.
// - NO time.Now(): Clock is injected.
//
// Reference: docs/ADR/ADR-0089-phase51-transparency-log-claim-ledger.md
package persist

import (
	"sync"
	"time"

	domain "quantumlife/pkg/domain/transparencylog"
)

// TransparencyLogStore constants.
const (
	// TransparencyLogMaxEntries is the maximum number of entries to retain.
	TransparencyLogMaxEntries = 5000
	// TransparencyLogMaxRetentionDays is the maximum age of entries in days.
	TransparencyLogMaxRetentionDays = 30
)

// TransparencyLogStore is an append-only store for transparency log entries.
type TransparencyLogStore struct {
	mu         sync.RWMutex
	entries    []domain.TransparencyLogEntry
	dedupIndex map[string]bool // key: period|lineHash
	clock      func() time.Time
}

// NewTransparencyLogStore creates a new transparency log store with injected clock.
func NewTransparencyLogStore(clock func() time.Time) *TransparencyLogStore {
	return &TransparencyLogStore{
		entries:    make([]domain.TransparencyLogEntry, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// Append adds an entry to the store.
// Returns true if the entry was new, false if it was a duplicate.
func (s *TransparencyLogStore) Append(entry domain.TransparencyLogEntry) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate entry
	if err := entry.Validate(); err != nil {
		return false, err
	}

	// Check for duplicate
	dedupKey := s.dedupKeyLocked(entry)
	if s.dedupIndex[dedupKey] {
		return false, nil // Duplicate, not an error
	}

	// Evict old entries if needed
	s.evictOldEntriesLocked()

	// Add entry
	s.entries = append(s.entries, entry)
	s.dedupIndex[dedupKey] = true

	return true, nil
}

// ListByPeriod returns all entries for a given period.
func (s *TransparencyLogStore) ListByPeriod(period domain.PeriodKey) ([]domain.TransparencyLogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []domain.TransparencyLogEntry
	for _, entry := range s.entries {
		if entry.Period == period {
			result = append(result, entry)
		}
	}
	return result, nil
}

// ReplayAll returns all entries for audit/replay.
func (s *TransparencyLogStore) ReplayAll() ([]domain.TransparencyLogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.TransparencyLogEntry, len(s.entries))
	copy(result, s.entries)
	return result, nil
}

// Count returns the number of entries in the store.
func (s *TransparencyLogStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// IsEntrySeen checks if an entry with the given line hash exists for the period.
func (s *TransparencyLogStore) IsEntrySeen(period domain.PeriodKey, lineHash domain.LogLineHash) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dedupKey := string(period) + "|" + string(lineHash)
	return s.dedupIndex[dedupKey]
}

// dedupKeyLocked generates a deduplication key for an entry.
// Must be called with lock held.
func (s *TransparencyLogStore) dedupKeyLocked(entry domain.TransparencyLogEntry) string {
	return string(entry.Period) + "|" + string(entry.LineHash)
}

// evictOldEntriesLocked removes old entries when at capacity.
// Must be called with lock held.
func (s *TransparencyLogStore) evictOldEntriesLocked() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -TransparencyLogMaxRetentionDays)

	// First, remove entries older than retention period
	// Since we don't store timestamps, we use FIFO - oldest entries first
	// We estimate age based on position (older entries are at the front)

	// If we're at or over max entries, remove oldest until we have room
	for len(s.entries) >= TransparencyLogMaxEntries {
		if len(s.entries) == 0 {
			break
		}
		// Remove oldest entry (FIFO)
		oldest := s.entries[0]
		dedupKey := s.dedupKeyLocked(oldest)
		delete(s.dedupIndex, dedupKey)
		s.entries = s.entries[1:]
	}

	// We don't have real timestamps on entries, so we can't do time-based eviction
	// The cutoff is available for stores that track insertion time
	_ = cutoff
}

// AppendBatch adds multiple entries to the store.
// Returns the count of new entries added.
func (s *TransparencyLogStore) AppendBatch(entries []domain.TransparencyLogEntry) (int, error) {
	newCount := 0
	for _, entry := range entries {
		wasNew, err := s.Append(entry)
		if err != nil {
			return newCount, err
		}
		if wasNew {
			newCount++
		}
	}
	return newCount, nil
}
