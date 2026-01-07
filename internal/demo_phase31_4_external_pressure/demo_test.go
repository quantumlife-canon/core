// Package demo_phase31_4_external_pressure provides demo tests for Phase 31.4.
//
// Phase 31.4: External Pressure Circles + Intersection Pressure Map
// Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
//
// CRITICAL INVARIANTS:
//   - NO raw merchant strings, NO vendor identifiers, NO amounts, NO timestamps
//   - Only category hints, magnitude buckets, and horizon buckets
//   - Derived circles CANNOT approve, CANNOT execute, CANNOT receive drafts
//   - Hash-only persistence; deterministic: same inputs => same hashes
//   - No goroutines. No time.Now() - clock injection only.
//   - stdlib only.
package demo_phase31_4_external_pressure

import (
	"strings"
	"testing"
	"time"

	internalexternalpressure "quantumlife/internal/externalpressure"
	"quantumlife/internal/persist"
	domainexternalpressure "quantumlife/pkg/domain/externalpressure"
)

// Fixed clock for deterministic testing.
func fixedClock() time.Time {
	return time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)
}

// TestDeterminism_SameInputsSameHash verifies deterministic output.
func TestDeterminism_SameInputsSameHash(t *testing.T) {
	engine := internalexternalpressure.NewEngine(fixedClock)

	inputs := &domainexternalpressure.PressureInputs{
		SovereignCircleIDHash: "sov-hash-123",
		PeriodKey:             "2026-01-07",
		Observations: []domainexternalpressure.ObservationInput{
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategoryDelivery, EvidenceHash: "ev-001"},
			{Source: domainexternalpressure.SourceFinanceTrueLayer, Category: domainexternalpressure.PressureCategoryTransport, EvidenceHash: "ev-002"},
		},
	}

	// Compute twice
	snapshot1 := engine.ComputePressureMap(inputs)
	snapshot2 := engine.ComputePressureMap(inputs)

	if snapshot1 == nil || snapshot2 == nil {
		t.Fatal("Expected non-nil snapshots")
	}

	if snapshot1.StatusHash != snapshot2.StatusHash {
		t.Errorf("Determinism failed: hash1=%s, hash2=%s", snapshot1.StatusHash, snapshot2.StatusHash)
	}
}

// TestMaxItemsEnforced verifies max 3 pressure items.
func TestMaxItemsEnforced(t *testing.T) {
	engine := internalexternalpressure.NewEngine(fixedClock)

	// Create observations for all 5 categories
	inputs := &domainexternalpressure.PressureInputs{
		SovereignCircleIDHash: "sov-hash-456",
		PeriodKey:             "2026-01-07",
		Observations: []domainexternalpressure.ObservationInput{
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategoryDelivery, EvidenceHash: "ev-1"},
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategoryTransport, EvidenceHash: "ev-2"},
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategoryRetail, EvidenceHash: "ev-3"},
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategorySubscription, EvidenceHash: "ev-4"},
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategoryOther, EvidenceHash: "ev-5"},
		},
	}

	snapshot := engine.ComputePressureMap(inputs)
	if snapshot == nil {
		t.Fatal("Expected non-nil snapshot")
	}

	if len(snapshot.Items) > domainexternalpressure.MaxPressureItems {
		t.Errorf("Max items exceeded: got %d, max %d", len(snapshot.Items), domainexternalpressure.MaxPressureItems)
	}
}

// TestNoForbiddenTokensInCanonicalStrings verifies no merchant strings leak.
func TestNoForbiddenTokensInCanonicalStrings(t *testing.T) {
	engine := internalexternalpressure.NewEngine(fixedClock)

	inputs := &domainexternalpressure.PressureInputs{
		SovereignCircleIDHash: "sov-hash-789",
		PeriodKey:             "2026-01-07",
		Observations: []domainexternalpressure.ObservationInput{
			{Source: domainexternalpressure.SourceGmailReceipt, Category: domainexternalpressure.PressureCategoryDelivery, EvidenceHash: "ev-001"},
		},
	}

	snapshot := engine.ComputePressureMap(inputs)
	if snapshot == nil {
		t.Fatal("Expected non-nil snapshot")
	}

	canonical := snapshot.CanonicalString()

	// Check for forbidden patterns
	forbidden := []string{
		"deliveroo", "uber", "amazon", "netflix", "spotify",
		"@", "http://", "https://", "£", "$", "€",
	}

	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(canonical), f) {
			t.Errorf("Forbidden token found in canonical string: %s", f)
		}
	}
}

// TestForbiddenPatternValidation verifies guard against merchant strings.
func TestForbiddenPatternValidation(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected bool // true = forbidden detected
	}{
		{"clean values", []string{"delivery", "transport", "retail"}, false},
		{"merchant name", []string{"deliveroo"}, true},
		{"email pattern", []string{"user@example.com"}, true},
		{"url pattern", []string{"https://example.com"}, true},
		{"amount pattern", []string{"£50.00"}, true},
		{"mixed case merchant", []string{"Amazon"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := internalexternalpressure.ValidateForbiddenPatterns(tt.values...)
			if result != tt.expected {
				t.Errorf("expected forbidden=%v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExternalCircleIDDerivation verifies stable circle ID derivation.
func TestExternalCircleIDDerivation(t *testing.T) {
	// Derive the same circle ID twice
	id1 := domainexternalpressure.ComputeExternalCircleID(
		domainexternalpressure.SourceGmailReceipt,
		domainexternalpressure.PressureCategoryDelivery,
		"sov-hash-abc",
	)

	id2 := domainexternalpressure.ComputeExternalCircleID(
		domainexternalpressure.SourceGmailReceipt,
		domainexternalpressure.PressureCategoryDelivery,
		"sov-hash-abc",
	)

	if id1 != id2 {
		t.Errorf("External circle ID not stable: %s != %s", id1, id2)
	}

	// Different inputs should produce different IDs
	id3 := domainexternalpressure.ComputeExternalCircleID(
		domainexternalpressure.SourceFinanceTrueLayer,
		domainexternalpressure.PressureCategoryDelivery,
		"sov-hash-abc",
	)

	if id1 == id3 {
		t.Errorf("Different inputs should produce different IDs: %s == %s", id1, id3)
	}
}

// TestMagnitudeBuckets verifies magnitude bucket conversion.
func TestMagnitudeBuckets(t *testing.T) {
	tests := []struct {
		count    int
		expected domainexternalpressure.PressureMagnitude
	}{
		{0, domainexternalpressure.PressureMagnitudeNothing},
		{1, domainexternalpressure.PressureMagnitudeAFew},
		{2, domainexternalpressure.PressureMagnitudeAFew},
		{3, domainexternalpressure.PressureMagnitudeAFew},
		{4, domainexternalpressure.PressureMagnitudeSeveral},
		{10, domainexternalpressure.PressureMagnitudeSeveral},
	}

	for _, tt := range tests {
		result := domainexternalpressure.ToPressureMagnitude(tt.count)
		if result != tt.expected {
			t.Errorf("count=%d: expected %s, got %s", tt.count, tt.expected, result)
		}
	}
}

// TestCategoryMapping verifies commerce category to pressure category mapping.
func TestCategoryMapping(t *testing.T) {
	tests := []struct {
		commerce string
		expected domainexternalpressure.PressureCategory
	}{
		{"food_delivery", domainexternalpressure.PressureCategoryDelivery},
		{"transport", domainexternalpressure.PressureCategoryTransport},
		{"retail", domainexternalpressure.PressureCategoryRetail},
		{"subscriptions", domainexternalpressure.PressureCategorySubscription},
		{"unknown", domainexternalpressure.PressureCategoryOther},
	}

	for _, tt := range tests {
		result := domainexternalpressure.MapCommerceCategoryToPressure(tt.commerce)
		if result != tt.expected {
			t.Errorf("commerce=%s: expected %s, got %s", tt.commerce, tt.expected, result)
		}
	}
}

// TestPressureMapStoreIdempotency verifies replay idempotency.
func TestPressureMapStoreIdempotency(t *testing.T) {
	store := persist.NewPressureMapStore(fixedClock)

	snapshot := &domainexternalpressure.PressureMapSnapshot{
		SovereignCircleIDHash: "sov-hash-test",
		PeriodKey:             "2026-01-07",
		Items: []domainexternalpressure.PressureItem{
			{
				Category:           domainexternalpressure.PressureCategoryDelivery,
				Magnitude:          domainexternalpressure.PressureMagnitudeAFew,
				Horizon:            domainexternalpressure.PressureHorizonUnknown,
				SourceKindsPresent: []domainexternalpressure.SourceKind{domainexternalpressure.SourceGmailReceipt},
				EvidenceHash:       "ev-hash-123",
			},
		},
		StatusHash: "status-hash-abc",
	}

	// Persist twice
	err1 := store.PersistSnapshot(snapshot)
	err2 := store.PersistSnapshot(snapshot)

	if err1 != nil || err2 != nil {
		t.Errorf("Persist failed: err1=%v, err2=%v", err1, err2)
	}

	// Should only have one snapshot
	if store.Count() != 1 {
		t.Errorf("Expected 1 snapshot, got %d", store.Count())
	}
}

// TestExternalCircleStoreIdempotency verifies circle store idempotency.
func TestExternalCircleStoreIdempotency(t *testing.T) {
	store := persist.NewExternalCircleStore(fixedClock)

	circle := &domainexternalpressure.ExternalDerivedCircle{
		CircleIDHash:  "ext-circle-hash",
		SourceKind:    domainexternalpressure.SourceGmailReceipt,
		CategoryHint:  domainexternalpressure.PressureCategoryDelivery,
		CreatedPeriod: "2026-01-07",
		EvidenceHash:  "ev-hash-xyz",
	}

	sovHash := "sov-hash-test"

	// Persist twice
	err1 := store.PersistCircle(sovHash, circle)
	err2 := store.PersistCircle(sovHash, circle)

	if err1 != nil || err2 != nil {
		t.Errorf("Persist failed: err1=%v, err2=%v", err1, err2)
	}

	// Should only have one circle
	if store.Count() != 1 {
		t.Errorf("Expected 1 circle, got %d", store.Count())
	}
}

// TestPressureProofPage verifies proof page construction.
func TestPressureProofPage(t *testing.T) {
	snapshot := &domainexternalpressure.PressureMapSnapshot{
		SovereignCircleIDHash: "sov-hash-page",
		PeriodKey:             "2026-01-07",
		Items: []domainexternalpressure.PressureItem{
			{
				Category:           domainexternalpressure.PressureCategoryDelivery,
				Magnitude:          domainexternalpressure.PressureMagnitudeSeveral,
				Horizon:            domainexternalpressure.PressureHorizonUnknown,
				SourceKindsPresent: []domainexternalpressure.SourceKind{domainexternalpressure.SourceGmailReceipt, domainexternalpressure.SourceFinanceTrueLayer},
				EvidenceHash:       "ev-hash-page",
			},
		},
		StatusHash: "status-hash-page",
	}

	page := domainexternalpressure.NewPressureProofPage(snapshot)
	if page == nil {
		t.Fatal("Expected non-nil page")
	}

	if page.Title != domainexternalpressure.DefaultPressureTitle {
		t.Errorf("Wrong title: %s", page.Title)
	}

	if len(page.CategoryChips) != 1 {
		t.Errorf("Expected 1 category chip, got %d", len(page.CategoryChips))
	}

	if page.MagnitudeText != "several" {
		t.Errorf("Wrong magnitude text: %s", page.MagnitudeText)
	}

	// Check sources text contains abstract terms
	if !strings.Contains(page.SourcesText, "email") || !strings.Contains(page.SourcesText, "bank") {
		t.Errorf("Sources text should contain 'email' and 'bank': %s", page.SourcesText)
	}
}

// TestNilInputsReturnNil verifies nil safety.
func TestNilInputsReturnNil(t *testing.T) {
	engine := internalexternalpressure.NewEngine(fixedClock)

	// Nil inputs
	result := engine.ComputePressureMap(nil)
	if result != nil {
		t.Error("Expected nil for nil inputs")
	}

	// Empty observations
	result = engine.ComputePressureMap(&domainexternalpressure.PressureInputs{
		SovereignCircleIDHash: "test",
		PeriodKey:             "2026-01-07",
		Observations:          nil,
	})
	if result != nil {
		t.Error("Expected nil for empty observations")
	}
}

// TestSourceKindValidation verifies source kind validation.
func TestSourceKindValidation(t *testing.T) {
	validSources := []domainexternalpressure.SourceKind{
		domainexternalpressure.SourceGmailReceipt,
		domainexternalpressure.SourceFinanceTrueLayer,
	}

	for _, s := range validSources {
		if err := s.Validate(); err != nil {
			t.Errorf("Valid source kind failed validation: %s", s)
		}
	}

	// Invalid source
	invalid := domainexternalpressure.SourceKind("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("Expected validation error for invalid source")
	}
}

// TestPressureCategoryValidation verifies category validation.
func TestPressureCategoryValidation(t *testing.T) {
	validCategories := domainexternalpressure.AllPressureCategories()

	for _, c := range validCategories {
		if err := c.Validate(); err != nil {
			t.Errorf("Valid category failed validation: %s", c)
		}
	}

	// Invalid category
	invalid := domainexternalpressure.PressureCategory("invalid")
	if err := invalid.Validate(); err == nil {
		t.Error("Expected validation error for invalid category")
	}
}

// TestExternalCircleDeriveFromSnapshot verifies circle derivation.
func TestExternalCircleDeriveFromSnapshot(t *testing.T) {
	engine := internalexternalpressure.NewEngine(fixedClock)

	snapshot := &domainexternalpressure.PressureMapSnapshot{
		SovereignCircleIDHash: "sov-hash-derive",
		PeriodKey:             "2026-01-07",
		Items: []domainexternalpressure.PressureItem{
			{
				Category:           domainexternalpressure.PressureCategoryDelivery,
				Magnitude:          domainexternalpressure.PressureMagnitudeAFew,
				Horizon:            domainexternalpressure.PressureHorizonUnknown,
				SourceKindsPresent: []domainexternalpressure.SourceKind{domainexternalpressure.SourceGmailReceipt, domainexternalpressure.SourceFinanceTrueLayer},
				EvidenceHash:       "ev-hash-derive",
			},
		},
		StatusHash: "status-hash-derive",
	}

	circles := engine.DeriveExternalCirclesFromSnapshot(snapshot)

	// Should derive 2 circles (one per source kind)
	if len(circles) != 2 {
		t.Errorf("Expected 2 circles, got %d", len(circles))
	}

	// All circles should have valid data
	for _, c := range circles {
		if c.CircleIDHash == "" {
			t.Error("Circle ID hash is empty")
		}
		if c.CreatedPeriod != snapshot.PeriodKey {
			t.Errorf("Wrong period: %s", c.CreatedPeriod)
		}
	}
}

// TestPeriodFromTime verifies period key generation.
func TestPeriodFromTime(t *testing.T) {
	testTime := time.Date(2026, 1, 7, 10, 30, 0, 0, time.UTC)
	period := internalexternalpressure.PeriodFromTime(testTime)

	expected := "2026-01-07"
	if period != expected {
		t.Errorf("Expected period %s, got %s", expected, period)
	}
}

// TestConvertCommerceObservations verifies observation conversion.
func TestConvertCommerceObservations(t *testing.T) {
	sovHash := "sov-hash-convert"
	periodKey := "2026-01-07"

	obsData := []internalexternalpressure.CommerceObservationData{
		{Source: "gmail_receipt", Category: "food_delivery", EvidenceHash: "ev-1"},
		{Source: "finance_truelayer", Category: "transport", EvidenceHash: "ev-2"},
	}

	inputs := internalexternalpressure.ConvertCommerceObservations(sovHash, periodKey, obsData)

	if inputs == nil {
		t.Fatal("Expected non-nil inputs")
	}

	if len(inputs.Observations) != 2 {
		t.Errorf("Expected 2 observations, got %d", len(inputs.Observations))
	}

	if inputs.SovereignCircleIDHash != sovHash {
		t.Errorf("Wrong sovereign hash: %s", inputs.SovereignCircleIDHash)
	}
}

// TestCanonicalStringVersioned verifies version prefix in canonical strings.
func TestCanonicalStringVersioned(t *testing.T) {
	snapshot := &domainexternalpressure.PressureMapSnapshot{
		SovereignCircleIDHash: "sov-hash-version",
		PeriodKey:             "2026-01-07",
		Items:                 nil,
		StatusHash:            "status-hash-version",
	}

	canonical := snapshot.CanonicalString()
	if !strings.HasPrefix(canonical, "PRESSURE_MAP|v1|") {
		t.Errorf("Missing version prefix in canonical string: %s", canonical)
	}
}

// TestDisplayTextAbstract verifies display text is abstract.
func TestDisplayTextAbstract(t *testing.T) {
	// Source kinds should display as abstract words
	if domainexternalpressure.SourceGmailReceipt.DisplayText() != "email" {
		t.Error("Gmail source should display as 'email'")
	}

	if domainexternalpressure.SourceFinanceTrueLayer.DisplayText() != "bank" {
		t.Error("TrueLayer source should display as 'bank'")
	}

	// Categories should have calm display text
	for _, cat := range domainexternalpressure.AllPressureCategories() {
		text := cat.DisplayText()
		if text == "" || text == "Unknown" {
			t.Errorf("Category %s has invalid display text: %s", cat, text)
		}
		// Ensure no vendor names in display text
		if internalexternalpressure.ValidateForbiddenPatterns(text) {
			t.Errorf("Forbidden pattern in category display text: %s", text)
		}
	}
}
