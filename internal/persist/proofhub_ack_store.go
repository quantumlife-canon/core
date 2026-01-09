// Package persist provides persistence for the proof hub ack store.
//
// CRITICAL INVARIANTS:
// - APPEND-ONLY: Entries can only be appended, never mutated or deleted.
// - BOUNDED: Maximum 200 entries, 30 days retention, FIFO eviction.
// - HASH-ONLY: Only stores hashes, never raw identifiers.
// - CLOCK INJECTION: Clock function is injected, no direct time calls.
//
// Reference: docs/ADR/ADR-0090-phase52-proof-hub-connected-status.md
package persist

import (
	"sync"
	"time"

	domain "quantumlife/pkg/domain/proofhub"
)

// ProofHubAckStore constants.
const (
	// ProofHubAckMaxEntries is the maximum number of entries to retain.
	ProofHubAckMaxEntries = 200
	// ProofHubAckMaxRetentionDays is the maximum age of entries in days.
	ProofHubAckMaxRetentionDays = 30
)

// ProofHubAckEntry is a stored ack entry with metadata.
type ProofHubAckEntry struct {
	Ack       domain.ProofHubAck
	CreatedAt time.Time
}

// ProofHubAckStore is an append-only store for proof hub acknowledgments.
type ProofHubAckStore struct {
	mu         sync.RWMutex
	entries    []ProofHubAckEntry
	dedupIndex map[string]bool // key: circleIDHash|periodKey|statusHash
	clock      func() time.Time
}

// NewProofHubAckStore creates a new proof hub ack store with injected clock.
func NewProofHubAckStore(clock func() time.Time) *ProofHubAckStore {
	return &ProofHubAckStore{
		entries:    make([]ProofHubAckEntry, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// RecordDismissed records that the proof hub was dismissed.
// Returns true if the entry was new, false if it was a duplicate.
func (s *ProofHubAckStore) RecordDismissed(circleIDHash, periodKey, statusHash string) (bool, error) {
	ack := domain.ProofHubAck{
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
		StatusHash:   statusHash,
		AckKind:      domain.AckDismissed,
	}

	if err := ack.Validate(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	dedupKey := s.dedupKeyLocked(circleIDHash, periodKey, statusHash)
	if s.dedupIndex[dedupKey] {
		return false, nil // Duplicate, not an error
	}

	// Evict old entries if needed
	s.evictOldEntriesLocked()

	// Add entry
	entry := ProofHubAckEntry{
		Ack:       ack,
		CreatedAt: s.clock(),
	}
	s.entries = append(s.entries, entry)
	s.dedupIndex[dedupKey] = true

	return true, nil
}

// IsDismissed checks if the proof hub was dismissed for this period+status.
func (s *ProofHubAckStore) IsDismissed(circleIDHash, periodKey, statusHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dedupKey := s.dedupKeyLocked(circleIDHash, periodKey, statusHash)
	return s.dedupIndex[dedupKey]
}

// LastAckedStatusHash returns the last acked status hash for the period.
func (s *ProofHubAckStore) LastAckedStatusHash(circleIDHash, periodKey string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the most recent ack for this circle+period
	var lastHash string
	var found bool
	for _, entry := range s.entries {
		if entry.Ack.CircleIDHash == circleIDHash && entry.Ack.PeriodKey == periodKey {
			lastHash = entry.Ack.StatusHash
			found = true
		}
	}
	return lastHash, found
}

// Count returns the number of entries in the store.
func (s *ProofHubAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// ReplayAll returns all entries for audit/replay.
func (s *ProofHubAckStore) ReplayAll() []ProofHubAckEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ProofHubAckEntry, len(s.entries))
	copy(result, s.entries)
	return result
}

// dedupKeyLocked generates a deduplication key for an entry.
// Must be called with lock held.
func (s *ProofHubAckStore) dedupKeyLocked(circleIDHash, periodKey, statusHash string) string {
	return circleIDHash + "|" + periodKey + "|" + statusHash
}

// evictOldEntriesLocked removes old entries when at capacity.
// Must be called with lock held.
func (s *ProofHubAckStore) evictOldEntriesLocked() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -ProofHubAckMaxRetentionDays)

	// First, remove entries older than retention period
	var newEntries []ProofHubAckEntry
	newDedupIndex := make(map[string]bool)

	for _, entry := range s.entries {
		if entry.CreatedAt.After(cutoff) {
			newEntries = append(newEntries, entry)
			dedupKey := s.dedupKeyLocked(entry.Ack.CircleIDHash, entry.Ack.PeriodKey, entry.Ack.StatusHash)
			newDedupIndex[dedupKey] = true
		}
	}

	s.entries = newEntries
	s.dedupIndex = newDedupIndex

	// If still at or over max entries, remove oldest (FIFO)
	for len(s.entries) >= ProofHubAckMaxEntries {
		if len(s.entries) == 0 {
			break
		}
		// Remove oldest entry
		oldest := s.entries[0]
		dedupKey := s.dedupKeyLocked(oldest.Ack.CircleIDHash, oldest.Ack.PeriodKey, oldest.Ack.StatusHash)
		delete(s.dedupIndex, dedupKey)
		s.entries = s.entries[1:]
	}
}
