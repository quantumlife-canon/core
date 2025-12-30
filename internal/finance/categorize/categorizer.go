// Package categorize provides deterministic rule-based transaction categorization.
//
// CRITICAL: No ML, no probabilities. Pure rule-based matching only.
// Every categorization is deterministic and explainable.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md §6
package categorize

import (
	"strings"

	"quantumlife/pkg/primitives/finance"
)

// Categorizer performs deterministic transaction categorization.
type Categorizer struct {
	rules []CategoryRule
}

// CategoryRule defines a categorization rule.
type CategoryRule struct {
	// RuleID uniquely identifies this rule.
	RuleID string

	// Priority determines rule ordering (lower = higher priority).
	Priority int

	// Category is the category to assign.
	Category string

	// CategoryID is the category identifier.
	CategoryID string

	// MerchantPatterns are substrings to match in merchant name.
	MerchantPatterns []string

	// DescriptionPatterns are substrings to match in description.
	DescriptionPatterns []string
}

// NewCategorizer creates a new categorizer with default rules.
func NewCategorizer() *Categorizer {
	return &Categorizer{
		rules: defaultRules(),
	}
}

// NewCategorizerWithRules creates a categorizer with custom rules.
func NewCategorizerWithRules(rules []CategoryRule) *Categorizer {
	return &Categorizer{
		rules: rules,
	}
}

// Categorize assigns a category to a transaction.
// Returns a CategorizationResult with certainty information.
func (c *Categorizer) Categorize(merchantName, description string) finance.CategorizationResult {
	merchantLower := strings.ToLower(merchantName)
	descLower := strings.ToLower(description)

	// Try each rule in priority order
	for _, rule := range c.rules {
		if c.matchesRule(merchantLower, descLower, rule) {
			return finance.CategorizationResult{
				Category:    rule.Category,
				CategoryID:  rule.CategoryID,
				MatchedRule: rule.RuleID,
				Certain:     true,
				Reason:      "Matched rule: " + rule.RuleID,
			}
		}
	}

	// Fallback to uncategorized
	return finance.CategorizationResult{
		Category:    CategoryUncategorized,
		CategoryID:  CategoryIDUncategorized,
		MatchedRule: "fallback",
		Certain:     false,
		Reason:      "No matching rule found",
	}
}

// matchesRule checks if a transaction matches a rule.
func (c *Categorizer) matchesRule(merchantLower, descLower string, rule CategoryRule) bool {
	// Check merchant patterns
	for _, pattern := range rule.MerchantPatterns {
		if strings.Contains(merchantLower, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check description patterns
	for _, pattern := range rule.DescriptionPatterns {
		if strings.Contains(descLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// Category constants.
const (
	CategoryGroceries      = "Groceries"
	CategoryGasStations    = "Gas Stations"
	CategoryEntertainment  = "Entertainment"
	CategoryShopping       = "Shopping"
	CategoryFoodAndDrink   = "Food and Drink"
	CategoryTransportation = "Transportation"
	CategoryHealth         = "Health"
	CategoryUtilities      = "Utilities"
	CategorySubscriptions  = "Subscriptions"
	CategoryTransfer       = "Transfer"
	CategoryIncome         = "Income"
	CategoryUncategorized  = "Uncategorized"

	CategoryIDGroceries      = "cat-groceries"
	CategoryIDGasStations    = "cat-gas"
	CategoryIDEntertainment  = "cat-entertainment"
	CategoryIDShopping       = "cat-shopping"
	CategoryIDFoodAndDrink   = "cat-food"
	CategoryIDTransportation = "cat-transport"
	CategoryIDHealth         = "cat-health"
	CategoryIDUtilities      = "cat-utilities"
	CategoryIDSubscriptions  = "cat-subscriptions"
	CategoryIDTransfer       = "cat-transfer"
	CategoryIDIncome         = "cat-income"
	CategoryIDUncategorized  = "cat-uncategorized"
)

// defaultRules returns the default categorization rules.
func defaultRules() []CategoryRule {
	return []CategoryRule{
		// Groceries
		{
			RuleID:     "groceries-supermarkets",
			Priority:   1,
			Category:   CategoryGroceries,
			CategoryID: CategoryIDGroceries,
			MerchantPatterns: []string{
				"whole foods", "trader joe", "safeway", "kroger", "walmart",
				"target", "costco", "publix", "aldi", "grocery", "market",
				"supermarket", "food lion", "stop & shop", "giant",
			},
		},
		// Gas Stations
		{
			RuleID:     "gas-stations",
			Priority:   2,
			Category:   CategoryGasStations,
			CategoryID: CategoryIDGasStations,
			MerchantPatterns: []string{
				"shell", "exxon", "mobil", "chevron", "bp", "gas", "fuel",
				"sunoco", "citgo", "marathon", "speedway", "valero",
			},
		},
		// Entertainment
		{
			RuleID:     "entertainment-streaming",
			Priority:   3,
			Category:   CategoryEntertainment,
			CategoryID: CategoryIDEntertainment,
			MerchantPatterns: []string{
				"netflix", "spotify", "hulu", "disney+", "hbo", "amazon prime",
				"apple music", "youtube", "pandora", "paramount+", "peacock",
			},
		},
		// Subscriptions
		{
			RuleID:     "subscriptions",
			Priority:   4,
			Category:   CategorySubscriptions,
			CategoryID: CategoryIDSubscriptions,
			MerchantPatterns: []string{
				"membership", "subscription", "monthly fee", "annual fee",
			},
		},
		// Food and Drink
		{
			RuleID:     "food-restaurants",
			Priority:   5,
			Category:   CategoryFoodAndDrink,
			CategoryID: CategoryIDFoodAndDrink,
			MerchantPatterns: []string{
				"starbucks", "dunkin", "mcdonald", "burger king", "wendy",
				"chipotle", "subway", "pizza", "restaurant", "cafe", "coffee",
				"doordash", "uber eats", "grubhub", "postmates",
			},
		},
		// Transportation
		{
			RuleID:     "transportation",
			Priority:   6,
			Category:   CategoryTransportation,
			CategoryID: CategoryIDTransportation,
			MerchantPatterns: []string{
				"uber", "lyft", "taxi", "metro", "transit", "parking",
				"toll", "airline", "southwest", "delta", "united", "jetblue",
			},
		},
		// Health
		{
			RuleID:     "health",
			Priority:   7,
			Category:   CategoryHealth,
			CategoryID: CategoryIDHealth,
			MerchantPatterns: []string{
				"cvs", "walgreens", "pharmacy", "hospital", "clinic",
				"doctor", "medical", "dental", "health", "insurance",
			},
		},
		// Utilities
		{
			RuleID:     "utilities",
			Priority:   8,
			Category:   CategoryUtilities,
			CategoryID: CategoryIDUtilities,
			MerchantPatterns: []string{
				"electric", "gas company", "water", "utility", "internet",
				"comcast", "verizon", "at&t", "t-mobile", "phone", "cable",
			},
		},
		// Shopping
		{
			RuleID:     "shopping-general",
			Priority:   9,
			Category:   CategoryShopping,
			CategoryID: CategoryIDShopping,
			MerchantPatterns: []string{
				"amazon", "ebay", "etsy", "best buy", "home depot", "lowes",
				"ikea", "nordstrom", "macys", "kohls", "bed bath",
			},
		},
		// Transfers
		{
			RuleID:     "transfers",
			Priority:   10,
			Category:   CategoryTransfer,
			CategoryID: CategoryIDTransfer,
			MerchantPatterns: []string{
				"transfer", "venmo", "paypal", "zelle", "cash app",
			},
			DescriptionPatterns: []string{
				"transfer from", "transfer to", "ach transfer",
			},
		},
		// Income
		{
			RuleID:     "income",
			Priority:   11,
			Category:   CategoryIncome,
			CategoryID: CategoryIDIncome,
			DescriptionPatterns: []string{
				"payroll", "direct deposit", "salary", "wages", "bonus",
				"interest payment", "dividend",
			},
		},
	}
}

// AllCategories returns all known categories.
func AllCategories() []string {
	return []string{
		CategoryGroceries,
		CategoryGasStations,
		CategoryEntertainment,
		CategoryShopping,
		CategoryFoodAndDrink,
		CategoryTransportation,
		CategoryHealth,
		CategoryUtilities,
		CategorySubscriptions,
		CategoryTransfer,
		CategoryIncome,
		CategoryUncategorized,
	}
}
