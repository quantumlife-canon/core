// Package audit provides immutable logging and explainability.
// Logs are append-only with hash chaining for tamper evidence.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Intersections (Audit Trail)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.7 Audit & Governance Layer
//
// CRITICAL: Audit logs MUST NOT be used as operational memory or decision input.
package audit

import (
	"context"

	"quantumlife/pkg/primitives"
)

// Logger provides append-only audit logging.
type Logger interface {
	// Log appends an entry to the audit log.
	// Entry is immutable once logged.
	Log(ctx context.Context, entry Entry) error

	// LogWithExplanation appends an entry with explanation.
	LogWithExplanation(ctx context.Context, entry Entry, explanation Explanation) error
}

// Reader provides read access to audit logs.
// NOTE: This is for review/export only, NOT for operational decisions.
type Reader interface {
	// Get retrieves a single audit entry.
	Get(ctx context.Context, entryID string) (*Entry, error)

	// List retrieves audit entries matching filter.
	List(ctx context.Context, filter Filter) ([]Entry, error)

	// GetExplanation retrieves the explanation for an entry.
	GetExplanation(ctx context.Context, entryID string) (*Explanation, error)
}

// HashChain provides hash chain operations for tamper evidence.
type HashChain interface {
	// Verify verifies the integrity of the hash chain.
	Verify(ctx context.Context, ownerID string) (*VerificationResult, error)

	// GetChainHead returns the current head of the hash chain.
	GetChainHead(ctx context.Context, ownerID string) (*ChainHead, error)

	// Anchor anchors the chain to an external witness (optional).
	Anchor(ctx context.Context, ownerID string) (*Anchor, error)
}

// Exporter provides audit export for exit rights.
type Exporter interface {
	// Export exports the complete audit chain for a circle.
	// Includes full hash chain for independent verification.
	Export(ctx context.Context, circleID string) (*ExportPackage, error)

	// VerifyExport verifies an exported audit package.
	VerifyExport(ctx context.Context, pkg *ExportPackage) (*VerificationResult, error)
}

// GovernanceChecker enforces governance rules.
type GovernanceChecker interface {
	// Check evaluates an operation against governance rules.
	Check(ctx context.Context, operation GovernanceCheck) (*GovernanceResult, error)

	// GetViolations lists governance violations.
	GetViolations(ctx context.Context, filter Filter) ([]Violation, error)
}

// LoopEventEmitter emits events at loop step transitions.
// Used by the orchestrator to create audit trail for the Irreducible Loop.
type LoopEventEmitter interface {
	// EmitStepStarted emits an event when a loop step begins.
	EmitStepStarted(ctx context.Context, loopCtx LoopContext, step string) error

	// EmitStepCompleted emits an event when a loop step completes.
	EmitStepCompleted(ctx context.Context, loopCtx LoopContext, step string, resultSummary string) error

	// EmitStepFailed emits an event when a loop step fails.
	EmitStepFailed(ctx context.Context, loopCtx LoopContext, step string, errMsg string) error

	// EmitLoopCompleted emits an event when the entire loop completes.
	EmitLoopCompleted(ctx context.Context, loopCtx LoopContext, success bool, summary string) error

	// EmitLoopAborted emits an event when a loop is aborted.
	EmitLoopAborted(ctx context.Context, loopCtx LoopContext, reason string) error
}

// LoopContext is imported from primitives for loop threading.
type LoopContext = primitives.LoopContext
