// Package execution provides v9 financial execution primitives.
//
// This file defines the ExecutionAdapter interface for provider-specific execution.
//
// CRITICAL: All adapters in v9 Slice 2 are GUARDED.
// They CANNOT move money. Execute() ALWAYS fails with GuardedExecutionError.
//
// Subordinate to:
// - docs/CANON_ADDENDUM_V9_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"fmt"
	"time"
)

// ExecutionAdapter defines the interface for provider-specific execution.
//
// CRITICAL: In v9 Slice 2, all implementations are GUARDED.
// Execute() MUST always fail with GuardedExecutionError.
// NO REAL MONEY MOVES.
type ExecutionAdapter interface {
	// Provider returns the provider name (e.g., "mock-finance", "plaid-stub").
	Provider() string

	// Prepare validates that the envelope can be executed.
	// This performs pre-execution checks WITHOUT side effects.
	// Returns error if envelope is invalid or cannot be executed.
	Prepare(envelope *ExecutionEnvelope) (*PrepareResult, error)

	// Execute attempts to execute the envelope.
	// CRITICAL: In v9 Slice 2, this ALWAYS fails with GuardedExecutionError.
	// NO REAL MONEY MOVES. NO SIDE EFFECTS.
	Execute(envelope *ExecutionEnvelope, approval *ApprovalArtifact) (*ExecutionAttempt, error)
}

// PrepareResult contains the result of preparing an execution.
type PrepareResult struct {
	// EnvelopeID is the envelope being prepared.
	EnvelopeID string

	// Provider is the adapter provider name.
	Provider string

	// PreparedAt is when preparation completed.
	PreparedAt time.Time

	// Valid indicates if the envelope is valid for execution.
	Valid bool

	// InvalidReason explains why the envelope is invalid (if applicable).
	InvalidReason string

	// ProviderRef is an opaque reference from the provider (if any).
	// In guarded mode, this is always empty.
	ProviderRef string
}

// ExecutionAttempt contains the result of an execution attempt.
type ExecutionAttempt struct {
	// AttemptID uniquely identifies this attempt.
	AttemptID string

	// EnvelopeID is the envelope that was attempted.
	EnvelopeID string

	// Provider is the adapter provider name.
	Provider string

	// AttemptedAt is when execution was attempted.
	AttemptedAt time.Time

	// Status is the attempt status.
	Status ExecutionAttemptStatus

	// BlockedReason explains why execution was blocked (if applicable).
	BlockedReason string

	// ProviderRef is an opaque reference from the provider (if any).
	// In guarded mode, this is always empty.
	ProviderRef string

	// MoneyMoved indicates if any money was moved.
	// CRITICAL: In v9 Slice 2, this is ALWAYS false.
	MoneyMoved bool
}

// ExecutionAttemptStatus represents the status of an execution attempt.
type ExecutionAttemptStatus string

const (
	// AttemptPrepared means preparation succeeded but execution not yet attempted.
	AttemptPrepared ExecutionAttemptStatus = "prepared"

	// AttemptBlocked means execution was blocked by the adapter.
	// This is the ONLY terminal status in v9 Slice 2.
	AttemptBlocked ExecutionAttemptStatus = "blocked"

	// AttemptFailed means execution failed due to an error.
	AttemptFailed ExecutionAttemptStatus = "failed"

	// AttemptSucceeded means execution succeeded.
	// CRITICAL: This status is FORBIDDEN in v9 Slice 2.
	// If this status ever appears, the implementation is WRONG.
	AttemptSucceeded ExecutionAttemptStatus = "succeeded"
)

// GuardedExecutionError is returned when a guarded adapter blocks execution.
// This is the EXPECTED error in v9 Slice 2.
type GuardedExecutionError struct {
	// EnvelopeID is the envelope that was blocked.
	EnvelopeID string

	// Provider is the adapter that blocked execution.
	Provider string

	// Reason explains why execution was blocked.
	Reason string

	// BlockedAt is when the block occurred.
	BlockedAt time.Time
}

// Error implements the error interface.
func (e *GuardedExecutionError) Error() string {
	return fmt.Sprintf("guarded execution blocked: provider=%s envelope=%s reason=%s",
		e.Provider, e.EnvelopeID, e.Reason)
}

// IsGuardedExecutionError checks if an error is a GuardedExecutionError.
func IsGuardedExecutionError(err error) bool {
	_, ok := err.(*GuardedExecutionError)
	return ok
}

// AdapterRegistry manages execution adapters.
type AdapterRegistry struct {
	adapters map[string]ExecutionAdapter
}

// NewAdapterRegistry creates a new adapter registry.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[string]ExecutionAdapter),
	}
}

// Register registers an adapter for a provider.
func (r *AdapterRegistry) Register(adapter ExecutionAdapter) {
	r.adapters[adapter.Provider()] = adapter
}

// Get returns the adapter for a provider.
func (r *AdapterRegistry) Get(provider string) (ExecutionAdapter, bool) {
	adapter, ok := r.adapters[provider]
	return adapter, ok
}

// Providers returns all registered provider names.
func (r *AdapterRegistry) Providers() []string {
	providers := make([]string, 0, len(r.adapters))
	for p := range r.adapters {
		providers = append(providers, p)
	}
	return providers
}
