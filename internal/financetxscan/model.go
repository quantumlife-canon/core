// Package financetxscan classifies TrueLayer transactions into abstract commerce categories.
//
// Phase 31.2: Commerce from Finance (TrueLayer → CommerceSignals)
// Reference: docs/ADR/ADR-0064-phase31-2-commerce-from-finance.md
//
// CRITICAL INVARIANTS:
//   - NO merchant names stored or used for classification
//   - NO amounts stored or used
//   - NO raw timestamps stored
//   - Only ProviderCategory, ProviderCategoryID, PaymentChannel are used
//   - Output is abstract category buckets only
//   - Deterministic: same inputs always produce same outputs
//   - stdlib only, no goroutines, no time.Now()
//
// This complements Phase 31.1 (Gmail receipts) by adding bank transaction signals.
package financetxscan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"quantumlife/pkg/domain/commerceobserver"
)

// TransactionInput contains the minimal transaction metadata for classification.
//
// CRITICAL: These fields are used for classification ONLY.
// NO amounts, NO merchant names, NO timestamps.
type TransactionInput struct {
	// CircleID identifies the circle.
	CircleID string

	// TransactionIDHash is a hash of the transaction ID (never raw).
	TransactionIDHash string

	// ProviderCategory is the bank-assigned category (e.g., "FOOD_AND_DRINK").
	// Used for classification, NOT stored raw.
	ProviderCategory string

	// ProviderCategoryID is the bank-assigned category code (e.g., MCC codes).
	// Used for classification, NOT stored raw.
	ProviderCategoryID string

	// PaymentChannel indicates payment type (e.g., "online", "in_store").
	// Used for classification, NOT stored raw.
	PaymentChannel string
}

// TransactionSignal represents a single classified transaction signal.
// Contains ONLY abstract category - no raw data.
type TransactionSignal struct {
	// Category is the abstract commerce category.
	Category commerceobserver.CategoryBucket

	// ConfidenceLevel indicates classification confidence.
	// high = provider category matches known pattern
	// medium = MCC code matches known pattern
	// low = payment channel inference only
	ConfidenceLevel ConfidenceLevel

	// EvidenceHash is computed from abstract tokens only.
	EvidenceHash string
}

// ConfidenceLevel indicates how confident we are in the classification.
type ConfidenceLevel string

const (
	// ConfidenceHigh means provider category matched exactly.
	ConfidenceHigh ConfidenceLevel = "high"
	// ConfidenceMedium means MCC code matched known pattern.
	ConfidenceMedium ConfidenceLevel = "medium"
	// ConfidenceLow means inference from payment channel only.
	ConfidenceLow ConfidenceLevel = "low"
)

// AllConfidenceLevels returns all confidence levels in deterministic order.
func AllConfidenceLevels() []ConfidenceLevel {
	return []ConfidenceLevel{
		ConfidenceHigh,
		ConfidenceMedium,
		ConfidenceLow,
	}
}

// Validate checks if the confidence level is valid.
func (c ConfidenceLevel) Validate() error {
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return nil
	default:
		return fmt.Errorf("invalid confidence level: %s", c)
	}
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (s *TransactionSignal) CanonicalString() string {
	return fmt.Sprintf("TX_SIGNAL|v1|%s|%s|%s",
		s.Category, s.ConfidenceLevel, s.EvidenceHash)
}

// TransactionScanResult contains the classification results for a single transaction.
type TransactionScanResult struct {
	// TransactionIDHash is a hash of the transaction ID.
	TransactionIDHash string

	// IsClassified indicates if the transaction could be classified.
	IsClassified bool

	// Signal contains the classification result (if IsClassified is true).
	Signal *TransactionSignal
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (r *TransactionScanResult) CanonicalString() string {
	classifiedStr := "false"
	if r.IsClassified {
		classifiedStr = "true"
	}
	signalStr := ""
	if r.Signal != nil {
		signalStr = r.Signal.CanonicalString()
	}
	return fmt.Sprintf("TX_SCAN_RESULT|v1|%s|%s|%s",
		r.TransactionIDHash, classifiedStr, signalStr)
}

// FinanceIngestInput contains all inputs for commerce observation building.
type FinanceIngestInput struct {
	// CircleID identifies the circle.
	CircleID string

	// Period is the observation period (e.g., "2024-W03").
	Period string

	// SyncReceiptHash is the hash of the sync receipt.
	SyncReceiptHash string

	// ScanResults contains all transaction classification results.
	ScanResults []TransactionScanResult
}

// Validate checks if the input is valid.
func (in *FinanceIngestInput) Validate() error {
	if in.CircleID == "" {
		return fmt.Errorf("missing circle_id")
	}
	if in.Period == "" {
		return fmt.Errorf("missing period")
	}
	if in.SyncReceiptHash == "" {
		return fmt.Errorf("missing sync_receipt_hash")
	}
	return nil
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (in *FinanceIngestInput) CanonicalString() string {
	var b strings.Builder
	b.WriteString("FINANCE_INGEST_INPUT|v1|")
	b.WriteString(in.CircleID)
	b.WriteString("|")
	b.WriteString(in.Period)
	b.WriteString("|")
	b.WriteString(in.SyncReceiptHash)
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%d", len(in.ScanResults)))

	return b.String()
}

// FinanceIngestResult contains the commerce observations built from transactions.
type FinanceIngestResult struct {
	// Observations contains the abstract commerce observations.
	Observations []commerceobserver.CommerceObservation

	// OverallMagnitude is the abstract magnitude of all classified transactions.
	OverallMagnitude MagnitudeBucket

	// StatusHash is a deterministic hash of the result.
	StatusHash string
}

// MagnitudeBucket represents abstract quantity.
type MagnitudeBucket string

const (
	// MagnitudeNothing means no transactions classified (0).
	MagnitudeNothing MagnitudeBucket = "nothing"
	// MagnitudeAFew means a few transactions classified (1-5).
	MagnitudeAFew MagnitudeBucket = "a_few"
	// MagnitudeSeveral means several transactions classified (6+).
	MagnitudeSeveral MagnitudeBucket = "several"
)

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

// ToMagnitudeBucket converts a raw count to a magnitude bucket.
func ToMagnitudeBucket(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 5:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (r *FinanceIngestResult) CanonicalString() string {
	var b strings.Builder
	b.WriteString("FINANCE_INGEST_RESULT|v1|")
	b.WriteString(string(r.OverallMagnitude))
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("%d", len(r.Observations)))

	// Include sorted observation hashes for determinism
	if len(r.Observations) > 0 {
		hashes := make([]string, len(r.Observations))
		for i, obs := range r.Observations {
			hashes[i] = obs.ComputeHash()
		}
		sort.Strings(hashes)
		for _, h := range hashes {
			b.WriteString("|")
			b.WriteString(h)
		}
	}

	return b.String()
}

// ComputeHash computes a deterministic hash of the result.
func (r *FinanceIngestResult) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// HashTransactionID hashes a transaction ID for storage.
// Raw transaction IDs are NEVER stored.
func HashTransactionID(txID string) string {
	h := sha256.Sum256([]byte("TX_ID|v1|" + txID))
	return hex.EncodeToString(h[:16])
}
