package sharedview

import (
	"fmt"

	"quantumlife/pkg/primitives/finance"
)

// VisibilityEnforcer applies visibility policies to financial data.
// All filtering is deterministic and auditable.
//
// CRITICAL: This enforces what data can be shared in intersections.
// Rejected data is never exposed.
type VisibilityEnforcer struct{}

// NewVisibilityEnforcer creates a new visibility enforcer.
func NewVisibilityEnforcer() *VisibilityEnforcer {
	return &VisibilityEnforcer{}
}

// FilterTransactions applies the policy to filter transactions.
// Returns only transactions that pass the policy checks.
func (e *VisibilityEnforcer) FilterTransactions(
	transactions []finance.TransactionRecord,
	policy finance.VisibilityPolicy,
) FilterResult {
	if !policy.Enabled {
		return FilterResult{
			Allowed:       []finance.TransactionRecord{},
			RejectedCount: len(transactions),
			Reason:        "policy_disabled",
		}
	}

	var allowed []finance.TransactionRecord
	rejectedByCategory := 0
	rejectedByAccount := 0

	for _, tx := range transactions {
		// Check category filter
		if len(policy.CategoriesAllowed) > 0 {
			if !categoryAllowed(tx.Category, policy.CategoriesAllowed) {
				rejectedByCategory++
				continue
			}
		}

		// Check account filter
		if len(policy.AccountsIncluded) > 0 {
			if !accountAllowed(tx.AccountID, policy.AccountsIncluded) {
				rejectedByAccount++
				continue
			}
		}

		allowed = append(allowed, tx)
	}

	return FilterResult{
		Allowed:            allowed,
		RejectedCount:      rejectedByCategory + rejectedByAccount,
		RejectedByCategory: rejectedByCategory,
		RejectedByAccount:  rejectedByAccount,
		Reason:             "",
	}
}

// FilterResult contains the results of visibility filtering.
type FilterResult struct {
	// Allowed transactions that pass the policy.
	Allowed []finance.TransactionRecord

	// RejectedCount is the total number of rejected transactions.
	RejectedCount int

	// RejectedByCategory is rejected due to category filter.
	RejectedByCategory int

	// RejectedByAccount is rejected due to account filter.
	RejectedByAccount int

	// Reason is set if all transactions rejected (e.g., "policy_disabled").
	Reason string
}

// AnonymizeTransaction applies anonymization to a transaction.
// Returns a copy with sensitive fields removed based on policy.
func (e *VisibilityEnforcer) AnonymizeTransaction(
	tx finance.TransactionRecord,
	policy finance.VisibilityPolicy,
) AnonymizedTransaction {
	result := AnonymizedTransaction{
		RecordID:           tx.RecordID,
		Date:               tx.Date.Format("2006-01-02"),
		Currency:           tx.Currency,
		CategoryNormalized: tx.Category,
		IsPending:          tx.Pending,
	}

	// Apply visibility level
	switch policy.VisibilityLevel {
	case finance.VisibilityFull:
		result.MerchantName = tx.MerchantName
		result.Description = tx.Description
	case finance.VisibilityAnonymized:
		result.MerchantName = anonymizeMerchant(tx.MerchantName)
		result.Description = ""
	case finance.VisibilityCategoryOnly, finance.VisibilityTotalsOnly:
		result.MerchantName = ""
		result.Description = ""
	}

	// Apply amount granularity
	switch policy.AmountGranularity {
	case finance.GranularityExact:
		result.AmountCents = tx.AmountCents
		result.AmountBucket = ""
	case finance.GranularityBucketed:
		result.AmountCents = 0
		result.AmountBucket = computeBucket(tx.AmountCents)
	case finance.GranularityHidden:
		result.AmountCents = 0
		result.AmountBucket = BucketHidden
	}

	return result
}

// AnonymizedTransaction is a transaction with visibility rules applied.
type AnonymizedTransaction struct {
	RecordID           string
	Date               string
	Currency           string
	CategoryNormalized string
	MerchantName       string       // May be empty or anonymized
	Description        string       // May be empty
	AmountCents        int64        // 0 when hidden/bucketed
	AmountBucket       AmountBucket // Set when bucketed/hidden
	IsPending          bool
}

// AnonymizeAmount converts an amount to a display format based on policy.
func (e *VisibilityEnforcer) AnonymizeAmount(
	cents int64,
	currency string,
	granularity finance.AmountGranularity,
) string {
	switch granularity {
	case finance.GranularityExact:
		return formatAmount(cents, currency)
	case finance.GranularityBucketed:
		bucket := computeBucket(cents)
		return bucketToRange(bucket, currency)
	case finance.GranularityHidden:
		return "[amount hidden]"
	default:
		return bucketToRange(computeBucket(cents), currency)
	}
}

// ValidatePolicy checks if a policy is valid for use.
func (e *VisibilityEnforcer) ValidatePolicy(policy finance.VisibilityPolicy) error {
	return policy.Validate()
}

// categoryAllowed checks if a category is in the allowed list.
func categoryAllowed(category string, allowed []string) bool {
	for _, a := range allowed {
		if a == category {
			return true
		}
	}
	return false
}

// accountAllowed checks if an account is in the allowed list.
func accountAllowed(accountID string, allowed []string) bool {
	for _, a := range allowed {
		if a == accountID {
			return true
		}
	}
	return false
}

// anonymizeMerchant replaces merchant name with category-based placeholder.
func anonymizeMerchant(merchant string) string {
	if merchant == "" {
		return ""
	}
	return "Merchant"
}

// formatAmount formats cents as a currency string.
func formatAmount(cents int64, currency string) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	dollars := cents / 100
	remaining := cents % 100

	sign := ""
	if negative {
		sign = "-"
	}

	switch currency {
	case "USD":
		return fmt.Sprintf("%s$%d.%02d", sign, dollars, remaining)
	case "EUR":
		return fmt.Sprintf("%s€%d.%02d", sign, dollars, remaining)
	case "GBP":
		return fmt.Sprintf("%s£%d.%02d", sign, dollars, remaining)
	default:
		return fmt.Sprintf("%s%s %d.%02d", sign, currency, dollars, remaining)
	}
}

// bucketToRange converts a bucket to a human-readable range.
func bucketToRange(bucket AmountBucket, currency string) string {
	symbol := "$"
	switch currency {
	case "EUR":
		symbol = "€"
	case "GBP":
		symbol = "£"
	}

	switch bucket {
	case BucketLow:
		return fmt.Sprintf("<%s100", symbol)
	case BucketMedium:
		return fmt.Sprintf("%s100-%s500", symbol, symbol)
	case BucketHigh:
		return fmt.Sprintf("%s500-%s2000", symbol, symbol)
	case BucketVeryHigh:
		return fmt.Sprintf(">%s2000", symbol)
	case BucketHidden:
		return "[hidden]"
	default:
		return "[unknown]"
	}
}
