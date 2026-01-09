// Package demo_phase54_urgency_delivery_binding provides demo tests for Phase 54.
//
// Phase 54: Urgency → Delivery Binding (Explicit POST, Proof-first)
//
// These tests demonstrate and verify:
// - Deterministic decision making
// - Commerce always excluded
// - Urgency level requirements
// - Rate limiting
// - Enforcement clamp handling
// - Hash-only storage
// - Bounded retention
//
// Reference: docs/ADR/ADR-0092-phase54-urgency-delivery-binding.md
package demo_phase54_urgency_delivery_binding

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	internalurgencydelivery "quantumlife/internal/urgencydelivery"
	domainurgencydelivery "quantumlife/pkg/domain/urgencydelivery"
)

// ============================================================================
// Engine Tests
// ============================================================================

func TestEngine_ComputeDecision_Determinism(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	// Run twice with same inputs
	decision1 := engine.ComputeDecision(inputs)
	decision2 := engine.ComputeDecision(inputs)

	if decision1.DeterministicDecisionHash != decision2.DeterministicDecisionHash {
		t.Errorf("Determinism failed: hash1=%s, hash2=%s",
			decision1.DeterministicDecisionHash, decision2.DeterministicDecisionHash)
	}

	if decision1.Intent != decision2.Intent {
		t.Errorf("Determinism failed: intent1=%s, intent2=%s",
			decision1.Intent, decision2.Intent)
	}

	if decision1.ShouldAttemptDelivery != decision2.ShouldAttemptDelivery {
		t.Errorf("Determinism failed: shouldDeliver1=%v, shouldDeliver2=%v",
			decision1.ShouldAttemptDelivery, decision2.ShouldAttemptDelivery)
	}
}

func TestEngine_ComputeDecision_DifferentInputs_DifferentHash(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs1 := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	inputs2 := inputs1
	inputs2.UrgencyBucket = domainurgencydelivery.UrgencyLow // Different urgency

	decision1 := engine.ComputeDecision(inputs1)
	decision2 := engine.ComputeDecision(inputs2)

	// Different inputs should yield different decisions
	if decision1.Intent == decision2.Intent &&
		decision1.ShouldAttemptDelivery == decision2.ShouldAttemptDelivery {
		t.Errorf("Expected different decisions for different inputs")
	}
}

func TestEngine_ComputeDecision_CommerceExcluded(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeCommerce, // Commerce!
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Commerce should never be delivered")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectCommerceExcluded {
		t.Errorf("Expected RejectCommerceExcluded, got %s", decision.RejectionReason)
	}

	if decision.Intent != domainurgencydelivery.IntentHold {
		t.Errorf("Commerce should have IntentHold, got %s", decision.Intent)
	}
}

func TestEngine_ComputeDecision_NoCandidate(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "", // No candidate
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNone,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeNothing,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver without candidate")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectNoCandidate {
		t.Errorf("Expected RejectNoCandidate, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_UrgencyLow_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyLow, // Low urgency
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Low urgency should not deliver")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectNotPermittedByUrgency {
		t.Errorf("Expected RejectNotPermittedByUrgency, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_UrgencyNone_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyNone, // No urgency
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("No urgency should not deliver")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectNotPermittedByUrgency {
		t.Errorf("Expected RejectNotPermittedByUrgency, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_UrgencyMedium_Allowed(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyMedium, // Medium urgency
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if !decision.ShouldAttemptDelivery {
		t.Errorf("Medium urgency should allow delivery, rejection=%s", decision.RejectionReason)
	}

	if decision.Intent != domainurgencydelivery.IntentDeliver {
		t.Errorf("Expected IntentDeliver, got %s", decision.Intent)
	}
}

func TestEngine_ComputeDecision_UrgencyHigh_Allowed(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh, // High urgency
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if !decision.ShouldAttemptDelivery {
		t.Errorf("High urgency should allow delivery, rejection=%s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_NoDevice_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 false, // No device
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver without device")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectNoDevice {
		t.Errorf("Expected RejectNoDevice, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_TransportUnavailable_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        false, // Transport unavailable
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver without transport")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectTransportUnavailable {
		t.Errorf("Expected RejectTransportUnavailable, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_SealedKeyMissing_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        false, // Sealed key missing
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver without sealed key")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectSealedKeyMissing {
		t.Errorf("Expected RejectSealedKeyMissing, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_RateLimited_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       2, // Already at max (2)
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver when rate limited")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectRateLimited {
		t.Errorf("Expected RejectRateLimited, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_EnforcementClamped_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementClamped, // Clamped
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver when enforcement clamped")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectEnforcementClamped {
		t.Errorf("Expected RejectEnforcementClamped, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_PolicyDenied_Rejected(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyDenied, // Policy denied
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	if decision.ShouldAttemptDelivery {
		t.Error("Should not deliver when policy denied")
	}

	if decision.RejectionReason != domainurgencydelivery.RejectPolicyDisallows {
		t.Errorf("Expected RejectPolicyDisallows, got %s", decision.RejectionReason)
	}
}

func TestEngine_ComputeDecision_EnforcementClamp_Dominates(t *testing.T) {
	// Enforcement clamp should take precedence over policy denied
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyDenied, // Also denied
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementClamped, // Clamped
		DeliveredTodayCount:       0,
	}

	decision := engine.ComputeDecision(inputs)

	// Enforcement clamp should be checked before policy
	if decision.RejectionReason != domainurgencydelivery.RejectEnforcementClamped {
		t.Errorf("Expected RejectEnforcementClamped to dominate, got %s", decision.RejectionReason)
	}
}

func TestEngine_BuildReceipt_NoAttempt(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		UrgencyBucket:             domainurgencydelivery.UrgencyLow,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNone,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeNothing,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
	}

	decision := domainurgencydelivery.BindingDecision{
		Intent:                    domainurgencydelivery.IntentSurfaceOnly,
		ShouldAttemptDelivery:     false,
		RejectionReason:           domainurgencydelivery.RejectNotPermittedByUrgency,
		DeterministicDecisionHash: "test123",
	}

	receipt := engine.BuildReceipt(inputs, decision, false)

	if receipt.OutcomeKind != domainurgencydelivery.OutcomeNotDelivered {
		t.Errorf("Expected OutcomeNotDelivered, got %s", receipt.OutcomeKind)
	}

	if receipt.AttemptIDHash != "" {
		t.Errorf("Expected empty AttemptIDHash, got %s", receipt.AttemptIDHash)
	}

	if receipt.ReceiptHash == "" {
		t.Error("ReceiptHash should be computed")
	}
}

func TestEngine_BuildReceiptWithAttempt(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
	}

	decision := domainurgencydelivery.BindingDecision{
		Intent:                    domainurgencydelivery.IntentDeliver,
		ShouldAttemptDelivery:     true,
		RejectionReason:           domainurgencydelivery.RejectNone,
		DeterministicDecisionHash: "test123",
	}

	attemptIDHash := "attempt_abc_123_xyz"
	receipt := engine.BuildReceiptWithAttempt(inputs, decision, attemptIDHash)

	if receipt.OutcomeKind != domainurgencydelivery.OutcomeDelivered {
		t.Errorf("Expected OutcomeDelivered, got %s", receipt.OutcomeKind)
	}

	if receipt.AttemptIDHash != attemptIDHash {
		t.Errorf("Expected AttemptIDHash=%s, got %s", attemptIDHash, receipt.AttemptIDHash)
	}
}

func TestEngine_BuildProofPage_Empty(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	page := engine.BuildProofPage(nil)

	if page.Title == "" {
		t.Error("Page title should not be empty")
	}

	if len(page.Lines) == 0 {
		t.Error("Page lines should not be empty")
	}

	if len(page.RecentReceipts) != 0 {
		t.Error("Recent receipts should be empty")
	}

	if page.StatusHash == "" {
		t.Error("StatusHash should be computed")
	}
}

func TestEngine_BuildProofPage_MaxReceipts(t *testing.T) {
	engine := internalurgencydelivery.NewEngine()

	// Create 10 receipts
	receipts := make([]domainurgencydelivery.UrgencyDeliveryReceipt, 10)
	for i := 0; i < 10; i++ {
		receipts[i] = domainurgencydelivery.UrgencyDeliveryReceipt{
			ReceiptHash:     "hash" + string(rune('A'+i)),
			CircleIDHash:    "circle123",
			PeriodKey:       "2025-01-09",
			RunKind:         domainurgencydelivery.RunManual,
			OutcomeKind:     domainurgencydelivery.OutcomeNotDelivered,
			UrgencyBucket:   domainurgencydelivery.UrgencyLow,
			Intent:          domainurgencydelivery.IntentHold,
			RejectionReason: domainurgencydelivery.RejectNotPermittedByUrgency,
			CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
		}
	}

	page := engine.BuildProofPage(receipts)

	// Should be capped at MaxProofPageReceipts (6)
	if len(page.RecentReceipts) > domainurgencydelivery.MaxProofPageReceipts {
		t.Errorf("RecentReceipts should be capped at %d, got %d",
			domainurgencydelivery.MaxProofPageReceipts, len(page.RecentReceipts))
	}
}

func TestMapUrgencyLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected domainurgencydelivery.UrgencyBucket
	}{
		{"urg_none", domainurgencydelivery.UrgencyNone},
		{"urg_low", domainurgencydelivery.UrgencyLow},
		{"urg_medium", domainurgencydelivery.UrgencyMedium},
		{"urg_high", domainurgencydelivery.UrgencyHigh},
		{"unknown", domainurgencydelivery.UrgencyNone},
	}

	for _, tt := range tests {
		result := internalurgencydelivery.MapUrgencyLevel(tt.input)
		if result != tt.expected {
			t.Errorf("MapUrgencyLevel(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestMapCircleType(t *testing.T) {
	tests := []struct {
		input    string
		expected domainurgencydelivery.CircleTypeBucket
	}{
		{"bucket_human", domainurgencydelivery.CircleTypeHuman},
		{"bucket_institution", domainurgencydelivery.CircleTypeInstitution},
		{"bucket_commerce", domainurgencydelivery.CircleTypeCommerce},
		{"bucket_unknown", domainurgencydelivery.CircleTypeUnknown},
		{"anything_else", domainurgencydelivery.CircleTypeUnknown},
	}

	for _, tt := range tests {
		result := internalurgencydelivery.MapCircleType(tt.input)
		if result != tt.expected {
			t.Errorf("MapCircleType(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

// ============================================================================
// Store Tests
// ============================================================================

func TestStore_AppendReceipt_Success(t *testing.T) {
	now := time.Date(2025, 1, 9, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := persist.NewUrgencyDeliveryStore(clock)

	receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
		ReceiptHash:     "hash123",
		CircleIDHash:    "circle123",
		PeriodKey:       "2025-01-09",
		RunKind:         domainurgencydelivery.RunManual,
		OutcomeKind:     domainurgencydelivery.OutcomeDelivered,
		UrgencyBucket:   domainurgencydelivery.UrgencyHigh,
		CandidateHash:   "candidate123",
		Intent:          domainurgencydelivery.IntentDeliver,
		RejectionReason: domainurgencydelivery.RejectNone,
		AttemptIDHash:   "attempt123",
		StatusHash:      "status123",
		CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
	}

	recorded, err := store.AppendReceipt(receipt)
	if err != nil {
		t.Fatalf("AppendReceipt failed: %v", err)
	}

	if !recorded {
		t.Error("Expected receipt to be recorded")
	}

	if store.Count() != 1 {
		t.Errorf("Expected count=1, got %d", store.Count())
	}
}

func TestStore_AppendReceipt_Dedup(t *testing.T) {
	now := time.Date(2025, 1, 9, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := persist.NewUrgencyDeliveryStore(clock)

	receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
		ReceiptHash:     "hash123",
		CircleIDHash:    "circle123",
		PeriodKey:       "2025-01-09",
		RunKind:         domainurgencydelivery.RunManual,
		OutcomeKind:     domainurgencydelivery.OutcomeDelivered,
		UrgencyBucket:   domainurgencydelivery.UrgencyHigh,
		CandidateHash:   "candidate123",
		Intent:          domainurgencydelivery.IntentDeliver,
		RejectionReason: domainurgencydelivery.RejectNone,
		AttemptIDHash:   "attempt123",
		StatusHash:      "status123",
		CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
	}

	// First append
	recorded1, _ := store.AppendReceipt(receipt)
	if !recorded1 {
		t.Error("First append should succeed")
	}

	// Second append with same dedup key
	recorded2, _ := store.AppendReceipt(receipt)
	if recorded2 {
		t.Error("Second append should be deduped")
	}

	if store.Count() != 1 {
		t.Errorf("Expected count=1 after dedup, got %d", store.Count())
	}
}

func TestStore_ListRecentByCircle(t *testing.T) {
	now := time.Date(2025, 1, 9, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := persist.NewUrgencyDeliveryStore(clock)

	// Add receipts for different circles
	for i := 0; i < 5; i++ {
		receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
			ReceiptHash:     "hash" + string(rune('A'+i)),
			CircleIDHash:    "circle123",
			PeriodKey:       "2025-01-09",
			RunKind:         domainurgencydelivery.RunManual,
			OutcomeKind:     domainurgencydelivery.OutcomeNotDelivered,
			UrgencyBucket:   domainurgencydelivery.UrgencyLow,
			CandidateHash:   "candidate" + string(rune('A'+i)),
			Intent:          domainurgencydelivery.IntentHold,
			RejectionReason: domainurgencydelivery.RejectNotPermittedByUrgency,
			StatusHash:      "status" + string(rune('A'+i)),
			CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
		}
		store.AppendReceipt(receipt)
	}

	// Add receipt for different circle
	other := domainurgencydelivery.UrgencyDeliveryReceipt{
		ReceiptHash:     "other",
		CircleIDHash:    "other_circle",
		PeriodKey:       "2025-01-09",
		RunKind:         domainurgencydelivery.RunManual,
		OutcomeKind:     domainurgencydelivery.OutcomeNotDelivered,
		UrgencyBucket:   domainurgencydelivery.UrgencyLow,
		CandidateHash:   "other_candidate",
		Intent:          domainurgencydelivery.IntentHold,
		RejectionReason: domainurgencydelivery.RejectNotPermittedByUrgency,
		StatusHash:      "other_status",
		CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
	}
	store.AppendReceipt(other)

	// List for circle123, limit 3
	results := store.ListRecentByCircle("circle123", 3)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for _, r := range results {
		if r.CircleIDHash != "circle123" {
			t.Errorf("Expected circle123, got %s", r.CircleIDHash)
		}
	}
}

func TestStore_HasReceiptForCandidatePeriod(t *testing.T) {
	now := time.Date(2025, 1, 9, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := persist.NewUrgencyDeliveryStore(clock)

	receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
		ReceiptHash:     "hash123",
		CircleIDHash:    "circle123",
		PeriodKey:       "2025-01-09",
		RunKind:         domainurgencydelivery.RunManual,
		OutcomeKind:     domainurgencydelivery.OutcomeDelivered,
		UrgencyBucket:   domainurgencydelivery.UrgencyHigh,
		CandidateHash:   "candidate123",
		Intent:          domainurgencydelivery.IntentDeliver,
		RejectionReason: domainurgencydelivery.RejectNone,
		AttemptIDHash:   "attempt123",
		StatusHash:      "status123",
		CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
	}

	store.AppendReceipt(receipt)

	// Should find it
	if !store.HasReceiptForCandidatePeriod("circle123", "candidate123", "2025-01-09") {
		t.Error("Should find existing receipt")
	}

	// Should not find different combination
	if store.HasReceiptForCandidatePeriod("circle123", "candidate999", "2025-01-09") {
		t.Error("Should not find non-existent receipt")
	}
}

func TestStore_CountDeliveredForPeriod(t *testing.T) {
	now := time.Date(2025, 1, 9, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := persist.NewUrgencyDeliveryStore(clock)

	// Add 2 delivered + 1 not delivered
	for i := 0; i < 3; i++ {
		outcome := domainurgencydelivery.OutcomeDelivered
		if i == 2 {
			outcome = domainurgencydelivery.OutcomeNotDelivered
		}
		receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
			ReceiptHash:     "hash" + string(rune('A'+i)),
			CircleIDHash:    "circle123",
			PeriodKey:       "2025-01-09",
			RunKind:         domainurgencydelivery.RunManual,
			OutcomeKind:     outcome,
			UrgencyBucket:   domainurgencydelivery.UrgencyHigh,
			CandidateHash:   "candidate" + string(rune('A'+i)),
			Intent:          domainurgencydelivery.IntentDeliver,
			RejectionReason: domainurgencydelivery.RejectNone,
			StatusHash:      "status" + string(rune('A'+i)),
			CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
		}
		store.AppendReceipt(receipt)
	}

	count := store.CountDeliveredForPeriod("circle123", "2025-01-09")
	if count != 2 {
		t.Errorf("Expected 2 delivered, got %d", count)
	}
}

func TestStore_BoundedRetention_MaxEntries(t *testing.T) {
	now := time.Date(2025, 1, 9, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := persist.NewUrgencyDeliveryStore(clock)

	// Add more than MaxEntries
	for i := 0; i < persist.UrgencyDeliveryMaxEntries+50; i++ {
		receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
			ReceiptHash:     "hash" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			CircleIDHash:    "circle123",
			PeriodKey:       "2025-01-09",
			RunKind:         domainurgencydelivery.RunManual,
			OutcomeKind:     domainurgencydelivery.OutcomeNotDelivered,
			UrgencyBucket:   domainurgencydelivery.UrgencyLow,
			CandidateHash:   "candidate" + string(rune(i)),
			Intent:          domainurgencydelivery.IntentHold,
			RejectionReason: domainurgencydelivery.RejectNotPermittedByUrgency,
			StatusHash:      "status" + string(rune(i)),
			CreatedBucket:   domainurgencydelivery.CreatedBucketThisPeriod,
		}
		store.AppendReceipt(receipt)
	}

	// Should not exceed max
	if store.Count() > persist.UrgencyDeliveryMaxEntries {
		t.Errorf("Store should be capped at %d, got %d",
			persist.UrgencyDeliveryMaxEntries, store.Count())
	}
}

// ============================================================================
// Domain Type Tests
// ============================================================================

func TestBindingInputs_Validate(t *testing.T) {
	valid := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("Valid inputs should pass validation: %v", err)
	}

	// Test missing CircleIDHash
	invalid := valid
	invalid.CircleIDHash = ""
	if err := invalid.Validate(); err == nil {
		t.Error("Missing CircleIDHash should fail validation")
	}

	// Test missing PeriodKey
	invalid = valid
	invalid.PeriodKey = ""
	if err := invalid.Validate(); err == nil {
		t.Error("Missing PeriodKey should fail validation")
	}

	// Test pipe in PeriodKey
	invalid = valid
	invalid.PeriodKey = "2025|01|09"
	if err := invalid.Validate(); err == nil {
		t.Error("Pipe in PeriodKey should fail validation")
	}
}

func TestBindingInputs_CanonicalString(t *testing.T) {
	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		HasDevice:                 true,
		TransportAvailable:        true,
		SealedKeyAvailable:        true,
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateHash:             "candidate123",
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNow,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeAFew,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
		DeliveredTodayCount:       0,
	}

	canonical := inputs.CanonicalString()

	// Should use pipe delimiter
	if !strings.Contains(canonical, "|") {
		t.Error("Canonical string should use pipe delimiter")
	}

	// Should contain v1
	if !strings.HasPrefix(canonical, "v1|") {
		t.Error("Canonical string should start with v1|")
	}

	// Should contain key=value pairs
	if !strings.Contains(canonical, "circle=abc123") {
		t.Error("Canonical string should contain circle=abc123")
	}
}

func TestBindingInputs_Hash(t *testing.T) {
	inputs := domainurgencydelivery.BindingInputs{
		CircleIDHash:              "abc123",
		PeriodKey:                 "2025-01-09",
		UrgencyBucket:             domainurgencydelivery.UrgencyHigh,
		CandidateCircleTypeBucket: domainurgencydelivery.CircleTypeHuman,
		CandidateHorizonBucket:    domainurgencydelivery.HorizonNone,
		CandidateMagnitudeBucket:  domainurgencydelivery.MagnitudeNothing,
		PolicyAllowanceBucket:     domainurgencydelivery.PolicyAllowed,
		EnvelopeActivityBucket:    domainurgencydelivery.EnvelopeNone,
		EnforcementClampBucket:    domainurgencydelivery.EnforcementNotClamped,
	}

	hash1 := inputs.Hash()
	hash2 := inputs.Hash()

	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	if len(hash1) != 64 { // SHA256 hex
		t.Errorf("Hash should be 64 chars (SHA256 hex), got %d", len(hash1))
	}
}

func TestUrgencyBucket_AllowsDelivery(t *testing.T) {
	tests := []struct {
		bucket   domainurgencydelivery.UrgencyBucket
		expected bool
	}{
		{domainurgencydelivery.UrgencyNone, false},
		{domainurgencydelivery.UrgencyLow, false},
		{domainurgencydelivery.UrgencyMedium, true},
		{domainurgencydelivery.UrgencyHigh, true},
	}

	for _, tt := range tests {
		result := tt.bucket.AllowsDelivery()
		if result != tt.expected {
			t.Errorf("%s.AllowsDelivery() = %v, expected %v", tt.bucket, result, tt.expected)
		}
	}
}

func TestCircleTypeBucket_IsCommerce(t *testing.T) {
	tests := []struct {
		bucket   domainurgencydelivery.CircleTypeBucket
		expected bool
	}{
		{domainurgencydelivery.CircleTypeHuman, false},
		{domainurgencydelivery.CircleTypeInstitution, false},
		{domainurgencydelivery.CircleTypeCommerce, true},
		{domainurgencydelivery.CircleTypeUnknown, false},
	}

	for _, tt := range tests {
		result := tt.bucket.IsCommerce()
		if result != tt.expected {
			t.Errorf("%s.IsCommerce() = %v, expected %v", tt.bucket, result, tt.expected)
		}
	}
}

func TestReceiptLineFromReceipt(t *testing.T) {
	receipt := domainurgencydelivery.UrgencyDeliveryReceipt{
		ReceiptHash:     "0123456789abcdef",
		OutcomeKind:     domainurgencydelivery.OutcomeDelivered,
		UrgencyBucket:   domainurgencydelivery.UrgencyHigh,
		Intent:          domainurgencydelivery.IntentDeliver,
		RejectionReason: domainurgencydelivery.RejectNone,
	}

	line := domainurgencydelivery.ReceiptLineFromReceipt(receipt)

	if line.OutcomeKind != receipt.OutcomeKind {
		t.Error("OutcomeKind should match")
	}

	if line.UrgencyBucket != receipt.UrgencyBucket {
		t.Error("UrgencyBucket should match")
	}

	if line.ReceiptHashPrefix != "01234567" {
		t.Errorf("ReceiptHashPrefix should be 8 chars, got %s", line.ReceiptHashPrefix)
	}
}

func TestProofPage_Validate(t *testing.T) {
	valid := domainurgencydelivery.ProofPage{
		Title: "Test Page",
		Lines: []string{"Line 1", "Line 2"},
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("Valid page should pass validation: %v", err)
	}

	// Test missing title
	invalid := domainurgencydelivery.ProofPage{
		Title: "",
		Lines: []string{"Line 1"},
	}
	if err := invalid.Validate(); err == nil {
		t.Error("Missing title should fail validation")
	}

	// Test too many lines
	manyLines := domainurgencydelivery.ProofPage{
		Title: "Test",
		Lines: make([]string, domainurgencydelivery.MaxProofPageLines+1),
	}
	for i := range manyLines.Lines {
		manyLines.Lines[i] = "Line"
	}
	if err := manyLines.Validate(); err == nil {
		t.Error("Too many lines should fail validation")
	}
}

func TestForbiddenPatterns(t *testing.T) {
	forbidden := []string{
		"test@example.com",
		"https://example.com",
		"vendor_id_123",
		"pack_id_456",
		"merchant: Amazon",
		"amount: $100",
		"device_token: abc123",
	}

	for _, s := range forbidden {
		if !domainurgencydelivery.ContainsForbiddenPattern(s) {
			t.Errorf("'%s' should be detected as forbidden", s)
		}
	}

	allowed := []string{
		"abc123",
		"circle_hash_xyz",
		"2025-01-09",
		"urgency_high",
	}

	for _, s := range allowed {
		if domainurgencydelivery.ContainsForbiddenPattern(s) {
			t.Errorf("'%s' should not be forbidden", s)
		}
	}
}
