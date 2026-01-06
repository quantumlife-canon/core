// Package firstminutes provides domain types for Phase 26B: First Five Minutes Proof.
//
// This is NOT analytics. This is NOT telemetry. This is NOT engagement tracking.
// This is narrative proof - a deterministic, privacy-safe, shareable summary
// that the first-time onboarding journey worked.
//
// CRITICAL: All payloads contain hashes only - never identifiers.
package firstminutes

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// FirstMinutesPeriod represents a day bucket (YYYY-MM-DD format).
// This is the only temporal granularity exposed - no timestamps.
type FirstMinutesPeriod string

// FirstMinutesSignalKind represents what happened in the first minutes.
// These map directly to existing stores - no new data collection.
type FirstMinutesSignalKind string

const (
	// SignalConnected - circle connected a data source (from connection store).
	SignalConnected FirstMinutesSignalKind = "connected"

	// SignalSynced - data was synced (from sync receipt store).
	SignalSynced FirstMinutesSignalKind = "synced"

	// SignalMirrored - data was mirrored quietly (from mirror store).
	SignalMirrored FirstMinutesSignalKind = "mirrored"

	// SignalHeld - items were held without interruption (from trust store).
	SignalHeld FirstMinutesSignalKind = "held"

	// SignalActionPreviewed - an action was previewed (from first action store).
	SignalActionPreviewed FirstMinutesSignalKind = "action_previewed"

	// SignalActionExecuted - an action was executed (from undo store).
	SignalActionExecuted FirstMinutesSignalKind = "action_executed"

	// SignalSilencePreserved - nothing required attention (derived).
	SignalSilencePreserved FirstMinutesSignalKind = "silence_preserved"
)

// AllSignalKinds returns all signal kinds in canonical order.
func AllSignalKinds() []FirstMinutesSignalKind {
	return []FirstMinutesSignalKind{
		SignalConnected,
		SignalSynced,
		SignalMirrored,
		SignalHeld,
		SignalActionPreviewed,
		SignalActionExecuted,
		SignalSilencePreserved,
	}
}

// MagnitudeBucket represents an abstract quantity - never raw counts.
type MagnitudeBucket string

const (
	// MagnitudeNothing - zero items.
	MagnitudeNothing MagnitudeBucket = "nothing"

	// MagnitudeAFew - a small number (1-5).
	MagnitudeAFew MagnitudeBucket = "a_few"

	// MagnitudeSeveral - a moderate number (6+).
	MagnitudeSeveral MagnitudeBucket = "several"
)

// FirstMinutesSignal represents a single signal with its magnitude.
type FirstMinutesSignal struct {
	Kind      FirstMinutesSignalKind
	Magnitude MagnitudeBucket
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (s FirstMinutesSignal) CanonicalString() string {
	return "SIGNAL|v1|" + string(s.Kind) + "|" + string(s.Magnitude)
}

// FirstMinutesSummary is the receipt - a single calm artifact.
type FirstMinutesSummary struct {
	Period     FirstMinutesPeriod
	Signals    []FirstMinutesSignal
	CalmLine   string // Single sentence summary - the narrative
	StatusHash string // 128-bit deterministic hash (32 hex chars)
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (s *FirstMinutesSummary) CanonicalString() string {
	var b strings.Builder
	b.WriteString("FIRST_MINUTES|v1|")
	b.WriteString(string(s.Period))
	b.WriteString("|")

	// Sort signals for determinism
	sortedSignals := make([]FirstMinutesSignal, len(s.Signals))
	copy(sortedSignals, s.Signals)
	sort.Slice(sortedSignals, func(i, j int) bool {
		return string(sortedSignals[i].Kind) < string(sortedSignals[j].Kind)
	})

	for i, sig := range sortedSignals {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(sig.CanonicalString())
	}
	b.WriteString("|")
	b.WriteString(s.CalmLine)

	return b.String()
}

// ComputeStatusHash computes the deterministic 128-bit hash of the summary.
// Returns 32 hex characters.
func (s *FirstMinutesSummary) ComputeStatusHash() string {
	canonical := s.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16]) // 128 bits = 16 bytes = 32 hex chars
}

// FirstMinutesDismissal tracks when a summary was dismissed.
type FirstMinutesDismissal struct {
	Period      FirstMinutesPeriod
	SummaryHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (d *FirstMinutesDismissal) CanonicalString() string {
	return "FIRST_MINUTES_DISMISS|v1|" + string(d.Period) + "|" + d.SummaryHash
}

// FirstMinutesInputs captures all inputs needed to compute a summary.
// These are gathered from existing stores - no new data collection.
type FirstMinutesInputs struct {
	// CircleID identifies the circle (required for scoping).
	CircleID string

	// Period is the day bucket being summarized.
	Period FirstMinutesPeriod

	// From connection store
	HasConnection  bool
	ConnectionMode string // "mock" or "real"

	// From sync receipt store
	HasSyncReceipt bool
	SyncMagnitude  MagnitudeBucket

	// From mirror store
	HasMirror       bool
	MirrorMagnitude MagnitudeBucket

	// From trust store
	HasHeldItems  bool
	HeldMagnitude MagnitudeBucket

	// From first action store
	ActionPreviewed bool

	// From undo store
	ActionExecuted bool

	// Dismissal state
	DismissedSummaryHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (i *FirstMinutesInputs) CanonicalString() string {
	var b strings.Builder
	b.WriteString("FIRST_MINUTES_INPUTS|v1|")
	b.WriteString(i.CircleID)
	b.WriteString("|")
	b.WriteString(string(i.Period))
	b.WriteString("|")

	// Connection
	if i.HasConnection {
		b.WriteString("conn:")
		b.WriteString(i.ConnectionMode)
	} else {
		b.WriteString("no_conn")
	}
	b.WriteString("|")

	// Sync
	if i.HasSyncReceipt {
		b.WriteString("sync:")
		b.WriteString(string(i.SyncMagnitude))
	} else {
		b.WriteString("no_sync")
	}
	b.WriteString("|")

	// Mirror
	if i.HasMirror {
		b.WriteString("mirror:")
		b.WriteString(string(i.MirrorMagnitude))
	} else {
		b.WriteString("no_mirror")
	}
	b.WriteString("|")

	// Held
	if i.HasHeldItems {
		b.WriteString("held:")
		b.WriteString(string(i.HeldMagnitude))
	} else {
		b.WriteString("no_held")
	}
	b.WriteString("|")

	// Actions
	if i.ActionPreviewed {
		b.WriteString("previewed")
	} else {
		b.WriteString("no_preview")
	}
	b.WriteString("|")

	if i.ActionExecuted {
		b.WriteString("executed")
	} else {
		b.WriteString("no_exec")
	}

	return b.String()
}

// ComputeInputsHash computes the deterministic hash of inputs.
func (i *FirstMinutesInputs) ComputeInputsHash() string {
	canonical := i.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// HasMeaningfulSignals returns true if there are any signals worth showing.
// If false, silence is the success state.
func (i *FirstMinutesInputs) HasMeaningfulSignals() bool {
	return i.HasConnection || i.HasSyncReceipt || i.HasMirror ||
		i.HasHeldItems || i.ActionPreviewed || i.ActionExecuted
}

// FirstMinutesCue represents the whisper cue for the first minutes receipt.
type FirstMinutesCue struct {
	Available   bool
	CueText     string
	SummaryHash string
	Period      FirstMinutesPeriod
}

// DefaultCueText is the subtle cue text shown on /today.
const DefaultCueText = "If you ever wondered how the beginning went."
