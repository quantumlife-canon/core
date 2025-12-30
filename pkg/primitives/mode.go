// Package primitives provides core domain types.
// This file defines run modes for the system.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §4 Control Plane vs Data Plane
package primitives

import "errors"

// RunMode defines the execution mode for the system.
type RunMode string

const (
	// ModeSuggestOnly means no Action is created; only suggestions are made.
	// This is the safest mode with zero side effects.
	ModeSuggestOnly RunMode = "suggest_only"

	// ModeSimulate means Action is created and simulated execution occurs,
	// but no external side effects happen. Settlement and memory updates are recorded.
	ModeSimulate RunMode = "simulate"

	// ModeExecute means Action is created and real execution occurs.
	// NOT IMPLEMENTED - will hard fail if used.
	ModeExecute RunMode = "execute"
)

// ErrExecuteNotImplemented is returned when ModeExecute is requested.
var ErrExecuteNotImplemented = errors.New("execute mode is not implemented; use simulate or suggest_only")

// ErrInvalidRunMode is returned when an unknown mode is specified.
var ErrInvalidRunMode = errors.New("invalid run mode")

// ValidateRunMode checks if the mode is valid and implemented.
func ValidateRunMode(mode RunMode) error {
	switch mode {
	case ModeSuggestOnly:
		return nil
	case ModeSimulate:
		return nil
	case ModeExecute:
		return ErrExecuteNotImplemented
	default:
		return ErrInvalidRunMode
	}
}

// String returns the string representation of the mode.
func (m RunMode) String() string {
	return string(m)
}

// AllowsAction returns true if this mode allows Action creation.
func (m RunMode) AllowsAction() bool {
	return m == ModeSimulate || m == ModeExecute
}

// AllowsExecution returns true if this mode allows execution (simulated or real).
func (m RunMode) AllowsExecution() bool {
	return m == ModeSimulate || m == ModeExecute
}

// IsSimulated returns true if this mode only simulates execution.
func (m RunMode) IsSimulated() bool {
	return m == ModeSimulate
}
