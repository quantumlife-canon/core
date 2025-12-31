// Package demo_v912_policy_snapshot demonstrates v9.12 Policy Snapshot Hash Binding.
//
// CRITICAL: v9.12 prevents policy drift between approval and execution:
// 1. PolicySnapshot captures provider allowlist, payee allowlist, and caps policy
// 2. PolicySnapshotHash is bound to the ExecutionEnvelope at creation time
// 3. At execution time, current policy hash is recomputed and compared
// 4. Any policy change between approval and execution blocks execution
// 5. Multi-party symmetry verification includes PolicySnapshotHash
//
// This ensures immutability of execution rules between approval and execution.
package demo_v912_policy_snapshot

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

// Runner executes the v9.12 policy snapshot demo scenarios.
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

	// Scenario 1: Valid policy snapshot (no drift)
	result1, err := r.runValidPolicyScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 1 failed: %w", err)
	}
	results = append(results, result1)

	// Scenario 2: Policy drift blocks execution
	result2, err := r.runPolicyDriftScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 2 failed: %w", err)
	}
	results = append(results, result2)

	// Scenario 3: Multi-party bundle/envelope hash match
	result3, err := r.runMultiPartyHashMatchScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 3 failed: %w", err)
	}
	results = append(results, result3)

	// Scenario 4: Caps policy change blocks execution
	result4, err := r.runCapsPolicyChangeScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 4 failed: %w", err)
	}
	results = append(results, result4)

	// Scenario 5 (v9.12.1): Missing hash blocks execution
	result5, err := r.runMissingHashScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 5 failed: %w", err)
	}
	results = append(results, result5)

	return results, nil
}

// runValidPolicyScenario demonstrates successful execution with matching policy.
func (r *Runner) runValidPolicyScenario() (Result, error) {
	result := Result{
		Scenario:    "S1: Valid Policy Snapshot",
		Description: "Envelope bound to policy hash matches current policy",
		Events:      make([]events.Event, 0),
	}

	capsPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
	}

	connector := newMockConnector(false)
	executor, _, _ := createExecutor(capsPolicy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Compute policy snapshot from executor
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()
	result.Details = append(result.Details, fmt.Sprintf("Computed policy hash: %s...", string(policyHash)[:16]))

	// Create envelope with the policy hash
	envelope := createEnvelopeWithPolicy("env-s1-valid", "circle-s1", "", 50, "GBP", now, string(policyHash))

	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s1-valid",
		AttemptID:       "attempt-s1-valid",
		Now:             now,
		Clock:           r.clock,
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Execution: SUCCESS (policy verified)")
		result.Success = true
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Execution: UNEXPECTED BLOCK - %s", execResult.BlockedReason))
		result.Success = false
	}

	// Check for policy verification in validation details
	for _, v := range execResult.ValidationDetails {
		if v.Check == "policy_snapshot_verified" && v.Passed {
			result.Details = append(result.Details, "Policy snapshot verification: PASSED")
		}
	}

	return result, nil
}

// runPolicyDriftScenario demonstrates blocking when policy changes.
func (r *Runner) runPolicyDriftScenario() (Result, error) {
	result := Result{
		Scenario:    "S2: Policy Drift Blocks Execution",
		Description: "Envelope bound to old policy hash; policy changed; execution blocked",
		Events:      make([]events.Event, 0),
	}

	capsPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
	}

	connector := newMockConnector(false)
	executor, capsGate, _ := createExecutor(capsPolicy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Compute initial policy snapshot
	_, initialHash := executor.ComputePolicySnapshotForEnvelope()
	result.Details = append(result.Details, fmt.Sprintf("Initial policy hash: %s...", string(initialHash)[:16]))

	// Create envelope with the initial policy hash
	envelope := createEnvelopeWithPolicy("env-s2-drift", "circle-s2", "", 50, "GBP", now, string(initialHash))

	// CHANGE THE POLICY (simulating drift between approval and execution)
	newPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 5000, // Changed from 10000 to 5000
		},
	}
	capsGate.UpdatePolicy(newPolicy)
	result.Details = append(result.Details, "Policy changed: cap 10000 -> 5000 cents")

	// Compute new hash
	_, newHash := executor.ComputePolicySnapshotForEnvelope()
	result.Details = append(result.Details, fmt.Sprintf("New policy hash: %s...", string(newHash)[:16]))

	// Attempt execution (should be blocked due to drift)
	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s2-drift",
		AttemptID:       "attempt-s2-drift",
		Now:             now,
		Clock:           r.clock,
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
		result.Success = true
	}

	return result, nil
}

// runMultiPartyHashMatchScenario verifies bundle/envelope hash consistency.
func (r *Runner) runMultiPartyHashMatchScenario() (Result, error) {
	result := Result{
		Scenario:    "S3: Multi-Party Policy Hash Match",
		Description: "Bundle and envelope PolicySnapshotHash must match for multi-party",
		Events:      make([]events.Event, 0),
	}

	capsPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
	}

	connector := newMockConnector(false)
	executor, _, _ := createExecutor(capsPolicy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Compute policy snapshot
	_, policyHash := executor.ComputePolicySnapshotForEnvelope()

	// Create envelope with policy hash
	envelope := createEnvelopeWithPolicy("env-s3-mp", "circle-s3", "intersection-s3", 50, "GBP", now, string(policyHash))

	// Create bundle with SAME policy hash
	bundle := &execution.ApprovalBundle{
		EnvelopeID:         envelope.EnvelopeID,
		ActionHash:         envelope.ActionHash,
		IntersectionID:     envelope.IntersectionID,
		PayeeID:            "sandbox-utility",
		AmountCents:        50,
		Currency:           "GBP",
		Expiry:             now.Add(1 * time.Hour),
		PolicySnapshotHash: string(policyHash),
		CreatedAt:          now,
	}
	bundle.Seal()
	result.Details = append(result.Details, fmt.Sprintf("Bundle policy hash: %s...", bundle.PolicySnapshotHash[:16]))
	result.Details = append(result.Details, fmt.Sprintf("Envelope policy hash: %s...", envelope.PolicySnapshotHash[:16]))

	// Create approvals for multi-party
	approval := execution.MultiPartyApprovalArtifact{
		ApprovalArtifact: execution.ApprovalArtifact{
			ArtifactID:       "approval-s3",
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
		TraceID:         "trace-s3-mp",
		AttemptID:       "attempt-s3-mp",
		Now:             now,
		Clock:           r.clock,
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

// runCapsPolicyChangeScenario demonstrates blocking when caps policy changes.
func (r *Runner) runCapsPolicyChangeScenario() (Result, error) {
	result := Result{
		Scenario:    "S4: Caps Policy Change Blocks",
		Description: "Rate limit added after approval; execution blocked",
		Events:      make([]events.Event, 0),
	}

	// Initial policy: no rate limit
	initialCapsPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
		MaxAttemptsPerDayCircle: 0, // No limit
	}

	connector := newMockConnector(false)
	executor, capsGate, _ := createExecutor(initialCapsPolicy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Compute initial policy snapshot
	_, initialHash := executor.ComputePolicySnapshotForEnvelope()
	result.Details = append(result.Details, "Initial: no rate limit")

	// Create envelope with the initial policy hash
	envelope := createEnvelopeWithPolicy("env-s4-caps", "circle-s4", "", 50, "GBP", now, string(initialHash))

	// CHANGE THE POLICY - add rate limit
	newPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
		MaxAttemptsPerDayCircle: 5, // Added rate limit
	}
	capsGate.UpdatePolicy(newPolicy)
	result.Details = append(result.Details, "Policy changed: added rate limit of 5 attempts/day")

	// Attempt execution (should be blocked due to policy drift)
	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s4-caps",
		AttemptID:       "attempt-s4-caps",
		Now:             now,
		Clock:           r.clock,
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
		result.Success = true
	}

	return result, nil
}

// runMissingHashScenario demonstrates v9.12.1 hard-block when hash is missing.
func (r *Runner) runMissingHashScenario() (Result, error) {
	result := Result{
		Scenario:    "S5: Missing Hash Blocks (v9.12.1)",
		Description: "Envelope with empty PolicySnapshotHash is blocked",
		Events:      make([]events.Event, 0),
	}

	capsPolicy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 10000,
		},
	}

	connector := newMockConnector(false)
	executor, _, _ := createExecutor(capsPolicy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Create envelope WITHOUT policy hash (empty string)
	envelope := createEnvelopeWithPolicy("env-s5-missing", "circle-s5", "", 50, "GBP", now, "") // Empty hash!
	result.Details = append(result.Details, "Envelope PolicySnapshotHash: (empty)")

	// Attempt execution (should be blocked due to missing hash)
	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s5-missing",
		AttemptID:       "attempt-s5-missing",
		Now:             now,
		Clock:           r.clock,
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

		// Check for specific v9.12.1 validation check
		foundCheck := false
		for _, v := range execResult.ValidationDetails {
			if v.Check == "policy_snapshot_hash_present" && !v.Passed {
				result.Details = append(result.Details, "Validation: policy_snapshot_hash_present = FAILED (correct)")
				foundCheck = true
			}
		}

		// Check for v9.12.1 events
		foundMissingEvent := false
		foundBlockedEvent := false
		for _, e := range execResult.AuditEvents {
			switch e.Type {
			case events.EventV912PolicySnapshotMissing:
				foundMissingEvent = true
			case events.EventV912ExecutionBlockedMissingHash:
				foundBlockedEvent = true
			}
		}

		if foundMissingEvent {
			result.Details = append(result.Details, "Event: EventV912PolicySnapshotMissing emitted")
		}
		if foundBlockedEvent {
			result.Details = append(result.Details, "Event: EventV912ExecutionBlockedMissingHash emitted")
		}

		result.Success = foundCheck && foundMissingEvent && foundBlockedEvent
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

	// Count v9.12 and v9.12.1 events
	v912Events := 0
	for _, e := range result.Events {
		switch e.Type {
		case events.EventV912PolicySnapshotComputed,
			events.EventV912PolicySnapshotBound,
			events.EventV912PolicySnapshotVerified,
			events.EventV912PolicySnapshotMismatch,
			events.EventV912ExecutionBlockedPolicyDrift,
			events.EventV912PolicySnapshotMissing,
			events.EventV912ExecutionBlockedMissingHash:
			v912Events++
		}
	}
	fmt.Printf("  v9.12/v9.12.1 Audit Events: %d\n", v912Events)
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

func createExecutor(policy caps.Policy, connector *mockConnector, clk clock.Clock) (*execution.V96Executor, *caps.DefaultGate, []events.Event) {
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

	capsGate := caps.NewDefaultGate(policy, emitter)
	executor.SetCapsGate(capsGate)

	// Set registries for policy snapshot computation
	executor.SetProviderRegistry(registry.NewDefaultRegistry())
	executor.SetPayeeRegistry(payees.NewDefaultRegistry())

	return executor, capsGate, capturedEvents
}

func createEnvelopeWithPolicy(envelopeID, circleID, intersectionID string, amountCents int64, currency string, now time.Time, policyHash string) *execution.ExecutionEnvelope {
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
