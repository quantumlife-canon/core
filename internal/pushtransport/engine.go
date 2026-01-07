// Package pushtransport implements the Phase 35 Push Transport engine.
//
// The engine computes delivery eligibility and prepares transport requests.
// It does NOT perform network calls — those are done in cmd/ layer.
//
// CRITICAL INVARIANTS:
//   - Transport-only. No new decision logic.
//   - Does NOT perform network calls. Returns TransportRequest for cmd/.
//   - Deterministic: same inputs => same outputs + same hashes.
//   - No goroutines. No time.Now().
//   - Commerce never interrupts.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package pushtransport

import (
	"quantumlife/pkg/domain/pushtransport"
)

// Engine computes push delivery eligibility and prepares transport requests.
// CRITICAL: Does NOT perform network calls. Pure function.
type Engine struct{}

// NewEngine creates a new push transport engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ComputeDeliveryAttempt computes a delivery attempt from eligibility input.
// Returns the attempt record and an optional transport request.
// If transport request is nil, delivery was skipped.
func (e *Engine) ComputeDeliveryAttempt(input *pushtransport.DeliveryEligibilityInput) (*pushtransport.PushDeliveryAttempt, *pushtransport.TransportRequest) {
	if input == nil {
		return e.skippedAttempt("", "", "", "", pushtransport.FailureNoCandidate), nil
	}

	// Create base attempt
	attempt := &pushtransport.PushDeliveryAttempt{
		CircleIDHash:  input.CircleIDHash,
		CandidateHash: input.CandidateHash,
		PeriodKey:     input.PeriodKey,
		AttemptBucket: input.TimeBucket,
	}

	// Check eligibility conditions in order

	// 1. Must have a candidate
	if !input.HasCandidate || input.CandidateHash == "" {
		attempt.Status = pushtransport.StatusSkipped
		attempt.FailureBucket = pushtransport.FailureNoCandidate
		attempt.ProviderKind = pushtransport.ProviderStub
		attempt.AttemptID = attempt.ComputeAttemptID()
		attempt.StatusHash = attempt.ComputeStatusHash()
		return attempt, nil
	}

	// 2. Must have policy enabled
	if !input.PolicyEnabled {
		attempt.Status = pushtransport.StatusSkipped
		attempt.FailureBucket = pushtransport.FailureNotPermitted
		attempt.ProviderKind = pushtransport.ProviderStub
		attempt.AttemptID = attempt.ComputeAttemptID()
		attempt.StatusHash = attempt.ComputeStatusHash()
		return attempt, nil
	}

	// 3. Must have registration
	if !input.HasRegistration || input.Registration == nil {
		attempt.Status = pushtransport.StatusSkipped
		attempt.FailureBucket = pushtransport.FailureNotConfigured
		attempt.ProviderKind = pushtransport.ProviderStub
		attempt.AttemptID = attempt.ComputeAttemptID()
		attempt.StatusHash = attempt.ComputeStatusHash()
		return attempt, nil
	}

	// 4. Must have push enabled in registration
	if !input.PushEnabled || !input.Registration.Enabled {
		attempt.Status = pushtransport.StatusSkipped
		attempt.FailureBucket = pushtransport.FailureNotConfigured
		attempt.ProviderKind = input.Registration.ProviderKind
		attempt.AttemptID = attempt.ComputeAttemptID()
		attempt.StatusHash = attempt.ComputeStatusHash()
		return attempt, nil
	}

	// 5. Must not exceed daily cap
	maxPerDay := input.MaxPerDay
	if maxPerDay <= 0 {
		maxPerDay = pushtransport.DefaultMaxPushPerDay
	}
	if input.DailyAttemptCount >= maxPerDay {
		attempt.Status = pushtransport.StatusSkipped
		attempt.FailureBucket = pushtransport.FailureCapReached
		attempt.ProviderKind = input.Registration.ProviderKind
		attempt.AttemptID = attempt.ComputeAttemptID()
		attempt.StatusHash = attempt.ComputeStatusHash()
		return attempt, nil
	}

	// All checks passed — prepare for delivery
	attempt.Status = pushtransport.StatusSent
	attempt.FailureBucket = pushtransport.FailureNone
	attempt.ProviderKind = input.Registration.ProviderKind
	attempt.AttemptID = attempt.ComputeAttemptID()
	attempt.StatusHash = attempt.ComputeStatusHash()

	// Build transport request
	request := &pushtransport.TransportRequest{
		ProviderKind: input.Registration.ProviderKind,
		TokenHash:    input.Registration.TokenHash,
		Payload:      pushtransport.DefaultTransportPayload(attempt.StatusHash),
		AttemptID:    attempt.AttemptID,
	}

	return attempt, request
}

// skippedAttempt creates a skipped attempt with the given reason.
func (e *Engine) skippedAttempt(circleIDHash, candidateHash, periodKey, timeBucket string, reason pushtransport.FailureBucket) *pushtransport.PushDeliveryAttempt {
	attempt := &pushtransport.PushDeliveryAttempt{
		CircleIDHash:  circleIDHash,
		CandidateHash: candidateHash,
		ProviderKind:  pushtransport.ProviderStub,
		Status:        pushtransport.StatusSkipped,
		FailureBucket: reason,
		PeriodKey:     periodKey,
		AttemptBucket: timeBucket,
	}
	attempt.AttemptID = attempt.ComputeAttemptID()
	attempt.StatusHash = attempt.ComputeStatusHash()
	return attempt
}

// BuildProofPage builds the proof page for a delivery attempt.
func (e *Engine) BuildProofPage(attempt *pushtransport.PushDeliveryAttempt, periodKey string) *pushtransport.PushDeliveryReceiptPage {
	if attempt == nil {
		return pushtransport.DefaultPushProofPage(periodKey)
	}

	page := &pushtransport.PushDeliveryReceiptPage{
		Status:        attempt.Status,
		FailureBucket: attempt.FailureBucket,
		PeriodKey:     periodKey,
		BackLink:      "/today",
	}

	// Build title and lines based on status
	switch attempt.Status {
	case pushtransport.StatusSent:
		page.Title = "Delivered, quietly."
		page.Subtitle = "A single abstract message was sent."
		page.Lines = []string{
			"Something needed your attention.",
			"A single push was delivered.",
			"No details were included.",
		}
	case pushtransport.StatusSkipped:
		page.Title = "Not delivered."
		page.Subtitle = "No push was sent this period."
		page.Lines = e.buildSkippedLines(attempt.FailureBucket)
	case pushtransport.StatusFailed:
		page.Title = "Delivery issue."
		page.Subtitle = "There was a transport problem."
		page.Lines = []string{
			"Delivery was attempted but could not complete.",
			"This is not your fault.",
			"The system will try again if appropriate.",
		}
	default:
		page.Title = "Unknown status."
		page.Subtitle = "Unable to determine delivery status."
		page.Lines = []string{"Please check back later."}
	}

	// Add evidence hashes
	page.EvidenceHashes = []string{attempt.AttemptID, attempt.StatusHash}

	page.StatusHash = page.ComputeStatusHash()
	return page
}

// buildSkippedLines builds calm copy for skipped deliveries.
func (e *Engine) buildSkippedLines(reason pushtransport.FailureBucket) []string {
	switch reason {
	case pushtransport.FailureNoCandidate:
		return []string{
			"Nothing needed your attention this period.",
			"Silence is the success state.",
		}
	case pushtransport.FailureNotConfigured:
		return []string{
			"Push delivery is not configured.",
			"You can enable it in settings.",
		}
	case pushtransport.FailureNotPermitted:
		return []string{
			"Your interrupt policy does not permit delivery.",
			"This is respecting your boundaries.",
		}
	case pushtransport.FailureCapReached:
		return []string{
			"Daily delivery limit reached.",
			"No more than two per day.",
			"Your attention is protected.",
		}
	default:
		return []string{
			"No push was sent.",
		}
	}
}

// ShouldShowProofCue determines if the proof cue should be shown.
// Returns false if no attempt has been made today.
func (e *Engine) ShouldShowProofCue(hasAttemptToday bool) bool {
	return hasAttemptToday
}

// CountSentToday counts sent attempts for the current period.
func CountSentToday(attempts []*pushtransport.PushDeliveryAttempt) int {
	count := 0
	for _, a := range attempts {
		if a.Status == pushtransport.StatusSent {
			count++
		}
	}
	return count
}
