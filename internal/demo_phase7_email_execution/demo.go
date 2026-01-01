// Package demo_phase7_email_execution demonstrates Phase 7: Email Execution Boundary.
//
// CRITICAL: This is the ONLY path to external email writes.
// CRITICAL: Execution ONLY from approved drafts.
// CRITICAL: No auto-retries. No background execution.
// CRITICAL: Must be idempotent - same envelope executed twice returns same result.
// CRITICAL: Reply-only - no new thread creation.
//
// Scenarios demonstrated:
// 1. Successful email send
// 2. Missing snapshot hash blocks execution
// 3. View snapshot mismatch blocks execution
// 4. Stale view snapshot blocks execution
// 5. Policy drift blocks execution
// 6. Idempotency - same envelope returns same result
package demo_phase7_email_execution

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/email/write"
	mockemail "quantumlife/internal/connectors/email/write/providers/mock"
	"quantumlife/internal/email/execution"
	"quantumlife/pkg/domain/draft"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

// DemoResult contains the result of a demo scenario.
type DemoResult struct {
	Scenario    string
	Success     bool
	EnvelopeID  string
	MessageID   string
	Status      string
	Error       string
	Description string
}

// eventCollector collects events for verification.
type eventCollector struct {
	events []events.Event
}

func (c *eventCollector) Emit(event events.Event) {
	c.events = append(c.events, event)
}

// RunAllScenarios runs all demo scenarios and returns results.
func RunAllScenarios() []DemoResult {
	var results []DemoResult

	results = append(results, RunScenario1_SuccessfulSend())
	results = append(results, RunScenario2_MissingHashBlocks())
	results = append(results, RunScenario3_ViewMismatchBlocks())
	results = append(results, RunScenario4_StaleViewBlocks())
	results = append(results, RunScenario5_PolicyDriftBlocks())
	results = append(results, RunScenario6_IdempotencyWorks())

	return results
}

// RunScenario1_SuccessfulSend demonstrates a successful email send.
func RunScenario1_SuccessfulSend() DemoResult {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	collector := &eventCollector{}

	// Create mock writer
	mockWriter := mockemail.NewWriter(
		mockemail.WithClock(func() time.Time { return now }),
	)

	// Create executor
	executor := execution.NewExecutor(
		execution.WithExecutorClock(func() time.Time { return now }),
		execution.WithWriter("mock", mockWriter),
		execution.WithEventEmitter(collector),
	)

	// Create policy snapshot
	policySnapshot := execution.NewPolicySnapshot(execution.PolicySnapshotParams{
		CircleID:          "circle-1",
		EmailWriteEnabled: true,
		AllowedProviders:  []string{"mock"},
		MaxSendsPerDay:    100,
	}, now)

	// Create view snapshot
	viewSnapshot := execution.NewViewSnapshot(execution.ViewSnapshotParams{
		Provider:           "mock",
		AccountID:          "account-1",
		CircleID:           "circle-1",
		ThreadID:           "thread-100",
		InReplyToMessageID: "msg-100",
		MessageCount:       1,
		LastMessageAt:      now.Add(-1 * time.Hour),
	}, now)

	// Create draft
	d := createEmailDraft("circle-1", "thread-100", "msg-100", now)

	// Create envelope
	envelope, err := execution.NewEnvelopeFromDraft(
		d,
		policySnapshot.PolicyHash,
		viewSnapshot.SnapshotHash,
		viewSnapshot.CapturedAt,
		"trace-001",
		now,
	)
	if err != nil {
		return DemoResult{
			Scenario:    "1. Successful Send",
			Success:     false,
			Error:       fmt.Sprintf("failed to create envelope: %v", err),
			Description: "Email send with valid policy and view snapshots",
		}
	}

	// Execute
	result, err := executor.Execute(context.Background(), *envelope)
	if err != nil {
		return DemoResult{
			Scenario:    "1. Successful Send",
			Success:     false,
			EnvelopeID:  envelope.EnvelopeID,
			Error:       err.Error(),
			Description: "Email send with valid policy and view snapshots",
		}
	}

	return DemoResult{
		Scenario:    "1. Successful Send",
		Success:     result.Status == execution.EnvelopeStatusExecuted,
		EnvelopeID:  result.EnvelopeID,
		MessageID:   getMessageID(result),
		Status:      string(result.Status),
		Description: "Email send with valid policy and view snapshots",
	}
}

// RunScenario2_MissingHashBlocks demonstrates that missing hash blocks execution.
func RunScenario2_MissingHashBlocks() DemoResult {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	collector := &eventCollector{}

	mockWriter := mockemail.NewWriter(
		mockemail.WithClock(func() time.Time { return now }),
	)

	executor := execution.NewExecutor(
		execution.WithExecutorClock(func() time.Time { return now }),
		execution.WithWriter("mock", mockWriter),
		execution.WithEventEmitter(collector),
	)

	// Create envelope with missing hashes
	envelope := execution.Envelope{
		EnvelopeID:         "env-missing-hash",
		DraftID:            "draft-001",
		CircleID:           "circle-1",
		Provider:           "mock",
		ThreadID:           "thread-100",
		InReplyToMessageID: "msg-100",
		Body:               "Test reply",
		// PolicySnapshotHash: "", // MISSING!
		// ViewSnapshotHash: "", // MISSING!
		IdempotencyKey: "idem-missing-001",
		CreatedAt:      now,
		Status:         execution.EnvelopeStatusPending,
	}

	result, _ := executor.Execute(context.Background(), envelope)

	return DemoResult{
		Scenario:    "2. Missing Hash Blocks",
		Success:     result.Status == execution.EnvelopeStatusBlocked,
		EnvelopeID:  result.EnvelopeID,
		Status:      string(result.Status),
		Error:       getBlockReason(result),
		Description: "Execution blocked due to missing policy/view snapshot hash",
	}
}

// RunScenario3_ViewMismatchBlocks demonstrates view mismatch blocking.
func RunScenario3_ViewMismatchBlocks() DemoResult {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	collector := &eventCollector{}

	mockWriter := mockemail.NewWriter(
		mockemail.WithClock(func() time.Time { return now }),
	)

	// Create view verifier that returns different hash
	currentViewProvider := func(threadID string) (execution.ViewSnapshot, error) {
		// Return a different view (simulating thread changed)
		return execution.NewViewSnapshot(execution.ViewSnapshotParams{
			Provider:           "mock",
			AccountID:          "account-1",
			CircleID:           "circle-1",
			ThreadID:           threadID,
			InReplyToMessageID: "msg-200", // Different message!
			MessageCount:       2,         // More messages!
			LastMessageAt:      now,
		}, now), nil
	}

	viewVerifier := execution.NewViewVerifier(
		currentViewProvider,
		execution.WithMaxStaleness(5*time.Minute),
		execution.WithViewClock(func() time.Time { return now }),
	)

	executor := execution.NewExecutor(
		execution.WithExecutorClock(func() time.Time { return now }),
		execution.WithWriter("mock", mockWriter),
		execution.WithViewVerifier(viewVerifier),
		execution.WithEventEmitter(collector),
	)

	// Create original view snapshot
	viewSnapshot := execution.NewViewSnapshot(execution.ViewSnapshotParams{
		Provider:           "mock",
		AccountID:          "account-1",
		CircleID:           "circle-1",
		ThreadID:           "thread-100",
		InReplyToMessageID: "msg-100",
		MessageCount:       1,
		LastMessageAt:      now.Add(-1 * time.Hour),
	}, now)

	policySnapshot := execution.NewPolicySnapshot(execution.PolicySnapshotParams{
		CircleID:          "circle-1",
		EmailWriteEnabled: true,
		AllowedProviders:  []string{"mock"},
	}, now)

	d := createEmailDraft("circle-1", "thread-100", "msg-100", now)

	envelope, _ := execution.NewEnvelopeFromDraft(
		d,
		policySnapshot.PolicyHash,
		viewSnapshot.SnapshotHash,
		viewSnapshot.CapturedAt,
		"trace-003",
		now,
	)

	result, _ := executor.Execute(context.Background(), *envelope)

	return DemoResult{
		Scenario:    "3. View Mismatch Blocks",
		Success:     result.Status == execution.EnvelopeStatusBlocked,
		EnvelopeID:  result.EnvelopeID,
		Status:      string(result.Status),
		Error:       getBlockReason(result),
		Description: "Execution blocked when thread has changed since snapshot",
	}
}

// RunScenario4_StaleViewBlocks demonstrates stale view blocking.
func RunScenario4_StaleViewBlocks() DemoResult {
	snapshotTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC) // 1 hour later
	collector := &eventCollector{}

	mockWriter := mockemail.NewWriter(
		mockemail.WithClock(func() time.Time { return now }),
	)

	// View verifier with 5-minute staleness
	viewVerifier := execution.NewViewVerifier(
		nil, // No current view provider needed
		execution.WithMaxStaleness(5*time.Minute),
		execution.WithViewClock(func() time.Time { return now }),
	)

	executor := execution.NewExecutor(
		execution.WithExecutorClock(func() time.Time { return now }),
		execution.WithWriter("mock", mockWriter),
		execution.WithViewVerifier(viewVerifier),
		execution.WithEventEmitter(collector),
	)

	// Create view snapshot 1 hour ago (stale)
	viewSnapshot := execution.NewViewSnapshot(execution.ViewSnapshotParams{
		Provider:           "mock",
		AccountID:          "account-1",
		CircleID:           "circle-1",
		ThreadID:           "thread-100",
		InReplyToMessageID: "msg-100",
		MessageCount:       1,
		LastMessageAt:      snapshotTime.Add(-2 * time.Hour),
	}, snapshotTime) // Captured 1 hour ago!

	policySnapshot := execution.NewPolicySnapshot(execution.PolicySnapshotParams{
		CircleID:          "circle-1",
		EmailWriteEnabled: true,
		AllowedProviders:  []string{"mock"},
	}, snapshotTime)

	d := createEmailDraft("circle-1", "thread-100", "msg-100", snapshotTime)

	envelope, _ := execution.NewEnvelopeFromDraft(
		d,
		policySnapshot.PolicyHash,
		viewSnapshot.SnapshotHash,
		viewSnapshot.CapturedAt,
		"trace-004",
		snapshotTime,
	)

	result, _ := executor.Execute(context.Background(), *envelope)

	return DemoResult{
		Scenario:    "4. Stale View Blocks",
		Success:     result.Status == execution.EnvelopeStatusBlocked,
		EnvelopeID:  result.EnvelopeID,
		Status:      string(result.Status),
		Error:       getBlockReason(result),
		Description: "Execution blocked when view snapshot is older than max staleness",
	}
}

// RunScenario5_PolicyDriftBlocks demonstrates policy drift blocking.
func RunScenario5_PolicyDriftBlocks() DemoResult {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	collector := &eventCollector{}

	mockWriter := mockemail.NewWriter(
		mockemail.WithClock(func() time.Time { return now }),
	)

	// Policy verifier that returns different policy
	currentPolicyProvider := func(circleID, intersectionID identity.EntityID) execution.PolicySnapshot {
		return execution.NewPolicySnapshot(execution.PolicySnapshotParams{
			CircleID:          circleID,
			IntersectionID:    intersectionID,
			EmailWriteEnabled: false, // Email now DISABLED!
			AllowedProviders:  []string{"mock"},
		}, now)
	}

	policyVerifier := execution.NewPolicyVerifier(currentPolicyProvider)

	executor := execution.NewExecutor(
		execution.WithExecutorClock(func() time.Time { return now }),
		execution.WithWriter("mock", mockWriter),
		execution.WithPolicyVerifier(policyVerifier),
		execution.WithEventEmitter(collector),
	)

	// Create original policy snapshot (email was enabled)
	originalPolicy := execution.NewPolicySnapshot(execution.PolicySnapshotParams{
		CircleID:          "circle-1",
		EmailWriteEnabled: true, // Was enabled when draft was approved
		AllowedProviders:  []string{"mock"},
	}, now)

	viewSnapshot := execution.NewViewSnapshot(execution.ViewSnapshotParams{
		Provider:           "mock",
		AccountID:          "account-1",
		CircleID:           "circle-1",
		ThreadID:           "thread-100",
		InReplyToMessageID: "msg-100",
	}, now)

	d := createEmailDraft("circle-1", "thread-100", "msg-100", now)

	envelope, _ := execution.NewEnvelopeFromDraft(
		d,
		originalPolicy.PolicyHash,
		viewSnapshot.SnapshotHash,
		viewSnapshot.CapturedAt,
		"trace-005",
		now,
	)

	result, _ := executor.Execute(context.Background(), *envelope)

	return DemoResult{
		Scenario:    "5. Policy Drift Blocks",
		Success:     result.Status == execution.EnvelopeStatusBlocked,
		EnvelopeID:  result.EnvelopeID,
		Status:      string(result.Status),
		Error:       getBlockReason(result),
		Description: "Execution blocked when policy has changed since approval",
	}
}

// RunScenario6_IdempotencyWorks demonstrates idempotency.
func RunScenario6_IdempotencyWorks() DemoResult {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	collector := &eventCollector{}

	mockWriter := mockemail.NewWriter(
		mockemail.WithClock(func() time.Time { return now }),
	)

	executor := execution.NewExecutor(
		execution.WithExecutorClock(func() time.Time { return now }),
		execution.WithWriter("mock", mockWriter),
		execution.WithEventEmitter(collector),
	)

	policySnapshot := execution.NewPolicySnapshot(execution.PolicySnapshotParams{
		CircleID:          "circle-1",
		EmailWriteEnabled: true,
		AllowedProviders:  []string{"mock"},
	}, now)

	viewSnapshot := execution.NewViewSnapshot(execution.ViewSnapshotParams{
		Provider:           "mock",
		AccountID:          "account-1",
		CircleID:           "circle-1",
		ThreadID:           "thread-100",
		InReplyToMessageID: "msg-100",
	}, now)

	d := createEmailDraft("circle-1", "thread-100", "msg-100", now)

	envelope, _ := execution.NewEnvelopeFromDraft(
		d,
		policySnapshot.PolicyHash,
		viewSnapshot.SnapshotHash,
		viewSnapshot.CapturedAt,
		"trace-006",
		now,
	)

	// Execute first time
	result1, _ := executor.Execute(context.Background(), *envelope)

	// Execute second time (should return same result)
	result2, _ := executor.Execute(context.Background(), *envelope)

	// Check that results are identical
	sameResult := result1.EnvelopeID == result2.EnvelopeID &&
		result1.Status == result2.Status &&
		getMessageID(result1) == getMessageID(result2)

	return DemoResult{
		Scenario:    "6. Idempotency Works",
		Success:     sameResult && result1.Status == execution.EnvelopeStatusExecuted,
		EnvelopeID:  result1.EnvelopeID,
		MessageID:   getMessageID(result1),
		Status:      string(result1.Status),
		Description: "Same envelope executed twice returns identical result",
	}
}

// createEmailDraft creates a draft for testing.
func createEmailDraft(circleID identity.EntityID, threadID, inReplyTo string, now time.Time) draft.Draft {
	content := draft.EmailDraftContent{
		To:                 "recipient@example.com",
		Subject:            "Re: Test Thread",
		Body:               "This is a test reply from Phase 7 demo.",
		ThreadID:           threadID,
		InReplyToMessageID: inReplyTo,
		ProviderHint:       "mock",
	}

	return draft.Draft{
		DraftID:           draft.DraftID(fmt.Sprintf("draft-%s-%s", circleID, threadID)),
		DraftType:         draft.DraftTypeEmailReply,
		CircleID:          circleID,
		CreatedAt:         now,
		ExpiresAt:         now.Add(24 * time.Hour),
		Status:            draft.StatusApproved,
		StatusChangedAt:   now,
		Content:           content,
		DeterministicHash: "test-hash",
		GenerationRuleID:  "demo-rule",
	}
}

// getMessageID extracts message ID from result.
func getMessageID(result *execution.Envelope) string {
	if result.ExecutionResult != nil {
		return result.ExecutionResult.MessageID
	}
	return ""
}

// getBlockReason extracts block reason from result.
func getBlockReason(result *execution.Envelope) string {
	if result.ExecutionResult != nil {
		if result.ExecutionResult.BlockedReason != "" {
			return result.ExecutionResult.BlockedReason
		}
		return result.ExecutionResult.Error
	}
	return ""
}

// Ensure mock writer implements the interface.
var _ write.Writer = (*mockemail.Writer)(nil)
