// Package firstaction provides domain types for first reversible action.
//
// Phase 24: First Reversible Real Action (Trust-Preserving)
//
// This package defines the model for the first real, circle-initiated action
// in QuantumLife. The action is explicitly requested, operates on one held
// item only, produces a preview (never execution), and leaves no lingering
// prompts after completion.
//
// CRITICAL INVARIANTS:
//   - Action ≠ Workflow
//   - One action per period maximum
//   - Preview only, never execution
//   - No identifiable data persisted
//   - Silence resumes after action
//   - No goroutines. No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0054-phase24-first-reversible-action.md
package firstaction

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ActionKind represents the type of action.
// Phase 24 supports preview_only.
type ActionKind string

const (
	// KindPreviewOnly - show a preview without taking action.
	KindPreviewOnly ActionKind = "preview_only"
)

// ActionState represents the state of an action invitation.
type ActionState string

const (
	// StateOffered - action was offered to the circle.
	StateOffered ActionState = "offered"

	// StateViewed - circle viewed the preview.
	StateViewed ActionState = "viewed"

	// StateDismissed - circle dismissed the invitation.
	StateDismissed ActionState = "dismissed"

	// StateAcknowledged - circle acknowledged the preview ("Hold this").
	StateAcknowledged ActionState = "acknowledged"
)

// ActionPeriod represents a time bucket for action eligibility.
// Derived from injected clock, never raw timestamps.
type ActionPeriod struct {
	// DateBucket is the date in "2006-01-02" format.
	DateBucket string

	// PeriodHash is the SHA-256 hash of the period for storage.
	PeriodHash string
}

// NewActionPeriod creates a period from a date bucket string.
func NewActionPeriod(dateBucket string) ActionPeriod {
	hash := sha256.Sum256([]byte("ACTION_PERIOD|v1|" + dateBucket))
	return ActionPeriod{
		DateBucket: dateBucket,
		PeriodHash: hex.EncodeToString(hash[:]),
	}
}

// ActionEligibility contains abstract inputs for eligibility check.
// CRITICAL: No identifiers, no raw data.
type ActionEligibility struct {
	// CircleID is the circle identifier.
	CircleID string

	// HasGmailConnection indicates Gmail is connected.
	HasGmailConnection bool

	// HasQuietBaseline indicates quiet baseline verified.
	HasQuietBaseline bool

	// HasMirrorViewed indicates mirror was viewed.
	HasMirrorViewed bool

	// HasTrustAccrual indicates trust exists for current period.
	HasTrustAccrual bool

	// HasPriorActionThisPeriod indicates an action was already taken.
	HasPriorActionThisPeriod bool

	// HasHeldItems indicates there are held items to show.
	HasHeldItems bool

	// Period is the current action period.
	Period ActionPeriod
}

// IsEligible returns true if an action can be offered.
func (e *ActionEligibility) IsEligible() bool {
	// Must have Gmail connected
	if !e.HasGmailConnection {
		return false
	}

	// Must have quiet baseline
	if !e.HasQuietBaseline {
		return false
	}

	// Must have viewed mirror
	if !e.HasMirrorViewed {
		return false
	}

	// Must have trust accrual
	if !e.HasTrustAccrual {
		return false
	}

	// No prior action this period
	if e.HasPriorActionThisPeriod {
		return false
	}

	// Must have something to show
	if !e.HasHeldItems {
		return false
	}

	return true
}

// Hash computes a hash of eligibility inputs for determinism.
func (e *ActionEligibility) Hash() string {
	flags := []string{}
	if e.HasGmailConnection {
		flags = append(flags, "gmail")
	}
	if e.HasQuietBaseline {
		flags = append(flags, "quiet")
	}
	if e.HasMirrorViewed {
		flags = append(flags, "mirror")
	}
	if e.HasTrustAccrual {
		flags = append(flags, "trust")
	}
	if e.HasHeldItems {
		flags = append(flags, "held")
	}

	parts := []string{
		"ACTION_ELIGIBILITY",
		"v1",
		e.CircleID,
		e.Period.DateBucket,
		strings.Join(flags, ","),
	}
	canonical := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// AbstractCategory represents an abstract category for display.
type AbstractCategory string

const (
	CategoryMoney  AbstractCategory = "money"
	CategoryTime   AbstractCategory = "time"
	CategoryWork   AbstractCategory = "work"
	CategoryPeople AbstractCategory = "people"
	CategoryHome   AbstractCategory = "home"
)

// DisplayText returns calm display text for the category.
func (c AbstractCategory) DisplayText() string {
	switch c {
	case CategoryMoney:
		return "Something about money"
	case CategoryTime:
		return "Something about time"
	case CategoryWork:
		return "Something about work"
	case CategoryPeople:
		return "Something about people"
	case CategoryHome:
		return "Something about home"
	default:
		return "Something held"
	}
}

// HorizonBucket represents an abstract time horizon.
type HorizonBucket string

const (
	HorizonSoon    HorizonBucket = "soon"
	HorizonLater   HorizonBucket = "later"
	HorizonSomeday HorizonBucket = "someday"
)

// DisplayText returns calm display text for the horizon.
func (h HorizonBucket) DisplayText() string {
	switch h {
	case HorizonSoon:
		return "This is often easier earlier."
	case HorizonLater:
		return "This can wait a bit."
	case HorizonSomeday:
		return "No rush on this one."
	default:
		return ""
	}
}

// MagnitudeBucket represents an abstract magnitude.
type MagnitudeBucket string

const (
	MagnitudeSmall  MagnitudeBucket = "small"
	MagnitudeMedium MagnitudeBucket = "medium"
	MagnitudeLarge  MagnitudeBucket = "large"
)

// ActionPreview contains the preview data to show.
// CRITICAL: Abstract only, no identifiers.
type ActionPreview struct {
	// CircleID is the circle this preview is for.
	CircleID string

	// Period is the action period.
	Period ActionPeriod

	// Category is the abstract category.
	Category AbstractCategory

	// Horizon is the abstract time horizon.
	Horizon HorizonBucket

	// Magnitude is the abstract magnitude.
	Magnitude MagnitudeBucket

	// Explanation is the calm explanation text.
	Explanation string

	// SourceHash is the hash of the source item (not the item itself).
	SourceHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *ActionPreview) CanonicalString() string {
	parts := []string{
		"ACTION_PREVIEW",
		"v1",
		p.CircleID,
		p.Period.DateBucket,
		string(p.Category),
		string(p.Horizon),
		string(p.Magnitude),
		p.SourceHash,
	}
	return strings.Join(parts, "|")
}

// Hash returns the SHA-256 hash of the canonical string.
func (p *ActionPreview) Hash() string {
	hash := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// ActionRecord represents a persisted action state.
type ActionRecord struct {
	// ActionHash is the hash of the action preview.
	ActionHash string

	// State is the current state.
	State ActionState

	// PeriodHash is the hash of the period.
	PeriodHash string

	// CircleID is the circle identifier.
	CircleID string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *ActionRecord) CanonicalString() string {
	parts := []string{
		"ACTION_RECORD",
		"v1",
		r.CircleID,
		r.ActionHash,
		string(r.State),
		r.PeriodHash,
	}
	return strings.Join(parts, "|")
}

// Hash returns the SHA-256 hash of the canonical string.
func (r *ActionRecord) Hash() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// ActionPage represents the UI page data for the action invitation.
type ActionPage struct {
	// Title is the page title.
	Title string

	// Subtitle is the page subtitle.
	Subtitle string

	// CategoryText is the abstract category display text.
	CategoryText string

	// Reassurance is the reassurance text.
	Reassurance string

	// HasAction indicates if there's an action to offer.
	HasAction bool

	// Footer is the footer text.
	Footer string
}

// NewEmptyActionPage returns a page with no action.
func NewEmptyActionPage() *ActionPage {
	return &ActionPage{
		Title:       "Nothing to look at.",
		Subtitle:    "Everything is being held quietly.",
		HasAction:   false,
		Reassurance: "",
		Footer:      "You can always come back later.",
	}
}

// NewActionPage creates a page from eligibility.
func NewActionPage(category AbstractCategory) *ActionPage {
	return &ActionPage{
		Title:        "Once, together.",
		Subtitle:     "We'll look at one thing. Then we'll stop.",
		CategoryText: category.DisplayText(),
		Reassurance:  "Nothing will be sent. Nothing will change.",
		HasAction:    true,
		Footer:       "This will wait. No rush.",
	}
}

// PreviewPage represents the UI page data for the preview result.
type PreviewPage struct {
	// Title is the page title.
	Title string

	// CategoryText is the category display text.
	CategoryText string

	// HorizonText is the horizon explanation text.
	HorizonText string

	// Disclaimer is the explicit disclaimer.
	Disclaimer string

	// PreviewHash is the hash for form submission.
	PreviewHash string

	// Footer is the footer text.
	Footer string
}

// NewPreviewPage creates a preview page from a preview.
func NewPreviewPage(preview *ActionPreview) *PreviewPage {
	if preview == nil {
		return &PreviewPage{
			Title:      "Nothing to show.",
			Disclaimer: "We did not act.",
			Footer:     "Quiet resumes.",
		}
	}
	return &PreviewPage{
		Title:        preview.Category.DisplayText(),
		CategoryText: string(preview.Category),
		HorizonText:  preview.Horizon.DisplayText(),
		Disclaimer:   "This is a preview. We did not act.",
		PreviewHash:  preview.Hash(),
		Footer:       "Quiet resumes.",
	}
}
