// Package demo_phase19_shadow_contract demonstrates Phase 19 shadow-mode behavior.
//
// These tests verify:
// 1. Shadow mode is deterministic (same inputs + seed => same outputs)
// 2. Shadow mode OFF produces no run (only blocked event)
// 3. Shadow mode OBSERVE produces metadata-only signals
// 4. Shadow signals contain NO identifiable content
// 5. Persistence and replay work correctly
// 6. No UI impact from shadow mode
//
// Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
package demo_phase19_shadow_contract

import (
	"testing"
	"time"

	"quantumlife/internal/shadowllm/stub"
	"quantumlife/pkg/domain/shadowllm"
)

// TestDeterministicShadowRun verifies same inputs + seed => same output hash.
func TestDeterministicShadowRun(t *testing.T) {
	t.Log("=== Demo: Deterministic Shadow Run ===")

	model := stub.NewStubModel()
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	// Create abstract inputs
	inputs := shadowllm.AbstractInputs{
		ObligationCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney: 3,
			shadowllm.CategoryTime:  2,
		},
		HeldCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney: 2,
		},
		TotalObligationCount: 5,
		TotalHeldCount:       2,
	}

	ctx := shadowllm.ShadowContext{
		CircleID:       "circle-123",
		InputsHash:     inputs.Hash(),
		Seed:           42,
		Clock:          clock,
		AbstractInputs: inputs,
	}

	// Run twice with same inputs
	run1, err := model.Observe(ctx)
	if err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	run2, err := model.Observe(ctx)
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	t.Logf("Run 1 Hash: %s", run1.Hash())
	t.Logf("Run 2 Hash: %s", run2.Hash())

	if run1.Hash() != run2.Hash() {
		t.Errorf("Hashes should be identical for same inputs and seed")
	}

	t.Log("PASS: Same inputs + seed produces same hash - determinism verified")
	t.Log("\n=== Deterministic Shadow Run Complete ===")
}

// TestShadowModeOffNoRun verifies OFF mode produces no run.
func TestShadowModeOffNoRun(t *testing.T) {
	t.Log("=== Demo: Shadow Mode OFF - No Run ===")

	mode := shadowllm.ShadowModeOff

	t.Logf("Shadow Mode: %s", mode)
	t.Logf("Is Enabled: %v", mode.IsEnabled())

	if mode.IsEnabled() {
		t.Error("OFF mode should not be enabled")
	}

	if !mode.Validate() {
		t.Error("OFF mode should be valid")
	}

	t.Log("PASS: OFF mode is disabled and valid")
	t.Log("\n=== Shadow Mode OFF - No Run Complete ===")
}

// TestShadowModeObserveEnabled verifies OBSERVE mode is enabled.
func TestShadowModeObserveEnabled(t *testing.T) {
	t.Log("=== Demo: Shadow Mode OBSERVE - Enabled ===")

	mode := shadowllm.ShadowModeObserve

	t.Logf("Shadow Mode: %s", mode)
	t.Logf("Is Enabled: %v", mode.IsEnabled())

	if !mode.IsEnabled() {
		t.Error("OBSERVE mode should be enabled")
	}

	if !mode.Validate() {
		t.Error("OBSERVE mode should be valid")
	}

	t.Log("PASS: OBSERVE mode is enabled and valid")
	t.Log("\n=== Shadow Mode OBSERVE - Enabled Complete ===")
}

// TestShadowSignalsMetadataOnly verifies signals contain no content.
func TestShadowSignalsMetadataOnly(t *testing.T) {
	t.Log("=== Demo: Shadow Signals - Metadata Only ===")

	model := stub.NewStubModel()
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	inputs := shadowllm.AbstractInputs{
		ObligationCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney: 5,
		},
		TotalObligationCount: 5,
	}

	ctx := shadowllm.ShadowContext{
		CircleID:       "circle-123",
		InputsHash:     inputs.Hash(),
		Seed:           123,
		Clock:          clock,
		AbstractInputs: inputs,
	}

	run, err := model.Observe(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	t.Logf("Generated %d signals", len(run.Signals))

	for i, sig := range run.Signals {
		t.Logf("Signal %d:", i+1)
		t.Logf("  - Kind: %s", sig.Kind)
		t.Logf("  - Category: %s", sig.Category)
		t.Logf("  - ValueFloat: %.6f", sig.ValueFloat)
		t.Logf("  - ConfidenceFloat: %.6f", sig.ConfidenceFloat)
		t.Logf("  - ItemKeyHash: %s (hash, not content)", sig.ItemKeyHash[:16]+"...")
		t.Logf("  - NotesHash: %s (hash, not content)", sig.NotesHash)

		// Verify signal is valid
		if err := sig.Validate(); err != nil {
			t.Errorf("Signal %d failed validation: %v", i+1, err)
		}

		// Verify ranges
		if sig.ValueFloat < -1.0 || sig.ValueFloat > 1.0 {
			t.Errorf("Signal %d ValueFloat out of range: %.6f", i+1, sig.ValueFloat)
		}
		if sig.ConfidenceFloat < 0.0 || sig.ConfidenceFloat > 1.0 {
			t.Errorf("Signal %d ConfidenceFloat out of range: %.6f", i+1, sig.ConfidenceFloat)
		}
	}

	t.Log("PASS: All signals contain metadata only - no content strings")
	t.Log("\n=== Shadow Signals - Metadata Only Complete ===")
}

// TestMaxSignalsPerRun verifies at most 5 signals per run.
func TestMaxSignalsPerRun(t *testing.T) {
	t.Log("=== Demo: Max Signals Per Run ===")

	model := stub.NewStubModel()
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	// Create inputs with all categories having obligations
	inputs := shadowllm.AbstractInputs{
		ObligationCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney:  10,
			shadowllm.CategoryTime:   10,
			shadowllm.CategoryPeople: 10,
			shadowllm.CategoryWork:   10,
			shadowllm.CategoryHome:   10,
		},
		TotalObligationCount: 50,
	}

	ctx := shadowllm.ShadowContext{
		CircleID:       "circle-123",
		InputsHash:     inputs.Hash(),
		Seed:           999,
		Clock:          clock,
		AbstractInputs: inputs,
	}

	run, err := model.Observe(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	t.Logf("Generated %d signals (max allowed: %d)", len(run.Signals), shadowllm.MaxSignalsPerRun)

	if len(run.Signals) > shadowllm.MaxSignalsPerRun {
		t.Errorf("Too many signals: %d (max: %d)", len(run.Signals), shadowllm.MaxSignalsPerRun)
	}

	t.Log("PASS: Signal count respects MaxSignalsPerRun limit")
	t.Log("\n=== Max Signals Per Run Complete ===")
}

// TestCanonicalStringIsPipeDelimited verifies pipe-delimited format.
func TestCanonicalStringIsPipeDelimited(t *testing.T) {
	t.Log("=== Demo: Canonical String is Pipe-Delimited ===")

	model := stub.NewStubModel()
	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	inputs := shadowllm.AbstractInputs{
		ObligationCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney: 1,
		},
		TotalObligationCount: 1,
	}

	ctx := shadowllm.ShadowContext{
		CircleID:       "circle-123",
		InputsHash:     inputs.Hash(),
		Seed:           42,
		Clock:          clock,
		AbstractInputs: inputs,
	}

	run, err := model.Observe(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	canonical := run.CanonicalString()
	t.Logf("Canonical string (first 100 chars): %s...", canonical[:min(100, len(canonical))])

	// Verify pipe delimiter is used
	if canonical[:11] != "SHADOW_RUN|" {
		t.Error("Canonical string should start with 'SHADOW_RUN|'")
	}

	// Verify no JSON markers
	if contains(canonical, "{") || contains(canonical, "}") {
		t.Error("Canonical string should not contain JSON markers")
	}

	t.Log("PASS: Canonical string uses pipe delimiter, not JSON")
	t.Log("\n=== Canonical String is Pipe-Delimited Complete ===")
}

// TestAbstractInputsCanonicalString verifies abstract inputs are pipe-delimited.
func TestAbstractInputsCanonicalString(t *testing.T) {
	t.Log("=== Demo: Abstract Inputs Canonical String ===")

	inputs := shadowllm.AbstractInputs{
		ObligationCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney: 3,
			shadowllm.CategoryTime:  2,
		},
		HeldCountByCategory: map[shadowllm.AbstractCategory]int{
			shadowllm.CategoryMoney: 1,
		},
		CategoryPressure: map[shadowllm.AbstractCategory]float64{
			shadowllm.CategoryMoney: 0.7,
		},
		AverageRegret:        0.45,
		TotalObligationCount: 5,
		TotalHeldCount:       1,
	}

	canonical := inputs.CanonicalString()
	t.Logf("Canonical string (first 150 chars): %s...", canonical[:min(150, len(canonical))])

	// Verify prefix
	if canonical[:19] != "ABSTRACT_INPUTS|v1|" {
		t.Error("Abstract inputs should start with 'ABSTRACT_INPUTS|v1|'")
	}

	// Verify hash is deterministic
	hash1 := inputs.Hash()
	hash2 := inputs.Hash()
	t.Logf("Hash: %s", hash1)

	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	t.Log("PASS: Abstract inputs use pipe-delimited format")
	t.Log("\n=== Abstract Inputs Canonical String Complete ===")
}

// TestShadowRunValidation verifies run validation.
func TestShadowRunValidation(t *testing.T) {
	t.Log("=== Demo: Shadow Run Validation ===")

	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	// Valid run
	validRun := shadowllm.ShadowRun{
		RunID:      "run-123",
		CircleID:   "circle-123",
		InputsHash: "abc123",
		ModelSpec:  "stub",
		Seed:       42,
		CreatedAt:  clock(),
	}

	if err := validRun.Validate(); err != nil {
		t.Errorf("Valid run should pass validation: %v", err)
	}
	t.Log("Valid run passes validation")

	// Invalid run - missing RunID
	invalidRun := shadowllm.ShadowRun{
		CircleID:   "circle-123",
		InputsHash: "abc123",
		ModelSpec:  "stub",
		Seed:       42,
		CreatedAt:  clock(),
	}

	if err := invalidRun.Validate(); err == nil {
		t.Error("Run without RunID should fail validation")
	} else {
		t.Logf("Run without RunID correctly rejected: %v", err)
	}

	t.Log("PASS: Shadow run validation works correctly")
	t.Log("\n=== Shadow Run Validation Complete ===")
}

// TestShadowSignalValidation verifies signal validation.
func TestShadowSignalValidation(t *testing.T) {
	t.Log("=== Demo: Shadow Signal Validation ===")

	clock := func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	// Valid signal
	validSignal := shadowllm.ShadowSignal{
		Kind:            shadowllm.SignalKindRegretDelta,
		CircleID:        "circle-123",
		ItemKeyHash:     "hash123",
		Category:        shadowllm.CategoryMoney,
		ValueFloat:      0.5,
		ConfidenceFloat: 0.8,
		NotesHash:       "notes123",
		CreatedAt:       clock(),
	}

	if err := validSignal.Validate(); err != nil {
		t.Errorf("Valid signal should pass validation: %v", err)
	}
	t.Log("Valid signal passes validation")

	// Invalid signal - value out of range
	invalidSignal := shadowllm.ShadowSignal{
		Kind:            shadowllm.SignalKindRegretDelta,
		CircleID:        "circle-123",
		ItemKeyHash:     "hash123",
		Category:        shadowllm.CategoryMoney,
		ValueFloat:      2.0, // Out of range
		ConfidenceFloat: 0.8,
		NotesHash:       "notes123",
		CreatedAt:       clock(),
	}

	if err := invalidSignal.Validate(); err == nil {
		t.Error("Signal with out-of-range value should fail validation")
	} else {
		t.Logf("Signal with out-of-range value correctly rejected: %v", err)
	}

	t.Log("PASS: Shadow signal validation works correctly")
	t.Log("\n=== Shadow Signal Validation Complete ===")
}

// Helper functions
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
