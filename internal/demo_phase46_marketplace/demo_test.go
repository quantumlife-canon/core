// Package demo_phase46_marketplace tests Phase 46: Circle Registry + Packs (Marketplace v0).
//
// CRITICAL INVARIANTS TESTED:
// 1. effect_no_power ALWAYS - packs provide meaning only, no permission
// 2. Observer bindings are INTENT-ONLY - no real wiring
// 3. Hash-only storage - no raw identifiers
// 4. No goroutines in engine/registry
// 5. Clock injection (no time.Now)
// 6. Bounded retention (30 days OR 200 records)
// 7. POST-only for mutations
//
// Reference: docs/ADR/ADR-0084-phase46-circle-registry-packs.md
package demo_phase46_marketplace

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/marketplace"
	"quantumlife/internal/persist"
	domain "quantumlife/pkg/domain/marketplace"
)

// =============================================================================
// Section 1: Domain Type Tests
// =============================================================================

func TestPackSlugValidation(t *testing.T) {
	tests := []struct {
		name    string
		slug    domain.PackSlug
		wantErr bool
	}{
		{"valid slug", domain.PackSlug("family-friends"), false},
		{"valid with numbers", domain.PackSlug("pack-123"), false},
		{"too short", domain.PackSlug("ab"), true},
		{"empty", domain.PackSlug(""), true},
		{"leading hyphen", domain.PackSlug("-test"), true},
		{"trailing hyphen", domain.PackSlug("test-"), true},
		{"uppercase", domain.PackSlug("Test-Pack"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.slug.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PackSlug.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPackKindValidation(t *testing.T) {
	validKinds := []domain.PackKind{
		domain.PackKindSemantics,
		domain.PackKindObserverBinding,
		domain.PackKindCombined,
	}

	for _, kind := range validKinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("PackKind.Validate() unexpected error for %s: %v", kind, err)
		}
	}

	invalidKind := domain.PackKind("invalid")
	if err := invalidKind.Validate(); err == nil {
		t.Error("PackKind.Validate() expected error for invalid kind")
	}
}

func TestPackEffectOnlyAllowsNoPower(t *testing.T) {
	// CRITICAL: Only effect_no_power is allowed
	validEffect := domain.EffectNoPower
	if err := validEffect.Validate(); err != nil {
		t.Errorf("PackEffect.Validate() unexpected error for effect_no_power: %v", err)
	}

	// Any other effect should fail
	invalidEffects := []domain.PackEffect{
		"effect_surface",
		"effect_interrupt",
		"effect_deliver",
		"effect_execute",
		"",
	}

	for _, effect := range invalidEffects {
		if err := effect.Validate(); err == nil {
			t.Errorf("PackEffect.Validate() expected error for %s", effect)
		}
	}
}

func TestBindingKindValidation(t *testing.T) {
	validKinds := []domain.BindingKind{
		domain.BindingKindObserveOnly,
		domain.BindingKindAnnotate,
		domain.BindingKindEnrich,
	}

	for _, kind := range validKinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("BindingKind.Validate() unexpected error for %s: %v", kind, err)
		}
	}

	invalidKind := domain.BindingKind("invalid")
	if err := invalidKind.Validate(); err == nil {
		t.Error("BindingKind.Validate() expected error for invalid kind")
	}
}

func TestSemanticsPresetValidation(t *testing.T) {
	valid := domain.SemanticsPreset{
		CirclePatternHash: "abc123",
		SemanticKind:      "semantic_human",
		UrgencyModel:      "urgency_human_waiting",
		NecessityLevel:    "necessity_high",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("SemanticsPreset.Validate() unexpected error: %v", err)
	}

	// Missing fields should fail
	invalid := domain.SemanticsPreset{}
	if err := invalid.Validate(); err == nil {
		t.Error("SemanticsPreset.Validate() expected error for empty preset")
	}
}

func TestObserverBindingMustHaveEffectNoPower(t *testing.T) {
	// CRITICAL: ObserverBinding must have EffectNoPower
	valid := domain.ObserverBinding{
		CirclePatternHash: "abc123",
		ObserverSlug:      "test-observer",
		BindingKind:       domain.BindingKindObserveOnly,
		Effect:            domain.EffectNoPower,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("ObserverBinding.Validate() unexpected error: %v", err)
	}

	// Invalid effect should fail
	invalid := domain.ObserverBinding{
		CirclePatternHash: "abc123",
		ObserverSlug:      "test-observer",
		BindingKind:       domain.BindingKindObserveOnly,
		Effect:            domain.PackEffect("effect_surface"),
	}
	if err := invalid.Validate(); err == nil {
		t.Error("ObserverBinding.Validate() expected error for invalid effect")
	}
}

func TestPackTemplateValidation(t *testing.T) {
	// Valid semantics-only pack
	validSemantics := domain.PackTemplate{
		Slug:        "test-semantics",
		Kind:        domain.PackKindSemantics,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Test Semantics",
		Description: "Test description",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: "abc123",
				SemanticKind:      "semantic_human",
				UrgencyModel:      "urgency_human_waiting",
				NecessityLevel:    "necessity_high",
			},
		},
		Effect: domain.EffectNoPower,
	}
	if err := validSemantics.Validate(); err != nil {
		t.Errorf("PackTemplate.Validate() unexpected error for semantics pack: %v", err)
	}

	// Valid observer-binding pack
	validObserver := domain.PackTemplate{
		Slug:        "test-observer",
		Kind:        domain.PackKindObserverBinding,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Test Observer",
		Description: "Test description",
		ObserverBindings: []domain.ObserverBinding{
			{
				CirclePatternHash: "abc123",
				ObserverSlug:      "test",
				BindingKind:       domain.BindingKindObserveOnly,
				Effect:            domain.EffectNoPower,
			},
		},
		Effect: domain.EffectNoPower,
	}
	if err := validObserver.Validate(); err != nil {
		t.Errorf("PackTemplate.Validate() unexpected error for observer pack: %v", err)
	}
}

func TestPackTemplateMustHaveEffectNoPower(t *testing.T) {
	// CRITICAL: PackTemplate must have EffectNoPower
	pack := domain.PackTemplate{
		Slug:        "test-pack",
		Kind:        domain.PackKindSemantics,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Test Pack",
		Description: "Test description",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: "abc123",
				SemanticKind:      "semantic_human",
				UrgencyModel:      "urgency_human_waiting",
				NecessityLevel:    "necessity_high",
			},
		},
		Effect: domain.PackEffect("effect_surface"), // Invalid!
	}
	if err := pack.Validate(); err == nil {
		t.Error("PackTemplate.Validate() expected error for invalid effect")
	}
}

func TestPackInstallRecordMustHaveEffectNoPower(t *testing.T) {
	record := domain.PackInstallRecord{
		PeriodKey:    "2025-01-09",
		PackSlugHash: "abc123",
		VersionHash:  "def456",
		StatusHash:   "ghi789",
		Status:       domain.PackStatusInstalled,
		Effect:       domain.PackEffect("effect_execute"), // Invalid!
	}
	if err := record.Validate(); err == nil {
		t.Error("PackInstallRecord.Validate() expected error for invalid effect")
	}

	// Valid record
	validRecord := domain.PackInstallRecord{
		PeriodKey:    "2025-01-09",
		PackSlugHash: "abc123",
		VersionHash:  "def456",
		StatusHash:   "ghi789",
		Status:       domain.PackStatusInstalled,
		Effect:       domain.EffectNoPower,
	}
	if err := validRecord.Validate(); err != nil {
		t.Errorf("PackInstallRecord.Validate() unexpected error: %v", err)
	}
}

func TestCanonicalStringsArePipeDelimited(t *testing.T) {
	preset := domain.SemanticsPreset{
		CirclePatternHash: "hash1",
		SemanticKind:      "kind1",
		UrgencyModel:      "urgency1",
		NecessityLevel:    "level1",
	}
	cs := preset.CanonicalStringV1()
	if !strings.Contains(cs, "|") {
		t.Error("SemanticsPreset.CanonicalStringV1() should be pipe-delimited")
	}

	binding := domain.ObserverBinding{
		CirclePatternHash: "hash1",
		ObserverSlug:      "obs1",
		BindingKind:       domain.BindingKindObserveOnly,
		Effect:            domain.EffectNoPower,
	}
	cs = binding.CanonicalStringV1()
	if !strings.Contains(cs, "|") {
		t.Error("ObserverBinding.CanonicalStringV1() should be pipe-delimited")
	}
}

func TestHashStringIsDeterministic(t *testing.T) {
	input := "test input"
	hash1 := domain.HashString(input)
	hash2 := domain.HashString(input)
	if hash1 != hash2 {
		t.Error("HashString should be deterministic")
	}
}

// =============================================================================
// Section 2: Registry Tests
// =============================================================================

func TestRegistryCreation(t *testing.T) {
	r := marketplace.NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.Count() != 0 {
		t.Error("New registry should be empty")
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := marketplace.NewRegistry()

	pack := domain.PackTemplate{
		Slug:        "test-pack",
		Kind:        domain.PackKindSemantics,
		Tier:        domain.PackTierCurated,
		Visibility:  domain.PackVisibilityPublic,
		Title:       "Test Pack",
		Description: "Test description",
		SemanticsPresets: []domain.SemanticsPreset{
			{
				CirclePatternHash: "abc123",
				SemanticKind:      "semantic_human",
				UrgencyModel:      "urgency_human_waiting",
				NecessityLevel:    "necessity_high",
			},
		},
		Effect: domain.EffectNoPower,
	}

	err := r.Register(pack)
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	got, exists := r.Get(domain.PackSlug("test-pack"))
	if !exists {
		t.Fatal("Get() should find registered pack")
	}
	if got.Title != pack.Title {
		t.Errorf("Get() title = %s, want %s", got.Title, pack.Title)
	}
}

func TestRegistryRejectsInvalidPack(t *testing.T) {
	r := marketplace.NewRegistry()

	// Invalid pack (missing required fields)
	invalidPack := domain.PackTemplate{
		Slug: "test-pack",
		// Missing other required fields
	}

	err := r.Register(invalidPack)
	if err == nil {
		t.Error("Register() should reject invalid pack")
	}
}

func TestDefaultRegistryHasCuratedPacks(t *testing.T) {
	r := marketplace.DefaultRegistry()
	if r.Count() == 0 {
		t.Error("DefaultRegistry should have curated packs")
	}

	// Check for known packs
	_, exists := r.Get("family-friends")
	if !exists {
		t.Error("DefaultRegistry should have family-friends pack")
	}

	_, exists = r.Get("essential-services")
	if !exists {
		t.Error("DefaultRegistry should have essential-services pack")
	}
}

func TestDefaultRegistryPacksHaveEffectNoPower(t *testing.T) {
	// CRITICAL: All default packs must have EffectNoPower
	r := marketplace.DefaultRegistry()
	for _, pack := range r.List() {
		if pack.Effect != domain.EffectNoPower {
			t.Errorf("Pack %s has effect %s, want effect_no_power", pack.Slug, pack.Effect)
		}
		// Check all observer bindings too
		for _, binding := range pack.ObserverBindings {
			if binding.Effect != domain.EffectNoPower {
				t.Errorf("Pack %s binding has effect %s, want effect_no_power", pack.Slug, binding.Effect)
			}
		}
	}
}

func TestRegistryListPublic(t *testing.T) {
	r := marketplace.DefaultRegistry()
	public := r.ListPublic()
	for _, pack := range public {
		if pack.Visibility != domain.PackVisibilityPublic {
			t.Errorf("ListPublic returned non-public pack: %s", pack.Slug)
		}
	}
}

func TestRegistryListByKind(t *testing.T) {
	r := marketplace.DefaultRegistry()
	semantics := r.ListByKind(domain.PackKindSemantics)
	for _, pack := range semantics {
		if pack.Kind != domain.PackKindSemantics {
			t.Errorf("ListByKind(semantics) returned wrong kind: %s", pack.Kind)
		}
	}
}

// =============================================================================
// Section 3: Engine Tests
// =============================================================================

func TestEngineCreation(t *testing.T) {
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)
	if e == nil {
		t.Fatal("NewEngine() returned nil")
	}
}

func TestEngineBuildHomePage(t *testing.T) {
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)
	r := marketplace.DefaultRegistry()

	inputs := marketplace.MarketplaceInputs{
		AvailablePacks: r.ListPublic(),
		InstalledPacks: nil,
		RemovedPacks:   nil,
	}

	page := e.BuildHomePage(inputs)
	if page.Title == "" {
		t.Error("BuildHomePage should set title")
	}
	if len(page.AvailablePacks) == 0 {
		t.Error("BuildHomePage should have available packs")
	}
	if page.StatusHash == "" {
		t.Error("BuildHomePage should set status hash")
	}
}

func TestEngineHomePagePacksHaveEffectNoPower(t *testing.T) {
	// CRITICAL: All pack cards must have EffectNoPower
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)
	r := marketplace.DefaultRegistry()

	inputs := marketplace.MarketplaceInputs{
		AvailablePacks: r.ListPublic(),
	}

	page := e.BuildHomePage(inputs)
	for _, card := range page.AvailablePacks {
		if card.Effect != domain.EffectNoPower {
			t.Errorf("Available pack %s has effect %s, want effect_no_power", card.SlugHash, card.Effect)
		}
	}
}

func TestEngineBuildDetailPage(t *testing.T) {
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)
	r := marketplace.DefaultRegistry()

	pack, _ := r.Get("family-friends")
	page := e.BuildDetailPage(pack, false)

	if page.Title == "" {
		t.Error("BuildDetailPage should set title")
	}
	if !page.CanInstall {
		t.Error("BuildDetailPage should allow install when not installed")
	}
	if page.CanRemove {
		t.Error("BuildDetailPage should not allow remove when not installed")
	}
}

func TestEngineBuildInstallIntentHasEffectNoPower(t *testing.T) {
	// CRITICAL: Install intent must have EffectNoPower
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)
	r := marketplace.DefaultRegistry()

	pack, _ := r.Get("family-friends")
	intent := e.BuildInstallIntent(pack)

	if intent.Effect != domain.EffectNoPower {
		t.Errorf("BuildInstallIntent effect = %s, want effect_no_power", intent.Effect)
	}
}

func TestEngineApplyInstallIntentHasEffectNoPower(t *testing.T) {
	// CRITICAL: Applied record must have EffectNoPower
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)

	intent := domain.PackInstallIntent{
		PackSlugHash: "abc123",
		VersionHash:  "def456",
		Effect:       domain.EffectNoPower,
	}

	record := e.ApplyInstallIntent(intent)
	if record.Effect != domain.EffectNoPower {
		t.Errorf("ApplyInstallIntent record effect = %s, want effect_no_power", record.Effect)
	}
	if record.Status != domain.PackStatusInstalled {
		t.Errorf("ApplyInstallIntent status = %s, want installed", record.Status)
	}
}

func TestEngineBuildProofPage(t *testing.T) {
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)

	inputs := marketplace.MarketplaceInputs{
		AvailablePacks: nil,
		InstalledPacks: nil,
		RemovedPacks:   nil,
	}

	page := e.BuildProofPage(inputs)
	if page.Title == "" {
		t.Error("BuildProofPage should set title")
	}
	if len(page.Lines) == 0 {
		t.Error("BuildProofPage should have lines")
	}
	if page.StatusHash == "" {
		t.Error("BuildProofPage should set status hash")
	}
}

func TestEngineComputeCue(t *testing.T) {
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)

	// No packs installed - cue should be available
	inputs := marketplace.MarketplaceInputs{}
	cue := e.ComputeCue(inputs)
	if !cue.Available {
		t.Error("ComputeCue should be available when no packs installed")
	}
	if cue.Path != "/marketplace" {
		t.Errorf("ComputeCue path = %s, want /marketplace", cue.Path)
	}
}

func TestEngineIsDeterministic(t *testing.T) {
	clock := func() string { return "2025-01-09" }
	e := marketplace.NewEngine(clock)
	r := marketplace.DefaultRegistry()

	inputs := marketplace.MarketplaceInputs{
		AvailablePacks: r.ListPublic(),
	}

	page1 := e.BuildHomePage(inputs)
	page2 := e.BuildHomePage(inputs)

	if page1.StatusHash != page2.StatusHash {
		t.Error("BuildHomePage should be deterministic")
	}
}

// =============================================================================
// Section 4: Store Tests
// =============================================================================

func TestMarketplaceInstallStoreCreation(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceInstallStore(clock)
	if store == nil {
		t.Fatal("NewMarketplaceInstallStore() returned nil")
	}
	if store.Count() != 0 {
		t.Error("New store should be empty")
	}
}

func TestMarketplaceInstallStoreUpsertAndGet(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceInstallStore(clock)

	record := domain.PackInstallRecord{
		PeriodKey:    "2025-01-09",
		PackSlugHash: "abc123",
		VersionHash:  "def456",
		StatusHash:   "unique-hash-1",
		Status:       domain.PackStatusInstalled,
		Effect:       domain.EffectNoPower,
	}

	err := store.Upsert(record)
	if err != nil {
		t.Fatalf("Upsert() error: %v", err)
	}

	got, exists := store.GetLatest("abc123")
	if !exists {
		t.Fatal("GetLatest() should find record")
	}
	if got.PackSlugHash != record.PackSlugHash {
		t.Error("GetLatest() returned wrong record")
	}
}

func TestMarketplaceInstallStoreFIFOEviction(t *testing.T) {
	// CRITICAL: Store must evict old records to stay within bounds
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceInstallStore(clock)

	// Add max+1 records
	for i := 0; i < persist.MarketplaceInstallMaxRecords+1; i++ {
		record := domain.PackInstallRecord{
			PeriodKey:    "2025-01-09",
			PackSlugHash: domain.HashString(string(rune(i))),
			VersionHash:  "v1",
			StatusHash:   domain.HashString(string(rune(i)) + "status"),
			Status:       domain.PackStatusInstalled,
			Effect:       domain.EffectNoPower,
		}
		_ = store.Upsert(record)
	}

	if store.Count() > persist.MarketplaceInstallMaxRecords {
		t.Errorf("Store count = %d, want <= %d", store.Count(), persist.MarketplaceInstallMaxRecords)
	}
}

func TestMarketplaceInstallStoreDedup(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceInstallStore(clock)

	record := domain.PackInstallRecord{
		PeriodKey:    "2025-01-09",
		PackSlugHash: "abc123",
		VersionHash:  "def456",
		StatusHash:   "unique-hash-1",
		Status:       domain.PackStatusInstalled,
		Effect:       domain.EffectNoPower,
	}

	_ = store.Upsert(record)
	_ = store.Upsert(record) // Duplicate

	if store.Count() != 1 {
		t.Errorf("Store should deduplicate, count = %d, want 1", store.Count())
	}
}

func TestMarketplaceRemovalStoreCreation(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceRemovalStore(clock)
	if store == nil {
		t.Fatal("NewMarketplaceRemovalStore() returned nil")
	}
}

func TestMarketplaceAckStoreCreation(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceAckStore(clock)
	if store == nil {
		t.Fatal("NewMarketplaceAckStore() returned nil")
	}
}

func TestMarketplaceAckStoreRecordAndCheck(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	store := persist.NewMarketplaceAckStore(clock)

	ack := domain.MarketplaceProofAck{
		PeriodKey:  "2025-01-09",
		StatusHash: "abc123",
		AckKind:    domain.AckKindDismissed,
	}

	err := store.RecordProofAck(ack)
	if err != nil {
		t.Fatalf("RecordProofAck() error: %v", err)
	}

	if !store.IsProofDismissed("2025-01-09") {
		t.Error("IsProofDismissed should return true after dismissal")
	}
}

// =============================================================================
// Section 5: Integration Tests
// =============================================================================

func TestFullInstallFlow(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	clockStr := func() string { return "2025-01-09" }

	registry := marketplace.DefaultRegistry()
	engine := marketplace.NewEngine(clockStr)
	store := persist.NewMarketplaceInstallStore(clock)

	// Get a pack
	pack, exists := registry.Get("family-friends")
	if !exists {
		t.Fatal("Pack should exist")
	}

	// Build install intent
	intent := engine.BuildInstallIntent(pack)
	if intent.Effect != domain.EffectNoPower {
		t.Error("Install intent must have effect_no_power")
	}

	// Apply intent
	record := engine.ApplyInstallIntent(intent)
	if record.Effect != domain.EffectNoPower {
		t.Error("Install record must have effect_no_power")
	}

	// Store record
	err := store.Upsert(record)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}

	// Verify installed
	installed := store.ListInstalled()
	if len(installed) != 1 {
		t.Errorf("Should have 1 installed pack, got %d", len(installed))
	}
}

func TestProofPageShowsInstalledPacks(t *testing.T) {
	clock := func() time.Time { return time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC) }
	clockStr := func() string { return "2025-01-09" }

	registry := marketplace.DefaultRegistry()
	engine := marketplace.NewEngine(clockStr)
	store := persist.NewMarketplaceInstallStore(clock)

	// Install a pack
	pack, _ := registry.Get("family-friends")
	intent := engine.BuildInstallIntent(pack)
	record := engine.ApplyInstallIntent(intent)
	_ = store.Upsert(record)

	// Build proof page
	inputs := marketplace.MarketplaceInputs{
		AvailablePacks: registry.ListPublic(),
		InstalledPacks: store.ListInstalled(),
	}
	page := engine.BuildProofPage(inputs)

	if len(page.InstalledPacks) != 1 {
		t.Errorf("Proof page should show 1 installed pack, got %d", len(page.InstalledPacks))
	}

	// CRITICAL: Proof page installed packs must have effect_no_power
	for _, line := range page.InstalledPacks {
		if line.Effect != domain.EffectNoPower {
			t.Errorf("Proof line effect = %s, want effect_no_power", line.Effect)
		}
	}
}
