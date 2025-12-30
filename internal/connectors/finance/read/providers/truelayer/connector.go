package truelayer

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/pkg/primitives"
)

// Connector implements read.ReadConnector for TrueLayer.
// CRITICAL: This connector is READ-ONLY by design.
// No payment, transfer, or write methods exist.
type Connector struct {
	client       *Client
	accessToken  string
	providerID   string
	providerName string
}

// ConnectorConfig configures the TrueLayer connector.
type ConnectorConfig struct {
	// Client is the TrueLayer HTTP client.
	Client *Client

	// AccessToken is the OAuth access token.
	// SENSITIVE: Never log this value.
	AccessToken string

	// ProviderID identifies this provider instance.
	ProviderID string

	// ProviderName is the display name from TrueLayer.
	ProviderName string
}

// NewConnector creates a new TrueLayer read connector.
// CRITICAL: Only read operations are possible.
func NewConnector(config ConnectorConfig) *Connector {
	providerID := config.ProviderID
	if providerID == "" {
		providerID = "truelayer"
	}

	providerName := config.ProviderName
	if providerName == "" {
		providerName = "TrueLayer"
	}

	return &Connector{
		client:       config.Client,
		accessToken:  config.AccessToken,
		providerID:   providerID,
		providerName: providerName,
	}
}

// ListAccounts returns financial accounts from TrueLayer.
// This is a READ-ONLY operation.
func (c *Connector) ListAccounts(ctx context.Context, env primitives.ExecutionEnvelope, req read.ListAccountsRequest) (*read.AccountsReceipt, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	// Fetch accounts from TrueLayer
	accountsResp, err := c.client.GetAccounts(ctx, c.accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}

	now := time.Now()
	var accounts []read.Account

	for _, tlAccount := range accountsResp.Results {
		account := read.Account{
			AccountID:       tlAccount.AccountID,
			Name:            tlAccount.DisplayName,
			Type:            mapAccountType(tlAccount.AccountType),
			Subtype:         tlAccount.AccountType,
			InstitutionID:   tlAccount.Provider.ProviderID,
			InstitutionName: tlAccount.Provider.DisplayName,
		}

		// Extract mask from account number if available
		if tlAccount.AccountNumber != nil {
			if tlAccount.AccountNumber.Number != "" && len(tlAccount.AccountNumber.Number) >= 4 {
				account.Mask = tlAccount.AccountNumber.Number[len(tlAccount.AccountNumber.Number)-4:]
			} else if tlAccount.AccountNumber.IBAN != "" && len(tlAccount.AccountNumber.IBAN) >= 4 {
				account.Mask = tlAccount.AccountNumber.IBAN[len(tlAccount.AccountNumber.IBAN)-4:]
			}
		}

		// Fetch balance if requested
		if req.IncludeBalances {
			balanceResp, err := c.client.GetBalance(ctx, c.accessToken, tlAccount.AccountID)
			if err == nil && len(balanceResp.Results) > 0 {
				bal := balanceResp.Results[0]
				account.Balance = &read.Balance{
					CurrentCents:   int64(bal.Current * 100),
					AvailableCents: int64(bal.Available * 100),
					Currency:       bal.Currency,
					AsOf:           now,
				}
			}
			// If balance fetch fails, continue without balance (partial data)
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

// ListTransactions returns transactions from TrueLayer.
// This is a READ-ONLY operation.
func (c *Connector) ListTransactions(ctx context.Context, env primitives.ExecutionEnvelope, req read.ListTransactionsRequest) (*read.TransactionsReceipt, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	now := time.Now()
	var allTransactions []read.Transaction

	// If no account IDs specified, fetch accounts first
	accountIDs := req.AccountIDs
	if len(accountIDs) == 0 {
		accountsResp, err := c.client.GetAccounts(ctx, c.accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch accounts: %w", err)
		}
		for _, acc := range accountsResp.Results {
			accountIDs = append(accountIDs, acc.AccountID)
		}
	}

	// Fetch transactions for each account
	for _, accountID := range accountIDs {
		txResp, err := c.client.GetTransactions(ctx, c.accessToken, accountID, req.StartDate, req.EndDate)
		if err != nil {
			// Continue with partial data if one account fails
			continue
		}

		for _, tlTx := range txResp.Results {
			tx := mapTransaction(tlTx, accountID)
			allTransactions = append(allTransactions, tx)
		}
	}

	// Apply limit if specified
	hasMore := false
	if req.Limit > 0 && len(allTransactions) > req.Limit {
		allTransactions = allTransactions[:req.Limit]
		hasMore = true
	}

	return &read.TransactionsReceipt{
		Transactions: allTransactions,
		FetchedAt:    now,
		ProviderID:   c.providerID,
		TotalCount:   len(allTransactions),
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

// ProviderInfo returns information about the TrueLayer provider.
func (c *Connector) ProviderInfo() read.ProviderInfo {
	return read.ProviderInfo{
		ID:   c.providerID,
		Name: c.providerName,
		Type: "truelayer",
	}
}

// mapAccountType maps TrueLayer account types to our canonical types.
func mapAccountType(tlType string) read.AccountType {
	switch tlType {
	case "TRANSACTION", "BUSINESS_TRANSACTION":
		return read.AccountTypeChecking
	case "SAVINGS", "BUSINESS_SAVINGS":
		return read.AccountTypeSavings
	default:
		return read.AccountTypeOther
	}
}

// mapTransaction converts a TrueLayer transaction to our canonical format.
func mapTransaction(tlTx TrueLayerTransaction, accountID string) read.Transaction {
	// Parse timestamp
	txDate, err := ParseTimestamp(tlTx.Timestamp)
	if err != nil {
		txDate = time.Now()
	}

	// Convert amount to cents (TrueLayer returns floats)
	amountCents := int64(tlTx.Amount * 100)

	// TrueLayer amounts are positive for credits, negative for debits
	// Our format uses negative for expenses (debits)
	if tlTx.TransactionType == "DEBIT" && amountCents > 0 {
		amountCents = -amountCents
	}

	tx := read.Transaction{
		TransactionID:    tlTx.TransactionID,
		AccountID:        accountID,
		Date:             txDate,
		AmountCents:      amountCents,
		Currency:         tlTx.Currency,
		Name:             tlTx.Description,
		MerchantName:     tlTx.MerchantName,
		Pending:          false, // TrueLayer returns settled transactions
		ProviderCategory: tlTx.TransactionCategory,
		PaymentChannel:   "unknown", // TrueLayer doesn't provide this
	}

	// Use first classification as category ID if available
	if len(tlTx.TransactionClassification) > 0 {
		tx.ProviderCategoryID = tlTx.TransactionClassification[0]
	}

	return tx
}

// Verify interface compliance at compile time.
var _ read.ReadConnector = (*Connector)(nil)
