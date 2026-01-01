// Package execrouter routes approved drafts to execution intents.
//
// The router converts an approved Draft into an ExecutionIntent that can
// be executed via the appropriate boundary executor (Phase 5 calendar,
// Phase 7 email).
//
// CRITICAL: Only approved drafts can be routed.
// CRITICAL: PolicySnapshotHash and ViewSnapshotHash MUST be present.
// CRITICAL: No external writes occur in this package.
//
// Reference: Phase 10 - Approved Draft → Execution Routing
package execrouter

import (
	"fmt"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/execintent"
	"quantumlife/pkg/events"
)

// Router builds ExecutionIntents from approved drafts.
type Router struct {
	clock   clock.Clock
	emitter events.Emitter
}

// NewRouter creates a new execution router.
func NewRouter(clk clock.Clock, emitter events.Emitter) *Router {
	return &Router{
		clock:   clk,
		emitter: emitter,
	}
}

// BuildIntentFromDraft creates an ExecutionIntent from an approved draft.
// Returns an error if:
// - Draft is not approved
// - Required hashes are missing
// - Draft type is not executable
func (r *Router) BuildIntentFromDraft(d *draft.Draft) (*execintent.ExecutionIntent, error) {
	now := r.clock.Now()

	// Validate draft is approved
	if d.Status != draft.StatusApproved {
		r.emitEvent(events.Phase10ExecutionBlockedNotApproved, d, "draft not approved")
		return nil, fmt.Errorf("draft not approved: status=%s", d.Status)
	}

	// Validate policy snapshot hash is present
	if d.PolicySnapshotHash == "" {
		r.emitEvent(events.Phase10PolicyHashMissing, d, "PolicySnapshotHash missing")
		return nil, fmt.Errorf("PolicySnapshotHash missing from draft")
	}

	// Validate view snapshot hash is present
	if d.ViewSnapshotHash == "" {
		r.emitEvent(events.Phase10ViewHashMissing, d, "ViewSnapshotHash missing")
		return nil, fmt.Errorf("ViewSnapshotHash missing from draft")
	}

	// Build intent based on draft type
	intent, err := r.buildIntent(d, now)
	if err != nil {
		r.emitEvent(events.Phase10IntentValidationError, d, err.Error())
		return nil, err
	}

	// Finalize the intent (compute ID and hash)
	intent.Finalize()

	// Validate the intent
	if err := intent.Validate(); err != nil {
		r.emitEvent(events.Phase10IntentValidationError, d, err.Error())
		return nil, fmt.Errorf("intent validation failed: %w", err)
	}

	r.emitEvent(events.Phase10IntentBuilt, d, "")
	r.emitEvent(events.Phase10IntentValidated, d, "")

	return intent, nil
}

// buildIntent creates the ExecutionIntent based on draft content type.
func (r *Router) buildIntent(d *draft.Draft, now time.Time) (*execintent.ExecutionIntent, error) {
	base := &execintent.ExecutionIntent{
		DraftID:            d.DraftID,
		CircleID:           string(d.CircleID),
		PolicySnapshotHash: d.PolicySnapshotHash,
		ViewSnapshotHash:   d.ViewSnapshotHash,
		CreatedAt:          now,
	}

	switch d.DraftType {
	case draft.DraftTypeEmailReply:
		return r.buildEmailIntent(base, d)

	case draft.DraftTypeCalendarResponse:
		return r.buildCalendarIntent(base, d)

	case draft.DraftTypeShipmentFollowUp,
		draft.DraftTypeRefundFollowUp,
		draft.DraftTypeInvoiceReminder,
		draft.DraftTypeSubscriptionReview:
		// Commerce drafts become email sends
		return r.buildCommerceEmailIntent(base, d)

	case draft.DraftTypePayment:
		// Finance payment drafts route to finance execution boundary
		return r.buildFinanceIntent(base, d)

	default:
		return nil, fmt.Errorf("unsupported draft type for execution: %s", d.DraftType)
	}
}

// buildEmailIntent builds an email send intent from an email draft.
func (r *Router) buildEmailIntent(base *execintent.ExecutionIntent, d *draft.Draft) (*execintent.ExecutionIntent, error) {
	content, ok := d.EmailContent()
	if !ok {
		return nil, fmt.Errorf("expected EmailDraftContent for email_reply draft")
	}

	base.Action = execintent.ActionEmailSend
	base.EmailThreadID = content.ThreadID
	base.EmailMessageID = content.InReplyToMessageID
	base.EmailTo = content.To
	base.EmailSubject = content.Subject
	base.EmailBody = content.Body

	// Ensure we have thread context
	if content.ThreadID == "" && content.InReplyToMessageID == "" {
		return nil, fmt.Errorf("email draft missing thread/message context for reply")
	}

	return base, nil
}

// buildCalendarIntent builds a calendar respond intent from a calendar draft.
func (r *Router) buildCalendarIntent(base *execintent.ExecutionIntent, d *draft.Draft) (*execintent.ExecutionIntent, error) {
	content, ok := d.CalendarContent()
	if !ok {
		return nil, fmt.Errorf("expected CalendarDraftContent for calendar_response draft")
	}

	base.Action = execintent.ActionCalendarRespond
	base.CalendarEventID = content.EventID
	base.CalendarResponse = string(content.Response)

	if content.EventID == "" {
		return nil, fmt.Errorf("calendar draft missing EventID")
	}

	return base, nil
}

// buildCommerceEmailIntent builds an email send intent from a commerce draft.
// Commerce drafts (follow-ups, reminders, reviews) become email sends.
func (r *Router) buildCommerceEmailIntent(base *execintent.ExecutionIntent, d *draft.Draft) (*execintent.ExecutionIntent, error) {
	base.Action = execintent.ActionEmailSend

	switch content := d.Content.(type) {
	case draft.ShipmentFollowUpContent:
		base.EmailTo = content.VendorContact.Email()
		base.EmailSubject = content.Subject
		base.EmailBody = content.Body
		// Use order ID as thread reference for commerce emails
		base.EmailThreadID = fmt.Sprintf("commerce-shipment-%s", content.OrderID)

	case draft.RefundFollowUpContent:
		base.EmailTo = content.VendorContact.Email()
		base.EmailSubject = content.Subject
		base.EmailBody = content.Body
		base.EmailThreadID = fmt.Sprintf("commerce-refund-%s", content.OrderID)

	case draft.InvoiceReminderContent:
		base.EmailTo = content.VendorContact.Email()
		base.EmailSubject = content.Subject
		base.EmailBody = content.Body
		base.EmailThreadID = fmt.Sprintf("commerce-invoice-%s", content.InvoiceID)

	case draft.SubscriptionReviewContent:
		base.EmailTo = content.VendorContact.Email()
		base.EmailSubject = content.Subject
		base.EmailBody = content.Body
		base.EmailThreadID = fmt.Sprintf("commerce-subscription-%s", content.SubscriptionID)

	default:
		return nil, fmt.Errorf("unsupported commerce content type: %T", content)
	}

	// Commerce emails require a known vendor contact
	if base.EmailTo == "" {
		return nil, fmt.Errorf("commerce draft has unknown vendor contact - cannot execute email")
	}

	return base, nil
}

// buildFinanceIntent builds a finance payment intent from a payment draft.
// CRITICAL: All finance payments flow through the Finance Execution Boundary.
// Phase 17b: Routes to V96Executor.
func (r *Router) buildFinanceIntent(base *execintent.ExecutionIntent, d *draft.Draft) (*execintent.ExecutionIntent, error) {
	content, ok := d.PaymentContent()
	if !ok {
		return nil, fmt.Errorf("expected PaymentDraftContent for payment draft")
	}

	base.Action = execintent.ActionFinancePayment

	// CRITICAL: PayeeID must be non-empty (pre-defined payee, not free-text)
	if content.PayeeID == "" {
		return nil, fmt.Errorf("payment draft missing PayeeID (pre-defined payees only)")
	}
	base.FinancePayeeID = content.PayeeID

	// Validate amount
	if content.AmountCents <= 0 {
		return nil, fmt.Errorf("payment draft has invalid AmountCents: %d", content.AmountCents)
	}
	base.FinanceAmountCents = content.AmountCents

	// Currency defaults to GBP if not specified
	if content.Currency == "" {
		base.FinanceCurrency = "GBP"
	} else {
		base.FinanceCurrency = content.Currency
	}

	base.FinanceDescription = content.Description
	base.FinanceEnvelopeID = content.EnvelopeID
	base.FinanceActionHash = content.ActionHash

	return base, nil
}

// emitEvent emits an event with draft context.
func (r *Router) emitEvent(eventType events.EventType, d *draft.Draft, reason string) {
	if r.emitter == nil {
		return
	}

	payload := map[string]string{
		"draft_id":   string(d.DraftID),
		"draft_type": string(d.DraftType),
		"circle_id":  string(d.CircleID),
	}
	if reason != "" {
		payload["reason"] = reason
	}

	r.emitter.Emit(events.Event{
		Type:      eventType,
		Timestamp: r.clock.Now(),
		CircleID:  string(d.CircleID),
		SubjectID: string(d.DraftID),
	})
}
