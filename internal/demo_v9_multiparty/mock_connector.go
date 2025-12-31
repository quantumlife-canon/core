// Package demo_v9_multiparty provides a mock write connector for v9.4 demos.
//
// CRITICAL: This mock connector does NOT move real money.
// It exists to demonstrate the multi-party execution pipeline when TrueLayer is not configured.
package demo_v9_multiparty

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/pkg/events"
)

// MockWriteConnector is a mock implementation of write.WriteConnector.
//
// CRITICAL: This connector does NOT move real money.
type MockWriteConnector struct {
	mu sync.RWMutex

	config           write.WriteConfig
	payeeRegistry    *write.PayeeRegistry
	abortedEnvelopes map[string]bool
	auditEmitter     func(event events.Event)
	idGenerator      func() string
}

// NewMockWriteConnector creates a new mock write connector.
func NewMockWriteConnector(idGen func() string, emitter func(event events.Event)) *MockWriteConnector {
	registry := write.NewPayeeRegistry()
	for _, payee := range write.SandboxPayees() {
		registry.Register(payee)
	}

	return &MockWriteConnector{
		config:           write.DefaultWriteConfig(),
		payeeRegistry:    registry,
		abortedEnvelopes: make(map[string]bool),
		auditEmitter:     emitter,
		idGenerator:      idGen,
	}
}

// Provider returns the provider name.
func (c *MockWriteConnector) Provider() string {
	return "mock-write"
}

// Prepare validates that the payment can be executed.
func (c *MockWriteConnector) Prepare(ctx context.Context, req write.PrepareRequest) (*write.PrepareResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := &write.PrepareResult{
		Valid:             true,
		ValidationDetails: make([]write.ValidationDetail, 0),
		PreparedAt:        now,
	}

	// Check envelope
	if req.Envelope == nil {
		result.Valid = false
		result.InvalidReason = "envelope is nil"
		return result, nil
	}

	if req.Envelope.SealHash == "" {
		result.Valid = false
		result.InvalidReason = "envelope is not sealed"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_sealed",
		Passed:  true,
		Details: "sealed",
	})

	// Check expiry
	if now.After(req.Envelope.Expiry) {
		result.Valid = false
		result.InvalidReason = "envelope has expired"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_not_expired",
		Passed:  true,
		Details: "not expired",
	})

	// Check revoked
	if req.Envelope.Revoked {
		result.Valid = false
		result.InvalidReason = "envelope has been revoked"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_not_revoked",
		Passed:  true,
		Details: "not revoked",
	})

	// Check revocation window
	if !req.Envelope.RevocationWaived && now.Before(req.Envelope.RevocationWindowEnd) {
		result.Valid = false
		result.InvalidReason = "revocation window is still active"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "revocation_window_closed",
		Passed:  true,
		Details: "closed or waived",
	})

	// Check approval
	if req.Approval == nil {
		result.Valid = false
		result.InvalidReason = "approval required"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_exists",
		Passed:  true,
		Details: fmt.Sprintf("artifact %s", req.Approval.ArtifactID),
	})

	// Check approval expiry
	if req.Approval.IsExpired(now) {
		result.Valid = false
		result.InvalidReason = "approval has expired"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_not_expired",
		Passed:  true,
		Details: "not expired",
	})

	// Check action hash match
	if req.Approval.ActionHash != req.Envelope.ActionHash {
		result.Valid = false
		result.InvalidReason = "approval action hash mismatch"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_hash_matches",
		Passed:  true,
		Details: "hash matches",
	})

	// Check cap
	if req.Envelope.ActionSpec.AmountCents > c.config.CapCents {
		result.Valid = false
		result.InvalidReason = fmt.Sprintf("amount %d exceeds cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "amount_within_cap",
		Passed:  true,
		Details: fmt.Sprintf("%d <= %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents),
	})

	// Check payee
	_, payeeExists := c.payeeRegistry.Get(req.PayeeID)
	if !payeeExists {
		result.Valid = false
		result.InvalidReason = "payee not found"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "payee_exists",
		Passed:  true,
		Details: fmt.Sprintf("payee %s", req.PayeeID),
	})

	// Check abort
	c.mu.RLock()
	aborted := c.abortedEnvelopes[req.Envelope.EnvelopeID]
	c.mu.RUnlock()
	if aborted {
		result.Valid = false
		result.InvalidReason = "execution was aborted"
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "not_aborted",
		Passed:  true,
		Details: "not aborted",
	})

	// Emit prepare event
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:             c.idGenerator(),
			Type:           events.EventV9AdapterPrepared,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       "mock-write",
			Metadata: map[string]string{
				"amount":   fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"currency": req.Envelope.ActionSpec.Currency,
				"payee_id": req.PayeeID,
				"valid":    "true",
			},
		})
	}

	return result, nil
}

// Execute creates a mock payment receipt.
//
// CRITICAL: This does NOT move real money.
func (c *MockWriteConnector) Execute(ctx context.Context, req write.ExecuteRequest) (*write.PaymentReceipt, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Check abort
	c.mu.RLock()
	aborted := c.abortedEnvelopes[req.Envelope.EnvelopeID]
	c.mu.RUnlock()
	if aborted {
		return nil, write.ErrExecutionAborted
	}

	// Emit invocation event
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:             c.idGenerator(),
			Type:           events.EventV9AdapterInvoked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       "mock-write",
			Metadata: map[string]string{
				"amount":          fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"currency":        req.Envelope.ActionSpec.Currency,
				"payee_id":        req.PayeeID,
				"idempotency_key": req.IdempotencyKey,
			},
		})
	}

	// Simulate forced pause
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(c.config.ForcedPauseDuration):
		// Continue
	}

	// Check abort after pause
	c.mu.RLock()
	aborted = c.abortedEnvelopes[req.Envelope.EnvelopeID]
	c.mu.RUnlock()
	if aborted {
		return nil, write.ErrExecutionAborted
	}

	// Create mock receipt
	// CRITICAL: Simulated=true and Status=PaymentSimulated because no real money moved.
	receiptID := c.idGenerator()
	receipt := &write.PaymentReceipt{
		ReceiptID:   receiptID,
		EnvelopeID:  req.Envelope.EnvelopeID,
		ProviderRef: fmt.Sprintf("mock-%s", receiptID),
		Status:      write.PaymentSimulated, // NOT succeeded - no real money moved
		AmountCents: req.Envelope.ActionSpec.AmountCents,
		Currency:    req.Envelope.ActionSpec.Currency,
		PayeeID:     req.PayeeID,
		CreatedAt:   now,
		CompletedAt: time.Now(),
		ProviderMetadata: map[string]string{
			"mock":      "true",
			"sandbox":   "true",
			"provider":  "mock-write",
			"simulated": "true",
		},
		Simulated: true, // CRITICAL: This is simulated, no external side-effect
	}

	// Emit simulated event (NOT succeeded - that would be false)
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:             c.idGenerator(),
			Type:           events.EventV9PaymentSimulated, // NOT EventV9PaymentCreated
			Timestamp:      receipt.CompletedAt,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      receiptID,
			SubjectType:    "receipt",
			Provider:       "mock-write",
			Metadata: map[string]string{
				"envelope_id":  req.Envelope.EnvelopeID,
				"provider_ref": receipt.ProviderRef,
				"amount":       fmt.Sprintf("%d", receipt.AmountCents),
				"currency":     receipt.Currency,
				"status":       string(receipt.Status),
				"simulated":    "true",
				"money_moved":  "false",
			},
		})
	}

	return receipt, nil
}

// Abort cancels execution if possible.
func (c *MockWriteConnector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.abortedEnvelopes[envelopeID] = true

	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:          c.idGenerator(),
			Type:        events.EventV9ExecutionAborted,
			Timestamp:   time.Now(),
			SubjectID:   envelopeID,
			SubjectType: "envelope",
			Provider:    "mock-write",
			Metadata: map[string]string{
				"reason": "multi-party abort requested",
			},
		})
	}

	return true, nil
}

// Verify interface compliance.
var _ write.WriteConnector = (*MockWriteConnector)(nil)
