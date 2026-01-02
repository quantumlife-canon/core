package demo_phase18_7_mirror

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/mirror"
	"quantumlife/pkg/domain/connection"
	domainmirror "quantumlife/pkg/domain/mirror"
)

// ═══════════════════════════════════════════════════════════════════════════
// Phase 18.7: Mirror Proof - Demo Tests
// Reference: docs/ADR/ADR-0039-phase18-7-mirror-proof.md
//
// CRITICAL: All tests must be deterministic.
// CRITICAL: No time.Now() - use fixed timestamps.
// CRITICAL: Abstract only - no identifiers in output.
// ═══════════════════════════════════════════════════════════════════════════

// fixedTime provides deterministic timestamps for testing.
var fixedTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

// fixedClock returns a clock function that always returns fixedTime.
func fixedClock() func() time.Time {
	return func() time.Time { return fixedTime }
}

// TestDeterministicMirrorHash verifies that same input produces same hash.
func TestDeterministicMirrorHash(t *testing.T) {
	engine := mirror.NewEngine(fixedClock())

	input := mirror.DefaultInput()

	// Build mirror page twice
	page1 := engine.BuildMirrorPage(input)
	page2 := engine.BuildMirrorPage(input)

	if page1.Hash != page2.Hash {
		t.Errorf("Same input produced different hashes:\n  page1: %s\n  page2: %s", page1.Hash, page2.Hash)
	}
}

// TestAbstractOnlyEnforcement verifies no identifiers leak into output.
func TestAbstractOnlyEnforcement(t *testing.T) {
	engine := mirror.NewEngine(fixedClock())

	input := mirror.DefaultInput()
	page := engine.BuildMirrorPage(input)

	// Check title is abstract
	if page.Title == "" {
		t.Error("Title should not be empty")
	}

	// Check subtitle is abstract
	if page.Subtitle == "" {
		t.Error("Subtitle should not be empty")
	}

	// Forbidden patterns that should never appear in user-facing content
	// Note: timestamps in canonical strings are acceptable for determinism
	forbidden := []string{
		"@", // email addresses
		"$", // amounts
		"http", // URLs
		// Month and day names indicate human-readable dates
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
		"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday",
	}

	// Check canonical string for forbidden patterns
	canonical := page.CanonicalString()
	for _, pattern := range forbidden {
		if strings.Contains(canonical, pattern) {
			t.Errorf("Canonical string contains forbidden pattern '%s': %s", pattern, canonical[:min(100, len(canonical))])
		}
	}

	// Check each source summary
	for _, src := range page.Sources {
		srcCanonical := src.CanonicalString()
		for _, pattern := range forbidden {
			if strings.Contains(srcCanonical, pattern) {
				t.Errorf("Source %s contains forbidden pattern '%s'", src.Kind, pattern)
			}
		}
	}
}

// TestNoIdentifiersLeakage verifies that specific values never appear.
func TestNoIdentifiersLeakage(t *testing.T) {
	engine := mirror.NewEngine(fixedClock())

	input := mirror.DefaultInput()
	page := engine.BuildMirrorPage(input)

	// NotStored should be generic categories, not specifics
	for _, src := range page.Sources {
		for _, notStored := range src.NotStored {
			// Should be generic like "messages" not "email from john@example.com"
			if strings.Contains(notStored, "@") {
				t.Errorf("NotStored contains email address: %s", notStored)
			}
			if strings.Contains(notStored, "$") {
				t.Errorf("NotStored contains amount: %s", notStored)
			}
		}
	}

	// Observed should use magnitude buckets, not raw counts
	for _, src := range page.Sources {
		for _, obs := range src.Observed {
			// Magnitude should be a bucket
			switch obs.Magnitude {
			case domainmirror.MagnitudeNone, domainmirror.MagnitudeAFew, domainmirror.MagnitudeSeveral:
				// OK
			default:
				t.Errorf("Invalid magnitude bucket: %s", obs.Magnitude)
			}

			// Horizon should be a bucket
			switch obs.Horizon {
			case domainmirror.HorizonRecent, domainmirror.HorizonOngoing, domainmirror.HorizonEarlier:
				// OK
			default:
				t.Errorf("Invalid horizon bucket: %s", obs.Horizon)
			}
		}
	}
}

// TestAckSuppressesRepeatCue verifies that ack prevents repeat mirror.
func TestAckSuppressesRepeatCue(t *testing.T) {
	store := mirror.NewAckStore(128)

	mirrorHash := "test-hash-12345"

	// Before ack, should not be recent
	if store.HasRecent(mirrorHash) {
		t.Error("Hash should not be recent before ack")
	}

	// Record ack
	if err := store.Record(domainmirror.AckViewed, mirrorHash, fixedTime); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// After ack, should be recent
	if !store.HasRecent(mirrorHash) {
		t.Error("Hash should be recent after ack")
	}
}

// TestNoMirrorWithoutConnections verifies empty output when no connections.
func TestNoMirrorWithoutConnections(t *testing.T) {
	engine := mirror.NewEngine(fixedClock())

	input := mirror.EmptyInput()

	// Should have no connected sources
	if engine.HasConnectedSources(input) {
		t.Error("Empty input should have no connected sources")
	}
}

// TestIdenticalReplayOutput verifies that replaying produces same output.
func TestIdenticalReplayOutput(t *testing.T) {
	// Create first engine and generate
	engine1 := mirror.NewEngine(fixedClock())
	input := mirror.DefaultInput()
	page1 := engine1.BuildMirrorPage(input)

	// Create second engine and generate with same input
	engine2 := mirror.NewEngine(fixedClock())
	page2 := engine2.BuildMirrorPage(input)

	// Hashes should match
	if page1.Hash != page2.Hash {
		t.Errorf("Replay produced different hash:\n  page1: %s\n  page2: %s", page1.Hash, page2.Hash)
	}

	// Canonical strings should match
	if page1.CanonicalString() != page2.CanonicalString() {
		t.Error("Replay produced different canonical string")
	}
}

// TestMagnitudeBucketing verifies count to magnitude conversion.
func TestMagnitudeBucketing(t *testing.T) {
	tests := []struct {
		count    int
		expected domainmirror.MagnitudeBucket
	}{
		{0, domainmirror.MagnitudeNone},
		{1, domainmirror.MagnitudeAFew},
		{2, domainmirror.MagnitudeAFew},
		{3, domainmirror.MagnitudeAFew},
		{4, domainmirror.MagnitudeSeveral},
		{10, domainmirror.MagnitudeSeveral},
		{100, domainmirror.MagnitudeSeveral},
	}

	for _, tt := range tests {
		got := domainmirror.BucketCount(tt.count)
		if got != tt.expected {
			t.Errorf("BucketCount(%d) = %s, want %s", tt.count, got, tt.expected)
		}
	}
}

// TestSourceSummaryCanonicalString verifies canonical string format.
func TestSourceSummaryCanonicalString(t *testing.T) {
	summary := domainmirror.MirrorSourceSummary{
		Kind:             connection.KindEmail,
		ReadSuccessfully: true,
		NotStored:        []string{"messages", "senders"},
		Observed: []domainmirror.ObservedItem{
			{Category: domainmirror.ObservedReceipts, Magnitude: domainmirror.MagnitudeAFew, Horizon: domainmirror.HorizonRecent},
		},
	}

	canonical := summary.CanonicalString()

	// Must start with MIRROR_SRC|v1
	if !strings.HasPrefix(canonical, "MIRROR_SRC|v1|") {
		t.Errorf("Canonical string doesn't start with 'MIRROR_SRC|v1|': %s", canonical[:min(30, len(canonical))])
	}

	// Must not contain JSON markers
	if containsJSONMarker(canonical) {
		t.Error("Canonical string contains JSON markers")
	}
}

// TestMirrorPageCanonicalString verifies page canonical string format.
func TestMirrorPageCanonicalString(t *testing.T) {
	page := domainmirror.MirrorPage{
		Title:              "Seen, quietly.",
		Subtitle:           "Test subtitle",
		Sources:            []domainmirror.MirrorSourceSummary{},
		Outcome:            domainmirror.MirrorOutcome{HeldQuietly: true, HeldMagnitude: domainmirror.MagnitudeAFew},
		RestraintStatement: "We chose not to interrupt you.",
		RestraintWhy:       "Quiet is a feature.",
		GeneratedAt:        fixedTime,
	}

	canonical := page.CanonicalString()

	// Must start with MIRROR_PAGE|v1
	if !strings.HasPrefix(canonical, "MIRROR_PAGE|v1|") {
		t.Errorf("Canonical string doesn't start with 'MIRROR_PAGE|v1|': %s", canonical[:min(30, len(canonical))])
	}

	// Must not contain JSON markers
	if containsJSONMarker(canonical) {
		t.Error("Canonical string contains JSON markers")
	}
}

// TestObservedItemCanonicalString verifies item canonical string format.
func TestObservedItemCanonicalString(t *testing.T) {
	item := domainmirror.ObservedItem{
		Category:  domainmirror.ObservedReceipts,
		Magnitude: domainmirror.MagnitudeAFew,
		Horizon:   domainmirror.HorizonRecent,
	}

	canonical := item.CanonicalString()

	// Must start with OBS_ITEM|v1
	if !strings.HasPrefix(canonical, "OBS_ITEM|v1|") {
		t.Errorf("Canonical string doesn't start with 'OBS_ITEM|v1|': %s", canonical)
	}

	// Must use pipe delimiter
	if !strings.Contains(canonical, "|") {
		t.Error("Canonical string doesn't use pipe delimiter")
	}
}

// TestMirrorAckCanonicalString verifies ack canonical string format.
func TestMirrorAckCanonicalString(t *testing.T) {
	ack := domainmirror.MirrorAck{
		PageHash: "test-hash",
		Action:   domainmirror.AckViewed,
		At:       fixedTime,
	}

	canonical := ack.CanonicalString()

	// Must start with MIRROR_ACK|v1
	if !strings.HasPrefix(canonical, "MIRROR_ACK|v1|") {
		t.Errorf("Canonical string doesn't start with 'MIRROR_ACK|v1|': %s", canonical)
	}
}

// TestAckStoreBounding verifies that store respects max records limit.
func TestAckStoreBounding(t *testing.T) {
	maxRecords := 5
	store := mirror.NewAckStore(maxRecords)

	// Add more than max records
	for i := 0; i < maxRecords+3; i++ {
		hash := string(rune('a'+i)) + "-hash"
		store.Record(domainmirror.AckViewed, hash, fixedTime.Add(time.Duration(i)*time.Minute))
	}

	// Store length should not exceed max
	if store.Len() > maxRecords {
		t.Errorf("Store length %d exceeds max %d", store.Len(), maxRecords)
	}
}

// TestOutcomeCanonicalString verifies outcome canonical string format.
func TestOutcomeCanonicalString(t *testing.T) {
	outcome := domainmirror.MirrorOutcome{
		HeldQuietly:              true,
		HeldMagnitude:            domainmirror.MagnitudeAFew,
		NothingRequiresAttention: true,
	}

	canonical := outcome.CanonicalString()

	// Must start with MIRROR_OUT|v1
	if !strings.HasPrefix(canonical, "MIRROR_OUT|v1|") {
		t.Errorf("Canonical string doesn't start with 'MIRROR_OUT|v1|': %s", canonical)
	}
}

// TestDisplayTextMethods verifies that display text methods return values.
func TestDisplayTextMethods(t *testing.T) {
	// Magnitude display text
	magnitudes := []domainmirror.MagnitudeBucket{
		domainmirror.MagnitudeNone,
		domainmirror.MagnitudeAFew,
		domainmirror.MagnitudeSeveral,
	}
	for _, m := range magnitudes {
		text := m.DisplayText()
		if text == "" {
			t.Errorf("Magnitude %s has empty display text", m)
		}
	}

	// Horizon display text
	horizons := []domainmirror.HorizonBucket{
		domainmirror.HorizonRecent,
		domainmirror.HorizonOngoing,
		domainmirror.HorizonEarlier,
	}
	for _, h := range horizons {
		text := h.DisplayText()
		if text == "" {
			t.Errorf("Horizon %s has empty display text", h)
		}
	}

	// Observed category display text
	categories := domainmirror.AllObservedCategories()
	for _, c := range categories {
		text := c.DisplayText()
		if text == "" {
			t.Errorf("Category %s has empty display text", c)
		}
	}
}

// TestRestraintStatementPresent verifies restraint messaging is present.
func TestRestraintStatementPresent(t *testing.T) {
	engine := mirror.NewEngine(fixedClock())

	input := mirror.DefaultInput()
	page := engine.BuildMirrorPage(input)

	if page.RestraintStatement == "" {
		t.Error("Restraint statement should not be empty")
	}

	if page.RestraintWhy == "" {
		t.Error("Restraint why line should not be empty")
	}
}

func containsJSONMarker(s string) bool {
	markers := []string{"{", "}", "\":"}
	for _, marker := range markers {
		if strings.Contains(s, marker) {
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
