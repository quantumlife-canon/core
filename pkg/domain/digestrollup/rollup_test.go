package digestrollup

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

func TestBuildRollupDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	interruptions := createTestInterruptions(now)

	// Build twice
	result1 := BuildRollup(interruptions)
	result2 := BuildRollup(interruptions)

	if len(result1) != len(result2) {
		t.Fatalf("Length mismatch: %d vs %d", len(result1), len(result2))
	}

	// Same order and content
	for i := range result1 {
		if result1[i].Key != result2[i].Key {
			t.Errorf("Key mismatch at %d: %s vs %s", i, result1[i].Key, result2[i].Key)
		}
		if result1[i].Hash() != result2[i].Hash() {
			t.Errorf("Hash mismatch at %d", i)
		}
	}
}

func TestBuildRollupDeduplication(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Same underlying condition appearing 3 times at different levels
	interruptions := []*interrupt.Interruption{
		createInterruption("circle-finance", "low-balance-event", "Account low", 70, interrupt.LevelQueued, now),
		createInterruption("circle-finance", "low-balance-event", "Account low", 85, interrupt.LevelNotify, now.Add(24*time.Hour)),
		createInterruption("circle-finance", "low-balance-event", "Account low", 95, interrupt.LevelUrgent, now.Add(48*time.Hour)),
	}

	result := BuildRollup(interruptions)

	// Should have exactly 1 rollup item
	if len(result) != 1 {
		t.Fatalf("Expected 1 rollup item, got %d", len(result))
	}

	item := result[0]

	// Should have max level (Urgent)
	if item.MaxLevel != interrupt.LevelUrgent {
		t.Errorf("Expected MaxLevel Urgent, got %s", item.MaxLevel)
	}

	// Should have max regret (95)
	if item.MaxRegret != 95 {
		t.Errorf("Expected MaxRegret 95, got %d", item.MaxRegret)
	}

	// Should have occurrence count 3
	if item.OccurrenceCount != 3 {
		t.Errorf("Expected OccurrenceCount 3, got %d", item.OccurrenceCount)
	}

	// FirstSeen should be earliest
	if !item.FirstSeen.Equal(now) {
		t.Errorf("Expected FirstSeen %v, got %v", now, item.FirstSeen)
	}

	// LastSeen should be latest
	expected := now.Add(48 * time.Hour)
	if !item.LastSeen.Equal(expected) {
		t.Errorf("Expected LastSeen %v, got %v", expected, item.LastSeen)
	}
}

func TestBuildRollupMaxLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Low level first, then high level
	interruptions := []*interrupt.Interruption{
		createInterruption("circle-work", "email-123", "Email needs reply", 50, interrupt.LevelQueued, now),
		createInterruption("circle-work", "email-123", "Email needs reply", 90, interrupt.LevelUrgent, now.Add(time.Hour)),
		createInterruption("circle-work", "email-123", "Email needs reply", 60, interrupt.LevelQueued, now.Add(2*time.Hour)),
	}

	result := BuildRollup(interruptions)

	if len(result) != 1 {
		t.Fatalf("Expected 1 rollup item, got %d", len(result))
	}

	// Max level should be Urgent even though it appeared in the middle
	if result[0].MaxLevel != interrupt.LevelUrgent {
		t.Errorf("Expected MaxLevel Urgent, got %s", result[0].MaxLevel)
	}

	// Level counts should be tracked
	if result[0].LevelCounts[interrupt.LevelQueued] != 2 {
		t.Errorf("Expected 2 Queued occurrences, got %d", result[0].LevelCounts[interrupt.LevelQueued])
	}
	if result[0].LevelCounts[interrupt.LevelUrgent] != 1 {
		t.Errorf("Expected 1 Urgent occurrence, got %d", result[0].LevelCounts[interrupt.LevelUrgent])
	}
}

func TestBuildRollupMultipleConditions(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Two different conditions
	interruptions := []*interrupt.Interruption{
		createInterruption("circle-finance", "low-balance", "Low balance", 90, interrupt.LevelUrgent, now),
		createInterruption("circle-finance", "large-txn", "Large transaction", 70, interrupt.LevelNotify, now),
		createInterruption("circle-finance", "low-balance", "Low balance", 85, interrupt.LevelNotify, now.Add(time.Hour)),
	}

	result := BuildRollup(interruptions)

	// Should have 2 rollup items
	if len(result) != 2 {
		t.Fatalf("Expected 2 rollup items, got %d", len(result))
	}

	// First should be low-balance (Urgent level)
	if result[0].MaxLevel != interrupt.LevelUrgent {
		t.Errorf("First item should be Urgent, got %s", result[0].MaxLevel)
	}
	if result[0].OccurrenceCount != 2 {
		t.Errorf("Low balance should have 2 occurrences, got %d", result[0].OccurrenceCount)
	}

	// Second should be large-txn (Notify level)
	if result[1].MaxLevel != interrupt.LevelNotify {
		t.Errorf("Second item should be Notify, got %s", result[1].MaxLevel)
	}
	if result[1].OccurrenceCount != 1 {
		t.Errorf("Large txn should have 1 occurrence, got %d", result[1].OccurrenceCount)
	}
}

func TestBuildRollupByCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	interruptions := []*interrupt.Interruption{
		createInterruption("circle-finance", "evt-1", "Finance item", 90, interrupt.LevelUrgent, now),
		createInterruption("circle-work", "evt-2", "Work item", 70, interrupt.LevelNotify, now),
		createInterruption("circle-finance", "evt-1", "Finance item", 85, interrupt.LevelNotify, now.Add(time.Hour)),
	}

	result := BuildRollupByCircle(interruptions)

	if len(result) != 2 {
		t.Fatalf("Expected 2 circles, got %d", len(result))
	}

	finance := result["circle-finance"]
	if len(finance) != 1 {
		t.Errorf("Expected 1 finance rollup, got %d", len(finance))
	}
	if finance[0].OccurrenceCount != 2 {
		t.Errorf("Expected 2 finance occurrences, got %d", finance[0].OccurrenceCount)
	}

	work := result["circle-work"]
	if len(work) != 1 {
		t.Errorf("Expected 1 work rollup, got %d", len(work))
	}
}

func TestDigestKeyStability(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Same underlying event at different times should have same key
	intr1 := createInterruption("circle-finance", "evt-123", "Summary", 80, interrupt.LevelNotify, now)
	intr2 := createInterruption("circle-finance", "evt-123", "Summary", 90, interrupt.LevelUrgent, now.Add(24*time.Hour))

	key1 := ComputeDigestKey(intr1)
	key2 := ComputeDigestKey(intr2)

	if key1 != key2 {
		t.Errorf("Same event should have same digest key: %s vs %s", key1, key2)
	}

	// Different events should have different keys
	intr3 := createInterruption("circle-finance", "evt-456", "Summary", 80, interrupt.LevelNotify, now)
	key3 := ComputeDigestKey(intr3)

	if key1 == key3 {
		t.Errorf("Different events should have different keys")
	}
}

func TestSortRollupItems(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	items := []RollupItem{
		{Key: "a", MaxLevel: interrupt.LevelQueued, MaxRegret: 50, LastSeen: now},
		{Key: "b", MaxLevel: interrupt.LevelUrgent, MaxRegret: 90, LastSeen: now},
		{Key: "c", MaxLevel: interrupt.LevelNotify, MaxRegret: 75, LastSeen: now},
		{Key: "d", MaxLevel: interrupt.LevelUrgent, MaxRegret: 95, LastSeen: now},
	}

	SortRollupItems(items)

	// First should be highest level + highest regret
	if items[0].Key != "d" {
		t.Errorf("First should be 'd' (Urgent, 95), got %s", items[0].Key)
	}
	if items[1].Key != "b" {
		t.Errorf("Second should be 'b' (Urgent, 90), got %s", items[1].Key)
	}
	if items[2].Key != "c" {
		t.Errorf("Third should be 'c' (Notify, 75), got %s", items[2].Key)
	}
	if items[3].Key != "a" {
		t.Errorf("Fourth should be 'a' (Queued, 50), got %s", items[3].Key)
	}
}

func TestFormatOccurrence(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{1, ""},
		{2, "x2"},
		{3, "x3"},
		{10, "x10"},
	}

	for _, tt := range tests {
		item := RollupItem{OccurrenceCount: tt.count}
		got := item.FormatOccurrence()
		if got != tt.expected {
			t.Errorf("FormatOccurrence(%d) = %q, want %q", tt.count, got, tt.expected)
		}
	}
}

func TestTrendIndicator(t *testing.T) {
	tests := []struct {
		name     string
		counts   map[interrupt.Level]int
		maxLevel interrupt.Level
		total    int
		expected string
	}{
		{
			name:     "single occurrence",
			counts:   map[interrupt.Level]int{interrupt.LevelNotify: 1},
			maxLevel: interrupt.LevelNotify,
			total:    1,
			expected: "",
		},
		{
			name:     "all same level",
			counts:   map[interrupt.Level]int{interrupt.LevelNotify: 3},
			maxLevel: interrupt.LevelNotify,
			total:    3,
			expected: "→",
		},
		{
			name:     "escalated to max",
			counts:   map[interrupt.Level]int{interrupt.LevelQueued: 2, interrupt.LevelUrgent: 1},
			maxLevel: interrupt.LevelUrgent,
			total:    3,
			expected: "↑",
		},
		{
			name:     "mostly at max",
			counts:   map[interrupt.Level]int{interrupt.LevelQueued: 1, interrupt.LevelUrgent: 2},
			maxLevel: interrupt.LevelUrgent,
			total:    3,
			expected: "↗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := RollupItem{
				LevelCounts:     tt.counts,
				MaxLevel:        tt.maxLevel,
				OccurrenceCount: tt.total,
			}
			got := item.TrendIndicator()
			if got != tt.expected {
				t.Errorf("TrendIndicator() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCleanSummary(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal summary", "Normal summary"},
		{"Downgraded item (quota)", "Downgraded item"},
		{"Short", "Short"},
		{"(quota)", "(quota)"}, // Too short to have prefix
	}

	for _, tt := range tests {
		got := cleanSummary(tt.input)
		if got != tt.expected {
			t.Errorf("cleanSummary(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTopNByCircle(t *testing.T) {
	rollups := map[identity.EntityID][]RollupItem{
		"circle-work": {
			{Key: "a", MaxLevel: interrupt.LevelUrgent},
			{Key: "b", MaxLevel: interrupt.LevelNotify},
			{Key: "c", MaxLevel: interrupt.LevelQueued},
			{Key: "d", MaxLevel: interrupt.LevelAmbient},
		},
		"circle-finance": {
			{Key: "e", MaxLevel: interrupt.LevelUrgent},
		},
	}

	result := TopNByCircle(rollups, 2)

	if len(result["circle-work"]) != 2 {
		t.Errorf("Expected 2 work items, got %d", len(result["circle-work"]))
	}
	if len(result["circle-finance"]) != 1 {
		t.Errorf("Expected 1 finance item, got %d", len(result["circle-finance"]))
	}
}

// Helper functions

func createTestInterruptions(now time.Time) []*interrupt.Interruption {
	return []*interrupt.Interruption{
		createInterruption("circle-finance", "evt-1", "Low balance", 90, interrupt.LevelUrgent, now),
		createInterruption("circle-work", "evt-2", "Email reply needed", 70, interrupt.LevelNotify, now),
		createInterruption("circle-finance", "evt-1", "Low balance", 85, interrupt.LevelNotify, now.Add(time.Hour)),
	}
}

func createInterruption(circleID, sourceEventID, summary string, regret int, level interrupt.Level, createdAt time.Time) *interrupt.Interruption {
	return interrupt.NewInterruption(
		identity.EntityID(circleID),
		interrupt.TriggerFinanceLowBalance,
		sourceEventID,
		"",
		regret,
		80,
		level,
		createdAt.Add(24*time.Hour),
		createdAt,
		summary,
	)
}
