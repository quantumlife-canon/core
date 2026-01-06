// Package demo_phase26B_first_minutes provides demo tests for Phase 26B: First Five Minutes Proof.
//
// This is NOT analytics. This is NOT telemetry. This is narrative proof.
//
// CRITICAL INVARIANTS:
//   - Determinism: same inputs always produce same hash
//   - Empty inputs return nil summary (silence is success)
//   - Single summary per period
//   - Dismissal suppresses re-render
//   - No identifiers leak
//   - Whisper priority respected
//
// Reference: docs/ADR/ADR-0056-phase26B-first-five-minutes-proof.md
package demo_phase26B_first_minutes

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/firstminutes"
	"quantumlife/internal/persist"
	domainfirstminutes "quantumlife/pkg/domain/firstminutes"
	"quantumlife/pkg/domain/identity"
)

// fixedClock returns a clock function that always returns the same time.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestDeterminism verifies same inputs always produce the same StatusHash.
func TestDeterminism(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	// Create inputs with some signals
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:       "circle-123",
		Period:         "2024-01-15",
		HasConnection:  true,
		ConnectionMode: "real",
		HasSyncReceipt: true,
		SyncMagnitude:  domainfirstminutes.MagnitudeAFew,
	}

	// Compute summary twice
	summary1 := engine.ComputeSummary(inputs)
	summary2 := engine.ComputeSummary(inputs)

	if summary1 == nil || summary2 == nil {
		t.Fatal("Expected non-nil summaries")
	}

	// Status hash must be identical
	if summary1.StatusHash != summary2.StatusHash {
		t.Errorf("Determinism violated: hash1=%s, hash2=%s", summary1.StatusHash, summary2.StatusHash)
	}

	// Status hash must be 32 hex characters (128 bits)
	if len(summary1.StatusHash) != 32 {
		t.Errorf("StatusHash should be 32 hex chars, got %d", len(summary1.StatusHash))
	}

	// CalmLine must be identical
	if summary1.CalmLine != summary2.CalmLine {
		t.Errorf("CalmLine should be deterministic: line1=%s, line2=%s", summary1.CalmLine, summary2.CalmLine)
	}
}

// TestEmptyInputsReturnsNil verifies that no signals = nil summary (silence is success).
func TestEmptyInputsReturnsNil(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	// Empty inputs - no signals
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID: "circle-123",
		Period:   "2024-01-15",
	}

	summary := engine.ComputeSummary(inputs)

	if summary != nil {
		t.Error("Empty inputs should return nil summary (silence is success)")
	}
}

// TestSingleSummaryPerPeriod verifies store only keeps one summary per period.
func TestSingleSummaryPerPeriod(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewFirstMinutesStore(fixedClock(now))

	circleID := identity.EntityID("circle-123")
	period := domainfirstminutes.FirstMinutesPeriod("2024-01-15")

	// First summary
	summary1 := &domainfirstminutes.FirstMinutesSummary{
		Period:     period,
		CalmLine:   "First calm line",
		StatusHash: "hash1",
	}
	_ = store.PersistSummary(circleID, summary1)

	// Second summary for same period (should overwrite)
	summary2 := &domainfirstminutes.FirstMinutesSummary{
		Period:     period,
		CalmLine:   "Second calm line",
		StatusHash: "hash2",
	}
	_ = store.PersistSummary(circleID, summary2)

	// Should only have 1 summary
	if store.CountSummaries() != 1 {
		t.Errorf("Expected 1 summary, got %d", store.CountSummaries())
	}

	// Should return the second summary
	retrieved := store.GetForPeriod(circleID, period)
	if retrieved == nil {
		t.Fatal("Expected to retrieve summary")
	}
	if retrieved.CalmLine != "Second calm line" {
		t.Errorf("Expected second calm line, got %s", retrieved.CalmLine)
	}
}

// TestDismissalSuppressesRerender verifies dismissed summary returns nil.
func TestDismissalSuppressesRerender(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	// Inputs with signals
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:       "circle-123",
		Period:         "2024-01-15",
		HasConnection:  true,
		ConnectionMode: "real",
	}

	// First computation should return summary
	summary1 := engine.ComputeSummary(inputs)
	if summary1 == nil {
		t.Fatal("Expected non-nil summary before dismissal")
	}

	// Now set dismissed hash to match
	inputs.DismissedSummaryHash = summary1.StatusHash

	// Should return nil now (dismissed)
	summary2 := engine.ComputeSummary(inputs)
	if summary2 != nil {
		t.Error("Expected nil summary after dismissal with matching hash")
	}
}

// TestDismissalMaterialChange verifies material change after dismissal shows summary.
func TestDismissalMaterialChange(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	// Initial inputs
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:       "circle-123",
		Period:         "2024-01-15",
		HasConnection:  true,
		ConnectionMode: "mock",
	}

	// Compute and "dismiss"
	summary1 := engine.ComputeSummary(inputs)
	if summary1 == nil {
		t.Fatal("Expected non-nil summary")
	}
	inputs.DismissedSummaryHash = summary1.StatusHash

	// Material change: add sync receipt
	inputs.HasSyncReceipt = true
	inputs.SyncMagnitude = domainfirstminutes.MagnitudeAFew

	// Should show new summary (material change)
	summary2 := engine.ComputeSummary(inputs)
	if summary2 == nil {
		t.Error("Expected summary after material change despite dismissal")
	}

	// Hashes should be different
	if summary2 != nil && summary2.StatusHash == summary1.StatusHash {
		t.Error("Expected different hash after material change")
	}
}

// TestCalmLineSelection verifies deterministic calm line selection.
func TestCalmLineSelection(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	tests := []struct {
		name     string
		inputs   *domainfirstminutes.FirstMinutesInputs
		contains string // Substring that calm line should contain
	}{
		{
			name: "action_executed",
			inputs: &domainfirstminutes.FirstMinutesInputs{
				CircleID:       "circle-123",
				Period:         "2024-01-15",
				HasConnection:  true,
				ConnectionMode: "real",
				ActionExecuted: true,
			},
			contains: "action",
		},
		{
			name: "action_previewed_only",
			inputs: &domainfirstminutes.FirstMinutesInputs{
				CircleID:        "circle-123",
				Period:          "2024-01-15",
				HasConnection:   true,
				ConnectionMode:  "real",
				ActionPreviewed: true,
			},
			contains: "previewed",
		},
		{
			name: "held",
			inputs: &domainfirstminutes.FirstMinutesInputs{
				CircleID:      "circle-123",
				Period:        "2024-01-15",
				HasConnection: true,
				HasHeldItems:  true,
				HeldMagnitude: domainfirstminutes.MagnitudeAFew,
			},
			contains: "held",
		},
		{
			name: "connected_only",
			inputs: &domainfirstminutes.FirstMinutesInputs{
				CircleID:       "circle-123",
				Period:         "2024-01-15",
				HasConnection:  true,
				ConnectionMode: "real",
			},
			contains: "quiet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := engine.ComputeSummary(tt.inputs)
			if summary == nil {
				t.Fatal("Expected non-nil summary")
			}
			if !strings.Contains(strings.ToLower(summary.CalmLine), strings.ToLower(tt.contains)) {
				t.Errorf("CalmLine %q should contain %q", summary.CalmLine, tt.contains)
			}
		})
	}
}

// TestNoIdentifiersLeak scans all strings for forbidden patterns.
func TestNoIdentifiersLeak(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	// Create inputs with all signals
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:        "circle-123",
		Period:          "2024-01-15",
		HasConnection:   true,
		ConnectionMode:  "real",
		HasSyncReceipt:  true,
		SyncMagnitude:   domainfirstminutes.MagnitudeAFew,
		HasMirror:       true,
		MirrorMagnitude: domainfirstminutes.MagnitudeSeveral,
		HasHeldItems:    true,
		HeldMagnitude:   domainfirstminutes.MagnitudeAFew,
		ActionPreviewed: true,
		ActionExecuted:  true,
	}

	summary := engine.ComputeSummary(inputs)
	if summary == nil {
		t.Fatal("Expected non-nil summary")
	}

	// Check CalmLine for forbidden patterns
	forbiddenPatterns := []string{
		"@",      // Email addresses
		"http",   // URLs
		"$",      // Currency
		"£",      // Currency
		"€",      // Currency
		"123",    // Raw numbers
		"circle", // Circle ID leak
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(summary.CalmLine, pattern) {
			t.Errorf("CalmLine contains forbidden pattern %q: %s", pattern, summary.CalmLine)
		}
	}

	// Check signal kinds for forbidden patterns
	for _, sig := range summary.Signals {
		sigStr := string(sig.Kind) + string(sig.Magnitude)
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(sigStr, pattern) {
				t.Errorf("Signal contains forbidden pattern %q: %s", pattern, sigStr)
			}
		}
	}
}

// TestWhisperPriorityRespected verifies single whisper rule.
func TestWhisperPriorityRespected(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:       "circle-123",
		Period:         "2024-01-15",
		HasConnection:  true,
		ConnectionMode: "real",
	}

	// With no other cue active, should show first-minutes cue
	shouldShow := engine.ShouldShowFirstMinutesCue(inputs, false)
	if !shouldShow {
		t.Error("Should show first-minutes cue when no other cue active")
	}

	// With another cue active, should NOT show first-minutes cue
	shouldShow = engine.ShouldShowFirstMinutesCue(inputs, true)
	if shouldShow {
		t.Error("Should NOT show first-minutes cue when another cue active")
	}
}

// TestSignalKindCompleteness verifies all signal kinds have canonical strings.
func TestSignalKindCompleteness(t *testing.T) {
	allKinds := domainfirstminutes.AllSignalKinds()

	if len(allKinds) != 7 {
		t.Errorf("Expected 7 signal kinds, got %d", len(allKinds))
	}

	expectedKinds := []domainfirstminutes.FirstMinutesSignalKind{
		domainfirstminutes.SignalConnected,
		domainfirstminutes.SignalSynced,
		domainfirstminutes.SignalMirrored,
		domainfirstminutes.SignalHeld,
		domainfirstminutes.SignalActionPreviewed,
		domainfirstminutes.SignalActionExecuted,
		domainfirstminutes.SignalSilencePreserved,
	}

	for i, kind := range expectedKinds {
		if allKinds[i] != kind {
			t.Errorf("Expected kind %s at index %d, got %s", kind, i, allKinds[i])
		}

		// Test canonical string
		signal := domainfirstminutes.FirstMinutesSignal{
			Kind:      kind,
			Magnitude: domainfirstminutes.MagnitudeAFew,
		}
		canonical := signal.CanonicalString()
		if !strings.HasPrefix(canonical, "SIGNAL|v1|") {
			t.Errorf("Canonical string should start with SIGNAL|v1|, got %s", canonical)
		}
	}
}

// TestCanonicalStringFormat verifies pipe-delimited, version-prefixed format.
func TestCanonicalStringFormat(t *testing.T) {
	// Test signal canonical string
	signal := domainfirstminutes.FirstMinutesSignal{
		Kind:      domainfirstminutes.SignalConnected,
		Magnitude: domainfirstminutes.MagnitudeAFew,
	}
	canonical := signal.CanonicalString()
	expected := "SIGNAL|v1|connected|a_few"
	if canonical != expected {
		t.Errorf("Signal canonical string: expected %s, got %s", expected, canonical)
	}

	// Test summary canonical string
	summary := &domainfirstminutes.FirstMinutesSummary{
		Period: "2024-01-15",
		Signals: []domainfirstminutes.FirstMinutesSignal{
			{Kind: domainfirstminutes.SignalConnected, Magnitude: domainfirstminutes.MagnitudeAFew},
		},
		CalmLine: "Test calm line",
	}
	summaryCanonical := summary.CanonicalString()
	if !strings.HasPrefix(summaryCanonical, "FIRST_MINUTES|v1|") {
		t.Errorf("Summary canonical string should start with FIRST_MINUTES|v1|, got %s", summaryCanonical)
	}
	if !strings.Contains(summaryCanonical, "2024-01-15") {
		t.Error("Summary canonical string should contain period")
	}

	// Test dismissal canonical string
	dismissal := &domainfirstminutes.FirstMinutesDismissal{
		Period:      "2024-01-15",
		SummaryHash: "abc123",
	}
	dismissalCanonical := dismissal.CanonicalString()
	if !strings.HasPrefix(dismissalCanonical, "FIRST_MINUTES_DISMISS|v1|") {
		t.Errorf("Dismissal canonical string should start with FIRST_MINUTES_DISMISS|v1|, got %s", dismissalCanonical)
	}
}

// TestStatusHashLength verifies hash is 32 hex chars (128 bits).
func TestStatusHashLength(t *testing.T) {
	summary := &domainfirstminutes.FirstMinutesSummary{
		Period: "2024-01-15",
		Signals: []domainfirstminutes.FirstMinutesSignal{
			{Kind: domainfirstminutes.SignalConnected, Magnitude: domainfirstminutes.MagnitudeAFew},
		},
		CalmLine: "Test",
	}

	hash := summary.ComputeStatusHash()

	if len(hash) != 32 {
		t.Errorf("StatusHash should be 32 hex chars (128 bits), got %d chars", len(hash))
	}

	// Verify it's valid hex
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("StatusHash should be lowercase hex, got char %c", c)
		}
	}
}

// TestBoundedRetention verifies store evicts after 30 periods.
func TestBoundedRetention(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	store := persist.NewFirstMinutesStore(fixedClock(baseTime))

	circleID := identity.EntityID("circle-123")

	// Add 35 summaries for different periods
	for i := 0; i < 35; i++ {
		period := domainfirstminutes.FirstMinutesPeriod(baseTime.AddDate(0, 0, i).Format("2006-01-02"))
		summary := &domainfirstminutes.FirstMinutesSummary{
			Period:     period,
			CalmLine:   "Test",
			StatusHash: "hash",
		}
		_ = store.PersistSummary(circleID, summary)
	}

	// Should have at most 30 summaries (bounded retention)
	count := store.CountSummaries()
	if count > 30 {
		t.Errorf("Store should have at most 30 summaries (bounded retention), got %d", count)
	}
}

// TestInputsHash verifies inputs produce deterministic hash.
func TestInputsHash(t *testing.T) {
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:       "circle-123",
		Period:         "2024-01-15",
		HasConnection:  true,
		ConnectionMode: "real",
		HasSyncReceipt: true,
		SyncMagnitude:  domainfirstminutes.MagnitudeAFew,
	}

	hash1 := inputs.ComputeInputsHash()
	hash2 := inputs.ComputeInputsHash()

	if hash1 != hash2 {
		t.Errorf("Inputs hash should be deterministic: %s != %s", hash1, hash2)
	}

	if len(hash1) != 32 {
		t.Errorf("Inputs hash should be 32 hex chars, got %d", len(hash1))
	}
}

// TestHasMeaningfulSignals verifies the helper function.
func TestHasMeaningfulSignals(t *testing.T) {
	// Empty inputs
	empty := &domainfirstminutes.FirstMinutesInputs{}
	if empty.HasMeaningfulSignals() {
		t.Error("Empty inputs should not have meaningful signals")
	}

	// With connection
	withConn := &domainfirstminutes.FirstMinutesInputs{HasConnection: true}
	if !withConn.HasMeaningfulSignals() {
		t.Error("Inputs with connection should have meaningful signals")
	}

	// With sync
	withSync := &domainfirstminutes.FirstMinutesInputs{HasSyncReceipt: true}
	if !withSync.HasMeaningfulSignals() {
		t.Error("Inputs with sync should have meaningful signals")
	}

	// With action
	withAction := &domainfirstminutes.FirstMinutesInputs{ActionExecuted: true}
	if !withAction.HasMeaningfulSignals() {
		t.Error("Inputs with action should have meaningful signals")
	}
}

// TestDefaultCueText verifies the cue text constant.
func TestDefaultCueText(t *testing.T) {
	expected := "If you ever wondered how the beginning went."
	if domainfirstminutes.DefaultCueText != expected {
		t.Errorf("DefaultCueText: expected %q, got %q", expected, domainfirstminutes.DefaultCueText)
	}
}

// TestComputeCue verifies cue computation.
func TestComputeCue(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	engine := firstminutes.NewEngine(fixedClock(now))

	// With signals - should have cue
	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID:       "circle-123",
		Period:         "2024-01-15",
		HasConnection:  true,
		ConnectionMode: "real",
	}

	cue := engine.ComputeCue(inputs)
	if !cue.Available {
		t.Error("Cue should be available when there are signals")
	}
	if cue.CueText != domainfirstminutes.DefaultCueText {
		t.Errorf("Cue text should be default, got %s", cue.CueText)
	}

	// Without signals - should not have cue
	emptyInputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID: "circle-123",
		Period:   "2024-01-15",
	}

	cue = engine.ComputeCue(emptyInputs)
	if cue.Available {
		t.Error("Cue should not be available when there are no signals")
	}
}

// TestStoreOperations verifies basic store operations.
func TestStoreOperations(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewFirstMinutesStore(fixedClock(now))

	circleID := identity.EntityID("circle-123")
	period := domainfirstminutes.FirstMinutesPeriod("2024-01-15")

	// Initially no summary
	if store.HasSummaryForPeriod(circleID, period) {
		t.Error("Should not have summary initially")
	}

	// Persist summary
	summary := &domainfirstminutes.FirstMinutesSummary{
		Period:     period,
		CalmLine:   "Test",
		StatusHash: "hash123",
	}
	_ = store.PersistSummary(circleID, summary)

	// Now should have summary
	if !store.HasSummaryForPeriod(circleID, period) {
		t.Error("Should have summary after persist")
	}

	// Initially not dismissed
	if store.IsDismissed(circleID, period) {
		t.Error("Should not be dismissed initially")
	}

	// Record dismissal
	_, _ = store.RecordDismissal(circleID, period, "hash123")

	// Now should be dismissed
	if !store.IsDismissed(circleID, period) {
		t.Error("Should be dismissed after recording")
	}

	// Get dismissed hash
	dismissedHash := store.GetDismissedSummaryHash(circleID, period)
	if dismissedHash != "hash123" {
		t.Errorf("Dismissed hash should be hash123, got %s", dismissedHash)
	}
}
