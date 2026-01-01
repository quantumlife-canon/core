// Package execexecutor adapts ExecutionIntents to boundary executors.
//
// This package bridges the execution routing layer (execrouter) with the
// existing boundary executors (Phase 5 calendar, Phase 7 email).
//
// CRITICAL: This is NOT an external write path itself.
// CRITICAL: All writes flow through the boundary executors.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent via boundary executor idempotency.
//
// Reference: Phase 10 - Approved Draft → Execution Routing
package execexecutor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	emailexec "quantumlife/internal/email/execution"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/execintent"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

// ExecutionOutcome represents the result of intent execution.
type ExecutionOutcome struct {
	// IntentID is the executed intent's ID.
	IntentID execintent.IntentID

	// Success indicates the execution succeeded.
	Success bool

	// Blocked indicates execution was blocked (policy/view mismatch).
	Blocked bool

	// BlockedReason explains why execution was blocked.
	BlockedReason string

	// Error contains error details if Success=false and Blocked=false.
	Error string

	// ProviderResponseID is the provider's response identifier.
	ProviderResponseID string

	// ExecutedAt is when execution completed.
	ExecutedAt time.Time

	// EnvelopeID is the ID of the envelope that was executed.
	EnvelopeID string
}

// EmailExecutor is the interface for email boundary execution.
type EmailExecutor interface {
	Execute(ctx context.Context, envelope emailexec.Envelope) (*emailexec.Envelope, error)
}

// CalendarExecutor is the interface for calendar boundary execution.
type CalendarExecutor interface {
	Execute(ctx context.Context, envelope *calexec.Envelope) calexec.ExecuteResult
}

// Executor adapts ExecutionIntents to boundary executors.
//
// CRITICAL: Routes intents to the correct boundary executor.
// CRITICAL: Does not perform any external writes itself.
type Executor struct {
	emailExecutor    EmailExecutor
	calendarExecutor CalendarExecutor
	clock            clock.Clock
	emitter          events.Emitter
}

// NewExecutor creates a new execution adapter.
func NewExecutor(clk clock.Clock, emitter events.Emitter) *Executor {
	return &Executor{
		clock:   clk,
		emitter: emitter,
	}
}

// WithEmailExecutor sets the email boundary executor.
func (e *Executor) WithEmailExecutor(exec EmailExecutor) *Executor {
	e.emailExecutor = exec
	return e
}

// WithCalendarExecutor sets the calendar boundary executor.
func (e *Executor) WithCalendarExecutor(exec CalendarExecutor) *Executor {
	e.calendarExecutor = exec
	return e
}

// ExecuteIntent routes an ExecutionIntent to the appropriate boundary executor.
//
// CRITICAL: This is the single entry point for intent execution.
// CRITICAL: Validates intent before routing.
// CRITICAL: All external writes flow through boundary executors.
func (e *Executor) ExecuteIntent(ctx context.Context, intent *execintent.ExecutionIntent, traceID string) ExecutionOutcome {
	now := e.clock.Now()

	// Emit execution requested event
	e.emitEvent(events.Phase10ExecutionRequested, intent, "")

	// Validate the intent
	if err := intent.Validate(); err != nil {
		e.emitEvent(events.Phase10ExecutionBlocked, intent, fmt.Sprintf("validation failed: %v", err))
		return ExecutionOutcome{
			IntentID:      intent.IntentID,
			Success:       false,
			Blocked:       true,
			BlockedReason: fmt.Sprintf("validation failed: %v", err),
			ExecutedAt:    now,
		}
	}

	// Route based on action type
	switch intent.Action {
	case execintent.ActionEmailSend:
		return e.executeEmail(ctx, intent, traceID, now)

	case execintent.ActionCalendarRespond:
		return e.executeCalendar(ctx, intent, traceID, now)

	default:
		e.emitEvent(events.Phase10ExecutionBlocked, intent, fmt.Sprintf("unknown action: %s", intent.Action))
		return ExecutionOutcome{
			IntentID:      intent.IntentID,
			Success:       false,
			Blocked:       true,
			BlockedReason: fmt.Sprintf("unknown action: %s", intent.Action),
			ExecutedAt:    now,
		}
	}
}

// executeEmail routes an email intent to the email boundary executor.
func (e *Executor) executeEmail(ctx context.Context, intent *execintent.ExecutionIntent, traceID string, now time.Time) ExecutionOutcome {
	// Check executor is configured
	if e.emailExecutor == nil {
		e.emitEvent(events.Phase10ExecutionBlocked, intent, "email executor not configured")
		return ExecutionOutcome{
			IntentID:      intent.IntentID,
			Success:       false,
			Blocked:       true,
			BlockedReason: "email executor not configured",
			ExecutedAt:    now,
		}
	}

	// Build email envelope from intent
	envelope := e.buildEmailEnvelope(intent, traceID, now)

	// Emit routing event
	e.emitEvent(events.Phase10ExecutionRouted, intent, "email")

	// Execute via boundary executor
	result, err := e.emailExecutor.Execute(ctx, envelope)
	if err != nil {
		e.emitEvent(events.Phase10ExecutionFailed, intent, err.Error())
		return ExecutionOutcome{
			IntentID:   intent.IntentID,
			Success:    false,
			Error:      err.Error(),
			ExecutedAt: now,
		}
	}

	// Map result to outcome
	outcome := e.mapEmailResult(intent, result, now)

	// Emit appropriate event
	if outcome.Success {
		e.emitEvent(events.Phase10ExecutionSucceeded, intent, "")
	} else if outcome.Blocked {
		e.emitEvent(events.Phase10ExecutionBlocked, intent, outcome.BlockedReason)
	} else {
		e.emitEvent(events.Phase10ExecutionFailed, intent, outcome.Error)
	}

	return outcome
}

// executeCalendar routes a calendar intent to the calendar boundary executor.
func (e *Executor) executeCalendar(ctx context.Context, intent *execintent.ExecutionIntent, traceID string, now time.Time) ExecutionOutcome {
	// Check executor is configured
	if e.calendarExecutor == nil {
		e.emitEvent(events.Phase10ExecutionBlocked, intent, "calendar executor not configured")
		return ExecutionOutcome{
			IntentID:      intent.IntentID,
			Success:       false,
			Blocked:       true,
			BlockedReason: "calendar executor not configured",
			ExecutedAt:    now,
		}
	}

	// Build calendar envelope from intent
	envelope := e.buildCalendarEnvelope(intent, traceID, now)

	// Emit routing event
	e.emitEvent(events.Phase10ExecutionRouted, intent, "calendar")

	// Execute via boundary executor
	result := e.calendarExecutor.Execute(ctx, envelope)

	// Map result to outcome
	outcome := e.mapCalendarResult(intent, result)

	// Emit appropriate event
	if outcome.Success {
		e.emitEvent(events.Phase10ExecutionSucceeded, intent, "")
	} else if outcome.Blocked {
		e.emitEvent(events.Phase10ExecutionBlocked, intent, outcome.BlockedReason)
	} else {
		e.emitEvent(events.Phase10ExecutionFailed, intent, outcome.Error)
	}

	return outcome
}

// buildEmailEnvelope creates an email envelope from an execution intent.
func (e *Executor) buildEmailEnvelope(intent *execintent.ExecutionIntent, traceID string, now time.Time) emailexec.Envelope {
	// Compute envelope ID deterministically
	envelopeID := computeEmailEnvelopeID(intent, traceID, now)
	idempotencyKey := computeIdempotencyKey(envelopeID)

	return emailexec.Envelope{
		EnvelopeID:         envelopeID,
		DraftID:            intent.DraftID,
		CircleID:           identity.EntityID(intent.CircleID),
		Provider:           "", // Will be resolved by email executor from context
		AccountID:          "", // Will be filled by context
		ThreadID:           intent.EmailThreadID,
		InReplyToMessageID: intent.EmailMessageID,
		Subject:            intent.EmailSubject,
		Body:               intent.EmailBody,
		PolicySnapshotHash: intent.PolicySnapshotHash,
		ViewSnapshotHash:   intent.ViewSnapshotHash,
		ViewSnapshotAt:     intent.CreatedAt, // Use intent creation time as view snapshot time
		IdempotencyKey:     idempotencyKey,
		TraceID:            traceID,
		CreatedAt:          now,
		ApprovedAt:         intent.CreatedAt,
		Status:             emailexec.EnvelopeStatusPending,
	}
}

// buildCalendarEnvelope creates a calendar envelope from an execution intent.
func (e *Executor) buildCalendarEnvelope(intent *execintent.ExecutionIntent, traceID string, now time.Time) *calexec.Envelope {
	// Compute envelope ID deterministically
	envelopeID := computeCalendarEnvelopeID(intent, traceID)
	idempotencyKey := computeIdempotencyKey(envelopeID)

	// Map response string to CalendarResponse
	response := draft.CalendarResponse(intent.CalendarResponse)

	return &calexec.Envelope{
		EnvelopeID:         envelopeID,
		DraftID:            intent.DraftID,
		CircleID:           identity.EntityID(intent.CircleID),
		Provider:           "", // Will be filled from draft content
		CalendarID:         "", // Will be filled from draft content
		EventID:            intent.CalendarEventID,
		Response:           response,
		Message:            "", // Optional message from draft
		ProposeNewTime:     false,
		PolicySnapshotHash: intent.PolicySnapshotHash,
		ViewSnapshotHash:   intent.ViewSnapshotHash,
		ViewSnapshotAt:     intent.CreatedAt,
		IdempotencyKey:     idempotencyKey,
		TraceID:            traceID,
		Status:             calexec.EnvelopeStatusPending,
		CreatedAt:          now,
	}
}

// mapEmailResult maps an email execution result to an ExecutionOutcome.
func (e *Executor) mapEmailResult(intent *execintent.ExecutionIntent, result *emailexec.Envelope, now time.Time) ExecutionOutcome {
	if result == nil {
		return ExecutionOutcome{
			IntentID:   intent.IntentID,
			Success:    false,
			Error:      "nil result from email executor",
			ExecutedAt: now,
		}
	}

	switch result.Status {
	case emailexec.EnvelopeStatusExecuted:
		providerResponseID := ""
		if result.ExecutionResult != nil {
			providerResponseID = result.ExecutionResult.ProviderResponseID
		}
		return ExecutionOutcome{
			IntentID:           intent.IntentID,
			Success:            true,
			ProviderResponseID: providerResponseID,
			ExecutedAt:         now,
			EnvelopeID:         result.EnvelopeID,
		}

	case emailexec.EnvelopeStatusBlocked:
		blockedReason := ""
		if result.ExecutionResult != nil {
			blockedReason = result.ExecutionResult.BlockedReason
		}
		return ExecutionOutcome{
			IntentID:      intent.IntentID,
			Success:       false,
			Blocked:       true,
			BlockedReason: blockedReason,
			ExecutedAt:    now,
			EnvelopeID:    result.EnvelopeID,
		}

	case emailexec.EnvelopeStatusFailed:
		errorMsg := ""
		if result.ExecutionResult != nil {
			errorMsg = result.ExecutionResult.Error
		}
		return ExecutionOutcome{
			IntentID:   intent.IntentID,
			Success:    false,
			Error:      errorMsg,
			ExecutedAt: now,
			EnvelopeID: result.EnvelopeID,
		}

	default:
		return ExecutionOutcome{
			IntentID:   intent.IntentID,
			Success:    false,
			Error:      fmt.Sprintf("unexpected envelope status: %s", result.Status),
			ExecutedAt: now,
			EnvelopeID: result.EnvelopeID,
		}
	}
}

// mapCalendarResult maps a calendar execution result to an ExecutionOutcome.
func (e *Executor) mapCalendarResult(intent *execintent.ExecutionIntent, result calexec.ExecuteResult) ExecutionOutcome {
	if result.Success {
		return ExecutionOutcome{
			IntentID:           intent.IntentID,
			Success:            true,
			ProviderResponseID: result.ProviderResponseID,
			ExecutedAt:         result.ExecutedAt,
			EnvelopeID:         result.EnvelopeID,
		}
	}

	if result.Blocked {
		return ExecutionOutcome{
			IntentID:      intent.IntentID,
			Success:       false,
			Blocked:       true,
			BlockedReason: result.BlockedReason,
			ExecutedAt:    result.ExecutedAt,
			EnvelopeID:    result.EnvelopeID,
		}
	}

	return ExecutionOutcome{
		IntentID:   intent.IntentID,
		Success:    false,
		Error:      result.Error,
		ExecutedAt: result.ExecutedAt,
		EnvelopeID: result.EnvelopeID,
	}
}

// emitEvent emits an event with intent context.
func (e *Executor) emitEvent(eventType events.EventType, intent *execintent.ExecutionIntent, detail string) {
	if e.emitter == nil {
		return
	}

	metadata := map[string]string{
		"intent_id": string(intent.IntentID),
		"draft_id":  string(intent.DraftID),
		"circle_id": intent.CircleID,
		"action":    string(intent.Action),
	}
	if detail != "" {
		metadata["detail"] = detail
	}

	e.emitter.Emit(events.Event{
		Type:      eventType,
		Timestamp: e.clock.Now(),
		CircleID:  intent.CircleID,
		SubjectID: string(intent.IntentID),
		Metadata:  metadata,
	})
}

// computeEmailEnvelopeID computes a deterministic envelope ID for email.
func computeEmailEnvelopeID(intent *execintent.ExecutionIntent, traceID string, createdAt time.Time) string {
	canonical := fmt.Sprintf("email-envelope|%s|%s|%s|%s|%s|%s",
		intent.DraftID,
		intent.CircleID,
		intent.PolicySnapshotHash,
		intent.ViewSnapshotHash,
		traceID,
		createdAt.UTC().Format(time.RFC3339Nano),
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8])
}

// computeCalendarEnvelopeID computes a deterministic envelope ID for calendar.
func computeCalendarEnvelopeID(intent *execintent.ExecutionIntent, traceID string) string {
	canonical := fmt.Sprintf("calendar-envelope|%s|%s|%s|%s|%s|%s|%s",
		intent.DraftID,
		intent.CircleID,
		intent.CalendarEventID,
		intent.CalendarResponse,
		intent.PolicySnapshotHash,
		intent.ViewSnapshotHash,
		traceID,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// computeIdempotencyKey computes an idempotency key from envelope ID.
func computeIdempotencyKey(envelopeID string) string {
	canonical := fmt.Sprintf("idem|%s", envelopeID)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}
