// Package financetxscan classification rules.
//
// Phase 31.2: Commerce from Finance (TrueLayer → CommerceSignals)
// Reference: docs/ADR/ADR-0064-phase31-2-commerce-from-finance.md
//
// This file contains deterministic, rule-based transaction classification.
//
// CRITICAL: Classification uses ONLY:
//   - ProviderCategory (bank-assigned category string)
//   - ProviderCategoryID (MCC codes or similar)
//   - PaymentChannel (online, in_store, etc.)
//
// NEVER uses:
//   - Merchant names
//   - Transaction amounts
//   - Raw timestamps
//
// Classification priority:
//  1. ProviderCategory exact match (high confidence)
//  2. ProviderCategoryID/MCC match (medium confidence)
//  3. PaymentChannel inference (low confidence)
package financetxscan

import (
	"strings"

	"quantumlife/pkg/domain/commerceobserver"
)

// Classify classifies a single transaction input into a commerce category.
// Returns TransactionScanResult with IsClassified=false if unclassifiable.
//
// CRITICAL: This function is deterministic - same inputs always produce same outputs.
func Classify(in TransactionInput) TransactionScanResult {
	// Try each classification strategy in priority order
	if signal := classifyByProviderCategory(in); signal != nil {
		return TransactionScanResult{
			TransactionIDHash: in.TransactionIDHash,
			IsClassified:      true,
			Signal:            signal,
		}
	}

	if signal := classifyByMCC(in); signal != nil {
		return TransactionScanResult{
			TransactionIDHash: in.TransactionIDHash,
			IsClassified:      true,
			Signal:            signal,
		}
	}

	if signal := classifyByPaymentChannel(in); signal != nil {
		return TransactionScanResult{
			TransactionIDHash: in.TransactionIDHash,
			IsClassified:      true,
			Signal:            signal,
		}
	}

	// Unable to classify
	return TransactionScanResult{
		TransactionIDHash: in.TransactionIDHash,
		IsClassified:      false,
		Signal:            nil,
	}
}

// ClassifyBatch classifies multiple transaction inputs.
// Results are in the same order as inputs for determinism.
func ClassifyBatch(inputs []TransactionInput) []TransactionScanResult {
	results := make([]TransactionScanResult, len(inputs))
	for i, in := range inputs {
		results[i] = Classify(in)
	}
	return results
}

// classifyByProviderCategory uses the bank-assigned category string.
// This is the highest confidence classification.
func classifyByProviderCategory(in TransactionInput) *TransactionSignal {
	if in.ProviderCategory == "" {
		return nil
	}

	cat := strings.ToLower(in.ProviderCategory)

	// Food delivery / restaurants
	if containsAny(cat, []string{
		"food_and_drink",
		"eating_out",
		"restaurants",
		"takeaway",
		"fast_food",
		"cafe",
		"dining",
	}) {
		return buildSignal(commerceobserver.CategoryFoodDelivery, ConfidenceHigh, in)
	}

	// Transport
	if containsAny(cat, []string{
		"transport",
		"transportation",
		"travel",
		"taxi",
		"uber",
		"ride_share",
		"public_transport",
		"rail",
		"bus",
		"parking",
		"fuel",
		"petrol",
		"gas_station",
	}) {
		return buildSignal(commerceobserver.CategoryTransport, ConfidenceHigh, in)
	}

	// Retail / Shopping
	if containsAny(cat, []string{
		"shopping",
		"retail",
		"general_merchandise",
		"clothing",
		"electronics",
		"home_improvement",
		"department_store",
		"supermarket",
		"grocery",
		"groceries",
	}) {
		return buildSignal(commerceobserver.CategoryRetail, ConfidenceHigh, in)
	}

	// Subscriptions
	if containsAny(cat, []string{
		"subscription",
		"subscriptions",
		"digital_services",
		"streaming",
		"software",
		"saas",
		"membership",
	}) {
		return buildSignal(commerceobserver.CategorySubscriptions, ConfidenceHigh, in)
	}

	// Utilities / Bills
	if containsAny(cat, []string{
		"utilities",
		"bills",
		"utility",
		"electricity",
		"gas",
		"water",
		"phone",
		"mobile",
		"internet",
		"broadband",
		"insurance",
		"rent",
		"mortgage",
	}) {
		return buildSignal(commerceobserver.CategoryUtilities, ConfidenceHigh, in)
	}

	return nil
}

// classifyByMCC uses Merchant Category Codes (ISO 18245).
// This is medium confidence classification.
//
// MCC ranges reference: https://en.wikipedia.org/wiki/Merchant_category_code
func classifyByMCC(in TransactionInput) *TransactionSignal {
	if in.ProviderCategoryID == "" {
		return nil
	}

	mcc := strings.TrimSpace(in.ProviderCategoryID)

	// Food / Restaurants: 5811-5814
	if isInMCCRange(mcc, "5811", "5814") || isInMCCRange(mcc, "5812", "5814") {
		return buildSignal(commerceobserver.CategoryFoodDelivery, ConfidenceMedium, in)
	}

	// Grocery stores: 5411, 5422, 5441
	if mcc == "5411" || mcc == "5422" || mcc == "5441" {
		return buildSignal(commerceobserver.CategoryRetail, ConfidenceMedium, in)
	}

	// Transportation: 4111-4131 (public transport), 4784 (tolls), 5541-5542 (gas)
	if isInMCCRange(mcc, "4111", "4131") || mcc == "4784" ||
		mcc == "5541" || mcc == "5542" {
		return buildSignal(commerceobserver.CategoryTransport, ConfidenceMedium, in)
	}

	// Ride share / Taxi: 4121
	if mcc == "4121" {
		return buildSignal(commerceobserver.CategoryTransport, ConfidenceMedium, in)
	}

	// Utilities: 4812 (telecom), 4813 (phone), 4814 (telecom), 4899 (cable), 4900 (utilities)
	if mcc == "4812" || mcc == "4813" || mcc == "4814" || mcc == "4899" || mcc == "4900" {
		return buildSignal(commerceobserver.CategoryUtilities, ConfidenceMedium, in)
	}

	// Retail: 5311 (dept stores), 5331 (variety), 5399 (misc retail)
	if mcc == "5311" || mcc == "5331" || mcc == "5399" {
		return buildSignal(commerceobserver.CategoryRetail, ConfidenceMedium, in)
	}

	// Electronics: 5732, 5734
	if mcc == "5732" || mcc == "5734" {
		return buildSignal(commerceobserver.CategoryRetail, ConfidenceMedium, in)
	}

	// Insurance: 6300-6399
	if isInMCCRange(mcc, "6300", "6399") {
		return buildSignal(commerceobserver.CategoryUtilities, ConfidenceMedium, in)
	}

	// Subscriptions: Digital goods 5815-5818
	if isInMCCRange(mcc, "5815", "5818") {
		return buildSignal(commerceobserver.CategorySubscriptions, ConfidenceMedium, in)
	}

	return nil
}

// classifyByPaymentChannel infers category from payment channel.
// This is low confidence classification - only used as fallback.
func classifyByPaymentChannel(in TransactionInput) *TransactionSignal {
	if in.PaymentChannel == "" {
		return nil
	}

	channel := strings.ToLower(in.PaymentChannel)

	// Online payments are often subscriptions or retail
	if channel == "online" || channel == "web" || channel == "digital" {
		// Can't reliably distinguish - classify as other
		return buildSignal(commerceobserver.CategoryOther, ConfidenceLow, in)
	}

	// In-store is usually retail
	if channel == "in_store" || channel == "pos" || channel == "point_of_sale" {
		return buildSignal(commerceobserver.CategoryRetail, ConfidenceLow, in)
	}

	// Contactless could be transport or retail
	if channel == "contactless" {
		return buildSignal(commerceobserver.CategoryOther, ConfidenceLow, in)
	}

	return nil
}

// buildSignal constructs a TransactionSignal with computed evidence hash.
func buildSignal(cat commerceobserver.CategoryBucket, conf ConfidenceLevel, in TransactionInput) *TransactionSignal {
	// Build evidence hash from abstract tokens only
	// NEVER include merchant name, amounts, or raw timestamps
	tokens := []string{
		in.CircleID,
		in.TransactionIDHash,
		string(cat),
		string(conf),
		// Use abstracted versions of provider data
		abstractProviderCategory(in.ProviderCategory),
		abstractProviderCategoryID(in.ProviderCategoryID),
		abstractPaymentChannel(in.PaymentChannel),
	}
	evidenceHash := commerceobserver.ComputeEvidenceHash(tokens)

	return &TransactionSignal{
		Category:        cat,
		ConfidenceLevel: conf,
		EvidenceHash:    evidenceHash,
	}
}

// abstractProviderCategory converts to abstract form for hashing.
// The actual category is not stored - only a hash token.
func abstractProviderCategory(cat string) string {
	if cat == "" {
		return "no_category"
	}
	return "has_category"
}

// abstractProviderCategoryID converts to abstract form for hashing.
func abstractProviderCategoryID(id string) string {
	if id == "" {
		return "no_mcc"
	}
	return "has_mcc"
}

// abstractPaymentChannel converts to abstract form for hashing.
func abstractPaymentChannel(ch string) string {
	if ch == "" {
		return "no_channel"
	}
	return "has_channel"
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// isInMCCRange checks if mcc is within [start, end] range (inclusive).
// MCCs are compared as strings since they may have leading zeros.
func isInMCCRange(mcc, start, end string) bool {
	return mcc >= start && mcc <= end
}

// FilterClassifiedOnly returns only results that were successfully classified.
func FilterClassifiedOnly(results []TransactionScanResult) []TransactionScanResult {
	filtered := make([]TransactionScanResult, 0, len(results))
	for _, r := range results {
		if r.IsClassified && r.Signal != nil {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// CountByCategory counts classified results by category.
func CountByCategory(results []TransactionScanResult) map[commerceobserver.CategoryBucket]int {
	counts := make(map[commerceobserver.CategoryBucket]int)
	for _, r := range results {
		if r.IsClassified && r.Signal != nil {
			counts[r.Signal.Category]++
		}
	}
	return counts
}
