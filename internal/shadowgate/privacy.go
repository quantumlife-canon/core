// Package shadowgate provides the candidate engine for Shadow Gating.
//
// Phase 19.5: Shadow Gating + Promotion Candidates (NO behavior change)
//
// This file provides privacy validation specific to candidates.
// Ensures WhyGeneric and other strings contain no identifiable information.
//
// Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
package shadowgate

import (
	"regexp"
	"strings"
)

// PrivacyGuard validates candidate strings for privacy compliance.
type PrivacyGuard struct {
	forbiddenPatterns []*regexp.Regexp
}

// NewPrivacyGuard creates a new privacy guard for candidates.
func NewPrivacyGuard() *PrivacyGuard {
	return &PrivacyGuard{
		forbiddenPatterns: compileForbiddenPatterns(),
	}
}

// compileForbiddenPatterns compiles regex patterns for forbidden content.
func compileForbiddenPatterns() []*regexp.Regexp {
	patterns := []string{
		// Email addresses
		`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		// URLs and domains
		`https?://[^\s]+`,
		`www\.[^\s]+`,
		// @ mentions
		`@[a-zA-Z0-9_]+`,
		// Currency amounts with symbols
		`[$£€¥₹]\s*\d+`,
		`\d+\s*[$£€¥₹]`,
		// Currency codes with amounts
		`(?i)(USD|EUR|GBP|INR|JPY)\s*\d+`,
		`\d+\s*(?i)(USD|EUR|GBP|INR|JPY)`,
		// Large numbers that could be amounts (4+ digits)
		`\b\d{4,}\b`,
		// Phone numbers
		`\d{3}[-.\s]?\d{3}[-.\s]?\d{4}`,
		`\+\d{1,3}[-.\s]?\d+`,
		// IP addresses
		`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`,
		// Credit card patterns
		`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`,
		// SSN patterns
		`\d{3}[-\s]?\d{2}[-\s]?\d{4}`,
		// Common vendor/company names (case insensitive)
		`(?i)\b(amazon|google|apple|microsoft|netflix|uber|lyft|venmo|paypal|chase|wells\s*fargo|bank\s*of\s*america|citibank|amex|visa|mastercard)\b`,
		// Names (simple heuristic: capitalized words that could be names)
		// Note: We're lenient here; main protection is length limit
		`\b[A-Z][a-z]+\s+[A-Z][a-z]+\s+[A-Z][a-z]+\b`, // Three capitalized words in sequence
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// DefaultSafeWhy is the fallback text when a why string fails validation.
const DefaultSafeWhy = "A pattern we've seen before."

// ValidateWhyGeneric checks if a WhyGeneric string is privacy-safe.
// Returns nil if safe, error otherwise.
func (g *PrivacyGuard) ValidateWhyGeneric(why string) error {
	if why == "" {
		return &PrivacyError{Field: "why_generic", Message: "empty string"}
	}

	// Length limit - reject long strings that could contain content
	if len(why) > 100 {
		return &PrivacyError{
			Field:   "why_generic",
			Message: "string too long (max 100 chars)",
		}
	}

	// Check for forbidden patterns
	for _, re := range g.forbiddenPatterns {
		if re.MatchString(why) {
			return &PrivacyError{
				Field:   "why_generic",
				Message: "contains forbidden pattern",
			}
		}
	}

	// Check for obviously unsafe characters that could indicate identifiers
	unsafeChars := []string{"@", "://", ".com", ".org", ".net", ".io", "http"}
	for _, uc := range unsafeChars {
		if strings.Contains(strings.ToLower(why), uc) {
			return &PrivacyError{
				Field:   "why_generic",
				Message: "contains unsafe substring",
			}
		}
	}

	return nil
}

// SanitizeWhyGeneric returns a safe version of the why string.
// If the string fails validation, returns the default safe why.
func (g *PrivacyGuard) SanitizeWhyGeneric(why string) string {
	if err := g.ValidateWhyGeneric(why); err != nil {
		return DefaultSafeWhy
	}
	return why
}

// IsPrivacySafe checks if a string is safe for output.
func (g *PrivacyGuard) IsPrivacySafe(s string) bool {
	if s == "" {
		return true
	}

	if len(s) > 256 {
		return false
	}

	for _, re := range g.forbiddenPatterns {
		if re.MatchString(s) {
			return false
		}
	}

	return true
}

// AllowedReasonPhrases are the pre-approved reason phrases.
// Using these ensures privacy compliance.
var AllowedReasonPhrases = []string{
	"A pattern we've seen before.",
	"Something that recurs in this category.",
	"A timing pattern you might want to address.",
	"Items that tend to need attention together.",
	"A spending pattern worth reviewing.",
	"Calendar items that often align.",
	"Messages that cluster by topic.",
	"Work items with similar urgency.",
	"Home tasks that appear together.",
	"People-related items requiring attention.",
}

// IsAllowedPhrase checks if a phrase is in the pre-approved list.
func IsAllowedPhrase(phrase string) bool {
	for _, allowed := range AllowedReasonPhrases {
		if phrase == allowed {
			return true
		}
	}
	return false
}

// SelectReasonPhrase selects a pre-approved reason phrase based on category.
func SelectReasonPhrase(category string) string {
	switch category {
	case "money":
		return "A spending pattern worth reviewing."
	case "time":
		return "A timing pattern you might want to address."
	case "work":
		return "Work items with similar urgency."
	case "home":
		return "Home tasks that appear together."
	case "people":
		return "People-related items requiring attention."
	default:
		return DefaultSafeWhy
	}
}

// PrivacyError represents a privacy validation error.
type PrivacyError struct {
	Field   string
	Message string
}

func (e *PrivacyError) Error() string {
	return "privacy validation failed: " + e.Field + ": " + e.Message
}
