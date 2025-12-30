package normalize

import (
	"time"

	"quantumlife/pkg/primitives/finance"
)

// MockNormalizer normalizes mock/test data to canonical form.
// Used for testing and demos.
type MockNormalizer struct{}

// Provider returns "mock".
func (n *MockNormalizer) Provider() string {
	return "mock"
}

// MockAccount represents a mock account for testing.
type MockAccount struct {
	AccountID      string
	DisplayName    string
	AccountType    string
	Mask           string
	BalanceCents   int64
	AvailableCents int64
	Currency       string
}

// MockTransaction represents a mock transaction for testing.
type MockTransaction struct {
	TransactionID string
	AccountID     string
	Date          time.Time
	AmountCents   int64
	Currency      string
	Description   string
	MerchantName  string
	Category      string
	Pending       bool
}

// NormalizeAccounts converts mock accounts to canonical form.
// raw must be []MockAccount.
func (n *MockNormalizer) NormalizeAccounts(provider string, raw any) ([]NormalizedAccountResult, error) {
	accounts, ok := raw.([]MockAccount)
	if !ok {
		return nil, &NormalizationError{
			Provider: "mock",
			Message:  "expected []MockAccount",
		}
	}

	results := make([]NormalizedAccountResult, 0, len(accounts))
	for _, acc := range accounts {
		normalized := n.normalizeAccount(acc)
		canonicalID := finance.CanonicalAccountID(finance.AccountIdentityInput{
			Provider:          "mock",
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

// NormalizeTransactions converts mock transactions to canonical form.
// raw must be []MockTransaction.
// accountMapping maps provider account IDs to canonical account IDs.
func (n *MockNormalizer) NormalizeTransactions(provider string, raw any, accountMapping map[string]string) ([]NormalizedTransactionResult, error) {
	transactions, ok := raw.([]MockTransaction)
	if !ok {
		return nil, &NormalizationError{
			Provider: "mock",
			Message:  "expected []MockTransaction",
		}
	}

	results := make([]NormalizedTransactionResult, 0, len(transactions))
	for _, txn := range transactions {
		normalized := n.normalizeTransaction(txn, accountMapping)

		// Compute canonical transaction ID
		canonicalID := finance.CanonicalTransactionID(finance.TransactionIdentityInput{
			Provider:              "mock",
			ProviderAccountID:     txn.AccountID,
			ProviderTransactionID: txn.TransactionID,
			Date:                  txn.Date,
			AmountMinorUnits:      txn.AmountCents,
			Currency:              txn.Currency,
			MerchantNormalized:    finance.NormalizeMerchant(txn.MerchantName),
		})

		// Compute match key for pending→posted matching
		canonicalAccountID := accountMapping[txn.AccountID]
		if canonicalAccountID == "" {
			canonicalAccountID = txn.AccountID
		}
		matchKey := finance.TransactionMatchKey(finance.TransactionMatchInput{
			CanonicalAccountID: canonicalAccountID,
			AmountMinorUnits:   txn.AmountCents,
			Currency:           txn.Currency,
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

// normalizeAccount converts a single mock account.
func (n *MockNormalizer) normalizeAccount(acc MockAccount) finance.NormalizedAccount {
	currency := acc.Currency
	if currency == "" {
		currency = "USD"
	}

	return finance.NormalizedAccount{
		ProviderAccountID: acc.AccountID,
		DisplayName:       acc.DisplayName,
		AccountType:       n.normalizeAccountType(acc.AccountType),
		Mask:              acc.Mask,
		BalanceCents:      acc.BalanceCents,
		AvailableCents:    acc.AvailableCents,
		Currency:          currency,
		BalanceAsOf:       time.Now().UTC(),
	}
}

// normalizeTransaction converts a single mock transaction.
func (n *MockNormalizer) normalizeTransaction(txn MockTransaction, accountMapping map[string]string) finance.TransactionRecord {
	currency := txn.Currency
	if currency == "" {
		currency = "USD"
	}

	// Map account ID to canonical ID
	accountID := accountMapping[txn.AccountID]
	if accountID == "" {
		accountID = txn.AccountID
	}

	return finance.TransactionRecord{
		SourceProvider:        "mock",
		ProviderTransactionID: txn.TransactionID,
		AccountID:             accountID,
		Date:                  txn.Date,
		AmountCents:           txn.AmountCents,
		Currency:              currency,
		Description:           txn.Description,
		MerchantName:          finance.NormalizeMerchant(txn.MerchantName),
		Category:              txn.Category,
		Pending:               txn.Pending,
		CreatedAt:             time.Now().UTC(),
		SchemaVersion:         "8.4",
		NormalizerVersion:     "mock-v1",
	}
}

// normalizeAccountType maps mock account types to canonical types.
func (n *MockNormalizer) normalizeAccountType(accountType string) finance.NormalizedAccountType {
	switch accountType {
	case "depository", "checking", "savings":
		return finance.AccountTypeDepository
	case "credit", "credit_card":
		return finance.AccountTypeCredit
	case "loan", "mortgage":
		return finance.AccountTypeLoan
	case "investment", "brokerage":
		return finance.AccountTypeInvestment
	default:
		return finance.AccountTypeOther
	}
}
