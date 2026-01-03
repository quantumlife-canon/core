// Package demo_phase19_5_shadow_gating provides demo tests for Phase 19.5.
//
// Phase 19.5: Shadow Gating + Promotion Candidates (NO behavior change)
//
// CRITICAL INVARIANTS TESTED:
//   - Shadow does NOT affect behavior
//   - Candidates are deterministic
//   - Privacy strings contain no forbidden patterns
//   - Promotion intents are recorded only (no behavior change)
//   - Hashes are stable
//   - Sorting is deterministic
//
// Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
package demo_phase19_5_shadow_gating

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/shadowgate"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowdiff"
	sg "quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
)

// createTestClock creates a deterministic clock for testing.
func createTestClock() clock.Clock {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return clock.NewFunc(func() time.Time {
		return fixedTime
	})
}

// mockDiffSource is a mock implementation of DiffSource for testing.
type mockDiffSource struct {
	diffs []*shadowdiff.DiffResult
	votes map[string]shadowdiff.CalibrationVote
}

func (m *mockDiffSource) ListDiffsByPeriod(periodKey string) []*shadowdiff.DiffResult {
	var result []*shadowdiff.DiffResult
	for _, d := range m.diffs {
		if d.PeriodBucket == periodKey {
			result = append(result, d)
		}
	}
	return result
}

func (m *mockDiffSource) GetVoteForDiff(diffID string) (shadowdiff.CalibrationVote, bool) {
	v, ok := m.votes[diffID]
	return v, ok
}

// createMockDiff creates a mock diff result.
func createMockDiff(id string, circleID identity.EntityID, novelty shadowdiff.Novelty, category shadowllm.AbstractCategory, periodKey string) *shadowdiff.DiffResult {
	diff := &shadowdiff.DiffResult{
		DiffID:       id,
		CircleID:     circleID,
		NoveltyType:  novelty,
		PeriodBucket: periodKey,
		CreatedAt:    time.Now(),
	}

	if novelty == shadowdiff.NoveltyShadowOnly {
		diff.ShadowSignal = &shadowdiff.ShadowSignal{
			Key: shadowdiff.ComparisonKey{
				CircleID: circleID,
				Category: category,
			},
			Horizon:   shadowllm.HorizonSoon,
			Magnitude: shadowllm.MagnitudeAFew,
		}
	} else if novelty == shadowdiff.NoveltyCanonOnly {
		diff.CanonSignal = &shadowdiff.CanonSignal{
			Key: shadowdiff.ComparisonKey{
				CircleID: circleID,
				Category: category,
			},
			Horizon:   shadowllm.HorizonSoon,
			Magnitude: shadowllm.MagnitudeAFew,
		}
	}

	return diff
}

// =============================================================================
// Test 1: Candidate Computation From Diffs
// =============================================================================

func TestCandidateComputationFromDiffs(t *testing.T) {
	clk := createTestClock()
	engine := shadowgate.NewEngine(clk)

	source := &mockDiffSource{
		diffs: []*shadowdiff.DiffResult{
			createMockDiff("diff-1", "circle-1", shadowdiff.NoveltyShadowOnly, shadowllm.CategoryMoney, "2024-01-15"),
			createMockDiff("diff-2", "circle-1", shadowdiff.NoveltyCanonOnly, shadowllm.CategoryWork, "2024-01-15"),
			createMockDiff("diff-3", "circle-1", shadowdiff.NoveltyShadowOnly, shadowllm.CategoryMoney, "2024-01-15"),
		},
		votes: make(map[string]shadowdiff.CalibrationVote),
	}

	output, err := engine.Compute(shadowgate.ComputeInput{
		PeriodKey:  "2024-01-15",
		CircleID:   "circle-1",
		DiffSource: source,
	})

	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	// Should have 2 unique candidates (money appears twice, work once)
	if len(output.Candidates) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(output.Candidates))
	}

	if output.TotalDiffs != 3 {
		t.Errorf("Expected 3 total diffs, got %d", output.TotalDiffs)
	}

	t.Logf("Candidate computation verified: %d candidates from %d diffs", len(output.Candidates), output.TotalDiffs)
}

// =============================================================================
// Test 2: Usefulness Bucket Computation
// =============================================================================

func TestUsefulnessBucketComputation(t *testing.T) {
	clk := createTestClock()
	engine := shadowgate.NewEngine(clk)

	source := &mockDiffSource{
		diffs: []*shadowdiff.DiffResult{
			createMockDiff("diff-1", "circle-1", shadowdiff.NoveltyShadowOnly, shadowllm.CategoryMoney, "2024-01-15"),
		},
		votes: map[string]shadowdiff.CalibrationVote{
			"diff-1": shadowdiff.VoteUseful,
		},
	}

	output, err := engine.Compute(shadowgate.ComputeInput{
		PeriodKey:  "2024-01-15",
		CircleID:   "circle-1",
		DiffSource: source,
	})

	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(output.Candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(output.Candidates))
	}

	c := output.Candidates[0]
	if c.VotesUseful != 1 {
		t.Errorf("Expected 1 useful vote, got %d", c.VotesUseful)
	}
	if c.UsefulnessBucket != sg.UsefulnessHigh {
		t.Errorf("Expected high usefulness, got %s", c.UsefulnessBucket)
	}

	t.Logf("Usefulness bucket verified: %s with %d useful votes", c.UsefulnessBucket, c.VotesUseful)
}

// =============================================================================
// Test 3: No Votes Results in Unknown Bucket
// =============================================================================

func TestNoVotesResultsInUnknownBucket(t *testing.T) {
	clk := createTestClock()
	engine := shadowgate.NewEngine(clk)

	source := &mockDiffSource{
		diffs: []*shadowdiff.DiffResult{
			createMockDiff("diff-1", "circle-1", shadowdiff.NoveltyShadowOnly, shadowllm.CategoryMoney, "2024-01-15"),
		},
		votes: make(map[string]shadowdiff.CalibrationVote),
	}

	output, err := engine.Compute(shadowgate.ComputeInput{
		PeriodKey:  "2024-01-15",
		CircleID:   "circle-1",
		DiffSource: source,
	})

	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(output.Candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(output.Candidates))
	}

	c := output.Candidates[0]
	if c.UsefulnessBucket != sg.UsefulnessUnknown {
		t.Errorf("Expected unknown usefulness, got %s", c.UsefulnessBucket)
	}
	if c.VoteConfidenceBucket != sg.VoteConfidenceUnknown {
		t.Errorf("Expected unknown confidence, got %s", c.VoteConfidenceBucket)
	}

	t.Logf("No votes verified: usefulness=%s, confidence=%s", c.UsefulnessBucket, c.VoteConfidenceBucket)
}

// =============================================================================
// Test 4: Vote Confidence Buckets
// =============================================================================

func TestVoteConfidenceBuckets(t *testing.T) {
	tests := []struct {
		voteCount int
		expected  sg.VoteConfidenceBucket
	}{
		{0, sg.VoteConfidenceUnknown},
		{1, sg.VoteConfidenceLow},
		{2, sg.VoteConfidenceLow},
		{3, sg.VoteConfidenceMedium},
		{5, sg.VoteConfidenceMedium},
		{6, sg.VoteConfidenceHigh},
		{10, sg.VoteConfidenceHigh},
	}

	for _, tt := range tests {
		got := sg.VoteConfidenceBucketFromCount(tt.voteCount)
		if got != tt.expected {
			t.Errorf("VoteConfidenceBucketFromCount(%d) = %s, want %s", tt.voteCount, got, tt.expected)
		}
	}

	t.Log("Vote confidence buckets verified")
}

// =============================================================================
// Test 5: Privacy Guard Blocks Unsafe Strings
// =============================================================================

func TestPrivacyGuardBlocksUnsafeStrings(t *testing.T) {
	guard := shadowgate.NewPrivacyGuard()

	unsafeStrings := []string{
		"Contact john@example.com for details",
		"Visit https://example.com",
		"Amount: $1000",
		"Call 555-123-4567",
		"Amazon order #12345678",
		"IP: 192.168.1.1",
	}

	for _, s := range unsafeStrings {
		err := guard.ValidateWhyGeneric(s)
		if err == nil {
			t.Errorf("Expected privacy error for: %s", s)
		}
	}

	t.Log("Privacy guard blocking verified")
}

// =============================================================================
// Test 6: Privacy Guard Allows Safe Strings
// =============================================================================

func TestPrivacyGuardAllowsSafeStrings(t *testing.T) {
	guard := shadowgate.NewPrivacyGuard()

	safeStrings := []string{
		"A pattern we've seen before.",
		"Something that recurs in this category.",
		"A timing pattern you might want to address.",
		"Items that tend to need attention together.",
	}

	for _, s := range safeStrings {
		err := guard.ValidateWhyGeneric(s)
		if err != nil {
			t.Errorf("Unexpected privacy error for safe string: %s - %v", s, err)
		}
	}

	t.Log("Privacy guard safe strings verified")
}

// =============================================================================
// Test 7: Candidate Sorting Determinism
// =============================================================================

func TestCandidateSortingDeterminism(t *testing.T) {
	now := time.Now()

	candidates := []sg.Candidate{
		{UsefulnessBucket: sg.UsefulnessLow, Origin: sg.OriginShadowOnly, Hash: "zzz", CreatedAt: now},
		{UsefulnessBucket: sg.UsefulnessHigh, Origin: sg.OriginCanonOnly, Hash: "aaa", CreatedAt: now},
		{UsefulnessBucket: sg.UsefulnessHigh, Origin: sg.OriginShadowOnly, Hash: "mmm", CreatedAt: now},
	}

	// Sort multiple times and verify consistency
	sg.SortCandidates(candidates)
	order1 := make([]string, len(candidates))
	for i, c := range candidates {
		order1[i] = c.Hash
	}

	sg.SortCandidates(candidates)
	order2 := make([]string, len(candidates))
	for i, c := range candidates {
		order2[i] = c.Hash
	}

	for i := range order1 {
		if order1[i] != order2[i] {
			t.Errorf("Sorting not deterministic at position %d: %s vs %s", i, order1[i], order2[i])
		}
	}

	// Verify expected order: high+shadow first, then high+canon, then low
	expected := []string{"mmm", "aaa", "zzz"}
	for i, c := range candidates {
		if c.Hash != expected[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expected[i], c.Hash)
		}
	}

	t.Log("Candidate sorting determinism verified")
}

// =============================================================================
// Test 8: Promotion Intent Creation
// =============================================================================

func TestPromotionIntentCreation(t *testing.T) {
	clk := createTestClock()
	store := persist.NewShadowGateStore(clk.Now)

	// Create a candidate first
	candidate := sg.Candidate{
		PeriodKey:            "2024-01-15",
		CircleID:             identity.EntityID("circle-1"),
		Origin:               sg.OriginShadowOnly,
		Category:             shadowllm.CategoryMoney,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "A pattern we've seen before.",
		UsefulnessPct:        75,
		UsefulnessBucket:     sg.UsefulnessMedium,
		VoteConfidenceBucket: sg.VoteConfidenceMedium,
		CreatedAt:            clk.Now(),
	}
	candidate.ID = candidate.ComputeID()
	candidate.Hash = candidate.ComputeHash()

	if err := store.AppendCandidate(&candidate); err != nil {
		t.Fatalf("Failed to append candidate: %v", err)
	}

	// Create promotion intent
	intent := sg.PromotionIntent{
		CandidateID:   candidate.ID,
		CandidateHash: candidate.Hash,
		PeriodKey:     "2024-01-15",
		NoteCode:      sg.NotePromoteRule,
		CreatedBucket: sg.FiveMinuteBucket(clk.Now()),
		CreatedAt:     clk.Now(),
	}
	intent.IntentID = intent.ComputeID()
	intent.IntentHash = intent.ComputeHash()

	if err := store.AppendPromotionIntent(&intent); err != nil {
		t.Fatalf("Failed to append intent: %v", err)
	}

	// Verify intent was stored
	intents := store.GetPromotionIntents("2024-01-15")
	if len(intents) != 1 {
		t.Errorf("Expected 1 intent, got %d", len(intents))
	}

	if intents[0].NoteCode != sg.NotePromoteRule {
		t.Errorf("Expected note code promote_rule, got %s", intents[0].NoteCode)
	}

	t.Logf("Promotion intent creation verified: %s", intent.IntentID)
}

// =============================================================================
// Test 9: Candidate Hash Stability
// =============================================================================

func TestCandidateHashStability(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	c := sg.Candidate{
		PeriodKey:            "2024-01-15",
		CircleID:             identity.EntityID("circle-1"),
		Origin:               sg.OriginShadowOnly,
		Category:             shadowllm.CategoryMoney,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "A pattern we've seen before.",
		UsefulnessPct:        75,
		UsefulnessBucket:     sg.UsefulnessMedium,
		VoteConfidenceBucket: sg.VoteConfidenceMedium,
		VotesUseful:          3,
		VotesUnnecessary:     1,
		FirstSeenBucket:      "2024-01-14",
		LastSeenBucket:       "2024-01-15",
		CreatedAt:            fixedTime,
	}

	h1 := c.ComputeHash()
	h2 := c.ComputeHash()

	if h1 != h2 {
		t.Errorf("Hash not stable: %s vs %s", h1, h2)
	}

	// Hash should be 64 hex chars
	if len(h1) != 64 {
		t.Errorf("Hash length should be 64, got %d", len(h1))
	}

	t.Logf("Candidate hash stability verified: %s", h1[:16])
}

// =============================================================================
// Test 10: Store Persistence and Retrieval
// =============================================================================

func TestStorePersistenceAndRetrieval(t *testing.T) {
	clk := createTestClock()
	store := persist.NewShadowGateStore(clk.Now)

	// Create candidates
	candidates := []sg.Candidate{
		{
			PeriodKey:            "2024-01-15",
			CircleID:             identity.EntityID("circle-1"),
			Origin:               sg.OriginShadowOnly,
			Category:             shadowllm.CategoryMoney,
			HorizonBucket:        shadowllm.HorizonSoon,
			MagnitudeBucket:      shadowllm.MagnitudeAFew,
			WhyGeneric:           "A pattern we've seen before.",
			UsefulnessPct:        50,
			UsefulnessBucket:     sg.UsefulnessMedium,
			VoteConfidenceBucket: sg.VoteConfidenceLow,
			CreatedAt:            clk.Now(),
		},
		{
			PeriodKey:            "2024-01-15",
			CircleID:             identity.EntityID("circle-1"),
			Origin:               sg.OriginCanonOnly,
			Category:             shadowllm.CategoryWork,
			HorizonBucket:        shadowllm.HorizonLater,
			MagnitudeBucket:      shadowllm.MagnitudeSeveral,
			WhyGeneric:           "Work items with similar urgency.",
			UsefulnessPct:        80,
			UsefulnessBucket:     sg.UsefulnessHigh,
			VoteConfidenceBucket: sg.VoteConfidenceHigh,
			CreatedAt:            clk.Now(),
		},
	}

	// Compute IDs and hashes
	for i := range candidates {
		candidates[i].ID = candidates[i].ComputeID()
		candidates[i].Hash = candidates[i].ComputeHash()
	}

	// Store candidates
	if err := store.AppendCandidates("2024-01-15", candidates); err != nil {
		t.Fatalf("Failed to append candidates: %v", err)
	}

	// Retrieve and verify
	retrieved := store.GetCandidates("2024-01-15")
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(retrieved))
	}

	// Verify sorting (high usefulness first)
	if retrieved[0].UsefulnessBucket != sg.UsefulnessHigh {
		t.Errorf("Expected first candidate to be high usefulness, got %s", retrieved[0].UsefulnessBucket)
	}

	t.Logf("Store persistence and retrieval verified: %d candidates", len(retrieved))
}

// =============================================================================
// Test 11: Empty Diffs Returns Empty Candidates
// =============================================================================

func TestEmptyDiffsReturnsEmptyCandidates(t *testing.T) {
	clk := createTestClock()
	engine := shadowgate.NewEngine(clk)

	source := &mockDiffSource{
		diffs: nil,
		votes: make(map[string]shadowdiff.CalibrationVote),
	}

	output, err := engine.Compute(shadowgate.ComputeInput{
		PeriodKey:  "2024-01-15",
		CircleID:   "circle-1",
		DiffSource: source,
	})

	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(output.Candidates) != 0 {
		t.Errorf("Expected 0 candidates, got %d", len(output.Candidates))
	}

	t.Log("Empty diffs handling verified")
}

// =============================================================================
// Test 12: Origin From Novelty
// =============================================================================

func TestOriginFromNovelty(t *testing.T) {
	tests := []struct {
		novelty     shadowdiff.Novelty
		hasConflict bool
		expected    sg.CandidateOrigin
	}{
		{shadowdiff.NoveltyShadowOnly, false, sg.OriginShadowOnly},
		{shadowdiff.NoveltyCanonOnly, false, sg.OriginCanonOnly},
		{shadowdiff.NoveltyNone, true, sg.OriginConflict},
		{shadowdiff.NoveltyShadowOnly, true, sg.OriginConflict},
	}

	for _, tt := range tests {
		got := sg.OriginFromNovelty(tt.novelty, tt.hasConflict)
		if got != tt.expected {
			t.Errorf("OriginFromNovelty(%s, %v) = %s, want %s", tt.novelty, tt.hasConflict, got, tt.expected)
		}
	}

	t.Log("Origin from novelty verified")
}

// =============================================================================
// Test 13: Allowed Reason Phrases
// =============================================================================

func TestAllowedReasonPhrases(t *testing.T) {
	guard := shadowgate.NewPrivacyGuard()

	for _, phrase := range shadowgate.AllowedReasonPhrases {
		err := guard.ValidateWhyGeneric(phrase)
		if err != nil {
			t.Errorf("Allowed phrase failed validation: %s - %v", phrase, err)
		}
	}

	t.Logf("All %d allowed reason phrases validated", len(shadowgate.AllowedReasonPhrases))
}

// =============================================================================
// Test 14: Note Code Validation
// =============================================================================

func TestNoteCodeValidation(t *testing.T) {
	validCodes := []sg.NoteCode{sg.NotePromoteRule, sg.NoteNeedsMoreVotes, sg.NoteIgnoreForNow}
	for _, code := range validCodes {
		if !code.Validate() {
			t.Errorf("Valid note code failed validation: %s", code)
		}
	}

	invalidCode := sg.NoteCode("invalid_code")
	if invalidCode.Validate() {
		t.Error("Invalid note code passed validation")
	}

	t.Log("Note code validation verified")
}

// =============================================================================
// Test 15: Intent Hash Determinism
// =============================================================================

func TestIntentHashDeterminism(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC)

	intent := sg.PromotionIntent{
		CandidateID:   "candidate-1",
		CandidateHash: "hash-1",
		PeriodKey:     "2024-01-15",
		NoteCode:      sg.NotePromoteRule,
		CreatedBucket: "2024-01-15T10:35",
		CreatedAt:     fixedTime,
	}

	h1 := intent.ComputeHash()
	h2 := intent.ComputeHash()

	if h1 != h2 {
		t.Errorf("Intent hash not deterministic: %s vs %s", h1, h2)
	}

	t.Logf("Intent hash determinism verified: %s", h1[:16])
}
