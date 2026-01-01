// Package obligations extends obligation extraction with commerce event rules.
//
// Commerce obligations are derived from CommerceEvents extracted from emails.
// These include shipment tracking, invoice payment, subscription review, and refunds.
//
// CRITICAL: Uses injected clock, never time.Now().
// CRITICAL: Deterministic rule application.
//
// Reference: docs/ADR/ADR-0024-phase8-commerce-mirror-email-derived.md
package obligations

import (
	"fmt"
	"time"

	"quantumlife/pkg/domain/commerce"
	"quantumlife/pkg/domain/obligation"
)

// CommerceConfig holds commerce-specific obligation settings.
type CommerceConfig struct {
	// ShipmentTrackingDays is how many days after dispatch to create tracking obligations
	ShipmentTrackingDays int

	// InvoiceDefaultDueDays is the default days until invoice is due if not specified
	InvoiceDefaultDueDays int

	// SubscriptionReviewDays is days after renewal to create review obligation
	SubscriptionReviewDays int

	// RefundPendingDays is days after refund to check status
	RefundPendingDays int

	// LargeAmountThreshold triggers higher regret (in cents)
	LargeAmountThreshold int64

	// RegretScores for different commerce obligations
	ShipmentPendingRegret     float64
	InvoiceDueRegret          float64
	SubscriptionRenewalRegret float64
	RefundPendingRegret       float64
	LargeOrderRegret          float64
}

// DefaultCommerceConfig returns sensible defaults.
func DefaultCommerceConfig() CommerceConfig {
	return CommerceConfig{
		ShipmentTrackingDays:      3,
		InvoiceDefaultDueDays:     7,
		SubscriptionReviewDays:    2,
		RefundPendingDays:         5,
		LargeAmountThreshold:      10000, // £100 / $100
		ShipmentPendingRegret:     0.45,
		InvoiceDueRegret:          0.70,
		SubscriptionRenewalRegret: 0.40,
		RefundPendingRegret:       0.55,
		LargeOrderRegret:          0.50,
	}
}

// CommerceObligationExtractor extracts obligations from commerce events.
type CommerceObligationExtractor struct {
	config CommerceConfig
}

// NewCommerceObligationExtractor creates a new commerce obligation extractor.
func NewCommerceObligationExtractor(config CommerceConfig) *CommerceObligationExtractor {
	return &CommerceObligationExtractor{config: config}
}

// ExtractFromCommerceEvents extracts obligations from commerce events.
// Returns obligations sorted deterministically.
func (e *CommerceObligationExtractor) ExtractFromCommerceEvents(
	events []*commerce.CommerceEvent,
	now time.Time,
) []*obligation.Obligation {
	var result []*obligation.Obligation

	for _, event := range events {
		obligs := e.extractFromEvent(event, now)
		result = append(result, obligs...)
	}

	// Sort deterministically
	obligation.SortObligations(result)

	return result
}

// extractFromEvent applies rules to a single commerce event.
func (e *CommerceObligationExtractor) extractFromEvent(
	event *commerce.CommerceEvent,
	now time.Time,
) []*obligation.Obligation {
	var result []*obligation.Obligation

	switch event.Type {
	case commerce.EventShipmentUpdate:
		if oblig := e.handleShipmentUpdate(event, now); oblig != nil {
			result = append(result, oblig)
		}

	case commerce.EventInvoiceIssued:
		if oblig := e.handleInvoice(event, now); oblig != nil {
			result = append(result, oblig)
		}

	case commerce.EventSubscriptionRenewed:
		if oblig := e.handleSubscriptionRenewal(event, now); oblig != nil {
			result = append(result, oblig)
		}

	case commerce.EventRefundIssued:
		if oblig := e.handleRefund(event, now); oblig != nil {
			result = append(result, oblig)
		}

	case commerce.EventOrderPlaced:
		if oblig := e.handleOrderPlaced(event, now); oblig != nil {
			result = append(result, oblig)
		}
	}

	return result
}

// handleShipmentUpdate creates tracking obligation for pending shipments.
func (e *CommerceObligationExtractor) handleShipmentUpdate(
	event *commerce.CommerceEvent,
	now time.Time,
) *obligation.Obligation {
	// Only create obligations for pending shipments
	if !event.HasPendingShipment() {
		return nil
	}

	// Calculate due date (expected delivery)
	dueDate := event.OccurredAt.Add(time.Duration(e.config.ShipmentTrackingDays) * 24 * time.Hour)

	// Skip if already past due (shipment should have arrived)
	if now.After(dueDate.Add(24 * time.Hour)) {
		return nil
	}

	oblig := obligation.NewObligation(
		event.CircleID,
		event.EventID,
		"commerce",
		obligation.ObligationFollowup,
		event.OccurredAt,
	)

	regret := e.config.ShipmentPendingRegret
	// Increase regret if out for delivery (imminent)
	if event.ShipmentStatus == commerce.ShipmentOutDelivery {
		regret += 0.15
	}

	oblig.WithDueBy(dueDate, now).
		WithScoring(regret, 0.80).
		WithReason(fmt.Sprintf("Track shipment from %s", event.Vendor)).
		WithEvidence("vendor", event.Vendor).
		WithEvidence("status", string(event.ShipmentStatus)).
		WithSeverity(obligation.SeverityLow)

	if event.TrackingID != "" {
		oblig.WithEvidence("tracking_id", event.TrackingID)
	}

	return oblig
}

// handleInvoice creates payment obligation for invoices.
func (e *CommerceObligationExtractor) handleInvoice(
	event *commerce.CommerceEvent,
	now time.Time,
) *obligation.Obligation {
	// Calculate due date
	dueDate := event.OccurredAt.Add(time.Duration(e.config.InvoiceDefaultDueDays) * 24 * time.Hour)

	// Skip if too far in the past
	if now.After(dueDate.Add(30 * 24 * time.Hour)) {
		return nil
	}

	oblig := obligation.NewObligation(
		event.CircleID,
		event.EventID,
		"commerce",
		obligation.ObligationPay,
		event.OccurredAt,
	)

	regret := e.config.InvoiceDueRegret
	// Increase regret for large amounts
	if event.AmountCents >= e.config.LargeAmountThreshold {
		regret += 0.10
	}
	// Increase regret if overdue
	if now.After(dueDate) {
		regret += 0.15
	}

	// Clamp regret to 1.0
	if regret > 1.0 {
		regret = 1.0
	}

	severity := obligation.SeverityMedium
	if now.After(dueDate) {
		severity = obligation.SeverityHigh
	}

	oblig.WithDueBy(dueDate, now).
		WithScoring(regret, 0.85).
		WithReason(fmt.Sprintf("Pay invoice from %s", event.Vendor)).
		WithEvidence("vendor", event.Vendor).
		WithSeverity(severity)

	if event.AmountCents > 0 {
		oblig.WithEvidence(obligation.EvidenceKeyAmount, formatCents(event.AmountCents, event.Currency))
	}
	if event.OrderID != "" {
		oblig.WithEvidence("order_id", event.OrderID)
	}

	return oblig
}

// handleSubscriptionRenewal creates review obligation for renewals.
func (e *CommerceObligationExtractor) handleSubscriptionRenewal(
	event *commerce.CommerceEvent,
	now time.Time,
) *obligation.Obligation {
	// Calculate review deadline
	dueDate := event.OccurredAt.Add(time.Duration(e.config.SubscriptionReviewDays) * 24 * time.Hour)

	// Skip if too old
	if now.After(dueDate.Add(7 * 24 * time.Hour)) {
		return nil
	}

	oblig := obligation.NewObligation(
		event.CircleID,
		event.EventID,
		"commerce",
		obligation.ObligationReview,
		event.OccurredAt,
	)

	regret := e.config.SubscriptionRenewalRegret
	// Higher regret for expensive subscriptions
	if event.AmountCents >= e.config.LargeAmountThreshold {
		regret += 0.20
	}

	oblig.WithDueBy(dueDate, now).
		WithScoring(regret, 0.75).
		WithReason(fmt.Sprintf("Review %s subscription renewal", event.Vendor)).
		WithEvidence("vendor", event.Vendor).
		WithSeverity(obligation.SeverityLow)

	if event.AmountCents > 0 {
		oblig.WithEvidence(obligation.EvidenceKeyAmount, formatCents(event.AmountCents, event.Currency))
	}

	return oblig
}

// handleRefund creates follow-up obligation for refunds.
func (e *CommerceObligationExtractor) handleRefund(
	event *commerce.CommerceEvent,
	now time.Time,
) *obligation.Obligation {
	// Calculate check date
	checkDate := event.OccurredAt.Add(time.Duration(e.config.RefundPendingDays) * 24 * time.Hour)

	// Skip if too old
	if now.After(checkDate.Add(14 * 24 * time.Hour)) {
		return nil
	}

	// Only create obligation if past check date (refund should be settled)
	if now.Before(checkDate) {
		return nil
	}

	oblig := obligation.NewObligation(
		event.CircleID,
		event.EventID,
		"commerce",
		obligation.ObligationFollowup,
		event.OccurredAt,
	)

	regret := e.config.RefundPendingRegret
	// Higher regret for large refunds
	if event.AmountCents >= e.config.LargeAmountThreshold {
		regret += 0.15
	}

	oblig.WithDueBy(now.Add(24*time.Hour), now).
		WithScoring(regret, 0.70).
		WithReason(fmt.Sprintf("Check refund status from %s", event.Vendor)).
		WithEvidence("vendor", event.Vendor).
		WithSeverity(obligation.SeverityMedium)

	if event.AmountCents > 0 {
		oblig.WithEvidence(obligation.EvidenceKeyAmount, formatCents(event.AmountCents, event.Currency))
	}

	return oblig
}

// handleOrderPlaced creates review obligation for large orders.
func (e *CommerceObligationExtractor) handleOrderPlaced(
	event *commerce.CommerceEvent,
	now time.Time,
) *obligation.Obligation {
	// Only create obligations for large orders
	if event.AmountCents < e.config.LargeAmountThreshold {
		return nil
	}

	// Skip if too old (more than 2 days)
	if now.Sub(event.OccurredAt) > 48*time.Hour {
		return nil
	}

	oblig := obligation.NewObligation(
		event.CircleID,
		event.EventID,
		"commerce",
		obligation.ObligationReview,
		event.OccurredAt,
	)

	oblig.WithScoring(e.config.LargeOrderRegret, 0.80).
		WithReason(fmt.Sprintf("Review large order from %s", event.Vendor)).
		WithEvidence("vendor", event.Vendor).
		WithEvidence(obligation.EvidenceKeyAmount, formatCents(event.AmountCents, event.Currency)).
		WithSeverity(obligation.SeverityLow)

	if event.OrderID != "" {
		oblig.WithEvidence("order_id", event.OrderID)
	}

	return oblig
}

// formatCents formats cents to a human-readable string.
func formatCents(amountCents int64, currency string) string {
	major := float64(amountCents) / 100.0
	symbol := currencySymbol(currency)
	return fmt.Sprintf("%s%.2f", symbol, major)
}

// CommerceObligationType defines additional commerce-specific obligation types.
const (
	ObligationTrackShipment    obligation.ObligationType = "track_shipment"
	ObligationPayInvoice       obligation.ObligationType = "pay_invoice"
	ObligationReviewRenewal    obligation.ObligationType = "review_renewal"
	ObligationCheckRefund      obligation.ObligationType = "check_refund"
	ObligationReviewLargeOrder obligation.ObligationType = "review_large_order"
)
