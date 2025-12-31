// Package gcal_read provides a read-only adapter for Google Calendar integration.
//
// CRITICAL: This adapter is READ-ONLY. It NEVER writes to Google Calendar.
// All data is transformed to canonical CalendarEventEvent format.
//
// Reference: docs/INTEGRATIONS_MATRIX_V1.md
package gcal_read

import (
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// Adapter defines the interface for Google Calendar read operations.
type Adapter interface {
	// FetchEvents retrieves calendar events and returns canonical events.
	// This is a synchronous operation - no background polling.
	FetchEvents(calendarID string, from, to time.Time) ([]*events.CalendarEventEvent, error)

	// FetchUpcomingCount returns count of events in the next N days.
	FetchUpcomingCount(calendarID string, days int) (int, error)

	// Name returns the adapter name.
	Name() string
}

// MockAdapter is a mock implementation for testing and demos.
type MockAdapter struct {
	clock  clock.Clock
	events []*MockCalendarEvent
}

// MockCalendarEvent represents a mock calendar event.
type MockCalendarEvent struct {
	CalendarID   string
	CalendarName string
	AccountEmail string
	EventUID     string
	Title        string
	Description  string
	Location     string
	StartTime    time.Time
	EndTime      time.Time
	IsAllDay     bool
	Timezone     string
	Organizer    *events.CalendarAttendee
	Attendees    []events.CalendarAttendee
	IsCancelled  bool
	IsBusy       bool
	CircleID     identity.EntityID
}

// NewMockAdapter creates a new mock Google Calendar adapter.
func NewMockAdapter(clk clock.Clock) *MockAdapter {
	return &MockAdapter{
		clock:  clk,
		events: make([]*MockCalendarEvent, 0),
	}
}

// AddMockEvent adds an event to the mock adapter.
func (a *MockAdapter) AddMockEvent(evt *MockCalendarEvent) {
	a.events = append(a.events, evt)
}

func (a *MockAdapter) Name() string {
	return "gcal_mock"
}

func (a *MockAdapter) FetchEvents(calendarID string, from, to time.Time) ([]*events.CalendarEventEvent, error) {
	now := a.clock.Now()
	var result []*events.CalendarEventEvent

	for _, evt := range a.events {
		if evt.CalendarID != calendarID {
			continue
		}
		// Check if event overlaps with the time range
		if evt.EndTime.Before(from) || evt.StartTime.After(to) {
			continue
		}

		event := events.NewCalendarEventEvent(
			"google_calendar",
			evt.CalendarID,
			evt.EventUID,
			evt.AccountEmail,
			now,
			evt.StartTime,
		)

		event.CalendarName = evt.CalendarName
		event.Title = evt.Title
		event.Description = evt.Description
		event.Location = evt.Location
		event.StartTime = evt.StartTime
		event.EndTime = evt.EndTime
		event.IsAllDay = evt.IsAllDay
		event.Timezone = evt.Timezone
		event.Organizer = evt.Organizer
		event.Attendees = evt.Attendees
		event.AttendeeCount = len(evt.Attendees)
		event.IsCancelled = evt.IsCancelled
		event.IsBusy = evt.IsBusy
		event.MyResponseStatus = events.RSVPAccepted // Default for owned events

		// Set circle
		event.Circle = evt.CircleID

		result = append(result, event)
	}

	return result, nil
}

func (a *MockAdapter) FetchUpcomingCount(calendarID string, days int) (int, error) {
	now := a.clock.Now()
	end := now.AddDate(0, 0, days)

	count := 0
	for _, evt := range a.events {
		if evt.CalendarID != calendarID {
			continue
		}
		if evt.StartTime.After(now) && evt.StartTime.Before(end) {
			count++
		}
	}
	return count, nil
}

// Verify interface compliance.
var _ Adapter = (*MockAdapter)(nil)
