// Package google provides a Google Calendar API adapter.
// This file implements write operations for v6 Execute mode.
//
// CRITICAL: All write operations require:
// - Mode == Execute
// - ApprovedByHuman == true
// - calendar:write scope granted
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/pkg/primitives"
)

// API endpoints for write operations.
const (
	// CalendarEventsInsertURL is the Google Calendar events.insert endpoint.
	// See: https://developers.google.com/calendar/api/v3/reference/events/insert
	CalendarEventsInsertURL = "https://www.googleapis.com/calendar/v3/calendars/%s/events"

	// CalendarEventsDeleteURL is the Google Calendar events.delete endpoint.
	// See: https://developers.google.com/calendar/api/v3/reference/events/delete
	CalendarEventsDeleteURL = "https://www.googleapis.com/calendar/v3/calendars/%s/events/%s"
)

// WriteAdapter implements calendar.WriteConnector for Google Calendar.
// This adapter wraps the read-only Adapter and adds write operations.
//
// CRITICAL: Write operations perform REAL external writes to Google Calendar.
type WriteAdapter struct {
	*Adapter // Embed the read-only adapter
}

// NewWriteAdapter creates a new Google Calendar write adapter.
// The adapter must be configured with valid OAuth credentials.
func NewWriteAdapter(broker auth.TokenBroker, isConfigured bool, opts ...AdapterOption) *WriteAdapter {
	return &WriteAdapter{
		Adapter: NewAdapter(broker, isConfigured, opts...),
	}
}

// Capabilities returns the connector's capabilities including write.
func (w *WriteAdapter) Capabilities() []string {
	return []string{"list_events", "find_free_slots", "propose_event", "create_event", "delete_event"}
}

// RequiredScopes returns scopes required for write operations.
func (w *WriteAdapter) RequiredScopes() []string {
	return []string{"calendar:read", "calendar:write"}
}

// SupportsWrite returns true since this adapter supports write operations.
func (w *WriteAdapter) SupportsWrite() bool {
	return true
}

// CreateEvent creates a new calendar event on Google Calendar.
//
// CRITICAL: This performs an EXTERNAL WRITE. Requirements enforced:
// - env.Mode MUST be Execute
// - env.ApprovedByHuman MUST be true
// - calendar:write scope MUST be in env.ScopesUsed
//
// Returns a receipt with the external event ID for rollback capability.
func (w *WriteAdapter) CreateEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.CreateEventRequest) (*calendar.CreateEventReceipt, error) {
	// CRITICAL: Validate envelope for write operation
	if err := env.ValidateForWrite(); err != nil {
		return nil, fmt.Errorf("write validation failed: %w", err)
	}

	// Check if configured
	if !w.isConfigured {
		return nil, auth.ErrProviderNotConfigured
	}

	// Mint access token with write scope
	token, err := w.broker.MintAccessToken(ctx, env, auth.ProviderGoogle, []string{"calendar:write"})
	if err != nil {
		return nil, fmt.Errorf("failed to mint access token: %w", err)
	}

	// Build event request body
	calendarID := req.CalendarID
	if calendarID == "" {
		calendarID = DefaultCalendarID
	}

	eventBody := googleEventInsertRequest{
		Summary:     req.Title,
		Description: req.Description,
		Start: googleDateTime{
			DateTime: req.StartTime.Format(time.RFC3339),
		},
		End: googleDateTime{
			DateTime: req.EndTime.Format(time.RFC3339),
		},
		Location: req.Location,
	}

	// Add attendees if provided
	for _, email := range req.Attendees {
		eventBody.Attendees = append(eventBody.Attendees, googleAttendee{Email: email})
	}

	// Serialize request body
	bodyBytes, err := json.Marshal(eventBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Build request URL
	apiURL := fmt.Sprintf(CalendarEventsInsertURL, url.PathEscape(calendarID))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token.Token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute request - THIS IS THE EXTERNAL WRITE
	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response googleEventResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build receipt
	receipt := &calendar.CreateEventReceipt{
		Provider:        calendar.SourceGoogle,
		CalendarID:      calendarID,
		ExternalEventID: response.ID,
		Status:          response.Status,
		CreatedAt:       w.clockFunc(),
		Link:            response.HTMLLink,
	}

	return receipt, nil
}

// DeleteEvent deletes a calendar event from Google Calendar.
//
// CRITICAL: This performs an EXTERNAL WRITE (deletion). Requirements:
// - env.Mode MUST be Execute
// - env.ApprovedByHuman MUST be true
// - calendar:write scope MUST be in env.ScopesUsed
//
// This is used for rollback after failed settlement.
func (w *WriteAdapter) DeleteEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.DeleteEventRequest) (*calendar.DeleteEventReceipt, error) {
	// CRITICAL: Validate envelope for write operation
	if err := env.ValidateForWrite(); err != nil {
		return nil, fmt.Errorf("write validation failed: %w", err)
	}

	// Check if configured
	if !w.isConfigured {
		return nil, auth.ErrProviderNotConfigured
	}

	// Mint access token with write scope
	token, err := w.broker.MintAccessToken(ctx, env, auth.ProviderGoogle, []string{"calendar:write"})
	if err != nil {
		return nil, fmt.Errorf("failed to mint access token: %w", err)
	}

	// Build request URL
	calendarID := req.CalendarID
	if calendarID == "" {
		calendarID = DefaultCalendarID
	}

	apiURL := fmt.Sprintf(CalendarEventsDeleteURL, url.PathEscape(calendarID), url.PathEscape(req.ExternalEventID))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token.Token)

	// Execute request - THIS IS THE EXTERNAL WRITE (DELETE)
	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	// 204 No Content is success for delete
	// 404 Not Found means already deleted (idempotent)
	// 410 Gone means resource was deleted
	status := "deleted"
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		status = "deleted"
	case http.StatusNotFound:
		status = "not_found"
	case http.StatusGone:
		status = "already_deleted"
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Build receipt
	receipt := &calendar.DeleteEventReceipt{
		Provider:        calendar.SourceGoogle,
		ExternalEventID: req.ExternalEventID,
		Status:          status,
		DeletedAt:       w.clockFunc(),
	}

	return receipt, nil
}

// Google API request types for write operations.
type googleEventInsertRequest struct {
	Summary     string           `json:"summary"`
	Description string           `json:"description,omitempty"`
	Start       googleDateTime   `json:"start"`
	End         googleDateTime   `json:"end"`
	Location    string           `json:"location,omitempty"`
	Attendees   []googleAttendee `json:"attendees,omitempty"`
}

// googleEventResponse is the response from events.insert.
type googleEventResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	HTMLLink string `json:"htmlLink"`
	Summary  string `json:"summary"`
}

// Verify interface compliance at compile time.
var _ calendar.WriteConnector = (*WriteAdapter)(nil)
