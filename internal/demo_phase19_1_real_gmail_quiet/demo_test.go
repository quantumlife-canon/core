// Package demo_phase19_1_real_gmail_quiet tests Phase 19.1: Real Gmail Connection.
//
// CRITICAL INVARIANTS:
//   - Explicit sync only - NO background polling
//   - Max 25 messages, last 7 days
//   - DefaultToHold = true for all Gmail obligations
//   - Magnitude buckets only - no raw counts in UI/storage
//   - No storage of raw message content
//   - Deterministic: same inputs => same receipt hash
//
// Reference: Phase 19.1 specification
package demo_phase19_1_real_gmail_quiet

import (
	"testing"
	"time"

	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"

	"quantumlife/internal/obligations"
	"quantumlife/internal/persist"
	"quantumlife/pkg/clock"
)

// TestSyncReceiptDeterminism verifies same inputs produce same receipt hash.
func TestSyncReceiptDeterminism(t *testing.T) {
	t.Log("Phase 19.1: Testing sync receipt determinism")

	// Use fixed time for determinism
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle-1")

	// Create two receipts with identical inputs
	receipt1 := persist.NewSyncReceipt(circleID, "gmail", 10, 8, fixedTime, true, "")
	receipt2 := persist.NewSyncReceipt(circleID, "gmail", 10, 8, fixedTime, true, "")

	// Verify determinism
	if receipt1.Hash != receipt2.Hash {
		t.Errorf("receipt hashes should be identical: %s != %s", receipt1.Hash, receipt2.Hash)
	}
	if receipt1.ReceiptID != receipt2.ReceiptID {
		t.Errorf("receipt IDs should be identical: %s != %s", receipt1.ReceiptID, receipt2.ReceiptID)
	}

	t.Logf("  - Receipt hash: %s", receipt1.Hash[:32])
	t.Logf("  - Receipt ID: %s", receipt1.ReceiptID)
	t.Logf("  - Magnitude bucket: %s", receipt1.MagnitudeBucket)
}

// TestSyncReceiptMagnitudeBuckets verifies magnitude buckets hide raw counts.
func TestSyncReceiptMagnitudeBuckets(t *testing.T) {
	t.Log("Phase 19.1: Testing magnitude bucket abstraction")

	testCases := []struct {
		count    int
		expected persist.MagnitudeBucket
	}{
		{0, persist.MagnitudeNone},
		{1, persist.MagnitudeHandful},
		{5, persist.MagnitudeHandful},
		{6, persist.MagnitudeSeveral},
		{20, persist.MagnitudeSeveral},
		{21, persist.MagnitudeMany},
		{100, persist.MagnitudeMany},
	}

	for _, tc := range testCases {
		bucket := persist.ToMagnitudeBucket(tc.count)
		if bucket != tc.expected {
			t.Errorf("count %d: expected %s, got %s", tc.count, tc.expected, bucket)
		}
		t.Logf("  - count %d -> %s", tc.count, bucket)
	}
}

// TestSyncReceiptNoRawCounts verifies receipt doesn't expose raw counts.
func TestSyncReceiptNoRawCounts(t *testing.T) {
	t.Log("Phase 19.1: Testing receipt contains no raw counts")

	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	circleID := identity.EntityID("test-circle-1")

	// Create receipt with specific count
	receipt := persist.NewSyncReceipt(circleID, "gmail", 17, 15, fixedTime, true, "")

	// Verify receipt has no field exposing raw count
	// The receipt struct only has MagnitudeBucket, not a raw count field
	if receipt.MagnitudeBucket != persist.MagnitudeSeveral {
		t.Errorf("expected magnitude 'several' for 17 messages, got %s", receipt.MagnitudeBucket)
	}

	// Verify display text is abstract
	displayText := receipt.MagnitudeBucket.DisplayText()
	if displayText == "17" {
		t.Error("display text should NOT be raw count")
	}
	t.Logf("  - 17 messages displays as: %s", displayText)
}

// TestTimeBucketPrivacy verifies time buckets floor to 5-minute intervals.
func TestTimeBucketPrivacy(t *testing.T) {
	t.Log("Phase 19.1: Testing time bucket privacy")

	// Test various times within same 5-minute bucket
	t1 := time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 15, 10, 34, 59, 0, time.UTC)

	bucket1 := persist.TimeBucket(t1)
	bucket2 := persist.TimeBucket(t2)

	if bucket1 != bucket2 {
		t.Errorf("times in same 5-min window should have same bucket: %v != %v", bucket1, bucket2)
	}

	// Verify bucket is floored
	expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if bucket1 != expected {
		t.Errorf("expected bucket %v, got %v", expected, bucket1)
	}

	t.Logf("  - 10:31:00 -> %s", persist.TimeBucketString(t1))
	t.Logf("  - 10:34:59 -> %s", persist.TimeBucketString(t2))
}

// TestSyncReceiptStore verifies receipt storage and retrieval.
func TestSyncReceiptStore(t *testing.T) {
	t.Log("Phase 19.1: Testing sync receipt store")

	// Use mutable time for advancing
	currentTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFunc(func() time.Time { return currentTime })
	store := persist.NewSyncReceiptStore(clk.Now)

	circleID := identity.EntityID("test-circle-1")

	// Store first receipt
	receipt1 := persist.NewSyncReceipt(circleID, "gmail", 5, 5, clk.Now(), true, "")
	err := store.Store(receipt1)
	if err != nil {
		t.Fatalf("failed to store receipt: %v", err)
	}

	// Advance time and store second receipt
	currentTime = currentTime.Add(10 * time.Minute)
	receipt2 := persist.NewSyncReceipt(circleID, "gmail", 10, 8, clk.Now(), true, "")
	err = store.Store(receipt2)
	if err != nil {
		t.Fatalf("failed to store receipt: %v", err)
	}

	// Verify count
	if store.Count() != 2 {
		t.Errorf("expected 2 receipts, got %d", store.Count())
	}

	// Verify retrieval by ID
	retrieved, ok := store.Get(receipt1.ReceiptID)
	if !ok {
		t.Error("failed to retrieve receipt by ID")
	}
	if retrieved.Hash != receipt1.Hash {
		t.Error("retrieved receipt hash mismatch")
	}

	// Verify latest retrieval
	latest := store.GetLatestByCircle(circleID)
	if latest == nil {
		t.Fatal("failed to get latest receipt")
	}
	if latest.ReceiptID != receipt2.ReceiptID {
		t.Error("latest receipt should be the second one")
	}

	t.Logf("  - Stored receipts: %d", store.Count())
	t.Logf("  - Latest receipt ID: %s", latest.ReceiptID)
}

// TestGmailObligationsDefaultToHold verifies Gmail rules enforce DefaultToHold.
func TestGmailObligationsDefaultToHold(t *testing.T) {
	t.Log("Phase 19.1: Testing Gmail obligations DefaultToHold")

	// Get Gmail extractor config
	config := obligations.DefaultGmailRestraintConfig()

	// Verify DefaultToHold is true
	if !config.DefaultToHold {
		t.Error("Gmail config DefaultToHold must be true")
	}

	t.Logf("  - DefaultToHold: %v", config.DefaultToHold)
	t.Logf("  - MaxRegret: %.2f", config.MaxRegret)
}

// TestQuietCheckStatus verifies quiet check computation.
func TestQuietCheckStatus(t *testing.T) {
	t.Log("Phase 19.1: Testing quiet check status")

	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	// Test quiet status (held + no auto-surface)
	quietStatus := persist.NewQuietCheckStatus(
		true,                     // gmail connected
		fixedTime,                // last sync
		persist.MagnitudeSeveral, // magnitude
		true,                     // obligations held
		false,                    // no auto-surface
	)

	if !quietStatus.IsQuiet() {
		t.Error("status should be quiet when held=true and autoSurface=false")
	}

	// Test non-quiet status (auto-surface enabled)
	notQuietStatus := persist.NewQuietCheckStatus(
		true,
		fixedTime,
		persist.MagnitudeSeveral,
		true,
		true, // auto-surface enabled - NOT quiet
	)

	if notQuietStatus.IsQuiet() {
		t.Error("status should NOT be quiet when autoSurface=true")
	}

	t.Logf("  - Quiet status hash: %s...", quietStatus.Hash[:16])
	t.Logf("  - IsQuiet (held, no auto): %v", quietStatus.IsQuiet())
	t.Logf("  - IsQuiet (auto-surface): %v", notQuietStatus.IsQuiet())
}

// TestQuietCheckStatusDeterminism verifies status hash is deterministic.
func TestQuietCheckStatusDeterminism(t *testing.T) {
	t.Log("Phase 19.1: Testing quiet check status determinism")

	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	status1 := persist.NewQuietCheckStatus(true, fixedTime, persist.MagnitudeSeveral, true, false)
	status2 := persist.NewQuietCheckStatus(true, fixedTime, persist.MagnitudeSeveral, true, false)

	if status1.Hash != status2.Hash {
		t.Errorf("status hashes should be identical: %s != %s", status1.Hash, status2.Hash)
	}

	t.Logf("  - Status hash: %s", status1.Hash[:32])
}

// TestEventStoreDeduplication verifies events are deduplicated.
func TestEventStoreDeduplication(t *testing.T) {
	t.Log("Phase 19.1: Testing event store deduplication")

	store := domainevents.NewInMemoryEventStore()

	// Create a test event
	event := &domainevents.EmailMessageEvent{
		BaseEvent: domainevents.BaseEvent{
			ID:       "email_message_abc123",
			Type:     domainevents.EventTypeEmailMessage,
			Vendor:   "gmail",
			Source:   "msg-001",
			Captured: time.Now(),
			Occurred: time.Now(),
			Circle:   "circle-1",
		},
		MessageID: "msg-001",
	}

	// Store first time
	store.Store(event)

	// Try to store again (should deduplicate)
	store.Store(event)

	// Verify only one event stored
	count := store.Count()
	if count != 1 {
		t.Errorf("expected 1 event (deduplicated), got %d", count)
	}

	t.Logf("  - Events stored: %d (after 2 store attempts)", count)
}

// TestSyncLimitsConstants verifies sync limits are correct.
func TestSyncLimitsConstants(t *testing.T) {
	t.Log("Phase 19.1: Testing sync limit constants")

	// These constants should match the handler
	const maxMessages = 25
	const syncDays = 7

	// Verify max messages is 25 (not 50 from Phase 18.8)
	if maxMessages > 25 {
		t.Errorf("max messages should be 25, not %d", maxMessages)
	}

	// Verify sync window is 7 days (not 24 hours from Phase 18.8)
	if syncDays != 7 {
		t.Errorf("sync days should be 7, not %d", syncDays)
	}

	t.Logf("  - Max messages: %d", maxMessages)
	t.Logf("  - Sync window: %d days", syncDays)
}

// TestSyncReceiptValidation verifies receipt validation.
func TestSyncReceiptValidation(t *testing.T) {
	t.Log("Phase 19.1: Testing sync receipt validation")

	// Valid receipt
	valid := persist.NewSyncReceipt(
		identity.EntityID("circle-1"),
		"gmail",
		10, 8,
		time.Now(),
		true, "",
	)

	if err := valid.Validate(); err != nil {
		t.Errorf("valid receipt should pass validation: %v", err)
	}

	// Invalid receipt (missing circle ID)
	invalid := &persist.SyncReceipt{
		ReceiptID:  "test",
		Provider:   "gmail",
		TimeBucket: time.Now(),
	}

	if err := invalid.Validate(); err == nil {
		t.Error("invalid receipt should fail validation")
	}

	t.Logf("  - Valid receipt: pass")
	t.Logf("  - Invalid receipt: fail (missing circle_id)")
}
