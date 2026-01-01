// Package draft defines the domain model for draft proposals.
//
// Drafts are AI-generated proposals (email replies, calendar responses)
// that require explicit user approval before any external action.
//
// CRITICAL: Drafts are internal artifacts only. NO external writes.
// CRITICAL: Deterministic. Same inputs + clock = same drafts.
// CRITICAL: All drafts are audit logged via events.
//
// Reference: docs/ADR/ADR-0021-phase4-drafts-only-assistance.md
package draft

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// DraftID is a deterministic identifier for a draft.
type DraftID string

// DraftType indicates the kind of draft.
type DraftType string

const (
	DraftTypeEmailReply       DraftType = "email_reply"
	DraftTypeCalendarResponse DraftType = "calendar_response"
)

// DraftStatus tracks the lifecycle of a draft.
type DraftStatus string

const (
	StatusProposed DraftStatus = "proposed"
	StatusApproved DraftStatus = "approved"
	StatusRejected DraftStatus = "rejected"
	StatusExpired  DraftStatus = "expired"
)

// CalendarResponse indicates the type of calendar response.
type CalendarResponse string

const (
	CalendarResponseAccept         CalendarResponse = "accept"
	CalendarResponseDecline        CalendarResponse = "decline"
	CalendarResponseTentative      CalendarResponse = "tentative"
	CalendarResponseProposeNewTime CalendarResponse = "propose_new_time"
)

// Draft represents a proposed action that requires user approval.
type Draft struct {
	// DraftID is deterministic: sha256(canonical)[:16]
	DraftID DraftID

	// DraftType indicates email_reply or calendar_response.
	DraftType DraftType

	// CircleID identifies the owning circle.
	CircleID identity.EntityID

	// IntersectionID is optional for shared contexts.
	IntersectionID identity.EntityID

	// SourceObligationID links to the obligation that triggered this draft.
	SourceObligationID string

	// SourceEventIDs are the canonical event IDs this draft references.
	SourceEventIDs []string

	// CreatedAt is when the draft was generated (from clock).
	CreatedAt time.Time

	// ExpiresAt is when the draft becomes stale (CreatedAt + policy TTL).
	ExpiresAt time.Time

	// Status tracks the draft lifecycle.
	Status DraftStatus

	// StatusReason explains why status changed (e.g., rejection reason).
	StatusReason string

	// StatusChangedAt is when status last changed.
	StatusChangedAt time.Time

	// StatusChangedBy identifies who changed the status.
	StatusChangedBy string

	// Content holds the draft-type-specific content.
	// Use EmailContent() or CalendarContent() accessors.
	Content DraftContent

	// SafetyNotes explain safety considerations for this draft.
	SafetyNotes []string

	// DeterministicHash is sha256 of the canonical content representation.
	DeterministicHash string

	// GenerationRuleID identifies which rule generated this draft.
	GenerationRuleID string
}

// DraftContent is the interface for type-specific draft content.
type DraftContent interface {
	ContentType() DraftType
	CanonicalString() string
}

// EmailDraftContent holds email reply draft content.
type EmailDraftContent struct {
	// To is the recipient email address.
	To string

	// Cc are carbon copy recipients (optional).
	Cc []string

	// Subject is the email subject line.
	Subject string

	// Body is the email body text.
	Body string

	// ThreadID links to the email thread (for threading).
	ThreadID string

	// ProviderHint indicates the email provider (gmail, outlook, etc.).
	ProviderHint string

	// InReplyToMessageID is the message ID being replied to.
	InReplyToMessageID string
}

// ContentType returns the draft type for email content.
func (e EmailDraftContent) ContentType() DraftType {
	return DraftTypeEmailReply
}

// CanonicalString returns a deterministic string representation.
func (e EmailDraftContent) CanonicalString() string {
	// Sort Cc for determinism
	ccSorted := make([]string, len(e.Cc))
	copy(ccSorted, e.Cc)
	sort.Strings(ccSorted)

	return fmt.Sprintf("email|to:%s|cc:%s|subject:%s|body:%s|thread:%s|provider:%s|reply_to:%s",
		e.To,
		strings.Join(ccSorted, ","),
		e.Subject,
		e.Body,
		e.ThreadID,
		e.ProviderHint,
		e.InReplyToMessageID,
	)
}

// CalendarDraftContent holds calendar response draft content.
type CalendarDraftContent struct {
	// EventID is the calendar event being responded to.
	EventID string

	// Response is the response type (accept, decline, tentative, propose_new_time).
	Response CalendarResponse

	// Message is an optional message to include with the response.
	Message string

	// ProposedStart is for propose_new_time responses.
	ProposedStart *time.Time

	// ProposedEnd is for propose_new_time responses.
	ProposedEnd *time.Time

	// ProviderHint indicates the calendar provider (google, outlook, etc.).
	ProviderHint string

	// CalendarID identifies the specific calendar.
	CalendarID string
}

// ContentType returns the draft type for calendar content.
func (c CalendarDraftContent) ContentType() DraftType {
	return DraftTypeCalendarResponse
}

// CanonicalString returns a deterministic string representation.
func (c CalendarDraftContent) CanonicalString() string {
	proposedStart := ""
	proposedEnd := ""
	if c.ProposedStart != nil {
		proposedStart = c.ProposedStart.UTC().Format(time.RFC3339)
	}
	if c.ProposedEnd != nil {
		proposedEnd = c.ProposedEnd.UTC().Format(time.RFC3339)
	}

	return fmt.Sprintf("calendar|event:%s|response:%s|message:%s|proposed_start:%s|proposed_end:%s|provider:%s|calendar:%s",
		c.EventID,
		c.Response,
		c.Message,
		proposedStart,
		proposedEnd,
		c.ProviderHint,
		c.CalendarID,
	)
}

// EmailContent returns the content as EmailDraftContent if applicable.
func (d *Draft) EmailContent() (EmailDraftContent, bool) {
	if d.DraftType != DraftTypeEmailReply {
		return EmailDraftContent{}, false
	}
	if content, ok := d.Content.(EmailDraftContent); ok {
		return content, true
	}
	return EmailDraftContent{}, false
}

// CalendarContent returns the content as CalendarDraftContent if applicable.
func (d *Draft) CalendarContent() (CalendarDraftContent, bool) {
	if d.DraftType != DraftTypeCalendarResponse {
		return CalendarDraftContent{}, false
	}
	if content, ok := d.Content.(CalendarDraftContent); ok {
		return content, true
	}
	return CalendarDraftContent{}, false
}

// CanonicalString returns a deterministic string representation of the draft.
func (d *Draft) CanonicalString() string {
	// Sort SourceEventIDs for determinism
	eventIDs := make([]string, len(d.SourceEventIDs))
	copy(eventIDs, d.SourceEventIDs)
	sort.Strings(eventIDs)

	// Sort SafetyNotes for determinism
	safetyNotes := make([]string, len(d.SafetyNotes))
	copy(safetyNotes, d.SafetyNotes)
	sort.Strings(safetyNotes)

	contentCanonical := ""
	if d.Content != nil {
		contentCanonical = d.Content.CanonicalString()
	}

	return fmt.Sprintf("draft|type:%s|circle:%s|intersection:%s|obligation:%s|events:%s|created:%s|expires:%s|status:%s|content:{%s}|safety:%s|rule:%s",
		d.DraftType,
		d.CircleID,
		d.IntersectionID,
		d.SourceObligationID,
		strings.Join(eventIDs, ","),
		d.CreatedAt.UTC().Format(time.RFC3339),
		d.ExpiresAt.UTC().Format(time.RFC3339),
		d.Status,
		contentCanonical,
		strings.Join(safetyNotes, ";"),
		d.GenerationRuleID,
	)
}

// Hash returns the deterministic hash of the draft.
func (d *Draft) Hash() string {
	hash := sha256.Sum256([]byte(d.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// ComputeDraftID generates a deterministic DraftID from key fields.
func ComputeDraftID(draftType DraftType, circleID identity.EntityID, sourceObligationID string, contentHash string) DraftID {
	canonical := fmt.Sprintf("draftid|%s|%s|%s|%s",
		draftType,
		circleID,
		sourceObligationID,
		contentHash,
	)
	hash := sha256.Sum256([]byte(canonical))
	return DraftID(hex.EncodeToString(hash[:8])) // 16 hex chars
}

// IsExpired returns true if the draft has expired based on the given time.
func (d *Draft) IsExpired(now time.Time) bool {
	return now.After(d.ExpiresAt)
}

// IsTerminal returns true if the draft is in a terminal status.
func (d *Draft) IsTerminal() bool {
	return d.Status == StatusApproved || d.Status == StatusRejected || d.Status == StatusExpired
}

// CanTransitionTo returns true if the status transition is allowed.
func (d *Draft) CanTransitionTo(newStatus DraftStatus) bool {
	// Terminal states cannot transition
	if d.IsTerminal() {
		return false
	}

	// From proposed, can go to any terminal state
	if d.Status == StatusProposed {
		return newStatus == StatusApproved || newStatus == StatusRejected || newStatus == StatusExpired
	}

	return false
}

// DedupKey returns a key for deduplication purposes.
func (d *Draft) DedupKey() string {
	var typeSpecificKey string
	switch content := d.Content.(type) {
	case EmailDraftContent:
		typeSpecificKey = content.ThreadID
	case CalendarDraftContent:
		typeSpecificKey = content.EventID
	default:
		typeSpecificKey = ""
	}

	return fmt.Sprintf("%s|%s|%s|%s",
		d.DraftType,
		d.CircleID,
		d.SourceObligationID,
		typeSpecificKey,
	)
}

// StatusOrder returns the priority order for status (for sorting).
// Lower number = higher priority in lists.
func StatusOrder(status DraftStatus) int {
	switch status {
	case StatusProposed:
		return 0
	case StatusApproved:
		return 1
	case StatusRejected:
		return 2
	case StatusExpired:
		return 3
	default:
		return 99
	}
}

// SortDrafts sorts drafts deterministically:
// 1. Status priority (proposed first)
// 2. ExpiresAt ascending (soonest first)
// 3. CreatedAt ascending
// 4. DraftID ascending (stable tiebreaker)
func SortDrafts(drafts []Draft) {
	sort.Slice(drafts, func(i, j int) bool {
		// Status priority
		statusI := StatusOrder(drafts[i].Status)
		statusJ := StatusOrder(drafts[j].Status)
		if statusI != statusJ {
			return statusI < statusJ
		}

		// ExpiresAt ascending
		if !drafts[i].ExpiresAt.Equal(drafts[j].ExpiresAt) {
			return drafts[i].ExpiresAt.Before(drafts[j].ExpiresAt)
		}

		// CreatedAt ascending
		if !drafts[i].CreatedAt.Equal(drafts[j].CreatedAt) {
			return drafts[i].CreatedAt.Before(drafts[j].CreatedAt)
		}

		// DraftID ascending
		return drafts[i].DraftID < drafts[j].DraftID
	})
}
