// Package persist provides persistence for Quiet Inbox Mirror summaries.
//
// Phase 22: Quiet Inbox Mirror (First Real Value Moment)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only (no raw data ever stored)
//   - Period-scoped (daily)
//   - Replay-safe
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md
package persist

import (
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/quietmirror"
)

// QuietMirrorStore stores Quiet Inbox Mirror summaries.
//
// CRITICAL: This store contains NO raw data.
// Only hashes and abstract buckets are stored.
type QuietMirrorStore struct {
	mu        sync.RWMutex
	summaries map[string]*quietmirror.QuietMirrorSummary // hash -> summary
	byCircle  map[identity.EntityID][]*quietmirror.QuietMirrorSummary
	byPeriod  map[string][]*quietmirror.QuietMirrorSummary // "circle:period" -> summaries
	clock     func() time.Time
}

// NewQuietMirrorStore creates a new store.
func NewQuietMirrorStore(clock func() time.Time) *QuietMirrorStore {
	return &QuietMirrorStore{
		summaries: make(map[string]*quietmirror.QuietMirrorSummary),
		byCircle:  make(map[identity.EntityID][]*quietmirror.QuietMirrorSummary),
		byPeriod:  make(map[string][]*quietmirror.QuietMirrorSummary),
		clock:     clock,
	}
}

// Store appends a mirror summary.
// Append-only: summaries are never modified or deleted.
func (s *QuietMirrorStore) Store(summary *quietmirror.QuietMirrorSummary) error {
	if summary == nil {
		return nil
	}

	hash := summary.Hash()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Deduplicate by hash
	if _, exists := s.summaries[hash]; exists {
		return nil
	}

	s.summaries[hash] = summary

	circleID := identity.EntityID(summary.CircleID)
	s.byCircle[circleID] = append(s.byCircle[circleID], summary)

	periodKey := summary.CircleID + ":" + summary.Period
	s.byPeriod[periodKey] = append(s.byPeriod[periodKey], summary)

	return nil
}

// GetByHash retrieves a summary by its hash.
func (s *QuietMirrorStore) GetByHash(hash string) (*quietmirror.QuietMirrorSummary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary, ok := s.summaries[hash]
	return summary, ok
}

// GetByCircle retrieves all summaries for a circle.
func (s *QuietMirrorStore) GetByCircle(circleID identity.EntityID) []*quietmirror.QuietMirrorSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byCircle[circleID]
}

// GetLatestForCircle retrieves the most recent summary for a circle.
func (s *QuietMirrorStore) GetLatestForCircle(circleID identity.EntityID) *quietmirror.QuietMirrorSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := s.byCircle[circleID]
	if len(summaries) == 0 {
		return nil
	}

	// Return the last one (most recently stored)
	return summaries[len(summaries)-1]
}

// GetForPeriod retrieves all summaries for a circle and period.
func (s *QuietMirrorStore) GetForPeriod(circleID identity.EntityID, period string) []*quietmirror.QuietMirrorSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + period
	return s.byPeriod[periodKey]
}

// GetLatestForPeriod retrieves the most recent summary for a circle and period.
func (s *QuietMirrorStore) GetLatestForPeriod(circleID identity.EntityID, period string) *quietmirror.QuietMirrorSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + period
	summaries := s.byPeriod[periodKey]
	if len(summaries) == 0 {
		return nil
	}

	return summaries[len(summaries)-1]
}

// Count returns the total number of summaries stored.
func (s *QuietMirrorStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.summaries)
}

// HasSummaryForPeriod returns true if a summary exists for the given period.
func (s *QuietMirrorStore) HasSummaryForPeriod(circleID identity.EntityID, period string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := string(circleID) + ":" + period
	summaries := s.byPeriod[periodKey]
	return len(summaries) > 0
}

// QuietMirrorDismissalStore tracks whisper cue dismissals.
//
// CRITICAL: Stores only hashes, never raw timestamps.
type QuietMirrorDismissalStore struct {
	mu         sync.RWMutex
	dismissals map[string]string // "circle:period" -> dismissal hash
	maxEntries int
	clock      func() time.Time
}

// NewQuietMirrorDismissalStore creates a new dismissal store.
func NewQuietMirrorDismissalStore(clock func() time.Time) *QuietMirrorDismissalStore {
	return &QuietMirrorDismissalStore{
		dismissals: make(map[string]string),
		maxEntries: 100,
		clock:      clock,
	}
}

// RecordDismissal records a whisper cue dismissal for a circle and period.
func (s *QuietMirrorDismissalStore) RecordDismissal(circleID identity.EntityID, period string, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := string(circleID) + ":" + period
	s.dismissals[key] = hash

	// Bounded size eviction (FIFO)
	if len(s.dismissals) > s.maxEntries {
		// Remove oldest (first key found)
		for k := range s.dismissals {
			delete(s.dismissals, k)
			break
		}
	}
}

// IsDismissed returns true if the whisper cue was dismissed for this period.
func (s *QuietMirrorDismissalStore) IsDismissed(circleID identity.EntityID, period string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + period
	_, exists := s.dismissals[key]
	return exists
}

// GetDismissalHash returns the dismissal hash for a circle and period.
func (s *QuietMirrorDismissalStore) GetDismissalHash(circleID identity.EntityID, period string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + period
	hash, exists := s.dismissals[key]
	return hash, exists
}
