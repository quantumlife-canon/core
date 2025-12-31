// Package clock provides a deterministic clock abstraction for QuantumLife.
//
// GUARDRAIL: Core logic packages MUST NOT call time.Now() directly.
// Instead, inject a Clock interface to enable deterministic testing
// and prevent timezone/timing-related bugs in ceiling checks.
//
// Usage:
//
//	// In production code
//	type Service struct {
//	    clock clock.Clock
//	}
//
//	func NewService(c clock.Clock) *Service {
//	    return &Service{clock: c}
//	}
//
//	func (s *Service) DoWork() {
//	    now := s.clock.Now()  // deterministic
//	    // ...
//	}
//
//	// In tests
//	fixed := clock.NewFixed(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
//	svc := NewService(fixed)
//
// Reference: v9.6.2 Clock Guardrail
package clock

import "time"

// Clock provides the current time.
// All core logic should depend on this interface, not time.Now().
type Clock interface {
	Now() time.Time
}

// RealClock returns the actual system time.
// Use only at application entry points (cmd/*).
type RealClock struct{}

// Now returns the current system time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// FixedClock always returns a fixed time.
// Use for deterministic testing.
type FixedClock struct {
	T time.Time
}

// Now returns the fixed time.
func (c FixedClock) Now() time.Time {
	return c.T
}

// FuncClock wraps a function as a Clock.
// Useful for incremental time or custom test scenarios.
type FuncClock func() time.Time

// Now calls the wrapped function.
func (f FuncClock) Now() time.Time {
	return f()
}

// NewReal returns a Clock that uses the real system time.
// ONLY use at application entry points (cmd/*).
func NewReal() Clock {
	return RealClock{}
}

// NewFixed returns a Clock that always returns the given time.
// Use for deterministic testing.
func NewFixed(t time.Time) Clock {
	return FixedClock{T: t}
}

// NewFunc returns a Clock backed by a custom function.
// Useful for tests that need incrementing or dynamic time.
func NewFunc(f func() time.Time) Clock {
	return FuncClock(f)
}

// Verify interface compliance at compile time.
var (
	_ Clock = RealClock{}
	_ Clock = FixedClock{}
	_ Clock = FuncClock(nil)
)
