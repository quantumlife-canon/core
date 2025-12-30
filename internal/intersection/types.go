package intersection

import (
	"fmt"
	"time"
)

// Intersection represents a shared domain between circles.
type Intersection struct {
	ID        string
	TenantID  string
	State     State
	Version   string // Semantic version
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SemVer represents a semantic version with Major.Minor.Patch.
type SemVer struct {
	Major int
	Minor int
	Patch int
}

// String returns the string representation of the version.
func (v SemVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Parse parses a version string into a SemVer.
func ParseSemVer(version string) (SemVer, error) {
	var v SemVer
	_, err := fmt.Sscanf(version, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid version format: %s", version)
	}
	return v, nil
}

// BumpMajor returns a new version with major incremented.
func (v SemVer) BumpMajor() SemVer {
	return SemVer{Major: v.Major + 1, Minor: 0, Patch: 0}
}

// BumpMinor returns a new version with minor incremented.
func (v SemVer) BumpMinor() SemVer {
	return SemVer{Major: v.Major, Minor: v.Minor + 1, Patch: 0}
}

// BumpPatch returns a new version with patch incremented.
func (v SemVer) BumpPatch() SemVer {
	return SemVer{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// Amendment represents a proposed change to an intersection contract.
type Amendment struct {
	ID             string
	IntersectionID string
	ProposerID     string
	Reason         string

	// Changes
	ScopeAdditions []Scope
	ScopeRemovals  []string // Scope names to remove
	CeilingChanges []Ceiling
	DurationExtend *time.Duration // Optional duration extension

	// Approval tracking
	Approvals  map[string]bool   // circleID -> approved
	Rejections map[string]string // circleID -> rejection reason

	// Version info
	FromVersion string
	ToVersion   string

	// Timestamps
	CreatedAt   time.Time
	FinalizedAt *time.Time
	State       AmendmentState
}

// AmendmentState represents the state of an amendment.
type AmendmentState string

const (
	AmendmentStatePending   AmendmentState = "pending"
	AmendmentStateApproved  AmendmentState = "approved"
	AmendmentStateRejected  AmendmentState = "rejected"
	AmendmentStateApplied   AmendmentState = "applied"
	AmendmentStateCancelled AmendmentState = "cancelled"
)

// State represents the lifecycle state of an intersection.
type State string

const (
	StateProposed    State = "proposed"
	StateNegotiating State = "negotiating"
	StateActive      State = "active"
	StateAmending    State = "amending"
	StateDissolved   State = "dissolved"
	StateRejected    State = "rejected"
)

// Contract represents the versioned agreement between parties.
type Contract struct {
	IntersectionID  string
	Version         string
	Parties         []Party
	Scopes          []Scope
	Ceilings        []Ceiling
	Governance      Governance
	ApprovalPolicy  ApprovalPolicy // v7: Multi-party approval requirements
	CreatedAt       time.Time
	PreviousVersion string
}

// ApprovalPolicy defines multi-party approval requirements for execute-mode writes.
// This is intersection-scoped - no global policies allowed.
//
// CRITICAL: Changing ApprovalPolicy bumps contract MINOR version.
//
// Reference: v7 Multi-party approval governance
type ApprovalPolicy struct {
	// Mode defines the approval mode: "single" or "multi"
	// "single" - standard v6 approval (--approve flag sufficient)
	// "multi" - requires approvals from multiple circles
	Mode string

	// RequiredApprovers lists specific circle IDs that MUST approve.
	// If empty and Mode="multi", any circles in the intersection can approve.
	RequiredApprovers []string

	// Threshold is the minimum number of approvals required.
	// Must be >= 1. For "single" mode, this is always 1.
	// For "multi" mode, must be <= Total.
	Threshold int

	// Total is the total number of potential approvers.
	// If 0, derived from len(RequiredApprovers) or contract parties count.
	Total int

	// ExpirySeconds defines how long an approval artifact is valid.
	// After expiry, the approval cannot be used for execution.
	// Default: 3600 (1 hour)
	ExpirySeconds int

	// AppliesToScopes lists which scopes require this policy.
	// If empty, applies to all write scopes (e.g., ["calendar:write"]).
	AppliesToScopes []string
}

// ApprovalPolicyMode constants.
const (
	ApprovalModeSingle = "single"
	ApprovalModeMulti  = "multi"
)

// DefaultApprovalPolicy returns the default single-approval policy.
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		Mode:          ApprovalModeSingle,
		Threshold:     1,
		ExpirySeconds: 3600,
	}
}

// IsMultiApproval returns true if multi-party approval is required.
func (p ApprovalPolicy) IsMultiApproval() bool {
	return p.Mode == ApprovalModeMulti && p.Threshold > 1
}

// AppliesToScope checks if this policy applies to the given scope.
func (p ApprovalPolicy) AppliesToScope(scope string) bool {
	if len(p.AppliesToScopes) == 0 {
		// Default: apply to all write scopes
		return isWriteScope(scope)
	}
	for _, s := range p.AppliesToScopes {
		if s == scope {
			return true
		}
	}
	return false
}

// isWriteScope checks if a scope is a write scope.
func isWriteScope(scope string) bool {
	// Write scopes end with :write or :execute
	return len(scope) > 6 && (scope[len(scope)-6:] == ":write" || scope[len(scope)-8:] == ":execute")
}

// Validate checks that the approval policy is valid.
func (p ApprovalPolicy) Validate() error {
	switch p.Mode {
	case ApprovalModeSingle:
		// Single mode: threshold must be 1
		if p.Threshold != 0 && p.Threshold != 1 {
			return fmt.Errorf("single mode requires threshold=1, got %d", p.Threshold)
		}
	case ApprovalModeMulti:
		// Multi mode: threshold must be >= 1
		if p.Threshold < 1 {
			return fmt.Errorf("multi mode requires threshold >= 1, got %d", p.Threshold)
		}
		// If Total is specified, threshold must be <= Total
		if p.Total > 0 && p.Threshold > p.Total {
			return fmt.Errorf("threshold (%d) cannot exceed total (%d)", p.Threshold, p.Total)
		}
		// If RequiredApprovers specified, threshold cannot exceed count
		if len(p.RequiredApprovers) > 0 && p.Threshold > len(p.RequiredApprovers) {
			return fmt.Errorf("threshold (%d) cannot exceed required approvers count (%d)",
				p.Threshold, len(p.RequiredApprovers))
		}
	case "":
		// Empty mode defaults to single
	default:
		return fmt.Errorf("invalid approval mode: %s", p.Mode)
	}

	if p.ExpirySeconds < 0 {
		return fmt.Errorf("expiry seconds cannot be negative: %d", p.ExpirySeconds)
	}

	return nil
}

// Party represents a circle's participation in an intersection.
type Party struct {
	CircleID      string
	PartyType     string // e.g., "initiator", "acceptor", "observer"
	JoinedAt      time.Time
	GrantedScopes []string
}

// Scope represents a capability granted within the intersection.
type Scope struct {
	Name        string
	Description string
	ReadWrite   string // "read", "write", "execute", "delegate"
}

// Ceiling represents a limit within the intersection.
type Ceiling struct {
	Type  string
	Value string
	Unit  string
}

// Governance defines rules for changing the contract.
type Governance struct {
	AmendmentRequires string // "all_parties", "majority", etc.
	DissolutionPolicy string
}

// InviteTokenRef references an invite token stored elsewhere.
// The actual token data is in pkg/primitives.InviteToken.
type InviteTokenRef struct {
	TokenID        string
	IssuerCircleID string
	AcceptedAt     *time.Time
	AcceptorID     string
}

// ContractTemplate contains proposed contract terms.
type ContractTemplate struct {
	Scopes     []Scope
	Ceilings   []Ceiling
	Governance Governance
}

// Message represents a message within an intersection channel.
type Message struct {
	ID             string
	IntersectionID string
	SenderCircleID string
	Type           string
	Payload        []byte
	Timestamp      time.Time
}

// CreateRequest contains parameters for creating an intersection.
type CreateRequest struct {
	TenantID    string
	InitiatorID string
	AcceptorID  string
	Contract    Contract
	InviteToken string
}

// AmendRequest contains parameters for amending an intersection.
type AmendRequest struct {
	IntersectionID string
	ProposerID     string
	NewContract    Contract
	Reason         string
}

// InviteRequest contains parameters for creating an invite.
type InviteRequest struct {
	IssuerCircleID string
	TargetCircleID string // Optional: specific target, or "" for open invite
	ProposedName   string
	Template       ContractTemplate
	ExpiresIn      time.Duration
}

// AcceptInviteRequest contains parameters for accepting an invite.
type AcceptInviteRequest struct {
	TokenID        string
	AcceptorID     string
	AcceptorTenant string
}

// IntersectionSummary provides a summary of an intersection for display.
type IntersectionSummary struct {
	ID             string
	Name           string
	Version        string
	State          State
	PartyIDs       []string
	ScopeNames     []string
	CeilingSummary string
	CreatedAt      time.Time
}
