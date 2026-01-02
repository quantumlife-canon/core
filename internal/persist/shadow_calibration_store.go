// Package persist provides storage for shadow calibration records.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only persistence (no raw content)
//   - Replayable (Phase 12 compliant)
//   - No goroutines, no I/O except storelog
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package persist

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/shadowdiff"
	"quantumlife/pkg/domain/storelog"
)

// =============================================================================
// Shadow Calibration Store
// =============================================================================

// ShadowCalibrationStore stores shadow diff and calibration records.
//
// CRITICAL: Append-only, hash-only. Never stores raw content.
type ShadowCalibrationStore struct {
	mu      sync.RWMutex
	nowFunc func() time.Time

	// Diff results indexed by DiffID
	diffs map[string]*shadowdiff.DiffResult

	// Calibration records indexed by RecordID
	calibrations map[string]*shadowdiff.CalibrationRecord

	// Diffs by period bucket for aggregation
	diffsByPeriod map[string][]string // period -> []diffID

	// Votes by DiffID
	votesByDiff map[string]shadowdiff.CalibrationVote
}

// NewShadowCalibrationStore creates a new calibration store.
// nowFunc is used for clock injection (deterministic testing).
func NewShadowCalibrationStore(nowFunc func() time.Time) *ShadowCalibrationStore {
	return &ShadowCalibrationStore{
		nowFunc:       nowFunc,
		diffs:         make(map[string]*shadowdiff.DiffResult),
		calibrations:  make(map[string]*shadowdiff.CalibrationRecord),
		diffsByPeriod: make(map[string][]string),
		votesByDiff:   make(map[string]shadowdiff.CalibrationVote),
	}
}

// =============================================================================
// Diff Result Operations
// =============================================================================

// AppendDiff stores a diff result.
// Returns error if the diff already exists.
func (s *ShadowCalibrationStore) AppendDiff(diff *shadowdiff.DiffResult) error {
	if diff == nil {
		return errors.New("nil diff result")
	}
	if err := diff.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.diffs[diff.DiffID]; exists {
		return storelog.ErrRecordExists
	}

	// Store the diff
	s.diffs[diff.DiffID] = diff

	// Index by period
	s.diffsByPeriod[diff.PeriodBucket] = append(s.diffsByPeriod[diff.PeriodBucket], diff.DiffID)

	return nil
}

// GetDiff retrieves a diff by ID.
func (s *ShadowCalibrationStore) GetDiff(diffID string) (*shadowdiff.DiffResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	diff, ok := s.diffs[diffID]
	return diff, ok
}

// ListDiffsByPeriod returns all diffs for a given period bucket.
// Results are sorted by DiffID for determinism.
func (s *ShadowCalibrationStore) ListDiffsByPeriod(periodBucket string) []*shadowdiff.DiffResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	diffIDs := s.diffsByPeriod[periodBucket]
	if len(diffIDs) == 0 {
		return nil
	}

	// Sort for determinism
	sorted := make([]string, len(diffIDs))
	copy(sorted, diffIDs)
	sort.Strings(sorted)

	results := make([]*shadowdiff.DiffResult, 0, len(sorted))
	for _, id := range sorted {
		if diff, ok := s.diffs[id]; ok {
			results = append(results, diff)
		}
	}

	return results
}

// VerifyDiffHash verifies that a stored diff has the expected hash.
func (s *ShadowCalibrationStore) VerifyDiffHash(diffID, expectedHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	diff, ok := s.diffs[diffID]
	if !ok {
		return false
	}
	return diff.Hash() == expectedHash
}

// =============================================================================
// Calibration Record Operations
// =============================================================================

// AppendCalibration stores a calibration vote.
// Returns error if the record already exists.
func (s *ShadowCalibrationStore) AppendCalibration(record *shadowdiff.CalibrationRecord) error {
	if record == nil {
		return errors.New("nil calibration record")
	}
	if err := record.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.calibrations[record.RecordID]; exists {
		return storelog.ErrRecordExists
	}

	// Store the record
	s.calibrations[record.RecordID] = record

	// Index vote by diff
	s.votesByDiff[record.DiffID] = record.Vote

	return nil
}

// GetCalibration retrieves a calibration record by ID.
func (s *ShadowCalibrationStore) GetCalibration(recordID string) (*shadowdiff.CalibrationRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.calibrations[recordID]
	return record, ok
}

// GetVoteForDiff returns the vote for a specific diff, if any.
func (s *ShadowCalibrationStore) GetVoteForDiff(diffID string) (shadowdiff.CalibrationVote, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vote, ok := s.votesByDiff[diffID]
	return vote, ok
}

// =============================================================================
// Aggregation Helpers
// =============================================================================

// GetDiffCountByPeriod returns the total diff count for a period.
func (s *ShadowCalibrationStore) GetDiffCountByPeriod(periodBucket string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.diffsByPeriod[periodBucket])
}

// GetVotedDiffsByPeriod returns all voted diffs for a period.
func (s *ShadowCalibrationStore) GetVotedDiffsByPeriod(periodBucket string) []*shadowdiff.DiffResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	diffIDs := s.diffsByPeriod[periodBucket]
	var results []*shadowdiff.DiffResult

	for _, id := range diffIDs {
		if _, hasVote := s.votesByDiff[id]; hasVote {
			if diff, ok := s.diffs[id]; ok {
				results = append(results, diff)
			}
		}
	}

	return results
}

// CountVotesByPeriod counts votes by type for a period.
func (s *ShadowCalibrationStore) CountVotesByPeriod(periodBucket string) (useful, unnecessary int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	diffIDs := s.diffsByPeriod[periodBucket]
	for _, id := range diffIDs {
		if vote, ok := s.votesByDiff[id]; ok {
			switch vote {
			case shadowdiff.VoteUseful:
				useful++
			case shadowdiff.VoteUnnecessary:
				unnecessary++
			}
		}
	}

	return useful, unnecessary
}

// =============================================================================
// Storelog Integration (Replay Support)
// =============================================================================

// diffRecord is the JSON structure for persisting diffs.
type diffRecord struct {
	DiffID       string `json:"diff_id"`
	CircleID     string `json:"circle_id"`
	Agreement    string `json:"agreement"`
	NoveltyType  string `json:"novelty_type"`
	PeriodBucket string `json:"period_bucket"`
	Hash         string `json:"hash"`
	CreatedAt    string `json:"created_at"`
}

// calibrationRecord is the JSON structure for persisting calibrations.
type calibrationRecord struct {
	RecordID     string `json:"record_id"`
	DiffID       string `json:"diff_id"`
	DiffHash     string `json:"diff_hash"`
	Vote         string `json:"vote"`
	PeriodBucket string `json:"period_bucket"`
	CreatedAt    string `json:"created_at"`
}

// ToStorelogRecord converts a diff result to a storelog record.
func (s *ShadowCalibrationStore) DiffToStorelogRecord(diff *shadowdiff.DiffResult) *storelog.LogRecord {
	payload := diffRecord{
		DiffID:       diff.DiffID,
		CircleID:     string(diff.CircleID),
		Agreement:    string(diff.Agreement),
		NoveltyType:  string(diff.NoveltyType),
		PeriodBucket: diff.PeriodBucket,
		Hash:         diff.Hash(),
		CreatedAt:    diff.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      storelog.RecordTypeShadowDiff,
		Version:   "v1",
		Timestamp: diff.CreatedAt,
		Payload:   string(data),
	}
}

// CalibrationToStorelogRecord converts a calibration record to a storelog record.
func (s *ShadowCalibrationStore) CalibrationToStorelogRecord(record *shadowdiff.CalibrationRecord) *storelog.LogRecord {
	payload := calibrationRecord{
		RecordID:     record.RecordID,
		DiffID:       record.DiffID,
		DiffHash:     record.DiffHash,
		Vote:         string(record.Vote),
		PeriodBucket: record.PeriodBucket,
		CreatedAt:    record.CreatedAt.UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      storelog.RecordTypeShadowCalibration,
		Version:   "v1",
		Timestamp: record.CreatedAt,
		Payload:   string(data),
	}
}
