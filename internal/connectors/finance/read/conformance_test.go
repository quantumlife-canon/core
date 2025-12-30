// Package read_test provides conformance tests for finance read connectors.
// These tests verify that all providers (mock, truelayer, plaid) implement
// the ReadConnector interface correctly and enforce v8 guardrails.
package read_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/internal/connectors/finance/read/providers/mock"
	"quantumlife/internal/connectors/finance/read/providers/plaid"
	"quantumlife/internal/connectors/finance/read/providers/truelayer"
	"quantumlife/pkg/primitives"
)

// ConnectorFactory creates a ReadConnector for testing.
type ConnectorFactory func(t *testing.T) read.ReadConnector

// conformanceTestCase defines a test case for the conformance suite.
type conformanceTestCase struct {
	name    string
	factory ConnectorFactory
}

// getConformanceTestCases returns all provider test cases.
func getConformanceTestCases(t *testing.T) []conformanceTestCase {
	return []conformanceTestCase{
		{
			name: "mock",
			factory: func(t *testing.T) read.ReadConnector {
				return mock.NewConnector(mock.Config{
					ProviderID: "conformance-mock",
					Seed:       "conformance-test",
				})
			},
		},
		{
			name: "truelayer",
			factory: func(t *testing.T) read.ReadConnector {
				// Create mock TrueLayer server
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/data/v1/accounts":
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"results":[{"account_id":"acc-123","display_name":"Test Account","account_type":"CHECKING","currency":"GBP"}],"status":"Succeeded"}`))
					case "/data/v1/accounts/acc-123/transactions":
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"results":[{"transaction_id":"tx-123","timestamp":"2025-01-15T10:00:00Z","amount":-25.00,"currency":"GBP","description":"Test Transaction"}],"status":"Succeeded"}`))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				t.Cleanup(server.Close)

				client, err := truelayer.NewClient(truelayer.ClientConfig{
					Environment:  "sandbox",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					HTTPClient:   server.Client(),
				})
				if err != nil {
					t.Fatalf("failed to create TrueLayer client: %v", err)
				}

				// Override base URL for testing
				client.SetBaseURL(server.URL)

				return truelayer.NewConnector(truelayer.ConnectorConfig{
					Client:      client,
					AccessToken: "test-token",
					ProviderID:  "conformance-truelayer",
				})
			},
		},
		{
			name: "plaid",
			factory: func(t *testing.T) read.ReadConnector {
				// Create mock Plaid server
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/accounts/get":
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"accounts":[{"account_id":"plaid-acc-123","name":"Test Account","type":"depository","subtype":"checking","mask":"1234","balances":{"current":1000.00,"available":900.00,"iso_currency_code":"USD"}}]}`))
					case "/transactions/get":
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"accounts":[],"transactions":[{"transaction_id":"plaid-tx-123","account_id":"plaid-acc-123","date":"2025-01-15","amount":25.00,"iso_currency_code":"USD","name":"Test Transaction","pending":false}],"total_transactions":1}`))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				t.Cleanup(server.Close)

				client, err := plaid.NewClient(plaid.ClientConfig{
					Environment: "sandbox",
					ClientID:    "test-client-id",
					Secret:      "test-secret",
					HTTPClient:  server.Client(),
				})
				if err != nil {
					t.Fatalf("failed to create Plaid client: %v", err)
				}

				// Override base URL for testing
				client.SetBaseURL(server.URL)

				return plaid.NewConnector(plaid.ConnectorConfig{
					Client:      client,
					AccessToken: "test-access-token",
					ProviderID:  "conformance-plaid",
				})
			},
		},
	}
}

// validEnvelope creates a valid execution envelope for testing.
func validEnvelope() primitives.ExecutionEnvelope {
	return primitives.ExecutionEnvelope{
		Mode:          primitives.ModeSuggestOnly,
		TraceID:       "trace-conformance-123",
		ActorCircleID: "circle-conformance",
		ScopesUsed:    []string{read.ScopeFinanceRead},
	}
}

// TestConformance_ListAccounts tests ListAccounts for all providers.
func TestConformance_ListAccounts(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()

			receipt, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{
				IncludeBalances: true,
			})

			if err != nil {
				t.Fatalf("ListAccounts failed: %v", err)
			}

			if receipt == nil {
				t.Fatal("expected receipt, got nil")
			}

			if len(receipt.Accounts) == 0 {
				t.Error("expected at least one account")
			}

			// Verify accounts have expected fields
			for i, acc := range receipt.Accounts {
				if acc.AccountID == "" {
					t.Errorf("account[%d]: AccountID should not be empty", i)
				}
				if acc.Name == "" {
					t.Errorf("account[%d]: Name should not be empty", i)
				}
			}

			if receipt.ProviderID == "" {
				t.Error("ProviderID should not be empty")
			}
		})
	}
}

// TestConformance_ListTransactions tests ListTransactions for all providers.
func TestConformance_ListTransactions(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			now := time.Now()

			receipt, err := connector.ListTransactions(ctx, env, read.ListTransactionsRequest{
				StartDate: now.AddDate(0, 0, -30),
				EndDate:   now,
			})

			if err != nil {
				t.Fatalf("ListTransactions failed: %v", err)
			}

			if receipt == nil {
				t.Fatal("expected receipt, got nil")
			}

			// Verify provider ID is set
			if receipt.ProviderID == "" {
				t.Error("ProviderID should not be empty")
			}
		})
	}
}

// TestConformance_RejectsExecuteMode verifies all providers reject ModeExecute.
// CRITICAL: This is a v8 guardrail - finance read MUST reject execute mode.
func TestConformance_RejectsExecuteMode(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name+"_ListAccounts", func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			env.Mode = primitives.ModeExecute // MUST be rejected

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

			if err == nil {
				t.Error("CRITICAL: ListAccounts should reject ModeExecute")
			}
		})

		t.Run(tc.name+"_ListTransactions", func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			env.Mode = primitives.ModeExecute // MUST be rejected

			_, err := connector.ListTransactions(ctx, env, read.ListTransactionsRequest{
				StartDate: time.Now().AddDate(0, 0, -30),
				EndDate:   time.Now(),
			})

			if err == nil {
				t.Error("CRITICAL: ListTransactions should reject ModeExecute")
			}
		})
	}
}

// TestConformance_RejectsForbiddenScopes verifies all providers reject forbidden scopes.
// CRITICAL: This is a v8 guardrail - finance read MUST reject write/payment scopes.
func TestConformance_RejectsForbiddenScopes(t *testing.T) {
	ctx := context.Background()
	forbiddenScopes := []string{"finance:write", "finance:execute", "finance:transfer", "payment:initiate"}

	for _, tc := range getConformanceTestCases(t) {
		for _, forbiddenScope := range forbiddenScopes {
			t.Run(tc.name+"_"+forbiddenScope, func(t *testing.T) {
				connector := tc.factory(t)
				env := validEnvelope()
				env.ScopesUsed = append(env.ScopesUsed, forbiddenScope)

				_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

				if err == nil {
					t.Errorf("CRITICAL: ListAccounts should reject forbidden scope %q", forbiddenScope)
				}
			})
		}
	}
}

// TestConformance_RequiresTraceID verifies all providers require trace ID.
func TestConformance_RequiresTraceID(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			env.TraceID = "" // Missing trace ID

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

			if err == nil {
				t.Error("ListAccounts should require trace ID")
			}
		})
	}
}

// TestConformance_RequiresActorCircleID verifies all providers require actor circle ID.
func TestConformance_RequiresActorCircleID(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			env.ActorCircleID = "" // Missing actor circle ID

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

			if err == nil {
				t.Error("ListAccounts should require actor circle ID")
			}
		})
	}
}

// TestConformance_Supports verifies all providers report correct capabilities.
func TestConformance_Supports(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			caps := connector.Supports(ctx)

			if !caps.Read {
				t.Error("expected Read capability to be true")
			}

			// CRITICAL: There should be no Write field (by design)
			// This is verified at compile time since the Capabilities struct
			// doesn't have a Write field
		})
	}
}

// TestConformance_ProviderInfo verifies all providers return valid provider info.
func TestConformance_ProviderInfo(t *testing.T) {
	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			info := connector.ProviderInfo()

			if info.ID == "" {
				t.Error("ProviderInfo.ID should not be empty")
			}

			if info.Type == "" {
				t.Error("ProviderInfo.Type should not be empty")
			}
		})
	}
}

// TestConformance_SimulateMode verifies all providers accept ModeSimulate.
func TestConformance_SimulateMode(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			env.Mode = primitives.ModeSimulate

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

			if err != nil {
				t.Errorf("ListAccounts should accept ModeSimulate: %v", err)
			}
		})
	}
}

// TestConformance_SuggestOnlyMode verifies all providers accept ModeSuggestOnly.
func TestConformance_SuggestOnlyMode(t *testing.T) {
	ctx := context.Background()

	for _, tc := range getConformanceTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			connector := tc.factory(t)
			env := validEnvelope()
			env.Mode = primitives.ModeSuggestOnly

			_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

			if err != nil {
				t.Errorf("ListAccounts should accept ModeSuggestOnly: %v", err)
			}
		})
	}
}
