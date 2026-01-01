package draft

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestShipmentFollowUpContent_CanonicalString(t *testing.T) {
	content := ShipmentFollowUpContent{
		Vendor:          "Amazon",
		VendorContact:   KnownVendorContact("support@amazon.co.uk"),
		OrderID:         "123-456-789",
		TrackingID:      "DPD1234567890",
		ShipmentStatus:  "in_transit",
		Subject:         "Where is my order 123-456-789?",
		Body:            "Hello,\n\nI placed an order and would like to know where it is.",
		OrderDate:       "2025-01-10",
		AmountFormatted: "£24.99",
	}

	canonical1 := content.CanonicalString()
	canonical2 := content.CanonicalString()

	if canonical1 != canonical2 {
		t.Error("canonical string is not stable")
	}

	// Verify key parts
	if !strings.Contains(canonical1, "shipment_followup") {
		t.Error("missing draft type prefix")
	}
	if !strings.Contains(canonical1, "vendor:amazon") {
		t.Error("missing vendor (should be lowercased)")
	}
	if !strings.Contains(canonical1, "tracking:dpd1234567890") {
		t.Error("missing tracking ID")
	}

	// Verify pipe replacement in body
	if strings.Contains(canonical1, "\n") {
		t.Error("newlines should be replaced")
	}
}

func TestRefundFollowUpContent_CanonicalString(t *testing.T) {
	content := RefundFollowUpContent{
		Vendor:          "Deliveroo",
		VendorContact:   UnknownVendorContact("deliveroo_hash"),
		OrderID:         "DEL-12345",
		Subject:         "Refund status enquiry",
		Body:            "I am enquiring about a refund.",
		RefundDate:      "2025-01-15",
		AmountFormatted: "£15.00",
	}

	canonical1 := content.CanonicalString()
	canonical2 := content.CanonicalString()

	if canonical1 != canonical2 {
		t.Error("canonical string is not stable")
	}

	if !strings.Contains(canonical1, "refund_followup") {
		t.Error("missing draft type prefix")
	}
	if !strings.Contains(canonical1, "vendor:deliveroo") {
		t.Error("missing vendor")
	}
	if !strings.Contains(canonical1, "vendor-contact:unknown:") {
		t.Error("missing unknown contact pattern")
	}
}

func TestInvoiceReminderContent_CanonicalString(t *testing.T) {
	content := InvoiceReminderContent{
		Vendor:          "EDF Energy",
		VendorContact:   KnownVendorContact("billing@edf.co.uk"),
		InvoiceID:       "INV-2025-001",
		OrderID:         "",
		Subject:         "Payment reminder",
		Body:            "Please find attached your invoice.",
		InvoiceDate:     "2025-01-01",
		DueDate:         "2025-02-01",
		AmountFormatted: "£120.00",
		IsOverdue:       false,
	}

	canonical1 := content.CanonicalString()

	if !strings.Contains(canonical1, "invoice_reminder") {
		t.Error("missing draft type prefix")
	}
	if !strings.Contains(canonical1, "overdue:false") {
		t.Error("missing overdue flag")
	}

	// Test with overdue
	content.IsOverdue = true
	canonical2 := content.CanonicalString()

	if !strings.Contains(canonical2, "overdue:true") {
		t.Error("overdue flag should be true")
	}
	if canonical1 == canonical2 {
		t.Error("overdue flag should change canonical string")
	}
}

func TestSubscriptionReviewContent_CanonicalString(t *testing.T) {
	content := SubscriptionReviewContent{
		Vendor:          "Netflix",
		VendorContact:   UnknownVendorContact("netflix_hash"),
		SubscriptionID:  "SUB-123",
		Action:          "review",
		Subject:         "Subscription renewal review",
		Body:            "Your subscription has renewed.",
		RenewalDate:     "2025-01-15",
		NextRenewalDate: "2025-02-15",
		AmountFormatted: "£9.99/month",
	}

	canonical1 := content.CanonicalString()

	if !strings.Contains(canonical1, "subscription_review") {
		t.Error("missing draft type prefix")
	}
	if !strings.Contains(canonical1, "action:review") {
		t.Error("missing action")
	}
}

func TestVendorContactRef(t *testing.T) {
	known := KnownVendorContact("support@amazon.co.uk")
	if !known.IsKnown() {
		t.Error("should be known contact")
	}
	if known.Email() != "support@amazon.co.uk" {
		t.Errorf("email mismatch: %s", known.Email())
	}

	unknown := UnknownVendorContact("vendor_hash_123")
	if unknown.IsKnown() {
		t.Error("should be unknown contact")
	}
	if unknown.Email() != "" {
		t.Error("unknown contact should have empty email")
	}

	expectedUnknown := "vendor-contact:unknown:vendor_hash_123"
	if string(unknown) != expectedUnknown {
		t.Errorf("unexpected unknown format: %s", unknown)
	}
}

func TestIsCommerceDraft(t *testing.T) {
	tests := []struct {
		draftType DraftType
		expected  bool
	}{
		{DraftTypeShipmentFollowUp, true},
		{DraftTypeRefundFollowUp, true},
		{DraftTypeInvoiceReminder, true},
		{DraftTypeSubscriptionReview, true},
		{DraftTypeEmailReply, false},
		{DraftTypeCalendarResponse, false},
	}

	for _, tt := range tests {
		if got := IsCommerceDraft(tt.draftType); got != tt.expected {
			t.Errorf("IsCommerceDraft(%s) = %v, want %v", tt.draftType, got, tt.expected)
		}
	}
}

func TestCommerceDraftContentInterface(t *testing.T) {
	// Verify all content types implement CommerceDraftContent
	var _ CommerceDraftContent = ShipmentFollowUpContent{}
	var _ CommerceDraftContent = RefundFollowUpContent{}
	var _ CommerceDraftContent = InvoiceReminderContent{}
	var _ CommerceDraftContent = SubscriptionReviewContent{}

	// Verify they also implement DraftContent
	var _ DraftContent = ShipmentFollowUpContent{}
	var _ DraftContent = RefundFollowUpContent{}
	var _ DraftContent = InvoiceReminderContent{}
	var _ DraftContent = SubscriptionReviewContent{}
}

func TestDraft_CommerceContentAccessors(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	shipmentContent := ShipmentFollowUpContent{
		Vendor:     "Amazon",
		TrackingID: "TRACK123",
	}

	draft := Draft{
		DraftID:            "test-draft-1",
		DraftType:          DraftTypeShipmentFollowUp,
		CircleID:           identity.EntityID("circle-1"),
		SourceObligationID: "obl-1",
		CreatedAt:          now,
		ExpiresAt:          now.Add(48 * time.Hour),
		Status:             StatusProposed,
		Content:            shipmentContent,
	}

	// Test CommerceContent
	cc, ok := draft.CommerceContent()
	if !ok {
		t.Error("should return commerce content")
	}
	if cc.VendorName() != "Amazon" {
		t.Errorf("vendor name mismatch: %s", cc.VendorName())
	}

	// Test ShipmentContent
	sc, ok := draft.ShipmentContent()
	if !ok {
		t.Error("should return shipment content")
	}
	if sc.TrackingID != "TRACK123" {
		t.Error("tracking ID mismatch")
	}

	// Test non-matching accessors
	_, ok = draft.RefundContent()
	if ok {
		t.Error("should not return refund content for shipment draft")
	}

	_, ok = draft.InvoiceContent()
	if ok {
		t.Error("should not return invoice content for shipment draft")
	}
}

func TestNormalizeForCanonical(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello|world", "hello_world"},
		{"line1\nline2", "line1 line2"},
		{"text\r\nwith\rcr", "text withcr"},
		{"normal text", "normal text"},
		{"UPPERCASE", "uppercase"},
		{"  trimmed  ", "trimmed"},
	}

	for _, tt := range tests {
		got := normalizeForCanonical(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeForCanonical(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCommerceContentDeterminism(t *testing.T) {
	// Create same content twice
	content1 := ShipmentFollowUpContent{
		Vendor:          "Amazon",
		VendorContact:   KnownVendorContact("test@amazon.co.uk"),
		OrderID:         "ORD-123",
		TrackingID:      "TRACK-456",
		ShipmentStatus:  "in_transit",
		Subject:         "Test subject",
		Body:            "Test body",
		OrderDate:       "2025-01-15",
		AmountFormatted: "£50.00",
	}

	content2 := ShipmentFollowUpContent{
		Vendor:          "Amazon",
		VendorContact:   KnownVendorContact("test@amazon.co.uk"),
		OrderID:         "ORD-123",
		TrackingID:      "TRACK-456",
		ShipmentStatus:  "in_transit",
		Subject:         "Test subject",
		Body:            "Test body",
		OrderDate:       "2025-01-15",
		AmountFormatted: "£50.00",
	}

	if content1.CanonicalString() != content2.CanonicalString() {
		t.Error("same inputs should produce same canonical string")
	}

	// Modify one field
	content2.TrackingID = "TRACK-789"
	if content1.CanonicalString() == content2.CanonicalString() {
		t.Error("different inputs should produce different canonical strings")
	}
}
