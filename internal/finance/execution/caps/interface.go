// Package caps implements v9.11 daily caps and rate-limited execution ledger.
//
// CRITICAL: This package enforces hard limits on financial execution:
// - Per-circle daily caps (by currency)
// - Per-intersection daily caps (by currency)
// - Per-payee daily caps (by currency)
// - Rate limits: maximum attempts per day
//
// All enforcement is deterministic, auditable, and applied BEFORE provider
// Prepare/Execute is invoked. Caps are hard blocks with no partial execution.
//
// Reference: ADR-0014, Canon Addendum v9
package caps

import (
	"context"

	"quantumlife/pkg/clock"
)

// Mode represents the execution mode.
type Mode string

const (
	// ModeExecute indicates real execution.
	ModeExecute Mode = "execute"

	// ModeSimulate indicates simulated execution (no real money movement).
	ModeSimulate Mode = "simulate"
)

// Policy defines the caps and rate limits for execution.
// All caps are per-currency to prevent cross-currency aggregation errors.
type Policy struct {
	// Enabled controls whether caps enforcement is active.
	// If false, all checks pass (unlimited).
	Enabled bool

	// PerCircleDailyCapCents maps currency -> daily cap in cents.
	// If a currency is not present, no cap for that currency.
	PerCircleDailyCapCents map[string]int64

	// PerIntersectionDailyCapCents maps currency -> daily cap in cents.
	// Applied only when IntersectionID is non-empty.
	PerIntersectionDailyCapCents map[string]int64

	// PerPayeeDailyCapCents maps currency -> daily cap in cents.
	PerPayeeDailyCapCents map[string]int64

	// MaxAttemptsPerDayCircle is the maximum number of execution attempts
	// per circle per day. 0 means unlimited.
	MaxAttemptsPerDayCircle int

	// MaxAttemptsPerDayIntersection is the maximum number of execution attempts
	// per intersection per day. 0 means unlimited.
	// Applied only when IntersectionID is non-empty.
	MaxAttemptsPerDayIntersection int

	// Notes is optional metadata for audit purposes only.
	// Never shown to users.
	Notes string
}

// Context provides all information needed for caps evaluation.
type Context struct {
	// Clock provides deterministic time.
	Clock clock.Clock

	// CircleID is the circle executing the payment.
	CircleID string

	// IntersectionID is the shared context (optional).
	// Empty string means no intersection context.
	IntersectionID string

	// PayeeID is the registered payee identifier.
	PayeeID string

	// Currency is the ISO 4217 currency code.
	Currency string

	// AmountCents is the amount to be executed.
	AmountCents int64

	// AttemptID uniquely identifies this execution attempt.
	AttemptID string

	// EnvelopeID is the execution envelope identifier.
	EnvelopeID string

	// ActionHash is the hash of the action being executed.
	ActionHash string

	// ProviderID is the write provider being used.
	ProviderID string

	// Mode is the execution mode (execute or simulate).
	Mode Mode
}

// Gate is the interface for caps and rate limit enforcement.
//
// The gate is called in the following order:
// 1. Check() - verify caps allow execution
// 2. OnAttemptStarted() - record attempt (increments attempt counter)
// 3. [execution happens]
// 4. OnAttemptFinalized() - record outcome (increments spend if money moved)
type Gate interface {
	// Check verifies that the execution is within caps and rate limits.
	// Returns a Result indicating whether execution is allowed.
	// Does NOT modify any state.
	Check(ctx context.Context, c Context) (*Result, error)

	// OnAttemptStarted is called after all preconditions are verified
	// and execution is about to proceed. Increments attempt counters.
	// Must be idempotent for the same AttemptID.
	OnAttemptStarted(ctx context.Context, c Context) error

	// OnAttemptFinalized is called exactly once when the attempt reaches
	// a terminal state. Increments spend counters if money moved.
	OnAttemptFinalized(ctx context.Context, c Context, finalized Finalized) error

	// GetPolicy returns the current caps policy.
	// v9.12: Used for policy snapshot computation.
	GetPolicy() Policy
}

// Result contains the outcome of a caps check.
type Result struct {
	// Allowed is true if execution may proceed.
	Allowed bool

	// Reasons contains neutral explanations for blocking.
	// Empty if Allowed is true.
	Reasons []string

	// RemainingCents is the remaining cap for the primary scope.
	// Optional; may be 0 if not applicable.
	RemainingCents int64

	// RemainingAttempts is the remaining attempts for the primary scope.
	// Optional; may be 0 if not applicable.
	RemainingAttempts int

	// ScopeChecks contains detailed results for each scope checked.
	// v9.11.1: Used by executor for granular audit events.
	ScopeChecks []ScopeCheckResult

	// DayKey is the UTC day key used for caps evaluation.
	// v9.11.1: Included in audit events for reproducibility.
	DayKey string
}

// ScopeCheckResult contains the result of a single scope check.
// Used for detailed per-scope audit events.
type ScopeCheckResult struct {
	// ScopeType is the type of scope checked.
	ScopeType ScopeType

	// ScopeID is the identifier of the scope.
	ScopeID string

	// CheckType is "cap" for spend caps or "ratelimit" for attempt limits.
	CheckType string

	// Currency is the currency for cap checks (empty for rate limits).
	Currency string

	// CurrentValue is the current spend cents or current attempts.
	CurrentValue int64

	// LimitValue is the cap cents or max attempts.
	LimitValue int64

	// RequestedValue is the amount requested (for caps) or 1 (for attempts).
	RequestedValue int64

	// Allowed is true if this specific check passed.
	Allowed bool

	// Reason is a neutral explanation for blocking (empty if allowed).
	Reason string
}

// CheckType constants for ScopeCheckResult.
const (
	CheckTypeCap       = "cap"
	CheckTypeRateLimit = "ratelimit"
)

// Finalized describes the terminal state of an execution attempt.
type Finalized struct {
	// Status is the terminal status of the attempt.
	// One of: blocked, aborted, revoked, expired, simulated, succeeded, failed
	Status string

	// MoneyMoved is true if real money was transferred.
	// Only true for succeeded status with non-simulated execution.
	MoneyMoved bool

	// AmountMovedCents is the actual amount transferred.
	// 0 if MoneyMoved is false.
	AmountMovedCents int64

	// Currency is the currency of the amount moved.
	Currency string
}

// ScopeType identifies the type of scope for caps tracking.
type ScopeType string

const (
	// ScopeCircle is a circle-level scope.
	ScopeCircle ScopeType = "circle"

	// ScopeIntersection is an intersection-level scope.
	ScopeIntersection ScopeType = "intersection"

	// ScopePayee is a payee-level scope.
	ScopePayee ScopeType = "payee"
)

// FinalizedStatus constants for terminal states.
const (
	StatusBlocked   = "blocked"
	StatusAborted   = "aborted"
	StatusRevoked   = "revoked"
	StatusExpired   = "expired"
	StatusSimulated = "simulated"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)
