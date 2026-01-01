// Package surface provides ambient availability cues for the Quiet Shift feature.
// Phase 18.4: Subtle availability without urgency or identifiers.
//
// Reference: docs/ADR/ADR-0036-phase18-4-quiet-shift.md
//
// Invariants:
//   - No identifiers (vendors, names, emails, exact timestamps, amounts)
//   - Abstract categories only (money/time/work/people/home)
//   - Magnitude buckets only (nothing/a_few/several)
//   - Horizon buckets only (soon/this_week/later) - NO dates
//   - Deterministic: same inputs + clock = same output
//   - Store only hashes, never raw content
package surface

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Category represents abstract life domains (reused from Phase 18.3).
type Category string

const (
	CategoryMoney  Category = "money"
	CategoryTime   Category = "time"
	CategoryWork   Category = "work"
	CategoryPeople Category = "people"
	CategoryHome   Category = "home"
)

// CategoryPriority defines the deterministic priority order for surfacing.
// money > time > work > people > home
var CategoryPriority = []Category{
	CategoryMoney,
	CategoryTime,
	CategoryWork,
	CategoryPeople,
	CategoryHome,
}

// CategoryDisplayName returns a human-friendly display name.
func CategoryDisplayName(c Category) string {
	names := map[Category]string{
		CategoryMoney:  "Money",
		CategoryTime:   "Time",
		CategoryWork:   "Work",
		CategoryPeople: "People",
		CategoryHome:   "Home",
	}
	if name, ok := names[c]; ok {
		return name
	}
	return string(c)
}

// HorizonBucket represents vague time horizons (no specific dates).
type HorizonBucket string

const (
	HorizonSoon     HorizonBucket = "soon"
	HorizonThisWeek HorizonBucket = "this_week"
	HorizonLater    HorizonBucket = "later"
)

// HorizonDisplayText returns human-friendly horizon text.
func HorizonDisplayText(h HorizonBucket) string {
	texts := map[HorizonBucket]string{
		HorizonSoon:     "soon",
		HorizonThisWeek: "this week",
		HorizonLater:    "later",
	}
	if text, ok := texts[h]; ok {
		return text
	}
	return string(h)
}

// MagnitudeBucket represents count ranges without exposing exact numbers.
type MagnitudeBucket string

const (
	MagnitudeNothing MagnitudeBucket = "nothing"
	MagnitudeAFew    MagnitudeBucket = "a_few"
	MagnitudeSeveral MagnitudeBucket = "several"
)

// SurfaceCue represents the subtle availability indicator shown on /today.
// This is intentionally minimal - just a hint that something exists.
type SurfaceCue struct {
	// Available indicates whether the cue should be shown at all.
	Available bool

	// CueText is the subtle, low-pressure text shown.
	// Example: "If you wanted to, there's one thing you could look at."
	CueText string

	// LinkText is the understated link text.
	// Example: "View, if you like"
	LinkText string

	// Hash is SHA256 of the canonical cue for audit.
	Hash string

	// GeneratedAt is when this cue was computed (injected clock).
	GeneratedAt time.Time
}

// ExplainLine represents one line of abstract explainability.
type ExplainLine struct {
	// Text is the explanation (must not contain identifiers).
	Text string
}

// SurfaceItem represents one abstract item that could be surfaced.
// Contains NO identifiers - only abstract categories and buckets.
type SurfaceItem struct {
	// Category is the abstract domain (money/time/work/people/home).
	Category Category

	// Magnitude indicates how much is held in this category.
	Magnitude MagnitudeBucket

	// Horizon is when this might become relevant (soon/this_week/later).
	Horizon HorizonBucket

	// ReasonSummary is one abstract sentence explaining why we noticed.
	// Must NOT contain identifiers.
	ReasonSummary string

	// Explain contains abstract explainability bullets (on demand).
	Explain []ExplainLine

	// ItemKeyHash is SHA256 of the canonical item key (not the content).
	ItemKeyHash string
}

// SurfacePage represents the full surface page data.
type SurfacePage struct {
	// Title is the page title.
	Title string

	// Subtitle is a calming subtitle.
	Subtitle string

	// Item is the single surfaced item.
	Item SurfaceItem

	// ShowExplain indicates whether to show the explain panel.
	ShowExplain bool

	// Hash is SHA256 of the canonical page for audit.
	Hash string

	// GeneratedAt is when this page was computed.
	GeneratedAt time.Time
}

// Action represents user actions on surfaced items.
type Action string

const (
	ActionViewed        Action = "viewed"
	ActionHeld          Action = "held"
	ActionWhy           Action = "why"
	ActionPreferShowAll Action = "prefer_show_all"
)

// ActionRecord represents a recorded action (hash-only, no raw content).
type ActionRecord struct {
	// CircleID is the circle context (may be empty for personal).
	CircleID string

	// ItemKeyHash is the hash of the item acted upon.
	ItemKeyHash string

	// Action is what the user did.
	Action Action

	// RecordedAt is when this action was recorded (injected clock).
	RecordedAt time.Time

	// RecordHash is SHA256 of the canonical record.
	RecordHash string
}

// SurfaceInput provides all inputs needed for surface computation.
type SurfaceInput struct {
	// HeldCategories maps category to magnitude (from Phase 18.3).
	HeldCategories map[Category]MagnitudeBucket

	// UserPreference is the current preference (quiet vs show_all).
	UserPreference string

	// SuppressedFinance indicates finance obligations are suppressed.
	SuppressedFinance bool

	// SuppressedWork indicates work items are suppressed.
	SuppressedWork bool

	// Now is the current time (injected).
	Now time.Time
}

// Hash computes a deterministic hash of the input.
func (i SurfaceInput) Hash() string {
	// Canonical representation for hashing
	canonical := fmt.Sprintf(
		"held:%v|pref:%s|fin:%v|work:%v|now:%d",
		i.HeldCategories,
		i.UserPreference,
		i.SuppressedFinance,
		i.SuppressedWork,
		i.Now.Unix(),
	)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

// DefaultInput returns a default input for testing/demo.
func DefaultInput() SurfaceInput {
	return SurfaceInput{
		HeldCategories: map[Category]MagnitudeBucket{
			CategoryMoney: MagnitudeAFew,
			CategoryTime:  MagnitudeAFew,
			CategoryWork:  MagnitudeSeveral,
		},
		UserPreference:    "quiet",
		SuppressedFinance: true,
		SuppressedWork:    true,
		Now:               time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}
}

// EmptyInput returns an input with no held items.
func EmptyInput() SurfaceInput {
	return SurfaceInput{
		HeldCategories: map[Category]MagnitudeBucket{},
		UserPreference: "quiet",
		Now:            time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}
}

// computeHash computes SHA256 hash of a string.
func computeHash(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}
