package reconcile

import (
	"testing"
	"time"

	"quantumlife/internal/finance/adjustments"
	"quantumlife/internal/finance/normalize"
	"quantumlife/internal/finance/visibility"
	"quantumlife/pkg/primitives/finance"
)

// ============================================================
// v8.5 Edge Case Tests - Comprehensive Coverage
// ============================================================

// --- Refund/Reversal/Chargeback Classification Tests ---

func TestEdgeCase_RefundDoesNotInflateSpend(t *testing.T) {
	// Scenario: $100 purchase followed by $30 partial refund
	// Effective spend should be $70, not $100

	transactions := []finance.TransactionRecord{
		{
			RecordID:    "purchase_1",
			AmountCents: -10000, // $100 purchase
			Currency:    "USD",
			Category:    "shopping",
			Kind:        finance.KindPurchase,
		},
		{
			RecordID:             "refund_1",
			AmountCents:          3000, // $30 refund
			EffectiveAmountCents: 3000,
			Currency:             "USD",
			Category:             "shopping",
			Kind:                 finance.KindRefund,
			IsAdjustment:         true,
		},
	}

	summary := visibility.ComputeSpendSummary(transactions)

	// Only the purchase counts (refunds are positive, not included in spend)
	if summary.TotalByCurrency["USD"] != -10000 {
		t.Errorf("expected -10000 (purchase only), got %d", summary.TotalByCurrency["USD"])
	}
}

func TestEdgeCase_ReversalClassification(t *testing.T) {
	classifier := adjustments.NewClassifier()

	tx := finance.TransactionRecord{
		Description: "PAYMENT REVERSAL - Transaction cancelled",
		AmountCents: 5000,
	}
	ctx := adjustments.ClassifyContext{
		Provider: "plaid",
	}

	result := classifier.Classify(tx, ctx)

	if result.Kind != finance.KindReversal {
		t.Errorf("expected reversal, got %s", result.Kind)
	}
	if result.Method != "description_pattern" {
		t.Errorf("expected description_pattern, got %s", result.Method)
	}
}

func TestEdgeCase_ChargebackClassification(t *testing.T) {
	classifier := adjustments.NewClassifier()

	tx := finance.TransactionRecord{
		Description: "DISPUTED CHARGEBACK - Fraud claim",
		AmountCents: 5000,
	}
	ctx := adjustments.ClassifyContext{
		Provider: "plaid",
	}

	result := classifier.Classify(tx, ctx)

	if result.Kind != finance.KindChargeback {
		t.Errorf("expected chargeback, got %s", result.Kind)
	}
}

// --- Partial Capture Edge Cases ---

func TestEdgeCase_PartialCapture_Tolerance(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "test",
		TraceID:   "partial_tolerance",
	}

	// Scenario: Gas station pre-auth for $1, final charge $45
	matchKey := finance.TransactionMatchKey(finance.TransactionMatchInput{
		CanonicalAccountID: "cac_credit",
		AmountMinorUnits:   -100, // $1 pre-auth
		Currency:           "USD",
		MerchantNormalized: "shell",
	})

	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				AmountCents: -100, // $1 pre-auth
				Currency:    "USD",
				Pending:     true,
			},
			CanonicalID: "ctx_pending_gas",
			MatchKey:    matchKey,
			IsPending:   true,
		},
		{
			Transaction: finance.TransactionRecord{
				AmountCents: -4500, // $45 final
				Currency:    "USD",
				Pending:     false,
			},
			CanonicalID: "ctx_posted_gas",
			MatchKey:    matchKey,
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Should detect partial capture
	if result.Report.PartialCaptureCount != 1 {
		t.Errorf("expected 1 partial capture, got %d", result.Report.PartialCaptureCount)
	}

	// Should have stored original pending amount
	if len(result.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(result.Transactions))
	}

	if result.Transactions[0].Transaction.PendingAmountCents == nil {
		t.Error("expected PendingAmountCents to be set")
	}
}

// --- Multi-Currency Safety Tests ---

func TestEdgeCase_MultiCurrency_NoSilentAggregation(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{AmountCents: -5000, Currency: "USD"},
		{AmountCents: -3000, Currency: "EUR"},
		{AmountCents: -2000, Currency: "GBP"},
	}

	summary := visibility.ComputeSpendSummary(transactions)

	// Should NOT have a single total
	if summary.IsSingleCurrency {
		t.Error("should detect multi-currency")
	}

	// Should have per-currency totals
	if len(summary.TotalByCurrency) != 3 {
		t.Errorf("expected 3 currencies, got %d", len(summary.TotalByCurrency))
	}

	// Should have inconclusive reason
	if summary.InconclusiveReason == "" {
		t.Error("expected inconclusive reason for multi-currency")
	}
}

func TestEdgeCase_MultiCurrency_ValidationError(t *testing.T) {
	transactions := []finance.TransactionRecord{
		{Currency: "USD"},
		{Currency: "EUR"},
	}

	err := visibility.ValidateSingleCurrency(transactions)
	if err == nil {
		t.Error("expected error for multi-currency validation")
	}
}

// --- Merchant Normalization Edge Cases ---

func TestEdgeCase_MerchantVariants_SameCanonical(t *testing.T) {
	variants := []string{
		"AMAZON.COM",
		"AMZN MKTP US",
		"AMZN Digital",
		"Amazon Prime",
	}

	normalized := make(map[string]bool)
	for _, v := range variants {
		n := finance.NormalizeMerchant(v)
		normalized[n] = true
	}

	// All should normalize to "amazon"
	if len(normalized) != 1 {
		t.Errorf("expected all variants to normalize to same value, got %d unique", len(normalized))
	}

	if !normalized["amazon"] {
		t.Error("expected normalization to 'amazon'")
	}
}

func TestEdgeCase_MerchantStoreNumber_Stripped(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"STARBUCKS 12345", "starbucks"},
		{"TARGET T0123", "target"},
		{"COSTCO WHSE 9999", "costco"},
	}

	for _, tt := range tests {
		result := finance.NormalizeMerchant(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeMerchant(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- Deduplication Edge Cases ---

func TestEdgeCase_TripleSyncOverlap(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "test",
		TraceID:   "triple_overlap",
	}

	// Same transaction appearing in 3 sync windows
	canonicalID := "ctx_coffee_jan15"

	transactions := []normalize.NormalizedTransactionResult{
		{
			Transaction: finance.TransactionRecord{
				Description: "Starbucks (sync 1)",
				AmountCents: -500,
				Currency:    "USD",
			},
			CanonicalID: canonicalID,
			MatchKey:    "tmk_coffee",
			IsPending:   false,
		},
		{
			Transaction: finance.TransactionRecord{
				Description: "Starbucks (sync 2)",
				AmountCents: -500,
				Currency:    "USD",
			},
			CanonicalID: canonicalID,
			MatchKey:    "tmk_coffee",
			IsPending:   false,
		},
		{
			Transaction: finance.TransactionRecord{
				Description: "Starbucks (sync 3)",
				AmountCents: -500,
				Currency:    "USD",
			},
			CanonicalID: canonicalID,
			MatchKey:    "tmk_coffee",
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Should deduplicate to single transaction
	if result.Report.OutputCount != 1 {
		t.Errorf("expected 1 output, got %d", result.Report.OutputCount)
	}

	// Should remove 2 duplicates
	if result.Report.DuplicatesRemoved != 2 {
		t.Errorf("expected 2 duplicates removed, got %d", result.Report.DuplicatesRemoved)
	}
}

// --- Related Transaction Matching Edge Cases ---

func TestEdgeCase_AmbiguousRelated_NoGuessing(t *testing.T) {
	classifier := adjustments.NewClassifier()

	refund := finance.TransactionRecord{
		RecordID:     "refund_1",
		AmountCents:  5000,
		MerchantName: "Amazon",
		Date:         time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
	}

	// Two identical purchases - ambiguous which one the refund relates to
	candidates := []finance.TransactionRecord{
		{
			RecordID:     "purchase_1",
			AmountCents:  -5000,
			MerchantName: "Amazon",
			Date:         time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			RecordID:     "purchase_2",
			AmountCents:  -5000,
			MerchantName: "Amazon",
			Date:         time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC),
		},
	}

	result := classifier.FindRelated(refund, candidates)

	// Should not guess - return empty ID for ambiguous case
	if result.RelatedCanonicalID != "" {
		t.Errorf("expected empty ID for ambiguous match, got %s", result.RelatedCanonicalID)
	}

	if !result.UncertainRelation {
		t.Error("expected UncertainRelation=true")
	}

	if result.MatchConfidence != "low" {
		t.Errorf("expected low confidence, got %s", result.MatchConfidence)
	}
}

// --- Sign Handling Edge Cases ---

func TestEdgeCase_NegativeRefund_FlippedCorrectly(t *testing.T) {
	// Some providers report refunds as negative
	tx := finance.TransactionRecord{
		AmountCents: -3000, // Reported negative by provider
		Kind:        finance.KindRefund,
	}

	result := adjustments.ComputeEffectiveAmount(tx, finance.KindRefund)

	// Should flip to positive for spend calculations
	if result.EffectiveAmountCents != 3000 {
		t.Errorf("expected 3000 (flipped), got %d", result.EffectiveAmountCents)
	}
}

// --- Transfer Exclusion Tests ---

func TestEdgeCase_Transfer_ExcludedFromSpend(t *testing.T) {
	tx := finance.TransactionRecord{
		AmountCents: -100000, // $1000 transfer
		Kind:        finance.KindTransfer,
	}

	result := adjustments.ComputeEffectiveAmount(tx, finance.KindTransfer)

	// Transfers should not affect spend (effective = 0)
	if result.EffectiveAmountCents != 0 {
		t.Errorf("expected 0 for transfer, got %d", result.EffectiveAmountCents)
	}
}

// --- Fee Classification Tests ---

func TestEdgeCase_Fee_Classification(t *testing.T) {
	classifier := adjustments.NewClassifier()

	tests := []struct {
		description string
	}{
		{"MONTHLY FEE"},
		{"SERVICE FEE"},
		{"OVERDRAFT FEE"},
		{"ATM FEE"},
	}

	for _, tt := range tests {
		tx := finance.TransactionRecord{
			Description: tt.description,
			AmountCents: -500,
		}
		ctx := adjustments.ClassifyContext{Provider: "plaid"}

		result := classifier.Classify(tx, ctx)

		if result.Kind != finance.KindFee {
			t.Errorf("expected fee for %q, got %s", tt.description, result.Kind)
		}
	}
}
