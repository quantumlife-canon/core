// Package visibility provides v8.5 financial visibility computations.
//
// CRITICAL INVARIANT: No amount aggregation across currencies.
// All totals are keyed by currency. No implicit conversion exists.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package visibility

import (
	"quantumlife/pkg/primitives/finance"
)

// AmountByCurrency maps currency codes to amounts in minor units.
// This prevents accidental cross-currency aggregation.
type AmountByCurrency map[string]int64

// SpendSummary contains spending totals keyed by currency.
// CRITICAL: Never contains a single "total" - always per-currency.
type SpendSummary struct {
	// TotalByCurrency contains spend totals for each currency.
	// Key: ISO 4217 currency code (e.g., "USD", "EUR", "GBP")
	// Value: Amount in minor units (cents/pence)
	TotalByCurrency AmountByCurrency

	// TransactionCountByCurrency tracks transaction counts per currency.
	TransactionCountByCurrency map[string]int

	// CurrencyCount is how many distinct currencies are present.
	CurrencyCount int

	// IsSingleCurrency is true if all transactions use the same currency.
	IsSingleCurrency bool

	// PrimaryCurrency is the most common currency (by transaction count).
	// Only set if IsSingleCurrency is true or if there's a clear majority.
	PrimaryCurrency string

	// InconclusiveReason explains why aggregation may be misleading.
	// Set when multiple currencies exist without conversion.
	InconclusiveReason string
}

// CategorySpend contains spending by category, keyed by currency.
type CategorySpend struct {
	// ByCategory maps category names to per-currency totals.
	// Key: Category name
	// Value: AmountByCurrency for that category
	ByCategory map[string]AmountByCurrency

	// TransactionsByCategory maps category names to transaction counts.
	TransactionsByCategory map[string]int
}

// MerchantSpend contains spending by merchant, keyed by currency.
type MerchantSpend struct {
	// ByMerchant maps merchant names to per-currency totals.
	ByMerchant map[string]AmountByCurrency

	// TransactionsByMerchant maps merchant names to transaction counts.
	TransactionsByMerchant map[string]int
}

// ComputeSpendSummary aggregates transactions into a spend summary.
// CRITICAL: Uses EffectiveAmountCents when available for refund handling.
func ComputeSpendSummary(transactions []finance.TransactionRecord) SpendSummary {
	summary := SpendSummary{
		TotalByCurrency:            make(AmountByCurrency),
		TransactionCountByCurrency: make(map[string]int),
	}

	currencies := make(map[string]bool)

	for _, tx := range transactions {
		currency := tx.Currency
		if currency == "" {
			currency = "UNKNOWN"
		}
		currencies[currency] = true

		// Use effective amount for spend calculations (handles refunds)
		amount := tx.GetEffectiveAmount()

		// Only count expenses (negative amounts) for spend totals
		if amount < 0 {
			summary.TotalByCurrency[currency] += amount
			summary.TransactionCountByCurrency[currency]++
		}
	}

	summary.CurrencyCount = len(currencies)
	summary.IsSingleCurrency = summary.CurrencyCount == 1

	if summary.IsSingleCurrency {
		for cur := range currencies {
			summary.PrimaryCurrency = cur
		}
	} else if summary.CurrencyCount > 1 {
		// Find majority currency
		maxCount := 0
		for cur, count := range summary.TransactionCountByCurrency {
			if count > maxCount {
				maxCount = count
				summary.PrimaryCurrency = cur
			}
		}
		summary.InconclusiveReason = "Multiple currencies present; totals shown per-currency without conversion"
	}

	return summary
}

// ComputeCategorySpend aggregates transactions by category.
func ComputeCategorySpend(transactions []finance.TransactionRecord) CategorySpend {
	result := CategorySpend{
		ByCategory:             make(map[string]AmountByCurrency),
		TransactionsByCategory: make(map[string]int),
	}

	for _, tx := range transactions {
		category := tx.Category
		if category == "" {
			category = "uncategorized"
		}

		currency := tx.Currency
		if currency == "" {
			currency = "UNKNOWN"
		}

		// Use effective amount for spend calculations
		amount := tx.GetEffectiveAmount()

		// Only count expenses
		if amount < 0 {
			if result.ByCategory[category] == nil {
				result.ByCategory[category] = make(AmountByCurrency)
			}
			result.ByCategory[category][currency] += amount
			result.TransactionsByCategory[category]++
		}
	}

	return result
}

// ComputeMerchantSpend aggregates transactions by merchant.
func ComputeMerchantSpend(transactions []finance.TransactionRecord) MerchantSpend {
	result := MerchantSpend{
		ByMerchant:             make(map[string]AmountByCurrency),
		TransactionsByMerchant: make(map[string]int),
	}

	for _, tx := range transactions {
		merchant := finance.NormalizeMerchant(tx.MerchantName)
		if merchant == "" {
			merchant = "unknown"
		}

		currency := tx.Currency
		if currency == "" {
			currency = "UNKNOWN"
		}

		// Use effective amount for spend calculations
		amount := tx.GetEffectiveAmount()

		// Only count expenses
		if amount < 0 {
			if result.ByMerchant[merchant] == nil {
				result.ByMerchant[merchant] = make(AmountByCurrency)
			}
			result.ByMerchant[merchant][currency] += amount
			result.TransactionsByMerchant[merchant]++
		}
	}

	return result
}

// FormatAmount formats an amount with currency symbol.
// CRITICAL: Always includes currency code for clarity.
func FormatAmount(amountCents int64, currency string) string {
	if currency == "" {
		currency = "???"
	}

	sign := ""
	if amountCents < 0 {
		sign = "-"
		amountCents = -amountCents
	}

	// Convert to dollars/pounds/euros
	major := amountCents / 100
	minor := amountCents % 100

	return sign + currency + " " + formatWithCommas(major) + "." + padLeft(minor, 2)
}

func formatWithCommas(n int64) string {
	if n < 1000 {
		return intToString(n)
	}
	return formatWithCommas(n/1000) + "," + padLeft(n%1000, 3)
}

func intToString(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte(n%10) + '0'}, digits...)
		n /= 10
	}
	return string(digits)
}

func padLeft(n int64, width int) string {
	s := intToString(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// ValidateSingleCurrency returns an error if transactions span multiple currencies.
// Use this when a single-currency total is required.
func ValidateSingleCurrency(transactions []finance.TransactionRecord) error {
	currencies := make(map[string]bool)
	for _, tx := range transactions {
		cur := tx.Currency
		if cur == "" {
			cur = "UNKNOWN"
		}
		currencies[cur] = true
	}

	if len(currencies) > 1 {
		return &MultiCurrencyError{Currencies: currencies}
	}
	return nil
}

// MultiCurrencyError indicates an operation that requires single currency
// was attempted on multi-currency data.
type MultiCurrencyError struct {
	Currencies map[string]bool
}

func (e *MultiCurrencyError) Error() string {
	return "operation requires single currency but multiple currencies present"
}
