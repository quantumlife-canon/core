// Package config provides multi-circle configuration types for QuantumLife.
//
// Configuration Format (.qlconf):
// A simple, line-based format parsed with stdlib only.
// No external dependencies (YAML, JSON libs) required.
//
// Format:
//
//	[circle:<id>]
//	name = <display name>
//	email = <provider>:<identifier>:<scopes>
//	calendar = <provider>:<calendar_id>:<scopes>
//	finance = <provider>:<identifier>:<scopes>
//
//	[routing]
//	work_domains = domain1.com, domain2.com
//	personal_domains = gmail.com, outlook.com
//	vip_senders = alice@work.com, bob@work.com
//
// Example:
//
//	[circle:work]
//	name = Work
//	email = google:work@company.com:email:read
//	calendar = google:primary:calendar:read
//
//	[circle:personal]
//	name = Personal
//	email = google:me@gmail.com:email:read
//	calendar = google:primary:calendar:read
//
//	[routing]
//	work_domains = company.com, corp.company.com
//	personal_domains = gmail.com, yahoo.com
//
// Reference: docs/ADR/ADR-0026-phase11-multicircle-real-loop.md
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// CircleConfig defines a single circle's configuration.
type CircleConfig struct {
	// ID is the unique circle identifier.
	ID identity.EntityID

	// Name is the human-readable display name.
	Name string

	// EmailIntegrations lists email integration configurations.
	EmailIntegrations []EmailIntegration

	// CalendarIntegrations lists calendar integration configurations.
	CalendarIntegrations []CalendarIntegration

	// FinanceIntegrations lists finance integration configurations.
	FinanceIntegrations []FinanceIntegration
}

// EmailIntegration defines an email integration.
type EmailIntegration struct {
	// Provider is the email provider (google, microsoft).
	Provider string

	// Identifier is the email address or integration identifier.
	Identifier string

	// Scopes are the required OAuth scopes.
	Scopes []string
}

// CalendarIntegration defines a calendar integration.
type CalendarIntegration struct {
	// Provider is the calendar provider (google, microsoft).
	Provider string

	// CalendarID is the calendar identifier (e.g., "primary").
	CalendarID string

	// Scopes are the required OAuth scopes.
	Scopes []string
}

// FinanceIntegration defines a finance integration (read-only).
type FinanceIntegration struct {
	// Provider is the finance provider (plaid, truelayer).
	Provider string

	// Identifier is the integration identifier.
	Identifier string

	// Scopes are the required OAuth scopes.
	Scopes []string
}

// RoutingConfig defines event routing rules.
type RoutingConfig struct {
	// WorkDomains are email domains that route to work circle.
	WorkDomains []string

	// PersonalDomains are email domains that route to personal circle.
	PersonalDomains []string

	// VIPSenders are email addresses that should be prioritized.
	VIPSenders []string

	// FamilyMembers are email addresses that route to family circle.
	FamilyMembers []string
}

// MultiCircleConfig is the complete configuration for multi-circle operation.
type MultiCircleConfig struct {
	// Circles contains all circle configurations, keyed by circle ID.
	Circles map[identity.EntityID]*CircleConfig

	// Routing contains event routing rules.
	Routing RoutingConfig

	// Shadow contains shadow-mode configuration (Phase 19).
	Shadow ShadowConfig

	// LoadedAt is when the config was loaded.
	LoadedAt time.Time

	// SourcePath is the path to the config file.
	SourcePath string

	// canonicalHash is the deterministic hash of the config.
	canonicalHash string
}

// ShadowConfig contains shadow-mode configuration.
//
// Phase 19: LLM Shadow-Mode Contract
// Phase 19.3: Azure OpenAI Shadow Provider
// Phase 19.3c: Real Azure Chat Shadow Run
//
// CRITICAL: Shadow mode is OFF by default.
// CRITICAL: Shadow mode emits METADATA ONLY - never content.
// CRITICAL: Shadow mode NEVER affects UI, obligations, drafts, or execution.
// CRITICAL: Real providers require explicit opt-in (RealAllowed=true) + consent.
type ShadowConfig struct {
	// Mode is the shadow-mode operation mode.
	// Valid values: "off" (default), "observe"
	Mode string

	// ModelName is the shadow model to use.
	// Valid values: "stub" (default), "azure_openai", "local_slm" (placeholder)
	ModelName string

	// Phase 19.3: Provider configuration
	// ProviderKind identifies the provider type.
	// Valid values: "none", "stub" (default), "azure_openai", "local_slm"
	ProviderKind string

	// RealAllowed indicates if real (non-stub) providers are permitted.
	// CRITICAL: Default is false. Must be explicitly enabled.
	RealAllowed bool

	// Phase 19.3c: MaxSuggestions limits suggestions per shadow run.
	// Default: 3. Clamped to 1-5.
	MaxSuggestions int

	// AzureOpenAI contains Azure OpenAI provider configuration.
	// Only used when ProviderKind="azure_openai" and RealAllowed=true.
	AzureOpenAI AzureOpenAIConfig
}

// AzureOpenAIConfig contains Azure OpenAI provider settings.
//
// Phase 19.3b: Extended with embeddings deployment and explicit env var names.
//
// CRITICAL: API key should come from environment variable, not config file.
// Config file only stores endpoint and deployment names.
type AzureOpenAIConfig struct {
	// Endpoint is the Azure OpenAI endpoint URL.
	// Example: "https://your-resource.openai.azure.com"
	// Read from AZURE_OPENAI_ENDPOINT env var if empty.
	Endpoint string

	// Deployment is the chat model deployment name (alias for ChatDeployment).
	// Example: "gpt-4o-mini"
	// Read from AZURE_OPENAI_DEPLOYMENT env var if empty.
	Deployment string

	// ChatDeployment is the chat model deployment name.
	// Takes precedence over Deployment if both are set.
	// Read from AZURE_OPENAI_CHAT_DEPLOYMENT env var if empty.
	ChatDeployment string

	// EmbedDeployment is the embeddings model deployment name.
	// Optional - only required for embeddings healthcheck.
	// Read from AZURE_OPENAI_EMBED_DEPLOYMENT env var if empty.
	EmbedDeployment string

	// APIVersion is the Azure OpenAI API version.
	// Default: "2024-02-15-preview"
	// Read from AZURE_OPENAI_API_VERSION env var if empty.
	APIVersion string

	// APIKeyEnvName is the environment variable name containing the API key.
	// Default: "AZURE_OPENAI_API_KEY"
	// CRITICAL: Never store actual keys in config files.
	APIKeyEnvName string
}

// GetChatDeployment returns the effective chat deployment name.
func (c *AzureOpenAIConfig) GetChatDeployment() string {
	if c.ChatDeployment != "" {
		return c.ChatDeployment
	}
	return c.Deployment
}

// GetAPIKeyEnvName returns the effective API key environment variable name.
func (c *AzureOpenAIConfig) GetAPIKeyEnvName() string {
	if c.APIKeyEnvName != "" {
		return c.APIKeyEnvName
	}
	return DefaultAzureAPIKeyEnvName
}

// HasEmbeddings returns true if embeddings deployment is configured.
func (c *AzureOpenAIConfig) HasEmbeddings() bool {
	return c.EmbedDeployment != ""
}

// CanonicalString returns a deterministic string representation.
// CRITICAL: Never includes actual API keys.
func (c *AzureOpenAIConfig) CanonicalString() string {
	return "AZURE_CONFIG|v1|endpoint:" + c.Endpoint +
		"|chat:" + c.GetChatDeployment() +
		"|embed:" + c.EmbedDeployment +
		"|api_version:" + c.APIVersion +
		"|key_env:" + c.GetAPIKeyEnvName()
}

// DefaultAzureOpenAIAPIVersion is the default Azure OpenAI API version.
const DefaultAzureOpenAIAPIVersion = "2024-02-15-preview"

// DefaultAzureAPIKeyEnvName is the default environment variable for Azure API key.
const DefaultAzureAPIKeyEnvName = "AZURE_OPENAI_API_KEY"

// =============================================================================
// Phase 19.3b: Shadow Runtime Flags and Embed Health Types
// =============================================================================

// ShadowRuntimeFlags captures the effective runtime state of shadow mode.
// Used for health reporting and debugging.
type ShadowRuntimeFlags struct {
	// Enabled indicates if shadow mode is enabled (mode != "off").
	Enabled bool

	// RealAllowed indicates if real providers are permitted.
	RealAllowed bool

	// ProviderKind is the effective provider kind.
	ProviderKind string

	// ChatConfigured indicates if chat deployment is configured.
	ChatConfigured bool

	// EmbedConfigured indicates if embeddings deployment is configured.
	EmbedConfigured bool
}

// CanonicalString returns a deterministic string representation.
func (f *ShadowRuntimeFlags) CanonicalString() string {
	enabled := "false"
	if f.Enabled {
		enabled = "true"
	}
	realAllowed := "false"
	if f.RealAllowed {
		realAllowed = "true"
	}
	chatCfg := "false"
	if f.ChatConfigured {
		chatCfg = "true"
	}
	embedCfg := "false"
	if f.EmbedConfigured {
		embedCfg = "true"
	}
	return "SHADOW_FLAGS|v1|enabled:" + enabled +
		"|real_allowed:" + realAllowed +
		"|provider:" + f.ProviderKind +
		"|chat:" + chatCfg +
		"|embed:" + embedCfg
}

// EmbedStatus indicates the result of an embeddings healthcheck.
type EmbedStatus string

const (
	// EmbedStatusOK indicates embeddings healthcheck succeeded.
	EmbedStatusOK EmbedStatus = "ok"

	// EmbedStatusFail indicates embeddings healthcheck failed.
	EmbedStatusFail EmbedStatus = "fail"

	// EmbedStatusSkipped indicates embeddings healthcheck was skipped.
	EmbedStatusSkipped EmbedStatus = "skipped"

	// EmbedStatusNotConfigured indicates embeddings not configured.
	EmbedStatusNotConfigured EmbedStatus = "not_configured"
)

// EmbedHealth captures the result of an embeddings healthcheck.
type EmbedHealth struct {
	// Status indicates the healthcheck result.
	Status EmbedStatus

	// LatencyBucket indicates response latency.
	LatencyBucket string

	// VectorHash is SHA256 of the embedding vector bytes.
	// Deterministic for a given input/model.
	VectorHash string

	// ErrorBucket contains abstract error category if failed.
	ErrorBucket string
}

// CanonicalString returns a deterministic string representation.
func (h *EmbedHealth) CanonicalString() string {
	return "EMBED_HEALTH|v1|status:" + string(h.Status) +
		"|latency:" + h.LatencyBucket +
		"|vector_hash:" + h.VectorHash +
		"|error:" + h.ErrorBucket
}

// DefaultMaxSuggestions is the default maximum suggestions per shadow run.
const DefaultMaxSuggestions = 3

// DefaultShadowConfig returns the default shadow configuration.
// CRITICAL: Mode is OFF by default.
// CRITICAL: RealAllowed is false by default.
func DefaultShadowConfig() ShadowConfig {
	return ShadowConfig{
		Mode:           "off",
		ModelName:      "stub",
		ProviderKind:   "stub",
		RealAllowed:    false,
		MaxSuggestions: DefaultMaxSuggestions,
		AzureOpenAI: AzureOpenAIConfig{
			APIVersion: DefaultAzureOpenAIAPIVersion,
		},
	}
}

// GetMaxSuggestions returns the effective max suggestions (clamped to 1-5).
func (c *ShadowConfig) GetMaxSuggestions() int {
	if c.MaxSuggestions <= 0 {
		return DefaultMaxSuggestions
	}
	if c.MaxSuggestions > 5 {
		return 5
	}
	return c.MaxSuggestions
}

// CircleIDs returns circle IDs in deterministic sorted order.
func (c *MultiCircleConfig) CircleIDs() []identity.EntityID {
	ids := make([]identity.EntityID, 0, len(c.Circles))
	for id := range c.Circles {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return string(ids[i]) < string(ids[j])
	})
	return ids
}

// GetCircle returns a circle config by ID.
func (c *MultiCircleConfig) GetCircle(id identity.EntityID) *CircleConfig {
	return c.Circles[id]
}

// CanonicalString returns a deterministic string representation.
func (c *MultiCircleConfig) CanonicalString() string {
	var b strings.Builder

	// Sort circles for determinism
	ids := c.CircleIDs()

	for _, id := range ids {
		circle := c.Circles[id]
		b.WriteString("circle:")
		b.WriteString(string(id))
		b.WriteString("|name:")
		b.WriteString(circle.Name)

		// Email integrations (sorted)
		emails := make([]string, len(circle.EmailIntegrations))
		for i, e := range circle.EmailIntegrations {
			emails[i] = e.Provider + ":" + e.Identifier + ":" + strings.Join(e.Scopes, ",")
		}
		sort.Strings(emails)
		for _, e := range emails {
			b.WriteString("|email:")
			b.WriteString(e)
		}

		// Calendar integrations (sorted)
		cals := make([]string, len(circle.CalendarIntegrations))
		for i, cal := range circle.CalendarIntegrations {
			cals[i] = cal.Provider + ":" + cal.CalendarID + ":" + strings.Join(cal.Scopes, ",")
		}
		sort.Strings(cals)
		for _, cal := range cals {
			b.WriteString("|calendar:")
			b.WriteString(cal)
		}

		// Finance integrations (sorted)
		fins := make([]string, len(circle.FinanceIntegrations))
		for i, f := range circle.FinanceIntegrations {
			fins[i] = f.Provider + ":" + f.Identifier + ":" + strings.Join(f.Scopes, ",")
		}
		sort.Strings(fins)
		for _, f := range fins {
			b.WriteString("|finance:")
			b.WriteString(f)
		}

		b.WriteString("\n")
	}

	// Routing config
	b.WriteString("routing|work_domains:")
	sortedWorkDomains := make([]string, len(c.Routing.WorkDomains))
	copy(sortedWorkDomains, c.Routing.WorkDomains)
	sort.Strings(sortedWorkDomains)
	b.WriteString(strings.Join(sortedWorkDomains, ","))

	b.WriteString("|personal_domains:")
	sortedPersonalDomains := make([]string, len(c.Routing.PersonalDomains))
	copy(sortedPersonalDomains, c.Routing.PersonalDomains)
	sort.Strings(sortedPersonalDomains)
	b.WriteString(strings.Join(sortedPersonalDomains, ","))

	b.WriteString("|vip_senders:")
	sortedVIP := make([]string, len(c.Routing.VIPSenders))
	copy(sortedVIP, c.Routing.VIPSenders)
	sort.Strings(sortedVIP)
	b.WriteString(strings.Join(sortedVIP, ","))

	b.WriteString("|family_members:")
	sortedFamily := make([]string, len(c.Routing.FamilyMembers))
	copy(sortedFamily, c.Routing.FamilyMembers)
	sort.Strings(sortedFamily)
	b.WriteString(strings.Join(sortedFamily, ","))

	// Shadow config (Phase 19 + 19.3)
	b.WriteString("\nshadow|mode:")
	b.WriteString(c.Shadow.Mode)
	b.WriteString("|model:")
	b.WriteString(c.Shadow.ModelName)
	b.WriteString("|provider_kind:")
	b.WriteString(c.Shadow.ProviderKind)
	b.WriteString("|real_allowed:")
	if c.Shadow.RealAllowed {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	// Note: Azure config excluded from canonical string as it contains runtime env vars

	return b.String()
}

// Hash returns a deterministic hash of the configuration.
func (c *MultiCircleConfig) Hash() string {
	if c.canonicalHash != "" {
		return c.canonicalHash
	}
	h := sha256.Sum256([]byte(c.CanonicalString()))
	c.canonicalHash = hex.EncodeToString(h[:])
	return c.canonicalHash
}

// Validate checks the configuration for errors.
func (c *MultiCircleConfig) Validate() error {
	if len(c.Circles) == 0 {
		return &ConfigError{Field: "circles", Message: "at least one circle required"}
	}

	for id, circle := range c.Circles {
		if string(id) == "" {
			return &ConfigError{Field: "circle.id", Message: "circle ID cannot be empty"}
		}
		if circle.Name == "" {
			return &ConfigError{Field: "circle.name", Message: "circle name cannot be empty for " + string(id)}
		}

		// Validate integrations
		for i, email := range circle.EmailIntegrations {
			if email.Provider == "" {
				return &ConfigError{Field: "email.provider", Message: "email provider cannot be empty", Index: i}
			}
			if email.Identifier == "" {
				return &ConfigError{Field: "email.identifier", Message: "email identifier cannot be empty", Index: i}
			}
		}

		for i, cal := range circle.CalendarIntegrations {
			if cal.Provider == "" {
				return &ConfigError{Field: "calendar.provider", Message: "calendar provider cannot be empty", Index: i}
			}
			if cal.CalendarID == "" {
				return &ConfigError{Field: "calendar.calendar_id", Message: "calendar ID cannot be empty", Index: i}
			}
		}

		for i, fin := range circle.FinanceIntegrations {
			if fin.Provider == "" {
				return &ConfigError{Field: "finance.provider", Message: "finance provider cannot be empty", Index: i}
			}
			if fin.Identifier == "" {
				return &ConfigError{Field: "finance.identifier", Message: "finance identifier cannot be empty", Index: i}
			}
		}
	}

	return nil
}

// ConfigError represents a configuration error.
type ConfigError struct {
	Field   string
	Message string
	Index   int
}

func (e *ConfigError) Error() string {
	if e.Index > 0 {
		return "config error: " + e.Field + "[" + itoa(e.Index) + "]: " + e.Message
	}
	return "config error: " + e.Field + ": " + e.Message
}

// itoa converts int to string without strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
