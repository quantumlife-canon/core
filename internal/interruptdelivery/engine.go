// Package interruptdelivery implements the Phase 36 Interrupt Delivery Orchestrator engine.
//
// The engine orchestrates the interrupt pipeline:
// External Pressure (31.4) → Decision Gate (32) → Permission Contract (33) →
// Preview (34) → Transport (35/35b)
//
// CRITICAL INVARIANTS:
//   - Delivery is EXPLICIT. POST-only. No background execution.
//   - NO goroutines. NO time.Now() in business logic.
//   - Max 2 deliveries per day. Hard cap enforced.
//   - Deterministic ordering. Candidates sorted by hash.
//   - Transport-agnostic. Uses Phase 35 transport interface.
//   - Does NOT implement new decision logic.
//   - Hash-only storage.
//
// Reference: docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md
package interruptdelivery

import (
	"sort"

	"quantumlife/pkg/domain/interruptdelivery"
)

// Engine orchestrates interrupt delivery.
// CRITICAL: Pure functions only. No side effects. No goroutines.
type Engine struct{}

// NewEngine creates a new delivery engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ComputeDeliveryRun processes delivery input and returns attempts + receipt.
// Returns nil receipt if nothing to deliver.
func (e *Engine) ComputeDeliveryRun(input *interruptdelivery.DeliveryInput) ([]*interruptdelivery.DeliveryAttempt, *interruptdelivery.DeliveryReceipt) {
	if input == nil {
		return nil, nil
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, nil
	}

	// No candidates = nothing to do
	if len(input.Candidates) == 0 {
		return nil, nil
	}

	// Sort candidates by hash for deterministic ordering
	sortedCandidates := make([]*interruptdelivery.DeliveryCandidate, len(input.Candidates))
	copy(sortedCandidates, input.Candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].CandidateHash < sortedCandidates[j].CandidateHash
	})

	// Calculate how many we can send
	maxPerDay := input.MaxPerDay
	if maxPerDay <= 0 {
		maxPerDay = interruptdelivery.MaxDeliveriesPerDay
	}
	remainingSlots := maxPerDay - input.SentToday
	if remainingSlots <= 0 {
		remainingSlots = 0
	}

	// Process each candidate
	var attempts []*interruptdelivery.DeliveryAttempt
	sentCount := 0

	for _, candidate := range sortedCandidates {
		attempt := e.processCandidate(input, candidate, sentCount, remainingSlots)
		attempts = append(attempts, attempt)

		if attempt.ResultBucket == interruptdelivery.ResultSent {
			sentCount++
			remainingSlots--
		}
	}

	// Build receipt
	receipt := e.buildReceipt(input, attempts)

	return attempts, receipt
}

// processCandidate evaluates a single candidate and returns an attempt.
func (e *Engine) processCandidate(
	input *interruptdelivery.DeliveryInput,
	candidate *interruptdelivery.DeliveryCandidate,
	sentThisRun int,
	remainingSlots int,
) *interruptdelivery.DeliveryAttempt {
	attempt := &interruptdelivery.DeliveryAttempt{
		CandidateHash: candidate.CandidateHash,
		CircleIDHash:  input.CircleIDHash,
		PeriodKey:     input.PeriodKey,
		AttemptBucket: input.TimeBucket,
		TransportKind: interruptdelivery.TransportStub,
	}

	// Check 1: Policy must allow
	if !input.PolicyAllowed {
		attempt.ResultBucket = interruptdelivery.ResultSkipped
		attempt.ReasonBucket = interruptdelivery.ReasonPolicyDenies
		e.finalizeAttempt(attempt)
		return attempt
	}

	// Check 2: Trust must not be fragile
	if input.TrustFragile {
		attempt.ResultBucket = interruptdelivery.ResultSkipped
		attempt.ReasonBucket = interruptdelivery.ReasonTrustFragile
		e.finalizeAttempt(attempt)
		return attempt
	}

	// Check 3: Push must be enabled
	if !input.PushEnabled {
		attempt.ResultBucket = interruptdelivery.ResultSkipped
		attempt.ReasonBucket = interruptdelivery.ReasonNotConfigured
		e.finalizeAttempt(attempt)
		return attempt
	}

	// Check 4: Deduplication - already sent this candidate today?
	if input.PriorAttempts != nil && input.PriorAttempts[candidate.CandidateHash] {
		attempt.ResultBucket = interruptdelivery.ResultDeduped
		attempt.ReasonBucket = interruptdelivery.ReasonAlreadySent
		e.finalizeAttempt(attempt)
		return attempt
	}

	// Check 5: Daily cap
	if remainingSlots <= 0 {
		attempt.ResultBucket = interruptdelivery.ResultSkipped
		attempt.ReasonBucket = interruptdelivery.ReasonCapReached
		e.finalizeAttempt(attempt)
		return attempt
	}

	// All checks passed - mark as sent
	// Note: Actual transport call happens in cmd/ layer
	attempt.ResultBucket = interruptdelivery.ResultSent
	attempt.ReasonBucket = interruptdelivery.ReasonNone
	e.finalizeAttempt(attempt)
	return attempt
}

// finalizeAttempt computes IDs and hashes for an attempt.
func (e *Engine) finalizeAttempt(attempt *interruptdelivery.DeliveryAttempt) {
	attempt.AttemptID = attempt.ComputeAttemptID()
	attempt.StatusHash = attempt.ComputeStatusHash()
}

// buildReceipt creates a delivery receipt from attempts.
func (e *Engine) buildReceipt(
	input *interruptdelivery.DeliveryInput,
	attempts []*interruptdelivery.DeliveryAttempt,
) *interruptdelivery.DeliveryReceipt {
	receipt := &interruptdelivery.DeliveryReceipt{
		CircleIDHash: input.CircleIDHash,
		PeriodKey:    input.PeriodKey,
		TimeBucket:   input.TimeBucket,
		Attempts:     make([]interruptdelivery.AttemptSummary, 0, len(attempts)),
	}

	for _, a := range attempts {
		summary := interruptdelivery.AttemptSummary{
			ResultBucket:  a.ResultBucket,
			ReasonBucket:  a.ReasonBucket,
			TransportKind: a.TransportKind,
			AttemptHash:   a.StatusHash,
		}
		receipt.Attempts = append(receipt.Attempts, summary)

		switch a.ResultBucket {
		case interruptdelivery.ResultSent:
			receipt.SentCount++
		case interruptdelivery.ResultSkipped:
			receipt.SkippedCount++
		case interruptdelivery.ResultDeduped:
			receipt.DedupedCount++
		}
	}

	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()
	return receipt
}

// BuildProofPage builds the proof page for a delivery receipt.
func (e *Engine) BuildProofPage(receipt *interruptdelivery.DeliveryReceipt, periodKey, circleIDHash string) *interruptdelivery.DeliveryProofPage {
	if receipt == nil {
		return interruptdelivery.DefaultDeliveryProofPage(periodKey, circleIDHash)
	}

	page := &interruptdelivery.DeliveryProofPage{
		PeriodKey:      periodKey,
		CircleIDHash:   circleIDHash,
		DismissPath:    "/proof/delivery/dismiss",
		DismissMethod:  "POST",
		BackLink:       "/today",
		EvidenceHashes: []string{receipt.ReceiptID, receipt.StatusHash},
	}

	// Build labels from counts
	page.SentLabel = interruptdelivery.MagnitudeLabel(interruptdelivery.MagnitudeFromCount(receipt.SentCount))
	page.SkippedLabel = interruptdelivery.MagnitudeLabel(interruptdelivery.MagnitudeFromCount(receipt.SkippedCount + receipt.DedupedCount))

	// Set title/subtitle/lines based on outcome
	if receipt.SentCount > 0 {
		page.Title = "Delivered, quietly."
		page.Subtitle = "Something was sent — with restraint."
		page.Lines = []string{
			"A single abstract message was delivered.",
			"No details were included.",
			"Your boundaries were respected.",
		}
	} else if receipt.SkippedCount > 0 || receipt.DedupedCount > 0 {
		page.Title = "Not delivered."
		page.Subtitle = "Nothing was sent this period."
		page.Lines = e.buildSkippedLines(receipt)
	} else {
		page.Title = "Nothing to deliver."
		page.Subtitle = "No interrupt candidates this period."
		page.Lines = []string{
			"Silence is the success state.",
			"We only interrupt when truly needed.",
		}
	}

	page.StatusHash = page.ComputeStatusHash()
	return page
}

// buildSkippedLines builds calm copy for skipped deliveries.
func (e *Engine) buildSkippedLines(receipt *interruptdelivery.DeliveryReceipt) []string {
	if len(receipt.Attempts) == 0 {
		return []string{"No delivery was attempted."}
	}

	// Find the primary reason
	for _, a := range receipt.Attempts {
		switch a.ReasonBucket {
		case interruptdelivery.ReasonCapReached:
			return []string{
				"Daily delivery limit reached.",
				"No more than two per day.",
				"Your attention is protected.",
			}
		case interruptdelivery.ReasonPolicyDenies:
			return []string{
				"Your policy does not permit delivery.",
				"This is respecting your boundaries.",
			}
		case interruptdelivery.ReasonNotConfigured:
			return []string{
				"Push delivery is not configured.",
				"You can enable it in settings.",
			}
		case interruptdelivery.ReasonTrustFragile:
			return []string{
				"Trust is being rebuilt.",
				"Interrupts are paused.",
			}
		case interruptdelivery.ReasonAlreadySent:
			return []string{
				"Already delivered this period.",
				"We don't repeat ourselves.",
			}
		}
	}

	return []string{"No push was sent."}
}

// ShouldShowDeliveryCue determines if the delivery cue should be shown.
// Returns true if a delivery occurred this period and hasn't been dismissed.
func (e *Engine) ShouldShowDeliveryCue(hasSentToday bool, isDismissed bool) bool {
	return hasSentToday && !isDismissed
}

// BuildDeliveryCue builds the delivery whisper cue.
func (e *Engine) BuildDeliveryCue(hasSentToday bool, isDismissed bool) *interruptdelivery.DeliveryCue {
	cue := interruptdelivery.DefaultDeliveryCue()

	if e.ShouldShowDeliveryCue(hasSentToday, isDismissed) {
		cue.Available = true
	}

	cue.StatusHash = cue.ComputeStatusHash()
	return cue
}

// CountSentAttempts counts sent attempts from a list.
func CountSentAttempts(attempts []*interruptdelivery.DeliveryAttempt) int {
	count := 0
	for _, a := range attempts {
		if a.ResultBucket == interruptdelivery.ResultSent {
			count++
		}
	}
	return count
}

// GetPriorAttemptMap builds a map of prior attempts for deduplication.
func GetPriorAttemptMap(attempts []*interruptdelivery.DeliveryAttempt) map[string]bool {
	m := make(map[string]bool)
	for _, a := range attempts {
		if a.ResultBucket == interruptdelivery.ResultSent {
			m[a.CandidateHash] = true
		}
	}
	return m
}
