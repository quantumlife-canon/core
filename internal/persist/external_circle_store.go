// Package persist provides persistence for external derived circles.
//
// Phase 31.4: External Pressure Circles
// Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
//
// CRITICAL INVARIANTS:
//   - Hash-only storage - NO raw merchant strings, NO vendor identifiers
//   - Bounded retention (30 days max)
//   - Append-only with storelog integration
//   - No goroutines. No time.Now() - clock injection only.
//   - External circles CANNOT approve, CANNOT execute, CANNOT receive drafts.
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/externalpressure"
	"quantumlife/pkg/domain/storelog"
)

// ExternalCircleStore stores external derived circles.
// Thread-safe, in-memory implementation with bounded retention.
type ExternalCircleStore struct {
	mu sync.RWMutex

	// Circles keyed by CircleIDHash
	circles map[string]*externalpressure.ExternalDerivedCircle

	// Circles by sovereign circle for retrieval
	circlesBySovereign map[string][]string // sovereign hash -> circle ID hashes

	// Circles by period for eviction
	circlesByPeriod map[string][]string // period -> circle ID hashes

	// Configuration
	maxPeriods int // Maximum periods to retain (30 days)
	clock      func() time.Time

	// Storelog reference for replay
	storelogRef storelog.AppendOnlyLog
}

// NewExternalCircleStore creates a new external circle store.
func NewExternalCircleStore(clock func() time.Time) *ExternalCircleStore {
	return &ExternalCircleStore{
		circles:            make(map[string]*externalpressure.ExternalDerivedCircle),
		circlesBySovereign: make(map[string][]string),
		circlesByPeriod:    make(map[string][]string),
		maxPeriods:         30,
		clock:              clock,
	}
}

// SetStorelog sets the storelog reference for replay.
func (s *ExternalCircleStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// PersistCircle stores an external derived circle.
// Idempotent: same circle (by CircleIDHash) will not duplicate.
func (s *ExternalCircleStore) PersistCircle(sovereignCircleIDHash string, circle *externalpressure.ExternalDerivedCircle) error {
	if err := circle.Validate(); err != nil {
		return err
	}

	if sovereignCircleIDHash == "" {
		return fmt.Errorf("missing sovereign_circle_id_hash")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if _, exists := s.circles[circle.CircleIDHash]; exists {
		return nil // Idempotent
	}

	// Store circle
	s.circles[circle.CircleIDHash] = circle

	// Update by-sovereign index
	s.circlesBySovereign[sovereignCircleIDHash] = append(
		s.circlesBySovereign[sovereignCircleIDHash],
		circle.CircleIDHash,
	)

	// Update by-period index
	s.circlesByPeriod[circle.CreatedPeriod] = append(
		s.circlesByPeriod[circle.CreatedPeriod],
		circle.CircleIDHash,
	)

	// Write to storelog if available
	if s.storelogRef != nil {
		record := &storelog.LogRecord{
			Type:    storelog.RecordTypeExternalDerivedCircle,
			Version: storelog.SchemaVersion,
			Payload: circle.CanonicalString(),
			Hash:    circle.ComputeHash(),
		}
		_ = s.storelogRef.Append(record)
	}

	// Bounded eviction
	s.evictOldPeriods()

	return nil
}

// GetCircle retrieves a circle by its ID hash.
func (s *ExternalCircleStore) GetCircle(circleIDHash string) *externalpressure.ExternalDerivedCircle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.circles[circleIDHash]
}

// GetCirclesForSovereign retrieves all external circles for a sovereign circle.
func (s *ExternalCircleStore) GetCirclesForSovereign(sovereignCircleIDHash string) []*externalpressure.ExternalDerivedCircle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashes := s.circlesBySovereign[sovereignCircleIDHash]
	result := make([]*externalpressure.ExternalDerivedCircle, 0, len(hashes))

	for _, hash := range hashes {
		if circle := s.circles[hash]; circle != nil {
			result = append(result, circle)
		}
	}

	// Sort by CircleIDHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].CircleIDHash < result[j].CircleIDHash
	})

	return result
}

// GetCirclesForPeriod retrieves all external circles for a period.
func (s *ExternalCircleStore) GetCirclesForPeriod(period string) []*externalpressure.ExternalDerivedCircle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashes := s.circlesByPeriod[period]
	result := make([]*externalpressure.ExternalDerivedCircle, 0, len(hashes))

	for _, hash := range hashes {
		if circle := s.circles[hash]; circle != nil {
			result = append(result, circle)
		}
	}

	// Sort by CircleIDHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].CircleIDHash < result[j].CircleIDHash
	})

	return result
}

// HasCircles checks if a sovereign circle has any external circles.
func (s *ExternalCircleStore) HasCircles(sovereignCircleIDHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.circlesBySovereign[sovereignCircleIDHash]) > 0
}

// Count returns the total number of external circles.
func (s *ExternalCircleStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.circles)
}

// evictOldPeriods removes circles older than maxPeriods.
func (s *ExternalCircleStore) evictOldPeriods() {
	if len(s.circlesByPeriod) <= s.maxPeriods {
		return
	}

	// Get sorted periods
	periods := make([]string, 0, len(s.circlesByPeriod))
	for p := range s.circlesByPeriod {
		periods = append(periods, p)
	}
	sort.Strings(periods)

	// Determine how many periods to remove
	toRemove := len(periods) - s.maxPeriods
	if toRemove <= 0 {
		return
	}

	// Remove oldest periods
	for i := 0; i < toRemove; i++ {
		period := periods[i]
		hashes := s.circlesByPeriod[period]

		for _, hash := range hashes {
			if circle := s.circles[hash]; circle != nil {
				// Remove from sovereign index
				// This is inefficient but acceptable for bounded data
				for sovHash, circleHashes := range s.circlesBySovereign {
					for j, ch := range circleHashes {
						if ch == hash {
							s.circlesBySovereign[sovHash] = append(
								circleHashes[:j],
								circleHashes[j+1:]...,
							)
							break
						}
					}
				}
				// Remove circle
				delete(s.circles, hash)
			}
		}

		// Remove period index
		delete(s.circlesByPeriod, period)
	}
}

// Clear removes all circles (for testing).
func (s *ExternalCircleStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.circles = make(map[string]*externalpressure.ExternalDerivedCircle)
	s.circlesBySovereign = make(map[string][]string)
	s.circlesByPeriod = make(map[string][]string)
}
