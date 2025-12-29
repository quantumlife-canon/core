package primitives

import "errors"

// Validation errors for primitives.
// These errors are returned by Validate() methods when required fields are missing.
var (
	// ErrMissingID is returned when a primitive is missing its ID.
	ErrMissingID = errors.New("missing required field: id")

	// ErrMissingIssuer is returned when a primitive is missing its issuer.
	ErrMissingIssuer = errors.New("missing required field: issuer")

	// ErrMissingTimestamp is returned when a primitive is missing its timestamp.
	ErrMissingTimestamp = errors.New("missing required field: created_at")

	// ErrMissingIntersectionID is returned when a primitive is missing its intersection ID.
	ErrMissingIntersectionID = errors.New("missing required field: intersection_id")

	// ErrMissingProposalID is returned when a commitment is missing its proposal ID.
	ErrMissingProposalID = errors.New("missing required field: proposal_id")

	// ErrMissingParties is returned when a commitment has no parties.
	ErrMissingParties = errors.New("missing required field: parties")

	// ErrMissingCommitmentID is returned when an action is missing its commitment ID.
	ErrMissingCommitmentID = errors.New("missing required field: commitment_id")

	// ErrMissingActionType is returned when an action is missing its type.
	ErrMissingActionType = errors.New("missing required field: type")

	// ErrMissingActionID is returned when a settlement is missing its action ID.
	ErrMissingActionID = errors.New("missing required field: action_id")
)
