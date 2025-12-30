// Package impl_inmem provides an in-memory implementation of the negotiation interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation uses deterministic logic only. No LLM/SLM calls.
// Production may use LLM/SLM for proposal analysis and counterproposal generation.
package impl_inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/audit"
	"quantumlife/internal/intersection"
	"quantumlife/internal/negotiation"
	"quantumlife/pkg/events"
)

// Engine implements negotiation.NegotiationLoop with in-memory storage.
type Engine struct {
	mu              sync.RWMutex
	proposals       map[string]*negotiation.ProposalThread
	proposalCounter int

	intRuntime  intersection.Runtime
	auditLogger audit.Logger
	trustStore  *TrustStore
}

// EngineConfig contains configuration for the negotiation engine.
type EngineConfig struct {
	IntRuntime  intersection.Runtime
	AuditLogger audit.Logger
	TrustStore  *TrustStore
}

// NewEngine creates a new in-memory negotiation engine.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		proposals:   make(map[string]*negotiation.ProposalThread),
		intRuntime:  cfg.IntRuntime,
		auditLogger: cfg.AuditLogger,
		trustStore:  cfg.TrustStore,
	}
}

// SubmitProposal submits a new proposal for an intersection amendment.
func (e *Engine) SubmitProposal(ctx context.Context, intersectionID string, req negotiation.SubmitProposalRequest) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Verify intersection exists
	_, err := e.intRuntime.Get(ctx, intersectionID)
	if err != nil {
		return "", fmt.Errorf("intersection not found: %w", err)
	}

	// Verify issuer is a party
	isParty, err := e.intRuntime.IsParty(ctx, intersectionID, req.IssuerCircleID)
	if err != nil {
		return "", fmt.Errorf("failed to check party: %w", err)
	}
	if !isParty {
		return "", fmt.Errorf("issuer %s is not a party to intersection %s", req.IssuerCircleID, intersectionID)
	}

	// Get all parties for approval tracking
	parties, err := e.intRuntime.ListParties(ctx, intersectionID)
	if err != nil {
		return "", fmt.Errorf("failed to list parties: %w", err)
	}

	e.proposalCounter++
	proposalID := fmt.Sprintf("prop-%d", e.proposalCounter)

	// Initialize approvals map
	approvals := make(map[string]bool)
	for _, party := range parties {
		approvals[party.CircleID] = false
	}
	// Issuer implicitly approves their own proposal
	approvals[req.IssuerCircleID] = true

	proposal := &negotiation.ProposalThread{
		ID:             proposalID,
		IntersectionID: intersectionID,
		IssuerCircleID: req.IssuerCircleID,
		ProposalType:   req.ProposalType,
		State:          negotiation.ProposalStatePending,
		Reason:         req.Reason,
		ScopeAdditions: req.ScopeAdditions,
		ScopeRemovals:  req.ScopeRemovals,
		CeilingChanges: req.CeilingChanges,
		ActionSpec:     req.ActionSpec,
		Approvals:      approvals,
		Rejections:     make(map[string]string),
		CounterIDs:     []string{},
		CreatedAt:      time.Now().Format(time.RFC3339),
	}

	e.proposals[proposalID] = proposal

	// Log audit event
	if e.auditLogger != nil {
		e.auditLogger.Log(ctx, audit.Entry{
			CircleID:       req.IssuerCircleID,
			IntersectionID: intersectionID,
			EventType:      string(events.EventProposalSubmitted),
			SubjectID:      proposalID,
			Action:         "submit_proposal",
			Outcome:        "success",
			Metadata: map[string]string{
				"proposal_type": string(req.ProposalType),
				"reason":        req.Reason,
			},
		})
	}

	return proposalID, nil
}

// CounterProposal creates a counterproposal to an existing proposal.
func (e *Engine) CounterProposal(ctx context.Context, proposalID string, req negotiation.CounterProposalRequest) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	parent, exists := e.proposals[proposalID]
	if !exists {
		return "", fmt.Errorf("proposal not found: %s", proposalID)
	}

	if parent.State == negotiation.ProposalStateFinalized {
		return "", fmt.Errorf("cannot counter a finalized proposal")
	}

	// Verify issuer is a party
	isParty, err := e.intRuntime.IsParty(ctx, parent.IntersectionID, req.IssuerCircleID)
	if err != nil {
		return "", fmt.Errorf("failed to check party: %w", err)
	}
	if !isParty {
		return "", fmt.Errorf("issuer %s is not a party to intersection", req.IssuerCircleID)
	}

	// Mark parent as countered
	parent.State = negotiation.ProposalStateCountered

	// Get all parties for approval tracking
	parties, err := e.intRuntime.ListParties(ctx, parent.IntersectionID)
	if err != nil {
		return "", fmt.Errorf("failed to list parties: %w", err)
	}

	e.proposalCounter++
	counterID := fmt.Sprintf("prop-%d", e.proposalCounter)

	// Initialize approvals
	approvals := make(map[string]bool)
	for _, party := range parties {
		approvals[party.CircleID] = false
	}
	// Counter issuer implicitly approves
	approvals[req.IssuerCircleID] = true

	counter := &negotiation.ProposalThread{
		ID:             counterID,
		IntersectionID: parent.IntersectionID,
		IssuerCircleID: req.IssuerCircleID,
		ProposalType:   parent.ProposalType,
		State:          negotiation.ProposalStatePending,
		Reason:         req.Reason,
		ScopeAdditions: req.ScopeAdditions,
		ScopeRemovals:  req.ScopeRemovals,
		CeilingChanges: req.CeilingChanges,
		ActionSpec:     parent.ActionSpec,
		Approvals:      approvals,
		Rejections:     make(map[string]string),
		ParentID:       proposalID,
		CounterIDs:     []string{},
		CreatedAt:      time.Now().Format(time.RFC3339),
	}

	e.proposals[counterID] = counter
	parent.CounterIDs = append(parent.CounterIDs, counterID)
	parent.ActiveCounter = counterID

	// Log audit event
	if e.auditLogger != nil {
		e.auditLogger.Log(ctx, audit.Entry{
			CircleID:       req.IssuerCircleID,
			IntersectionID: parent.IntersectionID,
			EventType:      string(events.EventCounterproposalMade),
			SubjectID:      counterID,
			Action:         "counter_proposal",
			Outcome:        "success",
			Metadata: map[string]string{
				"parent_proposal": proposalID,
				"reason":          req.Reason,
			},
		})
	}

	return counterID, nil
}

// Accept records a party's acceptance of a proposal or counterproposal.
func (e *Engine) Accept(ctx context.Context, proposalID string, byCircleID string) (*negotiation.AcceptResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	proposal, exists := e.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal not found: %s", proposalID)
	}

	if proposal.State == negotiation.ProposalStateFinalized {
		return nil, fmt.Errorf("proposal already finalized")
	}
	if proposal.State == negotiation.ProposalStateRejected {
		return nil, fmt.Errorf("proposal was rejected")
	}
	if proposal.State == negotiation.ProposalStateSuperseded {
		return nil, fmt.Errorf("proposal was superseded by a counterproposal")
	}

	// Verify acceptor is a party
	if _, ok := proposal.Approvals[byCircleID]; !ok {
		return nil, fmt.Errorf("circle %s is not a party to this proposal", byCircleID)
	}

	// Record acceptance
	proposal.Approvals[byCircleID] = true

	// Check if all parties have accepted
	allAccepted := true
	var pendingParties []string
	for circleID, approved := range proposal.Approvals {
		if !approved {
			allAccepted = false
			pendingParties = append(pendingParties, circleID)
		}
	}

	if allAccepted {
		proposal.State = negotiation.ProposalStateAccepted
	}

	// Log audit event
	if e.auditLogger != nil {
		e.auditLogger.Log(ctx, audit.Entry{
			CircleID:       byCircleID,
			IntersectionID: proposal.IntersectionID,
			EventType:      string(events.EventProposalAccepted),
			SubjectID:      proposalID,
			Action:         "accept_proposal",
			Outcome:        "success",
			Metadata: map[string]string{
				"all_accepted": fmt.Sprintf("%v", allAccepted),
			},
		})
	}

	// Update trust on acceptance
	if e.trustStore != nil {
		e.trustStore.RecordAcceptance(ctx, proposal.IntersectionID, byCircleID)
	}

	return &negotiation.AcceptResult{
		ProposalID:     proposalID,
		AcceptorID:     byCircleID,
		AllAccepted:    allAccepted,
		PendingParties: pendingParties,
	}, nil
}

// Reject records a party's rejection of a proposal.
func (e *Engine) Reject(ctx context.Context, proposalID string, byCircleID string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	proposal, exists := e.proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal not found: %s", proposalID)
	}

	if proposal.State == negotiation.ProposalStateFinalized {
		return fmt.Errorf("proposal already finalized")
	}

	// Verify rejector is a party
	if _, ok := proposal.Approvals[byCircleID]; !ok {
		return fmt.Errorf("circle %s is not a party to this proposal", byCircleID)
	}

	proposal.Rejections[byCircleID] = reason
	proposal.State = negotiation.ProposalStateRejected

	// Log audit event
	if e.auditLogger != nil {
		e.auditLogger.Log(ctx, audit.Entry{
			CircleID:       byCircleID,
			IntersectionID: proposal.IntersectionID,
			EventType:      string(events.EventProposalRejected),
			SubjectID:      proposalID,
			Action:         "reject_proposal",
			Outcome:        "rejected",
			Metadata: map[string]string{
				"reason": reason,
			},
		})
	}

	// Update trust on rejection
	if e.trustStore != nil {
		e.trustStore.RecordRejection(ctx, proposal.IntersectionID, byCircleID)
	}

	return nil
}

// Finalize completes the negotiation after all parties accept.
func (e *Engine) Finalize(ctx context.Context, proposalID string) (*negotiation.FinalizeResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	proposal, exists := e.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal not found: %s", proposalID)
	}

	if proposal.State == negotiation.ProposalStateFinalized {
		return nil, fmt.Errorf("proposal already finalized")
	}

	// Verify all parties have accepted
	for circleID, approved := range proposal.Approvals {
		if !approved {
			return nil, fmt.Errorf("not all parties have accepted: %s has not accepted", circleID)
		}
	}

	result := &negotiation.FinalizeResult{
		ProposalID:     proposalID,
		IntersectionID: proposal.IntersectionID,
	}

	now := time.Now()

	if proposal.ProposalType == negotiation.ProposalTypeAmendment {
		// Apply amendment to intersection contract
		newVersion, err := e.applyAmendment(ctx, proposal)
		if err != nil {
			return nil, fmt.Errorf("failed to apply amendment: %w", err)
		}
		result.ResultType = "amendment"
		result.NewVersion = newVersion

		// Log contract amended event
		if e.auditLogger != nil {
			e.auditLogger.Log(ctx, audit.Entry{
				CircleID:       proposal.IssuerCircleID,
				IntersectionID: proposal.IntersectionID,
				EventType:      string(events.EventIntersectionAmended),
				SubjectID:      proposal.IntersectionID,
				Action:         "amend_contract",
				Outcome:        "success",
				Metadata: map[string]string{
					"new_version": newVersion,
					"proposal_id": proposalID,
				},
			})
		}
	} else if proposal.ProposalType == negotiation.ProposalTypeCommitment {
		// Create commitment
		commitmentID, err := e.formCommitment(ctx, proposal)
		if err != nil {
			return nil, fmt.Errorf("failed to form commitment: %w", err)
		}
		result.ResultType = "commitment"
		result.CommitmentID = commitmentID

		// Log commitment formed event
		if e.auditLogger != nil {
			e.auditLogger.Log(ctx, audit.Entry{
				CircleID:       proposal.IssuerCircleID,
				IntersectionID: proposal.IntersectionID,
				EventType:      string(events.EventCommitmentFormed),
				SubjectID:      commitmentID,
				Action:         "form_commitment",
				Outcome:        "success",
				Metadata: map[string]string{
					"proposal_id": proposalID,
				},
			})
		}
	}

	proposal.State = negotiation.ProposalStateFinalized
	proposal.FinalizedAt = now.Format(time.RFC3339)

	// Log negotiation finalized event
	if e.auditLogger != nil {
		e.auditLogger.Log(ctx, audit.Entry{
			CircleID:       proposal.IssuerCircleID,
			IntersectionID: proposal.IntersectionID,
			EventType:      string(events.EventNegotiationFinalized),
			SubjectID:      proposalID,
			Action:         "finalize_negotiation",
			Outcome:        "success",
			Metadata: map[string]string{
				"result_type": result.ResultType,
			},
		})
	}

	return result, nil
}

// applyAmendment applies an amendment to the intersection contract.
func (e *Engine) applyAmendment(ctx context.Context, proposal *negotiation.ProposalThread) (string, error) {
	// Get current contract
	contract, err := e.intRuntime.GetContract(ctx, proposal.IntersectionID)
	if err != nil {
		return "", fmt.Errorf("failed to get contract: %w", err)
	}

	// Build new contract with amendments
	newScopes := make([]intersection.Scope, len(contract.Scopes))
	copy(newScopes, contract.Scopes)

	// Apply scope removals
	for _, scopeName := range proposal.ScopeRemovals {
		for i, s := range newScopes {
			if s.Name == scopeName {
				newScopes = append(newScopes[:i], newScopes[i+1:]...)
				break
			}
		}
	}

	// Apply scope additions
	for _, scopeChange := range proposal.ScopeAdditions {
		newScopes = append(newScopes, intersection.Scope{
			Name:        scopeChange.Name,
			Description: scopeChange.Description,
			ReadWrite:   scopeChange.Permission,
		})
	}

	// Apply ceiling changes
	newCeilings := make([]intersection.Ceiling, len(contract.Ceilings))
	copy(newCeilings, contract.Ceilings)

	for _, ceilingChange := range proposal.CeilingChanges {
		found := false
		for i, c := range newCeilings {
			if c.Type == ceilingChange.Type {
				newCeilings[i].Value = ceilingChange.Value
				newCeilings[i].Unit = ceilingChange.Unit
				found = true
				break
			}
		}
		if !found {
			newCeilings = append(newCeilings, intersection.Ceiling{
				Type:  ceilingChange.Type,
				Value: ceilingChange.Value,
				Unit:  ceilingChange.Unit,
			})
		}
	}

	// Calculate new version (minor bump for additions, could be major for removals)
	newVersion := bumpMinorVersion(contract.Version)

	// Create amendment request
	amendReq := intersection.AmendRequest{
		IntersectionID: proposal.IntersectionID,
		ProposerID:     proposal.IssuerCircleID,
		NewContract: intersection.Contract{
			Parties:    contract.Parties,
			Scopes:     newScopes,
			Ceilings:   newCeilings,
			Governance: contract.Governance,
		},
		Reason: proposal.Reason,
	}

	_, err = e.intRuntime.Amend(ctx, amendReq)
	if err != nil {
		return "", fmt.Errorf("failed to amend intersection: %w", err)
	}

	return newVersion, nil
}

// formCommitment creates a commitment from an accepted proposal.
func (e *Engine) formCommitment(ctx context.Context, proposal *negotiation.ProposalThread) (string, error) {
	// Get parties for the commitment
	parties, err := e.intRuntime.ListParties(ctx, proposal.IntersectionID)
	if err != nil {
		return "", fmt.Errorf("failed to list parties: %w", err)
	}

	partyIDs := make([]string, len(parties))
	for i, p := range parties {
		partyIDs[i] = p.CircleID
	}

	// Generate commitment ID
	commitmentID := fmt.Sprintf("commit-%s", proposal.ID)

	// Note: In a real implementation, this would create and store a primitives.Commitment
	// For the demo, we just return the ID
	return commitmentID, nil
}

// GetProposal retrieves a proposal by ID.
func (e *Engine) GetProposal(ctx context.Context, proposalID string) (*negotiation.ProposalThread, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	proposal, exists := e.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal not found: %s", proposalID)
	}

	// Return a copy
	copy := *proposal
	return &copy, nil
}

// ListProposals lists all proposals for an intersection.
func (e *Engine) ListProposals(ctx context.Context, intersectionID string) ([]negotiation.ProposalThread, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []negotiation.ProposalThread
	for _, p := range e.proposals {
		if p.IntersectionID == intersectionID {
			result = append(result, *p)
		}
	}
	return result, nil
}

// bumpMinorVersion increments the minor version of a semver string.
func bumpMinorVersion(version string) string {
	var major, minor, patch int
	fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	return fmt.Sprintf("%d.%d.%d", major, minor+1, 0)
}

// Verify interface compliance at compile time.
var _ negotiation.NegotiationLoop = (*Engine)(nil)
