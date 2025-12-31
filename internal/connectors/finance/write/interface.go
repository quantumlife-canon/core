// Package write provides financial write connector interfaces.
//
// CRITICAL: v9 Vertical Slice 3 is the FIRST slice where money may actually move.
// It must be minimal, constrained, auditable, interruptible, and boring.
//
// HARD SAFETY CONSTRAINTS (NON-NEGOTIABLE):
// 1) Provider: TrueLayer ONLY
// 2) Cap: DEFAULT hard cap = £1.00 (100 pence)
// 3) No free-text recipients - pre-defined payee IDs only
// 4) No standing/blanket approvals
// 5) Approval must be action-hash bound and single-use
// 6) Revocation window must be enforced
// 7) Forced pause between approval verification and execute attempt
// 8) No retries - failures require new approval
// 9) Execution must be fully abortable before external call
// 10) Audit must reconstruct the full story
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
// - docs/HUMAN_GUARANTEES_V1.md
package write

import (
	"context"
	"errors"
	"fmt"
	"time"

	"quantumlife/pkg/events"
)

// DefaultCapCents is the default hard cap in cents (£1.00 = 100 pence).
// CRITICAL: This is intentionally tiny for v9 Slice 3.
const DefaultCapCents int64 = 100

// ExecutionEnvelope represents the sealed execution envelope.
// This is a minimal interface to avoid import cycles.
type ExecutionEnvelope struct {
	EnvelopeID            string
	ActorCircleID         string
	IntersectionID        string
	ActionHash            string
	SealHash              string
	AmountCap             int64
	Expiry                time.Time
	RevocationWindowStart time.Time
	RevocationWindowEnd   time.Time
	RevocationWaived      bool
	Revoked               bool
	RevokedAt             time.Time
	RevokedBy             string
	ActionSpec            ActionSpec
}

// ActionSpec specifies what action to execute.
type ActionSpec struct {
	Type        string
	AmountCents int64
	Currency    string
	Recipient   string
	Description string
}

// ApprovalArtifact represents a signed approval.
type ApprovalArtifact struct {
	ArtifactID         string
	ApproverCircleID   string
	ApproverID         string
	ActionHash         string
	ApprovedAt         time.Time
	ExpiresAt          time.Time
	Signature          string
	SignatureAlgorithm string
}

// IsExpired returns true if the approval has expired.
func (a *ApprovalArtifact) IsExpired(now time.Time) bool {
	return now.After(a.ExpiresAt)
}

// WriteConnector defines the interface for financial write operations.
//
// CRITICAL: Only TrueLayer is implemented in v9 Slice 3.
// This interface exists to prove the pattern, not to enable multiple providers.
//
// v9.9 REQUIREMENT: Providers MUST expose their ProviderID for registry enforcement.
// Executors MUST consult the registry before invoking any WriteConnector.
type WriteConnector interface {
	// Provider returns the provider name (legacy - use ProviderID for v9.9+).
	Provider() string

	// ProviderID returns the canonical provider identifier for registry lookup.
	// This MUST match a registered provider ID in the write provider registry.
	// v9.9: Required for provider allowlist enforcement.
	ProviderID() string

	// ProviderInfo returns the provider identifier and environment.
	// Returns (id, environment) where environment is "sandbox", "live", or "mock".
	// v9.9: Used by executors for audit events and registry enforcement.
	ProviderInfo() (id string, env string)

	// Prepare validates that the payment can be executed.
	// This performs pre-execution checks WITHOUT side effects.
	// MUST validate: execute mode, scopes, approval, cap.
	Prepare(ctx context.Context, req PrepareRequest) (*PrepareResult, error)

	// Execute creates the payment with the provider.
	// CRITICAL: This is the ONLY method that can move money.
	// Returns receipt on success, error on failure.
	// NO RETRIES. Failures require new approval.
	Execute(ctx context.Context, req ExecuteRequest) (*PaymentReceipt, error)

	// Abort cancels execution before provider call if possible.
	// Returns true if abort was successful.
	Abort(ctx context.Context, envelopeID string) (bool, error)
}

// PrepareRequest contains parameters for Prepare.
type PrepareRequest struct {
	// Envelope is the sealed execution envelope.
	Envelope *ExecutionEnvelope

	// Approval is the approval artifact.
	Approval *ApprovalArtifact

	// PayeeID is the pre-defined payee identifier.
	// CRITICAL: No free-text recipients allowed.
	PayeeID string

	// Now is the current time for validation.
	Now time.Time
}

// PrepareResult contains the result of preparation.
type PrepareResult struct {
	// Valid indicates if the payment can proceed.
	Valid bool

	// InvalidReason explains why the payment cannot proceed.
	InvalidReason string

	// ValidationDetails provides detailed validation results.
	ValidationDetails []ValidationDetail

	// PreparedAt is when preparation completed.
	PreparedAt time.Time
}

// ValidationDetail represents a single validation check.
type ValidationDetail struct {
	// Check is the name of the validation check.
	Check string

	// Passed indicates if the check passed.
	Passed bool

	// Details provides additional information.
	Details string
}

// ExecuteRequest contains parameters for Execute.
type ExecuteRequest struct {
	// Envelope is the sealed execution envelope.
	Envelope *ExecutionEnvelope

	// Approval is the approval artifact.
	Approval *ApprovalArtifact

	// PayeeID is the pre-defined payee identifier.
	PayeeID string

	// IdempotencyKey prevents duplicate payments.
	IdempotencyKey string

	// Now is the current time.
	Now time.Time
}

// PaymentReceipt is the proof of payment from the provider.
type PaymentReceipt struct {
	// ReceiptID uniquely identifies this receipt.
	ReceiptID string

	// EnvelopeID is the envelope that was executed.
	EnvelopeID string

	// ProviderRef is the provider's reference for this payment.
	ProviderRef string

	// Status is the payment status from provider.
	Status PaymentStatus

	// AmountCents is the amount that was transferred.
	AmountCents int64

	// Currency is the currency of the transfer.
	Currency string

	// PayeeID is the payee that received the payment.
	PayeeID string

	// CreatedAt is when the payment was created.
	CreatedAt time.Time

	// CompletedAt is when the payment was completed.
	CompletedAt time.Time

	// ProviderMetadata contains provider-specific details.
	ProviderMetadata map[string]string

	// Simulated indicates this receipt is from a mock/simulated execution.
	// CRITICAL: When Simulated=true, NO real money was moved.
	Simulated bool
}

// PaymentStatus represents the status of a payment.
type PaymentStatus string

const (
	// PaymentPending indicates the payment is pending.
	PaymentPending PaymentStatus = "pending"

	// PaymentExecuting indicates the payment is being executed.
	PaymentExecuting PaymentStatus = "executing"

	// PaymentSucceeded indicates the payment succeeded.
	PaymentSucceeded PaymentStatus = "succeeded"

	// PaymentFailed indicates the payment failed.
	PaymentFailed PaymentStatus = "failed"

	// PaymentAborted indicates the payment was aborted.
	PaymentAborted PaymentStatus = "aborted"

	// PaymentSimulated indicates the payment was simulated (mock connector).
	// CRITICAL: This status means NO real money was moved.
	PaymentSimulated PaymentStatus = "simulated"
)

// Payee represents a pre-defined payment recipient.
//
// CRITICAL: Recipients MUST be pre-defined. No free-text allowed.
type Payee struct {
	// ID is the unique payee identifier.
	ID string

	// Name is the display name.
	Name string

	// AccountIdentifier is the bank account identifier.
	// For TrueLayer sandbox, this is a sandbox beneficiary ID.
	AccountIdentifier string

	// Currency is the supported currency.
	Currency string

	// IsSandbox indicates if this is a sandbox payee.
	IsSandbox bool
}

// PayeeRegistry manages pre-defined payees.
type PayeeRegistry struct {
	payees map[string]Payee
}

// NewPayeeRegistry creates a new payee registry.
func NewPayeeRegistry() *PayeeRegistry {
	return &PayeeRegistry{
		payees: make(map[string]Payee),
	}
}

// Register registers a payee.
func (r *PayeeRegistry) Register(payee Payee) {
	r.payees[payee.ID] = payee
}

// Get returns a payee by ID.
func (r *PayeeRegistry) Get(id string) (Payee, bool) {
	payee, ok := r.payees[id]
	return payee, ok
}

// List returns all registered payees.
func (r *PayeeRegistry) List() []Payee {
	payees := make([]Payee, 0, len(r.payees))
	for _, p := range r.payees {
		payees = append(payees, p)
	}
	return payees
}

// SandboxPayees returns pre-defined sandbox payees for testing.
func SandboxPayees() []Payee {
	return []Payee{
		{
			ID:                "sandbox-utility",
			Name:              "Sandbox Utility Provider",
			AccountIdentifier: "sandbox-beneficiary-utility",
			Currency:          "GBP",
			IsSandbox:         true,
		},
		{
			ID:                "sandbox-merchant",
			Name:              "Sandbox Test Merchant",
			AccountIdentifier: "sandbox-beneficiary-merchant",
			Currency:          "GBP",
			IsSandbox:         true,
		},
	}
}

// WriteConfig contains configuration for write operations.
type WriteConfig struct {
	// CapCents is the hard cap in cents.
	// Defaults to DefaultCapCents (100 = £1.00).
	CapCents int64

	// AllowedCurrencies is the list of allowed currencies.
	AllowedCurrencies []string

	// ForcedPauseDuration is the mandatory pause before execution.
	ForcedPauseDuration time.Duration

	// SandboxMode indicates if sandbox mode is enabled.
	SandboxMode bool

	// AuditEmitter emits audit events.
	AuditEmitter func(event events.Event)
}

// DefaultWriteConfig returns the default configuration.
func DefaultWriteConfig() WriteConfig {
	return WriteConfig{
		CapCents:            DefaultCapCents,
		AllowedCurrencies:   []string{"GBP"},
		ForcedPauseDuration: 2 * time.Second,
		SandboxMode:         true,
	}
}

// Validation errors.
var (
	// ErrCapExceeded is returned when amount exceeds the hard cap.
	ErrCapExceeded = errors.New("amount exceeds hard cap")

	// ErrInvalidPayee is returned when payee is not pre-defined.
	ErrInvalidPayee = errors.New("payee must be pre-defined (no free-text recipients)")

	// ErrMissingApproval is returned when approval is missing.
	ErrMissingApproval = errors.New("explicit approval required")

	// ErrApprovalExpired is returned when approval has expired.
	ErrApprovalExpired = errors.New("approval has expired")

	// ErrApprovalHashMismatch is returned when approval doesn't match action.
	ErrApprovalHashMismatch = errors.New("approval action hash does not match envelope")

	// ErrEnvelopeExpired is returned when envelope has expired.
	ErrEnvelopeExpired = errors.New("envelope has expired")

	// ErrEnvelopeRevoked is returned when envelope has been revoked.
	ErrEnvelopeRevoked = errors.New("envelope has been revoked")

	// ErrRevocationWindowActive is returned when revocation window hasn't closed.
	ErrRevocationWindowActive = errors.New("revocation window is still active")

	// ErrForbiddenCurrency is returned when currency is not allowed.
	ErrForbiddenCurrency = errors.New("currency not allowed")

	// ErrExecutionAborted is returned when execution was aborted.
	ErrExecutionAborted = errors.New("execution was aborted before provider call")

	// ErrProviderNotConfigured is returned when provider credentials are missing.
	ErrProviderNotConfigured = errors.New("provider credentials not configured")

	// ErrNoRetries is returned to indicate no retries are allowed.
	ErrNoRetries = errors.New("failures require new approval - no retries")
)

// ForbiddenFieldError is returned when envelope contains forbidden fields.
type ForbiddenFieldError struct {
	Field  string
	Reason string
}

func (e *ForbiddenFieldError) Error() string {
	return fmt.Sprintf("forbidden field detected: %s (%s)", e.Field, e.Reason)
}
