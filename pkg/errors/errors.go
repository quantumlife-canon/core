// Package errors defines common error types used across QuantumLife.
// These errors represent canon-level failures and boundary violations.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §6 Failure, Revocation, and Safety Semantics
package errors

import "errors"

// Authority errors — returned when authority checks fail.
var (
	// ErrUnauthorized is returned when an operation lacks required authority.
	ErrUnauthorized = errors.New("unauthorized: insufficient authority grant")

	// ErrAuthorityExpired is returned when an authority grant has expired.
	ErrAuthorityExpired = errors.New("unauthorized: authority grant expired")

	// ErrAuthorityRevoked is returned when an authority grant has been revoked.
	ErrAuthorityRevoked = errors.New("unauthorized: authority grant revoked")

	// ErrScopeExceeded is returned when an operation exceeds granted scope.
	ErrScopeExceeded = errors.New("unauthorized: operation exceeds granted scope")

	// ErrCeilingExceeded is returned when an operation exceeds authority ceiling.
	ErrCeilingExceeded = errors.New("unauthorized: operation exceeds authority ceiling")
)

// Execution errors — returned during action execution.
var (
	// ErrActionAborted is returned when an action is aborted before completion.
	ErrActionAborted = errors.New("action aborted")

	// ErrActionPaused is returned when an action is paused.
	ErrActionPaused = errors.New("action paused")

	// ErrRevocationDuringExecution is returned when authority is revoked mid-action.
	// Per Canon: "There is no 'finish what you started' exception."
	ErrRevocationDuringExecution = errors.New("authority revoked during execution: action halted")

	// ErrSettlementFailed is returned when settlement cannot be completed.
	ErrSettlementFailed = errors.New("settlement failed")
)

// Intersection errors — returned for intersection violations.
var (
	// ErrIntersectionNotFound is returned when an intersection does not exist.
	ErrIntersectionNotFound = errors.New("intersection not found")

	// ErrIntersectionDissolved is returned when an intersection has been dissolved.
	ErrIntersectionDissolved = errors.New("intersection dissolved")

	// ErrConsentRequired is returned when all parties must consent but haven't.
	ErrConsentRequired = errors.New("consent required from all parties")

	// ErrVersionConflict is returned when an intersection version conflict occurs.
	ErrVersionConflict = errors.New("intersection version conflict")
)

// Circle errors — returned for circle violations.
var (
	// ErrCircleNotFound is returned when a circle does not exist.
	ErrCircleNotFound = errors.New("circle not found")

	// ErrCircleTerminated is returned when a circle has been terminated.
	ErrCircleTerminated = errors.New("circle terminated")

	// ErrCrossCircleViolation is returned when attempting to access another circle directly.
	ErrCrossCircleViolation = errors.New("cross-circle access violation: use intersection")
)

// Audit errors — returned for audit violations.
var (
	// ErrAuditWriteFailed is returned when an audit log entry cannot be written.
	ErrAuditWriteFailed = errors.New("audit log write failed")

	// ErrAuditChainBroken is returned when the audit hash chain is invalid.
	ErrAuditChainBroken = errors.New("audit hash chain integrity violation")
)

// Seal errors — returned for capability seal violations.
var (
	// ErrCapabilityNotCertified is returned when an uncertified capability is used without approval.
	ErrCapabilityNotCertified = errors.New("capability not certified: explicit approval required")

	// ErrCapabilityRevoked is returned when a capability seal has been revoked.
	ErrCapabilityRevoked = errors.New("capability seal revoked")

	// ErrInvalidManifest is returned when a capability manifest is invalid.
	ErrInvalidManifest = errors.New("invalid capability manifest")

	// ErrSignatureInvalid is returned when a capability signature is invalid.
	ErrSignatureInvalid = errors.New("capability signature invalid")
)
