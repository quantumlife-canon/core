package demo_phase9_commerce_drafts

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/commerce"
	"quantumlife/internal/drafts/email"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

func TestRunDemo(t *testing.T) {
	result := RunDemo()

	if result.Err != nil {
		t.Fatalf("demo failed: %v", result.Err)
	}

	if result.Output == "" {
		t.Error("expected output, got empty string")
	}

	// Verify key sections are present
	expectedSections := []string{
		"PHASE 9: COMMERCE & LIFE ACTION DRAFTS",
		"SHIPMENT FOLLOW-UP DRAFT",
		"REFUND FOLLOW-UP DRAFT",
		"INVOICE REMINDER DRAFT",
		"SUBSCRIPTION REVIEW DRAFT",
		"DETERMINISM VERIFICATION",
		"DEDUPLICATION VERIFICATION",
		"ENGINE STATISTICS",
		"DEMO COMPLETE",
	}

	for _, section := range expectedSections {
		if !strings.Contains(result.Output, section) {
			t.Errorf("output missing section: %s", section)
		}
	}

	// Verify determinism passed
	if !strings.Contains(result.Output, "IDs match: true") {
		t.Error("determinism verification failed: IDs don't match")
	}
	if !strings.Contains(result.Output, "Hashes match: true") {
		t.Error("determinism verification failed: hashes don't match")
	}

	// Verify deduplication worked
	if !strings.Contains(result.Output, "Deduplicated=true") {
		t.Error("deduplication verification failed")
	}
}

func TestCommerceDraftGeneration_ShipmentFollowUp(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	store := draft.NewInMemoryStore()
	policy := draft.DefaultDraftPolicy()
	emailEngine := email.NewDefaultEngine()
	calendarEngine := calendar.NewDefaultEngine()
	commerceEngine := commerce.NewDefaultEngine()
	engine := drafts.NewEngine(store, policy, emailEngine, calendarEngine, commerceEngine)

	obl := obligation.NewObligation(
		circleID,
		"test_shipment_001",
		"commerce",
		obligation.ObligationFollowup,
		fixedTime,
	).WithReason("Track shipment").
		WithEvidence("vendor", "Amazon").
		WithEvidence("order_id", "ORD-123").
		WithEvidence("tracking_id", "TRK-456").
		WithEvidence("status", "in_transit").
		WithEvidence(obligation.EvidenceKeyAmount, "£25.00")

	result := engine.Process(circleID, "", obl, fixedTime)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !result.Generated {
		t.Error("expected draft to be generated")
	}

	d, found := engine.GetDraft(result.DraftID)
	if !found {
		t.Fatal("draft not found")
	}

	if d.DraftType != draft.DraftTypeShipmentFollowUp {
		t.Errorf("DraftType = %s, want %s", d.DraftType, draft.DraftTypeShipmentFollowUp)
	}

	content, ok := d.ShipmentContent()
	if !ok {
		t.Fatal("expected shipment content")
	}
	if content.Vendor != "Amazon" {
		t.Errorf("Vendor = %s, want Amazon", content.Vendor)
	}
	if content.TrackingID != "TRK-456" {
		t.Errorf("TrackingID = %s, want TRK-456", content.TrackingID)
	}
}

func TestCommerceDraftGeneration_RefundFollowUp(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	store := draft.NewInMemoryStore()
	policy := draft.DefaultDraftPolicy()
	engine := drafts.NewEngine(store, policy,
		email.NewDefaultEngine(),
		calendar.NewDefaultEngine(),
		commerce.NewDefaultEngine())

	obl := obligation.NewObligation(
		circleID,
		"test_refund_001",
		"commerce",
		obligation.ObligationFollowup,
		fixedTime,
	).WithReason("Check refund status").
		WithEvidence("vendor", "Deliveroo").
		WithEvidence("order_id", "DEL-789").
		WithEvidence(obligation.EvidenceKeyAmount, "£12.00")

	result := engine.Process(circleID, "", obl, fixedTime)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	d, _ := engine.GetDraft(result.DraftID)
	if d.DraftType != draft.DraftTypeRefundFollowUp {
		t.Errorf("DraftType = %s, want %s", d.DraftType, draft.DraftTypeRefundFollowUp)
	}

	content, ok := d.RefundContent()
	if !ok {
		t.Fatal("expected refund content")
	}
	if content.Vendor != "Deliveroo" {
		t.Errorf("Vendor = %s, want Deliveroo", content.Vendor)
	}
}

func TestCommerceDraftGeneration_InvoiceReminder(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	store := draft.NewInMemoryStore()
	policy := draft.DefaultDraftPolicy()
	engine := drafts.NewEngine(store, policy,
		email.NewDefaultEngine(),
		calendar.NewDefaultEngine(),
		commerce.NewDefaultEngine())

	dueDate := fixedTime.Add(-24 * time.Hour) // Overdue
	obl := obligation.NewObligation(
		circleID,
		"test_invoice_001",
		"commerce",
		obligation.ObligationPay,
		fixedTime,
	).WithReason("Pay invoice").
		WithEvidence("vendor", "EDF Energy").
		WithEvidence("invoice_id", "INV-001").
		WithEvidence(obligation.EvidenceKeyAmount, "£100.00").
		WithDueBy(dueDate, fixedTime)

	result := engine.Process(circleID, "", obl, fixedTime)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	d, _ := engine.GetDraft(result.DraftID)
	if d.DraftType != draft.DraftTypeInvoiceReminder {
		t.Errorf("DraftType = %s, want %s", d.DraftType, draft.DraftTypeInvoiceReminder)
	}

	content, ok := d.InvoiceContent()
	if !ok {
		t.Fatal("expected invoice content")
	}
	if !content.IsOverdue {
		t.Error("expected IsOverdue to be true")
	}
	if !strings.Contains(content.Subject, "OVERDUE") {
		t.Errorf("Subject missing OVERDUE: %s", content.Subject)
	}
}

func TestCommerceDraftGeneration_SubscriptionReview(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	store := draft.NewInMemoryStore()
	policy := draft.DefaultDraftPolicy()
	engine := drafts.NewEngine(store, policy,
		email.NewDefaultEngine(),
		calendar.NewDefaultEngine(),
		commerce.NewDefaultEngine())

	obl := obligation.NewObligation(
		circleID,
		"test_subscription_001",
		"commerce",
		obligation.ObligationReview,
		fixedTime,
	).WithReason("Review subscription").
		WithEvidence("vendor", "Netflix").
		WithEvidence(obligation.EvidenceKeyAmount, "£9.99")

	result := engine.Process(circleID, "", obl, fixedTime)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	d, _ := engine.GetDraft(result.DraftID)
	if d.DraftType != draft.DraftTypeSubscriptionReview {
		t.Errorf("DraftType = %s, want %s", d.DraftType, draft.DraftTypeSubscriptionReview)
	}

	content, ok := d.SubscriptionContent()
	if !ok {
		t.Fatal("expected subscription content")
	}
	if content.Vendor != "Netflix" {
		t.Errorf("Vendor = %s, want Netflix", content.Vendor)
	}
	if content.Action != "review" {
		t.Errorf("Action = %s, want review", content.Action)
	}
}

func TestCommerceDraftDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	// Create identical engines
	createEngine := func() *drafts.Engine {
		return drafts.NewEngine(
			draft.NewInMemoryStore(),
			draft.DefaultDraftPolicy(),
			email.NewDefaultEngine(),
			calendar.NewDefaultEngine(),
			commerce.NewDefaultEngine(),
		)
	}

	engine1 := createEngine()
	engine2 := createEngine()

	obl := obligation.NewObligation(
		circleID,
		"determinism_test_001",
		"commerce",
		obligation.ObligationFollowup,
		fixedTime,
	).WithReason("Track shipment").
		WithEvidence("vendor", "Amazon").
		WithEvidence("order_id", "ORD-999").
		WithEvidence("tracking_id", "TRK-888").
		WithEvidence(obligation.EvidenceKeyAmount, "£50.00")

	result1 := engine1.Process(circleID, "", obl, fixedTime)
	result2 := engine2.Process(circleID, "", obl, fixedTime)

	d1, _ := engine1.GetDraft(result1.DraftID)
	d2, _ := engine2.GetDraft(result2.DraftID)

	if d1.DraftID != d2.DraftID {
		t.Errorf("DraftIDs differ: %s vs %s", d1.DraftID, d2.DraftID)
	}

	if d1.DeterministicHash != d2.DeterministicHash {
		t.Errorf("Hashes differ: %s vs %s", d1.DeterministicHash, d2.DeterministicHash)
	}

	c1, _ := d1.ShipmentContent()
	c2, _ := d2.ShipmentContent()

	if c1.CanonicalString() != c2.CanonicalString() {
		t.Error("canonical strings differ")
	}
}

func TestCommerceDraftDeduplication(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	store := draft.NewInMemoryStore()
	engine := drafts.NewEngine(store, draft.DefaultDraftPolicy(),
		email.NewDefaultEngine(),
		calendar.NewDefaultEngine(),
		commerce.NewDefaultEngine())

	obl := obligation.NewObligation(
		circleID,
		"dedup_test_001",
		"commerce",
		obligation.ObligationFollowup,
		fixedTime,
	).WithReason("Track shipment").
		WithEvidence("vendor", "Amazon").
		WithEvidence("order_id", "ORD-DEDUP").
		WithEvidence("tracking_id", "TRK-DEDUP")

	// First processing should generate
	result1 := engine.Process(circleID, "", obl, fixedTime)
	if !result1.Generated {
		t.Error("expected first process to generate")
	}

	// Second processing should deduplicate
	result2 := engine.Process(circleID, "", obl, fixedTime)
	if !result2.Deduplicated {
		t.Error("expected second process to deduplicate")
	}

	if result1.DraftID != result2.DraftID {
		t.Errorf("deduplication returned different ID: %s vs %s", result1.DraftID, result2.DraftID)
	}
}

func TestNonCommerceObligationSkipped(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle")

	// Only commerce engine
	store := draft.NewInMemoryStore()
	engine := drafts.NewEngine(store, draft.DefaultDraftPolicy(),
		commerce.NewDefaultEngine())

	// Email obligation should not be handled by commerce engine
	emailObl := obligation.NewObligation(
		circleID,
		"email_test_001",
		"email",
		obligation.ObligationReply,
		fixedTime,
	)

	result := engine.Process(circleID, "", emailObl, fixedTime)

	if !result.Skipped {
		t.Error("expected email obligation to be skipped by commerce-only engine")
	}
}
