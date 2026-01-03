// Package azureopenai provides Azure OpenAI providers for shadow LLM.
//
// Phase 19.3b: Embeddings Healthcheck Provider
//
// CRITICAL INVARIANTS:
//   - Makes HTTP calls to Azure OpenAI Embeddings API
//   - NO retries - single request only
//   - Must honor context deadline
//   - Never logs API keys or response content
//   - Input is ALWAYS a safe constant - never user data
//   - Output is hash of vector only - never raw embeddings
//   - Stdlib net/http only
//
// Reference: docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md
package azureopenai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"quantumlife/pkg/domain/config"
)

// EmbedHealthcheckInput is the ONLY input used for embeddings healthcheck.
// CRITICAL: This is a safe constant - NEVER derived from user data.
const EmbedHealthcheckInput = "quantumlife-shadow-healthcheck"

// EmbedProvider implements the Azure OpenAI embeddings provider.
//
// CRITICAL: This provider makes REAL network calls.
// CRITICAL: Input is always EmbedHealthcheckInput constant.
// CRITICAL: Output is hash only, never raw embeddings.
type EmbedProvider struct {
	endpoint   string
	deployment string
	apiVersion string
	apiKey     string
	client     *http.Client
}

// EmbedConfig contains Azure OpenAI embeddings provider configuration.
type EmbedConfig struct {
	// Endpoint is the Azure OpenAI endpoint URL.
	Endpoint string

	// Deployment is the embeddings model deployment name.
	Deployment string

	// APIVersion is the Azure OpenAI API version.
	APIVersion string

	// APIKey is the API key (from environment variable).
	APIKey string
}

// NewEmbedProvider creates a new Azure OpenAI embeddings provider.
//
// CRITICAL: APIKey should come from environment variable.
func NewEmbedProvider(cfg EmbedConfig) (*EmbedProvider, error) {
	if cfg.Endpoint == "" {
		return nil, &EmbedError{Code: "missing_endpoint", Message: "endpoint is required"}
	}
	if cfg.Deployment == "" {
		return nil, &EmbedError{Code: "missing_deployment", Message: "deployment is required"}
	}
	if cfg.APIKey == "" {
		return nil, &EmbedError{Code: "missing_api_key", Message: "API key is required"}
	}

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	return &EmbedProvider{
		endpoint:   strings.TrimSuffix(cfg.Endpoint, "/"),
		deployment: cfg.Deployment,
		apiVersion: apiVersion,
		apiKey:     cfg.APIKey,
		client:     &http.Client{},
	}, nil
}

// NewEmbedProviderFromEnv creates an embeddings provider using environment variables.
//
// Environment variables:
//   - AZURE_OPENAI_ENDPOINT
//   - AZURE_OPENAI_EMBED_DEPLOYMENT
//   - AZURE_OPENAI_API_KEY
//   - AZURE_OPENAI_API_VERSION (optional)
func NewEmbedProviderFromEnv() (*EmbedProvider, error) {
	return NewEmbedProvider(EmbedConfig{
		Endpoint:   os.Getenv("AZURE_OPENAI_ENDPOINT"),
		Deployment: os.Getenv("AZURE_OPENAI_EMBED_DEPLOYMENT"),
		APIKey:     os.Getenv("AZURE_OPENAI_API_KEY"),
		APIVersion: os.Getenv("AZURE_OPENAI_API_VERSION"),
	})
}

// IsEmbedConfigured returns true if embeddings can be configured from environment.
func IsEmbedConfigured() bool {
	return os.Getenv("AZURE_OPENAI_ENDPOINT") != "" &&
		os.Getenv("AZURE_OPENAI_EMBED_DEPLOYMENT") != "" &&
		os.Getenv("AZURE_OPENAI_API_KEY") != ""
}

// Name returns the provider name.
func (p *EmbedProvider) Name() string {
	return "azure_openai_embed"
}

// Deployment returns the embeddings deployment name.
func (p *EmbedProvider) Deployment() string {
	return p.deployment
}

// EmbedHealthResult contains the result of an embeddings healthcheck.
type EmbedHealthResult struct {
	// Status is the health status.
	Status config.EmbedStatus

	// LatencyBucket indicates response latency.
	LatencyBucket string

	// VectorHash is SHA256 of the embedding vector bytes.
	VectorHash string

	// ErrorBucket contains abstract error category if failed.
	ErrorBucket string
}

// Healthcheck performs a single embeddings call with the safe constant input.
//
// CRITICAL: Input is ALWAYS EmbedHealthcheckInput - never user data.
// CRITICAL: Returns only hash of vector - never raw embeddings.
// CRITICAL: Single request - NO retries.
func (p *EmbedProvider) Healthcheck(ctx context.Context) (*EmbedHealthResult, error) {
	startTime := time.Now()

	result := &EmbedHealthResult{
		Status:        config.EmbedStatusFail,
		LatencyBucket: "na",
	}

	// Build request with safe constant input
	reqBody := embedRequest{
		Input: EmbedHealthcheckInput, // CRITICAL: Safe constant only
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.ErrorBucket = "marshal_error"
		return result, &EmbedError{Code: "marshal_error", Message: "failed to marshal request"}
	}

	// Build URL
	url := p.endpoint + "/openai/deployments/" + p.deployment + "/embeddings?api-version=" + p.apiVersion

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		result.ErrorBucket = "request_error"
		return result, &EmbedError{Code: "request_error", Message: "failed to create request"}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.apiKey)

	// Execute request (single attempt - NO retries)
	resp, err := p.client.Do(req)
	if err != nil {
		// Check for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			result.ErrorBucket = "timeout"
			result.LatencyBucket = "timeout"
			return result, &EmbedError{Code: "timeout", Message: "request timed out"}
		}
		result.ErrorBucket = "network_error"
		return result, &EmbedError{Code: "network_error", Message: "request failed"}
	}
	defer resp.Body.Close()

	// Calculate latency bucket
	elapsed := time.Since(startTime)
	result.LatencyBucket = latencyBucket(elapsed)

	// Read response (limit size)
	bodyReader := io.LimitReader(resp.Body, 256*1024) // 256KB max for embeddings
	respBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		result.ErrorBucket = "read_error"
		return result, &EmbedError{Code: "read_error", Message: "failed to read response"}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		result.ErrorBucket = "http_" + embedStatusBucket(resp.StatusCode)
		return result, &EmbedError{Code: result.ErrorBucket, Message: "non-200 status"}
	}

	// Parse response
	var embedResp embedResponse
	if err := json.Unmarshal(respBytes, &embedResp); err != nil {
		result.ErrorBucket = "parse_error"
		return result, &EmbedError{Code: "parse_error", Message: "failed to parse response"}
	}

	// Extract vector and compute hash
	if len(embedResp.Data) == 0 || len(embedResp.Data[0].Embedding) == 0 {
		result.ErrorBucket = "empty_response"
		return result, &EmbedError{Code: "empty_response", Message: "no embeddings in response"}
	}

	// Hash the vector (NEVER store raw embeddings)
	vectorHash := hashEmbedding(embedResp.Data[0].Embedding)
	result.VectorHash = vectorHash
	result.Status = config.EmbedStatusOK

	return result, nil
}

// hashEmbedding creates a SHA256 hash of the embedding vector.
// This provides a deterministic fingerprint without storing raw embeddings.
func hashEmbedding(embedding []float64) string {
	// Convert floats to deterministic string representation
	var b bytes.Buffer
	for i, f := range embedding {
		if i > 0 {
			b.WriteByte('|')
		}
		// Use fixed-precision format for determinism
		b.WriteString(floatToString(f))
	}
	h := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(h[:])
}

// floatToString converts float64 to string with fixed precision.
func floatToString(f float64) string {
	// Manual conversion without strconv for stdlib purity
	// Using 8 decimal places for precision
	if f < 0 {
		return "-" + floatToString(-f)
	}

	intPart := int64(f)
	fracPart := int64((f - float64(intPart)) * 100000000)

	intStr := int64ToString(intPart)
	fracStr := int64ToString(fracPart)

	// Pad fractional part to 8 digits
	for len(fracStr) < 8 {
		fracStr = "0" + fracStr
	}

	return intStr + "." + fracStr
}

// int64ToString converts int64 to string without strconv.
func int64ToString(n int64) string {
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

// latencyBucket converts duration to abstract bucket.
func latencyBucket(d time.Duration) string {
	switch {
	case d < time.Second:
		return "fast"
	case d < 5*time.Second:
		return "medium"
	default:
		return "slow"
	}
}

// embedStatusBucket converts HTTP status code to abstract bucket.
func embedStatusBucket(code int) string {
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

// embedRequest is the Azure OpenAI embeddings request structure.
type embedRequest struct {
	Input string `json:"input"`
}

// embedResponse is the Azure OpenAI embeddings response structure.
type embedResponse struct {
	Data []embedData `json:"data"`
}

type embedData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbedError represents an Azure OpenAI embeddings provider error.
//
// CRITICAL: Message is abstract - never contains API response details.
type EmbedError struct {
	Code    string
	Message string
}

func (e *EmbedError) Error() string {
	return "azure_openai_embed: " + e.Code + ": " + e.Message
}
