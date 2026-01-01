// Package held provides the "Held, not shown" projection.
//
// Phase 18.3: The Proof of Care
//
// CRITICAL: This is a projection, not a queue.
// CRITICAL: Held items NEVER contain raw event data.
// CRITICAL: Held items NEVER expose identifiers.
// CRITICAL: Held items NEVER are actionable.
//
// The purpose is to demonstrate stewardship without disclosure.
// Silence must feel intentional.
//
// Reference: docs/ADR/ADR-0035-phase18-3-proof-of-care.md
package held

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Category represents an abstract held category.
// These are conceptual buckets, not concrete entities.
type Category string

const (
	// CategoryTime represents time-related items (calendar, deadlines, rhythms).
	CategoryTime Category = "time"

	// CategoryMoney represents financial items (bills, budgets, flows).
	CategoryMoney Category = "money"

	// CategoryPeople represents relationship items (conversations, commitments).
	CategoryPeople Category = "people"

	// CategoryWork represents professional items (tasks, projects, obligations).
	CategoryWork Category = "work"

	// CategoryHome represents domestic items (maintenance, supplies, routines).
	CategoryHome Category = "home"
)

// ReasonHeld explains why something is held without surfacing.
// These are abstract reasons, not specific justifications.
type ReasonHeld string

const (
	// ReasonNotUrgent means the item doesn't require immediate attention.
	ReasonNotUrgent ReasonHeld = "not_urgent"

	// ReasonAwaitingContext means more information is needed before surfacing.
	ReasonAwaitingContext ReasonHeld = "awaiting_context"

	// ReasonProtectedByPolicy means a policy explicitly suppresses this.
	ReasonProtectedByPolicy ReasonHeld = "protected_by_policy"

	// ReasonNoRegretRisk means ignoring this won't cause future regret.
	ReasonNoRegretRisk ReasonHeld = "no_regret_risk"

	// ReasonQuietHours means the item is held due to quiet hours.
	ReasonQuietHours ReasonHeld = "quiet_hours"
)

// HeldItem represents a single abstract held item.
// CRITICAL: No identifiers, no raw data, no timestamps of source events.
type HeldItem struct {
	// Category is the abstract category (time, money, people, etc.).
	Category Category

	// Reason is why this item is held.
	Reason ReasonHeld
}

// CategorySummary provides an abstract summary for a category.
type CategorySummary struct {
	// Category is the abstract category.
	Category Category

	// Presence indicates items exist in this category (true/false only).
	// We do NOT expose counts.
	Presence bool

	// PrimaryReason is the dominant reason items are held in this category.
	PrimaryReason ReasonHeld
}

// HeldSummary is the deterministic projection shown to the person.
// CRITICAL: No counts tied to specific items.
// CRITICAL: No identifiers, names, vendors, or entities.
type HeldSummary struct {
	// Statement is the calm explanatory sentence.
	Statement string

	// Categories are the abstract categories with held items.
	// Maximum 3 categories shown.
	Categories []CategorySummary

	// Magnitude is a bucketed indicator: "nothing", "a few", "several".
	// Never a specific number.
	Magnitude string

	// Hash is the deterministic hash of this summary.
	Hash string

	// GeneratedAt is when this summary was computed.
	GeneratedAt time.Time
}

// HeldInput provides the signals used to compute the summary.
// These come from existing loop outputs, not raw events.
type HeldInput struct {
	// SuppressedObligationCount is from the interruption engine.
	SuppressedObligationCount int

	// PolicyBlockedCount is from circle policies.
	PolicyBlockedCount int

	// QuietHoursActive indicates quiet hours are in effect.
	QuietHoursActive bool

	// HasTimeItems indicates time-related items exist.
	HasTimeItems bool

	// HasMoneyItems indicates money-related items exist.
	HasMoneyItems bool

	// HasPeopleItems indicates people-related items exist.
	HasPeopleItems bool

	// HasWorkItems indicates work-related items exist.
	HasWorkItems bool

	// HasHomeItems indicates home-related items exist.
	HasHomeItems bool

	// CircleID is the circle context (for store records).
	CircleID string

	// Now is the current time (injected for determinism).
	Now time.Time
}

// Hash computes a deterministic hash of the input.
func (h *HeldInput) Hash() string {
	canonical := fmt.Sprintf(
		"suppressed:%d|policy_blocked:%d|quiet_hours:%t|time:%t|money:%t|people:%t|work:%t|home:%t|circle:%s|now:%s",
		h.SuppressedObligationCount,
		h.PolicyBlockedCount,
		h.QuietHoursActive,
		h.HasTimeItems,
		h.HasMoneyItems,
		h.HasPeopleItems,
		h.HasWorkItems,
		h.HasHomeItems,
		h.CircleID,
		h.Now.Format(time.RFC3339),
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ComputeHash computes the deterministic hash of the summary.
func (s *HeldSummary) ComputeHash() string {
	var parts []string
	parts = append(parts, s.Statement)
	parts = append(parts, s.Magnitude)

	// Sort categories for determinism
	cats := make([]string, 0, len(s.Categories))
	for _, c := range s.Categories {
		cats = append(cats, fmt.Sprintf("%s:%t:%s", c.Category, c.Presence, c.PrimaryReason))
	}
	sort.Strings(cats)
	parts = append(parts, cats...)

	parts = append(parts, s.GeneratedAt.Format(time.RFC3339))

	canonical := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// SummaryRecord is what gets stored (hash only, no data).
type SummaryRecord struct {
	// Hash is the SHA256 hash of the summary.
	Hash string

	// CircleID is the circle context.
	CircleID string

	// RecordedAt is when this was recorded.
	RecordedAt time.Time
}
