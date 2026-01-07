// Package financetxscan engine for building commerce observations from finance transactions.
//
// Phase 31.2: Commerce from Finance (TrueLayer → CommerceSignals)
// Reference: docs/ADR/ADR-0064-phase31-2-commerce-from-finance.md
//
// This file contains the observation building engine.
// CRITICAL: Deterministic - same inputs always produce same outputs.
//
// PRIVACY INVARIANTS:
//   - Only abstract buckets are output
//   - Max 3 categories shown (per Phase 31)
//   - Raw counts are converted to magnitude buckets
//   - Evidence hashes contain only abstract tokens
//   - NO merchant names, NO amounts, NO raw timestamps
package financetxscan

import (
	"sort"
	"time"

	"quantumlife/pkg/domain/commerceobserver"
)

// MaxCategories is the maximum number of categories to include.
// This matches Phase 31's MaxBuckets.
const MaxCategories = 3

// Engine builds commerce observations from finance transaction scan results.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new finance ingest engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// BuildObservations converts transaction scan results into commerce observations.
// CRITICAL: Output is deterministic - same inputs always produce same result.
//
// Algorithm:
// 1. Filter to classified transactions only
// 2. Count by category
// 3. Select top 3 categories by count (ties broken alphabetically)
// 4. Convert counts to frequency buckets
// 5. Build observations with evidence hashes
func (e *Engine) BuildObservations(in FinanceIngestInput) FinanceIngestResult {
	if err := in.Validate(); err != nil {
		return FinanceIngestResult{
			Observations:     nil,
			OverallMagnitude: MagnitudeNothing,
			StatusHash:       "invalid_input",
		}
	}

	// Filter to classified transactions only
	classified := FilterClassifiedOnly(in.ScanResults)
	if len(classified) == 0 {
		result := FinanceIngestResult{
			Observations:     nil,
			OverallMagnitude: MagnitudeNothing,
		}
		result.StatusHash = result.ComputeHash()
		return result
	}

	// Count by category
	categoryCounts := CountByCategory(classified)

	// Select top categories (up to MaxCategories)
	selectedCategories := selectTopCategories(categoryCounts, MaxCategories)

	// Track confidence levels per category for stability inference
	confidenceByCategory := make(map[commerceobserver.CategoryBucket]ConfidenceLevel)
	for _, r := range classified {
		if r.Signal != nil {
			cat := r.Signal.Category
			// Keep highest confidence level seen for each category
			existing, ok := confidenceByCategory[cat]
			if !ok || confidencePriority(r.Signal.ConfidenceLevel) > confidencePriority(existing) {
				confidenceByCategory[cat] = r.Signal.ConfidenceLevel
			}
		}
	}

	// Build observations
	observations := make([]commerceobserver.CommerceObservation, 0, len(selectedCategories))
	for _, cat := range selectedCategories {
		count := categoryCounts[cat]
		confidence := confidenceByCategory[cat]
		if confidence == "" {
			confidence = ConfidenceLow
		}

		// Convert count to frequency bucket
		frequency := commerceobserver.ToFrequencyBucket(count)

		// Map confidence to stability
		stability := ConfidenceToStability(confidence)

		// Build evidence hash from abstract tokens only
		evidenceTokens := []string{
			in.CircleID,
			in.Period,
			string(cat),
			string(frequency),
			string(stability),
			string(confidence),
			in.SyncReceiptHash,
		}
		evidenceHash := commerceobserver.ComputeEvidenceHash(evidenceTokens)

		obs := commerceobserver.CommerceObservation{
			Source:       commerceobserver.SourceFinanceTrueLayer,
			Category:     cat,
			Frequency:    frequency,
			Stability:    stability,
			Period:       in.Period,
			EvidenceHash: evidenceHash,
		}
		observations = append(observations, obs)
	}

	// Compute overall magnitude
	totalClassified := len(classified)
	overallMagnitude := ToMagnitudeBucket(totalClassified)

	result := FinanceIngestResult{
		Observations:     observations,
		OverallMagnitude: overallMagnitude,
	}
	result.StatusHash = result.ComputeHash()

	return result
}

// selectTopCategories selects the top N categories by count.
// Ties are broken alphabetically for determinism.
func selectTopCategories(counts map[commerceobserver.CategoryBucket]int, n int) []commerceobserver.CategoryBucket {
	if len(counts) == 0 {
		return nil
	}

	// Create sortable slice
	type categoryCount struct {
		category commerceobserver.CategoryBucket
		count    int
	}
	items := make([]categoryCount, 0, len(counts))
	for cat, count := range counts {
		if count > 0 {
			items = append(items, categoryCount{cat, count})
		}
	}

	// Sort by count (desc), then category name (asc) for determinism
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return string(items[i].category) < string(items[j].category)
	})

	// Take top N
	if len(items) > n {
		items = items[:n]
	}

	// Extract categories
	result := make([]commerceobserver.CategoryBucket, len(items))
	for i, item := range items {
		result[i] = item.category
	}

	return result
}

// confidencePriority returns a numeric priority for confidence levels.
// Higher = more confident.
func confidencePriority(c ConfidenceLevel) int {
	switch c {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}

// ConfidenceToStability maps confidence level to stability bucket.
// High confidence = stable pattern
// Medium confidence = drifting pattern
// Low confidence = volatile pattern
func ConfidenceToStability(c ConfidenceLevel) commerceobserver.StabilityBucket {
	switch c {
	case ConfidenceHigh:
		return commerceobserver.StabilityStable
	case ConfidenceMedium:
		return commerceobserver.StabilityDrifting
	case ConfidenceLow:
		return commerceobserver.StabilityVolatile
	default:
		return commerceobserver.StabilityVolatile
	}
}

// TransactionData contains the minimal transaction metadata for classification.
// CRITICAL: This data is used for classification ONLY and is NEVER stored.
//
// WHAT IS USED:
//   - ProviderCategory: bank-assigned category
//   - ProviderCategoryID: MCC code or similar
//   - PaymentChannel: online, in_store, etc.
//
// WHAT IS NEVER USED:
//   - MerchantName (ignored even if provided)
//   - Amount (ignored even if provided)
//   - Timestamp (ignored even if provided)
type TransactionData struct {
	// TransactionID is the bank transaction ID (will be hashed, never stored raw).
	TransactionID string

	// ProviderCategory is the bank-assigned category.
	// Used for classification, NOT stored raw.
	ProviderCategory string

	// ProviderCategoryID is the MCC code or similar.
	// Used for classification, NOT stored raw.
	ProviderCategoryID string

	// PaymentChannel indicates payment type.
	// Used for classification, NOT stored raw.
	PaymentChannel string
}

// BuildFromTransactions converts transaction metadata into observations.
// This is a convenience function that combines classification and ingestion.
//
// CRITICAL: transactionData is used for classification only and is NEVER stored.
// After this function returns, all raw data is discarded.
func (e *Engine) BuildFromTransactions(
	circleID string,
	period string,
	syncReceiptHash string,
	transactionData []TransactionData,
) FinanceIngestResult {
	if len(transactionData) == 0 {
		result := FinanceIngestResult{
			Observations:     nil,
			OverallMagnitude: MagnitudeNothing,
		}
		result.StatusHash = result.ComputeHash()
		return result
	}

	// Build scan inputs
	scanInputs := make([]TransactionInput, 0, len(transactionData))
	for _, tx := range transactionData {
		input := TransactionInput{
			CircleID:           circleID,
			TransactionIDHash:  HashTransactionID(tx.TransactionID),
			ProviderCategory:   tx.ProviderCategory,
			ProviderCategoryID: tx.ProviderCategoryID,
			PaymentChannel:     tx.PaymentChannel,
		}
		scanInputs = append(scanInputs, input)
	}

	// Classify all transactions
	scanResults := ClassifyBatch(scanInputs)

	// Build observations
	return e.BuildObservations(FinanceIngestInput{
		CircleID:        circleID,
		Period:          period,
		SyncReceiptHash: syncReceiptHash,
		ScanResults:     scanResults,
	})
}

// ExtractTransactionData extracts TransactionData from raw fields.
// This is the boundary where raw data enters and abstract signals exit.
//
// CRITICAL: merchantName and amount are deliberately NOT accepted.
// This function signature makes it impossible to pass forbidden data.
func ExtractTransactionData(
	transactionID string,
	providerCategory string,
	providerCategoryID string,
	paymentChannel string,
) TransactionData {
	return TransactionData{
		TransactionID:      transactionID,
		ProviderCategory:   providerCategory,
		ProviderCategoryID: providerCategoryID,
		PaymentChannel:     paymentChannel,
	}
}

// PeriodFromTime converts a time to a period string (ISO week format).
func PeriodFromTime(t time.Time) string {
	year, week := t.ISOWeek()
	return formatPeriod(year, week)
}

// formatPeriod formats a year and week into "YYYY-Www" format.
func formatPeriod(year, week int) string {
	weekStr := "0"
	if week < 10 {
		weekStr = "0" + intToStr(week)
	} else {
		weekStr = intToStr(week)
	}
	return intToStr(year) + "-W" + weekStr
}

// intToStr converts an int to string without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
