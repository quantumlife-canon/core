// Package microsoft provides a Microsoft Graph Calendar API adapter.
// This adapter implements read-only operations for v5.
//
// CRITICAL: This adapter does NOT perform any write operations.
// All operations are read-only (GET requests only).
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package microsoft

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
	// CalendarViewURL is the Microsoft Graph calendarView endpoint.
	// See: https://learn.microsoft.com/en-us/graph/api/calendar-list-calendarview
	CalendarViewURL = "https://graph.microsoft.com/v1.0/me/calendarView"
)

// Adapter implements the calendar.EnvelopeConnector for Microsoft Graph.
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

// NewAdapter creates a new Microsoft Graph adapter.
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
	return "microsoft-calendar"
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
		ID:           "microsoft",
		Name:         "Microsoft Calendar",
		Capabilities: a.Capabilities(),
		IsConfigured: a.isConfigured,
	}
}

// ListEvents returns events in the specified time range (legacy interface).
func (a *Adapter) ListEvents(ctx context.Context, req calendar.ListEventsRequest) ([]calendar.Event, error) {
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
	token, err := a.broker.MintAccessToken(ctx, env, auth.ProviderMicrosoft, []string{"calendar:read"})
	if err != nil {
		return nil, fmt.Errorf("failed to mint access token: %w", err)
	}

	// Build request
	params := url.Values{}
	params.Set("startDateTime", r.Start.Format(time.RFC3339))
	params.Set("endDateTime", r.End.Format(time.RFC3339))
	params.Set("$orderby", "start/dateTime")
	params.Set("$top", "250")

	fullURL := CalendarViewURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Prefer", "outlook.timezone=\"UTC\"")

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
	var response graphEventsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to domain events
	events := make([]calendar.Event, 0, len(response.Value))
	for _, item := range response.Value {
		event, err := convertGraphEvent(item)
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
	proposalID := fmt.Sprintf("microsoft-proposal-%d", a.idCounter)
	a.mu.Unlock()

	// Check for conflicts by listing events in the time range
	events, err := a.ListEventsWithEnvelope(ctx, env, calendar.EventRange{
		Start: req.StartTime,
		End:   req.EndTime,
	})
	if err != nil {
		events = nil
	}

	message := "SIMULATED: Event would be created on Microsoft Calendar (v5 read-only)"
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

// Microsoft Graph API response types.
type graphEventsResponse struct {
	Value []graphEvent `json:"value"`
}

type graphEvent struct {
	ID             string          `json:"id"`
	Subject        string          `json:"subject"`
	BodyPreview    string          `json:"bodyPreview"`
	Location       graphLocation   `json:"location"`
	Start          graphDateTime   `json:"start"`
	End            graphDateTime   `json:"end"`
	Attendees      []graphAttendee `json:"attendees"`
	IsAllDay       bool            `json:"isAllDay"`
	ShowAs         string          `json:"showAs"`
	ResponseStatus graphResponse   `json:"responseStatus"`
	Organizer      graphOrganizer  `json:"organizer"`
}

type graphLocation struct {
	DisplayName string `json:"displayName"`
}

type graphDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type graphAttendee struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

type graphEmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type graphResponse struct {
	Response string `json:"response"`
}

type graphOrganizer struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

// convertGraphEvent converts a Microsoft Graph event to a domain event.
func convertGraphEvent(g graphEvent) (calendar.Event, error) {
	startTime, err := parseGraphDateTime(g.Start)
	if err != nil {
		return calendar.Event{}, err
	}

	endTime, err := parseGraphDateTime(g.End)
	if err != nil {
		return calendar.Event{}, err
	}

	attendees := make([]string, len(g.Attendees))
	for i, a := range g.Attendees {
		attendees[i] = a.EmailAddress.Address
	}

	return calendar.Event{
		ID:          g.ID,
		Title:       g.Subject,
		Description: g.BodyPreview,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    g.Location.DisplayName,
		Attendees:   attendees,
		CalendarID:  "primary",
	}, nil
}

// parseGraphDateTime parses a Microsoft Graph datetime struct.
func parseGraphDateTime(dt graphDateTime) (time.Time, error) {
	// Microsoft Graph returns ISO 8601 without timezone offset when UTC is requested
	formats := []string{
		"2006-01-02T15:04:05.9999999",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dt.DateTime); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse datetime: %s", dt.DateTime)
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
					Confidence:       1.0,
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
