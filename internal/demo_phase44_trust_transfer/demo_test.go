// Package demo_phase44_trust_transfer demonstrates Phase 44 Cross-Circle Trust Transfer (HOLD-only).
//
// These tests verify proposal creation, contract acceptance, HOLD-only clamping, and revocation.
//
// CRITICAL INVARIANTS:
//   - NO goroutines. NO time.Now() - clock injection only.
//   - HOLD-only outcomes: ONLY NO_EFFECT, HOLD, or QUEUE_PROOF.
//   - NEVER SURFACE, INTERRUPT_CANDIDATE, DELIVER, or EXECUTE.
//   - Commerce excluded: commerce returns NO_EFFECT even under scope_all.
//   - One active contract per FromCircle per period.
//   - Deterministic: same inputs + same clock => same hashes.
//
// Reference: docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md
package demo_phase44_trust_transfer

import (
	"testing"
	"time"

	engine "quantumlife/internal/trusttransfer"
	tt "quantumlife/pkg/domain/trusttransfer"
)

// ============================================================================
// Stub Implementations
// ============================================================================

// StubContractStore provides stub contract storage for testing.
type StubContractStore struct {
	contracts map[string]tt.TrustTransferContract
}

func NewStubContractStore() *StubContractStore {
	return &StubContractStore{
		contracts: make(map[string]tt.TrustTransferContract),
	}
}

func (s *StubContractStore) AppendContract(contract tt.TrustTransferContract) error {
	s.contracts[contract.ContractHash] = contract
	return nil
}

func (s *StubContractStore) GetActiveForFromCircle(fromCircleHash, periodKey string) *tt.TrustTransferContract {
	for _, c := range s.contracts {
		if c.FromCircleHash == fromCircleHash && c.State == tt.StateActive {
			return &c
		}
	}
	return nil
}

func (s *StubContractStore) ListContracts() []tt.TrustTransferContract {
	result := make([]tt.TrustTransferContract, 0, len(s.contracts))
	for _, c := range s.contracts {
		result = append(result, c)
	}
	return result
}

func (s *StubContractStore) UpdateState(contractHash string, state tt.TransferState) error {
	if c, ok := s.contracts[contractHash]; ok {
		c.State = state
		s.contracts[contractHash] = c
	}
	return nil
}

// StubRevocationStore provides stub revocation storage for testing.
type StubRevocationStore struct {
	revocations map[string]tt.TrustTransferRevocation
}

func NewStubRevocationStore() *StubRevocationStore {
	return &StubRevocationStore{
		revocations: make(map[string]tt.TrustTransferRevocation),
	}
}

func (s *StubRevocationStore) AppendRevocation(rev tt.TrustTransferRevocation) error {
	s.revocations[rev.RevocationHash] = rev
	return nil
}

func (s *StubRevocationStore) ListRevocations() []tt.TrustTransferRevocation {
	result := make([]tt.TrustTransferRevocation, 0, len(s.revocations))
	for _, r := range s.revocations {
		result = append(result, r)
	}
	return result
}

// StubClock provides deterministic time for testing.
type StubClock struct {
	FixedTime time.Time
}

func (c *StubClock) Now() time.Time {
	return c.FixedTime
}

// ============================================================================
// Test 1: Determinism - same proposal input + clock => same proposal hash
// ============================================================================

func TestDeterminism_SameInputsSameHash(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal1, err1 := eng.BuildProposal(input)
	proposal2, err2 := eng.BuildProposal(input)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}

	if proposal1.ProposalHash != proposal2.ProposalHash {
		t.Errorf("expected same hash, got %s vs %s", proposal1.ProposalHash, proposal2.ProposalHash)
	}
}

// ============================================================================
// Test 2: Proposal creates valid proposal with HOLD-only mode
// ============================================================================

func TestProposal_HoldOnlyMode(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, err := eng.BuildProposal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proposal.Mode != tt.ModeHoldOnly {
		t.Errorf("expected ModeHoldOnly, got %s", proposal.Mode)
	}
}

// ============================================================================
// Test 3: One active contract per FromCircle enforced
// ============================================================================

func TestOneActivePerFromCircle(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	// First proposal and accept
	proposal1, _ := eng.BuildProposal(input)
	contract1, _ := eng.AcceptProposal(proposal1)

	if contract1 == nil {
		t.Fatal("expected first contract to be created")
	}

	// Second proposal should be rejected
	proposal2, _ := eng.BuildProposal(input)
	if proposal2 != nil {
		t.Error("expected second proposal to be nil (rejected)")
	}
}

// ============================================================================
// Test 4: AcceptProposal creates active contract
// ============================================================================

func TestAcceptProposal_CreatesActiveContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, err := eng.AcceptProposal(proposal)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if contract.State != tt.StateActive {
		t.Errorf("expected StateActive, got %s", contract.State)
	}

	if contract.Mode != tt.ModeHoldOnly {
		t.Errorf("expected ModeHoldOnly, got %s", contract.Mode)
	}
}

// ============================================================================
// Test 5: ApplyTransfer clamps SURFACE to HOLD
// ============================================================================

func TestApplyTransfer_ClampsSurfaceToHold(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	// Create decision input with SURFACE
	decisionInput := tt.Phase32DecisionInput{
		Decision:   "SURFACE",
		CircleType: "human",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionHold {
		t.Errorf("expected DecisionHold, got %s", effect.Decision)
	}
	if !effect.WasClamped {
		t.Error("expected WasClamped to be true")
	}
}

// ============================================================================
// Test 6: ApplyTransfer clamps INTERRUPT_CANDIDATE to HOLD
// ============================================================================

func TestApplyTransfer_ClampsInterruptToHold(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeAll,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonWork,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	decisionInput := tt.Phase32DecisionInput{
		Decision:   "INTERRUPT_CANDIDATE",
		CircleType: "human",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionHold {
		t.Errorf("expected DecisionHold, got %s", effect.Decision)
	}
	if !effect.WasClamped {
		t.Error("expected WasClamped to be true")
	}
}

// ============================================================================
// Test 7: ApplyTransfer clamps DELIVER to HOLD
// ============================================================================

func TestApplyTransfer_ClampsDeliverToHold(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeAll,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonHealth,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	decisionInput := tt.Phase32DecisionInput{
		Decision:   "DELIVER",
		CircleType: "institution",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionHold {
		t.Errorf("expected DecisionHold, got %s", effect.Decision)
	}
	if !effect.WasClamped {
		t.Error("expected WasClamped to be true")
	}
}

// ============================================================================
// Test 8: ApplyTransfer clamps EXECUTE to HOLD
// ============================================================================

func TestApplyTransfer_ClampsExecuteToHold(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeAll,
		Duration:       tt.DurationTrip,
		Reason:         tt.ReasonOverload,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	decisionInput := tt.Phase32DecisionInput{
		Decision:   "EXECUTE",
		CircleType: "human",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionHold {
		t.Errorf("expected DecisionHold, got %s", effect.Decision)
	}
	if !effect.WasClamped {
		t.Error("expected WasClamped to be true")
	}
}

// ============================================================================
// Test 9: Commerce always returns NO_EFFECT
// ============================================================================

func TestApplyTransfer_CommerceReturnsNoEffect(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeAll, // Even scope_all!
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonFamily,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	decisionInput := tt.Phase32DecisionInput{
		Decision:   "SURFACE",
		CircleType: "commerce",
		 // Commerce!
	}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionNoEffect {
		t.Errorf("expected DecisionNoEffect for commerce, got %s", effect.Decision)
	}
	if effect.WasClamped {
		t.Error("expected WasClamped to be false for commerce")
	}
}

// ============================================================================
// Test 10: Scope human only affects human circles
// ============================================================================

func TestApplyTransfer_ScopeHumanOnlyAffectsHuman(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman, // Human only
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	// Institution circle should not be affected
	decisionInput := tt.Phase32DecisionInput{
		Decision:   "SURFACE",
		CircleType: "institution",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionNoEffect {
		t.Errorf("expected DecisionNoEffect for institution with ScopeHuman, got %s", effect.Decision)
	}
}

// ============================================================================
// Test 11: Scope institution only affects institution circles
// ============================================================================

func TestApplyTransfer_ScopeInstitutionOnlyAffectsInstitution(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeInstitution, // Institution only
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonWork,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	// Human circle should not be affected
	decisionInput := tt.Phase32DecisionInput{
		Decision:   "SURFACE",
		CircleType: "human",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionNoEffect {
		t.Errorf("expected DecisionNoEffect for human with ScopeInstitution, got %s", effect.Decision)
	}
}

// ============================================================================
// Test 12: Revoke changes contract state to revoked
// ============================================================================

func TestRevoke_ChangesStateToRevoked(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	// Revoke the contract
	rev, err := eng.Revoke(contract.ContractHash, contract.FromCircleHash, tt.RevokeReasonDone)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rev == nil {
		t.Fatal("expected revocation to be created")
	}

	if rev.Reason != tt.RevokeReasonDone {
		t.Errorf("expected RevokeDone, got %s", rev.Reason)
	}
}

// ============================================================================
// Test 13: Revoked contract has no effect
// ============================================================================

func TestApplyTransfer_RevokedContractNoEffect(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	// Revoke the contract
	eng.Revoke(contract.ContractHash, contract.FromCircleHash, tt.RevokeReasonDone)

	// Apply transfer with revoked contract should return no effect
	contract.State = tt.StateRevoked // Simulate state change
	decisionInput := tt.Phase32DecisionInput{
		Decision:   "SURFACE",
		CircleType: "human",
			}

	effect := eng.ApplyTransfer(contract, decisionInput)

	if effect.Decision != tt.DecisionNoEffect {
		t.Errorf("expected DecisionNoEffect for revoked contract, got %s", effect.Decision)
	}
}

// ============================================================================
// Test 14: IsActive returns true for active contract
// ============================================================================

func TestIsActive_TrueForActiveContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	eng.AcceptProposal(proposal)

	if !eng.IsActive("from_circle_hash_abc") {
		t.Error("expected IsActive to return true")
	}
}

// ============================================================================
// Test 15: IsActive returns false when no contract
// ============================================================================

func TestIsActive_FalseWhenNoContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	if eng.IsActive("nonexistent_circle") {
		t.Error("expected IsActive to return false")
	}
}

// ============================================================================
// Test 16: BuildProofPage creates valid proof page
// ============================================================================

func TestBuildProofPage_CreatesValidPage(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	contract, _ := eng.AcceptProposal(proposal)

	page := eng.BuildProofPage(contract)

	if page == nil {
		t.Fatal("expected proof page to be created")
	}

	if page.Title == "" {
		t.Error("expected non-empty title")
	}

	if !page.HasContract {
		t.Error("expected HasContract to be true")
	}
}

// ============================================================================
// Test 17: BuildStatusPage with no contract
// ============================================================================

func TestBuildStatusPage_NoContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	page := eng.BuildStatusPage("nonexistent_circle")

	if page == nil {
		t.Fatal("expected status page to be created")
	}

	if page.HasActiveContract {
		t.Error("expected HasActiveContract to be false")
	}

	if !page.CanPropose {
		t.Error("expected CanPropose to be true when no contract exists")
	}
}

// ============================================================================
// Test 18: BuildStatusPage with active contract
// ============================================================================

func TestBuildStatusPage_WithActiveContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	eng.AcceptProposal(proposal)

	page := eng.BuildStatusPage("from_circle_hash_abc")

	if !page.HasActiveContract {
		t.Error("expected HasActiveContract to be true")
	}

	if page.CanPropose {
		t.Error("expected CanPropose to be false when contract exists")
	}
}

// ============================================================================
// Test 19: BuildCue returns nil when no active contract
// ============================================================================

func TestBuildCue_NilWhenNoContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	cue := eng.BuildCue("nonexistent_circle")

	if cue != nil {
		t.Error("expected cue to be nil when no contract exists")
	}
}

// ============================================================================
// Test 20: BuildCue returns available cue when active contract
// ============================================================================

func TestBuildCue_AvailableWhenActiveContract(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	eng.AcceptProposal(proposal)

	cue := eng.BuildCue("from_circle_hash_abc")

	if cue == nil {
		t.Fatal("expected cue to be created")
	}

	if !cue.Available {
		t.Error("expected cue.Available to be true")
	}

	if cue.CueText == "" {
		t.Error("expected non-empty CueText")
	}
}

// ============================================================================
// Test 21: Contract hash is deterministic
// ============================================================================

func TestContractHash_Deterministic(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	// Create two separate engines with same input
	store1 := NewStubContractStore()
	revStore1 := NewStubRevocationStore()
	eng1 := engine.NewEngine(store1, revStore1, &StubClock{FixedTime: now})

	store2 := NewStubContractStore()
	revStore2 := NewStubRevocationStore()
	eng2 := engine.NewEngine(store2, revStore2, &StubClock{FixedTime: now})

	proposal1, _ := eng1.BuildProposal(input)
	contract1, _ := eng1.AcceptProposal(proposal1)

	proposal2, _ := eng2.BuildProposal(input)
	contract2, _ := eng2.AcceptProposal(proposal2)

	if contract1.ContractHash != contract2.ContractHash {
		t.Errorf("expected same ContractHash, got %s vs %s",
			contract1.ContractHash, contract2.ContractHash)
	}
}

// ============================================================================
// Test 22: Validate proposal fails for same from and to circle
// ============================================================================

func TestProposal_SameFromAndToRejected(t *testing.T) {
	proposal := &tt.TrustTransferProposal{
		FromCircleHash: "same_circle",
		ToCircleHash:   "same_circle",
		Scope:          tt.ScopeHuman,
		Mode:           tt.ModeHoldOnly,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
		PeriodKey:      "2026-01-08-10",
	}

	err := proposal.Validate()
	if err == nil {
		t.Error("expected validation error for same from and to circle")
	}
}

// ============================================================================
// Test 23: All proposal reasons are valid
// ============================================================================

func TestProposalReasons_AllValid(t *testing.T) {
	reasons := []tt.ProposalReason{
		tt.ReasonTravel,
		tt.ReasonWork,
		tt.ReasonHealth,
		tt.ReasonOverload,
		tt.ReasonFamily,
	}

	for _, reason := range reasons {
		if err := reason.Validate(); err != nil {
			t.Errorf("expected %s to be valid, got error: %v", reason, err)
		}
	}
}

// ============================================================================
// Test 24: All revoke reasons are valid
// ============================================================================

func TestRevokeReasons_AllValid(t *testing.T) {
	reasons := []tt.RevokeReason{
		tt.RevokeReasonDone,
		tt.RevokeReasonTooMuch,
		tt.RevokeReasonChangedMind,
		tt.RevokeReasonTrustReset,
	}

	for _, reason := range reasons {
		if err := reason.Validate(); err != nil {
			t.Errorf("expected %s to be valid, got error: %v", reason, err)
		}
	}
}

// ============================================================================
// Test 25: ClampDecision helper works correctly
// ============================================================================

func TestClampDecision_HelperWorks(t *testing.T) {
	tests := []struct {
		input    string
		expected tt.TransferDecision
	}{
		{"SURFACE", tt.DecisionHold},
		{"surface", tt.DecisionHold},
		{"INTERRUPT_CANDIDATE", tt.DecisionHold},
		{"DELIVER", tt.DecisionHold},
		{"EXECUTE", tt.DecisionHold},
		{"HOLD", tt.DecisionHold},
		{"hold", tt.DecisionHold},
		{"NO_EFFECT", tt.DecisionNoEffect},
		{"QUEUE_PROOF", tt.DecisionQueueProof},
	}

	for _, tt := range tests {
		result := engine.ClampDecision(tt.input)
		if result != tt.expected {
			t.Errorf("ClampDecision(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

// ============================================================================
// Test 26: IsForbiddenDecision helper works correctly
// ============================================================================

func TestIsForbiddenDecision_HelperWorks(t *testing.T) {
	forbidden := []string{"SURFACE", "surface", "INTERRUPT_CANDIDATE", "DELIVER", "EXECUTE"}
	allowed := []string{"HOLD", "NO_EFFECT", "QUEUE_PROOF"}

	for _, d := range forbidden {
		if !engine.IsForbiddenDecision(d) {
			t.Errorf("expected %s to be forbidden", d)
		}
	}

	for _, d := range allowed {
		if engine.IsForbiddenDecision(d) {
			t.Errorf("expected %s to NOT be forbidden", d)
		}
	}
}

// ============================================================================
// Test 27: Scope MatchesCircleType for human scope
// ============================================================================

func TestScopeMatchesCircleType_Human(t *testing.T) {
	scope := tt.ScopeHuman

	if !scope.MatchesCircleType("human") {
		t.Error("expected human scope to match human circle")
	}

	if scope.MatchesCircleType("institution") {
		t.Error("expected human scope to NOT match institution circle")
	}

	if scope.MatchesCircleType("commerce") {
		t.Error("expected human scope to NOT match commerce circle")
	}
}

// ============================================================================
// Test 28: Scope MatchesCircleType for all scope (excludes commerce)
// ============================================================================

func TestScopeMatchesCircleType_AllExcludesCommerce(t *testing.T) {
	scope := tt.ScopeAll

	if !scope.MatchesCircleType("human") {
		t.Error("expected all scope to match human circle")
	}

	if !scope.MatchesCircleType("institution") {
		t.Error("expected all scope to match institution circle")
	}

	// CRITICAL: ScopeAll still excludes commerce
	if scope.MatchesCircleType("commerce") {
		t.Error("expected all scope to NOT match commerce (commerce ALWAYS excluded)")
	}
}

// ============================================================================
// Test 29: ApplyTransferForCircle convenience method
// ============================================================================

func TestApplyTransferForCircle_Works(t *testing.T) {
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	input := engine.ProposalInput{
		FromCircleHash: "from_circle_hash_abc",
		ToCircleHash:   "to_circle_hash_xyz",
		Scope:          tt.ScopeHuman,
		Duration:       tt.DurationDay,
		Reason:         tt.ReasonTravel,
	}

	store := NewStubContractStore()
	revStore := NewStubRevocationStore()
	eng := engine.NewEngine(store, revStore, &StubClock{FixedTime: now})

	proposal, _ := eng.BuildProposal(input)
	eng.AcceptProposal(proposal)

	decisionInput := tt.Phase32DecisionInput{
		Decision:   "SURFACE",
		CircleType: "human",
			}

	effect := eng.ApplyTransferForCircle("from_circle_hash_abc", decisionInput)

	if effect.Decision != tt.DecisionHold {
		t.Errorf("expected DecisionHold, got %s", effect.Decision)
	}
}

// ============================================================================
// Test 30: Duration display names are correct
// ============================================================================

func TestDuration_DisplayNames(t *testing.T) {
	tests := []struct {
		duration tt.TransferDuration
		expected string
	}{
		{tt.DurationHour, "One hour"},
		{tt.DurationDay, "One day"},
		{tt.DurationTrip, "Until revoked"},
	}

	for _, test := range tests {
		if test.duration.DisplayText() != test.expected {
			t.Errorf("expected %s display name to be %s, got %s",
				test.duration, test.expected, test.duration.DisplayText())
		}
	}
}
