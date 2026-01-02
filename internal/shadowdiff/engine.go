// Package shadowdiff provides the diff engine for comparing canon vs shadow.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL INVARIANTS:
//   - Shadow does NOT affect any execution path
//   - No policy mutation
//   - No routing hooks
//   - Pure function, clock-injected
//   - stdlib only, no goroutines, no I/O
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package shadowdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowdiff"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Engine
// =============================================================================

// Engine computes diffs between canon results and shadow receipts.
//
// CRITICAL: This engine is observation-only. It does NOT affect behavior.
type Engine struct {
	clock clock.Clock
}

// NewEngine creates a new diff engine with the given clock.
func NewEngine(clk clock.Clock) *Engine {
	return &Engine{
		clock: clk,
	}
}

// =============================================================================
// Canon Result (Input)
// =============================================================================

// CanonResult represents the canon system's output for a circle.
// This is what the rule-based system actually computed.
type CanonResult struct {
	// CircleID is the circle these results are for.
	CircleID identity.EntityID

	// Signals are the canon assessments per item/category.
	Signals []shadowdiff.CanonSignal

	// ComputedAt is when the canon result was computed.
	ComputedAt time.Time
}

// =============================================================================
// Diff Input
// =============================================================================

// DiffInput contains everything needed to compute a diff.
type DiffInput struct {
	// Canon is the rule-based system's result.
	Canon CanonResult

	// Shadow is the LLM observation receipt.
	Shadow *shadowllm.ShadowReceipt
}

// =============================================================================
// Diff Output
// =============================================================================

// DiffOutput contains the computed diff results.
type DiffOutput struct {
	// Results are the individual diff results.
	Results []shadowdiff.DiffResult

	// Summary contains aggregate counts.
	Summary DiffSummary

	// ComputedAt is when this diff was computed.
	ComputedAt time.Time
}

// DiffSummary contains aggregate counts for a diff batch.
type DiffSummary struct {
	// TotalDiffs is the total number of diffs computed.
	TotalDiffs int

	// MatchCount is how many diffs were matches.
	MatchCount int

	// ConflictCount is how many diffs were conflicts.
	ConflictCount int

	// EarlierCount is how many times shadow thought things were more urgent.
	EarlierCount int

	// LaterCount is how many times shadow thought things were less urgent.
	LaterCount int

	// SofterCount is how many times shadow was less confident.
	SofterCount int

	// ShadowOnlyCount is how many items shadow saw that canon missed.
	ShadowOnlyCount int

	// CanonOnlyCount is how many items canon saw that shadow missed.
	CanonOnlyCount int
}

// =============================================================================
// Compute
// =============================================================================

// Compute computes the diff between canon results and shadow receipt.
//
// CRITICAL: This is a pure function with clock injection.
// No side effects. No I/O. No behavior modification.
func (e *Engine) Compute(input DiffInput) (*DiffOutput, error) {
	now := e.clock.Now()
	periodBucket := now.UTC().Format("2006-01-02")

	// Build lookup maps by item key hash
	canonByKey := make(map[string]*shadowdiff.CanonSignal)
	for i := range input.Canon.Signals {
		sig := &input.Canon.Signals[i]
		canonByKey[sig.Key.ItemKeyHash] = sig
	}

	shadowByKey := make(map[string]*shadowdiff.ShadowSignal)
	if input.Shadow != nil {
		for i := range input.Shadow.Suggestions {
			sug := &input.Shadow.Suggestions[i]
			// Convert ShadowSuggestion to ShadowSignal
			sig := &shadowdiff.ShadowSignal{
				Key: shadowdiff.ComparisonKey{
					CircleID:    input.Shadow.CircleID,
					Category:    sug.Category,
					ItemKeyHash: sug.ItemKeyHash,
				},
				Horizon:        sug.Horizon,
				Magnitude:      sug.Magnitude,
				Confidence:     sug.Confidence,
				SuggestionType: sug.SuggestionType,
			}
			shadowByKey[sug.ItemKeyHash] = sig
		}
	}

	// Collect all unique keys
	allKeys := make(map[string]bool)
	for k := range canonByKey {
		allKeys[k] = true
	}
	for k := range shadowByKey {
		allKeys[k] = true
	}

	// Sort keys for deterministic ordering
	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Compute diffs
	results := make([]shadowdiff.DiffResult, 0, len(sortedKeys))
	summary := DiffSummary{}

	for _, key := range sortedKeys {
		canonSig := canonByKey[key]
		shadowSig := shadowByKey[key]

		hasCanon := canonSig != nil
		hasShadow := shadowSig != nil

		novelty := ComputeNovelty(hasCanon, hasShadow)
		var agreement shadowdiff.AgreementKind

		if novelty == shadowdiff.NoveltyNone {
			agreement = ComputeAgreement(canonSig, shadowSig)
		}

		// Generate deterministic diff ID
		diffID := generateDiffID(input.Canon.CircleID, key, periodBucket)

		// Build comparison key
		var compKey shadowdiff.ComparisonKey
		if canonSig != nil {
			compKey = canonSig.Key
		} else if shadowSig != nil {
			compKey = shadowSig.Key
		}

		result := shadowdiff.DiffResult{
			DiffID:       diffID,
			CircleID:     input.Canon.CircleID,
			Key:          compKey,
			CanonSignal:  canonSig,
			ShadowSignal: shadowSig,
			Agreement:    agreement,
			NoveltyType:  novelty,
			PeriodBucket: periodBucket,
			CreatedAt:    now,
		}

		results = append(results, result)
		summary.TotalDiffs++

		// Update summary counts
		switch novelty {
		case shadowdiff.NoveltyShadowOnly:
			summary.ShadowOnlyCount++
		case shadowdiff.NoveltyCanonOnly:
			summary.CanonOnlyCount++
		case shadowdiff.NoveltyNone:
			switch agreement {
			case shadowdiff.AgreementMatch:
				summary.MatchCount++
			case shadowdiff.AgreementConflict:
				summary.ConflictCount++
			case shadowdiff.AgreementEarlier:
				summary.EarlierCount++
			case shadowdiff.AgreementLater:
				summary.LaterCount++
			case shadowdiff.AgreementSofter:
				summary.SofterCount++
			}
		}
	}

	return &DiffOutput{
		Results:    results,
		Summary:    summary,
		ComputedAt: now,
	}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// generateDiffID generates a deterministic diff ID from components.
func generateDiffID(circleID identity.EntityID, itemKeyHash, periodBucket string) string {
	input := string(circleID) + "|" + itemKeyHash + "|" + periodBucket
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16]) // First 16 bytes = 32 hex chars
}
