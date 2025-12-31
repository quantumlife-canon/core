// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.6 Executor with idempotency and replay defense.
//
// CRITICAL: This executor extends v9.5 with:
// 1) Deterministic idempotency key derived from envelope + action hash + attempt ID
// 2) Attempt ledger preventing replays of terminal attempts
// 3) One in-flight attempt per envelope policy
// 4) Provider idempotency key propagation
// 5) Exactly-once trace finalization per attempt
//
// NON-NEGOTIABLE INVARIANTS (all v9.3/v9.4/v9.5 constraints remain):
// - No blanket/standing approvals
// - Neutral approval language
// - Symmetry - every approver receives IDENTICAL payload
// - Approvals do NOT bypass revocation windows
// - Single-use approvals only
// - Mock providers MUST report MoneyMoved=false
// - Cap: £1.00 (100 pence)
// - Pre-defined payees only
// - Forced pause before provider call
// - No retries - failures require new approvals
// - Bundle MUST be presented before approval accepted
// - Revocation during pause MUST abort BEFORE provider call
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/payees"
	"quantumlife/internal/connectors/finance/write/registry"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/events"
)

// V96Executor executes financial payments with idempotency and replay defense.
//
// CRITICAL: This executor adds:
// - Attempt ledger for replay prevention
// - Deterministic idempotency key derivation
// - One in-flight attempt per envelope enforcement
// - v9.9: Provider registry allowlist enforcement
// - v9.10: Payee registry enforcement (no free-text recipients)
type V96Executor struct {
	mu sync.RWMutex

	// Connectors
	trueLayerConnector write.WriteConnector
	mockConnector      write.WriteConnector

	// Config
	config V96ExecutorConfig

	// Components
	presentationGate  *PresentationGate
	multiPartyGate    *MultiPartyGate
	approvalVerifier  *ApprovalVerifier
	revocationChecker *RevocationChecker
	attemptLedger     attempts.AttemptLedger

	// v9.9: Provider registry for allowlist enforcement
	providerRegistry registry.Registry

	// v9.10: Payee registry for free-text recipient elimination
	payeeRegistry payees.Registry

	// State
	abortedEnvelopes map[string]bool
	auditEmitter     func(event events.Event)
	idGenerator      func() string
}

// V96ExecutorConfig configures the v9.6 executor.
type V96ExecutorConfig struct {
	// CapCents is the hard cap in cents.
	CapCents int64

	// AllowedCurrencies is the list of allowed currencies.
	AllowedCurrencies []string

	// ForcedPauseDuration is the mandatory pause before execution.
	ForcedPauseDuration time.Duration

	// RequireExplicitApproval requires explicit --approve flag.
	RequireExplicitApproval bool

	// TrueLayerConfigured indicates if TrueLayer credentials are available.
	TrueLayerConfigured bool

	// TrueLayerEnvironment is the TrueLayer environment (sandbox/live).
	TrueLayerEnvironment string

	// PresentationExpiryDuration is how long a presentation remains valid.
	PresentationExpiryDuration time.Duration

	// RevocationPollInterval is how often to check for revocation during pause.
	RevocationPollInterval time.Duration
}

// DefaultV96ExecutorConfig returns the default configuration.
func DefaultV96ExecutorConfig() V96ExecutorConfig {
	return V96ExecutorConfig{
		CapCents:                   write.DefaultCapCents,
		AllowedCurrencies:          []string{"GBP"},
		ForcedPauseDuration:        2 * time.Second,
		RequireExplicitApproval:    true,
		TrueLayerConfigured:        false,
		TrueLayerEnvironment:       "sandbox",
		PresentationExpiryDuration: 5 * time.Minute,
		RevocationPollInterval:     100 * time.Millisecond,
	}
}

// NewV96Executor creates a new v9.6 executor.
// v9.9: Uses the default provider registry for allowlist enforcement.
// v9.10: Uses the default payee registry for free-text recipient elimination.
func NewV96Executor(
	trueLayerConnector write.WriteConnector,
	mockConnector write.WriteConnector,
	presentationGate *PresentationGate,
	multiPartyGate *MultiPartyGate,
	approvalVerifier *ApprovalVerifier,
	revocationChecker *RevocationChecker,
	attemptLedger attempts.AttemptLedger,
	config V96ExecutorConfig,
	idGen func() string,
	emitter func(event events.Event),
) *V96Executor {
	return &V96Executor{
		trueLayerConnector: trueLayerConnector,
		mockConnector:      mockConnector,
		config:             config,
		presentationGate:   presentationGate,
		multiPartyGate:     multiPartyGate,
		approvalVerifier:   approvalVerifier,
		revocationChecker:  revocationChecker,
		attemptLedger:      attemptLedger,
		providerRegistry:   registry.NewDefaultRegistry(), // v9.9: Default registry
		payeeRegistry:      payees.NewDefaultRegistry(),   // v9.10: Default payee registry
		abortedEnvelopes:   make(map[string]bool),
		auditEmitter:       emitter,
		idGenerator:        idGen,
	}
}

// SetProviderRegistry sets a custom provider registry.
// This is primarily for testing - production code should use NewDefaultRegistry().
func (e *V96Executor) SetProviderRegistry(reg registry.Registry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providerRegistry = reg
}

// SetPayeeRegistry sets a custom payee registry.
// This is primarily for testing - production code should use NewDefaultRegistry().
func (e *V96Executor) SetPayeeRegistry(reg payees.Registry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.payeeRegistry = reg
}

// V96ExecuteRequest contains parameters for v9.6 execution.
type V96ExecuteRequest struct {
	// Envelope is the sealed execution envelope.
	Envelope *ExecutionEnvelope

	// Bundle is the approval bundle (for multi-party).
	Bundle *ApprovalBundle

	// Approvals are the multi-party approval artifacts.
	Approvals []MultiPartyApprovalArtifact

	// ApproverHashes are the hashes each approver received (for symmetry proof).
	ApproverHashes []ApproverBundleHash

	// Policy is the multi-party policy from intersection contract.
	Policy *MultiPartyPolicy

	// PayeeID is the pre-defined payee identifier.
	PayeeID string

	// ExplicitApprove indicates the user passed --approve flag.
	ExplicitApprove bool

	// TraceID is the execution trace ID.
	TraceID string

	// AttemptID is the explicit attempt ID (optional - will be generated if empty).
	// CRITICAL: Providing the same attempt ID for the same envelope blocks replay.
	AttemptID string

	// Now is the current time.
	Now time.Time
}

// V96ExecuteResult contains the result of v9.6 execution.
type V96ExecuteResult struct {
	// Success indicates if execution succeeded.
	Success bool

	// Receipt is the payment receipt (if successful).
	Receipt *write.PaymentReceipt

	// Status is the settlement status.
	Status SettlementStatus

	// BlockedReason explains why execution was blocked.
	BlockedReason string

	// ValidationDetails contains all validation checks.
	ValidationDetails []ValidationCheckResult

	// GateResult contains multi-party gate verification result.
	GateResult *MultiPartyGateResult

	// PresentationResult contains presentation verification result.
	PresentationResult *AllPresentationsResult

	// ProviderUsed indicates which provider was used.
	ProviderUsed string

	// AttemptID uniquely identifies this execution attempt.
	AttemptID string

	// IdempotencyKey is the derived key (prefix only for safety).
	IdempotencyKeyPrefix string

	// AttemptRecord is the ledger record for this attempt.
	AttemptRecord *attempts.AttemptRecord

	// ReplayBlocked indicates if this was a blocked replay.
	ReplayBlocked bool

	// InflightBlocked indicates if blocked due to in-flight attempt.
	InflightBlocked bool

	// AuditEvents contains all audit events.
	AuditEvents []events.Event

	// MoneyMoved indicates if any money was moved.
	MoneyMoved bool

	// CompletedAt is when execution completed.
	CompletedAt time.Time
}

// Execute performs the full v9.6 execution pipeline with idempotency.
//
// CRITICAL: This pipeline adds:
// 1) Attempt ledger start (blocks replays and in-flight duplicates)
// 2) Idempotency key derivation
// 3) All v9.5 validations and gates
// 4) Provider call with idempotency key
// 5) Attempt ledger finalization
func (e *V96Executor) Execute(ctx context.Context, req V96ExecuteRequest) (*V96ExecuteResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := &V96ExecuteResult{
		ValidationDetails: make([]ValidationCheckResult, 0),
		AuditEvents:       make([]events.Event, 0),
		MoneyMoved:        false,
		CompletedAt:       now,
	}

	// Step 0: Generate or validate attempt ID
	attemptID := req.AttemptID
	if attemptID == "" {
		// Generate new attempt ID
		attemptID = attempts.DeriveAttemptID(req.Envelope.EnvelopeID, 1)
	}
	result.AttemptID = attemptID

	// Step 1: Derive idempotency key
	idempotencyKey := attempts.DeriveIdempotencyKey(attempts.IdempotencyKeyInput{
		EnvelopeID: req.Envelope.EnvelopeID,
		ActionHash: req.Envelope.ActionHash,
		AttemptID:  attemptID,
		SealHash:   req.Envelope.SealHash,
	})
	result.IdempotencyKeyPrefix = attempts.IdempotencyKeyPrefix(idempotencyKey)

	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV96IdempotencyKeyDerived,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      attemptID,
		SubjectType:    "attempt",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"envelope_id":        req.Envelope.EnvelopeID,
			"idempotency_prefix": result.IdempotencyKeyPrefix,
		},
	})

	// Step 2: Select provider
	provider, providerName := e.selectProvider()
	result.ProviderUsed = providerName

	// Step 2.5 (v9.9): Check provider registry allowlist
	// CRITICAL: Block execution BEFORE ledger entry if provider is not allowed.
	// This prevents unapproved providers from being used for financial execution.
	providerID := registry.ProviderID(providerName)
	if err := e.providerRegistry.RequireAllowed(providerID); err != nil {
		// Emit provider blocked event
		blockReason := "provider not allowed"
		var providerErr *registry.ProviderError
		if errors.As(err, &providerErr) {
			if providerErr.BlockReason != "" {
				blockReason = providerErr.BlockReason
			}
		}

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV99ProviderBlocked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"provider_id":  string(providerID),
				"attempt_id":   attemptID,
				"block_reason": blockReason,
				"error":        err.Error(),
			},
		})

		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("provider %q blocked: %s", providerID, blockReason)
		return result, nil
	}

	// Emit provider allowed event
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV99ProviderAllowed,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"provider_id": string(providerID),
			"attempt_id":  attemptID,
		},
	})

	// Step 2.6 (v9.10): Check payee registry allowlist
	// CRITICAL: Block execution BEFORE ledger entry if payee is not allowed.
	// This prevents free-text recipients from being used in execution.
	payeeID := payees.PayeeID(req.PayeeID)
	if err := e.payeeRegistry.RequireAllowed(payeeID, providerName); err != nil {
		// Emit payee blocked event
		blockReason := "payee not allowed"
		var payeeErr *payees.PayeeError
		if errors.As(err, &payeeErr) {
			if payeeErr.BlockReason != "" {
				blockReason = payeeErr.BlockReason
			}
		}

		// Determine specific event type based on error
		eventType := events.EventV910PayeeNotAllowed
		if errors.Is(err, payees.ErrPayeeNotRegistered) {
			eventType = events.EventV910PayeeNotRegistered
		} else if errors.Is(err, payees.ErrPayeeLiveBlocked) {
			eventType = events.EventV910PayeeLiveBlocked
		} else if errors.Is(err, payees.ErrPayeeProviderMismatch) {
			eventType = events.EventV910PayeeProviderMismatch
		}

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           eventType,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"payee_id":     string(payeeID),
				"provider_id":  providerName,
				"attempt_id":   attemptID,
				"block_reason": blockReason,
				"error":        err.Error(),
			},
		})

		// Emit execution blocked event
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV910ExecutionBlockedInvalidPayee,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"payee_id":     string(payeeID),
				"provider_id":  providerName,
				"block_reason": blockReason,
			},
		})

		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("payee %q blocked: %s", payeeID, blockReason)
		return result, nil
	}

	// Emit payee allowed event
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV910PayeeAllowed,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"payee_id":    string(payeeID),
			"provider_id": providerName,
			"attempt_id":  attemptID,
		},
	})

	// Step 3: Start attempt in ledger (blocks replays and in-flight duplicates)
	attemptRecord, err := e.attemptLedger.StartAttempt(attempts.StartAttemptRequest{
		AttemptID:      attemptID,
		EnvelopeID:     req.Envelope.EnvelopeID,
		ActionHash:     req.Envelope.ActionHash,
		IdempotencyKey: idempotencyKey,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		TraceID:        req.TraceID,
		Provider:       providerName,
		Now:            now,
	})

	if err != nil {
		return e.handleLedgerError(result, req, err, now)
	}

	result.AttemptRecord = attemptRecord

	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV96AttemptStarted,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      attemptID,
		SubjectType:    "attempt",
		TraceID:        req.TraceID,
		Provider:       providerName,
		Metadata: map[string]string{
			"envelope_id":        req.Envelope.EnvelopeID,
			"idempotency_prefix": result.IdempotencyKeyPrefix,
		},
	})

	// Step 4: Provider selection event
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV95ExecutionProviderSelected,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"provider":             providerName,
			"truelayer_configured": fmt.Sprintf("%t", e.config.TrueLayerConfigured),
			"environment":          e.config.TrueLayerEnvironment,
		},
	})

	// Step 5: Emit execution started
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ExecutionStarted,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"provider":           providerName,
			"payee_id":           req.PayeeID,
			"multi_party":        fmt.Sprintf("%t", req.Policy.IsMultiParty()),
			"attempt_id":         attemptID,
			"idempotency_prefix": result.IdempotencyKeyPrefix,
		},
	})

	// Step 6: Check explicit approval flag
	if e.config.RequireExplicitApproval && !req.ExplicitApprove {
		return e.finalizeBlocked(result, req, attemptID, "explicit --approve flag required",
			SettlementBlocked, now, "explicit_approve_flag", "command must include --approve flag")
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "explicit_approve_flag",
		Passed:  true,
		Details: "--approve flag provided",
	})

	// Step 7: Check abort status
	e.mu.RLock()
	aborted := e.abortedEnvelopes[req.Envelope.EnvelopeID]
	e.mu.RUnlock()
	if aborted {
		return e.finalizeBlocked(result, req, attemptID, "execution was aborted",
			SettlementAborted, now, "not_aborted", "envelope was aborted before execution")
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "not_aborted",
		Passed:  true,
		Details: "not aborted",
	})

	// Step 8: PRESENTATION GATE
	if req.Policy.IsMultiParty() && req.Policy.AppliesToFinanceWrite() {
		presentationResult, err := e.presentationGate.VerifyAllPresentations(
			req.Approvals,
			req.Bundle,
			req.Envelope,
			now,
		)
		if err != nil {
			return e.finalizeBlocked(result, req, attemptID,
				fmt.Sprintf("presentation verification error: %v", err),
				SettlementBlocked, now, "presentation_verified", err.Error())
		}

		result.PresentationResult = presentationResult

		if !presentationResult.AllVerified {
			reason := fmt.Sprintf("presentation missing for approvers: %v", presentationResult.MissingApprovers)
			return e.finalizeBlocked(result, req, attemptID, reason,
				SettlementBlocked, now, "presentation_verified", presentationResult.BlockedReason)
		}

		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "presentation_verified",
			Passed:  true,
			Details: fmt.Sprintf("all %d approvers received bundle", len(presentationResult.VerifiedApprovers)),
		})
	}

	// Step 9: MULTI-PARTY GATE
	if req.Policy.IsMultiParty() && req.Policy.AppliesToFinanceWrite() {
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV94MultiPartyRequired,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"threshold":          fmt.Sprintf("%d", req.Policy.Threshold),
				"required_approvers": fmt.Sprintf("%v", req.Policy.RequiredApprovers),
			},
		})

		gateResult, err := e.multiPartyGate.Verify(ctx, MultiPartyGateRequest{
			Envelope:       req.Envelope,
			Bundle:         req.Bundle,
			Approvals:      req.Approvals,
			ApproverHashes: req.ApproverHashes,
			Policy:         req.Policy,
			Now:            now,
		})
		if err != nil {
			return e.finalizeBlocked(result, req, attemptID,
				fmt.Sprintf("multi-party gate error: %v", err),
				SettlementBlocked, now, "multi_party_gate", err.Error())
		}

		result.GateResult = gateResult

		if !gateResult.Passed {
			return e.finalizeBlocked(result, req, attemptID, gateResult.BlockedReason,
				SettlementBlocked, now, "multi_party_gate", gateResult.BlockedReason)
		}

		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "multi_party_gate",
			Passed:  true,
			Details: fmt.Sprintf("threshold met: %d/%d", len(gateResult.VerifiedApprovals), req.Policy.Threshold),
		})
	}

	// Step 10: Cap enforcement
	if req.Envelope.ActionSpec.AmountCents > e.config.CapCents {
		reason := fmt.Sprintf("amount %d exceeds cap %d", req.Envelope.ActionSpec.AmountCents, e.config.CapCents)
		return e.finalizeBlocked(result, req, attemptID, reason,
			SettlementBlocked, now, "amount_within_cap", reason)
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "amount_within_cap",
		Passed:  true,
		Details: fmt.Sprintf("amount %d <= cap %d", req.Envelope.ActionSpec.AmountCents, e.config.CapCents),
	})

	// Step 11: Check revocation
	if e.revocationChecker.IsRevoked(req.Envelope.EnvelopeID) {
		return e.finalizeBlocked(result, req, attemptID, "envelope was revoked",
			SettlementRevoked, now, "not_revoked", "envelope was revoked")
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "not_revoked",
		Passed:  true,
		Details: "no revocation",
	})

	// Step 12: Check revocation window
	if !req.Envelope.RevocationWaived && now.Before(req.Envelope.RevocationWindowEnd) {
		reason := fmt.Sprintf("revocation window open until %s", req.Envelope.RevocationWindowEnd.Format(time.RFC3339))
		return e.finalizeBlocked(result, req, attemptID, reason,
			SettlementBlocked, now, "revocation_window_closed", reason)
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "revocation_window_closed",
		Passed:  true,
		Details: "window closed or waived",
	})

	// Step 13: Check expiry
	if now.After(req.Envelope.Expiry) {
		reason := fmt.Sprintf("envelope expired at %s", req.Envelope.Expiry.Format(time.RFC3339))
		return e.finalizeBlocked(result, req, attemptID, reason,
			SettlementExpired, now, "envelope_not_expired", reason)
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "envelope_not_expired",
		Passed:  true,
		Details: fmt.Sprintf("expires at %s", req.Envelope.Expiry.Format(time.RFC3339)),
	})

	// Step 14: Update ledger to prepared
	if err := e.attemptLedger.UpdateStatus(attemptID, attempts.AttemptStatusPrepared, now); err != nil {
		return e.finalizeBlocked(result, req, attemptID,
			fmt.Sprintf("ledger update failed: %v", err),
			SettlementBlocked, now, "ledger_prepared", err.Error())
	}

	// Step 15: Forced pause with revocation polling
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ForcedPauseStarted,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"duration_ms": fmt.Sprintf("%d", e.config.ForcedPauseDuration.Milliseconds()),
		},
	})

	pauseEnd := now.Add(e.config.ForcedPauseDuration)
	pollInterval := e.config.RevocationPollInterval
	if pollInterval == 0 {
		pollInterval = 100 * time.Millisecond
	}

	for time.Now().Before(pauseEnd) {
		select {
		case <-ctx.Done():
			return e.finalizeBlocked(result, req, attemptID, "context cancelled during pause",
				SettlementAborted, time.Now(), "pause_completed", "context cancelled")
		case <-time.After(pollInterval):
			// Check for revocation during pause
			if e.revocationChecker.IsRevoked(req.Envelope.EnvelopeID) {
				e.emitEvent(result, events.Event{
					ID:             e.idGenerator(),
					Type:           events.EventV95RevocationDuringPause,
					Timestamp:      time.Now(),
					CircleID:       req.Envelope.ActorCircleID,
					IntersectionID: req.Envelope.IntersectionID,
					SubjectID:      req.Envelope.EnvelopeID,
					SubjectType:    "envelope",
					TraceID:        req.TraceID,
				})

				return e.finalizeBlocked(result, req, attemptID,
					"revoked during forced pause",
					SettlementRevoked, time.Now(), "revocation_during_pause", "revoked during forced pause")
			}

			// Check for abort during pause
			e.mu.RLock()
			aborted := e.abortedEnvelopes[req.Envelope.EnvelopeID]
			e.mu.RUnlock()
			if aborted {
				e.emitEvent(result, events.Event{
					ID:             e.idGenerator(),
					Type:           events.EventV95ExecutionAbortedBeforeProvider,
					Timestamp:      time.Now(),
					CircleID:       req.Envelope.ActorCircleID,
					IntersectionID: req.Envelope.IntersectionID,
					SubjectID:      req.Envelope.EnvelopeID,
					SubjectType:    "envelope",
					TraceID:        req.TraceID,
				})

				return e.finalizeBlocked(result, req, attemptID,
					"aborted during forced pause",
					SettlementAborted, time.Now(), "abort_during_pause", "aborted during forced pause")
			}
		}
	}

	pauseCompleteTime := time.Now()
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ForcedPauseCompleted,
		Timestamp:      pauseCompleteTime,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
	})

	// Step 16: Update ledger to invoked (before provider call)
	if err := e.attemptLedger.UpdateStatus(attemptID, attempts.AttemptStatusInvoked, pauseCompleteTime); err != nil {
		return e.finalizeBlocked(result, req, attemptID,
			fmt.Sprintf("ledger update failed: %v", err),
			SettlementBlocked, pauseCompleteTime, "ledger_invoked", err.Error())
	}

	// Step 17: Emit provider idempotency attached event
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV96ProviderIdempotencyAttached,
		Timestamp:      pauseCompleteTime,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      attemptID,
		SubjectType:    "attempt",
		TraceID:        req.TraceID,
		Provider:       providerName,
		Metadata: map[string]string{
			"envelope_id":        req.Envelope.EnvelopeID,
			"idempotency_prefix": result.IdempotencyKeyPrefix,
			"provider":           providerName,
		},
	})

	// Step 18: Provider prepare call
	prepareResult, err := provider.Prepare(ctx, write.PrepareRequest{
		Envelope: ToWriteEnvelope(req.Envelope),
		Approval: e.buildApprovalForProvider(req.Approvals),
		PayeeID:  req.PayeeID,
		Now:      pauseCompleteTime,
	})
	if err != nil {
		return e.finalizeBlocked(result, req, attemptID,
			fmt.Sprintf("provider prepare failed: %v", err),
			SettlementBlocked, time.Now(), "provider_prepare", err.Error())
	}

	if !prepareResult.Valid {
		return e.finalizeBlocked(result, req, attemptID,
			fmt.Sprintf("provider validation failed: %s", prepareResult.InvalidReason),
			SettlementBlocked, time.Now(), "provider_validation", prepareResult.InvalidReason)
	}

	// Step 19: Provider execute call with idempotency key
	executeTime := time.Now()
	receipt, err := provider.Execute(ctx, write.ExecuteRequest{
		Envelope:       ToWriteEnvelope(req.Envelope),
		Approval:       e.buildApprovalForProvider(req.Approvals),
		PayeeID:        req.PayeeID,
		IdempotencyKey: attempts.HashForProvider(idempotencyKey),
		Now:            executeTime,
	})
	if err != nil {
		errTime := time.Now()
		e.attemptLedger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
			AttemptID:     attemptID,
			Status:        attempts.AttemptStatusFailed,
			BlockedReason: err.Error(),
			MoneyMoved:    false,
			Now:           errTime,
		})

		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("provider execute failed: %v", err)
		result.CompletedAt = errTime
		e.emitBlocked(result, req.Envelope, result.BlockedReason, errTime, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, attemptID, errTime)
		return result, nil
	}

	// Step 20: Finalize success
	completedAt := time.Now()
	result.Success = true
	result.Receipt = receipt
	result.CompletedAt = completedAt

	// Determine terminal status based on provider type
	var terminalStatus attempts.AttemptStatus
	if receipt.Simulated {
		result.Status = SettlementSimulated
		result.MoneyMoved = false
		terminalStatus = attempts.AttemptStatusSimulated

		// GUARDRAIL: Mock providers must NEVER claim real money movement
		if providerName == "mock-write" && receipt.Status == write.PaymentSucceeded {
			panic("GUARDRAIL VIOLATION: mock provider reported PaymentSucceeded (real money)")
		}
	} else {
		result.Status = SettlementSuccessful
		// Money is moved only if provider succeeded with a non-simulated payment
		result.MoneyMoved = (receipt.Status == write.PaymentSucceeded)
		terminalStatus = attempts.AttemptStatusSettled
	}

	// Finalize in ledger
	e.attemptLedger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
		AttemptID:   attemptID,
		Status:      terminalStatus,
		ProviderRef: receipt.ProviderRef,
		MoneyMoved:  result.MoneyMoved,
		Now:         completedAt,
	})

	// Emit success events
	if receipt.Simulated {
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9PaymentSimulated,
			Timestamp:      completedAt,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      receipt.ReceiptID,
			SubjectType:    "receipt",
			TraceID:        req.TraceID,
			Provider:       providerName,
			Metadata: map[string]string{
				"envelope_id":  req.Envelope.EnvelopeID,
				"provider_ref": receipt.ProviderRef,
				"amount":       fmt.Sprintf("%d", receipt.AmountCents),
				"currency":     receipt.Currency,
				"simulated":    "true",
				"money_moved":  "false",
			},
		})
	} else {
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9PaymentSucceeded,
			Timestamp:      completedAt,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      receipt.ReceiptID,
			SubjectType:    "receipt",
			TraceID:        req.TraceID,
			Provider:       providerName,
			Metadata: map[string]string{
				"envelope_id":  req.Envelope.EnvelopeID,
				"provider_ref": receipt.ProviderRef,
				"amount":       fmt.Sprintf("%d", receipt.AmountCents),
				"currency":     receipt.Currency,
				"money_moved":  fmt.Sprintf("%t", result.MoneyMoved),
			},
		})
	}

	e.emitAttemptFinalized(result, req.Envelope, req.TraceID, attemptID, completedAt)

	return result, nil
}

// handleLedgerError handles errors from the attempt ledger.
func (e *V96Executor) handleLedgerError(result *V96ExecuteResult, req V96ExecuteRequest, err error, now time.Time) (*V96ExecuteResult, error) {
	switch err {
	case attempts.ErrAttemptAlreadyExists, attempts.ErrAttemptReplay:
		result.ReplayBlocked = true
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = "replay blocked: attempt already exists or finalized"

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV96AttemptReplayBlocked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      result.AttemptID,
			SubjectType:    "attempt",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id": req.Envelope.EnvelopeID,
				"reason":      err.Error(),
			},
		})

	case attempts.ErrAttemptInFlight:
		result.InflightBlocked = true
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = "blocked: another attempt is in flight for this envelope"

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV96AttemptInflightBlocked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      result.AttemptID,
			SubjectType:    "attempt",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id": req.Envelope.EnvelopeID,
				"reason":      err.Error(),
			},
		})

	case attempts.ErrIdempotencyKeyConflict:
		result.ReplayBlocked = true
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = "blocked: idempotency key already used"

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV96AttemptReplayBlocked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      result.AttemptID,
			SubjectType:    "attempt",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id":        req.Envelope.EnvelopeID,
				"idempotency_prefix": result.IdempotencyKeyPrefix,
				"reason":             "idempotency_key_conflict",
			},
		})

	default:
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("ledger error: %v", err)
	}

	result.CompletedAt = now
	return result, nil
}

// finalizeBlocked finalizes an attempt as blocked with proper ledger update.
func (e *V96Executor) finalizeBlocked(
	result *V96ExecuteResult,
	req V96ExecuteRequest,
	attemptID string,
	reason string,
	status SettlementStatus,
	now time.Time,
	checkName string,
	checkDetails string,
) (*V96ExecuteResult, error) {
	result.Success = false
	result.Status = status
	result.BlockedReason = reason
	result.CompletedAt = now

	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   checkName,
		Passed:  false,
		Details: checkDetails,
	})

	// Map settlement status to attempt status
	var attemptStatus attempts.AttemptStatus
	switch status {
	case SettlementRevoked:
		attemptStatus = attempts.AttemptStatusRevoked
	case SettlementExpired:
		attemptStatus = attempts.AttemptStatusExpired
	case SettlementAborted:
		attemptStatus = attempts.AttemptStatusAborted
	default:
		attemptStatus = attempts.AttemptStatusBlocked
	}

	// Finalize in ledger
	e.attemptLedger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
		AttemptID:     attemptID,
		Status:        attemptStatus,
		BlockedReason: reason,
		MoneyMoved:    false,
		Now:           now,
	})

	e.emitBlocked(result, req.Envelope, reason, now, req.TraceID)
	e.emitAttemptFinalized(result, req.Envelope, req.TraceID, attemptID, now)

	return result, nil
}

// selectProvider selects the appropriate payment provider.
func (e *V96Executor) selectProvider() (write.WriteConnector, string) {
	// TrueLayer is only used if configured AND in sandbox mode
	if e.config.TrueLayerConfigured && e.config.TrueLayerEnvironment == "sandbox" && e.trueLayerConnector != nil {
		return e.trueLayerConnector, "truelayer-sandbox"
	}
	return e.mockConnector, "mock-write"
}

// buildApprovalForProvider constructs an approval artifact for provider calls.
func (e *V96Executor) buildApprovalForProvider(approvals []MultiPartyApprovalArtifact) *write.ApprovalArtifact {
	if len(approvals) == 0 {
		return nil
	}
	// Use the first approval for provider (they all reference the same action hash)
	first := approvals[0]
	return &write.ApprovalArtifact{
		ArtifactID:       first.ArtifactID,
		ApproverCircleID: first.ApproverCircleID,
		ApproverID:       first.ApproverID,
		ActionHash:       first.ActionHash,
		ApprovedAt:       first.ApprovedAt,
		ExpiresAt:        first.ExpiresAt,
		Signature:        first.Signature,
	}
}

// Abort marks an envelope as aborted.
func (e *V96Executor) Abort(envelopeID string, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.abortedEnvelopes[envelopeID] = true
}

// Revoke marks an envelope as revoked via the revocation checker.
func (e *V96Executor) Revoke(envelopeID, revokerCircleID, revokerID, reason string, now time.Time) {
	e.revocationChecker.Revoke(envelopeID, revokerCircleID, revokerID, reason, now)
}

// emitEvent adds an event to the result and emits it.
func (e *V96Executor) emitEvent(result *V96ExecuteResult, event events.Event) {
	result.AuditEvents = append(result.AuditEvents, event)
	if e.auditEmitter != nil {
		e.auditEmitter(event)
	}
}

// emitBlocked emits a blocked event.
func (e *V96Executor) emitBlocked(result *V96ExecuteResult, envelope *ExecutionEnvelope, reason string, now time.Time, traceID string) {
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ExecutionBlocked,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        traceID,
		Metadata: map[string]string{
			"reason": reason,
		},
	})
}

// emitAttemptFinalized emits the attempt finalized event.
func (e *V96Executor) emitAttemptFinalized(result *V96ExecuteResult, envelope *ExecutionEnvelope, traceID, attemptID string, now time.Time) {
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV96AttemptFinalized,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      attemptID,
		SubjectType:    "attempt",
		TraceID:        traceID,
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"status":      string(result.Status),
			"success":     fmt.Sprintf("%t", result.Success),
			"money_moved": fmt.Sprintf("%t", result.MoneyMoved),
		},
	})
}
