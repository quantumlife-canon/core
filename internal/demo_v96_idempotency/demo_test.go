// Package demo_v96_idempotency provides tests for v9.6 idempotency and replay defense.
package demo_v96_idempotency

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write/payees"
	"quantumlife/internal/connectors/finance/write/registry"
	"quantumlife/internal/finance/execution"
	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/events"
)

// setupTestExecutor creates a test executor with all components.
func setupTestExecutor() (
	*execution.V96Executor,
	*execution.PresentationStore,
	*execution.RevocationChecker,
	*attempts.InMemoryLedger,
	func() string,
	func(events.Event),
) {
	counter := 0
	idGen := func() string {
		counter++
		return "test_" + string(rune('a'+counter))
	}

	var auditEvents []events.Event
	emitter := func(e events.Event) {
		auditEvents = append(auditEvents, e)
	}

	signingKey := []byte("test-signing-key")
	presentationStore := execution.NewPresentationStore(idGen, emitter)
	revocationChecker := execution.NewRevocationChecker(idGen)
	presentationGate := execution.NewPresentationGate(presentationStore, idGen, emitter)
	multiPartyGate := execution.NewMultiPartyGate(idGen, emitter)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)
	attemptLedger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, emitter)
	mockConnector := NewMockWriteConnector(idGen, emitter)

	config := execution.DefaultV96ExecutorConfig()
	config.ForcedPauseDuration = 100 * time.Millisecond
	config.RevocationPollInterval = 10 * time.Millisecond
	config.TrueLayerConfigured = false

	executor := execution.NewV96Executor(
		nil,
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		revocationChecker,
		attemptLedger,
		config,
		idGen,
		emitter,
	)

	// v9.12: Set registries for policy snapshot computation
	executor.SetProviderRegistry(registry.NewDefaultRegistry())
	executor.SetPayeeRegistry(payees.NewDefaultRegistry())

	// v9.13: Set view provider for view freshness verification
	viewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
		ProviderID:      "mock-view",
		Clock:           &testClock{},
		IDGenerator:     idGen,
		PayeeAllowed:    true,
		ProviderAllowed: true,
		BalanceOK:       true,
		Accounts:        []string{"acct-1"},
		SharedViewHash:  "test-shared-hash",
	})
	viewProvider.SetSnapshotIDOverride("test-snapshot")
	viewProvider.SetCapturedAtOverride(testTime)
	executor.SetViewProvider(viewProvider)

	return executor, presentationStore, revocationChecker, attemptLedger, idGen, emitter
}

// testTime is the fixed time used for all v9.13 view snapshot tests.
// Must be consistent between MockViewProvider and envelope creation.
var testTime = time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)

// testClock provides a fixed time for deterministic tests.
type testClock struct{}

func (c *testClock) Now() time.Time {
	return testTime
}

// realTimeClock uses actual wall clock time.
type realTimeClock struct{}

func (c *realTimeClock) Now() time.Time {
	return time.Now()
}

// createTestEnvelope creates a test envelope with bundle.
// v9.12.1: Now takes executor to compute PolicySnapshotHash.
// v9.13: Also sets ViewSnapshotHash.
func createTestEnvelope(executor *execution.V96Executor, idGen func() string, amountCents int64, currency string) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
	now := time.Now()
	builder := execution.NewEnvelopeBuilder(idGen)

	intent := execution.ExecutionIntent{
		IntentID:       idGen(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_test",
		Description:    "Test payment",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    amountCents,
		Currency:       currency,
		PayeeID:        "sandbox-utility",
		ViewHash:       "v8_view_" + idGen(),
		CreatedAt:      now,
	}

	envelope, _ := builder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        2,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                100,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  idGen(),
	}, now)
	envelope.SealHash = execution.ComputeSealHash(envelope)

	// v9.12.1: Compute and bind policy snapshot hash
	_, hash := executor.ComputePolicySnapshotForEnvelope()
	envelope.PolicySnapshotHash = string(hash)

	// v9.13: Compute and bind view snapshot hash (using deterministic snapshot)
	// Use testTime for CapturedAt to match MockViewProvider's override
	viewSnapshot := execution.ViewSnapshot{
		SnapshotID:         "test-snapshot",
		CapturedAt:         testTime,
		CircleID:           "circle_alice",
		IntersectionID:     "intersection_test",
		PayeeID:            "sandbox-utility",
		Currency:           currency,
		AmountCents:        amountCents,
		PayeeAllowed:       true,
		ProviderID:         "mock-write",
		ProviderAllowed:    true,
		AccountVisibility:  []string{"acct-1"},
		SharedViewHash:     "test-shared-hash",
		BalanceCheckPassed: true,
	}
	envelope.ViewSnapshotHash = string(execution.ComputeViewSnapshotHash(viewSnapshot))

	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Test approval request for payment.",
		300,
		idGen,
	)
	// v9.13: BuildApprovalBundle now copies ViewSnapshotHash from envelope automatically

	return envelope, bundle
}

// createTestApprovalsWithPresentation creates approvals with proper presentations.
func createTestApprovalsWithPresentation(
	idGen func() string,
	envelope *execution.ExecutionEnvelope,
	bundle *execution.ApprovalBundle,
	approvers []string,
	presentationStore *execution.PresentationStore,
	traceID string,
) ([]execution.MultiPartyApprovalArtifact, []execution.ApproverBundleHash) {
	now := time.Now()
	approvals := make([]execution.MultiPartyApprovalArtifact, 0, len(approvers))
	hashes := make([]execution.ApproverBundleHash, 0, len(approvers))

	for _, approverCircle := range approvers {
		presentationStore.RecordPresentation(
			approverCircle,
			approverCircle+"_user",
			bundle,
			envelope,
			traceID,
			5*time.Minute,
			now,
		)

		approval := execution.MultiPartyApprovalArtifact{
			ApprovalArtifact: execution.ApprovalArtifact{
				ArtifactID:       idGen(),
				ApproverCircleID: approverCircle,
				ApproverID:       approverCircle + "_user",
				ActionHash:       envelope.ActionHash,
				ApprovedAt:       now,
				ExpiresAt:        now.Add(5 * time.Minute),
				Signature:        idGen(),
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

// Test 1: Idempotency key derivation is deterministic
func TestIdempotencyKeyDeterminism(t *testing.T) {
	t.Run("same inputs produce same key", func(t *testing.T) {
		input := attempts.IdempotencyKeyInput{
			EnvelopeID: "env-123",
			ActionHash: "hash-456",
			AttemptID:  "attempt-1",
			SealHash:   "seal-789",
		}

		key1 := attempts.DeriveIdempotencyKey(input)
		key2 := attempts.DeriveIdempotencyKey(input)

		if key1 != key2 {
			t.Errorf("keys should be identical, got %s and %s", key1, key2)
		}

		if len(key1) != 64 {
			t.Errorf("key should be 64 hex chars, got %d", len(key1))
		}
	})

	t.Run("different inputs produce different keys", func(t *testing.T) {
		input1 := attempts.IdempotencyKeyInput{
			EnvelopeID: "env-123",
			ActionHash: "hash-456",
			AttemptID:  "attempt-1",
			SealHash:   "seal-789",
		}

		input2 := attempts.IdempotencyKeyInput{
			EnvelopeID: "env-123",
			ActionHash: "hash-456",
			AttemptID:  "attempt-2", // Different attempt
			SealHash:   "seal-789",
		}

		key1 := attempts.DeriveIdempotencyKey(input1)
		key2 := attempts.DeriveIdempotencyKey(input2)

		if key1 == key2 {
			t.Error("different inputs should produce different keys")
		}
	})
}

// Test 2: Ledger enforces unique attempt IDs
func TestLedgerUniqueAttemptID(t *testing.T) {
	t.Run("duplicate attempt ID is rejected", func(t *testing.T) {
		counter := 0
		idGen := func() string {
			counter++
			return "id_" + string(rune('a'+counter))
		}
		ledger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, nil)

		now := time.Now()
		req := attempts.StartAttemptRequest{
			AttemptID:      "attempt-1",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "key-1",
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		}

		_, err := ledger.StartAttempt(req)
		if err != nil {
			t.Fatalf("first attempt should succeed: %v", err)
		}

		// Finalize first attempt
		ledger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
			AttemptID: "attempt-1",
			Status:    attempts.AttemptStatusSimulated,
			Now:       now,
		})

		// Try same attempt ID
		_, err = ledger.StartAttempt(req)
		if err != attempts.ErrAttemptAlreadyExists {
			t.Errorf("expected ErrAttemptAlreadyExists, got %v", err)
		}
	})
}

// Test 3: Ledger enforces one in-flight attempt per envelope
func TestLedgerInflightPolicy(t *testing.T) {
	t.Run("second attempt blocked while first in-flight", func(t *testing.T) {
		counter := 0
		idGen := func() string {
			counter++
			return "id_" + string(rune('a'+counter))
		}
		ledger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, nil)

		now := time.Now()

		// Start first attempt (stays in-flight)
		_, err := ledger.StartAttempt(attempts.StartAttemptRequest{
			AttemptID:      "attempt-1",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "key-1",
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("first attempt should succeed: %v", err)
		}

		// Try second attempt for same envelope
		_, err = ledger.StartAttempt(attempts.StartAttemptRequest{
			AttemptID:      "attempt-2",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "key-2",
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		})
		if err != attempts.ErrAttemptInFlight {
			t.Errorf("expected ErrAttemptInFlight, got %v", err)
		}
	})
}

// Test 4: Terminal attempts cannot be replayed
func TestLedgerTerminalReplay(t *testing.T) {
	t.Run("replay of terminal attempt is blocked", func(t *testing.T) {
		counter := 0
		idGen := func() string {
			counter++
			return "id_" + string(rune('a'+counter))
		}
		ledger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, nil)

		now := time.Now()

		// Start and finalize attempt
		_, err := ledger.StartAttempt(attempts.StartAttemptRequest{
			AttemptID:      "attempt-1",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "key-1",
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("first attempt should succeed: %v", err)
		}

		ledger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
			AttemptID: "attempt-1",
			Status:    attempts.AttemptStatusSettled,
			Now:       now,
		})

		// Check replay
		record, isReplay := ledger.CheckReplay("env-1", "attempt-1")
		if !isReplay {
			t.Error("should detect replay")
		}
		if record.Status != attempts.AttemptStatusSettled {
			t.Errorf("expected settled status, got %s", record.Status)
		}
	})
}

// Test 5: Executor blocks replay of same attempt ID
func TestExecutorReplayBlocked(t *testing.T) {
	t.Run("second invocation with same attempt ID is blocked", func(t *testing.T) {
		executor, presentationStore, _, _, idGen, _ := setupTestExecutor()
		envelope, bundle := createTestEnvelope(executor, idGen, 100, "GBP")
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

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
			Now:             testTime,
		}

		// First invocation
		result1, _ := executor.Execute(context.Background(), req)
		if !result1.Success {
			t.Fatalf("first invocation should succeed: %s", result1.BlockedReason)
		}

		// Second invocation with same attempt ID
		req.Now = testTime
		result2, _ := executor.Execute(context.Background(), req)
		if result2.Success {
			t.Error("second invocation should be blocked")
		}
		if !result2.ReplayBlocked {
			t.Error("should be marked as replay blocked")
		}
	})
}

// Test 6: Executor blocks in-flight duplicates
func TestExecutorInflightBlocked(t *testing.T) {
	t.Run("concurrent attempt for same envelope is blocked", func(t *testing.T) {
		executor, presentationStore, _, _, idGen, _ := setupTestExecutor()
		envelope, bundle := createTestEnvelope(executor, idGen, 100, "GBP")
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		attemptID1 := attempts.DeriveAttemptID(envelope.EnvelopeID, 1)
		attemptID2 := attempts.DeriveAttemptID(envelope.EnvelopeID, 2)

		var result1, result2 *execution.V96ExecuteResult
		var wg sync.WaitGroup

		// Start first attempt in goroutine
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
				Now:             testTime,
			}
			result1, _ = executor.Execute(context.Background(), req)
		}()

		// Wait for first to start, then try second
		time.Sleep(20 * time.Millisecond)

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
			Now:             testTime,
		}
		result2, _ = executor.Execute(context.Background(), req2)

		wg.Wait()

		if !result1.Success {
			t.Errorf("first attempt should succeed: %s", result1.BlockedReason)
		}
		if result2.Success {
			t.Error("second attempt should be blocked")
		}
		if !result2.InflightBlocked {
			t.Error("should be marked as inflight blocked")
		}
	})
}

// Test 7: Mock connector reports MoneyMoved=false
func TestMockConnectorMoneyMovedFalse(t *testing.T) {
	t.Run("mock connector always reports MoneyMoved=false", func(t *testing.T) {
		executor, presentationStore, _, _, idGen, _ := setupTestExecutor()
		envelope, bundle := createTestEnvelope(executor, idGen, 100, "GBP")
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		req := execution.V96ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             testTime,
		}

		result, _ := executor.Execute(context.Background(), req)

		if !result.Success {
			t.Fatalf("execution should succeed: %s", result.BlockedReason)
		}
		if result.MoneyMoved {
			t.Error("mock connector should report MoneyMoved=false")
		}
		if result.ProviderUsed != "mock-write" {
			t.Errorf("expected mock-write provider, got %s", result.ProviderUsed)
		}
	})
}

// Test 8: Idempotency key prefix is safe to log
func TestIdempotencyKeyPrefixSafe(t *testing.T) {
	t.Run("prefix is truncated to 16 chars", func(t *testing.T) {
		key := attempts.DeriveIdempotencyKey(attempts.IdempotencyKeyInput{
			EnvelopeID: "env-123",
			ActionHash: "hash-456",
			AttemptID:  "attempt-1",
			SealHash:   "seal-789",
		})

		prefix := attempts.IdempotencyKeyPrefix(key)

		if len(prefix) > 20 { // 16 + "..."
			t.Errorf("prefix should be truncated, got length %d", len(prefix))
		}
		if !strings.HasSuffix(prefix, "...") {
			t.Error("prefix should end with ...")
		}
	})
}

// Test 9: Attempt status transitions are validated
func TestAttemptStatusTransitions(t *testing.T) {
	t.Run("valid transitions are allowed", func(t *testing.T) {
		counter := 0
		idGen := func() string {
			counter++
			return "id_" + string(rune('a'+counter))
		}
		ledger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, nil)

		now := time.Now()
		ledger.StartAttempt(attempts.StartAttemptRequest{
			AttemptID:      "attempt-1",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "key-1",
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		})

		// started -> prepared
		err := ledger.UpdateStatus("attempt-1", attempts.AttemptStatusPrepared, now)
		if err != nil {
			t.Errorf("started->prepared should be valid: %v", err)
		}

		// prepared -> invoked
		err = ledger.UpdateStatus("attempt-1", attempts.AttemptStatusInvoked, now)
		if err != nil {
			t.Errorf("prepared->invoked should be valid: %v", err)
		}

		// invoked -> settled (terminal)
		err = ledger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
			AttemptID: "attempt-1",
			Status:    attempts.AttemptStatusSettled,
			Now:       now,
		})
		if err != nil {
			t.Errorf("invoked->settled should be valid: %v", err)
		}

		// Terminal -> any should fail
		err = ledger.UpdateStatus("attempt-1", attempts.AttemptStatusPrepared, now)
		if err != attempts.ErrAttemptTerminal {
			t.Errorf("terminal->any should fail with ErrAttemptTerminal, got %v", err)
		}
	})
}

// Test 10: Idempotency key conflict is detected
func TestIdempotencyKeyConflict(t *testing.T) {
	t.Run("same idempotency key for same envelope is rejected", func(t *testing.T) {
		counter := 0
		idGen := func() string {
			counter++
			return "id_" + string(rune('a'+counter))
		}
		ledger := attempts.NewInMemoryLedger(attempts.DefaultLedgerConfig(), idGen, nil)

		now := time.Now()

		// First attempt
		_, err := ledger.StartAttempt(attempts.StartAttemptRequest{
			AttemptID:      "attempt-1",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "same-key",
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("first attempt should succeed: %v", err)
		}

		// Finalize first
		ledger.FinalizeAttempt(attempts.FinalizeAttemptRequest{
			AttemptID: "attempt-1",
			Status:    attempts.AttemptStatusSimulated,
			Now:       now,
		})

		// Second attempt with same idempotency key
		_, err = ledger.StartAttempt(attempts.StartAttemptRequest{
			AttemptID:      "attempt-2",
			EnvelopeID:     "env-1",
			ActionHash:     "hash-1",
			IdempotencyKey: "same-key", // Same key
			CircleID:       "circle-1",
			TraceID:        "trace-1",
			Provider:       "mock",
			Now:            now,
		})
		if err != attempts.ErrIdempotencyKeyConflict {
			t.Errorf("expected ErrIdempotencyKeyConflict, got %v", err)
		}
	})
}

// Test 11: Audit events contain required fields
func TestAuditEventsContainRequiredFields(t *testing.T) {
	t.Run("v9.6 events have proper metadata", func(t *testing.T) {
		executor, presentationStore, _, _, idGen, _ := setupTestExecutor()
		envelope, bundle := createTestEnvelope(executor, idGen, 100, "GBP")
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		req := execution.V96ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             testTime,
		}

		result, _ := executor.Execute(context.Background(), req)

		// Check for required v9.6 events
		hasIdempotencyDerived := false
		hasAttemptStarted := false
		hasAttemptFinalized := false

		for _, e := range result.AuditEvents {
			switch e.Type {
			case events.EventV96IdempotencyKeyDerived:
				hasIdempotencyDerived = true
				if e.Metadata["idempotency_prefix"] == "" {
					t.Error("idempotency event should have prefix in metadata")
				}
			case events.EventV96AttemptStarted:
				hasAttemptStarted = true
			case events.EventV96AttemptFinalized:
				hasAttemptFinalized = true
				if e.Metadata["status"] == "" {
					t.Error("finalized event should have status in metadata")
				}
			}
		}

		if !hasIdempotencyDerived {
			t.Error("missing EventV96IdempotencyKeyDerived")
		}
		if !hasAttemptStarted {
			t.Error("missing EventV96AttemptStarted")
		}
		if !hasAttemptFinalized {
			t.Error("missing EventV96AttemptFinalized")
		}
	})
}

// Test 12: Exactly one attempt finalization per execution
func TestSingleAttemptFinalization(t *testing.T) {
	t.Run("only one finalization event per attempt", func(t *testing.T) {
		executor, presentationStore, _, _, idGen, _ := setupTestExecutor()
		envelope, bundle := createTestEnvelope(executor, idGen, 100, "GBP")
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		req := execution.V96ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             testTime,
		}

		result, _ := executor.Execute(context.Background(), req)

		finalizationCount := 0
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV96AttemptFinalized {
				finalizationCount++
			}
		}

		if finalizationCount != 1 {
			t.Errorf("expected exactly 1 finalization event, got %d", finalizationCount)
		}
	})
}

// createTestEnvelopeWithRealTime creates a test envelope using real time for v9.13 view freshness.
// This is needed for tests that rely on wall clock behavior (like forced pause).
func createTestEnvelopeWithRealTime(executor *execution.V96Executor, idGen func() string, amountCents int64, currency string, realNow time.Time) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
	builder := execution.NewEnvelopeBuilder(idGen)

	intent := execution.ExecutionIntent{
		IntentID:       idGen(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_test",
		Description:    "Test payment",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    amountCents,
		Currency:       currency,
		PayeeID:        "sandbox-utility",
		ViewHash:       "v8_view_" + idGen(),
		CreatedAt:      realNow,
	}

	envelope, _ := builder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        2,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   realNow.Add(time.Hour),
		AmountCap:                100,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  idGen(),
	}, realNow)
	envelope.SealHash = execution.ComputeSealHash(envelope)

	// v9.12.1: Compute and bind policy snapshot hash
	_, hash := executor.ComputePolicySnapshotForEnvelope()
	envelope.PolicySnapshotHash = string(hash)

	// v9.13: Compute and bind view snapshot hash using real time
	viewSnapshot := execution.ViewSnapshot{
		SnapshotID:         "test-snapshot",
		CapturedAt:         realNow,
		CircleID:           "circle_alice",
		IntersectionID:     "intersection_test",
		PayeeID:            "sandbox-utility",
		Currency:           currency,
		AmountCents:        amountCents,
		PayeeAllowed:       true,
		ProviderID:         "mock-write",
		ProviderAllowed:    true,
		AccountVisibility:  []string{"acct-1"},
		SharedViewHash:     "test-shared-hash",
		BalanceCheckPassed: true,
	}
	envelope.ViewSnapshotHash = string(execution.ComputeViewSnapshotHash(viewSnapshot))

	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Test approval request for payment.",
		300,
		idGen,
	)

	return envelope, bundle
}

// Test 13: Revocation during pause aborts with proper ledger update
// NOTE: This test requires real time for the forced pause loop to work correctly.
// It uses a specialized executor/envelope setup that works with wall clock time.
func TestRevocationDuringPauseWithLedger(t *testing.T) {
	t.Run("revocation updates ledger to revoked status", func(t *testing.T) {
		executor, presentationStore, revocationChecker, ledger, idGen, _ := setupTestExecutor()

		// For this test, we need real time for the forced pause to work.
		// Create envelope with real time for ViewSnapshot.
		realNow := time.Now()

		// Update view provider to use real time instead of testTime
		viewProvider := execution.NewMockViewProvider(execution.MockViewProviderConfig{
			ProviderID:      "mock-view",
			Clock:           &realTimeClock{},
			IDGenerator:     idGen,
			PayeeAllowed:    true,
			ProviderAllowed: true,
			BalanceOK:       true,
			Accounts:        []string{"acct-1"},
			SharedViewHash:  "test-shared-hash",
		})
		viewProvider.SetSnapshotIDOverride("test-snapshot")
		viewProvider.SetCapturedAtOverride(realNow)
		executor.SetViewProvider(viewProvider)

		envelope, bundle := createTestEnvelopeWithRealTime(executor, idGen, 100, "GBP", realNow)
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		attemptID := attempts.DeriveAttemptID(envelope.EnvelopeID, 1)
		var result *execution.V96ExecuteResult
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
				AttemptID:       attemptID,
				Now:             time.Now(), // Use real time for forced pause to work
			}
			result, _ = executor.Execute(context.Background(), req)
		}()

		// Revoke during forced pause - wait a bit for goroutine to enter pause
		time.Sleep(20 * time.Millisecond)
		revocationChecker.Revoke(envelope.EnvelopeID, "test-circle", "test-user", "test revocation", time.Now())

		wg.Wait()

		if result.Success {
			t.Error("execution should be blocked due to revocation")
		}
		if result.Status != execution.SettlementRevoked {
			t.Errorf("expected SettlementRevoked, got %s", result.Status)
		}

		// Check ledger status
		record, exists := ledger.GetAttempt(attemptID)
		if !exists {
			t.Fatal("attempt record should exist")
		}
		if record.Status != attempts.AttemptStatusRevoked {
			t.Errorf("ledger status should be revoked, got %s", record.Status)
		}
	})
}

// Test 14: Provider receives idempotency key
func TestProviderReceivesIdempotencyKey(t *testing.T) {
	t.Run("idempotency key is attached to provider call", func(t *testing.T) {
		executor, presentationStore, _, _, idGen, _ := setupTestExecutor()
		envelope, bundle := createTestEnvelope(executor, idGen, 100, "GBP")
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		req := execution.V96ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             testTime,
		}

		result, _ := executor.Execute(context.Background(), req)

		// Check for provider idempotency attached event
		hasProviderIdempotency := false
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV96ProviderIdempotencyAttached {
				hasProviderIdempotency = true
				if e.Metadata["idempotency_prefix"] == "" {
					t.Error("provider event should have idempotency prefix")
				}
			}
		}

		if !hasProviderIdempotency {
			t.Error("missing EventV96ProviderIdempotencyAttached")
		}
	})
}

// Test 15: Cap enforcement still works with idempotency
func TestCapEnforcementWithIdempotency(t *testing.T) {
	t.Run("cap exceeded is blocked and recorded in ledger", func(t *testing.T) {
		executor, presentationStore, _, ledger, idGen, _ := setupTestExecutor()
		// Create envelope with amount > cap (using higher envelope cap to allow creation)
		envelope, bundle := createTestEnvelopeWithCap(executor, idGen, 150, "GBP", 200)
		traceID := idGen()

		approvals, hashes := createTestApprovalsWithPresentation(
			idGen, envelope, bundle,
			[]string{"circle_alice", "circle_bob"},
			presentationStore, traceID,
		)

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
			Now:             testTime,
		}

		result, _ := executor.Execute(context.Background(), req)

		if result.Success {
			t.Error("execution should be blocked due to cap exceeded")
		}
		if !strings.Contains(result.BlockedReason, "cap") {
			t.Errorf("blocked reason should mention cap: %s", result.BlockedReason)
		}

		// Check ledger recorded blocked status
		record, exists := ledger.GetAttempt(attemptID)
		if !exists {
			t.Fatal("attempt record should exist")
		}
		if record.Status != attempts.AttemptStatusBlocked {
			t.Errorf("ledger status should be blocked, got %s", record.Status)
		}
	})
}

// createTestEnvelopeWithCap creates a test envelope with custom cap.
// v9.12.1: Now takes executor to compute PolicySnapshotHash.
// v9.13: Also sets ViewSnapshotHash.
func createTestEnvelopeWithCap(executor *execution.V96Executor, idGen func() string, amountCents int64, currency string, cap int64) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
	now := time.Now()
	builder := execution.NewEnvelopeBuilder(idGen)

	intent := execution.ExecutionIntent{
		IntentID:       idGen(),
		CircleID:       "circle_alice",
		IntersectionID: "intersection_test",
		Description:    "Test payment",
		ActionType:     execution.ActionTypePayment,
		AmountCents:    amountCents,
		Currency:       currency,
		PayeeID:        "sandbox-utility",
		ViewHash:       "v8_view_" + idGen(),
		CreatedAt:      now,
	}

	envelope, _ := builder.Build(execution.BuildRequest{
		Intent:                   intent,
		ApprovalThreshold:        2,
		RevocationWindowDuration: 0,
		RevocationWaived:         true,
		Expiry:                   now.Add(time.Hour),
		AmountCap:                cap,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  idGen(),
	}, now)
	envelope.SealHash = execution.ComputeSealHash(envelope)

	// v9.12.1: Compute and bind policy snapshot hash
	_, hash := executor.ComputePolicySnapshotForEnvelope()
	envelope.PolicySnapshotHash = string(hash)

	// v9.13: Compute and bind view snapshot hash (using deterministic snapshot)
	// Use testTime for CapturedAt to match MockViewProvider's override
	viewSnapshot := execution.ViewSnapshot{
		SnapshotID:         "test-snapshot",
		CapturedAt:         testTime,
		CircleID:           "circle_alice",
		IntersectionID:     "intersection_test",
		PayeeID:            "sandbox-utility",
		Currency:           currency,
		AmountCents:        amountCents,
		PayeeAllowed:       true,
		ProviderID:         "mock-write",
		ProviderAllowed:    true,
		AccountVisibility:  []string{"acct-1"},
		SharedViewHash:     "test-shared-hash",
		BalanceCheckPassed: true,
	}
	envelope.ViewSnapshotHash = string(execution.ComputeViewSnapshotHash(viewSnapshot))

	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Test approval request for payment.",
		300,
		idGen,
	)
	// v9.13: BuildApprovalBundle now copies ViewSnapshotHash from envelope automatically

	return envelope, bundle
}
