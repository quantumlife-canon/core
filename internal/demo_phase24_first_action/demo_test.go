// Package demo_phase24_first_action demonstrates Phase 24: First Reversible Real Action.
//
// These tests verify:
//   - Preview only, never execution
//   - One action per period maximum
//   - Deterministic held item selection
//   - Hash-only persistence
//   - Silence resumes after action
//   - Pipe-delimited canonical strings
//   - Clock injection pattern
//
// Reference: docs/ADR/ADR-0054-phase24-first-reversible-action.md
package demo_phase24_first_action

import (
	"testing"
	"time"

	internalfirstaction "quantumlife/internal/firstaction"
	"quantumlife/internal/persist"
	domainfirstaction "quantumlife/pkg/domain/firstaction"
	"quantumlife/pkg/domain/identity"
)

// fixedClock returns a clock that always returns the same time.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestActionPeriodDeterminism verifies that same date produces same period hash.
func TestActionPeriodDeterminism(t *testing.T) {
	period1 := domainfirstaction.NewActionPeriod("2024-01-15")
	period2 := domainfirstaction.NewActionPeriod("2024-01-15")

	if period1.PeriodHash != period2.PeriodHash {
		t.Errorf("same date must produce same hash: got %s vs %s", period1.PeriodHash, period2.PeriodHash)
	}

	// Different dates produce different hashes
	period3 := domainfirstaction.NewActionPeriod("2024-01-16")
	if period1.PeriodHash == period3.PeriodHash {
		t.Error("different dates must produce different hashes")
	}
}

// TestEligibilityRequiresAllConditions verifies eligibility logic.
func TestEligibilityRequiresAllConditions(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	engine := internalfirstaction.NewEngine(clock)

	// Full eligibility
	trust := &internalfirstaction.TrustInputs{
		HasQuietBaseline: true,
		HasMirrorViewed:  true,
		HasTrustAccrual:  true,
		TrustScore:       0.8,
	}
	e := engine.ComputeEligibility("circle-1", true, trust, false, true)
	if !e.IsEligible() {
		t.Error("should be eligible with all conditions met")
	}

	// Missing Gmail connection
	e = engine.ComputeEligibility("circle-1", false, trust, false, true)
	if e.IsEligible() {
		t.Error("should not be eligible without Gmail connection")
	}

	// Missing quiet baseline
	trustNoQuiet := &internalfirstaction.TrustInputs{
		HasQuietBaseline: false,
		HasMirrorViewed:  true,
		HasTrustAccrual:  true,
		TrustScore:       0.8,
	}
	e = engine.ComputeEligibility("circle-1", true, trustNoQuiet, false, true)
	if e.IsEligible() {
		t.Error("should not be eligible without quiet baseline")
	}

	// Prior action this period
	e = engine.ComputeEligibility("circle-1", true, trust, true, true)
	if e.IsEligible() {
		t.Error("should not be eligible with prior action this period")
	}

	// No held items
	e = engine.ComputeEligibility("circle-1", true, trust, false, false)
	if e.IsEligible() {
		t.Error("should not be eligible without held items")
	}
}

// TestSelectHeldItemDeterministic verifies deterministic selection.
func TestSelectHeldItemDeterministic(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	engine := internalfirstaction.NewEngine(clock)

	items := []internalfirstaction.HeldItemAbstract{
		{Hash: "ccc", Category: domainfirstaction.CategoryWork, Horizon: domainfirstaction.HorizonLater, Magnitude: domainfirstaction.MagnitudeSmall},
		{Hash: "aaa", Category: domainfirstaction.CategoryMoney, Horizon: domainfirstaction.HorizonSoon, Magnitude: domainfirstaction.MagnitudeMedium},
		{Hash: "bbb", Category: domainfirstaction.CategoryTime, Horizon: domainfirstaction.HorizonSomeday, Magnitude: domainfirstaction.MagnitudeLarge},
	}

	// Should always select "aaa" (lowest hash)
	selected := engine.SelectHeldItem(items)
	if selected == nil {
		t.Fatal("expected a selection")
	}
	if selected.Hash != "aaa" {
		t.Errorf("expected lowest hash 'aaa', got %s", selected.Hash)
	}

	// Same items in different order should produce same result
	items2 := []internalfirstaction.HeldItemAbstract{items[1], items[2], items[0]}
	selected2 := engine.SelectHeldItem(items2)
	if selected2.Hash != selected.Hash {
		t.Error("selection must be deterministic regardless of input order")
	}
}

// TestSelectHeldItemEmpty verifies empty list handling.
func TestSelectHeldItemEmpty(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	engine := internalfirstaction.NewEngine(clock)

	selected := engine.SelectHeldItem(nil)
	if selected != nil {
		t.Error("expected nil for empty list")
	}

	selected = engine.SelectHeldItem([]internalfirstaction.HeldItemAbstract{})
	if selected != nil {
		t.Error("expected nil for empty slice")
	}
}

// TestBuildPreviewAbstractOnly verifies preview contains only abstract data.
func TestBuildPreviewAbstractOnly(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	engine := internalfirstaction.NewEngine(clock)

	item := &internalfirstaction.HeldItemAbstract{
		Hash:      "test-hash",
		Category:  domainfirstaction.CategoryMoney,
		Horizon:   domainfirstaction.HorizonSoon,
		Magnitude: domainfirstaction.MagnitudeMedium,
	}

	preview := engine.BuildPreview("circle-1", item)
	if preview == nil {
		t.Fatal("expected preview")
	}

	// Verify abstract data only
	if preview.CircleID != "circle-1" {
		t.Errorf("expected circle-1, got %s", preview.CircleID)
	}
	if preview.Category != domainfirstaction.CategoryMoney {
		t.Errorf("expected money category, got %s", preview.Category)
	}
	if preview.Horizon != domainfirstaction.HorizonSoon {
		t.Errorf("expected soon horizon, got %s", preview.Horizon)
	}
	if preview.Magnitude != domainfirstaction.MagnitudeMedium {
		t.Errorf("expected medium magnitude, got %s", preview.Magnitude)
	}
	if preview.SourceHash != "test-hash" {
		t.Errorf("expected test-hash, got %s", preview.SourceHash)
	}
}

// TestPreviewHashDeterminism verifies same preview produces same hash.
func TestPreviewHashDeterminism(t *testing.T) {
	preview1 := &domainfirstaction.ActionPreview{
		CircleID:   "circle-1",
		Period:     domainfirstaction.NewActionPeriod("2024-01-15"),
		Category:   domainfirstaction.CategoryMoney,
		Horizon:    domainfirstaction.HorizonSoon,
		Magnitude:  domainfirstaction.MagnitudeMedium,
		SourceHash: "source-123",
	}

	preview2 := &domainfirstaction.ActionPreview{
		CircleID:   "circle-1",
		Period:     domainfirstaction.NewActionPeriod("2024-01-15"),
		Category:   domainfirstaction.CategoryMoney,
		Horizon:    domainfirstaction.HorizonSoon,
		Magnitude:  domainfirstaction.MagnitudeMedium,
		SourceHash: "source-123",
	}

	if preview1.Hash() != preview2.Hash() {
		t.Error("same preview must produce same hash")
	}

	// Different preview produces different hash
	preview3 := &domainfirstaction.ActionPreview{
		CircleID:   "circle-2",
		Period:     domainfirstaction.NewActionPeriod("2024-01-15"),
		Category:   domainfirstaction.CategoryMoney,
		Horizon:    domainfirstaction.HorizonSoon,
		Magnitude:  domainfirstaction.MagnitudeMedium,
		SourceHash: "source-123",
	}
	if preview1.Hash() == preview3.Hash() {
		t.Error("different preview must produce different hash")
	}
}

// TestCanonicalStringPipeDelimited verifies pipe-delimited format.
func TestCanonicalStringPipeDelimited(t *testing.T) {
	preview := &domainfirstaction.ActionPreview{
		CircleID:   "circle-1",
		Period:     domainfirstaction.NewActionPeriod("2024-01-15"),
		Category:   domainfirstaction.CategoryMoney,
		Horizon:    domainfirstaction.HorizonSoon,
		Magnitude:  domainfirstaction.MagnitudeMedium,
		SourceHash: "source-123",
	}

	canonical := preview.CanonicalString()

	// Must contain pipe delimiters
	if len(canonical) == 0 {
		t.Fatal("canonical string is empty")
	}

	// Check format: "ACTION_PREVIEW|v1|circle|date|category|horizon|magnitude|source"
	expected := "ACTION_PREVIEW|v1|circle-1|2024-01-15|money|soon|medium|source-123"
	if canonical != expected {
		t.Errorf("expected %s, got %s", expected, canonical)
	}
}

// TestActionRecordCanonicalString verifies record canonical format.
func TestActionRecordCanonicalString(t *testing.T) {
	period := domainfirstaction.NewActionPeriod("2024-01-15")
	record := &domainfirstaction.ActionRecord{
		ActionHash: "action-hash-123",
		State:      domainfirstaction.StateViewed,
		PeriodHash: period.PeriodHash,
		CircleID:   "circle-1",
	}

	canonical := record.CanonicalString()
	expected := "ACTION_RECORD|v1|circle-1|action-hash-123|viewed|" + period.PeriodHash
	if canonical != expected {
		t.Errorf("expected %s, got %s", expected, canonical)
	}
}

// TestStoreOnePerPeriod verifies one action per period enforcement.
func TestStoreOnePerPeriod(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	store := persist.NewFirstActionStore(clock)
	circleID := identity.EntityID("circle-1")
	period := domainfirstaction.NewActionPeriod("2024-01-15")

	// Initially no action this period
	if store.HasActionThisPeriod(circleID, period.PeriodHash) {
		t.Error("should have no action initially")
	}

	// Record a view
	_ = store.RecordState(circleID, "action-1", domainfirstaction.StateViewed, period.PeriodHash)

	// Now has action this period
	if !store.HasActionThisPeriod(circleID, period.PeriodHash) {
		t.Error("should have action after view")
	}

	// Different period should not be affected
	period2 := domainfirstaction.NewActionPeriod("2024-01-16")
	if store.HasActionThisPeriod(circleID, period2.PeriodHash) {
		t.Error("different period should not have action")
	}
}

// TestStoreDeduplication verifies hash-based deduplication.
func TestStoreDeduplication(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	store := persist.NewFirstActionStore(clock)
	circleID := identity.EntityID("circle-1")
	period := domainfirstaction.NewActionPeriod("2024-01-15")

	// Store same record twice
	record := &domainfirstaction.ActionRecord{
		ActionHash: "action-1",
		State:      domainfirstaction.StateViewed,
		PeriodHash: period.PeriodHash,
		CircleID:   string(circleID),
	}

	_ = store.Store(record)
	_ = store.Store(record) // duplicate

	if store.Count() != 1 {
		t.Errorf("expected 1 record after dedup, got %d", store.Count())
	}
}

// TestStoreBoundedRetention verifies bounded storage.
func TestStoreBoundedRetention(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	store := persist.NewFirstActionStore(clock)
	circleID := identity.EntityID("circle-1")
	period := domainfirstaction.NewActionPeriod("2024-01-15")

	// Store many records (more than maxEntries)
	for i := 0; i < 1100; i++ {
		record := &domainfirstaction.ActionRecord{
			ActionHash: "action-" + string(rune(i)),
			State:      domainfirstaction.StateViewed,
			PeriodHash: period.PeriodHash,
			CircleID:   string(circleID),
		}
		_ = store.Store(record)
	}

	// Should be capped at maxEntries (1000)
	if store.Count() > 1000 {
		t.Errorf("expected max 1000 records, got %d", store.Count())
	}
}

// TestActionPageHasNoPanic verifies page building doesn't panic.
func TestActionPageHasNoPanic(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	engine := internalfirstaction.NewEngine(clock)

	// With nil eligibility
	page := engine.BuildActionPage(nil, domainfirstaction.CategoryWork)
	if page == nil {
		t.Fatal("expected page even with nil eligibility")
	}
	if page.HasAction {
		t.Error("nil eligibility should not have action")
	}

	// Empty page
	emptyPage := domainfirstaction.NewEmptyActionPage()
	if emptyPage.HasAction {
		t.Error("empty page should not have action")
	}
	if emptyPage.Title == "" {
		t.Error("empty page should have title")
	}
}

// TestPreviewPageHasDisclaimer verifies disclaimer text.
func TestPreviewPageHasDisclaimer(t *testing.T) {
	preview := &domainfirstaction.ActionPreview{
		CircleID:   "circle-1",
		Period:     domainfirstaction.NewActionPeriod("2024-01-15"),
		Category:   domainfirstaction.CategoryMoney,
		Horizon:    domainfirstaction.HorizonSoon,
		Magnitude:  domainfirstaction.MagnitudeMedium,
		SourceHash: "source-123",
	}

	page := domainfirstaction.NewPreviewPage(preview)
	if page.Disclaimer == "" {
		t.Error("preview page must have disclaimer")
	}

	// Must mention preview and no action
	if page.Disclaimer != "This is a preview. We did not act." {
		t.Errorf("unexpected disclaimer: %s", page.Disclaimer)
	}
}

// TestWhisperCueLowestPriority verifies whisper cue behavior.
func TestWhisperCueLowestPriority(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	engine := internalfirstaction.NewEngine(clock)

	// Not eligible - no whisper
	cue := engine.BuildWhisperCue(nil)
	if cue.Show {
		t.Error("nil eligibility should not show whisper")
	}

	// Not eligible - no whisper
	trust := &internalfirstaction.TrustInputs{
		HasQuietBaseline: true,
		HasMirrorViewed:  true,
		HasTrustAccrual:  true,
		TrustScore:       0.8,
	}
	e := engine.ComputeEligibility("circle-1", false, trust, false, true) // no gmail
	cue = engine.BuildWhisperCue(e)
	if cue.Show {
		t.Error("ineligible should not show whisper")
	}

	// Eligible - show whisper
	e = engine.ComputeEligibility("circle-1", true, trust, false, true)
	cue = engine.BuildWhisperCue(e)
	if !cue.Show {
		t.Error("eligible should show whisper")
	}
	if cue.Link != "/action/once" {
		t.Errorf("expected /action/once, got %s", cue.Link)
	}
}

// TestCategoryDisplayText verifies calm category text.
func TestCategoryDisplayText(t *testing.T) {
	tests := []struct {
		cat      domainfirstaction.AbstractCategory
		expected string
	}{
		{domainfirstaction.CategoryMoney, "Something about money"},
		{domainfirstaction.CategoryTime, "Something about time"},
		{domainfirstaction.CategoryWork, "Something about work"},
		{domainfirstaction.CategoryPeople, "Something about people"},
		{domainfirstaction.CategoryHome, "Something about home"},
	}

	for _, tt := range tests {
		got := tt.cat.DisplayText()
		if got != tt.expected {
			t.Errorf("category %s: expected %s, got %s", tt.cat, tt.expected, got)
		}
	}
}

// TestHorizonDisplayText verifies calm horizon text.
func TestHorizonDisplayText(t *testing.T) {
	tests := []struct {
		h        domainfirstaction.HorizonBucket
		expected string
	}{
		{domainfirstaction.HorizonSoon, "This is often easier earlier."},
		{domainfirstaction.HorizonLater, "This can wait a bit."},
		{domainfirstaction.HorizonSomeday, "No rush on this one."},
	}

	for _, tt := range tests {
		got := tt.h.DisplayText()
		if got != tt.expected {
			t.Errorf("horizon %s: expected %s, got %s", tt.h, tt.expected, got)
		}
	}
}

// TestComputeItemHash verifies deterministic item hashing.
func TestComputeItemHash(t *testing.T) {
	hash1 := internalfirstaction.ComputeItemHash("money", "soon", "small")
	hash2 := internalfirstaction.ComputeItemHash("money", "soon", "small")

	if hash1 != hash2 {
		t.Error("same inputs must produce same hash")
	}

	hash3 := internalfirstaction.ComputeItemHash("time", "soon", "small")
	if hash1 == hash3 {
		t.Error("different inputs must produce different hash")
	}
}

// TestStoreGetByHash verifies retrieval by hash.
func TestStoreGetByHash(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	store := persist.NewFirstActionStore(clock)
	circleID := identity.EntityID("circle-1")
	period := domainfirstaction.NewActionPeriod("2024-01-15")

	record := &domainfirstaction.ActionRecord{
		ActionHash: "action-1",
		State:      domainfirstaction.StateViewed,
		PeriodHash: period.PeriodHash,
		CircleID:   string(circleID),
	}
	_ = store.Store(record)

	// Retrieve by hash
	hash := record.Hash()
	retrieved, ok := store.GetByHash(hash)
	if !ok {
		t.Fatal("expected to find record by hash")
	}
	if retrieved.ActionHash != "action-1" {
		t.Errorf("expected action-1, got %s", retrieved.ActionHash)
	}

	// Non-existent hash
	_, ok = store.GetByHash("non-existent")
	if ok {
		t.Error("should not find non-existent hash")
	}
}

// TestStoreGetByCircle verifies retrieval by circle.
func TestStoreGetByCircle(t *testing.T) {
	clock := fixedClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	store := persist.NewFirstActionStore(clock)
	circleID1 := identity.EntityID("circle-1")
	circleID2 := identity.EntityID("circle-2")
	period := domainfirstaction.NewActionPeriod("2024-01-15")

	// Store records for different circles
	_ = store.RecordState(circleID1, "action-1", domainfirstaction.StateViewed, period.PeriodHash)
	_ = store.RecordState(circleID1, "action-2", domainfirstaction.StateDismissed, period.PeriodHash)
	_ = store.RecordState(circleID2, "action-3", domainfirstaction.StateViewed, period.PeriodHash)

	// Circle 1 should have 2 records
	records1 := store.GetByCircle(circleID1)
	if len(records1) != 2 {
		t.Errorf("expected 2 records for circle-1, got %d", len(records1))
	}

	// Circle 2 should have 1 record
	records2 := store.GetByCircle(circleID2)
	if len(records2) != 1 {
		t.Errorf("expected 1 record for circle-2, got %d", len(records2))
	}
}

// TestEligibilityHash verifies eligibility hash determinism.
func TestEligibilityHash(t *testing.T) {
	e1 := &domainfirstaction.ActionEligibility{
		CircleID:           "circle-1",
		HasGmailConnection: true,
		HasQuietBaseline:   true,
		HasMirrorViewed:    true,
		HasTrustAccrual:    true,
		HasHeldItems:       true,
		Period:             domainfirstaction.NewActionPeriod("2024-01-15"),
	}

	e2 := &domainfirstaction.ActionEligibility{
		CircleID:           "circle-1",
		HasGmailConnection: true,
		HasQuietBaseline:   true,
		HasMirrorViewed:    true,
		HasTrustAccrual:    true,
		HasHeldItems:       true,
		Period:             domainfirstaction.NewActionPeriod("2024-01-15"),
	}

	if e1.Hash() != e2.Hash() {
		t.Error("same eligibility must produce same hash")
	}

	e3 := &domainfirstaction.ActionEligibility{
		CircleID:           "circle-2",
		HasGmailConnection: true,
		HasQuietBaseline:   true,
		HasMirrorViewed:    true,
		HasTrustAccrual:    true,
		HasHeldItems:       true,
		Period:             domainfirstaction.NewActionPeriod("2024-01-15"),
	}
	if e1.Hash() == e3.Hash() {
		t.Error("different eligibility must produce different hash")
	}
}
