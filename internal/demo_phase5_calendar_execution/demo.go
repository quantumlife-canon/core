// Package demo_phase5_calendar_execution demonstrates Phase 5: Calendar Execution Boundary.
//
// CRITICAL: This is the FIRST real external write in QuantumLife.
// CRITICAL: Execution ONLY from approved drafts.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent.
//
// Reference: docs/ADR/ADR-0022-phase5-calendar-execution-boundary.md
package demo_phase5_calendar_execution

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/calendar/execution"
	"quantumlife/internal/connectors/calendar/write"
	mockcal "quantumlife/internal/connectors/calendar/write/providers/mock"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/events"
)

// DemoResult contains the result of running the demo.
type DemoResult struct {
	// DraftGenerated indicates a draft was generated.
	DraftGenerated bool

	// DraftApproved indicates the draft was approved.
	DraftApproved bool

	// EnvelopeCreated indicates an envelope was created.
	EnvelopeCreated bool

	// ExecutionSuccess indicates execution succeeded.
	ExecutionSuccess bool

	// ExecutionBlocked indicates execution was blocked.
	ExecutionBlocked bool

	// BlockedReason explains why execution was blocked.
	BlockedReason string

	// ProviderResponseID is the provider's response (if successful).
	ProviderResponseID string

	// Events contains events emitted during the demo.
	Events []events.Event
}

// Demo demonstrates the complete Phase 5 calendar execution flow.
type Demo struct {
	clock func() time.Time
}

// NewDemo creates a new demo.
func NewDemo(clock func() time.Time) *Demo {
	return &Demo{
		clock: clock,
	}
}

// RunFullFlow demonstrates the complete flow:
// 1. Generate a calendar response draft from an obligation
// 2. Review and approve the draft
// 3. Create an execution envelope with policy and view snapshots
// 4. Execute the calendar write
func (d *Demo) RunFullFlow() DemoResult {
	result := DemoResult{}
	now := d.clock()

	// =========================================================================
	// Step 1: Generate a calendar response draft from an obligation
	// =========================================================================

	// Create a mock obligation for a calendar invite
	obl := &obligation.Obligation{
		ID:            "obl-calendar-invite-001",
		Type:          obligation.ObligationDecide,
		SourceType:    "calendar",
		SourceEventID: "event-team-sync-123",
		Severity:      obligation.SeverityMedium,
		DueBy:         timePtr(now.Add(24 * time.Hour)),
		Evidence: map[string]string{
			obligation.EvidenceKeyEventTitle: "Weekly Team Sync",
			obligation.EvidenceKeySender:     "manager@example.com",
			"provider":                       "mock",
			"calendar_id":                    "primary",
			"suggested_response":             "accept",
		},
		CreatedAt: now,
	}

	// Create draft engine and generate draft
	draftEngine := calendar.NewDefaultEngine()
	draftPolicy := draft.DefaultDraftPolicy()

	genCtx := draft.GenerationContext{
		CircleID:       identity.EntityID("circle-personal"),
		IntersectionID: identity.EntityID("intersection-work"),
		Obligation:     obl,
		Now:            now,
		Policy:         draftPolicy,
	}

	genResult := draftEngine.Generate(genCtx)
	if genResult.Error != nil {
		return result
	}
	if genResult.Skipped {
		return result
	}

	result.DraftGenerated = true
	calendarDraft := genResult.Draft

	// =========================================================================
	// Step 2: Simulate approval (in production, user would review)
	// =========================================================================

	calendarDraft.Status = draft.StatusApproved
	result.DraftApproved = true

	// =========================================================================
	// Step 3: Create policy and view snapshots
	// =========================================================================

	// Create policy snapshot
	policySnapshot := execution.NewPolicySnapshot(execution.PolicySnapshotParams{
		CircleID:                identity.EntityID("circle-personal"),
		IntersectionID:          identity.EntityID("intersection-work"),
		CalendarWriteEnabled:    true,
		AllowedProviders:        []string{"mock", "google"},
		RequireExplicitApproval: true,
		MaxStalenessMinutes:     15,
		DryRunMode:              false,
	}, now)

	// Create view snapshot (simulating we read the calendar state)
	viewSnapshot := execution.NewViewSnapshot(execution.ViewSnapshotParams{
		CircleID:               identity.EntityID("circle-personal"),
		Provider:               "mock",
		CalendarID:             "primary",
		EventID:                "event-team-sync-123",
		EventETag:              "etag-v1",
		EventUpdatedAt:         now.Add(-1 * time.Hour),
		AttendeeResponseStatus: "needsAction",
		EventSummary:           "Weekly Team Sync",
		EventStart:             now.Add(24 * time.Hour),
		EventEnd:               now.Add(25 * time.Hour),
	}, now)

	// =========================================================================
	// Step 4: Create execution envelope
	// =========================================================================

	envelope, err := execution.NewEnvelopeFromDraft(
		*calendarDraft,
		policySnapshot.PolicyHash,
		viewSnapshot.ViewHash,
		viewSnapshot.CapturedAt,
		"trace-demo-001",
		now,
	)
	if err != nil {
		return result
	}
	result.EnvelopeCreated = true

	// =========================================================================
	// Step 5: Execute the calendar write
	// =========================================================================

	// Create mock writer
	mockWriter := mockcal.NewWriter(
		mockcal.WithClock(d.clock),
	)

	// Create executor
	executor := execution.NewExecutor(execution.ExecutorConfig{
		EnvelopeStore:   execution.NewMemoryStore(),
		FreshnessPolicy: execution.NewDefaultFreshnessPolicy(),
		Clock:           d.clock,
	})
	executor.RegisterWriter("mock", mockWriter)

	// Execute
	execResult := executor.Execute(context.Background(), envelope)

	if execResult.Success {
		result.ExecutionSuccess = true
		result.ProviderResponseID = execResult.ProviderResponseID
	} else if execResult.Blocked {
		result.ExecutionBlocked = true
		result.BlockedReason = execResult.BlockedReason
	}

	return result
}

// RunIdempotencyDemo demonstrates that executing the same envelope twice
// returns the same result (idempotency).
func (d *Demo) RunIdempotencyDemo() (firstResult, secondResult execution.ExecuteResult, callCount int) {
	now := d.clock()

	// Create mock writer
	mockWriter := mockcal.NewWriter(
		mockcal.WithClock(d.clock),
	)

	// Create executor
	store := execution.NewMemoryStore()
	executor := execution.NewExecutor(execution.ExecutorConfig{
		EnvelopeStore:   store,
		FreshnessPolicy: execution.NewDefaultFreshnessPolicy(),
		Clock:           d.clock,
	})
	executor.RegisterWriter("mock", mockWriter)

	// Create an envelope
	envelope := &execution.Envelope{
		EnvelopeID:         "env-idem-test",
		DraftID:            "draft-idem-test",
		CircleID:           identity.EntityID("circle-personal"),
		IntersectionID:     identity.EntityID("intersection-work"),
		Provider:           "mock",
		CalendarID:         "primary",
		EventID:            "event-idem-test",
		Response:           draft.CalendarResponseAccept,
		Message:            "I will attend",
		PolicySnapshotHash: "policy-hash-test",
		ViewSnapshotHash:   "view-hash-test",
		ViewSnapshotAt:     now.Add(-1 * time.Minute),
		IdempotencyKey:     "idem-key-test",
		TraceID:            "trace-idem-test",
		Status:             execution.EnvelopeStatusPending,
		CreatedAt:          now,
	}

	// Execute first time
	firstResult = executor.Execute(context.Background(), envelope)

	// Execute second time with same envelope
	secondEnvelope := &execution.Envelope{
		EnvelopeID:         "env-idem-test-2", // Different envelope ID
		DraftID:            "draft-idem-test",
		CircleID:           identity.EntityID("circle-personal"),
		IntersectionID:     identity.EntityID("intersection-work"),
		Provider:           "mock",
		CalendarID:         "primary",
		EventID:            "event-idem-test",
		Response:           draft.CalendarResponseAccept,
		Message:            "I will attend",
		PolicySnapshotHash: "policy-hash-test",
		ViewSnapshotHash:   "view-hash-test",
		ViewSnapshotAt:     now.Add(-1 * time.Minute),
		IdempotencyKey:     "idem-key-test", // Same idempotency key!
		TraceID:            "trace-idem-test",
		Status:             execution.EnvelopeStatusPending,
		CreatedAt:          now,
	}
	secondResult = executor.Execute(context.Background(), secondEnvelope)

	// Get call count from mock writer
	callCount = mockWriter.GetCallCount("event-idem-test")

	return firstResult, secondResult, callCount
}

// RunPolicyMismatchDemo demonstrates that execution is blocked
// when policy has changed since the snapshot was taken.
func (d *Demo) RunPolicyMismatchDemo() execution.ExecuteResult {
	now := d.clock()

	// Create mock writer
	mockWriter := mockcal.NewWriter(
		mockcal.WithClock(d.clock),
	)

	// Create policy verifier that always returns mismatch
	policyVerifier := execution.NewPolicyVerifier(func(circleID, intersectionID identity.EntityID) (string, error) {
		return "different-hash", nil // Always return different hash
	})

	// Create executor
	executor := execution.NewExecutor(execution.ExecutorConfig{
		EnvelopeStore:   execution.NewMemoryStore(),
		PolicyVerifier:  policyVerifier,
		FreshnessPolicy: execution.NewDefaultFreshnessPolicy(),
		Clock:           d.clock,
	})
	executor.RegisterWriter("mock", mockWriter)

	// Create an envelope with a specific policy hash
	envelope := &execution.Envelope{
		EnvelopeID:         "env-policy-mismatch",
		DraftID:            "draft-policy-mismatch",
		CircleID:           identity.EntityID("circle-personal"),
		IntersectionID:     identity.EntityID("intersection-work"),
		Provider:           "mock",
		CalendarID:         "primary",
		EventID:            "event-policy-test",
		Response:           draft.CalendarResponseAccept,
		PolicySnapshotHash: "original-hash", // Will not match "different-hash"
		ViewSnapshotHash:   "view-hash",
		ViewSnapshotAt:     now.Add(-1 * time.Minute),
		IdempotencyKey:     "idem-policy-test",
		TraceID:            "trace-policy-test",
		Status:             execution.EnvelopeStatusPending,
		CreatedAt:          now,
	}

	// Execute - should be blocked due to policy mismatch
	return executor.Execute(context.Background(), envelope)
}

// RunViewStaleDemo demonstrates that execution is blocked
// when the view snapshot is too old.
func (d *Demo) RunViewStaleDemo() execution.ExecuteResult {
	now := d.clock()

	// Create mock writer
	mockWriter := mockcal.NewWriter(
		mockcal.WithClock(d.clock),
	)

	// Create view verifier that checks freshness
	viewVerifier := execution.NewViewVerifier(func(provider, calendarID, eventID string) (string, string, error) {
		return "current-hash", "current-etag", nil
	})

	// Create executor with tight freshness policy
	executor := execution.NewExecutor(execution.ExecutorConfig{
		EnvelopeStore: execution.NewMemoryStore(),
		ViewVerifier:  viewVerifier,
		FreshnessPolicy: execution.FreshnessPolicy{
			DefaultMaxStaleness: 5 * time.Minute, // Very tight
		},
		Clock: d.clock,
	})
	executor.RegisterWriter("mock", mockWriter)

	// Create an envelope with an old view snapshot
	envelope := &execution.Envelope{
		EnvelopeID:         "env-view-stale",
		DraftID:            "draft-view-stale",
		CircleID:           identity.EntityID("circle-personal"),
		IntersectionID:     identity.EntityID("intersection-work"),
		Provider:           "mock",
		CalendarID:         "primary",
		EventID:            "event-stale-test",
		Response:           draft.CalendarResponseAccept,
		PolicySnapshotHash: "policy-hash",
		ViewSnapshotHash:   "old-hash",
		ViewSnapshotAt:     now.Add(-1 * time.Hour), // 1 hour old - too stale!
		IdempotencyKey:     "idem-stale-test",
		TraceID:            "trace-stale-test",
		Status:             execution.EnvelopeStatusPending,
		CreatedAt:          now,
	}

	// Execute - should be blocked due to stale view
	return executor.Execute(context.Background(), envelope)
}

// PrintDemoResults prints the results of the demo in a readable format.
func PrintDemoResults(result DemoResult) {
	fmt.Println("=== Phase 5: Calendar Execution Boundary Demo ===")
	fmt.Println()
	fmt.Printf("Draft Generated:    %v\n", result.DraftGenerated)
	fmt.Printf("Draft Approved:     %v\n", result.DraftApproved)
	fmt.Printf("Envelope Created:   %v\n", result.EnvelopeCreated)
	fmt.Printf("Execution Success:  %v\n", result.ExecutionSuccess)
	fmt.Printf("Execution Blocked:  %v\n", result.ExecutionBlocked)
	if result.BlockedReason != "" {
		fmt.Printf("Blocked Reason:     %s\n", result.BlockedReason)
	}
	if result.ProviderResponseID != "" {
		fmt.Printf("Provider Response:  %s\n", result.ProviderResponseID)
	}
	fmt.Println()
}

// timePtr returns a pointer to the given time.
func timePtr(t time.Time) *time.Time {
	return &t
}

// Verify interface compliance.
var _ write.Writer = (*mockcal.Writer)(nil)
