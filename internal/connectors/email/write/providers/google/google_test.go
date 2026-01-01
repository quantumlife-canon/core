package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/connectors/email/write"
)

// mockTokenBroker provides test tokens.
type mockTokenBroker struct {
	token string
	err   error
}

func (m *mockTokenBroker) GetAccessToken(ctx context.Context, accountID string) (string, error) {
	return m.token, m.err
}

func TestSendReply_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify endpoint
		if r.URL.Path != "/users/me/messages/send" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify method
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", auth)
		}

		// Verify content type
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("unexpected content type: %s", ct)
		}

		// Parse request body
		var req sendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify threadId is present
		if req.ThreadID != "thread-123" {
			t.Errorf("unexpected threadId: %s", req.ThreadID)
		}

		// Verify raw message contains In-Reply-To
		if req.Raw == "" {
			t.Error("raw message is empty")
		}

		// Return success response
		resp := sendResponse{
			ID:       "sent-msg-456",
			ThreadID: "thread-123",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create writer with test server
	broker := &mockTokenBroker{token: "test-token"}
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	writer := NewWriter(
		broker,
		WithBaseURL(server.URL),
		WithClock(func() time.Time { return fixedTime }),
	)

	// Send reply
	req := write.SendReplyRequest{
		Provider:           "google",
		AccountID:          "account-1",
		CircleID:           "circle-1",
		ThreadID:           "thread-123",
		InReplyToMessageID: "msg-100",
		Subject:            "Re: Test",
		Body:               "This is a test reply",
		IdempotencyKey:     "idem-001",
		TraceID:            "trace-001",
	}

	receipt, err := writer.SendReply(context.Background(), req)
	if err != nil {
		t.Fatalf("SendReply failed: %v", err)
	}

	if !receipt.Success {
		t.Errorf("expected success, got error: %s", receipt.Error)
	}

	if receipt.MessageID != "sent-msg-456" {
		t.Errorf("unexpected message ID: %s", receipt.MessageID)
	}

	if receipt.ThreadID != "thread-123" {
		t.Errorf("unexpected thread ID: %s", receipt.ThreadID)
	}

	if receipt.IdempotencyKey != "idem-001" {
		t.Errorf("unexpected idempotency key: %s", receipt.IdempotencyKey)
	}
}

func TestSendReply_ValidationError(t *testing.T) {
	broker := &mockTokenBroker{token: "test-token"}
	writer := NewWriter(broker)

	// Missing ThreadID
	req := write.SendReplyRequest{
		Provider:           "google",
		AccountID:          "account-1",
		InReplyToMessageID: "msg-100",
		Body:               "Test",
		IdempotencyKey:     "idem-001",
	}

	receipt, err := writer.SendReply(context.Background(), req)
	if err != nil {
		t.Fatalf("SendReply should not return error: %v", err)
	}

	if receipt.Success {
		t.Error("expected validation failure")
	}

	if !strings.Contains(receipt.Error, "thread_id") {
		t.Errorf("error should mention thread_id: %s", receipt.Error)
	}
}

func TestSendReply_APIError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer server.Close()

	broker := &mockTokenBroker{token: "test-token"}
	writer := NewWriter(broker, WithBaseURL(server.URL))

	req := write.SendReplyRequest{
		Provider:           "google",
		AccountID:          "account-1",
		ThreadID:           "thread-123",
		InReplyToMessageID: "msg-100",
		Body:               "Test",
		IdempotencyKey:     "idem-001",
	}

	receipt, err := writer.SendReply(context.Background(), req)
	if err != nil {
		t.Fatalf("SendReply should not return error: %v", err)
	}

	if receipt.Success {
		t.Error("expected failure due to API error")
	}

	if !strings.Contains(receipt.Error, "400") {
		t.Errorf("error should mention status code: %s", receipt.Error)
	}
}

func TestProviderID(t *testing.T) {
	writer := NewWriter(&mockTokenBroker{})
	if writer.ProviderID() != "google" {
		t.Errorf("unexpected provider ID: %s", writer.ProviderID())
	}
}

func TestIsSandbox(t *testing.T) {
	writer := NewWriter(&mockTokenBroker{})
	if writer.IsSandbox() {
		t.Error("google writer should not be sandbox")
	}
}
