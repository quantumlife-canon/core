// Package demo_phase43_held_proof demonstrates Phase 43 Held Under Agreement Proof Ledger.
//
// These tests verify signal creation, page building, cue computation, and commerce exclusion.
//
// CRITICAL INVARIANTS:
//   - NO goroutines. NO time.Now() - clock injection only.
//   - Proof-only. No decisions. No behavior changes.
//   - Commerce excluded. Max 3 signals per page. Max 1 per circle type.
//   - Deterministic: same inputs + same clock => same hashes.
//   - Bounded retention: 30 days OR max records, FIFO eviction.
//
// Reference: docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md
package demo_phase43_held_proof

import (
	"testing"
	"time"

	engine "quantumlife/internal/heldproof"
	"quantumlife/internal/persist"
	hp "quantumlife/pkg/domain/heldproof"
)

// ============================================================================
// Stub Implementations
// ============================================================================

// StubClock provides deterministic time for testing.
type StubClock struct {
	FixedTime time.Time
}

func (c *StubClock) Now() time.Time {
	return c.FixedTime
}

// StubSignalStore adapts persist store to engine interface.
type StubSignalStore struct {
	store *persist.HeldProofSignalStore
	clk   *StubClock
}

func (s *StubSignalStore) AppendSignal(dayKey string, sig hp.HeldProofSignal) error {
	return s.store.AppendSignal(dayKey, sig, s.clk.Now())
}

func (s *StubSignalStore) ListSignals(dayKey string) []hp.HeldProofSignal {
	return s.store.ListSignals(dayKey)
}

// StubAckStore adapts persist store to engine interface.
type StubAckStore struct {
	store *persist.HeldProofAckStore
	clk   *StubClock
}

func (s *StubAckStore) RecordViewed(dayKey, statusHash string) error {
	return s.store.RecordViewed(dayKey, statusHash, s.clk.Now())
}

func (s *StubAckStore) RecordDismissed(dayKey, statusHash string) error {
	return s.store.RecordDismissed(dayKey, statusHash, s.clk.Now())
}

func (s *StubAckStore) IsDismissed(dayKey, statusHash string) bool {
	return s.store.IsDismissed(dayKey, statusHash)
}

func (s *StubAckStore) HasViewed(dayKey, statusHash string) bool {
	return s.store.HasViewed(dayKey, statusHash)
}

// ============================================================================
// Test 1: Determinism - same signal inputs => same evidence hash
// ============================================================================

func TestDeterminism_SameInputsSameHash(t *testing.T) {
	dayKey := "2026-01-08"
	kind := hp.KindDelegatedHolding
	circleType := hp.CircleTypeHuman
	horizon := hp.HorizonSoon
	magnitude := hp.MagnitudeAFew
	sourceHash := "abc123def456"

	hash1 := hp.ComputeEvidenceHash(dayKey, kind, circleType, horizon, magnitude, sourceHash)
	hash2 := hp.ComputeEvidenceHash(dayKey, kind, circleType, horizon, magnitude, sourceHash)

	if hash1 != hash2 {
		t.Errorf("expected same hash, got %s vs %s", hash1, hash2)
	}

	if hash1 == "" {
		t.Error("expected non-empty hash")
	}

	// Length should be 64 (sha256 hex)
	if len(hash1) != 64 {
		t.Errorf("expected 64 char hash, got %d", len(hash1))
	}
}

// ============================================================================
// Test 2: Commerce Exclusion - commerce signals rejected
// ============================================================================

func TestCommerceExclusion_SignalRejected(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	dayKey := "2026-01-08"
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	// Try to create signal for commerce
	outcome := engine.Phase42QueueProofOutcome{
		CircleType: hp.CircleTypeCommerce, // Commerce!
		Horizon:    hp.HorizonSoon,
		Magnitude:  hp.MagnitudeAFew,
		SourceHash: "commerce_source_hash",
	}

	sig := eng.HandleQueueProofOutcome(dayKey, outcome)

	if sig != nil {
		t.Error("expected commerce signal to be rejected (nil)")
	}

	// Verify nothing was stored
	signals := signalStore.ListSignals(dayKey)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals, got %d", len(signals))
	}
}

// ============================================================================
// Test 3: Human Signal Created Successfully
// ============================================================================

func TestHumanSignal_Created(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	dayKey := "2026-01-08"
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	outcome := engine.Phase42QueueProofOutcome{
		CircleType: hp.CircleTypeHuman,
		Horizon:    hp.HorizonSoon,
		Magnitude:  hp.MagnitudeAFew,
		SourceHash: "human_source_hash",
	}

	sig := eng.HandleQueueProofOutcome(dayKey, outcome)

	if sig == nil {
		t.Fatal("expected signal to be created")
	}

	if sig.CircleType != hp.CircleTypeHuman {
		t.Errorf("expected CircleTypeHuman, got %s", sig.CircleType)
	}

	if sig.Kind != hp.KindDelegatedHolding {
		t.Errorf("expected KindDelegatedHolding, got %s", sig.Kind)
	}

	if sig.EvidenceHash == "" {
		t.Error("expected non-empty evidence hash")
	}

	// Verify stored
	signals := signalStore.ListSignals(dayKey)
	if len(signals) != 1 {
		t.Errorf("expected 1 signal, got %d", len(signals))
	}
}

// ============================================================================
// Test 4: Institution Signal Created Successfully
// ============================================================================

func TestInstitutionSignal_Created(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	dayKey := "2026-01-08"
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	outcome := engine.Phase42QueueProofOutcome{
		CircleType: hp.CircleTypeInstitution,
		Horizon:    hp.HorizonLater,
		Magnitude:  hp.MagnitudeSeveral,
		SourceHash: "institution_source_hash",
	}

	sig := eng.HandleQueueProofOutcome(dayKey, outcome)

	if sig == nil {
		t.Fatal("expected signal to be created")
	}

	if sig.CircleType != hp.CircleTypeInstitution {
		t.Errorf("expected CircleTypeInstitution, got %s", sig.CircleType)
	}
}

// ============================================================================
// Test 5: Page Building - Empty Signals = Nil Page
// ============================================================================

func TestBuildPage_EmptySignals(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	var signals []hp.HeldProofSignal
	period := hp.HeldProofPeriod{DayKey: "2026-01-08"}

	page, statusHash, err := eng.BuildPage(signals, period)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if page != nil {
		t.Error("expected nil page for empty signals")
	}

	if statusHash != "" {
		t.Error("expected empty status hash for nil page")
	}
}

// ============================================================================
// Test 6: Page Building - Single Signal
// ============================================================================

func TestBuildPage_SingleSignal(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	signals := []hp.HeldProofSignal{
		{
			Kind:         hp.KindDelegatedHolding,
			CircleType:   hp.CircleTypeHuman,
			Horizon:      hp.HorizonSoon,
			Magnitude:    hp.MagnitudeAFew,
			EvidenceHash: "hash_abc123",
		},
	}
	period := hp.HeldProofPeriod{DayKey: "2026-01-08"}

	page, statusHash, err := eng.BuildPage(signals, period)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if page == nil {
		t.Fatal("expected non-nil page")
	}

	if page.Title != hp.DefaultTitle {
		t.Errorf("expected title %q, got %q", hp.DefaultTitle, page.Title)
	}

	if statusHash == "" {
		t.Error("expected non-empty status hash")
	}

	if len(page.Chips) != 1 {
		t.Errorf("expected 1 chip, got %d", len(page.Chips))
	}
}

// ============================================================================
// Test 7: Page Building - Multiple Signals, Max 1 Per Circle Type
// ============================================================================

func TestBuildPage_MaxOnePerCircleType(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	// Two human signals - should only keep first
	signals := []hp.HeldProofSignal{
		{
			Kind:         hp.KindDelegatedHolding,
			CircleType:   hp.CircleTypeHuman,
			Horizon:      hp.HorizonSoon,
			Magnitude:    hp.MagnitudeAFew,
			EvidenceHash: "hash_human_1",
		},
		{
			Kind:         hp.KindDelegatedHolding,
			CircleType:   hp.CircleTypeHuman, // Same circle type!
			Horizon:      hp.HorizonLater,
			Magnitude:    hp.MagnitudeSeveral,
			EvidenceHash: "hash_human_2",
		},
	}
	period := hp.HeldProofPeriod{DayKey: "2026-01-08"}

	page, _, err := eng.BuildPage(signals, period)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if page == nil {
		t.Fatal("expected non-nil page")
	}

	// Should only have 1 chip (max 1 per circle type)
	if len(page.Chips) != 1 {
		t.Errorf("expected 1 chip (max 1 per circle type), got %d", len(page.Chips))
	}
}

// ============================================================================
// Test 8: Page Building - Max 3 Signals Cap
// ============================================================================

func TestBuildPage_Max3SignalsCap(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	eng := engine.NewEngine(nil, nil, clk)

	// Build signals input with 4 different circle types (only 2 valid: human, institution)
	inputs := engine.HeldProofInputs{
		DayKey: "2026-01-08",
		Decisions: []engine.HeldProofDecisionInput{
			{
				CircleType:  hp.CircleTypeHuman,
				Horizon:     hp.HorizonSoon,
				Magnitude:   hp.MagnitudeAFew,
				QueuedProof: true,
				SourceHash:  "human_hash",
			},
			{
				CircleType:  hp.CircleTypeInstitution,
				Horizon:     hp.HorizonLater,
				Magnitude:   hp.MagnitudeSeveral,
				QueuedProof: true,
				SourceHash:  "inst_hash",
			},
			{
				CircleType:  hp.CircleTypeUnknown,
				Horizon:     hp.HorizonNow,
				Magnitude:   hp.MagnitudeNothing,
				QueuedProof: true,
				SourceHash:  "unknown_hash",
			},
		},
	}

	signals, err := eng.BuildSignals(inputs)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Should have 3 signals (human, institution, unknown)
	if len(signals) > hp.MaxSignalsPerPage {
		t.Errorf("expected max %d signals, got %d", hp.MaxSignalsPerPage, len(signals))
	}
}

// ============================================================================
// Test 9: Cue Building - Available When Signals Exist
// ============================================================================

func TestBuildCue_AvailableWhenSignalsExist(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	eng := engine.NewEngine(nil, nil, clk)

	page := &hp.HeldProofPage{
		Title:      hp.DefaultTitle,
		Line:       hp.LineAFew,
		Chips:      []string{"human"},
		Magnitude:  hp.MagnitudeAFew,
		StatusHash: "abc123",
	}

	cue := eng.BuildCue(page, false, false)

	if cue == nil {
		t.Fatal("expected non-nil cue")
	}

	if !cue.Available {
		t.Error("expected cue to be available")
	}

	if cue.CueText != hp.DefaultCueText {
		t.Errorf("expected cue text %q, got %q", hp.DefaultCueText, cue.CueText)
	}

	if cue.Path != hp.DefaultPath {
		t.Errorf("expected path %q, got %q", hp.DefaultPath, cue.Path)
	}
}

// ============================================================================
// Test 10: Cue Building - Not Available When Dismissed
// ============================================================================

func TestBuildCue_NotAvailableWhenDismissed(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	eng := engine.NewEngine(nil, nil, clk)

	page := &hp.HeldProofPage{
		Title:      hp.DefaultTitle,
		Line:       hp.LineAFew,
		Chips:      []string{"human"},
		Magnitude:  hp.MagnitudeAFew,
		StatusHash: "abc123",
	}

	cue := eng.BuildCue(page, true, false) // dismissed=true

	if cue == nil {
		t.Fatal("expected non-nil cue")
	}

	if cue.Available {
		t.Error("expected cue to NOT be available when dismissed")
	}
}

// ============================================================================
// Test 11: Cue Building - Not Available When Viewed
// ============================================================================

func TestBuildCue_NotAvailableWhenViewed(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	eng := engine.NewEngine(nil, nil, clk)

	page := &hp.HeldProofPage{
		Title:      hp.DefaultTitle,
		Line:       hp.LineAFew,
		Chips:      []string{"human"},
		Magnitude:  hp.MagnitudeAFew,
		StatusHash: "abc123",
	}

	cue := eng.BuildCue(page, false, true) // viewed=true

	if cue == nil {
		t.Fatal("expected non-nil cue")
	}

	if cue.Available {
		t.Error("expected cue to NOT be available after viewing")
	}
}

// ============================================================================
// Test 12: Cue Building - Nil Page Returns Nil
// ============================================================================

func TestBuildCue_NilPageReturnsNil(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	clk := &StubClock{FixedTime: now}

	eng := engine.NewEngine(nil, nil, clk)

	cue := eng.BuildCue(nil, false, false)

	if cue != nil {
		t.Error("expected nil cue for nil page")
	}
}

// ============================================================================
// Test 13: Store Deduplication - Same Signal Not Duplicated
// ============================================================================

func TestStoreDedup_SameSignalNotDuplicated(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	dayKey := "2026-01-08"
	clk := &StubClock{FixedTime: now}

	signalStore := persist.NewHeldProofSignalStore(nil)
	ackStore := persist.NewHeldProofAckStore(nil)

	eng := engine.NewEngine(
		&StubSignalStore{store: signalStore, clk: clk},
		&StubAckStore{store: ackStore, clk: clk},
		clk,
	)

	outcome := engine.Phase42QueueProofOutcome{
		CircleType: hp.CircleTypeHuman,
		Horizon:    hp.HorizonSoon,
		Magnitude:  hp.MagnitudeAFew,
		SourceHash: "same_source_hash",
	}

	// Add same signal twice
	sig1 := eng.HandleQueueProofOutcome(dayKey, outcome)
	sig2 := eng.HandleQueueProofOutcome(dayKey, outcome)

	if sig1 == nil {
		t.Fatal("expected first signal to be created")
	}

	// Second should still return a signal (handled gracefully)
	if sig2 != nil && sig2.EvidenceHash != sig1.EvidenceHash {
		t.Error("expected same evidence hash for deduped signal")
	}

	// Should only have 1 signal in store
	signals := signalStore.ListSignals(dayKey)
	if len(signals) != 1 {
		t.Errorf("expected 1 signal (deduped), got %d", len(signals))
	}
}

// ============================================================================
// Test 14: Ack Store - View and Dismiss Work
// ============================================================================

func TestAckStore_ViewAndDismiss(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	dayKey := "2026-01-08"
	statusHash := "status_hash_abc"

	ackStore := persist.NewHeldProofAckStore(nil)

	// Initially not viewed or dismissed
	if ackStore.HasViewed(dayKey, statusHash) {
		t.Error("expected not viewed initially")
	}
	if ackStore.IsDismissed(dayKey, statusHash) {
		t.Error("expected not dismissed initially")
	}

	// Record view
	if err := ackStore.RecordViewed(dayKey, statusHash, now); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !ackStore.HasViewed(dayKey, statusHash) {
		t.Error("expected viewed after RecordViewed")
	}

	// Record dismiss
	if err := ackStore.RecordDismissed(dayKey, statusHash, now); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !ackStore.IsDismissed(dayKey, statusHash) {
		t.Error("expected dismissed after RecordDismissed")
	}
}

// ============================================================================
// Test 15: Enum Validation - All Valid Values Pass
// ============================================================================

func TestEnumValidation_ValidValuesPass(t *testing.T) {
	// HeldProofKind
	if err := hp.KindDelegatedHolding.Validate(); err != nil {
		t.Errorf("KindDelegatedHolding should be valid: %v", err)
	}

	// HeldProofCircleType
	for _, ct := range hp.AllCircleTypes() {
		if err := ct.Validate(); err != nil {
			t.Errorf("%s should be valid: %v", ct, err)
		}
	}

	// HeldProofMagnitudeBucket
	for _, m := range hp.AllMagnitudeBuckets() {
		if err := m.Validate(); err != nil {
			t.Errorf("%s should be valid: %v", m, err)
		}
	}

	// HeldProofHorizonBucket
	for _, h := range hp.AllHorizonBuckets() {
		if err := h.Validate(); err != nil {
			t.Errorf("%s should be valid: %v", h, err)
		}
	}

	// HeldProofAckKind
	for _, a := range hp.AllAckKinds() {
		if err := a.Validate(); err != nil {
			t.Errorf("%s should be valid: %v", a, err)
		}
	}
}

// ============================================================================
// Test 16: Signal Validate Method
// ============================================================================

func TestSignalValidate_ValidSignalPasses(t *testing.T) {
	sig := hp.HeldProofSignal{
		Kind:         hp.KindDelegatedHolding,
		CircleType:   hp.CircleTypeHuman,
		Horizon:      hp.HorizonSoon,
		Magnitude:    hp.MagnitudeAFew,
		EvidenceHash: "abc123",
	}

	if err := sig.Validate(); err != nil {
		t.Errorf("expected valid signal, got error: %v", err)
	}
}

// ============================================================================
// Test 17: Signal Validate Method - Missing Evidence Hash
// ============================================================================

func TestSignalValidate_MissingEvidenceHash(t *testing.T) {
	sig := hp.HeldProofSignal{
		Kind:         hp.KindDelegatedHolding,
		CircleType:   hp.CircleTypeHuman,
		Horizon:      hp.HorizonSoon,
		Magnitude:    hp.MagnitudeAFew,
		EvidenceHash: "", // Missing!
	}

	if err := sig.Validate(); err == nil {
		t.Error("expected error for missing evidence hash")
	}
}

// ============================================================================
// Test 18: CanonicalString Methods
// ============================================================================

func TestCanonicalString_AllTypes(t *testing.T) {
	// Kind
	if hp.KindDelegatedHolding.CanonicalString() != "heldproof_delegated_holding" {
		t.Error("KindDelegatedHolding canonical string mismatch")
	}

	// CircleType
	if hp.CircleTypeHuman.CanonicalString() != "human" {
		t.Error("CircleTypeHuman canonical string mismatch")
	}

	// Magnitude
	if hp.MagnitudeAFew.CanonicalString() != "a_few" {
		t.Error("MagnitudeAFew canonical string mismatch")
	}

	// Horizon
	if hp.HorizonSoon.CanonicalString() != "soon" {
		t.Error("HorizonSoon canonical string mismatch")
	}

	// AckKind
	if hp.AckViewed.CanonicalString() != "viewed" {
		t.Error("AckViewed canonical string mismatch")
	}
}

// ============================================================================
// Test 19: Line From Magnitude
// ============================================================================

func TestLineFromMagnitude(t *testing.T) {
	line := hp.LineFromMagnitude(hp.MagnitudeAFew)
	if line != hp.LineAFew {
		t.Errorf("expected %q, got %q", hp.LineAFew, line)
	}

	line = hp.LineFromMagnitude(hp.MagnitudeSeveral)
	if line != hp.LineSeveral {
		t.Errorf("expected %q, got %q", hp.LineSeveral, line)
	}
}

// ============================================================================
// Test 20: Magnitude From Count
// ============================================================================

func TestMagnitudeFromCount(t *testing.T) {
	if hp.MagnitudeFromCount(0) != hp.MagnitudeNothing {
		t.Error("count 0 should be MagnitudeNothing")
	}

	if hp.MagnitudeFromCount(1) != hp.MagnitudeAFew {
		t.Error("count 1 should be MagnitudeAFew")
	}

	// 2+ is "several" per implementation
	if hp.MagnitudeFromCount(2) != hp.MagnitudeSeveral {
		t.Error("count 2 should be MagnitudeSeveral")
	}

	if hp.MagnitudeFromCount(3) != hp.MagnitudeSeveral {
		t.Error("count 3 should be MagnitudeSeveral")
	}

	if hp.MagnitudeFromCount(10) != hp.MagnitudeSeveral {
		t.Error("count 10 should be MagnitudeSeveral")
	}
}

// ============================================================================
// Test 21: IsCommerce Check
// ============================================================================

func TestIsCommerce(t *testing.T) {
	if !hp.CircleTypeCommerce.IsCommerce() {
		t.Error("CircleTypeCommerce.IsCommerce() should be true")
	}

	if hp.CircleTypeHuman.IsCommerce() {
		t.Error("CircleTypeHuman.IsCommerce() should be false")
	}

	if hp.CircleTypeInstitution.IsCommerce() {
		t.Error("CircleTypeInstitution.IsCommerce() should be false")
	}
}

// ============================================================================
// Test 22: Page StatusHash Determinism
// ============================================================================

func TestPageStatusHash_Deterministic(t *testing.T) {
	page1 := &hp.HeldProofPage{
		Title:     hp.DefaultTitle,
		Line:      hp.LineAFew,
		Chips:     []string{"human"},
		Magnitude: hp.MagnitudeAFew,
	}
	page1.StatusHash = page1.ComputeHash()

	page2 := &hp.HeldProofPage{
		Title:     hp.DefaultTitle,
		Line:      hp.LineAFew,
		Chips:     []string{"human"},
		Magnitude: hp.MagnitudeAFew,
	}
	page2.StatusHash = page2.ComputeHash()

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("expected same status hash, got %s vs %s",
			page1.StatusHash, page2.StatusHash)
	}
}

// ============================================================================
// Test 23: Store Count Methods
// ============================================================================

func TestStoreCount(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	dayKey := "2026-01-08"

	signalStore := persist.NewHeldProofSignalStore(nil)

	if signalStore.Count() != 0 {
		t.Error("expected initial count 0")
	}

	sig := hp.HeldProofSignal{
		Kind:         hp.KindDelegatedHolding,
		CircleType:   hp.CircleTypeHuman,
		Horizon:      hp.HorizonSoon,
		Magnitude:    hp.MagnitudeAFew,
		EvidenceHash: "hash_1",
	}

	if err := signalStore.AppendSignal(dayKey, sig, now); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if signalStore.Count() != 1 {
		t.Errorf("expected count 1, got %d", signalStore.Count())
	}
}

// ============================================================================
// Test 24: Ack Store Count
// ============================================================================

func TestAckStoreCount(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	ackStore := persist.NewHeldProofAckStore(nil)

	if ackStore.Count() != 0 {
		t.Error("expected initial count 0")
	}

	if err := ackStore.RecordViewed("2026-01-08", "hash1", now); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if ackStore.Count() != 1 {
		t.Errorf("expected count 1, got %d", ackStore.Count())
	}
}

// ============================================================================
// Test 25: Constants Correct
// ============================================================================

func TestConstants(t *testing.T) {
	if hp.MaxRetentionDays != 30 {
		t.Errorf("expected MaxRetentionDays 30, got %d", hp.MaxRetentionDays)
	}

	if hp.MaxSignalRecords != 500 {
		t.Errorf("expected MaxSignalRecords 500, got %d", hp.MaxSignalRecords)
	}

	if hp.MaxAckRecords != 200 {
		t.Errorf("expected MaxAckRecords 200, got %d", hp.MaxAckRecords)
	}

	if hp.MaxSignalsPerPage != 3 {
		t.Errorf("expected MaxSignalsPerPage 3, got %d", hp.MaxSignalsPerPage)
	}
}
