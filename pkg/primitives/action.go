package primitives

import (
	"time"
)

// Action represents an executed operation within granted authority.
// Actions are always auditable and occur only after commitment formation.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Action)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
type Action struct {
	// ID uniquely identifies this action.
	ID string

	// Version tracks the schema version of this action.
	Version int

	// CreatedAt is the timestamp when this action was created.
	CreatedAt time.Time

	// Issuer identifies the circle that initiated this action.
	Issuer string

	// CommitmentID links this action to its governing commitment.
	CommitmentID string

	// IntersectionID identifies the intersection governing this action.
	IntersectionID string

	// Type identifies the kind of action being executed.
	Type string

	// Parameters contains action-specific parameters.
	Parameters map[string]string

	// State indicates the current state of the action.
	// Valid states: pending, executing, paused, aborted, completed
	State string

	// StartedAt is when execution began (nil if not started).
	StartedAt *time.Time

	// CompletedAt is when execution completed (nil if not completed).
	CompletedAt *time.Time
}

// Validate checks that the action has all required fields.
func (a *Action) Validate() error {
	if a.ID == "" {
		return ErrMissingID
	}
	if a.Issuer == "" {
		return ErrMissingIssuer
	}
	if a.CreatedAt.IsZero() {
		return ErrMissingTimestamp
	}
	if a.CommitmentID == "" {
		return ErrMissingCommitmentID
	}
	if a.Type == "" {
		return ErrMissingActionType
	}
	return nil
}

// IsPausable returns true if the action can be paused in its current state.
func (a *Action) IsPausable() bool {
	return a.State == "executing"
}

// IsAbortable returns true if the action can be aborted in its current state.
func (a *Action) IsAbortable() bool {
	return a.State == "pending" || a.State == "executing" || a.State == "paused"
}
