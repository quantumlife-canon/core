package normalize

import (
	"math"
	"time"

	"quantumlife/internal/connectors/finance/read/providers/plaid"
	"quantumlife/pkg/primitives/finance"
)

// PlaidNormalizer normalizes Plaid API responses to canonical form.
type PlaidNormalizer struct{}

// Provider returns "plaid".
func (n *PlaidNormalizer) Provider() string {
	return "plaid"
}

// NormalizeAccounts converts Plaid accounts to canonical form.
// raw must be *plaid.AccountsGetResponse or []plaid.PlaidAccount.
func (n *PlaidNormalizer) NormalizeAccounts(provider string, raw any) ([]NormalizedAccountResult, error) {
	var accounts []plaid.PlaidAccount

	switch v := raw.(type) {
	case *plaid.AccountsGetResponse:
		accounts = v.Accounts
	case []plaid.PlaidAccount:
		accounts = v
	default:
		return nil, &NormalizationError{
			Provider: "plaid",
			Message:  "expected *plaid.AccountsGetResponse or []plaid.PlaidAccount",
		}
	}

	results := make([]NormalizedAccountResult, 0, len(accounts))
	for _, acc := range accounts {
		normalized := n.normalizeAccount(acc)
		canonicalID := finance.CanonicalAccountID(finance.AccountIdentityInput{
			Provider:          "plaid",
			ProviderAccountID: acc.AccountID,
			AccountType:       normalized.AccountType,
			Currency:          normalized.Currency,
			Mask:              acc.Mask,
		})

		results = append(results, NormalizedAccountResult{
			Account:           normalized,
			CanonicalID:       canonicalID,
			ProviderAccountID: acc.AccountID,
		})
	}

	return results, nil
}

// NormalizeTransactions converts Plaid transactions to canonical form.
// raw must be *plaid.TransactionsGetResponse or []plaid.PlaidTransaction.
// accountMapping maps provider account IDs to canonical account IDs.
func (n *PlaidNormalizer) NormalizeTransactions(provider string, raw any, accountMapping map[string]string) ([]NormalizedTransactionResult, error) {
	var transactions []plaid.PlaidTransaction

	switch v := raw.(type) {
	case *plaid.TransactionsGetResponse:
		transactions = v.Transactions
	case []plaid.PlaidTransaction:
		transactions = v
	default:
		return nil, &NormalizationError{
			Provider: "plaid",
			Message:  "expected *plaid.TransactionsGetResponse or []plaid.PlaidTransaction",
		}
	}

	results := make([]NormalizedTransactionResult, 0, len(transactions))
	for _, txn := range transactions {
		normalized, err := n.normalizeTransaction(txn, accountMapping)
		if err != nil {
			return nil, err
		}

		// Parse date for canonical ID
		date, _ := plaid.ParseDate(txn.Date)
		if date.IsZero() {
			date = time.Now().UTC()
		}

		// Compute canonical transaction ID
		canonicalID := finance.CanonicalTransactionID(finance.TransactionIdentityInput{
			Provider:              "plaid",
			ProviderAccountID:     txn.AccountID,
			ProviderTransactionID: txn.TransactionID,
			Date:                  date,
			AmountMinorUnits:      normalized.AmountCents,
			Currency:              normalized.Currency,
			MerchantNormalized:    finance.NormalizeMerchant(txn.MerchantName),
		})

		// Compute match key for pending→posted matching
		canonicalAccountID := accountMapping[txn.AccountID]
		if canonicalAccountID == "" {
			canonicalAccountID = txn.AccountID
		}
		matchKey := finance.TransactionMatchKey(finance.TransactionMatchInput{
			CanonicalAccountID: canonicalAccountID,
			AmountMinorUnits:   normalized.AmountCents,
			Currency:           normalized.Currency,
			MerchantNormalized: finance.NormalizeMerchant(txn.MerchantName),
		})

		results = append(results, NormalizedTransactionResult{
			Transaction:           normalized,
			CanonicalID:           canonicalID,
			MatchKey:              matchKey,
			ProviderTransactionID: txn.TransactionID,
			IsPending:             txn.Pending,
		})
	}

	return results, nil
}

// normalizeAccount converts a single Plaid account.
func (n *PlaidNormalizer) normalizeAccount(acc plaid.PlaidAccount) finance.NormalizedAccount {
	currency := acc.Balances.IsoCurrencyCode
	if currency == "" {
		currency = acc.Balances.UnofficialCurrencyCode
	}
	if currency == "" {
		currency = "USD"
	}

	// Convert balances to cents (minor units)
	var balanceCents int64
	if acc.Balances.Current != nil {
		balanceCents = dollarsToCents(*acc.Balances.Current)
	}

	var availableCents int64
	if acc.Balances.Available != nil {
		availableCents = dollarsToCents(*acc.Balances.Available)
	}

	return finance.NormalizedAccount{
		ProviderAccountID: acc.AccountID,
		DisplayName:       firstNonEmpty(acc.OfficialName, acc.Name),
		AccountType:       n.normalizeAccountType(acc.Type, acc.Subtype),
		Mask:              acc.Mask,
		BalanceCents:      balanceCents,
		AvailableCents:    availableCents,
		Currency:          currency,
		BalanceAsOf:       time.Now().UTC(),
	}
}

// normalizeTransaction converts a single Plaid transaction.
func (n *PlaidNormalizer) normalizeTransaction(txn plaid.PlaidTransaction, accountMapping map[string]string) (finance.TransactionRecord, error) {
	currency := txn.IsoCurrencyCode
	if currency == "" {
		currency = txn.UnofficialCurrencyCode
	}
	if currency == "" {
		currency = "USD"
	}

	// Parse date
	date, err := plaid.ParseDate(txn.Date)
	if err != nil {
		date = time.Now().UTC()
	}

	// Plaid amounts: positive = money leaving account (debit)
	// Our convention: negative = expense/debit, positive = income/credit
	amountCents := dollarsToCents(txn.Amount)
	if amountCents > 0 {
		amountCents = -amountCents // Flip sign for debits
	} else {
		amountCents = -amountCents // Credits become positive
	}

	// Map account ID to canonical ID
	accountID := accountMapping[txn.AccountID]
	if accountID == "" {
		accountID = txn.AccountID
	}

	// Extract category
	category := ""
	categoryID := ""
	if len(txn.Category) > 0 {
		category = txn.Category[0]
	}
	if txn.CategoryID != "" {
		categoryID = txn.CategoryID
	}
	if txn.PersonalFinanceCategory != nil {
		category = txn.PersonalFinanceCategory.Primary
		categoryID = txn.PersonalFinanceCategory.Detailed
	}

	return finance.TransactionRecord{
		SourceProvider:        "plaid",
		ProviderTransactionID: txn.TransactionID,
		AccountID:             accountID,
		Date:                  date,
		AmountCents:           amountCents,
		Currency:              currency,
		Description:           txn.Name,
		MerchantName:          finance.NormalizeMerchant(txn.MerchantName),
		Category:              category,
		CategoryID:            categoryID,
		Pending:               txn.Pending,
		PaymentChannel:        txn.PaymentChannel,
		CreatedAt:             time.Now().UTC(),
		SchemaVersion:         "8.4",
		NormalizerVersion:     "plaid-v1",
	}, nil
}

// normalizeAccountType maps Plaid account types to canonical types.
func (n *PlaidNormalizer) normalizeAccountType(accountType, subtype string) finance.NormalizedAccountType {
	switch accountType {
	case "depository":
		return finance.AccountTypeDepository
	case "credit":
		return finance.AccountTypeCredit
	case "loan":
		return finance.AccountTypeLoan
	case "investment", "brokerage":
		return finance.AccountTypeInvestment
	default:
		return finance.AccountTypeOther
	}
}

// dollarsToCents converts a dollar amount to cents.
// Uses math.Round to handle floating-point precision issues.
func dollarsToCents(dollars float64) int64 {
	return int64(math.Round(dollars * 100))
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}
