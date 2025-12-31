// Package gcal_read provides a read-only adapter for Google Calendar integration.
// This file contains httptest-based tests for the real adapter.
package gcal_read

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
)

// mockTokenMinter implements TokenMinter for testing.
type mockTokenMinter struct {
	token auth.AccessToken
	err   error
}

func (m *mockTokenMinter) MintReadOnlyAccessToken(ctx context.Context, circleID string, provider auth.ProviderID, requiredScopes []string) (auth.AccessToken, error) {
	return m.token, m.err
}

func TestRealAdapter_FetchEvents(t *testing.T) {
	// Create a test server that mimics Google Calendar API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", authHeader)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/calendar/v3/calendars/primary/events" && r.Method == "GET" {
			resp := gcalListResponse{
				Kind:       "calendar#events",
				Summary:    "Primary Calendar",
				TimeZone:   "Europe/London",
				AccessRole: "owner",
				Items: []gcalEvent{
					{
						ID:       "event-1",
						Status:   "confirmed",
						Summary:  "Team Standup",
						Location: "Meeting Room A",
						Start: gcalDateTime{
							DateTime: "2024-01-15T09:00:00Z",
							TimeZone: "Europe/London",
						},
						End: gcalDateTime{
							DateTime: "2024-01-15T09:30:00Z",
							TimeZone: "Europe/London",
						},
						Organizer: gcalPerson{
							Email:       "organizer@example.com",
							DisplayName: "Team Lead",
							Self:        true,
						},
						Attendees: []gcalAttendee{
							{
								Email:          "member1@example.com",
								DisplayName:    "Team Member 1",
								ResponseStatus: "accepted",
							},
							{
								Email:          "member2@example.com",
								DisplayName:    "Team Member 2",
								ResponseStatus: "tentative",
								Optional:       true,
							},
						},
						ICalUID:      "event-1@google.com",
						HangoutLink:  "https://meet.google.com/abc-defg-hij",
						Transparency: "opaque",
					},
					{
						ID:      "event-2",
						Status:  "confirmed",
						Summary: "All Day Planning",
						Start: gcalDateTime{
							Date: "2024-01-16",
						},
						End: gcalDateTime{
							Date: "2024-01-17",
						},
						Organizer: gcalPerson{
							Email: "organizer@example.com",
							Self:  true,
						},
						ICalUID: "event-2@google.com",
					},
					{
						ID:         "event-3",
						Status:     "confirmed",
						Summary:    "Recurring Meeting",
						Recurrence: []string{"RRULE:FREQ=WEEKLY;BYDAY=MO"},
						Start: gcalDateTime{
							DateTime: "2024-01-15T14:00:00Z",
						},
						End: gcalDateTime{
							DateTime: "2024-01-15T15:00:00Z",
						},
						ICalUID: "event-3@google.com",
						ConferenceData: gcalConferenceData{
							EntryPoints: []gcalEntryPoint{
								{
									EntryPointType: "video",
									URI:            "https://zoom.us/j/123456",
								},
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		} else {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with test server
	minter := &mockTokenMinter{
		token: auth.AccessToken{
			Token:    "test-token",
			Expiry:   time.Now().Add(time.Hour),
			Provider: auth.ProviderGoogle,
		},
	}

	client := &http.Client{
		Transport: &testTransport{server: server},
	}

	fixedClock := clock.NewFixed(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	adapter := NewRealAdapterWithClient(minter, client, fixedClock, "test-circle")

	// Fetch events
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)
	calEvents, err := adapter.FetchEvents("primary", from, to)
	if err != nil {
		t.Fatalf("FetchEvents failed: %v", err)
	}

	// Verify results
	if len(calEvents) != 3 {
		t.Fatalf("expected 3 events, got %d", len(calEvents))
	}

	// Check first event (timed event with attendees)
	evt1 := calEvents[0]
	if evt1.Title != "Team Standup" {
		t.Errorf("expected title 'Team Standup', got %s", evt1.Title)
	}
	if evt1.Location != "Meeting Room A" {
		t.Errorf("expected location 'Meeting Room A', got %s", evt1.Location)
	}
	if evt1.IsAllDay {
		t.Error("expected timed event, not all-day")
	}
	if len(evt1.Attendees) != 2 {
		t.Errorf("expected 2 attendees, got %d", len(evt1.Attendees))
	}
	if evt1.ConferenceURL != "https://meet.google.com/abc-defg-hij" {
		t.Errorf("expected hangout link, got %s", evt1.ConferenceURL)
	}
	if !evt1.IsBusy {
		t.Error("expected event to block time (opaque)")
	}

	// Check attendee response statuses
	if evt1.Attendees[0].ResponseStatus != events.RSVPAccepted {
		t.Errorf("expected first attendee accepted, got %s", evt1.Attendees[0].ResponseStatus)
	}
	if evt1.Attendees[1].ResponseStatus != events.RSVPTentative {
		t.Errorf("expected second attendee tentative, got %s", evt1.Attendees[1].ResponseStatus)
	}
	if !evt1.Attendees[1].IsOptional {
		t.Error("expected second attendee to be optional")
	}

	// Check second event (all-day event)
	evt2 := calEvents[1]
	if evt2.Title != "All Day Planning" {
		t.Errorf("expected title 'All Day Planning', got %s", evt2.Title)
	}
	if !evt2.IsAllDay {
		t.Error("expected all-day event")
	}

	// Check third event (recurring with conference)
	evt3 := calEvents[2]
	if !evt3.IsRecurring {
		t.Error("expected recurring event")
	}
	if evt3.RecurrenceRule == "" {
		t.Error("expected recurrence rule")
	}
	if evt3.ConferenceURL != "https://zoom.us/j/123456" {
		t.Errorf("expected zoom link, got %s", evt3.ConferenceURL)
	}
}

func TestRealAdapter_FetchUpcomingCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/calendar/v3/calendars/primary/events" && r.Method == "GET" {
			resp := gcalListResponse{
				Items: []gcalEvent{
					{ID: "event-1", Start: gcalDateTime{DateTime: "2024-01-16T09:00:00Z"}},
					{ID: "event-2", Start: gcalDateTime{DateTime: "2024-01-17T09:00:00Z"}},
					{ID: "event-3", Start: gcalDateTime{DateTime: "2024-01-18T09:00:00Z"}},
					{ID: "event-4", Start: gcalDateTime{DateTime: "2024-01-19T09:00:00Z"}},
					{ID: "event-5", Start: gcalDateTime{DateTime: "2024-01-20T09:00:00Z"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	minter := &mockTokenMinter{
		token: auth.AccessToken{
			Token:    "test-token",
			Expiry:   time.Now().Add(time.Hour),
			Provider: auth.ProviderGoogle,
		},
	}

	client := &http.Client{
		Transport: &testTransport{server: server},
	}

	fixedClock := clock.NewFixed(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	adapter := NewRealAdapterWithClient(minter, client, fixedClock, "test-circle")

	count, err := adapter.FetchUpcomingCount("primary", 7)
	if err != nil {
		t.Fatalf("FetchUpcomingCount failed: %v", err)
	}

	if count != 5 {
		t.Errorf("expected 5 events, got %d", count)
	}
}

func TestRealAdapter_TokenError(t *testing.T) {
	minter := &mockTokenMinter{
		err: auth.ErrNoToken,
	}

	fixedClock := clock.NewFixed(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	adapter := NewRealAdapterWithClient(minter, http.DefaultClient, fixedClock, "test-circle")

	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	_, err := adapter.FetchEvents("primary", from, to)
	if err == nil {
		t.Error("expected error when token minting fails")
	}
}

func TestParseGcalTime(t *testing.T) {
	tests := []struct {
		name     string
		dt       gcalDateTime
		wantTime time.Time
		isAllDay bool
	}{
		{
			name: "timed event",
			dt: gcalDateTime{
				DateTime: "2024-01-15T09:00:00Z",
			},
			wantTime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			isAllDay: false,
		},
		{
			name: "all-day event",
			dt: gcalDateTime{
				Date: "2024-01-15",
			},
			wantTime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			isAllDay: true,
		},
		{
			name:     "empty datetime",
			dt:       gcalDateTime{},
			wantTime: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGcalTime(tt.dt)
			if !got.Equal(tt.wantTime) {
				t.Errorf("got %v, want %v", got, tt.wantTime)
			}
		})
	}
}

func TestMapResponseStatus(t *testing.T) {
	tests := []struct {
		status string
		want   events.RSVPStatus
	}{
		{"accepted", events.RSVPAccepted},
		{"declined", events.RSVPDeclined},
		{"tentative", events.RSVPTentative},
		{"needsAction", events.RSVPNeedsAction},
		{"unknown", events.RSVPNeedsAction},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := mapResponseStatus(tt.status)
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRealAdapter_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal error"}}`))
	}))
	defer server.Close()

	minter := &mockTokenMinter{
		token: auth.AccessToken{
			Token:    "test-token",
			Expiry:   time.Now().Add(time.Hour),
			Provider: auth.ProviderGoogle,
		},
	}

	client := &http.Client{
		Transport: &testTransport{server: server},
	}

	fixedClock := clock.NewFixed(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	adapter := NewRealAdapterWithClient(minter, client, fixedClock, "test-circle")

	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	_, err := adapter.FetchEvents("primary", from, to)
	if err == nil {
		t.Error("expected error on API error response")
	}
}

// testTransport redirects requests to the test server.
type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to test server
	req.URL.Scheme = "http"
	req.URL.Host = t.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}
