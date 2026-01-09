// Package demo_phase45_circle_semantics tests Phase 45: Circle Semantics & Necessity Declaration.
// These tests verify that semantics are meaning-only and do NOT affect behavior.
package demo_phase45_circle_semantics

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	engine "quantumlife/internal/circlesemantics"
	"quantumlife/internal/persist"
	domain "quantumlife/pkg/domain/circlesemantics"
)

// mockClock implements engine.Clock for testing.
type mockClock struct {
	t time.Time
}

func (c *mockClock) Now() time.Time {
	return c.t
}

func newMockClock(t time.Time) *mockClock {
	return &mockClock{t: t}
}

// ============================================================================
// Determinism Tests
// ============================================================================

func TestSemanticsHashDeterministic(t *testing.T) {
	s1 := domain.CircleSemantics{
		Kind:        domain.SemanticHuman,
		Urgency:     domain.UrgencyHumanWaiting,
		Necessity:   domain.NecessityMedium,
		Provenance:  domain.ProvenanceUserDeclared,
		Effect:      domain.EffectNoPower,
		NotesBucket: domain.NotesBucketUserSet,
	}
	s2 := domain.CircleSemantics{
		Kind:        domain.SemanticHuman,
		Urgency:     domain.UrgencyHumanWaiting,
		Necessity:   domain.NecessityMedium,
		Provenance:  domain.ProvenanceUserDeclared,
		Effect:      domain.EffectNoPower,
		NotesBucket: domain.NotesBucketUserSet,
	}

	hash1 := domain.ComputeSemanticsHash(s1)
	hash2 := domain.ComputeSemanticsHash(s2)

	if hash1 != hash2 {
		t.Errorf("Same semantics should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestCanonicalStringDeterministic(t *testing.T) {
	s := domain.CircleSemantics{
		Kind:        domain.SemanticInstitution,
		Urgency:     domain.UrgencyHardDeadline,
		Necessity:   domain.NecessityHigh,
		Provenance:  domain.ProvenanceDerivedRules,
		Effect:      domain.EffectNoPower,
		NotesBucket: domain.NotesBucketDerived,
	}

	c1 := s.CanonicalStringV1()
	c2 := s.CanonicalStringV1()

	if c1 != c2 {
		t.Errorf("CanonicalStringV1 should be deterministic: %s != %s", c1, c2)
	}

	// Verify pipe-delimited format
	if !strings.Contains(c1, "|") {
		t.Errorf("CanonicalStringV1 should be pipe-delimited: %s", c1)
	}
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestCircleSemanticKindValidate(t *testing.T) {
	validKinds := []domain.CircleSemanticKind{
		domain.SemanticHuman,
		domain.SemanticInstitution,
		domain.SemanticServiceEssential,
		domain.SemanticServiceTransactional,
		domain.SemanticServiceOptional,
		domain.SemanticUnknown,
	}

	for _, k := range validKinds {
		if err := k.Validate(); err != nil {
			t.Errorf("Valid kind %s should not error: %v", k, err)
		}
	}

	invalid := domain.CircleSemanticKind("invalid_kind")
	if err := invalid.Validate(); err == nil {
		t.Errorf("Invalid kind should error")
	}
}

func TestUrgencyModelValidate(t *testing.T) {
	validModels := []domain.UrgencyModel{
		domain.UrgencyNeverInterrupt,
		domain.UrgencyHardDeadline,
		domain.UrgencyHumanWaiting,
		domain.UrgencyTimeWindow,
		domain.UrgencySoftReminder,
		domain.UrgencyUnknown,
	}

	for _, m := range validModels {
		if err := m.Validate(); err != nil {
			t.Errorf("Valid model %s should not error: %v", m, err)
		}
	}

	invalid := domain.UrgencyModel("invalid_model")
	if err := invalid.Validate(); err == nil {
		t.Errorf("Invalid model should error")
	}
}

func TestNecessityLevelValidate(t *testing.T) {
	validLevels := []domain.NecessityLevel{
		domain.NecessityLow,
		domain.NecessityMedium,
		domain.NecessityHigh,
		domain.NecessityUnknown,
	}

	for _, l := range validLevels {
		if err := l.Validate(); err != nil {
			t.Errorf("Valid level %s should not error: %v", l, err)
		}
	}

	invalid := domain.NecessityLevel("invalid_level")
	if err := invalid.Validate(); err == nil {
		t.Errorf("Invalid level should error")
	}
}

func TestSemanticsEffectValidate_OnlyNoPower(t *testing.T) {
	// Only effect_no_power is valid in Phase 45
	if err := domain.EffectNoPower.Validate(); err != nil {
		t.Errorf("EffectNoPower should be valid: %v", err)
	}

	// Any other effect should be invalid
	invalid := domain.SemanticsEffect("effect_has_power")
	if err := invalid.Validate(); err == nil {
		t.Errorf("effect_has_power should be invalid in Phase 45")
	}
}

func TestNotesBucketValidate(t *testing.T) {
	validBuckets := []string{
		domain.NotesBucketNone,
		domain.NotesBucketUserSet,
		domain.NotesBucketDerived,
	}

	for _, b := range validBuckets {
		if err := domain.ValidateNotesBucket(b); err != nil {
			t.Errorf("Valid bucket %s should not error: %v", b, err)
		}
	}

	if err := domain.ValidateNotesBucket("invalid_bucket"); err == nil {
		t.Errorf("Invalid bucket should error")
	}
}

// ============================================================================
// Engine Default Derivation Tests
// ============================================================================

func TestDeriveDefaultSemantics_Commerce(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	s := eng.DeriveDefaultSemantics("hash123", engine.CircleTypeCommerce)

	if s.Kind != domain.SemanticServiceOptional {
		t.Errorf("Commerce should be semantic_service_optional, got %s", s.Kind)
	}
	if s.Urgency != domain.UrgencyNeverInterrupt {
		t.Errorf("Commerce should have urgency_never_interrupt, got %s", s.Urgency)
	}
	if s.Necessity != domain.NecessityLow {
		t.Errorf("Commerce should have necessity_low, got %s", s.Necessity)
	}
	if s.Effect != domain.EffectNoPower {
		t.Errorf("Commerce should have effect_no_power, got %s", s.Effect)
	}
}

func TestDeriveDefaultSemantics_Human(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	s := eng.DeriveDefaultSemantics("hash456", engine.CircleTypeHuman)

	if s.Kind != domain.SemanticHuman {
		t.Errorf("Human should be semantic_human, got %s", s.Kind)
	}
	if s.Urgency != domain.UrgencyHumanWaiting {
		t.Errorf("Human should have urgency_human_waiting, got %s", s.Urgency)
	}
	if s.Necessity != domain.NecessityMedium {
		t.Errorf("Human should have necessity_medium, got %s", s.Necessity)
	}
	if s.Effect != domain.EffectNoPower {
		t.Errorf("Human should have effect_no_power, got %s", s.Effect)
	}
}

func TestDeriveDefaultSemantics_Institution(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	s := eng.DeriveDefaultSemantics("hash789", engine.CircleTypeInstitution)

	if s.Kind != domain.SemanticInstitution {
		t.Errorf("Institution should be semantic_institution, got %s", s.Kind)
	}
	if s.Urgency != domain.UrgencyHardDeadline {
		t.Errorf("Institution should have urgency_hard_deadline, got %s", s.Urgency)
	}
	if s.Necessity != domain.NecessityHigh {
		t.Errorf("Institution should have necessity_high, got %s", s.Necessity)
	}
	if s.Effect != domain.EffectNoPower {
		t.Errorf("Institution should have effect_no_power, got %s", s.Effect)
	}
}

func TestDeriveDefaultSemantics_Unknown(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	s := eng.DeriveDefaultSemantics("hashabc", engine.CircleTypeUnknown)

	if s.Kind != domain.SemanticUnknown {
		t.Errorf("Unknown should be semantic_unknown, got %s", s.Kind)
	}
	if s.Urgency != domain.UrgencyUnknown {
		t.Errorf("Unknown should have urgency_unknown, got %s", s.Urgency)
	}
	if s.Necessity != domain.NecessityUnknown {
		t.Errorf("Unknown should have necessity_unknown, got %s", s.Necessity)
	}
	if s.Effect != domain.EffectNoPower {
		t.Errorf("Unknown should have effect_no_power, got %s", s.Effect)
	}
}

// ============================================================================
// ApplyUserDeclaration Tests
// ============================================================================

func TestApplyUserDeclaration_EnforcesProvenance(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	desired := domain.CircleSemantics{
		Kind:       domain.SemanticHuman,
		Urgency:    domain.UrgencyHumanWaiting,
		Necessity:  domain.NecessityMedium,
		Provenance: domain.ProvenanceDerivedRules, // Should be overridden
		Effect:     domain.EffectNoPower,
	}

	record, _, err := eng.ApplyUserDeclaration("circleHash", desired, nil)
	if err != nil {
		t.Fatalf("ApplyUserDeclaration failed: %v", err)
	}

	// Should enforce provenance_user_declared
	if record.Semantics.Provenance != domain.ProvenanceUserDeclared {
		t.Errorf("Provenance should be user_declared, got %s", record.Semantics.Provenance)
	}
}

func TestApplyUserDeclaration_EnforcesEffectNoPower(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	// Try to set a different effect (should be ignored)
	desired := domain.CircleSemantics{
		Kind:      domain.SemanticHuman,
		Urgency:   domain.UrgencyHumanWaiting,
		Necessity: domain.NecessityMedium,
		Effect:    domain.SemanticsEffect("effect_has_power"), // Should be overridden
	}

	record, _, err := eng.ApplyUserDeclaration("circleHash", desired, nil)
	if err != nil {
		t.Fatalf("ApplyUserDeclaration failed: %v", err)
	}

	// Should ALWAYS enforce effect_no_power
	if record.Semantics.Effect != domain.EffectNoPower {
		t.Errorf("Effect should be effect_no_power, got %s", record.Semantics.Effect)
	}
}

func TestApplyUserDeclaration_NoChangeDetection(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	desired := domain.CircleSemantics{
		Kind:        domain.SemanticHuman,
		Urgency:     domain.UrgencyHumanWaiting,
		Necessity:   domain.NecessityMedium,
		Provenance:  domain.ProvenanceUserDeclared,
		Effect:      domain.EffectNoPower,
		NotesBucket: domain.NotesBucketUserSet,
	}

	// First declaration
	record1, change1, err := eng.ApplyUserDeclaration("circleHash", desired, nil)
	if err != nil {
		t.Fatalf("First ApplyUserDeclaration failed: %v", err)
	}
	if change1.ChangeKind != domain.ChangeKindCreated {
		t.Errorf("First change should be 'created', got %s", change1.ChangeKind)
	}

	// Same declaration again
	_, change2, err := eng.ApplyUserDeclaration("circleHash", desired, &record1.Semantics)
	if err != nil {
		t.Fatalf("Second ApplyUserDeclaration failed: %v", err)
	}
	if change2.ChangeKind != domain.ChangeKindNoChange {
		t.Errorf("Second change should be 'no_change', got %s", change2.ChangeKind)
	}
}

func TestApplyUserDeclaration_UpdateDetection(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	desired1 := domain.CircleSemantics{
		Kind:      domain.SemanticHuman,
		Urgency:   domain.UrgencyHumanWaiting,
		Necessity: domain.NecessityMedium,
		Effect:    domain.EffectNoPower,
	}

	record1, _, _ := eng.ApplyUserDeclaration("circleHash", desired1, nil)

	// Different declaration
	desired2 := domain.CircleSemantics{
		Kind:      domain.SemanticInstitution, // Changed
		Urgency:   domain.UrgencyHardDeadline, // Changed
		Necessity: domain.NecessityHigh,       // Changed
		Effect:    domain.EffectNoPower,
	}

	_, change, err := eng.ApplyUserDeclaration("circleHash", desired2, &record1.Semantics)
	if err != nil {
		t.Fatalf("ApplyUserDeclaration failed: %v", err)
	}
	if change.ChangeKind != domain.ChangeKindUpdated {
		t.Errorf("Change should be 'updated', got %s", change.ChangeKind)
	}
}

// ============================================================================
// Store Tests
// ============================================================================

func TestStoreFIFOEviction(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	store := persist.NewCircleSemanticsStore(clk.Now)

	// Add more than max records
	for i := 0; i < 250; i++ {
		record := domain.SemanticsRecord{
			PeriodKey:     "2024-01-15",
			CircleIDHash:  domain.HashString(string(rune(i))),
			SemanticsHash: domain.HashString(string(rune(i))),
			StatusHash:    domain.HashString(string(rune(i)) + "|status"),
			Semantics: domain.CircleSemantics{
				Kind:      domain.SemanticHuman,
				Urgency:   domain.UrgencyHumanWaiting,
				Necessity: domain.NecessityMedium,
				Effect:    domain.EffectNoPower,
			},
		}
		_ = store.Upsert(record)
	}

	count := store.Count()
	if count > persist.CircleSemanticsMaxRecords {
		t.Errorf("Store should evict to max %d, got %d", persist.CircleSemanticsMaxRecords, count)
	}
}

func TestStoreRetentionWindow(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := baseTime
	clk := func() time.Time { return currentTime }
	store := persist.NewCircleSemanticsStore(clk)

	// Add a record with old period key
	oldRecord := domain.SemanticsRecord{
		PeriodKey:     "2023-12-01", // More than 30 days old
		CircleIDHash:  "old_circle",
		SemanticsHash: "old_hash",
		StatusHash:    "old_status",
		Semantics: domain.CircleSemantics{
			Kind:   domain.SemanticHuman,
			Effect: domain.EffectNoPower,
		},
	}
	_ = store.Upsert(oldRecord)

	// Add a new record (triggers eviction check)
	newRecord := domain.SemanticsRecord{
		PeriodKey:     "2024-01-15",
		CircleIDHash:  "new_circle",
		SemanticsHash: "new_hash",
		StatusHash:    "new_status",
		Semantics: domain.CircleSemantics{
			Kind:   domain.SemanticHuman,
			Effect: domain.EffectNoPower,
		},
	}
	_ = store.Upsert(newRecord)

	// Old record should be evicted
	_, found := store.GetLatest("old_circle")
	if found {
		t.Errorf("Old record should be evicted after 30 days")
	}
}

func TestProofAckDismissal(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	ackStore := persist.NewCircleSemanticsAckStore(clk.Now)

	periodKey := "2024-01-15"

	// Not dismissed initially
	if ackStore.IsProofDismissed(periodKey) {
		t.Errorf("Proof should not be dismissed initially")
	}

	// Record dismissal
	ack := domain.SemanticsProofAck{
		PeriodKey:  periodKey,
		StatusHash: "some_hash",
		AckKind:    domain.AckKindDismissed,
	}
	_ = ackStore.RecordProofAck(ack)

	// Should be dismissed now
	if !ackStore.IsProofDismissed(periodKey) {
		t.Errorf("Proof should be dismissed after ack")
	}
}

// ============================================================================
// Cue Tests
// ============================================================================

func TestComputeCue_ShowsWhenUnknownAndConnected(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)
	ackStore := persist.NewCircleSemanticsAckStore(clk.Now)

	inputs := engine.SemanticsInputs{
		CircleIDHashes: []string{"circle1"},
		CircleTypes:    map[string]string{"circle1": engine.CircleTypeUnknown},
		HasGmail:       true, // Connected
		HasTrueLayer:   false,
	}

	// No existing records (all are unknown)
	cue := eng.ComputeCue(inputs, nil, ackStore)

	if !cue.Available {
		t.Errorf("Cue should be available when unknown semantics exist and connected")
	}
	if cue.Path != "/settings/semantics" {
		t.Errorf("Cue path should be /settings/semantics, got %s", cue.Path)
	}
}

func TestComputeCue_HiddenWhenNotConnected(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)
	ackStore := persist.NewCircleSemanticsAckStore(clk.Now)

	inputs := engine.SemanticsInputs{
		CircleIDHashes: []string{"circle1"},
		CircleTypes:    map[string]string{"circle1": engine.CircleTypeUnknown},
		HasGmail:       false, // Not connected
		HasTrueLayer:   false,
	}

	cue := eng.ComputeCue(inputs, nil, ackStore)

	if cue.Available {
		t.Errorf("Cue should not be available when not connected")
	}
}

func TestComputeCue_HiddenWhenDismissed(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)
	ackStore := persist.NewCircleSemanticsAckStore(clk.Now)

	// Dismiss the proof
	ack := domain.SemanticsProofAck{
		PeriodKey:  "2024-01-15",
		StatusHash: "hash",
		AckKind:    domain.AckKindDismissed,
	}
	_ = ackStore.RecordProofAck(ack)

	inputs := engine.SemanticsInputs{
		CircleIDHashes: []string{"circle1"},
		CircleTypes:    map[string]string{"circle1": engine.CircleTypeUnknown},
		HasGmail:       true,
	}

	cue := eng.ComputeCue(inputs, nil, ackStore)

	if cue.Available {
		t.Errorf("Cue should not be available when dismissed")
	}
}

func TestComputeCue_HiddenWhenNoUnknown(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)
	ackStore := persist.NewCircleSemanticsAckStore(clk.Now)

	inputs := engine.SemanticsInputs{
		CircleIDHashes: []string{"circle1"},
		CircleTypes:    map[string]string{"circle1": engine.CircleTypeHuman}, // Not unknown
		HasGmail:       true,
	}

	// Existing record with known semantics
	records := []domain.SemanticsRecord{
		{
			CircleIDHash: "circle1",
			Semantics: domain.CircleSemantics{
				Kind:   domain.SemanticHuman, // Not unknown
				Effect: domain.EffectNoPower,
			},
		},
	}

	cue := eng.ComputeCue(inputs, records, ackStore)

	if cue.Available {
		t.Errorf("Cue should not be available when no unknown semantics")
	}
}

// ============================================================================
// No Behavior Change Test
// ============================================================================

func TestNoBehaviorChange_EngineProducesNoDecisionTypes(t *testing.T) {
	// Phase 45 MUST NOT produce any Decision or Outcome types
	// Verify by checking the engine return types

	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	// DeriveDefaultSemantics returns CircleSemantics (meaning-only)
	s := eng.DeriveDefaultSemantics("hash", engine.CircleTypeHuman)
	if s.Effect != domain.EffectNoPower {
		t.Errorf("All engine outputs must have effect_no_power")
	}

	// ApplyUserDeclaration returns record + change (no Decision)
	desired := domain.CircleSemantics{
		Kind:      domain.SemanticHuman,
		Urgency:   domain.UrgencyHumanWaiting,
		Necessity: domain.NecessityMedium,
		Effect:    domain.EffectNoPower,
	}
	record, _, err := eng.ApplyUserDeclaration("hash", desired, nil)
	if err != nil {
		t.Fatalf("ApplyUserDeclaration failed: %v", err)
	}
	if record.Semantics.Effect != domain.EffectNoPower {
		t.Errorf("All records must have effect_no_power")
	}

	// BuildProofPage returns proof page (no Decision)
	page := eng.BuildProofPage([]domain.SemanticsRecord{record})
	for _, entry := range page.Entries {
		if entry.Effect != domain.EffectNoPower {
			t.Errorf("All proof entries must have effect_no_power")
		}
	}

	// ComputeCue returns cue (no Decision)
	cue := eng.ComputeCue(engine.SemanticsInputs{}, nil, nil)
	// Cue has no Effect field - it's just a whisper hint
	if cue.Path != "" && cue.Path != "/settings/semantics" {
		t.Errorf("Cue path must be empty or /settings/semantics")
	}
}

// ============================================================================
// Web Handler Tests (using httptest)
// ============================================================================

func TestHandlerPOSTSaveRejectsMissingFields(t *testing.T) {
	// Simulate POST without required fields
	form := url.Values{}
	// Missing all required fields

	req := httptest.NewRequest(http.MethodPost, "/settings/semantics/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Note: This test verifies the pattern - actual handler tests would require server setup
	// The key assertion is that missing fields should result in BadRequest

	// For now, verify the ParseSemanticsFromForm function rejects invalid inputs
	_, err := engine.ParseSemanticsFromForm("", "", "")
	if err == nil {
		t.Errorf("ParseSemanticsFromForm should reject empty values")
	}
}

func TestParseSemanticsFromForm_ValidInputs(t *testing.T) {
	s, err := engine.ParseSemanticsFromForm(
		string(domain.SemanticHuman),
		string(domain.UrgencyHumanWaiting),
		string(domain.NecessityMedium),
	)
	if err != nil {
		t.Fatalf("ParseSemanticsFromForm failed: %v", err)
	}

	if s.Kind != domain.SemanticHuman {
		t.Errorf("Kind should be SemanticHuman, got %s", s.Kind)
	}
	if s.Urgency != domain.UrgencyHumanWaiting {
		t.Errorf("Urgency should be UrgencyHumanWaiting, got %s", s.Urgency)
	}
	if s.Necessity != domain.NecessityMedium {
		t.Errorf("Necessity should be NecessityMedium, got %s", s.Necessity)
	}
	// Effect should always be enforced to no_power
	if s.Effect != domain.EffectNoPower {
		t.Errorf("Effect should be EffectNoPower, got %s", s.Effect)
	}
}

func TestParseSemanticsFromForm_InvalidInputs(t *testing.T) {
	_, err := engine.ParseSemanticsFromForm("invalid_kind", "invalid_urgency", "invalid_necessity")
	if err == nil {
		t.Errorf("ParseSemanticsFromForm should reject invalid inputs")
	}
}

// ============================================================================
// BuildSettingsPage Tests
// ============================================================================

func TestBuildSettingsPage_StableSorted(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	inputs := engine.SemanticsInputs{
		CircleIDHashes: []string{"zzz_circle", "aaa_circle", "mmm_circle"},
		CircleTypes: map[string]string{
			"zzz_circle": engine.CircleTypeHuman,
			"aaa_circle": engine.CircleTypeInstitution,
			"mmm_circle": engine.CircleTypeCommerce,
		},
	}

	page := eng.BuildSettingsPage(inputs, nil)

	// Items should be sorted by CircleIDHash
	if len(page.Items) < 3 {
		t.Fatalf("Expected at least 3 items, got %d", len(page.Items))
	}
	if page.Items[0].CircleIDHash != "aaa_circle" {
		t.Errorf("First item should be aaa_circle, got %s", page.Items[0].CircleIDHash)
	}
	if page.Items[1].CircleIDHash != "mmm_circle" {
		t.Errorf("Second item should be mmm_circle, got %s", page.Items[1].CircleIDHash)
	}
	if page.Items[2].CircleIDHash != "zzz_circle" {
		t.Errorf("Third item should be zzz_circle, got %s", page.Items[2].CircleIDHash)
	}
}

func TestBuildProofPage_StableSorted(t *testing.T) {
	clk := newMockClock(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	eng := engine.NewEngine(clk)

	records := []domain.SemanticsRecord{
		{CircleIDHash: "zzz", Semantics: domain.CircleSemantics{Effect: domain.EffectNoPower}},
		{CircleIDHash: "aaa", Semantics: domain.CircleSemantics{Effect: domain.EffectNoPower}},
	}

	page := eng.BuildProofPage(records)

	if len(page.Entries) < 2 {
		t.Fatalf("Expected at least 2 entries, got %d", len(page.Entries))
	}
	if page.Entries[0].CircleIDHash != "aaa" {
		t.Errorf("First entry should be aaa, got %s", page.Entries[0].CircleIDHash)
	}
}

// ============================================================================
// CircleCountBucket Tests
// ============================================================================

func TestCircleCountBucket(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{0, "nothing"},
		{1, "a_few"},
		{3, "a_few"},
		{4, "several"},
		{100, "several"},
	}

	for _, tc := range tests {
		result := domain.CircleCountBucket(tc.count)
		if result != tc.expected {
			t.Errorf("CircleCountBucket(%d) = %s, want %s", tc.count, result, tc.expected)
		}
	}
}
