// Package draft extension for finance payment draft content types.
//
// Payment drafts are proposals for financial payments that require:
// 1. Explicit user approval
// 2. Multi-party household approval (if in intersection)
// 3. Policy snapshot binding (v9.12)
// 4. View snapshot binding (v9.13)
// 5. Pre-defined payee (v9.10 - no free-text recipients)
//
// CRITICAL: Drafts only. The draft does NOT execute the payment.
// CRITICAL: Execution requires separate explicit action via Finance Executor.
// CRITICAL: All payments go through Finance Execution Boundary.
// CRITICAL: Mock providers NEVER move real money.
//
// Phase 17: Finance Execution Boundary (Sandbox→Live) + Household Approvals
// Reference: docs/ADR/ADR-0033-phase17-finance-execution-boundary.md
package draft

import (
	"fmt"
)

// Payment draft type constant.
const (
	DraftTypePayment DraftType = "payment"
)

// PaymentDraftContent holds payment draft content.
//
// CRITICAL: PayeeID must be a pre-defined payee from the payee registry.
// Free-text recipients are NOT allowed (v9.10).
type PaymentDraftContent struct {
	// PayeeID is the pre-defined payee identifier.
	// CRITICAL: Must be from allowed payee registry.
	// NOT free-text.
	PayeeID string

	// AmountCents is the payment amount in minor units (pence/cents).
	AmountCents int64

	// Currency is the ISO currency code (e.g., "GBP").
	Currency string

	// Description is the payment description/reference.
	Description string

	// SourceAccountID identifies the source account (if multiple).
	SourceAccountID string

	// ProviderHint indicates the payment provider (mock, truelayer, etc.).
	ProviderHint string

	// IntersectionID is set if this payment is in a shared context.
	IntersectionID string

	// RequiresMultiPartyApproval indicates if household approval is needed.
	RequiresMultiPartyApproval bool

	// ApprovalThreshold is the required approval count.
	ApprovalThreshold int

	// RequiredApproverCircleIDs lists required approver circles.
	RequiredApproverCircleIDs []string

	// EnvelopeID links to the execution envelope (set after envelope creation).
	EnvelopeID string

	// ActionHash is the deterministic action hash for approval binding.
	ActionHash string
}

// ContentType returns the draft type.
func (p PaymentDraftContent) ContentType() DraftType {
	return DraftTypePayment
}

// ActionClass returns the commerce action class (payment).
func (p PaymentDraftContent) ActionClass() CommerceActionClass {
	return ActionClassPayment
}

// VendorName returns the payee ID as the vendor name.
func (p PaymentDraftContent) VendorName() string {
	return p.PayeeID
}

// OrderReference returns the envelope ID as the order reference.
func (p PaymentDraftContent) OrderReference() string {
	return p.EnvelopeID
}

// CanonicalString returns a deterministic string representation.
func (p PaymentDraftContent) CanonicalString() string {
	// Sort required approvers for determinism
	approvers := sortedStrings(p.RequiredApproverCircleIDs)
	approverList := joinStrings(approvers, ",")

	multiParty := "false"
	if p.RequiresMultiPartyApproval {
		multiParty = "true"
	}

	return fmt.Sprintf("payment|payee:%s|amount:%d|currency:%s|description:%s|source:%s|provider:%s|intersection:%s|multi_party:%s|threshold:%d|approvers:%s|envelope:%s|action_hash:%s",
		normalizeForCanonical(p.PayeeID),
		p.AmountCents,
		normalizeForCanonical(p.Currency),
		normalizeForCanonical(p.Description),
		normalizeForCanonical(p.SourceAccountID),
		normalizeForCanonical(p.ProviderHint),
		normalizeForCanonical(p.IntersectionID),
		multiParty,
		p.ApprovalThreshold,
		approverList,
		normalizeForCanonical(p.EnvelopeID),
		normalizeForCanonical(p.ActionHash),
	)
}

// AmountFormatted returns the amount in human-readable format.
func (p PaymentDraftContent) AmountFormatted() string {
	// Format based on currency
	switch p.Currency {
	case "GBP":
		pounds := p.AmountCents / 100
		pence := p.AmountCents % 100
		return fmt.Sprintf("£%d.%02d", pounds, pence)
	case "USD":
		dollars := p.AmountCents / 100
		cents := p.AmountCents % 100
		return fmt.Sprintf("$%d.%02d", dollars, cents)
	case "EUR":
		euros := p.AmountCents / 100
		cents := p.AmountCents % 100
		return fmt.Sprintf("€%d.%02d", euros, cents)
	default:
		return fmt.Sprintf("%d %s", p.AmountCents, p.Currency)
	}
}

// IsPaymentDraft returns true if the draft type is a payment type.
func IsPaymentDraft(draftType DraftType) bool {
	return draftType == DraftTypePayment
}

// PaymentContent returns the content as PaymentDraftContent if applicable.
func (d *Draft) PaymentContent() (PaymentDraftContent, bool) {
	if d.DraftType != DraftTypePayment {
		return PaymentDraftContent{}, false
	}
	if content, ok := d.Content.(PaymentDraftContent); ok {
		return content, true
	}
	return PaymentDraftContent{}, false
}

// Verify interface compliance.
var _ DraftContent = PaymentDraftContent{}
var _ CommerceDraftContent = PaymentDraftContent{}

// sortedStrings returns a sorted copy of the slice.
// stdlib-only bubble sort for determinism.
func sortedStrings(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// joinStrings joins strings with a separator.
func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += sep + s[i]
	}
	return result
}
