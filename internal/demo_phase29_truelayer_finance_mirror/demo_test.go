// Package demo_phase29_truelayer_finance_mirror contains tests for Phase 29.
//
// Phase 29: TrueLayer Read-Only Connect (UK Sandbox) + Finance Mirror Proof
// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
//
// CRITICAL INVARIANTS tested:
//   - Determinism: same seed+clock => same hashes
//   - Privacy guard blocks identifiable patterns
//   - Mirror page contains only abstract buckets
//   - Sync receipts persisted and replayed correctly
//   - No raw data in any output
package demo_phase29_truelayer_finance_mirror

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/financemirror"
)

// TestDomainModelTypes verifies all domain types exist and have correct structure.
func TestDomainModelTypes(t *testing.T) {
	// Test MagnitudeBucket constants exist
	buckets := []financemirror.MagnitudeBucket{
		financemirror.MagnitudeNothing,
		financemirror.MagnitudeAFew,
		financemirror.MagnitudeSeveral,
		financemirror.MagnitudeMany,
	}
	for _, b := range buckets {
		if b == "" {
			t.Error("MagnitudeBucket should not be empty")
		}
	}

	// Test CategoryBucket constants exist
	categories := financemirror.AllCategories()
	if len(categories) != 4 {
		t.Errorf("expected 4 categories, got %d", len(categories))
	}
}

// TestMagnitudeBucketConversion verifies count to bucket conversion.
func TestMagnitudeBucketConversion(t *testing.T) {
	tests := []struct {
		count    int
		expected financemirror.MagnitudeBucket
	}{
		{0, financemirror.MagnitudeNothing},
		{1, financemirror.MagnitudeAFew},
		{3, financemirror.MagnitudeAFew},
		{4, financemirror.MagnitudeSeveral},
		{10, financemirror.MagnitudeSeveral},
		{11, financemirror.MagnitudeMany},
		{100, financemirror.MagnitudeMany},
	}

	for _, tc := range tests {
		got := financemirror.ToMagnitudeBucket(tc.count)
		if got != tc.expected {
			t.Errorf("ToMagnitudeBucket(%d) = %s, want %s", tc.count, got, tc.expected)
		}
	}
}

// TestMagnitudeBucketDisplayText verifies display text is human-readable.
func TestMagnitudeBucketDisplayText(t *testing.T) {
	tests := []struct {
		bucket   financemirror.MagnitudeBucket
		expected string
	}{
		{financemirror.MagnitudeNothing, "nothing"},
		{financemirror.MagnitudeAFew, "a few"},
		{financemirror.MagnitudeSeveral, "several"},
		{financemirror.MagnitudeMany, "many"},
	}

	for _, tc := range tests {
		got := tc.bucket.DisplayText()
		if got != tc.expected {
			t.Errorf("%s.DisplayText() = %s, want %s", tc.bucket, got, tc.expected)
		}
	}
}

// TestFinanceSyncReceiptDeterminism verifies receipts are deterministic.
func TestFinanceSyncReceiptDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	circleID := "test-circle"
	provider := "truelayer"
	evidenceTokens := []string{"token1", "token2", "token3"}

	// Create two receipts with same inputs
	receipt1 := financemirror.NewFinanceSyncReceipt(
		circleID, provider, fixedTime,
		5, 10, evidenceTokens,
		true, "",
	)

	receipt2 := financemirror.NewFinanceSyncReceipt(
		circleID, provider, fixedTime,
		5, 10, evidenceTokens,
		true, "",
	)

	// Verify determinism
	if receipt1.ReceiptID != receipt2.ReceiptID {
		t.Errorf("ReceiptID should be deterministic: %s != %s", receipt1.ReceiptID, receipt2.ReceiptID)
	}
	if receipt1.StatusHash != receipt2.StatusHash {
		t.Errorf("StatusHash should be deterministic: %s != %s", receipt1.StatusHash, receipt2.StatusHash)
	}
	if receipt1.EvidenceHash != receipt2.EvidenceHash {
		t.Errorf("EvidenceHash should be deterministic: %s != %s", receipt1.EvidenceHash, receipt2.EvidenceHash)
	}
}

// TestFinanceSyncReceiptCanonicalString verifies canonical string format.
func TestFinanceSyncReceiptCanonicalString(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	receipt := financemirror.NewFinanceSyncReceipt(
		"test-circle", "truelayer", fixedTime,
		5, 10, nil,
		true, "",
	)

	canonical := receipt.CanonicalString()

	// Verify pipe-delimited format
	if canonical == "" {
		t.Error("CanonicalString should not be empty")
	}
	if canonical[0:2] != "v1" {
		t.Errorf("CanonicalString should start with v1, got %s", canonical[0:2])
	}

	// Verify no raw amounts in canonical string
	forbiddenPatterns := []string{"£", "$", "€", "GBP", "USD", "EUR"}
	for _, pattern := range forbiddenPatterns {
		if containsString(canonical, pattern) {
			t.Errorf("CanonicalString should not contain %s", pattern)
		}
	}
}

// TestFinanceSyncReceiptValidation verifies validation works.
func TestFinanceSyncReceiptValidation(t *testing.T) {
	// Valid receipt should pass
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	receipt := financemirror.NewFinanceSyncReceipt(
		"test-circle", "truelayer", fixedTime,
		5, 10, nil,
		true, "",
	)

	if err := receipt.Validate(); err != nil {
		t.Errorf("Valid receipt should pass validation: %v", err)
	}

	// Invalid receipt (empty circle ID) should fail
	invalidReceipt := &financemirror.FinanceSyncReceipt{
		ReceiptID: "test",
		Provider:  "truelayer",
	}
	if err := invalidReceipt.Validate(); err == nil {
		t.Error("Receipt with empty CircleID should fail validation")
	}
}

// TestFinanceMirrorPageCreation verifies page creation.
func TestFinanceMirrorPageCreation(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	categories := []financemirror.CategorySignal{
		{Category: financemirror.CategoryLiquidity, Magnitude: financemirror.MagnitudeAFew},
		{Category: financemirror.CategorySpendPattern, Magnitude: financemirror.MagnitudeSeveral},
	}

	page := financemirror.NewFinanceMirrorPage(
		true, fixedTime, financemirror.MagnitudeSeveral, categories,
	)

	// Verify page has correct structure
	if page.Title != "Seen, quietly." {
		t.Errorf("Page title should be 'Seen, quietly.', got %s", page.Title)
	}
	if page.Reassurance != financemirror.DefaultReassurance {
		t.Errorf("Page reassurance should be default, got %s", page.Reassurance)
	}
	if !page.Connected {
		t.Error("Page should show connected=true")
	}
	if page.StatusHash == "" {
		t.Error("Page should have StatusHash")
	}
}

// TestFinanceMirrorPageCalmLines verifies calm line selection.
func TestFinanceMirrorPageCalmLines(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		magnitude    financemirror.MagnitudeBucket
		expectedPart string
	}{
		{financemirror.MagnitudeNothing, "Nothing to see"},
		{financemirror.MagnitudeAFew, "A few things"},
		{financemirror.MagnitudeSeveral, "Several things"},
		{financemirror.MagnitudeMany, "Many things"},
	}

	for _, tc := range tests {
		page := financemirror.NewFinanceMirrorPage(
			true, fixedTime, tc.magnitude, nil,
		)
		if !containsString(page.CalmLine, tc.expectedPart) {
			t.Errorf("CalmLine for %s should contain '%s', got %s",
				tc.magnitude, tc.expectedPart, page.CalmLine)
		}
	}
}

// TestFinanceMirrorPageDeterminism verifies page is deterministic.
func TestFinanceMirrorPageDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	categories := []financemirror.CategorySignal{
		{Category: financemirror.CategoryLiquidity, Magnitude: financemirror.MagnitudeAFew},
	}

	page1 := financemirror.NewFinanceMirrorPage(true, fixedTime, financemirror.MagnitudeAFew, categories)
	page2 := financemirror.NewFinanceMirrorPage(true, fixedTime, financemirror.MagnitudeAFew, categories)

	if page1.StatusHash != page2.StatusHash {
		t.Errorf("Page StatusHash should be deterministic: %s != %s",
			page1.StatusHash, page2.StatusHash)
	}
}

// TestFinanceMirrorAck verifies acknowledgment creation.
func TestFinanceMirrorAck(t *testing.T) {
	ack := financemirror.NewFinanceMirrorAck("test-circle", "2025-01-15", "page-hash-123")

	if ack.CircleID != "test-circle" {
		t.Errorf("CircleID should be test-circle, got %s", ack.CircleID)
	}
	if ack.PeriodBucket != "2025-01-15" {
		t.Errorf("PeriodBucket should be 2025-01-15, got %s", ack.PeriodBucket)
	}
	if ack.AckHash == "" {
		t.Error("AckHash should not be empty")
	}
}

// TestFinanceMirrorCue verifies cue creation.
func TestFinanceMirrorCue(t *testing.T) {
	// Available cue
	cue := financemirror.NewFinanceMirrorCue(true)
	if !cue.Available {
		t.Error("Cue should be available")
	}
	if cue.CueText == "" {
		t.Error("CueText should not be empty")
	}
	if cue.LinkText == "" {
		t.Error("LinkText should not be empty")
	}
	if cue.CueHash == "" {
		t.Error("CueHash should not be empty")
	}

	// Unavailable cue
	unavailableCue := financemirror.NewFinanceMirrorCue(false)
	if unavailableCue.Available {
		t.Error("Cue should not be available")
	}
}

// TestFinanceMirrorStoreBasicOperations verifies store operations.
func TestFinanceMirrorStoreBasicOperations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	store := persist.NewFinanceMirrorStore(clock)

	// Create and store receipt
	receipt := financemirror.NewFinanceSyncReceipt(
		"test-circle", "truelayer", fixedTime,
		5, 10, nil,
		true, "",
	)

	err := store.StoreSyncReceipt(receipt)
	if err != nil {
		t.Fatalf("Failed to store receipt: %v", err)
	}

	// Retrieve by ID
	retrieved := store.GetSyncReceipt(receipt.ReceiptID)
	if retrieved == nil {
		t.Fatal("Failed to retrieve receipt by ID")
	}
	if retrieved.StatusHash != receipt.StatusHash {
		t.Error("Retrieved receipt should match stored receipt")
	}

	// Retrieve latest
	latest := store.GetLatestSyncReceipt("test-circle")
	if latest == nil {
		t.Fatal("Failed to retrieve latest receipt")
	}
	if latest.ReceiptID != receipt.ReceiptID {
		t.Error("Latest receipt should be the one we stored")
	}
}

// TestFinanceMirrorStoreIdempotency verifies store is idempotent.
func TestFinanceMirrorStoreIdempotency(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	store := persist.NewFinanceMirrorStore(clock)

	receipt := financemirror.NewFinanceSyncReceipt(
		"test-circle", "truelayer", fixedTime,
		5, 10, nil,
		true, "",
	)

	// Store twice
	_ = store.StoreSyncReceipt(receipt)
	_ = store.StoreSyncReceipt(receipt)

	// Should only have one
	if store.Count() != 1 {
		t.Errorf("Store should have 1 receipt, got %d", store.Count())
	}
}

// TestFinanceMirrorStoreAckOperations verifies ack operations.
func TestFinanceMirrorStoreAckOperations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	store := persist.NewFinanceMirrorStore(clock)

	ack := financemirror.NewFinanceMirrorAck("test-circle", "2025-01-15", "page-hash-123")

	// Store ack
	err := store.StoreAck(ack)
	if err != nil {
		t.Fatalf("Failed to store ack: %v", err)
	}

	// Check if acked
	if !store.IsAcked("test-circle", "2025-01-15") {
		t.Error("Should be acked")
	}

	// Check non-existent ack
	if store.IsAcked("test-circle", "2025-01-16") {
		t.Error("Should not be acked for different period")
	}
}

// TestFinanceMirrorStoreConnectionOperations verifies connection operations.
func TestFinanceMirrorStoreConnectionOperations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }
	store := persist.NewFinanceMirrorStore(clock)

	// Initially not connected
	if store.HasConnection("test-circle") {
		t.Error("Should not have connection initially")
	}

	// Set connection
	store.SetConnectionHash("test-circle", "conn-hash-123")

	// Now connected
	if !store.HasConnection("test-circle") {
		t.Error("Should have connection after setting")
	}

	// Get connection hash
	hash := store.GetConnectionHash("test-circle")
	if hash != "conn-hash-123" {
		t.Errorf("Connection hash should be conn-hash-123, got %s", hash)
	}

	// Remove connection
	store.RemoveConnection("test-circle")
	if store.HasConnection("test-circle") {
		t.Error("Should not have connection after removal")
	}
}

// TestTimeBucketFloors verifies time is bucketed correctly.
func TestTimeBucketFloors(t *testing.T) {
	tests := []struct {
		input    time.Time
		expected time.Time
	}{
		{
			time.Date(2025, 1, 15, 10, 33, 45, 0, time.UTC),
			time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			time.Date(2025, 1, 15, 10, 34, 59, 0, time.UTC),
			time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
			time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		got := financemirror.TimeBucket(tc.input)
		if !got.Equal(tc.expected) {
			t.Errorf("TimeBucket(%v) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

// TestPeriodBucket verifies period bucket format.
func TestPeriodBucket(t *testing.T) {
	input := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	expected := "2025-01-15"

	got := financemirror.PeriodBucket(input)
	if got != expected {
		t.Errorf("PeriodBucket(%v) = %s, want %s", input, got, expected)
	}
}

// TestNoRawDataInCanonicalStrings verifies no raw data leaks.
func TestNoRawDataInCanonicalStrings(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	receipt := financemirror.NewFinanceSyncReceipt(
		"test-circle", "truelayer", fixedTime,
		5, 10, nil,
		true, "",
	)

	canonical := receipt.CanonicalString()

	// Check for forbidden patterns
	forbiddenPatterns := []string{
		"@",                   // emails
		"http://", "https://", // URLs
		"GB", // IBAN prefix
		"sort_code", "account_number",
		"merchant", "payee",
	}

	for _, pattern := range forbiddenPatterns {
		if containsString(canonical, pattern) {
			t.Errorf("CanonicalString should not contain %s: %s", pattern, canonical)
		}
	}
}

// containsString is a helper to check if a string contains a substring.
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
