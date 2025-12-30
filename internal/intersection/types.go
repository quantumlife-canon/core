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
	CreatedAt       time.Time
	PreviousVersion string
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
