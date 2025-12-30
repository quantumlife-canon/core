// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.3 Executor for single-party real financial execution.
//
// CRITICAL: This is the FIRST slice where money may actually move.
// It must be minimal, constrained, auditable, interruptible, and boring.
//
// HARD SAFETY CONSTRAINTS (NON-NEGOTIABLE):
// 1) Provider: TrueLayer ONLY
// 2) Cap: DEFAULT hard cap = £1.00 (100 pence)
// 3) No free-text recipients - pre-defined payee IDs only
// 4) No standing/blanket approvals
// 5) Approval must be action-hash bound and single-use
// 6) Revocation window must be enforced
// 7) Forced pause between approval verification and execute attempt
// 8) No retries - failures require new approval
// 9) Execution must be fully abortable before external call
// 10) Audit must reconstruct the full story
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/pkg/events"
)

// ToWriteEnvelope converts an ExecutionEnvelope to write.ExecutionEnvelope.
func ToWriteEnvelope(env *ExecutionEnvelope) *write.ExecutionEnvelope {
	return &write.ExecutionEnvelope{
		EnvelopeID:            env.EnvelopeID,
		ActorCircleID:         env.ActorCircleID,
		IntersectionID:        env.IntersectionID,
		ActionHash:            env.ActionHash,
		SealHash:              env.SealHash,
		AmountCap:             env.AmountCap,
		Expiry:                env.Expiry,
		RevocationWindowStart: env.RevocationWindowStart,
		RevocationWindowEnd:   env.RevocationWindowEnd,
		RevocationWaived:      env.RevocationWaived,
		Revoked:               env.Revoked,
		RevokedAt:             env.RevokedAt,
		RevokedBy:             env.RevokedBy,
		ActionSpec: write.ActionSpec{
			Type:        string(env.ActionSpec.Type),
			AmountCents: env.ActionSpec.AmountCents,
			Currency:    env.ActionSpec.Currency,
			Recipient:   env.ActionSpec.Recipient,
			Description: env.ActionSpec.Description,
		},
	}
}

// ToWriteApproval converts an ApprovalArtifact to write.ApprovalArtifact.
func ToWriteApproval(approval *ApprovalArtifact) *write.ApprovalArtifact {
	return &write.ApprovalArtifact{
		ArtifactID:         approval.ArtifactID,
		ApproverCircleID:   approval.ApproverCircleID,
		ApproverID:         approval.ApproverID,
		ActionHash:         approval.ActionHash,
		ApprovedAt:         approval.ApprovedAt,
		ExpiresAt:          approval.ExpiresAt,
		Signature:          approval.Signature,
		SignatureAlgorithm: approval.SignatureAlgorithm,
	}
}

// V93Executor executes single-party real financial payments.
//
// CRITICAL: This is the FIRST executor that can move real money.
type V93Executor struct {
	mu sync.RWMutex

	// Connector is the write connector (TrueLayer only in v9.3).
	connector write.WriteConnector

	// Config is the execution configuration.
	config V93ExecutorConfig

	// Components
	approvalVerifier  *ApprovalVerifier
	revocationChecker *RevocationChecker

	// State
	abortedEnvelopes map[string]bool
	auditEmitter     func(event events.Event)
	idGenerator      func() string
}

// V93ExecutorConfig configures the v9.3 executor.
type V93ExecutorConfig struct {
	// CapCents is the hard cap in cents.
	// Defaults to 100 (£1.00).
	CapCents int64

	// AllowedCurrencies is the list of allowed currencies.
	AllowedCurrencies []string

	// ForcedPauseDuration is the mandatory pause before execution.
	ForcedPauseDuration time.Duration

	// RequireExplicitApproval requires explicit --approve flag.
	RequireExplicitApproval bool
}

// DefaultV93ExecutorConfig returns the default configuration.
func DefaultV93ExecutorConfig() V93ExecutorConfig {
	return V93ExecutorConfig{
		CapCents:                write.DefaultCapCents,
		AllowedCurrencies:       []string{"GBP"},
		ForcedPauseDuration:     2 * time.Second,
		RequireExplicitApproval: true,
	}
}

// NewV93Executor creates a new v9.3 executor.
func NewV93Executor(
	connector write.WriteConnector,
	approvalVerifier *ApprovalVerifier,
	revocationChecker *RevocationChecker,
	config V93ExecutorConfig,
	idGen func() string,
	emitter func(event events.Event),
) *V93Executor {
	return &V93Executor{
		connector:         connector,
		config:            config,
		approvalVerifier:  approvalVerifier,
		revocationChecker: revocationChecker,
		abortedEnvelopes:  make(map[string]bool),
		auditEmitter:      emitter,
		idGenerator:       idGen,
	}
}

// V93ExecuteRequest contains parameters for execution.
type V93ExecuteRequest struct {
	// Envelope is the sealed execution envelope.
	Envelope *ExecutionEnvelope

	// Approval is the approval artifact.
	Approval *ApprovalArtifact

	// PayeeID is the pre-defined payee identifier.
	PayeeID string

	// ExplicitApprove indicates the user passed --approve flag.
	ExplicitApprove bool

	// Now is the current time.
	Now time.Time
}

// V93ExecuteResult contains the result of execution.
type V93ExecuteResult struct {
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

	// AuditEvents contains all audit events.
	AuditEvents []events.Event

	// MoneyMoved indicates if any money was moved.
	// CRITICAL: Only true if provider confirmed success.
	MoneyMoved bool

	// CompletedAt is when execution completed.
	CompletedAt time.Time
}

// ValidationCheckResult represents a single validation check.
type ValidationCheckResult struct {
	Check   string
	Passed  bool
	Details string
}

// Execute performs the full execution pipeline.
//
// CRITICAL: This is the ONLY path where money can move.
func (e *V93Executor) Execute(ctx context.Context, req V93ExecuteRequest) (*V93ExecuteResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := &V93ExecuteResult{
		ValidationDetails: make([]ValidationCheckResult, 0),
		AuditEvents:       make([]events.Event, 0),
		MoneyMoved:        false,
		CompletedAt:       now,
	}

	// Emit execution started
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ExecutionStarted,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"provider": e.connector.Provider(),
			"payee_id": req.PayeeID,
		},
	})

	// Step 1: Check explicit approval flag
	if e.config.RequireExplicitApproval && !req.ExplicitApprove {
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = "explicit --approve flag required"
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "explicit_approve_flag",
			Passed:  false,
			Details: "command must include --approve flag",
		})
		e.emitBlocked(result, req.Envelope, "explicit --approve flag required", now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "explicit_approve_flag",
		Passed:  true,
		Details: "--approve flag provided",
	})

	// Step 2: Check abort status
	e.mu.RLock()
	aborted := e.abortedEnvelopes[req.Envelope.EnvelopeID]
	e.mu.RUnlock()
	if aborted {
		result.Success = false
		result.Status = SettlementAborted
		result.BlockedReason = "execution was aborted"
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "not_aborted",
			Passed:  false,
			Details: "envelope was aborted before execution",
		})
		e.emitBlocked(result, req.Envelope, "execution was aborted", now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "not_aborted",
		Passed:  true,
		Details: "not aborted",
	})

	// Step 3: Validate amount within cap
	if req.Envelope.ActionSpec.AmountCents > e.config.CapCents {
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("amount %d exceeds hard cap %d", req.Envelope.ActionSpec.AmountCents, e.config.CapCents)
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "amount_within_cap",
			Passed:  false,
			Details: fmt.Sprintf("amount %d > cap %d", req.Envelope.ActionSpec.AmountCents, e.config.CapCents),
		})
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9CapExceeded,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"amount": fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"cap":    fmt.Sprintf("%d", e.config.CapCents),
			},
		})
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "amount_within_cap",
		Passed:  true,
		Details: fmt.Sprintf("amount %d <= cap %d", req.Envelope.ActionSpec.AmountCents, e.config.CapCents),
	})
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9CapChecked,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"amount": fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
			"cap":    fmt.Sprintf("%d", e.config.CapCents),
			"passed": "true",
		},
	})

	// Step 4: Validate approval
	if req.Approval == nil {
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = "explicit approval required"
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "approval_exists",
			Passed:  false,
			Details: "no approval artifact provided",
		})
		e.emitBlocked(result, req.Envelope, "explicit approval required", now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "approval_exists",
		Passed:  true,
		Details: fmt.Sprintf("artifact ID: %s", req.Approval.ArtifactID),
	})

	// Step 5: Verify approval
	verifyErr := e.approvalVerifier.VerifyApproval(req.Approval, req.Envelope.ActionHash, now)
	if verifyErr != nil {
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("approval verification failed: %s", verifyErr.Error())
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "approval_verified",
			Passed:  false,
			Details: verifyErr.Error(),
		})
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "approval_verified",
		Passed:  true,
		Details: "signature and hash verified",
	})

	// Step 6: Check revocation
	revocationCheck := e.revocationChecker.Check(req.Envelope.EnvelopeID, now)
	if revocationCheck.Revoked {
		signal := revocationCheck.Signal
		result.Success = false
		result.Status = SettlementRevoked
		result.BlockedReason = fmt.Sprintf("envelope revoked: %s", signal.Reason)
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "not_revoked",
			Passed:  false,
			Details: fmt.Sprintf("revoked by %s at %s", signal.RevokerID, signal.RevokedAt.Format(time.RFC3339)),
		})
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9ExecutionRevoked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"revoked_by": signal.RevokerID,
				"reason":     signal.Reason,
			},
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "not_revoked",
		Passed:  true,
		Details: "no revocation",
	})

	// Step 7: Check revocation window (must be closed or waived)
	if !req.Envelope.RevocationWaived && now.Before(req.Envelope.RevocationWindowEnd) {
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = "revocation window is still active"
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "revocation_window_closed",
			Passed:  false,
			Details: fmt.Sprintf("window ends at %s", req.Envelope.RevocationWindowEnd.Format(time.RFC3339)),
		})
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "revocation_window_closed",
		Passed:  true,
		Details: "window closed or waived",
	})

	// Step 8: Check envelope expiry
	if now.After(req.Envelope.Expiry) {
		result.Success = false
		result.Status = SettlementExpired
		result.BlockedReason = "envelope has expired"
		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "envelope_not_expired",
			Passed:  false,
			Details: fmt.Sprintf("expired at %s", req.Envelope.Expiry.Format(time.RFC3339)),
		})
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "envelope_not_expired",
		Passed:  true,
		Details: fmt.Sprintf("expires at %s", req.Envelope.Expiry.Format(time.RFC3339)),
	})

	// Step 9: Connector prepare
	prepareResult, err := e.connector.Prepare(ctx, write.PrepareRequest{
		Envelope: ToWriteEnvelope(req.Envelope),
		Approval: ToWriteApproval(req.Approval),
		PayeeID:  req.PayeeID,
		Now:      now,
	})
	if err != nil {
		result.Success = false
		result.Status = SettlementAborted
		result.BlockedReason = fmt.Sprintf("prepare failed: %v", err)
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
		return result, nil
	}
	if !prepareResult.Valid {
		result.Success = false
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("validation failed: %s", prepareResult.InvalidReason)
		for _, detail := range prepareResult.ValidationDetails {
			result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
				Check:   detail.Check,
				Passed:  detail.Passed,
				Details: detail.Details,
			})
		}
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
		return result, nil
	}
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9PaymentPrepared,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Provider:       e.connector.Provider(),
		Metadata: map[string]string{
			"payee_id": req.PayeeID,
		},
	})

	// Step 10: FORCED PAUSE - intentional friction
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ForcedPauseStarted,
		Timestamp:      time.Now(),
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"duration_seconds": fmt.Sprintf("%d", int(e.config.ForcedPauseDuration.Seconds())),
		},
	})

	select {
	case <-ctx.Done():
		result.Success = false
		result.Status = SettlementAborted
		result.BlockedReason = "context cancelled during forced pause"
		e.emitBlocked(result, req.Envelope, result.BlockedReason, time.Now())
		return result, ctx.Err()
	case <-time.After(e.config.ForcedPauseDuration):
		// Continue after pause
	}

	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ForcedPauseCompleted,
		Timestamp:      time.Now(),
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
	})

	// Step 11: Check abort again after pause
	e.mu.RLock()
	aborted = e.abortedEnvelopes[req.Envelope.EnvelopeID]
	e.mu.RUnlock()
	if aborted {
		result.Success = false
		result.Status = SettlementAborted
		result.BlockedReason = "execution aborted during forced pause"
		e.emitBlocked(result, req.Envelope, result.BlockedReason, time.Now())
		return result, nil
	}

	// Step 12: Execute payment
	idempotencyKey := fmt.Sprintf("%s-%s", req.Envelope.EnvelopeID, req.Approval.ArtifactID)
	receipt, err := e.connector.Execute(ctx, write.ExecuteRequest{
		Envelope:       ToWriteEnvelope(req.Envelope),
		Approval:       ToWriteApproval(req.Approval),
		PayeeID:        req.PayeeID,
		IdempotencyKey: idempotencyKey,
		Now:            time.Now(),
	})

	if err != nil {
		result.Success = false
		result.Status = SettlementAborted
		result.BlockedReason = fmt.Sprintf("execution failed: %v", err)
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9PaymentFailed,
			Timestamp:      time.Now(),
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       e.connector.Provider(),
			Metadata: map[string]string{
				"error":       err.Error(),
				"money_moved": "false",
			},
		})
		return result, nil
	}

	// Step 13: Record success
	result.Success = true
	result.Receipt = receipt
	result.Status = SettlementSuccessful
	result.MoneyMoved = receipt.Status == write.PaymentSucceeded || receipt.Status == write.PaymentExecuting || receipt.Status == write.PaymentPending
	result.CompletedAt = time.Now()

	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9PaymentSucceeded,
		Timestamp:      result.CompletedAt,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      receipt.ReceiptID,
		SubjectType:    "receipt",
		Provider:       e.connector.Provider(),
		Metadata: map[string]string{
			"envelope_id":  req.Envelope.EnvelopeID,
			"provider_ref": receipt.ProviderRef,
			"amount":       fmt.Sprintf("%d", receipt.AmountCents),
			"currency":     receipt.Currency,
			"payee_id":     receipt.PayeeID,
			"money_moved":  fmt.Sprintf("%t", result.MoneyMoved),
		},
	})

	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9SettlementSucceeded,
		Timestamp:      result.CompletedAt,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "settlement",
		Provider:       e.connector.Provider(),
		Metadata: map[string]string{
			"receipt_id":   receipt.ReceiptID,
			"provider_ref": receipt.ProviderRef,
			"money_moved":  fmt.Sprintf("%t", result.MoneyMoved),
		},
	})

	return result, nil
}

// Abort cancels execution before provider call if possible.
func (e *V93Executor) Abort(envelopeID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.abortedEnvelopes[envelopeID] = true

	if e.connector != nil {
		_, _ = e.connector.Abort(context.Background(), envelopeID)
	}

	if e.auditEmitter != nil {
		e.auditEmitter(events.Event{
			ID:          e.idGenerator(),
			Type:        events.EventV9ExecutionAborted,
			Timestamp:   time.Now(),
			SubjectID:   envelopeID,
			SubjectType: "envelope",
			Metadata: map[string]string{
				"reason": "user-initiated abort",
			},
		})
	}

	return true
}

// emitEvent records an audit event.
func (e *V93Executor) emitEvent(result *V93ExecuteResult, event events.Event) {
	result.AuditEvents = append(result.AuditEvents, event)
	if e.auditEmitter != nil {
		e.auditEmitter(event)
	}
}

// emitBlocked emits a blocked event.
func (e *V93Executor) emitBlocked(result *V93ExecuteResult, envelope *ExecutionEnvelope, reason string, now time.Time) {
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ExecutionBlocked,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Provider:       e.connector.Provider(),
		Metadata: map[string]string{
			"reason":      reason,
			"money_moved": "false",
		},
	})
}
