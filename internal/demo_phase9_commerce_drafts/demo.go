// Package demo_phase9_commerce_drafts demonstrates Phase 9: Commerce Action Drafts.
//
// This demo shows:
// 1. Commerce draft generation from obligations (shipment, refund, invoice, subscription)
// 2. Deterministic draft creation (same inputs + clock = identical drafts)
// 3. Draft content structure and canonical hashing
// 4. Integration with the drafts engine and deduplication
//
// CRITICAL: Drafts only. NO external writes, NO payments, NO sends.
// CRITICAL: Deterministic. Same inputs + clock = same drafts.
// CRITICAL: Vendor-agnostic. Uses canonical CommerceEvent fields only.
//
// Reference: docs/ADR/ADR-0025-phase9-commerce-action-drafts.md
package demo_phase9_commerce_drafts

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/commerce"
	"quantumlife/internal/drafts/email"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// DemoResult contains the result of a demo run.
type DemoResult struct {
	Output string
	Err    error
}

// RunDemo executes the Phase 9 commerce drafts demo.
func RunDemo() DemoResult {
	var out strings.Builder

	out.WriteString("================================================================================\n")
	out.WriteString("          PHASE 9: COMMERCE & LIFE ACTION DRAFTS DEMO\n")
	out.WriteString("================================================================================\n\n")

	// Use fixed time for deterministic demo
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	out.WriteString(fmt.Sprintf("Demo time: %s\n", fixedTime.Format(time.RFC3339)))
	out.WriteString("Clock: Fixed (deterministic)\n\n")

	// Create domain entities
	circleID := identity.EntityID("circle-personal-demo")

	// Initialize components
	store := draft.NewInMemoryStore()
	policy := draft.DefaultDraftPolicy()
	emailEngine := email.NewDefaultEngine()
	calendarEngine := calendar.NewDefaultEngine()
	commerceEngine := commerce.NewDefaultEngine()
	engine := drafts.NewEngine(store, policy, emailEngine, calendarEngine, commerceEngine)

	// SECTION 1: SHIPMENT FOLLOW-UP DRAFT
	out.WriteString("1. SHIPMENT FOLLOW-UP DRAFT\n")
	out.WriteString("----------------------------\n\n")

	shipmentObl := obligation.NewObligation(
		circleID,
		"commerce_shipment_001",
		"commerce",
		obligation.ObligationFollowup,
		fixedTime.Add(-24*time.Hour),
	).WithReason("Track shipment from Amazon UK").
		WithEvidence("vendor", "Amazon").
		WithEvidence("order_id", "205-1234567-8901234").
		WithEvidence("tracking_id", "TBA12345678901").
		WithEvidence("status", "in_transit").
		WithEvidence(obligation.EvidenceKeyAmount, "£49.99")

	result := engine.Process(circleID, "", shipmentObl, fixedTime)
	if result.Error != nil {
		return DemoResult{Err: fmt.Errorf("shipment draft generation failed: %w", result.Error)}
	}

	if result.Generated {
		d, _ := engine.GetDraft(result.DraftID)
		out.WriteString(fmt.Sprintf("Generated draft: %s\n", d.DraftID))
		out.WriteString(fmt.Sprintf("  Type: %s\n", d.DraftType))
		out.WriteString(fmt.Sprintf("  Hash: %s\n", d.DeterministicHash))

		if content, ok := d.ShipmentContent(); ok {
			out.WriteString(fmt.Sprintf("  Vendor: %s\n", content.Vendor))
			out.WriteString(fmt.Sprintf("  Order ID: %s\n", content.OrderID))
			out.WriteString(fmt.Sprintf("  Tracking ID: %s\n", content.TrackingID))
			out.WriteString(fmt.Sprintf("  Subject: %s\n", content.Subject))
		}
	}
	out.WriteString("\n")

	// SECTION 2: REFUND FOLLOW-UP DRAFT
	out.WriteString("2. REFUND FOLLOW-UP DRAFT\n")
	out.WriteString("-------------------------\n\n")

	refundObl := obligation.NewObligation(
		circleID,
		"commerce_refund_001",
		"commerce",
		obligation.ObligationFollowup,
		fixedTime.Add(-5*24*time.Hour),
	).WithReason("Check refund status from Deliveroo").
		WithEvidence("vendor", "Deliveroo").
		WithEvidence("order_id", "DEL-98765").
		WithEvidence(obligation.EvidenceKeyAmount, "£18.50")

	result = engine.Process(circleID, "", refundObl, fixedTime)
	if result.Error != nil {
		return DemoResult{Err: fmt.Errorf("refund draft generation failed: %w", result.Error)}
	}

	if result.Generated {
		d, _ := engine.GetDraft(result.DraftID)
		out.WriteString(fmt.Sprintf("Generated draft: %s\n", d.DraftID))
		out.WriteString(fmt.Sprintf("  Type: %s\n", d.DraftType))
		out.WriteString(fmt.Sprintf("  Hash: %s\n", d.DeterministicHash))

		if content, ok := d.RefundContent(); ok {
			out.WriteString(fmt.Sprintf("  Vendor: %s\n", content.Vendor))
			out.WriteString(fmt.Sprintf("  Order ID: %s\n", content.OrderID))
			out.WriteString(fmt.Sprintf("  Amount: %s\n", content.AmountFormatted))
			out.WriteString(fmt.Sprintf("  Subject: %s\n", content.Subject))
		}
	}
	out.WriteString("\n")

	// SECTION 3: INVOICE REMINDER DRAFT
	out.WriteString("3. INVOICE REMINDER DRAFT (OVERDUE)\n")
	out.WriteString("------------------------------------\n\n")

	dueDate := fixedTime.Add(-48 * time.Hour) // 2 days overdue
	invoiceObl := obligation.NewObligation(
		circleID,
		"commerce_invoice_001",
		"commerce",
		obligation.ObligationPay,
		fixedTime.Add(-7*24*time.Hour),
	).WithReason("Pay invoice from EDF Energy").
		WithEvidence("vendor", "EDF Energy").
		WithEvidence("invoice_id", "INV-2025-001234").
		WithEvidence(obligation.EvidenceKeyAmount, "£156.00").
		WithDueBy(dueDate, fixedTime)

	result = engine.Process(circleID, "", invoiceObl, fixedTime)
	if result.Error != nil {
		return DemoResult{Err: fmt.Errorf("invoice draft generation failed: %w", result.Error)}
	}

	if result.Generated {
		d, _ := engine.GetDraft(result.DraftID)
		out.WriteString(fmt.Sprintf("Generated draft: %s\n", d.DraftID))
		out.WriteString(fmt.Sprintf("  Type: %s\n", d.DraftType))
		out.WriteString(fmt.Sprintf("  Hash: %s\n", d.DeterministicHash))

		if content, ok := d.InvoiceContent(); ok {
			out.WriteString(fmt.Sprintf("  Vendor: %s\n", content.Vendor))
			out.WriteString(fmt.Sprintf("  Invoice ID: %s\n", content.InvoiceID))
			out.WriteString(fmt.Sprintf("  Amount: %s\n", content.AmountFormatted))
			out.WriteString(fmt.Sprintf("  Overdue: %v\n", content.IsOverdue))
			out.WriteString(fmt.Sprintf("  Subject: %s\n", content.Subject))
		}
	}
	out.WriteString("\n")

	// SECTION 4: SUBSCRIPTION REVIEW DRAFT
	out.WriteString("4. SUBSCRIPTION REVIEW DRAFT\n")
	out.WriteString("-----------------------------\n\n")

	subscriptionObl := obligation.NewObligation(
		circleID,
		"commerce_subscription_001",
		"commerce",
		obligation.ObligationReview,
		fixedTime.Add(-24*time.Hour),
	).WithReason("Review Netflix subscription renewal").
		WithEvidence("vendor", "Netflix").
		WithEvidence(obligation.EvidenceKeyAmount, "£10.99")

	result = engine.Process(circleID, "", subscriptionObl, fixedTime)
	if result.Error != nil {
		return DemoResult{Err: fmt.Errorf("subscription draft generation failed: %w", result.Error)}
	}

	if result.Generated {
		d, _ := engine.GetDraft(result.DraftID)
		out.WriteString(fmt.Sprintf("Generated draft: %s\n", d.DraftID))
		out.WriteString(fmt.Sprintf("  Type: %s\n", d.DraftType))
		out.WriteString(fmt.Sprintf("  Hash: %s\n", d.DeterministicHash))

		if content, ok := d.SubscriptionContent(); ok {
			out.WriteString(fmt.Sprintf("  Vendor: %s\n", content.Vendor))
			out.WriteString(fmt.Sprintf("  Amount: %s\n", content.AmountFormatted))
			out.WriteString(fmt.Sprintf("  Action: %s\n", content.Action))
			out.WriteString(fmt.Sprintf("  Subject: %s\n", content.Subject))
		}
	}
	out.WriteString("\n")

	// SECTION 5: DETERMINISM VERIFICATION
	out.WriteString("5. DETERMINISM VERIFICATION\n")
	out.WriteString("----------------------------\n\n")

	// Create a fresh engine with same config
	store2 := draft.NewInMemoryStore()
	engine2 := drafts.NewEngine(store2, policy, emailEngine, calendarEngine, commerceEngine)

	// Process the same obligation
	result1 := engine.Process(circleID, "", shipmentObl, fixedTime)
	result2 := engine2.Process(circleID, "", shipmentObl, fixedTime)

	d1, _ := engine.GetDraft(result1.DraftID)
	d2, _ := engine2.GetDraft(result2.DraftID)

	out.WriteString(fmt.Sprintf("Run 1 Draft ID: %s\n", d1.DraftID))
	out.WriteString(fmt.Sprintf("Run 2 Draft ID: %s\n", d2.DraftID))
	out.WriteString(fmt.Sprintf("IDs match: %v\n", d1.DraftID == d2.DraftID))

	out.WriteString(fmt.Sprintf("Run 1 Hash: %s\n", d1.DeterministicHash))
	out.WriteString(fmt.Sprintf("Run 2 Hash: %s\n", d2.DeterministicHash))
	out.WriteString(fmt.Sprintf("Hashes match: %v\n", d1.DeterministicHash == d2.DeterministicHash))
	out.WriteString("\n")

	// SECTION 6: DEDUPLICATION
	out.WriteString("6. DEDUPLICATION VERIFICATION\n")
	out.WriteString("------------------------------\n\n")

	// Process same obligation again (should deduplicate)
	dedupResult := engine.Process(circleID, "", shipmentObl, fixedTime)
	out.WriteString(fmt.Sprintf("First result: Generated=%v\n", result1.Generated))
	out.WriteString(fmt.Sprintf("Dedup result: Deduplicated=%v\n", dedupResult.Deduplicated))
	out.WriteString(fmt.Sprintf("Same draft ID: %v\n", result1.DraftID == dedupResult.DraftID))
	out.WriteString("\n")

	// SECTION 7: ENGINE STATS
	out.WriteString("7. ENGINE STATISTICS\n")
	out.WriteString("---------------------\n\n")

	stats := engine.GetStats()
	out.WriteString(fmt.Sprintf("Total drafts: %d\n", stats.TotalDrafts))
	out.WriteString(fmt.Sprintf("Pending drafts: %d\n", stats.PendingDrafts))
	out.WriteString(fmt.Sprintf("Commerce drafts: %d\n", stats.CommerceDrafts))
	out.WriteString("\n")

	out.WriteString("================================================================================\n")
	out.WriteString("                         DEMO COMPLETE\n")
	out.WriteString("================================================================================\n")

	return DemoResult{Output: out.String()}
}
