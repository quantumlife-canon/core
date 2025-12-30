package demo_v9_dryrun

import (
	"fmt"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// Runner executes v9 dry-run scenarios.
//
// CRITICAL: This runner is DRY-RUN ONLY.
// It demonstrates the complete v9 execution pipeline
// without moving any real money.
type Runner struct {
	envelopeBuilder   *execution.EnvelopeBuilder
	approvalManager   *execution.ApprovalManager
	approvalVerifier  *execution.ApprovalVerifier
	revocationChecker *execution.RevocationChecker
	windowManager     *execution.RevocationWindowManager
	executionRunner   *execution.ExecutionRunner

	idCounter  int
	signingKey []byte

	// For deterministic testing
	clock func() time.Time

	// Audit log
	auditLog []events.Event
}

// NewRunner creates a new v9 dry-run runner.
func NewRunner() *Runner {
	signingKey := []byte("demo-signing-key-v9-dryrun")

	idGen := func() string { return "" } // placeholder
	r := &Runner{
		signingKey: signingKey,
		idCounter:  0,
		clock:      time.Now,
		auditLog:   []events.Event{},
	}

	// Wire up ID generator
	idGen = r.generateID

	r.envelopeBuilder = execution.NewEnvelopeBuilder(idGen)
	r.approvalManager = execution.NewApprovalManager(idGen, signingKey)
	r.approvalVerifier = execution.NewApprovalVerifier(signingKey)
	r.revocationChecker = execution.NewRevocationChecker(idGen)
	r.windowManager = execution.NewRevocationWindowManager(idGen)
	r.executionRunner = execution.NewExecutionRunner(r.approvalVerifier, r.revocationChecker, idGen)

	return r
}

// NewRunnerWithClock creates a runner with an injected clock for testing.
func NewRunnerWithClock(clock func() time.Time) *Runner {
	r := NewRunner()
	r.clock = clock
	return r
}

// generateID generates a unique ID.
func (r *Runner) generateID() string {
	r.idCounter++
	return fmt.Sprintf("v9_demo_%d", r.idCounter)
}

// emitEvent emits an audit event.
func (r *Runner) emitEvent(eventType events.EventType, circleID, intersectionID, subjectID, subjectType string, metadata map[string]string) {
	event := events.Event{
		ID:             r.generateID(),
		Type:           eventType,
		Timestamp:      r.clock(),
		CircleID:       circleID,
		IntersectionID: intersectionID,
		SubjectID:      subjectID,
		SubjectType:    subjectType,
		Metadata:       metadata,
	}
	r.auditLog = append(r.auditLog, event)
}

// Run executes the default scenario and returns the result.
func (r *Runner) Run() (*ScenarioResult, error) {
	return r.RunScenario(DefaultScenario())
}

// RunScenario executes a specific scenario.
func (r *Runner) RunScenario(scenario *Scenario) (*ScenarioResult, error) {
	now := r.clock()
	result := &ScenarioResult{
		Scenario:    scenario,
		AuditEvents: []events.Event{},
	}

	// Step 1: Create Intent
	intent := scenario.Intent
	intent.IntentID = r.generateID()
	intent.CreatedAt = now
	intent.ViewHash = "v8_view_hash_" + r.generateID() // Simulated v8 view reference
	result.Intent = intent

	r.emitEvent(events.EventExecutionIntentCreated,
		intent.CircleID, intent.IntersectionID, intent.IntentID, "intent",
		map[string]string{
			"action_type": string(intent.ActionType),
			"amount":      FormatAmount(intent.AmountCents, intent.Currency),
			"recipient":   intent.Recipient,
			"view_hash":   intent.ViewHash[:20] + "...",
		})

	// Step 2: Build Envelope
	envelope, err := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                intent.AmountCents * 2, // Cap at 2x amount
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1, // Single-party approval in v9 Slice 1
		RevocationWindowDuration: 5 * time.Minute,
		RevocationWaived:         false,
		TraceID:                  r.generateID(),
	}, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("envelope build failed: %v", err)
		return result, err
	}
	result.Envelope = envelope

	r.emitEvent(events.EventExecutionEnvelopeSealed,
		envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
		map[string]string{
			"action_hash": envelope.ActionHash[:20] + "...",
			"seal_hash":   envelope.SealHash[:20] + "...",
			"amount_cap":  FormatAmount(envelope.AmountCap, envelope.ActionSpec.Currency),
			"expiry":      envelope.Expiry.Format(time.RFC3339),
		})

	// Step 3: Request Approval
	approvalRequest, err := r.approvalManager.CreateApprovalRequest(
		envelope,
		intent.CircleID,
		now.Add(30*time.Minute),
		now,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("approval request failed: %v", err)
		return result, err
	}
	result.ApprovalRequest = approvalRequest

	r.emitEvent(events.EventV9ApprovalRequested,
		intent.CircleID, envelope.IntersectionID, approvalRequest.RequestID, "approval_request",
		map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"action_hash": approvalRequest.ActionHash[:20] + "...",
			"prompt_text": approvalRequest.PromptText,
			"expires_at":  approvalRequest.ExpiresAt.Format(time.RFC3339),
		})

	// Step 4: Submit Approval
	approval, err := r.approvalManager.SubmitApproval(
		approvalRequest,
		intent.CircleID,
		"user_alice",
		now.Add(25*time.Minute),
		now,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("approval submission failed: %v", err)
		return result, err
	}
	result.Approval = approval
	envelope.Approvals = append(envelope.Approvals, *approval)

	r.emitEvent(events.EventV9ApprovalSubmitted,
		approval.ApproverCircleID, envelope.IntersectionID, approval.ArtifactID, "approval_artifact",
		map[string]string{
			"envelope_id":    envelope.EnvelopeID,
			"action_hash":    approval.ActionHash[:20] + "...",
			"approver_id":    approval.ApproverID,
			"approved_at":    approval.ApprovedAt.Format(time.RFC3339),
			"signature_algo": approval.SignatureAlgorithm,
		})

	// Verify approval
	if err := r.approvalVerifier.VerifyApproval(approval, envelope.ActionHash, now); err != nil {
		r.emitEvent(events.EventV9ApprovalRejected,
			approval.ApproverCircleID, envelope.IntersectionID, approval.ArtifactID, "approval_artifact",
			map[string]string{"error": err.Error()})
		result.Success = false
		result.FailureReason = fmt.Sprintf("approval verification failed: %v", err)
		return result, err
	}

	r.emitEvent(events.EventV9ApprovalVerified,
		approval.ApproverCircleID, envelope.IntersectionID, approval.ArtifactID, "approval_artifact",
		map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"action_hash": approval.ActionHash[:20] + "...",
		})

	// Step 5: Revocation Window Opens
	r.emitEvent(events.EventV9RevocationWindowOpened,
		envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
		map[string]string{
			"window_start": envelope.RevocationWindowStart.Format(time.RFC3339),
			"window_end":   envelope.RevocationWindowEnd.Format(time.RFC3339),
		})

	// Step 6: Revocation (if applicable)
	if scenario.ShouldRevoke {
		// Simulate time passing within revocation window
		revocationTime := now.Add(2 * time.Minute) // 2 minutes into 5-minute window

		signal := r.revocationChecker.Revoke(
			envelope.EnvelopeID,
			intent.CircleID,
			"user_alice",
			scenario.RevocationReason,
			revocationTime,
		)
		result.RevocationSignal = signal

		r.emitEvent(events.EventV9RevocationTriggered,
			signal.RevokerCircleID, envelope.IntersectionID, signal.SignalID, "revocation_signal",
			map[string]string{
				"envelope_id": envelope.EnvelopeID,
				"revoker_id":  signal.RevokerID,
				"reason":      signal.Reason,
				"revoked_at":  signal.RevokedAt.Format(time.RFC3339),
			})
	}

	// Step 7: Attempt Execution
	// Simulate time after revocation window closes (or during if revoked)
	executionTime := now
	if scenario.ShouldRevoke {
		executionTime = now.Add(3 * time.Minute) // During window but after revocation
	} else {
		executionTime = now.Add(6 * time.Minute) // After window closes
	}

	r.emitEvent(events.EventV9ExecutionStarted,
		envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
		map[string]string{
			"attempted_at": executionTime.Format(time.RFC3339),
		})

	execResult, err := r.executionRunner.Execute(envelope, executionTime)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("execution failed: %v", err)
		return result, err
	}
	result.ExecutionResult = execResult
	result.ValidityCheck = execResult.ValidityCheck

	// Emit execution outcome event
	switch execResult.Status {
	case execution.SettlementRevoked:
		r.emitEvent(events.EventV9ExecutionRevoked,
			envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
			map[string]string{
				"revoked_by": execResult.RevokedBy,
				"reason":     execResult.BlockedReason,
			})
	case execution.SettlementBlocked:
		r.emitEvent(events.EventV9ExecutionBlocked,
			envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
			map[string]string{
				"reason": execResult.BlockedReason,
			})
	case execution.SettlementAborted:
		r.emitEvent(events.EventV9ExecutionAborted,
			envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
			map[string]string{
				"reason": execResult.BlockedReason,
			})
	case execution.SettlementExpired:
		r.emitEvent(events.EventV9ExecutionBlocked,
			envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "envelope",
			map[string]string{
				"reason": "envelope expired",
			})
	}

	// Step 8: Record Settlement
	settlementEventType := events.EventV9SettlementRecorded
	switch execResult.Status {
	case execution.SettlementRevoked:
		settlementEventType = events.EventV9SettlementRevoked
	case execution.SettlementBlocked:
		settlementEventType = events.EventV9SettlementBlocked
	case execution.SettlementAborted:
		settlementEventType = events.EventV9SettlementAborted
	case execution.SettlementExpired:
		settlementEventType = events.EventV9SettlementExpired
	}

	r.emitEvent(settlementEventType,
		envelope.ActorCircleID, envelope.IntersectionID, envelope.EnvelopeID, "settlement",
		map[string]string{
			"status":       string(execResult.Status),
			"completed_at": execResult.CompletedAt.Format(time.RFC3339),
		})

	// Step 9: Finalize Audit Trail
	r.emitEvent(events.EventV9AuditTraceFinalized,
		envelope.ActorCircleID, envelope.IntersectionID, envelope.TraceID, "audit_trace",
		map[string]string{
			"event_count":  fmt.Sprintf("%d", len(r.auditLog)),
			"final_status": string(execResult.Status),
			"money_moved":  "false", // ALWAYS false in dry-run
		})

	// Copy audit log to result
	result.AuditEvents = make([]events.Event, len(r.auditLog))
	copy(result.AuditEvents, r.auditLog)

	// Check if result matches expected status
	if execResult.Status == scenario.ExpectedStatus {
		result.Success = true
	} else {
		result.Success = false
		result.FailureReason = fmt.Sprintf("expected status %s, got %s",
			scenario.ExpectedStatus, execResult.Status)
	}

	// CRITICAL: Verify no successful settlement
	if execResult.Status == execution.SettlementSuccessful {
		result.Success = false
		result.FailureReason = "CRITICAL VIOLATION: Settlement was successful - this is forbidden in dry-run mode"
	}

	return result, nil
}

// GetAuditLog returns the complete audit log.
func (r *Runner) GetAuditLog() []events.Event {
	return r.auditLog
}

// ClearAuditLog clears the audit log.
func (r *Runner) ClearAuditLog() {
	r.auditLog = []events.Event{}
}

// PrintResult prints the scenario result in a human-readable format.
func PrintResult(result *ScenarioResult) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("  v9 Financial Execution Demo (DRY-RUN)")
	fmt.Println("============================================================")
	fmt.Println()
	fmt.Println("CRITICAL: This is DRY-RUN mode. NO REAL MONEY MOVES.")
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Scenario: %s\n", result.Scenario.Name)
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Description: %s\n", result.Scenario.Description)
	fmt.Println()

	// Intent
	fmt.Println("1. INTENT CREATED")
	fmt.Printf("   Intent ID: %s\n", result.Intent.IntentID)
	fmt.Printf("   Circle: %s\n", result.Intent.CircleID)
	fmt.Printf("   Action: %s\n", result.Intent.ActionType)
	fmt.Printf("   Amount: %s\n", FormatAmount(result.Intent.AmountCents, result.Intent.Currency))
	fmt.Printf("   Recipient: %s\n", result.Intent.Recipient)
	fmt.Printf("   View Hash: %s...\n", result.Intent.ViewHash[:20])
	fmt.Println()

	// Envelope
	if result.Envelope != nil {
		fmt.Println("2. ENVELOPE SEALED")
		fmt.Printf("   Envelope ID: %s\n", result.Envelope.EnvelopeID)
		fmt.Printf("   Action Hash: %s...\n", result.Envelope.ActionHash[:20])
		fmt.Printf("   Seal Hash: %s...\n", result.Envelope.SealHash[:20])
		fmt.Printf("   Amount Cap: %s\n", FormatAmount(result.Envelope.AmountCap, result.Envelope.ActionSpec.Currency))
		fmt.Printf("   Expiry: %s\n", result.Envelope.Expiry.Format(time.RFC3339))
		fmt.Printf("   Revocation Window: %s to %s\n",
			result.Envelope.RevocationWindowStart.Format(time.RFC3339),
			result.Envelope.RevocationWindowEnd.Format(time.RFC3339))
		fmt.Println()
	}

	// Approval
	if result.ApprovalRequest != nil {
		fmt.Println("3. APPROVAL REQUESTED")
		fmt.Printf("   Request ID: %s\n", result.ApprovalRequest.RequestID)
		fmt.Printf("   Prompt: %s\n", result.ApprovalRequest.PromptText)
		fmt.Printf("   (Neutral language verified)\n")
		fmt.Println()
	}

	if result.Approval != nil {
		fmt.Println("4. APPROVAL SUBMITTED")
		fmt.Printf("   Artifact ID: %s\n", result.Approval.ArtifactID)
		fmt.Printf("   Approver: %s\n", result.Approval.ApproverID)
		fmt.Printf("   Action Hash Bound: %s...\n", result.Approval.ActionHash[:20])
		fmt.Printf("   Signature Algorithm: %s\n", result.Approval.SignatureAlgorithm)
		fmt.Println()
	}

	// Revocation
	fmt.Println("5. REVOCATION WINDOW OPENED")
	if result.RevocationSignal != nil {
		fmt.Println("6. REVOCATION TRIGGERED")
		fmt.Printf("   Signal ID: %s\n", result.RevocationSignal.SignalID)
		fmt.Printf("   Revoker: %s\n", result.RevocationSignal.RevokerID)
		fmt.Printf("   Reason: %s\n", result.RevocationSignal.Reason)
		fmt.Printf("   Revoked At: %s\n", result.RevocationSignal.RevokedAt.Format(time.RFC3339))
		fmt.Println()
	} else {
		fmt.Println("6. NO REVOCATION")
		fmt.Println()
	}

	// Execution Result
	if result.ExecutionResult != nil {
		fmt.Println("7. EXECUTION ATTEMPTED")
		fmt.Printf("   Status: %s\n", result.ExecutionResult.Status)
		fmt.Printf("   Blocked Reason: %s\n", result.ExecutionResult.BlockedReason)
		if result.ExecutionResult.RevokedBy != "" {
			fmt.Printf("   Revoked By: %s\n", result.ExecutionResult.RevokedBy)
		}
		fmt.Println()

		// Validity Check
		fmt.Println("   Validity Check:")
		fmt.Printf("     Valid: %t\n", result.ExecutionResult.ValidityCheck.Valid)
		for _, cond := range result.ExecutionResult.ValidityCheck.Conditions {
			status := "✓"
			if !cond.Satisfied {
				status = "✗"
			}
			fmt.Printf("     %s %s: %s\n", status, cond.Condition, cond.Details)
		}
		fmt.Println()
	}

	// Settlement
	fmt.Println("8. SETTLEMENT RECORDED")
	fmt.Printf("   Final Status: %s\n", result.ExecutionResult.Status)
	fmt.Printf("   Money Moved: NO (dry-run mode)\n")
	fmt.Println()

	// Audit Summary
	fmt.Println("------------------------------------------------------------")
	fmt.Println("  AUDIT TRAIL SUMMARY")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Total Events: %d\n", len(result.AuditEvents))
	fmt.Println()
	for i, event := range result.AuditEvents {
		fmt.Printf("  [%d] %s\n", i+1, event.Type)
		fmt.Printf("      Subject: %s (%s)\n", event.SubjectID, event.SubjectType)
		if len(event.Metadata) > 0 {
			for k, v := range event.Metadata {
				if len(v) > 50 {
					v = v[:50] + "..."
				}
				fmt.Printf("      %s: %s\n", k, v)
			}
		}
	}
	fmt.Println()

	// Final Status
	fmt.Println("============================================================")
	if result.Success {
		fmt.Println("  ✓ DEMO COMPLETED SUCCESSFULLY")
		fmt.Println("  ✓ Execution was prepared, inspected, revoked, and safely stopped")
		fmt.Println("  ✓ ZERO money moved")
		fmt.Println("  ✓ Full audit trace recorded")
	} else {
		fmt.Println("  ✗ DEMO FAILED")
		fmt.Printf("  Reason: %s\n", result.FailureReason)
	}
	fmt.Println("============================================================")
	fmt.Println()
}
