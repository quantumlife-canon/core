package demo_phase7_email_execution

import (
	"testing"
)

func TestRunAllScenarios(t *testing.T) {
	results := RunAllScenarios()

	if len(results) != 6 {
		t.Errorf("expected 6 scenarios, got %d", len(results))
	}

	for _, r := range results {
		if !r.Success {
			t.Errorf("scenario '%s' failed: status=%s, error=%s",
				r.Scenario, r.Status, r.Error)
		}
		t.Logf("Scenario: %s - Success: %t, Status: %s", r.Scenario, r.Success, r.Status)
	}
}

func TestScenario1_SuccessfulSend(t *testing.T) {
	result := RunScenario1_SuccessfulSend()

	if !result.Success {
		t.Errorf("expected success, got status=%s, error=%s", result.Status, result.Error)
	}

	if result.MessageID == "" {
		t.Error("expected message ID to be set")
	}

	if result.Status != "executed" {
		t.Errorf("expected status 'executed', got %s", result.Status)
	}
}

func TestScenario2_MissingHashBlocks(t *testing.T) {
	result := RunScenario2_MissingHashBlocks()

	if !result.Success {
		t.Errorf("expected success (blocked is correct behavior), got status=%s", result.Status)
	}

	if result.Status != "blocked" {
		t.Errorf("expected status 'blocked', got %s", result.Status)
	}
}

func TestScenario3_ViewMismatchBlocks(t *testing.T) {
	result := RunScenario3_ViewMismatchBlocks()

	if !result.Success {
		t.Errorf("expected success (blocked is correct behavior), got status=%s", result.Status)
	}

	if result.Status != "blocked" {
		t.Errorf("expected status 'blocked', got %s", result.Status)
	}
}

func TestScenario4_StaleViewBlocks(t *testing.T) {
	result := RunScenario4_StaleViewBlocks()

	if !result.Success {
		t.Errorf("expected success (blocked is correct behavior), got status=%s", result.Status)
	}

	if result.Status != "blocked" {
		t.Errorf("expected status 'blocked', got %s", result.Status)
	}
}

func TestScenario5_PolicyDriftBlocks(t *testing.T) {
	result := RunScenario5_PolicyDriftBlocks()

	if !result.Success {
		t.Errorf("expected success (blocked is correct behavior), got status=%s", result.Status)
	}

	if result.Status != "blocked" {
		t.Errorf("expected status 'blocked', got %s", result.Status)
	}
}

func TestScenario6_IdempotencyWorks(t *testing.T) {
	result := RunScenario6_IdempotencyWorks()

	if !result.Success {
		t.Errorf("expected success, got status=%s, error=%s", result.Status, result.Error)
	}

	if result.Status != "executed" {
		t.Errorf("expected status 'executed', got %s", result.Status)
	}
}
