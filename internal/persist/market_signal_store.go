package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/marketsignal"
)

// MarketSignalStore stores market signals with hash-only, append-only log.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
//
// CRITICAL: No lookups by PackID (prevents analytics).
// CRITICAL: No cross-circle queries (prevents aggregation).
// CRITICAL: Keyed by (CircleHash, PeriodKey) only.
type MarketSignalStore struct {
	mu      sync.RWMutex
	signals []domain.MarketSignal
	// Index by CircleHash+PeriodKey for lookup
	byCirclePeriod map[string][]int // "circleHash|periodKey" -> indices
	// Dedup index by SignalID
	dedupIndex map[string]bool
	clock      func() time.Time
}

// MarketSignalStore retention bounds.
const (
	MarketSignalMaxRecords       = 200
	MarketSignalMaxRetentionDays = 30
)

// NewMarketSignalStore creates a new market signal store with injected clock.
func NewMarketSignalStore(clock func() time.Time) *MarketSignalStore {
	return &MarketSignalStore{
		signals:        make([]domain.MarketSignal, 0),
		byCirclePeriod: make(map[string][]int),
		dedupIndex:     make(map[string]bool),
		clock:          clock,
	}
}

// marketSignalKey generates a key for circle+period lookup.
func marketSignalKey(circleHash, periodKey string) string {
	return circleHash + "|" + periodKey
}

// AppendSignal stores a market signal.
func (s *MarketSignalStore) AppendSignal(signal domain.MarketSignal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if already have this exact signal (dedup by SignalID)
	if s.dedupIndex[signal.SignalID] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append record
	s.signals = append(s.signals, signal)
	idx := len(s.signals) - 1

	// Update circle+period index
	key := marketSignalKey(signal.CircleHash, signal.PeriodKey)
	s.byCirclePeriod[key] = append(s.byCirclePeriod[key], idx)

	// Update dedup index
	s.dedupIndex[signal.SignalID] = true

	return nil
}

// AppendSignals stores multiple signals.
func (s *MarketSignalStore) AppendSignals(signals []domain.MarketSignal) error {
	for _, sig := range signals {
		if err := s.AppendSignal(sig); err != nil {
			return err
		}
	}
	return nil
}

// ListByCirclePeriod returns signals for a specific circle and period.
// CRITICAL: This is the ONLY lookup method - no PackID lookups.
func (s *MarketSignalStore) ListByCirclePeriod(circleHash, periodKey string) []domain.MarketSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := marketSignalKey(circleHash, periodKey)
	indices, exists := s.byCirclePeriod[key]
	if !exists {
		return []domain.MarketSignal{}
	}

	result := make([]domain.MarketSignal, 0, len(indices))
	for _, idx := range indices {
		if idx < len(s.signals) {
			result = append(result, s.signals[idx])
		}
	}

	// Sort by SignalID for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].SignalID < result[j].SignalID
	})

	return result
}

// ListByPeriod returns all signals for a specific period.
func (s *MarketSignalStore) ListByPeriod(periodKey string) []domain.MarketSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.MarketSignal, 0)
	seen := make(map[string]bool)

	for _, sig := range s.signals {
		if sig.PeriodKey == periodKey && !seen[sig.SignalID] {
			result = append(result, sig)
			seen[sig.SignalID] = true
		}
	}

	// Sort by SignalID for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].SignalID < result[j].SignalID
	})

	return result
}

// ListAll returns all signals.
func (s *MarketSignalStore) ListAll() []domain.MarketSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.MarketSignal, len(s.signals))
	copy(result, s.signals)
	return result
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *MarketSignalStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -MarketSignalMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.MarketSignal, 0, len(s.signals))
	for _, sig := range s.signals {
		if sig.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, sig)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, sig.SignalID)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= MarketSignalMaxRecords {
		evictCount := len(keepRecords) - MarketSignalMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepRecords[i].SignalID)
		}
		keepRecords = keepRecords[evictCount:]
	}

	// Rebuild indexes
	s.signals = keepRecords
	s.byCirclePeriod = make(map[string][]int)
	for i, sig := range s.signals {
		key := marketSignalKey(sig.CircleHash, sig.PeriodKey)
		s.byCirclePeriod[key] = append(s.byCirclePeriod[key], i)
	}
}

// Count returns the total number of signals.
func (s *MarketSignalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.signals)
}

// MarketProofAckStore stores market proof acknowledgments.
// Bounded retention: max 200 acks OR 30 days (FIFO eviction).
type MarketProofAckStore struct {
	mu         sync.RWMutex
	acks       []domain.MarketProofAck
	dedupIndex map[string]bool // "circleHash|periodKey" -> dismissed
	clock      func() time.Time
}

// MarketProofAckStore retention bounds.
const (
	MarketProofAckMaxRecords       = 200
	MarketProofAckMaxRetentionDays = 30
)

// NewMarketProofAckStore creates a new ack store with injected clock.
func NewMarketProofAckStore(clock func() time.Time) *MarketProofAckStore {
	return &MarketProofAckStore{
		acks:       make([]domain.MarketProofAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// marketProofAckKey generates a key for the dedup index.
func marketProofAckKey(circleHash, periodKey string) string {
	return circleHash + "|" + periodKey
}

// AppendAck records a market proof acknowledgment.
func (s *MarketProofAckStore) AppendAck(ack domain.MarketProofAck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict old acks first
	s.evictOldAcksLocked()

	// Append ack
	s.acks = append(s.acks, ack)

	// Update index if dismissed
	if ack.AckKind == domain.AckDismissed {
		s.dedupIndex[marketProofAckKey(ack.CircleHash, ack.PeriodKey)] = true
	}

	return nil
}

// IsProofDismissed checks if proof was dismissed for a circle and period.
func (s *MarketProofAckStore) IsProofDismissed(circleHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[marketProofAckKey(circleHash, periodKey)]
}

// ListAll returns all acks.
func (s *MarketProofAckStore) ListAll() []domain.MarketProofAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.MarketProofAck, len(s.acks))
	copy(result, s.acks)
	return result
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *MarketProofAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -MarketProofAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.MarketProofAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) >= MarketProofAckMaxRecords {
		evictCount := len(keepAcks) - MarketProofAckMaxRecords + 1
		keepAcks = keepAcks[evictCount:]
	}

	// Rebuild index
	s.acks = keepAcks
	s.dedupIndex = make(map[string]bool)
	for _, ack := range s.acks {
		if ack.AckKind == domain.AckDismissed {
			s.dedupIndex[marketProofAckKey(ack.CircleHash, ack.PeriodKey)] = true
		}
	}
}

// Count returns the total number of acks.
func (s *MarketProofAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
