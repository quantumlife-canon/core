// Package approvalflow defines approval state and records for multi-party approvals.
//
// Phase 15: Household Approvals + Intersections (Deterministic)
//
// An approval flow tracks the state of approvals for a target (draft, execution intent,
// or envelope). Multiple approvers may be required based on intersection policies.
//
// CRITICAL: All operations are deterministic. Same inputs + clock => same outputs.
// CRITICAL: No goroutines. No time.Now(). Clock must be injected.
//
// Reference: docs/ADR/ADR-0031-phase15-household-approvals.md
package approvalflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/intersection"
)

// TargetType defines what is being approved.
type TargetType string

const (
	// TargetTypeDraft is an email or calendar draft awaiting approval.
	TargetTypeDraft TargetType = "draft"

	// TargetTypeExecutionIntent is an intent to execute an action.
	TargetTypeExecutionIntent TargetType = "execution_intent"

	// TargetTypeEnvelope is a sealed envelope containing an action.
	TargetTypeEnvelope TargetType = "envelope"
)

// Decision is the approver's decision.
type Decision string

const (
	// DecisionApproved indicates the approver approves the action.
	DecisionApproved Decision = "approved"

	// DecisionRejected indicates the approver rejects the action.
	DecisionRejected Decision = "rejected"
)

// Status is the computed status of an approval flow.
type Status string

const (
	// StatusPending indicates not enough approvals yet.
	StatusPending Status = "pending"

	// StatusApproved indicates threshold met.
	StatusApproved Status = "approved"

	// StatusRejected indicates at least one rejection.
	StatusRejected Status = "rejected"

	// StatusExpired indicates approvals have expired.
	StatusExpired Status = "expired"
)

// ApproverRef identifies a required approver.
type ApproverRef struct {
	// PersonID is the identity graph person ID.
	PersonID identity.EntityID

	// Role is the approver's role in the intersection.
	Role intersection.MemberRole
}

// CanonicalString returns a deterministic representation.
func (a ApproverRef) CanonicalString() string {
	return fmt.Sprintf("person:%s|role:%s", a.PersonID, a.Role)
}

// ApprovalRecord records a single approval decision.
type ApprovalRecord struct {
	// PersonID is the approver's identity graph ID.
	PersonID identity.EntityID

	// Decision is the approval decision.
	Decision Decision

	// Timestamp is when the decision was made.
	Timestamp time.Time

	// Reason is an optional reason for the decision.
	Reason string

	// TokenID is the approval token used (for audit).
	TokenID string
}

// CanonicalString returns a deterministic representation.
func (r ApprovalRecord) CanonicalString() string {
	return fmt.Sprintf("person:%s|decision:%s|ts:%s|token:%s",
		r.PersonID, r.Decision, r.Timestamp.UTC().Format(time.RFC3339), r.TokenID)
}

// ApprovalState tracks the state of a multi-party approval.
type ApprovalState struct {
	// StateID uniquely identifies this approval state.
	StateID string

	// TargetType is what is being approved.
	TargetType TargetType

	// TargetID identifies the target (draft ID, intent ID, etc.).
	TargetID string

	// IntersectionID is the intersection policy governing this approval.
	// May be empty for single-party approvals.
	IntersectionID string

	// ActionClass is the type of action being approved.
	ActionClass intersection.ActionClass

	// RequiredApprovers are the approvers needed.
	RequiredApprovers []ApproverRef

	// Threshold is the minimum approvals needed.
	// If 0, all required approvers must approve.
	Threshold int

	// MaxAgeMinutes is the freshness window for approvals.
	MaxAgeMinutes int

	// Approvals are the recorded approval decisions.
	Approvals []ApprovalRecord

	// CreatedAt is when this approval state was created.
	CreatedAt time.Time

	// ExpiresAt is when this approval request expires.
	ExpiresAt time.Time

	// Version is incremented on each update.
	Version int

	// Hash is the SHA256 hash of the canonical string.
	Hash string
}

// NewApprovalState creates a new approval state.
func NewApprovalState(
	targetType TargetType,
	targetID string,
	intersectionID string,
	actionClass intersection.ActionClass,
	requiredApprovers []ApproverRef,
	threshold int,
	maxAgeMinutes int,
	createdAt time.Time,
) *ApprovalState {
	if threshold <= 0 {
		threshold = len(requiredApprovers)
	}
	if maxAgeMinutes <= 0 {
		maxAgeMinutes = 60 // 1 hour default
	}

	expiresAt := createdAt.Add(time.Duration(maxAgeMinutes) * time.Minute)

	s := &ApprovalState{
		TargetType:        targetType,
		TargetID:          targetID,
		IntersectionID:    intersectionID,
		ActionClass:       actionClass,
		RequiredApprovers: make([]ApproverRef, len(requiredApprovers)),
		Threshold:         threshold,
		MaxAgeMinutes:     maxAgeMinutes,
		Approvals:         []ApprovalRecord{},
		CreatedAt:         createdAt,
		ExpiresAt:         expiresAt,
		Version:           1,
	}

	// Copy and sort approvers for determinism
	copy(s.RequiredApprovers, requiredApprovers)
	s.sortApprovers()

	// Generate deterministic state ID
	s.StateID = s.computeStateID()

	s.ComputeHash()
	return s
}

// computeStateID generates a deterministic ID for the state.
func (s *ApprovalState) computeStateID() string {
	input := fmt.Sprintf("approval_state|%s|%s|%s|%s|%s",
		s.TargetType, s.TargetID, s.IntersectionID, s.ActionClass,
		s.CreatedAt.UTC().Format(time.RFC3339))
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:16]
}

// RecordApproval records an approval decision.
func (s *ApprovalState) RecordApproval(record ApprovalRecord) {
	// Check if this person already approved
	for i, existing := range s.Approvals {
		if existing.PersonID == record.PersonID {
			// Update existing record
			s.Approvals[i] = record
			s.sortApprovals()
			s.Version++
			s.ComputeHash()
			return
		}
	}

	s.Approvals = append(s.Approvals, record)
	s.sortApprovals()
	s.Version++
	s.ComputeHash()
}

// ComputeStatus computes the current status deterministically.
func (s *ApprovalState) ComputeStatus(now time.Time) Status {
	// Check expiry first
	if now.After(s.ExpiresAt) {
		return StatusExpired
	}

	// Check for any rejections
	for _, approval := range s.Approvals {
		if approval.Decision == DecisionRejected {
			return StatusRejected
		}
	}

	// Count valid approvals
	approvedCount := 0
	for _, approval := range s.Approvals {
		if approval.Decision == DecisionApproved {
			// Check if approval is still fresh
			approvalAge := now.Sub(approval.Timestamp)
			if approvalAge <= time.Duration(s.MaxAgeMinutes)*time.Minute {
				approvedCount++
			}
		}
	}

	// Check threshold
	if approvedCount >= s.Threshold {
		return StatusApproved
	}

	return StatusPending
}

// IsApproverRequired checks if a person is a required approver.
func (s *ApprovalState) IsApproverRequired(personID identity.EntityID) bool {
	for _, approver := range s.RequiredApprovers {
		if approver.PersonID == personID {
			return true
		}
	}
	return false
}

// HasApproved checks if a person has already approved.
func (s *ApprovalState) HasApproved(personID identity.EntityID) bool {
	for _, approval := range s.Approvals {
		if approval.PersonID == personID && approval.Decision == DecisionApproved {
			return true
		}
	}
	return false
}

// GetApproval returns the approval record for a person.
func (s *ApprovalState) GetApproval(personID identity.EntityID) *ApprovalRecord {
	for i := range s.Approvals {
		if s.Approvals[i].PersonID == personID {
			return &s.Approvals[i]
		}
	}
	return nil
}

// GetPendingApprovers returns approvers who haven't decided yet.
func (s *ApprovalState) GetPendingApprovers() []ApproverRef {
	decided := make(map[identity.EntityID]bool)
	for _, approval := range s.Approvals {
		decided[approval.PersonID] = true
	}

	var pending []ApproverRef
	for _, approver := range s.RequiredApprovers {
		if !decided[approver.PersonID] {
			pending = append(pending, approver)
		}
	}
	return pending
}

// CanonicalString returns a deterministic representation.
func (s *ApprovalState) CanonicalString() string {
	var sb strings.Builder

	sb.WriteString("approval_state|")
	sb.WriteString("id:")
	sb.WriteString(s.StateID)
	sb.WriteString("|target_type:")
	sb.WriteString(string(s.TargetType))
	sb.WriteString("|target_id:")
	sb.WriteString(s.TargetID)
	sb.WriteString("|intersection:")
	sb.WriteString(s.IntersectionID)
	sb.WriteString("|action:")
	sb.WriteString(string(s.ActionClass))
	sb.WriteString("|threshold:")
	sb.WriteString(fmt.Sprintf("%d", s.Threshold))
	sb.WriteString("|max_age:")
	sb.WriteString(fmt.Sprintf("%d", s.MaxAgeMinutes))
	sb.WriteString("|created:")
	sb.WriteString(s.CreatedAt.UTC().Format(time.RFC3339))
	sb.WriteString("|expires:")
	sb.WriteString(s.ExpiresAt.UTC().Format(time.RFC3339))
	sb.WriteString("|version:")
	sb.WriteString(fmt.Sprintf("%d", s.Version))

	// Approvers (already sorted)
	sb.WriteString("|approvers:[")
	for i, a := range s.RequiredApprovers {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(a.CanonicalString())
	}
	sb.WriteString("]")

	// Approvals (already sorted)
	sb.WriteString("|approvals:[")
	for i, a := range s.Approvals {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(a.CanonicalString())
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (s *ApprovalState) ComputeHash() string {
	canonical := s.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	s.Hash = hex.EncodeToString(hash[:])
	return s.Hash
}

// sortApprovers sorts approvers by PersonID for determinism.
func (s *ApprovalState) sortApprovers() {
	for i := 0; i < len(s.RequiredApprovers); i++ {
		for j := i + 1; j < len(s.RequiredApprovers); j++ {
			if string(s.RequiredApprovers[i].PersonID) > string(s.RequiredApprovers[j].PersonID) {
				s.RequiredApprovers[i], s.RequiredApprovers[j] = s.RequiredApprovers[j], s.RequiredApprovers[i]
			}
		}
	}
}

// sortApprovals sorts approvals by PersonID for determinism.
func (s *ApprovalState) sortApprovals() {
	for i := 0; i < len(s.Approvals); i++ {
		for j := i + 1; j < len(s.Approvals); j++ {
			if string(s.Approvals[i].PersonID) > string(s.Approvals[j].PersonID) {
				s.Approvals[i], s.Approvals[j] = s.Approvals[j], s.Approvals[i]
			}
		}
	}
}

// ApprovalStateSet holds multiple approval states.
type ApprovalStateSet struct {
	// States maps state ID to state.
	States map[string]*ApprovalState

	// Version is incremented on each update.
	Version int

	// Hash is the SHA256 hash of the set.
	Hash string
}

// NewApprovalStateSet creates an empty state set.
func NewApprovalStateSet() *ApprovalStateSet {
	s := &ApprovalStateSet{
		States:  make(map[string]*ApprovalState),
		Version: 1,
	}
	s.ComputeHash()
	return s
}

// Add adds a state to the set.
func (s *ApprovalStateSet) Add(state *ApprovalState) {
	s.States[state.StateID] = state
	s.Version++
	s.ComputeHash()
}

// Get returns a state by ID.
func (s *ApprovalStateSet) Get(stateID string) *ApprovalState {
	return s.States[stateID]
}

// GetByTarget returns a state by target type and ID.
func (s *ApprovalStateSet) GetByTarget(targetType TargetType, targetID string) *ApprovalState {
	for _, state := range s.States {
		if state.TargetType == targetType && state.TargetID == targetID {
			return state
		}
	}
	return nil
}

// List returns all states in deterministic order.
func (s *ApprovalStateSet) List() []*ApprovalState {
	ids := make([]string, 0, len(s.States))
	for id := range s.States {
		ids = append(ids, id)
	}
	bubbleSort(ids)

	result := make([]*ApprovalState, len(ids))
	for i, id := range ids {
		result[i] = s.States[id]
	}
	return result
}

// ListPending returns all pending states.
func (s *ApprovalStateSet) ListPending(now time.Time) []*ApprovalState {
	var result []*ApprovalState
	for _, state := range s.List() {
		if state.ComputeStatus(now) == StatusPending {
			result = append(result, state)
		}
	}
	return result
}

// ListForPerson returns all states where person is a required approver.
func (s *ApprovalStateSet) ListForPerson(personID identity.EntityID) []*ApprovalState {
	var result []*ApprovalState
	for _, state := range s.List() {
		if state.IsApproverRequired(personID) {
			result = append(result, state)
		}
	}
	return result
}

// ListPendingForPerson returns pending states for a person.
func (s *ApprovalStateSet) ListPendingForPerson(personID identity.EntityID, now time.Time) []*ApprovalState {
	var result []*ApprovalState
	for _, state := range s.List() {
		if state.IsApproverRequired(personID) && state.ComputeStatus(now) == StatusPending {
			// Check if person hasn't approved yet
			if !state.HasApproved(personID) {
				result = append(result, state)
			}
		}
	}
	return result
}

// Remove removes a state from the set.
func (s *ApprovalStateSet) Remove(stateID string) bool {
	if _, exists := s.States[stateID]; exists {
		delete(s.States, stateID)
		s.Version++
		s.ComputeHash()
		return true
	}
	return false
}

// PruneExpired removes all expired states.
func (s *ApprovalStateSet) PruneExpired(now time.Time) int {
	pruned := 0
	for id, state := range s.States {
		if state.ComputeStatus(now) == StatusExpired {
			delete(s.States, id)
			pruned++
		}
	}
	if pruned > 0 {
		s.Version++
		s.ComputeHash()
	}
	return pruned
}

// CanonicalString returns a deterministic representation.
func (s *ApprovalStateSet) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("approval_state_set|version:")
	sb.WriteString(fmt.Sprintf("%d", s.Version))
	sb.WriteString("|states:[")

	states := s.List()
	for i, state := range states {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(state.CanonicalString())
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (s *ApprovalStateSet) ComputeHash() string {
	canonical := s.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	s.Hash = hex.EncodeToString(hash[:])
	return s.Hash
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

// Stats holds approval flow statistics.
type Stats struct {
	TotalStates   int
	PendingCount  int
	ApprovedCount int
	RejectedCount int
	ExpiredCount  int
}

// GetStats returns statistics for the set.
func (s *ApprovalStateSet) GetStats(now time.Time) Stats {
	stats := Stats{
		TotalStates: len(s.States),
	}

	for _, state := range s.States {
		switch state.ComputeStatus(now) {
		case StatusPending:
			stats.PendingCount++
		case StatusApproved:
			stats.ApprovedCount++
		case StatusRejected:
			stats.RejectedCount++
		case StatusExpired:
			stats.ExpiredCount++
		}
	}

	return stats
}
