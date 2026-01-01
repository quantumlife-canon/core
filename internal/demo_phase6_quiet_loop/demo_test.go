package demo_phase6_quiet_loop

import (
	"strings"
	"testing"
)

func TestRunDemo(t *testing.T) {
	result := RunDemo()

	if result.Err != nil {
		t.Fatalf("RunDemo() error = %v", result.Err)
	}

	if result.Output == "" {
		t.Error("RunDemo() produced empty output")
	}

	// Verify key outputs are present
	expectedPhrases := []string{
		"PHASE 6: THE QUIET LOOP DEMO",
		"SCENARIO 1: EMPTY STATE",
		"NOTHING NEEDS YOU",
		"SCENARIO 2: WITH EVENTS",
		"SCENARIO 3: FEEDBACK CAPTURE",
		"SCENARIO 4: DETERMINISM CHECK",
		"Run IDs are identical (deterministic)",
		"AUDIT EVENTS EMITTED",
		"DEMO COMPLETE",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(result.Output, phrase) {
			t.Errorf("Output missing expected phrase: %q", phrase)
		}
	}

	// Verify determinism check shows either match or appropriate message
	hasDeterminism := strings.Contains(result.Output, "hashes are identical (deterministic)") ||
		strings.Contains(result.Output, "hashes differ")
	if !hasDeterminism {
		t.Error("Output missing determinism check for hashes")
	}
}

func TestDemoDeterminism(t *testing.T) {
	// Run the demo twice and verify same output
	// Each run creates fresh stores, so output should be identical
	result1 := RunDemo()
	result2 := RunDemo()

	if result1.Err != nil {
		t.Fatalf("First RunDemo() error = %v", result1.Err)
	}

	if result2.Err != nil {
		t.Fatalf("Second RunDemo() error = %v", result2.Err)
	}

	// Output should be identical (deterministic)
	// Each call creates fresh engines/stores with fixed clock
	if result1.Output != result2.Output {
		t.Error("Demo output is not deterministic - two runs produced different output")
	}
}
