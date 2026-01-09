// Package marketsignal provides the engine for Phase 48: Market Signal Binding.
//
// This engine binds unmet necessities to available marketplace packs WITHOUT:
// - Recommendations
// - Nudges
// - Ranking
// - Persuasion
// - Execution
//
// This is signal exposure only, not a funnel.
//
// CRITICAL: No time.Now() - clock must be injected.
// CRITICAL: No goroutines.
// CRITICAL: No ranking or scoring logic.
// CRITICAL: No recommendation language.
//
// Reference: docs/ADR/ADR-0086-phase48-market-signal-binding.md
package marketsignal

import (
	"sort"
	"time"

	domain "quantumlife/pkg/domain/marketsignal"
)

// NecessityDeclaration represents a necessity declaration from Phase 45.
// This is a simplified view - we only need the circle hash and necessity level.
type NecessityDeclaration struct {
	CircleHash    string
	NecessityKind domain.NecessityKind
}

// CoveragePlanView represents a coverage plan from Phase 47.
// This is a simplified view - we only need the capabilities list.
type CoveragePlanView struct {
	CircleHash   string
	Capabilities []string // Capability strings
}

// HasCapability checks if a capability is present.
func (c CoveragePlanView) HasCapability(cap string) bool {
	for _, existing := range c.Capabilities {
		if existing == cap {
			return true
		}
	}
	return false
}

// PackCapability represents what a pack can cover.
type PackCapability struct {
	PackSlugHash string
	Capabilities []string // What this pack provides
}

// AvailablePack represents an available pack from Phase 46.
type AvailablePack struct {
	PackSlugHash string
	PackLabel    string // Abstract label for display
	Capabilities []PackCapability
}

// MarketSignalInput represents the inputs for signal generation.
type MarketSignalInput struct {
	Necessities    []NecessityDeclaration
	CoveragePlan   CoveragePlanView
	AvailablePacks []AvailablePack
	PeriodKey      string
}

// MarketSignalResult represents the output of signal generation.
type MarketSignalResult struct {
	Signals []domain.MarketSignal
}

// Engine generates market signals from inputs.
// CRITICAL: This engine is pure and deterministic.
// CRITICAL: No recommendations, no ranking, no scoring.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new market signal engine with injected clock.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{clock: clock}
}

// GenerateSignals generates market signals for unmet necessities.
//
// For each NecessityDeclaration:
// 1. If coverage plan indicates a gap (missing capabilities)
// 2. If ≥1 available pack advertises a matching capability
// 3. Emit one MarketSignal per (necessity, pack)
//
// Constraints:
// - Max 3 signals per circle per period
// - Deterministic ordering (hash sort)
// - No ranking or prioritization
// - No natural language generation
// - No inference or scoring
func (e *Engine) GenerateSignals(input MarketSignalInput) MarketSignalResult {
	var signals []domain.MarketSignal

	// Group necessities by circle
	circleNecessities := make(map[string][]NecessityDeclaration)
	for _, decl := range input.Necessities {
		circleNecessities[decl.CircleHash] = append(circleNecessities[decl.CircleHash], decl)
	}

	// Process each circle
	for circleHash, declarations := range circleNecessities {
		circleSignals := e.generateCircleSignals(circleHash, declarations, input.CoveragePlan, input.AvailablePacks, input.PeriodKey)
		signals = append(signals, circleSignals...)
	}

	// Normalize: sort and deduplicate
	signals = domain.NormalizeSignals(signals)

	return MarketSignalResult{Signals: signals}
}

// generateCircleSignals generates signals for a single circle.
func (e *Engine) generateCircleSignals(
	circleHash string,
	declarations []NecessityDeclaration,
	coverage CoveragePlanView,
	packs []AvailablePack,
	periodKey string,
) []domain.MarketSignal {
	var signals []domain.MarketSignal

	// Find highest necessity level for this circle
	highestNecessity := domain.NecessityKindUnknown
	for _, decl := range declarations {
		if necessityPriority(decl.NecessityKind) > necessityPriority(highestNecessity) {
			highestNecessity = decl.NecessityKind
		}
	}

	// Only generate signals for medium or high necessity
	if highestNecessity != domain.NecessityKindMedium && highestNecessity != domain.NecessityKindHigh {
		return signals
	}

	// Find coverage gaps
	gaps := e.findCoverageGaps(coverage, packs)
	if len(gaps) == 0 {
		return signals
	}

	// Generate signals for gaps (max 3 per circle)
	count := 0
	for _, gap := range gaps {
		if count >= domain.MaxSignalsPerCirclePeriod {
			break
		}

		signal := domain.MarketSignal{
			CircleHash:    circleHash,
			NecessityKind: highestNecessity,
			CoverageGap:   gap.GapKind,
			PackIDHash:    gap.PackSlugHash,
			Kind:          domain.MarketSignalCoverageGap,
			Effect:        domain.EffectNoPower,
			Visibility:    domain.VisibilityProofOnly,
			PeriodKey:     periodKey,
		}
		signal.SignalID = signal.ComputeSignalID()

		signals = append(signals, signal)
		count++
	}

	return signals
}

// coverageGap represents a gap between necessity and coverage.
type coverageGap struct {
	GapKind      domain.CoverageGapKind
	PackSlugHash string
}

// findCoverageGaps finds gaps that available packs could fill.
// CRITICAL: No ranking - gaps are sorted by hash for determinism.
func (e *Engine) findCoverageGaps(coverage CoveragePlanView, packs []AvailablePack) []coverageGap {
	var gaps []coverageGap

	// For each pack, check if it provides capabilities we don't have
	for _, pack := range packs {
		for _, packCap := range pack.Capabilities {
			for _, cap := range packCap.Capabilities {
				if !coverage.HasCapability(cap) {
					// This pack could fill a gap
					gapKind := domain.GapNoObserver
					if len(coverage.Capabilities) > 0 {
						gapKind = domain.GapPartialCover
					}
					gaps = append(gaps, coverageGap{
						GapKind:      gapKind,
						PackSlugHash: pack.PackSlugHash,
					})
					break // Only one gap per pack
				}
			}
		}
	}

	// Sort by pack hash for deterministic output (NO RANKING)
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].PackSlugHash < gaps[j].PackSlugHash
	})

	return gaps
}

// necessityPriority returns numeric priority for necessity (for finding highest).
// CRITICAL: This is NOT ranking - it's for finding the highest declared necessity.
func necessityPriority(n domain.NecessityKind) int {
	switch n {
	case domain.NecessityKindHigh:
		return 3
	case domain.NecessityKindMedium:
		return 2
	case domain.NecessityKindLow:
		return 1
	default:
		return 0
	}
}

// BuildProofPage builds the UI model for market proof.
// CRITICAL: No recommendation language, no pricing, no urgency.
func (e *Engine) BuildProofPage(signals []domain.MarketSignal) domain.MarketProofPage {
	var displaySignals []domain.MarketSignalDisplay

	for _, sig := range signals {
		displaySignals = append(displaySignals, domain.MarketSignalDisplay{
			SignalID:      sig.SignalID,
			NecessityKind: necessityDisplayLabel(sig.NecessityKind),
			GapKind:       gapDisplayLabel(sig.CoverageGap),
			PackLabel:     "Available pack", // Abstract - never specific
			Effect:        sig.Effect,
			Visibility:    sig.Visibility,
		})
	}

	// Neutral copy - no recommendations!
	var lines []string
	if len(signals) > 0 {
		lines = []string{
			"Some needs you marked as important are not yet covered.",
		}
	} else {
		lines = []string{
			"All declared necessities have coverage.",
		}
	}

	return domain.MarketProofPage{
		Title:      "Market Signals",
		Lines:      lines,
		Signals:    displaySignals,
		StatusHash: domain.ComputeProofStatusHash(signals),
	}
}

// BuildCue builds the whisper cue for market signals.
// CRITICAL: No recommendation language, no urgency.
func (e *Engine) BuildCue(signals []domain.MarketSignal, dismissed bool) domain.MarketProofCue {
	available := len(signals) > 0 && !dismissed

	var text string
	if available {
		text = "Some coverage options exist."
	}

	return domain.MarketProofCue{
		Available:  available,
		Text:       text,
		Path:       "/proof/market",
		StatusHash: domain.ComputeCueStatusHash(len(signals), available),
	}
}

// ShouldShowCue determines if the cue should be shown.
func (e *Engine) ShouldShowCue(signals []domain.MarketSignal, dismissed bool) bool {
	return len(signals) > 0 && !dismissed
}

// necessityDisplayLabel returns a display-safe label for necessity.
func necessityDisplayLabel(n domain.NecessityKind) string {
	switch n {
	case domain.NecessityKindHigh:
		return "High importance"
	case domain.NecessityKindMedium:
		return "Medium importance"
	case domain.NecessityKindLow:
		return "Low importance"
	default:
		return "Unknown importance"
	}
}

// gapDisplayLabel returns a display-safe label for gap kind.
func gapDisplayLabel(g domain.CoverageGapKind) string {
	switch g {
	case domain.GapNoObserver:
		return "No observer coverage"
	case domain.GapPartialCover:
		return "Partial coverage"
	default:
		return "Coverage gap"
	}
}
