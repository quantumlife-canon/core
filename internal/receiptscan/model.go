// Package receiptscan provides deterministic receipt classification for Gmail messages.
//
// Phase 31.1: Gmail Receipt Observers (Email -> CommerceSignals)
// Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md
//
// CRITICAL INVARIANTS:
//   - NO merchant names stored
//   - NO amounts stored
//   - NO sender emails stored
//   - NO subjects stored
//   - Only abstract category buckets + horizon buckets + evidence hashes
//   - Deterministic: same inputs => same outputs
//   - stdlib only, no goroutines, no time.Now()
//
// This package scans email metadata to detect receipt-like patterns
// and assigns abstract commerce categories WITHOUT leaking identifiers.
package receiptscan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ReceiptCategory represents an abstract commerce category.
// Derived from email patterns but NEVER stores vendor names.
type ReceiptCategory string

const (
	// CategoryDelivery represents food/grocery delivery services.
	CategoryDelivery ReceiptCategory = "delivery"
	// CategoryTransport represents transportation services.
	CategoryTransport ReceiptCategory = "transport"
	// CategoryRetail represents retail purchases.
	CategoryRetail ReceiptCategory = "retail"
	// CategorySubscription represents recurring subscriptions.
	CategorySubscription ReceiptCategory = "subscription"
	// CategoryBills represents utility/service bills.
	CategoryBills ReceiptCategory = "bills"
	// CategoryTravel represents travel bookings.
	CategoryTravel ReceiptCategory = "travel"
	// CategoryOther represents uncategorized receipts.
	CategoryOther ReceiptCategory = "other"
)

// AllReceiptCategories returns all categories in deterministic order.
func AllReceiptCategories() []ReceiptCategory {
	return []ReceiptCategory{
		CategoryDelivery,
		CategoryTransport,
		CategoryRetail,
		CategorySubscription,
		CategoryBills,
		CategoryTravel,
		CategoryOther,
	}
}

// Validate checks if the category is valid.
func (c ReceiptCategory) Validate() error {
	switch c {
	case CategoryDelivery, CategoryTransport, CategoryRetail,
		CategorySubscription, CategoryBills, CategoryTravel, CategoryOther:
		return nil
	default:
		return fmt.Errorf("invalid receipt category: %s", c)
	}
}

// HorizonBucket represents when something is relevant.
// Derived from email content but NEVER stores raw text.
type HorizonBucket string

const (
	// HorizonNow indicates immediate relevance (delivered, ready, arrived).
	HorizonNow HorizonBucket = "now"
	// HorizonSoon indicates near-future relevance (on the way, dispatched).
	HorizonSoon HorizonBucket = "soon"
	// HorizonLater indicates future relevance (scheduled, renewal, statement).
	HorizonLater HorizonBucket = "later"
)

// AllHorizonBuckets returns all horizon buckets in deterministic order.
func AllHorizonBuckets() []HorizonBucket {
	return []HorizonBucket{
		HorizonNow,
		HorizonSoon,
		HorizonLater,
	}
}

// Validate checks if the horizon bucket is valid.
func (h HorizonBucket) Validate() error {
	switch h {
	case HorizonNow, HorizonSoon, HorizonLater:
		return nil
	default:
		return fmt.Errorf("invalid horizon bucket: %s", h)
	}
}

// Priority returns the priority of the horizon (lower = more urgent).
func (h HorizonBucket) Priority() int {
	switch h {
	case HorizonNow:
		return 0
	case HorizonSoon:
		return 1
	case HorizonLater:
		return 2
	default:
		return 3
	}
}

// ReceiptSignal represents a detected receipt pattern.
// Contains ONLY abstract buckets and hashes - never raw data.
type ReceiptSignal struct {
	// Category is the abstract commerce category.
	Category ReceiptCategory

	// Horizon indicates when this is relevant.
	Horizon HorizonBucket

	// EvidenceHash is SHA256 of abstract classification tokens only.
	// Never contains raw subject, sender, or amounts.
	EvidenceHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (s *ReceiptSignal) CanonicalString() string {
	return fmt.Sprintf("RECEIPT_SIGNAL|v1|%s|%s|%s",
		s.Category, s.Horizon, s.EvidenceHash)
}

// ComputeHash computes a deterministic hash of the signal.
func (s *ReceiptSignal) ComputeHash() string {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the signal is valid.
func (s *ReceiptSignal) Validate() error {
	if err := s.Category.Validate(); err != nil {
		return err
	}
	if err := s.Horizon.Validate(); err != nil {
		return err
	}
	if s.EvidenceHash == "" {
		return fmt.Errorf("missing evidence_hash")
	}
	return nil
}

// ReceiptScanInput represents the input for receipt classification.
// These fields are used for classification but NEVER persisted.
//
// CRITICAL: FromDomain, Subject, and Snippet are used only during
// in-memory processing and are immediately discarded after classification.
type ReceiptScanInput struct {
	// CircleID identifies the circle (used in hash computation).
	CircleID string

	// MessageIDHash is SHA256 of the message ID (never store raw ID).
	MessageIDHash string

	// FromDomain is the sender domain ONLY (not full email).
	// Used for classification, NOT persisted.
	FromDomain string

	// Subject is the email subject.
	// Used for classification, NOT persisted.
	Subject string

	// Snippet is the email body preview.
	// Used for classification, NOT persisted.
	Snippet string
}

// Validate checks if the input is valid.
func (i *ReceiptScanInput) Validate() error {
	if i.CircleID == "" {
		return fmt.Errorf("missing circle_id")
	}
	if i.MessageIDHash == "" {
		return fmt.Errorf("missing message_id_hash")
	}
	return nil
}

// ReceiptScanResult represents the classification result.
// Contains ONLY abstract signals - never raw data.
type ReceiptScanResult struct {
	// IsReceipt indicates if the message appears to be a receipt.
	IsReceipt bool

	// Signals contains detected receipt signals (0-1 for now, extensible).
	Signals []ReceiptSignal

	// ResultHash is a deterministic hash of the entire result.
	ResultHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (r *ReceiptScanResult) CanonicalString() string {
	var b strings.Builder
	b.WriteString("RECEIPT_SCAN_RESULT|v1|")
	if r.IsReceipt {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	for _, sig := range r.Signals {
		b.WriteString("|")
		b.WriteString(string(sig.Category))
		b.WriteString(":")
		b.WriteString(string(sig.Horizon))
		b.WriteString(":")
		b.WriteString(sig.EvidenceHash)
	}
	return b.String()
}

// ComputeHash computes a deterministic hash of the result.
func (r *ReceiptScanResult) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the result is valid.
func (r *ReceiptScanResult) Validate() error {
	for i, sig := range r.Signals {
		if err := sig.Validate(); err != nil {
			return fmt.Errorf("signal %d: %w", i, err)
		}
	}
	if r.IsReceipt && len(r.Signals) == 0 {
		return fmt.Errorf("receipt detected but no signals")
	}
	return nil
}

// HashMessageID computes a SHA256 hash of a message ID.
// Used to ensure we never store raw message IDs.
func HashMessageID(messageID string) string {
	h := sha256.Sum256([]byte("MSG_ID|v1|" + messageID))
	return hex.EncodeToString(h[:16])
}

// ComputeEvidenceHash computes a deterministic hash from classification tokens.
// CRITICAL: Tokens must be abstract (category names, bucket names) - never raw text.
func ComputeEvidenceHash(tokens []string) string {
	if len(tokens) == 0 {
		return "empty"
	}

	var b strings.Builder
	b.WriteString("RECEIPT_EVIDENCE|v1")
	for _, t := range tokens {
		b.WriteString("|")
		b.WriteString(t)
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:16])
}
