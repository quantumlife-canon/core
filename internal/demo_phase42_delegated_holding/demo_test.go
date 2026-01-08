// Package demo_phase42_delegated_holding demonstrates Phase 42 Delegated Holding Contracts.
//
// These tests verify contract creation, eligibility, application, and revocation.
//
// CRITICAL INVARIANTS:
//   - NO goroutines. NO time.Now() - clock injection only.
//   - NO execution. NO delivery. NO interrupts. Only HOLD bias.
//   - ApplyContract can ONLY return NO_EFFECT, HOLD, or QUEUE_PROOF.
//   - Trust baseline required. One active contract per circle.
//   - Deterministic: same inputs + same clock => same hashes.
//
// Reference: docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md
package demo_phase42_delegated_holding

import (
	"testing"
	"time"

	engine "quantumlife/internal/delegatedholding"
	"quantumlife/internal/persist"
	dh "quantumlife/pkg/domain/delegatedholding"
	"quantumlife/pkg/domain/externalpressure"
)

// ============================================================================
// Stub Implementations
// ============================================================================

// StubTrustSource provides stub trust baseline checks.
type StubTrustSource struct {
	HasTrust bool
}

func (s *StubTrustSource) HasTrustBaseline(circleIDHash string) bool {
	return s.HasTrust
}

// StubPreviewSource provides stub interrupt preview checks.
type StubPreviewSource struct {
	HasPreview bool
}

func (s *StubPreviewSource) HasActivePreview(circleIDHash string) bool {
	return s.HasPreview
}

// StubClock provides deterministic time for testing.
type StubClock struct {
	FixedTime time.Time
}

func (c *StubClock) Now() time.Time {
	return c.FixedTime
}

// ============================================================================
// Test 1: Determinism - same CreateContractInput + clock => same ContractIDHash
// ============================================================================

func TestDeterminism_SameInputsSameHash(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	nowBucket := "2026-01-08-10"

	input := dh.CreateContractInput{
		CircleIDHash: "circle_hash_abc",
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    nowBucket,
	}

	hash1 := input.ComputeContractIDHash()
	hash2 := input.ComputeContractIDHash()

	if hash1 != hash2 {
		t.Errorf("expected same hash, got %s vs %s", hash1, hash2)
	}

	// Different engine invocations, same result
	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: now},
	)

	contract1 := eng.CreateContract(input)
	contract2 := eng.CreateContract(input)

	if contract1.ContractIDHash != contract2.ContractIDHash {
		t.Errorf("expected same ContractIDHash, got %s vs %s",
			contract1.ContractIDHash, contract2.ContractIDHash)
	}
}

// ============================================================================
// Test 2: Eligibility - trust baseline required
// ============================================================================

func TestEligibility_TrustBaselineRequired(t *testing.T) {
	circleIDHash := "circle_hash_abc"

	inputs := dh.DelegationInputs{
		CircleIDHash:              circleIDHash,
		HasTrustBaseline:          false, // No trust!
		HasActiveInterruptPreview: false,
		ExistingActiveContract:    false,
	}

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: false},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	decision := eng.CanCreateContract(inputs)

	if decision.Allowed {
		t.Error("expected creation to be disallowed without trust baseline")
	}
	if decision.Reason != dh.ReasonTrustMissing {
		t.Errorf("expected ReasonTrustMissing, got %s", decision.Reason)
	}
}

// ============================================================================
// Test 3: Eligibility - cannot create when interrupt preview active
// ============================================================================

func TestEligibility_CannotCreateWhenPreviewActive(t *testing.T) {
	circleIDHash := "circle_hash_abc"

	inputs := dh.DelegationInputs{
		CircleIDHash:              circleIDHash,
		HasTrustBaseline:          true,
		HasActiveInterruptPreview: true, // Preview active!
		ExistingActiveContract:    false,
	}

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: true},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	decision := eng.CanCreateContract(inputs)

	if decision.Allowed {
		t.Error("expected creation to be disallowed when preview active")
	}
	if decision.Reason != dh.ReasonInterruptPreviewActive {
		t.Errorf("expected ReasonInterruptPreviewActive, got %s", decision.Reason)
	}
}

// ============================================================================
// Test 4: One active contract per circle enforced
// ============================================================================

func TestEligibility_OneActivePerCircle(t *testing.T) {
	circleIDHash := "circle_hash_abc"

	inputs := dh.DelegationInputs{
		CircleIDHash:              circleIDHash,
		HasTrustBaseline:          true,
		HasActiveInterruptPreview: false,
		ExistingActiveContract:    true, // Contract already exists!
	}

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	decision := eng.CanCreateContract(inputs)

	if decision.Allowed {
		t.Error("expected creation to be disallowed when contract exists")
	}
	if decision.Reason != dh.ReasonActiveContractExists {
		t.Errorf("expected ReasonActiveContractExists, got %s", decision.Reason)
	}
}

// ============================================================================
// Test 5: Eligible when all conditions met
// ============================================================================

func TestEligibility_AllowedWhenConditionsMet(t *testing.T) {
	circleIDHash := "circle_hash_abc"

	inputs := dh.DelegationInputs{
		CircleIDHash:              circleIDHash,
		HasTrustBaseline:          true,
		HasActiveInterruptPreview: false,
		ExistingActiveContract:    false,
	}

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	decision := eng.CanCreateContract(inputs)

	if !decision.Allowed {
		t.Error("expected creation to be allowed")
	}
	if decision.Reason != dh.ReasonOK {
		t.Errorf("expected ReasonOK, got %s", decision.Reason)
	}
}

// ============================================================================
// Test 6: Revocation removes active contract
// ============================================================================

func TestRevocation_RemovesActiveContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: now},
	)

	// Create contract
	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)
	_ = eng.PersistContract(contract)

	// Verify active
	active := eng.GetActiveContract(circleIDHash)
	if active == nil {
		t.Fatal("expected active contract after creation")
	}

	// Revoke
	revokeInput := dh.RevokeContractInput{
		CircleIDHash:   circleIDHash,
		ContractIDHash: contract.ContractIDHash,
		NowBucket:      nowBucket,
	}
	_ = eng.RevokeContract(revokeInput)

	// Verify no longer active
	active = eng.GetActiveContract(circleIDHash)
	if active != nil {
		t.Error("expected no active contract after revocation")
	}
}

// ============================================================================
// Test 7: Expiry works by duration buckets - hour
// ============================================================================

func TestExpiry_HourDuration(t *testing.T) {
	createdTime := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	createdBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: createdTime},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationHour,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    createdBucket,
	}

	contract := eng.CreateContract(input)

	// Active immediately
	state := eng.ComputeState(contract, createdBucket)
	if state != dh.StateActive {
		t.Errorf("expected StateActive, got %s", state)
	}

	// Expired after 1 hour
	expiredBucket := "2026-01-08-11"
	state = eng.ComputeState(contract, expiredBucket)
	if state != dh.StateExpired {
		t.Errorf("expected StateExpired after 1 hour, got %s", state)
	}
}

// ============================================================================
// Test 8: Expiry works by duration buckets - day
// ============================================================================

func TestExpiry_DayDuration(t *testing.T) {
	createdBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    createdBucket,
	}

	contract := eng.CreateContract(input)

	// Still active after 10 hours
	state := eng.ComputeState(contract, "2026-01-08-20")
	if state != dh.StateActive {
		t.Errorf("expected StateActive after 10 hours, got %s", state)
	}

	// Expired after 24 hours
	state = eng.ComputeState(contract, "2026-01-09-10")
	if state != dh.StateExpired {
		t.Errorf("expected StateExpired after 24 hours, got %s", state)
	}
}

// ============================================================================
// Test 9: Expiry works by duration buckets - trip (7 days max)
// ============================================================================

func TestExpiry_TripDuration(t *testing.T) {
	createdBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationTrip,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    createdBucket,
	}

	contract := eng.CreateContract(input)

	// Still active after 5 days
	state := eng.ComputeState(contract, "2026-01-13-10")
	if state != dh.StateActive {
		t.Errorf("expected StateActive after 5 days, got %s", state)
	}

	// Expired after 7 days
	state = eng.ComputeState(contract, "2026-01-15-10")
	if state != dh.StateExpired {
		t.Errorf("expected StateExpired after 7 days, got %s", state)
	}
}

// ============================================================================
// Test 10: ApplyContract never returns SURFACE or INTERRUPT
// ============================================================================

func TestApplyContract_NeverReturnsSurfaceOrInterrupt(t *testing.T) {
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionQueueProof,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeSeveral,
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)

	pressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision := eng.ApplyContract(contract, pressure, nowBucket)

	// Must be one of the allowed results
	validResults := map[dh.ApplyResultKind]bool{
		dh.ResultNoEffect:   true,
		dh.ResultHold:       true,
		dh.ResultQueueProof: true,
	}

	if !validResults[decision.Result] {
		t.Errorf("unexpected result: %s (only NO_EFFECT, HOLD, QUEUE_PROOF allowed)", decision.Result)
	}
}

// ============================================================================
// Test 11: ApplyContract respects scope match - human
// ============================================================================

func TestApplyContract_RespectsHumanScope(t *testing.T) {
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeSeveral,
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)

	// Human scope, other category (should match)
	humanPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision := eng.ApplyContract(contract, humanPressure, nowBucket)
	if decision.Result != dh.ResultHold {
		t.Errorf("expected HOLD for human scope match, got %s", decision.Result)
	}

	// Institution category (should not match human scope)
	institutionPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryDelivery,
	}

	decision = eng.ApplyContract(contract, institutionPressure, nowBucket)
	if decision.Result != dh.ResultNoEffect {
		t.Errorf("expected NO_EFFECT for scope mismatch, got %s", decision.Result)
	}
}

// ============================================================================
// Test 12: ApplyContract respects scope match - institution
// ============================================================================

func TestApplyContract_RespectsInstitutionScope(t *testing.T) {
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeInstitution,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeSeveral,
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)

	// Institution category (should match)
	institutionPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryDelivery,
	}

	decision := eng.ApplyContract(contract, institutionPressure, nowBucket)
	if decision.Result != dh.ResultHold {
		t.Errorf("expected HOLD for institution scope match, got %s", decision.Result)
	}

	// External derived circle (institution scope)
	externalPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindExternalDerived,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision = eng.ApplyContract(contract, externalPressure, nowBucket)
	if decision.Result != dh.ResultHold {
		t.Errorf("expected HOLD for external derived circle, got %s", decision.Result)
	}
}

// ============================================================================
// Test 13: ApplyContract respects horizon constraint
// ============================================================================

func TestApplyContract_RespectsHorizonConstraint(t *testing.T) {
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonSoon, // Only SOON!
		MaxMagnitude: externalpressure.PressureMagnitudeSeveral,
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)

	// SOON horizon (should match)
	soonPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision := eng.ApplyContract(contract, soonPressure, nowBucket)
	if decision.Result != dh.ResultHold {
		t.Errorf("expected HOLD for SOON horizon, got %s", decision.Result)
	}

	// LATER horizon (exceeds max)
	laterPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonLater,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision = eng.ApplyContract(contract, laterPressure, nowBucket)
	if decision.Result != dh.ResultNoEffect {
		t.Errorf("expected NO_EFFECT for LATER horizon exceeding max, got %s", decision.Result)
	}
}

// ============================================================================
// Test 14: ApplyContract respects magnitude constraint
// ============================================================================

func TestApplyContract_RespectsMagnitudeConstraint(t *testing.T) {
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew, // Only A_FEW!
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)

	// A_FEW magnitude (should match)
	fewPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision := eng.ApplyContract(contract, fewPressure, nowBucket)
	if decision.Result != dh.ResultHold {
		t.Errorf("expected HOLD for A_FEW magnitude, got %s", decision.Result)
	}

	// SEVERAL magnitude (exceeds max)
	severalPressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeSeveral,
		Category:     externalpressure.PressureCategoryOther,
	}

	decision = eng.ApplyContract(contract, severalPressure, nowBucket)
	if decision.Result != dh.ResultNoEffect {
		t.Errorf("expected NO_EFFECT for SEVERAL magnitude exceeding max, got %s", decision.Result)
	}
}

// ============================================================================
// Test 15: ApplyContract action determines result type
// ============================================================================

func TestApplyContract_ActionDeterminesResult(t *testing.T) {
	nowBucket := "2026-01-08-10"
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: time.Now()},
	)

	pressure := dh.PressureInput{
		CircleIDHash: circleIDHash,
		CircleKind:   externalpressure.CircleKindSovereign,
		Horizon:      externalpressure.PressureHorizonSoon,
		Magnitude:    externalpressure.PressureMagnitudeAFew,
		Category:     externalpressure.PressureCategoryOther,
	}

	// ActionHoldSilently => ResultHold
	holdInput := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeSeveral,
		NowBucket:    nowBucket,
	}
	holdContract := eng.CreateContract(holdInput)
	decision := eng.ApplyContract(holdContract, pressure, nowBucket)
	if decision.Result != dh.ResultHold {
		t.Errorf("expected HOLD for ActionHoldSilently, got %s", decision.Result)
	}

	// ActionQueueProof => ResultQueueProof
	proofInput := dh.CreateContractInput{
		CircleIDHash: circleIDHash + "_2",
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionQueueProof,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeSeveral,
		NowBucket:    nowBucket,
	}
	proofContract := eng.CreateContract(proofInput)
	pressure.CircleIDHash = circleIDHash + "_2"
	decision = eng.ApplyContract(proofContract, pressure, nowBucket)
	if decision.Result != dh.ResultQueueProof {
		t.Errorf("expected QUEUE_PROOF for ActionQueueProof, got %s", decision.Result)
	}
}

// ============================================================================
// Test 16: Hash-only persistence - no raw identifiers
// ============================================================================

func TestPersistence_HashOnly(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	nowBucket := "2026-01-08-10"
	circleIDHash := "hashed_circle_id_abc123"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: now},
	)

	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    nowBucket,
	}

	contract := eng.CreateContract(input)
	_ = eng.PersistContract(contract)

	// Verify stored contract has hashed fields
	stored := store.GetActiveContract(circleIDHash, nowBucket)
	if stored == nil {
		t.Fatal("expected stored contract")
	}

	if stored.ContractIDHash == "" {
		t.Error("expected non-empty ContractIDHash")
	}
	if stored.CircleIDHash != circleIDHash {
		t.Errorf("expected CircleIDHash=%s, got %s", circleIDHash, stored.CircleIDHash)
	}
	if stored.StatusHash == "" {
		t.Error("expected non-empty StatusHash")
	}
}

// ============================================================================
// Test 17: Bounded retention - FIFO evicts beyond max records
// ============================================================================

func TestStore_FIFOEvictionBeyondMaxRecords(t *testing.T) {
	store := persist.NewDelegatedHoldingStore(nil)
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	nowBucket := "2026-01-08-10"

	// Add more than MaxRecords (200)
	for i := 0; i < 210; i++ {
		contract := &dh.DelegatedHoldingContract{
			ContractIDHash: "contract_" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			CircleIDHash:   "circle_" + string(rune('a'+i%26)),
			Scope:          dh.ScopeHuman,
			Action:         dh.ActionHoldSilently,
			Duration:       dh.DurationDay,
			MaxHorizon:     externalpressure.PressureHorizonLater,
			MaxMagnitude:   externalpressure.PressureMagnitudeAFew,
			State:          dh.StateActive,
			PeriodKey:      nowBucket,
		}
		contract.StatusHash = contract.ComputeHash()
		_ = store.UpsertActiveContract(contract.CircleIDHash, contract, now.Add(time.Duration(i)*time.Minute))
	}

	if store.Count() > dh.MaxRecords {
		t.Errorf("expected at most %d records, got %d", dh.MaxRecords, store.Count())
	}
}

// ============================================================================
// Test 18: Bounded retention - older than 30 days evicted
// ============================================================================

func TestStore_OldRecordsEvicted(t *testing.T) {
	store := persist.NewDelegatedHoldingStore(nil)
	oldTime := time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	nowBucket := "2026-01-08-10"

	// Add old contract
	oldContract := &dh.DelegatedHoldingContract{
		ContractIDHash: "old_contract",
		CircleIDHash:   "old_circle",
		Scope:          dh.ScopeHuman,
		Action:         dh.ActionHoldSilently,
		Duration:       dh.DurationDay,
		MaxHorizon:     externalpressure.PressureHorizonLater,
		MaxMagnitude:   externalpressure.PressureMagnitudeAFew,
		State:          dh.StateActive,
		PeriodKey:      "2025-11-01-10",
	}
	oldContract.StatusHash = oldContract.ComputeHash()
	_ = store.UpsertActiveContract(oldContract.CircleIDHash, oldContract, oldTime)

	// Force eviction with current time
	store.EvictOldRecords(now)

	// Should not find old contract
	found := store.GetActiveContract("old_circle", nowBucket)
	if found != nil {
		t.Error("expected old contract to be evicted")
	}
}

// ============================================================================
// Test 19: Enum validation works correctly
// ============================================================================

func TestEnumValidation(t *testing.T) {
	// Valid scope
	if err := dh.ScopeHuman.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid scope
	invalidScope := dh.DelegationScope("invalid_scope")
	if err := invalidScope.Validate(); err == nil {
		t.Error("expected validation error for invalid scope")
	}

	// Valid action
	if err := dh.ActionHoldSilently.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid duration
	if err := dh.DurationTrip.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid state
	if err := dh.StateActive.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid result
	if err := dh.ResultHold.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Valid reason
	if err := dh.ReasonOK.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// Test 20: BuildDelegatePage shows correct state
// ============================================================================

func TestBuildDelegatePage_ShowsCorrectState(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "circle_hash_abc"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: now},
	)

	// No contract - should show can create
	page := eng.BuildDelegatePage(circleIDHash)
	if page.HasActiveContract {
		t.Error("expected no active contract initially")
	}
	if !page.CanCreate {
		t.Error("expected CanCreate=true when eligible")
	}

	// Create contract
	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    "2026-01-08-10",
	}
	contract := eng.CreateContract(input)
	_ = eng.PersistContract(contract)

	// Should show active contract
	page = eng.BuildDelegatePage(circleIDHash)
	if !page.HasActiveContract {
		t.Error("expected HasActiveContract=true after creation")
	}
	if page.CanCreate {
		t.Error("expected CanCreate=false when contract exists")
	}
}

// ============================================================================
// Test 21: BuildProofPage returns correct data
// ============================================================================

func TestBuildProofPage_ReturnsCorrectData(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	circleIDHash := "circle_hash_abc"
	nowBucket := "2026-01-08-10"

	store := persist.NewDelegatedHoldingStore(nil)
	eng := engine.NewEngine(
		&StubTrustSource{HasTrust: true},
		&StubPreviewSource{HasPreview: false},
		store,
		&StubClock{FixedTime: now},
	)

	// No contract - empty proof page
	emptyPage := eng.BuildProofPage(circleIDHash)
	if emptyPage.HasContract {
		t.Error("expected no contract on empty proof page")
	}

	// Create contract
	input := dh.CreateContractInput{
		CircleIDHash: circleIDHash,
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionQueueProof,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    nowBucket,
	}
	contract := eng.CreateContract(input)
	_ = eng.PersistContract(contract)

	// Should show proof
	page := eng.BuildProofPage(circleIDHash)
	if !page.HasContract {
		t.Error("expected HasContract=true after creation")
	}
	if !page.IsActive {
		t.Error("expected IsActive=true for active contract")
	}
	if page.Scope != dh.ScopeHuman {
		t.Errorf("expected Scope=human, got %s", page.Scope)
	}
	if page.Action != dh.ActionQueueProof {
		t.Errorf("expected Action=queue_proof, got %s", page.Action)
	}
}

// ============================================================================
// Test 22: Proof page shows only hash prefixes (8 chars)
// ============================================================================

func TestProofPage_ShowsHashPrefixes(t *testing.T) {
	contract := &dh.DelegatedHoldingContract{
		ContractIDHash: "abcdef1234567890abcdef1234567890",
		CircleIDHash:   "circle123",
		Scope:          dh.ScopeHuman,
		Action:         dh.ActionHoldSilently,
		Duration:       dh.DurationDay,
		MaxHorizon:     externalpressure.PressureHorizonLater,
		MaxMagnitude:   externalpressure.PressureMagnitudeAFew,
		State:          dh.StateActive,
		PeriodKey:      "2026-01-08-10",
		StatusHash:     "1234567890abcdef1234567890abcdef",
	}

	proofPage := dh.BuildProofFromContract(contract)

	if len(proofPage.ContractHashPrefix) > 8 {
		t.Errorf("ContractHashPrefix too long: %d chars", len(proofPage.ContractHashPrefix))
	}
	if len(proofPage.StatusHashPrefix) > 8 {
		t.Errorf("StatusHashPrefix too long: %d chars", len(proofPage.StatusHashPrefix))
	}
}

// ============================================================================
// Test 23: Contract validation catches missing fields
// ============================================================================

func TestContractValidation_CatchesMissingFields(t *testing.T) {
	// Missing ContractIDHash
	c1 := &dh.DelegatedHoldingContract{
		CircleIDHash: "circle",
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		State:        dh.StateActive,
		PeriodKey:    "2026-01-08-10",
	}
	if err := c1.Validate(); err == nil {
		t.Error("expected validation error for missing ContractIDHash")
	}

	// Missing CircleIDHash
	c2 := &dh.DelegatedHoldingContract{
		ContractIDHash: "contract",
		Scope:          dh.ScopeHuman,
		Action:         dh.ActionHoldSilently,
		Duration:       dh.DurationDay,
		MaxHorizon:     externalpressure.PressureHorizonLater,
		MaxMagnitude:   externalpressure.PressureMagnitudeAFew,
		State:          dh.StateActive,
		PeriodKey:      "2026-01-08-10",
	}
	if err := c2.Validate(); err == nil {
		t.Error("expected validation error for missing CircleIDHash")
	}
}

// ============================================================================
// Test 24: CreateContractInput validation works
// ============================================================================

func TestCreateContractInput_Validation(t *testing.T) {
	// Valid input
	validInput := dh.CreateContractInput{
		CircleIDHash: "circle",
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    "2026-01-08-10",
	}
	if err := validInput.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid - missing NowBucket
	invalidInput := dh.CreateContractInput{
		CircleIDHash: "circle",
		Scope:        dh.ScopeHuman,
		Action:       dh.ActionHoldSilently,
		Duration:     dh.DurationDay,
		MaxHorizon:   externalpressure.PressureHorizonLater,
		MaxMagnitude: externalpressure.PressureMagnitudeAFew,
		NowBucket:    "",
	}
	if err := invalidInput.Validate(); err == nil {
		t.Error("expected validation error for missing NowBucket")
	}
}

// ============================================================================
// Test 25: CanonicalString format is deterministic
// ============================================================================

func TestCanonicalString_Deterministic(t *testing.T) {
	contract := &dh.DelegatedHoldingContract{
		ContractIDHash: "contract_hash",
		CircleIDHash:   "circle_hash",
		Scope:          dh.ScopeHuman,
		MaxHorizon:     externalpressure.PressureHorizonLater,
		MaxMagnitude:   externalpressure.PressureMagnitudeAFew,
		Action:         dh.ActionHoldSilently,
		Duration:       dh.DurationDay,
		State:          dh.StateActive,
		PeriodKey:      "2026-01-08-10",
	}

	str1 := contract.CanonicalString()
	str2 := contract.CanonicalString()

	if str1 != str2 {
		t.Errorf("expected same canonical string, got %s vs %s", str1, str2)
	}

	// Verify format
	if str1[:3] != "DHC" {
		t.Errorf("expected canonical string to start with DHC, got %s", str1[:3])
	}
}
