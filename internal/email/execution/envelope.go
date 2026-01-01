// Package execution provides email execution boundary enforcement.
//
// CRITICAL: This is the ONLY path to external email writes.
// CRITICAL: Must verify policy and view snapshots before execution.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent - same envelope executed twice returns same result.
//
// Reference: Phase 7 Email Execution Boundary
package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
)

// EnvelopeStatus represents the execution status.
type EnvelopeStatus string

const (
	EnvelopeStatusPending  EnvelopeStatus = "pending"
	EnvelopeStatusExecuted EnvelopeStatus = "executed"
	EnvelopeStatusFailed   EnvelopeStatus = "failed"
	EnvelopeStatusBlocked  EnvelopeStatus = "blocked"
)

// Envelope wraps an approved email draft for execution.
//
// CRITICAL: PolicySnapshotHash and ViewSnapshotHash are REQUIRED.
// CRITICAL: Execution is blocked if either is missing or mismatched.
type Envelope struct {
	// EnvelopeID is deterministic from inputs.
	EnvelopeID string

	// DraftID links to the approved draft.
	DraftID draft.DraftID

	// CircleID identifies the owning circle.
	CircleID identity.EntityID

	// IntersectionID is optional for shared contexts.
	IntersectionID identity.EntityID

	// Provider identifies the email provider.
	Provider string

	// AccountID identifies the email account.
	AccountID string

	// Reply fields
	ThreadID           string
	InReplyToMessageID string
	Subject            string
	Body               string

	// Snapshot bindings (REQUIRED)
	PolicySnapshotHash string
	ViewSnapshotHash   string
	ViewSnapshotAt     time.Time

	// Idempotency
	IdempotencyKey string
	TraceID        string

	// Lifecycle
	CreatedAt  time.Time
	ApprovedAt time.Time
	Status     EnvelopeStatus

	// Execution result (populated after execution)
	ExecutedAt      *time.Time
	ExecutionResult *ExecutionResult
}

// ExecutionResult contains the result of email execution.
type ExecutionResult struct {
	// Success indicates the execution succeeded.
	Success bool

	// MessageID is the ID of the sent message.
	MessageID string

	// ProviderResponseID is the provider's response identifier.
	ProviderResponseID string

	// Error contains error details if Success=false.
	Error string

	// BlockedReason explains why execution was blocked.
	BlockedReason string
}

// Validate validates the envelope for execution.
func (e *Envelope) Validate() error {
	if e.EnvelopeID == "" {
		return fmt.Errorf("missing envelope_id")
	}
	if e.DraftID == "" {
		return fmt.Errorf("missing draft_id")
	}
	if e.CircleID == "" {
		return fmt.Errorf("missing circle_id")
	}
	if e.Provider == "" {
		return fmt.Errorf("missing provider")
	}
	if e.ThreadID == "" {
		return fmt.Errorf("missing thread_id: reply-only execution")
	}
	if e.InReplyToMessageID == "" {
		return fmt.Errorf("missing in_reply_to_message_id")
	}
	if e.Body == "" {
		return fmt.Errorf("missing body")
	}
	if e.PolicySnapshotHash == "" {
		return fmt.Errorf("missing policy_snapshot_hash: HARD BLOCK")
	}
	if e.ViewSnapshotHash == "" {
		return fmt.Errorf("missing view_snapshot_hash: HARD BLOCK")
	}
	if e.IdempotencyKey == "" {
		return fmt.Errorf("missing idempotency_key")
	}
	return nil
}

// CanonicalString returns a deterministic string for hashing.
func (e *Envelope) CanonicalString() string {
	return fmt.Sprintf("email-envelope|draft:%s|circle:%s|provider:%s|thread:%s|reply_to:%s|subject:%s|body_hash:%s|policy:%s|view:%s|view_at:%s|trace:%s",
		e.DraftID,
		e.CircleID,
		e.Provider,
		e.ThreadID,
		e.InReplyToMessageID,
		e.Subject,
		hashBody(e.Body),
		e.PolicySnapshotHash,
		e.ViewSnapshotHash,
		e.ViewSnapshotAt.UTC().Format(time.RFC3339),
		e.TraceID,
	)
}

// hashBody returns a truncated hash of the body for canonical string.
func hashBody(body string) string {
	hash := sha256.Sum256([]byte(body))
	return hex.EncodeToString(hash[:8])
}

// ComputeEnvelopeID generates a deterministic envelope ID.
func ComputeEnvelopeID(draftID draft.DraftID, circleID identity.EntityID, policyHash, viewHash, traceID string, createdAt time.Time) string {
	canonical := fmt.Sprintf("envelope-id|%s|%s|%s|%s|%s|%s",
		draftID,
		circleID,
		policyHash,
		viewHash,
		traceID,
		createdAt.UTC().Format(time.RFC3339Nano),
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:8])
}

// ComputeIdempotencyKey generates a deterministic idempotency key.
func ComputeIdempotencyKey(envelopeID string) string {
	canonical := fmt.Sprintf("idem|%s", envelopeID)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// NewEnvelopeFromDraft creates an envelope from an approved draft.
func NewEnvelopeFromDraft(
	d draft.Draft,
	policyHash string,
	viewHash string,
	viewSnapshotAt time.Time,
	traceID string,
	now time.Time,
) (*Envelope, error) {
	// Extract email content
	emailContent, ok := d.EmailContent()
	if !ok {
		return nil, fmt.Errorf("draft is not an email draft")
	}

	// Validate required fields
	if emailContent.ThreadID == "" {
		return nil, fmt.Errorf("email draft missing ThreadID")
	}
	if emailContent.InReplyToMessageID == "" {
		return nil, fmt.Errorf("email draft missing InReplyToMessageID")
	}
	if emailContent.Body == "" {
		return nil, fmt.Errorf("email draft missing Body")
	}

	// Compute envelope ID
	envelopeID := ComputeEnvelopeID(d.DraftID, d.CircleID, policyHash, viewHash, traceID, now)

	return &Envelope{
		EnvelopeID:         envelopeID,
		DraftID:            d.DraftID,
		CircleID:           d.CircleID,
		IntersectionID:     d.IntersectionID,
		Provider:           emailContent.ProviderHint,
		AccountID:          "", // Filled from context
		ThreadID:           emailContent.ThreadID,
		InReplyToMessageID: emailContent.InReplyToMessageID,
		Subject:            emailContent.Subject,
		Body:               emailContent.Body,
		PolicySnapshotHash: policyHash,
		ViewSnapshotHash:   viewHash,
		ViewSnapshotAt:     viewSnapshotAt,
		IdempotencyKey:     ComputeIdempotencyKey(envelopeID),
		TraceID:            traceID,
		CreatedAt:          now,
		ApprovedAt:         d.StatusChangedAt,
		Status:             EnvelopeStatusPending,
	}, nil
}

// IsTerminal returns true if the envelope is in a terminal state.
func (e *Envelope) IsTerminal() bool {
	return e.Status == EnvelopeStatusExecuted ||
		e.Status == EnvelopeStatusFailed ||
		e.Status == EnvelopeStatusBlocked
}
