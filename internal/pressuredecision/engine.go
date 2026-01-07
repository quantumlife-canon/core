// Package pressuredecision implements the Phase 32 Pressure Decision Gate.
//
// The decision gate classifies external pressure into one of three states:
// - HOLD (default): Pressure acknowledged but not surfaced
// - SURFACE: Pressure may appear in calm mirror views
// - INTERRUPT_CANDIDATE: Pressure may compete for interruption (rare)
//
// CRITICAL INVARIANTS:
//   - Classification only. NO notifications. NO execution.
//   - Deterministic: same inputs => same outputs.
//   - No goroutines. Clock injection required.
//   - Rules applied in order, first match wins.
//   - Max 2 interrupt candidates per day.
//
// Reference: docs/ADR/ADR-0068-phase32-pressure-decision-gate.md
package pressuredecision

import (
	"quantumlife/pkg/domain/pressuredecision"
)

// Engine computes pressure decisions using deterministic rules.
// CRITICAL: No side effects. Pure function. Same inputs => same outputs.
type Engine struct{}

// NewEngine creates a new decision engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Classify computes a pressure decision from input.
// CRITICAL: Deterministic. Same input => same decision.
// CRITICAL: Rules applied in order, first match wins.
//
// Rule order:
// 0. Default: HOLD
// 1. Commerce never interrupts: CircleType==commerce => HOLD
// 2. Human + NOW: CircleType==human && Horizon==now && Magnitude!=nothing => INTERRUPT_CANDIDATE
// 3. Institution + Deadline: CircleType==institution && Horizon==soon && Magnitude==several => SURFACE
// 4. Trust fragile: max decision = SURFACE
// 5. Rate limit: interrupt_candidates >= 2 => downgrade to SURFACE
func (e *Engine) Classify(input *pressuredecision.PressureDecisionInput) *pressuredecision.PressureDecision {
	if input == nil {
		return e.defaultHold("", "", "")
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return e.defaultHold(input.CircleIDHash, input.PeriodKey, input.ComputeHash())
	}

	inputHash := input.ComputeHash()

	// Rule 0: Default is HOLD
	decision := pressuredecision.DecisionHold
	reason := pressuredecision.ReasonDefault

	// Rule 1: Commerce never interrupts alone
	if input.CircleType == pressuredecision.CircleTypeCommerce {
		decision = pressuredecision.DecisionHold
		reason = pressuredecision.ReasonCommerceNeverInterrupts
		return e.buildDecision(input.CircleIDHash, decision, reason, input.PeriodKey, inputHash)
	}

	// Rule 2: Human + Horizon NOW + non-trivial magnitude => INTERRUPT_CANDIDATE
	if input.CircleType == pressuredecision.CircleTypeHuman &&
		input.Horizon == pressuredecision.HorizonNow &&
		input.Magnitude != pressuredecision.MagnitudeNothing {
		decision = pressuredecision.DecisionInterruptCandidate
		reason = pressuredecision.ReasonHumanNow
	}

	// Rule 3: Institution + Horizon SOON + Magnitude SEVERAL => SURFACE
	if input.CircleType == pressuredecision.CircleTypeInstitution &&
		input.Horizon == pressuredecision.HorizonSoon &&
		input.Magnitude == pressuredecision.MagnitudeSeveral {
		decision = pressuredecision.DecisionSurface
		reason = pressuredecision.ReasonInstitutionDeadline
	}

	// Rule: No magnitude => HOLD
	if input.Magnitude == pressuredecision.MagnitudeNothing {
		decision = pressuredecision.DecisionHold
		reason = pressuredecision.ReasonNoMagnitude
		return e.buildDecision(input.CircleIDHash, decision, reason, input.PeriodKey, inputHash)
	}

	// Rule: Horizon LATER => HOLD (pressure too far out)
	if input.Horizon == pressuredecision.HorizonLater {
		decision = pressuredecision.DecisionHold
		reason = pressuredecision.ReasonHorizonLater
		return e.buildDecision(input.CircleIDHash, decision, reason, input.PeriodKey, inputHash)
	}

	// Rule 4: Trust fragile protection - cap at SURFACE
	if input.TrustStatus == pressuredecision.TrustStatusFragile {
		if decision == pressuredecision.DecisionInterruptCandidate {
			decision = pressuredecision.DecisionSurface
			reason = pressuredecision.ReasonTrustFragileDowngrade
		}
	}

	// Rule 5: Rate limit - max 2 interrupt candidates per day
	if decision == pressuredecision.DecisionInterruptCandidate {
		if input.InterruptCandidatesToday >= pressuredecision.MaxInterruptCandidatesPerDay {
			decision = pressuredecision.DecisionSurface
			reason = pressuredecision.ReasonRateLimitDowngrade
		}
	}

	return e.buildDecision(input.CircleIDHash, decision, reason, input.PeriodKey, inputHash)
}

// ClassifyBatch computes decisions for multiple inputs.
// Returns a batch with aggregated counts.
func (e *Engine) ClassifyBatch(inputs []*pressuredecision.PressureDecisionInput, periodKey string) *pressuredecision.DecisionBatch {
	batch := pressuredecision.NewDecisionBatch(periodKey)

	// Track interrupt candidates for rate limiting within batch
	interruptCount := 0

	for _, input := range inputs {
		if input == nil {
			continue
		}

		// Update rate limit count from previous decisions in this batch
		input.InterruptCandidatesToday += interruptCount

		decision := e.Classify(input)
		batch.AddDecision(decision)

		if decision.Decision == pressuredecision.DecisionInterruptCandidate {
			interruptCount++
		}
	}

	return batch
}

// buildDecision constructs a decision with computed hashes.
func (e *Engine) buildDecision(
	circleIDHash string,
	decision pressuredecision.PressureDecisionKind,
	reason pressuredecision.ReasonBucket,
	periodKey string,
	inputHash string,
) *pressuredecision.PressureDecision {
	d := &pressuredecision.PressureDecision{
		CircleIDHash: circleIDHash,
		Decision:     decision,
		ReasonBucket: reason,
		PeriodKey:    periodKey,
		InputHash:    inputHash,
	}
	d.StatusHash = d.ComputeStatusHash()
	d.DecisionID = d.ComputeDecisionID()
	return d
}

// defaultHold returns a default HOLD decision.
func (e *Engine) defaultHold(circleIDHash, periodKey, inputHash string) *pressuredecision.PressureDecision {
	return e.buildDecision(
		circleIDHash,
		pressuredecision.DecisionHold,
		pressuredecision.ReasonDefault,
		periodKey,
		inputHash,
	)
}

// ShouldPersist returns whether a decision should be persisted.
// HOLD decisions are not persisted (silence leaves no trace).
func (e *Engine) ShouldPersist(d *pressuredecision.PressureDecision) bool {
	if d == nil {
		return false
	}
	// Only persist non-HOLD decisions
	return d.Decision != pressuredecision.DecisionHold
}

// CountInterruptCandidates counts interrupt candidates in a list of decisions.
func CountInterruptCandidates(decisions []*pressuredecision.PressureDecision) int {
	count := 0
	for _, d := range decisions {
		if d != nil && d.Decision == pressuredecision.DecisionInterruptCandidate {
			count++
		}
	}
	return count
}

// FilterByDecision filters decisions by kind.
func FilterByDecision(decisions []*pressuredecision.PressureDecision, kind pressuredecision.PressureDecisionKind) []*pressuredecision.PressureDecision {
	var result []*pressuredecision.PressureDecision
	for _, d := range decisions {
		if d != nil && d.Decision == kind {
			result = append(result, d)
		}
	}
	return result
}

// DowngradeToSurface downgrades a decision to SURFACE if it's INTERRUPT_CANDIDATE.
// Returns a new decision (does not mutate input).
func (e *Engine) DowngradeToSurface(d *pressuredecision.PressureDecision, reason pressuredecision.ReasonBucket) *pressuredecision.PressureDecision {
	if d == nil || d.Decision != pressuredecision.DecisionInterruptCandidate {
		return d
	}

	return e.buildDecision(
		d.CircleIDHash,
		pressuredecision.DecisionSurface,
		reason,
		d.PeriodKey,
		d.InputHash,
	)
}
