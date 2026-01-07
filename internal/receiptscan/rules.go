// Package receiptscan provides deterministic receipt classification for Gmail messages.
//
// Phase 31.1: Gmail Receipt Observers (Email -> CommerceSignals)
// Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md
//
// This file contains the rule-based classification engine.
// CRITICAL: Classification is deterministic - same inputs always produce same outputs.
//
// PRIVACY INVARIANTS:
//   - Domain patterns are used for classification but NEVER stored
//   - Subject/snippet keywords are matched but NEVER stored
//   - Only abstract category + horizon buckets are output
//   - Evidence hash contains only abstract tokens, never raw text
package receiptscan

import (
	"sort"
	"strings"
)

// Classify performs deterministic receipt classification on email metadata.
// CRITICAL: Input fields (FromDomain, Subject, Snippet) are used ONLY for
// classification and are NEVER stored or logged.
//
// Returns a ReceiptScanResult with:
//   - IsReceipt: true if the message appears to be a receipt
//   - Signals: detected receipt signals with abstract buckets
//   - ResultHash: deterministic hash for verification
func Classify(in ReceiptScanInput) ReceiptScanResult {
	if err := in.Validate(); err != nil {
		return ReceiptScanResult{
			IsReceipt:  false,
			Signals:    nil,
			ResultHash: ComputeEvidenceHash([]string{"invalid_input"}),
		}
	}

	// Normalize inputs for matching (lowercase, trimmed)
	domain := strings.ToLower(strings.TrimSpace(in.FromDomain))
	subject := strings.ToLower(strings.TrimSpace(in.Subject))
	snippet := strings.ToLower(strings.TrimSpace(in.Snippet))
	combined := subject + " " + snippet

	// Step 1: Check if this looks like a receipt
	isReceipt := detectReceipt(subject, snippet)
	if !isReceipt {
		result := ReceiptScanResult{
			IsReceipt: false,
			Signals:   nil,
		}
		result.ResultHash = result.ComputeHash()
		return result
	}

	// Step 2: Classify category from domain patterns
	category := classifyCategory(domain, combined)

	// Step 3: Classify horizon from content patterns
	horizon := classifyHorizon(combined)

	// Step 4: Build evidence hash from ABSTRACT tokens only
	// CRITICAL: Never include raw domain, subject, or snippet
	evidenceTokens := []string{
		in.CircleID,
		in.MessageIDHash,
		string(category),
		string(horizon),
	}
	evidenceHash := ComputeEvidenceHash(evidenceTokens)

	signal := ReceiptSignal{
		Category:     category,
		Horizon:      horizon,
		EvidenceHash: evidenceHash,
	}

	result := ReceiptScanResult{
		IsReceipt: true,
		Signals:   []ReceiptSignal{signal},
	}
	result.ResultHash = result.ComputeHash()

	return result
}

// detectReceipt checks if the email appears to be a receipt.
// Uses generic keywords that indicate transaction/order/receipt emails.
func detectReceipt(subject, snippet string) bool {
	combined := subject + " " + snippet

	// Receipt-indicating keywords (sorted for determinism)
	receiptKeywords := []string{
		"booking",
		"confirmation",
		"delivered",
		"delivery",
		"invoice",
		"order",
		"payment",
		"receipt",
		"renewal",
		"ride",
		"statement",
		"subscription",
		"ticket",
		"trip",
	}

	for _, kw := range receiptKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}

	return false
}

// classifyCategory determines the commerce category from domain and content.
// CRITICAL: Domain patterns are used for classification but NEVER stored.
func classifyCategory(domain, content string) ReceiptCategory {
	// Domain-based classification (deterministic order)
	// NOTE: We match domain patterns but output ONLY the category bucket

	// Food delivery domains
	deliveryDomains := []string{
		"deliveroo",
		"doordash",
		"grubhub",
		"justeat",
		"ubereats",
	}
	for _, d := range deliveryDomains {
		if strings.Contains(domain, d) {
			return CategoryDelivery
		}
	}

	// Transport domains
	transportDomains := []string{
		"bolt",
		"gett",
		"grab",
		"lyft",
		"uber",
	}
	for _, d := range transportDomains {
		// uber.com but not ubereats
		if strings.Contains(domain, d) && !strings.Contains(domain, "ubereats") {
			return CategoryTransport
		}
	}

	// Travel domains
	travelDomains := []string{
		"airbnb",
		"booking",
		"expedia",
		"hotels",
		"kayak",
		"skyscanner",
		"tripadvisor",
		"vrbo",
	}
	for _, d := range travelDomains {
		if strings.Contains(domain, d) {
			return CategoryTravel
		}
	}

	// Subscription domains
	subscriptionDomains := []string{
		"amazon", // Could be retail too, but prime etc.
		"apple",
		"disney",
		"hbo",
		"hulu",
		"netflix",
		"spotify",
		"youtube",
	}
	for _, d := range subscriptionDomains {
		if strings.Contains(domain, d) {
			// Check for subscription keywords in content
			if strings.Contains(content, "subscription") ||
				strings.Contains(content, "renewal") ||
				strings.Contains(content, "membership") ||
				strings.Contains(content, "monthly") {
				return CategorySubscription
			}
		}
	}

	// Retail domains
	retailDomains := []string{
		"amazon",
		"argos",
		"asos",
		"bestbuy",
		"currys",
		"ebay",
		"etsy",
		"johnlewis",
		"target",
		"walmart",
		"wayfair",
	}
	for _, d := range retailDomains {
		if strings.Contains(domain, d) {
			return CategoryRetail
		}
	}

	// Bills domains
	billsDomains := []string{
		"bt.com",
		"ee.co",
		"eon",
		"octopus",
		"scottishpower",
		"sky.com",
		"talktalk",
		"threeuk",
		"virginmedia",
		"vodafone",
	}
	for _, d := range billsDomains {
		if strings.Contains(domain, d) {
			return CategoryBills
		}
	}

	// Content-based fallback classification
	return classifyCategoryFromContent(content)
}

// classifyCategoryFromContent uses content keywords as fallback.
func classifyCategoryFromContent(content string) ReceiptCategory {
	// Ordered checks for determinism

	// Delivery keywords
	deliveryKeywords := []string{"delivered", "delivery", "food order", "your order is on"}
	for _, kw := range deliveryKeywords {
		if strings.Contains(content, kw) {
			return CategoryDelivery
		}
	}

	// Transport keywords
	transportKeywords := []string{"ride", "trip", "driver", "fare"}
	for _, kw := range transportKeywords {
		if strings.Contains(content, kw) {
			return CategoryTransport
		}
	}

	// Travel keywords
	travelKeywords := []string{"flight", "hotel", "booking confirmation", "check-in", "itinerary"}
	for _, kw := range travelKeywords {
		if strings.Contains(content, kw) {
			return CategoryTravel
		}
	}

	// Subscription keywords
	subscriptionKeywords := []string{"subscription", "renewal", "membership", "monthly", "annual"}
	for _, kw := range subscriptionKeywords {
		if strings.Contains(content, kw) {
			return CategorySubscription
		}
	}

	// Bills keywords
	billsKeywords := []string{"bill", "statement", "invoice", "payment due", "account balance"}
	for _, kw := range billsKeywords {
		if strings.Contains(content, kw) {
			return CategoryBills
		}
	}

	// Retail keywords
	retailKeywords := []string{"order", "shipped", "purchase", "bought"}
	for _, kw := range retailKeywords {
		if strings.Contains(content, kw) {
			return CategoryRetail
		}
	}

	return CategoryOther
}

// classifyHorizon determines the temporal horizon from content.
// CRITICAL: Uses generic keywords, outputs ONLY the bucket.
func classifyHorizon(content string) HorizonBucket {
	// NOW keywords - immediate relevance
	nowKeywords := []string{
		"arrived",
		"completed",
		"delivered",
		"here",
		"now",
		"ready",
		"today",
	}
	for _, kw := range nowKeywords {
		if strings.Contains(content, kw) {
			return HorizonNow
		}
	}

	// SOON keywords - near-future relevance
	soonKeywords := []string{
		"arriving",
		"dispatch",
		"en route",
		"on the way",
		"on its way",
		"out for delivery",
		"scheduled",
		"shipping",
		"tomorrow",
	}
	for _, kw := range soonKeywords {
		if strings.Contains(content, kw) {
			return HorizonSoon
		}
	}

	// LATER keywords - future relevance (default for subscriptions/bills)
	laterKeywords := []string{
		"annual",
		"bill",
		"due",
		"monthly",
		"next month",
		"renewal",
		"statement",
		"subscription",
		"upcoming",
	}
	for _, kw := range laterKeywords {
		if strings.Contains(content, kw) {
			return HorizonLater
		}
	}

	// Default to "later" for receipts (they're records of past events)
	return HorizonLater
}

// ClassifyBatch processes multiple inputs deterministically.
// Results are sorted by MessageIDHash for stable output ordering.
func ClassifyBatch(inputs []ReceiptScanInput) []ReceiptScanResult {
	if len(inputs) == 0 {
		return nil
	}

	// Sort inputs by MessageIDHash for determinism
	sortedInputs := make([]ReceiptScanInput, len(inputs))
	copy(sortedInputs, inputs)
	sort.Slice(sortedInputs, func(i, j int) bool {
		return sortedInputs[i].MessageIDHash < sortedInputs[j].MessageIDHash
	})

	results := make([]ReceiptScanResult, 0, len(sortedInputs))
	for _, in := range sortedInputs {
		result := Classify(in)
		results = append(results, result)
	}

	return results
}

// FilterReceiptsOnly returns only the results that are receipts.
func FilterReceiptsOnly(results []ReceiptScanResult) []ReceiptScanResult {
	var receipts []ReceiptScanResult
	for _, r := range results {
		if r.IsReceipt {
			receipts = append(receipts, r)
		}
	}
	return receipts
}

// CountByCategory counts receipt signals by category.
// Returns abstract counts only - never individual receipts.
func CountByCategory(results []ReceiptScanResult) map[ReceiptCategory]int {
	counts := make(map[ReceiptCategory]int)
	for _, r := range results {
		if !r.IsReceipt {
			continue
		}
		for _, sig := range r.Signals {
			counts[sig.Category]++
		}
	}
	return counts
}

// EarliestHorizon returns the most urgent horizon from a set of results.
// Priority: now > soon > later
func EarliestHorizon(results []ReceiptScanResult) HorizonBucket {
	earliest := HorizonLater
	for _, r := range results {
		if !r.IsReceipt {
			continue
		}
		for _, sig := range r.Signals {
			if sig.Horizon.Priority() < earliest.Priority() {
				earliest = sig.Horizon
			}
		}
	}
	return earliest
}
