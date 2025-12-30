// Package sharedview builds neutral financial views for multi-party intersections.
//
// CRITICAL: This package is READ-ONLY. No execution, payment, or automation.
// All views are symmetric when RequireSymmetry=true - all parties see identical data.
//
// Reference: v8.6 Family Financial Intersections
package sharedview

import (
	"time"

	"quantumlife/pkg/primitives/finance"
)

// SharedFinancialView is the neutral view shared across all intersection parties.
// When RequireSymmetry=true in the policy, all parties receive identical instances.
//
// CRITICAL: No individual attribution. No execution authority.
type SharedFinancialView struct {
	// IntersectionID identifies the intersection this view belongs to.
	IntersectionID string

	// ViewID is a unique identifier for this snapshot.
	ViewID string

	// GeneratedAt is when this view was computed.
	GeneratedAt time.Time

	// Policy is the policy used to generate this view.
	Policy finance.VisibilityPolicy

	// WindowStart is the beginning of the data window.
	WindowStart time.Time

	// WindowEnd is the end of the data window.
	WindowEnd time.Time

	// SpendByCategory contains aggregated spend per category, per currency.
	// Map: currency -> category -> CategorySpend
	SpendByCategory map[string]map[string]CategorySpend

	// TotalsByCurrency contains aggregated totals per currency.
	// Map: currency -> CurrencyTotal
	TotalsByCurrency map[string]CurrencyTotal

	// Observations contains neutral observations about the data.
	Observations []SharedObservation

	// Provenance tracks which circles contributed without individual attribution.
	Provenance ViewProvenance

	// ContentHash is a deterministic hash of the view content.
	// Used for symmetry verification.
	ContentHash string
}

// CategorySpend represents aggregated spending in a category.
type CategorySpend struct {
	// Category is the normalized category name.
	Category string

	// Currency is the ISO currency code.
	Currency string

	// TotalCents is the total spend in minor units.
	// Only present when AmountGranularity="exact".
	TotalCents int64

	// Bucket is the amount range when AmountGranularity="bucketed".
	// Values: "low", "medium", "high", "very_high"
	Bucket AmountBucket

	// TransactionCount is the number of transactions in this category.
	TransactionCount int

	// PercentOfTotal is the percentage of total spend (0-100).
	PercentOfTotal float64
}

// CurrencyTotal represents total spending in a currency.
type CurrencyTotal struct {
	// Currency is the ISO currency code.
	Currency string

	// TotalCents is the total spend in minor units.
	// Only present when AmountGranularity="exact".
	TotalCents int64

	// Bucket is the amount range when AmountGranularity="bucketed".
	Bucket AmountBucket

	// TransactionCount is the total number of transactions.
	TransactionCount int
}

// SharedObservation is a neutral observation about the shared financial data.
// Language must be calm, neutral, and non-judgmental.
//
// CRITICAL: No urgency, fear, shame, authority, or optimization language.
type SharedObservation struct {
	// ID uniquely identifies this observation.
	ID string

	// Type categorizes the observation.
	Type ObservationType

	// Summary is the human-readable neutral description.
	// Must use language patterns from v8 acceptance tests.
	Summary string

	// Category is the related category (if applicable).
	Category string

	// Currency is the related currency.
	Currency string

	// Bucket indicates the magnitude (when amounts hidden).
	Bucket AmountBucket

	// Triggered is when this observation was generated.
	Triggered time.Time
}

// ViewProvenance tracks data sources without individual attribution.
type ViewProvenance struct {
	// ContributingCircleIDs lists circles that contributed data.
	// IDs only - no amounts or details per circle.
	ContributingCircleIDs []string

	// ContributorCount is the number of contributing circles.
	ContributorCount int

	// DataFreshness indicates the staleness of the data.
	DataFreshness DataFreshness

	// LastSyncTime is when data was last synchronized.
	LastSyncTime time.Time

	// SymmetryVerified indicates if all parties received identical views.
	SymmetryVerified bool

	// SymmetryProofHash is the hash proving symmetry (when verified).
	SymmetryProofHash string
}

// AmountBucket represents a bucketed amount range.
type AmountBucket string

const (
	// BucketLow is for small amounts (< $100).
	BucketLow AmountBucket = "low"

	// BucketMedium is for moderate amounts ($100-$500).
	BucketMedium AmountBucket = "medium"

	// BucketHigh is for significant amounts ($500-$2000).
	BucketHigh AmountBucket = "high"

	// BucketVeryHigh is for large amounts (> $2000).
	BucketVeryHigh AmountBucket = "very_high"

	// BucketHidden is used when amounts are completely hidden.
	BucketHidden AmountBucket = "hidden"
)

// ObservationType categorizes observations.
type ObservationType string

const (
	// ObservationCategoryShift indicates a change in category distribution.
	ObservationCategoryShift ObservationType = "category_shift"

	// ObservationRecurring indicates a recurring pattern.
	ObservationRecurring ObservationType = "recurring_pattern"

	// ObservationUnusual indicates unusual activity (neutral language).
	ObservationUnusual ObservationType = "unusual_activity"

	// ObservationTrend indicates a trend over time.
	ObservationTrend ObservationType = "trend"
)

// DataFreshness indicates how current the data is.
type DataFreshness string

const (
	// FreshnessCurrent means data is less than 1 hour old.
	FreshnessCurrent DataFreshness = "current"

	// FreshnessRecent means data is 1-24 hours old.
	FreshnessRecent DataFreshness = "recent"

	// FreshnessStale means data is more than 24 hours old.
	FreshnessStale DataFreshness = "stale"

	// FreshnessUnknown means freshness cannot be determined.
	FreshnessUnknown DataFreshness = "unknown"
)

// CircleContribution represents one circle's financial data for view building.
// This is input to the view builder, NOT part of the output.
//
// CRITICAL: This data is aggregated before inclusion in SharedFinancialView.
// Individual circle amounts are NEVER exposed in the output.
type CircleContribution struct {
	// CircleID identifies the contributing circle.
	CircleID string

	// SpendByCategory contains the circle's spend per category per currency.
	// Map: currency -> category -> amount in cents
	SpendByCategory map[string]map[string]int64

	// TotalsByCurrency contains the circle's totals per currency.
	// Map: currency -> total in cents
	TotalsByCurrency map[string]int64

	// TransactionCounts contains counts per category per currency.
	// Map: currency -> category -> count
	TransactionCounts map[string]map[string]int

	// LastSyncTime is when this circle's data was last synced.
	LastSyncTime time.Time
}

// BuildRequest contains parameters for building a shared view.
type BuildRequest struct {
	// IntersectionID is the intersection to build for.
	IntersectionID string

	// Policy is the visibility policy to apply.
	Policy finance.VisibilityPolicy

	// Contributions is the data from each contributing circle.
	Contributions []CircleContribution

	// WindowStart is the beginning of the data window.
	WindowStart time.Time

	// WindowEnd is the end of the data window.
	WindowEnd time.Time
}

// VerifyRequest contains parameters for symmetry verification.
type VerifyRequest struct {
	// View is the shared view to verify.
	View *SharedFinancialView

	// PartyViews maps party IDs to the views they received.
	// All views should have the same content hash if symmetric.
	PartyViews map[string]*SharedFinancialView
}
