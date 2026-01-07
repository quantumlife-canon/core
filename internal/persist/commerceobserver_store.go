// Package persist provides persistence for commerce observations.
//
// Phase 31: Commerce Observers (Silent by Default)
// Reference: docs/ADR/ADR-0062-phase31-commerce-observers.md
//
// CRITICAL INVARIANTS:
//   - Hash-only storage - NO raw amounts, NO merchant names, NO timestamps
//   - Bounded retention (30 days max)
//   - Append-only with storelog integration
//   - No goroutines. No time.Now() - clock injection only.
//
// This phase is OBSERVATION ONLY. Commerce is observed. Nothing else.
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/commerceobserver"
	"quantumlife/pkg/domain/storelog"
)

// CommerceObserverStore stores commerce observations and acknowledgments.
// Thread-safe, in-memory implementation with bounded retention.
type CommerceObserverStore struct {
	mu sync.RWMutex

	// Observations keyed by "circleID:period:category"
	observations map[string]*commerceobserver.CommerceObservation

	// Observations by circle for retrieval
	observationsByCircle map[string][]string // circleID -> keys

	// Observations by period for retrieval
	observationsByPeriod map[string][]string // "circleID:period" -> keys

	// Acknowledgments (hash-only)
	acks map[string]bool // hash -> true

	// Configuration
	maxPeriods int // Maximum periods to retain (30 days)
	clock      func() time.Time

	// Storelog reference for replay
	storelogRef storelog.AppendOnlyLog
}

// NewCommerceObserverStore creates a new commerce observer store.
func NewCommerceObserverStore(clock func() time.Time) *CommerceObserverStore {
	return &CommerceObserverStore{
		observations:         make(map[string]*commerceobserver.CommerceObservation),
		observationsByCircle: make(map[string][]string),
		observationsByPeriod: make(map[string][]string),
		acks:                 make(map[string]bool),
		maxPeriods:           30,
		clock:                clock,
	}
}

// SetStorelog sets the storelog reference for replay.
func (s *CommerceObserverStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// PersistObservation stores a commerce observation.
// Idempotent: same observation (by key) will not duplicate.
func (s *CommerceObserverStore) PersistObservation(circleID string, obs *commerceobserver.CommerceObservation) error {
	if err := obs.Validate(); err != nil {
		return err
	}

	if circleID == "" {
		return fmt.Errorf("missing circle_id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build key: "circleID:period:category"
	key := fmt.Sprintf("%s:%s:%s", circleID, obs.Period, obs.Category)

	// Check for duplicate
	if _, exists := s.observations[key]; exists {
		return nil // Idempotent
	}

	// Store observation
	s.observations[key] = obs

	// Update by-circle index
	s.observationsByCircle[circleID] = append(
		s.observationsByCircle[circleID],
		key,
	)

	// Update by-period index
	periodKey := fmt.Sprintf("%s:%s", circleID, obs.Period)
	s.observationsByPeriod[periodKey] = append(
		s.observationsByPeriod[periodKey],
		key,
	)

	// Write to storelog if available
	if s.storelogRef != nil {
		record := &storelog.LogRecord{
			Type:    storelog.RecordTypeCommerceObservation,
			Version: storelog.SchemaVersion,
			Payload: obs.CanonicalString(),
			Hash:    obs.ComputeHash(),
		}
		_ = s.storelogRef.Append(record)
	}

	// Bounded eviction
	s.evictOldPeriods()

	return nil
}

// GetObservationsForPeriod retrieves all observations for a circle and period.
func (s *CommerceObserverStore) GetObservationsForPeriod(circleID, period string) []commerceobserver.CommerceObservation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := fmt.Sprintf("%s:%s", circleID, period)
	keys := s.observationsByPeriod[periodKey]

	result := make([]commerceobserver.CommerceObservation, 0, len(keys))
	for _, key := range keys {
		if obs := s.observations[key]; obs != nil {
			result = append(result, *obs)
		}
	}

	// Sort by category for determinism
	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Category) < string(result[j].Category)
	})

	return result
}

// GetAllObservationsForCircle retrieves all observations for a circle.
func (s *CommerceObserverStore) GetAllObservationsForCircle(circleID string) []commerceobserver.CommerceObservation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := s.observationsByCircle[circleID]

	result := make([]commerceobserver.CommerceObservation, 0, len(keys))
	for _, key := range keys {
		if obs := s.observations[key]; obs != nil {
			result = append(result, *obs)
		}
	}

	// Sort by period then category for determinism
	sort.Slice(result, func(i, j int) bool {
		if result[i].Period != result[j].Period {
			return result[i].Period < result[j].Period
		}
		return string(result[i].Category) < string(result[j].Category)
	})

	return result
}

// GetLatestObservations retrieves the most recent observations for a circle.
func (s *CommerceObserverStore) GetLatestObservations(circleID string) []commerceobserver.CommerceObservation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := s.observationsByCircle[circleID]
	if len(keys) == 0 {
		return nil
	}

	// Find the latest period
	latestPeriod := ""
	for _, key := range keys {
		if obs := s.observations[key]; obs != nil {
			if obs.Period > latestPeriod {
				latestPeriod = obs.Period
			}
		}
	}

	if latestPeriod == "" {
		return nil
	}

	// Get all observations for the latest period
	result := make([]commerceobserver.CommerceObservation, 0)
	for _, key := range keys {
		if obs := s.observations[key]; obs != nil && obs.Period == latestPeriod {
			result = append(result, *obs)
		}
	}

	// Sort by category for determinism
	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Category) < string(result[j].Category)
	})

	return result
}

// RecordAck records an acknowledgment (hash-only).
func (s *CommerceObserverStore) RecordAck(hash string) error {
	if hash == "" {
		return fmt.Errorf("missing hash")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.acks[hash] = true
	return nil
}

// IsAcked checks if a hash has been acknowledged.
func (s *CommerceObserverStore) IsAcked(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.acks[hash]
}

// HasObservations checks if a circle has any observations.
func (s *CommerceObserverStore) HasObservations(circleID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.observationsByCircle[circleID]) > 0
}

// Count returns the total number of observations.
func (s *CommerceObserverStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.observations)
}

// evictOldPeriods removes observations older than maxPeriods.
// Called automatically on each persist.
func (s *CommerceObserverStore) evictOldPeriods() {
	// Count unique periods per circle
	periodCount := make(map[string]int)
	for _, obs := range s.observations {
		periodCount[obs.Period]++
	}

	// If we have too many periods, remove the oldest
	if len(periodCount) <= s.maxPeriods {
		return
	}

	// Get sorted periods
	periods := make([]string, 0, len(periodCount))
	for p := range periodCount {
		periods = append(periods, p)
	}
	sort.Strings(periods)

	// Determine how many periods to remove
	toRemove := len(periods) - s.maxPeriods
	if toRemove <= 0 {
		return
	}

	// Remove oldest periods
	periodsToRemove := make(map[string]bool)
	for i := 0; i < toRemove; i++ {
		periodsToRemove[periods[i]] = true
	}

	// Remove observations for old periods
	var keysToRemove []string
	for key, obs := range s.observations {
		if periodsToRemove[obs.Period] {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		obs := s.observations[key]
		if obs == nil {
			continue
		}

		// Remove from observations
		delete(s.observations, key)

		// Remove from by-period index
		periodKey := fmt.Sprintf(":%s", obs.Period)
		for circleID := range s.observationsByCircle {
			pk := fmt.Sprintf("%s%s", circleID, periodKey)
			if keys := s.observationsByPeriod[pk]; keys != nil {
				for i, k := range keys {
					if k == key {
						s.observationsByPeriod[pk] = append(keys[:i], keys[i+1:]...)
						break
					}
				}
			}
		}

		// Remove from by-circle index
		for circleID := range s.observationsByCircle {
			keys := s.observationsByCircle[circleID]
			for i, k := range keys {
				if k == key {
					s.observationsByCircle[circleID] = append(keys[:i], keys[i+1:]...)
					break
				}
			}
		}
	}
}

// ExpireOldObservations explicitly removes old observations.
// Can be called for cleanup if needed.
func (s *CommerceObserverStore) ExpireOldObservations() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictOldPeriods()
}

// Clear removes all observations (for testing).
func (s *CommerceObserverStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = make(map[string]*commerceobserver.CommerceObservation)
	s.observationsByCircle = make(map[string][]string)
	s.observationsByPeriod = make(map[string][]string)
	s.acks = make(map[string]bool)
}
