// Package execrouter tests for payment draft routing.
//
// Phase 17b: Payment draft → ActionFinancePayment routing tests.
package execrouter

import (
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/execintent"
	"quantumlife/pkg/domain/identity"
)

// TestPaymentDraftRouting verifies that payment drafts route to ActionFinancePayment.
func TestPaymentDraftRouting(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	router := NewRouter(clk, nil)

	// Create approved payment draft
	d := &draft.Draft{
		DraftID:            "draft-payment-001",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-satish"),
		Status:             draft.StatusApproved,
		CreatedAt:          now.Add(-1 * time.Hour),
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		Content: draft.PaymentDraftContent{
			PayeeID:     "sandbox-utility",
			AmountCents: 5000, // £50.00
			Currency:    "GBP",
			Description: "Utility bill payment",
		},
	}

	intent, err := router.BuildIntentFromDraft(d)
	if err != nil {
		t.Fatalf("BuildIntentFromDraft failed: %v", err)
	}

	// Verify action class
	if intent.Action != execintent.ActionFinancePayment {
		t.Errorf("expected ActionFinancePayment, got %s", intent.Action)
	}

	// Verify finance fields
	if intent.FinancePayeeID != "sandbox-utility" {
		t.Errorf("expected payee sandbox-utility, got %s", intent.FinancePayeeID)
	}
	if intent.FinanceAmountCents != 5000 {
		t.Errorf("expected 5000 cents, got %d", intent.FinanceAmountCents)
	}
	if intent.FinanceCurrency != "GBP" {
		t.Errorf("expected GBP, got %s", intent.FinanceCurrency)
	}

	// Verify snapshot hashes are preserved
	if intent.PolicySnapshotHash != "policy-hash-abc123" {
		t.Errorf("policy hash mismatch")
	}
	if intent.ViewSnapshotHash != "view-hash-def456" {
		t.Errorf("view hash mismatch")
	}

	t.Logf("Payment draft routed to ActionFinancePayment: intent=%s", intent.IntentID)
}

// TestPaymentDraftMissingHashBlock verifies that missing hashes block routing.
func TestPaymentDraftMissingHashBlock(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	router := NewRouter(clk, nil)

	// Test missing PolicySnapshotHash
	d := &draft.Draft{
		DraftID:            "draft-payment-missing-policy",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-satish"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "", // Missing!
		ViewSnapshotHash:   "view-hash-def456",
		Content: draft.PaymentDraftContent{
			PayeeID:     "sandbox-utility",
			AmountCents: 5000,
			Currency:    "GBP",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for missing PolicySnapshotHash")
	}

	// Test missing ViewSnapshotHash
	d.PolicySnapshotHash = "policy-hash-abc123"
	d.ViewSnapshotHash = "" // Missing!

	_, err = router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for missing ViewSnapshotHash")
	}

	t.Log("Missing hash blocking works correctly")
}

// TestPaymentDraftMissingPayeeBlock verifies that missing payee blocks routing.
func TestPaymentDraftMissingPayeeBlock(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            "draft-payment-missing-payee",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-satish"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		Content: draft.PaymentDraftContent{
			PayeeID:     "", // Missing - free-text not allowed!
			AmountCents: 5000,
			Currency:    "GBP",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for missing PayeeID")
	}

	t.Logf("Missing payee blocked: %v", err)
}

// TestPaymentDraftInvalidAmountBlock verifies that invalid amounts block routing.
func TestPaymentDraftInvalidAmountBlock(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            "draft-payment-zero-amount",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-satish"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		Content: draft.PaymentDraftContent{
			PayeeID:     "sandbox-utility",
			AmountCents: 0, // Invalid!
			Currency:    "GBP",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for zero AmountCents")
	}

	t.Logf("Zero amount blocked: %v", err)
}

// TestPaymentDraftNotApprovedBlock verifies that unapproved drafts block routing.
func TestPaymentDraftNotApprovedBlock(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            "draft-payment-not-approved",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-satish"),
		Status:             draft.StatusProposed, // Not approved!
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		Content: draft.PaymentDraftContent{
			PayeeID:     "sandbox-utility",
			AmountCents: 5000,
			Currency:    "GBP",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for unapproved draft")
	}

	t.Logf("Unapproved draft blocked: %v", err)
}

// TestPaymentDraftDeterministic verifies that routing is deterministic.
func TestPaymentDraftDeterministic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)

	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            "draft-payment-deterministic",
		DraftType:          draft.DraftTypePayment,
		CircleID:           identity.EntityID("circle-satish"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-abc123",
		ViewSnapshotHash:   "view-hash-def456",
		Content: draft.PaymentDraftContent{
			PayeeID:     "sandbox-utility",
			AmountCents: 5000,
			Currency:    "GBP",
			Description: "Test payment",
		},
	}

	intent1, err := router.BuildIntentFromDraft(d)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	intent2, err := router.BuildIntentFromDraft(d)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}

	// Verify deterministic intent ID
	if intent1.IntentID != intent2.IntentID {
		t.Errorf("intent IDs should match: %s != %s", intent1.IntentID, intent2.IntentID)
	}

	// Verify deterministic hash
	if intent1.DeterministicHash != intent2.DeterministicHash {
		t.Errorf("hashes should match")
	}

	t.Logf("Deterministic routing verified: %s", intent1.IntentID)
}
