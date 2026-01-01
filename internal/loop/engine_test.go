package loop

import (
	"context"
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/events"
)

// mockClock implements clock.Clock for testing.
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time { return m.now }

var _ clock.Clock = (*mockClock)(nil)

// mockEventEmitter implements events.Emitter for testing.
type mockEventEmitter struct {
	events []events.Event
}

func (m *mockEventEmitter) Emit(event events.Event) {
	m.events = append(m.events, event)
}

var _ events.Emitter = (*mockEventEmitter)(nil)

// mockIdentityRepo implements identity.Repository for testing.
type mockIdentityRepo struct {
	circles []*identity.Circle
}

func (m *mockIdentityRepo) Store(entity identity.Entity) error { return nil }

func (m *mockIdentityRepo) Get(id identity.EntityID) (identity.Entity, error) {
	for _, c := range m.circles {
		if c.ID() == id {
			return c, nil
		}
	}
	return nil, identity.ErrEntityNotFound
}

func (m *mockIdentityRepo) GetByType(entityType identity.EntityType) ([]identity.Entity, error) {
	if entityType != identity.EntityTypeCircle {
		return nil, nil
	}
	result := make([]identity.Entity, len(m.circles))
	for i, c := range m.circles {
		result[i] = c
	}
	return result, nil
}

func (m *mockIdentityRepo) Exists(id identity.EntityID) bool {
	for _, c := range m.circles {
		if c.ID() == id {
			return true
		}
	}
	return false
}

func (m *mockIdentityRepo) Delete(id identity.EntityID) error              { return nil }
func (m *mockIdentityRepo) Count() int                                     { return len(m.circles) }
func (m *mockIdentityRepo) CountByType(entityType identity.EntityType) int { return len(m.circles) }
func (m *mockIdentityRepo) List(page, perPage int) ([]identity.Entity, int, error) {
	return nil, 0, nil
}

var _ identity.Repository = (*mockIdentityRepo)(nil)

func createTestCircle(name string, t time.Time) *identity.Circle {
	gen := identity.NewGenerator()
	return gen.CircleFromName("owner-1", name, t)
}

func TestEngine_Run_Deterministic(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create circles using identity package
	circle1 := createTestCircle("Personal", fixedTime)
	circle2 := createTestCircle("Work", fixedTime)

	// Create engine with fixed clock
	engine := &Engine{
		Clock: &mockClock{now: fixedTime},
		IdentityRepo: &mockIdentityRepo{
			circles: []*identity.Circle{circle1, circle2},
		},
		DraftStore:    draft.NewInMemoryStore(),
		FeedbackStore: feedback.NewMemoryStore(),
		EventEmitter:  &mockEventEmitter{},
	}

	opts := RunOptions{}

	// Run twice
	result1 := engine.Run(context.Background(), opts)
	result2 := engine.Run(context.Background(), opts)

	// Run IDs should be identical
	if result1.RunID != result2.RunID {
		t.Errorf("RunID not deterministic: %s != %s", result1.RunID, result2.RunID)
	}

	// NeedsYou hash should be identical
	if result1.NeedsYou.Hash != result2.NeedsYou.Hash {
		t.Errorf("NeedsYou.Hash not deterministic: %s != %s", result1.NeedsYou.Hash, result2.NeedsYou.Hash)
	}
}

func TestEngine_Run_EmptyState(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	circle := createTestCircle("Personal", fixedTime)

	engine := &Engine{
		Clock: &mockClock{now: fixedTime},
		IdentityRepo: &mockIdentityRepo{
			circles: []*identity.Circle{circle},
		},
		DraftStore:    draft.NewInMemoryStore(),
		FeedbackStore: feedback.NewMemoryStore(),
		EventEmitter:  &mockEventEmitter{},
	}

	result := engine.Run(context.Background(), RunOptions{})

	// Should be quiet (no pending items)
	if !result.NeedsYou.IsQuiet {
		t.Error("expected IsQuiet to be true with no pending items")
	}

	if result.NeedsYou.TotalItems != 0 {
		t.Errorf("expected TotalItems=0, got %d", result.NeedsYou.TotalItems)
	}
}

func TestEngine_Run_WithPendingDrafts(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	draftStore := draft.NewInMemoryStore()

	circle := createTestCircle("Personal", fixedTime)

	// Add a pending draft - use the circle's actual ID
	testDraft := draft.Draft{
		DraftID:   "draft-123",
		DraftType: draft.DraftTypeEmailReply,
		CircleID:  circle.ID(),
		Status:    draft.StatusProposed,
		CreatedAt: fixedTime,
		ExpiresAt: fixedTime.Add(24 * time.Hour),
	}
	draftStore.Put(testDraft)

	engine := &Engine{
		Clock: &mockClock{now: fixedTime},
		IdentityRepo: &mockIdentityRepo{
			circles: []*identity.Circle{circle},
		},
		DraftStore:    draftStore,
		FeedbackStore: feedback.NewMemoryStore(),
		EventEmitter:  &mockEventEmitter{},
	}

	result := engine.Run(context.Background(), RunOptions{})

	// Should NOT be quiet
	if result.NeedsYou.IsQuiet {
		t.Error("expected IsQuiet to be false with pending draft")
	}

	if result.NeedsYou.TotalItems != 1 {
		t.Errorf("expected TotalItems=1, got %d", result.NeedsYou.TotalItems)
	}

	if len(result.NeedsYou.PendingDrafts) != 1 {
		t.Errorf("expected 1 pending draft, got %d", len(result.NeedsYou.PendingDrafts))
	}
}

func TestEngine_RecordFeedback(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	emitter := &mockEventEmitter{}

	engine := &Engine{
		Clock:         &mockClock{now: fixedTime},
		FeedbackStore: feedback.NewMemoryStore(),
		EventEmitter:  emitter,
	}

	record, err := engine.RecordFeedback(
		feedback.TargetDraft,
		"draft-123",
		identity.EntityID("circle-1"),
		feedback.SignalHelpful,
		"Good suggestion",
	)

	if err != nil {
		t.Fatalf("RecordFeedback error: %v", err)
	}

	if record.FeedbackID == "" {
		t.Error("expected non-empty FeedbackID")
	}

	// Verify event was emitted
	found := false
	for _, e := range emitter.events {
		if e.Type == events.Phase6FeedbackRecorded {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Phase6FeedbackRecorded event")
	}

	// Verify stored
	stored, exists := engine.FeedbackStore.Get(record.FeedbackID)
	if !exists {
		t.Error("expected feedback to be stored")
	}
	if stored.Signal != feedback.SignalHelpful {
		t.Errorf("expected SignalHelpful, got %s", stored.Signal)
	}
}

func TestComputeRunID_Deterministic(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	opts := RunOptions{CircleID: "circle-1"}

	id1 := computeRunID(now, opts)
	id2 := computeRunID(now, opts)

	if id1 != id2 {
		t.Errorf("RunID not deterministic: %s != %s", id1, id2)
	}

	// Different options should give different ID
	opts2 := RunOptions{CircleID: "circle-2"}
	id3 := computeRunID(now, opts2)

	if id1 == id3 {
		t.Error("expected different RunID for different options")
	}
}

func TestComputeNeedsYouHash_Deterministic(t *testing.T) {
	drafts := []draft.Draft{
		{DraftID: "draft-2"},
		{DraftID: "draft-1"},
	}
	interrupts := []*interrupt.Interruption{
		{InterruptionID: "int-2"},
		{InterruptionID: "int-1"},
	}

	hash1 := computeNeedsYouHash(drafts, interrupts)
	hash2 := computeNeedsYouHash(drafts, interrupts)

	if hash1 != hash2 {
		t.Errorf("hash not deterministic: %s != %s", hash1, hash2)
	}

	// Order shouldn't matter (sorted internally)
	draftsReordered := []draft.Draft{
		{DraftID: "draft-1"},
		{DraftID: "draft-2"},
	}
	hash3 := computeNeedsYouHash(draftsReordered, interrupts)

	if hash1 != hash3 {
		t.Error("hash should be same regardless of input order")
	}
}
