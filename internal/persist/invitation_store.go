// Package persist provides persistence for invitation decisions.
//
// Phase 23: Gentle Action Invitation (Trust-Preserving)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only (no raw data, no timestamps, no identifiers)
//   - Bounded retention
//   - Period-scoped
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
package persist

import (
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/invitation"
)

// InvitationStore stores invitation decisions.
//
// CRITICAL: This store contains NO raw data.
// Only hashes, enums, and period hashes are stored.
type InvitationStore struct {
	mu         sync.RWMutex
	records    map[string]*invitation.InvitationRecord // hash -> record
	byCircle   map[identity.EntityID][]*invitation.InvitationRecord
	byPeriod   map[string][]*invitation.InvitationRecord // "circle:periodHash" -> records
	maxEntries int
	clock      func() time.Time
}

// NewInvitationStore creates a new invitation store.
func NewInvitationStore(clock func() time.Time) *InvitationStore {
	return &InvitationStore{
		records:    make(map[string]*invitation.InvitationRecord),
		byCircle:   make(map[identity.EntityID][]*invitation.InvitationRecord),
		byPeriod:   make(map[string][]*invitation.InvitationRecord),
		maxEntries: 1000,
		clock:      clock,
	}
}

// Store appends an invitation record.
// Append-only: records are never modified or deleted (except for bounded eviction).
func (s *InvitationStore) Store(record *invitation.InvitationRecord) error {
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
func (s *InvitationStore) evictOldest() {
	// Find first key and delete
	for hash := range s.records {
		delete(s.records, hash)
		break
	}
}

// GetByHash retrieves a record by its hash.
func (s *InvitationStore) GetByHash(hash string) (*invitation.InvitationRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[hash]
	return record, ok
}

// GetByCircle retrieves all records for a circle.
func (s *InvitationStore) GetByCircle(circleID identity.EntityID) []*invitation.InvitationRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byCircle[circleID]
}

// GetForPeriod retrieves all records for a circle and period hash.
func (s *InvitationStore) GetForPeriod(circleID identity.EntityID, periodHash string) []*invitation.InvitationRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	return s.byPeriod[periodKey]
}

// HasDecisionForPeriod returns true if any decision exists for this period.
func (s *InvitationStore) HasDecisionForPeriod(circleID identity.EntityID, periodHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	records := s.byPeriod[periodKey]
	return len(records) > 0
}

// IsDismissedForPeriod returns true if dismissed this period.
func (s *InvitationStore) IsDismissedForPeriod(circleID identity.EntityID, periodHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	records := s.byPeriod[periodKey]

	for _, r := range records {
		if r.Decision == invitation.DecisionDismissed {
			return true
		}
	}
	return false
}

// IsAcceptedForPeriod returns true if accepted this period.
func (s *InvitationStore) IsAcceptedForPeriod(circleID identity.EntityID, periodHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + periodHash
	records := s.byPeriod[periodKey]

	for _, r := range records {
		if r.Decision == invitation.DecisionAccepted {
			return true
		}
	}
	return false
}

// Count returns the total number of records stored.
func (s *InvitationStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// RecordDecision creates and stores a decision record.
func (s *InvitationStore) RecordDecision(
	circleID identity.EntityID,
	invitationHash string,
	decision invitation.InvitationDecision,
	periodHash string,
) error {
	record := &invitation.InvitationRecord{
		InvitationHash: invitationHash,
		Decision:       decision,
		PeriodHash:     periodHash,
		CircleID:       string(circleID),
	}
	return s.Store(record)
}
