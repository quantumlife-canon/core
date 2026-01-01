// Package draft - store interface and implementation.
package draft

import (
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
)

// ListFilter specifies criteria for listing drafts.
type ListFilter struct {
	// CircleID filters by circle (empty = all).
	CircleID identity.EntityID

	// Status filters by status (empty = all).
	Status DraftStatus

	// DraftType filters by type (empty = all).
	DraftType DraftType

	// IncludeExpired includes expired drafts (default false).
	IncludeExpired bool

	// Limit caps the number of results (0 = no limit).
	Limit int
}

// Store defines the interface for draft persistence.
type Store interface {
	// Put stores a draft.
	Put(d Draft) error

	// Get retrieves a draft by ID.
	Get(id DraftID) (Draft, bool)

	// GetByDedupKey finds a draft by its dedup key.
	GetByDedupKey(dedupKey string) (Draft, bool)

	// List returns drafts matching the filter, sorted deterministically.
	List(filter ListFilter) []Draft

	// UpdateStatus changes the status of a draft.
	UpdateStatus(id DraftID, status DraftStatus, reason string, changedBy string, changedAt time.Time) error

	// MarkExpired marks all expired drafts as expired based on the given time.
	// Returns the count of drafts marked.
	MarkExpired(now time.Time) int

	// Count returns the total number of drafts.
	Count() int

	// CountByCircleAndDay returns the count of drafts for a circle on a day.
	CountByCircleAndDay(circleID identity.EntityID, dayKey string) int
}

// InMemoryStore implements Store with in-memory storage.
type InMemoryStore struct {
	mu     sync.RWMutex
	drafts map[DraftID]Draft
}

// NewInMemoryStore creates a new in-memory draft store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		drafts: make(map[DraftID]Draft),
	}
}

// Put stores a draft.
func (s *InMemoryStore) Put(d Draft) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute hash if not set
	if d.DeterministicHash == "" {
		d.DeterministicHash = d.Hash()
	}

	s.drafts[d.DraftID] = d
	return nil
}

// Get retrieves a draft by ID.
func (s *InMemoryStore) Get(id DraftID) (Draft, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.drafts[id]
	return d, ok
}

// List returns drafts matching the filter, sorted deterministically.
func (s *InMemoryStore) List(filter ListFilter) []Draft {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Draft

	for _, d := range s.drafts {
		// Apply filters
		if filter.CircleID != "" && d.CircleID != filter.CircleID {
			continue
		}
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		if filter.DraftType != "" && d.DraftType != filter.DraftType {
			continue
		}
		if !filter.IncludeExpired && d.Status == StatusExpired {
			continue
		}

		result = append(result, d)
	}

	// Sort deterministically
	SortDrafts(result)

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

// UpdateStatus changes the status of a draft.
func (s *InMemoryStore) UpdateStatus(id DraftID, status DraftStatus, reason string, changedBy string, changedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.drafts[id]
	if !ok {
		return fmt.Errorf("draft not found: %s", id)
	}

	if !d.CanTransitionTo(status) {
		return fmt.Errorf("cannot transition from %s to %s", d.Status, status)
	}

	d.Status = status
	d.StatusReason = reason
	d.StatusChangedBy = changedBy
	d.StatusChangedAt = changedAt

	s.drafts[id] = d
	return nil
}

// MarkExpired marks all expired drafts as expired based on the given time.
func (s *InMemoryStore) MarkExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, d := range s.drafts {
		if d.Status == StatusProposed && d.IsExpired(now) {
			d.Status = StatusExpired
			d.StatusReason = "TTL expired"
			d.StatusChangedAt = now
			d.StatusChangedBy = "system"
			s.drafts[id] = d
			count++
		}
	}

	return count
}

// Count returns the total number of drafts.
func (s *InMemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.drafts)
}

// CountByCircleAndDay returns the count of drafts for a circle on a day.
func (s *InMemoryStore) CountByCircleAndDay(circleID identity.EntityID, dayKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, d := range s.drafts {
		if d.CircleID == circleID && DayKey(d.CreatedAt) == dayKey {
			count++
		}
	}
	return count
}

// GetByDedupKey finds a draft by its dedup key.
func (s *InMemoryStore) GetByDedupKey(dedupKey string) (Draft, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, d := range s.drafts {
		if d.DedupKey() == dedupKey {
			return d, true
		}
	}
	return Draft{}, false
}

// Delete removes a draft (used for dedup replacement).
func (s *InMemoryStore) Delete(id DraftID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, id)
}

// Clear removes all drafts.
func (s *InMemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts = make(map[DraftID]Draft)
}

// Verify interface compliance.
var _ Store = (*InMemoryStore)(nil)
