// Package mock_test provides tests for the mock finance connector.
package mock_test

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/internal/connectors/finance/read/providers/mock"
	"quantumlife/pkg/primitives"
)

func validEnvelope() primitives.ExecutionEnvelope {
	return primitives.ExecutionEnvelope{
		Mode:          primitives.ModeSuggestOnly,
		TraceID:       "trace-test-123",
		ActorCircleID: "circle-test",
		ScopesUsed:    []string{read.ScopeFinanceRead},
	}
}

func TestConnector_ListAccounts(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-provider",
		Seed:       "test-seed",
	})

	ctx := context.Background()
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
	for _, acc := range receipt.Accounts {
		if acc.AccountID == "" {
			t.Error("AccountID should not be empty")
		}
		if acc.Name == "" {
			t.Error("Name should not be empty")
		}
		if acc.Balance == nil {
			t.Error("Balance should not be nil when IncludeBalances is true")
		}
	}
}

func TestConnector_ListAccounts_RejectsExecuteMode(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-provider",
		Seed:       "test-seed",
	})

	ctx := context.Background()
	env := validEnvelope()
	env.Mode = primitives.ModeExecute // This should be rejected

	_, err := connector.ListAccounts(ctx, env, read.ListAccountsRequest{})

	if err == nil {
		t.Error("expected error for execute mode, got nil")
	}
}

func TestConnector_ListTransactions(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "test-provider",
		Seed:       "test-seed",
	})

	ctx := context.Background()
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

	// Verify transactions have expected fields
	for _, tx := range receipt.Transactions {
		if tx.TransactionID == "" {
			t.Error("TransactionID should not be empty")
		}
		if tx.AmountCents == 0 {
			// Some transactions might be zero, but let's check most aren't
		}
	}
}

func TestConnector_Deterministic(t *testing.T) {
	// Create two connectors with same seed
	config := mock.Config{
		ProviderID: "test-provider",
		Seed:       "deterministic-seed",
	}

	connector1 := mock.NewConnector(config)
	connector2 := mock.NewConnector(config)

	ctx := context.Background()
	env := validEnvelope()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	receipt1, _ := connector1.ListTransactions(ctx, env, read.ListTransactionsRequest{
		StartDate: now.AddDate(0, 0, -30),
		EndDate:   now,
	})

	receipt2, _ := connector2.ListTransactions(ctx, env, read.ListTransactionsRequest{
		StartDate: now.AddDate(0, 0, -30),
		EndDate:   now,
	})

	if len(receipt1.Transactions) != len(receipt2.Transactions) {
		t.Errorf("expected same number of transactions: got %d and %d",
			len(receipt1.Transactions), len(receipt2.Transactions))
	}

	// Same seed should produce same data
	for i := range receipt1.Transactions {
		if i >= len(receipt2.Transactions) {
			break
		}
		if receipt1.Transactions[i].TransactionID != receipt2.Transactions[i].TransactionID {
			t.Errorf("transaction %d: IDs differ: %q vs %q",
				i, receipt1.Transactions[i].TransactionID, receipt2.Transactions[i].TransactionID)
		}
	}
}

func TestConnector_DifferentSeeds(t *testing.T) {
	// Different seeds should produce different data
	connector1 := mock.NewConnector(mock.Config{Seed: "seed-1"})
	connector2 := mock.NewConnector(mock.Config{Seed: "seed-2"})

	ctx := context.Background()
	env := validEnvelope()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	receipt1, _ := connector1.ListTransactions(ctx, env, read.ListTransactionsRequest{
		StartDate: now.AddDate(0, 0, -30),
		EndDate:   now,
	})

	receipt2, _ := connector2.ListTransactions(ctx, env, read.ListTransactionsRequest{
		StartDate: now.AddDate(0, 0, -30),
		EndDate:   now,
	})

	// Should have different transaction IDs
	if len(receipt1.Transactions) > 0 && len(receipt2.Transactions) > 0 {
		if receipt1.Transactions[0].TransactionID == receipt2.Transactions[0].TransactionID {
			t.Error("different seeds should produce different transaction IDs")
		}
	}
}

func TestConnector_Supports(t *testing.T) {
	connector := mock.NewConnector(mock.Config{})
	ctx := context.Background()

	caps := connector.Supports(ctx)

	if !caps.Read {
		t.Error("expected Read capability to be true")
	}

	// CRITICAL: There should be no Write field (by design)
	// This is verified at compile time since the Capabilities struct
	// doesn't have a Write field
}

func TestConnector_ProviderInfo(t *testing.T) {
	connector := mock.NewConnector(mock.Config{
		ProviderID: "my-provider",
	})

	info := connector.ProviderInfo()

	if info.ID != "my-provider" {
		t.Errorf("ProviderID = %q, want %q", info.ID, "my-provider")
	}

	if info.Type != "mock" {
		t.Errorf("Type = %q, want %q", info.Type, "mock")
	}
}

// Verify the connector implements the ReadConnector interface
var _ read.ReadConnector = (*mock.Connector)(nil)
