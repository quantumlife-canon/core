package primitives

import (
	"time"
)

// Proposal represents a request to change terms, scope, or authority within an intersection.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Proposal)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §5.4 Proposal → Commitment Lifecycle
type Proposal struct {
	// ID uniquely identifies this proposal.
	ID string

	// Version tracks the schema version of this proposal.
	Version int

	// CreatedAt is the timestamp when this proposal was created.
	CreatedAt time.Time

	// Issuer identifies the circle that created this proposal.
	Issuer string

	// IntersectionID identifies the intersection this proposal targets.
	IntersectionID string

	// IntentID links this proposal to the originating intent.
	IntentID string

	// ScopesRequested lists the authority scopes being requested.
	ScopesRequested []string

	// Terms contains the proposed contract terms.
	Terms map[string]string

	// ExpiresAt is when this proposal expires if not accepted.
	ExpiresAt time.Time

	// State indicates the current state of the proposal.
	// Valid states: draft, submitted, counterproposal, accepted, rejected
	State string
}

// Validate checks that the proposal has all required fields.
func (p *Proposal) Validate() error {
	if p.ID == "" {
		return ErrMissingID
	}
	if p.Issuer == "" {
		return ErrMissingIssuer
	}
	if p.CreatedAt.IsZero() {
		return ErrMissingTimestamp
	}
	if p.IntersectionID == "" {
		return ErrMissingIntersectionID
	}
	return nil
}
