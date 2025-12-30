// Package finance provides financial observation types.
//
// CRITICAL: Observations are human-readable insights for INFORMATIONAL purposes only.
// They are NOT actions, NOT recommendations, NOT urgency triggers.
// Language must be neutral, observational, and non-manipulative.
package finance

import (
	"time"
)

// FinancialObservation is a human-readable insight derived from financial data.
// Observations are informational only — they do not trigger actions.
//
// CRITICAL: Language requirements:
// - Neutral, observational tone
// - No urgency ("act now", "immediately")
// - No fear ("warning", "alert", "danger")
// - No shame ("overspending", "excessive")
// - No authority ("you should", "you must")
type FinancialObservation struct {
	// ObservationID uniquely identifies this observation.
	ObservationID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// Type describes what kind of observation this is.
	Type ObservationType

	// Title is a brief summary (neutral language).
	Title string

	// Description is the full observation text (neutral language).
	Description string

	// Basis lists the data points that produced this observation.
	Basis []string

	// Assumptions lists any assumptions made.
	Assumptions []string

	// Limitations lists what this observation does not account for.
	Limitations []string

	// NumericValue is the primary numeric value (if applicable).
	NumericValue *int64

	// ComparisonValue is what the numeric value is compared to (if applicable).
	ComparisonValue *int64

	// ChangeCents is the change amount (if applicable).
	ChangeCents *int64

	// ChangePercent is the percentage change (if applicable).
	ChangePercent *float64

	// Category is the relevant category (if applicable).
	Category string

	// WindowStart is the start of the observation window.
	WindowStart time.Time

	// WindowEnd is the end of the observation window.
	WindowEnd time.Time

	// Severity indicates how notable this observation is.
	// This is NOT urgency — it's just magnitude classification.
	Severity ObservationSeverity

	// CreatedAt is when this observation was created.
	CreatedAt time.Time

	// ExpiresAt is when this observation becomes stale.
	ExpiresAt time.Time

	// SchemaVersion is the version of this schema.
	SchemaVersion string

	// TraceID links to the operation that created this observation.
	TraceID string

	// SourceSnapshotIDs links to the snapshots used.
	SourceSnapshotIDs []string

	// Fingerprint is a stable hash for deduplication/dismissal.
	Fingerprint string

	// Reason explains why this observation was generated.
	// Used for generating proposal rationale.
	Reason string
}

// ObservationType describes the kind of observation.
type ObservationType string

const (
	// Balance observations
	ObservationBalanceChange   ObservationType = "balance_change"
	ObservationLowBalance      ObservationType = "low_balance"
	ObservationHighBalance     ObservationType = "high_balance"
	ObservationBalanceIncrease ObservationType = "balance_increase"
	ObservationBalanceDecrease ObservationType = "balance_decrease"

	// Spending observations
	ObservationCategoryShift     ObservationType = "category_shift"
	ObservationLargeTransaction  ObservationType = "large_transaction"
	ObservationUnusualSpending   ObservationType = "unusual_spending"
	ObservationSpendingIncrease  ObservationType = "spending_increase"
	ObservationSpendingDecrease  ObservationType = "spending_decrease"
	ObservationNewMerchant       ObservationType = "new_merchant"
	ObservationRecurringDetected ObservationType = "recurring_detected"

	// Cashflow observations
	ObservationNegativeCashflow ObservationType = "negative_cashflow"
	ObservationPositiveCashflow ObservationType = "positive_cashflow"
	ObservationCashflowShift    ObservationType = "cashflow_shift"

	// Data quality observations
	ObservationStaleData   ObservationType = "stale_data"
	ObservationPartialData ObservationType = "partial_data"
)

// ObservationSeverity indicates the magnitude (NOT urgency).
type ObservationSeverity string

const (
	// SeverityInfo is for informational observations.
	SeverityInfo ObservationSeverity = "info"

	// SeverityNotable is for observations that exceed typical thresholds.
	SeverityNotable ObservationSeverity = "notable"

	// SeveritySignificant is for observations with large magnitude.
	SeveritySignificant ObservationSeverity = "significant"
)

// IsExpired returns true if the observation has expired.
func (o *FinancialObservation) IsExpired(now time.Time) bool {
	return now.After(o.ExpiresAt)
}

// ObservationBatch contains multiple observations from an analysis.
type ObservationBatch struct {
	// BatchID uniquely identifies this batch.
	BatchID string

	// OwnerType is "circle" or "intersection".
	OwnerType string

	// OwnerID is the circle or intersection ID.
	OwnerID string

	// Observations contains the observations.
	Observations []FinancialObservation

	// AnalysisWindowStart is when analysis began.
	AnalysisWindowStart time.Time

	// AnalysisWindowEnd is when analysis ended.
	AnalysisWindowEnd time.Time

	// CreatedAt is when this batch was created.
	CreatedAt time.Time

	// TraceID links to the operation that created this batch.
	TraceID string

	// SuppressedCount is how many observations were suppressed (dismissed/decay).
	SuppressedCount int
}
