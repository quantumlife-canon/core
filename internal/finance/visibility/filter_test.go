// Package visibility_test provides tests for the visibility filter.
package visibility_test

import (
	"testing"
	"time"

	"quantumlife/internal/finance/visibility"
	"quantumlife/pkg/primitives/finance"
)

func TestFilter_FilterTransactions(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	policy := visibility.Policy{
		Enabled:          true,
		WindowDays:       30,
		AggregationLevel: visibility.LevelExact,
	}

	filter := visibility.NewFilter(policy)

	transactions := []finance.TransactionRecord{
		{
			RecordID:    "tx-1",
			AccountID:   "acc-1",
			Category:    "Groceries",
			Date:        now.AddDate(0, 0, -10), // Within window
			AmountCents: -5000,
		},
		{
			RecordID:    "tx-2",
			AccountID:   "acc-1",
			Category:    "Gas",
			Date:        now.AddDate(0, 0, -60), // Outside window
			AmountCents: -3000,
		},
		{
			RecordID:    "tx-3",
			AccountID:   "acc-2",
			Category:    "Shopping",
			Date:        now.AddDate(0, 0, -5), // Within window
			AmountCents: -10000,
		},
	}

	filtered := filter.FilterTransactions(transactions, now)

	// Should have 2 transactions (within 30 day window)
	if len(filtered) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(filtered))
	}

	// Verify tx-2 (outside window) is excluded
	for _, tx := range filtered {
		if tx.RecordID == "tx-2" {
			t.Error("tx-2 should be filtered out (outside window)")
		}
	}
}

func TestFilter_FilterTransactions_AccountFilter(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	policy := visibility.Policy{
		Enabled:         true,
		AllowedAccounts: []string{"acc-1"}, // Only allow acc-1
		WindowDays:      90,
	}

	filter := visibility.NewFilter(policy)

	transactions := []finance.TransactionRecord{
		{RecordID: "tx-1", AccountID: "acc-1", Date: now.AddDate(0, 0, -10)},
		{RecordID: "tx-2", AccountID: "acc-2", Date: now.AddDate(0, 0, -10)},
		{RecordID: "tx-3", AccountID: "acc-1", Date: now.AddDate(0, 0, -5)},
	}

	filtered := filter.FilterTransactions(transactions, now)

	// Should only have acc-1 transactions
	if len(filtered) != 2 {
		t.Errorf("expected 2 transactions (acc-1 only), got %d", len(filtered))
	}

	for _, tx := range filtered {
		if tx.AccountID != "acc-1" {
			t.Errorf("unexpected account %q, want %q", tx.AccountID, "acc-1")
		}
	}
}

func TestFilter_FilterTransactions_CategoryFilter(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	policy := visibility.Policy{
		Enabled:           true,
		AllowedCategories: []string{"Groceries", "Gas"},
		WindowDays:        90,
	}

	filter := visibility.NewFilter(policy)

	transactions := []finance.TransactionRecord{
		{RecordID: "tx-1", Category: "Groceries", Date: now.AddDate(0, 0, -10)},
		{RecordID: "tx-2", Category: "Shopping", Date: now.AddDate(0, 0, -10)},
		{RecordID: "tx-3", Category: "Gas", Date: now.AddDate(0, 0, -5)},
	}

	filtered := filter.FilterTransactions(transactions, now)

	// Should only have Groceries and Gas
	if len(filtered) != 2 {
		t.Errorf("expected 2 transactions (allowed categories), got %d", len(filtered))
	}

	for _, tx := range filtered {
		if tx.Category != "Groceries" && tx.Category != "Gas" {
			t.Errorf("unexpected category %q", tx.Category)
		}
	}
}

func TestFilter_FilterTransactions_Disabled(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	policy := visibility.Policy{
		Enabled: false, // Disabled
	}

	filter := visibility.NewFilter(policy)

	transactions := []finance.TransactionRecord{
		{RecordID: "tx-1", Date: now.AddDate(0, 0, -10)},
		{RecordID: "tx-2", Date: now.AddDate(0, 0, -5)},
	}

	filtered := filter.FilterTransactions(transactions, now)

	// Disabled policy returns nothing
	if filtered != nil {
		t.Errorf("expected nil when disabled, got %d transactions", len(filtered))
	}
}

func TestFilter_FilterTransactions_AnonymizeAmounts(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	policy := visibility.Policy{
		Enabled:          true,
		WindowDays:       90,
		AnonymizeAmounts: true,
	}

	filter := visibility.NewFilter(policy)

	transactions := []finance.TransactionRecord{
		{RecordID: "tx-1", Date: now.AddDate(0, 0, -10), AmountCents: -5432},
		{RecordID: "tx-2", Date: now.AddDate(0, 0, -5), AmountCents: -12789},
	}

	filtered := filter.FilterTransactions(transactions, now)

	// Amounts should be anonymized (rounded)
	for _, tx := range filtered {
		// Check that amounts are rounded to appropriate ranges
		abs := tx.AmountCents
		if abs < 0 {
			abs = -abs
		}

		// Amounts should be rounded (not exact)
		// 5432 -> 5000 or 5500 (rounded to 500)
		// 12789 -> 12000 or 13000 (rounded to 1000)
		if abs == 5432 || abs == 12789 {
			t.Errorf("expected anonymized amount, got exact value %d", tx.AmountCents)
		}
	}
}

func TestFilter_FilterSnapshot(t *testing.T) {
	policy := visibility.Policy{
		Enabled:    true,
		WindowDays: 90,
	}

	filter := visibility.NewFilter(policy)

	snapshot := &finance.AccountSnapshot{
		SnapshotID:        "snap-1",
		OwnerType:         "circle",
		OwnerID:           "circle-1",
		SourceProvider:    "mock",
		Currency:          "USD",
		TotalBalanceCents: 100000,
		Accounts: []finance.NormalizedAccount{
			{AccountID: "acc-1", BalanceCents: 50000},
			{AccountID: "acc-2", BalanceCents: 50000},
		},
	}

	filtered := filter.FilterSnapshot(snapshot)

	if filtered == nil {
		t.Fatal("expected filtered snapshot, got nil")
	}

	if filtered.OwnerID != snapshot.OwnerID {
		t.Errorf("OwnerID = %q, want %q", filtered.OwnerID, snapshot.OwnerID)
	}
}

func TestFilter_FilterSnapshot_WithAccountFilter(t *testing.T) {
	policy := visibility.Policy{
		Enabled:         true,
		AllowedAccounts: []string{"acc-1"}, // Only allow acc-1
	}

	filter := visibility.NewFilter(policy)

	snapshot := &finance.AccountSnapshot{
		SnapshotID: "snap-1",
		Accounts: []finance.NormalizedAccount{
			{AccountID: "acc-1", BalanceCents: 50000},
			{AccountID: "acc-2", BalanceCents: 30000},
			{AccountID: "acc-3", BalanceCents: 20000},
		},
	}

	filtered := filter.FilterSnapshot(snapshot)

	// Should only have acc-1
	if len(filtered.Accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(filtered.Accounts))
	}

	if filtered.Accounts[0].AccountID != "acc-1" {
		t.Errorf("expected acc-1, got %q", filtered.Accounts[0].AccountID)
	}

	// Total balance should be recalculated
	if filtered.TotalBalanceCents != 50000 {
		t.Errorf("TotalBalanceCents = %d, want %d", filtered.TotalBalanceCents, 50000)
	}
}

func TestAggregateByCategoryLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	transactions := []finance.TransactionRecord{
		{Category: "Groceries", AmountCents: -5000, Currency: "USD", Date: now.AddDate(0, 0, -10)},
		{Category: "Groceries", AmountCents: -3000, Currency: "USD", Date: now.AddDate(0, 0, -5)},
		{Category: "Gas", AmountCents: -4000, Currency: "USD", Date: now.AddDate(0, 0, -7)},
		{Category: "Gas", AmountCents: -2000, Currency: "USD", Date: now.AddDate(0, 0, -3)},
		{Category: "Entertainment", AmountCents: -1500, Currency: "USD", Date: now.AddDate(0, 0, -2)},
	}

	summaries := visibility.AggregateByCategoryLevel(transactions)

	// Should have 3 categories
	if len(summaries) != 3 {
		t.Errorf("expected 3 categories, got %d", len(summaries))
	}

	// Find Groceries summary
	var groceriesSummary *finance.CategorySummary
	for i := range summaries {
		if summaries[i].Category == "Groceries" {
			groceriesSummary = &summaries[i]
			break
		}
	}

	if groceriesSummary == nil {
		t.Fatal("expected Groceries category in summaries")
	}

	// Groceries: 5000 + 3000 = 8000
	if groceriesSummary.TotalSpentCents != 8000 {
		t.Errorf("Groceries TotalSpentCents = %d, want %d", groceriesSummary.TotalSpentCents, 8000)
	}

	if groceriesSummary.TransactionCount != 2 {
		t.Errorf("Groceries TransactionCount = %d, want %d", groceriesSummary.TransactionCount, 2)
	}
}

func TestAggregateTotalLevel(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	transactions := []finance.TransactionRecord{
		{AmountCents: -5000, Date: now.AddDate(0, 0, -10)}, // Expense
		{AmountCents: 20000, Date: now.AddDate(0, 0, -5)},  // Income
		{AmountCents: -3000, Date: now.AddDate(0, 0, -3)},  // Expense
	}

	window := visibility.AggregateTotalLevel(transactions)

	// Total income: 20000
	if window.TotalIncomeCents != 20000 {
		t.Errorf("TotalIncomeCents = %d, want %d", window.TotalIncomeCents, 20000)
	}

	// Total expenses: 5000 + 3000 = 8000
	if window.TotalExpensesCents != 8000 {
		t.Errorf("TotalExpensesCents = %d, want %d", window.TotalExpensesCents, 8000)
	}

	// Net: 20000 - 8000 = 12000
	if window.NetCashflowCents != 12000 {
		t.Errorf("NetCashflowCents = %d, want %d", window.NetCashflowCents, 12000)
	}

	if window.TransactionCount != 3 {
		t.Errorf("TransactionCount = %d, want %d", window.TransactionCount, 3)
	}

	if window.IncomeCount != 1 {
		t.Errorf("IncomeCount = %d, want %d", window.IncomeCount, 1)
	}

	if window.ExpenseCount != 2 {
		t.Errorf("ExpenseCount = %d, want %d", window.ExpenseCount, 2)
	}
}
