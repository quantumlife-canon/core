package commerce

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestNewCommerceEvent_DeterministicID(t *testing.T) {
	// Same inputs must produce same ID
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	extractTime := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	event1 := NewCommerceEvent(
		EventOrderPlaced,
		"gmail",
		"msg-001",
		"Amazon",
		fixedTime,
		extractTime,
	)

	event2 := NewCommerceEvent(
		EventOrderPlaced,
		"gmail",
		"msg-001",
		"Amazon",
		fixedTime,
		extractTime,
	)

	if event1.EventID != event2.EventID {
		t.Errorf("same inputs produced different IDs: %s vs %s", event1.EventID, event2.EventID)
	}

	// Different message ID should produce different ID
	event3 := NewCommerceEvent(
		EventOrderPlaced,
		"gmail",
		"msg-002",
		"Amazon",
		fixedTime,
		extractTime,
	)

	if event1.EventID == event3.EventID {
		t.Error("different message IDs should produce different event IDs")
	}
}

func TestCommerceEvent_CanonicalStringStability(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	extractTime := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	event := NewCommerceEvent(
		EventShipmentUpdate,
		"gmail",
		"msg-ship-001",
		"DPD",
		fixedTime,
		extractTime,
	).WithCategory(CategoryCourier).
		WithCircle("circle-personal").
		WithVendorDomain("dpd.co.uk").
		WithTrackingID("DPD123456").
		WithShipmentStatus(ShipmentInTransit).
		WithSignal(SignalSubject, "Your parcel is on its way")

	canonical1 := event.CanonicalString()
	canonical2 := event.CanonicalString()

	if canonical1 != canonical2 {
		t.Error("canonical string is not stable across calls")
	}

	// Verify key parts are present
	if !strings.Contains(canonical1, "id:commerce_") {
		t.Error("canonical string missing event ID")
	}
	if !strings.Contains(canonical1, "type:shipment_update") {
		t.Error("canonical string missing event type")
	}
	if !strings.Contains(canonical1, "vendor:dpd") {
		t.Error("canonical string missing vendor")
	}
	if !strings.Contains(canonical1, "tracking:DPD123456") {
		t.Error("canonical string missing tracking ID")
	}
}

func TestCommerceEvent_HashDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	extractTime := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	event := NewCommerceEvent(
		EventPaymentReceipt,
		"gmail",
		"msg-receipt-001",
		"Deliveroo",
		fixedTime,
		extractTime,
	).WithCategory(CategoryFoodDelivery).
		WithAmount("GBP", 2499)

	hash1 := event.ComputeHash()
	hash2 := event.ComputeHash()

	if hash1 != hash2 {
		t.Errorf("hash not deterministic: %s vs %s", hash1, hash2)
	}

	if len(hash1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("unexpected hash length: %d", len(hash1))
	}
}

func TestSortByTimeThenID(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events := []*CommerceEvent{
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-c", "VendorC", now.Add(2*time.Hour), extractTime),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-a", "VendorA", now, extractTime),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-b", "VendorB", now, extractTime),
	}

	SortByTimeThenID(events)

	// First two should have same time, sorted by ID
	if events[0].OccurredAt.After(events[1].OccurredAt) {
		t.Error("events not sorted by time")
	}

	// For same time, should be sorted by ID
	if events[0].OccurredAt.Equal(events[1].OccurredAt) {
		if events[0].EventID > events[1].EventID {
			t.Error("events with same time not sorted by ID")
		}
	}

	// Last event should be the one with latest time
	if !events[2].OccurredAt.After(events[0].OccurredAt) {
		t.Error("latest event should be last")
	}
}

func TestComputeEventsHash_Determinism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events1 := []*CommerceEvent{
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Amazon", now, extractTime),
		NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-2", "DPD", now.Add(time.Hour), extractTime),
	}

	// Create same events in different order
	events2 := []*CommerceEvent{
		NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-2", "DPD", now.Add(time.Hour), extractTime),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Amazon", now, extractTime),
	}

	hash1 := ComputeEventsHash(events1)
	hash2 := ComputeEventsHash(events2)

	// Should produce same hash regardless of input order (sorted before hashing)
	if hash1 != hash2 {
		t.Errorf("hash not deterministic across orderings: %s vs %s", hash1, hash2)
	}
}

func TestComputeEventsHash_Empty(t *testing.T) {
	hash := ComputeEventsHash(nil)
	if hash != "empty" {
		t.Errorf("empty events should produce 'empty' hash, got: %s", hash)
	}

	hash2 := ComputeEventsHash([]*CommerceEvent{})
	if hash2 != "empty" {
		t.Errorf("empty slice should produce 'empty' hash, got: %s", hash2)
	}
}

func TestFilterByType(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events := []*CommerceEvent{
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Amazon", now, extractTime),
		NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-2", "DPD", now, extractTime),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-3", "Tesco", now, extractTime),
	}

	orders := FilterByType(events, EventOrderPlaced)
	if len(orders) != 2 {
		t.Errorf("expected 2 order events, got %d", len(orders))
	}

	shipments := FilterByType(events, EventShipmentUpdate)
	if len(shipments) != 1 {
		t.Errorf("expected 1 shipment event, got %d", len(shipments))
	}
}

func TestFilterByCategory(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events := []*CommerceEvent{
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Deliveroo", now, extractTime).WithCategory(CategoryFoodDelivery),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-2", "Tesco", now, extractTime).WithCategory(CategoryGrocery),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-3", "UberEats", now, extractTime).WithCategory(CategoryFoodDelivery),
	}

	food := FilterByCategory(events, CategoryFoodDelivery)
	if len(food) != 2 {
		t.Errorf("expected 2 food delivery events, got %d", len(food))
	}

	grocery := FilterByCategory(events, CategoryGrocery)
	if len(grocery) != 1 {
		t.Errorf("expected 1 grocery event, got %d", len(grocery))
	}
}

func TestFilterByVendor(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events := []*CommerceEvent{
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Amazon", now, extractTime),
		NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-2", "Amazon", now, extractTime),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-3", "Tesco", now, extractTime),
	}

	// Case-insensitive matching
	amazon := FilterByVendor(events, "amazon")
	if len(amazon) != 2 {
		t.Errorf("expected 2 Amazon events, got %d", len(amazon))
	}

	amazonUpper := FilterByVendor(events, "AMAZON")
	if len(amazonUpper) != 2 {
		t.Errorf("expected 2 Amazon events (case-insensitive), got %d", len(amazonUpper))
	}
}

func TestFilterByCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events := []*CommerceEvent{
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Amazon", now, extractTime).WithCircle("circle-personal"),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-2", "Tesco", now, extractTime).WithCircle("circle-family"),
		NewCommerceEvent(EventOrderPlaced, "gmail", "msg-3", "DPD", now, extractTime).WithCircle("circle-personal"),
	}

	personal := FilterByCircle(events, "circle-personal")
	if len(personal) != 2 {
		t.Errorf("expected 2 personal events, got %d", len(personal))
	}
}

func TestHasPendingShipment(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	tests := []struct {
		name     string
		event    *CommerceEvent
		expected bool
	}{
		{
			name:     "dispatched is pending",
			event:    NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-1", "DPD", now, extractTime).WithShipmentStatus(ShipmentDispatched),
			expected: true,
		},
		{
			name:     "in transit is pending",
			event:    NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-2", "DPD", now, extractTime).WithShipmentStatus(ShipmentInTransit),
			expected: true,
		},
		{
			name:     "out for delivery is pending",
			event:    NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-3", "DPD", now, extractTime).WithShipmentStatus(ShipmentOutDelivery),
			expected: true,
		},
		{
			name:     "delivered is not pending",
			event:    NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-4", "DPD", now, extractTime).WithShipmentStatus(ShipmentDelivered),
			expected: false,
		},
		{
			name:     "failed is not pending",
			event:    NewCommerceEvent(EventShipmentUpdate, "gmail", "msg-5", "DPD", now, extractTime).WithShipmentStatus(ShipmentFailed),
			expected: false,
		},
		{
			name:     "non-shipment event is not pending",
			event:    NewCommerceEvent(EventOrderPlaced, "gmail", "msg-6", "Amazon", now, extractTime),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.HasPendingShipment(); got != tt.expected {
				t.Errorf("HasPendingShipment() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractionMetrics_Add(t *testing.T) {
	m1 := ExtractionMetrics{
		EmailsScanned:      10,
		EventsEmitted:      5,
		VendorMatchedCount: 4,
	}

	m2 := ExtractionMetrics{
		EmailsScanned:      20,
		EventsEmitted:      8,
		VendorMatchedCount: 6,
		UnknownVendorCount: 2,
	}

	m1.Add(m2)

	if m1.EmailsScanned != 30 {
		t.Errorf("EmailsScanned = %d, want 30", m1.EmailsScanned)
	}
	if m1.EventsEmitted != 13 {
		t.Errorf("EventsEmitted = %d, want 13", m1.EventsEmitted)
	}
	if m1.VendorMatchedCount != 10 {
		t.Errorf("VendorMatchedCount = %d, want 10", m1.VendorMatchedCount)
	}
	if m1.UnknownVendorCount != 2 {
		t.Errorf("UnknownVendorCount = %d, want 2", m1.UnknownVendorCount)
	}
}

func TestNormalizeForHash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Amazon", "amazon"},
		{"  Deliveroo  ", "deliveroo"},
		{"Pipe|Char", "pipe_char"},
		{"New\nLine", "new line"},
		{"Carriage\rReturn", "carriagereturn"},
	}

	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Use the canonical string to verify normalization indirectly
			event := NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", tt.input, fixedTime, fixedTime)
			// The CanonicalString() contains "vendor:{normalized}" format
			if !strings.Contains(event.CanonicalString(), "vendor:"+tt.expected) {
				t.Errorf("vendor not normalized correctly in canonical string: %s", event.CanonicalString())
			}
		})
	}
}

func TestSortByAmountDesc(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	events := []*CommerceEvent{
		NewCommerceEvent(EventPaymentReceipt, "gmail", "msg-1", "Amazon", now, extractTime).WithAmount("GBP", 1000),
		NewCommerceEvent(EventPaymentReceipt, "gmail", "msg-2", "Deliveroo", now, extractTime).WithAmount("GBP", 5000),
		NewCommerceEvent(EventPaymentReceipt, "gmail", "msg-3", "Tesco", now, extractTime).WithAmount("GBP", 2500),
	}

	SortByAmountDesc(events)

	if events[0].AmountCents != 5000 {
		t.Errorf("first event should have highest amount, got %d", events[0].AmountCents)
	}
	if events[1].AmountCents != 2500 {
		t.Errorf("second event should have middle amount, got %d", events[1].AmountCents)
	}
	if events[2].AmountCents != 1000 {
		t.Errorf("third event should have lowest amount, got %d", events[2].AmountCents)
	}
}

func TestCommerceEvent_WithMethods(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := now.Add(time.Hour)

	event := NewCommerceEvent(EventOrderPlaced, "gmail", "msg-1", "Amazon", now, extractTime).
		WithCategory(CategoryRetail).
		WithCircle(identity.EntityID("circle-personal")).
		WithVendorDomain("amazon.co.uk").
		WithAmount("GBP", 4999).
		WithOrderID("ORDER-12345").
		WithTrackingID("TRACK-67890").
		WithShipmentStatus(ShipmentDispatched).
		WithSignal(SignalSubject, "Your order has been placed")

	if event.Category != CategoryRetail {
		t.Error("WithCategory failed")
	}
	if event.CircleID != "circle-personal" {
		t.Error("WithCircle failed")
	}
	if event.VendorDomain != "amazon.co.uk" {
		t.Error("WithVendorDomain failed")
	}
	if event.Currency != "GBP" || event.AmountCents != 4999 {
		t.Error("WithAmount failed")
	}
	if event.OrderID != "ORDER-12345" {
		t.Error("WithOrderID failed")
	}
	if event.TrackingID != "TRACK-67890" {
		t.Error("WithTrackingID failed")
	}
	if event.ShipmentStatus != ShipmentDispatched {
		t.Error("WithShipmentStatus failed")
	}
	if event.RawSignals[SignalSubject] != "Your order has been placed" {
		t.Error("WithSignal failed")
	}
}
