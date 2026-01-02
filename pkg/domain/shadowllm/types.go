// Package shadowllm provides types for LLM shadow-mode observation.
//
// Phase 19: LLM Shadow-Mode Contract
//
// CRITICAL INVARIANTS:
//   - Shadow mode emits METADATA ONLY (scores, deltas, confidence) - never content
//   - Shadow mode can NEVER: surface UI text, create obligations/drafts,
//     alter interruption levels, trigger execution, write back to providers
//   - Shadow mode is OFF by default, requires explicit config flag
//   - No goroutines. No time.Now() - clock injection only.
//   - Deterministic: same inputs + same seed + same clock => identical outputs/hashes
//   - Stdlib only. No external dependencies.
//
// Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
package shadowllm

import (
	"time"

	"quantumlife/pkg/domain/identity"
)

// ShadowMode defines the shadow-mode operation mode.
type ShadowMode string

const (
	// ShadowModeOff means shadow-mode is disabled (default).
	ShadowModeOff ShadowMode = "off"

	// ShadowModeObserve means shadow-mode runs after the quiet loop, emitting metadata only.
	ShadowModeObserve ShadowMode = "observe"
)

// IsEnabled returns true if shadow mode is not "off".
func (m ShadowMode) IsEnabled() bool {
	return m == ShadowModeObserve
}

// Validate checks if the mode is valid.
func (m ShadowMode) Validate() bool {
	return m == ShadowModeOff || m == ShadowModeObserve
}

// ShadowSignalKind identifies the type of signal emitted by shadow-mode.
type ShadowSignalKind string

const (
	// SignalKindRegretDelta represents a suggested change to regret score.
	SignalKindRegretDelta ShadowSignalKind = "regret_delta"

	// SignalKindCategoryPressure represents pressure to surface a category.
	SignalKindCategoryPressure ShadowSignalKind = "category_pressure"

	// SignalKindConfidence represents confidence in current state.
	SignalKindConfidence ShadowSignalKind = "confidence"

	// SignalKindLabelSuggestion represents a suggested label/category.
	SignalKindLabelSuggestion ShadowSignalKind = "label_suggestion"
)

// Validate checks if the signal kind is valid.
func (k ShadowSignalKind) Validate() bool {
	switch k {
	case SignalKindRegretDelta, SignalKindCategoryPressure,
		SignalKindConfidence, SignalKindLabelSuggestion:
		return true
	default:
		return false
	}
}

// AbstractCategory represents abstract categories (no identifiable info).
type AbstractCategory string

const (
	CategoryMoney  AbstractCategory = "money"
	CategoryTime   AbstractCategory = "time"
	CategoryPeople AbstractCategory = "people"
	CategoryWork   AbstractCategory = "work"
	CategoryHome   AbstractCategory = "home"
)

// Validate checks if the category is valid.
func (c AbstractCategory) Validate() bool {
	switch c {
	case CategoryMoney, CategoryTime, CategoryPeople, CategoryWork, CategoryHome:
		return true
	default:
		return false
	}
}

// ShadowSignal represents a single metadata signal from shadow-mode.
//
// CRITICAL: Contains NO content, NO identifiable information.
// Only: kind, circle, item hash, abstract category, float values, and hashes.
type ShadowSignal struct {
	// Kind identifies the type of signal.
	Kind ShadowSignalKind

	// CircleID is the circle this signal relates to.
	CircleID identity.EntityID

	// ItemKeyHash is a SHA256 hash of the item key (not the item itself).
	ItemKeyHash string

	// Category is the abstract category (money, time, people, etc.)
	Category AbstractCategory

	// ValueFloat is the primary numeric value (e.g., delta, pressure level).
	// Range: -1.0 to 1.0
	ValueFloat float64

	// ConfidenceFloat is the confidence in this signal.
	// Range: 0.0 to 1.0
	ConfidenceFloat float64

	// NotesHash is a hash of any internal notes (NOT the notes themselves).
	// This ensures no content leaks through signals.
	NotesHash string

	// CreatedAt is when this signal was generated (injected clock).
	CreatedAt time.Time
}

// Validate checks if the signal is valid.
func (s *ShadowSignal) Validate() error {
	if !s.Kind.Validate() {
		return ErrInvalidSignalKind
	}
	if s.CircleID == "" {
		return ErrMissingCircleID
	}
	if s.ItemKeyHash == "" {
		return ErrMissingItemKeyHash
	}
	if !s.Category.Validate() {
		return ErrInvalidCategory
	}
	if s.ValueFloat < -1.0 || s.ValueFloat > 1.0 {
		return ErrValueOutOfRange
	}
	if s.ConfidenceFloat < 0.0 || s.ConfidenceFloat > 1.0 {
		return ErrConfidenceOutOfRange
	}
	if s.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}
	return nil
}

// ShadowRun represents a complete shadow-mode observation run.
//
// CRITICAL: Contains only metadata. Never content or identifiable info.
type ShadowRun struct {
	// RunID uniquely identifies this run.
	RunID string

	// CircleID is the circle this run is for.
	CircleID identity.EntityID

	// InputsHash is a SHA256 hash of the abstract inputs (not the inputs themselves).
	InputsHash string

	// ModelSpec identifies the model used (name only, no API keys/secrets).
	ModelSpec string

	// Seed is the deterministic seed for reproducibility.
	Seed int64

	// Signals are the metadata signals produced by this run.
	// Max 5 signals per run.
	Signals []ShadowSignal

	// CreatedAt is when this run was created (injected clock).
	CreatedAt time.Time

	// hash is cached after first computation.
	hash string
}

// MaxSignalsPerRun is the maximum number of signals allowed per run.
const MaxSignalsPerRun = 5

// Validate checks if the run is valid.
func (r *ShadowRun) Validate() error {
	if r.RunID == "" {
		return ErrMissingRunID
	}
	if r.CircleID == "" {
		return ErrMissingCircleID
	}
	if r.InputsHash == "" {
		return ErrMissingInputsHash
	}
	if r.ModelSpec == "" {
		return ErrMissingModelSpec
	}
	if len(r.Signals) > MaxSignalsPerRun {
		return ErrTooManySignals
	}
	if r.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}

	for i := range r.Signals {
		if err := r.Signals[i].Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Error types for validation.
type shadowError string

func (e shadowError) Error() string { return string(e) }

const (
	ErrInvalidSignalKind    shadowError = "invalid signal kind"
	ErrMissingCircleID      shadowError = "missing circle ID"
	ErrMissingItemKeyHash   shadowError = "missing item key hash"
	ErrInvalidCategory      shadowError = "invalid category"
	ErrValueOutOfRange      shadowError = "value out of range [-1.0, 1.0]"
	ErrConfidenceOutOfRange shadowError = "confidence out of range [0.0, 1.0]"
	ErrMissingCreatedAt     shadowError = "missing created at timestamp"
	ErrMissingRunID         shadowError = "missing run ID"
	ErrMissingInputsHash    shadowError = "missing inputs hash"
	ErrMissingModelSpec     shadowError = "missing model spec"
	ErrTooManySignals       shadowError = "too many signals (max 5)"
)
