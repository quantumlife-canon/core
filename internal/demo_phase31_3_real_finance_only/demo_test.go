// Package demo_phase31_3_real_finance_only contains tests for Phase 31.3.
//
// Phase 31.3: Real Finance Sync → Commerce Observer (No Mock Path)
//
// Key invariants:
// - Provider field is REQUIRED on TransactionData
// - Mock/empty providers are REJECTED
// - Only real TrueLayer API responses are processed
// - BuildFromTransactions returns "rejected_mock_provider" for mock data
//
// Reference: docs/ADR/ADR-0065-phase31-3-real-finance-only.md
package demo_phase31_3_real_finance_only

import (
	"testing"
	"time"

	"quantumlife/internal/financetxscan"
)

// TestProviderValidationRejectsMock verifies that mock providers are rejected.
func TestProviderValidationRejectsMock(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderMock)
	if err == nil {
		t.Error("Expected mock provider to be rejected, but it was accepted")
	}

	if err.Error() != "phase31_3: mock provider rejected - real finance connection required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestProviderValidationRejectsEmpty verifies that empty providers are rejected.
func TestProviderValidationRejectsEmpty(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderEmpty)
	if err == nil {
		t.Error("Expected empty provider to be rejected, but it was accepted")
	}

	if err.Error() != "phase31_3: provider is empty - real finance connection required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestProviderValidationRejectsUnknown verifies that unknown providers are rejected.
func TestProviderValidationRejectsUnknown(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderKind("unknown"))
	if err == nil {
		t.Error("Expected unknown provider to be rejected, but it was accepted")
	}
}

// TestProviderValidationAcceptsTrueLayer verifies that TrueLayer provider is accepted.
func TestProviderValidationAcceptsTrueLayer(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderTrueLayer)
	if err != nil {
		t.Errorf("Expected TrueLayer provider to be accepted, but got error: %v", err)
	}
}

// TestIsValidProviderTrueLayer verifies TrueLayer is valid.
func TestIsValidProviderTrueLayer(t *testing.T) {
	if !financetxscan.IsValidProvider(financetxscan.ProviderTrueLayer) {
		t.Error("Expected TrueLayer to be valid provider")
	}
}

// TestIsValidProviderMock verifies mock is NOT valid.
func TestIsValidProviderMock(t *testing.T) {
	if financetxscan.IsValidProvider(financetxscan.ProviderMock) {
		t.Error("Expected mock to NOT be valid provider")
	}
}

// TestIsValidProviderEmpty verifies empty is NOT valid.
func TestIsValidProviderEmpty(t *testing.T) {
	if financetxscan.IsValidProvider(financetxscan.ProviderEmpty) {
		t.Error("Expected empty to NOT be valid provider")
	}
}

// TestAllValidProviders verifies only TrueLayer is in valid list.
func TestAllValidProviders(t *testing.T) {
	valid := financetxscan.AllValidProviders()
	if len(valid) != 1 {
		t.Errorf("Expected exactly 1 valid provider, got %d", len(valid))
	}
	if valid[0] != financetxscan.ProviderTrueLayer {
		t.Errorf("Expected ProviderTrueLayer, got %v", valid[0])
	}
}

// TestBuildFromTransactionsRejectsMock verifies that mock data is rejected.
func TestBuildFromTransactionsRejectsMock(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := financetxscan.NewEngine(clock)

	// Create transaction with mock provider
	mockTransactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(
			financetxscan.ProviderMock, // Mock provider
			"tx-001",
			"FOOD_AND_DRINK",
			"5812",
			"online",
		),
	}

	result := engine.BuildFromTransactions(
		"circle_test",
		"2025-W03",
		"sync_hash_123",
		mockTransactions,
	)

	// Verify rejection
	if result.StatusHash != "rejected_mock_provider" {
		t.Errorf("Expected status hash 'rejected_mock_provider', got '%s'", result.StatusHash)
	}

	if len(result.Observations) != 0 {
		t.Errorf("Expected 0 observations for mock data, got %d", len(result.Observations))
	}
}

// TestBuildFromTransactionsRejectsEmpty verifies that empty provider is rejected.
func TestBuildFromTransactionsRejectsEmpty(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := financetxscan.NewEngine(clock)

	// Create transaction with empty provider
	emptyProviderTx := []financetxscan.TransactionData{
		{
			TransactionID:      "tx-001",
			Provider:           financetxscan.ProviderEmpty, // Empty provider
			ProviderCategory:   "FOOD_AND_DRINK",
			ProviderCategoryID: "5812",
			PaymentChannel:     "online",
		},
	}

	result := engine.BuildFromTransactions(
		"circle_test",
		"2025-W03",
		"sync_hash_123",
		emptyProviderTx,
	)

	// Verify rejection
	if result.StatusHash != "rejected_mock_provider" {
		t.Errorf("Expected status hash 'rejected_mock_provider', got '%s'", result.StatusHash)
	}
}

// TestBuildFromTransactionsAcceptsRealProvider verifies real provider is accepted.
func TestBuildFromTransactionsAcceptsRealProvider(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := financetxscan.NewEngine(clock)

	// Create transaction with real TrueLayer provider
	realTransactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(
			financetxscan.ProviderTrueLayer, // Real provider
			"tx-001",
			"FOOD_AND_DRINK",
			"5812",
			"online",
		),
	}

	result := engine.BuildFromTransactions(
		"circle_test",
		"2025-W03",
		"sync_hash_123",
		realTransactions,
	)

	// Verify acceptance
	if result.StatusHash == "rejected_mock_provider" {
		t.Error("Expected real provider to be accepted, but got rejection")
	}

	if len(result.Observations) == 0 {
		t.Error("Expected observations for real data, got 0")
	}
}

// TestMixedProvidersRejected verifies that mixed real/mock is rejected.
func TestMixedProvidersRejected(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := financetxscan.NewEngine(clock)

	// Create transactions with mixed providers
	mixedTransactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(
			financetxscan.ProviderTrueLayer, // Real
			"tx-001",
			"FOOD_AND_DRINK",
			"5812",
			"online",
		),
		financetxscan.ExtractTransactionData(
			financetxscan.ProviderMock, // Mock
			"tx-002",
			"TRANSPORT",
			"4121",
			"contactless",
		),
	}

	result := engine.BuildFromTransactions(
		"circle_test",
		"2025-W03",
		"sync_hash_123",
		mixedTransactions,
	)

	// Verify rejection (even though some are real)
	if result.StatusHash != "rejected_mock_provider" {
		t.Errorf("Expected mixed providers to be rejected, got '%s'", result.StatusHash)
	}
}

// TestExtractTransactionDataIncludesProvider verifies provider is included.
func TestExtractTransactionDataIncludesProvider(t *testing.T) {
	tx := financetxscan.ExtractTransactionData(
		financetxscan.ProviderTrueLayer,
		"tx-123",
		"FOOD_AND_DRINK",
		"5812",
		"online",
	)

	if tx.Provider != financetxscan.ProviderTrueLayer {
		t.Errorf("Expected Provider to be ProviderTrueLayer, got %v", tx.Provider)
	}

	if tx.TransactionID != "tx-123" {
		t.Errorf("Expected TransactionID 'tx-123', got '%s'", tx.TransactionID)
	}

	if tx.ProviderCategory != "FOOD_AND_DRINK" {
		t.Errorf("Expected ProviderCategory 'FOOD_AND_DRINK', got '%s'", tx.ProviderCategory)
	}
}

// TestEmptyTransactionsStillWork verifies empty input doesn't crash.
func TestEmptyTransactionsStillWork(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := financetxscan.NewEngine(clock)

	result := engine.BuildFromTransactions(
		"circle_test",
		"2025-W03",
		"sync_hash_123",
		nil,
	)

	// Empty input should return empty result, not rejection
	if result.StatusHash == "rejected_mock_provider" {
		t.Error("Empty input should not be treated as mock rejection")
	}

	if len(result.Observations) != 0 {
		t.Errorf("Expected 0 observations for empty input, got %d", len(result.Observations))
	}
}

// TestDeterminismWithRealProvider verifies deterministic output with real provider.
func TestDeterminismWithRealProvider(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	engine := financetxscan.NewEngine(clock)

	transactions := []financetxscan.TransactionData{
		financetxscan.ExtractTransactionData(
			financetxscan.ProviderTrueLayer,
			"tx-001",
			"FOOD_AND_DRINK",
			"5812",
			"online",
		),
		financetxscan.ExtractTransactionData(
			financetxscan.ProviderTrueLayer,
			"tx-002",
			"TRANSPORT",
			"4121",
			"contactless",
		),
	}

	// Run twice
	result1 := engine.BuildFromTransactions("circle_test", "2025-W03", "sync_hash_123", transactions)
	result2 := engine.BuildFromTransactions("circle_test", "2025-W03", "sync_hash_123", transactions)

	// Verify determinism
	if result1.StatusHash != result2.StatusHash {
		t.Errorf("Status hashes differ: %s vs %s", result1.StatusHash, result2.StatusHash)
	}

	if len(result1.Observations) != len(result2.Observations) {
		t.Errorf("Observation counts differ: %d vs %d", len(result1.Observations), len(result2.Observations))
	}
}
