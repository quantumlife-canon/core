// Package finance provides canonical identity computation for v8.4 reconciliation.
//
// CRITICAL: These are deterministic identity algorithms. No randomness allowed.
// Canonical IDs enable cross-window deduplication and pending→posted merging.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package finance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// CanonicalTransactionID computes a stable, deterministic transaction ID.
//
// The ID is computed from immutable transaction properties:
// - Provider identifier
// - Provider's account ID
// - Provider's transaction ID (if present)
// - Posted date (YYYY-MM-DD format)
// - Amount in minor units (cents)
// - Currency code (ISO 4217)
// - Normalized merchant/payee
// - Direction (debit/credit)
//
// This ensures the same economic event produces the same canonical ID
// regardless of when or how often it is synced.
func CanonicalTransactionID(input TransactionIdentityInput) string {
	// Build deterministic input string
	var parts []string

	// Provider context
	parts = append(parts, "provider:"+normalizeForHash(input.Provider))
	parts = append(parts, "account:"+normalizeForHash(input.ProviderAccountID))

	// Provider's native transaction ID (primary key when present)
	if input.ProviderTransactionID != "" {
		parts = append(parts, "txn_id:"+normalizeForHash(input.ProviderTransactionID))
	}

	// Date (use posted date if available, else transaction date)
	dateStr := input.Date.UTC().Format("2006-01-02")
	parts = append(parts, "date:"+dateStr)

	// Amount (exact match in minor units)
	parts = append(parts, fmt.Sprintf("amount:%d", input.AmountMinorUnits))

	// Currency (uppercase ISO code)
	parts = append(parts, "currency:"+strings.ToUpper(input.Currency))

	// Normalized merchant/payee (for matching when txn_id missing)
	if input.MerchantNormalized != "" {
		parts = append(parts, "merchant:"+normalizeForHash(input.MerchantNormalized))
	}

	// Direction
	direction := "credit"
	if input.AmountMinorUnits < 0 {
		direction = "debit"
	}
	parts = append(parts, "direction:"+direction)

	// Compute SHA256 hash
	hashInput := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(hashInput))

	return "ctx_" + hex.EncodeToString(hash[:])
}

// TransactionIdentityInput contains fields for canonical transaction ID computation.
type TransactionIdentityInput struct {
	// Provider is the source provider (e.g., "plaid", "truelayer", "mock").
	Provider string

	// ProviderAccountID is the provider's account identifier.
	ProviderAccountID string

	// ProviderTransactionID is the provider's transaction identifier (may be empty).
	ProviderTransactionID string

	// Date is the transaction date (or posted date if available).
	Date time.Time

	// AmountMinorUnits is the amount in cents/pence (int64).
	AmountMinorUnits int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// MerchantNormalized is the normalized merchant/payee name.
	MerchantNormalized string
}

// CanonicalAccountID computes a stable, deterministic account ID.
//
// The ID is computed from:
// - Provider identifier
// - Provider's account ID
// - Account type (normalized)
// - Currency (for multi-currency differentiation)
// - Mask (last 4 digits, for disambiguation)
func CanonicalAccountID(input AccountIdentityInput) string {
	var parts []string

	// Provider context
	parts = append(parts, "provider:"+normalizeForHash(input.Provider))
	parts = append(parts, "account:"+normalizeForHash(input.ProviderAccountID))

	// Account type (normalized)
	parts = append(parts, "type:"+normalizeForHash(string(input.AccountType)))

	// Currency
	parts = append(parts, "currency:"+strings.ToUpper(input.Currency))

	// Mask (if present, for disambiguation of multiple accounts)
	if input.Mask != "" {
		parts = append(parts, "mask:"+input.Mask)
	}

	// Compute SHA256 hash
	hashInput := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(hashInput))

	return "cac_" + hex.EncodeToString(hash[:])
}

// AccountIdentityInput contains fields for canonical account ID computation.
type AccountIdentityInput struct {
	// Provider is the source provider.
	Provider string

	// ProviderAccountID is the provider's account identifier.
	ProviderAccountID string

	// AccountType is the normalized account type.
	AccountType NormalizedAccountType

	// Currency is the ISO 4217 currency code.
	Currency string

	// Mask is the last 4 digits of the account number.
	Mask string
}

// TransactionMatchKey computes a key for pending→posted matching.
//
// This key is used to find transactions that may represent the same
// economic event when provider transaction IDs differ between pending
// and posted states.
//
// Match key components:
// - Canonical account ID
// - Amount in minor units (exact)
// - Currency
// - Direction
// - Normalized merchant (if present)
//
// Date is NOT included because pending→posted may have different dates.
func TransactionMatchKey(input TransactionMatchInput) string {
	var parts []string

	// Account
	parts = append(parts, "account:"+normalizeForHash(input.CanonicalAccountID))

	// Amount (exact)
	parts = append(parts, fmt.Sprintf("amount:%d", input.AmountMinorUnits))

	// Currency
	parts = append(parts, "currency:"+strings.ToUpper(input.Currency))

	// Direction
	direction := "credit"
	if input.AmountMinorUnits < 0 {
		direction = "debit"
	}
	parts = append(parts, "direction:"+direction)

	// Merchant (optional - both sides must have same value or one empty)
	if input.MerchantNormalized != "" {
		parts = append(parts, "merchant:"+normalizeForHash(input.MerchantNormalized))
	}

	hashInput := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(hashInput))

	return "tmk_" + hex.EncodeToString(hash[:16]) // Shorter for match keys
}

// TransactionMatchInput contains fields for match key computation.
type TransactionMatchInput struct {
	// CanonicalAccountID is the computed canonical account ID.
	CanonicalAccountID string

	// AmountMinorUnits is the amount in cents/pence.
	AmountMinorUnits int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// MerchantNormalized is the normalized merchant name.
	MerchantNormalized string
}

// normalizeForHash normalizes a string for hash input.
// - Trims whitespace
// - Converts to lowercase
// - Removes non-alphanumeric characters except spaces
func normalizeForHash(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	// Keep alphanumeric and spaces only
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		}
	}

	// Collapse multiple spaces to single space
	return strings.Join(strings.Fields(result.String()), " ")
}

// NormalizeMerchant normalizes a merchant name for consistent matching.
func NormalizeMerchant(name string) string {
	if name == "" {
		return ""
	}

	// Lowercase and trim
	name = strings.ToLower(strings.TrimSpace(name))

	// Remove common suffixes that vary
	suffixes := []string{
		" inc", " inc.", " llc", " ltd", " ltd.", " corp", " corp.",
		" co", " co.", " company",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
		}
	}

	// Remove punctuation
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		}
	}

	// Collapse spaces and trim
	return strings.Join(strings.Fields(result.String()), " ")
}
