// Package obligation defines the obligation domain model for Phase 2.
//
// Obligations represent items requiring user attention. They are ephemeral -
// computed on demand from events, not persisted. This ensures determinism:
// same events + same clock = same obligations.
//
// CRITICAL: No store. Obligations are recomputed each time.
// CRITICAL: Deterministic ID generation via canonical strings (NOT JSON).
// CRITICAL: Centralized ordering and hashing for consistency.
//
// Reference: docs/ADR/ADR-0019-phase2-obligation-extraction.md
package obligation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// ObligationType categorizes what kind of attention is needed.
type ObligationType string

const (
	ObligationReply    ObligationType = "reply"    // Email needs reply
	ObligationAttend   ObligationType = "attend"   // Calendar event to attend
	ObligationPay      ObligationType = "pay"      // Payment due
	ObligationReview   ObligationType = "review"   // Review needed (email, transaction)
	ObligationDecide   ObligationType = "decide"   // Decision needed (calendar conflict, invite)
	ObligationFollowup ObligationType = "followup" // Follow-up on stale item
)

// AttentionHorizon indicates urgency bucket.
type AttentionHorizon string

const (
	HorizonToday   AttentionHorizon = "today"   // Due today
	Horizon24h     AttentionHorizon = "24h"     // Due within 24 hours
	Horizon7d      AttentionHorizon = "7d"      // Due within 7 days
	HorizonSomeday AttentionHorizon = "someday" // No specific deadline
)

// Severity indicates the impact level.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Obligation represents an item requiring user attention.
// Obligations are ephemeral - computed from events, not stored.
type Obligation struct {
	// ID is deterministic: sha256(canonical_string)[:16]
	ID string

	// Source identification
	CircleID      identity.EntityID
	SourceEventID string
	SourceType    string // "email", "calendar", "finance"

	// Classification
	Type     ObligationType
	Horizon  AttentionHorizon
	Severity Severity

	// Timing
	DueBy     *time.Time // nil if no specific deadline
	CreatedAt time.Time  // When the source event occurred

	// Scoring (deterministic, rule-based)
	RegretScore float64 // 0.0 - 1.0: probability of regret if ignored
	Confidence  float64 // 0.0 - 1.0: confidence in the extraction

	// Human-readable context
	Reason   string            // Short explanation: "Unread email from manager"
	Evidence map[string]string // Structured evidence fields

	// Behavior flags
	Suppressible bool // Can user snooze/dismiss?

	// Internal: canonical string used for ID generation
	canonicalStr string
}

// Evidence keys (standardized)
const (
	EvidenceKeySubject      = "subject"
	EvidenceKeySender       = "sender"
	EvidenceKeySenderDomain = "sender_domain"
	EvidenceKeyEventTitle   = "event_title"
	EvidenceKeyMerchant     = "merchant"
	EvidenceKeyAmount       = "amount"
	EvidenceKeyBalance      = "balance"
	EvidenceKeyThreshold    = "threshold"
	EvidenceKeyDueDate      = "due_date"
	EvidenceKeyConflictWith = "conflict_with"
)

// NewObligation creates an obligation with deterministic ID.
// The canonical string format is strictly defined for reproducibility.
func NewObligation(
	circleID identity.EntityID,
	sourceEventID string,
	sourceType string,
	obligationType ObligationType,
	createdAt time.Time,
) *Obligation {
	// Canonical string format: obligation:{circle}:{source_event}:{type}
	// This ensures same input always produces same ID
	canonicalStr := fmt.Sprintf("obligation:%s:%s:%s",
		circleID, sourceEventID, obligationType)

	hash := sha256.Sum256([]byte(canonicalStr))
	id := hex.EncodeToString(hash[:])[:16]

	return &Obligation{
		ID:            id,
		CircleID:      circleID,
		SourceEventID: sourceEventID,
		SourceType:    sourceType,
		Type:          obligationType,
		CreatedAt:     createdAt,
		Horizon:       HorizonSomeday, // Default, will be computed
		Severity:      SeverityMedium, // Default
		RegretScore:   0.0,
		Confidence:    0.0,
		Evidence:      make(map[string]string),
		Suppressible:  true,
		canonicalStr:  canonicalStr,
	}
}

// WithDueBy sets the due date and computes attention horizon.
func (o *Obligation) WithDueBy(dueBy time.Time, now time.Time) *Obligation {
	o.DueBy = &dueBy
	o.Horizon = ComputeHorizon(dueBy, now)
	return o
}

// WithScoring sets regret score and confidence.
func (o *Obligation) WithScoring(regret, confidence float64) *Obligation {
	// Clamp to [0, 1]
	if regret < 0 {
		regret = 0
	} else if regret > 1 {
		regret = 1
	}
	if confidence < 0 {
		confidence = 0
	} else if confidence > 1 {
		confidence = 1
	}
	o.RegretScore = regret
	o.Confidence = confidence
	return o
}

// WithReason sets the human-readable reason.
func (o *Obligation) WithReason(reason string) *Obligation {
	o.Reason = reason
	return o
}

// WithEvidence adds an evidence field.
func (o *Obligation) WithEvidence(key, value string) *Obligation {
	o.Evidence[key] = value
	return o
}

// WithSeverity sets the severity level.
func (o *Obligation) WithSeverity(severity Severity) *Obligation {
	o.Severity = severity
	return o
}

// WithSuppressible sets whether the obligation can be dismissed.
func (o *Obligation) WithSuppressible(suppressible bool) *Obligation {
	o.Suppressible = suppressible
	return o
}

// ComputeHorizon determines the attention horizon from due date.
func ComputeHorizon(dueBy time.Time, now time.Time) AttentionHorizon {
	until := dueBy.Sub(now)

	switch {
	case until <= 0:
		return HorizonToday // Overdue or due now
	case until <= 24*time.Hour:
		return Horizon24h
	case until <= 7*24*time.Hour:
		return Horizon7d
	default:
		return HorizonSomeday
	}
}

// CanonicalString returns the canonical representation for hashing.
// Format is strictly defined and does NOT use JSON.
func (o *Obligation) CanonicalString() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("id:%s", o.ID))
	parts = append(parts, fmt.Sprintf("circle:%s", o.CircleID))
	parts = append(parts, fmt.Sprintf("source:%s", o.SourceEventID))
	parts = append(parts, fmt.Sprintf("type:%s", o.Type))
	parts = append(parts, fmt.Sprintf("horizon:%s", o.Horizon))
	parts = append(parts, fmt.Sprintf("severity:%s", o.Severity))
	parts = append(parts, fmt.Sprintf("regret:%.4f", o.RegretScore))
	parts = append(parts, fmt.Sprintf("confidence:%.4f", o.Confidence))

	if o.DueBy != nil {
		parts = append(parts, fmt.Sprintf("due:%d", o.DueBy.Unix()))
	}

	parts = append(parts, fmt.Sprintf("created:%d", o.CreatedAt.Unix()))

	// Evidence keys sorted for determinism
	if len(o.Evidence) > 0 {
		keys := make([]string, 0, len(o.Evidence))
		for k := range o.Evidence {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("ev.%s:%s", k, o.Evidence[k]))
		}
	}

	return strings.Join(parts, "|")
}

// HorizonOrder returns a sort key for horizon (lower = more urgent).
func HorizonOrder(h AttentionHorizon) int {
	switch h {
	case HorizonToday:
		return 0
	case Horizon24h:
		return 1
	case Horizon7d:
		return 2
	case HorizonSomeday:
		return 3
	default:
		return 99
	}
}

// SeverityOrder returns a sort key for severity (lower = more severe).
func SeverityOrder(s Severity) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 3
	default:
		return 99
	}
}

// SortObligations sorts obligations deterministically.
// Order: horizon ASC, regret DESC, due ASC (nil last), severity ASC, ID ASC
func SortObligations(obligations []*Obligation) {
	sort.SliceStable(obligations, func(i, j int) bool {
		a, b := obligations[i], obligations[j]

		// 1. Horizon (more urgent first)
		ha, hb := HorizonOrder(a.Horizon), HorizonOrder(b.Horizon)
		if ha != hb {
			return ha < hb
		}

		// 2. Regret score (higher first)
		if a.RegretScore != b.RegretScore {
			return a.RegretScore > b.RegretScore
		}

		// 3. Due date (earlier first, nil last)
		if a.DueBy != nil && b.DueBy != nil {
			if !a.DueBy.Equal(*b.DueBy) {
				return a.DueBy.Before(*b.DueBy)
			}
		} else if a.DueBy != nil {
			return true // a has due, b doesn't
		} else if b.DueBy != nil {
			return false // b has due, a doesn't
		}

		// 4. Severity (more severe first)
		sa, sb := SeverityOrder(a.Severity), SeverityOrder(b.Severity)
		if sa != sb {
			return sa < sb
		}

		// 5. ID for tie-breaking (deterministic)
		return a.ID < b.ID
	})
}

// ComputeObligationsHash computes a deterministic hash over a sorted slice.
func ComputeObligationsHash(obligations []*Obligation) string {
	if len(obligations) == 0 {
		return "empty"
	}

	// Obligations must be sorted before hashing
	sorted := make([]*Obligation, len(obligations))
	copy(sorted, obligations)
	SortObligations(sorted)

	var parts []string
	for _, o := range sorted {
		parts = append(parts, o.CanonicalString())
	}

	canonical := strings.Join(parts, "\n")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// FilterByHorizon returns obligations within the given horizons.
func FilterByHorizon(obligations []*Obligation, horizons ...AttentionHorizon) []*Obligation {
	horizonSet := make(map[AttentionHorizon]bool)
	for _, h := range horizons {
		horizonSet[h] = true
	}

	var result []*Obligation
	for _, o := range obligations {
		if horizonSet[o.Horizon] {
			result = append(result, o)
		}
	}
	return result
}

// FilterByMinRegret returns obligations with regret >= threshold.
func FilterByMinRegret(obligations []*Obligation, minRegret float64) []*Obligation {
	var result []*Obligation
	for _, o := range obligations {
		if o.RegretScore >= minRegret {
			result = append(result, o)
		}
	}
	return result
}

// FilterByCircle returns obligations for a specific circle.
func FilterByCircle(obligations []*Obligation, circleID identity.EntityID) []*Obligation {
	var result []*Obligation
	for _, o := range obligations {
		if o.CircleID == circleID {
			result = append(result, o)
		}
	}
	return result
}
