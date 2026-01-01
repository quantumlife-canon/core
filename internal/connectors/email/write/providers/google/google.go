// Package google provides a Google Gmail write connector.
//
// CRITICAL: Uses stdlib net/http only (no google client libs).
// CRITICAL: Real external write - must be used with full audit.
// CRITICAL: No auto-retries on failure.
//
// Reference: Phase 7 Email Execution Boundary
package google

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"quantumlife/internal/connectors/email/write"
)

const (
	// gmailAPIBase is the base URL for Gmail API.
	gmailAPIBase = "https://gmail.googleapis.com/gmail/v1"

	// sendEndpoint is the endpoint for sending messages.
	sendEndpoint = "/users/me/messages/send"
)

// TokenBroker provides OAuth access tokens.
type TokenBroker interface {
	// GetAccessToken returns a valid access token for the given account.
	GetAccessToken(ctx context.Context, accountID string) (string, error)
}

// Writer is a Gmail write connector.
type Writer struct {
	// tokenBroker provides access tokens.
	tokenBroker TokenBroker

	// httpClient is the HTTP client to use.
	httpClient *http.Client

	// clock provides time.
	clock func() time.Time

	// baseURL allows overriding for testing.
	baseURL string
}

// Option configures the writer.
type Option func(*Writer)

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(w *Writer) {
		w.httpClient = client
	}
}

// WithClock sets the clock function.
func WithClock(clock func() time.Time) Option {
	return func(w *Writer) {
		w.clock = clock
	}
}

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) Option {
	return func(w *Writer) {
		w.baseURL = url
	}
}

// NewWriter creates a new Gmail writer.
func NewWriter(tokenBroker TokenBroker, opts ...Option) *Writer {
	w := &Writer{
		tokenBroker: tokenBroker,
		httpClient:  http.DefaultClient,
		clock:       time.Now,
		baseURL:     gmailAPIBase,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// SendReply sends an email reply via Gmail API.
//
// CRITICAL: This performs a REAL external write.
// CRITICAL: No auto-retries on failure.
func (w *Writer) SendReply(ctx context.Context, req write.SendReplyRequest) (write.SendReplyReceipt, error) {
	// Validate request
	if err := write.ValidateSendReplyRequest(req); err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          err.Error(),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	// Get access token
	token, err := w.tokenBroker.GetAccessToken(ctx, req.AccountID)
	if err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("failed to get access token: %v", err),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	// Build the raw RFC 2822 message
	rawMessage := w.buildRawMessage(req)

	// Encode as base64url
	encodedMessage := base64.URLEncoding.EncodeToString([]byte(rawMessage))

	// Build request body
	requestBody := sendRequest{
		Raw:      encodedMessage,
		ThreadID: req.ThreadID,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("failed to marshal request: %v", err),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	// Create HTTP request
	url := w.baseURL + sendEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("failed to create request: %v", err),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute the request
	// CRITICAL: This is the REAL external write. No retries.
	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("HTTP request failed: %v", err),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("failed to read response: %v", err),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("Gmail API error (status %d): %s", resp.StatusCode, string(respBody)),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	// Parse response
	var sendResp sendResponse
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		return write.SendReplyReceipt{
			Success:        false,
			Error:          fmt.Sprintf("failed to parse response: %v", err),
			IdempotencyKey: req.IdempotencyKey,
		}, nil
	}

	return write.SendReplyReceipt{
		Success:            true,
		MessageID:          sendResp.ID,
		ThreadID:           sendResp.ThreadID,
		SentAt:             w.clock(),
		ProviderResponseID: sendResp.ID,
		IdempotencyKey:     req.IdempotencyKey,
	}, nil
}

// buildRawMessage creates an RFC 2822 message.
func (w *Writer) buildRawMessage(req write.SendReplyRequest) string {
	var sb strings.Builder

	// Headers for threading
	sb.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", req.InReplyToMessageID))
	sb.WriteString(fmt.Sprintf("References: %s\r\n", req.InReplyToMessageID))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(req.Body)

	return sb.String()
}

// ProviderID returns the provider identifier.
func (w *Writer) ProviderID() string {
	return "google"
}

// IsSandbox returns false (this is a real provider).
func (w *Writer) IsSandbox() bool {
	return false
}

// sendRequest is the Gmail API send request body.
type sendRequest struct {
	Raw      string `json:"raw"`
	ThreadID string `json:"threadId"`
}

// sendResponse is the Gmail API send response.
type sendResponse struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

// Ensure Writer implements write.Writer.
var _ write.Writer = (*Writer)(nil)
