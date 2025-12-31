package caps

import (
	"context"
	"fmt"

	"quantumlife/pkg/events"
)

// DefaultGate implements the Gate interface using an in-memory store.
type DefaultGate struct {
	store   *Store
	policy  Policy
	emitter func(events.Event)
}

// NewDefaultGate creates a new caps gate with the given policy.
func NewDefaultGate(policy Policy, emitter func(events.Event)) *DefaultGate {
	return &DefaultGate{
		store:   NewStore(),
		policy:  policy,
		emitter: emitter,
	}
}

// NewDefaultGateWithStore creates a gate with a custom store (for testing).
func NewDefaultGateWithStore(policy Policy, store *Store, emitter func(events.Event)) *DefaultGate {
	return &DefaultGate{
		store:   store,
		policy:  policy,
		emitter: emitter,
	}
}

// Check verifies that the execution is within caps and rate limits.
func (g *DefaultGate) Check(ctx context.Context, c Context) (*Result, error) {
	// If caps not enabled, allow everything
	if !g.policy.Enabled {
		return &Result{Allowed: true}, nil
	}

	dayKey := DayKey(c.Clock)
	result := &Result{Allowed: true}

	// Check circle daily cap
	if cap, ok := g.policy.PerCircleDailyCapCents[c.Currency]; ok {
		counters := g.store.GetCounters(dayKey, ScopeCircle, c.CircleID, c.Currency)
		if counters.MoneyMovedCents+c.AmountCents > cap {
			result.Allowed = false
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("circle daily cap reached (%d/%d cents %s)",
					counters.MoneyMovedCents, cap, c.Currency))
			result.RemainingCents = cap - counters.MoneyMovedCents
			if result.RemainingCents < 0 {
				result.RemainingCents = 0
			}
		} else {
			result.RemainingCents = cap - counters.MoneyMovedCents - c.AmountCents
		}
	}

	// Check intersection daily cap (if intersection context exists)
	if c.IntersectionID != "" {
		if cap, ok := g.policy.PerIntersectionDailyCapCents[c.Currency]; ok {
			counters := g.store.GetCounters(dayKey, ScopeIntersection, c.IntersectionID, c.Currency)
			if counters.MoneyMovedCents+c.AmountCents > cap {
				result.Allowed = false
				result.Reasons = append(result.Reasons,
					fmt.Sprintf("intersection daily cap reached (%d/%d cents %s)",
						counters.MoneyMovedCents, cap, c.Currency))
			}
		}
	}

	// Check payee daily cap
	if cap, ok := g.policy.PerPayeeDailyCapCents[c.Currency]; ok {
		counters := g.store.GetCounters(dayKey, ScopePayee, c.PayeeID, c.Currency)
		if counters.MoneyMovedCents+c.AmountCents > cap {
			result.Allowed = false
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("payee daily cap reached (%d/%d cents %s)",
					counters.MoneyMovedCents, cap, c.Currency))
		}
	}

	// Check circle attempt limit
	if g.policy.MaxAttemptsPerDayCircle > 0 {
		attempts := g.store.GetAttemptCount(dayKey, ScopeCircle, c.CircleID)
		if attempts >= g.policy.MaxAttemptsPerDayCircle {
			result.Allowed = false
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("circle daily attempt limit reached (%d/%d)",
					attempts, g.policy.MaxAttemptsPerDayCircle))
			result.RemainingAttempts = 0
		} else {
			result.RemainingAttempts = g.policy.MaxAttemptsPerDayCircle - attempts
		}
	}

	// Check intersection attempt limit (if intersection context exists)
	if c.IntersectionID != "" && g.policy.MaxAttemptsPerDayIntersection > 0 {
		attempts := g.store.GetAttemptCount(dayKey, ScopeIntersection, c.IntersectionID)
		if attempts >= g.policy.MaxAttemptsPerDayIntersection {
			result.Allowed = false
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("intersection daily attempt limit reached (%d/%d)",
					attempts, g.policy.MaxAttemptsPerDayIntersection))
		}
	}

	return result, nil
}

// OnAttemptStarted records that an execution attempt has started.
func (g *DefaultGate) OnAttemptStarted(ctx context.Context, c Context) error {
	if !g.policy.Enabled {
		return nil
	}

	dayKey := DayKey(c.Clock)

	// Increment attempt counters for each scope
	// These are idempotent per attemptID

	g.store.IncrementAttempt(dayKey, ScopeCircle, c.CircleID, c.Currency, c.AttemptID)

	if c.IntersectionID != "" {
		g.store.IncrementAttempt(dayKey, ScopeIntersection, c.IntersectionID, c.Currency, c.AttemptID)
	}

	g.store.IncrementAttempt(dayKey, ScopePayee, c.PayeeID, c.Currency, c.AttemptID)

	return nil
}

// OnAttemptFinalized records the outcome of an execution attempt.
func (g *DefaultGate) OnAttemptFinalized(ctx context.Context, c Context, finalized Finalized) error {
	if !g.policy.Enabled {
		return nil
	}

	dayKey := DayKey(c.Clock)

	// Only increment spend if money actually moved
	if finalized.MoneyMoved && finalized.AmountMovedCents > 0 {
		g.store.IncrementSpend(dayKey, ScopeCircle, c.CircleID, finalized.Currency, finalized.AmountMovedCents)

		if c.IntersectionID != "" {
			g.store.IncrementSpend(dayKey, ScopeIntersection, c.IntersectionID, finalized.Currency, finalized.AmountMovedCents)
		}

		g.store.IncrementSpend(dayKey, ScopePayee, c.PayeeID, finalized.Currency, finalized.AmountMovedCents)
	}

	return nil
}

// emit sends an event if emitter is configured.
func (g *DefaultGate) emit(e events.Event) {
	if g.emitter != nil {
		g.emitter(e)
	}
}

// GetStore returns the underlying store (for testing).
func (g *DefaultGate) GetStore() *Store {
	return g.store
}

// UpdatePolicy updates the gate's policy.
func (g *DefaultGate) UpdatePolicy(policy Policy) {
	g.policy = policy
}

// Verify interface compliance at compile time.
var _ Gate = (*DefaultGate)(nil)
