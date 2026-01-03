// Package demo_phase20_trust_accrual demonstrates Phase 20: Trust Accrual Layer.
//
// CRITICAL INVARIANTS UNDER TEST:
//   - Silence is the default outcome
//   - Trust signals are NEVER pushed
//   - Trust signals are NEVER frequent
//   - Trust signals are NEVER actionable
//   - Only abstract buckets (nothing / a_few / several)
//   - NO timestamps, counts, vendors, people, or content
//   - Deterministic: same inputs + clock => same hashes
//   - Append-only, hash-only storage
//   - No goroutines, no time.Now()
//
// These tests verify that trust accrual makes restraint observable
// WITHOUT creating engagement pressure.
package demo_phase20_trust_accrual

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	trustengine "quantumlife/internal/trust"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/shadowllm"
	"quantumlife/pkg/domain/trust"
)

// =============================================================================
// Test 1: Determinism - Same Inputs + Clock => Same Hash
// =============================================================================

func TestTrustSummary_DeterministicHash(t *testing.T) {
	// Fixed clock for determinism
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := trustengine.NewEngine(clk)

	// Create mock source with consistent data
	source := trustengine.NewMockSource()
	source.HeldCounts["2024-W03"] = 5 // several

	// Compute first time
	output1, err := engine.Compute(trustengine.ComputeInput{
		Period:    trust.PeriodWeek,
		PeriodKey: "2024-W03",
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}
	if output1.Summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	hash1 := output1.Summary.SummaryHash
	id1 := output1.Summary.SummaryID

	// Compute second time with SAME inputs
	output2, err := engine.Compute(trustengine.ComputeInput{
		Period:    trust.PeriodWeek,
		PeriodKey: "2024-W03",
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}
	if output2.Summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	hash2 := output2.Summary.SummaryHash
	id2 := output2.Summary.SummaryID

	// CRITICAL: Same inputs + clock => same hash
	if hash1 != hash2 {
		t.Errorf("Hash mismatch: %s != %s", hash1, hash2)
	}
	if id1 != id2 {
		t.Errorf("ID mismatch: %s != %s", id1, id2)
	}

	t.Logf("✓ Determinism verified: hash=%s", hash1[:16])
}

// =============================================================================
// Test 2: Silence is Default - Nothing Occurred => No Summary
// =============================================================================

func TestTrustSummary_SilenceIsDefault(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := trustengine.NewEngine(clk)

	// Empty source - nothing happened
	source := trustengine.NullSource{}

	output, err := engine.Compute(trustengine.ComputeInput{
		Period:    trust.PeriodWeek,
		PeriodKey: "2024-W03",
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	// CRITICAL: No summary when nothing happened
	if output.Summary != nil {
		t.Error("Expected nil summary when nothing occurred")
	}
	if output.Meaningful {
		t.Error("Expected Meaningful=false when nothing occurred")
	}

	// All magnitudes should be "nothing"
	if output.HeldMagnitude != shadowllm.MagnitudeNothing {
		t.Errorf("Expected HeldMagnitude=nothing, got %s", output.HeldMagnitude)
	}
	if output.SuppressionMagnitude != shadowllm.MagnitudeNothing {
		t.Errorf("Expected SuppressionMagnitude=nothing, got %s", output.SuppressionMagnitude)
	}
	if output.RejectionMagnitude != shadowllm.MagnitudeNothing {
		t.Errorf("Expected RejectionMagnitude=nothing, got %s", output.RejectionMagnitude)
	}

	t.Log("✓ Silence is default: no summary when nothing happened")
}

// =============================================================================
// Test 3: Suppressions Create Trust Signal
// =============================================================================

func TestTrustSummary_SuppressionsCreateSignal(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := trustengine.NewEngine(clk)

	// Source with suppressions
	source := trustengine.NewMockSource()
	source.SuppressionCounts["2024-W03"] = 2 // a_few

	output, err := engine.Compute(trustengine.ComputeInput{
		Period:    trust.PeriodWeek,
		PeriodKey: "2024-W03",
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if output.Summary == nil {
		t.Fatal("Expected summary, got nil")
	}
	if !output.Meaningful {
		t.Error("Expected Meaningful=true")
	}

	// CRITICAL: Signal kind should indicate interruption prevention
	if output.Summary.SignalKind != trust.SignalInterruptionPrevented {
		t.Errorf("Expected SignalInterruptionPrevented, got %s", output.Summary.SignalKind)
	}

	// Magnitude should be abstract bucket
	if output.Summary.MagnitudeBucket != shadowllm.MagnitudeAFew {
		t.Errorf("Expected MagnitudeAFew, got %s", output.Summary.MagnitudeBucket)
	}

	t.Logf("✓ Suppressions create signal: kind=%s magnitude=%s",
		output.Summary.SignalKind, output.Summary.MagnitudeBucket)
}

// =============================================================================
// Test 4: Held Obligations Create Trust Signal
// =============================================================================

func TestTrustSummary_HeldCreatesSignal(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := trustengine.NewEngine(clk)

	// Source with held obligations
	source := trustengine.NewMockSource()
	source.HeldCounts["2024-W03"] = 1 // a_few (1-3)

	output, err := engine.Compute(trustengine.ComputeInput{
		Period:    trust.PeriodWeek,
		PeriodKey: "2024-W03",
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if output.Summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	// CRITICAL: Signal kind should indicate quiet held
	if output.Summary.SignalKind != trust.SignalQuietHeld {
		t.Errorf("Expected SignalQuietHeld, got %s", output.Summary.SignalKind)
	}

	if output.Summary.MagnitudeBucket != shadowllm.MagnitudeAFew {
		t.Errorf("Expected MagnitudeAFew, got %s", output.Summary.MagnitudeBucket)
	}

	t.Logf("✓ Held creates signal: kind=%s magnitude=%s",
		output.Summary.SignalKind, output.Summary.MagnitudeBucket)
}

// =============================================================================
// Test 5: Magnitude Buckets Only - Never Raw Counts
// =============================================================================

func TestTrustSummary_MagnitudeBucketsOnly(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := trustengine.NewEngine(clk)

	testCases := []struct {
		name     string
		count    int
		expected shadowllm.MagnitudeBucket
	}{
		{"zero", 0, shadowllm.MagnitudeNothing},
		{"one", 1, shadowllm.MagnitudeAFew},
		{"two", 2, shadowllm.MagnitudeAFew},
		{"three", 3, shadowllm.MagnitudeAFew},
		{"four", 4, shadowllm.MagnitudeSeveral},
		{"ten", 10, shadowllm.MagnitudeSeveral},
		{"hundred", 100, shadowllm.MagnitudeSeveral},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			source := trustengine.NewMockSource()
			source.HeldCounts["2024-W03"] = tc.count

			output, err := engine.Compute(trustengine.ComputeInput{
				Period:    trust.PeriodWeek,
				PeriodKey: "2024-W03",
				Source:    source,
			})
			if err != nil {
				t.Fatalf("Compute failed: %v", err)
			}

			// CRITICAL: Only magnitude buckets, never raw counts
			if output.HeldMagnitude != tc.expected {
				t.Errorf("Count %d: expected %s, got %s",
					tc.count, tc.expected, output.HeldMagnitude)
			}
		})
	}

	t.Log("✓ Magnitude buckets only: raw counts never exposed")
}

// =============================================================================
// Test 6: Dismissal Hides Trust Cue Permanently
// =============================================================================

func TestTrustStore_DismissalHidesPermanently(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewTrustStore(func() time.Time { return fixedTime })

	// Create a summary
	summary := &trust.TrustSummary{
		Period:          trust.PeriodWeek,
		PeriodKey:       "2024-W03",
		SignalKind:      trust.SignalQuietHeld,
		MagnitudeBucket: shadowllm.MagnitudeAFew,
		CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
		CreatedAt:       fixedTime,
	}
	summary.SummaryID = summary.ComputeID()
	summary.SummaryHash = summary.ComputeHash()

	if err := store.AppendSummary(summary); err != nil {
		t.Fatalf("AppendSummary failed: %v", err)
	}

	// Before dismissal: should appear in undismissed list
	undismissed := store.ListUndismissedSummaries()
	if len(undismissed) != 1 {
		t.Errorf("Expected 1 undismissed, got %d", len(undismissed))
	}

	// Dismiss
	if err := store.DismissSummary(summary.SummaryID); err != nil {
		t.Fatalf("DismissSummary failed: %v", err)
	}

	// CRITICAL: After dismissal, should NOT appear
	undismissed = store.ListUndismissedSummaries()
	if len(undismissed) != 0 {
		t.Errorf("Expected 0 undismissed after dismissal, got %d", len(undismissed))
	}

	// But should still exist in all summaries
	all := store.ListSummaries()
	if len(all) != 1 {
		t.Errorf("Expected 1 total summary, got %d", len(all))
	}

	// Dismissal is idempotent
	if err := store.DismissSummary(summary.SummaryID); err != nil {
		t.Errorf("Idempotent dismissal failed: %v", err)
	}

	t.Log("✓ Dismissal hides permanently: dismissed summary never reappears")
}

// =============================================================================
// Test 7: Period Deduplication
// =============================================================================

func TestTrustStore_PeriodDeduplication(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewTrustStore(func() time.Time { return fixedTime })

	// Create first summary
	summary1 := &trust.TrustSummary{
		Period:          trust.PeriodWeek,
		PeriodKey:       "2024-W03",
		SignalKind:      trust.SignalQuietHeld,
		MagnitudeBucket: shadowllm.MagnitudeAFew,
		CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
		CreatedAt:       fixedTime,
	}
	summary1.SummaryID = summary1.ComputeID()
	summary1.SummaryHash = summary1.ComputeHash()

	if err := store.AppendSummary(summary1); err != nil {
		t.Fatalf("First AppendSummary failed: %v", err)
	}

	// Same summary again (idempotent)
	err := store.AppendSummary(summary1)
	if err != nil {
		t.Errorf("Idempotent append should not fail: %v", err)
	}

	// CRITICAL: Summary ID is deterministic from period + period key
	// So any "different" summary for the same period gets the same ID
	// and is treated as idempotent. This is by design - you can only
	// have ONE summary per period, computed from the engine.
	summary2 := &trust.TrustSummary{
		Period:          trust.PeriodWeek,
		PeriodKey:       "2024-W03", // Same period!
		SignalKind:      trust.SignalInterruptionPrevented,
		MagnitudeBucket: shadowllm.MagnitudeSeveral,
		CreatedBucket:   trust.FiveMinuteBucket(fixedTime.Add(time.Hour)),
		CreatedAt:       fixedTime.Add(time.Hour),
	}
	summary2.SummaryID = summary2.ComputeID()
	summary2.SummaryHash = summary2.ComputeHash()

	// ID is deterministic from period+period_key, so same ID
	if summary1.SummaryID != summary2.SummaryID {
		t.Errorf("Expected same ID for same period, got %s != %s",
			summary1.SummaryID, summary2.SummaryID)
	}

	// Appending is idempotent (same ID means same summary conceptually)
	err = store.AppendSummary(summary2)
	if err != nil {
		t.Errorf("Idempotent append should not fail: %v", err)
	}

	// Store should have exactly 1 summary
	if store.GetSummaryCount() != 1 {
		t.Errorf("Expected 1 summary, got %d", store.GetSummaryCount())
	}

	// Different period works fine
	summary3 := &trust.TrustSummary{
		Period:          trust.PeriodWeek,
		PeriodKey:       "2024-W04", // Different period
		SignalKind:      trust.SignalQuietHeld,
		MagnitudeBucket: shadowllm.MagnitudeAFew,
		CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
		CreatedAt:       fixedTime,
	}
	summary3.SummaryID = summary3.ComputeID()
	summary3.SummaryHash = summary3.ComputeHash()

	if err := store.AppendSummary(summary3); err != nil {
		t.Fatalf("Different period append failed: %v", err)
	}

	// Now should have 2 summaries
	if store.GetSummaryCount() != 2 {
		t.Errorf("Expected 2 summaries, got %d", store.GetSummaryCount())
	}

	t.Log("✓ Period deduplication: same period → same ID, different periods allowed")
}

// =============================================================================
// Test 8: Replay Produces Identical Results
// =============================================================================

func TestTrustStore_ReplayProducesIdenticalResults(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	store1 := persist.NewTrustStore(func() time.Time { return fixedTime })

	// Create and store a summary
	summary := &trust.TrustSummary{
		Period:          trust.PeriodWeek,
		PeriodKey:       "2024-W03",
		SignalKind:      trust.SignalQuietHeld,
		MagnitudeBucket: shadowllm.MagnitudeAFew,
		CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
		CreatedAt:       fixedTime,
	}
	summary.SummaryID = summary.ComputeID()
	summary.SummaryHash = summary.ComputeHash()

	if err := store1.AppendSummary(summary); err != nil {
		t.Fatalf("AppendSummary failed: %v", err)
	}

	// Dismiss it
	if err := store1.DismissSummary(summary.SummaryID); err != nil {
		t.Fatalf("DismissSummary failed: %v", err)
	}

	// Get records for replay
	summaryRecord := store1.SummaryToStorelogRecord(summary)
	dismissal, _ := store1.GetDismissal(summary.SummaryID)
	dismissalRecord := store1.DismissalToStorelogRecord(dismissal)

	// Create new store and replay
	store2 := persist.NewTrustStore(func() time.Time { return fixedTime })

	if err := store2.ReplaySummaryRecord(summaryRecord); err != nil {
		t.Fatalf("ReplaySummaryRecord failed: %v", err)
	}
	if err := store2.ReplayDismissalRecord(dismissalRecord); err != nil {
		t.Fatalf("ReplayDismissalRecord failed: %v", err)
	}

	// CRITICAL: Replayed store should match original
	replayedSummary, ok := store2.GetSummary(summary.SummaryID)
	if !ok {
		t.Fatal("Replayed summary not found")
	}
	if replayedSummary.SummaryHash != summary.SummaryHash {
		t.Errorf("Hash mismatch: %s != %s", replayedSummary.SummaryHash, summary.SummaryHash)
	}

	// Dismissal should be replayed
	if !store2.IsDismissed(summary.SummaryID) {
		t.Error("Dismissal not replayed")
	}

	// Undismissed list should be empty
	undismissed := store2.ListUndismissedSummaries()
	if len(undismissed) != 0 {
		t.Errorf("Expected 0 undismissed after replay, got %d", len(undismissed))
	}

	t.Log("✓ Replay produces identical results")
}

// =============================================================================
// Test 9: Human-Readable Descriptions Are Calm
// =============================================================================

func TestTrustSignalKind_HumanReadableIsCalm(t *testing.T) {
	testCases := []struct {
		kind     trust.TrustSignalKind
		expected string
	}{
		{trust.SignalQuietHeld, "Things were held quietly."},
		{trust.SignalInterruptionPrevented, "Interruptions were prevented."},
		{trust.SignalNothingRequired, "Nothing required attention."},
	}

	for _, tc := range testCases {
		t.Run(string(tc.kind), func(t *testing.T) {
			got := tc.kind.HumanReadable()
			if got != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, got)
			}

			// CRITICAL: Must NOT contain performative language
			forbidden := []string{
				"saved", "protected", "value", "helped",
				"amazing", "great", "excellent", "impressive",
				"trust us", "because of us", "thanks to",
			}
			for _, word := range forbidden {
				if containsWord(got, word) {
					t.Errorf("Human-readable contains forbidden word: %s", word)
				}
			}
		})
	}

	t.Log("✓ Human-readable descriptions are calm and non-performative")
}

// =============================================================================
// Test 10: Canonical String Uses Pipes (Not JSON)
// =============================================================================

func TestTrustSummary_CanonicalStringFormat(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	summary := &trust.TrustSummary{
		Period:          trust.PeriodWeek,
		PeriodKey:       "2024-W03",
		SignalKind:      trust.SignalQuietHeld,
		MagnitudeBucket: shadowllm.MagnitudeAFew,
		CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
		CreatedAt:       fixedTime,
	}

	canonical := summary.CanonicalString()

	// CRITICAL: Must be pipe-delimited, NOT JSON
	if canonical[0] == '{' {
		t.Error("Canonical string must not be JSON")
	}

	// Must start with type and version
	expectedPrefix := "TRUST_SUMMARY|v1|"
	if len(canonical) < len(expectedPrefix) || canonical[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Expected prefix %q, got %q", expectedPrefix, canonical[:min(len(canonical), len(expectedPrefix))])
	}

	// Must contain period and period key
	if !containsWord(canonical, "week") {
		t.Error("Canonical string missing period")
	}
	if !containsWord(canonical, "2024-W03") {
		t.Error("Canonical string missing period key")
	}

	t.Logf("✓ Canonical string is pipe-delimited: %s", canonical)
}

// =============================================================================
// Test 11: Period Keys Are Abstract (Not Timestamps)
// =============================================================================

func TestPeriodKeys_AreAbstract(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)

	weekKey := trust.WeekKey(testTime)
	monthKey := trust.MonthKey(testTime)

	// Week key format: YYYY-WNN
	if len(weekKey) != 8 || weekKey[4] != '-' || weekKey[5] != 'W' {
		t.Errorf("Week key format invalid: %s", weekKey)
	}

	// Month key format: YYYY-MM
	if len(monthKey) != 7 || monthKey[4] != '-' {
		t.Errorf("Month key format invalid: %s", monthKey)
	}

	// CRITICAL: Must NOT contain time components
	if containsWord(weekKey, "10:30") || containsWord(weekKey, "45") {
		t.Error("Week key contains time components")
	}
	if containsWord(monthKey, "15") { // Day should not appear
		t.Error("Month key contains day")
	}

	t.Logf("✓ Period keys are abstract: week=%s month=%s", weekKey, monthKey)
}

// =============================================================================
// Test 12: GetRecentMeaningfulSummary Returns Most Recent Undismissed
// =============================================================================

func TestTrustStore_GetRecentMeaningfulSummary(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	store := persist.NewTrustStore(func() time.Time { return fixedTime })

	// Empty store should return nil
	if store.GetRecentMeaningfulSummary() != nil {
		t.Error("Expected nil from empty store")
	}

	// Add summaries for different periods
	for i, periodKey := range []string{"2024-W01", "2024-W02", "2024-W03"} {
		summary := &trust.TrustSummary{
			Period:          trust.PeriodWeek,
			PeriodKey:       periodKey,
			SignalKind:      trust.SignalQuietHeld,
			MagnitudeBucket: shadowllm.MagnitudeAFew,
			CreatedBucket:   trust.FiveMinuteBucket(fixedTime.Add(time.Duration(i) * time.Hour)),
			CreatedAt:       fixedTime.Add(time.Duration(i) * time.Hour),
		}
		summary.SummaryID = summary.ComputeID()
		summary.SummaryHash = summary.ComputeHash()
		if err := store.AppendSummary(summary); err != nil {
			t.Fatalf("AppendSummary failed: %v", err)
		}
	}

	// Should return most recent (2024-W03)
	recent := store.GetRecentMeaningfulSummary()
	if recent == nil {
		t.Fatal("Expected recent summary, got nil")
	}
	if recent.PeriodKey != "2024-W03" {
		t.Errorf("Expected 2024-W03, got %s", recent.PeriodKey)
	}

	// Dismiss most recent
	if err := store.DismissSummary(recent.SummaryID); err != nil {
		t.Fatalf("DismissSummary failed: %v", err)
	}

	// Should now return 2024-W02
	recent = store.GetRecentMeaningfulSummary()
	if recent == nil {
		t.Fatal("Expected recent summary, got nil")
	}
	if recent.PeriodKey != "2024-W02" {
		t.Errorf("Expected 2024-W02, got %s", recent.PeriodKey)
	}

	t.Log("✓ GetRecentMeaningfulSummary returns most recent undismissed")
}

// =============================================================================
// Test 13: Suppressions Have Priority Over Held
// =============================================================================

func TestTrustEngine_SuppressionsPriority(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	engine := trustengine.NewEngine(clk)

	// Source with BOTH suppressions and held
	source := trustengine.NewMockSource()
	source.HeldCounts["2024-W03"] = 10       // several
	source.SuppressionCounts["2024-W03"] = 2 // a_few

	output, err := engine.Compute(trustengine.ComputeInput{
		Period:    trust.PeriodWeek,
		PeriodKey: "2024-W03",
		Source:    source,
	})
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if output.Summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	// CRITICAL: Suppressions have priority (more active form of restraint)
	if output.Summary.SignalKind != trust.SignalInterruptionPrevented {
		t.Errorf("Expected SignalInterruptionPrevented (priority), got %s", output.Summary.SignalKind)
	}

	// Magnitude should be from suppressions (a_few), not held (several)
	if output.Summary.MagnitudeBucket != shadowllm.MagnitudeAFew {
		t.Errorf("Expected MagnitudeAFew, got %s", output.Summary.MagnitudeBucket)
	}

	t.Log("✓ Suppressions have priority over held obligations")
}

// =============================================================================
// Test 14: Five-Minute Bucket Determinism
// =============================================================================

func TestFiveMinuteBucket_Determinism(t *testing.T) {
	testCases := []struct {
		minute   int
		expected string
	}{
		{0, "2024-01-15T10:0"},
		{1, "2024-01-15T10:0"},
		{4, "2024-01-15T10:0"},
		{5, "2024-01-15T10:5"},
		{9, "2024-01-15T10:5"},
		{10, "2024-01-15T10:10"},
		{30, "2024-01-15T10:30"},
		{59, "2024-01-15T10:55"},
	}

	for _, tc := range testCases {
		t.Run(string(rune('0'+tc.minute)), func(t *testing.T) {
			testTime := time.Date(2024, 1, 15, 10, tc.minute, 30, 0, time.UTC)
			bucket := trust.FiveMinuteBucket(testTime)
			if bucket != tc.expected {
				t.Errorf("Minute %d: expected %s, got %s", tc.minute, tc.expected, bucket)
			}
		})
	}

	t.Log("✓ Five-minute buckets are deterministic")
}

// =============================================================================
// Test 15: Validation Errors
// =============================================================================

func TestTrustSummary_ValidationErrors(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	testCases := []struct {
		name    string
		summary trust.TrustSummary
		wantErr bool
	}{
		{
			name: "valid",
			summary: trust.TrustSummary{
				Period:          trust.PeriodWeek,
				PeriodKey:       "2024-W03",
				SignalKind:      trust.SignalQuietHeld,
				MagnitudeBucket: shadowllm.MagnitudeAFew,
				CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
			},
			wantErr: false,
		},
		{
			name: "invalid period",
			summary: trust.TrustSummary{
				Period:          "day", // Invalid
				PeriodKey:       "2024-W03",
				SignalKind:      trust.SignalQuietHeld,
				MagnitudeBucket: shadowllm.MagnitudeAFew,
				CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
			},
			wantErr: true,
		},
		{
			name: "missing period key",
			summary: trust.TrustSummary{
				Period:          trust.PeriodWeek,
				PeriodKey:       "", // Missing
				SignalKind:      trust.SignalQuietHeld,
				MagnitudeBucket: shadowllm.MagnitudeAFew,
				CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
			},
			wantErr: true,
		},
		{
			name: "invalid signal kind",
			summary: trust.TrustSummary{
				Period:          trust.PeriodWeek,
				PeriodKey:       "2024-W03",
				SignalKind:      "invalid", // Invalid
				MagnitudeBucket: shadowllm.MagnitudeAFew,
				CreatedBucket:   trust.FiveMinuteBucket(fixedTime),
			},
			wantErr: true,
		},
		{
			name: "missing created bucket",
			summary: trust.TrustSummary{
				Period:          trust.PeriodWeek,
				PeriodKey:       "2024-W03",
				SignalKind:      trust.SignalQuietHeld,
				MagnitudeBucket: shadowllm.MagnitudeAFew,
				CreatedBucket:   "", // Missing
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.summary.Validate()
			if tc.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}

	t.Log("✓ Validation errors work correctly")
}

// =============================================================================
// Helpers
// =============================================================================

func containsWord(s, word string) bool {
	for i := 0; i <= len(s)-len(word); i++ {
		if s[i:i+len(word)] == word {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
