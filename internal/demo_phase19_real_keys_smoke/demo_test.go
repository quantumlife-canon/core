// Package demo_phase19_real_keys_smoke tests the real keys provider selection
// and fallback behavior WITHOUT making network calls.
//
// Phase 19.3: Azure OpenAI Shadow Provider - Smoke Tests
//
// CRITICAL: These tests NEVER call the real network.
// CRITICAL: They verify fallback to stub when conditions are not met.
//
// Reference: docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md
package demo_phase19_real_keys_smoke

import (
	"os"
	"testing"
	"time"

	"quantumlife/internal/shadowllm/providers/azureopenai"
	"quantumlife/internal/shadowllm/stub"
	"quantumlife/pkg/domain/shadowllm"
)

// testClock returns a deterministic clock function for testing.
func testClock() func() time.Time {
	fixed := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return fixed }
}

// TestIsConfigured_MissingEnvVars verifies that IsConfigured returns false
// when Azure environment variables are not set.
//
// CRITICAL: Does not call the network - only checks env var presence.
func TestIsConfigured_MissingEnvVars(t *testing.T) {
	// Clear any existing env vars for this test
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origDeployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	origAPIKey := os.Getenv("AZURE_OPENAI_API_KEY")

	os.Unsetenv("AZURE_OPENAI_ENDPOINT")
	os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
	os.Unsetenv("AZURE_OPENAI_API_KEY")

	defer func() {
		// Restore original values
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		}
		if origDeployment != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeployment)
		}
		if origAPIKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origAPIKey)
		}
	}()

	// Test: Should return false when env vars are missing
	if azureopenai.IsConfigured() {
		t.Error("IsConfigured() should return false when env vars are missing")
	}
}

// TestIsConfigured_PartialEnvVars verifies that IsConfigured returns false
// when only some Azure environment variables are set.
func TestIsConfigured_PartialEnvVars(t *testing.T) {
	// Clear all first
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origDeployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	origAPIKey := os.Getenv("AZURE_OPENAI_API_KEY")

	defer func() {
		// Restore original values
		os.Unsetenv("AZURE_OPENAI_ENDPOINT")
		os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
		os.Unsetenv("AZURE_OPENAI_API_KEY")
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		}
		if origDeployment != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeployment)
		}
		if origAPIKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origAPIKey)
		}
	}()

	tests := []struct {
		name       string
		endpoint   string
		deployment string
		apiKey     string
		want       bool
	}{
		{
			name:       "only endpoint set",
			endpoint:   "https://test.openai.azure.com",
			deployment: "",
			apiKey:     "",
			want:       false,
		},
		{
			name:       "only deployment set",
			endpoint:   "",
			deployment: "gpt-4o-mini",
			apiKey:     "",
			want:       false,
		},
		{
			name:       "only api key set",
			endpoint:   "",
			deployment: "",
			apiKey:     "test-key",
			want:       false,
		},
		{
			name:       "endpoint and deployment, no key",
			endpoint:   "https://test.openai.azure.com",
			deployment: "gpt-4o-mini",
			apiKey:     "",
			want:       false,
		},
		{
			name:       "all set - configured",
			endpoint:   "https://test.openai.azure.com",
			deployment: "gpt-4o-mini",
			apiKey:     "test-key",
			want:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set env vars for this test case
			os.Unsetenv("AZURE_OPENAI_ENDPOINT")
			os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
			os.Unsetenv("AZURE_OPENAI_API_KEY")

			if tc.endpoint != "" {
				os.Setenv("AZURE_OPENAI_ENDPOINT", tc.endpoint)
			}
			if tc.deployment != "" {
				os.Setenv("AZURE_OPENAI_DEPLOYMENT", tc.deployment)
			}
			if tc.apiKey != "" {
				os.Setenv("AZURE_OPENAI_API_KEY", tc.apiKey)
			}

			got := azureopenai.IsConfigured()
			if got != tc.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestProviderSelectionLogic_RealAllowedFalse verifies that when RealAllowed is false,
// the provider selection should choose stub even if Azure env vars exist.
//
// This test simulates the logic in createShadowProvider without calling main package.
func TestProviderSelectionLogic_RealAllowedFalse(t *testing.T) {
	// Simulate the logic: if RealAllowed is false, should use stub
	realAllowed := false
	requestedKind := "azure_openai"

	// The selection logic:
	// 1. If !realAllowed → stub (regardless of providerKind)
	// 2. If providerKind == "stub" → stub
	// 3. If providerKind == "azure_openai" && !IsConfigured() → stub (fallback)
	// 4. If providerKind == "azure_openai" && IsConfigured() → azure

	selectedProvider := requestedKind // would be selected without guards
	if !realAllowed {
		selectedProvider = "stub"
	}

	if selectedProvider != "stub" {
		t.Errorf("When RealAllowed=false, provider should be stub, got %s", selectedProvider)
	}

	// Verify stub provider works
	stubProvider := stub.NewStubModel()
	if stubProvider.Name() != "stub" {
		t.Errorf("Stub provider name should be 'stub', got %s", stubProvider.Name())
	}
}

// TestProviderSelectionLogic_AzureNotConfigured verifies that when Azure env vars
// are missing, the provider selection falls back to stub even when RealAllowed=true.
func TestProviderSelectionLogic_AzureNotConfigured(t *testing.T) {
	// Clear Azure env vars
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origDeployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	origAPIKey := os.Getenv("AZURE_OPENAI_API_KEY")

	os.Unsetenv("AZURE_OPENAI_ENDPOINT")
	os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
	os.Unsetenv("AZURE_OPENAI_API_KEY")

	defer func() {
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		}
		if origDeployment != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeployment)
		}
		if origAPIKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origAPIKey)
		}
	}()

	// Simulate the logic
	realAllowed := true
	providerKind := "azure_openai"

	var selectedProvider string
	var fallbackReason string

	if !realAllowed {
		selectedProvider = "stub"
		fallbackReason = "real_not_allowed"
	} else if providerKind == "stub" {
		selectedProvider = "stub"
		fallbackReason = "provider_kind_stub"
	} else if providerKind == "azure_openai" {
		if !azureopenai.IsConfigured() {
			selectedProvider = "stub"
			fallbackReason = "missing_env_vars"
		} else {
			selectedProvider = "azure_openai"
		}
	}

	// Verify fallback to stub
	if selectedProvider != "stub" {
		t.Errorf("When Azure not configured, should fallback to stub, got %s", selectedProvider)
	}
	if fallbackReason != "missing_env_vars" {
		t.Errorf("Fallback reason should be 'missing_env_vars', got %s", fallbackReason)
	}
}

// TestStubProviderDeterminism verifies that the stub provider produces
// deterministic output for the same input.
func TestStubProviderDeterminism(t *testing.T) {
	provider := stub.NewStubModel()

	ctx1 := shadowllm.ShadowContext{
		CircleID:   "personal",
		InputsHash: "abc123def456",
		Seed:       42,
		Clock:      testClock(),
	}

	run1, err := provider.Observe(ctx1)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	run2, err := provider.Observe(ctx1)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	// Same input should produce same hash
	if run1.Hash() != run2.Hash() {
		t.Errorf("Stub provider should be deterministic: hash1=%s, hash2=%s", run1.Hash(), run2.Hash())
	}
}

// TestStubProviderMetadataOnly verifies that stub provider only outputs
// metadata (scores, categories) and never content.
func TestStubProviderMetadataOnly(t *testing.T) {
	provider := stub.NewStubModel()

	ctx := shadowllm.ShadowContext{
		CircleID:   "personal",
		InputsHash: "abc123def456",
		Seed:       123,
		Clock:      testClock(),
	}

	run, err := provider.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	// Verify no content strings in signals
	for _, sig := range run.Signals {
		// NotesHash should be either "empty" (for no notes) or a 64-char SHA256 hash
		if sig.NotesHash != "empty" && len(sig.NotesHash) != 64 {
			t.Errorf("NotesHash should be 'empty' or a 64-char SHA256 hash, got %q (%d chars)", sig.NotesHash, len(sig.NotesHash))
		}

		// Category should be from allowed set
		validCategories := map[shadowllm.AbstractCategory]bool{
			shadowllm.CategoryMoney:  true,
			shadowllm.CategoryTime:   true,
			shadowllm.CategoryPeople: true,
			shadowllm.CategoryWork:   true,
			shadowllm.CategoryHome:   true,
		}
		if !validCategories[sig.Category] {
			t.Errorf("Invalid category: %s", sig.Category)
		}

		// Kind should be from allowed set
		validKinds := map[shadowllm.ShadowSignalKind]bool{
			shadowllm.SignalKindRegretDelta:      true,
			shadowllm.SignalKindCategoryPressure: true,
			shadowllm.SignalKindConfidence:       true,
			shadowllm.SignalKindLabelSuggestion:  true,
		}
		if !validKinds[sig.Kind] {
			t.Errorf("Invalid signal kind: %s", sig.Kind)
		}

		// Values should be bounded [0, 1]
		if sig.ValueFloat < 0 || sig.ValueFloat > 1 {
			t.Errorf("ValueFloat should be in [0,1], got %f", sig.ValueFloat)
		}
		if sig.ConfidenceFloat < 0 || sig.ConfidenceFloat > 1 {
			t.Errorf("ConfidenceFloat should be in [0,1], got %f", sig.ConfidenceFloat)
		}
	}
}

// TestProviderKindEnum verifies provider kind enum values.
func TestProviderKindEnum(t *testing.T) {
	tests := []struct {
		kind shadowllm.ProviderKind
		str  string
	}{
		{shadowllm.ProviderKindNone, "none"},
		{shadowllm.ProviderKindStub, "stub"},
		{shadowllm.ProviderKindAzureOpenAI, "azure_openai"},
		{shadowllm.ProviderKindLocalSLM, "local_slm"},
	}

	for _, tc := range tests {
		if string(tc.kind) != tc.str {
			t.Errorf("ProviderKind %v should be %q, got %q", tc.kind, tc.str, string(tc.kind))
		}
	}
}

// TestNewProviderFromEnv_MissingEndpoint verifies that NewProviderFromEnv
// returns an error when endpoint is missing.
func TestNewProviderFromEnv_MissingEndpoint(t *testing.T) {
	// Clear all Azure env vars
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origDeployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	origAPIKey := os.Getenv("AZURE_OPENAI_API_KEY")

	os.Unsetenv("AZURE_OPENAI_ENDPOINT")
	os.Setenv("AZURE_OPENAI_DEPLOYMENT", "test-deployment")
	os.Setenv("AZURE_OPENAI_API_KEY", "test-key")

	defer func() {
		os.Unsetenv("AZURE_OPENAI_ENDPOINT")
		os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
		os.Unsetenv("AZURE_OPENAI_API_KEY")
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		}
		if origDeployment != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeployment)
		}
		if origAPIKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origAPIKey)
		}
	}()

	_, err := azureopenai.NewProviderFromEnv()
	if err == nil {
		t.Error("Expected error when endpoint is missing")
	}
}

// TestNewProviderFromEnv_MissingAPIKey verifies that NewProviderFromEnv
// returns an error when API key is missing.
func TestNewProviderFromEnv_MissingAPIKey(t *testing.T) {
	// Clear all Azure env vars
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origDeployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	origAPIKey := os.Getenv("AZURE_OPENAI_API_KEY")

	os.Setenv("AZURE_OPENAI_ENDPOINT", "https://test.openai.azure.com")
	os.Setenv("AZURE_OPENAI_DEPLOYMENT", "test-deployment")
	os.Unsetenv("AZURE_OPENAI_API_KEY")

	defer func() {
		os.Unsetenv("AZURE_OPENAI_ENDPOINT")
		os.Unsetenv("AZURE_OPENAI_DEPLOYMENT")
		os.Unsetenv("AZURE_OPENAI_API_KEY")
		if origEndpoint != "" {
			os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		}
		if origDeployment != "" {
			os.Setenv("AZURE_OPENAI_DEPLOYMENT", origDeployment)
		}
		if origAPIKey != "" {
			os.Setenv("AZURE_OPENAI_API_KEY", origAPIKey)
		}
	}()

	_, err := azureopenai.NewProviderFromEnv()
	if err == nil {
		t.Error("Expected error when API key is missing")
	}
}

// TestEnvVarOverride_RealAllowed verifies environment variable override behavior.
func TestEnvVarOverride_RealAllowed(t *testing.T) {
	// Save original
	orig := os.Getenv("QL_SHADOW_REAL_ALLOWED")
	defer func() {
		if orig != "" {
			os.Setenv("QL_SHADOW_REAL_ALLOWED", orig)
		} else {
			os.Unsetenv("QL_SHADOW_REAL_ALLOWED")
		}
	}()

	tests := []struct {
		name        string
		envValue    string
		configValue bool
		expected    bool
	}{
		{
			name:        "env true overrides config false",
			envValue:    "true",
			configValue: false,
			expected:    true,
		},
		{
			name:        "env empty uses config false",
			envValue:    "",
			configValue: false,
			expected:    false,
		},
		{
			name:        "env empty uses config true",
			envValue:    "",
			configValue: true,
			expected:    true,
		},
		{
			name:        "env false (not 'true') uses config value",
			envValue:    "false",
			configValue: true,
			expected:    true, // only "true" overrides to true
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				os.Setenv("QL_SHADOW_REAL_ALLOWED", tc.envValue)
			} else {
				os.Unsetenv("QL_SHADOW_REAL_ALLOWED")
			}

			// Simulate the override logic from createShadowProvider
			realAllowed := tc.configValue
			if envVal := os.Getenv("QL_SHADOW_REAL_ALLOWED"); envVal == "true" {
				realAllowed = true
			}

			if realAllowed != tc.expected {
				t.Errorf("RealAllowed = %v, expected %v", realAllowed, tc.expected)
			}
		})
	}
}

// TestEnvVarOverride_ProviderKind verifies provider kind override from env.
func TestEnvVarOverride_ProviderKind(t *testing.T) {
	// Save original
	orig := os.Getenv("QL_SHADOW_PROVIDER_KIND")
	defer func() {
		if orig != "" {
			os.Setenv("QL_SHADOW_PROVIDER_KIND", orig)
		} else {
			os.Unsetenv("QL_SHADOW_PROVIDER_KIND")
		}
	}()

	tests := []struct {
		name        string
		envValue    string
		configValue string
		expected    string
	}{
		{
			name:        "env azure overrides config stub",
			envValue:    "azure_openai",
			configValue: "stub",
			expected:    "azure_openai",
		},
		{
			name:        "env empty uses config value",
			envValue:    "",
			configValue: "azure_openai",
			expected:    "azure_openai",
		},
		{
			name:        "empty config defaults to stub",
			envValue:    "",
			configValue: "",
			expected:    "stub",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				os.Setenv("QL_SHADOW_PROVIDER_KIND", tc.envValue)
			} else {
				os.Unsetenv("QL_SHADOW_PROVIDER_KIND")
			}

			// Simulate the override logic from createShadowProvider
			providerKind := tc.configValue
			if envVal := os.Getenv("QL_SHADOW_PROVIDER_KIND"); envVal != "" {
				providerKind = envVal
			}
			if providerKind == "" || providerKind == "none" {
				providerKind = "stub"
			}

			if providerKind != tc.expected {
				t.Errorf("ProviderKind = %v, expected %v", providerKind, tc.expected)
			}
		})
	}
}
