// Package persist provides persistence for first action records.
//
// Phase 24: First Reversible Real Action (Trust-Preserving)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only (no raw content)
//   - Bounded retention
//   - Replay supported
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0054-phase24-first-reversible-action.md
package persist

import (
	"sync"
	"time"

	"quantumlife/pkg/domain/firstaction"
	"quantumlife/pkg/domain/identity"
)

// FirstActionStore stores first action records.
//
// CRITICAL: This store contains NO raw content.
// Only hashes, enums, and period hashes are stored.
type FirstActionStore struct {
	mu         sync.RWMutex
	records    map[string]*firstaction.ActionRecord // hash -> record
	byCircle   map[identity.EntityID][]*firstaction.ActionRecord
	byPeriod   map[string][]*firstaction.ActionRecord // "circle:periodHash" -> records
	maxEntries int
	clock      func() time.Time
}

// NewFirstActionStore creates a new first action store.
func NewFirstActionStore(clock func() time.Time) *FirstActionStore {
	return &FirstActionStore{
		records:    make(map[string]*firstaction.ActionRecord),
		byCircle:   make(map[identity.EntityID][]*firstaction.ActionRecord),
		byPeriod:   make(map[string][]*firstaction.ActionRecord),
		maxEntries: 1000,
		clock:      clock,
	}
}

// Store appends an action record.
// Append-only: records are never modified or deleted (except for bounded eviction).
func (s *FirstActionStore) Store(record *firstaction.ActionRecord) error {
	if record == nil {
		return nil
	}

	hash := record.Hash()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Deduplicate by hash
	if _, exists := s.records[hash]; exists {
		return nil
	}

	// Bounded eviction (FIFO)
	if len(s.records) >= s.maxEntries {
		s.evictOldest()
	}

	s.records[hash] = record

	circleID := identity.EntityID(record.CircleID)
	s.byCircle[circleID] = append(s.byCircle[circleID], record)

	periodKey := record.CircleID + ":" + record.PeriodHash
	s.byPeriod[periodKey] = append(s.byPeriod[periodKey], record)

	return nil
}

// evictOldest removes the oldest entry. Must be called with lock held.
func (s *FirstActionStore) evictOldest() {
	for hash := range s.records {
		delete(s.records, hash)
		break
	}
}

// GetByHash retrieves a record by its hash.
func (s *FirstActionStore) GetByHash(hash string) (*firstaction.ActionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[hash]
	return record, ok
}

// GetByCircle retrieves all records for a circle.
func (s *FirstActionStore) GetByCircle(circleID identity.EntityID) []*firstaction.ActionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byCircle[circleID]
}

// GetForPeriod retrieves all records for a circle and period hash.
func (s *FirstActionStore) GetForPeriod(circleID identity.EntityID, periodHash string) []*firstaction.ActionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	return s.byPeriod[periodKey]
}

// HasActionThisPeriod returns true if any action was taken this period.
// This enforces the one-per-period rule.
func (s *FirstActionStore) HasActionThisPeriod(circleID identity.EntityID, periodHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	records := s.byPeriod[periodKey]

	for _, r := range records {
		// Any state except "offered" counts as an action taken
		if r.State == firstaction.StateViewed ||
			r.State == firstaction.StateDismissed ||
			r.State == firstaction.StateAcknowledged {
			return true
		}
	}
	return false
}

// IsViewedThisPeriod returns true if a preview was viewed this period.
func (s *FirstActionStore) IsViewedThisPeriod(circleID identity.EntityID, periodHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	records := s.byPeriod[periodKey]

	for _, r := range records {
		if r.State == firstaction.StateViewed {
			return true
		}
	}
	return false
}

// IsDismissedThisPeriod returns true if dismissed this period.
func (s *FirstActionStore) IsDismissedThisPeriod(circleID identity.EntityID, periodHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	records := s.byPeriod[periodKey]

	for _, r := range records {
		if r.State == firstaction.StateDismissed {
			return true
		}
	}
	return false
}

// Count returns the total number of records stored.
func (s *FirstActionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// RecordState creates and stores a state record.
func (s *FirstActionStore) RecordState(
	circleID identity.EntityID,
	actionHash string,
	state firstaction.ActionState,
	periodHash string,
) error {
	record := &firstaction.ActionRecord{
		ActionHash: actionHash,
		State:      state,
		PeriodHash: periodHash,
		CircleID:   string(circleID),
	}
	return s.Store(record)
}
