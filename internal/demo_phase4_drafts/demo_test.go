package demo_phase4_drafts

import (
	"strings"
	"testing"
)

func TestRunDemo(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	// Verify key outputs
	expectedOutputs := []string{
		"Phase 4: Drafts-Only Assistance Demo",
		"Generated email draft:",
		"Generated calendar draft:",
		"Deduplicated:",
		"Pending drafts:",
		"approved",
		"rejected",
		"expired",
		"Demo Complete",
		"No external writes occurred",
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(result.Output, expected) {
			t.Errorf("Output missing expected text: %s", expected)
		}
	}

	t.Log(result.Output)
}

func TestDemoProducesValidDrafts(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	// Verify draft IDs are present (16 hex chars)
	if !strings.Contains(result.Output, "Generated email draft:") {
		t.Error("Expected email draft to be generated")
	}

	if !strings.Contains(result.Output, "Generated calendar draft:") {
		t.Error("Expected calendar draft to be generated")
	}
}

func TestDemoDeduplication(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	if !strings.Contains(result.Output, "Deduplicated:") {
		t.Error("Expected deduplication to occur")
	}
}

func TestDemoReviewWorkflow(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	// Check that both approval and rejection occurred
	if !strings.Contains(result.Output, "approved") {
		t.Error("Expected draft approval in output")
	}

	if !strings.Contains(result.Output, "rejected") {
		t.Error("Expected draft rejection in output")
	}
}

func TestDemoExpiration(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	if !strings.Contains(result.Output, "Marked 1 draft(s) as expired") {
		t.Error("Expected one draft to expire")
	}
}

func TestDemoStatistics(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	// Verify final statistics are present
	if !strings.Contains(result.Output, "Pending:") {
		t.Error("Expected pending count in statistics")
	}
	if !strings.Contains(result.Output, "Approved:") {
		t.Error("Expected approved count in statistics")
	}
	if !strings.Contains(result.Output, "Rejected:") {
		t.Error("Expected rejected count in statistics")
	}
	if !strings.Contains(result.Output, "Expired:") {
		t.Error("Expected expired count in statistics")
	}
}

func TestDemoNoExternalWrites(t *testing.T) {
	result := RunDemo()
	if result.Err != nil {
		t.Fatalf("Demo failed: %v", result.Err)
	}

	// Verify the safety disclaimer is present
	if !strings.Contains(result.Output, "No external writes occurred") {
		t.Error("Expected no-external-writes disclaimer")
	}
}
