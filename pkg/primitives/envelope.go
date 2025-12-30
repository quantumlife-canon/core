// Package primitives provides core domain types.
// This file defines the ExecutionEnvelope for connector operations.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package primitives

import (
	"errors"
	"time"
)

// ExecutionEnvelope wraps all connector operations with traceability context.
// Every provider call must be enveloped with this context for audit and authorization.
//
// This envelope enforces:
// - Mode must be SuggestOnly or Simulate for read operations
// - Execute mode requires ApprovedByHuman=true for write operations
// - IntersectionID required for shared reads
// - ScopesUsed must be non-empty
// - AuthorizationProofID links to the authorization check
type ExecutionEnvelope struct {
	// TraceID links this operation to a distributed trace.
	TraceID string

	// Mode specifies the run mode (suggest_only, simulate, execute).
	Mode RunMode

	// ActorCircleID identifies the circle initiating the operation.
	ActorCircleID string

	// IntersectionID identifies the intersection authorizing the operation.
	// Required for all shared resource access.
	IntersectionID string

	// ContractVersion is the version of the contract used for authorization.
	ContractVersion string

	// ScopesUsed lists the scopes being exercised for this operation.
	ScopesUsed []string

	// AuthorizationProofID links to the AuthorizationProof that authorized this operation.
	AuthorizationProofID string

	// IssuedAt is when this envelope was created.
	IssuedAt time.Time

	// ApprovedByHuman indicates explicit human approval for Execute mode.
	// REQUIRED: Must be true for any write operations.
	// v6: This is set via CLI --approve flag or explicit API consent.
	ApprovedByHuman bool

	// ApprovalArtifact records how approval was obtained.
	// Examples: "cli:--approve", "api:explicit_consent", "ui:approval_dialog"
	// v6: Required when ApprovedByHuman is true.
	ApprovalArtifact string
}

// Envelope validation errors.
var (
	// ErrEnvelopeExecuteModeNotAllowed is returned when execute mode is used without approval.
	ErrEnvelopeExecuteModeNotAllowed = errors.New("execute mode is not allowed in envelope; use suggest_only or simulate")

	// ErrEnvelopeIntersectionIDRequired is returned when intersection ID is missing.
	ErrEnvelopeIntersectionIDRequired = errors.New("intersection ID is required for shared reads")

	// ErrEnvelopeScopesRequired is returned when scopes are empty.
	ErrEnvelopeScopesRequired = errors.New("scopes used must be non-empty")

	// ErrEnvelopeTraceIDRequired is returned when trace ID is missing.
	ErrEnvelopeTraceIDRequired = errors.New("trace ID is required for audit")

	// ErrEnvelopeActorCircleIDRequired is returned when actor circle ID is missing.
	ErrEnvelopeActorCircleIDRequired = errors.New("actor circle ID is required")

	// ErrEnvelopeAuthProofIDRequired is returned when authorization proof ID is missing.
	ErrEnvelopeAuthProofIDRequired = errors.New("authorization proof ID is required")

	// ErrEnvelopeApprovalRequired is returned when Execute mode is used without human approval.
	// v6: Execute mode requires explicit human approval via CLI --approve flag or equivalent.
	ErrEnvelopeApprovalRequired = errors.New("execute mode requires explicit human approval (use --approve flag)")

	// ErrEnvelopeWriteScopeRequired is returned when write operation lacks calendar:write scope.
	ErrEnvelopeWriteScopeRequired = errors.New("calendar:write scope required for write operations")
)

// Validate checks that the envelope has all required fields and valid mode.
func (e *ExecutionEnvelope) Validate() error {
	// Mode must be suggest_only or simulate
	if e.Mode == ModeExecute {
		return ErrEnvelopeExecuteModeNotAllowed
	}
	if e.Mode != ModeSuggestOnly && e.Mode != ModeSimulate {
		return ErrInvalidRunMode
	}

	// Required fields
	if e.TraceID == "" {
		return ErrEnvelopeTraceIDRequired
	}
	if e.ActorCircleID == "" {
		return ErrEnvelopeActorCircleIDRequired
	}
	if e.IntersectionID == "" {
		return ErrEnvelopeIntersectionIDRequired
	}
	if len(e.ScopesUsed) == 0 {
		return ErrEnvelopeScopesRequired
	}
	if e.AuthorizationProofID == "" {
		return ErrEnvelopeAuthProofIDRequired
	}

	return nil
}

// ValidateForRead validates the envelope for read-only operations.
// This is a stricter validation that ensures no write scopes are used.
func (e *ExecutionEnvelope) ValidateForRead() error {
	if err := e.Validate(); err != nil {
		return err
	}

	// Check that only read scopes are used
	for _, scope := range e.ScopesUsed {
		if !IsReadOnlyScope(scope) {
			return errors.New("write scope not allowed for read operations: " + scope)
		}
	}

	return nil
}

// ValidateForWrite validates the envelope for write operations.
// v6: Write operations have stricter requirements:
// - Mode MUST be Execute
// - ApprovedByHuman MUST be true
// - calendar:write scope MUST be present
//
// CRITICAL: This is the gatekeeper for all external writes.
// If this validation passes, the operation is authorized to modify external state.
func (e *ExecutionEnvelope) ValidateForWrite() error {
	// Basic field validation (same as Validate but without mode restriction)
	if e.TraceID == "" {
		return ErrEnvelopeTraceIDRequired
	}
	if e.ActorCircleID == "" {
		return ErrEnvelopeActorCircleIDRequired
	}
	if e.IntersectionID == "" {
		return ErrEnvelopeIntersectionIDRequired
	}
	if len(e.ScopesUsed) == 0 {
		return ErrEnvelopeScopesRequired
	}
	if e.AuthorizationProofID == "" {
		return ErrEnvelopeAuthProofIDRequired
	}

	// CRITICAL: Execute mode is required for writes
	if e.Mode != ModeExecute {
		return ErrEnvelopeExecuteModeNotAllowed
	}

	// CRITICAL: Human approval is required for writes
	if !e.ApprovedByHuman {
		return ErrEnvelopeApprovalRequired
	}

	// CRITICAL: calendar:write scope must be present
	hasWriteScope := false
	for _, scope := range e.ScopesUsed {
		if scope == "calendar:write" {
			hasWriteScope = true
			break
		}
	}
	if !hasWriteScope {
		return ErrEnvelopeWriteScopeRequired
	}

	return nil
}

// HasApproval returns true if the envelope has explicit human approval.
func (e *ExecutionEnvelope) HasApproval() bool {
	return e.ApprovedByHuman && e.ApprovalArtifact != ""
}

// IsWriteOperation returns true if the envelope contains write scopes.
func (e *ExecutionEnvelope) IsWriteOperation() bool {
	for _, scope := range e.ScopesUsed {
		if !IsReadOnlyScope(scope) {
			return true
		}
	}
	return false
}

// IsReadOnlyScope returns true if the scope is read-only.
// Write scopes (ending with :write) return false.
func IsReadOnlyScope(scope string) bool {
	// Scopes ending with :write are write scopes
	if len(scope) > 6 && scope[len(scope)-6:] == ":write" {
		return false
	}
	return true
}

// NewExecutionEnvelope creates a new execution envelope with the given parameters.
// The IssuedAt field is set to the provided timestamp.
// Note: For write operations, use NewExecutionEnvelopeWithApproval instead.
func NewExecutionEnvelope(
	traceID string,
	mode RunMode,
	actorCircleID string,
	intersectionID string,
	contractVersion string,
	scopesUsed []string,
	authProofID string,
	issuedAt time.Time,
) *ExecutionEnvelope {
	return &ExecutionEnvelope{
		TraceID:              traceID,
		Mode:                 mode,
		ActorCircleID:        actorCircleID,
		IntersectionID:       intersectionID,
		ContractVersion:      contractVersion,
		ScopesUsed:           scopesUsed,
		AuthorizationProofID: authProofID,
		IssuedAt:             issuedAt,
		ApprovedByHuman:      false, // Default: no approval
		ApprovalArtifact:     "",
	}
}

// NewExecutionEnvelopeWithApproval creates an envelope for Execute mode with explicit approval.
// v6: This is the ONLY way to create a valid envelope for write operations.
//
// Parameters:
//   - approvalArtifact: How approval was obtained (e.g., "cli:--approve")
//
// CRITICAL: This function sets ApprovedByHuman=true, enabling write operations.
func NewExecutionEnvelopeWithApproval(
	traceID string,
	actorCircleID string,
	intersectionID string,
	contractVersion string,
	scopesUsed []string,
	authProofID string,
	issuedAt time.Time,
	approvalArtifact string,
) *ExecutionEnvelope {
	return &ExecutionEnvelope{
		TraceID:              traceID,
		Mode:                 ModeExecute, // Always Execute for approved operations
		ActorCircleID:        actorCircleID,
		IntersectionID:       intersectionID,
		ContractVersion:      contractVersion,
		ScopesUsed:           scopesUsed,
		AuthorizationProofID: authProofID,
		IssuedAt:             issuedAt,
		ApprovedByHuman:      true,
		ApprovalArtifact:     approvalArtifact,
	}
}

// Copy creates a copy of the envelope.
func (e *ExecutionEnvelope) Copy() *ExecutionEnvelope {
	scopesCopy := make([]string, len(e.ScopesUsed))
	copy(scopesCopy, e.ScopesUsed)

	return &ExecutionEnvelope{
		TraceID:              e.TraceID,
		Mode:                 e.Mode,
		ActorCircleID:        e.ActorCircleID,
		IntersectionID:       e.IntersectionID,
		ContractVersion:      e.ContractVersion,
		ScopesUsed:           scopesCopy,
		AuthorizationProofID: e.AuthorizationProofID,
		IssuedAt:             e.IssuedAt,
		ApprovedByHuman:      e.ApprovedByHuman,
		ApprovalArtifact:     e.ApprovalArtifact,
	}
}
