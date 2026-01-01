// Package main demonstrates Phase 10: Approved Draft → Execution Routing.
//
// This demo shows:
// 1. Creating an approved draft with policy/view snapshot hashes
// 2. Building an ExecutionIntent from the draft via execrouter
// 3. Executing the intent via execexecutor
// 4. The full audit trail via events
//
// CRITICAL: Execution only happens for approved drafts.
// CRITICAL: PolicySnapshotHash and ViewSnapshotHash must be present.
// CRITICAL: Execution flows through boundary executors (email/calendar).
//
// Run: go run ./demo/demo_phase10_execute_routing
package main

import (
	"context"
	"fmt"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	mockcal "quantumlife/internal/connectors/calendar/write/providers/mock"
	mockemail "quantumlife/internal/connectors/email/write/providers/mock"
	emailexec "quantumlife/internal/email/execution"
	"quantumlife/internal/execexecutor"
	"quantumlife/internal/execrouter"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

// demoEmitter captures events for display.
type demoEmitter struct {
	events []events.Event
}

func (e *demoEmitter) Emit(event events.Event) {
	e.events = append(e.events, event)
	fmt.Printf("  [EVENT] %s\n", event.Type)
}

func main() {
	fmt.Println("=== Phase 10 Demo: Approved Draft → Execution Routing ===")
	fmt.Println()

	// Fixed time for determinism
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	// Event emitter
	emitter := &demoEmitter{}

	// Create email boundary executor
	emailMockWriter := mockemail.NewWriter(
		mockemail.WithClock(clk.Now),
	)
	emailExecutor := emailexec.NewExecutor(
		emailexec.WithExecutorClock(clk.Now),
		emailexec.WithWriter("mock", emailMockWriter),
		emailexec.WithEventEmitter(emitter),
	)

	// Create calendar boundary executor
	calMockWriter := mockcal.NewWriter(
		mockcal.WithClock(clk.Now),
	)
	calExecutor := calexec.NewExecutor(calexec.ExecutorConfig{
		EnvelopeStore:   calexec.NewMemoryStore(),
		FreshnessPolicy: calexec.NewDefaultFreshnessPolicy(),
		Clock:           clk.Now,
		EventEmitter:    emitter,
	})
	calExecutor.RegisterWriter("mock", calMockWriter)

	// Create Phase 10 components
	router := execrouter.NewRouter(clk, emitter)
	executor := execexecutor.NewExecutor(clk, emitter).
		WithEmailExecutor(emailExecutor).
		WithCalendarExecutor(calExecutor)

	// Demo 1: Email Reply Execution
	fmt.Println("--- Demo 1: Email Reply Execution ---")
	demoEmailExecution(router, executor, clk, emitter)

	fmt.Println()

	// Demo 2: Calendar Response Execution
	fmt.Println("--- Demo 2: Calendar Response Execution ---")
	demoCalendarExecution(router, executor, clk, emitter)

	fmt.Println()

	// Demo 3: Blocked Execution (Missing Hash)
	fmt.Println("--- Demo 3: Blocked Execution (Missing Hash) ---")
	demoBlockedExecution(router, clk, emitter)

	fmt.Println()
	fmt.Println("=== Phase 10 Demo Complete ===")
}

func demoEmailExecution(
	router *execrouter.Router,
	executor *execexecutor.Executor,
	clk clock.Clock,
	emitter *demoEmitter,
) {
	now := clk.Now()

	// Create an approved email draft with snapshot hashes
	emailDraft := &draft.Draft{
		DraftID:            draft.DraftID("draft-email-demo-001"),
		DraftType:          draft.DraftTypeEmailReply,
		CircleID:           identity.EntityID("circle-personal"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-email-001",
		ViewSnapshotHash:   "view-hash-email-001",
		CreatedAt:          now,
		ExpiresAt:          now.Add(24 * time.Hour),
		Content: draft.EmailDraftContent{
			To:                 "recipient@example.com",
			Subject:            "Re: Meeting Request",
			Body:               "Thank you for your email. I'll review the proposal.",
			ThreadID:           "thread-email-001",
			InReplyToMessageID: "msg-original-001",
			ProviderHint:       "mock",
		},
	}

	fmt.Printf("  Draft ID: %s\n", emailDraft.DraftID)
	fmt.Printf("  Status: %s\n", emailDraft.Status)
	fmt.Printf("  Policy Hash: %s\n", emailDraft.PolicySnapshotHash)
	fmt.Printf("  View Hash: %s\n", emailDraft.ViewSnapshotHash)

	// Build execution intent
	fmt.Println()
	fmt.Println("  Building execution intent...")
	intent, err := router.BuildIntentFromDraft(emailDraft)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}

	fmt.Printf("  Intent ID: %s\n", intent.IntentID)
	fmt.Printf("  Action: %s\n", intent.Action)
	fmt.Printf("  Deterministic Hash: %s\n", intent.DeterministicHash[:16]+"...")

	// Execute the intent
	fmt.Println()
	fmt.Println("  Executing intent...")
	traceID := fmt.Sprintf("demo-email-trace-%d", now.UnixNano())
	outcome := executor.ExecuteIntent(context.Background(), intent, traceID)

	fmt.Printf("  Success: %t\n", outcome.Success)
	if outcome.Success {
		fmt.Printf("  Provider Response ID: %s\n", outcome.ProviderResponseID)
		fmt.Printf("  Envelope ID: %s\n", outcome.EnvelopeID)
	} else if outcome.Blocked {
		fmt.Printf("  Blocked: %s\n", outcome.BlockedReason)
	} else {
		fmt.Printf("  Error: %s\n", outcome.Error)
	}
}

func demoCalendarExecution(
	router *execrouter.Router,
	executor *execexecutor.Executor,
	clk clock.Clock,
	emitter *demoEmitter,
) {
	now := clk.Now()

	// Create an approved calendar draft with snapshot hashes
	calendarDraft := &draft.Draft{
		DraftID:            draft.DraftID("draft-cal-demo-001"),
		DraftType:          draft.DraftTypeCalendarResponse,
		CircleID:           identity.EntityID("circle-work"),
		Status:             draft.StatusApproved,
		PolicySnapshotHash: "policy-hash-cal-001",
		ViewSnapshotHash:   "view-hash-cal-001",
		CreatedAt:          now,
		ExpiresAt:          now.Add(24 * time.Hour),
		Content: draft.CalendarDraftContent{
			EventID:      "event-meeting-001",
			Response:     draft.CalendarResponseAccept,
			Message:      "Looking forward to the meeting!",
			ProviderHint: "mock",
			CalendarID:   "primary",
		},
	}

	fmt.Printf("  Draft ID: %s\n", calendarDraft.DraftID)
	fmt.Printf("  Status: %s\n", calendarDraft.Status)
	fmt.Printf("  Policy Hash: %s\n", calendarDraft.PolicySnapshotHash)
	fmt.Printf("  View Hash: %s\n", calendarDraft.ViewSnapshotHash)

	// Build execution intent
	fmt.Println()
	fmt.Println("  Building execution intent...")
	intent, err := router.BuildIntentFromDraft(calendarDraft)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}

	fmt.Printf("  Intent ID: %s\n", intent.IntentID)
	fmt.Printf("  Action: %s\n", intent.Action)
	fmt.Printf("  Calendar Event ID: %s\n", intent.CalendarEventID)
	fmt.Printf("  Calendar Response: %s\n", intent.CalendarResponse)

	// Execute the intent
	fmt.Println()
	fmt.Println("  Executing intent...")
	traceID := fmt.Sprintf("demo-cal-trace-%d", now.UnixNano())
	outcome := executor.ExecuteIntent(context.Background(), intent, traceID)

	fmt.Printf("  Success: %t\n", outcome.Success)
	if outcome.Success {
		fmt.Printf("  Provider Response ID: %s\n", outcome.ProviderResponseID)
		fmt.Printf("  Envelope ID: %s\n", outcome.EnvelopeID)
	} else if outcome.Blocked {
		fmt.Printf("  Blocked: %s\n", outcome.BlockedReason)
	} else {
		fmt.Printf("  Error: %s\n", outcome.Error)
	}
}

func demoBlockedExecution(
	router *execrouter.Router,
	clk clock.Clock,
	emitter *demoEmitter,
) {
	now := clk.Now()

	// Create a draft WITHOUT policy hash - should be blocked
	blockedDraft := &draft.Draft{
		DraftID:          draft.DraftID("draft-blocked-001"),
		DraftType:        draft.DraftTypeEmailReply,
		CircleID:         identity.EntityID("circle-test"),
		Status:           draft.StatusApproved,
		ViewSnapshotHash: "view-hash-only", // Missing PolicySnapshotHash!
		CreatedAt:        now,
		ExpiresAt:        now.Add(24 * time.Hour),
		Content: draft.EmailDraftContent{
			To:                 "test@example.com",
			Subject:            "Test",
			Body:               "Test body",
			ThreadID:           "thread-test",
			InReplyToMessageID: "msg-test",
		},
	}

	fmt.Printf("  Draft ID: %s\n", blockedDraft.DraftID)
	fmt.Printf("  Status: %s\n", blockedDraft.Status)
	fmt.Printf("  Policy Hash: (MISSING)\n")
	fmt.Printf("  View Hash: %s\n", blockedDraft.ViewSnapshotHash)

	// Try to build execution intent - should fail
	fmt.Println()
	fmt.Println("  Attempting to build execution intent...")
	_, err := router.BuildIntentFromDraft(blockedDraft)
	if err != nil {
		fmt.Printf("  BLOCKED: %v\n", err)
		fmt.Println("  (This is expected - PolicySnapshotHash is required)")
	} else {
		fmt.Println("  ERROR: Should have been blocked!")
	}
}
