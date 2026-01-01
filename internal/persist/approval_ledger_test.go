package persist

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/approvalflow"
	"quantumlife/pkg/domain/approvaltoken"
	"quantumlife/pkg/domain/intersection"
	"quantumlife/pkg/domain/storelog"
)

func TestApprovalLedgerIntersectionPolicies(t *testing.T) {
	log := storelog.NewInMemoryLog()
	ledger, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create a policy
	policy := intersection.NewIntersectionPolicy("family-001", "Family", now)
	policy.AddMember("person-satish", intersection.RoleOwner, "Satish")
	policy.AddMember("person-wife", intersection.RoleSpouse, "Wife")
	policy.AddRequirement(intersection.ApprovalRequirement{
		ActionClass:   intersection.ActionCalendarRespond,
		RequiredRoles: []intersection.MemberRole{intersection.RoleOwner, intersection.RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 60,
	})

	if err := ledger.AddIntersectionPolicy(policy); err != nil {
		t.Fatalf("add policy: %v", err)
	}

	// Retrieve policy
	retrieved := ledger.GetIntersectionPolicy("family-001")
	if retrieved == nil {
		t.Fatal("policy not found")
	}
	if retrieved.Name != "Family" {
		t.Errorf("expected name Family, got %s", retrieved.Name)
	}
	if len(retrieved.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(retrieved.Members))
	}

	// List policies
	policies := ledger.ListIntersectionPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	// Find by person
	satishPolicies := ledger.FindPoliciesForPerson("person-satish")
	if len(satishPolicies) != 1 {
		t.Errorf("expected 1 policy for satish, got %d", len(satishPolicies))
	}

	t.Logf("Intersection policy hash: %s", ledger.PolicySetHash()[:16])
}

func TestApprovalLedgerApprovalStates(t *testing.T) {
	log := storelog.NewInMemoryLog()
	ledger, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create an approval state
	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-123",
		"family-001",
		intersection.ActionCalendarRespond,
		approvers,
		2,
		60,
		now,
	)

	if err := ledger.CreateApprovalState(state); err != nil {
		t.Fatalf("create state: %v", err)
	}

	// Retrieve state
	retrieved := ledger.GetApprovalState(state.StateID)
	if retrieved == nil {
		t.Fatal("state not found")
	}
	if retrieved.TargetID != "draft-123" {
		t.Errorf("expected target draft-123, got %s", retrieved.TargetID)
	}

	// Get by target
	byTarget := ledger.GetApprovalStateByTarget(approvalflow.TargetTypeDraft, "draft-123")
	if byTarget == nil {
		t.Fatal("state not found by target")
	}

	// Record an approval
	approval := approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(5 * time.Minute),
		TokenID:   "tok-1",
	}

	if err := ledger.RecordApproval(state.StateID, approval); err != nil {
		t.Fatalf("record approval: %v", err)
	}

	// Check status
	retrieved = ledger.GetApprovalState(state.StateID)
	status := retrieved.ComputeStatus(now.Add(10 * time.Minute))
	if status != approvalflow.StatusPending {
		t.Errorf("expected pending (need 2 approvals), got %s", status)
	}

	// Record second approval
	approval2 := approvalflow.ApprovalRecord{
		PersonID:  "person-wife",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(10 * time.Minute),
		TokenID:   "tok-2",
	}

	if err := ledger.RecordApproval(state.StateID, approval2); err != nil {
		t.Fatalf("record second approval: %v", err)
	}

	// Check status again
	retrieved = ledger.GetApprovalState(state.StateID)
	status = retrieved.ComputeStatus(now.Add(15 * time.Minute))
	if status != approvalflow.StatusApproved {
		t.Errorf("expected approved, got %s", status)
	}

	t.Logf("Approval state hash: %s", ledger.StateSetHash()[:16])
}

func TestApprovalLedgerTokens(t *testing.T) {
	log := storelog.NewInMemoryLog()
	ledger, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	// Create a token
	token := approvaltoken.NewToken("state-123", "person-satish", approvaltoken.ActionTypeApprove, now, expires)
	token.SetSignature("Ed25519", "key-001", []byte("testsig"))

	if err := ledger.CreateToken(token); err != nil {
		t.Fatalf("create token: %v", err)
	}

	// Retrieve token
	retrieved := ledger.GetToken(token.TokenID)
	if retrieved == nil {
		t.Fatal("token not found")
	}
	if retrieved.StateID != "state-123" {
		t.Errorf("expected state state-123, got %s", retrieved.StateID)
	}

	// Get by hash
	byHash := ledger.GetTokenByHash(token.Hash)
	if byHash == nil {
		t.Fatal("token not found by hash")
	}

	// List active tokens
	active := ledger.ListActiveTokens(now.Add(30 * time.Minute))
	if len(active) != 1 {
		t.Errorf("expected 1 active token, got %d", len(active))
	}

	// Revoke token
	if err := ledger.RevokeToken(token.TokenID, now.Add(20*time.Minute)); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	// Should be gone
	retrieved = ledger.GetToken(token.TokenID)
	if retrieved != nil {
		t.Error("revoked token should not be found")
	}

	t.Logf("Token set hash: %s", ledger.TokenSetHash()[:16])
}

func TestApprovalLedgerPendingForPerson(t *testing.T) {
	log := storelog.NewInMemoryLog()
	ledger, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create states with different approvers
	approvers1 := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	approvers2 := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
	}

	state1 := approvalflow.NewApprovalState(approvalflow.TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers1, 2, 60, now)
	state2 := approvalflow.NewApprovalState(approvalflow.TargetTypeDraft, "draft-2", "", intersection.ActionEmailSend, approvers2, 1, 60, now)

	ledger.CreateApprovalState(state1)
	ledger.CreateApprovalState(state2)

	// Satish has 2 pending
	pending := ledger.ListPendingForPerson("person-satish", now.Add(5*time.Minute))
	if len(pending) != 2 {
		t.Errorf("expected 2 pending for satish, got %d", len(pending))
	}

	// Wife has 1 pending
	pending = ledger.ListPendingForPerson("person-wife", now.Add(5*time.Minute))
	if len(pending) != 1 {
		t.Errorf("expected 1 pending for wife, got %d", len(pending))
	}

	// After satish approves state1
	ledger.RecordApproval(state1.StateID, approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(10 * time.Minute),
	})

	// Satish now has 1 pending (already approved state1, but state2 pending)
	pending = ledger.ListPendingForPerson("person-satish", now.Add(15*time.Minute))
	if len(pending) != 1 {
		t.Errorf("expected 1 pending for satish after approval, got %d", len(pending))
	}
}

func TestApprovalLedgerStats(t *testing.T) {
	log := storelog.NewInMemoryLog()
	ledger, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add a policy
	policy := intersection.NewIntersectionPolicy("family-001", "Family", now)
	ledger.AddIntersectionPolicy(policy)

	// Add an approval state
	approvers := []approvalflow.ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}
	state := approvalflow.NewApprovalState(approvalflow.TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	ledger.CreateApprovalState(state)

	// Add a token
	token := approvaltoken.NewToken("state-1", "person-1", approvaltoken.ActionTypeApprove, now, now.Add(60*time.Minute))
	ledger.CreateToken(token)

	stats := ledger.Stats(now.Add(5 * time.Minute))

	if stats.PolicyCount != 1 {
		t.Errorf("expected 1 policy, got %d", stats.PolicyCount)
	}
	if stats.StateCount != 1 {
		t.Errorf("expected 1 state, got %d", stats.StateCount)
	}
	if stats.PendingStateCount != 1 {
		t.Errorf("expected 1 pending state, got %d", stats.PendingStateCount)
	}
	if stats.TokenCount != 1 {
		t.Errorf("expected 1 token, got %d", stats.TokenCount)
	}
	if stats.ActiveTokenCount != 1 {
		t.Errorf("expected 1 active token, got %d", stats.ActiveTokenCount)
	}
}

func TestApprovalLedgerReplay(t *testing.T) {
	log := storelog.NewInMemoryLog()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// First ledger instance
	ledger1, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	// Add data
	policy := intersection.NewIntersectionPolicy("family-001", "Family", now)
	policy.AddMember("person-satish", intersection.RoleOwner, "Satish")
	ledger1.AddIntersectionPolicy(policy)

	approvers := []approvalflow.ApproverRef{{PersonID: "person-satish", Role: intersection.RoleOwner}}
	state := approvalflow.NewApprovalState(approvalflow.TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	ledger1.CreateApprovalState(state)

	ledger1.RecordApproval(state.StateID, approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(5 * time.Minute),
	})

	token := approvaltoken.NewToken("state-1", "person-1", approvaltoken.ActionTypeApprove, now, now.Add(60*time.Minute))
	ledger1.CreateToken(token)

	// Second ledger instance (replay from same log)
	ledger2, err := NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create second ledger: %v", err)
	}

	// Verify data was replayed
	if ledger2.GetIntersectionPolicy("family-001") == nil {
		t.Error("policy should be replayed")
	}

	replayedState := ledger2.GetApprovalState(state.StateID)
	if replayedState == nil {
		t.Fatal("state should be replayed")
	}

	status := replayedState.ComputeStatus(now.Add(10 * time.Minute))
	if status != approvalflow.StatusApproved {
		t.Errorf("replayed state should be approved, got %s", status)
	}

	if ledger2.GetToken(token.TokenID) == nil {
		t.Error("token should be replayed")
	}

	t.Logf("Replay verified: policies=%d, states=%d, tokens=%d",
		len(ledger2.ListIntersectionPolicies()),
		len(ledger2.ListApprovalStates()),
		len(ledger2.ListTokens()))
}

func TestApprovalLedgerDeterminism(t *testing.T) {
	log1 := storelog.NewInMemoryLog()
	log2 := storelog.NewInMemoryLog()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create two identical ledgers
	ledger1, _ := NewApprovalLedger(log1)
	ledger2, _ := NewApprovalLedger(log2)

	// Add same data
	policy := intersection.NewIntersectionPolicy("family-001", "Family", now)
	policy.AddMember("person-satish", intersection.RoleOwner, "Satish")

	ledger1.AddIntersectionPolicy(policy)
	ledger2.AddIntersectionPolicy(policy)

	if ledger1.PolicySetHash() != ledger2.PolicySetHash() {
		t.Errorf("policy hashes should match: %s != %s", ledger1.PolicySetHash(), ledger2.PolicySetHash())
	}

	t.Logf("Ledger determinism verified")
}
