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
//
// v8.5: Enhanced normalization with:
// - Noise token removal (POS, CARD, CONTACTLESS, etc.)
// - Trailing store number stripping
// - Alias map lookup for common merchant variants
func NormalizeMerchant(name string) string {
	if name == "" {
		return ""
	}

	// Step 1: Lowercase and trim
	name = strings.ToLower(strings.TrimSpace(name))

	// Step 2: Remove noise tokens (common POS/card prefixes)
	// Order matters - longer prefixes first
	noiseTokens := []string{
		"pos debit purchase ",
		"pos debit ",
		"pos purchase ",
		"pos ",
		"debit card purchase ",
		"debit card ",
		"credit card purchase ",
		"credit card ",
		"card purchase ",
		"card ",
		"contactless ",
		"tap ",
		"chip ",
		"purchase ",
		"payment ",
		"debit ",
		"chk ",
		"ach ",
	}
	for _, noise := range noiseTokens {
		if strings.HasPrefix(name, noise) {
			name = strings.TrimPrefix(name, noise)
			break // Only remove one prefix
		}
	}

	// Step 3: Handle asterisk-delimited formats like "SQ *MERCHANT" or "TST* MERCHANT"
	// Only strip if prefix is a known short code (2-4 chars)
	if idx := strings.Index(name, "*"); idx != -1 && idx >= 2 && idx <= 5 {
		prefix := strings.TrimSpace(name[:idx])
		// Only strip known POS prefixes
		if isKnownPOSPrefix(prefix) {
			name = strings.TrimSpace(name[idx+1:])
		}
	}

	// Step 4: Replace punctuation with spaces (keep alphanumeric)
	var cleaned strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cleaned.WriteRune(r)
		} else if r == ' ' || r == '/' || r == '-' || r == '*' {
			cleaned.WriteRune(' ')
		}
	}
	name = cleaned.String()

	// Step 5: Remove common business suffixes
	suffixes := []string{
		" inc", " llc", " ltd", " limited", " corp", " corporation",
		" co", " company", " plc", " uk", " us", " usa",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
		}
	}

	// Step 6: Strip trailing store/location numbers (e.g., "starbucks 12345")
	name = stripTrailingNumbers(name)

	// Step 7: Collapse spaces and trim
	name = strings.Join(strings.Fields(name), " ")

	// Step 8: Check alias map for canonical form
	if alias, ok := merchantAliases[name]; ok {
		return alias
	}

	return name
}

// isKnownPOSPrefix returns true if the prefix is a known POS system code.
func isKnownPOSPrefix(prefix string) bool {
	knownPrefixes := map[string]bool{
		"sq":  true, // Square
		"tst": true, // Toast
		"chk": true, // Check
		"pp":  true, // PayPal
	}
	return knownPrefixes[prefix]
}

// stripTrailingNumbers removes trailing numeric store/location identifiers.
// Examples: "starbucks 12345" -> "starbucks", "target t1234" -> "target"
func stripTrailingNumbers(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}

	// Check if last word is purely numeric or short alphanumeric store code
	last := words[len(words)-1]
	if isStoreNumber(last) && len(words) > 1 {
		return strings.Join(words[:len(words)-1], " ")
	}

	return s
}

// isStoreNumber returns true if the string looks like a store number.
// Matches: pure digits, or short alphanumeric codes starting with # or letter.
func isStoreNumber(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Pure numeric (any length)
	allDigits := true
	for _, r := range s {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(s) >= 2 {
		return true
	}

	// Short alphanumeric codes (3-7 chars, at least 2 digits)
	if len(s) >= 3 && len(s) <= 7 {
		digitCount := 0
		for _, r := range s {
			if r >= '0' && r <= '9' {
				digitCount++
			}
		}
		if digitCount >= 2 {
			return true
		}
	}

	return false
}

// merchantAliases maps normalized merchant variants to canonical names.
// This map is deterministic and does not require external deps at runtime.
// Source: configs/merchant_aliases.json (embedded at build time conceptually)
var merchantAliases = map[string]string{
	// Amazon variants
	"amzn":               "amazon",
	"amazoncom":          "amazon", // dots removed without space
	"amazon com":         "amazon",
	"amazon digital":     "amazon",
	"amazon prime":       "amazon",
	"amazon marketplace": "amazon",
	"amzn mktp":          "amazon",
	"amzn digital":       "amazon",

	// Uber variants
	"uber trip": "uber",
	"uber one":  "uber",
	"uber bv":   "uber",

	// Uber Eats (kept separate)
	"uber eats": "uber eats",

	// Lyft variants
	"lyft ride": "lyft",
	"lyft inc":  "lyft",

	// Food delivery
	"dd doordash":     "doordash",
	"doordash dasher": "doordash",
	"grubhub":         "grubhub",
	"grubhub holding": "grubhub",

	// Coffee
	"sbux":            "starbucks",
	"starbux":         "starbucks",
	"starbucks store": "starbucks",

	// Grocery/retail
	"wm supercenter":      "walmart",
	"wal mart":            "walmart",
	"walmart supercenter": "walmart",
	"walmart grocery":     "walmart",
	"target stores":       "target",
	"target com":          "target",
	"costco whse":         "costco",
	"costco wholesale":    "costco",
	"cvs":                 "cvs",
	"cvs pharmacy":        "cvs",
	"cvs store":           "cvs",
	"walgreen":            "walgreens",
	"walgreens store":     "walgreens",

	// Fast food
	"mcdonalds":   "mcdonalds",
	"mcdonald s":  "mcdonalds",
	"chick fil a": "chick fil a",
	"chickfila":   "chick fil a",

	// Streaming/digital
	"netflix com": "netflix",
	"netflix inc": "netflix",
	"spotify usa": "spotify",
	"spotify ab":  "spotify",
	"apple com":   "apple",
	"apple store": "apple",
	"apple itune": "apple",
	"google play": "google",
	"google one":  "google",
	"google clou": "google",
	"msft":        "microsoft",

	// Payments
	"paypal":           "paypal",
	"paypal inst xfer": "paypal",
	"venmo":            "venmo",
	"venmo payment":    "venmo",
	"zelle":            "zelle",
	"zelle payment":    "zelle",

	// Gas stations / convenience
	"shell oil":     "shell",
	"shell service": "shell",
	"chevron":       "chevron",
	"bp gas":        "bp",
	"bp amoco":      "bp",
	"exxonmobil":    "exxon",
	"exxon":         "exxon",
	"mobil":         "exxon",
	"7 eleven":      "7eleven",
	"7eleven":       "7eleven",

	// Shipping
	"usps":                "usps",
	"usps po":             "usps",
	"united states posta": "usps",
	"fedex":               "fedex",
	"fedex office":        "fedex",
	"ups store":           "ups",
	"ups freight":         "ups",

	// Grocery chains
	"kroger":            "kroger",
	"kroger fuel":       "kroger",
	"safeway":           "safeway",
	"safeway store":     "safeway",
	"whole foods":       "whole foods",
	"whole foods marke": "whole foods",
	"trader joes":       "trader joes",
	"trader joe s":      "trader joes",

	// Retail
	"tjx":            "tj maxx",
	"tjmaxx":         "tj maxx",
	"homegoods":      "home goods",
	"home goods":     "home goods",
	"marshalls":      "marshalls",
	"ross stores":    "ross",
	"ross dress":     "ross",
	"nordstrom":      "nordstrom",
	"nordstrom rack": "nordstrom",
	"macys":          "macys",
	"macy s":         "macys",
	"kohls":          "kohls",
	"kohl s":         "kohls",
	"jcpenney":       "jcpenney",
	"jc penney":      "jcpenney",
	"bestbuy":        "best buy",
	"best buy":       "best buy",
	"homedepot":      "home depot",
	"home depot":     "home depot",
	"lowes":          "lowes",
	"lowe s":         "lowes",
}
