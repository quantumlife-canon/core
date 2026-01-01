// Package draft extension for commerce draft content types.
//
// Commerce drafts are proposals for commerce-related actions:
// - Shipment follow-up (track/enquire about order)
// - Refund follow-up (check refund status)
// - Invoice reminder (email vendor or prepare payment)
// - Subscription review/cancel request
//
// CRITICAL: Drafts only. NO external writes, NO payments, NO sends.
// CRITICAL: Deterministic. Same inputs + clock = same drafts.
// CRITICAL: Vendor-agnostic. Uses canonical CommerceEvent fields only.
//
// Reference: docs/ADR/ADR-0025-phase9-commerce-action-drafts.md
package draft

import (
	"fmt"
	"strings"
)

// Commerce draft type constants.
const (
	DraftTypeShipmentFollowUp   DraftType = "shipment_followup"
	DraftTypeRefundFollowUp     DraftType = "refund_followup"
	DraftTypeInvoiceReminder    DraftType = "invoice_reminder"
	DraftTypeSubscriptionReview DraftType = "subscription_review"
)

// CommerceActionClass categorizes commerce draft actions.
type CommerceActionClass string

const (
	ActionClassShipment     CommerceActionClass = "shipment"
	ActionClassRefund       CommerceActionClass = "refund"
	ActionClassPayment      CommerceActionClass = "payment"
	ActionClassSubscription CommerceActionClass = "subscription"
)

// VendorContactRef is a deterministic vendor contact reference.
// NOT free-text. Either derived from event fields or a canonical placeholder.
type VendorContactRef string

// UnknownVendorContact creates a deterministic placeholder for unknown contacts.
func UnknownVendorContact(vendorHash string) VendorContactRef {
	return VendorContactRef(fmt.Sprintf("vendor-contact:unknown:%s", vendorHash))
}

// KnownVendorContact creates a contact ref from a known email.
func KnownVendorContact(email string) VendorContactRef {
	return VendorContactRef(fmt.Sprintf("vendor-contact:email:%s", email))
}

// IsKnown returns true if the contact is a known email.
func (v VendorContactRef) IsKnown() bool {
	return strings.HasPrefix(string(v), "vendor-contact:email:")
}

// Email returns the email address if known, empty string otherwise.
func (v VendorContactRef) Email() string {
	if !v.IsKnown() {
		return ""
	}
	return strings.TrimPrefix(string(v), "vendor-contact:email:")
}

// CommerceDraftContent is the base interface for all commerce draft content types.
type CommerceDraftContent interface {
	DraftContent
	ActionClass() CommerceActionClass
	VendorName() string
	OrderReference() string
}

// ShipmentFollowUpContent holds shipment follow-up draft content.
type ShipmentFollowUpContent struct {
	// Vendor is the canonical merchant name.
	Vendor string

	// VendorContact is the deterministic contact reference.
	VendorContact VendorContactRef

	// OrderID is the order reference (may be empty).
	OrderID string

	// TrackingID is the shipment tracking number.
	TrackingID string

	// ShipmentStatus is the current known status.
	ShipmentStatus string

	// Subject is the email subject line.
	Subject string

	// Body is the email body text.
	Body string

	// OrderDate is the ISO-8601 date of the original order.
	OrderDate string

	// AmountFormatted is the human-readable amount (e.g., "£24.99").
	AmountFormatted string
}

// ContentType returns the draft type.
func (s ShipmentFollowUpContent) ContentType() DraftType {
	return DraftTypeShipmentFollowUp
}

// ActionClass returns the commerce action class.
func (s ShipmentFollowUpContent) ActionClass() CommerceActionClass {
	return ActionClassShipment
}

// VendorName returns the vendor name.
func (s ShipmentFollowUpContent) VendorName() string {
	return s.Vendor
}

// OrderReference returns the order ID or tracking ID.
func (s ShipmentFollowUpContent) OrderReference() string {
	if s.OrderID != "" {
		return s.OrderID
	}
	return s.TrackingID
}

// CanonicalString returns a deterministic string representation.
func (s ShipmentFollowUpContent) CanonicalString() string {
	return fmt.Sprintf("shipment_followup|vendor:%s|contact:%s|order:%s|tracking:%s|status:%s|subject:%s|body:%s|date:%s|amount:%s",
		normalizeForCanonical(s.Vendor),
		string(s.VendorContact),
		normalizeForCanonical(s.OrderID),
		normalizeForCanonical(s.TrackingID),
		normalizeForCanonical(s.ShipmentStatus),
		normalizeForCanonical(s.Subject),
		normalizeForCanonical(s.Body),
		s.OrderDate,
		normalizeForCanonical(s.AmountFormatted),
	)
}

// RefundFollowUpContent holds refund follow-up draft content.
type RefundFollowUpContent struct {
	// Vendor is the canonical merchant name.
	Vendor string

	// VendorContact is the deterministic contact reference.
	VendorContact VendorContactRef

	// OrderID is the order reference.
	OrderID string

	// Subject is the email subject line.
	Subject string

	// Body is the email body text.
	Body string

	// RefundDate is the ISO-8601 date when refund was issued.
	RefundDate string

	// AmountFormatted is the refund amount (e.g., "£15.00").
	AmountFormatted string
}

// ContentType returns the draft type.
func (r RefundFollowUpContent) ContentType() DraftType {
	return DraftTypeRefundFollowUp
}

// ActionClass returns the commerce action class.
func (r RefundFollowUpContent) ActionClass() CommerceActionClass {
	return ActionClassRefund
}

// VendorName returns the vendor name.
func (r RefundFollowUpContent) VendorName() string {
	return r.Vendor
}

// OrderReference returns the order ID.
func (r RefundFollowUpContent) OrderReference() string {
	return r.OrderID
}

// CanonicalString returns a deterministic string representation.
func (r RefundFollowUpContent) CanonicalString() string {
	return fmt.Sprintf("refund_followup|vendor:%s|contact:%s|order:%s|subject:%s|body:%s|date:%s|amount:%s",
		normalizeForCanonical(r.Vendor),
		string(r.VendorContact),
		normalizeForCanonical(r.OrderID),
		normalizeForCanonical(r.Subject),
		normalizeForCanonical(r.Body),
		r.RefundDate,
		normalizeForCanonical(r.AmountFormatted),
	)
}

// InvoiceReminderContent holds invoice/payment reminder draft content.
type InvoiceReminderContent struct {
	// Vendor is the canonical merchant name.
	Vendor string

	// VendorContact is the deterministic contact reference.
	VendorContact VendorContactRef

	// InvoiceID is the invoice reference.
	InvoiceID string

	// OrderID is the related order reference (may be empty).
	OrderID string

	// Subject is the email subject line.
	Subject string

	// Body is the email body text.
	Body string

	// InvoiceDate is the ISO-8601 date when invoice was issued.
	InvoiceDate string

	// DueDate is the ISO-8601 date when payment is due.
	DueDate string

	// AmountFormatted is the invoice amount (e.g., "£120.00").
	AmountFormatted string

	// IsOverdue indicates if the invoice is past due.
	IsOverdue bool
}

// ContentType returns the draft type.
func (i InvoiceReminderContent) ContentType() DraftType {
	return DraftTypeInvoiceReminder
}

// ActionClass returns the commerce action class.
func (i InvoiceReminderContent) ActionClass() CommerceActionClass {
	return ActionClassPayment
}

// VendorName returns the vendor name.
func (i InvoiceReminderContent) VendorName() string {
	return i.Vendor
}

// OrderReference returns the invoice or order ID.
func (i InvoiceReminderContent) OrderReference() string {
	if i.InvoiceID != "" {
		return i.InvoiceID
	}
	return i.OrderID
}

// CanonicalString returns a deterministic string representation.
func (i InvoiceReminderContent) CanonicalString() string {
	overdue := "false"
	if i.IsOverdue {
		overdue = "true"
	}
	return fmt.Sprintf("invoice_reminder|vendor:%s|contact:%s|invoice:%s|order:%s|subject:%s|body:%s|invoice_date:%s|due_date:%s|amount:%s|overdue:%s",
		normalizeForCanonical(i.Vendor),
		string(i.VendorContact),
		normalizeForCanonical(i.InvoiceID),
		normalizeForCanonical(i.OrderID),
		normalizeForCanonical(i.Subject),
		normalizeForCanonical(i.Body),
		i.InvoiceDate,
		i.DueDate,
		normalizeForCanonical(i.AmountFormatted),
		overdue,
	)
}

// SubscriptionReviewContent holds subscription review/cancel draft content.
type SubscriptionReviewContent struct {
	// Vendor is the canonical merchant name (e.g., "Netflix").
	Vendor string

	// VendorContact is the deterministic contact reference.
	VendorContact VendorContactRef

	// SubscriptionID is the subscription reference (may be empty).
	SubscriptionID string

	// Action is the proposed action: "review", "cancel", "keep".
	Action string

	// Subject is the email subject line.
	Subject string

	// Body is the email body text.
	Body string

	// RenewalDate is the ISO-8601 date when renewal occurred.
	RenewalDate string

	// NextRenewalDate is the ISO-8601 date of next renewal (if known).
	NextRenewalDate string

	// AmountFormatted is the subscription amount (e.g., "£9.99/month").
	AmountFormatted string
}

// ContentType returns the draft type.
func (sub SubscriptionReviewContent) ContentType() DraftType {
	return DraftTypeSubscriptionReview
}

// ActionClass returns the commerce action class.
func (sub SubscriptionReviewContent) ActionClass() CommerceActionClass {
	return ActionClassSubscription
}

// VendorName returns the vendor name.
func (sub SubscriptionReviewContent) VendorName() string {
	return sub.Vendor
}

// OrderReference returns the subscription ID.
func (sub SubscriptionReviewContent) OrderReference() string {
	return sub.SubscriptionID
}

// CanonicalString returns a deterministic string representation.
func (sub SubscriptionReviewContent) CanonicalString() string {
	return fmt.Sprintf("subscription_review|vendor:%s|contact:%s|subscription:%s|action:%s|subject:%s|body:%s|renewal_date:%s|next_renewal:%s|amount:%s",
		normalizeForCanonical(sub.Vendor),
		string(sub.VendorContact),
		normalizeForCanonical(sub.SubscriptionID),
		normalizeForCanonical(sub.Action),
		normalizeForCanonical(sub.Subject),
		normalizeForCanonical(sub.Body),
		sub.RenewalDate,
		sub.NextRenewalDate,
		normalizeForCanonical(sub.AmountFormatted),
	)
}

// normalizeForCanonical normalizes a string for canonical representation.
// Lowercases and replaces problematic characters for deterministic hashing.
func normalizeForCanonical(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "|", "_")
	// Handle CRLF before LF to avoid double spacing
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// IsCommerceDraft returns true if the draft type is a commerce type.
func IsCommerceDraft(draftType DraftType) bool {
	switch draftType {
	case DraftTypeShipmentFollowUp, DraftTypeRefundFollowUp,
		DraftTypeInvoiceReminder, DraftTypeSubscriptionReview:
		return true
	default:
		return false
	}
}

// CommerceContent returns the content as CommerceDraftContent if applicable.
func (d *Draft) CommerceContent() (CommerceDraftContent, bool) {
	if !IsCommerceDraft(d.DraftType) {
		return nil, false
	}
	if content, ok := d.Content.(CommerceDraftContent); ok {
		return content, true
	}
	return nil, false
}

// ShipmentContent returns the content as ShipmentFollowUpContent if applicable.
func (d *Draft) ShipmentContent() (ShipmentFollowUpContent, bool) {
	if d.DraftType != DraftTypeShipmentFollowUp {
		return ShipmentFollowUpContent{}, false
	}
	if content, ok := d.Content.(ShipmentFollowUpContent); ok {
		return content, true
	}
	return ShipmentFollowUpContent{}, false
}

// RefundContent returns the content as RefundFollowUpContent if applicable.
func (d *Draft) RefundContent() (RefundFollowUpContent, bool) {
	if d.DraftType != DraftTypeRefundFollowUp {
		return RefundFollowUpContent{}, false
	}
	if content, ok := d.Content.(RefundFollowUpContent); ok {
		return content, true
	}
	return RefundFollowUpContent{}, false
}

// InvoiceContent returns the content as InvoiceReminderContent if applicable.
func (d *Draft) InvoiceContent() (InvoiceReminderContent, bool) {
	if d.DraftType != DraftTypeInvoiceReminder {
		return InvoiceReminderContent{}, false
	}
	if content, ok := d.Content.(InvoiceReminderContent); ok {
		return content, true
	}
	return InvoiceReminderContent{}, false
}

// SubscriptionContent returns the content as SubscriptionReviewContent if applicable.
func (d *Draft) SubscriptionContent() (SubscriptionReviewContent, bool) {
	if d.DraftType != DraftTypeSubscriptionReview {
		return SubscriptionReviewContent{}, false
	}
	if content, ok := d.Content.(SubscriptionReviewContent); ok {
		return content, true
	}
	return SubscriptionReviewContent{}, false
}
