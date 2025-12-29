package authority

import (
	"time"
)

// Grant represents an authority grant.
// Mirrors circle.AuthorityGrant for read-only access.
type Grant struct {
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
	Type  string
	Value string
	Unit  string
}

// ValidationRequest contains parameters for authority validation.
type ValidationRequest struct {
	GrantID        string
	CircleID       string
	IntersectionID string
	Operation      Operation
	RequiredScopes []string
}

// Operation represents an operation being validated.
type Operation struct {
	Type       string
	Amount     *float64          // For spend ceilings
	Timestamp  time.Time         // For time window ceilings
	Location   string            // For geography ceilings
	Parameters map[string]string // Additional parameters
}

// ValidationResult contains the result of authority validation.
type ValidationResult struct {
	Authorized bool
	Reason     string
	GrantID    string
	ExpiresAt  time.Time
}
