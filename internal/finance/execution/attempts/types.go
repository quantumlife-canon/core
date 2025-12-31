// Package attempts provides idempotency and replay defense primitives for v9.6 financial execution.
//
// CRITICAL: This package prevents duplicate payments and replays by:
// - Enforcing unique (envelope_id, attempt_id) pairs
// - Blocking retries of terminal attempts (settled/aborted/blocked/revoked/expired/simulated)
// - Enforcing one in-flight attempt per envelope at any time
// - Deriving deterministic idempotency keys for provider calls
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package attempts

import (
	"time"
)

// AttemptStatus represents the lifecycle state of an execution attempt.
type AttemptStatus string

const (
	// AttemptStatusStarted indicates the attempt has been initiated.
	AttemptStatusStarted AttemptStatus = "started"

	// AttemptStatusPrepared indicates the provider has validated the request.
	AttemptStatusPrepared AttemptStatus = "prepared"

	// AttemptStatusInvoked indicates the provider Execute call has been made.
	AttemptStatusInvoked AttemptStatus = "invoked"

	// Terminal states - no further transitions allowed.

	// AttemptStatusSettled indicates successful provider settlement.
	AttemptStatusSettled AttemptStatus = "settled"

	// AttemptStatusSimulated indicates mock provider execution (no real money moved).
	AttemptStatusSimulated AttemptStatus = "simulated"

	// AttemptStatusAborted indicates the attempt was aborted before provider call.
	AttemptStatusAborted AttemptStatus = "aborted"

	// AttemptStatusBlocked indicates the attempt was blocked by a gate or validation.
	AttemptStatusBlocked AttemptStatus = "blocked"

	// AttemptStatusRevoked indicates the envelope was revoked.
	AttemptStatusRevoked AttemptStatus = "revoked"

	// AttemptStatusExpired indicates the envelope or approval expired.
	AttemptStatusExpired AttemptStatus = "expired"

	// AttemptStatusFailed indicates a provider error occurred.
	AttemptStatusFailed AttemptStatus = "failed"
)

// IsTerminal returns true if the status is a terminal state.
func (s AttemptStatus) IsTerminal() bool {
	switch s {
	case AttemptStatusSettled,
		AttemptStatusSimulated,
		AttemptStatusAborted,
		AttemptStatusBlocked,
		AttemptStatusRevoked,
		AttemptStatusExpired,
		AttemptStatusFailed:
		return true
	default:
		return false
	}
}

// IsInFlight returns true if the status indicates an in-progress attempt.
func (s AttemptStatus) IsInFlight() bool {
	switch s {
	case AttemptStatusStarted,
		AttemptStatusPrepared,
		AttemptStatusInvoked:
		return true
	default:
		return false
	}
}

// AttemptRecord represents a single execution attempt in the ledger.
type AttemptRecord struct {
	// AttemptID uniquely identifies this attempt.
	AttemptID string

	// EnvelopeID identifies the envelope being executed.
	EnvelopeID string

	// ActionHash is the deterministic hash of the action specification.
	ActionHash string

	// IdempotencyKey is the derived key for provider deduplication.
	IdempotencyKey string

	// CircleID is the actor circle initiating the execution.
	CircleID string

	// IntersectionID is the intersection context (if applicable).
	IntersectionID string

	// TraceID links to the execution trace for audit.
	TraceID string

	// Status is the current lifecycle state.
	Status AttemptStatus

	// Provider identifies which connector was used (truelayer/mock).
	Provider string

	// ProviderRef is the provider's reference ID for the transaction.
	ProviderRef string

	// CreatedAt is when the attempt was started.
	CreatedAt time.Time

	// UpdatedAt is when the status was last updated.
	UpdatedAt time.Time

	// FinalizedAt is when the attempt reached terminal status.
	FinalizedAt time.Time

	// BlockedReason explains why the attempt was blocked (if applicable).
	BlockedReason string

	// MoneyMoved indicates if real money was transferred.
	MoneyMoved bool
}

// Errors for attempt ledger operations.
var (
	// ErrAttemptAlreadyExists is returned when creating a duplicate attempt.
	ErrAttemptAlreadyExists = attemptError("attempt already exists")

	// ErrAttemptNotFound is returned when an attempt cannot be found.
	ErrAttemptNotFound = attemptError("attempt not found")

	// ErrAttemptTerminal is returned when trying to modify a terminal attempt.
	ErrAttemptTerminal = attemptError("attempt is in terminal state")

	// ErrAttemptReplay is returned when detecting a replay of a terminal attempt.
	ErrAttemptReplay = attemptError("replay detected: attempt already finalized")

	// ErrAttemptInFlight is returned when another attempt for the same envelope is in progress.
	ErrAttemptInFlight = attemptError("another attempt is already in flight for this envelope")

	// ErrIdempotencyKeyConflict is returned when the idempotency key already exists.
	ErrIdempotencyKeyConflict = attemptError("idempotency key already used for this envelope")

	// ErrInvalidTransition is returned when an invalid status transition is attempted.
	ErrInvalidTransition = attemptError("invalid status transition")
)

type attemptError string

func (e attemptError) Error() string { return string(e) }
