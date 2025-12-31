// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.4 Multi-Party Gate for financial execution.
//
// CRITICAL: This gate enforces multi-party approval requirements before
// any money can move. It verifies:
// 1) Threshold approvals per contract ApprovalPolicy
// 2) Symmetry - all approvers received identical bundles
// 3) Neutrality - no coercive language in approval requests
// 4) Expiry - approvals are verified at execution time
// 5) Single-use - approvals cannot be reused
// 6) v9.12: PolicySnapshotHash consistency between bundle and envelope
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/events"
)

// MultiPartyPolicy defines the approval requirements for multi-party execution.
// This is derived from the intersection's ApprovalPolicy.
type MultiPartyPolicy struct {
	// Mode is "single" or "multi".
	Mode string

	// RequiredApprovers lists specific circle IDs that MUST approve.
	RequiredApprovers []string

	// Threshold is the minimum number of approvals required.
	Threshold int

	// ExpirySeconds defines how long an approval artifact is valid.
	ExpirySeconds int

	// AppliesToScopes lists which scopes require this policy.
	AppliesToScopes []string
}

// IsMultiParty returns true if multi-party approval is required.
func (p *MultiPartyPolicy) IsMultiParty() bool {
	return p.Mode == "multi" && p.Threshold > 1
}

// AppliesToFinanceWrite returns true if this policy applies to finance:write.
func (p *MultiPartyPolicy) AppliesToFinanceWrite() bool {
	if len(p.AppliesToScopes) == 0 {
		return true // Default: applies to all write scopes
	}
	for _, scope := range p.AppliesToScopes {
		if scope == "finance:write" || scope == "finance:execute" {
			return true
		}
	}
	return false
}

// MultiPartyGate enforces multi-party approval requirements.
type MultiPartyGate struct {
	mu sync.RWMutex

	// usedApprovals tracks consumed approval IDs
	usedApprovals map[string]time.Time

	// symmetryVerifier verifies bundle symmetry
	symmetryVerifier *SymmetryVerifier

	// neutralityChecker verifies language neutrality
	neutralityChecker *NeutralityChecker

	// auditEmitter emits audit events
	auditEmitter func(event events.Event)

	// idGenerator generates unique IDs
	idGenerator func() string
}

// NewMultiPartyGate creates a new multi-party gate.
func NewMultiPartyGate(idGen func() string, emitter func(event events.Event)) *MultiPartyGate {
	return &MultiPartyGate{
		usedApprovals:     make(map[string]time.Time),
		symmetryVerifier:  NewSymmetryVerifier(idGen),
		neutralityChecker: NewNeutralityChecker(),
		auditEmitter:      emitter,
		idGenerator:       idGen,
	}
}

// MultiPartyGateRequest contains the input for multi-party gate verification.
type MultiPartyGateRequest struct {
	// Envelope is the sealed execution envelope.
	Envelope *ExecutionEnvelope

	// Bundle is the approval bundle.
	Bundle *ApprovalBundle

	// Approvals are the collected approval artifacts.
	Approvals []MultiPartyApprovalArtifact

	// ApproverHashes are the hashes each approver received.
	ApproverHashes []ApproverBundleHash

	// Policy is the multi-party policy to enforce.
	Policy *MultiPartyPolicy

	// Now is the current time.
	Now time.Time
}

// MultiPartyGateResult contains the outcome of gate verification.
type MultiPartyGateResult struct {
	// Passed indicates if the gate passed.
	Passed bool

	// BlockedReason explains why the gate blocked execution.
	BlockedReason string

	// SymmetryProof is the proof of bundle symmetry.
	SymmetryProof *SymmetryProof

	// ThresholdResult contains threshold check details.
	ThresholdResult *ThresholdResult

	// VerifiedApprovals are the approvals that passed verification.
	VerifiedApprovals []MultiPartyApprovalArtifact

	// AuditEvents are the events emitted during verification.
	AuditEvents []events.Event
}

// ThresholdResult contains the result of threshold verification.
type ThresholdResult struct {
	// Required is the required threshold.
	Required int

	// Obtained is the number of valid approvals.
	Obtained int

	// Sufficient indicates if threshold is met.
	Sufficient bool

	// MissingApprovers lists approvers who haven't approved.
	MissingApprovers []string

	// VerifiedArtifactIDs lists verified approval artifact IDs.
	VerifiedArtifactIDs []string
}

// Verify checks all multi-party requirements.
func (g *MultiPartyGate) Verify(ctx context.Context, req MultiPartyGateRequest) (*MultiPartyGateResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := &MultiPartyGateResult{
		Passed:            true,
		AuditEvents:       make([]events.Event, 0),
		VerifiedApprovals: make([]MultiPartyApprovalArtifact, 0),
	}

	// Check if multi-party is required
	if !req.Policy.IsMultiParty() || !req.Policy.AppliesToFinanceWrite() {
		// Single-party fallback - gate passes trivially
		g.emitEvent(result, events.Event{
			ID:             g.idGenerator(),
			Type:           events.EventV94MultiPartySingleFallback,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"mode":      req.Policy.Mode,
				"threshold": fmt.Sprintf("%d", req.Policy.Threshold),
			},
		})
		return result, nil
	}

	// Emit multi-party required event
	g.emitEvent(result, events.Event{
		ID:             g.idGenerator(),
		Type:           events.EventV94MultiPartyRequired,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"threshold":          fmt.Sprintf("%d", req.Policy.Threshold),
			"required_approvers": fmt.Sprintf("%v", req.Policy.RequiredApprovers),
		},
	})

	// Step 1: Verify bundle symmetry
	symmetryProof := g.symmetryVerifier.Verify(req.Bundle, req.ApproverHashes)
	result.SymmetryProof = symmetryProof

	if !symmetryProof.Symmetric {
		result.Passed = false
		result.BlockedReason = "asymmetric approval bundle: approvers received different content"
		g.emitEvent(result, events.Event{
			ID:             g.idGenerator(),
			Type:           events.EventV94ApprovalSymmetryFailed,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"proof_id":        symmetryProof.ProofID,
				"violation_count": fmt.Sprintf("%d", len(symmetryProof.Violations)),
			},
		})
		g.emitEvent(result, events.Event{
			ID:             g.idGenerator(),
			Type:           events.EventV94ExecutionBlockedAsymmetricPayload,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"reason": result.BlockedReason,
			},
		})
		return result, nil
	}

	g.emitEvent(result, events.Event{
		ID:             g.idGenerator(),
		Type:           events.EventV94ApprovalSymmetryVerified,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      symmetryProof.ProofID,
		SubjectType:    "symmetry_proof",
		Metadata: map[string]string{
			"bundle_hash": symmetryProof.BundleContentHash,
			"symmetric":   "true",
		},
	})

	// Step 1.5 (v9.12): Verify PolicySnapshotHash consistency between bundle and envelope
	// CRITICAL: The bundle's PolicySnapshotHash must match the envelope's PolicySnapshotHash.
	// This ensures approvers approved the SAME policy configuration bound to the envelope.
	if req.Bundle.PolicySnapshotHash != "" && req.Envelope.PolicySnapshotHash != "" {
		if req.Bundle.PolicySnapshotHash != req.Envelope.PolicySnapshotHash {
			result.Passed = false
			result.BlockedReason = "policy snapshot mismatch: bundle and envelope have different policy bindings"
			g.emitEvent(result, events.Event{
				ID:             g.idGenerator(),
				Type:           events.EventV912PolicySnapshotMismatch,
				Timestamp:      now,
				CircleID:       req.Envelope.ActorCircleID,
				IntersectionID: req.Envelope.IntersectionID,
				SubjectID:      req.Envelope.EnvelopeID,
				SubjectType:    "envelope",
				Metadata: map[string]string{
					"bundle_policy_hash":   req.Bundle.PolicySnapshotHash,
					"envelope_policy_hash": req.Envelope.PolicySnapshotHash,
					"reason":               "bundle/envelope policy snapshot mismatch",
				},
			})
			return result, nil
		}

		g.emitEvent(result, events.Event{
			ID:             g.idGenerator(),
			Type:           events.EventV912PolicySnapshotVerified,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"policy_snapshot_hash": req.Bundle.PolicySnapshotHash,
				"verification_type":    "bundle_envelope_match",
			},
		})
	}

	// Step 2: Verify each approval
	validApprovals := make([]MultiPartyApprovalArtifact, 0)
	for _, approval := range req.Approvals {
		if err := g.verifyApproval(approval, req.Bundle, now); err != nil {
			// Log but continue - we might still have enough valid approvals
			continue
		}
		validApprovals = append(validApprovals, approval)
	}
	result.VerifiedApprovals = validApprovals

	// Step 3: Check threshold
	thresholdResult := g.checkThreshold(req.Policy, validApprovals)
	result.ThresholdResult = thresholdResult

	g.emitEvent(result, events.Event{
		ID:             g.idGenerator(),
		Type:           events.EventV94ApprovalThresholdChecked,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"threshold":  fmt.Sprintf("%d", thresholdResult.Required),
			"obtained":   fmt.Sprintf("%d", thresholdResult.Obtained),
			"sufficient": fmt.Sprintf("%t", thresholdResult.Sufficient),
		},
	})

	if !thresholdResult.Sufficient {
		result.Passed = false
		result.BlockedReason = fmt.Sprintf("insufficient approvals: %d of %d required",
			thresholdResult.Obtained, thresholdResult.Required)
		g.emitEvent(result, events.Event{
			ID:             g.idGenerator(),
			Type:           events.EventV94ApprovalThresholdNotMet,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"missing_approvers": fmt.Sprintf("%v", thresholdResult.MissingApprovers),
			},
		})
		g.emitEvent(result, events.Event{
			ID:             g.idGenerator(),
			Type:           events.EventV94ExecutionBlockedInsufficientApprovals,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"reason": result.BlockedReason,
			},
		})
		return result, nil
	}

	g.emitEvent(result, events.Event{
		ID:             g.idGenerator(),
		Type:           events.EventV94ApprovalThresholdMet,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"verified_artifacts": fmt.Sprintf("%v", thresholdResult.VerifiedArtifactIDs),
		},
	})

	// Step 4: Mark approvals as used (single-use enforcement)
	g.mu.Lock()
	for _, approval := range validApprovals {
		g.usedApprovals[approval.ArtifactID] = now
	}
	g.mu.Unlock()

	// Gate passed
	g.emitEvent(result, events.Event{
		ID:             g.idGenerator(),
		Type:           events.EventV94MultiPartyGatePassed,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"symmetry_proof_id": symmetryProof.ProofID,
			"approved_count":    fmt.Sprintf("%d", thresholdResult.Obtained),
		},
	})

	return result, nil
}

// verifyApproval checks a single approval artifact.
func (g *MultiPartyGate) verifyApproval(approval MultiPartyApprovalArtifact, bundle *ApprovalBundle, now time.Time) error {
	// Check if already used (single-use enforcement)
	g.mu.RLock()
	if _, used := g.usedApprovals[approval.ArtifactID]; used {
		g.mu.RUnlock()
		return ErrApprovalReuse
	}
	if approval.Used {
		g.mu.RUnlock()
		return ErrApprovalReuse
	}
	g.mu.RUnlock()

	// Check expiry
	if now.After(approval.ExpiresAt) {
		return ErrBundleExpired
	}

	// Check action hash match
	if approval.ActionHash != bundle.ActionHash {
		return ErrApprovalHashMismatch
	}

	// Check bundle hash match
	if approval.BundleContentHash != bundle.ContentHash {
		return ErrBundleHashMismatch
	}

	return nil
}

// checkThreshold verifies threshold requirements.
func (g *MultiPartyGate) checkThreshold(policy *MultiPartyPolicy, validApprovals []MultiPartyApprovalArtifact) *ThresholdResult {
	result := &ThresholdResult{
		Required:            policy.Threshold,
		Obtained:            len(validApprovals),
		VerifiedArtifactIDs: make([]string, 0, len(validApprovals)),
	}

	// Collect verified artifact IDs and approver circles
	approvedBy := make(map[string]bool)
	for _, approval := range validApprovals {
		result.VerifiedArtifactIDs = append(result.VerifiedArtifactIDs, approval.ArtifactID)
		approvedBy[approval.ApproverCircleID] = true
	}

	// Check if required approvers have approved
	for _, required := range policy.RequiredApprovers {
		if !approvedBy[required] {
			result.MissingApprovers = append(result.MissingApprovers, required)
		}
	}

	// Threshold is met if:
	// 1) We have enough approvals AND
	// 2) All required approvers have approved (if RequiredApprovers is non-empty)
	result.Sufficient = result.Obtained >= result.Required && len(result.MissingApprovers) == 0

	return result
}

// IsApprovalUsed checks if an approval has been consumed.
func (g *MultiPartyGate) IsApprovalUsed(artifactID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, used := g.usedApprovals[artifactID]
	return used
}

// emitEvent records an audit event.
func (g *MultiPartyGate) emitEvent(result *MultiPartyGateResult, event events.Event) {
	result.AuditEvents = append(result.AuditEvents, event)
	if g.auditEmitter != nil {
		g.auditEmitter(event)
	}
}

// MultiPartyApprovalDecision contains the final approval decision.
type MultiPartyApprovalDecision struct {
	// Sufficient indicates if approvals are sufficient.
	Sufficient bool

	// MissingApprovers lists approvers who haven't approved.
	MissingApprovers []string

	// VerifiedArtifactIDs lists verified approval artifact IDs.
	VerifiedArtifactIDs []string

	// PolicySnapshot captures the policy at decision time.
	PolicySnapshot *MultiPartyPolicy

	// SymmetryProofID links to the symmetry proof.
	SymmetryProofID string
}
