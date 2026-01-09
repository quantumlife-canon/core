// Package enforcementclamp provides the single choke-point enforcement wrapper.
//
// This package is the heart of Phase 44.2. ALL pipelines that can produce
// obligations/pressure MUST pass through ClampOutcome before any decision
// can escape to downstream components.
//
// CRITICAL INVARIANTS:
//   - HOLD-only: When any applicable contract says HOLD-only, clamp to HOLD or QUEUE_PROOF.
//   - Commerce always clamped to HOLD (redundant safety).
//   - Envelope (Phase 39) CANNOT override HOLD-only clamp.
//   - Interrupt policy (Phase 33) CANNOT override HOLD-only clamp.
//   - NEVER returns SURFACE, INTERRUPT_CANDIDATE, DELIVER, EXECUTE when contract present.
//   - Deterministic: same inputs => same outputs.
//   - NO time.Now() - clock injection required.
//   - NO goroutines.
//
// Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
package enforcementclamp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	ea "quantumlife/pkg/domain/enforcementaudit"
)

// ============================================================================
// Input/Output Types
// ============================================================================

// ClampInput contains the input for the clamp operation.
type ClampInput struct {
	CircleIDHash       string // SHA256 hash of circle ID
	IntersectionIDHash string // SHA256 hash of intersection ID (if applicable)
	RawDecisionKind    string // e.g., "hold", "surface", "interrupt_candidate"
	RawReasonBucket    string // e.g., "travel", "work", "health"
	ContractsSummary   ContractsSummary
}

// ContractsSummary summarizes the applicable contracts.
type ContractsSummary struct {
	HasHoldOnlyContract   bool // Delegated holding or trust transfer active
	HasTransferContract   bool // Cross-circle trust transfer active
	QueueProofRequested   bool // Contract requests queue_proof instead of hold
	IsCommerce            bool // Commerce pressure (always clamped)
	EnvelopeActive        bool // Phase 39 envelope active (cannot override)
	InterruptPolicyActive bool // Phase 33 policy active (cannot override)
}

// ClampOutput contains the result of the clamp operation.
type ClampOutput struct {
	ClampedDecisionKind ea.ClampedDecisionKind
	ClampedReasonBucket string
	ClampEvidenceHash   string
	WasClamped          bool
	ClampReason         string // e.g., "hold_only_contract", "commerce", "transfer_contract"
}

// ============================================================================
// Forbidden Decision Kinds
// ============================================================================

// ForbiddenDecisions are decision kinds that CANNOT be returned when a
// HOLD-only contract is active. This is the core invariant.
var ForbiddenDecisions = map[string]bool{
	"surface":              true,
	"SURFACE":              true,
	"interrupt_candidate":  true,
	"INTERRUPT_CANDIDATE":  true,
	"deliver":              true,
	"DELIVER":              true,
	"execute":              true,
	"EXECUTE":              true,
}

// IsForbiddenDecision checks if a decision is forbidden under HOLD-only mode.
func IsForbiddenDecision(decision string) bool {
	return ForbiddenDecisions[decision]
}

// ============================================================================
// Clamp Engine
// ============================================================================

// Engine provides the enforcement clamp wrapper.
type Engine struct{}

// NewEngine creates a new clamp engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ClampOutcome clamps the raw decision according to active contracts.
//
// CRITICAL: This is the single choke-point that enforces HOLD-only.
//
// Rules (in order of precedence):
//  1. If commerce => clamp to HOLD (commerce always held)
//  2. If any HOLD-only contract (delegated/transfer) => clamp forbidden decisions to HOLD/QUEUE_PROOF
//  3. Envelope (Phase 39) CANNOT override HOLD-only clamp
//  4. Interrupt policy (Phase 33) CANNOT override HOLD-only clamp
//
// Returns ClampOutput with evidence hash for audit trail.
func (e *Engine) ClampOutcome(input ClampInput) ClampOutput {
	// Build canonical evidence string for hashing
	evidence := e.buildEvidenceString(input)
	evidenceHash := computeHash(evidence)

	// Rule 1: Commerce is ALWAYS clamped to HOLD
	if input.ContractsSummary.IsCommerce {
		return ClampOutput{
			ClampedDecisionKind: ea.ClampedHold,
			ClampedReasonBucket: input.RawReasonBucket,
			ClampEvidenceHash:   evidenceHash,
			WasClamped:          true,
			ClampReason:         "commerce",
		}
	}

	// Rule 2: HOLD-only contract active => clamp forbidden decisions
	if input.ContractsSummary.HasHoldOnlyContract || input.ContractsSummary.HasTransferContract {
		if IsForbiddenDecision(input.RawDecisionKind) {
			// Determine if queue_proof or hold
			clampedKind := ea.ClampedHold
			if input.ContractsSummary.QueueProofRequested {
				clampedKind = ea.ClampedQueueProof
			}

			clampReason := "hold_only_contract"
			if input.ContractsSummary.HasTransferContract {
				clampReason = "transfer_contract"
			}

			return ClampOutput{
				ClampedDecisionKind: clampedKind,
				ClampedReasonBucket: input.RawReasonBucket,
				ClampEvidenceHash:   evidenceHash,
				WasClamped:          true,
				ClampReason:         clampReason,
			}
		}

		// Even non-forbidden decisions get normalized under contract
		return ClampOutput{
			ClampedDecisionKind: e.normalizeDecision(input.RawDecisionKind),
			ClampedReasonBucket: input.RawReasonBucket,
			ClampEvidenceHash:   evidenceHash,
			WasClamped:          false,
			ClampReason:         "",
		}
	}

	// No contract active: pass through normalized decision
	return ClampOutput{
		ClampedDecisionKind: e.normalizeDecision(input.RawDecisionKind),
		ClampedReasonBucket: input.RawReasonBucket,
		ClampEvidenceHash:   evidenceHash,
		WasClamped:          false,
		ClampReason:         "",
	}
}

// normalizeDecision normalizes a raw decision to a ClampedDecisionKind.
// When no contract is active, this just maps the string to the enum.
func (e *Engine) normalizeDecision(raw string) ea.ClampedDecisionKind {
	switch raw {
	case "hold", "HOLD":
		return ea.ClampedHold
	case "queue_proof", "QUEUE_PROOF":
		return ea.ClampedQueueProof
	case "no_effect", "NO_EFFECT":
		return ea.ClampedNoEffect
	default:
		// For unknown or forbidden decisions, default to HOLD for safety
		// This should only happen when no contract is active AND the
		// decision is something unexpected
		if IsForbiddenDecision(raw) {
			return ea.ClampedHold
		}
		return ea.ClampedNoEffect
	}
}

// buildEvidenceString builds a canonical evidence string for hashing.
func (e *Engine) buildEvidenceString(input ClampInput) string {
	return fmt.Sprintf("v1|%s|%s|%s|%s|%t|%t|%t|%t|%t",
		input.CircleIDHash,
		input.IntersectionIDHash,
		input.RawDecisionKind,
		input.RawReasonBucket,
		input.ContractsSummary.HasHoldOnlyContract,
		input.ContractsSummary.HasTransferContract,
		input.ContractsSummary.QueueProofRequested,
		input.ContractsSummary.IsCommerce,
		input.ContractsSummary.EnvelopeActive,
	)
}

// computeHash computes SHA256 of a string.
func computeHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ============================================================================
// Validation Helpers
// ============================================================================

// ValidateClampInput validates the clamp input.
func ValidateClampInput(input ClampInput) error {
	if input.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if len(input.CircleIDHash) != 64 {
		return fmt.Errorf("circle_id_hash must be 64 hex chars")
	}
	if input.RawDecisionKind == "" {
		return fmt.Errorf("raw_decision_kind required")
	}
	return nil
}

// ============================================================================
// Envelope/Policy Override Prevention
// ============================================================================

// CanEnvelopeOverride checks if an envelope can override the clamp.
// CRITICAL: This ALWAYS returns false when a HOLD-only contract is active.
func CanEnvelopeOverride(summary ContractsSummary) bool {
	if summary.HasHoldOnlyContract || summary.HasTransferContract {
		return false // NEVER override HOLD-only
	}
	return summary.EnvelopeActive
}

// CanInterruptPolicyOverride checks if an interrupt policy can override the clamp.
// CRITICAL: This ALWAYS returns false when a HOLD-only contract is active.
func CanInterruptPolicyOverride(summary ContractsSummary) bool {
	if summary.HasHoldOnlyContract || summary.HasTransferContract {
		return false // NEVER override HOLD-only
	}
	return summary.InterruptPolicyActive
}

// ============================================================================
// Batch Clamping
// ============================================================================

// ClampOutcomes clamps multiple inputs. Used for batch processing.
func (e *Engine) ClampOutcomes(inputs []ClampInput) []ClampOutput {
	outputs := make([]ClampOutput, len(inputs))
	for i, input := range inputs {
		outputs[i] = e.ClampOutcome(input)
	}
	return outputs
}

// ============================================================================
// Audit Support
// ============================================================================

// ClampStats tracks clamping statistics for audit.
type ClampStats struct {
	TotalClamped    int
	CommerceBlocked int
	ContractBlocked int
	TransferBlocked int
}

// ComputeStats computes statistics from a batch of outputs.
func ComputeStats(outputs []ClampOutput) ClampStats {
	stats := ClampStats{}
	for _, out := range outputs {
		if out.WasClamped {
			stats.TotalClamped++
			switch out.ClampReason {
			case "commerce":
				stats.CommerceBlocked++
			case "hold_only_contract":
				stats.ContractBlocked++
			case "transfer_contract":
				stats.TransferBlocked++
			}
		}
	}
	return stats
}
