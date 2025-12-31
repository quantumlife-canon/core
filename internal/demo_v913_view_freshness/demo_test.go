package demo_v913_view_freshness

import (
	"testing"
)

func TestViewFreshnessDemoScenarios(t *testing.T) {
	runner := NewRunner()
	results, err := runner.Run()
	if err != nil {
		t.Fatalf("Demo runner failed: %v", err)
	}

	for _, result := range results {
		t.Run(result.Scenario, func(t *testing.T) {
			PrintResult(result)

			if !result.Success {
				t.Errorf("Scenario %s failed", result.Scenario)
				for _, detail := range result.Details {
					t.Logf("  %s", detail)
				}
			}
		})
	}

	// Summary
	passed := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			passed++
		} else {
			failed++
		}
	}

	t.Logf("\nDemo Summary: %d passed, %d failed", passed, failed)

	if failed > 0 {
		t.Fatalf("Some demo scenarios failed")
	}
}

func TestS1ValidViewSnapshot(t *testing.T) {
	runner := NewRunner()
	result, err := runner.runValidViewScenario()
	if err != nil {
		t.Fatalf("Scenario failed: %v", err)
	}

	PrintResult(result)

	if !result.Success {
		t.Errorf("S1: Valid view scenario should pass")
	}
}

func TestS2StaleViewBlocks(t *testing.T) {
	runner := NewRunner()
	result, err := runner.runStaleViewScenario()
	if err != nil {
		t.Fatalf("Scenario failed: %v", err)
	}

	PrintResult(result)

	if !result.Success {
		t.Errorf("S2: Stale view should block execution")
	}
}

func TestS3HashMismatchBlocks(t *testing.T) {
	runner := NewRunner()
	result, err := runner.runHashMismatchScenario()
	if err != nil {
		t.Fatalf("Scenario failed: %v", err)
	}

	PrintResult(result)

	if !result.Success {
		t.Errorf("S3: Hash mismatch should block execution")
	}
}

func TestS4MissingHashBlocks(t *testing.T) {
	runner := NewRunner()
	result, err := runner.runMissingHashScenario()
	if err != nil {
		t.Fatalf("Scenario failed: %v", err)
	}

	PrintResult(result)

	if !result.Success {
		t.Errorf("S4: Missing hash should block execution")
	}
}

func TestS5MultiPartySymmetry(t *testing.T) {
	runner := NewRunner()
	result, err := runner.runMultiPartySymmetryScenario()
	if err != nil {
		t.Fatalf("Scenario failed: %v", err)
	}

	PrintResult(result)

	if !result.Success {
		t.Errorf("S5: Multi-party with matching view hashes should pass")
	}
}
