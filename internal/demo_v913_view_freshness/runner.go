// Package demo_v913_view_freshness demonstrates v9.13 View Freshness Binding.
//
// CRITICAL: v9.13 ensures read-before-write and view freshness:
// 1. ViewSnapshot captures view state at approval time
// 2. ViewSnapshotHash is bound to the ExecutionEnvelope
// 3. At execution time, current view hash is recomputed and compared
// 4. Any view drift (hash mismatch) or staleness blocks execution
// 5. Multi-party symmetry verification includes ViewSnapshotHash
//
// This ensures execution is based on fresh, consistent view data.
package demo_v913_view_freshness

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/payees"
	"quantumlife/internal/connectors/finance/write/registry"
	"quantumlife/internal/finance/execution"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/internal/finance/execution/caps"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/events"
)

// Runner executes the v9.13 view freshness demo scenarios.
type Runner struct {
	clock clock.Clock
}

// NewRunner creates a new demo runner.
func NewRunner() *Runner {
	return &Runner{
		clock: clock.NewReal(),
	}
}

// Result contains the demo execution results.
type Result struct {
	Scenario    string
	Description string
	Success     bool
	Details     []string
	Events      []events.Event
}

// Run executes all demo scenarios.
func (r *Runner) Run() ([]Result, error) {
	var results []Result

	// Scenario 1: Valid view snapshot (fresh view, hash matches)
	result1, err := r.runValidViewScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 1 failed: %w", err)
	}
	results = append(results, result1)

	// Scenario 2: View stale blocks execution
	result2, err := r.runStaleViewScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 2 failed: %w", err)
	}
	results = append(results, result2)

	// Scenario 3: View hash mismatch blocks execution
	result3, err := r.runHashMismatchScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 3 failed: %w", err)
	}
	results = append(results, result3)

	// Scenario 4: Missing view hash blocks execution (v9.13 hardening)
	result4, err := r.runMissingHashScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 4 failed: %w", err)
	}
	results = append(results, result4)

	// Scenario 5: Multi-party view hash symmetry
	result5, err := r.runMultiPartySymmetryScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 5 failed: %w", err)
	}
	results = append(results, result5)

	return results, nil
}

// runValidViewScenario demonstrates successful execution with fresh view.
func (r *Runner) runValidViewScenario() (Result, error) {
	result := Result{
		Scenario:    "S1: Valid View Snapshot",
		Description: "Envelope bound to view hash; view fresh; execution succeeds",
		Events:      make([]events.Event, 0),
	}

	now := r.clock.Now()
	counter := 0
	idGen := func() string {
		counter++
		return fmt.Sprintf("s1-id-%d", counter)
	}

	// Create view provider that returns fresh views
	viewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view-s1",
		Clock:           clock.NewFixed(now),
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
		Accounts:        []string{"acct-1"},
		SharedViewHash:  "shared-hash-s1",
	})

	// Use fixed SnapshotID for deterministic hashing
	fixedSnapshotID := "snapshot-s1-fixed"
	viewProvider.SetSnapshotIDOverride(fixedSnapshotID)

	// Get initial view and compute hash
	initialView, _ := viewProvider.GetViewSnapshot(context.Background(), execution.ViewSnapshotRequest{
		CircleID:       "circle-s1",
		IntersectionID: "",
		PayeeID:        "sandbox-utility",
		Currency:       "GBP",
		AmountCents:    50,
		ProviderID:     "mock-write",
		Clock:          clock.NewFixed(now),
	})
	viewHash := execution.ComputeViewSnapshotHash(initialView)
	result.Details = append(result.Details, fmt.Sprintf("Computed view hash: %s...", string(viewHash)[:16]))

	connector := newMockConnector(false)
	executor := createExecutorWithViewProvider(viewProvider, connector, r.clock)
	ctx := context.Background()

	// Compute policy snapshot too (v9.12 requirement)
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()

	// Create envelope with view hash
	envelope := createEnvelopeWithViewHash("env-s1-valid", "circle-s1", "", 50, "GBP", now, string(policyHash), string(viewHash))

	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s1-valid",
		AttemptID:       "attempt-s1-valid",
		Now:             now,
		Clock:           clock.NewFixed(now),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Execution: SUCCESS (view verified)")
		result.Success = true
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Execution: UNEXPECTED BLOCK - %s", execResult.BlockedReason))
		result.Success = false
	}

	// Check for view verification in validation details
	for _, v := range execResult.ValidationDetails {
		if v.Check == "view_snapshot_verified" && v.Passed {
			result.Details = append(result.Details, fmt.Sprintf("View snapshot verification: %s", v.Details))
		}
	}

	return result, nil
}

// runStaleViewScenario demonstrates blocking when view is stale.
func (r *Runner) runStaleViewScenario() (Result, error) {
	result := Result{
		Scenario:    "S2: Stale View Blocks Execution",
		Description: "View captured 10 minutes ago; MaxStaleness=5 minutes; execution blocked",
		Events:      make([]events.Event, 0),
	}

	now := r.clock.Now()
	counter := 0
	idGen := func() string {
		counter++
		return fmt.Sprintf("s2-id-%d", counter)
	}

	// Create view provider with stale view (captured 10 minutes ago)
	staleTime := now.Add(-10 * time.Minute)
	viewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view-s2",
		Clock:           clock.NewFixed(now),
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
		Accounts:        []string{"acct-1"},
		SharedViewHash:  "shared-hash-s2",
	})

	// Use fixed SnapshotID for deterministic hashing
	fixedSnapshotID := "snapshot-s2-fixed"
	viewProvider.SetSnapshotIDOverride(fixedSnapshotID)
	viewProvider.SetCapturedAtOverride(staleTime)

	// Get view and compute hash (with stale time)
	staleView, _ := viewProvider.GetViewSnapshot(context.Background(), execution.ViewSnapshotRequest{
		CircleID:       "circle-s2",
		IntersectionID: "",
		PayeeID:        "sandbox-utility",
		Currency:       "GBP",
		AmountCents:    50,
		ProviderID:     "mock-write",
		Clock:          clock.NewFixed(staleTime),
	})
	viewHash := execution.ComputeViewSnapshotHash(staleView)
	result.Details = append(result.Details, fmt.Sprintf("View captured at: %s", staleTime.Format(time.RFC3339)))
	result.Details = append(result.Details, fmt.Sprintf("Execution time: %s", now.Format(time.RFC3339)))
	result.Details = append(result.Details, "Staleness: 10 minutes (exceeds 5 minute max)")

	connector := newMockConnector(false)
	executor := createExecutorWithViewProvider(viewProvider, connector, r.clock)
	ctx := context.Background()

	// Compute policy snapshot
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()

	// Create envelope with view hash
	envelope := createEnvelopeWithViewHash("env-s2-stale", "circle-s2", "", 50, "GBP", now, string(policyHash), string(viewHash))

	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s2-stale",
		AttemptID:       "attempt-s2-stale",
		Now:             now,
		Clock:           clock.NewFixed(now),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Execution: UNEXPECTED SUCCESS (should be blocked)")
		result.Success = false
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Execution: BLOCKED - %s", execResult.BlockedReason))

		// Check for staleness event
		foundStaleEvent := false
		for _, e := range execResult.AuditEvents {
			if e.Type == events.EventV913ExecutionBlockedViewStale {
				foundStaleEvent = true
				result.Details = append(result.Details, "Event: EventV913ExecutionBlockedViewStale emitted")
			}
		}

		result.Success = foundStaleEvent
	}

	return result, nil
}

// runHashMismatchScenario demonstrates blocking when view hash mismatches.
func (r *Runner) runHashMismatchScenario() (Result, error) {
	result := Result{
		Scenario:    "S3: View Hash Mismatch Blocks",
		Description: "View changed between approval and execution; hash mismatch; execution blocked",
		Events:      make([]events.Event, 0),
	}

	now := r.clock.Now()
	counter := 0
	idGen := func() string {
		counter++
		return fmt.Sprintf("s3-id-%d", counter)
	}

	// Create initial view hash
	initialViewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view-s3",
		Clock:           clock.NewFixed(now),
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
		Accounts:        []string{"acct-1"},
		SharedViewHash:  "shared-hash-s3-v1",
	})

	initialView, _ := initialViewProvider.GetViewSnapshot(context.Background(), execution.ViewSnapshotRequest{
		CircleID:       "circle-s3",
		IntersectionID: "",
		PayeeID:        "sandbox-utility",
		Currency:       "GBP",
		AmountCents:    50,
		ProviderID:     "mock-write",
		Clock:          clock.NewFixed(now),
	})
	initialHash := execution.ComputeViewSnapshotHash(initialView)
	result.Details = append(result.Details, fmt.Sprintf("Initial view hash: %s...", string(initialHash)[:16]))

	// Create changed view provider (different shared hash simulates view drift)
	changedViewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view-s3",
		Clock:           clock.NewFixed(now),
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
		Accounts:        []string{"acct-1"},
		SharedViewHash:  "shared-hash-s3-v2", // Changed!
	})

	connector := newMockConnector(false)
	executor := createExecutorWithViewProvider(changedViewProvider, connector, r.clock)
	ctx := context.Background()

	// Compute policy snapshot
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()

	// Create envelope with INITIAL view hash (before change)
	envelope := createEnvelopeWithViewHash("env-s3-mismatch", "circle-s3", "", 50, "GBP", now, string(policyHash), string(initialHash))
	result.Details = append(result.Details, "View changed: SharedViewHash v1 -> v2")

	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s3-mismatch",
		AttemptID:       "attempt-s3-mismatch",
		Now:             now,
		Clock:           clock.NewFixed(now),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Execution: UNEXPECTED SUCCESS (should be blocked)")
		result.Success = false
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Execution: BLOCKED - %s", execResult.BlockedReason))

		// Check for hash mismatch event
		foundMismatchEvent := false
		for _, e := range execResult.AuditEvents {
			if e.Type == events.EventV913ViewHashMismatch || e.Type == events.EventV913ExecutionBlockedViewHashMismatch {
				foundMismatchEvent = true
				result.Details = append(result.Details, fmt.Sprintf("Event: %s emitted", e.Type))
			}
		}

		result.Success = foundMismatchEvent
	}

	return result, nil
}

// runMissingHashScenario demonstrates v9.13 hard-block when hash is missing.
func (r *Runner) runMissingHashScenario() (Result, error) {
	result := Result{
		Scenario:    "S4: Missing View Hash Blocks (v9.13)",
		Description: "Envelope with empty ViewSnapshotHash is blocked",
		Events:      make([]events.Event, 0),
	}

	now := r.clock.Now()
	counter := 0
	idGen := func() string {
		counter++
		return fmt.Sprintf("s4-id-%d", counter)
	}

	viewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view-s4",
		Clock:           clock.NewFixed(now),
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
	})

	connector := newMockConnector(false)
	executor := createExecutorWithViewProvider(viewProvider, connector, r.clock)
	ctx := context.Background()

	// Compute policy snapshot
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()

	// Create envelope WITHOUT view hash (empty string)
	envelope := createEnvelopeWithViewHash("env-s4-missing", "circle-s4", "", 50, "GBP", now, string(policyHash), "") // Empty hash!
	result.Details = append(result.Details, "Envelope ViewSnapshotHash: (empty)")

	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s4-missing",
		AttemptID:       "attempt-s4-missing",
		Now:             now,
		Clock:           clock.NewFixed(now),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Execution: UNEXPECTED SUCCESS (should be blocked)")
		result.Success = false
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Execution: BLOCKED - %s", execResult.BlockedReason))

		// Check for specific v9.13 validation check
		foundCheck := false
		for _, v := range execResult.ValidationDetails {
			if v.Check == "view_snapshot_hash_present" && !v.Passed {
				result.Details = append(result.Details, "Validation: view_snapshot_hash_present = FAILED (correct)")
				foundCheck = true
			}
		}

		// Check for v9.13 missing hash event
		foundMissingEvent := false
		for _, e := range execResult.AuditEvents {
			if e.Type == events.EventV913ExecutionBlockedViewHashMissing {
				foundMissingEvent = true
				result.Details = append(result.Details, "Event: EventV913ExecutionBlockedViewHashMissing emitted")
			}
		}

		result.Success = foundCheck && foundMissingEvent
	}

	return result, nil
}

// runMultiPartySymmetryScenario verifies bundle/envelope view hash consistency.
func (r *Runner) runMultiPartySymmetryScenario() (Result, error) {
	result := Result{
		Scenario:    "S5: Multi-Party View Hash Symmetry",
		Description: "Bundle and envelope ViewSnapshotHash must match for multi-party",
		Events:      make([]events.Event, 0),
	}

	now := r.clock.Now()
	counter := 0
	idGen := func() string {
		counter++
		return fmt.Sprintf("s5-id-%d", counter)
	}

	viewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view-s5",
		Clock:           clock.NewFixed(now),
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
		Accounts:        []string{"acct-1"},
		SharedViewHash:  "shared-hash-s5",
	})

	// Use fixed SnapshotID for deterministic hashing
	fixedSnapshotID := "snapshot-s5-fixed"
	viewProvider.SetSnapshotIDOverride(fixedSnapshotID)

	initialView, _ := viewProvider.GetViewSnapshot(context.Background(), execution.ViewSnapshotRequest{
		CircleID:       "circle-s5",
		IntersectionID: "intersection-s5",
		PayeeID:        "sandbox-utility",
		Currency:       "GBP",
		AmountCents:    50,
		ProviderID:     "mock-write",
		Clock:          clock.NewFixed(now),
	})
	viewHash := execution.ComputeViewSnapshotHash(initialView)

	connector := newMockConnector(false)
	executor := createExecutorWithViewProvider(viewProvider, connector, r.clock)
	ctx := context.Background()

	// Compute policy snapshot
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()

	// Create envelope with view hash
	envelope := createEnvelopeWithViewHash("env-s5-mp", "circle-s5", "intersection-s5", 50, "GBP", now, string(policyHash), string(viewHash))

	// Create bundle with SAME view hash (symmetry)
	bundle := &execution.ApprovalBundle{
		EnvelopeID:         envelope.EnvelopeID,
		ActionHash:         envelope.ActionHash,
		IntersectionID:     envelope.IntersectionID,
		PayeeID:            "sandbox-utility",
		AmountCents:        50,
		Currency:           "GBP",
		Expiry:             now.Add(1 * time.Hour),
		PolicySnapshotHash: string(policyHash),
		ViewSnapshotHash:   string(viewHash), // Same as envelope
		CreatedAt:          now,
	}
	bundle.Seal()
	result.Details = append(result.Details, fmt.Sprintf("Bundle view hash: %s...", bundle.ViewSnapshotHash[:16]))
	result.Details = append(result.Details, fmt.Sprintf("Envelope view hash: %s...", envelope.ViewSnapshotHash[:16]))

	// Create approvals for multi-party
	approval := execution.MultiPartyApprovalArtifact{
		ApprovalArtifact: execution.ApprovalArtifact{
			ArtifactID:       "approval-s5",
			ApproverCircleID: "approver-1",
			ApproverID:       "member-1",
			ActionHash:       envelope.ActionHash,
			ApprovedAt:       now,
			ExpiresAt:        now.Add(1 * time.Hour),
		},
		BundleContentHash: bundle.ContentHash,
	}

	multiPartyPolicy := &execution.MultiPartyPolicy{
		Mode:              "multi",
		Threshold:         1,
		RequiredApprovers: []string{"approver-1"},
		AppliesToScopes:   []string{"finance:write"},
	}

	approverHashes := []execution.ApproverBundleHash{
		{
			ApproverCircleID: "approver-1",
			ContentHash:      bundle.ContentHash,
			PresentedAt:      now,
		},
	}

	// Execute with multi-party
	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		Bundle:          bundle,
		Approvals:       []execution.MultiPartyApprovalArtifact{approval},
		ApproverHashes:  approverHashes,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s5-mp",
		AttemptID:       "attempt-s5-mp",
		Now:             now,
		Clock:           clock.NewFixed(now),
		Policy:          multiPartyPolicy,
	})

	if err != nil {
		return result, err
	}

	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Multi-party execution: SUCCESS")
		result.Success = true
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Multi-party execution: BLOCK - %s", execResult.BlockedReason))
		result.Success = false
	}

	return result, nil
}

// PrintResult prints a demo result.
func PrintResult(result Result) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Scenario: %s\n", result.Scenario)
	fmt.Printf("Description: %s\n", result.Description)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, detail := range result.Details {
		fmt.Printf("  %s\n", detail)
	}

	fmt.Println()
	if result.Success {
		fmt.Println("  Result: PASS")
	} else {
		fmt.Println("  Result: FAIL")
	}

	// Count v9.13 events
	v913Events := 0
	for _, e := range result.Events {
		switch e.Type {
		case events.EventV913ViewSnapshotRequested,
			events.EventV913ViewSnapshotReceived,
			events.EventV913ViewFreshnessChecked,
			events.EventV913ViewHashVerified,
			events.EventV913ViewHashMismatch,
			events.EventV913ExecutionBlockedViewStale,
			events.EventV913ExecutionBlockedViewHashMismatch,
			events.EventV913ExecutionBlockedViewHashMissing,
			events.EventV913ViewSnapshotBound:
			v913Events++
		}
	}
	fmt.Printf("  v9.13 Audit Events: %d\n", v913Events)
}

// Helper types and functions

type mockConnector struct {
	moneyMoved bool
}

func newMockConnector(moneyMoved bool) *mockConnector {
	return &mockConnector{moneyMoved: moneyMoved}
}

func (c *mockConnector) Provider() string               { return "mock-write" }
func (c *mockConnector) ProviderID() string             { return "mock-write" }
func (c *mockConnector) ProviderInfo() (string, string) { return "mock-write", "mock" }

func (c *mockConnector) Prepare(ctx context.Context, req write.PrepareRequest) (*write.PrepareResult, error) {
	return &write.PrepareResult{Valid: true}, nil
}

func (c *mockConnector) Execute(ctx context.Context, req write.ExecuteRequest) (*write.PaymentReceipt, error) {
	status := write.PaymentSimulated
	simulated := true
	if c.moneyMoved {
		status = write.PaymentSucceeded
		simulated = false
	}

	return &write.PaymentReceipt{
		ReceiptID:   "demo-receipt",
		EnvelopeID:  req.Envelope.EnvelopeID,
		Status:      status,
		Simulated:   simulated,
		AmountCents: req.Envelope.ActionSpec.AmountCents,
		Currency:    req.Envelope.ActionSpec.Currency,
		PayeeID:     req.PayeeID,
	}, nil
}

func (c *mockConnector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	return true, nil
}

func createExecutorWithViewProvider(viewProvider *execution.MockViewProvider, connector *mockConnector, clk clock.Clock) *execution.V96Executor {
	counter := 0
	idGen := func() string {
		counter++
		return fmt.Sprintf("demo-id-%d", counter)
	}

	var capturedEvents []events.Event
	emitter := func(e events.Event) {
		capturedEvents = append(capturedEvents, e)
	}

	signingKey := []byte("demo-signing-key-32-bytes-long!!")
	presentationStore := execution.NewPresentationStore(idGen, emitter)
	revocationChecker := execution.NewRevocationChecker(idGen)
	presentationGate := execution.NewPresentationGate(presentationStore, idGen, emitter)
	multiPartyGate := execution.NewMultiPartyGate(idGen, emitter)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)
	attemptLedger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, emitter)

	config := execution.DefaultV96ExecutorConfig()
	config.ForcedPauseDuration = 10 * time.Millisecond
	config.RevocationPollInterval = 5 * time.Millisecond
	config.TrueLayerConfigured = connector.moneyMoved

	executor := execution.NewV96Executor(
		nil,
		connector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		revocationChecker,
		attemptLedger,
		config,
		idGen,
		emitter,
	)

	// Set v9.13 view provider
	executor.SetViewProvider(viewProvider)

	// Set caps gate for policy snapshot
	capsPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
	}
	capsGate := caps.NewDefaultGate(capsPolicy, emitter)
	executor.SetCapsGate(capsGate)

	// Set registries for policy snapshot computation
	executor.SetProviderRegistry(registry.NewDefaultRegistry())
	executor.SetPayeeRegistry(payees.NewDefaultRegistry())

	return executor
}

func createEnvelopeWithViewHash(envelopeID, circleID, intersectionID string, amountCents int64, currency string, now time.Time, policyHash, viewHash string) *execution.ExecutionEnvelope {
	return &execution.ExecutionEnvelope{
		EnvelopeID:          envelopeID,
		ActorCircleID:       circleID,
		IntersectionID:      intersectionID,
		ActionHash:          "hash-" + envelopeID,
		SealHash:            "seal-" + envelopeID,
		Expiry:              now.Add(1 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now.Add(-1 * time.Second),
		PolicySnapshotHash:  policyHash,
		ViewSnapshotHash:    viewHash,
		ActionSpec: execution.ActionSpec{
			Type:        "payment",
			AmountCents: amountCents,
			Currency:    currency,
			PayeeID:     "sandbox-utility",
			Description: "Demo payment",
		},
	}
}

// Verify interface compliance
var _ write.WriteConnector = (*mockConnector)(nil)
