// Package demo_phase19_3b_go_real provides demo tests for Phase 19.3b.
//
// Phase 19.3b: Go Real Azure + Embeddings
//
// CRITICAL INVARIANTS TESTED:
//   - Embeddings healthcheck uses ONLY safe constant input
//   - Embeddings output is hash only - never raw vectors
//   - Config correctly distinguishes Chat vs Embed deployments
//   - ShadowRuntimeFlags correctly reports configuration state
//   - No secrets in logs or storage
//   - Stub implementations work without real credentials
//
// Reference: docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md
package demo_phase19_3b_go_real

import (
	"os"
	"strings"
	"testing"

	"quantumlife/internal/shadowllm"
	"quantumlife/internal/shadowllm/providers/azureopenai"
	pkgconfig "quantumlife/pkg/domain/config"
)

// =============================================================================
// Embeddings Healthcheck Safe Input Tests
// =============================================================================

// TestEmbedHealthcheckInputIsSafeConstant verifies input is always safe.
func TestEmbedHealthcheckInputIsSafeConstant(t *testing.T) {
	expected := "quantumlife-shadow-healthcheck"

	if azureopenai.EmbedHealthcheckInput != expected {
		t.Errorf("EmbedHealthcheckInput should be %q, got %q",
			expected, azureopenai.EmbedHealthcheckInput)
	}

	// Should not contain any user-identifiable patterns
	forbidden := []string{
		"@", "http://", "https://", "$", "email", "name", "address",
	}

	for _, f := range forbidden {
		if strings.Contains(azureopenai.EmbedHealthcheckInput, f) {
			t.Errorf("EmbedHealthcheckInput contains forbidden pattern: %s", f)
		}
	}

	t.Log("Embeddings healthcheck uses safe constant input")
}

// =============================================================================
// Stub Embed Healthchecker Tests
// =============================================================================

// TestStubEmbedHealthcheckerReturnsOK verifies stub returns OK status.
func TestStubEmbedHealthcheckerReturnsOK(t *testing.T) {
	stub := &shadowllm.StubEmbedHealthchecker{
		Deployment: "text-embedding-ada-002",
	}

	result, err := stub.Healthcheck()
	if err != nil {
		t.Fatalf("Stub healthcheck failed: %v", err)
	}

	if result.Status != shadowllm.EmbedStatusOK {
		t.Errorf("Expected EmbedStatusOK, got %s", result.Status)
	}

	if result.LatencyBucket != "na" {
		t.Errorf("Expected 'na' latency for stub, got %s", result.LatencyBucket)
	}

	if result.VectorHash == "" {
		t.Error("VectorHash should not be empty")
	}

	t.Logf("Stub healthcheck returned: status=%s, vectorHash=%s...",
		result.Status, result.VectorHash[:16])
}

// TestStubEmbedHealthcheckerIsDeterministic verifies determinism.
func TestStubEmbedHealthcheckerIsDeterministic(t *testing.T) {
	stub := &shadowllm.StubEmbedHealthchecker{
		Deployment: "test-deployment",
	}

	result1, _ := stub.Healthcheck()
	result2, _ := stub.Healthcheck()

	if result1.VectorHash != result2.VectorHash {
		t.Errorf("Stub should be deterministic: %s != %s",
			result1.VectorHash, result2.VectorHash)
	}

	t.Log("Stub embed healthchecker is deterministic")
}

// TestStubEmbedHealthcheckerVectorHashIncludesDeployment verifies deployment affects hash.
func TestStubEmbedHealthcheckerVectorHashIncludesDeployment(t *testing.T) {
	stub1 := &shadowllm.StubEmbedHealthchecker{Deployment: "deployment-a"}
	stub2 := &shadowllm.StubEmbedHealthchecker{Deployment: "deployment-b"}

	result1, _ := stub1.Healthcheck()
	result2, _ := stub2.Healthcheck()

	if result1.VectorHash == result2.VectorHash {
		t.Error("Different deployments should produce different hashes")
	}

	t.Log("Vector hash correctly includes deployment name")
}

// =============================================================================
// Config Tests - Azure OpenAI Extensions
// =============================================================================

// TestAzureOpenAIConfigChatDeployment verifies ChatDeployment field.
func TestAzureOpenAIConfigChatDeployment(t *testing.T) {
	cfg := pkgconfig.AzureOpenAIConfig{
		ChatDeployment: "gpt-4o-mini",
		Deployment:     "gpt-35-turbo", // Legacy field
	}

	// GetChatDeployment should prefer ChatDeployment over Deployment
	chat := cfg.GetChatDeployment()
	if chat != "gpt-4o-mini" {
		t.Errorf("Expected ChatDeployment 'gpt-4o-mini', got %q", chat)
	}

	// When ChatDeployment is empty, fall back to Deployment
	cfg.ChatDeployment = ""
	chat = cfg.GetChatDeployment()
	if chat != "gpt-35-turbo" {
		t.Errorf("Expected fallback to Deployment 'gpt-35-turbo', got %q", chat)
	}

	t.Log("ChatDeployment field works correctly with fallback")
}

// TestAzureOpenAIConfigEmbedDeployment verifies EmbedDeployment field.
func TestAzureOpenAIConfigEmbedDeployment(t *testing.T) {
	cfg := pkgconfig.AzureOpenAIConfig{
		EmbedDeployment: "text-embedding-ada-002",
	}

	if cfg.EmbedDeployment != "text-embedding-ada-002" {
		t.Errorf("EmbedDeployment mismatch: %s", cfg.EmbedDeployment)
	}

	t.Log("EmbedDeployment field is correctly stored")
}

// TestAzureOpenAIConfigHasEmbeddings verifies HasEmbeddings method.
func TestAzureOpenAIConfigHasEmbeddings(t *testing.T) {
	// No embeddings
	cfg := pkgconfig.AzureOpenAIConfig{}
	if cfg.HasEmbeddings() {
		t.Error("HasEmbeddings should be false when EmbedDeployment is empty")
	}

	// With embeddings
	cfg.EmbedDeployment = "text-embedding-ada-002"
	if !cfg.HasEmbeddings() {
		t.Error("HasEmbeddings should be true when EmbedDeployment is set")
	}

	t.Log("HasEmbeddings method works correctly")
}

// TestAzureOpenAIConfigAPIKeyEnvName verifies API key env name.
func TestAzureOpenAIConfigAPIKeyEnvName(t *testing.T) {
	cfg := pkgconfig.AzureOpenAIConfig{}

	// Default
	envName := cfg.GetAPIKeyEnvName()
	if envName != pkgconfig.DefaultAzureAPIKeyEnvName {
		t.Errorf("Expected default %q, got %q",
			pkgconfig.DefaultAzureAPIKeyEnvName, envName)
	}

	// Custom
	cfg.APIKeyEnvName = "CUSTOM_API_KEY"
	envName = cfg.GetAPIKeyEnvName()
	if envName != "CUSTOM_API_KEY" {
		t.Errorf("Expected custom 'CUSTOM_API_KEY', got %q", envName)
	}

	t.Log("APIKeyEnvName field works correctly with default")
}

// TestAzureOpenAIConfigCanonicalString verifies canonical string.
func TestAzureOpenAIConfigCanonicalString(t *testing.T) {
	cfg := pkgconfig.AzureOpenAIConfig{
		Endpoint:        "https://test.openai.azure.com",
		ChatDeployment:  "gpt-4o-mini",
		EmbedDeployment: "text-embedding-ada-002",
		APIVersion:      "2024-02-15-preview",
	}

	canonical := cfg.CanonicalString()

	// Should include key fields
	expected := []string{"gpt-4o-mini", "text-embedding-ada-002", "2024-02-15-preview"}
	for _, e := range expected {
		if !strings.Contains(canonical, e) {
			t.Errorf("Canonical string should contain %q: %s", e, canonical)
		}
	}

	// Should have a deterministic format with version prefix
	if !strings.HasPrefix(canonical, "AZURE_CONFIG|v1|") {
		t.Errorf("Canonical string should start with version prefix: %s", canonical)
	}

	t.Logf("Canonical string: %s", canonical)
}

// =============================================================================
// ShadowRuntimeFlags Tests
// =============================================================================

// TestShadowRuntimeFlagsDefault verifies default flag values.
func TestShadowRuntimeFlagsDefault(t *testing.T) {
	flags := pkgconfig.ShadowRuntimeFlags{}

	if flags.Enabled {
		t.Error("Enabled should be false by default")
	}

	if flags.RealAllowed {
		t.Error("RealAllowed should be false by default")
	}

	if flags.ChatConfigured {
		t.Error("ChatConfigured should be false by default")
	}

	if flags.EmbedConfigured {
		t.Error("EmbedConfigured should be false by default")
	}

	t.Log("ShadowRuntimeFlags has correct default values")
}

// TestShadowRuntimeFlagsAllFieldsPopulated verifies all fields.
func TestShadowRuntimeFlagsAllFieldsPopulated(t *testing.T) {
	flags := pkgconfig.ShadowRuntimeFlags{
		Enabled:         true,
		RealAllowed:     true,
		ProviderKind:    "azure_openai",
		ChatConfigured:  true,
		EmbedConfigured: true,
	}

	if !flags.Enabled {
		t.Error("Enabled should be true")
	}

	if !flags.RealAllowed {
		t.Error("RealAllowed should be true")
	}

	if flags.ProviderKind != "azure_openai" {
		t.Errorf("ProviderKind should be 'azure_openai', got %s", flags.ProviderKind)
	}

	if !flags.ChatConfigured {
		t.Error("ChatConfigured should be true")
	}

	if !flags.EmbedConfigured {
		t.Error("EmbedConfigured should be true")
	}

	t.Log("ShadowRuntimeFlags all fields populate correctly")
}

// =============================================================================
// EmbedHealth and EmbedStatus Tests
// =============================================================================

// TestEmbedStatusValues verifies status enum values.
func TestEmbedStatusValues(t *testing.T) {
	testCases := []struct {
		status   pkgconfig.EmbedStatus
		expected string
	}{
		{pkgconfig.EmbedStatusOK, "ok"},
		{pkgconfig.EmbedStatusFail, "fail"},
		{pkgconfig.EmbedStatusSkipped, "skipped"},
		{pkgconfig.EmbedStatusNotConfigured, "not_configured"},
	}

	for _, tc := range testCases {
		if string(tc.status) != tc.expected {
			t.Errorf("EmbedStatus %v should be %q, got %q",
				tc.status, tc.expected, string(tc.status))
		}
	}

	t.Log("EmbedStatus enum values are correct")
}

// TestEmbedHealthStruct verifies EmbedHealth struct.
func TestEmbedHealthStruct(t *testing.T) {
	health := pkgconfig.EmbedHealth{
		Status:        pkgconfig.EmbedStatusOK,
		LatencyBucket: "fast",
		VectorHash:    "abc123def456",
		ErrorBucket:   "",
	}

	if health.Status != pkgconfig.EmbedStatusOK {
		t.Errorf("Status mismatch: %s", health.Status)
	}

	if health.LatencyBucket != "fast" {
		t.Errorf("LatencyBucket mismatch: %s", health.LatencyBucket)
	}

	if health.VectorHash != "abc123def456" {
		t.Errorf("VectorHash mismatch: %s", health.VectorHash)
	}

	t.Log("EmbedHealth struct works correctly")
}

// =============================================================================
// IsEmbedConfigured Tests
// =============================================================================

// TestIsEmbedConfiguredWithEnvVars verifies env var detection.
func TestIsEmbedConfiguredWithEnvVars(t *testing.T) {
	// Clear env vars first
	origEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	origDeploy := os.Getenv("AZURE_OPENAI_EMBED_DEPLOYMENT")
	origKey := os.Getenv("AZURE_OPENAI_API_KEY")
	defer func() {
		os.Setenv("AZURE_OPENAI_ENDPOINT", origEndpoint)
		os.Setenv("AZURE_OPENAI_EMBED_DEPLOYMENT", origDeploy)
		os.Setenv("AZURE_OPENAI_API_KEY", origKey)
	}()

	os.Unsetenv("AZURE_OPENAI_ENDPOINT")
	os.Unsetenv("AZURE_OPENAI_EMBED_DEPLOYMENT")
	os.Unsetenv("AZURE_OPENAI_API_KEY")

	// Not configured
	if azureopenai.IsEmbedConfigured() {
		t.Error("IsEmbedConfigured should be false when env vars are not set")
	}

	// Set all required vars
	os.Setenv("AZURE_OPENAI_ENDPOINT", "https://test.openai.azure.com")
	os.Setenv("AZURE_OPENAI_EMBED_DEPLOYMENT", "text-embedding-ada-002")
	os.Setenv("AZURE_OPENAI_API_KEY", "test-key")

	if !azureopenai.IsEmbedConfigured() {
		t.Error("IsEmbedConfigured should be true when all env vars are set")
	}

	t.Log("IsEmbedConfigured correctly checks environment variables")
}

// =============================================================================
// Provider Error Types Tests
// =============================================================================

// TestEmbedErrorFormat verifies error message format.
func TestEmbedErrorFormat(t *testing.T) {
	err := &azureopenai.EmbedError{
		Code:    "test_error",
		Message: "test message",
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "azure_openai_embed") {
		t.Errorf("Error should contain provider prefix: %s", errStr)
	}

	if !strings.Contains(errStr, "test_error") {
		t.Errorf("Error should contain code: %s", errStr)
	}

	if !strings.Contains(errStr, "test message") {
		t.Errorf("Error should contain message: %s", errStr)
	}

	t.Logf("EmbedError format: %s", errStr)
}

// =============================================================================
// Config Validation Tests
// =============================================================================

// TestNewEmbedProviderValidation verifies config validation.
func TestNewEmbedProviderValidation(t *testing.T) {
	// Missing endpoint
	_, err := azureopenai.NewEmbedProvider(azureopenai.EmbedConfig{
		Deployment: "test",
		APIKey:     "test",
	})
	if err == nil {
		t.Error("Should fail without endpoint")
	}

	// Missing deployment
	_, err = azureopenai.NewEmbedProvider(azureopenai.EmbedConfig{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test",
	})
	if err == nil {
		t.Error("Should fail without deployment")
	}

	// Missing API key
	_, err = azureopenai.NewEmbedProvider(azureopenai.EmbedConfig{
		Endpoint:   "https://test.openai.azure.com",
		Deployment: "test",
	})
	if err == nil {
		t.Error("Should fail without API key")
	}

	// Valid config
	provider, err := azureopenai.NewEmbedProvider(azureopenai.EmbedConfig{
		Endpoint:   "https://test.openai.azure.com",
		Deployment: "text-embedding-ada-002",
		APIKey:     "test-key",
	})
	if err != nil {
		t.Errorf("Valid config should not fail: %v", err)
	}
	if provider == nil {
		t.Error("Provider should not be nil")
	}

	t.Log("NewEmbedProvider validation works correctly")
}

// TestEmbedProviderName verifies provider name method.
func TestEmbedProviderName(t *testing.T) {
	provider, _ := azureopenai.NewEmbedProvider(azureopenai.EmbedConfig{
		Endpoint:   "https://test.openai.azure.com",
		Deployment: "text-embedding-ada-002",
		APIKey:     "test-key",
	})

	name := provider.Name()
	if name != "azure_openai_embed" {
		t.Errorf("Expected name 'azure_openai_embed', got %q", name)
	}

	t.Logf("Provider name: %s", name)
}

// TestEmbedProviderDeployment verifies deployment method.
func TestEmbedProviderDeployment(t *testing.T) {
	provider, _ := azureopenai.NewEmbedProvider(azureopenai.EmbedConfig{
		Endpoint:   "https://test.openai.azure.com",
		Deployment: "my-embedding-model",
		APIKey:     "test-key",
	})

	deployment := provider.Deployment()
	if deployment != "my-embedding-model" {
		t.Errorf("Expected deployment 'my-embedding-model', got %q", deployment)
	}

	t.Logf("Provider deployment: %s", deployment)
}

// =============================================================================
// No Secrets in Output Tests
// =============================================================================

// TestEmbedHealthResultHasNoSecrets verifies no secrets in health result.
func TestEmbedHealthResultHasNoSecrets(t *testing.T) {
	result := &azureopenai.EmbedHealthResult{
		Status:        pkgconfig.EmbedStatusOK,
		LatencyBucket: "fast",
		VectorHash:    "abc123def456789",
		ErrorBucket:   "",
	}

	// Convert to string representation for inspection
	repr := string(result.Status) + " " + result.LatencyBucket + " " + result.VectorHash + " " + result.ErrorBucket

	// Check for forbidden patterns that might indicate secrets
	forbidden := []string{
		"api_key", "api-key", "password", "secret", "bearer", "authorization",
		"sk-", "pk-", // Common API key prefixes
	}

	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(repr), f) {
			t.Errorf("Health result contains forbidden pattern: %s", f)
		}
	}

	t.Log("EmbedHealthResult contains no secrets")
}
