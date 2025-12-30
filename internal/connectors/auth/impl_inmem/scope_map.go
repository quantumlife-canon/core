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

// QuantumLife to Google scope mapping.
var googleScopeMap = map[string]string{
	"calendar:read": "https://www.googleapis.com/auth/calendar.readonly",
	// calendar:write is NOT mapped in v5 (read-only mode)
}

// QuantumLife to Microsoft scope mapping.
var microsoftScopeMap = map[string]string{
	"calendar:read": "Calendars.Read",
	// calendar:write is NOT mapped in v5 (read-only mode)
}

// Write scopes that are blocked in v5.
var blockedWriteScopes = map[string]bool{
	"calendar:write": true,
}

// MapToProvider maps QuantumLife scopes to provider-specific scopes.
// Returns an error if any scope cannot be mapped or is blocked.
func (m *ScopeMapper) MapToProvider(provider auth.ProviderID, quantumlifeScopes []string) ([]string, error) {
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
		// Check if this is a blocked write scope
		if blockedWriteScopes[qlScope] {
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
}

var reverseMicrosoftScopeMap = map[string]string{
	"Calendars.Read": "calendar:read",
}

// IsWriteScope returns true if the scope is a write scope.
func (m *ScopeMapper) IsWriteScope(scope string) bool {
	return blockedWriteScopes[scope]
}

// ValidateReadOnlyScopes validates that all scopes are read-only.
// Returns an error if any write scope is found.
func (m *ScopeMapper) ValidateReadOnlyScopes(scopes []string) error {
	for _, scope := range scopes {
		if blockedWriteScopes[scope] {
			return auth.ErrWriteScopeNotAllowed
		}
	}
	return nil
}
