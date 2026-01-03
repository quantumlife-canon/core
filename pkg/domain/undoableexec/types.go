// Package undoableexec provides domain types for undoable execution.
//
// Phase 25: First Undoable Execution (Opt-In, Single-Shot)
//
// This package defines the model for the first real external write that is
// meaningfully undoable. Phase 25 supports ONLY calendar_respond because:
//   - Email send is not truly undoable
//   - Finance is not undoable
//   - Calendar RSVP can be reversed by applying previous response
//
// CRITICAL INVARIANTS:
//   - Only calendar_respond is undoable
//   - Single-shot per period (max one execution)
//   - Undo window is bounded (bucketed time)
//   - Undo is a first-class flow, not "best effort"
//   - No identifiers stored (hashes only)
//   - No goroutines. No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0055-phase25-first-undoable-execution.md
package undoableexec

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// UndoableActionKind represents the type of undoable action.
// Phase 25 ONLY supports calendar_respond.
type UndoableActionKind string

const (
	// ActionKindCalendarRespond is the only undoable action in Phase 25.
	ActionKindCalendarRespond UndoableActionKind = "calendar_respond"
)

// IsSupported returns true if this action kind is supported for undo.
func (k UndoableActionKind) IsSupported() bool {
	return k == ActionKindCalendarRespond
}

// ResponseStatus represents the status of a calendar response.
// Must match the calendar provider response values.
type ResponseStatus string

const (
	StatusNeedsAction ResponseStatus = "needs_action"
	StatusAccepted    ResponseStatus = "accepted"
	StatusDeclined    ResponseStatus = "declined"
	StatusTentative   ResponseStatus = "tentative"
)

// UndoState represents the state of an undo record.
type UndoState string

const (
	// StatePending - execution not yet completed.
	StatePending UndoState = "pending"

	// StateExecuted - execution completed, undo not yet available.
	StateExecuted UndoState = "executed"

	// StateUndoAvailable - undo window is open.
	StateUndoAvailable UndoState = "undo_available"

	// StateUndone - undo was performed.
	StateUndone UndoState = "undone"

	// StateExpired - undo window expired without undo.
	StateExpired UndoState = "expired"
)

// UndoWindow defines the time bucket for undo availability.
// Uses 15-minute buckets for privacy (no exact timestamps).
type UndoWindow struct {
	// BucketStartRFC3339 is the start of the 15-minute bucket.
	// Format: "2006-01-02T15:04:00Z" (always :00 or :15 or :30 or :45)
	BucketStartRFC3339 string

	// BucketDurationMinutes is always 15.
	BucketDurationMinutes int
}

// NewUndoWindow creates an undo window from a timestamp.
// Rounds down to the nearest 15-minute bucket.
func NewUndoWindow(t time.Time) UndoWindow {
	// Round down to 15-minute bucket
	minute := t.Minute()
	bucketMinute := (minute / 15) * 15
	bucketStart := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), bucketMinute, 0, 0, time.UTC)

	return UndoWindow{
		BucketStartRFC3339:    bucketStart.Format(time.RFC3339),
		BucketDurationMinutes: 15,
	}
}

// DeadlineWindow returns the undo deadline window (one bucket after execution).
func (w UndoWindow) DeadlineWindow() UndoWindow {
	start, _ := time.Parse(time.RFC3339, w.BucketStartRFC3339)
	deadline := start.Add(time.Duration(w.BucketDurationMinutes) * time.Minute)
	return NewUndoWindow(deadline)
}

// IsExpired returns true if the current time is past the undo window.
func (w UndoWindow) IsExpired(now time.Time) bool {
	start, err := time.Parse(time.RFC3339, w.BucketStartRFC3339)
	if err != nil {
		return true // Treat parse errors as expired
	}
	deadline := start.Add(time.Duration(w.BucketDurationMinutes) * time.Minute)
	return now.After(deadline)
}

// UndoRecord captures the state needed to undo an execution.
//
// CRITICAL: No identifiers stored - only hashes and enums.
type UndoRecord struct {
	// ID is a deterministic hash of the record.
	ID string

	// PeriodKey is the daily bucket (e.g., "2024-01-15").
	PeriodKey string

	// CircleID identifies the circle.
	CircleID string

	// ActionKind is the type of undoable action.
	ActionKind UndoableActionKind

	// DraftID is the hash of the draft that was executed.
	DraftID string

	// EnvelopeID is the execution envelope ID.
	EnvelopeID string

	// BeforeStatus is the response status before execution.
	BeforeStatus ResponseStatus

	// AfterStatus is the response status after execution.
	AfterStatus ResponseStatus

	// UndoAvailableUntilBucket is when the undo window closes.
	UndoAvailableUntilBucket UndoWindow

	// State is the current state of the undo record.
	State UndoState

	// ExecutedAtBucket is when execution occurred (bucketed).
	ExecutedAtBucket UndoWindow
}

// NewUndoRecord creates a new undo record.
func NewUndoRecord(
	periodKey string,
	circleID string,
	actionKind UndoableActionKind,
	draftID string,
	envelopeID string,
	beforeStatus ResponseStatus,
	afterStatus ResponseStatus,
	executedAt time.Time,
) *UndoRecord {
	executedBucket := NewUndoWindow(executedAt)
	undoDeadline := executedBucket.DeadlineWindow()

	record := &UndoRecord{
		PeriodKey:                periodKey,
		CircleID:                 circleID,
		ActionKind:               actionKind,
		DraftID:                  draftID,
		EnvelopeID:               envelopeID,
		BeforeStatus:             beforeStatus,
		AfterStatus:              afterStatus,
		UndoAvailableUntilBucket: undoDeadline,
		State:                    StateUndoAvailable,
		ExecutedAtBucket:         executedBucket,
	}
	record.ID = record.Hash()
	return record
}

// CanonicalString returns the pipe-delimited canonical representation.
// Used for deterministic hashing.
func (r *UndoRecord) CanonicalString() string {
	parts := []string{
		"UNDO_RECORD",
		"v1",
		r.PeriodKey,
		r.CircleID,
		string(r.ActionKind),
		r.DraftID,
		r.EnvelopeID,
		string(r.BeforeStatus),
		string(r.AfterStatus),
		r.ExecutedAtBucket.BucketStartRFC3339,
	}
	return strings.Join(parts, "|")
}

// Hash returns the SHA-256 hash of the canonical string.
func (r *UndoRecord) Hash() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// IsUndoAvailable returns true if undo can be performed.
func (r *UndoRecord) IsUndoAvailable(now time.Time) bool {
	if r.State != StateUndoAvailable {
		return false
	}
	return !r.UndoAvailableUntilBucket.IsExpired(now)
}

// UndoAck represents an acknowledgement/state transition for an undo record.
// Appended to the store to record state changes.
type UndoAck struct {
	// RecordID is the undo record being acknowledged.
	RecordID string

	// NewState is the state being transitioned to.
	NewState UndoState

	// AckBucket is when the ack occurred (bucketed).
	AckBucket UndoWindow

	// Reason provides context for the transition.
	Reason string
}

// NewUndoAck creates a new undo acknowledgement.
func NewUndoAck(recordID string, newState UndoState, now time.Time, reason string) *UndoAck {
	return &UndoAck{
		RecordID:  recordID,
		NewState:  newState,
		AckBucket: NewUndoWindow(now),
		Reason:    reason,
	}
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *UndoAck) CanonicalString() string {
	parts := []string{
		"UNDO_ACK",
		"v1",
		a.RecordID,
		string(a.NewState),
		a.AckBucket.BucketStartRFC3339,
		a.Reason,
	}
	return strings.Join(parts, "|")
}

// Hash returns the SHA-256 hash of the canonical string.
func (a *UndoAck) Hash() string {
	hash := sha256.Sum256([]byte(a.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// ActionEligibility contains the eligibility status for undoable execution.
type ActionEligibility struct {
	// Eligible indicates if an undoable action is available.
	Eligible bool

	// Reason explains why eligible or not.
	Reason string

	// DraftID is the selected draft (if eligible).
	DraftID string

	// ActionKind is the type of action (if eligible).
	ActionKind UndoableActionKind

	// PeriodKey is the current period.
	PeriodKey string

	// CircleID is the circle.
	CircleID string
}

// UndoablePage represents the UI data for the undoable action page.
type UndoablePage struct {
	// Title is the page title.
	Title string

	// Subtitle is the page subtitle.
	Subtitle string

	// HasAction indicates if there's an action available.
	HasAction bool

	// ActionLabel is the CTA button text.
	ActionLabel string

	// DismissLabel is the dismiss link text.
	DismissLabel string

	// Footer is the footer text.
	Footer string
}

// NewUndoablePage creates a page from eligibility.
func NewUndoablePage(eligible bool) *UndoablePage {
	if !eligible {
		return &UndoablePage{
			Title:        "Nothing to do.",
			Subtitle:     "Everything is being held quietly.",
			HasAction:    false,
			DismissLabel: "",
			Footer:       "You can always come back later.",
		}
	}
	return &UndoablePage{
		Title:        "Once, quietly.",
		Subtitle:     "We can do one reversible thing for you. If you want.",
		HasAction:    true,
		ActionLabel:  "Run once",
		DismissLabel: "Not now",
		Footer:       "This can be undone briefly.",
	}
}

// DonePage represents the UI data for the done confirmation page.
type DonePage struct {
	// Title is the page title.
	Title string

	// Message is the confirmation message.
	Message string

	// UndoAvailable indicates if undo is still possible.
	UndoAvailable bool

	// UndoMessage describes undo availability.
	UndoMessage string

	// Footer is the footer text.
	Footer string
}

// NewDonePage creates a done page.
func NewDonePage(undoAvailable bool) *DonePage {
	undoMsg := ""
	if undoAvailable {
		undoMsg = "Undo is available briefly."
	}
	return &DonePage{
		Title:         "Done.",
		Message:       "",
		UndoAvailable: undoAvailable,
		UndoMessage:   undoMsg,
		Footer:        "Quiet resumes.",
	}
}

// UndoPage represents the UI data for the undo confirmation page.
type UndoPage struct {
	// Title is the page title.
	Title string

	// Message is the undo message.
	Message string

	// CanUndo indicates if undo is still possible.
	CanUndo bool

	// ActionLabel is the undo button text.
	ActionLabel string

	// Footer is the footer text.
	Footer string
}

// NewUndoPage creates an undo page.
func NewUndoPage(canUndo bool) *UndoPage {
	if !canUndo {
		return &UndoPage{
			Title:   "Too late.",
			Message: "The undo window has closed.",
			CanUndo: false,
			Footer:  "Quiet resumes.",
		}
	}
	return &UndoPage{
		Title:       "Undo?",
		Message:     "This will reverse what was done.",
		CanUndo:     true,
		ActionLabel: "Undo",
		Footer:      "This cannot be undone again.",
	}
}

// PeriodKeyFromTime creates a period key from a timestamp.
func PeriodKeyFromTime(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
