// Package extract provides email-to-commerce event extraction.
//
// CRITICAL: Vendor-specific pattern matching is isolated here.
// CRITICAL: Maps to canonical vendor names, not vendor-specific logic.
// CRITICAL: Deterministic matching order for reproducibility.
//
// Reference: docs/ADR/ADR-0024-phase8-commerce-mirror-email-derived.md
package extract

import (
	"regexp"
	"sort"
	"strings"

	"quantumlife/pkg/domain/commerce"
)

// VendorInfo contains canonical vendor information.
type VendorInfo struct {
	CanonicalName string
	Category      commerce.CommerceCategory
	Domains       []string // Known domains
}

// VendorRegistry maps domains to vendor information.
// Order is deterministic: sorted by domain.
var VendorRegistry = map[string]VendorInfo{
	// UK Food Delivery
	"deliveroo.co.uk": {CanonicalName: "Deliveroo", Category: commerce.CategoryFoodDelivery, Domains: []string{"deliveroo.co.uk", "deliveroo.com"}},
	"deliveroo.com":   {CanonicalName: "Deliveroo", Category: commerce.CategoryFoodDelivery, Domains: []string{"deliveroo.co.uk", "deliveroo.com"}},
	"just-eat.co.uk":  {CanonicalName: "Just Eat", Category: commerce.CategoryFoodDelivery, Domains: []string{"just-eat.co.uk", "just-eat.com"}},
	"just-eat.com":    {CanonicalName: "Just Eat", Category: commerce.CategoryFoodDelivery, Domains: []string{"just-eat.co.uk", "just-eat.com"}},

	// US Food Delivery
	"uber.com":     {CanonicalName: "Uber", Category: commerce.CategoryRideHailing, Domains: []string{"uber.com"}},
	"ubereats.com": {CanonicalName: "Uber Eats", Category: commerce.CategoryFoodDelivery, Domains: []string{"ubereats.com"}},
	"doordash.com": {CanonicalName: "DoorDash", Category: commerce.CategoryFoodDelivery, Domains: []string{"doordash.com"}},
	"grubhub.com":  {CanonicalName: "Grubhub", Category: commerce.CategoryFoodDelivery, Domains: []string{"grubhub.com"}},

	// India Food Delivery
	"swiggy.com": {CanonicalName: "Swiggy", Category: commerce.CategoryFoodDelivery, Domains: []string{"swiggy.com", "swiggy.in"}},
	"swiggy.in":  {CanonicalName: "Swiggy", Category: commerce.CategoryFoodDelivery, Domains: []string{"swiggy.com", "swiggy.in"}},
	"zomato.com": {CanonicalName: "Zomato", Category: commerce.CategoryFoodDelivery, Domains: []string{"zomato.com"}},

	// UK Grocery
	"tesco.com":        {CanonicalName: "Tesco", Category: commerce.CategoryGrocery, Domains: []string{"tesco.com"}},
	"sainsburys.co.uk": {CanonicalName: "Sainsburys", Category: commerce.CategoryGrocery, Domains: []string{"sainsburys.co.uk"}},
	"ocado.com":        {CanonicalName: "Ocado", Category: commerce.CategoryGrocery, Domains: []string{"ocado.com"}},
	"asda.com":         {CanonicalName: "Asda", Category: commerce.CategoryGrocery, Domains: []string{"asda.com"}},
	"waitrose.com":     {CanonicalName: "Waitrose", Category: commerce.CategoryGrocery, Domains: []string{"waitrose.com"}},

	// UK Courier/Delivery
	"dpd.co.uk":           {CanonicalName: "DPD", Category: commerce.CategoryCourier, Domains: []string{"dpd.co.uk", "dpdlocal.co.uk"}},
	"dpdlocal.co.uk":      {CanonicalName: "DPD", Category: commerce.CategoryCourier, Domains: []string{"dpd.co.uk", "dpdlocal.co.uk"}},
	"royalmail.com":       {CanonicalName: "Royal Mail", Category: commerce.CategoryCourier, Domains: []string{"royalmail.com"}},
	"parcelforce.com":     {CanonicalName: "Parcelforce", Category: commerce.CategoryCourier, Domains: []string{"parcelforce.com"}},
	"hermes-europe.co.uk": {CanonicalName: "Evri", Category: commerce.CategoryCourier, Domains: []string{"hermes-europe.co.uk", "evri.com"}},
	"evri.com":            {CanonicalName: "Evri", Category: commerce.CategoryCourier, Domains: []string{"hermes-europe.co.uk", "evri.com"}},
	"yodel.co.uk":         {CanonicalName: "Yodel", Category: commerce.CategoryCourier, Domains: []string{"yodel.co.uk"}},

	// US Courier
	"ups.com":   {CanonicalName: "UPS", Category: commerce.CategoryCourier, Domains: []string{"ups.com"}},
	"fedex.com": {CanonicalName: "FedEx", Category: commerce.CategoryCourier, Domains: []string{"fedex.com"}},
	"usps.com":  {CanonicalName: "USPS", Category: commerce.CategoryCourier, Domains: []string{"usps.com"}},

	// India Courier
	"delhivery.com":  {CanonicalName: "Delhivery", Category: commerce.CategoryCourier, Domains: []string{"delhivery.com"}},
	"bluedart.com":   {CanonicalName: "Blue Dart", Category: commerce.CategoryCourier, Domains: []string{"bluedart.com"}},
	"ecomexpress.in": {CanonicalName: "Ecom Express", Category: commerce.CategoryCourier, Domains: []string{"ecomexpress.in"}},

	// Global Retail
	"amazon.com":   {CanonicalName: "Amazon", Category: commerce.CategoryRetail, Domains: []string{"amazon.com", "amazon.co.uk", "amazon.in", "amazon.de", "amazon.fr"}},
	"amazon.co.uk": {CanonicalName: "Amazon", Category: commerce.CategoryRetail, Domains: []string{"amazon.com", "amazon.co.uk", "amazon.in", "amazon.de", "amazon.fr"}},
	"amazon.in":    {CanonicalName: "Amazon", Category: commerce.CategoryRetail, Domains: []string{"amazon.com", "amazon.co.uk", "amazon.in", "amazon.de", "amazon.fr"}},
	"ebay.com":     {CanonicalName: "eBay", Category: commerce.CategoryRetail, Domains: []string{"ebay.com", "ebay.co.uk"}},
	"ebay.co.uk":   {CanonicalName: "eBay", Category: commerce.CategoryRetail, Domains: []string{"ebay.com", "ebay.co.uk"}},

	// India Retail
	"flipkart.com": {CanonicalName: "Flipkart", Category: commerce.CategoryRetail, Domains: []string{"flipkart.com"}},
	"myntra.com":   {CanonicalName: "Myntra", Category: commerce.CategoryRetail, Domains: []string{"myntra.com"}},

	// Ride Hailing
	"lyft.com":    {CanonicalName: "Lyft", Category: commerce.CategoryRideHailing, Domains: []string{"lyft.com"}},
	"ola.com":     {CanonicalName: "Ola", Category: commerce.CategoryRideHailing, Domains: []string{"ola.com", "olacabs.com"}},
	"olacabs.com": {CanonicalName: "Ola", Category: commerce.CategoryRideHailing, Domains: []string{"ola.com", "olacabs.com"}},
	"bolt.eu":     {CanonicalName: "Bolt", Category: commerce.CategoryRideHailing, Domains: []string{"bolt.eu"}},

	// Subscriptions
	"netflix.com": {CanonicalName: "Netflix", Category: commerce.CategorySubscriptions, Domains: []string{"netflix.com"}},
	"spotify.com": {CanonicalName: "Spotify", Category: commerce.CategorySubscriptions, Domains: []string{"spotify.com"}},
	"apple.com":   {CanonicalName: "Apple", Category: commerce.CategorySubscriptions, Domains: []string{"apple.com"}},
	"google.com":  {CanonicalName: "Google", Category: commerce.CategorySubscriptions, Domains: []string{"google.com"}},

	// UK Utilities
	"britishgas.co.uk": {CanonicalName: "British Gas", Category: commerce.CategoryUtilities, Domains: []string{"britishgas.co.uk"}},
	"edf.co.uk":        {CanonicalName: "EDF", Category: commerce.CategoryUtilities, Domains: []string{"edf.co.uk"}},
	"eon.co.uk":        {CanonicalName: "E.ON", Category: commerce.CategoryUtilities, Domains: []string{"eon.co.uk"}},
	"octopus.energy":   {CanonicalName: "Octopus Energy", Category: commerce.CategoryUtilities, Domains: []string{"octopus.energy"}},
}

// VendorMatcher provides deterministic vendor detection from email signals.
type VendorMatcher struct {
	domainIndex   map[string]VendorInfo
	sortedDomains []string
}

// NewVendorMatcher creates a vendor matcher with sorted domain index.
func NewVendorMatcher() *VendorMatcher {
	m := &VendorMatcher{
		domainIndex: make(map[string]VendorInfo),
	}

	// Copy registry to index
	for domain, info := range VendorRegistry {
		m.domainIndex[strings.ToLower(domain)] = info
	}

	// Build sorted domain list for deterministic iteration
	for domain := range m.domainIndex {
		m.sortedDomains = append(m.sortedDomains, domain)
	}
	sort.Strings(m.sortedDomains)

	return m
}

// VendorMatchResult contains vendor detection result.
type VendorMatchResult struct {
	CanonicalName string
	Category      commerce.CommerceCategory
	Domain        string
	Matched       bool
}

// MatchByDomain attempts to match vendor by sender domain.
func (m *VendorMatcher) MatchByDomain(senderDomain string) VendorMatchResult {
	domain := strings.ToLower(strings.TrimSpace(senderDomain))

	if info, ok := m.domainIndex[domain]; ok {
		return VendorMatchResult{
			CanonicalName: info.CanonicalName,
			Category:      info.Category,
			Domain:        domain,
			Matched:       true,
		}
	}

	// Try matching subdomains (e.g., "email.amazon.co.uk" -> "amazon.co.uk")
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts)-1; i++ {
		subdomain := strings.Join(parts[i:], ".")
		if info, ok := m.domainIndex[subdomain]; ok {
			return VendorMatchResult{
				CanonicalName: info.CanonicalName,
				Category:      info.Category,
				Domain:        subdomain,
				Matched:       true,
			}
		}
	}

	return VendorMatchResult{Matched: false}
}

// MatchBySubjectPatterns attempts to match vendor by subject line patterns.
// This is a fallback when domain matching fails.
func (m *VendorMatcher) MatchBySubjectPatterns(subject string) VendorMatchResult {
	subjectLower := strings.ToLower(subject)

	// Deterministic order: iterate through sorted vendor names
	patterns := []struct {
		pattern  *regexp.Regexp
		vendor   string
		category commerce.CommerceCategory
	}{
		{regexp.MustCompile(`\bamazon\b`), "Amazon", commerce.CategoryRetail},
		{regexp.MustCompile(`\bdeliveroo\b`), "Deliveroo", commerce.CategoryFoodDelivery},
		{regexp.MustCompile(`\buber\s*eats\b`), "Uber Eats", commerce.CategoryFoodDelivery},
		{regexp.MustCompile(`\buber\b`), "Uber", commerce.CategoryRideHailing},
		{regexp.MustCompile(`\bdpd\b`), "DPD", commerce.CategoryCourier},
		{regexp.MustCompile(`\broyal\s*mail\b`), "Royal Mail", commerce.CategoryCourier},
		{regexp.MustCompile(`\btesco\b`), "Tesco", commerce.CategoryGrocery},
		{regexp.MustCompile(`\bsainsbury'?s\b`), "Sainsburys", commerce.CategoryGrocery},
		{regexp.MustCompile(`\bswiggy\b`), "Swiggy", commerce.CategoryFoodDelivery},
		{regexp.MustCompile(`\bzomato\b`), "Zomato", commerce.CategoryFoodDelivery},
		{regexp.MustCompile(`\bflipkart\b`), "Flipkart", commerce.CategoryRetail},
		{regexp.MustCompile(`\bdelhivery\b`), "Delhivery", commerce.CategoryCourier},
		{regexp.MustCompile(`\bnetflix\b`), "Netflix", commerce.CategorySubscriptions},
		{regexp.MustCompile(`\bspotify\b`), "Spotify", commerce.CategorySubscriptions},
	}

	for _, p := range patterns {
		if p.pattern.MatchString(subjectLower) {
			return VendorMatchResult{
				CanonicalName: p.vendor,
				Category:      p.category,
				Matched:       true,
			}
		}
	}

	return VendorMatchResult{Matched: false}
}

// FallbackVendorFromDomain creates a vendor name from an unknown domain.
// Used when no specific vendor match is found.
func FallbackVendorFromDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "Unknown"
	}

	// Remove common email subdomains
	domain = strings.TrimPrefix(domain, "mail.")
	domain = strings.TrimPrefix(domain, "email.")
	domain = strings.TrimPrefix(domain, "notify.")
	domain = strings.TrimPrefix(domain, "noreply.")
	domain = strings.TrimPrefix(domain, "no-reply.")

	// Extract main domain name (before TLD)
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		// Use the main domain part (e.g., "amazon" from "amazon.co.uk")
		mainPart := parts[0]
		if len(parts) > 2 && (parts[len(parts)-2] == "co" || parts[len(parts)-2] == "com") {
			mainPart = parts[len(parts)-3]
		}
		// Capitalize first letter
		if len(mainPart) > 0 {
			return strings.ToUpper(string(mainPart[0])) + mainPart[1:]
		}
	}

	return domain
}

// IsCommerceRelatedDomain checks if a domain is likely commerce-related.
// Used to filter out non-commerce emails before extraction.
func IsCommerceRelatedDomain(domain string) bool {
	domain = strings.ToLower(domain)

	// Check known vendors
	if _, ok := VendorRegistry[domain]; ok {
		return true
	}

	// Check for commerce-related substrings
	commerceHints := []string{
		"order", "receipt", "invoice", "payment", "delivery",
		"shipping", "track", "confirm", "purchase", "subscribe",
		"renew", "billing", "shop", "store", "buy", "cart",
	}

	for _, hint := range commerceHints {
		if strings.Contains(domain, hint) {
			return true
		}
	}

	return false
}
