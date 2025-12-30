package visibility

import (
	"testing"

	"quantumlife/pkg/primitives/finance"
)

func TestComputeSpendSummary_SingleCurrency(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{AmountCents: -5000, Currency: "USD"},
		{AmountCents: -3000, Currency: "USD"},
		{AmountCents: -2000, Currency: "USD"},
	}

	summary := ComputeSpendSummary(transactions)

	if !summary.IsSingleCurrency {
		t.Error("expected single currency")
	}

	if summary.CurrencyCount != 1 {
		t.Errorf("expected 1 currency, got %d", summary.CurrencyCount)
	}

	if summary.PrimaryCurrency != "USD" {
		t.Errorf("expected USD, got %s", summary.PrimaryCurrency)
	}

	if summary.TotalByCurrency["USD"] != -10000 {
		t.Errorf("expected -10000, got %d", summary.TotalByCurrency["USD"])
	}

	if summary.InconclusiveReason != "" {
		t.Errorf("expected no inconclusive reason, got %s", summary.InconclusiveReason)
	}
}

func TestComputeSpendSummary_MultiCurrency(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{AmountCents: -5000, Currency: "USD"},
		{AmountCents: -3000, Currency: "EUR"},
		{AmountCents: -2000, Currency: "GBP"},
	}

	summary := ComputeSpendSummary(transactions)

	if summary.IsSingleCurrency {
		t.Error("expected multi-currency")
	}

	if summary.CurrencyCount != 3 {
		t.Errorf("expected 3 currencies, got %d", summary.CurrencyCount)
	}

	// Each currency has its own total
	if summary.TotalByCurrency["USD"] != -5000 {
		t.Errorf("USD: expected -5000, got %d", summary.TotalByCurrency["USD"])
	}

	if summary.TotalByCurrency["EUR"] != -3000 {
		t.Errorf("EUR: expected -3000, got %d", summary.TotalByCurrency["EUR"])
	}

	if summary.TotalByCurrency["GBP"] != -2000 {
		t.Errorf("GBP: expected -2000, got %d", summary.TotalByCurrency["GBP"])
	}

	if summary.InconclusiveReason == "" {
		t.Error("expected inconclusive reason for multi-currency")
	}
}

func TestComputeSpendSummary_EffectiveAmount(t *testing.T) {
	transactions := []finance.TransactionRecord{
		// Regular purchase
		{AmountCents: -10000, Currency: "USD"},
		// Refund (positive effective reduces spend)
		{
			AmountCents:          5000, // Positive (credit)
			EffectiveAmountCents: 5000, // Positive (reduces spend)
			Currency:             "USD",
			Kind:                 finance.KindRefund,
		},
	}

	summary := ComputeSpendSummary(transactions)

	// Only the expense (-10000) should be counted
	// The refund is positive and shouldn't be in spend totals
	if summary.TotalByCurrency["USD"] != -10000 {
		t.Errorf("expected -10000 (expense only), got %d", summary.TotalByCurrency["USD"])
	}
}

func TestValidateSingleCurrency_Pass(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{Currency: "USD"},
		{Currency: "USD"},
	}

	err := ValidateSingleCurrency(transactions)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSingleCurrency_Fail(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{Currency: "USD"},
		{Currency: "EUR"},
	}

	err := ValidateSingleCurrency(transactions)
	if err == nil {
		t.Error("expected error for multi-currency")
	}

	mcErr, ok := err.(*MultiCurrencyError)
	if !ok {
		t.Error("expected MultiCurrencyError")
	}

	if len(mcErr.Currencies) != 2 {
		t.Errorf("expected 2 currencies, got %d", len(mcErr.Currencies))
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		amount   int64
		currency string
		expected string
	}{
		{-5000, "USD", "-USD 50.00"},
		{10000, "EUR", "EUR 100.00"},
		{-123456, "GBP", "-GBP 1,234.56"},
		{0, "USD", "USD 0.00"},
		{-99, "USD", "-USD 0.99"},
	}

	for _, tt := range tests {
		result := FormatAmount(tt.amount, tt.currency)
		if result != tt.expected {
			t.Errorf("FormatAmount(%d, %s) = %s, want %s", tt.amount, tt.currency, result, tt.expected)
		}
	}
}

func TestComputeCategorySpend_MultiCurrency(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{AmountCents: -5000, Currency: "USD", Category: "food"},
		{AmountCents: -3000, Currency: "EUR", Category: "food"},
		{AmountCents: -2000, Currency: "USD", Category: "transport"},
	}

	result := ComputeCategorySpend(transactions)

	// Food has both USD and EUR
	if result.ByCategory["food"]["USD"] != -5000 {
		t.Errorf("food USD: expected -5000, got %d", result.ByCategory["food"]["USD"])
	}
	if result.ByCategory["food"]["EUR"] != -3000 {
		t.Errorf("food EUR: expected -3000, got %d", result.ByCategory["food"]["EUR"])
	}

	// Transport has only USD
	if result.ByCategory["transport"]["USD"] != -2000 {
		t.Errorf("transport USD: expected -2000, got %d", result.ByCategory["transport"]["USD"])
	}
}
