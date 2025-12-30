package reconcile

import (
	"sort"
	"time"

	"quantumlife/internal/finance/normalize"
	"quantumlife/pkg/primitives/finance"
)

// DefaultEngine is the standard reconciliation engine implementation.
type DefaultEngine struct {
	// Version is the reconciler version string.
	Version string
}

// NewEngine creates a new reconciliation engine.
func NewEngine() *DefaultEngine {
	return &DefaultEngine{
		Version: "v8.5-reconcile-v2",
	}
}

// ReconcileAccounts merges normalized accounts, deduplicating by canonical ID.
func (e *DefaultEngine) ReconcileAccounts(ctx ReconcileContext, accounts []normalize.NormalizedAccountResult) (*AccountReconcileResult, error) {
	report := AccountReconcileReport{
		InputCount:    len(accounts),
		ByProvider:    make(map[string]int),
		ByAccountType: make(map[finance.NormalizedAccountType]int),
	}

	// Index by canonical ID
	byCanonicalID := make(map[string][]normalize.NormalizedAccountResult)
	for _, acc := range accounts {
		byCanonicalID[acc.CanonicalID] = append(byCanonicalID[acc.CanonicalID], acc)

		// Track provider counts
		provider := extractProvider(acc.Account.ProviderAccountID)
		if provider == "" {
			provider = "unknown"
		}
		report.ByProvider[provider]++
	}

	// Reconcile each canonical ID
	reconciled := make([]ReconciledAccount, 0, len(byCanonicalID))
	for canonicalID, candidates := range byCanonicalID {
		// Merge accounts with same canonical ID
		merged := e.mergeAccounts(candidates)

		// Collect provider sources
		providers := make([]string, 0)
		seen := make(map[string]bool)
		for _, c := range candidates {
			p := extractProvider(c.Account.ProviderAccountID)
			if p != "" && !seen[p] {
				providers = append(providers, p)
				seen[p] = true
			}
		}

		action := "new"
		if len(candidates) > 1 {
			action = "duplicate"
			report.DuplicatesRemoved += len(candidates) - 1
		}

		reconciled = append(reconciled, ReconciledAccount{
			Account:              merged,
			CanonicalID:          canonicalID,
			ProviderSources:      providers,
			ReconciliationAction: action,
		})

		report.ByAccountType[merged.AccountType]++
	}

	// Sort by canonical ID for deterministic output
	sort.Slice(reconciled, func(i, j int) bool {
		return reconciled[i].CanonicalID < reconciled[j].CanonicalID
	})

	report.OutputCount = len(reconciled)

	return &AccountReconcileResult{
		Accounts: reconciled,
		Report:   report,
	}, nil
}

// ReconcileTransactions merges normalized transactions.
func (e *DefaultEngine) ReconcileTransactions(ctx ReconcileContext, transactions []normalize.NormalizedTransactionResult) (*TransactionReconcileResult, error) {
	report := TransactionReconcileReport{
		InputCount: len(transactions),
		ByProvider: make(map[string]int),
		ByCategory: make(map[string]int),
	}

	// Index by canonical ID
	byCanonicalID := make(map[string][]normalize.NormalizedTransactionResult)
	// Index pending transactions by match key
	pendingByMatchKey := make(map[string][]normalize.NormalizedTransactionResult)

	for _, txn := range transactions {
		byCanonicalID[txn.CanonicalID] = append(byCanonicalID[txn.CanonicalID], txn)

		if txn.IsPending {
			pendingByMatchKey[txn.MatchKey] = append(pendingByMatchKey[txn.MatchKey], txn)
		}

		// Track provider counts
		report.ByProvider[txn.Transaction.SourceProvider]++
	}

	// Track which canonical IDs have been processed
	processed := make(map[string]bool)
	// Track pending transactions that were merged
	mergedPending := make(map[string]bool)

	reconciled := make([]ReconciledTransaction, 0)

	// First pass: process non-pending transactions, looking for pending matches
	for _, txn := range transactions {
		if txn.IsPending {
			continue
		}
		if processed[txn.CanonicalID] {
			continue
		}

		// Copy transaction record to allow modifications
		mergedTxn := txn.Transaction

		// Check for pending transactions with same match key
		var mergedFrom []string
		var pendingAmountCents *int64
		if pending, ok := pendingByMatchKey[txn.MatchKey]; ok {
			for _, p := range pending {
				if !mergedPending[p.CanonicalID] {
					mergedFrom = append(mergedFrom, p.CanonicalID)
					mergedPending[p.CanonicalID] = true
					report.PendingMerged++

					// v8.5: Detect partial capture (amount changed from pending to posted)
					if p.Transaction.AmountCents != txn.Transaction.AmountCents {
						report.PartialCaptureCount++
						// Store original pending amount for reference
						amt := p.Transaction.AmountCents
						pendingAmountCents = &amt
					}
				}
			}
		}

		// Set pending amount if partial capture detected
		if pendingAmountCents != nil {
			mergedTxn.PendingAmountCents = pendingAmountCents
		}

		// Check for duplicates with same canonical ID
		duplicates := byCanonicalID[txn.CanonicalID]
		action := "new"
		if len(duplicates) > 1 {
			action = "duplicate"
			report.DuplicatesRemoved += len(duplicates) - 1
		}
		if len(mergedFrom) > 0 {
			if pendingAmountCents != nil {
				action = "partial_capture"
			} else {
				action = "pending_to_posted"
			}
		}

		reconciled = append(reconciled, ReconciledTransaction{
			Transaction:          mergedTxn,
			CanonicalID:          txn.CanonicalID,
			MatchKey:             txn.MatchKey,
			ProviderSources:      []string{txn.Transaction.SourceProvider},
			ReconciliationAction: action,
			MergedFrom:           mergedFrom,
		})

		processed[txn.CanonicalID] = true
		report.PostedCount++
		report.ByCategory[txn.Transaction.Category]++

		if txn.Transaction.AmountCents < 0 {
			report.DebitCount++
		} else {
			report.CreditCount++
		}
	}

	// Second pass: add pending transactions that weren't merged
	for _, txn := range transactions {
		if !txn.IsPending {
			continue
		}
		if mergedPending[txn.CanonicalID] {
			continue
		}
		if processed[txn.CanonicalID] {
			continue
		}

		// Check for duplicates
		duplicates := byCanonicalID[txn.CanonicalID]
		action := "new"
		if len(duplicates) > 1 {
			action = "duplicate"
			report.DuplicatesRemoved += len(duplicates) - 1
		}

		reconciled = append(reconciled, ReconciledTransaction{
			Transaction:          txn.Transaction,
			CanonicalID:          txn.CanonicalID,
			MatchKey:             txn.MatchKey,
			ProviderSources:      []string{txn.Transaction.SourceProvider},
			ReconciliationAction: action,
		})

		processed[txn.CanonicalID] = true
		report.PendingCount++
		report.ByCategory[txn.Transaction.Category]++

		if txn.Transaction.AmountCents < 0 {
			report.DebitCount++
		} else {
			report.CreditCount++
		}
	}

	// Sort by date, then canonical ID for deterministic output
	sort.Slice(reconciled, func(i, j int) bool {
		if reconciled[i].Transaction.Date.Equal(reconciled[j].Transaction.Date) {
			return reconciled[i].CanonicalID < reconciled[j].CanonicalID
		}
		return reconciled[i].Transaction.Date.After(reconciled[j].Transaction.Date)
	})

	report.OutputCount = len(reconciled)

	return &TransactionReconcileResult{
		Transactions: reconciled,
		Report:       report,
	}, nil
}

// MergeMultiProvider reconciles data from multiple providers.
func (e *DefaultEngine) MergeMultiProvider(ctx ReconcileContext, providerData []ProviderData) (*MultiProviderResult, error) {
	// Collect all accounts and transactions
	var allAccounts []normalize.NormalizedAccountResult
	var allTransactions []normalize.NormalizedTransactionResult
	providers := make([]string, 0, len(providerData))

	for _, pd := range providerData {
		providers = append(providers, pd.Provider)
		allAccounts = append(allAccounts, pd.Accounts...)
		allTransactions = append(allTransactions, pd.Transactions...)
	}

	// Reconcile accounts
	accountResult, err := e.ReconcileAccounts(ctx, allAccounts)
	if err != nil {
		return nil, &ReconcileError{
			Phase:   "accounts",
			Message: "failed to reconcile accounts",
			Cause:   err,
		}
	}

	// Reconcile transactions
	txnResult, err := e.ReconcileTransactions(ctx, allTransactions)
	if err != nil {
		return nil, &ReconcileError{
			Phase:   "transactions",
			Message: "failed to reconcile transactions",
			Cause:   err,
		}
	}

	// Count overlaps (entities from multiple providers)
	overlapCount := 0
	for _, acc := range accountResult.Accounts {
		if len(acc.ProviderSources) > 1 {
			overlapCount++
		}
	}

	return &MultiProviderResult{
		Accounts:     accountResult.Accounts,
		Transactions: txnResult.Transactions,
		Report: MultiProviderReport{
			ProvidersProcessed: providers,
			AccountReport:      accountResult.Report,
			TransactionReport:  txnResult.Report,
			OverlapCount:       overlapCount,
		},
	}, nil
}

// mergeAccounts merges multiple accounts with the same canonical ID.
// Uses most recent data when available.
func (e *DefaultEngine) mergeAccounts(accounts []normalize.NormalizedAccountResult) finance.NormalizedAccount {
	if len(accounts) == 0 {
		return finance.NormalizedAccount{}
	}
	if len(accounts) == 1 {
		return accounts[0].Account
	}

	// Find most recent
	var newest finance.NormalizedAccount
	var newestTime time.Time

	for _, acc := range accounts {
		if acc.Account.BalanceAsOf.After(newestTime) {
			newest = acc.Account
			newestTime = acc.Account.BalanceAsOf
		}
	}

	// Fill in any missing fields from other sources
	for _, acc := range accounts {
		if newest.DisplayName == "" && acc.Account.DisplayName != "" {
			newest.DisplayName = acc.Account.DisplayName
		}
		if newest.InstitutionName == "" && acc.Account.InstitutionName != "" {
			newest.InstitutionName = acc.Account.InstitutionName
		}
		if newest.Mask == "" && acc.Account.Mask != "" {
			newest.Mask = acc.Account.Mask
		}
	}

	return newest
}

// extractProvider extracts the provider name from an account ID.
// This is a best-effort extraction based on common patterns.
func extractProvider(accountID string) string {
	// Provider-specific prefixes
	prefixes := map[string]string{
		"plaid_":     "plaid",
		"truelayer_": "truelayer",
		"tl_":        "truelayer",
		"mock_":      "mock",
	}

	for prefix, provider := range prefixes {
		if len(accountID) >= len(prefix) && accountID[:len(prefix)] == prefix {
			return provider
		}
	}

	return ""
}

// Ensure DefaultEngine implements Engine interface.
var _ Engine = (*DefaultEngine)(nil)
