// Package demo provides demo-specific components for the suggest-only vertical slice.
package demo

import (
	"context"
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/primitives"
)

// TestDemoCalendarSuggest runs the full demo and validates outputs.
func TestDemoCalendarSuggest(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Demo was not successful: %v", result.Error)
	}

	// Verify circle was created
	if result.CircleID == "" {
		t.Error("Expected CircleID to be set")
	}

	// Verify trace ID was set
	if result.TraceID == "" {
		t.Error("Expected TraceID to be set")
	}
}

// TestNoExecutionLayerInvoked ensures no execution layer is invoked.
// The suggest-only orchestrator should never call real execution.
func TestNoExecutionLayerInvoked(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Check that all audit entries are suggest-only related
	for _, entry := range result.AuditLog {
		// Verify no "execute" or "write" actions in audit
		if strings.Contains(entry.Action, "execute_external") {
			t.Errorf("Found external execution in audit: %s", entry.Action)
		}
		if strings.Contains(entry.Action, "write_external") {
			t.Errorf("Found external write in audit: %s", entry.Action)
		}
		if strings.Contains(entry.Action, "connector_call") {
			t.Errorf("Found connector call in audit: %s", entry.Action)
		}
	}

	// Verify suggestions have suggest-only indicators
	for i, sug := range result.Suggestions {
		if sug.Description == "" {
			t.Errorf("Suggestion %d has empty description", i)
		}
		// All suggestions should be about proposing/suggesting, not executing
		if strings.Contains(strings.ToLower(sug.Description), "executed") {
			t.Errorf("Suggestion %d appears to describe execution: %s", i, sug.Description)
		}
	}
}

// TestAuditEntriesExistForEachStep ensures audit entries exist for each loop step.
func TestAuditEntriesExistForEachStep(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Expected steps in the irreducible loop
	expectedSteps := []string{
		"intent",
		"intersection_discovery",
		"authority_negotiation",
		"commitment",
		"action",
		"settlement",
		"memory_update",
	}

	// Track which steps we found (started and completed)
	stepsStarted := make(map[string]bool)
	stepsCompleted := make(map[string]bool)

	for _, entry := range result.AuditLog {
		for _, step := range expectedSteps {
			if strings.Contains(entry.EventType, step+".started") {
				stepsStarted[step] = true
			}
			if strings.Contains(entry.EventType, step+".completed") {
				stepsCompleted[step] = true
			}
		}
	}

	// Verify all steps were started and completed
	for _, step := range expectedSteps {
		if !stepsStarted[step] {
			t.Errorf("Step %s was not started (no audit entry)", step)
		}
		if !stepsCompleted[step] {
			t.Errorf("Step %s was not completed (no audit entry)", step)
		}
	}

	// Verify loop completion event exists
	foundLoopComplete := false
	for _, entry := range result.AuditLog {
		if entry.EventType == "loop.completed" {
			foundLoopComplete = true
			if entry.Outcome != "success" {
				t.Errorf("Loop completed with non-success outcome: %s", entry.Outcome)
			}
		}
	}
	if !foundLoopComplete {
		t.Error("No loop.completed audit entry found")
	}

	// Verify all entries have trace ID
	for _, entry := range result.AuditLog {
		if entry.TraceID == "" {
			t.Errorf("Audit entry %s has no TraceID", entry.ID)
		}
		if entry.TraceID != result.TraceID {
			t.Errorf("Audit entry %s has wrong TraceID: got %s, want %s",
				entry.ID, entry.TraceID, result.TraceID)
		}
	}
}

// TestSuggestionsAreDeterministic ensures suggestions are produced deterministically.
func TestSuggestionsAreDeterministic(t *testing.T) {
	// Run the demo multiple times
	const runs = 3
	var allSuggestions [][]string

	for i := 0; i < runs; i++ {
		runner := NewRunner()
		result, err := runner.Run(context.Background())

		if err != nil {
			t.Fatalf("Demo run %d failed: %v", i, err)
		}

		// Extract suggestion descriptions
		var descriptions []string
		for _, sug := range result.Suggestions {
			descriptions = append(descriptions, sug.Description)
		}
		allSuggestions = append(allSuggestions, descriptions)
	}

	// Verify all runs produced the same suggestions
	if len(allSuggestions) < 2 {
		t.Fatal("Not enough runs to compare")
	}

	baseline := allSuggestions[0]
	for i := 1; i < len(allSuggestions); i++ {
		if len(allSuggestions[i]) != len(baseline) {
			t.Errorf("Run %d produced %d suggestions, baseline had %d",
				i, len(allSuggestions[i]), len(baseline))
			continue
		}

		for j := range baseline {
			if allSuggestions[i][j] != baseline[j] {
				t.Errorf("Run %d suggestion %d differs: got %q, want %q",
					i, j, allSuggestions[i][j], baseline[j])
			}
		}
	}
}

// TestSuggestionsHaveExplanations ensures each suggestion has a "why" explanation.
func TestSuggestionsHaveExplanations(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	if len(result.Suggestions) == 0 {
		t.Fatal("No suggestions produced")
	}

	for i, sug := range result.Suggestions {
		if sug.Explanation == "" {
			t.Errorf("Suggestion %d has no explanation", i)
		}
		if sug.TimeSlot == "" {
			t.Errorf("Suggestion %d has no time slot", i)
		}
	}
}

// TestSuggestionsProducedFromCalendar ensures suggestions are based on calendar data.
func TestSuggestionsProducedFromCalendar(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// Suggestions should mention days of the week (from calendar analysis)
	daysFound := make(map[string]bool)
	for _, sug := range result.Suggestions {
		for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"} {
			if strings.Contains(sug.TimeSlot, day) || strings.Contains(sug.Description, day) {
				daysFound[day] = true
			}
		}
	}

	if len(daysFound) == 0 {
		t.Error("No day of week mentioned in suggestions - may not be calendar-based")
	}
}

// TestMinimumSuggestionsProduced ensures at least the expected number of suggestions.
func TestMinimumSuggestionsProduced(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Demo run failed: %v", err)
	}

	// The demo should produce at least 3 suggestions
	const minSuggestions = 3
	if len(result.Suggestions) < minSuggestions {
		t.Errorf("Expected at least %d suggestions, got %d", minSuggestions, len(result.Suggestions))
	}
}

// TestCalendarFreeSlotDetection tests the calendar free slot detection.
func TestCalendarFreeSlotDetection(t *testing.T) {
	calendar := NewMockCalendar()

	// Get free slots of at least 1 hour
	freeSlots := calendar.GetFreeSlots(1 * time.Hour)

	if len(freeSlots) == 0 {
		t.Fatal("No free slots found in mock calendar")
	}

	// Verify there's an evening slot on a weekday (after 5pm)
	foundWeekdayEvening := false
	for _, slot := range freeSlots {
		if slot.DayName != "Saturday" && slot.DayName != "Sunday" {
			if slot.Start.Hour() >= 17 || slot.End.Hour() >= 18 {
				foundWeekdayEvening = true
				break
			}
		}
	}

	if !foundWeekdayEvening {
		t.Error("Expected to find a weekday evening as a free slot")
	}
}

// TestSuggestionEngineIsDeterministic tests the suggestion engine directly.
func TestSuggestionEngineIsDeterministic(t *testing.T) {
	calendar := NewMockCalendar()
	engine := NewDeterministicSuggestionEngine(calendar)

	ctx := context.Background()
	loopCtx := createTestLoopContext()

	// Generate suggestions twice
	sug1, err := engine.GenerateSuggestions(ctx, loopCtx, nil)
	if err != nil {
		t.Fatalf("First generation failed: %v", err)
	}

	sug2, err := engine.GenerateSuggestions(ctx, loopCtx, nil)
	if err != nil {
		t.Fatalf("Second generation failed: %v", err)
	}

	if len(sug1) != len(sug2) {
		t.Fatalf("Different number of suggestions: %d vs %d", len(sug1), len(sug2))
	}

	for i := range sug1 {
		if sug1[i].Description != sug2[i].Description {
			t.Errorf("Suggestion %d differs: %q vs %q", i, sug1[i].Description, sug2[i].Description)
		}
	}
}

// createTestLoopContext creates a test loop context.
func createTestLoopContext() primitives.LoopContext {
	return primitives.LoopContext{
		TraceID:        "test-trace",
		IssuerCircleID: "test-circle",
		CreatedAt:      time.Now(),
		RiskClass:      primitives.RiskLow,
		AutonomyMode:   "suggest_only",
		CurrentStep:    primitives.StepIntent,
	}
}
