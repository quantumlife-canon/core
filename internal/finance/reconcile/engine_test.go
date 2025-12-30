package reconcile

import (
	"testing"
	"time"

	"quantumlife/internal/finance/normalize"
	"quantumlife/pkg/primitives/finance"
)

// TestReconcileTransactions_Deduplication verifies that exact duplicates
// (same canonical ID) are removed.
func TestReconcileTransactions_Deduplication(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	// Two transactions with same canonical ID (duplicates)
	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Amazon Purchase",
				AmountCents:    -5000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_duplicate_123",
			MatchKey:    "tmk_match_abc",
			IsPending:   false,
		},
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Amazon Purchase (sync 2)",
				AmountCents:    -5000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_duplicate_123", // Same canonical ID = duplicate
			MatchKey:    "tmk_match_abc",
			IsPending:   false,
		},
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Starbucks Coffee",
				AmountCents:    -500,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_unique_456",
			MatchKey:    "tmk_match_def",
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Should have 2 unique transactions (1 duplicate removed)
	if result.Report.OutputCount != 2 {
		t.Errorf("expected 2 transactions after dedup, got %d", result.Report.OutputCount)
	}

	if result.Report.DuplicatesRemoved != 1 {
		t.Errorf("expected 1 duplicate removed, got %d", result.Report.DuplicatesRemoved)
	}
}

// TestReconcileTransactions_PendingToPostedMerge verifies that pending
// transactions are merged with their posted counterparts.
func TestReconcileTransactions_PendingToPostedMerge(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	// A pending transaction and its posted counterpart
	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Amazon Purchase (pending)",
				AmountCents:    -5000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Pending:        true,
			},
			CanonicalID: "ctx_pending_123",
			MatchKey:    "tmk_match_amazon", // Same match key
			IsPending:   true,
		},
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Amazon Purchase (posted)",
				AmountCents:    -5000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC), // Different date
				Pending:        false,
			},
			CanonicalID: "ctx_posted_456",   // Different canonical ID
			MatchKey:    "tmk_match_amazon", // Same match key = same economic event
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Should have 1 transaction (pending merged into posted)
	if result.Report.OutputCount != 1 {
		t.Errorf("expected 1 transaction after pending→posted merge, got %d", result.Report.OutputCount)
	}

	if result.Report.PendingMerged != 1 {
		t.Errorf("expected 1 pending merged, got %d", result.Report.PendingMerged)
	}

	if result.Report.PostedCount != 1 {
		t.Errorf("expected 1 posted transaction, got %d", result.Report.PostedCount)
	}

	if result.Report.PendingCount != 0 {
		t.Errorf("expected 0 pending transactions, got %d", result.Report.PendingCount)
	}

	// Verify the reconciled transaction has MergedFrom set
	if len(result.Transactions) != 1 {
		t.Fatalf("expected 1 reconciled transaction, got %d", len(result.Transactions))
	}

	if len(result.Transactions[0].MergedFrom) == 0 {
		t.Error("expected MergedFrom to contain the pending canonical ID")
	}
}

// TestReconcileTransactions_UnmatchedPendingRemains verifies that pending
// transactions without a matching posted transaction remain pending.
func TestReconcileTransactions_UnmatchedPendingRemains(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Pending Purchase",
				AmountCents:    -3000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
				Pending:        true,
			},
			CanonicalID: "ctx_pending_only",
			MatchKey:    "tmk_no_match",
			IsPending:   true,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	if result.Report.OutputCount != 1 {
		t.Errorf("expected 1 transaction, got %d", result.Report.OutputCount)
	}

	if result.Report.PendingCount != 1 {
		t.Errorf("expected 1 pending transaction, got %d", result.Report.PendingCount)
	}

	if result.Report.PendingMerged != 0 {
		t.Errorf("expected 0 pending merged, got %d", result.Report.PendingMerged)
	}
}

// TestReconcileAccounts_Deduplication verifies account deduplication.
func TestReconcileAccounts_Deduplication(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	accounts := []normalize.NormalizedAccountResult{
		{
			Account: finance.NormalizedAccount{
				ProviderAccountID: "plaid_acc_123",
				DisplayName:       "Checking Account",
				AccountType:       finance.AccountTypeDepository,
				Currency:          "USD",
				BalanceCents:      100000,
			},
			CanonicalID:       "cac_checking_123",
			ProviderAccountID: "plaid_acc_123",
		},
		{
			Account: finance.NormalizedAccount{
				ProviderAccountID: "plaid_acc_123",
				DisplayName:       "Checking Account (refresh)",
				AccountType:       finance.AccountTypeDepository,
				Currency:          "USD",
				BalanceCents:      100500, // Slightly different balance
			},
			CanonicalID:       "cac_checking_123", // Same canonical ID
			ProviderAccountID: "plaid_acc_123",
		},
	}

	result, err := engine.ReconcileAccounts(ctx, accounts)
	if err != nil {
		t.Fatalf("ReconcileAccounts failed: %v", err)
	}

	if result.Report.OutputCount != 1 {
		t.Errorf("expected 1 account after dedup, got %d", result.Report.OutputCount)
	}

	if result.Report.DuplicatesRemoved != 1 {
		t.Errorf("expected 1 duplicate removed, got %d", result.Report.DuplicatesRemoved)
	}
}

// TestReconcileReport_ContainsCountsOnly verifies that the report contains
// only counts, not raw amounts (privacy protection).
func TestReconcileReport_ContainsCountsOnly(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Transaction 1",
				AmountCents:    -999999, // Large amount
				Currency:       "USD",
				Category:       "shopping",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_1",
			MatchKey:    "tmk_1",
			IsPending:   false,
		},
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Transaction 2",
				AmountCents:    50000, // Income
				Currency:       "USD",
				Category:       "income",
				Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_2",
			MatchKey:    "tmk_2",
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Verify report contains counts
	if result.Report.InputCount != 2 {
		t.Errorf("expected InputCount=2, got %d", result.Report.InputCount)
	}

	if result.Report.OutputCount != 2 {
		t.Errorf("expected OutputCount=2, got %d", result.Report.OutputCount)
	}

	if result.Report.DebitCount != 1 {
		t.Errorf("expected DebitCount=1, got %d", result.Report.DebitCount)
	}

	if result.Report.CreditCount != 1 {
		t.Errorf("expected CreditCount=1, got %d", result.Report.CreditCount)
	}

	// Verify category counts
	if result.Report.ByCategory["shopping"] != 1 {
		t.Errorf("expected shopping count=1, got %d", result.Report.ByCategory["shopping"])
	}

	if result.Report.ByCategory["income"] != 1 {
		t.Errorf("expected income count=1, got %d", result.Report.ByCategory["income"])
	}
}

// TestMergeMultiProvider verifies multi-provider reconciliation.
func TestMergeMultiProvider(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	providerData := []ProviderData{
		{
			Provider: "plaid",
			Accounts: []normalize.NormalizedAccountResult{
				{
					Account: finance.NormalizedAccount{
						ProviderAccountID: "plaid_acc",
						DisplayName:       "Checking (Plaid)",
						AccountType:       finance.AccountTypeDepository,
						Currency:          "USD",
					},
					CanonicalID: "cac_checking",
				},
			},
			Transactions: []normalize.NormalizedTransactionResult{
				{
					Transaction: finance.TransactionRecord{
						SourceProvider: "plaid",
						Description:    "Amazon (Plaid)",
						AmountCents:    -5000,
						Currency:       "USD",
						Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					},
					CanonicalID: "ctx_plaid_1",
					MatchKey:    "tmk_amazon",
					IsPending:   false,
				},
			},
		},
		{
			Provider: "truelayer",
			Accounts: []normalize.NormalizedAccountResult{
				{
					Account: finance.NormalizedAccount{
						ProviderAccountID: "tl_acc",
						DisplayName:       "Checking (TrueLayer)",
						AccountType:       finance.AccountTypeDepository,
						Currency:          "USD",
					},
					CanonicalID: "cac_checking", // Same canonical ID = same account
				},
			},
			Transactions: []normalize.NormalizedTransactionResult{
				{
					Transaction: finance.TransactionRecord{
						SourceProvider: "truelayer",
						Description:    "Starbucks (TrueLayer)",
						AmountCents:    -500,
						Currency:       "USD",
						Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
					},
					CanonicalID: "ctx_tl_1",
					MatchKey:    "tmk_starbucks",
					IsPending:   false,
				},
			},
		},
	}

	result, err := engine.MergeMultiProvider(ctx, providerData)
	if err != nil {
		t.Fatalf("MergeMultiProvider failed: %v", err)
	}

	// Should have 1 account (deduplicated) and 2 transactions (unique)
	if len(result.Accounts) != 1 {
		t.Errorf("expected 1 account after multi-provider merge, got %d", len(result.Accounts))
	}

	if len(result.Transactions) != 2 {
		t.Errorf("expected 2 transactions after multi-provider merge, got %d", len(result.Transactions))
	}

	if result.Report.OverlapCount != 1 {
		t.Errorf("expected 1 overlapping entity, got %d", result.Report.OverlapCount)
	}

	// Verify providers processed
	if len(result.Report.ProvidersProcessed) != 2 {
		t.Errorf("expected 2 providers processed, got %d", len(result.Report.ProvidersProcessed))
	}
}

// TestReconcileTransactions_DeterministicOutput verifies that reconciliation
// produces deterministic output (sorted by date then canonical ID).
func TestReconcileTransactions_DeterministicOutput(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_123",
		TraceID:   "test_trace",
	}

	// Input in non-sorted order
	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Third",
				AmountCents:    -3000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_c",
			MatchKey:    "tmk_c",
		},
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "First",
				AmountCents:    -1000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_a",
			MatchKey:    "tmk_a",
		},
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "Second",
				AmountCents:    -2000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			CanonicalID: "ctx_b",
			MatchKey:    "tmk_b",
		},
	}

	result1, _ := engine.ReconcileTransactions(ctx, transactions)
	result2, _ := engine.ReconcileTransactions(ctx, transactions)

	// Output should be identical (deterministic)
	if len(result1.Transactions) != len(result2.Transactions) {
		t.Fatal("non-deterministic output count")
	}

	for i := range result1.Transactions {
		if result1.Transactions[i].CanonicalID != result2.Transactions[i].CanonicalID {
			t.Errorf("non-deterministic order at position %d", i)
		}
	}

	// Should be sorted by date (newest first)
	if result1.Transactions[0].Transaction.Description != "First" {
		t.Errorf("expected newest transaction first, got %s", result1.Transactions[0].Transaction.Description)
	}
}
