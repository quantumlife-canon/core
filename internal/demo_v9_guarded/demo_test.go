package demo_v9_guarded

import (
	"fmt"
	"testing"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// =============================================================================
// v9 Slice 2: Guarded Execution Acceptance Tests
// =============================================================================
//
// These tests verify that guarded adapters:
// - Block all execution attempts
// - Never move money
// - Produce auditable events
// - Respect revocation and expiry
//
// Reference: docs/ACCEPTANCE_TESTS_V9_EXECUTION.md

// Category A: Guarded Adapter Behavior

func TestA1_GuardedAdapterAlwaysBlocks(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(DefaultScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("scenario should succeed: %s", result.FailureReason)
	}

	if result.ExecutionResult.Status != execution.SettlementBlocked {
		t.Errorf("expected blocked, got %s", result.ExecutionResult.Status)
	}
}

func TestA2_GuardedAdapterNeverMoveMoney(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(DefaultScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExecutionAttempt == nil {
		t.Fatal("expected execution attempt")
	}

	if result.ExecutionAttempt.MoneyMoved {
		t.Error("CRITICAL: money moved - this should NEVER happen")
	}
}

func TestA3_GuardedAdapterEmitsInvokedEvent(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(DefaultScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, event := range result.AuditEvents {
		if event.Type == events.EventV9AdapterInvoked {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected v9.adapter.invoked event")
	}
}

func TestA4_GuardedAdapterEmitsBlockedEvent(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(DefaultScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, event := range result.AuditEvents {
		if event.Type == events.EventV9AdapterBlocked {
			found = true
			if event.Metadata["money_moved"] != "false" {
				t.Error("adapter blocked event should have money_moved=false")
			}
			break
		}
	}

	if !found {
		t.Error("expected v9.adapter.blocked event")
	}
}

// Category B: Approval Does Not Cause Execution

func TestB1_ApprovalAloneDoesNotTriggerExecution(t *testing.T) {
	// Create adapter and runner components
	signingKey := []byte("test-key")
	idCounter := 0
	idGen := func() string {
		idCounter++
		return "test_" + string(rune('0'+idCounter))
	}

	var auditEvents []events.Event
	emitter := func(e events.Event) {
		auditEvents = append(auditEvents, e)
	}

	adapter := execution.NewMockFinanceAdapter(idGen, emitter)

	// Create minimal envelope
	now := time.Now()
	intent := execution.ExecutionIntent{
		IntentID:    "intent_b1",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 1000,
		Currency:    "GBP",
		PayeeID:     "sandbox-utility",
		ViewHash:    "view_b1",
		CreatedAt:   now,
	}

	builder := execution.NewEnvelopeBuilder(idGen)
	envelope, _ := builder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                2000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_b1",
	}, now)

	// Create approval WITHOUT calling Execute
	approvalMgr := execution.NewApprovalManager(idGen, signingKey)
	approvalReq, _ := approvalMgr.CreateApprovalRequest(
		envelope,
		intent.CircleID,
		now.Add(30*time.Minute),
		now,
	)
	approval, _ := approvalMgr.SubmitApproval(
		approvalReq,
		intent.CircleID,
		"approver_1",
		now.Add(30*time.Minute),
		now,
	)

	// Add approval to envelope
	envelope.Approvals = append(envelope.Approvals, *approval)

	// Verify NO adapter events were emitted just from approval
	for _, event := range auditEvents {
		if event.Type == events.EventV9AdapterInvoked {
			t.Error("approval alone should not trigger adapter invocation")
		}
		if event.Type == events.EventV9AdapterBlocked {
			t.Error("approval alone should not trigger adapter block")
		}
	}

	// Now call adapter.Execute - this should emit events
	auditEvents = nil // Reset
	_, execErr := adapter.Execute(envelope, approval)

	if !execution.IsGuardedExecutionError(execErr) {
		t.Error("adapter should return GuardedExecutionError")
	}

	// NOW we should see the events
	hasInvoked := false
	hasBlocked := false
	for _, event := range auditEvents {
		if event.Type == events.EventV9AdapterInvoked {
			hasInvoked = true
		}
		if event.Type == events.EventV9AdapterBlocked {
			hasBlocked = true
		}
	}

	if !hasInvoked {
		t.Error("adapter.Execute should emit invoked event")
	}
	if !hasBlocked {
		t.Error("adapter.Execute should emit blocked event")
	}
}

// Category C: Revocation Still Halts Execution

func TestC1_RevocationBlocksGuardedExecution(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(RevocationScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExecutionResult.Status != execution.SettlementRevoked {
		t.Errorf("expected revoked, got %s", result.ExecutionResult.Status)
	}

	// Revocation should prevent adapter from being called
	// (execution blocked before reaching adapter)
	if result.ExecutionAttempt != nil {
		t.Error("revocation should prevent adapter execution attempt")
	}
}

func TestC2_RevocationTakesPrecedenceOverAdapter(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(RevocationScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that revocation event was emitted
	hasRevocationEvent := false
	for _, event := range result.AuditEvents {
		if event.Type == events.EventV9RevocationTriggered {
			hasRevocationEvent = true
			break
		}
	}

	if !hasRevocationEvent {
		t.Error("expected revocation event")
	}
}

// Category D: Expiry Halts Execution

func TestD1_ExpiredEnvelopeBlocksExecution(t *testing.T) {
	signingKey := []byte("test-key")
	idCounter := 0
	idGen := func() string {
		idCounter++
		return "test_" + string(rune('0'+idCounter))
	}

	revocationChecker := execution.NewRevocationChecker(idGen)
	approvalVerifier := execution.NewApprovalVerifier(signingKey)
	executionRunner := execution.NewExecutionRunner(
		approvalVerifier,
		revocationChecker,
		idGen,
	)

	var auditEvents []events.Event
	emitter := func(e events.Event) {
		auditEvents = append(auditEvents, e)
	}
	adapter := execution.NewMockFinanceAdapter(idGen, emitter)

	// Create envelope that expires
	now := time.Now()
	intent := execution.ExecutionIntent{
		IntentID:    "intent_d1",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 1000,
		Currency:    "GBP",
		PayeeID:     "sandbox-utility",
		ViewHash:    "view_d1",
		CreatedAt:   now,
	}

	builder := execution.NewEnvelopeBuilder(idGen)
	envelope, _ := builder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                2000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(-1 * time.Minute), // Already expired
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_d1",
	}, now)

	// Attempt execution
	result, _, _ := executionRunner.ExecuteWithAdapter(envelope, adapter, now)

	if result.Status != execution.SettlementExpired {
		t.Errorf("expected expired, got %s", result.Status)
	}
}

// Category E: Multiple Provider Stubs

func TestE1_PlaidStubBlocks(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(PlaidStubScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("scenario should succeed: %s", result.FailureReason)
	}

	if result.ExecutionResult.Status != execution.SettlementBlocked {
		t.Errorf("expected blocked, got %s", result.ExecutionResult.Status)
	}

	if result.ExecutionAttempt.Provider != "plaid-stub" {
		t.Errorf("expected plaid-stub provider, got %s", result.ExecutionAttempt.Provider)
	}
}

func TestE2_AllStubsReturnGuardedError(t *testing.T) {
	idGen := func() string { return "test" }
	emitter := func(e events.Event) {}

	stubs := []execution.ExecutionAdapter{
		execution.NewMockFinanceAdapter(idGen, emitter),
		execution.NewPlaidStubAdapter(idGen, emitter),
		execution.NewTrueLayerStubAdapter(idGen, emitter),
	}

	now := time.Now()
	envelope := &execution.ExecutionEnvelope{
		EnvelopeID: "env_test",
		ActionSpec: execution.ActionSpec{
			AmountCents: 1000,
			Currency:    "GBP",
			PayeeID:     "sandbox-utility",
		},
		SealHash: "sealed_hash_test", // Non-empty means sealed
		Expiry:   now.Add(1 * time.Hour),
	}
	approval := &execution.ApprovalArtifact{
		ArtifactID: "approval_test",
		ActionHash: "hash_test",
	}

	for _, stub := range stubs {
		attempt, err := stub.Execute(envelope, approval)

		if !execution.IsGuardedExecutionError(err) {
			t.Errorf("stub %s should return GuardedExecutionError", stub.Provider())
		}

		if attempt == nil {
			t.Errorf("stub %s should return attempt", stub.Provider())
			continue
		}

		if attempt.MoneyMoved {
			t.Errorf("CRITICAL: stub %s moved money", stub.Provider())
		}

		if attempt.Status != execution.AttemptBlocked {
			t.Errorf("stub %s should have blocked status, got %s", stub.Provider(), attempt.Status)
		}
	}
}

// Category F: Audit Trail Completeness

func TestF1_AuditTrailContainsAllRequiredEvents(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(DefaultScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requiredEvents := map[events.EventType]bool{
		events.EventExecutionIntentCreated:   false,
		events.EventExecutionEnvelopeSealed:  false,
		events.EventV9ApprovalRequested:      false,
		events.EventV9ApprovalSubmitted:      false,
		events.EventV9RevocationWindowOpened: false,
		events.EventV9RevocationWindowClosed: false,
		events.EventV9ExecutionStarted:       false,
		events.EventV9AdapterInvoked:         false,
		events.EventV9AdapterBlocked:         false,
		events.EventV9ExecutionBlocked:       false,
		events.EventV9SettlementBlocked:      false,
		events.EventV9AuditTraceFinalized:    false,
	}

	for _, event := range result.AuditEvents {
		if _, ok := requiredEvents[event.Type]; ok {
			requiredEvents[event.Type] = true
		}
	}

	for eventType, found := range requiredEvents {
		if !found {
			t.Errorf("missing required event: %s", eventType)
		}
	}
}

func TestF2_AuditTrailFinalizedWithMoneyMovedFalse(t *testing.T) {
	runner := NewRunner()
	result, err := runner.RunScenario(DefaultScenario())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, event := range result.AuditEvents {
		if event.Type == events.EventV9AuditTraceFinalized {
			found = true
			if event.Metadata["money_moved"] != "false" {
				t.Error("audit finalized should have money_moved=false")
			}
			if event.Metadata["final_status"] != string(execution.SettlementBlocked) {
				t.Errorf("expected blocked status, got %s", event.Metadata["final_status"])
			}
			break
		}
	}

	if !found {
		t.Error("expected audit trace finalized event")
	}
}

// Category G: Settlement Status Constraints

func TestG1_SettlementNeverSucceeds(t *testing.T) {
	scenarios := []*Scenario{
		DefaultScenario(),
		RevocationScenario(),
		PlaidStubScenario(),
	}

	for _, scenario := range scenarios {
		runner := NewRunner()
		result, err := runner.RunScenario(scenario)

		if err != nil {
			continue // Some scenarios may error, that's OK
		}

		if result.ExecutionResult.Status == execution.SettlementSuccessful {
			t.Errorf("scenario %s: settlement should NEVER be successful", scenario.Name)
		}
	}
}

func TestG2_OnlyAllowedSettlementStatuses(t *testing.T) {
	allowedStatuses := map[execution.SettlementStatus]bool{
		execution.SettlementBlocked: true,
		execution.SettlementRevoked: true,
		execution.SettlementExpired: true,
		execution.SettlementAborted: true,
	}

	scenarios := []*Scenario{
		DefaultScenario(),
		RevocationScenario(),
		PlaidStubScenario(),
	}

	for _, scenario := range scenarios {
		runner := NewRunner()
		result, err := runner.RunScenario(scenario)

		if err != nil {
			continue
		}

		if !allowedStatuses[result.ExecutionResult.Status] {
			t.Errorf("scenario %s: unexpected status %s", scenario.Name, result.ExecutionResult.Status)
		}
	}
}

// Category H: GuardedExecutionError

func TestH1_GuardedExecutionErrorIsDetectable(t *testing.T) {
	err := &execution.GuardedExecutionError{
		EnvelopeID: "env_test",
		Provider:   "test-provider",
		Reason:     "test reason",
		BlockedAt:  time.Now(),
	}

	if !execution.IsGuardedExecutionError(err) {
		t.Error("IsGuardedExecutionError should return true")
	}
}

func TestH2_NonGuardedErrorsAreNotFalsePositive(t *testing.T) {
	regularErr := fmt.Errorf("not a guarded error")

	if execution.IsGuardedExecutionError(regularErr) {
		t.Error("IsGuardedExecutionError should return false for non-guarded errors")
	}
}

func TestH3_GuardedExecutionErrorHasUsefulMessage(t *testing.T) {
	err := &execution.GuardedExecutionError{
		EnvelopeID: "env_123",
		Provider:   "mock-finance",
		Reason:     "execution disabled",
		BlockedAt:  time.Now(),
	}

	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}

	// Should contain useful information
	if !containsAll(msg, "mock-finance", "env_123", "execution disabled") {
		t.Errorf("error message missing expected content: %s", msg)
	}
}

// Category I: Adapter Prepare Validation

func TestI1_PrepareValidatesEnvelope(t *testing.T) {
	idGen := func() string { return "test" }
	emitter := func(e events.Event) {}
	adapter := execution.NewMockFinanceAdapter(idGen, emitter)

	// Test with nil envelope
	_, err := adapter.Prepare(nil)
	if err == nil {
		t.Error("prepare should fail with nil envelope")
	}
}

func TestI2_PrepareRejectsExpiredEnvelope(t *testing.T) {
	idGen := func() string { return "test" }
	emitter := func(e events.Event) {}
	adapter := execution.NewMockFinanceAdapter(idGen, emitter)

	now := time.Now()
	envelope := &execution.ExecutionEnvelope{
		EnvelopeID: "env_test",
		SealHash:   "sealed_hash",           // Non-empty means sealed
		Expiry:     now.Add(-1 * time.Hour), // Already expired
		ActionSpec: execution.ActionSpec{
			AmountCents: 1000,
		},
	}

	result, err := adapter.Prepare(envelope)
	if err == nil {
		t.Error("prepare should fail with expired envelope")
	}
	if result != nil && result.Valid {
		t.Error("prepare result should be invalid")
	}
}

func TestI3_PrepareRejectsUnsealedEnvelope(t *testing.T) {
	idGen := func() string { return "test" }
	emitter := func(e events.Event) {}
	adapter := execution.NewMockFinanceAdapter(idGen, emitter)

	envelope := &execution.ExecutionEnvelope{
		EnvelopeID: "env_test",
		SealHash:   "", // Empty means not sealed
		Expiry:     time.Now().Add(1 * time.Hour),
		ActionSpec: execution.ActionSpec{
			AmountCents: 1000,
		},
	}

	result, err := adapter.Prepare(envelope)
	if err == nil {
		t.Error("prepare should fail with unsealed envelope")
	}
	if result != nil && result.Valid {
		t.Error("prepare result should be invalid")
	}
}

// Helper function
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
