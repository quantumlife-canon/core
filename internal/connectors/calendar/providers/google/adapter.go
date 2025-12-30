// Package google provides a Google Calendar API adapter.
// This adapter implements read-only operations for v5.
//
// CRITICAL: This adapter does NOT perform any write operations.
// All operations are read-only (GET requests only).
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/pkg/primitives"
)

// API endpoints.
const (
	// CalendarEventsListURL is the Google Calendar events.list endpoint.
	// See: https://developers.google.com/calendar/api/v3/reference/events/list
	CalendarEventsListURL = "https://www.googleapis.com/calendar/v3/calendars/%s/events"

	// DefaultCalendarID is the primary calendar.
	DefaultCalendarID = "primary"
)

// Adapter implements the calendar.EnvelopeConnector for Google Calendar.
type Adapter struct {
	mu           sync.RWMutex
	broker       auth.TokenBroker
	httpClient   *http.Client
	clockFunc    func() time.Time
	idCounter    int
	isConfigured bool
}

// AdapterOption configures an Adapter.
type AdapterOption func(*Adapter)

// WithHTTPClient sets a custom HTTP client (for testing).
func WithHTTPClient(client *http.Client) AdapterOption {
	return func(a *Adapter) {
		a.httpClient = client
	}
}

// WithClock sets a custom clock function (for testing).
func WithClock(clockFunc func() time.Time) AdapterOption {
	return func(a *Adapter) {
		a.clockFunc = clockFunc
	}
}

// NewAdapter creates a new Google Calendar adapter.
func NewAdapter(broker auth.TokenBroker, isConfigured bool, opts ...AdapterOption) *Adapter {
	a := &Adapter{
		broker:       broker,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		clockFunc:    time.Now,
		isConfigured: isConfigured,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// ID returns the connector identifier.
func (a *Adapter) ID() string {
	return "google-calendar"
}

// Capabilities returns the connector's capabilities.
func (a *Adapter) Capabilities() []string {
	return []string{"list_events", "find_free_slots", "propose_event"}
}

// RequiredScopes returns scopes required for this connector.
func (a *Adapter) RequiredScopes() []string {
	return []string{"calendar:read"}
}

// ProviderInfo returns information about the provider.
func (a *Adapter) ProviderInfo() calendar.ProviderInfo {
	return calendar.ProviderInfo{
		ID:           "google",
		Name:         "Google Calendar",
		Capabilities: a.Capabilities(),
		IsConfigured: a.isConfigured,
	}
}

// ListEvents returns events in the specified time range (legacy interface).
func (a *Adapter) ListEvents(ctx context.Context, req calendar.ListEventsRequest) ([]calendar.Event, error) {
	// This legacy method cannot be used with real providers since it lacks envelope
	return nil, fmt.Errorf("ListEvents requires ExecutionEnvelope; use ListEventsWithEnvelope")
}

// ListEventsWithEnvelope returns events with full traceability.
func (a *Adapter) ListEventsWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange) ([]calendar.Event, error) {
	// Validate envelope
	if err := env.ValidateForRead(); err != nil {
		return nil, fmt.Errorf("invalid envelope: %w", err)
	}

	// Check if configured
	if !a.isConfigured {
		return nil, auth.ErrProviderNotConfigured
	}

	// Mint access token
	token, err := a.broker.MintAccessToken(ctx, env, auth.ProviderGoogle, []string{"calendar:read"})
	if err != nil {
		return nil, fmt.Errorf("failed to mint access token: %w", err)
	}

	// Build request
	calendarID := DefaultCalendarID
	apiURL := fmt.Sprintf(CalendarEventsListURL, url.PathEscape(calendarID))

	params := url.Values{}
	params.Set("timeMin", r.Start.Format(time.RFC3339))
	params.Set("timeMax", r.End.Format(time.RFC3339))
	params.Set("singleEvents", "true")
	params.Set("orderBy", "startTime")
	params.Set("maxResults", "250")

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response googleEventsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to domain events
	events := make([]calendar.Event, 0, len(response.Items))
	for _, item := range response.Items {
		event, err := convertGoogleEvent(item)
		if err != nil {
			continue // Skip malformed events
		}
		events = append(events, event)
	}

	return events, nil
}

// FindFreeSlots finds free slots in the calendar.
func (a *Adapter) FindFreeSlots(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange, minDuration time.Duration) ([]calendar.FreeSlot, error) {
	// Get events
	events, err := a.ListEventsWithEnvelope(ctx, env, r)
	if err != nil {
		return nil, err
	}

	// Find free slots between events
	slots := findFreeSlotsFromEvents(events, r, minDuration)

	return slots, nil
}

// ProposeEvent creates a proposed event (legacy interface).
func (a *Adapter) ProposeEvent(ctx context.Context, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return nil, fmt.Errorf("ProposeEvent requires ExecutionEnvelope; use ProposeEventWithEnvelope")
}

// ProposeEventWithEnvelope creates a proposed event without writing.
func (a *Adapter) ProposeEventWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	a.mu.Lock()
	a.idCounter++
	proposalID := fmt.Sprintf("google-proposal-%d", a.idCounter)
	a.mu.Unlock()

	// Check for conflicts by listing events in the time range
	events, err := a.ListEventsWithEnvelope(ctx, env, calendar.EventRange{
		Start: req.StartTime,
		End:   req.EndTime,
	})
	if err != nil {
		// If we can't check conflicts, still return the proposal
		events = nil
	}

	message := "SIMULATED: Event would be created on Google Calendar (v5 read-only)"
	if len(events) > 0 {
		message = fmt.Sprintf("SIMULATED: Event would be created with %d potential conflicts", len(events))
	}

	return &calendar.ProposedEvent{
		ProposalID: proposalID,
		Event: calendar.Event{
			ID:          fmt.Sprintf("proposed-%s", proposalID),
			Title:       req.Title,
			Description: req.Description,
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Location:    req.Location,
			Attendees:   req.Attendees,
			CalendarID:  req.CalendarID,
		},
		Simulated:         true,
		Message:           message,
		ConflictingEvents: events,
	}, nil
}

// HealthCheck verifies the connector is operational.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	if !a.isConfigured {
		return auth.ErrProviderNotConfigured
	}
	return nil
}

// Google API response types.
type googleEventsResponse struct {
	Items []googleEvent `json:"items"`
}

type googleEvent struct {
	ID          string           `json:"id"`
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Location    string           `json:"location"`
	Start       googleDateTime   `json:"start"`
	End         googleDateTime   `json:"end"`
	Attendees   []googleAttendee `json:"attendees"`
}

type googleDateTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
	TimeZone string `json:"timeZone"`
}

type googleAttendee struct {
	Email string `json:"email"`
}

// convertGoogleEvent converts a Google event to a domain event.
func convertGoogleEvent(g googleEvent) (calendar.Event, error) {
	startTime, err := parseGoogleDateTime(g.Start)
	if err != nil {
		return calendar.Event{}, err
	}

	endTime, err := parseGoogleDateTime(g.End)
	if err != nil {
		return calendar.Event{}, err
	}

	attendees := make([]string, len(g.Attendees))
	for i, a := range g.Attendees {
		attendees[i] = a.Email
	}

	return calendar.Event{
		ID:          g.ID,
		Title:       g.Summary,
		Description: g.Description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    g.Location,
		Attendees:   attendees,
		CalendarID:  DefaultCalendarID,
	}, nil
}

// parseGoogleDateTime parses a Google datetime struct.
func parseGoogleDateTime(dt googleDateTime) (time.Time, error) {
	if dt.DateTime != "" {
		return time.Parse(time.RFC3339, dt.DateTime)
	}
	if dt.Date != "" {
		return time.Parse("2006-01-02", dt.Date)
	}
	return time.Time{}, fmt.Errorf("no date or dateTime in response")
}

// findFreeSlotsFromEvents finds free slots between events.
func findFreeSlotsFromEvents(events []calendar.Event, r calendar.EventRange, minDuration time.Duration) []calendar.FreeSlot {
	// Sort events by start time
	sorted := make([]calendar.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime.Before(sorted[j].StartTime)
	})

	var slots []calendar.FreeSlot
	current := r.Start

	for _, event := range sorted {
		// If there's a gap before this event
		if event.StartTime.After(current) {
			gap := event.StartTime.Sub(current)
			if gap >= minDuration {
				slots = append(slots, calendar.FreeSlot{
					Start:            current,
					End:              event.StartTime,
					Duration:         gap,
					Confidence:       1.0, // Deterministic
					ParticipantCount: 1,
				})
			}
		}

		// Move current to after this event
		if event.EndTime.After(current) {
			current = event.EndTime
		}
	}

	// Check for gap at the end
	if current.Before(r.End) {
		gap := r.End.Sub(current)
		if gap >= minDuration {
			slots = append(slots, calendar.FreeSlot{
				Start:            current,
				End:              r.End,
				Duration:         gap,
				Confidence:       1.0,
				ParticipantCount: 1,
			})
		}
	}

	return slots
}

// Verify interface compliance at compile time.
var _ calendar.EnvelopeConnector = (*Adapter)(nil)
