// Package orchestrator implements the Irreducible Loop orchestration.
//
// The orchestrator coordinates the seven-step loop defined in the Canon,
// delegating to appropriate layer services at each step while maintaining
// context and emitting audit events.
//
// Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
package orchestrator

import (
	"context"

	"quantumlife/pkg/primitives"
)

// LoopOrchestrator orchestrates the Irreducible Loop.
// It coordinates the flow through all seven steps, delegating to
// layer-specific services and maintaining loop context.
type LoopOrchestrator interface {
	// ExecuteLoop runs a complete loop from intent to memory update.
	// The loop context must have TraceID and IssuerCircleID set.
	// Returns the final loop result or an error if any step fails.
	ExecuteLoop(ctx context.Context, loopCtx primitives.LoopContext, intent Intent) (*LoopResult, error)

	// ResumeLoop resumes a paused or failed loop from a specific step.
	// Used for retry scenarios or human-in-the-loop continuations.
	ResumeLoop(ctx context.Context, loopCtx primitives.LoopContext, fromStep primitives.LoopStep) (*LoopResult, error)

	// AbortLoop aborts an in-progress loop.
	// Records the abort reason and emits appropriate events.
	AbortLoop(ctx context.Context, traceID primitives.LoopTraceID, reason string) error

	// GetLoopStatus returns the current status of a loop.
	GetLoopStatus(ctx context.Context, traceID primitives.LoopTraceID) (*LoopStatus, error)
}

// IntentProcessor handles the Intent step (step 1).
// This is typically implemented by the negotiation layer.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.3 Negotiation Layer
type IntentProcessor interface {
	// ProcessIntent classifies and validates an intent.
	// Returns the processed result including suggested actions.
	ProcessIntent(ctx context.Context, loopCtx primitives.LoopContext, intent Intent) (*IntentResult, error)
}

// IntersectionDiscoverer handles the Intersection Discovery step (step 2).
// This is typically implemented by the intersection layer.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.2 Orchestration Layer
type IntersectionDiscoverer interface {
	// DiscoverIntersection finds or creates an intersection for the intent.
	// Returns the intersection ID and discovery metadata.
	DiscoverIntersection(ctx context.Context, loopCtx primitives.LoopContext, intentResult *IntentResult) (*DiscoveryResult, error)
}

// AuthorityNegotiator handles the Authority Negotiation step (step 3).
// This is typically implemented by the authority layer.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.4 Authority Layer
type AuthorityNegotiator interface {
	// NegotiateAuthority confirms or acquires necessary authority.
	// Returns the authority grant or denial with reasons.
	NegotiateAuthority(ctx context.Context, loopCtx primitives.LoopContext, discovery *DiscoveryResult) (*AuthorityResult, error)
}

// CommitmentFormer handles the Commitment step (step 4).
// This bridges negotiation and execution.
//
// Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
type CommitmentFormer interface {
	// FormCommitment creates a binding commitment for the action.
	// Returns the commitment details including conditions.
	FormCommitment(ctx context.Context, loopCtx primitives.LoopContext, authority *AuthorityResult) (*Commitment, error)
}

// ActionExecutor handles the Action step (step 5).
// This is implemented by the execution layer.
//
// CRITICAL: The execution layer is data plane — NO LLM/SLM usage.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Execution Layer
type ActionExecutor interface {
	// ExecuteAction executes the committed action.
	// This is deterministic execution with no model inference.
	ExecuteAction(ctx context.Context, loopCtx primitives.LoopContext, commitment *Commitment) (*ActionResult, error)

	// PauseAction pauses an in-progress action.
	PauseAction(ctx context.Context, loopCtx primitives.LoopContext, actionID string) error

	// ResumeAction resumes a paused action.
	ResumeAction(ctx context.Context, loopCtx primitives.LoopContext, actionID string) error
}

// SettlementProcessor handles the Settlement step (step 6).
// This confirms completion and handles value exchange.
//
// Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
type SettlementProcessor interface {
	// ProcessSettlement confirms completion and settles the action.
	// Returns the settlement result including any value exchange.
	ProcessSettlement(ctx context.Context, loopCtx primitives.LoopContext, action *ActionResult) (*SettlementResult, error)

	// DisputeSettlement raises a dispute on a settlement.
	DisputeSettlement(ctx context.Context, loopCtx primitives.LoopContext, settlementID string, reason string) error
}

// MemoryUpdater handles the Memory Update step (step 7).
// This is implemented by the memory layer.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.6 Memory Layer
type MemoryUpdater interface {
	// UpdateMemory records the loop outcome for future reference.
	// Returns confirmation of the memory update.
	UpdateMemory(ctx context.Context, loopCtx primitives.LoopContext, settlement *SettlementResult) (*MemoryResult, error)
}

// LoopEventEmitter emits events at loop step transitions.
// This feeds the audit layer.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.7 Audit & Governance Layer
type LoopEventEmitter interface {
	// EmitStepStarted emits an event when a step begins.
	EmitStepStarted(ctx context.Context, loopCtx primitives.LoopContext, step primitives.LoopStep) error

	// EmitStepCompleted emits an event when a step completes.
	EmitStepCompleted(ctx context.Context, loopCtx primitives.LoopContext, step primitives.LoopStep, result interface{}) error

	// EmitStepFailed emits an event when a step fails.
	EmitStepFailed(ctx context.Context, loopCtx primitives.LoopContext, step primitives.LoopStep, err error) error

	// EmitLoopCompleted emits an event when the entire loop completes.
	EmitLoopCompleted(ctx context.Context, loopCtx primitives.LoopContext, result *LoopResult) error

	// EmitLoopAborted emits an event when a loop is aborted.
	EmitLoopAborted(ctx context.Context, loopCtx primitives.LoopContext, reason string) error
}
