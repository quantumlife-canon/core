package persist

import (
	"sort"
	"sync"
	"time"

	domain "quantumlife/pkg/domain/signedclaims"
)

// ============================================================================
// Phase 50: Signed Claims Store
// ============================================================================
//
// CRITICAL INVARIANTS:
// - HASH-ONLY STORAGE: No raw keys, signatures, vendor names, emails, URLs.
// - BOUNDED RETENTION: Max 200 records OR 30 days (FIFO eviction).
// - PER-CIRCLE DEDUP: By (circleIDHash, periodKey, claimHash/manifestHash).
// - CLOCK INJECTION: Time is injected, never uses time.Now() directly.
//
// Reference: docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md
// ============================================================================

// SignedClaimStore retention bounds.
const (
	SignedClaimMaxRecords       = 200
	SignedClaimMaxRetentionDays = 30
)

// SignedClaimStore stores signed claim records with hash-only, append-only log.
type SignedClaimStore struct {
	mu         sync.RWMutex
	records    []domain.SignedClaimRecord
	dedupIndex map[string]bool // "circleIDHash|periodKey|claimHash" -> exists
	clock      func() time.Time
}

// NewSignedClaimStore creates a new signed claim store with injected clock.
func NewSignedClaimStore(clock func() time.Time) *SignedClaimStore {
	return &SignedClaimStore{
		records:    make([]domain.SignedClaimRecord, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// signedClaimDedupKey generates a dedup key for a claim record.
func signedClaimDedupKey(circleIDHash domain.SafeRefHash, periodKey string, claimHash domain.SafeRefHash) string {
	return string(circleIDHash) + "|" + periodKey + "|" + string(claimHash)
}

// AppendClaim appends a verified claim record.
// Idempotent: same claim hash for same circle+period is a no-op.
func (s *SignedClaimStore) AppendClaim(record domain.SignedClaimRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate dedup key
	dedupKey := signedClaimDedupKey(record.CircleIDHash, record.PeriodKey, record.ClaimHash)

	// Skip if already have this exact claim (dedup)
	if s.dedupIndex[dedupKey] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append record
	s.records = append(s.records, record)

	// Update dedup index
	s.dedupIndex[dedupKey] = true

	return nil
}

// IsClaimSeen checks if a claim has been seen before.
func (s *SignedClaimStore) IsClaimSeen(claimHash domain.SafeRefHash, circleIDHash domain.SafeRefHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dedupKey := signedClaimDedupKey(circleIDHash, periodKey, claimHash)
	return s.dedupIndex[dedupKey]
}

// GetByClaimHash returns a claim record by its hash.
// Returns nil if not found.
func (s *SignedClaimStore) GetByClaimHash(claimHash domain.SafeRefHash) *domain.SignedClaimRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.records {
		if s.records[i].ClaimHash == claimHash {
			rec := s.records[i]
			return &rec
		}
	}
	return nil
}

// ListByCircle returns all claim records for a specific circle.
func (s *SignedClaimStore) ListByCircle(circleIDHash domain.SafeRefHash) []domain.SignedClaimRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedClaimRecord, 0)
	for _, rec := range s.records {
		if rec.CircleIDHash == circleIDHash {
			result = append(result, rec)
		}
	}

	// Sort by ClaimHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ClaimHash < result[j].ClaimHash
	})

	return result
}

// ListByPeriod returns all claim records for a specific period.
func (s *SignedClaimStore) ListByPeriod(periodKey string) []domain.SignedClaimRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedClaimRecord, 0)
	for _, rec := range s.records {
		if rec.PeriodKey == periodKey {
			result = append(result, rec)
		}
	}

	// Sort by ClaimHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ClaimHash < result[j].ClaimHash
	})

	return result
}

// ListByCircleAndPeriod returns claim records for a specific circle and period.
func (s *SignedClaimStore) ListByCircleAndPeriod(circleIDHash domain.SafeRefHash, periodKey string) []domain.SignedClaimRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedClaimRecord, 0)
	for _, rec := range s.records {
		if rec.CircleIDHash == circleIDHash && rec.PeriodKey == periodKey {
			result = append(result, rec)
		}
	}

	// Sort by ClaimHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ClaimHash < result[j].ClaimHash
	})

	return result
}

// ListVerifiedByPeriod returns only verified claim records for a period.
func (s *SignedClaimStore) ListVerifiedByPeriod(periodKey string) []domain.SignedClaimRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedClaimRecord, 0)
	for _, rec := range s.records {
		if rec.PeriodKey == periodKey && rec.Status == domain.VerifiedOK {
			result = append(result, rec)
		}
	}

	// Sort by ClaimHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ClaimHash < result[j].ClaimHash
	})

	return result
}

// ListAll returns all claim records.
func (s *SignedClaimStore) ListAll() []domain.SignedClaimRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedClaimRecord, len(s.records))
	copy(result, s.records)
	return result
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *SignedClaimStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -SignedClaimMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.SignedClaimRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			// Remove from dedup index
			dedupKey := signedClaimDedupKey(rec.CircleIDHash, rec.PeriodKey, rec.ClaimHash)
			delete(s.dedupIndex, dedupKey)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= SignedClaimMaxRecords {
		evictCount := len(keepRecords) - SignedClaimMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			rec := keepRecords[i]
			dedupKey := signedClaimDedupKey(rec.CircleIDHash, rec.PeriodKey, rec.ClaimHash)
			delete(s.dedupIndex, dedupKey)
		}
		keepRecords = keepRecords[evictCount:]
	}

	s.records = keepRecords
}

// Count returns the total number of records.
func (s *SignedClaimStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// ============================================================================
// Signed Manifest Store
// ============================================================================

// SignedManifestStore retention bounds.
const (
	SignedManifestMaxRecords       = 200
	SignedManifestMaxRetentionDays = 30
)

// SignedManifestStore stores signed manifest records with hash-only, append-only log.
type SignedManifestStore struct {
	mu         sync.RWMutex
	records    []domain.SignedManifestRecord
	dedupIndex map[string]bool // "circleIDHash|periodKey|manifestHash" -> exists
	clock      func() time.Time
}

// NewSignedManifestStore creates a new signed manifest store with injected clock.
func NewSignedManifestStore(clock func() time.Time) *SignedManifestStore {
	return &SignedManifestStore{
		records:    make([]domain.SignedManifestRecord, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// signedManifestDedupKey generates a dedup key for a manifest record.
func signedManifestDedupKey(circleIDHash domain.SafeRefHash, periodKey string, manifestHash domain.SafeRefHash) string {
	return string(circleIDHash) + "|" + periodKey + "|" + string(manifestHash)
}

// AppendManifest appends a verified manifest record.
// Idempotent: same manifest hash for same circle+period is a no-op.
func (s *SignedManifestStore) AppendManifest(record domain.SignedManifestRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate dedup key
	dedupKey := signedManifestDedupKey(record.CircleIDHash, record.PeriodKey, record.ManifestHash)

	// Skip if already have this exact manifest (dedup)
	if s.dedupIndex[dedupKey] {
		return nil
	}

	// Evict old records first
	s.evictOldRecordsLocked()

	// Append record
	s.records = append(s.records, record)

	// Update dedup index
	s.dedupIndex[dedupKey] = true

	return nil
}

// IsManifestSeen checks if a manifest has been seen before.
func (s *SignedManifestStore) IsManifestSeen(manifestHash domain.SafeRefHash, circleIDHash domain.SafeRefHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dedupKey := signedManifestDedupKey(circleIDHash, periodKey, manifestHash)
	return s.dedupIndex[dedupKey]
}

// GetByManifestHash returns a manifest record by its hash.
// Returns nil if not found.
func (s *SignedManifestStore) GetByManifestHash(manifestHash domain.SafeRefHash) *domain.SignedManifestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.records {
		if s.records[i].ManifestHash == manifestHash {
			rec := s.records[i]
			return &rec
		}
	}
	return nil
}

// ListByCircle returns all manifest records for a specific circle.
func (s *SignedManifestStore) ListByCircle(circleIDHash domain.SafeRefHash) []domain.SignedManifestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedManifestRecord, 0)
	for _, rec := range s.records {
		if rec.CircleIDHash == circleIDHash {
			result = append(result, rec)
		}
	}

	// Sort by ManifestHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ManifestHash < result[j].ManifestHash
	})

	return result
}

// ListByPeriod returns all manifest records for a specific period.
func (s *SignedManifestStore) ListByPeriod(periodKey string) []domain.SignedManifestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedManifestRecord, 0)
	for _, rec := range s.records {
		if rec.PeriodKey == periodKey {
			result = append(result, rec)
		}
	}

	// Sort by ManifestHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ManifestHash < result[j].ManifestHash
	})

	return result
}

// ListByCircleAndPeriod returns manifest records for a specific circle and period.
func (s *SignedManifestStore) ListByCircleAndPeriod(circleIDHash domain.SafeRefHash, periodKey string) []domain.SignedManifestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedManifestRecord, 0)
	for _, rec := range s.records {
		if rec.CircleIDHash == circleIDHash && rec.PeriodKey == periodKey {
			result = append(result, rec)
		}
	}

	// Sort by ManifestHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ManifestHash < result[j].ManifestHash
	})

	return result
}

// ListVerifiedByPeriod returns only verified manifest records for a period.
func (s *SignedManifestStore) ListVerifiedByPeriod(periodKey string) []domain.SignedManifestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedManifestRecord, 0)
	for _, rec := range s.records {
		if rec.PeriodKey == periodKey && rec.Status == domain.VerifiedOK {
			result = append(result, rec)
		}
	}

	// Sort by ManifestHash for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ManifestHash < result[j].ManifestHash
	})

	return result
}

// ListAll returns all manifest records.
func (s *SignedManifestStore) ListAll() []domain.SignedManifestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedManifestRecord, len(s.records))
	copy(result, s.records)
	return result
}

// evictOldRecordsLocked removes records exceeding bounds.
// Must be called with lock held.
func (s *SignedManifestStore) evictOldRecordsLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -SignedManifestMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find records to keep (within retention window)
	keepRecords := make([]domain.SignedManifestRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec.PeriodKey >= cutoffKey {
			keepRecords = append(keepRecords, rec)
		} else {
			// Remove from dedup index
			dedupKey := signedManifestDedupKey(rec.CircleIDHash, rec.PeriodKey, rec.ManifestHash)
			delete(s.dedupIndex, dedupKey)
		}
	}

	// If at or over max, apply FIFO eviction (leave room for new record)
	if len(keepRecords) >= SignedManifestMaxRecords {
		evictCount := len(keepRecords) - SignedManifestMaxRecords + 1
		for i := 0; i < evictCount; i++ {
			rec := keepRecords[i]
			dedupKey := signedManifestDedupKey(rec.CircleIDHash, rec.PeriodKey, rec.ManifestHash)
			delete(s.dedupIndex, dedupKey)
		}
		keepRecords = keepRecords[evictCount:]
	}

	s.records = keepRecords
}

// Count returns the total number of records.
func (s *SignedManifestStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// ============================================================================
// Signed Claim Proof Ack Store
// ============================================================================

// SignedClaimProofAckStore retention bounds.
const (
	SignedClaimProofAckMaxRecords       = 200
	SignedClaimProofAckMaxRetentionDays = 30
)

// SignedClaimProofAckStore stores proof acknowledgments for signed claims.
type SignedClaimProofAckStore struct {
	mu         sync.RWMutex
	acks       []domain.SignedClaimProofAck
	dedupIndex map[string]bool // "circleIDHash|periodKey" -> dismissed
	clock      func() time.Time
}

// NewSignedClaimProofAckStore creates a new proof ack store with injected clock.
func NewSignedClaimProofAckStore(clock func() time.Time) *SignedClaimProofAckStore {
	return &SignedClaimProofAckStore{
		acks:       make([]domain.SignedClaimProofAck, 0),
		dedupIndex: make(map[string]bool),
		clock:      clock,
	}
}

// signedClaimProofAckDedupKey generates a dedup key for proof ack.
func signedClaimProofAckDedupKey(circleIDHash domain.SafeRefHash, periodKey string) string {
	return string(circleIDHash) + "|" + periodKey
}

// AppendAck records a proof acknowledgment.
func (s *SignedClaimProofAckStore) AppendAck(ack domain.SignedClaimProofAck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict old acks first
	s.evictOldAcksLocked()

	// Append ack
	s.acks = append(s.acks, ack)

	// Update index if dismissed
	if ack.AckKind == domain.ProofAckDismissed {
		s.dedupIndex[signedClaimProofAckDedupKey(ack.CircleIDHash, ack.PeriodKey)] = true
	}

	return nil
}

// IsProofDismissed checks if proof was dismissed for a circle and period.
func (s *SignedClaimProofAckStore) IsProofDismissed(circleIDHash domain.SafeRefHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dedupIndex[signedClaimProofAckDedupKey(circleIDHash, periodKey)]
}

// ListAll returns all acks.
func (s *SignedClaimProofAckStore) ListAll() []domain.SignedClaimProofAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.SignedClaimProofAck, len(s.acks))
	copy(result, s.acks)
	return result
}

// evictOldAcksLocked removes acks exceeding bounds.
// Must be called with lock held.
func (s *SignedClaimProofAckStore) evictOldAcksLocked() {
	now := s.clock()
	cutoffDate := now.AddDate(0, 0, -SignedClaimProofAckMaxRetentionDays)
	cutoffKey := cutoffDate.Format("2006-01-02")

	// Find acks to keep (within retention window)
	keepAcks := make([]domain.SignedClaimProofAck, 0, len(s.acks))
	for _, ack := range s.acks {
		if ack.PeriodKey >= cutoffKey {
			keepAcks = append(keepAcks, ack)
		}
	}

	// If still over max, apply FIFO eviction
	if len(keepAcks) >= SignedClaimProofAckMaxRecords {
		evictCount := len(keepAcks) - SignedClaimProofAckMaxRecords + 1
		keepAcks = keepAcks[evictCount:]
	}

	// Rebuild index
	s.acks = keepAcks
	s.dedupIndex = make(map[string]bool)
	for _, ack := range s.acks {
		if ack.AckKind == domain.ProofAckDismissed {
			s.dedupIndex[signedClaimProofAckDedupKey(ack.CircleIDHash, ack.PeriodKey)] = true
		}
	}
}

// Count returns the total number of acks.
func (s *SignedClaimProofAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.acks)
}
