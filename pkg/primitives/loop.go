// Package primitives defines the immutable data structures for all canon primitives.
//
// This file defines types for the Irreducible Loop.
// Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
package primitives

import (
	"time"
)

// LoopStep represents a step in the Irreducible Loop.
// The loop is: Intent → Intersection Discovery → Authority Negotiation →
// Commitment → Action → Settlement → Memory Update
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
type LoopStep string

const (
	// StepIntent is step 1: A desire or goal is expressed.
	StepIntent LoopStep = "intent"

	// StepIntersectionDiscovery is step 2: Find or create the relevant intersection.
	StepIntersectionDiscovery LoopStep = "intersection_discovery"

	// StepAuthorityNegotiation is step 3: Confirm or acquire necessary authority.
	StepAuthorityNegotiation LoopStep = "authority_negotiation"

	// StepCommitment is step 4: Bind to an action under stated conditions.
	StepCommitment LoopStep = "commitment"

	// StepAction is step 5: Execute within granted authority.
	StepAction LoopStep = "action"

	// StepSettlement is step 6: Confirm completion, exchange value if needed.
	StepSettlement LoopStep = "settlement"

	// StepMemoryUpdate is step 7: Record outcome for future reference.
	StepMemoryUpdate LoopStep = "memory_update"
)

// AllLoopSteps returns all steps in order.
func AllLoopSteps() []LoopStep {
	return []LoopStep{
		StepIntent,
		StepIntersectionDiscovery,
		StepAuthorityNegotiation,
		StepCommitment,
		StepAction,
		StepSettlement,
		StepMemoryUpdate,
	}
}

// IsValid returns true if the step is a valid loop step.
func (s LoopStep) IsValid() bool {
	switch s {
	case StepIntent, StepIntersectionDiscovery, StepAuthorityNegotiation,
		StepCommitment, StepAction, StepSettlement, StepMemoryUpdate:
		return true
	default:
		return false
	}
}

// LoopTraceID uniquely identifies a single traversal of the loop.
// All events and actions within one loop execution share the same trace ID.
type LoopTraceID string

// RiskClass indicates the risk level of an operation.
// Higher risk requires stricter approval modes.
//
// Reference: docs/HUMAN_GUARANTEES_V1.md §3 Authority & Autonomy
type RiskClass string

const (
	// RiskLow indicates low-risk operations (routine, reversible).
	RiskLow RiskClass = "low"

	// RiskStandard indicates standard operations.
	RiskStandard RiskClass = "standard"

	// RiskElevated indicates elevated risk (requires extra validation).
	RiskElevated RiskClass = "elevated"

	// RiskHigh indicates high-risk operations (financial, legal, irreversible).
	// These require LLM escalation and human confirmation.
	RiskHigh RiskClass = "high"
)

// LoopContext contains immutable context metadata for a loop traversal.
// This context is threaded through all steps and captured in audit.
//
// LoopContext is immutable once created — do not modify fields.
type LoopContext struct {
	// TraceID uniquely identifies this loop traversal.
	TraceID LoopTraceID

	// IssuerCircleID is the circle that initiated the loop.
	IssuerCircleID string

	// IntersectionID is the intersection governing this loop (set after discovery).
	IntersectionID string

	// CreatedAt is when the loop was initiated.
	CreatedAt time.Time

	// RiskClass is the assessed risk level for this loop.
	RiskClass RiskClass

	// AutonomyMode indicates the approval mode for this loop.
	// Values: "pre_action", "exception_only", "post_action"
	AutonomyMode string

	// CurrentStep tracks the current position in the loop.
	CurrentStep LoopStep

	// Metadata contains additional context (read-only after creation).
	Metadata map[string]string
}

// Validate checks that the loop context has all required fields.
func (lc *LoopContext) Validate() error {
	if lc.TraceID == "" {
		return ErrMissingTraceID
	}
	if lc.IssuerCircleID == "" {
		return ErrMissingIssuer
	}
	if lc.CreatedAt.IsZero() {
		return ErrMissingTimestamp
	}
	if !lc.CurrentStep.IsValid() {
		return ErrInvalidLoopStep
	}
	return nil
}

// WithStep returns a copy of the context with the step updated.
// LoopContext is treated as immutable; this creates a new instance.
func (lc *LoopContext) WithStep(step LoopStep) LoopContext {
	newCtx := *lc
	newCtx.CurrentStep = step
	return newCtx
}

// WithIntersection returns a copy of the context with the intersection set.
func (lc *LoopContext) WithIntersection(intersectionID string) LoopContext {
	newCtx := *lc
	newCtx.IntersectionID = intersectionID
	return newCtx
}

// Loop validation errors.
var (
	ErrMissingTraceID  = loopError("missing trace id")
	ErrInvalidLoopStep = loopError("invalid loop step")
)

type loopError string

func (e loopError) Error() string { return string(e) }
