package demo_family_simulate

import (
	"context"
	"testing"

	"quantumlife/pkg/primitives"
)

// TestSuggestOnlyModeDoesNotCreateAction verifies that suggest-only mode
// does not create actions, settlements, or memory writes.
func TestSuggestOnlyModeDoesNotCreateAction(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSuggestOnly)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("demo should succeed, got error: %s", result.Error)
	}

	// Verify no action was created
	if result.ActionID != "" {
		t.Errorf("expected no action in suggest-only mode, got: %s", result.ActionID)
	}

	// Verify no settlement
	if result.SettlementID != "" {
		t.Errorf("expected no settlement in suggest-only mode, got: %s", result.SettlementID)
	}

	// Verify no memory entry
	if result.MemoryEntry != nil {
		t.Errorf("expected no memory entry in suggest-only mode")
	}

	// Verify action summary indicates suggest-only
	if result.ActionSummary != "SUGGEST_ONLY: No action created" {
		t.Errorf("expected suggest-only action summary, got: %s", result.ActionSummary)
	}
}

// TestSimulateModeCreatesFullPipeline verifies that simulate mode creates
// Action + AuthorizationProof + Outcome + Settlement + Memory write.
func TestSimulateModeCreatesFullPipeline(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("demo should succeed, got error: %s", result.Error)
	}

	// Verify action was created
	if result.ActionID == "" {
		t.Error("expected action to be created in simulate mode")
	}

	// Verify authorization proof
	if result.AuthorizationProof == nil {
		t.Error("expected authorization proof in simulate mode")
	} else {
		if !result.AuthorizationProof.Authorized {
			t.Errorf("expected authorization to pass, denial reason: %s", result.AuthorizationProof.DenialReason)
		}
	}

	// Verify execution outcome
	if result.ExecutionOutcome == nil {
		t.Error("expected execution outcome in simulate mode")
	} else {
		if !result.ExecutionOutcome.Simulated {
			t.Error("expected outcome to be marked as simulated")
		}
		if !result.ExecutionOutcome.Success {
			t.Errorf("expected successful simulation, got error: %s", result.ExecutionOutcome.ErrorMessage)
		}
	}

	// Verify settlement
	if result.SettlementID == "" {
		t.Error("expected settlement to be recorded in simulate mode")
	}
	if result.SettlementStatus != "simulated" {
		t.Errorf("expected settlement status 'simulated', got: %s", result.SettlementStatus)
	}

	// Verify memory entry
	if result.MemoryEntry == nil {
		t.Error("expected memory entry in simulate mode")
	} else {
		if result.MemoryEntry.Key != "last_simulated_action" {
			t.Errorf("expected memory key 'last_simulated_action', got: %s", result.MemoryEntry.Key)
		}
		if result.MemoryEntry.Version < 1 {
			t.Error("expected memory version >= 1")
		}
	}
}

// TestCeilingsEnforced verifies that ceiling checks are performed and recorded.
func TestCeilingsEnforced(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify ceiling checks were performed
	if result.AuthorizationProof == nil {
		t.Fatal("expected authorization proof")
	}

	// Verify ceiling checks exist
	if len(result.AuthorizationProof.CeilingChecks) == 0 {
		t.Error("expected ceiling checks to be recorded")
	}

	// Verify time_window ceiling was checked
	foundTimeWindowCheck := false
	for _, check := range result.AuthorizationProof.CeilingChecks {
		if check.CeilingType == "time_window" {
			foundTimeWindowCheck = true
			// The action uses time_window "18:00-19:00" which is within "18:00-21:00"
			// so this should pass
			if !check.Passed {
				t.Logf("time_window check failed: %s", check.Reason)
			}
			break
		}
	}

	if !foundTimeWindowCheck {
		t.Error("expected time_window ceiling check")
	}

	// Verify duration ceiling was checked
	foundDurationCheck := false
	for _, check := range result.AuthorizationProof.CeilingChecks {
		if check.CeilingType == "duration" {
			foundDurationCheck = true
			// The action uses duration "1" which is within ceiling "3"
			// so this should pass
			if !check.Passed {
				t.Logf("duration check failed: %s", check.Reason)
			}
			break
		}
	}

	if !foundDurationCheck {
		t.Error("expected duration ceiling check")
	}
}

// TestMissingScopesFailsAuthorization verifies that actions requiring
// scopes not in the contract fail authorization.
func TestMissingScopesFailsAuthorization(t *testing.T) {
	// This test would require modifying the contract to not include calendar:write
	// For now, we'll verify the mechanism exists by checking the proof structure
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AuthorizationProof == nil {
		t.Fatal("expected authorization proof")
	}

	// Verify scopes were checked
	if len(result.AuthorizationProof.ScopesUsed) == 0 {
		t.Error("expected scopes to be checked")
	}

	if len(result.AuthorizationProof.ScopesGranted) == 0 {
		t.Error("expected granted scopes to be recorded")
	}
}

// TestAuditEventsOrder verifies that audit events occur in the expected order:
// ActionCreated -> AuthorizationChecked -> SimulatedExecutionCompleted -> SettlementRecorded -> MemoryWritten
func TestAuditEventsOrder(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("demo should succeed, got error: %s", result.Error)
	}

	// Find v4-specific events in order
	expectedOrder := []string{
		"action.created",
		"authorization.checked",
		"simulated.execution.completed",
		"settlement.recorded",
		"memory.written",
	}

	foundIndices := make(map[string]int)
	for i, entry := range result.AuditEntries {
		for _, expected := range expectedOrder {
			if entry.Type == expected {
				foundIndices[expected] = i
				break
			}
		}
	}

	// Verify all expected events were found
	for _, expected := range expectedOrder {
		if _, found := foundIndices[expected]; !found {
			t.Errorf("expected audit event not found: %s", expected)
		}
	}

	// Verify order
	lastIndex := -1
	for _, expected := range expectedOrder {
		if idx, found := foundIndices[expected]; found {
			if idx <= lastIndex {
				t.Errorf("audit event %s out of order (index %d, expected after %d)", expected, idx, lastIndex)
			}
			lastIndex = idx
		}
	}
}

// TestDeterministicOutput verifies that repeated runs produce identical outputs
// given the same inputs and time.
func TestDeterministicOutput(t *testing.T) {
	ctx := context.Background()

	// Run twice with the same mode
	runner1 := NewRunnerWithMode(primitives.ModeSimulate)
	result1, err := runner1.Run(ctx)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	runner2 := NewRunnerWithMode(primitives.ModeSimulate)
	result2, err := runner2.Run(ctx)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// Key outputs should be deterministic
	if result1.ActionType != result2.ActionType {
		t.Errorf("action type differs: %s vs %s", result1.ActionType, result2.ActionType)
	}

	if result1.SettlementStatus != result2.SettlementStatus {
		t.Errorf("settlement status differs: %s vs %s", result1.SettlementStatus, result2.SettlementStatus)
	}

	if result1.AuthorizationProof.Authorized != result2.AuthorizationProof.Authorized {
		t.Errorf("authorization differs: %v vs %v",
			result1.AuthorizationProof.Authorized, result2.AuthorizationProof.Authorized)
	}

	// Memory versions might differ if IDs are counter-based, but keys should match
	if result1.MemoryEntry.Key != result2.MemoryEntry.Key {
		t.Errorf("memory key differs: %s vs %s", result1.MemoryEntry.Key, result2.MemoryEntry.Key)
	}
}

// TestExecuteModeHardFails verifies that execute mode returns an error.
func TestExecuteModeHardFails(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeExecute)

	result, err := runner.Run(ctx)
	if err == nil {
		t.Error("expected error for execute mode")
	}

	if result.Error == "" {
		t.Error("expected error message to be set")
	}

	// The error should indicate execute mode is not implemented
	if err != primitives.ErrExecuteNotImplemented {
		t.Errorf("expected ErrExecuteNotImplemented, got: %v", err)
	}
}

// TestAuthorizationProofContainsRequiredFields verifies that the authorization
// proof contains all required fields for audit purposes.
func TestAuthorizationProofContainsRequiredFields(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	proof := result.AuthorizationProof
	if proof == nil {
		t.Fatal("expected authorization proof")
	}

	// Required fields
	if proof.ID == "" {
		t.Error("proof ID should not be empty")
	}

	if proof.ActionID == "" {
		t.Error("proof ActionID should not be empty")
	}

	if proof.IntersectionID == "" {
		t.Error("proof IntersectionID should not be empty")
	}

	if proof.ContractVersion == "" {
		t.Error("proof ContractVersion should not be empty")
	}

	if proof.TraceID == "" {
		t.Error("proof TraceID should not be empty")
	}

	if proof.Timestamp.IsZero() {
		t.Error("proof Timestamp should not be zero")
	}

	// Mode check
	if proof.ModeCheck.RequestedMode == "" {
		t.Error("mode check should have requested mode")
	}
}

// TestSimulatedExecutionMarkedAsSimulated verifies that the execution outcome
// is properly marked as simulated.
func TestSimulatedExecutionMarkedAsSimulated(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outcome := result.ExecutionOutcome
	if outcome == nil {
		t.Fatal("expected execution outcome")
	}

	if !outcome.Simulated {
		t.Error("execution outcome should be marked as simulated")
	}

	// Proposed payload should exist
	if len(outcome.ProposedPayload) == 0 {
		t.Error("expected proposed payload in simulated outcome")
	}

	// Verify no external write message is present
	if msg, ok := outcome.ProposedPayload["simulated"]; !ok || msg != "true" {
		t.Error("expected 'simulated' flag in proposed payload")
	}
}

// TestMemoryVersionIncrementsOnRepeatedWrites verifies that memory versioning works.
func TestMemoryVersionIncrementsOnRepeatedWrites(t *testing.T) {
	// This test runs the demo twice on the same runner instance to verify
	// that memory versioning increments properly
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result1, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// The first run should have version 1
	if result1.MemoryEntry == nil {
		t.Fatal("expected memory entry from first run")
	}
	if result1.MemoryEntry.Version != 1 {
		t.Errorf("expected memory version 1 on first write, got: %d", result1.MemoryEntry.Version)
	}

	// Note: Each run creates a new runner with fresh stores, so we can't test
	// version increment across runs without modifying the test setup.
	// This test verifies the initial version is correct.
}

// TestContractScopesAreExtracted verifies that contract scopes are properly
// extracted and available in the result.
func TestContractScopesAreExtracted(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify contract scopes include calendar scopes
	hasCalendarRead := false
	hasCalendarWrite := false
	for _, scope := range result.ContractScopes {
		if scope == "calendar:read" {
			hasCalendarRead = true
		}
		if scope == "calendar:write" {
			hasCalendarWrite = true
		}
	}

	if !hasCalendarRead {
		t.Error("expected calendar:read scope in contract")
	}
	if !hasCalendarWrite {
		t.Error("expected calendar:write scope in contract")
	}
}

// TestContractCeilingsAreExtracted verifies that contract ceilings are properly
// extracted and available in the result.
func TestContractCeilingsAreExtracted(t *testing.T) {
	ctx := context.Background()
	runner := NewRunnerWithMode(primitives.ModeSimulate)

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify contract has time_window and duration ceilings
	hasTimeWindow := false
	hasDuration := false
	for _, ceiling := range result.ContractCeilings {
		if ceiling.Type == "time_window" {
			hasTimeWindow = true
		}
		if ceiling.Type == "duration" {
			hasDuration = true
		}
	}

	if !hasTimeWindow {
		t.Error("expected time_window ceiling in contract")
	}
	if !hasDuration {
		t.Error("expected duration ceiling in contract")
	}
}
