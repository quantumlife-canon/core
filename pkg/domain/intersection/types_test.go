package intersection

import (
	"testing"
	"time"
)

func TestIntersectionPolicyDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create two identical policies
	p1 := NewIntersectionPolicy("family-001", "Family Intersection", now)
	p1.AddMember("person-satish", RoleOwner, "Satish")
	p1.AddMember("person-wife", RoleSpouse, "Wife")
	p1.AddRequirement(ApprovalRequirement{
		ActionClass:   ActionCalendarRespond,
		RequiredRoles: []MemberRole{RoleOwner, RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 60,
	})

	p2 := NewIntersectionPolicy("family-001", "Family Intersection", now)
	p2.AddMember("person-satish", RoleOwner, "Satish")
	p2.AddMember("person-wife", RoleSpouse, "Wife")
	p2.AddRequirement(ApprovalRequirement{
		ActionClass:   ActionCalendarRespond,
		RequiredRoles: []MemberRole{RoleOwner, RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 60,
	})

	if p1.Hash != p2.Hash {
		t.Errorf("policy hashes should match: %s != %s", p1.Hash, p2.Hash)
	}

	if p1.CanonicalString() != p2.CanonicalString() {
		t.Error("canonical strings should match")
	}

	t.Logf("Policy determinism verified: hash=%s", p1.Hash[:16])
}

func TestMemberSortingDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Add members in different orders
	p1 := NewIntersectionPolicy("test", "Test", now)
	p1.AddMember("person-a", RoleOwner, "A")
	p1.AddMember("person-c", RoleSpouse, "C")
	p1.AddMember("person-b", RoleChild, "B")

	p2 := NewIntersectionPolicy("test", "Test", now)
	p2.AddMember("person-c", RoleSpouse, "C")
	p2.AddMember("person-a", RoleOwner, "A")
	p2.AddMember("person-b", RoleChild, "B")

	if p1.Hash != p2.Hash {
		t.Errorf("member order should not affect hash: %s != %s", p1.Hash, p2.Hash)
	}

	// Verify sorted order
	if p1.Members[0].PersonID != "person-a" {
		t.Errorf("first member should be person-a, got %s", p1.Members[0].PersonID)
	}

	t.Logf("Member sorting verified")
}

func TestRequirementSortingDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p1 := NewIntersectionPolicy("test", "Test", now)
	p1.AddRequirement(ApprovalRequirement{ActionClass: ActionFinancePayment})
	p1.AddRequirement(ApprovalRequirement{ActionClass: ActionCalendarRespond})
	p1.AddRequirement(ApprovalRequirement{ActionClass: ActionEmailSend})

	p2 := NewIntersectionPolicy("test", "Test", now)
	p2.AddRequirement(ApprovalRequirement{ActionClass: ActionCalendarRespond})
	p2.AddRequirement(ApprovalRequirement{ActionClass: ActionEmailSend})
	p2.AddRequirement(ApprovalRequirement{ActionClass: ActionFinancePayment})

	if p1.Hash != p2.Hash {
		t.Errorf("requirement order should not affect hash: %s != %s", p1.Hash, p2.Hash)
	}

	t.Logf("Requirement sorting verified")
}

func TestGetRequirement(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p := NewIntersectionPolicy("test", "Test", now)
	p.AddRequirement(ApprovalRequirement{
		ActionClass:   ActionCalendarRespond,
		RequiredRoles: []MemberRole{RoleOwner, RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 30,
	})

	req := p.GetRequirement(ActionCalendarRespond)
	if req == nil {
		t.Fatal("should find calendar_respond requirement")
	}
	if req.Threshold != 2 {
		t.Errorf("threshold should be 2, got %d", req.Threshold)
	}
	if req.MaxAgeMinutes != 30 {
		t.Errorf("max age should be 30, got %d", req.MaxAgeMinutes)
	}

	// Non-existent requirement
	req = p.GetRequirement(ActionFinancePayment)
	if req != nil {
		t.Error("should not find finance_payment requirement")
	}
}

func TestGetMemberByPersonID(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p := NewIntersectionPolicy("test", "Test", now)
	p.AddMember("person-satish", RoleOwner, "Satish")
	p.AddMember("person-wife", RoleSpouse, "Wife")

	member := p.GetMemberByPersonID("person-satish")
	if member == nil {
		t.Fatal("should find person-satish")
	}
	if member.Role != RoleOwner {
		t.Errorf("role should be owner, got %s", member.Role)
	}

	// Non-existent member
	member = p.GetMemberByPersonID("person-unknown")
	if member != nil {
		t.Error("should not find unknown person")
	}
}

func TestGetMembersByRole(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p := NewIntersectionPolicy("test", "Test", now)
	p.AddMember("person-satish", RoleOwner, "Satish")
	p.AddMember("person-wife", RoleSpouse, "Wife")
	p.AddMember("person-child1", RoleChild, "Child 1")
	p.AddMember("person-child2", RoleChild, "Child 2")

	children := p.GetMembersByRole(RoleChild)
	if len(children) != 2 {
		t.Errorf("should have 2 children, got %d", len(children))
	}

	owners := p.GetMembersByRole(RoleOwner)
	if len(owners) != 1 {
		t.Errorf("should have 1 owner, got %d", len(owners))
	}
}

func TestIntersectionPolicySetDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create two identical sets
	s1 := NewIntersectionPolicySet()
	p1 := NewIntersectionPolicy("family-001", "Family", now)
	p1.AddMember("person-1", RoleOwner, "Owner")
	s1.Add(p1)

	s2 := NewIntersectionPolicySet()
	p2 := NewIntersectionPolicy("family-001", "Family", now)
	p2.AddMember("person-1", RoleOwner, "Owner")
	s2.Add(p2)

	if s1.Hash != s2.Hash {
		t.Errorf("set hashes should match: %s != %s", s1.Hash, s2.Hash)
	}

	t.Logf("PolicySet determinism verified: hash=%s", s1.Hash[:16])
}

func TestIntersectionPolicySetList(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s := NewIntersectionPolicySet()
	s.Add(NewIntersectionPolicy("z-family", "Z Family", now))
	s.Add(NewIntersectionPolicy("a-family", "A Family", now))
	s.Add(NewIntersectionPolicy("m-family", "M Family", now))

	policies := s.List()
	if len(policies) != 3 {
		t.Fatalf("should have 3 policies, got %d", len(policies))
	}

	// Should be sorted by ID
	if policies[0].IntersectionID != "a-family" {
		t.Errorf("first should be a-family, got %s", policies[0].IntersectionID)
	}
	if policies[1].IntersectionID != "m-family" {
		t.Errorf("second should be m-family, got %s", policies[1].IntersectionID)
	}
	if policies[2].IntersectionID != "z-family" {
		t.Errorf("third should be z-family, got %s", policies[2].IntersectionID)
	}
}

func TestFindPoliciesForPerson(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s := NewIntersectionPolicySet()

	p1 := NewIntersectionPolicy("family-1", "Family 1", now)
	p1.AddMember("person-satish", RoleOwner, "Satish")
	p1.AddMember("person-wife", RoleSpouse, "Wife")
	s.Add(p1)

	p2 := NewIntersectionPolicy("work-1", "Work 1", now)
	p2.AddMember("person-satish", RoleOwner, "Satish")
	p2.AddMember("person-colleague", RoleSpouse, "Colleague")
	s.Add(p2)

	p3 := NewIntersectionPolicy("other-1", "Other 1", now)
	p3.AddMember("person-other", RoleOwner, "Other")
	s.Add(p3)

	// Satish is in 2 policies
	policies := s.FindPoliciesForPerson("person-satish")
	if len(policies) != 2 {
		t.Errorf("satish should be in 2 policies, got %d", len(policies))
	}

	// Wife is in 1 policy
	policies = s.FindPoliciesForPerson("person-wife")
	if len(policies) != 1 {
		t.Errorf("wife should be in 1 policy, got %d", len(policies))
	}

	// Unknown is in 0 policies
	policies = s.FindPoliciesForPerson("person-unknown")
	if len(policies) != 0 {
		t.Errorf("unknown should be in 0 policies, got %d", len(policies))
	}
}

func TestDefaultMaxAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p := NewIntersectionPolicy("test", "Test", now)
	p.AddRequirement(ApprovalRequirement{
		ActionClass:   ActionCalendarRespond,
		RequiredRoles: []MemberRole{RoleOwner},
		// MaxAgeMinutes not set
	})

	req := p.GetRequirement(ActionCalendarRespond)
	if req.MaxAgeMinutes != 60 {
		t.Errorf("default max age should be 60, got %d", req.MaxAgeMinutes)
	}
}

func TestDefaultThreshold(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	p := NewIntersectionPolicy("test", "Test", now)
	p.AddRequirement(ApprovalRequirement{
		ActionClass:   ActionCalendarRespond,
		RequiredRoles: []MemberRole{RoleOwner, RoleSpouse},
		// Threshold not set
	})

	req := p.GetRequirement(ActionCalendarRespond)
	if req.Threshold != 2 {
		t.Errorf("default threshold should be 2 (all roles), got %d", req.Threshold)
	}
}
