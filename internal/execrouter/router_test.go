package execrouter

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/execintent"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

type mockEmitter struct {
	events []events.Event
}

func (m *mockEmitter) Emit(e events.Event) {
	m.events = append(m.events, e)
}

func TestRouter_BuildIntentFromDraft_EmailReply(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}
	router := NewRouter(clk, emitter)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-email-001"),
		DraftType:          draft.DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-123",
		ViewSnapshotHash:   "view-hash-456",
		Content: draft.EmailDraftContent{
			To:                 "recipient@example.com",
			Subject:            "Re: Test Subject",
			Body:               "Test body content",
			ThreadID:           "thread-001",
			InReplyToMessageID: "msg-001",
		},
	}

	intent, err := router.BuildIntentFromDraft(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intent.Action != execintent.ActionEmailSend {
		t.Errorf("Action = %s, want %s", intent.Action, execintent.ActionEmailSend)
	}
	if intent.DraftID != d.DraftID {
		t.Errorf("DraftID = %s, want %s", intent.DraftID, d.DraftID)
	}
	if intent.EmailTo != "recipient@example.com" {
		t.Errorf("EmailTo = %s, want recipient@example.com", intent.EmailTo)
	}
	if intent.EmailThreadID != "thread-001" {
		t.Errorf("EmailThreadID = %s, want thread-001", intent.EmailThreadID)
	}
	if intent.PolicySnapshotHash != "policy-hash-123" {
		t.Errorf("PolicySnapshotHash = %s, want policy-hash-123", intent.PolicySnapshotHash)
	}
	if intent.ViewSnapshotHash != "view-hash-456" {
		t.Errorf("ViewSnapshotHash = %s, want view-hash-456", intent.ViewSnapshotHash)
	}
	if intent.IntentID == "" {
		t.Error("IntentID should be set after Finalize")
	}
}

func TestRouter_BuildIntentFromDraft_CalendarResponse(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-cal-001"),
		DraftType:          draft.DraftTypeCalendarResponse,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-789",
		ViewSnapshotHash:   "view-hash-abc",
		Content: draft.CalendarDraftContent{
			EventID:  "event-001",
			Response: draft.CalendarResponseAccept,
		},
	}

	intent, err := router.BuildIntentFromDraft(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intent.Action != execintent.ActionCalendarRespond {
		t.Errorf("Action = %s, want %s", intent.Action, execintent.ActionCalendarRespond)
	}
	if intent.CalendarEventID != "event-001" {
		t.Errorf("CalendarEventID = %s, want event-001", intent.CalendarEventID)
	}
	if intent.CalendarResponse != "accept" {
		t.Errorf("CalendarResponse = %s, want accept", intent.CalendarResponse)
	}
}

func TestRouter_BuildIntentFromDraft_CommerceShipment(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-commerce-001"),
		DraftType:          draft.DraftTypeShipmentFollowUp,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-def",
		ViewSnapshotHash:   "view-hash-ghi",
		Content: draft.ShipmentFollowUpContent{
			Vendor:        "Amazon",
			VendorContact: draft.KnownVendorContact("support@amazon.co.uk"),
			OrderID:       "ORD-123",
			Subject:       "Where is my order?",
			Body:          "I would like to know the status of my order.",
		},
	}

	intent, err := router.BuildIntentFromDraft(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intent.Action != execintent.ActionEmailSend {
		t.Errorf("Action = %s, want %s", intent.Action, execintent.ActionEmailSend)
	}
	if intent.EmailTo != "support@amazon.co.uk" {
		t.Errorf("EmailTo = %s, want support@amazon.co.uk", intent.EmailTo)
	}
	if !strings.Contains(intent.EmailThreadID, "commerce-shipment-ORD-123") {
		t.Errorf("EmailThreadID = %s, should contain commerce-shipment-ORD-123", intent.EmailThreadID)
	}
}

func TestRouter_BuildIntentFromDraft_NotApproved(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}
	router := NewRouter(clk, emitter)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-001"),
		DraftType:          draft.DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusProposed, // Not approved
		PolicySnapshotHash: "hash",
		ViewSnapshotHash:   "hash",
		Content: draft.EmailDraftContent{
			ThreadID: "thread-001",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for non-approved draft")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Errorf("error should mention 'not approved': %v", err)
	}

	// Check event was emitted
	hasBlockedEvent := false
	for _, e := range emitter.events {
		if e.Type == events.Phase10ExecutionBlockedNotApproved {
			hasBlockedEvent = true
			break
		}
	}
	if !hasBlockedEvent {
		t.Error("expected Phase10ExecutionBlockedNotApproved event")
	}
}

func TestRouter_BuildIntentFromDraft_MissingPolicyHash(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}
	router := NewRouter(clk, emitter)

	d := &draft.Draft{
		DraftID:          draft.DraftID("draft-001"),
		DraftType:        draft.DraftTypeEmailReply,
		CircleID:         identity.EntityID("circle-test"),
		Status:           draft.StatusApproved,
		ViewSnapshotHash: "hash",
		// PolicySnapshotHash is missing
		Content: draft.EmailDraftContent{
			ThreadID: "thread-001",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for missing policy hash")
	}
	if !strings.Contains(err.Error(), "PolicySnapshotHash") {
		t.Errorf("error should mention 'PolicySnapshotHash': %v", err)
	}

	// Check event was emitted
	hasHashEvent := false
	for _, e := range emitter.events {
		if e.Type == events.Phase10PolicyHashMissing {
			hasHashEvent = true
			break
		}
	}
	if !hasHashEvent {
		t.Error("expected Phase10PolicyHashMissing event")
	}
}

func TestRouter_BuildIntentFromDraft_MissingViewHash(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	emitter := &mockEmitter{}
	router := NewRouter(clk, emitter)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-001"),
		DraftType:          draft.DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "hash",
		// ViewSnapshotHash is missing
		Content: draft.EmailDraftContent{
			ThreadID: "thread-001",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for missing view hash")
	}
	if !strings.Contains(err.Error(), "ViewSnapshotHash") {
		t.Errorf("error should mention 'ViewSnapshotHash': %v", err)
	}
}

func TestRouter_BuildIntentFromDraft_UnknownVendorContact(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-commerce-001"),
		DraftType:          draft.DraftTypeShipmentFollowUp,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "hash",
		ViewSnapshotHash:   "hash",
		Content: draft.ShipmentFollowUpContent{
			Vendor:        "SomeVendor",
			VendorContact: draft.UnknownVendorContact("vendor-hash-123"), // Unknown contact
			OrderID:       "ORD-123",
			Subject:       "Test",
			Body:          "Test",
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for unknown vendor contact")
	}
	if !strings.Contains(err.Error(), "unknown vendor contact") {
		t.Errorf("error should mention 'unknown vendor contact': %v", err)
	}
}

func TestRouter_BuildIntentFromDraft_EmailMissingThread(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	router := NewRouter(clk, nil)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-001"),
		DraftType:          draft.DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "hash",
		ViewSnapshotHash:   "hash",
		Content: draft.EmailDraftContent{
			To:      "test@example.com",
			Subject: "Test",
			Body:    "Test",
			// ThreadID and InReplyToMessageID are both empty
		},
	}

	_, err := router.BuildIntentFromDraft(d)
	if err == nil {
		t.Error("expected error for missing thread context")
	}
	if !strings.Contains(err.Error(), "thread") || !strings.Contains(err.Error(), "message") {
		t.Errorf("error should mention thread/message: %v", err)
	}
}

func TestRouter_BuildIntentFromDraft_Determinism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	d := &draft.Draft{
		DraftID:            draft.DraftID("draft-det-001"),
		DraftType:          draft.DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-test"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-det",
		ViewSnapshotHash:   "view-hash-det",
		Content: draft.EmailDraftContent{
			To:       "test@example.com",
			Subject:  "Test Subject",
			Body:     "Test body",
			ThreadID: "thread-det-001",
		},
	}

	router1 := NewRouter(clk, nil)
	router2 := NewRouter(clk, nil)

	intent1, err1 := router1.BuildIntentFromDraft(d)
	intent2, err2 := router2.BuildIntentFromDraft(d)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}

	if intent1.IntentID != intent2.IntentID {
		t.Errorf("IntentIDs should match: %s vs %s", intent1.IntentID, intent2.IntentID)
	}
	if intent1.DeterministicHash != intent2.DeterministicHash {
		t.Errorf("Hashes should match: %s vs %s", intent1.DeterministicHash, intent2.DeterministicHash)
	}
}
