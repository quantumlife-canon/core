// Package persist provides persistence for journey dismissals.
//
// Phase 26A: Guided Journey (Product/UX)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only (no raw data, no timestamps, no identifiers)
//   - Bounded retention (last N periods)
//   - Period-scoped (daily)
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md
package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// JourneyDismissalStore stores journey dismissal records.
//
// CRITICAL: This store contains NO raw data.
// Only hashes, period keys, and circle IDs are stored.
type JourneyDismissalStore struct {
	mu          sync.RWMutex
	dismissals  map[string]*journeyDismissalRecord // "circle:period" -> record
	byHash      map[string]*journeyDismissalRecord // dismissal hash -> record
	maxPeriods  int
	clock       func() time.Time
	storelogRef storelog.AppendOnlyLog
}

// journeyDismissalRecord is the internal representation.
type journeyDismissalRecord struct {
	CircleID       string `json:"circle_id"`
	PeriodKey      string `json:"period_key"`
	StatusHash     string `json:"status_hash"`
	DismissalHash  string `json:"dismissal_hash"`
	TimeBucketUnix int64  `json:"time_bucket_unix"`
}

// NewJourneyDismissalStore creates a new journey dismissal store.
func NewJourneyDismissalStore(clock func() time.Time) *JourneyDismissalStore {
	return &JourneyDismissalStore{
		dismissals: make(map[string]*journeyDismissalRecord),
		byHash:     make(map[string]*journeyDismissalRecord),
		maxPeriods: 30, // Keep last 30 days
		clock:      clock,
	}
}

// SetStorelog sets the storelog reference for persistence.
func (s *JourneyDismissalStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// RecordDismissal records a journey dismissal.
// Returns the dismissal hash.
func (s *JourneyDismissalStore) RecordDismissal(
	circleID identity.EntityID,
	periodKey string,
	statusHash string,
) (string, error) {
	now := s.clock()
	timeBucket := now.Truncate(5 * time.Minute)

	// Compute dismissal hash
	dismissalHash := computeJourneyDismissalHash(string(circleID), periodKey, statusHash, timeBucket)

	record := &journeyDismissalRecord{
		CircleID:       string(circleID),
		PeriodKey:      periodKey,
		StatusHash:     statusHash,
		DismissalHash:  dismissalHash,
		TimeBucketUnix: timeBucket.Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store by period key
	key := string(circleID) + ":" + periodKey
	s.dismissals[key] = record
	s.byHash[dismissalHash] = record

	// Bounded eviction
	s.evictOldPeriods()

	// Persist to storelog if available
	if s.storelogRef != nil {
		s.persistToStorelog(record)
	}

	return dismissalHash, nil
}

// IsDismissedForPeriod returns true if the journey was dismissed for this period.
func (s *JourneyDismissalStore) IsDismissedForPeriod(circleID identity.EntityID, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + periodKey
	_, exists := s.dismissals[key]
	return exists
}

// GetDismissedStatusHash returns the status hash of the dismissed journey for this period.
// Returns empty string if not dismissed.
func (s *JourneyDismissalStore) GetDismissedStatusHash(circleID identity.EntityID, periodKey string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + periodKey
	record, exists := s.dismissals[key]
	if !exists {
		return ""
	}
	return record.StatusHash
}

// GetByHash retrieves a dismissal record by hash.
func (s *JourneyDismissalStore) GetByHash(dismissalHash string) (*journeyDismissalRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.byHash[dismissalHash]
	return record, exists
}

// evictOldPeriods removes dismissals older than maxPeriods.
// Must be called with lock held.
func (s *JourneyDismissalStore) evictOldPeriods() {
	if len(s.dismissals) <= s.maxPeriods {
		return
	}

	// Find oldest entries and remove
	// Simple FIFO eviction
	count := len(s.dismissals) - s.maxPeriods
	for key, record := range s.dismissals {
		if count <= 0 {
			break
		}
		delete(s.dismissals, key)
		delete(s.byHash, record.DismissalHash)
		count--
	}
}

// persistToStorelog writes the record to the storelog.
func (s *JourneyDismissalStore) persistToStorelog(record *journeyDismissalRecord) {
	if s.storelogRef == nil {
		return
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return // Silent fail - in-memory state is still valid
	}

	logRecord := storelog.NewRecord(
		storelog.RecordTypeJourneyDismissal,
		time.Unix(record.TimeBucketUnix, 0),
		identity.EntityID(record.CircleID),
		string(payload),
	)
	_ = s.storelogRef.Append(logRecord)
}

// ReplayFromStorelog replays dismissal records from the storelog.
func (s *JourneyDismissalStore) ReplayFromStorelog(log storelog.AppendOnlyLog) error {
	records, err := log.ListByType(storelog.RecordTypeJourneyDismissal)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, logRecord := range records {
		var record journeyDismissalRecord
		if err := json.Unmarshal([]byte(logRecord.Payload), &record); err != nil {
			continue // Skip invalid records
		}

		key := record.CircleID + ":" + record.PeriodKey
		s.dismissals[key] = &record
		s.byHash[record.DismissalHash] = &record
	}

	return nil
}

// Count returns the total number of dismissals stored.
func (s *JourneyDismissalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.dismissals)
}

// computeJourneyDismissalHash computes a deterministic hash.
func computeJourneyDismissalHash(circleID, periodKey, statusHash string, timeBucket time.Time) string {
	canonical := "JOURNEY_DISMISS|v1|" + circleID + "|" + periodKey + "|" + statusHash + "|" + timeBucket.UTC().Format(time.RFC3339)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}
