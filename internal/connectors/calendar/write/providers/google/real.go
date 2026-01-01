// Package google provides a real Google Calendar write connector.
//
// CRITICAL: This performs REAL external writes to Google Calendar.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent - same IdempotencyKey returns same result.
//
// Reference: docs/ADR/ADR-0022-phase5-calendar-execution-boundary.md
package google

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/connectors/calendar/write"
)

const (
	// googleCalendarAPIBase is the base URL for Google Calendar API.
	googleCalendarAPIBase = "https://www.googleapis.com/calendar/v3"

	// defaultTimeout is the default HTTP timeout.
	defaultTimeout = 30 * time.Second
)

// TokenProvider provides OAuth2 access tokens.
type TokenProvider interface {
	// GetAccessToken returns a valid access token.
	// CRITICAL: Implementation must handle token refresh.
	GetAccessToken(ctx context.Context) (string, error)
}

// Writer is a real Google Calendar write connector.
type Writer struct {
	mu sync.RWMutex

	// tokenProvider provides OAuth2 tokens.
	tokenProvider TokenProvider

	// httpClient is the HTTP client (stdlib only).
	httpClient *http.Client

	// baseURL allows overriding for tests.
	baseURL string

	// idempotencyStore stores completed operations.
	// In production, this would be a persistent store.
	idempotencyStore map[string]write.RespondReceipt

	// sandbox indicates this is a sandbox provider (for testing).
	sandbox bool

	// clock for deterministic timestamps.
	clock func() time.Time
}

// Option configures the Google writer.
type Option func(*Writer)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(w *Writer) {
		w.httpClient = client
	}
}

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(baseURL string) Option {
	return func(w *Writer) {
		w.baseURL = baseURL
	}
}

// WithSandbox marks this as a sandbox provider.
func WithSandbox(sandbox bool) Option {
	return func(w *Writer) {
		w.sandbox = sandbox
	}
}

// WithClock sets the clock function.
func WithClock(clock func() time.Time) Option {
	return func(w *Writer) {
		w.clock = clock
	}
}

// NewWriter creates a new Google Calendar write connector.
func NewWriter(tokenProvider TokenProvider, opts ...Option) *Writer {
	w := &Writer{
		tokenProvider:    tokenProvider,
		baseURL:          googleCalendarAPIBase,
		idempotencyStore: make(map[string]write.RespondReceipt),
		sandbox:          false,
		clock:            time.Now,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// RespondToEvent implements write.Writer.
//
// Google Calendar API for responding to events:
// POST https://www.googleapis.com/calendar/v3/calendars/{calendarId}/events/{eventId}/instances
// OR update attendee responseStatus via PATCH
//
// For simplicity and safety, we use the dedicated response endpoint if available,
// otherwise PATCH the event's attendees array.
func (w *Writer) RespondToEvent(ctx context.Context, input write.RespondInput) (write.RespondReceipt, error) {
	if err := write.ValidateRespondInput(input); err != nil {
		return write.RespondReceipt{}, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Check idempotency - return prior result if exists
	if prior, exists := w.idempotencyStore[input.IdempotencyKey]; exists {
		return prior, nil
	}

	// Build the request
	var receipt write.RespondReceipt

	if input.ProposeNewTime {
		// For propose_new_time: add a comment, don't modify the event
		receipt = w.handleProposeNewTime(ctx, input)
	} else {
		// For accept/decline/tentative: update response status
		receipt = w.handleResponseStatus(ctx, input)
	}

	// Store for idempotency
	w.idempotencyStore[input.IdempotencyKey] = receipt

	return receipt, nil
}

// handleResponseStatus handles accept/decline/tentative responses.
func (w *Writer) handleResponseStatus(ctx context.Context, input write.RespondInput) write.RespondReceipt {
	// Get current event to find our attendee entry
	event, err := w.getEvent(ctx, input.CalendarID, input.EventID)
	if err != nil {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          fmt.Sprintf("failed to get event: %v", err),
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	// Find our email in attendees and update response status
	email, err := w.getCurrentUserEmail(ctx)
	if err != nil {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          fmt.Sprintf("failed to get user email: %v", err),
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	// Update attendee response status
	updated := false
	for i, att := range event.Attendees {
		if strings.EqualFold(att.Email, email) {
			event.Attendees[i].ResponseStatus = string(input.ResponseStatus)
			if input.Message != "" {
				event.Attendees[i].Comment = input.Message
			}
			updated = true
			break
		}
	}

	if !updated {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          "user not found in event attendees",
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	// PATCH the event
	patchedEvent, err := w.patchEvent(ctx, input.CalendarID, input.EventID, event)
	if err != nil {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          fmt.Sprintf("failed to patch event: %v", err),
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	// Generate deterministic response ID
	responseID := w.generateResponseID(input)

	return write.RespondReceipt{
		Success:            true,
		EventID:            input.EventID,
		UpdatedAt:          w.clock(),
		ETag:               patchedEvent.ETag,
		ProviderResponseID: responseID,
		IdempotencyKey:     input.IdempotencyKey,
	}
}

// handleProposeNewTime handles propose new time (adds comment, no event modification).
func (w *Writer) handleProposeNewTime(ctx context.Context, input write.RespondInput) write.RespondReceipt {
	// For propose_new_time: we do NOT modify the event times.
	// Instead, add a comment/note proposing new times.
	// This is safe and reversible.

	// Get current event
	event, err := w.getEvent(ctx, input.CalendarID, input.EventID)
	if err != nil {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          fmt.Sprintf("failed to get event: %v", err),
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	// Build proposal note
	proposalNote := fmt.Sprintf(
		"\n\n[Time Proposal from attendee]\nProposed: %s - %s\nMessage: %s",
		input.ProposedStart.Format(time.RFC3339),
		input.ProposedEnd.Format(time.RFC3339),
		input.Message,
	)

	// Append to description (safe, reversible)
	if event.Description == "" {
		event.Description = proposalNote
	} else {
		event.Description = event.Description + proposalNote
	}

	// Also set our response to tentative
	email, err := w.getCurrentUserEmail(ctx)
	if err != nil {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          fmt.Sprintf("failed to get user email: %v", err),
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	for i, att := range event.Attendees {
		if strings.EqualFold(att.Email, email) {
			event.Attendees[i].ResponseStatus = string(write.ResponseTentative)
			event.Attendees[i].Comment = "Proposing new time"
			break
		}
	}

	// PATCH the event
	patchedEvent, err := w.patchEvent(ctx, input.CalendarID, input.EventID, event)
	if err != nil {
		return write.RespondReceipt{
			Success:        false,
			EventID:        input.EventID,
			Error:          fmt.Sprintf("failed to patch event: %v", err),
			IdempotencyKey: input.IdempotencyKey,
		}
	}

	// Generate deterministic response ID
	responseID := w.generateResponseID(input)

	return write.RespondReceipt{
		Success:            true,
		EventID:            input.EventID,
		UpdatedAt:          w.clock(),
		ETag:               patchedEvent.ETag,
		ProviderResponseID: responseID,
		IdempotencyKey:     input.IdempotencyKey,
	}
}

// GoogleEvent represents a Google Calendar event (minimal fields).
type GoogleEvent struct {
	ID          string           `json:"id,omitempty"`
	ETag        string           `json:"etag,omitempty"`
	Summary     string           `json:"summary,omitempty"`
	Description string           `json:"description,omitempty"`
	Attendees   []GoogleAttendee `json:"attendees,omitempty"`
}

// GoogleAttendee represents an event attendee.
type GoogleAttendee struct {
	Email          string `json:"email,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
	Comment        string `json:"comment,omitempty"`
	Self           bool   `json:"self,omitempty"`
}

// getEvent retrieves an event from Google Calendar.
func (w *Writer) getEvent(ctx context.Context, calendarID, eventID string) (*GoogleEvent, error) {
	endpoint := fmt.Sprintf("%s/calendars/%s/events/%s",
		w.baseURL,
		url.PathEscape(calendarID),
		url.PathEscape(eventID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	token, err := w.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var event GoogleEvent
	if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &event, nil
}

// patchEvent updates an event in Google Calendar.
func (w *Writer) patchEvent(ctx context.Context, calendarID, eventID string, event *GoogleEvent) (*GoogleEvent, error) {
	endpoint := fmt.Sprintf("%s/calendars/%s/events/%s",
		w.baseURL,
		url.PathEscape(calendarID),
		url.PathEscape(eventID),
	)

	// Only send the fields we want to update
	patchData := map[string]interface{}{
		"attendees":   event.Attendees,
		"description": event.Description,
	}

	body, err := json.Marshal(patchData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	token, err := w.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var updatedEvent GoogleEvent
	if err := json.NewDecoder(resp.Body).Decode(&updatedEvent); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &updatedEvent, nil
}

// getCurrentUserEmail gets the authenticated user's email.
func (w *Writer) getCurrentUserEmail(ctx context.Context) (string, error) {
	endpoint := "https://www.googleapis.com/oauth2/v1/userinfo"
	if strings.HasPrefix(w.baseURL, "http://") {
		// Test mode - use same base
		endpoint = w.baseURL + "/userinfo"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	token, err := w.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return userInfo.Email, nil
}

// generateResponseID creates a deterministic response ID.
func (w *Writer) generateResponseID(input write.RespondInput) string {
	canonical := fmt.Sprintf("google|%s|%s|%s|%s",
		input.CalendarID,
		input.EventID,
		input.ResponseStatus,
		input.IdempotencyKey,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ProviderID implements write.Writer.
func (w *Writer) ProviderID() string {
	return "google"
}

// IsSandbox implements write.Writer.
func (w *Writer) IsSandbox() bool {
	return w.sandbox
}

// ClearIdempotencyCache clears the idempotency cache (for testing only).
func (w *Writer) ClearIdempotencyCache() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.idempotencyStore = make(map[string]write.RespondReceipt)
}

// Verify interface compliance.
var _ write.Writer = (*Writer)(nil)
