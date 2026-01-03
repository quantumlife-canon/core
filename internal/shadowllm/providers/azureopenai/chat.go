// Package azureopenai provides Azure OpenAI providers for shadow LLM.
//
// Phase 19.3c: Real Azure Chat Shadow Run
//
// CRITICAL INVARIANTS:
//   - Makes HTTP calls to Azure OpenAI Chat Completions API
//   - NO retries - single request only
//   - Must honor context deadline
//   - Never logs API keys or response content
//   - Input is ALWAYS privacy-guarded abstract data
//   - Output is validated and sanitized (strict JSON)
//   - Stdlib net/http only
//   - No goroutines. No time.Now() (caller measures latency).
//
// Reference: docs/ADR/ADR-0050-phase19-3c-real-azure-chat-shadow.md
package azureopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"quantumlife/internal/shadowllm/privacy"
	"quantumlife/internal/shadowllm/prompt"
	"quantumlife/internal/shadowllm/validate"
	"quantumlife/pkg/domain/shadowllm"
)

// ChatProvider implements the Azure OpenAI chat provider.
//
// CRITICAL: This provider makes REAL network calls.
// CRITICAL: Input is always privacy-guarded abstract data.
// CRITICAL: Output is validated for privacy compliance.
type ChatProvider struct {
	endpoint       string
	deployment     string
	apiVersion     string
	apiKey         string
	client         *http.Client
	maxSuggestions int
	validator      *validate.Validator
}

// ChatConfig contains Azure OpenAI chat provider configuration.
type ChatConfig struct {
	// Endpoint is the Azure OpenAI endpoint URL.
	Endpoint string

	// Deployment is the chat model deployment name.
	Deployment string

	// APIVersion is the Azure OpenAI API version.
	APIVersion string

	// APIKey is the API key (from environment variable).
	APIKey string

	// MaxSuggestions limits suggestions per run.
	MaxSuggestions int
}

// NewChatProvider creates a new Azure OpenAI chat provider.
//
// CRITICAL: APIKey should come from environment variable.
func NewChatProvider(cfg ChatConfig) (*ChatProvider, error) {
	if cfg.Endpoint == "" {
		return nil, &ChatError{Code: "missing_endpoint", Message: "endpoint is required"}
	}
	if cfg.Deployment == "" {
		return nil, &ChatError{Code: "missing_deployment", Message: "deployment is required"}
	}
	if cfg.APIKey == "" {
		return nil, &ChatError{Code: "missing_api_key", Message: "API key is required"}
	}

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	maxSuggestions := cfg.MaxSuggestions
	if maxSuggestions <= 0 {
		maxSuggestions = 3
	}
	if maxSuggestions > 5 {
		maxSuggestions = 5
	}

	return &ChatProvider{
		endpoint:       strings.TrimSuffix(cfg.Endpoint, "/"),
		deployment:     cfg.Deployment,
		apiVersion:     apiVersion,
		apiKey:         cfg.APIKey,
		client:         &http.Client{},
		maxSuggestions: maxSuggestions,
		validator:      validate.NewValidator(),
	}, nil
}

// NewChatProviderFromEnv creates a chat provider using environment variables.
//
// Environment variables:
//   - AZURE_OPENAI_ENDPOINT
//   - AZURE_OPENAI_DEPLOYMENT (or AZURE_OPENAI_CHAT_DEPLOYMENT)
//   - AZURE_OPENAI_API_KEY
//   - AZURE_OPENAI_API_VERSION (optional)
//   - SHADOW_MAX_SUGGESTIONS (optional)
func NewChatProviderFromEnv() (*ChatProvider, error) {
	deployment := os.Getenv("AZURE_OPENAI_CHAT_DEPLOYMENT")
	if deployment == "" {
		deployment = os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	}

	maxSuggestions := 3
	if env := os.Getenv("SHADOW_MAX_SUGGESTIONS"); env != "" {
		if n := parseEnvInt(env); n > 0 {
			maxSuggestions = n
		}
	}

	return NewChatProvider(ChatConfig{
		Endpoint:       os.Getenv("AZURE_OPENAI_ENDPOINT"),
		Deployment:     deployment,
		APIKey:         os.Getenv("AZURE_OPENAI_API_KEY"),
		APIVersion:     os.Getenv("AZURE_OPENAI_API_VERSION"),
		MaxSuggestions: maxSuggestions,
	})
}

// IsChatConfigured returns true if chat can be configured from environment.
func IsChatConfigured() bool {
	deployment := os.Getenv("AZURE_OPENAI_CHAT_DEPLOYMENT")
	if deployment == "" {
		deployment = os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	}
	return os.Getenv("AZURE_OPENAI_ENDPOINT") != "" &&
		deployment != "" &&
		os.Getenv("AZURE_OPENAI_API_KEY") != ""
}

// Name returns the provider name.
func (p *ChatProvider) Name() string {
	return "azure_openai_chat"
}

// Deployment returns the chat deployment name.
func (p *ChatProvider) Deployment() string {
	return p.deployment
}

// ProviderKind returns the provider kind for provenance tracking.
func (p *ChatProvider) ProviderKind() shadowllm.ProviderKind {
	return shadowllm.ProviderKindAzureOpenAI
}

// ChatResult contains the result of a chat completion.
type ChatResult struct {
	// Suggestions are the validated and sanitized suggestions.
	Suggestions []shadowllm.ShadowSuggestion

	// WhyGeneric is the generic rationale (sanitized, may be empty).
	WhyGeneric string

	// Status indicates the outcome.
	Status ChatStatus

	// ErrorBucket contains abstract error category if failed.
	ErrorBucket string

	// UsedStubFallback indicates if stub was used due to error.
	UsedStubFallback bool
}

// ChatStatus indicates the result of a chat completion.
type ChatStatus string

const (
	ChatStatusOK      ChatStatus = "ok"
	ChatStatusFail    ChatStatus = "fail"
	ChatStatusBlocked ChatStatus = "blocked"
)

// Complete performs a chat completion with privacy-safe input.
//
// CRITICAL: Input is ALWAYS privacy-guarded abstract data.
// CRITICAL: Returns validated suggestions only.
// CRITICAL: Single request - NO retries.
func (p *ChatProvider) Complete(ctx context.Context, input *privacy.ShadowInput) (*ChatResult, error) {
	result := &ChatResult{
		Status:      ChatStatusFail,
		Suggestions: nil,
	}

	// Validate input for privacy compliance
	guard := privacy.NewGuard()
	if err := guard.ValidateInput(input); err != nil {
		result.Status = ChatStatusBlocked
		result.ErrorBucket = "privacy_guard_blocked"
		return result, &ChatError{Code: "privacy_blocked", Message: "input blocked by privacy guard"}
	}

	// Render prompt
	systemPrompt, userPrompt := prompt.RenderPrompt(input)

	// Build request
	reqBody := ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature:    0.3, // Low temperature for deterministic output
		MaxTokens:      500,
		ResponseFormat: &ChatResponseFormat{Type: "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.ErrorBucket = "marshal_error"
		return result, &ChatError{Code: "marshal_error", Message: "failed to marshal request"}
	}

	// Build URL
	url := p.endpoint + "/openai/deployments/" + p.deployment + "/chat/completions?api-version=" + p.apiVersion

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		result.ErrorBucket = "request_error"
		return result, &ChatError{Code: "request_error", Message: "failed to create request"}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.apiKey)

	// Execute request (single attempt - NO retries)
	resp, err := p.client.Do(req)
	if err != nil {
		// Check for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			result.ErrorBucket = "timeout"
			return result, &ChatError{Code: "timeout", Message: "request timed out"}
		}
		result.ErrorBucket = "network_error"
		return result, &ChatError{Code: "network_error", Message: "request failed"}
	}
	defer resp.Body.Close()

	// Read response (limit size)
	bodyReader := io.LimitReader(resp.Body, 64*1024) // 64KB max
	respBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		result.ErrorBucket = "read_error"
		return result, &ChatError{Code: "read_error", Message: "failed to read response"}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		result.ErrorBucket = "http_" + chatStatusBucket(resp.StatusCode)
		return result, &ChatError{Code: result.ErrorBucket, Message: "non-200 status"}
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		result.ErrorBucket = "parse_error"
		return result, &ChatError{Code: "parse_error", Message: "failed to parse response"}
	}

	// Extract content
	if len(chatResp.Choices) == 0 {
		result.ErrorBucket = "empty_response"
		return result, &ChatError{Code: "empty_response", Message: "no choices in response"}
	}

	content := chatResp.Choices[0].Message.Content

	// Validate and parse the JSON output
	suggestions, whyGeneric, err := p.parseAndValidateOutput(content)
	if err != nil {
		result.ErrorBucket = "validation_error"
		// Still return partial result with empty suggestions
		result.Status = ChatStatusOK
		return result, nil
	}

	// Clamp to max suggestions
	if len(suggestions) > p.maxSuggestions {
		suggestions = suggestions[:p.maxSuggestions]
	}

	result.Suggestions = suggestions
	result.WhyGeneric = whyGeneric
	result.Status = ChatStatusOK

	return result, nil
}

// parseAndValidateOutput parses and validates the model output.
func (p *ChatProvider) parseAndValidateOutput(content string) ([]shadowllm.ShadowSuggestion, string, error) {
	// Try to parse as array format first (v1.1.0)
	var arrayOutput prompt.ModelOutputArraySchema
	if err := json.Unmarshal([]byte(content), &arrayOutput); err == nil && len(arrayOutput.Suggestions) > 0 {
		return p.validateSuggestionsArray(arrayOutput.Suggestions)
	}

	// Fall back to single output format (v1.0.0)
	validated := p.validator.ParseAndValidate(content)
	if !validated.IsValid {
		return nil, "", &ChatError{Code: "invalid_output", Message: validated.ValidationError}
	}

	suggestion := shadowllm.ShadowSuggestion{
		Category:       validated.Category,
		Horizon:        validated.Horizon,
		Magnitude:      validated.Magnitude,
		Confidence:     validated.Confidence,
		SuggestionType: validated.SuggestedActionClass,
	}

	return []shadowllm.ShadowSuggestion{suggestion}, validated.WhyGeneric, nil
}

// validateSuggestionsArray validates an array of suggestions.
func (p *ChatProvider) validateSuggestionsArray(raw []prompt.SuggestionSchema) ([]shadowllm.ShadowSuggestion, string, error) {
	var suggestions []shadowllm.ShadowSuggestion
	var whyGeneric string

	for _, r := range raw {
		// Validate each field
		category, ok := validate.ValidateCategory(r.Category)
		if !ok {
			continue // Skip invalid
		}

		horizon, ok := validate.ValidateHorizon(r.Horizon)
		if !ok {
			continue
		}

		magnitude, ok := validate.ValidateMagnitude(r.Magnitude)
		if !ok {
			continue
		}

		confidence, ok := validate.ValidateConfidence(r.Confidence)
		if !ok {
			continue
		}

		// Validate why_generic for privacy
		validatedWhy := ""
		if r.WhyGeneric != "" {
			if err := p.validator.ValidateWhyGeneric(r.WhyGeneric); err == nil {
				validatedWhy = r.WhyGeneric
				if whyGeneric == "" {
					whyGeneric = validatedWhy
				}
			}
		}

		suggestion := shadowllm.ShadowSuggestion{
			Category:       category,
			Horizon:        horizon,
			Magnitude:      magnitude,
			Confidence:     confidence,
			SuggestionType: shadowllm.SuggestHold, // Default
		}
		suggestions = append(suggestions, suggestion)

		if len(suggestions) >= p.maxSuggestions {
			break
		}
	}

	return suggestions, whyGeneric, nil
}

// chatStatusBucket converts HTTP status code to abstract bucket.
func chatStatusBucket(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"
	case code == 400:
		return "bad_request"
	case code == 401:
		return "unauthorized"
	case code == 403:
		return "forbidden"
	case code == 404:
		return "not_found"
	case code == 429:
		return "rate_limited"
	case code >= 500:
		return "server_error"
	default:
		return "unknown_error"
	}
}

// parseEnvInt parses an integer from environment variable.
func parseEnvInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ChatError represents an Azure OpenAI chat provider error.
//
// CRITICAL: Message is abstract - never contains API response details.
type ChatError struct {
	Code    string
	Message string
}

func (e *ChatError) Error() string {
	return "azure_openai_chat: " + e.Code + ": " + e.Message
}
