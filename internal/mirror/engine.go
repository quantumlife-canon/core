// Package mirror provides the engine for building mirror proof pages.
//
// Phase 18.7: Mirror Proof - Trust Through Evidence of Reading
//
// CRITICAL: Abstract only - no names, dates, vendors, senders, amounts.
// CRITICAL: No timestamps rendered - use horizon buckets only.
// CRITICAL: No goroutines. No time.Now(). stdlib-only.
// CRITICAL: NEVER read raw events - only abstractions from stores.
// CRITICAL: NEVER expose identifiers.
//
// Reference: docs/ADR/ADR-0039-phase18-7-mirror-proof.md
package mirror

import (
	"sort"
	"time"

	"quantumlife/pkg/domain/connection"
	"quantumlife/pkg/domain/mirror"
)

// Engine builds mirror proof pages deterministically.
// Same inputs + same clock = same output.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new mirror engine with injected clock.
func NewEngine(clock func() time.Time) *Engine {
	if clock == nil {
		clock = time.Now
	}
	return &Engine{clock: clock}
}

// notStoredStatements defines what we explicitly did NOT store per source.
// These are reassuring statements that build trust.
var notStoredStatements = map[connection.ConnectionKind][]string{
	connection.KindEmail: {
		"messages",
		"senders",
		"subjects",
	},
	connection.KindCalendar: {
		"event details",
		"attendees",
		"locations",
	},
	connection.KindFinance: {
		"account numbers",
		"specific amounts",
		"vendor details",
	},
}

// observedCategoriesByKind defines which categories can be observed per source.
var observedCategoriesByKind = map[connection.ConnectionKind][]mirror.ObservedCategory{
	connection.KindEmail: {
		mirror.ObservedTimeCommitments,
		mirror.ObservedReceipts,
	},
	connection.KindCalendar: {
		mirror.ObservedTimeCommitments,
	},
	connection.KindFinance: {
		mirror.ObservedReceipts,
		mirror.ObservedPatterns,
	},
}

// BuildMirrorPage generates the mirror proof page from input.
// Output is deterministic: same input + same clock = same output.
func (e *Engine) BuildMirrorPage(input mirror.MirrorInput) mirror.MirrorPage {
	now := e.clock()

	page := mirror.MirrorPage{
		Title:       "Seen, quietly.",
		Subtitle:    "A record of what we noticed — and what we didn't keep.",
		Sources:     e.buildSourceSummaries(input),
		Outcome:     e.buildOutcome(input),
		GeneratedAt: now,
	}

	// Add restraint statements
	page.RestraintStatement = "We chose not to interrupt you."
	page.RestraintWhy = "Quiet is a feature, not a gap."

	// Compute hash
	page.Hash = page.ComputeHash()

	return page
}

// buildSourceSummaries creates abstract summaries for each connected source.
func (e *Engine) buildSourceSummaries(input mirror.MirrorInput) []mirror.MirrorSourceSummary {
	var summaries []mirror.MirrorSourceSummary

	// Process sources in deterministic order (alphabetical by kind)
	kinds := make([]connection.ConnectionKind, 0, len(input.ConnectedSources))
	for kind := range input.ConnectedSources {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i] < kinds[j]
	})

	for _, kind := range kinds {
		state := input.ConnectedSources[kind]
		if !state.Connected {
			continue
		}

		summary := mirror.MirrorSourceSummary{
			Kind:             kind,
			ReadSuccessfully: state.ReadSuccess,
			NotStored:        notStoredStatements[kind],
			Observed:         e.buildObservedItems(kind, state),
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

// buildObservedItems creates abstract observed items for a source.
// CRITICAL: Uses magnitude buckets only - never raw counts.
func (e *Engine) buildObservedItems(kind connection.ConnectionKind, state mirror.SourceInputState) []mirror.ObservedItem {
	var items []mirror.ObservedItem

	// Get allowed categories for this kind
	allowedCategories := observedCategoriesByKind[kind]
	if allowedCategories == nil {
		return items
	}

	// Process categories in deterministic order
	sort.Slice(allowedCategories, func(i, j int) bool {
		return allowedCategories[i] < allowedCategories[j]
	})

	for _, cat := range allowedCategories {
		count, ok := state.ObservedCounts[cat]
		if !ok || count == 0 {
			continue
		}

		item := mirror.ObservedItem{
			Category:  cat,
			Magnitude: mirror.BucketCount(count),
			Horizon:   e.selectHorizon(cat),
		}
		items = append(items, item)
	}

	return items
}

// selectHorizon assigns a horizon bucket based on category.
// Deterministic: same category = same horizon.
func (e *Engine) selectHorizon(cat mirror.ObservedCategory) mirror.HorizonBucket {
	// Simple deterministic assignment based on category
	switch cat {
	case mirror.ObservedTimeCommitments:
		return mirror.HorizonOngoing
	case mirror.ObservedReceipts:
		return mirror.HorizonRecent
	case mirror.ObservedMessages:
		return mirror.HorizonRecent
	case mirror.ObservedPatterns:
		return mirror.HorizonOngoing
	default:
		return mirror.HorizonEarlier
	}
}

// buildOutcome creates the abstract outcome section.
func (e *Engine) buildOutcome(input mirror.MirrorInput) mirror.MirrorOutcome {
	heldMag := mirror.BucketCount(input.HeldCount)

	return mirror.MirrorOutcome{
		HeldQuietly:              input.HeldCount > 0,
		HeldMagnitude:            heldMag,
		NothingRequiresAttention: input.SurfacedCount == 0,
	}
}

// HasConnectedSources checks if there are any connected sources.
// Mirror should not be shown if no sources are connected.
func (e *Engine) HasConnectedSources(input mirror.MirrorInput) bool {
	for _, state := range input.ConnectedSources {
		if state.Connected {
			return true
		}
	}
	return false
}

// DefaultInput returns a default mirror input for demo/testing.
func DefaultInput() mirror.MirrorInput {
	return mirror.MirrorInput{
		ConnectedSources: map[connection.ConnectionKind]mirror.SourceInputState{
			connection.KindEmail: {
				Connected:   true,
				Mode:        connection.ModeMock,
				ReadSuccess: true,
				ObservedCounts: map[mirror.ObservedCategory]int{
					mirror.ObservedTimeCommitments: 2,
					mirror.ObservedReceipts:        3,
				},
			},
			connection.KindCalendar: {
				Connected:   true,
				Mode:        connection.ModeMock,
				ReadSuccess: true,
				ObservedCounts: map[mirror.ObservedCategory]int{
					mirror.ObservedTimeCommitments: 5,
				},
			},
		},
		HeldCount:     3,
		SurfacedCount: 0,
		CircleID:      "demo-circle",
	}
}

// EmptyInput returns an input with no connections.
func EmptyInput() mirror.MirrorInput {
	return mirror.MirrorInput{
		ConnectedSources: make(map[connection.ConnectionKind]mirror.SourceInputState),
		HeldCount:        0,
		SurfacedCount:    0,
		CircleID:         "demo-circle",
	}
}
