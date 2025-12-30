// Package demo_v9_guarded demonstrates v9 guarded execution.
//
// CRITICAL: This demo uses GUARDED adapters.
// NO REAL MONEY MOVES. Adapter ALWAYS blocks execution.
//
// Subordinate to:
// - docs/CANON_ADDENDUM_V9_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v9_guarded

import (
	"fmt"
	"sync/atomic"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// Runner orchestrates v9 guarded execution demos.
type Runner struct {
	envelopeBuilder   *execution.EnvelopeBuilder
	approvalManager   *execution.ApprovalManager
	approvalVerifier  *execution.ApprovalVerifier
	revocationChecker *execution.RevocationChecker
	executionRunner   *execution.ExecutionRunner
	adapterRegistry   *execution.AdapterRegistry

	auditEvents []events.Event
	idCounter   uint64
	signingKey  []byte
}

// NewRunner creates a new demo runner.
func NewRunner() *Runner {
	signingKey := []byte("demo-signing-key-v9-guarded")

	r := &Runner{
		auditEvents: make([]events.Event, 0),
		signingKey:  signingKey,
	}

	idGen := r.generateID
	emitter := r.emitEvent

	r.envelopeBuilder = execution.NewEnvelopeBuilder(idGen)
	r.approvalManager = execution.NewApprovalManager(idGen, signingKey)
	r.approvalVerifier = execution.NewApprovalVerifier(signingKey)
	r.revocationChecker = execution.NewRevocationChecker(idGen)
	r.executionRunner = execution.NewExecutionRunner(
		r.approvalVerifier,
		r.revocationChecker,
		idGen,
	)

	// Set up adapter registry with guarded adapters
	r.adapterRegistry = execution.NewAdapterRegistry()
	r.adapterRegistry.Register(execution.NewMockFinanceAdapter(idGen, emitter))
	r.adapterRegistry.Register(execution.NewPlaidStubAdapter(idGen, emitter))
	r.adapterRegistry.Register(execution.NewTrueLayerStubAdapter(idGen, emitter))

	return r
}

// generateID generates sequential IDs for demo purposes.
func (r *Runner) generateID() string {
	id := atomic.AddUint64(&r.idCounter, 1)
	return fmt.Sprintf("v9g_demo_%d", id)
}

// emitEvent records an audit event.
func (r *Runner) emitEvent(event events.Event) {
	r.auditEvents = append(r.auditEvents, event)
}

// Run executes the default scenario.
func (r *Runner) Run() (*ScenarioResult, error) {
	return r.RunScenario(DefaultScenario())
}

// RunScenario executes a specific scenario.
func (r *Runner) RunScenario(scenario *Scenario) (*ScenarioResult, error) {
	result := &ScenarioResult{
		Scenario:    scenario,
		AuditEvents: make([]events.Event, 0),
	}

	// Reset state
	r.auditEvents = make([]events.Event, 0)
	r.idCounter = 0

	now := time.Now()

	// Step 1: Create intent
	intent := scenario.Intent
	intent.IntentID = r.generateID()
	intent.CreatedAt = now
	intent.ViewHash = "v8_view_hash_" + intent.IntentID
	result.Intent = intent

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventExecutionIntentCreated,
		Timestamp:      now,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      intent.IntentID,
		SubjectType:    "intent",
		Metadata: map[string]string{
			"action_type": string(intent.ActionType),
			"amount":      FormatAmount(intent.AmountCents, intent.Currency),
			"recipient":   intent.Recipient,
			"view_hash":   safePrefix(intent.ViewHash, 20) + "...",
		},
	})

	// Step 2: Build sealed envelope
	revocationWindowDuration := 5 * time.Minute
	traceID := r.generateID()
	envelope, err := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                intent.AmountCents * 2, // 200% cap
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: revocationWindowDuration,
		TraceID:                  traceID,
	}, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("envelope build failed: %v", err)
		return result, err
	}
	result.Envelope = envelope

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventExecutionEnvelopeSealed,
		Timestamp:      now,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"action_hash": envelope.ActionHash,
			"seal_hash":   envelope.SealHash,
			"amount_cap":  FormatAmount(envelope.AmountCap, intent.Currency),
			"expiry":      envelope.Expiry.Format(time.RFC3339),
		},
	})

	// Step 3: Create approval request
	approvalExpiry := now.Add(30 * time.Minute)
	if scenario.ShouldExpireApproval {
		// Set approval to expire in the past for testing
		approvalExpiry = now.Add(-1 * time.Minute)
	}

	approvalReq, err := r.approvalManager.CreateApprovalRequest(
		envelope,
		intent.CircleID,
		approvalExpiry,
		now,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("approval request failed: %v", err)
		return result, err
	}
	result.ApprovalRequest = approvalReq

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9ApprovalRequested,
		Timestamp:      now,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      approvalReq.RequestID,
		SubjectType:    "approval_request",
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"action_hash": approvalReq.ActionHash,
			"prompt_text": truncate(approvalReq.PromptText, 50),
			"expires_at":  approvalReq.ExpiresAt.Format(time.RFC3339),
		},
	})

	// Step 4: Submit approval
	approval, err := r.approvalManager.SubmitApproval(
		approvalReq,
		intent.CircleID,
		"circle_member_alice",
		approvalExpiry,
		now,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("approval submission failed: %v", err)
		return result, err
	}
	result.Approval = approval

	// Add approval to envelope
	envelope.Approvals = append(envelope.Approvals, *approval)

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9ApprovalSubmitted,
		Timestamp:      now,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      approval.ArtifactID,
		SubjectType:    "approval_artifact",
		Metadata: map[string]string{
			"envelope_id":    envelope.EnvelopeID,
			"action_hash":    approval.ActionHash,
			"approver_id":    approval.ApproverID,
			"approved_at":    approval.ApprovedAt.Format(time.RFC3339),
			"signature_algo": approval.SignatureAlgorithm,
		},
	})

	// Step 5: Emit revocation window open
	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9RevocationWindowOpened,
		Timestamp:      now,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"window_start": envelope.RevocationWindowStart.Format(time.RFC3339),
			"window_end":   envelope.RevocationWindowEnd.Format(time.RFC3339),
		},
	})

	// Step 6: Handle revocation if scenario requires it
	if scenario.ShouldRevoke {
		revokeTime := now.Add(2 * time.Minute) // During window

		signal := r.revocationChecker.Revoke(
			envelope.EnvelopeID,
			intent.CircleID,
			"circle_member_alice",
			scenario.RevocationReason,
			revokeTime,
		)

		r.emitEvent(events.Event{
			ID:             r.generateID(),
			Type:           events.EventV9RevocationTriggered,
			Timestamp:      revokeTime,
			CircleID:       intent.CircleID,
			IntersectionID: intent.IntersectionID,
			SubjectID:      signal.SignalID,
			SubjectType:    "revocation_signal",
			Metadata: map[string]string{
				"envelope_id": envelope.EnvelopeID,
				"revoker_id":  signal.RevokerID,
				"reason":      truncate(signal.Reason, 50),
				"revoked_at":  signal.RevokedAt.Format(time.RFC3339),
			},
		})
	}

	// Step 7: Simulate revocation window closing (advance time)
	executionTime := now.Add(revocationWindowDuration + 1*time.Minute)

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9RevocationWindowClosed,
		Timestamp:      envelope.RevocationWindowEnd,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"window_end": envelope.RevocationWindowEnd.Format(time.RFC3339),
		},
	})

	// Step 8: Get adapter
	adapter, ok := r.adapterRegistry.Get(scenario.AdapterProvider)
	if !ok {
		result.Success = false
		result.FailureReason = fmt.Sprintf("adapter not found: %s", scenario.AdapterProvider)
		return result, fmt.Errorf("adapter not found: %s", scenario.AdapterProvider)
	}

	// Step 9: Emit execution started
	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9ExecutionStarted,
		Timestamp:      executionTime,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"attempted_at": executionTime.Format(time.RFC3339),
			"adapter":      adapter.Provider(),
		},
	})

	// Step 10: Execute with adapter
	// CRITICAL: In v9 Slice 2, adapter ALWAYS blocks execution
	execResult, attempt, execErr := r.executionRunner.ExecuteWithAdapter(
		envelope,
		adapter,
		executionTime,
	)
	if execErr != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("execution error: %v", execErr)
		return result, execErr
	}
	result.ExecutionResult = execResult
	result.ExecutionAttempt = attempt

	// Step 11: Emit execution result event
	var execEventType events.EventType
	switch execResult.Status {
	case execution.SettlementBlocked:
		execEventType = events.EventV9ExecutionBlocked
	case execution.SettlementRevoked:
		execEventType = events.EventV9ExecutionRevoked
	case execution.SettlementExpired:
		execEventType = events.EventV9ExecutionBlocked // Expired maps to blocked
	default:
		execEventType = events.EventV9ExecutionAborted
	}

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           execEventType,
		Timestamp:      execResult.CompletedAt,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"status":         string(execResult.Status),
			"blocked_reason": truncate(execResult.BlockedReason, 50),
		},
	})

	// Step 12: Emit settlement event
	var settlementEventType events.EventType
	switch execResult.Status {
	case execution.SettlementBlocked:
		settlementEventType = events.EventV9SettlementBlocked
	case execution.SettlementRevoked:
		settlementEventType = events.EventV9SettlementRevoked
	case execution.SettlementExpired:
		settlementEventType = events.EventV9SettlementExpired
	default:
		settlementEventType = events.EventV9SettlementAborted
	}

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           settlementEventType,
		Timestamp:      execResult.CompletedAt,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "settlement",
		Metadata: map[string]string{
			"status":       string(execResult.Status),
			"completed_at": execResult.CompletedAt.Format(time.RFC3339),
		},
	})

	// Step 13: Finalize audit trace
	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9AuditTraceFinalized,
		Timestamp:      execResult.CompletedAt,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      envelope.TraceID,
		SubjectType:    "audit_trace",
		Metadata: map[string]string{
			"final_status": string(execResult.Status),
			"money_moved":  "false", // CRITICAL: Always false in v9 Slice 2
			"event_count":  fmt.Sprintf("%d", len(r.auditEvents)),
			"adapter":      adapter.Provider(),
		},
	})

	// Copy audit events to result
	result.AuditEvents = make([]events.Event, len(r.auditEvents))
	copy(result.AuditEvents, r.auditEvents)

	// Verify expected outcome
	if execResult.Status != scenario.ExpectedStatus {
		result.Success = false
		result.FailureReason = fmt.Sprintf("unexpected status: got %s, expected %s",
			execResult.Status, scenario.ExpectedStatus)
		return result, nil
	}

	// CRITICAL: Verify money never moved
	if attempt != nil && attempt.MoneyMoved {
		result.Success = false
		result.FailureReason = "CRITICAL: money moved - this should NEVER happen"
		return result, fmt.Errorf("money moved in guarded execution")
	}

	// CRITICAL: Verify settlement is not successful
	if execResult.Status == execution.SettlementSuccessful {
		result.Success = false
		result.FailureReason = "CRITICAL: settlement successful - this is forbidden in v9 Slice 2"
		return result, fmt.Errorf("settlement successful in guarded execution")
	}

	result.Success = true
	return result, nil
}

// truncate truncates a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// safePrefix returns a safe prefix of a string, handling short strings.
func safePrefix(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// PrintResult prints the scenario result in a human-readable format.
func PrintResult(result *ScenarioResult) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("  v9 Guarded Execution Demo")
	fmt.Println("============================================================")
	fmt.Println()
	fmt.Println("CRITICAL: This uses GUARDED adapters. NO REAL MONEY MOVES.")
	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Scenario: %s\n", result.Scenario.Name)
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Description: %s\n", result.Scenario.Description)
	fmt.Printf("  Adapter: %s\n", result.Scenario.AdapterProvider)
	fmt.Println()

	// Print intent
	fmt.Println("1. INTENT CREATED")
	fmt.Printf("   Intent ID: %s\n", result.Intent.IntentID)
	fmt.Printf("   Circle: %s\n", result.Intent.CircleID)
	fmt.Printf("   Action: %s\n", result.Intent.ActionType)
	fmt.Printf("   Amount: %s\n", FormatAmount(result.Intent.AmountCents, result.Intent.Currency))
	fmt.Printf("   Recipient: %s\n", result.Intent.Recipient)
	fmt.Printf("   View Hash: %s...\n", safePrefix(result.Intent.ViewHash, 20))
	fmt.Println()

	// Print envelope
	if result.Envelope != nil {
		fmt.Println("2. ENVELOPE SEALED")
		fmt.Printf("   Envelope ID: %s\n", result.Envelope.EnvelopeID)
		fmt.Printf("   Action Hash: %s...\n", safePrefix(result.Envelope.ActionHash, 20))
		fmt.Printf("   Seal Hash: %s...\n", safePrefix(result.Envelope.SealHash, 20))
		fmt.Printf("   Amount Cap: %s\n", FormatAmount(result.Envelope.AmountCap, result.Intent.Currency))
		fmt.Printf("   Expiry: %s\n", result.Envelope.Expiry.Format(time.RFC3339))
		fmt.Printf("   Revocation Window: %s to %s\n",
			result.Envelope.RevocationWindowStart.Format(time.RFC3339),
			result.Envelope.RevocationWindowEnd.Format(time.RFC3339))
		fmt.Println()
	}

	// Print approval
	if result.ApprovalRequest != nil {
		fmt.Println("3. APPROVAL REQUESTED")
		fmt.Printf("   Request ID: %s\n", result.ApprovalRequest.RequestID)
		fmt.Printf("   Prompt: %s\n", result.ApprovalRequest.PromptText)
		fmt.Println("   (Neutral language verified)")
		fmt.Println()
	}

	if result.Approval != nil {
		fmt.Println("4. APPROVAL SUBMITTED")
		fmt.Printf("   Artifact ID: %s\n", result.Approval.ArtifactID)
		fmt.Printf("   Approver: %s\n", result.Approval.ApproverID)
		fmt.Printf("   Action Hash Bound: %s...\n", safePrefix(result.Approval.ActionHash, 20))
		fmt.Printf("   Signature Algorithm: %s\n", result.Approval.SignatureAlgorithm)
		fmt.Println()
	}

	// Print revocation window
	fmt.Println("5. REVOCATION WINDOW")
	if result.Scenario.ShouldRevoke {
		fmt.Println("   Status: REVOCATION TRIGGERED")
		fmt.Printf("   Reason: %s\n", result.Scenario.RevocationReason)
	} else {
		fmt.Println("   Status: Window expired without revocation")
	}
	fmt.Println()

	// Print execution attempt
	if result.ExecutionAttempt != nil {
		fmt.Println("6. ADAPTER EXECUTION ATTEMPTED")
		fmt.Printf("   Attempt ID: %s\n", result.ExecutionAttempt.AttemptID)
		fmt.Printf("   Provider: %s\n", result.ExecutionAttempt.Provider)
		fmt.Printf("   Status: %s\n", result.ExecutionAttempt.Status)
		fmt.Printf("   Blocked Reason: %s\n", result.ExecutionAttempt.BlockedReason)
		fmt.Printf("   Money Moved: %t\n", result.ExecutionAttempt.MoneyMoved)
		fmt.Println()
	}

	// Print execution result
	if result.ExecutionResult != nil {
		fmt.Println("7. EXECUTION RESULT")
		fmt.Printf("   Status: %s\n", result.ExecutionResult.Status)
		fmt.Printf("   Blocked Reason: %s\n", result.ExecutionResult.BlockedReason)
		fmt.Printf("   Money Moved: NO (guarded adapter)\n")
		fmt.Println()
	}

	// Print audit trail summary
	fmt.Println("------------------------------------------------------------")
	fmt.Println("  AUDIT TRAIL SUMMARY")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Total Events: %d\n", len(result.AuditEvents))
	fmt.Println()

	for i, event := range result.AuditEvents {
		fmt.Printf("  [%d] %s\n", i+1, event.Type)
		fmt.Printf("      Subject: %s (%s)\n", event.SubjectID, event.SubjectType)
		if event.Provider != "" {
			fmt.Printf("      Provider: %s\n", event.Provider)
		}
		for k, v := range event.Metadata {
			fmt.Printf("      %s: %s\n", k, v)
		}
	}
	fmt.Println()

	// Print final status
	fmt.Println("============================================================")
	if result.Success {
		fmt.Println("  DEMO COMPLETED SUCCESSFULLY")
		fmt.Println("  Guarded adapter blocked execution as expected")
		fmt.Println("  ZERO money moved")
		fmt.Println("  Full audit trace recorded")
	} else {
		fmt.Println("  DEMO FAILED")
		fmt.Printf("  Reason: %s\n", result.FailureReason)
	}
	fmt.Println("============================================================")
}
