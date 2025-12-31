// Package demo_phase3_interruptions demonstrates the Phase 3 interruption engine.
//
// This demo shows:
// 1. Transforming obligations into prioritized interruptions
// 2. Dedup and quota enforcement
// 3. Weekly digest generation
//
// Run: go run ./internal/demo_phase3_interruptions
package main

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/digest"
	"quantumlife/internal/interruptions"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/domain/view"
)

func main() {
	fmt.Println("==============================================")
	fmt.Println(" Phase 3: Interruption Engine + Weekly Digest")
	fmt.Println("==============================================")
	fmt.Println()

	// Use fixed time for deterministic demo
	// Wednesday, January 15, 2025, 10:00 AM UTC
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	fmt.Printf("Demo time: %s\n\n", fixedTime.Format(time.RFC3339))

	// Create sample obligations
	obligations := createDemoObligations(fixedTime)
	fmt.Printf("Created %d sample obligations\n\n", len(obligations))

	// Create daily view
	dailyView := createDailyView(fixedTime)

	// Create and run the engine
	config := interruptions.DefaultConfig()
	dedupStore := interruptions.NewInMemoryDeduper()
	quotaStore := interruptions.NewInMemoryQuotaStore()
	engine := interruptions.NewEngine(config, clk, dedupStore, quotaStore)

	fmt.Println("--- Running Interruption Engine ---")
	result := engine.Process(dailyView, obligations)

	printEngineResults(result)

	// Demonstrate dedup by running again
	fmt.Println("\n--- Running Again (Same Dedup Store) ---")
	result2 := engine.Process(dailyView, obligations)
	fmt.Printf("Second run: %d interruptions (dedup dropped %d)\n",
		len(result2.Interruptions), result2.Report.DedupDropped)

	// Demonstrate weekly digest
	fmt.Println("\n--- Weekly Digest Generation ---")
	weekStart := time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC) // Previous Monday
	digestGen := digest.NewGenerator(clk)
	weeklyDigest := generateWeeklyDigest(digestGen, weekStart, fixedTime, obligations)
	fmt.Println(weeklyDigest.FormatText())

	fmt.Println("==============================================")
	fmt.Println(" Phase 3 Demo Complete")
	fmt.Println("==============================================")
}

func createDemoObligations(now time.Time) []*obligation.Obligation {
	return []*obligation.Obligation{
		// Urgent finance item (due in 2 hours, critical)
		obligation.NewObligation(
			"circle-finance", "evt-low-balance", "finance",
			obligation.ObligationReview, now,
		).WithDueBy(now.Add(2*time.Hour), now).
			WithSeverity(obligation.SeverityCritical).
			WithScoring(0.95, 0.9).
			WithReason("Account balance below $500").
			WithEvidence(obligation.EvidenceKeyBalance, "387.42"),

		// Work email requiring reply (due tomorrow morning)
		obligation.NewObligation(
			"circle-work", "evt-email-ceo", "email",
			obligation.ObligationReply, now,
		).WithDueBy(now.Add(20*time.Hour), now).
			WithSeverity(obligation.SeverityHigh).
			WithScoring(0.75, 0.85).
			WithReason("CEO budget review email awaiting response").
			WithEvidence(obligation.EvidenceKeySubject, "Q4 Budget Review Required"),

		// Family calendar event (meeting in 3 hours)
		obligation.NewObligation(
			"circle-family", "evt-parent-meeting", "calendar",
			obligation.ObligationAttend, now,
		).WithDueBy(now.Add(3*time.Hour), now).
			WithSeverity(obligation.SeverityMedium).
			WithScoring(0.65, 0.95).
			WithReason("School parent-teacher meeting").
			WithEvidence(obligation.EvidenceKeyEventTitle, "Parent-Teacher Conference"),

		// Work task due next week
		obligation.NewObligation(
			"circle-work", "evt-report-due", "task",
			obligation.ObligationReview, now,
		).WithDueBy(now.Add(5*24*time.Hour), now).
			WithSeverity(obligation.SeverityMedium).
			WithScoring(0.5, 0.8).
			WithReason("Quarterly report due Friday"),

		// Health reminder (low priority)
		obligation.NewObligation(
			"circle-health", "evt-checkup", "reminder",
			obligation.ObligationFollowup, now,
		).WithDueBy(now.Add(14*24*time.Hour), now).
			WithSeverity(obligation.SeverityLow).
			WithScoring(0.3, 0.7).
			WithReason("Schedule annual checkup"),

		// Home task (very low priority)
		obligation.NewObligation(
			"circle-home", "evt-filter", "reminder",
			obligation.ObligationFollowup, now,
		).WithSeverity(obligation.SeverityLow).
			WithScoring(0.2, 0.6).
			WithReason("Replace air filter"),

		// Another work email (to test quota)
		obligation.NewObligation(
			"circle-work", "evt-email-team", "email",
			obligation.ObligationReview, now,
		).WithDueBy(now.Add(36*time.Hour), now).
			WithSeverity(obligation.SeverityHigh).
			WithScoring(0.7, 0.8).
			WithReason("Team status update to review").
			WithEvidence(obligation.EvidenceKeySubject, "Weekly Team Update"),

		// Third work item (to test quota limits)
		obligation.NewObligation(
			"circle-work", "evt-email-vendor", "email",
			obligation.ObligationReply, now,
		).WithDueBy(now.Add(30*time.Hour), now).
			WithSeverity(obligation.SeverityHigh).
			WithScoring(0.68, 0.75).
			WithReason("Vendor contract negotiation").
			WithEvidence(obligation.EvidenceKeySubject, "Contract Terms Review"),
	}
}

func createDailyView(now time.Time) *view.DailyView {
	config := view.DefaultNeedsYouConfig()
	builder := view.NewDailyViewBuilder(now, config)
	builder.AddCircle("circle-work", "Work")
	builder.AddCircle("circle-family", "Family")
	builder.AddCircle("circle-finance", "Finance")
	builder.AddCircle("circle-health", "Health")
	builder.AddCircle("circle-home", "Home")
	return builder.Build()
}

func printEngineResults(result interruptions.ProcessResult) {
	fmt.Printf("\nTotal processed: %d\n", result.Report.TotalProcessed)
	fmt.Printf("Dedup dropped: %d\n", result.Report.DedupDropped)
	fmt.Printf("Quota downgraded: %d\n", result.Report.QuotaDowngraded)
	fmt.Printf("Result hash: %s\n\n", result.Hash)

	// Print by level
	fmt.Println("Counts by level:")
	for _, level := range []interrupt.Level{
		interrupt.LevelUrgent,
		interrupt.LevelNotify,
		interrupt.LevelQueued,
		interrupt.LevelAmbient,
		interrupt.LevelSilent,
	} {
		count := result.Report.CountByLevel[level]
		if count > 0 {
			fmt.Printf("  %s: %d\n", level, count)
		}
	}

	// Print interruptions (sorted by priority)
	fmt.Println("\nInterruptions (by priority):")
	fmt.Println(strings.Repeat("-", 70))
	for i, intr := range result.Interruptions {
		fmt.Printf("%d. [%s] %s\n", i+1, levelBadge(intr.Level), intr.Summary)
		fmt.Printf("   Circle: %s | Regret: %d | Trigger: %s\n",
			intr.CircleID, intr.RegretScore, intr.Trigger)
		fmt.Printf("   Expires: %s\n", intr.ExpiresAt.Format(time.RFC3339))
		fmt.Println()
	}
}

func levelBadge(level interrupt.Level) string {
	switch level {
	case interrupt.LevelUrgent:
		return "URGENT"
	case interrupt.LevelNotify:
		return "NOTIFY"
	case interrupt.LevelQueued:
		return "QUEUED"
	case interrupt.LevelAmbient:
		return "AMBIENT"
	case interrupt.LevelSilent:
		return "SILENT"
	default:
		return string(level)
	}
}

func generateWeeklyDigest(gen *digest.Generator, weekStart, _ time.Time, obligations []*obligation.Obligation) *digest.WeeklyDigest {
	// Simulate a week of interruptions by processing each day
	config := interruptions.DefaultConfig()

	var buckets []digest.DailyBucket

	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		dayClk := clock.NewFixed(day.Add(10 * time.Hour)) // 10 AM each day

		// Create fresh stores for each day
		engine := interruptions.NewEngine(config, dayClk, interruptions.NewInMemoryDeduper(), interruptions.NewInMemoryQuotaStore())
		dailyView := createDailyView(day)

		// Process a subset of obligations each day
		dayObligs := selectDayObligations(obligations, i)
		result := engine.Process(dailyView, dayObligs)

		buckets = append(buckets, digest.DailyBucket{
			Date:          day,
			Interruptions: result.Interruptions,
		})
	}

	circleNames := map[identity.EntityID]string{
		"circle-work":    "Work",
		"circle-family":  "Family",
		"circle-finance": "Finance",
		"circle-health":  "Health",
		"circle-home":    "Home",
	}

	return gen.Generate(weekStart, buckets, circleNames)
}

// selectDayObligations returns a subset of obligations for the given day index.
func selectDayObligations(obligations []*obligation.Obligation, dayIndex int) []*obligation.Obligation {
	// Simple strategy: rotate through obligations to simulate daily variation
	var result []*obligation.Obligation
	for i, oblig := range obligations {
		// Include ~60% of obligations each day, rotating
		if (i+dayIndex)%3 != 0 {
			result = append(result, oblig)
		}
	}
	return result
}
