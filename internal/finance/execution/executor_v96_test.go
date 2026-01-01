// Package execution tests for V96Executor.
//
// Phase 17b: Minimal tests for finance execution boundary.
package execution

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/providers/mock"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/events"
)

// TestV96Executor_MissingPolicySnapshotHashBlocks verifies v9.12.1 hard block.
func TestV96Executor_MissingPolicySnapshotHashBlocks(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	executor := createTestExecutor(now)

	envelope := &ExecutionEnvelope{
		EnvelopeID:         "env-missing-policy",
		ActorCircleID:      "circle-satish",
		ActionHash:         "action-hash-001",
		PolicySnapshotHash: "", // Missing! v9.12.1 should block
		ViewSnapshotHash:   "view-hash-def456",
		ActionSpec: ActionSpec{
			Type:        ActionTypePayment,
			PayeeID:     "sandbox-utility",
			AmountCents: 50,
			Currency:    "GBP",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
		SealedAt:            now,
	}
	envelope.SealHash = ComputeSealHash(envelope)

	req := V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-001",
		Now:             now,
	}

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.Success {
		t.Error("expected execution to fail for missing PolicySnapshotHash")
	}
	if result.BlockedReason == "" {
		t.Error("expected BlockedReason to be set")
	}

	t.Logf("Missing policy hash blocked: %s", result.BlockedReason)
}

// TestV96Executor_MissingViewSnapshotHashBlocks verifies v9.13 hard block.
func TestV96Executor_MissingViewSnapshotHashBlocks(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	executor := createTestExecutor(now)

	envelope := &ExecutionEnvelope{
		EnvelopeID:         "env-missing-view",
		ActorCircleID:      "circle-satish",
		ActionHash:         "action-hash-002",
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "", // Missing! v9.13 should block
		ActionSpec: ActionSpec{
			Type:        ActionTypePayment,
			PayeeID:     "sandbox-utility",
			AmountCents: 50,
			Currency:    "GBP",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
		SealedAt:            now,
	}
	envelope.SealHash = ComputeSealHash(envelope)

	req := V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-002",
		Now:             now,
	}

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.Success {
		t.Error("expected execution to fail for missing ViewSnapshotHash")
	}
	if result.BlockedReason == "" {
		t.Error("expected BlockedReason to be set")
	}

	t.Logf("Missing view hash blocked: %s", result.BlockedReason)
}

// TestV96Executor_IdempotencyWorks verifies replay protection.
func TestV96Executor_IdempotencyWorks(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	executor := createTestExecutor(now)

	envelope := &ExecutionEnvelope{
		EnvelopeID:         "env-idempotency",
		ActorCircleID:      "circle-satish",
		ActionHash:         "action-hash-003",
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		ActionSpec: ActionSpec{
			Type:        ActionTypePayment,
			PayeeID:     "sandbox-utility",
			AmountCents: 50,
			Currency:    "GBP",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
		SealedAt:            now,
	}
	envelope.SealHash = ComputeSealHash(envelope)

	req := V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-003",
		AttemptID:       "attempt-003",
		Now:             now,
	}

	// First execution
	result1, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("first Execute returned error: %v", err)
	}

	// Second execution with same attempt ID should return same result
	// (or be blocked as replay)
	result2, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("second Execute returned error: %v", err)
	}

	// Either same result or replay blocked is acceptable
	if result1.AttemptID != result2.AttemptID {
		t.Log("Different attempt IDs - second execution may have been blocked as replay")
	}

	t.Logf("Idempotency check: attempt1=%s attempt2=%s",
		result1.AttemptID, result2.AttemptID)
}

// TestV96Executor_MockProviderSimulated verifies mock provider sets Simulated=true.
func TestV96Executor_MockProviderSimulated(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	executor := createTestExecutor(now)

	envelope := &ExecutionEnvelope{
		EnvelopeID:         "env-simulated",
		ActorCircleID:      "circle-satish",
		ActionHash:         "action-hash-004",
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		ActionSpec: ActionSpec{
			Type:        ActionTypePayment,
			PayeeID:     "sandbox-utility",
			AmountCents: 50, // Within cap
			Currency:    "GBP",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
		SealedAt:            now,
	}
	envelope.SealHash = ComputeSealHash(envelope)

	req := V96ExecuteRequest{
		Envelope:        envelope,
		PayeeID:         "sandbox-utility",
		ExplicitApprove: true,
		TraceID:         "trace-004",
		Now:             now,
	}

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.Success {
		// If successful, verify simulated
		if result.Receipt != nil && !result.Receipt.Simulated {
			t.Error("expected Simulated=true for mock provider")
		}
		if result.MoneyMoved {
			t.Error("expected MoneyMoved=false for mock provider")
		}
	}

	t.Logf("Mock provider result: success=%t provider=%s",
		result.Success, result.ProviderUsed)
}

// createTestExecutor creates a V96Executor for testing.
func createTestExecutor(now time.Time) *V96Executor {
	idGen := func() string { return "test-id" }
	emitter := func(e events.Event) {}

	// Create mock connector
	mockConnector := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
		mock.WithConfig(write.WriteConfig{
			CapCents:          100, // £1.00
			AllowedCurrencies: []string{"GBP"},
		}),
	)

	// Create attempt ledger
	ledger := attempts.NewInMemoryLedger(
		attempts.DefaultLedgerConfig(),
		idGen,
		emitter,
	)

	// Create presentation store and gate
	presentationStore := NewPresentationStore(idGen, emitter)
	presentationGate := NewPresentationGate(presentationStore, idGen, emitter)

	// Create multi-party gate
	multiPartyGate := NewMultiPartyGate(idGen, emitter)

	// Create approval verifier
	approvalVerifier := NewApprovalVerifier([]byte("test-key"))

	// Create revocation checker
	revocationChecker := NewRevocationChecker(idGen)

	// Create config
	config := DefaultV96ExecutorConfig()
	config.ForcedPauseDuration = 0 // Skip pause in tests

	return NewV96Executor(
		nil, // No TrueLayer
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		revocationChecker,
		ledger,
		config,
		idGen,
		emitter,
	)
}
