// Package urgencyresolve provides the engine for Phase 53: Urgency Resolution Layer.
//
// CRITICAL INVARIANTS:
// - NO POWER: This engine is cap-only, clamp-only. It MUST NOT deliver push,
//   execute anything, or add any observers. Proof only.
// - PURE FUNCTIONS: All operations are deterministic.
// - CLOCK INJECTION: Time is passed in via injected clock interface.
// - COMMERCE NEVER ESCALATES: Commerce always gets cap_hold_only.
// - CAPS ONLY REDUCE: Caps can only reduce escalation; never increase power.
// - REASONS MAX 3: Reasons are deterministically sorted and capped at 3.
//
// Reference: docs/ADR/ADR-0091-phase53-urgency-resolution-layer.md
package urgencyresolve

import (
	"time"

	domain "quantumlife/pkg/domain/urgencyresolve"
)

// ============================================================================
// Clock Interface
// ============================================================================

// Clock provides the current time for period key derivation.
type Clock interface {
	Now() time.Time
}

// ============================================================================
// Source Interfaces (read-only adapters)
// ============================================================================

// PressureSource provides pressure outcome data.
type PressureSource interface {
	// GetPressureOutcome returns the pressure outcome kind for a circle.
	GetPressureOutcome(circleIDHash string) string
}

// EnvelopeSource provides envelope status.
type EnvelopeSource interface {
	// IsEnvelopeActive returns whether an envelope is active for a circle.
	IsEnvelopeActive(circleIDHash string) bool
}

// TimeWindowSource provides time window signal data.
type TimeWindowSource interface {
	// GetWindowSignal returns the window signal state for a circle.
	GetWindowSignal(circleIDHash string) domain.WindowSignalBucket
}

// VendorCapSource provides vendor contract cap data.
type VendorCapSource interface {
	// GetVendorCap returns the vendor contract cap for a circle.
	GetVendorCap(circleIDHash string) domain.EscalationCap
}

// SemanticsSource provides semantics/necessity data.
type SemanticsSource interface {
	// IsNecessityDeclared returns whether necessity was declared for a circle.
	IsNecessityDeclared(circleIDHash string) bool
}

// TrustSource provides trust status data.
type TrustSource interface {
	// IsTrustFragile returns whether trust is fragile for a circle.
	IsTrustFragile(circleIDHash string) bool
}

// PolicySource provides interrupt policy data.
type PolicySource interface {
	// GetInterruptAllowance returns the interrupt allowance for a circle.
	GetInterruptAllowance(circleIDHash string) domain.InterruptAllowanceBucket
}

// CircleTypeSource provides circle type data.
type CircleTypeSource interface {
	// GetCircleType returns the circle type bucket for a circle.
	GetCircleType(circleIDHash string) domain.CircleTypeBucket
}

// HorizonSource provides time horizon data.
type HorizonSource interface {
	// GetHorizonBucket returns the horizon bucket for a circle.
	GetHorizonBucket(circleIDHash string) domain.HorizonBucket
}

// MagnitudeSource provides magnitude data.
type MagnitudeSource interface {
	// GetMagnitudeBucket returns the magnitude bucket for a circle.
	GetMagnitudeBucket(circleIDHash string) domain.MagnitudeBucket
}

// AckSource provides ack status data.
type AckSource interface {
	// IsDismissed returns whether the resolution was dismissed.
	IsDismissed(circleIDHash, periodKey, resolutionHash string) bool
}

// ============================================================================
// Engine
// ============================================================================

// Engine computes urgency resolutions.
type Engine struct {
	clk              Clock
	pressureSource   PressureSource
	envelopeSource   EnvelopeSource
	windowSource     TimeWindowSource
	vendorCapSource  VendorCapSource
	semanticsSource  SemanticsSource
	trustSource      TrustSource
	policySource     PolicySource
	circleTypeSource CircleTypeSource
	horizonSource    HorizonSource
	magnitudeSource  MagnitudeSource
	ackSource        AckSource
}

// NewEngine creates a new Engine with the given sources.
func NewEngine(
	clk Clock,
	pressureSource PressureSource,
	envelopeSource EnvelopeSource,
	windowSource TimeWindowSource,
	vendorCapSource VendorCapSource,
	semanticsSource SemanticsSource,
	trustSource TrustSource,
	policySource PolicySource,
	circleTypeSource CircleTypeSource,
	horizonSource HorizonSource,
	magnitudeSource MagnitudeSource,
	ackSource AckSource,
) *Engine {
	return &Engine{
		clk:              clk,
		pressureSource:   pressureSource,
		envelopeSource:   envelopeSource,
		windowSource:     windowSource,
		vendorCapSource:  vendorCapSource,
		semanticsSource:  semanticsSource,
		trustSource:      trustSource,
		policySource:     policySource,
		circleTypeSource: circleTypeSource,
		horizonSource:    horizonSource,
		magnitudeSource:  magnitudeSource,
		ackSource:        ackSource,
	}
}

// BuildInputs builds UrgencyInputs from sources.
func (e *Engine) BuildInputs(circleIDHash string) domain.UrgencyInputs {
	periodKey := ComputePeriodKey(e.clk.Now())

	inputs := domain.UrgencyInputs{
		CircleIDHash:       circleIDHash,
		PeriodKey:          periodKey,
		CircleType:         domain.BucketUnknown,
		HorizonBucket:      domain.HorizonNone,
		MagnitudeBucket:    domain.MagNothing,
		EnvelopeActive:     false,
		WindowSignal:       domain.WindowNone,
		VendorCap:          domain.CapHoldOnly,
		NecessityDeclared:  false,
		TrustFragile:       false,
		InterruptAllowance: domain.AllowanceNone,
	}

	if e.pressureSource != nil {
		inputs.PressureOutcomeKind = e.pressureSource.GetPressureOutcome(circleIDHash)
	}
	if e.circleTypeSource != nil {
		inputs.CircleType = e.circleTypeSource.GetCircleType(circleIDHash)
	}
	if e.horizonSource != nil {
		inputs.HorizonBucket = e.horizonSource.GetHorizonBucket(circleIDHash)
	}
	if e.magnitudeSource != nil {
		inputs.MagnitudeBucket = e.magnitudeSource.GetMagnitudeBucket(circleIDHash)
	}
	if e.envelopeSource != nil {
		inputs.EnvelopeActive = e.envelopeSource.IsEnvelopeActive(circleIDHash)
	}
	if e.windowSource != nil {
		inputs.WindowSignal = e.windowSource.GetWindowSignal(circleIDHash)
	}
	if e.vendorCapSource != nil {
		inputs.VendorCap = e.vendorCapSource.GetVendorCap(circleIDHash)
	}
	if e.semanticsSource != nil {
		inputs.NecessityDeclared = e.semanticsSource.IsNecessityDeclared(circleIDHash)
	}
	if e.trustSource != nil {
		inputs.TrustFragile = e.trustSource.IsTrustFragile(circleIDHash)
	}
	if e.policySource != nil {
		inputs.InterruptAllowance = e.policySource.GetInterruptAllowance(circleIDHash)
	}

	return inputs
}

// ComputeResolution computes the urgency resolution from inputs.
// This implements the deterministic resolution rules.
func (e *Engine) ComputeResolution(in domain.UrgencyInputs) (domain.UrgencyResolution, error) {
	// Validate inputs first
	if err := in.Validate(); err != nil {
		return domain.UrgencyResolution{
			CircleIDHash: in.CircleIDHash,
			PeriodKey:    in.PeriodKey,
			Level:        domain.UrgNone,
			Cap:          domain.CapHoldOnly,
			Reasons:      []domain.UrgencyReasonBucket{domain.ReasonDefaultHold},
			Status:       domain.StatusRejected,
		}, err
	}

	// Start with defaults
	res := resolutionState{
		level:   domain.UrgNone,
		cap:     domain.CapHoldOnly,
		reasons: []domain.UrgencyReasonBucket{domain.ReasonDefaultHold},
		clamped: false,
	}

	// Rule0: Default HOLD
	// Already set above

	// Rule1: Commerce => cap_hold_only always
	if in.CircleType == domain.BucketCommerce {
		res.cap = domain.CapHoldOnly
		res.clamped = true
	}

	// Rule2: Apply VendorCap as min() clamp
	if in.VendorCap.Order() < res.cap.Order() {
		res.cap = in.VendorCap
		res.addReason(domain.ReasonVendorContractCap)
		res.clamped = true
	}

	// Rule3: TrustFragile clamps max to cap_surface_only
	if in.TrustFragile && res.cap.Order() > domain.CapSurfaceOnly.Order() {
		res.cap = domain.CapSurfaceOnly
		res.addReason(domain.ReasonTrustProtection)
		res.clamped = true
	}

	// Rule5: Institution + soon + several => propose cap_surface_only
	if in.CircleType == domain.BucketInstitution &&
		in.HorizonBucket == domain.HorizonSoon &&
		in.MagnitudeBucket == domain.MagSeveral {
		proposedCap := domain.CapSurfaceOnly
		if proposedCap.Order() <= res.cap.Order() {
			// Can only reduce or match, never increase
			res.addReason(domain.ReasonInstitutionDeadline)
			// Propose level increase
			if res.level.Order() < domain.UrgLow.Order() {
				res.level = domain.UrgLow
			}
		}
	}

	// Rule6: Human + now => propose cap_interrupt_candidate_only
	if in.CircleType == domain.BucketHuman && in.HorizonBucket == domain.HorizonNow {
		proposedCap := domain.CapInterruptCandidateOnly
		if proposedCap.Order() <= res.cap.Order() {
			res.addReason(domain.ReasonHumanNow)
			// Propose level increase
			if res.level.Order() < domain.UrgMedium.Order() {
				res.level = domain.UrgMedium
			}
		}
	}

	// Rule4: WindowSignal may raise Level by +1 step max (none->low->medium)
	if in.WindowSignal == domain.WindowActive {
		newLevel := domain.LevelFromOrder(res.level.Order() + 1)
		// But never exceed cap's implied max level
		maxLevelForCap := capToMaxLevel(res.cap)
		if newLevel.Order() <= maxLevelForCap.Order() && newLevel.Order() <= domain.UrgMedium.Order() {
			res.level = newLevel
			res.addReason(domain.ReasonTimeWindow)
		}
	}

	// Rule7: EnvelopeActive allows one-step Level shift up, but never exceed cap
	if in.EnvelopeActive {
		newLevel := domain.LevelFromOrder(res.level.Order() + 1)
		maxLevelForCap := capToMaxLevel(res.cap)
		if newLevel.Order() <= maxLevelForCap.Order() {
			res.level = newLevel
			res.addReason(domain.ReasonEnvelopeActive)
		}
	}

	// Rule8: NecessityDeclared can only reduce (never increase)
	// If false for institution, clamp max surface
	if !in.NecessityDeclared && in.CircleType == domain.BucketInstitution {
		if res.cap.Order() > domain.CapSurfaceOnly.Order() {
			res.cap = domain.CapSurfaceOnly
			res.addReason(domain.ReasonSemanticsNecessity)
			res.clamped = true
		}
	}

	// Final clamp: ensure level doesn't exceed cap
	maxLevelForCap := capToMaxLevel(res.cap)
	if res.level.Order() > maxLevelForCap.Order() {
		res.level = maxLevelForCap
		res.clamped = true
	}

	// Rule9: Sort reasons and keep first 3
	sortedReasons := domain.SortReasons(res.reasons)

	// Rule10: Determine status
	status := domain.StatusOK
	if res.clamped {
		status = domain.StatusClamped
	}

	result := domain.UrgencyResolution{
		CircleIDHash: in.CircleIDHash,
		PeriodKey:    in.PeriodKey,
		Level:        res.level,
		Cap:          res.cap,
		Reasons:      sortedReasons,
		Status:       status,
	}
	result.ResolutionHash = result.ComputeHash()

	return result, nil
}

// resolutionState tracks state during resolution computation.
type resolutionState struct {
	level   domain.UrgencyLevel
	cap     domain.EscalationCap
	reasons []domain.UrgencyReasonBucket
	clamped bool
}

func (s *resolutionState) addReason(r domain.UrgencyReasonBucket) {
	// Avoid duplicates
	for _, existing := range s.reasons {
		if existing == r {
			return
		}
	}
	s.reasons = append(s.reasons, r)
}

// capToMaxLevel returns the max level allowed for a cap.
func capToMaxLevel(cap domain.EscalationCap) domain.UrgencyLevel {
	switch cap {
	case domain.CapHoldOnly:
		return domain.UrgNone
	case domain.CapSurfaceOnly:
		return domain.UrgLow
	case domain.CapInterruptCandidateOnly:
		return domain.UrgHigh
	default:
		return domain.UrgNone
	}
}

// BuildProofPage builds a proof page from a resolution.
func (e *Engine) BuildProofPage(res domain.UrgencyResolution) domain.UrgencyProofPage {
	lines := []string{
		"Resolution computed for this period.",
		"Level: " + levelToDisplayText(res.Level),
		"Cap: " + capToDisplayText(res.Cap),
	}

	if res.Status == domain.StatusClamped {
		lines = append(lines, "Some escalation was reduced by caps.")
	}

	// Add reason descriptions (max 8 lines total)
	for _, reason := range res.Reasons {
		if len(lines) >= 8 {
			break
		}
		lines = append(lines, reasonToDisplayText(reason))
	}

	// Build reason chips
	chips := make([]string, 0, 3)
	for _, reason := range res.Reasons {
		if len(chips) >= 3 {
			break
		}
		chips = append(chips, reasonToChip(reason))
	}

	return domain.UrgencyProofPage{
		Title:       "Urgency Resolution",
		Lines:       lines,
		Level:       res.Level,
		Cap:         res.Cap,
		ReasonChips: chips,
		StatusHash:  res.ResolutionHash,
		PeriodKey:   res.PeriodKey,
	}
}

// BuildCue builds an optional cue from a resolution.
func (e *Engine) BuildCue(res domain.UrgencyResolution, alreadyHasHigherCue bool) domain.UrgencyCue {
	// Single-whisper rule: if a higher priority cue exists, don't show this one
	if alreadyHasHigherCue {
		return domain.UrgencyCue{Available: false}
	}

	// Only show cue if resolution has some activity
	if res.Level == domain.UrgNone && res.Status == domain.StatusOK {
		return domain.UrgencyCue{Available: false}
	}

	return domain.UrgencyCue{
		Available: true,
		Line:      "Resolution is available for review.",
		Priority:  50, // Low priority, below reality/shadow/trust
	}
}

// ShouldShowCue returns whether the cue should be shown.
func (e *Engine) ShouldShowCue(circleIDHash, periodKey, resolutionHash string) bool {
	if e.ackSource == nil {
		return true
	}
	return !e.ackSource.IsDismissed(circleIDHash, periodKey, resolutionHash)
}

// ComputePeriodKey computes the period key from a time.
func ComputePeriodKey(t time.Time) string {
	return t.Format("2006-01")
}

// ============================================================================
// Display Helpers
// ============================================================================

func levelToDisplayText(level domain.UrgencyLevel) string {
	switch level {
	case domain.UrgNone:
		return "None"
	case domain.UrgLow:
		return "Low"
	case domain.UrgMedium:
		return "Medium"
	case domain.UrgHigh:
		return "High"
	default:
		return "Unknown"
	}
}

func capToDisplayText(cap domain.EscalationCap) string {
	switch cap {
	case domain.CapHoldOnly:
		return "Hold Only"
	case domain.CapSurfaceOnly:
		return "Surface Only"
	case domain.CapInterruptCandidateOnly:
		return "Interrupt Candidate"
	default:
		return "Unknown"
	}
}

func reasonToDisplayText(reason domain.UrgencyReasonBucket) string {
	switch reason {
	case domain.ReasonTimeWindow:
		return "Time window signal contributed."
	case domain.ReasonInstitutionDeadline:
		return "Institution deadline considered."
	case domain.ReasonHumanNow:
		return "Human-now signal considered."
	case domain.ReasonTrustProtection:
		return "Trust protection applied."
	case domain.ReasonVendorContractCap:
		return "Vendor contract cap applied."
	case domain.ReasonSemanticsNecessity:
		return "Semantics necessity considered."
	case domain.ReasonEnvelopeActive:
		return "Active envelope contributed."
	case domain.ReasonDefaultHold:
		return "Default hold applied."
	default:
		return "Unknown reason."
	}
}

func reasonToChip(reason domain.UrgencyReasonBucket) string {
	switch reason {
	case domain.ReasonTimeWindow:
		return "window"
	case domain.ReasonInstitutionDeadline:
		return "deadline"
	case domain.ReasonHumanNow:
		return "human-now"
	case domain.ReasonTrustProtection:
		return "trust"
	case domain.ReasonVendorContractCap:
		return "vendor-cap"
	case domain.ReasonSemanticsNecessity:
		return "necessity"
	case domain.ReasonEnvelopeActive:
		return "envelope"
	case domain.ReasonDefaultHold:
		return "default"
	default:
		return "unknown"
	}
}
