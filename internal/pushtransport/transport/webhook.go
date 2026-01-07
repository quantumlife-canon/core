// Package transport provides push transport implementations.
//
// This file implements WebhookTransport for posting to an endpoint.
//
// CRITICAL INVARIANTS:
//   - stdlib net/http only. No external HTTP clients.
//   - No goroutines. Synchronous delivery only.
//   - No retries. Single attempt.
//   - No secrets in logs. TokenHash only.
//   - Abstract payload only. No identifiers in body.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package transport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"quantumlife/pkg/domain/pushtransport"
)

// WebhookTransport posts push notifications to a configured endpoint.
// Uses stdlib net/http only.
type WebhookTransport struct {
	// defaultEndpoint is used when request.Endpoint is empty.
	defaultEndpoint string

	// client is the HTTP client (stdlib only).
	client *http.Client
}

// NewWebhookTransport creates a new webhook transport.
func NewWebhookTransport(defaultEndpoint string) *WebhookTransport {
	return &WebhookTransport{
		defaultEndpoint: defaultEndpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ProviderKind returns the provider kind.
func (t *WebhookTransport) ProviderKind() pushtransport.PushProviderKind {
	return pushtransport.ProviderWebhook
}

// webhookPayload is the JSON payload sent to the webhook.
type webhookPayload struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	StatusHash string `json:"status_hash"`
}

// Send posts a push notification to the webhook endpoint.
// CRITICAL: Uses stdlib net/http only. No goroutines. No retries.
func (t *WebhookTransport) Send(ctx context.Context, req *pushtransport.TransportRequest) (*pushtransport.TransportResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	// Determine endpoint
	endpoint := req.Endpoint
	if endpoint == "" {
		endpoint = t.defaultEndpoint
	}
	if endpoint == "" {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureNotConfigured,
			ResponseHash: "",
		}, fmt.Errorf("no endpoint configured")
	}

	// Build payload
	// CRITICAL: Only constant title/body. No identifiers.
	payload := webhookPayload{
		Title:      req.Payload.Title,
		Body:       req.Payload.Body,
		StatusHash: req.Payload.StatusHash,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, fmt.Errorf("marshal payload: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	// CRITICAL: No retries. Single attempt only.
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response (limited to prevent memory exhaustion)
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// Compute response hash (for audit, never log actual response)
	responseInput := fmt.Sprintf("WEBHOOK_RESPONSE|v1|%d|%s",
		resp.StatusCode,
		string(respBody),
	)
	h := sha256.Sum256([]byte(responseInput))
	responseHash := hex.EncodeToString(h[:16])

	// Check status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &pushtransport.TransportResult{
			Success:      true,
			ErrorBucket:  pushtransport.FailureNone,
			ResponseHash: responseHash,
		}, nil
	}

	return &pushtransport.TransportResult{
		Success:      false,
		ErrorBucket:  pushtransport.FailureTransportError,
		ResponseHash: responseHash,
	}, nil
}
