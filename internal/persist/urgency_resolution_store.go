// Package persist provides persistence for the urgency resolution store.
//
// CRITICAL INVARIANTS:
// - APPEND-ONLY: Entries can only be appended, never mutated or deleted.
// - BOUNDED: Maximum 500 entries, 30 days retention, FIFO eviction.
// - HASH-ONLY: Only stores hashes, never raw identifiers.
// - CLOCK INJECTION: Clock function is injected, no direct time calls.
// - DEDUP: Deduplication on period|circle|resolutionHash.
//
// Reference: docs/ADR/ADR-0091-phase53-urgency-resolution-layer.md
package persist

import (
	"sync"
	"time"

	domain "quantumlife/pkg/domain/urgencyresolve"
)

// UrgencyResolutionStore constants.
const (
	// UrgencyResolutionMaxEntries is the maximum number of entries to retain.
	UrgencyResolutionMaxEntries = 500
	// UrgencyResolutionMaxRetentionDays is the maximum age of entries in days.
	UrgencyResolutionMaxRetentionDays = 30
)

// UrgencyResolutionEntry is a stored resolution entry with metadata.
type UrgencyResolutionEntry struct {
	Resolution domain.UrgencyResolution
	CreatedAt  time.Time
}

// UrgencyResolutionStore is an append-only store for urgency resolutions.
type UrgencyResolutionStore struct {
	mu         sync.RWMutex
	entries    []UrgencyResolutionEntry
	dedupIndex map[string]bool
	clock      func() time.Time
}

// NewUrgencyResolutionStore creates a new UrgencyResolutionStore with the given clock.
func NewUrgencyResolutionStore(clock func() time.Time) *UrgencyResolutionStore {
	return &UrgencyResolutionStore{
		entries:    make([]UrgencyResolutionEntry, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// dedupKey generates the deduplication key for an entry.
func (s *UrgencyResolutionStore) dedupKey(circleIDHash, periodKey, resolutionHash string) string {
	return periodKey + "|" + circleIDHash + "|" + resolutionHash
}

// RecordResolution records a resolution entry.
// Returns (true, nil) if recorded, (false, nil) if duplicate.
func (s *UrgencyResolutionStore) RecordResolution(resolution domain.UrgencyResolution) (bool, error) {
	if err := resolution.Validate(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	key := s.dedupKey(resolution.CircleIDHash, resolution.PeriodKey, resolution.ResolutionHash)
	if s.dedupIndex[key] {
		return false, nil
	}

	// Evict old entries before adding new one
	s.evictOldEntriesLocked()

	// Add entry
	entry := UrgencyResolutionEntry{
		Resolution: resolution,
		CreatedAt:  s.clock(),
	}
	s.entries = append(s.entries, entry)
	s.dedupIndex[key] = true

	return true, nil
}

// GetLatestResolution returns the latest resolution for a circle and period.
func (s *UrgencyResolutionStore) GetLatestResolution(circleIDHash, periodKey string) *domain.UrgencyResolution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search from newest to oldest
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := s.entries[i]
		if entry.Resolution.CircleIDHash == circleIDHash &&
			entry.Resolution.PeriodKey == periodKey {
			return &entry.Resolution
		}
	}
	return nil
}

// evictOldEntriesLocked evicts old entries. Must be called with lock held.
func (s *UrgencyResolutionStore) evictOldEntriesLocked() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -UrgencyResolutionMaxRetentionDays)

	// Remove entries older than retention period
	newEntries := make([]UrgencyResolutionEntry, 0, len(s.entries))
	newIndex := make(map[string]bool)

	for _, entry := range s.entries {
		if entry.CreatedAt.After(cutoff) {
			newEntries = append(newEntries, entry)
			key := s.dedupKey(entry.Resolution.CircleIDHash, entry.Resolution.PeriodKey, entry.Resolution.ResolutionHash)
			newIndex[key] = true
		}
	}

	s.entries = newEntries
	s.dedupIndex = newIndex

	// If still over max entries, FIFO evict oldest
	for len(s.entries) >= UrgencyResolutionMaxEntries {
		oldest := s.entries[0]
		key := s.dedupKey(oldest.Resolution.CircleIDHash, oldest.Resolution.PeriodKey, oldest.Resolution.ResolutionHash)
		delete(s.dedupIndex, key)
		s.entries = s.entries[1:]
	}
}

// Count returns the number of entries in the store.
func (s *UrgencyResolutionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// ============================================================================
// Ack Store
// ============================================================================

// UrgencyAckEntry is a stored ack entry with metadata.
type UrgencyAckEntry struct {
	Ack       domain.UrgencyAck
	CreatedAt time.Time
}

// UrgencyAckStore is an append-only store for urgency acks.
type UrgencyAckStore struct {
	mu         sync.RWMutex
	entries    []UrgencyAckEntry
	dedupIndex map[string]bool
	clock      func() time.Time
}

// NewUrgencyAckStore creates a new UrgencyAckStore with the given clock.
func NewUrgencyAckStore(clock func() time.Time) *UrgencyAckStore {
	return &UrgencyAckStore{
		entries:    make([]UrgencyAckEntry, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// dedupKey generates the deduplication key for an ack.
func (s *UrgencyAckStore) dedupKey(circleIDHash, periodKey, resolutionHash string) string {
	return periodKey + "|" + circleIDHash + "|" + resolutionHash
}

// RecordDismissed records a dismissal ack.
// Returns (true, nil) if recorded, (false, nil) if duplicate.
func (s *UrgencyAckStore) RecordDismissed(circleIDHash, periodKey, resolutionHash string) (bool, error) {
	ack := domain.UrgencyAck{
		CircleIDHash:   circleIDHash,
		PeriodKey:      periodKey,
		ResolutionHash: resolutionHash,
		AckKind:        domain.AckDismissed,
	}

	if err := ack.Validate(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	key := s.dedupKey(circleIDHash, periodKey, resolutionHash)
	if s.dedupIndex[key] {
		return false, nil
	}

	// Evict old entries before adding new one
	s.evictOldEntriesLocked()

	// Add entry
	entry := UrgencyAckEntry{
		Ack:       ack,
		CreatedAt: s.clock(),
	}
	s.entries = append(s.entries, entry)
	s.dedupIndex[key] = true

	return true, nil
}

// IsDismissed checks if a resolution was dismissed.
func (s *UrgencyAckStore) IsDismissed(circleIDHash, periodKey, resolutionHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.dedupKey(circleIDHash, periodKey, resolutionHash)
	return s.dedupIndex[key]
}

// LastAckedResolutionHash returns the last acked resolution hash for a circle and period.
func (s *UrgencyAckStore) LastAckedResolutionHash(circleIDHash, periodKey string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search from newest to oldest
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := s.entries[i]
		if entry.Ack.CircleIDHash == circleIDHash &&
			entry.Ack.PeriodKey == periodKey {
			return entry.Ack.ResolutionHash, true
		}
	}
	return "", false
}

// evictOldEntriesLocked evicts old entries. Must be called with lock held.
func (s *UrgencyAckStore) evictOldEntriesLocked() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -UrgencyResolutionMaxRetentionDays)

	// Remove entries older than retention period
	newEntries := make([]UrgencyAckEntry, 0, len(s.entries))
	newIndex := make(map[string]bool)

	for _, entry := range s.entries {
		if entry.CreatedAt.After(cutoff) {
			newEntries = append(newEntries, entry)
			key := s.dedupKey(entry.Ack.CircleIDHash, entry.Ack.PeriodKey, entry.Ack.ResolutionHash)
			newIndex[key] = true
		}
	}

	s.entries = newEntries
	s.dedupIndex = newIndex

	// If still over max entries, FIFO evict oldest
	for len(s.entries) >= UrgencyResolutionMaxEntries {
		oldest := s.entries[0]
		key := s.dedupKey(oldest.Ack.CircleIDHash, oldest.Ack.PeriodKey, oldest.Ack.ResolutionHash)
		delete(s.dedupIndex, key)
		s.entries = s.entries[1:]
	}
}

// Count returns the number of entries in the store.
func (s *UrgencyAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}
