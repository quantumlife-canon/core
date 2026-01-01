// Package events defines canonical event models for read-only ingestion.
//
// These events represent data ingested from external sources (Gmail, Calendar, Finance)
// transformed into a vendor-agnostic canonical format.
//
// CRITICAL: This package is READ-ONLY. No write/execution events are defined here.
// Write events belong in pkg/events (the audit event system).
//
// Reference: docs/TECH_SPEC_V1.md
package events

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"quantumlife/pkg/domain/identity"
)

// EventType identifies the kind of canonical event.
type EventType string

const (
	EventTypeEmailMessage  EventType = "email_message"
	EventTypeCalendarEvent EventType = "calendar_event"
	EventTypeTransaction   EventType = "transaction"
	EventTypeBalance       EventType = "balance"
)

// CanonicalEvent is the base interface for all ingested events.
type CanonicalEvent interface {
	// EventID returns the deterministic event ID.
	EventID() string

	// EventType returns the type of event.
	EventType() EventType

	// SourceVendor returns the vendor that produced this event.
	SourceVendor() string

	// SourceID returns the original ID in the source system.
	SourceID() string

	// CapturedAt returns when this event was captured (ingestion time).
	CapturedAt() time.Time

	// OccurredAt returns when the event actually occurred.
	OccurredAt() time.Time

	// CircleID returns the suggested circle for this event.
	CircleID() identity.EntityID

	// EntityRefs returns references to identity graph entities.
	EntityRefs() []identity.EntityRef

	// CanonicalString returns the string used for ID generation.
	CanonicalString() string
}

// BaseEvent provides common fields for all canonical events.
type BaseEvent struct {
	ID        string               `json:"event_id"`
	Type      EventType            `json:"event_type"`
	Vendor    string               `json:"source_vendor"`
	Source    string               `json:"source_id"`
	Captured  time.Time            `json:"captured_at"`
	Occurred  time.Time            `json:"occurred_at"`
	Circle    identity.EntityID    `json:"circle_id"`
	Entities  []identity.EntityRef `json:"entity_refs"`
	Canonical string               `json:"-"` // Not serialized
}

func (e *BaseEvent) EventID() string                  { return e.ID }
func (e *BaseEvent) EventType() EventType             { return e.Type }
func (e *BaseEvent) SourceVendor() string             { return e.Vendor }
func (e *BaseEvent) SourceID() string                 { return e.Source }
func (e *BaseEvent) CapturedAt() time.Time            { return e.Captured }
func (e *BaseEvent) OccurredAt() time.Time            { return e.Occurred }
func (e *BaseEvent) CircleID() identity.EntityID      { return e.Circle }
func (e *BaseEvent) EntityRefs() []identity.EntityRef { return e.Entities }
func (e *BaseEvent) CanonicalString() string          { return e.Canonical }

// SetCircleID sets the circle ID for this event.
// Used during ingestion to assign events to circles based on routing rules.
func (e *BaseEvent) SetCircleID(circleID identity.EntityID) { e.Circle = circleID }

// generateEventID creates a deterministic event ID from type and canonical string.
func generateEventID(eventType EventType, canonicalStr string) string {
	hash := sha256.Sum256([]byte(canonicalStr))
	return fmt.Sprintf("%s_%s", eventType, hex.EncodeToString(hash[:])[:16])
}

// EmailMessageEvent represents an ingested email message.
type EmailMessageEvent struct {
	BaseEvent

	// Message identification
	MessageID string `json:"message_id"`
	ThreadID  string `json:"thread_id,omitempty"`

	// Account info
	AccountEmail string `json:"account_email"`
	Folder       string `json:"folder"` // INBOX, SENT, etc.

	// Participants
	From    EmailAddress   `json:"from"`
	To      []EmailAddress `json:"to"`
	Cc      []EmailAddress `json:"cc,omitempty"`
	ReplyTo *EmailAddress  `json:"reply_to,omitempty"`

	// Content (preview only - full body not stored)
	Subject     string `json:"subject"`
	BodyPreview string `json:"body_preview"` // First 500 chars

	// Metadata
	HasAttachments  bool     `json:"has_attachments"`
	AttachmentCount int      `json:"attachment_count"`
	Labels          []string `json:"labels,omitempty"`

	// Flags
	IsRead      bool `json:"is_read"`
	IsStarred   bool `json:"is_starred"`
	IsImportant bool `json:"is_important"`

	// Classification hints
	SenderDomain    string `json:"sender_domain"`
	IsAutomated     bool   `json:"is_automated"`     // Newsletters, notifications
	IsTransactional bool   `json:"is_transactional"` // Receipts, confirmations
}

// EmailAddress represents an email participant.
type EmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

// NewEmailMessageEvent creates an EmailMessageEvent with deterministic ID.
func NewEmailMessageEvent(
	vendor string,
	messageID string,
	accountEmail string,
	capturedAt time.Time,
	occurredAt time.Time,
) *EmailMessageEvent {
	canonicalStr := fmt.Sprintf("email:%s:%s:%s", vendor, accountEmail, messageID)

	return &EmailMessageEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(EventTypeEmailMessage, canonicalStr),
			Type:      EventTypeEmailMessage,
			Vendor:    vendor,
			Source:    messageID,
			Captured:  capturedAt,
			Occurred:  occurredAt,
			Canonical: canonicalStr,
		},
		MessageID:    messageID,
		AccountEmail: accountEmail,
	}
}

// CalendarEventEvent represents an ingested calendar event.
type CalendarEventEvent struct {
	BaseEvent

	// Calendar info
	CalendarID   string `json:"calendar_id"`
	CalendarName string `json:"calendar_name"`
	AccountEmail string `json:"account_email"`

	// Event identification
	EventUID     string `json:"event_uid"` // iCal UID
	RecurrenceID string `json:"recurrence_id,omitempty"`

	// Content
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`

	// Timing
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	IsAllDay  bool      `json:"is_all_day"`
	Timezone  string    `json:"timezone"`

	// Recurrence
	IsRecurring    bool   `json:"is_recurring"`
	RecurrenceRule string `json:"recurrence_rule,omitempty"`

	// Attendees
	Organizer        *CalendarAttendee  `json:"organizer,omitempty"`
	Attendees        []CalendarAttendee `json:"attendees,omitempty"`
	AttendeeCount    int                `json:"attendee_count"`
	MyResponseStatus RSVPStatus         `json:"my_response_status"`

	// Metadata
	IsCancelled   bool   `json:"is_cancelled"`
	IsBusy        bool   `json:"is_busy"`
	ConferenceURL string `json:"conference_url,omitempty"`
}

// CalendarAttendee represents a calendar event participant.
type CalendarAttendee struct {
	Email          string     `json:"email"`
	Name           string     `json:"name,omitempty"`
	ResponseStatus RSVPStatus `json:"response_status"`
	IsOptional     bool       `json:"is_optional"`
}

// RSVPStatus represents calendar response status.
type RSVPStatus string

const (
	RSVPNeedsAction RSVPStatus = "NEEDS_ACTION"
	RSVPAccepted    RSVPStatus = "ACCEPTED"
	RSVPDeclined    RSVPStatus = "DECLINED"
	RSVPTentative   RSVPStatus = "TENTATIVE"
)

// NewCalendarEventEvent creates a CalendarEventEvent with deterministic ID.
func NewCalendarEventEvent(
	vendor string,
	calendarID string,
	eventUID string,
	accountEmail string,
	capturedAt time.Time,
	occurredAt time.Time,
) *CalendarEventEvent {
	canonicalStr := fmt.Sprintf("calendar:%s:%s:%s", vendor, calendarID, eventUID)

	return &CalendarEventEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(EventTypeCalendarEvent, canonicalStr),
			Type:      EventTypeCalendarEvent,
			Vendor:    vendor,
			Source:    eventUID,
			Captured:  capturedAt,
			Occurred:  occurredAt,
			Canonical: canonicalStr,
		},
		CalendarID:   calendarID,
		EventUID:     eventUID,
		AccountEmail: accountEmail,
	}
}

// TransactionEvent represents an ingested financial transaction.
type TransactionEvent struct {
	BaseEvent

	// Account info
	AccountID    string `json:"account_id"`
	AccountType  string `json:"account_type"` // CHECKING, SAVINGS, CREDIT
	Institution  string `json:"institution"`
	MaskedNumber string `json:"masked_number"`

	// Transaction details
	TransactionType   string `json:"transaction_type"`   // DEBIT, CREDIT
	TransactionKind   string `json:"transaction_kind"`   // PURCHASE, REFUND, TRANSFER, FEE
	TransactionStatus string `json:"transaction_status"` // PENDING, POSTED

	// Amount (always in minor units - pence/cents)
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"` // ISO 4217

	// Counterparty
	MerchantName     string `json:"merchant_name"`     // Normalized
	MerchantNameRaw  string `json:"merchant_name_raw"` // Original from bank
	MerchantCategory string `json:"merchant_category,omitempty"`

	// Dates
	TransactionDate time.Time  `json:"transaction_date"`
	PostedDate      *time.Time `json:"posted_date,omitempty"`

	// Reference
	Reference string `json:"reference,omitempty"`
}

// NewTransactionEvent creates a TransactionEvent with deterministic ID.
func NewTransactionEvent(
	vendor string,
	accountID string,
	transactionID string,
	capturedAt time.Time,
	occurredAt time.Time,
) *TransactionEvent {
	canonicalStr := fmt.Sprintf("transaction:%s:%s:%s", vendor, accountID, transactionID)

	return &TransactionEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(EventTypeTransaction, canonicalStr),
			Type:      EventTypeTransaction,
			Vendor:    vendor,
			Source:    transactionID,
			Captured:  capturedAt,
			Occurred:  occurredAt,
			Canonical: canonicalStr,
		},
		AccountID: accountID,
	}
}

// BalanceEvent represents an account balance snapshot.
type BalanceEvent struct {
	BaseEvent

	// Account info
	AccountID    string `json:"account_id"`
	AccountType  string `json:"account_type"`
	Institution  string `json:"institution"`
	MaskedNumber string `json:"masked_number"`

	// Balances (always in minor units)
	CurrentMinor   int64  `json:"current_minor"`
	AvailableMinor int64  `json:"available_minor"`
	Currency       string `json:"currency"`

	// Credit-specific (optional)
	CreditLimitMinor *int64 `json:"credit_limit_minor,omitempty"`

	// Timestamp of balance
	AsOf time.Time `json:"as_of"`
}

// NewBalanceEvent creates a BalanceEvent with deterministic ID.
func NewBalanceEvent(
	vendor string,
	accountID string,
	capturedAt time.Time,
	asOf time.Time,
) *BalanceEvent {
	// Balance events include timestamp in canonical string since they're point-in-time
	canonicalStr := fmt.Sprintf("balance:%s:%s:%d", vendor, accountID, asOf.Unix())

	return &BalanceEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(EventTypeBalance, canonicalStr),
			Type:      EventTypeBalance,
			Vendor:    vendor,
			Source:    accountID,
			Captured:  capturedAt,
			Occurred:  asOf,
			Canonical: canonicalStr,
		},
		AccountID: accountID,
		AsOf:      asOf,
	}
}

// EventStore provides storage for canonical events.
type EventStore interface {
	// Store saves an event.
	Store(event CanonicalEvent) error

	// GetByID retrieves an event by ID.
	GetByID(id string) (CanonicalEvent, error)

	// GetByCircle returns events for a circle, ordered by occurred time.
	GetByCircle(circleID identity.EntityID, eventType *EventType, limit int) ([]CanonicalEvent, error)

	// GetByTimeRange returns events in a time range.
	GetByTimeRange(start, end time.Time, eventType *EventType) ([]CanonicalEvent, error)

	// Count returns total event count.
	Count() int

	// CountByType returns count for a specific type.
	CountByType(eventType EventType) int
}

// Verify interface compliance.
var (
	_ CanonicalEvent = (*EmailMessageEvent)(nil)
	_ CanonicalEvent = (*CalendarEventEvent)(nil)
	_ CanonicalEvent = (*TransactionEvent)(nil)
	_ CanonicalEvent = (*BalanceEvent)(nil)
)
