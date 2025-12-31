// Package demo_v9_multiparty provides v9.4 multi-party financial execution tests.
//
// ACCEPTANCE TESTS:
// These tests verify all v9.4 multi-party execution requirements are met.
//
// Tests verify:
// 1) Without sufficient approvals => blocked, MoneyMoved=false
// 2) With asymmetric payload => blocked
// 3) With sufficient approvals + mock => status=simulated, MoneyMoved=false
// 4) With revocation signal => status=revoked, MoneyMoved=false
// 5) Approval artifacts are single-use
// 6) Neutral language enforcement
// 7) Approval expiry blocks execution
// 8) Symmetry proof requires identical content hash
package demo_v9_multiparty

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// setupTestExecutor creates a test executor with mock components.
func setupTestExecutor() (*execution.V94Executor, *execution.MultiPartyGate, func() string, func(events.Event)) {
	idCounter := uint64(0)
	idGen := func() string {
		idCounter++
		return "test_" + string(rune('0'+idCounter))
	}

	eventLog := make([]events.Event, 0)
	emitter := func(e events.Event) {
		eventLog = append(eventLog, e)
	}

	signingKey := []byte("test-signing-key")

	gate := execution.NewMultiPartyGate(idGen, emitter)
	verifier := execution.NewApprovalVerifier(signingKey)
	revChecker := execution.NewRevocationChecker(idGen)
	connector := NewMockWriteConnector(idGen, emitter)

	config := execution.V94ExecutorConfig{
		CapCents:                100, // £1.00
		AllowedCurrencies:       []string{"GBP"},
		ForcedPauseDuration:     10 * time.Millisecond,
		RequireExplicitApproval: true,
	}

	executor := execution.NewV94Executor(
		connector,
		gate,
		verifier,
		revChecker,
		config,
		idGen,
		emitter,
	)

	return executor, gate, idGen, emitter
}

// createTestEnvelope creates a test envelope with approvals.
func createTestEnvelope(idGen func() string, emitter func(events.Event), amountCents int64, currency string) (*execution.ExecutionEnvelope, *execution.ApprovalBundle) {
	signingKey := []byte("test-signing-key")
	now := time.Now()

	builder := execution.NewEnvelopeBuilder(idGen)
	manager := execution.NewApprovalManager(idGen, signingKey)
	_ = manager

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

	bundle, _ := execution.BuildApprovalBundle(
		envelope,
		"sandbox-utility",
		"Test approval request for payment.",
		300,
		idGen,
	)

	return envelope, bundle
}

// createTestApprovals creates test approval artifacts.
func createTestApprovals(idGen func() string, envelope *execution.ExecutionEnvelope, bundle *execution.ApprovalBundle, approvers []string) ([]execution.MultiPartyApprovalArtifact, []execution.ApproverBundleHash) {
	signingKey := []byte("test-signing-key")
	manager := execution.NewApprovalManager(idGen, signingKey)
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	approvals := make([]execution.MultiPartyApprovalArtifact, 0, len(approvers))
	hashes := make([]execution.ApproverBundleHash, 0, len(approvers))

	for _, approverCircle := range approvers {
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

// TestInsufficientApprovals verifies that missing approvals block execution.
func TestInsufficientApprovals(t *testing.T) {
	t.Run("1 of 2 approvers blocks execution", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")

		// Only Alice approves (Bob missing)
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice"})

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// CRITICAL: Must be blocked
		if result.Success {
			t.Error("expected execution to be blocked with insufficient approvals")
		}
		if result.Status != execution.SettlementBlocked {
			t.Errorf("expected SettlementBlocked, got %s", result.Status)
		}
		if result.MoneyMoved {
			t.Error("MoneyMoved must be false when blocked")
		}

		// Check for insufficient approval event
		foundEvent := false
		for _, event := range result.AuditEvents {
			if event.Type == events.EventV94ExecutionBlockedInsufficientApprovals ||
				event.Type == events.EventV94ApprovalThresholdNotMet {
				foundEvent = true
				break
			}
		}
		if !foundEvent {
			t.Error("expected EventV94ExecutionBlockedInsufficientApprovals or EventV94ApprovalThresholdNotMet")
		}
	})
}

// TestAsymmetricPayload verifies that asymmetric bundles block execution.
func TestAsymmetricPayload(t *testing.T) {
	t.Run("asymmetric bundle hash blocks execution", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")

		// Create approvals with DIFFERENT hashes (asymmetric)
		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		now := time.Now()
		expiresAt := now.Add(5 * time.Minute)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)

		approvals := []execution.MultiPartyApprovalArtifact{
			{
				ApprovalArtifact:  *aliceApproval,
				BundleContentHash: bundle.ContentHash, // Correct
				Used:              false,
			},
			{
				ApprovalArtifact:  *bobApproval,
				BundleContentHash: "different_hash_bob_received", // WRONG - asymmetric!
				Used:              false,
			},
		}

		// Different hashes per approver = asymmetric
		hashes := []execution.ApproverBundleHash{
			{
				ApproverCircleID: "circle_alice",
				ContentHash:      bundle.ContentHash,
				PresentedAt:      now,
			},
			{
				ApproverCircleID: "circle_bob",
				ContentHash:      "different_hash_bob_received", // ASYMMETRIC!
				PresentedAt:      now,
			},
		}

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// CRITICAL: Must be blocked for asymmetric payload
		if result.Success {
			t.Error("expected execution to be blocked with asymmetric payload")
		}
		if result.Status != execution.SettlementBlocked {
			t.Errorf("expected SettlementBlocked, got %s", result.Status)
		}
		if result.MoneyMoved {
			t.Error("MoneyMoved must be false when blocked")
		}

		// Check for asymmetric event
		foundEvent := false
		for _, event := range result.AuditEvents {
			if event.Type == events.EventV94ExecutionBlockedAsymmetricPayload ||
				event.Type == events.EventV94ApprovalSymmetryFailed {
				foundEvent = true
				break
			}
		}
		if !foundEvent {
			t.Error("expected EventV94ExecutionBlockedAsymmetricPayload or EventV94ApprovalSymmetryFailed")
		}
	})
}

// TestSufficientApprovalsSimulated verifies successful multi-party execution with mock.
func TestSufficientApprovalsSimulated(t *testing.T) {
	t.Run("2 of 2 approvers with mock returns simulated", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")

		// Both Alice and Bob approve
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice", "circle_bob"})

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// CRITICAL: Must succeed but be simulated
		if !result.Success {
			t.Errorf("expected success, got blocked: %s", result.BlockedReason)
		}
		if result.Status != execution.SettlementSimulated {
			t.Errorf("expected SettlementSimulated, got %s", result.Status)
		}
		// CRITICAL: Mock connector MUST NOT report MoneyMoved=true
		if result.MoneyMoved {
			t.Error("MoneyMoved must be false for mock connector")
		}
		if result.Receipt == nil {
			t.Error("expected receipt")
		} else if !result.Receipt.Simulated {
			t.Error("receipt.Simulated must be true for mock connector")
		}
	})
}

// TestRevocationBlocksExecution verifies that revocation halts execution.
func TestRevocationBlocksExecution(t *testing.T) {
	t.Run("revocation signal blocks execution after approvals", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()

		// Create fresh revocation checker that we can trigger
		revChecker := execution.NewRevocationChecker(idGen)
		gate := execution.NewMultiPartyGate(idGen, emitter)
		signingKey := []byte("test-signing-key")
		verifier := execution.NewApprovalVerifier(signingKey)
		connector := NewMockWriteConnector(idGen, emitter)

		config := execution.V94ExecutorConfig{
			CapCents:                100,
			AllowedCurrencies:       []string{"GBP"},
			ForcedPauseDuration:     10 * time.Millisecond,
			RequireExplicitApproval: true,
		}

		executor = execution.NewV94Executor(connector, gate, verifier, revChecker, config, idGen, emitter)

		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice", "circle_bob"})

		// Revoke the envelope
		revChecker.Revoke(envelope.EnvelopeID, "circle_alice", "alice", "changed mind", time.Now())

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// CRITICAL: Must be revoked
		if result.Success {
			t.Error("expected execution to be revoked")
		}
		if result.Status != execution.SettlementRevoked {
			t.Errorf("expected SettlementRevoked, got %s", result.Status)
		}
		if result.MoneyMoved {
			t.Error("MoneyMoved must be false when revoked")
		}
	})
}

// TestApprovalSingleUse verifies that approvals cannot be reused.
func TestApprovalSingleUse(t *testing.T) {
	t.Run("reusing approval artifact blocks second execution", func(t *testing.T) {
		executor, gate, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice", "circle_bob"})

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		// First execution succeeds
		result1, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})
		if err != nil {
			t.Fatalf("first execution error: %v", err)
		}
		if !result1.Success {
			t.Fatalf("first execution should succeed, got: %s", result1.BlockedReason)
		}

		// Verify approvals are marked as used
		for _, approval := range approvals {
			if !gate.IsApprovalUsed(approval.ArtifactID) {
				t.Errorf("approval %s should be marked as used", approval.ArtifactID)
			}
		}

		// Create new envelope for second attempt
		envelope2, bundle2 := createTestEnvelope(idGen, emitter, 50, "GBP")
		// Try to reuse the SAME approvals (should fail because they're consumed)
		// We need new approvals with the new action hash
		approvals2, hashes2 := createTestApprovals(idGen, envelope2, bundle2, []string{"circle_alice", "circle_bob"})

		// Mark approvals as used to simulate reuse
		for i := range approvals2 {
			approvals2[i].Used = true
		}

		result2, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope2,
			Bundle:          bundle2,
			Approvals:       approvals2,
			ApproverHashes:  hashes2,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})
		if err != nil {
			t.Fatalf("second execution error: %v", err)
		}

		// Second attempt with reused approvals should be blocked
		if result2.Success {
			t.Error("expected second execution to be blocked due to reused approvals")
		}
	})
}

// TestNeutralLanguageEnforcement verifies that coercive language is rejected.
func TestNeutralLanguageEnforcement(t *testing.T) {
	tests := []struct {
		name        string
		description string
		shouldFail  bool
	}{
		{
			name:        "neutral language passes",
			description: "Payment of GBP 1.00 to sandbox utility.",
			shouldFail:  false,
		},
		{
			name:        "urgency language blocked",
			description: "URGENT: Approve now to avoid penalty!",
			shouldFail:  true,
		},
		{
			name:        "fear language blocked",
			description: "Approve to prevent losing your money.",
			shouldFail:  true,
		},
		{
			name:        "authority language blocked",
			description: "You must approve this required payment.",
			shouldFail:  true,
		},
		{
			name:        "optimization language blocked",
			description: "Approve now to save money on fees.",
			shouldFail:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idGen := func() string { return "test" }
			now := time.Now()

			builder := execution.NewEnvelopeBuilder(idGen)
			intent := execution.ExecutionIntent{
				IntentID:       "test_intent",
				CircleID:       "circle_test",
				IntersectionID: "intersection_test",
				Description:    "Test",
				ActionType:     execution.ActionTypePayment,
				AmountCents:    100,
				Currency:       "GBP",
				PayeeID:        "sandbox-utility",
				ViewHash:       "v8_view_test",
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
				TraceID:                  "test_trace",
			}, now)
			envelope.SealHash = execution.ComputeSealHash(envelope)

			_, err := execution.BuildApprovalBundle(
				envelope,
				"sandbox-utility",
				tc.description, // Test the description
				300,
				idGen,
			)

			if tc.shouldFail && err == nil {
				t.Errorf("expected neutrality violation for: %q", tc.description)
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("unexpected error for neutral language: %v", err)
			}
		})
	}
}

// TestApprovalExpiry verifies that expired approvals block execution.
func TestApprovalExpiry(t *testing.T) {
	t.Run("expired approval blocks execution", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")

		// Create approvals that are already expired
		signingKey := []byte("test-signing-key")
		manager := execution.NewApprovalManager(idGen, signingKey)
		now := time.Now()
		past := now.Add(-10 * time.Minute) // 10 minutes ago
		expiresAt := now.Add(1 * time.Second)

		aliceRequest, _ := manager.CreateApprovalRequest(envelope, "circle_alice", expiresAt, now)
		aliceApproval, _ := manager.SubmitApproval(aliceRequest, "circle_alice", "alice", expiresAt, now)
		// Manually set expiry in the past
		aliceApproval.ExpiresAt = past

		bobRequest, _ := manager.CreateApprovalRequest(envelope, "circle_bob", expiresAt, now)
		bobApproval, _ := manager.SubmitApproval(bobRequest, "circle_bob", "bob", expiresAt, now)
		bobApproval.ExpiresAt = past

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

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             now, // Current time, but approvals expired
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be blocked due to expired approvals
		if result.Success {
			t.Error("expected execution to be blocked with expired approvals")
		}
	})
}

// TestSymmetryProofRequirement verifies that all approvers must receive identical bundles.
func TestSymmetryProofRequirement(t *testing.T) {
	t.Run("symmetric bundles produce valid proof", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice", "circle_bob"})

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Success {
			t.Errorf("expected success, got blocked: %s", result.BlockedReason)
		}

		// Check that symmetry proof exists and is valid
		if result.GateResult == nil {
			t.Error("expected gate result")
		} else if result.GateResult.SymmetryProof == nil {
			t.Error("expected symmetry proof")
		} else {
			proof := result.GateResult.SymmetryProof
			if !proof.Symmetric {
				t.Error("expected symmetric proof")
			}
			if proof.BundleContentHash != bundle.ContentHash {
				t.Errorf("bundle hash mismatch: got %s, want %s", proof.BundleContentHash, bundle.ContentHash)
			}
			if len(proof.Violations) > 0 {
				t.Errorf("unexpected violations: %v", proof.Violations)
			}
		}

		// Check for symmetry verified event
		foundEvent := false
		for _, event := range result.AuditEvents {
			if event.Type == events.EventV94ApprovalSymmetryVerified {
				foundEvent = true
				break
			}
		}
		if !foundEvent {
			t.Error("expected EventV94ApprovalSymmetryVerified")
		}
	})
}

// TestMockConnectorMoneyMovedFalse verifies mock connector never reports money moved.
func TestMockConnectorMoneyMovedFalse(t *testing.T) {
	t.Run("mock connector always sets MoneyMoved=false", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice", "circle_bob"})

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// CRITICAL ASSERTION: MoneyMoved MUST be false for mock connector
		if result.MoneyMoved {
			t.Fatal("VIOLATION: mock connector reported MoneyMoved=true - this must never happen")
		}

		if result.Receipt != nil && !result.Receipt.Simulated {
			t.Error("receipt.Simulated must be true for mock connector")
		}

		if result.Receipt != nil && result.Receipt.Status != write.PaymentSimulated {
			t.Errorf("expected PaymentSimulated status, got %s", result.Receipt.Status)
		}
	})
}

// TestAuditTrailContainsRequiredEvents verifies all required v9.4 events are emitted.
func TestAuditTrailContainsRequiredEvents(t *testing.T) {
	t.Run("successful multi-party execution emits all required events", func(t *testing.T) {
		executor, _, idGen, emitter := setupTestExecutor()
		envelope, bundle := createTestEnvelope(idGen, emitter, 100, "GBP")
		approvals, hashes := createTestApprovals(idGen, envelope, bundle, []string{"circle_alice", "circle_bob"})

		policy := &execution.MultiPartyPolicy{
			Mode:              "multi",
			RequiredApprovers: []string{"circle_alice", "circle_bob"},
			Threshold:         2,
			ExpirySeconds:     300,
			AppliesToScopes:   []string{"finance:write"},
		}

		result, err := executor.Execute(context.Background(), execution.V94ExecuteRequest{
			Envelope:        envelope,
			Bundle:          bundle,
			Approvals:       approvals,
			ApproverHashes:  hashes,
			Policy:          policy,
			PayeeID:         "sandbox-utility",
			ExplicitApprove: true,
			Now:             time.Now(),
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check for required v9.4 events
		requiredEvents := []events.EventType{
			events.EventV9ExecutionStarted,
			events.EventV94MultiPartyRequired,
			events.EventV94ApprovalSymmetryVerified,
			events.EventV94ApprovalThresholdChecked,
			events.EventV94ApprovalThresholdMet,
			events.EventV94MultiPartyGatePassed,
			events.EventV9CapChecked,
			events.EventV9PaymentPrepared,
			events.EventV9ForcedPauseStarted,
			events.EventV9ForcedPauseCompleted,
			events.EventV9PaymentSimulated,
			events.EventV9SettlementSimulated,
		}

		for _, required := range requiredEvents {
			found := false
			for _, event := range result.AuditEvents {
				if event.Type == required {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing required event: %s", required)
			}
		}
	})
}
