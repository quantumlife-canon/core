// Package persist provides persistence for the urgency delivery store.
//
// CRITICAL INVARIANTS:
// - APPEND-ONLY: Entries can only be appended, never mutated or deleted.
// - BOUNDED: Maximum 200 entries, 30 days retention, FIFO eviction.
// - HASH-ONLY: Only stores hashes, never raw identifiers.
// - CLOCK INJECTION: Clock function is injected, no direct time calls.
// - DEDUP: Deduplication on circle|candidate|period.
//
// Reference: docs/ADR/ADR-0092-phase54-urgency-delivery-binding.md
package persist

import (
	"sync"
	"time"

	domain "quantumlife/pkg/domain/urgencydelivery"
)

// UrgencyDeliveryStore constants.
const (
	// UrgencyDeliveryMaxEntries is the maximum number of entries to retain.
	UrgencyDeliveryMaxEntries = 200
	// UrgencyDeliveryMaxRetentionDays is the maximum age of entries in days.
	UrgencyDeliveryMaxRetentionDays = 30
)

// UrgencyDeliveryEntry is a stored delivery receipt entry with metadata.
type UrgencyDeliveryEntry struct {
	Receipt   domain.UrgencyDeliveryReceipt
	CreatedAt time.Time
}

// UrgencyDeliveryStore is an append-only store for urgency delivery receipts.
type UrgencyDeliveryStore struct {
	mu         sync.RWMutex
	entries    []UrgencyDeliveryEntry
	dedupIndex map[string]bool
	clock      func() time.Time
}

// NewUrgencyDeliveryStore creates a new UrgencyDeliveryStore with the given clock.
func NewUrgencyDeliveryStore(clock func() time.Time) *UrgencyDeliveryStore {
	return &UrgencyDeliveryStore{
		entries:    make([]UrgencyDeliveryEntry, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// AppendReceipt appends a delivery receipt to the store.
// Returns (true, nil) if recorded, (false, nil) if duplicate.
func (s *UrgencyDeliveryStore) AppendReceipt(receipt domain.UrgencyDeliveryReceipt) (bool, error) {
	if err := receipt.Validate(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	key := receipt.DedupKey()
	if s.dedupIndex[key] {
		return false, nil
	}

	// Evict old entries before adding new one
	s.evictOldEntriesLocked()

	// Add entry
	entry := UrgencyDeliveryEntry{
		Receipt:   receipt,
		CreatedAt: s.clock(),
	}
	s.entries = append(s.entries, entry)
	s.dedupIndex[key] = true

	return true, nil
}

// ListRecentByCircle returns the most recent receipts for a circle, up to limit.
func (s *UrgencyDeliveryStore) ListRecentByCircle(circleIDHash string, limit int) []domain.UrgencyDeliveryReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []domain.UrgencyDeliveryReceipt

	// Collect matching entries in reverse order (newest first)
	for i := len(s.entries) - 1; i >= 0 && len(results) < limit; i-- {
		entry := s.entries[i]
		if entry.Receipt.CircleIDHash == circleIDHash {
			results = append(results, entry.Receipt)
		}
	}

	return results
}

// HasReceiptForCandidatePeriod checks if a receipt exists for the given combination.
// Used for deduplication.
func (s *UrgencyDeliveryStore) HasReceiptForCandidatePeriod(circleIDHash, candidateHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := circleIDHash + "|" + candidateHash + "|" + periodKey
	return s.dedupIndex[key]
}

// CountDeliveredForPeriod counts how many deliveries occurred for a circle in a period.
func (s *UrgencyDeliveryStore) CountDeliveredForPeriod(circleIDHash, periodKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, entry := range s.entries {
		if entry.Receipt.CircleIDHash == circleIDHash &&
			entry.Receipt.PeriodKey == periodKey &&
			entry.Receipt.OutcomeKind == domain.OutcomeDelivered {
			count++
		}
	}
	return count
}

// GetLatestReceipt returns the latest receipt for a circle and period.
func (s *UrgencyDeliveryStore) GetLatestReceipt(circleIDHash, periodKey string) *domain.UrgencyDeliveryReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search from newest to oldest
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := s.entries[i]
		if entry.Receipt.CircleIDHash == circleIDHash &&
			entry.Receipt.PeriodKey == periodKey {
			return &entry.Receipt
		}
	}
	return nil
}

// evictOldEntriesLocked evicts old entries. Must be called with lock held.
func (s *UrgencyDeliveryStore) evictOldEntriesLocked() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -UrgencyDeliveryMaxRetentionDays)

	// Remove entries older than retention period
	newEntries := make([]UrgencyDeliveryEntry, 0, len(s.entries))
	newIndex := make(map[string]bool)

	for _, entry := range s.entries {
		if entry.CreatedAt.After(cutoff) {
			newEntries = append(newEntries, entry)
			key := entry.Receipt.DedupKey()
			newIndex[key] = true
		}
	}

	s.entries = newEntries
	s.dedupIndex = newIndex

	// If still over max entries, FIFO evict oldest
	for len(s.entries) >= UrgencyDeliveryMaxEntries {
		oldest := s.entries[0]
		key := oldest.Receipt.DedupKey()
		delete(s.dedupIndex, key)
		s.entries = s.entries[1:]
	}
}

// Count returns the number of entries in the store.
func (s *UrgencyDeliveryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// ListAll returns all entries in the store (for testing/debugging).
func (s *UrgencyDeliveryStore) ListAll() []domain.UrgencyDeliveryReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]domain.UrgencyDeliveryReceipt, len(s.entries))
	for i, entry := range s.entries {
		results[i] = entry.Receipt
	}
	return results
}
