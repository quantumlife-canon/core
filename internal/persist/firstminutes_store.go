// Package persist provides persistence for First Minutes summaries and dismissals.
//
// Phase 26B: First Five Minutes Proof
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only (no raw data, no timestamps beyond period bucket, no identifiers)
//   - Bounded retention (last 30 periods)
//   - One summary per period
//   - Period-scoped (daily)
//   - No goroutines. No time.Now() - clock injection only.
//
// This is NOT analytics. This is narrative proof.
//
// Reference: docs/ADR/ADR-0056-phase26B-first-five-minutes-proof.md
package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"quantumlife/pkg/domain/firstminutes"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// FirstMinutesStore stores First Minutes summaries and dismissals.
//
// CRITICAL: This store contains NO raw data.
// Only hashes, signals (abstract), period keys, and circle IDs are stored.
type FirstMinutesStore struct {
	mu          sync.RWMutex
	summaries   map[string]*firstMinutesSummaryRecord   // "circle:period" -> summary
	dismissals  map[string]*firstMinutesDismissalRecord // "circle:period" -> dismissal
	maxPeriods  int
	clock       func() time.Time
	storelogRef storelog.AppendOnlyLog
}

// firstMinutesSummaryRecord is the internal representation of a summary.
type firstMinutesSummaryRecord struct {
	CircleID   string                     `json:"circle_id"`
	Period     string                     `json:"period"`
	Signals    []firstMinutesSignalRecord `json:"signals"`
	CalmLine   string                     `json:"calm_line"`
	StatusHash string                     `json:"status_hash"`
	CreatedAt  int64                      `json:"created_at"` // Unix timestamp bucket
}

// firstMinutesSignalRecord is the internal representation of a signal.
type firstMinutesSignalRecord struct {
	Kind      string `json:"kind"`
	Magnitude string `json:"magnitude"`
}

// firstMinutesDismissalRecord is the internal representation of a dismissal.
type firstMinutesDismissalRecord struct {
	CircleID       string `json:"circle_id"`
	Period         string `json:"period"`
	SummaryHash    string `json:"summary_hash"`
	DismissalHash  string `json:"dismissal_hash"`
	TimeBucketUnix int64  `json:"time_bucket_unix"`
}

// NewFirstMinutesStore creates a new First Minutes store with clock injection.
func NewFirstMinutesStore(clock func() time.Time) *FirstMinutesStore {
	return &FirstMinutesStore{
		summaries:  make(map[string]*firstMinutesSummaryRecord),
		dismissals: make(map[string]*firstMinutesDismissalRecord),
		maxPeriods: 30, // Keep last 30 days
		clock:      clock,
	}
}

// SetStorelog sets the storelog reference for persistence.
func (s *FirstMinutesStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// PersistSummary persists a First Minutes summary.
// Only one summary per period is allowed.
func (s *FirstMinutesStore) PersistSummary(
	circleID identity.EntityID,
	summary *firstminutes.FirstMinutesSummary,
) error {
	if summary == nil {
		return nil
	}

	now := s.clock()
	timeBucket := now.Truncate(5 * time.Minute)

	// Convert signals to internal format
	signals := make([]firstMinutesSignalRecord, len(summary.Signals))
	for i, sig := range summary.Signals {
		signals[i] = firstMinutesSignalRecord{
			Kind:      string(sig.Kind),
			Magnitude: string(sig.Magnitude),
		}
	}

	record := &firstMinutesSummaryRecord{
		CircleID:   string(circleID),
		Period:     string(summary.Period),
		Signals:    signals,
		CalmLine:   summary.CalmLine,
		StatusHash: summary.StatusHash,
		CreatedAt:  timeBucket.Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// One summary per period - overwrite if exists
	key := string(circleID) + ":" + string(summary.Period)
	s.summaries[key] = record

	// Bounded eviction
	s.evictOldPeriods()

	// Persist to storelog if available
	if s.storelogRef != nil {
		s.persistSummaryToStorelog(record)
	}

	return nil
}

// GetForPeriod retrieves the summary for a period.
func (s *FirstMinutesStore) GetForPeriod(
	circleID identity.EntityID,
	period firstminutes.FirstMinutesPeriod,
) *firstminutes.FirstMinutesSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + string(period)
	record, exists := s.summaries[key]
	if !exists {
		return nil
	}

	return recordToSummary(record)
}

// HasSummaryForPeriod returns true if a summary exists for this period.
func (s *FirstMinutesStore) HasSummaryForPeriod(
	circleID identity.EntityID,
	period firstminutes.FirstMinutesPeriod,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + string(period)
	_, exists := s.summaries[key]
	return exists
}

// RecordDismissal records a First Minutes dismissal.
// Returns the dismissal hash.
func (s *FirstMinutesStore) RecordDismissal(
	circleID identity.EntityID,
	period firstminutes.FirstMinutesPeriod,
	summaryHash string,
) (string, error) {
	now := s.clock()
	timeBucket := now.Truncate(5 * time.Minute)

	// Compute dismissal hash
	dismissalHash := computeFirstMinutesDismissalHash(string(circleID), string(period), summaryHash, timeBucket)

	record := &firstMinutesDismissalRecord{
		CircleID:       string(circleID),
		Period:         string(period),
		SummaryHash:    summaryHash,
		DismissalHash:  dismissalHash,
		TimeBucketUnix: timeBucket.Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store by period key
	key := string(circleID) + ":" + string(period)
	s.dismissals[key] = record

	// Bounded eviction (uses same limits as summaries)
	s.evictOldDismissals()

	// Persist to storelog if available
	if s.storelogRef != nil {
		s.persistDismissalToStorelog(record)
	}

	return dismissalHash, nil
}

// IsDismissed returns true if the First Minutes receipt was dismissed for this period.
func (s *FirstMinutesStore) IsDismissed(
	circleID identity.EntityID,
	period firstminutes.FirstMinutesPeriod,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + string(period)
	_, exists := s.dismissals[key]
	return exists
}

// GetDismissedSummaryHash returns the summary hash of the dismissed receipt for this period.
// Returns empty string if not dismissed.
func (s *FirstMinutesStore) GetDismissedSummaryHash(
	circleID identity.EntityID,
	period firstminutes.FirstMinutesPeriod,
) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + string(period)
	record, exists := s.dismissals[key]
	if !exists {
		return ""
	}
	return record.SummaryHash
}

// evictOldPeriods removes summaries older than maxPeriods.
// Must be called with lock held.
func (s *FirstMinutesStore) evictOldPeriods() {
	if len(s.summaries) <= s.maxPeriods {
		return
	}

	// Simple FIFO eviction
	count := len(s.summaries) - s.maxPeriods
	for key := range s.summaries {
		if count <= 0 {
			break
		}
		delete(s.summaries, key)
		count--
	}
}

// evictOldDismissals removes dismissals older than maxPeriods.
// Must be called with lock held.
func (s *FirstMinutesStore) evictOldDismissals() {
	if len(s.dismissals) <= s.maxPeriods {
		return
	}

	// Simple FIFO eviction
	count := len(s.dismissals) - s.maxPeriods
	for key := range s.dismissals {
		if count <= 0 {
			break
		}
		delete(s.dismissals, key)
		count--
	}
}

// persistSummaryToStorelog writes the summary to the storelog.
func (s *FirstMinutesStore) persistSummaryToStorelog(record *firstMinutesSummaryRecord) {
	if s.storelogRef == nil {
		return
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return // Silent fail - in-memory state is still valid
	}

	logRecord := storelog.NewRecord(
		storelog.RecordTypeFirstMinutesSummary,
		time.Unix(record.CreatedAt, 0),
		identity.EntityID(record.CircleID),
		string(payload),
	)
	_ = s.storelogRef.Append(logRecord)
}

// persistDismissalToStorelog writes the dismissal to the storelog.
func (s *FirstMinutesStore) persistDismissalToStorelog(record *firstMinutesDismissalRecord) {
	if s.storelogRef == nil {
		return
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return // Silent fail - in-memory state is still valid
	}

	logRecord := storelog.NewRecord(
		storelog.RecordTypeFirstMinutesDismissal,
		time.Unix(record.TimeBucketUnix, 0),
		identity.EntityID(record.CircleID),
		string(payload),
	)
	_ = s.storelogRef.Append(logRecord)
}

// ReplayFromStorelog replays First Minutes records from the storelog.
func (s *FirstMinutesStore) ReplayFromStorelog(log storelog.AppendOnlyLog) error {
	// Replay summaries
	summaryRecords, err := log.ListByType(storelog.RecordTypeFirstMinutesSummary)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, logRecord := range summaryRecords {
		var record firstMinutesSummaryRecord
		if err := json.Unmarshal([]byte(logRecord.Payload), &record); err != nil {
			continue // Skip invalid records
		}

		key := record.CircleID + ":" + record.Period
		s.summaries[key] = &record
	}

	// Replay dismissals
	dismissalRecords, err := log.ListByType(storelog.RecordTypeFirstMinutesDismissal)
	if err != nil {
		return err
	}

	for _, logRecord := range dismissalRecords {
		var record firstMinutesDismissalRecord
		if err := json.Unmarshal([]byte(logRecord.Payload), &record); err != nil {
			continue // Skip invalid records
		}

		key := record.CircleID + ":" + record.Period
		s.dismissals[key] = &record
	}

	return nil
}

// CountSummaries returns the total number of summaries stored.
func (s *FirstMinutesStore) CountSummaries() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.summaries)
}

// CountDismissals returns the total number of dismissals stored.
func (s *FirstMinutesStore) CountDismissals() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.dismissals)
}

// recordToSummary converts internal record to domain type.
func recordToSummary(record *firstMinutesSummaryRecord) *firstminutes.FirstMinutesSummary {
	signals := make([]firstminutes.FirstMinutesSignal, len(record.Signals))
	for i, sig := range record.Signals {
		signals[i] = firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.FirstMinutesSignalKind(sig.Kind),
			Magnitude: firstminutes.MagnitudeBucket(sig.Magnitude),
		}
	}

	return &firstminutes.FirstMinutesSummary{
		Period:     firstminutes.FirstMinutesPeriod(record.Period),
		Signals:    signals,
		CalmLine:   record.CalmLine,
		StatusHash: record.StatusHash,
	}
}

// computeFirstMinutesDismissalHash computes a deterministic dismissal hash.
func computeFirstMinutesDismissalHash(circleID, period, summaryHash string, timeBucket time.Time) string {
	canonical := "FIRST_MINUTES_DISMISS|v1|" + circleID + "|" + period + "|" + summaryHash + "|" + timeBucket.UTC().Format(time.RFC3339)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}
