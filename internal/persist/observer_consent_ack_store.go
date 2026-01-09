package persist

import (
	"sync"
	"time"

	domain "quantumlife/pkg/domain/observerconsent"
)

// ObserverConsentAckStore stores observer consent proof acknowledgments.
// Bounded retention: max 200 acks OR 30 days (FIFO eviction).
type ObserverConsentAckStore struct {
	mu         sync.RWMutex
	acks       []domain.ObserverConsentAck
	// Dedup index by dedup key
	dedupIndex map[string]bool
	clock      func() time.Time
}

// ObserverConsentAckStore retention bounds.
const (
	ObserverConsentAckMaxRecords       = 200
	ObserverConsentAckMaxRetentionDays = 30
)

// NewObserverConsentAckStore creates a new ack store with injected clock.
func NewObserverConsentAckStore(clock func() time.Time) *ObserverConsentAckStore {
	return &ObserverConsentAckStore{
		acks:       make([]domain.ObserverConsentAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// AppendAck records a consent proof acknowledgment.
// Returns true if the ack was stored (not a duplicate).
func (s *ObserverConsentAckStore) AppendAck(ack domain.ObserverConsentAck) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check dedup
	dedupKey := ack.DedupKey()
	if s.dedupIndex[dedupKey] {
		return false
	}

	// Evict old acks first
	s.evictOldAcksLocked()

	// Append ack
	s.acks = append(s.acks, ack)

	// Update dedup index
	s.dedupIndex[dedupKey] = true

	return true
}

// IsProofDismissed checks if proof was dismissed for a circle and period.
func (s *ObserverConsentAckStore) IsProofDismissed(circleIDHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dedupKey := periodKey + "|" + circleIDHash + "|" + domain.AckDismissed.CanonicalString()
	return s.dedupIndex[dedupKey]
}

// IsDuplicate checks if an ack with the same dedup key already exists.
func (s *ObserverConsentAckStore) IsDuplicate(ack domain.ObserverConsentAck) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[ack.DedupKey()]
}

// ListAll returns all acks.
func (s *ObserverConsentAckStore) ListAll() []domain.ObserverConsentAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ObserverConsentAck, len(s.acks))
	copy(result, s.acks)
	return result
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *ObserverConsentAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -ObserverConsentAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.ObserverConsentAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, ack.DedupKey())
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) >= ObserverConsentAckMaxRecords {
		evictCount := len(keepAcks) - ObserverConsentAckMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepAcks[i].DedupKey())
		}
		keepAcks = keepAcks[evictCount:]
	}

	// Update acks
	s.acks = keepAcks
}

// Count returns the total number of acks.
func (s *ObserverConsentAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
