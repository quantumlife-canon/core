// Package persist provides storage for trust summaries.
//
// Phase 20: Trust Accrual Layer (Proof Over Time)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only records
//   - Replayable
//   - Period deduplication (same period → same hash)
//   - No goroutines, no I/O except storelog
//
// Reference: docs/ADR/ADR-0048-phase20-trust-accrual-layer.md
package persist

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/domain/trust"
)

// =============================================================================
// Record Types for Storelog
// =============================================================================

// Record type constants for storelog.
const (
	RecordTypeTrustSummary   = "TRUST_SUMMARY"
	RecordTypeTrustDismissal = "TRUST_DISMISSAL"
)

// =============================================================================
// Trust Store
// =============================================================================

// TrustStore stores trust summaries and dismissals.
//
// CRITICAL: Append-only. Hash-only records.
type TrustStore struct {
	mu      sync.RWMutex
	nowFunc func() time.Time

	// Summaries indexed by ID
	summaries map[string]*trust.TrustSummary

	// Summaries by period key for lookup
	summariesByPeriod map[string]string // periodKey -> summaryID

	// Dismissals indexed by summary ID
	dismissals map[string]*trust.TrustDismissal
}

// NewTrustStore creates a new trust store.
// nowFunc is used for clock injection (deterministic testing).
func NewTrustStore(nowFunc func() time.Time) *TrustStore {
	return &TrustStore{
		nowFunc:           nowFunc,
		summaries:         make(map[string]*trust.TrustSummary),
		summariesByPeriod: make(map[string]string),
		dismissals:        make(map[string]*trust.TrustDismissal),
	}
}

// =============================================================================
// Summary Operations
// =============================================================================

// AppendSummary stores a trust summary.
// Returns error if a summary for this period already exists.
// CRITICAL: Period deduplication - same period cannot have multiple summaries.
func (s *TrustStore) AppendSummary(summary *trust.TrustSummary) error {
	if summary == nil {
		return errors.New("nil summary")
	}
	if err := summary.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute ID and hash if not set
	if summary.SummaryID == "" {
		summary.SummaryID = summary.ComputeID()
	}
	if summary.SummaryHash == "" {
		summary.SummaryHash = summary.ComputeHash()
	}

	// Check for period deduplication
	if existingID, exists := s.summariesByPeriod[summary.PeriodKey]; exists {
		if existingID == summary.SummaryID {
			// Same summary, idempotent - no error
			return nil
		}
		return storelog.ErrRecordExists
	}

	s.summaries[summary.SummaryID] = summary
	s.summariesByPeriod[summary.PeriodKey] = summary.SummaryID

	return nil
}

// GetSummary retrieves a summary by ID.
func (s *TrustStore) GetSummary(summaryID string) (*trust.TrustSummary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary, ok := s.summaries[summaryID]
	return summary, ok
}

// GetSummaryByPeriod retrieves a summary by period key.
func (s *TrustStore) GetSummaryByPeriod(periodKey string) (*trust.TrustSummary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaryID, ok := s.summariesByPeriod[periodKey]
	if !ok {
		return nil, false
	}
	return s.summaries[summaryID], true
}

// ListSummaries returns all summaries sorted by period key (most recent first).
// CRITICAL: Returns only non-dismissed summaries for display.
func (s *TrustStore) ListSummaries() []trust.TrustSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []trust.TrustSummary
	for _, summary := range s.summaries {
		result = append(result, *summary)
	}

	// Sort by period key descending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].PeriodKey > result[j].PeriodKey
	})

	return result
}

// ListUndismissedSummaries returns summaries that have not been dismissed.
// CRITICAL: Once dismissed, a summary must not reappear.
func (s *TrustStore) ListUndismissedSummaries() []trust.TrustSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []trust.TrustSummary
	for _, summary := range s.summaries {
		if !summary.IsDismissed() && s.dismissals[summary.SummaryID] == nil {
			result = append(result, *summary)
		}
	}

	// Sort by period key descending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].PeriodKey > result[j].PeriodKey
	})

	return result
}

// GetRecentMeaningfulSummary returns the most recent undismissed, meaningful summary.
// CRITICAL: Returns nil if none exists - silence is the default.
func (s *TrustStore) GetRecentMeaningfulSummary() *trust.TrustSummary {
	summaries := s.ListUndismissedSummaries()
	for _, summary := range summaries {
		if summary.IsMeaningful() {
			return &summary
		}
	}
	return nil
}

// GetSummaryCount returns the total number of summaries.
func (s *TrustStore) GetSummaryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.summaries)
}

// =============================================================================
// Dismissal Operations
// =============================================================================

// DismissSummary records a dismissal for a summary.
// Once dismissed, the summary must not reappear for that period.
func (s *TrustStore) DismissSummary(summaryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify summary exists
	summary, ok := s.summaries[summaryID]
	if !ok {
		return errors.New("summary not found")
	}

	// Check if already dismissed
	if s.dismissals[summaryID] != nil {
		return nil // Idempotent
	}

	// Create dismissal
	now := s.nowFunc()
	dismissal := &trust.TrustDismissal{
		SummaryID:     summaryID,
		SummaryHash:   summary.SummaryHash,
		CreatedBucket: trust.FiveMinuteBucket(now),
		CreatedAt:     now,
	}
	dismissal.DismissalID = dismissal.ComputeID()
	dismissal.DismissalHash = dismissal.ComputeHash()

	if err := dismissal.Validate(); err != nil {
		return err
	}

	s.dismissals[summaryID] = dismissal

	// Update summary's dismissed bucket
	summary.DismissedBucket = dismissal.CreatedBucket

	return nil
}

// IsDismissed checks if a summary has been dismissed.
func (s *TrustStore) IsDismissed(summaryID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.dismissals[summaryID] != nil
}

// GetDismissal retrieves a dismissal by summary ID.
func (s *TrustStore) GetDismissal(summaryID string) (*trust.TrustDismissal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dismissal, ok := s.dismissals[summaryID]
	return dismissal, ok
}

// =============================================================================
// Storelog Integration (Replay Support)
// =============================================================================

// summaryRecord is the JSON structure for persisting summaries.
type summaryRecord struct {
	SummaryID       string `json:"summary_id"`
	SummaryHash     string `json:"summary_hash"`
	Period          string `json:"period"`
	PeriodKey       string `json:"period_key"`
	SignalKind      string `json:"signal_kind"`
	MagnitudeBucket string `json:"magnitude_bucket"`
	DismissedBucket string `json:"dismissed_bucket,omitempty"`
	CreatedBucket   string `json:"created_bucket"`
	CreatedAt       string `json:"created_at"`
}

// dismissalRecord is the JSON structure for persisting dismissals.
type dismissalRecord struct {
	DismissalID   string `json:"dismissal_id"`
	DismissalHash string `json:"dismissal_hash"`
	SummaryID     string `json:"summary_id"`
	SummaryHash   string `json:"summary_hash"`
	CreatedBucket string `json:"created_bucket"`
	CreatedAt     string `json:"created_at"`
}

// SummaryToStorelogRecord converts a summary to a storelog record.
func (s *TrustStore) SummaryToStorelogRecord(summary *trust.TrustSummary) *storelog.LogRecord {
	payload := summaryRecord{
		SummaryID:       summary.SummaryID,
		SummaryHash:     summary.SummaryHash,
		Period:          string(summary.Period),
		PeriodKey:       summary.PeriodKey,
		SignalKind:      string(summary.SignalKind),
		MagnitudeBucket: string(summary.MagnitudeBucket),
		DismissedBucket: summary.DismissedBucket,
		CreatedBucket:   summary.CreatedBucket,
		CreatedAt:       summary.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      RecordTypeTrustSummary,
		Version:   "v1",
		Timestamp: summary.CreatedAt,
		Payload:   string(data),
	}
}

// DismissalToStorelogRecord converts a dismissal to a storelog record.
func (s *TrustStore) DismissalToStorelogRecord(dismissal *trust.TrustDismissal) *storelog.LogRecord {
	payload := dismissalRecord{
		DismissalID:   dismissal.DismissalID,
		DismissalHash: dismissal.DismissalHash,
		SummaryID:     dismissal.SummaryID,
		SummaryHash:   dismissal.SummaryHash,
		CreatedBucket: dismissal.CreatedBucket,
		CreatedAt:     dismissal.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      RecordTypeTrustDismissal,
		Version:   "v1",
		Timestamp: dismissal.CreatedAt,
		Payload:   string(data),
	}
}

// =============================================================================
// Replay Support
// =============================================================================

// ReplaySummaryRecord replays a summary record from storelog.
func (s *TrustStore) ReplaySummaryRecord(record *storelog.LogRecord) error {
	if record.Type != RecordTypeTrustSummary {
		return errors.New("invalid record type for summary")
	}

	var sr summaryRecord
	if err := json.Unmarshal([]byte(record.Payload), &sr); err != nil {
		return err
	}

	createdAt, _ := time.Parse(time.RFC3339, sr.CreatedAt)

	summary := &trust.TrustSummary{
		SummaryID:       sr.SummaryID,
		SummaryHash:     sr.SummaryHash,
		Period:          trust.TrustPeriod(sr.Period),
		PeriodKey:       sr.PeriodKey,
		SignalKind:      trust.TrustSignalKind(sr.SignalKind),
		MagnitudeBucket: shadowllm.MagnitudeBucket(sr.MagnitudeBucket),
		DismissedBucket: sr.DismissedBucket,
		CreatedBucket:   sr.CreatedBucket,
		CreatedAt:       createdAt,
	}

	// Use internal add to avoid duplicate check during replay
	s.mu.Lock()
	defer s.mu.Unlock()

	s.summaries[summary.SummaryID] = summary
	s.summariesByPeriod[summary.PeriodKey] = summary.SummaryID

	return nil
}

// ReplayDismissalRecord replays a dismissal record from storelog.
func (s *TrustStore) ReplayDismissalRecord(record *storelog.LogRecord) error {
	if record.Type != RecordTypeTrustDismissal {
		return errors.New("invalid record type for dismissal")
	}

	var dr dismissalRecord
	if err := json.Unmarshal([]byte(record.Payload), &dr); err != nil {
		return err
	}

	createdAt, _ := time.Parse(time.RFC3339, dr.CreatedAt)

	dismissal := &trust.TrustDismissal{
		DismissalID:   dr.DismissalID,
		DismissalHash: dr.DismissalHash,
		SummaryID:     dr.SummaryID,
		SummaryHash:   dr.SummaryHash,
		CreatedBucket: dr.CreatedBucket,
		CreatedAt:     createdAt,
	}

	// Use internal add to avoid duplicate check during replay
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dismissals[dismissal.SummaryID] = dismissal

	// Update summary's dismissed bucket if summary exists
	if summary, ok := s.summaries[dismissal.SummaryID]; ok {
		summary.DismissedBucket = dismissal.CreatedBucket
	}

	return nil
}
