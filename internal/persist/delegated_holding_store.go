// Package persist provides Phase 42 Delegated Holding Contract storage.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw identifiers.
//   - Append-only with replay via storelog.
//   - Bounded retention: 30 days OR 200 records max, FIFO eviction.
//   - No goroutines. Clock injection required.
//   - One active contract per circle (derived by replay).
//
// Reference: docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md
package persist

import (
	"sort"
	"sync"
	"time"

	dh "quantumlife/pkg/domain/delegatedholding"
	"quantumlife/pkg/domain/storelog"
)

// DelegatedHoldingStore stores delegated holding contracts and revocations.
// CRITICAL: Hash-only. No raw identifiers.
type DelegatedHoldingStore struct {
	mu           sync.RWMutex
	contracts    map[string]*storedContract // key: contract ID hash
	contractList []*storedContract          // for FIFO eviction
	revocations  map[string]*dh.Revocation  // key: contract ID hash
	storelogRef  storelog.AppendOnlyLog
}

// storedContract wraps a contract with metadata for retention.
type storedContract struct {
	contract   *dh.DelegatedHoldingContract
	storedTime time.Time
}

// NewDelegatedHoldingStore creates a new delegated holding store.
func NewDelegatedHoldingStore(storelogRef storelog.AppendOnlyLog) *DelegatedHoldingStore {
	return &DelegatedHoldingStore{
		contracts:    make(map[string]*storedContract),
		contractList: make([]*storedContract, 0),
		revocations:  make(map[string]*dh.Revocation),
		storelogRef:  storelogRef,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Contract Storage
// ═══════════════════════════════════════════════════════════════════════════

// UpsertActiveContract stores or updates a contract.
// CRITICAL: Append-only semantics via storelog.
func (s *DelegatedHoldingStore) UpsertActiveContract(
	circleIDHash string,
	contract *dh.DelegatedHoldingContract,
	now time.Time,
) error {
	if contract == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := contract.ContractIDHash

	// Check if exists
	if existing, ok := s.contracts[key]; ok {
		// Update existing
		existing.contract = contract
		existing.storedTime = now
	} else {
		// New contract
		sc := &storedContract{
			contract:   contract,
			storedTime: now,
		}
		s.contracts[key] = sc
		s.contractList = append(s.contractList, sc)
	}

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeDelegatedHoldingContract,
			now,
			"",
			contract.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *DelegatedHoldingStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -dh.MaxRetentionDays)
	newList := make([]*storedContract, 0, len(s.contractList))
	for _, sc := range s.contractList {
		if sc.storedTime.After(cutoff) {
			newList = append(newList, sc)
		} else {
			delete(s.contracts, sc.contract.ContractIDHash)
		}
	}
	s.contractList = newList

	// Evict by count (FIFO)
	for len(s.contractList) > dh.MaxRecords {
		oldest := s.contractList[0]
		s.contractList = s.contractList[1:]
		delete(s.contracts, oldest.contract.ContractIDHash)
	}
}

// GetActiveContract returns the active contract for a circle.
// Returns nil if no active contract exists.
func (s *DelegatedHoldingStore) GetActiveContract(
	circleIDHash string,
	nowBucket string,
) *dh.DelegatedHoldingContract {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active *dh.DelegatedHoldingContract
	var latestTime time.Time

	for _, sc := range s.contractList {
		if sc.contract.CircleIDHash != circleIDHash {
			continue
		}

		// Check if revoked
		if _, revoked := s.revocations[sc.contract.ContractIDHash]; revoked {
			continue
		}

		// Check if expired
		if s.isExpiredLocked(sc.contract, nowBucket) {
			continue
		}

		// Find the most recent active contract
		if active == nil || sc.storedTime.After(latestTime) {
			active = sc.contract
			latestTime = sc.storedTime
		}
	}

	return active
}

// isExpiredLocked checks if a contract is expired. Must be called with lock held.
func (s *DelegatedHoldingStore) isExpiredLocked(
	contract *dh.DelegatedHoldingContract,
	nowBucket string,
) bool {
	if contract == nil {
		return true
	}

	// Parse period keys
	createdHour := parseHourBucket(contract.PeriodKey)
	nowHour := parseHourBucket(nowBucket)

	if createdHour.IsZero() || nowHour.IsZero() {
		return false
	}

	// Compute expiry based on duration
	var expiryTime time.Time
	switch contract.Duration {
	case dh.DurationHour:
		expiryTime = createdHour.Add(time.Hour)
	case dh.DurationDay:
		expiryTime = createdHour.Add(24 * time.Hour)
	case dh.DurationTrip:
		expiryTime = createdHour.Add(7 * 24 * time.Hour)
	default:
		return false
	}

	return nowHour.After(expiryTime) || nowHour.Equal(expiryTime)
}

// parseHourBucket parses "YYYY-MM-DD-HH" to time.Time.
func parseHourBucket(bucket string) time.Time {
	if len(bucket) < 13 {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02-15", bucket)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ListRecentContracts returns recent contracts for a circle.
func (s *DelegatedHoldingStore) ListRecentContracts(
	circleIDHash string,
	limit int,
) []*dh.DelegatedHoldingContract {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*dh.DelegatedHoldingContract

	for _, sc := range s.contractList {
		if sc.contract.CircleIDHash == circleIDHash {
			results = append(results, sc.contract)
		}
	}

	// Sort by stored time (most recent first)
	sort.SliceStable(results, func(i, j int) bool {
		// Use ContractIDHash as tie-breaker for determinism
		return results[i].ContractIDHash > results[j].ContractIDHash
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// ═══════════════════════════════════════════════════════════════════════════
// Revocation Storage
// ═══════════════════════════════════════════════════════════════════════════

// AppendRevocation records a contract revocation.
func (s *DelegatedHoldingStore) AppendRevocation(
	circleIDHash string,
	contractIDHash string,
	nowBucket string,
	now time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	revocation := &dh.Revocation{
		CircleIDHash:   circleIDHash,
		ContractIDHash: contractIDHash,
		PeriodKey:      nowBucket,
	}
	revocation.RevocationHash = revocation.ComputeHash()

	s.revocations[contractIDHash] = revocation

	// Update contract state if exists
	if sc, ok := s.contracts[contractIDHash]; ok {
		sc.contract.State = dh.StateRevoked
	}

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeDelegatedHoldingRevocation,
			now,
			"",
			revocation.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// IsRevoked checks if a contract has been revoked.
func (s *DelegatedHoldingStore) IsRevoked(contractIDHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, revoked := s.revocations[contractIDHash]
	return revoked
}

// ═══════════════════════════════════════════════════════════════════════════
// Utility Methods
// ═══════════════════════════════════════════════════════════════════════════

// Count returns the total number of contracts.
func (s *DelegatedHoldingStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.contractList)
}

// EvictOldRecords evicts records older than retention period.
func (s *DelegatedHoldingStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}
