package audit

import (
	"time"
)

// Entry represents an immutable audit log entry.
type Entry struct {
	ID             string
	TenantID       string
	CircleID       string
	IntersectionID string
	EventType      string
	SubjectID      string
	SubjectType    string
	Action         string
	Outcome        string
	Timestamp      time.Time
	PreviousHash   string
	Hash           string
	Metadata       map[string]string
}

// Explanation captures the rationale for a decision.
type Explanation struct {
	EntryID        string
	DecisionType   string
	ModelUsed      string
	ModelVersion   string
	InputSummary   string
	ReasoningTrace string
	Confidence     float64
	Timestamp      time.Time
}

// Filter specifies criteria for querying audit entries.
type Filter struct {
	CircleID       string
	IntersectionID string
	EventType      string
	SubjectID      string
	After          time.Time
	Before         time.Time
	Limit          int
	Offset         int
}

// VerificationResult contains hash chain verification results.
type VerificationResult struct {
	Valid          bool
	EntriesChecked int
	FirstEntry     string
	LastEntry      string
	BrokenAt       string // Empty if valid
	Error          string
}

// ChainHead contains information about the hash chain head.
type ChainHead struct {
	OwnerID      string
	EntryID      string
	Hash         string
	EntryCount   int
	LastModified time.Time
}

// Anchor represents an external anchor for the hash chain.
type Anchor struct {
	OwnerID    string
	ChainHash  string
	AnchorID   string
	AnchorType string // e.g., "timestamp_service"
	AnchoredAt time.Time
}

// ExportPackage contains a complete audit export.
type ExportPackage struct {
	CircleID     string
	Entries      []Entry
	Explanations []Explanation
	ChainHead    ChainHead
	Anchors      []Anchor
	ExportedAt   time.Time
	Signature    []byte
}

// GovernanceCheck represents an operation to check against rules.
type GovernanceCheck struct {
	OperationType  string
	CircleID       string
	IntersectionID string
	Parameters     map[string]string
}

// GovernanceResult contains the result of a governance check.
type GovernanceResult struct {
	Allowed    bool
	Violations []Violation
}

// Violation represents a governance rule violation.
type Violation struct {
	RuleID      string
	RuleName    string
	Description string
	Severity    string // "warning", "error", "critical"
	Timestamp   time.Time
}
