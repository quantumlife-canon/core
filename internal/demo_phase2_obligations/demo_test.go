package demo_phase2_obligations

import (
	"testing"
	"time"

	"quantumlife/pkg/clock"
)

// TestDemoDeterminism verifies same inputs produce same outputs.
func TestDemoDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	fixedClock := clock.NewFixed(fixedTime)

	config := DemoConfig{
		Clock:    fixedClock,
		Scenario: ScenarioMixed,
		Verbose:  false, // No output during test
	}

	// Run twice
	result1, err := Run(config)
	if err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	result2, err := Run(config)
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	// Hash must be identical
	if result1.Hash != result2.Hash {
		t.Errorf("Hash mismatch: %s vs %s", result1.Hash, result2.Hash)
	}

	// NeedsYou must be identical
	if result1.NeedsYou != result2.NeedsYou {
		t.Errorf("NeedsYou mismatch: %v vs %v", result1.NeedsYou, result2.NeedsYou)
	}

	// Same number of obligations
	if len(result1.Obligations) != len(result2.Obligations) {
		t.Errorf("Obligation count mismatch: %d vs %d",
			len(result1.Obligations), len(result2.Obligations))
	}

	// Same obligation IDs in same order
	for i := range result1.Obligations {
		if result1.Obligations[i].ID != result2.Obligations[i].ID {
			t.Errorf("Obligation[%d] ID mismatch: %s vs %s",
				i, result1.Obligations[i].ID, result2.Obligations[i].ID)
		}
	}
}

// TestScenarioNeedsYou verifies NeedsYou scenario produces NeedsYou=true.
func TestScenarioNeedsYou(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	config := DemoConfig{
		Clock:    clock.NewFixed(fixedTime),
		Scenario: ScenarioNeedsYou,
		Verbose:  false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !result.NeedsYou {
		t.Error("Expected NeedsYou=true for ScenarioNeedsYou")
	}

	if len(result.NeedsYouReasons) == 0 {
		t.Error("Expected at least one NeedsYouReason")
	}

	if len(result.Obligations) == 0 {
		t.Error("Expected at least one obligation")
	}
}

// TestScenarioNothingNeedsYou verifies NothingNeedsYou scenario produces NeedsYou=false.
func TestScenarioNothingNeedsYou(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	config := DemoConfig{
		Clock:    clock.NewFixed(fixedTime),
		Scenario: ScenarioNothingNeedsYou,
		Verbose:  false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.NeedsYou {
		t.Errorf("Expected NeedsYou=false for ScenarioNothingNeedsYou, got reasons: %v",
			result.NeedsYouReasons)
	}
}

// TestMixedScenarioHasObligations verifies mixed scenario produces obligations.
func TestMixedScenarioHasObligations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	config := DemoConfig{
		Clock:    clock.NewFixed(fixedTime),
		Scenario: ScenarioMixed,
		Verbose:  false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have obligations from multiple circles
	circleCount := make(map[string]int)
	for _, o := range result.Obligations {
		circleCount[o.CircleID]++
	}

	if len(circleCount) < 2 {
		t.Errorf("Expected obligations from at least 2 circles, got %d", len(circleCount))
	}

	// Should have circle summaries
	if len(result.CircleSummaries) == 0 {
		t.Error("Expected circle summaries")
	}
}

// TestObligationOrdering verifies obligations are sorted correctly.
func TestObligationOrdering(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	config := DemoConfig{
		Clock:    clock.NewFixed(fixedTime),
		Scenario: ScenarioMixed,
		Verbose:  false,
	}

	result, err := Run(config)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Obligations should be sorted by horizon (today < 24h < 7d < someday)
	horizonOrder := map[string]int{
		"today":   0,
		"24h":     1,
		"7d":      2,
		"someday": 3,
	}

	for i := 1; i < len(result.Obligations); i++ {
		prev := result.Obligations[i-1]
		curr := result.Obligations[i]

		prevOrder := horizonOrder[prev.Horizon]
		currOrder := horizonOrder[curr.Horizon]

		if prevOrder > currOrder {
			t.Errorf("Obligation[%d] has worse horizon than Obligation[%d]: %s vs %s",
				i-1, i, prev.Horizon, curr.Horizon)
		}

		// Within same horizon, higher regret should come first
		if prevOrder == currOrder && prev.RegretScore < curr.RegretScore {
			t.Errorf("Obligation[%d] has lower regret than Obligation[%d] within same horizon: %.2f vs %.2f",
				i-1, i, prev.RegretScore, curr.RegretScore)
		}
	}
}

// TestHashStability verifies hash is stable across multiple runs.
func TestHashStability(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	config := DemoConfig{
		Clock:    clock.NewFixed(fixedTime),
		Scenario: ScenarioMixed,
		Verbose:  false,
	}

	var hashes []string
	for i := 0; i < 5; i++ {
		result, err := Run(config)
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		hashes = append(hashes, result.Hash)
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Hash changed between runs: %s vs %s", hashes[0], hashes[i])
		}
	}
}

// TestDifferentTimesProduceDifferentHashes verifies different clocks produce different results.
func TestDifferentTimesProduceDifferentHashes(t *testing.T) {
	time1 := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC) // 1 hour later

	config1 := DemoConfig{
		Clock:    clock.NewFixed(time1),
		Scenario: ScenarioMixed,
		Verbose:  false,
	}

	config2 := DemoConfig{
		Clock:    clock.NewFixed(time2),
		Scenario: ScenarioMixed,
		Verbose:  false,
	}

	result1, _ := Run(config1)
	result2, _ := Run(config2)

	// Different times should produce different hashes
	// (because ComputedAt is included in the hash)
	if result1.Hash == result2.Hash {
		t.Error("Expected different hashes for different times")
	}
}
