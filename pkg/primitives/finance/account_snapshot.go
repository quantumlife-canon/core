// Package finance provides canonical financial primitives for v8 Financial Read.
//
// CRITICAL: These are READ-ONLY data structures. No execution primitives exist.
// All types are observations, not actions.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package finance

import (
	"time"
)

// AccountSnapshot represents a point-in-time view of financial accounts.
// This is the canonical representation after normalization from provider data.
type AccountSnapshot struct {
	// SnapshotID uniquely identifies this snapshot.
	SnapshotID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID that owns this snapshot.
	OwnerID string

	// SourceProvider identifies where this data came from.
	SourceProvider string

	// Accounts contains the normalized account data.
	Accounts []NormalizedAccount

	// TotalBalanceCents is the sum of all account balances in cents.
	TotalBalanceCents int64

	// Currency is the primary currency (ISO 4217).
	Currency string

	// FetchedAt is when the data was retrieved from the provider.
	FetchedAt time.Time

	// CreatedAt is when this snapshot was created.
	CreatedAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// NormalizerVersion is the version of normalization rules applied.
	NormalizerVersion string

	// TraceID links to the operation that created this snapshot.
	TraceID string

	// Freshness describes data freshness.
	Freshness DataFreshness

	// PartialReason explains partiality (if applicable).
	PartialReason string
}

// NormalizedAccount is the canonical account representation.
type NormalizedAccount struct {
	// AccountID is the normalized account identifier.
	AccountID string

	// ProviderAccountID is the original provider's account ID.
	ProviderAccountID string

	// DisplayName is the account display name.
	DisplayName string

	// AccountType is the normalized account type.
	AccountType NormalizedAccountType

	// Mask is the last 4 digits of the account number.
	Mask string

	// BalanceCents is the current balance in cents.
	BalanceCents int64

	// AvailableCents is the available balance in cents.
	AvailableCents int64

	// Currency is the account currency (ISO 4217).
	Currency string

	// InstitutionName is the financial institution name.
	InstitutionName string

	// BalanceAsOf is when the balance was last updated.
	BalanceAsOf time.Time
}

// NormalizedAccountType is the canonical account type.
type NormalizedAccountType string

const (
	AccountTypeDepository NormalizedAccountType = "depository"
	AccountTypeCredit     NormalizedAccountType = "credit"
	AccountTypeLoan       NormalizedAccountType = "loan"
	AccountTypeInvestment NormalizedAccountType = "investment"
	AccountTypeOther      NormalizedAccountType = "other"
)

// DataFreshness describes the freshness of financial data.
type DataFreshness string

const (
	FreshnessCurrent     DataFreshness = "current"
	FreshnessStale       DataFreshness = "stale"
	FreshnessPartial     DataFreshness = "partial"
	FreshnessUnavailable DataFreshness = "unavailable"
)

// IsPartial returns true if the snapshot has incomplete data.
func (s *AccountSnapshot) IsPartial() bool {
	return s.Freshness == FreshnessPartial || s.Freshness == FreshnessUnavailable
}

// IsStale returns true if the snapshot data is stale.
func (s *AccountSnapshot) IsStale() bool {
	return s.Freshness == FreshnessStale
}
