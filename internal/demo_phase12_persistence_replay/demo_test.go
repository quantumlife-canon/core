// Package demo_phase12_persistence_replay demonstrates Phase 12 functionality.
//
// This package tests:
// - Append-only log storage and replay
// - Persistent stores (dedup, draft, approval, feedback)
// - Run snapshot creation and verification
// - Deterministic replay verification
//
// CRITICAL: All operations are synchronous. No goroutines.
// CRITICAL: Uses injected clock, never time.Now().
//
// Reference: docs/ADR/ADR-0027-phase12-persistence-replay.md
package demo_phase12_persistence_replay

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/runlog"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/primitives"
)

// TestAppendOnlyLog_Persistence tests that the append-only log persists and replays.
func TestAppendOnlyLog_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "events.log")

	// Create log and add records
	log1, err := storelog.NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog failed: %v", err)
	}

	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Add some records
	for i := 0; i < 10; i++ {
		record := storelog.NewRecord(
			storelog.RecordTypeEvent,
			ts.Add(time.Duration(i)*time.Minute),
			"work",
			"email|google|msg-"+itoa(i)+"|test@example.com",
		)
		if err := log1.Append(record); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	if log1.Count() != 10 {
		t.Errorf("log.Count() = %d, want 10", log1.Count())
	}

	// Reopen log and verify records
	log2, err := storelog.NewFileLog(logPath)
	if err != nil {
		t.Fatalf("NewFileLog (reopen) failed: %v", err)
	}

	if log2.Count() != 10 {
		t.Errorf("log.Count() after reopen = %d, want 10", log2.Count())
	}

	// Verify all hashes match
	if err := log2.Verify(); err != nil {
		t.Errorf("log.Verify() failed: %v", err)
	}
}

// TestAppendOnlyLog_DuplicateRejection tests that duplicates are rejected.
func TestAppendOnlyLog_DuplicateRejection(t *testing.T) {
	log := storelog.NewInMemoryLog()

	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	record := storelog.NewRecord(storelog.RecordTypeEvent, ts, "work", "email|test")

	// First append should succeed
	if err := log.Append(record); err != nil {
		t.Fatalf("First append failed: %v", err)
	}

	// Second append should fail
	if err := log.Append(record); err != storelog.ErrRecordExists {
		t.Errorf("expected ErrRecordExists, got %v", err)
	}
}

// TestPersistentDraftStore tests the persistent draft store.
func TestPersistentDraftStore(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := persist.NewDraftStore(log)
	if err != nil {
		t.Fatalf("NewDraftStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Create and store drafts
	d1 := draft.Draft{
		DraftID:   "draft-1",
		DraftType: draft.DraftTypeEmailReply,
		CircleID:  "work",
		Status:    draft.StatusProposed,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := store.Put(d1); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify it was stored
	got, ok := store.Get("draft-1")
	if !ok {
		t.Fatal("draft not found")
	}
	if got.CircleID != "work" {
		t.Errorf("CircleID = %q, want 'work'", got.CircleID)
	}

	// Verify log has record
	records, _ := log.ListByType(storelog.RecordTypeDraft)
	if len(records) != 1 {
		t.Errorf("log has %d draft records, want 1", len(records))
	}
}

// TestPersistentFeedbackStore tests the persistent feedback store.
func TestPersistentFeedbackStore(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := persist.NewFeedbackStore(log)
	if err != nil {
		t.Fatalf("NewFeedbackStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	record := feedback.NewFeedbackRecord(
		feedback.TargetInterruption,
		"interrupt-1",
		"work",
		now,
		feedback.SignalHelpful,
		"This was useful",
	)

	if err := store.Put(record); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, ok := store.Get(record.FeedbackID)
	if !ok {
		t.Fatal("feedback not found")
	}
	if got.Signal != feedback.SignalHelpful {
		t.Errorf("Signal = %q, want 'helpful'", got.Signal)
	}

	stats := store.Stats()
	if stats.HelpfulCount != 1 {
		t.Errorf("HelpfulCount = %d, want 1", stats.HelpfulCount)
	}
}

// TestPersistentDedupStore tests the persistent dedup store.
func TestPersistentDedupStore(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := persist.NewDedupStore(log)
	if err != nil {
		t.Fatalf("NewDedupStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Mark some keys as seen
	store.MarkSeenAt("email:msg-1:work:20250115", now)
	store.MarkSeenAt("calendar:event-1:work:20250115", now)

	if !store.HasSeen("email:msg-1:work:20250115") {
		t.Error("should have seen email key")
	}
	if !store.HasSeen("calendar:event-1:work:20250115") {
		t.Error("should have seen calendar key")
	}
	if store.HasSeen("nonexistent") {
		t.Error("should not have seen nonexistent key")
	}

	if store.Count() != 2 {
		t.Errorf("Count() = %d, want 2", store.Count())
	}
}

// TestPersistentApprovalStore tests the persistent approval store.
func TestPersistentApprovalStore(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := persist.NewApprovalStore(log)
	if err != nil {
		t.Fatalf("NewApprovalStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	artifact := &primitives.ApprovalArtifact{
		ApprovalID:       "approval-1",
		IntersectionID:   "intersection-1",
		ActionID:         "action-1",
		ActionHash:       "hash123",
		ApproverCircleID: "circle-a",
		ApprovedAt:       now,
		ExpiresAt:        now.Add(time.Hour),
		Signature:        []byte("sig123"),
	}

	if err := store.StoreApproval(ctx, artifact); err != nil {
		t.Fatalf("StoreApproval failed: %v", err)
	}

	got, err := store.GetApprovalByID(ctx, "approval-1")
	if err != nil {
		t.Fatalf("GetApprovalByID failed: %v", err)
	}

	if got.ApproverCircleID != "circle-a" {
		t.Errorf("ApproverCircleID = %q, want 'circle-a'", got.ApproverCircleID)
	}
}

// TestRunSnapshot_DeterministicHash tests that run snapshots have deterministic hashes.
func TestRunSnapshot_DeterministicHash(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Create two identical snapshots
	s1 := runlog.NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	s1.EventsIngested = 10
	s1.InterruptionsCreated = 5
	s1.DraftsCreated = 3
	s1.EventHashes = []string{"hash-a", "hash-b"}
	s1.FinalizeSnapshot()

	s2 := runlog.NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	s2.EventsIngested = 10
	s2.InterruptionsCreated = 5
	s2.DraftsCreated = 3
	s2.EventHashes = []string{"hash-a", "hash-b"}
	s2.FinalizeSnapshot()

	if s1.ResultHash != s2.ResultHash {
		t.Errorf("identical snapshots have different hashes: %s != %s", s1.ResultHash, s2.ResultHash)
	}
}

// TestRunSnapshot_ReplayVerification tests that replay verification works.
func TestRunSnapshot_ReplayVerification(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Original run
	original := runlog.NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	original.EventsIngested = 10
	original.InterruptionsCreated = 5
	original.DraftsCreated = 3
	original.NeedsYouHash = "needs-you-hash-1"
	original.FinalizeSnapshot()

	// Identical replay
	replay := runlog.NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	replay.EventsIngested = 10
	replay.InterruptionsCreated = 5
	replay.DraftsCreated = 3
	replay.NeedsYouHash = "needs-you-hash-1"
	replay.FinalizeSnapshot()

	result := runlog.VerifyReplay(original, replay)
	if !result.Success {
		t.Errorf("identical replay should succeed, differences: %v", result.Differences)
	}

	// Different replay
	badReplay := runlog.NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	badReplay.EventsIngested = 10
	badReplay.InterruptionsCreated = 6 // Different!
	badReplay.DraftsCreated = 3
	badReplay.NeedsYouHash = "needs-you-hash-1"
	badReplay.FinalizeSnapshot()

	badResult := runlog.VerifyReplay(original, badReplay)
	if badResult.Success {
		t.Error("different replay should fail")
	}
	if len(badResult.Differences) == 0 {
		t.Error("differences should be reported")
	}
}

// TestRunStore_Persistence tests that run snapshots persist and replay.
func TestRunStore_Persistence(t *testing.T) {
	log := storelog.NewInMemoryLog()

	store1, err := runlog.NewFileRunStore(log)
	if err != nil {
		t.Fatalf("NewFileRunStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	s := runlog.NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	s.EventsIngested = 10
	s.InterruptionsCreated = 5
	s.FinalizeSnapshot()

	if err := store1.Store(s); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Create new store from same log - should replay
	store2, err := runlog.NewFileRunStore(log)
	if err != nil {
		t.Fatalf("NewFileRunStore (reopen) failed: %v", err)
	}

	got, err := store2.Get("run-1")
	if err != nil {
		t.Fatalf("Get after replay failed: %v", err)
	}

	if got.EventsIngested != 10 {
		t.Errorf("EventsIngested after replay = %d, want 10", got.EventsIngested)
	}
}

// TestEndToEnd_PersistenceAndReplay tests the complete persistence and replay flow.
func TestEndToEnd_PersistenceAndReplay(t *testing.T) {
	log := storelog.NewInMemoryLog()
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Create stores
	draftStore, _ := persist.NewDraftStore(log)
	feedbackStore, _ := persist.NewFeedbackStore(log)
	dedupStore, _ := persist.NewDedupStore(log)
	runStore, _ := runlog.NewFileRunStore(log)

	// Simulate a run

	// 1. Add some drafts
	for i := 0; i < 5; i++ {
		d := draft.Draft{
			DraftID:   draft.DraftID("draft-" + itoa(i)),
			DraftType: draft.DraftTypeEmailReply,
			CircleID:  "work",
			Status:    draft.StatusProposed,
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
			ExpiresAt: now.Add(24 * time.Hour),
		}
		draftStore.Put(d)
	}

	// 2. Add some feedback
	for i := 0; i < 3; i++ {
		record := feedback.NewFeedbackRecord(
			feedback.TargetDraft,
			"draft-"+itoa(i),
			"work",
			now.Add(time.Duration(i)*time.Minute),
			feedback.SignalHelpful,
			"",
		)
		feedbackStore.Put(record)
	}

	// 3. Mark some dedup keys
	dedupStore.MarkSeenAt("key-1", now)
	dedupStore.MarkSeenAt("key-2", now)

	// 4. Create run snapshot
	runSnapshot := runlog.NewRunSnapshot(
		runlog.ComputeRunID(now, "config-hash"),
		now,
		now.Add(time.Minute),
		"work",
		"config-hash",
	)
	runSnapshot.EventsIngested = 100
	runSnapshot.InterruptionsCreated = 20
	runSnapshot.InterruptionsDeduplicated = 5
	runSnapshot.DraftsCreated = 5
	runSnapshot.NeedsYouItems = 10
	runSnapshot.NeedsYouHash = "needs-you-hash"
	runSnapshot.FinalizeSnapshot()

	runStore.Store(runSnapshot)

	// Verify counts
	if draftStore.Count() != 5 {
		t.Errorf("draftStore.Count() = %d, want 5", draftStore.Count())
	}
	if feedbackStore.Count() != 3 {
		t.Errorf("feedbackStore.Count() = %d, want 3", feedbackStore.Count())
	}
	if dedupStore.Count() != 2 {
		t.Errorf("dedupStore.Count() = %d, want 2", dedupStore.Count())
	}
	if runStore.Count() != 1 {
		t.Errorf("runStore.Count() = %d, want 1", runStore.Count())
	}

	// Create new stores from same log - should replay
	draftStore2, _ := persist.NewDraftStore(log)
	feedbackStore2, _ := persist.NewFeedbackStore(log)
	dedupStore2, _ := persist.NewDedupStore(log)
	runStore2, _ := runlog.NewFileRunStore(log)

	if draftStore2.Count() != 5 {
		t.Errorf("draftStore2.Count() after replay = %d, want 5", draftStore2.Count())
	}
	if feedbackStore2.Count() != 3 {
		t.Errorf("feedbackStore2.Count() after replay = %d, want 3", feedbackStore2.Count())
	}
	if dedupStore2.Count() != 2 {
		t.Errorf("dedupStore2.Count() after replay = %d, want 2", dedupStore2.Count())
	}
	if runStore2.Count() != 1 {
		t.Errorf("runStore2.Count() after replay = %d, want 1", runStore2.Count())
	}
}

// itoa converts int to string without strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
