// Package demo_phase40_timewindow_sources provides demonstration tests for Phase 40.
//
// These tests verify the Time-Window Pressure Sources implementation against
// all Phase 40 requirements.
//
// CRITICAL: This is OBSERVATION ONLY. No delivery, no execution, no notifications.
//
// Reference: docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md
package demo_phase40_timewindow_sources

import (
	"testing"
	"time"

	ae "quantumlife/pkg/domain/attentionenvelope"
	pd "quantumlife/pkg/domain/pressuredecision"
	tw "quantumlife/pkg/domain/timewindow"

	internaltw "quantumlife/internal/timewindow"
)

// Test 1: Deterministic BuildSignals
func TestDeterministicBuildSignals(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowSoon,
			EvidenceHashes:      []string{"hash1", "hash2"},
		},
	}

	result1 := engine.BuildSignals(inputs, clock)
	result2 := engine.BuildSignals(inputs, clock)

	if result1.ResultHash != result2.ResultHash {
		t.Errorf("BuildSignals not deterministic: %s != %s", result1.ResultHash, result2.ResultHash)
	}
}

// Test 2: Empty inputs return empty status
func TestEmptyInputsReturnEmpty(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         false,
			UpcomingCountBucket: tw.MagnitudeNothing,
		},
		Inbox: tw.InboxWindowInputs{
			InstitutionalCountBucket: tw.MagnitudeNothing,
			HumanCountBucket:         tw.MagnitudeNothing,
		},
		DeviceHints: tw.DeviceHintInputs{
			TransportSignals:   tw.MagnitudeNothing,
			HealthSignals:      tw.MagnitudeNothing,
			InstitutionSignals: tw.MagnitudeNothing,
		},
	}

	result := engine.BuildSignals(inputs, clock)

	if result.Status != tw.StatusEmpty {
		t.Errorf("expected status empty, got %s", result.Status)
	}
	if len(result.Signals) != 0 {
		t.Errorf("expected 0 signals, got %d", len(result.Signals))
	}
}

// Test 3: Calendar precedence over inbox
func TestCalendarPrecedenceOverInbox(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowSoon,
			EvidenceHashes:      []string{"cal_hash"},
		},
		Inbox: tw.InboxWindowInputs{
			HumanCountBucket: tw.MagnitudeAFew,
			HumanWindowKind:  tw.WindowNow,
			EvidenceHashes:   []string{"inbox_hash"},
		},
	}

	result := engine.BuildSignals(inputs, clock)

	if result.Status != tw.StatusOK {
		t.Errorf("expected status ok, got %s", result.Status)
	}
	if len(result.Signals) < 1 {
		t.Fatalf("expected at least 1 signal")
	}

	// First signal should be from calendar (highest precedence)
	if result.Signals[0].Source != tw.SourceCalendar {
		t.Errorf("expected first signal from calendar, got %s", result.Signals[0].Source)
	}
}

// Test 4: Human vs institution separation
func TestHumanVsInstitutionSeparation(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Inbox: tw.InboxWindowInputs{
			InstitutionalCountBucket: tw.MagnitudeAFew,
			HumanCountBucket:         tw.MagnitudeAFew,
			InstitutionWindowKind:    tw.WindowSoon,
			HumanWindowKind:          tw.WindowNow,
		},
	}

	result := engine.BuildSignals(inputs, clock)

	// Should have separate signals for institution and human
	hasInstitution := false
	hasHuman := false
	for _, s := range result.Signals {
		if s.Source == tw.SourceInboxInstitution {
			hasInstitution = true
		}
		if s.Source == tw.SourceInboxHuman {
			hasHuman = true
		}
	}

	if !hasInstitution || !hasHuman {
		t.Errorf("expected both institution and human signals, got institution=%v human=%v", hasInstitution, hasHuman)
	}
}

// Test 5: Evidence cap enforcement (max 3)
func TestEvidenceCapEnforcement(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowSoon,
			EvidenceHashes:      []string{"h1", "h2", "h3", "h4", "h5"}, // 5 hashes
		},
	}

	result := engine.BuildSignals(inputs, clock)

	if len(result.Signals) < 1 {
		t.Fatalf("expected at least 1 signal")
	}

	// Each signal should have at most 3 evidence hashes
	for _, s := range result.Signals {
		if len(s.EvidenceHashes) > tw.MaxEvidenceHashes {
			t.Errorf("evidence hashes exceeded max: %d > %d", len(s.EvidenceHashes), tw.MaxEvidenceHashes)
		}
	}
}

// Test 6: Envelope bounded effect (max 1 step shift)
func TestEnvelopeBoundedShift(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowLater, // Start at Later
			EvidenceHashes:      []string{"hash1"},
		},
		EnvelopeSummary: tw.AttentionEnvelopeSummary{
			IsActive: true,
			Kind:     ae.EnvelopeKindOnCall, // Should shift by 1
		},
	}

	result := engine.BuildSignals(inputs, clock)

	if len(result.Signals) < 1 {
		t.Fatalf("expected at least 1 signal")
	}

	// Later should shift to Today (not Now) - max 1 step
	if result.Signals[0].Kind != tw.WindowToday {
		t.Errorf("expected WindowToday after 1-step shift, got %s", result.Signals[0].Kind)
	}
}

// Test 7: Max 3 signals enforced
func TestMax3SignalsEnforced(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create inputs that would generate many candidates
	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowSoon,
		},
		Inbox: tw.InboxWindowInputs{
			InstitutionalCountBucket: tw.MagnitudeAFew,
			HumanCountBucket:         tw.MagnitudeAFew,
			InstitutionWindowKind:    tw.WindowNow,
			HumanWindowKind:          tw.WindowSoon,
		},
		DeviceHints: tw.DeviceHintInputs{
			TransportSignals:   tw.MagnitudeAFew,
			HealthSignals:      tw.MagnitudeAFew,
			InstitutionSignals: tw.MagnitudeAFew,
		},
	}

	result := engine.BuildSignals(inputs, clock)

	if len(result.Signals) > tw.MaxSignals {
		t.Errorf("signals exceeded max: %d > %d", len(result.Signals), tw.MaxSignals)
	}
}

// Test 8: Stable hash selection (deterministic)
func TestStableHashSelection(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowSoon,
		},
		Inbox: tw.InboxWindowInputs{
			InstitutionalCountBucket: tw.MagnitudeAFew,
			InstitutionWindowKind:    tw.WindowNow,
		},
	}

	// Run multiple times
	var resultHashes []string
	for i := 0; i < 5; i++ {
		result := engine.BuildSignals(inputs, clock)
		resultHashes = append(resultHashes, result.ResultHash)
	}

	// All hashes should be identical
	for i := 1; i < len(resultHashes); i++ {
		if resultHashes[i] != resultHashes[0] {
			t.Errorf("hash selection not stable: %s != %s", resultHashes[i], resultHashes[0])
		}
	}
}

// Test 9: Adapter maps to pressure input correctly
func TestAdapterMapsToPressureInput(t *testing.T) {
	engine := internaltw.NewEngine()

	signal := &tw.TimeWindowSignal{
		Source:     tw.SourceCalendar,
		CircleType: tw.CircleHuman,
		Kind:       tw.WindowSoon,
		Reason:     tw.ReasonAppointment,
		Magnitude:  tw.MagnitudeAFew,
	}
	signal.StatusHash = signal.ComputeStatusHash()

	pressureInput := engine.SignalToPressureInput(signal, "2025-01-15")

	if pressureInput == nil {
		t.Fatal("expected pressure input, got nil")
	}

	// Verify mappings
	if pressureInput.Horizon != pd.HorizonSoon {
		t.Errorf("expected horizon soon, got %s", pressureInput.Horizon)
	}
}

// Test 10: One signal per CircleType
func TestOneSignalPerCircleType(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create inputs that generate multiple candidates for same circle type
	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowSoon,
		},
		DeviceHints: tw.DeviceHintInputs{
			TransportSignals: tw.MagnitudeAFew, // Also maps to Self
			HealthSignals:    tw.MagnitudeAFew, // Also maps to Self
		},
	}

	result := engine.BuildSignals(inputs, clock)

	// Count signals per circle type
	circleTypeCounts := make(map[tw.WindowCircleType]int)
	for _, s := range result.Signals {
		circleTypeCounts[s.CircleType]++
	}

	for ct, count := range circleTypeCounts {
		if count > 1 {
			t.Errorf("more than 1 signal for circle type %s: %d", ct, count)
		}
	}
}

// Test 11: Canonical strings are stable
func TestCanonicalStringsAreStable(t *testing.T) {
	signal := &tw.TimeWindowSignal{
		Source:         tw.SourceCalendar,
		CircleType:     tw.CircleSelf,
		Kind:           tw.WindowSoon,
		Reason:         tw.ReasonAppointment,
		Magnitude:      tw.MagnitudeAFew,
		EvidenceHashes: []string{"hash1", "hash2"},
	}

	canonical1 := signal.CanonicalString()
	canonical2 := signal.CanonicalString()

	if canonical1 != canonical2 {
		t.Errorf("canonical strings not stable: %s != %s", canonical1, canonical2)
	}
}

// Test 12: Validate rejects invalid enums
func TestValidateRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"invalid source", func() error { return tw.WindowSourceKind("invalid").Validate() }},
		{"invalid kind", func() error { return tw.WindowKind("invalid").Validate() }},
		{"invalid reason", func() error { return tw.WindowReasonBucket("invalid").Validate() }},
		{"invalid circle type", func() error { return tw.WindowCircleType("invalid").Validate() }},
		{"invalid magnitude", func() error { return tw.WindowMagnitudeBucket("invalid").Validate() }},
		{"invalid evidence kind", func() error { return tw.WindowEvidenceKind("invalid").Validate() }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

// Test 13: Period key generation is deterministic
func TestPeriodKeyGeneration(t *testing.T) {
	testCases := []struct {
		input    time.Time
		expected string
	}{
		{time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), "2025-01-15T10:00"},
		{time.Date(2025, 1, 15, 10, 7, 0, 0, time.UTC), "2025-01-15T10:00"},
		{time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC), "2025-01-15T10:15"},
		{time.Date(2025, 1, 15, 10, 29, 0, 0, time.UTC), "2025-01-15T10:15"},
		{time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), "2025-01-15T10:30"},
		{time.Date(2025, 1, 15, 10, 44, 0, 0, time.UTC), "2025-01-15T10:30"},
		{time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC), "2025-01-15T10:45"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := tw.NewPeriodKey(tc.input)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// Test 14: ShiftEarlier works correctly
func TestShiftEarlier(t *testing.T) {
	tests := []struct {
		input    tw.WindowKind
		expected tw.WindowKind
	}{
		{tw.WindowLater, tw.WindowToday},
		{tw.WindowToday, tw.WindowSoon},
		{tw.WindowSoon, tw.WindowNow},
		{tw.WindowNow, tw.WindowNow}, // Max boundary
	}

	for _, tc := range tests {
		t.Run(string(tc.input), func(t *testing.T) {
			result := tc.input.ShiftEarlier()
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// Test 15: IncrementMagnitude works correctly
func TestIncrementMagnitude(t *testing.T) {
	tests := []struct {
		input    tw.WindowMagnitudeBucket
		expected tw.WindowMagnitudeBucket
	}{
		{tw.MagnitudeNothing, tw.MagnitudeAFew},
		{tw.MagnitudeAFew, tw.MagnitudeSeveral},
		{tw.MagnitudeSeveral, tw.MagnitudeSeveral}, // Max boundary
	}

	for _, tc := range tests {
		t.Run(string(tc.input), func(t *testing.T) {
			result := tc.input.IncrementMagnitude()
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// Test 16: Source precedence ordering
func TestSourcePrecedenceOrdering(t *testing.T) {
	sources := tw.AllWindowSourceKinds()

	// calendar < inbox_institution < inbox_human < device_hint
	if sources[0] != tw.SourceCalendar {
		t.Errorf("expected first source to be calendar, got %s", sources[0])
	}
	if sources[1] != tw.SourceInboxInstitution {
		t.Errorf("expected second source to be inbox_institution, got %s", sources[1])
	}
	if sources[2] != tw.SourceInboxHuman {
		t.Errorf("expected third source to be inbox_human, got %s", sources[2])
	}
	if sources[3] != tw.SourceDeviceHint {
		t.Errorf("expected fourth source to be device_hint, got %s", sources[3])
	}
}

// Test 17: Proof page building
func TestProofPageBuilding(t *testing.T) {
	result := &tw.TimeWindowBuildResult{
		CircleIDHash: "circle123",
		PeriodKey:    "2025-01-15T10:00",
		Status:       tw.StatusOK,
		Signals: []tw.TimeWindowSignal{
			{
				Source:     tw.SourceCalendar,
				CircleType: tw.CircleSelf,
				Magnitude:  tw.MagnitudeAFew,
			},
		},
	}
	result.ResultHash = result.ComputeResultHash()

	page := tw.BuildWindowsProofPage("circle123", result)

	if page.MagnitudeBucket != tw.MagnitudeAFew {
		t.Errorf("expected magnitude a_few, got %s", page.MagnitudeBucket)
	}
	if page.Status != tw.StatusOK {
		t.Errorf("expected status ok, got %s", page.Status)
	}
}

// Test 18: GetOverallMagnitude returns highest
func TestGetOverallMagnitudeReturnsHighest(t *testing.T) {
	result := &tw.TimeWindowBuildResult{
		Signals: []tw.TimeWindowSignal{
			{Magnitude: tw.MagnitudeNothing},
			{Magnitude: tw.MagnitudeSeveral},
			{Magnitude: tw.MagnitudeAFew},
		},
	}

	mag := result.GetOverallMagnitude()
	if mag != tw.MagnitudeSeveral {
		t.Errorf("expected several, got %s", mag)
	}
}

// Test 19: GetSourceChips returns unique sources
func TestGetSourceChipsReturnsUnique(t *testing.T) {
	result := &tw.TimeWindowBuildResult{
		Signals: []tw.TimeWindowSignal{
			{Source: tw.SourceCalendar},
			{Source: tw.SourceCalendar},
			{Source: tw.SourceInboxHuman},
		},
	}

	chips := result.GetSourceChips()

	// Should have 2 unique chips
	if len(chips) != 2 {
		t.Errorf("expected 2 unique chips, got %d", len(chips))
	}
}

// Test 20: Nil inputs return empty result
func TestNilInputsReturnEmpty(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	result := engine.BuildSignals(nil, clock)

	if result.Status != tw.StatusEmpty {
		t.Errorf("expected status empty for nil inputs, got %s", result.Status)
	}
}

// Test 21: All enum functions work
func TestAllEnumsFunctions(t *testing.T) {
	// Test AllWindowSourceKinds
	sources := tw.AllWindowSourceKinds()
	if len(sources) != 4 {
		t.Errorf("expected 4 sources, got %d", len(sources))
	}

	// Test AllWindowKinds
	kinds := tw.AllWindowKinds()
	if len(kinds) != 4 {
		t.Errorf("expected 4 window kinds, got %d", len(kinds))
	}

	// Test AllWindowReasonBuckets
	reasons := tw.AllWindowReasonBuckets()
	if len(reasons) != 7 {
		t.Errorf("expected 7 reason buckets, got %d", len(reasons))
	}

	// Test AllWindowCircleTypes
	circleTypes := tw.AllWindowCircleTypes()
	if len(circleTypes) != 3 {
		t.Errorf("expected 3 circle types, got %d", len(circleTypes))
	}

	// Test AllWindowMagnitudeBuckets
	magnitudes := tw.AllWindowMagnitudeBuckets()
	if len(magnitudes) != 3 {
		t.Errorf("expected 3 magnitudes, got %d", len(magnitudes))
	}
}

// Test 22: ToMagnitudeBucket works correctly
func TestToMagnitudeBucket(t *testing.T) {
	tests := []struct {
		count    int
		expected tw.WindowMagnitudeBucket
	}{
		{0, tw.MagnitudeNothing},
		{1, tw.MagnitudeAFew},
		{3, tw.MagnitudeAFew},
		{4, tw.MagnitudeSeveral},
		{100, tw.MagnitudeSeveral},
	}

	for _, tc := range tests {
		result := tw.ToMagnitudeBucket(tc.count)
		if result != tc.expected {
			t.Errorf("count %d: expected %s, got %s", tc.count, tc.expected, result)
		}
	}
}

// Test 23: DisplayText methods work
func TestDisplayTextMethods(t *testing.T) {
	// Test source display
	if tw.SourceCalendar.DisplayText() != "calendar" {
		t.Errorf("unexpected calendar display text")
	}

	// Test kind display
	if tw.WindowNow.DisplayText() != "now" {
		t.Errorf("unexpected now display text")
	}

	// Test magnitude display
	if tw.MagnitudeAFew.DisplayText() != "a few" {
		t.Errorf("unexpected a_few display text")
	}

	// Test circle type display
	if tw.CircleHuman.DisplayText() != "human" {
		t.Errorf("unexpected human display text")
	}

	// Test reason display
	if tw.ReasonAppointment.DisplayText() != "appointment" {
		t.Errorf("unexpected appointment display text")
	}
}

// Test 24: Envelope without shift (working kind)
func TestEnvelopeWithoutShift(t *testing.T) {
	engine := internaltw.NewEngine()
	clock := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	inputs := &tw.TimeWindowInputs{
		CircleIDHash: "circle123",
		NowBucket:    tw.NewPeriodKey(clock),
		Calendar: tw.CalendarWindowInputs{
			HasUpcoming:         true,
			UpcomingCountBucket: tw.MagnitudeAFew,
			NextStartsIn:        tw.WindowLater,
		},
		EnvelopeSummary: tw.AttentionEnvelopeSummary{
			IsActive: true,
			Kind:     ae.EnvelopeKindWorking, // Working does not shift
		},
	}

	result := engine.BuildSignals(inputs, clock)

	if len(result.Signals) < 1 {
		t.Fatalf("expected at least 1 signal")
	}

	// Working envelope should NOT shift window
	if result.Signals[0].Kind != tw.WindowLater {
		t.Errorf("expected WindowLater (no shift for working), got %s", result.Signals[0].Kind)
	}
}

// Test 25: Calm whisper cue
func TestCalmWhisperCue(t *testing.T) {
	engine := internaltw.NewEngine()

	// With signals
	resultWithSignals := &tw.TimeWindowBuildResult{
		Signals: []tw.TimeWindowSignal{{Magnitude: tw.MagnitudeAFew}},
	}
	cue := engine.GetCalmWhisperCue(resultWithSignals)
	if cue == "" {
		t.Error("expected cue for result with signals")
	}

	// Without signals
	resultEmpty := &tw.TimeWindowBuildResult{
		Signals: nil,
	}
	cueEmpty := engine.GetCalmWhisperCue(resultEmpty)
	if cueEmpty != "" {
		t.Error("expected no cue for empty result")
	}
}

// Test 26: Build result validation
func TestBuildResultValidation(t *testing.T) {
	// Valid result
	validResult := &tw.TimeWindowBuildResult{
		Status:     tw.StatusOK,
		ResultHash: "abc123",
	}
	if err := validResult.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	// Missing hash
	invalidResult := &tw.TimeWindowBuildResult{
		Status:     tw.StatusOK,
		ResultHash: "",
	}
	if err := invalidResult.Validate(); err == nil {
		t.Error("expected validation error for missing hash")
	}
}

// Test 27: Signal validation
func TestSignalValidation(t *testing.T) {
	validSignal := tw.TimeWindowSignal{
		Source:     tw.SourceCalendar,
		CircleType: tw.CircleSelf,
		Kind:       tw.WindowSoon,
		Reason:     tw.ReasonAppointment,
		Magnitude:  tw.MagnitudeAFew,
	}
	if err := validSignal.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	// Too many evidence hashes
	invalidSignal := tw.TimeWindowSignal{
		Source:         tw.SourceCalendar,
		CircleType:     tw.CircleSelf,
		Kind:           tw.WindowSoon,
		Reason:         tw.ReasonAppointment,
		Magnitude:      tw.MagnitudeAFew,
		EvidenceHashes: []string{"h1", "h2", "h3", "h4"}, // > MaxEvidenceHashes
	}
	if err := invalidSignal.Validate(); err == nil {
		t.Error("expected validation error for too many evidence hashes")
	}
}
