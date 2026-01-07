// Package persist provides persistence for pressure map snapshots.
//
// Phase 31.4: External Pressure Circles
// Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
//
// CRITICAL INVARIANTS:
//   - Hash-only storage - NO raw amounts, NO merchant names, NO timestamps
//   - Bounded retention (30 days max)
//   - Append-only with storelog integration
//   - No goroutines. No time.Now() - clock injection only.
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/externalpressure"
	"quantumlife/pkg/domain/storelog"
)

// PressureMapStore stores pressure map snapshots.
// Thread-safe, in-memory implementation with bounded retention.
type PressureMapStore struct {
	mu sync.RWMutex

	// Snapshots keyed by "sovereignCircleIDHash:periodKey"
	snapshots map[string]*externalpressure.PressureMapSnapshot

	// Snapshots by sovereign circle for retrieval
	snapshotsBySovereign map[string][]string // sovereign hash -> keys

	// Periods for eviction tracking
	periods map[string]bool

	// Configuration
	maxPeriods int // Maximum periods to retain (30 days)
	clock      func() time.Time

	// Storelog reference for replay
	storelogRef storelog.AppendOnlyLog
}

// NewPressureMapStore creates a new pressure map store.
func NewPressureMapStore(clock func() time.Time) *PressureMapStore {
	return &PressureMapStore{
		snapshots:            make(map[string]*externalpressure.PressureMapSnapshot),
		snapshotsBySovereign: make(map[string][]string),
		periods:              make(map[string]bool),
		maxPeriods:           30,
		clock:                clock,
	}
}

// SetStorelog sets the storelog reference for replay.
func (s *PressureMapStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// PersistSnapshot stores a pressure map snapshot.
// Idempotent: same snapshot (by sovereign+period) will overwrite previous.
func (s *PressureMapStore) PersistSnapshot(snapshot *externalpressure.PressureMapSnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build key
	key := fmt.Sprintf("%s:%s", snapshot.SovereignCircleIDHash, snapshot.PeriodKey)

	// Check if this is a new key
	isNew := s.snapshots[key] == nil

	// Store snapshot (overwrites if exists)
	s.snapshots[key] = snapshot

	// Update by-sovereign index if new
	if isNew {
		s.snapshotsBySovereign[snapshot.SovereignCircleIDHash] = append(
			s.snapshotsBySovereign[snapshot.SovereignCircleIDHash],
			key,
		)
	}

	// Track period
	s.periods[snapshot.PeriodKey] = true

	// Write to storelog if available
	if s.storelogRef != nil {
		record := &storelog.LogRecord{
			Type:    storelog.RecordTypePressureMapSnapshot,
			Version: storelog.SchemaVersion,
			Payload: snapshot.CanonicalString(),
			Hash:    snapshot.StatusHash,
		}
		_ = s.storelogRef.Append(record)
	}

	// Bounded eviction
	s.evictOldPeriods()

	return nil
}

// GetSnapshot retrieves a snapshot by sovereign circle and period.
func (s *PressureMapStore) GetSnapshot(sovereignCircleIDHash, periodKey string) *externalpressure.PressureMapSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", sovereignCircleIDHash, periodKey)
	return s.snapshots[key]
}

// GetLatestSnapshot retrieves the most recent snapshot for a sovereign circle.
func (s *PressureMapStore) GetLatestSnapshot(sovereignCircleIDHash string) *externalpressure.PressureMapSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := s.snapshotsBySovereign[sovereignCircleIDHash]
	if len(keys) == 0 {
		return nil
	}

	// Find the latest period
	var latestSnapshot *externalpressure.PressureMapSnapshot
	latestPeriod := ""

	for _, key := range keys {
		if snapshot := s.snapshots[key]; snapshot != nil {
			if snapshot.PeriodKey > latestPeriod {
				latestPeriod = snapshot.PeriodKey
				latestSnapshot = snapshot
			}
		}
	}

	return latestSnapshot
}

// GetSnapshotsForSovereign retrieves all snapshots for a sovereign circle.
func (s *PressureMapStore) GetSnapshotsForSovereign(sovereignCircleIDHash string) []*externalpressure.PressureMapSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := s.snapshotsBySovereign[sovereignCircleIDHash]
	result := make([]*externalpressure.PressureMapSnapshot, 0, len(keys))

	for _, key := range keys {
		if snapshot := s.snapshots[key]; snapshot != nil {
			result = append(result, snapshot)
		}
	}

	// Sort by period for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].PeriodKey < result[j].PeriodKey
	})

	return result
}

// HasSnapshots checks if a sovereign circle has any snapshots.
func (s *PressureMapStore) HasSnapshots(sovereignCircleIDHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshotsBySovereign[sovereignCircleIDHash]) > 0
}

// Count returns the total number of snapshots.
func (s *PressureMapStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}

// evictOldPeriods removes snapshots older than maxPeriods.
func (s *PressureMapStore) evictOldPeriods() {
	if len(s.periods) <= s.maxPeriods {
		return
	}

	// Get sorted periods
	periods := make([]string, 0, len(s.periods))
	for p := range s.periods {
		periods = append(periods, p)
	}
	sort.Strings(periods)

	// Determine how many periods to remove
	toRemove := len(periods) - s.maxPeriods
	if toRemove <= 0 {
		return
	}

	// Collect periods to remove
	periodsToRemove := make(map[string]bool)
	for i := 0; i < toRemove; i++ {
		periodsToRemove[periods[i]] = true
	}

	// Remove snapshots for old periods
	var keysToRemove []string
	for key, snapshot := range s.snapshots {
		if periodsToRemove[snapshot.PeriodKey] {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		snapshot := s.snapshots[key]
		if snapshot == nil {
			continue
		}

		// Remove from snapshots
		delete(s.snapshots, key)

		// Remove from by-sovereign index
		sovHash := snapshot.SovereignCircleIDHash
		keys := s.snapshotsBySovereign[sovHash]
		for i, k := range keys {
			if k == key {
				s.snapshotsBySovereign[sovHash] = append(keys[:i], keys[i+1:]...)
				break
			}
		}
	}

	// Remove old periods
	for period := range periodsToRemove {
		delete(s.periods, period)
	}
}

// Clear removes all snapshots (for testing).
func (s *PressureMapStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[string]*externalpressure.PressureMapSnapshot)
	s.snapshotsBySovereign = make(map[string][]string)
	s.periods = make(map[string]bool)
}
