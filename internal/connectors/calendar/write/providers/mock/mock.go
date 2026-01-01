// Package mock provides a mock calendar write connector for testing.
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/calendar/write"
)

// Writer is a mock calendar write connector.
type Writer struct {
	mu sync.RWMutex

	// responses stores idempotent responses keyed by IdempotencyKey.
	responses map[string]write.RespondReceipt

	// callCount tracks calls per EventID for testing.
	callCount map[string]int

	// failNext causes the next call to fail.
	failNext bool

	// failError is the error to return on failure.
	failError string

	// sandbox indicates this is a test provider.
	sandbox bool

	// clock for deterministic timestamps.
	clock func() time.Time
}

// Option configures the mock writer.
type Option func(*Writer)

// WithClock sets the clock function.
func WithClock(clock func() time.Time) Option {
	return func(w *Writer) {
		w.clock = clock
	}
}

// NewWriter creates a new mock calendar write connector.
func NewWriter(opts ...Option) *Writer {
	w := &Writer{
		responses: make(map[string]write.RespondReceipt),
		callCount: make(map[string]int),
		sandbox:   true,
		clock:     time.Now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// RespondToEvent implements write.Writer.
func (w *Writer) RespondToEvent(ctx context.Context, input write.RespondInput) (write.RespondReceipt, error) {
	if err := write.ValidateRespondInput(input); err != nil {
		return write.RespondReceipt{}, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Note: Latency simulation removed to comply with v9.7 guardrail.
	// Mocks must not use timers/tickers. Tests needing latency
	// should use context deadlines from the caller instead.
	if ctx.Err() != nil {
		return write.RespondReceipt{}, ctx.Err()
	}

	// Check idempotency - return prior result if exists
	if prior, exists := w.responses[input.IdempotencyKey]; exists {
		return prior, nil
	}

	// Track call count
	w.callCount[input.EventID]++

	// Check if we should fail
	if w.failNext {
		w.failNext = false
		receipt := write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          w.failError,
			IdempotencyKey: input.IdempotencyKey,
		}
		w.responses[input.IdempotencyKey] = receipt
		return receipt, nil
	}

	// Generate deterministic response ID
	responseID := w.generateResponseID(input)

	// Build success receipt
	receipt := write.RespondReceipt{
		Success:            true,
		EventID:            input.EventID,
		UpdatedAt:          w.clock(),
		ETag:               fmt.Sprintf("etag-%s", responseID[:8]),
		ProviderResponseID: responseID,
		IdempotencyKey:     input.IdempotencyKey,
	}

	// Store for idempotency
	w.responses[input.IdempotencyKey] = receipt

	return receipt, nil
}

// ProviderID implements write.Writer.
func (w *Writer) ProviderID() string {
	return "mock"
}

// IsSandbox implements write.Writer.
func (w *Writer) IsSandbox() bool {
	return w.sandbox
}

// SetFailNext causes the next call to fail with the given error.
func (w *Writer) SetFailNext(errMsg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failNext = true
	w.failError = errMsg
}

// GetCallCount returns the number of calls for an event.
func (w *Writer) GetCallCount(eventID string) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.callCount[eventID]
}

// Reset clears all state.
func (w *Writer) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.responses = make(map[string]write.RespondReceipt)
	w.callCount = make(map[string]int)
	w.failNext = false
	w.failError = ""
}

// generateResponseID creates a deterministic response ID.
func (w *Writer) generateResponseID(input write.RespondInput) string {
	canonical := fmt.Sprintf("mock|%s|%s|%s|%s",
		input.CalendarID,
		input.EventID,
		input.ResponseStatus,
		input.IdempotencyKey,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// Verify interface compliance.
var _ write.Writer = (*Writer)(nil)
