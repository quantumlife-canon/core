package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/marketplace"
)

// MarketplaceInstallStore stores pack installation records with hash-only, append-only log.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
type MarketplaceInstallStore struct {
	mu         sync.RWMutex
	records    []domain.PackInstallRecord
	// Index by PackSlugHash for latest lookup
	latestByPack map[string]int // packSlugHash -> index in records
	// Dedup index by status hash
	dedupIndex map[string]bool
	clock      func() time.Time
}

// MarketplaceInstallStore retention bounds.
const (
	MarketplaceInstallMaxRecords       = 200
	MarketplaceInstallMaxRetentionDays = 30
)

// NewMarketplaceInstallStore creates a new install store with injected clock.
func NewMarketplaceInstallStore(clock func() time.Time) *MarketplaceInstallStore {
	return &MarketplaceInstallStore{
		records:      make([]domain.PackInstallRecord, 0),
		latestByPack: make(map[string]int),
		dedupIndex:   make(map[string]bool),
		clock:        clock,
	}
}

// Upsert stores or updates an installation record.
// Updates the latest for the pack while maintaining append-only log semantics.
func (s *MarketplaceInstallStore) Upsert(record domain.PackInstallRecord) error {
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

	// Update latest index for this pack
	s.latestByPack[record.PackSlugHash] = idx

	// Update dedup index
	s.dedupIndex[record.StatusHash] = true

	return nil
}

// GetLatest returns the latest installation record for a pack.
func (s *MarketplaceInstallStore) GetLatest(packSlugHash string) (domain.PackInstallRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, exists := s.latestByPack[packSlugHash]
	if !exists || idx >= len(s.records) {
		return domain.PackInstallRecord{}, false
	}

	return s.records[idx], true
}

// ListInstalled returns all currently installed packs.
func (s *MarketplaceInstallStore) ListInstalled() []domain.PackInstallRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.PackInstallRecord, 0)
	for _, idx := range s.latestByPack {
		if idx < len(s.records) {
			rec := s.records[idx]
			if rec.Status == domain.PackStatusInstalled {
				result = append(result, rec)
			}
		}
	}

	// Sort by PackSlugHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].PackSlugHash < result[j].PackSlugHash
	})

	return result
}

// ListAll returns all installation records.
func (s *MarketplaceInstallStore) ListAll() []domain.PackInstallRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.PackInstallRecord, len(s.records))
	copy(result, s.records)
	return result
}

// ListByPeriod returns all records for a specific period.
func (s *MarketplaceInstallStore) ListByPeriod(periodKey string) []domain.PackInstallRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.PackInstallRecord, 0)
	seen := make(map[string]bool)

	// Iterate in reverse to get latest per pack for the period
	for i := len(s.records) - 1; i >= 0; i-- {
		rec := s.records[i]
		if rec.PeriodKey == periodKey && !seen[rec.PackSlugHash] {
			result = append(result, rec)
			seen[rec.PackSlugHash] = true
		}
	}

	// Sort by PackSlugHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].PackSlugHash < result[j].PackSlugHash
	})

	return result
}

// MarkRemoved updates the status of a pack to indicate removal.
// This does NOT delete the record - it appends a new record with removed status.
func (s *MarketplaceInstallStore) MarkRemoved(packSlugHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, exists := s.latestByPack[packSlugHash]
	if !exists || idx >= len(s.records) {
		return nil // Nothing to remove
	}

	oldRec := s.records[idx]
	if oldRec.Status != domain.PackStatusInstalled {
		return nil // Already not installed
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Create new record with available status
	periodKey := s.clock().Format("2006-01-02")
	newRec := domain.PackInstallRecord{
		PeriodKey:    periodKey,
		PackSlugHash: packSlugHash,
		VersionHash:  oldRec.VersionHash,
		StatusHash:   domain.ComputeStatusHash(periodKey, packSlugHash, oldRec.VersionHash),
		Status:       domain.PackStatusAvailable,
		Effect:       domain.EffectNoPower,
	}

	// Append record
	s.records = append(s.records, newRec)
	newIdx := len(s.records) - 1

	// Update latest index
	s.latestByPack[packSlugHash] = newIdx
	s.dedupIndex[newRec.StatusHash] = true

	return nil
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *MarketplaceInstallStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -MarketplaceInstallMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.PackInstallRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, rec.StatusHash)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= MarketplaceInstallMaxRecords {
		evictCount := len(keepRecords) - MarketplaceInstallMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepRecords[i].StatusHash)
		}
		keepRecords = keepRecords[evictCount:]
	}

	// Rebuild indexes
	s.records = keepRecords
	s.latestByPack = make(map[string]int)
	for i, rec := range s.records {
		s.latestByPack[rec.PackSlugHash] = i
	}
}

// Count returns the total number of records.
func (s *MarketplaceInstallStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// MarketplaceRemovalStore stores pack removal records.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
type MarketplaceRemovalStore struct {
	mu         sync.RWMutex
	records    []domain.PackRemovalRecord
	dedupIndex map[string]bool // statusHash -> exists
	clock      func() time.Time
}

// MarketplaceRemovalStore retention bounds.
const (
	MarketplaceRemovalMaxRecords       = 200
	MarketplaceRemovalMaxRetentionDays = 30
)

// NewMarketplaceRemovalStore creates a new removal store with injected clock.
func NewMarketplaceRemovalStore(clock func() time.Time) *MarketplaceRemovalStore {
	return &MarketplaceRemovalStore{
		records:    make([]domain.PackRemovalRecord, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// Record stores a removal record.
func (s *MarketplaceRemovalStore) Record(record domain.PackRemovalRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if already have this exact record
	if s.dedupIndex[record.StatusHash] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append record
	s.records = append(s.records, record)
	s.dedupIndex[record.StatusHash] = true

	return nil
}

// ListAll returns all removal records.
func (s *MarketplaceRemovalStore) ListAll() []domain.PackRemovalRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.PackRemovalRecord, len(s.records))
	copy(result, s.records)
	return result
}

// ListByPeriod returns removal records for a specific period.
func (s *MarketplaceRemovalStore) ListByPeriod(periodKey string) []domain.PackRemovalRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.PackRemovalRecord, 0)
	for _, rec := range s.records {
		if rec.PeriodKey == periodKey {
			result = append(result, rec)
		}
	}

	// Sort for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].PackSlugHash < result[j].PackSlugHash
	})

	return result
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *MarketplaceRemovalStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -MarketplaceRemovalMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.PackRemovalRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			delete(s.dedupIndex, rec.StatusHash)
		}
	}

	// If at or over max, apply FIFO eviction
	if len(keepRecords) >= MarketplaceRemovalMaxRecords {
		evictCount := len(keepRecords) - MarketplaceRemovalMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepRecords[i].StatusHash)
		}
		keepRecords = keepRecords[evictCount:]
	}

	s.records = keepRecords
}

// Count returns the total number of records.
func (s *MarketplaceRemovalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// MarketplaceAckStore stores proof acknowledgments.
// Bounded retention: max 200 acks OR 30 days (FIFO eviction).
type MarketplaceAckStore struct {
	mu         sync.RWMutex
	acks       []domain.MarketplaceProofAck
	dedupIndex map[string]bool // periodKey -> dismissed
	clock      func() time.Time
}

// MarketplaceAckStore retention bounds.
const (
	MarketplaceAckMaxRecords       = 200
	MarketplaceAckMaxRetentionDays = 30
)

// NewMarketplaceAckStore creates a new ack store with injected clock.
func NewMarketplaceAckStore(clock func() time.Time) *MarketplaceAckStore {
	return &MarketplaceAckStore{
		acks:       make([]domain.MarketplaceProofAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// RecordProofAck records a proof acknowledgment.
func (s *MarketplaceAckStore) RecordProofAck(ack domain.MarketplaceProofAck) error {
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
func (s *MarketplaceAckStore) IsProofDismissed(periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[periodKey]
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *MarketplaceAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -MarketplaceAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.MarketplaceProofAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) >= MarketplaceAckMaxRecords {
		evictCount := len(keepAcks) - MarketplaceAckMaxRecords + 1
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
func (s *MarketplaceAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
