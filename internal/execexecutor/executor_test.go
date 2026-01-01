package execexecutor

import (
	"context"
	"testing"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	emailexec "quantumlife/internal/email/execution"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/execintent"
	"quantumlife/pkg/events"
)

type mockEmitter struct {
	events []events.Event
}

func (m *mockEmitter) Emit(e events.Event) {
	m.events = append(m.events, e)
}

type mockEmailExecutor struct {
	result   *emailexec.Envelope
	err      error
	called   bool
	envelope emailexec.Envelope
}

func (m *mockEmailExecutor) Execute(ctx context.Context, envelope emailexec.Envelope) (*emailexec.Envelope, error) {
	m.called = true
	m.envelope = envelope
	return m.result, m.err
}

type mockCalendarExecutor struct {
	result   calexec.ExecuteResult
	called   bool
	envelope *calexec.Envelope
}

func (m *mockCalendarExecutor) Execute(ctx context.Context, envelope *calexec.Envelope) calexec.ExecuteResult {
	m.called = true
	m.envelope = envelope
	return m.result
}

func TestExecutor_ExecuteIntent_EmailSuccess(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	// Create successful email executor mock
	emailMock := &mockEmailExecutor{
		result: &emailexec.Envelope{
			EnvelopeID: "env-001",
			DraftID:    "draft-001",
			Status:     emailexec.EnvelopeStatusExecuted,
			ExecutionResult: &emailexec.ExecutionResult{
				Success:            true,
				MessageID:          "msg-001",
				ProviderResponseID: "prov-resp-001",
			},
		},
	}

	executor := NewExecutor(clk, emitter).
		WithEmailExecutor(emailMock)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-001",
		DraftID:            "draft-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-000",
		EmailTo:            "test@example.com",
		EmailSubject:       "Test Subject",
		EmailBody:          "Test Body",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if !outcome.Success {
		t.Errorf("expected success, got error: %s", outcome.Error)
	}
	if outcome.Blocked {
		t.Errorf("unexpected block: %s", outcome.BlockedReason)
	}
	if !emailMock.called {
		t.Error("email executor should have been called")
	}
	if outcome.ProviderResponseID != "prov-resp-001" {
		t.Errorf("ProviderResponseID = %s, want prov-resp-001", outcome.ProviderResponseID)
	}

	// Check events
	hasRequestedEvent := false
	hasRoutedEvent := false
	hasSucceededEvent := false
	for _, e := range emitter.events {
		switch e.Type {
		case events.Phase10ExecutionRequested:
			hasRequestedEvent = true
		case events.Phase10ExecutionRouted:
			hasRoutedEvent = true
		case events.Phase10ExecutionSucceeded:
			hasSucceededEvent = true
		}
	}
	if !hasRequestedEvent {
		t.Error("expected Phase10ExecutionRequested event")
	}
	if !hasRoutedEvent {
		t.Error("expected Phase10ExecutionRouted event")
	}
	if !hasSucceededEvent {
		t.Error("expected Phase10ExecutionSucceeded event")
	}
}

func TestExecutor_ExecuteIntent_CalendarSuccess(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	// Create successful calendar executor mock
	calendarMock := &mockCalendarExecutor{
		result: calexec.ExecuteResult{
			EnvelopeID:         "env-cal-001",
			Success:            true,
			ProviderResponseID: "prov-cal-001",
			ExecutedAt:         fixedTime,
		},
	}

	executor := NewExecutor(clk, emitter).
		WithCalendarExecutor(calendarMock)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-cal-001",
		DraftID:            "draft-cal-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionCalendarRespond,
		CalendarEventID:    "event-001",
		CalendarResponse:   "accept",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if !outcome.Success {
		t.Errorf("expected success, got error: %s", outcome.Error)
	}
	if !calendarMock.called {
		t.Error("calendar executor should have been called")
	}
	if outcome.ProviderResponseID != "prov-cal-001" {
		t.Errorf("ProviderResponseID = %s, want prov-cal-001", outcome.ProviderResponseID)
	}
}

func TestExecutor_ExecuteIntent_EmailBlocked(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	// Create blocked email executor mock
	emailMock := &mockEmailExecutor{
		result: &emailexec.Envelope{
			EnvelopeID: "env-001",
			DraftID:    "draft-001",
			Status:     emailexec.EnvelopeStatusBlocked,
			ExecutionResult: &emailexec.ExecutionResult{
				Success:       false,
				BlockedReason: "policy mismatch",
			},
		},
	}

	executor := NewExecutor(clk, emitter).
		WithEmailExecutor(emailMock)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-001",
		DraftID:            "draft-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-000",
		EmailTo:            "test@example.com",
		EmailSubject:       "Test Subject",
		EmailBody:          "Test Body",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if outcome.Success {
		t.Error("expected blocked, got success")
	}
	if !outcome.Blocked {
		t.Error("expected Blocked=true")
	}
	if outcome.BlockedReason != "policy mismatch" {
		t.Errorf("BlockedReason = %s, want 'policy mismatch'", outcome.BlockedReason)
	}

	// Check blocked event emitted
	hasBlockedEvent := false
	for _, e := range emitter.events {
		if e.Type == events.Phase10ExecutionBlocked {
			hasBlockedEvent = true
			break
		}
	}
	if !hasBlockedEvent {
		t.Error("expected Phase10ExecutionBlocked event")
	}
}

func TestExecutor_ExecuteIntent_CalendarBlocked(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	// Create blocked calendar executor mock
	calendarMock := &mockCalendarExecutor{
		result: calexec.ExecuteResult{
			EnvelopeID:    "env-cal-001",
			Success:       false,
			Blocked:       true,
			BlockedReason: "view snapshot stale",
			ExecutedAt:    fixedTime,
		},
	}

	executor := NewExecutor(clk, emitter).
		WithCalendarExecutor(calendarMock)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-cal-001",
		DraftID:            "draft-cal-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionCalendarRespond,
		CalendarEventID:    "event-001",
		CalendarResponse:   "accept",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if outcome.Success {
		t.Error("expected blocked, got success")
	}
	if !outcome.Blocked {
		t.Error("expected Blocked=true")
	}
	if outcome.BlockedReason != "view snapshot stale" {
		t.Errorf("BlockedReason = %s, want 'view snapshot stale'", outcome.BlockedReason)
	}
}

func TestExecutor_ExecuteIntent_NoExecutorConfigured(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	executor := NewExecutor(clk, emitter) // No executors configured

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-001",
		DraftID:            "draft-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-000",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if outcome.Success {
		t.Error("expected blocked, got success")
	}
	if !outcome.Blocked {
		t.Error("expected Blocked=true")
	}
	if outcome.BlockedReason != "email executor not configured" {
		t.Errorf("BlockedReason = %s, want 'email executor not configured'", outcome.BlockedReason)
	}
}

func TestExecutor_ExecuteIntent_InvalidIntent(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	executor := NewExecutor(clk, emitter)

	// Intent missing required fields
	intent := &execintent.ExecutionIntent{
		IntentID: "intent-001",
		// Missing DraftID, CircleID, etc.
		Action:    execintent.ActionEmailSend,
		CreatedAt: fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if outcome.Success {
		t.Error("expected blocked for invalid intent")
	}
	if !outcome.Blocked {
		t.Error("expected Blocked=true for validation failure")
	}
	if outcome.BlockedReason == "" {
		t.Error("expected BlockedReason to be set")
	}
}

func TestExecutor_ExecuteIntent_UnknownAction(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}

	executor := NewExecutor(clk, emitter)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-001",
		DraftID:            "draft-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionClass("unknown_action"),
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	outcome := executor.ExecuteIntent(context.Background(), intent, "trace-001")

	if outcome.Success {
		t.Error("expected blocked for unknown action")
	}
	if !outcome.Blocked {
		t.Error("expected Blocked=true")
	}
	if outcome.BlockedReason == "" {
		t.Error("expected BlockedReason to mention unknown action")
	}
}

func TestExecutor_BuildEmailEnvelope_Determinism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	executor := NewExecutor(clk, nil)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-001",
		DraftID:            "draft-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-000",
		EmailTo:            "test@example.com",
		EmailSubject:       "Test Subject",
		EmailBody:          "Test Body",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	env1 := executor.buildEmailEnvelope(intent, "trace-001", fixedTime)
	env2 := executor.buildEmailEnvelope(intent, "trace-001", fixedTime)

	if env1.EnvelopeID != env2.EnvelopeID {
		t.Errorf("envelope IDs should match: %s vs %s", env1.EnvelopeID, env2.EnvelopeID)
	}
	if env1.IdempotencyKey != env2.IdempotencyKey {
		t.Errorf("idempotency keys should match: %s vs %s", env1.IdempotencyKey, env2.IdempotencyKey)
	}
}

func TestExecutor_BuildCalendarEnvelope_Determinism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	executor := NewExecutor(clk, nil)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-cal-001",
		DraftID:            "draft-cal-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionCalendarRespond,
		CalendarEventID:    "event-001",
		CalendarResponse:   "accept",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	env1 := executor.buildCalendarEnvelope(intent, "trace-001", fixedTime)
	env2 := executor.buildCalendarEnvelope(intent, "trace-001", fixedTime)

	if env1.EnvelopeID != env2.EnvelopeID {
		t.Errorf("envelope IDs should match: %s vs %s", env1.EnvelopeID, env2.EnvelopeID)
	}
	if env1.IdempotencyKey != env2.IdempotencyKey {
		t.Errorf("idempotency keys should match: %s vs %s", env1.IdempotencyKey, env2.IdempotencyKey)
	}
}

func TestExecutor_BuildEmailEnvelope_Fields(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	executor := NewExecutor(clk, nil)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-001",
		DraftID:            "draft-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionEmailSend,
		EmailThreadID:      "thread-001",
		EmailMessageID:     "msg-000",
		EmailTo:            "test@example.com",
		EmailSubject:       "Test Subject",
		EmailBody:          "Test Body",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	env := executor.buildEmailEnvelope(intent, "trace-001", fixedTime)

	if env.DraftID != intent.DraftID {
		t.Errorf("DraftID = %s, want %s", env.DraftID, intent.DraftID)
	}
	if string(env.CircleID) != intent.CircleID {
		t.Errorf("CircleID = %s, want %s", env.CircleID, intent.CircleID)
	}
	if env.ThreadID != intent.EmailThreadID {
		t.Errorf("ThreadID = %s, want %s", env.ThreadID, intent.EmailThreadID)
	}
	if env.InReplyToMessageID != intent.EmailMessageID {
		t.Errorf("InReplyToMessageID = %s, want %s", env.InReplyToMessageID, intent.EmailMessageID)
	}
	if env.Subject != intent.EmailSubject {
		t.Errorf("Subject = %s, want %s", env.Subject, intent.EmailSubject)
	}
	if env.Body != intent.EmailBody {
		t.Errorf("Body = %s, want %s", env.Body, intent.EmailBody)
	}
	if env.PolicySnapshotHash != intent.PolicySnapshotHash {
		t.Errorf("PolicySnapshotHash = %s, want %s", env.PolicySnapshotHash, intent.PolicySnapshotHash)
	}
	if env.ViewSnapshotHash != intent.ViewSnapshotHash {
		t.Errorf("ViewSnapshotHash = %s, want %s", env.ViewSnapshotHash, intent.ViewSnapshotHash)
	}
	if env.TraceID != "trace-001" {
		t.Errorf("TraceID = %s, want trace-001", env.TraceID)
	}
	if env.Status != emailexec.EnvelopeStatusPending {
		t.Errorf("Status = %s, want pending", env.Status)
	}
}

func TestExecutor_BuildCalendarEnvelope_Fields(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	executor := NewExecutor(clk, nil)

	intent := &execintent.ExecutionIntent{
		IntentID:           "intent-cal-001",
		DraftID:            "draft-cal-001",
		CircleID:           "circle-001",
		Action:             execintent.ActionCalendarRespond,
		CalendarEventID:    "event-001",
		CalendarResponse:   "accept",
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		CreatedAt:          fixedTime,
	}

	env := executor.buildCalendarEnvelope(intent, "trace-001", fixedTime)

	if env.DraftID != intent.DraftID {
		t.Errorf("DraftID = %s, want %s", env.DraftID, intent.DraftID)
	}
	if string(env.CircleID) != intent.CircleID {
		t.Errorf("CircleID = %s, want %s", env.CircleID, intent.CircleID)
	}
	if env.EventID != intent.CalendarEventID {
		t.Errorf("EventID = %s, want %s", env.EventID, intent.CalendarEventID)
	}
	if string(env.Response) != intent.CalendarResponse {
		t.Errorf("Response = %s, want %s", env.Response, intent.CalendarResponse)
	}
	if env.PolicySnapshotHash != intent.PolicySnapshotHash {
		t.Errorf("PolicySnapshotHash = %s, want %s", env.PolicySnapshotHash, intent.PolicySnapshotHash)
	}
	if env.ViewSnapshotHash != intent.ViewSnapshotHash {
		t.Errorf("ViewSnapshotHash = %s, want %s", env.ViewSnapshotHash, intent.ViewSnapshotHash)
	}
	if env.TraceID != "trace-001" {
		t.Errorf("TraceID = %s, want trace-001", env.TraceID)
	}
	if env.Status != calexec.EnvelopeStatusPending {
		t.Errorf("Status = %s, want pending", env.Status)
	}
}
