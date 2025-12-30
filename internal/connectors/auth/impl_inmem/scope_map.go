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
var googleScopeMap = map[string]string{
	"calendar:read":  "https://www.googleapis.com/auth/calendar.readonly",
	"calendar:write": "https://www.googleapis.com/auth/calendar.events",
}

// QuantumLife to Microsoft scope mapping (v6: includes write scopes).
var microsoftScopeMap = map[string]string{
	"calendar:read":  "Calendars.Read",
	"calendar:write": "Calendars.ReadWrite",
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

// IsExecuteOnlyScope returns true if the scope requires Execute mode.
func (m *ScopeMapper) IsExecuteOnlyScope(scope string) bool {
	return executeOnlyScopes[scope]
}

// MapFromProvider maps provider scopes back to QuantumLife scopes.
// This is used when displaying what scopes were granted.
func (m *ScopeMapper) MapFromProvider(provider auth.ProviderID, providerScopes []string) []string {
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

// Reverse mappings (provider -> QuantumLife).
var reverseGoogleScopeMap = map[string]string{
	"https://www.googleapis.com/auth/calendar.readonly": "calendar:read",
	"https://www.googleapis.com/auth/calendar.events":   "calendar:write",
}

var reverseMicrosoftScopeMap = map[string]string{
	"Calendars.Read":      "calendar:read",
	"Calendars.ReadWrite": "calendar:write",
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
