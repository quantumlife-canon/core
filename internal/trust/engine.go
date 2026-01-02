// Package trust provides the Trust Accrual Engine.
//
// Phase 20: Trust Accrual Layer (Proof Over Time)
//
// CRITICAL INVARIANTS:
//   - Silence is the default outcome
//   - Trust signals are NEVER pushed
//   - Trust signals are NEVER frequent
//   - Trust signals are NEVER actionable
//   - Only abstract buckets (nothing / a_few / several)
//   - NO timestamps, counts, vendors, people, or content
//   - Deterministic: same inputs + clock => same hashes
//   - Idempotent and replay-safe
//   - No goroutines, no time.Now()
//
// This engine consumes existing storelogs and aggregates them into
// TrustSummaries that make restraint observable retrospectively.
//
// Reference: docs/ADR/ADR-0048-phase20-trust-accrual-layer.md
package trust

import (
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/trust"
)

// =============================================================================
// Data Source Interfaces
// =============================================================================

// RestraintSource provides access to evidence of restraint.
// Implementations wrap existing stores without exposing raw data.
type RestraintSource interface {
	// GetHeldCount returns the count of obligations held quietly in a period.
	// CRITICAL: The engine will bucket this - callers must not expose counts.
	GetHeldCount(periodKey string, period trust.TrustPeriod) int

	// GetSuppressionCount returns the count of suppressions applied in a period.
	// CRITICAL: The engine will bucket this - callers must not expose counts.
	GetSuppressionCount(periodKey string, period trust.TrustPeriod) int

	// GetShadowRejectionCount returns the count of shadow suggestions rejected.
	// CRITICAL: The engine will bucket this - callers must not expose counts.
	GetShadowRejectionCount(periodKey string, period trust.TrustPeriod) int
}

// =============================================================================
// Engine
// =============================================================================

// Engine computes TrustSummaries from restraint evidence.
//
// CRITICAL:
//   - Does NOT push notifications
//   - Does NOT create engagement pressure
//   - Produces zero output if nothing meaningful occurred
//   - Deterministic: same inputs + clock => same outputs
type Engine struct {
	clk clock.Clock
}

// NewEngine creates a new Trust Accrual Engine.
func NewEngine(clk clock.Clock) *Engine {
	return &Engine{clk: clk}
}

// ComputeInput contains inputs for computing a trust summary.
type ComputeInput struct {
	// Period is the time granularity (week | month).
	Period trust.TrustPeriod

	// PeriodKey is the abstract period identifier.
	PeriodKey string

	// Source provides access to restraint evidence.
	Source RestraintSource
}

// ComputeOutput contains the computed trust summary.
type ComputeOutput struct {
	// Summary is the computed TrustSummary.
	// May be nil if nothing meaningful occurred.
	Summary *trust.TrustSummary

	// HeldMagnitude is the magnitude of held obligations.
	HeldMagnitude shadowllm.MagnitudeBucket

	// SuppressionMagnitude is the magnitude of suppressions.
	SuppressionMagnitude shadowllm.MagnitudeBucket

	// RejectionMagnitude is the magnitude of shadow rejections.
	RejectionMagnitude shadowllm.MagnitudeBucket

	// Meaningful is true if any evidence of restraint was found.
	Meaningful bool
}

// Compute calculates a TrustSummary for the given period.
//
// CRITICAL:
//   - Returns nil Summary if nothing meaningful occurred
//   - Uses magnitude buckets only - never raw counts
//   - Deterministic: same inputs + clock => same hash
func (e *Engine) Compute(input ComputeInput) (*ComputeOutput, error) {
	if input.PeriodKey == "" {
		return nil, trust.ErrMissingPeriodKey
	}
	if !input.Period.Validate() {
		return nil, trust.ErrInvalidPeriod
	}

	// Gather evidence (raw counts, internal only)
	heldCount := 0
	suppressionCount := 0
	rejectionCount := 0

	if input.Source != nil {
		heldCount = input.Source.GetHeldCount(input.PeriodKey, input.Period)
		suppressionCount = input.Source.GetSuppressionCount(input.PeriodKey, input.Period)
		rejectionCount = input.Source.GetShadowRejectionCount(input.PeriodKey, input.Period)
	}

	// Convert to magnitude buckets (abstract only)
	heldMagnitude := countToMagnitude(heldCount)
	suppressionMagnitude := countToMagnitude(suppressionCount)
	rejectionMagnitude := countToMagnitude(rejectionCount)

	// Determine the dominant signal kind
	signalKind, overallMagnitude := determineSignal(
		heldMagnitude,
		suppressionMagnitude,
		rejectionMagnitude,
	)

	// Check if anything meaningful occurred
	meaningful := overallMagnitude != shadowllm.MagnitudeNothing

	output := &ComputeOutput{
		HeldMagnitude:        heldMagnitude,
		SuppressionMagnitude: suppressionMagnitude,
		RejectionMagnitude:   rejectionMagnitude,
		Meaningful:           meaningful,
	}

	// If nothing meaningful, return nil summary
	// "Nothing happened" is valid - no summary needed
	if !meaningful {
		return output, nil
	}

	// Build the summary
	now := e.clk.Now()
	summary := &trust.TrustSummary{
		Period:          input.Period,
		PeriodKey:       input.PeriodKey,
		SignalKind:      signalKind,
		MagnitudeBucket: overallMagnitude,
		CreatedBucket:   trust.FiveMinuteBucket(now),
		CreatedAt:       now,
	}

	// Compute deterministic ID and hash
	summary.SummaryID = summary.ComputeID()
	summary.SummaryHash = summary.ComputeHash()

	output.Summary = summary
	return output, nil
}

// =============================================================================
// Period Key Helpers
// =============================================================================

// CurrentPeriodKey returns the period key for the current time.
func (e *Engine) CurrentPeriodKey(period trust.TrustPeriod) string {
	now := e.clk.Now()
	switch period {
	case trust.PeriodWeek:
		return trust.WeekKey(now)
	case trust.PeriodMonth:
		return trust.MonthKey(now)
	default:
		return ""
	}
}

// PreviousPeriodKey returns the period key for the previous period.
func (e *Engine) PreviousPeriodKey(period trust.TrustPeriod) string {
	now := e.clk.Now()
	switch period {
	case trust.PeriodWeek:
		// Go back 7 days
		prev := now.AddDate(0, 0, -7)
		return trust.WeekKey(prev)
	case trust.PeriodMonth:
		// Go back 1 month
		prev := now.AddDate(0, -1, 0)
		return trust.MonthKey(prev)
	default:
		return ""
	}
}

// =============================================================================
// Internal Helpers
// =============================================================================

// countToMagnitude converts a raw count to a magnitude bucket.
// CRITICAL: This is the ONLY place where raw counts are processed.
// The counts never leave this function - only buckets are returned.
func countToMagnitude(count int) shadowllm.MagnitudeBucket {
	if count == 0 {
		return shadowllm.MagnitudeNothing
	}
	if count <= 3 {
		return shadowllm.MagnitudeAFew
	}
	return shadowllm.MagnitudeSeveral
}

// determineSignal determines the dominant signal kind and overall magnitude.
func determineSignal(
	held shadowllm.MagnitudeBucket,
	suppressed shadowllm.MagnitudeBucket,
	rejected shadowllm.MagnitudeBucket,
) (trust.TrustSignalKind, shadowllm.MagnitudeBucket) {

	// Priority order: suppressions > held > rejected
	// (suppressions are most active form of restraint)

	if suppressed != shadowllm.MagnitudeNothing {
		return trust.SignalInterruptionPrevented, suppressed
	}

	if held != shadowllm.MagnitudeNothing {
		return trust.SignalQuietHeld, held
	}

	if rejected != shadowllm.MagnitudeNothing {
		// Shadow rejections count as held quietly
		return trust.SignalQuietHeld, rejected
	}

	// Nothing happened - natural silence
	return trust.SignalNothingRequired, shadowllm.MagnitudeNothing
}

// =============================================================================
// Null Source (for testing)
// =============================================================================

// NullSource is a RestraintSource that returns zero for all counts.
// Used for testing and when no real data is available.
type NullSource struct{}

func (NullSource) GetHeldCount(string, trust.TrustPeriod) int         { return 0 }
func (NullSource) GetSuppressionCount(string, trust.TrustPeriod) int  { return 0 }
func (NullSource) GetShadowRejectionCount(string, trust.TrustPeriod) int { return 0 }

// =============================================================================
// Mock Source (for testing)
// =============================================================================

// MockSource is a configurable RestraintSource for testing.
type MockSource struct {
	HeldCounts       map[string]int
	SuppressionCounts map[string]int
	RejectionCounts  map[string]int
}

// NewMockSource creates a new mock source.
func NewMockSource() *MockSource {
	return &MockSource{
		HeldCounts:       make(map[string]int),
		SuppressionCounts: make(map[string]int),
		RejectionCounts:  make(map[string]int),
	}
}

func (m *MockSource) GetHeldCount(periodKey string, _ trust.TrustPeriod) int {
	return m.HeldCounts[periodKey]
}

func (m *MockSource) GetSuppressionCount(periodKey string, _ trust.TrustPeriod) int {
	return m.SuppressionCounts[periodKey]
}

func (m *MockSource) GetShadowRejectionCount(periodKey string, _ trust.TrustPeriod) int {
	return m.RejectionCounts[periodKey]
}
