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
