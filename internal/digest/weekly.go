// Package digest generates weekly summaries from interruption data.
//
// The weekly digest aggregates interruptions over a 7-day period,
// providing circle-by-circle summaries and pattern observations.
//
// Phase 3.1: Uses digestrollup for deduplication and trend tracking.
// Same underlying condition appearing multiple times shows as ONE line
// with max_level, occurrence_count, and trend indicator.
//
// CRITICAL: Deterministic. Same inputs + same clock = same digest.
// CRITICAL: Synchronous processing, no goroutines.
// CRITICAL: Read-only. Generates text, never acts.
//
// Reference: docs/ADR/ADR-0020-phase3-interruptions-and-digest.md
package digest

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/digestrollup"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

// WeeklyDigest represents a week's worth of interruption summaries.
type WeeklyDigest struct {
	// WeekStart is the Monday of the digest week (UTC).
	WeekStart time.Time

	// WeekEnd is the Sunday of the digest week (UTC).
	WeekEnd time.Time

	// GeneratedAt is when this digest was created.
	GeneratedAt time.Time

	// CircleSummaries contains per-circle stats.
	CircleSummaries map[identity.EntityID]*CircleSummary

	// TotalInterruptions across all circles.
	TotalInterruptions int

	// ByLevel counts interruptions by level.
	ByLevel map[interrupt.Level]int

	// Observations are pattern-based notes.
	Observations []string

	// Hash is deterministic content hash.
	Hash string
}

// CircleSummary holds weekly stats for one circle.
type CircleSummary struct {
	CircleID   identity.EntityID
	CircleName string

	// Counts by level
	Urgent  int
	Notify  int
	Queued  int
	Ambient int
	Silent  int
	Total   int

	// UniqueConditions is the count of unique underlying conditions.
	UniqueConditions int

	// Top triggers seen
	TopTriggers []interrupt.Trigger

	// PeakDay is the day with most interruptions (0=Mon, 6=Sun).
	PeakDay int

	// RollupItems are the deduplicated top items (max 5).
	// Phase 3.1: Each item represents a unique condition with:
	// - max_level, occurrence_count, first_seen, last_seen, trend
	RollupItems []digestrollup.RollupItem

	// TopItems are the highest-regret items of the week (max 3).
	// Deprecated: Use RollupItems instead. Kept for backward compatibility.
	TopItems []DigestItem
}

// DigestItem represents a notable item in the digest.
// Deprecated: Use digestrollup.RollupItem instead.
type DigestItem struct {
	Summary     string
	Level       interrupt.Level
	RegretScore int
	Trigger     interrupt.Trigger
	Day         time.Weekday
}

// Generator creates weekly digests.
type Generator struct {
	clk clock.Clock
}

// NewGenerator creates a new digest generator.
func NewGenerator(clk clock.Clock) *Generator {
	return &Generator{clk: clk}
}

// DailyBucket holds one day's interruptions for digest processing.
type DailyBucket struct {
	Date          time.Time
	Interruptions []*interrupt.Interruption
}

// Generate creates a weekly digest from daily buckets.
// Expects 7 days of data (Monday-Sunday).
// Phase 3.1: Uses rollup for deduplication - same condition appears once with counts.
func (g *Generator) Generate(weekStart time.Time, dailyBuckets []DailyBucket, circleNames map[identity.EntityID]string) *WeeklyDigest {
	now := g.clk.Now()

	digest := &WeeklyDigest{
		WeekStart:       weekStart,
		WeekEnd:         weekStart.AddDate(0, 0, 6),
		GeneratedAt:     now,
		CircleSummaries: make(map[identity.EntityID]*CircleSummary),
		ByLevel:         make(map[interrupt.Level]int),
	}

	// Collect all interruptions across all days
	var allInterruptions []*interrupt.Interruption
	for _, bucket := range dailyBuckets {
		allInterruptions = append(allInterruptions, bucket.Interruptions...)
	}

	// Process all daily buckets for raw counts
	for _, bucket := range dailyBuckets {
		for _, intr := range bucket.Interruptions {
			g.processInterruption(digest, intr, bucket.Date, circleNames)
		}
	}

	// Phase 3.1: Build rollups by circle for deduplicated top items
	rollupByCircle := digestrollup.BuildRollupByCircle(allInterruptions)
	topRollups := digestrollup.TopNByCircle(rollupByCircle, 5) // Top 5 per circle

	// Populate RollupItems in each circle summary
	for circleID, rollups := range topRollups {
		if summary, ok := digest.CircleSummaries[circleID]; ok {
			summary.RollupItems = rollups
			// Count unique conditions from full rollup (not just top N)
			if fullRollup, ok := rollupByCircle[circleID]; ok {
				summary.UniqueConditions = len(fullRollup)
			}
		}
	}

	// Generate observations
	digest.Observations = g.generateObservations(digest)

	// Compute hash
	digest.Hash = g.computeHash(digest)

	return digest
}

// processInterruption adds an interruption to the digest.
func (g *Generator) processInterruption(digest *WeeklyDigest, intr *interrupt.Interruption, day time.Time, circleNames map[identity.EntityID]string) {
	digest.TotalInterruptions++
	digest.ByLevel[intr.Level]++

	// Get or create circle summary
	summary, ok := digest.CircleSummaries[intr.CircleID]
	if !ok {
		name := string(intr.CircleID)
		if n, ok := circleNames[intr.CircleID]; ok {
			name = n
		}
		summary = &CircleSummary{
			CircleID:   intr.CircleID,
			CircleName: name,
		}
		digest.CircleSummaries[intr.CircleID] = summary
	}

	summary.Total++

	// Count by level
	switch intr.Level {
	case interrupt.LevelUrgent:
		summary.Urgent++
	case interrupt.LevelNotify:
		summary.Notify++
	case interrupt.LevelQueued:
		summary.Queued++
	case interrupt.LevelAmbient:
		summary.Ambient++
	case interrupt.LevelSilent:
		summary.Silent++
	}

	// Track triggers (up to 5)
	g.addTrigger(summary, intr.Trigger)

	// Track top items (by regret, up to 3)
	g.addTopItem(summary, intr, day)
}

// addTrigger adds a trigger to the summary if not already present.
func (g *Generator) addTrigger(summary *CircleSummary, trigger interrupt.Trigger) {
	for _, t := range summary.TopTriggers {
		if t == trigger {
			return
		}
	}
	if len(summary.TopTriggers) < 5 {
		summary.TopTriggers = append(summary.TopTriggers, trigger)
	}
}

// addTopItem adds a high-regret item to the summary.
func (g *Generator) addTopItem(summary *CircleSummary, intr *interrupt.Interruption, day time.Time) {
	item := DigestItem{
		Summary:     intr.Summary,
		Level:       intr.Level,
		RegretScore: intr.RegretScore,
		Trigger:     intr.Trigger,
		Day:         day.Weekday(),
	}

	summary.TopItems = append(summary.TopItems, item)

	// Sort by regret descending
	sort.Slice(summary.TopItems, func(i, j int) bool {
		return summary.TopItems[i].RegretScore > summary.TopItems[j].RegretScore
	})

	// Keep only top 3
	if len(summary.TopItems) > 3 {
		summary.TopItems = summary.TopItems[:3]
	}
}

// generateObservations creates pattern-based notes.
func (g *Generator) generateObservations(digest *WeeklyDigest) []string {
	var observations []string

	// Observation: High urgent count
	if digest.ByLevel[interrupt.LevelUrgent] > 5 {
		observations = append(observations,
			fmt.Sprintf("High urgent count this week (%d). Consider reviewing priorities.",
				digest.ByLevel[interrupt.LevelUrgent]))
	}

	// Observation: Dominant circle
	var maxCircle identity.EntityID
	var maxCount int
	for id, summary := range digest.CircleSummaries {
		if summary.Total > maxCount {
			maxCount = summary.Total
			maxCircle = id
		}
	}
	if maxCount > 0 && len(digest.CircleSummaries) > 1 {
		if summary, ok := digest.CircleSummaries[maxCircle]; ok {
			pct := (maxCount * 100) / digest.TotalInterruptions
			if pct > 60 {
				observations = append(observations,
					fmt.Sprintf("%s dominated this week (%d%% of interruptions).",
						summary.CircleName, pct))
			}
		}
	}

	// Observation: Low activity
	if digest.TotalInterruptions < 5 {
		observations = append(observations, "Light week with few interruptions.")
	}

	// Observation: No urgent items
	if digest.ByLevel[interrupt.LevelUrgent] == 0 && digest.TotalInterruptions > 0 {
		observations = append(observations, "No urgent items this week. Well managed!")
	}

	return observations
}

// computeHash generates a deterministic hash of the digest.
func (g *Generator) computeHash(digest *WeeklyDigest) string {
	if digest.TotalInterruptions == 0 {
		return "empty"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("week:%s|", digest.WeekStart.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("total:%d|", digest.TotalInterruptions))

	// Add level counts in deterministic order
	levels := []interrupt.Level{
		interrupt.LevelUrgent,
		interrupt.LevelNotify,
		interrupt.LevelQueued,
		interrupt.LevelAmbient,
		interrupt.LevelSilent,
	}
	for _, level := range levels {
		sb.WriteString(fmt.Sprintf("%s:%d|", level, digest.ByLevel[level]))
	}

	// Add circle IDs sorted
	var circleIDs []string
	for id := range digest.CircleSummaries {
		circleIDs = append(circleIDs, string(id))
	}
	sort.Strings(circleIDs)
	for _, id := range circleIDs {
		summary := digest.CircleSummaries[identity.EntityID(id)]
		sb.WriteString(fmt.Sprintf("circle:%s:%d|", id, summary.Total))
	}

	return interrupt.HashCanonical("digest", sb.String())
}

// FormatText generates a human-readable text representation of the digest.
// Phase 3.1: Shows deduplicated items with occurrence counts and trends.
func (digest *WeeklyDigest) FormatText() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Weekly Digest: %s to %s\n",
		digest.WeekStart.Format("Jan 2"),
		digest.WeekEnd.Format("Jan 2, 2006")))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Overall stats
	sb.WriteString(fmt.Sprintf("Total Interruptions: %d\n", digest.TotalInterruptions))
	if digest.ByLevel[interrupt.LevelUrgent] > 0 {
		sb.WriteString(fmt.Sprintf("  Urgent: %d\n", digest.ByLevel[interrupt.LevelUrgent]))
	}
	if digest.ByLevel[interrupt.LevelNotify] > 0 {
		sb.WriteString(fmt.Sprintf("  Notify: %d\n", digest.ByLevel[interrupt.LevelNotify]))
	}
	if digest.ByLevel[interrupt.LevelQueued] > 0 {
		sb.WriteString(fmt.Sprintf("  Queued: %d\n", digest.ByLevel[interrupt.LevelQueued]))
	}
	if digest.ByLevel[interrupt.LevelAmbient] > 0 {
		sb.WriteString(fmt.Sprintf("  Ambient: %d\n", digest.ByLevel[interrupt.LevelAmbient]))
	}
	sb.WriteString("\n")

	// Circle summaries (sorted by total descending)
	var circleIDs []identity.EntityID
	for id := range digest.CircleSummaries {
		circleIDs = append(circleIDs, id)
	}
	sort.Slice(circleIDs, func(i, j int) bool {
		return digest.CircleSummaries[circleIDs[i]].Total > digest.CircleSummaries[circleIDs[j]].Total
	})

	for _, id := range circleIDs {
		summary := digest.CircleSummaries[id]
		sb.WriteString(fmt.Sprintf("--- %s ---\n", summary.CircleName))
		sb.WriteString(fmt.Sprintf("  Total: %d", summary.Total))
		if summary.UniqueConditions > 0 && summary.UniqueConditions != summary.Total {
			sb.WriteString(fmt.Sprintf(" (%d unique)", summary.UniqueConditions))
		}
		if summary.Urgent > 0 {
			sb.WriteString(fmt.Sprintf(" | Urgent: %d", summary.Urgent))
		}
		sb.WriteString("\n")

		// Phase 3.1: Show rollup items (deduplicated with counts)
		if len(summary.RollupItems) > 0 {
			sb.WriteString("  Top items:\n")
			for _, item := range summary.RollupItems {
				// Format: - Summary [LEVEL] x3 ↑
				line := fmt.Sprintf("    - %s [%s]", item.Summary, item.MaxLevel)
				if occ := item.FormatOccurrence(); occ != "" {
					line += " " + occ
				}
				if trend := item.TrendIndicator(); trend != "" {
					line += " " + trend
				}
				sb.WriteString(line + "\n")
			}
		} else if len(summary.TopItems) > 0 {
			// Fallback to deprecated TopItems if no rollups
			sb.WriteString("  Top items:\n")
			for _, item := range summary.TopItems {
				sb.WriteString(fmt.Sprintf("    - %s [%s]\n", item.Summary, item.Level))
			}
		}
		sb.WriteString("\n")
	}

	// Observations
	if len(digest.Observations) > 0 {
		sb.WriteString("Observations:\n")
		for _, obs := range digest.Observations {
			sb.WriteString(fmt.Sprintf("  * %s\n", obs))
		}
	}

	return sb.String()
}
