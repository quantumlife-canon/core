// Package execution provides email execution boundary enforcement.
//
// CRITICAL: This is the ONLY path to external email writes.
// CRITICAL: Must verify policy and view snapshots before execution.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent - same envelope executed twice returns same result.
//
// Reference: Phase 7 Email Execution Boundary
package execution

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/email/write"
	"quantumlife/pkg/events"
)

// Executor executes email sends with full boundary enforcement.
//
// CRITICAL: This is the ONLY external write path.
// CRITICAL: All writes flow through Execute().
type Executor struct {
	// store persists envelopes.
	store Store

	// policyVerifier verifies policy snapshots.
	policyVerifier *PolicyVerifier

	// viewVerifier verifies view snapshots.
	viewVerifier *ViewVerifier

	// writerRegistry provides email writers by provider.
	writerRegistry map[string]write.Writer

	// eventEmitter emits audit events.
	eventEmitter events.Emitter

	// clock provides current time.
	clock func() time.Time
}

// ExecutorOption configures the executor.
type ExecutorOption func(*Executor)

// WithStore sets the envelope store.
func WithStore(store Store) ExecutorOption {
	return func(e *Executor) {
		e.store = store
	}
}

// WithPolicyVerifier sets the policy verifier.
func WithPolicyVerifier(v *PolicyVerifier) ExecutorOption {
	return func(e *Executor) {
		e.policyVerifier = v
	}
}

// WithViewVerifier sets the view verifier.
func WithViewVerifier(v *ViewVerifier) ExecutorOption {
	return func(e *Executor) {
		e.viewVerifier = v
	}
}

// WithWriter registers a writer for a provider.
func WithWriter(provider string, w write.Writer) ExecutorOption {
	return func(e *Executor) {
		e.writerRegistry[provider] = w
	}
}

// WithEventEmitter sets the event emitter.
func WithEventEmitter(emitter events.Emitter) ExecutorOption {
	return func(e *Executor) {
		e.eventEmitter = emitter
	}
}

// WithExecutorClock sets the clock function.
func WithExecutorClock(clock func() time.Time) ExecutorOption {
	return func(e *Executor) {
		e.clock = clock
	}
}

// NewExecutor creates a new email executor.
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		store:          NewMemoryStore(),
		writerRegistry: make(map[string]write.Writer),
		clock:          time.Now,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute executes an email send with full boundary enforcement.
//
// CRITICAL: This is the ONLY external write path.
// CRITICAL: No auto-retries on failure.
// CRITICAL: Idempotent - same IdempotencyKey returns same result.
func (e *Executor) Execute(ctx context.Context, envelope Envelope) (*Envelope, error) {
	now := e.clock()

	// Emit execution attempt event
	e.emit(events.Event{
		Type:      events.EmailExecutionAttempted,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id":     envelope.EnvelopeID,
			"draft_id":        string(envelope.DraftID),
			"circle_id":       string(envelope.CircleID),
			"provider":        envelope.Provider,
			"thread_id":       envelope.ThreadID,
			"idempotency_key": envelope.IdempotencyKey,
		},
	})

	// Check idempotency first
	if existing, found := e.store.GetByIdempotencyKey(envelope.IdempotencyKey); found {
		// Return existing result without re-executing
		e.emit(events.Event{
			Type:      events.EmailExecutionIdempotent,
			Timestamp: now,
			Metadata: map[string]string{
				"envelope_id":          envelope.EnvelopeID,
				"existing_envelope_id": existing.EnvelopeID,
				"idempotency_key":      envelope.IdempotencyKey,
				"status":               string(existing.Status),
			},
		})
		return &existing, nil
	}

	// Validate envelope
	if err := envelope.Validate(); err != nil {
		return e.block(envelope, fmt.Sprintf("validation failed: %v", err), now)
	}

	// Verify policy snapshot
	if e.policyVerifier != nil {
		policySnapshot := PolicySnapshot{
			PolicyHash: envelope.PolicySnapshotHash,
			CircleID:   envelope.CircleID,
		}
		if err := e.policyVerifier.Verify(policySnapshot); err != nil {
			return e.block(envelope, fmt.Sprintf("policy verification failed: %v", err), now)
		}
	}

	// Verify view snapshot
	if e.viewVerifier != nil {
		viewSnapshot := ViewSnapshot{
			SnapshotHash: envelope.ViewSnapshotHash,
			CapturedAt:   envelope.ViewSnapshotAt,
			ThreadID:     envelope.ThreadID,
		}
		if err := e.viewVerifier.Verify(viewSnapshot); err != nil {
			return e.block(envelope, fmt.Sprintf("view verification failed: %v", err), now)
		}
	}

	// Get writer for provider
	writer, found := e.writerRegistry[envelope.Provider]
	if !found {
		return e.block(envelope, fmt.Sprintf("unknown provider: %s", envelope.Provider), now)
	}

	// Build send request
	sendReq := write.SendReplyRequest{
		Provider:           envelope.Provider,
		AccountID:          envelope.AccountID,
		CircleID:           string(envelope.CircleID),
		ThreadID:           envelope.ThreadID,
		InReplyToMessageID: envelope.InReplyToMessageID,
		Subject:            envelope.Subject,
		Body:               envelope.Body,
		IdempotencyKey:     envelope.IdempotencyKey,
		TraceID:            envelope.TraceID,
	}

	// CRITICAL: Execute the external write
	// NO AUTO-RETRIES. Single attempt only.
	receipt, err := writer.SendReply(ctx, sendReq)
	if err != nil {
		return e.fail(envelope, fmt.Sprintf("send failed: %v", err), now)
	}

	if !receipt.Success {
		return e.fail(envelope, receipt.Error, now)
	}

	// Success - update envelope
	envelope.Status = EnvelopeStatusExecuted
	executedAt := now
	envelope.ExecutedAt = &executedAt
	envelope.ExecutionResult = &ExecutionResult{
		Success:            true,
		MessageID:          receipt.MessageID,
		ProviderResponseID: receipt.ProviderResponseID,
	}

	// Store the result
	if err := e.store.Put(envelope); err != nil {
		// Log but don't fail - the write succeeded
		e.emit(events.Event{
			Type:      events.EmailExecutionStoreError,
			Timestamp: now,
			Metadata: map[string]string{
				"envelope_id": envelope.EnvelopeID,
				"error":       err.Error(),
			},
		})
	}

	// Emit success event
	e.emit(events.Event{
		Type:      events.EmailExecutionSucceeded,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id":          envelope.EnvelopeID,
			"draft_id":             string(envelope.DraftID),
			"circle_id":            string(envelope.CircleID),
			"provider":             envelope.Provider,
			"message_id":           receipt.MessageID,
			"thread_id":            receipt.ThreadID,
			"provider_response_id": receipt.ProviderResponseID,
		},
	})

	return &envelope, nil
}

// block marks an envelope as blocked and returns it.
func (e *Executor) block(envelope Envelope, reason string, now time.Time) (*Envelope, error) {
	envelope.Status = EnvelopeStatusBlocked
	envelope.ExecutionResult = &ExecutionResult{
		Success:       false,
		BlockedReason: reason,
	}

	if err := e.store.Put(envelope); err != nil {
		// Log but continue
		e.emit(events.Event{
			Type:      events.EmailExecutionStoreError,
			Timestamp: now,
			Metadata: map[string]string{
				"envelope_id": envelope.EnvelopeID,
				"error":       err.Error(),
			},
		})
	}

	e.emit(events.Event{
		Type:      events.EmailExecutionBlocked,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"draft_id":    string(envelope.DraftID),
			"circle_id":   string(envelope.CircleID),
			"reason":      reason,
		},
	})

	return &envelope, nil
}

// fail marks an envelope as failed and returns it.
func (e *Executor) fail(envelope Envelope, errorMsg string, now time.Time) (*Envelope, error) {
	envelope.Status = EnvelopeStatusFailed
	envelope.ExecutionResult = &ExecutionResult{
		Success: false,
		Error:   errorMsg,
	}

	if err := e.store.Put(envelope); err != nil {
		// Log but continue
		e.emit(events.Event{
			Type:      events.EmailExecutionStoreError,
			Timestamp: now,
			Metadata: map[string]string{
				"envelope_id": envelope.EnvelopeID,
				"error":       err.Error(),
			},
		})
	}

	e.emit(events.Event{
		Type:      events.EmailExecutionFailed,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"draft_id":    string(envelope.DraftID),
			"circle_id":   string(envelope.CircleID),
			"error":       errorMsg,
		},
	})

	return &envelope, nil
}

// emit emits an event if emitter is configured.
func (e *Executor) emit(event events.Event) {
	if e.eventEmitter != nil {
		e.eventEmitter.Emit(event)
	}
}

// GetEnvelope retrieves an envelope by ID.
func (e *Executor) GetEnvelope(id string) (Envelope, bool) {
	return e.store.Get(id)
}

// GetEnvelopeByIdempotencyKey retrieves an envelope by idempotency key.
func (e *Executor) GetEnvelopeByIdempotencyKey(key string) (Envelope, bool) {
	return e.store.GetByIdempotencyKey(key)
}

// ListEnvelopes returns all envelopes matching the filter.
func (e *Executor) ListEnvelopes(filter ListFilter) []Envelope {
	return e.store.List(filter)
}
