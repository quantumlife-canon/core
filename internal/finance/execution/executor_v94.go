// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.4 Executor for multi-party real financial execution.
//
// CRITICAL: This executor extends v9.3 with multi-party approval support.
// It enforces all v9.3 constraints PLUS multi-party symmetry verification.
//
// NON-NEGOTIABLE INVARIANTS:
// 1) All v9.3 constraints remain in force
// 2) No blanket/standing approvals - each approval binds to specific ActionHash
// 3) Neutral approval language - reject urgency/fear/shame/authority/optimization
// 4) Symmetry - every approver receives IDENTICAL approval payload (provable)
// 5) Approvals do NOT bypass revocation windows
// 6) Single-use approvals only
// 7) Mock providers MUST report MoneyMoved=false
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

// V94Executor executes multi-party real financial payments.
//
// CRITICAL: This executor adds multi-party gate before v9.3 execution pipeline.
type V94Executor struct {
	mu sync.RWMutex

	// Connector is the write connector (TrueLayer only in v9.4).
	connector write.WriteConnector

	// Config is the execution configuration.
	config V94ExecutorConfig

	// Components
	multiPartyGate    *MultiPartyGate
	approvalVerifier  *ApprovalVerifier
	revocationChecker *RevocationChecker

	// State
	abortedEnvelopes map[string]bool
	auditEmitter     func(event events.Event)
	idGenerator      func() string
}

// V94ExecutorConfig configures the v9.4 executor.
type V94ExecutorConfig struct {
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

// DefaultV94ExecutorConfig returns the default configuration.
func DefaultV94ExecutorConfig() V94ExecutorConfig {
	return V94ExecutorConfig{
		CapCents:                write.DefaultCapCents,
		AllowedCurrencies:       []string{"GBP"},
		ForcedPauseDuration:     2 * time.Second,
		RequireExplicitApproval: true,
	}
}

// NewV94Executor creates a new v9.4 executor.
func NewV94Executor(
	connector write.WriteConnector,
	multiPartyGate *MultiPartyGate,
	approvalVerifier *ApprovalVerifier,
	revocationChecker *RevocationChecker,
	config V94ExecutorConfig,
	idGen func() string,
	emitter func(event events.Event),
) *V94Executor {
	return &V94Executor{
		connector:         connector,
		config:            config,
		multiPartyGate:    multiPartyGate,
		approvalVerifier:  approvalVerifier,
		revocationChecker: revocationChecker,
		abortedEnvelopes:  make(map[string]bool),
		auditEmitter:      emitter,
		idGenerator:       idGen,
	}
}

// V94ExecuteRequest contains parameters for multi-party execution.
type V94ExecuteRequest struct {
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

	// Now is the current time.
	Now time.Time
}

// V94ExecuteResult contains the result of multi-party execution.
type V94ExecuteResult struct {
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

	// AuditEvents contains all audit events.
	AuditEvents []events.Event

	// MoneyMoved indicates if any money was moved.
	// CRITICAL: Only true if provider confirmed success AND not simulated.
	MoneyMoved bool

	// CompletedAt is when execution completed.
	CompletedAt time.Time
}

// Execute performs the full multi-party execution pipeline.
//
// CRITICAL: This pipeline adds multi-party gate BEFORE v9.3 execution flow.
func (e *V94Executor) Execute(ctx context.Context, req V94ExecuteRequest) (*V94ExecuteResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := &V94ExecuteResult{
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
			"provider":    e.connector.Provider(),
			"payee_id":    req.PayeeID,
			"multi_party": fmt.Sprintf("%t", req.Policy.IsMultiParty()),
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

	// Step 3: MULTI-PARTY GATE - enforces threshold + symmetry
	if req.Policy.IsMultiParty() && req.Policy.AppliesToFinanceWrite() {
		gateResult, err := e.multiPartyGate.Verify(ctx, MultiPartyGateRequest{
			Envelope:       req.Envelope,
			Bundle:         req.Bundle,
			Approvals:      req.Approvals,
			ApproverHashes: req.ApproverHashes,
			Policy:         req.Policy,
			Now:            now,
		})
		if err != nil {
			result.Success = false
			result.Status = SettlementBlocked
			result.BlockedReason = fmt.Sprintf("multi-party gate error: %v", err)
			e.emitBlocked(result, req.Envelope, result.BlockedReason, now)
			return result, nil
		}

		result.GateResult = gateResult
		result.AuditEvents = append(result.AuditEvents, gateResult.AuditEvents...)

		if !gateResult.Passed {
			result.Success = false
			result.Status = SettlementBlocked
			result.BlockedReason = gateResult.BlockedReason
			result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
				Check:   "multi_party_gate",
				Passed:  false,
				Details: gateResult.BlockedReason,
			})
			e.emitEvent(result, events.Event{
				ID:             e.idGenerator(),
				Type:           events.EventV94MultiPartyGateBlocked,
				Timestamp:      now,
				CircleID:       req.Envelope.ActorCircleID,
				IntersectionID: req.Envelope.IntersectionID,
				SubjectID:      req.Envelope.EnvelopeID,
				SubjectType:    "envelope",
				Metadata: map[string]string{
					"reason": gateResult.BlockedReason,
				},
			})
			return result, nil
		}

		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "multi_party_gate",
			Passed:  true,
			Details: fmt.Sprintf("threshold met: %d/%d", gateResult.ThresholdResult.Obtained, gateResult.ThresholdResult.Required),
		})
	} else {
		// Single-party mode - verify single approval
		if len(req.Approvals) == 0 {
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

		// Verify single approval using existing v9.3 verifier
		singleApproval := &ApprovalArtifact{
			ArtifactID:         req.Approvals[0].ArtifactID,
			ApproverCircleID:   req.Approvals[0].ApproverCircleID,
			ApproverID:         req.Approvals[0].ApproverID,
			ActionHash:         req.Approvals[0].ActionHash,
			ApprovedAt:         req.Approvals[0].ApprovedAt,
			ExpiresAt:          req.Approvals[0].ExpiresAt,
			Signature:          req.Approvals[0].Signature,
			SignatureAlgorithm: req.Approvals[0].SignatureAlgorithm,
		}

		verifyErr := e.approvalVerifier.VerifyApproval(singleApproval, req.Envelope.ActionHash, now)
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
	}

	// Step 4: Validate amount within cap
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

	// Step 5: Check revocation
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

	// Step 6: Check revocation window (must be closed or waived)
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

	// Step 7: Check envelope expiry
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

	// Build write.ApprovalArtifact for connector
	var writeApproval *write.ApprovalArtifact
	if len(req.Approvals) > 0 {
		writeApproval = &write.ApprovalArtifact{
			ArtifactID:         req.Approvals[0].ArtifactID,
			ApproverCircleID:   req.Approvals[0].ApproverCircleID,
			ApproverID:         req.Approvals[0].ApproverID,
			ActionHash:         req.Approvals[0].ActionHash,
			ApprovedAt:         req.Approvals[0].ApprovedAt,
			ExpiresAt:          req.Approvals[0].ExpiresAt,
			Signature:          req.Approvals[0].Signature,
			SignatureAlgorithm: req.Approvals[0].SignatureAlgorithm,
		}
	}

	// Step 8: Connector prepare
	prepareResult, err := e.connector.Prepare(ctx, write.PrepareRequest{
		Envelope: ToWriteEnvelope(req.Envelope),
		Approval: writeApproval,
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

	// Step 9: FORCED PAUSE - intentional friction
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

	// Step 10: Check abort again after pause
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

	// Step 11: Check revocation again after pause (approvals don't bypass revocation)
	revocationCheck = e.revocationChecker.Check(req.Envelope.EnvelopeID, time.Now())
	if revocationCheck.Revoked {
		signal := revocationCheck.Signal
		result.Success = false
		result.Status = SettlementRevoked
		result.BlockedReason = fmt.Sprintf("envelope revoked during pause: %s", signal.Reason)
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9ExecutionRevoked,
			Timestamp:      time.Now(),
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Metadata: map[string]string{
				"revoked_by": signal.RevokerID,
				"reason":     signal.Reason,
				"phase":      "post_pause",
			},
		})
		return result, nil
	}

	// Step 12: Execute payment
	idempotencyKey := fmt.Sprintf("%s-%s", req.Envelope.EnvelopeID, req.Approvals[0].ArtifactID)
	receipt, err := e.connector.Execute(ctx, write.ExecuteRequest{
		Envelope:       ToWriteEnvelope(req.Envelope),
		Approval:       writeApproval,
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

	// Step 13: Record result
	result.Success = true
	result.Receipt = receipt
	result.CompletedAt = time.Now()

	// CRITICAL: Determine if money actually moved
	if receipt.Simulated {
		result.MoneyMoved = false
		result.Status = SettlementSimulated
	} else {
		result.MoneyMoved = receipt.Status == write.PaymentSucceeded ||
			receipt.Status == write.PaymentExecuting ||
			receipt.Status == write.PaymentPending
		result.Status = SettlementSuccessful
	}

	// GUARDRAIL ASSERTION: If provider is mock, MoneyMoved MUST be false.
	if e.connector.Provider() == "mock-write" && result.MoneyMoved {
		panic("GUARDRAIL VIOLATION: mock-write provider reported MoneyMoved=true. This must never happen.")
	}

	// Emit appropriate events based on simulated vs real
	if receipt.Simulated {
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9PaymentSimulated,
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
				"simulated":    "true",
				"money_moved":  "false",
				"multi_party":  fmt.Sprintf("%t", req.Policy.IsMultiParty()),
			},
		})

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV9SettlementSimulated,
			Timestamp:      result.CompletedAt,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "settlement",
			Provider:       e.connector.Provider(),
			Metadata: map[string]string{
				"receipt_id":   receipt.ReceiptID,
				"provider_ref": receipt.ProviderRef,
				"simulated":    "true",
				"money_moved":  "false",
			},
		})
	} else {
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
				"multi_party":  fmt.Sprintf("%t", req.Policy.IsMultiParty()),
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
	}

	return result, nil
}

// Abort cancels execution before provider call if possible.
func (e *V94Executor) Abort(envelopeID string) bool {
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
				"reason": "multi-party abort requested",
			},
		})
	}

	return true
}

// emitEvent records an audit event.
func (e *V94Executor) emitEvent(result *V94ExecuteResult, event events.Event) {
	result.AuditEvents = append(result.AuditEvents, event)
	if e.auditEmitter != nil {
		e.auditEmitter(event)
	}
}

// emitBlocked emits a blocked event.
func (e *V94Executor) emitBlocked(result *V94ExecuteResult, envelope *ExecutionEnvelope, reason string, now time.Time) {
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
