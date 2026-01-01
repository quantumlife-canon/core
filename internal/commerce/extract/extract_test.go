package extract

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/commerce"
	"quantumlife/pkg/domain/events"
)

// mockClock provides deterministic time for tests.
type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time {
	return c.now
}

func TestEngine_ExtractFromEmails_VendorDetection(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	tests := []struct {
		name           string
		senderDomain   string
		subject        string
		expectedVendor string
		expectedCat    commerce.CommerceCategory
	}{
		{
			name:           "Deliveroo UK",
			senderDomain:   "deliveroo.co.uk",
			subject:        "Your order has been placed",
			expectedVendor: "Deliveroo",
			expectedCat:    commerce.CategoryFoodDelivery,
		},
		{
			name:           "Amazon UK",
			senderDomain:   "amazon.co.uk",
			subject:        "Your Amazon order has shipped",
			expectedVendor: "Amazon",
			expectedCat:    commerce.CategoryRetail,
		},
		{
			name:           "DPD courier",
			senderDomain:   "dpd.co.uk",
			subject:        "Your parcel is on its way",
			expectedVendor: "DPD",
			expectedCat:    commerce.CategoryCourier,
		},
		{
			name:           "Uber ride",
			senderDomain:   "uber.com",
			subject:        "Your trip receipt",
			expectedVendor: "Uber",
			expectedCat:    commerce.CategoryRideHailing,
		},
		{
			name:           "Swiggy India",
			senderDomain:   "swiggy.in",
			subject:        "Order confirmed",
			expectedVendor: "Swiggy",
			expectedCat:    commerce.CategoryFoodDelivery,
		},
		{
			name:           "Flipkart India",
			senderDomain:   "flipkart.com",
			subject:        "Your order has been placed",
			expectedVendor: "Flipkart",
			expectedCat:    commerce.CategoryRetail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := createTestEmail("msg-"+tt.name, tt.senderDomain, tt.subject, "", fixedTime)
			events, metrics := engine.ExtractFromEmails([]*events.EmailMessageEvent{email})

			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			if events[0].Vendor != tt.expectedVendor {
				t.Errorf("vendor = %s, want %s", events[0].Vendor, tt.expectedVendor)
			}

			if events[0].Category != tt.expectedCat {
				t.Errorf("category = %s, want %s", events[0].Category, tt.expectedCat)
			}

			if metrics.VendorMatchedCount != 1 {
				t.Errorf("VendorMatchedCount = %d, want 1", metrics.VendorMatchedCount)
			}
		})
	}
}

func TestEngine_ExtractFromEmails_EventTypeClassification(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	tests := []struct {
		name         string
		subject      string
		body         string
		expectedType commerce.CommerceEventType
	}{
		{
			name:         "order placed",
			subject:      "Your order has been placed",
			body:         "Thank you for your order #12345",
			expectedType: commerce.EventOrderPlaced,
		},
		{
			name:         "shipment dispatched",
			subject:      "Your order has been dispatched",
			body:         "Your parcel is on its way",
			expectedType: commerce.EventShipmentUpdate,
		},
		{
			name:         "delivered",
			subject:      "Your order has been delivered",
			body:         "Successfully delivered to your address",
			expectedType: commerce.EventShipmentUpdate,
		},
		{
			name:         "invoice",
			subject:      "Invoice for your recent purchase",
			body:         "Please pay the amount due",
			expectedType: commerce.EventInvoiceIssued,
		},
		{
			name:         "payment receipt",
			subject:      "Payment receipt",
			body:         "We have received your payment of £24.99",
			expectedType: commerce.EventPaymentReceipt,
		},
		{
			name:         "subscription renewed",
			subject:      "Your subscription has been renewed",
			body:         "Auto-renewal successful",
			expectedType: commerce.EventSubscriptionRenewed,
		},
		{
			name:         "ride receipt",
			subject:      "Your trip receipt",
			body:         "Thanks for riding with us",
			expectedType: commerce.EventRideReceipt,
		},
		{
			name:         "refund",
			subject:      "Your refund has been processed",
			body:         "We have refunded £15.00 to your card",
			expectedType: commerce.EventRefundIssued,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := createTestEmail("msg-"+tt.name, "amazon.co.uk", tt.subject, tt.body, fixedTime)
			events, _ := engine.ExtractFromEmails([]*events.EmailMessageEvent{email})

			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			if events[0].Type != tt.expectedType {
				t.Errorf("type = %s, want %s", events[0].Type, tt.expectedType)
			}
		})
	}
}

func TestEngine_ExtractFromEmails_AmountParsing(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	tests := []struct {
		name        string
		subject     string
		body        string
		expectedCur string
		expectedAmt int64
	}{
		{
			name:        "GBP symbol",
			subject:     "Payment receipt £24.99",
			body:        "",
			expectedCur: "GBP",
			expectedAmt: 2499,
		},
		{
			name:        "USD symbol",
			subject:     "Your receipt for $15.50",
			body:        "",
			expectedCur: "USD",
			expectedAmt: 1550,
		},
		{
			name:        "EUR symbol",
			subject:     "Payment confirmation €12.00",
			body:        "",
			expectedCur: "EUR",
			expectedAmt: 1200,
		},
		{
			name:        "INR symbol",
			subject:     "Order placed ₹1,234.56",
			body:        "",
			expectedCur: "INR",
			expectedAmt: 123456,
		},
		{
			name:        "INR Rs format",
			subject:     "Order placed Rs. 999",
			body:        "",
			expectedCur: "INR",
			expectedAmt: 99900,
		},
		{
			name:        "GBP ISO code",
			subject:     "Payment of GBP 50.00",
			body:        "",
			expectedCur: "GBP",
			expectedAmt: 5000,
		},
		{
			name:        "large amount with commas",
			subject:     "Invoice for £1,234.56",
			body:        "",
			expectedCur: "GBP",
			expectedAmt: 123456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := createTestEmail("msg-"+tt.name, "amazon.co.uk", tt.subject, tt.body, fixedTime)
			events, metrics := engine.ExtractFromEmails([]*events.EmailMessageEvent{email})

			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			if events[0].Currency != tt.expectedCur {
				t.Errorf("currency = %s, want %s", events[0].Currency, tt.expectedCur)
			}

			if events[0].AmountCents != tt.expectedAmt {
				t.Errorf("amountCents = %d, want %d", events[0].AmountCents, tt.expectedAmt)
			}

			if metrics.AmountsParsed != 1 {
				t.Errorf("AmountsParsed = %d, want 1", metrics.AmountsParsed)
			}
		})
	}
}

func TestEngine_ExtractFromEmails_OrderAndTrackingIDs(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	tests := []struct {
		name            string
		subject         string
		body            string
		expectedOrderID string
		expectedTrackID string
	}{
		{
			name:            "order with hash",
			subject:         "Order #ABC-12345 confirmed",
			body:            "",
			expectedOrderID: "ABC-12345",
			expectedTrackID: "",
		},
		{
			name:            "tracking number",
			subject:         "Your parcel is on its way",
			body:            "Tracking number: DPD12345678901234",
			expectedOrderID: "",
			expectedTrackID: "DPD12345678901234",
		},
		{
			name:            "both order and tracking",
			subject:         "Order 12345678 shipped",
			body:            "Track your parcel: RM123456789GB",
			expectedOrderID: "12345678",
			expectedTrackID: "RM123456789GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := createTestEmail("msg-"+tt.name, "dpd.co.uk", tt.subject, tt.body, fixedTime)
			events, _ := engine.ExtractFromEmails([]*events.EmailMessageEvent{email})

			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			if tt.expectedOrderID != "" && events[0].OrderID != tt.expectedOrderID {
				t.Errorf("orderID = %s, want %s", events[0].OrderID, tt.expectedOrderID)
			}

			if tt.expectedTrackID != "" && events[0].TrackingID != tt.expectedTrackID {
				t.Errorf("trackingID = %s, want %s", events[0].TrackingID, tt.expectedTrackID)
			}
		})
	}
}

func TestEngine_ExtractFromEmails_Determinism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	emails := []*events.EmailMessageEvent{
		createTestEmail("msg-1", "deliveroo.co.uk", "Your order is on its way", "Tracking: DEL123456789012", fixedTime),
		createTestEmail("msg-2", "amazon.co.uk", "Order confirmed £24.99", "", fixedTime.Add(time.Hour)),
		createTestEmail("msg-3", "dpd.co.uk", "Parcel delivered", "", fixedTime.Add(2*time.Hour)),
	}

	// Run extraction twice
	events1, _ := engine.ExtractFromEmails(emails)
	events2, _ := engine.ExtractFromEmails(emails)

	// Should produce identical results
	if len(events1) != len(events2) {
		t.Fatalf("different event counts: %d vs %d", len(events1), len(events2))
	}

	for i := range events1 {
		if events1[i].EventID != events2[i].EventID {
			t.Errorf("event[%d] ID differs: %s vs %s", i, events1[i].EventID, events2[i].EventID)
		}
		if events1[i].CanonicalString() != events2[i].CanonicalString() {
			t.Errorf("event[%d] canonical string differs", i)
		}
	}

	// Hash should be identical
	hash1 := commerce.ComputeEventsHash(events1)
	hash2 := commerce.ComputeEventsHash(events2)

	if hash1 != hash2 {
		t.Errorf("hashes differ: %s vs %s", hash1, hash2)
	}
}

func TestEngine_ExtractFromEmails_DeterministicOrdering(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	// Create emails in random order
	emails := []*events.EmailMessageEvent{
		createTestEmail("msg-3", "amazon.co.uk", "Order confirmed", "", fixedTime.Add(2*time.Hour)),
		createTestEmail("msg-1", "deliveroo.co.uk", "Order placed", "", fixedTime),
		createTestEmail("msg-2", "dpd.co.uk", "Parcel shipped", "", fixedTime.Add(time.Hour)),
	}

	events, _ := engine.ExtractFromEmails(emails)

	// Should be sorted by time
	for i := 1; i < len(events); i++ {
		if events[i].OccurredAt.Before(events[i-1].OccurredAt) {
			t.Errorf("events not sorted by time at index %d", i)
		}
	}
}

func TestEngine_ExtractFromEmails_NonCommerceFiltered(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	emails := []*events.EmailMessageEvent{
		createTestEmail("msg-1", "amazon.co.uk", "Order confirmed", "", fixedTime),
		createTestEmail("msg-2", "random-sender.com", "Hello, how are you?", "Just checking in", fixedTime),
		createTestEmail("msg-3", "newsletter.com", "Weekly digest", "Here are the top stories", fixedTime),
	}

	events, metrics := engine.ExtractFromEmails(emails)

	if len(events) != 1 {
		t.Errorf("expected 1 commerce event, got %d", len(events))
	}

	if events[0].Vendor != "Amazon" {
		t.Errorf("expected Amazon vendor, got %s", events[0].Vendor)
	}

	if metrics.EmailsScanned != 3 {
		t.Errorf("EmailsScanned = %d, want 3", metrics.EmailsScanned)
	}
}

func TestEngine_ShipmentStatusExtraction(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := &mockClock{now: fixedTime.Add(time.Hour)}

	engine := NewEngine(clock)

	tests := []struct {
		name           string
		subject        string
		body           string
		expectedStatus commerce.ShipmentStatus
	}{
		{
			name:           "dispatched",
			subject:        "Your order has been dispatched",
			body:           "",
			expectedStatus: commerce.ShipmentDispatched,
		},
		{
			name:           "in transit",
			subject:        "Your parcel is in transit",
			body:           "",
			expectedStatus: commerce.ShipmentInTransit,
		},
		{
			name:           "out for delivery",
			subject:        "Your parcel is out for delivery",
			body:           "",
			expectedStatus: commerce.ShipmentOutDelivery,
		},
		{
			name:           "delivered",
			subject:        "Your parcel has been delivered",
			body:           "",
			expectedStatus: commerce.ShipmentDelivered,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := createTestEmail("msg-"+tt.name, "dpd.co.uk", tt.subject, tt.body, fixedTime)
			events, _ := engine.ExtractFromEmails([]*events.EmailMessageEvent{email})

			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			if events[0].ShipmentStatus != tt.expectedStatus {
				t.Errorf("status = %s, want %s", events[0].ShipmentStatus, tt.expectedStatus)
			}
		})
	}
}

func TestAmountParser_Parse(t *testing.T) {
	parser := NewAmountParser()

	tests := []struct {
		name          string
		text          string
		expectedCur   string
		expectedAmt   int64
		expectedValid bool
	}{
		{"GBP simple", "Total: £24.99", "GBP", 2499, true},
		{"GBP no decimal", "Total: £25", "GBP", 2500, true},
		{"GBP with comma", "Total: £1,234.56", "GBP", 123456, true},
		{"USD", "Amount: $15.00", "USD", 1500, true},
		{"EUR", "Price: €10.50", "EUR", 1050, true},
		{"INR symbol", "Order: ₹999", "INR", 99900, true},
		{"INR Rs", "Order: Rs. 1234.56", "INR", 123456, true},
		{"GBP ISO", "GBP 100.00", "GBP", 10000, true},
		{"no amount", "Hello world", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.text)

			if result.Valid != tt.expectedValid {
				t.Errorf("valid = %v, want %v", result.Valid, tt.expectedValid)
			}

			if tt.expectedValid {
				if result.Currency != tt.expectedCur {
					t.Errorf("currency = %s, want %s", result.Currency, tt.expectedCur)
				}
				if result.AmountCents != tt.expectedAmt {
					t.Errorf("amount = %d, want %d", result.AmountCents, tt.expectedAmt)
				}
			}
		})
	}
}

func TestVendorMatcher_MatchByDomain(t *testing.T) {
	matcher := NewVendorMatcher()

	tests := []struct {
		domain         string
		expectedVendor string
		expectedMatch  bool
	}{
		{"deliveroo.co.uk", "Deliveroo", true},
		{"amazon.co.uk", "Amazon", true},
		{"email.amazon.co.uk", "Amazon", true}, // subdomain matching
		{"unknown.com", "", false},
		{"dpd.co.uk", "DPD", true},
		{"swiggy.in", "Swiggy", true},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := matcher.MatchByDomain(tt.domain)

			if result.Matched != tt.expectedMatch {
				t.Errorf("matched = %v, want %v", result.Matched, tt.expectedMatch)
			}

			if tt.expectedMatch && result.CanonicalName != tt.expectedVendor {
				t.Errorf("vendor = %s, want %s", result.CanonicalName, tt.expectedVendor)
			}
		})
	}
}

func TestEventTypeClassifier_Classify(t *testing.T) {
	classifier := NewEventTypeClassifier()

	tests := []struct {
		subject      string
		body         string
		expectedType commerce.CommerceEventType
	}{
		{"Your order has been confirmed", "", commerce.EventOrderPlaced},
		{"Order shipped", "Your package is on its way", commerce.EventShipmentUpdate},
		{"Delivered", "Your parcel was delivered successfully", commerce.EventShipmentUpdate},
		{"Invoice attached", "Please pay the amount due", commerce.EventInvoiceIssued},
		{"Payment receipt", "Thank you for your payment", commerce.EventPaymentReceipt},
		{"Subscription renewed", "Your subscription has been renewed", commerce.EventSubscriptionRenewed},
		{"Your Uber trip", "Thanks for riding", commerce.EventRideReceipt},
		{"Refund processed", "We have refunded your order", commerce.EventRefundIssued},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			result := classifier.Classify(tt.subject, tt.body)

			if result.EventType != tt.expectedType {
				t.Errorf("type = %s, want %s", result.EventType, tt.expectedType)
			}
		})
	}
}

func TestGroupEventsByVendor(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := fixedTime.Add(time.Hour)

	events := []*commerce.CommerceEvent{
		commerce.NewCommerceEvent(commerce.EventOrderPlaced, "gmail", "msg-1", "Amazon", fixedTime, extractTime),
		commerce.NewCommerceEvent(commerce.EventShipmentUpdate, "gmail", "msg-2", "Amazon", fixedTime, extractTime),
		commerce.NewCommerceEvent(commerce.EventOrderPlaced, "gmail", "msg-3", "Deliveroo", fixedTime, extractTime),
	}

	groups := GroupEventsByVendor(events)

	if len(groups["Amazon"]) != 2 {
		t.Errorf("expected 2 Amazon events, got %d", len(groups["Amazon"]))
	}

	if len(groups["Deliveroo"]) != 1 {
		t.Errorf("expected 1 Deliveroo event, got %d", len(groups["Deliveroo"]))
	}
}

func TestSumAmountsByCurrency(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	extractTime := fixedTime.Add(time.Hour)

	events := []*commerce.CommerceEvent{
		commerce.NewCommerceEvent(commerce.EventPaymentReceipt, "gmail", "msg-1", "Amazon", fixedTime, extractTime).WithAmount("GBP", 2499),
		commerce.NewCommerceEvent(commerce.EventPaymentReceipt, "gmail", "msg-2", "Tesco", fixedTime, extractTime).WithAmount("GBP", 5000),
		commerce.NewCommerceEvent(commerce.EventPaymentReceipt, "gmail", "msg-3", "Uber", fixedTime, extractTime).WithAmount("USD", 1500),
	}

	sums := SumAmountsByCurrency(events)

	if sums["GBP"] != 7499 {
		t.Errorf("GBP sum = %d, want 7499", sums["GBP"])
	}

	if sums["USD"] != 1500 {
		t.Errorf("USD sum = %d, want 1500", sums["USD"])
	}
}

// Helper function to create test emails
func createTestEmail(messageID, senderDomain, subject, body string, occurredAt time.Time) *events.EmailMessageEvent {
	email := events.NewEmailMessageEvent(
		"gmail",
		messageID,
		"test@example.com",
		occurredAt.Add(time.Hour),
		occurredAt,
	)
	email.Subject = subject
	email.BodyPreview = body
	email.SenderDomain = senderDomain
	email.From = events.EmailAddress{
		Address: "sender@" + senderDomain,
		Name:    "Sender",
	}
	email.Folder = "INBOX"
	email.IsTransactional = true
	return email
}
