// Package gmail_read provides a read-only adapter for Gmail integration.
// This file contains httptest-based tests for the real adapter.
package gmail_read

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/pkg/clock"
)

// mockTokenMinter implements TokenMinter for testing.
type mockTokenMinter struct {
	token auth.AccessToken
	err   error
}

func (m *mockTokenMinter) MintReadOnlyAccessToken(ctx context.Context, circleID string, provider auth.ProviderID, requiredScopes []string) (auth.AccessToken, error) {
	return m.token, m.err
}

func TestRealAdapter_FetchMessages(t *testing.T) {
	// Create a test server that mimics Gmail API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", authHeader)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Handle different endpoints
		switch {
		case r.URL.Path == "/gmail/v1/users/me/messages" && r.Method == "GET":
			// Return list of messages
			resp := gmailListResponse{
				Messages: []gmailMessageRef{
					{ID: "msg-1", ThreadID: "thread-1"},
					{ID: "msg-2", ThreadID: "thread-2"},
				},
				ResultSizeEstimate: 2,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/gmail/v1/users/me/messages/msg-1" && r.Method == "GET":
			// Return message details
			resp := gmailMessage{
				ID:           "msg-1",
				ThreadID:     "thread-1",
				LabelIDs:     []string{"INBOX", "UNREAD"},
				Snippet:      "Hello, this is a test email...",
				InternalDate: 1704067200000, // 2024-01-01 00:00:00 UTC
				Payload: gmailPayload{
					Headers: []gmailHeader{
						{Name: "From", Value: "sender@example.com"},
						{Name: "To", Value: "recipient@example.com"},
						{Name: "Subject", Value: "Test Subject"},
						{Name: "Date", Value: "Mon, 01 Jan 2024 12:00:00 +0000"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/gmail/v1/users/me/messages/msg-2" && r.Method == "GET":
			// Return message details
			resp := gmailMessage{
				ID:           "msg-2",
				ThreadID:     "thread-2",
				LabelIDs:     []string{"INBOX", "STARRED", "IMPORTANT"},
				Snippet:      "This is an important message...",
				InternalDate: 1704153600000, // 2024-01-02 00:00:00 UTC
				Payload: gmailPayload{
					Headers: []gmailHeader{
						{Name: "From", Value: "Important Person <important@company.com>"},
						{Name: "To", Value: "me@example.com"},
						{Name: "Subject", Value: "Important Update"},
						{Name: "Date", Value: "Tue, 02 Jan 2024 12:00:00 +0000"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
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

	// Create custom HTTP client that redirects to test server
	client := &http.Client{
		Transport: &testTransport{
			server: server,
		},
	}

	fixedClock := clock.NewFixed(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	adapter := NewRealAdapterWithClient(minter, client, fixedClock, "test-circle")

	// Fetch messages
	messages, err := adapter.FetchMessages("test@example.com", time.Time{}, 10)
	if err != nil {
		t.Fatalf("FetchMessages failed: %v", err)
	}

	// Verify results
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Check first message
	msg1 := messages[0]
	if msg1.MessageID != "msg-1" {
		t.Errorf("expected message ID msg-1, got %s", msg1.MessageID)
	}
	if msg1.Subject != "Test Subject" {
		t.Errorf("expected subject 'Test Subject', got %s", msg1.Subject)
	}
	if msg1.From.Address != "sender@example.com" {
		t.Errorf("expected from sender@example.com, got %s", msg1.From.Address)
	}
	if msg1.IsRead {
		t.Error("expected message to be unread")
	}

	// Check second message
	msg2 := messages[1]
	if msg2.MessageID != "msg-2" {
		t.Errorf("expected message ID msg-2, got %s", msg2.MessageID)
	}
	if !msg2.IsStarred {
		t.Error("expected message to be starred")
	}
	if !msg2.IsImportant {
		t.Error("expected message to be important")
	}
	if msg2.From.Name != "Important Person" {
		t.Errorf("expected from name 'Important Person', got %s", msg2.From.Name)
	}
}

func TestRealAdapter_FetchUnreadCount(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gmail/v1/users/me/messages" && r.Method == "GET" {
			// Verify query includes is:unread
			q := r.URL.Query().Get("q")
			if q != "in:inbox is:unread" {
				t.Errorf("expected query 'in:inbox is:unread', got %s", q)
			}

			resp := gmailListResponse{
				Messages: []gmailMessageRef{
					{ID: "msg-1"},
					{ID: "msg-2"},
					{ID: "msg-3"},
				},
				ResultSizeEstimate: 3,
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

	count, err := adapter.FetchUnreadCount("test@example.com")
	if err != nil {
		t.Fatalf("FetchUnreadCount failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 unread, got %d", count)
	}
}

func TestRealAdapter_TokenError(t *testing.T) {
	minter := &mockTokenMinter{
		err: auth.ErrNoToken,
	}

	fixedClock := clock.NewFixed(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	adapter := NewRealAdapterWithClient(minter, http.DefaultClient, fixedClock, "test-circle")

	_, err := adapter.FetchMessages("test@example.com", time.Time{}, 10)
	if err == nil {
		t.Error("expected error when token minting fails")
	}
}

func TestParseEmailAddress(t *testing.T) {
	tests := []struct {
		input       string
		wantAddress string
		wantName    string
	}{
		{
			input:       "test@example.com",
			wantAddress: "test@example.com",
			wantName:    "",
		},
		{
			input:       "John Doe <john@example.com>",
			wantAddress: "john@example.com",
			wantName:    "John Doe",
		},
		{
			input:       "<noreply@example.com>",
			wantAddress: "noreply@example.com",
			wantName:    "",
		},
		{
			input:       "  spaces@example.com  ",
			wantAddress: "spaces@example.com",
			wantName:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			addr := parseEmailAddress(tt.input)
			if addr.Address != tt.wantAddress {
				t.Errorf("address: got %s, want %s", addr.Address, tt.wantAddress)
			}
			if addr.Name != tt.wantName {
				t.Errorf("name: got %s, want %s", addr.Name, tt.wantName)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		email  string
		domain string
	}{
		{"user@example.com", "example.com"},
		{"user@sub.example.com", "sub.example.com"},
		{"noemail", ""},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := extractDomain(tt.email)
			if got != tt.domain {
				t.Errorf("got %s, want %s", got, tt.domain)
			}
		})
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
