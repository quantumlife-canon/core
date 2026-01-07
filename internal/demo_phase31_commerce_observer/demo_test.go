// Package demo_phase31_commerce_observer contains tests for Phase 31: Commerce Observers.
//
// Commerce Observers are NOT finance. They are NOT budgeting. They are NOT insights.
// They are long-horizon behavioral signals that MAY matter someday, but usually do not.
//
// Default outcome: NOTHING SHOWN. Commerce is observed. Nothing else.
//
// Reference: docs/ADR/ADR-0062-phase31-commerce-observers.md
package demo_phase31_commerce_observer

import (
	"testing"
	"time"

	internalcommerceobserver "quantumlife/internal/commerceobserver"
	"quantumlife/internal/persist"
	domaincommerceobserver "quantumlife/pkg/domain/commerceobserver"
)

// TestDeterminism verifies that same inputs + same clock = same output.
func TestDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := internalcommerceobserver.NewEngine(clock)

	inputs := &domaincommerceobserver.CommerceInputs{
		CircleID: "circle_test",
		Period:   "2025-W03",
		CategoryCounts: map[domaincommerceobserver.CategoryBucket]int{
			domaincommerceobserver.CategoryFoodDelivery: 5,
			domaincommerceobserver.CategoryTransport:    3,
		},
		CategoryTrends: map[domaincommerceobserver.CategoryBucket]string{
			domaincommerceobserver.CategoryFoodDelivery: "stable",
			domaincommerceobserver.CategoryTransport:    "increasing",
		},
	}

	// First run
	obs1 := engine.Observe(inputs)
	page1 := engine.BuildMirrorPage(obs1)

	// Second run with same inputs
	obs2 := engine.Observe(inputs)
	page2 := engine.BuildMirrorPage(obs2)

	// Verify determinism
	if len(obs1) != len(obs2) {
		t.Errorf("Observation count differs: %d vs %d", len(obs1), len(obs2))
	}

	for i := range obs1 {
		if obs1[i].ComputeHash() != obs2[i].ComputeHash() {
			t.Errorf("Observation %d hash differs", i)
		}
	}

	if page1 == nil || page2 == nil {
		t.Fatal("Expected pages to be non-nil")
	}

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("Page hash differs: %s vs %s", page1.StatusHash, page2.StatusHash)
	}
}

// TestSilenceDefault verifies that no observations = nil mirror page.
func TestSilenceDefault(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := internalcommerceobserver.NewEngine(clock)

	// Empty inputs
	inputs := &domaincommerceobserver.CommerceInputs{
		CircleID:       "circle_test",
		Period:         "2025-W03",
		CategoryCounts: map[domaincommerceobserver.CategoryBucket]int{},
	}

	observations := engine.Observe(inputs)
	if len(observations) != 0 {
		t.Errorf("Expected 0 observations, got %d", len(observations))
	}

	page := engine.BuildMirrorPage(observations)
	if page != nil {
		t.Error("Expected nil page for empty observations (silence is success)")
	}
}

// TestNoMirrorWhenNoData verifies that nil inputs = no page rendered.
func TestNoMirrorWhenNoData(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := internalcommerceobserver.NewEngine(clock)

	observations := engine.Observe(nil)
	if observations != nil {
		t.Errorf("Expected nil observations for nil input, got %v", observations)
	}

	page := engine.BuildMirrorPage(nil)
	if page != nil {
		t.Error("Expected nil page for nil observations")
	}
}

// TestHashOnlyStorage verifies that no raw data is persisted.
func TestHashOnlyStorage(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	store := persist.NewCommerceObserverStore(clock)

	obs := &domaincommerceobserver.CommerceObservation{
		Category:     domaincommerceobserver.CategoryFoodDelivery,
		Frequency:    domaincommerceobserver.FrequencyOccasional,
		Stability:    domaincommerceobserver.StabilityStable,
		Period:       "2025-W03",
		EvidenceHash: "abc123",
	}

	err := store.PersistObservation("circle_test", obs)
	if err != nil {
		t.Fatalf("Failed to persist observation: %v", err)
	}

	// Verify we can retrieve observations
	retrieved := store.GetObservationsForPeriod("circle_test", "2025-W03")
	if len(retrieved) != 1 {
		t.Fatalf("Expected 1 observation, got %d", len(retrieved))
	}

	// Verify only abstract data is stored
	if retrieved[0].Category != obs.Category {
		t.Errorf("Category mismatch: %s vs %s", retrieved[0].Category, obs.Category)
	}
	if retrieved[0].Frequency != obs.Frequency {
		t.Errorf("Frequency mismatch: %s vs %s", retrieved[0].Frequency, obs.Frequency)
	}
	if retrieved[0].EvidenceHash != obs.EvidenceHash {
		t.Errorf("EvidenceHash mismatch: %s vs %s", retrieved[0].EvidenceHash, obs.EvidenceHash)
	}
}

// TestCategoryCaps verifies that max 3 categories are shown.
func TestCategoryCaps(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := internalcommerceobserver.NewEngine(clock)

	// Create observations for 5 categories
	inputs := &domaincommerceobserver.CommerceInputs{
		CircleID: "circle_test",
		Period:   "2025-W03",
		CategoryCounts: map[domaincommerceobserver.CategoryBucket]int{
			domaincommerceobserver.CategoryFoodDelivery:  5,
			domaincommerceobserver.CategoryTransport:     3,
			domaincommerceobserver.CategoryRetail:        4,
			domaincommerceobserver.CategorySubscriptions: 2,
			domaincommerceobserver.CategoryUtilities:     1,
		},
	}

	observations := engine.Observe(inputs)
	page := engine.BuildMirrorPage(observations)

	if page == nil {
		t.Fatal("Expected non-nil page")
	}

	if len(page.Buckets) > domaincommerceobserver.MaxBuckets {
		t.Errorf("Expected max %d buckets, got %d", domaincommerceobserver.MaxBuckets, len(page.Buckets))
	}
}

// TestSingleWhisperRule verifies that cue is hidden when other cue is active.
func TestSingleWhisperRule(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := internalcommerceobserver.NewEngine(clock)

	observations := []domaincommerceobserver.CommerceObservation{
		{
			Category:     domaincommerceobserver.CategoryFoodDelivery,
			Frequency:    domaincommerceobserver.FrequencyOccasional,
			Stability:    domaincommerceobserver.StabilityStable,
			Period:       "2025-W03",
			EvidenceHash: "abc123",
		},
	}

	// With connection and observations, but no other cue active
	shouldShow := engine.ShouldShowCommerceCue(true, observations, false)
	if !shouldShow {
		t.Error("Expected cue to show when connected, has observations, no other cue active")
	}

	// With other cue active - should NOT show (single whisper rule)
	shouldShow = engine.ShouldShowCommerceCue(true, observations, true)
	if shouldShow {
		t.Error("Expected cue to NOT show when other cue is active (single whisper rule)")
	}

	// Without connection - should NOT show
	shouldShow = engine.ShouldShowCommerceCue(false, observations, false)
	if shouldShow {
		t.Error("Expected cue to NOT show when not connected")
	}

	// Without observations - should NOT show
	shouldShow = engine.ShouldShowCommerceCue(true, nil, false)
	if shouldShow {
		t.Error("Expected cue to NOT show when no observations")
	}
}

// TestBucketConversion verifies that raw counts are converted to buckets.
func TestBucketConversion(t *testing.T) {
	tests := []struct {
		count    int
		expected domaincommerceobserver.FrequencyBucket
	}{
		{0, domaincommerceobserver.FrequencyRare},
		{1, domaincommerceobserver.FrequencyRare},
		{2, domaincommerceobserver.FrequencyOccasional},
		{5, domaincommerceobserver.FrequencyOccasional},
		{8, domaincommerceobserver.FrequencyOccasional},
		{9, domaincommerceobserver.FrequencyFrequent},
		{100, domaincommerceobserver.FrequencyFrequent},
	}

	for _, tt := range tests {
		bucket := domaincommerceobserver.ToFrequencyBucket(tt.count)
		if bucket != tt.expected {
			t.Errorf("ToFrequencyBucket(%d) = %s, expected %s", tt.count, bucket, tt.expected)
		}
	}
}

// TestCanonicalStrings verifies that pipe-delimited format is used.
func TestCanonicalStrings(t *testing.T) {
	obs := &domaincommerceobserver.CommerceObservation{
		Category:     domaincommerceobserver.CategoryFoodDelivery,
		Frequency:    domaincommerceobserver.FrequencyOccasional,
		Stability:    domaincommerceobserver.StabilityStable,
		Period:       "2025-W03",
		EvidenceHash: "abc123",
	}

	canonical := obs.CanonicalString()

	// Should be pipe-delimited
	if canonical[0:12] != "COMMERCE_OBS" {
		t.Errorf("Canonical string should start with COMMERCE_OBS, got: %s", canonical[:20])
	}

	// Should contain version
	if !containsSubstring(canonical, "|v1|") {
		t.Error("Canonical string should contain version |v1|")
	}

	// Should contain all fields
	if !containsSubstring(canonical, "food_delivery") {
		t.Error("Canonical string should contain category")
	}
	if !containsSubstring(canonical, "occasional") {
		t.Error("Canonical string should contain frequency")
	}
	if !containsSubstring(canonical, "stable") {
		t.Error("Canonical string should contain stability")
	}
}

// TestBoundedRetention verifies that 30-day eviction works.
func TestBoundedRetention(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	store := persist.NewCommerceObserverStore(clock)

	// Add observations for 35 periods
	for i := 0; i < 35; i++ {
		period := formatPeriod(i)
		obs := &domaincommerceobserver.CommerceObservation{
			Category:     domaincommerceobserver.CategoryFoodDelivery,
			Frequency:    domaincommerceobserver.FrequencyOccasional,
			Stability:    domaincommerceobserver.StabilityStable,
			Period:       period,
			EvidenceHash: "hash" + period,
		}
		err := store.PersistObservation("circle_test", obs)
		if err != nil {
			t.Fatalf("Failed to persist observation %d: %v", i, err)
		}
	}

	// Trigger eviction
	store.ExpireOldObservations()

	// Verify count is bounded
	count := store.Count()
	if count > 30 {
		t.Errorf("Expected at most 30 observations after eviction, got %d", count)
	}
}

// TestValidation verifies that validation works correctly.
func TestValidation(t *testing.T) {
	// Valid observation
	obs := &domaincommerceobserver.CommerceObservation{
		Category:     domaincommerceobserver.CategoryFoodDelivery,
		Frequency:    domaincommerceobserver.FrequencyOccasional,
		Stability:    domaincommerceobserver.StabilityStable,
		Period:       "2025-W03",
		EvidenceHash: "abc123",
	}
	if err := obs.Validate(); err != nil {
		t.Errorf("Valid observation should pass validation: %v", err)
	}

	// Invalid category
	obs.Category = "invalid"
	if err := obs.Validate(); err == nil {
		t.Error("Invalid category should fail validation")
	}
}

// Helper functions

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func formatPeriod(i int) string {
	week := (i % 52) + 1
	year := 2025 - (i / 52)
	weekStr := ""
	if week < 10 {
		weekStr = "0" + string(rune('0'+week))
	} else {
		weekStr = string(rune('0'+week/10)) + string(rune('0'+week%10))
	}
	return string(rune('0'+year/1000%10)) + string(rune('0'+year/100%10)) +
		string(rune('0'+year/10%10)) + string(rune('0'+year%10)) + "-W" + weekStr
}
