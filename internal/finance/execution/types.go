// Package execution implements v9 financial execution primitives.
//
// CRITICAL: This package is for DRY-RUN ONLY in v9 Slice 1.
// NO REAL MONEY MOVES. NO PROVIDER WRITE CALLS.
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SettlementStatus represents the outcome of an execution attempt.
// In v9 Slice 1 (dry-run), only non-success statuses are valid.
type SettlementStatus string

const (
	// SettlementPending indicates execution has not been attempted.
	SettlementPending SettlementStatus = "pending"

	// SettlementBlocked indicates execution was blocked by precondition failure.
	SettlementBlocked SettlementStatus = "blocked"

	// SettlementRevoked indicates execution was halted by revocation.
	SettlementRevoked SettlementStatus = "revoked"

	// SettlementExpired indicates approval or envelope expired before execution.
	SettlementExpired SettlementStatus = "expired"

	// SettlementAborted indicates execution was aborted due to error.
	SettlementAborted SettlementStatus = "aborted"

	// SettlementSuccessful is FORBIDDEN in v9 Slice 1.
	// This constant exists only to detect violations.
	SettlementSuccessful SettlementStatus = "settled_successfully"

	// SettlementSimulated indicates execution was simulated (mock connector).
	// CRITICAL: This status means NO real money was moved.
	// Used when TrueLayer is not configured and mock connector is active.
	SettlementSimulated SettlementStatus = "simulated"
)

// ExecutionIntent represents the initial request for financial execution.
// This is created before any approval or envelope construction.
type ExecutionIntent struct {
	// IntentID uniquely identifies this intent.
	IntentID string

	// CircleID is the circle initiating execution.
	CircleID string

	// IntersectionID is the intersection context (if multi-party).
	IntersectionID string

	// Description is a neutral, factual description of the intent.
	// MUST NOT contain urgency, fear, authority, or optimization language.
	Description string

	// ActionType specifies the type of financial action.
	ActionType ActionType

	// AmountCents is the amount in cents (for display/audit only in dry-run).
	AmountCents int64

	// Currency is the currency code.
	Currency string

	// PayeeID is the pre-registered payee identifier.
	// CRITICAL (v9.10): No free-text recipients allowed.
	// This MUST reference a registered payee in the payee registry.
	PayeeID string

	// ViewHash is the ContentHash of the v8 SharedFinancialView this is based on.
	ViewHash string

	// CreatedAt is when the intent was created.
	CreatedAt time.Time
}

// ActionType specifies the category of financial action.
type ActionType string

const (
	ActionTypePayment  ActionType = "payment"
	ActionTypeTransfer ActionType = "transfer"
)

// ExecutionEnvelope is the sealed, immutable container for execution.
// Per Technical Split v9 §4, once sealed this cannot be modified.
//
// REQUIRED FIELDS (per Technical Split v9 §4.1):
// - EnvelopeID, ActorCircleID, IntersectionID, ViewHash, ActionHash
// - ActionSpec, AmountCap, FrequencyCap, DurationCap, Expiry
// - Approvals, ApprovalThreshold, RevocationWindowStart, RevocationWindowEnd
// - RevocationWaived, TraceID, SealedAt, SealHash
//
// FORBIDDEN FIELDS (per Technical Split v9 §4.2):
// - Probabilistic scores, "Recommended" flags, "Urgency" indicators
// - Optimization hints, Batch identifiers, Retry counters, Fallback specs
type ExecutionEnvelope struct {
	// EnvelopeID uniquely identifies this envelope.
	EnvelopeID string

	// ActorCircleID is the circle initiating execution.
	ActorCircleID string

	// IntersectionID is the intersection context (empty for single-party).
	IntersectionID string

	// ViewHash is the ContentHash of the referenced v8 SharedFinancialView.
	ViewHash string

	// ActionHash cryptographically binds all action parameters.
	ActionHash string

	// ActionSpec is the exact specification of what to execute.
	ActionSpec ActionSpec

	// AmountCap is the maximum amount (hard ceiling) in cents.
	AmountCap int64

	// FrequencyCap is the maximum frequency (1 for single execution).
	FrequencyCap int

	// DurationCap is the maximum duration of authority.
	DurationCap time.Duration

	// Expiry is when this envelope becomes invalid.
	Expiry time.Time

	// Approvals is the list of approval artifacts.
	Approvals []ApprovalArtifact

	// ApprovalThreshold is the required approval count.
	ApprovalThreshold int

	// RevocationWindowStart is when revocation window opened.
	RevocationWindowStart time.Time

	// RevocationWindowEnd is when revocation window closes.
	RevocationWindowEnd time.Time

	// RevocationWaived is true only if explicitly waived for this action.
	RevocationWaived bool

	// TraceID is the correlation ID for audit reconstruction.
	TraceID string

	// SealedAt is when envelope was sealed.
	SealedAt time.Time

	// SealHash is the hash of all fields proving immutability.
	SealHash string

	// PolicySnapshotHash is the v9.12 policy snapshot hash.
	// CRITICAL: Binds this envelope to a specific policy configuration.
	// Execution will be blocked if current policy hash doesn't match.
	PolicySnapshotHash string

	// --- Internal state (not part of seal) ---

	// Revoked indicates if this envelope has been revoked.
	Revoked bool

	// RevokedAt is when revocation occurred.
	RevokedAt time.Time

	// RevokedBy is who revoked.
	RevokedBy string
}

// ActionSpec specifies exactly what action to execute.
type ActionSpec struct {
	// Type is the action type.
	Type ActionType

	// AmountCents is the exact amount.
	AmountCents int64

	// Currency is the currency code.
	Currency string

	// PayeeID is the pre-registered payee identifier.
	// CRITICAL (v9.10): No free-text recipients allowed.
	PayeeID string

	// Description is a neutral description.
	Description string
}

// ApprovalArtifact is a signed, timestamped approval bound to an ActionHash.
// Per Technical Split v9 §5.5, this provides non-repudiation.
type ApprovalArtifact struct {
	// ArtifactID uniquely identifies this artifact.
	ArtifactID string

	// ApproverCircleID is the circle providing approval.
	ApproverCircleID string

	// ApproverID is the specific approver within the circle.
	ApproverID string

	// ActionHash binds this approval to a specific action.
	ActionHash string

	// ApprovedAt is when approval was given.
	ApprovedAt time.Time

	// ExpiresAt is when this approval expires.
	ExpiresAt time.Time

	// Signature is the cryptographic signature.
	Signature string

	// SignatureAlgorithm identifies the signature algorithm.
	SignatureAlgorithm string
}

// IsExpired returns true if the approval has expired.
func (a *ApprovalArtifact) IsExpired(now time.Time) bool {
	return now.After(a.ExpiresAt)
}

// ApprovalRequest represents a request for approval.
// Language MUST be neutral per Canon Addendum v9 §3.6.
type ApprovalRequest struct {
	// RequestID uniquely identifies this request.
	RequestID string

	// EnvelopeID is the envelope requiring approval.
	EnvelopeID string

	// ActionHash is the hash to approve.
	ActionHash string

	// PromptText is the approval prompt (MUST be neutral).
	PromptText string

	// RequestedAt is when approval was requested.
	RequestedAt time.Time

	// ExpiresAt is when the request expires.
	ExpiresAt time.Time

	// TargetCircleID is who should approve.
	TargetCircleID string
}

// RevocationSignal represents a revocation request.
type RevocationSignal struct {
	// SignalID uniquely identifies this signal.
	SignalID string

	// EnvelopeID is the envelope being revoked.
	EnvelopeID string

	// RevokerCircleID is who is revoking.
	RevokerCircleID string

	// RevokerID is the specific revoker.
	RevokerID string

	// RevokedAt is when revocation was signaled.
	RevokedAt time.Time

	// Reason is an optional reason (factual only).
	Reason string
}

// ValidityCheckResult represents the outcome of an affirmative validity check.
// Per Canon Addendum v9 §8.3, absence of revocation alone is insufficient.
type ValidityCheckResult struct {
	// Valid is true only if all conditions are affirmatively satisfied.
	Valid bool

	// CheckedAt is when the check was performed.
	CheckedAt time.Time

	// Conditions lists each condition and its result.
	Conditions []ConditionResult

	// FailureReason is set if Valid is false.
	FailureReason string
}

// ConditionResult represents a single condition check.
type ConditionResult struct {
	// Condition is the name of the condition.
	Condition string

	// Satisfied is true if the condition is met.
	Satisfied bool

	// Details provides additional information.
	Details string
}

// ExecutionResult represents the outcome of an execution attempt.
type ExecutionResult struct {
	// EnvelopeID is the envelope that was processed.
	EnvelopeID string

	// Status is the settlement status.
	Status SettlementStatus

	// AttemptedAt is when execution was attempted.
	AttemptedAt time.Time

	// CompletedAt is when the attempt completed.
	CompletedAt time.Time

	// ValidityCheck is the validity check result.
	ValidityCheck ValidityCheckResult

	// BlockedReason is set if Status is blocked.
	BlockedReason string

	// RevokedBy is set if Status is revoked.
	RevokedBy string

	// AuditTraceID links to the full audit trail.
	AuditTraceID string
}

// ComputeActionHash computes the ActionHash for an intent.
// This binds together all action parameters cryptographically.
func ComputeActionHash(intent ExecutionIntent) string {
	h := sha256.New()
	h.Write([]byte(intent.IntentID))
	h.Write([]byte(intent.CircleID))
	h.Write([]byte(intent.IntersectionID))
	h.Write([]byte(intent.ActionType))
	h.Write([]byte(fmt.Sprintf("%d", intent.AmountCents)))
	h.Write([]byte(intent.Currency))
	h.Write([]byte(intent.PayeeID)) // v9.10: PayeeID instead of free-text Recipient
	h.Write([]byte(intent.ViewHash))
	h.Write([]byte(intent.CreatedAt.Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeSealHash computes the seal hash for an envelope.
// Once computed, the envelope is immutable.
func ComputeSealHash(env *ExecutionEnvelope) string {
	h := sha256.New()
	h.Write([]byte(env.EnvelopeID))
	h.Write([]byte(env.ActorCircleID))
	h.Write([]byte(env.IntersectionID))
	h.Write([]byte(env.ViewHash))
	h.Write([]byte(env.ActionHash))
	h.Write([]byte(env.ActionSpec.Type))
	h.Write([]byte(fmt.Sprintf("%d", env.ActionSpec.AmountCents)))
	h.Write([]byte(env.ActionSpec.Currency))
	h.Write([]byte(env.ActionSpec.PayeeID)) // v9.10: PayeeID instead of free-text Recipient
	h.Write([]byte(fmt.Sprintf("%d", env.AmountCap)))
	h.Write([]byte(fmt.Sprintf("%d", env.FrequencyCap)))
	h.Write([]byte(fmt.Sprintf("%d", env.DurationCap)))
	h.Write([]byte(env.Expiry.Format(time.RFC3339Nano)))
	h.Write([]byte(fmt.Sprintf("%d", env.ApprovalThreshold)))
	h.Write([]byte(env.RevocationWindowStart.Format(time.RFC3339Nano)))
	h.Write([]byte(env.RevocationWindowEnd.Format(time.RFC3339Nano)))
	h.Write([]byte(fmt.Sprintf("%t", env.RevocationWaived)))
	h.Write([]byte(env.TraceID))
	h.Write([]byte(env.SealedAt.Format(time.RFC3339Nano)))
	h.Write([]byte(env.PolicySnapshotHash)) // v9.12: Policy snapshot binding
	return hex.EncodeToString(h.Sum(nil))
}
