// Package execution provides v9 financial execution primitives.
//
// This file implements GuardedExecutionAdapter - an adapter that
// looks like real execution but CANNOT move money.
//
// CRITICAL: This adapter ALWAYS blocks execution.
// Execute() ALWAYS returns GuardedExecutionError.
// NO REAL MONEY MOVES. NO SIDE EFFECTS.
//
// Subordinate to:
// - docs/CANON_ADDENDUM_V9_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"fmt"
	"time"

	"quantumlife/pkg/events"
)

// GuardedExecutionAdapter is an execution adapter that CANNOT move money.
//
// It exists to:
// - Prove the execution pipeline structure is correct
// - Generate auditable execution attempts
// - Demonstrate that revocation/expiry block execution
//
// It CANNOT:
// - Move money
// - Perform side effects
// - Succeed at execution
type GuardedExecutionAdapter struct {
	// provider is the provider name.
	provider string

	// idGenerator generates unique IDs.
	idGenerator func() string

	// auditEmitter emits audit events.
	auditEmitter func(event events.Event)

	// clock provides current time.
	clock func() time.Time
}

// GuardedAdapterConfig configures a guarded adapter.
type GuardedAdapterConfig struct {
	// Provider is the provider name (e.g., "mock-finance").
	Provider string

	// IDGenerator generates unique IDs.
	IDGenerator func() string

	// AuditEmitter emits audit events.
	AuditEmitter func(event events.Event)

	// Clock provides current time (defaults to time.Now).
	Clock func() time.Time
}

// NewGuardedExecutionAdapter creates a new guarded execution adapter.
func NewGuardedExecutionAdapter(config GuardedAdapterConfig) *GuardedExecutionAdapter {
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}

	return &GuardedExecutionAdapter{
		provider:     config.Provider,
		idGenerator:  config.IDGenerator,
		auditEmitter: config.AuditEmitter,
		clock:        clock,
	}
}

// Provider returns the provider name.
func (a *GuardedExecutionAdapter) Provider() string {
	return a.provider
}

// Prepare validates that the envelope can be executed.
// This performs pre-execution checks WITHOUT side effects.
func (a *GuardedExecutionAdapter) Prepare(envelope *ExecutionEnvelope) (*PrepareResult, error) {
	now := a.clock()

	// Validate envelope exists
	if envelope == nil {
		return &PrepareResult{
			Provider:      a.provider,
			PreparedAt:    now,
			Valid:         false,
			InvalidReason: "envelope is nil",
		}, fmt.Errorf("envelope is nil")
	}

	// Validate envelope is sealed (SealHash is non-empty when sealed)
	if envelope.SealHash == "" {
		return &PrepareResult{
			EnvelopeID:    envelope.EnvelopeID,
			Provider:      a.provider,
			PreparedAt:    now,
			Valid:         false,
			InvalidReason: "envelope not sealed: missing seal hash",
		}, fmt.Errorf("envelope not sealed: missing seal hash")
	}

	// Validate envelope has not expired
	if now.After(envelope.Expiry) {
		return &PrepareResult{
			EnvelopeID:    envelope.EnvelopeID,
			Provider:      a.provider,
			PreparedAt:    now,
			Valid:         false,
			InvalidReason: "envelope has expired",
		}, fmt.Errorf("envelope expired at %s", envelope.Expiry.Format(time.RFC3339))
	}

	// Validate action spec
	if envelope.ActionSpec.AmountCents <= 0 {
		return &PrepareResult{
			EnvelopeID:    envelope.EnvelopeID,
			Provider:      a.provider,
			PreparedAt:    now,
			Valid:         false,
			InvalidReason: "invalid amount",
		}, fmt.Errorf("invalid amount: %d", envelope.ActionSpec.AmountCents)
	}

	// Emit prepare event
	if a.auditEmitter != nil {
		a.auditEmitter(events.Event{
			ID:             a.idGenerator(),
			Type:           events.EventV9AdapterPrepared,
			Timestamp:      now,
			CircleID:       envelope.ActorCircleID,
			IntersectionID: envelope.IntersectionID,
			SubjectID:      envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       a.provider,
			Metadata: map[string]string{
				"action_hash": envelope.ActionHash,
				"amount":      fmt.Sprintf("%d", envelope.ActionSpec.AmountCents),
				"currency":    envelope.ActionSpec.Currency,
			},
		})
	}

	return &PrepareResult{
		EnvelopeID: envelope.EnvelopeID,
		Provider:   a.provider,
		PreparedAt: now,
		Valid:      true,
	}, nil
}

// Execute attempts to execute the envelope.
//
// CRITICAL: This ALWAYS fails with GuardedExecutionError.
// NO REAL MONEY MOVES. NO SIDE EFFECTS.
//
// This method exists to:
// - Prove the execution pipeline reaches the adapter
// - Generate an auditable execution attempt
// - Demonstrate the guardrail blocks execution
func (a *GuardedExecutionAdapter) Execute(envelope *ExecutionEnvelope, approval *ApprovalArtifact) (*ExecutionAttempt, error) {
	now := a.clock()
	attemptID := a.idGenerator()

	// Emit invocation event BEFORE blocking
	if a.auditEmitter != nil {
		a.auditEmitter(events.Event{
			ID:             a.idGenerator(),
			Type:           events.EventV9AdapterInvoked,
			Timestamp:      now,
			CircleID:       envelope.ActorCircleID,
			IntersectionID: envelope.IntersectionID,
			SubjectID:      attemptID,
			SubjectType:    "execution_attempt",
			Provider:       a.provider,
			Metadata: map[string]string{
				"envelope_id": envelope.EnvelopeID,
				"action_hash": envelope.ActionHash,
				"amount":      fmt.Sprintf("%d", envelope.ActionSpec.AmountCents),
				"currency":    envelope.ActionSpec.Currency,
				"payee_id":    envelope.ActionSpec.PayeeID, // v9.10: PayeeID instead of free-text Recipient
				"approver_id": approval.ApproverID,
				"approval_id": approval.ArtifactID,
			},
		})
	}

	// CRITICAL: Always block execution
	// This is the guardrail that prevents money movement in v9 Slice 2
	blockReason := "guarded adapter: execution disabled in v9 Slice 2"

	// Emit blocked event
	if a.auditEmitter != nil {
		a.auditEmitter(events.Event{
			ID:             a.idGenerator(),
			Type:           events.EventV9AdapterBlocked,
			Timestamp:      now,
			CircleID:       envelope.ActorCircleID,
			IntersectionID: envelope.IntersectionID,
			SubjectID:      attemptID,
			SubjectType:    "execution_attempt",
			Provider:       a.provider,
			Metadata: map[string]string{
				"envelope_id":    envelope.EnvelopeID,
				"action_hash":    envelope.ActionHash,
				"blocked_reason": blockReason,
				"money_moved":    "false",
			},
		})
	}

	// Create the attempt record
	attempt := &ExecutionAttempt{
		AttemptID:     attemptID,
		EnvelopeID:    envelope.EnvelopeID,
		Provider:      a.provider,
		AttemptedAt:   now,
		Status:        AttemptBlocked,
		BlockedReason: blockReason,
		MoneyMoved:    false, // CRITICAL: Always false
	}

	// Return GuardedExecutionError
	// This is the EXPECTED error in v9 Slice 2
	return attempt, &GuardedExecutionError{
		EnvelopeID: envelope.EnvelopeID,
		Provider:   a.provider,
		Reason:     blockReason,
		BlockedAt:  now,
	}
}

// NewMockFinanceAdapter creates a guarded adapter for the mock-finance provider.
func NewMockFinanceAdapter(idGen func() string, emitter func(events.Event)) *GuardedExecutionAdapter {
	return NewGuardedExecutionAdapter(GuardedAdapterConfig{
		Provider:     "mock-finance",
		IDGenerator:  idGen,
		AuditEmitter: emitter,
	})
}

// NewPlaidStubAdapter creates a guarded adapter for Plaid (stub).
// CRITICAL: This is NOT a real Plaid adapter. It is a stub that CANNOT move money.
func NewPlaidStubAdapter(idGen func() string, emitter func(events.Event)) *GuardedExecutionAdapter {
	return NewGuardedExecutionAdapter(GuardedAdapterConfig{
		Provider:     "plaid-stub",
		IDGenerator:  idGen,
		AuditEmitter: emitter,
	})
}

// NewTrueLayerStubAdapter creates a guarded adapter for TrueLayer (stub).
// CRITICAL: This is NOT a real TrueLayer adapter. It is a stub that CANNOT move money.
func NewTrueLayerStubAdapter(idGen func() string, emitter func(events.Event)) *GuardedExecutionAdapter {
	return NewGuardedExecutionAdapter(GuardedAdapterConfig{
		Provider:     "truelayer-stub",
		IDGenerator:  idGen,
		AuditEmitter: emitter,
	})
}
