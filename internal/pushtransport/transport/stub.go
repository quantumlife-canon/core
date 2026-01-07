// Package transport provides push transport implementations.
//
// This file implements StubTransport for testing without network.
//
// CRITICAL INVARIANTS:
//   - Deterministic. Same input => same output.
//   - No network calls.
//   - No goroutines.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"quantumlife/pkg/domain/pushtransport"
)

// StubTransport is a deterministic transport for testing.
// It does NOT make network calls.
type StubTransport struct{}

// NewStubTransport creates a new stub transport.
func NewStubTransport() *StubTransport {
	return &StubTransport{}
}

// ProviderKind returns the provider kind.
func (t *StubTransport) ProviderKind() pushtransport.PushProviderKind {
	return pushtransport.ProviderStub
}

// Send simulates sending a push notification.
// Always succeeds with deterministic response hash.
func (t *StubTransport) Send(ctx context.Context, req *pushtransport.TransportRequest) (*pushtransport.TransportResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	// Check context
	select {
	case <-ctx.Done():
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, ctx.Err()
	default:
	}

	// Compute deterministic response hash
	responseInput := fmt.Sprintf("STUB_RESPONSE|v1|%s|%s|%s",
		req.AttemptID,
		req.Payload.Title,
		req.Payload.StatusHash,
	)
	h := sha256.Sum256([]byte(responseInput))
	responseHash := hex.EncodeToString(h[:16])

	return &pushtransport.TransportResult{
		Success:      true,
		ErrorBucket:  pushtransport.FailureNone,
		ResponseHash: responseHash,
	}, nil
}
