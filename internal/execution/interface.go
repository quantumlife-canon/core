// Package execution executes committed actions within granted authority.
// This is a DATA PLANE component — deterministic only, NO LLM/SLM.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
//
// CRITICAL: This package MUST NOT make decisions. It executes what was committed.
package execution

import (
	"context"

	"quantumlife/pkg/primitives"
)

// Executor executes committed actions.
// All operations are deterministic — NO LLM/SLM.
type Executor interface {
	// Execute runs an action derived from a commitment.
	// Returns when action completes, is paused, or is aborted.
	Execute(ctx context.Context, action *primitives.Action) (*ExecutionResult, error)

	// Pause pauses an executing action.
	Pause(ctx context.Context, actionID string) error

	// Resume resumes a paused action.
	Resume(ctx context.Context, actionID string) error

	// Abort aborts an action (cannot be resumed).
	// Per Canon: "There is no 'finish what you started' exception."
	Abort(ctx context.Context, actionID string, reason string) error

	// GetStatus returns the current execution status.
	GetStatus(ctx context.Context, actionID string) (*ExecutionStatus, error)
}

// Settler handles action settlement.
type Settler interface {
	// Settle confirms completion of an action.
	// Settlement is atomic — complete or not at all.
	Settle(ctx context.Context, actionID string, outcome Outcome) (*primitives.Settlement, error)

	// Dispute marks a settlement as disputed.
	Dispute(ctx context.Context, settlementID string, reason string) error

	// Resolve resolves a disputed settlement.
	Resolve(ctx context.Context, settlementID string, resolution Resolution) error
}

// Connector defines the interface for external service connectors.
// Connectors are data-plane components — deterministic, no models.
type Connector interface {
	// ID returns the connector identifier.
	ID() string

	// Capabilities returns the connector's capabilities.
	Capabilities() []string

	// RequiredScopes returns scopes required for this connector.
	RequiredScopes() []string

	// Execute performs the connector action.
	// Must be deterministic and idempotent where possible.
	Execute(ctx context.Context, params ConnectorParams) (*ConnectorResult, error)

	// HealthCheck verifies the connector is operational.
	HealthCheck(ctx context.Context) error
}

// ConnectorRegistry provides access to registered connectors.
type ConnectorRegistry interface {
	// Get retrieves a connector by ID.
	Get(ctx context.Context, connectorID string) (Connector, error)

	// List returns all registered connectors.
	List(ctx context.Context) ([]ConnectorInfo, error)

	// Register adds a connector to the registry.
	Register(ctx context.Context, connector Connector) error
}

// LoopActionExecutor provides loop-aware action execution.
// Used by the orchestrator at step 5 (Action) of the Irreducible Loop.
//
// CRITICAL: This is data plane — deterministic only, NO LLM/SLM.
type LoopActionExecutor interface {
	// ExecuteForLoop executes an action within a loop context.
	// The loop context provides trace ID for correlation.
	ExecuteForLoop(ctx context.Context, loopCtx LoopContext, commitment Commitment) (*ExecutionResult, error)

	// PauseForLoop pauses an action within a loop.
	PauseForLoop(ctx context.Context, loopCtx LoopContext, actionID string) error

	// ResumeForLoop resumes a paused action within a loop.
	ResumeForLoop(ctx context.Context, loopCtx LoopContext, actionID string) error

	// AbortForLoop aborts an action within a loop.
	AbortForLoop(ctx context.Context, loopCtx LoopContext, actionID string, reason string) error
}

// LoopSettler provides loop-aware settlement.
// Used by the orchestrator at step 6 (Settlement) of the Irreducible Loop.
type LoopSettler interface {
	// SettleForLoop settles an action within a loop context.
	SettleForLoop(ctx context.Context, loopCtx LoopContext, actionID string, outcome Outcome) (*SettlementResult, error)

	// DisputeForLoop raises a dispute within a loop context.
	DisputeForLoop(ctx context.Context, loopCtx LoopContext, settlementID string, reason string) error
}

// LoopContext is imported from primitives for loop threading.
type LoopContext = primitives.LoopContext

// Commitment represents a commitment to be executed.
type Commitment struct {
	ID               string
	ActionType       string
	ActionParameters map[string]string
	AuthorityGrantID string
	IdempotencyKey   string
}

// SettlementResult contains the result of settlement.
type SettlementResult struct {
	SettlementID string
	ActionID     string
	Status       string
	SettledAt    string
}
