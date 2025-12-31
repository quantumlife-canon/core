package digest

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
)

func TestGeneratorDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC) // A Monday
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC) // Previous Monday

	buckets := createTestBuckets(weekStart)
	circleNames := map[identity.EntityID]string{
		"circle-work":    "Work",
		"circle-family":  "Family",
		"circle-finance": "Finance",
	}

	// Generate twice
	gen := NewGenerator(clk)
	digest1 := gen.Generate(weekStart, buckets, circleNames)
	digest2 := gen.Generate(weekStart, buckets, circleNames)

	// Hash must be identical
	if digest1.Hash != digest2.Hash {
		t.Errorf("Hash mismatch: %s vs %s", digest1.Hash, digest2.Hash)
	}

	// Same counts
	if digest1.TotalInterruptions != digest2.TotalInterruptions {
		t.Errorf("Total mismatch: %d vs %d", digest1.TotalInterruptions, digest2.TotalInterruptions)
	}
}

func TestGeneratorCircleSummaries(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	buckets := createTestBuckets(weekStart)
	circleNames := map[identity.EntityID]string{
		"circle-work":    "Work",
		"circle-family":  "Family",
		"circle-finance": "Finance",
	}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	// Should have 3 circles
	if len(digest.CircleSummaries) != 3 {
		t.Errorf("Expected 3 circles, got %d", len(digest.CircleSummaries))
	}

	// Check work circle
	work, ok := digest.CircleSummaries["circle-work"]
	if !ok {
		t.Fatal("Expected circle-work summary")
	}
	if work.CircleName != "Work" {
		t.Errorf("Expected name 'Work', got %s", work.CircleName)
	}
	if work.Total < 1 {
		t.Errorf("Expected work total >= 1, got %d", work.Total)
	}
}

func TestGeneratorLevelCounts(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	buckets := createMixedLevelBuckets(weekStart)
	circleNames := map[identity.EntityID]string{"circle-work": "Work"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	// Should have counts for each level we added
	if digest.ByLevel[interrupt.LevelUrgent] != 1 {
		t.Errorf("Expected 1 urgent, got %d", digest.ByLevel[interrupt.LevelUrgent])
	}
	if digest.ByLevel[interrupt.LevelNotify] != 2 {
		t.Errorf("Expected 2 notify, got %d", digest.ByLevel[interrupt.LevelNotify])
	}
	if digest.ByLevel[interrupt.LevelQueued] != 3 {
		t.Errorf("Expected 3 queued, got %d", digest.ByLevel[interrupt.LevelQueued])
	}
}

func TestGeneratorTopItems(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Create buckets with various regret scores
	day := weekStart
	buckets := []DailyBucket{{
		Date: day,
		Interruptions: []*interrupt.Interruption{
			createInterruption("circle-work", "Low item", 30, interrupt.LevelQueued, day),
			createInterruption("circle-work", "High item", 90, interrupt.LevelUrgent, day),
			createInterruption("circle-work", "Medium item", 60, interrupt.LevelQueued, day),
			createInterruption("circle-work", "Very high", 95, interrupt.LevelUrgent, day),
			createInterruption("circle-work", "Another medium", 55, interrupt.LevelQueued, day),
		},
	}}

	circleNames := map[identity.EntityID]string{"circle-work": "Work"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	work := digest.CircleSummaries["circle-work"]
	if work == nil {
		t.Fatal("Expected work summary")
	}

	// Should have max 3 top items
	if len(work.TopItems) != 3 {
		t.Errorf("Expected 3 top items, got %d", len(work.TopItems))
	}

	// Should be sorted by regret descending
	if work.TopItems[0].RegretScore != 95 {
		t.Errorf("Expected first item regret 95, got %d", work.TopItems[0].RegretScore)
	}
	if work.TopItems[1].RegretScore != 90 {
		t.Errorf("Expected second item regret 90, got %d", work.TopItems[1].RegretScore)
	}
}

func TestGeneratorObservations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Create empty buckets
	buckets := []DailyBucket{{
		Date:          weekStart,
		Interruptions: []*interrupt.Interruption{},
	}}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, nil)

	// Empty digest should have no observations (or light week observation)
	if digest.TotalInterruptions != 0 {
		t.Errorf("Expected 0 interruptions, got %d", digest.TotalInterruptions)
	}
}

func TestGeneratorHighUrgentObservation(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Create buckets with many urgent items
	day := weekStart
	var interruptions []*interrupt.Interruption
	for i := 0; i < 7; i++ {
		interruptions = append(interruptions,
			createInterruption("circle-work", "Urgent item", 95, interrupt.LevelUrgent, day))
	}

	buckets := []DailyBucket{{Date: day, Interruptions: interruptions}}
	circleNames := map[identity.EntityID]string{"circle-work": "Work"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	// Should have "high urgent" observation
	found := false
	for _, obs := range digest.Observations {
		if strings.Contains(obs, "High urgent count") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'High urgent count' observation")
	}
}

func TestGeneratorNoUrgentObservation(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Create buckets with only queued items (no urgent)
	day := weekStart
	buckets := []DailyBucket{{
		Date: day,
		Interruptions: []*interrupt.Interruption{
			createInterruption("circle-work", "Queued 1", 50, interrupt.LevelQueued, day),
			createInterruption("circle-work", "Queued 2", 55, interrupt.LevelQueued, day),
			createInterruption("circle-work", "Queued 3", 60, interrupt.LevelQueued, day),
		},
	}}
	circleNames := map[identity.EntityID]string{"circle-work": "Work"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	// Should have "no urgent" observation
	found := false
	for _, obs := range digest.Observations {
		if strings.Contains(obs, "No urgent") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'No urgent items' observation")
	}
}

func TestDigestFormatText(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	buckets := createTestBuckets(weekStart)
	circleNames := map[identity.EntityID]string{
		"circle-work":    "Work",
		"circle-family":  "Family",
		"circle-finance": "Finance",
	}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	text := digest.FormatText()

	// Should contain header
	if !strings.Contains(text, "Weekly Digest") {
		t.Error("Expected 'Weekly Digest' header")
	}

	// Should contain dates
	if !strings.Contains(text, "Jan 13") {
		t.Error("Expected week start date")
	}

	// Should contain circle names
	if !strings.Contains(text, "Work") {
		t.Error("Expected 'Work' circle")
	}

	// Should contain total
	if !strings.Contains(text, "Total Interruptions") {
		t.Error("Expected 'Total Interruptions'")
	}
}

func TestEmptyDigestHash(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	buckets := []DailyBucket{}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, nil)

	if digest.Hash != "empty" {
		t.Errorf("Expected 'empty' hash for empty digest, got %s", digest.Hash)
	}
}

// Phase 3.1: Test deduplication - same condition 3 times = ONE line with x3
func TestDigestRollupDeduplication(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Same underlying condition (same event ID) appearing 3 times at different levels
	buckets := []DailyBucket{
		{
			Date: weekStart,
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-finance", "low-balance-evt", "Account low", 70, interrupt.LevelQueued, weekStart),
			},
		},
		{
			Date: weekStart.AddDate(0, 0, 1),
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-finance", "low-balance-evt", "Account low", 85, interrupt.LevelNotify, weekStart.AddDate(0, 0, 1)),
			},
		},
		{
			Date: weekStart.AddDate(0, 0, 2),
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-finance", "low-balance-evt", "Account low", 95, interrupt.LevelUrgent, weekStart.AddDate(0, 0, 2)),
			},
		},
	}

	circleNames := map[identity.EntityID]string{"circle-finance": "Finance"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	// Should have 3 total interruptions (raw count)
	if digest.TotalInterruptions != 3 {
		t.Errorf("Expected 3 total interruptions, got %d", digest.TotalInterruptions)
	}

	// But only 1 unique condition
	finance := digest.CircleSummaries["circle-finance"]
	if finance == nil {
		t.Fatal("Expected finance summary")
	}

	if finance.UniqueConditions != 1 {
		t.Errorf("Expected 1 unique condition, got %d", finance.UniqueConditions)
	}

	// Should have exactly 1 rollup item
	if len(finance.RollupItems) != 1 {
		t.Fatalf("Expected 1 rollup item, got %d", len(finance.RollupItems))
	}

	rollup := finance.RollupItems[0]

	// Max level should be Urgent
	if rollup.MaxLevel != interrupt.LevelUrgent {
		t.Errorf("Expected MaxLevel Urgent, got %s", rollup.MaxLevel)
	}

	// Max regret should be 95
	if rollup.MaxRegret != 95 {
		t.Errorf("Expected MaxRegret 95, got %d", rollup.MaxRegret)
	}

	// Occurrence count should be 3
	if rollup.OccurrenceCount != 3 {
		t.Errorf("Expected OccurrenceCount 3, got %d", rollup.OccurrenceCount)
	}

	// Format should show "x3"
	if rollup.FormatOccurrence() != "x3" {
		t.Errorf("Expected 'x3', got %q", rollup.FormatOccurrence())
	}

	// FormatText should show ONE line with x3
	text := digest.FormatText()
	if !strings.Contains(text, "x3") {
		t.Error("Expected digest text to contain 'x3' for repeated item")
	}

	// Should NOT have 3 separate lines for "Account low"
	count := strings.Count(text, "Account low")
	if count != 1 {
		t.Errorf("Expected 1 'Account low' line, got %d", count)
	}
}

// Phase 3.1: Test that different conditions are NOT rolled up together
func TestDigestRollupDistinctConditions(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Two different conditions
	buckets := []DailyBucket{
		{
			Date: weekStart,
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-finance", "low-balance", "Low balance", 90, interrupt.LevelUrgent, weekStart),
				createInterruptionWithEvent("circle-finance", "large-txn", "Large transaction", 70, interrupt.LevelNotify, weekStart),
			},
		},
	}

	circleNames := map[identity.EntityID]string{"circle-finance": "Finance"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	finance := digest.CircleSummaries["circle-finance"]
	if finance == nil {
		t.Fatal("Expected finance summary")
	}

	// Should have 2 unique conditions
	if finance.UniqueConditions != 2 {
		t.Errorf("Expected 2 unique conditions, got %d", finance.UniqueConditions)
	}

	// Should have 2 rollup items
	if len(finance.RollupItems) != 2 {
		t.Errorf("Expected 2 rollup items, got %d", len(finance.RollupItems))
	}

	// Each should have occurrence count 1 (no "x2" etc)
	for _, item := range finance.RollupItems {
		if item.OccurrenceCount != 1 {
			t.Errorf("Expected OccurrenceCount 1, got %d", item.OccurrenceCount)
		}
		if item.FormatOccurrence() != "" {
			t.Errorf("Expected empty occurrence format for count 1, got %q", item.FormatOccurrence())
		}
	}
}

// Phase 3.1: Test trend indicators
func TestDigestRollupTrendIndicators(t *testing.T) {
	fixedTime := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)

	// Same condition escalating from Queued to Urgent (more at lower levels)
	buckets := []DailyBucket{
		{
			Date: weekStart,
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-work", "deadline", "Deadline approaching", 50, interrupt.LevelQueued, weekStart),
			},
		},
		{
			Date: weekStart.AddDate(0, 0, 1),
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-work", "deadline", "Deadline approaching", 55, interrupt.LevelQueued, weekStart.AddDate(0, 0, 1)),
			},
		},
		{
			Date: weekStart.AddDate(0, 0, 2),
			Interruptions: []*interrupt.Interruption{
				createInterruptionWithEvent("circle-work", "deadline", "Deadline approaching", 95, interrupt.LevelUrgent, weekStart.AddDate(0, 0, 2)),
			},
		},
	}

	circleNames := map[identity.EntityID]string{"circle-work": "Work"}

	gen := NewGenerator(clk)
	digest := gen.Generate(weekStart, buckets, circleNames)

	work := digest.CircleSummaries["circle-work"]
	if work == nil {
		t.Fatal("Expected work summary")
	}

	if len(work.RollupItems) != 1 {
		t.Fatalf("Expected 1 rollup item, got %d", len(work.RollupItems))
	}

	rollup := work.RollupItems[0]

	// Should have escalation indicator (more at lower levels than max)
	trend := rollup.TrendIndicator()
	if trend != "↑" {
		t.Errorf("Expected '↑' trend (escalated), got %q", trend)
	}

	// FormatText should include the trend
	text := digest.FormatText()
	if !strings.Contains(text, "↑") {
		t.Error("Expected digest text to contain '↑' trend indicator")
	}
}

// Helper functions

func createTestBuckets(weekStart time.Time) []DailyBucket {
	var buckets []DailyBucket

	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		buckets = append(buckets, DailyBucket{
			Date: day,
			Interruptions: []*interrupt.Interruption{
				createInterruption("circle-work", "Work item", 60, interrupt.LevelQueued, day),
				createInterruption("circle-family", "Family item", 70, interrupt.LevelNotify, day),
				createInterruption("circle-finance", "Finance item", 80, interrupt.LevelNotify, day),
			},
		})
	}

	return buckets
}

func createMixedLevelBuckets(weekStart time.Time) []DailyBucket {
	day := weekStart
	return []DailyBucket{{
		Date: day,
		Interruptions: []*interrupt.Interruption{
			createInterruption("circle-work", "Urgent", 95, interrupt.LevelUrgent, day),
			createInterruption("circle-work", "Notify 1", 80, interrupt.LevelNotify, day),
			createInterruption("circle-work", "Notify 2", 75, interrupt.LevelNotify, day),
			createInterruption("circle-work", "Queued 1", 60, interrupt.LevelQueued, day),
			createInterruption("circle-work", "Queued 2", 55, interrupt.LevelQueued, day),
			createInterruption("circle-work", "Queued 3", 50, interrupt.LevelQueued, day),
		},
	}}
}

func createInterruption(circleID, summary string, regret int, level interrupt.Level, createdAt time.Time) *interrupt.Interruption {
	return interrupt.NewInterruption(
		identity.EntityID(circleID),
		interrupt.TriggerObligationDueSoon,
		"event-test",
		"",
		regret,
		80,
		level,
		createdAt.Add(24*time.Hour),
		createdAt,
		summary,
	)
}

// createInterruptionWithEvent creates an interruption with a specific event ID for rollup testing.
func createInterruptionWithEvent(circleID, eventID, summary string, regret int, level interrupt.Level, createdAt time.Time) *interrupt.Interruption {
	return interrupt.NewInterruption(
		identity.EntityID(circleID),
		interrupt.TriggerFinanceLowBalance,
		eventID,
		"",
		regret,
		80,
		level,
		createdAt.Add(24*time.Hour),
		createdAt,
		summary,
	)
}
