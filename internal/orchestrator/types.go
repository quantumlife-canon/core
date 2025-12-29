package orchestrator

import (
	"time"

	"quantumlife/pkg/primitives"
)

// Intent represents the initial desire or goal that starts a loop.
type Intent struct {
	// ID uniquely identifies this intent.
	ID string

	// IssuerCircleID is the circle expressing the intent.
	IssuerCircleID string

	// Type categorizes the intent (e.g., "query", "action", "delegation").
	Type string

	// Description is the natural language or structured intent.
	Description string

	// TargetCircleID is the intended recipient (optional).
	TargetCircleID string

	// Parameters contains intent-specific parameters.
	Parameters map[string]string

	// CreatedAt is when the intent was expressed.
	CreatedAt time.Time
}

// IntentResult contains the result of intent processing.
type IntentResult struct {
	// IntentID references the processed intent.
	IntentID string

	// Classification categorizes the intent.
	Classification string

	// SuggestedAction is the recommended action type.
	SuggestedAction string

	// RequiredScopes lists scopes needed for this intent.
	RequiredScopes []string

	// SuggestedIntersectionID is a known intersection to use (optional).
	SuggestedIntersectionID string

	// RequiresNewIntersection indicates if a new intersection is needed.
	RequiresNewIntersection bool

	// Confidence is the classification confidence (0.0 to 1.0).
	Confidence float64

	// ProcessedAt is when processing completed.
	ProcessedAt time.Time
}

// DiscoveryResult contains the result of intersection discovery.
type DiscoveryResult struct {
	// IntersectionID is the discovered or created intersection.
	IntersectionID string

	// IsNew indicates if this is a newly created intersection.
	IsNew bool

	// ContractVersion is the active contract version.
	ContractVersion string

	// AvailableScopes lists scopes available in this intersection.
	AvailableScopes []string

	// DiscoveredAt is when discovery completed.
	DiscoveredAt time.Time
}

// AuthorityResult contains the result of authority negotiation.
type AuthorityResult struct {
	// Granted indicates if authority was granted.
	Granted bool

	// GrantID is the authority grant ID (if granted).
	GrantID string

	// GrantedScopes lists the scopes that were granted.
	GrantedScopes []string

	// Conditions lists any conditions on the grant.
	Conditions []string

	// DenialReason explains why authority was denied (if not granted).
	DenialReason string

	// RequiresEscalation indicates if human approval is needed.
	RequiresEscalation bool

	// ExpiresAt is when the grant expires.
	ExpiresAt *time.Time

	// NegotiatedAt is when negotiation completed.
	NegotiatedAt time.Time
}

// Commitment represents a binding commitment to an action.
type Commitment struct {
	// ID uniquely identifies this commitment.
	ID string

	// ActionType is the type of action committed to.
	ActionType string

	// ActionParameters contains the action parameters.
	ActionParameters map[string]string

	// AuthorityGrantID references the authorizing grant.
	AuthorityGrantID string

	// Conditions lists conditions that must hold.
	Conditions []CommitmentCondition

	// IdempotencyKey ensures at-most-once execution.
	IdempotencyKey string

	// CommittedAt is when the commitment was formed.
	CommittedAt time.Time
}

// CommitmentCondition represents a condition on a commitment.
type CommitmentCondition struct {
	// Type is the condition type (e.g., "timeout", "threshold", "approval").
	Type string

	// Value is the condition value.
	Value string

	// Satisfied indicates if the condition is currently met.
	Satisfied bool
}

// ActionResult contains the result of action execution.
type ActionResult struct {
	// ActionID uniquely identifies the executed action.
	ActionID string

	// CommitmentID references the originating commitment.
	CommitmentID string

	// Success indicates if the action succeeded.
	Success bool

	// ResultCode is a machine-readable result code.
	ResultCode string

	// ResultData contains action-specific result data.
	ResultData map[string]string

	// ErrorMessage contains the error if action failed.
	ErrorMessage string

	// StartedAt is when execution began.
	StartedAt time.Time

	// CompletedAt is when execution completed.
	CompletedAt time.Time
}

// SettlementResult contains the result of settlement.
type SettlementResult struct {
	// SettlementID uniquely identifies this settlement.
	SettlementID string

	// ActionID references the settled action.
	ActionID string

	// Status is the settlement status.
	Status SettlementStatus

	// ValueExchanged describes any value exchanged.
	ValueExchanged *ValueExchange

	// SettledAt is when settlement completed.
	SettledAt time.Time
}

// SettlementStatus indicates the settlement outcome.
type SettlementStatus string

const (
	// SettlementComplete indicates successful settlement.
	SettlementComplete SettlementStatus = "complete"

	// SettlementDisputed indicates the settlement is disputed.
	SettlementDisputed SettlementStatus = "disputed"

	// SettlementFailed indicates settlement failed.
	SettlementFailed SettlementStatus = "failed"
)

// ValueExchange describes value exchanged in settlement.
type ValueExchange struct {
	// Type is the exchange type (e.g., "data", "token", "acknowledgment").
	Type string

	// FromCircleID is the source circle.
	FromCircleID string

	// ToCircleID is the destination circle.
	ToCircleID string

	// Amount is the exchange amount (if applicable).
	Amount string

	// Unit is the exchange unit (if applicable).
	Unit string
}

// MemoryResult contains the result of memory update.
type MemoryResult struct {
	// RecordID uniquely identifies the memory record.
	RecordID string

	// TraceID links to the loop trace.
	TraceID primitives.LoopTraceID

	// StoredAt is when the record was stored.
	StoredAt time.Time
}

// LoopResult contains the final result of a complete loop traversal.
type LoopResult struct {
	// TraceID identifies this loop traversal.
	TraceID primitives.LoopTraceID

	// FinalStep is the last step completed.
	FinalStep primitives.LoopStep

	// Success indicates if the loop completed successfully.
	Success bool

	// IntentResult contains the intent processing result.
	IntentResult *IntentResult

	// DiscoveryResult contains the intersection discovery result.
	DiscoveryResult *DiscoveryResult

	// AuthorityResult contains the authority negotiation result.
	AuthorityResult *AuthorityResult

	// Commitment contains the formed commitment.
	Commitment *Commitment

	// ActionResult contains the action execution result.
	ActionResult *ActionResult

	// SettlementResult contains the settlement result.
	SettlementResult *SettlementResult

	// MemoryResult contains the memory update result.
	MemoryResult *MemoryResult

	// FailureStep is the step where failure occurred (if not successful).
	FailureStep *primitives.LoopStep

	// FailureReason explains the failure (if not successful).
	FailureReason string

	// StartedAt is when the loop began.
	StartedAt time.Time

	// CompletedAt is when the loop completed.
	CompletedAt time.Time
}

// LoopStatus contains the current status of a loop.
type LoopStatus struct {
	// TraceID identifies this loop.
	TraceID primitives.LoopTraceID

	// CurrentStep is the current position in the loop.
	CurrentStep primitives.LoopStep

	// State is the overall loop state.
	State LoopState

	// StepStatuses contains status for each step.
	StepStatuses map[primitives.LoopStep]StepStatus

	// LastUpdated is when the status was last updated.
	LastUpdated time.Time
}

// LoopState indicates the overall loop state.
type LoopState string

const (
	// LoopStateActive indicates the loop is actively progressing.
	LoopStateActive LoopState = "active"

	// LoopStatePaused indicates the loop is paused.
	LoopStatePaused LoopState = "paused"

	// LoopStateAwaitingApproval indicates the loop awaits human approval.
	LoopStateAwaitingApproval LoopState = "awaiting_approval"

	// LoopStateCompleted indicates the loop completed.
	LoopStateCompleted LoopState = "completed"

	// LoopStateAborted indicates the loop was aborted.
	LoopStateAborted LoopState = "aborted"

	// LoopStateFailed indicates the loop failed.
	LoopStateFailed LoopState = "failed"
)

// StepStatus contains status for a single step.
type StepStatus struct {
	// Step identifies the step.
	Step primitives.LoopStep

	// State is the step state.
	State StepState

	// StartedAt is when the step began.
	StartedAt *time.Time

	// CompletedAt is when the step completed.
	CompletedAt *time.Time

	// Error contains any error message.
	Error string
}

// StepState indicates the state of a step.
type StepState string

const (
	// StepStatePending indicates the step has not started.
	StepStatePending StepState = "pending"

	// StepStateInProgress indicates the step is in progress.
	StepStateInProgress StepState = "in_progress"

	// StepStateCompleted indicates the step completed successfully.
	StepStateCompleted StepState = "completed"

	// StepStateFailed indicates the step failed.
	StepStateFailed StepState = "failed"

	// StepStateSkipped indicates the step was skipped.
	StepStateSkipped StepState = "skipped"
)
