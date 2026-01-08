// Package devicereg provides the engine for Phase 37: Device Registration + Deep-Link.
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only
// - No goroutines
// - Pure functions (no side effects)
// - Deterministic: same inputs => same outputs
//
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
package devicereg

import (
	"quantumlife/pkg/domain/devicereg"
)

// Engine handles device registration logic.
// Pure, deterministic, no side effects.
type Engine struct{}

// NewEngine creates a new device registration engine.
func NewEngine() *Engine {
	return &Engine{}
}

// BuildRegistrationReceipt creates a registration receipt from inputs.
// CRITICAL: Does NOT seal the token. That must happen in the handler.
func (e *Engine) BuildRegistrationReceipt(
	periodKey string,
	platform devicereg.DevicePlatform,
	circleIDHash string,
	tokenHash string,
	sealedRefHash string,
) *devicereg.DeviceRegistrationReceipt {
	receipt := &devicereg.DeviceRegistrationReceipt{
		PeriodKey:     periodKey,
		Platform:      platform,
		CircleIDHash:  circleIDHash,
		TokenHash:     tokenHash,
		SealedRefHash: sealedRefHash,
		State:         devicereg.DeviceRegStateRegistered,
	}

	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()

	return receipt
}

// BuildProofPage builds the proof page for a registration receipt.
// Returns a default page if receipt is nil.
func (e *Engine) BuildProofPage(receipt *devicereg.DeviceRegistrationReceipt) *devicereg.DeviceRegistrationProofPage {
	if receipt == nil {
		page := &devicereg.DeviceRegistrationProofPage{
			Title: "Device, quietly.",
			Lines: []string{
				"No device registered yet.",
				"When you're ready, you can register one.",
			},
			HasRegistration:  false,
			DismissAvailable: false,
		}
		page.StatusHash = page.ComputeStatusHash()
		return page
	}

	page := &devicereg.DeviceRegistrationProofPage{
		Title: "Sealed, quietly.",
		Lines: []string{
			"A device token was sealed.",
			"We can notify only when you permit it.",
			"Nothing else was stored.",
		},
		TokenHashPrefix:  receipt.TokenHashPrefix(),
		StatusHashPrefix: receipt.StatusHashPrefix(),
		HasRegistration:  true,
		Platform:         receipt.Platform,
		DismissAvailable: true,
	}
	page.StatusHash = page.ComputeStatusHash()

	return page
}

// BuildDevicesPage builds the devices overview page.
func (e *Engine) BuildDevicesPage(hasIOSRegistration bool, registrationCount int) *devicereg.DevicesPage {
	page := &devicereg.DevicesPage{
		Title:                 "Device, quietly.",
		HasiOSRegistration:    hasIOSRegistration,
		RegistrationMagnitude: devicereg.MagnitudeFromCount(registrationCount),
	}
	page.StatusHash = page.ComputeStatusHash()

	return page
}

// ComputeDeepLinkTarget computes the target screen for a deep link.
// Deterministic priority chain:
//  1. Interrupt preview available AND not acked => interrupts
//  2. Trust action receipt available AND not dismissed => trust
//  3. Shadow receipt primary cue available AND not dismissed => shadow
//  4. Reality cue available => reality
//  5. else => today
func (e *Engine) ComputeDeepLinkTarget(input *devicereg.DeepLinkComputeInput) devicereg.DeepLinkTarget {
	if input == nil {
		return devicereg.DeepLinkTargetToday
	}

	// Priority 1: Interrupt preview
	if input.InterruptPreviewAvailable && !input.InterruptPreviewAcked {
		return devicereg.DeepLinkTargetInterrupts
	}

	// Priority 2: Trust action receipt
	if input.TrustActionReceiptAvailable && !input.TrustActionReceiptDismissed {
		return devicereg.DeepLinkTargetTrust
	}

	// Priority 3: Shadow receipt cue
	if input.ShadowReceiptCueAvailable && !input.ShadowReceiptCueDismissed {
		return devicereg.DeepLinkTargetShadow
	}

	// Priority 4: Reality cue
	if input.RealityCueAvailable {
		return devicereg.DeepLinkTargetReality
	}

	// Default: Today
	return devicereg.DeepLinkTargetToday
}

// ShouldShowDeviceRegCue determines if the device registration cue should show.
// Only shows if:
// - Connected mode (not demo)
// - No device registered for the circle
// - (Caller must check single whisper rule priority)
func (e *Engine) ShouldShowDeviceRegCue(isConnectedMode bool, hasDeviceRegistered bool) bool {
	return isConnectedMode && !hasDeviceRegistered
}

// BuildDeviceRegCue builds the device registration whisper cue.
func (e *Engine) BuildDeviceRegCue(show bool) *devicereg.DeviceRegisterCue {
	return &devicereg.DeviceRegisterCue{
		Available: show,
		Text:      devicereg.DefaultDeviceRegCueText,
		LinkPath:  devicereg.DefaultDeviceRegCuePath,
		Priority:  devicereg.DefaultDeviceRegCuePriority,
	}
}
