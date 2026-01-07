// Package trustaction provides domain types for Phase 28: Trust Kept.
// This phase implements the first and only trust-confirming real action.
// After execution: silence forever. No growth mechanics, engagement loops, or escalation paths.
package trustaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// TrustActionKind represents the type of action.
// ONLY calendar_respond is allowed - no email, payments, or other actions.
type TrustActionKind string

const (
	// ActionKindCalendarRespond is the ONLY allowed action kind.
	// This responds to calendar invitations - lowest risk, most reversible.
	ActionKindCalendarRespond TrustActionKind = "calendar_respond"
)

// TrustActionState represents the state machine for trust actions.
type TrustActionState string

const (
	// StateEligible - action is available for execution.
	StateEligible TrustActionState = "eligible"
	// StateExecuted - action was executed, undo may still be available.
	StateExecuted TrustActionState = "executed"
	// StateUndone - action was reversed after execution.
	StateUndone TrustActionState = "undone"
	// StateExpired - undo window has passed.
	StateExpired TrustActionState = "expired"
)

// HorizonBucket represents abstract time horizon (no timestamps).
type HorizonBucket string

const (
	// HorizonSoon - within 24 hours.
	HorizonSoon HorizonBucket = "soon"
	// HorizonLater - 1-7 days.
	HorizonLater HorizonBucket = "later"
	// HorizonSomeday - more than 7 days.
	HorizonSomeday HorizonBucket = "someday"
)

// TrustActionPreview describes what would happen (abstract only, no identifiers).
type TrustActionPreview struct {
	ActionKind     TrustActionKind
	AbstractTarget string // "a calendar event" - NEVER names or identifiers
	HorizonBucket  HorizonBucket
	Reversible     bool // always true for Phase 28
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (p *TrustActionPreview) CanonicalString() string {
	return fmt.Sprintf("v1|preview|%s|%s|%s|%t",
		p.ActionKind,
		p.AbstractTarget,
		p.HorizonBucket,
		p.Reversible,
	)
}

// UndoBucket represents a 15-minute bucket for undo window.
// Time is floored to :00, :15, :30, :45 boundaries.
type UndoBucket struct {
	BucketStartRFC3339    string // RFC3339 format
	BucketDurationMinutes int    // always 15
}

// NewUndoBucket creates an undo bucket starting from the given time.
// Time is floored to the nearest 15-minute boundary.
func NewUndoBucket(t time.Time) UndoBucket {
	// Floor to 15-minute boundary
	minute := t.Minute()
	flooredMinute := (minute / 15) * 15
	floored := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), flooredMinute, 0, 0, t.Location())

	return UndoBucket{
		BucketStartRFC3339:    floored.Format(time.RFC3339),
		BucketDurationMinutes: 15,
	}
}

// IsExpired checks if the undo window has passed.
func (u *UndoBucket) IsExpired(now time.Time) bool {
	start, err := time.Parse(time.RFC3339, u.BucketStartRFC3339)
	if err != nil {
		return true // invalid bucket, treat as expired
	}
	end := start.Add(time.Duration(u.BucketDurationMinutes) * time.Minute)
	return now.After(end)
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (u *UndoBucket) CanonicalString() string {
	return fmt.Sprintf("v1|undo_bucket|%s|%d",
		u.BucketStartRFC3339,
		u.BucketDurationMinutes,
	)
}

// TrustActionReceipt is proof of action - contains only hashes, no identifiers.
type TrustActionReceipt struct {
	ReceiptID    string // deterministic hash computed from content
	ActionKind   TrustActionKind
	State        TrustActionState
	UndoBucket   UndoBucket
	Period       string // "2025-01-15" format (day bucket)
	CircleID     string // circle that executed the action
	StatusHash   string // 32 hex chars, computed from state
	DraftIDHash  string // hash of draft ID, not raw
	EnvelopeHash string // hash of envelope ID, not raw (for calendar execution)
}

// CanonicalString returns a deterministic, pipe-delimited representation.
// Used for hashing and deterministic verification.
func (r *TrustActionReceipt) CanonicalString() string {
	return fmt.Sprintf("v1|trust_action_receipt|%s|%s|%s|%s|%s|%s|%s|%s",
		r.ActionKind,
		r.State,
		r.UndoBucket.CanonicalString(),
		r.Period,
		r.CircleID,
		r.DraftIDHash,
		r.EnvelopeHash,
		r.StatusHash,
	)
}

// ComputeStatusHash computes the status hash from receipt content.
func (r *TrustActionReceipt) ComputeStatusHash() string {
	content := fmt.Sprintf("v1|status|%s|%s|%s|%s|%s|%s",
		r.ActionKind,
		r.State,
		r.Period,
		r.CircleID,
		r.DraftIDHash,
		r.EnvelopeHash,
	)
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16]) // 32 hex chars
}

// ComputeReceiptID computes the receipt ID from content hash.
func (r *TrustActionReceipt) ComputeReceiptID() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:16]) // 32 hex chars
}

// HashString computes a SHA256 hash of the input string.
// Returns first 32 hex characters.
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:16])
}

// TrustActionCue represents the whisper cue for trust actions.
// Appears with lowest priority in the single whisper rule chain.
type TrustActionCue struct {
	Available bool
	CueText   string // "One thing could happen — if you let it."
	LinkText  string // "preview"
	CueHash   string // deterministic hash for tracking
}

// NewTrustActionCue creates a cue for display.
func NewTrustActionCue(available bool) *TrustActionCue {
	if !available {
		return &TrustActionCue{Available: false}
	}
	cue := &TrustActionCue{
		Available: true,
		CueText:   "One thing could happen — if you let it.",
		LinkText:  "preview",
	}
	cue.CueHash = HashString(fmt.Sprintf("v1|cue|%s|%s", cue.CueText, cue.LinkText))
	return cue
}

// EligibilityResult contains the result of eligibility check.
type EligibilityResult struct {
	Eligible  bool
	Reason    string
	Preview   *TrustActionPreview
	DraftID   string // internal use only, not exposed to UI
	PeriodKey string
}

// ExecuteResult contains the result of execution.
type ExecuteResult struct {
	Success bool
	Error   string
	Receipt *TrustActionReceipt
}

// UndoResult contains the result of undo.
type UndoResult struct {
	Success bool
	Error   string
	Receipt *TrustActionReceipt
}
