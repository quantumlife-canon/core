// Package azureopenai provides an Azure OpenAI provider for shadow LLM.
//
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL INVARIANTS:
//   - Makes HTTP calls to Azure OpenAI Chat Completions API
//   - NO retries - single request only
//   - Must honor context deadline
//   - Never logs API keys or response content
//   - Returns abstract error buckets only
//   - Stdlib net/http only
//
// Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md
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

// Provider implements the Azure OpenAI shadow LLM provider.
//
// CRITICAL: This provider makes REAL network calls.
// CRITICAL: Must be explicitly enabled via config + consent.
type Provider struct {
	endpoint   string
	deployment string
	apiVersion string
	apiKey     string
	client     *http.Client
	validator  *validate.Validator
}

// Config contains Azure OpenAI provider configuration.
type Config struct {
	// Endpoint is the Azure OpenAI endpoint URL.
	// Example: "https://your-resource.openai.azure.com"
	Endpoint string

	// Deployment is the model deployment name.
	Deployment string

	// APIVersion is the Azure OpenAI API version.
	APIVersion string

	// APIKey is the API key (from environment variable).
	APIKey string
}

// NewProvider creates a new Azure OpenAI provider.
//
// CRITICAL: APIKey should come from environment variable.
func NewProvider(cfg Config) (*Provider, error) {
	// Validate required fields
	if cfg.Endpoint == "" {
		return nil, &ProviderError{Code: "missing_endpoint", Message: "endpoint is required"}
	}
	if cfg.Deployment == "" {
		return nil, &ProviderError{Code: "missing_deployment", Message: "deployment is required"}
	}
	if cfg.APIKey == "" {
		return nil, &ProviderError{Code: "missing_api_key", Message: "API key is required"}
	}

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	return &Provider{
		endpoint:   strings.TrimSuffix(cfg.Endpoint, "/"),
		deployment: cfg.Deployment,
		apiVersion: apiVersion,
		apiKey:     cfg.APIKey,
		client:     &http.Client{},
		validator:  validate.NewValidator(),
	}, nil
}

// NewProviderFromEnv creates a provider using environment variables.
//
// Environment variables:
//   - AZURE_OPENAI_ENDPOINT
//   - AZURE_OPENAI_DEPLOYMENT
//   - AZURE_OPENAI_API_KEY
//   - AZURE_OPENAI_API_VERSION (optional)
func NewProviderFromEnv() (*Provider, error) {
	return NewProvider(Config{
		Endpoint:   os.Getenv("AZURE_OPENAI_ENDPOINT"),
		Deployment: os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		APIKey:     os.Getenv("AZURE_OPENAI_API_KEY"),
		APIVersion: os.Getenv("AZURE_OPENAI_API_VERSION"),
	})
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "azure_openai"
}

// Kind returns the provider kind.
func (p *Provider) Kind() shadowllm.ProviderKind {
	return shadowllm.ProviderKindAzureOpenAI
}

// Deployment returns the model deployment name.
func (p *Provider) Deployment() string {
	return p.deployment
}

// RunResult contains the result of a shadow run.
type RunResult struct {
	// Output is the validated model output.
	Output *validate.ValidatedOutput

	// ResponseStatus is an abstract status bucket.
	ResponseStatus string

	// ErrorBucket contains an error category if failed.
	ErrorBucket string

	// Raw response is intentionally NOT included to prevent content leakage.
}

// Run executes a shadow analysis request.
//
// CRITICAL: Makes a single HTTP request - NO retries.
// CRITICAL: Must honor context deadline.
// CRITICAL: Never logs response content.
func (p *Provider) Run(ctx context.Context, input *privacy.ShadowInput) (*RunResult, error) {
	result := &RunResult{
		ResponseStatus: "unknown",
	}

	// Build request
	systemPrompt, userPrompt := prompt.RenderPrompt(input)
	reqBody := chatRequest{
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   256,
		Temperature: 0.3, // Low temperature for more deterministic output
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.ErrorBucket = "marshal_error"
		result.ResponseStatus = "error"
		return result, &ProviderError{Code: "marshal_error", Message: "failed to marshal request"}
	}

	// Build URL
	url := p.endpoint + "/openai/deployments/" + p.deployment + "/chat/completions?api-version=" + p.apiVersion

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		result.ErrorBucket = "request_error"
		result.ResponseStatus = "error"
		return result, &ProviderError{Code: "request_error", Message: "failed to create request"}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.apiKey)

	// Execute request (single attempt - NO retries)
	resp, err := p.client.Do(req)
	if err != nil {
		// Check for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			result.ErrorBucket = "timeout"
			result.ResponseStatus = "timeout"
			return result, shadowllm.ErrProviderTimeout
		}
		result.ErrorBucket = "network_error"
		result.ResponseStatus = "error"
		return result, &ProviderError{Code: "network_error", Message: "request failed"}
	}
	defer resp.Body.Close()

	// Read response (limit size to prevent memory issues)
	bodyReader := io.LimitReader(resp.Body, 64*1024) // 64KB max
	respBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		result.ErrorBucket = "read_error"
		result.ResponseStatus = "error"
		return result, &ProviderError{Code: "read_error", Message: "failed to read response"}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		result.ResponseStatus = statusBucket(resp.StatusCode)
		result.ErrorBucket = "http_" + result.ResponseStatus
		return result, &ProviderError{Code: result.ErrorBucket, Message: "non-200 status"}
	}

	result.ResponseStatus = "success"

	// Parse response
	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		result.ErrorBucket = "parse_error"
		return result, &ProviderError{Code: "parse_error", Message: "failed to parse response"}
	}

	// Extract content
	if len(chatResp.Choices) == 0 {
		result.ErrorBucket = "empty_response"
		return result, &ProviderError{Code: "empty_response", Message: "no choices in response"}
	}

	content := chatResp.Choices[0].Message.Content

	// Validate and parse content
	result.Output = p.validator.ParseAndValidate(content)

	if !result.Output.IsValid {
		result.ErrorBucket = "validation_failed"
	}

	return result, nil
}

// statusBucket converts HTTP status code to abstract bucket.
func statusBucket(code int) string {
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

// chatRequest is the Azure OpenAI chat request structure.
type chatRequest struct {
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the Azure OpenAI chat response structure.
type chatResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}

// ProviderError represents an Azure OpenAI provider error.
//
// CRITICAL: Message is abstract - never contains API response details.
type ProviderError struct {
	Code    string
	Message string
}

func (e *ProviderError) Error() string {
	return "azure_openai: " + e.Code + ": " + e.Message
}

// IsConfigured returns true if the provider can be configured from environment.
func IsConfigured() bool {
	return os.Getenv("AZURE_OPENAI_ENDPOINT") != "" &&
		os.Getenv("AZURE_OPENAI_DEPLOYMENT") != "" &&
		os.Getenv("AZURE_OPENAI_API_KEY") != ""
}
