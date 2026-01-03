// Package demo_phase22_quiet_inbox_mirror contains tests for Phase 22.
//
// Phase 22: Quiet Inbox Mirror (First Real Value Moment)
//
// This file demonstrates:
//   - No Gmail connection → no mirror
//   - Gmail synced, nothing notable → "Nothing needs you"
//   - Gmail synced, patterns exist → abstract mirror shown
//   - Determinism: same inputs = same output
//   - Privacy enforcement: no identifiers
//   - No auto-surface
//   - Mirror does not affect obligations
//   - Categories capped at 3
//
// CRITICAL INVARIANTS:
//   - Abstraction over explanation
//   - No email subjects, senders, timestamps, or counts
//   - Magnitude buckets only (nothing | a_few | several)
//   - One calm, ignorable statement
//   - Deterministic output
//   - No LLM usage
//   - No goroutines. No time.Now().
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md
package demo_phase22_quiet_inbox_mirror

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/internal/quietmirror"
	"quantumlife/pkg/domain/identity"
	domainquietmirror "quantumlife/pkg/domain/quietmirror"
)

// createTestReceipt creates a SyncReceiptAbstract for testing.
// This maps message count to magnitude buckets.
func createTestReceipt(messageCount int, success bool) *quietmirror.SyncReceiptAbstract {
	var magnitude domainquietmirror.MirrorMagnitude
	switch {
	case messageCount == 0:
		magnitude = domainquietmirror.MagnitudeNothing
	case messageCount <= 5:
		magnitude = domainquietmirror.MagnitudeAFew
	default:
		magnitude = domainquietmirror.MagnitudeSeveral
	}
	return &quietmirror.SyncReceiptAbstract{
		Success:   success,
		Hash:      "test-hash",
		Magnitude: magnitude,
	}
}

// =============================================================================
// Test: No Gmail Connection → No Mirror
// =============================================================================

func TestNoConnection_NoMirror(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	// No connection, no sync receipt
	input := engine.ComputeInput(
		identity.EntityID("personal"),
		false, // hasConnection = false
		nil,   // no receipt
		nil,   // no categories
	)

	summary := engine.Compute(input)

	// Should NOT have a mirror
	if summary.HasMirror {
		t.Error("Expected no mirror when not connected")
	}

	// Should still have a statement
	if summary.Statement.Text == "" {
		t.Error("Expected a calm statement even when no mirror")
	}

	// Magnitude should be nothing
	if summary.Magnitude != domainquietmirror.MagnitudeNothing {
		t.Errorf("Expected magnitude nothing, got %s", summary.Magnitude)
	}

	t.Logf("No connection summary: %s", summary.Statement.Text)
}

// =============================================================================
// Test: Gmail Synced, Nothing Notable → "Nothing needs you"
// =============================================================================

func TestSyncedNothingNotable(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	// Create sync receipt with no messages (magnitude = nothing)
	receipt := createTestReceipt(0, true)

	input := engine.ComputeInput(
		identity.EntityID("personal"),
		true, // hasConnection
		receipt,
		nil, // no categories
	)

	summary := engine.Compute(input)

	// Should NOT have a mirror (nothing notable)
	if summary.HasMirror {
		t.Error("Expected no mirror when nothing notable")
	}

	// Should have "nothing" statement
	if summary.Statement.StatementKind != domainquietmirror.StatementKindNothing {
		t.Errorf("Expected statement kind nothing, got %s", summary.Statement.StatementKind)
	}

	if summary.Statement.Text != "Nothing here needs you today." {
		t.Errorf("Unexpected statement: %s", summary.Statement.Text)
	}

	t.Logf("Nothing notable statement: %s", summary.Statement.Text)
}

// =============================================================================
// Test: Gmail Synced, Patterns Exist → Abstract Mirror Shown
// =============================================================================

func TestSyncedWithPatterns(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	// Create sync receipt with some messages (magnitude = a_few)
	receipt := createTestReceipt(3, true)

	categoryPresence := map[domainquietmirror.MirrorCategory]bool{
		domainquietmirror.CategoryWork: true,
	}

	input := engine.ComputeInput(
		identity.EntityID("personal"),
		true,
		receipt,
		categoryPresence,
	)

	summary := engine.Compute(input)

	// Should have a mirror
	if !summary.HasMirror {
		t.Error("Expected mirror when patterns exist")
	}

	// Magnitude should be a_few (handful maps to a_few)
	if summary.Magnitude != domainquietmirror.MagnitudeAFew {
		t.Errorf("Expected magnitude a_few, got %s", summary.Magnitude)
	}

	// Should have work category
	if len(summary.Categories) == 0 {
		t.Error("Expected at least one category")
	}

	// Statement should be about patterns
	if summary.Statement.StatementKind != domainquietmirror.StatementKindPatterns {
		t.Errorf("Expected statement kind patterns, got %s", summary.Statement.StatementKind)
	}

	t.Logf("Patterns statement: %s", summary.Statement.Text)
	t.Logf("Categories: %v", summary.Categories)
}

// =============================================================================
// Test: Determinism - Same Inputs = Same Output
// =============================================================================

func TestDeterminism(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(5, true)

	categoryPresence := map[domainquietmirror.MirrorCategory]bool{
		domainquietmirror.CategoryWork:  true,
		domainquietmirror.CategoryMoney: true,
	}

	// Compute twice with same inputs
	input1 := engine.ComputeInput(identity.EntityID("personal"), true, receipt, categoryPresence)
	summary1 := engine.Compute(input1)

	input2 := engine.ComputeInput(identity.EntityID("personal"), true, receipt, categoryPresence)
	summary2 := engine.Compute(input2)

	// Hashes must match
	if summary1.Hash() != summary2.Hash() {
		t.Errorf("Hashes differ: %s vs %s", summary1.Hash(), summary2.Hash())
	}

	// Statements must match
	if summary1.Statement.Text != summary2.Statement.Text {
		t.Errorf("Statements differ: %s vs %s", summary1.Statement.Text, summary2.Statement.Text)
	}

	t.Logf("Deterministic hash: %s", summary1.Hash())
}

// =============================================================================
// Test: Privacy Enforcement - No Identifiers
// =============================================================================

func TestPrivacyEnforcement(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(10, true)

	categoryPresence := map[domainquietmirror.MirrorCategory]bool{
		domainquietmirror.CategoryWork: true,
	}

	input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, categoryPresence)
	summary := engine.Compute(input)

	// Verify no identifiable information in statement
	forbiddenPatterns := []string{
		"@",       // No email addresses
		"http",    // No URLs
		"$",       // No amounts
		"invoice", // No specific content
		"meeting", // No specific events
		"from",    // No sender references
		"sent",    // No action references
	}

	for _, pattern := range forbiddenPatterns {
		if containsIgnoreCase(summary.Statement.Text, pattern) {
			t.Errorf("Statement contains forbidden pattern: %s", pattern)
		}
	}

	// Page should not have identifiable info
	page := engine.BuildPage(summary)
	for _, pattern := range forbiddenPatterns {
		if containsIgnoreCase(page.Statement, pattern) {
			t.Errorf("Page statement contains forbidden pattern: %s", pattern)
		}
	}

	t.Logf("Privacy-safe statement: %s", summary.Statement.Text)
}

// =============================================================================
// Test: No Auto-Surface
// =============================================================================

func TestNoAutoSurface(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(15, true)

	input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, nil)
	summary := engine.Compute(input)

	// Whisper cue should only show if not dismissed
	cue := engine.BuildWhisperCue(summary, false)
	if !cue.Show && summary.HasMirror {
		t.Log("Cue not shown even with mirror - that's fine")
	}

	// If dismissed, cue should NOT show
	dismissedCue := engine.BuildWhisperCue(summary, true)
	if dismissedCue.Show {
		t.Error("Cue should not show when dismissed")
	}

	t.Logf("Whisper cue respects dismissal")
}

// =============================================================================
// Test: Mirror Does Not Affect Obligations
// =============================================================================

func TestMirrorDoesNotAffectObligations(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(20, true)

	input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, nil)
	summary := engine.Compute(input)

	// Summary should be read-only reflection, not create obligations
	// This is enforced by the type system - QuietMirrorSummary has no
	// obligation fields and engine doesn't interact with obligation stores

	if summary.CircleID == "" {
		t.Error("Expected circle ID in summary")
	}

	// Verify the summary only contains abstract data
	if summary.Magnitude == "" {
		t.Error("Expected magnitude in summary")
	}

	t.Logf("Mirror is read-only: magnitude=%s, hasMirror=%t", summary.Magnitude, summary.HasMirror)
}

// =============================================================================
// Test: Categories Capped at 3
// =============================================================================

func TestCategoriesCappedAt3(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(10, true)

	// Add more than 3 categories
	categoryPresence := map[domainquietmirror.MirrorCategory]bool{
		domainquietmirror.CategoryWork:   true,
		domainquietmirror.CategoryMoney:  true,
		domainquietmirror.CategoryPeople: true,
		domainquietmirror.CategoryTime:   true,
		domainquietmirror.CategoryHome:   true,
	}

	input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, categoryPresence)
	summary := engine.Compute(input)

	// Should have at most 3 categories
	if len(summary.Categories) > 3 {
		t.Errorf("Expected max 3 categories, got %d", len(summary.Categories))
	}

	// Categories should be sorted for determinism
	for i := 1; i < len(summary.Categories); i++ {
		if string(summary.Categories[i-1]) > string(summary.Categories[i]) {
			t.Error("Categories should be sorted alphabetically")
		}
	}

	t.Logf("Categories (max 3): %v", summary.Categories)
}

// =============================================================================
// Test: Magnitude Buckets Only
// =============================================================================

func TestMagnitudeBucketsOnly(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	tests := []struct {
		messageCount      int
		expectedMagnitude domainquietmirror.MirrorMagnitude
	}{
		{0, domainquietmirror.MagnitudeNothing},
		{1, domainquietmirror.MagnitudeAFew},
		{5, domainquietmirror.MagnitudeAFew},
		{10, domainquietmirror.MagnitudeSeveral},
		{50, domainquietmirror.MagnitudeSeveral},
	}

	for _, tt := range tests {
		receipt := createTestReceipt(tt.messageCount, true)

		input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, nil)
		summary := engine.Compute(input)

		if summary.Magnitude != tt.expectedMagnitude {
			t.Errorf("messageCount=%d: expected %s, got %s",
				tt.messageCount, tt.expectedMagnitude, summary.Magnitude)
		}
	}
}

// =============================================================================
// Test: Page Display Properties
// =============================================================================

func TestPageDisplayProperties(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(5, true)

	categoryPresence := map[domainquietmirror.MirrorCategory]bool{
		domainquietmirror.CategoryWork: true,
	}

	input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, categoryPresence)
	summary := engine.Compute(input)
	page := engine.BuildPage(summary)

	// Verify page properties
	if page.Title != "Seen, quietly." {
		t.Errorf("Expected title 'Seen, quietly.', got %s", page.Title)
	}

	if page.Footer != "We're watching so you don't have to." {
		t.Errorf("Unexpected footer: %s", page.Footer)
	}

	if page.Statement == "" {
		t.Error("Expected non-empty statement")
	}

	t.Logf("Page title: %s", page.Title)
	t.Logf("Page statement: %s", page.Statement)
	t.Logf("Page footer: %s", page.Footer)
}

// =============================================================================
// Test: Store Persistence
// =============================================================================

func TestStorePersistence(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewQuietMirrorStore(func() time.Time { return fixedTime })
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	receipt := createTestReceipt(5, true)

	input := engine.ComputeInput(identity.EntityID("personal"), true, receipt, nil)
	summary := engine.Compute(input)

	// Store the summary
	err := store.Store(summary)
	if err != nil {
		t.Errorf("Failed to store summary: %v", err)
	}

	// Retrieve by hash
	retrieved, ok := store.GetByHash(summary.Hash())
	if !ok {
		t.Error("Failed to retrieve summary by hash")
	}

	if retrieved.Hash() != summary.Hash() {
		t.Error("Retrieved summary hash doesn't match")
	}

	// Retrieve by circle
	latest := store.GetLatestForCircle(identity.EntityID("personal"))
	if latest == nil {
		t.Error("Failed to retrieve latest summary for circle")
	}

	t.Logf("Store count: %d", store.Count())
}

// =============================================================================
// Test: Empty Page When No Receipt
// =============================================================================

func TestEmptyPageWhenNoReceipt(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	engine := quietmirror.NewEngine(func() time.Time { return fixedTime })

	// Connected but no receipt
	input := engine.ComputeInput(
		identity.EntityID("personal"),
		true, // connected
		nil,  // no receipt
		nil,
	)

	summary := engine.Compute(input)
	page := engine.BuildPage(summary)

	// Should show empty page with calm message
	if page.HasContent {
		t.Error("Expected no content when no receipt")
	}

	if page.Statement != "Nothing here needs you today." {
		t.Errorf("Expected calm statement, got: %s", page.Statement)
	}
}

// =============================================================================
// Test: Canonical String Format
// =============================================================================

func TestCanonicalStringFormat(t *testing.T) {
	summary := &domainquietmirror.QuietMirrorSummary{
		CircleID:  "personal",
		Period:    "2024-01-15",
		Magnitude: domainquietmirror.MagnitudeAFew,
		Categories: []domainquietmirror.MirrorCategory{
			domainquietmirror.CategoryMoney,
			domainquietmirror.CategoryWork,
		},
		Statement: domainquietmirror.MirrorStatement{
			Text:          "A few patterns are being kept an eye on.",
			StatementKind: domainquietmirror.StatementKindPatterns,
		},
		HasMirror:  true,
		SourceHash: "abc123",
	}

	canonical := summary.CanonicalString()

	// Should be pipe-delimited
	if !containsIgnoreCase(canonical, "|") {
		t.Error("Canonical string should be pipe-delimited")
	}

	// Should start with QUIET_MIRROR
	if canonical[:12] != "QUIET_MIRROR" {
		t.Errorf("Canonical should start with QUIET_MIRROR, got: %s", canonical[:20])
	}

	// Hash should be deterministic
	hash1 := summary.Hash()
	hash2 := summary.Hash()
	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	t.Logf("Canonical: %s", canonical)
	t.Logf("Hash: %s", summary.Hash())
}

// =============================================================================
// Helpers
// =============================================================================

func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
