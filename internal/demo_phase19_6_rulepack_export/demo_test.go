// Package demo_phase19_6_rulepack_export provides demo tests for Phase 19.6.
//
// Phase 19.6: Rule Pack Export (Promotion Pipeline)
//
// CRITICAL INVARIANTS TESTED:
//   - RulePack does NOT apply itself
//   - Deterministic: same inputs + clock => same hashes
//   - Privacy: no raw identifiers in exports
//   - Stable ordering: reordering inputs yields identical output
//   - Gating: candidates below threshold excluded
//   - Replay: storelog replay restores state
//
// Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
package demo_phase19_6_rulepack_export

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	rulepackengine "quantumlife/internal/rulepack"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/rulepack"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/storelog"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestClock creates a deterministic clock for testing.
func createTestClock() clock.Clock {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return clock.NewFunc(func() time.Time {
		return fixedTime
	})
}

// mockIntentSource implements rulepackengine.IntentSource for testing.
type mockIntentSource struct {
	intents    map[string][]shadowgate.PromotionIntent
	candidates map[string]*shadowgate.Candidate
}

func newMockIntentSource() *mockIntentSource {
	return &mockIntentSource{
		intents:    make(map[string][]shadowgate.PromotionIntent),
		candidates: make(map[string]*shadowgate.Candidate),
	}
}

func (m *mockIntentSource) GetPromotionIntents(periodKey string) []shadowgate.PromotionIntent {
	return m.intents[periodKey]
}

func (m *mockIntentSource) GetCandidate(candidateID string) (*shadowgate.Candidate, bool) {
	c, ok := m.candidates[candidateID]
	return c, ok
}

func (m *mockIntentSource) AddCandidate(c *shadowgate.Candidate) {
	m.candidates[c.ID] = c
}

func (m *mockIntentSource) AddIntent(i shadowgate.PromotionIntent) {
	m.intents[i.PeriodKey] = append(m.intents[i.PeriodKey], i)
}

// createQualifiedCandidate creates a candidate that meets gating thresholds.
func createQualifiedCandidate(id string, circleID identity.EntityID, category shadowllm.AbstractCategory, origin shadowgate.CandidateOrigin, clk clock.Clock) *shadowgate.Candidate {
	now := clk.Now()
	c := &shadowgate.Candidate{
		PeriodKey:            rulepack.PeriodKeyFromTime(now),
		CircleID:             circleID,
		Origin:               origin,
		Category:             category,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "A pattern we've seen before.",
		UsefulnessPct:        80,
		UsefulnessBucket:     shadowgate.UsefulnessHigh, // >= medium
		VotesUseful:          4,
		VotesUnnecessary:     1,
		VoteConfidenceBucket: shadowgate.VoteConfidenceMedium, // >= medium
		FirstSeenBucket:      "2024-01-14",
		LastSeenBucket:       "2024-01-15",
		CreatedAt:            now,
	}
	c.ID = id
	c.Hash = c.ComputeHash()
	return c
}

// createUnqualifiedCandidate creates a candidate that fails gating.
func createUnqualifiedCandidate(id string, circleID identity.EntityID, clk clock.Clock) *shadowgate.Candidate {
	now := clk.Now()
	c := &shadowgate.Candidate{
		PeriodKey:            rulepack.PeriodKeyFromTime(now),
		CircleID:             circleID,
		Origin:               shadowgate.OriginShadowOnly,
		Category:             shadowllm.CategoryMoney,
		HorizonBucket:        shadowllm.HorizonSoon,
		MagnitudeBucket:      shadowllm.MagnitudeAFew,
		WhyGeneric:           "Something we noticed.",
		UsefulnessPct:        20,
		UsefulnessBucket:     shadowgate.UsefulnessLow, // < medium (fails)
		VotesUseful:          1,
		VotesUnnecessary:     0,
		VoteConfidenceBucket: shadowgate.VoteConfidenceLow, // < medium (fails)
		FirstSeenBucket:      "2024-01-15",
		LastSeenBucket:       "2024-01-15",
		CreatedAt:            now,
	}
	c.ID = id
	c.Hash = c.ComputeHash()
	return c
}

// =============================================================================
// Test 1: Determinism - Same Inputs Produce Same Output
// =============================================================================

func TestDeterminism_SameInputsProduceSameOutput(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	source := newMockIntentSource()

	// Add qualified candidate
	c := createQualifiedCandidate("candidate-1", "circle-1", shadowllm.CategoryMoney, shadowgate.OriginShadowOnly, clk)
	source.AddCandidate(c)

	// Add promotion intent
	intent := shadowgate.PromotionIntent{
		CandidateID:   c.ID,
		CandidateHash: c.Hash,
		PeriodKey:     "2024-01-15",
		NoteCode:      shadowgate.NotePromoteRule,
		CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
		CreatedAt:     clk.Now(),
	}
	intent.IntentID = intent.ComputeID()
	intent.IntentHash = intent.ComputeHash()
	source.AddIntent(intent)

	// Build twice
	output1, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}

	output2, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}

	// Verify identical PackID and PackHash
	if output1.Pack.PackID != output2.Pack.PackID {
		t.Errorf("PackID not deterministic: %s vs %s", output1.Pack.PackID, output2.Pack.PackID)
	}
	if output1.Pack.PackHash != output2.Pack.PackHash {
		t.Errorf("PackHash not deterministic: %s vs %s", output1.Pack.PackHash, output2.Pack.PackHash)
	}

	// Verify identical export text
	text1 := output1.Pack.ToText()
	text2 := output2.Pack.ToText()
	if text1 != text2 {
		t.Error("Export text not deterministic")
	}

	t.Logf("Determinism verified: PackID=%s", output1.Pack.PackID[:16])
}

// =============================================================================
// Test 2: Privacy - No Forbidden Patterns in Export
// =============================================================================

func TestPrivacy_NoForbiddenPatternsInExport(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	source := newMockIntentSource()
	c := createQualifiedCandidate("candidate-1", "circle-1", shadowllm.CategoryMoney, shadowgate.OriginShadowOnly, clk)
	source.AddCandidate(c)

	intent := shadowgate.PromotionIntent{
		CandidateID:   c.ID,
		CandidateHash: c.Hash,
		PeriodKey:     "2024-01-15",
		NoteCode:      shadowgate.NotePromoteRule,
		CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
		CreatedAt:     clk.Now(),
	}
	intent.IntentID = intent.ComputeID()
	intent.IntentHash = intent.ComputeHash()
	source.AddIntent(intent)

	output, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	text := output.Pack.ToText()

	// Validate privacy
	if err := rulepack.ValidateExportPrivacy(text); err != nil {
		t.Errorf("Privacy violation in export: %v", err)
	}

	// Explicit checks
	forbiddenPatterns := []string{"@", "http://", "$", "amazon", "google"}
	for _, pattern := range forbiddenPatterns {
		if containsPattern(text, pattern) {
			t.Errorf("Export contains forbidden pattern: %s", pattern)
		}
	}

	t.Log("Privacy verified: no forbidden patterns in export")
}

func containsPattern(text, pattern string) bool {
	return len(text) > 0 && len(pattern) > 0 &&
		(len(pattern) <= len(text)) &&
		(text == pattern || contains(text, pattern))
}

func contains(text, pattern string) bool {
	for i := 0; i <= len(text)-len(pattern); i++ {
		if text[i:i+len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// =============================================================================
// Test 3: Stable Ordering - Reordering Inputs Yields Identical Export
// =============================================================================

func TestStableOrdering_ReorderingInputsYieldsIdenticalExport(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	// Create candidates in different order
	c1 := createQualifiedCandidate("candidate-1", "circle-1", shadowllm.CategoryMoney, shadowgate.OriginShadowOnly, clk)
	c2 := createQualifiedCandidate("candidate-2", "circle-1", shadowllm.CategoryWork, shadowgate.OriginCanonOnly, clk)
	c3 := createQualifiedCandidate("candidate-3", "circle-2", shadowllm.CategoryTime, shadowgate.OriginShadowOnly, clk)

	// Source with order 1, 2, 3
	source1 := newMockIntentSource()
	for _, c := range []*shadowgate.Candidate{c1, c2, c3} {
		source1.AddCandidate(c)
		intent := shadowgate.PromotionIntent{
			CandidateID:   c.ID,
			CandidateHash: c.Hash,
			PeriodKey:     "2024-01-15",
			NoteCode:      shadowgate.NotePromoteRule,
			CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
			CreatedAt:     clk.Now(),
		}
		intent.IntentID = intent.ComputeID()
		intent.IntentHash = intent.ComputeHash()
		source1.AddIntent(intent)
	}

	// Source with order 3, 1, 2
	source2 := newMockIntentSource()
	for _, c := range []*shadowgate.Candidate{c3, c1, c2} {
		source2.AddCandidate(c)
		intent := shadowgate.PromotionIntent{
			CandidateID:   c.ID,
			CandidateHash: c.Hash,
			PeriodKey:     "2024-01-15",
			NoteCode:      shadowgate.NotePromoteRule,
			CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
			CreatedAt:     clk.Now(),
		}
		intent.IntentID = intent.ComputeID()
		intent.IntentHash = intent.ComputeHash()
		source2.AddIntent(intent)
	}

	output1, _ := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source1,
	})
	output2, _ := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source2,
	})

	// PackHash must be identical regardless of input order
	if output1.Pack.PackHash != output2.Pack.PackHash {
		t.Errorf("PackHash differs with reordering: %s vs %s", output1.Pack.PackHash[:16], output2.Pack.PackHash[:16])
	}

	// Export text must be identical
	if output1.Pack.ToText() != output2.Pack.ToText() {
		t.Error("Export text differs with reordering")
	}

	t.Log("Stable ordering verified: reordering inputs produces identical output")
}

// =============================================================================
// Test 4: Gating - Candidates Below Threshold Excluded
// =============================================================================

func TestGating_CandidatesBelowThresholdExcluded(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	source := newMockIntentSource()

	// Add one qualified and one unqualified candidate
	qualified := createQualifiedCandidate("qualified-1", "circle-1", shadowllm.CategoryMoney, shadowgate.OriginShadowOnly, clk)
	unqualified := createUnqualifiedCandidate("unqualified-1", "circle-1", clk)

	source.AddCandidate(qualified)
	source.AddCandidate(unqualified)

	// Add intents for both
	for _, c := range []*shadowgate.Candidate{qualified, unqualified} {
		intent := shadowgate.PromotionIntent{
			CandidateID:   c.ID,
			CandidateHash: c.Hash,
			PeriodKey:     "2024-01-15",
			NoteCode:      shadowgate.NotePromoteRule,
			CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
			CreatedAt:     clk.Now(),
		}
		intent.IntentID = intent.ComputeID()
		intent.IntentHash = intent.ComputeHash()
		source.AddIntent(intent)
	}

	output, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Only qualified should be included
	if output.QualifiedIntents != 1 {
		t.Errorf("Expected 1 qualified intent, got %d", output.QualifiedIntents)
	}
	if output.SkippedIntents != 1 {
		t.Errorf("Expected 1 skipped intent, got %d", output.SkippedIntents)
	}
	if len(output.Pack.Changes) != 1 {
		t.Errorf("Expected 1 change in pack, got %d", len(output.Pack.Changes))
	}

	t.Log("Gating verified: unqualified candidates excluded")
}

// =============================================================================
// Test 5: Replay - Storelog Replay Restores State
// =============================================================================

func TestReplay_StorelogRestoresState(t *testing.T) {
	clk := createTestClock()

	// Create original store and add pack
	store1 := persist.NewRulePackStore(clk.Now)

	pack := &rulepack.RulePack{
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     rulepack.FiveMinuteBucket(clk.Now()),
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes: []rulepack.RuleChange{
			{
				CandidateHash:        "hash1",
				IntentHash:           "intent1",
				CircleID:             "circle-1",
				ChangeKind:           rulepack.ChangeBiasAdjust,
				TargetScope:          rulepack.ScopeCategory,
				TargetHash:           "target1",
				Category:             shadowllm.CategoryMoney,
				SuggestedDelta:       rulepack.DeltaMedium,
				UsefulnessBucket:     shadowgate.UsefulnessHigh,
				VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
				NoveltyBucket:        rulepack.NoveltyShadowOnly,
				AgreementBucket:      rulepack.AgreementMatch,
			},
		},
		CreatedAt: clk.Now(),
	}
	pack.PackID = pack.ComputeID()
	pack.PackHash = pack.ComputeHash()
	for i := range pack.Changes {
		pack.Changes[i].ChangeID = pack.Changes[i].ComputeID()
	}

	if err := store1.AppendPack(pack); err != nil {
		t.Fatalf("Failed to append pack: %v", err)
	}

	// Add an ack
	if err := store1.AckPack(pack.PackID, rulepack.AckViewed); err != nil {
		t.Fatalf("Failed to ack pack: %v", err)
	}

	// Convert to storelog records
	packRecord := store1.PackToStorelogRecord(pack)

	// Create new store and replay
	store2 := persist.NewRulePackStore(clk.Now)
	if err := store2.ReplayPackRecord(packRecord); err != nil {
		t.Fatalf("Failed to replay pack: %v", err)
	}

	// Verify pack was restored
	restored, ok := store2.GetPack(pack.PackID)
	if !ok {
		t.Fatal("Replayed pack not found")
	}
	if restored.PackHash != pack.PackHash {
		t.Errorf("Replayed pack hash mismatch: %s vs %s", restored.PackHash, pack.PackHash)
	}
	if len(restored.Changes) != len(pack.Changes) {
		t.Errorf("Replayed pack change count mismatch: %d vs %d", len(restored.Changes), len(pack.Changes))
	}

	t.Log("Replay verified: storelog restores pack state")
}

// =============================================================================
// Test 6: Export Format Validation
// =============================================================================

func TestExportFormatValidation(t *testing.T) {
	clk := createTestClock()

	pack := &rulepack.RulePack{
		PackID:              "pack-123",
		PackHash:            "hash-abc",
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     rulepack.FiveMinuteBucket(clk.Now()),
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes: []rulepack.RuleChange{
			{
				ChangeID:             "change-1",
				CandidateHash:        "hash1",
				IntentHash:           "intent1",
				CircleID:             "circle-1",
				ChangeKind:           rulepack.ChangeBiasAdjust,
				TargetScope:          rulepack.ScopeCategory,
				Category:             shadowllm.CategoryMoney,
				SuggestedDelta:       rulepack.DeltaMedium,
				UsefulnessBucket:     shadowgate.UsefulnessHigh,
				VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
				NoveltyBucket:        rulepack.NoveltyShadowOnly,
				AgreementBucket:      rulepack.AgreementMatch,
			},
		},
		CreatedAt: clk.Now(),
	}

	text := pack.ToText()

	// Verify header
	if !hasPrefix(text, rulepack.ExportHeader) {
		t.Error("Export missing header")
	}

	// Verify PACK line
	if !containsLine(text, "PACK|") {
		t.Error("Export missing PACK line")
	}

	// Verify CHANGE line
	if !containsLine(text, "CHANGE|") {
		t.Error("Export missing CHANGE line")
	}

	// Verify footer
	if !containsLine(text, "# END") {
		t.Error("Export missing END footer")
	}

	t.Log("Export format validation passed")
}

func hasPrefix(text, prefix string) bool {
	return len(text) >= len(prefix) && text[:len(prefix)] == prefix
}

func containsLine(text, pattern string) bool {
	lines := splitLines(text)
	for _, line := range lines {
		if hasPrefix(line, pattern) {
			return true
		}
	}
	return false
}

func splitLines(text string) []string {
	var lines []string
	var current []byte
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, string(current))
			current = nil
		} else {
			current = append(current, text[i])
		}
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return lines
}

// =============================================================================
// Test 7: ParseText Round-Trip
// =============================================================================

func TestParseText_RoundTrip(t *testing.T) {
	clk := createTestClock()

	original := &rulepack.RulePack{
		PackID:              "pack-abc",
		PackHash:            "hash-xyz",
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes: []rulepack.RuleChange{
			{
				ChangeID:             "change-1",
				CandidateHash:        "cand-hash",
				IntentHash:           "intent-hash",
				CircleID:             "circle-1",
				ChangeKind:           rulepack.ChangeThresholdAdjust,
				TargetScope:          rulepack.ScopeCategory,
				TargetHash:           "target-hash",
				Category:             shadowllm.CategoryWork,
				SuggestedDelta:       rulepack.DeltaLarge,
				UsefulnessBucket:     shadowgate.UsefulnessHigh,
				VoteConfidenceBucket: shadowgate.VoteConfidenceHigh,
				NoveltyBucket:        rulepack.NoveltyCanonOnly,
				AgreementBucket:      rulepack.AgreementSofter,
			},
		},
		CreatedAt: clk.Now(),
	}

	// Export to text
	text := original.ToText()

	// Parse back
	parsed, err := rulepack.ParseText(text)
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}

	// Verify key fields
	if parsed.PackID != original.PackID {
		t.Errorf("PackID mismatch: %s vs %s", parsed.PackID, original.PackID)
	}
	if parsed.PeriodKey != original.PeriodKey {
		t.Errorf("PeriodKey mismatch: %s vs %s", parsed.PeriodKey, original.PeriodKey)
	}
	if len(parsed.Changes) != len(original.Changes) {
		t.Errorf("Change count mismatch: %d vs %d", len(parsed.Changes), len(original.Changes))
	}

	if len(parsed.Changes) > 0 {
		pc := parsed.Changes[0]
		oc := original.Changes[0]
		if pc.ChangeKind != oc.ChangeKind {
			t.Errorf("ChangeKind mismatch: %s vs %s", pc.ChangeKind, oc.ChangeKind)
		}
		if pc.Category != oc.Category {
			t.Errorf("Category mismatch: %s vs %s", pc.Category, oc.Category)
		}
	}

	t.Log("ParseText round-trip verified")
}

// =============================================================================
// Test 8: Ack Recording and Retrieval
// =============================================================================

func TestAckRecordingAndRetrieval(t *testing.T) {
	clk := createTestClock()
	store := persist.NewRulePackStore(clk.Now)

	// Create and store a pack
	pack := &rulepack.RulePack{
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     rulepack.FiveMinuteBucket(clk.Now()),
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes:             []rulepack.RuleChange{},
		CreatedAt:           clk.Now(),
	}
	pack.PackID = pack.ComputeID()
	pack.PackHash = pack.ComputeHash()

	if err := store.AppendPack(pack); err != nil {
		t.Fatalf("Failed to append pack: %v", err)
	}

	// Add acks
	ackKinds := []rulepack.AckKind{rulepack.AckViewed, rulepack.AckExported}
	for _, kind := range ackKinds {
		if err := store.AckPack(pack.PackID, kind); err != nil {
			t.Fatalf("Failed to ack pack with %s: %v", kind, err)
		}
	}

	// Verify acks
	acks := store.GetAcksForPack(pack.PackID)
	if len(acks) != 2 {
		t.Errorf("Expected 2 acks, got %d", len(acks))
	}

	// Verify HasAckKind
	if !store.HasAckKind(pack.PackID, rulepack.AckViewed) {
		t.Error("Expected HasAckKind(viewed) to return true")
	}
	if !store.HasAckKind(pack.PackID, rulepack.AckExported) {
		t.Error("Expected HasAckKind(exported) to return true")
	}
	if store.HasAckKind(pack.PackID, rulepack.AckDismissed) {
		t.Error("Expected HasAckKind(dismissed) to return false")
	}

	t.Log("Ack recording and retrieval verified")
}

// =============================================================================
// Test 9: Change Magnitude Buckets
// =============================================================================

func TestChangeMagnitudeBuckets(t *testing.T) {
	tests := []struct {
		changeCount int
		expected    shadowllm.MagnitudeBucket
	}{
		{0, shadowllm.MagnitudeNothing},
		{1, shadowllm.MagnitudeAFew},
		{3, shadowllm.MagnitudeAFew},
		{4, shadowllm.MagnitudeSeveral},
		{10, shadowllm.MagnitudeSeveral},
	}

	for _, tt := range tests {
		pack := &rulepack.RulePack{
			Changes: make([]rulepack.RuleChange, tt.changeCount),
		}
		got := pack.ChangeMagnitude()
		if got != tt.expected {
			t.Errorf("ChangeMagnitude(%d changes) = %s, want %s", tt.changeCount, got, tt.expected)
		}
	}

	t.Log("Change magnitude buckets verified")
}

// =============================================================================
// Test 10: Empty Pack Handling
// =============================================================================

func TestEmptyPackHandling(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	// Empty source with no intents
	source := newMockIntentSource()

	output, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(output.Pack.Changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(output.Pack.Changes))
	}
	if output.TotalIntents != 0 {
		t.Errorf("Expected 0 total intents, got %d", output.TotalIntents)
	}

	// Empty pack should still have valid structure
	if err := output.Pack.Validate(); err != nil {
		t.Errorf("Empty pack validation failed: %v", err)
	}

	// Export should still work
	text := output.Pack.ToText()
	if len(text) == 0 {
		t.Error("Empty pack export produced no text")
	}

	t.Log("Empty pack handling verified")
}

// =============================================================================
// Test 11: FiveMinuteBucket Determinism
// =============================================================================

func TestFiveMinuteBucketDeterminism(t *testing.T) {
	times := []struct {
		input    time.Time
		expected string
	}{
		{time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), "2024-01-15T10:0"},
		{time.Date(2024, 1, 15, 10, 4, 59, 0, time.UTC), "2024-01-15T10:0"},
		{time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC), "2024-01-15T10:5"},
		{time.Date(2024, 1, 15, 10, 9, 59, 0, time.UTC), "2024-01-15T10:5"},
		{time.Date(2024, 1, 15, 10, 10, 0, 0, time.UTC), "2024-01-15T10:10"},
		{time.Date(2024, 1, 15, 10, 59, 59, 0, time.UTC), "2024-01-15T10:55"},
	}

	for _, tt := range times {
		got := rulepack.FiveMinuteBucket(tt.input)
		if got != tt.expected {
			t.Errorf("FiveMinuteBucket(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}

	t.Log("FiveMinuteBucket determinism verified")
}

// =============================================================================
// Test 12: ChangeKind Inference From Origin
// =============================================================================

func TestChangeKindInferenceFromOrigin(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	tests := []struct {
		origin       shadowgate.CandidateOrigin
		expectedKind rulepack.ChangeKind
	}{
		{shadowgate.OriginShadowOnly, rulepack.ChangeBiasAdjust},
		{shadowgate.OriginCanonOnly, rulepack.ChangeThresholdAdjust},
		{shadowgate.OriginConflict, rulepack.ChangeSuppressSuggest},
	}

	for _, tt := range tests {
		source := newMockIntentSource()
		c := createQualifiedCandidate("test-"+string(tt.origin), "circle-1", shadowllm.CategoryMoney, tt.origin, clk)
		source.AddCandidate(c)

		intent := shadowgate.PromotionIntent{
			CandidateID:   c.ID,
			CandidateHash: c.Hash,
			PeriodKey:     "2024-01-15",
			NoteCode:      shadowgate.NotePromoteRule,
			CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
			CreatedAt:     clk.Now(),
		}
		intent.IntentID = intent.ComputeID()
		intent.IntentHash = intent.ComputeHash()
		source.AddIntent(intent)

		output, _ := engine.Build(rulepackengine.BuildInput{
			PeriodKey:    "2024-01-15",
			IntentSource: source,
		})

		if len(output.Pack.Changes) != 1 {
			t.Fatalf("Expected 1 change for %s, got %d", tt.origin, len(output.Pack.Changes))
		}
		if output.Pack.Changes[0].ChangeKind != tt.expectedKind {
			t.Errorf("Origin %s: expected kind %s, got %s", tt.origin, tt.expectedKind, output.Pack.Changes[0].ChangeKind)
		}
	}

	t.Log("ChangeKind inference from origin verified")
}

// =============================================================================
// Test 13: TargetScope Inference From Category
// =============================================================================

func TestTargetScopeInferenceFromCategory(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	categories := []shadowllm.AbstractCategory{
		shadowllm.CategoryMoney,
		shadowllm.CategoryTime,
		shadowllm.CategoryWork,
		shadowllm.CategoryPeople,
		shadowllm.CategoryHome,
	}

	for _, cat := range categories {
		source := newMockIntentSource()
		c := createQualifiedCandidate("test-"+string(cat), "circle-1", cat, shadowgate.OriginShadowOnly, clk)
		source.AddCandidate(c)

		intent := shadowgate.PromotionIntent{
			CandidateID:   c.ID,
			CandidateHash: c.Hash,
			PeriodKey:     "2024-01-15",
			NoteCode:      shadowgate.NotePromoteRule,
			CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
			CreatedAt:     clk.Now(),
		}
		intent.IntentID = intent.ComputeID()
		intent.IntentHash = intent.ComputeHash()
		source.AddIntent(intent)

		output, _ := engine.Build(rulepackengine.BuildInput{
			PeriodKey:    "2024-01-15",
			IntentSource: source,
		})

		if len(output.Pack.Changes) != 1 {
			t.Fatalf("Expected 1 change for %s, got %d", cat, len(output.Pack.Changes))
		}
		// All known categories should map to ScopeCategory
		if output.Pack.Changes[0].TargetScope != rulepack.ScopeCategory {
			t.Errorf("Category %s: expected scope %s, got %s", cat, rulepack.ScopeCategory, output.Pack.Changes[0].TargetScope)
		}
	}

	t.Log("TargetScope inference from category verified")
}

// =============================================================================
// Test 14: Non-Promote Intents Skipped
// =============================================================================

func TestNonPromoteIntentsSkipped(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	source := newMockIntentSource()
	c := createQualifiedCandidate("candidate-1", "circle-1", shadowllm.CategoryMoney, shadowgate.OriginShadowOnly, clk)
	source.AddCandidate(c)

	// Add non-promote intent (needs_more_votes)
	intent := shadowgate.PromotionIntent{
		CandidateID:   c.ID,
		CandidateHash: c.Hash,
		PeriodKey:     "2024-01-15",
		NoteCode:      shadowgate.NoteNeedsMoreVotes, // Not promote_rule
		CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
		CreatedAt:     clk.Now(),
	}
	intent.IntentID = intent.ComputeID()
	intent.IntentHash = intent.ComputeHash()
	source.AddIntent(intent)

	output, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have 0 changes since the intent was not "promote_rule"
	if len(output.Pack.Changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(output.Pack.Changes))
	}
	if output.SkippedIntents != 1 {
		t.Errorf("Expected 1 skipped intent, got %d", output.SkippedIntents)
	}

	t.Log("Non-promote intents skipping verified")
}

// =============================================================================
// Test 15: Circle Filtering
// =============================================================================

func TestCircleFiltering(t *testing.T) {
	clk := createTestClock()
	engine := rulepackengine.NewEngine(clk)

	source := newMockIntentSource()

	// Add candidates from different circles
	c1 := createQualifiedCandidate("candidate-1", "circle-1", shadowllm.CategoryMoney, shadowgate.OriginShadowOnly, clk)
	c2 := createQualifiedCandidate("candidate-2", "circle-2", shadowllm.CategoryWork, shadowgate.OriginCanonOnly, clk)

	source.AddCandidate(c1)
	source.AddCandidate(c2)

	for _, c := range []*shadowgate.Candidate{c1, c2} {
		intent := shadowgate.PromotionIntent{
			CandidateID:   c.ID,
			CandidateHash: c.Hash,
			PeriodKey:     "2024-01-15",
			NoteCode:      shadowgate.NotePromoteRule,
			CreatedBucket: shadowgate.FiveMinuteBucket(clk.Now()),
			CreatedAt:     clk.Now(),
		}
		intent.IntentID = intent.ComputeID()
		intent.IntentHash = intent.ComputeHash()
		source.AddIntent(intent)
	}

	// Build with circle filter
	output, err := engine.Build(rulepackengine.BuildInput{
		PeriodKey:    "2024-01-15",
		CircleID:     "circle-1", // Filter to circle-1 only
		IntentSource: source,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should only have 1 change (from circle-1)
	if len(output.Pack.Changes) != 1 {
		t.Errorf("Expected 1 change with circle filter, got %d", len(output.Pack.Changes))
	}
	if output.QualifiedIntents != 1 {
		t.Errorf("Expected 1 qualified intent, got %d", output.QualifiedIntents)
	}
	if output.SkippedIntents != 1 {
		t.Errorf("Expected 1 skipped intent, got %d", output.SkippedIntents)
	}

	t.Log("Circle filtering verified")
}

// =============================================================================
// Test 16: Store Pack Count
// =============================================================================

func TestStorePackCount(t *testing.T) {
	clk := createTestClock()
	store := persist.NewRulePackStore(clk.Now)

	if store.GetPackCount() != 0 {
		t.Error("Expected 0 packs initially")
	}

	// Add packs with different circles and buckets to ensure unique IDs
	circles := []string{"circle-1", "circle-2", "circle-3"}
	for i, circle := range circles {
		pack := &rulepack.RulePack{
			PeriodKey:           "2024-01-15",
			CircleID:            identity.EntityID(circle),
			CreatedAtBucket:     "2024-01-15T10:" + rulepack.FiveMinuteBucket(clk.Now().Add(time.Duration(i*5)*time.Minute))[16:],
			ExportFormatVersion: rulepack.ExportFormatVersion,
			Changes:             []rulepack.RuleChange{},
			CreatedAt:           clk.Now(),
		}
		pack.PackID = pack.ComputeID()
		pack.PackHash = pack.ComputeHash()
		_ = store.AppendPack(pack)
	}

	if store.GetPackCount() != 3 {
		t.Errorf("Expected 3 packs, got %d", store.GetPackCount())
	}

	t.Log("Store pack count verified")
}

// =============================================================================
// Test 17: List All Packs Sorted
// =============================================================================

func TestListAllPacksSorted(t *testing.T) {
	store := persist.NewRulePackStore(func() time.Time {
		return time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	})

	// Add packs with different created buckets
	buckets := []string{"2024-01-15T10:0", "2024-01-15T10:30", "2024-01-15T10:15"}
	for _, bucket := range buckets {
		pack := &rulepack.RulePack{
			PeriodKey:           "2024-01-15",
			CircleID:            "circle-1",
			CreatedAtBucket:     bucket,
			ExportFormatVersion: rulepack.ExportFormatVersion,
			Changes:             []rulepack.RuleChange{},
			CreatedAt:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}
		pack.PackID = pack.ComputeID()
		pack.PackHash = pack.ComputeHash()
		_ = store.AppendPack(pack)
	}

	packs := store.ListAllPacks()

	// Should be sorted by CreatedAtBucket desc (most recent first)
	if len(packs) != 3 {
		t.Fatalf("Expected 3 packs, got %d", len(packs))
	}
	if packs[0].CreatedAtBucket != "2024-01-15T10:30" {
		t.Errorf("First pack should be most recent, got %s", packs[0].CreatedAtBucket)
	}
	if packs[2].CreatedAtBucket != "2024-01-15T10:0" {
		t.Errorf("Last pack should be oldest, got %s", packs[2].CreatedAtBucket)
	}

	t.Log("List all packs sorted verified")
}

// =============================================================================
// Test 18: Replay Ack Record
// =============================================================================

func TestReplayAckRecord(t *testing.T) {
	clk := createTestClock()
	store1 := persist.NewRulePackStore(clk.Now)

	// Add pack first
	pack := &rulepack.RulePack{
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     rulepack.FiveMinuteBucket(clk.Now()),
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes:             []rulepack.RuleChange{},
		CreatedAt:           clk.Now(),
	}
	pack.PackID = pack.ComputeID()
	pack.PackHash = pack.ComputeHash()
	_ = store1.AppendPack(pack)

	// Add ack
	_ = store1.AckPack(pack.PackID, rulepack.AckExported)

	// Get the ack
	acks := store1.GetAcksForPack(pack.PackID)
	if len(acks) == 0 {
		t.Fatal("No acks found")
	}
	ack := acks[0]

	// Create storelog record
	ackRecord := store1.AckToStorelogRecord(&ack)
	if ackRecord.Type != persist.RecordTypeRulePackAck {
		t.Errorf("Expected record type %s, got %s", persist.RecordTypeRulePackAck, ackRecord.Type)
	}

	// Replay in new store (with pack already present)
	store2 := persist.NewRulePackStore(clk.Now)
	packRecord := store1.PackToStorelogRecord(pack)
	_ = store2.ReplayPackRecord(packRecord)
	_ = store2.ReplayAckRecord(ackRecord)

	// Verify ack was restored
	if !store2.HasAckKind(pack.PackID, rulepack.AckExported) {
		t.Error("Replayed ack not found")
	}

	t.Log("Replay ack record verified")
}

// =============================================================================
// Test 19: Duplicate Pack Append Fails
// =============================================================================

func TestDuplicatePackAppendFails(t *testing.T) {
	clk := createTestClock()
	store := persist.NewRulePackStore(clk.Now)

	pack := &rulepack.RulePack{
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     rulepack.FiveMinuteBucket(clk.Now()),
		ExportFormatVersion: rulepack.ExportFormatVersion,
		Changes:             []rulepack.RuleChange{},
		CreatedAt:           clk.Now(),
	}
	pack.PackID = pack.ComputeID()
	pack.PackHash = pack.ComputeHash()

	// First append should succeed
	if err := store.AppendPack(pack); err != nil {
		t.Fatalf("First append failed: %v", err)
	}

	// Second append should fail
	err := store.AppendPack(pack)
	if err == nil {
		t.Error("Expected error on duplicate append")
	}
	if err != storelog.ErrRecordExists {
		t.Errorf("Expected ErrRecordExists, got %v", err)
	}

	t.Log("Duplicate pack append fails verified")
}
