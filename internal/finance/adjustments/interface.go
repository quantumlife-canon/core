// Package adjustments provides v8.5 adjustment classification for refunds,
// reversals, and chargebacks.
//
// CRITICAL: Deterministic classification only. No ML, no probabilities.
// Uses provider category codes and description pattern matching.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package adjustments

import (
	"quantumlife/pkg/primitives/finance"
)

// Classifier classifies transactions as adjustments (refunds, reversals, chargebacks)
// and attempts to match them to original transactions.
type Classifier interface {
	// Classify determines the TransactionKind for a transaction.
	// Returns classification result with method and confidence metadata.
	Classify(tx finance.TransactionRecord, ctx ClassifyContext) ClassifyResult

	// FindRelated attempts to find the original transaction that this
	// adjustment relates to. Returns empty string if no match or ambiguous.
	FindRelated(adjustment finance.TransactionRecord, candidates []finance.TransactionRecord) RelatedResult
}

// ClassifyContext provides context for classification decisions.
type ClassifyContext struct {
	// Provider is the source provider (affects category code interpretation).
	Provider string

	// ProviderCategory is the provider's raw category code.
	ProviderCategory string

	// ProviderSubCategory is the provider's sub-category (if any).
	ProviderSubCategory string
}

// ClassifyResult contains the classification outcome.
type ClassifyResult struct {
	// Kind is the determined transaction kind.
	Kind finance.TransactionKind

	// Method describes how classification was determined.
	// Values: "provider_category", "description_pattern", "sign_inference", "default"
	Method string

	// Confidence indicates classification confidence.
	// Values: "high", "medium", "low"
	Confidence string

	// MatchedPattern is the pattern that matched (for description_pattern method).
	MatchedPattern string
}

// RelatedResult contains the result of finding a related transaction.
type RelatedResult struct {
	// RelatedCanonicalID is the canonical ID of the matched original transaction.
	// Empty if no match found or match is ambiguous.
	RelatedCanonicalID string

	// MatchConfidence indicates match quality.
	// Values: "high" (single exact match), "low" (ambiguous), "none" (no match)
	MatchConfidence string

	// UncertainRelation is true when multiple candidates match equally well.
	UncertainRelation bool

	// CandidateCount is how many potential matches were found.
	CandidateCount int
}

// EffectiveAmountResult contains the computed effective amount for spend calculations.
type EffectiveAmountResult struct {
	// EffectiveAmountCents is the amount that impacts spend.
	// For purchases: same as AmountCents
	// For refunds/reversals: opposite sign to reduce effective spend
	EffectiveAmountCents int64

	// IsAdjustment is true if this transaction modifies spend semantics.
	IsAdjustment bool
}

// ComputeEffectiveAmount calculates the effective amount for spend calculations.
//
// Logic:
// - Purchases: effective = raw amount (negative for expenses)
// - Refunds/Reversals/Chargebacks: effective = positive (reduces spend)
// - Fee: effective = raw amount
// - Transfer: effective = 0 (doesn't affect spend)
// - Unknown: effective = raw amount
func ComputeEffectiveAmount(tx finance.TransactionRecord, kind finance.TransactionKind) EffectiveAmountResult {
	switch kind {
	case finance.KindPurchase, finance.KindFee, finance.KindUnknown:
		return EffectiveAmountResult{
			EffectiveAmountCents: tx.AmountCents,
			IsAdjustment:         false,
		}

	case finance.KindRefund, finance.KindReversal, finance.KindChargeback:
		// Refunds typically come as positive amounts from providers
		// but if they come as negative, we flip them
		effective := tx.AmountCents
		if effective < 0 {
			effective = -effective
		}
		return EffectiveAmountResult{
			EffectiveAmountCents: effective,
			IsAdjustment:         true,
		}

	case finance.KindTransfer:
		return EffectiveAmountResult{
			EffectiveAmountCents: 0,
			IsAdjustment:         false,
		}

	default:
		return EffectiveAmountResult{
			EffectiveAmountCents: tx.AmountCents,
			IsAdjustment:         false,
		}
	}
}
