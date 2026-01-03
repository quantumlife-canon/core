// Package quietmirror provides the Quiet Inbox Mirror engine.
//
// Phase 22: Quiet Inbox Mirror (First Real Value Moment)
//
// This engine consumes Gmail SyncReceipts and produces abstract reflections.
// It proves the system is working WITHOUT showing content, urgency, or actions.
//
// CRITICAL INVARIANTS:
//   - Consumes ONLY abstract inputs (magnitude buckets, category presence)
//   - Produces ONLY abstract outputs (magnitude buckets, calm statements)
//   - No email subjects, senders, timestamps, or counts
//   - One calm, ignorable statement
//   - Deterministic output
//   - No LLM usage
//   - No goroutines. No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md
package quietmirror

import (
	"sort"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/quietmirror"
)

// Engine computes Quiet Inbox Mirror summaries.
//
// CRITICAL: This engine NEVER receives raw Gmail data.
// It only receives abstract inputs from SyncReceipts.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new Quiet Inbox Mirror engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// ComputeInput creates a mirror input from abstract sources.
//
// CRITICAL: This is the boundary where raw data becomes abstract.
// The receipt already contains only magnitude buckets.
func (e *Engine) ComputeInput(
	circleID identity.EntityID,
	hasConnection bool,
	receipt *persist.SyncReceipt,
	categoryPresence map[quietmirror.MirrorCategory]bool,
) *quietmirror.QuietMirrorInput {
	now := e.clock()
	period := now.Format("2006-01-02")

	input := &quietmirror.QuietMirrorInput{
		CircleID:         string(circleID),
		Period:           period,
		HasConnection:    hasConnection,
		HasSyncReceipt:   receipt != nil && receipt.Success,
		CategoryPresence: categoryPresence,
	}

	if receipt != nil {
		input.SyncReceiptHash = receipt.Hash
		input.ObligationMagnitude = mapMagnitude(receipt.MagnitudeBucket)
	} else {
		input.ObligationMagnitude = quietmirror.MagnitudeNothing
	}

	return input
}

// Compute produces a QuietMirrorSummary from abstract inputs.
//
// CRITICAL: The output contains NO identifiable information.
// Only magnitude buckets, category presence, and one calm statement.
func (e *Engine) Compute(input *quietmirror.QuietMirrorInput) *quietmirror.QuietMirrorSummary {
	summary := &quietmirror.QuietMirrorSummary{
		CircleID:   input.CircleID,
		Period:     input.Period,
		SourceHash: input.SourceHash(),
	}

	// No connection or no sync? No mirror.
	if !input.HasConnection || !input.HasSyncReceipt {
		summary.Magnitude = quietmirror.MagnitudeNothing
		summary.Statement = e.selectStatement(quietmirror.MagnitudeNothing)
		summary.HasMirror = false
		return summary
	}

	// Determine magnitude
	summary.Magnitude = input.ObligationMagnitude

	// Extract categories (max 3, sorted for determinism)
	summary.Categories = e.extractCategories(input.CategoryPresence)

	// Select calm statement deterministically based on magnitude
	summary.Statement = e.selectStatement(summary.Magnitude)

	// Has mirror if there's anything to reflect
	summary.HasMirror = summary.Magnitude != quietmirror.MagnitudeNothing

	return summary
}

// extractCategories extracts and limits categories to max 3.
// Categories are sorted alphabetically for determinism.
func (e *Engine) extractCategories(presence map[quietmirror.MirrorCategory]bool) []quietmirror.MirrorCategory {
	var cats []quietmirror.MirrorCategory
	for cat, present := range presence {
		if present {
			cats = append(cats, cat)
		}
	}

	// Sort alphabetically for determinism
	sort.Slice(cats, func(i, j int) bool {
		return string(cats[i]) < string(cats[j])
	})

	// Cap at 3 categories
	if len(cats) > 3 {
		cats = cats[:3]
	}

	return cats
}

// selectStatement chooses a calm statement based on magnitude.
// This is deterministic - same magnitude always produces same statement.
func (e *Engine) selectStatement(magnitude quietmirror.MirrorMagnitude) quietmirror.MirrorStatement {
	switch magnitude {
	case quietmirror.MagnitudeNothing:
		return quietmirror.MirrorStatement{
			Text:          "Nothing here needs you today.",
			StatementKind: quietmirror.StatementKindNothing,
		}
	case quietmirror.MagnitudeAFew:
		return quietmirror.MirrorStatement{
			Text:          "A few patterns are being kept an eye on.",
			StatementKind: quietmirror.StatementKindPatterns,
		}
	case quietmirror.MagnitudeSeveral:
		return quietmirror.MirrorStatement{
			Text:          "Some things are being watched quietly.",
			StatementKind: quietmirror.StatementKindWatching,
		}
	default:
		return quietmirror.MirrorStatement{
			Text:          "Nothing here needs you today.",
			StatementKind: quietmirror.StatementKindNothing,
		}
	}
}

// BuildPage creates the UI page data from a summary.
func (e *Engine) BuildPage(summary *quietmirror.QuietMirrorSummary) *quietmirror.QuietMirrorPage {
	if summary == nil {
		return quietmirror.NewEmptyPage()
	}
	return quietmirror.NewMirrorPage(summary)
}

// mapMagnitude converts SyncReceipt magnitude to QuietMirror magnitude.
func mapMagnitude(m persist.MagnitudeBucket) quietmirror.MirrorMagnitude {
	switch m {
	case persist.MagnitudeNone:
		return quietmirror.MagnitudeNothing
	case persist.MagnitudeHandful:
		return quietmirror.MagnitudeAFew
	case persist.MagnitudeSeveral, persist.MagnitudeMany:
		return quietmirror.MagnitudeSeveral
	default:
		return quietmirror.MagnitudeNothing
	}
}

// WhisperCue represents an optional whisper link for /today.
//
// CRITICAL: This is a single optional cue, never pushed.
// The cue is calm and dismissable.
type WhisperCue struct {
	// Show indicates if the whisper should be shown.
	Show bool

	// Text is the whisper text.
	Text string

	// Link is the destination URL.
	Link string
}

// BuildWhisperCue creates the optional whisper cue for /today.
//
// CRITICAL: This returns a cue only if there's something to show
// AND the caller hasn't dismissed it. Never auto-surface.
func (e *Engine) BuildWhisperCue(summary *quietmirror.QuietMirrorSummary, dismissed bool) *WhisperCue {
	// No summary or no mirror? No cue.
	if summary == nil || !summary.HasMirror {
		return &WhisperCue{Show: false}
	}

	// Already dismissed? Respect that.
	if dismissed {
		return &WhisperCue{Show: false}
	}

	return &WhisperCue{
		Show: true,
		Text: "If you were curious, we noticed something — quietly.",
		Link: "/mirror/inbox",
	}
}
