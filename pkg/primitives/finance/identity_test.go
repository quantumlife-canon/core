package finance

import (
	"testing"
	"time"
)

// TestCanonicalTransactionID_Determinism verifies that the same input
// always produces the same canonical ID.
func TestCanonicalTransactionID_Determinism(t *testing.T) {
	input := TransactionIdentityInput{
		Provider:              "plaid",
		ProviderAccountID:     "acc123",
		ProviderTransactionID: "txn456",
		Date:                  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		AmountMinorUnits:      -5000, // $50.00 debit
		Currency:              "USD",
		MerchantNormalized:    "amazon",
	}

	// Compute twice
	id1 := CanonicalTransactionID(input)
	id2 := CanonicalTransactionID(input)

	if id1 != id2 {
		t.Errorf("canonical ID not deterministic: %s != %s", id1, id2)
	}

	// Verify prefix
	if id1[:4] != "ctx_" {
		t.Errorf("expected ctx_ prefix, got %s", id1[:4])
	}
}

// TestCanonicalTransactionID_DifferentInputsDifferentIDs verifies that
// different inputs produce different canonical IDs.
func TestCanonicalTransactionID_DifferentInputsDifferentIDs(t *testing.T) {
	base := TransactionIdentityInput{
		Provider:              "plaid",
		ProviderAccountID:     "acc123",
		ProviderTransactionID: "txn456",
		Date:                  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		AmountMinorUnits:      -5000,
		Currency:              "USD",
		MerchantNormalized:    "amazon",
	}

	// Same input
	id1 := CanonicalTransactionID(base)

	// Different amount
	diff := base
	diff.AmountMinorUnits = -5001
	id2 := CanonicalTransactionID(diff)

	if id1 == id2 {
		t.Error("different amounts should produce different IDs")
	}

	// Different date
	diff = base
	diff.Date = time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	id3 := CanonicalTransactionID(diff)

	if id1 == id3 {
		t.Error("different dates should produce different IDs")
	}

	// Different provider
	diff = base
	diff.Provider = "truelayer"
	id4 := CanonicalTransactionID(diff)

	if id1 == id4 {
		t.Error("different providers should produce different IDs")
	}
}

// TestCanonicalTransactionID_DirectionInferred verifies that direction
// is correctly inferred from amount sign.
func TestCanonicalTransactionID_DirectionInferred(t *testing.T) {
	// Same values but different amount signs
	debit := TransactionIdentityInput{
		Provider:              "plaid",
		ProviderAccountID:     "acc123",
		ProviderTransactionID: "txn456",
		Date:                  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		AmountMinorUnits:      -5000, // Debit (expense)
		Currency:              "USD",
	}

	credit := TransactionIdentityInput{
		Provider:              "plaid",
		ProviderAccountID:     "acc123",
		ProviderTransactionID: "txn456",
		Date:                  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		AmountMinorUnits:      5000, // Credit (income)
		Currency:              "USD",
	}

	debitID := CanonicalTransactionID(debit)
	creditID := CanonicalTransactionID(credit)

	if debitID == creditID {
		t.Error("debit and credit with same absolute amount should have different IDs")
	}
}

// TestCanonicalAccountID_Determinism verifies deterministic account IDs.
func TestCanonicalAccountID_Determinism(t *testing.T) {
	input := AccountIdentityInput{
		Provider:          "plaid",
		ProviderAccountID: "acc123",
		AccountType:       AccountTypeDepository,
		Currency:          "USD",
		Mask:              "1234",
	}

	id1 := CanonicalAccountID(input)
	id2 := CanonicalAccountID(input)

	if id1 != id2 {
		t.Errorf("canonical account ID not deterministic: %s != %s", id1, id2)
	}

	if id1[:4] != "cac_" {
		t.Errorf("expected cac_ prefix, got %s", id1[:4])
	}
}

// TestTransactionMatchKey_PendingPostedMatching verifies that pending and
// posted transactions with same economic details produce same match key.
func TestTransactionMatchKey_PendingPostedMatching(t *testing.T) {
	// Pending transaction
	pending := TransactionMatchInput{
		CanonicalAccountID: "cac_123",
		AmountMinorUnits:   -2500, // $25.00 debit
		Currency:           "USD",
		MerchantNormalized: "starbucks",
	}

	// Posted transaction (same economic event)
	posted := TransactionMatchInput{
		CanonicalAccountID: "cac_123",
		AmountMinorUnits:   -2500,
		Currency:           "USD",
		MerchantNormalized: "starbucks",
	}

	pendingKey := TransactionMatchKey(pending)
	postedKey := TransactionMatchKey(posted)

	if pendingKey != postedKey {
		t.Error("pending and posted with same details should have same match key")
	}

	if pendingKey[:4] != "tmk_" {
		t.Errorf("expected tmk_ prefix, got %s", pendingKey[:4])
	}
}

// TestTransactionMatchKey_DifferentMerchantsDifferentKeys verifies that
// different merchants produce different match keys.
func TestTransactionMatchKey_DifferentMerchantsDifferentKeys(t *testing.T) {
	base := TransactionMatchInput{
		CanonicalAccountID: "cac_123",
		AmountMinorUnits:   -2500,
		Currency:           "USD",
		MerchantNormalized: "starbucks",
	}

	diff := TransactionMatchInput{
		CanonicalAccountID: "cac_123",
		AmountMinorUnits:   -2500,
		Currency:           "USD",
		MerchantNormalized: "dunkin",
	}

	key1 := TransactionMatchKey(base)
	key2 := TransactionMatchKey(diff)

	if key1 == key2 {
		t.Error("different merchants should produce different match keys")
	}
}

// TestNormalizeMerchant_Consistency verifies merchant normalization.
func TestNormalizeMerchant_Consistency(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AMAZON.COM", "amazon"},            // Dots removed, alias applied
		{"Amazon Inc.", "amazon"},           // Inc suffix removed
		{"STARBUCKS CORP", "starbucks"},     // Corp suffix removed
		{"  Trader Joe's  ", "trader joes"}, // Apostrophe removed, alias applied
		{"Whole Foods LLC", "whole foods"},  // LLC suffix removed, alias applied
		{"", ""},
	}

	for _, tt := range tests {
		result := NormalizeMerchant(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeMerchant(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestNormalizeMerchant_NoiseTokens verifies noise token removal.
func TestNormalizeMerchant_NoiseTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"POS DEBIT STARBUCKS", "starbucks"}, // POS DEBIT prefix removed
		{"CARD PURCHASE AMAZON", "amazon"},   // Card prefix removed
		{"CONTACTLESS TARGET", "target"},     // Contactless removed
		{"SQ *COFFEE SHOP", "coffee shop"},   // Square prefix stripped
		{"TST* RESTAURANT", "restaurant"},    // Toast prefix stripped
		{"UBER *TRIP", "uber"},               // Asterisk -> space, "uber trip" -> alias
		{"AMAZON*DIGITAL", "amazon"},         // Asterisk -> space, "amazon digital" -> alias
	}

	for _, tt := range tests {
		result := NormalizeMerchant(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeMerchant(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestNormalizeMerchant_StoreNumbers verifies trailing store number removal.
func TestNormalizeMerchant_StoreNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"STARBUCKS 12345", "starbucks"},        // Pure numeric store number
		{"TARGET T1234", "target"},              // Alphanumeric store code
		{"WALMART SUPERCENTER 5432", "walmart"}, // Long store name + number, alias
		{"CVS/PHARMACY 00123", "cvs"},           // Pharmacy with store number, alias
		{"SHELL 7890", "shell"},                 // Gas station with number
		{"7-ELEVEN", "7eleven"},                 // Number is part of brand name (kept)
	}

	for _, tt := range tests {
		result := NormalizeMerchant(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeMerchant(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestNormalizeMerchant_Aliases verifies alias map lookups.
func TestNormalizeMerchant_Aliases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AMZN", "amazon"},
		{"AMZN MKTP", "amazon"},
		{"SBUX", "starbucks"},
		{"WM SUPERCENTER", "walmart"},
		{"WAL-MART", "walmart"},
		{"UBER TRIP", "uber"},
		{"UBER EATS", "uber eats"}, // Kept separate from Uber
		{"DD DOORDASH", "doordash"},
		{"MSFT", "microsoft"},
		{"EXXONMOBIL", "exxon"},
		{"MOBIL", "exxon"},
	}

	for _, tt := range tests {
		result := NormalizeMerchant(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeMerchant(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestNormalizeMerchant_Determinism verifies deterministic normalization.
func TestNormalizeMerchant_Determinism(t *testing.T) {
	inputs := []string{
		"AMAZON.COM",
		"Amazon Inc.",
		"amazon",
		"AMAZON INC",
	}

	// All should normalize to the same value
	var normalized []string
	for _, input := range inputs {
		normalized = append(normalized, NormalizeMerchant(input))
	}

	// Verify determinism (same input = same output)
	for i := 0; i < len(inputs); i++ {
		result := NormalizeMerchant(inputs[i])
		if result != normalized[i] {
			t.Errorf("non-deterministic normalization for %q", inputs[i])
		}
	}
}

// TestCanonicalTransactionID_CrossProviderSameTransaction verifies that
// the same economic transaction from different providers gets different
// canonical IDs (by design - they are different reports of the same event).
func TestCanonicalTransactionID_CrossProviderSameTransaction(t *testing.T) {
	// Same transaction reported by Plaid
	plaid := TransactionIdentityInput{
		Provider:              "plaid",
		ProviderAccountID:     "plaid_acc_123",
		ProviderTransactionID: "plaid_txn_456",
		Date:                  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		AmountMinorUnits:      -5000,
		Currency:              "USD",
		MerchantNormalized:    "amazon",
	}

	// Same transaction reported by TrueLayer
	truelayer := TransactionIdentityInput{
		Provider:              "truelayer",
		ProviderAccountID:     "tl_acc_789",
		ProviderTransactionID: "tl_txn_012",
		Date:                  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		AmountMinorUnits:      -5000,
		Currency:              "USD",
		MerchantNormalized:    "amazon",
	}

	plaidID := CanonicalTransactionID(plaid)
	truelayerID := CanonicalTransactionID(truelayer)

	// These should be different because they come from different providers
	// with different provider transaction IDs
	if plaidID == truelayerID {
		t.Error("same transaction from different providers should have different canonical IDs")
	}

	// But match keys can be used to detect they're the same economic event
	plaidMatch := TransactionMatchKey(TransactionMatchInput{
		CanonicalAccountID: "shared_account", // Mapped to same canonical account
		AmountMinorUnits:   -5000,
		Currency:           "USD",
		MerchantNormalized: "amazon",
	})

	truelayerMatch := TransactionMatchKey(TransactionMatchInput{
		CanonicalAccountID: "shared_account",
		AmountMinorUnits:   -5000,
		Currency:           "USD",
		MerchantNormalized: "amazon",
	})

	if plaidMatch != truelayerMatch {
		t.Error("same economic event should have same match key for cross-provider deduplication")
	}
}
