// Package demo_phase28_trust_kept provides demo tests for Phase 28: Trust Kept.
//
// These tests verify the critical safety invariants:
//   - Only calendar_respond action allowed
//   - Single execution per period
//   - 15-minute undo window
//   - Hash-only storage
//   - Silence after completion
//   - No re-invitation
//   - Deterministic hashing
//
// Reference: docs/ADR/ADR-0059-phase28-trust-kept.md
package demo_phase28_trust_kept

import (
	"testing"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/trustaction"
)

// testClock returns a deterministic clock for testing.
func testClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestDomainModelTypes verifies domain model types are correctly defined.
func TestDomainModelTypes(t *testing.T) {
	// Verify TrustActionKind
	if trustaction.ActionKindCalendarRespond != "calendar_respond" {
		t.Errorf("ActionKindCalendarRespond = %q, want %q", trustaction.ActionKindCalendarRespond, "calendar_respond")
	}

	// Verify TrustActionState values
	states := []trustaction.TrustActionState{
		trustaction.StateEligible,
		trustaction.StateExecuted,
		trustaction.StateUndone,
		trustaction.StateExpired,
	}
	for _, s := range states {
		if s == "" {
			t.Errorf("TrustActionState value is empty")
		}
	}

	// Verify HorizonBucket values
	horizons := []trustaction.HorizonBucket{
		trustaction.HorizonSoon,
		trustaction.HorizonLater,
		trustaction.HorizonSomeday,
	}
	for _, h := range horizons {
		if h == "" {
			t.Errorf("HorizonBucket value is empty")
		}
	}
}

// TestUndoBucketCreation verifies undo bucket 15-minute flooring.
func TestUndoBucketCreation(t *testing.T) {
	testCases := []struct {
		name           string
		inputTime      time.Time
		expectedMinute int
	}{
		{"At :00", time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), 0},
		{"At :07", time.Date(2025, 1, 15, 10, 7, 30, 0, time.UTC), 0},
		{"At :14", time.Date(2025, 1, 15, 10, 14, 59, 0, time.UTC), 0},
		{"At :15", time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC), 15},
		{"At :22", time.Date(2025, 1, 15, 10, 22, 0, 0, time.UTC), 15},
		{"At :30", time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), 30},
		{"At :45", time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC), 45},
		{"At :59", time.Date(2025, 1, 15, 10, 59, 59, 0, time.UTC), 45},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bucket := trustaction.NewUndoBucket(tc.inputTime)

			// Parse the bucket start time
			parsed, err := time.Parse(time.RFC3339, bucket.BucketStartRFC3339)
			if err != nil {
				t.Fatalf("Failed to parse bucket start: %v", err)
			}

			if parsed.Minute() != tc.expectedMinute {
				t.Errorf("Bucket minute = %d, want %d", parsed.Minute(), tc.expectedMinute)
			}

			if bucket.BucketDurationMinutes != 15 {
				t.Errorf("BucketDurationMinutes = %d, want 15", bucket.BucketDurationMinutes)
			}
		})
	}
}

// TestUndoBucketExpiry verifies undo window expiry logic.
func TestUndoBucketExpiry(t *testing.T) {
	bucketStart := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	bucket := trustaction.NewUndoBucket(bucketStart)

	testCases := []struct {
		name        string
		checkTime   time.Time
		wantExpired bool
	}{
		{"At bucket start", time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC), false},
		{"5 min after", time.Date(2025, 1, 15, 10, 5, 0, 0, time.UTC), false},
		{"14 min after", time.Date(2025, 1, 15, 10, 14, 59, 0, time.UTC), false},
		{"Exactly 15 min", time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC), false},
		{"15 min + 1 sec", time.Date(2025, 1, 15, 10, 15, 1, 0, time.UTC), true},
		{"20 min after", time.Date(2025, 1, 15, 10, 20, 0, 0, time.UTC), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expired := bucket.IsExpired(tc.checkTime)
			if expired != tc.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", expired, tc.wantExpired)
			}
		})
	}
}

// TestReceiptHashDeterminism verifies hash computation is deterministic.
func TestReceiptHashDeterminism(t *testing.T) {
	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		DraftIDHash:  "abc123",
		EnvelopeHash: "def456",
	}

	// Compute hash multiple times
	hash1 := receipt.ComputeStatusHash()
	hash2 := receipt.ComputeStatusHash()
	hash3 := receipt.ComputeStatusHash()

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash not deterministic: %s, %s, %s", hash1, hash2, hash3)
	}

	// Verify hash is not empty
	if len(hash1) != 32 {
		t.Errorf("Hash length = %d, want 32", len(hash1))
	}
}

// TestCanonicalStringFormat verifies canonical string format.
func TestCanonicalStringFormat(t *testing.T) {
	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		StatusHash:   "status123",
		DraftIDHash:  "abc123",
		EnvelopeHash: "def456",
	}

	canonical := receipt.CanonicalString()

	// Verify version prefix
	if canonical[:2] != "v1" {
		t.Errorf("Canonical string should start with v1, got: %s", canonical[:10])
	}

	// Verify pipe-delimited
	if canonical[2] != '|' {
		t.Errorf("Canonical string should be pipe-delimited")
	}
}

// TestPreviewCanonicalString verifies preview canonical string format.
func TestPreviewCanonicalString(t *testing.T) {
	preview := &trustaction.TrustActionPreview{
		ActionKind:     trustaction.ActionKindCalendarRespond,
		AbstractTarget: "a calendar event",
		HorizonBucket:  trustaction.HorizonSoon,
		Reversible:     true,
	}

	canonical := preview.CanonicalString()

	// Verify version prefix
	if canonical[:2] != "v1" {
		t.Errorf("Canonical string should start with v1, got: %s", canonical[:10])
	}

	// Verify pipe-delimited
	if canonical[2] != '|' {
		t.Errorf("Canonical string should be pipe-delimited")
	}
}

// TestStoreAppendAndRetrieve verifies store append and retrieve operations.
func TestStoreAppendAndRetrieve(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewTrustActionStore(testClock(now))

	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(now),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		DraftIDHash:  "abc123",
		EnvelopeHash: "def456",
	}
	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()

	// Append
	if err := store.AppendReceipt(receipt); err != nil {
		t.Fatalf("AppendReceipt failed: %v", err)
	}

	// Retrieve by ID
	retrieved := store.GetByID(receipt.ReceiptID)
	if retrieved == nil {
		t.Fatal("GetByID returned nil")
	}

	if retrieved.ReceiptID != receipt.ReceiptID {
		t.Errorf("ReceiptID mismatch: got %s, want %s", retrieved.ReceiptID, receipt.ReceiptID)
	}
}

// TestStoreSingleExecutionPerPeriod verifies single-shot enforcement.
func TestStoreSingleExecutionPerPeriod(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewTrustActionStore(testClock(now))

	receipt1 := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(now),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		DraftIDHash:  "abc123",
		EnvelopeHash: "def456",
	}
	receipt1.StatusHash = receipt1.ComputeStatusHash()
	receipt1.ReceiptID = receipt1.ComputeReceiptID()

	// First append should succeed
	if err := store.AppendReceipt(receipt1); err != nil {
		t.Fatalf("First AppendReceipt failed: %v", err)
	}

	// Check HasExecutedThisPeriod
	if !store.HasExecutedThisPeriod("circle_123", "2025-01-15") {
		t.Error("HasExecutedThisPeriod returned false after execution")
	}

	// Second append for same period should fail
	receipt2 := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(now),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		DraftIDHash:  "xyz789",
		EnvelopeHash: "uvw012",
	}
	receipt2.StatusHash = receipt2.ComputeStatusHash()
	receipt2.ReceiptID = receipt2.ComputeReceiptID()

	err := store.AppendReceipt(receipt2)
	if err == nil {
		t.Error("Second AppendReceipt should have failed")
	}
}

// TestStoreStateTransition verifies valid state transitions.
func TestStoreStateTransition(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewTrustActionStore(testClock(now))

	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(now),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		DraftIDHash:  "abc123",
		EnvelopeHash: "def456",
	}
	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()

	store.AppendReceipt(receipt)

	// Valid transition: executed -> undone
	if err := store.UpdateState(receipt.ReceiptID, trustaction.StateUndone); err != nil {
		t.Errorf("Valid transition to undone failed: %v", err)
	}

	// Verify state changed
	updated := store.GetByID(receipt.ReceiptID)
	if updated.State != trustaction.StateUndone {
		t.Errorf("State = %s, want %s", updated.State, trustaction.StateUndone)
	}
}

// TestCueCreation verifies cue creation.
func TestCueCreation(t *testing.T) {
	availableCue := trustaction.NewTrustActionCue(true)
	if !availableCue.Available {
		t.Error("Available cue should have Available=true")
	}
	if availableCue.CueText == "" {
		t.Error("Available cue should have CueText")
	}
	if availableCue.LinkText == "" {
		t.Error("Available cue should have LinkText")
	}
	if availableCue.CueHash == "" {
		t.Error("Available cue should have CueHash")
	}

	unavailableCue := trustaction.NewTrustActionCue(false)
	if unavailableCue.Available {
		t.Error("Unavailable cue should have Available=false")
	}
}

// TestHashStringFunction verifies hash string function.
func TestHashStringFunction(t *testing.T) {
	hash1 := trustaction.HashString("test")
	hash2 := trustaction.HashString("test")
	hash3 := trustaction.HashString("different")

	if hash1 != hash2 {
		t.Error("HashString not deterministic")
	}

	if hash1 == hash3 {
		t.Error("HashString should produce different hashes for different inputs")
	}

	if len(hash1) != 32 {
		t.Errorf("Hash length = %d, want 32", len(hash1))
	}
}

// TestNoIdentifiersInCanonicalString verifies no raw identifiers in canonical strings.
func TestNoIdentifiersInCanonicalString(t *testing.T) {
	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		StatusHash:   trustaction.HashString("status"),
		DraftIDHash:  trustaction.HashString("draft_id_12345"),
		EnvelopeHash: trustaction.HashString("envelope_abc"),
	}

	canonical := receipt.CanonicalString()

	// Check for forbidden patterns
	forbiddenPatterns := []string{
		"@",              // email
		"http://",        // URL
		"https://",       // URL
		"draft_id_12345", // raw draft ID
		"envelope_abc",   // raw envelope ID
	}

	for _, pattern := range forbiddenPatterns {
		if contains(canonical, pattern) {
			t.Errorf("Canonical string contains forbidden pattern: %s", pattern)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGetLatestForCircle verifies getting latest receipt for a circle.
func TestGetLatestForCircle(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	store := persist.NewTrustActionStore(testClock(now))

	// No receipts initially
	latest := store.GetLatestForCircle("circle_123")
	if latest != nil {
		t.Error("Should return nil when no receipts exist")
	}

	// Add a receipt
	receipt := &trustaction.TrustActionReceipt{
		ActionKind:   trustaction.ActionKindCalendarRespond,
		State:        trustaction.StateExecuted,
		UndoBucket:   trustaction.NewUndoBucket(now),
		Period:       "2025-01-15",
		CircleID:     "circle_123",
		DraftIDHash:  "abc123",
		EnvelopeHash: "def456",
	}
	receipt.StatusHash = receipt.ComputeStatusHash()
	receipt.ReceiptID = receipt.ComputeReceiptID()

	store.AppendReceipt(receipt)

	// Now should find it
	latest = store.GetLatestForCircle("circle_123")
	if latest == nil {
		t.Error("Should return receipt after append")
	}
	if latest.ReceiptID != receipt.ReceiptID {
		t.Errorf("ReceiptID mismatch: got %s, want %s", latest.ReceiptID, receipt.ReceiptID)
	}
}
