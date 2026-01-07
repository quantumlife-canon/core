// Package interruptpolicy implements the Phase 33 Interrupt Permission Contract engine.
//
// The engine evaluates whether INTERRUPT_CANDIDATE decisions from Phase 32
// are permitted to interrupt based on user policy. This is policy evaluation
// only — NO delivery capability.
//
// CRITICAL INVARIANTS:
//   - NO interrupt delivery. No alerts. No messages. No external signals.
//   - Deterministic: same inputs => same outputs + same hashes.
//   - No goroutines. Clock injection required.
//   - No side effects. Pure evaluation.
//   - Commerce always blocked regardless of policy.
//   - Default stance: NO interrupts allowed.
//
// Reference: docs/ADR/ADR-0069-phase33-interrupt-permission-contract.md
package interruptpolicy

import (
	"sort"

	"quantumlife/pkg/domain/interruptpolicy"
)

// Engine evaluates interrupt permission using deterministic rules.
// CRITICAL: No side effects. Pure function. Same inputs => same outputs.
type Engine struct{}

// NewEngine creates a new permission engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Evaluate computes permission decisions for interrupt candidates.
// CRITICAL: Deterministic. Same input => same result.
//
// Permission Rules (in order):
// 1. If trust fragile => deny all (reason_trust_fragile)
// 2. If policy is nil or allow_none => deny all (reason_policy_denies)
// 3. If CircleType == commerce => deny always (reason_category_blocked)
// 4. If allow_humans_now => allow only (CircleType==human AND Horizon==now)
// 5. If allow_institutions_soon => allow only (CircleType==institution AND Horizon in {soon, now})
// 6. If allow_two_per_day => allow any eligible up to cap
// 7. Apply MaxPerDay cap: sort by CandidateHash asc, allow first N only
func (e *Engine) Evaluate(input *interruptpolicy.InterruptPermissionInput) *interruptpolicy.InterruptPermissionResult {
	result := &interruptpolicy.InterruptPermissionResult{
		Decisions:          make([]*interruptpolicy.InterruptPermissionDecision, 0),
		PermittedMagnitude: interruptpolicy.MagnitudeNothing,
		DeniedMagnitude:    interruptpolicy.MagnitudeNothing,
	}

	if input == nil {
		result.StatusHash = result.ComputeStatusHash()
		return result
	}

	result.InputHash = input.ComputeInputHash()

	// No candidates? Return empty result.
	if len(input.Candidates) == 0 {
		result.StatusHash = result.ComputeStatusHash()
		return result
	}

	// Get effective policy (default if nil)
	policy := input.Policy
	if policy == nil {
		policy = interruptpolicy.DefaultInterruptPolicy(input.CircleIDHash, input.PeriodKey, input.TimeBucket)
	}

	// Rule 1: Trust fragile => deny all
	if input.TrustFragile {
		for _, candidate := range input.Candidates {
			decision := e.buildDecision(candidate.CandidateHash, false, interruptpolicy.ReasonTrustFragile)
			result.Decisions = append(result.Decisions, decision)
		}
		result.DeniedMagnitude = interruptpolicy.MagnitudeFromCount(len(input.Candidates))
		result.StatusHash = result.ComputeStatusHash()
		return result
	}

	// Rule 2: Policy denies all
	if policy.Allowance == interruptpolicy.AllowNone {
		for _, candidate := range input.Candidates {
			decision := e.buildDecision(candidate.CandidateHash, false, interruptpolicy.ReasonPolicyDenies)
			result.Decisions = append(result.Decisions, decision)
		}
		result.DeniedMagnitude = interruptpolicy.MagnitudeFromCount(len(input.Candidates))
		result.StatusHash = result.ComputeStatusHash()
		return result
	}

	// Evaluate each candidate with category and horizon checks
	// First pass: determine eligibility (before rate limiting)
	type candidateEval struct {
		candidate *interruptpolicy.InterruptCandidate
		eligible  bool
		reason    interruptpolicy.ReasonBucket
	}

	evals := make([]candidateEval, len(input.Candidates))
	for i, candidate := range input.Candidates {
		eligible, reason := e.evaluateCandidate(candidate, policy)
		evals[i] = candidateEval{
			candidate: candidate,
			eligible:  eligible,
			reason:    reason,
		}
	}

	// Sort eligible candidates by CandidateHash for deterministic rate limiting
	eligibleIndices := make([]int, 0)
	for i, ev := range evals {
		if ev.eligible {
			eligibleIndices = append(eligibleIndices, i)
		}
	}

	// Sort by CandidateHash ascending
	sort.Slice(eligibleIndices, func(a, b int) bool {
		return evals[eligibleIndices[a]].candidate.CandidateHash < evals[eligibleIndices[b]].candidate.CandidateHash
	})

	// Apply rate limit: allow first N only
	maxAllowed := policy.MaxPerDay
	if maxAllowed > interruptpolicy.MaxInterruptsPerDay {
		maxAllowed = interruptpolicy.MaxInterruptsPerDay
	}

	allowedSet := make(map[string]bool)
	for i, idx := range eligibleIndices {
		if i < maxAllowed {
			allowedSet[evals[idx].candidate.CandidateHash] = true
		}
	}

	// Build final decisions
	permittedCount := 0
	deniedCount := 0

	for _, ev := range evals {
		var decision *interruptpolicy.InterruptPermissionDecision

		if !ev.eligible {
			// Already ineligible from category/horizon check
			decision = e.buildDecision(ev.candidate.CandidateHash, false, ev.reason)
			deniedCount++
		} else if allowedSet[ev.candidate.CandidateHash] {
			// Eligible and within rate limit
			decision = e.buildDecision(ev.candidate.CandidateHash, true, interruptpolicy.ReasonNone)
			permittedCount++
		} else {
			// Eligible but rate limited
			decision = e.buildDecision(ev.candidate.CandidateHash, false, interruptpolicy.ReasonRateLimited)
			deniedCount++
		}

		result.Decisions = append(result.Decisions, decision)
	}

	result.PermittedMagnitude = interruptpolicy.MagnitudeFromCount(permittedCount)
	result.DeniedMagnitude = interruptpolicy.MagnitudeFromCount(deniedCount)
	result.StatusHash = result.ComputeStatusHash()

	return result
}

// evaluateCandidate determines if a candidate is eligible based on category and horizon.
// Returns (eligible, reason).
func (e *Engine) evaluateCandidate(candidate *interruptpolicy.InterruptCandidate, policy *interruptpolicy.InterruptPolicy) (bool, interruptpolicy.ReasonBucket) {
	// Rule 3: Commerce always blocked
	if candidate.CircleType == interruptpolicy.CircleTypeCommerce {
		return false, interruptpolicy.ReasonCategoryBlocked
	}

	// Apply allowance-specific rules
	switch policy.Allowance {
	case interruptpolicy.AllowHumansNow:
		// Only human + now
		if candidate.CircleType != interruptpolicy.CircleTypeHuman {
			return false, interruptpolicy.ReasonCategoryMismatch
		}
		if candidate.Horizon != interruptpolicy.HorizonNow {
			return false, interruptpolicy.ReasonHorizonMismatch
		}
		return true, interruptpolicy.ReasonNone

	case interruptpolicy.AllowInstitutionsSoon:
		// Only institution + (soon or now)
		if candidate.CircleType != interruptpolicy.CircleTypeInstitution {
			return false, interruptpolicy.ReasonCategoryMismatch
		}
		if candidate.Horizon != interruptpolicy.HorizonSoon && candidate.Horizon != interruptpolicy.HorizonNow {
			return false, interruptpolicy.ReasonHorizonMismatch
		}
		return true, interruptpolicy.ReasonNone

	case interruptpolicy.AllowTwoPerDay:
		// Any eligible (non-commerce) up to cap
		// Commerce already blocked above
		return true, interruptpolicy.ReasonNone

	default:
		// Unknown allowance — deny
		return false, interruptpolicy.ReasonPolicyDenies
	}
}

// buildDecision constructs a permission decision with computed hash.
func (e *Engine) buildDecision(candidateHash string, allowed bool, reason interruptpolicy.ReasonBucket) *interruptpolicy.InterruptPermissionDecision {
	d := &interruptpolicy.InterruptPermissionDecision{
		CandidateHash: candidateHash,
		Allowed:       allowed,
		ReasonBucket:  reason,
	}
	d.DeterministicHash = d.ComputeDeterministicHash()
	return d
}

// BuildProofPage constructs the proof page from permission result.
// CRITICAL: No raw identifiers. Abstract buckets only.
func (e *Engine) BuildProofPage(
	result *interruptpolicy.InterruptPermissionResult,
	policy *interruptpolicy.InterruptPolicy,
	periodKey, circleIDHash string,
) *interruptpolicy.InterruptProofPage {
	page := interruptpolicy.DefaultInterruptProofPage(periodKey, circleIDHash)

	if result == nil {
		page.StatusHash = page.ComputeStatusHash()
		return page
	}

	page.PermittedMagnitude = result.PermittedMagnitude
	page.DeniedMagnitude = result.DeniedMagnitude

	// Build calm lines based on state
	page.Lines = e.buildProofLines(result.PermittedMagnitude, result.DeniedMagnitude)

	// Build policy summary
	page.PolicySummary = e.buildPolicySummary(policy)

	page.StatusHash = page.ComputeStatusHash()
	return page
}

// buildProofLines generates calm copy based on permission state.
func (e *Engine) buildProofLines(permitted, denied interruptpolicy.MagnitudeBucket) []string {
	lines := []string{}

	switch permitted {
	case interruptpolicy.MagnitudeNothing:
		lines = append(lines, "Interruptions are being held.")
	case interruptpolicy.MagnitudeAFew:
		lines = append(lines, "A few things could reach you, if needed.")
	case interruptpolicy.MagnitudeSeveral:
		lines = append(lines, "Several things could reach you, if needed.")
	}

	if denied != interruptpolicy.MagnitudeNothing {
		lines = append(lines, "Some pressure is being held back.")
	}

	lines = append(lines, "We will still ask before action.")

	return lines
}

// buildPolicySummary generates a calm summary of the policy.
func (e *Engine) buildPolicySummary(policy *interruptpolicy.InterruptPolicy) string {
	if policy == nil {
		return "Interruptions are off."
	}

	switch policy.Allowance {
	case interruptpolicy.AllowNone:
		return "Interruptions are off."
	case interruptpolicy.AllowHumansNow:
		return "Only immediate human matters may reach you."
	case interruptpolicy.AllowInstitutionsSoon:
		return "Only pressing institutional matters may reach you."
	case interruptpolicy.AllowTwoPerDay:
		return "Up to two things per day may reach you."
	default:
		return "Interruptions are off."
	}
}

// ShouldShowWhisperCue determines if the Phase 33 whisper cue should be shown.
// Returns true if:
// - There are INTERRUPT_CANDIDATE decisions (regardless of permission)
// - AND the cue has not been dismissed for this period
func (e *Engine) ShouldShowWhisperCue(
	candidateCount int,
	dismissed bool,
) bool {
	// No candidates => no cue
	if candidateCount == 0 {
		return false
	}

	// Already dismissed => no cue
	if dismissed {
		return false
	}

	return true
}

// BuildWhisperCue builds the Phase 33 whisper cue.
func (e *Engine) BuildWhisperCue(visible bool) *interruptpolicy.InterruptWhisperCue {
	cue := interruptpolicy.DefaultInterruptWhisperCue()
	cue.Visible = visible
	return cue
}

// CountPermitted counts permitted decisions.
func CountPermitted(decisions []*interruptpolicy.InterruptPermissionDecision) int {
	count := 0
	for _, d := range decisions {
		if d != nil && d.Allowed {
			count++
		}
	}
	return count
}

// CountDenied counts denied decisions.
func CountDenied(decisions []*interruptpolicy.InterruptPermissionDecision) int {
	count := 0
	for _, d := range decisions {
		if d != nil && !d.Allowed {
			count++
		}
	}
	return count
}

// FilterByReason filters decisions by reason bucket.
func FilterByReason(decisions []*interruptpolicy.InterruptPermissionDecision, reason interruptpolicy.ReasonBucket) []*interruptpolicy.InterruptPermissionDecision {
	var result []*interruptpolicy.InterruptPermissionDecision
	for _, d := range decisions {
		if d != nil && d.ReasonBucket == reason {
			result = append(result, d)
		}
	}
	return result
}
