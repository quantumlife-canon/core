// Package shadowview provides the shadow receipt viewer for Phase 21.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
//
// CRITICAL INVARIANTS:
//   - Deterministic projection from existing receipts
//   - No goroutines. No time.Now().
//   - Stdlib only.
//   - Shows ONLY abstract buckets and hashes
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
package shadowview

import (
	"sort"
	"time"

	"quantumlife/pkg/domain/shadowllm"
)

// Engine builds shadow receipt page views.
//
// CRITICAL: Engine does NOT store state.
// CRITICAL: Engine does NOT spawn goroutines.
// CRITICAL: Engine uses clock injection for determinism.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new shadow view engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// BuildPageInput contains the inputs needed to build the receipt page.
type BuildPageInput struct {
	// Receipt is the shadow receipt to display (may be nil).
	Receipt *shadowllm.ShadowReceipt

	// HasGmailConnection indicates if Gmail is connected.
	HasGmailConnection bool

	// CalibrationAgreement is the calibration agreement bucket (if Phase 19.4+).
	// Empty string if no calibration.
	CalibrationAgreement string

	// CalibrationVote is the usefulness vote (if recorded).
	CalibrationVote string
}

// BuildPage creates the shadow receipt page view.
//
// CRITICAL: Deterministic projection - same input => same output.
// CRITICAL: Shows ONLY abstract buckets and hashes.
func (e *Engine) BuildPage(input BuildPageInput) ShadowReceiptPage {
	page := ShadowReceiptPage{
		HasReceipt: input.Receipt != nil,
	}

	// Source section
	page.Source = e.buildSourceSection(input.HasGmailConnection)

	if input.Receipt == nil {
		// No receipt - return minimal page
		page.Observation = ObservationSection{
			Magnitude: MagnitudeDisplayNothing,
			Statement: "No observations yet.",
		}
		page.Confidence = ConfidenceSection{
			Bucket:    "low",
			Statement: "Observation only.",
		}
		page.Restraint = e.buildRestraintSection()
		page.Calibration = e.buildCalibrationSection("", "")
		page.TrustAnchor = TrustAnchorSection{
			PeriodLabel: "today",
			ReceiptHash: "",
			Statement:   "No receipt recorded yet.",
		}
		return page
	}

	// Build from receipt
	page.Observation = e.buildObservationSection(input.Receipt)
	page.Confidence = e.buildConfidenceSection(input.Receipt)
	page.Restraint = e.buildRestraintSection()
	page.Calibration = e.buildCalibrationSection(input.CalibrationAgreement, input.CalibrationVote)
	page.TrustAnchor = e.buildTrustAnchorSection(input.Receipt)
	page.ReceiptHash = input.Receipt.Hash()

	return page
}

// buildSourceSection creates the source section.
func (e *Engine) buildSourceSection(hasGmail bool) SourceSection {
	if hasGmail {
		return SourceSection{
			Statement:   "Connected: email (read-only)",
			IsConnected: true,
		}
	}
	return SourceSection{
		Statement:   "No sources connected",
		IsConnected: false,
	}
}

// buildObservationSection creates the observation section from a receipt.
func (e *Engine) buildObservationSection(receipt *shadowllm.ShadowReceipt) ObservationSection {
	// Extract unique categories from suggestions
	categorySet := make(map[shadowllm.AbstractCategory]bool)
	var overallMagnitude shadowllm.MagnitudeBucket = shadowllm.MagnitudeNothing
	var overallHorizon shadowllm.Horizon = shadowllm.HorizonSomeday

	for _, sug := range receipt.Suggestions {
		categorySet[sug.Category] = true

		// Track highest magnitude
		if sug.Magnitude == shadowllm.MagnitudeSeveral {
			overallMagnitude = shadowllm.MagnitudeSeveral
		} else if sug.Magnitude == shadowllm.MagnitudeAFew && overallMagnitude != shadowllm.MagnitudeSeveral {
			overallMagnitude = shadowllm.MagnitudeAFew
		}

		// Track most urgent horizon
		if sug.Horizon == shadowllm.HorizonNow {
			overallHorizon = shadowllm.HorizonNow
		} else if sug.Horizon == shadowllm.HorizonSoon && overallHorizon != shadowllm.HorizonNow {
			overallHorizon = shadowllm.HorizonSoon
		} else if sug.Horizon == shadowllm.HorizonLater && overallHorizon == shadowllm.HorizonSomeday {
			overallHorizon = shadowllm.HorizonLater
		}
	}

	// Sort categories for determinism
	var categories []string
	for cat := range categorySet {
		categories = append(categories, CategoryDisplayText(cat))
	}
	sort.Strings(categories)

	// Build statement
	var statement string
	if len(receipt.Suggestions) == 0 {
		statement = "No patterns observed."
	} else if overallMagnitude == shadowllm.MagnitudeAFew {
		statement = "A few patterns observed."
	} else if overallMagnitude == shadowllm.MagnitudeSeveral {
		statement = "Several patterns observed."
	} else {
		statement = "Minimal activity observed."
	}

	return ObservationSection{
		Magnitude:  MagnitudeDisplayText(overallMagnitude),
		Categories: categories,
		Horizon:    HorizonDisplayText(overallHorizon),
		Statement:  statement,
	}
}

// buildConfidenceSection creates the confidence section.
func (e *Engine) buildConfidenceSection(receipt *shadowllm.ShadowReceipt) ConfidenceSection {
	// Find highest confidence from suggestions
	var highestConf shadowllm.ConfidenceBucket = shadowllm.ConfidenceLow
	for _, sug := range receipt.Suggestions {
		if sug.Confidence == shadowllm.ConfidenceHigh {
			highestConf = shadowllm.ConfidenceHigh
			break
		} else if sug.Confidence == shadowllm.ConfidenceMed && highestConf != shadowllm.ConfidenceHigh {
			highestConf = shadowllm.ConfidenceMed
		}
	}

	return ConfidenceSection{
		Bucket:    ConfidenceDisplayText(highestConf),
		Statement: "Observation only.",
	}
}

// buildRestraintSection creates the restraint section.
// CRITICAL: These are ALWAYS true - shadow mode never executes.
func (e *Engine) buildRestraintSection() RestraintSection {
	return RestraintSection{
		NoActionsTaken:      true,
		NoDraftsCreated:     true,
		NoNotificationsSent: true,
		NoRulesPromoted:     true,
		Statements: []string{
			"No actions taken.",
			"No drafts created.",
			"No notifications sent.",
			"No rules promoted.",
		},
	}
}

// buildCalibrationSection creates the calibration section.
func (e *Engine) buildCalibrationSection(agreement, vote string) CalibrationSection {
	if agreement == "" && vote == "" {
		return CalibrationSection{
			HasCalibration: false,
			Statement:      "No calibration recorded.",
		}
	}

	section := CalibrationSection{
		HasCalibration:  true,
		AgreementBucket: agreement,
		VoteUsefulness:  vote,
	}

	if agreement != "" {
		section.Statement = "Calibration: " + agreement
	} else {
		section.Statement = "Calibration in progress."
	}

	return section
}

// buildTrustAnchorSection creates the trust anchor section.
func (e *Engine) buildTrustAnchorSection(receipt *shadowllm.ShadowReceipt) TrustAnchorSection {
	return TrustAnchorSection{
		PeriodLabel: receipt.WindowBucket,
		ReceiptHash: receipt.Hash(),
		Statement:   "Append-only. This proof cannot be edited.",
	}
}

// EmptyPage returns a page for when no receipts exist.
func (e *Engine) EmptyPage(hasGmailConnection bool) ShadowReceiptPage {
	return e.BuildPage(BuildPageInput{
		Receipt:            nil,
		HasGmailConnection: hasGmailConnection,
	})
}

// ReceiptCue represents the whisper cue for the shadow receipt proof page.
//
// CRITICAL: Shows ONLY abstract statement and link text.
// CRITICAL: Follows single whisper rule - at most ONE cue on /today.
type ReceiptCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the subtle text hinting at proof availability.
	CueText string

	// LinkText is the text for the link to /shadow/receipt.
	LinkText string

	// ReceiptHash for dismissal tracking.
	ReceiptHash string
}

// BuildCueInput contains the inputs needed to build a receipt cue.
type BuildCueInput struct {
	// Receipt is the shadow receipt (may be nil).
	Receipt *shadowllm.ShadowReceipt

	// IsDismissed indicates if the cue was dismissed for current period.
	IsDismissed bool

	// OtherCueActive indicates if another whisper cue is already active.
	// CRITICAL: Single whisper rule - only ONE cue per page.
	OtherCueActive bool
}

// BuildCue determines if and what receipt cue to show on /today.
//
// Cue shows when:
// - Receipt exists for current period
// - Not dismissed for current period
// - No other whisper cue is active (single whisper rule)
//
// CRITICAL: Deterministic - same input => same output.
func (e *Engine) BuildCue(input BuildCueInput) ReceiptCue {
	// No cue if no receipt
	if input.Receipt == nil {
		return ReceiptCue{Available: false}
	}

	// No cue if dismissed
	if input.IsDismissed {
		return ReceiptCue{Available: false}
	}

	// No cue if another whisper is active (single whisper rule)
	if input.OtherCueActive {
		return ReceiptCue{Available: false}
	}

	// Build the cue
	return ReceiptCue{
		Available:   true,
		CueText:     "Proof of observation recorded.",
		LinkText:    "View shadow receipt",
		ReceiptHash: input.Receipt.Hash(),
	}
}
