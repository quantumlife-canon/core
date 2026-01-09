// Package demo_phase47_coverage_realization contains demo tests for Phase 47.
//
// Phase 47: Pack Coverage Realization
// Reference: docs/ADR/ADR-0085-phase47-pack-coverage-realization.md
//
// CRITICAL: Coverage realization expands OBSERVERS only, NEVER grants permission.
// CRITICAL: NEVER changes interrupt policy, delivery, or execution.
// CRITICAL: Track B: Expand observers, not actions.
package demo_phase47_coverage_realization

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coverageplan "quantumlife/internal/coverageplan"
	"quantumlife/internal/persist"
	domain "quantumlife/pkg/domain/coverageplan"
	marketplace "quantumlife/pkg/domain/marketplace"
)

// =============================================================================
// Test Fixtures
// =============================================================================

func fixedClock() string {
	return "2024-01-15"
}

func fixedTime() time.Time {
	return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
}

func makeInstalledPack(slug string) marketplace.PackInstallRecord {
	return marketplace.PackInstallRecord{
		PeriodKey:    "2024-01-15",
		PackSlugHash: marketplace.HashString(slug),
		VersionHash:  marketplace.HashString("v1"),
		StatusHash:   marketplace.ComputeStatusHash("2024-01-15", marketplace.HashString(slug), marketplace.HashString("v1")),
		Status:       marketplace.PackStatusInstalled,
		Effect:       marketplace.EffectNoPower,
	}
}

// =============================================================================
// Section 1: Domain Type Tests
// =============================================================================

func TestCoverageSourceKind_Validate(t *testing.T) {
	tests := []struct {
		kind    domain.CoverageSourceKind
		wantErr bool
	}{
		{domain.SourceGmail, false},
		{domain.SourceFinanceTrueLayer, false},
		{domain.SourceDeviceNotification, false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("CoverageSourceKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

func TestCoverageCapability_Validate(t *testing.T) {
	tests := []struct {
		cap     domain.CoverageCapability
		wantErr bool
	}{
		{domain.CapReceiptObserver, false},
		{domain.CapCommerceObserver, false},
		{domain.CapFinanceCommerceObserver, false},
		{domain.CapPressureMap, false},
		{domain.CapTimeWindowSources, false},
		{domain.CapNotificationMetadata, false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.cap.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("CoverageCapability(%q).Validate() error = %v, wantErr %v", tt.cap, err, tt.wantErr)
		}
	}
}

func TestCoverageChangeKind_Validate(t *testing.T) {
	tests := []struct {
		kind    domain.CoverageChangeKind
		wantErr bool
	}{
		{domain.ChangeAdded, false},
		{domain.ChangeRemoved, false},
		{domain.ChangeUnchanged, false},
		{"invalid", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("CoverageChangeKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

func TestCoverageProofAckKind_Validate(t *testing.T) {
	tests := []struct {
		kind    domain.CoverageProofAckKind
		wantErr bool
	}{
		{domain.AckViewed, false},
		{domain.AckDismissed, false},
		{"invalid", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("CoverageProofAckKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

func TestCoveragePlan_Validate(t *testing.T) {
	validPlan := domain.CoveragePlan{
		CircleIDHash: domain.HashString("circle-1"),
		PeriodKey:    "2024-01-15",
		Sources:      []domain.CoverageSourcePlan{},
		Capabilities: []domain.CoverageCapability{},
		PlanHash:     "abc123",
	}

	if err := validPlan.Validate(); err != nil {
		t.Errorf("Valid CoveragePlan.Validate() error = %v", err)
	}

	// Test missing CircleIDHash
	invalidPlan := validPlan
	invalidPlan.CircleIDHash = ""
	if err := invalidPlan.Validate(); err == nil {
		t.Error("CoveragePlan with empty CircleIDHash should fail validation")
	}
}

func TestCoverageProofAck_Validate(t *testing.T) {
	validAck := domain.CoverageProofAck{
		CircleIDHash: domain.HashString("circle-1"),
		PeriodKey:    "2024-01-15",
		AckKind:      domain.AckDismissed,
		StatusHash:   "abc123",
	}

	if err := validAck.Validate(); err != nil {
		t.Errorf("Valid CoverageProofAck.Validate() error = %v", err)
	}

	// Test invalid AckKind
	invalidAck := validAck
	invalidAck.AckKind = "invalid"
	if err := invalidAck.Validate(); err == nil {
		t.Error("CoverageProofAck with invalid AckKind should fail validation")
	}
}

// =============================================================================
// Section 2: Normalization Tests
// =============================================================================

func TestNormalizeCapabilities_Dedup(t *testing.T) {
	caps := []domain.CoverageCapability{
		domain.CapReceiptObserver,
		domain.CapCommerceObserver,
		domain.CapReceiptObserver, // Duplicate
	}

	result := domain.NormalizeCapabilities(caps)
	if len(result) != 2 {
		t.Errorf("NormalizeCapabilities() expected 2 caps, got %d", len(result))
	}
}

func TestNormalizeCapabilities_Sorted(t *testing.T) {
	caps := []domain.CoverageCapability{
		domain.CapTimeWindowSources,
		domain.CapCommerceObserver,
		domain.CapReceiptObserver,
	}

	result := domain.NormalizeCapabilities(caps)
	for i := 1; i < len(result); i++ {
		if result[i-1] > result[i] {
			t.Errorf("NormalizeCapabilities() not sorted: %v", result)
		}
	}
}

func TestNormalizeSources_Sorted(t *testing.T) {
	sources := []domain.CoverageSourcePlan{
		{Source: domain.SourceGmail, Enabled: []domain.CoverageCapability{domain.CapReceiptObserver}},
		{Source: domain.SourceDeviceNotification, Enabled: []domain.CoverageCapability{domain.CapNotificationMetadata}},
	}

	result := domain.NormalizeSources(sources)
	for i := 1; i < len(result); i++ {
		if result[i-1].Source > result[i].Source {
			t.Errorf("NormalizeSources() not sorted: %v", result)
		}
	}
}

// =============================================================================
// Section 3: Engine Tests - Plan Building
// =============================================================================

func TestEngine_BuildPlan_Empty(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)
	plan := engine.BuildPlan("circle-hash", []marketplace.PackInstallRecord{})

	if len(plan.Capabilities) != 0 {
		t.Errorf("BuildPlan with no packs should have 0 capabilities, got %d", len(plan.Capabilities))
	}
	if plan.CircleIDHash != "circle-hash" {
		t.Errorf("BuildPlan circleIDHash = %s, want circle-hash", plan.CircleIDHash)
	}
	if plan.PeriodKey != "2024-01-15" {
		t.Errorf("BuildPlan periodKey = %s, want 2024-01-15", plan.PeriodKey)
	}
	if plan.PlanHash == "" {
		t.Error("BuildPlan should compute PlanHash")
	}
}

func TestEngine_BuildPlan_WithKnownPack(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)
	installed := []marketplace.PackInstallRecord{
		makeInstalledPack("commerce-observer"),
	}

	plan := engine.BuildPlan("circle-hash", installed)

	// commerce-observer should enable cap_commerce_observer
	if len(plan.Capabilities) == 0 {
		t.Error("BuildPlan with commerce-observer should have capabilities")
	}

	hasCommerce := false
	for _, cap := range plan.Capabilities {
		if cap == domain.CapCommerceObserver {
			hasCommerce = true
		}
	}
	if !hasCommerce {
		t.Error("BuildPlan with commerce-observer should include cap_commerce_observer")
	}
}

func TestEngine_BuildPlan_UnknownPackIgnored(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)
	installed := []marketplace.PackInstallRecord{
		makeInstalledPack("unknown-pack-xyz"),
	}

	plan := engine.BuildPlan("circle-hash", installed)

	// Unknown pack should be ignored, no error
	if len(plan.Capabilities) != 0 {
		t.Errorf("BuildPlan with unknown pack should have 0 capabilities, got %d", len(plan.Capabilities))
	}
}

func TestEngine_BuildPlan_Deterministic(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)
	installed := []marketplace.PackInstallRecord{
		makeInstalledPack("commerce-observer"),
		makeInstalledPack("inbox-enrichment"),
	}

	plan1 := engine.BuildPlan("circle-hash", installed)
	plan2 := engine.BuildPlan("circle-hash", installed)

	if plan1.PlanHash != plan2.PlanHash {
		t.Errorf("BuildPlan should be deterministic: hash1=%s, hash2=%s", plan1.PlanHash, plan2.PlanHash)
	}
}

func TestEngine_BuildPlan_DifferentOrderSameHash(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	installed1 := []marketplace.PackInstallRecord{
		makeInstalledPack("commerce-observer"),
		makeInstalledPack("inbox-enrichment"),
	}
	installed2 := []marketplace.PackInstallRecord{
		makeInstalledPack("inbox-enrichment"),
		makeInstalledPack("commerce-observer"),
	}

	plan1 := engine.BuildPlan("circle-hash", installed1)
	plan2 := engine.BuildPlan("circle-hash", installed2)

	if plan1.PlanHash != plan2.PlanHash {
		t.Errorf("BuildPlan should be order-independent: hash1=%s, hash2=%s", plan1.PlanHash, plan2.PlanHash)
	}
}

// =============================================================================
// Section 4: Engine Tests - Delta Computation
// =============================================================================

func TestEngine_DiffPlans_AllAdded(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	prev := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-14",
		Capabilities: []domain.CoverageCapability{},
	}
	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver, domain.CapCommerceObserver},
	}

	delta := engine.DiffPlans(prev, next)

	if len(delta.Added) != 2 {
		t.Errorf("DiffPlans should have 2 added, got %d", len(delta.Added))
	}
	if len(delta.Removed) != 0 {
		t.Errorf("DiffPlans should have 0 removed, got %d", len(delta.Removed))
	}
}

func TestEngine_DiffPlans_AllRemoved(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	prev := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-14",
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver, domain.CapCommerceObserver},
	}
	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Capabilities: []domain.CoverageCapability{},
	}

	delta := engine.DiffPlans(prev, next)

	if len(delta.Removed) != 2 {
		t.Errorf("DiffPlans should have 2 removed, got %d", len(delta.Removed))
	}
	if len(delta.Added) != 0 {
		t.Errorf("DiffPlans should have 0 added, got %d", len(delta.Added))
	}
}

func TestEngine_DiffPlans_Unchanged(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	caps := []domain.CoverageCapability{domain.CapReceiptObserver}
	prev := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-14",
		Capabilities: caps,
	}
	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Capabilities: caps,
	}

	delta := engine.DiffPlans(prev, next)

	if len(delta.Unchanged) != 1 {
		t.Errorf("DiffPlans should have 1 unchanged, got %d", len(delta.Unchanged))
	}
	if len(delta.Added) != 0 || len(delta.Removed) != 0 {
		t.Error("DiffPlans should have no added or removed")
	}
}

func TestEngine_DiffPlans_Mixed(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	prev := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-14",
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver, domain.CapPressureMap},
	}
	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver, domain.CapCommerceObserver},
	}

	delta := engine.DiffPlans(prev, next)

	if len(delta.Added) != 1 {
		t.Errorf("DiffPlans should have 1 added, got %d", len(delta.Added))
	}
	if len(delta.Removed) != 1 {
		t.Errorf("DiffPlans should have 1 removed, got %d", len(delta.Removed))
	}
	if len(delta.Unchanged) != 1 {
		t.Errorf("DiffPlans should have 1 unchanged, got %d", len(delta.Unchanged))
	}
}

// =============================================================================
// Section 5: Engine Tests - Proof Page
// =============================================================================

func TestEngine_BuildProofPage_NoChanges(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	delta := domain.CoverageDelta{
		Added:     []domain.CoverageCapability{},
		Removed:   []domain.CoverageCapability{},
		Unchanged: []domain.CoverageCapability{domain.CapReceiptObserver},
	}
	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		PlanHash:     "abc123",
	}

	page := engine.BuildProofPage("circle-hash", nil, next, delta)

	if len(page.Lines) == 0 {
		t.Error("BuildProofPage should have lines")
	}
	if page.Lines[0] != "Nothing changed." {
		t.Errorf("BuildProofPage should say 'Nothing changed.' when no changes, got %s", page.Lines[0])
	}
}

func TestEngine_BuildProofPage_Added(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	delta := domain.CoverageDelta{
		Added:   []domain.CoverageCapability{domain.CapReceiptObserver},
		Removed: []domain.CoverageCapability{},
	}
	delta.DeltaHash = delta.ComputeDeltaHash()

	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		PlanHash:     "abc123",
	}

	page := engine.BuildProofPage("circle-hash", nil, next, delta)

	if len(page.Added) != 1 {
		t.Errorf("BuildProofPage should have 1 added, got %d", len(page.Added))
	}
	if page.Lines[0] != "Coverage widened quietly." {
		t.Errorf("BuildProofPage should say 'Coverage widened quietly.' got %s", page.Lines[0])
	}
}

func TestEngine_BuildProofPage_NoPackIDs(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	delta := domain.CoverageDelta{
		Added: []domain.CoverageCapability{domain.CapReceiptObserver},
	}
	delta.DeltaHash = delta.ComputeDeltaHash()

	next := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		PlanHash:     "abc123",
	}

	page := engine.BuildProofPage("circle-hash", nil, next, delta)

	// Check that added items are display labels, not pack IDs
	for _, added := range page.Added {
		if strings.Contains(added, "pack") || strings.Contains(added, "core-") {
			t.Errorf("Proof page should not contain pack IDs: %s", added)
		}
	}
}

// =============================================================================
// Section 6: Engine Tests - Cue
// =============================================================================

func TestEngine_BuildCue_Available(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	page := domain.CoverageProofPage{
		Added:    []string{"Receipt scanning"},
		PlanHash: "abc123",
	}

	cue := engine.BuildCue(page, false)

	if !cue.Available {
		t.Error("Cue should be available when there are additions and not acked")
	}
	if cue.Text == "" {
		t.Error("Cue should have text when available")
	}
}

func TestEngine_BuildCue_NotAvailableWhenAcked(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	page := domain.CoverageProofPage{
		Added:    []string{"Receipt scanning"},
		PlanHash: "abc123",
	}

	cue := engine.BuildCue(page, true) // acked

	if cue.Available {
		t.Error("Cue should not be available when acked")
	}
}

func TestEngine_BuildCue_NotAvailableWhenNoAdditions(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	page := domain.CoverageProofPage{
		Added:    []string{},
		PlanHash: "abc123",
	}

	cue := engine.BuildCue(page, false)

	if cue.Available {
		t.Error("Cue should not be available when no additions")
	}
}

func TestEngine_ShouldShowCue_Priorities(t *testing.T) {
	engine := coverageplan.NewEngine(fixedClock)

	// Coverage cue available, no other cues
	if !engine.ShouldShowCue(true, false, false, false, false) {
		t.Error("Should show coverage cue when available and no other cues")
	}

	// Coverage cue blocked by shadow receipt
	if engine.ShouldShowCue(true, true, false, false, false) {
		t.Error("Coverage cue should be blocked by shadow receipt cue")
	}

	// Coverage cue blocked by reality
	if engine.ShouldShowCue(true, false, true, false, false) {
		t.Error("Coverage cue should be blocked by reality cue")
	}

	// Coverage cue blocked by first minutes
	if engine.ShouldShowCue(true, false, false, true, false) {
		t.Error("Coverage cue should be blocked by first minutes cue")
	}
}

// =============================================================================
// Section 7: Store Tests
// =============================================================================

func TestCoveragePlanStore_AppendAndRetrieve(t *testing.T) {
	store := persist.NewCoveragePlanStore(fixedTime)

	plan := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver},
		PlanHash:     "plan-hash-1",
	}

	err := store.AppendPlan(plan)
	if err != nil {
		t.Errorf("AppendPlan error = %v", err)
	}

	retrieved, found := store.LastPlan("circle-hash")
	if !found {
		t.Error("LastPlan should find the plan")
	}
	if retrieved.PlanHash != plan.PlanHash {
		t.Errorf("LastPlan hash = %s, want %s", retrieved.PlanHash, plan.PlanHash)
	}
}

func TestCoveragePlanStore_Dedup(t *testing.T) {
	store := persist.NewCoveragePlanStore(fixedTime)

	plan := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver},
		PlanHash:     "plan-hash-1",
	}

	_ = store.AppendPlan(plan)
	_ = store.AppendPlan(plan) // Duplicate

	if store.Count() != 1 {
		t.Errorf("Store should deduplicate, count = %d, want 1", store.Count())
	}
}

func TestCoverageProofAckStore_AppendAndCheck(t *testing.T) {
	store := persist.NewCoverageProofAckStore(fixedTime)

	ack := domain.CoverageProofAck{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		AckKind:      domain.AckDismissed,
		StatusHash:   "status-hash",
	}

	err := store.AppendAck(ack)
	if err != nil {
		t.Errorf("AppendAck error = %v", err)
	}

	if !store.IsProofDismissed("circle-hash", "2024-01-15") {
		t.Error("IsProofDismissed should return true after dismissal")
	}
}

func TestCoverageProofAckStore_NotDismissedDifferentPeriod(t *testing.T) {
	store := persist.NewCoverageProofAckStore(fixedTime)

	ack := domain.CoverageProofAck{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		AckKind:      domain.AckDismissed,
		StatusHash:   "status-hash",
	}

	_ = store.AppendAck(ack)

	// Different period should not be dismissed
	if store.IsProofDismissed("circle-hash", "2024-01-16") {
		t.Error("IsProofDismissed should return false for different period")
	}
}

// =============================================================================
// Section 8: Hash Tests
// =============================================================================

func TestComputePlanHash_Deterministic(t *testing.T) {
	plan := domain.CoveragePlan{
		CircleIDHash: "circle-hash",
		PeriodKey:    "2024-01-15",
		Sources:      []domain.CoverageSourcePlan{},
		Capabilities: []domain.CoverageCapability{domain.CapReceiptObserver},
	}

	hash1 := plan.ComputePlanHash()
	hash2 := plan.ComputePlanHash()

	if hash1 != hash2 {
		t.Error("ComputePlanHash should be deterministic")
	}
}

func TestComputeDeltaHash_Deterministic(t *testing.T) {
	delta := domain.CoverageDelta{
		Added:   []domain.CoverageCapability{domain.CapReceiptObserver},
		Removed: []domain.CoverageCapability{},
	}

	hash1 := delta.ComputeDeltaHash()
	hash2 := delta.ComputeDeltaHash()

	if hash1 != hash2 {
		t.Error("ComputeDeltaHash should be deterministic")
	}
}

// =============================================================================
// Section 9: Display Label Tests
// =============================================================================

func TestCoverageCapability_DisplayLabel(t *testing.T) {
	tests := []struct {
		cap      domain.CoverageCapability
		expected string
	}{
		{domain.CapReceiptObserver, "Receipt scanning"},
		{domain.CapCommerceObserver, "Commerce patterns"},
		{domain.CapFinanceCommerceObserver, "Finance observation"},
		{domain.CapPressureMap, "Pressure mapping"},
		{domain.CapTimeWindowSources, "Time-window analysis"},
		{domain.CapNotificationMetadata, "Notification metadata"},
	}

	for _, tt := range tests {
		label := tt.cap.DisplayLabel()
		if label != tt.expected {
			t.Errorf("DisplayLabel(%s) = %s, want %s", tt.cap, label, tt.expected)
		}
	}
}

// =============================================================================
// Section 10: HTTP Handler Tests (using httptest)
// =============================================================================

func TestCoverageProofHandler_MethodNotAllowed(t *testing.T) {
	// Create a simple handler that checks method
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/proof/coverage", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST, got %d", rec.Code)
	}
}

func TestCoverageProofDismissHandler_PostOnly(t *testing.T) {
	// Create a simple handler that checks method
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.Redirect(w, r, "/today", http.StatusSeeOther)
	})

	req := httptest.NewRequest(http.MethodGet, "/proof/coverage/dismiss", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET on dismiss, got %d", rec.Code)
	}
}

// =============================================================================
// Section 11: Plan HasCapability Tests
// =============================================================================

func TestCoveragePlan_HasCapability(t *testing.T) {
	plan := domain.CoveragePlan{
		Capabilities: []domain.CoverageCapability{
			domain.CapReceiptObserver,
			domain.CapCommerceObserver,
		},
	}

	if !plan.HasCapability(domain.CapReceiptObserver) {
		t.Error("HasCapability should return true for existing cap")
	}
	if !plan.HasCapability(domain.CapCommerceObserver) {
		t.Error("HasCapability should return true for existing cap")
	}
	if plan.HasCapability(domain.CapPressureMap) {
		t.Error("HasCapability should return false for non-existing cap")
	}
}

// =============================================================================
// Section 12: Delta Helper Tests
// =============================================================================

func TestCoverageDelta_IsEmpty(t *testing.T) {
	empty := domain.CoverageDelta{
		Added:     []domain.CoverageCapability{},
		Removed:   []domain.CoverageCapability{},
		Unchanged: []domain.CoverageCapability{domain.CapReceiptObserver},
	}

	if !empty.IsEmpty() {
		t.Error("IsEmpty should return true when no adds/removes")
	}

	notEmpty := domain.CoverageDelta{
		Added: []domain.CoverageCapability{domain.CapReceiptObserver},
	}

	if notEmpty.IsEmpty() {
		t.Error("IsEmpty should return false when there are additions")
	}
}

func TestCoverageDelta_HasAdditions(t *testing.T) {
	delta := domain.CoverageDelta{
		Added: []domain.CoverageCapability{domain.CapReceiptObserver},
	}

	if !delta.HasAdditions() {
		t.Error("HasAdditions should return true")
	}

	delta.Added = []domain.CoverageCapability{}
	if delta.HasAdditions() {
		t.Error("HasAdditions should return false when no additions")
	}
}

func TestCoverageDelta_HasRemovals(t *testing.T) {
	delta := domain.CoverageDelta{
		Removed: []domain.CoverageCapability{domain.CapReceiptObserver},
	}

	if !delta.HasRemovals() {
		t.Error("HasRemovals should return true")
	}

	delta.Removed = []domain.CoverageCapability{}
	if delta.HasRemovals() {
		t.Error("HasRemovals should return false when no removals")
	}
}

// =============================================================================
// Section 13: All Capabilities List
// =============================================================================

func TestAllCoverageCapabilities(t *testing.T) {
	caps := domain.AllCoverageCapabilities()

	if len(caps) != 6 {
		t.Errorf("AllCoverageCapabilities should return 6 caps, got %d", len(caps))
	}

	// Check sorted
	for i := 1; i < len(caps); i++ {
		if caps[i-1] > caps[i] {
			t.Error("AllCoverageCapabilities should be sorted")
		}
	}
}

func TestAllCoverageSourceKinds(t *testing.T) {
	kinds := domain.AllCoverageSourceKinds()

	if len(kinds) != 3 {
		t.Errorf("AllCoverageSourceKinds should return 3 kinds, got %d", len(kinds))
	}

	// Check sorted
	for i := 1; i < len(kinds); i++ {
		if kinds[i-1] > kinds[i] {
			t.Error("AllCoverageSourceKinds should be sorted")
		}
	}
}
