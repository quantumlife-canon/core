// Package gcal_read provides a read-only adapter for Google Calendar integration.
//
// This file implements the real HTTP-based Google Calendar adapter.
// CRITICAL: This adapter is READ-ONLY. It NEVER writes to Google Calendar.
//
// Reference: docs/INTEGRATIONS_MATRIX_V1.md
package gcal_read

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

const (
	// Google Calendar API base URL
	calendarAPIBase = "https://www.googleapis.com/calendar/v3"
)

// TokenMinter mints access tokens for read-only operations.
// This interface allows injection for testing.
type TokenMinter interface {
	MintReadOnlyAccessToken(ctx context.Context, circleID string, provider auth.ProviderID, requiredScopes []string) (auth.AccessToken, error)
}

// RealAdapter implements the Google Calendar read adapter using real HTTP calls.
type RealAdapter struct {
	broker     TokenMinter
	httpClient *http.Client
	clock      clock.Clock
	circleID   string
}

// NewRealAdapter creates a new real Google Calendar adapter.
func NewRealAdapter(broker *impl_inmem.Broker, clk clock.Clock, circleID string) *RealAdapter {
	return &RealAdapter{
		broker:     broker,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		clock:      clk,
		circleID:   circleID,
	}
}

// NewRealAdapterWithClient creates a real adapter with custom HTTP client (for testing).
func NewRealAdapterWithClient(broker TokenMinter, httpClient *http.Client, clk clock.Clock, circleID string) *RealAdapter {
	return &RealAdapter{
		broker:     broker,
		httpClient: httpClient,
		clock:      clk,
		circleID:   circleID,
	}
}

func (a *RealAdapter) Name() string {
	return "gcal_real"
}

// FetchEvents retrieves calendar events and returns canonical events.
func (a *RealAdapter) FetchEvents(calendarID string, from, to time.Time) ([]*events.CalendarEventEvent, error) {
	ctx := context.Background()

	// Mint read-only access token
	token, err := a.broker.MintReadOnlyAccessToken(ctx, a.circleID, auth.ProviderGoogle, []string{"calendar:read"})
	if err != nil {
		return nil, fmt.Errorf("mint token: %w", err)
	}

	// Use 'primary' as default calendar ID
	if calendarID == "" {
		calendarID = "primary"
	}

	// List events
	gcalEvents, err := a.listEvents(ctx, token.Token, calendarID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	// Convert to canonical events
	now := a.clock.Now()
	var result []*events.CalendarEventEvent

	for _, evt := range gcalEvents {
		event := a.eventToCanonical(calendarID, evt, now)
		result = append(result, event)
	}

	return result, nil
}

// FetchUpcomingCount returns count of events in the next N days.
func (a *RealAdapter) FetchUpcomingCount(calendarID string, days int) (int, error) {
	ctx := context.Background()

	// Mint read-only access token
	token, err := a.broker.MintReadOnlyAccessToken(ctx, a.circleID, auth.ProviderGoogle, []string{"calendar:read"})
	if err != nil {
		return 0, fmt.Errorf("mint token: %w", err)
	}

	// Use 'primary' as default calendar ID
	if calendarID == "" {
		calendarID = "primary"
	}

	now := a.clock.Now()
	to := now.AddDate(0, 0, days)

	gcalEvents, err := a.listEvents(ctx, token.Token, calendarID, now, to)
	if err != nil {
		return 0, fmt.Errorf("list events: %w", err)
	}

	return len(gcalEvents), nil
}

// listEvents lists calendar events in a time range.
func (a *RealAdapter) listEvents(ctx context.Context, accessToken, calendarID string, from, to time.Time) ([]gcalEvent, error) {
	endpoint := fmt.Sprintf("%s/calendars/%s/events", calendarAPIBase, url.PathEscape(calendarID))

	params := url.Values{}
	params.Set("timeMin", from.Format(time.RFC3339))
	params.Set("timeMax", to.Format(time.RFC3339))
	params.Set("singleEvents", "true") // Expand recurring events
	params.Set("orderBy", "startTime")
	params.Set("maxResults", "250")

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("calendar API error: %d - %s", resp.StatusCode, string(body))
	}

	var listResp gcalListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	return listResp.Items, nil
}

// eventToCanonical converts a Google Calendar event to a canonical event.
func (a *RealAdapter) eventToCanonical(calendarID string, evt gcalEvent, capturedAt time.Time) *events.CalendarEventEvent {
	// Parse start time
	startTime := parseGcalTime(evt.Start)
	endTime := parseGcalTime(evt.End)
	isAllDay := evt.Start.Date != ""

	// Determine account email from organizer or calendar ID
	accountEmail := calendarID
	if evt.Organizer.Email != "" && evt.Organizer.Self {
		accountEmail = evt.Organizer.Email
	}

	event := events.NewCalendarEventEvent(
		"google_calendar",
		calendarID,
		evt.ID,
		accountEmail,
		capturedAt,
		startTime,
	)

	event.CalendarName = calendarID // Could be enhanced with calendar metadata
	event.EventUID = evt.ICalUID
	event.Title = evt.Summary
	event.Description = evt.Description
	event.Location = evt.Location
	event.StartTime = startTime
	event.EndTime = endTime
	event.IsAllDay = isAllDay
	event.Timezone = evt.Start.TimeZone
	event.IsCancelled = evt.Status == "cancelled"
	event.IsBusy = evt.Transparency != "transparent"

	// Parse recurrence
	if len(evt.Recurrence) > 0 {
		event.IsRecurring = true
		event.RecurrenceRule = strings.Join(evt.Recurrence, "\n")
	}
	if evt.RecurringEventID != "" {
		event.IsRecurring = true
	}

	// Parse organizer
	if evt.Organizer.Email != "" {
		event.Organizer = &events.CalendarAttendee{
			Email:          evt.Organizer.Email,
			Name:           evt.Organizer.DisplayName,
			ResponseStatus: events.RSVPAccepted,
		}
	}

	// Parse attendees
	for _, att := range evt.Attendees {
		attendee := events.CalendarAttendee{
			Email:          att.Email,
			Name:           att.DisplayName,
			ResponseStatus: mapResponseStatus(att.ResponseStatus),
			IsOptional:     att.Optional,
		}
		event.Attendees = append(event.Attendees, attendee)

		// Track our own response status
		if att.Self {
			event.MyResponseStatus = mapResponseStatus(att.ResponseStatus)
		}
	}
	event.AttendeeCount = len(evt.Attendees)

	// If no self attendee found, assume accepted
	if event.MyResponseStatus == "" {
		event.MyResponseStatus = events.RSVPAccepted
	}

	// Parse conference URL from conferenceData
	if evt.ConferenceData.EntryPoints != nil {
		for _, ep := range evt.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" {
				event.ConferenceURL = ep.URI
				break
			}
		}
	}

	// Also check hangoutLink for older events
	if event.ConferenceURL == "" && evt.HangoutLink != "" {
		event.ConferenceURL = evt.HangoutLink
	}

	return event
}

// Google Calendar API response types

type gcalListResponse struct {
	Kind             string      `json:"kind"`
	Etag             string      `json:"etag"`
	Summary          string      `json:"summary"`
	Description      string      `json:"description"`
	Updated          string      `json:"updated"`
	TimeZone         string      `json:"timeZone"`
	AccessRole       string      `json:"accessRole"`
	NextPageToken    string      `json:"nextPageToken"`
	NextSyncToken    string      `json:"nextSyncToken"`
	Items            []gcalEvent `json:"items"`
	DefaultReminders []struct {
		Method  string `json:"method"`
		Minutes int    `json:"minutes"`
	} `json:"defaultReminders"`
}

type gcalEvent struct {
	Kind                    string             `json:"kind"`
	Etag                    string             `json:"etag"`
	ID                      string             `json:"id"`
	Status                  string             `json:"status"` // confirmed, tentative, cancelled
	HTMLLink                string             `json:"htmlLink"`
	Created                 string             `json:"created"`
	Updated                 string             `json:"updated"`
	Summary                 string             `json:"summary"`
	Description             string             `json:"description"`
	Location                string             `json:"location"`
	Creator                 gcalPerson         `json:"creator"`
	Organizer               gcalPerson         `json:"organizer"`
	Start                   gcalDateTime       `json:"start"`
	End                     gcalDateTime       `json:"end"`
	EndTimeUnspecified      bool               `json:"endTimeUnspecified"`
	Recurrence              []string           `json:"recurrence"`
	RecurringEventID        string             `json:"recurringEventId"`
	OriginalStartTime       gcalDateTime       `json:"originalStartTime"`
	Transparency            string             `json:"transparency"` // opaque (default, blocks time) or transparent
	Visibility              string             `json:"visibility"`
	ICalUID                 string             `json:"iCalUID"`
	Sequence                int                `json:"sequence"`
	Attendees               []gcalAttendee     `json:"attendees"`
	AttendeesOmitted        bool               `json:"attendeesOmitted"`
	HangoutLink             string             `json:"hangoutLink"`
	ConferenceData          gcalConferenceData `json:"conferenceData"`
	GuestsCanInviteOthers   bool               `json:"guestsCanInviteOthers"`
	GuestsCanModify         bool               `json:"guestsCanModify"`
	GuestsCanSeeOtherGuests bool               `json:"guestsCanSeeOtherGuests"`
	PrivateCopy             bool               `json:"privateCopy"`
	Locked                  bool               `json:"locked"`
	EventType               string             `json:"eventType"` // default, outOfOffice, focusTime
}

type gcalPerson struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Self        bool   `json:"self"`
}

type gcalDateTime struct {
	Date     string `json:"date"`     // For all-day events (YYYY-MM-DD)
	DateTime string `json:"dateTime"` // For timed events (RFC3339)
	TimeZone string `json:"timeZone"` // IANA timezone
}

type gcalAttendee struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	DisplayName      string `json:"displayName"`
	Organizer        bool   `json:"organizer"`
	Self             bool   `json:"self"`
	Resource         bool   `json:"resource"`
	Optional         bool   `json:"optional"`
	ResponseStatus   string `json:"responseStatus"` // needsAction, declined, tentative, accepted
	Comment          string `json:"comment"`
	AdditionalGuests int    `json:"additionalGuests"`
}

type gcalConferenceData struct {
	EntryPoints        []gcalEntryPoint `json:"entryPoints"`
	ConferenceSolution struct {
		Key struct {
			Type string `json:"type"`
		} `json:"key"`
		Name    string `json:"name"`
		IconURI string `json:"iconUri"`
	} `json:"conferenceSolution"`
	ConferenceID string `json:"conferenceId"`
}

type gcalEntryPoint struct {
	EntryPointType string `json:"entryPointType"` // video, phone, sip, more
	URI            string `json:"uri"`
	Label          string `json:"label"`
	Pin            string `json:"pin"`
	AccessCode     string `json:"accessCode"`
	MeetingCode    string `json:"meetingCode"`
	Passcode       string `json:"passcode"`
	Password       string `json:"password"`
}

// Helper functions

// parseGcalTime parses a Google Calendar datetime.
func parseGcalTime(dt gcalDateTime) time.Time {
	// All-day events use Date
	if dt.Date != "" {
		t, err := time.Parse("2006-01-02", dt.Date)
		if err != nil {
			return time.Time{}
		}
		return t
	}

	// Timed events use DateTime
	if dt.DateTime != "" {
		t, err := time.Parse(time.RFC3339, dt.DateTime)
		if err != nil {
			return time.Time{}
		}
		return t
	}

	return time.Time{}
}

// mapResponseStatus maps Google Calendar response status to canonical.
func mapResponseStatus(status string) events.RSVPStatus {
	switch status {
	case "accepted":
		return events.RSVPAccepted
	case "declined":
		return events.RSVPDeclined
	case "tentative":
		return events.RSVPTentative
	case "needsAction":
		return events.RSVPNeedsAction
	default:
		return events.RSVPNeedsAction
	}
}

// Verify interface compliance.
var _ Adapter = (*RealAdapter)(nil)

// Unused but kept for reference - used in identity normalization
var _ = identity.EntityID("")
