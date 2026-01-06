// Package reality provides the engine for Phase 26C: Connected Reality Check.
//
// This is NOT analytics. This is a trust proof page.
// The engine builds a RealityPage from existing stores without any new data collection.
//
// CRITICAL: No secrets ever rendered.
// CRITICAL: No time.Now() - clock injection only.
// CRITICAL: No goroutines.
// CRITICAL: stdlib only.
// CRITICAL: Deterministic: same inputs + clock => identical output + hash.
//
// Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
package reality

import (
	"time"

	"quantumlife/pkg/domain/reality"
)

// Clock provides the current time for the engine.
type Clock interface {
	Now() time.Time
}

// Engine builds RealityPage from inputs.
type Engine struct {
	clock Clock
}

// NewEngine creates a new reality engine with the given clock.
func NewEngine(clock Clock) *Engine {
	return &Engine{clock: clock}
}

// BuildPage builds a RealityPage from the given inputs.
// This is a pure function: same inputs => same output.
func (e *Engine) BuildPage(inputs *reality.RealityInputs) *reality.RealityPage {
	page := &reality.RealityPage{
		Title:    "Connected, quietly.",
		Subtitle: "Proof that real data stays quiet.",
		BackPath: "/today",
	}

	// Build lines in deterministic order
	page.Lines = e.buildLines(inputs)

	// Select calm line based on state
	page.CalmLine = e.selectCalmLine(inputs)

	// Compute status hash
	page.StatusHash = page.ComputeStatusHash()

	return page
}

// buildLines builds the reality lines in deterministic order.
func (e *Engine) buildLines(inputs *reality.RealityInputs) []reality.RealityLine {
	lines := make([]reality.RealityLine, 0, 12)

	// Gmail connected
	lines = append(lines, reality.RealityLine{
		Label: "Gmail connected",
		Value: boolToYesNo(inputs.GmailConnected),
		Kind:  reality.LineKindBool,
	})

	// Last sync
	lines = append(lines, reality.RealityLine{
		Label: "Last sync",
		Value: reality.SyncBucketToDisplay(inputs.SyncBucket),
		Kind:  reality.LineKindEnum,
	})

	// Messages noticed (only if synced)
	if inputs.SyncBucket != reality.SyncBucketNever {
		lines = append(lines, reality.RealityLine{
			Label: "Messages noticed",
			Value: reality.MagnitudeToDisplay(inputs.SyncMagnitude),
			Kind:  reality.LineKindBucket,
		})
	}

	// Obligations held
	lines = append(lines, reality.RealityLine{
		Label: "Obligations held",
		Value: boolToYesNo(inputs.ObligationsHeld),
		Kind:  reality.LineKindBool,
	})

	// Auto-surface (should always be no)
	lines = append(lines, reality.RealityLine{
		Label: "Auto-surface",
		Value: boolToYesNo(inputs.AutoSurface),
		Kind:  reality.LineKindBool,
	})

	// Shadow mode section
	lines = append(lines, reality.RealityLine{
		Label: "Shadow mode",
		Value: reality.ProviderKindToDisplay(inputs.ShadowProviderKind),
		Kind:  reality.LineKindEnum,
	})

	// Real providers allowed
	lines = append(lines, reality.RealityLine{
		Label: "Real providers allowed",
		Value: boolToYesNo(inputs.ShadowRealAllowed),
		Kind:  reality.LineKindBool,
	})

	// Shadow receipts (only if shadow mode is on)
	if inputs.ShadowProviderKind != reality.ProviderOff {
		lines = append(lines, reality.RealityLine{
			Label: "Shadow receipts",
			Value: reality.MagnitudeToDisplay(inputs.ShadowMagnitude),
			Kind:  reality.LineKindBucket,
		})
	}

	// Chat configured
	lines = append(lines, reality.RealityLine{
		Label: "Chat configured",
		Value: boolToYesNo(inputs.ChatConfigured),
		Kind:  reality.LineKindBool,
	})

	// Embeddings configured
	lines = append(lines, reality.RealityLine{
		Label: "Embeddings configured",
		Value: boolToYesNo(inputs.EmbedConfigured),
		Kind:  reality.LineKindBool,
	})

	// Endpoint configured
	lines = append(lines, reality.RealityLine{
		Label: "Endpoint configured",
		Value: boolToYesNo(inputs.EndpointConfigured),
		Kind:  reality.LineKindBool,
	})

	// Region (only if explicitly configured)
	if inputs.Region != "" {
		lines = append(lines, reality.RealityLine{
			Label: "Region",
			Value: inputs.Region,
			Kind:  reality.LineKindNote,
		})
	}

	return lines
}

// selectCalmLine selects the appropriate calm line based on inputs.
// Deterministic: same inputs => same calm line.
func (e *Engine) selectCalmLine(inputs *reality.RealityInputs) string {
	if !inputs.GmailConnected {
		return "Nothing is connected yet. Quiet is still the baseline."
	}

	if inputs.SyncBucket == reality.SyncBucketNever {
		return "Connected. Waiting for your explicit sync."
	}

	return "Quiet baseline verified."
}

// ComputeCue builds the whisper cue for the /today page.
// Returns a cue with Available=false if conditions are not met.
func (e *Engine) ComputeCue(inputs *reality.RealityInputs, acked bool) *reality.RealityCue {
	cue := &reality.RealityCue{
		Available:  false,
		CueText:    reality.DefaultCueText,
		LinkText:   reality.DefaultLinkText,
		StatusHash: inputs.Hash(),
	}

	// Cue only available if:
	// 1. Gmail is connected
	// 2. At least one sync has happened
	// 3. Not already acknowledged for this status hash
	if !inputs.GmailConnected {
		return cue
	}

	if inputs.SyncBucket == reality.SyncBucketNever {
		return cue
	}

	if acked {
		return cue
	}

	cue.Available = true
	return cue
}

// ShouldShowRealityCue determines if the reality cue should be shown.
// Respects the single whisper rule - returns false if higher priority cues exist.
func (e *Engine) ShouldShowRealityCue(
	inputs *reality.RealityInputs,
	acked bool,
	surfaceCueActive bool,
	proofCueActive bool,
	journeyCueActive bool,
	firstMinutesCueActive bool,
) bool {
	// Reality cue is lowest priority
	// Priority order: journey > surface > proof > first-minutes > reality
	if journeyCueActive || surfaceCueActive || proofCueActive || firstMinutesCueActive {
		return false
	}

	cue := e.ComputeCue(inputs, acked)
	return cue.Available
}

// boolToYesNo converts a bool to "yes" or "no" string.
func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// PeriodKey returns the day bucket key for the given time.
// Format: YYYY-MM-DD
func PeriodKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// TimeBucketMinutes floors a timestamp to the given minute interval.
// Used for privacy-preserving time comparisons.
func TimeBucketMinutes(t time.Time, minutes int) time.Time {
	if minutes <= 0 {
		minutes = 5
	}
	return t.Truncate(time.Duration(minutes) * time.Minute)
}

// ComputeSyncBucket determines the sync bucket from the last sync time.
// Uses 15-minute threshold for "recent" vs "stale".
func ComputeSyncBucket(lastSyncTime time.Time, now time.Time) reality.SyncBucket {
	if lastSyncTime.IsZero() {
		return reality.SyncBucketNever
	}

	// Use bucketed times for comparison (15 minute buckets)
	lastBucket := TimeBucketMinutes(lastSyncTime, 15)
	nowBucket := TimeBucketMinutes(now, 15)

	// If within same bucket or adjacent bucket, consider recent
	diff := nowBucket.Sub(lastBucket)
	if diff <= 30*time.Minute {
		return reality.SyncBucketRecent
	}

	return reality.SyncBucketStale
}

// MapSyncReceiptMagnitude maps sync receipt magnitude to reality magnitude.
func MapSyncReceiptMagnitude(syncMag string) reality.MagnitudeBucket {
	switch syncMag {
	case "none":
		return reality.MagnitudeNothing
	case "handful":
		return reality.MagnitudeAFew
	case "several", "many":
		return reality.MagnitudeSeveral
	default:
		return reality.MagnitudeNA
	}
}

// MapShadowReceiptCount maps shadow receipt count to magnitude bucket.
func MapShadowReceiptCount(count int) reality.MagnitudeBucket {
	switch {
	case count == 0:
		return reality.MagnitudeNothing
	case count <= 5:
		return reality.MagnitudeAFew
	default:
		return reality.MagnitudeSeveral
	}
}

// MapProviderKind maps config provider kind to reality provider kind.
func MapProviderKind(configKind string, mode string) reality.ShadowProviderKind {
	if mode == "off" || mode == "" {
		return reality.ProviderOff
	}

	switch configKind {
	case "stub", "none", "":
		return reality.ProviderStub
	case "azure_openai":
		return reality.ProviderAzureChat
	default:
		return reality.ProviderStub
	}
}
