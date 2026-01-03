// Package demo_phase25_first_undoable_execution contains demo tests for Phase 25.
//
// Phase 25: First Undoable Execution (Opt-In, Single-Shot)
//
// This phase introduces the first real external write that is undoable.
// Only calendar_respond (RSVP) is supported because:
//   - Email send is not truly undoable
//   - Finance is not undoable
//   - Calendar RSVP can be reversed by applying previous response
//
// CRITICAL INVARIANTS:
//   - Single-shot per period (max one execution)
//   - Undo window is bounded (bucketed time)
//   - Undo is a first-class flow, not "best effort"
//   - No goroutines, no retries, no background execution
//   - Clock injection only
//
// Reference: docs/ADR/ADR-0055-phase25-first-undoable-execution.md
package demo_phase25_first_undoable_execution

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	domainundoableexec "quantumlife/pkg/domain/undoableexec"
)

// =============================================================================
// Test Fixtures
// =============================================================================

// mockClock returns a fixed time for determinism.
func mockClock(t time.Time) func() time.Time {
	return func() time.Time {
		return t
	}
}

// =============================================================================
// Domain Model Tests
// =============================================================================

func TestUndoWindow_BucketedTime(t *testing.T) {
	// Test that undo windows use 15-minute buckets
	now := time.Date(2025, 1, 15, 10, 7, 30, 0, time.UTC)

	window := domainundoableexec.NewUndoWindow(now)

	// Should bucket to 10:00 (floor to 15 minutes)
	if window.BucketStartRFC3339 != "2025-01-15T10:00:00Z" {
		t.Errorf("Expected bucket start 2025-01-15T10:00:00Z, got %s", window.BucketStartRFC3339)
	}

	// Duration should be 15 minutes
	if window.BucketDurationMinutes != 15 {
		t.Errorf("Expected 15 minute duration, got %d", window.BucketDurationMinutes)
	}
}

func TestUndoWindow_BucketedTime_45Minutes(t *testing.T) {
	// Test 45-minute boundary
	now := time.Date(2025, 1, 15, 10, 47, 30, 0, time.UTC)

	window := domainundoableexec.NewUndoWindow(now)

	// Should bucket to 10:45
	if window.BucketStartRFC3339 != "2025-01-15T10:45:00Z" {
		t.Errorf("Expected bucket start 2025-01-15T10:45:00Z, got %s", window.BucketStartRFC3339)
	}
}

func TestUndoWindow_IsExpired(t *testing.T) {
	// Create a window for 10:00
	window := domainundoableexec.UndoWindow{
		BucketStartRFC3339:    "2025-01-15T10:00:00Z",
		BucketDurationMinutes: 15,
	}

	// At 10:14 - NOT expired
	t1 := time.Date(2025, 1, 15, 10, 14, 59, 0, time.UTC)
	if window.IsExpired(t1) {
		t.Error("Window should not be expired at 10:14")
	}

	// At 10:15:01 - expired (after boundary)
	t2 := time.Date(2025, 1, 15, 10, 15, 1, 0, time.UTC)
	if !window.IsExpired(t2) {
		t.Error("Window should be expired after 10:15")
	}
}

func TestUndoWindow_DeadlineWindow(t *testing.T) {
	window := domainundoableexec.UndoWindow{
		BucketStartRFC3339:    "2025-01-15T10:00:00Z",
		BucketDurationMinutes: 15,
	}

	deadline := window.DeadlineWindow()

	// Deadline should be at 10:15
	if deadline.BucketStartRFC3339 != "2025-01-15T10:15:00Z" {
		t.Errorf("Expected deadline 2025-01-15T10:15:00Z, got %s", deadline.BucketStartRFC3339)
	}
}

func TestUndoRecord_Hash_Deterministic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	record1 := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	record2 := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	// Same inputs should produce same hash
	if record1.Hash() != record2.Hash() {
		t.Error("Identical records should have identical hashes")
	}

	// Hash should be deterministic (64 hex chars = 32 bytes)
	if len(record1.Hash()) != 64 {
		t.Errorf("Hash should be 64 chars, got %d", len(record1.Hash()))
	}
}

func TestUndoRecord_IsUndoAvailable(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	// Initially in undo_available state
	if record.State != domainundoableexec.StateUndoAvailable {
		t.Errorf("Expected undo_available state, got %s", record.State)
	}

	// Within window - should be available
	t1 := time.Date(2025, 1, 15, 10, 10, 0, 0, time.UTC)
	if !record.IsUndoAvailable(t1) {
		t.Error("Undo should be available within window")
	}

	// After window (deadline is at 10:15, window is 10:15-10:30, so at 10:35 it's expired)
	t2 := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)
	if record.IsUndoAvailable(t2) {
		t.Error("Undo should not be available after window")
	}
}

func TestUndoRecord_IsUndoAvailable_WrongState(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	// Change state to undone
	record.State = domainundoableexec.StateUndone

	// Even within window, should not be available because state is wrong
	t1 := time.Date(2025, 1, 15, 10, 10, 0, 0, time.UTC)
	if record.IsUndoAvailable(t1) {
		t.Error("Undo should not be available when state is undone")
	}
}

func TestPeriodKey_DailyBucket(t *testing.T) {
	// Test that period keys use daily buckets
	t1 := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 15, 23, 59, 59, 0, time.UTC)

	key1 := domainundoableexec.PeriodKeyFromTime(t1)
	key2 := domainundoableexec.PeriodKeyFromTime(t2)

	if key1 != key2 {
		t.Errorf("Same day should have same period key: %s vs %s", key1, key2)
	}

	if key1 != "2025-01-15" {
		t.Errorf("Expected 2025-01-15, got %s", key1)
	}
}

func TestPeriodKey_DifferentDays(t *testing.T) {
	t1 := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 16, 10, 30, 0, 0, time.UTC)

	key1 := domainundoableexec.PeriodKeyFromTime(t1)
	key2 := domainundoableexec.PeriodKeyFromTime(t2)

	if key1 == key2 {
		t.Errorf("Different days should have different period keys: %s vs %s", key1, key2)
	}
}

// =============================================================================
// Persistence Tests
// =============================================================================

func TestUndoableExecStore_AppendRecord(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))

	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	err := store.AppendRecord(record)
	if err != nil {
		t.Fatalf("AppendRecord failed: %v", err)
	}

	// Verify stored
	retrieved, found := store.GetByID(record.ID)
	if !found {
		t.Fatal("Record not found after append")
	}

	if retrieved.DraftID != "draft-abc" {
		t.Errorf("Expected draft-abc, got %s", retrieved.DraftID)
	}
}

func TestUndoableExecStore_AppendRecord_Deduplication(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))

	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	// Append twice
	_ = store.AppendRecord(record)
	_ = store.AppendRecord(record)

	// Should only have one record
	if store.Count() != 1 {
		t.Errorf("Expected 1 record after dedup, got %d", store.Count())
	}
}

func TestUndoableExecStore_HasExecutedThisPeriod(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))
	circleID := identity.EntityID("circle-1")
	periodKey := "2025-01-15"

	// Initially no execution
	if store.HasExecutedThisPeriod(circleID, periodKey) {
		t.Error("Should not have executed before any records")
	}

	// Add an undo_available record
	record := domainundoableexec.NewUndoRecord(
		periodKey,
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)
	store.AppendRecord(record)

	// Now should report executed (state is undo_available, not pending)
	if !store.HasExecutedThisPeriod(circleID, periodKey) {
		t.Error("Should report executed after adding undo_available record")
	}
}

func TestUndoableExecStore_HasExecutedThisPeriod_DifferentCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))
	circleID1 := identity.EntityID("circle-1")
	circleID2 := identity.EntityID("circle-2")
	periodKey := "2025-01-15"

	// Add record for circle-1
	record := domainundoableexec.NewUndoRecord(
		periodKey,
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)
	store.AppendRecord(record)

	// circle-1 has executed
	if !store.HasExecutedThisPeriod(circleID1, periodKey) {
		t.Error("circle-1 should have executed")
	}

	// circle-2 has NOT executed
	if store.HasExecutedThisPeriod(circleID2, periodKey) {
		t.Error("circle-2 should NOT have executed")
	}
}

func TestUndoableExecStore_AppendAck(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))

	// Create and store record
	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)
	store.AppendRecord(record)

	// Append ack to mark as undone
	ack := domainundoableexec.NewUndoAck(
		record.ID,
		domainundoableexec.StateUndone,
		now.Add(5*time.Minute),
		"undo requested",
	)
	err := store.AppendAck(ack)
	if err != nil {
		t.Fatalf("AppendAck failed: %v", err)
	}

	// Verify state updated
	retrieved, _ := store.GetByID(record.ID)
	if retrieved.State != domainundoableexec.StateUndone {
		t.Errorf("Expected undone state, got %s", retrieved.State)
	}
}

func TestUndoableExecStore_GetLatestUndoable(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))
	circleID := identity.EntityID("circle-1")

	// No records initially
	if store.GetLatestUndoable(circleID) != nil {
		t.Error("Should return nil when no records")
	}

	// Add an undoable record
	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)
	store.AppendRecord(record)

	// Should find the undoable record
	latest := store.GetLatestUndoable(circleID)
	if latest == nil {
		t.Error("Should find latest undoable record")
	}
	if latest.DraftID != "draft-abc" {
		t.Errorf("Expected draft-abc, got %s", latest.DraftID)
	}
}

func TestUndoableExecStore_GetByCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))
	circleID := identity.EntityID("circle-1")

	// Add two records for the same circle
	record1 := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-1",
		"env-1",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)
	record2 := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-2",
		"env-2",
		domainundoableexec.StatusAccepted,
		domainundoableexec.StatusDeclined,
		now.Add(time.Hour),
	)
	store.AppendRecord(record1)
	store.AppendRecord(record2)

	records := store.GetByCircle(circleID)
	if len(records) != 2 {
		t.Errorf("Expected 2 records for circle, got %d", len(records))
	}
}

// =============================================================================
// UI Page Tests
// =============================================================================

func TestUndoablePage_Eligible(t *testing.T) {
	page := domainundoableexec.NewUndoablePage(true)

	if !page.HasAction {
		t.Error("Page should have action when eligible")
	}
	if page.ActionLabel == "" {
		t.Error("ActionLabel should not be empty when eligible")
	}
}

func TestUndoablePage_NotEligible(t *testing.T) {
	page := domainundoableexec.NewUndoablePage(false)

	if page.HasAction {
		t.Error("Page should not have action when not eligible")
	}
	if page.Title == "" {
		t.Error("Title should not be empty")
	}
}

func TestDonePage_UndoAvailable(t *testing.T) {
	page := domainundoableexec.NewDonePage(true)

	if !page.UndoAvailable {
		t.Error("Page should show undo available")
	}
	if page.UndoMessage == "" {
		t.Error("UndoMessage should not be empty when undo available")
	}
}

func TestDonePage_UndoNotAvailable(t *testing.T) {
	page := domainundoableexec.NewDonePage(false)

	if page.UndoAvailable {
		t.Error("Page should show undo not available")
	}
	if page.UndoMessage != "" {
		t.Error("UndoMessage should be empty when undo not available")
	}
}

func TestUndoPage_CanUndo(t *testing.T) {
	page := domainundoableexec.NewUndoPage(true)

	if !page.CanUndo {
		t.Error("Page should show can undo")
	}
	if page.ActionLabel == "" {
		t.Error("ActionLabel should not be empty")
	}
}

func TestUndoPage_CannotUndo(t *testing.T) {
	page := domainundoableexec.NewUndoPage(false)

	if page.CanUndo {
		t.Error("Page should show cannot undo")
	}
	if page.Message == "" {
		t.Error("Message should explain why undo is not available")
	}
}

// =============================================================================
// Draft Integration Tests
// =============================================================================

func TestCalendarDraftContent_PreviousResponseStatus(t *testing.T) {
	// Test default value
	content1 := draft.CalendarDraftContent{
		EventID:  "event-1",
		Response: draft.CalendarResponseAccept,
	}
	if content1.GetPreviousResponseStatus() != "needs_action" {
		t.Errorf("Expected needs_action default, got %s", content1.GetPreviousResponseStatus())
	}

	// Test explicit value
	content2 := draft.CalendarDraftContent{
		EventID:                "event-2",
		Response:               draft.CalendarResponseDecline,
		PreviousResponseStatus: draft.CalendarResponseAccept,
	}
	if content2.GetPreviousResponseStatus() != draft.CalendarResponseAccept {
		t.Errorf("Expected accept, got %s", content2.GetPreviousResponseStatus())
	}
}

// =============================================================================
// Action Kind Tests
// =============================================================================

func TestActionKind_OnlyCalendarRespond(t *testing.T) {
	// Verify only calendar_respond is defined
	if domainundoableexec.ActionKindCalendarRespond != "calendar_respond" {
		t.Errorf("Expected calendar_respond, got %s", domainundoableexec.ActionKindCalendarRespond)
	}

	// Verify IsSupported works
	if !domainundoableexec.ActionKindCalendarRespond.IsSupported() {
		t.Error("calendar_respond should be supported")
	}

	// Verify unsupported kinds
	var unsupported domainundoableexec.UndoableActionKind = "email_send"
	if unsupported.IsSupported() {
		t.Error("email_send should not be supported")
	}
}

// =============================================================================
// Response Status Tests
// =============================================================================

func TestResponseStatus_Values(t *testing.T) {
	// Verify all expected response statuses exist
	statuses := []domainundoableexec.ResponseStatus{
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		domainundoableexec.StatusDeclined,
		domainundoableexec.StatusTentative,
	}

	expected := []string{"needs_action", "accepted", "declined", "tentative"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], s)
		}
	}
}

// =============================================================================
// Storelog Integration Tests
// =============================================================================

func TestStorelog_RecordRoundtrip(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))

	// Create record
	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)

	// Convert to storelog
	logRecord := store.RecordToStorelogRecord(record)

	// Verify storelog record
	if logRecord.Type != "UNDO_EXEC_RECORD" {
		t.Errorf("Expected UNDO_EXEC_RECORD, got %s", logRecord.Type)
	}
	if logRecord.Version != "v1" {
		t.Errorf("Expected v1, got %s", logRecord.Version)
	}
	if logRecord.Payload == "" {
		t.Error("Payload should not be empty")
	}

	// Replay from storelog
	store2 := persist.NewUndoableExecStore(mockClock(now))
	err := store2.ReplayRecordFromStorelog(logRecord)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Verify replayed record
	replayed, found := store2.GetByID(record.ID)
	if !found {
		t.Fatal("Replayed record not found")
	}
	if replayed.Hash() != record.Hash() {
		t.Error("Replayed record hash should match original")
	}
}

func TestStorelog_AckRoundtrip(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewUndoableExecStore(mockClock(now))

	// Create record first
	record := domainundoableexec.NewUndoRecord(
		"2025-01-15",
		"circle-1",
		domainundoableexec.ActionKindCalendarRespond,
		"draft-abc",
		"env-123",
		domainundoableexec.StatusNeedsAction,
		domainundoableexec.StatusAccepted,
		now,
	)
	store.AppendRecord(record)

	// Create ack
	ack := domainundoableexec.NewUndoAck(
		record.ID,
		domainundoableexec.StateUndone,
		now.Add(5*time.Minute),
		"undo performed",
	)

	// Convert to storelog
	logAck := store.AckToStorelogRecord(ack)

	// Verify storelog ack
	if logAck.Type != "UNDO_EXEC_ACK" {
		t.Errorf("Expected UNDO_EXEC_ACK, got %s", logAck.Type)
	}

	// Create new store with record and replay ack
	store2 := persist.NewUndoableExecStore(mockClock(now))
	store2.AppendRecord(record)
	err := store2.ReplayAckFromStorelog(logAck)
	if err != nil {
		t.Fatalf("Replay ack failed: %v", err)
	}

	// Verify state updated
	replayed, _ := store2.GetByID(record.ID)
	if replayed.State != domainundoableexec.StateUndone {
		t.Errorf("Expected undone state after ack replay, got %s", replayed.State)
	}
}

// =============================================================================
// UndoAck Tests
// =============================================================================

func TestUndoAck_Hash_Deterministic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	ack1 := domainundoableexec.NewUndoAck("record-1", domainundoableexec.StateUndone, now, "undo performed")
	ack2 := domainundoableexec.NewUndoAck("record-1", domainundoableexec.StateUndone, now, "undo performed")

	if ack1.Hash() != ack2.Hash() {
		t.Error("Identical acks should have identical hashes")
	}
}

func TestUndoAck_CanonicalString(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	ack := domainundoableexec.NewUndoAck("record-1", domainundoableexec.StateUndone, now, "undo performed")

	canonical := ack.CanonicalString()

	// Should contain key parts
	if canonical == "" {
		t.Error("Canonical string should not be empty")
	}
	// Should start with UNDO_ACK
	if canonical[:8] != "UNDO_ACK" {
		t.Errorf("Should start with UNDO_ACK, got %s", canonical[:8])
	}
}

// =============================================================================
// State Transition Tests
// =============================================================================

func TestUndoState_Transitions(t *testing.T) {
	// Verify state values match expected strings
	states := map[domainundoableexec.UndoState]string{
		domainundoableexec.StatePending:       "pending",
		domainundoableexec.StateExecuted:      "executed",
		domainundoableexec.StateUndoAvailable: "undo_available",
		domainundoableexec.StateUndone:        "undone",
		domainundoableexec.StateExpired:       "expired",
	}

	for state, expected := range states {
		if string(state) != expected {
			t.Errorf("Expected %s, got %s", expected, state)
		}
	}
}
