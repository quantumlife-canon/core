package intersection

import (
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

// InviteToken represents a signed invitation to create an intersection.
type InviteToken struct {
	ID                 string
	IssuerCircleID     string
	Template           ContractTemplate
	ScopesOffered      []string
	ExpiresAt          time.Time
	Signature          []byte
	SignatureAlgorithm string
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
	Template       ContractTemplate
	ExpiresIn      time.Duration
}
