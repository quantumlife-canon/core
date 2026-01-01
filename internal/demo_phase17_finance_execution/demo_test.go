// Package demo_phase17_finance_execution demonstrates Phase 17 features.
//
// Phase 17: Finance Execution Boundary (Sandbox→Live) + Household Approvals
// - Finance Write Connector with mock and TrueLayer providers
// - Finance Execution Boundary (mirroring calendar/email patterns)
// - Draft support for finance payments (PaymentDraftContent)
// - Household/Intersection approvals integration
// - Policy/View snapshot binding (v9.12, v9.13)
// - Provider/Payee registry enforcement (v9.9, v9.10)
// - Caps and rate limiting (v9.11)
// - Idempotency and replay protection (v9.6)
//
// CRITICAL: Mock providers NEVER move real money.
// CRITICAL: All payments go through Finance Execution Boundary.
// CRITICAL: Deterministic - same inputs + clock = same results.
//
// Reference: docs/ADR/ADR-0033-phase17-finance-execution-boundary.md
package demo_phase17_finance_execution

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/internal/connectors/finance/write/payees"
	"quantumlife/internal/connectors/finance/write/providers/mock"
	"quantumlife/pkg/domain/approvalflow"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/intersection"
)

// TestPaymentDraftContent demonstrates payment draft creation and determinism.
func TestPaymentDraftContent(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create two identical payment drafts
	content1 := draft.PaymentDraftContent{
		PayeeID:                    "payee-utility-001",
		AmountCents:                5000, // £50.00
		Currency:                   "GBP",
		Description:                "Utility bill payment",
		ProviderHint:               "mock",
		IntersectionID:             "family-001",
		RequiresMultiPartyApproval: true,
		ApprovalThreshold:          2,
		RequiredApproverCircleIDs:  []string{"circle-satish", "circle-wife"},
	}

	content2 := draft.PaymentDraftContent{
		PayeeID:                    "payee-utility-001",
		AmountCents:                5000,
		Currency:                   "GBP",
		Description:                "Utility bill payment",
		ProviderHint:               "mock",
		IntersectionID:             "family-001",
		RequiresMultiPartyApproval: true,
		ApprovalThreshold:          2,
		RequiredApproverCircleIDs:  []string{"circle-wife", "circle-satish"}, // Different order
	}

	// Canonical strings should be identical (sorted approvers)
	if content1.CanonicalString() != content2.CanonicalString() {
		t.Errorf("canonical strings should match despite approver order")
	}
	t.Logf("Payment draft determinism verified")

	// Create full draft
	d := draft.Draft{
		DraftID:            "draft-payment-001",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-family"),
		IntersectionID:     identity.EntityID("family-001"),
		SourceObligationID: "obligation-utility",
		CreatedAt:          now,
		ExpiresAt:          expires,
		Status:             draft.StatusProposed,
		Content:            content1,
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
	}

	// Test content accessor
	paymentContent, ok := d.PaymentContent()
	if !ok {
		t.Fatal("should return payment content")
	}
	if paymentContent.AmountCents != 5000 {
		t.Errorf("expected 5000 cents, got %d", paymentContent.AmountCents)
	}
	if paymentContent.AmountFormatted() != "£50.00" {
		t.Errorf("expected £50.00, got %s", paymentContent.AmountFormatted())
	}

	t.Logf("Payment draft: payee=%s, amount=%s, multi_party=%t",
		paymentContent.PayeeID, paymentContent.AmountFormatted(), paymentContent.RequiresMultiPartyApproval)
}

// TestMockConnectorSimulation demonstrates mock provider (no real money).
func TestMockConnectorSimulation(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create mock connector with injected clock
	connector := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
		mock.WithConfig(write.WriteConfig{
			CapCents:          10000, // £100.00
			AllowedCurrencies: []string{"GBP"},
		}),
	)

	// Verify provider ID
	if connector.ProviderID() != "mock-write" {
		t.Errorf("expected mock-write, got %s", connector.ProviderID())
	}

	// Create a test envelope
	envelope := &write.ExecutionEnvelope{
		EnvelopeID: "env-001",
		SealHash:   "seal-hash-abc123",
		ActionHash: "action-hash-def456",
		ActionSpec: write.ActionSpec{
			Type:        "payment",
			AmountCents: 5000, // £50.00
			Currency:    "GBP",
			PayeeID:     string(payees.PayeeSandboxUtility),
			Description: "Test payment",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	// Create approval artifact
	approval := &write.ApprovalArtifact{
		ArtifactID:       "approval-001",
		ApproverCircleID: "circle-satish",
		ApproverID:       "person-satish",
		ActionHash:       envelope.ActionHash,
		ApprovedAt:       now,
		ExpiresAt:        now.Add(1 * time.Hour),
	}

	// Prepare
	ctx := context.Background()
	prepResult, err := connector.Prepare(ctx, write.PrepareRequest{
		Envelope: envelope,
		Approval: approval,
		PayeeID:  string(payees.PayeeSandboxUtility),
		Now:      now,
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if !prepResult.Valid {
		t.Fatalf("prepare should be valid: %s", prepResult.InvalidReason)
	}
	t.Logf("Prepare passed: %d validation checks", len(prepResult.ValidationDetails))

	// Execute
	receipt, err := connector.Execute(ctx, write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		PayeeID:        string(payees.PayeeSandboxUtility),
		IdempotencyKey: "idemp-key-001",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	// CRITICAL: Mock MUST report simulated, no real money
	if !receipt.Simulated {
		t.Fatal("CRITICAL: mock provider must report Simulated=true")
	}
	if receipt.Status != write.PaymentSimulated {
		t.Errorf("expected PaymentSimulated status, got %s", receipt.Status)
	}

	t.Logf("Mock execution complete:")
	t.Logf("  - Receipt ID: %s", receipt.ReceiptID)
	t.Logf("  - Provider Ref: %s", receipt.ProviderRef)
	t.Logf("  - Amount: %d %s", receipt.AmountCents, receipt.Currency)
	t.Logf("  - Simulated: %t (NO REAL MONEY MOVED)", receipt.Simulated)
}

// TestMockConnectorIdempotency demonstrates idempotent execution.
func TestMockConnectorIdempotency(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	connector := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
	)

	envelope := &write.ExecutionEnvelope{
		EnvelopeID: "env-idemp-001",
		SealHash:   "seal-hash-001",
		ActionHash: "action-hash-001",
		ActionSpec: write.ActionSpec{
			AmountCents: 2500,
			Currency:    "GBP",
			PayeeID:     string(payees.PayeeSandboxMerchant),
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	approval := &write.ApprovalArtifact{
		ArtifactID: "approval-idemp",
		ActionHash: envelope.ActionHash,
		ApprovedAt: now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	ctx := context.Background()
	idempotencyKey := "idemp-key-unique"

	// First execution
	receipt1, err := connector.Execute(ctx, write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		PayeeID:        string(payees.PayeeSandboxMerchant),
		IdempotencyKey: idempotencyKey,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}

	// Second execution with same key - should return same result
	receipt2, err := connector.Execute(ctx, write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		PayeeID:        string(payees.PayeeSandboxMerchant),
		IdempotencyKey: idempotencyKey,
		Now:            now.Add(5 * time.Minute), // Later time
	})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	// Same receipt should be returned
	if receipt1.ReceiptID != receipt2.ReceiptID {
		t.Errorf("idempotent executions should return same receipt: %s != %s",
			receipt1.ReceiptID, receipt2.ReceiptID)
	}
	if receipt1.ProviderRef != receipt2.ProviderRef {
		t.Errorf("provider refs should match: %s != %s",
			receipt1.ProviderRef, receipt2.ProviderRef)
	}

	t.Logf("Idempotency verified: same key returns same receipt")
	t.Logf("  - Receipt ID: %s", receipt1.ReceiptID)
}

// TestMockConnectorAbort demonstrates abort functionality.
func TestMockConnectorAbort(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	connector := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
	)

	envelopeID := "env-abort-001"
	ctx := context.Background()

	// Abort the envelope
	aborted, err := connector.Abort(ctx, envelopeID)
	if err != nil {
		t.Fatalf("abort failed: %v", err)
	}
	if !aborted {
		t.Error("abort should return true")
	}

	// Now try to execute - should fail
	envelope := &write.ExecutionEnvelope{
		EnvelopeID: envelopeID,
		SealHash:   "seal-hash-001",
		ActionHash: "action-hash-001",
		ActionSpec: write.ActionSpec{
			AmountCents: 1000,
			Currency:    "GBP",
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	approval := &write.ApprovalArtifact{
		ArtifactID: "approval-abort",
		ActionHash: envelope.ActionHash,
		ApprovedAt: now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	_, err = connector.Execute(ctx, write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		IdempotencyKey: "idemp-abort",
		Now:            now,
	})
	if err != write.ErrExecutionAborted {
		t.Errorf("expected ErrExecutionAborted, got %v", err)
	}

	t.Log("Abort functionality verified: execution blocked after abort")
}

// TestPayeeRegistryEnforcement demonstrates v9.10 payee registry.
func TestPayeeRegistryEnforcement(t *testing.T) {
	registry := payees.NewDefaultRegistry()

	// Sandbox payees should be allowed
	sandboxPayees := []payees.PayeeID{
		payees.PayeeSandboxUtility,
		payees.PayeeSandboxRent,
		payees.PayeeSandboxMerchant,
	}

	for _, payeeID := range sandboxPayees {
		err := registry.RequireAllowed(payeeID, "mock-write")
		if err != nil {
			t.Errorf("sandbox payee %s should be allowed: %v", payeeID, err)
		}
	}
	t.Logf("Sandbox payees verified: %d allowed", len(sandboxPayees))

	// Unknown payee should be blocked
	err := registry.RequireAllowed("unknown-free-text-payee", "mock-write")
	if err == nil {
		t.Error("unknown payee should be blocked")
	}
	t.Logf("Free-text payee blocked: %v", err)

	// Sandbox payees should be blocked for live provider
	err = registry.RequireAllowed(payees.PayeeSandboxUtility, "truelayer-live")
	if err == nil {
		t.Error("sandbox payee should be blocked for live provider")
	}
	t.Logf("Sandbox payee blocked for live: %v", err)
}

// TestHouseholdPaymentApproval demonstrates household approval flow for payments.
func TestHouseholdPaymentApproval(t *testing.T) {
	t.Log("=== Household Payment Approval Scenario ===")
	t.Log("Satish wants to pay £50 for utility bill from joint account.")
	t.Log("Intersection policy requires both Satish and Wife to approve.")

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create household intersection policy for payments
	policy := intersection.NewIntersectionPolicy("household-finance", "Household Finance", now)
	policy.AddMember("person-satish", intersection.RoleOwner, "Satish")
	policy.AddMember("person-wife", intersection.RoleSpouse, "Wife")
	policy.AddRequirement(intersection.ApprovalRequirement{
		ActionClass:   intersection.ActionFinancePayment, // Phase 17 action
		RequiredRoles: []intersection.MemberRole{intersection.RoleOwner, intersection.RoleSpouse},
		Threshold:     2,
		MaxAgeMinutes: 60,
	})

	t.Logf("Policy: %s", policy.Name)
	t.Logf("  - Members: %d (both must approve payments)", len(policy.Members))

	// Create payment draft
	paymentContent := draft.PaymentDraftContent{
		PayeeID:                    string(payees.PayeeSandboxUtility),
		AmountCents:                5000, // £50.00
		Currency:                   "GBP",
		Description:                "Utility bill - Jan 2025",
		IntersectionID:             "household-finance",
		RequiresMultiPartyApproval: true,
		ApprovalThreshold:          2,
		RequiredApproverCircleIDs:  []string{"circle-satish", "circle-wife"},
	}

	t.Log("")
	t.Logf("Payment Draft: %s for %s", paymentContent.PayeeID, paymentContent.AmountFormatted())

	// Create approval state
	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-payment-utility",
		"household-finance",
		intersection.ActionFinancePayment,
		approvers,
		2,  // Both must approve
		60, // 1 hour max age
		now,
	)

	// Satish approves
	satishApprovalTime := now.Add(5 * time.Minute)
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: satishApprovalTime,
		TokenID:   "token-satish-001",
		Reason:    "Approved - regular bill",
	})

	status := state.ComputeStatus(satishApprovalTime)
	t.Log("")
	t.Logf("Satish approved at %s", satishApprovalTime.Format("3:04 PM"))
	t.Logf("  - Status: %s (waiting for Wife)", status)

	if status != approvalflow.StatusPending {
		t.Errorf("expected pending, got %s", status)
	}

	// Wife approves
	wifeApprovalTime := now.Add(20 * time.Minute)
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-wife",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: wifeApprovalTime,
		TokenID:   "token-wife-001",
	})

	status = state.ComputeStatus(wifeApprovalTime)
	t.Log("")
	t.Logf("Wife approved at %s", wifeApprovalTime.Format("3:04 PM"))
	t.Logf("  - Status: %s (threshold met!)", status)

	if status != approvalflow.StatusApproved {
		t.Errorf("expected approved, got %s", status)
	}

	t.Log("")
	t.Log("=== Payment can now be executed via Finance Boundary ===")
}

// TestPaymentDraftRejection demonstrates rejection flow.
func TestPaymentDraftRejection(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	approvers := []approvalflow.ApproverRef{
		{PersonID: "person-satish", Role: intersection.RoleOwner},
		{PersonID: "person-wife", Role: intersection.RoleSpouse},
	}

	state := approvalflow.NewApprovalState(
		approvalflow.TargetTypeDraft,
		"draft-payment-questionable",
		"household-finance",
		intersection.ActionFinancePayment,
		approvers,
		2,
		60,
		now,
	)

	// Satish approves
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-satish",
		Decision:  approvalflow.DecisionApproved,
		Timestamp: now.Add(5 * time.Minute),
	})

	// Wife rejects
	state.RecordApproval(approvalflow.ApprovalRecord{
		PersonID:  "person-wife",
		Decision:  approvalflow.DecisionRejected,
		Timestamp: now.Add(10 * time.Minute),
		Reason:    "Not in budget this month",
	})

	status := state.ComputeStatus(now.Add(15 * time.Minute))
	if status != approvalflow.StatusRejected {
		t.Errorf("expected rejected, got %s", status)
	}

	t.Logf("Payment rejected by Wife: %s", status)
	t.Log("Payment will NOT be executed (any rejection blocks)")
}

// TestCapEnforcement demonstrates the £1.00 hard cap.
func TestCapEnforcement(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	connector := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
		mock.WithConfig(write.WriteConfig{
			CapCents:          100, // £1.00 hard cap
			AllowedCurrencies: []string{"GBP"},
		}),
	)

	// Try to pay more than cap
	envelope := &write.ExecutionEnvelope{
		EnvelopeID: "env-overcap",
		SealHash:   "seal-hash-001",
		ActionHash: "action-hash-001",
		ActionSpec: write.ActionSpec{
			AmountCents: 500, // £5.00 - exceeds cap
			Currency:    "GBP",
			PayeeID:     string(payees.PayeeSandboxUtility),
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	approval := &write.ApprovalArtifact{
		ArtifactID: "approval-overcap",
		ActionHash: envelope.ActionHash,
		ApprovedAt: now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	ctx := context.Background()
	prepResult, err := connector.Prepare(ctx, write.PrepareRequest{
		Envelope: envelope,
		Approval: approval,
		PayeeID:  string(payees.PayeeSandboxUtility),
		Now:      now,
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if prepResult.Valid {
		t.Error("prepare should fail for amount exceeding cap")
	}

	t.Logf("Cap enforcement verified: %s", prepResult.InvalidReason)
}

// TestDeterministicReceipts demonstrates deterministic receipt generation.
func TestDeterministicReceipts(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create two connectors with same clock
	connector1 := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
	)
	connector2 := mock.NewConnector(
		mock.WithClock(func() time.Time { return now }),
	)

	envelope := &write.ExecutionEnvelope{
		EnvelopeID: "env-determ",
		SealHash:   "seal-hash-determ",
		ActionHash: "action-hash-determ",
		ActionSpec: write.ActionSpec{
			AmountCents: 1000,
			Currency:    "GBP",
			PayeeID:     string(payees.PayeeSandboxMerchant),
		},
		Expiry:              now.Add(24 * time.Hour),
		RevocationWaived:    true,
		RevocationWindowEnd: now,
	}

	approval := &write.ApprovalArtifact{
		ArtifactID: "approval-determ",
		ActionHash: envelope.ActionHash,
		ApprovedAt: now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	ctx := context.Background()
	idempotencyKey := "idemp-determ-001"

	receipt1, _ := connector1.Execute(ctx, write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		PayeeID:        string(payees.PayeeSandboxMerchant),
		IdempotencyKey: idempotencyKey,
		Now:            now,
	})

	receipt2, _ := connector2.Execute(ctx, write.ExecuteRequest{
		Envelope:       envelope,
		Approval:       approval,
		PayeeID:        string(payees.PayeeSandboxMerchant),
		IdempotencyKey: idempotencyKey,
		Now:            now,
	})

	// Same inputs should produce same receipt IDs
	if receipt1.ReceiptID != receipt2.ReceiptID {
		t.Errorf("deterministic receipts should match: %s != %s",
			receipt1.ReceiptID, receipt2.ReceiptID)
	}

	t.Logf("Deterministic receipts verified: %s", receipt1.ReceiptID)
}

// TestFinanceActionClass demonstrates the finance action class.
func TestFinanceActionClass(t *testing.T) {
	// Verify ActionFinancePayment is defined
	action := intersection.ActionFinancePayment

	if action != "finance_payment" {
		t.Errorf("expected finance_payment, got %s", action)
	}

	t.Logf("Finance action class defined: %s", action)
}
