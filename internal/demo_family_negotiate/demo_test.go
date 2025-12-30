// Package demo_family_negotiate provides tests for the Negotiation demo (Vertical Slice v3).
package demo_family_negotiate

import (
	"context"
	"strings"
	"testing"
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

// TestNegotiationDemoFullFlow runs the complete negotiation demo.
func TestNegotiationDemoFullFlow(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Demo was not successful: %v", result.Error)
	}

	// Verify circles were created
	if result.CircleAID == "" {
		t.Error("Expected CircleAID to be set")
	}
	if result.CircleBID == "" {
		t.Error("Expected CircleBID to be set")
	}

	// Verify intersection was created
	if result.IntersectionID == "" {
		t.Error("Expected IntersectionID to be set")
	}

	// Verify commitment was formed
	if result.CommitmentID == "" {
		t.Error("Expected CommitmentID to be set")
	}
}

// TestNegotiationRequiresAllParties ensures all parties must accept.
func TestNegotiationRequiresAllParties(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()
	trustStore := negImpl.NewTrustStore(auditStore)

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
		TrustStore:  trustStore,
	})

	// Create circles and intersection
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{{Name: "test:read", Permission: "read"}},
		Governance: primitives.IntersectionGovernance{
			AmendmentRequires: "all_parties",
			DissolutionPolicy: "any_party",
		},
	}

	token, _ := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Test",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	intRef, _ := inviteService.AcceptInviteToken(ctx, token, circleB.ID)

	// Submit proposal (Circle A implicitly approves)
	proposalID, err := negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Test amendment",
		CeilingChanges: []negotiation.CeilingChange{{Type: "test", Value: "100", Unit: "units"}},
	})
	if err != nil {
		t.Fatalf("Failed to submit proposal: %v", err)
	}

	// Try to finalize without Circle B accepting - should fail
	_, err = negEngine.Finalize(ctx, proposalID)
	if err == nil {
		t.Error("Expected finalize to fail without all parties accepting")
	}
	if !strings.Contains(err.Error(), "not all parties") {
		t.Errorf("Expected 'not all parties' error, got: %v", err)
	}

	// Now Circle B accepts
	_, err = negEngine.Accept(ctx, proposalID, circleB.ID)
	if err != nil {
		t.Fatalf("Failed to accept: %v", err)
	}

	// Now finalize should succeed
	result, err := negEngine.Finalize(ctx, proposalID)
	if err != nil {
		t.Fatalf("Finalize should succeed after all parties accept: %v", err)
	}
	if result.ResultType != "amendment" {
		t.Errorf("Expected amendment result, got %s", result.ResultType)
	}
}

// TestCounterproposalSupersedesParent ensures counterproposal supersedes parent.
func TestCounterproposalSupersedesParent(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
	})

	// Create circles and intersection
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{{Name: "test:read", Permission: "read"}},
	}

	token, _ := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Test",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	intRef, _ := inviteService.AcceptInviteToken(ctx, token, circleB.ID)

	// Submit proposal
	proposalID, _ := negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Original proposal",
	})

	// Counter propose
	counterID, err := negEngine.CounterProposal(ctx, proposalID, negotiation.CounterProposalRequest{
		IssuerCircleID: circleB.ID,
		Reason:         "Modified terms",
	})
	if err != nil {
		t.Fatalf("Failed to counter: %v", err)
	}

	// Original proposal should be in 'countered' state
	original, _ := negEngine.GetProposal(ctx, proposalID)
	if original.State != negotiation.ProposalStateCountered {
		t.Errorf("Expected parent to be countered, got %s", original.State)
	}

	// Counter should have parent reference
	counter, _ := negEngine.GetProposal(ctx, counterID)
	if counter.ParentID != proposalID {
		t.Errorf("Counter should reference parent, got %s", counter.ParentID)
	}
}

// TestFinalizeWithoutApprovalsFailsfunction.
func TestFinalizeWithoutApprovalsFails(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
	})

	// Create circles and intersection
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{{Name: "test:read", Permission: "read"}},
	}

	token, _ := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Test",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	intRef, _ := inviteService.AcceptInviteToken(ctx, token, circleB.ID)

	// Submit proposal
	proposalID, _ := negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Test",
	})

	// Try to finalize immediately - should fail
	_, err := negEngine.Finalize(ctx, proposalID)
	if err == nil {
		t.Error("Expected finalize to fail without approvals")
	}
}

// TestContractVersionBumpsTo110 ensures version bumps correctly.
func TestContractVersionBumpsTo110(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
	})

	// Create circles and intersection
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{{Name: "test:read", Permission: "read"}},
	}

	token, _ := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Test",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	intRef, _ := inviteService.AcceptInviteToken(ctx, token, circleB.ID)

	// Verify initial version is 1.0.0
	contract, _ := intRuntime.GetContract(ctx, intRef.IntersectionID)
	if contract.Version != "1.0.0" {
		t.Errorf("Expected initial version 1.0.0, got %s", contract.Version)
	}

	// Submit and finalize amendment
	proposalID, _ := negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Add scope",
		ScopeAdditions: []negotiation.ScopeChange{{Name: "test:write", Permission: "write"}},
	})

	negEngine.Accept(ctx, proposalID, circleB.ID)
	result, err := negEngine.Finalize(ctx, proposalID)
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// Version should be 1.1.0
	if result.NewVersion != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", result.NewVersion)
	}

	// Verify contract reflects new version
	amendedContract, _ := intRuntime.GetContract(ctx, intRef.IntersectionID)
	if amendedContract.Version != "1.1.0" {
		t.Errorf("Contract version should be 1.1.0, got %s", amendedContract.Version)
	}
}

// TestContractHistoryPreserved ensures contract history is maintained.
func TestContractHistoryPreserved(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
	})

	// Create circles and intersection
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{{Name: "test:read", Permission: "read"}},
	}

	token, _ := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Test",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	intRef, _ := inviteService.AcceptInviteToken(ctx, token, circleB.ID)

	// Initial history should have 1 version
	history, _ := intRuntime.GetContractHistory(ctx, intRef.IntersectionID)
	if len(history) != 1 {
		t.Errorf("Expected 1 version in history, got %d", len(history))
	}

	// Submit and finalize amendment
	proposalID, _ := negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeAmendment,
		Reason:         "Amendment",
	})
	negEngine.Accept(ctx, proposalID, circleB.ID)
	negEngine.Finalize(ctx, proposalID)

	// History should now have 2 versions
	history, _ = intRuntime.GetContractHistory(ctx, intRef.IntersectionID)
	if len(history) != 2 {
		t.Errorf("Expected 2 versions in history, got %d", len(history))
	}

	// Verify versions are in order
	if history[0].Version != "1.0.0" {
		t.Errorf("First version should be 1.0.0, got %s", history[0].Version)
	}
	if history[1].Version != "1.1.0" {
		t.Errorf("Second version should be 1.1.0, got %s", history[1].Version)
	}
}

// TestCommitmentRequiresFinalizedNegotiation ensures commitment needs finalized negotiation.
func TestCommitmentRequiresFinalizedNegotiation(t *testing.T) {
	ctx := context.Background()

	// Create components
	auditStore := auditImpl.NewStore()
	circleRuntime := circleImpl.NewRuntime()
	intRuntime := intImpl.NewRuntime()
	keyManager := cryptoImpl.NewKeyManager()

	inviteService := circleImpl.NewInviteService(circleImpl.InviteServiceConfig{
		CircleRuntime: circleRuntime,
		IntRuntime:    intRuntime,
		KeyManager:    keyManager,
		AuditLogger:   auditStore,
	})

	negEngine := negImpl.NewEngine(negImpl.EngineConfig{
		IntRuntime:  intRuntime,
		AuditLogger: auditStore,
	})

	// Create circles and intersection
	circleA, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	circleB, _ := circleRuntime.Create(ctx, circle.CreateRequest{TenantID: "test"})
	keyManager.CreateKey(ctx, "key-"+circleA.ID, 24*time.Hour)
	keyManager.CreateKey(ctx, "key-"+circleB.ID, 24*time.Hour)

	template := primitives.IntersectionTemplate{
		Scopes: []primitives.IntersectionScope{{Name: "test:read", Permission: "read"}},
	}

	token, _ := inviteService.IssueInviteToken(ctx, circle.IssueInviteRequest{
		IssuerCircleID: circleA.ID,
		TargetCircleID: circleB.ID,
		ProposedName:   "Test",
		Template:       template,
		ValidFor:       1 * time.Hour,
	})
	intRef, _ := inviteService.AcceptInviteToken(ctx, token, circleB.ID)

	// Submit commitment proposal
	proposalID, _ := negEngine.SubmitProposal(ctx, intRef.IntersectionID, negotiation.SubmitProposalRequest{
		IssuerCircleID: circleA.ID,
		ProposalType:   negotiation.ProposalTypeCommitment,
		Reason:         "Form commitment",
		ActionSpec: &primitives.ActionSpec{
			Type:        "test_action",
			Description: "Test",
		},
	})

	// Cannot finalize without all acceptances
	_, err := negEngine.Finalize(ctx, proposalID)
	if err == nil {
		t.Error("Should not finalize commitment without all acceptances")
	}

	// Accept and finalize
	negEngine.Accept(ctx, proposalID, circleB.ID)
	result, err := negEngine.Finalize(ctx, proposalID)
	if err != nil {
		t.Fatalf("Finalize should succeed: %v", err)
	}

	if result.ResultType != "commitment" {
		t.Errorf("Expected commitment result, got %s", result.ResultType)
	}
	if result.CommitmentID == "" {
		t.Error("Expected commitment ID to be set")
	}
}

// TestAuditContainsExpectedEvents ensures audit log has expected events.
func TestAuditContainsExpectedEvents(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Verify all expected events exist
	eventTypes := make(map[string]int)
	for _, entry := range result.AuditLog {
		eventTypes[entry.EventType]++
	}

	for _, event := range []string{
		"invite.token.issued",
		"invite.token.accepted",
		"intersection.created",
		"proposal.submitted",
		"proposal.rejected",
		"proposal.counterproposal",
		"proposal.accepted",
		"intersection.amended",
		"negotiation.finalized",
		"commitment.formed",
		"trust.updated",
	} {
		if eventTypes[event] == 0 {
			t.Errorf("Expected event %s not found in audit log", event)
		}
	}
}

// TestExecutionLayerNeverInvoked ensures no execution happens.
func TestExecutionLayerNeverInvoked(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Check audit for any execution events
	for _, entry := range result.AuditLog {
		if strings.Contains(entry.Action, "execute_external") {
			t.Errorf("Found external execution in audit: %s", entry.Action)
		}
		if entry.EventType == "action.executing" {
			t.Errorf("Found action execution event: %s", entry.EventType)
		}
		if entry.EventType == "action.completed" {
			t.Errorf("Found action completed event: %s", entry.EventType)
		}
	}

	// Commitment should be marked as not executed
	if !result.CommitmentSummary.NotExecuted {
		t.Error("Commitment should be marked as not executed")
	}
}

// TestDeterministicOutput ensures outputs are deterministic.
func TestDeterministicOutput(t *testing.T) {
	const runs = 3
	var results []*DemoResult

	for i := 0; i < runs; i++ {
		runner := NewRunner()
		result, err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		results = append(results, result)
	}

	// Compare key outputs
	for i := 1; i < len(results); i++ {
		if results[i].InitialContract.Version != results[0].InitialContract.Version {
			t.Errorf("Initial contract version differs")
		}
		if results[i].AmendedContract.Version != results[0].AmendedContract.Version {
			t.Errorf("Amended contract version differs")
		}
		if len(results[i].AuditLog) != len(results[0].AuditLog) {
			t.Errorf("Audit log length differs: %d vs %d", len(results[i].AuditLog), len(results[0].AuditLog))
		}
	}
}

// TestTrustUpdateOnRejection ensures trust degrades on rejection.
func TestTrustUpdateOnRejection(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Should have trust updates
	if len(result.TrustUpdates) == 0 {
		t.Error("Expected trust updates in result")
	}

	// Find rejection-triggered update
	foundRejection := false
	for _, update := range result.TrustUpdates {
		if update.Reason == "rejection" {
			foundRejection = true
			if update.NewLevel != "low" {
				t.Errorf("Expected trust to degrade to low on rejection, got %s", update.NewLevel)
			}
		}
	}
	if !foundRejection {
		t.Error("Expected trust update for rejection")
	}
}

// TestSemVerParsing tests semantic version parsing.
func TestSemVerParsing(t *testing.T) {
	tests := []struct {
		version string
		major   int
		minor   int
		patch   int
	}{
		{"1.0.0", 1, 0, 0},
		{"1.1.0", 1, 1, 0},
		{"2.3.4", 2, 3, 4},
	}

	for _, tt := range tests {
		v, err := intersection.ParseSemVer(tt.version)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", tt.version, err)
			continue
		}
		if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
			t.Errorf("ParseSemVer(%s) = %d.%d.%d, want %d.%d.%d",
				tt.version, v.Major, v.Minor, v.Patch, tt.major, tt.minor, tt.patch)
		}
	}
}

// TestSemVerBump tests version bumping.
func TestSemVerBump(t *testing.T) {
	v := intersection.SemVer{Major: 1, Minor: 0, Patch: 0}

	minor := v.BumpMinor()
	if minor.String() != "1.1.0" {
		t.Errorf("BumpMinor should give 1.1.0, got %s", minor.String())
	}

	major := v.BumpMajor()
	if major.String() != "2.0.0" {
		t.Errorf("BumpMajor should give 2.0.0, got %s", major.String())
	}

	patch := v.BumpPatch()
	if patch.String() != "1.0.1" {
		t.Errorf("BumpPatch should give 1.0.1, got %s", patch.String())
	}
}
