// Package commerceobserver provides the commerce observer engine.
//
// Phase 31: Commerce Observers (Silent by Default)
// Reference: docs/ADR/ADR-0062-phase31-commerce-observers.md
//
// Commerce Observers are NOT finance. They are NOT budgeting. They are NOT insights.
// They are long-horizon behavioral signals that MAY matter someday, but usually do not.
//
// CRITICAL INVARIANTS:
//   - NO amounts, NO merchant names, NO timestamps, NO items
//   - Only category buckets, frequency buckets, stability buckets
//   - Deterministic outputs: sorted inputs, canonical strings, SHA256 hashing
//   - Default outcome: NOTHING SHOWN
//   - No goroutines. No time.Now() - clock injection only.
//   - stdlib only.
//
// This phase is OBSERVATION ONLY. Commerce is observed. Nothing else.
package commerceobserver

import (
	"sort"
	"time"

	"quantumlife/pkg/domain/commerceobserver"
)

// Engine orchestrates commerce observation operations.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new commerce observer engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// Observe computes commerce observations from inputs.
// Returns empty slice if no meaningful patterns detected.
//
// CRITICAL: Inputs contain raw counts but outputs contain ONLY buckets.
// This is the boundary where raw data is abstracted.
func (e *Engine) Observe(inputs *commerceobserver.CommerceInputs) []commerceobserver.CommerceObservation {
	if inputs == nil {
		return nil
	}

	if len(inputs.CategoryCounts) == 0 {
		return nil
	}

	// Get sorted categories for determinism
	categories := make([]commerceobserver.CategoryBucket, 0, len(inputs.CategoryCounts))
	for cat := range inputs.CategoryCounts {
		categories = append(categories, cat)
	}
	sort.Slice(categories, func(i, j int) bool {
		return string(categories[i]) < string(categories[j])
	})

	observations := make([]commerceobserver.CommerceObservation, 0, len(categories))

	for _, cat := range categories {
		rawCount := inputs.CategoryCounts[cat]

		// Skip if no activity in this category
		if rawCount == 0 {
			continue
		}

		// Convert to frequency bucket (abstracts the raw count)
		frequency := commerceobserver.ToFrequencyBucket(rawCount)

		// Get trend and convert to stability
		trend := inputs.CategoryTrends[cat]
		if trend == "" {
			trend = "stable"
		}
		stability := commerceobserver.TrendToStability(trend)

		// Build evidence hash from abstract tokens only
		evidenceTokens := []string{
			string(cat),
			string(frequency),
			string(stability),
			inputs.Period,
		}
		evidenceHash := commerceobserver.ComputeEvidenceHash(evidenceTokens)

		obs := commerceobserver.CommerceObservation{
			Category:     cat,
			Frequency:    frequency,
			Stability:    stability,
			Period:       inputs.Period,
			EvidenceHash: evidenceHash,
		}

		observations = append(observations, obs)
	}

	return observations
}

// BuildMirrorPage builds the commerce mirror proof page.
// Returns nil if no observations (silence is success).
//
// CRITICAL: Page contains NO raw data, NO identifiable info.
// Only: title, calm lines, category buckets, status hash.
func (e *Engine) BuildMirrorPage(observations []commerceobserver.CommerceObservation) *commerceobserver.CommerceMirrorPage {
	return commerceobserver.NewCommerceMirrorPage(observations)
}

// ShouldShowCommerceCue determines if the commerce cue should be shown.
// Respects single whisper rule: returns false if another cue is already active.
//
// Returns true only if:
//   - Commerce data is connected
//   - At least one observation exists
//   - No higher-priority whisper is active
func (e *Engine) ShouldShowCommerceCue(hasConnection bool, observations []commerceobserver.CommerceObservation, otherCueActive bool) bool {
	// Single whisper rule: defer to other cues
	if otherCueActive {
		return false
	}

	// Must be connected
	if !hasConnection {
		return false
	}

	// Must have at least one observation
	if len(observations) == 0 {
		return false
	}

	return true
}

// ComputeCue computes the commerce cue for display.
// Returns a cue with Available=false if no observations.
func (e *Engine) ComputeCue(observations []commerceobserver.CommerceObservation) *commerceobserver.CommerceCue {
	if len(observations) == 0 {
		return commerceobserver.NewCommerceCue(false)
	}
	return commerceobserver.NewCommerceCue(true)
}

// PeriodFromTime converts a time to a period string (ISO week format).
// Format: "2024-W03" (year-week)
func PeriodFromTime(t time.Time) string {
	year, week := t.ISOWeek()
	return formatWeekPeriod(year, week)
}

// formatWeekPeriod formats a year and week number as a period string.
func formatWeekPeriod(year, week int) string {
	// Use simple string formatting (stdlib only)
	weekStr := "0"
	if week < 10 {
		weekStr = "0" + string(rune('0'+week))
	} else {
		weekStr = string(rune('0'+week/10)) + string(rune('0'+week%10))
	}
	yearStr := ""
	for i := 0; i < 4; i++ {
		digit := (year / pow10(3-i)) % 10
		yearStr += string(rune('0' + digit))
	}
	return yearStr + "-W" + weekStr
}

// pow10 returns 10^n for small n.
func pow10(n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// ObservationsForPeriod filters observations to a specific period.
func ObservationsForPeriod(observations []commerceobserver.CommerceObservation, period string) []commerceobserver.CommerceObservation {
	if period == "" {
		return observations
	}

	filtered := make([]commerceobserver.CommerceObservation, 0)
	for _, obs := range observations {
		if obs.Period == period {
			filtered = append(filtered, obs)
		}
	}
	return filtered
}

// HasStablePatterns checks if any observations show stable patterns.
// Used to select appropriate calm lines.
func HasStablePatterns(observations []commerceobserver.CommerceObservation) bool {
	for _, obs := range observations {
		if obs.Stability == commerceobserver.StabilityStable {
			return true
		}
	}
	return false
}

// CountByCategory returns the number of observations per category.
// Returns magnitude buckets, not raw counts.
func CountByCategory(observations []commerceobserver.CommerceObservation) map[commerceobserver.CategoryBucket]int {
	counts := make(map[commerceobserver.CategoryBucket]int)
	for _, obs := range observations {
		counts[obs.Category]++
	}
	return counts
}
