// Package commerce implements commerce draft generation.
//
// Generates drafts for commerce-derived obligations:
// - Shipment follow-up (track order status)
// - Refund follow-up (check refund status)
// - Invoice reminder (payment due)
// - Subscription review (renewal notification)
//
// CRITICAL: Drafts only. NO external writes, NO payments, NO sends.
// CRITICAL: Deterministic. Same inputs + clock = same drafts.
// CRITICAL: Vendor-agnostic. Uses canonical CommerceEvent fields only.
//
// Reference: docs/ADR/ADR-0025-phase9-commerce-action-drafts.md
package commerce

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/obligation"
)

// Engine generates commerce action drafts from commerce obligations.
type Engine struct {
	rules draft.CommerceRules
}

// NewEngine creates a new commerce draft engine.
func NewEngine(rules draft.CommerceRules) *Engine {
	return &Engine{
		rules: rules,
	}
}

// NewDefaultEngine creates an engine with default rules.
func NewDefaultEngine() *Engine {
	return NewEngine(draft.DefaultCommerceRules())
}

// CanHandle returns true if this engine handles the obligation.
func (e *Engine) CanHandle(obl *obligation.Obligation) bool {
	if obl == nil {
		return false
	}
	// Handle commerce obligations
	return obl.SourceType == "commerce"
}

// Generate creates a commerce draft from an obligation.
func (e *Engine) Generate(ctx draft.GenerationContext) draft.GenerationResult {
	if !e.CanHandle(ctx.Obligation) {
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: "obligation not from commerce source",
		}
	}

	// Check minimum regret score
	if ctx.Obligation.RegretScore < e.rules.MinRegretForDraft {
		return draft.GenerationResult{
			Skipped: true,
			SkipReason: fmt.Sprintf("regret score %.2f below threshold %.2f",
				ctx.Obligation.RegretScore, e.rules.MinRegretForDraft),
		}
	}

	// Extract commerce context from obligation evidence
	commerceCtx := e.extractCommerceContext(ctx.Obligation, ctx.Now)

	// Route to specific generator based on obligation type
	switch ctx.Obligation.Type {
	case obligation.ObligationFollowup:
		// Could be shipment or refund follow-up
		if commerceCtx.TrackingID != "" || commerceCtx.ShipmentStatus != "" {
			return e.generateShipmentFollowUp(ctx, commerceCtx)
		}
		// Check if it's a refund follow-up (has amount and "refund" in reason)
		if strings.Contains(strings.ToLower(ctx.Obligation.Reason), "refund") {
			return e.generateRefundFollowUp(ctx, commerceCtx)
		}
		// Default to shipment follow-up
		return e.generateShipmentFollowUp(ctx, commerceCtx)

	case obligation.ObligationPay:
		return e.generateInvoiceReminder(ctx, commerceCtx)

	case obligation.ObligationReview:
		// Could be subscription review or large order review
		if strings.Contains(strings.ToLower(ctx.Obligation.Reason), "subscription") ||
			strings.Contains(strings.ToLower(ctx.Obligation.Reason), "renewal") {
			return e.generateSubscriptionReview(ctx, commerceCtx)
		}
		// For large order review, skip draft (user can review in commerce mirror)
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: "large order review does not require draft",
		}

	default:
		return draft.GenerationResult{
			Skipped:    true,
			SkipReason: fmt.Sprintf("obligation type %s not handled for commerce", ctx.Obligation.Type),
		}
	}
}

// extractCommerceContext extracts commerce-specific context from obligation evidence.
func (e *Engine) extractCommerceContext(obl *obligation.Obligation, now time.Time) draft.CommerceContext {
	ctx := draft.CommerceContext{
		EventDate: obl.CreatedAt,
	}

	// Vendor from evidence
	if v, ok := obl.Evidence["vendor"]; ok {
		ctx.Vendor = v
	}

	// Order ID from evidence
	if v, ok := obl.Evidence["order_id"]; ok {
		ctx.OrderID = v
	}

	// Tracking ID from evidence
	if v, ok := obl.Evidence["tracking_id"]; ok {
		ctx.TrackingID = v
	}

	// Invoice ID from evidence
	if v, ok := obl.Evidence["invoice_id"]; ok {
		ctx.InvoiceID = v
	}

	// Shipment status from evidence
	if v, ok := obl.Evidence["status"]; ok {
		ctx.ShipmentStatus = v
	}

	// Amount from evidence (parse from formatted string like "£24.99")
	if v, ok := obl.Evidence[obligation.EvidenceKeyAmount]; ok {
		ctx.AmountCents, ctx.Currency = parseFormattedAmount(v)
	}

	// Due date from obligation
	if obl.DueBy != nil {
		ctx.DueDate = *obl.DueBy
		ctx.IsOverdue = now.After(*obl.DueBy)
	}

	return ctx
}

// generateShipmentFollowUp generates a shipment tracking follow-up draft.
func (e *Engine) generateShipmentFollowUp(ctx draft.GenerationContext, commerceCtx draft.CommerceContext) draft.GenerationResult {
	// Build subject
	var subject strings.Builder
	subject.WriteString("Where is my order")
	if commerceCtx.OrderID != "" {
		subject.WriteString(fmt.Sprintf(" %s", commerceCtx.OrderID))
	}
	if commerceCtx.Vendor != "" {
		subject.WriteString(fmt.Sprintf(" from %s", commerceCtx.Vendor))
	}
	subject.WriteString("?")

	// Build body
	var body strings.Builder
	body.WriteString("Hello,\n\n")
	body.WriteString("I am writing to enquire about the status of my order.\n\n")

	if commerceCtx.OrderID != "" {
		body.WriteString(fmt.Sprintf("Order Reference: %s\n", commerceCtx.OrderID))
	}
	if commerceCtx.TrackingID != "" {
		body.WriteString(fmt.Sprintf("Tracking Number: %s\n", commerceCtx.TrackingID))
	}
	if !commerceCtx.EventDate.IsZero() {
		body.WriteString(fmt.Sprintf("Order Date: %s\n", commerceCtx.EventDate.Format("2006-01-02")))
	}
	if commerceCtx.AmountCents > 0 {
		body.WriteString(fmt.Sprintf("Amount: %s\n", formatAmount(commerceCtx.AmountCents, commerceCtx.Currency)))
	}
	if commerceCtx.ShipmentStatus != "" {
		body.WriteString(fmt.Sprintf("Last Known Status: %s\n", commerceCtx.ShipmentStatus))
	}

	body.WriteString("\nPlease provide an update on the delivery status.\n\n")
	body.WriteString("Thank you for your assistance.\n")

	// Create content
	content := draft.ShipmentFollowUpContent{
		Vendor:          commerceCtx.Vendor,
		VendorContact:   e.deriveVendorContact(commerceCtx),
		OrderID:         commerceCtx.OrderID,
		TrackingID:      commerceCtx.TrackingID,
		ShipmentStatus:  commerceCtx.ShipmentStatus,
		Subject:         subject.String(),
		Body:            body.String(),
		OrderDate:       commerceCtx.EventDate.Format("2006-01-02"),
		AmountFormatted: formatAmount(commerceCtx.AmountCents, commerceCtx.Currency),
	}

	return e.buildDraft(ctx, draft.DraftTypeShipmentFollowUp, content, "commerce-shipment-followup")
}

// generateRefundFollowUp generates a refund status follow-up draft.
func (e *Engine) generateRefundFollowUp(ctx draft.GenerationContext, commerceCtx draft.CommerceContext) draft.GenerationResult {
	// Build subject
	var subject strings.Builder
	subject.WriteString("Refund status enquiry")
	if commerceCtx.OrderID != "" {
		subject.WriteString(fmt.Sprintf(" - Order %s", commerceCtx.OrderID))
	}
	if commerceCtx.Vendor != "" {
		subject.WriteString(fmt.Sprintf(" from %s", commerceCtx.Vendor))
	}

	// Build body
	var body strings.Builder
	body.WriteString("Hello,\n\n")
	body.WriteString("I am writing to enquire about the status of a refund.\n\n")

	if commerceCtx.OrderID != "" {
		body.WriteString(fmt.Sprintf("Order Reference: %s\n", commerceCtx.OrderID))
	}
	if !commerceCtx.EventDate.IsZero() {
		body.WriteString(fmt.Sprintf("Refund Requested: %s\n", commerceCtx.EventDate.Format("2006-01-02")))
	}
	if commerceCtx.AmountCents > 0 {
		body.WriteString(fmt.Sprintf("Expected Refund Amount: %s\n", formatAmount(commerceCtx.AmountCents, commerceCtx.Currency)))
	}

	body.WriteString("\nI have not yet received confirmation that this refund has been processed.\n")
	body.WriteString("Please confirm when the refund will be credited to my account.\n\n")
	body.WriteString("Thank you for your assistance.\n")

	// Create content
	content := draft.RefundFollowUpContent{
		Vendor:          commerceCtx.Vendor,
		VendorContact:   e.deriveVendorContact(commerceCtx),
		OrderID:         commerceCtx.OrderID,
		Subject:         subject.String(),
		Body:            body.String(),
		RefundDate:      commerceCtx.EventDate.Format("2006-01-02"),
		AmountFormatted: formatAmount(commerceCtx.AmountCents, commerceCtx.Currency),
	}

	return e.buildDraft(ctx, draft.DraftTypeRefundFollowUp, content, "commerce-refund-followup")
}

// generateInvoiceReminder generates an invoice/payment reminder draft.
func (e *Engine) generateInvoiceReminder(ctx draft.GenerationContext, commerceCtx draft.CommerceContext) draft.GenerationResult {
	// Build subject
	var subject strings.Builder
	if commerceCtx.IsOverdue {
		subject.WriteString("OVERDUE: ")
	}
	subject.WriteString("Payment reminder")
	if commerceCtx.InvoiceID != "" {
		subject.WriteString(fmt.Sprintf(" - Invoice %s", commerceCtx.InvoiceID))
	} else if commerceCtx.OrderID != "" {
		subject.WriteString(fmt.Sprintf(" - Order %s", commerceCtx.OrderID))
	}
	if commerceCtx.Vendor != "" {
		subject.WriteString(fmt.Sprintf(" from %s", commerceCtx.Vendor))
	}

	// Build body
	var body strings.Builder
	body.WriteString("Hello,\n\n")

	if commerceCtx.IsOverdue {
		body.WriteString("This is a reminder that the following payment is now overdue.\n\n")
	} else {
		body.WriteString("This is a reminder about an upcoming payment.\n\n")
	}

	if commerceCtx.Vendor != "" {
		body.WriteString(fmt.Sprintf("Vendor: %s\n", commerceCtx.Vendor))
	}
	if commerceCtx.InvoiceID != "" {
		body.WriteString(fmt.Sprintf("Invoice Number: %s\n", commerceCtx.InvoiceID))
	}
	if commerceCtx.OrderID != "" {
		body.WriteString(fmt.Sprintf("Order Reference: %s\n", commerceCtx.OrderID))
	}
	if commerceCtx.AmountCents > 0 {
		body.WriteString(fmt.Sprintf("Amount Due: %s\n", formatAmount(commerceCtx.AmountCents, commerceCtx.Currency)))
	}
	if !commerceCtx.DueDate.IsZero() {
		body.WriteString(fmt.Sprintf("Due Date: %s\n", commerceCtx.DueDate.Format("2006-01-02")))
	}

	body.WriteString("\nPlease confirm payment has been made or arrange payment.\n\n")
	body.WriteString("Thank you.\n")

	// Create content
	content := draft.InvoiceReminderContent{
		Vendor:          commerceCtx.Vendor,
		VendorContact:   e.deriveVendorContact(commerceCtx),
		InvoiceID:       commerceCtx.InvoiceID,
		OrderID:         commerceCtx.OrderID,
		Subject:         subject.String(),
		Body:            body.String(),
		InvoiceDate:     commerceCtx.EventDate.Format("2006-01-02"),
		DueDate:         commerceCtx.DueDate.Format("2006-01-02"),
		AmountFormatted: formatAmount(commerceCtx.AmountCents, commerceCtx.Currency),
		IsOverdue:       commerceCtx.IsOverdue,
	}

	return e.buildDraft(ctx, draft.DraftTypeInvoiceReminder, content, "commerce-invoice-reminder")
}

// generateSubscriptionReview generates a subscription review/cancel draft.
func (e *Engine) generateSubscriptionReview(ctx draft.GenerationContext, commerceCtx draft.CommerceContext) draft.GenerationResult {
	// Build subject
	var subject strings.Builder
	subject.WriteString("Subscription renewal review")
	if commerceCtx.Vendor != "" {
		subject.WriteString(fmt.Sprintf(" - %s", commerceCtx.Vendor))
	}

	// Build body
	var body strings.Builder
	body.WriteString("Hello,\n\n")
	body.WriteString("Your subscription has recently renewed. Please review the details below.\n\n")

	if commerceCtx.Vendor != "" {
		body.WriteString(fmt.Sprintf("Service: %s\n", commerceCtx.Vendor))
	}
	if commerceCtx.SubscriptionID != "" {
		body.WriteString(fmt.Sprintf("Subscription ID: %s\n", commerceCtx.SubscriptionID))
	}
	if !commerceCtx.EventDate.IsZero() {
		body.WriteString(fmt.Sprintf("Renewal Date: %s\n", commerceCtx.EventDate.Format("2006-01-02")))
	}
	if commerceCtx.AmountCents > 0 {
		body.WriteString(fmt.Sprintf("Amount Charged: %s\n", formatAmount(commerceCtx.AmountCents, commerceCtx.Currency)))
	}

	body.WriteString("\nIf you wish to cancel this subscription, please contact the vendor.\n\n")
	body.WriteString("No action is required if you wish to continue.\n")

	// Create content
	content := draft.SubscriptionReviewContent{
		Vendor:          commerceCtx.Vendor,
		VendorContact:   e.deriveVendorContact(commerceCtx),
		SubscriptionID:  commerceCtx.SubscriptionID,
		Action:          "review",
		Subject:         subject.String(),
		Body:            body.String(),
		RenewalDate:     commerceCtx.EventDate.Format("2006-01-02"),
		NextRenewalDate: "", // Not known from current data
		AmountFormatted: formatAmount(commerceCtx.AmountCents, commerceCtx.Currency),
	}

	return e.buildDraft(ctx, draft.DraftTypeSubscriptionReview, content, "commerce-subscription-review")
}

// buildDraft assembles the final draft structure.
func (e *Engine) buildDraft(
	ctx draft.GenerationContext,
	draftType draft.DraftType,
	content draft.DraftContent,
	ruleID string,
) draft.GenerationResult {
	// Compute content hash
	contentHash := hashContent(content.CanonicalString())

	// Compute draft ID
	draftID := draft.ComputeDraftID(
		draftType,
		ctx.CircleID,
		ctx.Obligation.ID,
		contentHash,
	)

	// Compute expiry (use commerce TTL)
	expiresAt := ctx.Now.Add(time.Duration(e.rules.DefaultTTLHours) * time.Hour)

	// Build safety notes
	safetyNotes := e.buildSafetyNotes(content, ctx.Obligation)

	// Assemble draft
	d := draft.Draft{
		DraftID:            draftID,
		DraftType:          draftType,
		CircleID:           ctx.CircleID,
		IntersectionID:     ctx.IntersectionID,
		SourceObligationID: ctx.Obligation.ID,
		SourceEventIDs:     []string{ctx.Obligation.SourceEventID},
		CreatedAt:          ctx.Now,
		ExpiresAt:          expiresAt,
		Status:             draft.StatusProposed,
		Content:            content,
		SafetyNotes:        safetyNotes,
		DeterministicHash:  hashContent(content.CanonicalString() + ctx.Now.UTC().Format(time.RFC3339)),
		GenerationRuleID:   ruleID,
	}

	return draft.GenerationResult{
		Draft: &d,
	}
}

// deriveVendorContact derives a vendor contact reference.
// Returns a known contact if email can be inferred, otherwise returns unknown placeholder.
func (e *Engine) deriveVendorContact(ctx draft.CommerceContext) draft.VendorContactRef {
	// If we have a vendor domain, try to derive support email
	if ctx.VendorDomain != "" {
		// Common support email patterns
		supportEmail := "support@" + ctx.VendorDomain
		return draft.KnownVendorContact(supportEmail)
	}

	// If we have vendor name, create a deterministic hash for the placeholder
	if ctx.Vendor != "" {
		vendorHash := hashContent(strings.ToLower(ctx.Vendor))[:12]
		return draft.UnknownVendorContact(vendorHash)
	}

	return draft.UnknownVendorContact("unknown")
}

// buildSafetyNotes generates safety notes for commerce drafts.
func (e *Engine) buildSafetyNotes(content draft.DraftContent, obl *obligation.Obligation) []string {
	var notes []string

	// Check for commerce-specific content
	switch c := content.(type) {
	case draft.ShipmentFollowUpContent:
		if !c.VendorContact.IsKnown() {
			notes = append(notes, "Vendor contact is unknown - verify recipient before sending")
		}
		if c.TrackingID == "" && c.OrderID == "" {
			notes = append(notes, "No order or tracking reference - may need to add details")
		}

	case draft.RefundFollowUpContent:
		if !c.VendorContact.IsKnown() {
			notes = append(notes, "Vendor contact is unknown - verify recipient before sending")
		}
		notes = append(notes, "Verify refund has not already been received before sending")

	case draft.InvoiceReminderContent:
		if c.IsOverdue {
			notes = append(notes, "Invoice is overdue - review payment status carefully")
		}
		notes = append(notes, "Verify payment has not already been made before proceeding")

	case draft.SubscriptionReviewContent:
		notes = append(notes, "Review if you still need this subscription")
		if c.AmountFormatted != "" {
			notes = append(notes, fmt.Sprintf("Current charge: %s", c.AmountFormatted))
		}
	}

	// Check severity
	if obl.Severity == obligation.SeverityCritical || obl.Severity == obligation.SeverityHigh {
		notes = append(notes, "Flagged as high priority - review carefully")
	}

	// Sort for determinism
	sort.Strings(notes)

	return notes
}

// parseFormattedAmount parses a formatted amount string like "£24.99" or "$50.00".
// Returns amount in cents and currency code.
func parseFormattedAmount(formatted string) (int64, string) {
	if formatted == "" {
		return 0, ""
	}

	// Determine currency and extract number
	var currency string
	var numStr string

	formatted = strings.TrimSpace(formatted)

	// Check for currency symbols
	switch {
	case strings.HasPrefix(formatted, "£"):
		currency = "GBP"
		numStr = strings.TrimPrefix(formatted, "£")
	case strings.HasPrefix(formatted, "$"):
		currency = "USD"
		numStr = strings.TrimPrefix(formatted, "$")
	case strings.HasPrefix(formatted, "€"):
		currency = "EUR"
		numStr = strings.TrimPrefix(formatted, "€")
	case strings.HasPrefix(formatted, "₹"):
		currency = "INR"
		numStr = strings.TrimPrefix(formatted, "₹")
	case strings.HasPrefix(formatted, "Rs."):
		currency = "INR"
		numStr = strings.TrimPrefix(formatted, "Rs.")
	default:
		// Try ISO code prefix
		if len(formatted) >= 3 {
			prefix := formatted[:3]
			switch prefix {
			case "GBP", "USD", "EUR", "INR":
				currency = prefix
				numStr = strings.TrimSpace(formatted[3:])
			default:
				return 0, ""
			}
		} else {
			return 0, ""
		}
	}

	// Remove commas and parse
	numStr = strings.TrimSpace(numStr)
	numStr = strings.ReplaceAll(numStr, ",", "")

	// Parse as float and convert to cents
	amount, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, ""
	}

	cents := int64(amount * 100)
	return cents, currency
}

// formatAmount formats cents to a human-readable string.
func formatAmount(cents int64, currency string) string {
	if cents == 0 {
		return ""
	}

	major := float64(cents) / 100.0

	switch currency {
	case "GBP":
		return fmt.Sprintf("£%.2f", major)
	case "USD":
		return fmt.Sprintf("$%.2f", major)
	case "EUR":
		return fmt.Sprintf("€%.2f", major)
	case "INR":
		return fmt.Sprintf("₹%.2f", major)
	default:
		return fmt.Sprintf("%.2f %s", major, currency)
	}
}

// hashContent returns a SHA256 hash of content.
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// Verify interface compliance.
var _ draft.DraftGenerator = (*Engine)(nil)
