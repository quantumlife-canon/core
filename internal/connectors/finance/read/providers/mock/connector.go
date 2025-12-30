// Package mock provides a deterministic mock implementation of the finance read connector.
// This is for demo and testing purposes — v8.1 is mock-only.
//
// CRITICAL: This mock generates deterministic data for testing.
// Real provider implementations come in v8.2.
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/pkg/primitives"
)

// Connector implements read.ReadConnector with deterministic mock data.
type Connector struct {
	providerID string
	clockFunc  func() time.Time
	seed       string // For deterministic data generation
}

// Config configures the mock connector.
type Config struct {
	// ProviderID is the provider identifier.
	ProviderID string

	// ClockFunc provides the current time (for testing).
	ClockFunc func() time.Time

	// Seed controls deterministic data generation.
	Seed string
}

// NewConnector creates a new mock finance read connector.
func NewConnector(config Config) *Connector {
	providerID := config.ProviderID
	if providerID == "" {
		providerID = "mock-finance"
	}

	clockFunc := config.ClockFunc
	if clockFunc == nil {
		clockFunc = time.Now
	}

	seed := config.Seed
	if seed == "" {
		seed = "default-seed"
	}

	return &Connector{
		providerID: providerID,
		clockFunc:  clockFunc,
		seed:       seed,
	}
}

// ListAccounts returns deterministic mock accounts.
func (c *Connector) ListAccounts(ctx context.Context, env primitives.ExecutionEnvelope, req read.ListAccountsRequest) (*read.AccountsReceipt, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	now := c.clockFunc()

	accounts := []read.Account{
		{
			AccountID:       c.deterministicID("account", "checking"),
			Name:            "Primary Checking",
			Type:            read.AccountTypeChecking,
			Subtype:         "checking",
			Mask:            "1234",
			InstitutionID:   "mock-bank",
			InstitutionName: "Mock Bank",
		},
		{
			AccountID:       c.deterministicID("account", "savings"),
			Name:            "Savings Account",
			Type:            read.AccountTypeSavings,
			Subtype:         "savings",
			Mask:            "5678",
			InstitutionID:   "mock-bank",
			InstitutionName: "Mock Bank",
		},
		{
			AccountID:       c.deterministicID("account", "credit"),
			Name:            "Rewards Credit Card",
			Type:            read.AccountTypeCredit,
			Subtype:         "credit card",
			Mask:            "9012",
			InstitutionID:   "mock-bank",
			InstitutionName: "Mock Bank",
		},
	}

	// Add balances if requested
	if req.IncludeBalances {
		accounts[0].Balance = &read.Balance{
			CurrentCents:   523847, // $5,238.47
			AvailableCents: 523847,
			Currency:       "USD",
			AsOf:           now,
		}
		accounts[1].Balance = &read.Balance{
			CurrentCents:   1250000, // $12,500.00
			AvailableCents: 1250000,
			Currency:       "USD",
			AsOf:           now,
		}
		accounts[2].Balance = &read.Balance{
			CurrentCents:   -185632, // -$1,856.32 (credit card balance)
			AvailableCents: 814368,  // $8,143.68 available credit
			Currency:       "USD",
			AsOf:           now,
		}
	}

	return &read.AccountsReceipt{
		Accounts:   accounts,
		FetchedAt:  now,
		ProviderID: c.providerID,
		Partial:    false,
	}, nil
}

// ListTransactions returns deterministic mock transactions.
func (c *Connector) ListTransactions(ctx context.Context, env primitives.ExecutionEnvelope, req read.ListTransactionsRequest) (*read.TransactionsReceipt, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	now := c.clockFunc()

	// Generate deterministic transactions based on date range
	transactions := c.generateTransactions(req.StartDate, req.EndDate, req.AccountIDs)

	// Apply limit
	if req.Limit > 0 && len(transactions) > req.Limit {
		transactions = transactions[:req.Limit]
	}

	return &read.TransactionsReceipt{
		Transactions: transactions,
		FetchedAt:    now,
		ProviderID:   c.providerID,
		TotalCount:   len(transactions),
		HasMore:      false,
		Partial:      false,
	}, nil
}

// Supports returns the connector's capabilities.
// CRITICAL: Only Read is true. No Write capability exists.
func (c *Connector) Supports(ctx context.Context) read.Capabilities {
	return read.Capabilities{
		Read: true,
		// NOTE: There is no Write field — it doesn't exist by design
	}
}

// ProviderInfo returns information about the mock provider.
func (c *Connector) ProviderInfo() read.ProviderInfo {
	return read.ProviderInfo{
		ID:              c.providerID,
		Name:            "Mock Finance Provider",
		Type:            "mock",
		InstitutionID:   "mock-bank",
		InstitutionName: "Mock Bank",
	}
}

// generateTransactions creates deterministic transactions for the date range.
func (c *Connector) generateTransactions(startDate, endDate time.Time, accountIDs []string) []read.Transaction {
	var transactions []read.Transaction

	// Default to checking account if no filter
	accounts := accountIDs
	if len(accounts) == 0 {
		accounts = []string{
			c.deterministicID("account", "checking"),
			c.deterministicID("account", "credit"),
		}
	}

	// Generate transactions for each day in range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dayTransactions := c.transactionsForDay(d, accounts)
		transactions = append(transactions, dayTransactions...)
	}

	return transactions
}

// transactionsForDay generates deterministic transactions for a specific day.
func (c *Connector) transactionsForDay(date time.Time, accounts []string) []read.Transaction {
	var transactions []read.Transaction

	// Use date as seed for deterministic generation
	daySeed := fmt.Sprintf("%s:%s", c.seed, date.Format("2006-01-02"))
	hash := sha256.Sum256([]byte(daySeed))

	// Determine number of transactions (0-4 based on hash)
	numTx := int(hash[0]) % 5

	for i := 0; i < numTx; i++ {
		tx := c.generateTransaction(date, i, accounts, hash[:])
		transactions = append(transactions, tx)
	}

	return transactions
}

// generateTransaction creates a single deterministic transaction.
func (c *Connector) generateTransaction(date time.Time, index int, accounts []string, hash []byte) read.Transaction {
	// Deterministic selection based on hash
	merchantIndex := int(hash[index%len(hash)]) % len(mockMerchants)
	merchant := mockMerchants[merchantIndex]

	// Deterministic amount based on merchant and hash
	baseAmount := merchant.baseAmountCents
	variation := int64(hash[(index+1)%len(hash)]) * 100 // Up to $255 variation
	amount := baseAmount + variation

	// Deterministic account selection
	accountIndex := int(hash[(index+2)%len(hash)]) % len(accounts)
	accountID := accounts[accountIndex]

	return read.Transaction{
		TransactionID:      c.deterministicID("tx", fmt.Sprintf("%s-%d", date.Format("20060102"), index)),
		AccountID:          accountID,
		Date:               date,
		AmountCents:        -amount, // Negative for debits
		Currency:           "USD",
		Name:               merchant.name,
		MerchantName:       merchant.name,
		Pending:            false,
		ProviderCategory:   merchant.category,
		ProviderCategoryID: merchant.categoryID,
		PaymentChannel:     merchant.channel,
	}
}

// deterministicID generates a deterministic ID based on seed.
func (c *Connector) deterministicID(prefix, suffix string) string {
	data := fmt.Sprintf("%s:%s:%s", c.seed, prefix, suffix)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(hash[:8]))
}

// mockMerchant represents a mock merchant for transaction generation.
type mockMerchant struct {
	name            string
	category        string
	categoryID      string
	channel         string
	baseAmountCents int64
}

// mockMerchants provides deterministic merchant data.
var mockMerchants = []mockMerchant{
	{name: "Whole Foods Market", category: "Groceries", categoryID: "19047000", channel: "in store", baseAmountCents: 8500},
	{name: "Shell Gas Station", category: "Gas Stations", categoryID: "22001000", channel: "in store", baseAmountCents: 4500},
	{name: "Netflix", category: "Entertainment", categoryID: "18000000", channel: "online", baseAmountCents: 1599},
	{name: "Amazon.com", category: "Shopping", categoryID: "19000000", channel: "online", baseAmountCents: 3500},
	{name: "Starbucks", category: "Food and Drink", categoryID: "13005000", channel: "in store", baseAmountCents: 650},
	{name: "Target", category: "Shopping", categoryID: "19046000", channel: "in store", baseAmountCents: 7500},
	{name: "Uber", category: "Transportation", categoryID: "22000000", channel: "online", baseAmountCents: 2500},
	{name: "CVS Pharmacy", category: "Health", categoryID: "21007000", channel: "in store", baseAmountCents: 2200},
	{name: "Electric Company", category: "Utilities", categoryID: "18068000", channel: "online", baseAmountCents: 15000},
	{name: "Local Restaurant", category: "Food and Drink", categoryID: "13005000", channel: "in store", baseAmountCents: 4500},
}

// Verify interface compliance at compile time.
var _ read.ReadConnector = (*Connector)(nil)
