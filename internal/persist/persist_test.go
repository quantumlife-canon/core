package persist

import (
	"context"
	"testing"
	"time"

	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/primitives"
)

func TestDraftStore_PutAndGet(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := NewDraftStore(log)
	if err != nil {
		t.Fatalf("NewDraftStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	d := draft.Draft{
		DraftID:   "draft-1",
		DraftType: draft.DraftTypeEmailReply,
		CircleID:  "work",
		Status:    draft.StatusProposed,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := store.Put(d); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, ok := store.Get("draft-1")
	if !ok {
		t.Fatal("Get returned not found")
	}

	if got.CircleID != d.CircleID {
		t.Errorf("CircleID mismatch: %q != %q", got.CircleID, d.CircleID)
	}

	// Check log has record
	if log.Count() != 1 {
		t.Errorf("log.Count() = %d, want 1", log.Count())
	}
}

func TestDraftStore_Replay(t *testing.T) {
	log := storelog.NewInMemoryLog()

	// Create store and add draft
	store1, _ := NewDraftStore(log)
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	d := draft.Draft{
		DraftID:   "draft-1",
		DraftType: draft.DraftTypeEmailReply,
		CircleID:  "work",
		Status:    draft.StatusProposed,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	store1.Put(d)

	// Create new store from same log - should replay
	store2, err := NewDraftStore(log)
	if err != nil {
		t.Fatalf("NewDraftStore failed: %v", err)
	}

	got, ok := store2.Get("draft-1")
	if !ok {
		t.Fatal("draft not found after replay")
	}

	if got.CircleID != "work" {
		t.Errorf("CircleID after replay: %q, want 'work'", got.CircleID)
	}
}

func TestDraftStore_UpdateStatus(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, _ := NewDraftStore(log)

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	d := draft.Draft{
		DraftID:   "draft-1",
		DraftType: draft.DraftTypeEmailReply,
		CircleID:  "work",
		Status:    draft.StatusProposed,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	store.Put(d)

	// Update status
	later := now.Add(time.Hour)
	err := store.UpdateStatus("draft-1", draft.StatusApproved, "person approved", "person-1", later)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := store.Get("draft-1")
	if got.Status != draft.StatusApproved {
		t.Errorf("Status = %q, want 'executed'", got.Status)
	}

	// Check log has 2 records (create + update)
	if log.Count() != 2 {
		t.Errorf("log.Count() = %d, want 2", log.Count())
	}
}

func TestFeedbackStore_PutAndList(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := NewFeedbackStore(log)
	if err != nil {
		t.Fatalf("NewFeedbackStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	record := feedback.FeedbackRecord{
		FeedbackID: "fb-1",
		CircleID:   "work",
		TargetType: feedback.TargetInterruption,
		TargetID:   "interrupt-1",
		Signal:     feedback.SignalHelpful,
		CapturedAt: now,
	}

	if err := store.Put(record); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	records := store.List()
	if len(records) != 1 {
		t.Errorf("List() returned %d records, want 1", len(records))
	}

	stats := store.Stats()
	if stats.HelpfulCount != 1 {
		t.Errorf("HelpfulCount = %d, want 1", stats.HelpfulCount)
	}
}

func TestDedupStore_HasSeen(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := NewDedupStore(log)
	if err != nil {
		t.Fatalf("NewDedupStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	if store.HasSeen("key-1") {
		t.Error("should not have seen key-1 yet")
	}

	store.MarkSeenAt("key-1", now)

	if !store.HasSeen("key-1") {
		t.Error("should have seen key-1 after marking")
	}

	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store.Count())
	}
}

func TestDedupStore_Replay(t *testing.T) {
	log := storelog.NewInMemoryLog()

	// Create store and mark keys
	store1, _ := NewDedupStore(log)
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	store1.MarkSeenAt("key-1", now)
	store1.MarkSeenAt("key-2", now)

	// Create new store from same log - should replay
	store2, err := NewDedupStore(log)
	if err != nil {
		t.Fatalf("NewDedupStore failed: %v", err)
	}

	if !store2.HasSeen("key-1") {
		t.Error("key-1 should be seen after replay")
	}
	if !store2.HasSeen("key-2") {
		t.Error("key-2 should be seen after replay")
	}
	if store2.Count() != 2 {
		t.Errorf("Count() after replay = %d, want 2", store2.Count())
	}
}

func TestApprovalStore_StoreAndGet(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, err := NewApprovalStore(log)
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

func TestApprovalStore_GetApprovals(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, _ := NewApprovalStore(log)

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Add two approvals for same action
	artifact1 := &primitives.ApprovalArtifact{
		ApprovalID:       "approval-1",
		IntersectionID:   "intersection-1",
		ActionID:         "action-1",
		ActionHash:       "hash123",
		ApproverCircleID: "circle-a",
		ApprovedAt:       now,
		ExpiresAt:        now.Add(time.Hour),
		Signature:        []byte("sig1"),
	}
	artifact2 := &primitives.ApprovalArtifact{
		ApprovalID:       "approval-2",
		IntersectionID:   "intersection-1",
		ActionID:         "action-1",
		ActionHash:       "hash123",
		ApproverCircleID: "circle-b",
		ApprovedAt:       now.Add(time.Minute),
		ExpiresAt:        now.Add(time.Hour),
		Signature:        []byte("sig2"),
	}

	store.StoreApproval(ctx, artifact1)
	store.StoreApproval(ctx, artifact2)

	approvals, err := store.GetApprovals(ctx, "intersection-1", "action-1")
	if err != nil {
		t.Fatalf("GetApprovals failed: %v", err)
	}

	if len(approvals) != 2 {
		t.Errorf("GetApprovals returned %d, want 2", len(approvals))
	}

	// Should be sorted by approval time
	if approvals[0].ApprovalID != "approval-1" {
		t.Error("approvals should be sorted by approval time")
	}
}

func TestApprovalStore_DuplicateRejection(t *testing.T) {
	log := storelog.NewInMemoryLog()
	store, _ := NewApprovalStore(log)

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
		Signature:        []byte("sig1"),
	}

	store.StoreApproval(ctx, artifact)

	// Same circle trying to approve again
	artifact2 := &primitives.ApprovalArtifact{
		ApprovalID:       "approval-2",
		IntersectionID:   "intersection-1",
		ActionID:         "action-1",
		ActionHash:       "hash123",
		ApproverCircleID: "circle-a", // Same circle
		ApprovedAt:       now.Add(time.Minute),
		ExpiresAt:        now.Add(time.Hour),
		Signature:        []byte("sig2"),
	}

	err := store.StoreApproval(ctx, artifact2)
	if err == nil {
		t.Error("expected duplicate rejection error")
	}
}
