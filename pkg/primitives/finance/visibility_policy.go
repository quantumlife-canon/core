// Package finance provides financial visibility policy types.
//
// CRITICAL: This is for READ visibility only. No execution authority.
// These types are used by internal/* packages for shared financial views.
//
// Reference: v8.6 Family Financial Intersections

package finance

import "fmt"

// VisibilityPolicy defines how financial data is shared in intersections.
// This is the canonical policy type used across internal packages.
//
// CRITICAL: READ + PROPOSE only. No execution authority.
// All parties receive identical views when RequireSymmetry=true.
type VisibilityPolicy struct {
	// Enabled indicates if shared financial view is enabled.
	Enabled bool

	// VisibilityLevel controls what financial data is visible.
	// Values: "full", "anonymized", "category_only", "totals_only"
	// Default: "category_only"
	VisibilityLevel VisibilityLevel

	// AmountGranularity controls how amounts are displayed.
	// Values: "exact", "bucketed", "hidden"
	// Default: "bucketed"
	AmountGranularity AmountGranularity

	// CategoriesAllowed lists categories included in shared view.
	// Empty = all categories allowed.
	CategoriesAllowed []string

	// AccountsIncluded lists canonical account IDs to include.
	// Empty = all accounts included.
	AccountsIncluded []string

	// RequireSymmetry ensures all parties receive identical views.
	// If true, any asymmetric visibility requires explicit approval.
	// Default: true (STRONGLY RECOMMENDED)
	RequireSymmetry bool

	// ProposalAllowed indicates if shared proposals can be generated.
	// Default: true
	ProposalAllowed bool

	// ContributingCircles lists circle IDs that contribute data.
	// Must be parties to the intersection.
	ContributingCircles []string

	// CurrencyDisplay controls currency visibility.
	// "all" - show all currencies separately
	// "primary" - show only primary currency
	// Default: "all"
	CurrencyDisplay string
}

// VisibilityLevel defines how much financial detail is shared.
type VisibilityLevel string

const (
	// VisibilityFull shows individual transactions with merchant names.
	VisibilityFull VisibilityLevel = "full"

	// VisibilityAnonymized shows transactions with anonymized merchants.
	VisibilityAnonymized VisibilityLevel = "anonymized"

	// VisibilityCategoryOnly shows category summaries only.
	VisibilityCategoryOnly VisibilityLevel = "category_only"

	// VisibilityTotalsOnly shows only aggregate totals.
	VisibilityTotalsOnly VisibilityLevel = "totals_only"
)

// AmountGranularity defines how amounts are displayed.
type AmountGranularity string

const (
	// GranularityExact shows exact amounts.
	GranularityExact AmountGranularity = "exact"

	// GranularityBucketed shows buckets (low/medium/high).
	GranularityBucketed AmountGranularity = "bucketed"

	// GranularityHidden hides all amounts.
	GranularityHidden AmountGranularity = "hidden"
)

// DefaultVisibilityPolicy returns conservative defaults for shared views.
// Conservative = more privacy, less detail, symmetry required.
func DefaultVisibilityPolicy() VisibilityPolicy {
	return VisibilityPolicy{
		Enabled:           false,
		VisibilityLevel:   VisibilityCategoryOnly,
		AmountGranularity: GranularityBucketed,
		RequireSymmetry:   true, // CRITICAL: Default to symmetric
		ProposalAllowed:   true,
		CurrencyDisplay:   "all",
	}
}

// Validate checks that the policy is valid.
func (p VisibilityPolicy) Validate() error {
	if !p.Enabled {
		return nil // Disabled policy is always valid
	}

	// Validate visibility level
	switch p.VisibilityLevel {
	case VisibilityFull, VisibilityAnonymized, VisibilityCategoryOnly, VisibilityTotalsOnly:
		// Valid
	case "":
		// Empty defaults to category_only
	default:
		return fmt.Errorf("invalid visibility level: %s", p.VisibilityLevel)
	}

	// Validate amount granularity
	switch p.AmountGranularity {
	case GranularityExact, GranularityBucketed, GranularityHidden:
		// Valid
	case "":
		// Empty defaults to bucketed
	default:
		return fmt.Errorf("invalid amount granularity: %s", p.AmountGranularity)
	}

	// Validate currency display
	switch p.CurrencyDisplay {
	case "all", "primary", "":
		// Valid
	default:
		return fmt.Errorf("invalid currency display: %s", p.CurrencyDisplay)
	}

	return nil
}

// IsSymmetric returns true if all parties receive identical views.
func (p VisibilityPolicy) IsSymmetric() bool {
	return p.RequireSymmetry
}
