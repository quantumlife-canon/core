// Package interrupt defines the interruption domain model for Phase 3.
//
// Interruptions are prioritized signals that determine when and how
// to surface obligations to users. They implement "Nothing Needs You"
// by earning the right to interrupt through regret-based scoring.
//
// CRITICAL: Deterministic computation. Same inputs = same outputs.
// CRITICAL: Uses canonical string hashing (NOT JSON).
// CRITICAL: Read-only. This package only classifies, never acts.
//
// Reference: docs/ADR/ADR-0020-phase3-interruptions-and-digest.md
package interrupt

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// Level indicates how urgently to surface an interruption.
// Higher levels earn the right to interrupt the user more.
type Level string

const (
	LevelSilent  Level = "silent"  // Do not surface; log only
	LevelAmbient Level = "ambient" // Passive display (e.g., badge count)
	LevelQueued  Level = "queued"  // Show in digest/queue, no push
	LevelNotify  Level = "notify"  // Push notification worthy
	LevelUrgent  Level = "urgent"  // Immediate attention required
)

// LevelOrder returns a sort key for level (higher = more urgent).
func LevelOrder(l Level) int {
	switch l {
	case LevelUrgent:
		return 4
	case LevelNotify:
		return 3
	case LevelQueued:
		return 2
	case LevelAmbient:
		return 1
	case LevelSilent:
		return 0
	default:
		return -1
	}
}

// Trigger identifies what caused the interruption.
type Trigger string

const (
	TriggerObligationDueSoon     Trigger = "obligation_due_soon"
	TriggerEmailActionNeeded     Trigger = "email_action_needed"
	TriggerCalendarInvitePending Trigger = "calendar_invite_pending"
	TriggerCalendarConflict      Trigger = "calendar_conflict"
	TriggerCalendarUpcoming      Trigger = "calendar_upcoming"
	TriggerFinanceLowBalance     Trigger = "finance_low_balance"
	TriggerFinanceLargeTxn       Trigger = "finance_large_txn"
	TriggerFinancePending        Trigger = "finance_pending"
	TriggerUnknown               Trigger = "unknown"
)

// CircleType for regret scoring.
type CircleType string

const (
	CircleTypeFinance CircleType = "finance"
	CircleTypeFamily  CircleType = "family"
	CircleTypeWork    CircleType = "work"
	CircleTypeHealth  CircleType = "health"
	CircleTypeHome    CircleType = "home"
	CircleTypeUnknown CircleType = "unknown"
)

// Interruption represents a prioritized signal to surface to the user.
// Interruptions are computed from obligations and events, never stored.
type Interruption struct {
	// InterruptionID is deterministic: sha256(canonical)[:16]
	InterruptionID string

	// Source identification
	CircleID       identity.EntityID
	IntersectionID string // Optional; may be empty

	// Classification
	Level   Level
	Trigger Trigger

	// Source references
	SourceEventID string // Event that led to this (if any)
	ObligationID  string // Derived from obligation (if any)

	// Scoring (0-100, deterministic, rule-based)
	RegretScore int // 0-100: urgency/importance
	Confidence  int // 0-100: confidence in classification

	// Timing
	ExpiresAt time.Time // When this interruption becomes stale
	CreatedAt time.Time // When this was computed

	// Deduplication
	DedupKey string // Stable key for dedup within time window

	// Human-readable context
	Summary string // Short, deterministic description
}

// NewInterruption creates an interruption with computed fields.
func NewInterruption(
	circleID identity.EntityID,
	trigger Trigger,
	sourceEventID string,
	obligationID string,
	regretScore int,
	confidence int,
	level Level,
	expiresAt time.Time,
	createdAt time.Time,
	summary string,
) *Interruption {
	// Clamp scores to 0-100
	if regretScore < 0 {
		regretScore = 0
	} else if regretScore > 100 {
		regretScore = 100
	}
	if confidence < 0 {
		confidence = 0
	} else if confidence > 100 {
		confidence = 100
	}

	i := &Interruption{
		CircleID:      circleID,
		Trigger:       trigger,
		SourceEventID: sourceEventID,
		ObligationID:  obligationID,
		RegretScore:   regretScore,
		Confidence:    confidence,
		Level:         level,
		ExpiresAt:     expiresAt,
		CreatedAt:     createdAt,
		Summary:       summary,
	}

	// Compute dedup key and ID
	i.DedupKey = i.computeDedupKey()
	i.InterruptionID = i.computeID()

	return i
}

// computeDedupKey generates a stable dedup key.
// Format: dedup|circle|trigger|source_ref|bucket
// Bucket is day (YYYY-MM-DD) or hour (YYYY-MM-DDTHH) for urgent items.
func (i *Interruption) computeDedupKey() string {
	sourceRef := i.SourceEventID
	if sourceRef == "" {
		sourceRef = i.ObligationID
	}
	if sourceRef == "" {
		sourceRef = "none"
	}

	// Determine bucket based on urgency
	var bucket string
	if i.Level == LevelUrgent || i.Level == LevelNotify {
		// Hour bucket for urgent/notify
		bucket = i.CreatedAt.UTC().Format("2006-01-02T15")
	} else {
		// Day bucket for others
		bucket = i.CreatedAt.UTC().Format("2006-01-02")
	}

	return fmt.Sprintf("dedup|%s|%s|%s|%s",
		i.CircleID, i.Trigger, sourceRef, bucket)
}

// computeID generates a deterministic interruption ID.
func (i *Interruption) computeID() string {
	canonical := i.CanonicalString()
	return HashCanonical("interrupt", canonical)
}

// CanonicalString returns the canonical representation for hashing.
// Format is strictly defined and does NOT use JSON.
func (i *Interruption) CanonicalString() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("circle:%s", i.CircleID))
	parts = append(parts, fmt.Sprintf("trigger:%s", i.Trigger))
	parts = append(parts, fmt.Sprintf("level:%s", i.Level))
	parts = append(parts, fmt.Sprintf("source_event:%s", i.SourceEventID))
	parts = append(parts, fmt.Sprintf("obligation:%s", i.ObligationID))
	parts = append(parts, fmt.Sprintf("regret:%d", i.RegretScore))
	parts = append(parts, fmt.Sprintf("confidence:%d", i.Confidence))
	parts = append(parts, fmt.Sprintf("expires:%d", i.ExpiresAt.Unix()))
	parts = append(parts, fmt.Sprintf("created:%d", i.CreatedAt.Unix()))
	parts = append(parts, fmt.Sprintf("summary:%s", i.Summary))

	return strings.Join(parts, "|")
}

// WithIntersection sets the intersection ID.
func (i *Interruption) WithIntersection(intersectionID string) *Interruption {
	i.IntersectionID = intersectionID
	return i
}

// SortInterruptions sorts interruptions deterministically.
// Order: Level DESC, RegretScore DESC, ExpiresAt ASC, InterruptionID ASC
func SortInterruptions(interruptions []*Interruption) {
	sort.SliceStable(interruptions, func(i, j int) bool {
		a, b := interruptions[i], interruptions[j]

		// 1. Level (higher first)
		la, lb := LevelOrder(a.Level), LevelOrder(b.Level)
		if la != lb {
			return la > lb
		}

		// 2. RegretScore (higher first)
		if a.RegretScore != b.RegretScore {
			return a.RegretScore > b.RegretScore
		}

		// 3. ExpiresAt (earlier first)
		if !a.ExpiresAt.Equal(b.ExpiresAt) {
			return a.ExpiresAt.Before(b.ExpiresAt)
		}

		// 4. InterruptionID for deterministic tie-breaking
		return a.InterruptionID < b.InterruptionID
	})
}

// FilterByLevel returns interruptions at or above the given level.
func FilterByLevel(interruptions []*Interruption, minLevel Level) []*Interruption {
	minOrder := LevelOrder(minLevel)
	var result []*Interruption
	for _, i := range interruptions {
		if LevelOrder(i.Level) >= minOrder {
			result = append(result, i)
		}
	}
	return result
}

// FilterByCircle returns interruptions for a specific circle.
func FilterByCircle(interruptions []*Interruption, circleID identity.EntityID) []*Interruption {
	var result []*Interruption
	for _, i := range interruptions {
		if i.CircleID == circleID {
			result = append(result, i)
		}
	}
	return result
}

// CountByLevel returns counts per level.
func CountByLevel(interruptions []*Interruption) map[Level]int {
	counts := make(map[Level]int)
	for _, i := range interruptions {
		counts[i.Level]++
	}
	return counts
}

// DecisionReport summarizes interruption processing.
type DecisionReport struct {
	TotalProcessed  int
	CountByLevel    map[Level]int
	DedupDropped    int
	QuotaDowngraded int
	CircleSummaries map[identity.EntityID]*CircleDecisionSummary
}

// CircleDecisionSummary summarizes decisions for a circle.
type CircleDecisionSummary struct {
	CircleID    identity.EntityID
	Total       int
	Urgent      int
	Notify      int
	Queued      int
	Ambient     int
	Silent      int
	TopTriggers []Trigger
}

// NewDecisionReport creates an empty decision report.
func NewDecisionReport() *DecisionReport {
	return &DecisionReport{
		CountByLevel:    make(map[Level]int),
		CircleSummaries: make(map[identity.EntityID]*CircleDecisionSummary),
	}
}
