package reconcile

import (
	"fmt"
	"testing"
	"time"

	"quantumlife/internal/finance/normalize"
	"quantumlife/pkg/primitives/finance"
)

// TestDeduplication_OverlappingSyncWindows demonstrates deduplication when
// the same transactions appear in overlapping sync windows.
//
// Scenario: Two syncs with 7-day overlap
// - Sync 1: Jan 1-15 (15 days)
// - Sync 2: Jan 8-22 (15 days)
// - Overlap: Jan 8-15 (8 days of duplicate transactions)
func TestDeduplication_OverlappingSyncWindows(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_demo",
		TraceID:   "dedup_demo",
	}

	// Simulate transactions from two overlapping sync windows
	var transactions []normalize.NormalizedTransactionResult

	// Sync 1: Jan 1-15 transactions
	sync1Txns := []struct {
		date        time.Time
		amount      int64
		merchant    string
		description string
	}{
		{time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), -2500, "starbucks", "Coffee"},
		{time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), -5000, "amazon", "Books"},        // Overlap
		{time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), -3500, "grocery store", "Food"}, // Overlap
		{time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC), -1500, "netflix", "Streaming"},  // Overlap
		{time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), -8000, "gas station", "Fuel"},   // Overlap
	}

	// Sync 2: Jan 8-22 transactions (includes duplicates from overlap period)
	sync2Txns := []struct {
		date        time.Time
		amount      int64
		merchant    string
		description string
	}{
		{time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), -5000, "amazon", "Books"},        // DUPLICATE
		{time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), -3500, "grocery store", "Food"}, // DUPLICATE
		{time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC), -1500, "netflix", "Streaming"},  // DUPLICATE
		{time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), -8000, "gas station", "Fuel"},   // DUPLICATE
		{time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC), -4500, "restaurant", "Dinner"},
		{time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), -2000, "pharmacy", "Medicine"},
	}

	// Provider transaction IDs - the same transaction from provider has same ID
	// regardless of when it's synced
	providerTxIDs := map[string]string{
		"2024-01-05-starbucks":     "plaid_tx_jan05_coffee",
		"2024-01-08-amazon":        "plaid_tx_jan08_books",
		"2024-01-10-grocery store": "plaid_tx_jan10_food",
		"2024-01-12-netflix":       "plaid_tx_jan12_streaming",
		"2024-01-14-gas station":   "plaid_tx_jan14_fuel",
		"2024-01-18-restaurant":    "plaid_tx_jan18_dinner",
		"2024-01-20-pharmacy":      "plaid_tx_jan20_medicine",
	}

	// Helper to get canonical ID for a transaction
	getCanonicalID := func(tx struct {
		date        time.Time
		amount      int64
		merchant    string
		description string
	}) string {
		key := tx.date.Format("2006-01-02") + "-" + tx.merchant
		providerTxID := providerTxIDs[key]
		return finance.CanonicalTransactionID(finance.TransactionIdentityInput{
			Provider:              "plaid",
			ProviderAccountID:     "acc_checking",
			ProviderTransactionID: providerTxID,
			Date:                  tx.date,
			AmountMinorUnits:      tx.amount,
			Currency:              "USD",
			MerchantNormalized:    finance.NormalizeMerchant(tx.merchant),
		})
	}

	// Add sync 1 transactions
	for _, tx := range sync1Txns {
		canonicalID := getCanonicalID(tx)
		transactions = append(transactions, normalize.NormalizedTransactionResult{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    tx.description + " (sync 1)",
				AmountCents:    tx.amount,
				Currency:       "USD",
				Date:           tx.date,
				MerchantName:   tx.merchant,
				Category:       "expense",
			},
			CanonicalID: canonicalID,
			MatchKey:    fmt.Sprintf("tmk_%s", tx.date.Format("0102")),
			IsPending:   false,
		})
	}

	// Add sync 2 transactions - duplicates will have SAME canonical ID
	// because the provider returns the same transaction_id for the same transaction
	for _, tx := range sync2Txns {
		canonicalID := getCanonicalID(tx)
		transactions = append(transactions, normalize.NormalizedTransactionResult{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    tx.description + " (sync 2)",
				AmountCents:    tx.amount,
				Currency:       "USD",
				Date:           tx.date,
				MerchantName:   tx.merchant,
				Category:       "expense",
			},
			CanonicalID: canonicalID,
			MatchKey:    fmt.Sprintf("tmk_%s", tx.date.Format("0102")),
			IsPending:   false,
		})
	}

	// Run reconciliation
	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Print demo output
	fmt.Println("\n=== v8.4 Deduplication Demo ===")
	fmt.Printf("Scenario: Two syncs with overlapping windows\n")
	fmt.Printf("  Sync 1: Jan 1-15 (5 transactions)\n")
	fmt.Printf("  Sync 2: Jan 8-22 (6 transactions, 4 duplicates)\n")
	fmt.Println()
	fmt.Println("Reconciliation Report (counts only):")
	fmt.Printf("  Input transactions:  %d\n", result.Report.InputCount)
	fmt.Printf("  Output transactions: %d\n", result.Report.OutputCount)
	fmt.Printf("  Duplicates removed:  %d\n", result.Report.DuplicatesRemoved)
	fmt.Printf("  Debit count:         %d\n", result.Report.DebitCount)
	fmt.Printf("  Credit count:        %d\n", result.Report.CreditCount)
	fmt.Println()
	fmt.Println("Result: Same economic events counted only ONCE")
	fmt.Println("=============================")

	// Assertions
	expectedInput := 11 // 5 from sync 1 + 6 from sync 2
	expectedOutput := 7 // 5 unique from sync 1 + 2 new from sync 2
	expectedDupes := 4  // 4 duplicates from overlap

	if result.Report.InputCount != expectedInput {
		t.Errorf("expected InputCount=%d, got %d", expectedInput, result.Report.InputCount)
	}

	if result.Report.OutputCount != expectedOutput {
		t.Errorf("expected OutputCount=%d, got %d", expectedOutput, result.Report.OutputCount)
	}

	if result.Report.DuplicatesRemoved != expectedDupes {
		t.Errorf("expected DuplicatesRemoved=%d, got %d", expectedDupes, result.Report.DuplicatesRemoved)
	}
}

// TestDeduplication_PendingToPostedMerge demonstrates merging pending
// transactions with their posted counterparts.
//
// Scenario: Credit card purchase flow
// - Day 1: Pending charge appears
// - Day 3: Transaction posts (same economic event, different provider IDs)
func TestDeduplication_PendingToPostedMerge(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_demo",
		TraceID:   "pending_demo",
	}

	// Common match key components (same economic event)
	matchKey := finance.TransactionMatchKey(finance.TransactionMatchInput{
		CanonicalAccountID: "cac_credit_card",
		AmountMinorUnits:   -15000, // $150.00
		Currency:           "USD",
		MerchantNormalized: finance.NormalizeMerchant("Best Buy"),
	})

	transactions := []normalize.NormalizedTransactionResult{
		// Pending transaction (Day 1)
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "BEST BUY #1234 (Pending)",
				AmountCents:    -15000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				MerchantName:   "Best Buy",
				Category:       "shopping",
				Pending:        true,
			},
			CanonicalID: "ctx_pending_bestbuy_123", // Different canonical ID
			MatchKey:    matchKey,                  // Same match key
			IsPending:   true,
		},
		// Posted transaction (Day 3) - same economic event
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "BEST BUY #1234",
				AmountCents:    -15000,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC), // Posted later
				MerchantName:   "Best Buy",
				Category:       "shopping",
				Pending:        false,
			},
			CanonicalID: "ctx_posted_bestbuy_456", // Different canonical ID
			MatchKey:    matchKey,                 // Same match key = same event
			IsPending:   false,
		},
		// Another unrelated transaction
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "STARBUCKS",
				AmountCents:    -550,
				Currency:       "USD",
				Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
				MerchantName:   "Starbucks",
				Category:       "food",
				Pending:        false,
			},
			CanonicalID: "ctx_starbucks_789",
			MatchKey:    "tmk_different",
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Print demo output
	fmt.Println("\n=== v8.4 Pending → Posted Merge Demo ===")
	fmt.Printf("Scenario: Credit card purchase\n")
	fmt.Printf("  Day 1: Pending charge ($150.00 at Best Buy)\n")
	fmt.Printf("  Day 3: Transaction posts (same purchase)\n")
	fmt.Printf("  Plus: One unrelated Starbucks transaction\n")
	fmt.Println()
	fmt.Println("Reconciliation Report (counts only):")
	fmt.Printf("  Input transactions:  %d\n", result.Report.InputCount)
	fmt.Printf("  Output transactions: %d\n", result.Report.OutputCount)
	fmt.Printf("  Pending merged:      %d\n", result.Report.PendingMerged)
	fmt.Printf("  Pending remaining:   %d\n", result.Report.PendingCount)
	fmt.Printf("  Posted count:        %d\n", result.Report.PostedCount)
	fmt.Println()
	fmt.Println("Result: Pending absorbed into posted, no double-counting")
	fmt.Println("==========================================")

	// Assertions
	if result.Report.InputCount != 3 {
		t.Errorf("expected InputCount=3, got %d", result.Report.InputCount)
	}

	if result.Report.OutputCount != 2 {
		t.Errorf("expected OutputCount=2 (merged pending+posted, plus starbucks), got %d", result.Report.OutputCount)
	}

	if result.Report.PendingMerged != 1 {
		t.Errorf("expected PendingMerged=1, got %d", result.Report.PendingMerged)
	}

	if result.Report.PendingCount != 0 {
		t.Errorf("expected PendingCount=0 (all merged), got %d", result.Report.PendingCount)
	}

	if result.Report.PostedCount != 2 {
		t.Errorf("expected PostedCount=2, got %d", result.Report.PostedCount)
	}
}

// TestDeduplication_PartialCapture demonstrates handling of partial captures
// where pending authorization amount differs from final posted amount.
//
// Scenario: Restaurant payment with tip
// - Pending auth: $50.00 (meal only)
// - Final posted: $60.00 (meal + tip)
func TestDeduplication_PartialCapture(t *testing.T) {
	engine := NewEngine()
	ctx := ReconcileContext{
		OwnerType: "circle",
		OwnerID:   "circle_demo",
		TraceID:   "partial_capture_demo",
	}

	// Common match key (same economic event despite different amounts)
	matchKey := finance.TransactionMatchKey(finance.TransactionMatchInput{
		CanonicalAccountID: "cac_credit_card",
		AmountMinorUnits:   -5000, // Original auth amount (used for matching)
		Currency:           "USD",
		MerchantNormalized: finance.NormalizeMerchant("Restaurant XYZ"),
	})

	transactions := []normalize.NormalizedTransactionResult{
		// Pending authorization ($50.00)
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "RESTAURANT XYZ (Pending)",
				AmountCents:    -5000, // $50.00
				Currency:       "USD",
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				MerchantName:   "Restaurant XYZ",
				Category:       "food",
				Pending:        true,
			},
			CanonicalID: "ctx_pending_restaurant_123",
			MatchKey:    matchKey,
			IsPending:   true,
		},
		// Posted with tip ($60.00 = meal + tip)
		{
			Transaction: finance.TransactionRecord{
				SourceProvider: "plaid",
				Description:    "RESTAURANT XYZ",
				AmountCents:    -6000, // $60.00 (final with tip)
				Currency:       "USD",
				Date:           time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
				MerchantName:   "Restaurant XYZ",
				Category:       "food",
				Pending:        false,
			},
			CanonicalID: "ctx_posted_restaurant_456",
			MatchKey:    matchKey, // Same match key = same event
			IsPending:   false,
		},
	}

	result, err := engine.ReconcileTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("ReconcileTransactions failed: %v", err)
	}

	// Print demo output
	fmt.Println("\n=== v8.5 Partial Capture Demo ===")
	fmt.Printf("Scenario: Restaurant meal with tip\n")
	fmt.Printf("  Pending auth:  $50.00 (meal only)\n")
	fmt.Printf("  Final posted:  $60.00 (meal + tip)\n")
	fmt.Println()
	fmt.Println("Reconciliation Report (counts only):")
	fmt.Printf("  Input transactions:   %d\n", result.Report.InputCount)
	fmt.Printf("  Output transactions:  %d\n", result.Report.OutputCount)
	fmt.Printf("  Pending merged:       %d\n", result.Report.PendingMerged)
	fmt.Printf("  Partial captures:     %d\n", result.Report.PartialCaptureCount)
	fmt.Println()
	fmt.Println("Result: Pending absorbed into posted, amount difference recorded")
	fmt.Println("================================")

	// Assertions
	if result.Report.InputCount != 2 {
		t.Errorf("expected InputCount=2, got %d", result.Report.InputCount)
	}

	if result.Report.OutputCount != 1 {
		t.Errorf("expected OutputCount=1 (merged), got %d", result.Report.OutputCount)
	}

	if result.Report.PendingMerged != 1 {
		t.Errorf("expected PendingMerged=1, got %d", result.Report.PendingMerged)
	}

	if result.Report.PartialCaptureCount != 1 {
		t.Errorf("expected PartialCaptureCount=1, got %d", result.Report.PartialCaptureCount)
	}

	// Verify the merged transaction has pending amount stored
	if len(result.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(result.Transactions))
	}

	merged := result.Transactions[0]
	if merged.Transaction.PendingAmountCents == nil {
		t.Error("expected PendingAmountCents to be set for partial capture")
	} else if *merged.Transaction.PendingAmountCents != -5000 {
		t.Errorf("expected PendingAmountCents=-5000, got %d", *merged.Transaction.PendingAmountCents)
	}

	if merged.ReconciliationAction != "partial_capture" {
		t.Errorf("expected action=partial_capture, got %s", merged.ReconciliationAction)
	}
}
