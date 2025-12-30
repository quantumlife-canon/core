// Package negotiation handles proposals, counterproposals, and commitment formation.
// This is a control-plane component that may use LLM/SLM.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §The Irreducible Loop
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.4 Negotiation Engine
package negotiation

import (
	"context"

	"quantumlife/pkg/primitives"
)

// Engine defines the interface for negotiation operations.
type Engine interface {
	// Intent processing

	// ProcessIntent analyzes an intent and determines next steps.
	ProcessIntent(ctx context.Context, intent *primitives.Intent) (*IntentResult, error)

	// Proposal lifecycle

	// CreateProposal creates a proposal from an intent.
	CreateProposal(ctx context.Context, req ProposalRequest) (*primitives.Proposal, error)

	// SubmitProposal submits a proposal to parties.
	SubmitProposal(ctx context.Context, proposalID string) error

	// AnalyzeProposal analyzes a received proposal.
	AnalyzeProposal(ctx context.Context, proposal *primitives.Proposal) (*ProposalAnalysis, error)

	// GenerateCounterproposal creates a counterproposal.
	GenerateCounterproposal(ctx context.Context, proposalID string, modifications []Modification) (*primitives.Proposal, error)

	// AcceptProposal accepts a proposal and forms a commitment.
	AcceptProposal(ctx context.Context, proposalID string, acceptorID string) (*primitives.Commitment, error)

	// RejectProposal rejects a proposal.
	RejectProposal(ctx context.Context, proposalID string, rejectorID string, reason string) error

	// Commitment formation

	// FormCommitment creates a commitment from an accepted proposal.
	FormCommitment(ctx context.Context, proposalID string) (*primitives.Commitment, error)
}

// NegotiationLoop defines the interface for the complete negotiation loop.
// This is used for proposal → counterproposal → acceptance → finalization flows.
type NegotiationLoop interface {
	// SubmitProposal submits a new proposal for an intersection amendment.
	// Returns the proposal ID on success.
	SubmitProposal(ctx context.Context, intersectionID string, proposal SubmitProposalRequest) (string, error)

	// CounterProposal creates a counterproposal to an existing proposal.
	// Returns the counter proposal ID on success.
	CounterProposal(ctx context.Context, proposalID string, counter CounterProposalRequest) (string, error)

	// Accept records a party's acceptance of a proposal or counterproposal.
	Accept(ctx context.Context, proposalOrCounterID string, byCircleID string) (*AcceptResult, error)

	// Reject records a party's rejection of a proposal.
	Reject(ctx context.Context, proposalID string, byCircleID string, reason string) error

	// Finalize completes the negotiation after all parties accept.
	// Returns the commitment or contract change depending on the proposal type.
	Finalize(ctx context.Context, proposalID string) (*FinalizeResult, error)

	// GetProposal retrieves a proposal by ID.
	GetProposal(ctx context.Context, proposalID string) (*ProposalThread, error)

	// ListProposals lists all proposals for an intersection.
	ListProposals(ctx context.Context, intersectionID string) ([]ProposalThread, error)
}

// SubmitProposalRequest contains parameters for submitting a proposal.
type SubmitProposalRequest struct {
	IssuerCircleID string
	ProposalType   ProposalType
	Reason         string

	// For contract amendments
	ScopeAdditions []ScopeChange
	ScopeRemovals  []string
	CeilingChanges []CeilingChange

	// For commitment formation
	ActionSpec *primitives.ActionSpec
}

// CounterProposalRequest contains parameters for a counterproposal.
type CounterProposalRequest struct {
	IssuerCircleID string
	Reason         string

	// Modified terms
	ScopeAdditions []ScopeChange
	ScopeRemovals  []string
	CeilingChanges []CeilingChange
}

// ScopeChange represents a scope addition or modification.
type ScopeChange struct {
	Name        string
	Description string
	Permission  string
}

// CeilingChange represents a ceiling modification.
type CeilingChange struct {
	Type  string
	Value string
	Unit  string
}

// ProposalType indicates the type of proposal.
type ProposalType string

const (
	ProposalTypeAmendment  ProposalType = "amendment"
	ProposalTypeCommitment ProposalType = "commitment"
)

// ProposalState indicates the state of a proposal.
type ProposalState string

const (
	ProposalStatePending    ProposalState = "pending"
	ProposalStateCountered  ProposalState = "countered"
	ProposalStateAccepted   ProposalState = "accepted"
	ProposalStateRejected   ProposalState = "rejected"
	ProposalStateFinalized  ProposalState = "finalized"
	ProposalStateSuperseded ProposalState = "superseded"
)

// ProposalThread represents a proposal with its counters and approvals.
type ProposalThread struct {
	ID             string
	IntersectionID string
	IssuerCircleID string
	ProposalType   ProposalType
	State          ProposalState
	Reason         string

	// Amendment details
	ScopeAdditions []ScopeChange
	ScopeRemovals  []string
	CeilingChanges []CeilingChange

	// Action spec for commitment proposals
	ActionSpec *primitives.ActionSpec

	// Approval tracking
	Approvals  map[string]bool   // circleID -> approved
	Rejections map[string]string // circleID -> reason

	// Counterproposal chain
	ParentID      string   // If this is a counter, points to parent
	CounterIDs    []string // IDs of counterproposals to this proposal
	ActiveCounter string   // The currently active counterproposal

	// Timestamps
	CreatedAt   string
	FinalizedAt string
}

// AcceptResult contains the result of accepting a proposal.
type AcceptResult struct {
	ProposalID     string
	AcceptorID     string
	AllAccepted    bool     // True if all parties have now accepted
	PendingParties []string // Parties that still need to accept
}

// FinalizeResult contains the result of finalizing a negotiation.
type FinalizeResult struct {
	ProposalID     string
	ResultType     string // "amendment" or "commitment"
	NewVersion     string // For amendments: the new contract version
	CommitmentID   string // For commitments: the new commitment ID
	IntersectionID string
}

// ModelRouter routes requests between SLM and LLM.
// Per Technology Selection: SLM-first with LLM escalation.
type ModelRouter interface {
	// Route determines which model to use for a request.
	Route(ctx context.Context, req ModelRequest) (ModelChoice, error)

	// InvokeSLM invokes the small language model.
	InvokeSLM(ctx context.Context, req ModelRequest) (*ModelResponse, error)

	// InvokeLLM invokes the large language model.
	InvokeLLM(ctx context.Context, req ModelRequest) (*ModelResponse, error)
}

// ExplainabilityCapture records model decision rationale.
type ExplainabilityCapture interface {
	// Capture records a model invocation with its rationale.
	Capture(ctx context.Context, record ExplainabilityRecord) error

	// GetExplanation retrieves the explanation for a decision.
	GetExplanation(ctx context.Context, decisionID string) (*ExplainabilityRecord, error)
}

// LoopIntentProcessor provides loop-aware intent processing.
// Used by the orchestrator at step 1 (Intent) of the Irreducible Loop.
type LoopIntentProcessor interface {
	// ProcessLoopIntent processes an intent within a loop context.
	// The loop context provides trace ID and issuer information.
	ProcessLoopIntent(ctx context.Context, loopCtx primitives.LoopContext, intent Intent) (*IntentResult, error)
}

// Intent represents the initial desire or goal for negotiation.
type Intent struct {
	ID             string
	IssuerCircleID string
	Type           string
	Description    string
	TargetCircleID string
	Parameters     map[string]string
}
