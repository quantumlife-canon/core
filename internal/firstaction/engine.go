// Package firstaction provides the First Reversible Action engine.
//
// Phase 24: First Reversible Real Action (Trust-Preserving)
//
// This engine determines eligibility, selects exactly one held item
// (deterministically), and builds a preview. It enforces the one-per-period
// rule and never mutates obligations.
//
// CRITICAL INVARIANTS:
//   - No goroutines
//   - No retries
//   - No background execution
//   - No mutation of obligations
//   - Preview only, never execution
//   - One action per period maximum
//   - No time.Now() - clock injection only.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0054-phase24-first-reversible-action.md
package firstaction

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"quantumlife/pkg/domain/firstaction"
)

// Engine computes first reversible actions.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new first action engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// HeldItemAbstract contains abstract held item data.
// CRITICAL: No identifiers, no raw content.
type HeldItemAbstract struct {
	// Hash is the hash of the held item.
	Hash string

	// Category is the abstract category.
	Category firstaction.AbstractCategory

	// Horizon is the abstract time horizon.
	Horizon firstaction.HorizonBucket

	// Magnitude is the abstract magnitude.
	Magnitude firstaction.MagnitudeBucket
}

// TrustInputs contains abstract trust-related inputs.
type TrustInputs struct {
	// HasQuietBaseline indicates quiet baseline is verified.
	HasQuietBaseline bool

	// HasMirrorViewed indicates mirror was viewed.
	HasMirrorViewed bool

	// HasTrustAccrual indicates trust exists.
	HasTrustAccrual bool

	// TrustScore is the abstract trust level (0-1).
	TrustScore float64
}

// ComputeEligibility builds an eligibility check from abstract inputs.
func (e *Engine) ComputeEligibility(
	circleID string,
	hasGmailConnection bool,
	trustInputs *TrustInputs,
	hasPriorActionThisPeriod bool,
	hasHeldItems bool,
) *firstaction.ActionEligibility {
	now := e.clock()
	period := firstaction.NewActionPeriod(now.Format("2006-01-02"))

	eligibility := &firstaction.ActionEligibility{
		CircleID:                 circleID,
		HasGmailConnection:       hasGmailConnection,
		HasPriorActionThisPeriod: hasPriorActionThisPeriod,
		HasHeldItems:             hasHeldItems,
		Period:                   period,
	}

	if trustInputs != nil {
		eligibility.HasQuietBaseline = trustInputs.HasQuietBaseline
		eligibility.HasMirrorViewed = trustInputs.HasMirrorViewed
		eligibility.HasTrustAccrual = trustInputs.HasTrustAccrual && trustInputs.TrustScore > 0
	}

	return eligibility
}

// SelectHeldItem deterministically selects one held item from the list.
// Selection is based on hash ordering for determinism.
func (e *Engine) SelectHeldItem(items []HeldItemAbstract) *HeldItemAbstract {
	if len(items) == 0 {
		return nil
	}

	// Deterministic selection: pick the item with the lowest hash
	selected := &items[0]
	for i := 1; i < len(items); i++ {
		if items[i].Hash < selected.Hash {
			selected = &items[i]
		}
	}

	return selected
}

// BuildPreview creates an action preview from a held item.
func (e *Engine) BuildPreview(
	circleID string,
	item *HeldItemAbstract,
) *firstaction.ActionPreview {
	if item == nil {
		return nil
	}

	now := e.clock()
	period := firstaction.NewActionPeriod(now.Format("2006-01-02"))

	return &firstaction.ActionPreview{
		CircleID:    circleID,
		Period:      period,
		Category:    item.Category,
		Horizon:     item.Horizon,
		Magnitude:   item.Magnitude,
		Explanation: item.Horizon.DisplayText(),
		SourceHash:  item.Hash,
	}
}

// BuildActionPage creates the action invitation page.
func (e *Engine) BuildActionPage(eligibility *firstaction.ActionEligibility, category firstaction.AbstractCategory) *firstaction.ActionPage {
	if eligibility == nil || !eligibility.IsEligible() {
		return firstaction.NewEmptyActionPage()
	}
	return firstaction.NewActionPage(category)
}

// BuildPreviewPage creates the preview result page.
func (e *Engine) BuildPreviewPage(preview *firstaction.ActionPreview) *firstaction.PreviewPage {
	return firstaction.NewPreviewPage(preview)
}

// WhisperCue represents an optional whisper link for /today.
type WhisperCue struct {
	// Show indicates if the whisper should be shown.
	Show bool

	// Text is the whisper text.
	Text string

	// Link is the destination URL.
	Link string
}

// BuildWhisperCue creates the optional whisper cue for /today.
// CRITICAL: Lowest priority, can be ignored with zero cost.
func (e *Engine) BuildWhisperCue(eligibility *firstaction.ActionEligibility) *WhisperCue {
	if eligibility == nil || !eligibility.IsEligible() {
		return &WhisperCue{Show: false}
	}

	return &WhisperCue{
		Show: true,
		Text: "If you'd like, we can look at one thing together.",
		Link: "/action/once",
	}
}

// CurrentPeriod returns the current action period.
func (e *Engine) CurrentPeriod() firstaction.ActionPeriod {
	now := e.clock()
	return firstaction.NewActionPeriod(now.Format("2006-01-02"))
}

// ComputeItemHash computes a deterministic hash for an abstract item.
// Used for consistent selection across calls.
func ComputeItemHash(category, horizon, magnitude string) string {
	canonical := "HELD_ITEM|v1|" + category + "|" + horizon + "|" + magnitude
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}
