package runlog

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/storelog"
)

func TestRunSnapshot_ComputeResultHash(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	s1 := &RunSnapshot{
		RunID:          "run-1",
		StartTime:      now,
		EndTime:        now.Add(time.Minute),
		CircleID:       "work",
		EventsIngested: 10,
		ConfigHash:     "config-hash",
	}

	hash1 := s1.ComputeResultHash()
	hash2 := s1.ComputeResultHash()

	if hash1 != hash2 {
		t.Error("hash should be deterministic")
	}

	// Different events should produce different hash
	s2 := &RunSnapshot{
		RunID:          "run-1",
		StartTime:      now,
		EndTime:        now.Add(time.Minute),
		CircleID:       "work",
		EventsIngested: 11, // Different
		ConfigHash:     "config-hash",
	}

	hash3 := s2.ComputeResultHash()
	if hash1 == hash3 {
		t.Error("different snapshots should have different hashes")
	}
}

func TestRunSnapshot_Validate(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	s := &RunSnapshot{
		RunID:      "run-1",
		StartTime:  now,
		EndTime:    now.Add(time.Minute),
		CircleID:   "work",
		ConfigHash: "config-hash",
	}
	s.FinalizeSnapshot()

	if err := s.Validate(); err != nil {
		t.Errorf("valid snapshot failed validation: %v", err)
	}

	// Invalid: missing run ID
	s2 := &RunSnapshot{
		StartTime: now,
	}
	s2.ResultHash = s2.ComputeResultHash()
	if err := s2.Validate(); err == nil {
		t.Error("expected validation error for missing run ID")
	}

	// Invalid: hash mismatch
	s3 := &RunSnapshot{
		RunID:      "run-1",
		StartTime:  now,
		ResultHash: "wrong-hash",
	}
	if err := s3.Validate(); err != ErrHashMismatch {
		t.Errorf("expected ErrHashMismatch, got %v", err)
	}
}

func TestRunSnapshot_FinalizeSnapshot(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	s := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	s.EventsIngested = 10
	s.InterruptionsCreated = 5
	s.DraftsCreated = 3
	s.EventHashes = []string{"hash-c", "hash-a", "hash-b"}

	s.FinalizeSnapshot()

	// Hashes should be sorted
	if s.EventHashes[0] != "hash-a" {
		t.Error("event hashes should be sorted")
	}

	// Result hash should be set
	if s.ResultHash == "" {
		t.Error("result hash should be set after finalize")
	}
}

func TestVerifyReplay_Success(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	original := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	original.EventsIngested = 10
	original.InterruptionsCreated = 5
	original.FinalizeSnapshot()

	// Identical replay
	replay := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	replay.EventsIngested = 10
	replay.InterruptionsCreated = 5
	replay.FinalizeSnapshot()

	result := VerifyReplay(original, replay)
	if !result.Success {
		t.Errorf("replay should succeed, differences: %v", result.Differences)
	}
}

func TestVerifyReplay_Failure(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	original := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	original.EventsIngested = 10
	original.InterruptionsCreated = 5
	original.FinalizeSnapshot()

	// Different replay
	replay := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	replay.EventsIngested = 10
	replay.InterruptionsCreated = 6 // Different
	replay.FinalizeSnapshot()

	result := VerifyReplay(original, replay)
	if result.Success {
		t.Error("replay should fail due to different interruption count")
	}

	if len(result.Differences) == 0 {
		t.Error("differences should be reported")
	}
}

func TestInMemoryRunStore_StoreAndGet(t *testing.T) {
	store := NewInMemoryRunStore()

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	s := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	s.EventsIngested = 10
	s.FinalizeSnapshot()

	if err := store.Store(s); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	got, err := store.Get("run-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.EventsIngested != 10 {
		t.Errorf("EventsIngested = %d, want 10", got.EventsIngested)
	}
}

func TestInMemoryRunStore_List(t *testing.T) {
	store := NewInMemoryRunStore()

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Add in reverse order to test sorting
	s2 := NewRunSnapshot("run-2", now.Add(time.Hour), now.Add(time.Hour+time.Minute), "work", "cfg")
	s2.FinalizeSnapshot()
	store.Store(s2)

	s1 := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "cfg")
	s1.FinalizeSnapshot()
	store.Store(s1)

	list, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("List returned %d, want 2", len(list))
	}

	// Should be sorted by start time
	if list[0].RunID != "run-1" {
		t.Error("list should be sorted by start time")
	}
}

func TestInMemoryRunStore_ListByCircle(t *testing.T) {
	store := NewInMemoryRunStore()

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	s1 := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "cfg")
	s1.FinalizeSnapshot()
	store.Store(s1)

	s2 := NewRunSnapshot("run-2", now.Add(time.Hour), now.Add(time.Hour+time.Minute), "personal", "cfg")
	s2.FinalizeSnapshot()
	store.Store(s2)

	workRuns, _ := store.ListByCircle("work")
	if len(workRuns) != 1 {
		t.Errorf("ListByCircle(work) = %d, want 1", len(workRuns))
	}

	personalRuns, _ := store.ListByCircle("personal")
	if len(personalRuns) != 1 {
		t.Errorf("ListByCircle(personal) = %d, want 1", len(personalRuns))
	}
}

func TestFileRunStore_Persistence(t *testing.T) {
	log := storelog.NewInMemoryLog()

	store1, err := NewFileRunStore(log)
	if err != nil {
		t.Fatalf("NewFileRunStore failed: %v", err)
	}

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	s := NewRunSnapshot("run-1", now, now.Add(time.Minute), "work", "config-hash")
	s.EventsIngested = 10
	s.InterruptionsCreated = 5
	s.FinalizeSnapshot()

	if err := store1.Store(s); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Create new store from same log - should replay
	store2, err := NewFileRunStore(log)
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

func TestComputeRunID(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	id1 := ComputeRunID(now, "config-hash")
	id2 := ComputeRunID(now, "config-hash")

	if id1 != id2 {
		t.Error("run ID should be deterministic")
	}

	if !hasPrefix(id1, "run-") {
		t.Error("run ID should start with 'run-'")
	}

	// Different config should produce different ID
	id3 := ComputeRunID(now, "different-config")
	if id1 == id3 {
		t.Error("different config should produce different run ID")
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
