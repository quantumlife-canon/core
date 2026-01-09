// Package urgencydelivery implements the Phase 54 Urgency → Delivery Binding engine.
//
// The engine binds urgency resolution (Phase 53) to the interrupt delivery pipeline
// (Phases 32-36) without introducing new decision logic.
//
// CRITICAL INVARIANTS:
//   - NO BACKGROUND EXECUTION: No goroutines, no timers, no polling.
//   - POST-TRIGGERED ONLY: Delivery attempts only via explicit POST.
//   - NO NEW DECISION LOGIC: Reuses existing pipelines.
//   - COMMERCE EXCLUDED: Never escalated, never delivered.
//   - DETERMINISTIC: Same inputs + same clock → same output hash/ID.
//   - HASH-ONLY: Never stores raw identifiers.
//   - No time.Now() - uses injected clock.
//
// Reference: docs/ADR/ADR-0092-phase54-urgency-delivery-binding.md
package urgencydelivery

import (
	"quantumlife/pkg/domain/urgencydelivery"
)

// Engine implements the Phase 54 urgency delivery binding logic.
// CRITICAL: Pure functions only. No side effects. No goroutines.
type Engine struct{}

// NewEngine creates a new urgency delivery binding engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ComputeDecision computes the binding decision from inputs.
// Returns a deterministic decision based on the rules defined in Phase 54.
//
// Rules (deterministic precedence):
//
//  0. No candidate → reject_no_candidate
//  1. Commerce circle → reject_commerce_excluded
//  2. Enforcement clamped → reject_enforcement_clamped
//  3. Policy disallows → reject_policy_disallows
//  4. Urgency < medium → reject_not_permitted_by_urgency
//  5. No device → reject_no_device
//  6. Transport unavailable → reject_transport_unavailable
//  7. Sealed key missing → reject_sealed_key_missing
//  8. Rate limited → reject_rate_limited
//  9. Else → intent_deliver, ShouldAttemptDelivery=true
func (e *Engine) ComputeDecision(inputs urgencydelivery.BindingInputs) urgencydelivery.BindingDecision {
	decision := urgencydelivery.BindingDecision{
		Intent:                IntentFromInputs(inputs),
		ShouldAttemptDelivery: false,
		RejectionReason:       urgencydelivery.RejectNone,
	}

	// Rule 0: No candidate
	if inputs.CandidateHash == "" {
		decision.Intent = urgencydelivery.IntentHold
		decision.RejectionReason = urgencydelivery.RejectNoCandidate
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 1: Commerce excluded
	if inputs.CandidateCircleTypeBucket.IsCommerce() {
		decision.Intent = urgencydelivery.IntentHold
		decision.RejectionReason = urgencydelivery.RejectCommerceExcluded
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 2: Enforcement clamped
	if inputs.EnforcementClampBucket == urgencydelivery.EnforcementClamped {
		decision.Intent = urgencydelivery.IntentHold
		decision.RejectionReason = urgencydelivery.RejectEnforcementClamped
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 3: Policy disallows
	if inputs.PolicyAllowanceBucket == urgencydelivery.PolicyDenied {
		decision.Intent = urgencydelivery.IntentHold
		decision.RejectionReason = urgencydelivery.RejectPolicyDisallows
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 4: Urgency < medium
	if !inputs.UrgencyBucket.AllowsDelivery() {
		decision.Intent = urgencydelivery.IntentSurfaceOnly
		decision.RejectionReason = urgencydelivery.RejectNotPermittedByUrgency
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 5: No device
	if !inputs.HasDevice {
		decision.Intent = urgencydelivery.IntentInterruptCandidate
		decision.RejectionReason = urgencydelivery.RejectNoDevice
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 6: Transport unavailable
	if !inputs.TransportAvailable {
		decision.Intent = urgencydelivery.IntentInterruptCandidate
		decision.RejectionReason = urgencydelivery.RejectTransportUnavailable
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 7: Sealed key missing
	if !inputs.SealedKeyAvailable {
		decision.Intent = urgencydelivery.IntentInterruptCandidate
		decision.RejectionReason = urgencydelivery.RejectSealedKeyMissing
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 8: Rate limited
	if inputs.DeliveredTodayCount >= urgencydelivery.MaxDeliveriesPerDay {
		decision.Intent = urgencydelivery.IntentInterruptCandidate
		decision.RejectionReason = urgencydelivery.RejectRateLimited
		decision.DeterministicDecisionHash = decision.ComputeHash()
		return decision
	}

	// Rule 9: All checks passed - attempt delivery
	decision.Intent = urgencydelivery.IntentDeliver
	decision.ShouldAttemptDelivery = true
	decision.RejectionReason = urgencydelivery.RejectNone
	decision.DeterministicDecisionHash = decision.ComputeHash()
	return decision
}

// IntentFromInputs derives the initial intent from inputs.
func IntentFromInputs(inputs urgencydelivery.BindingInputs) urgencydelivery.DeliveryIntentKind {
	// Commerce is always hold
	if inputs.CandidateCircleTypeBucket.IsCommerce() {
		return urgencydelivery.IntentHold
	}

	// Map urgency to intent
	switch inputs.UrgencyBucket {
	case urgencydelivery.UrgencyNone:
		return urgencydelivery.IntentHold
	case urgencydelivery.UrgencyLow:
		return urgencydelivery.IntentSurfaceOnly
	case urgencydelivery.UrgencyMedium:
		return urgencydelivery.IntentInterruptCandidate
	case urgencydelivery.UrgencyHigh:
		return urgencydelivery.IntentDeliver
	default:
		return urgencydelivery.IntentHold
	}
}

// BuildReceipt builds a delivery receipt from inputs and decision.
// The receipt does not include AttemptIDHash - that is added after actual delivery.
func (e *Engine) BuildReceipt(
	inputs urgencydelivery.BindingInputs,
	decision urgencydelivery.BindingDecision,
	delivered bool,
) urgencydelivery.UrgencyDeliveryReceipt {
	outcome := urgencydelivery.OutcomeNotDelivered
	if delivered {
		outcome = urgencydelivery.OutcomeDelivered
	}

	receipt := urgencydelivery.UrgencyDeliveryReceipt{
		CircleIDHash:    inputs.CircleIDHash,
		PeriodKey:       inputs.PeriodKey,
		RunKind:         urgencydelivery.RunManual,
		OutcomeKind:     outcome,
		UrgencyBucket:   inputs.UrgencyBucket,
		CandidateHash:   inputs.CandidateHash,
		Intent:          decision.Intent,
		RejectionReason: decision.RejectionReason,
		AttemptIDHash:   "", // Filled in by caller if delivered
		CreatedBucket:   urgencydelivery.CreatedBucketThisPeriod,
	}

	receipt.ReceiptHash = receipt.ComputeReceiptHash()
	receipt.StatusHash = receipt.ComputeStatusHash()
	return receipt
}

// BuildReceiptWithAttempt builds a delivery receipt with the attempt ID hash.
func (e *Engine) BuildReceiptWithAttempt(
	inputs urgencydelivery.BindingInputs,
	decision urgencydelivery.BindingDecision,
	attemptIDHash string,
) urgencydelivery.UrgencyDeliveryReceipt {
	receipt := e.BuildReceipt(inputs, decision, attemptIDHash != "")
	if attemptIDHash != "" {
		receipt.AttemptIDHash = attemptIDHash
		// Recompute hashes with attempt ID
		receipt.ReceiptHash = receipt.ComputeReceiptHash()
		receipt.StatusHash = receipt.ComputeStatusHash()
	}
	return receipt
}

// BuildProofPage builds the proof page from receipts.
func (e *Engine) BuildProofPage(receipts []urgencydelivery.UrgencyDeliveryReceipt) urgencydelivery.ProofPage {
	if len(receipts) == 0 {
		return urgencydelivery.DefaultProofPage()
	}

	// Limit to max receipts
	displayReceipts := receipts
	if len(displayReceipts) > urgencydelivery.MaxProofPageReceipts {
		displayReceipts = displayReceipts[:urgencydelivery.MaxProofPageReceipts]
	}

	// Convert to receipt lines
	lines := make([]urgencydelivery.ReceiptLine, len(displayReceipts))
	for i, r := range displayReceipts {
		lines[i] = urgencydelivery.ReceiptLineFromReceipt(r)
	}

	// Determine outcome summary
	deliveredCount := 0
	for _, r := range receipts {
		if r.OutcomeKind == urgencydelivery.OutcomeDelivered {
			deliveredCount++
		}
	}

	page := urgencydelivery.ProofPage{
		Title:          "Delivery binding, quietly.",
		Lines:          e.buildProofLines(deliveredCount, len(receipts)),
		RecentReceipts: lines,
	}
	page.StatusHash = page.ComputeStatusHash()
	return page
}

// buildProofLines builds calm copy for the proof page.
func (e *Engine) buildProofLines(deliveredCount, totalCount int) []string {
	if totalCount == 0 {
		return []string{
			"Delivery is explicit, not automatic.",
			"We only deliver when you ask.",
			"Nothing was delivered this period.",
		}
	}

	if deliveredCount == 0 {
		return []string{
			"Delivery was considered but declined.",
			"Your boundaries were respected.",
			"Nothing was sent.",
		}
	}

	if deliveredCount == 1 {
		return []string{
			"One message was delivered.",
			"Abstract content only — no details.",
			"Your attention is protected.",
		}
	}

	return []string{
		"A few messages were delivered.",
		"Abstract content only — no details.",
		"Daily limits were respected.",
	}
}

// MapUrgencyLevel maps Phase 53 urgency level string to Phase 54 bucket.
func MapUrgencyLevel(level string) urgencydelivery.UrgencyBucket {
	switch level {
	case "urg_none":
		return urgencydelivery.UrgencyNone
	case "urg_low":
		return urgencydelivery.UrgencyLow
	case "urg_medium":
		return urgencydelivery.UrgencyMedium
	case "urg_high":
		return urgencydelivery.UrgencyHigh
	default:
		return urgencydelivery.UrgencyNone
	}
}

// MapCircleType maps Phase 53 circle type string to Phase 54 bucket.
func MapCircleType(circleType string) urgencydelivery.CircleTypeBucket {
	switch circleType {
	case "bucket_human":
		return urgencydelivery.CircleTypeHuman
	case "bucket_institution":
		return urgencydelivery.CircleTypeInstitution
	case "bucket_commerce":
		return urgencydelivery.CircleTypeCommerce
	case "bucket_unknown":
		return urgencydelivery.CircleTypeUnknown
	default:
		return urgencydelivery.CircleTypeUnknown
	}
}
