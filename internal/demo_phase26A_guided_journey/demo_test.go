// Package demo_phase26A_guided_journey demonstrates Phase 26A: Guided Journey.
//
// These tests verify:
// 1. Determinism: same inputs → same StatusHash
// 2. Precedence rules: connect → sync → mirror → today → action → done
// 3. Dismissal: when dismissed, journey returns done
// 4. Material state change: dismissal hash differs when state changes
// 5. Privacy: rendered strings contain no forbidden patterns
// 6. Hash-only persistence
//
// Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md
package demo_phase26A_guided_journey

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/journey"
	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/identity"
)

// Fixed test time for determinism
var testTime = time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)

func testClock() time.Time {
	return testTime
}

// TestDeterminism verifies that same inputs produce same StatusHash.
func TestDeterminism(t *testing.T) {
	inputs1 := &journey.JourneyInputs{
		CircleID:          "circle-1",
		HasGmail:          true,
		GmailMode:         "mock",
		HasSyncReceipt:    true,
		LastSyncMagnitude: persist.MagnitudeHandful,
		MirrorViewed:      false,
		ActionEligible:    false,
		Now:               testTime,
	}

	inputs2 := &journey.JourneyInputs{
		CircleID:          "circle-1",
		HasGmail:          true,
		GmailMode:         "mock",
		HasSyncReceipt:    true,
		LastSyncMagnitude: persist.MagnitudeHandful,
		MirrorViewed:      false,
		ActionEligible:    false,
		Now:               testTime,
	}

	hash1 := inputs1.ComputeStatusHash()
	hash2 := inputs2.ComputeStatusHash()

	if hash1 != hash2 {
		t.Errorf("Determinism violation: same inputs produced different hashes\n  hash1: %s\n  hash2: %s", hash1, hash2)
	}

	// Verify hash is 32 hex characters (128 bits)
	if len(hash1) != 32 {
		t.Errorf("Expected 32 character hash, got %d: %s", len(hash1), hash1)
	}
}

// TestPrecedenceConnect verifies step_connect when no Gmail.
func TestPrecedenceConnect(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID: "circle-1",
		HasGmail: false,
		Now:      testTime,
	}

	step := engine.NextStep(inputs)
	if step != journey.StepConnect {
		t.Errorf("Expected StepConnect, got %s", step)
	}

	page := engine.BuildPage(inputs)
	if page.CurrentStep != journey.StepConnect {
		t.Errorf("Expected page step StepConnect, got %s", page.CurrentStep)
	}

	if page.Title != "Start, quietly." {
		t.Errorf("Expected title 'Start, quietly.', got '%s'", page.Title)
	}
}

// TestPrecedenceSync verifies step_sync when Gmail connected but no sync.
func TestPrecedenceSync(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID:       "circle-1",
		HasGmail:       true,
		GmailMode:      "mock",
		HasSyncReceipt: false,
		Now:            testTime,
	}

	step := engine.NextStep(inputs)
	if step != journey.StepSync {
		t.Errorf("Expected StepSync, got %s", step)
	}

	page := engine.BuildPage(inputs)
	if page.Title != "One small read." {
		t.Errorf("Expected title 'One small read.', got '%s'", page.Title)
	}
}

// TestPrecedenceMirror verifies step_mirror after sync but before mirror viewed.
func TestPrecedenceMirror(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID:          "circle-1",
		HasGmail:          true,
		GmailMode:         "mock",
		HasSyncReceipt:    true,
		LastSyncMagnitude: persist.MagnitudeHandful,
		MirrorViewed:      false,
		Now:               testTime,
	}

	step := engine.NextStep(inputs)
	if step != journey.StepMirror {
		t.Errorf("Expected StepMirror, got %s", step)
	}

	page := engine.BuildPage(inputs)
	if page.Title != "Seen, quietly." {
		t.Errorf("Expected title 'Seen, quietly.', got '%s'", page.Title)
	}
}

// TestPrecedenceToday verifies step_today after mirror viewed.
func TestPrecedenceToday(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID:          "circle-1",
		HasGmail:          true,
		GmailMode:         "mock",
		HasSyncReceipt:    true,
		LastSyncMagnitude: persist.MagnitudeHandful,
		MirrorViewed:      true,
		ActionEligible:    false,
		Now:               testTime,
	}

	step := engine.NextStep(inputs)
	if step != journey.StepToday {
		t.Errorf("Expected StepToday, got %s", step)
	}

	page := engine.BuildPage(inputs)
	if page.Title != "Today, quietly." {
		t.Errorf("Expected title 'Today, quietly.', got '%s'", page.Title)
	}
}

// TestPrecedenceAction verifies step_action when action eligible.
func TestPrecedenceAction(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID:             "circle-1",
		HasGmail:             true,
		GmailMode:            "mock",
		HasSyncReceipt:       true,
		LastSyncMagnitude:    persist.MagnitudeHandful,
		MirrorViewed:         true,
		ActionEligible:       true,
		ActionUsedThisPeriod: false,
		Now:                  testTime,
	}

	step := engine.NextStep(inputs)
	if step != journey.StepAction {
		t.Errorf("Expected StepAction, got %s", step)
	}

	page := engine.BuildPage(inputs)
	if page.Title != "One action, reversible." {
		t.Errorf("Expected title 'One action, reversible.', got '%s'", page.Title)
	}
}

// TestDismissal verifies that dismissed journey returns StepDone.
func TestDismissal(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID: "circle-1",
		HasGmail: false,
		Now:      testTime,
	}

	// Compute current status hash
	currentHash := inputs.ComputeStatusHash()

	// Set dismissed status hash to match current
	inputs.DismissedStatusHash = currentHash

	step := engine.NextStep(inputs)
	if step != journey.StepDone {
		t.Errorf("Expected StepDone when dismissed, got %s", step)
	}

	page := engine.BuildPage(inputs)
	if !page.IsDone {
		t.Error("Expected page.IsDone to be true when dismissed")
	}

	if page.Title != "Done." {
		t.Errorf("Expected title 'Done.', got '%s'", page.Title)
	}
}

// TestMaterialStateChange verifies that dismissal hash differs when state changes.
func TestMaterialStateChange(t *testing.T) {
	inputsBefore := &journey.JourneyInputs{
		CircleID: "circle-1",
		HasGmail: false,
		Now:      testTime,
	}

	inputsAfter := &journey.JourneyInputs{
		CircleID:  "circle-1",
		HasGmail:  true,
		GmailMode: "real",
		Now:       testTime,
	}

	hashBefore := inputsBefore.ComputeStatusHash()
	hashAfter := inputsAfter.ComputeStatusHash()

	if hashBefore == hashAfter {
		t.Error("Expected different hashes when state changes (no gmail → gmail connected)")
	}

	// Verify that dismissal from before state doesn't block after state
	engine := journey.NewEngine(testClock)
	inputsAfter.DismissedStatusHash = hashBefore

	step := engine.NextStep(inputsAfter)
	if step == journey.StepDone {
		t.Error("Expected journey to continue when state changed after dismissal")
	}
}

// TestDismissalStore verifies hash-only persistence.
func TestDismissalStore(t *testing.T) {
	store := persist.NewJourneyDismissalStore(testClock)

	circleID := identity.EntityID("circle-1")
	periodKey := testTime.UTC().Format("2006-01-02")
	statusHash := "abc123def456abc123def456abc12345"

	// Record dismissal
	dismissalHash, err := store.RecordDismissal(circleID, periodKey, statusHash)
	if err != nil {
		t.Fatalf("RecordDismissal failed: %v", err)
	}

	// Verify hash is returned
	if dismissalHash == "" {
		t.Error("Expected non-empty dismissal hash")
	}

	// Verify is dismissed
	if !store.IsDismissedForPeriod(circleID, periodKey) {
		t.Error("Expected IsDismissedForPeriod to return true")
	}

	// Verify status hash is returned
	storedHash := store.GetDismissedStatusHash(circleID, periodKey)
	if storedHash != statusHash {
		t.Errorf("Expected status hash %s, got %s", statusHash, storedHash)
	}

	// Verify different period is not dismissed
	if store.IsDismissedForPeriod(circleID, "2024-06-16") {
		t.Error("Expected IsDismissedForPeriod to return false for different period")
	}
}

// TestPrivacyNoForbiddenTokens verifies no forbidden tokens in rendered strings.
func TestPrivacyNoForbiddenTokens(t *testing.T) {
	engine := journey.NewEngine(testClock)

	testCases := []struct {
		name   string
		inputs *journey.JourneyInputs
	}{
		{
			name: "connect step",
			inputs: &journey.JourneyInputs{
				CircleID: "circle-1",
				HasGmail: false,
				Now:      testTime,
			},
		},
		{
			name: "sync step",
			inputs: &journey.JourneyInputs{
				CircleID:       "circle-1",
				HasGmail:       true,
				GmailMode:      "mock",
				HasSyncReceipt: false,
				Now:            testTime,
			},
		},
		{
			name: "mirror step",
			inputs: &journey.JourneyInputs{
				CircleID:          "circle-1",
				HasGmail:          true,
				GmailMode:         "mock",
				HasSyncReceipt:    true,
				LastSyncMagnitude: persist.MagnitudeHandful,
				MirrorViewed:      false,
				Now:               testTime,
			},
		},
		{
			name: "action step",
			inputs: &journey.JourneyInputs{
				CircleID:             "circle-1",
				HasGmail:             true,
				GmailMode:            "mock",
				HasSyncReceipt:       true,
				LastSyncMagnitude:    persist.MagnitudeHandful,
				MirrorViewed:         true,
				ActionEligible:       true,
				ActionUsedThisPeriod: false,
				Now:                  testTime,
			},
		},
	}

	forbiddenPatterns := []string{
		"@", // No email addresses
		"http://",
		"https://",
		"$", // No currency
		"£",
		"€",
	}

	for _, tc := range testCases {
		page := engine.BuildPage(tc.inputs)

		// Check all text fields
		allText := page.Title + " " + page.Subtitle
		for _, line := range page.Lines {
			allText += " " + line
		}
		allText += " " + page.PrimaryAction.Label
		if page.SecondaryAction != nil {
			allText += " " + page.SecondaryAction.Label
		}

		for _, pattern := range forbiddenPatterns {
			if strings.Contains(allText, pattern) {
				t.Errorf("Test %s: Found forbidden pattern '%s' in page text: %s",
					tc.name, pattern, allText)
			}
		}
	}
}

// TestJourneyPageFields verifies required page fields are set.
func TestJourneyPageFields(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID: "circle-1",
		HasGmail: false,
		Now:      testTime,
	}

	page := engine.BuildPage(inputs)

	// Check required fields
	if page.Title == "" {
		t.Error("Expected non-empty Title")
	}

	if page.PrimaryAction.Label == "" {
		t.Error("Expected non-empty PrimaryAction.Label")
	}

	if page.PrimaryAction.Path == "" {
		t.Error("Expected non-empty PrimaryAction.Path")
	}

	if page.StatusHash == "" {
		t.Error("Expected non-empty StatusHash")
	}

	if page.CurrentStep == "" {
		t.Error("Expected non-empty CurrentStep")
	}
}

// TestPeriodKey verifies period key format.
func TestPeriodKey(t *testing.T) {
	inputs := &journey.JourneyInputs{
		CircleID: "circle-1",
		Now:      testTime,
	}

	periodKey := inputs.PeriodKey()
	expected := "2024-06-15"

	if periodKey != expected {
		t.Errorf("Expected period key '%s', got '%s'", expected, periodKey)
	}
}

// TestStepIndex verifies step index ordering.
func TestStepIndex(t *testing.T) {
	testCases := []struct {
		step     journey.StepKind
		expected int
	}{
		{journey.StepConnect, 1},
		{journey.StepSync, 2},
		{journey.StepMirror, 3},
		{journey.StepToday, 4},
		{journey.StepAction, 5},
		{journey.StepDone, 0},
	}

	for _, tc := range testCases {
		index := journey.StepIndex(tc.step)
		if index != tc.expected {
			t.Errorf("StepIndex(%s): expected %d, got %d", tc.step, tc.expected, index)
		}
	}
}

// TestSingleWhisperRule verifies journey cue respects single whisper rule.
func TestSingleWhisperRule(t *testing.T) {
	engine := journey.NewEngine(testClock)

	inputs := &journey.JourneyInputs{
		CircleID: "circle-1",
		HasGmail: false,
		Now:      testTime,
	}

	// When no other cue is active, journey cue should show
	if !engine.ShouldShowJourneyCue(inputs, false) {
		t.Error("Expected journey cue to show when no other cue active")
	}

	// When another cue is active, journey cue should not show
	if engine.ShouldShowJourneyCue(inputs, true) {
		t.Error("Expected journey cue to NOT show when other cue active (single whisper rule)")
	}
}
