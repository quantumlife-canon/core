// Package mock provides a mock email write connector for testing.
//
// CRITICAL: Deterministic behavior for testing.
// CRITICAL: Same IdempotencyKey returns same receipt.
//
// Reference: Phase 7 Email Execution Boundary
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/email/write"
)

// Writer is a mock email writer for testing.
type Writer struct {
	mu sync.Mutex

	// sentMessages tracks sent messages by idempotency key.
	sentMessages map[string]write.SendReplyReceipt

	// clock provides deterministic time.
	clock func() time.Time

	// failNext causes the next send to fail.
	failNext bool

	// failNextError is the error to return on failure.
	failNextError string
}

// Option configures the mock writer.
type Option func(*Writer)

// WithClock sets the clock function.
func WithClock(clock func() time.Time) Option {
	return func(w *Writer) {
		w.clock = clock
	}
}

// NewWriter creates a new mock email writer.
func NewWriter(opts ...Option) *Writer {
	w := &Writer{
		sentMessages: make(map[string]write.SendReplyReceipt),
		clock:        time.Now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// SendReply sends a mock email reply.
//
// CRITICAL: Deterministic - same IdempotencyKey returns same receipt.
func (w *Writer) SendReply(ctx context.Context, req write.SendReplyRequest) (write.SendReplyReceipt, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Validate request
	if err := write.ValidateSendReplyRequest(req); err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          err.Error(),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	// Check idempotency - return prior receipt if exists
	if prior, exists := w.sentMessages[req.IdempotencyKey]; exists {
		return prior, nil
	}

	// Check if we should fail
	if w.failNext {
		w.failNext = false
		return write.SendReplyReceipt{
			Success:        false,
			Error:          w.failNextError,
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	now := w.clock()

	// Generate deterministic message ID from request
	messageID := w.generateMessageID(req)

	receipt := write.SendReplyReceipt{
		Success:            true,
		MessageID:          messageID,
		ThreadID:           req.ThreadID,
		SentAt:             now,
		ProviderResponseID: fmt.Sprintf("mock-response-%s", messageID[:8]),
		IdempotencyKey:     req.IdempotencyKey,
	}

	// Store for idempotency
	w.sentMessages[req.IdempotencyKey] = receipt

	return receipt, nil
}

// generateMessageID creates a deterministic message ID.
func (w *Writer) generateMessageID(req write.SendReplyRequest) string {
	canonical := fmt.Sprintf("mock-email|%s|%s|%s|%s|%s",
		req.ThreadID,
		req.InReplyToMessageID,
		req.Subject,
		req.Body,
		req.IdempotencyKey,
	)
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("mock-msg-%s", hex.EncodeToString(hash[:8]))
}

// ProviderID returns the provider identifier.
func (w *Writer) ProviderID() string {
	return "mock"
}

// IsSandbox returns true (mock is always sandbox).
func (w *Writer) IsSandbox() bool {
	return true
}

// SetFailNext causes the next send to fail with the given error.
func (w *Writer) SetFailNext(errorMsg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failNext = true
	w.failNextError = errorMsg
}

// GetSentCount returns the number of messages sent.
func (w *Writer) GetSentCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.sentMessages)
}

// GetSentMessages returns all sent messages.
func (w *Writer) GetSentMessages() []write.SendReplyReceipt {
	w.mu.Lock()
	defer w.mu.Unlock()

	result := make([]write.SendReplyReceipt, 0, len(w.sentMessages))
	for _, receipt := range w.sentMessages {
		result = append(result, receipt)
	}
	return result
}

// Reset clears all sent messages.
func (w *Writer) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sentMessages = make(map[string]write.SendReplyReceipt)
	w.failNext = false
}

// Ensure Writer implements write.Writer.
var _ write.Writer = (*Writer)(nil)
