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

// AuthorizationProof records the proof of authorization for an action.
// This is attached to audit events for full traceability.
type AuthorizationProof struct {
	// ID uniquely identifies this proof.
	ID string

	// ActionID is the action being authorized.
	ActionID string

	// IntersectionID is the intersection providing authorization.
	IntersectionID string

	// ContractVersion is the version of the contract used for authorization.
	ContractVersion string

	// ScopesUsed lists the scopes that were checked and used.
	ScopesUsed []string

	// ScopesGranted lists all scopes granted in the contract.
	ScopesGranted []string

	// CeilingChecks records the results of ceiling validations.
	CeilingChecks []CeilingCheck

	// ModeCheck records the run mode validation.
	ModeCheck ModeCheck

	// Authorized indicates whether authorization was granted.
	Authorized bool

	// DenialReason explains why authorization was denied (if applicable).
	DenialReason string

	// Timestamp is when the authorization check was performed.
	Timestamp time.Time

	// TraceID links this proof to a distributed trace.
	TraceID string

	// ApprovedByHuman indicates if explicit human approval was provided.
	// v6: Required for Execute mode with write scopes.
	ApprovedByHuman bool

	// ApprovalArtifact records how approval was obtained.
	// Examples: "cli:--approve", "api:explicit_consent"
	// v6: Required when ApprovedByHuman is true.
	ApprovalArtifact string

	// EvaluatedAt is when authorization was evaluated (more precise than Timestamp).
	// v6: Used for temporal correlation with receipts.
	EvaluatedAt time.Time
}

// CeilingCheck records the result of a single ceiling validation.
type CeilingCheck struct {
	// CeilingType is the type of ceiling (e.g., "time_window", "duration").
	CeilingType string

	// CeilingValue is the configured ceiling value.
	CeilingValue string

	// CeilingUnit is the unit of the ceiling.
	CeilingUnit string

	// RequestedValue is what was requested.
	RequestedValue string

	// Passed indicates whether the check passed.
	Passed bool

	// Reason explains the check result.
	Reason string
}

// ModeCheck records the result of run mode validation.
type ModeCheck struct {
	// RequestedMode is the mode that was requested.
	RequestedMode string

	// Allowed indicates whether the mode is allowed.
	Allowed bool

	// Reason explains the check result.
	Reason string
}
