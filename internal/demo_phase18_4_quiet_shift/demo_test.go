package demo_phase18_4_quiet_shift

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/surface"
)

// TestDeterministicCueGeneration verifies same inputs + same clock produce identical output.
func TestDeterministicCueGeneration(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine1 := surface.NewEngine(clock)
	engine2 := surface.NewEngine(clock)

	input := surface.DefaultInput()

	cue1 := engine1.BuildCue(input)
	cue2 := engine2.BuildCue(input)

	// Same inputs + same clock = same hash
	if cue1.Hash != cue2.Hash {
		t.Errorf("cue hashes differ: %s vs %s", cue1.Hash, cue2.Hash)
	}

	// Availability should match
	if cue1.Available != cue2.Available {
		t.Errorf("availability differs: %v vs %v", cue1.Available, cue2.Available)
	}

	// Cue text should match
	if cue1.CueText != cue2.CueText {
		t.Errorf("cue text differs: %s vs %s", cue1.CueText, cue2.CueText)
	}

	t.Log("PASS: Deterministic cue generation verified")
}

// TestDeterministicPageGeneration verifies same inputs + same clock produce identical page.
func TestDeterministicPageGeneration(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine1 := surface.NewEngine(clock)
	engine2 := surface.NewEngine(clock)

	input := surface.DefaultInput()

	page1 := engine1.BuildSurfacePage(input, false)
	page2 := engine2.BuildSurfacePage(input, false)

	// Same inputs + same clock = same hash
	if page1.Hash != page2.Hash {
		t.Errorf("page hashes differ: %s vs %s", page1.Hash, page2.Hash)
	}

	// Item should match
	if page1.Item.Category != page2.Item.Category {
		t.Errorf("item category differs: %s vs %s", page1.Item.Category, page2.Item.Category)
	}

	if page1.Item.ItemKeyHash != page2.Item.ItemKeyHash {
		t.Errorf("item key hash differs: %s vs %s", page1.Item.ItemKeyHash, page2.Item.ItemKeyHash)
	}

	t.Log("PASS: Deterministic page generation verified")
}

// TestNoIdentifiersInOutput verifies output contains no identifiable information.
func TestNoIdentifiersInOutput(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	input := surface.DefaultInput()
	page := engine.BuildSurfacePage(input, true)

	// Forbidden patterns that would indicate data leakage
	forbiddenPatterns := []string{
		"@",        // Email addresses
		"$",        // Currency amounts
		"£",        // UK currency
		"€",        // Euro
		"http",     // URLs
		"amazon",   // Vendor names
		"uber",     // Vendor names
		"dpd",      // Vendor names
		"netflix",  // Vendor names
		"spotify",  // Vendor names
		"paypal",   // Vendor names
		"2025-01",  // Specific dates
		"January",  // Month names
		"Monday",   // Day names
		"10:00",    // Specific times
	}

	// Check title
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(page.Title), strings.ToLower(pattern)) {
			t.Errorf("title contains forbidden pattern '%s': %s", pattern, page.Title)
		}
	}

	// Check subtitle
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(page.Subtitle), strings.ToLower(pattern)) {
			t.Errorf("subtitle contains forbidden pattern '%s': %s", pattern, page.Subtitle)
		}
	}

	// Check reason summary
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(page.Item.ReasonSummary), strings.ToLower(pattern)) {
			t.Errorf("reason contains forbidden pattern '%s': %s", pattern, page.Item.ReasonSummary)
		}
	}

	// Check explain lines
	for _, line := range page.Item.Explain {
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(strings.ToLower(line.Text), strings.ToLower(pattern)) {
				t.Errorf("explain line contains forbidden pattern '%s': %s", pattern, line.Text)
			}
		}
	}

	// Check for date patterns (YYYY-MM-DD, DD/MM/YYYY, etc.)
	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}|\d{1,2}(st|nd|rd|th)`)
	if datePattern.MatchString(page.Item.ReasonSummary) {
		t.Errorf("reason contains date pattern: %s", page.Item.ReasonSummary)
	}

	t.Log("PASS: No identifiers in output")
}

// TestPrioritySelectionOrder verifies priority is money > time > work > people > home.
func TestPrioritySelectionOrder(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	tests := []struct {
		name             string
		heldCategories   map[surface.Category]surface.MagnitudeBucket
		expectedCategory surface.Category
	}{
		{
			name: "money_first",
			heldCategories: map[surface.Category]surface.MagnitudeBucket{
				surface.CategoryMoney:  surface.MagnitudeAFew,
				surface.CategoryTime:   surface.MagnitudeAFew,
				surface.CategoryWork:   surface.MagnitudeAFew,
				surface.CategoryPeople: surface.MagnitudeAFew,
				surface.CategoryHome:   surface.MagnitudeAFew,
			},
			expectedCategory: surface.CategoryMoney,
		},
		{
			name: "time_when_no_money",
			heldCategories: map[surface.Category]surface.MagnitudeBucket{
				surface.CategoryTime:   surface.MagnitudeSeveral,
				surface.CategoryWork:   surface.MagnitudeAFew,
				surface.CategoryPeople: surface.MagnitudeAFew,
			},
			expectedCategory: surface.CategoryTime,
		},
		{
			name: "work_when_no_money_or_time",
			heldCategories: map[surface.Category]surface.MagnitudeBucket{
				surface.CategoryWork:   surface.MagnitudeAFew,
				surface.CategoryPeople: surface.MagnitudeSeveral,
				surface.CategoryHome:   surface.MagnitudeAFew,
			},
			expectedCategory: surface.CategoryWork,
		},
		{
			name: "people_when_only_with_home",
			heldCategories: map[surface.Category]surface.MagnitudeBucket{
				surface.CategoryPeople: surface.MagnitudeAFew,
				surface.CategoryHome:   surface.MagnitudeSeveral,
			},
			expectedCategory: surface.CategoryPeople,
		},
		{
			name: "home_when_only_one",
			heldCategories: map[surface.Category]surface.MagnitudeBucket{
				surface.CategoryHome: surface.MagnitudeAFew,
			},
			expectedCategory: surface.CategoryHome,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := surface.SurfaceInput{
				HeldCategories: tt.heldCategories,
				UserPreference: "quiet",
				Now:            fixedTime,
			}
			page := engine.BuildSurfacePage(input, false)

			if page.Item.Category != tt.expectedCategory {
				t.Errorf("expected category %s, got %s", tt.expectedCategory, page.Item.Category)
			}
		})
	}

	t.Log("PASS: Priority selection order verified")
}

// TestPreferenceAffectsCueAvailability verifies quiet vs show_all affects cue.
func TestPreferenceAffectsCueAvailability(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	heldCategories := map[surface.Category]surface.MagnitudeBucket{
		surface.CategoryMoney: surface.MagnitudeAFew,
	}

	// With quiet preference, cue should be available
	quietInput := surface.SurfaceInput{
		HeldCategories: heldCategories,
		UserPreference: "quiet",
		Now:            fixedTime,
	}
	quietCue := engine.BuildCue(quietInput)
	if !quietCue.Available {
		t.Error("cue should be available with quiet preference and held items")
	}

	// With show_all preference, cue should NOT be available
	// (they'll see it elsewhere)
	showAllInput := surface.SurfaceInput{
		HeldCategories: heldCategories,
		UserPreference: "show_all",
		Now:            fixedTime,
	}
	showAllCue := engine.BuildCue(showAllInput)
	if showAllCue.Available {
		t.Error("cue should NOT be available with show_all preference")
	}

	t.Log("PASS: Preference affects cue availability")
}

// TestCueNotAvailableWhenNothingHeld verifies no cue when nothing is held.
func TestCueNotAvailableWhenNothingHeld(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	input := surface.EmptyInput()
	cue := engine.BuildCue(input)

	if cue.Available {
		t.Error("cue should not be available when nothing is held")
	}

	t.Log("PASS: Cue not available when nothing held")
}

// TestCueNotAvailableWhenOnlyNothing verifies magnitude=nothing doesn't trigger cue.
func TestCueNotAvailableWhenOnlyNothing(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	input := surface.SurfaceInput{
		HeldCategories: map[surface.Category]surface.MagnitudeBucket{
			surface.CategoryMoney: surface.MagnitudeNothing,
			surface.CategoryTime:  surface.MagnitudeNothing,
		},
		UserPreference: "quiet",
		Now:            fixedTime,
	}
	cue := engine.BuildCue(input)

	if cue.Available {
		t.Error("cue should not be available when all magnitudes are 'nothing'")
	}

	t.Log("PASS: Cue not available when only 'nothing' magnitude")
}

// TestStoreWritesHashOnly verifies store only records hashes.
func TestStoreWritesHashOnly(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	store := surface.NewActionStore(
		surface.WithStoreClock(clock),
		surface.WithMaxRecords(10),
	)

	// Record some actions
	if err := store.RecordViewed("circle1", "item-hash-123"); err != nil {
		t.Fatalf("record error: %v", err)
	}
	if err := store.RecordHeld("circle1", "item-hash-456"); err != nil {
		t.Fatalf("record error: %v", err)
	}

	records := store.Records()
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}

	// Verify records only contain hashes, not raw content
	for _, r := range records {
		if r.RecordHash == "" {
			t.Error("record hash should not be empty")
		}
		// RecordHash should be a hex string (SHA256)
		if len(r.RecordHash) != 64 {
			t.Errorf("record hash should be 64 hex chars, got %d", len(r.RecordHash))
		}
	}

	t.Log("PASS: Store writes hash only")
}

// TestStoreBoundedGrowth verifies store doesn't grow unbounded.
func TestStoreBoundedGrowth(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	maxRecords := 5
	store := surface.NewActionStore(
		surface.WithStoreClock(clock),
		surface.WithMaxRecords(maxRecords),
	)

	// Record more than max
	for i := 0; i < 20; i++ {
		if err := store.RecordViewed("", "item-"+string(rune('a'+i%26))); err != nil {
			t.Fatalf("record error: %v", err)
		}
	}

	// Store should not exceed max
	if store.Count() > maxRecords {
		t.Errorf("store exceeded max records: got %d, max %d", store.Count(), maxRecords)
	}

	t.Log("PASS: Store bounded growth verified")
}

// TestHorizonBuckets verifies horizon buckets are assigned correctly.
func TestHorizonBuckets(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	tests := []struct {
		name            string
		category        surface.Category
		suppressedFin   bool
		suppressedWork  bool
		expectedHorizon surface.HorizonBucket
	}{
		{
			name:            "money_with_suppressed_finance_is_soon",
			category:        surface.CategoryMoney,
			suppressedFin:   true,
			expectedHorizon: surface.HorizonSoon,
		},
		{
			name:            "money_without_suppressed_finance_is_this_week",
			category:        surface.CategoryMoney,
			suppressedFin:   false,
			expectedHorizon: surface.HorizonThisWeek,
		},
		{
			name:            "work_with_suppressed_work_is_this_week",
			category:        surface.CategoryWork,
			suppressedWork:  true,
			expectedHorizon: surface.HorizonThisWeek,
		},
		{
			name:            "work_without_suppressed_work_is_later",
			category:        surface.CategoryWork,
			suppressedWork:  false,
			expectedHorizon: surface.HorizonLater,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := surface.SurfaceInput{
				HeldCategories: map[surface.Category]surface.MagnitudeBucket{
					tt.category: surface.MagnitudeAFew,
				},
				UserPreference:    "quiet",
				SuppressedFinance: tt.suppressedFin,
				SuppressedWork:    tt.suppressedWork,
				Now:               fixedTime,
			}
			page := engine.BuildSurfacePage(input, false)

			if page.Item.Horizon != tt.expectedHorizon {
				t.Errorf("expected horizon %s, got %s", tt.expectedHorizon, page.Item.Horizon)
			}
		})
	}

	t.Log("PASS: Horizon buckets verified")
}

// TestExplainLinesAreAbstract verifies explain lines contain no identifiers.
func TestExplainLinesAreAbstract(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	engine := surface.NewEngine(clock)

	// Test all categories
	for _, cat := range surface.CategoryPriority {
		input := surface.SurfaceInput{
			HeldCategories: map[surface.Category]surface.MagnitudeBucket{
				cat: surface.MagnitudeAFew,
			},
			UserPreference: "quiet",
			Now:            fixedTime,
		}
		page := engine.BuildSurfacePage(input, true)

		if len(page.Item.Explain) == 0 {
			t.Errorf("category %s should have explain lines", cat)
			continue
		}

		for _, line := range page.Item.Explain {
			// Ensure lines are generic, not specific
			if strings.Contains(line.Text, "$") || strings.Contains(line.Text, "£") {
				t.Errorf("explain line contains currency: %s", line.Text)
			}
			if strings.Contains(line.Text, "@") {
				t.Errorf("explain line contains email marker: %s", line.Text)
			}
		}
	}

	t.Log("PASS: Explain lines are abstract")
}

// TestInputHashDeterminism verifies input hash is stable.
func TestInputHashDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	input1 := surface.SurfaceInput{
		HeldCategories: map[surface.Category]surface.MagnitudeBucket{
			surface.CategoryMoney: surface.MagnitudeAFew,
		},
		UserPreference:    "quiet",
		SuppressedFinance: true,
		Now:               fixedTime,
	}

	input2 := surface.SurfaceInput{
		HeldCategories: map[surface.Category]surface.MagnitudeBucket{
			surface.CategoryMoney: surface.MagnitudeAFew,
		},
		UserPreference:    "quiet",
		SuppressedFinance: true,
		Now:               fixedTime,
	}

	if input1.Hash() != input2.Hash() {
		t.Errorf("same inputs should produce same hash: %s vs %s", input1.Hash(), input2.Hash())
	}

	// Different input should produce different hash
	input3 := surface.SurfaceInput{
		HeldCategories: map[surface.Category]surface.MagnitudeBucket{
			surface.CategoryMoney: surface.MagnitudeSeveral, // Changed
		},
		UserPreference:    "quiet",
		SuppressedFinance: true,
		Now:               fixedTime,
	}

	if input1.Hash() == input3.Hash() {
		t.Error("different inputs should produce different hash")
	}

	t.Log("PASS: Input hash is deterministic")
}
