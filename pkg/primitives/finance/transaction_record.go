// Package finance provides canonical transaction record types.
package finance

import (
	"time"
)

// TransactionRecord is the canonical transaction representation.
// All amounts are in cents to avoid floating-point issues.
type TransactionRecord struct {
	// RecordID uniquely identifies this record in our system.
	RecordID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID that owns this record.
	OwnerID string

	// SourceProvider identifies where this data came from.
	SourceProvider string

	// ProviderTransactionID is the original provider's transaction ID.
	ProviderTransactionID string

	// AccountID is our normalized account identifier.
	AccountID string

	// Date is the transaction date.
	Date time.Time

	// PostedDate is when the transaction posted (may differ from Date).
	PostedDate *time.Time

	// AmountCents is the transaction amount in cents.
	// Negative for debits/expenses, positive for credits/income.
	AmountCents int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// Description is the original transaction description.
	Description string

	// MerchantName is the normalized merchant name.
	MerchantName string

	// Category is our normalized category.
	Category string

	// CategoryID is our category identifier.
	CategoryID string

	// Categorization contains categorization metadata.
	Categorization CategorizationResult

	// Pending indicates if the transaction is pending.
	Pending bool

	// PaymentChannel describes how the transaction occurred.
	PaymentChannel string

	// CreatedAt is when this record was created.
	CreatedAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// NormalizerVersion is the version of normalization rules applied.
	NormalizerVersion string

	// TraceID links to the operation that created this record.
	TraceID string
}

// CategorizationResult describes how a transaction was categorized.
type CategorizationResult struct {
	// Category is the assigned category.
	Category string

	// CategoryID is the category identifier.
	CategoryID string

	// MatchedRule is the rule that produced this categorization.
	MatchedRule string

	// Certain indicates if the match was exact (true) or fallback (false).
	Certain bool

	// Reason explains why this category was assigned.
	Reason string
}

// IsExpense returns true if this is an expense (negative amount).
func (t *TransactionRecord) IsExpense() bool {
	return t.AmountCents < 0
}

// IsIncome returns true if this is income (positive amount).
func (t *TransactionRecord) IsIncome() bool {
	return t.AmountCents > 0
}

// AbsAmountCents returns the absolute value of the amount.
func (t *TransactionRecord) AbsAmountCents() int64 {
	if t.AmountCents < 0 {
		return -t.AmountCents
	}
	return t.AmountCents
}

// TransactionBatch represents a batch of transactions from a sync.
type TransactionBatch struct {
	// BatchID uniquely identifies this batch.
	BatchID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// SourceProvider identifies the data source.
	SourceProvider string

	// Transactions contains the normalized transactions.
	Transactions []TransactionRecord

	// WindowStart is the start of the time window.
	WindowStart time.Time

	// WindowEnd is the end of the time window.
	WindowEnd time.Time

	// FetchedAt is when the data was retrieved.
	FetchedAt time.Time

	// CreatedAt is when this batch was created.
	CreatedAt time.Time

	// TraceID links to the operation that created this batch.
	TraceID string

	// TransactionCount is the number of transactions.
	TransactionCount int

	// Partial indicates if the batch is incomplete.
	Partial bool

	// PartialReason explains partiality.
	PartialReason string
}
