package primitives

import (
	"time"
)

// Commitment represents a binding agreement to perform an action under stated conditions.
// Once formed, a commitment is immutable.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Commitment)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §4.3 The Commitment Boundary
type Commitment struct {
	// ID uniquely identifies this commitment.
	ID string

	// Version tracks the schema version of this commitment.
	Version int

	// CreatedAt is the timestamp when this commitment was formed.
	CreatedAt time.Time

	// Issuer identifies the circle that made this commitment.
	Issuer string

	// ProposalID links this commitment to the accepted proposal.
	ProposalID string

	// IntersectionID identifies the intersection governing this commitment.
	IntersectionID string

	// Parties lists all circles bound by this commitment.
	Parties []string

	// ActionSpec defines the action to be executed.
	ActionSpec ActionSpec

	// Conditions lists conditions that must be met for execution.
	Conditions []string

	// ExpiresAt is when this commitment expires if not executed.
	ExpiresAt time.Time
}

// ActionSpec defines the specification for an action to be executed.
type ActionSpec struct {
	// Type identifies the kind of action.
	Type string

	// Parameters contains action-specific parameters.
	Parameters map[string]string

	// RequiredScopes lists authority scopes required for execution.
	RequiredScopes []string
}

// Validate checks that the commitment has all required fields.
func (c *Commitment) Validate() error {
	if c.ID == "" {
		return ErrMissingID
	}
	if c.Issuer == "" {
		return ErrMissingIssuer
	}
	if c.CreatedAt.IsZero() {
		return ErrMissingTimestamp
	}
	if c.ProposalID == "" {
		return ErrMissingProposalID
	}
	if len(c.Parties) == 0 {
		return ErrMissingParties
	}
	return nil
}
