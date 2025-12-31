// Package impl_inmem provides in-memory implementation of the token broker.
// This file defines the scope mapping between QuantumLife and provider scopes.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package impl_inmem

import (
	"quantumlife/internal/connectors/auth"
)

// ScopeMapper maps QuantumLife scopes to provider-specific scopes.
type ScopeMapper struct{}

// NewScopeMapper creates a new scope mapper.
func NewScopeMapper() *ScopeMapper {
	return &ScopeMapper{}
}

// QuantumLife to Google scope mapping (v6: includes write scopes).
// Phase 1: Added email:read for Gmail API read-only access.
var googleScopeMap = map[string]string{
	"calendar:read":  "https://www.googleapis.com/auth/calendar.readonly",
	"calendar:write": "https://www.googleapis.com/auth/calendar.events",
	"email:read":     "https://www.googleapis.com/auth/gmail.readonly",
}

// QuantumLife to Microsoft scope mapping (v6: includes write scopes).
var microsoftScopeMap = map[string]string{
	"calendar:read":  "Calendars.Read",
	"calendar:write": "Calendars.ReadWrite",
}

// QuantumLife to TrueLayer scope mapping (v8.2: READ-ONLY).
// CRITICAL: Only read scopes are mapped. No payment or write scopes exist.
var truelayerScopeMap = map[string]string{
	"finance:read": "accounts balance transactions offline_access",
}

// Individual TrueLayer scopes for granular mapping.
var truelayerGranularScopes = map[string][]string{
	"finance:read":     {"accounts", "balance", "transactions", "offline_access"},
	"finance:accounts": {"accounts", "offline_access"},
	"finance:balance":  {"balance", "offline_access"},
	"finance:txn":      {"transactions", "offline_access"},
}

// QuantumLife to Plaid products mapping (v8.3: READ-ONLY).
// CRITICAL: Only read products are mapped. No payment or transfer products exist.
var plaidProductsMap = map[string][]string{
	"finance:read":         {"transactions"},
	"finance:transactions": {"transactions"},
	"finance:auth":         {"auth"},
	"finance:identity":     {"identity"},
	"finance:investments":  {"investments"},
	"finance:liabilities":  {"liabilities"},
}

// Write scopes that require Execute mode and explicit approval.
// In v6, these are no longer blocked but require special handling.
var executeOnlyScopes = map[string]bool{
	"calendar:write": true,
}

// MapToProvider maps QuantumLife scopes to provider-specific scopes.
// Returns an error if any scope cannot be mapped.
// NOTE: In v6, write scopes are allowed but require Execute mode validation elsewhere.
func (m *ScopeMapper) MapToProvider(provider auth.ProviderID, quantumlifeScopes []string) ([]string, error) {
	return m.MapToProviderWithMode(provider, quantumlifeScopes, false)
}

// MapToProviderWithMode maps scopes with explicit Execute mode flag.
// When executeMode is false, write scopes are rejected.
// When executeMode is true, write scopes are allowed.
func (m *ScopeMapper) MapToProviderWithMode(provider auth.ProviderID, quantumlifeScopes []string, executeMode bool) ([]string, error) {
	// Special handling for TrueLayer (uses array scopes)
	if provider == auth.ProviderTrueLayer {
		return m.mapTrueLayerScopes(quantumlifeScopes)
	}

	// Special handling for Plaid (uses products, not OAuth scopes)
	if provider == auth.ProviderPlaid {
		return m.mapPlaidProducts(quantumlifeScopes)
	}

	var scopeMap map[string]string
	switch provider {
	case auth.ProviderGoogle:
		scopeMap = googleScopeMap
	case auth.ProviderMicrosoft:
		scopeMap = microsoftScopeMap
	default:
		return nil, auth.ErrInvalidProvider
	}

	var providerScopes []string
	for _, qlScope := range quantumlifeScopes {
		// Check if this is an execute-only scope
		if executeOnlyScopes[qlScope] && !executeMode {
			return nil, auth.ErrWriteScopeNotAllowed
		}

		// Map to provider scope
		providerScope, ok := scopeMap[qlScope]
		if !ok {
			// Unknown scope - skip (could also error)
			continue
		}
		providerScopes = append(providerScopes, providerScope)
	}

	// Add base scopes required by each provider
	switch provider {
	case auth.ProviderGoogle:
		// Google requires openid for some flows
		// For calendar API, the specific scope is enough
	case auth.ProviderMicrosoft:
		// Microsoft Graph requires offline_access for refresh tokens
		providerScopes = append(providerScopes, "offline_access")
	}

	return providerScopes, nil
}

// mapTrueLayerScopes maps QuantumLife finance scopes to TrueLayer scopes.
// CRITICAL: Only read scopes are allowed. Payment scopes are rejected.
func (m *ScopeMapper) mapTrueLayerScopes(quantumlifeScopes []string) ([]string, error) {
	scopeSet := make(map[string]bool)

	for _, qlScope := range quantumlifeScopes {
		// Check if this is a finance scope
		granularScopes, ok := truelayerGranularScopes[qlScope]
		if !ok {
			// Unknown scope - skip
			continue
		}
		for _, s := range granularScopes {
			scopeSet[s] = true
		}
	}

	// Convert set to slice
	var providerScopes []string
	for scope := range scopeSet {
		providerScopes = append(providerScopes, scope)
	}

	return providerScopes, nil
}

// mapPlaidProducts maps QuantumLife finance scopes to Plaid products.
// CRITICAL: Only read products are allowed. Payment/transfer products are rejected.
func (m *ScopeMapper) mapPlaidProducts(quantumlifeScopes []string) ([]string, error) {
	productSet := make(map[string]bool)

	for _, qlScope := range quantumlifeScopes {
		// Check if this is a finance scope
		products, ok := plaidProductsMap[qlScope]
		if !ok {
			// Unknown scope - skip
			continue
		}
		for _, p := range products {
			productSet[p] = true
		}
	}

	// Convert set to slice
	var providerProducts []string
	for product := range productSet {
		providerProducts = append(providerProducts, product)
	}

	return providerProducts, nil
}

// IsExecuteOnlyScope returns true if the scope requires Execute mode.
func (m *ScopeMapper) IsExecuteOnlyScope(scope string) bool {
	return executeOnlyScopes[scope]
}

// MapFromProvider maps provider scopes back to QuantumLife scopes.
// This is used when displaying what scopes were granted.
func (m *ScopeMapper) MapFromProvider(provider auth.ProviderID, providerScopes []string) []string {
	// Special handling for TrueLayer
	if provider == auth.ProviderTrueLayer {
		return m.mapFromTrueLayer(providerScopes)
	}

	// Special handling for Plaid
	if provider == auth.ProviderPlaid {
		return m.mapFromPlaid(providerScopes)
	}

	var reverseMap map[string]string
	switch provider {
	case auth.ProviderGoogle:
		reverseMap = reverseGoogleScopeMap
	case auth.ProviderMicrosoft:
		reverseMap = reverseMicrosoftScopeMap
	default:
		return nil
	}

	var qlScopes []string
	for _, ps := range providerScopes {
		if qs, ok := reverseMap[ps]; ok {
			qlScopes = append(qlScopes, qs)
		}
	}

	return qlScopes
}

// mapFromTrueLayer maps TrueLayer scopes back to QuantumLife scopes.
func (m *ScopeMapper) mapFromTrueLayer(providerScopes []string) []string {
	// Check if any finance read indicator scope is present
	for _, ps := range providerScopes {
		for _, indicator := range reverseTrueLayerScopeIndicators {
			if ps == indicator {
				return []string{"finance:read"}
			}
		}
	}
	return nil
}

// Reverse mappings (provider -> QuantumLife).
var reverseGoogleScopeMap = map[string]string{
	"https://www.googleapis.com/auth/calendar.readonly": "calendar:read",
	"https://www.googleapis.com/auth/calendar.events":   "calendar:write",
	"https://www.googleapis.com/auth/gmail.readonly":    "email:read",
}

var reverseMicrosoftScopeMap = map[string]string{
	"Calendars.Read":      "calendar:read",
	"Calendars.ReadWrite": "calendar:write",
}

// TrueLayer scopes that indicate finance:read access (v8.2).
// Any of these scopes grants the finance:read capability.
var reverseTrueLayerScopeIndicators = []string{
	"accounts", "balance", "transactions",
}

// Plaid products that indicate finance:read access (v8.3).
// Any of these products grants the finance:read capability.
var reversePlaidProductIndicators = []string{
	"transactions", "auth", "identity", "investments", "liabilities",
}

// mapFromPlaid maps Plaid products back to QuantumLife scopes.
func (m *ScopeMapper) mapFromPlaid(providerProducts []string) []string {
	// Check if any finance read indicator product is present
	for _, product := range providerProducts {
		for _, indicator := range reversePlaidProductIndicators {
			if product == indicator {
				return []string{"finance:read"}
			}
		}
	}
	return nil
}

// IsWriteScope returns true if the scope is a write scope.
func (m *ScopeMapper) IsWriteScope(scope string) bool {
	return executeOnlyScopes[scope]
}

// ValidateReadOnlyScopes validates that all scopes are read-only.
// Returns an error if any write scope is found.
func (m *ScopeMapper) ValidateReadOnlyScopes(scopes []string) error {
	for _, scope := range scopes {
		if executeOnlyScopes[scope] {
			return auth.ErrWriteScopeNotAllowed
		}
	}
	return nil
}
