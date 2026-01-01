package held

import (
	"time"
)

// Engine produces HeldSummary projections deterministically.
type Engine struct {
	// clock provides the current time (injected for determinism).
	clock func() time.Time
}

// NewEngine creates a new held projection engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// statements are calm explanatory sentences.
// Selected deterministically based on magnitude.
var statements = map[string]string{
	"nothing": "Everything that could need you has been considered. Nothing does.",
	"a_few":   "There are a few things we're holding quietly for you. None of them need you today.",
	"several": "We're holding several things quietly. None of them are urgent.",
}

// Generate produces a HeldSummary from the given input.
// Output is deterministic: same input + same clock = same output.
func (e *Engine) Generate(input HeldInput) HeldSummary {
	now := e.clock()
	input.Now = now

	summary := HeldSummary{
		GeneratedAt: now,
		Categories:  make([]CategorySummary, 0, 3),
	}

	// Compute total held count (abstract)
	totalHeld := input.SuppressedObligationCount + input.PolicyBlockedCount

	// Determine magnitude (bucketed, never specific)
	summary.Magnitude = computeMagnitude(totalHeld)

	// Select statement based on magnitude
	summary.Statement = statements[summary.Magnitude]

	// Build category summaries (max 3)
	categories := e.buildCategories(input)
	if len(categories) > 3 {
		categories = categories[:3]
	}
	summary.Categories = categories

	// Compute hash
	summary.Hash = summary.ComputeHash()

	return summary
}

// computeMagnitude converts a count to a bucketed magnitude.
// CRITICAL: Never expose specific numbers.
func computeMagnitude(count int) string {
	switch {
	case count == 0:
		return "nothing"
	case count <= 3:
		return "a_few"
	default:
		return "several"
	}
}

// buildCategories constructs abstract category summaries.
func (e *Engine) buildCategories(input HeldInput) []CategorySummary {
	var categories []CategorySummary

	// Determine primary reason based on input signals
	primaryReason := ReasonNotUrgent
	if input.QuietHoursActive {
		primaryReason = ReasonQuietHours
	} else if input.PolicyBlockedCount > 0 {
		primaryReason = ReasonProtectedByPolicy
	}

	// Add categories in deterministic order (alphabetical by category name)
	if input.HasHomeItems {
		categories = append(categories, CategorySummary{
			Category:      CategoryHome,
			Presence:      true,
			PrimaryReason: primaryReason,
		})
	}

	if input.HasMoneyItems {
		categories = append(categories, CategorySummary{
			Category:      CategoryMoney,
			Presence:      true,
			PrimaryReason: primaryReason,
		})
	}

	if input.HasPeopleItems {
		categories = append(categories, CategorySummary{
			Category:      CategoryPeople,
			Presence:      true,
			PrimaryReason: primaryReason,
		})
	}

	if input.HasTimeItems {
		categories = append(categories, CategorySummary{
			Category:      CategoryTime,
			Presence:      true,
			PrimaryReason: primaryReason,
		})
	}

	if input.HasWorkItems {
		categories = append(categories, CategorySummary{
			Category:      CategoryWork,
			Presence:      true,
			PrimaryReason: primaryReason,
		})
	}

	return categories
}

// DefaultInput returns a default held input for demo/testing.
func DefaultInput() HeldInput {
	return HeldInput{
		SuppressedObligationCount: 2,
		PolicyBlockedCount:        1,
		QuietHoursActive:          false,
		HasTimeItems:              true,
		HasMoneyItems:             true,
		HasPeopleItems:            false,
		HasWorkItems:              true,
		HasHomeItems:              false,
		CircleID:                  "demo-circle",
	}
}

// EmptyInput returns an input with nothing held.
func EmptyInput() HeldInput {
	return HeldInput{
		SuppressedObligationCount: 0,
		PolicyBlockedCount:        0,
		QuietHoursActive:          false,
		HasTimeItems:              false,
		HasMoneyItems:             false,
		HasPeopleItems:            false,
		HasWorkItems:              false,
		HasHomeItems:              false,
		CircleID:                  "demo-circle",
	}
}

// CategoryDisplayName returns a human-friendly name for a category.
func CategoryDisplayName(c Category) string {
	switch c {
	case CategoryTime:
		return "Time"
	case CategoryMoney:
		return "Money"
	case CategoryPeople:
		return "People"
	case CategoryWork:
		return "Work"
	case CategoryHome:
		return "Home"
	default:
		return string(c)
	}
}
