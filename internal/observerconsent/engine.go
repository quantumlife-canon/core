// Package observerconsent provides the engine for Phase 55: Observer Consent Activation UI.
//
// This package provides pure, deterministic logic for processing observer consent
// requests and building proof pages.
//
// CRITICAL: No time.Now() in this package - clock must be injected.
// CRITICAL: No goroutines in this package.
// CRITICAL: No I/O in this package - stores are injected.
//
// Reference: docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
package observerconsent

import (
	"fmt"

	domaincoverageplan "quantumlife/pkg/domain/coverageplan"
	domain "quantumlife/pkg/domain/observerconsent"
)

// Engine provides observer consent processing logic.
// All methods are pure and deterministic - no I/O, no time.Now(), no goroutines.
type Engine struct {
	periodKeyFunc func() string // Returns current period key (YYYY-MM-DD)
}

// NewEngine creates a new observer consent engine.
func NewEngine(periodKeyFunc func() string) *Engine {
	return &Engine{
		periodKeyFunc: periodKeyFunc,
	}
}

// ApplyConsentInput contains the inputs for applying a consent request.
type ApplyConsentInput struct {
	Request     domain.ObserverConsentRequest
	CurrentCaps []domaincoverageplan.CoverageCapability
}

// ApplyConsentOutput contains the outputs from applying a consent request.
type ApplyConsentOutput struct {
	NewCaps []domaincoverageplan.CoverageCapability
	Receipt domain.ObserverConsentReceipt
}

// ApplyConsent processes a consent request and returns the new capability set and receipt.
// This is pure logic - no I/O.
func (e *Engine) ApplyConsent(input ApplyConsentInput) ApplyConsentOutput {
	periodKey := e.periodKeyFunc()
	req := input.Request

	// Build base receipt
	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    periodKey,
		CircleIDHash: req.CircleIDHash,
		Action:       req.Action,
		Capability:   req.Capability,
		Kind:         domain.KindFromCapability(req.Capability),
		Result:       domain.ResultRejected,
		RejectReason: domain.RejectNone,
	}

	// Validate circle ID
	if req.CircleIDHash == "" {
		receipt.RejectReason = domain.RejectMissingCircle
		receipt.ReceiptHash = receipt.ComputeReceiptHash()
		return ApplyConsentOutput{
			NewCaps: input.CurrentCaps,
			Receipt: receipt,
		}
	}

	// Validate action
	if err := req.Action.Validate(); err != nil {
		receipt.RejectReason = domain.RejectInvalid
		receipt.ReceiptHash = receipt.ComputeReceiptHash()
		return ApplyConsentOutput{
			NewCaps: input.CurrentCaps,
			Receipt: receipt,
		}
	}

	// Check allowlist
	if !domain.IsAllowlisted(req.Capability) {
		receipt.RejectReason = domain.RejectNotAllowlisted
		receipt.ReceiptHash = receipt.ComputeReceiptHash()
		return ApplyConsentOutput{
			NewCaps: input.CurrentCaps,
			Receipt: receipt,
		}
	}

	// Check current state
	currentlyEnabled := e.hasCapability(input.CurrentCaps, req.Capability)

	// Apply action
	var newCaps []domaincoverageplan.CoverageCapability
	switch req.Action {
	case domain.ActionEnable:
		if currentlyEnabled {
			// Already enabled - no change
			receipt.Result = domain.ResultNoChange
			receipt.ReceiptHash = receipt.ComputeReceiptHash()
			return ApplyConsentOutput{
				NewCaps: input.CurrentCaps,
				Receipt: receipt,
			}
		}
		// Add capability
		newCaps = append(input.CurrentCaps, req.Capability)
		newCaps = domain.NormalizeCapabilities(newCaps)
		receipt.Result = domain.ResultApplied

	case domain.ActionDisable:
		if !currentlyEnabled {
			// Already disabled - no change
			receipt.Result = domain.ResultNoChange
			receipt.ReceiptHash = receipt.ComputeReceiptHash()
			return ApplyConsentOutput{
				NewCaps: input.CurrentCaps,
				Receipt: receipt,
			}
		}
		// Remove capability
		newCaps = e.removeCapability(input.CurrentCaps, req.Capability)
		receipt.Result = domain.ResultApplied
	}

	receipt.ReceiptHash = receipt.ComputeReceiptHash()
	return ApplyConsentOutput{
		NewCaps: newCaps,
		Receipt: receipt,
	}
}

// hasCapability checks if a capability is in the list.
func (e *Engine) hasCapability(caps []domaincoverageplan.CoverageCapability, cap domaincoverageplan.CoverageCapability) bool {
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}

// removeCapability removes a capability from the list.
func (e *Engine) removeCapability(caps []domaincoverageplan.CoverageCapability, cap domaincoverageplan.CoverageCapability) []domaincoverageplan.CoverageCapability {
	result := make([]domaincoverageplan.CoverageCapability, 0, len(caps))
	for _, c := range caps {
		if c != cap {
			result = append(result, c)
		}
	}
	return domain.NormalizeCapabilities(result)
}

// BuildSettingsPage builds the settings page model.
func (e *Engine) BuildSettingsPage(currentCaps []domaincoverageplan.CoverageCapability) domain.ObserverSettingsPage {
	allowlisted := domain.AllowlistedCapabilities()
	capabilities := make([]domain.ObserverCapabilityStatus, len(allowlisted))

	for i, cap := range allowlisted {
		kind := domain.KindFromCapability(cap)
		capabilities[i] = domain.ObserverCapabilityStatus{
			Capability:  cap,
			Kind:        kind,
			Label:       cap.DisplayLabel(),
			Description: e.capabilityDescription(cap),
			Enabled:     e.hasCapability(currentCaps, cap),
			Allowlisted: true,
		}
	}

	// Compute status hash
	statusHash := e.computeSettingsStatusHash(capabilities)

	return domain.ObserverSettingsPage{
		Title:        "Observer Capabilities",
		Description:  "Consent changes what QuantumLife can observe. Not what it can do without you.",
		Capabilities: capabilities,
		StatusHash:   statusHash,
	}
}

// capabilityDescription returns a user-friendly description for a capability.
func (e *Engine) capabilityDescription(cap domaincoverageplan.CoverageCapability) string {
	switch cap {
	case domaincoverageplan.CapReceiptObserver:
		return "Observe receipt patterns in your email to identify purchases and deadlines."
	case domaincoverageplan.CapCommerceObserver:
		return "Observe commerce patterns from receipts and transactions."
	case domaincoverageplan.CapFinanceCommerceObserver:
		return "Observe commerce patterns from your connected bank account."
	case domaincoverageplan.CapNotificationMetadata:
		return "Observe notification patterns from your device (app class and frequency only)."
	default:
		return "Observe patterns from this source."
	}
}

// computeSettingsStatusHash computes the status hash for the settings page.
func (e *Engine) computeSettingsStatusHash(capabilities []domain.ObserverCapabilityStatus) string {
	canonical := "settings"
	for _, cap := range capabilities {
		enabledStr := "off"
		if cap.Enabled {
			enabledStr = "on"
		}
		canonical += fmt.Sprintf("|%s:%s", cap.Capability.CanonicalString(), enabledStr)
	}
	return domain.HashString(canonical)
}

// BuildProofPage builds the proof page model.
func (e *Engine) BuildProofPage(receipts []domain.ObserverConsentReceipt) domain.ObserverProofPage {
	// Limit to max display
	displayReceipts := receipts
	if len(displayReceipts) > domain.MaxProofDisplayReceipts {
		displayReceipts = displayReceipts[:domain.MaxProofDisplayReceipts]
	}

	// Build lines
	var lines []string
	if len(displayReceipts) == 0 {
		lines = []string{
			"No recent changes to observer consent.",
		}
	} else {
		lines = []string{
			"Here are your recent observer consent changes.",
			"These changes affect only what patterns are observed, not what actions are taken.",
		}
	}

	// Compute status hash
	statusHash := e.computeProofStatusHash(displayReceipts)

	return domain.ObserverProofPage{
		Title:       "Observer Consent Proof",
		Lines:       lines,
		Receipts:    displayReceipts,
		StatusHash:  statusHash,
		MaxReceipts: domain.MaxProofDisplayReceipts,
	}
}

// computeProofStatusHash computes the status hash for the proof page.
func (e *Engine) computeProofStatusHash(receipts []domain.ObserverConsentReceipt) string {
	canonical := "proof"
	for _, r := range receipts {
		canonical += fmt.Sprintf("|%s", r.ReceiptHash)
	}
	return domain.HashString(canonical)
}

// ValidateNoForbiddenFields checks if any forbidden fields are present.
func ValidateNoForbiddenFields(fields map[string]string) error {
	forbidden := domain.ForbiddenClientFields()
	for _, f := range forbidden {
		if _, exists := fields[f]; exists {
			return fmt.Errorf("forbidden field '%s' provided by client", f)
		}
	}
	return nil
}

// CreateRejectionReceipt creates a rejection receipt for validation failures.
func (e *Engine) CreateRejectionReceipt(circleIDHash string, action domain.ConsentAction, cap domaincoverageplan.CoverageCapability, reason domain.RejectReason) domain.ObserverConsentReceipt {
	periodKey := e.periodKeyFunc()
	receipt := domain.ObserverConsentReceipt{
		PeriodKey:    periodKey,
		CircleIDHash: circleIDHash,
		Action:       action,
		Capability:   cap,
		Kind:         domain.KindFromCapability(cap),
		Result:       domain.ResultRejected,
		RejectReason: reason,
	}
	receipt.ReceiptHash = receipt.ComputeReceiptHash()
	return receipt
}
