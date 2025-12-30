package primitives

import (
	"time"
)

// Settlement represents the completion and confirmation of an action.
// Settlement is atomic — complete or not at all. No partial settlements.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Settlement)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §5.5 Action → Settlement Lifecycle
type Settlement struct {
	// ID uniquely identifies this settlement.
	ID string

	// Version tracks the schema version of this settlement.
	Version int

	// CreatedAt is the timestamp when this settlement was created.
	CreatedAt time.Time

	// Issuer identifies the circle that confirmed this settlement.
	Issuer string

	// ActionID links this settlement to the completed action.
	ActionID string

	// CommitmentID links this settlement to the governing commitment.
	CommitmentID string

	// IntersectionID identifies the intersection governing this settlement.
	IntersectionID string

	// Outcome describes the result of the action.
	Outcome Outcome

	// State indicates the current state of the settlement.
	// Valid states: pending, settled, disputed, resolved
	State string

	// SettledAt is when the settlement was confirmed.
	SettledAt *time.Time
}

// Outcome represents the result of an action execution.
type Outcome struct {
	// Success indicates whether the action completed successfully.
	Success bool

	// ResultCode is a machine-readable result identifier.
	ResultCode string

	// ResultData contains action-specific result data.
	ResultData map[string]string

	// ErrorMessage contains error details if Success is false.
	ErrorMessage string
}

// Validate checks that the settlement has all required fields.
func (s *Settlement) Validate() error {
	if s.ID == "" {
		return ErrMissingID
	}
	if s.Issuer == "" {
		return ErrMissingIssuer
	}
	if s.CreatedAt.IsZero() {
		return ErrMissingTimestamp
	}
	if s.ActionID == "" {
		return ErrMissingActionID
	}
	if s.CommitmentID == "" {
		return ErrMissingCommitmentID
	}
	return nil
}

// SettlementStatus represents the status of a settlement.
type SettlementStatus string

const (
	// SettlementStatusProposed means the settlement has been proposed but not executed.
	SettlementStatusProposed SettlementStatus = "proposed"

	// SettlementStatusSimulated means the settlement was simulated (no external writes).
	SettlementStatusSimulated SettlementStatus = "simulated"

	// SettlementStatusSettled means the settlement has been completed.
	SettlementStatusSettled SettlementStatus = "settled"

	// SettlementStatusDisputed means the settlement is under dispute.
	SettlementStatusDisputed SettlementStatus = "disputed"
)

// SimulatedSettlement represents a settlement from a simulated execution.
type SimulatedSettlement struct {
	Settlement

	// Status indicates the settlement status.
	Status SettlementStatus

	// SimulatedAt is when the simulation occurred.
	SimulatedAt time.Time

	// AuthorizationProofID links to the authorization proof.
	AuthorizationProofID string

	// ProposedPayload contains what would have been sent to external service.
	ProposedPayload map[string]string

	// Message describes the simulation result.
	Message string
}
