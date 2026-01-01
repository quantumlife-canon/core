package feedback

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestComputeFeedbackID_Deterministic(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	id1 := ComputeFeedbackID(
		TargetDraft,
		"draft-123",
		identity.EntityID("circle-1"),
		now,
		SignalHelpful,
	)

	id2 := ComputeFeedbackID(
		TargetDraft,
		"draft-123",
		identity.EntityID("circle-1"),
		now,
		SignalHelpful,
	)

	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %s != %s", id1, id2)
	}

	// Different input should give different ID
	id3 := ComputeFeedbackID(
		TargetDraft,
		"draft-456", // Different target
		identity.EntityID("circle-1"),
		now,
		SignalHelpful,
	)

	if id1 == id3 {
		t.Error("expected different IDs for different inputs")
	}
}

func TestNewFeedbackRecord(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	record := NewFeedbackRecord(
		TargetInterruption,
		"interrupt-123",
		identity.EntityID("circle-1"),
		now,
		SignalUnnecessary,
		"Too many notifications",
	)

	if record.FeedbackID == "" {
		t.Error("expected non-empty FeedbackID")
	}
	if record.CanonicalHash == "" {
		t.Error("expected non-empty CanonicalHash")
	}
	if record.TargetType != TargetInterruption {
		t.Errorf("expected TargetInterruption, got %s", record.TargetType)
	}
	if record.Signal != SignalUnnecessary {
		t.Errorf("expected SignalUnnecessary, got %s", record.Signal)
	}
}

func TestFeedbackRecord_Validate(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		record  FeedbackRecord
		wantErr error
	}{
		{
			name: "valid record",
			record: NewFeedbackRecord(
				TargetDraft,
				"draft-123",
				identity.EntityID("circle-1"),
				now,
				SignalHelpful,
				"",
			),
			wantErr: nil,
		},
		{
			name: "missing feedback ID",
			record: FeedbackRecord{
				TargetType: TargetDraft,
				TargetID:   "draft-123",
				CircleID:   identity.EntityID("circle-1"),
				Signal:     SignalHelpful,
			},
			wantErr: ErrMissingFeedbackID,
		},
		{
			name: "missing target type",
			record: FeedbackRecord{
				FeedbackID: "fb-123",
				TargetID:   "draft-123",
				CircleID:   identity.EntityID("circle-1"),
				Signal:     SignalHelpful,
			},
			wantErr: ErrMissingTargetType,
		},
		{
			name: "invalid target type",
			record: FeedbackRecord{
				FeedbackID: "fb-123",
				TargetType: "invalid",
				TargetID:   "draft-123",
				CircleID:   identity.EntityID("circle-1"),
				Signal:     SignalHelpful,
			},
			wantErr: ErrInvalidTargetType,
		},
		{
			name: "invalid signal",
			record: FeedbackRecord{
				FeedbackID: "fb-123",
				TargetType: TargetDraft,
				TargetID:   "draft-123",
				CircleID:   identity.EntityID("circle-1"),
				Signal:     "invalid",
			},
			wantErr: ErrInvalidSignal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryStore_PutAndGet(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	record := NewFeedbackRecord(
		TargetDraft,
		"draft-123",
		identity.EntityID("circle-1"),
		now,
		SignalHelpful,
		"Good suggestion",
	)

	err := store.Put(record)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, exists := store.Get(record.FeedbackID)
	if !exists {
		t.Fatal("expected record to exist")
	}
	if got.FeedbackID != record.FeedbackID {
		t.Errorf("got FeedbackID = %s, want %s", got.FeedbackID, record.FeedbackID)
	}
}

func TestMemoryStore_GetByTarget(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add multiple records
	record1 := NewFeedbackRecord(TargetDraft, "draft-123", identity.EntityID("circle-1"), now, SignalHelpful, "")
	record2 := NewFeedbackRecord(TargetDraft, "draft-123", identity.EntityID("circle-1"), now.Add(time.Hour), SignalUnnecessary, "")
	record3 := NewFeedbackRecord(TargetDraft, "draft-456", identity.EntityID("circle-1"), now, SignalHelpful, "")

	store.Put(record1)
	store.Put(record2)
	store.Put(record3)

	// Get feedback for draft-123
	results := store.GetByTarget(TargetDraft, "draft-123")
	if len(results) != 2 {
		t.Errorf("expected 2 records, got %d", len(results))
	}

	// Verify deterministic ordering (by CapturedAt)
	if len(results) >= 2 && !results[0].CapturedAt.Before(results[1].CapturedAt) {
		t.Error("expected records ordered by CapturedAt")
	}
}

func TestMemoryStore_List_Deterministic(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add records in random order
	store.Put(NewFeedbackRecord(TargetDraft, "draft-3", identity.EntityID("circle-1"), now.Add(2*time.Hour), SignalHelpful, ""))
	store.Put(NewFeedbackRecord(TargetDraft, "draft-1", identity.EntityID("circle-1"), now, SignalHelpful, ""))
	store.Put(NewFeedbackRecord(TargetDraft, "draft-2", identity.EntityID("circle-1"), now.Add(time.Hour), SignalHelpful, ""))

	// List should be deterministic
	list1 := store.List()
	list2 := store.List()

	if len(list1) != len(list2) {
		t.Fatal("lists have different lengths")
	}

	for i := range list1 {
		if list1[i].FeedbackID != list2[i].FeedbackID {
			t.Errorf("list not deterministic at index %d", i)
		}
	}

	// Verify sorted by time
	for i := 1; i < len(list1); i++ {
		if list1[i].CapturedAt.Before(list1[i-1].CapturedAt) {
			t.Errorf("list not sorted by CapturedAt at index %d", i)
		}
	}
}

func TestMemoryStore_Stats(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	store.Put(NewFeedbackRecord(TargetInterruption, "int-1", identity.EntityID("circle-1"), now, SignalHelpful, ""))
	store.Put(NewFeedbackRecord(TargetInterruption, "int-2", identity.EntityID("circle-1"), now.Add(time.Hour), SignalUnnecessary, ""))
	store.Put(NewFeedbackRecord(TargetDraft, "draft-1", identity.EntityID("circle-1"), now, SignalHelpful, ""))

	stats := store.Stats()

	if stats.TotalRecords != 3 {
		t.Errorf("TotalRecords = %d, want 3", stats.TotalRecords)
	}
	if stats.InterruptFeedback != 2 {
		t.Errorf("InterruptFeedback = %d, want 2", stats.InterruptFeedback)
	}
	if stats.DraftFeedback != 1 {
		t.Errorf("DraftFeedback = %d, want 1", stats.DraftFeedback)
	}
	if stats.HelpfulCount != 2 {
		t.Errorf("HelpfulCount = %d, want 2", stats.HelpfulCount)
	}
	if stats.UnnecessaryCount != 1 {
		t.Errorf("UnnecessaryCount = %d, want 1", stats.UnnecessaryCount)
	}
}
