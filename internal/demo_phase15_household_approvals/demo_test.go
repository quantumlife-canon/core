// Package demo_phase15_household_approvals demonstrates Phase 15 features.
//
// Phase 15: Household Approvals + Intersections (Deterministic, Web-first)
// - Intersection policy model for household multi-party approvals
// - Approval flow tracking with deterministic status computation
// - Signed approval tokens for link-based approvals
// - Persistent approval ledger with replay
//
// Reference: docs/ADR/ADR-0031-phase15-household-approvals.md
package demo_phase15_household_approvals

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/crypto"
	"quantumlife/pkg/domain/approvalflow"
	"quantumlife/pkg/domain/approvaltoken"
	"quantumlife/pkg/domain/intersection"
	"quantumlife/pkg/domain/storelog"
)

// TestIntersectionPolicyDeterminism demonstrates deterministic intersection hashing.
func TestIntersectionPolicyDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create two identical intersection policies
	p1 := intersection.NewIntersectionPolicy("family-001", "Family Intersection", now)
	p1.AddMember("person-satish", intersection.RoleOwner, "Satish")
	p1.AddMember("person-wife", intersection.RoleSpouse, "Wife")
	p1.AddRequirement(intersection.ApprovalRequirement{
		ActionClass:   intersection.ActionCalendarRespond,
		RequiredRoles: []intersection.MemberRole{intersection.RoleOwner, intersection.RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 60,
	})

	p2 := intersection.NewIntersectionPolicy("family-001", "Family Intersection", now)
	p2.AddMember("person-satish", intersection.RoleOwner, "Satish")
	p2.AddMember("person-wife", intersection.RoleSpouse, "Wife")
	p2.AddRequirement(intersection.ApprovalRequirement{
		ActionClass:   intersection.ActionCalendarRespond,
		RequiredRoles: []intersection.MemberRole{intersection.RoleOwner, intersection.RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 60,
	})

	if p1.Hash != p2.Hash {
		t.Errorf("intersection policy hashes should match: %s != %s", p1.Hash, p2.Hash)
	}

	t.Logf("Intersection policy determinism verified: hash=%s", p1.Hash[:16])
}

// TestApprovalFlowLifecycle demonstrates the approval flow lifecycle.
func TestApprovalFlowLifecycle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create an approval state requiring both spouses
	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-calendar-123",
		"family-001",
		intersection.ActionCalendarRespond,
		approvers,
		2,  // threshold: both must approve
		60, // max age: 1 hour
		now,
	)

	// Initially pending
	status := state.ComputeStatus(now)
	if status != approvalflow.StatusPending {
		t.Errorf("expected pending, got %s", status)
	}
	t.Logf("Initial status: %s (0 of %d approvals)", status, state.Threshold)

	// Satish approves
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(5 * time.Minute),
		TokenID:   "tok-satish",
		Reason:    "Looks good",
	})

	status = state.ComputeStatus(now.Add(10 * time.Minute))
	if status != approvalflow.StatusPending {
		t.Errorf("expected pending with 1 approval, got %s", status)
	}
	t.Logf("After Satish approves: %s (1 of %d approvals)", status, state.Threshold)

	// Wife approves
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-wife",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(15 * time.Minute),
		TokenID:   "tok-wife",
	})

	status = state.ComputeStatus(now.Add(20 * time.Minute))
	if status != approvalflow.StatusApproved {
		t.Errorf("expected approved, got %s", status)
	}
	t.Logf("After Wife approves: %s (threshold met)", status)
}

// TestApprovalRejection demonstrates rejection flow.
func TestApprovalRejection(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-email-456",
		"family-001",
		intersection.ActionEmailSend,
		approvers,
		2,
		60,
		now,
	)

	// Satish approves
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(5 * time.Minute),
	})

	// Wife rejects
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-wife",
		Decision:  approvalflow.DecisionRejected,
		Timestamp: now.Add(10 * time.Minute),
		Reason:    "Not appropriate",
	})

	status := state.ComputeStatus(now.Add(15 * time.Minute))
	if status != approvalflow.StatusRejected {
		t.Errorf("expected rejected, got %s", status)
	}
	t.Logf("After Wife rejects: %s (any rejection = rejected)", status)
}

// TestSignedApprovalTokens demonstrates token creation and signing.
func TestSignedApprovalTokens(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	// Generate a signing key pair
	keyPair, err := crypto.GenerateEd25519KeyPair("phase15-approval-key", now)
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	signer, err := keyPair.Signer()
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	verifier, err := keyPair.Verifier()
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	// Create a token
	token := approvaltoken.NewToken(
		"state-123",
		"person-satish",
		approvaltoken.ActionTypeApprove,
		now,
		expires,
	)

	// Sign the token
	ctx := context.Background()
	signableBytes := token.SignableBytes()
	sigRecord, err := signer.SignWithRecord(ctx, signableBytes, now)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	token.SetSignature(string(sigRecord.Algorithm), sigRecord.KeyID, sigRecord.Signature)

	t.Logf("Token created: id=%s", token.TokenID)
	t.Logf("Token signed: alg=%s, key=%s", token.SignatureAlgorithm, token.KeyID)

	// Verify the token
	err = verifier.VerifyRecord(ctx, signableBytes, sigRecord)
	if err != nil {
		t.Errorf("token verification failed: %v", err)
	} else {
		t.Log("Token signature verified successfully")
	}

	// Encode token to URL-safe string
	encoded := token.Encode()
	t.Logf("Encoded token length: %d bytes", len(encoded))

	// Decode token
	decoded, err := approvaltoken.Decode(encoded)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}

	if decoded.TokenID != token.TokenID {
		t.Errorf("decoded token ID mismatch")
	}
	if decoded.StateID != token.StateID {
		t.Errorf("decoded state ID mismatch")
	}

	t.Log("Token encode/decode roundtrip verified")
}

// TestApprovalLedgerPersistence demonstrates ledger operations.
func TestApprovalLedgerPersistence(t *testing.T) {
	log := storelog.NewInMemoryLog()
	ledger, err := persist.NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add an intersection policy
	policy := intersection.NewIntersectionPolicy("family-001", "Family", now)
	policy.AddMember("person-satish", intersection.RoleOwner, "Satish")
	policy.AddMember("person-wife", intersection.RoleSpouse, "Wife")
	policy.AddRequirement(intersection.ApprovalRequirement{
		ActionClass:   intersection.ActionCalendarRespond,
		RequiredRoles: []intersection.MemberRole{intersection.RoleOwner, intersection.RoleSpouse},
		Threshold:     2,
	})

	if err := ledger.AddIntersectionPolicy(policy); err != nil {
		t.Fatalf("add policy: %v", err)
	}
	t.Logf("Policy added: %s (members=%d)", policy.IntersectionID, len(policy.Members))

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
	t.Logf("Approval state created: %s (target=%s)", state.StateID, state.TargetID)

	// Record approvals
	if err := ledger.RecordApproval(state.StateID, approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("record approval: %v", err)
	}
	t.Log("Approval recorded: person-satish -> approved")

	// Check stats
	stats := ledger.Stats(now.Add(10 * time.Minute))
	t.Logf("Ledger stats: policies=%d, states=%d, pending=%d",
		stats.PolicyCount, stats.StateCount, stats.PendingStateCount)

	// Verify replay
	ledger2, err := persist.NewApprovalLedger(log)
	if err != nil {
		t.Fatalf("create second ledger: %v", err)
	}

	replayedPolicy := ledger2.GetIntersectionPolicy("family-001")
	if replayedPolicy == nil {
		t.Fatal("policy should be replayed")
	}

	replayedState := ledger2.GetApprovalState(state.StateID)
	if replayedState == nil {
		t.Fatal("state should be replayed")
	}

	if len(replayedState.Approvals) != 1 {
		t.Errorf("expected 1 approval after replay, got %d", len(replayedState.Approvals))
	}

	t.Logf("Ledger replay verified: policy=%s, state=%s, approvals=%d",
		replayedPolicy.IntersectionID, replayedState.StateID, len(replayedState.Approvals))
}

// TestFamilyCalendarScenario demonstrates a realistic family calendar scenario.
func TestFamilyCalendarScenario(t *testing.T) {
	t.Log("=== Family Calendar Approval Scenario ===")
	t.Log("Satish receives a calendar invite for Saturday family dinner.")
	t.Log("The intersection policy requires both Satish and Wife to approve.")

	now := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC) // Wednesday 9 AM

	// Create the family intersection policy
	policy := intersection.NewIntersectionPolicy("family-calendar", "Family Calendar Decisions", now)
	policy.AddMember("person-satish", intersection.RoleOwner, "Satish")
	policy.AddMember("person-wife", intersection.RoleSpouse, "Wife")
	policy.AddRequirement(intersection.ApprovalRequirement{
		ActionClass:   intersection.ActionCalendarRespond,
		RequiredRoles: []intersection.MemberRole{intersection.RoleOwner, intersection.RoleSpouse},
		Threshold:     2, // Both must approve
		MaxAgeMinutes: 120,
	})

	t.Logf("Policy created: %s", policy.Name)
	t.Logf("  - Members: %d (Satish=owner, Wife=spouse)", len(policy.Members))
	t.Logf("  - Threshold: %d (both must approve)", 2)

	// Create approval state for the calendar response draft
	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-saturday-dinner-rsvp",
		"family-calendar",
		intersection.ActionCalendarRespond,
		approvers,
		2,
		120,
		now,
	)

	t.Log("")
	t.Logf("Approval state created at %s", now.Format("Mon 3:04 PM"))
	t.Logf("  - Target: %s", state.TargetID)
	t.Logf("  - Expires: %s", state.ExpiresAt.Format("Mon 3:04 PM"))

	// Generate approval tokens for both
	satishToken := approvaltoken.NewToken(state.StateID, "person-satish", approvaltoken.ActionTypeApprove, now, state.ExpiresAt)
	wifeToken := approvaltoken.NewToken(state.StateID, "person-wife", approvaltoken.ActionTypeApprove, now, state.ExpiresAt)

	t.Log("")
	t.Log("Approval tokens generated:")
	t.Logf("  - Satish: %s", satishToken.TokenID)
	t.Logf("  - Wife: %s", wifeToken.TokenID)

	// Satish approves at 9:15 AM
	satishApprovalTime := now.Add(15 * time.Minute)
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: satishApprovalTime,
		TokenID:   satishToken.TokenID,
	})

	t.Log("")
	t.Logf("Satish approved at %s", satishApprovalTime.Format("Mon 3:04 PM"))
	status := state.ComputeStatus(satishApprovalTime)
	t.Logf("  - Status: %s (waiting for Wife)", status)
	pending := state.GetPendingApprovers()
	t.Logf("  - Pending: %d approvers", len(pending))

	// Wife approves at 10:30 AM
	wifeApprovalTime := now.Add(90 * time.Minute)
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-wife",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: wifeApprovalTime,
		TokenID:   wifeToken.TokenID,
	})

	t.Log("")
	t.Logf("Wife approved at %s", wifeApprovalTime.Format("Mon 3:04 PM"))
	status = state.ComputeStatus(wifeApprovalTime.Add(time.Minute))
	t.Logf("  - Status: %s (threshold met!)", status)

	if status != approvalflow.StatusApproved {
		t.Errorf("expected approved, got %s", status)
	}

	t.Log("")
	t.Log("=== Calendar response can now be sent ===")
	t.Logf("Draft %s is approved by both parties.", state.TargetID)
}

// TestApprovalExpiry demonstrates approval freshness checking.
func TestApprovalExpiry(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
	}

	// Create state with 30 minute max age
	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-test",
		"family",
		intersection.ActionEmailSend,
		approvers,
		1,
		30, // 30 minute max age
		now,
	)

	// Approve now
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now,
	})

	// Check at 20 minutes (should be approved, approval is fresh)
	status := state.ComputeStatus(now.Add(20 * time.Minute))
	if status != approvalflow.StatusApproved {
		t.Errorf("at 20 min: expected approved, got %s", status)
	}
	t.Logf("At 20 minutes: %s (approval still fresh)", status)

	// Check at 40 minutes (state itself expires at 30 min)
	status = state.ComputeStatus(now.Add(40 * time.Minute))
	if status != approvalflow.StatusExpired {
		t.Errorf("at 40 min: expected expired, got %s", status)
	}
	t.Logf("At 40 minutes: %s (state expired)", status)
}

// TestMemberOrderDeterminism demonstrates that member order doesn't affect hash.
func TestMemberOrderDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add members in different orders
	p1 := intersection.NewIntersectionPolicy("test", "Test", now)
	p1.AddMember("person-z", intersection.RoleChild, "Z")
	p1.AddMember("person-a", intersection.RoleOwner, "A")
	p1.AddMember("person-m", intersection.RoleSpouse, "M")

	p2 := intersection.NewIntersectionPolicy("test", "Test", now)
	p2.AddMember("person-a", intersection.RoleOwner, "A")
	p2.AddMember("person-m", intersection.RoleSpouse, "M")
	p2.AddMember("person-z", intersection.RoleChild, "Z")

	if p1.Hash != p2.Hash {
		t.Errorf("member order should not affect hash: %s != %s", p1.Hash, p2.Hash)
	}

	// Verify first member is always person-a (sorted)
	if p1.Members[0].PersonID != "person-a" {
		t.Errorf("first member should be person-a (sorted), got %s", p1.Members[0].PersonID)
	}

	t.Logf("Member order determinism verified: hash=%s", p1.Hash[:16])
}
