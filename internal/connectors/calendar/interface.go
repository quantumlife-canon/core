// Package calendar defines the calendar connector interface.
// This is a DATA PLANE component — deterministic only, NO LLM/SLM.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
//
// CRITICAL: This package is read-only in v4. ProposeEvent returns a proposal
// without making any external writes.
package calendar

import (
	"context"
	"time"
)

// Connector defines the calendar connector interface.
// All operations in v4 are read-only or propose-only (no external writes).
type Connector interface {
	// ID returns the connector identifier.
	ID() string

	// Capabilities returns the connector's capabilities.
	Capabilities() []string

	// RequiredScopes returns scopes required for this connector.
	RequiredScopes() []string

	// ListEvents returns events in the specified time range.
	// This is a read-only operation.
	ListEvents(ctx context.Context, req ListEventsRequest) ([]Event, error)

	// ProposeEvent creates a proposed event without writing to external calendar.
	// Returns the proposal that would be created.
	// CRITICAL: This does NOT write to any external service.
	ProposeEvent(ctx context.Context, req ProposeEventRequest) (*ProposedEvent, error)

	// HealthCheck verifies the connector is operational.
	HealthCheck(ctx context.Context) error
}

// ListEventsRequest contains parameters for listing events.
type ListEventsRequest struct {
	// StartTime is the beginning of the time range.
	StartTime time.Time

	// EndTime is the end of the time range.
	EndTime time.Time

	// CalendarID identifies which calendar to query (optional).
	CalendarID string
}

// Event represents a calendar event.
type Event struct {
	// ID uniquely identifies this event.
	ID string

	// Title is the event title.
	Title string

	// Description is the event description.
	Description string

	// StartTime is when the event starts.
	StartTime time.Time

	// EndTime is when the event ends.
	EndTime time.Time

	// Location is the event location.
	Location string

	// Attendees lists the event attendees.
	Attendees []string

	// CalendarID identifies the calendar.
	CalendarID string
}

// ProposeEventRequest contains parameters for proposing an event.
type ProposeEventRequest struct {
	// Title is the proposed event title.
	Title string

	// Description is the proposed event description.
	Description string

	// StartTime is the proposed start time.
	StartTime time.Time

	// EndTime is the proposed end time.
	EndTime time.Time

	// Location is the proposed location.
	Location string

	// Attendees lists the proposed attendees.
	Attendees []string

	// CalendarID identifies the target calendar.
	CalendarID string
}

// ProposedEvent represents a proposed calendar event.
// This is NOT written to any external service.
type ProposedEvent struct {
	// ProposalID uniquely identifies this proposal.
	ProposalID string

	// Event contains the proposed event details.
	Event Event

	// Simulated indicates this is a simulated proposal (no external write).
	Simulated bool

	// Message describes what would happen if executed.
	Message string

	// ConflictingEvents lists any events that would conflict.
	ConflictingEvents []Event
}
