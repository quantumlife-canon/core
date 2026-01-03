// Package invitation provides domain types for gentle action invitations.
//
// Phase 23: Gentle Action Invitation (Trust-Preserving)
//
// This package defines the invitation model for the first optional,
// reversible choice in QuantumLife.
//
// CRITICAL INVARIANTS:
//   - Invitations appear only after trust is proven
//   - Never create urgency
//   - Never auto-execute
//   - Never surface identifiers
//   - Silence remains the success state
//   - Pipe-delimited canonical strings
//   - SHA-256 hashing
//   - Deterministic for same inputs + clock
//   - No goroutines. No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
package invitation

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// InvitationKind represents the type of invitation offered.
type InvitationKind string

const (
	// KindHoldContinue - "We can keep holding this."
	KindHoldContinue InvitationKind = "hold_continue"

	// KindReviewOnce - "You can look once, if you want."
	KindReviewOnce InvitationKind = "review_once"

	// KindNotifyNextTime - "Tell us how you'd like this to reach you next time."
	KindNotifyNextTime InvitationKind = "notify_next_time"
)

// AllKinds returns all valid invitation kinds in priority order.
func AllKinds() []InvitationKind {
	return []InvitationKind{
		KindHoldContinue,
		KindReviewOnce,
		KindNotifyNextTime,
	}
}

// DisplayText returns the calm, whisper-level text for this kind.
// These are the ONLY allowed phrases.
func (k InvitationKind) DisplayText() string {
	switch k {
	case KindHoldContinue:
		return "We can keep holding this."
	case KindReviewOnce:
		return "You can look once, if you want."
	case KindNotifyNextTime:
		return "Tell us how you'd like this to reach you next time."
	default:
		return ""
	}
}

// InvitationDecision represents the circle's response to an invitation.
type InvitationDecision string

const (
	// DecisionAccepted - circle accepted the invitation.
	DecisionAccepted InvitationDecision = "accepted"

	// DecisionDismissed - circle dismissed the invitation.
	DecisionDismissed InvitationDecision = "dismissed"
)

// InvitationPeriod represents a time bucket for invitation eligibility.
// Derived from injected clock, never raw timestamps.
type InvitationPeriod struct {
	// DateBucket is the date in "2006-01-02" format.
	DateBucket string

	// PeriodHash is the SHA-256 hash of the period for storage.
	PeriodHash string
}

// NewInvitationPeriod creates a period from a date bucket string.
func NewInvitationPeriod(dateBucket string) InvitationPeriod {
	hash := sha256.Sum256([]byte("INVITATION_PERIOD|v1|" + dateBucket))
	return InvitationPeriod{
		DateBucket: dateBucket,
		PeriodHash: hex.EncodeToString(hash[:]),
	}
}

// InvitationEligibility contains the abstract inputs for eligibility check.
// CRITICAL: No identifiers, no raw data.
type InvitationEligibility struct {
	// CircleID is the circle identifier.
	CircleID string

	// HasGmailConnection indicates Gmail is connected.
	HasGmailConnection bool

	// HasSyncReceipt indicates at least one real sync occurred.
	HasSyncReceipt bool

	// HasQuietMirrorViewed indicates the mirror was viewed.
	HasQuietMirrorViewed bool

	// HasTrustBaseline indicates trust accrual exists.
	HasTrustBaseline bool

	// HasShadowReceipt indicates shadow observation exists (boolean only).
	HasShadowReceipt bool

	// HeldMagnitude is the abstract magnitude of held items.
	HeldMagnitude string

	// Period is the current invitation period.
	Period InvitationPeriod

	// DismissedThisPeriod indicates if already dismissed.
	DismissedThisPeriod bool

	// AcceptedThisPeriod indicates if already accepted.
	AcceptedThisPeriod bool
}

// IsEligible returns true if an invitation can be shown.
func (e *InvitationEligibility) IsEligible() bool {
	// Must have Gmail connected
	if !e.HasGmailConnection {
		return false
	}

	// Must have at least one real sync
	if !e.HasSyncReceipt {
		return false
	}

	// Must have viewed the quiet mirror
	if !e.HasQuietMirrorViewed {
		return false
	}

	// Must have trust baseline
	if !e.HasTrustBaseline {
		return false
	}

	// Not if already dismissed this period
	if e.DismissedThisPeriod {
		return false
	}

	// Not if already accepted this period
	if e.AcceptedThisPeriod {
		return false
	}

	return true
}

// InvitationSummary contains the computed invitation to show.
// CRITICAL: Abstract text only, no identifiers.
type InvitationSummary struct {
	// CircleID is the circle this invitation is for.
	CircleID string

	// Period is the invitation period.
	Period InvitationPeriod

	// Kind is the selected invitation kind.
	Kind InvitationKind

	// Text is the display text (from allowed phrases only).
	Text string

	// WhisperCue is the cue text for /today.
	WhisperCue string

	// SourceHash is the hash of the eligibility inputs.
	SourceHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (s *InvitationSummary) CanonicalString() string {
	parts := []string{
		"INVITATION",
		"v1",
		s.CircleID,
		s.Period.DateBucket,
		string(s.Kind),
		s.SourceHash,
	}
	return strings.Join(parts, "|")
}

// Hash returns the SHA-256 hash of the canonical string.
func (s *InvitationSummary) Hash() string {
	hash := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// InvitationRecord represents a persisted invitation decision.
type InvitationRecord struct {
	// InvitationHash is the hash of the invitation summary.
	InvitationHash string

	// Decision is the circle's decision.
	Decision InvitationDecision

	// PeriodHash is the hash of the period.
	PeriodHash string

	// CircleID is the circle identifier.
	CircleID string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *InvitationRecord) CanonicalString() string {
	parts := []string{
		"INVITATION_RECORD",
		"v1",
		r.CircleID,
		r.InvitationHash,
		string(r.Decision),
		r.PeriodHash,
	}
	return strings.Join(parts, "|")
}

// Hash returns the SHA-256 hash of the canonical string.
func (r *InvitationRecord) Hash() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// InvitationPage represents the UI page data.
type InvitationPage struct {
	// Title is the page title.
	Title string

	// Statement is the main invitation text.
	Statement string

	// Kind is the invitation kind for form submission.
	Kind InvitationKind

	// HasInvitation indicates if there's an invitation to show.
	HasInvitation bool

	// Footer is the reassurance text.
	Footer string
}

// NewEmptyPage returns a page with no invitation.
func NewEmptyPage() *InvitationPage {
	return &InvitationPage{
		Title:         "Nothing to decide.",
		Statement:     "Everything is being held quietly.",
		HasInvitation: false,
		Footer:        "You can always come back later.",
	}
}

// NewInvitationPage creates a page from an invitation summary.
func NewInvitationPage(summary *InvitationSummary) *InvitationPage {
	if summary == nil {
		return NewEmptyPage()
	}
	return &InvitationPage{
		Title:         "If you ever want.",
		Statement:     summary.Text,
		Kind:          summary.Kind,
		HasInvitation: true,
		Footer:        "This will wait. No rush.",
	}
}

// EligibilityHash computes a hash of eligibility inputs for determinism.
func (e *InvitationEligibility) Hash() string {
	// Sort boolean flags for determinism
	flags := []string{}
	if e.HasGmailConnection {
		flags = append(flags, "gmail")
	}
	if e.HasSyncReceipt {
		flags = append(flags, "sync")
	}
	if e.HasQuietMirrorViewed {
		flags = append(flags, "mirror")
	}
	if e.HasTrustBaseline {
		flags = append(flags, "trust")
	}
	if e.HasShadowReceipt {
		flags = append(flags, "shadow")
	}
	sort.Strings(flags)

	parts := []string{
		"INVITATION_ELIGIBILITY",
		"v1",
		e.CircleID,
		e.Period.DateBucket,
		e.HeldMagnitude,
		strings.Join(flags, ","),
	}
	canonical := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}
