// Package demo_phase39_attention_envelope contains demo tests for Phase 39.
//
// These tests verify:
// - Deterministic envelope building (same clock => same hash)
// - Auto-expiry logic
// - Stop transitions
// - ApplyEnvelope effects (horizon shift, magnitude bias)
// - Bounded effects (max 1 step, +1 bucket)
// - Commerce exclusion (never escalated)
// - No forced interrupts
// - Store constraints (one active per circle, bounded retention)
// - Canonical string stability
// - Enum validation
//
// Reference: docs/ADR/ADR-0076-phase39-attention-envelopes.md
package demo_phase39_attention_envelope

import (
	"testing"
	"time"

	"quantumlife/internal/attentionenvelope"
	"quantumlife/internal/persist"
	ae "quantumlife/pkg/domain/attentionenvelope"
	pd "quantumlife/pkg/domain/pressuredecision"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Test 1: Deterministic BuildEnvelope
// ═══════════════════════════════════════════════════════════════════════════════

func TestDeterministicBuildEnvelope(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Build twice with same inputs
	env1, err1 := engine.BuildEnvelope(
		ae.EnvelopeKindOnCall,
		ae.Duration1h,
		ae.ReasonOnCallDuty,
		"circle_hash_123",
		clock,
	)
	if err1 != nil {
		t.Fatalf("BuildEnvelope 1 failed: %v", err1)
	}

	env2, err2 := engine.BuildEnvelope(
		ae.EnvelopeKindOnCall,
		ae.Duration1h,
		ae.ReasonOnCallDuty,
		"circle_hash_123",
		clock,
	)
	if err2 != nil {
		t.Fatalf("BuildEnvelope 2 failed: %v", err2)
	}

	// Verify same hash
	if env1.EnvelopeID != env2.EnvelopeID {
		t.Errorf("EnvelopeID mismatch: %s != %s", env1.EnvelopeID, env2.EnvelopeID)
	}

	if env1.StatusHash != env2.StatusHash {
		t.Errorf("StatusHash mismatch: %s != %s", env1.StatusHash, env2.StatusHash)
	}

	// Verify canonical string is same
	if env1.CanonicalString() != env2.CanonicalString() {
		t.Errorf("CanonicalString mismatch")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 2: Auto-Expiry Logic
// ═══════════════════════════════════════════════════════════════════════════════

func TestAutoExpiryLogic(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	startClock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Build 15-minute envelope
	env, err := engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration15m,
		ae.ReasonDeadline,
		"circle_hash_abc",
		startClock,
	)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}

	// Should be active at start
	if !engine.IsActive(env, startClock) {
		t.Error("Envelope should be active at start")
	}

	// Should be active 10 minutes later
	clock10m := startClock.Add(10 * time.Minute)
	if !engine.IsActive(env, clock10m) {
		t.Error("Envelope should be active after 10 minutes")
	}

	// Should be expired 15 minutes later (at expiry boundary)
	clock15m := startClock.Add(15 * time.Minute)
	if engine.IsActive(env, clock15m) {
		t.Error("Envelope should be expired at 15 minutes")
	}

	// HasExpired should return true
	if !engine.HasExpired(env, clock15m) {
		t.Error("HasExpired should return true after expiry")
	}

	// Should be expired 20 minutes later
	clock20m := startClock.Add(20 * time.Minute)
	if engine.IsActive(env, clock20m) {
		t.Error("Envelope should be expired after 20 minutes")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 3: Stop Makes Inactive
// ═══════════════════════════════════════════════════════════════════════════════

func TestStopMakesInactive(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	env, err := engine.BuildEnvelope(
		ae.EnvelopeKindOnCall,
		ae.Duration4h,
		ae.ReasonOnCallDuty,
		"circle_hash_xyz",
		clock,
	)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}

	// Should be active initially
	if !engine.IsActive(env, clock) {
		t.Error("Envelope should be active initially")
	}

	// Stop the envelope
	stopped := engine.StopEnvelope(env)

	// Stopped envelope should not be active
	if engine.IsActive(stopped, clock) {
		t.Error("Stopped envelope should not be active")
	}

	// State should be Stopped
	if stopped.State != ae.StateStopped {
		t.Errorf("Expected StateStopped, got %s", stopped.State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 4: ApplyEnvelope No-Op When Inactive
// ═══════════════════════════════════════════════════════════════════════════════

func TestApplyEnvelopeNoOpWhenInactive(t *testing.T) {
	engine := attentionenvelope.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeAFew,
		Horizon:      pd.HorizonSoon,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-15",
	}

	// Apply with nil envelope
	result := engine.ApplyEnvelope(nil, input)
	if result != input {
		t.Error("ApplyEnvelope with nil envelope should return original input")
	}

	// Apply with none kind
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	noneEnv, _ := engine.BuildEnvelope(
		ae.EnvelopeKindNone,
		ae.Duration1h,
		ae.ReasonAwaitingImportant,
		"circle_hash",
		clock,
	)

	result = engine.ApplyEnvelope(noneEnv, input)
	if result != input {
		t.Error("ApplyEnvelope with EnvelopeKindNone should return original input")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 5: Horizon Shift Max 1 Step
// ═══════════════════════════════════════════════════════════════════════════════

func TestHorizonShiftMax1(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create on_call envelope (has horizon shift)
	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindOnCall,
		ae.Duration1h,
		ae.ReasonOnCallDuty,
		"circle_hash",
		clock,
	)

	testCases := []struct {
		name           string
		inputHorizon   pd.PressureHorizon
		expectedResult pd.PressureHorizon
	}{
		{"Later to Soon", pd.HorizonLater, pd.HorizonSoon},
		{"Soon to Now", pd.HorizonSoon, pd.HorizonNow},
		{"Now stays Now", pd.HorizonNow, pd.HorizonNow},
		{"Unknown to Soon", pd.HorizonUnknown, pd.HorizonSoon},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &pd.PressureDecisionInput{
				CircleIDHash: "human_circle",
				CircleType:   pd.CircleTypeHuman,
				Magnitude:    pd.MagnitudeAFew,
				Horizon:      tc.inputHorizon,
				TrustStatus:  pd.TrustStatusNormal,
				PeriodKey:    "2025-01-15",
			}

			result := engine.ApplyEnvelope(env, input)

			if result.Horizon != tc.expectedResult {
				t.Errorf("Expected horizon %s, got %s", tc.expectedResult, result.Horizon)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 6: Magnitude Bias Max +1
// ═══════════════════════════════════════════════════════════════════════════════

func TestMagnitudeBiasMax1(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create working envelope (has magnitude bias only)
	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration1h,
		ae.ReasonDeadline,
		"circle_hash",
		clock,
	)

	testCases := []struct {
		name           string
		inputMagnitude pd.PressureMagnitude
		expectedResult pd.PressureMagnitude
	}{
		{"Nothing to AFew", pd.MagnitudeNothing, pd.MagnitudeAFew},
		{"AFew to Several", pd.MagnitudeAFew, pd.MagnitudeSeveral},
		{"Several stays Several", pd.MagnitudeSeveral, pd.MagnitudeSeveral},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &pd.PressureDecisionInput{
				CircleIDHash: "human_circle",
				CircleType:   pd.CircleTypeHuman,
				Magnitude:    tc.inputMagnitude,
				Horizon:      pd.HorizonSoon,
				TrustStatus:  pd.TrustStatusNormal,
				PeriodKey:    "2025-01-15",
			}

			result := engine.ApplyEnvelope(env, input)

			if result.Magnitude != tc.expectedResult {
				t.Errorf("Expected magnitude %s, got %s", tc.expectedResult, result.Magnitude)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 7: Cap Delta Max +1
// ═══════════════════════════════════════════════════════════════════════════════

func TestCapDeltaMax1(t *testing.T) {
	engine := attentionenvelope.NewEngine()

	testCases := []struct {
		kind     ae.EnvelopeKind
		expected int
	}{
		{ae.EnvelopeKindNone, 0},
		{ae.EnvelopeKindWorking, 0},
		{ae.EnvelopeKindTravel, 0},
		{ae.EnvelopeKindOnCall, 1},
		{ae.EnvelopeKindEmergency, 1},
	}

	for _, tc := range testCases {
		t.Run(string(tc.kind), func(t *testing.T) {
			delta := engine.ComputeCapDelta(tc.kind)

			if delta < 0 || delta > 1 {
				t.Errorf("Cap delta out of bounds: %d", delta)
			}

			if delta != tc.expected {
				t.Errorf("Expected cap delta %d, got %d", tc.expected, delta)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 8: Commerce Exclusion
// ═══════════════════════════════════════════════════════════════════════════════

func TestCommerceExclusion(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create emergency envelope (most aggressive effects)
	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindEmergency,
		ae.Duration1h,
		ae.ReasonFamilyMatter,
		"circle_hash",
		clock,
	)

	// Commerce input
	commerceInput := &pd.PressureDecisionInput{
		CircleIDHash: "commerce_circle",
		CircleType:   pd.CircleTypeCommerce,
		Magnitude:    pd.MagnitudeNothing,
		Horizon:      pd.HorizonLater,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-15",
	}

	result := engine.ApplyEnvelope(env, commerceInput)

	// Commerce should be unchanged - no escalation
	if result.Magnitude != pd.MagnitudeNothing {
		t.Errorf("Commerce magnitude should not be escalated, got %s", result.Magnitude)
	}

	if result.Horizon != pd.HorizonLater {
		t.Errorf("Commerce horizon should not be shifted, got %s", result.Horizon)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 9: ApplyEnvelope Does Not Force Interrupt
// ═══════════════════════════════════════════════════════════════════════════════

func TestApplyEnvelopeDoesNotForceInterrupt(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create emergency envelope
	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindEmergency,
		ae.Duration1h,
		ae.ReasonFamilyMatter,
		"circle_hash",
		clock,
	)

	// Input with no pressure
	input := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeNothing,
		Horizon:      pd.HorizonUnknown,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-15",
	}

	result := engine.ApplyEnvelope(env, input)

	// Result only modifies magnitude and horizon, nothing else
	// Envelope does NOT add any "force interrupt" flag
	if result.CircleIDHash != input.CircleIDHash {
		t.Error("CircleIDHash should not change")
	}

	if result.CircleType != input.CircleType {
		t.Error("CircleType should not change")
	}

	if result.TrustStatus != input.TrustStatus {
		t.Error("TrustStatus should not change")
	}

	if result.PeriodKey != input.PeriodKey {
		t.Error("PeriodKey should not change")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 10: Envelope Effects by Kind
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnvelopeKindEffects(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Base input: Later horizon, Nothing magnitude
	baseInput := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeNothing,
		Horizon:      pd.HorizonLater,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-15",
	}

	testCases := []struct {
		kind              ae.EnvelopeKind
		expectedHorizon   pd.PressureHorizon
		expectedMagnitude pd.PressureMagnitude
	}{
		{ae.EnvelopeKindNone, pd.HorizonLater, pd.MagnitudeNothing},
		{ae.EnvelopeKindWorking, pd.HorizonLater, pd.MagnitudeAFew},  // +1 magnitude only
		{ae.EnvelopeKindTravel, pd.HorizonSoon, pd.MagnitudeNothing}, // +1 horizon only
		{ae.EnvelopeKindOnCall, pd.HorizonSoon, pd.MagnitudeAFew},    // +1 both
		{ae.EnvelopeKindEmergency, pd.HorizonSoon, pd.MagnitudeAFew}, // +1 both
	}

	for _, tc := range testCases {
		t.Run(string(tc.kind), func(t *testing.T) {
			env, _ := engine.BuildEnvelope(
				tc.kind,
				ae.Duration1h,
				ae.ReasonAwaitingImportant,
				"circle_hash",
				clock,
			)

			result := engine.ApplyEnvelope(env, baseInput)

			if result.Horizon != tc.expectedHorizon {
				t.Errorf("Expected horizon %s, got %s", tc.expectedHorizon, result.Horizon)
			}

			if result.Magnitude != tc.expectedMagnitude {
				t.Errorf("Expected magnitude %s, got %s", tc.expectedMagnitude, result.Magnitude)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 11: Store One Active Per Circle
// ═══════════════════════════════════════════════════════════════════════════════

func TestOneActiveEnvelopePerCircle(t *testing.T) {
	store := persist.NewAttentionEnvelopeStore(persist.DefaultAttentionEnvelopeStoreConfig())
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	circleID := "circle_123"

	// Create and start first envelope
	env1, _ := engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration1h,
		ae.ReasonDeadline,
		circleID,
		clock,
	)
	err := store.StartEnvelope(env1)
	if err != nil {
		t.Fatalf("First StartEnvelope failed: %v", err)
	}

	// Create and start second envelope (should replace first)
	env2, _ := engine.BuildEnvelope(
		ae.EnvelopeKindOnCall,
		ae.Duration4h,
		ae.ReasonOnCallDuty,
		circleID,
		clock,
	)
	err = store.StartEnvelope(env2)
	if err != nil {
		t.Fatalf("Second StartEnvelope failed: %v", err)
	}

	// Get active envelope - should be the second one
	active := store.GetActiveEnvelope(circleID, clock)
	if active == nil {
		t.Fatal("Expected active envelope")
	}

	if active.Kind != ae.EnvelopeKindOnCall {
		t.Errorf("Expected on_call envelope, got %s", active.Kind)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 12: Store Bounded Retention
// ═══════════════════════════════════════════════════════════════════════════════

func TestStoreBoundedRetention(t *testing.T) {
	cfg := persist.DefaultAttentionEnvelopeStoreConfig()
	cfg.MaxRecords = 5 // Small limit for testing
	store := persist.NewAttentionEnvelopeStore(cfg)

	// Create receipts
	for i := 0; i < 10; i++ {
		receipt := &ae.EnvelopeReceipt{
			EnvelopeHash: "env_hash",
			CircleIDHash: "circle_hash",
			Action:       ae.ActionStarted,
			PeriodKey:    "2025-01-15",
		}
		receipt.ReceiptID = receipt.ComputeReceiptID() + string(rune('0'+i))
		receipt.StatusHash = receipt.ComputeStatusHash()

		err := store.PersistReceipt(receipt)
		if err != nil {
			t.Fatalf("PersistReceipt %d failed: %v", i, err)
		}
	}

	// Should have max 5 receipts due to FIFO eviction
	total := store.TotalReceipts()
	if total > 5 {
		t.Errorf("Expected max 5 receipts, got %d", total)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 13: Canonical Strings Stable
// ═══════════════════════════════════════════════════════════════════════════════

func TestCanonicalStringsAreStable(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindTravel,
		ae.Duration4h,
		ae.ReasonTravelTransit,
		"circle_hash_stable",
		clock,
	)

	canonical1 := env.CanonicalString()
	canonical2 := env.CanonicalString()

	if canonical1 != canonical2 {
		t.Error("CanonicalString should be stable")
	}

	// Verify format
	if len(canonical1) == 0 {
		t.Error("CanonicalString should not be empty")
	}

	// Should contain version prefix
	if canonical1[:11] != "ENVELOPE|v1" {
		t.Errorf("CanonicalString should start with ENVELOPE|v1, got %s", canonical1[:11])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 14: Guard Rejects Invalid Enums
// ═══════════════════════════════════════════════════════════════════════════════

func TestGuardRejectsInvalidEnums(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Invalid kind
	_, err := engine.BuildEnvelope(
		ae.EnvelopeKind("invalid_kind"),
		ae.Duration1h,
		ae.ReasonDeadline,
		"circle_hash",
		clock,
	)
	if err == nil {
		t.Error("Expected error for invalid kind")
	}

	// Invalid duration
	_, err = engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.DurationBucket("invalid_duration"),
		ae.ReasonDeadline,
		"circle_hash",
		clock,
	)
	if err == nil {
		t.Error("Expected error for invalid duration")
	}

	// Invalid reason
	_, err = engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration1h,
		ae.EnvelopeReason("invalid_reason"),
		"circle_hash",
		clock,
	)
	if err == nil {
		t.Error("Expected error for invalid reason")
	}

	// Empty circle ID
	_, err = engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration1h,
		ae.ReasonDeadline,
		"",
		clock,
	)
	if err == nil {
		t.Error("Expected error for empty circle ID")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 15: Receipt Building
// ═══════════════════════════════════════════════════════════════════════════════

func TestReceiptBuilding(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration1h,
		ae.ReasonDeadline,
		"circle_hash",
		clock,
	)

	receipt := engine.BuildReceipt(env, ae.ActionStarted, clock)

	if receipt == nil {
		t.Fatal("Receipt should not be nil")
	}

	if receipt.EnvelopeHash != env.EnvelopeID {
		t.Error("Receipt should reference envelope ID")
	}

	if receipt.CircleIDHash != env.CircleIDHash {
		t.Error("Receipt should have circle ID hash")
	}

	if receipt.Action != ae.ActionStarted {
		t.Errorf("Expected ActionStarted, got %s", receipt.Action)
	}

	if receipt.ReceiptID == "" {
		t.Error("Receipt should have ID")
	}

	if receipt.StatusHash == "" {
		t.Error("Receipt should have status hash")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 16: Period Key Generation
// ═══════════════════════════════════════════════════════════════════════════════

func TestPeriodKeyGeneration(t *testing.T) {
	testCases := []struct {
		time     time.Time
		expected string
	}{
		{time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), "2025-01-15T10:00"},
		{time.Date(2025, 1, 15, 10, 14, 0, 0, time.UTC), "2025-01-15T10:00"},
		{time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC), "2025-01-15T10:15"},
		{time.Date(2025, 1, 15, 10, 29, 0, 0, time.UTC), "2025-01-15T10:15"},
		{time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), "2025-01-15T10:30"},
		{time.Date(2025, 1, 15, 10, 44, 0, 0, time.UTC), "2025-01-15T10:30"},
		{time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC), "2025-01-15T10:45"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := ae.NewPeriodKey(tc.time)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 17: Duration Bucket Conversion
// ═══════════════════════════════════════════════════════════════════════════════

func TestDurationBucketConversion(t *testing.T) {
	testCases := []struct {
		bucket   ae.DurationBucket
		expected time.Duration
	}{
		{ae.Duration15m, 15 * time.Minute},
		{ae.Duration1h, 1 * time.Hour},
		{ae.Duration4h, 4 * time.Hour},
		{ae.DurationDay, 24 * time.Hour},
	}

	for _, tc := range testCases {
		t.Run(string(tc.bucket), func(t *testing.T) {
			result := tc.bucket.ToDuration()
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 18: Proof Page Building
// ═══════════════════════════════════════════════════════════════════════════════

func TestProofPageBuilding(t *testing.T) {
	engine := attentionenvelope.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Test with active envelope
	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindOnCall,
		ae.Duration4h,
		ae.ReasonOnCallDuty,
		"circle_hash",
		clock,
	)

	page := engine.BuildProofPage("circle_hash", env, 5)

	if page.CircleIDHash != "circle_hash" {
		t.Error("Page should have circle ID hash")
	}

	if page.CurrentEnvelopeHash != env.EnvelopeID {
		t.Error("Page should have current envelope hash")
	}

	if page.CurrentKind != ae.EnvelopeKindOnCall {
		t.Errorf("Expected on_call kind, got %s", page.CurrentKind)
	}

	if page.RecentReceiptCount != "several" {
		t.Errorf("Expected 'several' for 5 receipts, got %s", page.RecentReceiptCount)
	}

	if page.PageHash == "" {
		t.Error("Page should have hash")
	}

	// Test without envelope
	pageNoEnv := engine.BuildProofPage("circle_hash", nil, 0)

	if pageNoEnv.CurrentKind != ae.EnvelopeKindNone {
		t.Error("Page without envelope should have kind none")
	}

	if pageNoEnv.RecentReceiptCount != "none" {
		t.Errorf("Expected 'none' for 0 receipts, got %s", pageNoEnv.RecentReceiptCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 19: Store Auto-Expires on Get
// ═══════════════════════════════════════════════════════════════════════════════

func TestStoreAutoExpiresOnGet(t *testing.T) {
	store := persist.NewAttentionEnvelopeStore(persist.DefaultAttentionEnvelopeStoreConfig())
	engine := attentionenvelope.NewEngine()

	startClock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := "circle_auto_expire"

	// Create and start a 15-minute envelope
	env, _ := engine.BuildEnvelope(
		ae.EnvelopeKindWorking,
		ae.Duration15m,
		ae.ReasonDeadline,
		circleID,
		startClock,
	)
	store.StartEnvelope(env)

	// Should be active at start time
	active := store.GetActiveEnvelope(circleID, startClock)
	if active == nil {
		t.Fatal("Expected active envelope at start time")
	}

	// Should be nil (expired) after 15 minutes
	expiredClock := startClock.Add(15 * time.Minute)
	expired := store.GetActiveEnvelope(circleID, expiredClock)
	if expired != nil {
		t.Error("Expected nil after envelope expires")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Test 20: AllEnums Functions
// ═══════════════════════════════════════════════════════════════════════════════

func TestAllEnumsFunctions(t *testing.T) {
	// Test AllEnvelopeKinds
	kinds := ae.AllEnvelopeKinds()
	if len(kinds) != 5 {
		t.Errorf("Expected 5 envelope kinds, got %d", len(kinds))
	}

	// Test AllDurationBuckets
	durations := ae.AllDurationBuckets()
	if len(durations) != 4 {
		t.Errorf("Expected 4 duration buckets, got %d", len(durations))
	}

	// Test AllEnvelopeReasons
	reasons := ae.AllEnvelopeReasons()
	if len(reasons) != 5 {
		t.Errorf("Expected 5 envelope reasons, got %d", len(reasons))
	}

	// Test AllEnvelopeStates
	states := ae.AllEnvelopeStates()
	if len(states) != 3 {
		t.Errorf("Expected 3 envelope states, got %d", len(states))
	}
}
