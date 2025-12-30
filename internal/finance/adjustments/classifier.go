package adjustments

import (
	"strings"
	"time"

	"quantumlife/pkg/primitives/finance"
)

// DefaultClassifier implements Classifier using deterministic rules.
type DefaultClassifier struct{}

// NewClassifier creates a new adjustment classifier.
func NewClassifier() Classifier {
	return &DefaultClassifier{}
}

// Classify determines the transaction kind using a priority-ordered approach:
// 1. Provider category codes (most reliable)
// 2. Description pattern matching
// 3. Sign inference (least reliable)
func (c *DefaultClassifier) Classify(tx finance.TransactionRecord, ctx ClassifyContext) ClassifyResult {
	// Priority 1: Provider category codes
	if result := c.classifyByProviderCategory(ctx); result.Kind != finance.KindUnknown {
		return result
	}

	// Priority 2: Description patterns
	if result := c.classifyByDescription(tx.Description); result.Kind != finance.KindUnknown {
		return result
	}

	// Priority 3: Sign inference
	if result := c.classifyBySign(tx); result.Kind != finance.KindUnknown {
		return result
	}

	// Default: Unknown (treated as purchase for spend)
	return ClassifyResult{
		Kind:       finance.KindPurchase,
		Method:     "default",
		Confidence: "low",
	}
}

// classifyByProviderCategory uses provider-specific category codes.
func (c *DefaultClassifier) classifyByProviderCategory(ctx ClassifyContext) ClassifyResult {
	category := strings.ToLower(ctx.ProviderCategory)

	// Plaid categories
	plaidRefundCategories := map[string]bool{
		"refund":             true,
		"credit_return":      true,
		"return":             true,
		"merchandise_return": true,
	}

	plaidFeeCategories := map[string]bool{
		"bank_fee":      true,
		"service_fee":   true,
		"overdraft_fee": true,
		"atm_fee":       true,
		"fee":           true,
	}

	plaidTransferCategories := map[string]bool{
		"transfer":          true,
		"internal_transfer": true,
		"wire_transfer":     true,
		"ach_transfer":      true,
	}

	if plaidRefundCategories[category] {
		return ClassifyResult{
			Kind:       finance.KindRefund,
			Method:     "provider_category",
			Confidence: "high",
		}
	}

	if plaidFeeCategories[category] {
		return ClassifyResult{
			Kind:       finance.KindFee,
			Method:     "provider_category",
			Confidence: "high",
		}
	}

	if plaidTransferCategories[category] {
		return ClassifyResult{
			Kind:       finance.KindTransfer,
			Method:     "provider_category",
			Confidence: "high",
		}
	}

	// Check sub-category for chargebacks
	subCategory := strings.ToLower(ctx.ProviderSubCategory)
	if strings.Contains(subCategory, "chargeback") || strings.Contains(subCategory, "dispute") {
		return ClassifyResult{
			Kind:       finance.KindChargeback,
			Method:     "provider_category",
			Confidence: "high",
		}
	}

	return ClassifyResult{Kind: finance.KindUnknown}
}

// classifyByDescription uses deterministic pattern matching on descriptions.
func (c *DefaultClassifier) classifyByDescription(description string) ClassifyResult {
	desc := strings.ToLower(description)

	// Refund patterns (ordered by specificity)
	refundPatterns := []string{
		"refund",
		"credit return",
		"merchandise credit",
		"return credit",
		"adjustment credit",
		"customer refund",
	}
	for _, pattern := range refundPatterns {
		if strings.Contains(desc, pattern) {
			return ClassifyResult{
				Kind:           finance.KindRefund,
				Method:         "description_pattern",
				Confidence:     "high",
				MatchedPattern: pattern,
			}
		}
	}

	// Reversal patterns
	reversalPatterns := []string{
		"reversal",
		"reversed",
		"void",
		"cancelled",
		"authorization reversal",
		"payment reversal",
	}
	for _, pattern := range reversalPatterns {
		if strings.Contains(desc, pattern) {
			return ClassifyResult{
				Kind:           finance.KindReversal,
				Method:         "description_pattern",
				Confidence:     "high",
				MatchedPattern: pattern,
			}
		}
	}

	// Chargeback patterns
	chargebackPatterns := []string{
		"chargeback",
		"dispute",
		"disputed charge",
		"fraud claim",
		"unauthorized charge",
	}
	for _, pattern := range chargebackPatterns {
		if strings.Contains(desc, pattern) {
			return ClassifyResult{
				Kind:           finance.KindChargeback,
				Method:         "description_pattern",
				Confidence:     "high",
				MatchedPattern: pattern,
			}
		}
	}

	// Fee patterns
	feePatterns := []string{
		"monthly fee",
		"service fee",
		"overdraft fee",
		"nsf fee",
		"atm fee",
		"foreign transaction fee",
		"maintenance fee",
	}
	for _, pattern := range feePatterns {
		if strings.Contains(desc, pattern) {
			return ClassifyResult{
				Kind:           finance.KindFee,
				Method:         "description_pattern",
				Confidence:     "high",
				MatchedPattern: pattern,
			}
		}
	}

	// Transfer patterns
	transferPatterns := []string{
		"transfer to",
		"transfer from",
		"wire transfer",
		"ach transfer",
		"internal transfer",
		"between accounts",
		"zelle transfer",
	}
	for _, pattern := range transferPatterns {
		if strings.Contains(desc, pattern) {
			return ClassifyResult{
				Kind:           finance.KindTransfer,
				Method:         "description_pattern",
				Confidence:     "medium",
				MatchedPattern: pattern,
			}
		}
	}

	return ClassifyResult{Kind: finance.KindUnknown}
}

// classifyBySign uses amount sign as a weak signal.
// Positive amounts on expense accounts often indicate refunds.
func (c *DefaultClassifier) classifyBySign(tx finance.TransactionRecord) ClassifyResult {
	// This is a weak signal - only use when amount is positive
	// which typically indicates a credit/refund on expense accounts
	if tx.AmountCents > 0 && !tx.Pending {
		return ClassifyResult{
			Kind:       finance.KindRefund,
			Method:     "sign_inference",
			Confidence: "low",
		}
	}

	return ClassifyResult{Kind: finance.KindUnknown}
}

// FindRelated attempts to find the original transaction for an adjustment.
// Uses merchant name, amount, and date proximity for matching.
func (c *DefaultClassifier) FindRelated(adjustment finance.TransactionRecord, candidates []finance.TransactionRecord) RelatedResult {
	if len(candidates) == 0 {
		return RelatedResult{
			MatchConfidence: "none",
			CandidateCount:  0,
		}
	}

	// Normalize merchant for matching
	adjMerchant := finance.NormalizeMerchant(adjustment.MerchantName)
	adjAmount := abs(adjustment.AmountCents)

	var matches []matchCandidate
	for _, cand := range candidates {
		// Skip if this is the same transaction
		if cand.RecordID == adjustment.RecordID {
			continue
		}

		// Skip if candidate is also an adjustment
		if cand.IsAdjustment {
			continue
		}

		// Must be opposite sign (adjustment is typically positive, purchase negative)
		if (adjustment.AmountCents > 0) == (cand.AmountCents > 0) {
			continue
		}

		candMerchant := finance.NormalizeMerchant(cand.MerchantName)
		candAmount := abs(cand.AmountCents)

		score := 0
		hasBasicMatch := false

		// Merchant match (most important)
		if adjMerchant == candMerchant && adjMerchant != "" {
			score += 100
			hasBasicMatch = true
		}

		// Amount match
		if adjAmount == candAmount {
			score += 50 // Exact match
			hasBasicMatch = true
		} else if isWithinTolerance(adjAmount, candAmount, 5) {
			score += 25 // Within 5% (partial refunds)
			hasBasicMatch = true
		}

		// Only consider date proximity if there's already a basic match
		// Date alone is not sufficient evidence
		if hasBasicMatch {
			daysDiff := daysBetween(adjustment.Date, cand.Date)
			if daysDiff <= 90 {
				score += 30 - (daysDiff / 3) // More recent = higher score
			}
		}

		if hasBasicMatch && score > 0 {
			matches = append(matches, matchCandidate{
				canonicalID: cand.RecordID,
				score:       score,
			})
		}
	}

	if len(matches) == 0 {
		return RelatedResult{
			MatchConfidence: "none",
			CandidateCount:  0,
		}
	}

	// Sort by score (highest first)
	sortMatches(matches)

	// Check if top match is significantly better than others
	topScore := matches[0].score
	if len(matches) == 1 {
		return RelatedResult{
			RelatedCanonicalID: matches[0].canonicalID,
			MatchConfidence:    "high",
			UncertainRelation:  false,
			CandidateCount:     1,
		}
	}

	// If multiple matches with similar scores, uncertain
	secondScore := matches[1].score
	if topScore-secondScore < 20 {
		return RelatedResult{
			RelatedCanonicalID: "", // Don't guess when uncertain
			MatchConfidence:    "low",
			UncertainRelation:  true,
			CandidateCount:     len(matches),
		}
	}

	return RelatedResult{
		RelatedCanonicalID: matches[0].canonicalID,
		MatchConfidence:    "high",
		UncertainRelation:  false,
		CandidateCount:     len(matches),
	}
}

type matchCandidate struct {
	canonicalID string
	score       int
}

func sortMatches(matches []matchCandidate) {
	// Simple bubble sort for small arrays
	for i := 0; i < len(matches)-1; i++ {
		for j := 0; j < len(matches)-i-1; j++ {
			if matches[j].score < matches[j+1].score {
				matches[j], matches[j+1] = matches[j+1], matches[j]
			}
		}
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func isWithinTolerance(a, b int64, tolerancePercent int64) bool {
	if a == 0 || b == 0 {
		return false
	}
	diff := abs(a - b)
	maxVal := a
	if b > a {
		maxVal = b
	}
	return (diff * 100 / maxVal) <= tolerancePercent
}

func daysBetween(a, b time.Time) int {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return int(diff.Hours() / 24)
}
