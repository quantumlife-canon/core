// Package demo_phase34_interrupt_preview contains demo tests for Phase 34.
//
// Phase 34: Permitted Interrupt Preview (Web-only, No External Signals)
//
// CRITICAL INVARIANTS:
//   - NO external signals (no notifications, no emails, no SMS, no push, no OS alerts)
//   - Web-only preview. User-initiated.
//   - Hash-only, bucket-only. No raw identifiers.
//   - Deterministic: same inputs => same outputs + same hashes.
//   - Commerce always excluded.
//   - Single-whisper rule respected.
//
// Reference: docs/ADR/ADR-0070-phase34-interrupt-preview-web-only.md
package demo_phase34_interrupt_preview

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/interruptpreview"
	"quantumlife/internal/persist"
	ip "quantumlife/pkg/domain/interruptpreview"
)

// ═══════════════════════════════════════════════════════════════════════════
// Test 1: Determinism — same inputs produce same outputs + hashes
// ═══════════════════════════════════════════════════════════════════════════

func TestDeterminism_SameInputsSameOutputs(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	// Run twice
	cue1 := engine.BuildCue(input)
	cue2 := engine.BuildCue(input)

	// Verify same status hashes
	if cue1.StatusHash != cue2.StatusHash {
		t.Errorf("determinism violated: cue status hashes differ: %s != %s", cue1.StatusHash, cue2.StatusHash)
	}

	page1 := engine.BuildPage(input)
	page2 := engine.BuildPage(input)

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("determinism violated: page status hashes differ: %s != %s", page1.StatusHash, page2.StatusHash)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 2: No candidates means no cue
// ═══════════════════════════════════════════════════════════════════════════

func TestNoCandidates_NoCue(t *testing.T) {
	engine := interruptpreview.NewEngine()

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: nil,
	}

	cue := engine.BuildCue(input)

	if cue.Available {
		t.Errorf("expected no cue when no candidates, but Available=true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 3: Dismissed state prevents cue
// ═══════════════════════════════════════════════════════════════════════════

func TestDismissed_NoCue(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         true, // dismissed
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	cue := engine.BuildCue(input)

	if cue.Available {
		t.Errorf("expected no cue when dismissed, but Available=true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 4: Held state prevents cue
// ═══════════════════════════════════════════════════════════════════════════

func TestHeld_NoCue(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              true, // held
		PermittedCandidates: candidates,
	}

	cue := engine.BuildCue(input)

	if cue.Available {
		t.Errorf("expected no cue when held, but Available=true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 5: Candidate selection is deterministic by lowest hash
// ═══════════════════════════════════════════════════════════════════════════

func TestCandidateSelection_LowestHash(t *testing.T) {
	engine := interruptpreview.NewEngine()

	// Create candidates with different hashes
	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "zzz999",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
		{
			CandidateHash: "aaa111",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonSoon,
			Magnitude:     ip.MagnitudeSeveral,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	// Select candidate
	selected := engine.SelectCandidate(input)

	if selected == nil {
		t.Fatal("expected a candidate to be selected")
	}

	// The selected candidate should have a deterministic selection hash
	// Run selection multiple times to verify determinism
	for i := 0; i < 5; i++ {
		s := engine.SelectCandidate(input)
		if s.CandidateHash != selected.CandidateHash {
			t.Errorf("selection not deterministic: run %d got %s, expected %s", i, s.CandidateHash, selected.CandidateHash)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 6: Page shows abstract labels only
// ═══════════════════════════════════════════════════════════════════════════

func TestPage_ShowsAbstractLabelsOnly(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	page := engine.BuildPage(input)

	if page == nil {
		t.Fatal("expected page to be built")
	}

	// Verify labels are abstract (not raw identifiers)
	if page.CircleTypeLabel == "" {
		t.Error("expected circle type label to be set")
	}
	if page.HorizonLabel == "" {
		t.Error("expected horizon label to be set")
	}
	if page.MagnitudeLabel == "" {
		t.Error("expected magnitude label to be set")
	}
	if page.ReasonLabel == "" {
		t.Error("expected reason label to be set")
	}

	// Verify title and subtitle are calm
	if page.Title == "" {
		t.Error("expected title to be set")
	}
	if page.Subtitle == "" {
		t.Error("expected subtitle to be set")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 7: Commerce candidates are filtered out
// ═══════════════════════════════════════════════════════════════════════════

func TestFilterCommerce_RemovesCommerceCircles(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "human1",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
		{
			CandidateHash: "commerce1",
			CircleType:    "commerce", // commerce type
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	filtered := engine.FilterCommerce(candidates)

	if len(filtered) != 1 {
		t.Errorf("expected 1 candidate after filtering commerce, got %d", len(filtered))
	}

	if filtered[0].CircleType == "commerce" {
		t.Error("commerce candidate should have been filtered")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 8: Proof page shows correct state
// ═══════════════════════════════════════════════════════════════════════════

func TestProofPage_ShowsCorrectState(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	// Test with candidates available, user dismissed
	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         true,
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	proof := engine.BuildProofPage(input)

	if !proof.PreviewAvailable {
		t.Error("expected PreviewAvailable=true when candidates exist")
	}

	if !proof.UserDismissed {
		t.Error("expected UserDismissed=true when IsDismissed=true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 9: ShouldShowCue returns false for empty candidates
// ═══════════════════════════════════════════════════════════════════════════

func TestShouldShowCue_FalseForEmpty(t *testing.T) {
	engine := interruptpreview.NewEngine()

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: nil,
	}

	if engine.ShouldShowCue(input) {
		t.Error("expected ShouldShowCue=false when no candidates")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 10: ShouldShowCue returns true for non-empty, non-dismissed
// ═══════════════════════════════════════════════════════════════════════════

func TestShouldShowCue_TrueForAvailable(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	if !engine.ShouldShowCue(input) {
		t.Error("expected ShouldShowCue=true when candidates available and not dismissed")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 11: Ack store persists acknowledgments
// ═══════════════════════════════════════════════════════════════════════════

func TestAckStore_PersistsAcknowledgments(t *testing.T) {
	store := persist.NewInterruptPreviewAckStore(persist.DefaultInterruptPreviewAckStoreConfig())

	ack := &ip.PreviewAck{
		CircleIDHash:  "circle-hash-123",
		PeriodKey:     "2026-01-07",
		CandidateHash: "abc123",
		Kind:          ip.AckDismissed,
		AckBucket:     "10:00",
	}
	ack.AckID = ack.ComputeAckID()

	err := store.Append(ack)
	if err != nil {
		t.Fatalf("failed to append ack: %v", err)
	}

	// Verify ack is stored
	retrieved := store.GetAck(ack.AckID)
	if retrieved == nil {
		t.Fatal("expected ack to be retrieved")
	}

	if retrieved.Kind != ip.AckDismissed {
		t.Errorf("expected kind=dismissed, got %s", retrieved.Kind)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 12: IsDismissed returns correct state
// ═══════════════════════════════════════════════════════════════════════════

func TestAckStore_IsDismissed(t *testing.T) {
	store := persist.NewInterruptPreviewAckStore(persist.DefaultInterruptPreviewAckStoreConfig())

	circleIDHash := "circle-hash-123"
	periodKey := "2026-01-07"

	// Initially not dismissed
	if store.IsDismissed(circleIDHash, periodKey) {
		t.Error("expected IsDismissed=false initially")
	}

	// Append dismissed ack
	ack := &ip.PreviewAck{
		CircleIDHash:  circleIDHash,
		PeriodKey:     periodKey,
		CandidateHash: "abc123",
		Kind:          ip.AckDismissed,
		AckBucket:     "10:00",
	}
	ack.AckID = ack.ComputeAckID()

	if err := store.Append(ack); err != nil {
		t.Fatalf("failed to append ack: %v", err)
	}

	// Now should be dismissed
	if !store.IsDismissed(circleIDHash, periodKey) {
		t.Error("expected IsDismissed=true after dismissal")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 13: IsHeld returns correct state
// ═══════════════════════════════════════════════════════════════════════════

func TestAckStore_IsHeld(t *testing.T) {
	store := persist.NewInterruptPreviewAckStore(persist.DefaultInterruptPreviewAckStoreConfig())

	circleIDHash := "circle-hash-123"
	periodKey := "2026-01-07"

	// Initially not held
	if store.IsHeld(circleIDHash, periodKey) {
		t.Error("expected IsHeld=false initially")
	}

	// Append held ack
	ack := &ip.PreviewAck{
		CircleIDHash:  circleIDHash,
		PeriodKey:     periodKey,
		CandidateHash: "abc123",
		Kind:          ip.AckHeld,
		AckBucket:     "10:00",
	}
	ack.AckID = ack.ComputeAckID()

	if err := store.Append(ack); err != nil {
		t.Fatalf("failed to append ack: %v", err)
	}

	// Now should be held
	if !store.IsHeld(circleIDHash, periodKey) {
		t.Error("expected IsHeld=true after hold")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 14: Duplicate acks are rejected
// ═══════════════════════════════════════════════════════════════════════════

func TestAckStore_RejectsDuplicates(t *testing.T) {
	store := persist.NewInterruptPreviewAckStore(persist.DefaultInterruptPreviewAckStoreConfig())

	ack := &ip.PreviewAck{
		CircleIDHash:  "circle-hash-123",
		PeriodKey:     "2026-01-07",
		CandidateHash: "abc123",
		Kind:          ip.AckDismissed,
		AckBucket:     "10:00",
	}
	ack.AckID = ack.ComputeAckID()

	// First append should succeed
	err := store.Append(ack)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Second append should fail
	err = store.Append(ack)
	if err == nil {
		t.Error("expected duplicate append to fail")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 15: Canonical strings are deterministic
// ═══════════════════════════════════════════════════════════════════════════

func TestCanonicalStrings_Deterministic(t *testing.T) {
	ack := &ip.PreviewAck{
		CircleIDHash:  "circle-hash-123",
		PeriodKey:     "2026-01-07",
		CandidateHash: "abc123",
		Kind:          ip.AckDismissed,
		AckBucket:     "10:00",
	}

	s1 := ack.CanonicalString()
	s2 := ack.CanonicalString()

	if s1 != s2 {
		t.Errorf("canonical strings not deterministic: %s != %s", s1, s2)
	}

	// Verify format is pipe-delimited
	if !strings.Contains(s1, "|") {
		t.Error("expected canonical string to be pipe-delimited")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 16: Preview cue has correct defaults
// ═══════════════════════════════════════════════════════════════════════════

func TestPreviewCue_Defaults(t *testing.T) {
	cue := ip.DefaultPreviewCue()

	if cue.Available {
		t.Error("default cue should have Available=false")
	}

	if cue.Priority != ip.DefaultPreviewCuePriority {
		t.Errorf("default priority should be %d, got %d", ip.DefaultPreviewCuePriority, cue.Priority)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 17: Circle type bucket validation
// ═══════════════════════════════════════════════════════════════════════════

func TestCircleTypeBucket_Validation(t *testing.T) {
	validTypes := []ip.CircleTypeBucket{ip.CircleTypeHuman, ip.CircleTypeInstitution}

	for _, ct := range validTypes {
		if err := ct.Validate(); err != nil {
			t.Errorf("expected %s to be valid, got error: %v", ct, err)
		}
	}

	// Invalid type
	invalid := ip.CircleTypeBucket("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid circle type to fail validation")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 18: Horizon bucket validation
// ═══════════════════════════════════════════════════════════════════════════

func TestHorizonBucket_Validation(t *testing.T) {
	validHorizons := []ip.HorizonBucket{ip.HorizonNow, ip.HorizonSoon, ip.HorizonLater}

	for _, h := range validHorizons {
		if err := h.Validate(); err != nil {
			t.Errorf("expected %s to be valid, got error: %v", h, err)
		}
	}

	// Invalid horizon
	invalid := ip.HorizonBucket("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("expected invalid horizon to fail validation")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 19: Magnitude bucket has display labels
// ═══════════════════════════════════════════════════════════════════════════

func TestMagnitudeBucket_DisplayLabels(t *testing.T) {
	magnitudes := []ip.MagnitudeBucket{
		ip.MagnitudeNothing,
		ip.MagnitudeAFew,
		ip.MagnitudeSeveral,
	}

	for _, m := range magnitudes {
		label := m.DisplayLabel()
		if label == "" {
			t.Errorf("expected display label for %s, got empty string", m)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 20: Eviction respects retention period
// ═══════════════════════════════════════════════════════════════════════════

func TestAckStore_Eviction(t *testing.T) {
	cfg := persist.InterruptPreviewAckStoreConfig{
		MaxRetentionDays: 30,
	}
	store := persist.NewInterruptPreviewAckStore(cfg)

	// Add old ack
	oldAck := &ip.PreviewAck{
		CircleIDHash:  "circle-hash-123",
		PeriodKey:     "2020-01-01", // Very old
		CandidateHash: "abc123",
		Kind:          ip.AckDismissed,
		AckBucket:     "10:00",
	}
	oldAck.AckID = oldAck.ComputeAckID()

	if err := store.Append(oldAck); err != nil {
		t.Fatalf("failed to append old ack: %v", err)
	}

	// Explicit eviction with current time
	now := time.Now()
	store.EvictOldPeriods(now)

	// Old ack should be evicted
	periods := store.GetAllPeriods()
	for _, p := range periods {
		if p == "2020-01-01" {
			t.Error("old period should have been evicted")
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 21: Page lines are calm copy
// ═══════════════════════════════════════════════════════════════════════════

func TestPage_LinesAreCalmCopy(t *testing.T) {
	engine := interruptpreview.NewEngine()

	candidates := []*ip.PreviewCandidate{
		{
			CandidateHash: "abc123",
			CircleType:    ip.CircleTypeHuman,
			Horizon:       ip.HorizonNow,
			Magnitude:     ip.MagnitudeAFew,
			ReasonBucket:  ip.ReasonPolicyAllows,
			Allowance:     ip.AllowanceHumansNow,
		},
	}

	input := &ip.PreviewInput{
		PeriodKey:           "2026-01-07",
		CircleIDHash:        "circle-hash-123",
		IsDismissed:         false,
		IsHeld:              false,
		PermittedCandidates: candidates,
	}

	page := engine.BuildPage(input)

	if page == nil {
		t.Fatal("expected page to be built")
	}

	if len(page.Lines) == 0 {
		t.Error("expected page to have lines")
	}

	// Verify lines don't contain identifiers
	forbiddenPatterns := []string{"@", "http://", "https://", "$", "€", "£"}
	for _, line := range page.Lines {
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(line, pattern) {
				t.Errorf("line contains forbidden pattern %s: %s", pattern, line)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 22: ComputePermittedCandidates validates input
// ═══════════════════════════════════════════════════════════════════════════

func TestComputePermittedCandidates_ValidatesInput(t *testing.T) {
	engine := interruptpreview.NewEngine()

	// Empty input
	result := engine.ComputePermittedCandidates(nil, nil, nil, nil, nil, nil)
	if result != nil {
		t.Error("expected nil for empty input")
	}

	// Mismatched array lengths
	hashes := []string{"h1", "h2"}
	circleTypes := []string{string(ip.CircleTypeHuman)} // Only 1

	result = engine.ComputePermittedCandidates(
		hashes,
		circleTypes,
		[]string{string(ip.HorizonNow), string(ip.HorizonSoon)},
		[]string{string(ip.MagnitudeAFew), string(ip.MagnitudeSeveral)},
		[]string{string(ip.ReasonPolicyAllows), string(ip.ReasonPolicyAllows)},
		[]string{string(ip.AllowanceHumansNow), string(ip.AllowanceHumansNow)},
	)

	// Should only return 1 candidate (minimum of all array lengths)
	if len(result) != 1 {
		t.Errorf("expected 1 candidate for mismatched lengths, got %d", len(result))
	}
}
