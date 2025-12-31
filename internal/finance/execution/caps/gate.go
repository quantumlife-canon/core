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
// v9.11.1: Returns detailed ScopeChecks for granular audit events.
func (g *DefaultGate) Check(ctx context.Context, c Context) (*Result, error) {
	// If caps not enabled, allow everything
	if !g.policy.Enabled {
		return &Result{Allowed: true}, nil
	}

	dayKey := DayKey(c.Clock)
	result := &Result{
		Allowed:     true,
		DayKey:      dayKey,
		ScopeChecks: make([]ScopeCheckResult, 0),
	}

	// Check circle daily cap
	if cap, ok := g.policy.PerCircleDailyCapCents[c.Currency]; ok {
		counters := g.store.GetCounters(dayKey, ScopeCircle, c.CircleID, c.Currency)
		wouldExceed := counters.MoneyMovedCents+c.AmountCents > cap

		scopeCheck := ScopeCheckResult{
			ScopeType:      ScopeCircle,
			ScopeID:        c.CircleID,
			CheckType:      CheckTypeCap,
			Currency:       c.Currency,
			CurrentValue:   counters.MoneyMovedCents,
			LimitValue:     cap,
			RequestedValue: c.AmountCents,
			Allowed:        !wouldExceed,
		}

		if wouldExceed {
			scopeCheck.Reason = fmt.Sprintf("circle daily cap reached (%d/%d cents %s)",
				counters.MoneyMovedCents, cap, c.Currency)
			result.Allowed = false
			result.Reasons = append(result.Reasons, scopeCheck.Reason)
			result.RemainingCents = cap - counters.MoneyMovedCents
			if result.RemainingCents < 0 {
				result.RemainingCents = 0
			}
		} else {
			result.RemainingCents = cap - counters.MoneyMovedCents - c.AmountCents
		}

		result.ScopeChecks = append(result.ScopeChecks, scopeCheck)
	}

	// Check intersection daily cap (if intersection context exists)
	if c.IntersectionID != "" {
		if cap, ok := g.policy.PerIntersectionDailyCapCents[c.Currency]; ok {
			counters := g.store.GetCounters(dayKey, ScopeIntersection, c.IntersectionID, c.Currency)
			wouldExceed := counters.MoneyMovedCents+c.AmountCents > cap

			scopeCheck := ScopeCheckResult{
				ScopeType:      ScopeIntersection,
				ScopeID:        c.IntersectionID,
				CheckType:      CheckTypeCap,
				Currency:       c.Currency,
				CurrentValue:   counters.MoneyMovedCents,
				LimitValue:     cap,
				RequestedValue: c.AmountCents,
				Allowed:        !wouldExceed,
			}

			if wouldExceed {
				scopeCheck.Reason = fmt.Sprintf("intersection daily cap reached (%d/%d cents %s)",
					counters.MoneyMovedCents, cap, c.Currency)
				result.Allowed = false
				result.Reasons = append(result.Reasons, scopeCheck.Reason)
			}

			result.ScopeChecks = append(result.ScopeChecks, scopeCheck)
		}
	}

	// Check payee daily cap
	if cap, ok := g.policy.PerPayeeDailyCapCents[c.Currency]; ok {
		counters := g.store.GetCounters(dayKey, ScopePayee, c.PayeeID, c.Currency)
		wouldExceed := counters.MoneyMovedCents+c.AmountCents > cap

		scopeCheck := ScopeCheckResult{
			ScopeType:      ScopePayee,
			ScopeID:        c.PayeeID,
			CheckType:      CheckTypeCap,
			Currency:       c.Currency,
			CurrentValue:   counters.MoneyMovedCents,
			LimitValue:     cap,
			RequestedValue: c.AmountCents,
			Allowed:        !wouldExceed,
		}

		if wouldExceed {
			scopeCheck.Reason = fmt.Sprintf("payee daily cap reached (%d/%d cents %s)",
				counters.MoneyMovedCents, cap, c.Currency)
			result.Allowed = false
			result.Reasons = append(result.Reasons, scopeCheck.Reason)
		}

		result.ScopeChecks = append(result.ScopeChecks, scopeCheck)
	}

	// Check circle attempt limit
	if g.policy.MaxAttemptsPerDayCircle > 0 {
		attempts := g.store.GetAttemptCount(dayKey, ScopeCircle, c.CircleID)
		atLimit := attempts >= g.policy.MaxAttemptsPerDayCircle

		scopeCheck := ScopeCheckResult{
			ScopeType:      ScopeCircle,
			ScopeID:        c.CircleID,
			CheckType:      CheckTypeRateLimit,
			Currency:       "", // Rate limits are not currency-specific
			CurrentValue:   int64(attempts),
			LimitValue:     int64(g.policy.MaxAttemptsPerDayCircle),
			RequestedValue: 1,
			Allowed:        !atLimit,
		}

		if atLimit {
			scopeCheck.Reason = fmt.Sprintf("circle daily attempt limit reached (%d/%d)",
				attempts, g.policy.MaxAttemptsPerDayCircle)
			result.Allowed = false
			result.Reasons = append(result.Reasons, scopeCheck.Reason)
			result.RemainingAttempts = 0
		} else {
			result.RemainingAttempts = g.policy.MaxAttemptsPerDayCircle - attempts
		}

		result.ScopeChecks = append(result.ScopeChecks, scopeCheck)
	}

	// Check intersection attempt limit (if intersection context exists)
	if c.IntersectionID != "" && g.policy.MaxAttemptsPerDayIntersection > 0 {
		attempts := g.store.GetAttemptCount(dayKey, ScopeIntersection, c.IntersectionID)
		atLimit := attempts >= g.policy.MaxAttemptsPerDayIntersection

		scopeCheck := ScopeCheckResult{
			ScopeType:      ScopeIntersection,
			ScopeID:        c.IntersectionID,
			CheckType:      CheckTypeRateLimit,
			Currency:       "",
			CurrentValue:   int64(attempts),
			LimitValue:     int64(g.policy.MaxAttemptsPerDayIntersection),
			RequestedValue: 1,
			Allowed:        !atLimit,
		}

		if atLimit {
			scopeCheck.Reason = fmt.Sprintf("intersection daily attempt limit reached (%d/%d)",
				attempts, g.policy.MaxAttemptsPerDayIntersection)
			result.Allowed = false
			result.Reasons = append(result.Reasons, scopeCheck.Reason)
		}

		result.ScopeChecks = append(result.ScopeChecks, scopeCheck)
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
