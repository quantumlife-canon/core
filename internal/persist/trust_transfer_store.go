// Package persist provides Phase 44 Trust Transfer storage.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw identifiers.
//   - Append-only with replay via storelog.
//   - Bounded retention: 30 days OR 200 records, FIFO eviction.
//   - One active contract per FromCircle per period.
//   - No goroutines. Clock injection required.
//
// Reference: docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md
package persist

import (
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/storelog"
	tt "quantumlife/pkg/domain/trusttransfer"
)

// ============================================================================
// Contract Store
// ============================================================================

// TrustTransferContractStore stores trust transfer contracts.
// CRITICAL: Hash-only. No raw identifiers.
type TrustTransferContractStore struct {
	mu           sync.RWMutex
	contracts    map[string]*ttStoredContract // key: contractHash
	byFromCircle map[string][]string        // fromCircleHash -> []contractHash
	contractList []*ttStoredContract          // for FIFO eviction
	dedupIndex   map[string]bool            // key: contractHash
	storelogRef  storelog.AppendOnlyLog
}

// ttStoredContract wraps a contract with metadata for retention.
type ttStoredContract struct {
	contract   tt.TrustTransferContract
	storedTime time.Time
}

// NewTrustTransferContractStore creates a new trust transfer contract store.
func NewTrustTransferContractStore(storelogRef storelog.AppendOnlyLog) *TrustTransferContractStore {
	return &TrustTransferContractStore{
		contracts:    make(map[string]*ttStoredContract),
		byFromCircle: make(map[string][]string),
		contractList: make([]*ttStoredContract, 0),
		dedupIndex:   make(map[string]bool),
		storelogRef:  storelogRef,
	}
}

// AppendContract stores a contract.
// CRITICAL: Dedup by contractHash.
func (s *TrustTransferContractStore) AppendContract(contract tt.TrustTransferContract, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup by contractHash
	if s.dedupIndex[contract.ContractHash] {
		return nil // Already exists
	}

	stored := &ttStoredContract{
		contract:   contract,
		storedTime: now,
	}

	s.contracts[contract.ContractHash] = stored
	s.contractList = append(s.contractList, stored)
	s.dedupIndex[contract.ContractHash] = true

	// Index by FromCircle
	s.byFromCircle[contract.FromCircleHash] = append(s.byFromCircle[contract.FromCircleHash], contract.ContractHash)

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeTrustTransferContract,
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
func (s *TrustTransferContractStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -tt.MaxRetentionDays)
	newList := make([]*ttStoredContract, 0, len(s.contractList))

	for _, stored := range s.contractList {
		if stored.storedTime.After(cutoff) {
			newList = append(newList, stored)
		} else {
			// Remove from indexes
			delete(s.contracts, stored.contract.ContractHash)
			delete(s.dedupIndex, stored.contract.ContractHash)
			s.removeFromCircleIndex(stored.contract.FromCircleHash, stored.contract.ContractHash)
		}
	}
	s.contractList = newList

	// Evict by count (FIFO)
	for len(s.contractList) > tt.MaxRecords {
		oldest := s.contractList[0]
		s.contractList = s.contractList[1:]
		delete(s.contracts, oldest.contract.ContractHash)
		delete(s.dedupIndex, oldest.contract.ContractHash)
		s.removeFromCircleIndex(oldest.contract.FromCircleHash, oldest.contract.ContractHash)
	}
}

// removeFromCircleIndex removes a contract from the fromCircle index.
func (s *TrustTransferContractStore) removeFromCircleIndex(fromCircleHash, contractHash string) {
	hashes := s.byFromCircle[fromCircleHash]
	var remaining []string
	for _, h := range hashes {
		if h != contractHash {
			remaining = append(remaining, h)
		}
	}
	if len(remaining) == 0 {
		delete(s.byFromCircle, fromCircleHash)
	} else {
		s.byFromCircle[fromCircleHash] = remaining
	}
}

// GetActiveForFromCircle returns the active contract for a FromCircle.
// Returns nil if no active contract exists.
func (s *TrustTransferContractStore) GetActiveForFromCircle(fromCircleHash, periodKey string) *tt.TrustTransferContract {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashes := s.byFromCircle[fromCircleHash]
	if len(hashes) == 0 {
		return nil
	}

	// Find active contract (most recent first)
	// Sort by contract hash for determinism
	sortedHashes := make([]string, len(hashes))
	copy(sortedHashes, hashes)
	sort.Strings(sortedHashes)

	for i := len(sortedHashes) - 1; i >= 0; i-- {
		stored := s.contracts[sortedHashes[i]]
		if stored != nil && stored.contract.State == tt.StateActive {
			contract := stored.contract
			return &contract
		}
	}

	return nil
}

// ListContracts returns all contracts.
func (s *TrustTransferContractStore) ListContracts() []tt.TrustTransferContract {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]tt.TrustTransferContract, 0, len(s.contractList))
	for _, stored := range s.contractList {
		result = append(result, stored.contract)
	}

	// Sort by ContractHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].ContractHash < result[j].ContractHash
	})

	return result
}

// UpdateState updates the state of a contract.
func (s *TrustTransferContractStore) UpdateState(contractHash string, state tt.TransferState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := s.contracts[contractHash]
	if stored == nil {
		return nil // Not found
	}

	stored.contract.State = state
	stored.contract.StatusHash = stored.contract.ComputeHash()

	return nil
}

// Count returns the total number of contracts.
func (s *TrustTransferContractStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.contractList)
}

// EvictOldRecords evicts records older than retention period.
func (s *TrustTransferContractStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}

// ============================================================================
// Revocation Store
// ============================================================================

// TrustTransferRevocationStore stores trust transfer revocations.
// CRITICAL: Hash-only. No raw identifiers.
type TrustTransferRevocationStore struct {
	mu             sync.RWMutex
	revocations    map[string]*ttStoredRevocation // key: revocationHash
	byContract     map[string]string            // contractHash -> revocationHash
	revocationList []*ttStoredRevocation          // for FIFO eviction
	dedupIndex     map[string]bool              // key: revocationHash
	storelogRef    storelog.AppendOnlyLog
}

// ttStoredRevocation wraps a revocation with metadata for retention.
type ttStoredRevocation struct {
	revocation tt.TrustTransferRevocation
	storedTime time.Time
}

// NewTrustTransferRevocationStore creates a new trust transfer revocation store.
func NewTrustTransferRevocationStore(storelogRef storelog.AppendOnlyLog) *TrustTransferRevocationStore {
	return &TrustTransferRevocationStore{
		revocations:    make(map[string]*ttStoredRevocation),
		byContract:     make(map[string]string),
		revocationList: make([]*ttStoredRevocation, 0),
		dedupIndex:     make(map[string]bool),
		storelogRef:    storelogRef,
	}
}

// AppendRevocation stores a revocation.
// CRITICAL: Dedup by revocationHash.
func (s *TrustTransferRevocationStore) AppendRevocation(rev tt.TrustTransferRevocation, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup by revocationHash
	if s.dedupIndex[rev.RevocationHash] {
		return nil // Already exists
	}

	stored := &ttStoredRevocation{
		revocation: rev,
		storedTime: now,
	}

	s.revocations[rev.RevocationHash] = stored
	s.revocationList = append(s.revocationList, stored)
	s.dedupIndex[rev.RevocationHash] = true

	// Index by contract
	s.byContract[rev.ContractHash] = rev.RevocationHash

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeTrustTransferRevocation,
			now,
			"",
			rev.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *TrustTransferRevocationStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -tt.MaxRetentionDays)
	newList := make([]*ttStoredRevocation, 0, len(s.revocationList))

	for _, stored := range s.revocationList {
		if stored.storedTime.After(cutoff) {
			newList = append(newList, stored)
		} else {
			// Remove from indexes
			delete(s.revocations, stored.revocation.RevocationHash)
			delete(s.dedupIndex, stored.revocation.RevocationHash)
			delete(s.byContract, stored.revocation.ContractHash)
		}
	}
	s.revocationList = newList

	// Evict by count (FIFO)
	for len(s.revocationList) > tt.MaxRecords {
		oldest := s.revocationList[0]
		s.revocationList = s.revocationList[1:]
		delete(s.revocations, oldest.revocation.RevocationHash)
		delete(s.dedupIndex, oldest.revocation.RevocationHash)
		delete(s.byContract, oldest.revocation.ContractHash)
	}
}

// ListRevocations returns all revocations.
func (s *TrustTransferRevocationStore) ListRevocations() []tt.TrustTransferRevocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]tt.TrustTransferRevocation, 0, len(s.revocationList))
	for _, stored := range s.revocationList {
		result = append(result, stored.revocation)
	}

	// Sort by RevocationHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].RevocationHash < result[j].RevocationHash
	})

	return result
}

// IsRevoked checks if a contract has been revoked.
func (s *TrustTransferRevocationStore) IsRevoked(contractHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.byContract[contractHash]
	return exists
}

// Count returns the total number of revocations.
func (s *TrustTransferRevocationStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.revocationList)
}

// EvictOldRecords evicts records older than retention period.
func (s *TrustTransferRevocationStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}
