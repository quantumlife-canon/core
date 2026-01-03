// Package mode provides system mode derivation for Phase 21.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
//
// CRITICAL INVARIANTS:
//   - Mode is derived from existing state, not stored
//   - No goroutines. No time.Now().
//   - Stdlib only.
//   - Deterministic: same inputs => same mode
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
package mode

import (
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

// Engine derives the current system mode from state.
//
// CRITICAL: Engine does NOT store state.
// CRITICAL: Engine does NOT spawn goroutines.
// CRITICAL: Engine uses clock injection for determinism.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new mode derivation engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// DeriveModeInput contains the inputs needed to derive the mode.
type DeriveModeInput struct {
	// HasGmailConnection indicates if Gmail OAuth token exists and is valid.
	HasGmailConnection bool

	// ShadowProviderIsStub indicates if the shadow provider is stub.
	ShadowProviderIsStub bool

	// ShadowRealAllowed indicates if real shadow providers are allowed.
	ShadowRealAllowed bool

	// LatestShadowReceipt is the most recent shadow receipt, if any.
	LatestShadowReceipt *shadowllm.ShadowReceipt

	// CircleID is the current circle being evaluated.
	CircleID identity.EntityID
}

// DeriveMode determines the system mode from current state.
//
// Mode derivation rules (deterministic):
//
// Demo:
//   - No Gmail connection
//   - OR shadow provider is stub AND real not allowed
//
// Connected:
//   - Gmail connected
//   - AND no shadow receipt for current period
//
// Shadow:
//   - Shadow receipt exists for current period
//
// CRITICAL: This is read-only derivation, not configuration.
func (e *Engine) DeriveMode(input DeriveModeInput) Mode {
	now := e.clock()
	currentPeriod := now.UTC().Format("2006-01-02")

	// Rule 1: Demo mode if no connection OR shadow is stub-only
	if !input.HasGmailConnection {
		return ModeDemo
	}

	// At this point, Gmail is connected
	// Check if we have a shadow receipt for current period
	if input.LatestShadowReceipt != nil {
		receiptPeriod := input.LatestShadowReceipt.CreatedAt.UTC().Format("2006-01-02")
		if receiptPeriod == currentPeriod {
			return ModeShadow
		}
	}

	// Gmail connected but no shadow receipt for current period
	// Additional check: if shadow is stub-only, still consider it demo-like
	if input.ShadowProviderIsStub && !input.ShadowRealAllowed {
		return ModeDemo
	}

	return ModeConnected
}

// DeriveModeIndicator derives the mode and returns a full indicator.
func (e *Engine) DeriveModeIndicator(input DeriveModeInput) ModeIndicator {
	mode := e.DeriveMode(input)
	return NewModeIndicator(mode)
}
