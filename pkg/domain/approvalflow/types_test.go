package approvalflow

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/intersection"
)

func TestApprovalStateDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	// Create two identical states
	s1 := NewApprovalState(
		TargetTypeDraft,
		"draft-123",
		"family-001",
		intersection.ActionCalendarRespond,
		approvers,
		2,
		60,
		now,
	)

	s2 := NewApprovalState(
		TargetTypeDraft,
		"draft-123",
		"family-001",
		intersection.ActionCalendarRespond,
		approvers,
		2,
		60,
		now,
	)

	if s1.StateID != s2.StateID {
		t.Errorf("state IDs should match: %s != %s", s1.StateID, s2.StateID)
	}

	if s1.Hash != s2.Hash {
		t.Errorf("hashes should match: %s != %s", s1.Hash, s2.Hash)
	}

	if s1.CanonicalString() != s2.CanonicalString() {
		t.Error("canonical strings should match")
	}

	t.Logf("State determinism verified: id=%s", s1.StateID)
}

func TestApproverOrderingDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add approvers in different orders
	approvers1 := []ApproverRef{
		{PersonID: "person-a", Role: intersection.RoleOwner},
		{PersonID: "person-c", Role: intersection.RoleChild},
		{PersonID: "person-b", Role: intersection.RoleSpouse},
	}

	approvers2 := []ApproverRef{
		{PersonID: "person-c", Role: intersection.RoleChild},
		{PersonID: "person-a", Role: intersection.RoleOwner},
		{PersonID: "person-b", Role: intersection.RoleSpouse},
	}

	s1 := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers1, 2, 60, now)
	s2 := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers2, 2, 60, now)

	if s1.Hash != s2.Hash {
		t.Errorf("approver order should not affect hash: %s != %s", s1.Hash, s2.Hash)
	}

	// Verify sorted order
	if string(s1.RequiredApprovers[0].PersonID) != "person-a" {
		t.Errorf("first approver should be person-a, got %s", s1.RequiredApprovers[0].PersonID)
	}

	t.Logf("Approver sorting verified")
}

func TestApprovalRecordingDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	// Create two states and record approvals in different orders
	s1 := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)
	s2 := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)

	// Record approvals in different orders
	s1.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute), TokenID: "tok-1"})
	s1.RecordApproval(ApprovalRecord{PersonID: "person-wife", Decision: DecisionApproved, Timestamp: now.Add(2 * time.Minute), TokenID: "tok-2"})

	s2.RecordApproval(ApprovalRecord{PersonID: "person-wife", Decision: DecisionApproved, Timestamp: now.Add(2 * time.Minute), TokenID: "tok-2"})
	s2.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute), TokenID: "tok-1"})

	if s1.Hash != s2.Hash {
		t.Errorf("approval order should not affect hash: %s != %s", s1.Hash, s2.Hash)
	}

	t.Logf("Approval ordering determinism verified")
}

func TestComputeStatusPending(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)

	// No approvals yet
	status := s.ComputeStatus(now)
	if status != StatusPending {
		t.Errorf("expected pending, got %s", status)
	}

	// One approval (below threshold)
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})
	status = s.ComputeStatus(now.Add(2 * time.Minute))
	if status != StatusPending {
		t.Errorf("expected pending with 1 approval, got %s", status)
	}
}

func TestComputeStatusApproved(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)

	// Record both approvals
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})
	s.RecordApproval(ApprovalRecord{PersonID: "person-wife", Decision: DecisionApproved, Timestamp: now.Add(2 * time.Minute)})

	status := s.ComputeStatus(now.Add(5 * time.Minute))
	if status != StatusApproved {
		t.Errorf("expected approved, got %s", status)
	}
}

func TestComputeStatusRejected(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)

	// One approval, one rejection
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})
	s.RecordApproval(ApprovalRecord{PersonID: "person-wife", Decision: DecisionRejected, Timestamp: now.Add(2 * time.Minute), Reason: "Not available"})

	status := s.ComputeStatus(now.Add(5 * time.Minute))
	if status != StatusRejected {
		t.Errorf("expected rejected, got %s", status)
	}
}

func TestComputeStatusExpired(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 1, 30, now)

	// Check after expiry
	status := s.ComputeStatus(now.Add(31 * time.Minute))
	if status != StatusExpired {
		t.Errorf("expected expired after 31 minutes (max_age=30), got %s", status)
	}
}

func TestApprovalFreshnessExpiry(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	// Use 60 min state expiry, but 30 min approval freshness
	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 30, now)
	// Override expiry to be longer than max_age for this test
	s.ExpiresAt = now.Add(60 * time.Minute)

	// Record old approval and fresh approval
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now})
	s.RecordApproval(ApprovalRecord{PersonID: "person-wife", Decision: DecisionApproved, Timestamp: now.Add(25 * time.Minute)})

	// At 20 minutes: both approvals fresh, should be approved
	status := s.ComputeStatus(now.Add(20 * time.Minute))
	if status != StatusApproved {
		t.Errorf("expected approved at 20 min, got %s", status)
	}

	// At 35 minutes: satish's approval is stale (35 min old > 30 min max_age)
	// but wife's approval is still fresh (10 min old), only 1 fresh approval
	status = s.ComputeStatus(now.Add(35 * time.Minute))
	if status != StatusPending {
		t.Errorf("expected pending at 35 min (stale approval), got %s", status)
	}
}

func TestIsApproverRequired(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)

	if !s.IsApproverRequired("person-satish") {
		t.Error("person-satish should be required")
	}

	if !s.IsApproverRequired("person-wife") {
		t.Error("person-wife should be required")
	}

	if s.IsApproverRequired("person-unknown") {
		t.Error("person-unknown should not be required")
	}
}

func TestGetPendingApprovers(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
		{PersonID: "person-child", Role: intersection.RoleChild},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 2, 60, now)

	// All pending initially
	pending := s.GetPendingApprovers()
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}

	// Record one approval
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})

	pending = s.GetPendingApprovers()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending after 1 approval, got %d", len(pending))
	}

	// Verify person-satish is not in pending
	for _, p := range pending {
		if p.PersonID == "person-satish" {
			t.Error("person-satish should not be in pending after approval")
		}
	}
}

func TestApprovalStateSetDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create two identical sets
	set1 := NewApprovalStateSet()
	set2 := NewApprovalStateSet()

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
	}

	s1 := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 1, 60, now)
	s2 := NewApprovalState(TargetTypeDraft, "draft-1", "family", intersection.ActionCalendarRespond, approvers, 1, 60, now)

	set1.Add(s1)
	set2.Add(s2)

	if set1.Hash != set2.Hash {
		t.Errorf("set hashes should match: %s != %s", set1.Hash, set2.Hash)
	}

	t.Logf("StateSet determinism verified: hash=%s", set1.Hash[:16])
}

func TestApprovalStateSetListOrder(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewApprovalStateSet()

	approvers := []ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}

	// Add states (they'll have different IDs based on target)
	s1 := NewApprovalState(TargetTypeDraft, "draft-zzz", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	s2 := NewApprovalState(TargetTypeDraft, "draft-aaa", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	s3 := NewApprovalState(TargetTypeDraft, "draft-mmm", "", intersection.ActionEmailSend, approvers, 1, 60, now)

	set.Add(s1)
	set.Add(s2)
	set.Add(s3)

	states := set.List()
	if len(states) != 3 {
		t.Fatalf("expected 3 states, got %d", len(states))
	}

	// Should be sorted by state ID
	for i := 0; i < len(states)-1; i++ {
		if states[i].StateID > states[i+1].StateID {
			t.Errorf("states not sorted: %s > %s", states[i].StateID, states[i+1].StateID)
		}
	}
}

func TestApprovalStateSetListPending(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewApprovalStateSet()

	approvers := []ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}

	s1 := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	s2 := NewApprovalState(TargetTypeDraft, "draft-2", "", intersection.ActionEmailSend, approvers, 1, 60, now)

	// Approve s1
	s1.RecordApproval(ApprovalRecord{PersonID: "person-1", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})

	set.Add(s1)
	set.Add(s2)

	pending := set.ListPending(now.Add(5 * time.Minute))
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}

	if pending[0].TargetID != "draft-2" {
		t.Errorf("expected draft-2 to be pending, got %s", pending[0].TargetID)
	}
}

func TestApprovalStateSetListForPerson(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewApprovalStateSet()

	approvers1 := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	approvers2 := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
	}

	approvers3 := []ApproverRef{
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	set.Add(NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers1, 2, 60, now))
	set.Add(NewApprovalState(TargetTypeDraft, "draft-2", "", intersection.ActionEmailSend, approvers2, 1, 60, now))
	set.Add(NewApprovalState(TargetTypeDraft, "draft-3", "", intersection.ActionEmailSend, approvers3, 1, 60, now))

	// Satish is in draft-1 and draft-2
	satishStates := set.ListForPerson("person-satish")
	if len(satishStates) != 2 {
		t.Errorf("expected 2 states for satish, got %d", len(satishStates))
	}

	// Wife is in draft-1 and draft-3
	wifeStates := set.ListForPerson("person-wife")
	if len(wifeStates) != 2 {
		t.Errorf("expected 2 states for wife, got %d", len(wifeStates))
	}
}

func TestApprovalStateSetPruneExpired(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewApprovalStateSet()

	approvers := []ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}

	// One with short expiry
	s1 := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 1, 10, now)
	// One with longer expiry
	s2 := NewApprovalState(TargetTypeDraft, "draft-2", "", intersection.ActionEmailSend, approvers, 1, 60, now)

	set.Add(s1)
	set.Add(s2)

	// Prune at 15 minutes (s1 should be expired)
	pruned := set.PruneExpired(now.Add(15 * time.Minute))
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	if len(set.States) != 1 {
		t.Errorf("expected 1 state remaining, got %d", len(set.States))
	}

	if set.Get(s2.StateID) == nil {
		t.Error("s2 should still exist")
	}
}

func TestGetStats(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewApprovalStateSet()

	approvers := []ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}

	// Pending
	s1 := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 1, 60, now)

	// Approved
	s2 := NewApprovalState(TargetTypeDraft, "draft-2", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	s2.RecordApproval(ApprovalRecord{PersonID: "person-1", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})

	// Rejected
	s3 := NewApprovalState(TargetTypeDraft, "draft-3", "", intersection.ActionEmailSend, approvers, 1, 60, now)
	s3.RecordApproval(ApprovalRecord{PersonID: "person-1", Decision: DecisionRejected, Timestamp: now.Add(time.Minute)})

	// Will be expired
	s4 := NewApprovalState(TargetTypeDraft, "draft-4", "", intersection.ActionEmailSend, approvers, 1, 10, now)

	set.Add(s1)
	set.Add(s2)
	set.Add(s3)
	set.Add(s4)

	stats := set.GetStats(now.Add(15 * time.Minute))

	if stats.TotalStates != 4 {
		t.Errorf("expected 4 total, got %d", stats.TotalStates)
	}
	if stats.PendingCount != 1 {
		t.Errorf("expected 1 pending, got %d", stats.PendingCount)
	}
	if stats.ApprovedCount != 1 {
		t.Errorf("expected 1 approved, got %d", stats.ApprovedCount)
	}
	if stats.RejectedCount != 1 {
		t.Errorf("expected 1 rejected, got %d", stats.RejectedCount)
	}
	if stats.ExpiredCount != 1 {
		t.Errorf("expected 1 expired, got %d", stats.ExpiredCount)
	}
}

func TestDefaultThresholdAndMaxAge(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-1", Role: intersection.RoleOwner},
		{PersonID: "person-2", Role: intersection.RoleSpouse},
	}

	// Threshold 0 defaults to all approvers
	s := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 0, 0, now)

	if s.Threshold != 2 {
		t.Errorf("default threshold should be 2 (all approvers), got %d", s.Threshold)
	}

	if s.MaxAgeMinutes != 60 {
		t.Errorf("default max_age should be 60, got %d", s.MaxAgeMinutes)
	}
}

func TestUpdateExistingApproval(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
	}

	s := NewApprovalState(TargetTypeDraft, "draft-1", "", intersection.ActionEmailSend, approvers, 1, 60, now)

	// First approval
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionApproved, Timestamp: now.Add(time.Minute)})
	if len(s.Approvals) != 1 {
		t.Errorf("expected 1 approval, got %d", len(s.Approvals))
	}

	// Change mind - reject
	s.RecordApproval(ApprovalRecord{PersonID: "person-satish", Decision: DecisionRejected, Timestamp: now.Add(2 * time.Minute), Reason: "Changed mind"})
	if len(s.Approvals) != 1 {
		t.Errorf("expected still 1 approval after update, got %d", len(s.Approvals))
	}

	approval := s.GetApproval("person-satish")
	if approval.Decision != DecisionRejected {
		t.Errorf("expected decision to be rejected, got %s", approval.Decision)
	}

	status := s.ComputeStatus(now.Add(5 * time.Minute))
	if status != StatusRejected {
		t.Errorf("expected status rejected, got %s", status)
	}
}

func TestTargetTypes(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	approvers := []ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}

	tests := []struct {
		targetType TargetType
		targetID   string
	}{
		{TargetTypeDraft, "draft-123"},
		{TargetTypeExecutionIntent, "intent-456"},
		{TargetTypeEnvelope, "envelope-789"},
	}

	for _, tt := range tests {
		s := NewApprovalState(tt.targetType, tt.targetID, "", intersection.ActionEmailSend, approvers, 1, 60, now)
		if s.TargetType != tt.targetType {
			t.Errorf("expected target type %s, got %s", tt.targetType, s.TargetType)
		}
		if s.TargetID != tt.targetID {
			t.Errorf("expected target ID %s, got %s", tt.targetID, s.TargetID)
		}
	}
}

func TestGetByTarget(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewApprovalStateSet()

	approvers := []ApproverRef{{PersonID: "person-1", Role: intersection.RoleOwner}}

	set.Add(NewApprovalState(TargetTypeDraft, "draft-123", "", intersection.ActionEmailSend, approvers, 1, 60, now))
	set.Add(NewApprovalState(TargetTypeExecutionIntent, "intent-456", "", intersection.ActionEmailSend, approvers, 1, 60, now))

	// Find by target
	s := set.GetByTarget(TargetTypeDraft, "draft-123")
	if s == nil {
		t.Fatal("should find draft-123")
	}
	if s.TargetID != "draft-123" {
		t.Errorf("expected draft-123, got %s", s.TargetID)
	}

	// Non-existent
	s = set.GetByTarget(TargetTypeDraft, "draft-999")
	if s != nil {
		t.Error("should not find draft-999")
	}
}

func TestApproverRefCanonicalString(t *testing.T) {
	a := ApproverRef{
		PersonID: identity.EntityID("person-satish"),
		Role:     intersection.RoleOwner,
	}

	cs := a.CanonicalString()
	expected := "person:person-satish|role:owner"
	if cs != expected {
		t.Errorf("expected %s, got %s", expected, cs)
	}
}

func TestApprovalRecordCanonicalString(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	r := ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  DecisionApproved,
		Timestamp: now,
		TokenID:   "tok-123",
	}

	cs := r.CanonicalString()
	if cs == "" {
		t.Error("canonical string should not be empty")
	}
	if !contains(cs, "person:person-satish") {
		t.Error("should contain person ID")
	}
	if !contains(cs, "decision:approved") {
		t.Error("should contain decision")
	}
	if !contains(cs, "token:tok-123") {
		t.Error("should contain token ID")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
