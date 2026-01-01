// Package draft - generator interfaces for draft generation engines.
package draft

import (
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// GenerationContext provides context for draft generation.
type GenerationContext struct {
	// CircleID identifies the owning circle.
	CircleID identity.EntityID

	// IntersectionID is optional for shared contexts.
	IntersectionID identity.EntityID

	// Obligation is the source obligation triggering this draft.
	Obligation *obligation.Obligation

	// Now is the current time (injected clock).
	Now time.Time

	// Policy is the draft policy to use.
	Policy DraftPolicy
}

// GenerationResult contains the outcome of draft generation.
type GenerationResult struct {
	// Draft is the generated draft (nil if none generated).
	Draft *Draft

	// Skipped indicates the generation was intentionally skipped.
	Skipped bool

	// SkipReason explains why generation was skipped.
	SkipReason string

	// Error indicates a generation failure.
	Error error
}

// DraftGenerator generates drafts from obligations.
type DraftGenerator interface {
	// CanHandle returns true if this generator handles the obligation type.
	CanHandle(obl *obligation.Obligation) bool

	// Generate creates a draft from an obligation.
	// Returns nil Draft if no draft should be generated.
	Generate(ctx GenerationContext) GenerationResult
}

// EmailContext provides email-specific context for generation.
type EmailContext struct {
	// ThreadID is the email thread being replied to.
	ThreadID string

	// InReplyToMessageID is the message being replied to.
	InReplyToMessageID string

	// OriginalFrom is who sent the original email.
	OriginalFrom string

	// OriginalTo is who the email was sent to.
	OriginalTo []string

	// OriginalSubject is the subject line.
	OriginalSubject string

	// OriginalBody is the original email body.
	OriginalBody string

	// ProviderHint indicates the email provider (gmail, outlook, etc.).
	ProviderHint string
}

// CalendarContext provides calendar-specific context for generation.
type CalendarContext struct {
	// EventID is the calendar event being responded to.
	EventID string

	// OrganizerEmail is who organized the event.
	OrganizerEmail string

	// EventTitle is the event title.
	EventTitle string

	// EventStart is the event start time.
	EventStart time.Time

	// EventEnd is the event end time.
	EventEnd time.Time

	// ProviderHint indicates the calendar provider (google, outlook, etc.).
	ProviderHint string

	// CalendarID identifies the specific calendar.
	CalendarID string
}

// ReplyRules defines deterministic rules for email reply generation.
type ReplyRules struct {
	// MinimumBodyLength is the minimum email body to include.
	MinimumBodyLength int

	// IncludeSubjectPrefix adds "Re: " prefix if not present.
	IncludeSubjectPrefix bool

	// DefaultSignature is appended to all replies.
	DefaultSignature string
}

// DefaultReplyRules returns sensible defaults for email replies.
func DefaultReplyRules() ReplyRules {
	return ReplyRules{
		MinimumBodyLength:    1,
		IncludeSubjectPrefix: true,
		DefaultSignature:     "",
	}
}

// CalendarRules defines deterministic rules for calendar response generation.
type CalendarRules struct {
	// AutoAcceptFromContacts auto-accepts events from known contacts.
	AutoAcceptFromContacts bool

	// AutoDeclineConflicts auto-declines conflicting events.
	AutoDeclineConflicts bool

	// DefaultResponse is used when no specific rule matches.
	DefaultResponse CalendarResponse
}

// DefaultCalendarRules returns sensible defaults for calendar responses.
func DefaultCalendarRules() CalendarRules {
	return CalendarRules{
		AutoAcceptFromContacts: false,
		AutoDeclineConflicts:   false,
		DefaultResponse:        CalendarResponseTentative,
	}
}

// CommerceContext provides commerce-specific context for draft generation.
type CommerceContext struct {
	// Vendor is the canonical merchant name.
	Vendor string

	// VendorDomain is the vendor's email domain (e.g., "amazon.co.uk").
	VendorDomain string

	// OrderID is the order reference (may be empty).
	OrderID string

	// TrackingID is the shipment tracking number (may be empty).
	TrackingID string

	// InvoiceID is the invoice reference (may be empty).
	InvoiceID string

	// SubscriptionID is the subscription reference (may be empty).
	SubscriptionID string

	// AmountCents is the amount in minor currency units.
	AmountCents int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// ShipmentStatus is the current shipment status.
	ShipmentStatus string

	// EventDate is the date the commerce event occurred.
	EventDate time.Time

	// DueDate is when payment/action is due (may be zero).
	DueDate time.Time

	// SourceMessageID is the email message ID that triggered this.
	SourceMessageID string

	// IsOverdue indicates if the obligation is past due.
	IsOverdue bool
}

// CommerceRules defines deterministic rules for commerce draft generation.
type CommerceRules struct {
	// DefaultTTLHours is the TTL for commerce drafts.
	DefaultTTLHours int

	// MinRegretForDraft is the minimum regret score to generate a draft.
	MinRegretForDraft float64

	// IncludeAmountInSubject includes the amount in email subjects.
	IncludeAmountInSubject bool
}

// DefaultCommerceRules returns sensible defaults for commerce drafts.
func DefaultCommerceRules() CommerceRules {
	return CommerceRules{
		DefaultTTLHours:        72,
		MinRegretForDraft:      0.0, // Generate for all commerce obligations
		IncludeAmountInSubject: true,
	}
}
