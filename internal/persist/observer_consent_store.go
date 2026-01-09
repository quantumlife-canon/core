package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/observerconsent"
)

// ObserverConsentStore stores observer consent receipts with hash-only, append-only log.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
type ObserverConsentStore struct {
	mu       sync.RWMutex
	receipts []domain.ObserverConsentReceipt
	// Dedup index by dedup key
	dedupIndex map[string]bool
	clock      func() time.Time
}

// ObserverConsentStore retention bounds.
const (
	ObserverConsentMaxRecords       = 200
	ObserverConsentMaxRetentionDays = 30
)

// NewObserverConsentStore creates a new observer consent store with injected clock.
func NewObserverConsentStore(clock func() time.Time) *ObserverConsentStore {
	return &ObserverConsentStore{
		receipts:   make([]domain.ObserverConsentReceipt, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// AppendReceipt stores a consent receipt.
// Returns true if the receipt was stored (not a duplicate).
func (s *ObserverConsentStore) AppendReceipt(receipt domain.ObserverConsentReceipt) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check dedup
	dedupKey := receipt.DedupKey()
	if s.dedupIndex[dedupKey] {
		return false
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append receipt
	s.receipts = append(s.receipts, receipt)

	// Update dedup index
	s.dedupIndex[dedupKey] = true

	return true
}

// ListByCircle returns all receipts for a specific circle, sorted by period descending.
func (s *ObserverConsentStore) ListByCircle(circleIDHash string) []domain.ObserverConsentReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ObserverConsentReceipt, 0)
	for _, receipt := range s.receipts {
		if receipt.CircleIDHash == circleIDHash {
			result = append(result, receipt)
		}
	}

	// Sort by period key descending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].PeriodKey > result[j].PeriodKey
	})

	return result
}

// ListByCircleAndPeriod returns all receipts for a specific circle and period.
func (s *ObserverConsentStore) ListByCircleAndPeriod(circleIDHash, periodKey string) []domain.ObserverConsentReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ObserverConsentReceipt, 0)
	for _, receipt := range s.receipts {
		if receipt.CircleIDHash == circleIDHash && receipt.PeriodKey == periodKey {
			result = append(result, receipt)
		}
	}

	return result
}

// ListAll returns all receipts.
func (s *ObserverConsentStore) ListAll() []domain.ObserverConsentReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ObserverConsentReceipt, len(s.receipts))
	copy(result, s.receipts)
	return result
}

// IsDuplicate checks if a receipt with the same dedup key already exists.
func (s *ObserverConsentStore) IsDuplicate(receipt domain.ObserverConsentReceipt) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[receipt.DedupKey()]
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *ObserverConsentStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -ObserverConsentMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepReceipts := make([]domain.ObserverConsentReceipt, 0, len(s.receipts))
	for _, receipt := range s.receipts {
		if receipt.PeriodKey >= cutoffKey {
			keepReceipts = append(keepReceipts, receipt)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, receipt.DedupKey())
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepReceipts) >= ObserverConsentMaxRecords {
		evictCount := len(keepReceipts) - ObserverConsentMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepReceipts[i].DedupKey())
		}
		keepReceipts = keepReceipts[evictCount:]
	}

	// Update receipts
	s.receipts = keepReceipts
}

// Count returns the total number of receipts.
func (s *ObserverConsentStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receipts)
}
