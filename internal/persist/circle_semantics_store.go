package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/circlesemantics"
)

// CircleSemanticsStore stores semantics records with hash-only, append-only log.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
type CircleSemanticsStore struct {
	mu      sync.RWMutex
	records []domain.SemanticsRecord
	// Index by CircleIDHash for latest lookup
	latestByCircle map[string]int // circleIDHash -> index in records
	// Dedup index by status hash
	dedupIndex map[string]bool
	clock      func() time.Time
}

// CircleSemanticsStore retention bounds.
const (
	CircleSemanticsMaxRecords       = 200
	CircleSemanticsMaxRetentionDays = 30
)

// NewCircleSemanticsStore creates a new semantics store with injected clock.
func NewCircleSemanticsStore(clock func() time.Time) *CircleSemanticsStore {
	return &CircleSemanticsStore{
		records:        make([]domain.SemanticsRecord, 0),
		latestByCircle: make(map[string]int),
		dedupIndex:     make(map[string]bool),
		clock:          clock,
	}
}

// Upsert stores or updates a semantics record.
// Updates the latest for the circle while maintaining append-only log semantics.
func (s *CircleSemanticsStore) Upsert(record domain.SemanticsRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if already have this exact record (dedup by status hash)
	if s.dedupIndex[record.StatusHash] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append record
	s.records = append(s.records, record)
	idx := len(s.records) - 1

	// Update latest index for this circle
	s.latestByCircle[record.CircleIDHash] = idx

	// Update dedup index
	s.dedupIndex[record.StatusHash] = true

	return nil
}

// GetLatest returns the latest semantics record for a circle.
func (s *CircleSemanticsStore) GetLatest(circleIDHash string) (domain.SemanticsRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, exists := s.latestByCircle[circleIDHash]
	if !exists || idx >= len(s.records) {
		return domain.SemanticsRecord{}, false
	}

	return s.records[idx], true
}

// ListLatestAll returns the latest record for each circle.
func (s *CircleSemanticsStore) ListLatestAll() []domain.SemanticsRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SemanticsRecord, 0, len(s.latestByCircle))
	for _, idx := range s.latestByCircle {
		if idx < len(s.records) {
			result = append(result, s.records[idx])
		}
	}

	// Sort by CircleIDHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].CircleIDHash < result[j].CircleIDHash
	})

	return result
}

// ListByPeriod returns all records for a specific period.
func (s *CircleSemanticsStore) ListByPeriod(periodKey string) []domain.SemanticsRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SemanticsRecord, 0)
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
func (s *CircleSemanticsStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -CircleSemanticsMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.SemanticsRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, rec.StatusHash)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= CircleSemanticsMaxRecords {
		evictCount := len(keepRecords) - CircleSemanticsMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepRecords[i].StatusHash)
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
func (s *CircleSemanticsStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// CircleSemanticsAckStore stores proof acknowledgments.
// Bounded retention: max 200 acks OR 30 days (FIFO eviction).
type CircleSemanticsAckStore struct {
	mu         sync.RWMutex
	acks       []domain.SemanticsProofAck
	dedupIndex map[string]bool // periodKey -> dismissed
	clock      func() time.Time
}

// CircleSemanticsAckStore retention bounds.
const (
	CircleSemanticsAckMaxRecords       = 200
	CircleSemanticsAckMaxRetentionDays = 30
)

// NewCircleSemanticsAckStore creates a new ack store with injected clock.
func NewCircleSemanticsAckStore(clock func() time.Time) *CircleSemanticsAckStore {
	return &CircleSemanticsAckStore{
		acks:       make([]domain.SemanticsProofAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// RecordProofAck records a proof acknowledgment.
func (s *CircleSemanticsAckStore) RecordProofAck(ack domain.SemanticsProofAck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict old acks first
	s.evictOldAcksLocked()

	// Append ack
	s.acks = append(s.acks, ack)

	// Update index if dismissed
	if ack.AckKind == domain.AckKindDismissed {
		s.dedupIndex[ack.PeriodKey] = true
	}

	return nil
}

// IsProofDismissed checks if proof was dismissed for a period.
func (s *CircleSemanticsAckStore) IsProofDismissed(periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[periodKey]
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *CircleSemanticsAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -CircleSemanticsAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.SemanticsProofAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) > CircleSemanticsAckMaxRecords {
		evictCount := len(keepAcks) - CircleSemanticsAckMaxRecords
		keepAcks = keepAcks[evictCount:]
	}

	// Rebuild index
	s.acks = keepAcks
	s.dedupIndex = make(map[string]bool)
	for _, ack := range s.acks {
		if ack.AckKind == domain.AckKindDismissed {
			s.dedupIndex[ack.PeriodKey] = true
		}
	}
}

// Count returns the total number of acks.
func (s *CircleSemanticsAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
