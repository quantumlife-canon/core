// Package microsoft provides a Microsoft Graph Calendar API adapter.
// This file implements write operations for v6 Execute mode.
//
// CRITICAL: All write operations require:
// - Mode == Execute
// - ApprovedByHuman == true
// - calendar:write scope granted
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package microsoft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/pkg/primitives"
)

// API endpoints for write operations.
const (
	// CalendarEventsURL is the Microsoft Graph events endpoint.
	// See: https://learn.microsoft.com/en-us/graph/api/calendar-post-events
	CalendarEventsURL = "https://graph.microsoft.com/v1.0/me/events"

	// CalendarEventURL is the Microsoft Graph single event endpoint.
	// See: https://learn.microsoft.com/en-us/graph/api/event-delete
	CalendarEventURL = "https://graph.microsoft.com/v1.0/me/events/%s"
)

// WriteAdapter implements calendar.WriteConnector for Microsoft Graph.
// This adapter wraps the read-only Adapter and adds write operations.
//
// CRITICAL: Write operations perform REAL external writes to Microsoft Calendar.
type WriteAdapter struct {
	*Adapter // Embed the read-only adapter
}

// NewWriteAdapter creates a new Microsoft Graph write adapter.
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

// CreateEvent creates a new calendar event on Microsoft Calendar.
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
	token, err := w.broker.MintAccessToken(ctx, env, auth.ProviderMicrosoft, []string{"calendar:write"})
	if err != nil {
		return nil, fmt.Errorf("failed to mint access token: %w", err)
	}

	// Build event request body
	eventBody := graphEventCreateRequest{
		Subject: req.Title,
		Body: graphBody{
			ContentType: "text",
			Content:     req.Description,
		},
		Start: graphDateTime{
			DateTime: req.StartTime.Format("2006-01-02T15:04:05"),
			TimeZone: "UTC",
		},
		End: graphDateTime{
			DateTime: req.EndTime.Format("2006-01-02T15:04:05"),
			TimeZone: "UTC",
		},
	}

	// Add location if provided
	if req.Location != "" {
		eventBody.Location = &graphLocation{
			DisplayName: req.Location,
		}
	}

	// Add attendees if provided
	for _, email := range req.Attendees {
		eventBody.Attendees = append(eventBody.Attendees, graphAttendeeWrite{
			EmailAddress: graphEmailAddress{
				Address: email,
			},
			Type: "required",
		})
	}

	// Serialize request body
	bodyBytes, err := json.Marshal(eventBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", CalendarEventsURL, bytes.NewReader(bodyBytes))
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
	var response graphEventResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build receipt
	receipt := &calendar.CreateEventReceipt{
		Provider:        calendar.SourceMicrosoft,
		CalendarID:      "primary", // Microsoft doesn't use calendar IDs in the same way
		ExternalEventID: response.ID,
		Status:          "created",
		CreatedAt:       w.clockFunc(),
		Link:            response.WebLink,
	}

	return receipt, nil
}

// DeleteEvent deletes a calendar event from Microsoft Calendar.
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
	token, err := w.broker.MintAccessToken(ctx, env, auth.ProviderMicrosoft, []string{"calendar:write"})
	if err != nil {
		return nil, fmt.Errorf("failed to mint access token: %w", err)
	}

	// Build request URL
	apiURL := fmt.Sprintf(CalendarEventURL, url.PathEscape(req.ExternalEventID))

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
	status := "deleted"
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		status = "deleted"
	case http.StatusNotFound:
		status = "not_found"
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Build receipt
	receipt := &calendar.DeleteEventReceipt{
		Provider:        calendar.SourceMicrosoft,
		ExternalEventID: req.ExternalEventID,
		Status:          status,
		DeletedAt:       w.clockFunc(),
	}

	return receipt, nil
}

// Microsoft Graph API request types for write operations.
type graphEventCreateRequest struct {
	Subject   string               `json:"subject"`
	Body      graphBody            `json:"body"`
	Start     graphDateTime        `json:"start"`
	End       graphDateTime        `json:"end"`
	Location  *graphLocation       `json:"location,omitempty"`
	Attendees []graphAttendeeWrite `json:"attendees,omitempty"`
}

type graphBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// graphAttendeeWrite is used for creating events (includes Type field).
type graphAttendeeWrite struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
	Type         string            `json:"type"` // "required" or "optional"
}

// graphEventResponse is the response from creating an event.
type graphEventResponse struct {
	ID      string `json:"id"`
	WebLink string `json:"webLink"`
	Subject string `json:"subject"`
}

// Verify interface compliance at compile time.
var _ calendar.WriteConnector = (*WriteAdapter)(nil)
