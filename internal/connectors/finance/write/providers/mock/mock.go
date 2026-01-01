// Package mock provides a deterministic mock finance write connector.
//
// CRITICAL: This connector NEVER moves real money.
// It provides deterministic responses for testing and development.
//
// GUARANTEES:
// - Idempotent: same request produces same result
// - Deterministic: no randomness (clock injected)
// - No goroutines
// - No time.Now() calls
//
// Reference: docs/ADR/ADR-0033-phase17-finance-execution-boundary.md
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/payees"
	"quantumlife/pkg/events"
)

// Connector is a deterministic mock write connector.
// CRITICAL: This connector NEVER moves real money.
type Connector struct {
	// config is the write configuration.
	config write.WriteConfig

	// payeeRegistry validates payees.
	payeeRegistry payees.Registry

	// executedPayments tracks executed payments by idempotency key.
	executedPayments map[string]*write.PaymentReceipt

	// abortedEnvelopes tracks aborted envelopes.
	abortedEnvelopes map[string]bool

	// eventEmitter emits audit events.
	eventEmitter events.Emitter

	// clock provides current time (injected).
	clock func() time.Time

	// idGenerator generates deterministic IDs.
	idGenerator func(input string) string
}

// ConnectorOption configures the mock connector.
type ConnectorOption func(*Connector)

// WithConfig sets the write configuration.
func WithConfig(config write.WriteConfig) ConnectorOption {
	return func(c *Connector) {
		c.config = config
	}
}

// WithPayeeRegistry sets the payee registry.
func WithPayeeRegistry(registry payees.Registry) ConnectorOption {
	return func(c *Connector) {
		c.payeeRegistry = registry
	}
}

// WithEventEmitter sets the event emitter.
func WithEventEmitter(emitter events.Emitter) ConnectorOption {
	return func(c *Connector) {
		c.eventEmitter = emitter
	}
}

// WithClock sets the clock function.
func WithClock(clock func() time.Time) ConnectorOption {
	return func(c *Connector) {
		c.clock = clock
	}
}

// NewConnector creates a new mock write connector.
func NewConnector(opts ...ConnectorOption) *Connector {
	c := &Connector{
		config:           write.DefaultWriteConfig(),
		payeeRegistry:    payees.NewDefaultRegistry(),
		executedPayments: make(map[string]*write.PaymentReceipt),
		abortedEnvelopes: make(map[string]bool),
		clock:            time.Now,
		idGenerator:      defaultIDGenerator,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Provider returns the provider name (legacy).
func (c *Connector) Provider() string {
	return "mock"
}

// ProviderID returns the canonical provider identifier.
// v9.9: Returns "mock-write" for registry enforcement.
func (c *Connector) ProviderID() string {
	return "mock-write"
}

// ProviderInfo returns the provider identifier and environment.
func (c *Connector) ProviderInfo() (string, string) {
	return "mock-write", "mock"
}

// Prepare validates that the payment can be executed.
// CRITICAL: This performs ALL validation BEFORE any money moves.
func (c *Connector) Prepare(ctx context.Context, req write.PrepareRequest) (*write.PrepareResult, error) {
	now := req.Now
	if now.IsZero() {
		now = c.clock()
	}

	result := &write.PrepareResult{
		Valid:             true,
		ValidationDetails: make([]write.ValidationDetail, 0),
		PreparedAt:        now,
	}

	// Check 1: Envelope exists and is sealed
	if req.Envelope == nil {
		result.Valid = false
		result.InvalidReason = "envelope is nil"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_exists",
			Passed:  false,
			Details: "envelope is nil",
		})
		return result, nil
	}

	if req.Envelope.SealHash == "" {
		result.Valid = false
		result.InvalidReason = "envelope is not sealed"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_sealed",
			Passed:  false,
			Details: "envelope missing seal hash",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_sealed",
		Passed:  true,
		Details: truncateHash(req.Envelope.SealHash),
	})

	// Check 2: Envelope not expired
	if now.After(req.Envelope.Expiry) {
		result.Valid = false
		result.InvalidReason = "envelope has expired"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_not_expired",
			Passed:  false,
			Details: fmt.Sprintf("expired at %s", req.Envelope.Expiry.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_not_expired",
		Passed:  true,
		Details: fmt.Sprintf("expires at %s", req.Envelope.Expiry.Format(time.RFC3339)),
	})

	// Check 3: Envelope not revoked
	if req.Envelope.Revoked {
		result.Valid = false
		result.InvalidReason = "envelope has been revoked"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_not_revoked",
			Passed:  false,
			Details: fmt.Sprintf("revoked by %s", req.Envelope.RevokedBy),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_not_revoked",
		Passed:  true,
		Details: "no revocation",
	})

	// Check 4: Revocation window has closed (or explicitly waived)
	if !req.Envelope.RevocationWaived && now.Before(req.Envelope.RevocationWindowEnd) {
		result.Valid = false
		result.InvalidReason = "revocation window is still active"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "revocation_window_closed",
			Passed:  false,
			Details: fmt.Sprintf("window ends at %s", req.Envelope.RevocationWindowEnd.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "revocation_window_closed",
		Passed:  true,
		Details: "window closed or waived",
	})

	// Check 5: Approval exists
	if req.Approval == nil {
		result.Valid = false
		result.InvalidReason = "explicit approval required"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "approval_exists",
			Passed:  false,
			Details: "no approval artifact provided",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_exists",
		Passed:  true,
		Details: truncateID(req.Approval.ArtifactID),
	})

	// Check 6: Approval not expired
	if req.Approval.IsExpired(now) {
		result.Valid = false
		result.InvalidReason = "approval has expired"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "approval_not_expired",
			Passed:  false,
			Details: fmt.Sprintf("expired at %s", req.Approval.ExpiresAt.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_not_expired",
		Passed:  true,
		Details: fmt.Sprintf("expires at %s", req.Approval.ExpiresAt.Format(time.RFC3339)),
	})

	// Check 7: Approval action hash matches envelope
	if req.Approval.ActionHash != req.Envelope.ActionHash {
		result.Valid = false
		result.InvalidReason = "approval action hash does not match envelope"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "approval_hash_matches",
			Passed:  false,
			Details: "action hash mismatch",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_hash_matches",
		Passed:  true,
		Details: truncateHash(req.Envelope.ActionHash),
	})

	// Check 8: Amount does not exceed cap
	if req.Envelope.ActionSpec.AmountCents > c.config.CapCents {
		result.Valid = false
		result.InvalidReason = fmt.Sprintf("amount %d exceeds hard cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents)
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "amount_within_cap",
			Passed:  false,
			Details: fmt.Sprintf("amount %d > cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "amount_within_cap",
		Passed:  true,
		Details: fmt.Sprintf("amount %d <= cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents),
	})

	// Check 9: Currency is allowed
	currencyAllowed := false
	for _, allowed := range c.config.AllowedCurrencies {
		if req.Envelope.ActionSpec.Currency == allowed {
			currencyAllowed = true
			break
		}
	}
	if !currencyAllowed {
		result.Valid = false
		result.InvalidReason = fmt.Sprintf("currency %s not allowed", req.Envelope.ActionSpec.Currency)
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "currency_allowed",
			Passed:  false,
			Details: fmt.Sprintf("currency %s not in allowed list", req.Envelope.ActionSpec.Currency),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "currency_allowed",
		Passed:  true,
		Details: fmt.Sprintf("currency %s is allowed", req.Envelope.ActionSpec.Currency),
	})

	// Check 10: Payee is pre-defined and allowed
	if err := c.payeeRegistry.RequireAllowed(payees.PayeeID(req.PayeeID), c.ProviderID()); err != nil {
		result.Valid = false
		result.InvalidReason = err.Error()
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "payee_allowed",
			Passed:  false,
			Details: err.Error(),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "payee_allowed",
		Passed:  true,
		Details: fmt.Sprintf("payee %s is allowed", req.PayeeID),
	})

	// Check 11: Not aborted
	if c.abortedEnvelopes[req.Envelope.EnvelopeID] {
		result.Valid = false
		result.InvalidReason = "execution was aborted"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "not_aborted",
			Passed:  false,
			Details: "envelope was aborted before execution",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "not_aborted",
		Passed:  true,
		Details: "not aborted",
	})

	// Emit prepare event
	c.emit(events.Event{
		Type:      events.Phase17FinanceAdapterPrepared,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id": req.Envelope.EnvelopeID,
			"amount":      fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
			"currency":    req.Envelope.ActionSpec.Currency,
			"payee_id":    req.PayeeID,
			"valid":       fmt.Sprintf("%t", result.Valid),
		},
	})

	return result, nil
}

// Execute simulates payment execution.
// CRITICAL: This NEVER moves real money. Always returns Simulated=true.
func (c *Connector) Execute(ctx context.Context, req write.ExecuteRequest) (*write.PaymentReceipt, error) {
	now := req.Now
	if now.IsZero() {
		now = c.clock()
	}

	// Check idempotency - return same result for same key
	if existing, ok := c.executedPayments[req.IdempotencyKey]; ok {
		c.emit(events.Event{
			Type:      events.Phase17FinanceIdempotencyReplayBlocked,
			Timestamp: now,
			Metadata: map[string]string{
				"envelope_id":     req.Envelope.EnvelopeID,
				"idempotency_key": truncateKey(req.IdempotencyKey),
				"existing_ref":    existing.ProviderRef,
			},
		})
		return existing, nil
	}

	// Check abort status
	if c.abortedEnvelopes[req.Envelope.EnvelopeID] {
		return nil, write.ErrExecutionAborted
	}

	// Emit invocation event
	c.emit(events.Event{
		Type:      events.Phase17FinanceAdapterInvoked,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id":     req.Envelope.EnvelopeID,
			"amount":          fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
			"currency":        req.Envelope.ActionSpec.Currency,
			"payee_id":        req.PayeeID,
			"idempotency_key": truncateKey(req.IdempotencyKey),
		},
	})

	// Generate deterministic receipt ID
	receiptID := c.idGenerator(fmt.Sprintf("receipt|%s|%s", req.Envelope.EnvelopeID, req.IdempotencyKey))
	providerRef := c.idGenerator(fmt.Sprintf("mock-ref|%s|%d", req.Envelope.EnvelopeID, now.Unix()))

	// Create simulated receipt
	receipt := &write.PaymentReceipt{
		ReceiptID:   receiptID,
		EnvelopeID:  req.Envelope.EnvelopeID,
		ProviderRef: providerRef,
		Status:      write.PaymentSimulated,
		AmountCents: req.Envelope.ActionSpec.AmountCents,
		Currency:    req.Envelope.ActionSpec.Currency,
		PayeeID:     req.PayeeID,
		CreatedAt:   now,
		CompletedAt: now,
		ProviderMetadata: map[string]string{
			"mock":            "true",
			"idempotency_key": req.IdempotencyKey,
			"action_hash":     truncateHash(req.Envelope.ActionHash),
		},
		Simulated: true, // CRITICAL: Always true for mock connector
	}

	// Store for idempotency
	c.executedPayments[req.IdempotencyKey] = receipt

	// Emit success event
	c.emit(events.Event{
		Type:      events.Phase17FinanceExecutionSucceeded,
		Timestamp: now,
		Metadata: map[string]string{
			"envelope_id":  req.Envelope.EnvelopeID,
			"receipt_id":   receiptID,
			"provider_ref": providerRef,
			"amount":       fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
			"currency":     req.Envelope.ActionSpec.Currency,
			"simulated":    "true",
			"money_moved":  "false",
		},
	})

	return receipt, nil
}

// Abort cancels execution before provider call.
func (c *Connector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	c.abortedEnvelopes[envelopeID] = true

	c.emit(events.Event{
		Type:      events.Phase17FinanceExecutionAborted,
		Timestamp: c.clock(),
		Metadata: map[string]string{
			"envelope_id": envelopeID,
			"reason":      "abort requested",
		},
	})

	return true, nil
}

// GetExecutedPayments returns all executed payments (for testing).
func (c *Connector) GetExecutedPayments() map[string]*write.PaymentReceipt {
	result := make(map[string]*write.PaymentReceipt)
	for k, v := range c.executedPayments {
		result[k] = v
	}
	return result
}

// Reset clears all state (for testing).
func (c *Connector) Reset() {
	c.executedPayments = make(map[string]*write.PaymentReceipt)
	c.abortedEnvelopes = make(map[string]bool)
}

func (c *Connector) emit(event events.Event) {
	if c.eventEmitter != nil {
		c.eventEmitter.Emit(event)
	}
}

// defaultIDGenerator generates deterministic IDs from input.
func defaultIDGenerator(input string) string {
	hash := sha256.Sum256([]byte(input))
	return "mock_" + hex.EncodeToString(hash[:8])
}

func truncateHash(h string) string {
	if len(h) > 16 {
		return h[:16] + "..."
	}
	return h
}

func truncateID(id string) string {
	if len(id) > 20 {
		return id[:20] + "..."
	}
	return id
}

func truncateKey(key string) string {
	if len(key) > 16 {
		return key[:16] + "..."
	}
	return key
}

// AllowedPayeeIDs returns allowed payee IDs (for policy snapshot).
func (c *Connector) AllowedPayeeIDs() []string {
	return c.payeeRegistry.AllowedPayeeIDs()
}

// BlockedPayeeIDs returns blocked payee IDs (for policy snapshot).
func (c *Connector) BlockedPayeeIDs() []string {
	return c.payeeRegistry.BlockedPayeeIDs()
}

// Verify interface compliance.
var _ write.WriteConnector = (*Connector)(nil)

// PayeeRegistry interface for policy snapshot support.
type PayeeDescriptor interface {
	AllowedPayeeIDs() []string
	BlockedPayeeIDs() []string
}

var _ PayeeDescriptor = (*Connector)(nil)

// SortedStrings returns sorted strings (stdlib-only bubble sort).
func SortedStrings(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// CanonicalPayeeList returns a canonical string representation of payee IDs.
func CanonicalPayeeList(ids []string) string {
	sorted := SortedStrings(ids)
	return strings.Join(sorted, ",")
}
