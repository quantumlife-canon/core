// Package demo_v96_idempotency demonstrates v9.6 idempotency and replay defense.
//
// CRITICAL: This demo proves the following protections:
// 1) Double CLI invocation with same attempt ID is blocked
// 2) Two attempts for same envelope while first in-flight is blocked
// 3) After terminal settle/simulated, replay is blocked
// 4) Mock provider respects idempotency + MoneyMoved=false
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v96_idempotency

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/events"
)

// Runner executes the v9.6 idempotency demo scenarios.
type Runner struct {
	mu sync.Mutex

	// Components
	envelopeBuilder   *execution.EnvelopeBuilder
	approvalManager   *execution.ApprovalManager
	presentationStore *execution.PresentationStore
	revocationChecker *execution.RevocationChecker
	attemptLedger     *attempts.InMemoryLedger
	executor          *execution.V96Executor

	// State
	idCounter   int64
	auditEvents []events.Event

	// Config
	trueLayerConfigured bool
	trueLayerEnv        string
}

// NewRunner creates a new v9.6 demo runner.
func NewRunner() *Runner {
	return NewRunnerWithConfig(false, "sandbox")
}

// NewRunnerWithConfig creates a runner with specific configuration.
func NewRunnerWithConfig(trueLayerConfigured bool, trueLayerEnv string) *Runner {
	r := &Runner{
		auditEvents:         make([]events.Event, 0),
		trueLayerConfigured: trueLayerConfigured,
		trueLayerEnv:        trueLayerEnv,
	}

	// Initialize components
	signingKey := []byte("demo-signing-key-v96")

	r.envelopeBuilder = execution.NewEnvelopeBuilder(r.generateID)
	r.approvalManager = execution.NewApprovalManager(r.generateID, signingKey)
	r.presentationStore = execution.NewPresentationStore(r.generateID, r.emitEvent)
	r.revocationChecker = execution.NewRevocationChecker(r.generateID)
	r.attemptLedger = attempts.NewInMemoryLedger(
		attempts.DefaultLedgerConfig(),
		r.generateID,
		r.emitEvent,
	)

	presentationGate := execution.NewPresentationGate(r.presentationStore, r.generateID, r.emitEvent)
	multiPartyGate := execution.NewMultiPartyGate(r.generateID, r.emitEvent)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)

	// Create mock connector
	mockConnector := NewMockWriteConnector(r.generateID, r.emitEvent)

	// Create executor config
	config := execution.DefaultV96ExecutorConfig()
	config.TrueLayerConfigured = trueLayerConfigured
	config.TrueLayerEnvironment = trueLayerEnv
	config.ForcedPauseDuration = 500 * time.Millisecond // Shorter for demo
	config.RevocationPollInterval = 50 * time.Millisecond

	// Create executor
	r.executor = execution.NewV96Executor(
		nil, // TrueLayer connector (nil for demo)
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		r.revocationChecker,
		r.attemptLedger,
		config,
		r.generateID,
		r.emitEvent,
	)

	return r
}

// generateID generates a unique ID.
func (r *Runner) generateID() string {
	id := atomic.AddInt64(&r.idCounter, 1)
	return fmt.Sprintf("v96_demo_%d", id)
}

// emitEvent records an audit event.
func (r *Runner) emitEvent(event events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditEvents = append(r.auditEvents, event)
}

// DemoResult contains the result of a demo scenario.
type DemoResult struct {
	ScenarioName      string
	Success           bool
	ExecuteResult     *execution.V96ExecuteResult
	ReplayBlocked     bool
	InflightBlocked   bool
	AuditEventCount   int
	IdempotencyPrefix string
	AttemptID         string
	MoneyMoved        bool
	Description       string
}

// Run executes all demo scenarios.
func (r *Runner) Run() ([]*DemoResult, error) {
	results := make([]*DemoResult, 0, 4)

	// Scenario 1: Double-invoke same attempt ID (replay blocked)
	result1, err := r.runDoubleInvokeScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 1 failed: %w", err)
	}
	results = append(results, result1)

	// Scenario 2: In-flight blocking (new runner to reset state)
	r2 := NewRunnerWithConfig(r.trueLayerConfigured, r.trueLayerEnv)
	result2, err := r2.runInflightBlockedScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 2 failed: %w", err)
	}
	results = append(results, result2)

	// Scenario 3: Terminal replay blocked (new runner)
	r3 := NewRunnerWithConfig(r.trueLayerConfigured, r.trueLayerEnv)
	result3, err := r3.runTerminalReplayScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 3 failed: %w", err)
	}
	results = append(results, result3)

	// Scenario 4: Mock idempotency + MoneyMoved=false (new runner)
	r4 := NewRunnerWithConfig(r.trueLayerConfigured, r.trueLayerEnv)
	result4, err := r4.runMockIdempotencyScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 4 failed: %w", err)
	}
	results = append(results, result4)

	return results, nil
}

// runDoubleInvokeScenario demonstrates replay blocking for same attempt ID.
func (r *Runner) runDoubleInvokeScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()

	// Create envelope
	envelope, bundle := r.createTestEnvelope(100, "GBP", now)
	traceID := r.generateID()

	// Create approvals with presentations
	approvals, hashes := r.createApprovalsWithPresentation(envelope, bundle, traceID, now)

	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	// First invocation - should succeed
	attemptID := attempts.DeriveAttemptID(envelope.EnvelopeID, 1)
	req := execution.V96ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  hashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         traceID,
		AttemptID:       attemptID,
		Now:             now,
	}

	result1, err := r.executor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("first invoke failed: %w", err)
	}

	if !result1.Success {
		return nil, fmt.Errorf("first invoke should succeed, got: %s", result1.BlockedReason)
	}

	// Second invocation with SAME attempt ID - should be blocked
	req.Now = time.Now()
	result2, err := r.executor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("second invoke error: %w", err)
	}

	return &DemoResult{
		ScenarioName:      "Double-Invoke Same Attempt ID",
		Success:           !result2.Success && result2.ReplayBlocked,
		ExecuteResult:     result2,
		ReplayBlocked:     result2.ReplayBlocked,
		InflightBlocked:   result2.InflightBlocked,
		AuditEventCount:   len(r.auditEvents),
		IdempotencyPrefix: result2.IdempotencyKeyPrefix,
		AttemptID:         attemptID,
		MoneyMoved:        result2.MoneyMoved,
		Description:       "Second call with same attempt ID was blocked as replay",
	}, nil
}

// runInflightBlockedScenario demonstrates in-flight blocking.
func (r *Runner) runInflightBlockedScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()

	// Create envelope
	envelope, bundle := r.createTestEnvelope(75, "GBP", now)
	traceID := r.generateID()

	// Create approvals with presentations
	approvals, hashes := r.createApprovalsWithPresentation(envelope, bundle, traceID, now)

	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	attemptID1 := attempts.DeriveAttemptID(envelope.EnvelopeID, 1)
	attemptID2 := attempts.DeriveAttemptID(envelope.EnvelopeID, 2)

	// Start first attempt in a goroutine (it will take ~500ms due to forced pause)
	var result1 *execution.V96ExecuteResult
	var result2 *execution.V96ExecuteResult
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		req := execution.V96ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			AttemptID:       attemptID1,
			Now:             now,
		}
		result1, _ = r.executor.Execute(ctx, req)
	}()

	// Wait a bit for first attempt to start, then try second attempt
	time.Sleep(100 * time.Millisecond)

	req2 := execution.V96ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  hashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         traceID,
		AttemptID:       attemptID2,
		Now:             time.Now(),
	}
	result2, _ = r.executor.Execute(ctx, req2)

	wg.Wait()

	// First should succeed, second should be blocked as in-flight
	return &DemoResult{
		ScenarioName:      "In-Flight Attempt Blocked",
		Success:           result1.Success && !result2.Success && result2.InflightBlocked,
		ExecuteResult:     result2,
		ReplayBlocked:     result2.ReplayBlocked,
		InflightBlocked:   result2.InflightBlocked,
		AuditEventCount:   len(r.auditEvents),
		IdempotencyPrefix: result2.IdempotencyKeyPrefix,
		AttemptID:         attemptID2,
		MoneyMoved:        result2.MoneyMoved,
		Description:       "Second attempt blocked because first is still in-flight",
	}, nil
}

// runTerminalReplayScenario demonstrates replay blocking after terminal state.
func (r *Runner) runTerminalReplayScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()

	// Create envelope
	envelope, bundle := r.createTestEnvelope(50, "GBP", now)
	traceID := r.generateID()

	// Create approvals with presentations
	approvals, hashes := r.createApprovalsWithPresentation(envelope, bundle, traceID, now)

	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	attemptID := attempts.DeriveAttemptID(envelope.EnvelopeID, 1)

	// First invocation - should succeed
	req := execution.V96ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  hashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         traceID,
		AttemptID:       attemptID,
		Now:             now,
	}

	result1, err := r.executor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("first invoke failed: %w", err)
	}

	if !result1.Success {
		return nil, fmt.Errorf("first invoke should succeed, got: %s", result1.BlockedReason)
	}

	// Wait for first to complete
	time.Sleep(100 * time.Millisecond)

	// Try replay after terminal state
	req.Now = time.Now()
	result2, err := r.executor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("replay invoke error: %w", err)
	}

	return &DemoResult{
		ScenarioName:      "Terminal State Replay Blocked",
		Success:           !result2.Success && result2.ReplayBlocked,
		ExecuteResult:     result2,
		ReplayBlocked:     result2.ReplayBlocked,
		InflightBlocked:   result2.InflightBlocked,
		AuditEventCount:   len(r.auditEvents),
		IdempotencyPrefix: result2.IdempotencyKeyPrefix,
		AttemptID:         attemptID,
		MoneyMoved:        result2.MoneyMoved,
		Description:       "Replay blocked after terminal simulated state",
	}, nil
}

// runMockIdempotencyScenario verifies mock respects idempotency and MoneyMoved=false.
func (r *Runner) runMockIdempotencyScenario() (*DemoResult, error) {
	now := time.Now()
	ctx := context.Background()

	// Create envelope
	envelope, bundle := r.createTestEnvelope(100, "GBP", now)
	traceID := r.generateID()

	// Create approvals with presentations
	approvals, hashes := r.createApprovalsWithPresentation(envelope, bundle, traceID, now)

	policy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		RequiredApprovers: []string{"circle_alice", "circle_bob"},
		Threshold:         2,
		ExpirySeconds:     300,
		AppliesToScopes:   []string{"finance:write"},
	}

	attemptID := attempts.DeriveAttemptID(envelope.EnvelopeID, 1)

	req := execution.V96ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       approvals,
		ApproverHashes:  hashes,
		Policy:          policy,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         traceID,
		AttemptID:       attemptID,
		Now:             now,
	}

	result, err := r.executor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute failed: %w", err)
	}

	// Verify mock behavior
	mockSuccess := result.Success &&
		result.ProviderUsed == "mock-write" &&
		!result.MoneyMoved &&
		result.IdempotencyKeyPrefix != ""

	return &DemoResult{
		ScenarioName:      "Mock Idempotency + MoneyMoved=false",
		Success:           mockSuccess,
		ExecuteResult:     result,
		ReplayBlocked:     result.ReplayBlocked,
		InflightBlocked:   result.InflightBlocked,
		AuditEventCount:   len(r.auditEvents),
		IdempotencyPrefix: result.IdempotencyKeyPrefix,
		AttemptID:         attemptID,
		MoneyMoved:        result.MoneyMoved,
		Description:       "Mock provider used idempotency key and reported MoneyMoved=false",
	}, nil
}

// createTestEnvelope creates a test envelope with bundle.
func (r *Runner) createTestEnvelope(amountCents int64, currency string, now time.Time) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
	intent := execution.ExecutionIntent{
		IntentID:       r.generateID(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_family",
		Description:    "Demo payment for v9.6 idempotency test",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    amountCents,
		Currency:       currency,
		Recipient:      "sandbox-utility",
		ViewHash:       "v8_view_" + r.generateID(),
		CreatedAt:      now,
	}

	envelope, _ := r.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        2,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  r.generateID(),
	}, now)
	envelope.SealHash = execution.ComputeSealHash(envelope)

	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Demo approval request for idempotency test.",
		300,
		r.generateID,
	)

	return envelope, bundle
}

// createApprovalsWithPresentation creates approvals with proper presentations.
func (r *Runner) createApprovalsWithPresentation(
	envelope *execution.ExecutionEnvelope,
	bundle *execution.ApprovalBundle,
	traceID string,
	now time.Time,
) ([]execution.MultiPartyApprovalArtifact, []execution.ApproverBundleHash) {
	approvers := []string{"circle_alice", "circle_bob"}
	approvals := make([]execution.MultiPartyApprovalArtifact, 0, len(approvers))
	hashes := make([]execution.ApproverBundleHash, 0, len(approvers))

	for _, approverCircle := range approvers {
		// Record presentation
		r.presentationStore.RecordPresentation(
			approverCircle,
			approverCircle+"_user",
			bundle,
			envelope,
			traceID,
			5*time.Minute,
			now,
		)

		// Create approval
		approval := execution.MultiPartyApprovalArtifact{
			ApprovalArtifact: execution.ApprovalArtifact{
				ArtifactID:       r.generateID(),
				ApproverCircleID: approverCircle,
				ApproverID:       approverCircle + "_user",
				ActionHash:       envelope.ActionHash,
				ApprovedAt:       now,
				ExpiresAt:        now.Add(5 * time.Minute),
				Signature:        r.generateID(),
			},
			BundleContentHash: bundle.ContentHash,
		}
		approvals = append(approvals, approval)

		hashes = append(hashes, execution.ApproverBundleHash{
			ApproverCircleID: approverCircle,
			ContentHash:      bundle.ContentHash,
		})
	}

	return approvals, hashes
}

// PrintResult prints a demo result in a formatted way.
func PrintResult(result *DemoResult) {
	fmt.Println()
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  SCENARIO: %s\n", result.ScenarioName)
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	fmt.Println("1. ATTEMPT DETAILS")
	fmt.Printf("   Attempt ID: %s\n", result.AttemptID)
	fmt.Printf("   Idempotency Key Prefix: %s\n", result.IdempotencyPrefix)
	fmt.Println()

	fmt.Println("2. RESULT")
	if result.Success {
		fmt.Println("   Status: SCENARIO PASSED")
	} else {
		fmt.Println("   Status: SCENARIO FAILED")
	}
	fmt.Printf("   Replay Blocked: %t\n", result.ReplayBlocked)
	fmt.Printf("   In-Flight Blocked: %t\n", result.InflightBlocked)
	fmt.Printf("   Money Moved: %t\n", result.MoneyMoved)
	fmt.Println()

	if result.ExecuteResult != nil {
		fmt.Println("3. EXECUTION RESULT")
		fmt.Printf("   Success: %t\n", result.ExecuteResult.Success)
		fmt.Printf("   Provider: %s\n", result.ExecuteResult.ProviderUsed)
		if result.ExecuteResult.BlockedReason != "" {
			fmt.Printf("   Blocked Reason: %s\n", result.ExecuteResult.BlockedReason)
		}
		fmt.Println()
	}

	fmt.Printf("4. AUDIT TRAIL: %d events\n", result.AuditEventCount)
	fmt.Println()

	fmt.Println("5. DESCRIPTION")
	fmt.Printf("   %s\n", result.Description)
	fmt.Println()

	fmt.Println("============================================================")
	if result.Success {
		fmt.Println("  SCENARIO PASSED - Idempotency/Replay Defense Working")
	} else {
		fmt.Println("  SCENARIO FAILED - Check implementation")
	}
	fmt.Println("============================================================")
}
