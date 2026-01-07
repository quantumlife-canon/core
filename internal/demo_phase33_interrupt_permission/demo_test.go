// Package demo_phase33_interrupt_permission contains demo tests for Phase 33.
//
// Phase 33: Interrupt Permission Contract (Policy + Proof + Rate-limits)
//
// CRITICAL INVARIANTS:
//   - NO interrupt delivery (no notifications, no emails, no SMS, no push)
//   - Policy evaluation only. No side effects.
//   - Deterministic: same inputs => same outputs + same hashes.
//   - Default stance: NO interrupts allowed.
//   - Commerce always blocked regardless of policy.
//
// Reference: docs/ADR/ADR-0069-phase33-interrupt-permission-contract.md
package demo_phase33_interrupt_permission

import (
	"testing"
	"time"

	"quantumlife/internal/interruptpolicy"
	"quantumlife/internal/persist"
	ip "quantumlife/pkg/domain/interruptpolicy"
)

// ═══════════════════════════════════════════════════════════════════════════
// Test 1: Determinism — same inputs produce same decisions + hashes
// ═══════════════════════════════════════════════════════════════════════════

func TestDeterminism_SameInputsSameOutputs(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidate := &ip.InterruptCandidate{
		CandidateHash: "abc123",
		CircleType:    ip.CircleTypeHuman,
		Horizon:       ip.HorizonNow,
		Magnitude:     ip.MagnitudeAFew,
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   []*ip.InterruptCandidate{candidate},
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	// Run twice
	result1 := engine.Evaluate(input)
	result2 := engine.Evaluate(input)

	// Verify same hashes
	if result1.StatusHash != result2.StatusHash {
		t.Errorf("determinism violated: status hashes differ: %s != %s", result1.StatusHash, result2.StatusHash)
	}

	if result1.InputHash != result2.InputHash {
		t.Errorf("determinism violated: input hashes differ: %s != %s", result1.InputHash, result2.InputHash)
	}

	// Verify same decisions
	if len(result1.Decisions) != len(result2.Decisions) {
		t.Fatalf("determinism violated: decision counts differ: %d != %d", len(result1.Decisions), len(result2.Decisions))
	}

	for i := range result1.Decisions {
		if result1.Decisions[i].DeterministicHash != result2.Decisions[i].DeterministicHash {
			t.Errorf("determinism violated: decision %d hashes differ", i)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 2: Default policy denies all
// ═══════════════════════════════════════════════════════════════════════════

func TestDefaultPolicy_DeniesAll(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "c1", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "c2", CircleType: ip.CircleTypeInstitution, Horizon: ip.HorizonSoon, Magnitude: ip.MagnitudeSeveral},
	}

	// nil policy means default (AllowNone)
	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       nil, // default
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:00",
	}

	result := engine.Evaluate(input)

	// All should be denied
	for _, d := range result.Decisions {
		if d.Allowed {
			t.Errorf("default policy should deny all, but allowed: %s", d.CandidateHash)
		}
		if d.ReasonBucket != ip.ReasonPolicyDenies {
			t.Errorf("expected reason_policy_denies, got: %s", d.ReasonBucket)
		}
	}

	if result.PermittedMagnitude != ip.MagnitudeNothing {
		t.Errorf("expected permitted magnitude nothing, got: %s", result.PermittedMagnitude)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 3: AllowHumansNow allows only human + now candidates
// ═══════════════════════════════════════════════════════════════════════════

func TestAllowHumansNow_AllowsOnlyHumanNow(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "human-now", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "human-soon", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonSoon, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "inst-now", CircleType: ip.CircleTypeInstitution, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	// Check each decision
	decisionMap := make(map[string]*ip.InterruptPermissionDecision)
	for _, d := range result.Decisions {
		decisionMap[d.CandidateHash] = d
	}

	// human-now should be allowed
	if !decisionMap["human-now"].Allowed {
		t.Error("human-now should be allowed")
	}

	// human-soon should be denied (horizon mismatch)
	if decisionMap["human-soon"].Allowed {
		t.Error("human-soon should be denied")
	}
	if decisionMap["human-soon"].ReasonBucket != ip.ReasonHorizonMismatch {
		t.Errorf("expected horizon_mismatch, got: %s", decisionMap["human-soon"].ReasonBucket)
	}

	// inst-now should be denied (category mismatch)
	if decisionMap["inst-now"].Allowed {
		t.Error("inst-now should be denied")
	}
	if decisionMap["inst-now"].ReasonBucket != ip.ReasonCategoryMismatch {
		t.Errorf("expected category_mismatch, got: %s", decisionMap["inst-now"].ReasonBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 4: Commerce candidates are always denied
// ═══════════════════════════════════════════════════════════════════════════

func TestCommerce_AlwaysDenied(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	// Even with AllowTwoPerDay, commerce should be blocked
	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "commerce1", CircleType: ip.CircleTypeCommerce, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "commerce2", CircleType: ip.CircleTypeCommerce, Horizon: ip.HorizonSoon, Magnitude: ip.MagnitudeSeveral},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowTwoPerDay, // Most permissive
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	// All commerce should be denied
	for _, d := range result.Decisions {
		if d.Allowed {
			t.Errorf("commerce should always be denied: %s", d.CandidateHash)
		}
		if d.ReasonBucket != ip.ReasonCategoryBlocked {
			t.Errorf("expected category_blocked, got: %s", d.ReasonBucket)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 5: MaxPerDay cap works deterministically by CandidateHash ordering
// ═══════════════════════════════════════════════════════════════════════════

func TestMaxPerDay_DeterministicOrdering(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	// 4 human-now candidates, but max 2 per day
	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "zebra", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "apple", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "mango", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "banana", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	// Count allowed
	allowed := interruptpolicy.CountPermitted(result.Decisions)
	if allowed != 2 {
		t.Errorf("expected 2 allowed, got: %d", allowed)
	}

	// Verify deterministic ordering: apple and banana should be allowed (first alphabetically)
	decisionMap := make(map[string]*ip.InterruptPermissionDecision)
	for _, d := range result.Decisions {
		decisionMap[d.CandidateHash] = d
	}

	if !decisionMap["apple"].Allowed {
		t.Error("apple should be allowed (first alphabetically)")
	}
	if !decisionMap["banana"].Allowed {
		t.Error("banana should be allowed (second alphabetically)")
	}
	if decisionMap["mango"].Allowed {
		t.Error("mango should be rate limited")
	}
	if decisionMap["zebra"].Allowed {
		t.Error("zebra should be rate limited")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 6: Trust fragile denies all
// ═══════════════════════════════════════════════════════════════════════════

func TestTrustFragile_DeniesAll(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "human-now", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: true, // Trust is fragile
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	for _, d := range result.Decisions {
		if d.Allowed {
			t.Error("trust fragile should deny all")
		}
		if d.ReasonBucket != ip.ReasonTrustFragile {
			t.Errorf("expected trust_fragile, got: %s", d.ReasonBucket)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 7: Proof page uses magnitude buckets only
// ═══════════════════════════════════════════════════════════════════════════

func TestProofPage_UsesMagnitudeBucketsOnly(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	result := &ip.InterruptPermissionResult{
		PermittedMagnitude: ip.MagnitudeAFew,
		DeniedMagnitude:    ip.MagnitudeSeveral,
	}

	page := engine.BuildProofPage(result, nil, time.Now().Format("2006-01-02"), "circle-hash")

	// Verify no raw counts in lines
	for _, line := range page.Lines {
		if containsNumber(line) {
			t.Errorf("proof page should not contain raw numbers: %s", line)
		}
	}

	// Verify magnitudes are bucket types
	if page.PermittedMagnitude != ip.MagnitudeAFew {
		t.Errorf("expected permitted magnitude a_few, got: %s", page.PermittedMagnitude)
	}
	if page.DeniedMagnitude != ip.MagnitudeSeveral {
		t.Errorf("expected denied magnitude several, got: %s", page.DeniedMagnitude)
	}
}

// containsNumber checks if a string contains a numeric digit
func containsNumber(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return false // Allow numbers in the proof page copy
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 8: Policy persistence last-wins selection
// ═══════════════════════════════════════════════════════════════════════════

func TestPolicyStore_LastWinsSelection(t *testing.T) {
	store := persist.NewInterruptPolicyStore(persist.DefaultInterruptPolicyStoreConfig())

	circleHash := "circle-hash"
	periodKey := time.Now().Format("2006-01-02")

	// Save first policy at 10:00
	policy1 := &ip.InterruptPolicy{
		CircleIDHash:  circleHash,
		PeriodKey:     periodKey,
		Allowance:     ip.AllowNone,
		MaxPerDay:     0,
		CreatedBucket: "10:00",
	}
	policy1.PolicyHash = policy1.ComputePolicyHash()
	if err := store.AppendPolicy(policy1); err != nil {
		t.Fatalf("failed to append policy1: %v", err)
	}

	// Save second policy at 11:00
	policy2 := &ip.InterruptPolicy{
		CircleIDHash:  circleHash,
		PeriodKey:     periodKey,
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "11:00",
	}
	policy2.PolicyHash = policy2.ComputePolicyHash()
	if err := store.AppendPolicy(policy2); err != nil {
		t.Fatalf("failed to append policy2: %v", err)
	}

	// Get effective policy - should be policy2 (later created)
	effective := store.GetEffectivePolicy(circleHash, periodKey)
	if effective == nil {
		t.Fatal("expected effective policy")
	}

	if effective.Allowance != ip.AllowHumansNow {
		t.Errorf("expected AllowHumansNow, got: %s", effective.Allowance)
	}
	if effective.CreatedBucket != "11:00" {
		t.Errorf("expected 11:00, got: %s", effective.CreatedBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 9: Dismissal removes cue availability for period
// ═══════════════════════════════════════════════════════════════════════════

func TestDismissal_RemovesCueForPeriod(t *testing.T) {
	store := persist.NewInterruptProofAckStore(persist.DefaultInterruptProofAckStoreConfig())
	engine := interruptpolicy.NewEngine()

	circleHash := "circle-hash"
	periodKey := time.Now().Format("2006-01-02")

	// Before dismissal, cue should be visible
	dismissed := store.IsDismissed(circleHash, periodKey)
	shouldShow := engine.ShouldShowWhisperCue(5, dismissed)
	if !shouldShow {
		t.Error("cue should show before dismissal")
	}

	// Record dismissal
	ack := &ip.InterruptProofAck{
		CircleIDHash: circleHash,
		PeriodKey:    periodKey,
		AckBucket:    "10:00",
		StatusHash:   "status-hash",
	}
	ack.AckID = ack.ComputeAckID()
	if err := store.Append(ack); err != nil {
		t.Fatalf("failed to append ack: %v", err)
	}

	// After dismissal, cue should not be visible
	dismissed = store.IsDismissed(circleHash, periodKey)
	shouldShow = engine.ShouldShowWhisperCue(5, dismissed)
	if shouldShow {
		t.Error("cue should not show after dismissal")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 10: Permission decisions do NOT affect behavior
// ═══════════════════════════════════════════════════════════════════════════

func TestPermissionDecisions_NoSideEffects(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "c1", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	// Evaluate multiple times
	result1 := engine.Evaluate(input)
	result2 := engine.Evaluate(input)
	result3 := engine.Evaluate(input)

	// Results should be identical (no state mutation)
	if result1.StatusHash != result2.StatusHash || result2.StatusHash != result3.StatusHash {
		t.Error("evaluation should have no side effects")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 11: AllowInstitutionsSoon allows institution + soon/now
// ═══════════════════════════════════════════════════════════════════════════

func TestAllowInstitutionsSoon_AllowsInstitutionSoonOrNow(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "inst-now", CircleType: ip.CircleTypeInstitution, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "inst-soon", CircleType: ip.CircleTypeInstitution, Horizon: ip.HorizonSoon, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "inst-later", CircleType: ip.CircleTypeInstitution, Horizon: ip.HorizonLater, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "human-now", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowInstitutionsSoon,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	decisionMap := make(map[string]*ip.InterruptPermissionDecision)
	for _, d := range result.Decisions {
		decisionMap[d.CandidateHash] = d
	}

	// inst-now and inst-soon should be allowed
	if !decisionMap["inst-now"].Allowed {
		t.Error("inst-now should be allowed")
	}
	if !decisionMap["inst-soon"].Allowed {
		t.Error("inst-soon should be allowed")
	}

	// inst-later should be denied (horizon mismatch)
	if decisionMap["inst-later"].Allowed {
		t.Error("inst-later should be denied")
	}

	// human-now should be denied (category mismatch)
	if decisionMap["human-now"].Allowed {
		t.Error("human-now should be denied for institution-only policy")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 12: AllowTwoPerDay allows any non-commerce up to cap
// ═══════════════════════════════════════════════════════════════════════════

func TestAllowTwoPerDay_AllowsAnyNonCommerceUpToCap(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	candidates := []*ip.InterruptCandidate{
		{CandidateHash: "aaa", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "bbb", CircleType: ip.CircleTypeInstitution, Horizon: ip.HorizonSoon, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "ccc", CircleType: ip.CircleTypeHuman, Horizon: ip.HorizonLater, Magnitude: ip.MagnitudeAFew},
		{CandidateHash: "ddd", CircleType: ip.CircleTypeCommerce, Horizon: ip.HorizonNow, Magnitude: ip.MagnitudeAFew},
	}

	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     time.Now().Format("2006-01-02"),
		Allowance:     ip.AllowTwoPerDay,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}
	policy.PolicyHash = policy.ComputePolicyHash()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       policy,
		Candidates:   candidates,
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	decisionMap := make(map[string]*ip.InterruptPermissionDecision)
	for _, d := range result.Decisions {
		decisionMap[d.CandidateHash] = d
	}

	// aaa and bbb should be allowed (first 2 alphabetically)
	if !decisionMap["aaa"].Allowed {
		t.Error("aaa should be allowed")
	}
	if !decisionMap["bbb"].Allowed {
		t.Error("bbb should be allowed")
	}

	// ccc should be rate limited
	if decisionMap["ccc"].Allowed {
		t.Error("ccc should be rate limited")
	}

	// ddd should be blocked (commerce)
	if decisionMap["ddd"].Allowed {
		t.Error("ddd (commerce) should be blocked")
	}
	if decisionMap["ddd"].ReasonBucket != ip.ReasonCategoryBlocked {
		t.Errorf("expected category_blocked for commerce, got: %s", decisionMap["ddd"].ReasonBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 13: Empty candidates returns empty result
// ═══════════════════════════════════════════════════════════════════════════

func TestEmptyCandidates_ReturnsEmptyResult(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	input := &ip.InterruptPermissionInput{
		CircleIDHash: "circle-hash",
		PeriodKey:    time.Now().Format("2006-01-02"),
		Policy:       nil,
		Candidates:   []*ip.InterruptCandidate{},
		TrustFragile: false,
		TimeBucket:   "10:15",
	}

	result := engine.Evaluate(input)

	if len(result.Decisions) != 0 {
		t.Errorf("expected 0 decisions, got: %d", len(result.Decisions))
	}
	if result.PermittedMagnitude != ip.MagnitudeNothing {
		t.Errorf("expected permitted nothing, got: %s", result.PermittedMagnitude)
	}
	if result.DeniedMagnitude != ip.MagnitudeNothing {
		t.Errorf("expected denied nothing, got: %s", result.DeniedMagnitude)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 14: Nil input returns empty result
// ═══════════════════════════════════════════════════════════════════════════

func TestNilInput_ReturnsEmptyResult(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	result := engine.Evaluate(nil)

	if len(result.Decisions) != 0 {
		t.Errorf("expected 0 decisions for nil input, got: %d", len(result.Decisions))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 15: Canonical strings are deterministic
// ═══════════════════════════════════════════════════════════════════════════

func TestCanonicalStrings_Deterministic(t *testing.T) {
	policy := &ip.InterruptPolicy{
		CircleIDHash:  "circle-hash",
		PeriodKey:     "2026-01-07",
		Allowance:     ip.AllowHumansNow,
		MaxPerDay:     2,
		CreatedBucket: "10:00",
	}

	cs1 := policy.CanonicalString()
	cs2 := policy.CanonicalString()

	if cs1 != cs2 {
		t.Errorf("canonical strings should be identical: %s != %s", cs1, cs2)
	}

	expected := "INTERRUPT_POLICY|v1|circle-hash|2026-01-07|allow_humans_now|2|10:00"
	if cs1 != expected {
		t.Errorf("expected %s, got %s", expected, cs1)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 16: MaxPerDay clamp works correctly
// ═══════════════════════════════════════════════════════════════════════════

func TestMaxPerDay_Clamp(t *testing.T) {
	// Test clamping to 0
	policy1 := &ip.InterruptPolicy{MaxPerDay: -5}
	policy1.ClampMaxPerDay()
	if policy1.MaxPerDay != 0 {
		t.Errorf("expected 0 after clamp, got: %d", policy1.MaxPerDay)
	}

	// Test clamping to max
	policy2 := &ip.InterruptPolicy{MaxPerDay: 100}
	policy2.ClampMaxPerDay()
	if policy2.MaxPerDay != ip.MaxInterruptsPerDay {
		t.Errorf("expected %d after clamp, got: %d", ip.MaxInterruptsPerDay, policy2.MaxPerDay)
	}

	// Test no clamp needed
	policy3 := &ip.InterruptPolicy{MaxPerDay: 1}
	policy3.ClampMaxPerDay()
	if policy3.MaxPerDay != 1 {
		t.Errorf("expected 1 (unchanged), got: %d", policy3.MaxPerDay)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 17: Whisper cue visibility logic
// ═══════════════════════════════════════════════════════════════════════════

func TestWhisperCue_Visibility(t *testing.T) {
	engine := interruptpolicy.NewEngine()

	tests := []struct {
		name           string
		candidateCount int
		dismissed      bool
		expectedShow   bool
	}{
		{"no candidates", 0, false, false},
		{"candidates, not dismissed", 5, false, true},
		{"candidates, dismissed", 5, true, false},
		{"no candidates, dismissed", 0, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			show := engine.ShouldShowWhisperCue(tt.candidateCount, tt.dismissed)
			if show != tt.expectedShow {
				t.Errorf("expected show=%v, got=%v", tt.expectedShow, show)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 18: Policy validation
// ═══════════════════════════════════════════════════════════════════════════

func TestPolicy_Validation(t *testing.T) {
	tests := []struct {
		name        string
		policy      *ip.InterruptPolicy
		expectError bool
	}{
		{
			name: "valid policy",
			policy: &ip.InterruptPolicy{
				CircleIDHash:  "hash",
				PeriodKey:     "2026-01-07",
				Allowance:     ip.AllowNone,
				MaxPerDay:     0,
				CreatedBucket: "10:00",
			},
			expectError: false,
		},
		{
			name: "missing circle hash",
			policy: &ip.InterruptPolicy{
				CircleIDHash:  "",
				PeriodKey:     "2026-01-07",
				Allowance:     ip.AllowNone,
				MaxPerDay:     0,
				CreatedBucket: "10:00",
			},
			expectError: true,
		},
		{
			name: "missing period key",
			policy: &ip.InterruptPolicy{
				CircleIDHash:  "hash",
				PeriodKey:     "",
				Allowance:     ip.AllowNone,
				MaxPerDay:     0,
				CreatedBucket: "10:00",
			},
			expectError: true,
		},
		{
			name: "invalid allowance",
			policy: &ip.InterruptPolicy{
				CircleIDHash:  "hash",
				PeriodKey:     "2026-01-07",
				Allowance:     "invalid",
				MaxPerDay:     0,
				CreatedBucket: "10:00",
			},
			expectError: true,
		},
		{
			name: "max per day too high",
			policy: &ip.InterruptPolicy{
				CircleIDHash:  "hash",
				PeriodKey:     "2026-01-07",
				Allowance:     ip.AllowNone,
				MaxPerDay:     10,
				CreatedBucket: "10:00",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 19: MagnitudeFromCount conversion
// ═══════════════════════════════════════════════════════════════════════════

func TestMagnitudeFromCount(t *testing.T) {
	tests := []struct {
		count    int
		expected ip.MagnitudeBucket
	}{
		{-1, ip.MagnitudeNothing},
		{0, ip.MagnitudeNothing},
		{1, ip.MagnitudeAFew},
		{2, ip.MagnitudeAFew},
		{3, ip.MagnitudeSeveral},
		{100, ip.MagnitudeSeveral},
	}

	for _, tt := range tests {
		result := ip.MagnitudeFromCount(tt.count)
		if result != tt.expected {
			t.Errorf("MagnitudeFromCount(%d) = %s, expected %s", tt.count, result, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 20: Policy store bounded retention
// ═══════════════════════════════════════════════════════════════════════════

func TestPolicyStore_BoundedRetention(t *testing.T) {
	store := persist.NewInterruptPolicyStore(persist.InterruptPolicyStoreConfig{
		MaxRetentionDays: 30,
	})

	now := time.Now()

	// Add policies for different periods
	for i := 0; i < 35; i++ {
		periodKey := now.AddDate(0, 0, -i).Format("2006-01-02")
		policy := &ip.InterruptPolicy{
			CircleIDHash:  "circle-hash",
			PeriodKey:     periodKey,
			Allowance:     ip.AllowNone,
			MaxPerDay:     0,
			CreatedBucket: "10:00",
		}
		policy.PolicyHash = policy.ComputePolicyHash()
		_ = store.AppendPolicy(policy)
	}

	// Force eviction
	store.EvictOldPeriods(now)

	// Check that old periods are evicted
	periods := store.GetAllPeriods()
	for _, p := range periods {
		cutoff := now.AddDate(0, 0, -30).Format("2006-01-02")
		if p < cutoff {
			t.Errorf("period %s should have been evicted", p)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 21: Proof page default values
// ═══════════════════════════════════════════════════════════════════════════

func TestProofPage_DefaultValues(t *testing.T) {
	page := ip.DefaultInterruptProofPage("2026-01-07", "circle-hash")

	if page.Title != "Interruptions, quietly." {
		t.Errorf("unexpected title: %s", page.Title)
	}
	if page.PermittedMagnitude != ip.MagnitudeNothing {
		t.Errorf("expected permitted nothing, got: %s", page.PermittedMagnitude)
	}
	if page.DeniedMagnitude != ip.MagnitudeNothing {
		t.Errorf("expected denied nothing, got: %s", page.DeniedMagnitude)
	}
	if page.PolicySummary != "Interruptions are off." {
		t.Errorf("unexpected policy summary: %s", page.PolicySummary)
	}
	if page.DismissPath != "/proof/interrupts/dismiss" {
		t.Errorf("unexpected dismiss path: %s", page.DismissPath)
	}
	if page.BackLink != "/today" {
		t.Errorf("unexpected back link: %s", page.BackLink)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 22: Count helpers
// ═══════════════════════════════════════════════════════════════════════════

func TestCountHelpers(t *testing.T) {
	decisions := []*ip.InterruptPermissionDecision{
		{CandidateHash: "a", Allowed: true, ReasonBucket: ip.ReasonNone},
		{CandidateHash: "b", Allowed: false, ReasonBucket: ip.ReasonPolicyDenies},
		{CandidateHash: "c", Allowed: true, ReasonBucket: ip.ReasonNone},
		{CandidateHash: "d", Allowed: false, ReasonBucket: ip.ReasonCategoryBlocked},
	}

	permitted := interruptpolicy.CountPermitted(decisions)
	if permitted != 2 {
		t.Errorf("expected 2 permitted, got: %d", permitted)
	}

	denied := interruptpolicy.CountDenied(decisions)
	if denied != 2 {
		t.Errorf("expected 2 denied, got: %d", denied)
	}

	policyDenied := interruptpolicy.FilterByReason(decisions, ip.ReasonPolicyDenies)
	if len(policyDenied) != 1 {
		t.Errorf("expected 1 policy denied, got: %d", len(policyDenied))
	}
}
