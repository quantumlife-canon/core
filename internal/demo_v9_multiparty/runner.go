// Package demo_v9_multiparty demonstrates v9.4 multi-party financial execution.
//
// CRITICAL: This is the FIRST multi-party demo where money may actually move.
// It must enforce all v9.3 constraints PLUS multi-party symmetry verification.
//
// HARD SAFETY CONSTRAINTS (NON-NEGOTIABLE):
// 1) All v9.3 constraints remain in force
// 2) No blanket/standing approvals - each approval binds to specific ActionHash
// 3) Neutral approval language - reject urgency/fear/shame/authority/optimization
// 4) Symmetry - every approver receives IDENTICAL approval payload (provable)
// 5) Approvals do NOT bypass revocation windows
// 6) Single-use approvals only
// 7) Mock providers MUST report MoneyMoved=false
//
// Demo scenarios:
// 1) Successful multi-party execution (2 approvers, threshold=2)
// 2) Blocked due to insufficient approvals (1 of 2 missing)
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v9_multiparty

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

// Runner orchestrates v9.4 multi-party execution demos.
type Runner struct {
	envelopeBuilder   *execution.EnvelopeBuilder
	approvalManager   *execution.ApprovalManager
	approvalVerifier  *execution.ApprovalVerifier
	revocationChecker *execution.RevocationChecker
	multiPartyGate    *execution.MultiPartyGate
	executor          *execution.V94Executor
	payeeRegistry     *write.PayeeRegistry

	connector    write.WriteConnector
	isConfigured bool

	auditEvents []events.Event
	idCounter   uint64
	signingKey  []byte
}

// NewRunner creates a new demo runner.
func NewRunner() *Runner {
	signingKey := []byte("demo-signing-key-v9-multiparty")

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
	r.multiPartyGate = execution.NewMultiPartyGate(idGen, emitter)

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
	r.executor = execution.NewV94Executor(
		r.connector,
		r.multiPartyGate,
		r.approvalVerifier,
		r.revocationChecker,
		execution.DefaultV94ExecutorConfig(),
		idGen,
		emitter,
	)

	return r
}

// generateID generates sequential IDs for demo purposes.
func (r *Runner) generateID() string {
	id := atomic.AddUint64(&r.idCounter, 1)
	return fmt.Sprintf("v94_demo_%d", id)
}

// emitEvent records an audit event.
func (r *Runner) emitEvent(event events.Event) {
	r.auditEvents = append(r.auditEvents, event)
}

// DemoResult contains the result of running the demo.
type DemoResult struct {
	// ScenarioName identifies which scenario ran.
	ScenarioName string

	// IsConfigured indicates if TrueLayer is configured.
	IsConfigured bool

	// Intent is the execution intent.
	Intent execution.ExecutionIntent

	// Envelope is the sealed envelope.
	Envelope *execution.ExecutionEnvelope

	// Bundle is the approval bundle.
	Bundle *execution.ApprovalBundle

	// Approvals are the multi-party approvals.
	Approvals []execution.MultiPartyApprovalArtifact

	// ApproverHashes are the hashes each approver received.
	ApproverHashes []execution.ApproverBundleHash

	// SymmetryProof is the symmetry verification proof.
	SymmetryProof *execution.SymmetryProof

	// Policy is the multi-party policy used.
	Policy *execution.MultiPartyPolicy

	// ExecuteResult is the execution result.
	ExecuteResult *execution.V94ExecuteResult

	// AuditEvents is the list of audit events.
	AuditEvents []events.Event

	// Success indicates if the demo completed successfully.
	Success bool

	// FailureReason explains failure if applicable.
	FailureReason string
}

// RunSuccessScenario runs the successful multi-party execution scenario.
func (r *Runner) RunSuccessScenario() (*DemoResult, error) {
	result := &DemoResult{
		ScenarioName: "Multi-party Success (2/2 approvers)",
		IsConfigured: r.isConfigured,
		AuditEvents:  make([]events.Event, 0),
	}

	// Reset state
	r.auditEvents = make([]events.Event, 0)
	r.idCounter = 0

	now := time.Now()

	// Step 1: Create policy (2 approvers, threshold=2)
	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300, // 5 minutes
		AppliesToScopes:   []string{"finance:write"},
	}
	result.Policy = policy

	// Step 2: Create intent (£1.00 payment from family intersection)
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_alice", // Initiator
		IntersectionID: "intersection_family_alice_bob",
		Description:    "Payment of GBP 1.00 to sandbox utility from shared funds",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    100, // £1.00
		Currency:       "GBP",
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_shared_view_" + r.generateID(),
		CreatedAt:      now,
	}
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
			"amount":      fmt.Sprintf("%d", intent.AmountCents),
			"currency":    intent.Currency,
			"recipient":   intent.Recipient,
			"multi_party": "true",
			"threshold":   fmt.Sprintf("%d", policy.Threshold),
		},
	})

	// Step 3: Build and seal envelope
	envelope, err := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        policy.Threshold,
		RevocationWindowDuration: 0, // Waived for demo
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100, // £1.00
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  r.generateID(),
	}, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("envelope build failed: %v", err)
		return result, nil
	}
	envelope.SealHash = execution.ComputeSealHash(envelope)
	result.Envelope = envelope

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventExecutionEnvelopeSealed,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"action_hash": envelope.ActionHash,
			"seal_hash":   envelope.SealHash,
			"amount":      fmt.Sprintf("%d", envelope.ActionSpec.AmountCents),
		},
	})

	// Step 4: Build approval bundle
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

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV94ApprovalBundleCreated,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "bundle",
		Metadata: map[string]string{
			"content_hash": bundle.ContentHash,
			"action_hash":  bundle.ActionHash,
		},
	})

	// Step 5: Present bundle to each approver (record hashes)
	approverHashes := []execution.ApproverBundleHash{
		{
			ApproverCircleID: "circle_alice",
			ContentHash:      bundle.ContentHash, // Same hash
			PresentedAt:      now,
		},
		{
			ApproverCircleID: "circle_bob",
			ContentHash:      bundle.ContentHash, // Same hash - symmetry!
			PresentedAt:      now,
		},
	}
	result.ApproverHashes = approverHashes

	for _, ah := range approverHashes {
		r.emitEvent(events.Event{
			ID:             r.generateID(),
			Type:           events.EventV94ApprovalBundlePresented,
			Timestamp:      ah.PresentedAt,
			CircleID:       ah.ApproverCircleID,
			IntersectionID: envelope.IntersectionID,
			SubjectID:      envelope.EnvelopeID,
			SubjectType:    "bundle",
			Metadata: map[string]string{
				"content_hash": ah.ContentHash,
			},
		})
	}

	// Step 6: Collect approvals from both parties
	approvals := make([]execution.MultiPartyApprovalArtifact, 0, 2)
	expiresAt := now.Add(time.Duration(policy.ExpirySeconds) * time.Second)

	// Alice approves - create request then submit
	aliceRequest, err := r.approvalManager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Alice approval request failed: %v", err)
		return result, nil
	}
	aliceApproval, err := r.approvalManager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Alice approval submission failed: %v", err)
		return result, nil
	}
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *aliceApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9ApprovalSubmitted,
		Timestamp:      aliceApproval.ApprovedAt,
		CircleID:       aliceApproval.ApproverCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      aliceApproval.ArtifactID,
		SubjectType:    "approval",
		Metadata: map[string]string{
			"action_hash": aliceApproval.ActionHash,
			"bundle_hash": bundle.ContentHash,
		},
	})

	// Bob approves - create request then submit
	bobRequest, err := r.approvalManager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Bob approval request failed: %v", err)
		return result, nil
	}
	bobApproval, err := r.approvalManager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Bob approval submission failed: %v", err)
		return result, nil
	}
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *bobApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9ApprovalSubmitted,
		Timestamp:      bobApproval.ApprovedAt,
		CircleID:       bobApproval.ApproverCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      bobApproval.ArtifactID,
		SubjectType:    "approval",
		Metadata: map[string]string{
			"action_hash": bobApproval.ActionHash,
			"bundle_hash": bundle.ContentHash,
		},
	})

	result.Approvals = approvals

	// Step 7: Close revocation window
	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9RevocationWindowClosed,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
		Metadata: map[string]string{
			"waived": "true",
		},
	})

	// Step 8: Execute with multi-party gate
	executeResult, err := r.executor.Execute(context.Background(), execution.V94ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  approverHashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		Now:             now,
	})
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("execution error: %v", err)
		return result, nil
	}
	result.ExecuteResult = executeResult

	// Capture symmetry proof from gate result
	if executeResult.GateResult != nil && executeResult.GateResult.SymmetryProof != nil {
		result.SymmetryProof = executeResult.GateResult.SymmetryProof
	}

	// Finalize
	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9AuditTraceFinalized,
		Timestamp:      time.Now(),
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "trace",
		Metadata: map[string]string{
			"event_count": fmt.Sprintf("%d", len(r.auditEvents)+len(executeResult.AuditEvents)),
			"multi_party": "true",
			"threshold":   fmt.Sprintf("%d", policy.Threshold),
			"approved_by": fmt.Sprintf("%v", policy.RequiredApprovers),
		},
	})

	result.AuditEvents = append(r.auditEvents, executeResult.AuditEvents...)
	result.Success = executeResult.Success
	if !executeResult.Success {
		result.FailureReason = executeResult.BlockedReason
	}

	return result, nil
}

// RunBlockedScenario runs the blocked multi-party execution scenario (missing approver).
func (r *Runner) RunBlockedScenario() (*DemoResult, error) {
	result := &DemoResult{
		ScenarioName: "Multi-party Blocked (1/2 approvers missing)",
		IsConfigured: r.isConfigured,
		AuditEvents:  make([]events.Event, 0),
	}

	// Reset state
	r.auditEvents = make([]events.Event, 0)
	r.idCounter = 0

	now := time.Now()

	// Step 1: Create policy (2 approvers required, but we'll only have 1)
	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}
	result.Policy = policy

	// Step 2: Create intent
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_family_alice_bob",
		Description:    "Payment of GBP 0.50 to sandbox utility from shared funds",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    50, // £0.50
		Currency:       "GBP",
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_shared_view_" + r.generateID(),
		CreatedAt:      now,
	}
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
			"amount":      fmt.Sprintf("%d", intent.AmountCents),
			"multi_party": "true",
			"threshold":   fmt.Sprintf("%d", policy.Threshold),
		},
	})

	// Step 3: Build and seal envelope
	envelope, err := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        policy.Threshold,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100, // £1.00
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  r.generateID(),
	}, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("envelope build failed: %v", err)
		return result, nil
	}
	envelope.SealHash = execution.ComputeSealHash(envelope)
	result.Envelope = envelope

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventExecutionEnvelopeSealed,
		Timestamp:      now,
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "envelope",
	})

	// Step 4: Build approval bundle
	bundle, err := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Approval requested for payment of GBP 0.50 to sandbox-utility from shared funds.",
		policy.ExpirySeconds,
		r.generateID,
	)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("bundle build failed: %v", err)
		return result, nil
	}
	result.Bundle = bundle

	// Step 5: Only Alice approves (Bob is missing)
	approverHashes := []execution.ApproverBundleHash{
		{
			ApproverCircleID: "circle_alice",
			ContentHash:      bundle.ContentHash,
			PresentedAt:      now,
		},
		// Bob doesn't receive bundle (or doesn't approve)
	}
	result.ApproverHashes = approverHashes

	approvals := make([]execution.MultiPartyApprovalArtifact, 0, 1)
	expiresAt := now.Add(time.Duration(policy.ExpirySeconds) * time.Second)

	// Only Alice approves - create request then submit
	aliceRequest, err := r.approvalManager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Alice approval request failed: %v", err)
		return result, nil
	}
	aliceApproval, err := r.approvalManager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Alice approval submission failed: %v", err)
		return result, nil
	}
	approvals = append(approvals, execution.MultiPartyApprovalArtifact{
		ApprovalArtifact:  *aliceApproval,
		BundleContentHash: bundle.ContentHash,
		Used:              false,
	})

	result.Approvals = approvals

	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9ApprovalSubmitted,
		Timestamp:      aliceApproval.ApprovedAt,
		CircleID:       aliceApproval.ApproverCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      aliceApproval.ArtifactID,
		SubjectType:    "approval",
		Metadata: map[string]string{
			"action_hash": aliceApproval.ActionHash,
		},
	})

	// Step 6: Attempt execution (should be blocked)
	executeResult, err := r.executor.Execute(context.Background(), execution.V94ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  approverHashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		Now:             now,
	})
	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("execution error: %v", err)
		return result, nil
	}
	result.ExecuteResult = executeResult

	// Finalize
	r.emitEvent(events.Event{
		ID:             r.generateID(),
		Type:           events.EventV9AuditTraceFinalized,
		Timestamp:      time.Now(),
		CircleID:       envelope.ActorCircleID,
		IntersectionID: envelope.IntersectionID,
		SubjectID:      envelope.EnvelopeID,
		SubjectType:    "trace",
		Metadata: map[string]string{
			"blocked": "true",
			"reason":  "insufficient approvals",
		},
	})

	result.AuditEvents = append(r.auditEvents, executeResult.AuditEvents...)
	result.Success = false // This scenario is expected to be blocked
	result.FailureReason = executeResult.BlockedReason

	return result, nil
}

// Run executes both demo scenarios.
func (r *Runner) Run() ([]*DemoResult, error) {
	results := make([]*DemoResult, 0, 2)

	// Scenario 1: Success
	successResult, err := r.RunSuccessScenario()
	if err != nil {
		return nil, err
	}
	results = append(results, successResult)

	// Reset for next scenario
	r.revocationChecker = execution.NewRevocationChecker(r.generateID)
	r.multiPartyGate = execution.NewMultiPartyGate(r.generateID, r.emitEvent)
	r.executor = execution.NewV94Executor(
		r.connector,
		r.multiPartyGate,
		r.approvalVerifier,
		r.revocationChecker,
		execution.DefaultV94ExecutorConfig(),
		r.generateID,
		r.emitEvent,
	)

	// Scenario 2: Blocked
	blockedResult, err := r.RunBlockedScenario()
	if err != nil {
		return nil, err
	}
	results = append(results, blockedResult)

	return results, nil
}

// PrintResult prints a demo result.
func PrintResult(result *DemoResult) {
	fmt.Printf("\n------------------------------------------------------------\n")
	fmt.Printf("  SCENARIO: %s\n", result.ScenarioName)
	fmt.Printf("------------------------------------------------------------\n")

	fmt.Printf("\n1. INTENT CREATED\n")
	fmt.Printf("   Intent ID: %s\n", result.Intent.IntentID)
	fmt.Printf("   Circle: %s\n", result.Intent.CircleID)
	fmt.Printf("   Intersection: %s\n", result.Intent.IntersectionID)
	fmt.Printf("   Action: %s\n", result.Intent.ActionType)
	fmt.Printf("   Amount: £%.2f\n", float64(result.Intent.AmountCents)/100)
	fmt.Printf("   Currency: %s\n", result.Intent.Currency)
	fmt.Printf("   Recipient: %s\n", result.Intent.Recipient)

	if result.Envelope != nil {
		fmt.Printf("\n2. ENVELOPE SEALED\n")
		fmt.Printf("   Envelope ID: %s\n", result.Envelope.EnvelopeID)
		fmt.Printf("   Action Hash: %s...\n", safePrefix(result.Envelope.ActionHash, 32))
		fmt.Printf("   Seal Hash: %s...\n", safePrefix(result.Envelope.SealHash, 32))
		fmt.Printf("   Threshold: %d\n", result.Envelope.ApprovalThreshold)
	}

	if result.Bundle != nil {
		fmt.Printf("\n3. APPROVAL BUNDLE CREATED\n")
		fmt.Printf("   Content Hash: %s...\n", safePrefix(result.Bundle.ContentHash, 32))
		fmt.Printf("   Neutrality: %t\n", result.Bundle.NeutralityAttestation.Verified)
	}

	if result.Policy != nil {
		fmt.Printf("\n4. MULTI-PARTY POLICY\n")
		fmt.Printf("   Mode: %s\n", result.Policy.Mode)
		fmt.Printf("   Threshold: %d\n", result.Policy.Threshold)
		fmt.Printf("   Required Approvers: %v\n", result.Policy.RequiredApprovers)
	}

	if len(result.Approvals) > 0 {
		fmt.Printf("\n5. APPROVALS COLLECTED\n")
		for i, approval := range result.Approvals {
			fmt.Printf("   [%d] %s: %s\n", i+1, approval.ApproverCircleID, approval.ArtifactID)
		}
	}

	if result.SymmetryProof != nil {
		fmt.Printf("\n6. SYMMETRY VERIFICATION\n")
		fmt.Printf("   Proof ID: %s\n", result.SymmetryProof.ProofID)
		fmt.Printf("   Symmetric: %t\n", result.SymmetryProof.Symmetric)
		fmt.Printf("   Bundle Hash: %s...\n", safePrefix(result.SymmetryProof.BundleContentHash, 32))
	}

	if result.ExecuteResult != nil {
		fmt.Printf("\n7. EXECUTION RESULT\n")
		fmt.Printf("   Status: %s\n", result.ExecuteResult.Status)
		fmt.Printf("   Money Moved: %t\n", result.ExecuteResult.MoneyMoved)

		if result.ExecuteResult.Receipt != nil {
			fmt.Println()
			fmt.Println("   RECEIPT:")
			fmt.Printf("     Receipt ID: %s\n", result.ExecuteResult.Receipt.ReceiptID)
			fmt.Printf("     Amount: £%.2f %s\n",
				float64(result.ExecuteResult.Receipt.AmountCents)/100,
				result.ExecuteResult.Receipt.Currency)
			fmt.Printf("     Status: %s\n", result.ExecuteResult.Receipt.Status)
			if result.ExecuteResult.Receipt.Simulated {
				fmt.Println("     Simulated: true (no external payment was initiated)")
			}
		}

		if len(result.ExecuteResult.ValidationDetails) > 0 {
			fmt.Println()
			fmt.Println("   VALIDATION DETAILS:")
			for _, detail := range result.ExecuteResult.ValidationDetails {
				mark := "✓"
				if !detail.Passed {
					mark = "✗"
				}
				fmt.Printf("     %s %s: %s\n", mark, detail.Check, detail.Details)
			}
		}

		if result.ExecuteResult.BlockedReason != "" {
			fmt.Printf("\n   Blocked Reason: %s\n", result.ExecuteResult.BlockedReason)
		}
	}

	fmt.Printf("\n------------------------------------------------------------\n")
	fmt.Printf("  AUDIT TRAIL SUMMARY\n")
	fmt.Printf("------------------------------------------------------------\n")
	fmt.Printf("  Total Events: %d\n\n", len(result.AuditEvents))

	eventCount := 0
	for _, event := range result.AuditEvents {
		eventCount++
		if eventCount <= 20 { // Limit output
			fmt.Printf("  [%d] %s\n", eventCount, event.Type)
		}
	}
	if eventCount > 20 {
		fmt.Printf("  ... and %d more events\n", eventCount-20)
	}

	fmt.Println()
	fmt.Println("============================================================")
	if result.Success {
		if result.ExecuteResult != nil && result.ExecuteResult.MoneyMoved {
			fmt.Println("  SCENARIO COMPLETED - REAL PAYMENT EXECUTED")
		} else if result.IsConfigured {
			fmt.Println("  SCENARIO COMPLETED - PAYMENT INITIATED")
		} else {
			fmt.Println("  SCENARIO COMPLETED - SIMULATED EXECUTION")
			fmt.Println("  Execution completed in simulated mode.")
			fmt.Println("  No external payment was initiated.")
		}
	} else {
		fmt.Println("  SCENARIO COMPLETED - EXECUTION BLOCKED")
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
