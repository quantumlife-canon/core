// Package journey provides the guided journey engine.
//
// Phase 26A: Guided Journey (Product/UX)
//
// CRITICAL INVARIANTS:
//   - Deterministic: same inputs => same outputs
//   - No goroutines
//   - No time.Now() - clock injection only
//   - No identifiable info in outputs
//
// Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md
package journey

import (
	"fmt"
	"time"
)

// Engine computes the journey state and pages.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new journey engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// NextStep computes the next step based on inputs.
// Deterministic precedence rules:
//  1. If dismissed for this period AND same status hash: StepDone
//  2. If !HasGmail: StepConnect
//  3. If HasGmail && !HasSyncReceipt: StepSync
//  4. If synced but !MirrorViewed: StepMirror
//  5. If ActionEligible && !ActionUsedThisPeriod: StepAction
//  6. Else: StepToday (then StepDone on next view)
func (e *Engine) NextStep(input *JourneyInputs) StepKind {
	if input == nil {
		return StepDone
	}

	// Check for dismissal with matching status hash
	currentHash := input.ComputeStatusHash()
	if input.DismissedStatusHash != "" && input.DismissedStatusHash == currentHash {
		return StepDone
	}

	// Precedence 1: Connect Gmail if not connected
	if !input.HasGmail {
		return StepConnect
	}

	// Precedence 2: Sync if Gmail connected but no sync receipt
	if !input.HasSyncReceipt {
		return StepSync
	}

	// Precedence 3: Mirror if synced but not viewed
	if !input.MirrorViewed {
		return StepMirror
	}

	// Precedence 4: Action if eligible and not used
	if input.ActionEligible && !input.ActionUsedThisPeriod {
		return StepAction
	}

	// Precedence 5: Today page (always safe fallback)
	// Note: We don't track "Today viewed" - can always revisit Today
	return StepToday
}

// BuildPage constructs the journey page for the current step.
func (e *Engine) BuildPage(input *JourneyInputs) *JourneyPage {
	step := e.NextStep(input)
	statusHash := input.ComputeStatusHash()

	// If done, return done page
	if step == StepDone {
		return e.buildDonePage(input, statusHash)
	}

	switch step {
	case StepConnect:
		return e.buildConnectPage(input, statusHash)
	case StepSync:
		return e.buildSyncPage(input, statusHash)
	case StepMirror:
		return e.buildMirrorPage(input, statusHash)
	case StepToday:
		return e.buildTodayPage(input, statusHash)
	case StepAction:
		return e.buildActionPage(input, statusHash)
	default:
		return e.buildDonePage(input, statusHash)
	}
}

// buildConnectPage builds the "Connect Gmail" step page.
func (e *Engine) buildConnectPage(input *JourneyInputs, statusHash string) *JourneyPage {
	return &JourneyPage{
		Title:    "Start, quietly.",
		Subtitle: "",
		Lines: []string{
			"If you want, you can connect one source.",
			"Read-only. Nothing is stored.",
		},
		PrimaryAction: JourneyAction{
			Label:  "Connect Gmail",
			Method: "GET",
			Path:   fmt.Sprintf("/connect/gmail?circle_id=%s", input.CircleID),
		},
		SecondaryAction: &JourneyAction{
			Label:  "Not now",
			Method: "POST",
			Path:   "/journey/dismiss",
			FormFields: map[string]string{
				"circle_id":   input.CircleID,
				"status_hash": statusHash,
			},
		},
		StepLabel:   stepLabel(StepConnect),
		CurrentStep: StepConnect,
		StatusHash:  statusHash,
		IsDone:      false,
	}
}

// buildSyncPage builds the "Sync" step page.
func (e *Engine) buildSyncPage(input *JourneyInputs, statusHash string) *JourneyPage {
	return &JourneyPage{
		Title:    "One small read.",
		Subtitle: "",
		Lines: []string{
			"We'll notice up to a few recent messages.",
			"No subjects. No senders. Held by default.",
		},
		PrimaryAction: JourneyAction{
			Label:  "Sync now",
			Method: "POST",
			Path:   "/run/gmail-sync",
			FormFields: map[string]string{
				"circle_id": input.CircleID,
			},
		},
		SecondaryAction: &JourneyAction{
			Label:  "Not now",
			Method: "POST",
			Path:   "/journey/dismiss",
			FormFields: map[string]string{
				"circle_id":   input.CircleID,
				"status_hash": statusHash,
			},
		},
		StepLabel:   stepLabel(StepSync),
		CurrentStep: StepSync,
		StatusHash:  statusHash,
		IsDone:      false,
	}
}

// buildMirrorPage builds the "Mirror" step page.
func (e *Engine) buildMirrorPage(input *JourneyInputs, statusHash string) *JourneyPage {
	return &JourneyPage{
		Title:    "Seen, quietly.",
		Subtitle: "",
		Lines: []string{
			"Here's what was noticed — and what was not stored.",
		},
		PrimaryAction: JourneyAction{
			Label:  "View inbox mirror",
			Method: "GET",
			Path:   fmt.Sprintf("/mirror/inbox?circle_id=%s", input.CircleID),
		},
		SecondaryAction: &JourneyAction{
			Label:  "Skip",
			Method: "POST",
			Path:   "/journey/dismiss",
			FormFields: map[string]string{
				"circle_id":   input.CircleID,
				"status_hash": statusHash,
			},
		},
		StepLabel:   stepLabel(StepMirror),
		CurrentStep: StepMirror,
		StatusHash:  statusHash,
		IsDone:      false,
	}
}

// buildTodayPage builds the "Today" step page.
func (e *Engine) buildTodayPage(input *JourneyInputs, statusHash string) *JourneyPage {
	return &JourneyPage{
		Title:    "Today, quietly.",
		Subtitle: "",
		Lines: []string{
			"Nothing needs you — unless it truly does.",
		},
		PrimaryAction: JourneyAction{
			Label:  "Go to Today",
			Method: "GET",
			Path:   "/today",
		},
		SecondaryAction: &JourneyAction{
			Label:  "Skip",
			Method: "POST",
			Path:   "/journey/dismiss",
			FormFields: map[string]string{
				"circle_id":   input.CircleID,
				"status_hash": statusHash,
			},
		},
		StepLabel:   stepLabel(StepToday),
		CurrentStep: StepToday,
		StatusHash:  statusHash,
		IsDone:      false,
	}
}

// buildActionPage builds the "One reversible action" step page.
func (e *Engine) buildActionPage(input *JourneyInputs, statusHash string) *JourneyPage {
	return &JourneyPage{
		Title:    "One action, reversible.",
		Subtitle: "",
		Lines: []string{
			"If you want, you can respond to one invitation.",
			"Undo is available.",
		},
		PrimaryAction: JourneyAction{
			Label:  "Try one action",
			Method: "GET",
			Path:   fmt.Sprintf("/action/undoable?circle_id=%s", input.CircleID),
		},
		SecondaryAction: &JourneyAction{
			Label:  "Not now",
			Method: "POST",
			Path:   "/journey/dismiss",
			FormFields: map[string]string{
				"circle_id":   input.CircleID,
				"status_hash": statusHash,
			},
		},
		StepLabel:   stepLabel(StepAction),
		CurrentStep: StepAction,
		StatusHash:  statusHash,
		IsDone:      false,
	}
}

// buildDonePage builds the "Done" completion page.
func (e *Engine) buildDonePage(input *JourneyInputs, statusHash string) *JourneyPage {
	return &JourneyPage{
		Title:    "Done.",
		Subtitle: "Back to quiet.",
		Lines: []string{
			"We'll keep holding what doesn't need you.",
		},
		PrimaryAction: JourneyAction{
			Label:  "Go to Today",
			Method: "GET",
			Path:   "/today",
		},
		SecondaryAction: nil, // No secondary action for done
		StepLabel:       "",
		CurrentStep:     StepDone,
		StatusHash:      statusHash,
		IsDone:          true,
	}
}

// stepLabel returns the progress label for a step.
// Using calm language, not numeric pressure.
func stepLabel(step StepKind) string {
	switch step {
	case StepConnect:
		return "First step"
	case StepSync:
		return "Next step"
	case StepMirror:
		return "Then"
	case StepToday:
		return "Almost there"
	case StepAction:
		return "One more thing"
	default:
		return ""
	}
}

// ShouldShowJourneyCue determines if the journey cue should show on Today page.
// Respects single whisper rule: returns false if another cue is already active.
func (e *Engine) ShouldShowJourneyCue(input *JourneyInputs, otherCueActive bool) bool {
	// If another cue is active, don't show journey cue (single whisper rule)
	if otherCueActive {
		return false
	}

	// Compute next step
	step := e.NextStep(input)

	// Don't show if journey is done
	if step == StepDone {
		return false
	}

	// Show if there's a pending step
	return true
}
