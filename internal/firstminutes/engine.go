// Package firstminutes provides the engine for Phase 26B: First Five Minutes Proof.
//
// This engine reads from existing stores to compute a narrative summary.
// It has NO side effects, NO goroutines, and uses clock injection only.
//
// CRITICAL: This is not analytics. This is narrative proof.
package firstminutes

import (
	"sort"
	"time"

	"quantumlife/pkg/domain/firstminutes"
)

// Engine computes First Minutes summaries from existing store data.
// It is deterministic: same inputs always produce the same output.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new First Minutes engine with clock injection.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// calmLinePatterns maps signal patterns to calm lines.
// Selection is deterministic based on which signals are present.
var calmLinePatterns = []struct {
	pattern  func(signals []firstminutes.FirstMinutesSignal) bool
	calmLine string
}{
	// Most specific patterns first
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalActionExecuted)
		},
		calmLine: "One action happened. Silence resumed.",
	},
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalActionPreviewed) &&
				!hasSignal(signals, firstminutes.SignalActionExecuted)
		},
		calmLine: "One action was previewed. Nothing else needed you.",
	},
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalHeld)
		},
		calmLine: "A few things were seen and held without interruption.",
	},
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalConnected) &&
				hasSignal(signals, firstminutes.SignalSynced) &&
				hasSignal(signals, firstminutes.SignalMirrored)
		},
		calmLine: "Your data arrived. We watched quietly.",
	},
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalSynced) ||
				hasSignal(signals, firstminutes.SignalMirrored)
		},
		calmLine: "Data was seen quietly. Nothing more was needed.",
	},
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalConnected)
		},
		calmLine: "You connected once, and we stayed quiet.",
	},
	{
		pattern: func(signals []firstminutes.FirstMinutesSignal) bool {
			return hasSignal(signals, firstminutes.SignalSilencePreserved)
		},
		calmLine: "Nothing happened - and that was the point.",
	},
}

// defaultCalmLine is used when no pattern matches (shouldn't happen with proper signals).
const defaultCalmLine = "Quiet is the outcome."

// hasSignal checks if a signal kind is present in the signals list.
func hasSignal(signals []firstminutes.FirstMinutesSignal, kind firstminutes.FirstMinutesSignalKind) bool {
	for _, s := range signals {
		if s.Kind == kind {
			return true
		}
	}
	return false
}

// ComputeSummary computes a First Minutes summary from inputs.
// Returns nil if there are no meaningful signals (silence is success).
//
// This method has NO side effects. It only reads inputs and returns output.
func (e *Engine) ComputeSummary(inputs *firstminutes.FirstMinutesInputs) *firstminutes.FirstMinutesSummary {
	if inputs == nil {
		return nil
	}

	// If dismissed with matching state, return nil
	if inputs.DismissedSummaryHash != "" {
		// Compute what the current hash would be
		testSummary := e.computeSummaryInternal(inputs)
		if testSummary != nil {
			testSummary.StatusHash = testSummary.ComputeStatusHash()
			if testSummary.StatusHash == inputs.DismissedSummaryHash {
				return nil // Already dismissed
			}
		}
	}

	// No meaningful signals = silence is success (return nil)
	if !inputs.HasMeaningfulSignals() {
		return nil
	}

	return e.computeSummaryInternal(inputs)
}

// computeSummaryInternal builds the summary without dismissal checks.
func (e *Engine) computeSummaryInternal(inputs *firstminutes.FirstMinutesInputs) *firstminutes.FirstMinutesSummary {
	signals := e.collectSignals(inputs)
	if len(signals) == 0 {
		return nil
	}

	// Sort signals for determinism
	sort.Slice(signals, func(i, j int) bool {
		return string(signals[i].Kind) < string(signals[j].Kind)
	})

	// Select calm line deterministically
	calmLine := selectCalmLine(signals)

	summary := &firstminutes.FirstMinutesSummary{
		Period:   inputs.Period,
		Signals:  signals,
		CalmLine: calmLine,
	}

	// Compute status hash
	summary.StatusHash = summary.ComputeStatusHash()

	return summary
}

// collectSignals gathers signals from inputs.
func (e *Engine) collectSignals(inputs *firstminutes.FirstMinutesInputs) []firstminutes.FirstMinutesSignal {
	var signals []firstminutes.FirstMinutesSignal

	// Connected signal
	if inputs.HasConnection {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalConnected,
			Magnitude: firstminutes.MagnitudeAFew, // Connection is always "a_few" (1)
		})
	}

	// Synced signal
	if inputs.HasSyncReceipt {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalSynced,
			Magnitude: inputs.SyncMagnitude,
		})
	}

	// Mirrored signal
	if inputs.HasMirror {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalMirrored,
			Magnitude: inputs.MirrorMagnitude,
		})
	}

	// Held signal
	if inputs.HasHeldItems {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalHeld,
			Magnitude: inputs.HeldMagnitude,
		})
	}

	// Action previewed signal
	if inputs.ActionPreviewed {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalActionPreviewed,
			Magnitude: firstminutes.MagnitudeAFew, // Preview is always "a_few" (1)
		})
	}

	// Action executed signal
	if inputs.ActionExecuted {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalActionExecuted,
			Magnitude: firstminutes.MagnitudeAFew, // Execution is always "a_few" (1)
		})
	}

	// If we have connection/sync but no other activity, add silence preserved
	if len(signals) > 0 && !inputs.ActionPreviewed && !inputs.ActionExecuted && !inputs.HasHeldItems {
		signals = append(signals, firstminutes.FirstMinutesSignal{
			Kind:      firstminutes.SignalSilencePreserved,
			Magnitude: firstminutes.MagnitudeNothing,
		})
	}

	return signals
}

// selectCalmLine selects the appropriate calm line based on signals.
// Selection is deterministic.
func selectCalmLine(signals []firstminutes.FirstMinutesSignal) string {
	for _, pattern := range calmLinePatterns {
		if pattern.pattern(signals) {
			return pattern.calmLine
		}
	}
	return defaultCalmLine
}

// ComputeCue computes the First Minutes cue for display on /today.
// Returns a cue with Available=false if:
// - No meaningful signals
// - Already dismissed for this period
// - Another cue is active (single whisper rule - caller must check)
func (e *Engine) ComputeCue(inputs *firstminutes.FirstMinutesInputs) *firstminutes.FirstMinutesCue {
	summary := e.ComputeSummary(inputs)
	if summary == nil {
		return &firstminutes.FirstMinutesCue{
			Available: false,
		}
	}

	return &firstminutes.FirstMinutesCue{
		Available:   true,
		CueText:     firstminutes.DefaultCueText,
		LinkText:    firstminutes.DefaultLinkText,
		SummaryHash: summary.StatusHash,
		Period:      summary.Period,
	}
}

// ShouldShowFirstMinutesCue returns whether the first minutes cue should show.
// Respects single whisper rule: returns false if another cue is active.
func (e *Engine) ShouldShowFirstMinutesCue(inputs *firstminutes.FirstMinutesInputs, otherCueActive bool) bool {
	if otherCueActive {
		return false // Single whisper rule
	}

	cue := e.ComputeCue(inputs)
	return cue.Available
}

// PeriodFromTime converts a time to a FirstMinutesPeriod (YYYY-MM-DD).
func PeriodFromTime(t time.Time) firstminutes.FirstMinutesPeriod {
	return firstminutes.FirstMinutesPeriod(t.Format("2006-01-02"))
}
