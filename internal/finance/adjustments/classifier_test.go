package adjustments

import (
	"testing"
	"time"

	"quantumlife/pkg/primitives/finance"
)

func TestClassify_ProviderCategory(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		name             string
		providerCategory string
		expectedKind     finance.TransactionKind
	}{
		{"refund category", "refund", finance.KindRefund},
		{"return category", "return", finance.KindRefund},
		{"fee category", "bank_fee", finance.KindFee},
		{"transfer category", "transfer", finance.KindTransfer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := finance.TransactionRecord{}
			ctx := ClassifyContext{
				Provider:         "plaid",
				ProviderCategory: tt.providerCategory,
			}

			result := c.Classify(tx, ctx)
			if result.Kind != tt.expectedKind {
				t.Errorf("expected kind %s, got %s", tt.expectedKind, result.Kind)
			}
			if result.Method != "provider_category" {
				t.Errorf("expected method provider_category, got %s", result.Method)
			}
		})
	}
}

func TestClassify_DescriptionPattern(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		name         string
		description  string
		expectedKind finance.TransactionKind
	}{
		{"refund in description", "Customer Refund - Order 12345", finance.KindRefund},
		{"reversal in description", "Payment Reversal", finance.KindReversal},
		{"chargeback in description", "Disputed Chargeback", finance.KindChargeback},
		{"fee in description", "Monthly Fee", finance.KindFee},
		{"transfer in description", "Transfer to Savings", finance.KindTransfer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := finance.TransactionRecord{
				Description: tt.description,
			}
			ctx := ClassifyContext{Provider: "plaid"}

			result := c.Classify(tx, ctx)
			if result.Kind != tt.expectedKind {
				t.Errorf("expected kind %s, got %s", tt.expectedKind, result.Kind)
			}
			if result.Method != "description_pattern" {
				t.Errorf("expected method description_pattern, got %s", result.Method)
			}
		})
	}
}

func TestClassify_SignInference(t *testing.T) {
	c := NewClassifier()

	// Positive amount with no other signals = refund inference
	tx := finance.TransactionRecord{
		AmountCents: 5000, // Positive (credit)
		Description: "Random transaction",
		Pending:     false,
	}
	ctx := ClassifyContext{Provider: "plaid"}

	result := c.Classify(tx, ctx)
	if result.Kind != finance.KindRefund {
		t.Errorf("expected refund for positive amount, got %s", result.Kind)
	}
	if result.Method != "sign_inference" {
		t.Errorf("expected method sign_inference, got %s", result.Method)
	}
	if result.Confidence != "low" {
		t.Errorf("expected low confidence, got %s", result.Confidence)
	}
}

func TestClassify_DefaultToPurchase(t *testing.T) {
	c := NewClassifier()

	// Negative amount with no signals = purchase
	tx := finance.TransactionRecord{
		AmountCents: -5000, // Negative (debit)
		Description: "Random purchase",
		Pending:     false,
	}
	ctx := ClassifyContext{Provider: "plaid"}

	result := c.Classify(tx, ctx)
	if result.Kind != finance.KindPurchase {
		t.Errorf("expected purchase for negative amount default, got %s", result.Kind)
	}
	if result.Method != "default" {
		t.Errorf("expected method default, got %s", result.Method)
	}
}

func TestFindRelated_SingleMatch(t *testing.T) {
	c := NewClassifier()

	refund := finance.TransactionRecord{
		RecordID:     "refund_1",
		AmountCents:  5000, // Positive refund
		MerchantName: "Amazon",
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	candidates := []finance.TransactionRecord{
		{
			RecordID:     "purchase_1",
			AmountCents:  -5000, // Original purchase
			MerchantName: "Amazon",
			Date:         time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	result := c.FindRelated(refund, candidates)
	if result.RelatedCanonicalID != "purchase_1" {
		t.Errorf("expected to find purchase_1, got %s", result.RelatedCanonicalID)
	}
	if result.MatchConfidence != "high" {
		t.Errorf("expected high confidence, got %s", result.MatchConfidence)
	}
	if result.UncertainRelation {
		t.Error("expected certain relation")
	}
}

func TestFindRelated_AmbiguousMatch(t *testing.T) {
	c := NewClassifier()

	refund := finance.TransactionRecord{
		RecordID:     "refund_1",
		AmountCents:  5000,
		MerchantName: "Amazon",
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	// Two similar purchases - ambiguous
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

	result := c.FindRelated(refund, candidates)
	if result.RelatedCanonicalID != "" {
		t.Errorf("expected empty ID for ambiguous match, got %s", result.RelatedCanonicalID)
	}
	if result.MatchConfidence != "low" {
		t.Errorf("expected low confidence for ambiguous, got %s", result.MatchConfidence)
	}
	if !result.UncertainRelation {
		t.Error("expected uncertain relation")
	}
	if result.CandidateCount != 2 {
		t.Errorf("expected 2 candidates, got %d", result.CandidateCount)
	}
}

func TestFindRelated_NoMatch(t *testing.T) {
	c := NewClassifier()

	refund := finance.TransactionRecord{
		RecordID:     "refund_1",
		AmountCents:  5000,
		MerchantName: "Amazon",
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	// No matching purchases
	candidates := []finance.TransactionRecord{
		{
			RecordID:     "purchase_1",
			AmountCents:  -10000,   // Different amount
			MerchantName: "Target", // Different merchant
			Date:         time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	result := c.FindRelated(refund, candidates)
	if result.MatchConfidence != "none" {
		t.Errorf("expected no match, got confidence %s", result.MatchConfidence)
	}
}

func TestComputeEffectiveAmount_Purchase(t *testing.T) {
	tx := finance.TransactionRecord{AmountCents: -5000}
	result := ComputeEffectiveAmount(tx, finance.KindPurchase)

	if result.EffectiveAmountCents != -5000 {
		t.Errorf("expected -5000, got %d", result.EffectiveAmountCents)
	}
	if result.IsAdjustment {
		t.Error("purchase should not be adjustment")
	}
}

func TestComputeEffectiveAmount_Refund(t *testing.T) {
	tx := finance.TransactionRecord{AmountCents: 5000}
	result := ComputeEffectiveAmount(tx, finance.KindRefund)

	if result.EffectiveAmountCents != 5000 {
		t.Errorf("expected 5000, got %d", result.EffectiveAmountCents)
	}
	if !result.IsAdjustment {
		t.Error("refund should be adjustment")
	}
}

func TestComputeEffectiveAmount_RefundNegative(t *testing.T) {
	// Some providers report refunds as negative
	tx := finance.TransactionRecord{AmountCents: -5000}
	result := ComputeEffectiveAmount(tx, finance.KindRefund)

	if result.EffectiveAmountCents != 5000 {
		t.Errorf("expected 5000 (flipped), got %d", result.EffectiveAmountCents)
	}
	if !result.IsAdjustment {
		t.Error("refund should be adjustment")
	}
}

func TestComputeEffectiveAmount_Transfer(t *testing.T) {
	tx := finance.TransactionRecord{AmountCents: -10000}
	result := ComputeEffectiveAmount(tx, finance.KindTransfer)

	if result.EffectiveAmountCents != 0 {
		t.Errorf("expected 0 for transfer, got %d", result.EffectiveAmountCents)
	}
	if result.IsAdjustment {
		t.Error("transfer should not be adjustment")
	}
}
