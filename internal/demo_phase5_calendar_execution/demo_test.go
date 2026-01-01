package demo_phase5_calendar_execution

import (
	"testing"
	"time"
)

func fixedClock() time.Time {
	return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
}

func TestDemo_RunFullFlow(t *testing.T) {
	demo := NewDemo(fixedClock)

	result := demo.RunFullFlow()

	// Verify draft was generated
	if !result.DraftGenerated {
		t.Error("expected draft to be generated")
	}

	// Verify draft was approved
	if !result.DraftApproved {
		t.Error("expected draft to be approved")
	}

	// Verify envelope was created
	if !result.EnvelopeCreated {
		t.Error("expected envelope to be created")
	}

	// Verify execution succeeded
	if !result.ExecutionSuccess {
		t.Error("expected execution to succeed")
	}

	// Verify provider response
	if result.ProviderResponseID == "" {
		t.Error("expected provider response ID")
	}
}

func TestDemo_RunIdempotencyDemo(t *testing.T) {
	demo := NewDemo(fixedClock)

	firstResult, secondResult, callCount := demo.RunIdempotencyDemo()

	// Both executions should succeed
	if !firstResult.Success {
		t.Errorf("first execution should succeed: %s", firstResult.Error)
	}
	if !secondResult.Success {
		t.Errorf("second execution should succeed: %s", secondResult.Error)
	}

	// Both should return same provider response ID (idempotency)
	if firstResult.ProviderResponseID != secondResult.ProviderResponseID {
		t.Errorf("idempotency failed: first=%s, second=%s",
			firstResult.ProviderResponseID, secondResult.ProviderResponseID)
	}

	// Mock writer should only be called once (idempotency)
	if callCount != 1 {
		t.Errorf("expected 1 call to mock writer, got %d", callCount)
	}
}

func TestDemo_RunPolicyMismatchDemo(t *testing.T) {
	demo := NewDemo(fixedClock)

	result := demo.RunPolicyMismatchDemo()

	// Execution should be blocked
	if result.Success {
		t.Error("expected execution to be blocked due to policy mismatch")
	}

	// Should be marked as blocked
	if !result.Blocked {
		t.Error("expected result.Blocked to be true")
	}

	// Should have a blocked reason
	if result.BlockedReason == "" {
		t.Error("expected blocked reason")
	}
}

func TestDemo_RunViewStaleDemo(t *testing.T) {
	demo := NewDemo(fixedClock)

	result := demo.RunViewStaleDemo()

	// Execution should be blocked
	if result.Success {
		t.Error("expected execution to be blocked due to stale view")
	}

	// Should be marked as blocked
	if !result.Blocked {
		t.Error("expected result.Blocked to be true")
	}
}
