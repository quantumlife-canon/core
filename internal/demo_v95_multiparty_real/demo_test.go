// Package demo_v95_multiparty_real provides acceptance tests for v9.5 multi-party execution.
//
// These tests verify:
// 1) Presentation required - approval without presentation is blocked
// 2) Presentation hash mismatch - approval references different bundle
// 3) Symmetry mismatch - approvers received different bundles
// 4) Insufficient approvals - threshold not met
// 5) Expired approval - approval has expired
// 6) Revocation during forced pause - aborts before provider call
// 7) No retries - second attempt requires new approvals
// 8) Exactly one trace finalization per attempt
// 9) Provider selection - mock when not configured
// 10) Cap enforced at 100 pence
// 11) Predefined payees only
// 12) Mock connector always reports MoneyMoved=false
// 13) Sandbox enforcement
// 14) Audit trail contains required events
// 15) Presentation expiry blocks approval
//
// Reference: docs/ACCEPTANCE_TESTS_V9_EXECUTION.md
package demo_v95_multiparty_real

import (
	"context"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// setupTestExecutor creates a test executor with all components.
func setupTestExecutor() (*execution.V95Executor, *execution.PresentationStore, *execution.RevocationChecker, func() string, func(events.Event)) {
	counter := 0
	idGen := func() string {
		counter++
		return "test_id_" + string(rune('0'+counter))
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
	mockConnector := NewMockWriteConnector(idGen, emitter)

	config := execution.DefaultV95ExecutorConfig()
	config.ForcedPauseDuration = 100 * time.Millisecond // Short for tests
	config.TrueLayerConfigured = false                  // Use mock

	executor := execution.NewV95Executor(
		nil, // No TrueLayer for tests
		mockConnector,
		presentationGate,
		multiPartyGate,
		approvalVerifier,
		revocationChecker,
		config,
		idGen,
		emitter,
	)

	return executor, presentationStore, revocationChecker, idGen, emitter
}

// createTestEnvelope creates a test envelope with bundle.
func createTestEnvelope(idGen func() string, emitter func(events.Event), amountCents int64, currency string) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
	return createTestEnvelopeWithCap(idGen, emitter, amountCents, currency, 100)
}

// createTestEnvelopeWithCap creates a test envelope with bundle and custom cap.
func createTestEnvelopeWithCap(idGen func() string, emitter func(events.Event), amountCents int64, currency string, envelopeCap int64) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
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
		AmountCap:                envelopeCap,
		FrequencyCap:             1,
		DurationCap:              time.Hour,
		TraceID:                  idGen(),
	}, now)
	envelope.SealHash = execution.ComputeSealHash(envelope)

	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Test approval request for payment.",
		300,
		idGen,
	)

	return envelope, bundle
}

// createTestApprovals creates test approval artifacts with presentations.
func createTestApprovalsWithPresentation(
	idGen func() string,
	envelope *execution.ExecutionEnvelope,
	bundle *execution.ApprovalBundle,
	approvers []string,
	presentationStore *execution.PresentationStore,
	traceID string,
) ([]execution.MultiPartyApprovalArtifact, []execution.ApproverBundleHash) {
	signingKey := []byte("test-signing-key")
	manager := execution.NewApprovalManager(idGen, signingKey)
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	approvals := make([]execution.MultiPartyApprovalArtifact, 0, len(approvers))
	hashes := make([]execution.ApproverBundleHash, 0, len(approvers))

	for _, approverCircle := range approvers {
		// Record presentation first
		presentationStore.RecordPresentation(approverCircle, approverCircle+"_id", bundle, envelope, traceID, 5*time.Minute, now)

		// Then create approval
		request, _ := manager.CreateApprovalRequest(envelope, approverCircle, expiresAt, now)
		approval, _ := manager.SubmitApproval(request, approverCircle, approverCircle+"_id", expiresAt, now)
		approvals = append(approvals, execution.MultiPartyApprovalArtifact{
			ApprovalArtifact:  *approval,
			BundleContentHash: bundle.ContentHash,
			Used:              false,
		})
		hashes = append(hashes, execution.ApproverBundleHash{
			ApproverCircleID: approverCircle,
			ContentHash:      bundle.ContentHash,
			PresentedAt:      now,
		})
	}

	return approvals, hashes
}

// Test 1: Presentation required - missing presentation blocks execution
func TestPresentationRequired(t *testing.T) {
	t.Run("missing presentation blocks execution", func(t *testing.T) {
		executor, _, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")

		// Create approvals WITHOUT recording presentations
		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		now := time.Now()
		expiresAt := now.Add(5 * time.Minute)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)

		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)

		approvals := []execution.MultiPartyApprovalArtifact{
			{ApprovalArtifact: *aliceApproval, BundleContentHash: bundle.ContentHash},
			{ApprovalArtifact: *bobApproval, BundleContentHash: bundle.ContentHash},
		}
		hashes := []execution.ApproverBundleHash{
			{ApproverCircleID: "circle_alice", ContentHash: bundle.ContentHash, PresentedAt: now},
			{ApproverCircleID: "circle_bob", ContentHash: bundle.ContentHash, PresentedAt: now},
		}

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         idGen(),
			Now:             now,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Success {
			t.Error("expected execution to be blocked due to missing presentation")
		}

		if !strings.Contains(result.BlockedReason, "presentation") {
			t.Errorf("expected blocked reason to mention presentation, got: %s", result.BlockedReason)
		}

		if result.MoneyMoved {
			t.Error("money should not move when presentation is missing")
		}
	})
}

// Test 2: Presentation hash mismatch
func TestPresentationHashMismatch(t *testing.T) {
	t.Run("different bundle hash blocks execution", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		traceID := idGen()
		now := time.Now()

		// Record presentation with correct hash
		presentationStore.RecordPresentation("circle_alice", "alice", bundle, envelope, traceID, 5*time.Minute, now)
		presentationStore.RecordPresentation("circle_bob", "bob", bundle, envelope, traceID, 5*time.Minute, now)

		// Create approvals with WRONG bundle hash
		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		expiresAt := now.Add(5 * time.Minute)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)

		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)

		wrongHash := "wrong_hash_that_does_not_match_presentation"
		approvals := []execution.MultiPartyApprovalArtifact{
			{ApprovalArtifact: *aliceApproval, BundleContentHash: wrongHash},
			{ApprovalArtifact: *bobApproval, BundleContentHash: wrongHash},
		}
		hashes := []execution.ApproverBundleHash{
			{ApproverCircleID: "circle_alice", ContentHash: wrongHash, PresentedAt: now},
			{ApproverCircleID: "circle_bob", ContentHash: wrongHash, PresentedAt: now},
		}

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             now,
		})

		if result.Success {
			t.Error("expected execution to be blocked due to hash mismatch")
		}

		if result.MoneyMoved {
			t.Error("money should not move on hash mismatch")
		}
	})
}

// Test 3: Symmetry mismatch
func TestSymmetryMismatch(t *testing.T) {
	t.Run("asymmetric bundles block execution", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		traceID := idGen()
		now := time.Now()

		// Record presentations
		presentationStore.RecordPresentation("circle_alice", "alice", bundle, envelope, traceID, 5*time.Minute, now)
		presentationStore.RecordPresentation("circle_bob", "bob", bundle, envelope, traceID, 5*time.Minute, now)

		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		expiresAt := now.Add(5 * time.Minute)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)

		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)

		// Different hashes for each approver (asymmetric)
		approvals := []execution.MultiPartyApprovalArtifact{
			{ApprovalArtifact: *aliceApproval, BundleContentHash: bundle.ContentHash},
			{ApprovalArtifact: *bobApproval, BundleContentHash: bundle.ContentHash},
		}
		hashes := []execution.ApproverBundleHash{
			{ApproverCircleID: "circle_alice", ContentHash: bundle.ContentHash, PresentedAt: now},
			{ApproverCircleID: "circle_bob", ContentHash: "different_hash", PresentedAt: now}, // ASYMMETRIC
		}

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             now,
		})

		if result.Success {
			t.Error("expected execution to be blocked due to symmetry violation")
		}

		if !strings.Contains(result.BlockedReason, "asymmetric") {
			t.Errorf("expected blocked reason to mention asymmetric, got: %s", result.BlockedReason)
		}
	})
}

// Test 4: Insufficient approvals
func TestInsufficientApprovals(t *testing.T) {
	t.Run("1 of 2 approvers blocks execution", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		traceID := idGen()

		// Only one approver
		approvals, hashes := createTestApprovalsWithPresentation(idGen, envelope, bundle, []string{"circle_alice"}, presentationStore, traceID)

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if result.Success {
			t.Error("expected execution to be blocked due to insufficient approvals")
		}

		if !strings.Contains(result.BlockedReason, "insufficient") || !strings.Contains(result.BlockedReason, "1 of 2") {
			t.Errorf("expected blocked reason to mention insufficient approvals, got: %s", result.BlockedReason)
		}
	})
}

// Test 5: Expired approval
func TestExpiredApproval(t *testing.T) {
	t.Run("expired approval blocks execution", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		traceID := idGen()
		now := time.Now()

		// Record presentations
		presentationStore.RecordPresentation("circle_alice", "alice", bundle, envelope, traceID, 5*time.Minute, now)
		presentationStore.RecordPresentation("circle_bob", "bob", bundle, envelope, traceID, 5*time.Minute, now)

		// Create expired approvals
		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		past := now.Add(-10 * time.Minute)
		expiresAt := now.Add(1 * time.Second)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
		aliceApproval.ExpiresAt = past // Force expired

		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)
		bobApproval.ExpiresAt = past // Force expired

		approvals := []execution.MultiPartyApprovalArtifact{
			{ApprovalArtifact: *aliceApproval, BundleContentHash: bundle.ContentHash},
			{ApprovalArtifact: *bobApproval, BundleContentHash: bundle.ContentHash},
		}
		hashes := []execution.ApproverBundleHash{
			{ApproverCircleID: "circle_alice", ContentHash: bundle.ContentHash, PresentedAt: now},
			{ApproverCircleID: "circle_bob", ContentHash: bundle.ContentHash, PresentedAt: now},
		}

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             now,
		})

		if result.Success {
			t.Error("expected execution to be blocked due to expired approval")
		}
	})
}

// Test 6: Revocation during forced pause
func TestRevocationDuringPause(t *testing.T) {
	t.Run("revocation during pause aborts before provider", func(t *testing.T) {
		executor, presentationStore, revocationChecker, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		// Start execution in goroutine
		var result *execution.V95ExecuteResult
		done := make(chan struct{})

		go func() {
			result, _ = executor.Execute(context.Background(), execution.V95ExecuteRequest{
				Envelope:        envelope,
				Bundle:          bundle,
				Approvals:       approvals,
				ApproverHashes:  hashes,
				Policy:          policy,
				PayeeID:         "sandbox-utility",
				ExplicitApprove: true,
				TraceID:         traceID,
				Now:             time.Now(),
			})
			close(done)
		}()

		// Wait a bit then revoke
		time.Sleep(50 * time.Millisecond)
		revocationChecker.Revoke(envelope.EnvelopeID, "circle_bob", "bob", "changed mind", time.Now())

		<-done

		if result.Success {
			t.Error("expected execution to be blocked due to revocation during pause")
		}

		if result.MoneyMoved {
			t.Error("money should not move when revoked during pause")
		}

		if result.Status != execution.SettlementRevoked {
			t.Errorf("expected status revoked, got %s", result.Status)
		}
	})
}

// Test 7: No retries - second attempt requires new approvals
func TestNoRetries(t *testing.T) {
	t.Run("single-use approvals block reuse", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		// First execution should succeed
		result1, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if !result1.Success {
			t.Fatalf("first execution should succeed, got: %s", result1.BlockedReason)
		}

		// Second execution with same approvals should fail
		result2, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         idGen(),
			Now:             time.Now(),
		})

		if result2.Success {
			t.Error("second execution with same approvals should fail")
		}
	})
}

// Test 8: Exactly one trace finalization per attempt
func TestSingleTraceFinalization(t *testing.T) {
	t.Run("exactly one attempt finalized event per execution", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		// Count attempt finalized events
		finalizedCount := 0
		for _, e := range result.AuditEvents {
			if e.Type == events.EventV95AttemptFinalized {
				finalizedCount++
			}
		}

		if finalizedCount != 1 {
			t.Errorf("expected exactly 1 attempt finalized event, got %d", finalizedCount)
		}
	})
}

// Test 9: Provider selection - mock when not configured
func TestProviderSelectionMock(t *testing.T) {
	t.Run("uses mock when TrueLayer not configured", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if result.ProviderUsed != "mock-write" {
			t.Errorf("expected provider mock-write, got %s", result.ProviderUsed)
		}
	})
}

// Test 10: Cap enforced at 100 pence
func TestCapEnforced(t *testing.T) {
	t.Run("amount exceeding cap is blocked", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		// Use higher envelope cap (200) to allow envelope creation,
		// but executor enforces 100 cap
		envelope, bundle := createTestEnvelopeWithCap(idGen, emitter, 150, "GBP", 200)
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

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if result.Success {
			t.Error("expected execution to be blocked due to cap exceeded")
		}

		if !strings.Contains(result.BlockedReason, "cap") {
			t.Errorf("expected blocked reason to mention cap, got: %s", result.BlockedReason)
		}
	})
}

// Test 11: Predefined payees only
func TestPredefinedPayeesOnly(t *testing.T) {
	t.Run("unknown payee is rejected", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "unknown-payee-not-in-registry",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if result.Success {
			t.Error("expected execution to be blocked due to unknown payee")
		}

		if !strings.Contains(result.BlockedReason, "payee") {
			t.Errorf("expected blocked reason to mention payee, got: %s", result.BlockedReason)
		}
	})
}

// Test 12: Mock connector always reports MoneyMoved=false
func TestMockConnectorMoneyMoved(t *testing.T) {
	t.Run("mock connector sets MoneyMoved=false", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if result.MoneyMoved {
			t.Error("mock connector should never report MoneyMoved=true")
		}

		if result.Receipt != nil && !result.Receipt.Simulated {
			t.Error("mock connector receipt should be marked as simulated")
		}
	})
}

// Test 13: Sandbox enforcement
func TestSandboxEnforcement(t *testing.T) {
	t.Run("executor enforces sandbox mode", func(t *testing.T) {
		executor, _, _, _, _ := setupTestExecutor()

		if !executor.IsSandboxEnforced() {
			t.Error("executor should enforce sandbox mode")
		}
	})
}

// Test 14: Audit trail contains required events
func TestAuditTrailContainsRequiredEvents(t *testing.T) {
	t.Run("successful execution has required audit events", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		// Check for required events
		requiredEvents := []events.EventType{
			events.EventV95AttemptStarted,
			events.EventV95ExecutionProviderSelected,
			events.EventV9ExecutionStarted,
			events.EventV9PaymentPrepared,
			events.EventV9ForcedPauseStarted,
			events.EventV9ForcedPauseCompleted,
			events.EventV95AttemptFinalized,
		}

		eventTypeSet := make(map[events.EventType]bool)
		for _, e := range result.AuditEvents {
			eventTypeSet[e.Type] = true
		}

		for _, required := range requiredEvents {
			if !eventTypeSet[required] {
				t.Errorf("missing required audit event: %s", required)
			}
		}
	})
}

// Test 15: Presentation expiry
func TestPresentationExpiry(t *testing.T) {
	t.Run("expired presentation blocks approval", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		traceID := idGen()
		past := time.Now().Add(-10 * time.Minute)

		// Record presentation that has already expired
		presentationStore.RecordPresentation("circle_alice", "alice", bundle, envelope, traceID, -5*time.Minute, past)
		presentationStore.RecordPresentation("circle_bob", "bob", bundle, envelope, traceID, -5*time.Minute, past)

		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		now := time.Now()
		expiresAt := now.Add(5 * time.Minute)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)

		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)

		approvals := []execution.MultiPartyApprovalArtifact{
			{ApprovalArtifact: *aliceApproval, BundleContentHash: bundle.ContentHash},
			{ApprovalArtifact: *bobApproval, BundleContentHash: bundle.ContentHash},
		}
		hashes := []execution.ApproverBundleHash{
			{ApproverCircleID: "circle_alice", ContentHash: bundle.ContentHash, PresentedAt: now},
			{ApproverCircleID: "circle_bob", ContentHash: bundle.ContentHash, PresentedAt: now},
		}

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, _ := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             now,
		})

		if result.Success {
			t.Error("expected execution to be blocked due to expired presentation")
		}
	})
}

// Test 16: Successful simulated execution
func TestSuccessfulSimulatedExecution(t *testing.T) {
	t.Run("successful execution with mock provider", func(t *testing.T) {
		executor, presentationStore, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
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

		result, err := executor.Execute(context.Background(), execution.V95ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			TraceID:         traceID,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Success {
			t.Errorf("expected success, got blocked: %s", result.BlockedReason)
		}

		if result.MoneyMoved {
			t.Error("mock should not move money")
		}

		if result.Receipt == nil {
			t.Error("expected receipt")
		}

		if result.Receipt != nil && result.Receipt.Status != write.PaymentSimulated {
			t.Errorf("expected simulated status, got %s", result.Receipt.Status)
		}
	})
}
