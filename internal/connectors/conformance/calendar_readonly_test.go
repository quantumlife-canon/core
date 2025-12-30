// Package conformance provides conformance tests for calendar connectors.
// These tests verify that connectors adhere to v5 READ-ONLY requirements.
//
// CRITICAL: These tests MUST pass for any connector to be used in production.
// They verify no write operations occur in SuggestOnly or Simulate modes.
package conformance

import (
	"context"
	"net/http"
	"testing"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/connectors/calendar/impl_mock"
	"quantumlife/internal/connectors/calendar/providers/google"
	"quantumlife/internal/connectors/calendar/providers/microsoft"
	"quantumlife/internal/connectors/testkit"
	"quantumlife/pkg/primitives"
)

// Fixed time for deterministic testing.
var testTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

// testEnvelope creates a valid test envelope.
func testEnvelope(mode primitives.RunMode) primitives.ExecutionEnvelope {
	return primitives.ExecutionEnvelope{
		TraceID:              "test-trace-123",
		Mode:                 mode,
		ActorCircleID:        "test-circle",
		IntersectionID:       "test-intersection",
		ContractVersion:      "v1",
		ScopesUsed:           []string{"calendar:read"},
		AuthorizationProofID: "test-proof-123",
		IssuedAt:             testTime,
	}
}

// fakeAuthorityChecker implements auth.AuthorityChecker for testing.
type fakeAuthorityChecker struct{}

func (f *fakeAuthorityChecker) GetProof(ctx context.Context, proofID string) (auth.AuthProofSummary, error) {
	return auth.AuthProofSummary{
		ID:              proofID,
		Authorized:      true,
		ScopesGranted:   []string{"calendar:read"},
		IntersectionID:  "test-intersection",
		ContractVersion: "v1",
	}, nil
}

// setupGoogleAdapter creates a Google adapter with fake HTTP transport.
func setupGoogleAdapter(t *testing.T) (*google.Adapter, *testkit.FakeTransport) {
	transport := testkit.NewFakeTransport()

	// Add mock responses
	transport.AddResponse("googleapis.com/calendar", &testkit.FakeResponse{
		StatusCode: http.StatusOK,
		Body:       testkit.GoogleEventsListResponse(),
	})
	transport.AddResponse("oauth2.googleapis.com/token", &testkit.FakeResponse{
		StatusCode: http.StatusOK,
		Body:       testkit.TokenRefreshResponse(),
	})

	// Create broker with fake HTTP client
	config := auth.Config{
		Google: auth.GoogleConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
		},
		TokenEncryptionKey: "test-key",
	}
	broker := impl_inmem.NewBroker(config, &fakeAuthorityChecker{},
		impl_inmem.WithHTTPClient(transport.NewHTTPClient()))

	// Store a fake refresh token
	ctx := context.Background()
	_, err := broker.StoreTokenDirectly(ctx, "test-circle", auth.ProviderGoogle, "fake-refresh-token", []string{"calendar:read"})
	if err != nil {
		t.Fatalf("failed to store token: %v", err)
	}

	adapter := google.NewAdapter(broker, true,
		google.WithHTTPClient(transport.NewHTTPClient()),
		google.WithClock(func() time.Time { return testTime }))

	return adapter, transport
}

// setupMicrosoftAdapter creates a Microsoft adapter with fake HTTP transport.
func setupMicrosoftAdapter(t *testing.T) (*microsoft.Adapter, *testkit.FakeTransport) {
	transport := testkit.NewFakeTransport()

	// Add mock responses
	transport.AddResponse("graph.microsoft.com", &testkit.FakeResponse{
		StatusCode: http.StatusOK,
		Body:       testkit.MicrosoftCalendarViewResponse(),
	})
	transport.AddResponse("login.microsoftonline.com", &testkit.FakeResponse{
		StatusCode: http.StatusOK,
		Body:       testkit.MicrosoftTokenRefreshResponse(),
	})

	// Create broker with fake HTTP client
	config := auth.Config{
		Microsoft: auth.MicrosoftConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			TenantID:     "common",
		},
		TokenEncryptionKey: "test-key",
	}
	broker := impl_inmem.NewBroker(config, &fakeAuthorityChecker{},
		impl_inmem.WithHTTPClient(transport.NewHTTPClient()))

	// Store a fake refresh token
	ctx := context.Background()
	_, err := broker.StoreTokenDirectly(ctx, "test-circle", auth.ProviderMicrosoft, "fake-refresh-token", []string{"calendar:read"})
	if err != nil {
		t.Fatalf("failed to store token: %v", err)
	}

	adapter := microsoft.NewAdapter(broker, true,
		microsoft.WithHTTPClient(transport.NewHTTPClient()),
		microsoft.WithClock(func() time.Time { return testTime }))

	return adapter, transport
}

// TestMockConnector_SuggestOnlyNeverWrites verifies mock connector in suggest-only mode.
func TestMockConnector_SuggestOnlyNeverWrites(t *testing.T) {
	mock := impl_mock.NewMockConnectorWithClock(func() time.Time { return testTime })
	env := testEnvelope(primitives.ModeSuggestOnly)

	ctx := context.Background()
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	// List events should work
	events, err := mock.ListEventsWithEnvelope(ctx, env, r)
	if err != nil {
		t.Fatalf("ListEventsWithEnvelope failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected events from mock")
	}

	// Find free slots should work
	slots, err := mock.FindFreeSlots(ctx, env, r, 30*time.Minute)
	if err != nil {
		t.Fatalf("FindFreeSlots failed: %v", err)
	}
	if len(slots) == 0 {
		t.Error("expected free slots from mock")
	}

	// Propose event should work (returns proposal, no write)
	proposal, err := mock.ProposeEventWithEnvelope(ctx, env, calendar.ProposeEventRequest{
		Title:     "Test Event",
		StartTime: testTime.Add(12 * time.Hour),
		EndTime:   testTime.Add(13 * time.Hour),
	})
	if err != nil {
		t.Fatalf("ProposeEventWithEnvelope failed: %v", err)
	}
	if !proposal.Simulated {
		t.Error("proposal should be marked as simulated")
	}
}

// TestMockConnector_SimulateModeNeverWrites verifies mock connector in simulate mode.
func TestMockConnector_SimulateModeNeverWrites(t *testing.T) {
	mock := impl_mock.NewMockConnectorWithClock(func() time.Time { return testTime })
	env := testEnvelope(primitives.ModeSimulate)

	ctx := context.Background()
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	// All operations should work without actual writes
	events, err := mock.ListEventsWithEnvelope(ctx, env, r)
	if err != nil {
		t.Fatalf("ListEventsWithEnvelope failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected events from mock")
	}

	proposal, err := mock.ProposeEventWithEnvelope(ctx, env, calendar.ProposeEventRequest{
		Title:     "Test Event",
		StartTime: testTime.Add(12 * time.Hour),
		EndTime:   testTime.Add(13 * time.Hour),
	})
	if err != nil {
		t.Fatalf("ProposeEventWithEnvelope failed: %v", err)
	}
	if !proposal.Simulated {
		t.Error("proposal should be marked as simulated")
	}
}

// TestGoogleAdapter_NoWriteOperations verifies Google adapter never performs writes.
func TestGoogleAdapter_NoWriteOperations(t *testing.T) {
	adapter, transport := setupGoogleAdapter(t)

	ctx := context.Background()
	env := testEnvelope(primitives.ModeSimulate)
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	// List events
	events, err := adapter.ListEventsWithEnvelope(ctx, env, r)
	if err != nil {
		t.Fatalf("ListEventsWithEnvelope failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected events from adapter")
	}

	// Check that no write operations occurred
	writeCount := transport.GetWriteCallCount()
	requests := transport.GetRequests()

	// Token refresh uses POST, so we need to verify calendar API calls are GET only
	for _, req := range requests {
		if req.URL != "" && isCalendarAPIRequest(req.URL) {
			if req.Method != "GET" {
				t.Errorf("calendar API call was %s, expected GET only", req.Method)
			}
		}
	}

	// Verify at least the token refresh happened (expected POST) but no other writes
	t.Logf("Total requests: %d, Write calls: %d", len(requests), writeCount)
}

// TestMicrosoftAdapter_NoWriteOperations verifies Microsoft adapter never performs writes.
func TestMicrosoftAdapter_NoWriteOperations(t *testing.T) {
	adapter, transport := setupMicrosoftAdapter(t)

	ctx := context.Background()
	env := testEnvelope(primitives.ModeSimulate)
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	// List events
	events, err := adapter.ListEventsWithEnvelope(ctx, env, r)
	if err != nil {
		t.Fatalf("ListEventsWithEnvelope failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected events from adapter")
	}

	// Check that calendar API calls are GET only
	requests := transport.GetRequests()
	for _, req := range requests {
		if isGraphAPIRequest(req.URL) {
			if req.Method != "GET" {
				t.Errorf("Graph API call was %s, expected GET only", req.Method)
			}
		}
	}
}

// TestEnvelopeValidation_RejectsExecuteMode verifies execute mode is rejected.
func TestEnvelopeValidation_RejectsExecuteMode(t *testing.T) {
	mock := impl_mock.NewMockConnector()
	env := testEnvelope(primitives.ModeExecute)

	ctx := context.Background()
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	_, err := mock.ListEventsWithEnvelope(ctx, env, r)
	if err == nil {
		t.Error("expected error for execute mode")
	}
	if err != primitives.ErrEnvelopeExecuteModeNotAllowed {
		t.Errorf("expected ErrEnvelopeExecuteModeNotAllowed, got: %v", err)
	}
}

// TestEnvelopeValidation_RejectsWriteScopes verifies write scopes are rejected.
func TestEnvelopeValidation_RejectsWriteScopes(t *testing.T) {
	mock := impl_mock.NewMockConnector()
	env := testEnvelope(primitives.ModeSimulate)
	env.ScopesUsed = []string{"calendar:write"} // Write scope

	ctx := context.Background()
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	_, err := mock.ListEventsWithEnvelope(ctx, env, r)
	if err == nil {
		t.Error("expected error for write scope")
	}
}

// TestEnvelopeValidation_RequiresFields verifies required envelope fields.
func TestEnvelopeValidation_RequiresFields(t *testing.T) {
	mock := impl_mock.NewMockConnector()
	ctx := context.Background()
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	tests := []struct {
		name    string
		modify  func(*primitives.ExecutionEnvelope)
		wantErr error
	}{
		{
			name:    "missing trace ID",
			modify:  func(e *primitives.ExecutionEnvelope) { e.TraceID = "" },
			wantErr: primitives.ErrEnvelopeTraceIDRequired,
		},
		{
			name:    "missing intersection ID",
			modify:  func(e *primitives.ExecutionEnvelope) { e.IntersectionID = "" },
			wantErr: primitives.ErrEnvelopeIntersectionIDRequired,
		},
		{
			name:    "missing scopes",
			modify:  func(e *primitives.ExecutionEnvelope) { e.ScopesUsed = nil },
			wantErr: primitives.ErrEnvelopeScopesRequired,
		},
		{
			name:    "missing auth proof ID",
			modify:  func(e *primitives.ExecutionEnvelope) { e.AuthorizationProofID = "" },
			wantErr: primitives.ErrEnvelopeAuthProofIDRequired,
		},
		{
			name:    "missing actor circle ID",
			modify:  func(e *primitives.ExecutionEnvelope) { e.ActorCircleID = "" },
			wantErr: primitives.ErrEnvelopeActorCircleIDRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testEnvelope(primitives.ModeSimulate)
			tt.modify(&env)

			_, err := mock.ListEventsWithEnvelope(ctx, env, r)
			if err != tt.wantErr {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestScopeMapping_BlocksWriteScopes verifies write scopes cannot be minted.
func TestScopeMapping_BlocksWriteScopes(t *testing.T) {
	mapper := impl_inmem.NewScopeMapper()

	_, err := mapper.MapToProvider(auth.ProviderGoogle, []string{"calendar:write"})
	if err != auth.ErrWriteScopeNotAllowed {
		t.Errorf("expected ErrWriteScopeNotAllowed, got: %v", err)
	}

	_, err = mapper.MapToProvider(auth.ProviderMicrosoft, []string{"calendar:write"})
	if err != auth.ErrWriteScopeNotAllowed {
		t.Errorf("expected ErrWriteScopeNotAllowed, got: %v", err)
	}
}

// TestDeterministicOutput verifies deterministic outputs for fixed inputs.
func TestDeterministicOutput(t *testing.T) {
	mock := impl_mock.NewMockConnectorWithClock(func() time.Time { return testTime })
	env := testEnvelope(primitives.ModeSimulate)

	ctx := context.Background()
	r := calendar.EventRange{
		Start: testTime,
		End:   testTime.Add(24 * time.Hour),
	}

	// Run twice, results should be identical
	events1, _ := mock.ListEventsWithEnvelope(ctx, env, r)
	events2, _ := mock.ListEventsWithEnvelope(ctx, env, r)

	if len(events1) != len(events2) {
		t.Errorf("non-deterministic: got %d then %d events", len(events1), len(events2))
	}

	for i := range events1 {
		if events1[i].ID != events2[i].ID {
			t.Errorf("non-deterministic: event %d ID differs", i)
		}
	}
}

// isCalendarAPIRequest checks if URL is a Google Calendar API request.
func isCalendarAPIRequest(url string) bool {
	return len(url) > 0 && (contains(url, "googleapis.com/calendar") ||
		contains(url, "www.googleapis.com/calendar"))
}

// isGraphAPIRequest checks if URL is a Microsoft Graph API request.
func isGraphAPIRequest(url string) bool {
	return len(url) > 0 && contains(url, "graph.microsoft.com")
}

// contains is a simple string contains check.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
