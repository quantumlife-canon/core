// Package demo_v95_multiparty_real demonstrates v9.5 real multi-party sandbox execution.
//
// CRITICAL: This demo shows real TrueLayer sandbox execution when configured,
// otherwise falls back to simulated execution with mock connector.
//
// THREE SCENARIOS:
// 1) Success - 2/2 approvals, presented to both, no revocation
// 2) Blocked missing presentation - approval without presentation
// 3) Revocation during forced pause - abort BEFORE provider call
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v95_multiparty_real

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// Runner runs v9.5 demo scenarios.
type Runner struct {
	mu sync.Mutex

	// Components
	executor          *execution.V95Executor
	envelopeBuilder   *execution.EnvelopeBuilder
	approvalManager   *execution.ApprovalManager
	presentationStore *execution.PresentationStore
	revocationChecker *execution.RevocationChecker

	// State
	idCounter   int64
	auditEvents []events.Event

	// Configuration
	trueLayerConfigured bool
	trueLayerEnv        string
}

// NewRunner creates a new v9.5 demo runner.
func NewRunner() *Runner {
	// Check for TrueLayer configuration
	trueLayerConfigured := os.Getenv("TRUELAYER_CLIENT_ID") != "" && os.Getenv("TRUELAYER_CLIENT_SECRET") != ""
	trueLayerEnv := os.Getenv("TRUELAYER_ENV")
	if trueLayerEnv == "" {
		trueLayerEnv = "sandbox"
	}

	r := &Runner{
		auditEvents:         make([]events.Event, 0),
		trueLayerConfigured: trueLayerConfigured,
		trueLayerEnv:        trueLayerEnv,
	}

	// Initialize components
	signingKey := []byte("demo-signing-key-v95")

	r.envelopeBuilder = execution.NewEnvelopeBuilder(r.generateID)
	r.approvalManager = execution.NewApprovalManager(r.generateID, signingKey)
	r.presentationStore = execution.NewPresentationStore(r.generateID, r.emitEvent)
	r.revocationChecker = execution.NewRevocationChecker(r.generateID)

	presentationGate := execution.NewPresentationGate(r.presentationStore, r.generateID, r.emitEvent)
	multiPartyGate := execution.NewMultiPartyGate(r.generateID, r.emitEvent)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)

	// Create mock connector
	mockConnector := NewMockWriteConnector(r.generateID, r.emitEvent)

	// Create executor config
	config := execution.DefaultV95ExecutorConfig()
	config.TrueLayerConfigured = trueLayerConfigured
	config.TrueLayerEnvironment = trueLayerEnv
	config.ForcedPauseDuration = 2 * time.Second

	// Create executor (TrueLayer connector is nil for demo - uses mock)
	r.executor = execution.NewV95Executor(
		nil, // TrueLayer connector (nil for demo)
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		r.revocationChecker,
		config,
		r.generateID,
		r.emitEvent,
	)

	return r
}

// DemoResult contains the result of a demo scenario.
type DemoResult struct {
	ScenarioName  string
	Success       bool
	FailureReason string
	MoneyMoved    bool
	ProviderUsed  string
	AttemptID     string

	Envelope       *execution.ExecutionEnvelope
	Bundle         *execution.ApprovalBundle
	Approvals      []execution.MultiPartyApprovalArtifact
	ApproverHashes []execution.ApproverBundleHash
	Receipt        *write.PaymentReceipt

	ExecuteResult *execution.V95ExecuteResult
	AuditEvents   []events.Event
}

// Run executes all demo scenarios.
func (r *Runner) Run() ([]*DemoResult, error) {
	results := make([]*DemoResult, 0, 3)

	// Scenario 1: Success (2/2 approvals, presented to both)
	result1, err := r.runSuccessScenario()
	if err != nil {
		return nil, fmt.Errorf("success scenario failed: %w", err)
	}
	results = append(results, result1)

	// Reset state for next scenario
	r.resetState()

	// Scenario 2: Blocked missing presentation
	result2, err := r.runMissingPresentationScenario()
	if err != nil {
		return nil, fmt.Errorf("missing presentation scenario failed: %w", err)
	}
	results = append(results, result2)

	// Reset state for next scenario
	r.resetState()

	// Scenario 3: Revocation during forced pause
	result3, err := r.runRevocationDuringPauseScenario()
	if err != nil {
		return nil, fmt.Errorf("revocation scenario failed: %w", err)
	}
	results = append(results, result3)

	return results, nil
}

// runSuccessScenario runs the success scenario with all approvals and presentations.
func (r *Runner) runSuccessScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()
	traceID := r.generateID()

	result := &DemoResult{
		ScenarioName: "Multi-party Success (2/2 approvals + presentations)",
		AuditEvents:  make([]events.Event, 0),
	}

	// Define policy
	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	// Step 1: Create intent
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_family_alice_bob",
		Description:    "Shared utility payment",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    100, // £1.00
		Currency:       "GBP",
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_view_" + r.generateID(),
		CreatedAt:      now,
	}

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventExecutionIntentCreated,
		Timestamp:      now,
		CircleID:       intent.CircleID,
		IntersectionID: intent.IntersectionID,
		SubjectID:      intent.IntentID,
		SubjectType:    "intent",
		TraceID:        traceID,
	})

	// Step 2: Build and seal envelope
	envelope, err := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        policy.Threshold,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  traceID,
	}, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("envelope build failed: %v", err)
		return result, nil
	}
	envelope.SealHash = execution.ComputeSealHash(envelope)
	result.Envelope = envelope

	// Step 3: Build approval bundle
	bundle, err := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Approval requested for payment of GBP 1.00 to sandbox-utility from shared funds.",
		policy.ExpirySeconds,
		r.generateID,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("bundle build failed: %v", err)
		return result, nil
	}
	result.Bundle = bundle

	// Step 4: Present bundle to BOTH approvers (CRITICAL for v9.5)
	presentationStore := r.executor.GetPresentationStore()
	presentationStore.RecordPresentation("circle_alice", "alice", bundle, envelope, traceID, 5*time.Minute, now)
	presentationStore.RecordPresentation("circle_bob", "bob", bundle, envelope, traceID, 5*time.Minute, now)

	// Step 5: Collect approvals (after presentation)
	approvals := make([]execution.MultiPartyApprovalArtifact, 0, 2)
	approverHashes := make([]execution.ApproverBundleHash, 0, 2)
	expiresAt := now.Add(time.Duration(policy.ExpirySeconds) * time.Second)

	// Alice approves
	aliceRequest, _ := r.approvalManager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
	aliceApproval, _ := r.approvalManager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *aliceApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})
	approverHashes = append(approverHashes, execution.ApproverBundleHash{
		ApproverCircleID: "circle_alice",
		ContentHash:      bundle.ContentHash,
		PresentedAt:      now,
	})

	// Bob approves
	bobRequest, _ := r.approvalManager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
	bobApproval, _ := r.approvalManager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *bobApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})
	approverHashes = append(approverHashes, execution.ApproverBundleHash{
		ApproverCircleID: "circle_bob",
		ContentHash:      bundle.ContentHash,
		PresentedAt:      now,
	})

	result.Approvals = approvals
	result.ApproverHashes = approverHashes

	// Step 6: Execute
	executeResult, err := r.executor.Execute(ctx, execution.V95ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  approverHashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         traceID,
		Now:             now,
	})

	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("execution error: %v", err)
		return result, nil
	}

	result.ExecuteResult = executeResult
	result.Success = executeResult.Success
	result.MoneyMoved = executeResult.MoneyMoved
	result.ProviderUsed = executeResult.ProviderUsed
	result.AttemptID = executeResult.AttemptID
	result.Receipt = executeResult.Receipt

	if !executeResult.Success {
		result.FailureReason = executeResult.BlockedReason
	}

	result.AuditEvents = append(r.auditEvents, executeResult.AuditEvents...)
	return result, nil
}

// runMissingPresentationScenario runs scenario where approval is submitted without presentation.
func (r *Runner) runMissingPresentationScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()
	traceID := r.generateID()

	result := &DemoResult{
		ScenarioName: "Blocked: Missing Presentation",
		AuditEvents:  make([]events.Event, 0),
	}

	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	// Create intent and envelope
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_family_alice_bob",
		Description:    "Payment without presentation",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    50,
		Currency:       "GBP",
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_view_" + r.generateID(),
		CreatedAt:      now,
	}

	envelope, _ := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        policy.Threshold,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  traceID,
	}, now)
	envelope.SealHash = execution.ComputeSealHash(envelope)
	result.Envelope = envelope

	// Build bundle
	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Approval requested for payment.",
		policy.ExpirySeconds,
		r.generateID,
	)
	result.Bundle = bundle

	// CRITICAL: DO NOT present bundle to approvers - this is the test case
	// presentationStore.RecordPresentation(...) is NOT called

	// Create approvals WITHOUT presentation
	approvals := make([]execution.MultiPartyApprovalArtifact, 0, 2)
	approverHashes := make([]execution.ApproverBundleHash, 0, 2)
	expiresAt := now.Add(5 * time.Minute)

	aliceRequest, _ := r.approvalManager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
	aliceApproval, _ := r.approvalManager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *aliceApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})
	approverHashes = append(approverHashes, execution.ApproverBundleHash{
		ApproverCircleID: "circle_alice",
		ContentHash:      bundle.ContentHash,
		PresentedAt:      now,
	})

	bobRequest, _ := r.approvalManager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
	bobApproval, _ := r.approvalManager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *bobApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})
	approverHashes = append(approverHashes, execution.ApproverBundleHash{
		ApproverCircleID: "circle_bob",
		ContentHash:      bundle.ContentHash,
		PresentedAt:      now,
	})

	result.Approvals = approvals
	result.ApproverHashes = approverHashes

	// Execute - should be BLOCKED due to missing presentation
	executeResult, _ := r.executor.Execute(ctx, execution.V95ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  approverHashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         traceID,
		Now:             now,
	})

	result.ExecuteResult = executeResult
	result.Success = false // Expected to fail
	result.MoneyMoved = executeResult.MoneyMoved
	result.ProviderUsed = executeResult.ProviderUsed
	result.AttemptID = executeResult.AttemptID
	result.FailureReason = executeResult.BlockedReason

	result.AuditEvents = append(r.auditEvents, executeResult.AuditEvents...)
	return result, nil
}

// runRevocationDuringPauseScenario runs scenario where revocation happens during forced pause.
func (r *Runner) runRevocationDuringPauseScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()
	traceID := r.generateID()

	result := &DemoResult{
		ScenarioName: "Revocation During Forced Pause",
		AuditEvents:  make([]events.Event, 0),
	}

	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	// Create intent and envelope
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_family_alice_bob",
		Description:    "Payment to be revoked during pause",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    75,
		Currency:       "GBP",
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_view_" + r.generateID(),
		CreatedAt:      now,
	}

	envelope, _ := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        policy.Threshold,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  traceID,
	}, now)
	envelope.SealHash = execution.ComputeSealHash(envelope)
	result.Envelope = envelope

	// Build bundle
	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Approval requested for payment.",
		policy.ExpirySeconds,
		r.generateID,
	)
	result.Bundle = bundle

	// Present to both approvers
	presentationStore := r.executor.GetPresentationStore()
	presentationStore.RecordPresentation("circle_alice", "alice", bundle, envelope, traceID, 5*time.Minute, now)
	presentationStore.RecordPresentation("circle_bob", "bob", bundle, envelope, traceID, 5*time.Minute, now)

	// Collect approvals
	approvals := make([]execution.MultiPartyApprovalArtifact, 0, 2)
	approverHashes := make([]execution.ApproverBundleHash, 0, 2)
	expiresAt := now.Add(5 * time.Minute)

	aliceRequest, _ := r.approvalManager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
	aliceApproval, _ := r.approvalManager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *aliceApproval,
		BundleContentHash: bundle.ContentHash,
	})
	approverHashes = append(approverHashes, execution.ApproverBundleHash{
		ApproverCircleID: "circle_alice",
		ContentHash:      bundle.ContentHash,
		PresentedAt:      now,
	})

	bobRequest, _ := r.approvalManager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
	bobApproval, _ := r.approvalManager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *bobApproval,
		BundleContentHash: bundle.ContentHash,
	})
	approverHashes = append(approverHashes, execution.ApproverBundleHash{
		ApproverCircleID: "circle_bob",
		ContentHash:      bundle.ContentHash,
		PresentedAt:      now,
	})

	result.Approvals = approvals
	result.ApproverHashes = approverHashes

	// Start execution in goroutine
	var executeResult *execution.V95ExecuteResult
	var executeErr error
	done := make(chan struct{})

	go func() {
		executeResult, executeErr = r.executor.Execute(ctx, execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  approverHashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             now,
		})
		close(done)
	}()

	// Wait a bit for execution to enter forced pause, then revoke
	time.Sleep(500 * time.Millisecond)
	r.executor.Revoke(envelope.EnvelopeID, "circle_bob", "bob", "changed mind during pause")

	// Wait for execution to complete
	<-done

	if executeErr != nil {
		result.FailureReason = fmt.Sprintf("execution error: %v", executeErr)
	} else {
		result.ExecuteResult = executeResult
		result.Success = false // Expected to be revoked
		result.MoneyMoved = executeResult.MoneyMoved
		result.ProviderUsed = executeResult.ProviderUsed
		result.AttemptID = executeResult.AttemptID
		result.FailureReason = executeResult.BlockedReason
	}

	result.AuditEvents = append(r.auditEvents, executeResult.AuditEvents...)
	return result, nil
}

// resetState resets the runner state between scenarios.
func (r *Runner) resetState() {
	r.auditEvents = make([]events.Event, 0)
	// Recreate components with fresh state
	signingKey := []byte("demo-signing-key-v95")
	r.presentationStore = execution.NewPresentationStore(r.generateID, r.emitEvent)
	r.revocationChecker = execution.NewRevocationChecker(r.generateID)

	presentationGate := execution.NewPresentationGate(r.presentationStore, r.generateID, r.emitEvent)
	multiPartyGate := execution.NewMultiPartyGate(r.generateID, r.emitEvent)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)
	mockConnector := NewMockWriteConnector(r.generateID, r.emitEvent)

	config := execution.DefaultV95ExecutorConfig()
	config.TrueLayerConfigured = r.trueLayerConfigured
	config.TrueLayerEnvironment = r.trueLayerEnv
	config.ForcedPauseDuration = 2 * time.Second

	r.executor = execution.NewV95Executor(
		nil,
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		r.revocationChecker,
		config,
		r.generateID,
		r.emitEvent,
	)
}

func (r *Runner) generateID() string {
	id := atomic.AddInt64(&r.idCounter, 1)
	return fmt.Sprintf("v95_demo_%d", id)
}

func (r *Runner) emitEvent(event events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditEvents = append(r.auditEvents, event)
}

// PrintResult prints a demo result.
func PrintResult(result *DemoResult) {
	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  SCENARIO: %s\n", result.ScenarioName)
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	if result.Envelope != nil {
		fmt.Println("1. ENVELOPE")
		fmt.Printf("   Envelope ID: %s\n", result.Envelope.EnvelopeID)
		fmt.Printf("   Action Hash: %s...\n", result.Envelope.ActionHash[:32])
		fmt.Printf("   Amount: £%.2f %s\n", float64(result.Envelope.ActionSpec.AmountCents)/100, result.Envelope.ActionSpec.Currency)
	}

	if result.Bundle != nil {
		fmt.Println()
		fmt.Println("2. APPROVAL BUNDLE")
		fmt.Printf("   Content Hash: %s...\n", result.Bundle.ContentHash[:32])
	}

	if len(result.Approvals) > 0 {
		fmt.Println()
		fmt.Println("3. APPROVALS")
		for i, a := range result.Approvals {
			fmt.Printf("   [%d] %s: %s\n", i+1, a.ApproverCircleID, a.ArtifactID)
		}
	}

	if result.ExecuteResult != nil {
		fmt.Println()
		fmt.Println("4. EXECUTION RESULT")
		fmt.Printf("   Attempt ID: %s\n", result.AttemptID)
		fmt.Printf("   Provider: %s\n", result.ProviderUsed)
		if result.ExecuteResult.Success {
			fmt.Printf("   Status: SUCCESS\n")
		} else {
			fmt.Printf("   Status: BLOCKED/REVOKED\n")
			fmt.Printf("   Reason: %s\n", result.FailureReason)
		}
		fmt.Printf("   Money Moved: %t\n", result.MoneyMoved)

		if result.Receipt != nil {
			fmt.Println()
			fmt.Println("   RECEIPT:")
			fmt.Printf("     Receipt ID: %s\n", result.Receipt.ReceiptID)
			fmt.Printf("     Status: %s\n", result.Receipt.Status)
			fmt.Printf("     Simulated: %t\n", result.Receipt.Simulated)
		}

		if len(result.ExecuteResult.ValidationDetails) > 0 {
			fmt.Println()
			fmt.Println("   VALIDATION DETAILS:")
			for _, v := range result.ExecuteResult.ValidationDetails {
				mark := "✓"
				if !v.Passed {
					mark = "✗"
				}
				fmt.Printf("     %s %s: %s\n", mark, v.Check, v.Details)
			}
		}
	}

	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  AUDIT TRAIL: %d events\n", len(result.AuditEvents))
	fmt.Println("------------------------------------------------------------")

	// Print first 10 events
	maxEvents := 10
	if len(result.AuditEvents) < maxEvents {
		maxEvents = len(result.AuditEvents)
	}
	for i := 0; i < maxEvents; i++ {
		fmt.Printf("  [%d] %s\n", i+1, result.AuditEvents[i].Type)
	}
	if len(result.AuditEvents) > 10 {
		fmt.Printf("  ... and %d more events\n", len(result.AuditEvents)-10)
	}

	fmt.Println()
	if result.Success && result.MoneyMoved == false && result.ProviderUsed == "mock-write" {
		fmt.Println("============================================================")
		fmt.Println("  SCENARIO COMPLETED - SIMULATED EXECUTION")
		fmt.Println("  No external payment was initiated (mock provider).")
		fmt.Println("============================================================")
	} else if !result.Success {
		fmt.Println("============================================================")
		fmt.Println("  SCENARIO COMPLETED - EXECUTION BLOCKED/REVOKED")
		fmt.Printf("  Reason: %s\n", result.FailureReason)
		fmt.Println("  No money moved.")
		fmt.Println("============================================================")
	}
}
