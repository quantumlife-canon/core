// Package trusttransfer provides the engine for Phase 44: Cross-Circle Trust Transfer (HOLD-only).
//
// This engine manages trust transfer contracts and applies them to upstream decisions,
// clamping any escalation to HOLD-only outcomes.
//
// CRITICAL INVARIANTS:
//   - HOLD-only outcomes: returns ONLY NO_EFFECT, HOLD, or QUEUE_PROOF.
//     NEVER SURFACE, NEVER INTERRUPT_CANDIDATE, NEVER DELIVER, NEVER EXECUTE.
//   - Commerce excluded: commerce pressure returns NO_EFFECT, even under scope_all.
//   - NO time.Now() - clock injection required.
//   - NO goroutines.
//   - Deterministic: same inputs + clock => same hashes and outcomes.
//   - One active contract per FromCircle per period.
//
// Reference: docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md
package trusttransfer

import (
	"time"

	tt "quantumlife/pkg/domain/trusttransfer"
)

// ============================================================================
// Interfaces
// ============================================================================

// ContractStore stores trust transfer contracts.
type ContractStore interface {
	AppendContract(contract tt.TrustTransferContract) error
	GetActiveForFromCircle(fromCircleHash, periodKey string) *tt.TrustTransferContract
	ListContracts() []tt.TrustTransferContract
	UpdateState(contractHash string, state tt.TransferState) error
}

// RevocationStore stores trust transfer revocations.
type RevocationStore interface {
	AppendRevocation(rev tt.TrustTransferRevocation) error
	ListRevocations() []tt.TrustTransferRevocation
}

// Clock provides time injection.
type Clock interface {
	Now() time.Time
}

// ============================================================================
// Engine
// ============================================================================

// Engine manages trust transfer contracts and applies them.
type Engine struct {
	contractStore   ContractStore
	revocationStore RevocationStore
	clk             Clock
}

// NewEngine creates a new trust transfer engine.
func NewEngine(contractStore ContractStore, revocationStore RevocationStore, clk Clock) *Engine {
	return &Engine{
		contractStore:   contractStore,
		revocationStore: revocationStore,
		clk:             clk,
	}
}

// ============================================================================
// Time Helpers
// ============================================================================

// GetCurrentPeriodKey returns the current hour bucket from the clock.
// Format: "YYYY-MM-DD-HH"
func (e *Engine) GetCurrentPeriodKey() string {
	return e.clk.Now().UTC().Format("2006-01-02-15")
}

// GetCurrentDayKey returns the current day key from the clock.
// Format: "YYYY-MM-DD"
func (e *Engine) GetCurrentDayKey() string {
	return e.clk.Now().UTC().Format("2006-01-02")
}

// ============================================================================
// Proposal Building
// ============================================================================

// ProposalInput contains inputs for building a proposal.
type ProposalInput struct {
	FromCircleHash string
	ToCircleHash   string
	Scope          tt.TransferScope
	Duration       tt.TransferDuration
	Reason         tt.ProposalReason
}

// BuildProposal creates a new trust transfer proposal.
// Returns nil if a contract already exists for FromCircle.
func (e *Engine) BuildProposal(input ProposalInput) (*tt.TrustTransferProposal, error) {
	periodKey := e.GetCurrentPeriodKey()

	// Check for existing active contract
	existing := e.GetActiveForFromCircle(input.FromCircleHash)
	if existing != nil {
		return nil, nil // Reject: one active per FromCircle
	}

	proposal := &tt.TrustTransferProposal{
		FromCircleHash: input.FromCircleHash,
		ToCircleHash:   input.ToCircleHash,
		Scope:          input.Scope,
		Mode:           tt.ModeHoldOnly,
		Duration:       input.Duration,
		Reason:         input.Reason,
		PeriodKey:      periodKey,
	}

	// Compute proposal hash
	proposal.ProposalHash = proposal.ComputeHash()

	// Validate
	if err := proposal.Validate(); err != nil {
		return nil, err
	}

	return proposal, nil
}

// ============================================================================
// Contract Management
// ============================================================================

// AcceptProposal converts a proposal into an active contract.
// For now, acceptance is immediate (no signature verification).
func (e *Engine) AcceptProposal(proposal *tt.TrustTransferProposal) (*tt.TrustTransferContract, error) {
	if proposal == nil {
		return nil, nil
	}

	// Check for existing active contract (reject second)
	existing := e.GetActiveForFromCircle(proposal.FromCircleHash)
	if existing != nil {
		return nil, nil // One active per FromCircle
	}

	periodKey := e.GetCurrentPeriodKey()

	contract := &tt.TrustTransferContract{
		FromCircleHash:   proposal.FromCircleHash,
		ToCircleHash:     proposal.ToCircleHash,
		Scope:            proposal.Scope,
		Mode:             tt.ModeHoldOnly,
		Duration:         proposal.Duration,
		Reason:           proposal.Reason,
		State:            tt.StateActive,
		CreatedPeriodKey: periodKey,
	}

	// Compute contract hash
	contract.ContractHash = tt.ComputeContractHash(
		contract.FromCircleHash,
		contract.ToCircleHash,
		contract.Scope.CanonicalString(),
		contract.Duration.CanonicalString(),
		contract.Reason.CanonicalString(),
		contract.CreatedPeriodKey,
	)

	// Compute status hash
	contract.StatusHash = contract.ComputeHash()

	// Validate
	if err := contract.Validate(); err != nil {
		return nil, err
	}

	// Persist
	if e.contractStore != nil {
		if err := e.contractStore.AppendContract(*contract); err != nil {
			return nil, err
		}
	}

	return contract, nil
}

// Revoke revokes an active contract.
func (e *Engine) Revoke(contractHash, fromCircleHash string, reason tt.RevokeReason) (*tt.TrustTransferRevocation, error) {
	periodKey := e.GetCurrentPeriodKey()

	rev := &tt.TrustTransferRevocation{
		ContractHash:   contractHash,
		FromCircleHash: fromCircleHash,
		Reason:         reason,
		PeriodKey:      periodKey,
	}

	// Compute revocation hash
	rev.RevocationHash = rev.ComputeHash()

	// Validate
	if err := rev.Validate(); err != nil {
		return nil, err
	}

	// Update contract state
	if e.contractStore != nil {
		if err := e.contractStore.UpdateState(contractHash, tt.StateRevoked); err != nil {
			return nil, err
		}
	}

	// Persist revocation
	if e.revocationStore != nil {
		if err := e.revocationStore.AppendRevocation(*rev); err != nil {
			return nil, err
		}
	}

	return rev, nil
}

// IsActive checks if a contract is active for the given FromCircle.
func (e *Engine) IsActive(fromCircleHash string) bool {
	contract := e.GetActiveForFromCircle(fromCircleHash)
	return contract != nil && contract.IsActive()
}

// GetActiveForFromCircle returns the active contract for a FromCircle.
func (e *Engine) GetActiveForFromCircle(fromCircleHash string) *tt.TrustTransferContract {
	if e.contractStore == nil {
		return nil
	}
	periodKey := e.GetCurrentDayKey()
	return e.contractStore.GetActiveForFromCircle(fromCircleHash, periodKey)
}

// ============================================================================
// Apply Transfer (HOLD-only clamping)
// ============================================================================

// ApplyTransfer applies a trust transfer contract to an upstream decision.
// CRITICAL: This is where HOLD-only enforcement happens.
// - If decision is SURFACE, INTERRUPT_CANDIDATE, DELIVER, or EXECUTE => clamp to HOLD.
// - If commerce => return NO_EFFECT (commerce never affected).
// - If no active contract applies => return NO_EFFECT.
func (e *Engine) ApplyTransfer(contract *tt.TrustTransferContract, input tt.Phase32DecisionInput) *tt.TrustTransferEffect {
	// CRITICAL: Commerce is NEVER affected
	if input.IsCommerce() {
		effect := &tt.TrustTransferEffect{
			Decision:         tt.DecisionNoEffect,
			ContractHash:     "",
			OriginalDecision: input.Decision,
			WasClamped:       false,
		}
		effect.EffectHash = effect.ComputeHash()
		return effect
	}

	// No contract => no effect
	if contract == nil || !contract.IsActive() {
		effect := &tt.TrustTransferEffect{
			Decision:         tt.DecisionNoEffect,
			ContractHash:     "",
			OriginalDecision: input.Decision,
			WasClamped:       false,
		}
		effect.EffectHash = effect.ComputeHash()
		return effect
	}

	// Check if scope matches
	if !contract.Scope.MatchesCircleType(input.CircleType) {
		effect := &tt.TrustTransferEffect{
			Decision:         tt.DecisionNoEffect,
			ContractHash:     contract.ContractHash,
			OriginalDecision: input.Decision,
			WasClamped:       false,
		}
		effect.EffectHash = effect.ComputeHash()
		return effect
	}

	// CRITICAL: Clamp forbidden decisions to HOLD
	// Forbidden: SURFACE, INTERRUPT_CANDIDATE, DELIVER, EXECUTE
	if input.IsForbiddenDecision() {
		effect := &tt.TrustTransferEffect{
			Decision:         tt.DecisionHold,
			ContractHash:     contract.ContractHash,
			OriginalDecision: input.Decision,
			WasClamped:       true,
		}
		effect.EffectHash = effect.ComputeHash()
		return effect
	}

	// Already a HOLD-compatible decision, apply QUEUE_PROOF if contract wants it
	// For now, default to HOLD
	effect := &tt.TrustTransferEffect{
		Decision:         tt.DecisionHold,
		ContractHash:     contract.ContractHash,
		OriginalDecision: input.Decision,
		WasClamped:       false,
	}
	effect.EffectHash = effect.ComputeHash()
	return effect
}

// ApplyTransferForCircle looks up the active contract and applies it.
// This is a convenience method that combines GetActiveForFromCircle and ApplyTransfer.
func (e *Engine) ApplyTransferForCircle(fromCircleHash string, input tt.Phase32DecisionInput) *tt.TrustTransferEffect {
	contract := e.GetActiveForFromCircle(fromCircleHash)
	return e.ApplyTransfer(contract, input)
}

// ============================================================================
// Page Building
// ============================================================================

// BuildProofPage builds the proof page for a contract.
func (e *Engine) BuildProofPage(contract *tt.TrustTransferContract) *tt.TrustTransferProofPage {
	return tt.BuildProofFromContract(contract)
}

// BuildStatusPage builds the status page for the current state.
func (e *Engine) BuildStatusPage(fromCircleHash string) *tt.TrustTransferStatusPage {
	page := tt.NewDefaultStatusPage()

	contract := e.GetActiveForFromCircle(fromCircleHash)
	if contract != nil && contract.IsActive() {
		page.HasActiveContract = true
		page.ActiveContract = contract
		page.CanPropose = false
		page.BlockedReason = "active_contract_exists"
		page.Lines = []string{"Shared holding is active."}
	}

	page.StatusHash = page.ComputeHash()
	return page
}

// ============================================================================
// Cue Building
// ============================================================================

// BuildCue builds a cue for /today if shared holding is active.
func (e *Engine) BuildCue(fromCircleHash string) *tt.TrustTransferCue {
	contract := e.GetActiveForFromCircle(fromCircleHash)
	if contract == nil || !contract.IsActive() {
		return nil
	}

	return &tt.TrustTransferCue{
		Available:  true,
		CueText:    tt.DefaultCueText,
		Path:       tt.DefaultPath,
		StatusHash: contract.StatusHash,
	}
}

// ============================================================================
// Validation Helpers
// ============================================================================

// ValidateProposalInput validates proposal inputs.
func ValidateProposalInput(input ProposalInput) error {
	if input.FromCircleHash == "" {
		proposal := &tt.TrustTransferProposal{}
		return proposal.Validate() // Will return missing from_circle_hash
	}
	if input.ToCircleHash == "" {
		proposal := &tt.TrustTransferProposal{}
		return proposal.Validate() // Will return missing to_circle_hash
	}
	if input.FromCircleHash == input.ToCircleHash {
		proposal := &tt.TrustTransferProposal{
			FromCircleHash: input.FromCircleHash,
			ToCircleHash:   input.ToCircleHash,
		}
		return proposal.Validate() // Will return from and to must be different
	}
	if err := input.Scope.Validate(); err != nil {
		return err
	}
	if err := input.Duration.Validate(); err != nil {
		return err
	}
	if err := input.Reason.Validate(); err != nil {
		return err
	}
	return nil
}

// ============================================================================
// Decision Clamping Helpers (exported for testing)
// ============================================================================

// ClampDecision clamps a forbidden decision to HOLD.
// CRITICAL: This enforces the HOLD-only invariant.
// Forbidden: SURFACE, INTERRUPT_CANDIDATE, DELIVER, EXECUTE.
func ClampDecision(decision string) tt.TransferDecision {
	switch decision {
	case "surface", "SURFACE",
		"interrupt_candidate", "INTERRUPT_CANDIDATE",
		"deliver", "DELIVER",
		"execute", "EXECUTE":
		return tt.DecisionHold
	case "hold", "HOLD":
		return tt.DecisionHold
	case "queue_proof", "QUEUE_PROOF":
		return tt.DecisionQueueProof
	case "no_effect", "NO_EFFECT":
		return tt.DecisionNoEffect
	default:
		// Unknown decisions default to HOLD for safety
		return tt.DecisionHold
	}
}

// IsForbiddenDecision checks if a decision string is forbidden under HOLD-only mode.
func IsForbiddenDecision(decision string) bool {
	switch decision {
	case "surface", "SURFACE",
		"interrupt_candidate", "INTERRUPT_CANDIDATE",
		"deliver", "DELIVER",
		"execute", "EXECUTE":
		return true
	default:
		return false
	}
}
