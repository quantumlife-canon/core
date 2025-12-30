// Package approval provides multi-party approval governance for execute-mode writes.
// This is a CONTROL PLANE component for managing approval workflows.
//
// CRITICAL: Approvals are intersection-scoped - no global policies.
// Each approval is bound to a specific action via ActionHash.
//
// Reference: v7 Multi-party approval governance
package approval

import (
	"context"
	"errors"
	"time"

	"quantumlife/pkg/primitives"
)

// ApprovalPolicy defines multi-party approval requirements for verification.
// This is a subset of intersection.ApprovalPolicy used for approval verification.
type ApprovalPolicy struct {
	// Mode defines the approval mode: "single" or "multi"
	Mode string

	// RequiredApprovers lists specific circle IDs that MUST approve.
	RequiredApprovers []string

	// Threshold is the minimum number of approvals required.
	Threshold int

	// ExpirySeconds defines how long an approval artifact is valid.
	ExpirySeconds int

	// AppliesToScopes lists which scopes require this policy.
	AppliesToScopes []string
}

// ApprovalPolicy mode constants.
const (
	ApprovalModeSingle = "single"
	ApprovalModeMulti  = "multi"
)

// ContractForApproval contains the contract fields needed for approval verification.
// This avoids importing the intersection package.
type ContractForApproval struct {
	// IntersectionID is the intersection this contract belongs to.
	IntersectionID string

	// ApprovalPolicy defines the approval requirements.
	ApprovalPolicy ApprovalPolicy

	// Parties lists the circle IDs that are party to this contract.
	Parties []string
}

// Store provides storage and retrieval of approval artifacts.
type Store interface {
	// StoreApproval stores an approval artifact.
	// Returns error if approval already exists for this circle+action.
	StoreApproval(ctx context.Context, approval *primitives.ApprovalArtifact) error

	// GetApprovals retrieves all approvals for an action.
	GetApprovals(ctx context.Context, intersectionID, actionID string) ([]*primitives.ApprovalArtifact, error)

	// GetApprovalByID retrieves a specific approval by ID.
	GetApprovalByID(ctx context.Context, approvalID string) (*primitives.ApprovalArtifact, error)

	// StoreRequestToken stores an approval request token.
	StoreRequestToken(ctx context.Context, token *primitives.ApprovalRequestToken) error

	// GetRequestToken retrieves a request token by ID.
	GetRequestToken(ctx context.Context, tokenID string) (*primitives.ApprovalRequestToken, error)

	// DeleteExpiredApprovals removes expired approvals.
	DeleteExpiredApprovals(ctx context.Context, before time.Time) (int, error)
}

// Requester creates approval request tokens.
type Requester interface {
	// RequestApproval creates an approval request token for an action.
	// The token can be shared with approvers to collect their approvals.
	RequestApproval(ctx context.Context, req ApprovalRequest) (*primitives.ApprovalRequestToken, error)
}

// Submitter handles approval submissions.
type Submitter interface {
	// SubmitApproval submits an approval for an action.
	// Validates the token and creates an ApprovalArtifact.
	SubmitApproval(ctx context.Context, req SubmitApprovalRequest) (*primitives.ApprovalArtifact, error)
}

// Verifier validates approvals against contract policy.
type Verifier interface {
	// VerifyApprovals checks if approvals satisfy the contract's ApprovalPolicy.
	// Returns a detailed result with pass/fail status and reasons.
	VerifyApprovals(ctx context.Context, req VerifyApprovalsRequest) (*primitives.ApprovalVerificationResult, error)
}

// Manager combines all approval operations.
type Manager interface {
	Store
	Requester
	Submitter
	Verifier
}

// ApprovalRequest contains parameters for requesting approval.
type ApprovalRequest struct {
	// IntersectionID is the intersection for this action.
	IntersectionID string

	// ContractVersion is the contract version.
	ContractVersion string

	// Action is the action requiring approval.
	Action *primitives.Action

	// ScopesRequired lists the scopes the action needs.
	ScopesRequired []string

	// RequestingCircleID is the circle creating the request.
	RequestingCircleID string

	// ExpirySeconds is how long the request token is valid.
	// If 0, uses the contract's ApprovalPolicy.ExpirySeconds.
	ExpirySeconds int

	// TraceID links to a distributed trace.
	TraceID string
}

// SubmitApprovalRequest contains parameters for submitting an approval.
type SubmitApprovalRequest struct {
	// Token is the encoded approval request token.
	Token string

	// ApproverCircleID is the circle submitting the approval.
	ApproverCircleID string

	// TraceID links to a distributed trace.
	TraceID string
}

// VerifyApprovalsRequest contains parameters for verifying approvals.
type VerifyApprovalsRequest struct {
	// Contract contains the contract fields needed for approval verification.
	Contract *ContractForApproval

	// Action is the action being executed.
	Action *primitives.Action

	// ActionHash is the computed hash of the action.
	ActionHash string

	// ScopesUsed lists the scopes being used.
	ScopesUsed []string

	// TraceID links to a distributed trace.
	TraceID string
}

// Errors for approval operations.
var (
	// ErrApprovalNotFound is returned when an approval is not found.
	ErrApprovalNotFound = errors.New("approval not found")

	// ErrRequestTokenNotFound is returned when a request token is not found.
	ErrRequestTokenNotFound = errors.New("approval request token not found")

	// ErrRequestTokenExpired is returned when a request token has expired.
	ErrRequestTokenExpired = errors.New("approval request token has expired")

	// ErrApprovalExpired is returned when an approval has expired.
	ErrApprovalExpired = errors.New("approval has expired")

	// ErrApprovalHashMismatch is returned when approval hash doesn't match action.
	ErrApprovalHashMismatch = errors.New("approval hash does not match action")

	// ErrDuplicateApproval is returned when circle has already approved.
	ErrDuplicateApproval = errors.New("circle has already submitted approval for this action")

	// ErrCircleNotAuthorized is returned when circle is not authorized to approve.
	ErrCircleNotAuthorized = errors.New("circle is not authorized to approve this action")

	// ErrInsufficientApprovals is returned when not enough approvals exist.
	ErrInsufficientApprovals = errors.New("insufficient approvals for execution")

	// ErrInvalidSignature is returned when a signature is invalid.
	ErrInvalidSignature = errors.New("invalid signature")

	// ErrApprovalPolicyNotMet is returned when approval policy is not satisfied.
	ErrApprovalPolicyNotMet = errors.New("approval policy requirements not met")
)
