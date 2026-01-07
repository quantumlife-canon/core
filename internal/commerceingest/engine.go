// Package commerceingest converts receipt scan results into Phase 31 CommerceObserver inputs.
//
// Phase 31.1: Gmail Receipt Observers (Email -> CommerceSignals)
// Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md
//
// This file contains the observation building engine.
// CRITICAL: Deterministic - same inputs always produce same outputs.
//
// PRIVACY INVARIANTS:
//   - Only abstract buckets are output
//   - Max 3 categories shown (per Phase 31)
//   - Raw counts are converted to magnitude buckets
//   - Evidence hashes contain only abstract tokens
package commerceingest

import (
	"sort"
	"time"

	"quantumlife/internal/receiptscan"
	"quantumlife/pkg/domain/commerceobserver"
)

// MaxCategories is the maximum number of categories to include.
// This matches Phase 31's MaxBuckets.
const MaxCategories = 3

// Engine builds commerce observations from receipt scan results.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new commerce ingest engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// BuildObservations converts receipt scan results into commerce observations.
// CRITICAL: Output is deterministic - same inputs always produce same result.
//
// Algorithm:
// 1. Filter to receipts only
// 2. Count by category (using receipt categories)
// 3. Map to commerce observer categories
// 4. Select top 3 categories by count (ties broken alphabetically)
// 5. Convert counts to frequency buckets
// 6. Build observations with evidence hashes
func (e *Engine) BuildObservations(in CommerceIngestInput) CommerceIngestResult {
	if err := in.Validate(); err != nil {
		return CommerceIngestResult{
			Observations:     nil,
			OverallMagnitude: MagnitudeNothing,
			StatusHash:       "invalid_input",
		}
	}

	// Filter to receipts only
	receipts := receiptscan.FilterReceiptsOnly(in.ScanResults)
	if len(receipts) == 0 {
		result := CommerceIngestResult{
			Observations:     nil,
			OverallMagnitude: MagnitudeNothing,
		}
		result.StatusHash = result.ComputeHash()
		return result
	}

	// Count by receipt category
	receiptCounts := receiptscan.CountByCategory(receipts)

	// Map to commerce observer categories and aggregate
	commerceCounts := make(map[commerceobserver.CategoryBucket]int)
	horizonByCategory := make(map[commerceobserver.CategoryBucket]receiptscan.HorizonBucket)

	for rc, count := range receiptCounts {
		cc := MapReceiptCategory(rc)
		commerceCounts[cc] += count

		// Track earliest horizon per category
		for _, r := range receipts {
			for _, sig := range r.Signals {
				if sig.Category == rc {
					if existing, ok := horizonByCategory[cc]; !ok || sig.Horizon.Priority() < existing.Priority() {
						horizonByCategory[cc] = sig.Horizon
					}
				}
			}
		}
	}

	// Select top categories (up to MaxCategories)
	selectedCategories := selectTopCategories(commerceCounts, MaxCategories)

	// Build observations
	observations := make([]commerceobserver.CommerceObservation, 0, len(selectedCategories))
	for _, cat := range selectedCategories {
		count := commerceCounts[cat]
		horizon := horizonByCategory[cat]
		if horizon == "" {
			horizon = receiptscan.HorizonLater
		}

		// Convert count to frequency bucket
		frequency := commerceobserver.ToFrequencyBucket(count)

		// Map horizon to stability
		stability := HorizonToStability(horizon)

		// Build evidence hash from abstract tokens only
		evidenceTokens := []string{
			in.CircleID,
			in.Period,
			string(cat),
			string(frequency),
			string(stability),
			in.SyncReceiptHash,
		}
		evidenceHash := commerceobserver.ComputeEvidenceHash(evidenceTokens)

		obs := commerceobserver.CommerceObservation{
			Source:       commerceobserver.SourceGmailReceipt,
			Category:     cat,
			Frequency:    frequency,
			Stability:    stability,
			Period:       in.Period,
			EvidenceHash: evidenceHash,
		}
		observations = append(observations, obs)
	}

	// Compute overall magnitude
	totalReceipts := len(receipts)
	overallMagnitude := ToMagnitudeBucket(totalReceipts)

	result := CommerceIngestResult{
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

// BuildFromGmailMessages converts Gmail message metadata into observations.
// This is a convenience function that combines receipt scanning and ingestion.
//
// CRITICAL: messageData is used for classification only and is NEVER stored.
// After this function returns, all raw data is discarded.
func (e *Engine) BuildFromGmailMessages(
	circleID string,
	period string,
	syncReceiptHash string,
	messageData []MessageData,
) CommerceIngestResult {
	if len(messageData) == 0 {
		result := CommerceIngestResult{
			Observations:     nil,
			OverallMagnitude: MagnitudeNothing,
		}
		result.StatusHash = result.ComputeHash()
		return result
	}

	// Build scan inputs
	scanInputs := make([]receiptscan.ReceiptScanInput, 0, len(messageData))
	for _, msg := range messageData {
		input := receiptscan.ReceiptScanInput{
			CircleID:      circleID,
			MessageIDHash: receiptscan.HashMessageID(msg.MessageID),
			FromDomain:    msg.SenderDomain,
			Subject:       msg.Subject,
			Snippet:       msg.Snippet,
		}
		scanInputs = append(scanInputs, input)
	}

	// Classify all messages
	scanResults := receiptscan.ClassifyBatch(scanInputs)

	// Build observations
	return e.BuildObservations(CommerceIngestInput{
		CircleID:        circleID,
		Period:          period,
		SyncReceiptHash: syncReceiptHash,
		ScanResults:     scanResults,
	})
}

// MessageData contains the message metadata needed for receipt classification.
// CRITICAL: This data is used for classification only and is NEVER stored.
type MessageData struct {
	// MessageID is the Gmail message ID (will be hashed, never stored raw).
	MessageID string

	// SenderDomain is the domain part of the sender email.
	// Used for classification, NOT stored.
	SenderDomain string

	// Subject is the email subject.
	// Used for classification, NOT stored.
	Subject string

	// Snippet is the email body preview.
	// Used for classification, NOT stored.
	Snippet string
}

// ExtractMessageData extracts MessageData from raw Gmail API responses.
// This is the boundary where raw data enters and abstract signals exit.
func ExtractMessageData(messageID, senderDomain, subject, snippet string) MessageData {
	return MessageData{
		MessageID:    messageID,
		SenderDomain: senderDomain,
		Subject:      subject,
		Snippet:      snippet,
	}
}
