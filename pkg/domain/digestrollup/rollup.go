// Package digestrollup - rollup builder.
package digestrollup

import (
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

// BuildRollup aggregates interruptions into deduplicated rollup items.
// Items with the same DigestKey are merged, tracking max level, counts, and timestamps.
func BuildRollup(interruptions []*interrupt.Interruption) []RollupItem {
	if len(interruptions) == 0 {
		return nil
	}

	// Group by DigestKey
	rollupMap := make(map[DigestKey]*RollupItem)

	for _, intr := range interruptions {
		key := ComputeDigestKey(intr)

		existing, ok := rollupMap[key]
		if !ok {
			// First occurrence
			rollupMap[key] = &RollupItem{
				Key:             key,
				CircleID:        intr.CircleID,
				MaxLevel:        intr.Level,
				MaxRegret:       intr.RegretScore,
				OccurrenceCount: 1,
				FirstSeen:       intr.CreatedAt,
				LastSeen:        intr.CreatedAt,
				Summary:         cleanSummary(intr.Summary),
				Trigger:         intr.Trigger,
				LevelCounts:     map[interrupt.Level]int{intr.Level: 1},
			}
			continue
		}

		// Merge with existing
		existing.OccurrenceCount++

		// Update max level
		if interrupt.LevelOrder(intr.Level) > interrupt.LevelOrder(existing.MaxLevel) {
			existing.MaxLevel = intr.Level
		}

		// Update max regret
		if intr.RegretScore > existing.MaxRegret {
			existing.MaxRegret = intr.RegretScore
		}

		// Update timestamps
		if intr.CreatedAt.Before(existing.FirstSeen) {
			existing.FirstSeen = intr.CreatedAt
		}
		if intr.CreatedAt.After(existing.LastSeen) {
			existing.LastSeen = intr.CreatedAt
		}

		// Track level counts
		existing.LevelCounts[intr.Level]++
	}

	// Convert map to slice
	result := make([]RollupItem, 0, len(rollupMap))
	for _, item := range rollupMap {
		result = append(result, *item)
	}

	// Sort deterministically
	SortRollupItems(result)

	return result
}

// BuildRollupByCircle groups rollup items by circle ID.
func BuildRollupByCircle(interruptions []*interrupt.Interruption) map[identity.EntityID][]RollupItem {
	if len(interruptions) == 0 {
		return nil
	}

	// Group interruptions by circle first
	byCircle := make(map[identity.EntityID][]*interrupt.Interruption)
	for _, intr := range interruptions {
		byCircle[intr.CircleID] = append(byCircle[intr.CircleID], intr)
	}

	// Build rollup for each circle
	result := make(map[identity.EntityID][]RollupItem)
	for circleID, circleInterruptions := range byCircle {
		result[circleID] = BuildRollup(circleInterruptions)
	}

	return result
}

// cleanSummary removes quota annotations like "(quota)" from summaries
// to ensure stable summaries across rollups.
func cleanSummary(summary string) string {
	// Remove "(quota)" suffix if present
	if len(summary) > 8 && summary[len(summary)-8:] == " (quota)" {
		return summary[:len(summary)-8]
	}
	return summary
}

// TopNByCircle returns the top N rollup items per circle.
func TopNByCircle(rollupByCircle map[identity.EntityID][]RollupItem, n int) map[identity.EntityID][]RollupItem {
	result := make(map[identity.EntityID][]RollupItem)
	for circleID, items := range rollupByCircle {
		if len(items) > n {
			result[circleID] = items[:n]
		} else {
			result[circleID] = items
		}
	}
	return result
}
