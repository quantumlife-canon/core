package normalize

import (
	"strings"
	"time"

	"quantumlife/internal/connectors/finance/read/providers/truelayer"
	"quantumlife/pkg/primitives/finance"
)

// TrueLayerNormalizer normalizes TrueLayer API responses to canonical form.
type TrueLayerNormalizer struct{}

// Provider returns "truelayer".
func (n *TrueLayerNormalizer) Provider() string {
	return "truelayer"
}

// NormalizeAccounts converts TrueLayer accounts to canonical form.
// raw must be *truelayer.AccountsResponse or []truelayer.TrueLayerAccount.
func (n *TrueLayerNormalizer) NormalizeAccounts(provider string, raw any) ([]NormalizedAccountResult, error) {
	var accounts []truelayer.TrueLayerAccount

	switch v := raw.(type) {
	case *truelayer.AccountsResponse:
		accounts = v.Results
	case []truelayer.TrueLayerAccount:
		accounts = v
	default:
		return nil, &NormalizationError{
			Provider: "truelayer",
			Message:  "expected *truelayer.AccountsResponse or []truelayer.TrueLayerAccount",
		}
	}

	results := make([]NormalizedAccountResult, 0, len(accounts))
	for _, acc := range accounts {
		normalized := n.normalizeAccount(acc)

		// Extract mask from account number if available
		mask := ""
		if acc.AccountNumber != nil {
			if acc.AccountNumber.Number != "" && len(acc.AccountNumber.Number) >= 4 {
				mask = acc.AccountNumber.Number[len(acc.AccountNumber.Number)-4:]
			} else if acc.AccountNumber.IBAN != "" && len(acc.AccountNumber.IBAN) >= 4 {
				mask = acc.AccountNumber.IBAN[len(acc.AccountNumber.IBAN)-4:]
			}
		}

		canonicalID := finance.CanonicalAccountID(finance.AccountIdentityInput{
			Provider:          "truelayer",
			ProviderAccountID: acc.AccountID,
			AccountType:       normalized.AccountType,
			Currency:          normalized.Currency,
			Mask:              mask,
		})

		results = append(results, NormalizedAccountResult{
			Account:           normalized,
			CanonicalID:       canonicalID,
			ProviderAccountID: acc.AccountID,
		})
	}

	return results, nil
}

// TrueLayerTransactionsInput combines accounts and transactions for normalization.
type TrueLayerTransactionsInput struct {
	AccountID    string
	Currency     string
	Transactions []truelayer.TrueLayerTransaction
}

// NormalizeTransactions converts TrueLayer transactions to canonical form.
// raw must be *truelayer.TransactionsResponse, []truelayer.TrueLayerTransaction,
// or TrueLayerTransactionsInput.
// accountMapping maps provider account IDs to canonical account IDs.
func (n *TrueLayerNormalizer) NormalizeTransactions(provider string, raw any, accountMapping map[string]string) ([]NormalizedTransactionResult, error) {
	var transactions []truelayer.TrueLayerTransaction
	var defaultAccountID string
	var defaultCurrency string

	switch v := raw.(type) {
	case *truelayer.TransactionsResponse:
		transactions = v.Results
	case []truelayer.TrueLayerTransaction:
		transactions = v
	case TrueLayerTransactionsInput:
		transactions = v.Transactions
		defaultAccountID = v.AccountID
		defaultCurrency = v.Currency
	default:
		return nil, &NormalizationError{
			Provider: "truelayer",
			Message:  "expected *truelayer.TransactionsResponse, []truelayer.TrueLayerTransaction, or TrueLayerTransactionsInput",
		}
	}

	results := make([]NormalizedTransactionResult, 0, len(transactions))
	for _, txn := range transactions {
		normalized, err := n.normalizeTransaction(txn, accountMapping, defaultAccountID, defaultCurrency)
		if err != nil {
			return nil, err
		}

		// Parse timestamp for canonical ID
		date, _ := truelayer.ParseTimestamp(txn.Timestamp)
		if date.IsZero() {
			date = time.Now().UTC()
		}

		// Determine the provider account ID
		providerAccountID := defaultAccountID
		if providerAccountID == "" {
			// TrueLayer transactions don't include account_id,
			// it must be provided via context
			providerAccountID = "unknown"
		}

		// Compute canonical transaction ID
		canonicalID := finance.CanonicalTransactionID(finance.TransactionIdentityInput{
			Provider:              "truelayer",
			ProviderAccountID:     providerAccountID,
			ProviderTransactionID: txn.TransactionID,
			Date:                  date,
			AmountMinorUnits:      normalized.AmountCents,
			Currency:              normalized.Currency,
			MerchantNormalized:    finance.NormalizeMerchant(txn.MerchantName),
		})

		// Compute match key for pending→posted matching
		canonicalAccountID := accountMapping[providerAccountID]
		if canonicalAccountID == "" {
			canonicalAccountID = providerAccountID
		}
		matchKey := finance.TransactionMatchKey(finance.TransactionMatchInput{
			CanonicalAccountID: canonicalAccountID,
			AmountMinorUnits:   normalized.AmountCents,
			Currency:           normalized.Currency,
			MerchantNormalized: finance.NormalizeMerchant(txn.MerchantName),
		})

		// TrueLayer doesn't have explicit pending status — all transactions are settled
		isPending := false

		results = append(results, NormalizedTransactionResult{
			Transaction:           normalized,
			CanonicalID:           canonicalID,
			MatchKey:              matchKey,
			ProviderTransactionID: txn.TransactionID,
			IsPending:             isPending,
		})
	}

	return results, nil
}

// normalizeAccount converts a single TrueLayer account.
func (n *TrueLayerNormalizer) normalizeAccount(acc truelayer.TrueLayerAccount) finance.NormalizedAccount {
	currency := acc.Currency
	if currency == "" {
		currency = "GBP" // TrueLayer is UK-focused
	}

	// Extract mask from account number
	mask := ""
	if acc.AccountNumber != nil {
		if acc.AccountNumber.Number != "" && len(acc.AccountNumber.Number) >= 4 {
			mask = acc.AccountNumber.Number[len(acc.AccountNumber.Number)-4:]
		} else if acc.AccountNumber.IBAN != "" && len(acc.AccountNumber.IBAN) >= 4 {
			mask = acc.AccountNumber.IBAN[len(acc.AccountNumber.IBAN)-4:]
		}
	}

	institutionName := ""
	if acc.Provider.DisplayName != "" {
		institutionName = acc.Provider.DisplayName
	}

	return finance.NormalizedAccount{
		ProviderAccountID: acc.AccountID,
		DisplayName:       acc.DisplayName,
		AccountType:       n.normalizeAccountType(acc.AccountType),
		Mask:              mask,
		Currency:          currency,
		InstitutionName:   institutionName,
		BalanceAsOf:       time.Now().UTC(),
		// Balance is fetched separately via balance endpoint
	}
}

// normalizeTransaction converts a single TrueLayer transaction.
func (n *TrueLayerNormalizer) normalizeTransaction(txn truelayer.TrueLayerTransaction, accountMapping map[string]string, defaultAccountID, defaultCurrency string) (finance.TransactionRecord, error) {
	currency := txn.Currency
	if currency == "" {
		currency = defaultCurrency
	}
	if currency == "" {
		currency = "GBP"
	}

	// Parse timestamp
	date, err := truelayer.ParseTimestamp(txn.Timestamp)
	if err != nil {
		date = time.Now().UTC()
	}

	// TrueLayer amounts: positive = credit, negative = debit
	// Our convention: negative = expense/debit, positive = income/credit
	amountCents := dollarsToCents(txn.Amount)
	// TrueLayer convention matches ours — no sign flip needed

	// Map account ID to canonical ID
	accountID := accountMapping[defaultAccountID]
	if accountID == "" {
		accountID = defaultAccountID
	}

	// Extract category
	category := txn.TransactionCategory
	categoryID := ""
	if len(txn.TransactionClassification) > 0 {
		category = txn.TransactionClassification[0]
	}

	return finance.TransactionRecord{
		SourceProvider:        "truelayer",
		ProviderTransactionID: txn.TransactionID,
		AccountID:             accountID,
		Date:                  date,
		AmountCents:           amountCents,
		Currency:              currency,
		Description:           txn.Description,
		MerchantName:          finance.NormalizeMerchant(txn.MerchantName),
		Category:              category,
		CategoryID:            categoryID,
		Pending:               false, // TrueLayer transactions are settled
		PaymentChannel:        "",    // TrueLayer doesn't provide this
		CreatedAt:             time.Now().UTC(),
		SchemaVersion:         "8.4",
		NormalizerVersion:     "truelayer-v1",
	}, nil
}

// normalizeAccountType maps TrueLayer account types to canonical types.
func (n *TrueLayerNormalizer) normalizeAccountType(accountType string) finance.NormalizedAccountType {
	// TrueLayer types: TRANSACTION, SAVINGS, BUSINESS_TRANSACTION, BUSINESS_SAVINGS
	upper := strings.ToUpper(accountType)
	switch {
	case strings.Contains(upper, "SAVINGS"):
		return finance.AccountTypeDepository
	case strings.Contains(upper, "TRANSACTION"):
		return finance.AccountTypeDepository
	case strings.Contains(upper, "CREDIT"):
		return finance.AccountTypeCredit
	case strings.Contains(upper, "LOAN"):
		return finance.AccountTypeLoan
	case strings.Contains(upper, "INVEST"):
		return finance.AccountTypeInvestment
	default:
		return finance.AccountTypeOther
	}
}
