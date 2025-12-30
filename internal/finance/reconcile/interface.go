// Package reconcile provides the v8.4 reconciliation engine.
//
// The reconciliation engine is the "truth layer" that merges normalized
// data from multiple providers into a single canonical view, handling:
// - Cross-window deduplication using canonical IDs
// - Pending → Posted transaction merging using match keys
// - Multi-provider conflict resolution
//
// CRITICAL: This is a READ-ONLY component. No execution capability exists.
// Reports contain COUNTS ONLY — never raw amounts.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package reconcile

import (
	"quantumlife/internal/finance/normalize"
	"quantumlife/pkg/primitives/finance"
)

// Engine reconciles normalized financial data into a canonical view.
// CRITICAL: No execution methods exist. This is read-only.
type Engine interface {
	// ReconcileAccounts merges normalized accounts, deduplicating by canonical ID.
	// Returns the reconciled accounts and a report with counts only.
	ReconcileAccounts(ctx ReconcileContext, accounts []normalize.NormalizedAccountResult) (*AccountReconcileResult, error)

	// ReconcileTransactions merges normalized transactions:
	// - Deduplicates by canonical ID
	// - Merges pending → posted by match key
	// Returns the reconciled transactions and a report with counts only.
	ReconcileTransactions(ctx ReconcileContext, transactions []normalize.NormalizedTransactionResult) (*TransactionReconcileResult, error)

	// MergeMultiProvider reconciles data from multiple providers for the same entity.
	// Applies conflict resolution and produces a unified canonical view.
	MergeMultiProvider(ctx ReconcileContext, providerData []ProviderData) (*MultiProviderResult, error)
}

// ReconcileContext provides context for reconciliation operations.
type ReconcileContext struct {
	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// TraceID links to the reconciliation operation.
	TraceID string

	// ReconcilerVersion is the version of reconciliation rules.
	ReconcilerVersion string
}

// AccountReconcileResult contains reconciled accounts and metrics.
type AccountReconcileResult struct {
	// Accounts contains the deduplicated canonical accounts.
	Accounts []ReconciledAccount

	// Report contains reconciliation metrics (counts only).
	Report AccountReconcileReport
}

// ReconciledAccount wraps a normalized account with reconciliation metadata.
type ReconciledAccount struct {
	// Account is the canonical account.
	Account finance.NormalizedAccount

	// CanonicalID is the computed canonical ID.
	CanonicalID string

	// ProviderSources lists providers that reported this account.
	ProviderSources []string

	// ReconciliationAction describes what happened during reconciliation.
	ReconciliationAction string
}

// AccountReconcileReport contains metrics for account reconciliation.
// CRITICAL: Contains COUNTS ONLY. No raw amounts.
type AccountReconcileReport struct {
	// InputCount is the number of accounts before reconciliation.
	InputCount int

	// OutputCount is the number of accounts after reconciliation.
	OutputCount int

	// DuplicatesRemoved is the count of duplicates eliminated.
	DuplicatesRemoved int

	// ConflictsResolved is the count of conflicts resolved.
	ConflictsResolved int

	// ByProvider breaks down input counts by provider.
	ByProvider map[string]int

	// ByAccountType breaks down output counts by account type.
	ByAccountType map[finance.NormalizedAccountType]int
}

// TransactionReconcileResult contains reconciled transactions and metrics.
type TransactionReconcileResult struct {
	// Transactions contains the deduplicated canonical transactions.
	Transactions []ReconciledTransaction

	// Report contains reconciliation metrics (counts only).
	Report TransactionReconcileReport
}

// ReconciledTransaction wraps a normalized transaction with reconciliation metadata.
type ReconciledTransaction struct {
	// Transaction is the canonical transaction.
	Transaction finance.TransactionRecord

	// CanonicalID is the computed canonical ID.
	CanonicalID string

	// MatchKey is the pending→posted match key.
	MatchKey string

	// ProviderSources lists providers that reported this transaction.
	ProviderSources []string

	// ReconciliationAction describes what happened during reconciliation.
	// Values: "new", "duplicate", "pending_to_posted", "conflict_resolved"
	ReconciliationAction string

	// MergedFrom contains IDs of transactions that were merged into this one.
	MergedFrom []string
}

// TransactionReconcileReport contains metrics for transaction reconciliation.
// CRITICAL: Contains COUNTS ONLY. No raw amounts.
type TransactionReconcileReport struct {
	// InputCount is the number of transactions before reconciliation.
	InputCount int

	// OutputCount is the number of transactions after reconciliation.
	OutputCount int

	// DuplicatesRemoved is the count of exact duplicates eliminated.
	DuplicatesRemoved int

	// PendingMerged is the count of pending transactions merged to posted.
	PendingMerged int

	// ConflictsResolved is the count of conflicts resolved.
	ConflictsResolved int

	// PendingCount is the count of transactions still pending.
	PendingCount int

	// PostedCount is the count of posted transactions.
	PostedCount int

	// ByProvider breaks down input counts by provider.
	ByProvider map[string]int

	// ByCategory breaks down output counts by category.
	ByCategory map[string]int

	// ByDirection breaks down output counts by direction.
	DebitCount  int
	CreditCount int
}

// ProviderData contains normalized data from a single provider.
type ProviderData struct {
	// Provider is the provider name.
	Provider string

	// Accounts contains normalized accounts.
	Accounts []normalize.NormalizedAccountResult

	// Transactions contains normalized transactions.
	Transactions []normalize.NormalizedTransactionResult
}

// MultiProviderResult contains merged data from multiple providers.
type MultiProviderResult struct {
	// Accounts contains the merged accounts.
	Accounts []ReconciledAccount

	// Transactions contains the merged transactions.
	Transactions []ReconciledTransaction

	// Report contains reconciliation metrics.
	Report MultiProviderReport
}

// MultiProviderReport contains metrics for multi-provider reconciliation.
// CRITICAL: Contains COUNTS ONLY. No raw amounts.
type MultiProviderReport struct {
	// ProvidersProcessed lists providers that contributed data.
	ProvidersProcessed []string

	// AccountReport contains account reconciliation metrics.
	AccountReport AccountReconcileReport

	// TransactionReport contains transaction reconciliation metrics.
	TransactionReport TransactionReconcileReport

	// OverlapCount is the count of entities seen from multiple providers.
	OverlapCount int
}

// ReconcileError represents a reconciliation error.
type ReconcileError struct {
	Phase   string
	Message string
	Cause   error
}

func (e *ReconcileError) Error() string {
	if e.Cause != nil {
		return "reconcile: " + e.Phase + ": " + e.Message + ": " + e.Cause.Error()
	}
	return "reconcile: " + e.Phase + ": " + e.Message
}

func (e *ReconcileError) Unwrap() error {
	return e.Cause
}
