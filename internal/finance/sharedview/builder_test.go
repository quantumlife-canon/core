package sharedview

import (
	"testing"
	"time"

	"quantumlife/pkg/primitives/finance"
)

func testIDGen() string {
	return "test-view-id"
}

func TestViewBuilder_Build_Basic(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityExact,
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000, "dining": 5000},
				},
				TotalsByCurrency: map[string]int64{"USD": 20000},
				TransactionCounts: map[string]map[string]int{
					"USD": {"groceries": 3, "dining": 2},
				},
				LastSyncTime: time.Now().Add(-30 * time.Minute),
			},
			{
				CircleID: "circle_bob",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 10000, "entertainment": 8000},
				},
				TotalsByCurrency: map[string]int64{"USD": 18000},
				TransactionCounts: map[string]map[string]int{
					"USD": {"groceries": 2, "entertainment": 4},
				},
				LastSyncTime: time.Now().Add(-45 * time.Minute),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify view basics
	if view.IntersectionID != "int_123" {
		t.Errorf("wrong intersection ID: %s", view.IntersectionID)
	}
	if view.ViewID != "test-view-id" {
		t.Errorf("wrong view ID: %s", view.ViewID)
	}

	// Verify aggregation
	usdSpend := view.SpendByCategory["USD"]
	if usdSpend == nil {
		t.Fatal("missing USD spend")
	}

	groceries := usdSpend["groceries"]
	if groceries.TotalCents != 25000 { // 15000 + 10000
		t.Errorf("groceries total wrong: %d", groceries.TotalCents)
	}
	if groceries.TransactionCount != 5 { // 3 + 2
		t.Errorf("groceries count wrong: %d", groceries.TransactionCount)
	}

	dining := usdSpend["dining"]
	if dining.TotalCents != 5000 {
		t.Errorf("dining total wrong: %d", dining.TotalCents)
	}

	entertainment := usdSpend["entertainment"]
	if entertainment.TotalCents != 8000 {
		t.Errorf("entertainment total wrong: %d", entertainment.TotalCents)
	}

	// Verify provenance
	if view.Provenance.ContributorCount != 2 {
		t.Errorf("wrong contributor count: %d", view.Provenance.ContributorCount)
	}
	if !view.Provenance.SymmetryVerified {
		t.Error("symmetry should be verified")
	}

	// Verify content hash exists
	if view.ContentHash == "" {
		t.Error("content hash should be set")
	}
}

func TestViewBuilder_Build_Bucketed(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityBucketed, // Bucketed
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 75000}, // $750 -> high bucket
				},
				TotalsByCurrency: map[string]int64{"USD": 75000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Exact amount should be hidden
	groceries := view.SpendByCategory["USD"]["groceries"]
	if groceries.TotalCents != 0 {
		t.Errorf("expected 0 for bucketed, got %d", groceries.TotalCents)
	}
	if groceries.Bucket != BucketHigh {
		t.Errorf("expected high bucket, got %s", groceries.Bucket)
	}
}

func TestViewBuilder_Build_Hidden(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityHidden, // Hidden
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 75000},
				},
				TotalsByCurrency: map[string]int64{"USD": 75000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	groceries := view.SpendByCategory["USD"]["groceries"]
	if groceries.TotalCents != 0 {
		t.Errorf("expected 0 for hidden, got %d", groceries.TotalCents)
	}
	if groceries.Bucket != BucketHidden {
		t.Errorf("expected hidden bucket, got %s", groceries.Bucket)
	}
}

func TestViewBuilder_Build_CategoryFilter(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityExact,
			CategoriesAllowed: []string{"groceries"}, // Only groceries
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000, "dining": 5000, "entertainment": 8000},
				},
				TotalsByCurrency: map[string]int64{"USD": 28000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	usdSpend := view.SpendByCategory["USD"]
	if _, ok := usdSpend["groceries"]; !ok {
		t.Error("groceries should be present")
	}
	if _, ok := usdSpend["dining"]; ok {
		t.Error("dining should be filtered out")
	}
	if _, ok := usdSpend["entertainment"]; ok {
		t.Error("entertainment should be filtered out")
	}
}

func TestViewBuilder_Build_MultiCurrency(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityExact,
			CurrencyDisplay:   "all",
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000},
					"EUR": {"groceries": 10000},
				},
				TotalsByCurrency: map[string]int64{"USD": 15000, "EUR": 10000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if _, ok := view.SpendByCategory["USD"]; !ok {
		t.Error("USD should be present")
	}
	if _, ok := view.SpendByCategory["EUR"]; !ok {
		t.Error("EUR should be present")
	}
}

func TestViewBuilder_Build_PrimaryCurrencyOnly(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityExact,
			CurrencyDisplay:   "primary", // Only USD
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000},
					"EUR": {"groceries": 10000},
				},
				TotalsByCurrency: map[string]int64{"USD": 15000, "EUR": 10000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if _, ok := view.SpendByCategory["USD"]; !ok {
		t.Error("USD should be present")
	}
	if _, ok := view.SpendByCategory["EUR"]; ok {
		t.Error("EUR should be filtered out with primary currency")
	}
}

func TestViewBuilder_Build_Deterministic(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityExact,
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_bob", // Note: Bob first
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 10000},
				},
				TotalsByCurrency: map[string]int64{"USD": 10000},
				LastSyncTime:     time.Now(),
			},
			{
				CircleID: "circle_alice", // Alice second
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000},
				},
				TotalsByCurrency: map[string]int64{"USD": 15000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	view1, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build 1 failed: %v", err)
	}

	// Reverse order of contributions
	req.Contributions = []CircleContribution{req.Contributions[1], req.Contributions[0]}

	view2, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build 2 failed: %v", err)
	}

	// Content hash should be the same regardless of input order
	if view1.ContentHash != view2.ContentHash {
		t.Errorf("content hashes should match: %s != %s", view1.ContentHash, view2.ContentHash)
	}

	// Provenance circle IDs should be sorted
	if view1.Provenance.ContributingCircleIDs[0] != "circle_alice" {
		t.Errorf("circle IDs should be sorted, got first: %s", view1.Provenance.ContributingCircleIDs[0])
	}
}

func TestViewBuilder_Build_NoIndividualAttribution(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:           true,
			VisibilityLevel:   finance.VisibilityCategoryOnly,
			AmountGranularity: finance.GranularityExact,
			RequireSymmetry:   true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 50000}, // $500
				},
				TotalsByCurrency: map[string]int64{"USD": 50000},
				LastSyncTime:     time.Now(),
			},
			{
				CircleID: "circle_bob",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 10000}, // $100
				},
				TotalsByCurrency: map[string]int64{"USD": 10000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should only see aggregated total, not per-circle amounts
	groceries := view.SpendByCategory["USD"]["groceries"]
	if groceries.TotalCents != 60000 { // 50000 + 10000
		t.Errorf("expected aggregated 60000, got %d", groceries.TotalCents)
	}

	// Provenance should list circles but not their individual amounts
	if view.Provenance.ContributorCount != 2 {
		t.Errorf("expected 2 contributors, got %d", view.Provenance.ContributorCount)
	}

	// CRITICAL: View should not contain any per-circle amounts
	// This is enforced by the data structure - CategorySpend doesn't have circle attribution
}

func TestComputeBucket(t *testing.T) {
	tests := []struct {
		cents    int64
		expected AmountBucket
	}{
		{0, BucketLow},
		{5000, BucketLow},         // $50
		{9999, BucketLow},         // Just under $100
		{10000, BucketMedium},     // $100
		{25000, BucketMedium},     // $250
		{49999, BucketMedium},     // Just under $500
		{50000, BucketHigh},       // $500
		{100000, BucketHigh},      // $1000
		{199999, BucketHigh},      // Just under $2000
		{200000, BucketVeryHigh},  // $2000
		{1000000, BucketVeryHigh}, // $10000
		{-50000, BucketHigh},      // Negative amount (abs value used)
	}

	for _, tt := range tests {
		result := ComputeBucket(tt.cents)
		if result != tt.expected {
			t.Errorf("ComputeBucket(%d) = %s, want %s", tt.cents, result, tt.expected)
		}
	}
}

func TestViewBuilder_Build_DisabledPolicy(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled: false, // Disabled
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000},
				},
				TotalsByCurrency: map[string]int64{"USD": 15000},
				LastSyncTime:     time.Now(),
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	_, err := builder.Build(req)
	if err == nil {
		t.Error("expected error for disabled policy")
	}
}

func TestViewBuilder_Build_EmptyContributions(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:         true,
			RequireSymmetry: true,
		},
		Contributions: []CircleContribution{}, // Empty
		WindowStart:   time.Now().AddDate(0, 0, -30),
		WindowEnd:     time.Now(),
	}

	_, err := builder.Build(req)
	if err == nil {
		t.Error("expected error for empty contributions")
	}
}

func TestViewBuilder_Build_DataFreshness(t *testing.T) {
	builder := NewViewBuilder(testIDGen)

	// Test stale data
	req := BuildRequest{
		IntersectionID: "int_123",
		Policy: finance.VisibilityPolicy{
			Enabled:         true,
			RequireSymmetry: true,
		},
		Contributions: []CircleContribution{
			{
				CircleID: "circle_alice",
				SpendByCategory: map[string]map[string]int64{
					"USD": {"groceries": 15000},
				},
				TotalsByCurrency: map[string]int64{"USD": 15000},
				LastSyncTime:     time.Now().Add(-48 * time.Hour), // 2 days ago
			},
		},
		WindowStart: time.Now().AddDate(0, 0, -30),
		WindowEnd:   time.Now(),
	}

	view, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if view.Provenance.DataFreshness != FreshnessStale {
		t.Errorf("expected stale freshness, got %s", view.Provenance.DataFreshness)
	}
}
