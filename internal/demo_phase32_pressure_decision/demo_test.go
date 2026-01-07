// Package demo_phase32_pressure_decision contains demo tests for Phase 32: Pressure Decision Gate.
//
// These tests verify:
// - Classification rules work correctly
// - Commerce always HOLD
// - Human + NOW => INTERRUPT_CANDIDATE
// - Institution + SOON => SURFACE
// - Rate limiting downgrades excess interrupt candidates
// - Trust fragile protection caps decisions
// - Determinism (same input => same hash)
// - Persistence operations
//
// Reference: docs/ADR/ADR-0068-phase32-pressure-decision-gate.md
package demo_phase32_pressure_decision

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/pressuredecision"
	pd "quantumlife/pkg/domain/pressuredecision"
)

// ============================================================================
// Test 1: Commerce Always HOLD
// ============================================================================

func TestCommerceAlwaysHold(t *testing.T) {
	engine := pressuredecision.NewEngine()

	testCases := []struct {
		name      string
		magnitude pd.PressureMagnitude
		horizon   pd.PressureHorizon
	}{
		{"nothing_now", pd.MagnitudeNothing, pd.HorizonNow},
		{"a_few_now", pd.MagnitudeAFew, pd.HorizonNow},
		{"several_now", pd.MagnitudeSeveral, pd.HorizonNow},
		{"several_soon", pd.MagnitudeSeveral, pd.HorizonSoon},
		{"a_few_later", pd.MagnitudeAFew, pd.HorizonLater},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &pd.PressureDecisionInput{
				CircleIDHash: "commerce_circle_hash",
				CircleType:   pd.CircleTypeCommerce,
				Magnitude:    tc.magnitude,
				Horizon:      tc.horizon,
				TrustStatus:  pd.TrustStatusNormal,
				PeriodKey:    "2025-01-07",
			}

			decision := engine.Classify(input)

			if decision.Decision != pd.DecisionHold {
				t.Errorf("commerce should always HOLD, got %s", decision.Decision)
			}
			if decision.ReasonBucket != pd.ReasonCommerceNeverInterrupts {
				t.Errorf("reason should be commerce_never_interrupts, got %s", decision.ReasonBucket)
			}
		})
	}
}

// ============================================================================
// Test 2: Human + NOW => INTERRUPT_CANDIDATE
// ============================================================================

func TestHumanNowInterruptCandidate(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash:             "human_circle_hash",
		CircleType:               pd.CircleTypeHuman,
		Magnitude:                pd.MagnitudeAFew,
		Horizon:                  pd.HorizonNow,
		TrustStatus:              pd.TrustStatusNormal,
		InterruptCandidatesToday: 0,
		PeriodKey:                "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionInterruptCandidate {
		t.Errorf("human + NOW + magnitude should be INTERRUPT_CANDIDATE, got %s", decision.Decision)
	}
	if decision.ReasonBucket != pd.ReasonHumanNow {
		t.Errorf("reason should be human_now, got %s", decision.ReasonBucket)
	}
}

// ============================================================================
// Test 3: Human + NOW + Nothing => HOLD
// ============================================================================

func TestHumanNowNothingHold(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle_hash",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeNothing,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionHold {
		t.Errorf("human + NOW + nothing should be HOLD, got %s", decision.Decision)
	}
	if decision.ReasonBucket != pd.ReasonNoMagnitude {
		t.Errorf("reason should be no_magnitude, got %s", decision.ReasonBucket)
	}
}

// ============================================================================
// Test 4: Institution + SOON + SEVERAL => SURFACE
// ============================================================================

func TestInstitutionSoonSeveralSurface(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "institution_circle_hash",
		CircleType:   pd.CircleTypeInstitution,
		Magnitude:    pd.MagnitudeSeveral,
		Horizon:      pd.HorizonSoon,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionSurface {
		t.Errorf("institution + SOON + several should be SURFACE, got %s", decision.Decision)
	}
	if decision.ReasonBucket != pd.ReasonInstitutionDeadline {
		t.Errorf("reason should be institution_deadline, got %s", decision.ReasonBucket)
	}
}

// ============================================================================
// Test 5: Institution + SOON + A_FEW => HOLD (not enough magnitude)
// ============================================================================

func TestInstitutionSoonAFewHold(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "institution_circle_hash",
		CircleType:   pd.CircleTypeInstitution,
		Magnitude:    pd.MagnitudeAFew,
		Horizon:      pd.HorizonSoon,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	// Institution with a_few doesn't meet the "several" threshold
	if decision.Decision != pd.DecisionHold {
		t.Errorf("institution + SOON + a_few should be HOLD, got %s", decision.Decision)
	}
}

// ============================================================================
// Test 6: Rate Limit Downgrade
// ============================================================================

func TestRateLimitDowngrade(t *testing.T) {
	engine := pressuredecision.NewEngine()

	// First two should be INTERRUPT_CANDIDATE
	for i := 0; i < 2; i++ {
		input := &pd.PressureDecisionInput{
			CircleIDHash:             "human_circle_hash",
			CircleType:               pd.CircleTypeHuman,
			Magnitude:                pd.MagnitudeSeveral,
			Horizon:                  pd.HorizonNow,
			TrustStatus:              pd.TrustStatusNormal,
			InterruptCandidatesToday: i,
			PeriodKey:                "2025-01-07",
		}

		decision := engine.Classify(input)
		if decision.Decision != pd.DecisionInterruptCandidate {
			t.Errorf("first two should be INTERRUPT_CANDIDATE, got %s at i=%d", decision.Decision, i)
		}
	}

	// Third should be downgraded to SURFACE
	input := &pd.PressureDecisionInput{
		CircleIDHash:             "human_circle_hash",
		CircleType:               pd.CircleTypeHuman,
		Magnitude:                pd.MagnitudeSeveral,
		Horizon:                  pd.HorizonNow,
		TrustStatus:              pd.TrustStatusNormal,
		InterruptCandidatesToday: 2, // Already 2 today
		PeriodKey:                "2025-01-07",
	}

	decision := engine.Classify(input)
	if decision.Decision != pd.DecisionSurface {
		t.Errorf("third should be downgraded to SURFACE, got %s", decision.Decision)
	}
	if decision.ReasonBucket != pd.ReasonRateLimitDowngrade {
		t.Errorf("reason should be rate_limit_downgrade, got %s", decision.ReasonBucket)
	}
}

// ============================================================================
// Test 7: Trust Fragile Downgrade
// ============================================================================

func TestTrustFragileDowngrade(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle_hash",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeSeveral,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusFragile, // Fragile trust
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionSurface {
		t.Errorf("fragile trust should cap at SURFACE, got %s", decision.Decision)
	}
	if decision.ReasonBucket != pd.ReasonTrustFragileDowngrade {
		t.Errorf("reason should be trust_fragile_downgrade, got %s", decision.ReasonBucket)
	}
}

// ============================================================================
// Test 8: Determinism - Same Input = Same Hash
// ============================================================================

func TestDeterminism(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "test_circle_hash",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeAFew,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision1 := engine.Classify(input)
	decision2 := engine.Classify(input)

	if decision1.DecisionID != decision2.DecisionID {
		t.Errorf("same input should produce same DecisionID: %s != %s", decision1.DecisionID, decision2.DecisionID)
	}
	if decision1.StatusHash != decision2.StatusHash {
		t.Errorf("same input should produce same StatusHash: %s != %s", decision1.StatusHash, decision2.StatusHash)
	}
	if decision1.InputHash != decision2.InputHash {
		t.Errorf("same input should produce same InputHash: %s != %s", decision1.InputHash, decision2.InputHash)
	}
}

// ============================================================================
// Test 9: Different Input = Different Hash
// ============================================================================

func TestDifferentInputDifferentHash(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input1 := &pd.PressureDecisionInput{
		CircleIDHash: "circle_1",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeAFew,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	input2 := &pd.PressureDecisionInput{
		CircleIDHash: "circle_2", // Different circle
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeAFew,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision1 := engine.Classify(input1)
	decision2 := engine.Classify(input2)

	if decision1.DecisionID == decision2.DecisionID {
		t.Errorf("different input should produce different DecisionID")
	}
}

// ============================================================================
// Test 10: No Pressure => No Record
// ============================================================================

func TestNoPressureNoRecord(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "empty_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeNothing, // No pressure
		Horizon:      pd.HorizonUnknown,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionHold {
		t.Errorf("no pressure should be HOLD, got %s", decision.Decision)
	}

	// ShouldPersist should return false for HOLD decisions
	if engine.ShouldPersist(decision) {
		t.Errorf("HOLD decisions should not be persisted")
	}
}

// ============================================================================
// Test 11: Period Rollover Behavior
// ============================================================================

func TestPeriodRollover(t *testing.T) {
	engine := pressuredecision.NewEngine()

	// Day 1: Use up rate limit
	inputDay1 := &pd.PressureDecisionInput{
		CircleIDHash:             "human_circle",
		CircleType:               pd.CircleTypeHuman,
		Magnitude:                pd.MagnitudeSeveral,
		Horizon:                  pd.HorizonNow,
		TrustStatus:              pd.TrustStatusNormal,
		InterruptCandidatesToday: 2, // Rate limit reached
		PeriodKey:                "2025-01-07",
	}

	decision1 := engine.Classify(inputDay1)
	if decision1.Decision != pd.DecisionSurface {
		t.Errorf("day 1 rate limited should be SURFACE, got %s", decision1.Decision)
	}

	// Day 2: Rate limit resets
	inputDay2 := &pd.PressureDecisionInput{
		CircleIDHash:             "human_circle",
		CircleType:               pd.CircleTypeHuman,
		Magnitude:                pd.MagnitudeSeveral,
		Horizon:                  pd.HorizonNow,
		TrustStatus:              pd.TrustStatusNormal,
		InterruptCandidatesToday: 0, // New day, reset
		PeriodKey:                "2025-01-08",
	}

	decision2 := engine.Classify(inputDay2)
	if decision2.Decision != pd.DecisionInterruptCandidate {
		t.Errorf("day 2 should have fresh rate limit, got %s", decision2.Decision)
	}
}

// ============================================================================
// Test 12: Horizon LATER => HOLD
// ============================================================================

func TestHorizonLaterHold(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeSeveral,
		Horizon:      pd.HorizonLater, // Too far out
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionHold {
		t.Errorf("horizon LATER should be HOLD, got %s", decision.Decision)
	}
	if decision.ReasonBucket != pd.ReasonHorizonLater {
		t.Errorf("reason should be horizon_later, got %s", decision.ReasonBucket)
	}
}

// ============================================================================
// Test 13: Nil Input => Default HOLD
// ============================================================================

func TestNilInputDefaultHold(t *testing.T) {
	engine := pressuredecision.NewEngine()

	decision := engine.Classify(nil)

	if decision.Decision != pd.DecisionHold {
		t.Errorf("nil input should be HOLD, got %s", decision.Decision)
	}
}

// ============================================================================
// Test 14: Invalid Input => Default HOLD
// ============================================================================

func TestInvalidInputDefaultHold(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "", // Invalid - missing
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeAFew,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	if decision.Decision != pd.DecisionHold {
		t.Errorf("invalid input should be HOLD, got %s", decision.Decision)
	}
}

// ============================================================================
// Test 15: Batch Classification
// ============================================================================

func TestBatchClassification(t *testing.T) {
	engine := pressuredecision.NewEngine()

	inputs := []*pd.PressureDecisionInput{
		{
			CircleIDHash: "commerce_1",
			CircleType:   pd.CircleTypeCommerce,
			Magnitude:    pd.MagnitudeSeveral,
			Horizon:      pd.HorizonNow,
			TrustStatus:  pd.TrustStatusNormal,
			PeriodKey:    "2025-01-07",
		},
		{
			CircleIDHash: "human_1",
			CircleType:   pd.CircleTypeHuman,
			Magnitude:    pd.MagnitudeSeveral,
			Horizon:      pd.HorizonNow,
			TrustStatus:  pd.TrustStatusNormal,
			PeriodKey:    "2025-01-07",
		},
		{
			CircleIDHash: "human_2",
			CircleType:   pd.CircleTypeHuman,
			Magnitude:    pd.MagnitudeSeveral,
			Horizon:      pd.HorizonNow,
			TrustStatus:  pd.TrustStatusNormal,
			PeriodKey:    "2025-01-07",
		},
		{
			CircleIDHash: "human_3",
			CircleType:   pd.CircleTypeHuman,
			Magnitude:    pd.MagnitudeSeveral,
			Horizon:      pd.HorizonNow,
			TrustStatus:  pd.TrustStatusNormal,
			PeriodKey:    "2025-01-07",
		},
	}

	batch := engine.ClassifyBatch(inputs, "2025-01-07")

	if batch.HoldCount != 1 {
		t.Errorf("expected 1 HOLD (commerce), got %d", batch.HoldCount)
	}
	if batch.InterruptCandidateCount != 2 {
		t.Errorf("expected 2 INTERRUPT_CANDIDATE (rate limited), got %d", batch.InterruptCandidateCount)
	}
	if batch.SurfaceCount != 1 {
		t.Errorf("expected 1 SURFACE (rate limited third human), got %d", batch.SurfaceCount)
	}
}

// ============================================================================
// Test 16: Persistence Store Operations
// ============================================================================

func TestPersistenceStore(t *testing.T) {
	cfg := persist.DefaultPressureDecisionStoreConfig()
	store := persist.NewPressureDecisionStore(cfg)

	engine := pressuredecision.NewEngine()

	// Use current period to avoid eviction
	currentPeriod := time.Now().Format("2006-01-02")

	input := &pd.PressureDecisionInput{
		CircleIDHash: "test_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeSeveral,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    currentPeriod,
	}

	decision := engine.Classify(input)

	// Store decision
	err := store.AppendDecision(decision)
	if err != nil {
		t.Fatalf("failed to store decision: %v", err)
	}

	// Retrieve by period
	records := store.GetByPeriod(currentPeriod)
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
		return
	}

	// Verify content
	if records[0].Decision != pd.DecisionInterruptCandidate {
		t.Errorf("stored decision should be INTERRUPT_CANDIDATE, got %s", records[0].Decision)
	}
}

// ============================================================================
// Test 17: Persistence Duplicate Rejection
// ============================================================================

func TestPersistenceDuplicateRejection(t *testing.T) {
	cfg := persist.DefaultPressureDecisionStoreConfig()
	store := persist.NewPressureDecisionStore(cfg)

	engine := pressuredecision.NewEngine()

	currentPeriod := time.Now().Format("2006-01-02")

	input := &pd.PressureDecisionInput{
		CircleIDHash: "test_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeSeveral,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusNormal,
		PeriodKey:    currentPeriod,
	}

	decision := engine.Classify(input)

	// Store first time
	err := store.AppendDecision(decision)
	if err != nil {
		t.Fatalf("first store should succeed: %v", err)
	}

	// Store same decision again
	err = store.AppendDecision(decision)
	if err == nil {
		t.Error("duplicate store should fail")
	}
}

// ============================================================================
// Test 18: Persistence Bounded Retention
// ============================================================================

func TestPersistenceBoundedRetention(t *testing.T) {
	cfg := persist.PressureDecisionStoreConfig{
		MaxRetentionDays: 30,
	}
	store := persist.NewPressureDecisionStore(cfg)

	engine := pressuredecision.NewEngine()

	// Use dates relative to a fixed reference time
	refTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	recentPeriod := refTime.Format("2006-01-02")                    // 2026-01-07
	oldPeriod := refTime.AddDate(0, 0, -35).Format("2006-01-02")    // 35 days ago
	borderPeriod := refTime.AddDate(0, 0, -29).Format("2006-01-02") // 29 days ago (should remain)

	periods := []string{oldPeriod, borderPeriod, recentPeriod}

	for _, period := range periods {
		input := &pd.PressureDecisionInput{
			CircleIDHash: "test_circle_" + period,
			CircleType:   pd.CircleTypeHuman,
			Magnitude:    pd.MagnitudeSeveral,
			Horizon:      pd.HorizonNow,
			TrustStatus:  pd.TrustStatusNormal,
			PeriodKey:    period,
		}
		decision := engine.Classify(input)
		_ = store.AppendDecision(decision)
	}

	// Evict with reference time
	store.EvictOldPeriods(refTime)

	// Check which periods remain
	remaining := store.GetAllPeriods()

	// Old period (35 days ago) should be evicted
	for _, p := range remaining {
		if p == oldPeriod {
			t.Errorf("period %s should be evicted (>30 days old)", oldPeriod)
		}
	}

	// Recent period should remain
	found := false
	for _, p := range remaining {
		if p == recentPeriod {
			found = true
		}
	}
	if !found {
		t.Errorf("period %s should remain", recentPeriod)
	}
}

// ============================================================================
// Test 19: Count Interrupt Candidates For Period
// ============================================================================

func TestCountInterruptCandidatesForPeriod(t *testing.T) {
	cfg := persist.DefaultPressureDecisionStoreConfig()
	store := persist.NewPressureDecisionStore(cfg)

	engine := pressuredecision.NewEngine()

	currentPeriod := time.Now().Format("2006-01-02")

	// Add two interrupt candidates
	for i := 0; i < 2; i++ {
		input := &pd.PressureDecisionInput{
			CircleIDHash:             "human_" + string(rune('a'+i)),
			CircleType:               pd.CircleTypeHuman,
			Magnitude:                pd.MagnitudeSeveral,
			Horizon:                  pd.HorizonNow,
			TrustStatus:              pd.TrustStatusNormal,
			InterruptCandidatesToday: i,
			PeriodKey:                currentPeriod,
		}
		decision := engine.Classify(input)
		_ = store.AppendDecision(decision)
	}

	count := store.CountInterruptCandidatesForPeriod(currentPeriod)
	if count != 2 {
		t.Errorf("expected 2 interrupt candidates, got %d", count)
	}
}

// ============================================================================
// Test 20: Trust Unknown Treated as Normal
// ============================================================================

func TestTrustUnknownTreatedAsNormal(t *testing.T) {
	engine := pressuredecision.NewEngine()

	input := &pd.PressureDecisionInput{
		CircleIDHash: "human_circle",
		CircleType:   pd.CircleTypeHuman,
		Magnitude:    pd.MagnitudeSeveral,
		Horizon:      pd.HorizonNow,
		TrustStatus:  pd.TrustStatusUnknown, // Unknown
		PeriodKey:    "2025-01-07",
	}

	decision := engine.Classify(input)

	// Should not be downgraded
	if decision.Decision != pd.DecisionInterruptCandidate {
		t.Errorf("unknown trust should be treated as normal, got %s", decision.Decision)
	}
}

// ============================================================================
// Test 21: Canonical String Format
// ============================================================================

func TestCanonicalStringFormat(t *testing.T) {
	input := &pd.PressureDecisionInput{
		CircleIDHash:             "test_hash",
		CircleType:               pd.CircleTypeHuman,
		Magnitude:                pd.MagnitudeAFew,
		Horizon:                  pd.HorizonNow,
		TrustStatus:              pd.TrustStatusNormal,
		InterruptCandidatesToday: 0,
		PeriodKey:                "2025-01-07",
	}

	canonical := input.CanonicalString()

	// Should start with version prefix
	if len(canonical) < 15 || canonical[:15] != "DECISION_INPUT|" {
		t.Errorf("canonical string should start with DECISION_INPUT|, got %s", canonical)
	}

	// Count pipe delimiters
	// Format: DECISION_INPUT|v1|hash|type|mag|horizon|trust|count|period = 9 parts = 8 delimiters
	expectedDelimiters := 8
	count := 0
	for _, c := range canonical {
		if c == '|' {
			count++
		}
	}
	if count != expectedDelimiters {
		t.Errorf("expected %d pipe delimiters, got %d (canonical: %s)", expectedDelimiters, count, canonical)
	}
}

// ============================================================================
// Test 22: Decision Batch Hash Determinism
// ============================================================================

func TestDecisionBatchHashDeterminism(t *testing.T) {
	engine := pressuredecision.NewEngine()

	inputs := []*pd.PressureDecisionInput{
		{
			CircleIDHash: "human_1",
			CircleType:   pd.CircleTypeHuman,
			Magnitude:    pd.MagnitudeAFew,
			Horizon:      pd.HorizonNow,
			TrustStatus:  pd.TrustStatusNormal,
			PeriodKey:    "2025-01-07",
		},
	}

	batch1 := engine.ClassifyBatch(inputs, "2025-01-07")
	batch2 := engine.ClassifyBatch(inputs, "2025-01-07")

	if batch1.BatchHash != batch2.BatchHash {
		t.Errorf("batch hash should be deterministic: %s != %s", batch1.BatchHash, batch2.BatchHash)
	}
}
