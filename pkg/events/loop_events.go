// Package events defines event types for system observability.
// This file defines loop-specific events for the Irreducible Loop.
//
// Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
package events

import (
	"time"

	"quantumlife/pkg/primitives"
)

// Loop step event types.
const (
	// Step start events
	EventLoopStepIntentStarted       EventType = "loop.step.intent.started"
	EventLoopStepDiscoveryStarted    EventType = "loop.step.discovery.started"
	EventLoopStepAuthorityStarted    EventType = "loop.step.authority.started"
	EventLoopStepCommitmentStarted   EventType = "loop.step.commitment.started"
	EventLoopStepActionStarted       EventType = "loop.step.action.started"
	EventLoopStepSettlementStarted   EventType = "loop.step.settlement.started"
	EventLoopStepMemoryUpdateStarted EventType = "loop.step.memory_update.started"

	// Step completion events
	EventLoopStepIntentCompleted       EventType = "loop.step.intent.completed"
	EventLoopStepDiscoveryCompleted    EventType = "loop.step.discovery.completed"
	EventLoopStepAuthorityCompleted    EventType = "loop.step.authority.completed"
	EventLoopStepCommitmentCompleted   EventType = "loop.step.commitment.completed"
	EventLoopStepActionCompleted       EventType = "loop.step.action.completed"
	EventLoopStepSettlementCompleted   EventType = "loop.step.settlement.completed"
	EventLoopStepMemoryUpdateCompleted EventType = "loop.step.memory_update.completed"

	// Step failure events
	EventLoopStepIntentFailed       EventType = "loop.step.intent.failed"
	EventLoopStepDiscoveryFailed    EventType = "loop.step.discovery.failed"
	EventLoopStepAuthorityFailed    EventType = "loop.step.authority.failed"
	EventLoopStepCommitmentFailed   EventType = "loop.step.commitment.failed"
	EventLoopStepActionFailed       EventType = "loop.step.action.failed"
	EventLoopStepSettlementFailed   EventType = "loop.step.settlement.failed"
	EventLoopStepMemoryUpdateFailed EventType = "loop.step.memory_update.failed"

	// Loop lifecycle events
	EventLoopStarted   EventType = "loop.started"
	EventLoopCompleted EventType = "loop.completed"
	EventLoopFailed    EventType = "loop.failed"
	EventLoopAborted   EventType = "loop.aborted"
	EventLoopPaused    EventType = "loop.paused"
	EventLoopResumed   EventType = "loop.resumed"
)

// LoopEvent extends Event with loop-specific fields.
type LoopEvent struct {
	Event

	// TraceID is the loop trace identifier.
	TraceID primitives.LoopTraceID

	// Step is the current loop step (if applicable).
	Step primitives.LoopStep

	// IssuerCircleID is the circle that initiated the loop.
	IssuerCircleID string

	// RiskClass is the assessed risk level for this loop.
	RiskClass primitives.RiskClass

	// AutonomyMode is the approval mode for this loop.
	AutonomyMode string
}

// StepEvent contains details about a step transition.
type StepEvent struct {
	LoopEvent

	// PreviousStep is the step before this transition.
	PreviousStep primitives.LoopStep

	// Duration is how long the step took.
	Duration time.Duration

	// Result contains step result summary.
	Result string

	// Error contains error message if step failed.
	Error string
}

// LoopLifecycleEvent contains details about loop lifecycle changes.
type LoopLifecycleEvent struct {
	LoopEvent

	// FinalStep is the last step completed.
	FinalStep primitives.LoopStep

	// Success indicates if the loop completed successfully.
	Success bool

	// Duration is total loop duration.
	Duration time.Duration

	// FailureStep is the step where failure occurred (if applicable).
	FailureStep primitives.LoopStep

	// FailureReason explains the failure (if applicable).
	FailureReason string

	// AbortReason explains why the loop was aborted (if applicable).
	AbortReason string
}

// NewLoopEvent creates a new LoopEvent with common fields populated.
func NewLoopEvent(eventType EventType, loopCtx primitives.LoopContext) LoopEvent {
	return LoopEvent{
		Event: Event{
			Type:      eventType,
			Timestamp: time.Now(),
			CircleID:  loopCtx.IssuerCircleID,
			TraceID:   string(loopCtx.TraceID),
		},
		TraceID:        loopCtx.TraceID,
		Step:           loopCtx.CurrentStep,
		IssuerCircleID: loopCtx.IssuerCircleID,
		RiskClass:      loopCtx.RiskClass,
		AutonomyMode:   loopCtx.AutonomyMode,
	}
}

// NewStepStartedEvent creates an event for when a step begins.
func NewStepStartedEvent(loopCtx primitives.LoopContext, step primitives.LoopStep) StepEvent {
	eventType := stepToStartEventType(step)
	return StepEvent{
		LoopEvent: NewLoopEvent(eventType, loopCtx),
	}
}

// NewStepCompletedEvent creates an event for when a step completes.
func NewStepCompletedEvent(loopCtx primitives.LoopContext, step primitives.LoopStep, duration time.Duration, result string) StepEvent {
	eventType := stepToCompletedEventType(step)
	event := StepEvent{
		LoopEvent: NewLoopEvent(eventType, loopCtx),
		Duration:  duration,
		Result:    result,
	}
	return event
}

// NewStepFailedEvent creates an event for when a step fails.
func NewStepFailedEvent(loopCtx primitives.LoopContext, step primitives.LoopStep, errMsg string) StepEvent {
	eventType := stepToFailedEventType(step)
	event := StepEvent{
		LoopEvent: NewLoopEvent(eventType, loopCtx),
		Error:     errMsg,
	}
	return event
}

// NewLoopCompletedEvent creates an event for when a loop completes.
func NewLoopCompletedEvent(loopCtx primitives.LoopContext, success bool, duration time.Duration) LoopLifecycleEvent {
	eventType := EventLoopCompleted
	if !success {
		eventType = EventLoopFailed
	}
	return LoopLifecycleEvent{
		LoopEvent: NewLoopEvent(eventType, loopCtx),
		FinalStep: loopCtx.CurrentStep,
		Success:   success,
		Duration:  duration,
	}
}

// NewLoopAbortedEvent creates an event for when a loop is aborted.
func NewLoopAbortedEvent(loopCtx primitives.LoopContext, reason string) LoopLifecycleEvent {
	return LoopLifecycleEvent{
		LoopEvent:   NewLoopEvent(EventLoopAborted, loopCtx),
		FinalStep:   loopCtx.CurrentStep,
		AbortReason: reason,
	}
}

// stepToStartEventType maps a loop step to its start event type.
func stepToStartEventType(step primitives.LoopStep) EventType {
	switch step {
	case primitives.StepIntent:
		return EventLoopStepIntentStarted
	case primitives.StepIntersectionDiscovery:
		return EventLoopStepDiscoveryStarted
	case primitives.StepAuthorityNegotiation:
		return EventLoopStepAuthorityStarted
	case primitives.StepCommitment:
		return EventLoopStepCommitmentStarted
	case primitives.StepAction:
		return EventLoopStepActionStarted
	case primitives.StepSettlement:
		return EventLoopStepSettlementStarted
	case primitives.StepMemoryUpdate:
		return EventLoopStepMemoryUpdateStarted
	default:
		return EventLoopStarted
	}
}

// stepToCompletedEventType maps a loop step to its completed event type.
func stepToCompletedEventType(step primitives.LoopStep) EventType {
	switch step {
	case primitives.StepIntent:
		return EventLoopStepIntentCompleted
	case primitives.StepIntersectionDiscovery:
		return EventLoopStepDiscoveryCompleted
	case primitives.StepAuthorityNegotiation:
		return EventLoopStepAuthorityCompleted
	case primitives.StepCommitment:
		return EventLoopStepCommitmentCompleted
	case primitives.StepAction:
		return EventLoopStepActionCompleted
	case primitives.StepSettlement:
		return EventLoopStepSettlementCompleted
	case primitives.StepMemoryUpdate:
		return EventLoopStepMemoryUpdateCompleted
	default:
		return EventLoopCompleted
	}
}

// stepToFailedEventType maps a loop step to its failed event type.
func stepToFailedEventType(step primitives.LoopStep) EventType {
	switch step {
	case primitives.StepIntent:
		return EventLoopStepIntentFailed
	case primitives.StepIntersectionDiscovery:
		return EventLoopStepDiscoveryFailed
	case primitives.StepAuthorityNegotiation:
		return EventLoopStepAuthorityFailed
	case primitives.StepCommitment:
		return EventLoopStepCommitmentFailed
	case primitives.StepAction:
		return EventLoopStepActionFailed
	case primitives.StepSettlement:
		return EventLoopStepSettlementFailed
	case primitives.StepMemoryUpdate:
		return EventLoopStepMemoryUpdateFailed
	default:
		return EventLoopFailed
	}
}
