package plaid

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/pkg/primitives"
)

// Connector implements read.ReadConnector for Plaid.
// CRITICAL: This connector is READ-ONLY by design.
// No payment, transfer, or write methods exist.
type Connector struct {
	client          *Client
	accessToken     string
	providerID      string
	institutionID   string
	institutionName string
}

// ConnectorConfig configures the Plaid connector.
type ConnectorConfig struct {
	// Client is the Plaid HTTP client.
	Client *Client

	// AccessToken is the Plaid access token.
	// SENSITIVE: Never log this value.
	AccessToken string

	// ProviderID identifies this provider instance.
	ProviderID string

	// InstitutionID is the Plaid institution ID.
	InstitutionID string

	// InstitutionName is the display name of the institution.
	InstitutionName string
}

// NewConnector creates a new Plaid read connector.
// CRITICAL: Only read operations are possible.
func NewConnector(config ConnectorConfig) *Connector {
	providerID := config.ProviderID
	if providerID == "" {
		providerID = "plaid"
	}

	return &Connector{
		client:          config.Client,
		accessToken:     config.AccessToken,
		providerID:      providerID,
		institutionID:   config.InstitutionID,
		institutionName: config.InstitutionName,
	}
}

// ListAccounts returns financial accounts from Plaid.
// This is a READ-ONLY operation.
func (c *Connector) ListAccounts(ctx context.Context, env primitives.ExecutionEnvelope, req read.ListAccountsRequest) (*read.AccountsReceipt, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	// Fetch accounts from Plaid
	accountsResp, err := c.client.GetAccounts(ctx, c.accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}

	now := time.Now()
	var accounts []read.Account

	for _, plaidAccount := range accountsResp.Accounts {
		account := read.Account{
			AccountID:       plaidAccount.AccountID,
			Name:            plaidAccount.Name,
			Type:            mapAccountType(plaidAccount.Type),
			Subtype:         plaidAccount.Subtype,
			Mask:            plaidAccount.Mask,
			InstitutionID:   c.institutionID,
			InstitutionName: c.institutionName,
		}

		// Add balance if available
		if req.IncludeBalances {
			account.Balance = mapBalance(plaidAccount.Balances, now)
		}

		accounts = append(accounts, account)
	}

	return &read.AccountsReceipt{
		Accounts:   accounts,
		FetchedAt:  now,
		ProviderID: c.providerID,
		Partial:    false,
	}, nil
}

// ListTransactions returns transactions from Plaid.
// This is a READ-ONLY operation.
func (c *Connector) ListTransactions(ctx context.Context, env primitives.ExecutionEnvelope, req read.ListTransactionsRequest) (*read.TransactionsReceipt, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	now := time.Now()

	// Build options
	var opts *TransactionsGetOptions
	if len(req.AccountIDs) > 0 || req.Limit > 0 {
		opts = &TransactionsGetOptions{
			AccountIDs: req.AccountIDs,
			Count:      req.Limit,
		}
	}

	// Fetch transactions from Plaid
	txResp, err := c.client.GetTransactions(ctx, c.accessToken, req.StartDate, req.EndDate, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}

	var transactions []read.Transaction
	for _, plaidTx := range txResp.Transactions {
		tx := mapTransaction(plaidTx)
		transactions = append(transactions, tx)
	}

	hasMore := txResp.TotalTransactions > len(transactions)

	return &read.TransactionsReceipt{
		Transactions: transactions,
		FetchedAt:    now,
		ProviderID:   c.providerID,
		TotalCount:   txResp.TotalTransactions,
		HasMore:      hasMore,
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

// ProviderInfo returns information about the Plaid provider.
func (c *Connector) ProviderInfo() read.ProviderInfo {
	return read.ProviderInfo{
		ID:              c.providerID,
		Name:            "Plaid",
		Type:            "plaid",
		InstitutionID:   c.institutionID,
		InstitutionName: c.institutionName,
	}
}

// mapAccountType maps Plaid account types to our canonical types.
func mapAccountType(plaidType string) read.AccountType {
	switch plaidType {
	case "depository":
		return read.AccountTypeChecking // Default depository to checking
	case "credit":
		return read.AccountTypeCredit
	case "loan":
		return read.AccountTypeLoan
	case "investment":
		return read.AccountTypeInvestment
	default:
		return read.AccountTypeOther
	}
}

// mapBalance converts Plaid balances to our canonical format.
func mapBalance(plaidBal PlaidBalances, asOf time.Time) *read.Balance {
	balance := &read.Balance{
		AsOf: asOf,
	}

	// Determine currency
	if plaidBal.IsoCurrencyCode != "" {
		balance.Currency = plaidBal.IsoCurrencyCode
	} else if plaidBal.UnofficialCurrencyCode != "" {
		balance.Currency = plaidBal.UnofficialCurrencyCode
	} else {
		balance.Currency = "USD"
	}

	// Convert to cents
	if plaidBal.Current != nil {
		balance.CurrentCents = int64(*plaidBal.Current * 100)
	}
	if plaidBal.Available != nil {
		balance.AvailableCents = int64(*plaidBal.Available * 100)
	}

	return balance
}

// mapTransaction converts a Plaid transaction to our canonical format.
func mapTransaction(plaidTx PlaidTransaction) read.Transaction {
	// Parse date
	txDate, err := ParseDate(plaidTx.Date)
	if err != nil {
		txDate = time.Now()
	}

	// Determine currency
	currency := plaidTx.IsoCurrencyCode
	if currency == "" {
		currency = plaidTx.UnofficialCurrencyCode
	}
	if currency == "" {
		currency = "USD"
	}

	// Convert amount to cents
	// Plaid amounts are positive for debits (money out) and negative for credits (money in)
	// We use negative for expenses (debits), so we negate Plaid's convention
	amountCents := int64(plaidTx.Amount * 100)
	if amountCents > 0 {
		amountCents = -amountCents // Expenses are negative
	} else {
		amountCents = -amountCents // Income is positive
	}

	tx := read.Transaction{
		TransactionID:  plaidTx.TransactionID,
		AccountID:      plaidTx.AccountID,
		Date:           txDate,
		AmountCents:    amountCents,
		Currency:       currency,
		Name:           plaidTx.Name,
		MerchantName:   plaidTx.MerchantName,
		Pending:        plaidTx.Pending,
		PaymentChannel: plaidTx.PaymentChannel,
	}

	// Use category if available
	if len(plaidTx.Category) > 0 {
		tx.ProviderCategory = plaidTx.Category[0]
	}
	if plaidTx.CategoryID != "" {
		tx.ProviderCategoryID = plaidTx.CategoryID
	}

	// Use personal finance category if available
	if plaidTx.PersonalFinanceCategory != nil {
		tx.ProviderCategory = plaidTx.PersonalFinanceCategory.Primary
	}

	return tx
}

// Verify interface compliance at compile time.
var _ read.ReadConnector = (*Connector)(nil)
