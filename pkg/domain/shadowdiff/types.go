// Package shadowdiff provides types for comparing canon decisions vs shadow observations.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL INVARIANTS:
//   - Shadow does NOT affect any execution path
//   - No policy mutation
//   - No routing hooks
//   - Abstract buckets only - no raw text, no identifiers
//   - Deterministic: same canon + same shadow + same clock = identical diff hash
//   - stdlib only, no goroutines, no time.Now()
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package shadowdiff

import (
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Comparison Key
// =============================================================================

// ComparisonKey uniquely identifies a diffable item.
// Uses abstract identifiers only - never raw content.
type ComparisonKey struct {
	// CircleID is the circle this comparison belongs to.
	CircleID identity.EntityID

	// Category is the abstract category being compared.
	Category shadowllm.AbstractCategory

	// ItemKeyHash is the SHA256 hash of the underlying item.
	// This allows correlation without exposing identifiable info.
	ItemKeyHash string
}

// Validate checks if the comparison key is valid.
func (k *ComparisonKey) Validate() error {
	if k.CircleID == "" {
		return ErrMissingCircleID
	}
	if !k.Category.Validate() {
		return ErrInvalidCategory
	}
	if k.ItemKeyHash == "" {
		return ErrMissingItemKeyHash
	}
	return nil
}

// =============================================================================
// Canon Signal (from rule-based system)
// =============================================================================

// CanonSignal represents the canon (rule-based) system's assessment.
//
// CRITICAL: This is what the rule-based system actually decided.
// Shadow observations are compared against this as ground truth.
type CanonSignal struct {
	// Key uniquely identifies what is being assessed.
	Key ComparisonKey

	// Horizon indicates when the canon system thinks this is relevant.
	Horizon shadowllm.Horizon

	// Magnitude indicates how much is present per canon rules.
	Magnitude shadowllm.MagnitudeBucket

	// SurfaceDecision indicates whether canon decided to surface this.
	SurfaceDecision bool

	// HoldDecision indicates whether canon decided to hold this.
	HoldDecision bool
}

// Validate checks if the canon signal is valid.
func (s *CanonSignal) Validate() error {
	if err := s.Key.Validate(); err != nil {
		return err
	}
	if !s.Horizon.Validate() {
		return ErrInvalidHorizon
	}
	if !s.Magnitude.Validate() {
		return ErrInvalidMagnitude
	}
	return nil
}

// =============================================================================
// Shadow Signal (from LLM observation)
// =============================================================================

// ShadowSignal represents the shadow (LLM) observation.
//
// CRITICAL: This is observation-only. It does NOT affect canon decisions.
type ShadowSignal struct {
	// Key uniquely identifies what is being assessed.
	Key ComparisonKey

	// Horizon indicates when shadow thinks this is relevant.
	Horizon shadowllm.Horizon

	// Magnitude indicates how much shadow thinks is present.
	Magnitude shadowllm.MagnitudeBucket

	// Confidence indicates shadow's confidence in this assessment.
	Confidence shadowllm.ConfidenceBucket

	// SuggestionType indicates what shadow thinks might be appropriate.
	// CRITICAL: This is LOGGED ONLY. Does NOT affect behavior.
	SuggestionType shadowllm.SuggestionType
}

// Validate checks if the shadow signal is valid.
func (s *ShadowSignal) Validate() error {
	if err := s.Key.Validate(); err != nil {
		return err
	}
	if !s.Horizon.Validate() {
		return ErrInvalidHorizon
	}
	if !s.Magnitude.Validate() {
		return ErrInvalidMagnitude
	}
	if !s.Confidence.Validate() {
		return ErrInvalidConfidence
	}
	if !s.SuggestionType.Validate() {
		return ErrInvalidSuggestionType
	}
	return nil
}

// =============================================================================
// Agreement Kind
// =============================================================================

// AgreementKind indicates how canon and shadow assessments align.
type AgreementKind string

const (
	// AgreementMatch means shadow agrees with canon.
	// Same horizon + same magnitude.
	AgreementMatch AgreementKind = "match"

	// AgreementSofter means shadow suggests less urgency.
	// Same magnitude, but shadow has lower confidence.
	AgreementSofter AgreementKind = "softer"

	// AgreementEarlier means shadow thinks this is more urgent.
	// Shadow's horizon is earlier than canon's.
	AgreementEarlier AgreementKind = "earlier"

	// AgreementLater means shadow thinks this is less urgent.
	// Shadow's horizon is later than canon's.
	AgreementLater AgreementKind = "later"

	// AgreementConflict means shadow and canon have opposite assessments.
	// Different magnitude bands or fundamentally different conclusions.
	AgreementConflict AgreementKind = "conflict"
)

// Validate checks if the agreement kind is valid.
func (a AgreementKind) Validate() bool {
	switch a {
	case AgreementMatch, AgreementSofter, AgreementEarlier, AgreementLater, AgreementConflict:
		return true
	default:
		return false
	}
}

// AllAgreementKinds returns all valid agreement kinds in sorted order.
func AllAgreementKinds() []AgreementKind {
	return []AgreementKind{
		AgreementConflict, AgreementEarlier, AgreementLater, AgreementMatch, AgreementSofter,
	}
}

// =============================================================================
// Novelty
// =============================================================================

// Novelty indicates whether signals exist in one system but not the other.
type Novelty string

const (
	// NoveltyNone means both canon and shadow have signals for this item.
	NoveltyNone Novelty = "none"

	// NoveltyShadowOnly means shadow has a signal but canon does not.
	// Shadow noticed something the rules missed.
	NoveltyShadowOnly Novelty = "shadow_only"

	// NoveltyCanonOnly means canon has a signal but shadow does not.
	// Rules caught something shadow missed.
	NoveltyCanonOnly Novelty = "canon_only"
)

// Validate checks if the novelty is valid.
func (n Novelty) Validate() bool {
	switch n {
	case NoveltyNone, NoveltyShadowOnly, NoveltyCanonOnly:
		return true
	default:
		return false
	}
}

// =============================================================================
// Calibration Vote
// =============================================================================

// CalibrationVote indicates whether a diff result was useful.
// This is human feedback for calibrating shadow usefulness.
type CalibrationVote string

const (
	// VoteUseful means the shadow observation was helpful.
	VoteUseful CalibrationVote = "useful"

	// VoteUnnecessary means the shadow observation was not helpful.
	VoteUnnecessary CalibrationVote = "unnecessary"
)

// Validate checks if the vote is valid.
func (v CalibrationVote) Validate() bool {
	switch v {
	case VoteUseful, VoteUnnecessary:
		return true
	default:
		return false
	}
}

// =============================================================================
// Diff Result
// =============================================================================

// DiffResult represents the comparison between canon and shadow signals.
//
// CRITICAL: This is for measurement ONLY. Does NOT affect behavior.
type DiffResult struct {
	// DiffID uniquely identifies this diff result.
	DiffID string

	// CircleID is the circle this diff belongs to.
	CircleID identity.EntityID

	// Key identifies the item being compared.
	Key ComparisonKey

	// CanonSignal is the canon (rule-based) assessment.
	// May be nil if NoveltyType is NoveltyShadowOnly.
	CanonSignal *CanonSignal

	// ShadowSignal is the shadow (LLM) observation.
	// May be nil if NoveltyType is NoveltyCanonOnly.
	ShadowSignal *ShadowSignal

	// Agreement indicates how the signals align.
	// Only meaningful when both signals are present (NoveltyNone).
	Agreement AgreementKind

	// NoveltyType indicates if one system saw something the other missed.
	NoveltyType Novelty

	// PeriodBucket is the time bucket for aggregation (e.g., "2024-01-15").
	PeriodBucket string

	// CreatedAt is when this diff was computed (injected clock).
	CreatedAt time.Time

	// hash is cached after first computation.
	hash string
}

// Validate checks if the diff result is valid.
func (d *DiffResult) Validate() error {
	if d.DiffID == "" {
		return ErrMissingDiffID
	}
	if d.CircleID == "" {
		return ErrMissingCircleID
	}
	if err := d.Key.Validate(); err != nil {
		return err
	}

	// Validate signals based on novelty type
	switch d.NoveltyType {
	case NoveltyNone:
		if d.CanonSignal == nil || d.ShadowSignal == nil {
			return ErrMissingSignals
		}
		if err := d.CanonSignal.Validate(); err != nil {
			return err
		}
		if err := d.ShadowSignal.Validate(); err != nil {
			return err
		}
		if !d.Agreement.Validate() {
			return ErrInvalidAgreement
		}
	case NoveltyShadowOnly:
		if d.ShadowSignal == nil {
			return ErrMissingShadowSignal
		}
		if err := d.ShadowSignal.Validate(); err != nil {
			return err
		}
	case NoveltyCanonOnly:
		if d.CanonSignal == nil {
			return ErrMissingCanonSignal
		}
		if err := d.CanonSignal.Validate(); err != nil {
			return err
		}
	default:
		return ErrInvalidNovelty
	}

	if d.PeriodBucket == "" {
		return ErrMissingPeriodBucket
	}
	if d.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}

	return nil
}

// =============================================================================
// Calibration Record
// =============================================================================

// CalibrationRecord stores a vote for a diff result.
// Append-only, hash-only persistence.
type CalibrationRecord struct {
	// RecordID uniquely identifies this record.
	RecordID string

	// DiffID references the diff being voted on.
	DiffID string

	// DiffHash is the hash of the diff at vote time.
	// Allows verification that the diff hasn't changed.
	DiffHash string

	// Vote is the calibration vote (useful/unnecessary).
	Vote CalibrationVote

	// PeriodBucket is the time bucket for aggregation.
	PeriodBucket string

	// CreatedAt is when this vote was recorded.
	CreatedAt time.Time
}

// Validate checks if the calibration record is valid.
func (r *CalibrationRecord) Validate() error {
	if r.RecordID == "" {
		return ErrMissingRecordID
	}
	if r.DiffID == "" {
		return ErrMissingDiffID
	}
	if r.DiffHash == "" {
		return ErrMissingDiffHash
	}
	if !r.Vote.Validate() {
		return ErrInvalidVote
	}
	if r.PeriodBucket == "" {
		return ErrMissingPeriodBucket
	}
	if r.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}
	return nil
}

// =============================================================================
// Calibration Stats
// =============================================================================

// CalibrationStats contains aggregated calibration metrics.
// All values are percentages (0.0 to 1.0) or counts.
type CalibrationStats struct {
	// PeriodBucket is the time period for these stats.
	PeriodBucket string

	// TotalDiffs is the total number of diffs in this period.
	TotalDiffs int

	// AgreementRate is the percentage of diffs where shadow matched canon.
	AgreementRate float64

	// NoveltyRate is the percentage of diffs where one system saw something the other missed.
	NoveltyRate float64

	// ConflictRate is the percentage of diffs where shadow and canon conflicted.
	ConflictRate float64

	// UsefulnessScore is the percentage of voted diffs marked as useful.
	// Only includes diffs that have been voted on.
	UsefulnessScore float64

	// VotedCount is the number of diffs that have been voted on.
	VotedCount int

	// AgreementCounts breaks down by agreement kind.
	AgreementCounts map[AgreementKind]int

	// NoveltyCounts breaks down by novelty type.
	NoveltyCounts map[Novelty]int
}

// Validate checks if the stats are valid.
func (s *CalibrationStats) Validate() error {
	if s.PeriodBucket == "" {
		return ErrMissingPeriodBucket
	}
	if s.TotalDiffs < 0 {
		return ErrInvalidStats
	}
	if s.AgreementRate < 0 || s.AgreementRate > 1 {
		return ErrInvalidStats
	}
	if s.NoveltyRate < 0 || s.NoveltyRate > 1 {
		return ErrInvalidStats
	}
	if s.ConflictRate < 0 || s.ConflictRate > 1 {
		return ErrInvalidStats
	}
	if s.UsefulnessScore < 0 || s.UsefulnessScore > 1 {
		return ErrInvalidStats
	}
	if s.VotedCount < 0 {
		return ErrInvalidStats
	}
	return nil
}

// =============================================================================
// Error Types
// =============================================================================

type diffError string

func (e diffError) Error() string { return string(e) }

const (
	ErrMissingCircleID       diffError = "missing circle ID"
	ErrMissingItemKeyHash    diffError = "missing item key hash"
	ErrInvalidCategory       diffError = "invalid category"
	ErrInvalidHorizon        diffError = "invalid horizon"
	ErrInvalidMagnitude      diffError = "invalid magnitude"
	ErrInvalidConfidence     diffError = "invalid confidence"
	ErrInvalidSuggestionType diffError = "invalid suggestion type"
	ErrMissingDiffID         diffError = "missing diff ID"
	ErrMissingSignals        diffError = "missing signals for novelty none"
	ErrMissingShadowSignal   diffError = "missing shadow signal"
	ErrMissingCanonSignal    diffError = "missing canon signal"
	ErrInvalidAgreement      diffError = "invalid agreement kind"
	ErrInvalidNovelty        diffError = "invalid novelty type"
	ErrMissingPeriodBucket   diffError = "missing period bucket"
	ErrMissingCreatedAt      diffError = "missing created at timestamp"
	ErrMissingRecordID       diffError = "missing record ID"
	ErrMissingDiffHash       diffError = "missing diff hash"
	ErrInvalidVote           diffError = "invalid vote"
	ErrInvalidStats          diffError = "invalid stats value"
)
