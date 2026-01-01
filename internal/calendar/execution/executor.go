package execution

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/calendar/write"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/events"
)

// Executor executes calendar envelopes.
//
// CRITICAL: This is the ONLY path to external calendar writes.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must verify policy and view before execution.
type Executor struct {
	// writers provides write connectors by provider ID.
	writers map[string]write.Writer

	// envelopeStore stores execution envelopes.
	envelopeStore Store

	// policyVerifier verifies policy snapshots.
	policyVerifier *PolicyVerifier

	// viewVerifier verifies view snapshots.
	viewVerifier *ViewVerifier

	// freshnessPolicy defines staleness thresholds.
	freshnessPolicy FreshnessPolicy

	// draftStore accesses approved drafts.
	draftStore draft.Store

	// eventEmitter emits execution events.
	eventEmitter events.Emitter

	// clock for deterministic timestamps.
	clock func() time.Time
}

// ExecutorConfig contains configuration for the executor.
type ExecutorConfig struct {
	EnvelopeStore   Store
	DraftStore      draft.Store
	PolicyVerifier  *PolicyVerifier
	ViewVerifier    *ViewVerifier
	FreshnessPolicy FreshnessPolicy
	EventEmitter    events.Emitter
	Clock           func() time.Time
}

// NewExecutor creates a new executor.
func NewExecutor(config ExecutorConfig) *Executor {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.FreshnessPolicy.DefaultMaxStaleness == 0 {
		config.FreshnessPolicy = NewDefaultFreshnessPolicy()
	}

	return &Executor{
		writers:         make(map[string]write.Writer),
		envelopeStore:   config.EnvelopeStore,
		policyVerifier:  config.PolicyVerifier,
		viewVerifier:    config.ViewVerifier,
		freshnessPolicy: config.FreshnessPolicy,
		draftStore:      config.DraftStore,
		eventEmitter:    config.EventEmitter,
		clock:           config.Clock,
	}
}

// RegisterWriter registers a write connector for a provider.
func (e *Executor) RegisterWriter(providerID string, w write.Writer) {
	e.writers[providerID] = w
}

// ExecuteResult contains the result of executing an envelope.
type ExecuteResult struct {
	// EnvelopeID is the envelope that was executed.
	EnvelopeID string

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

	// ETag is the new etag after update.
	ETag string

	// ExecutedAt is when execution completed.
	ExecutedAt time.Time
}

// Execute executes an envelope.
//
// CRITICAL: This is the ONLY external write path.
// CRITICAL: No retries on failure. No background execution.
func (e *Executor) Execute(ctx context.Context, envelope *Envelope) ExecuteResult {
	now := e.clock()

	// Emit execution started event
	e.emitEvent(events.Phase5CalendarExecutionStarted, map[string]string{
		"envelope_id": envelope.EnvelopeID,
		"event_id":    envelope.EventID,
		"provider":    envelope.Provider,
		"response":    string(envelope.Response),
	})

	// Validate envelope
	if err := envelope.Validate(); err != nil {
		return e.fail(envelope, fmt.Sprintf("validation failed: %v", err), now)
	}

	// Check idempotency - if already executed, return prior result
	if existing, found := e.envelopeStore.GetByIdempotencyKey(envelope.IdempotencyKey); found {
		if existing.Status == EnvelopeStatusExecuted {
			e.emitEvent(events.Phase5CalendarExecutionIdempotent, map[string]string{
				"envelope_id":          envelope.EnvelopeID,
				"prior_envelope_id":    existing.EnvelopeID,
				"provider_response_id": existing.ExecutionResult.ProviderResponseID,
			})
			return ExecuteResult{
				EnvelopeID:         existing.EnvelopeID,
				Success:            true,
				ProviderResponseID: existing.ExecutionResult.ProviderResponseID,
				ETag:               existing.ExecutionResult.ETag,
				ExecutedAt:         *existing.ExecutedAt,
			}
		}
	}

	// Verify policy snapshot
	if e.policyVerifier != nil {
		policySnapshot := PolicySnapshot{
			PolicyHash:     envelope.PolicySnapshotHash,
			CircleID:       envelope.CircleID,
			IntersectionID: envelope.IntersectionID,
		}
		if err := e.policyVerifier.Verify(policySnapshot); err != nil {
			return e.block(envelope, fmt.Sprintf("policy verification failed: %v", err), now)
		}
	}

	// Verify view snapshot freshness
	if e.viewVerifier != nil {
		viewSnapshot := ViewSnapshot{
			ViewHash:   envelope.ViewSnapshotHash,
			Provider:   envelope.Provider,
			CalendarID: envelope.CalendarID,
			EventID:    envelope.EventID,
			CapturedAt: envelope.ViewSnapshotAt,
		}
		maxStaleness := e.freshnessPolicy.GetMaxStaleness(string(envelope.Response))
		_, err := e.viewVerifier.Verify(viewSnapshot, maxStaleness, now)
		if err != nil {
			return e.block(envelope, fmt.Sprintf("view verification failed: %v", err), now)
		}
	}

	// Get the writer for this provider
	writer, exists := e.writers[envelope.Provider]
	if !exists {
		return e.block(envelope, fmt.Sprintf("no writer registered for provider: %s", envelope.Provider), now)
	}

	// Build the write input
	input := write.RespondInput{
		Provider:       envelope.Provider,
		CalendarID:     envelope.CalendarID,
		EventID:        envelope.EventID,
		ResponseStatus: toResponseStatus(envelope.Response),
		Message:        envelope.Message,
		ProposeNewTime: envelope.ProposeNewTime,
		ProposedStart:  envelope.ProposedStart,
		ProposedEnd:    envelope.ProposedEnd,
		IdempotencyKey: envelope.IdempotencyKey,
		TraceID:        envelope.TraceID,
	}

	// Execute the write
	// CRITICAL: This is the ONLY external write.
	// CRITICAL: No retries on failure.
	receipt, err := writer.RespondToEvent(ctx, input)
	if err != nil {
		return e.fail(envelope, fmt.Sprintf("write failed: %v", err), now)
	}

	if !receipt.Success {
		return e.fail(envelope, fmt.Sprintf("write rejected: %s", receipt.Error), now)
	}

	// Update envelope with result
	envelope.Status = EnvelopeStatusExecuted
	envelope.ExecutedAt = &now
	envelope.ExecutionResult = &ExecutionResult{
		Success:            true,
		ProviderResponseID: receipt.ProviderResponseID,
		ETag:               receipt.ETag,
	}

	// Store the executed envelope
	if err := e.envelopeStore.Put(*envelope); err != nil {
		// Log but don't fail - the external write succeeded
		e.emitEvent(events.Phase5CalendarEnvelopeStoreError, map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"error":       err.Error(),
		})
	}

	// Emit success event
	e.emitEvent(events.Phase5CalendarExecutionSuccess, map[string]string{
		"envelope_id":          envelope.EnvelopeID,
		"event_id":             envelope.EventID,
		"provider":             envelope.Provider,
		"provider_response_id": receipt.ProviderResponseID,
		"etag":                 receipt.ETag,
	})

	return ExecuteResult{
		EnvelopeID:         envelope.EnvelopeID,
		Success:            true,
		ProviderResponseID: receipt.ProviderResponseID,
		ETag:               receipt.ETag,
		ExecutedAt:         now,
	}
}

// ExecuteFromDraft creates an envelope from an approved draft and executes it.
//
// CRITICAL: Only approved drafts can be executed.
func (e *Executor) ExecuteFromDraft(
	ctx context.Context,
	d draft.Draft,
	policySnapshot PolicySnapshot,
	viewSnapshot ViewSnapshot,
	traceID string,
) ExecuteResult {
	now := e.clock()

	// Create envelope from draft
	envelope, err := NewEnvelopeFromDraft(d, policySnapshot.PolicyHash, viewSnapshot.ViewHash, viewSnapshot.CapturedAt, traceID, now)
	if err != nil {
		return ExecuteResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create envelope: %v", err),
		}
	}

	// Execute the envelope
	return e.Execute(ctx, envelope)
}

// fail marks an envelope as failed and returns a failure result.
func (e *Executor) fail(envelope *Envelope, errorMsg string, now time.Time) ExecuteResult {
	envelope.Status = EnvelopeStatusFailed
	envelope.ExecutedAt = &now
	envelope.ExecutionResult = &ExecutionResult{
		Success: false,
		Error:   errorMsg,
	}

	// Store the failed envelope
	_ = e.envelopeStore.Put(*envelope)

	// Emit failure event
	e.emitEvent(events.Phase5CalendarExecutionFailed, map[string]string{
		"envelope_id": envelope.EnvelopeID,
		"event_id":    envelope.EventID,
		"error":       errorMsg,
	})

	return ExecuteResult{
		EnvelopeID: envelope.EnvelopeID,
		Success:    false,
		Error:      errorMsg,
		ExecutedAt: now,
	}
}

// block marks an envelope as blocked and returns a blocked result.
func (e *Executor) block(envelope *Envelope, reason string, now time.Time) ExecuteResult {
	envelope.Status = EnvelopeStatusBlocked
	envelope.ExecutedAt = &now
	envelope.ExecutionResult = &ExecutionResult{
		Success:       false,
		BlockedReason: reason,
	}

	// Store the blocked envelope
	_ = e.envelopeStore.Put(*envelope)

	// Emit blocked event
	e.emitEvent(events.Phase5CalendarExecutionBlocked, map[string]string{
		"envelope_id":    envelope.EnvelopeID,
		"event_id":       envelope.EventID,
		"blocked_reason": reason,
	})

	return ExecuteResult{
		EnvelopeID:    envelope.EnvelopeID,
		Success:       false,
		Blocked:       true,
		BlockedReason: reason,
		ExecutedAt:    now,
	}
}

// emitEvent emits an execution event.
func (e *Executor) emitEvent(eventType events.EventType, metadata map[string]string) {
	if e.eventEmitter == nil {
		return
	}
	e.eventEmitter.Emit(events.Event{
		Type:     eventType,
		Metadata: metadata,
	})
}

// toResponseStatus converts draft response to write response status.
func toResponseStatus(response draft.CalendarResponse) write.ResponseStatus {
	switch response {
	case draft.CalendarResponseAccept:
		return write.ResponseAccepted
	case draft.CalendarResponseDecline:
		return write.ResponseDeclined
	case draft.CalendarResponseTentative:
		return write.ResponseTentative
	default:
		return write.ResponseTentative
	}
}

// ExecutorStats contains executor statistics.
type ExecutorStats struct {
	TotalEnvelopes   int
	PendingEnvelopes int
	ExecutedCount    int
	FailedCount      int
	BlockedCount     int
}

// GetStats returns executor statistics.
func (e *Executor) GetStats() ExecutorStats {
	all := e.envelopeStore.List(ListFilter{IncludeAll: true})

	stats := ExecutorStats{
		TotalEnvelopes: len(all),
	}

	for _, env := range all {
		switch env.Status {
		case EnvelopeStatusPending:
			stats.PendingEnvelopes++
		case EnvelopeStatusExecuted:
			stats.ExecutedCount++
		case EnvelopeStatusFailed:
			stats.FailedCount++
		case EnvelopeStatusBlocked:
			stats.BlockedCount++
		}
	}

	return stats
}

// GetEnvelope retrieves an envelope by ID.
func (e *Executor) GetEnvelope(id string) (Envelope, bool) {
	return e.envelopeStore.Get(id)
}

// GetEnvelopeByDraft retrieves an envelope by draft ID.
func (e *Executor) GetEnvelopeByDraft(draftID draft.DraftID) (Envelope, bool) {
	return e.envelopeStore.GetByDraftID(draftID)
}

// CreateEnvelope creates an envelope from a draft without executing it.
func (e *Executor) CreateEnvelope(
	d draft.Draft,
	policySnapshot PolicySnapshot,
	viewSnapshot ViewSnapshot,
	traceID string,
) (*Envelope, error) {
	now := e.clock()
	return NewEnvelopeFromDraft(d, policySnapshot.PolicyHash, viewSnapshot.ViewHash, viewSnapshot.CapturedAt, traceID, now)
}

// ValidateExecutability checks if an envelope can be executed.
func (e *Executor) ValidateExecutability(envelope *Envelope) error {
	now := e.clock()

	// Validate envelope
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check for prior execution
	if existing, found := e.envelopeStore.GetByIdempotencyKey(envelope.IdempotencyKey); found {
		if existing.Status == EnvelopeStatusExecuted {
			return nil // Already executed successfully
		}
	}

	// Verify policy
	if e.policyVerifier != nil {
		policySnapshot := PolicySnapshot{
			PolicyHash:     envelope.PolicySnapshotHash,
			CircleID:       envelope.CircleID,
			IntersectionID: envelope.IntersectionID,
		}
		if err := e.policyVerifier.Verify(policySnapshot); err != nil {
			return fmt.Errorf("policy: %w", err)
		}
	}

	// Verify view freshness
	if e.viewVerifier != nil {
		viewSnapshot := ViewSnapshot{
			ViewHash:   envelope.ViewSnapshotHash,
			Provider:   envelope.Provider,
			CalendarID: envelope.CalendarID,
			EventID:    envelope.EventID,
			CapturedAt: envelope.ViewSnapshotAt,
		}
		maxStaleness := e.freshnessPolicy.GetMaxStaleness(string(envelope.Response))
		_, err := e.viewVerifier.Verify(viewSnapshot, maxStaleness, now)
		if err != nil {
			return fmt.Errorf("view: %w", err)
		}
	}

	// Check writer exists
	if _, exists := e.writers[envelope.Provider]; !exists {
		return fmt.Errorf("no writer for provider: %s", envelope.Provider)
	}

	return nil
}

// PipelineCheck defines the execution pipeline verification.
type PipelineCheck struct {
	// EnvelopeValid indicates the envelope passed validation.
	EnvelopeValid bool

	// PolicyVerified indicates policy snapshot matches current.
	PolicyVerified bool

	// ViewFresh indicates view snapshot is within staleness.
	ViewFresh bool

	// ViewUnchanged indicates view hasn't changed.
	ViewUnchanged bool

	// WriterAvailable indicates a writer is registered.
	WriterAvailable bool

	// Errors contains any verification errors.
	Errors []string
}

// CheckPipeline performs pre-execution pipeline verification.
func (e *Executor) CheckPipeline(envelope *Envelope) PipelineCheck {
	now := e.clock()
	check := PipelineCheck{}

	// Validate envelope
	if err := envelope.Validate(); err != nil {
		check.Errors = append(check.Errors, fmt.Sprintf("validation: %v", err))
	} else {
		check.EnvelopeValid = true
	}

	// Verify policy
	if e.policyVerifier != nil {
		policySnapshot := PolicySnapshot{
			PolicyHash:     envelope.PolicySnapshotHash,
			CircleID:       envelope.CircleID,
			IntersectionID: envelope.IntersectionID,
		}
		if err := e.policyVerifier.Verify(policySnapshot); err != nil {
			check.Errors = append(check.Errors, fmt.Sprintf("policy: %v", err))
		} else {
			check.PolicyVerified = true
		}
	} else {
		check.PolicyVerified = true // No verifier = pass
	}

	// Verify view
	if e.viewVerifier != nil {
		viewSnapshot := ViewSnapshot{
			ViewHash:   envelope.ViewSnapshotHash,
			Provider:   envelope.Provider,
			CalendarID: envelope.CalendarID,
			EventID:    envelope.EventID,
			CapturedAt: envelope.ViewSnapshotAt,
		}
		maxStaleness := e.freshnessPolicy.GetMaxStaleness(string(envelope.Response))
		result, err := e.viewVerifier.Verify(viewSnapshot, maxStaleness, now)
		if err != nil {
			check.Errors = append(check.Errors, fmt.Sprintf("view: %v", err))
		} else {
			check.ViewFresh = result.Fresh
			check.ViewUnchanged = result.Unchanged
		}
	} else {
		check.ViewFresh = true // No verifier = pass
		check.ViewUnchanged = true
	}

	// Check writer
	if _, exists := e.writers[envelope.Provider]; exists {
		check.WriterAvailable = true
	} else {
		check.Errors = append(check.Errors, fmt.Sprintf("no writer for provider: %s", envelope.Provider))
	}

	return check
}
