// Package timewindow implements the Phase 40 Time-Window Pressure Sources engine.
//
// The engine builds abstract time window signals from calendar, inbox, and
// device inputs. This is OBSERVATION ONLY - no delivery, no execution.
//
// CRITICAL INVARIANTS:
//   - Pure functions. No side effects.
//   - Deterministic: same inputs + clock => same outputs.
//   - No goroutines. Clock injection required.
//   - No network calls.
//   - Max 3 signals per build result.
//   - Max 3 evidence hashes per signal.
//   - Source precedence: calendar > inbox_institution > inbox_human > device_hint
//   - Max ONE signal per CircleType.
//   - Envelope shift bounded to 1 step.
//   - Commerce MUST NOT appear as a source.
//
// Reference: docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md
package timewindow

import (
	"sort"
	"time"

	ae "quantumlife/pkg/domain/attentionenvelope"
	pd "quantumlife/pkg/domain/pressuredecision"
	tw "quantumlife/pkg/domain/timewindow"
)

// Engine builds time window signals from inputs.
// CRITICAL: No side effects. Pure functions. Same inputs => same outputs.
type Engine struct{}

// NewEngine creates a new time window engine.
func NewEngine() *Engine {
	return &Engine{}
}

// BuildSignals builds time window signals from inputs.
// CRITICAL: Deterministic. Same inputs + clock => same signals.
// CRITICAL: Max 3 signals. Max 1 per CircleType.
func (e *Engine) BuildSignals(inputs *tw.TimeWindowInputs, clock time.Time) *tw.TimeWindowBuildResult {
	if inputs == nil {
		return &tw.TimeWindowBuildResult{
			Status:       tw.StatusEmpty,
			CircleIDHash: "",
			PeriodKey:    tw.NewPeriodKey(clock),
		}
	}

	result := &tw.TimeWindowBuildResult{
		CircleIDHash: inputs.CircleIDHash,
		PeriodKey:    inputs.NowBucket,
		InputHash:    inputs.ComputeInputHash(),
	}

	// Collect candidate signals from all sources
	candidates := e.collectCandidates(inputs)

	if len(candidates) == 0 {
		result.Status = tw.StatusEmpty
		result.ResultHash = result.ComputeResultHash()
		return result
	}

	// Apply envelope effect (shift window kind by at most 1 step)
	if inputs.EnvelopeSummary.IsActive {
		candidates = e.applyEnvelopeShift(candidates, inputs.EnvelopeSummary)
	}

	// Select final signals (max 3, max 1 per CircleType, lowest hash wins)
	result.Signals = e.selectFinalSignals(candidates)

	if len(result.Signals) == 0 {
		result.Status = tw.StatusEmpty
	} else {
		result.Status = tw.StatusOK
	}

	result.ResultHash = result.ComputeResultHash()
	return result
}

// collectCandidates gathers candidate signals from all sources.
// CRITICAL: Commerce is excluded. Never appears as a source.
func (e *Engine) collectCandidates(inputs *tw.TimeWindowInputs) []tw.TimeWindowSignal {
	var candidates []tw.TimeWindowSignal

	// Calendar signals (highest precedence)
	if inputs.Calendar.HasUpcoming && inputs.Calendar.UpcomingCountBucket != tw.MagnitudeNothing {
		signal := tw.TimeWindowSignal{
			Source:         tw.SourceCalendar,
			CircleType:     tw.CircleSelf,
			Kind:           inputs.Calendar.NextStartsIn,
			Reason:         tw.ReasonAppointment,
			Magnitude:      inputs.Calendar.UpcomingCountBucket,
			EvidenceHashes: e.capEvidenceHashes(inputs.Calendar.EvidenceHashes),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		candidates = append(candidates, signal)
	}

	// Inbox institution signals
	if inputs.Inbox.InstitutionalCountBucket != tw.MagnitudeNothing {
		signal := tw.TimeWindowSignal{
			Source:         tw.SourceInboxInstitution,
			CircleType:     tw.CircleInstitution,
			Kind:           inputs.Inbox.InstitutionWindowKind,
			Reason:         tw.ReasonWaiting, // Institution waiting for response
			Magnitude:      inputs.Inbox.InstitutionalCountBucket,
			EvidenceHashes: e.capEvidenceHashes(inputs.Inbox.EvidenceHashes),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		candidates = append(candidates, signal)
	}

	// Inbox human signals
	if inputs.Inbox.HumanCountBucket != tw.MagnitudeNothing {
		signal := tw.TimeWindowSignal{
			Source:         tw.SourceInboxHuman,
			CircleType:     tw.CircleHuman,
			Kind:           inputs.Inbox.HumanWindowKind,
			Reason:         tw.ReasonWaiting, // Human waiting for response
			Magnitude:      inputs.Inbox.HumanCountBucket,
			EvidenceHashes: e.capEvidenceHashes(inputs.Inbox.EvidenceHashes),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		candidates = append(candidates, signal)
	}

	// Device hint signals
	if inputs.DeviceHints.TransportSignals != tw.MagnitudeNothing {
		signal := tw.TimeWindowSignal{
			Source:         tw.SourceDeviceHint,
			CircleType:     tw.CircleSelf,
			Kind:           tw.WindowSoon, // Transport hints are typically soon
			Reason:         tw.ReasonTravel,
			Magnitude:      inputs.DeviceHints.TransportSignals,
			EvidenceHashes: e.capEvidenceHashes(inputs.DeviceHints.EvidenceHashes),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		candidates = append(candidates, signal)
	}

	if inputs.DeviceHints.HealthSignals != tw.MagnitudeNothing {
		signal := tw.TimeWindowSignal{
			Source:         tw.SourceDeviceHint,
			CircleType:     tw.CircleSelf,
			Kind:           tw.WindowNow, // Health signals are typically now
			Reason:         tw.ReasonHealth,
			Magnitude:      inputs.DeviceHints.HealthSignals,
			EvidenceHashes: e.capEvidenceHashes(inputs.DeviceHints.EvidenceHashes),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		candidates = append(candidates, signal)
	}

	if inputs.DeviceHints.InstitutionSignals != tw.MagnitudeNothing {
		signal := tw.TimeWindowSignal{
			Source:         tw.SourceDeviceHint,
			CircleType:     tw.CircleInstitution,
			Kind:           tw.WindowSoon,
			Reason:         tw.ReasonUnknown,
			Magnitude:      inputs.DeviceHints.InstitutionSignals,
			EvidenceHashes: e.capEvidenceHashes(inputs.DeviceHints.EvidenceHashes),
		}
		signal.StatusHash = signal.ComputeStatusHash()
		candidates = append(candidates, signal)
	}

	return candidates
}

// applyEnvelopeShift shifts window kind by at most 1 step based on envelope.
// CRITICAL: Max 1 step. Envelope may NOT create new windows.
func (e *Engine) applyEnvelopeShift(candidates []tw.TimeWindowSignal, summary tw.AttentionEnvelopeSummary) []tw.TimeWindowSignal {
	if !summary.IsActive {
		return candidates
	}

	// Only certain envelope kinds shift windows
	shouldShift := false
	switch summary.Kind {
	case ae.EnvelopeKindOnCall, ae.EnvelopeKindTravel, ae.EnvelopeKindEmergency:
		shouldShift = true
	}

	if !shouldShift {
		return candidates
	}

	result := make([]tw.TimeWindowSignal, len(candidates))
	for i, c := range candidates {
		shifted := c
		shifted.Kind = c.Kind.ShiftEarlier()
		shifted.StatusHash = shifted.ComputeStatusHash()
		result[i] = shifted
	}

	return result
}

// selectFinalSignals selects up to MaxSignals, max 1 per CircleType.
// CRITICAL: If >3 candidates, select lowest StatusHash deterministically.
func (e *Engine) selectFinalSignals(candidates []tw.TimeWindowSignal) []tw.TimeWindowSignal {
	if len(candidates) == 0 {
		return nil
	}

	// Sort by source precedence first, then by status hash
	sort.SliceStable(candidates, func(i, j int) bool {
		// First by source precedence (lower = higher priority)
		pi := candidates[i].Source.Precedence()
		pj := candidates[j].Source.Precedence()
		if pi != pj {
			return pi < pj
		}
		// Then by status hash (deterministic tie-breaker)
		return candidates[i].StatusHash < candidates[j].StatusHash
	})

	// Select max 1 per CircleType
	seenCircleTypes := make(map[tw.WindowCircleType]bool)
	var selected []tw.TimeWindowSignal

	for _, c := range candidates {
		if len(selected) >= tw.MaxSignals {
			break
		}
		if seenCircleTypes[c.CircleType] {
			continue
		}
		seenCircleTypes[c.CircleType] = true
		selected = append(selected, c)
	}

	return selected
}

// capEvidenceHashes caps evidence hashes to MaxEvidenceHashes.
func (e *Engine) capEvidenceHashes(hashes []string) []string {
	if len(hashes) <= tw.MaxEvidenceHashes {
		// Return a copy to avoid mutation
		result := make([]string, len(hashes))
		copy(result, hashes)
		return result
	}

	// Sort for determinism, then take first MaxEvidenceHashes
	sorted := make([]string, len(hashes))
	copy(sorted, hashes)
	sort.Strings(sorted)

	return sorted[:tw.MaxEvidenceHashes]
}

// BuildProofPage builds a proof page for display.
// CRITICAL: Contains only hashes and buckets. No raw data.
func (e *Engine) BuildProofPage(circleIDHash string, result *tw.TimeWindowBuildResult) *tw.WindowsProofPage {
	return tw.BuildWindowsProofPage(circleIDHash, result)
}

// ValidateInputs validates time window inputs.
func (e *Engine) ValidateInputs(inputs *tw.TimeWindowInputs) error {
	if inputs == nil {
		return nil
	}
	if inputs.CircleIDHash == "" {
		return nil // Empty inputs are valid
	}
	if err := inputs.Calendar.UpcomingCountBucket.Validate(); err != nil {
		return err
	}
	if inputs.Calendar.HasUpcoming {
		if err := inputs.Calendar.NextStartsIn.Validate(); err != nil {
			return err
		}
	}
	if err := inputs.Inbox.InstitutionalCountBucket.Validate(); err != nil {
		return err
	}
	if err := inputs.Inbox.HumanCountBucket.Validate(); err != nil {
		return err
	}
	if err := inputs.DeviceHints.TransportSignals.Validate(); err != nil {
		return err
	}
	if err := inputs.DeviceHints.HealthSignals.Validate(); err != nil {
		return err
	}
	if err := inputs.DeviceHints.InstitutionSignals.Validate(); err != nil {
		return err
	}
	return nil
}

// SignalToPressureInput converts a TimeWindowSignal to a PressureDecisionInput.
// CRITICAL: No new enums downstream. Maps to existing pressure types.
// CRITICAL: Commerce NEVER escalates (not applicable - commerce excluded from sources).
func (e *Engine) SignalToPressureInput(signal *tw.TimeWindowSignal, periodKey string) *pd.PressureDecisionInput {
	if signal == nil {
		return nil
	}

	input := &pd.PressureDecisionInput{
		CircleIDHash: "", // Caller must set
		PeriodKey:    periodKey,
	}

	// Map CircleType
	input.CircleType = e.mapCircleType(signal.CircleType)

	// Map Magnitude
	input.Magnitude = e.mapMagnitude(signal.Magnitude)

	// Map WindowKind -> PressureHorizon
	input.Horizon = e.mapWindowKindToHorizon(signal.Kind)

	// Default trust status
	input.TrustStatus = pd.TrustStatusUnknown

	return input
}

// mapCircleType maps WindowCircleType to pressuredecision.CircleType.
func (e *Engine) mapCircleType(ct tw.WindowCircleType) pd.CircleType {
	switch ct {
	case tw.CircleHuman:
		return pd.CircleTypeHuman
	case tw.CircleInstitution:
		return pd.CircleTypeInstitution
	case tw.CircleSelf:
		return pd.CircleTypeHuman // Self maps to human for pressure decisions
	default:
		return pd.CircleTypeHuman
	}
}

// mapMagnitude maps WindowMagnitudeBucket to pressuredecision.PressureMagnitude.
func (e *Engine) mapMagnitude(m tw.WindowMagnitudeBucket) pd.PressureMagnitude {
	switch m {
	case tw.MagnitudeNothing:
		return pd.MagnitudeNothing
	case tw.MagnitudeAFew:
		return pd.MagnitudeAFew
	case tw.MagnitudeSeveral:
		return pd.MagnitudeSeveral
	default:
		return pd.MagnitudeNothing
	}
}

// mapWindowKindToHorizon maps WindowKind to pressuredecision.PressureHorizon.
func (e *Engine) mapWindowKindToHorizon(k tw.WindowKind) pd.PressureHorizon {
	switch k {
	case tw.WindowNow:
		return pd.HorizonNow
	case tw.WindowSoon:
		return pd.HorizonSoon
	case tw.WindowToday:
		return pd.HorizonLater // Today maps to Later
	case tw.WindowLater:
		return pd.HorizonLater
	default:
		return pd.HorizonUnknown
	}
}

// GetCalmWhisperCue returns the optional whisper cue for windows.
// CRITICAL: Lowest priority. Must not add second whisper.
func (e *Engine) GetCalmWhisperCue(result *tw.TimeWindowBuildResult) string {
	if result == nil || len(result.Signals) == 0 {
		return ""
	}
	return "A window is coming up. Quietly held."
}
