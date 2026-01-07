// Package interruptpreview implements the Phase 34 Permitted Interrupt Preview engine.
//
// The engine builds web-only preview cues and pages for permitted interrupt
// candidates. It surfaces abstract buckets only — never raw identifiers.
//
// CRITICAL INVARIANTS:
//   - NO notifications. Web-only preview.
//   - NO background work. No goroutines.
//   - NO raw identifiers. Hash-only, bucket-only.
//   - Deterministic: same inputs => same outputs.
//   - No side effects. Pure evaluation.
//
// Reference: docs/ADR/ADR-0070-phase34-interrupt-preview-web-only.md
package interruptpreview

import (
	"sort"

	"quantumlife/pkg/domain/interruptpreview"
)

// Engine builds preview cues and pages using deterministic rules.
// CRITICAL: No side effects. Pure function. Same inputs => same outputs.
type Engine struct{}

// NewEngine creates a new preview engine.
func NewEngine() *Engine {
	return &Engine{}
}

// SelectCandidate deterministically selects ONE candidate from the permitted set.
// Selection is by lowest SHA256 hash of CanonicalString(candidate)+periodKey.
// Returns nil if no candidates are available.
func (e *Engine) SelectCandidate(input *interruptpreview.PreviewInput) *interruptpreview.PreviewCandidate {
	if input == nil || len(input.PermittedCandidates) == 0 {
		return nil
	}

	// Compute selection hashes for all candidates
	for _, c := range input.PermittedCandidates {
		c.SelectionHash = c.ComputeSelectionHash(input.PeriodKey)
	}

	// Sort by selection hash (ascending)
	sorted := make([]*interruptpreview.PreviewCandidate, len(input.PermittedCandidates))
	copy(sorted, input.PermittedCandidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SelectionHash < sorted[j].SelectionHash
	})

	// Return the first (lowest hash)
	return sorted[0]
}

// BuildCue builds the preview cue for /today.
// Returns a cue with Available=false if no preview is available.
func (e *Engine) BuildCue(input *interruptpreview.PreviewInput) *interruptpreview.PreviewCue {
	cue := interruptpreview.DefaultPreviewCue()

	if input == nil {
		return cue
	}

	// If already dismissed or held, no cue
	if input.IsDismissed || input.IsHeld {
		cue.Available = false
		cue.StatusHash = cue.ComputeStatusHash()
		return cue
	}

	// Select a candidate
	candidate := e.SelectCandidate(input)
	if candidate == nil {
		cue.Available = false
		cue.StatusHash = cue.ComputeStatusHash()
		return cue
	}

	// We have a permitted, non-dismissed candidate
	cue.Available = true
	cue.Text = interruptpreview.DefaultPreviewCueText
	cue.LinkPath = interruptpreview.DefaultPreviewCuePath
	cue.Priority = interruptpreview.DefaultPreviewCuePriority
	cue.StatusHash = cue.ComputeStatusHash()

	return cue
}

// BuildPage builds the preview page for /interrupts/preview.
// Returns nil if no preview is available.
func (e *Engine) BuildPage(input *interruptpreview.PreviewInput) *interruptpreview.PreviewPage {
	if input == nil {
		return nil
	}

	// If already dismissed or held, no page
	if input.IsDismissed || input.IsHeld {
		return nil
	}

	// Select a candidate
	candidate := e.SelectCandidate(input)
	if candidate == nil {
		return nil
	}

	// Build the page with abstract labels only
	page := &interruptpreview.PreviewPage{
		Title:           "Available, if you want it.",
		Subtitle:        "This is time-sensitive, but still your choice.",
		Lines:           e.buildPageLines(candidate),
		CircleTypeLabel: candidate.CircleType.DisplayLabel(),
		HorizonLabel:    candidate.Horizon.DisplayLabel(),
		MagnitudeLabel:  e.magnitudeToLabel(candidate.Magnitude),
		ReasonLabel:     candidate.ReasonBucket.DisplayLabel(),
		AllowanceLabel:  candidate.Allowance.DisplayLabel(),
		HoldPath:        "/interrupts/preview/hold",
		DismissPath:     "/interrupts/preview/dismiss",
		BackLink:        "/today",
		CandidateHash:   candidate.CandidateHash,
		PeriodKey:       input.PeriodKey,
		CircleIDHash:    input.CircleIDHash,
	}
	page.StatusHash = page.ComputeStatusHash()

	return page
}

// buildPageLines builds the calm copy lines for the preview page.
func (e *Engine) buildPageLines(candidate *interruptpreview.PreviewCandidate) []string {
	lines := []string{}

	// Add horizon-based line
	switch candidate.Horizon {
	case interruptpreview.HorizonNow:
		lines = append(lines, "Something needs attention now.")
	case interruptpreview.HorizonSoon:
		lines = append(lines, "Something needs attention soon.")
	default:
		lines = append(lines, "Something may need your attention.")
	}

	// Add reminder about boundaries
	lines = append(lines, "Your boundaries are still being respected.")
	lines = append(lines, "You can hold this quietly if you prefer.")

	return lines
}

// magnitudeToLabel converts magnitude to a display label.
func (e *Engine) magnitudeToLabel(m interruptpreview.MagnitudeBucket) string {
	switch m {
	case interruptpreview.MagnitudeNothing:
		return "Nothing specific"
	case interruptpreview.MagnitudeAFew:
		return "A few items"
	case interruptpreview.MagnitudeSeveral:
		return "Several items"
	default:
		return "Unknown"
	}
}

// BuildProofPage builds the proof page for /proof/interrupts/preview.
func (e *Engine) BuildProofPage(input *interruptpreview.PreviewInput) *interruptpreview.PreviewProofPage {
	proof := interruptpreview.DefaultPreviewProofPage(input.PeriodKey)

	if input == nil {
		return proof
	}

	// Check if any preview was available
	candidate := e.SelectCandidate(input)
	proof.PreviewAvailable = candidate != nil

	// Check if user dismissed or held
	proof.UserDismissed = input.IsDismissed || input.IsHeld

	// Build lines based on state
	proof.Lines = e.buildProofLines(proof.PreviewAvailable, proof.UserDismissed)

	proof.StatusHash = proof.ComputeStatusHash()
	return proof
}

// buildProofLines builds the calm copy for the proof page.
func (e *Engine) buildProofLines(available, dismissed bool) []string {
	lines := []string{}

	if available {
		lines = append(lines, "A permitted preview was available this period.")
	} else {
		lines = append(lines, "No preview was available this period.")
	}

	if dismissed {
		lines = append(lines, "You chose to hold or dismiss it.")
	}

	lines = append(lines, "Previews are only shown when permitted by your policy.")
	lines = append(lines, "Your boundaries are being respected.")

	return lines
}

// ShouldShowCue determines if the preview cue should be shown.
// Returns true if:
// - There are permitted candidates
// - AND not dismissed for this period
// - AND not held for this period
func (e *Engine) ShouldShowCue(input *interruptpreview.PreviewInput) bool {
	if input == nil {
		return false
	}

	if input.IsDismissed || input.IsHeld {
		return false
	}

	return len(input.PermittedCandidates) > 0
}

// ComputePermittedCandidates filters candidates that are permitted.
// This is a passthrough for Phase 33 output — candidates are already filtered.
// This method validates and converts to preview candidates.
func (e *Engine) ComputePermittedCandidates(
	candidateHashes []string,
	circleTypes []string,
	horizons []string,
	magnitudes []string,
	reasons []string,
	allowances []string,
) []*interruptpreview.PreviewCandidate {
	if len(candidateHashes) == 0 {
		return nil
	}

	n := len(candidateHashes)
	candidates := make([]*interruptpreview.PreviewCandidate, 0, n)

	for i := 0; i < n; i++ {
		if i >= len(circleTypes) || i >= len(horizons) || i >= len(magnitudes) || i >= len(reasons) || i >= len(allowances) {
			break
		}

		c := &interruptpreview.PreviewCandidate{
			CandidateHash: candidateHashes[i],
			CircleType:    interruptpreview.CircleTypeBucket(circleTypes[i]),
			Horizon:       interruptpreview.HorizonBucket(horizons[i]),
			Magnitude:     interruptpreview.MagnitudeBucket(magnitudes[i]),
			ReasonBucket:  interruptpreview.ReasonBucket(reasons[i]),
			Allowance:     interruptpreview.AllowanceBucket(allowances[i]),
		}

		// Validate and skip invalid candidates
		if err := c.Validate(); err != nil {
			continue
		}

		candidates = append(candidates, c)
	}

	return candidates
}

// FilterCommerce removes any commerce candidates (should never happen, but safety check).
func (e *Engine) FilterCommerce(candidates []*interruptpreview.PreviewCandidate) []*interruptpreview.PreviewCandidate {
	result := make([]*interruptpreview.PreviewCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.CircleType != "commerce" {
			result = append(result, c)
		}
	}
	return result
}

// CountPermitted counts permitted candidates.
func CountPermitted(candidates []*interruptpreview.PreviewCandidate) int {
	return len(candidates)
}

// HasPermitted checks if there are any permitted candidates.
func HasPermitted(candidates []*interruptpreview.PreviewCandidate) bool {
	return len(candidates) > 0
}
