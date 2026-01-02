// Package shadowcalibration provides calibration aggregation for shadow diff.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL INVARIANTS:
//   - Observation-only - does NOT affect behavior
//   - Deterministic ordering for reproducibility
//   - stdlib only, no goroutines
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package shadowcalibration

import (
	"sort"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/shadowdiff"
)

// =============================================================================
// Engine
// =============================================================================

// Engine computes rolling calibration aggregates.
//
// CRITICAL: This engine is observation-only. Does NOT affect behavior.
type Engine struct {
	clock clock.Clock
}

// NewEngine creates a new calibration engine with the given clock.
func NewEngine(clk clock.Clock) *Engine {
	return &Engine{
		clock: clk,
	}
}

// =============================================================================
// Store Interface
// =============================================================================

// Store provides access to diff and calibration data.
type Store interface {
	// ListDiffsByPeriod returns all diffs for a given period bucket.
	ListDiffsByPeriod(periodBucket string) []*shadowdiff.DiffResult

	// GetVoteForDiff returns the vote for a specific diff, if any.
	GetVoteForDiff(diffID string) (shadowdiff.CalibrationVote, bool)
}

// =============================================================================
// Compute
// =============================================================================

// ComputeForPeriod computes calibration stats for a specific period.
//
// CRITICAL: Pure function with clock injection. Deterministic output.
func (e *Engine) ComputeForPeriod(store Store, periodBucket string) *shadowdiff.CalibrationStats {
	diffs := store.ListDiffsByPeriod(periodBucket)

	// Build votes map
	votes := make(map[string]shadowdiff.CalibrationVote)
	for _, diff := range diffs {
		if vote, ok := store.GetVoteForDiff(diff.DiffID); ok {
			votes[diff.DiffID] = vote
		}
	}

	return ComputeStats(periodBucket, diffs, votes)
}

// ComputeForToday computes calibration stats for today.
func (e *Engine) ComputeForToday(store Store) *shadowdiff.CalibrationStats {
	today := e.clock.Now().UTC().Format("2006-01-02")
	return e.ComputeForPeriod(store, today)
}

// ComputeRolling computes rolling stats over multiple periods.
//
// Returns stats for each period in sorted order (oldest first).
func (e *Engine) ComputeRolling(store Store, periods []string) []*shadowdiff.CalibrationStats {
	// Sort periods for deterministic ordering
	sortedPeriods := make([]string, len(periods))
	copy(sortedPeriods, periods)
	sort.Strings(sortedPeriods)

	results := make([]*shadowdiff.CalibrationStats, 0, len(sortedPeriods))
	for _, period := range sortedPeriods {
		stats := e.ComputeForPeriod(store, period)
		results = append(results, stats)
	}

	return results
}

// =============================================================================
// Report
// =============================================================================

// Report contains a human-readable calibration report.
type Report struct {
	// PeriodBucket is the period for this report.
	PeriodBucket string

	// Stats are the computed statistics.
	Stats *shadowdiff.CalibrationStats

	// Summary is a plain language summary.
	Summary string

	// AgreementPercentage is the agreement rate as a percentage string.
	AgreementPercentage string

	// NoveltyPercentage is the novelty rate as a percentage string.
	NoveltyPercentage string

	// ConflictPercentage is the conflict rate as a percentage string.
	ConflictPercentage string

	// UsefulnessPercentage is the usefulness score as a percentage string.
	UsefulnessPercentage string

	// HasVotes indicates whether any votes have been recorded.
	HasVotes bool
}

// GenerateReport generates a human-readable report for a period.
func (e *Engine) GenerateReport(store Store, periodBucket string) *Report {
	stats := e.ComputeForPeriod(store, periodBucket)

	return &Report{
		PeriodBucket:         periodBucket,
		Stats:                stats,
		Summary:              OverallSummary(stats),
		AgreementPercentage:  RateToPercentage(stats.AgreementRate),
		NoveltyPercentage:    RateToPercentage(stats.NoveltyRate),
		ConflictPercentage:   RateToPercentage(stats.ConflictRate),
		UsefulnessPercentage: RateToPercentage(stats.UsefulnessScore),
		HasVotes:             stats.VotedCount > 0,
	}
}

// =============================================================================
// Diff Summary
// =============================================================================

// DiffSummary contains a human-readable summary of a single diff.
type DiffSummary struct {
	// DiffID is the diff identifier.
	DiffID string

	// Agreement is the agreement kind.
	Agreement shadowdiff.AgreementKind

	// AgreementText is the plain language summary of agreement.
	AgreementText string

	// Novelty is the novelty type.
	Novelty shadowdiff.Novelty

	// NoveltyText is the plain language summary of novelty.
	NoveltyText string

	// Category is the abstract category.
	Category string

	// HasVote indicates whether this diff has been voted on.
	HasVote bool

	// Vote is the vote if HasVote is true.
	Vote shadowdiff.CalibrationVote
}

// SummarizeDiff creates a human-readable summary of a diff.
func SummarizeDiff(diff *shadowdiff.DiffResult, vote shadowdiff.CalibrationVote, hasVote bool) *DiffSummary {
	category := ""
	if diff.CanonSignal != nil {
		category = string(diff.CanonSignal.Key.Category)
	} else if diff.ShadowSignal != nil {
		category = string(diff.ShadowSignal.Key.Category)
	}

	return &DiffSummary{
		DiffID:        diff.DiffID,
		Agreement:     diff.Agreement,
		AgreementText: AgreementSummary(diff.Agreement),
		Novelty:       diff.NoveltyType,
		NoveltyText:   NoveltySummary(diff.NoveltyType),
		Category:      category,
		HasVote:       hasVote,
		Vote:          vote,
	}
}
