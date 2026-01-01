// Package write provides calendar write connector interfaces and implementations.
//
// CRITICAL: This is the first REAL external write in QuantumLife.
// CRITICAL: Must be "boringly safe" - explicit approval, full audit.
// CRITICAL: No auto-retries, no background execution.
//
// Reference: docs/ADR/ADR-0022-phase5-calendar-execution-boundary.md
package write

import (
	"context"
	"time"
)

// ResponseStatus represents the calendar event response status.
type ResponseStatus string

const (
	ResponseAccepted  ResponseStatus = "accepted"
	ResponseDeclined  ResponseStatus = "declined"
	ResponseTentative ResponseStatus = "tentative"
)

// RespondInput contains the input for responding to a calendar event.
type RespondInput struct {
	// Provider identifies the calendar provider (google, outlook, etc.).
	Provider string

	// CalendarID is the calendar containing the event.
	CalendarID string

	// EventID is the event to respond to.
	EventID string

	// ResponseStatus is the response (accepted, declined, tentative).
	ResponseStatus ResponseStatus

	// Message is an optional message to include with the response.
	Message string

	// ProposeNewTime indicates this is a counter-proposal.
	// If true, the event is NOT modified; only a comment/note is added.
	ProposeNewTime bool

	// ProposedStart is the proposed new start time (only if ProposeNewTime=true).
	ProposedStart *time.Time

	// ProposedEnd is the proposed new end time (only if ProposeNewTime=true).
	ProposedEnd *time.Time

	// IdempotencyKey is used to prevent duplicate writes.
	IdempotencyKey string

	// TraceID links this operation to the execution trace.
	TraceID string
}

// RespondReceipt contains the result of a calendar response operation.
type RespondReceipt struct {
	// Success indicates the operation succeeded.
	Success bool

	// EventID is the event that was responded to.
	EventID string

	// UpdatedAt is when the event was updated.
	UpdatedAt time.Time

	// ETag is the new etag/version after update (if available).
	ETag string

	// ProviderResponseID is the provider's response identifier.
	ProviderResponseID string

	// Error contains error details if Success=false.
	Error string

	// IdempotencyKey echoes back the key used.
	IdempotencyKey string
}

// Writer defines the interface for calendar write operations.
type Writer interface {
	// RespondToEvent responds to a calendar event invitation.
	//
	// CRITICAL: This performs a REAL external write.
	// CRITICAL: Must be idempotent - same IdempotencyKey returns same result.
	// CRITICAL: No auto-retries on failure.
	RespondToEvent(ctx context.Context, input RespondInput) (RespondReceipt, error)

	// ProviderID returns the provider identifier.
	ProviderID() string

	// IsSandbox returns true if this is a sandbox/test provider.
	IsSandbox() bool
}

// ValidateRespondInput validates the input for RespondToEvent.
func ValidateRespondInput(input RespondInput) error {
	if input.Provider == "" {
		return ErrMissingProvider
	}
	if input.CalendarID == "" {
		return ErrMissingCalendarID
	}
	if input.EventID == "" {
		return ErrMissingEventID
	}
	if input.ResponseStatus == "" {
		return ErrMissingResponseStatus
	}
	if input.ResponseStatus != ResponseAccepted &&
		input.ResponseStatus != ResponseDeclined &&
		input.ResponseStatus != ResponseTentative {
		return ErrInvalidResponseStatus
	}
	if input.ProposeNewTime {
		if input.ProposedStart == nil || input.ProposedEnd == nil {
			return ErrMissingProposedTimes
		}
		if input.ProposedEnd.Before(*input.ProposedStart) {
			return ErrInvalidProposedTimes
		}
	}
	if input.IdempotencyKey == "" {
		return ErrMissingIdempotencyKey
	}
	return nil
}

// Validation errors.
var (
	ErrMissingProvider       = writeError("missing provider")
	ErrMissingCalendarID     = writeError("missing calendar_id")
	ErrMissingEventID        = writeError("missing event_id")
	ErrMissingResponseStatus = writeError("missing response_status")
	ErrInvalidResponseStatus = writeError("invalid response_status")
	ErrMissingProposedTimes  = writeError("propose_new_time requires proposed_start and proposed_end")
	ErrInvalidProposedTimes  = writeError("proposed_end must be after proposed_start")
	ErrMissingIdempotencyKey = writeError("missing idempotency_key")
)

type writeError string

func (e writeError) Error() string { return string(e) }
