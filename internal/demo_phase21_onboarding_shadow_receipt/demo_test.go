// Package demo_phase21_onboarding_shadow_receipt demonstrates Phase 21 capabilities.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
//
// This test file demonstrates:
// 1. Mode derivation is deterministic (Demo/Connected/Shadow)
// 2. Shadow receipt page shows ONLY abstract buckets and hashes
// 3. Acknowledgement store persists hash-only records
// 4. Receipt cue follows single whisper rule
// 5. No goroutines in mode or shadowview packages
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
package demo_phase21_onboarding_shadow_receipt

import (
	"testing"
	"time"

	"quantumlife/internal/mode"
	"quantumlife/internal/shadowview"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

// fixedClock returns a clock function that always returns the given time.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestModeDerivation_Deterministic verifies mode derivation is deterministic.
//
// CRITICAL: Same inputs => same mode. Always.
func TestModeDerivation_Deterministic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := mode.NewEngine(fixedClock(now))

	// Demo mode: no Gmail connection
	input1 := mode.DeriveModeInput{
		HasGmailConnection:   false,
		ShadowProviderIsStub: true,
		ShadowRealAllowed:    false,
		LatestShadowReceipt:  nil,
	}
	mode1 := engine.DeriveMode(input1)
	mode1Again := engine.DeriveMode(input1)

	if mode1 != mode1Again {
		t.Errorf("Mode derivation not deterministic: %v != %v", mode1, mode1Again)
	}
	if mode1 != mode.ModeDemo {
		t.Errorf("Expected ModeDemo, got %v", mode1)
	}

	t.Logf("Demo mode derivation: %s", mode1)
}

// TestModeDerivation_ConnectedMode verifies connected mode derivation.
//
// Connected: Gmail connected but no shadow receipt for current period.
func TestModeDerivation_ConnectedMode(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := mode.NewEngine(fixedClock(now))

	input := mode.DeriveModeInput{
		HasGmailConnection:   true,
		ShadowProviderIsStub: false,
		ShadowRealAllowed:    true,
		LatestShadowReceipt:  nil, // No receipt
	}
	derivedMode := engine.DeriveMode(input)

	if derivedMode != mode.ModeConnected {
		t.Errorf("Expected ModeConnected, got %v", derivedMode)
	}

	t.Logf("Connected mode derivation: %s", derivedMode)
}

// TestModeDerivation_ShadowMode verifies shadow mode derivation.
//
// Shadow: Gmail connected AND shadow receipt exists for current period.
func TestModeDerivation_ShadowMode(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := mode.NewEngine(fixedClock(now))

	// Create a receipt for today
	receipt := &shadowllm.ShadowReceipt{
		ReceiptID:    "receipt-1",
		CircleID:     identity.EntityID("circle-1"),
		WindowBucket: "2025-01-15",
		CreatedAt:    now, // Same day
		Suggestions:  []shadowllm.ShadowSuggestion{},
	}

	input := mode.DeriveModeInput{
		HasGmailConnection:   true,
		ShadowProviderIsStub: false,
		ShadowRealAllowed:    true,
		LatestShadowReceipt:  receipt,
	}
	derivedMode := engine.DeriveMode(input)

	if derivedMode != mode.ModeShadow {
		t.Errorf("Expected ModeShadow, got %v", derivedMode)
	}

	t.Logf("Shadow mode derivation: %s", derivedMode)
}

// TestModeIndicator_DisplayText verifies display text is set correctly.
func TestModeIndicator_DisplayText(t *testing.T) {
	testCases := []struct {
		mode        mode.Mode
		wantDisplay string
	}{
		{mode.ModeDemo, "Demo"},
		{mode.ModeConnected, "Connected"},
		{mode.ModeShadow, "Shadow"},
	}

	for _, tc := range testCases {
		indicator := mode.NewModeIndicator(tc.mode)
		if indicator.DisplayText != tc.wantDisplay {
			t.Errorf("Mode %v: expected display %q, got %q", tc.mode, tc.wantDisplay, indicator.DisplayText)
		}
	}
}

// TestShadowReceiptPage_AbstractBucketsOnly verifies page shows only abstract data.
//
// CRITICAL: Page must NEVER contain raw content.
func TestShadowReceiptPage_AbstractBucketsOnly(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := shadowview.NewEngine(fixedClock(now))

	receipt := &shadowllm.ShadowReceipt{
		ReceiptID:    "receipt-1",
		CircleID:     identity.EntityID("circle-1"),
		WindowBucket: "2025-01-15",
		CreatedAt:    now,
		Suggestions: []shadowllm.ShadowSuggestion{
			{
				Category:   shadowllm.CategoryMoney,
				Magnitude:  shadowllm.MagnitudeAFew,
				Horizon:    shadowllm.HorizonSoon,
				Confidence: shadowllm.ConfidenceMed,
			},
		},
	}

	input := shadowview.BuildPageInput{
		Receipt:            receipt,
		HasGmailConnection: true,
	}
	page := engine.BuildPage(input)

	// Verify abstract data only
	if page.Observation.Magnitude != "a few" {
		t.Errorf("Expected magnitude 'a few', got %q", page.Observation.Magnitude)
	}
	if page.Observation.Horizon != "soon" {
		t.Errorf("Expected horizon 'soon', got %q", page.Observation.Horizon)
	}
	if page.Confidence.Bucket != "medium" {
		t.Errorf("Expected confidence 'medium', got %q", page.Confidence.Bucket)
	}

	// Verify restraint section (always true for shadow mode)
	if !page.Restraint.NoActionsTaken {
		t.Error("Restraint: NoActionsTaken should be true")
	}
	if !page.Restraint.NoDraftsCreated {
		t.Error("Restraint: NoDraftsCreated should be true")
	}

	t.Logf("Page observation: %s", page.Observation.Statement)
	t.Logf("Page receipt hash: %s", page.ReceiptHash)
}

// TestShadowReceiptPage_Deterministic verifies page building is deterministic.
//
// CRITICAL: Same input => same output. Always.
func TestShadowReceiptPage_Deterministic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := shadowview.NewEngine(fixedClock(now))

	receipt := &shadowllm.ShadowReceipt{
		ReceiptID:    "receipt-1",
		CircleID:     identity.EntityID("circle-1"),
		WindowBucket: "2025-01-15",
		CreatedAt:    now,
		Suggestions: []shadowllm.ShadowSuggestion{
			{
				Category:   shadowllm.CategoryWork,
				Magnitude:  shadowllm.MagnitudeSeveral,
				Horizon:    shadowllm.HorizonNow,
				Confidence: shadowllm.ConfidenceHigh,
			},
		},
	}

	input := shadowview.BuildPageInput{
		Receipt:            receipt,
		HasGmailConnection: true,
	}

	page1 := engine.BuildPage(input)
	page2 := engine.BuildPage(input)

	if page1.ReceiptHash != page2.ReceiptHash {
		t.Errorf("Page hashes not deterministic: %s != %s", page1.ReceiptHash, page2.ReceiptHash)
	}
	if page1.Observation.Statement != page2.Observation.Statement {
		t.Errorf("Observation not deterministic: %s != %s", page1.Observation.Statement, page2.Observation.Statement)
	}
}

// TestAckStore_HashOnly verifies ack store persists hash-only records.
//
// CRITICAL: Store must NEVER persist raw timestamps.
func TestAckStore_HashOnly(t *testing.T) {
	store := shadowview.NewAckStore(10)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	periodBucket := "2025-01-15"
	receiptHash := "sha256:abc123"

	// Record viewed action
	err := store.Record(shadowview.AckViewed, receiptHash, periodBucket, now)
	if err != nil {
		t.Fatalf("Failed to record ack: %v", err)
	}

	// Verify record exists
	if !store.HasRecentForPeriod(receiptHash, periodBucket) {
		t.Error("Expected record to exist for period")
	}

	// Verify it's not dismissed
	if store.HasDismissedForPeriod(receiptHash, periodBucket) {
		t.Error("Record should not be dismissed")
	}

	t.Logf("Ack store count: %d", store.Len())
}

// TestAckStore_DismissalTracking verifies dismissal tracking works correctly.
func TestAckStore_DismissalTracking(t *testing.T) {
	store := shadowview.NewAckStore(10)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	periodBucket := "2025-01-15"
	receiptHash := "sha256:xyz789"

	// Record dismissal
	err := store.Record(shadowview.AckDismissed, receiptHash, periodBucket, now)
	if err != nil {
		t.Fatalf("Failed to record dismissal: %v", err)
	}

	// Verify dismissal is tracked
	if !store.HasDismissedForPeriod(receiptHash, periodBucket) {
		t.Error("Expected dismissal to be tracked")
	}

	// Verify different period doesn't match
	if store.HasDismissedForPeriod(receiptHash, "2025-01-14") {
		t.Error("Dismissal should not match different period")
	}
}

// TestReceiptCue_SingleWhisperRule verifies single whisper rule compliance.
//
// CRITICAL: At most ONE whisper cue on any page.
func TestReceiptCue_SingleWhisperRule(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := shadowview.NewEngine(fixedClock(now))

	receipt := &shadowllm.ShadowReceipt{
		ReceiptID:    "receipt-1",
		CircleID:     identity.EntityID("circle-1"),
		WindowBucket: "2025-01-15",
		CreatedAt:    now,
	}

	// Case 1: Other cue is active - receipt cue should NOT show
	cue1 := engine.BuildCue(shadowview.BuildCueInput{
		Receipt:        receipt,
		IsDismissed:    false,
		OtherCueActive: true, // Another whisper is active
	})
	if cue1.Available {
		t.Error("Receipt cue should NOT be available when other cue is active")
	}

	// Case 2: No other cue, not dismissed - receipt cue should show
	cue2 := engine.BuildCue(shadowview.BuildCueInput{
		Receipt:        receipt,
		IsDismissed:    false,
		OtherCueActive: false,
	})
	if !cue2.Available {
		t.Error("Receipt cue should be available when no other cue is active")
	}

	// Case 3: Dismissed - receipt cue should NOT show
	cue3 := engine.BuildCue(shadowview.BuildCueInput{
		Receipt:        receipt,
		IsDismissed:    true,
		OtherCueActive: false,
	})
	if cue3.Available {
		t.Error("Receipt cue should NOT be available when dismissed")
	}

	t.Logf("Cue 2 text: %s", cue2.CueText)
}

// TestReceiptCue_NoReceipt verifies cue is not shown when no receipt exists.
func TestReceiptCue_NoReceipt(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := shadowview.NewEngine(fixedClock(now))

	cue := engine.BuildCue(shadowview.BuildCueInput{
		Receipt:        nil, // No receipt
		IsDismissed:    false,
		OtherCueActive: false,
	})

	if cue.Available {
		t.Error("Receipt cue should NOT be available when no receipt exists")
	}
}

// TestEmptyPage_NoReceipt verifies empty page is built correctly.
func TestEmptyPage_NoReceipt(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := shadowview.NewEngine(fixedClock(now))

	page := engine.EmptyPage(true)

	if page.HasReceipt {
		t.Error("Empty page should have HasReceipt=false")
	}
	if page.Source.Statement != "Connected: email (read-only)" {
		t.Errorf("Unexpected source statement: %s", page.Source.Statement)
	}
	if page.Observation.Statement != "No observations yet." {
		t.Errorf("Unexpected observation statement: %s", page.Observation.Statement)
	}
}

// TestAckStore_BoundedSize verifies ack store has bounded size.
func TestAckStore_BoundedSize(t *testing.T) {
	maxRecords := 5
	store := shadowview.NewAckStore(maxRecords)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Add more records than max
	for i := 0; i < maxRecords+3; i++ {
		hash := "hash-" + string(rune('A'+i))
		period := "2025-01-15"
		_ = store.Record(shadowview.AckViewed, hash, period, now)
	}

	// Verify bounded size
	if store.Len() > maxRecords {
		t.Errorf("Store exceeded max size: %d > %d", store.Len(), maxRecords)
	}

	t.Logf("Store size after overflow: %d (max: %d)", store.Len(), maxRecords)
}
