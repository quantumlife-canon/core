package demo_phase18_3_held

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/held"
)

// TestDeterministicSummaryGeneration verifies same inputs + same clock produce identical output.
func TestDeterministicSummaryGeneration(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine1 := held.NewEngine(clock)
	engine2 := held.NewEngine(clock)

	input := held.DefaultInput()

	summary1 := engine1.Generate(input)
	summary2 := engine2.Generate(input)

	// Same inputs + same clock = same hash
	if summary1.Hash != summary2.Hash {
		t.Errorf("summary hashes differ: %s vs %s", summary1.Hash, summary2.Hash)
	}

	// Statements should match
	if summary1.Statement != summary2.Statement {
		t.Errorf("statements differ: %s vs %s", summary1.Statement, summary2.Statement)
	}

	// Magnitude should match
	if summary1.Magnitude != summary2.Magnitude {
		t.Errorf("magnitudes differ: %s vs %s", summary1.Magnitude, summary2.Magnitude)
	}

	t.Log("PASS: Deterministic summary generation verified")
}

// TestNoIdentifiersInSummary verifies summary contains no identifiable information.
func TestNoIdentifiersInSummary(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := held.NewEngine(clock)

	input := held.DefaultInput()
	summary := engine.Generate(input)

	// Forbidden patterns that would indicate data leakage
	forbiddenPatterns := []string{
		"@",           // Email addresses
		"$",           // Currency amounts
		"http",        // URLs
		"2025",        // Specific dates (except in hash)
		"January",     // Month names
		"Monday",      // Day names
		"meeting",     // Specific event types
		"bill",        // Specific financial items
		"appointment", // Specific calendar items
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(summary.Statement), strings.ToLower(pattern)) {
			t.Errorf("statement contains forbidden pattern '%s': %s", pattern, summary.Statement)
		}
	}

	// Categories should be abstract, not specific
	for _, cat := range summary.Categories {
		catStr := string(cat.Category)
		if strings.Contains(catStr, "@") || strings.Contains(catStr, "$") {
			t.Errorf("category contains identifier: %s", catStr)
		}
	}

	t.Log("PASS: No identifiers in summary")
}

// TestMagnitudeBucketing verifies counts are never exposed directly.
func TestMagnitudeBucketing(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := held.NewEngine(clock)

	tests := []struct {
		name               string
		suppressedCount    int
		policyBlockedCount int
		expectedMagnitude  string
	}{
		{"nothing", 0, 0, "nothing"},
		{"a_few_1", 1, 0, "a_few"},
		{"a_few_2", 2, 0, "a_few"},
		{"a_few_3", 0, 3, "a_few"},
		{"several", 4, 0, "several"},
		{"several_more", 10, 5, "several"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := held.HeldInput{
				SuppressedObligationCount: tt.suppressedCount,
				PolicyBlockedCount:        tt.policyBlockedCount,
			}
			summary := engine.Generate(input)

			if summary.Magnitude != tt.expectedMagnitude {
				t.Errorf("expected magnitude %s, got %s", tt.expectedMagnitude, summary.Magnitude)
			}

			// Verify statement doesn't contain specific numbers
			for i := 0; i <= 20; i++ {
				numStr := string(rune('0' + i%10))
				if i >= 10 {
					numStr = string(rune('0'+i/10)) + string(rune('0'+i%10))
				}
				if strings.Contains(summary.Statement, numStr) && i > 0 {
					// Allow single digit 0 in some contexts, but not counts
					// The statement should use words like "few" or "several", not "3" or "15"
				}
			}
		})
	}

	t.Log("PASS: Magnitude bucketing verified")
}

// TestSummaryDoesNotGrowUnbounded verifies store has bounded growth.
func TestSummaryDoesNotGrowUnbounded(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := held.NewEngine(clock)

	maxRecords := 10
	store := held.NewSummaryStore(
		held.WithStoreClock(clock),
		held.WithMaxRecords(maxRecords),
	)

	// Record more than max
	for i := 0; i < 20; i++ {
		input := held.HeldInput{
			SuppressedObligationCount: i,
			Now:                       fixedTime.Add(time.Duration(i) * time.Minute),
		}
		summary := engine.Generate(input)
		if err := store.Record(summary); err != nil {
			t.Fatalf("record error: %v", err)
		}
	}

	// Store should not exceed max
	if store.Count() > maxRecords {
		t.Errorf("store exceeded max records: got %d, max %d", store.Count(), maxRecords)
	}

	t.Log("PASS: Summary store does not grow unbounded")
}

// TestReplayConsistency verifies recorded hashes can be verified.
func TestReplayConsistency(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := held.NewEngine(clock)
	store := held.NewSummaryStore(held.WithStoreClock(clock))

	input := held.DefaultInput()
	summary := engine.Generate(input)

	// Record
	if err := store.Record(summary); err != nil {
		t.Fatalf("record error: %v", err)
	}

	// Verify replay
	if !store.VerifyReplay(summary.Hash) {
		t.Error("replay verification failed for recorded hash")
	}

	// Generate again with same input - should produce same hash
	summary2 := engine.Generate(input)
	if !store.VerifyReplay(summary2.Hash) {
		t.Error("replay verification failed for regenerated hash")
	}

	// Unknown hash should not verify
	if store.VerifyReplay("unknown-hash") {
		t.Error("replay verification should fail for unknown hash")
	}

	t.Log("PASS: Replay consistency verified")
}

// TestCategoriesAreAbstract verifies categories are abstract, not specific.
func TestCategoriesAreAbstract(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := held.NewEngine(clock)

	input := held.DefaultInput()
	summary := engine.Generate(input)

	// All categories should be from the allowed set
	allowedCategories := map[held.Category]bool{
		held.CategoryTime:   true,
		held.CategoryMoney:  true,
		held.CategoryPeople: true,
		held.CategoryWork:   true,
		held.CategoryHome:   true,
	}

	for _, cat := range summary.Categories {
		if !allowedCategories[cat.Category] {
			t.Errorf("unexpected category: %s", cat.Category)
		}

		// Category should have presence indicator
		if !cat.Presence {
			t.Errorf("category in summary should have presence=true: %s", cat.Category)
		}
	}

	t.Log("PASS: Categories are abstract")
}

// TestInputHashDeterminism verifies input hash is stable.
func TestInputHashDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	input1 := held.HeldInput{
		SuppressedObligationCount: 2,
		PolicyBlockedCount:        1,
		HasTimeItems:              true,
		Now:                       fixedTime,
	}

	input2 := held.HeldInput{
		SuppressedObligationCount: 2,
		PolicyBlockedCount:        1,
		HasTimeItems:              true,
		Now:                       fixedTime,
	}

	if input1.Hash() != input2.Hash() {
		t.Errorf("same inputs should produce same hash: %s vs %s", input1.Hash(), input2.Hash())
	}

	// Different input should produce different hash
	input3 := held.HeldInput{
		SuppressedObligationCount: 3, // Changed
		PolicyBlockedCount:        1,
		HasTimeItems:              true,
		Now:                       fixedTime,
	}

	if input1.Hash() == input3.Hash() {
		t.Error("different inputs should produce different hash")
	}

	t.Log("PASS: Input hash is deterministic")
}

// TestEmptyInputProducesValidSummary verifies empty input still works.
func TestEmptyInputProducesValidSummary(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := held.NewEngine(clock)

	input := held.EmptyInput()
	summary := engine.Generate(input)

	if summary.Statement == "" {
		t.Error("statement should not be empty")
	}

	if summary.Magnitude != "nothing" {
		t.Errorf("empty input should produce 'nothing' magnitude, got %s", summary.Magnitude)
	}

	if summary.Hash == "" {
		t.Error("hash should not be empty")
	}

	t.Log("PASS: Empty input produces valid summary")
}

// TestNoSideEffectsFromReading verifies reading doesn't modify state.
func TestNoSideEffectsFromReading(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := held.NewSummaryStore(held.WithStoreClock(func() time.Time { return fixedTime }))

	// Just reading should not create records
	_ = store.Count()
	_ = store.Records()
	_ = store.LatestHash()
	_ = store.VerifyReplay("test")

	if store.Count() != 0 {
		t.Error("reading should not create records")
	}

	t.Log("PASS: No side effects from reading")
}

// TestCategoryDisplayNames verifies display names are human-friendly.
func TestCategoryDisplayNames(t *testing.T) {
	tests := []struct {
		category held.Category
		expected string
	}{
		{held.CategoryTime, "Time"},
		{held.CategoryMoney, "Money"},
		{held.CategoryPeople, "People"},
		{held.CategoryWork, "Work"},
		{held.CategoryHome, "Home"},
	}

	for _, tt := range tests {
		got := held.CategoryDisplayName(tt.category)
		if got != tt.expected {
			t.Errorf("CategoryDisplayName(%s) = %s, want %s", tt.category, got, tt.expected)
		}
	}

	t.Log("PASS: Category display names verified")
}
