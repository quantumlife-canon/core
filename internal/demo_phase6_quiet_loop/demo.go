// Package demo_phase6_quiet_loop demonstrates Phase 6: The Quiet Loop.
//
// This demo shows:
// 1. End-to-end daily loop: ingest → obligations → interruptions → drafts → quiet state
// 2. Feedback capture (helpful/unnecessary signals)
// 3. Draft approval and rejection workflow
// 4. "Nothing Needs You" quiet state when everything is handled
//
// CRITICAL: Loop runs SYNCHRONOUSLY per request.
// CRITICAL: No background workers, no auto-retries.
// CRITICAL: Deterministic given same inputs + clock.
//
// Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md
package demo_phase6_quiet_loop

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/internal/calendar/execution"
	mockcal "quantumlife/internal/connectors/calendar/write/providers/mock"
	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/email"
	"quantumlife/internal/drafts/review"
	"quantumlife/internal/interruptions"
	"quantumlife/internal/loop"
	"quantumlife/internal/obligations"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

// DemoResult contains the demo output.
type DemoResult struct {
	Output string
	Err    error
}

// mockIdentityRepo implements IdentityRepository for obligations engine.
type mockIdentityRepo struct{}

func (m *mockIdentityRepo) GetByID(id identity.EntityID) (identity.Entity, error) {
	return nil, nil
}

func (m *mockIdentityRepo) IsHighPriority(id identity.EntityID) bool {
	return false
}

// eventCollector collects audit events.
type eventCollector struct {
	events []events.Event
}

func (c *eventCollector) Emit(event events.Event) {
	c.events = append(c.events, event)
}

// RunDemo executes the Phase 6 quiet loop demo.
func RunDemo() DemoResult {
	var out strings.Builder

	out.WriteString("================================================================================\n")
	out.WriteString("                    PHASE 6: THE QUIET LOOP DEMO\n")
	out.WriteString("================================================================================\n\n")

	// Use fixed time for deterministic demo
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	out.WriteString(fmt.Sprintf("Demo time: %s\n", fixedTime.Format(time.RFC3339)))
	out.WriteString("Clock: Fixed (deterministic)\n\n")

	// Create all the stores and engines
	emitter := &eventCollector{}
	draftStore := draft.NewInMemoryStore()
	feedbackStore := feedback.NewMemoryStore()
	identityRepo := identity.NewInMemoryRepository()
	eventStore := domainevents.NewInMemoryEventStore()

	// Create circles
	gen := identity.NewGenerator()
	personalCircle := gen.CircleFromName("owner-1", "Personal", fixedTime)
	workCircle := gen.CircleFromName("owner-1", "Work", fixedTime)
	identityRepo.Store(personalCircle)
	identityRepo.Store(workCircle)

	out.WriteString("Created circles:\n")
	out.WriteString(fmt.Sprintf("  - Personal: %s\n", personalCircle.ID()))
	out.WriteString(fmt.Sprintf("  - Work: %s\n", workCircle.ID()))
	out.WriteString("\n")

	// Create engines
	oblConfig := obligations.DefaultConfig()
	oblIdentityRepo := &mockIdentityRepo{}
	obligationEngine := obligations.NewEngine(oblConfig, clk, oblIdentityRepo)

	intConfig := interruptions.DefaultConfig()
	dedupStore := interruptions.NewInMemoryDeduper()
	quotaStore := interruptions.NewInMemoryQuotaStore()
	interruptionEngine := interruptions.NewEngine(intConfig, clk, dedupStore, quotaStore)

	draftPolicy := draft.DefaultDraftPolicy()
	emailEngine := email.NewDefaultEngine()
	calendarEngine := calendar.NewDefaultEngine()
	draftEngine := drafts.NewEngine(draftStore, draftPolicy, emailEngine, calendarEngine)

	reviewService := review.NewService(draftStore)

	mockWriter := mockcal.NewWriter(mockcal.WithClock(clk.Now))
	executor := execution.NewExecutor(execution.ExecutorConfig{
		EnvelopeStore:   execution.NewMemoryStore(),
		FreshnessPolicy: execution.NewDefaultFreshnessPolicy(),
		Clock:           clk.Now,
	})
	executor.RegisterWriter("mock", mockWriter)

	// Create loop engine
	engine := &loop.Engine{
		Clock:              clk,
		IdentityRepo:       identityRepo,
		EventStore:         eventStore,
		ObligationEngine:   obligationEngine,
		InterruptionEngine: interruptionEngine,
		DraftEngine:        draftEngine,
		DraftStore:         draftStore,
		ReviewService:      reviewService,
		CalendarExecutor:   executor,
		FeedbackStore:      feedbackStore,
		EventEmitter:       emitter,
	}

	// =========================================================================
	// Scenario 1: Empty state - "Nothing Needs You"
	// =========================================================================
	out.WriteString("--- SCENARIO 1: EMPTY STATE ---\n\n")

	result1 := engine.Run(context.Background(), loop.RunOptions{})

	out.WriteString(fmt.Sprintf("Run ID: %s\n", result1.RunID))
	out.WriteString(fmt.Sprintf("Circles processed: %d\n", len(result1.Circles)))
	out.WriteString(fmt.Sprintf("Is Quiet: %v\n", result1.NeedsYou.IsQuiet))
	out.WriteString(fmt.Sprintf("Total items needing attention: %d\n", result1.NeedsYou.TotalItems))
	out.WriteString(fmt.Sprintf("NeedsYou Hash: %s\n", result1.NeedsYou.Hash))

	if result1.NeedsYou.IsQuiet {
		out.WriteString("\n  ✓ NOTHING NEEDS YOU\n")
		out.WriteString("    All caught up. Enjoy the quiet.\n")
	}
	out.WriteString("\n")

	// =========================================================================
	// Scenario 2: Populate with events and run loop
	// =========================================================================
	out.WriteString("--- SCENARIO 2: WITH EVENTS ---\n\n")

	// Populate mock events
	populateMockEvents(eventStore, fixedTime, personalCircle.ID(), workCircle.ID())
	out.WriteString("Populated 3 mock events (email + 2 calendar invites)\n\n")

	result2 := engine.Run(context.Background(), loop.RunOptions{})

	out.WriteString(fmt.Sprintf("Run ID: %s\n", result2.RunID))
	out.WriteString(fmt.Sprintf("Is Quiet: %v\n", result2.NeedsYou.IsQuiet))
	out.WriteString(fmt.Sprintf("Total items needing attention: %d\n", result2.NeedsYou.TotalItems))
	out.WriteString(fmt.Sprintf("  - Pending drafts: %d\n", len(result2.NeedsYou.PendingDrafts)))
	out.WriteString(fmt.Sprintf("  - Active interruptions: %d\n", len(result2.NeedsYou.ActiveInterruptions)))

	// Show circle results
	out.WriteString("\nPer-circle results:\n")
	for _, cr := range result2.Circles {
		out.WriteString(fmt.Sprintf("  [%s] %s:\n", cr.CircleID, cr.CircleName))
		out.WriteString(fmt.Sprintf("    Obligations: %d\n", cr.ObligationCount))
		out.WriteString(fmt.Sprintf("    Interruptions: %d\n", cr.InterruptionCount))
		out.WriteString(fmt.Sprintf("    Drafts generated: %d\n", cr.DraftCount))
		out.WriteString(fmt.Sprintf("    Drafts pending: %d\n", len(cr.DraftsPending)))
	}
	out.WriteString("\n")

	// =========================================================================
	// Scenario 3: Feedback capture
	// =========================================================================
	out.WriteString("--- SCENARIO 3: FEEDBACK CAPTURE ---\n\n")

	// Record some feedback
	if len(result2.NeedsYou.ActiveInterruptions) > 0 {
		intr := result2.NeedsYou.ActiveInterruptions[0]
		record, err := engine.RecordFeedback(
			feedback.TargetInterruption,
			intr.InterruptionID,
			intr.CircleID,
			feedback.SignalHelpful,
			"This was a useful reminder",
		)
		if err != nil {
			return DemoResult{Err: fmt.Errorf("recording feedback: %w", err)}
		}
		out.WriteString(fmt.Sprintf("Recorded feedback:\n"))
		out.WriteString(fmt.Sprintf("  FeedbackID: %s\n", record.FeedbackID))
		out.WriteString(fmt.Sprintf("  Target: %s (%s)\n", record.TargetType, record.TargetID))
		out.WriteString(fmt.Sprintf("  Signal: %s\n", record.Signal))
		out.WriteString(fmt.Sprintf("  Reason: %s\n", record.Reason))
	}

	// Get feedback stats
	stats := feedbackStore.Stats()
	out.WriteString(fmt.Sprintf("\nFeedback statistics:\n"))
	out.WriteString(fmt.Sprintf("  Total records: %d\n", stats.TotalRecords))
	out.WriteString(fmt.Sprintf("  Helpful count: %d\n", stats.HelpfulCount))
	out.WriteString(fmt.Sprintf("  Unnecessary count: %d\n", stats.UnnecessaryCount))
	out.WriteString("\n")

	// =========================================================================
	// Scenario 4: Determinism check
	// =========================================================================
	out.WriteString("--- SCENARIO 4: DETERMINISM CHECK ---\n\n")

	result3 := engine.Run(context.Background(), loop.RunOptions{})

	out.WriteString(fmt.Sprintf("Run 1 ID: %s\n", result2.RunID))
	out.WriteString(fmt.Sprintf("Run 2 ID: %s\n", result3.RunID))

	if result2.RunID == result3.RunID {
		out.WriteString("✓ Run IDs are identical (deterministic)\n")
	} else {
		out.WriteString("✗ Run IDs differ (non-deterministic!)\n")
	}

	if result2.NeedsYou.Hash == result3.NeedsYou.Hash {
		out.WriteString("✓ NeedsYou hashes are identical (deterministic)\n")
	} else {
		out.WriteString("✗ NeedsYou hashes differ (non-deterministic!)\n")
	}
	out.WriteString("\n")

	// =========================================================================
	// Summary: Events emitted
	// =========================================================================
	out.WriteString("--- AUDIT EVENTS EMITTED ---\n\n")

	eventCounts := make(map[events.EventType]int)
	for _, e := range emitter.events {
		eventCounts[e.Type]++
	}

	// Sort event types for deterministic output
	var eventTypes []events.EventType
	for et := range eventCounts {
		eventTypes = append(eventTypes, et)
	}
	sortEventTypes(eventTypes)

	for _, et := range eventTypes {
		out.WriteString(fmt.Sprintf("  %s: %d\n", et, eventCounts[et]))
	}
	out.WriteString(fmt.Sprintf("\nTotal events: %d\n", len(emitter.events)))

	out.WriteString("\n================================================================================\n")
	out.WriteString("                         DEMO COMPLETE\n")
	out.WriteString("================================================================================\n")
	out.WriteString("\nPhase 6 demonstrates:\n")
	out.WriteString("  - Synchronous daily loop execution (no background workers)\n")
	out.WriteString("  - \"Nothing Needs You\" quiet state when all is handled\n")
	out.WriteString("  - Feedback capture with deterministic IDs\n")
	out.WriteString("  - Deterministic run IDs and state hashes\n")
	out.WriteString("  - Full audit trail via events\n")

	return DemoResult{Output: out.String()}
}

// sortEventTypes sorts event types alphabetically for deterministic output.
func sortEventTypes(types []events.EventType) {
	sort.Slice(types, func(i, j int) bool {
		return types[i] < types[j]
	})
}

// populateMockEvents creates realistic mock events.
func populateMockEvents(store *domainevents.InMemoryEventStore, now time.Time, personal, work identity.EntityID) {
	// Work: Unread important email
	importantEmail := domainevents.NewEmailMessageEvent("gmail", "msg-100", "self@work.com", now, now.Add(-3*time.Hour))
	importantEmail.Circle = work
	importantEmail.Subject = "URGENT: Approval needed - Q1 Budget Review"
	importantEmail.BodyPreview = "Please review and approve the attached budget by Friday."
	importantEmail.From = domainevents.EmailAddress{Address: "cfo@company.com", Name: "Sarah CFO"}
	importantEmail.IsRead = false
	importantEmail.IsImportant = true
	importantEmail.SenderDomain = "company.com"
	store.Store(importantEmail)

	// Work: Unresponded calendar invite
	meetingInvite := domainevents.NewCalendarEventEvent("google", "cal-work", "evt-001", "self@work.com", now, now)
	meetingInvite.Circle = work
	meetingInvite.Title = "Quarterly Review Meeting"
	meetingInvite.StartTime = now.Add(4 * time.Hour)
	meetingInvite.EndTime = now.Add(5 * time.Hour)
	meetingInvite.MyResponseStatus = domainevents.RSVPNeedsAction
	meetingInvite.AttendeeCount = 10
	store.Store(meetingInvite)

	// Personal: School event needing decision
	schoolEvent := domainevents.NewCalendarEventEvent("google", "cal-personal", "evt-002", "self@personal.com", now, now)
	schoolEvent.Circle = personal
	schoolEvent.Title = "Parent-Teacher Conference"
	schoolEvent.StartTime = now.Add(6 * time.Hour)
	schoolEvent.EndTime = now.Add(7 * time.Hour)
	schoolEvent.MyResponseStatus = domainevents.RSVPNeedsAction
	store.Store(schoolEvent)
}
