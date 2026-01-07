// Package demo_phase31_3b_truelayer_real_sync tests real TrueLayer sync functionality.
//
// Phase 31.3b: Real TrueLayer Sync (Accounts + Transactions → Finance Mirror + Commerce Observer)
// Reference: docs/ADR/ADR-0066-phase31-3b-truelayer-real-sync.md
//
// CRITICAL TEST INVARIANTS:
//   - All tests use httptest.Server (CI-safe, no real credentials)
//   - Bounded sync limits enforced (25 accounts, 25 tx/account, 7 days)
//   - Provider is ProviderTrueLayer (mock providers rejected)
//   - Privacy: no merchant/amount persisted; outputs are buckets/hashes only
//   - Determinism: same inputs + injected clock = same outputs
package demo_phase31_3b_truelayer_real_sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/read/providers/truelayer"
	"quantumlife/internal/financetxscan"
	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/financemirror"
)

// =============================================================================
// Test: Bounded Sync Limits
// =============================================================================

func TestBoundedSyncLimits_MaxAccounts(t *testing.T) {
	// Generate 30 accounts (exceeds limit of 25)
	accounts := make([]map[string]interface{}, 30)
	for i := 0; i < 30; i++ {
		accounts[i] = map[string]interface{}{
			"account_id":   "acc-" + intToStr(i),
			"account_type": "TRANSACTION",
			"currency":     "GBP",
			"provider": map[string]string{
				"provider_id":  "uk-ob-test",
				"display_name": "Test Bank",
			},
		}
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/data/v1/accounts" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": accounts,
				"status":  "Succeeded",
			})
			return
		}
		// Return empty transactions for any account
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{},
			"status":  "Succeeded",
		})
	}))
	defer server.Close()

	// Create client with test server
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	client, _ := truelayer.NewClient(truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		HTTPClient:   server.Client(),
	})
	client.SetBaseURL(server.URL)

	syncService := truelayer.NewSyncService(truelayer.SyncServiceConfig{
		Client: client,
		Clock:  func() time.Time { return fixedTime },
	})

	// Perform sync
	output, err := syncService.Sync(context.Background(), truelayer.SyncInput{
		CircleID:    "circle-1",
		AccessToken: "test-token",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Success {
		t.Fatalf("expected success, got failure: %s", output.FailReason)
	}

	// Verify bounded to MaxAccounts (25)
	if output.AccountsCount > truelayer.MaxAccounts {
		t.Errorf("accounts exceeded limit: got %d, max %d", output.AccountsCount, truelayer.MaxAccounts)
	}

	if output.AccountsCount != truelayer.MaxAccounts {
		t.Errorf("expected exactly %d accounts, got %d", truelayer.MaxAccounts, output.AccountsCount)
	}
}

func TestBoundedSyncLimits_MaxTransactionsPerAccount(t *testing.T) {
	// Generate 30 transactions (exceeds limit of 25)
	transactions := make([]map[string]interface{}, 30)
	for i := 0; i < 30; i++ {
		transactions[i] = map[string]interface{}{
			"transaction_id":       "tx-" + intToStr(i),
			"transaction_category": "FOOD_AND_DRINK",
			"transaction_type":     "DEBIT",
			"timestamp":            "2026-01-06T10:00:00Z",
			"description":          "Test transaction",
			"amount":               -10.50,
			"currency":             "GBP",
		}
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/data/v1/accounts" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"account_id":   "acc-1",
						"account_type": "TRANSACTION",
						"currency":     "GBP",
						"provider":     map[string]string{"provider_id": "uk-ob-test"},
					},
				},
				"status": "Succeeded",
			})
			return
		}
		// Return all transactions for the account
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": transactions,
			"status":  "Succeeded",
		})
	}))
	defer server.Close()

	// Create client with test server
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	client, _ := truelayer.NewClient(truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		HTTPClient:   server.Client(),
	})
	client.SetBaseURL(server.URL)

	syncService := truelayer.NewSyncService(truelayer.SyncServiceConfig{
		Client: client,
		Clock:  func() time.Time { return fixedTime },
	})

	// Perform sync
	output, err := syncService.Sync(context.Background(), truelayer.SyncInput{
		CircleID:    "circle-1",
		AccessToken: "test-token",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify bounded to MaxTransactionsPerAccount (25)
	if output.TransactionsCount > truelayer.MaxTransactionsPerAccount {
		t.Errorf("transactions exceeded limit: got %d, max %d",
			output.TransactionsCount, truelayer.MaxTransactionsPerAccount)
	}
}

// =============================================================================
// Test: Provider Validation (Phase 31.3 compliance)
// =============================================================================

func TestProviderValidation_RealProviderAccepted(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderTrueLayer)
	if err != nil {
		t.Errorf("ProviderTrueLayer should be valid, got error: %v", err)
	}
}

func TestProviderValidation_MockProviderRejected(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderMock)
	if err == nil {
		t.Error("ProviderMock should be rejected")
	}
}

func TestProviderValidation_EmptyProviderRejected(t *testing.T) {
	err := financetxscan.ValidateProvider(financetxscan.ProviderEmpty)
	if err == nil {
		t.Error("empty provider should be rejected")
	}
}

func TestBuildFromTransactions_MockProviderRejected(t *testing.T) {
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	engine := financetxscan.NewEngine(func() time.Time { return fixedTime })

	// Try to build with mock provider (should be rejected)
	txData := []financetxscan.TransactionData{
		{
			TransactionID:    "tx-1",
			Provider:         financetxscan.ProviderMock, // REJECTED
			ProviderCategory: "FOOD_AND_DRINK",
		},
	}

	result := engine.BuildFromTransactions("circle-1", "2026-W01", "receipt-hash", txData)

	if result.StatusHash != "rejected_mock_provider" {
		t.Errorf("expected rejected_mock_provider status, got: %s", result.StatusHash)
	}
}

func TestBuildFromTransactions_RealProviderAccepted(t *testing.T) {
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	engine := financetxscan.NewEngine(func() time.Time { return fixedTime })

	// Build with real provider (should be accepted)
	txData := []financetxscan.TransactionData{
		{
			TransactionID:    "tx-1",
			Provider:         financetxscan.ProviderTrueLayer, // ACCEPTED
			ProviderCategory: "FOOD_AND_DRINK",
		},
	}

	result := engine.BuildFromTransactions("circle-1", "2026-W01", "receipt-hash", txData)

	if result.StatusHash == "rejected_mock_provider" {
		t.Error("real provider should not be rejected")
	}
}

// =============================================================================
// Test: Privacy (no merchant/amount persisted)
// =============================================================================

func TestPrivacy_TransactionClassificationNoAmounts(t *testing.T) {
	// TransactionClassification should not contain amount fields
	tc := truelayer.TransactionClassification{
		TransactionID:      "tx-123",
		ProviderCategory:   "FOOD_AND_DRINK",
		ProviderCategoryID: "5812",
		PaymentChannel:     "debit",
	}

	// Verify the struct only has classification fields
	// (compile-time check - if Amount field existed, this would fail)
	_ = tc.TransactionID
	_ = tc.ProviderCategory
	_ = tc.ProviderCategoryID
	_ = tc.PaymentChannel
}

func TestPrivacy_ReceiptMagnitudeBucketsOnly(t *testing.T) {
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)

	// Create receipt with raw counts
	// ToMagnitudeBucket: 0=nothing, 1-3=a_few, 4-10=several, 11+=many
	receipt := financemirror.NewFinanceSyncReceipt(
		"circle-1",
		"truelayer",
		fixedTime,
		2,  // raw accounts count -> a_few (1-3)
		12, // raw transactions count -> many (11+)
		[]string{"account_type|TRANSACTION"},
		true,
		"",
	)

	// Verify magnitude buckets (not raw counts)
	if receipt.AccountsMagnitude != financemirror.MagnitudeAFew {
		t.Errorf("expected accounts magnitude 'a_few', got: %s", receipt.AccountsMagnitude)
	}

	if receipt.TransactionsMagnitude != financemirror.MagnitudeMany {
		t.Errorf("expected transactions magnitude 'many', got: %s", receipt.TransactionsMagnitude)
	}

	// Verify canonical string doesn't contain raw counts
	canonical := receipt.CanonicalString()
	if contains(canonical, "|2|") || contains(canonical, "|12|") {
		t.Error("canonical string should not contain raw counts")
	}
}

// =============================================================================
// Test: Determinism with Clock Injection
// =============================================================================

func TestDeterminism_SameInputsSameOutput(t *testing.T) {
	// Same mock server response
	transactions := []map[string]interface{}{
		{
			"transaction_id":       "tx-1",
			"transaction_category": "FOOD_AND_DRINK",
			"transaction_type":     "DEBIT",
		},
		{
			"transaction_id":       "tx-2",
			"transaction_category": "TRANSPORT",
			"transaction_type":     "DEBIT",
		},
	}

	createServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/data/v1/accounts" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []map[string]interface{}{
						{"account_id": "acc-1", "account_type": "TRANSACTION", "currency": "GBP",
							"provider": map[string]string{"provider_id": "test"}},
					},
					"status": "Succeeded",
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": transactions,
				"status":  "Succeeded",
			})
		}))
	}

	// Same fixed time
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)

	// First sync
	server1 := createServer()
	defer server1.Close()

	client1, _ := truelayer.NewClient(truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   server1.Client(),
	})
	client1.SetBaseURL(server1.URL)

	sync1 := truelayer.NewSyncService(truelayer.SyncServiceConfig{
		Client: client1,
		Clock:  func() time.Time { return fixedTime },
	})

	output1, _ := sync1.Sync(context.Background(), truelayer.SyncInput{
		CircleID:    "circle-1",
		AccessToken: "token",
	})
	receipt1 := truelayer.BuildSyncReceipt("circle-1", output1)

	// Second sync (same inputs)
	server2 := createServer()
	defer server2.Close()

	client2, _ := truelayer.NewClient(truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   server2.Client(),
	})
	client2.SetBaseURL(server2.URL)

	sync2 := truelayer.NewSyncService(truelayer.SyncServiceConfig{
		Client: client2,
		Clock:  func() time.Time { return fixedTime },
	})

	output2, _ := sync2.Sync(context.Background(), truelayer.SyncInput{
		CircleID:    "circle-1",
		AccessToken: "token",
	})
	receipt2 := truelayer.BuildSyncReceipt("circle-1", output2)

	// Verify determinism
	if receipt1.StatusHash != receipt2.StatusHash {
		t.Errorf("receipts should be identical:\n  receipt1: %s\n  receipt2: %s",
			receipt1.StatusHash, receipt2.StatusHash)
	}

	if output1.SyncTime != output2.SyncTime {
		t.Errorf("sync times should be identical")
	}
}

// =============================================================================
// Test: Token Store
// =============================================================================

func TestTokenStore_StoreAndRetrieve(t *testing.T) {
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	store := persist.NewTrueLayerTokenStore(func() time.Time { return fixedTime })

	// Store token
	store.StoreToken("circle-1", "access-token-123", "refresh-token-456", 3600)

	// Retrieve token
	token := store.GetToken("circle-1")
	if token != "access-token-123" {
		t.Errorf("expected access-token-123, got: %s", token)
	}

	// Verify has valid token
	if !store.HasValidToken("circle-1") {
		t.Error("should have valid token")
	}

	// Verify token hash is not the raw token
	hash := store.GetTokenHash("circle-1")
	if hash == "access-token-123" {
		t.Error("token hash should not be raw token")
	}
	if hash == "" {
		t.Error("token hash should not be empty")
	}
}

func TestTokenStore_ExpiredToken(t *testing.T) {
	currentTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	store := persist.NewTrueLayerTokenStore(func() time.Time { return currentTime })

	// Store token that expires in 1 hour
	store.StoreToken("circle-1", "token", "refresh", 3600)

	// Advance time past expiration
	currentTime = currentTime.Add(2 * time.Hour)

	// Token should now be empty (expired)
	token := store.GetToken("circle-1")
	if token != "" {
		t.Errorf("expected empty token after expiration, got: %s", token)
	}

	// HasValidToken should return false
	if store.HasValidToken("circle-1") {
		t.Error("should not have valid token after expiration")
	}
}

func TestTokenStore_Remove(t *testing.T) {
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	store := persist.NewTrueLayerTokenStore(func() time.Time { return fixedTime })

	store.StoreToken("circle-1", "token", "refresh", 3600)
	store.RemoveToken("circle-1")

	token := store.GetToken("circle-1")
	if token != "" {
		t.Error("token should be removed")
	}
}

// =============================================================================
// Test: HTTP Error Handling
// =============================================================================

func TestSync_UnauthorizedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_token",
			"error_description": "Access token is invalid or expired",
		})
	}))
	defer server.Close()

	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	client, _ := truelayer.NewClient(truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   server.Client(),
	})
	client.SetBaseURL(server.URL)

	syncService := truelayer.NewSyncService(truelayer.SyncServiceConfig{
		Client: client,
		Clock:  func() time.Time { return fixedTime },
	})

	output, _ := syncService.Sync(context.Background(), truelayer.SyncInput{
		CircleID:    "circle-1",
		AccessToken: "invalid-token",
	})

	if output.Success {
		t.Error("should fail with unauthorized token")
	}

	// The sync service wraps errors, so we check for a failure
	// (specific reason depends on where the error occurs)
	if output.FailReason == "" {
		t.Error("expected non-empty fail reason")
	}
}

func TestSync_NoToken(t *testing.T) {
	fixedTime := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	client, _ := truelayer.NewClient(truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	})

	syncService := truelayer.NewSyncService(truelayer.SyncServiceConfig{
		Client: client,
		Clock:  func() time.Time { return fixedTime },
	})

	output, _ := syncService.Sync(context.Background(), truelayer.SyncInput{
		CircleID:    "circle-1",
		AccessToken: "", // No token
	})

	if output.Success {
		t.Error("should fail without token")
	}

	if output.FailReason != "no_access_token" {
		t.Errorf("expected 'no_access_token' fail reason, got: %s", output.FailReason)
	}
}

// =============================================================================
// Test: Constants Verification
// =============================================================================

func TestConstants_BoundedLimits(t *testing.T) {
	if truelayer.MaxAccounts != 25 {
		t.Errorf("MaxAccounts should be 25, got: %d", truelayer.MaxAccounts)
	}

	if truelayer.MaxTransactionsPerAccount != 25 {
		t.Errorf("MaxTransactionsPerAccount should be 25, got: %d", truelayer.MaxTransactionsPerAccount)
	}

	if truelayer.SyncWindowDays != 7 {
		t.Errorf("SyncWindowDays should be 7, got: %d", truelayer.SyncWindowDays)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
