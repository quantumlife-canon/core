// Package attentionenvelope implements the Phase 39 Attention Envelope engine.
//
// The engine provides functions to:
// - Build envelopes with deterministic hashes
// - Check envelope active/expired status
// - Apply envelope effects to pressure input
// - Build receipts for audit trail
//
// CRITICAL INVARIANTS:
//   - Pure functions. No side effects.
//   - Deterministic: same inputs + clock => same outputs.
//   - No goroutines. Clock injection required.
//   - Effects bounded: max 1 step horizon, +1 magnitude, +1 cap.
//   - Commerce excluded: never escalated.
//   - Does NOT force interrupts (Phase 33/34 still apply).
//
// Reference: docs/ADR/ADR-0076-phase39-attention-envelopes.md
package attentionenvelope

import (
	"fmt"
	"time"

	ae "quantumlife/pkg/domain/attentionenvelope"
	pd "quantumlife/pkg/domain/pressuredecision"
)

// Engine computes attention envelope effects.
// CRITICAL: No side effects. Pure functions. Same inputs => same outputs.
type Engine struct{}

// NewEngine creates a new envelope engine.
func NewEngine() *Engine {
	return &Engine{}
}

// BuildEnvelope creates a new attention envelope with computed hashes.
// CRITICAL: Deterministic. Same inputs + clock => same envelope.
func (e *Engine) BuildEnvelope(
	kind ae.EnvelopeKind,
	duration ae.DurationBucket,
	reason ae.EnvelopeReason,
	circleIDHash string,
	clock time.Time,
) (*ae.AttentionEnvelope, error) {
	// Validate inputs
	if err := kind.Validate(); err != nil {
		return nil, err
	}
	if err := duration.Validate(); err != nil {
		return nil, err
	}
	if err := reason.Validate(); err != nil {
		return nil, err
	}
	if circleIDHash == "" {
		return nil, fmt.Errorf("missing circle_id_hash")
	}

	startPeriod := ae.NewPeriodKey(clock)
	expiryPeriod, err := ae.ComputeExpiryPeriod(startPeriod, duration)
	if err != nil {
		return nil, err
	}

	envelope := &ae.AttentionEnvelope{
		CircleIDHash:    circleIDHash,
		Kind:            kind,
		Duration:        duration,
		Reason:          reason,
		State:           ae.StateActive,
		StartedPeriod:   startPeriod,
		ExpiresAtPeriod: expiryPeriod,
	}

	envelope.EnvelopeID = envelope.ComputeEnvelopeID()
	envelope.StatusHash = envelope.ComputeStatusHash()

	return envelope, nil
}

// IsActive checks if an envelope is currently active.
// CRITICAL: Checks both state and expiry against clock.
func (e *Engine) IsActive(envelope *ae.AttentionEnvelope, clock time.Time) bool {
	if envelope == nil {
		return false
	}

	// Must be in active state
	if envelope.State != ae.StateActive {
		return false
	}

	// Must not be expired
	expired, err := ae.IsExpired(envelope.ExpiresAtPeriod, clock)
	if err != nil {
		return false
	}

	return !expired
}

// HasExpired checks if an envelope has expired based on clock.
// CRITICAL: Only checks time, not state.
func (e *Engine) HasExpired(envelope *ae.AttentionEnvelope, clock time.Time) bool {
	if envelope == nil {
		return true
	}

	expired, err := ae.IsExpired(envelope.ExpiresAtPeriod, clock)
	if err != nil {
		return true
	}

	return expired
}

// ApplyEnvelope applies envelope effects to a pressure input.
// CRITICAL: Returns a COPY. Does not mutate input.
// CRITICAL: Commerce excluded - returns input unchanged.
// CRITICAL: Effects bounded - max 1 step horizon, +1 magnitude.
func (e *Engine) ApplyEnvelope(
	envelope *ae.AttentionEnvelope,
	input *pd.PressureDecisionInput,
) *pd.PressureDecisionInput {
	if envelope == nil || input == nil {
		return input
	}

	// If envelope kind is none, no changes
	if envelope.Kind == ae.EnvelopeKindNone {
		return input
	}

	// Commerce exclusion: NEVER escalate commerce pressure
	if input.CircleType == pd.CircleTypeCommerce {
		return input
	}

	// Create a copy to avoid mutating input
	modified := *input

	// Apply horizon shift (max 1 step earlier)
	modified.Horizon = e.computeHorizonShift(envelope.Kind, input.Horizon)

	// Apply magnitude bias (max +1 bucket)
	modified.Magnitude = e.computeMagnitudeBias(envelope.Kind, input.Magnitude)

	return &modified
}

// ComputeCapDelta returns the interrupt cap delta for an envelope kind.
// CRITICAL: Returns 0 or 1 only. Never more.
func (e *Engine) ComputeCapDelta(kind ae.EnvelopeKind) int {
	switch kind {
	case ae.EnvelopeKindOnCall, ae.EnvelopeKindEmergency:
		return 1
	default:
		return 0
	}
}

// computeHorizonShift shifts horizon by at most 1 step earlier.
// CRITICAL: Max 1 step. later->soon, soon->now, now stays now.
func (e *Engine) computeHorizonShift(kind ae.EnvelopeKind, current pd.PressureHorizon) pd.PressureHorizon {
	// Only certain kinds shift horizon
	shouldShift := false
	switch kind {
	case ae.EnvelopeKindOnCall, ae.EnvelopeKindTravel, ae.EnvelopeKindEmergency:
		shouldShift = true
	}

	if !shouldShift {
		return current
	}

	// Shift by exactly 1 step earlier (never more)
	switch current {
	case pd.HorizonLater:
		return pd.HorizonSoon
	case pd.HorizonSoon:
		return pd.HorizonNow
	case pd.HorizonUnknown:
		return pd.HorizonSoon // Unknown becomes Soon
	default:
		return current // Now stays Now
	}
}

// computeMagnitudeBias increases magnitude by at most 1 bucket.
// CRITICAL: Max +1. nothing->a_few, a_few->several, several stays several.
func (e *Engine) computeMagnitudeBias(kind ae.EnvelopeKind, current pd.PressureMagnitude) pd.PressureMagnitude {
	// Only certain kinds add magnitude bias
	shouldBias := false
	switch kind {
	case ae.EnvelopeKindOnCall, ae.EnvelopeKindWorking, ae.EnvelopeKindEmergency:
		shouldBias = true
	}

	if !shouldBias {
		return current
	}

	// Increase by exactly 1 bucket (never more)
	switch current {
	case pd.MagnitudeNothing:
		return pd.MagnitudeAFew
	case pd.MagnitudeAFew:
		return pd.MagnitudeSeveral
	default:
		return current // Several stays Several
	}
}

// BuildReceipt creates a receipt for an envelope action.
// CRITICAL: Deterministic. Same inputs => same receipt.
func (e *Engine) BuildReceipt(
	envelope *ae.AttentionEnvelope,
	action ae.EnvelopeAction,
	clock time.Time,
) *ae.EnvelopeReceipt {
	if envelope == nil {
		return nil
	}

	receipt := &ae.EnvelopeReceipt{
		EnvelopeHash: envelope.EnvelopeID,
		CircleIDHash: envelope.CircleIDHash,
		Action:       action,
		PeriodKey:    ae.NewDayKey(clock),
	}

	receipt.ReceiptID = receipt.ComputeReceiptID()
	receipt.StatusHash = receipt.ComputeStatusHash()

	return receipt
}

// StopEnvelope transitions an envelope to stopped state.
// CRITICAL: Returns a NEW envelope. Does not mutate input.
func (e *Engine) StopEnvelope(envelope *ae.AttentionEnvelope) *ae.AttentionEnvelope {
	if envelope == nil {
		return nil
	}

	// Create a copy with new state
	stopped := *envelope
	stopped.State = ae.StateStopped
	stopped.StatusHash = stopped.ComputeStatusHash()

	return &stopped
}

// ExpireEnvelope transitions an envelope to expired state.
// CRITICAL: Returns a NEW envelope. Does not mutate input.
func (e *Engine) ExpireEnvelope(envelope *ae.AttentionEnvelope) *ae.AttentionEnvelope {
	if envelope == nil {
		return nil
	}

	// Create a copy with new state
	expired := *envelope
	expired.State = ae.StateExpired
	expired.StatusHash = expired.ComputeStatusHash()

	return &expired
}

// BuildProofPage builds a proof page for display.
// CRITICAL: Contains only hashes and buckets. No raw timestamps.
func (e *Engine) BuildProofPage(
	circleIDHash string,
	currentEnvelope *ae.AttentionEnvelope,
	recentReceiptCount int,
) *ae.EnvelopeProofPage {
	page := &ae.EnvelopeProofPage{
		CircleIDHash:       circleIDHash,
		RecentReceiptCount: ae.BucketReceiptCount(recentReceiptCount),
	}

	if currentEnvelope != nil {
		page.CurrentEnvelopeHash = currentEnvelope.EnvelopeID
		page.CurrentKind = currentEnvelope.Kind
		page.CurrentDuration = currentEnvelope.Duration
		page.CurrentState = currentEnvelope.State
	} else {
		page.CurrentKind = ae.EnvelopeKindNone
		page.CurrentState = ae.StateStopped // No active envelope
	}

	page.PageHash = page.ComputePageHash()

	return page
}

// ValidateEnvelopeStart validates parameters for starting an envelope.
// CRITICAL: All parameters must be valid enums.
func (e *Engine) ValidateEnvelopeStart(kind, duration, reason string) error {
	if err := ae.EnvelopeKind(kind).Validate(); err != nil {
		return err
	}
	if err := ae.DurationBucket(duration).Validate(); err != nil {
		return err
	}
	if err := ae.EnvelopeReason(reason).Validate(); err != nil {
		return err
	}
	return nil
}

// GetEffectDescription returns a human-readable description of envelope effects.
// CRITICAL: Calm language. No urgency.
func (e *Engine) GetEffectDescription(kind ae.EnvelopeKind) string {
	switch kind {
	case ae.EnvelopeKindNone:
		return "No effect. Normal behavior."
	case ae.EnvelopeKindOnCall:
		return "Slightly heightened responsiveness for on-call duty."
	case ae.EnvelopeKindWorking:
		return "Slightly increased awareness during focused work."
	case ae.EnvelopeKindTravel:
		return "Slightly earlier awareness for time-sensitive transit."
	case ae.EnvelopeKindEmergency:
		return "Heightened responsiveness for emergencies."
	default:
		return "Unknown effect."
	}
}
