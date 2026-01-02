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
	"quantumlife/pkg/domain/shadowdiff"
)

// =============================================================================
// Stats Computation
// =============================================================================

// ComputeStats computes calibration statistics from diff results and votes.
//
// CRITICAL: Pure function. No side effects. Deterministic output.
func ComputeStats(
	periodBucket string,
	diffs []*shadowdiff.DiffResult,
	votes map[string]shadowdiff.CalibrationVote,
) *shadowdiff.CalibrationStats {
	stats := &shadowdiff.CalibrationStats{
		PeriodBucket:    periodBucket,
		AgreementCounts: make(map[shadowdiff.AgreementKind]int),
		NoveltyCounts:   make(map[shadowdiff.Novelty]int),
	}

	if len(diffs) == 0 {
		return stats
	}

	// Initialize counts
	for _, kind := range shadowdiff.AllAgreementKinds() {
		stats.AgreementCounts[kind] = 0
	}
	stats.NoveltyCounts[shadowdiff.NoveltyNone] = 0
	stats.NoveltyCounts[shadowdiff.NoveltyShadowOnly] = 0
	stats.NoveltyCounts[shadowdiff.NoveltyCanonOnly] = 0

	matchCount := 0
	conflictCount := 0
	novelCount := 0
	usefulCount := 0
	votedCount := 0

	for _, diff := range diffs {
		stats.TotalDiffs++

		// Count by novelty
		stats.NoveltyCounts[diff.NoveltyType]++

		if diff.NoveltyType != shadowdiff.NoveltyNone {
			novelCount++
		} else {
			// Count by agreement (only for non-novel diffs)
			stats.AgreementCounts[diff.Agreement]++

			if diff.Agreement == shadowdiff.AgreementMatch {
				matchCount++
			}
			if diff.Agreement == shadowdiff.AgreementConflict {
				conflictCount++
			}
		}

		// Count votes
		if vote, ok := votes[diff.DiffID]; ok {
			votedCount++
			if vote == shadowdiff.VoteUseful {
				usefulCount++
			}
		}
	}

	// Compute rates
	if stats.TotalDiffs > 0 {
		stats.AgreementRate = float64(matchCount) / float64(stats.TotalDiffs)
		stats.NoveltyRate = float64(novelCount) / float64(stats.TotalDiffs)
		stats.ConflictRate = float64(conflictCount) / float64(stats.TotalDiffs)
	}

	stats.VotedCount = votedCount
	if votedCount > 0 {
		stats.UsefulnessScore = float64(usefulCount) / float64(votedCount)
	}

	return stats
}

// =============================================================================
// Stats Helpers
// =============================================================================

// RateToPercentage converts a rate (0.0-1.0) to a percentage string.
func RateToPercentage(rate float64) string {
	pct := rate * 100
	if pct == 0 {
		return "0%"
	}
	if pct < 1 {
		return "<1%"
	}
	if pct > 99 && pct < 100 {
		return ">99%"
	}
	return formatPercent(pct)
}

// formatPercent formats a percentage value.
func formatPercent(pct float64) string {
	if pct == float64(int(pct)) {
		return formatInt(int(pct)) + "%"
	}
	return formatFloat(pct) + "%"
}

// formatInt formats an integer.
func formatInt(n int) string {
	if n < 0 {
		return "-" + formatUint(-n)
	}
	return formatUint(n)
}

// formatUint formats an unsigned integer.
func formatUint(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// formatFloat formats a float to one decimal place.
func formatFloat(f float64) string {
	// Simple implementation - one decimal place
	intPart := int(f)
	fracPart := int((f - float64(intPart)) * 10)
	if fracPart < 0 {
		fracPart = -fracPart
	}
	return formatInt(intPart) + "." + formatInt(fracPart)
}

// =============================================================================
// Plain Language Summaries
// =============================================================================

// AgreementSummary returns a plain language summary of agreement.
func AgreementSummary(agreement shadowdiff.AgreementKind) string {
	switch agreement {
	case shadowdiff.AgreementMatch:
		return "Shadow agreed with the system."
	case shadowdiff.AgreementEarlier:
		return "Shadow noticed something earlier."
	case shadowdiff.AgreementLater:
		return "Shadow thought it could wait."
	case shadowdiff.AgreementSofter:
		return "Shadow was less certain."
	case shadowdiff.AgreementConflict:
		return "Shadow saw it differently."
	default:
		return "Unknown comparison."
	}
}

// NoveltySummary returns a plain language summary of novelty.
func NoveltySummary(novelty shadowdiff.Novelty) string {
	switch novelty {
	case shadowdiff.NoveltyNone:
		return "Both systems noticed this."
	case shadowdiff.NoveltyShadowOnly:
		return "Shadow noticed something the rules missed."
	case shadowdiff.NoveltyCanonOnly:
		return "Rules caught something Shadow missed."
	default:
		return "Unknown comparison."
	}
}

// OverallSummary returns a plain language summary of calibration stats.
func OverallSummary(stats *shadowdiff.CalibrationStats) string {
	if stats.TotalDiffs == 0 {
		return "No comparisons yet."
	}

	// Primary metric: agreement rate
	if stats.AgreementRate >= 0.9 {
		return "Shadow strongly agrees with the system."
	}
	if stats.AgreementRate >= 0.7 {
		return "Shadow mostly agrees with the system."
	}
	if stats.AgreementRate >= 0.5 {
		return "Shadow partially agrees with the system."
	}
	if stats.ConflictRate >= 0.3 {
		return "Shadow often sees things differently."
	}

	return "Shadow has mixed observations."
}
