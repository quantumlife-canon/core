// Package execexecutor tests for finance execution routing.
//
// Phase 17b: Finance execution via mock provider tests.
package execexecutor

import (
	"context"
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/execintent"
	"quantumlife/pkg/events"
)

// TestFinanceExecutionViaAdapter verifies finance execution through the adapter.
func TestFinanceExecutionViaAdapter(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	// Create finance executor adapter
	adapter := NewFinanceExecutorAdapter(
		clk,
		nil, // No emitter for test
		func() string { return "test-id-001" },
		DefaultFinanceExecutorAdapterConfig(),
	)

	// Get expected hashes from the adapter
	policyHash := adapter.GetExpectedPolicyHash()
	viewHash := adapter.GetExpectedViewHash()

	// Create execution request with correct hashes
	req := FinanceExecuteRequest{
		IntentID:           "intent-test-001",
		DraftID:            "draft-payment-001",
		CircleID:           "circle-satish",
		PayeeID:            "sandbox-utility",
		AmountCents:        50, // 50p - within £1.00 cap
		Currency:           "GBP",
		Description:        "Test payment",
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   viewHash,
		TraceID:            "trace-001",
		Now:                now,
	}

	result := adapter.ExecuteFromIntent(context.Background(), req)

	// Verify success
	if !result.Success {
		t.Fatalf("execution failed: blocked=%t reason=%s error=%s",
			result.Blocked, result.BlockedReason, result.Error)
	}

	// Verify simulated (mock provider)
	if !result.Simulated {
		t.Error("expected Simulated=true for mock provider")
	}

	// Verify no money moved
	if result.MoneyMoved {
		t.Error("expected MoneyMoved=false for mock provider")
	}

	// Verify provider
	if result.ProviderUsed != "mock-write" {
		t.Errorf("expected provider mock-write, got %s", result.ProviderUsed)
	}

	t.Logf("Finance execution successful: envelope=%s provider=%s simulated=%t",
		result.EnvelopeID, result.ProviderUsed, result.Simulated)
}

// TestFinanceExecutionMissingPolicyHashBlock verifies policy hash blocking.
func TestFinanceExecutionMissingPolicyHashBlock(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	adapter := NewFinanceExecutorAdapter(
		clk,
		nil,
		func() string { return "test-id-002" },
		DefaultFinanceExecutorAdapterConfig(),
	)

	viewHash := adapter.GetExpectedViewHash()

	req := FinanceExecuteRequest{
		IntentID:           "intent-test-002",
		DraftID:            "draft-payment-002",
		CircleID:           "circle-satish",
		PayeeID:            "sandbox-utility",
		AmountCents:        50,
		Currency:           "GBP",
		PolicySnapshotHash: "", // Missing!
		ViewSnapshotHash:   viewHash,
		Now:                now,
	}

	result := adapter.ExecuteFromIntent(context.Background(), req)

	if result.Success {
		t.Error("expected failure for missing PolicySnapshotHash")
	}
	if !result.Blocked {
		t.Error("expected blocked=true")
	}
	if result.BlockedReason == "" {
		t.Error("expected BlockedReason to be set")
	}

	t.Logf("Missing policy hash blocked: %s", result.BlockedReason)
}

// TestFinanceExecutionMissingViewHashBlock verifies view hash blocking.
func TestFinanceExecutionMissingViewHashBlock(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	adapter := NewFinanceExecutorAdapter(
		clk,
		nil,
		func() string { return "test-id-003" },
		DefaultFinanceExecutorAdapterConfig(),
	)

	policyHash := adapter.GetExpectedPolicyHash()

	req := FinanceExecuteRequest{
		IntentID:           "intent-test-003",
		DraftID:            "draft-payment-003",
		CircleID:           "circle-satish",
		PayeeID:            "sandbox-utility",
		AmountCents:        50,
		Currency:           "GBP",
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   "", // Missing!
		Now:                now,
	}

	result := adapter.ExecuteFromIntent(context.Background(), req)

	if result.Success {
		t.Error("expected failure for missing ViewSnapshotHash")
	}
	if !result.Blocked {
		t.Error("expected blocked=true")
	}

	t.Logf("Missing view hash blocked: %s", result.BlockedReason)
}

// TestFinanceExecutionIdempotency verifies idempotent execution.
func TestFinanceExecutionIdempotency(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	adapter := NewFinanceExecutorAdapter(
		clk,
		nil,
		func() string { return "test-id-004" },
		DefaultFinanceExecutorAdapterConfig(),
	)

	policyHash := adapter.GetExpectedPolicyHash()
	viewHash := adapter.GetExpectedViewHash()

	req := FinanceExecuteRequest{
		IntentID:           "intent-test-004",
		DraftID:            "draft-payment-004",
		CircleID:           "circle-satish",
		PayeeID:            "sandbox-utility",
		AmountCents:        50,
		Currency:           "GBP",
		Description:        "Idempotency test",
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   viewHash,
		Now:                now,
	}

	// First execution
	result1 := adapter.ExecuteFromIntent(context.Background(), req)
	if !result1.Success {
		t.Fatalf("first execution failed: %s", result1.Error)
	}

	// Second execution with same intent ID - should return same envelope
	result2 := adapter.ExecuteFromIntent(context.Background(), req)

	// Both should return same envelope ID (idempotency)
	if result1.EnvelopeID != result2.EnvelopeID {
		t.Errorf("envelope IDs should match: %s != %s", result1.EnvelopeID, result2.EnvelopeID)
	}

	t.Logf("Idempotency verified: envelope=%s", result1.EnvelopeID)
}

// TestExecutorRoutingToFinance verifies the full routing path.
func TestExecutorRoutingToFinance(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	// Create finance adapter
	financeAdapter := NewFinanceExecutorAdapter(
		clk,
		nil,
		func() string { return "test-id-005" },
		DefaultFinanceExecutorAdapterConfig(),
	)

	policyHash := financeAdapter.GetExpectedPolicyHash()
	viewHash := financeAdapter.GetExpectedViewHash()

	// Create executor with finance adapter
	executor := NewExecutor(clk, nil).WithFinanceExecutor(financeAdapter)

	// Create finance intent
	intent := &execintent.ExecutionIntent{
		DraftID:            "draft-payment-005",
		CircleID:           "circle-satish",
		Action:             execintent.ActionFinancePayment,
		FinancePayeeID:     "sandbox-utility",
		FinanceAmountCents: 50,
		FinanceCurrency:    "GBP",
		FinanceDescription: "Full path test",
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   viewHash,
		CreatedAt:          now,
	}
	intent.Finalize()

	// Execute
	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-005")

	if !outcome.Success {
		t.Fatalf("execution failed: blocked=%t reason=%s error=%s",
			outcome.Blocked, outcome.BlockedReason, outcome.Error)
	}

	t.Logf("Full routing path succeeded: intent=%s envelope=%s",
		intent.IntentID, outcome.EnvelopeID)
}

// TestExecutorFinanceNotConfigured verifies behavior when finance executor missing.
func TestExecutorFinanceNotConfigured(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	// Create executor WITHOUT finance adapter
	executor := NewExecutor(clk, nil)

	intent := &execintent.ExecutionIntent{
		DraftID:            "draft-payment-006",
		CircleID:           "circle-satish",
		Action:             execintent.ActionFinancePayment,
		FinancePayeeID:     "sandbox-utility",
		FinanceAmountCents: 50,
		FinanceCurrency:    "GBP",
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		CreatedAt:          now,
	}
	intent.Finalize()

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-006")

	if outcome.Success {
		t.Error("expected failure when finance executor not configured")
	}
	if !outcome.Blocked {
		t.Error("expected blocked=true")
	}
	if outcome.BlockedReason != "finance executor not configured" {
		t.Errorf("unexpected reason: %s", outcome.BlockedReason)
	}

	t.Logf("Missing finance executor blocked: %s", outcome.BlockedReason)
}

// TestFinanceExecutionEvents verifies events are emitted.
func TestFinanceExecutionEvents(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)
	emitter := &mockEmitter{}

	adapter := NewFinanceExecutorAdapter(
		clk,
		emitter,
		func() string { return "test-id-007" },
		DefaultFinanceExecutorAdapterConfig(),
	)

	policyHash := adapter.GetExpectedPolicyHash()
	viewHash := adapter.GetExpectedViewHash()

	req := FinanceExecuteRequest{
		IntentID:           "intent-test-007",
		DraftID:            "draft-payment-007",
		CircleID:           "circle-satish",
		PayeeID:            "sandbox-utility",
		AmountCents:        50,
		Currency:           "GBP",
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   viewHash,
		Now:                now,
	}

	adapter.ExecuteFromIntent(context.Background(), req)

	// Should have emitted at least one event
	if len(emitter.events) == 0 {
		t.Error("expected events to be emitted")
	}

	// Find completion event
	var foundCompletion bool
	for _, e := range emitter.events {
		if e.Type == events.Phase17FinanceExecutionSucceeded ||
			e.Type == events.Phase17FinanceExecutionBlocked ||
			e.Type == events.Phase17FinanceExecutionFailed {
			foundCompletion = true
			break
		}
	}

	if !foundCompletion {
		t.Error("expected completion event")
	}

	t.Logf("Events emitted: %d", len(emitter.events))
}
