package demo_phase18_2_today_quietly

import (
	"testing"
	"time"

	"quantumlife/internal/todayquietly"
)

// TestDeterministicPageGeneration verifies same inputs + same clock produce identical output.
func TestDeterministicPageGeneration(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine1 := todayquietly.NewEngine(clock)
	engine2 := todayquietly.NewEngine(clock)

	input := todayquietly.DefaultInput()

	page1 := engine1.Generate(input)
	page2 := engine2.Generate(input)

	// Same inputs + same clock = same page hash
	if page1.PageHash != page2.PageHash {
		t.Errorf("page hashes differ: %s vs %s", page1.PageHash, page2.PageHash)
	}

	// Title and subtitle are always the same
	if page1.Title != "Today, quietly." {
		t.Errorf("expected title 'Today, quietly.', got %s", page1.Title)
	}
	if page1.Subtitle != "Nothing needs you — unless it truly does." {
		t.Errorf("unexpected subtitle: %s", page1.Subtitle)
	}

	t.Log("PASS: Deterministic page generation verified")
}

// TestExactlyThreeObservations verifies engine always produces exactly 3 observations.
func TestExactlyThreeObservations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := todayquietly.NewEngine(clock)

	// Test with full signals
	fullInput := todayquietly.DefaultInput()
	page1 := engine.Generate(fullInput)
	if len(page1.Observations) != 3 {
		t.Errorf("expected 3 observations, got %d", len(page1.Observations))
	}

	// Test with minimal signals (should still produce 3 via fallbacks)
	minimalInput := todayquietly.ProjectionInput{
		CircleCount: 1,
	}
	page2 := engine.Generate(minimalInput)
	if len(page2.Observations) != 3 {
		t.Errorf("expected 3 observations with minimal input, got %d", len(page2.Observations))
	}

	// Test with no signals (should use all fallbacks)
	emptyInput := todayquietly.ProjectionInput{}
	page3 := engine.Generate(emptyInput)
	if len(page3.Observations) != 3 {
		t.Errorf("expected 3 observations with empty input, got %d", len(page3.Observations))
	}

	t.Log("PASS: Exactly 3 observations verified")
}

// TestExactlyOneSuppressedInsight verifies engine always produces exactly 1 suppressed insight.
func TestExactlyOneSuppressedInsight(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := todayquietly.NewEngine(clock)

	input := todayquietly.DefaultInput()
	page := engine.Generate(input)

	// Suppressed insight must always be present
	if page.SuppressedInsight.Title == "" {
		t.Error("suppressed insight title is empty")
	}
	if page.SuppressedInsight.Reason == "" {
		t.Error("suppressed insight reason is empty")
	}

	// Check specific content
	expectedTitle := "There's one thing we chose not to surface yet."
	if page.SuppressedInsight.Title != expectedTitle {
		t.Errorf("unexpected suppressed insight title: %s", page.SuppressedInsight.Title)
	}

	expectedReason := "Because it doesn't need you today."
	if page.SuppressedInsight.Reason != expectedReason {
		t.Errorf("unexpected suppressed insight reason: %s", page.SuppressedInsight.Reason)
	}

	t.Log("PASS: Exactly 1 suppressed insight verified")
}

// TestPreferenceStoreRecording verifies preference recording works correctly.
func TestPreferenceStoreRecording(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := todayquietly.NewPreferenceStore(
		todayquietly.WithStoreClock(func() time.Time { return fixedTime }),
	)

	// First recording should succeed
	isNew, err := store.Record("quiet", "web")
	if err != nil {
		t.Fatalf("recording error: %v", err)
	}
	if !isNew {
		t.Error("first recording should be new")
	}

	// Count should be 1
	if store.Count() != 1 {
		t.Errorf("expected count=1, got %d", store.Count())
	}

	// Latest preference should be "quiet"
	if store.LatestPreference() != "quiet" {
		t.Errorf("expected latest preference 'quiet', got %s", store.LatestPreference())
	}

	t.Log("PASS: Preference store recording works correctly")
}

// TestPreferenceStoreDifferentModesProduceDifferentHashes verifies different preferences produce different hashes.
func TestPreferenceStoreDifferentModesProduceDifferentHashes(t *testing.T) {
	time1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	time2 := time.Date(2025, 1, 15, 10, 1, 0, 0, time.UTC) // 1 minute later

	store1 := todayquietly.NewPreferenceStore(
		todayquietly.WithStoreClock(func() time.Time { return time1 }),
	)
	store2 := todayquietly.NewPreferenceStore(
		todayquietly.WithStoreClock(func() time.Time { return time2 }),
	)

	store1.Record("quiet", "web")
	store2.Record("show_all", "web")

	records1 := store1.Records()
	records2 := store2.Records()

	if len(records1) != 1 || len(records2) != 1 {
		t.Fatal("expected 1 record in each store")
	}

	// Different mode + different time = different hash
	if records1[0].Hash == records2[0].Hash {
		t.Error("different preferences should produce different hashes")
	}

	t.Log("PASS: Different preferences produce different hashes")
}

// TestPreferenceStoreInvalidMode verifies invalid modes are rejected.
func TestPreferenceStoreInvalidMode(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := todayquietly.NewPreferenceStore(
		todayquietly.WithStoreClock(func() time.Time { return fixedTime }),
	)

	// Invalid mode should return error
	_, err := store.Record("invalid_mode", "web")
	if err == nil {
		t.Error("invalid mode should return error")
	}

	// Count should be 0
	if store.Count() != 0 {
		t.Errorf("expected count=0 after invalid mode, got %d", store.Count())
	}

	t.Log("PASS: Invalid mode rejected")
}

// TestObservationIDsAreDeterministic verifies observation IDs are stable.
func TestObservationIDsAreDeterministic(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine1 := todayquietly.NewEngine(clock)
	engine2 := todayquietly.NewEngine(clock)

	input := todayquietly.DefaultInput()

	page1 := engine1.Generate(input)
	page2 := engine2.Generate(input)

	// Observation IDs should be identical
	for i := range page1.Observations {
		if page1.Observations[i].ID != page2.Observations[i].ID {
			t.Errorf("observation ID mismatch at index %d: %s vs %s",
				i, page1.Observations[i].ID, page2.Observations[i].ID)
		}
	}

	t.Log("PASS: Observation IDs are deterministic")
}

// TestPermissionPivotStructure verifies permission pivot has exactly 2 choices.
func TestPermissionPivotStructure(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := todayquietly.NewEngine(clock)

	input := todayquietly.DefaultInput()
	page := engine.Generate(input)

	// Must have exactly 2 choices
	if len(page.PermissionPivot.Choices) != 2 {
		t.Errorf("expected 2 permission choices, got %d", len(page.PermissionPivot.Choices))
	}

	// Default choice should be "quiet"
	if page.PermissionPivot.DefaultChoice != "quiet" {
		t.Errorf("expected default choice 'quiet', got %s", page.PermissionPivot.DefaultChoice)
	}

	// Verify the modes
	modes := make(map[string]bool)
	for _, choice := range page.PermissionPivot.Choices {
		modes[choice.Mode] = true
	}
	if !modes["quiet"] {
		t.Error("missing 'quiet' mode")
	}
	if !modes["show_all"] {
		t.Error("missing 'show_all' mode")
	}

	t.Log("PASS: Permission pivot structure verified")
}

// TestConfirmationMessages verifies confirmation messages are correct.
func TestConfirmationMessages(t *testing.T) {
	quietMsg := todayquietly.ConfirmationMessage("quiet")
	if quietMsg != "Noted. QuantumLife will stay quiet unless it truly matters." {
		t.Errorf("unexpected quiet confirmation: %s", quietMsg)
	}

	showAllMsg := todayquietly.ConfirmationMessage("show_all")
	if showAllMsg != "Noted. We'll show you everything — and help you silence it later." {
		t.Errorf("unexpected show_all confirmation: %s", showAllMsg)
	}

	defaultMsg := todayquietly.ConfirmationMessage("unknown")
	if defaultMsg != "Preference recorded." {
		t.Errorf("unexpected default confirmation: %s", defaultMsg)
	}

	t.Log("PASS: Confirmation messages verified")
}

// TestRecognitionSentenceNoRawCounts verifies recognition sentence contains no raw counts.
func TestRecognitionSentenceNoRawCounts(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := todayquietly.NewEngine(clock)

	// Test with various inputs
	inputs := []todayquietly.ProjectionInput{
		{CircleCount: 5, HasWorkObligations: true, HasFamilyObligations: true},
		{CircleCount: 10, HasFinanceObligations: true},
		{CircleCount: 1},
	}

	for _, input := range inputs {
		page := engine.Generate(input)

		// Recognition should not contain digits
		for _, c := range page.Recognition {
			if c >= '0' && c <= '9' {
				t.Errorf("recognition sentence contains digit: %s", page.Recognition)
				break
			}
		}
	}

	t.Log("PASS: Recognition sentence contains no raw counts")
}

// TestProjectionInputHash verifies projection input hash is deterministic.
func TestProjectionInputHash(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	input1 := todayquietly.ProjectionInput{
		HasWorkObligations:   true,
		HasFamilyObligations: true,
		CircleCount:          3,
		Now:                  fixedTime,
	}

	input2 := todayquietly.ProjectionInput{
		HasWorkObligations:   true,
		HasFamilyObligations: true,
		CircleCount:          3,
		Now:                  fixedTime,
	}

	// Same inputs = same hash
	if input1.Hash() != input2.Hash() {
		t.Errorf("same inputs should produce same hash: %s vs %s", input1.Hash(), input2.Hash())
	}

	// Different inputs = different hash
	input3 := todayquietly.ProjectionInput{
		HasWorkObligations:   true,
		HasFamilyObligations: false, // Changed
		CircleCount:          3,
		Now:                  fixedTime,
	}
	if input1.Hash() == input3.Hash() {
		t.Error("different inputs should produce different hash")
	}

	t.Log("PASS: Projection input hash is deterministic")
}

// TestNoSideEffectsFromReading verifies reading doesn't modify state.
func TestNoSideEffectsFromReading(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := todayquietly.NewPreferenceStore(
		todayquietly.WithStoreClock(func() time.Time { return fixedTime }),
	)

	// Just reading should not create records
	_ = store.Count()
	_ = store.Records()
	_ = store.LatestPreference()

	if store.Count() != 0 {
		t.Error("reading should not create records")
	}

	t.Log("PASS: No side effects from reading")
}
