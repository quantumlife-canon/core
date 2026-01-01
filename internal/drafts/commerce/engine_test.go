package commerce

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

func TestEngine_CanHandle(t *testing.T) {
	engine := NewDefaultEngine()

	tests := []struct {
		name     string
		obl      *obligation.Obligation
		expected bool
	}{
		{
			name:     "nil obligation",
			obl:      nil,
			expected: false,
		},
		{
			name: "commerce obligation",
			obl: &obligation.Obligation{
				SourceType: "commerce",
				Type:       obligation.ObligationFollowup,
			},
			expected: true,
		},
		{
			name: "email obligation",
			obl: &obligation.Obligation{
				SourceType: "email",
				Type:       obligation.ObligationReply,
			},
			expected: false,
		},
		{
			name: "calendar obligation",
			obl: &obligation.Obligation{
				SourceType: "calendar",
				Type:       obligation.ObligationAttend,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engine.CanHandle(tt.obl); got != tt.expected {
				t.Errorf("CanHandle() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEngine_Generate_ShipmentFollowUp(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"commerce_event_123",
		"commerce",
		obligation.ObligationFollowup,
		now.Add(-24*time.Hour),
	).WithReason("Track shipment from Amazon").
		WithEvidence("vendor", "Amazon").
		WithEvidence("order_id", "123-456-789").
		WithEvidence("tracking_id", "DPD1234567890").
		WithEvidence("status", "in_transit").
		WithEvidence(obligation.EvidenceKeyAmount, "£24.99")

	ctx := draft.GenerationContext{
		CircleID:   identity.EntityID("circle-personal"),
		Obligation: obl,
		Now:        now,
		Policy:     draft.DefaultDraftPolicy(),
	}

	result := engine.Generate(ctx)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Skipped {
		t.Fatalf("unexpected skip: %s", result.SkipReason)
	}
	if result.Draft == nil {
		t.Fatal("expected draft, got nil")
	}

	d := result.Draft
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
	if content.TrackingID != "DPD1234567890" {
		t.Errorf("TrackingID = %s, want DPD1234567890", content.TrackingID)
	}
	if content.OrderID != "123-456-789" {
		t.Errorf("OrderID = %s, want 123-456-789", content.OrderID)
	}
	if !strings.Contains(content.Subject, "Where is my order") {
		t.Errorf("Subject missing key phrase: %s", content.Subject)
	}
	if !strings.Contains(content.Body, "123-456-789") {
		t.Errorf("Body missing order ID: %s", content.Body)
	}
}

func TestEngine_Generate_RefundFollowUp(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"commerce_refund_456",
		"commerce",
		obligation.ObligationFollowup,
		now.Add(-5*24*time.Hour),
	).WithReason("Check refund status from Deliveroo").
		WithEvidence("vendor", "Deliveroo").
		WithEvidence("order_id", "DEL-12345").
		WithEvidence(obligation.EvidenceKeyAmount, "£15.00")

	ctx := draft.GenerationContext{
		CircleID:   identity.EntityID("circle-personal"),
		Obligation: obl,
		Now:        now,
		Policy:     draft.DefaultDraftPolicy(),
	}

	result := engine.Generate(ctx)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Draft == nil {
		t.Fatal("expected draft, got nil")
	}

	d := result.Draft
	if d.DraftType != draft.DraftTypeRefundFollowUp {
		t.Errorf("DraftType = %s, want %s", d.DraftType, draft.DraftTypeRefundFollowUp)
	}

	content, ok := d.RefundContent()
	if !ok {
		t.Fatal("expected refund content")
	}

	if !strings.Contains(content.Subject, "Refund") {
		t.Errorf("Subject missing 'Refund': %s", content.Subject)
	}
	if content.AmountFormatted != "£15.00" {
		t.Errorf("AmountFormatted = %s, want £15.00", content.AmountFormatted)
	}
}

func TestEngine_Generate_InvoiceReminder(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	dueDate := now.Add(-24 * time.Hour) // Overdue

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"commerce_invoice_789",
		"commerce",
		obligation.ObligationPay,
		now.Add(-7*24*time.Hour),
	).WithReason("Pay invoice from EDF Energy").
		WithEvidence("vendor", "EDF Energy").
		WithEvidence("invoice_id", "INV-2025-001").
		WithEvidence(obligation.EvidenceKeyAmount, "£120.00").
		WithDueBy(dueDate, now)

	ctx := draft.GenerationContext{
		CircleID:   identity.EntityID("circle-personal"),
		Obligation: obl,
		Now:        now,
		Policy:     draft.DefaultDraftPolicy(),
	}

	result := engine.Generate(ctx)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Draft == nil {
		t.Fatal("expected draft, got nil")
	}

	d := result.Draft
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
		t.Errorf("Subject missing 'OVERDUE' for overdue invoice: %s", content.Subject)
	}
	if content.InvoiceID != "INV-2025-001" {
		t.Errorf("InvoiceID = %s, want INV-2025-001", content.InvoiceID)
	}
}

func TestEngine_Generate_SubscriptionReview(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"commerce_sub_123",
		"commerce",
		obligation.ObligationReview,
		now.Add(-24*time.Hour),
	).WithReason("Review Netflix subscription renewal").
		WithEvidence("vendor", "Netflix").
		WithEvidence(obligation.EvidenceKeyAmount, "£9.99")

	ctx := draft.GenerationContext{
		CircleID:   identity.EntityID("circle-personal"),
		Obligation: obl,
		Now:        now,
		Policy:     draft.DefaultDraftPolicy(),
	}

	result := engine.Generate(ctx)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Draft == nil {
		t.Fatal("expected draft, got nil")
	}

	d := result.Draft
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

func TestEngine_Generate_Determinism(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"commerce_event_123",
		"commerce",
		obligation.ObligationFollowup,
		now.Add(-24*time.Hour),
	).WithReason("Track shipment from Amazon").
		WithEvidence("vendor", "Amazon").
		WithEvidence("order_id", "123-456-789").
		WithEvidence("tracking_id", "DPD1234567890")

	ctx := draft.GenerationContext{
		CircleID:   identity.EntityID("circle-personal"),
		Obligation: obl,
		Now:        now,
		Policy:     draft.DefaultDraftPolicy(),
	}

	// Generate twice
	result1 := engine.Generate(ctx)
	result2 := engine.Generate(ctx)

	if result1.Draft == nil || result2.Draft == nil {
		t.Fatal("expected drafts")
	}

	// Should be identical
	if result1.Draft.DraftID != result2.Draft.DraftID {
		t.Errorf("DraftIDs differ: %s vs %s", result1.Draft.DraftID, result2.Draft.DraftID)
	}
	if result1.Draft.DeterministicHash != result2.Draft.DeterministicHash {
		t.Errorf("Hashes differ: %s vs %s", result1.Draft.DeterministicHash, result2.Draft.DeterministicHash)
	}

	content1, _ := result1.Draft.ShipmentContent()
	content2, _ := result2.Draft.ShipmentContent()

	if content1.CanonicalString() != content2.CanonicalString() {
		t.Error("canonical strings differ")
	}
}

func TestEngine_Generate_SkipsNonCommerce(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"email_event_123",
		"email",
		obligation.ObligationReply,
		now,
	)

	ctx := draft.GenerationContext{
		CircleID:   identity.EntityID("circle-personal"),
		Obligation: obl,
		Now:        now,
		Policy:     draft.DefaultDraftPolicy(),
	}

	result := engine.Generate(ctx)

	if !result.Skipped {
		t.Error("expected to skip non-commerce obligation")
	}
	if result.Draft != nil {
		t.Error("expected no draft for non-commerce obligation")
	}
}

func TestParseFormattedAmount(t *testing.T) {
	tests := []struct {
		formatted        string
		expectedCents    int64
		expectedCurrency string
	}{
		{"£24.99", 2499, "GBP"},
		{"$15.00", 1500, "USD"},
		{"€10.50", 1050, "EUR"},
		{"₹999.00", 99900, "INR"},
		{"Rs.1234.56", 123456, "INR"},
		{"GBP 50.00", 5000, "GBP"},
		{"£1,234.56", 123456, "GBP"},
		{"", 0, ""},
		{"invalid", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.formatted, func(t *testing.T) {
			cents, currency := parseFormattedAmount(tt.formatted)
			if cents != tt.expectedCents {
				t.Errorf("cents = %d, want %d", cents, tt.expectedCents)
			}
			if currency != tt.expectedCurrency {
				t.Errorf("currency = %s, want %s", currency, tt.expectedCurrency)
			}
		})
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		cents    int64
		currency string
		expected string
	}{
		{2499, "GBP", "£24.99"},
		{1500, "USD", "$15.00"},
		{1050, "EUR", "€10.50"},
		{99900, "INR", "₹999.00"},
		{0, "GBP", ""},
		{5000, "XYZ", "50.00 XYZ"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatAmount(tt.cents, tt.currency)
			if got != tt.expected {
				t.Errorf("formatAmount(%d, %s) = %s, want %s", tt.cents, tt.currency, got, tt.expected)
			}
		})
	}
}

func TestEngine_DeriveVendorContact(t *testing.T) {
	engine := NewDefaultEngine()

	tests := []struct {
		name        string
		ctx         draft.CommerceContext
		expectKnown bool
	}{
		{
			name: "with domain",
			ctx: draft.CommerceContext{
				Vendor:       "Amazon",
				VendorDomain: "amazon.co.uk",
			},
			expectKnown: true,
		},
		{
			name: "vendor only",
			ctx: draft.CommerceContext{
				Vendor: "Amazon",
			},
			expectKnown: false,
		},
		{
			name:        "empty",
			ctx:         draft.CommerceContext{},
			expectKnown: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contact := engine.deriveVendorContact(tt.ctx)
			if contact.IsKnown() != tt.expectKnown {
				t.Errorf("IsKnown() = %v, want %v", contact.IsKnown(), tt.expectKnown)
			}
		})
	}
}

func TestEngine_BuildSafetyNotes(t *testing.T) {
	engine := NewDefaultEngine()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	obl := obligation.NewObligation(
		identity.EntityID("circle-personal"),
		"test",
		"commerce",
		obligation.ObligationPay,
		now,
	).WithSeverity(obligation.SeverityHigh)

	// Test invoice with high priority
	content := draft.InvoiceReminderContent{
		IsOverdue: true,
	}

	notes := engine.buildSafetyNotes(content, obl)

	// Should have overdue note and high priority note
	if len(notes) < 2 {
		t.Errorf("expected at least 2 safety notes, got %d", len(notes))
	}

	// Should be sorted
	for i := 1; i < len(notes); i++ {
		if notes[i-1] > notes[i] {
			t.Error("safety notes not sorted")
			break
		}
	}
}
