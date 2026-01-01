// Package demo_phase8_commerce_mirror demonstrates the Commerce Mirror.
//
// This demo shows email-derived commerce event extraction across UK, US, and India vendors.
//
// CRITICAL: Deterministic - same inputs + clock = same outputs and hashes.
// CRITICAL: No goroutines, no external dependencies.
//
// Reference: docs/ADR/ADR-0024-phase8-commerce-mirror-email-derived.md
package demo_phase8_commerce_mirror

import (
	"fmt"
	"time"

	"quantumlife/internal/commerce/extract"
	"quantumlife/internal/obligations"
	"quantumlife/pkg/domain/commerce"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/obligation"
)

// mockClock provides deterministic time.
type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time {
	return c.now
}

// DemoResult contains the result of a demo run.
type DemoResult struct {
	ScenarioName            string
	Success                 bool
	CommerceEvents          []*commerce.CommerceEvent
	CommerceObligations     []*obligation.Obligation
	ExtractionMetrics       commerce.ExtractionMetrics
	CommerceEventCount      int
	CommerceObligationCount int
	EventsHash              string
	ObligationsHash         string
}

// RunAllScenarios runs all demo scenarios.
func RunAllScenarios() ([]DemoResult, string) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name   string
		emails []*domainevents.EmailMessageEvent
	}{
		{"UK Vendors", createUKVendorEmails(fixedTime)},
		{"US Vendors", createUSVendorEmails(fixedTime)},
		{"India Vendors", createIndiaVendorEmails(fixedTime)},
		{"Mixed Vendors", createMixedEmails(fixedTime)},
	}

	var results []DemoResult
	for _, s := range scenarios {
		result := runScenario(s.name, s.emails, fixedTime)
		results = append(results, result)
	}

	// Verify determinism by running again
	deterministicHash := verifyDeterminism(fixedTime)

	return results, deterministicHash
}

// runScenario executes a single demo scenario.
func runScenario(name string, emails []*domainevents.EmailMessageEvent, now time.Time) DemoResult {
	clock := &mockClock{now: now.Add(time.Hour)}

	// Create extraction engine
	extractor := extract.NewEngine(clock)

	// Extract commerce events
	events, metrics := extractor.ExtractFromEmails(emails)

	// Create obligation extractor
	obligExtractor := obligations.NewCommerceObligationExtractor(obligations.DefaultCommerceConfig())

	// Extract obligations from commerce events
	obligs := obligExtractor.ExtractFromCommerceEvents(events, clock.Now())

	return DemoResult{
		ScenarioName:            name,
		Success:                 true,
		CommerceEvents:          events,
		CommerceObligations:     obligs,
		ExtractionMetrics:       metrics,
		CommerceEventCount:      len(events),
		CommerceObligationCount: len(obligs),
		EventsHash:              commerce.ComputeEventsHash(events),
		ObligationsHash:         obligation.ComputeObligationsHash(obligs),
	}
}

// verifyDeterminism runs the same scenario twice and verifies identical output.
func verifyDeterminism(fixedTime time.Time) string {
	emails := createMixedEmails(fixedTime)

	// Run twice
	result1 := runScenario("Determinism Check 1", emails, fixedTime)
	result2 := runScenario("Determinism Check 2", emails, fixedTime)

	if result1.EventsHash != result2.EventsHash {
		return fmt.Sprintf("FAILED: Events hash mismatch: %s vs %s", result1.EventsHash, result2.EventsHash)
	}

	if result1.ObligationsHash != result2.ObligationsHash {
		return fmt.Sprintf("FAILED: Obligations hash mismatch: %s vs %s", result1.ObligationsHash, result2.ObligationsHash)
	}

	return result1.EventsHash
}

// createUKVendorEmails creates UK-focused test emails.
func createUKVendorEmails(now time.Time) []*domainevents.EmailMessageEvent {
	return []*domainevents.EmailMessageEvent{
		createEmail("uk-1", "deliveroo.co.uk", "Your order has been placed", "Order #DEL-12345 Total: £24.99", now),
		createEmail("uk-2", "dpd.co.uk", "Your parcel is out for delivery", "Tracking: 1234567890123456", now.Add(-time.Hour)),
		createEmail("uk-3", "tesco.com", "Order confirmation", "Your grocery order £85.50 is being prepared", now.Add(-2*time.Hour)),
		createEmail("uk-4", "royalmail.com", "Delivered: Your item has arrived", "Successfully delivered to your address", now.Add(-3*time.Hour)),
		createEmail("uk-5", "edf.co.uk", "Your invoice is ready", "Invoice for £120.00 due by 2025-02-01", now.Add(-4*time.Hour)),
	}
}

// createUSVendorEmails creates US-focused test emails.
func createUSVendorEmails(now time.Time) []*domainevents.EmailMessageEvent {
	return []*domainevents.EmailMessageEvent{
		createEmail("us-1", "amazon.com", "Your order has shipped", "Order #123-4567890 Tracking: 1Z999AA10123456784", now),
		createEmail("us-2", "uber.com", "Your trip receipt", "Thanks for riding! Total: $15.50", now.Add(-time.Hour)),
		createEmail("us-3", "doordash.com", "Order confirmed", "Your order from Pizza Place $32.99", now.Add(-2*time.Hour)),
		createEmail("us-4", "netflix.com", "Your subscription has been renewed", "Monthly renewal $15.99", now.Add(-3*time.Hour)),
		createEmail("us-5", "fedex.com", "Package in transit", "Your package is on its way", now.Add(-4*time.Hour)),
	}
}

// createIndiaVendorEmails creates India-focused test emails.
func createIndiaVendorEmails(now time.Time) []*domainevents.EmailMessageEvent {
	return []*domainevents.EmailMessageEvent{
		createEmail("in-1", "swiggy.in", "Order placed successfully", "Order #SW-789012 ₹450.00", now),
		createEmail("in-2", "zomato.com", "Your order is on its way", "Order #ZOM-123 Rs. 350", now.Add(-time.Hour)),
		createEmail("in-3", "flipkart.com", "Order confirmed", "Order #FK-456789 INR 2,499", now.Add(-2*time.Hour)),
		createEmail("in-4", "delhivery.com", "Shipment dispatched", "Your shipment has left our warehouse", now.Add(-3*time.Hour)),
		createEmail("in-5", "ola.com", "Trip receipt", "Thanks for riding! ₹125", now.Add(-4*time.Hour)),
	}
}

// createMixedEmails creates a mix of vendors for comprehensive testing.
func createMixedEmails(now time.Time) []*domainevents.EmailMessageEvent {
	var emails []*domainevents.EmailMessageEvent
	emails = append(emails, createUKVendorEmails(now)...)
	emails = append(emails, createUSVendorEmails(now)...)
	emails = append(emails, createIndiaVendorEmails(now)...)

	// Add non-commerce emails (should be filtered)
	emails = append(emails, createEmail("misc-1", "newsletter.example.com", "Weekly newsletter", "Check out our latest articles", now))
	emails = append(emails, createEmail("misc-2", "social.example.com", "You have new followers", "3 people started following you", now))

	return emails
}

// createEmail creates a test email.
func createEmail(msgID, senderDomain, subject, body string, occurredAt time.Time) *domainevents.EmailMessageEvent {
	email := domainevents.NewEmailMessageEvent(
		"gmail",
		msgID,
		"test@example.com",
		occurredAt.Add(time.Hour),
		occurredAt,
	)
	email.Subject = subject
	email.BodyPreview = body
	email.SenderDomain = senderDomain
	email.From = domainevents.EmailAddress{
		Address: "sender@" + senderDomain,
		Name:    "Sender",
	}
	email.Folder = "INBOX"
	email.IsTransactional = true
	return email
}

// PrintResults prints demo results to stdout.
func PrintResults(results []DemoResult, deterministicHash string) {
	fmt.Println("==============================================")
	fmt.Println("Phase 8: Commerce Mirror Demo")
	fmt.Println("==============================================")
	fmt.Println()

	for _, r := range results {
		fmt.Printf("Scenario: %s\n", r.ScenarioName)
		fmt.Printf("  Success: %t\n", r.Success)
		fmt.Printf("  Commerce Events: %d\n", r.CommerceEventCount)
		fmt.Printf("  Commerce Obligations: %d\n", r.CommerceObligationCount)
		fmt.Printf("  Emails Scanned: %d\n", r.ExtractionMetrics.EmailsScanned)
		fmt.Printf("  Vendors Matched: %d\n", r.ExtractionMetrics.VendorMatchedCount)
		fmt.Printf("  Amounts Parsed: %d\n", r.ExtractionMetrics.AmountsParsed)
		fmt.Printf("  Events Hash: %s\n", r.EventsHash[:16])
		fmt.Printf("  Obligations Hash: %s\n", r.ObligationsHash[:16])
		fmt.Println()

		// Print event details
		if len(r.CommerceEvents) > 0 {
			fmt.Println("  Events:")
			for _, evt := range r.CommerceEvents {
				amount := ""
				if evt.AmountCents > 0 {
					amount = fmt.Sprintf(" %s%d.%02d", evt.Currency, evt.AmountCents/100, evt.AmountCents%100)
				}
				fmt.Printf("    - [%s] %s: %s%s\n", evt.Type, evt.Vendor, evt.Category, amount)
			}
			fmt.Println()
		}

		// Print obligation details
		if len(r.CommerceObligations) > 0 {
			fmt.Println("  Obligations:")
			for _, obl := range r.CommerceObligations {
				fmt.Printf("    - [%s] %s (regret: %.2f)\n", obl.Type, obl.Reason, obl.RegretScore)
			}
			fmt.Println()
		}
	}

	fmt.Println("==============================================")
	fmt.Println("Determinism Verification")
	fmt.Println("==============================================")
	fmt.Printf("Result Hash: %s\n", deterministicHash)
	fmt.Println()
}

// RunDemo is the main entry point for the demo.
func RunDemo() {
	results, hash := RunAllScenarios()
	PrintResults(results, hash)
}
