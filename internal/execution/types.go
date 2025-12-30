package execution

import (
	"time"
)

// ExecutionResult contains the result of action execution.
type ExecutionResult struct {
	ActionID    string
	State       State
	Outcome     *Outcome
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
}

// State represents the execution state.
type State string

const (
	StatePending   State = "pending"
	StateExecuting State = "executing"
	StatePaused    State = "paused"
	StateAborted   State = "aborted"
	StateCompleted State = "completed"
)

// ExecutionStatus contains current execution status.
type ExecutionStatus struct {
	ActionID  string
	State     State
	Progress  float64 // 0.0 to 1.0
	StartedAt *time.Time
	PausedAt  *time.Time
	Message   string
}

// Outcome represents the result of an executed action.
type Outcome struct {
	Success      bool
	ResultCode   string
	ResultData   map[string]string
	ErrorMessage string
}

// Resolution represents how a dispute was resolved.
type Resolution struct {
	Outcome     string // "completed", "reversed", "partial"
	Description string
	ResolvedBy  string
	ResolvedAt  time.Time
}

// ConnectorParams contains parameters for connector execution.
type ConnectorParams struct {
	ActionID       string
	ActionType     string
	Parameters     map[string]string
	IdempotencyKey string
	Timeout        time.Duration
}

// ConnectorResult contains the result of connector execution.
type ConnectorResult struct {
	Success      bool
	ResultCode   string
	ResultData   map[string]string
	ErrorMessage string
	Retryable    bool
}

// ConnectorInfo contains metadata about a connector.
type ConnectorInfo struct {
	ID             string
	Name           string
	Capabilities   []string
	RequiredScopes []string
	Healthy        bool
}

// RetryPolicy defines retry behavior for connector execution.
type RetryPolicy struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	RetryOn        []string // Error codes to retry on
}

// ExecutionOutcome represents the outcome of executing an action.
// Used by the simulator to return deterministic results.
type ExecutionOutcome struct {
	// ActionID is the action that was executed.
	ActionID string

	// Success indicates whether execution succeeded.
	Success bool

	// Simulated indicates this was a simulated execution (no real side effects).
	Simulated bool

	// ResultCode is a machine-readable result code.
	ResultCode string

	// ResultData contains action-specific result data.
	ResultData map[string]string

	// ErrorMessage contains error details if Success is false.
	ErrorMessage string

	// ConnectorID identifies which connector was used.
	ConnectorID string

	// ProposedPayload contains the payload that would be sent to the external service.
	// Only populated in simulate mode.
	ProposedPayload map[string]string

	// ExecutedAt is when the execution occurred.
	ExecutedAt time.Time

	// AuthorizationProofID links to the authorization proof.
	AuthorizationProofID string
}
