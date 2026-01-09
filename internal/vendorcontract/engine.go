// Package vendorcontract provides the engine for Phase 49: Vendor Reality Contracts.
//
// This engine validates vendor contracts, computes effective caps, and provides
// a single choke-point clamp function for pressure reduction.
//
// CRITICAL: No time.Now() - clock must be injected.
// CRITICAL: No goroutines.
// CRITICAL: Contracts can only REDUCE pressure, never increase it.
// CRITICAL: Commerce vendors capped at SURFACE_ONLY regardless of declaration.
// CRITICAL: Invalid contracts default to HOLD_ONLY.
//
// Reference: docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
package vendorcontract

import (
	"time"

	domain "quantumlife/pkg/domain/vendorcontract"
)

// ============================================================================
// Engine
// ============================================================================

// Engine provides vendor contract validation and clamping.
// CRITICAL: This engine is pure and deterministic.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new vendor contract engine with injected clock.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// ============================================================================
// Contract Validation
// ============================================================================

// ValidateContract validates a vendor contract.
// Returns deterministic error messages (string constants, no dynamic values).
func (e *Engine) ValidateContract(c domain.VendorContract) error {
	return c.Validate()
}

// ============================================================================
// Effective Cap Computation
// ============================================================================

// ComputeEffectiveCap computes the effective pressure cap for a contract.
//
// Rules:
// 1. Default cap is allow_hold_only if anything invalid
// 2. If isCommerce => cap at min(requested cap, allow_surface_only)
// 3. Otherwise cap is requested allowance
// 4. Never allow anything beyond allow_interrupt_candidate
func (e *Engine) ComputeEffectiveCap(c domain.VendorContract, circleTypeHint string, isCommerce bool) (domain.PressureAllowance, domain.ContractReasonBucket) {
	// Validate contract first
	if err := c.Validate(); err != nil {
		return domain.AllowHoldOnly, domain.ReasonInvalid
	}

	requestedCap := c.AllowedPressure

	// Commerce vendors are ALWAYS capped at SURFACE_ONLY
	if isCommerce || c.Scope == domain.ScopeCommerce {
		if requestedCap.Level() > domain.AllowSurfaceOnly.Level() {
			return domain.AllowSurfaceOnly, domain.ReasonCommerceCapped
		}
		return requestedCap, domain.ReasonOK
	}

	// Non-commerce: use requested cap (already bounded by enum)
	return requestedCap, domain.ReasonOK
}

// ============================================================================
// Decision Outcome
// ============================================================================

// DecideOutcome processes a contract and returns the outcome.
//
// - Accepted is true only if ValidateContract passes
// - Even if accepted, EffectiveCap may be reduced (commerce cap)
// - Reason indicates ok, invalid, commerce_capped, etc.
func (e *Engine) DecideOutcome(c domain.VendorContract, isCommerce bool) domain.VendorContractOutcome {
	// Validate first
	if err := e.ValidateContract(c); err != nil {
		return domain.VendorContractOutcome{
			Accepted:     false,
			EffectiveCap: domain.AllowHoldOnly,
			Reason:       domain.ReasonInvalid,
		}
	}

	// Compute effective cap
	effectiveCap, reason := e.ComputeEffectiveCap(c, "", isCommerce)

	return domain.VendorContractOutcome{
		Accepted:     true,
		EffectiveCap: effectiveCap,
		Reason:       reason,
	}
}

// ============================================================================
// Pressure Clamping (Single Choke-Point)
// ============================================================================

// ClampPressureAllowance clamps a pressure allowance to the contract cap.
// Returns min(current, contractCap) based on ordering:
//
//	allow_hold_only < allow_surface_only < allow_interrupt_candidate
//
// CRITICAL: This is deterministic and total (handles all cases).
// CRITICAL: This can only REDUCE pressure, never increase it.
func (e *Engine) ClampPressureAllowance(current domain.PressureAllowance, contractCap domain.PressureAllowance) domain.PressureAllowance {
	// If current is already at or below cap, no change
	if current.Level() <= contractCap.Level() {
		return current
	}
	// Otherwise clamp to cap
	return contractCap
}

// ClampDecisionKind maps decision kinds to allowance levels and clamps.
// Returns the clamped decision kind.
//
// Input mapping:
//
//	"hold", "HOLD" => allow_hold_only
//	"surface", "SURFACE" => allow_surface_only
//	"interrupt_candidate", "INTERRUPT_CANDIDATE" => allow_interrupt_candidate
//
// Output mapping (reverse):
//
//	allow_hold_only => "hold"
//	allow_surface_only => "surface"
//	allow_interrupt_candidate => "interrupt_candidate"
func (e *Engine) ClampDecisionKind(rawDecision string, contractCap domain.PressureAllowance) string {
	// Map raw decision to allowance
	var currentAllowance domain.PressureAllowance
	switch rawDecision {
	case "hold", "HOLD":
		currentAllowance = domain.AllowHoldOnly
	case "surface", "SURFACE":
		currentAllowance = domain.AllowSurfaceOnly
	case "interrupt_candidate", "INTERRUPT_CANDIDATE":
		currentAllowance = domain.AllowInterruptCandidate
	default:
		// Unknown defaults to HOLD for safety
		currentAllowance = domain.AllowHoldOnly
	}

	// Clamp
	clamped := e.ClampPressureAllowance(currentAllowance, contractCap)

	// Map back to decision kind
	switch clamped {
	case domain.AllowHoldOnly:
		return "hold"
	case domain.AllowSurfaceOnly:
		return "surface"
	case domain.AllowInterruptCandidate:
		return "interrupt_candidate"
	default:
		return "hold"
	}
}

// WasClamped checks if clamping changed the decision.
func (e *Engine) WasClamped(rawDecision string, contractCap domain.PressureAllowance) bool {
	clamped := e.ClampDecisionKind(rawDecision, contractCap)

	// Normalize raw decision for comparison
	switch rawDecision {
	case "hold", "HOLD":
		return clamped != "hold"
	case "surface", "SURFACE":
		return clamped != "surface"
	case "interrupt_candidate", "INTERRUPT_CANDIDATE":
		return clamped != "interrupt_candidate"
	default:
		return false
	}
}

// ============================================================================
// Proof Building
// ============================================================================

// BuildProofLine builds a proof line from contract data.
func (e *Engine) BuildProofLine(vendorHash string, scope domain.ContractScope, cap domain.PressureAllowance, period string) domain.VendorContractProofLine {
	line := domain.VendorContractProofLine{
		VendorCircleHash: vendorHash,
		Scope:            scope,
		EffectiveCap:     cap,
		PeriodKey:        period,
	}
	line.ProofHash = line.ComputeProofHash()
	return line
}

// BuildProofPage builds the UI model for vendor contract proof.
// CRITICAL: No vendor names, URLs, or identifiers.
func (e *Engine) BuildProofPage(proofLines []domain.VendorContractProofLine) domain.VendorProofPage {
	var lines []string
	if len(proofLines) > 0 {
		lines = []string{
			"Vendor contracts define pressure limits.",
			"Contracts can only reduce pressure, never increase it.",
		}
	} else {
		lines = []string{
			"No active vendor contracts.",
		}
	}

	return domain.VendorProofPage{
		Title:      "Vendor Contracts",
		Lines:      lines,
		ProofLines: proofLines,
		StatusHash: domain.ComputeProofPageStatusHash(proofLines),
	}
}

// BuildCue builds the whisper cue for vendor contracts.
func (e *Engine) BuildCue(proofLines []domain.VendorContractProofLine, dismissed bool) domain.VendorProofCue {
	available := len(proofLines) > 0 && !dismissed

	var text string
	if available {
		text = "Vendor contract limits active."
	}

	return domain.VendorProofCue{
		Available:  available,
		Text:       text,
		Path:       "/proof/vendor",
		StatusHash: domain.ComputeVendorCueStatusHash(len(proofLines), available),
	}
}

// ShouldShowCue determines if the cue should be shown.
func (e *Engine) ShouldShowCue(proofLines []domain.VendorContractProofLine, dismissed bool) bool {
	return len(proofLines) > 0 && !dismissed
}

// ============================================================================
// Contract Record Building
// ============================================================================

// BuildContractRecord builds a contract record from a contract and outcome.
func (e *Engine) BuildContractRecord(c domain.VendorContract, outcome domain.VendorContractOutcome, periodKey string) domain.VendorContractRecord {
	status := domain.StatusActive
	if !outcome.Accepted {
		status = domain.StatusRevoked
	}

	return domain.VendorContractRecord{
		ContractHash:     c.ComputeContractHash(),
		VendorCircleHash: c.VendorCircleHash,
		Scope:            c.Scope,
		EffectiveCap:     outcome.EffectiveCap,
		Status:           status,
		CreatedAtBucket:  periodKey,
		PeriodKey:        periodKey,
	}
}

// ============================================================================
// Helper: Period Key from Clock
// ============================================================================

// CurrentPeriodKey returns the current period key from injected clock.
func (e *Engine) CurrentPeriodKey() string {
	return e.clock().Format("2006-01-02")
}
