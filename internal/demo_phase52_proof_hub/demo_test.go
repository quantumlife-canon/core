// Package demo_phase52_proof_hub contains tests for Phase 52: Proof Hub + Connected Status.
//
// These tests verify the Phase 52 invariants:
// - NO POWER: Observation/proof only, no runtime behavior changes
// - HASH-ONLY: No raw identifiers stored or rendered
// - DETERMINISTIC: Same inputs = same outputs
// - NO TIMESTAMPS: Only recency buckets
// - NO COUNTS: Only magnitude buckets
//
// Reference: docs/ADR/ADR-0090-phase52-proof-hub-connected-status.md
package demo_phase52_proof_hub

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/proofhub"
	domain "quantumlife/pkg/domain/proofhub"
)

// ============================================================================
// Test Data
// ============================================================================

const (
	testCircleIDHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

type testClock struct {
	t time.Time
}

func (c *testClock) Now() time.Time {
	return c.t
}

func newTestClock() *testClock {
	return &testClock{t: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)}
}

func makeTestInputs(circleIDHash string, periodKey string) domain.ProofHubInputs {
	return domain.ProofHubInputs{
		CircleIDHash:               circleIDHash,
		NowPeriodKey:               periodKey,
		GmailConnected:             true,
		GmailLastSyncBucket:        domain.SyncRecent,
		GmailNoticedMagnitude:      domain.MagAFew,
		TrueLayerConnected:         true,
		TrueLayerLastSyncBucket:    domain.SyncRecent,
		TrueLayerNoticedMagnitude:  domain.MagAFew,
		ShadowProviderKind:         "stub",
		ShadowRealAllowed:          false,
		ShadowHealthStatus:         domain.StatusOK,
		DeviceRegistered:           true,
		TransparencyLinesMagnitude: domain.MagAFew,
		LastLedgerPeriodBucket:     domain.SyncRecent,
		EnforcementAuditRecent:     true,
		InterruptPolicyConfigured:  true,
	}
}

// ============================================================================
// Determinism Tests
// ============================================================================

func TestDeterminism_SameInputsSameClock_SameStatusHash(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs1 := makeTestInputs(testCircleIDHash, "2025-W03")
	inputs2 := makeTestInputs(testCircleIDHash, "2025-W03")

	page1 := engine.BuildPage(inputs1)
	page2 := engine.BuildPage(inputs2)

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("same inputs should produce same status hash: %s != %s", page1.StatusHash, page2.StatusHash)
	}
}

func TestDeterminism_SectionOrderingDeterministic(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")

	page1 := engine.BuildPage(inputs)
	page2 := engine.BuildPage(inputs)

	if len(page1.Sections) != len(page2.Sections) {
		t.Fatalf("section counts differ: %d != %d", len(page1.Sections), len(page2.Sections))
	}

	for i := range page1.Sections {
		if page1.Sections[i].Kind != page2.Sections[i].Kind {
			t.Errorf("section %d kind differs: %s != %s", i, page1.Sections[i].Kind, page2.Sections[i].Kind)
		}
	}
}

func TestDeterminism_BadgeOrderingDeterministic(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")

	page1 := engine.BuildPage(inputs)
	page2 := engine.BuildPage(inputs)

	for i, section := range page1.Sections {
		if len(section.Badges) != len(page2.Sections[i].Badges) {
			t.Errorf("section %d badge counts differ", i)
			continue
		}
		for j := range section.Badges {
			if section.Badges[j].Label != page2.Sections[i].Badges[j].Label {
				t.Errorf("section %d badge %d label differs: %s != %s",
					i, j, section.Badges[j].Label, page2.Sections[i].Badges[j].Label)
			}
		}
	}
}

// ============================================================================
// Privacy Tests
// ============================================================================

func TestPrivacy_NoForbiddenPatternsInPage(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	// Check canonical string for forbidden patterns
	canonical := page.CanonicalString()

	forbiddenPatterns := []string{"@", "http://", "https://", ".com", ".org", "$", "USD", "GBP", "EUR"}
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(canonical), strings.ToLower(pattern)) {
			t.Errorf("page canonical string contains forbidden pattern: %s", pattern)
		}
	}
}

func TestPrivacy_NoRawCircleIDInPage(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	// Use a recognizable raw ID
	rawCircleID := "my-personal-circle-name"
	inputs := makeTestInputs(rawCircleID, "2025-W03")
	page := engine.BuildPage(inputs)

	canonical := page.CanonicalString()

	// Raw ID should not appear in output - only hash
	if strings.Contains(canonical, rawCircleID) {
		t.Error("raw circle ID found in page output")
	}
}

func TestPrivacy_NoTimestampsInPage(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	// Check all sections and badges for timestamp patterns
	for _, section := range page.Sections {
		for _, badge := range section.Badges {
			// Timestamps typically have colons (HH:MM) or dashes with specific patterns
			if strings.Contains(badge.Value, "2025-01-") || strings.Contains(badge.Value, ":") {
				t.Errorf("badge appears to contain timestamp: %s=%s", badge.Label, badge.Value)
			}
		}
	}
}

func TestPrivacy_NoCounts_OnlyBuckets(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	// Check that magnitude values are only buckets, not raw numbers
	validMagnitudes := map[string]bool{
		"mag_nothing": true, "mag_a_few": true, "mag_several": true,
	}

	for _, section := range page.Sections {
		for _, badge := range section.Badges {
			if badge.Kind == "magnitude" {
				if !validMagnitudes[badge.Value] {
					t.Errorf("badge has non-bucket magnitude value: %s", badge.Value)
				}
			}
		}
	}
}

// ============================================================================
// Cue/Ack Behavior Tests
// ============================================================================

func TestCue_ShownWhenNotDismissed(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	cue := engine.BuildCue(page, false)

	if !cue.Available {
		t.Error("cue should be available when not dismissed")
	}
	if cue.Text == "" {
		t.Error("cue should have text when available")
	}
}

func TestCue_HiddenAfterDismiss(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	cue := engine.BuildCue(page, true) // dismissed = true

	if cue.Available {
		t.Error("cue should not be available when dismissed")
	}
}

func TestCue_ReappearsIfStatusHashChanges(t *testing.T) {
	clk := newTestClock()
	store := persist.NewProofHubAckStore(clk.Now)
	engine := proofhub.New(clk, proofhub.WithAckReader(store))

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	// Dismiss current status
	store.RecordDismissed(testCircleIDHash, "2025-W03", page.StatusHash)

	// Should not show cue for same status
	if engine.ShouldShowCue(testCircleIDHash, "2025-W03", page.StatusHash) {
		t.Error("cue should not show for dismissed status")
	}

	// Should show cue for different status
	if !engine.ShouldShowCue(testCircleIDHash, "2025-W03", "different_hash") {
		t.Error("cue should show for different status hash")
	}
}

func TestDismiss_Idempotent(t *testing.T) {
	clk := newTestClock()
	store := persist.NewProofHubAckStore(clk.Now)

	// First dismiss
	wasNew1, err := store.RecordDismissed(testCircleIDHash, "2025-W03", "hash1")
	if err != nil {
		t.Fatalf("first dismiss failed: %v", err)
	}
	if !wasNew1 {
		t.Error("first dismiss should report as new")
	}

	// Second dismiss (same params)
	wasNew2, err := store.RecordDismissed(testCircleIDHash, "2025-W03", "hash1")
	if err != nil {
		t.Fatalf("second dismiss failed: %v", err)
	}
	if wasNew2 {
		t.Error("second dismiss should not report as new")
	}

	// Count should be 1
	if store.Count() != 1 {
		t.Errorf("store should have 1 entry, got %d", store.Count())
	}
}

// ============================================================================
// Bucketing Tests
// ============================================================================

func TestBucketing_MissingGmailConnection(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	inputs.GmailConnected = false

	page := engine.BuildPage(inputs)

	// Find connections section
	var connectionsSection *domain.ProofHubSection
	for _, s := range page.Sections {
		if s.Kind == domain.SectionConnections {
			connectionsSection = &s
			break
		}
	}

	if connectionsSection == nil {
		t.Fatal("connections section not found")
	}

	// Find Gmail badge
	var gmailBadge *domain.ProofHubBadge
	for _, b := range connectionsSection.Badges {
		if b.Label == "Gmail" {
			gmailBadge = &b
			break
		}
	}

	if gmailBadge == nil {
		t.Fatal("Gmail badge not found")
	}

	if gmailBadge.Value != "connect_no" {
		t.Errorf("Gmail badge should show connect_no when not connected, got %s", gmailBadge.Value)
	}
}

func TestBucketing_MissingSync_SyncNever(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	inputs.GmailLastSyncBucket = domain.SyncNever

	page := engine.BuildPage(inputs)

	// Find sync section
	var syncSection *domain.ProofHubSection
	for _, s := range page.Sections {
		if s.Kind == domain.SectionSync {
			syncSection = &s
			break
		}
	}

	if syncSection == nil {
		t.Fatal("sync section not found")
	}

	// Check Gmail sync badge
	found := false
	for _, b := range syncSection.Badges {
		if b.Label == "Gmail sync" && b.Value == "sync_never" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Gmail sync badge should show sync_never")
	}
}

func TestBucketing_ShadowProviderUnknown(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	inputs.ShadowProviderKind = "unknown"

	page := engine.BuildPage(inputs)

	// Find shadow section
	var shadowSection *domain.ProofHubSection
	for _, s := range page.Sections {
		if s.Kind == domain.SectionShadow {
			shadowSection = &s
			break
		}
	}

	if shadowSection == nil {
		t.Fatal("shadow section not found")
	}

	// Check Provider badge
	found := false
	for _, b := range shadowSection.Badges {
		if b.Label == "Provider" && b.Value == "unknown" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Provider badge should show unknown")
	}
}

func TestBucketing_LedgerEmpty_MagnitudeNothing(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	inputs.TransparencyLinesMagnitude = domain.MagNothing

	page := engine.BuildPage(inputs)

	// Find ledger section
	var ledgerSection *domain.ProofHubSection
	for _, s := range page.Sections {
		if s.Kind == domain.SectionLedger {
			ledgerSection = &s
			break
		}
	}

	if ledgerSection == nil {
		t.Fatal("ledger section not found")
	}

	// Check magnitude badge
	found := false
	for _, b := range ledgerSection.Badges {
		if b.Label == "Ledger entries" && b.Value == "mag_nothing" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Ledger entries badge should show mag_nothing when empty")
	}
}

// ============================================================================
// Store Tests
// ============================================================================

func TestStore_FIFOEvictionAfterMaxRecords(t *testing.T) {
	clk := newTestClock()
	store := persist.NewProofHubAckStore(clk.Now)

	// Add MaxEntries + 1 entries
	for i := 0; i <= persist.ProofHubAckMaxEntries; i++ {
		hash := strings.Repeat("a", 64)[:60] + strings.Replace(
			strings.Repeat("0", 4), "", string(rune('0'+i%10)), 1)
		store.RecordDismissed(testCircleIDHash, "2025-W03", hash+string(rune('a'+i%26)))
	}

	// Should not exceed max
	if store.Count() > persist.ProofHubAckMaxEntries {
		t.Errorf("store should not exceed max entries (%d), got %d",
			persist.ProofHubAckMaxEntries, store.Count())
	}
}

func TestStore_RetentionRespects30Days(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	store := persist.NewProofHubAckStore(func() time.Time { return now })

	// Add entry
	store.RecordDismissed(testCircleIDHash, "2025-W01", "hash1")

	// Advance clock past retention
	now = now.AddDate(0, 0, 31)

	// Add another entry to trigger eviction
	store.RecordDismissed(testCircleIDHash, "2025-W05", "hash2")

	// Old entry should be evicted
	if store.IsDismissed(testCircleIDHash, "2025-W01", "hash1") {
		t.Error("old entry should have been evicted")
	}
}

// ============================================================================
// Page Structure Tests
// ============================================================================

func TestPage_HasTitle(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	if page.Title == "" {
		t.Error("page should have a title")
	}
	if page.Title != "Proof, quietly." {
		t.Errorf("page title should be 'Proof, quietly.', got %s", page.Title)
	}
}

func TestPage_HasPeriodKey(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	if page.PeriodKey == "" {
		t.Error("page should have a period key")
	}
	if page.PeriodKey != "2025-W03" {
		t.Errorf("page period key should be '2025-W03', got %s", page.PeriodKey)
	}
}

func TestPage_HasStatusHash(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	if page.StatusHash == "" {
		t.Error("page should have a status hash")
	}
	if len(page.StatusHash) != 64 {
		t.Errorf("status hash should be 64 chars (SHA256), got %d", len(page.StatusHash))
	}
}

func TestPage_Has6Sections(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	if len(page.Sections) != 6 {
		t.Errorf("page should have 6 sections, got %d", len(page.Sections))
	}
}

func TestPage_InvariantsSectionHasCorrectLines(t *testing.T) {
	clk := newTestClock()
	engine := proofhub.New(clk)

	inputs := makeTestInputs(testCircleIDHash, "2025-W03")
	page := engine.BuildPage(inputs)

	// Find invariants section
	var invariantsSection *domain.ProofHubSection
	for _, s := range page.Sections {
		if s.Kind == domain.SectionInvariants {
			invariantsSection = &s
			break
		}
	}

	if invariantsSection == nil {
		t.Fatal("invariants section not found")
	}

	expectedLines := []string{
		"No background execution.",
		"Hash-only storage.",
		"Silence is default.",
	}

	if len(invariantsSection.Lines) != len(expectedLines) {
		t.Errorf("invariants section should have %d lines, got %d",
			len(expectedLines), len(invariantsSection.Lines))
	}

	for i, expected := range expectedLines {
		if i < len(invariantsSection.Lines) && invariantsSection.Lines[i] != expected {
			t.Errorf("invariants line %d: expected %q, got %q",
				i, expected, invariantsSection.Lines[i])
		}
	}
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestValidation_InputsRequireCircleIDHash(t *testing.T) {
	inputs := domain.ProofHubInputs{
		CircleIDHash:               "",
		NowPeriodKey:               "2025-W03",
		GmailLastSyncBucket:        domain.SyncNever,
		GmailNoticedMagnitude:      domain.MagNothing,
		TrueLayerLastSyncBucket:    domain.SyncNever,
		TrueLayerNoticedMagnitude:  domain.MagNothing,
		ShadowHealthStatus:         domain.StatusUnknown,
		TransparencyLinesMagnitude: domain.MagNothing,
		LastLedgerPeriodBucket:     domain.SyncNever,
	}

	if err := inputs.Validate(); err == nil {
		t.Error("inputs with empty CircleIDHash should fail validation")
	}
}

func TestValidation_InputsRequirePeriodKey(t *testing.T) {
	inputs := domain.ProofHubInputs{
		CircleIDHash:               testCircleIDHash,
		NowPeriodKey:               "",
		GmailLastSyncBucket:        domain.SyncNever,
		GmailNoticedMagnitude:      domain.MagNothing,
		TrueLayerLastSyncBucket:    domain.SyncNever,
		TrueLayerNoticedMagnitude:  domain.MagNothing,
		ShadowHealthStatus:         domain.StatusUnknown,
		TransparencyLinesMagnitude: domain.MagNothing,
		LastLedgerPeriodBucket:     domain.SyncNever,
	}

	if err := inputs.Validate(); err == nil {
		t.Error("inputs with empty NowPeriodKey should fail validation")
	}
}

func TestValidation_PeriodKeyCannotContainPipe(t *testing.T) {
	inputs := makeTestInputs(testCircleIDHash, "2025|W03")

	if err := inputs.Validate(); err == nil {
		t.Error("inputs with pipe in period key should fail validation")
	}
}

// ============================================================================
// Enum Tests
// ============================================================================

func TestEnum_ProviderStatusValidation(t *testing.T) {
	valid := []domain.ProviderStatus{
		domain.StatusUnknown, domain.StatusOK, domain.StatusMissing, domain.StatusError,
	}

	for _, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("ProviderStatus %s should be valid: %v", s, err)
		}
	}

	invalid := domain.ProviderStatus("invalid_status")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid ProviderStatus should fail validation")
	}
}

func TestEnum_ConnectStatusValidation(t *testing.T) {
	if err := domain.ConnectNo.Validate(); err != nil {
		t.Errorf("ConnectNo should be valid: %v", err)
	}
	if err := domain.ConnectYes.Validate(); err != nil {
		t.Errorf("ConnectYes should be valid: %v", err)
	}

	invalid := domain.ConnectStatus("maybe")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid ConnectStatus should fail validation")
	}
}

func TestEnum_SyncRecencyBucketValidation(t *testing.T) {
	valid := []domain.SyncRecencyBucket{
		domain.SyncNever, domain.SyncRecent, domain.SyncStale,
	}

	for _, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("SyncRecencyBucket %s should be valid: %v", s, err)
		}
	}

	invalid := domain.SyncRecencyBucket("sync_yesterday")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid SyncRecencyBucket should fail validation")
	}
}

func TestEnum_MagnitudeBucketValidation(t *testing.T) {
	valid := []domain.MagnitudeBucket{
		domain.MagNothing, domain.MagAFew, domain.MagSeveral,
	}

	for _, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("MagnitudeBucket %s should be valid: %v", s, err)
		}
	}

	invalid := domain.MagnitudeBucket("exactly_42")
	if err := invalid.Validate(); err == nil {
		t.Error("invalid MagnitudeBucket should fail validation")
	}
}

func TestEnum_ComputeMagnitudeBucket(t *testing.T) {
	tests := []struct {
		count    int
		expected domain.MagnitudeBucket
	}{
		{0, domain.MagNothing},
		{1, domain.MagAFew},
		{2, domain.MagAFew},
		{3, domain.MagAFew},
		{4, domain.MagSeveral},
		{12, domain.MagSeveral},
		{100, domain.MagSeveral},
	}

	for _, tc := range tests {
		got := domain.ComputeMagnitudeBucket(tc.count)
		if got != tc.expected {
			t.Errorf("ComputeMagnitudeBucket(%d) = %s, want %s", tc.count, got, tc.expected)
		}
	}
}
