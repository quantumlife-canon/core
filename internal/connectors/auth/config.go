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

	// TrueLayer OAuth configuration (v8.2).
	TrueLayer TrueLayerConfig

	// Plaid configuration (v8.3).
	Plaid PlaidConfig

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

// TrueLayerConfig holds TrueLayer OAuth configuration (v8.2).
// CRITICAL: TrueLayer is READ-ONLY. No payment scopes are allowed.
type TrueLayerConfig struct {
	// ClientID is the TrueLayer client ID.
	ClientID string

	// ClientSecret is the TrueLayer client secret.
	// SENSITIVE: Never log this value.
	ClientSecret string

	// Environment is "sandbox" or "live".
	// Default: sandbox (for safety).
	Environment string

	// RedirectURL is the default redirect URL (optional).
	RedirectURL string
}

// IsConfigured returns true if Google OAuth is configured.
func (c GoogleConfig) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// IsConfigured returns true if Microsoft OAuth is configured.
func (c MicrosoftConfig) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// IsConfigured returns true if TrueLayer OAuth is configured.
func (c TrueLayerConfig) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// PlaidConfig holds Plaid configuration (v8.3).
// CRITICAL: Plaid is READ-ONLY. No payment products are allowed.
type PlaidConfig struct {
	// ClientID is the Plaid client ID.
	ClientID string

	// Secret is the Plaid secret.
	// SENSITIVE: Never log this value.
	Secret string

	// Environment is "sandbox", "development", or "production".
	// Default: sandbox (for safety).
	Environment string
}

// IsConfigured returns true if Plaid is configured.
func (c PlaidConfig) IsConfigured() bool {
	return c.ClientID != "" && c.Secret != ""
}

// LoadConfigFromEnv loads configuration from environment variables.
//
// Environment variables:
//   - GOOGLE_CLIENT_ID: Google OAuth client ID
//   - GOOGLE_CLIENT_SECRET: Google OAuth client secret
//   - MICROSOFT_CLIENT_ID: Microsoft OAuth client ID
//   - MICROSOFT_CLIENT_SECRET: Microsoft OAuth client secret
//   - MICROSOFT_TENANT_ID: Microsoft Azure AD tenant ID (default: "common")
//   - TRUELAYER_CLIENT_ID: TrueLayer client ID (v8.2)
//   - TRUELAYER_CLIENT_SECRET: TrueLayer client secret (v8.2)
//   - TRUELAYER_ENV: TrueLayer environment (sandbox|live, default: sandbox)
//   - TRUELAYER_REDIRECT_URL: TrueLayer default redirect URL (optional)
//   - PLAID_CLIENT_ID: Plaid client ID (v8.3)
//   - PLAID_SECRET: Plaid secret (v8.3)
//   - PLAID_ENV: Plaid environment (sandbox|development|production, default: sandbox)
//   - TOKEN_ENC_KEY: Token encryption key (placeholder)
func LoadConfigFromEnv() Config {
	tenantID := os.Getenv("MICROSOFT_TENANT_ID")
	if tenantID == "" {
		tenantID = "common" // Default to multi-tenant
	}

	trueLayerEnv := os.Getenv("TRUELAYER_ENV")
	if trueLayerEnv == "" {
		trueLayerEnv = "sandbox" // Default to sandbox for safety
	}

	plaidEnv := os.Getenv("PLAID_ENV")
	if plaidEnv == "" {
		plaidEnv = "sandbox" // Default to sandbox for safety
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
		TrueLayer: TrueLayerConfig{
			ClientID:     os.Getenv("TRUELAYER_CLIENT_ID"),
			ClientSecret: os.Getenv("TRUELAYER_CLIENT_SECRET"),
			Environment:  trueLayerEnv,
			RedirectURL:  os.Getenv("TRUELAYER_REDIRECT_URL"),
		},
		Plaid: PlaidConfig{
			ClientID:    os.Getenv("PLAID_CLIENT_ID"),
			Secret:      os.Getenv("PLAID_SECRET"),
			Environment: plaidEnv,
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
	case ProviderTrueLayer:
		return c.TrueLayer.IsConfigured()
	case ProviderPlaid:
		return c.Plaid.IsConfigured()
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
	if c.TrueLayer.IsConfigured() {
		providers = append(providers, ProviderTrueLayer)
	}
	if c.Plaid.IsConfigured() {
		providers = append(providers, ProviderPlaid)
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

// TrueLayer OAuth endpoints (v8.2).
// CRITICAL: Only read-only scopes are allowed.
const (
	// Sandbox endpoints (default for safety)
	TrueLayerSandboxAuthURL  = "https://auth.truelayer-sandbox.com"
	TrueLayerSandboxTokenURL = "https://auth.truelayer-sandbox.com/connect/token"

	// Production endpoints
	TrueLayerLiveAuthURL  = "https://auth.truelayer.com"
	TrueLayerLiveTokenURL = "https://auth.truelayer.com/connect/token"
)

// TrueLayerAuthURL returns the TrueLayer auth URL for the configured environment.
func (c TrueLayerConfig) TrueLayerAuthURL() string {
	if c.Environment == "live" || c.Environment == "production" {
		return TrueLayerLiveAuthURL
	}
	return TrueLayerSandboxAuthURL
}

// TrueLayerTokenURL returns the TrueLayer token URL for the configured environment.
func (c TrueLayerConfig) TrueLayerTokenURL() string {
	if c.Environment == "live" || c.Environment == "production" {
		return TrueLayerLiveTokenURL
	}
	return TrueLayerSandboxTokenURL
}
