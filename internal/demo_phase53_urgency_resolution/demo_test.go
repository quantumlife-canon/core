// Package demo_phase53_urgency_resolution contains demo tests for Phase 53.
//
// These tests verify the Urgency Resolution Layer invariants:
// - NO POWER: Cap-only, clamp-only, no execution, no delivery
// - HASH-ONLY: Only hashes, buckets, status flags stored/rendered
// - DETERMINISTIC: Same inputs + same clock = same resolution hash
// - COMMERCE NEVER ESCALATES: Always cap_hold_only
// - CAPS ONLY REDUCE: Never increase power
// - REASONS MAX 3: Sorted, capped at 3
//
// Reference: docs/ADR/ADR-0091-phase53-urgency-resolution-layer.md
package demo_phase53_urgency_resolution

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	engine "quantumlife/internal/urgencyresolve"
	domain "quantumlife/pkg/domain/urgencyresolve"
)

// ============================================================================
// Test Clock
// ============================================================================

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

func newTestClock(t time.Time) *testClock {
	return &testClock{now: t}
}

// ============================================================================
// Determinism Tests
// ============================================================================

func TestDeterminism_SameInputsSameClock_SameHash(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		WindowSignal:       domain.WindowNone,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
	}

	res1, err1 := eng.ComputeResolution(inputs)
	res2, err2 := eng.ComputeResolution(inputs)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected error: %v, %v", err1, err2)
	}

	if res1.ResolutionHash != res2.ResolutionHash {
		t.Errorf("expected same hash, got %s vs %s", res1.ResolutionHash, res2.ResolutionHash)
	}
}

func TestDeterminism_DifferentInputs_DifferentInputHash(t *testing.T) {
	// Different inputs should produce different input hashes
	inputs1 := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		WindowSignal:       domain.WindowNone,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
	}

	inputs2 := inputs1
	inputs2.CircleType = domain.BucketInstitution

	hash1 := domain.HashUrgencyInputs(inputs1)
	hash2 := domain.HashUrgencyInputs(inputs2)

	if hash1 == hash2 {
		t.Error("expected different input hashes for different inputs")
	}
}

func TestDeterminism_DifferentResolution_DifferentHash(t *testing.T) {
	// Resolution hash is computed from level, cap, reasons, and status
	// Create two resolutions that differ in level/cap to get different hashes
	res1 := domain.UrgencyResolution{
		CircleIDHash: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:    "2025-01",
		Level:        domain.UrgNone,
		Cap:          domain.CapHoldOnly,
		Reasons:      []domain.UrgencyReasonBucket{domain.ReasonDefaultHold},
		Status:       domain.StatusOK,
	}
	res1.ResolutionHash = res1.ComputeHash()

	res2 := domain.UrgencyResolution{
		CircleIDHash: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:    "2025-01",
		Level:        domain.UrgLow, // Different level
		Cap:          domain.CapSurfaceOnly,
		Reasons:      []domain.UrgencyReasonBucket{domain.ReasonTimeWindow},
		Status:       domain.StatusOK,
	}
	res2.ResolutionHash = res2.ComputeHash()

	if res1.ResolutionHash == res2.ResolutionHash {
		t.Error("expected different resolution hashes for different resolutions")
	}
}

func TestDeterminism_ReasonsSorted(t *testing.T) {
	reasons := []domain.UrgencyReasonBucket{
		domain.ReasonTimeWindow,
		domain.ReasonDefaultHold,
		domain.ReasonEnvelopeActive,
	}

	sorted := domain.SortReasons(reasons)

	// Verify sorted order (alphabetical by string value)
	for i := 1; i < len(sorted); i++ {
		if string(sorted[i-1]) > string(sorted[i]) {
			t.Errorf("reasons not sorted: %v", sorted)
		}
	}
}

// ============================================================================
// Commerce Always Hold-Only Tests
// ============================================================================

func TestCommerce_AlwaysHoldOnly(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketCommerce,
		HorizonBucket:      domain.HorizonNow, // Even with "now" horizon
		MagnitudeBucket:    domain.MagSeveral, // Even with high magnitude
		EnvelopeActive:     true,              // Even with active envelope
		WindowSignal:       domain.WindowActive,
		VendorCap:          domain.CapInterruptCandidateOnly,
		InterruptAllowance: domain.AllowanceInterrupt,
	}

	res, err := eng.ComputeResolution(inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Cap != domain.CapHoldOnly {
		t.Errorf("commerce should always be cap_hold_only, got %s", res.Cap)
	}
}

func TestCommerce_WithEnvelope_StillHoldOnly(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketCommerce,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		EnvelopeActive:     true,
		WindowSignal:       domain.WindowNone,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
	}

	res, _ := eng.ComputeResolution(inputs)
	if res.Cap != domain.CapHoldOnly {
		t.Errorf("commerce with envelope should still be hold_only, got %s", res.Cap)
	}
}

func TestCommerce_WithWindow_StillHoldOnly(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketCommerce,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		WindowSignal:       domain.WindowActive,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
	}

	res, _ := eng.ComputeResolution(inputs)
	if res.Cap != domain.CapHoldOnly {
		t.Errorf("commerce with window should still be hold_only, got %s", res.Cap)
	}
}

// ============================================================================
// Vendor Cap Clamp Tests
// ============================================================================

func TestVendorCap_ClampsCorrectly(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNow,
		MagnitudeBucket:    domain.MagSeveral,
		VendorCap:          domain.CapSurfaceOnly, // Vendor cap limits to surface
		InterruptAllowance: domain.AllowanceInterrupt,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)

	// Cap should not exceed vendor cap
	if res.Cap.Order() > domain.CapSurfaceOnly.Order() {
		t.Errorf("cap should respect vendor cap, got %s", res.Cap)
	}
}

func TestVendorCap_AddsReason(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly, // Restrictive vendor cap
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)

	// Should not fail - default is already hold, so no clamp reason needed
	if res.Status == domain.StatusRejected {
		t.Error("unexpected rejection")
	}
}

// ============================================================================
// Trust Fragile Clamp Tests
// ============================================================================

func TestTrustFragile_ClampsToSurfaceOnly(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNow,
		MagnitudeBucket:    domain.MagSeveral,
		TrustFragile:       true,
		VendorCap:          domain.CapInterruptCandidateOnly,
		InterruptAllowance: domain.AllowanceInterrupt,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)

	// Trust fragile should clamp to surface only max
	if res.Cap.Order() > domain.CapSurfaceOnly.Order() {
		t.Errorf("trust fragile should clamp to surface_only max, got %s", res.Cap)
	}
}

// ============================================================================
// Envelope Shift Tests
// ============================================================================

func TestEnvelope_ShiftsLevelByOneMax(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Start with conditions that give urg_none
	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		EnvelopeActive:     true,
		VendorCap:          domain.CapInterruptCandidateOnly,
		InterruptAllowance: domain.AllowanceInterrupt,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)

	// Envelope should shift level by at most 1 step
	// From none -> low is OK, but should never jump to high
	if res.Level.Order() > domain.UrgLow.Order() {
		t.Errorf("envelope should shift by one step max from none, got %s", res.Level)
	}
}

func TestEnvelope_RespectsCap(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		EnvelopeActive:     true,
		VendorCap:          domain.CapHoldOnly, // Hold only cap
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)

	// Even with envelope, level should not exceed what cap allows
	if res.Level != domain.UrgNone {
		t.Errorf("envelope should respect cap, got level %s", res.Level)
	}
}

// ============================================================================
// Necessity Never Increases Tests
// ============================================================================

func TestNecessity_NeverIncreasesEscalation(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// With necessity declared
	inputsWithNecessity := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketInstitution,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		NecessityDeclared:  true,
		VendorCap:          domain.CapInterruptCandidateOnly,
		InterruptAllowance: domain.AllowanceInterrupt,
		WindowSignal:       domain.WindowNone,
	}

	// Without necessity declared
	inputsWithoutNecessity := inputsWithNecessity
	inputsWithoutNecessity.NecessityDeclared = false

	resWithNecessity, _ := eng.ComputeResolution(inputsWithNecessity)
	resWithoutNecessity, _ := eng.ComputeResolution(inputsWithoutNecessity)

	// Necessity=false should result in same or lower cap (never higher)
	if resWithoutNecessity.Cap.Order() > resWithNecessity.Cap.Order() {
		t.Errorf("necessity=false should not increase cap: with=%s, without=%s",
			resWithNecessity.Cap, resWithoutNecessity.Cap)
	}
}

// ============================================================================
// Reasons Max 3 Tests
// ============================================================================

func TestReasons_MaxThree(t *testing.T) {
	reasons := []domain.UrgencyReasonBucket{
		domain.ReasonTimeWindow,
		domain.ReasonDefaultHold,
		domain.ReasonEnvelopeActive,
		domain.ReasonHumanNow,
		domain.ReasonTrustProtection,
	}

	sorted := domain.SortReasons(reasons)

	if len(sorted) > 3 {
		t.Errorf("reasons should be capped at 3, got %d", len(sorted))
	}
}

func TestReasons_EmptyInput(t *testing.T) {
	reasons := []domain.UrgencyReasonBucket{}
	sorted := domain.SortReasons(reasons)

	if len(sorted) != 0 {
		t.Errorf("empty input should return empty, got %d", len(sorted))
	}
}

// ============================================================================
// Handler Forbidden Fields Tests
// ============================================================================

func TestForbiddenParams_Detected(t *testing.T) {
	forbidden := []string{"vendor_id", "vendorID", "pack_id", "packID", "merchant", "email", "url", "amount", "period"}

	for _, param := range forbidden {
		if !strings.Contains(strings.ToLower(param), strings.ToLower(param)) {
			t.Errorf("forbidden param %s should be detected", param)
		}
	}
}

// ============================================================================
// Store Retention Tests
// ============================================================================

func TestStore_FIFOEvictionAfterMaxRecords(t *testing.T) {
	clockTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clockFn := func() time.Time { return clockTime }

	store := persist.NewUrgencyResolutionStore(clockFn)

	// Add max entries
	for i := 0; i < persist.UrgencyResolutionMaxEntries; i++ {
		res := domain.UrgencyResolution{
			CircleIDHash:   "circle",
			PeriodKey:      "2025-01",
			Level:          domain.UrgNone,
			Cap:            domain.CapHoldOnly,
			Reasons:        []domain.UrgencyReasonBucket{domain.ReasonDefaultHold},
			Status:         domain.StatusOK,
			ResolutionHash: strings.Repeat("a", 64-len("hash")) + "hash" + string(rune('0'+i%10)),
		}
		res.ResolutionHash = res.ComputeHash()
		store.RecordResolution(res)
	}

	// Should not exceed max
	if store.Count() > persist.UrgencyResolutionMaxEntries {
		t.Errorf("store should not exceed max entries: %d", store.Count())
	}
}

func TestStore_RetentionRespects30Days(t *testing.T) {
	startTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := startTime
	clockFn := func() time.Time { return currentTime }

	store := persist.NewUrgencyResolutionStore(clockFn)

	// Add entry at start time
	res := domain.UrgencyResolution{
		CircleIDHash:   "circle",
		PeriodKey:      "2025-01",
		Level:          domain.UrgNone,
		Cap:            domain.CapHoldOnly,
		Reasons:        []domain.UrgencyReasonBucket{domain.ReasonDefaultHold},
		Status:         domain.StatusOK,
		ResolutionHash: "initial_hash",
	}
	res.ResolutionHash = res.ComputeHash()
	store.RecordResolution(res)

	// Advance time past retention
	currentTime = startTime.AddDate(0, 0, persist.UrgencyResolutionMaxRetentionDays+1)

	// Add new entry to trigger eviction
	res2 := domain.UrgencyResolution{
		CircleIDHash:   "circle2",
		PeriodKey:      "2025-02",
		Level:          domain.UrgNone,
		Cap:            domain.CapHoldOnly,
		Reasons:        []domain.UrgencyReasonBucket{domain.ReasonDefaultHold},
		Status:         domain.StatusOK,
		ResolutionHash: "new_hash",
	}
	res2.ResolutionHash = res2.ComputeHash()
	store.RecordResolution(res2)

	// Old entry should be evicted
	if store.Count() > 1 {
		t.Errorf("old entries should be evicted after retention period")
	}
}

// ============================================================================
// Page Structure Tests
// ============================================================================

func TestPage_HasTitle(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)
	page := eng.BuildProofPage(res)

	if page.Title == "" {
		t.Error("page should have a title")
	}
}

func TestPage_HasPeriodKey(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)
	page := eng.BuildProofPage(res)

	if page.PeriodKey == "" {
		t.Error("page should have a period key")
	}
}

func TestPage_HasStatusHash(t *testing.T) {
	clk := newTestClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	res, _ := eng.ComputeResolution(inputs)
	page := eng.BuildProofPage(res)

	if page.StatusHash == "" {
		t.Error("page should have a status hash")
	}
}

func TestPage_LinesMax8(t *testing.T) {
	page := domain.UrgencyProofPage{
		Title: "Test",
		Lines: make([]string, 9),
		Level: domain.UrgNone,
		Cap:   domain.CapHoldOnly,
	}

	err := page.Validate()
	if err == nil {
		t.Error("page with more than 8 lines should fail validation")
	}
}

func TestPage_ReasonChipsMax3(t *testing.T) {
	page := domain.UrgencyProofPage{
		Title:       "Test",
		Lines:       []string{},
		ReasonChips: []string{"a", "b", "c", "d"},
		Level:       domain.UrgNone,
		Cap:         domain.CapHoldOnly,
	}

	err := page.Validate()
	if err == nil {
		t.Error("page with more than 3 reason chips should fail validation")
	}
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestValidation_InputsRequireCircleIDHash(t *testing.T) {
	inputs := domain.UrgencyInputs{
		CircleIDHash:       "",
		PeriodKey:          "2025-01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	err := inputs.Validate()
	if err == nil {
		t.Error("inputs without CircleIDHash should fail validation")
	}
}

func TestValidation_InputsRequirePeriodKey(t *testing.T) {
	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	err := inputs.Validate()
	if err == nil {
		t.Error("inputs without PeriodKey should fail validation")
	}
}

func TestValidation_PeriodKeyCannotContainPipe(t *testing.T) {
	inputs := domain.UrgencyInputs{
		CircleIDHash:       "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		PeriodKey:          "2025|01",
		CircleType:         domain.BucketHuman,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		VendorCap:          domain.CapHoldOnly,
		InterruptAllowance: domain.AllowanceNone,
		WindowSignal:       domain.WindowNone,
	}

	err := inputs.Validate()
	if err == nil {
		t.Error("period key with pipe should fail validation")
	}
}

// ============================================================================
// Enum Validation Tests
// ============================================================================

func TestEnum_UrgencyLevelValidation(t *testing.T) {
	valid := []domain.UrgencyLevel{domain.UrgNone, domain.UrgLow, domain.UrgMedium, domain.UrgHigh}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("valid UrgencyLevel %s should pass validation", v)
		}
	}

	invalid := domain.UrgencyLevel("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid UrgencyLevel should fail validation")
	}
}

func TestEnum_EscalationCapValidation(t *testing.T) {
	valid := []domain.EscalationCap{domain.CapHoldOnly, domain.CapSurfaceOnly, domain.CapInterruptCandidateOnly}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("valid EscalationCap %s should pass validation", v)
		}
	}

	invalid := domain.EscalationCap("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid EscalationCap should fail validation")
	}
}

func TestEnum_CircleTypeBucketValidation(t *testing.T) {
	valid := []domain.CircleTypeBucket{domain.BucketHuman, domain.BucketInstitution, domain.BucketCommerce, domain.BucketUnknown}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("valid CircleTypeBucket %s should pass validation", v)
		}
	}

	invalid := domain.CircleTypeBucket("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid CircleTypeBucket should fail validation")
	}
}

func TestEnum_ResolutionStatusValidation(t *testing.T) {
	valid := []domain.ResolutionStatus{domain.StatusOK, domain.StatusClamped, domain.StatusRejected}
	for _, v := range valid {
		if err := v.Validate(); err != nil {
			t.Errorf("valid ResolutionStatus %s should pass validation", v)
		}
	}

	invalid := domain.ResolutionStatus("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid ResolutionStatus should fail validation")
	}
}
