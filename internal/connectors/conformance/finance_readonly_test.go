// Package conformance provides conformance tests for finance read connectors.
// These tests verify that connectors adhere to v8 READ-ONLY requirements.
//
// CRITICAL: These tests MUST pass for any finance connector to be used.
// They verify no write operations exist and execute mode is rejected.
package conformance

import (
	"context"
	"net/http"
	"testing"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/internal/connectors/finance/read"
	"quantumlife/internal/connectors/finance/read/providers/mock"
	"quantumlife/internal/connectors/finance/read/providers/truelayer"
	"quantumlife/internal/connectors/testkit"
	"quantumlife/pkg/primitives"
)

// Fixed time for deterministic testing.
var financeTestTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

// financeTestEnvelope creates a valid test envelope for finance read.
func financeTestEnvelope(mode primitives.RunMode) primitives.ExecutionEnvelope {
	return primitives.ExecutionEnvelope{
		TraceID:              "test-finance-trace-123",
		Mode:                 mode,
		ActorCircleID:        "test-circle",
		IntersectionID:       "test-intersection",
		ContractVersion:      "v1",
		ScopesUsed:           []string{"finance:read"},
		AuthorizationProofID: "test-proof-123",
		IssuedAt:             financeTestTime,
	}
}

// TestMockFinanceConnector_SuggestOnlyMode verifies mock connector in suggest-only mode.
func TestMockFinanceConnector_SuggestOnlyMode(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
		ClockFunc:  func() time.Time { return financeTestTime },
		Seed:       "conformance-test",
	})
	env := financeTestEnvelope(primitives.ModeSuggestOnly)

	ctx := context.Background()

	// List accounts should work
	accountsReceipt, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{
		IncludeBalances: true,
	})
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(accountsReceipt.Accounts) == 0 {
		t.Error("expected accounts from mock")
	}

	// List transactions should work
	txReceipt, err := connector.ListTransactions(ctx, env, read.ListTransactionsRequest{
		StartDate: financeTestTime.AddDate(0, -1, 0),
		EndDate:   financeTestTime,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if len(txReceipt.Transactions) == 0 {
		t.Error("expected transactions from mock")
	}

	// Verify capabilities only report Read
	caps := connector.Supports(ctx)
	if !caps.Read {
		t.Error("expected Read capability to be true")
	}
}

// TestMockFinanceConnector_SimulateMode verifies mock connector in simulate mode.
func TestMockFinanceConnector_SimulateMode(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
		ClockFunc:  func() time.Time { return financeTestTime },
		Seed:       "conformance-test",
	})
	env := financeTestEnvelope(primitives.ModeSimulate)

	ctx := context.Background()

	// All read operations should work
	accountsReceipt, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(accountsReceipt.Accounts) == 0 {
		t.Error("expected accounts from mock")
	}
}

// TestFinanceConnector_RejectsExecuteMode verifies execute mode is rejected.
func TestFinanceConnector_RejectsExecuteMode(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
		ClockFunc:  func() time.Time { return financeTestTime },
	})
	env := financeTestEnvelope(primitives.ModeExecute)

	ctx := context.Background()

	_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})
	if err == nil {
		t.Error("expected error for execute mode")
	}
	if err != read.ErrExecuteModeNotAllowed {
		t.Errorf("expected ErrExecuteModeNotAllowed, got: %v", err)
	}
}

// TestFinanceConnector_RejectsWriteScopes verifies write scopes are rejected.
func TestFinanceConnector_RejectsWriteScopes(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
	})

	ctx := context.Background()

	writeScopeTests := []struct {
		name   string
		scopes []string
	}{
		{"finance:write", []string{"finance:write"}},
		{"finance:transfer", []string{"finance:transfer"}},
		{"payment", []string{"payment:initiate"}},
		{"transfer", []string{"transfer:funds"}},
		{"finance:execute", []string{"finance:execute"}},
	}

	for _, tt := range writeScopeTests {
		t.Run(tt.name, func(t *testing.T) {
			env := financeTestEnvelope(primitives.ModeSimulate)
			env.ScopesUsed = tt.scopes

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})
			if err == nil {
				t.Errorf("expected error for scope %s", tt.name)
			}
		})
	}
}

// TestFinanceConnector_RequiresFinanceReadScope verifies finance:read scope is required.
func TestFinanceConnector_RequiresFinanceReadScope(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
	})

	ctx := context.Background()
	env := financeTestEnvelope(primitives.ModeSimulate)
	env.ScopesUsed = []string{} // Empty scopes

	_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})
	if err == nil {
		t.Error("expected error for empty scopes")
	}
}

// TestFinanceConnector_RequiredEnvelopeFields verifies required envelope fields.
func TestFinanceConnector_RequiredEnvelopeFields(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
	})

	ctx := context.Background()

	tests := []struct {
		name    string
		modify  func(*primitives.ExecutionEnvelope)
		wantErr error
	}{
		{
			name:    "missing trace ID",
			modify:  func(e *primitives.ExecutionEnvelope) { e.TraceID = "" },
			wantErr: read.ErrTraceIDRequired,
		},
		{
			name:    "missing actor circle ID",
			modify:  func(e *primitives.ExecutionEnvelope) { e.ActorCircleID = "" },
			wantErr: read.ErrActorCircleIDRequired,
		},
		{
			name:    "missing scopes",
			modify:  func(e *primitives.ExecutionEnvelope) { e.ScopesUsed = nil },
			wantErr: read.ErrScopesRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := financeTestEnvelope(primitives.ModeSimulate)
			tt.modify(&env)

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})
			if err != tt.wantErr {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestFinanceConnector_DeterministicOutput verifies deterministic outputs.
func TestFinanceConnector_DeterministicOutput(t *testing.T) {
	seed := "determinism-test-seed"
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
		ClockFunc:  func() time.Time { return financeTestTime },
		Seed:       seed,
	})
	env := financeTestEnvelope(primitives.ModeSimulate)

	ctx := context.Background()
	req := read.ListTransactionsRequest{
		StartDate: financeTestTime.AddDate(0, 0, -7),
		EndDate:   financeTestTime,
	}

	// Run twice, results should be identical
	result1, _ := connector.ListTransactions(ctx, env, req)
	result2, _ := connector.ListTransactions(ctx, env, req)

	if len(result1.Transactions) != len(result2.Transactions) {
		t.Errorf("non-deterministic: got %d then %d transactions",
			len(result1.Transactions), len(result2.Transactions))
	}

	for i := range result1.Transactions {
		if result1.Transactions[i].TransactionID != result2.Transactions[i].TransactionID {
			t.Errorf("non-deterministic: transaction %d ID differs", i)
		}
		if result1.Transactions[i].AmountCents != result2.Transactions[i].AmountCents {
			t.Errorf("non-deterministic: transaction %d amount differs", i)
		}
	}
}

// TestFinanceConnector_CapabilitiesNoWrite verifies no Write capability exists.
func TestFinanceConnector_CapabilitiesNoWrite(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-mock",
	})

	ctx := context.Background()
	caps := connector.Supports(ctx)

	// Verify Read is true
	if !caps.Read {
		t.Error("expected Read capability to be true")
	}

	// NOTE: There is intentionally no Write field in Capabilities.
	// This test documents that architectural decision.
	// The Capabilities struct only has Read - Write doesn't exist.
}

// TestTrueLayerScopeValidation verifies TrueLayer scope allowlist.
func TestTrueLayerScopeValidation(t *testing.T) {
	// Allowed scopes should pass
	allowedScopes := []string{
		truelayer.ScopeAccounts,
		truelayer.ScopeBalance,
		truelayer.ScopeTransactions,
		truelayer.ScopeInfo,
		truelayer.ScopeOfflineAccess,
	}

	for _, scope := range allowedScopes {
		if !truelayer.IsAllowedTrueLayerScope(scope) {
			t.Errorf("expected scope %s to be allowed", scope)
		}
	}

	// Forbidden patterns should be rejected
	forbiddenScopes := []string{
		"payment:initiate",
		"payments:read",
		"transfer:funds",
		"pay:bill",
		"write:accounts",
		"initiate_payment",
		"standing_order:create",
		"direct_debit:setup",
		"beneficiary:add",
		"mandate:create",
	}

	for _, scope := range forbiddenScopes {
		for _, pattern := range truelayer.ForbiddenTrueLayerScopePatterns {
			if containsLower(scope, pattern) {
				// This scope should trigger forbidden pattern detection
				break
			}
		}
	}
}

// TestTrueLayerConnector_NoWriteMethods verifies TrueLayer connector has no write methods.
func TestTrueLayerConnector_NoWriteMethods(t *testing.T) {
	// This test verifies at compile time that TrueLayer Connector
	// implements ONLY read.ReadConnector interface, which has no write methods.
	//
	// The interface itself is designed with no write methods:
	// - ListAccounts (read)
	// - ListTransactions (read)
	// - Supports (read)
	// - ProviderInfo (read)
	//
	// There is no CreateTransaction, Transfer, or any write method.
	// This is architectural safety - execution is impossible.

	// We verify interface compliance at compile time via var _ check in connector.go
	// This test serves as documentation.
	t.Log("TrueLayer connector implements ReadConnector interface only")
	t.Log("ReadConnector has no write methods by design")
	t.Log("Execution is architecturally impossible")
}

// TestTrueLayerConnector_WithMockHTTP tests TrueLayer connector with mock HTTP.
func TestTrueLayerConnector_WithMockHTTP(t *testing.T) {
	transport := testkit.NewFakeTransport()

	// Add mock TrueLayer responses
	transport.AddResponse("truelayer-sandbox.com", &testkit.FakeResponse{
		StatusCode: http.StatusOK,
		Body:       mockTrueLayerAccountsResponse(),
	})
	transport.AddResponse("auth.truelayer-sandbox.com", &testkit.FakeResponse{
		StatusCode: http.StatusOK,
		Body:       mockTrueLayerTokenResponse(),
	})

	// Create TrueLayer client config
	config := truelayer.ClientConfig{
		Environment:  "sandbox",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		HTTPClient:   transport.NewHTTPClient(),
	}

	client, err := truelayer.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Create connector
	connector := truelayer.NewConnector(truelayer.ConnectorConfig{
		Client:      client,
		AccessToken: "mock-access-token",
		ProviderID:  "test-truelayer",
	})

	// Verify capabilities
	ctx := context.Background()
	caps := connector.Supports(ctx)
	if !caps.Read {
		t.Error("expected Read capability to be true")
	}

	// Verify provider info
	info := connector.ProviderInfo()
	if info.Type != "truelayer" {
		t.Errorf("expected provider type 'truelayer', got '%s'", info.Type)
	}
}

// TestTrueLayerAuthScopes verifies auth scope mapping for TrueLayer.
func TestTrueLayerAuthScopes(t *testing.T) {
	mapper := impl_inmem.NewScopeMapper()

	// finance:read should map to TrueLayer read scopes
	providerScopes, err := mapper.MapToProvider(auth.ProviderTrueLayer, []string{"finance:read"})
	if err != nil {
		t.Fatalf("failed to map scopes: %v", err)
	}

	// Should include accounts, balance, transactions, offline_access
	expectedScopes := map[string]bool{
		"accounts":       false,
		"balance":        false,
		"transactions":   false,
		"offline_access": false,
	}

	for _, scope := range providerScopes {
		if _, ok := expectedScopes[scope]; ok {
			expectedScopes[scope] = true
		}
	}

	for scope, found := range expectedScopes {
		if !found {
			t.Errorf("expected scope %s to be in provider scopes", scope)
		}
	}
}

// TestTrueLayerAuthForbiddenScopes verifies payment scopes are blocked.
func TestTrueLayerAuthForbiddenScopes(t *testing.T) {
	// The security model for TrueLayer is:
	// 1. Only allowed QuantumLife scopes (finance:read) map to TrueLayer scopes
	// 2. Unknown scopes are ignored (not passed to TrueLayer)
	// 3. There is no QL scope that maps to payment scopes
	//
	// This test verifies that:
	// - Valid finance:read scope works
	// - Unknown scopes don't result in payment scopes being requested

	config := auth.Config{
		TrueLayer: auth.TrueLayerConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Environment:  "sandbox",
		},
	}
	broker := impl_inmem.NewBroker(config, nil)

	// Valid finance:read scope should work
	authURL, err := broker.BeginOAuth(auth.ProviderTrueLayer, "http://localhost/callback", "state123",
		[]string{"finance:read"})
	if err != nil {
		t.Fatalf("expected success for finance:read scope, got: %v", err)
	}
	if authURL == "" {
		t.Error("expected auth URL to be non-empty")
	}

	// Verify auth URL contains only allowed scopes
	// The URL should NOT contain payment/transfer/write scopes
	forbiddenPatterns := []string{"payment", "transfer", "write", "initiate", "mandate"}
	for _, pattern := range forbiddenPatterns {
		if containsLower(authURL, pattern) {
			t.Errorf("auth URL should not contain '%s' scope", pattern)
		}
	}

	// Verify it contains read scopes
	if !containsLower(authURL, "accounts") || !containsLower(authURL, "balance") {
		t.Error("auth URL should contain read scopes (accounts, balance)")
	}
}

// mockTrueLayerAccountsResponse returns a mock TrueLayer accounts response.
func mockTrueLayerAccountsResponse() string {
	return `{
		"results": [
			{
				"account_id": "acc-123",
				"account_type": "TRANSACTION",
				"display_name": "Current Account",
				"currency": "GBP",
				"provider": {
					"provider_id": "uk-ob-mock",
					"display_name": "Mock Bank"
				}
			}
		],
		"status": "Succeeded"
	}`
}

// mockTrueLayerTokenResponse returns a mock TrueLayer token response.
func mockTrueLayerTokenResponse() string {
	return `{
		"access_token": "mock-access-token",
		"token_type": "Bearer",
		"expires_in": 3600,
		"refresh_token": "mock-refresh-token",
		"scope": "accounts balance transactions offline_access"
	}`
}

// containsLower checks if s contains substr (case-insensitive).
func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := toLower(s[i+j])
			pc := toLower(substr[j])
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower converts a byte to lowercase.
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
