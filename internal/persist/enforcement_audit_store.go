// Package persist provides Phase 44.2 Enforcement Audit storage.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw identifiers.
//   - Append-only with replay via storelog.
//   - Bounded retention: 30 days OR 100 records, FIFO eviction.
//   - Dedup by runHash.
//   - No goroutines. Clock injection required.
//
// Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
package persist

import (
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/enforcementaudit"
	"quantumlife/pkg/domain/storelog"
)

// ============================================================================
// Audit Run Store
// ============================================================================

// EnforcementAuditStore stores enforcement audit runs.
// CRITICAL: Hash-only. No raw identifiers.
type EnforcementAuditStore struct {
	mu          sync.RWMutex
	runs        map[string]*eaStoredRun // key: runHash
	runList     []*eaStoredRun          // for FIFO eviction
	dedupIndex  map[string]bool         // key: runHash
	storelogRef storelog.AppendOnlyLog
}

// eaStoredRun wraps a run with metadata for retention.
type eaStoredRun struct {
	run        enforcementaudit.AuditRun
	storedTime time.Time
}

// NewEnforcementAuditStore creates a new enforcement audit store.
func NewEnforcementAuditStore(storelogRef storelog.AppendOnlyLog) *EnforcementAuditStore {
	return &EnforcementAuditStore{
		runs:        make(map[string]*eaStoredRun),
		runList:     make([]*eaStoredRun, 0),
		dedupIndex:  make(map[string]bool),
		storelogRef: storelogRef,
	}
}

// AppendRun stores an audit run.
// CRITICAL: Dedup by runHash.
func (s *EnforcementAuditStore) AppendRun(run enforcementaudit.AuditRun, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup by runHash
	if s.dedupIndex[run.RunHash] {
		return nil // Already exists
	}

	stored := &eaStoredRun{
		run:        run,
		storedTime: now,
	}

	s.runs[run.RunHash] = stored
	s.runList = append(s.runList, stored)
	s.dedupIndex[run.RunHash] = true

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeEnforcementAuditRun,
			now,
			"",
			run.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *EnforcementAuditStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -enforcementaudit.MaxRetentionDays)
	newList := make([]*eaStoredRun, 0, len(s.runList))

	for _, stored := range s.runList {
		if stored.storedTime.After(cutoff) {
			newList = append(newList, stored)
		} else {
			// Remove from indexes
			delete(s.runs, stored.run.RunHash)
			delete(s.dedupIndex, stored.run.RunHash)
		}
	}
	s.runList = newList

	// Evict by count (FIFO)
	for len(s.runList) > enforcementaudit.MaxRecords {
		oldest := s.runList[0]
		s.runList = s.runList[1:]
		delete(s.runs, oldest.run.RunHash)
		delete(s.dedupIndex, oldest.run.RunHash)
	}
}

// GetLatestRun returns the most recent audit run.
func (s *EnforcementAuditStore) GetLatestRun() *enforcementaudit.AuditRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.runList) == 0 {
		return nil
	}

	// Return the most recent
	latest := s.runList[len(s.runList)-1]
	run := latest.run
	return &run
}

// ListRuns returns all audit runs.
func (s *EnforcementAuditStore) ListRuns() []enforcementaudit.AuditRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]enforcementaudit.AuditRun, 0, len(s.runList))
	for _, stored := range s.runList {
		result = append(result, stored.run)
	}

	// Sort by RunHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].RunHash < result[j].RunHash
	})

	return result
}

// Count returns the total number of runs.
func (s *EnforcementAuditStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runList)
}

// EvictOldRecords evicts records older than retention period.
func (s *EnforcementAuditStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}

// ============================================================================
// Audit Ack Store
// ============================================================================

// EnforcementAuditAckStore stores enforcement audit acknowledgments.
// CRITICAL: Hash-only. No raw identifiers.
type EnforcementAuditAckStore struct {
	mu          sync.RWMutex
	acks        map[string]*eaStoredAck // key: ackHash
	byRunHash   map[string]string       // runHash -> ackHash
	ackList     []*eaStoredAck          // for FIFO eviction
	dedupIndex  map[string]bool         // key: ackHash
	storelogRef storelog.AppendOnlyLog
}

// eaStoredAck wraps an ack with metadata for retention.
type eaStoredAck struct {
	ack        enforcementaudit.AuditAck
	storedTime time.Time
}

// NewEnforcementAuditAckStore creates a new enforcement audit ack store.
func NewEnforcementAuditAckStore(storelogRef storelog.AppendOnlyLog) *EnforcementAuditAckStore {
	return &EnforcementAuditAckStore{
		acks:        make(map[string]*eaStoredAck),
		byRunHash:   make(map[string]string),
		ackList:     make([]*eaStoredAck, 0),
		dedupIndex:  make(map[string]bool),
		storelogRef: storelogRef,
	}
}

// AppendAck stores an audit acknowledgment.
// CRITICAL: Dedup by ackHash.
func (s *EnforcementAuditAckStore) AppendAck(ack enforcementaudit.AuditAck, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup by ackHash
	if s.dedupIndex[ack.AckHash] {
		return nil // Already exists
	}

	stored := &eaStoredAck{
		ack:        ack,
		storedTime: now,
	}

	s.acks[ack.AckHash] = stored
	s.ackList = append(s.ackList, stored)
	s.dedupIndex[ack.AckHash] = true

	// Index by runHash
	s.byRunHash[ack.RunHash] = ack.AckHash

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeEnforcementAuditAck,
			now,
			"",
			ack.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *EnforcementAuditAckStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -enforcementaudit.MaxRetentionDays)
	newList := make([]*eaStoredAck, 0, len(s.ackList))

	for _, stored := range s.ackList {
		if stored.storedTime.After(cutoff) {
			newList = append(newList, stored)
		} else {
			// Remove from indexes
			delete(s.acks, stored.ack.AckHash)
			delete(s.dedupIndex, stored.ack.AckHash)
			delete(s.byRunHash, stored.ack.RunHash)
		}
	}
	s.ackList = newList

	// Evict by count (FIFO)
	for len(s.ackList) > enforcementaudit.MaxRecords {
		oldest := s.ackList[0]
		s.ackList = s.ackList[1:]
		delete(s.acks, oldest.ack.AckHash)
		delete(s.dedupIndex, oldest.ack.AckHash)
		delete(s.byRunHash, oldest.ack.RunHash)
	}
}

// IsAcked checks if a run has been acknowledged.
func (s *EnforcementAuditAckStore) IsAcked(runHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.byRunHash[runHash]
	return exists
}

// ListAcks returns all acknowledgments.
func (s *EnforcementAuditAckStore) ListAcks() []enforcementaudit.AuditAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]enforcementaudit.AuditAck, 0, len(s.ackList))
	for _, stored := range s.ackList {
		result = append(result, stored.ack)
	}

	// Sort by AckHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].AckHash < result[j].AckHash
	})

	return result
}

// Count returns the total number of acks.
func (s *EnforcementAuditAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.ackList)
}

// EvictOldRecords evicts records older than retention period.
func (s *EnforcementAuditAckStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}
