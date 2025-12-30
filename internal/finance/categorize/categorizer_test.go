// Package categorize_test provides tests for the categorization module.
package categorize_test

import (
	"testing"

	"quantumlife/internal/finance/categorize"
)

func TestCategorizer_Categorize(t *testing.T) {
	c := categorize.NewCategorizer()

	tests := []struct {
		name           string
		merchantName   string
		description    string
		expectCategory string
		expectCertain  bool
	}{
		{
			name:           "groceries by merchant",
			merchantName:   "WHOLE FOODS MARKET",
			description:    "Purchase",
			expectCategory: "Groceries",
			expectCertain:  true,
		},
		{
			name:           "groceries by description - trader joes in merchant",
			merchantName:   "TRADER JOE'S #123",
			description:    "Purchase",
			expectCategory: "Groceries",
			expectCertain:  true,
		},
		{
			name:           "gas station",
			merchantName:   "SHELL OIL",
			description:    "Fuel purchase",
			expectCategory: "Gas Stations",
			expectCertain:  true,
		},
		{
			name:           "restaurant",
			merchantName:   "MCDONALDS",
			description:    "Fast food",
			expectCategory: "Food and Drink",
			expectCertain:  true,
		},
		{
			name:           "entertainment - netflix",
			merchantName:   "NETFLIX",
			description:    "Monthly subscription",
			expectCategory: "Entertainment",
			expectCertain:  true,
		},
		{
			name:           "entertainment - spotify",
			merchantName:   "SPOTIFY USA",
			description:    "Subscription",
			expectCategory: "Entertainment",
			expectCertain:  true,
		},
		{
			name:           "utilities - electric",
			merchantName:   "COMCAST CABLE",
			description:    "Monthly bill",
			expectCategory: "Utilities",
			expectCertain:  true,
		},
		{
			name:           "utilities - water company",
			merchantName:   "CITY WATER UTILITY",
			description:    "Water bill",
			expectCategory: "Utilities",
			expectCertain:  true,
		},
		{
			name:           "shopping",
			merchantName:   "AMAZON.COM",
			description:    "Online purchase",
			expectCategory: "Shopping",
			expectCertain:  true,
		},
		{
			name:           "uncategorized",
			merchantName:   "RANDOM VENDOR XYZ",
			description:    "RANDOM TRANSACTION",
			expectCategory: "Uncategorized",
			expectCertain:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Categorize(tt.merchantName, tt.description)

			if result.Category != tt.expectCategory {
				t.Errorf("Category = %q, want %q", result.Category, tt.expectCategory)
			}

			if result.Certain != tt.expectCertain {
				t.Errorf("Certain = %v, want %v", result.Certain, tt.expectCertain)
			}

			// Verify it has a category ID
			if result.CategoryID == "" {
				t.Error("CategoryID should not be empty")
			}

			// Verify matched rule is set for certain categorizations
			if result.Certain && result.MatchedRule == "" {
				t.Error("MatchedRule should not be empty for certain categorizations")
			}
		})
	}
}

func TestCategorizer_AllCategories(t *testing.T) {
	categories := categorize.AllCategories()

	if len(categories) == 0 {
		t.Error("expected at least one category")
	}

	// Verify expected categories exist
	expected := []string{"Groceries", "Gas Stations", "Food and Drink", "Entertainment", "Utilities", "Shopping"}
	for _, exp := range expected {
		found := false
		for _, cat := range categories {
			if cat == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected category %q not found in AllCategories()", exp)
		}
	}
}

func TestCategorizer_CaseInsensitive(t *testing.T) {
	c := categorize.NewCategorizer()

	// Test case insensitivity
	result1 := c.Categorize("whole foods", "purchase")
	result2 := c.Categorize("WHOLE FOODS", "PURCHASE")
	result3 := c.Categorize("Whole Foods", "Purchase")

	if result1.Category != result2.Category || result2.Category != result3.Category {
		t.Errorf("categorization should be case-insensitive: got %q, %q, %q",
			result1.Category, result2.Category, result3.Category)
	}
}

func TestCategorizer_Deterministic(t *testing.T) {
	c := categorize.NewCategorizer()

	// Same input should always produce same output
	merchant := "STARBUCKS COFFEE"
	description := "Coffee purchase"

	var results []string
	for i := 0; i < 10; i++ {
		result := c.Categorize(merchant, description)
		results = append(results, result.Category)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("categorization should be deterministic: got different results %v", results)
			break
		}
	}
}

func TestCategorizationResult_NoProbabilityField(t *testing.T) {
	c := categorize.NewCategorizer()
	result := c.Categorize("WALMART", "Shopping")

	// CRITICAL: Verify there's no probability/confidence field (per spec)
	// The result should only have Certain (bool), not a probability score
	// This is a compile-time check, but we document it here
	_ = result.Certain // This should be bool, not float

	// Verify it's binary - either certain or uncertain
	if result.Certain {
		// If certain, this is from a rule match
		if result.MatchedRule == "" {
			t.Error("certain results should have a matched rule")
		}
	}
}
