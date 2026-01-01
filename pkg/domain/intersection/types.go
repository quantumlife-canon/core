// Package intersection defines household intersection policies for multi-party approvals.
//
// Phase 15: Household Approvals + Intersections (Deterministic)
//
// An intersection is a shared domain between household members where certain
// actions require multi-party approval. Examples:
// - Family calendar responses may require both spouses to approve
// - Financial payments may require household consensus
//
// CRITICAL: All operations are deterministic. Same inputs + clock => same outputs.
// CRITICAL: No goroutines. No time.Now(). Clock must be injected.
//
// Reference: docs/ADR/ADR-0031-phase15-household-approvals.md
package intersection

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MemberRole defines the role a member plays in an intersection.
type MemberRole string

const (
	// RoleOwner is the primary account holder.
	RoleOwner MemberRole = "owner"

	// RoleSpouse is a spouse or partner.
	RoleSpouse MemberRole = "spouse"

	// RoleParent is a parent (for child intersections).
	RoleParent MemberRole = "parent"

	// RoleChild is a dependent child.
	RoleChild MemberRole = "child"

	// RoleGuardian is a legal guardian.
	RoleGuardian MemberRole = "guardian"
)

// ActionClass defines categories of actions that may require approval.
type ActionClass string

const (
	// ActionEmailSend is sending emails.
	ActionEmailSend ActionClass = "email_send"

	// ActionCalendarRespond is responding to calendar invites.
	ActionCalendarRespond ActionClass = "calendar_respond"

	// ActionCalendarCreate is creating calendar events.
	ActionCalendarCreate ActionClass = "calendar_create"

	// ActionFinancePayment is making financial payments.
	ActionFinancePayment ActionClass = "finance_payment"

	// ActionFinanceTransfer is transferring funds.
	ActionFinanceTransfer ActionClass = "finance_transfer"
)

// MemberRef identifies a member of an intersection.
type MemberRef struct {
	// PersonID is the identity graph person ID.
	PersonID string

	// Role is the member's role in the intersection.
	Role MemberRole

	// DisplayName is the member's display name for UI.
	DisplayName string
}

// CanonicalString returns a deterministic representation.
func (m MemberRef) CanonicalString() string {
	return fmt.Sprintf("person:%s|role:%s", m.PersonID, m.Role)
}

// ApprovalRequirement defines what approval is needed for an action class.
type ApprovalRequirement struct {
	// ActionClass is the type of action requiring approval.
	ActionClass ActionClass

	// RequiredRoles are the roles that must approve.
	RequiredRoles []MemberRole

	// Threshold is the minimum number of approvals needed.
	// If 0, all RequiredRoles must approve.
	Threshold int

	// MaxAgeMinutes is the freshness window for approvals.
	// Approvals older than this must be re-requested.
	// Default: 60 (1 hour)
	MaxAgeMinutes int
}

// CanonicalString returns a deterministic representation.
func (r ApprovalRequirement) CanonicalString() string {
	roles := make([]string, len(r.RequiredRoles))
	for i, role := range r.RequiredRoles {
		roles[i] = string(role)
	}
	// Sort roles for determinism
	bubbleSort(roles)
	return fmt.Sprintf("action:%s|roles:[%s]|threshold:%d|max_age:%d",
		r.ActionClass, strings.Join(roles, ","), r.Threshold, r.MaxAgeMinutes)
}

// IntersectionPolicy defines the policy for a household intersection.
type IntersectionPolicy struct {
	// IntersectionID uniquely identifies this intersection.
	IntersectionID string

	// Name is the human-readable name.
	Name string

	// Members are the members of this intersection.
	Members []MemberRef

	// Requirements define approval requirements for action classes.
	Requirements []ApprovalRequirement

	// CreatedAt is when this policy was created.
	CreatedAt time.Time

	// Version is incremented on each update.
	Version int

	// Hash is the SHA256 hash of the canonical string.
	Hash string
}

// NewIntersectionPolicy creates a new intersection policy.
func NewIntersectionPolicy(id, name string, createdAt time.Time) *IntersectionPolicy {
	p := &IntersectionPolicy{
		IntersectionID: id,
		Name:           name,
		Members:        []MemberRef{},
		Requirements:   []ApprovalRequirement{},
		CreatedAt:      createdAt,
		Version:        1,
	}
	p.ComputeHash()
	return p
}

// AddMember adds a member to the intersection.
func (p *IntersectionPolicy) AddMember(personID string, role MemberRole, displayName string) {
	p.Members = append(p.Members, MemberRef{
		PersonID:    personID,
		Role:        role,
		DisplayName: displayName,
	})
	p.sortMembers()
	p.ComputeHash()
}

// AddRequirement adds an approval requirement.
func (p *IntersectionPolicy) AddRequirement(req ApprovalRequirement) {
	// Set default max age if not specified
	if req.MaxAgeMinutes <= 0 {
		req.MaxAgeMinutes = 60 // 1 hour default
	}
	// Set threshold to all roles if not specified
	if req.Threshold <= 0 {
		req.Threshold = len(req.RequiredRoles)
	}
	p.Requirements = append(p.Requirements, req)
	p.sortRequirements()
	p.ComputeHash()
}

// GetRequirement returns the requirement for an action class.
func (p *IntersectionPolicy) GetRequirement(action ActionClass) *ApprovalRequirement {
	for i := range p.Requirements {
		if p.Requirements[i].ActionClass == action {
			return &p.Requirements[i]
		}
	}
	return nil
}

// GetMemberByPersonID returns a member by person ID.
func (p *IntersectionPolicy) GetMemberByPersonID(personID string) *MemberRef {
	for i := range p.Members {
		if p.Members[i].PersonID == personID {
			return &p.Members[i]
		}
	}
	return nil
}

// GetMembersByRole returns all members with a given role.
func (p *IntersectionPolicy) GetMembersByRole(role MemberRole) []MemberRef {
	var result []MemberRef
	for _, m := range p.Members {
		if m.Role == role {
			result = append(result, m)
		}
	}
	return result
}

// CanonicalString returns a deterministic representation.
func (p *IntersectionPolicy) CanonicalString() string {
	var sb strings.Builder

	sb.WriteString("intersection|")
	sb.WriteString("id:")
	sb.WriteString(p.IntersectionID)
	sb.WriteString("|name:")
	sb.WriteString(p.Name)
	sb.WriteString("|created:")
	sb.WriteString(p.CreatedAt.UTC().Format(time.RFC3339))
	sb.WriteString("|version:")
	sb.WriteString(fmt.Sprintf("%d", p.Version))

	// Members (already sorted)
	sb.WriteString("|members:[")
	for i, m := range p.Members {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(m.CanonicalString())
	}
	sb.WriteString("]")

	// Requirements (already sorted)
	sb.WriteString("|requirements:[")
	for i, r := range p.Requirements {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(r.CanonicalString())
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (p *IntersectionPolicy) ComputeHash() string {
	canonical := p.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	p.Hash = hex.EncodeToString(hash[:])
	return p.Hash
}

// sortMembers sorts members by PersonID for determinism.
func (p *IntersectionPolicy) sortMembers() {
	// Bubble sort for stdlib-only
	for i := 0; i < len(p.Members); i++ {
		for j := i + 1; j < len(p.Members); j++ {
			if p.Members[i].PersonID > p.Members[j].PersonID {
				p.Members[i], p.Members[j] = p.Members[j], p.Members[i]
			}
		}
	}
}

// sortRequirements sorts requirements by ActionClass for determinism.
func (p *IntersectionPolicy) sortRequirements() {
	// Bubble sort for stdlib-only
	for i := 0; i < len(p.Requirements); i++ {
		for j := i + 1; j < len(p.Requirements); j++ {
			if string(p.Requirements[i].ActionClass) > string(p.Requirements[j].ActionClass) {
				p.Requirements[i], p.Requirements[j] = p.Requirements[j], p.Requirements[i]
			}
		}
	}
}

// bubbleSort sorts strings in place.
func bubbleSort(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// IntersectionPolicySet holds multiple intersection policies.
type IntersectionPolicySet struct {
	// Policies maps intersection ID to policy.
	Policies map[string]*IntersectionPolicy

	// Version is incremented on each update.
	Version int

	// Hash is the SHA256 hash of the set.
	Hash string
}

// NewIntersectionPolicySet creates an empty policy set.
func NewIntersectionPolicySet() *IntersectionPolicySet {
	s := &IntersectionPolicySet{
		Policies: make(map[string]*IntersectionPolicy),
		Version:  1,
	}
	s.ComputeHash()
	return s
}

// Add adds a policy to the set.
func (s *IntersectionPolicySet) Add(policy *IntersectionPolicy) {
	s.Policies[policy.IntersectionID] = policy
	s.Version++
	s.ComputeHash()
}

// Get returns a policy by ID.
func (s *IntersectionPolicySet) Get(intersectionID string) *IntersectionPolicy {
	return s.Policies[intersectionID]
}

// List returns all policies in deterministic order.
func (s *IntersectionPolicySet) List() []*IntersectionPolicy {
	// Collect IDs
	ids := make([]string, 0, len(s.Policies))
	for id := range s.Policies {
		ids = append(ids, id)
	}
	// Sort IDs
	bubbleSort(ids)
	// Build result
	result := make([]*IntersectionPolicy, len(ids))
	for i, id := range ids {
		result[i] = s.Policies[id]
	}
	return result
}

// CanonicalString returns a deterministic representation.
func (s *IntersectionPolicySet) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("policy_set|version:")
	sb.WriteString(fmt.Sprintf("%d", s.Version))
	sb.WriteString("|policies:[")

	policies := s.List()
	for i, p := range policies {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(p.CanonicalString())
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (s *IntersectionPolicySet) ComputeHash() string {
	canonical := s.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	s.Hash = hex.EncodeToString(hash[:])
	return s.Hash
}

// FindPoliciesForPerson returns all policies where person is a member.
func (s *IntersectionPolicySet) FindPoliciesForPerson(personID string) []*IntersectionPolicy {
	var result []*IntersectionPolicy
	for _, p := range s.Policies {
		for _, m := range p.Members {
			if m.PersonID == personID {
				result = append(result, p)
				break
			}
		}
	}
	return result
}
