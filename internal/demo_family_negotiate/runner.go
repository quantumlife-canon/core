// Package demo_family_negotiate provides the Family Negotiation demo for vertical slice v3.
//
// This demo showcases:
// - Proposal → Counterproposal → Acceptance → Finalization
// - Contract version bump after amendment
// - Commitment formation
// - Trust tracking across negotiation
// - Full audit trail
//
// CRITICAL: This is SUGGEST-ONLY mode. No external actions are executed.
package demo_family_negotiate

import (
	"context"
	"fmt"
	"time"

	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	"quantumlife/internal/intersection"
	intImpl "quantumlife/internal/intersection/impl_inmem"
	"quantumlife/internal/negotiation"
	negImpl "quantumlife/internal/negotiation/impl_inmem"
	cryptoImpl "quantumlife/pkg/crypto/impl_inmem"
	"quantumlife/pkg/primitives"
)

// DemoResult contains the output of the negotiate-commit demo.
type DemoResult struct {
	// Circles
	CircleAID string // "You"
	CircleBID string // "Spouse"

	// Initial intersection
	IntersectionID  string
	InitialContract ContractSummary

	// Negotiation flow
	ProposalID       string
	ProposalSummary  ProposalSummary
	CounterID        string
	CounterSummary   ProposalSummary
	AcceptanceResult string

	// Amended contract
	AmendedContract ContractSummary

	// Commitment
	CommitmentID      string
	CommitmentSummary CommitmentSummary

	// Trust updates
	TrustUpdates []TrustUpdateSummary

	// Audit
	AuditLog []AuditEntry

	// Status
	Success bool
	Error   error
}

// ContractSummary provides a summary of a contract version.
type ContractSummary struct {
	IntersectionID string
	Version        string
	Scopes         []string
	Ceilings       []CeilingSummary
	Governance     string
	PartyIDs       []string
}

// CeilingSummary provides a summary of a ceiling.
type CeilingSummary struct {
	Type  string
	Value string
	Unit  string
}

// ProposalSummary provides a summary of a proposal.
type ProposalSummary struct {
	ID             string
	IssuerID       string
	Type           string
	State          string
	Reason         string
	ScopeAdditions []string
	CeilingChanges []string
	Approvals      map[string]bool
}

// CommitmentSummary provides a summary of a commitment.
type CommitmentSummary struct {
	ID             string
	IntersectionID string
	ProposalID     string
	ActionType     string
	ActionDesc     string
	Parties        []string
	NotExecuted    bool // Always true in demo
}

// TrustUpdateSummary provides a summary of a trust update.
type TrustUpdateSummary struct {
	CircleID       string
	IntersectionID string
	OldLevel       string
	NewLevel       string
	Reason         string
}

// AuditEntry is a simplified audit entry for demo output.
type AuditEntry struct {
	ID             string
	EventType      string
	CircleID       string
	IntersectionID string
	Action         string
	Outcome        string
	TraceID        string
	Timestamp      time.Time
}

// Runner executes the family negotiation demo.
type Runner struct {
	circleRuntime *circleImpl.Runtime
	intRuntime    *intImpl.Runtime
	negEngine     *negImpl.Engine
	auditStore    *auditImpl.Store
	keyManager    *cryptoImpl.KeyManager
	inviteService *circleImpl.InviteService
	trustStore    *negImpl.TrustStore
}

// NewRunner creates a new demo runner with all components wired together.
func NewRunner() *Runner {
	// Create in-memory stores
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()
	trustStore := negImpl.NewTrustStore(auditStore)

	// Create invite service
	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	// Create negotiation engine
	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
		TrustStore:  trustStore,
	})

	return &Runner{
		circleRuntime: circleRuntime,
		intRuntime:    intRuntime,
		negEngine:     negEngine,
		auditStore:    auditStore,
		keyManager:    keyManager,
		inviteService: inviteService,
		trustStore:    trustStore,
	}
}

// Run executes the family negotiation demo.
func (r *Runner) Run(ctx context.Context) (*DemoResult, error) {
	result := &DemoResult{}

	// ===== PHASE 1: Create intersection (reuse v2 flow) =====

	// Step 1: Create Circle A ("You")
	circleA, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to create Circle A: %w", err)
		return result, result.Error
	}
	result.CircleAID = circleA.ID

	// Create key for Circle A
	_, err = r.keyManager.CreateKey(ctx, fmt.Sprintf("key-%s", circleA.ID), 24*time.Hour)
	if err != nil {
		result.Error = fmt.Errorf("failed to create key for Circle A: %w", err)
		return result, result.Error
	}

	// Step 2: Create Circle B ("Spouse")
	circleB, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to create Circle B: %w", err)
		return result, result.Error
	}
	result.CircleBID = circleB.ID

	// Create key for Circle B
	_, err = r.keyManager.CreateKey(ctx, fmt.Sprintf("key-%s", circleB.ID), 24*time.Hour)
	if err != nil {
		result.Error = fmt.Errorf("failed to create key for Circle B: %w", err)
		return result, result.Error
	}

	// Step 3: Create intersection via invite token
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{
				Name:        "calendar:read",
				Description: "Read calendar events",
				Permission:  "read",
			},
		},
		Ceilings: []primitives.IntersectionCeiling{
			{
				Type:  "time_window",
				Value: "17:00-20:00",
				Unit:  "hours",
			},
			{
				Type:  "duration",
				Value: "3",
				Unit:  "hours",
			},
		},
		Governance: primitives.IntersectionGovernance{
			AmendmentRequires: "all_parties",
			DissolutionPolicy: "any_party",
		},
	}

	token, err := r.inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Family Intersection",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to issue invite token: %w", err)
		return result, result.Error
	}

	intRef, err := r.inviteService.AcceptInviteToken(ctx, token, circleB.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to accept invite token: %w", err)
		return result, result.Error
	}
	result.IntersectionID = intRef.IntersectionID

	// Get initial contract summary
	contract, err := r.intRuntime.GetContract(ctx, intRef.IntersectionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get contract: %w", err)
		return result, result.Error
	}
	result.InitialContract = r.buildContractSummary(contract, circleA.ID, circleB.ID)

	// ===== PHASE 2: Negotiation Loop =====

	// Step 4: Circle A submits proposal to extend time window and add write scope
	proposalID, err := r.negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Extend family time and add calendar write access",
		ScopeAdditions: []negotiation.ScopeChange{
			{
				Name:        "calendar:write",
				Description: "Create/modify calendar events (NOT executed)",
				Permission:  "write",
			},
		},
		CeilingChanges: []negotiation.CeilingChange{
			{
				Type:  "time_window",
				Value: "16:00-22:00",
				Unit:  "hours",
			},
			{
				Type:  "duration",
				Value: "6",
				Unit:  "hours",
			},
		},
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to submit proposal: %w", err)
		return result, result.Error
	}
	result.ProposalID = proposalID

	proposal, _ := r.negEngine.GetProposal(ctx, proposalID)
	result.ProposalSummary = r.buildProposalSummary(proposal)

	// Step 5: Circle B rejects initially (to trigger trust update)
	err = r.negEngine.Reject(ctx, proposalID, circleB.ID, "Time window too wide")
	if err != nil {
		result.Error = fmt.Errorf("failed to reject proposal: %w", err)
		return result, result.Error
	}

	// Step 6: Circle A submits new proposal with narrower terms
	proposal2ID, err := r.negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Revised: Moderate extension with calendar write",
		ScopeAdditions: []negotiation.ScopeChange{
			{
				Name:        "calendar:write",
				Description: "Create/modify calendar events (NOT executed)",
				Permission:  "write",
			},
		},
		CeilingChanges: []negotiation.CeilingChange{
			{
				Type:  "time_window",
				Value: "17:00-21:00",
				Unit:  "hours",
			},
			{
				Type:  "duration",
				Value: "4",
				Unit:  "hours",
			},
		},
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to submit revised proposal: %w", err)
		return result, result.Error
	}

	// Step 7: Circle B counterproposals with even narrower time window
	counterID, err := r.negEngine.CounterProposal(ctx, proposal2ID, negotiation.CounterProposalRequest{
		IssuerCircleID: circleB.ID,
		Reason:         "Keep write access but reduce evening hours",
		ScopeAdditions: []negotiation.ScopeChange{
			{
				Name:        "calendar:write",
				Description: "Create/modify calendar events (NOT executed)",
				Permission:  "write",
			},
		},
		CeilingChanges: []negotiation.CeilingChange{
			{
				Type:  "time_window",
				Value: "18:00-21:00",
				Unit:  "hours",
			},
			{
				Type:  "duration",
				Value: "3",
				Unit:  "hours",
			},
		},
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to submit counterproposal: %w", err)
		return result, result.Error
	}
	result.CounterID = counterID

	counter, _ := r.negEngine.GetProposal(ctx, counterID)
	result.CounterSummary = r.buildProposalSummary(counter)

	// Step 8: Circle A accepts the counterproposal
	acceptResult, err := r.negEngine.Accept(ctx, counterID, circleA.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to accept counterproposal: %w", err)
		return result, result.Error
	}
	result.AcceptanceResult = fmt.Sprintf("All accepted: %v", acceptResult.AllAccepted)

	// Step 9: Finalize negotiation -> amendment applied
	finalResult, err := r.negEngine.Finalize(ctx, counterID)
	if err != nil {
		result.Error = fmt.Errorf("failed to finalize negotiation: %w", err)
		return result, result.Error
	}

	// Get amended contract
	amendedContract, err := r.intRuntime.GetContract(ctx, intRef.IntersectionID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get amended contract: %w", err)
		return result, result.Error
	}
	result.AmendedContract = r.buildContractSummary(amendedContract, circleA.ID, circleB.ID)

	// ===== PHASE 3: Commitment Formation =====

	// Step 10: Submit commitment proposal
	commitPropID, err := r.negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeCommitment,
		Reason:         "Suggest scheduling family activity within next 7 days",
		ActionSpec: &primitives.ActionSpec{
			Type:        "calendar_suggestion",
			Description: "Suggest family activity in agreed time window",
			Parameters: map[string]string{
				"within_days": "7",
				"activity":    "family_dinner_or_activity",
			},
			RequiredScopes: []string{"calendar:read"},
		},
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to submit commitment proposal: %w", err)
		return result, result.Error
	}

	// Circle B accepts
	_, err = r.negEngine.Accept(ctx, commitPropID, circleB.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to accept commitment proposal: %w", err)
		return result, result.Error
	}

	// Finalize commitment
	commitResult, err := r.negEngine.Finalize(ctx, commitPropID)
	if err != nil {
		result.Error = fmt.Errorf("failed to finalize commitment: %w", err)
		return result, result.Error
	}

	result.CommitmentID = commitResult.CommitmentID
	result.CommitmentSummary = CommitmentSummary{
		ID:             commitResult.CommitmentID,
		IntersectionID: intRef.IntersectionID,
		ProposalID:     commitPropID,
		ActionType:     "calendar_suggestion",
		ActionDesc:     "Suggest family activity in agreed time window",
		Parties:        []string{circleA.ID, circleB.ID},
		NotExecuted:    true,
	}

	// ===== PHASE 4: Collect results =====

	// Get trust updates
	for _, update := range r.trustStore.GetUpdates() {
		result.TrustUpdates = append(result.TrustUpdates, TrustUpdateSummary{
			CircleID:       update.CircleID,
			IntersectionID: update.IntersectionID,
			OldLevel:       update.OldLevel.String(),
			NewLevel:       update.NewLevel.String(),
			Reason:         update.Reason,
		})
	}

	// Collect audit log
	auditEntries := r.auditStore.GetAllEntries()
	for _, entry := range auditEntries {
		traceID := ""
		if entry.Metadata != nil {
			traceID = entry.Metadata["trace_id"]
		}
		result.AuditLog = append(result.AuditLog, AuditEntry{
			ID:             entry.ID,
			EventType:      entry.EventType,
			CircleID:       entry.CircleID,
			IntersectionID: entry.IntersectionID,
			Action:         entry.Action,
			Outcome:        entry.Outcome,
			TraceID:        traceID,
			Timestamp:      entry.Timestamp,
		})
	}

	// Verify contract history
	history, err := r.intRuntime.GetContractHistory(ctx, intRef.IntersectionID)
	if err == nil && len(history) >= 2 {
		// Contract history is preserved
		result.AmendedContract.Version = fmt.Sprintf("%s (history: %d versions)", finalResult.NewVersion, len(history))
	}

	result.Success = true
	return result, nil
}

// buildContractSummary builds a contract summary for display.
func (r *Runner) buildContractSummary(contract *intersection.Contract, partyA, partyB string) ContractSummary {
	var scopes []string
	for _, s := range contract.Scopes {
		scopes = append(scopes, s.Name)
	}

	var ceilings []CeilingSummary
	for _, c := range contract.Ceilings {
		ceilings = append(ceilings, CeilingSummary{
			Type:  c.Type,
			Value: c.Value,
			Unit:  c.Unit,
		})
	}

	return ContractSummary{
		IntersectionID: contract.IntersectionID,
		Version:        contract.Version,
		Scopes:         scopes,
		Ceilings:       ceilings,
		Governance:     fmt.Sprintf("amendment=%s, dissolution=%s", contract.Governance.AmendmentRequires, contract.Governance.DissolutionPolicy),
		PartyIDs:       []string{partyA, partyB},
	}
}

// buildProposalSummary builds a proposal summary for display.
func (r *Runner) buildProposalSummary(proposal *negotiation.ProposalThread) ProposalSummary {
	var scopeAdds []string
	for _, s := range proposal.ScopeAdditions {
		scopeAdds = append(scopeAdds, s.Name)
	}

	var ceilingChanges []string
	for _, c := range proposal.CeilingChanges {
		ceilingChanges = append(ceilingChanges, fmt.Sprintf("%s=%s %s", c.Type, c.Value, c.Unit))
	}

	return ProposalSummary{
		ID:             proposal.ID,
		IssuerID:       proposal.IssuerCircleID,
		Type:           string(proposal.ProposalType),
		State:          string(proposal.State),
		Reason:         proposal.Reason,
		ScopeAdditions: scopeAdds,
		CeilingChanges: ceilingChanges,
		Approvals:      proposal.Approvals,
	}
}
