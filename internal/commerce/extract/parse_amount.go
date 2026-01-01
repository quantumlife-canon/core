package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// AmountResult contains parsed currency and amount.
type AmountResult struct {
	Currency    string // ISO 4217 code
	AmountCents int64  // Amount in minor units
	RawMatch    string // Original matched string
	Valid       bool
}

// AmountParser extracts currency amounts from text.
// Supports GBP (£), USD ($), EUR (€), INR (₹) and ISO codes.
type AmountParser struct {
	// Pre-compiled patterns for performance
	symbolPatterns []*regexp.Regexp
	isoPatterns    []*regexp.Regexp
}

// NewAmountParser creates an amount parser with compiled patterns.
func NewAmountParser() *AmountParser {
	return &AmountParser{
		symbolPatterns: []*regexp.Regexp{
			// £12.99 or £ 12.99 or £12
			regexp.MustCompile(`£\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
			// $12.99 or $ 12.99
			regexp.MustCompile(`\$\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
			// €12.99 or € 12.99 or €12,99 (European format)
			regexp.MustCompile(`€\s*([0-9.]+(?:,[0-9]{1,2})?)`),
			regexp.MustCompile(`€\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
			// ₹1,234.56 or ₹ 1234
			regexp.MustCompile(`₹\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
			// Rs. 1,234.56 or Rs 1234 (Indian Rupee alternative)
			regexp.MustCompile(`(?i)Rs\.?\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
		},
		isoPatterns: []*regexp.Regexp{
			// GBP 12.99 or GBP12.99
			regexp.MustCompile(`(?i)GBP\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
			// USD 12.99
			regexp.MustCompile(`(?i)USD\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
			// EUR 12.99 or EUR 12,99
			regexp.MustCompile(`(?i)EUR\s*([0-9.,]+)`),
			// INR 1234.56
			regexp.MustCompile(`(?i)INR\s*([0-9,]+(?:\.[0-9]{1,2})?)`),
		},
	}
}

// Parse extracts the first currency amount from text.
// Returns the parsed result with deterministic rounding.
func (p *AmountParser) Parse(text string) AmountResult {
	// Try symbol-based patterns first (more specific)
	// £ -> GBP
	if result := p.tryPattern(text, p.symbolPatterns[0], "GBP"); result.Valid {
		return result
	}
	// $ -> USD (assuming USD, could be other dollars)
	if result := p.tryPattern(text, p.symbolPatterns[1], "USD"); result.Valid {
		return result
	}
	// € -> EUR
	if result := p.tryPatternEUR(text, p.symbolPatterns[2]); result.Valid {
		return result
	}
	if result := p.tryPattern(text, p.symbolPatterns[3], "EUR"); result.Valid {
		return result
	}
	// ₹ -> INR
	if result := p.tryPattern(text, p.symbolPatterns[4], "INR"); result.Valid {
		return result
	}
	// Rs. -> INR
	if result := p.tryPattern(text, p.symbolPatterns[5], "INR"); result.Valid {
		return result
	}

	// Try ISO code patterns
	if result := p.tryPattern(text, p.isoPatterns[0], "GBP"); result.Valid {
		return result
	}
	if result := p.tryPattern(text, p.isoPatterns[1], "USD"); result.Valid {
		return result
	}
	if result := p.tryPatternEUR(text, p.isoPatterns[2]); result.Valid {
		return result
	}
	if result := p.tryPattern(text, p.isoPatterns[3], "INR"); result.Valid {
		return result
	}

	return AmountResult{Valid: false}
}

// tryPattern attempts to match a pattern and parse the amount.
func (p *AmountParser) tryPattern(text string, pattern *regexp.Regexp, currency string) AmountResult {
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return AmountResult{Valid: false}
	}

	rawAmount := matches[1]
	cents, ok := parseToCents(rawAmount, false)
	if !ok {
		return AmountResult{Valid: false}
	}

	return AmountResult{
		Currency:    currency,
		AmountCents: cents,
		RawMatch:    matches[0],
		Valid:       true,
	}
}

// tryPatternEUR handles European number format (comma as decimal).
func (p *AmountParser) tryPatternEUR(text string, pattern *regexp.Regexp) AmountResult {
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return AmountResult{Valid: false}
	}

	rawAmount := matches[1]
	cents, ok := parseToCents(rawAmount, true)
	if !ok {
		return AmountResult{Valid: false}
	}

	return AmountResult{
		Currency:    "EUR",
		AmountCents: cents,
		RawMatch:    matches[0],
		Valid:       true,
	}
}

// parseToCents converts a number string to cents.
// Uses deterministic rounding (round half up).
// europeanFormat: true means comma is decimal separator (e.g., "12,99" = 12.99)
func parseToCents(s string, europeanFormat bool) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	// Handle European format: 1.234,56 -> 1234.56
	if europeanFormat {
		// Check if it looks like European format
		if strings.Contains(s, ",") && !strings.Contains(s, ".") {
			// Simple case: 12,99 -> 12.99
			s = strings.Replace(s, ",", ".", 1)
		} else if strings.Contains(s, ",") && strings.Contains(s, ".") {
			// Complex case: 1.234,56 -> 1234.56
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		}
	} else {
		// Standard format: remove thousand separators
		s = strings.ReplaceAll(s, ",", "")
	}

	// Parse as float
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}

	// Sanity check: reasonable amount range
	if f < 0 || f > 1000000 { // Max 1 million
		return 0, false
	}

	// Convert to cents with deterministic rounding (round half up)
	cents := int64(f*100 + 0.5)
	return cents, true
}

// ParseAll extracts all currency amounts from text.
// Returns amounts in order of appearance.
func (p *AmountParser) ParseAll(text string) []AmountResult {
	var results []AmountResult
	seen := make(map[string]bool) // Dedupe by raw match

	// Helper to add unique results
	addResult := func(pattern *regexp.Regexp, currency string, european bool) {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			if seen[match[0]] {
				continue
			}

			var cents int64
			var ok bool
			if european {
				cents, ok = parseToCents(match[1], true)
			} else {
				cents, ok = parseToCents(match[1], false)
			}
			if !ok {
				continue
			}

			results = append(results, AmountResult{
				Currency:    currency,
				AmountCents: cents,
				RawMatch:    match[0],
				Valid:       true,
			})
			seen[match[0]] = true
		}
	}

	// Symbol patterns
	addResult(p.symbolPatterns[0], "GBP", false)
	addResult(p.symbolPatterns[1], "USD", false)
	addResult(p.symbolPatterns[2], "EUR", true)
	addResult(p.symbolPatterns[3], "EUR", false)
	addResult(p.symbolPatterns[4], "INR", false)
	addResult(p.symbolPatterns[5], "INR", false)

	// ISO patterns
	addResult(p.isoPatterns[0], "GBP", false)
	addResult(p.isoPatterns[1], "USD", false)
	addResult(p.isoPatterns[2], "EUR", true)
	addResult(p.isoPatterns[3], "INR", false)

	return results
}

// FormatAmount formats cents to a human-readable string.
func FormatAmount(currency string, amountCents int64) string {
	major := amountCents / 100
	minor := amountCents % 100

	symbol := currencySymbol(currency)
	return symbol + formatNumber(major) + "." + padZero(minor)
}

// currencySymbol returns the symbol for a currency code.
func currencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "GBP":
		return "£"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "INR":
		return "₹"
	default:
		return currency + " "
	}
}

// formatNumber formats a number with thousand separators.
func formatNumber(n int64) string {
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}

	s := strconv.FormatInt(n, 10)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// padZero pads a number to 2 digits.
func padZero(n int64) string {
	if n < 0 {
		n = -n
	}
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
