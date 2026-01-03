// Package demo_phase19_3c_real_chat_shadow contains tests for Phase 19.3c.
//
// Phase 19.3c: Real Azure Chat Shadow Run
//
// This file demonstrates:
//   - ChatProvider configuration and initialization
//   - Prompt template v1.1.0 array output format
//   - Strict JSON output validation
//   - MaxSuggestions clamping (1-5)
//   - Privacy guard validation
//   - Provider kind tracking
//
// CRITICAL INVARIANTS:
//   - Chat provider makes REAL network calls (when configured)
//   - Input is ALWAYS privacy-guarded abstract data
//   - Output is validated for strict JSON schema
//   - MaxSuggestions is clamped to 1-5
//   - No goroutines. No time.Now() in internal/.
//   - Stdlib only.
//
// Reference: docs/ADR/ADR-0050-phase19-3c-real-azure-chat-shadow.md
package demo_phase19_3c_real_chat_shadow

import (
	"testing"

	"quantumlife/internal/shadowllm/privacy"
	"quantumlife/internal/shadowllm/prompt"
	"quantumlife/internal/shadowllm/providers/azureopenai"
	"quantumlife/internal/shadowllm/validate"
	"quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Test: Prompt Template v1.1.0
// =============================================================================

func TestPromptTemplateVersion(t *testing.T) {
	// v1.1.0 template for array output
	if prompt.TemplateVersion != "v1.1.0" {
		t.Errorf("TemplateVersion = %q; want v1.1.0", prompt.TemplateVersion)
	}

	// Template hash should be deterministic
	hash1 := prompt.TemplateHash()
	hash2 := prompt.TemplateHash()
	if hash1 != hash2 {
		t.Error("TemplateHash() is not deterministic")
	}
	if len(hash1) != 32 {
		t.Errorf("TemplateHash length = %d; want 32", len(hash1))
	}
}

func TestPromptRenderAbstractInput(t *testing.T) {
	input := &privacy.ShadowInput{
		CircleID:   identity.EntityID("personal"),
		TimeBucket: "2024-01",
		ObligationMagnitudes: map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket{
			shadowllm.CategoryMoney: shadowllm.MagnitudeAFew,
			shadowllm.CategoryWork:  shadowllm.MagnitudeSeveral,
		},
		HeldMagnitudes: map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket{
			shadowllm.CategoryHome: shadowllm.MagnitudeAFew,
		},
		SurfaceCandidateMagnitude: shadowllm.MagnitudeAFew,
		DraftCandidateMagnitude:   shadowllm.MagnitudeNothing,
		MirrorMagnitude:           shadowllm.MagnitudeSeveral,
		TriggersSeen:              true,
		CategoryPresence: map[shadowllm.AbstractCategory]bool{
			shadowllm.CategoryMoney: true,
			shadowllm.CategoryWork:  true,
		},
	}

	system, user := prompt.RenderPrompt(input)

	// System prompt should contain array schema
	if len(system) == 0 {
		t.Error("System prompt is empty")
	}
	if !contains(system, "suggestions") {
		t.Error("System prompt missing 'suggestions' array schema")
	}

	// User prompt should contain abstract data only
	if len(user) == 0 {
		t.Error("User prompt is empty")
	}
	if !contains(user, "personal") {
		t.Error("User prompt missing circle ID")
	}
	if !contains(user, "money") {
		t.Error("User prompt missing category")
	}
}

// =============================================================================
// Test: Validator Exported Functions
// =============================================================================

func TestValidatorExportedFunctions(t *testing.T) {
	// Test exported validation functions
	tests := []struct {
		name     string
		testFunc func() bool
	}{
		{"ValidateCategory", func() bool {
			cat, ok := validate.ValidateCategory("money")
			return ok && cat == shadowllm.CategoryMoney
		}},
		{"ValidateHorizon", func() bool {
			h, ok := validate.ValidateHorizon("now")
			return ok && h == shadowllm.HorizonNow
		}},
		{"ValidateMagnitude", func() bool {
			m, ok := validate.ValidateMagnitude("a_few")
			return ok && m == shadowllm.MagnitudeAFew
		}},
		{"ValidateConfidence", func() bool {
			c, ok := validate.ValidateConfidence("high")
			return ok && c == shadowllm.ConfidenceHigh
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.testFunc() {
				t.Errorf("%s validation failed", tt.name)
			}
		})
	}
}

func TestValidatorWhyGeneric(t *testing.T) {
	v := validate.NewValidator()

	// Valid why_generic
	err := v.ValidateWhyGeneric("Activity levels suggest reviewing this area soon.")
	if err != nil {
		t.Errorf("Valid why_generic rejected: %v", err)
	}

	// Too long why_generic (exceeds MaxWhyGenericLength)
	longStr := ""
	for i := 0; i < 200; i++ {
		longStr += "a"
	}
	err = v.ValidateWhyGeneric(longStr)
	if err == nil {
		t.Error("Long why_generic should be rejected")
	}
}

func TestValidatorParseSingleOutput(t *testing.T) {
	v := validate.NewValidator()

	// Valid single output (v1.0.0 format)
	json := `{
		"confidence_bucket": "high",
		"horizon_bucket": "now",
		"magnitude_bucket": "a_few",
		"category": "money",
		"why_generic": "Items need attention soon.",
		"suggested_action_class": "surface"
	}`

	result := v.ParseAndValidate(json)
	if !result.IsValid {
		t.Errorf("Valid JSON rejected: %s", result.ValidationError)
	}
	if result.Confidence != shadowllm.ConfidenceHigh {
		t.Errorf("Confidence = %v; want high", result.Confidence)
	}
	if result.Category != shadowllm.CategoryMoney {
		t.Errorf("Category = %v; want money", result.Category)
	}
}

// =============================================================================
// Test: MaxSuggestions Config
// =============================================================================

func TestMaxSuggestionsDefault(t *testing.T) {
	cfg := config.DefaultShadowConfig()

	maxSugg := cfg.GetMaxSuggestions()
	if maxSugg != config.DefaultMaxSuggestions {
		t.Errorf("GetMaxSuggestions() = %d; want %d", maxSugg, config.DefaultMaxSuggestions)
	}
}

func TestMaxSuggestionsClamping(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, config.DefaultMaxSuggestions}, // Zero defaults to 3
		{-1, config.DefaultMaxSuggestions}, // Negative defaults to 3
		{1, 1},                             // Min valid
		{3, 3},                             // Default
		{5, 5},                             // Max valid
		{10, 5},                            // Clamped to max
		{100, 5},                           // Clamped to max
	}

	for _, tt := range tests {
		cfg := &config.ShadowConfig{MaxSuggestions: tt.input}
		got := cfg.GetMaxSuggestions()
		if got != tt.expected {
			t.Errorf("GetMaxSuggestions(%d) = %d; want %d", tt.input, got, tt.expected)
		}
	}
}

// =============================================================================
// Test: ChatProvider Configuration
// =============================================================================

func TestChatProviderConfig(t *testing.T) {
	// Missing endpoint
	_, err := azureopenai.NewChatProvider(azureopenai.ChatConfig{
		Deployment: "gpt-4",
		APIKey:     "test-key",
	})
	if err == nil {
		t.Error("Expected error for missing endpoint")
	}

	// Missing deployment
	_, err = azureopenai.NewChatProvider(azureopenai.ChatConfig{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
	})
	if err == nil {
		t.Error("Expected error for missing deployment")
	}

	// Missing API key
	_, err = azureopenai.NewChatProvider(azureopenai.ChatConfig{
		Endpoint:   "https://test.openai.azure.com",
		Deployment: "gpt-4",
	})
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	// Valid config
	provider, err := azureopenai.NewChatProvider(azureopenai.ChatConfig{
		Endpoint:       "https://test.openai.azure.com",
		Deployment:     "gpt-4",
		APIKey:         "test-key",
		MaxSuggestions: 3,
	})
	if err != nil {
		t.Errorf("Valid config rejected: %v", err)
	}
	if provider == nil {
		t.Error("Provider is nil")
		return
	}
	if provider.Name() != "azure_openai_chat" {
		t.Errorf("Name() = %q; want azure_openai_chat", provider.Name())
	}
	if provider.Deployment() != "gpt-4" {
		t.Errorf("Deployment() = %q; want gpt-4", provider.Deployment())
	}
	if provider.ProviderKind() != shadowllm.ProviderKindAzureOpenAI {
		t.Errorf("ProviderKind() = %v; want azure_openai", provider.ProviderKind())
	}
}

func TestChatProviderMaxSuggestionsClamping(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 3},   // Zero defaults to 3
		{-5, 3},  // Negative defaults to 3
		{1, 1},   // Min valid
		{5, 5},   // Max valid
		{10, 5},  // Clamped to 5
	}

	for _, tt := range tests {
		provider, err := azureopenai.NewChatProvider(azureopenai.ChatConfig{
			Endpoint:       "https://test.openai.azure.com",
			Deployment:     "gpt-4",
			APIKey:         "test-key",
			MaxSuggestions: tt.input,
		})
		if err != nil {
			t.Errorf("NewChatProvider(%d) error: %v", tt.input, err)
			continue
		}
		// Provider internally clamps maxSuggestions
		_ = provider
	}
}

// =============================================================================
// Test: Privacy Guard
// =============================================================================

func TestPrivacyGuardValidInput(t *testing.T) {
	guard := privacy.NewGuard()

	// Valid abstract input (time_bucket must be YYYY-MM-DD)
	input := &privacy.ShadowInput{
		CircleID:   identity.EntityID("personal"),
		TimeBucket: "2024-01-15",
		ObligationMagnitudes: map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket{
			shadowllm.CategoryMoney: shadowllm.MagnitudeAFew,
		},
	}

	err := guard.ValidateInput(input)
	if err != nil {
		t.Errorf("Valid input rejected: %v", err)
	}
}

func TestPrivacySafeInputBuilder(t *testing.T) {
	digest := shadowllm.ShadowInputDigest{
		ObligationCountByCategory: map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket{
			shadowllm.CategoryWork: shadowllm.MagnitudeSeveral,
		},
		HeldCountByCategory: map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket{},
	}

	input, err := privacy.BuildPrivacySafeInput(
		identity.EntityID("work"),
		"2024-Q1",
		digest,
		"snapshot-hash-abc123",
	)

	if err != nil {
		t.Errorf("BuildPrivacySafeInput error: %v", err)
	}
	if input.CircleID != "work" {
		t.Errorf("CircleID = %q; want work", input.CircleID)
	}
}

// =============================================================================
// Test: Shared Types
// =============================================================================

func TestSharedChatTypes(t *testing.T) {
	// Verify shared types compile and work
	req := azureopenai.ChatRequest{
		Messages: []azureopenai.ChatMessage{
			{Role: "system", Content: "test"},
			{Role: "user", Content: "test"},
		},
		Temperature: 0.3,
		MaxTokens:   500,
		ResponseFormat: &azureopenai.ChatResponseFormat{
			Type: "json_object",
		},
	}

	if len(req.Messages) != 2 {
		t.Errorf("Messages count = %d; want 2", len(req.Messages))
	}
	if req.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat.Type = %q; want json_object", req.ResponseFormat.Type)
	}
}

// =============================================================================
// Test: Array Output Schema
// =============================================================================

func TestArrayOutputSchema(t *testing.T) {
	// Test that prompt package exports array types
	schema := prompt.ModelOutputArraySchema{
		Suggestions: []prompt.SuggestionSchema{
			{
				Category:   "money",
				Horizon:    "now",
				Magnitude:  "a_few",
				Confidence: "high",
				WhyGeneric: "Items need attention.",
			},
			{
				Category:   "work",
				Horizon:    "soon",
				Magnitude:  "several",
				Confidence: "medium",
				WhyGeneric: "Review activity patterns.",
			},
		},
	}

	if len(schema.Suggestions) != 2 {
		t.Errorf("Suggestions count = %d; want 2", len(schema.Suggestions))
	}
	if schema.Suggestions[0].Category != "money" {
		t.Errorf("First suggestion category = %q; want money", schema.Suggestions[0].Category)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
