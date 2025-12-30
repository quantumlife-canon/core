// Package read provides types for finance read connector operations.
//
// CRITICAL: All types are for READ operations only.
// No request or receipt types for write operations exist.
package read

import (
	"time"
)

// ListAccountsRequest contains parameters for listing accounts.
type ListAccountsRequest struct {
	// IncludeBalances indicates whether to include current balances.
	IncludeBalances bool
}

// AccountsReceipt contains the result of listing accounts.
type AccountsReceipt struct {
	// Accounts is the list of financial accounts.
	Accounts []Account

	// FetchedAt is when the data was retrieved.
	FetchedAt time.Time

	// ProviderID identifies the source provider.
	ProviderID string

	// Partial indicates if only some accounts were returned.
	Partial bool

	// PartialReason explains why data is partial (if applicable).
	PartialReason string
}

// Account represents a financial account.
type Account struct {
	// AccountID uniquely identifies this account at the provider.
	AccountID string

	// Name is the account name (may be masked).
	Name string

	// Type is the account type (checking, savings, credit, etc.).
	Type AccountType

	// Subtype provides more detail (e.g., "money market").
	Subtype string

	// Mask is the last 4 digits of the account number.
	Mask string

	// Balance is the current balance (if requested).
	Balance *Balance

	// InstitutionID identifies the financial institution.
	InstitutionID string

	// InstitutionName is the institution's display name.
	InstitutionName string
}

// AccountType represents the type of financial account.
type AccountType string

const (
	AccountTypeChecking   AccountType = "checking"
	AccountTypeSavings    AccountType = "savings"
	AccountTypeCredit     AccountType = "credit"
	AccountTypeLoan       AccountType = "loan"
	AccountTypeInvestment AccountType = "investment"
	AccountTypeOther      AccountType = "other"
)

// Balance represents an account balance.
type Balance struct {
	// Current is the current balance in cents.
	CurrentCents int64

	// Available is the available balance in cents (may differ from current).
	AvailableCents int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// AsOf is when this balance was last updated.
	AsOf time.Time
}

// ListTransactionsRequest contains parameters for listing transactions.
type ListTransactionsRequest struct {
	// AccountIDs filters to specific accounts (empty = all accounts).
	AccountIDs []string

	// StartDate is the beginning of the date range (inclusive).
	StartDate time.Time

	// EndDate is the end of the date range (inclusive).
	EndDate time.Time

	// Limit is the maximum number of transactions to return.
	// 0 means use provider default.
	Limit int

	// Offset is for pagination.
	Offset int
}

// TransactionsReceipt contains the result of listing transactions.
type TransactionsReceipt struct {
	// Transactions is the list of transactions.
	Transactions []Transaction

	// FetchedAt is when the data was retrieved.
	FetchedAt time.Time

	// ProviderID identifies the source provider.
	ProviderID string

	// TotalCount is the total number of transactions (for pagination).
	TotalCount int

	// HasMore indicates if more transactions are available.
	HasMore bool

	// Partial indicates if data is incomplete.
	Partial bool

	// PartialReason explains why data is partial (if applicable).
	PartialReason string
}

// Transaction represents a financial transaction.
type Transaction struct {
	// TransactionID uniquely identifies this transaction at the provider.
	TransactionID string

	// AccountID identifies the account this transaction belongs to.
	AccountID string

	// Date is the transaction date.
	Date time.Time

	// PostedDate is when the transaction posted (may differ from Date).
	PostedDate *time.Time

	// AmountCents is the transaction amount in cents.
	// Negative for debits, positive for credits.
	AmountCents int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// Name is the transaction description from the provider.
	Name string

	// MerchantName is the merchant name (if available).
	MerchantName string

	// Pending indicates if the transaction is pending.
	Pending bool

	// ProviderCategory is the category assigned by the provider.
	ProviderCategory string

	// ProviderCategoryID is the provider's category identifier.
	ProviderCategoryID string

	// PaymentChannel describes how the transaction occurred.
	PaymentChannel string
}

// DataFreshness describes how fresh the data is.
type DataFreshness string

const (
	// FreshnessCurrent indicates data is fresh (within refresh window).
	FreshnessCurrent DataFreshness = "current"

	// FreshnessStale indicates data is older than the refresh window.
	FreshnessStale DataFreshness = "stale"

	// FreshnessPartial indicates only some data was available.
	FreshnessPartial DataFreshness = "partial"

	// FreshnessUnavailable indicates the provider is unreachable.
	FreshnessUnavailable DataFreshness = "unavailable"
)
