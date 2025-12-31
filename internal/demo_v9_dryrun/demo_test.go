package demo_v9_dryrun

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// =============================================================================
// v9 Acceptance Tests (per ACCEPTANCE_TESTS_V9_EXECUTION.md)
// =============================================================================

// Category A: Consent and Silence

func TestA1_SilenceResultsInNoExecution(t *testing.T) {
	// Setup: Intent created, no approval submitted
	runner := NewRunner()

	scenario := &Scenario{
		Name:        "silence-test",
		Description: "No approval submitted",
		Intent: execution.ExecutionIntent{
			CircleID:    "circle_test",
			ActionType:  execution.ActionTypePayment,
			AmountCents: 1000,
			Currency:    "GBP",
			PayeeID:     "sandbox-utility",
		},
		ShouldRevoke:   false,
		ExpectedStatus: execution.SettlementBlocked, // Will be blocked due to no approval
	}

	// Create intent only (don't run full scenario)
	now := time.Now()
	intent := scenario.Intent
	intent.IntentID = "test_intent"
	intent.ViewHash = "test_view_hash"
	intent.CreatedAt = now

	// Build envelope but don't add approvals
	envelope, err := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                2000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		RevocationWaived:         false,
		TraceID:                  "test_trace",
	}, now)
	if err != nil {
		t.Fatalf("envelope build failed: %v", err)
	}

	// Attempt execution without approval (after revocation window)
	execTime := now.Add(10 * time.Minute)
	result, err := runner.executionRunner.Execute(envelope, execTime)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Expected: Execution blocked due to insufficient approvals
	if result.Status == execution.SettlementSuccessful {
		t.Error("CRITICAL: Execution succeeded without approval")
	}
	if result.Status != execution.SettlementBlocked {
		t.Errorf("expected blocked, got %s", result.Status)
	}
	if !strings.Contains(result.BlockedReason, "approval") {
		t.Errorf("expected approval-related block reason, got: %s", result.BlockedReason)
	}
}

func TestA2_ClosingWindowWithoutApprovalResultsInNoExecution(t *testing.T) {
	// Similar to A1 - no approval = no execution
	runner := NewRunner()
	now := time.Now()

	intent := execution.ExecutionIntent{
		IntentID:    "test_intent_a2",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 500,
		Currency:    "GBP",
		PayeeID:     "sandbox-utility",
		ViewHash:    "test_view_hash",
		CreatedAt:   now,
	}

	envelope, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                1000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "test_trace_a2",
	}, now)

	// Try to execute - should be blocked
	result, _ := runner.executionRunner.Execute(envelope, now.Add(10*time.Minute))

	if result.Status == execution.SettlementSuccessful {
		t.Error("CRITICAL: Execution succeeded without approval")
	}
}

// Category B: Approval Neutrality

func TestB1_ApprovalPromptContainsNoUrgencyLanguage(t *testing.T) {
	runner := NewRunner()
	now := time.Now()

	intent := execution.ExecutionIntent{
		IntentID:    "test_intent_b1",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 5000,
		Currency:    "GBP",
		PayeeID:     "sandbox-utility", // Potentially urgent context
		ViewHash:    "test_view_hash",
		CreatedAt:   now,
	}

	envelope, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                10000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "test_trace_b1",
	}, now)

	request, err := runner.approvalManager.CreateApprovalRequest(
		envelope, "circle_test", now.Add(30*time.Minute), now)
	if err != nil {
		t.Fatalf("approval request failed: %v", err)
	}

	// Check prompt for forbidden language
	forbiddenWords := []string{"urgent", "immediately", "now", "hurry", "deadline", "last chance"}
	promptLower := strings.ToLower(request.PromptText)

	for _, word := range forbiddenWords {
		if strings.Contains(promptLower, word) {
			t.Errorf("approval prompt contains forbidden urgency word: %s", word)
		}
	}
}

func TestB2_ApprovalPromptContainsNoFearLanguage(t *testing.T) {
	checker := execution.NewApprovalLanguageChecker()

	fearPhrases := []string{
		"This is a dangerous situation",
		"Risk of losing money",
		"Warning: payment required",
		"Problem with your payment",
	}

	for _, phrase := range fearPhrases {
		violations := checker.Check(phrase)
		if len(violations) == 0 {
			t.Errorf("expected fear violation for: %s", phrase)
		}

		foundFear := false
		for _, v := range violations {
			if v.Category == "fear" {
				foundFear = true
				break
			}
		}
		if !foundFear {
			t.Errorf("expected fear category violation for: %s", phrase)
		}
	}
}

func TestB3_ApprovalPromptContainsNoAuthorityLanguage(t *testing.T) {
	checker := execution.NewApprovalLanguageChecker()

	authorityPhrases := []string{
		"We recommend this payment",
		"You should approve this",
		"This is the best option",
		"Advised action: pay now",
	}

	for _, phrase := range authorityPhrases {
		violations := checker.Check(phrase)
		if len(violations) == 0 {
			t.Errorf("expected authority violation for: %s", phrase)
		}
	}
}

func TestB4_ApprovalPromptContainsNoOptimizationLanguage(t *testing.T) {
	checker := execution.NewApprovalLanguageChecker()

	optimizationPhrases := []string{
		"Save money by paying now",
		"Optimize your spending",
		"Better rate available",
		"Improve your finances",
	}

	for _, phrase := range optimizationPhrases {
		violations := checker.Check(phrase)
		if len(violations) == 0 {
			t.Errorf("expected optimization violation for: %s", phrase)
		}
	}
}

// Category C: No Standing Approval

func TestC1_ApprovalBoundToSpecificActionHash(t *testing.T) {
	now := time.Now()

	// Create two different intents
	intent1 := execution.ExecutionIntent{
		IntentID:    "intent_1",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 1000,
		Currency:    "GBP",
		PayeeID:     "sandbox-merchant",
		ViewHash:    "view_1",
		CreatedAt:   now,
	}

	intent2 := execution.ExecutionIntent{
		IntentID:    "intent_2",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 1000, // Same amount
		Currency:    "GBP",
		PayeeID:     "sandbox-merchant", // Same recipient
		ViewHash:    "view_2",
		CreatedAt:   now.Add(1 * time.Second),
	}

	// Compute action hashes
	hash1 := execution.ComputeActionHash(intent1)
	hash2 := execution.ComputeActionHash(intent2)

	// Hashes must be different even for similar payments
	if hash1 == hash2 {
		t.Error("different intents should have different action hashes")
	}
}

func TestC3_ApprovalReuseRejected(t *testing.T) {
	runner := NewRunner()
	now := time.Now()

	intent := execution.ExecutionIntent{
		IntentID:    "intent_c3",
		CircleID:    "circle_test",
		ActionType:  execution.ActionTypePayment,
		AmountCents: 1000,
		Currency:    "GBP",
		PayeeID:     "sandbox-utility",
		ViewHash:    "view_c3",
		CreatedAt:   now,
	}

	envelope1, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                2000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_c3_1",
	}, now)

	// Create approval for envelope1
	request1, _ := runner.approvalManager.CreateApprovalRequest(
		envelope1, "circle_test", now.Add(30*time.Minute), now)
	approval1, _ := runner.approvalManager.SubmitApproval(
		request1, "circle_test", "user_test", now.Add(25*time.Minute), now)

	// Create second envelope with different intent
	intent2 := intent
	intent2.IntentID = "intent_c3_2"
	intent2.CreatedAt = now.Add(1 * time.Second)

	envelope2, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent2,
		AmountCap:                2000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_c3_2",
	}, now)

	// Try to use approval1 for envelope2 - should fail
	err := runner.approvalVerifier.VerifyApproval(approval1, envelope2.ActionHash, now)
	if err == nil {
		t.Error("approval reuse should be rejected")
	}
	if !strings.Contains(err.Error(), "different ActionHash") {
		t.Errorf("expected ActionHash mismatch error, got: %v", err)
	}
}

// Category D: Revocation

func TestD1_PreExecutionRevocationBlocksExecution(t *testing.T) {
	result, _ := NewRunner().RunScenario(&Scenario{
		Name:             "revocation-test",
		Description:      "Pre-execution revocation",
		Intent:           DefaultScenario().Intent,
		ShouldRevoke:     true,
		RevocationReason: "Test revocation",
		ExpectedStatus:   execution.SettlementRevoked,
	})

	if result.ExecutionResult.Status != execution.SettlementRevoked {
		t.Errorf("expected revoked, got %s", result.ExecutionResult.Status)
	}
}

func TestD3_NoFinishWhatYouStartedBehavior(t *testing.T) {
	runner := NewRunner()
	now := time.Now()

	intent := DefaultScenario().Intent
	intent.IntentID = "intent_d3"
	intent.ViewHash = "view_d3"
	intent.CreatedAt = now

	envelope, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                10000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_d3",
	}, now)

	// Add approval
	request, _ := runner.approvalManager.CreateApprovalRequest(
		envelope, intent.CircleID, now.Add(30*time.Minute), now)
	approval, _ := runner.approvalManager.SubmitApproval(
		request, intent.CircleID, "user_test", now.Add(25*time.Minute), now)
	envelope.Approvals = append(envelope.Approvals, *approval)

	// Revoke right before execution
	runner.revocationChecker.Revoke(
		envelope.EnvelopeID, intent.CircleID, "user_test", "changed my mind", now)

	// Execute - should be immediately blocked
	result, _ := runner.executionRunner.Execute(envelope, now.Add(10*time.Minute))

	if result.Status != execution.SettlementRevoked {
		t.Errorf("expected revoked, got %s", result.Status)
	}

	// There should be no "completing" or "finishing" behavior
	if result.Status == execution.SettlementSuccessful {
		t.Error("CRITICAL: Execution completed despite revocation")
	}
}

// Category E: Validity Check

func TestE1_AffirmativeValidityCheckRequired(t *testing.T) {
	runner := NewRunner()
	now := time.Now()

	intent := DefaultScenario().Intent
	intent.IntentID = "intent_e1"
	intent.ViewHash = "view_e1"
	intent.CreatedAt = now

	envelope, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                10000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(1 * time.Hour),
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_e1",
	}, now)

	// Add approval
	request, _ := runner.approvalManager.CreateApprovalRequest(
		envelope, intent.CircleID, now.Add(30*time.Minute), now)
	approval, _ := runner.approvalManager.SubmitApproval(
		request, intent.CircleID, "user_test", now.Add(25*time.Minute), now)
	envelope.Approvals = append(envelope.Approvals, *approval)

	// Execute after revocation window
	execTime := now.Add(10 * time.Minute)
	result, _ := runner.executionRunner.Execute(envelope, execTime)

	// Validity check must have been performed
	if result.ValidityCheck.CheckedAt.IsZero() {
		t.Error("validity check was not performed")
	}

	// Check that conditions were actually evaluated
	if len(result.ValidityCheck.Conditions) == 0 {
		t.Error("no conditions were checked")
	}

	// Each condition should have been evaluated
	expectedConditions := []string{
		"envelope_not_expired",
		"envelope_not_revoked",
		"no_revocation_signal",
		"sufficient_approvals",
		"amount_within_cap",
		"revocation_window_closed",
	}

	for _, expected := range expectedConditions {
		found := false
		for _, cond := range result.ValidityCheck.Conditions {
			if cond.Condition == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected condition %s was not checked", expected)
		}
	}
}

func TestE3_ExpiredApprovalDetectedByValidityCheck(t *testing.T) {
	runner := NewRunner()
	now := time.Now()

	intent := DefaultScenario().Intent
	intent.IntentID = "intent_e3"
	intent.ViewHash = "view_e3"
	intent.CreatedAt = now

	envelope, _ := runner.envelopeBuilder.Build(execution.BuildRequest{
		Intent:                   intent,
		AmountCap:                10000,
		FrequencyCap:             1,
		DurationCap:              24 * time.Hour,
		Expiry:                   now.Add(2 * time.Hour), // Envelope valid for 2 hours
		ApprovalThreshold:        1,
		RevocationWindowDuration: 5 * time.Minute,
		TraceID:                  "trace_e3",
	}, now)

	// Add approval that expires in 10 minutes
	request, _ := runner.approvalManager.CreateApprovalRequest(
		envelope, intent.CircleID, now.Add(30*time.Minute), now)
	approval, _ := runner.approvalManager.SubmitApproval(
		request, intent.CircleID, "user_test", now.Add(10*time.Minute), now) // Expires in 10 min
	envelope.Approvals = append(envelope.Approvals, *approval)

	// Try to execute after approval expires (but before envelope expires)
	execTime := now.Add(20 * time.Minute)
	result, _ := runner.executionRunner.Execute(envelope, execTime)

	// Should be blocked due to expired approval
	if result.Status == execution.SettlementSuccessful {
		t.Error("CRITICAL: Execution succeeded with expired approval")
	}
}

// Category G: No Retry

func TestG1_FailedExecutionDoesNotAutoRetry(t *testing.T) {
	runner := NewRunner()

	// Run scenario that results in revocation
	result, _ := runner.RunScenario(&Scenario{
		Name:             "no-retry-test",
		Description:      "Test no auto-retry",
		Intent:           DefaultScenario().Intent,
		ShouldRevoke:     true,
		RevocationReason: "Test",
		ExpectedStatus:   execution.SettlementRevoked,
	})

	// Count execution.started events
	startedCount := 0
	for _, event := range result.AuditEvents {
		if event.Type == events.EventV9ExecutionStarted {
			startedCount++
		}
	}

	// Should only be one execution attempt
	if startedCount != 1 {
		t.Errorf("expected exactly 1 execution attempt, got %d", startedCount)
	}
}

// Category H: Audit Reconstruction

func TestH1_PlainLanguageExplanationAlwaysPossible(t *testing.T) {
	result, _ := NewRunner().Run()

	// Must have audit events
	if len(result.AuditEvents) == 0 {
		t.Fatal("no audit events recorded")
	}

	// Must be able to identify key events
	hasIntentCreated := false
	hasEnvelopeSealed := false
	hasApprovalRequested := false
	hasApprovalSubmitted := false
	hasSettlement := false

	for _, event := range result.AuditEvents {
		switch event.Type {
		case events.EventExecutionIntentCreated:
			hasIntentCreated = true
			// Must have readable metadata
			if event.Metadata["amount"] == "" {
				t.Error("intent event missing amount")
			}
		case events.EventExecutionEnvelopeSealed:
			hasEnvelopeSealed = true
		case events.EventV9ApprovalRequested:
			hasApprovalRequested = true
			if event.Metadata["prompt_text"] == "" {
				t.Error("approval request missing prompt text")
			}
		case events.EventV9ApprovalSubmitted:
			hasApprovalSubmitted = true
		case events.EventV9SettlementRevoked, events.EventV9SettlementAborted,
			events.EventV9SettlementBlocked, events.EventV9SettlementExpired:
			hasSettlement = true
		}
	}

	if !hasIntentCreated {
		t.Error("missing intent.created event")
	}
	if !hasEnvelopeSealed {
		t.Error("missing envelope.sealed event")
	}
	if !hasApprovalRequested {
		t.Error("missing approval.requested event")
	}
	if !hasApprovalSubmitted {
		t.Error("missing approval.submitted event")
	}
	if !hasSettlement {
		t.Error("missing settlement event")
	}
}

func TestH2_AuthorityPathFullyTraceable(t *testing.T) {
	result, _ := NewRunner().Run()

	// Find approval events
	var approvalRequested, approvalSubmitted, approvalVerified bool
	var approverID string

	for _, event := range result.AuditEvents {
		switch event.Type {
		case events.EventV9ApprovalRequested:
			approvalRequested = true
		case events.EventV9ApprovalSubmitted:
			approvalSubmitted = true
			approverID = event.Metadata["approver_id"]
		case events.EventV9ApprovalVerified:
			approvalVerified = true
		}
	}

	if !approvalRequested || !approvalSubmitted || !approvalVerified {
		t.Error("incomplete approval trail")
	}

	if approverID == "" {
		t.Error("approver identity not recorded")
	}
}

// Category I: Failure Defaults

func TestI_FailureDefaultsToNonExecution(t *testing.T) {
	runner := NewRunner()

	// All scenarios should result in non-success in dry-run mode
	scenarios := []*Scenario{
		DefaultScenario(),
		NoRevocationScenario(),
	}

	for _, scenario := range scenarios {
		result, err := runner.RunScenario(scenario)
		if err != nil {
			t.Errorf("scenario %s failed: %v", scenario.Name, err)
			continue
		}

		if result.ExecutionResult.Status == execution.SettlementSuccessful {
			t.Errorf("scenario %s: CRITICAL - execution succeeded in dry-run mode", scenario.Name)
		}

		runner.ClearAuditLog()
	}
}

// Category J: Anti-Drift Guards

func TestJ_NoSuccessfulSettlementInDryRunMode(t *testing.T) {
	runner := NewRunner()

	// Run multiple scenarios
	for i := 0; i < 5; i++ {
		result, _ := runner.Run()

		if result.ExecutionResult.Status == execution.SettlementSuccessful {
			t.Fatal("CRITICAL VIOLATION: Successful settlement in dry-run mode")
		}

		// Check audit log for any successful settlement events
		for _, event := range result.AuditEvents {
			if event.Type == "v9.settlement.successful" {
				t.Fatal("CRITICAL VIOLATION: Successful settlement event in dry-run mode")
			}
		}

		runner.ClearAuditLog()
	}
}

func TestJ_MoneyMovedAlwaysFalse(t *testing.T) {
	result, _ := NewRunner().Run()

	// Find the audit finalization event
	for _, event := range result.AuditEvents {
		if event.Type == events.EventV9AuditTraceFinalized {
			if event.Metadata["money_moved"] != "false" {
				t.Errorf("money_moved should be false, got: %s", event.Metadata["money_moved"])
			}
			return
		}
	}

	t.Error("missing audit finalization event")
}

// =============================================================================
// Demo Runs Successfully Test
// =============================================================================

func TestDemoRunsSuccessfully(t *testing.T) {
	result, err := NewRunner().Run()
	if err != nil {
		t.Fatalf("demo failed: %v", err)
	}

	if !result.Success {
		t.Errorf("demo did not succeed: %s", result.FailureReason)
	}

	// Verify key outcomes
	if result.ExecutionResult == nil {
		t.Fatal("no execution result")
	}

	if result.ExecutionResult.Status == execution.SettlementSuccessful {
		t.Fatal("CRITICAL: Execution succeeded in dry-run mode")
	}

	if result.ExecutionResult.Status != execution.SettlementRevoked {
		t.Errorf("expected revoked status, got %s", result.ExecutionResult.Status)
	}
}

func TestDemoAuditTrailComplete(t *testing.T) {
	result, _ := NewRunner().Run()

	// Must have a reasonable number of events
	if len(result.AuditEvents) < 10 {
		t.Errorf("expected at least 10 audit events, got %d", len(result.AuditEvents))
	}

	// Must end with audit finalization
	lastEvent := result.AuditEvents[len(result.AuditEvents)-1]
	if lastEvent.Type != events.EventV9AuditTraceFinalized {
		t.Errorf("last event should be audit finalization, got %s", lastEvent.Type)
	}
}
