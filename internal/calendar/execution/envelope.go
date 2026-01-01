// Package execution implements the calendar execution envelope and executor.
//
// CRITICAL: This is the FIRST real external write in QuantumLife.
// CRITICAL: Execution ONLY happens from approved drafts.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent.
//
// Reference: docs/ADR/ADR-0022-phase5-calendar-execution-boundary.md
package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
)

// EnvelopeStatus represents the status of an execution envelope.
type EnvelopeStatus string

const (
	// EnvelopeStatusPending means execution has not been attempted.
	EnvelopeStatusPending EnvelopeStatus = "pending"

	// EnvelopeStatusExecuted means execution completed successfully.
	EnvelopeStatusExecuted EnvelopeStatus = "executed"

	// EnvelopeStatusFailed means execution failed.
	EnvelopeStatusFailed EnvelopeStatus = "failed"

	// EnvelopeStatusBlocked means execution was blocked by policy/view mismatch.
	EnvelopeStatusBlocked EnvelopeStatus = "blocked"

	// EnvelopeStatusCancelled means the user cancelled before execution.
	EnvelopeStatusCancelled EnvelopeStatus = "cancelled"
)

// Envelope is the execution envelope for calendar operations.
// CRITICAL: This binds together all components needed for safe execution.
type Envelope struct {
	// EnvelopeID is the deterministic ID for this envelope.
	// Computed from canonical hash of contents.
	EnvelopeID string

	// DraftID references the approved draft that created this envelope.
	DraftID draft.DraftID

	// CircleID is the circle context for execution.
	CircleID identity.EntityID

	// IntersectionID is the intersection context for execution.
	IntersectionID identity.EntityID

	// Provider identifies the calendar provider (google, outlook, etc.).
	Provider string

	// CalendarID is the calendar to update.
	CalendarID string

	// EventID is the event to respond to.
	EventID string

	// Response is the calendar response action.
	Response draft.CalendarResponse

	// Message is the optional message for the response.
	Message string

	// ProposeNewTime indicates this is a time proposal.
	ProposeNewTime bool

	// ProposedStart is the proposed start time (if ProposeNewTime=true).
	ProposedStart *time.Time

	// ProposedEnd is the proposed end time (if ProposeNewTime=true).
	ProposedEnd *time.Time

	// PolicySnapshotHash binds this envelope to a specific policy state.
	// CRITICAL: Execution MUST fail if current policy hash differs.
	PolicySnapshotHash string

	// ViewSnapshotHash binds this envelope to a specific view of the calendar.
	// CRITICAL: Execution MUST fail if view has changed beyond staleness.
	ViewSnapshotHash string

	// ViewSnapshotAt is when the view snapshot was taken.
	ViewSnapshotAt time.Time

	// IdempotencyKey ensures at-most-once execution.
	IdempotencyKey string

	// TraceID links this envelope to the execution trace.
	TraceID string

	// Status is the current status of this envelope.
	Status EnvelopeStatus

	// CreatedAt is when this envelope was created.
	CreatedAt time.Time

	// ExecutedAt is when execution completed (if Status=executed).
	ExecutedAt *time.Time

	// ExecutionResult contains the result of execution.
	ExecutionResult *ExecutionResult
}

// ExecutionResult contains the result of executing an envelope.
type ExecutionResult struct {
	// Success indicates the external write succeeded.
	Success bool

	// ProviderResponseID is the provider's response identifier.
	ProviderResponseID string

	// ETag is the new etag/version after update.
	ETag string

	// Error contains error details if Success=false.
	Error string

	// BlockedReason explains why execution was blocked (if Status=blocked).
	BlockedReason string
}

// ComputeEnvelopeID computes a deterministic envelope ID.
func ComputeEnvelopeID(
	draftID draft.DraftID,
	circleID identity.EntityID,
	provider string,
	calendarID string,
	eventID string,
	response draft.CalendarResponse,
	policyHash string,
	viewHash string,
) string {
	canonical := fmt.Sprintf("envelope|%s|%s|%s|%s|%s|%s|%s|%s",
		draftID,
		circleID,
		provider,
		calendarID,
		eventID,
		response,
		policyHash,
		viewHash,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ComputeIdempotencyKey computes a deterministic idempotency key.
func ComputeIdempotencyKey(
	envelopeID string,
	traceID string,
) string {
	canonical := fmt.Sprintf("idem|%s|%s", envelopeID, traceID)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// NewEnvelopeFromDraft creates an execution envelope from an approved draft.
// CRITICAL: Only approved drafts can create envelopes.
func NewEnvelopeFromDraft(
	d draft.Draft,
	policySnapshotHash string,
	viewSnapshotHash string,
	viewSnapshotAt time.Time,
	traceID string,
	now time.Time,
) (*Envelope, error) {
	// Verify draft is approved
	if d.Status != draft.StatusApproved {
		return nil, fmt.Errorf("draft status must be approved, got %s", d.Status)
	}

	// Verify draft type is calendar
	if d.DraftType != draft.DraftTypeCalendarResponse {
		return nil, fmt.Errorf("draft type must be calendar_response, got %s", d.DraftType)
	}

	// Extract calendar content
	calContent, ok := d.Content.(draft.CalendarDraftContent)
	if !ok {
		return nil, fmt.Errorf("invalid draft content type")
	}

	// Compute envelope ID
	envelopeID := ComputeEnvelopeID(
		d.DraftID,
		d.CircleID,
		calContent.ProviderHint,
		calContent.CalendarID,
		calContent.EventID,
		calContent.Response,
		policySnapshotHash,
		viewSnapshotHash,
	)

	// Compute idempotency key
	idempotencyKey := ComputeIdempotencyKey(envelopeID, traceID)

	env := &Envelope{
		EnvelopeID:         envelopeID,
		DraftID:            d.DraftID,
		CircleID:           d.CircleID,
		IntersectionID:     d.IntersectionID,
		Provider:           calContent.ProviderHint,
		CalendarID:         calContent.CalendarID,
		EventID:            calContent.EventID,
		Response:           calContent.Response,
		Message:            calContent.Message,
		ProposeNewTime:     false, // TODO: Support from draft content
		PolicySnapshotHash: policySnapshotHash,
		ViewSnapshotHash:   viewSnapshotHash,
		ViewSnapshotAt:     viewSnapshotAt,
		IdempotencyKey:     idempotencyKey,
		TraceID:            traceID,
		Status:             EnvelopeStatusPending,
		CreatedAt:          now,
	}

	return env, nil
}

// CanonicalString returns a canonical string representation for hashing.
func (e *Envelope) CanonicalString() string {
	proposeStr := "false"
	if e.ProposeNewTime {
		proposeStr = fmt.Sprintf("true|%s|%s",
			e.ProposedStart.UTC().Format(time.RFC3339),
			e.ProposedEnd.UTC().Format(time.RFC3339),
		)
	}

	return fmt.Sprintf("envelope|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		e.EnvelopeID,
		e.DraftID,
		e.CircleID,
		e.Provider,
		e.CalendarID,
		e.EventID,
		e.Response,
		e.Message,
		proposeStr,
		e.PolicySnapshotHash,
		e.ViewSnapshotHash,
	)
}

// Validate validates the envelope for execution.
func (e *Envelope) Validate() error {
	if e.EnvelopeID == "" {
		return ErrMissingEnvelopeID
	}
	if e.DraftID == "" {
		return ErrMissingDraftID
	}
	if e.CircleID == "" {
		return ErrMissingCircleID
	}
	if e.Provider == "" {
		return ErrMissingProvider
	}
	if e.CalendarID == "" {
		return ErrMissingCalendarID
	}
	if e.EventID == "" {
		return ErrMissingEventID
	}
	if e.Response == "" {
		return ErrMissingResponse
	}
	if e.PolicySnapshotHash == "" {
		return ErrMissingPolicyHash
	}
	if e.ViewSnapshotHash == "" {
		return ErrMissingViewHash
	}
	if e.IdempotencyKey == "" {
		return ErrMissingIdempotencyKey
	}
	if e.TraceID == "" {
		return ErrMissingTraceID
	}
	if e.ProposeNewTime && (e.ProposedStart == nil || e.ProposedEnd == nil) {
		return ErrMissingProposedTimes
	}
	return nil
}

// Validation errors.
var (
	ErrMissingEnvelopeID     = envelopeError("missing envelope_id")
	ErrMissingDraftID        = envelopeError("missing draft_id")
	ErrMissingCircleID       = envelopeError("missing circle_id")
	ErrMissingProvider       = envelopeError("missing provider")
	ErrMissingCalendarID     = envelopeError("missing calendar_id")
	ErrMissingEventID        = envelopeError("missing event_id")
	ErrMissingResponse       = envelopeError("missing response")
	ErrMissingPolicyHash     = envelopeError("missing policy_snapshot_hash")
	ErrMissingViewHash       = envelopeError("missing view_snapshot_hash")
	ErrMissingIdempotencyKey = envelopeError("missing idempotency_key")
	ErrMissingTraceID        = envelopeError("missing trace_id")
	ErrMissingProposedTimes  = envelopeError("propose_new_time requires proposed_start and proposed_end")
)

type envelopeError string

func (e envelopeError) Error() string { return string(e) }
