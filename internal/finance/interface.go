// Package finance provides the financial read runtime.
// This is a CONTROL PLANE component — READ and PROPOSE only.
//
// CRITICAL: No execution capability exists. This package cannot:
// - Move money
// - Initiate transfers
// - Execute payments
// - Schedule automated actions
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package finance

import (
	"context"
	"time"

	"quantumlife/pkg/primitives"
	"quantumlife/pkg/primitives/finance"
)

// Reader provides financial read and propose operations.
// CRITICAL: No write or execute methods exist.
type Reader interface {
	// Sync fetches and normalizes financial data from connected providers.
	// This is a READ operation — no external writes occur.
	Sync(ctx context.Context, env primitives.ExecutionEnvelope, req SyncRequest) (*SyncResult, error)

	// Observe generates observations from financial data.
	// Observations are informational only — no actions triggered.
	Observe(ctx context.Context, env primitives.ExecutionEnvelope, req ObserveRequest) (*ObservationsResult, error)

	// Propose generates proposals from observations.
	// Proposals are NON-BINDING — humans may freely ignore.
	Propose(ctx context.Context, env primitives.ExecutionEnvelope, req ProposeRequest) (*ProposalsResult, error)

	// Dismiss marks a proposal as dismissed.
	// Dismissed proposals are suppressed from future generation.
	Dismiss(ctx context.Context, env primitives.ExecutionEnvelope, req DismissRequest) error

	// GetSnapshot retrieves the latest financial snapshot.
	GetSnapshot(ctx context.Context, env primitives.ExecutionEnvelope, ownerType, ownerID string) (*finance.AccountSnapshot, error)

	// GetTransactions retrieves transactions within a window.
	GetTransactions(ctx context.Context, env primitives.ExecutionEnvelope, req GetTransactionsRequest) (*finance.TransactionBatch, error)

	// GetProposals retrieves active proposals.
	GetProposals(ctx context.Context, env primitives.ExecutionEnvelope, ownerType, ownerID string) ([]finance.FinancialProposal, error)
}

// SyncRequest contains parameters for syncing financial data.
type SyncRequest struct {
	// CircleID is the circle whose financial data to sync.
	CircleID string

	// ProviderID is the provider to sync from (optional, "" = all).
	ProviderID string

	// WindowDays is how many days of data to fetch.
	// Default: 90
	WindowDays int

	// IncludeBalances indicates whether to fetch current balances.
	IncludeBalances bool
}

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	// SnapshotID is the ID of the created snapshot.
	SnapshotID string

	// TransactionBatchID is the ID of the transaction batch.
	TransactionBatchID string

	// AccountCount is the number of accounts synced.
	AccountCount int

	// TransactionCount is the number of transactions synced.
	TransactionCount int

	// Freshness indicates data freshness.
	Freshness finance.DataFreshness

	// PartialReason explains partiality (if applicable).
	PartialReason string

	// SyncedAt is when the sync completed.
	SyncedAt time.Time

	// TraceID links to the sync operation.
	TraceID string
}

// ObserveRequest contains parameters for generating observations.
type ObserveRequest struct {
	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// SnapshotID is the snapshot to analyze (optional, "" = latest).
	SnapshotID string

	// CompareToSnapshotID is the previous snapshot for comparison (optional).
	CompareToSnapshotID string

	// WindowDays is the analysis window in days.
	WindowDays int
}

// ObservationsResult contains generated observations.
type ObservationsResult struct {
	// Observations contains the generated observations.
	Observations []finance.FinancialObservation

	// SuppressedCount is how many observations were suppressed.
	SuppressedCount int

	// AnalysisWindowStart is the start of the analysis window.
	AnalysisWindowStart time.Time

	// AnalysisWindowEnd is the end of the analysis window.
	AnalysisWindowEnd time.Time

	// TraceID links to the observe operation.
	TraceID string
}

// ProposeRequest contains parameters for generating proposals.
type ProposeRequest struct {
	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// ObservationIDs specifies which observations to consider.
	// If empty, uses all recent observations.
	ObservationIDs []string
}

// ProposalsResult contains generated proposals.
type ProposalsResult struct {
	// Proposals contains the generated proposals.
	Proposals []finance.FinancialProposal

	// SuppressedCount is how many proposals were suppressed.
	SuppressedCount int

	// SilenceApplied indicates if silence policy resulted in zero proposals.
	SilenceApplied bool

	// SilenceReason explains why silence was applied (if applicable).
	SilenceReason string

	// TraceID links to the propose operation.
	TraceID string
}

// DismissRequest contains parameters for dismissing a proposal.
type DismissRequest struct {
	// ProposalID is the proposal to dismiss.
	ProposalID string

	// DismissedBy is who dismissed the proposal.
	DismissedBy string
}

// GetTransactionsRequest contains parameters for getting transactions.
type GetTransactionsRequest struct {
	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// WindowStart is the start of the window.
	WindowStart time.Time

	// WindowEnd is the end of the window.
	WindowEnd time.Time

	// Categories filters by category (empty = all).
	Categories []string

	// AccountIDs filters by account (empty = all).
	AccountIDs []string
}

// ScopeView defines the visibility scope for financial operations.
type ScopeView struct {
	// CircleID is the viewing circle.
	CircleID string

	// IntersectionID is the intersection context (if any).
	IntersectionID string

	// VisibilityPolicy is the visibility policy to apply.
	VisibilityPolicy VisibilityPolicy
}

// VisibilityPolicy controls what data is visible.
// This is a copy of intersection.FinancialVisibilityPolicy to avoid import cycles.
type VisibilityPolicy struct {
	Enabled           bool
	AllowedAccounts   []string
	AllowedCategories []string
	WindowDays        int
	AggregationLevel  string
	AnonymizeAmounts  bool
}
