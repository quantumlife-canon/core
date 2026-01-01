package todayquietly

import (
	"time"
)

// Engine generates the "Today, quietly." page deterministically.
type Engine struct {
	// clock provides the current time (injected for determinism).
	clock func() time.Time
}

// NewEngine creates a new projection engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// observationTemplate defines a potential observation.
type observationTemplate struct {
	signal   string
	text     string
	requires func(ProjectionInput) bool
}

// observationPool contains all possible observations.
// Engine selects exactly 3 based on input signals.
var observationPool = []observationTemplate{
	{
		signal:   "work_family",
		text:     "Work and family both expect something from you this week.",
		requires: func(p ProjectionInput) bool { return p.HasWorkObligations && p.HasFamilyObligations },
	},
	{
		signal:   "open_conversations",
		text:     "There are conversations you're keeping open without deciding yet.",
		requires: func(p ProjectionInput) bool { return p.HasOpenConversations },
	},
	{
		signal:   "finance_present",
		text:     "Money isn't urgent today, but it isn't invisible either.",
		requires: func(p ProjectionInput) bool { return p.HasFinanceObligations },
	},
	{
		signal:   "calendar_attention",
		text:     "Your calendar has commitments, but your attention does not.",
		requires: func(p ProjectionInput) bool { return p.HasCalendarCommitments },
	},
	{
		signal:   "important_not_urgent",
		text:     "Some things are important — and not time-sensitive.",
		requires: func(p ProjectionInput) bool { return p.HasImportantNotTimeSensitive },
	},
	{
		signal:   "work_present",
		text:     "Work is present in your week, but it doesn't control it.",
		requires: func(p ProjectionInput) bool { return p.HasWorkObligations && !p.HasFamilyObligations },
	},
	{
		signal:   "family_present",
		text:     "Family matters are on your mind, without demanding action.",
		requires: func(p ProjectionInput) bool { return p.HasFamilyObligations && !p.HasWorkObligations },
	},
	{
		signal:   "multiple_circles",
		text:     "You hold multiple responsibilities — none of them are in conflict today.",
		requires: func(p ProjectionInput) bool { return p.CircleCount > 1 },
	},
}

// recognitionVariants contains recognition sentences.
// Selected deterministically based on input hash.
var recognitionVariants = []string{
	"Today is already full — even if nothing urgent is happening.",
	"Your day has weight, even when it looks quiet.",
}

// Generate produces a TodayQuietlyPage from the given input.
// Output is deterministic: same input + same clock = same output.
func (e *Engine) Generate(input ProjectionInput) TodayQuietlyPage {
	now := e.clock()
	input.Now = now

	page := TodayQuietlyPage{
		Title:       "Today, quietly.",
		Subtitle:    "Nothing needs you — unless it truly does.",
		GeneratedAt: now,
	}

	// Select recognition sentence deterministically
	inputHash := input.Hash()
	recognitionIndex := hashToIndex(inputHash, len(recognitionVariants))
	page.Recognition = recognitionVariants[recognitionIndex]

	// Collect applicable observations
	var candidates []QuietObservation
	for _, tmpl := range observationPool {
		if tmpl.requires(input) {
			obs := QuietObservation{
				Text:   tmpl.text,
				Signal: tmpl.signal,
				ID:     computeObservationID(tmpl.text, tmpl.signal),
			}
			candidates = append(candidates, obs)
		}
	}

	// Sort for determinism
	sortObservations(candidates)

	// Select exactly 3 observations
	page.Observations = selectThree(candidates, inputHash)

	// Set suppressed insight (always exactly 1)
	page.SuppressedInsight = SuppressedInsight{
		Title:  "There's one thing we chose not to surface yet.",
		Reason: "Because it doesn't need you today.",
	}

	// Set permission pivot
	page.PermissionPivot = PermissionPivot{
		Prompt:        "When something truly needs you, how should it reach you?",
		DefaultChoice: "quiet",
		Choices: []PermissionChoice{
			{
				Mode:      "quiet",
				Label:     "Don't interrupt me unless it matters.",
				IsDefault: true,
			},
			{
				Mode:      "show_all",
				Label:     "I want to see everything.",
				IsDefault: false,
			},
		},
	}

	// Compute page hash
	page.PageHash = page.ComputePageHash()

	return page
}

// selectThree selects exactly 3 observations from candidates.
// If fewer than 3 candidates, fills with fallback observations.
func selectThree(candidates []QuietObservation, inputHash string) []QuietObservation {
	result := make([]QuietObservation, 0, 3)

	// Take from candidates first
	for i := 0; i < len(candidates) && len(result) < 3; i++ {
		result = append(result, candidates[i])
	}

	// Fill with fallbacks if needed
	fallbacks := []QuietObservation{
		{
			Signal: "fallback_1",
			Text:   "Your day has shape, even without a plan.",
			ID:     computeObservationID("Your day has shape, even without a plan.", "fallback_1"),
		},
		{
			Signal: "fallback_2",
			Text:   "Nothing urgent is happening — that's worth noticing.",
			ID:     computeObservationID("Nothing urgent is happening — that's worth noticing.", "fallback_2"),
		},
		{
			Signal: "fallback_3",
			Text:   "The quiet parts of your life are still your life.",
			ID:     computeObservationID("The quiet parts of your life are still your life.", "fallback_3"),
		},
	}

	for i := 0; len(result) < 3 && i < len(fallbacks); i++ {
		result = append(result, fallbacks[i])
	}

	return result
}

// hashToIndex converts a hash to an index in range [0, max).
func hashToIndex(hash string, max int) int {
	if max <= 0 {
		return 0
	}
	// Use first 8 chars of hash as a number
	if len(hash) < 8 {
		return 0
	}
	var sum int
	for i := 0; i < 8; i++ {
		sum = sum*16 + hexValue(hash[i])
	}
	if sum < 0 {
		sum = -sum
	}
	return sum % max
}

// hexValue returns the numeric value of a hex character.
func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return 0
	}
}

// DefaultInput returns a default projection input for demo/testing.
func DefaultInput() ProjectionInput {
	return ProjectionInput{
		HasWorkObligations:           true,
		HasFamilyObligations:         true,
		HasFinanceObligations:        true,
		HasCalendarCommitments:       true,
		HasOpenConversations:         true,
		HasImportantNotTimeSensitive: true,
		CircleCount:                  3,
	}
}
