// Package digestrollup provides rollup/deduplication for weekly digest items.
//
// When the same underlying condition appears multiple times across a week
// (e.g., "low balance" each day), the digest should show ONE line with:
// - max_level observed
// - occurrence_count
// - first_seen, last_seen timestamps
//
// CRITICAL: Deterministic. Same inputs = same rollup.
// CRITICAL: Uses canonical strings for hashing, NOT JSON.
package digestrollup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

// DigestKey uniquely identifies an underlying condition for rollup purposes.
// Items with the same DigestKey are rolled up into a single RollupItem.
type DigestKey string

// RollupItem represents a deduplicated digest entry.
type RollupItem struct {
	// Key is the stable identifier for this condition.
	Key DigestKey

	// CircleID identifies which circle this belongs to.
	CircleID identity.EntityID

	// MaxLevel is the highest level observed this week.
	MaxLevel interrupt.Level

	// MaxRegret is the highest regret score observed.
	MaxRegret int

	// OccurrenceCount is how many times this appeared.
	OccurrenceCount int

	// FirstSeen is the earliest occurrence timestamp.
	FirstSeen time.Time

	// LastSeen is the latest occurrence timestamp.
	LastSeen time.Time

	// Summary is the stable summary text (from first occurrence).
	Summary string

	// Trigger is the trigger type.
	Trigger interrupt.Trigger

	// LevelCounts tracks how many times each level was seen.
	LevelCounts map[interrupt.Level]int
}

// ComputeDigestKey generates a stable key from an interruption.
// The key is based on: circleID | trigger | sourceEventID (without time bucket).
// This ensures the same underlying condition rolls up across days.
func ComputeDigestKey(intr *interrupt.Interruption) DigestKey {
	canonical := fmt.Sprintf("%s|%s|%s",
		intr.CircleID,
		intr.Trigger,
		intr.SourceEventID,
	)
	hash := sha256.Sum256([]byte(canonical))
	return DigestKey(hex.EncodeToString(hash[:8])) // 16 hex chars
}

// CanonicalString returns a deterministic string representation for hashing.
func (r *RollupItem) CanonicalString() string {
	return fmt.Sprintf("rollup|%s|%s|%s|%d|%d|%d|%s|%s",
		r.Key,
		r.CircleID,
		r.MaxLevel,
		r.MaxRegret,
		r.OccurrenceCount,
		r.FirstSeen.Unix(),
		r.LastSeen.Format(time.RFC3339),
		r.Summary,
	)
}

// Hash returns a deterministic hash of the rollup item.
func (r *RollupItem) Hash() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// SortRollupItems sorts rollup items deterministically:
// 1. MaxLevel descending (Urgent first)
// 2. MaxRegret descending
// 3. LastSeen descending (most recent first)
// 4. Key ascending (stable tiebreaker)
func SortRollupItems(items []RollupItem) {
	sort.Slice(items, func(i, j int) bool {
		// Level descending
		orderI := interrupt.LevelOrder(items[i].MaxLevel)
		orderJ := interrupt.LevelOrder(items[j].MaxLevel)
		if orderI != orderJ {
			return orderI > orderJ
		}

		// Regret descending
		if items[i].MaxRegret != items[j].MaxRegret {
			return items[i].MaxRegret > items[j].MaxRegret
		}

		// LastSeen descending
		if !items[i].LastSeen.Equal(items[j].LastSeen) {
			return items[i].LastSeen.After(items[j].LastSeen)
		}

		// Key ascending (stable tiebreaker)
		return items[i].Key < items[j].Key
	})
}

// FormatOccurrence returns a human-readable occurrence string.
func (r *RollupItem) FormatOccurrence() string {
	if r.OccurrenceCount == 1 {
		return ""
	}
	return fmt.Sprintf("x%d", r.OccurrenceCount)
}

// TrendIndicator returns a trend indicator based on level changes.
// "↑" if max level is higher than average
// "→" if levels are stable
// "↓" if max level is lower than most occurrences (rare)
func (r *RollupItem) TrendIndicator() string {
	if r.OccurrenceCount <= 1 {
		return ""
	}

	// Count occurrences at max level vs lower levels
	maxLevelCount := r.LevelCounts[r.MaxLevel]
	totalAtLower := r.OccurrenceCount - maxLevelCount

	if totalAtLower > maxLevelCount {
		// More occurrences at lower levels, but we hit max at least once
		return "↑"
	}
	if maxLevelCount == r.OccurrenceCount {
		// All at same level
		return "→"
	}
	// Mixed levels, max is dominant
	return "↗"
}
