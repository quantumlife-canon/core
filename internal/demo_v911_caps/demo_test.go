// Package demo_v911_caps tests v9.11 daily caps and rate-limited execution ledger.
//
// CRITICAL: v9.11 enforces:
// 1. Per-circle daily caps (by currency)
// 2. Per-intersection daily caps (by currency)
// 3. Per-payee daily caps (by currency)
// 4. Rate limits: maximum attempts per day
// 5. All enforcement before provider Prepare/Execute
// 6. Caps are hard blocks with no partial execution
//
// NOTE: Spend caps only count when money actually moves.
// Simulated payments (MoneyMoved=false) increment attempt counters
// but NOT spend counters. This is by design per the spec.
package demo_v911_caps

import (
	"context"
	"testing"
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

// MockWriteConnector for testing
type MockWriteConnector struct {
	providerID  string
	environment string
	// MoneyMoved controls whether the mock reports real money movement
	MoneyMoved bool
}

func NewMockWriteConnector() *MockWriteConnector {
	return &MockWriteConnector{
		providerID:  "mock-write",
		environment: "mock",
		MoneyMoved:  false, // Default: simulated
	}
}

func NewMockWriteConnectorWithMoneyMoved() *MockWriteConnector {
	return &MockWriteConnector{
		providerID:  "mock-write",
		environment: "mock",
		MoneyMoved:  true, // Reports real money movement
	}
}

func (c *MockWriteConnector) Provider() string               { return c.providerID }
func (c *MockWriteConnector) ProviderID() string             { return c.providerID }
func (c *MockWriteConnector) ProviderInfo() (string, string) { return c.providerID, c.environment }

func (c *MockWriteConnector) Prepare(ctx context.Context, req write.PrepareRequest) (*write.PrepareResult, error) {
	return &write.PrepareResult{Valid: true}, nil
}

func (c *MockWriteConnector) Execute(ctx context.Context, req write.ExecuteRequest) (*write.PaymentReceipt, error) {
	status := write.PaymentSimulated
	simulated := true
	if c.MoneyMoved {
		status = write.PaymentSucceeded
		simulated = false
	}

	return &write.PaymentReceipt{
		ReceiptID:   "test-receipt",
		EnvelopeID:  req.Envelope.EnvelopeID,
		Status:      status,
		Simulated:   simulated,
		AmountCents: req.Envelope.ActionSpec.AmountCents,
		Currency:    req.Envelope.ActionSpec.Currency,
		PayeeID:     req.PayeeID,
	}, nil
}

func (c *MockWriteConnector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	return true, nil
}

// Helper to create a test clock
func testClock() clock.Clock {
	return clock.NewFixed(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
}

// Helper to create an executor with caps gate
func createTestExecutor(policy caps.Policy, connector *MockWriteConnector) (*execution.V96Executor, *caps.DefaultGate, []events.Event) {
	counter := 0
	idGen := func() string {
		counter++
		return "test-id-" + string(rune('a'+counter))
	}

	var capturedEvents []events.Event
	emitter := func(e events.Event) {
		capturedEvents = append(capturedEvents, e)
	}

	signingKey := []byte("test-signing-key-32-bytes-long!!")
	presentationStore := execution.NewPresentationStore(idGen, emitter)
	revocationChecker := execution.NewRevocationChecker(idGen)
	presentationGate := execution.NewPresentationGate(presentationStore, idGen, emitter)
	multiPartyGate := execution.NewMultiPartyGate(idGen, emitter)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)
	attemptLedger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, emitter)

	config := execution.DefaultV96ExecutorConfig()
	config.ForcedPauseDuration = 10 * time.Millisecond
	config.RevocationPollInterval = 5 * time.Millisecond
	config.TrueLayerConfigured = connector.MoneyMoved // If money moves, simulate TrueLayer

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

	// Create and set caps gate
	capsGate := caps.NewDefaultGate(policy, emitter)
	executor.SetCapsGate(capsGate)

	// v9.12: Set registries for policy snapshot computation
	executor.SetProviderRegistry(registry.NewDefaultRegistry())
	executor.SetPayeeRegistry(payees.NewDefaultRegistry())

	return executor, capsGate, capturedEvents
}

// Helper to create a test envelope with policy snapshot hash
func createTestEnvelope(envelopeID, circleID, intersectionID string, amountCents int64, currency string) *execution.ExecutionEnvelope {
	now := testClock().Now()
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
			Description: "Test payment",
		},
	}
}

// createTestEnvelopeWithHash creates a test envelope bound to the executor's current policy.
// v9.12.1: PolicySnapshotHash is required - empty hash blocks execution.
func createTestEnvelopeWithHash(executor *execution.V96Executor, envelopeID, circleID, intersectionID string, amountCents int64, currency string) *execution.ExecutionEnvelope {
	envelope := createTestEnvelope(envelopeID, circleID, intersectionID, amountCents, currency)
	_, hash := executor.ComputePolicySnapshotForEnvelope()
	envelope.PolicySnapshotHash = string(hash)
	return envelope
}

// Scenario 1: Attempt limit blocks after N attempts (simulated mode)
// This is the primary test for caps blocking because attempt limits
// count regardless of whether money moves.
func TestAttemptLimitBlocks(t *testing.T) {
	policy := caps.Policy{
		Enabled:                 true,
		MaxAttemptsPerDayCircle: 3,
	}

	connector := NewMockWriteConnector()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	// Execute 3 attempts (all should succeed - they're simulated)
	for i := 1; i <= 3; i++ {
		envelope := createTestEnvelopeWithHash(executor, "env-"+string(rune('0'+i)), "circle-1", "", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-" + string(rune('0'+i)),
			AttemptID:       "attempt-" + string(rune('0'+i)),
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution %d failed: %v", i, err)
		}

		if !result.Success {
			t.Errorf("attempt %d should succeed: %s", i, result.BlockedReason)
		}
	}

	// 4th attempt should be blocked
	t.Run("4th attempt blocked by rate limit", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-4", "circle-1", "", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-4",
			AttemptID:       "attempt-4",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if result.Success {
			t.Error("4th attempt should be blocked by rate limit")
		}

		if result.Status != execution.SettlementBlocked {
			t.Errorf("expected blocked status, got %s", result.Status)
		}

		// Verify rate limit blocked event was emitted (v9.11.1: rate limits emit separate events)
		rateLimitBlocked := false
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV911RateLimitBlocked {
				rateLimitBlocked = true
				break
			}
		}
		if !rateLimitBlocked {
			t.Error("expected rate limit blocked event")
		}
	})
}

// Scenario 2: Intersection attempt limit blocks
func TestIntersectionAttemptLimit(t *testing.T) {
	policy := caps.Policy{
		Enabled:                       true,
		MaxAttemptsPerDayIntersection: 2,
	}

	connector := NewMockWriteConnector()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	// Execute 2 attempts via intersection
	for i := 1; i <= 2; i++ {
		envelope := createTestEnvelopeWithHash(executor, "env-"+string(rune('0'+i)), "circle-1", "intersection-1", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-" + string(rune('0'+i)),
			AttemptID:       "attempt-" + string(rune('0'+i)),
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution %d failed: %v", i, err)
		}

		if !result.Success {
			t.Errorf("intersection attempt %d should succeed: %s", i, result.BlockedReason)
		}
	}

	// 3rd attempt via same intersection should be blocked
	t.Run("3rd intersection attempt blocked", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-3", "circle-1", "intersection-1", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-3",
			AttemptID:       "attempt-3",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if result.Success {
			t.Error("3rd intersection attempt should be blocked")
		}
	})

	// Different intersection should have its own limit
	t.Run("different intersection has own limit", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-4", "circle-1", "intersection-2", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-4",
			AttemptID:       "attempt-4",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("different intersection should succeed: %s", result.BlockedReason)
		}
	})
}

// Scenario 3: Spend cap blocks when money actually moves
func TestCircleDailyCapBlocks(t *testing.T) {
	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100, // 100 cents = £1.00
		},
	}

	// Use connector that reports money moved
	connector := NewMockWriteConnectorWithMoneyMoved()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	t.Run("first payment within cap succeeds", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-1", "circle-1", "", 50, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-1",
			AttemptID:       "attempt-1",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("first payment should succeed: %s", result.BlockedReason)
		}

		if !result.MoneyMoved {
			t.Error("expected money moved for this test")
		}
	})

	t.Run("second payment exceeding cap is blocked", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-2", "circle-1", "", 60, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-2",
			AttemptID:       "attempt-2",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if result.Success {
			t.Error("second payment should be blocked (exceeds cap)")
		}

		if result.Status != execution.SettlementBlocked {
			t.Errorf("expected blocked status, got %s", result.Status)
		}

		// Verify caps blocked event was emitted
		capsBlocked := false
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV911CapsBlocked {
				capsBlocked = true
				break
			}
		}
		if !capsBlocked {
			t.Error("expected caps blocked event")
		}
	})
}

// Scenario 4: Simulated execution does not count toward spend
func TestSimulatedDoesNotCountSpend(t *testing.T) {
	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
		},
	}

	// Use standard mock (simulated, no money moves)
	connector := NewMockWriteConnector()
	executor, capsGate, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	// Execute 3 simulated payments of 50 cents each
	// Since simulated, all should pass (spend not counted)
	for i := 1; i <= 3; i++ {
		envelope := createTestEnvelopeWithHash(executor, "env-"+string(rune('0'+i)), "circle-1", "", 50, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-" + string(rune('0'+i)),
			AttemptID:       "attempt-" + string(rune('0'+i)),
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution %d failed: %v", i, err)
		}

		if !result.Success {
			t.Errorf("simulated payment %d should succeed: %s", i, result.BlockedReason)
		}

		if result.MoneyMoved {
			t.Errorf("simulated payment should not move money")
		}
	}

	// Verify spend counter is still 0
	store := capsGate.GetStore()
	dayKey := caps.DayKey(testClock())
	counters := store.GetCounters(dayKey, caps.ScopeCircle, "circle-1", "GBP")

	if counters.MoneyMovedCents != 0 {
		t.Errorf("simulated payments should not count toward spend, got %d", counters.MoneyMovedCents)
	}

	// But attempts should be counted
	if counters.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", counters.Attempts)
	}
}

// Test: Exactly one trace finalization per attempt
func TestExactlyOneTraceFinalization(t *testing.T) {
	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
		},
	}

	connector := NewMockWriteConnector()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	envelope := createTestEnvelopeWithHash(executor, "env-1", "circle-1", "", 50, "GBP")

	result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-1",
		AttemptID:       "attempt-1",
		Now:             now,
		Clock:           testClock(),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Count trace finalization events
	finalizedCount := 0
	for _, e := range result.AuditEvents {
		if e.Type == events.EventV96AttemptFinalized {
			finalizedCount++
		}
	}

	if finalizedCount != 1 {
		t.Errorf("expected exactly 1 trace finalization, got %d", finalizedCount)
	}
}

// Test: v9.11 audit events are emitted
func TestV911AuditEvents(t *testing.T) {
	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
		},
	}

	connector := NewMockWriteConnector()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	envelope := createTestEnvelopeWithHash(executor, "env-1", "circle-1", "", 50, "GBP")

	result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-1",
		AttemptID:       "attempt-1",
		Now:             now,
		Clock:           testClock(),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Check for v9.11 events
	foundPolicyApplied := false
	foundChecked := false
	foundAttemptCounted := false

	for _, e := range result.AuditEvents {
		switch e.Type {
		case events.EventV911CapsPolicyApplied:
			foundPolicyApplied = true
		case events.EventV911CapsChecked:
			foundChecked = true
		case events.EventV911CapsAttemptCounted:
			foundAttemptCounted = true
		}
	}

	if !foundPolicyApplied {
		t.Error("missing v9.caps.policy.applied event")
	}
	if !foundChecked {
		t.Error("missing v9.caps.checked event")
	}
	if !foundAttemptCounted {
		t.Error("missing v9.caps.attempt.counted event")
	}
}

// Test: Payee daily cap blocks
func TestPayeeDailyCapBlocks(t *testing.T) {
	policy := caps.Policy{
		Enabled: true,
		PerPayeeDailyCapCents: map[string]int64{
			"GBP": 50, // 50 cents per payee
		},
	}

	// Use connector that reports money moved
	connector := NewMockWriteConnectorWithMoneyMoved()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	t.Run("payment to payee succeeds", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-1", "circle-1", "", 50, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-1",
			AttemptID:       "attempt-1",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("first payee payment should succeed: %s", result.BlockedReason)
		}
	})

	t.Run("second payment to same payee is blocked", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-2", "circle-1", "", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-2",
			AttemptID:       "attempt-2",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if result.Success {
			t.Error("second payee payment should be blocked")
		}
	})

	t.Run("payment to different payee succeeds", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-3", "circle-1", "", 50, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-rent",
			ExplicitApprove: true,
			TraceID:         "trace-3",
			AttemptID:       "attempt-3",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("payment to different payee should succeed: %s", result.BlockedReason)
		}
	})
}

// Test: v9.11.1 Rate limit audit events are emitted
func TestV911RateLimitAuditEvents(t *testing.T) {
	policy := caps.Policy{
		Enabled:                 true,
		MaxAttemptsPerDayCircle: 2,
	}

	connector := NewMockWriteConnector()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	// First attempt - should emit rate limit checked (passed)
	t.Run("rate limit checked event on pass", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-1", "circle-rl", "", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-1",
			AttemptID:       "attempt-1",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("first attempt should succeed: %s", result.BlockedReason)
		}

		// Check for rate limit checked event
		foundRateLimitChecked := false
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV911RateLimitChecked {
				foundRateLimitChecked = true
				// Verify metadata
				if e.Metadata["scope_type"] != "circle" {
					t.Errorf("expected scope_type=circle, got %s", e.Metadata["scope_type"])
				}
				if e.Metadata["decision"] != "allowed" {
					t.Errorf("expected decision=allowed, got %s", e.Metadata["decision"])
				}
				if e.Metadata["day_key"] == "" {
					t.Error("expected day_key to be set")
				}
				if e.Metadata["limit_value"] != "2" {
					t.Errorf("expected limit_value=2, got %s", e.Metadata["limit_value"])
				}
				break
			}
		}
		if !foundRateLimitChecked {
			t.Error("missing v9.ratelimit.checked event")
		}
	})

	// Execute second attempt to use up the limit
	envelope2 := createTestEnvelopeWithHash(executor, "env-2", "circle-rl", "", 10, "GBP")
	_, _ = executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope2,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-2",
		AttemptID:       "attempt-2",
		Now:             now,
		Clock:           testClock(),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	// Third attempt - should emit rate limit blocked
	t.Run("rate limit blocked event on block", func(t *testing.T) {
		envelope := createTestEnvelopeWithHash(executor, "env-3", "circle-rl", "", 10, "GBP")

		result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
			Envelope:        envelope,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         "trace-3",
			AttemptID:       "attempt-3",
			Now:             now,
			Clock:           testClock(),
			Policy:          &execution.MultiPartyPolicy{Mode: "single"},
		})

		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		if result.Success {
			t.Error("third attempt should be blocked")
		}

		// Check for rate limit blocked event
		foundRateLimitBlocked := false
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV911RateLimitBlocked {
				foundRateLimitBlocked = true
				// Verify metadata
				if e.Metadata["scope_type"] != "circle" {
					t.Errorf("expected scope_type=circle, got %s", e.Metadata["scope_type"])
				}
				if e.Metadata["decision"] != "blocked" {
					t.Errorf("expected decision=blocked, got %s", e.Metadata["decision"])
				}
				if e.Metadata["reason"] == "" {
					t.Error("expected reason to be set for blocked event")
				}
				if e.Metadata["current_value"] != "2" {
					t.Errorf("expected current_value=2, got %s", e.Metadata["current_value"])
				}
				break
			}
		}
		if !foundRateLimitBlocked {
			t.Error("missing v9.ratelimit.blocked event")
		}
	})
}

// Test: v9.11.1 Caps audit events include required metadata
func TestV911CapsEventMetadata(t *testing.T) {
	policy := caps.Policy{
		Enabled: true,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 100,
		},
	}

	// Use connector that reports money moved
	connector := NewMockWriteConnectorWithMoneyMoved()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	envelope := createTestEnvelopeWithHash(executor, "env-meta-1", "circle-meta", "", 50, "GBP")

	result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-meta",
		AttemptID:       "attempt-meta",
		Now:             now,
		Clock:           testClock(),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Find caps checked event and verify all required metadata
	for _, e := range result.AuditEvents {
		if e.Type == events.EventV911CapsChecked {
			// Required fields per v9.11.1 spec
			requiredFields := []string{
				"envelope_id",
				"attempt_id",
				"day_key",
				"scope_type",
				"scope_id",
				"current_value",
				"limit_value",
				"requested_value",
				"decision",
				"currency",
			}

			for _, field := range requiredFields {
				if e.Metadata[field] == "" {
					t.Errorf("missing required metadata field: %s", field)
				}
			}

			// Verify specific values
			if e.Metadata["envelope_id"] != "env-meta-1" {
				t.Errorf("expected envelope_id=env-meta-1, got %s", e.Metadata["envelope_id"])
			}
			if e.Metadata["attempt_id"] != "attempt-meta" {
				t.Errorf("expected attempt_id=attempt-meta, got %s", e.Metadata["attempt_id"])
			}
			if e.Metadata["currency"] != "GBP" {
				t.Errorf("expected currency=GBP, got %s", e.Metadata["currency"])
			}
			if e.Metadata["limit_value"] != "100" {
				t.Errorf("expected limit_value=100, got %s", e.Metadata["limit_value"])
			}
			if e.Metadata["requested_value"] != "50" {
				t.Errorf("expected requested_value=50, got %s", e.Metadata["requested_value"])
			}
			break
		}
	}
}

// Test: No duplicate trace finalization with v9.11.1 events
func TestNoExtraFinalizationWithV911Events(t *testing.T) {
	policy := caps.Policy{
		Enabled:                 true,
		MaxAttemptsPerDayCircle: 5,
		PerCircleDailyCapCents: map[string]int64{
			"GBP": 1000,
		},
	}

	connector := NewMockWriteConnector()
	executor, _, _ := createTestExecutor(policy, connector)
	ctx := context.Background()
	now := testClock().Now()

	envelope := createTestEnvelopeWithHash(executor, "env-fin", "circle-fin", "", 50, "GBP")

	result, err := executor.Execute(ctx, execution.V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-fin",
		AttemptID:       "attempt-fin",
		Now:             now,
		Clock:           testClock(),
		Policy:          &execution.MultiPartyPolicy{Mode: "single"},
	})

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Count trace finalization events
	finalizedCount := 0
	for _, e := range result.AuditEvents {
		if e.Type == events.EventV96AttemptFinalized {
			finalizedCount++
		}
	}

	if finalizedCount != 1 {
		t.Errorf("expected exactly 1 trace finalization, got %d", finalizedCount)
	}

	// Also count v9.11 events - should have multiple
	v911Count := 0
	for _, e := range result.AuditEvents {
		switch e.Type {
		case events.EventV911CapsPolicyApplied,
			events.EventV911CapsChecked,
			events.EventV911CapsBlocked,
			events.EventV911CapsAttemptCounted,
			events.EventV911CapsSpendCounted,
			events.EventV911RateLimitChecked,
			events.EventV911RateLimitBlocked:
			v911Count++
		}
	}

	if v911Count == 0 {
		t.Error("expected at least one v9.11 audit event")
	}
}

// Verify interface compliance
var _ write.WriteConnector = (*MockWriteConnector)(nil)
