// Package write provides email write connector interfaces and implementations.
//
// CRITICAL: Email write is a REAL external action.
// CRITICAL: Must be "boringly safe" - explicit approval, full audit.
// CRITICAL: No auto-retries, no background execution.
// CRITICAL: Reply-only for now (no new outbound threads).
//
// Reference: Phase 7 Email Execution Boundary
package write

import (
	"context"
	"time"
)

// SendReplyRequest contains the input for sending an email reply.
type SendReplyRequest struct {
	// Provider identifies the email provider (google, outlook, etc.).
	Provider string

	// AccountID is the email account identifier.
	AccountID string

	// CircleID identifies the owning circle.
	CircleID string

	// ThreadID is the email thread to reply to.
	// REQUIRED: We only support replies, not new threads.
	ThreadID string

	// InReplyToMessageID is the message ID being replied to.
	// REQUIRED: This ensures proper threading.
	InReplyToMessageID string

	// Subject is the email subject line.
	Subject string

	// Body is the email body text.
	// REQUIRED: Cannot send empty replies.
	Body string

	// IdempotencyKey is used to prevent duplicate sends.
	// REQUIRED: Must be unique per logical send.
	IdempotencyKey string

	// TraceID links this operation to the execution trace.
	TraceID string
}

// SendReplyReceipt contains the result of a send reply operation.
type SendReplyReceipt struct {
	// Success indicates the operation succeeded.
	Success bool

	// MessageID is the ID of the sent message.
	MessageID string

	// ThreadID is the thread the message was added to.
	ThreadID string

	// SentAt is when the message was sent.
	SentAt time.Time

	// ProviderResponseID is the provider's response identifier.
	ProviderResponseID string

	// Error contains error details if Success=false.
	Error string

	// IdempotencyKey echoes back the key used.
	IdempotencyKey string
}

// Writer defines the interface for email write operations.
type Writer interface {
	// SendReply sends an email reply within an existing thread.
	//
	// CRITICAL: This performs a REAL external write.
	// CRITICAL: Must be idempotent - same IdempotencyKey returns same result.
	// CRITICAL: No auto-retries on failure.
	// CRITICAL: Reply-only - ThreadID and InReplyToMessageID are required.
	SendReply(ctx context.Context, req SendReplyRequest) (SendReplyReceipt, error)

	// ProviderID returns the provider identifier.
	ProviderID() string

	// IsSandbox returns true if this is a sandbox/test provider.
	IsSandbox() bool
}

// ValidateSendReplyRequest validates the input for SendReply.
func ValidateSendReplyRequest(req SendReplyRequest) error {
	if req.Provider == "" {
		return ErrMissingProvider
	}
	if req.ThreadID == "" {
		return ErrMissingThreadID
	}
	if req.InReplyToMessageID == "" {
		return ErrMissingInReplyToMessageID
	}
	if req.Body == "" {
		return ErrMissingBody
	}
	if req.IdempotencyKey == "" {
		return ErrMissingIdempotencyKey
	}
	return nil
}

// Validation errors.
var (
	ErrMissingProvider           = writeError("missing provider")
	ErrMissingThreadID           = writeError("missing thread_id: reply-only, must reference existing thread")
	ErrMissingInReplyToMessageID = writeError("missing in_reply_to_message_id: must reference message being replied to")
	ErrMissingBody               = writeError("missing body: cannot send empty reply")
	ErrMissingIdempotencyKey     = writeError("missing idempotency_key")
	ErrDuplicateSend             = writeError("duplicate send detected via idempotency key")
)

type writeError string

func (e writeError) Error() string { return string(e) }
