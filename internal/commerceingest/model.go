// Package commerceingest converts receipt scan results into Phase 31 CommerceObserver inputs.
//
// Phase 31.1: Gmail Receipt Observers (Email -> CommerceSignals)
// Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md
//
// CRITICAL INVARIANTS:
//   - NO raw data stored (no subjects, senders, amounts, merchants)
//   - Only abstract category buckets + magnitude buckets + horizon buckets
//   - Deterministic: same inputs => same observations
//   - stdlib only, no goroutines, no time.Now()
//
// This package takes receipt scan results and produces CommerceObservation
// inputs for the Phase 31 commerce observer engine.
package commerceingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/receiptscan"
	"quantumlife/pkg/domain/commerceobserver"
)

// SourceKind identifies the origin of commerce observations.
type SourceKind string

const (
	// SourceGmailReceipt indicates observations from Gmail receipt scanning.
	SourceGmailReceipt SourceKind = "gmail_receipt"
)

// MagnitudeBucket represents an abstract quantity.
// CRITICAL: Never expose raw counts in UI or storage.
type MagnitudeBucket string

const (
	// MagnitudeNothing indicates zero items.
	MagnitudeNothing MagnitudeBucket = "nothing"
	// MagnitudeAFew indicates 1-2 items.
	MagnitudeAFew MagnitudeBucket = "a_few"
	// MagnitudeSeveral indicates 3+ items.
	MagnitudeSeveral MagnitudeBucket = "several"
)

// ToMagnitudeBucket converts a raw count to a magnitude bucket.
// This is the ONLY place where raw counts are abstracted.
func ToMagnitudeBucket(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 2:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// AllMagnitudeBuckets returns all magnitude buckets in deterministic order.
func AllMagnitudeBuckets() []MagnitudeBucket {
	return []MagnitudeBucket{
		MagnitudeNothing,
		MagnitudeAFew,
		MagnitudeSeveral,
	}
}

// Validate checks if the magnitude bucket is valid.
func (m MagnitudeBucket) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid magnitude bucket: %s", m)
	}
}

// CommerceIngestInput represents the input for commerce observation generation.
type CommerceIngestInput struct {
	// CircleID identifies the circle.
	CircleID string

	// Period is the observation period (e.g., "2025-W03").
	Period string

	// SyncReceiptHash is the hash of the Gmail sync receipt (optional).
	SyncReceiptHash string

	// ScanResults contains the receipt scan results.
	ScanResults []receiptscan.ReceiptScanResult
}

// Validate checks if the input is valid.
func (i *CommerceIngestInput) Validate() error {
	if i.CircleID == "" {
		return fmt.Errorf("missing circle_id")
	}
	if i.Period == "" {
		return fmt.Errorf("missing period")
	}
	return nil
}

// CommerceIngestResult represents the output of commerce observation generation.
type CommerceIngestResult struct {
	// Observations contains the generated commerce observations.
	Observations []commerceobserver.CommerceObservation

	// OverallMagnitude is the abstract magnitude across all categories.
	OverallMagnitude MagnitudeBucket

	// StatusHash is a deterministic hash of the result.
	StatusHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (r *CommerceIngestResult) CanonicalString() string {
	var b strings.Builder
	b.WriteString("COMMERCE_INGEST_RESULT|v1|")
	b.WriteString(string(r.OverallMagnitude))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%d", len(r.Observations)))

	for _, obs := range r.Observations {
		b.WriteString("|")
		b.WriteString(string(obs.Category))
		b.WriteString(":")
		b.WriteString(string(obs.Frequency))
	}

	return b.String()
}

// ComputeHash computes a deterministic hash of the result.
func (r *CommerceIngestResult) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the result is valid.
func (r *CommerceIngestResult) Validate() error {
	if err := r.OverallMagnitude.Validate(); err != nil {
		return err
	}
	for i, obs := range r.Observations {
		if err := obs.Validate(); err != nil {
			return fmt.Errorf("observation %d: %w", i, err)
		}
	}
	return nil
}

// CategoryMapping maps receipt categories to commerce observer categories.
var CategoryMapping = map[receiptscan.ReceiptCategory]commerceobserver.CategoryBucket{
	receiptscan.CategoryDelivery:     commerceobserver.CategoryFoodDelivery,
	receiptscan.CategoryTransport:    commerceobserver.CategoryTransport,
	receiptscan.CategoryRetail:       commerceobserver.CategoryRetail,
	receiptscan.CategorySubscription: commerceobserver.CategorySubscriptions,
	receiptscan.CategoryBills:        commerceobserver.CategoryUtilities,
	receiptscan.CategoryTravel:       commerceobserver.CategoryOther, // Could add CategoryTravel to Phase 31
	receiptscan.CategoryOther:        commerceobserver.CategoryOther,
}

// MapReceiptCategory converts a receipt category to a commerce observer category.
func MapReceiptCategory(rc receiptscan.ReceiptCategory) commerceobserver.CategoryBucket {
	if mapped, ok := CategoryMapping[rc]; ok {
		return mapped
	}
	return commerceobserver.CategoryOther
}

// HorizonToStability maps horizon buckets to stability buckets.
// Now = volatile (changing), Soon = drifting, Later = stable
func HorizonToStability(h receiptscan.HorizonBucket) commerceobserver.StabilityBucket {
	switch h {
	case receiptscan.HorizonNow:
		return commerceobserver.StabilityVolatile
	case receiptscan.HorizonSoon:
		return commerceobserver.StabilityDrifting
	case receiptscan.HorizonLater:
		return commerceobserver.StabilityStable
	default:
		return commerceobserver.StabilityStable
	}
}

// PeriodFromTime converts a time to a period string (ISO week format).
// Format: "2025-W03" (year-week)
func PeriodFromTime(t time.Time) string {
	year, week := t.ISOWeek()
	return formatWeekPeriod(year, week)
}

// formatWeekPeriod formats a year and week number as a period string.
func formatWeekPeriod(year, week int) string {
	weekStr := ""
	if week < 10 {
		weekStr = "0" + digitToString(week)
	} else {
		weekStr = digitToString(week/10) + digitToString(week%10)
	}

	yearStr := ""
	for i := 3; i >= 0; i-- {
		digit := (year / pow10(i)) % 10
		yearStr += digitToString(digit)
	}

	return yearStr + "-W" + weekStr
}

func digitToString(d int) string {
	return string(rune('0' + d))
}

func pow10(n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}
