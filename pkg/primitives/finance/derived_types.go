// Package finance provides derived types for financial analysis.
//
// CRITICAL: These are derived observations, not action primitives.
// All types support READ and PROPOSE only.
package finance

import (
	"time"
)

// RecurringPattern represents a detected recurring transaction pattern.
type RecurringPattern struct {
	// PatternID uniquely identifies this pattern.
	PatternID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// MerchantName is the merchant associated with this pattern.
	MerchantName string

	// Category is the transaction category.
	Category string

	// EstimatedAmountCents is the typical transaction amount.
	EstimatedAmountCents int64

	// Currency is the currency code.
	Currency string

	// Frequency describes how often this occurs.
	Frequency RecurringFrequency

	// LastOccurrence is the most recent transaction date.
	LastOccurrence time.Time

	// OccurrenceCount is how many times this pattern was observed.
	OccurrenceCount int

	// MatchingTransactionIDs lists the transactions that form this pattern.
	MatchingTransactionIDs []string

	// Certain indicates if this pattern is definitively recurring.
	Certain bool

	// DetectionReason explains how this pattern was detected.
	DetectionReason string

	// CreatedAt is when this pattern was detected.
	CreatedAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// TraceID links to the operation that created this pattern.
	TraceID string
}

// RecurringFrequency describes transaction recurrence.
type RecurringFrequency string

const (
	FrequencyWeekly    RecurringFrequency = "weekly"
	FrequencyBiWeekly  RecurringFrequency = "biweekly"
	FrequencyMonthly   RecurringFrequency = "monthly"
	FrequencyQuarterly RecurringFrequency = "quarterly"
	FrequencyAnnual    RecurringFrequency = "annual"
	FrequencyUnknown   RecurringFrequency = "unknown"
)

// CategorySummary summarizes spending in a category.
type CategorySummary struct {
	// SummaryID uniquely identifies this summary.
	SummaryID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// Category is the category being summarized.
	Category string

	// CategoryID is the category identifier.
	CategoryID string

	// WindowStart is the start of the analysis window.
	WindowStart time.Time

	// WindowEnd is the end of the analysis window.
	WindowEnd time.Time

	// TotalSpentCents is the total amount spent (absolute value).
	TotalSpentCents int64

	// TransactionCount is the number of transactions.
	TransactionCount int

	// AverageTransactionCents is the average transaction size.
	AverageTransactionCents int64

	// LargestTransactionCents is the largest single transaction.
	LargestTransactionCents int64

	// Currency is the currency code.
	Currency string

	// PercentOfTotal is the percentage of total spending.
	PercentOfTotal float64

	// CreatedAt is when this summary was created.
	CreatedAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// TraceID links to the operation that created this summary.
	TraceID string
}

// CashflowWindow summarizes cash flow over a time period.
type CashflowWindow struct {
	// WindowID uniquely identifies this window.
	WindowID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// WindowStart is the start of the window.
	WindowStart time.Time

	// WindowEnd is the end of the window.
	WindowEnd time.Time

	// TotalIncomeCents is total income (positive transactions).
	TotalIncomeCents int64

	// TotalExpensesCents is total expenses (absolute value of negative transactions).
	TotalExpensesCents int64

	// NetCashflowCents is income minus expenses.
	NetCashflowCents int64

	// Currency is the currency code.
	Currency string

	// TransactionCount is the total number of transactions.
	TransactionCount int

	// IncomeCount is the number of income transactions.
	IncomeCount int

	// ExpenseCount is the number of expense transactions.
	ExpenseCount int

	// CategoryBreakdown summarizes by category.
	CategoryBreakdown []CategorySummary

	// OpeningBalanceCents is the balance at window start (if known).
	OpeningBalanceCents *int64

	// ClosingBalanceCents is the balance at window end (if known).
	ClosingBalanceCents *int64

	// CreatedAt is when this window was created.
	CreatedAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// TraceID links to the operation that created this window.
	TraceID string

	// Partial indicates if data is incomplete.
	Partial bool

	// PartialReason explains partiality.
	PartialReason string
}

// IsPositiveCashflow returns true if income exceeds expenses.
func (c *CashflowWindow) IsPositiveCashflow() bool {
	return c.NetCashflowCents > 0
}

// IsNegativeCashflow returns true if expenses exceed income.
func (c *CashflowWindow) IsNegativeCashflow() bool {
	return c.NetCashflowCents < 0
}
