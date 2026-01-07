// Package transport provides push transport implementations.
//
// This package defines the Transport interface and provides implementations
// for different push delivery mechanisms: stub, webhook, and (optionally) APNs.
//
// CRITICAL INVARIANTS:
//   - All transports use stdlib net/http only.
//   - No goroutines. Synchronous delivery only.
//   - Payload body is constant literal (no customization).
//   - No secrets in logs. TokenHash only for audit.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package transport

import (
	"context"

	"quantumlife/pkg/domain/pushtransport"
)

// Transport is the interface for push delivery mechanisms.
type Transport interface {
	// ProviderKind returns the provider kind this transport handles.
	ProviderKind() pushtransport.PushProviderKind

	// Send delivers a push notification.
	// Returns the result and any error.
	Send(ctx context.Context, req *pushtransport.TransportRequest) (*pushtransport.TransportResult, error)
}

// Registry holds available transports.
type Registry struct {
	transports map[pushtransport.PushProviderKind]Transport
}

// NewRegistry creates a new transport registry.
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[pushtransport.PushProviderKind]Transport),
	}
}

// Register adds a transport to the registry.
func (r *Registry) Register(t Transport) {
	r.transports[t.ProviderKind()] = t
}

// Get retrieves a transport by provider kind.
func (r *Registry) Get(kind pushtransport.PushProviderKind) (Transport, bool) {
	t, ok := r.transports[kind]
	return t, ok
}

// DefaultRegistry creates a registry with stub and webhook transports.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(NewStubTransport())
	reg.Register(NewWebhookTransport(""))
	return reg
}
