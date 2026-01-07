// Package demo_phase31_1_gmail_receipt_observer contains tests for Phase 31.1: Gmail Receipt Observers.
//
// Phase 31.1 takes real Gmail message metadata and generates CommerceObservation inputs.
// This is a restraint-first observer that proves: "We noticed commerce without becoming a budgeting dashboard."
//
// CRITICAL INVARIANTS:
//   - NO merchant names stored
//   - NO amounts, currency symbols
//   - NO sender emails stored
//   - NO subjects stored
//   - Only abstract category + magnitude + horizon buckets + evidence hashes
//   - Deterministic: same inputs => same outputs
//
// Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md
package demo_phase31_1_gmail_receipt_observer

import (
	"strings"
	"testing"
	"time"

	"quantumlife/internal/commerceingest"
	"quantumlife/internal/persist"
	"quantumlife/internal/receiptscan"
	"quantumlife/pkg/domain/commerceobserver"
)

// TestDeterminism verifies that same inputs + same clock = same output hashes.
func TestDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := commerceingest.NewEngine(clock)

	messageData := []commerceingest.MessageData{
		{MessageID: "msg1", SenderDomain: "deliveroo.co.uk", Subject: "Your order is on the way", Snippet: "Your food will arrive soon"},
		{MessageID: "msg2", SenderDomain: "uber.com", Subject: "Trip receipt", Snippet: "Thanks for riding with us"},
		{MessageID: "msg3", SenderDomain: "amazon.co.uk", Subject: "Order confirmation", Snippet: "Your order has been dispatched"},
	}

	// First run
	result1 := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash_1", messageData)

	// Second run with same inputs
	result2 := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash_1", messageData)

	// Verify determinism
	if result1.StatusHash != result2.StatusHash {
		t.Errorf("Status hash differs: %s vs %s", result1.StatusHash, result2.StatusHash)
	}

	if len(result1.Observations) != len(result2.Observations) {
		t.Errorf("Observation count differs: %d vs %d", len(result1.Observations), len(result2.Observations))
	}

	for i := range result1.Observations {
		if result1.Observations[i].EvidenceHash != result2.Observations[i].EvidenceHash {
			t.Errorf("Observation %d hash differs", i)
		}
	}
}

// TestPrivacy verifies that output contains no forbidden patterns.
func TestPrivacy(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := commerceingest.NewEngine(clock)

	messageData := []commerceingest.MessageData{
		{MessageID: "msg1", SenderDomain: "deliveroo.co.uk", Subject: "Your £15.50 order", Snippet: "Paid with card ending 1234"},
		{MessageID: "msg2", SenderDomain: "uber.com", Subject: "Receipt for $25.00", Snippet: "Driver: John Smith"},
	}

	result := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash", messageData)

	// Check that result contains no forbidden patterns
	resultStr := result.CanonicalString()

	forbiddenPatterns := []string{
		"deliveroo",
		"uber",
		"£15.50",
		"$25.00",
		"1234",
		"John Smith",
		"@",
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(resultStr), strings.ToLower(pattern)) {
			t.Errorf("Result contains forbidden pattern: %s", pattern)
		}
	}

	// Check observations
	for _, obs := range result.Observations {
		obsStr := obs.CanonicalString()
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(strings.ToLower(obsStr), strings.ToLower(pattern)) {
				t.Errorf("Observation contains forbidden pattern: %s", pattern)
			}
		}
	}
}

// TestCategorySelection verifies max 3 categories with stable ordering.
func TestCategorySelection(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := commerceingest.NewEngine(clock)

	// Create messages for many categories
	messageData := []commerceingest.MessageData{
		{MessageID: "msg1", SenderDomain: "deliveroo.co.uk", Subject: "Order confirmation", Snippet: "Your delivery"},
		{MessageID: "msg2", SenderDomain: "deliveroo.co.uk", Subject: "Order delivered", Snippet: "Food arrived"},
		{MessageID: "msg3", SenderDomain: "uber.com", Subject: "Trip receipt", Snippet: "Thanks for riding"},
		{MessageID: "msg4", SenderDomain: "amazon.co.uk", Subject: "Order shipped", Snippet: "Your order"},
		{MessageID: "msg5", SenderDomain: "netflix.com", Subject: "Subscription renewal", Snippet: "Your monthly subscription"},
		{MessageID: "msg6", SenderDomain: "spotify.com", Subject: "Subscription renewal", Snippet: "Your monthly subscription"},
		{MessageID: "msg7", SenderDomain: "bt.com", Subject: "Monthly bill", Snippet: "Your bill is ready"},
	}

	result := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash", messageData)

	// Should have at most 3 observations (max categories)
	if len(result.Observations) > commerceingest.MaxCategories {
		t.Errorf("Expected max %d observations, got %d", commerceingest.MaxCategories, len(result.Observations))
	}

	// Verify stable ordering (multiple runs should produce same order)
	result2 := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash", messageData)

	for i := range result.Observations {
		if i >= len(result2.Observations) {
			break
		}
		if result.Observations[i].Category != result2.Observations[i].Category {
			t.Errorf("Category order differs at position %d: %s vs %s",
				i, result.Observations[i].Category, result2.Observations[i].Category)
		}
	}
}

// TestMagnitudeBucketing verifies 0=>nothing, 1-2=>a_few, >=3=>several.
func TestMagnitudeBucketing(t *testing.T) {
	tests := []struct {
		count    int
		expected commerceingest.MagnitudeBucket
	}{
		{0, commerceingest.MagnitudeNothing},
		{1, commerceingest.MagnitudeAFew},
		{2, commerceingest.MagnitudeAFew},
		{3, commerceingest.MagnitudeSeveral},
		{10, commerceingest.MagnitudeSeveral},
		{100, commerceingest.MagnitudeSeveral},
	}

	for _, tt := range tests {
		bucket := commerceingest.ToMagnitudeBucket(tt.count)
		if bucket != tt.expected {
			t.Errorf("ToMagnitudeBucket(%d) = %s, expected %s", tt.count, bucket, tt.expected)
		}
	}
}

// TestHorizonRuleDeterminism verifies horizon classification is deterministic.
func TestHorizonRuleDeterminism(t *testing.T) {
	// Test inputs with different horizon signals
	inputs := []receiptscan.ReceiptScanInput{
		{CircleID: "c1", MessageIDHash: "h1", Subject: "Your order is on the way", Snippet: "Arriving soon"},
		{CircleID: "c1", MessageIDHash: "h2", Subject: "Order delivered", Snippet: "Your food is here"},
		{CircleID: "c1", MessageIDHash: "h3", Subject: "Subscription renewal", Snippet: "Your monthly payment"},
	}

	results := receiptscan.ClassifyBatch(inputs)

	// Run again
	results2 := receiptscan.ClassifyBatch(inputs)

	// Verify determinism
	for i := range results {
		if len(results[i].Signals) > 0 && len(results2[i].Signals) > 0 {
			if results[i].Signals[0].Horizon != results2[i].Signals[0].Horizon {
				t.Errorf("Horizon differs for input %d: %s vs %s",
					i, results[i].Signals[0].Horizon, results2[i].Signals[0].Horizon)
			}
		}
	}
}

// TestIntegration simulates Gmail sync -> ingest -> persist -> /mirror/commerce.
func TestIntegration(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	// Create stores and engines
	store := persist.NewCommerceObserverStore(clock)
	engine := commerceingest.NewEngine(clock)

	// Simulate Gmail sync data
	messageData := []commerceingest.MessageData{
		{MessageID: "msg1", SenderDomain: "deliveroo.co.uk", Subject: "Order confirmation", Snippet: "Your food delivery"},
		{MessageID: "msg2", SenderDomain: "uber.com", Subject: "Trip receipt", Snippet: "Thanks for riding"},
	}

	// Build observations
	period := commerceingest.PeriodFromTime(fixedTime)
	result := engine.BuildFromGmailMessages("circle_test", period, "sync_receipt_hash", messageData)

	// Persist observations
	for _, obs := range result.Observations {
		err := store.PersistObservation("circle_test", &obs)
		if err != nil {
			t.Fatalf("Failed to persist observation: %v", err)
		}
	}

	// Retrieve observations
	retrieved := store.GetObservationsForPeriod("circle_test", period)
	if len(retrieved) != len(result.Observations) {
		t.Errorf("Expected %d observations, got %d", len(result.Observations), len(retrieved))
	}

	// Build mirror page
	observerEngine := commerceobserver.NewCommerceMirrorPage(retrieved)
	if observerEngine == nil && len(retrieved) > 0 {
		t.Error("Expected non-nil mirror page for non-empty observations")
	}
}

// TestBoundedRetention verifies store capped to 30 days.
func TestBoundedRetention(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	store := persist.NewCommerceObserverStore(clock)

	// Add observations for 35 periods
	for i := 0; i < 35; i++ {
		period := formatPeriod(i)
		obs := &commerceobserver.CommerceObservation{
			Source:       commerceobserver.SourceGmailReceipt,
			Category:     commerceobserver.CategoryFoodDelivery,
			Frequency:    commerceobserver.FrequencyOccasional,
			Stability:    commerceobserver.StabilityStable,
			Period:       period,
			EvidenceHash: "hash_" + period,
		}
		err := store.PersistObservation("circle_test", obs)
		if err != nil {
			t.Fatalf("Failed to persist observation %d: %v", i, err)
		}
	}

	// Trigger eviction
	store.ExpireOldObservations()

	// Verify count is bounded
	count := store.Count()
	if count > 30 {
		t.Errorf("Expected at most 30 observations after eviction, got %d", count)
	}
}

// TestStorelogReplay verifies persist then reload => identical state hash.
func TestStorelogReplay(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	store := persist.NewCommerceObserverStore(clock)

	obs := &commerceobserver.CommerceObservation{
		Source:       commerceobserver.SourceGmailReceipt,
		Category:     commerceobserver.CategoryTransport,
		Frequency:    commerceobserver.FrequencyFrequent,
		Stability:    commerceobserver.StabilityDrifting,
		Period:       "2025-W03",
		EvidenceHash: "test_hash",
	}

	// Persist
	err := store.PersistObservation("circle_test", obs)
	if err != nil {
		t.Fatalf("Failed to persist observation: %v", err)
	}

	// Retrieve and verify
	retrieved := store.GetObservationsForPeriod("circle_test", "2025-W03")
	if len(retrieved) != 1 {
		t.Fatalf("Expected 1 observation, got %d", len(retrieved))
	}

	// Verify hash matches
	if retrieved[0].ComputeHash() != obs.ComputeHash() {
		t.Error("Observation hash differs after retrieve")
	}
}

// TestNoReceiptsNoObservations verifies empty input produces no observations.
func TestNoReceiptsNoObservations(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := commerceingest.NewEngine(clock)

	// Empty input
	result := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash", nil)

	if len(result.Observations) != 0 {
		t.Errorf("Expected 0 observations for empty input, got %d", len(result.Observations))
	}

	if result.OverallMagnitude != commerceingest.MagnitudeNothing {
		t.Errorf("Expected magnitude 'nothing' for empty input, got %s", result.OverallMagnitude)
	}
}

// TestNonReceiptMessagesFiltered verifies non-receipt messages don't produce observations.
func TestNonReceiptMessagesFiltered(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := commerceingest.NewEngine(clock)

	// Non-receipt messages
	messageData := []commerceingest.MessageData{
		{MessageID: "msg1", SenderDomain: "github.com", Subject: "New pull request", Snippet: "Review requested"},
		{MessageID: "msg2", SenderDomain: "linkedin.com", Subject: "New connection", Snippet: "wants to connect"},
		{MessageID: "msg3", SenderDomain: "twitter.com", Subject: "New follower", Snippet: "started following you"},
	}

	result := engine.BuildFromGmailMessages("circle_test", "2025-W03", "sync_hash", messageData)

	if len(result.Observations) != 0 {
		t.Errorf("Expected 0 observations for non-receipt messages, got %d", len(result.Observations))
	}
}

// TestReceiptKeywordDetection verifies receipt-indicating keywords are detected.
func TestReceiptKeywordDetection(t *testing.T) {
	receiptInputs := []receiptscan.ReceiptScanInput{
		{CircleID: "c1", MessageIDHash: "h1", Subject: "Your receipt", Snippet: "Thank you"},
		{CircleID: "c1", MessageIDHash: "h2", Subject: "Order confirmation", Snippet: "Order placed"},
		{CircleID: "c1", MessageIDHash: "h3", Subject: "Payment received", Snippet: "We got your payment"},
		{CircleID: "c1", MessageIDHash: "h4", Subject: "Invoice", Snippet: "Your invoice"},
		{CircleID: "c1", MessageIDHash: "h5", Subject: "Booking confirmed", Snippet: "Your booking"},
	}

	for _, input := range receiptInputs {
		result := receiptscan.Classify(input)
		if !result.IsReceipt {
			t.Errorf("Expected '%s' to be classified as receipt", input.Subject)
		}
	}
}

// TestCategoryMapping verifies receipt categories map to commerce observer categories.
func TestCategoryMapping(t *testing.T) {
	tests := []struct {
		receiptCat  receiptscan.ReceiptCategory
		expectedCat commerceobserver.CategoryBucket
	}{
		{receiptscan.CategoryDelivery, commerceobserver.CategoryFoodDelivery},
		{receiptscan.CategoryTransport, commerceobserver.CategoryTransport},
		{receiptscan.CategoryRetail, commerceobserver.CategoryRetail},
		{receiptscan.CategorySubscription, commerceobserver.CategorySubscriptions},
		{receiptscan.CategoryBills, commerceobserver.CategoryUtilities},
		{receiptscan.CategoryOther, commerceobserver.CategoryOther},
	}

	for _, tt := range tests {
		mapped := commerceingest.MapReceiptCategory(tt.receiptCat)
		if mapped != tt.expectedCat {
			t.Errorf("MapReceiptCategory(%s) = %s, expected %s",
				tt.receiptCat, mapped, tt.expectedCat)
		}
	}
}

// TestCanonicalStringFormat verifies pipe-delimited format.
func TestCanonicalStringFormat(t *testing.T) {
	signal := &receiptscan.ReceiptSignal{
		Category:     receiptscan.CategoryDelivery,
		Horizon:      receiptscan.HorizonNow,
		EvidenceHash: "test_hash",
	}

	canonical := signal.CanonicalString()

	// Should be pipe-delimited
	if !strings.HasPrefix(canonical, "RECEIPT_SIGNAL|v1|") {
		t.Errorf("Canonical string should start with RECEIPT_SIGNAL|v1|, got: %s", canonical)
	}

	// Should contain pipes
	pipes := strings.Count(canonical, "|")
	if pipes < 3 {
		t.Errorf("Expected at least 3 pipes in canonical string, got %d", pipes)
	}

	// Should NOT be JSON
	if strings.Contains(canonical, "{") || strings.Contains(canonical, "}") {
		t.Error("Canonical string should not be JSON")
	}
}

// TestHashMessageID verifies message IDs are hashed, never stored raw.
func TestHashMessageID(t *testing.T) {
	rawID := "CADtU9+xyz123_message_id"

	hash := receiptscan.HashMessageID(rawID)

	// Hash should not contain the raw ID
	if strings.Contains(hash, rawID) {
		t.Error("Hash should not contain raw message ID")
	}

	// Hash should be deterministic
	hash2 := receiptscan.HashMessageID(rawID)
	if hash != hash2 {
		t.Error("Hash should be deterministic")
	}

	// Hash should be 32 hex chars
	if len(hash) != 32 {
		t.Errorf("Expected 32 hex chars, got %d", len(hash))
	}
}

// Helper functions

func formatPeriod(i int) string {
	week := (i % 52) + 1
	year := 2025 - (i / 52)

	weekStr := ""
	if week < 10 {
		weekStr = "0" + string(rune('0'+week))
	} else {
		weekStr = string(rune('0'+week/10)) + string(rune('0'+week%10))
	}

	yearStr := ""
	for j := 3; j >= 0; j-- {
		digit := (year / pow10(j)) % 10
		yearStr += string(rune('0' + digit))
	}

	return yearStr + "-W" + weekStr
}

func pow10(n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}
