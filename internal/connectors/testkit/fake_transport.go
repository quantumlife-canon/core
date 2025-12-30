// Package testkit provides testing utilities for connector tests.
// This includes fake HTTP transports for deterministic testing.
//
// CRITICAL: This is for testing only. Never use in production.
package testkit

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
)

// FakeTransport implements http.RoundTripper for testing.
// It returns predefined responses based on URL patterns.
type FakeTransport struct {
	mu         sync.Mutex
	responses  map[string]*FakeResponse
	requests   []RecordedRequest
	writeCalls int // Tracks POST/PUT/DELETE calls
}

// FakeResponse represents a fake HTTP response.
type FakeResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// RecordedRequest records details of an HTTP request for verification.
type RecordedRequest struct {
	Method  string
	URL     string
	Body    string
	Headers http.Header
}

// NewFakeTransport creates a new fake transport.
func NewFakeTransport() *FakeTransport {
	return &FakeTransport{
		responses: make(map[string]*FakeResponse),
	}
}

// AddResponse adds a fake response for a URL pattern.
func (t *FakeTransport) AddResponse(urlPattern string, resp *FakeResponse) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.responses[urlPattern] = resp
}

// RoundTrip implements http.RoundTripper.
func (t *FakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Record the request
	var bodyStr string
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		bodyStr = string(body)
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	t.requests = append(t.requests, RecordedRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Body:    bodyStr,
		Headers: req.Header.Clone(),
	})

	// Track write operations
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "DELETE" || req.Method == "PATCH" {
		t.writeCalls++
	}

	// Find matching response
	for pattern, resp := range t.responses {
		if strings.Contains(req.URL.String(), pattern) {
			header := http.Header{}
			header.Set("Content-Type", "application/json")
			for k, v := range resp.Headers {
				header.Set(k, v)
			}

			return &http.Response{
				StatusCode: resp.StatusCode,
				Body:       io.NopCloser(strings.NewReader(resp.Body)),
				Header:     header,
				Request:    req,
			}, nil
		}
	}

	// Default 404 response
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

// GetRequests returns all recorded requests.
func (t *FakeTransport) GetRequests() []RecordedRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]RecordedRequest, len(t.requests))
	copy(result, t.requests)
	return result
}

// GetWriteCallCount returns the number of write operations (POST/PUT/DELETE/PATCH).
func (t *FakeTransport) GetWriteCallCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writeCalls
}

// Reset clears all recorded requests and responses.
func (t *FakeTransport) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requests = nil
	t.writeCalls = 0
}

// NewHTTPClient creates an http.Client using this fake transport.
func (t *FakeTransport) NewHTTPClient() *http.Client {
	return &http.Client{Transport: t}
}

// Google Calendar fixtures.

// GoogleEventsListResponse returns a sample Google Calendar events.list response.
func GoogleEventsListResponse() string {
	return `{
  "kind": "calendar#events",
  "etag": "\"abc123\"",
  "summary": "test@example.com",
  "updated": "2025-01-15T10:00:00.000Z",
  "timeZone": "UTC",
  "items": [
    {
      "id": "google-evt-1",
      "summary": "Team Meeting",
      "description": "Weekly sync",
      "location": "Conference Room A",
      "start": {"dateTime": "2025-01-15T09:00:00Z"},
      "end": {"dateTime": "2025-01-15T10:00:00Z"},
      "attendees": [
        {"email": "alice@example.com"},
        {"email": "bob@example.com"}
      ]
    },
    {
      "id": "google-evt-2",
      "summary": "Lunch Break",
      "start": {"dateTime": "2025-01-15T12:00:00Z"},
      "end": {"dateTime": "2025-01-15T13:00:00Z"}
    },
    {
      "id": "google-evt-3",
      "summary": "Project Review",
      "description": "Q1 planning",
      "start": {"dateTime": "2025-01-15T14:00:00Z"},
      "end": {"dateTime": "2025-01-15T15:00:00Z"}
    }
  ]
}`
}

// Microsoft Graph fixtures.

// MicrosoftCalendarViewResponse returns a sample Microsoft Graph calendarView response.
func MicrosoftCalendarViewResponse() string {
	return `{
  "@odata.context": "https://graph.microsoft.com/v1.0/$metadata#users('test')/calendarView",
  "value": [
    {
      "id": "ms-evt-1",
      "subject": "Morning Standup",
      "bodyPreview": "Daily sync meeting",
      "location": {"displayName": "Teams Call"},
      "start": {"dateTime": "2025-01-15T09:30:00", "timeZone": "UTC"},
      "end": {"dateTime": "2025-01-15T09:45:00", "timeZone": "UTC"},
      "attendees": [
        {"emailAddress": {"address": "dev1@example.com", "name": "Developer 1"}},
        {"emailAddress": {"address": "dev2@example.com", "name": "Developer 2"}}
      ],
      "isAllDay": false,
      "showAs": "busy"
    },
    {
      "id": "ms-evt-2",
      "subject": "Focus Time",
      "bodyPreview": "Blocked for deep work",
      "location": {"displayName": ""},
      "start": {"dateTime": "2025-01-15T10:00:00", "timeZone": "UTC"},
      "end": {"dateTime": "2025-01-15T12:00:00", "timeZone": "UTC"},
      "attendees": [],
      "isAllDay": false,
      "showAs": "busy"
    },
    {
      "id": "ms-evt-3",
      "subject": "1:1 with Manager",
      "bodyPreview": "Weekly check-in",
      "location": {"displayName": "Office"},
      "start": {"dateTime": "2025-01-15T15:00:00", "timeZone": "UTC"},
      "end": {"dateTime": "2025-01-15T15:30:00", "timeZone": "UTC"},
      "attendees": [
        {"emailAddress": {"address": "manager@example.com", "name": "Manager"}}
      ],
      "isAllDay": false,
      "showAs": "busy"
    }
  ]
}`
}

// TokenRefreshResponse returns a sample OAuth token refresh response.
func TokenRefreshResponse() string {
	return `{
  "access_token": "fake-access-token-12345",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "https://www.googleapis.com/auth/calendar.readonly"
}`
}

// MicrosoftTokenRefreshResponse returns a sample Microsoft token refresh response.
func MicrosoftTokenRefreshResponse() string {
	return `{
  "access_token": "fake-ms-access-token-67890",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "Calendars.Read offline_access"
}`
}
