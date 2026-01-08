// Package notificationobserver provides the engine for Phase 38: Mobile Notification Metadata Observer.
//
// This engine observes notification metadata and produces abstract pressure signals.
// It DOES NOT read notification content, identify apps, or trigger any actions.
//
// CRITICAL INVARIANTS:
//   - Pure functions (no side effects)
//   - No time.Now() - clock injection only
//   - No goroutines
//   - No network calls
//   - No decision logic - observation ONLY
//   - Deterministic: same inputs => same outputs
//   - stdlib only
//
// Reference: docs/ADR/ADR-0075-phase38-notification-metadata-observer.md
package notificationobserver

import (
	"quantumlife/pkg/domain/externalpressure"
	"quantumlife/pkg/domain/notificationobserver"
)

// Engine handles notification metadata observation.
// Pure, deterministic, no side effects.
type Engine struct{}

// NewEngine creates a new notification observer engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ObserveNotificationMetadata processes notification metadata and produces a pressure signal.
// Returns nil if:
//   - Input is nil
//   - Input is invalid
//   - Magnitude is "nothing" (no meaningful pressure)
//
// CRITICAL: Does NOT read content, does NOT identify apps, does NOT trigger actions.
func (e *Engine) ObserveNotificationMetadata(input *notificationobserver.NotificationObserverInput) *notificationobserver.NotificationPressureSignal {
	if input == nil {
		return nil
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil
	}

	// No signal when magnitude is "nothing"
	if input.Magnitude == notificationobserver.MagnitudeNothing {
		return nil
	}

	// Build the pressure signal
	signal := &notificationobserver.NotificationPressureSignal{
		Source:    notificationobserver.SourceMobileOS,
		AppClass:  input.AppClass,
		Magnitude: input.Magnitude,
		Horizon:   input.Horizon,
		PeriodKey: input.PeriodKey,
	}

	// Compute evidence hash
	signal.EvidenceHash = input.ComputeEvidenceHash()

	// Compute status hash
	signal.StatusHash = signal.ComputeStatusHash()

	// Compute signal ID (ensures max 1 per app class per period)
	signal.SignalID = signal.ComputeSignalID()

	return signal
}

// ShouldHold determines if the signal should be held (not surfaced).
// Default is HOLD - we observe without surfacing.
//
// Returns true (HOLD) unless:
//   - Signal is nil (nothing to hold)
//   - All other cases: HOLD is the default
func (e *Engine) ShouldHold(signal *notificationobserver.NotificationPressureSignal) bool {
	if signal == nil {
		return false // Nothing to hold
	}
	// Default: HOLD everything
	// The signal exists but is not surfaced by default
	return true
}

// ConvertToPressureInput converts a notification signal to external pressure input.
// This bridges Phase 38 signals into Phase 31.4 pressure pipeline.
//
// CRITICAL: This only creates pipeline input - it does NOT affect any decisions.
func (e *Engine) ConvertToPressureInput(
	signal *notificationobserver.NotificationPressureSignal,
	sovereignCircleIDHash string,
) *externalpressure.ObservationInput {
	if signal == nil {
		return nil
	}

	if sovereignCircleIDHash == "" {
		return nil
	}

	// Map app class to pressure category
	category := mapAppClassToPressureCategory(signal.AppClass)

	return &externalpressure.ObservationInput{
		Source:       externalpressure.SourceGmailReceipt, // Using existing source for now
		Category:     category,
		EvidenceHash: signal.EvidenceHash,
	}
}

// mapAppClassToPressureCategory maps notification app class to pressure category.
// This is the bridge between Phase 38 and Phase 31.4.
func mapAppClassToPressureCategory(appClass notificationobserver.NotificationAppClass) externalpressure.PressureCategory {
	switch appClass {
	case notificationobserver.AppClassTransport:
		return externalpressure.PressureCategoryTransport
	case notificationobserver.AppClassHealth:
		return externalpressure.PressureCategoryOther // Health maps to Other (no direct mapping)
	case notificationobserver.AppClassInstitution:
		return externalpressure.PressureCategoryOther // Institution maps to Other
	case notificationobserver.AppClassCommerce:
		return externalpressure.PressureCategoryDelivery // Commerce often means delivery
	case notificationobserver.AppClassUnknown:
		return externalpressure.PressureCategoryOther
	default:
		return externalpressure.PressureCategoryOther
	}
}

// ValidateInput validates observer input and rejects forbidden content.
// Returns error if input contains identifiable information.
func (e *Engine) ValidateInput(input *notificationobserver.NotificationObserverInput) error {
	if input == nil {
		return nil
	}
	return input.Validate()
}

// IsValidAppClass checks if a string is a valid app class.
func (e *Engine) IsValidAppClass(appClass string) bool {
	ac := notificationobserver.NotificationAppClass(appClass)
	return ac.Validate() == nil
}

// IsValidMagnitude checks if a string is a valid magnitude bucket.
func (e *Engine) IsValidMagnitude(magnitude string) bool {
	m := notificationobserver.MagnitudeBucket(magnitude)
	return m.Validate() == nil
}

// IsValidHorizon checks if a string is a valid horizon bucket.
func (e *Engine) IsValidHorizon(horizon string) bool {
	h := notificationobserver.HorizonBucket(horizon)
	return h.Validate() == nil
}

// BuildInputFromParams builds observer input from validated parameters.
// Returns nil if any parameter is invalid.
func (e *Engine) BuildInputFromParams(
	appClass string,
	magnitude string,
	horizon string,
	periodKey string,
) *notificationobserver.NotificationObserverInput {
	ac := notificationobserver.NotificationAppClass(appClass)
	if ac.Validate() != nil {
		return nil
	}

	m := notificationobserver.MagnitudeBucket(magnitude)
	if m.Validate() != nil {
		return nil
	}

	h := notificationobserver.HorizonBucket(horizon)
	if h.Validate() != nil {
		return nil
	}

	if periodKey == "" {
		return nil
	}

	return &notificationobserver.NotificationObserverInput{
		AppClass:  ac,
		Magnitude: m,
		Horizon:   h,
		PeriodKey: periodKey,
	}
}

// ComputeSignalKey computes the unique key for deduplication.
// Format: source|app_class|period_key
// Ensures max 1 signal per app class per period.
func (e *Engine) ComputeSignalKey(signal *notificationobserver.NotificationPressureSignal) string {
	if signal == nil {
		return ""
	}
	return signal.ComputeSignalID()
}

// MergeSignals merges multiple signals for the same app class in the same period.
// Returns the signal with the higher magnitude/urgency.
func (e *Engine) MergeSignals(
	existing *notificationobserver.NotificationPressureSignal,
	new *notificationobserver.NotificationPressureSignal,
) *notificationobserver.NotificationPressureSignal {
	if existing == nil {
		return new
	}
	if new == nil {
		return existing
	}

	// Compare magnitudes (several > a_few)
	existingMag := magnitudeRank(existing.Magnitude)
	newMag := magnitudeRank(new.Magnitude)

	// Compare horizons (now > soon > later)
	existingHor := horizonRank(existing.Horizon)
	newHor := horizonRank(new.Horizon)

	// Prefer higher magnitude, then more urgent horizon
	if newMag > existingMag {
		return new
	}
	if newMag == existingMag && newHor > existingHor {
		return new
	}

	return existing
}

// magnitudeRank returns a numeric rank for magnitude comparison.
func magnitudeRank(m notificationobserver.MagnitudeBucket) int {
	switch m {
	case notificationobserver.MagnitudeSeveral:
		return 2
	case notificationobserver.MagnitudeAFew:
		return 1
	default:
		return 0
	}
}

// horizonRank returns a numeric rank for horizon comparison.
func horizonRank(h notificationobserver.HorizonBucket) int {
	switch h {
	case notificationobserver.HorizonNow:
		return 2
	case notificationobserver.HorizonSoon:
		return 1
	default:
		return 0
	}
}
