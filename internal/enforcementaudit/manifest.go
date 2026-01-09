// Package enforcementaudit provides the enforcement wiring audit engine.
//
// This file contains the EnforcementManifest which tracks which enforcement
// wrappers are wired in the runtime.
//
// Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md
package enforcementaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ============================================================================
// Enforcement Manifest
// ============================================================================

// EnforcementManifest tracks which enforcement wrappers are wired.
// This is built from the actual handlers in cmd/quantumlife-web/main.go
// using literal bools, no magic.
type EnforcementManifest struct {
	// Pipeline enforcement
	PressureGateApplied            bool
	DelegatedHoldingApplied        bool
	TrustTransferApplied           bool
	InterruptPreviewApplied        bool
	DeliveryOrchestratorUsesClamp  bool
	TimeWindowAdapterApplied       bool

	// Safety invariants
	CommerceExcluded               bool
	EnvelopeCannotOverride         bool
	InterruptPolicyCannotOverride  bool

	// Clamp wrapper
	ClampWrapperRegistered         bool

	// Version
	Version                        string // e.g., "v1.0.0"
}

// Validate validates the manifest.
func (m *EnforcementManifest) Validate() error {
	if m.Version == "" {
		return fmt.Errorf("version required")
	}
	return nil
}

// CanonicalString returns a deterministic canonical string representation.
func (m *EnforcementManifest) CanonicalString() string {
	return fmt.Sprintf("v1|%t|%t|%t|%t|%t|%t|%t|%t|%t|%t|%s",
		m.PressureGateApplied,
		m.DelegatedHoldingApplied,
		m.TrustTransferApplied,
		m.InterruptPreviewApplied,
		m.DeliveryOrchestratorUsesClamp,
		m.TimeWindowAdapterApplied,
		m.CommerceExcluded,
		m.EnvelopeCannotOverride,
		m.InterruptPolicyCannotOverride,
		m.ClampWrapperRegistered,
		m.Version,
	)
}

// ComputeHash computes SHA256 of the canonical string.
func (m *EnforcementManifest) ComputeHash() string {
	h := sha256.Sum256([]byte(m.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// IsComplete checks if all required enforcement wrappers are wired.
func (m *EnforcementManifest) IsComplete() bool {
	return m.PressureGateApplied &&
		m.DelegatedHoldingApplied &&
		m.TrustTransferApplied &&
		m.InterruptPreviewApplied &&
		m.DeliveryOrchestratorUsesClamp &&
		m.TimeWindowAdapterApplied &&
		m.CommerceExcluded &&
		m.EnvelopeCannotOverride &&
		m.InterruptPolicyCannotOverride &&
		m.ClampWrapperRegistered
}

// MissingComponents returns a list of components that are not wired.
func (m *EnforcementManifest) MissingComponents() []string {
	var missing []string

	if !m.PressureGateApplied {
		missing = append(missing, "pressure_gate")
	}
	if !m.DelegatedHoldingApplied {
		missing = append(missing, "delegated_holding")
	}
	if !m.TrustTransferApplied {
		missing = append(missing, "trust_transfer")
	}
	if !m.InterruptPreviewApplied {
		missing = append(missing, "interrupt_preview")
	}
	if !m.DeliveryOrchestratorUsesClamp {
		missing = append(missing, "delivery_orchestrator")
	}
	if !m.TimeWindowAdapterApplied {
		missing = append(missing, "timewindow_adapter")
	}
	if !m.CommerceExcluded {
		missing = append(missing, "commerce_filter")
	}
	if !m.EnvelopeCannotOverride {
		missing = append(missing, "envelope_adapter")
	}
	if !m.InterruptPolicyCannotOverride {
		missing = append(missing, "interrupt_policy")
	}
	if !m.ClampWrapperRegistered {
		missing = append(missing, "clamp_wrapper")
	}

	return missing
}

// ============================================================================
// Manifest Builder
// ============================================================================

// ManifestBuilder helps construct an EnforcementManifest.
type ManifestBuilder struct {
	manifest EnforcementManifest
}

// NewManifestBuilder creates a new manifest builder.
func NewManifestBuilder() *ManifestBuilder {
	return &ManifestBuilder{
		manifest: EnforcementManifest{
			Version: "v1.0.0",
		},
	}
}

// WithPressureGate marks pressure gate as applied.
func (b *ManifestBuilder) WithPressureGate() *ManifestBuilder {
	b.manifest.PressureGateApplied = true
	return b
}

// WithDelegatedHolding marks delegated holding as applied.
func (b *ManifestBuilder) WithDelegatedHolding() *ManifestBuilder {
	b.manifest.DelegatedHoldingApplied = true
	return b
}

// WithTrustTransfer marks trust transfer as applied.
func (b *ManifestBuilder) WithTrustTransfer() *ManifestBuilder {
	b.manifest.TrustTransferApplied = true
	return b
}

// WithInterruptPreview marks interrupt preview as applied.
func (b *ManifestBuilder) WithInterruptPreview() *ManifestBuilder {
	b.manifest.InterruptPreviewApplied = true
	return b
}

// WithDeliveryOrchestrator marks delivery orchestrator as using clamp.
func (b *ManifestBuilder) WithDeliveryOrchestrator() *ManifestBuilder {
	b.manifest.DeliveryOrchestratorUsesClamp = true
	return b
}

// WithTimeWindowAdapter marks time window adapter as applied.
func (b *ManifestBuilder) WithTimeWindowAdapter() *ManifestBuilder {
	b.manifest.TimeWindowAdapterApplied = true
	return b
}

// WithCommerceExcluded marks commerce as excluded.
func (b *ManifestBuilder) WithCommerceExcluded() *ManifestBuilder {
	b.manifest.CommerceExcluded = true
	return b
}

// WithEnvelopeCannotOverride marks envelope as cannot override.
func (b *ManifestBuilder) WithEnvelopeCannotOverride() *ManifestBuilder {
	b.manifest.EnvelopeCannotOverride = true
	return b
}

// WithInterruptPolicyCannotOverride marks interrupt policy as cannot override.
func (b *ManifestBuilder) WithInterruptPolicyCannotOverride() *ManifestBuilder {
	b.manifest.InterruptPolicyCannotOverride = true
	return b
}

// WithClampWrapper marks clamp wrapper as registered.
func (b *ManifestBuilder) WithClampWrapper() *ManifestBuilder {
	b.manifest.ClampWrapperRegistered = true
	return b
}

// Build returns the completed manifest.
func (b *ManifestBuilder) Build() EnforcementManifest {
	return b.manifest
}

// BuildComplete builds a complete manifest with all components wired.
func BuildCompleteManifest() EnforcementManifest {
	return NewManifestBuilder().
		WithPressureGate().
		WithDelegatedHolding().
		WithTrustTransfer().
		WithInterruptPreview().
		WithDeliveryOrchestrator().
		WithTimeWindowAdapter().
		WithCommerceExcluded().
		WithEnvelopeCannotOverride().
		WithInterruptPolicyCannotOverride().
		WithClampWrapper().
		Build()
}
