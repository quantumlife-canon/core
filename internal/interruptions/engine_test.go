package interruptions

import (
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/domain/view"
)

func TestEngineDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	obligations := createTestObligations(fixedTime)
	dailyView := createTestDailyView(fixedTime)

	// Run twice with fresh stores
	engine1 := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), NewInMemoryQuotaStore())
	result1 := engine1.Process(dailyView, obligations)

	engine2 := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), NewInMemoryQuotaStore())
	result2 := engine2.Process(dailyView, obligations)

	// Hash must be identical
	if result1.Hash != result2.Hash {
		t.Errorf("Hash mismatch: %s vs %s", result1.Hash, result2.Hash)
	}

	// Same number of interruptions
	if len(result1.Interruptions) != len(result2.Interruptions) {
		t.Errorf("Count mismatch: %d vs %d", len(result1.Interruptions), len(result2.Interruptions))
	}

	// Same IDs in same order
	for i := range result1.Interruptions {
		if result1.Interruptions[i].InterruptionID != result2.Interruptions[i].InterruptionID {
			t.Errorf("Interruption[%d] ID mismatch", i)
		}
	}
}

func TestEngineRegretScoring(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), NewInMemoryQuotaStore())

	// Scoring formula:
	// - Base: finance=30, family=25, work=15, health=20, home=10
	// - DueIn24h: +30, DueIn7d: +15
	// - ActionNeeded (reply/pay/decide or high/critical): +15
	// - Obligation regret * 30
	// - Severity: critical=+20, high=+10

	tests := []struct {
		name          string
		circleID      string
		sourceType    string
		dueIn         time.Duration
		obligType     obligation.ObligationType
		severity      obligation.Severity
		obligRegret   float64
		minRegret     int
		expectedLevel interrupt.Level
	}{
		{
			// finance(30) + due24h(30) + high(10) + actionNeeded(15) + obligRegret(0.9*30=27) = 100+ -> capped 100
			name:          "finance due soon high urgency",
			circleID:      "circle-finance",
			sourceType:    "finance",
			dueIn:         2 * time.Hour,
			obligType:     obligation.ObligationPay, // ActionNeeded
			severity:      obligation.SeverityCritical,
			obligRegret:   0.9,
			minRegret:     90,
			expectedLevel: interrupt.LevelUrgent,
		},
		{
			// work(15) + due7d(15) + high(10) + actionNeeded(15) + obligRegret(0.5*30=15) = 70
			name:          "work email due in week",
			circleID:      "circle-work",
			sourceType:    "email",
			dueIn:         3 * 24 * time.Hour,         // 3 days - within 7d
			obligType:     obligation.ObligationReply, // ActionNeeded
			severity:      obligation.SeverityHigh,
			obligRegret:   0.5,
			minRegret:     50,
			expectedLevel: interrupt.LevelQueued, // 70 >= 50 but not notify (needs 75 + due48h)
		},
		{
			// home(10) + no due boost + low = 10
			name:          "home low priority far future",
			circleID:      "circle-home",
			sourceType:    "email",
			dueIn:         10 * 24 * time.Hour,
			obligType:     obligation.ObligationFollowup,
			severity:      obligation.SeverityLow,
			obligRegret:   0.1,
			minRegret:     10,
			expectedLevel: interrupt.LevelSilent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dueBy := fixedTime.Add(tt.dueIn)
			oblig := obligation.NewObligation(
				identity.EntityID(tt.circleID),
				"event-test",
				tt.sourceType,
				tt.obligType,
				fixedTime,
			).WithDueBy(dueBy, fixedTime).WithSeverity(tt.severity).WithScoring(tt.obligRegret, 0.8)

			dailyView := createTestDailyView(fixedTime)
			result := engine.Process(dailyView, []*obligation.Obligation{oblig})

			if len(result.Interruptions) != 1 {
				t.Fatalf("Expected 1 interruption, got %d", len(result.Interruptions))
			}

			intr := result.Interruptions[0]
			if intr.RegretScore < tt.minRegret {
				t.Errorf("Expected regret >= %d, got %d", tt.minRegret, intr.RegretScore)
			}

			if intr.Level != tt.expectedLevel {
				t.Errorf("Expected level %s, got %s (regret=%d)", tt.expectedLevel, intr.Level, intr.RegretScore)
			}
		})
	}
}

func TestEngineDedup(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	// Use shared dedup store
	dedupStore := NewInMemoryDeduper()
	engine := NewEngine(DefaultConfig(), clk, dedupStore, NewInMemoryQuotaStore())

	obligations := createTestObligations(fixedTime)
	dailyView := createTestDailyView(fixedTime)

	// First run
	result1 := engine.Process(dailyView, obligations)
	count1 := len(result1.Interruptions)

	// Second run with same dedup store should drop duplicates
	result2 := engine.Process(dailyView, obligations)
	count2 := len(result2.Interruptions)

	if count2 >= count1 {
		t.Errorf("Expected dedup to reduce count, got %d then %d", count1, count2)
	}

	if result2.Report.DedupDropped == 0 {
		t.Error("Expected dedup dropped count > 0")
	}
}

func TestEngineDedupDifferentBuckets(t *testing.T) {
	// Different hours should have different buckets for urgent items
	time1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	dedupStore := NewInMemoryDeduper()

	// First run at 10:00
	clk1 := clock.NewFixed(time1)
	engine1 := NewEngine(DefaultConfig(), clk1, dedupStore, NewInMemoryQuotaStore())

	// Create urgent obligation (uses hour bucket)
	dueBy := time1.Add(2 * time.Hour)
	oblig := obligation.NewObligation(
		"circle-finance",
		"event-urgent",
		"finance",
		obligation.ObligationReview,
		time1,
	).WithDueBy(dueBy, time1).WithSeverity(obligation.SeverityCritical).WithScoring(0.9, 0.9)

	dailyView := createTestDailyView(time1)
	result1 := engine1.Process(dailyView, []*obligation.Obligation{oblig})

	// Second run at 11:00 (different hour bucket)
	clk2 := clock.NewFixed(time2)
	dueBy2 := time2.Add(2 * time.Hour)
	oblig2 := obligation.NewObligation(
		"circle-finance",
		"event-urgent",
		"finance",
		obligation.ObligationReview,
		time2,
	).WithDueBy(dueBy2, time2).WithSeverity(obligation.SeverityCritical).WithScoring(0.9, 0.9)

	engine2 := NewEngine(DefaultConfig(), clk2, dedupStore, NewInMemoryQuotaStore())
	dailyView2 := createTestDailyView(time2)
	result2 := engine2.Process(dailyView2, []*obligation.Obligation{oblig2})

	// Both should produce interruptions (different hour buckets)
	if len(result1.Interruptions) != 1 || len(result2.Interruptions) != 1 {
		t.Errorf("Expected 1 interruption each, got %d and %d", len(result1.Interruptions), len(result2.Interruptions))
	}
}

func TestEngineQuota(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	// Work quota is 2/day
	quotaStore := NewInMemoryQuotaStore()
	engine := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), quotaStore)

	// Create 4 NOTIFY-level work obligations (not Urgent!)
	// For Notify: need regret >= 75 AND due within 48h (but NOT regret >= 90 AND due within 24h)
	// Key: Due between 24-48h to avoid Urgent threshold
	// work(15) + due7d(15) + actionNeeded(15) + critical(20) + obligRegret(0.8*30=24) = 89
	// 89 >= 75 AND within 48h → NOTIFY ✓
	// NOT (89 >= 90 AND within 24h) → NOT URGENT ✓
	var obligations []*obligation.Obligation
	eventIDs := []string{"event-a", "event-b", "event-c", "event-d"}
	for i := 0; i < 4; i++ {
		// Due between 30-36 hours (within 48h but outside 24h)
		dueBy := fixedTime.Add(time.Duration(30+i) * time.Hour)
		oblig := obligation.NewObligation(
			"circle-work",
			eventIDs[i],
			"email",
			obligation.ObligationReply, // ActionNeeded
			fixedTime,
		).WithDueBy(dueBy, fixedTime).WithSeverity(obligation.SeverityCritical).WithScoring(0.8, 0.85)
		obligations = append(obligations, oblig)
	}

	dailyView := createTestDailyView(fixedTime)
	result := engine.Process(dailyView, obligations)

	// Count notify+urgent levels
	notifyPlusCount := 0
	for _, intr := range result.Interruptions {
		if intr.Level == interrupt.LevelNotify || intr.Level == interrupt.LevelUrgent {
			notifyPlusCount++
		}
	}

	// Work quota is 2, so at most 2 should be at notify/urgent level
	// Others should be downgraded to queued
	if notifyPlusCount > 2 {
		t.Errorf("Expected at most 2 notify/urgent (quota=2), got %d", notifyPlusCount)
	}

	// Should have 2 downgraded (4 items - 2 quota = 2 downgraded)
	if result.Report.QuotaDowngraded != 2 {
		t.Errorf("Expected 2 downgraded, got %d", result.Report.QuotaDowngraded)
	}
}

func TestEngineUrgentNeverDowngraded(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	// Pre-fill quota
	quotaStore := NewInMemoryQuotaStore()
	dayKey := fixedTime.UTC().Format("2006-01-02")
	for i := 0; i < 10; i++ {
		quotaStore.IncrementUsage("finance", dayKey)
	}

	engine := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), quotaStore)

	// Create urgent finance obligation
	dueBy := fixedTime.Add(1 * time.Hour)
	oblig := obligation.NewObligation(
		"circle-finance",
		"event-urgent",
		"finance",
		obligation.ObligationReview,
		fixedTime,
	).WithDueBy(dueBy, fixedTime).WithSeverity(obligation.SeverityCritical).WithScoring(0.95, 0.95)

	dailyView := createTestDailyView(fixedTime)
	result := engine.Process(dailyView, []*obligation.Obligation{oblig})

	if len(result.Interruptions) != 1 {
		t.Fatalf("Expected 1 interruption, got %d", len(result.Interruptions))
	}

	// Urgent should NOT be downgraded even when over quota
	if result.Interruptions[0].Level != interrupt.LevelUrgent {
		t.Errorf("Expected Urgent (never downgraded), got %s", result.Interruptions[0].Level)
	}
}

func TestEngineSorting(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), NewInMemoryQuotaStore())

	// Create mixed obligations
	obligations := []*obligation.Obligation{
		// Low priority
		obligation.NewObligation("circle-home", "e1", "email", obligation.ObligationFollowup, fixedTime).
			WithSeverity(obligation.SeverityLow),
		// High priority
		obligation.NewObligation("circle-finance", "e2", "finance", obligation.ObligationReview, fixedTime).
			WithDueBy(fixedTime.Add(2*time.Hour), fixedTime).WithSeverity(obligation.SeverityCritical).WithScoring(0.95, 0.9),
		// Medium priority
		obligation.NewObligation("circle-work", "e3", "email", obligation.ObligationReply, fixedTime).
			WithDueBy(fixedTime.Add(12*time.Hour), fixedTime).WithSeverity(obligation.SeverityHigh).WithScoring(0.7, 0.8),
	}

	dailyView := createTestDailyView(fixedTime)
	result := engine.Process(dailyView, obligations)

	if len(result.Interruptions) < 2 {
		t.Fatal("Expected at least 2 interruptions")
	}

	// First should be highest level
	first := result.Interruptions[0]
	last := result.Interruptions[len(result.Interruptions)-1]

	if interrupt.LevelOrder(first.Level) < interrupt.LevelOrder(last.Level) {
		t.Errorf("First should have higher level than last: %s vs %s", first.Level, last.Level)
	}
}

func TestEngineNoObligations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := NewEngine(DefaultConfig(), clk, NewInMemoryDeduper(), NewInMemoryQuotaStore())

	dailyView := createTestDailyView(fixedTime)
	result := engine.Process(dailyView, []*obligation.Obligation{})

	if len(result.Interruptions) != 0 {
		t.Errorf("Expected 0 interruptions for no obligations, got %d", len(result.Interruptions))
	}

	if result.Hash != "empty" {
		t.Errorf("Expected 'empty' hash, got %s", result.Hash)
	}
}

func TestCircleTypeFromID(t *testing.T) {
	tests := []struct {
		circleID string
		expected string
	}{
		{"circle-finance", "finance"},
		{"circle-family", "family"},
		{"circle-work", "work"},
		{"circle-health", "health"},
		{"circle-home", "home"},
		{"c-finance", "finance"},
		{"random-id", "unknown"},
	}

	for _, tt := range tests {
		got := circleTypeFromID(identity.EntityID(tt.circleID))
		if got != tt.expected {
			t.Errorf("circleTypeFromID(%s) = %s, want %s", tt.circleID, got, tt.expected)
		}
	}
}

func createTestObligations(now time.Time) []*obligation.Obligation {
	return []*obligation.Obligation{
		obligation.NewObligation("circle-work", "email-1", "email", obligation.ObligationReview, now).
			WithDueBy(now.Add(6*time.Hour), now).WithSeverity(obligation.SeverityHigh).WithScoring(0.7, 0.8).
			WithReason("Email requires action").WithEvidence(obligation.EvidenceKeySubject, "Budget Review"),

		obligation.NewObligation("circle-family", "cal-1", "calendar", obligation.ObligationAttend, now).
			WithDueBy(now.Add(3*time.Hour), now).WithSeverity(obligation.SeverityMedium).WithScoring(0.6, 0.9).
			WithReason("Meeting upcoming").WithEvidence(obligation.EvidenceKeyEventTitle, "Parent Meeting"),

		obligation.NewObligation("circle-finance", "bal-1", "finance", obligation.ObligationReview, now).
			WithSeverity(obligation.SeverityLow).WithScoring(0.4, 0.85).
			WithReason("Low balance alert").WithEvidence(obligation.EvidenceKeyBalance, "450.00"),
	}
}

func createTestDailyView(now time.Time) *view.DailyView {
	config := view.DefaultNeedsYouConfig()
	builder := view.NewDailyViewBuilder(now, config)
	builder.AddCircle("circle-work", "Work")
	builder.AddCircle("circle-family", "Family")
	builder.AddCircle("circle-finance", "Finance")
	return builder.Build()
}
