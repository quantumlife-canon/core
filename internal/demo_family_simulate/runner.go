// Package demo_family_simulate provides the Vertical Slice v4 demo.
// This demo shows the full simulation pipeline:
// Commitment -> Action -> (Simulated) Execution -> Settlement -> Memory Update -> Audit
//
// CRITICAL: This uses ModeSimulate - no external side effects occur.
package demo_family_simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/internal/authority"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	calendarMock "quantumlife/internal/connectors/calendar/impl_mock"
	"quantumlife/internal/execution"
	executionImpl "quantumlife/internal/execution/impl_inmem"
	intImpl "quantumlife/internal/intersection/impl_inmem"
	"quantumlife/internal/memory"
	memoryImpl "quantumlife/internal/memory/impl_inmem"
	"quantumlife/internal/negotiation"
	negImpl "quantumlife/internal/negotiation/impl_inmem"
	cryptoImpl "quantumlife/pkg/crypto/impl_inmem"
	"quantumlife/pkg/events"
	"quantumlife/pkg/primitives"
)

// Result contains the demo output.
type Result struct {
	// Circles
	CircleA string
	CircleB string

	// Intersection
	IntersectionID   string
	ContractVersion  string
	ContractScopes   []string
	ContractCeilings []CeilingSummary

	// Commitment
	CommitmentID      string
	CommitmentSummary string

	// Action
	ActionID      string
	ActionType    string
	ActionSummary string

	// Authorization
	AuthorizationProof *authority.AuthorizationProof

	// Execution Outcome
	ExecutionOutcome *execution.ExecutionOutcome

	// Settlement
	SettlementID      string
	SettlementStatus  string
	SettlementSummary string

	// Memory
	MemoryEntry   *memory.MemoryEntry
	MemorySummary string

	// Audit
	AuditEntries []AuditEntry

	// Status
	Success bool
	Error   string
	Mode    string
}

// CeilingSummary provides a summary of a ceiling.
type CeilingSummary struct {
	Type  string
	Value string
	Unit  string
}

// AuditEntry is a simplified audit entry for display.
type AuditEntry struct {
	ID                   string
	Type                 string
	CircleID             string
	IntersectionID       string
	SubjectID            string
	AuthorizationProofID string
}

// Runner executes the v4 demo.
type Runner struct {
	mode          primitives.RunMode
	clockFunc     func() time.Time
	circleRuntime *circleImpl.Runtime
	intRuntime    *intImpl.Runtime
	negEngine     *negImpl.Engine
	auditStore    *auditImpl.Store
	memoryStore   *memoryImpl.Store
	keyManager    *cryptoImpl.KeyManager
	inviteService *circleImpl.InviteService
	trustStore    *negImpl.TrustStore
	calendarConn  *calendarMock.MockConnector
	authEngine    *authorityImpl.Engine
	simulator     *executionImpl.Simulator
	traceID       string
}

// NewRunner creates a new demo runner with default mode (simulate).
func NewRunner() *Runner {
	return NewRunnerWithMode(primitives.ModeSimulate)
}

// NewRunnerWithMode creates a demo runner with a specific mode.
func NewRunnerWithMode(mode primitives.RunMode) *Runner {
	clockFunc := func() time.Time { return time.Date(2025, 1, 15, 18, 30, 0, 0, time.UTC) }

	// Create in-memory stores
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()
	trustStore := negImpl.NewTrustStore(auditStore)
	memoryStore := memoryImpl.NewStore()
	calendarConn := calendarMock.NewMockConnectorWithClock(clockFunc)

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

	// Create authority engine (using intRuntime which implements intersection.Runtime)
	authEngine := authorityImpl.NewEngineWithClock(intRuntime, clockFunc)

	// Create execution simulator
	simulator := executionImpl.NewSimulatorWithClock(calendarConn, clockFunc)

	return &Runner{
		mode:          mode,
		clockFunc:     clockFunc,
		circleRuntime: circleRuntime,
		intRuntime:    intRuntime,
		negEngine:     negEngine,
		auditStore:    auditStore,
		memoryStore:   memoryStore,
		keyManager:    keyManager,
		inviteService: inviteService,
		trustStore:    trustStore,
		calendarConn:  calendarConn,
		authEngine:    authEngine,
		simulator:     simulator,
		traceID:       "trace-v4-demo-1",
	}
}

// NewRunnerWithClock creates a demo runner with an injected clock.
func NewRunnerWithClock(mode primitives.RunMode, clockFunc func() time.Time) *Runner {
	runner := NewRunnerWithMode(mode)
	runner.clockFunc = clockFunc
	return runner
}

// Run executes the demo.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	result := &Result{
		Mode: string(r.mode),
	}

	// Validate run mode
	if err := primitives.ValidateRunMode(r.mode); err != nil {
		result.Error = err.Error()
		return result, err
	}

	// ===== PHASE 1: Create intersection (reuse v2/v3 flow) =====

	// Step 1: Create Circle A ("You")
	circleA, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create Circle A: %v", err)
		return result, err
	}
	result.CircleA = circleA.ID

	// Create key for Circle A
	_, err = r.keyManager.CreateKey(ctx, fmt.Sprintf("key-%s", circleA.ID), 24*time.Hour)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create key for Circle A: %v", err)
		return result, err
	}

	// Step 2: Create Circle B ("Spouse")
	circleB, err := r.circleRuntime.Create(ctx, circle.CreateRequest{
		TenantID: "demo-tenant",
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create Circle B: %v", err)
		return result, err
	}
	result.CircleB = circleB.ID

	// Create key for Circle B
	_, err = r.keyManager.CreateKey(ctx, fmt.Sprintf("key-%s", circleB.ID), 24*time.Hour)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create key for Circle B: %v", err)
		return result, err
	}

	// Step 3: Create intersection via invite token with scopes for simulation
	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{
			{
				Name:        "calendar:read",
				Description: "Read calendar events",
				Permission:  "read",
			},
			{
				Name:        "calendar:write",
				Description: "Create/modify calendar events",
				Permission:  "write",
			},
		},
		Ceilings: []primitives.IntersectionCeiling{
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
		result.Error = fmt.Sprintf("failed to issue invite token: %v", err)
		return result, err
	}

	intRef, err := r.inviteService.AcceptInviteToken(ctx, token, circleB.ID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to accept invite token: %v", err)
		return result, err
	}
	result.IntersectionID = intRef.IntersectionID

	// Get contract details
	contract, err := r.intRuntime.GetContract(ctx, intRef.IntersectionID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get contract: %v", err)
		return result, err
	}
	result.ContractVersion = contract.Version
	for _, s := range contract.Scopes {
		result.ContractScopes = append(result.ContractScopes, s.Name)
	}
	for _, c := range contract.Ceilings {
		result.ContractCeilings = append(result.ContractCeilings, CeilingSummary{
			Type:  c.Type,
			Value: c.Value,
			Unit:  c.Unit,
		})
	}

	// ===== PHASE 2: Form commitment via negotiation =====

	// Submit commitment proposal
	proposalID, err := r.negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeCommitment,
		Reason:         "Propose family activity in agreed time window",
		ActionSpec: &primitives.ActionSpec{
			Type:        "propose_family_activity",
			Description: "Suggest family activity in agreed time window",
			Parameters: map[string]string{
				"title":       "Family Game Night",
				"description": "Board games with the kids",
				"time_window": "18:00-19:00",
				"duration":    "1",
			},
			RequiredScopes: []string{"calendar:write"},
		},
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to submit proposal: %v", err)
		return result, err
	}

	// Circle B accepts
	_, err = r.negEngine.Accept(ctx, proposalID, circleB.ID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to accept proposal: %v", err)
		return result, err
	}

	// Finalize to form commitment
	finalizeResult, err := r.negEngine.Finalize(ctx, proposalID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to finalize: %v", err)
		return result, err
	}

	result.CommitmentID = finalizeResult.CommitmentID
	result.CommitmentSummary = "Commitment to propose family activity in agreed time window"

	// ===== PHASE 3: Mode-specific execution =====

	if r.mode == primitives.ModeSuggestOnly {
		result.Success = true
		result.ActionSummary = "SUGGEST_ONLY: No action created"
		result.SettlementSummary = "SUGGEST_ONLY: No settlement recorded"
		result.MemorySummary = "SUGGEST_ONLY: No memory written"
		result.AuditEntries = r.getAuditEntries()
		return result, nil
	}

	// ===== PHASE 4: Create Action from Commitment (ModeSimulate) =====

	// Get the proposal to get action spec
	proposal, _ := r.negEngine.GetProposal(ctx, proposalID)

	commitment := &primitives.Commitment{
		ID:             finalizeResult.CommitmentID,
		Issuer:         circleA.ID,
		IntersectionID: intRef.IntersectionID,
	}
	if proposal.ActionSpec != nil {
		commitment.ActionSpec = *proposal.ActionSpec
	}

	action, err := r.simulator.CreateActionFromCommitment(ctx, commitment)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create action: %v", err)
		return result, err
	}

	result.ActionID = action.ID
	result.ActionType = action.Type
	result.ActionSummary = fmt.Sprintf("Action %s created from commitment", action.ID)

	// Log action created
	r.auditStore.Append(ctx, auditImpl.Entry{
		Type:           string(events.EventActionCreated),
		CircleID:       circleA.ID,
		IntersectionID: intRef.IntersectionID,
		Action:         "create_action",
		Outcome:        "success",
		TraceID:        r.traceID,
	})

	// ===== PHASE 5: Authorization check =====

	requiredScopes := []string{"calendar:write"}
	authProof, err := r.authEngine.AuthorizeAction(ctx, action, requiredScopes, r.mode, r.traceID)
	if err != nil {
		result.Error = fmt.Sprintf("authorization check failed: %v", err)
		return result, err
	}

	result.AuthorizationProof = authProof

	// Log authorization checked
	r.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventAuthorizationChecked),
		CircleID:             circleA.ID,
		IntersectionID:       intRef.IntersectionID,
		Action:               "authorize_action",
		Outcome:              fmt.Sprintf("authorized=%t", authProof.Authorized),
		TraceID:              r.traceID,
		AuthorizationProofID: authProof.ID,
	})

	if !authProof.Authorized {
		result.Error = fmt.Sprintf("Authorization denied: %s", authProof.DenialReason)
		result.AuditEntries = r.getAuditEntries()
		return result, fmt.Errorf("authorization denied: %s", authProof.DenialReason)
	}

	// ===== PHASE 6: Simulated execution =====

	r.simulator.SetAuthProofLookup(func(actionID string) string {
		return authProof.ID
	})

	execOutcome, err := r.simulator.SimulateExecution(ctx, action)
	if err != nil {
		result.Error = fmt.Sprintf("simulation failed: %v", err)
		return result, err
	}

	result.ExecutionOutcome = execOutcome

	// Log simulated execution completed
	r.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventSimulatedExecutionCompleted),
		CircleID:             circleA.ID,
		IntersectionID:       intRef.IntersectionID,
		Action:               "simulate_execution",
		Outcome:              fmt.Sprintf("success=%t, simulated=%t", execOutcome.Success, execOutcome.Simulated),
		TraceID:              r.traceID,
		AuthorizationProofID: authProof.ID,
	})

	// ===== PHASE 7: Record settlement =====

	now := r.clockFunc()
	settlement := &primitives.SimulatedSettlement{
		Settlement: primitives.Settlement{
			ID:             fmt.Sprintf("settlement-%s", action.ID),
			Version:        1,
			CreatedAt:      now,
			Issuer:         circleA.ID,
			ActionID:       action.ID,
			CommitmentID:   commitment.ID,
			IntersectionID: intRef.IntersectionID,
			Outcome: primitives.Outcome{
				Success:    execOutcome.Success,
				ResultCode: execOutcome.ResultCode,
				ResultData: execOutcome.ResultData,
			},
			State: string(primitives.SettlementStatusSimulated),
		},
		Status:               primitives.SettlementStatusSimulated,
		SimulatedAt:          now,
		AuthorizationProofID: authProof.ID,
		ProposedPayload:      execOutcome.ProposedPayload,
		Message:              "SIMULATED: No external write performed",
	}

	result.SettlementID = settlement.ID
	result.SettlementStatus = string(settlement.Status)
	result.SettlementSummary = fmt.Sprintf("Settlement %s recorded as SIMULATED", settlement.ID)

	// Log settlement recorded
	r.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventSettlementRecorded),
		CircleID:             circleA.ID,
		IntersectionID:       intRef.IntersectionID,
		Action:               "record_settlement",
		Outcome:              string(settlement.Status),
		TraceID:              r.traceID,
		AuthorizationProofID: authProof.ID,
	})

	// ===== PHASE 8: Memory update =====

	memoryValue, _ := json.Marshal(map[string]interface{}{
		"action_id":      action.ID,
		"action_type":    action.Type,
		"outcome":        execOutcome.ResultCode,
		"simulated":      execOutcome.Simulated,
		"settlement_id":  settlement.ID,
		"proposed_event": execOutcome.ProposedPayload,
		"timestamp":      now.Format(time.RFC3339),
	})

	memEntry, err := r.memoryStore.WriteVersioned(ctx, "intersection", intRef.IntersectionID, "last_simulated_action", memoryValue)
	if err != nil {
		result.Error = fmt.Sprintf("memory write failed: %v", err)
		return result, err
	}

	result.MemoryEntry = memEntry
	result.MemorySummary = fmt.Sprintf("Memory entry %s written (version %d)", memEntry.ID, memEntry.Version)

	// Log memory written
	r.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventMemoryWritten),
		CircleID:             circleA.ID,
		IntersectionID:       intRef.IntersectionID,
		Action:               "write_memory",
		Outcome:              fmt.Sprintf("key=last_simulated_action, version=%d", memEntry.Version),
		TraceID:              r.traceID,
		AuthorizationProofID: authProof.ID,
	})

	// Get audit entries
	result.AuditEntries = r.getAuditEntries()
	result.Success = true

	return result, nil
}

// getAuditEntries extracts audit entries from the store.
func (r *Runner) getAuditEntries() []AuditEntry {
	ctx := context.Background()
	entries, _ := r.auditStore.ListAll(ctx)

	var result []AuditEntry
	for _, e := range entries {
		result = append(result, AuditEntry{
			ID:                   e.ID,
			Type:                 e.EventType,
			CircleID:             e.CircleID,
			IntersectionID:       e.IntersectionID,
			SubjectID:            e.Action,
			AuthorizationProofID: e.AuthorizationProofID,
		})
	}
	return result
}
