// Package demo_phase31_2_finance_commerce_observer tests Phase 31.2 functionality.
//
// Phase 31.2: Commerce from Finance (TrueLayer → CommerceSignals)
// Reference: docs/ADR/ADR-0064-phase31-2-commerce-from-finance.md
//
// These tests verify that:
// - Transaction classification is deterministic
// - Only abstract buckets are output (no merchant names, amounts, timestamps)
// - Max 3 categories are selected
// - Source kind is set to SourceFinanceTrueLayer
// - Evidence hashes are computed from abstract tokens only
package demo_phase31_2_finance_commerce_observer

import (
	"testing"
	"time"

	"quantumlife/internal/financetxscan"
	"quantumlife/pkg/domain/commerceobserver"
)

// mockClock returns a fixed time for deterministic testing.
func mockClock() time.Time {
	return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
}

// TestDeterminism verifies that same inputs always produce same outputs.
func TestDeterminism(t *testing.T) {
	t.Parallel()

	engine := financetxscan.NewEngine(mockClock)

	// Phase 31.3: Use ProviderTrueLayer for real provider
	transactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-001", "FOOD_AND_DRINK", "5812", "online"),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-002", "TRANSPORT", "4121", "contactless"),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-003", "SHOPPING", "5311", "in_store"),
	}

	result1 := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", transactions)
	result2 := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", transactions)

	if result1.StatusHash != result2.StatusHash {
		t.Errorf("Expected deterministic output, got different hashes: %s != %s",
			result1.StatusHash, result2.StatusHash)
	}

	if len(result1.Observations) != len(result2.Observations) {
		t.Errorf("Expected same number of observations: %d != %d",
			len(result1.Observations), len(result2.Observations))
	}
}

// TestEmptyTransactions verifies behavior with no transactions.
func TestEmptyTransactions(t *testing.T) {
	t.Parallel()

	engine := financetxscan.NewEngine(mockClock)

	result := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", nil)

	if len(result.Observations) != 0 {
		t.Errorf("Expected no observations for empty input, got %d", len(result.Observations))
	}

	if result.OverallMagnitude != financetxscan.MagnitudeNothing {
		t.Errorf("Expected MagnitudeNothing, got %s", result.OverallMagnitude)
	}
}

// TestSourceKindIsFinanceTrueLayer verifies observations have correct source.
func TestSourceKindIsFinanceTrueLayer(t *testing.T) {
	t.Parallel()

	engine := financetxscan.NewEngine(mockClock)

	transactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-001", "FOOD_AND_DRINK", "5812", "online"),
	}

	result := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", transactions)

	for _, obs := range result.Observations {
		if obs.Source != commerceobserver.SourceFinanceTrueLayer {
			t.Errorf("Expected SourceFinanceTrueLayer, got %s", obs.Source)
		}
	}
}

// TestMaxThreeCategories verifies at most 3 categories are selected.
func TestMaxThreeCategories(t *testing.T) {
	t.Parallel()

	engine := financetxscan.NewEngine(mockClock)

	// Create transactions in 5 different categories
	transactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-001", "FOOD_AND_DRINK", "5812", "online"),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-002", "TRANSPORT", "4121", "contactless"),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-003", "SHOPPING", "5311", "in_store"),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-004", "UTILITIES", "4900", "online"),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-005", "SUBSCRIPTIONS", "5815", "online"),
	}

	result := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", transactions)

	if len(result.Observations) > 3 {
		t.Errorf("Expected max 3 categories, got %d", len(result.Observations))
	}
}

// TestCategoryClassificationByProviderCategory verifies provider category classification.
func TestCategoryClassificationByProviderCategory(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		providerCategory string
		expectedCategory commerceobserver.CategoryBucket
	}{
		{"FoodAndDrink", "FOOD_AND_DRINK", commerceobserver.CategoryFoodDelivery},
		{"Transport", "TRANSPORT", commerceobserver.CategoryTransport},
		{"Shopping", "SHOPPING", commerceobserver.CategoryRetail},
		{"Utilities", "UTILITIES", commerceobserver.CategoryUtilities},
		{"Subscriptions", "SUBSCRIPTIONS", commerceobserver.CategorySubscriptions},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := financetxscan.TransactionInput{
				CircleID:           "circle-1",
				TransactionIDHash:  "hash-1",
				Provider:           financetxscan.ProviderTrueLayer,
				ProviderCategory:   tc.providerCategory,
				ProviderCategoryID: "",
				PaymentChannel:     "",
			}

			result := financetxscan.Classify(input)

			if !result.IsClassified {
				t.Errorf("Expected classification for %s", tc.providerCategory)
				return
			}

			if result.Signal.Category != tc.expectedCategory {
				t.Errorf("Expected category %s, got %s", tc.expectedCategory, result.Signal.Category)
			}
		})
	}
}

// TestCategoryClassificationByMCC verifies MCC code classification.
func TestCategoryClassificationByMCC(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		mcc              string
		expectedCategory commerceobserver.CategoryBucket
	}{
		{"Restaurant5812", "5812", commerceobserver.CategoryFoodDelivery},
		{"FastFood5814", "5814", commerceobserver.CategoryFoodDelivery},
		{"Taxi4121", "4121", commerceobserver.CategoryTransport},
		{"GasStation5541", "5541", commerceobserver.CategoryTransport},
		{"DeptStore5311", "5311", commerceobserver.CategoryRetail},
		{"Utilities4900", "4900", commerceobserver.CategoryUtilities},
		{"DigitalGoods5815", "5815", commerceobserver.CategorySubscriptions},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := financetxscan.TransactionInput{
				CircleID:           "circle-1",
				TransactionIDHash:  "hash-1",
				Provider:           financetxscan.ProviderTrueLayer,
				ProviderCategory:   "",
				ProviderCategoryID: tc.mcc,
				PaymentChannel:     "",
			}

			result := financetxscan.Classify(input)

			if !result.IsClassified {
				t.Errorf("Expected classification for MCC %s", tc.mcc)
				return
			}

			if result.Signal.Category != tc.expectedCategory {
				t.Errorf("Expected category %s for MCC %s, got %s",
					tc.expectedCategory, tc.mcc, result.Signal.Category)
			}
		})
	}
}

// TestConfidenceLevelsBySource verifies confidence levels are assigned correctly.
func TestConfidenceLevelsBySource(t *testing.T) {
	t.Parallel()

	// Provider category = high confidence
	input1 := financetxscan.TransactionInput{
		CircleID:          "circle-1",
		TransactionIDHash: "hash-1",
		Provider:          financetxscan.ProviderTrueLayer,
		ProviderCategory:  "FOOD_AND_DRINK",
	}
	result1 := financetxscan.Classify(input1)
	if result1.Signal.ConfidenceLevel != financetxscan.ConfidenceHigh {
		t.Errorf("Expected high confidence for provider category, got %s",
			result1.Signal.ConfidenceLevel)
	}

	// MCC only = medium confidence
	input2 := financetxscan.TransactionInput{
		CircleID:           "circle-1",
		TransactionIDHash:  "hash-2",
		Provider:           financetxscan.ProviderTrueLayer,
		ProviderCategoryID: "5812",
	}
	result2 := financetxscan.Classify(input2)
	if result2.Signal.ConfidenceLevel != financetxscan.ConfidenceMedium {
		t.Errorf("Expected medium confidence for MCC, got %s",
			result2.Signal.ConfidenceLevel)
	}

	// Payment channel only = low confidence
	input3 := financetxscan.TransactionInput{
		CircleID:          "circle-1",
		TransactionIDHash: "hash-3",
		Provider:          financetxscan.ProviderTrueLayer,
		PaymentChannel:    "in_store",
	}
	result3 := financetxscan.Classify(input3)
	if result3.Signal.ConfidenceLevel != financetxscan.ConfidenceLow {
		t.Errorf("Expected low confidence for payment channel, got %s",
			result3.Signal.ConfidenceLevel)
	}
}

// TestUnclassifiableTransaction verifies unclassifiable transactions.
func TestUnclassifiableTransaction(t *testing.T) {
	t.Parallel()

	input := financetxscan.TransactionInput{
		CircleID:           "circle-1",
		TransactionIDHash:  "hash-1",
		Provider:           financetxscan.ProviderTrueLayer,
		ProviderCategory:   "",
		ProviderCategoryID: "",
		PaymentChannel:     "",
	}

	result := financetxscan.Classify(input)

	if result.IsClassified {
		t.Error("Expected transaction with no data to be unclassifiable")
	}
}

// TestMagnitudeBuckets verifies magnitude bucket assignment.
func TestMagnitudeBuckets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		count          int
		expectedBucket financetxscan.MagnitudeBucket
	}{
		{0, financetxscan.MagnitudeNothing},
		{1, financetxscan.MagnitudeAFew},
		{3, financetxscan.MagnitudeAFew},
		{5, financetxscan.MagnitudeAFew},
		{6, financetxscan.MagnitudeSeveral},
		{10, financetxscan.MagnitudeSeveral},
	}

	for _, tc := range testCases {
		bucket := financetxscan.ToMagnitudeBucket(tc.count)
		if bucket != tc.expectedBucket {
			t.Errorf("For count %d, expected %s, got %s",
				tc.count, tc.expectedBucket, bucket)
		}
	}
}

// TestCanonicalStringFormat verifies pipe-delimited format.
func TestCanonicalStringFormat(t *testing.T) {
	t.Parallel()

	signal := &financetxscan.TransactionSignal{
		Category:        commerceobserver.CategoryFoodDelivery,
		ConfidenceLevel: financetxscan.ConfidenceHigh,
		EvidenceHash:    "abc123",
	}

	canonical := signal.CanonicalString()

	// Verify pipe-delimited format
	if canonical[0:10] != "TX_SIGNAL|" {
		t.Errorf("Expected TX_SIGNAL| prefix, got %s", canonical[:10])
	}

	// Verify version prefix
	if len(canonical) < 13 || canonical[10:13] != "v1|" {
		t.Errorf("Expected v1| version, got %s", canonical)
	}
}

// TestTransactionIDHashing verifies transaction IDs are hashed.
func TestTransactionIDHashing(t *testing.T) {
	t.Parallel()

	hash1 := financetxscan.HashTransactionID("tx-001")
	hash2 := financetxscan.HashTransactionID("tx-002")
	hash3 := financetxscan.HashTransactionID("tx-001")

	if hash1 == hash2 {
		t.Error("Different transaction IDs should produce different hashes")
	}

	if hash1 != hash3 {
		t.Error("Same transaction ID should produce same hash")
	}

	// Verify hash is not the raw ID
	if hash1 == "tx-001" {
		t.Error("Hash should not be the raw transaction ID")
	}
}

// TestPeriodFormat verifies ISO week format.
func TestPeriodFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		time           time.Time
		expectedPeriod string
	}{
		{time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "2024-W03"},
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "2024-W01"},
		{time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), "2025-W01"}, // Dec 31, 2024 is in week 1 of 2025
	}

	for _, tc := range testCases {
		period := financetxscan.PeriodFromTime(tc.time)
		if period != tc.expectedPeriod {
			t.Errorf("For %v, expected %s, got %s", tc.time, tc.expectedPeriod, period)
		}
	}
}

// TestConfidenceToStability verifies confidence to stability mapping.
func TestConfidenceToStability(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		confidence        financetxscan.ConfidenceLevel
		expectedStability commerceobserver.StabilityBucket
	}{
		{financetxscan.ConfidenceHigh, commerceobserver.StabilityStable},
		{financetxscan.ConfidenceMedium, commerceobserver.StabilityDrifting},
		{financetxscan.ConfidenceLow, commerceobserver.StabilityVolatile},
	}

	for _, tc := range testCases {
		stability := financetxscan.ConfidenceToStability(tc.confidence)
		if stability != tc.expectedStability {
			t.Errorf("For confidence %s, expected stability %s, got %s",
				tc.confidence, tc.expectedStability, stability)
		}
	}
}

// TestFilterClassifiedOnly verifies filtering of unclassified transactions.
func TestFilterClassifiedOnly(t *testing.T) {
	t.Parallel()

	results := []financetxscan.TransactionScanResult{
		{TransactionIDHash: "h1", IsClassified: true, Signal: &financetxscan.TransactionSignal{Category: commerceobserver.CategoryRetail}},
		{TransactionIDHash: "h2", IsClassified: false, Signal: nil},
		{TransactionIDHash: "h3", IsClassified: true, Signal: &financetxscan.TransactionSignal{Category: commerceobserver.CategoryTransport}},
	}

	filtered := financetxscan.FilterClassifiedOnly(results)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 classified results, got %d", len(filtered))
	}
}

// TestCountByCategory verifies category counting.
func TestCountByCategory(t *testing.T) {
	t.Parallel()

	results := []financetxscan.TransactionScanResult{
		{IsClassified: true, Signal: &financetxscan.TransactionSignal{Category: commerceobserver.CategoryRetail}},
		{IsClassified: true, Signal: &financetxscan.TransactionSignal{Category: commerceobserver.CategoryRetail}},
		{IsClassified: true, Signal: &financetxscan.TransactionSignal{Category: commerceobserver.CategoryTransport}},
		{IsClassified: false, Signal: nil},
	}

	counts := financetxscan.CountByCategory(results)

	if counts[commerceobserver.CategoryRetail] != 2 {
		t.Errorf("Expected 2 retail, got %d", counts[commerceobserver.CategoryRetail])
	}

	if counts[commerceobserver.CategoryTransport] != 1 {
		t.Errorf("Expected 1 transport, got %d", counts[commerceobserver.CategoryTransport])
	}
}

// TestTopCategoriesSelection verifies top categories are selected by count.
func TestTopCategoriesSelection(t *testing.T) {
	t.Parallel()

	engine := financetxscan.NewEngine(mockClock)

	// Create transactions with varying counts per category
	// Retail: 3, Transport: 2, Food: 1, Utilities: 1, Subscriptions: 1
	transactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-001", "SHOPPING", "5311", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-002", "SHOPPING", "5311", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-003", "SHOPPING", "5311", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-004", "TRANSPORT", "4121", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-005", "TRANSPORT", "4121", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-006", "FOOD_AND_DRINK", "5812", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-007", "UTILITIES", "4900", ""),
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-008", "SUBSCRIPTIONS", "5815", ""),
	}

	result := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", transactions)

	if len(result.Observations) != 3 {
		t.Errorf("Expected exactly 3 observations, got %d", len(result.Observations))
	}

	// Top 3 should be: Retail (3), Transport (2), then one of the 1-count categories
	foundRetail := false
	foundTransport := false
	for _, obs := range result.Observations {
		if obs.Category == commerceobserver.CategoryRetail {
			foundRetail = true
		}
		if obs.Category == commerceobserver.CategoryTransport {
			foundTransport = true
		}
	}

	if !foundRetail {
		t.Error("Expected Retail to be in top 3 categories")
	}
	if !foundTransport {
		t.Error("Expected Transport to be in top 3 categories")
	}
}

// TestObservationValidation verifies observations pass validation.
func TestObservationValidation(t *testing.T) {
	t.Parallel()

	engine := financetxscan.NewEngine(mockClock)

	transactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(financetxscan.ProviderTrueLayer, "tx-001", "FOOD_AND_DRINK", "5812", "online"),
	}

	result := engine.BuildFromTransactions("circle-1", "2024-W03", "sync-hash-1", transactions)

	for i, obs := range result.Observations {
		if err := obs.Validate(); err != nil {
			t.Errorf("Observation %d failed validation: %v", i, err)
		}
	}
}
