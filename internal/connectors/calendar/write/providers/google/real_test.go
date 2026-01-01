package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/connectors/calendar/write"
)

// mockTokenProvider implements TokenProvider for testing.
type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

// fixedClock returns a fixed time for deterministic tests.
func fixedClock() time.Time {
	return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
}

func TestRespondToEvent_Accept(t *testing.T) {
	// Setup fake server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			// Return user info
			json.NewEncoder(w).Encode(map[string]string{
				"email": "user@example.com",
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events/"):
			// Return event with attendees
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:   "event-123",
				ETag: "etag-before",
				Attendees: []GoogleAttendee{
					{Email: "user@example.com", ResponseStatus: "needsAction"},
					{Email: "other@example.com", ResponseStatus: "accepted"},
				},
			})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/events/"):
			// Return patched event
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:   "event-123",
				ETag: "etag-after",
				Attendees: []GoogleAttendee{
					{Email: "user@example.com", ResponseStatus: "accepted"},
					{Email: "other@example.com", ResponseStatus: "accepted"},
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create writer with test server
	tokenProvider := &mockTokenProvider{token: "test-token"}
	writer := NewWriter(
		tokenProvider,
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	// Test accept
	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "primary",
		EventID:        "event-123",
		ResponseStatus: write.ResponseAccepted,
		Message:        "I will attend",
		IdempotencyKey: "idem-key-1",
	}

	receipt, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receipt.Success {
		t.Errorf("expected success, got failure: %s", receipt.Error)
	}
	if receipt.EventID != "event-123" {
		t.Errorf("expected EventID=event-123, got %s", receipt.EventID)
	}
	if receipt.ETag != "etag-after" {
		t.Errorf("expected ETag=etag-after, got %s", receipt.ETag)
	}
	if receipt.IdempotencyKey != "idem-key-1" {
		t.Errorf("expected IdempotencyKey=idem-key-1, got %s", receipt.IdempotencyKey)
	}
}

func TestRespondToEvent_Decline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			json.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:        "event-456",
				Attendees: []GoogleAttendee{{Email: "user@example.com", ResponseStatus: "needsAction"}},
			})
		case r.Method == http.MethodPatch:
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:        "event-456",
				ETag:      "new-etag",
				Attendees: []GoogleAttendee{{Email: "user@example.com", ResponseStatus: "declined"}},
			})
		}
	}))
	defer server.Close()

	writer := NewWriter(
		&mockTokenProvider{token: "test-token"},
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "primary",
		EventID:        "event-456",
		ResponseStatus: write.ResponseDeclined,
		Message:        "Cannot attend",
		IdempotencyKey: "idem-decline-1",
	}

	receipt, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receipt.Success {
		t.Errorf("expected success, got failure: %s", receipt.Error)
	}
}

func TestRespondToEvent_Tentative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			json.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:        "event-789",
				Attendees: []GoogleAttendee{{Email: "user@example.com"}},
			})
		case r.Method == http.MethodPatch:
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:        "event-789",
				ETag:      "tentative-etag",
				Attendees: []GoogleAttendee{{Email: "user@example.com", ResponseStatus: "tentative"}},
			})
		}
	}))
	defer server.Close()

	writer := NewWriter(
		&mockTokenProvider{token: "test-token"},
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "work",
		EventID:        "event-789",
		ResponseStatus: write.ResponseTentative,
		IdempotencyKey: "idem-tentative-1",
	}

	receipt, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receipt.Success {
		t.Errorf("expected success, got failure: %s", receipt.Error)
	}
}

func TestRespondToEvent_ProposeNewTime(t *testing.T) {
	var patchedDescription string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			json.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:          "event-propose",
				Description: "Original description",
				Attendees:   []GoogleAttendee{{Email: "user@example.com"}},
			})
		case r.Method == http.MethodPatch:
			// Capture the patch body
			var patchData map[string]interface{}
			json.NewDecoder(r.Body).Decode(&patchData)
			if desc, ok := patchData["description"].(string); ok {
				patchedDescription = desc
			}

			json.NewEncoder(w).Encode(GoogleEvent{
				ID:          "event-propose",
				ETag:        "proposed-etag",
				Description: patchedDescription,
				Attendees:   []GoogleAttendee{{Email: "user@example.com", ResponseStatus: "tentative"}},
			})
		}
	}))
	defer server.Close()

	writer := NewWriter(
		&mockTokenProvider{token: "test-token"},
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	proposedStart := time.Date(2024, 1, 20, 14, 0, 0, 0, time.UTC)
	proposedEnd := time.Date(2024, 1, 20, 15, 0, 0, 0, time.UTC)

	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "primary",
		EventID:        "event-propose",
		ResponseStatus: write.ResponseTentative,
		Message:        "Can we move this?",
		ProposeNewTime: true,
		ProposedStart:  &proposedStart,
		ProposedEnd:    &proposedEnd,
		IdempotencyKey: "idem-propose-1",
	}

	receipt, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receipt.Success {
		t.Errorf("expected success, got failure: %s", receipt.Error)
	}

	// Verify description was updated with proposal
	if !strings.Contains(patchedDescription, "Time Proposal") {
		t.Errorf("expected proposal note in description, got: %s", patchedDescription)
	}
	if !strings.Contains(patchedDescription, "Original description") {
		t.Errorf("expected original description preserved, got: %s", patchedDescription)
	}
}

func TestRespondToEvent_Idempotency(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			json.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:        "event-idem",
				Attendees: []GoogleAttendee{{Email: "user@example.com"}},
			})
		case r.Method == http.MethodPatch:
			callCount++
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:   "event-idem",
				ETag: "idem-etag",
			})
		}
	}))
	defer server.Close()

	writer := NewWriter(
		&mockTokenProvider{token: "test-token"},
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "primary",
		EventID:        "event-idem",
		ResponseStatus: write.ResponseAccepted,
		IdempotencyKey: "same-key",
	}

	// First call
	receipt1, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call with same idempotency key
	receipt2, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Should have same result
	if receipt1.ProviderResponseID != receipt2.ProviderResponseID {
		t.Errorf("idempotency failed: got different response IDs")
	}

	// Should only call API once
	if callCount != 1 {
		t.Errorf("expected 1 PATCH call, got %d", callCount)
	}
}

func TestRespondToEvent_UserNotInAttendees(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			json.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
		case r.Method == http.MethodGet:
			// Return event WITHOUT the current user in attendees
			json.NewEncoder(w).Encode(GoogleEvent{
				ID:        "event-no-user",
				Attendees: []GoogleAttendee{{Email: "other@example.com"}},
			})
		}
	}))
	defer server.Close()

	writer := NewWriter(
		&mockTokenProvider{token: "test-token"},
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "primary",
		EventID:        "event-no-user",
		ResponseStatus: write.ResponseAccepted,
		IdempotencyKey: "idem-no-user",
	}

	receipt, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receipt.Success {
		t.Error("expected failure when user not in attendees")
	}
	if !strings.Contains(receipt.Error, "not found in event attendees") {
		t.Errorf("expected 'not found' error, got: %s", receipt.Error)
	}
}

func TestRespondToEvent_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/userinfo"):
			json.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
		case r.Method == http.MethodGet:
			http.Error(w, "event not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	writer := NewWriter(
		&mockTokenProvider{token: "test-token"},
		WithBaseURL(server.URL),
		WithClock(fixedClock),
	)

	input := write.RespondInput{
		Provider:       "google",
		CalendarID:     "primary",
		EventID:        "nonexistent",
		ResponseStatus: write.ResponseAccepted,
		IdempotencyKey: "idem-error",
	}

	receipt, err := writer.RespondToEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receipt.Success {
		t.Error("expected failure on API error")
	}
	if !strings.Contains(receipt.Error, "failed to get event") {
		t.Errorf("expected 'failed to get event' error, got: %s", receipt.Error)
	}
}

func TestRespondToEvent_ValidationError(t *testing.T) {
	writer := NewWriter(&mockTokenProvider{token: "test-token"})

	// Missing provider
	input := write.RespondInput{
		CalendarID:     "primary",
		EventID:        "event-123",
		ResponseStatus: write.ResponseAccepted,
		IdempotencyKey: "idem-1",
	}

	_, err := writer.RespondToEvent(context.Background(), input)
	if err == nil {
		t.Error("expected validation error for missing provider")
	}
}

func TestProviderID(t *testing.T) {
	writer := NewWriter(&mockTokenProvider{token: "test"})
	if writer.ProviderID() != "google" {
		t.Errorf("expected provider ID 'google', got '%s'", writer.ProviderID())
	}
}

func TestIsSandbox(t *testing.T) {
	// Default is not sandbox
	writer := NewWriter(&mockTokenProvider{token: "test"})
	if writer.IsSandbox() {
		t.Error("expected not sandbox by default")
	}

	// With sandbox option
	sandboxWriter := NewWriter(&mockTokenProvider{token: "test"}, WithSandbox(true))
	if !sandboxWriter.IsSandbox() {
		t.Error("expected sandbox when WithSandbox(true)")
	}
}

func TestDeterministicResponseID(t *testing.T) {
	writer := NewWriter(&mockTokenProvider{token: "test"})

	input1 := write.RespondInput{
		CalendarID:     "cal-1",
		EventID:        "event-1",
		ResponseStatus: write.ResponseAccepted,
		IdempotencyKey: "key-1",
	}

	input2 := write.RespondInput{
		CalendarID:     "cal-1",
		EventID:        "event-1",
		ResponseStatus: write.ResponseAccepted,
		IdempotencyKey: "key-1",
	}

	id1 := writer.generateResponseID(input1)
	id2 := writer.generateResponseID(input2)

	if id1 != id2 {
		t.Errorf("expected deterministic response IDs to match: %s != %s", id1, id2)
	}

	// Different input should give different ID
	input3 := write.RespondInput{
		CalendarID:     "cal-1",
		EventID:        "event-1",
		ResponseStatus: write.ResponseDeclined, // Different response
		IdempotencyKey: "key-1",
	}

	id3 := writer.generateResponseID(input3)
	if id1 == id3 {
		t.Error("expected different response IDs for different inputs")
	}
}
