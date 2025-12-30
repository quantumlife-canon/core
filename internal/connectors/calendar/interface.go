// Package calendar defines the calendar connector interface.
// This is a DATA PLANE component — deterministic only, NO LLM/SLM.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
//
// CRITICAL: This package is read-only in v5. ProposeEvent returns a proposal
// without making any external writes. Write operations are NOT implemented.
package calendar

import (
	"context"
	"time"

	"quantumlife/pkg/primitives"
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

// EnvelopeConnector extends Connector with envelope-based operations.
// All operations require an ExecutionEnvelope for traceability.
// This is the v5 interface for real provider integrations.
type EnvelopeConnector interface {
	Connector

	// ListEventsWithEnvelope returns events in the specified time range.
	// The envelope provides traceability context (TraceID, Mode, etc.).
	// This is a read-only operation.
	ListEventsWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, r EventRange) ([]Event, error)

	// FindFreeSlots finds free slots across calendars.
	// The envelope provides traceability context.
	// Returns slots of at least minDuration length.
	FindFreeSlots(ctx context.Context, env primitives.ExecutionEnvelope, r EventRange, minDuration time.Duration) ([]FreeSlot, error)

	// ProposeEventWithEnvelope creates a proposed event without writing.
	// The envelope provides traceability context.
	// CRITICAL: This does NOT write to any external service in v5.
	ProposeEventWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, req ProposeEventRequest) (*ProposedEvent, error)

	// ProviderInfo returns information about the provider.
	ProviderInfo() ProviderInfo
}

// MultiProviderConnector aggregates multiple calendar providers.
// This allows reading from multiple calendars (e.g., Google + Microsoft).
type MultiProviderConnector interface {
	// AddProvider adds a provider to the aggregator.
	AddProvider(provider EnvelopeConnector)

	// ListEventsFromAll lists events from all configured providers.
	ListEventsFromAll(ctx context.Context, env primitives.ExecutionEnvelope, r EventRange) ([]ReadResult, error)

	// FindFreeSlotsAcrossAll finds free slots across all providers.
	FindFreeSlotsAcrossAll(ctx context.Context, env primitives.ExecutionEnvelope, r EventRange, minDuration time.Duration) (FreeSlotResult, error)

	// GetConfiguredProviders returns info about all configured providers.
	GetConfiguredProviders() []ProviderInfo
}

// ============================================================================
// v6 Write Connector Interface - Execute Mode Only
// ============================================================================

// WriteConnector extends EnvelopeConnector with write operations.
// CRITICAL: All write operations require:
// - Mode == Execute
// - Explicit human approval in the envelope
// - calendar:write scope granted
type WriteConnector interface {
	EnvelopeConnector

	// CreateEvent creates a new calendar event.
	//
	// CRITICAL: This performs an EXTERNAL WRITE. Requirements:
	// - env.Mode MUST be Execute
	// - ApprovedByHuman must be true (from authorization proof)
	// - calendar:write scope must be granted
	//
	// Returns a receipt with the external event ID for rollback capability.
	CreateEvent(ctx context.Context, env primitives.ExecutionEnvelope, req CreateEventRequest) (*CreateEventReceipt, error)

	// DeleteEvent deletes a calendar event.
	//
	// CRITICAL: This performs an EXTERNAL WRITE (deletion). Requirements:
	// - env.Mode MUST be Execute
	// - ApprovedByHuman must be true (from authorization proof)
	// - calendar:write scope must be granted
	//
	// This is used for rollback after failed settlement.
	DeleteEvent(ctx context.Context, env primitives.ExecutionEnvelope, req DeleteEventRequest) (*DeleteEventReceipt, error)

	// SupportsWrite returns true if the provider supports write operations.
	SupportsWrite() bool
}

// Write operation errors.
var (
	// ErrExecuteModeRequired is returned when write is attempted without Execute mode.
	ErrExecuteModeRequired = writeError("execute mode required for write operations")

	// ErrApprovalRequired is returned when write is attempted without human approval.
	ErrApprovalRequired = writeError("explicit human approval required for write operations")

	// ErrWriteScopeRequired is returned when calendar:write scope is not granted.
	ErrWriteScopeRequired = writeError("calendar:write scope required for write operations")

	// ErrWriteNotSupported is returned when provider doesn't support writes.
	ErrWriteNotSupported = writeError("write operations not supported by this provider")

	// ErrRollbackFailed is returned when event deletion fails during rollback.
	ErrRollbackFailed = writeError("rollback (event deletion) failed")
)

type writeError string

func (e writeError) Error() string { return string(e) }
