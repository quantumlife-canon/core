// Package demo_v911_caps demonstrates v9.11 daily caps and rate-limited execution ledger.
//
// CRITICAL: v9.11 enforces:
// 1. Per-circle daily caps (by currency)
// 2. Per-intersection daily caps (by currency)
// 3. Per-payee daily caps (by currency)
// 4. Rate limits: maximum attempts per day
// 5. All enforcement before provider Prepare/Execute
// 6. Caps are hard blocks with no partial execution
package demo_v911_caps

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/finance/execution"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/internal/finance/execution/caps"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/events"
)

// Runner executes the v9.11 caps demo scenarios.
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

	// Scenario 1: Attempt limit blocks
	result1, err := r.runAttemptLimitScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 1 failed: %w", err)
	}
	results = append(results, result1)

	// Scenario 2: Circle daily cap blocks
	result2, err := r.runCircleCapScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 2 failed: %w", err)
	}
	results = append(results, result2)

	// Scenario 3: Simulated payments don't count spend
	result3, err := r.runSimulatedNoSpendScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 3 failed: %w", err)
	}
	results = append(results, result3)

	// Scenario 4: Intersection attempt limit
	result4, err := r.runIntersectionLimitScenario()
	if err != nil {
		return nil, fmt.Errorf("scenario 4 failed: %w", err)
	}
	results = append(results, result4)

	return results, nil
}

// runAttemptLimitScenario demonstrates attempt rate limiting.
func (r *Runner) runAttemptLimitScenario() (Result, error) {
	result := Result{
		Scenario:    "S1: Attempt Limit Blocking",
		Description: "After 3 attempts, 4th is blocked by rate limit",
		Events:      make([]events.Event, 0),
	}

	policy := caps.Policy{
		Enabled:                 true,
		MaxAttemptsPerDayCircle: 3,
	}

	connector := newMockConnector(false)
	executor, _, _ := createExecutor(policy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Execute 3 attempts (all should succeed)
	for i := 1; i <= 3; i++ {
		envelope := createEnvelope(fmt.Sprintf("env-s1-%d", i), "circle-demo", "", 10, "GBP", now)

		execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         fmt.Sprintf("trace-s1-%d", i),
			AttemptID:       fmt.Sprintf("attempt-s1-%d", i),
			Now:             now,
			Clock:           r.clock,
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			return result, err
		}

		// Collect audit events from execution result
		result.Events = append(result.Events, execResult.AuditEvents...)

		if !execResult.Success {
			result.Details = append(result.Details, fmt.Sprintf("Attempt %d: UNEXPECTED BLOCK - %s", i, execResult.BlockedReason))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("Attempt %d: OK (simulated)", i))
		}
	}

	// 4th attempt should be blocked
	envelope := createEnvelope("env-s1-4", "circle-demo", "", 10, "GBP", now)

	execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s1-4",
		AttemptID:       "attempt-s1-4",
		Now:             now,
		Clock:           r.clock,
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	// Collect audit events from execution result
	result.Events = append(result.Events, execResult.AuditEvents...)

	if execResult.Success {
		result.Details = append(result.Details, "Attempt 4: UNEXPECTED SUCCESS (should be blocked)")
		result.Success = false
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Attempt 4: BLOCKED - %s", execResult.BlockedReason))
		result.Success = true
	}

	return result, nil
}

// runCircleCapScenario demonstrates circle daily cap blocking.
func (r *Runner) runCircleCapScenario() (Result, error) {
	result := Result{
		Scenario:    "S2: Circle Daily Cap Blocking",
		Description: "Cap of 100 cents; 50+60=110 exceeds cap",
		Events:      make([]events.Event, 0),
	}

	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100, // 100 cents = £1.00
		},
	}

	// Use connector that reports money moved (to test spend caps)
	connector := newMockConnector(true)
	executor, _, _ := createExecutor(policy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// First payment: 50 cents (should succeed)
	envelope1 := createEnvelope("env-s2-1", "circle-cap-demo", "", 50, "GBP", now)

	execResult1, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope1,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s2-1",
		AttemptID:       "attempt-s2-1",
		Now:             now,
		Clock:           r.clock,
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	// Collect audit events from execution result
	result.Events = append(result.Events, execResult1.AuditEvents...)

	if execResult1.Success {
		result.Details = append(result.Details, "Payment 1 (50 cents): OK")
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Payment 1: UNEXPECTED BLOCK - %s", execResult1.BlockedReason))
	}

	// Second payment: 60 cents (should be blocked, 50+60=110 > 100 cap)
	envelope2 := createEnvelope("env-s2-2", "circle-cap-demo", "", 60, "GBP", now)

	execResult2, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope2,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s2-2",
		AttemptID:       "attempt-s2-2",
		Now:             now,
		Clock:           r.clock,
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	// Collect audit events from execution result
	result.Events = append(result.Events, execResult2.AuditEvents...)

	if execResult2.Success {
		result.Details = append(result.Details, "Payment 2 (60 cents): UNEXPECTED SUCCESS (should exceed cap)")
		result.Success = false
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Payment 2 (60 cents): BLOCKED - %s", execResult2.BlockedReason))
		result.Success = true
	}

	return result, nil
}

// runSimulatedNoSpendScenario demonstrates that simulated payments don't count spend.
func (r *Runner) runSimulatedNoSpendScenario() (Result, error) {
	result := Result{
		Scenario:    "S3: Simulated Payments Don't Count Spend",
		Description: "Three 50-cent simulated payments all succeed (cap=100) because no money moves",
		Events:      make([]events.Event, 0),
	}

	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
		},
	}

	// Use connector that returns simulated (no money moves)
	connector := newMockConnector(false)
	executor, capsGate, _ := createExecutor(policy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	allSucceeded := true

	// Execute 3 simulated payments of 50 cents each
	for i := 1; i <= 3; i++ {
		envelope := createEnvelope(fmt.Sprintf("env-s3-%d", i), "circle-sim-demo", "", 50, "GBP", now)

		execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         fmt.Sprintf("trace-s3-%d", i),
			AttemptID:       fmt.Sprintf("attempt-s3-%d", i),
			Now:             now,
			Clock:           r.clock,
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			return result, err
		}

		// Collect audit events from execution result
		result.Events = append(result.Events, execResult.AuditEvents...)

		if execResult.Success {
			result.Details = append(result.Details, fmt.Sprintf("Simulated payment %d (50 cents): OK", i))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("Simulated payment %d: BLOCKED - %s", i, execResult.BlockedReason))
			allSucceeded = false
		}
	}

	// Verify spend counter is 0
	store := capsGate.GetStore()
	dayKey := caps.DayKey(r.clock)
	counters := store.GetCounters(dayKey, caps.ScopeCircle, "circle-sim-demo", "GBP")

	result.Details = append(result.Details, fmt.Sprintf("Spend counter: %d cents (expected: 0)", counters.MoneyMovedCents))
	result.Details = append(result.Details, fmt.Sprintf("Attempt counter: %d (expected: 3)", counters.Attempts))

	result.Success = allSucceeded && counters.MoneyMovedCents == 0 && counters.Attempts == 3
	return result, nil
}

// runIntersectionLimitScenario demonstrates intersection attempt limits.
func (r *Runner) runIntersectionLimitScenario() (Result, error) {
	result := Result{
		Scenario:    "S4: Intersection Attempt Limit",
		Description: "Intersection limit of 2; 3rd blocked, different intersection succeeds",
		Events:      make([]events.Event, 0),
	}

	policy := caps.Policy{
		Enabled:                       true,
		MaxAttemptsPerDayIntersection: 2,
	}

	connector := newMockConnector(false)
	executor, _, _ := createExecutor(policy, connector, r.clock)
	ctx := context.Background()
	now := r.clock.Now()

	// Execute 2 attempts via intersection-1
	for i := 1; i <= 2; i++ {
		envelope := createEnvelope(fmt.Sprintf("env-s4-%d", i), "circle-s4", "intersection-1", 10, "GBP", now)

		execResult, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         fmt.Sprintf("trace-s4-%d", i),
			AttemptID:       fmt.Sprintf("attempt-s4-%d", i),
			Now:             now,
			Clock:           r.clock,
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			return result, err
		}

		// Collect audit events from execution result
		result.Events = append(result.Events, execResult.AuditEvents...)

		if execResult.Success {
			result.Details = append(result.Details, fmt.Sprintf("Intersection-1 attempt %d: OK", i))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("Intersection-1 attempt %d: BLOCKED - %s", i, execResult.BlockedReason))
		}
	}

	// 3rd attempt via intersection-1 should be blocked
	envelope3 := createEnvelope("env-s4-3", "circle-s4", "intersection-1", 10, "GBP", now)

	execResult3, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope3,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s4-3",
		AttemptID:       "attempt-s4-3",
		Now:             now,
		Clock:           r.clock,
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	// Collect audit events from execution result
	result.Events = append(result.Events, execResult3.AuditEvents...)

	if execResult3.Success {
		result.Details = append(result.Details, "Intersection-1 attempt 3: UNEXPECTED SUCCESS")
		result.Success = false
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Intersection-1 attempt 3: BLOCKED - %s", execResult3.BlockedReason))
		result.Success = true
	}

	// Different intersection should succeed
	envelope4 := createEnvelope("env-s4-4", "circle-s4", "intersection-2", 10, "GBP", now)

	execResult4, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope4,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-s4-4",
		AttemptID:       "attempt-s4-4",
		Now:             now,
		Clock:           r.clock,
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		return result, err
	}

	// Collect audit events from execution result
	result.Events = append(result.Events, execResult4.AuditEvents...)

	if execResult4.Success {
		result.Details = append(result.Details, "Intersection-2 attempt 1: OK (own limit)")
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Intersection-2 attempt 1: UNEXPECTED BLOCK - %s", execResult4.BlockedReason))
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

	// Count v9.11 events
	v911Events := 0
	for _, e := range result.Events {
		switch e.Type {
		case events.EventV911CapsPolicyApplied,
			events.EventV911CapsChecked,
			events.EventV911CapsBlocked,
			events.EventV911CapsAttemptCounted,
			events.EventV911CapsSpendCounted,
			events.EventV911RateLimitChecked,
			events.EventV911RateLimitBlocked:
			v911Events++
		}
	}
	fmt.Printf("  v9.11 Audit Events: %d\n", v911Events)
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

	return executor, capsGate, capturedEvents
}

func createEnvelope(envelopeID, circleID, intersectionID string, amountCents int64, currency string, now time.Time) *execution.ExecutionEnvelope {
	return &execution.ExecutionEnvelope{
		EnvelopeID:          envelopeID,
		ActorCircleID:       circleID,
		IntersectionID:      intersectionID,
		ActionHash:          "hash-" + envelopeID,
		SealHash:            "seal-" + envelopeID,
		Expiry:              now.Add(1 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now.Add(-1 * time.Second),
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
