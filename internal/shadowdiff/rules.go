// Package shadowdiff provides the diff engine for comparing canon vs shadow.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL: Shadow does NOT affect any execution path.
// This is measurement ONLY.
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package shadowdiff

import (
	"quantumlife/pkg/domain/shadowdiff"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Diff Rules
// =============================================================================

// horizonOrder maps horizons to their urgency order.
// Lower number = more urgent.
var horizonOrder = map[shadowllm.Horizon]int{
	shadowllm.HorizonNow:     0,
	shadowllm.HorizonSoon:    1,
	shadowllm.HorizonLater:   2,
	shadowllm.HorizonSomeday: 3,
}

// magnitudeOrder maps magnitudes to their size order.
// Lower number = smaller.
var magnitudeOrder = map[shadowllm.MagnitudeBucket]int{
	shadowllm.MagnitudeNothing: 0,
	shadowllm.MagnitudeAFew:    1,
	shadowllm.MagnitudeSeveral: 2,
}

// confidenceOrder maps confidence buckets to their order.
// Lower number = less confident.
var confidenceOrder = map[shadowllm.ConfidenceBucket]int{
	shadowllm.ConfidenceLow:  0,
	shadowllm.ConfidenceMed:  1,
	shadowllm.ConfidenceHigh: 2,
}

// ComputeAgreement determines the agreement kind between canon and shadow signals.
//
// Rules:
//   - Same horizon + same magnitude → match
//   - Shadow earlier horizon → earlier
//   - Shadow later horizon → later
//   - Same magnitude, lower confidence → softer
//   - Opposite magnitude bands → conflict
//
// CRITICAL: This is a pure function. No side effects. No I/O.
func ComputeAgreement(canon *shadowdiff.CanonSignal, shadow *shadowdiff.ShadowSignal) shadowdiff.AgreementKind {
	if canon == nil || shadow == nil {
		// This shouldn't happen for NoveltyNone, but return conflict as fallback
		return shadowdiff.AgreementConflict
	}

	canonHorizon := horizonOrder[canon.Horizon]
	shadowHorizon := horizonOrder[shadow.Horizon]
	canonMagnitude := magnitudeOrder[canon.Magnitude]
	shadowMagnitude := magnitudeOrder[shadow.Magnitude]
	shadowConfidence := confidenceOrder[shadow.Confidence]

	// Rule 1: Same horizon and magnitude - check confidence for softer vs match
	if canon.Horizon == shadow.Horizon && canon.Magnitude == shadow.Magnitude {
		// If shadow has low confidence, it's softer (less certain)
		if shadowConfidence < confidenceOrder[shadowllm.ConfidenceHigh] {
			return shadowdiff.AgreementSofter
		}
		// Full confidence = match
		return shadowdiff.AgreementMatch
	}

	// Rule 2: Conflict - opposite magnitude bands
	// (nothing vs several is a conflict)
	if abs(canonMagnitude-shadowMagnitude) >= 2 {
		return shadowdiff.AgreementConflict
	}

	// Rule 3: Earlier - shadow thinks it's more urgent
	if shadowHorizon < canonHorizon {
		return shadowdiff.AgreementEarlier
	}

	// Rule 4: Later - shadow thinks it's less urgent
	if shadowHorizon > canonHorizon {
		return shadowdiff.AgreementLater
	}

	// Rule 5: Adjacent magnitudes - treat as conflict
	// (a_few vs several or nothing vs a_few)
	if canonMagnitude != shadowMagnitude {
		return shadowdiff.AgreementConflict
	}

	// Default to match if we get here
	return shadowdiff.AgreementMatch
}

// ComputeNovelty determines the novelty type based on signal presence.
//
// CRITICAL: Pure function. No side effects.
func ComputeNovelty(hasCanon, hasShadow bool) shadowdiff.Novelty {
	switch {
	case hasCanon && hasShadow:
		return shadowdiff.NoveltyNone
	case !hasCanon && hasShadow:
		return shadowdiff.NoveltyShadowOnly
	case hasCanon && !hasShadow:
		return shadowdiff.NoveltyCanonOnly
	default:
		// Neither has signal - shouldn't happen in practice
		return shadowdiff.NoveltyNone
	}
}

// IsConflict returns true if the agreement indicates a conflict.
func IsConflict(a shadowdiff.AgreementKind) bool {
	return a == shadowdiff.AgreementConflict
}

// IsMatch returns true if the agreement indicates a match.
func IsMatch(a shadowdiff.AgreementKind) bool {
	return a == shadowdiff.AgreementMatch
}

// IsNovel returns true if there's novelty (one side saw something the other missed).
func IsNovel(n shadowdiff.Novelty) bool {
	return n == shadowdiff.NoveltyShadowOnly || n == shadowdiff.NoveltyCanonOnly
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
