package circle

import (
	"time"
)

// Circle represents a sovereign agent boundary.
type Circle struct {
	ID        string
	TenantID  string
	State     State
	CreatedAt time.Time
	UpdatedAt time.Time
}

// State represents the lifecycle state of a circle.
type State string

const (
	StateActive     State = "active"
	StateSuspended  State = "suspended"
	StateTerminated State = "terminated"
)

// Identity represents a circle's self-sovereign identity.
type Identity struct {
	CircleID  string
	PublicKey []byte
	Algorithm string
	CreatedAt time.Time
}

// Policy represents a circle's declared boundaries and preferences.
type Policy struct {
	CircleID    string
	Version     int
	Boundaries  []Boundary
	Preferences map[string]string
	UpdatedAt   time.Time
}

// Boundary represents a declared policy boundary.
type Boundary struct {
	Type       string
	Constraint string
	Enabled    bool
}

// AuthorityGrant represents an explicit delegation of capability.
type AuthorityGrant struct {
	ID             string
	CircleID       string
	IntersectionID string
	Scopes         []string
	Ceilings       []Ceiling
	GrantedAt      time.Time
	ExpiresAt      time.Time
	RevokedAt      *time.Time
}

// Ceiling represents a limit on authority.
type Ceiling struct {
	Type  string // e.g., "spend", "time_window", "geography"
	Value string
}

// Operation represents an operation being evaluated against policy.
type Operation struct {
	Type       string
	Scopes     []string
	Parameters map[string]string
}

// Decision represents a policy evaluation result.
type Decision struct {
	Allowed bool
	Reason  string
}

// CreateRequest contains parameters for creating a new circle.
type CreateRequest struct {
	TenantID string
	Identity ClaimRequest
	Policy   *Policy
}

// ClaimRequest contains parameters for claiming an identity.
type ClaimRequest struct {
	TenantID  string
	Algorithm string
}

// GrantRequest contains parameters for creating an authority grant.
type GrantRequest struct {
	CircleID       string
	IntersectionID string
	Scopes         []string
	Ceilings       []Ceiling
	ExpiresAt      time.Time
}
