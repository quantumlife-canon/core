// Package impl_mock provides a mock implementation of the calendar connector.
// This is for demo and testing purposes only.
//
// CRITICAL: This implementation does NOT perform any external writes.
// All operations are deterministic for testing.
package impl_mock

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/internal/connectors/calendar"
	"quantumlife/pkg/primitives"
)

// MockConnector implements the calendar.Connector interface with mock data.
type MockConnector struct {
	mu            sync.RWMutex
	events        []calendar.Event
	proposalCount int
	clockFunc     func() time.Time // Injected clock for determinism
}

// NewMockConnector creates a new mock calendar connector.
func NewMockConnector() *MockConnector {
	return &MockConnector{
		events:    defaultMockEvents(),
		clockFunc: time.Now,
	}
}

// NewMockConnectorWithClock creates a mock connector with an injected clock.
func NewMockConnectorWithClock(clockFunc func() time.Time) *MockConnector {
	return &MockConnector{
		events:    defaultMockEvents(),
		clockFunc: clockFunc,
	}
}

// defaultMockEvents returns a set of mock calendar events for testing.
func defaultMockEvents() []calendar.Event {
	// Use a fixed base time for deterministic testing
	baseDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	return []calendar.Event{
		{
			ID:          "evt-1",
			Title:       "Morning Exercise",
			Description: "Daily workout routine",
			StartTime:   baseDate.Add(7 * time.Hour),
			EndTime:     baseDate.Add(8 * time.Hour),
			Location:    "Home Gym",
			CalendarID:  "family",
		},
		{
			ID:          "evt-2",
			Title:       "Family Breakfast",
			Description: "Morning meal together",
			StartTime:   baseDate.Add(8 * time.Hour),
			EndTime:     baseDate.Add(9 * time.Hour),
			Location:    "Kitchen",
			CalendarID:  "family",
		},
		{
			ID:          "evt-3",
			Title:       "Work Block",
			Description: "Focus time for work",
			StartTime:   baseDate.Add(9 * time.Hour),
			EndTime:     baseDate.Add(17 * time.Hour),
			Location:    "Office",
			CalendarID:  "personal",
		},
		{
			ID:          "evt-4",
			Title:       "Family Dinner",
			Description: "Evening meal together",
			StartTime:   baseDate.Add(18 * time.Hour),
			EndTime:     baseDate.Add(19 * time.Hour),
			Location:    "Dining Room",
			CalendarID:  "family",
		},
		{
			ID:          "evt-5",
			Title:       "Kids Bedtime Routine",
			Description: "Story time and bedtime",
			StartTime:   baseDate.Add(20 * time.Hour),
			EndTime:     baseDate.Add(21 * time.Hour),
			Location:    "Kids Room",
			CalendarID:  "family",
		},
	}
}

// ID returns the connector identifier.
func (m *MockConnector) ID() string {
	return "calendar-mock"
}

// Capabilities returns the connector's capabilities.
func (m *MockConnector) Capabilities() []string {
	return []string{"list_events", "propose_event"}
}

// RequiredScopes returns scopes required for this connector.
func (m *MockConnector) RequiredScopes() []string {
	return []string{"calendar:read", "calendar:write"}
}

// ListEvents returns events in the specified time range.
func (m *MockConnector) ListEvents(ctx context.Context, req calendar.ListEventsRequest) ([]calendar.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []calendar.Event
	for _, evt := range m.events {
		// Check if event is within the time range
		if !evt.EndTime.Before(req.StartTime) && !evt.StartTime.After(req.EndTime) {
			// Check calendar filter
			if req.CalendarID == "" || evt.CalendarID == req.CalendarID {
				results = append(results, evt)
			}
		}
	}

	return results, nil
}

// ProposeEvent creates a proposed event without writing to external calendar.
func (m *MockConnector) ProposeEvent(ctx context.Context, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.proposalCount++
	proposalID := fmt.Sprintf("proposal-%d", m.proposalCount)

	// Create the proposed event
	proposed := calendar.Event{
		ID:          fmt.Sprintf("proposed-%s", proposalID),
		Title:       req.Title,
		Description: req.Description,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Location:    req.Location,
		Attendees:   req.Attendees,
		CalendarID:  req.CalendarID,
	}

	// Check for conflicts
	var conflicts []calendar.Event
	for _, evt := range m.events {
		if req.CalendarID != "" && evt.CalendarID != req.CalendarID {
			continue
		}
		// Check for time overlap
		if !evt.EndTime.Before(req.StartTime) && !evt.StartTime.After(req.EndTime) {
			conflicts = append(conflicts, evt)
		}
	}

	message := "SIMULATED: Event would be created on calendar"
	if len(conflicts) > 0 {
		message = fmt.Sprintf("SIMULATED: Event would be created with %d potential conflicts", len(conflicts))
	}

	return &calendar.ProposedEvent{
		ProposalID:        proposalID,
		Event:             proposed,
		Simulated:         true,
		Message:           message,
		ConflictingEvents: conflicts,
	}, nil
}

// HealthCheck verifies the connector is operational.
func (m *MockConnector) HealthCheck(ctx context.Context) error {
	return nil // Mock is always healthy
}

// SetEvents allows tests to set custom events.
func (m *MockConnector) SetEvents(events []calendar.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = events
}

// GetProposalCount returns the number of proposals made (for testing).
func (m *MockConnector) GetProposalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proposalCount
}

// ListEventsWithEnvelope returns events with envelope validation.
func (m *MockConnector) ListEventsWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange) ([]calendar.Event, error) {
	// Validate envelope for read operations
	if err := env.ValidateForRead(); err != nil {
		return nil, err
	}

	return m.ListEvents(ctx, calendar.ListEventsRequest{
		StartTime: r.Start,
		EndTime:   r.End,
	})
}

// FindFreeSlots finds free slots between events.
func (m *MockConnector) FindFreeSlots(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange, minDuration time.Duration) ([]calendar.FreeSlot, error) {
	// Validate envelope for read operations
	if err := env.ValidateForRead(); err != nil {
		return nil, err
	}

	events, err := m.ListEvents(ctx, calendar.ListEventsRequest{
		StartTime: r.Start,
		EndTime:   r.End,
	})
	if err != nil {
		return nil, err
	}

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

	return slots, nil
}

// ProposeEventWithEnvelope creates a proposed event with envelope validation.
func (m *MockConnector) ProposeEventWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	// Validate envelope
	if err := env.Validate(); err != nil {
		return nil, err
	}

	return m.ProposeEvent(ctx, req)
}

// ProviderInfo returns information about the mock provider.
func (m *MockConnector) ProviderInfo() calendar.ProviderInfo {
	return calendar.ProviderInfo{
		ID:           "mock",
		Name:         "Mock Calendar",
		Capabilities: m.Capabilities(),
		IsConfigured: true,
	}
}

// Verify interface compliance at compile time.
var (
	_ calendar.Connector         = (*MockConnector)(nil)
	_ calendar.EnvelopeConnector = (*MockConnector)(nil)
)
