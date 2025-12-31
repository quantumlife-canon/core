// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.5 Executor for real multi-party sandbox execution.
//
// CRITICAL: This executor extends v9.4 with:
// 1) Presentation gate - bundle MUST be presented before approval accepted
// 2) Provider selection - TrueLayer (sandbox only) or mock
// 3) Revocation during forced pause - MUST abort BEFORE provider call
// 4) Attempt tracking - exactly one trace finalization per attempt
// 5) Sandbox enforcement - TrueLayer only allowed in sandbox mode
//
// NON-NEGOTIABLE INVARIANTS:
// All v9.3 and v9.4 constraints remain in force:
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

// V95Executor executes real multi-party financial payments with sandbox enforcement.
//
// CRITICAL: This executor adds presentation gate and provider selection.
type V95Executor struct {
	mu sync.RWMutex

	// Connectors
	trueLayerConnector write.WriteConnector
	mockConnector      write.WriteConnector

	// Config
	config V95ExecutorConfig

	// Components
	presentationGate  *PresentationGate
	multiPartyGate    *MultiPartyGate
	approvalVerifier  *ApprovalVerifier
	revocationChecker *RevocationChecker

	// State
	abortedEnvelopes map[string]bool
	attemptCounter   map[string]int // envelope_id -> attempt count
	auditEmitter     func(event events.Event)
	idGenerator      func() string
}

// V95ExecutorConfig configures the v9.5 executor.
type V95ExecutorConfig struct {
	// CapCents is the hard cap in cents.
	// Defaults to 100 (£1.00).
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
	// CRITICAL: Only "sandbox" is allowed in v9.5.
	TrueLayerEnvironment string

	// PresentationExpiryDuration is how long a presentation remains valid.
	PresentationExpiryDuration time.Duration
}

// DefaultV95ExecutorConfig returns the default configuration.
func DefaultV95ExecutorConfig() V95ExecutorConfig {
	return V95ExecutorConfig{
		CapCents:                   write.DefaultCapCents,
		AllowedCurrencies:          []string{"GBP"},
		ForcedPauseDuration:        2 * time.Second,
		RequireExplicitApproval:    true,
		TrueLayerConfigured:        false,
		TrueLayerEnvironment:       "sandbox",
		PresentationExpiryDuration: 5 * time.Minute,
	}
}

// NewV95Executor creates a new v9.5 executor.
func NewV95Executor(
	trueLayerConnector write.WriteConnector,
	mockConnector write.WriteConnector,
	presentationGate *PresentationGate,
	multiPartyGate *MultiPartyGate,
	approvalVerifier *ApprovalVerifier,
	revocationChecker *RevocationChecker,
	config V95ExecutorConfig,
	idGen func() string,
	emitter func(event events.Event),
) *V95Executor {
	return &V95Executor{
		trueLayerConnector: trueLayerConnector,
		mockConnector:      mockConnector,
		config:             config,
		presentationGate:   presentationGate,
		multiPartyGate:     multiPartyGate,
		approvalVerifier:   approvalVerifier,
		revocationChecker:  revocationChecker,
		abortedEnvelopes:   make(map[string]bool),
		attemptCounter:     make(map[string]int),
		auditEmitter:       emitter,
		idGenerator:        idGen,
	}
}

// V95ExecuteRequest contains parameters for v9.5 execution.
type V95ExecuteRequest struct {
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

	// Now is the current time.
	Now time.Time
}

// V95ExecuteResult contains the result of v9.5 execution.
type V95ExecuteResult struct {
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

	// AuditEvents contains all audit events.
	AuditEvents []events.Event

	// MoneyMoved indicates if any money was moved.
	// CRITICAL: Only true if provider confirmed success AND not simulated.
	MoneyMoved bool

	// CompletedAt is when execution completed.
	CompletedAt time.Time
}

// Execute performs the full v9.5 execution pipeline.
//
// CRITICAL: This pipeline adds:
// 1) Presentation gate BEFORE multi-party gate
// 2) Provider selection (TrueLayer sandbox or mock)
// 3) Revocation check during forced pause with explicit abort
// 4) Attempt tracking for exactly one trace finalization
func (e *V95Executor) Execute(ctx context.Context, req V95ExecuteRequest) (*V95ExecuteResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Generate attempt ID for deduplication
	e.mu.Lock()
	e.attemptCounter[req.Envelope.EnvelopeID]++
	attemptNum := e.attemptCounter[req.Envelope.EnvelopeID]
	e.mu.Unlock()
	attemptID := fmt.Sprintf("%s-attempt-%d", req.Envelope.EnvelopeID, attemptNum)

	result := &V95ExecuteResult{
		ValidationDetails: make([]ValidationCheckResult, 0),
		AuditEvents:       make([]events.Event, 0),
		MoneyMoved:        false,
		CompletedAt:       now,
		AttemptID:         attemptID,
	}

	// Emit attempt started
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV95AttemptStarted,
		Timestamp:      now,
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      attemptID,
		SubjectType:    "attempt",
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"envelope_id":    req.Envelope.EnvelopeID,
			"attempt_number": fmt.Sprintf("%d", attemptNum),
		},
	})

	// Select provider
	provider, providerName := e.selectProvider()
	result.ProviderUsed = providerName

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

	// Emit execution started
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
			"provider":    providerName,
			"payee_id":    req.PayeeID,
			"multi_party": fmt.Sprintf("%t", req.Policy.IsMultiParty()),
			"attempt_id":  attemptID,
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
		e.emitBlocked(result, req.Envelope, "explicit --approve flag required", now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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
		e.emitBlocked(result, req.Envelope, "execution was aborted", now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "not_aborted",
		Passed:  true,
		Details: "not aborted",
	})

	// Step 3: PRESENTATION GATE - verify bundle was presented to all approvers
	if req.Policy.IsMultiParty() && req.Policy.AppliesToFinanceWrite() {
		presentationResult, err := e.presentationGate.VerifyAllPresentations(
			req.Approvals,
			req.Bundle,
			req.Envelope,
			now,
		)
		if err != nil {
			result.Success = false
			result.Status = SettlementBlocked
			result.BlockedReason = fmt.Sprintf("presentation verification error: %v", err)
			e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
			e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
			return result, nil
		}

		result.PresentationResult = presentationResult

		if !presentationResult.AllVerified {
			result.Success = false
			result.Status = SettlementBlocked
			result.BlockedReason = fmt.Sprintf("presentation missing for approvers: %v", presentationResult.MissingApprovers)
			result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
				Check:   "presentation_verified",
				Passed:  false,
				Details: presentationResult.BlockedReason,
			})
			e.emitEvent(result, events.Event{
				ID:             e.idGenerator(),
				Type:           events.EventV95ApprovalPresentationMissing,
				Timestamp:      now,
				CircleID:       req.Envelope.ActorCircleID,
				IntersectionID: req.Envelope.IntersectionID,
				SubjectID:      req.Envelope.EnvelopeID,
				SubjectType:    "envelope",
				TraceID:        req.TraceID,
				Metadata: map[string]string{
					"missing_approvers": fmt.Sprintf("%v", presentationResult.MissingApprovers),
					"reason":            presentationResult.BlockedReason,
				},
			})
			e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
			e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
			return result, nil
		}

		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "presentation_verified",
			Passed:  true,
			Details: fmt.Sprintf("all %d approvers received bundle", len(presentationResult.VerifiedApprovers)),
		})
	}

	// Step 4: MULTI-PARTY GATE - enforces threshold + symmetry
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
			e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
			e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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
				TraceID:        req.TraceID,
				Metadata: map[string]string{
					"reason": gateResult.BlockedReason,
				},
			})
			e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
			return result, nil
		}

		result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
			Check:   "multi_party_gate",
			Passed:  true,
			Details: fmt.Sprintf("threshold met: %d/%d", gateResult.ThresholdResult.Obtained, gateResult.ThresholdResult.Required),
		})
	}

	// Step 5: Validate amount within cap
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
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"amount": fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"cap":    fmt.Sprintf("%d", e.config.CapCents),
			},
		})
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
		Check:   "amount_within_cap",
		Passed:  true,
		Details: fmt.Sprintf("amount %d <= cap %d", req.Envelope.ActionSpec.AmountCents, e.config.CapCents),
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
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"revoked_by": signal.RevokerID,
				"reason":     signal.Reason,
			},
		})
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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

	// Step 9: Connector prepare
	prepareResult, err := provider.Prepare(ctx, write.PrepareRequest{
		Envelope: ToWriteEnvelope(req.Envelope),
		Approval: writeApproval,
		PayeeID:  req.PayeeID,
		Now:      now,
	})
	if err != nil {
		result.Success = false
		result.Status = SettlementAborted
		result.BlockedReason = fmt.Sprintf("prepare failed: %v", err)
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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
		e.emitBlocked(result, req.Envelope, result.BlockedReason, now, req.TraceID)
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, now)
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
		Provider:       providerName,
		TraceID:        req.TraceID,
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
		TraceID:        req.TraceID,
		Metadata: map[string]string{
			"duration_seconds": fmt.Sprintf("%d", int(e.config.ForcedPauseDuration.Seconds())),
		},
	})

	// Forced pause with revocation check
	pauseStart := time.Now()
	pauseEnd := pauseStart.Add(e.config.ForcedPauseDuration)
	checkInterval := 100 * time.Millisecond

	for time.Now().Before(pauseEnd) {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Status = SettlementAborted
			result.BlockedReason = "context cancelled during forced pause"
			e.emitBlocked(result, req.Envelope, result.BlockedReason, time.Now(), req.TraceID)
			e.emitAttemptFinalized(result, req.Envelope, req.TraceID, time.Now())
			return result, ctx.Err()
		case <-time.After(checkInterval):
			// Check for revocation during pause
			revocationCheck = e.revocationChecker.Check(req.Envelope.EnvelopeID, time.Now())
			if revocationCheck.Revoked {
				signal := revocationCheck.Signal
				result.Success = false
				result.Status = SettlementRevoked
				result.BlockedReason = fmt.Sprintf("envelope revoked DURING forced pause: %s", signal.Reason)

				e.emitEvent(result, events.Event{
					ID:             e.idGenerator(),
					Type:           events.EventV95RevocationDuringPause,
					Timestamp:      time.Now(),
					CircleID:       req.Envelope.ActorCircleID,
					IntersectionID: req.Envelope.IntersectionID,
					SubjectID:      req.Envelope.EnvelopeID,
					SubjectType:    "envelope",
					TraceID:        req.TraceID,
					Metadata: map[string]string{
						"revoked_by": signal.RevokerID,
						"reason":     signal.Reason,
						"phase":      "during_forced_pause",
					},
				})

				e.emitEvent(result, events.Event{
					ID:             e.idGenerator(),
					Type:           events.EventV95ExecutionAbortedRevocation,
					Timestamp:      time.Now(),
					CircleID:       req.Envelope.ActorCircleID,
					IntersectionID: req.Envelope.IntersectionID,
					SubjectID:      req.Envelope.EnvelopeID,
					SubjectType:    "envelope",
					TraceID:        req.TraceID,
					Metadata: map[string]string{
						"revoked_by":  signal.RevokerID,
						"reason":      signal.Reason,
						"aborted_at":  "before_provider_call",
						"money_moved": "false",
					},
				})

				e.emitAttemptFinalized(result, req.Envelope, req.TraceID, time.Now())
				return result, nil
			}

			// Check for abort during pause
			e.mu.RLock()
			aborted = e.abortedEnvelopes[req.Envelope.EnvelopeID]
			e.mu.RUnlock()
			if aborted {
				result.Success = false
				result.Status = SettlementAborted
				result.BlockedReason = "execution aborted during forced pause"

				e.emitEvent(result, events.Event{
					ID:             e.idGenerator(),
					Type:           events.EventV95ExecutionAbortedBeforeProvider,
					Timestamp:      time.Now(),
					CircleID:       req.Envelope.ActorCircleID,
					IntersectionID: req.Envelope.IntersectionID,
					SubjectID:      req.Envelope.EnvelopeID,
					SubjectType:    "envelope",
					TraceID:        req.TraceID,
					Metadata: map[string]string{
						"phase":       "during_forced_pause",
						"money_moved": "false",
					},
				})

				e.emitAttemptFinalized(result, req.Envelope, req.TraceID, time.Now())
				return result, nil
			}
		}
	}

	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ForcedPauseCompleted,
		Timestamp:      time.Now(),
		CircleID:       req.Envelope.ActorCircleID,
		IntersectionID: req.Envelope.IntersectionID,
		SubjectID:      req.Envelope.EnvelopeID,
		SubjectType:    "envelope",
		TraceID:        req.TraceID,
	})

	// Step 11: Final revocation check after pause
	revocationCheck = e.revocationChecker.Check(req.Envelope.EnvelopeID, time.Now())
	if revocationCheck.Revoked {
		signal := revocationCheck.Signal
		result.Success = false
		result.Status = SettlementRevoked
		result.BlockedReason = fmt.Sprintf("envelope revoked after pause: %s", signal.Reason)

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV95ExecutionAbortedRevocation,
			Timestamp:      time.Now(),
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"revoked_by":  signal.RevokerID,
				"reason":      signal.Reason,
				"aborted_at":  "after_forced_pause",
				"money_moved": "false",
			},
		})

		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, time.Now())
		return result, nil
	}

	// Step 12: Execute payment
	idempotencyKey := fmt.Sprintf("%s-%s-%s", req.Envelope.EnvelopeID, req.Approvals[0].ArtifactID, attemptID)

	// Emit provider-specific event
	if providerName == "truelayer" {
		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           events.EventV95PaymentTrueLayerCreated,
			Timestamp:      time.Now(),
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       providerName,
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"environment":     e.config.TrueLayerEnvironment,
				"idempotency_key": idempotencyKey,
			},
		})
	}

	receipt, err := provider.Execute(ctx, write.ExecuteRequest{
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

		eventType := events.EventV9PaymentFailed
		if providerName == "truelayer" {
			eventType = events.EventV95PaymentTrueLayerFailed
		}

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           eventType,
			Timestamp:      time.Now(),
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       providerName,
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"error":       err.Error(),
				"money_moved": "false",
			},
		})
		e.emitAttemptFinalized(result, req.Envelope, req.TraceID, time.Now())
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
	if providerName == "mock-write" && result.MoneyMoved {
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
			Provider:       providerName,
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id":  req.Envelope.EnvelopeID,
				"provider_ref": receipt.ProviderRef,
				"amount":       fmt.Sprintf("%d", receipt.AmountCents),
				"currency":     receipt.Currency,
				"payee_id":     receipt.PayeeID,
				"simulated":    "true",
				"money_moved":  "false",
				"multi_party":  fmt.Sprintf("%t", req.Policy.IsMultiParty()),
				"attempt_id":   attemptID,
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
			Provider:       providerName,
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"receipt_id":   receipt.ReceiptID,
				"provider_ref": receipt.ProviderRef,
				"simulated":    "true",
				"money_moved":  "false",
			},
		})
	} else {
		eventType := events.EventV9PaymentSucceeded
		if providerName == "truelayer" {
			eventType = events.EventV95PaymentTrueLayerSucceeded
		}

		e.emitEvent(result, events.Event{
			ID:             e.idGenerator(),
			Type:           eventType,
			Timestamp:      result.CompletedAt,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      receipt.ReceiptID,
			SubjectType:    "receipt",
			Provider:       providerName,
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id":  req.Envelope.EnvelopeID,
				"provider_ref": receipt.ProviderRef,
				"amount":       fmt.Sprintf("%d", receipt.AmountCents),
				"currency":     receipt.Currency,
				"payee_id":     receipt.PayeeID,
				"money_moved":  fmt.Sprintf("%t", result.MoneyMoved),
				"multi_party":  fmt.Sprintf("%t", req.Policy.IsMultiParty()),
				"attempt_id":   attemptID,
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
			Provider:       providerName,
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"receipt_id":   receipt.ReceiptID,
				"provider_ref": receipt.ProviderRef,
				"money_moved":  fmt.Sprintf("%t", result.MoneyMoved),
			},
		})
	}

	e.emitAttemptFinalized(result, req.Envelope, req.TraceID, result.CompletedAt)
	return result, nil
}

// selectProvider selects the appropriate connector.
// Returns TrueLayer if configured AND sandbox mode, otherwise mock.
func (e *V95Executor) selectProvider() (write.WriteConnector, string) {
	// CRITICAL: Only allow TrueLayer in sandbox mode
	if e.config.TrueLayerConfigured && e.config.TrueLayerEnvironment == "sandbox" && e.trueLayerConnector != nil {
		return e.trueLayerConnector, "truelayer"
	}

	// Fall back to mock
	return e.mockConnector, "mock-write"
}

// Abort cancels execution before provider call if possible.
func (e *V95Executor) Abort(envelopeID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.abortedEnvelopes[envelopeID] = true

	// Also abort on connectors
	if e.trueLayerConnector != nil {
		_, _ = e.trueLayerConnector.Abort(context.Background(), envelopeID)
	}
	if e.mockConnector != nil {
		_, _ = e.mockConnector.Abort(context.Background(), envelopeID)
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

// Revoke triggers a revocation for the envelope.
func (e *V95Executor) Revoke(envelopeID, revokerCircleID, revokerID, reason string) {
	e.revocationChecker.Revoke(envelopeID, revokerCircleID, revokerID, reason, time.Now())
}

// emitEvent records an audit event.
func (e *V95Executor) emitEvent(result *V95ExecuteResult, event events.Event) {
	result.AuditEvents = append(result.AuditEvents, event)
	if e.auditEmitter != nil {
		e.auditEmitter(event)
	}
}

// emitBlocked emits a blocked event.
func (e *V95Executor) emitBlocked(result *V95ExecuteResult, envelope *ExecutionEnvelope, reason string, now time.Time, traceID string) {
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV9ExecutionBlocked,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Provider:       result.ProviderUsed,
		TraceID:        traceID,
		Metadata: map[string]string{
			"reason":      reason,
			"money_moved": "false",
		},
	})
}

// emitAttemptFinalized emits attempt finalization event.
func (e *V95Executor) emitAttemptFinalized(result *V95ExecuteResult, envelope *ExecutionEnvelope, traceID string, now time.Time) {
	e.emitEvent(result, events.Event{
		ID:             e.idGenerator(),
		Type:           events.EventV95AttemptFinalized,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      result.AttemptID,
		SubjectType:    "attempt",
		TraceID:        traceID,
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"success":     fmt.Sprintf("%t", result.Success),
			"money_moved": fmt.Sprintf("%t", result.MoneyMoved),
			"provider":    result.ProviderUsed,
		},
	})
}

// IsSandboxEnforced returns true if sandbox mode is enforced.
func (e *V95Executor) IsSandboxEnforced() bool {
	return e.config.TrueLayerEnvironment == "sandbox"
}

// GetPresentationStore returns the presentation store for recording presentations.
func (e *V95Executor) GetPresentationStore() *PresentationStore {
	if e.presentationGate != nil {
		return e.presentationGate.store
	}
	return nil
}
