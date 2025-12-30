// Package auth provides configuration for OAuth providers.
//
// CRITICAL: Secrets are read from environment variables and never logged.
package auth

import (
	"os"
)

// Config holds OAuth configuration for all providers.
// All fields are read from environment variables.
type Config struct {
	// Google OAuth configuration.
	Google GoogleConfig

	// Microsoft OAuth configuration.
	Microsoft MicrosoftConfig

	// TokenEncryptionKey is the key for encrypting stored tokens.
	// PLACEHOLDER: In production, this would be fetched from a key vault.
	TokenEncryptionKey string
}

// GoogleConfig holds Google OAuth configuration.
type GoogleConfig struct {
	// ClientID is the OAuth client ID.
	ClientID string

	// ClientSecret is the OAuth client secret.
	// SENSITIVE: Never log this value.
	ClientSecret string
}

// MicrosoftConfig holds Microsoft OAuth configuration.
type MicrosoftConfig struct {
	// ClientID is the OAuth client ID (Application ID).
	ClientID string

	// ClientSecret is the OAuth client secret.
	// SENSITIVE: Never log this value.
	ClientSecret string

	// TenantID is the Azure AD tenant ID.
	// Use "common" for multi-tenant or "organizations" for work accounts.
	TenantID string
}

// IsConfigured returns true if Google OAuth is configured.
func (c GoogleConfig) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// IsConfigured returns true if Microsoft OAuth is configured.
func (c MicrosoftConfig) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// LoadConfigFromEnv loads configuration from environment variables.
//
// Environment variables:
//   - GOOGLE_CLIENT_ID: Google OAuth client ID
//   - GOOGLE_CLIENT_SECRET: Google OAuth client secret
//   - MICROSOFT_CLIENT_ID: Microsoft OAuth client ID
//   - MICROSOFT_CLIENT_SECRET: Microsoft OAuth client secret
//   - MICROSOFT_TENANT_ID: Microsoft Azure AD tenant ID (default: "common")
//   - TOKEN_ENC_KEY: Token encryption key (placeholder)
func LoadConfigFromEnv() Config {
	tenantID := os.Getenv("MICROSOFT_TENANT_ID")
	if tenantID == "" {
		tenantID = "common" // Default to multi-tenant
	}

	return Config{
		Google: GoogleConfig{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		},
		Microsoft: MicrosoftConfig{
			ClientID:     os.Getenv("MICROSOFT_CLIENT_ID"),
			ClientSecret: os.Getenv("MICROSOFT_CLIENT_SECRET"),
			TenantID:     tenantID,
		},
		TokenEncryptionKey: os.Getenv("TOKEN_ENC_KEY"),
	}
}

// IsProviderConfigured checks if a specific provider is configured.
func (c Config) IsProviderConfigured(provider ProviderID) bool {
	switch provider {
	case ProviderGoogle:
		return c.Google.IsConfigured()
	case ProviderMicrosoft:
		return c.Microsoft.IsConfigured()
	default:
		return false
	}
}

// ConfiguredProviders returns a list of configured providers.
func (c Config) ConfiguredProviders() []ProviderID {
	var providers []ProviderID
	if c.Google.IsConfigured() {
		providers = append(providers, ProviderGoogle)
	}
	if c.Microsoft.IsConfigured() {
		providers = append(providers, ProviderMicrosoft)
	}
	return providers
}

// Google OAuth endpoints.
const (
	GoogleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
)

// Microsoft OAuth endpoints.
// TenantID is interpolated into the URL.
const (
	MicrosoftAuthURLTemplate  = "https://login.microsoftonline.com/%s/oauth2/v2/authorize"
	MicrosoftTokenURLTemplate = "https://login.microsoftonline.com/%s/oauth2/v2/token"
)
