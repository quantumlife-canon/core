package interrupt

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestNewInterruptionDeterministicID(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Create same interruption twice
	i1 := NewInterruption(
		identity.EntityID("circle-work"),
		TriggerEmailActionNeeded,
		"event-123",
		"oblig-456",
		75,
		85,
		LevelNotify,
		expires,
		now,
		"Email requires action",
	)

	i2 := NewInterruption(
		identity.EntityID("circle-work"),
		TriggerEmailActionNeeded,
		"event-123",
		"oblig-456",
		75,
		85,
		LevelNotify,
		expires,
		now,
		"Email requires action",
	)

	if i1.InterruptionID != i2.InterruptionID {
		t.Errorf("Expected same ID, got %s vs %s", i1.InterruptionID, i2.InterruptionID)
	}

	if len(i1.InterruptionID) != 16 {
		t.Errorf("Expected 16 char ID, got %d chars: %s", len(i1.InterruptionID), i1.InterruptionID)
	}

	if i1.DedupKey != i2.DedupKey {
		t.Errorf("Expected same DedupKey, got %s vs %s", i1.DedupKey, i2.DedupKey)
	}
}

func TestNewInterruptionDifferentInputsDifferentIDs(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	i1 := NewInterruption("circle-work", TriggerEmailActionNeeded, "event-1", "", 75, 85, LevelNotify, expires, now, "Summary 1")
	i2 := NewInterruption("circle-work", TriggerEmailActionNeeded, "event-2", "", 75, 85, LevelNotify, expires, now, "Summary 1")
	i3 := NewInterruption("circle-family", TriggerEmailActionNeeded, "event-1", "", 75, 85, LevelNotify, expires, now, "Summary 1")
	i4 := NewInterruption("circle-work", TriggerCalendarUpcoming, "event-1", "", 75, 85, LevelNotify, expires, now, "Summary 1")

	ids := map[string]bool{
		i1.InterruptionID: true,
		i2.InterruptionID: true,
		i3.InterruptionID: true,
		i4.InterruptionID: true,
	}

	if len(ids) != 4 {
		t.Errorf("Expected 4 unique IDs, got %d", len(ids))
	}
}

func TestRegretScoreClamping(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Test clamping above 100
	i := NewInterruption("circle-work", TriggerEmailActionNeeded, "e1", "", 150, 200, LevelNotify, expires, now, "Test")
	if i.RegretScore != 100 {
		t.Errorf("Expected regret clamped to 100, got %d", i.RegretScore)
	}
	if i.Confidence != 100 {
		t.Errorf("Expected confidence clamped to 100, got %d", i.Confidence)
	}

	// Test clamping below 0
	i2 := NewInterruption("circle-work", TriggerEmailActionNeeded, "e2", "", -10, -20, LevelSilent, expires, now, "Test")
	if i2.RegretScore != 0 {
		t.Errorf("Expected regret clamped to 0, got %d", i2.RegretScore)
	}
	if i2.Confidence != 0 {
		t.Errorf("Expected confidence clamped to 0, got %d", i2.Confidence)
	}
}

func TestLevelOrder(t *testing.T) {
	tests := []struct {
		level Level
		order int
	}{
		{LevelUrgent, 4},
		{LevelNotify, 3},
		{LevelQueued, 2},
		{LevelAmbient, 1},
		{LevelSilent, 0},
	}

	for _, tt := range tests {
		got := LevelOrder(tt.level)
		if got != tt.order {
			t.Errorf("LevelOrder(%s) = %d, want %d", tt.level, got, tt.order)
		}
	}
}

func TestSortInterruptions(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	interruptions := []*Interruption{
		NewInterruption("c1", TriggerEmailActionNeeded, "e1", "", 50, 80, LevelQueued, now.Add(48*time.Hour), now, "Low priority"),
		NewInterruption("c2", TriggerFinanceLowBalance, "e2", "", 95, 90, LevelUrgent, now.Add(12*time.Hour), now, "Urgent finance"),
		NewInterruption("c3", TriggerCalendarUpcoming, "e3", "", 75, 85, LevelNotify, now.Add(24*time.Hour), now, "Meeting soon"),
		NewInterruption("c4", TriggerEmailActionNeeded, "e4", "", 30, 70, LevelAmbient, now.Add(72*time.Hour), now, "Low regret"),
	}

	SortInterruptions(interruptions)

	// First should be Urgent (highest level)
	if interruptions[0].Level != LevelUrgent {
		t.Errorf("Expected first to be Urgent, got %s", interruptions[0].Level)
	}

	// Second should be Notify
	if interruptions[1].Level != LevelNotify {
		t.Errorf("Expected second to be Notify, got %s", interruptions[1].Level)
	}

	// Third should be Queued
	if interruptions[2].Level != LevelQueued {
		t.Errorf("Expected third to be Queued, got %s", interruptions[2].Level)
	}

	// Fourth should be Ambient
	if interruptions[3].Level != LevelAmbient {
		t.Errorf("Expected fourth to be Ambient, got %s", interruptions[3].Level)
	}
}

func TestSortInterruptionsByRegretWithinLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	interruptions := []*Interruption{
		NewInterruption("c1", TriggerEmailActionNeeded, "e1", "", 60, 80, LevelQueued, now.Add(24*time.Hour), now, "Lower regret"),
		NewInterruption("c2", TriggerEmailActionNeeded, "e2", "", 80, 80, LevelQueued, now.Add(24*time.Hour), now, "Higher regret"),
		NewInterruption("c3", TriggerEmailActionNeeded, "e3", "", 70, 80, LevelQueued, now.Add(24*time.Hour), now, "Medium regret"),
	}

	SortInterruptions(interruptions)

	// All same level, should be sorted by regret descending
	if interruptions[0].RegretScore != 80 {
		t.Errorf("Expected first regret 80, got %d", interruptions[0].RegretScore)
	}
	if interruptions[1].RegretScore != 70 {
		t.Errorf("Expected second regret 70, got %d", interruptions[1].RegretScore)
	}
	if interruptions[2].RegretScore != 60 {
		t.Errorf("Expected third regret 60, got %d", interruptions[2].RegretScore)
	}
}

func TestFilterByLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	interruptions := []*Interruption{
		NewInterruption("c1", TriggerEmailActionNeeded, "e1", "", 20, 80, LevelSilent, now.Add(24*time.Hour), now, "Silent"),
		NewInterruption("c2", TriggerEmailActionNeeded, "e2", "", 30, 80, LevelAmbient, now.Add(24*time.Hour), now, "Ambient"),
		NewInterruption("c3", TriggerEmailActionNeeded, "e3", "", 60, 80, LevelQueued, now.Add(24*time.Hour), now, "Queued"),
		NewInterruption("c4", TriggerEmailActionNeeded, "e4", "", 80, 80, LevelNotify, now.Add(24*time.Hour), now, "Notify"),
		NewInterruption("c5", TriggerEmailActionNeeded, "e5", "", 95, 80, LevelUrgent, now.Add(24*time.Hour), now, "Urgent"),
	}

	// Filter for Queued and above
	result := FilterByLevel(interruptions, LevelQueued)
	if len(result) != 3 {
		t.Errorf("Expected 3 interruptions at Queued+, got %d", len(result))
	}

	// Filter for Notify and above
	result = FilterByLevel(interruptions, LevelNotify)
	if len(result) != 2 {
		t.Errorf("Expected 2 interruptions at Notify+, got %d", len(result))
	}

	// Filter for Urgent only
	result = FilterByLevel(interruptions, LevelUrgent)
	if len(result) != 1 {
		t.Errorf("Expected 1 urgent interruption, got %d", len(result))
	}
}

func TestFilterByCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	interruptions := []*Interruption{
		NewInterruption("circle-work", TriggerEmailActionNeeded, "e1", "", 50, 80, LevelQueued, now.Add(24*time.Hour), now, "Work 1"),
		NewInterruption("circle-work", TriggerEmailActionNeeded, "e2", "", 60, 80, LevelQueued, now.Add(24*time.Hour), now, "Work 2"),
		NewInterruption("circle-family", TriggerCalendarUpcoming, "e3", "", 70, 80, LevelNotify, now.Add(24*time.Hour), now, "Family"),
		NewInterruption("circle-finance", TriggerFinanceLowBalance, "e4", "", 90, 80, LevelUrgent, now.Add(24*time.Hour), now, "Finance"),
	}

	work := FilterByCircle(interruptions, "circle-work")
	if len(work) != 2 {
		t.Errorf("Expected 2 work interruptions, got %d", len(work))
	}

	family := FilterByCircle(interruptions, "circle-family")
	if len(family) != 1 {
		t.Errorf("Expected 1 family interruption, got %d", len(family))
	}
}

func TestCountByLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	interruptions := []*Interruption{
		NewInterruption("c1", TriggerEmailActionNeeded, "e1", "", 20, 80, LevelSilent, now.Add(24*time.Hour), now, "Silent"),
		NewInterruption("c2", TriggerEmailActionNeeded, "e2", "", 30, 80, LevelAmbient, now.Add(24*time.Hour), now, "Ambient"),
		NewInterruption("c3", TriggerEmailActionNeeded, "e3", "", 60, 80, LevelQueued, now.Add(24*time.Hour), now, "Queued 1"),
		NewInterruption("c4", TriggerEmailActionNeeded, "e4", "", 65, 80, LevelQueued, now.Add(24*time.Hour), now, "Queued 2"),
		NewInterruption("c5", TriggerEmailActionNeeded, "e5", "", 80, 80, LevelNotify, now.Add(24*time.Hour), now, "Notify"),
	}

	counts := CountByLevel(interruptions)

	if counts[LevelSilent] != 1 {
		t.Errorf("Expected 1 silent, got %d", counts[LevelSilent])
	}
	if counts[LevelAmbient] != 1 {
		t.Errorf("Expected 1 ambient, got %d", counts[LevelAmbient])
	}
	if counts[LevelQueued] != 2 {
		t.Errorf("Expected 2 queued, got %d", counts[LevelQueued])
	}
	if counts[LevelNotify] != 1 {
		t.Errorf("Expected 1 notify, got %d", counts[LevelNotify])
	}
}

func TestDedupKeyBuckets(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	// Urgent/Notify use hour bucket
	urgent := NewInterruption("circle-work", TriggerFinanceLowBalance, "e1", "", 95, 90, LevelUrgent, expires, now, "Urgent")
	notify := NewInterruption("circle-work", TriggerEmailActionNeeded, "e2", "", 80, 85, LevelNotify, expires, now, "Notify")

	// Queued uses day bucket
	queued := NewInterruption("circle-work", TriggerEmailActionNeeded, "e3", "", 60, 80, LevelQueued, expires, now, "Queued")

	// Check hour bucket format in urgent/notify
	if !contains(urgent.DedupKey, "2025-01-15T10") {
		t.Errorf("Urgent should have hour bucket, got: %s", urgent.DedupKey)
	}
	if !contains(notify.DedupKey, "2025-01-15T10") {
		t.Errorf("Notify should have hour bucket, got: %s", notify.DedupKey)
	}

	// Check day bucket format in queued
	if !contains(queued.DedupKey, "2025-01-15") && contains(queued.DedupKey, "T10") {
		t.Errorf("Queued should have day bucket (no hour), got: %s", queued.DedupKey)
	}
}

func TestCanonicalStringDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(24 * time.Hour)

	i1 := NewInterruption("circle-work", TriggerEmailActionNeeded, "event-123", "oblig-456", 75, 85, LevelNotify, expires, now, "Summary")
	i2 := NewInterruption("circle-work", TriggerEmailActionNeeded, "event-123", "oblig-456", 75, 85, LevelNotify, expires, now, "Summary")

	cs1 := i1.CanonicalString()
	cs2 := i2.CanonicalString()

	if cs1 != cs2 {
		t.Errorf("Canonical strings differ:\n%s\nvs\n%s", cs1, cs2)
	}
}

// Note: contains helper is defined in explain_test.go
