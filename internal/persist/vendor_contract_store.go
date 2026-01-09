package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/vendorcontract"
)

// VendorContractStore stores vendor contracts with hash-only, append-only log.
// Bounded retention: max 200 records OR 30 days (FIFO eviction).
//
// CRITICAL: Hash-only storage - no vendor names, emails, URLs.
// CRITICAL: Keyed by (VendorCircleHash, PeriodKey).
// CRITICAL: One active contract per VendorCircleHash per period.
type VendorContractStore struct {
	mu      sync.RWMutex
	records []domain.VendorContractRecord
	// Index by VendorCircleHash+PeriodKey for active contract lookup
	byVendorPeriod map[string]int // "vendorHash|periodKey" -> index
	// Dedup index by ContractHash
	dedupIndex map[string]bool
	clock      func() time.Time
}

// VendorContractStore retention bounds.
const (
	VendorContractMaxRecords       = 200
	VendorContractMaxRetentionDays = 30
)

// NewVendorContractStore creates a new vendor contract store with injected clock.
func NewVendorContractStore(clock func() time.Time) *VendorContractStore {
	return &VendorContractStore{
		records:        make([]domain.VendorContractRecord, 0),
		byVendorPeriod: make(map[string]int),
		dedupIndex:     make(map[string]bool),
		clock:          clock,
	}
}

// vendorContractKey generates a key for vendor+period lookup.
func vendorContractKey(vendorHash, periodKey string) string {
	return vendorHash + "|" + periodKey
}

// UpsertActiveContract upserts an active contract.
// Idempotent: same contract hash is a no-op.
func (s *VendorContractStore) UpsertActiveContract(vendorHash, periodKey, contractHash string, cap domain.PressureAllowance, scope domain.ContractScope) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if already have this exact contract (dedup by ContractHash)
	if s.dedupIndex[contractHash] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Build record
	record := domain.VendorContractRecord{
		ContractHash:     contractHash,
		VendorCircleHash: vendorHash,
		Scope:            scope,
		EffectiveCap:     cap,
		Status:           domain.StatusActive,
		CreatedAtBucket:  periodKey,
		PeriodKey:        periodKey,
	}

	// Append record
	s.records = append(s.records, record)
	idx := len(s.records) - 1

	// Update vendor+period index (overwrites previous if exists)
	key := vendorContractKey(vendorHash, periodKey)
	s.byVendorPeriod[key] = idx

	// Update dedup index
	s.dedupIndex[contractHash] = true

	return nil
}

// RevokeContract marks a contract as revoked.
// Idempotent: revoking non-existent contract is a no-op.
func (s *VendorContractStore) RevokeContract(vendorHash, periodKey, contractHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find contract by hash
	for i := range s.records {
		if s.records[i].ContractHash == contractHash {
			s.records[i].Status = domain.StatusRevoked
			// Remove from active index
			key := vendorContractKey(vendorHash, periodKey)
			if s.byVendorPeriod[key] == i {
				delete(s.byVendorPeriod, key)
			}
			break
		}
	}

	return nil
}

// GetActiveContract returns the active contract for a vendor and period.
// Returns nil if no active contract exists.
func (s *VendorContractStore) GetActiveContract(vendorHash, periodKey string) *domain.VendorContractRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := vendorContractKey(vendorHash, periodKey)
	idx, exists := s.byVendorPeriod[key]
	if !exists || idx >= len(s.records) {
		return nil
	}

	record := s.records[idx]
	if record.Status != domain.StatusActive {
		return nil
	}

	return &record
}

// ListByPeriod returns all records for a specific period.
func (s *VendorContractStore) ListByPeriod(periodKey string) []domain.VendorContractRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.VendorContractRecord, 0)
	seen := make(map[string]bool)

	for _, rec := range s.records {
		if rec.PeriodKey == periodKey && !seen[rec.ContractHash] {
			result = append(result, rec)
			seen[rec.ContractHash] = true
		}
	}

	// Sort by ContractHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ContractHash < result[j].ContractHash
	})

	return result
}

// ListActiveByPeriod returns only active records for a specific period.
func (s *VendorContractStore) ListActiveByPeriod(periodKey string) []domain.VendorContractRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.VendorContractRecord, 0)
	seen := make(map[string]bool)

	for _, rec := range s.records {
		if rec.PeriodKey == periodKey && rec.Status == domain.StatusActive && !seen[rec.ContractHash] {
			result = append(result, rec)
			seen[rec.ContractHash] = true
		}
	}

	// Sort by ContractHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ContractHash < result[j].ContractHash
	})

	return result
}

// ListAll returns all records.
func (s *VendorContractStore) ListAll() []domain.VendorContractRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.VendorContractRecord, len(s.records))
	copy(result, s.records)
	return result
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *VendorContractStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -VendorContractMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.VendorContractRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			// Remove from dedup index
			delete(s.dedupIndex, rec.ContractHash)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= VendorContractMaxRecords {
		evictCount := len(keepRecords) - VendorContractMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			delete(s.dedupIndex, keepRecords[i].ContractHash)
		}
		keepRecords = keepRecords[evictCount:]
	}

	// Rebuild indexes
	s.records = keepRecords
	s.byVendorPeriod = make(map[string]int)
	for i, rec := range s.records {
		if rec.Status == domain.StatusActive {
			key := vendorContractKey(rec.VendorCircleHash, rec.PeriodKey)
			s.byVendorPeriod[key] = i
		}
	}
}

// Count returns the total number of records.
func (s *VendorContractStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// ============================================================================
// VendorProofAckStore
// ============================================================================

// VendorProofAckStore stores vendor proof acknowledgments.
// Bounded retention: max 200 acks OR 30 days (FIFO eviction).
type VendorProofAckStore struct {
	mu         sync.RWMutex
	acks       []domain.VendorProofAck
	dedupIndex map[string]bool // "vendorHash|periodKey" -> dismissed
	clock      func() time.Time
}

// VendorProofAckStore retention bounds.
const (
	VendorProofAckMaxRecords       = 200
	VendorProofAckMaxRetentionDays = 30
)

// NewVendorProofAckStore creates a new ack store with injected clock.
func NewVendorProofAckStore(clock func() time.Time) *VendorProofAckStore {
	return &VendorProofAckStore{
		acks:       make([]domain.VendorProofAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// vendorProofAckKey generates a key for the dedup index.
func vendorProofAckKey(vendorHash, periodKey string) string {
	return vendorHash + "|" + periodKey
}

// AppendAck records a vendor proof acknowledgment.
func (s *VendorProofAckStore) AppendAck(ack domain.VendorProofAck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict old acks first
	s.evictOldAcksLocked()

	// Append ack
	s.acks = append(s.acks, ack)

	// Update index if dismissed
	if ack.AckKind == domain.VendorAckDismissed {
		s.dedupIndex[vendorProofAckKey(ack.VendorCircleHash, ack.PeriodKey)] = true
	}

	return nil
}

// IsProofDismissed checks if proof was dismissed for a vendor and period.
func (s *VendorProofAckStore) IsProofDismissed(vendorHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[vendorProofAckKey(vendorHash, periodKey)]
}

// IsProofDismissedForPeriod checks if any proof was dismissed for a period.
func (s *VendorProofAckStore) IsProofDismissedForPeriod(periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Look for any dismissal in this period
	for _, ack := range s.acks {
		if ack.PeriodKey == periodKey && ack.AckKind == domain.VendorAckDismissed {
			return true
		}
	}
	return false
}

// ListAll returns all acks.
func (s *VendorProofAckStore) ListAll() []domain.VendorProofAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.VendorProofAck, len(s.acks))
	copy(result, s.acks)
	return result
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *VendorProofAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -VendorProofAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.VendorProofAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) >= VendorProofAckMaxRecords {
		evictCount := len(keepAcks) - VendorProofAckMaxRecords + 1
		keepAcks = keepAcks[evictCount:]
	}

	// Rebuild index
	s.acks = keepAcks
	s.dedupIndex = make(map[string]bool)
	for _, ack := range s.acks {
		if ack.AckKind == domain.VendorAckDismissed {
			s.dedupIndex[vendorProofAckKey(ack.VendorCircleHash, ack.PeriodKey)] = true
		}
	}
}

// Count returns the total number of acks.
func (s *VendorProofAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
