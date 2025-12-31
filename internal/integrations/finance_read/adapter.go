// Package finance_read provides a read-only adapter for financial data integration.
//
// CRITICAL: This adapter is READ-ONLY. It NEVER initiates payments or writes.
// All data is transformed to canonical TransactionEvent and BalanceEvent formats.
//
// Reference: docs/INTEGRATIONS_MATRIX_V1.md, docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package finance_read

import (
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// Adapter defines the interface for financial data read operations.
type Adapter interface {
	// FetchTransactions retrieves transactions and returns canonical events.
	// This is a synchronous operation - no background polling.
	FetchTransactions(accountID string, since time.Time, limit int) ([]*events.TransactionEvent, error)

	// FetchBalance retrieves the current balance for an account.
	FetchBalance(accountID string) (*events.BalanceEvent, error)

	// FetchPendingCount returns count of pending transactions.
	FetchPendingCount(accountID string) (int, error)

	// Name returns the adapter name.
	Name() string
}

// MockAdapter is a mock implementation for testing and demos.
type MockAdapter struct {
	clock        clock.Clock
	transactions []*MockTransaction
	balances     map[string]*MockBalance
}

// MockTransaction represents a mock financial transaction.
type MockTransaction struct {
	AccountID         string
	TransactionID     string
	Institution       string
	MaskedNumber      string
	TransactionType   string // DEBIT, CREDIT
	TransactionKind   string // PURCHASE, REFUND, TRANSFER
	TransactionStatus string // PENDING, POSTED
	AmountMinor       int64
	Currency          string
	MerchantName      string
	MerchantNameRaw   string
	MerchantCategory  string
	TransactionDate   time.Time
	PostedDate        *time.Time
	Reference         string
	CircleID          identity.EntityID
}

// MockBalance represents a mock account balance.
type MockBalance struct {
	AccountID        string
	AccountType      string
	Institution      string
	MaskedNumber     string
	CurrentMinor     int64
	AvailableMinor   int64
	Currency         string
	CreditLimitMinor *int64
	AsOf             time.Time
	CircleID         identity.EntityID
}

// NewMockAdapter creates a new mock finance adapter.
func NewMockAdapter(clk clock.Clock) *MockAdapter {
	return &MockAdapter{
		clock:        clk,
		transactions: make([]*MockTransaction, 0),
		balances:     make(map[string]*MockBalance),
	}
}

// AddMockTransaction adds a transaction to the mock adapter.
func (a *MockAdapter) AddMockTransaction(tx *MockTransaction) {
	a.transactions = append(a.transactions, tx)
}

// SetMockBalance sets the balance for an account.
func (a *MockAdapter) SetMockBalance(balance *MockBalance) {
	a.balances[balance.AccountID] = balance
}

func (a *MockAdapter) Name() string {
	return "finance_mock"
}

func (a *MockAdapter) FetchTransactions(accountID string, since time.Time, limit int) ([]*events.TransactionEvent, error) {
	now := a.clock.Now()
	var result []*events.TransactionEvent

	for _, tx := range a.transactions {
		if tx.AccountID != accountID {
			continue
		}
		if !since.IsZero() && tx.TransactionDate.Before(since) {
			continue
		}

		event := events.NewTransactionEvent(
			"finance_mock",
			tx.AccountID,
			tx.TransactionID,
			now,
			tx.TransactionDate,
		)

		event.AccountType = "CHECKING" // Default
		event.Institution = tx.Institution
		event.MaskedNumber = tx.MaskedNumber
		event.TransactionType = tx.TransactionType
		event.TransactionKind = tx.TransactionKind
		event.TransactionStatus = tx.TransactionStatus
		event.AmountMinor = tx.AmountMinor
		event.Currency = tx.Currency
		event.MerchantName = tx.MerchantName
		event.MerchantNameRaw = tx.MerchantNameRaw
		event.MerchantCategory = tx.MerchantCategory
		event.TransactionDate = tx.TransactionDate
		event.PostedDate = tx.PostedDate
		event.Reference = tx.Reference

		// Set circle
		event.Circle = tx.CircleID

		result = append(result, event)

		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

func (a *MockAdapter) FetchBalance(accountID string) (*events.BalanceEvent, error) {
	now := a.clock.Now()

	balance, exists := a.balances[accountID]
	if !exists {
		// Return zero balance if not found
		event := events.NewBalanceEvent("finance_mock", accountID, now, now)
		event.Currency = "GBP"
		return event, nil
	}

	event := events.NewBalanceEvent(
		"finance_mock",
		balance.AccountID,
		now,
		balance.AsOf,
	)

	event.AccountType = balance.AccountType
	event.Institution = balance.Institution
	event.MaskedNumber = balance.MaskedNumber
	event.CurrentMinor = balance.CurrentMinor
	event.AvailableMinor = balance.AvailableMinor
	event.Currency = balance.Currency
	event.CreditLimitMinor = balance.CreditLimitMinor
	event.AsOf = balance.AsOf

	// Set circle
	event.Circle = balance.CircleID

	return event, nil
}

func (a *MockAdapter) FetchPendingCount(accountID string) (int, error) {
	count := 0
	for _, tx := range a.transactions {
		if tx.AccountID == accountID && tx.TransactionStatus == "PENDING" {
			count++
		}
	}
	return count, nil
}

// Verify interface compliance.
var _ Adapter = (*MockAdapter)(nil)
