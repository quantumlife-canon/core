// Package calendar provides calendar connector types and interfaces.
// This file defines provider-neutral domain types for calendar operations.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package calendar

import (
	"time"
)

// EventRange specifies a time range for querying events.
type EventRange struct {
	// Start is the beginning of the time range (inclusive).
	Start time.Time

	// End is the end of the time range (exclusive).
	End time.Time
}

// FreeSlot represents a period of free time across calendars.
type FreeSlot struct {
	// Start is when the free slot begins.
	Start time.Time

	// End is when the free slot ends.
	End time.Time

	// Duration is the length of the free slot.
	Duration time.Duration

	// Confidence indicates how certain we are about this slot.
	// For deterministic operations, this is always 1.0.
	Confidence float64

	// ParticipantCount is how many participants are free during this slot.
	ParticipantCount int
}

// ProviderInfo contains metadata about a calendar provider.
type ProviderInfo struct {
	// ID is the provider identifier (e.g., "google", "microsoft", "mock").
	ID string

	// Name is the human-readable provider name.
	Name string

	// Capabilities lists what operations this provider supports.
	Capabilities []string

	// IsConfigured indicates if the provider has valid credentials.
	IsConfigured bool
}

// ReadResult contains the result of a calendar read operation.
type ReadResult struct {
	// Events are the calendar events retrieved.
	Events []Event

	// Provider is which provider the events came from.
	Provider string

	// Range is the time range that was queried.
	Range EventRange

	// FromCache indicates if the result came from cache.
	FromCache bool

	// FetchedAt is when the data was fetched.
	FetchedAt time.Time
}

// FreeSlotResult contains the result of finding free slots.
type FreeSlotResult struct {
	// Slots are the free slots found.
	Slots []FreeSlot

	// Range is the time range that was searched.
	Range EventRange

	// MinDuration is the minimum slot duration that was requested.
	MinDuration time.Duration

	// ParticipantCalendars lists the calendars that were checked.
	ParticipantCalendars []string
}

// SourceProvider identifies which provider an event came from.
type SourceProvider string

// Known source providers.
const (
	SourceMock      SourceProvider = "mock"
	SourceGoogle    SourceProvider = "google"
	SourceMicrosoft SourceProvider = "microsoft"
)

// ExtendedEvent extends Event with provider-specific metadata.
type ExtendedEvent struct {
	Event

	// SourceProvider identifies which provider this event came from.
	SourceProvider SourceProvider

	// ProviderEventID is the provider's native event ID.
	ProviderEventID string

	// LastSynced is when this event was last synced from the provider.
	LastSynced time.Time

	// IsAllDay indicates if this is an all-day event.
	IsAllDay bool

	// Organizer is the event organizer email.
	Organizer string

	// ResponseStatus is the user's response (accepted, tentative, declined).
	ResponseStatus string

	// IsBusy indicates if this time should be considered busy.
	IsBusy bool
}
