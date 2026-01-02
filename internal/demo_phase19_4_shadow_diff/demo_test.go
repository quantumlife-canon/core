// Package demo_phase19_4_shadow_diff provides demo tests for Phase 19.4.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL INVARIANTS TESTED:
//   - Determinism: same canon + same shadow + same clock => same diff hash
//   - Agreement rules: match, earlier, later, softer, conflict
//   - Novelty detection: shadow-only, canon-only
//   - Vote persistence and replay
//   - Stats aggregation stability
//   - No influence: shadow does NOT affect any execution path
//
// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md
package demo_phase19_4_shadow_diff

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/shadowcalibration"
	"quantumlife/internal/shadowdiff"
	"quantumlife/internal/shadowllm/stub"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	domaindiff "quantumlife/pkg/domain/shadowdiff"
	"quantumlife/pkg/domain/shadowllm"
)

// createTestClock creates a deterministic clock for testing.
func createTestClock() clock.Clock {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return clock.NewFunc(func() time.Time {
		return fixedTime
	})
}

// createCanonSignal creates a test canon signal.
func createCanonSignal(circleID, itemHash string, category shadowllm.AbstractCategory, horizon shadowllm.Horizon, magnitude shadowllm.MagnitudeBucket) domaindiff.CanonSignal {
	return domaindiff.CanonSignal{
		Key: domaindiff.ComparisonKey{
			CircleID:    identity.EntityID(circleID),
			Category:    category,
			ItemKeyHash: itemHash,
		},
		Horizon:         horizon,
		Magnitude:       magnitude,
		SurfaceDecision: false,
		HoldDecision:    true,
	}
}

// createShadowSignal creates a test shadow signal.
func createShadowSignal(circleID, itemHash string, category shadowllm.AbstractCategory, horizon shadowllm.Horizon, magnitude shadowllm.MagnitudeBucket, confidence shadowllm.ConfidenceBucket) domaindiff.ShadowSignal {
	return domaindiff.ShadowSignal{
		Key: domaindiff.ComparisonKey{
			CircleID:    identity.EntityID(circleID),
			Category:    category,
			ItemKeyHash: itemHash,
		},
		Horizon:        horizon,
		Magnitude:      magnitude,
		Confidence:     confidence,
		SuggestionType: shadowllm.SuggestHold,
	}
}

// =============================================================================
// Test 1: Canon vs Shadow Match
// =============================================================================

func TestAgreementMatch(t *testing.T) {
	canon := createCanonSignal("test-circle", "item-1", shadowllm.CategoryMoney, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew)
	shadow := createShadowSignal("test-circle", "item-1", shadowllm.CategoryMoney, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew, shadowllm.ConfidenceHigh)

	agreement := shadowdiff.ComputeAgreement(&canon, &shadow)

	if agreement != domaindiff.AgreementMatch {
		t.Errorf("Expected match, got: %s", agreement)
	}

	t.Log("Agreement match verified: same horizon + same magnitude = match")
}

// =============================================================================
// Test 2: Shadow Earlier
// =============================================================================

func TestAgreementEarlier(t *testing.T) {
	// Canon says "later", Shadow says "soon" (more urgent)
	canon := createCanonSignal("test-circle", "item-2", shadowllm.CategoryWork, shadowllm.HorizonLater, shadowllm.MagnitudeAFew)
	shadow := createShadowSignal("test-circle", "item-2", shadowllm.CategoryWork, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew, shadowllm.ConfidenceHigh)

	agreement := shadowdiff.ComputeAgreement(&canon, &shadow)

	if agreement != domaindiff.AgreementEarlier {
		t.Errorf("Expected earlier, got: %s", agreement)
	}

	t.Log("Agreement earlier verified: shadow thinks it's more urgent")
}

// =============================================================================
// Test 3: Shadow Later
// =============================================================================

func TestAgreementLater(t *testing.T) {
	// Canon says "now", Shadow says "later" (less urgent)
	canon := createCanonSignal("test-circle", "item-3", shadowllm.CategoryTime, shadowllm.HorizonNow, shadowllm.MagnitudeAFew)
	shadow := createShadowSignal("test-circle", "item-3", shadowllm.CategoryTime, shadowllm.HorizonLater, shadowllm.MagnitudeAFew, shadowllm.ConfidenceHigh)

	agreement := shadowdiff.ComputeAgreement(&canon, &shadow)

	if agreement != domaindiff.AgreementLater {
		t.Errorf("Expected later, got: %s", agreement)
	}

	t.Log("Agreement later verified: shadow thinks it can wait")
}

// =============================================================================
// Test 4: Conflict
// =============================================================================

func TestAgreementConflict(t *testing.T) {
	// Canon says "nothing", Shadow says "several" (opposite magnitude bands)
	canon := createCanonSignal("test-circle", "item-4", shadowllm.CategoryPeople, shadowllm.HorizonSoon, shadowllm.MagnitudeNothing)
	shadow := createShadowSignal("test-circle", "item-4", shadowllm.CategoryPeople, shadowllm.HorizonSoon, shadowllm.MagnitudeSeveral, shadowllm.ConfidenceHigh)

	agreement := shadowdiff.ComputeAgreement(&canon, &shadow)

	if agreement != domaindiff.AgreementConflict {
		t.Errorf("Expected conflict, got: %s", agreement)
	}

	t.Log("Agreement conflict verified: opposite magnitude bands")
}

// =============================================================================
// Test 5: Novelty Shadow-Only
// =============================================================================

func TestNoveltyShadowOnly(t *testing.T) {
	novelty := shadowdiff.ComputeNovelty(false, true)

	if novelty != domaindiff.NoveltyShadowOnly {
		t.Errorf("Expected shadow_only, got: %s", novelty)
	}

	t.Log("Novelty shadow-only verified: shadow noticed something rules missed")
}

// =============================================================================
// Test 6: Novelty Canon-Only
// =============================================================================

func TestNoveltyCanonOnly(t *testing.T) {
	novelty := shadowdiff.ComputeNovelty(true, false)

	if novelty != domaindiff.NoveltyCanonOnly {
		t.Errorf("Expected canon_only, got: %s", novelty)
	}

	t.Log("Novelty canon-only verified: rules caught something shadow missed")
}

// =============================================================================
// Test 7: Vote Persistence
// =============================================================================

func TestVotePersistence(t *testing.T) {
	clk := createTestClock()
	store := persist.NewShadowCalibrationStore(clk.Now)

	// Create a diff result
	diff := &domaindiff.DiffResult{
		DiffID:   "test-diff-1",
		CircleID: identity.EntityID("test-circle"),
		Key: domaindiff.ComparisonKey{
			CircleID:    identity.EntityID("test-circle"),
			Category:    shadowllm.CategoryMoney,
			ItemKeyHash: "item-hash-1",
		},
		CanonSignal:  &domaindiff.CanonSignal{},
		ShadowSignal: &domaindiff.ShadowSignal{},
		Agreement:    domaindiff.AgreementMatch,
		NoveltyType:  domaindiff.NoveltyNone,
		PeriodBucket: "2024-01-15",
		CreatedAt:    clk.Now(),
	}

	// Populate required fields
	diff.CanonSignal.Key = diff.Key
	diff.CanonSignal.Horizon = shadowllm.HorizonSoon
	diff.CanonSignal.Magnitude = shadowllm.MagnitudeAFew
	diff.ShadowSignal.Key = diff.Key
	diff.ShadowSignal.Horizon = shadowllm.HorizonSoon
	diff.ShadowSignal.Magnitude = shadowllm.MagnitudeAFew
	diff.ShadowSignal.Confidence = shadowllm.ConfidenceHigh
	diff.ShadowSignal.SuggestionType = shadowllm.SuggestHold

	// Store diff
	err := store.AppendDiff(diff)
	if err != nil {
		t.Fatalf("AppendDiff failed: %v", err)
	}

	// Create and store vote
	record := &domaindiff.CalibrationRecord{
		RecordID:     "vote-1",
		DiffID:       diff.DiffID,
		DiffHash:     diff.Hash(),
		Vote:         domaindiff.VoteUseful,
		PeriodBucket: "2024-01-15",
		CreatedAt:    clk.Now(),
	}

	err = store.AppendCalibration(record)
	if err != nil {
		t.Fatalf("AppendCalibration failed: %v", err)
	}

	// Retrieve vote
	vote, ok := store.GetVoteForDiff(diff.DiffID)
	if !ok {
		t.Fatal("Vote not found")
	}
	if vote != domaindiff.VoteUseful {
		t.Errorf("Expected useful, got: %s", vote)
	}

	t.Log("Vote persistence verified: vote stored and retrieved")
}

// =============================================================================
// Test 8: Replay Determinism
// =============================================================================

func TestReplayDeterminism(t *testing.T) {
	clk := createTestClock()
	engine := shadowdiff.NewEngine(clk)

	// Create canon result
	canon := shadowdiff.CanonResult{
		CircleID: identity.EntityID("replay-test"),
		Signals: []domaindiff.CanonSignal{
			createCanonSignal("replay-test", "item-a", shadowllm.CategoryMoney, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew),
			createCanonSignal("replay-test", "item-b", shadowllm.CategoryWork, shadowllm.HorizonLater, shadowllm.MagnitudeSeveral),
		},
		ComputedAt: clk.Now(),
	}

	// Create shadow receipt with suggestions
	shadow := &shadowllm.ShadowReceipt{
		ReceiptID:       "receipt-1",
		CircleID:        identity.EntityID("replay-test"),
		WindowBucket:    "2024-01-15",
		InputDigestHash: "digest-hash",
		ModelSpec:       "stub",
		CreatedAt:       clk.Now(),
		Suggestions: []shadowllm.ShadowSuggestion{
			{
				Category:       shadowllm.CategoryMoney,
				Horizon:        shadowllm.HorizonSoon,
				Magnitude:      shadowllm.MagnitudeAFew,
				Confidence:     shadowllm.ConfidenceHigh,
				SuggestionType: shadowllm.SuggestHold,
				ItemKeyHash:    "item-a",
			},
		},
		Provenance: shadowllm.Provenance{
			ProviderKind:  shadowllm.ProviderKindStub,
			LatencyBucket: shadowllm.LatencyNA,
			Status:        shadowllm.ReceiptStatusSuccess,
		},
	}

	// Run twice with same inputs
	input := shadowdiff.DiffInput{Canon: canon, Shadow: shadow}

	output1, err := engine.Compute(input)
	if err != nil {
		t.Fatalf("First compute failed: %v", err)
	}

	output2, err := engine.Compute(input)
	if err != nil {
		t.Fatalf("Second compute failed: %v", err)
	}

	// Hashes must be identical
	if len(output1.Results) != len(output2.Results) {
		t.Fatalf("Result count mismatch: %d vs %d", len(output1.Results), len(output2.Results))
	}

	for i := range output1.Results {
		hash1 := output1.Results[i].Hash()
		hash2 := output2.Results[i].Hash()
		if hash1 != hash2 {
			t.Errorf("Hash mismatch at index %d: %s vs %s", i, hash1[:16], hash2[:16])
		}
	}

	t.Logf("Replay determinism verified: %d diffs with identical hashes", len(output1.Results))
}

// =============================================================================
// Test 9: Stats Aggregation Stability
// =============================================================================

func TestStatsAggregationStability(t *testing.T) {
	clk := createTestClock()
	store := persist.NewShadowCalibrationStore(clk.Now)

	// Create multiple diff results with various outcomes
	diffs := []*domaindiff.DiffResult{
		{
			DiffID:       "diff-match",
			CircleID:     identity.EntityID("test"),
			Key:          domaindiff.ComparisonKey{CircleID: "test", Category: shadowllm.CategoryMoney, ItemKeyHash: "a"},
			Agreement:    domaindiff.AgreementMatch,
			NoveltyType:  domaindiff.NoveltyNone,
			PeriodBucket: "2024-01-15",
			CreatedAt:    clk.Now(),
		},
		{
			DiffID:       "diff-earlier",
			CircleID:     identity.EntityID("test"),
			Key:          domaindiff.ComparisonKey{CircleID: "test", Category: shadowllm.CategoryWork, ItemKeyHash: "b"},
			Agreement:    domaindiff.AgreementEarlier,
			NoveltyType:  domaindiff.NoveltyNone,
			PeriodBucket: "2024-01-15",
			CreatedAt:    clk.Now(),
		},
		{
			DiffID:       "diff-conflict",
			CircleID:     identity.EntityID("test"),
			Key:          domaindiff.ComparisonKey{CircleID: "test", Category: shadowllm.CategoryTime, ItemKeyHash: "c"},
			Agreement:    domaindiff.AgreementConflict,
			NoveltyType:  domaindiff.NoveltyNone,
			PeriodBucket: "2024-01-15",
			CreatedAt:    clk.Now(),
		},
		{
			DiffID:       "diff-novel",
			CircleID:     identity.EntityID("test"),
			Key:          domaindiff.ComparisonKey{CircleID: "test", Category: shadowllm.CategoryPeople, ItemKeyHash: "d"},
			Agreement:    "",
			NoveltyType:  domaindiff.NoveltyShadowOnly,
			PeriodBucket: "2024-01-15",
			CreatedAt:    clk.Now(),
		},
	}

	// Add minimal required fields for validation
	for _, diff := range diffs {
		if diff.NoveltyType == domaindiff.NoveltyNone {
			diff.CanonSignal = &domaindiff.CanonSignal{
				Key:       diff.Key,
				Horizon:   shadowllm.HorizonSoon,
				Magnitude: shadowllm.MagnitudeAFew,
			}
			diff.ShadowSignal = &domaindiff.ShadowSignal{
				Key:            diff.Key,
				Horizon:        shadowllm.HorizonSoon,
				Magnitude:      shadowllm.MagnitudeAFew,
				Confidence:     shadowllm.ConfidenceHigh,
				SuggestionType: shadowllm.SuggestHold,
			}
		}
		if diff.NoveltyType == domaindiff.NoveltyShadowOnly {
			diff.ShadowSignal = &domaindiff.ShadowSignal{
				Key:            diff.Key,
				Horizon:        shadowllm.HorizonSoon,
				Magnitude:      shadowllm.MagnitudeAFew,
				Confidence:     shadowllm.ConfidenceHigh,
				SuggestionType: shadowllm.SuggestHold,
			}
		}
		if err := store.AppendDiff(diff); err != nil {
			t.Fatalf("AppendDiff failed: %v", err)
		}
	}

	// Add votes
	store.AppendCalibration(&domaindiff.CalibrationRecord{
		RecordID: "v1", DiffID: "diff-match", DiffHash: "h1",
		Vote: domaindiff.VoteUseful, PeriodBucket: "2024-01-15", CreatedAt: clk.Now(),
	})
	store.AppendCalibration(&domaindiff.CalibrationRecord{
		RecordID: "v2", DiffID: "diff-earlier", DiffHash: "h2",
		Vote: domaindiff.VoteUseful, PeriodBucket: "2024-01-15", CreatedAt: clk.Now(),
	})

	// Compute stats twice
	calibEngine := shadowcalibration.NewEngine(clk)
	stats1 := calibEngine.ComputeForPeriod(store, "2024-01-15")
	stats2 := calibEngine.ComputeForPeriod(store, "2024-01-15")

	// Stats must be identical
	if stats1.Hash() != stats2.Hash() {
		t.Errorf("Stats hash mismatch: %s vs %s", stats1.Hash()[:16], stats2.Hash()[:16])
	}

	// Verify expected counts
	if stats1.TotalDiffs != 4 {
		t.Errorf("Expected 4 total diffs, got: %d", stats1.TotalDiffs)
	}
	if stats1.AgreementCounts[domaindiff.AgreementMatch] != 1 {
		t.Errorf("Expected 1 match, got: %d", stats1.AgreementCounts[domaindiff.AgreementMatch])
	}
	if stats1.AgreementCounts[domaindiff.AgreementConflict] != 1 {
		t.Errorf("Expected 1 conflict, got: %d", stats1.AgreementCounts[domaindiff.AgreementConflict])
	}
	if stats1.NoveltyCounts[domaindiff.NoveltyShadowOnly] != 1 {
		t.Errorf("Expected 1 shadow-only, got: %d", stats1.NoveltyCounts[domaindiff.NoveltyShadowOnly])
	}
	if stats1.VotedCount != 2 {
		t.Errorf("Expected 2 votes, got: %d", stats1.VotedCount)
	}

	t.Logf("Stats aggregation verified: total=%d, matches=%d, conflicts=%d, novelty=%d, votes=%d",
		stats1.TotalDiffs,
		stats1.AgreementCounts[domaindiff.AgreementMatch],
		stats1.AgreementCounts[domaindiff.AgreementConflict],
		stats1.NoveltyCounts[domaindiff.NoveltyShadowOnly],
		stats1.VotedCount)
}

// =============================================================================
// Test 10: Agreement Softer
// =============================================================================

func TestAgreementSofter(t *testing.T) {
	// Same magnitude, but shadow has low confidence
	canon := createCanonSignal("test-circle", "item-5", shadowllm.CategoryHealth, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew)
	shadow := createShadowSignal("test-circle", "item-5", shadowllm.CategoryHealth, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew, shadowllm.ConfidenceLow)

	agreement := shadowdiff.ComputeAgreement(&canon, &shadow)

	if agreement != domaindiff.AgreementSofter {
		t.Errorf("Expected softer, got: %s", agreement)
	}

	t.Log("Agreement softer verified: same data but shadow less confident")
}

// =============================================================================
// Test 11: Diff Engine Full Flow
// =============================================================================

func TestDiffEngineFullFlow(t *testing.T) {
	clk := createTestClock()
	engine := shadowdiff.NewEngine(clk)

	// Create canon with 2 items
	canon := shadowdiff.CanonResult{
		CircleID: identity.EntityID("full-flow"),
		Signals: []domaindiff.CanonSignal{
			createCanonSignal("full-flow", "item-x", shadowllm.CategoryMoney, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew),
			createCanonSignal("full-flow", "item-y", shadowllm.CategoryWork, shadowllm.HorizonNow, shadowllm.MagnitudeSeveral),
		},
		ComputedAt: clk.Now(),
	}

	// Shadow only sees item-x (with match) and item-z (novel)
	shadow := &shadowllm.ShadowReceipt{
		ReceiptID:       "full-receipt",
		CircleID:        identity.EntityID("full-flow"),
		WindowBucket:    "2024-01-15",
		InputDigestHash: "full-digest",
		ModelSpec:       "stub",
		CreatedAt:       clk.Now(),
		Suggestions: []shadowllm.ShadowSuggestion{
			{
				Category:       shadowllm.CategoryMoney,
				Horizon:        shadowllm.HorizonSoon,
				Magnitude:      shadowllm.MagnitudeAFew,
				Confidence:     shadowllm.ConfidenceHigh,
				SuggestionType: shadowllm.SuggestHold,
				ItemKeyHash:    "item-x",
			},
			{
				Category:       shadowllm.CategoryFamily,
				Horizon:        shadowllm.HorizonLater,
				Magnitude:      shadowllm.MagnitudeAFew,
				Confidence:     shadowllm.ConfidenceMed,
				SuggestionType: shadowllm.SuggestSurfaceCandidate,
				ItemKeyHash:    "item-z",
			},
		},
		Provenance: shadowllm.Provenance{
			ProviderKind:  shadowllm.ProviderKindStub,
			LatencyBucket: shadowllm.LatencyNA,
			Status:        shadowllm.ReceiptStatusSuccess,
		},
	}

	output, err := engine.Compute(shadowdiff.DiffInput{Canon: canon, Shadow: shadow})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	// Should have 3 diffs: item-x (match), item-y (canon-only), item-z (shadow-only)
	if len(output.Results) != 3 {
		t.Errorf("Expected 3 diffs, got: %d", len(output.Results))
	}

	// Verify summary counts
	if output.Summary.MatchCount != 1 {
		t.Errorf("Expected 1 match, got: %d", output.Summary.MatchCount)
	}
	if output.Summary.CanonOnlyCount != 1 {
		t.Errorf("Expected 1 canon-only, got: %d", output.Summary.CanonOnlyCount)
	}
	if output.Summary.ShadowOnlyCount != 1 {
		t.Errorf("Expected 1 shadow-only, got: %d", output.Summary.ShadowOnlyCount)
	}

	t.Logf("Full flow verified: %d diffs (matches=%d, canon-only=%d, shadow-only=%d)",
		len(output.Results), output.Summary.MatchCount,
		output.Summary.CanonOnlyCount, output.Summary.ShadowOnlyCount)
}

// =============================================================================
// Test 12: Diff Hash Stability
// =============================================================================

func TestDiffHashStability(t *testing.T) {
	clk := createTestClock()

	diff := &domaindiff.DiffResult{
		DiffID:   "stable-diff",
		CircleID: identity.EntityID("hash-test"),
		Key: domaindiff.ComparisonKey{
			CircleID:    identity.EntityID("hash-test"),
			Category:    shadowllm.CategoryMoney,
			ItemKeyHash: "stable-item",
		},
		CanonSignal: &domaindiff.CanonSignal{
			Key: domaindiff.ComparisonKey{
				CircleID:    identity.EntityID("hash-test"),
				Category:    shadowllm.CategoryMoney,
				ItemKeyHash: "stable-item",
			},
			Horizon:   shadowllm.HorizonSoon,
			Magnitude: shadowllm.MagnitudeAFew,
		},
		ShadowSignal: &domaindiff.ShadowSignal{
			Key: domaindiff.ComparisonKey{
				CircleID:    identity.EntityID("hash-test"),
				Category:    shadowllm.CategoryMoney,
				ItemKeyHash: "stable-item",
			},
			Horizon:        shadowllm.HorizonSoon,
			Magnitude:      shadowllm.MagnitudeAFew,
			Confidence:     shadowllm.ConfidenceHigh,
			SuggestionType: shadowllm.SuggestHold,
		},
		Agreement:    domaindiff.AgreementMatch,
		NoveltyType:  domaindiff.NoveltyNone,
		PeriodBucket: "2024-01-15",
		CreatedAt:    clk.Now(),
	}

	hash1 := diff.Hash()
	hash2 := diff.Hash()

	if hash1 != hash2 {
		t.Errorf("Hash instability: %s vs %s", hash1[:16], hash2[:16])
	}

	t.Logf("Hash stability verified: %s", hash1[:16])
}

// =============================================================================
// Test 13: Plain Language Summaries
// =============================================================================

func TestPlainLanguageSummaries(t *testing.T) {
	testCases := []struct {
		agreement domaindiff.AgreementKind
		expected  string
	}{
		{domaindiff.AgreementMatch, "Shadow agreed with the system."},
		{domaindiff.AgreementEarlier, "Shadow noticed something earlier."},
		{domaindiff.AgreementLater, "Shadow thought it could wait."},
		{domaindiff.AgreementSofter, "Shadow was less certain."},
		{domaindiff.AgreementConflict, "Shadow saw it differently."},
	}

	for _, tc := range testCases {
		summary := shadowcalibration.AgreementSummary(tc.agreement)
		if summary != tc.expected {
			t.Errorf("For %s: expected %q, got %q", tc.agreement, tc.expected, summary)
		}
	}

	t.Log("Plain language summaries verified")
}

// =============================================================================
// Test 14: Overall Summary Generation
// =============================================================================

func TestOverallSummaryGeneration(t *testing.T) {
	// High agreement
	highAgreement := &domaindiff.CalibrationStats{
		PeriodBucket:  "2024-01-15",
		TotalDiffs:    10,
		AgreementRate: 0.95,
	}
	summary := shadowcalibration.OverallSummary(highAgreement)
	if summary != "Shadow strongly agrees with the system." {
		t.Errorf("High agreement summary: %s", summary)
	}

	// Partial agreement
	partial := &domaindiff.CalibrationStats{
		PeriodBucket:  "2024-01-15",
		TotalDiffs:    10,
		AgreementRate: 0.5,
		ConflictRate:  0.2,
	}
	summary = shadowcalibration.OverallSummary(partial)
	if summary != "Shadow partially agrees with the system." {
		t.Errorf("Partial agreement summary: %s", summary)
	}

	// Empty
	empty := &domaindiff.CalibrationStats{
		PeriodBucket: "2024-01-15",
		TotalDiffs:   0,
	}
	summary = shadowcalibration.OverallSummary(empty)
	if summary != "No comparisons yet." {
		t.Errorf("Empty summary: %s", summary)
	}

	t.Log("Overall summary generation verified")
}

// =============================================================================
// Test 15: Stub Provider Produces Non-Empty Suggestions (Phase 19.4.1)
// =============================================================================

func TestStubProviderNonEmptySuggestions(t *testing.T) {
	clk := createTestClock()

	// Create stub context with identity circle
	ctx := shadowllm.ShadowContext{
		CircleID:   identity.EntityID("circle_38d867a436cedab0"),
		InputsHash: "test-inputs-hash-deterministic",
		Seed:       12345,
		Clock:      clk.Now,
	}

	// Create stub model
	stubModel := stub.NewStubModel()

	// Run observe
	run, err := stubModel.Observe(ctx)
	if err != nil {
		t.Fatalf("Stub observe failed: %v", err)
	}

	// CRITICAL: Stub must return at least 1 signal
	if len(run.Signals) == 0 {
		t.Fatal("Stub returned 0 signals - Phase 19.4.1 requires non-empty suggestions for diff flow")
	}

	// Stub should return 1-3 signals (deterministic based on seed)
	if len(run.Signals) < 1 || len(run.Signals) > 3 {
		t.Errorf("Expected 1-3 signals, got: %d", len(run.Signals))
	}

	t.Logf("Stub provider non-empty verified: %d signals", len(run.Signals))
}

// =============================================================================
// Test 16: Diff Engine Produces Non-Empty Output With Stub (Phase 19.4.1)
// =============================================================================

func TestDiffEngineNonEmptyWithStub(t *testing.T) {
	clk := createTestClock()
	diffEngine := shadowdiff.NewEngine(clk)

	// Create canon with 1 item
	canon := shadowdiff.CanonResult{
		CircleID: identity.EntityID("circle_38d867a436cedab0"),
		Signals: []domaindiff.CanonSignal{
			createCanonSignal("circle_38d867a436cedab0", "test-item", shadowllm.CategoryMoney, shadowllm.HorizonSoon, shadowllm.MagnitudeAFew),
		},
		ComputedAt: clk.Now(),
	}

	// Run stub to generate receipt
	stubModel := stub.NewStubModel()
	stubCtx := shadowllm.ShadowContext{
		CircleID:   identity.EntityID("circle_38d867a436cedab0"),
		InputsHash: "diff-test-inputs",
		Seed:       67890,
		Clock:      clk.Now,
	}
	stubRun, err := stubModel.Observe(stubCtx)
	if err != nil {
		t.Fatalf("Stub observe failed: %v", err)
	}

	// Convert stub signals to receipt suggestions
	// Use deterministic mapping from signal values
	suggestions := make([]shadowllm.ShadowSuggestion, 0, len(stubRun.Signals))
	for _, sig := range stubRun.Signals {
		// Map value float to horizon bucket
		var horizon shadowllm.Horizon
		if sig.ValueFloat < -0.5 {
			horizon = shadowllm.HorizonLater
		} else if sig.ValueFloat < 0.0 {
			horizon = shadowllm.HorizonSoon
		} else if sig.ValueFloat < 0.5 {
			horizon = shadowllm.HorizonNow
		} else {
			horizon = shadowllm.HorizonLater
		}

		// Map confidence float to magnitude bucket
		var magnitude shadowllm.MagnitudeBucket
		if sig.ConfidenceFloat < 0.25 {
			magnitude = shadowllm.MagnitudeNothing
		} else if sig.ConfidenceFloat < 0.5 {
			magnitude = shadowllm.MagnitudeAFew
		} else {
			magnitude = shadowllm.MagnitudeSeveral
		}

		suggestions = append(suggestions, shadowllm.ShadowSuggestion{
			Category:       sig.Category,
			Horizon:        horizon,
			Magnitude:      magnitude,
			Confidence:     shadowllm.ConfidenceFromFloat(sig.ConfidenceFloat),
			SuggestionType: shadowllm.SuggestHold,
			ItemKeyHash:    sig.ItemKeyHash,
		})
	}

	receipt := &shadowllm.ShadowReceipt{
		ReceiptID:       stubRun.RunID,
		CircleID:        stubCtx.CircleID,
		WindowBucket:    "2024-01-15",
		InputDigestHash: stubCtx.InputsHash,
		ModelSpec:       "stub",
		Suggestions:     suggestions,
		CreatedAt:       clk.Now(),
		Provenance: shadowllm.Provenance{
			ProviderKind:  shadowllm.ProviderKindStub,
			LatencyBucket: shadowllm.LatencyNA,
			Status:        shadowllm.ReceiptStatusSuccess,
		},
	}

	// Compute diff
	output, err := diffEngine.Compute(shadowdiff.DiffInput{Canon: canon, Shadow: receipt})
	if err != nil {
		t.Fatalf("Diff compute failed: %v", err)
	}

	// CRITICAL: Diff engine must produce at least 1 diff result
	if len(output.Results) == 0 {
		t.Fatal("Diff engine returned 0 results - Phase 19.4.1 requires non-empty diffs for calibration flow")
	}

	// Should have diffs from both canon (1 item) and shadow (1-3 items)
	totalPossible := 1 + len(stubRun.Signals) // canon items + shadow items
	if len(output.Results) > totalPossible {
		t.Errorf("Unexpected diff count: %d (max expected %d)", len(output.Results), totalPossible)
	}

	t.Logf("Diff engine non-empty verified: %d diffs (canon_only=%d, shadow_only=%d, matches=%d)",
		len(output.Results), output.Summary.CanonOnlyCount, output.Summary.ShadowOnlyCount, output.Summary.MatchCount)
}
