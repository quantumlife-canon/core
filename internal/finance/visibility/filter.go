// Package visibility provides intersection-governed visibility filtering.
//
// CRITICAL: This filter controls what financial data is visible in intersections.
// It enforces the FinancialVisibilityPolicy from the intersection contract.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md §D
package visibility

import (
	"time"

	"quantumlife/pkg/primitives/finance"
)

// Filter applies visibility rules to financial data.
type Filter struct {
	policy Policy
}

// Policy defines visibility rules.
// This mirrors FinancialVisibilityPolicy to avoid import cycles.
type Policy struct {
	Enabled           bool
	AllowedAccounts   []string
	AllowedCategories []string
	WindowDays        int
	AggregationLevel  string
	AnonymizeAmounts  bool
}

// AggregationLevel constants.
const (
	LevelExact    = "exact"
	LevelCategory = "category"
	LevelTotal    = "total"
)

// NewFilter creates a new visibility filter.
func NewFilter(policy Policy) *Filter {
	// Apply defaults
	if policy.WindowDays == 0 {
		policy.WindowDays = 90
	}
	if policy.AggregationLevel == "" {
		policy.AggregationLevel = LevelCategory
	}

	return &Filter{policy: policy}
}

// FilterSnapshot filters an account snapshot based on policy.
func (f *Filter) FilterSnapshot(snapshot *finance.AccountSnapshot) *finance.AccountSnapshot {
	if !f.policy.Enabled {
		return nil // Not enabled, return nothing
	}

	filtered := &finance.AccountSnapshot{
		SnapshotID:        snapshot.SnapshotID,
		OwnerType:         snapshot.OwnerType,
		OwnerID:           snapshot.OwnerID,
		SourceProvider:    snapshot.SourceProvider,
		Currency:          snapshot.Currency,
		FetchedAt:         snapshot.FetchedAt,
		CreatedAt:         snapshot.CreatedAt,
		SchemaVersion:     snapshot.SchemaVersion,
		NormalizerVersion: snapshot.NormalizerVersion,
		TraceID:           snapshot.TraceID,
		Freshness:         snapshot.Freshness,
		PartialReason:     snapshot.PartialReason,
	}

	// Filter accounts
	for _, account := range snapshot.Accounts {
		if f.isAccountAllowed(account.AccountID) {
			filteredAccount := f.filterAccount(account)
			filtered.Accounts = append(filtered.Accounts, filteredAccount)
			if filteredAccount.BalanceCents != 0 {
				filtered.TotalBalanceCents += filteredAccount.BalanceCents
			}
		}
	}

	return filtered
}

// FilterTransactions filters transactions based on policy.
func (f *Filter) FilterTransactions(transactions []finance.TransactionRecord, now time.Time) []finance.TransactionRecord {
	if !f.policy.Enabled {
		return nil // Not enabled, return nothing
	}

	cutoff := now.AddDate(0, 0, -f.policy.WindowDays)
	var filtered []finance.TransactionRecord

	for _, tx := range transactions {
		// Check time window
		if tx.Date.Before(cutoff) {
			continue
		}

		// Check account filter
		if !f.isAccountAllowed(tx.AccountID) {
			continue
		}

		// Check category filter
		if !f.isCategoryAllowed(tx.Category) {
			continue
		}

		filteredTx := f.filterTransaction(tx)
		filtered = append(filtered, filteredTx)
	}

	return filtered
}

// FilterObservations filters observations based on policy.
func (f *Filter) FilterObservations(observations []finance.FinancialObservation, now time.Time) []finance.FinancialObservation {
	if !f.policy.Enabled {
		return nil
	}

	cutoff := now.AddDate(0, 0, -f.policy.WindowDays)
	var filtered []finance.FinancialObservation

	for _, obs := range observations {
		// Check time window
		if obs.WindowEnd.Before(cutoff) {
			continue
		}

		// Check category filter (if applicable)
		if obs.Category != "" && !f.isCategoryAllowed(obs.Category) {
			continue
		}

		filteredObs := f.filterObservation(obs)
		filtered = append(filtered, filteredObs)
	}

	return filtered
}

// FilterProposals filters proposals based on policy.
func (f *Filter) FilterProposals(proposals []finance.FinancialProposal) []finance.FinancialProposal {
	if !f.policy.Enabled {
		return nil
	}

	// Proposals don't have direct filtering - they reference observations
	// Just pass through for now
	return proposals
}

// isAccountAllowed checks if an account is in the allowed list.
func (f *Filter) isAccountAllowed(accountID string) bool {
	// If no filter specified, allow all
	if len(f.policy.AllowedAccounts) == 0 {
		return true
	}

	for _, allowed := range f.policy.AllowedAccounts {
		if allowed == accountID {
			return true
		}
	}
	return false
}

// isCategoryAllowed checks if a category is in the allowed list.
func (f *Filter) isCategoryAllowed(category string) bool {
	// If no filter specified, allow all
	if len(f.policy.AllowedCategories) == 0 {
		return true
	}

	for _, allowed := range f.policy.AllowedCategories {
		if allowed == category {
			return true
		}
	}
	return false
}

// filterAccount applies filtering rules to a single account.
func (f *Filter) filterAccount(account finance.NormalizedAccount) finance.NormalizedAccount {
	filtered := account

	if f.policy.AnonymizeAmounts {
		// Anonymize to nearest range
		filtered.BalanceCents = anonymizeAmount(account.BalanceCents)
		filtered.AvailableCents = anonymizeAmount(account.AvailableCents)
	}

	return filtered
}

// filterTransaction applies filtering rules to a single transaction.
func (f *Filter) filterTransaction(tx finance.TransactionRecord) finance.TransactionRecord {
	filtered := tx

	if f.policy.AnonymizeAmounts {
		filtered.AmountCents = anonymizeAmount(tx.AmountCents)
	}

	return filtered
}

// filterObservation applies filtering rules to a single observation.
func (f *Filter) filterObservation(obs finance.FinancialObservation) finance.FinancialObservation {
	filtered := obs

	if f.policy.AnonymizeAmounts {
		if filtered.NumericValue != nil {
			anon := anonymizeAmount(*filtered.NumericValue)
			filtered.NumericValue = &anon
		}
		if filtered.ComparisonValue != nil {
			anon := anonymizeAmount(*filtered.ComparisonValue)
			filtered.ComparisonValue = &anon
		}
		if filtered.ChangeCents != nil {
			anon := anonymizeAmount(*filtered.ChangeCents)
			filtered.ChangeCents = &anon
		}
	}

	return filtered
}

// anonymizeAmount rounds an amount to the nearest range.
func anonymizeAmount(cents int64) int64 {
	// Preserve sign
	sign := int64(1)
	if cents < 0 {
		sign = -1
		cents = -cents
	}

	// Round to ranges:
	// 0-1000 -> nearest 100
	// 1000-10000 -> nearest 500
	// 10000-100000 -> nearest 1000
	// 100000+ -> nearest 10000
	var rounded int64
	switch {
	case cents < 1000:
		rounded = (cents / 100) * 100
	case cents < 10000:
		rounded = (cents / 500) * 500
	case cents < 100000:
		rounded = (cents / 1000) * 1000
	default:
		rounded = (cents / 10000) * 10000
	}

	return rounded * sign
}

// AggregateByCategoryLevel aggregates transactions to category level.
func AggregateByCategoryLevel(transactions []finance.TransactionRecord) []finance.CategorySummary {
	categoryTotals := make(map[string]*finance.CategorySummary)

	for _, tx := range transactions {
		cat := tx.Category
		if cat == "" {
			cat = "Uncategorized"
		}

		summary, exists := categoryTotals[cat]
		if !exists {
			summary = &finance.CategorySummary{
				Category:   cat,
				CategoryID: tx.CategoryID,
				Currency:   tx.Currency,
			}
			categoryTotals[cat] = summary
		}

		// Add to totals (use absolute value for spending)
		amount := tx.AmountCents
		if amount < 0 {
			amount = -amount
		}
		summary.TotalSpentCents += amount
		summary.TransactionCount++

		if amount > summary.LargestTransactionCents {
			summary.LargestTransactionCents = amount
		}

		// Update window
		if summary.WindowStart.IsZero() || tx.Date.Before(summary.WindowStart) {
			summary.WindowStart = tx.Date
		}
		if summary.WindowEnd.IsZero() || tx.Date.After(summary.WindowEnd) {
			summary.WindowEnd = tx.Date
		}
	}

	// Calculate averages and convert to slice
	var result []finance.CategorySummary
	for _, summary := range categoryTotals {
		if summary.TransactionCount > 0 {
			summary.AverageTransactionCents = summary.TotalSpentCents / int64(summary.TransactionCount)
		}
		result = append(result, *summary)
	}

	return result
}

// AggregateTotalLevel aggregates transactions to total level only.
func AggregateTotalLevel(transactions []finance.TransactionRecord) finance.CashflowWindow {
	window := finance.CashflowWindow{
		Currency: "USD",
	}

	for _, tx := range transactions {
		window.TransactionCount++

		if tx.AmountCents > 0 {
			window.TotalIncomeCents += tx.AmountCents
			window.IncomeCount++
		} else {
			window.TotalExpensesCents += -tx.AmountCents
			window.ExpenseCount++
		}

		// Update window
		if window.WindowStart.IsZero() || tx.Date.Before(window.WindowStart) {
			window.WindowStart = tx.Date
		}
		if window.WindowEnd.IsZero() || tx.Date.After(window.WindowEnd) {
			window.WindowEnd = tx.Date
		}
	}

	window.NetCashflowCents = window.TotalIncomeCents - window.TotalExpensesCents

	return window
}
