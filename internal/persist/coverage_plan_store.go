package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/coverageplan"
)

// CoveragePlanStore stores coverage plan records with hash-only, append-only log.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
type CoveragePlanStore struct {
	mu         sync.RWMutex
	records    []domain.CoveragePlan
	// Index by CircleIDHash for latest lookup
	latestByCircle map[string]int // circleIDHash -> index in records
	// Dedup index by plan hash
	dedupIndex map[string]bool
	clock      func() time.Time
}

// CoveragePlanStore retention bounds.
const (
	CoveragePlanMaxRecords       = 200
	CoveragePlanMaxRetentionDays = 30
)

// NewCoveragePlanStore creates a new coverage plan store with injected clock.
func NewCoveragePlanStore(clock func() time.Time) *CoveragePlanStore {
	return &CoveragePlanStore{
		records:        make([]domain.CoveragePlan, 0),
		latestByCircle: make(map[string]int),
		dedupIndex:     make(map[string]bool),
		clock:          clock,
	}
}

// AppendPlan stores a coverage plan.
// Updates the latest for the circle while maintaining append-only log semantics.
func (s *CoveragePlanStore) AppendPlan(plan domain.CoveragePlan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if already have this exact plan (dedup by plan hash)
	if s.dedupIndex[plan.PlanHash] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append record
	s.records = append(s.records, plan)
	idx := len(s.records) - 1

	// Update latest index for this circle
	s.latestByCircle[plan.CircleIDHash] = idx

	// Update dedup index
	s.dedupIndex[plan.PlanHash] = true

	return nil
}

// LastPlan returns the latest coverage plan for a circle.
func (s *CoveragePlanStore) LastPlan(circleIDHash string) (domain.CoveragePlan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, exists := s.latestByCircle[circleIDHash]
	if !exists || idx >= len(s.records) {
		return domain.CoveragePlan{}, false
	}

	return s.records[idx], true
}

// ListAll returns all coverage plan records.
func (s *CoveragePlanStore) ListAll() []domain.CoveragePlan {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.CoveragePlan, len(s.records))
	copy(result, s.records)
	return result
}

// ListByPeriod returns all plans for a specific period.
func (s *CoveragePlanStore) ListByPeriod(periodKey string) []domain.CoveragePlan {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.CoveragePlan, 0)
	seen := make(map[string]bool)

	// Iterate in reverse to get latest per circle for the period
	for i := len(s.records) - 1; i >= 0; i-- {
		rec := s.records[i]
		if rec.PeriodKey == periodKey && !seen[rec.CircleIDHash] {
			result = append(result, rec)
			seen[rec.CircleIDHash] = true
		}
	}

	// Sort by CircleIDHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].CircleIDHash < result[j].CircleIDHash
	})

	return result
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *CoveragePlanStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -CoveragePlanMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.CoveragePlan, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, rec.PlanHash)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= CoveragePlanMaxRecords {
		evictCount := len(keepRecords) - CoveragePlanMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepRecords[i].PlanHash)
		}
		keepRecords = keepRecords[evictCount:]
	}

	// Rebuild indexes
	s.records = keepRecords
	s.latestByCircle = make(map[string]int)
	for i, rec := range s.records {
		s.latestByCircle[rec.CircleIDHash] = i
	}
}

// Count returns the total number of records.
func (s *CoveragePlanStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// CoverageProofAckStore stores coverage proof acknowledgments.
// Bounded retention: max 200 acks OR 30 days (FIFO eviction).
type CoverageProofAckStore struct {
	mu         sync.RWMutex
	acks       []domain.CoverageProofAck
	// Index by circleIDHash+periodKey for lookup
	dedupIndex map[string]bool // "circleIDHash|periodKey" -> dismissed
	clock      func() time.Time
}

// CoverageProofAckStore retention bounds.
const (
	CoverageProofAckMaxRecords       = 200
	CoverageProofAckMaxRetentionDays = 30
)

// NewCoverageProofAckStore creates a new ack store with injected clock.
func NewCoverageProofAckStore(clock func() time.Time) *CoverageProofAckStore {
	return &CoverageProofAckStore{
		acks:       make([]domain.CoverageProofAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// coverageAckKey generates a key for the dedup index.
func coverageAckKey(circleIDHash, periodKey string) string {
	return circleIDHash + "|" + periodKey
}

// AppendAck records a coverage proof acknowledgment.
func (s *CoverageProofAckStore) AppendAck(ack domain.CoverageProofAck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict old acks first
	s.evictOldAcksLocked()

	// Append ack
	s.acks = append(s.acks, ack)

	// Update index if dismissed
	if ack.AckKind == domain.AckDismissed {
		s.dedupIndex[coverageAckKey(ack.CircleIDHash, ack.PeriodKey)] = true
	}

	return nil
}

// IsAcked checks if proof was acknowledged (dismissed) for a circle and period.
func (s *CoverageProofAckStore) IsAcked(circleIDHash, periodKey, statusHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[coverageAckKey(circleIDHash, periodKey)]
}

// IsProofDismissed checks if proof was dismissed for a circle and period.
func (s *CoverageProofAckStore) IsProofDismissed(circleIDHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[coverageAckKey(circleIDHash, periodKey)]
}

// ListAll returns all acks.
func (s *CoverageProofAckStore) ListAll() []domain.CoverageProofAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.CoverageProofAck, len(s.acks))
	copy(result, s.acks)
	return result
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *CoverageProofAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -CoverageProofAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.CoverageProofAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) >= CoverageProofAckMaxRecords {
		evictCount := len(keepAcks) - CoverageProofAckMaxRecords + 1
		keepAcks = keepAcks[evictCount:]
	}

	// Rebuild index
	s.acks = keepAcks
	s.dedupIndex = make(map[string]bool)
	for _, ack := range s.acks {
		if ack.AckKind == domain.AckDismissed {
			s.dedupIndex[coverageAckKey(ack.CircleIDHash, ack.PeriodKey)] = true
		}
	}
}

// Count returns the total number of acks.
func (s *CoverageProofAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
