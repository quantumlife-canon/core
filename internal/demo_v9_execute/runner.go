// Package demo_v9_execute demonstrates v9.3 single-party real financial execution.
//
// CRITICAL: This is the FIRST demo where money may actually move.
// It must be minimal, constrained, auditable, interruptible, and boring.
//
// HARD SAFETY CONSTRAINTS:
// 1) TrueLayer ONLY
// 2) Cap: £1.00 (100 pence) default
// 3) Pre-defined sandbox payees only
// 4) Explicit per-action approval
// 5) Forced pause before execution
// 6) No retries
// 7) Full audit trail
//
// If TrueLayer credentials are not configured:
// - Demo skips real execution
// - Demonstrates full pipeline with mock
// - Explains neutrally what would happen
//
// Subordinate to:
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v9_execute

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"quantumlife/internal/connectors/finance/write"
	truelayer "quantumlife/internal/connectors/finance/write/providers/truelayer"
	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// Runner orchestrates v9.3 single-party real execution demos.
type Runner struct {
	envelopeBuilder   *execution.EnvelopeBuilder
	approvalManager   *execution.ApprovalManager
	approvalVerifier  *execution.ApprovalVerifier
	revocationChecker *execution.RevocationChecker
	executor          *execution.V93Executor
	payeeRegistry     *write.PayeeRegistry

	connector    write.WriteConnector
	isConfigured bool

	auditEvents []events.Event
	idCounter   uint64
	signingKey  []byte
}

// NewRunner creates a new demo runner.
func NewRunner() *Runner {
	signingKey := []byte("demo-signing-key-v9-execute")

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

	// Set up payee registry with sandbox payees
	r.payeeRegistry = write.NewPayeeRegistry()
	for _, payee := range write.SandboxPayees() {
		r.payeeRegistry.Register(payee)
	}

	// Try to create real TrueLayer connector
	clientID := os.Getenv("TRUELAYER_CLIENT_ID")
	clientSecret := os.Getenv("TRUELAYER_CLIENT_SECRET")

	if clientID != "" && clientSecret != "" {
		connector, err := truelayer.NewConnector(truelayer.ConnectorConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Environment:  "sandbox", // CRITICAL: Always sandbox for demo
			Config: write.WriteConfig{
				CapCents:            write.DefaultCapCents,
				AllowedCurrencies:   []string{"GBP"},
				ForcedPauseDuration: 2 * time.Second,
				SandboxMode:         true,
			},
			AuditEmitter: emitter,
			IDGenerator:  idGen,
		})
		if err == nil {
			r.connector = connector
			r.isConfigured = true
		}
	}

	// If not configured, use mock connector
	if r.connector == nil {
		r.connector = NewMockWriteConnector(idGen, emitter)
		r.isConfigured = false
	}

	// Create executor
	r.executor = execution.NewV93Executor(
		r.connector,
		r.approvalVerifier,
		r.revocationChecker,
		execution.DefaultV93ExecutorConfig(),
		idGen,
		emitter,
	)

	return r
}

// generateID generates sequential IDs for demo purposes.
func (r *Runner) generateID() string {
	id := atomic.AddUint64(&r.idCounter, 1)
	return fmt.Sprintf("v9e_demo_%d", id)
}

// emitEvent records an audit event.
func (r *Runner) emitEvent(event events.Event) {
	r.auditEvents = append(r.auditEvents, event)
}

// DemoResult contains the result of running the demo.
type DemoResult struct {
	// IsConfigured indicates if TrueLayer is configured.
	IsConfigured bool

	// Intent is the execution intent.
	Intent execution.ExecutionIntent

	// Envelope is the sealed envelope.
	Envelope *execution.ExecutionEnvelope

	// ApprovalRequest is the approval request.
	ApprovalRequest *execution.ApprovalRequest

	// Approval is the approval artifact.
	Approval *execution.ApprovalArtifact

	// ExecuteResult is the execution result.
	ExecuteResult *execution.V93ExecuteResult

	// AuditEvents is the list of audit events.
	AuditEvents []events.Event

	// Success indicates if the demo completed successfully.
	Success bool

	// FailureReason explains failure if applicable.
	FailureReason string
}

// Run executes the demo.
func (r *Runner) Run() (*DemoResult, error) {
	result := &DemoResult{
		IsConfigured: r.isConfigured,
		AuditEvents:  make([]events.Event, 0),
	}

	// Reset state
	r.auditEvents = make([]events.Event, 0)
	r.idCounter = 0

	now := time.Now()

	// Step 1: Create intent (£1.00 payment)
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_demo_user",
		IntersectionID: "", // Single-party
		Description:    "Payment of GBP 1.00 to sandbox utility",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    100, // £1.00
		Currency:       "GBP",
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_view_hash_" + r.generateID(),
		CreatedAt:      now,
	}
	result.Intent = intent

	r.emitEvent(events.Event{
		ID:          r.generateID(),
		Type:        events.EventExecutionIntentCreated,
		Timestamp:   now,
		CircleID:    intent.CircleID,
		SubjectID:   intent.IntentID,
		SubjectType: "intent",
		Metadata: map[string]string{
			"action_type": string(intent.ActionType),
			"amount":      fmt.Sprintf("%d", intent.AmountCents),
			"currency":    intent.Currency,
			"recipient":   intent.Recipient,
		},
	})

	// Step 2: Build sealed envelope
	revocationWindowDuration := 1 * time.Minute // Shorter for demo
	traceID := r.generateID()
	envelope, err := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                write.DefaultCapCents, // £1.00 cap
		FrequencyCap:             1,
		DurationCap:              1 * time.Hour,
		Expiry:                   now.Add(30 * time.Minute),
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
		ID:          r.generateID(),
		Type:        events.EventExecutionEnvelopeSealed,
		Timestamp:   now,
		CircleID:    intent.CircleID,
		SubjectID:   envelope.EnvelopeID,
		SubjectType: "envelope",
		Metadata: map[string]string{
			"action_hash": envelope.ActionHash[:32],
			"seal_hash":   envelope.SealHash[:32],
			"amount_cap":  fmt.Sprintf("%d", envelope.AmountCap),
		},
	})

	// Step 3: Create approval request
	approvalExpiry := now.Add(15 * time.Minute)
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
		ID:          r.generateID(),
		Type:        events.EventV9ApprovalRequested,
		Timestamp:   now,
		CircleID:    intent.CircleID,
		SubjectID:   approvalReq.RequestID,
		SubjectType: "approval_request",
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"action_hash": approvalReq.ActionHash[:32],
			"expires_at":  approvalReq.ExpiresAt.Format(time.RFC3339),
		},
	})

	// Step 4: Submit approval
	approval, err := r.approvalManager.SubmitApproval(
		approvalReq,
		intent.CircleID,
		"demo_user",
		approvalExpiry,
		now,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("approval submission failed: %v", err)
		return result, err
	}
	result.Approval = approval
	envelope.Approvals = append(envelope.Approvals, *approval)

	r.emitEvent(events.Event{
		ID:          r.generateID(),
		Type:        events.EventV9ApprovalSubmitted,
		Timestamp:   now,
		CircleID:    intent.CircleID,
		SubjectID:   approval.ArtifactID,
		SubjectType: "approval_artifact",
		Metadata: map[string]string{
			"envelope_id": envelope.EnvelopeID,
			"action_hash": approval.ActionHash[:32],
			"approver_id": approval.ApproverID,
		},
	})

	// Step 5: Waive revocation window for demo (explicit waiver)
	envelope.RevocationWaived = true
	r.emitEvent(events.Event{
		ID:          r.generateID(),
		Type:        events.EventV9RevocationWindowClosed,
		Timestamp:   now,
		CircleID:    intent.CircleID,
		SubjectID:   envelope.EnvelopeID,
		SubjectType: "envelope",
		Metadata: map[string]string{
			"waived": "true",
			"reason": "demo explicit waiver",
		},
	})

	// Step 6: Execute payment
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execResult, err := r.executor.Execute(ctx, execution.V93ExecuteRequest{
		Envelope:        envelope,
		Approval:        approval,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true, // Demo simulates --approve flag
		Now:             now,
	})
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("execution failed: %v", err)
		return result, err
	}
	result.ExecuteResult = execResult

	// Step 7: Finalize audit trace
	r.emitEvent(events.Event{
		ID:          r.generateID(),
		Type:        events.EventV9AuditTraceFinalized,
		Timestamp:   time.Now(),
		CircleID:    intent.CircleID,
		SubjectID:   envelope.TraceID,
		SubjectType: "audit_trace",
		Metadata: map[string]string{
			"final_status":  string(execResult.Status),
			"money_moved":   fmt.Sprintf("%t", execResult.MoneyMoved),
			"event_count":   fmt.Sprintf("%d", len(r.auditEvents)),
			"is_configured": fmt.Sprintf("%t", r.isConfigured),
		},
	})

	// Copy audit events to result
	result.AuditEvents = make([]events.Event, len(r.auditEvents))
	copy(result.AuditEvents, r.auditEvents)

	result.Success = execResult.Success
	if !execResult.Success {
		result.FailureReason = execResult.BlockedReason
	}

	return result, nil
}

// PrintResult prints the demo result in a human-readable format.
func PrintResult(result *DemoResult) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("  v9.3 Single-Party Real Financial Execution Demo")
	fmt.Println("============================================================")
	fmt.Println()

	if result.IsConfigured {
		fmt.Println("  TrueLayer: CONFIGURED (Sandbox Mode)")
		fmt.Println("  Real execution may occur with sandbox provider.")
	} else {
		fmt.Println("  TrueLayer: NOT CONFIGURED")
		fmt.Println("  Using mock connector - no real money moves.")
		fmt.Println()
		fmt.Println("  To enable real sandbox execution, set:")
		fmt.Println("    TRUELAYER_CLIENT_ID=<your-client-id>")
		fmt.Println("    TRUELAYER_CLIENT_SECRET=<your-client-secret>")
	}
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  HARD SAFETY CONSTRAINTS")
	fmt.Println("------------------------------------------------------------")
	fmt.Println("  1) Provider: TrueLayer ONLY")
	fmt.Println("  2) Cap: £1.00 (100 pence)")
	fmt.Println("  3) Pre-defined payees only")
	fmt.Println("  4) Explicit per-action approval")
	fmt.Println("  5) Forced pause before execution")
	fmt.Println("  6) No retries")
	fmt.Println("  7) Full audit trail")
	fmt.Println()

	// Print intent
	fmt.Println("1. INTENT CREATED")
	fmt.Printf("   Intent ID: %s\n", result.Intent.IntentID)
	fmt.Printf("   Circle: %s\n", result.Intent.CircleID)
	fmt.Printf("   Action: %s\n", result.Intent.ActionType)
	fmt.Printf("   Amount: £%.2f\n", float64(result.Intent.AmountCents)/100)
	fmt.Printf("   Currency: %s\n", result.Intent.Currency)
	fmt.Printf("   Recipient: %s\n", result.Intent.Recipient)
	fmt.Println()

	// Print envelope
	if result.Envelope != nil {
		fmt.Println("2. ENVELOPE SEALED")
		fmt.Printf("   Envelope ID: %s\n", result.Envelope.EnvelopeID)
		fmt.Printf("   Action Hash: %s...\n", safePrefix(result.Envelope.ActionHash, 32))
		fmt.Printf("   Seal Hash: %s...\n", safePrefix(result.Envelope.SealHash, 32))
		fmt.Printf("   Amount Cap: £%.2f\n", float64(result.Envelope.AmountCap)/100)
		fmt.Printf("   Expiry: %s\n", result.Envelope.Expiry.Format(time.RFC3339))
		fmt.Println()
	}

	// Print approval
	if result.ApprovalRequest != nil {
		fmt.Println("3. APPROVAL REQUESTED")
		fmt.Printf("   Request ID: %s\n", result.ApprovalRequest.RequestID)
		fmt.Printf("   Prompt: %s\n", result.ApprovalRequest.PromptText)
		fmt.Println()
	}

	if result.Approval != nil {
		fmt.Println("4. APPROVAL SUBMITTED")
		fmt.Printf("   Artifact ID: %s\n", result.Approval.ArtifactID)
		fmt.Printf("   Approver: %s\n", result.Approval.ApproverID)
		fmt.Printf("   Action Hash Bound: %s...\n", safePrefix(result.Approval.ActionHash, 32))
		fmt.Println()
	}

	// Print execution result
	if result.ExecuteResult != nil {
		fmt.Println("5. EXECUTION RESULT")
		fmt.Printf("   Status: %s\n", result.ExecuteResult.Status)
		fmt.Printf("   Money Moved: %t\n", result.ExecuteResult.MoneyMoved)

		if result.ExecuteResult.Receipt != nil {
			fmt.Println()
			fmt.Println("   RECEIPT:")
			fmt.Printf("     Receipt ID: %s\n", result.ExecuteResult.Receipt.ReceiptID)
			fmt.Printf("     Provider Ref: %s\n", result.ExecuteResult.Receipt.ProviderRef)
			fmt.Printf("     Amount: £%.2f %s\n",
				float64(result.ExecuteResult.Receipt.AmountCents)/100,
				result.ExecuteResult.Receipt.Currency)
			fmt.Printf("     Payee: %s\n", result.ExecuteResult.Receipt.PayeeID)
			fmt.Printf("     Status: %s\n", result.ExecuteResult.Receipt.Status)
		}

		if result.ExecuteResult.BlockedReason != "" {
			fmt.Printf("   Blocked Reason: %s\n", result.ExecuteResult.BlockedReason)
		}
		fmt.Println()

		fmt.Println("   VALIDATION DETAILS:")
		for _, v := range result.ExecuteResult.ValidationDetails {
			status := "✓"
			if !v.Passed {
				status = "✗"
			}
			fmt.Printf("     %s %s: %s\n", status, v.Check, v.Details)
		}
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
		if event.Provider != "" {
			fmt.Printf("      Provider: %s\n", event.Provider)
		}
	}
	fmt.Println()

	// Print final status
	fmt.Println("============================================================")
	if result.Success {
		if result.IsConfigured && result.ExecuteResult != nil && result.ExecuteResult.MoneyMoved {
			fmt.Println("  DEMO COMPLETED - REAL PAYMENT EXECUTED")
			fmt.Println("  Money was moved via TrueLayer sandbox.")
		} else if result.IsConfigured {
			fmt.Println("  DEMO COMPLETED - PAYMENT INITIATED")
			fmt.Println("  TrueLayer sandbox payment was initiated.")
		} else {
			fmt.Println("  DEMO COMPLETED - MOCK EXECUTION")
			fmt.Println("  No real money moved (TrueLayer not configured).")
		}
	} else {
		fmt.Println("  DEMO BLOCKED")
		fmt.Printf("  Reason: %s\n", result.FailureReason)
		fmt.Println("  No money moved.")
	}
	fmt.Println("============================================================")
}

// safePrefix returns a safe prefix of a string.
func safePrefix(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
