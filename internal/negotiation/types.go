package negotiation

import (
	"time"
)

// IntentResult contains the result of intent processing.
type IntentResult struct {
	IntentID           string
	Classification     string
	SuggestedAction    string
	IntersectionID     string
	RequiresProposal   bool
	RequiresEscalation bool
	Confidence         float64
	ModelUsed          string
}

// ProposalRequest contains parameters for creating a proposal.
type ProposalRequest struct {
	IntentID        string
	IssuerCircleID  string
	IntersectionID  string
	ScopesRequested []string
	Terms           map[string]string
	ExpiresIn       time.Duration
}

// ProposalAnalysis contains the analysis of a proposal.
type ProposalAnalysis struct {
	ProposalID     string
	RiskLevel      string // "low", "standard", "elevated", "high"
	Recommendation string // "accept", "counter", "reject", "escalate"
	Concerns       []string
	Confidence     float64
	ModelUsed      string
}

// Modification represents a change for a counterproposal.
type Modification struct {
	Field    string
	OldValue string
	NewValue string
	Reason   string
}

// ModelRequest contains a request to an LLM/SLM.
type ModelRequest struct {
	RequestID   string
	RequestType string // "classification", "analysis", "generation"
	Prompt      string
	Context     map[string]string
	RiskLevel   string
	MaxTokens   int
}

// ModelResponse contains a response from an LLM/SLM.
type ModelResponse struct {
	RequestID      string
	ModelID        string
	ModelVersion   string
	Response       string
	Confidence     float64
	ReasoningTrace string
	TokensUsed     int
	Latency        time.Duration
}

// ModelChoice indicates which model to use.
type ModelChoice struct {
	UseSLM          bool
	UseLLM          bool
	Reason          string
	EscalationCause string
}

// ExplainabilityRecord captures model decision rationale.
type ExplainabilityRecord struct {
	DecisionID       string
	RequestID        string
	ModelID          string
	ModelVersion     string
	InputHash        string // Hash of prompt for reproducibility
	Output           string
	Confidence       float64
	ReasoningTrace   string
	EscalationReason string
	Timestamp        time.Time
}
