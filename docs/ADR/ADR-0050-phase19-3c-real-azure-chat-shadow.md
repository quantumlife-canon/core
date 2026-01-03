# ADR-0050: Phase 19.3c Real Azure Chat Shadow Run

## Status

Accepted

## Context

Phase 19.3b implemented Azure OpenAI embeddings healthcheck with strict safety invariants.
Phase 19.3c extends this to chat completions with strict JSON output validation and
multiple suggestions support.

Key requirements:
- Support array of suggestions (1-5 per request)
- Strict JSON schema validation
- Privacy-safe input/output
- MaxSuggestions clamping
- Feature flags for provider selection
- No time.Now() in internal/ (clock injection)
- Stdlib net/http only

## Decision

### 1. Chat Provider Architecture

New `ChatProvider` type in `internal/shadowllm/providers/azureopenai/chat.go`:

```go
type ChatProvider struct {
    endpoint       string
    deployment     string
    apiVersion     string
    apiKey         string
    client         *http.Client
    maxSuggestions int
    validator      *validate.Validator
}
```

Key design choices:
- Separate from existing `Provider` for clear responsibility
- Uses `Complete()` method instead of `Run()` for chat-specific interface
- Returns `ChatResult` with `[]ShadowSuggestion` array
- Validator is embedded for strict output parsing

### 2. Prompt Template v1.1.0

Updated `internal/shadowllm/prompt/template.go`:

```go
const TemplateVersion = "v1.1.0"
```

Changes from v1.0.0:
- Output schema is now an array of suggestions
- Each suggestion has category, horizon, magnitude, confidence, why_generic
- Removed `suggested_action_class` in favor of implicit hold behavior

New types:
```go
type ModelOutputArraySchema struct {
    Suggestions []SuggestionSchema `json:"suggestions"`
}

type SuggestionSchema struct {
    Category   string `json:"category"`
    Horizon    string `json:"horizon"`
    Magnitude  string `json:"magnitude"`
    Confidence string `json:"confidence"`
    WhyGeneric string `json:"why_generic"`
}
```

### 3. Exported Validator Functions

Updated `internal/shadowllm/validate/validator.go`:

```go
func ValidateCategory(s string) (shadowllm.AbstractCategory, bool)
func ValidateHorizon(s string) (shadowllm.Horizon, bool)
func ValidateMagnitude(s string) (shadowllm.MagnitudeBucket, bool)
func ValidateConfidence(s string) (shadowllm.ConfidenceBucket, bool)
func (v *Validator) ValidateWhyGeneric(s string) error
```

Exported for use by `ChatProvider.validateSuggestionsArray()`.

### 4. MaxSuggestions Config

Updated `pkg/domain/config/types.go`:

```go
const DefaultMaxSuggestions = 3

type ShadowConfig struct {
    // ... existing fields
    MaxSuggestions int
}

func (c *ShadowConfig) GetMaxSuggestions() int {
    // Clamps to 1-5, defaults to 3
}
```

### 5. Provider Selection

Updated `cmd/quantumlife-web/main.go`:

New provider kind: `azure_openai_chat`

Environment variables:
- `QL_SHADOW_PROVIDER_KIND=azure_openai_chat`
- `QL_SHADOW_REAL_ALLOWED=true`
- `AZURE_OPENAI_ENDPOINT`
- `AZURE_OPENAI_CHAT_DEPLOYMENT` (or `AZURE_OPENAI_DEPLOYMENT`)
- `AZURE_OPENAI_API_KEY`
- `AZURE_OPENAI_API_VERSION` (optional)
- `SHADOW_MAX_SUGGESTIONS` (optional, defaults to 3)

### 6. Shared Types

Created `internal/shadowllm/providers/azureopenai/types.go`:

```go
type ChatRequest struct {
    Messages       []ChatMessage        `json:"messages"`
    MaxTokens      int                  `json:"max_tokens,omitempty"`
    Temperature    float64              `json:"temperature,omitempty"`
    ResponseFormat *ChatResponseFormat  `json:"response_format,omitempty"`
}

type ChatMessage struct { Role, Content string }
type ChatResponseFormat struct { Type string }
type ChatResponse struct { Choices []ChatChoice }
type ChatChoice struct { Message ChatMessage }
```

Used by both `Provider` and `ChatProvider` to avoid duplication.

## Safety Invariants

| Invariant | Enforcement |
|-----------|-------------|
| Stdlib only | Guardrail: no cloud SDK imports |
| No time.Now() | Guardrail: grep internal/shadowllm/ |
| No goroutines | Guardrail: no 'go func' patterns |
| No retries | Guardrail: no retry/backoff patterns |
| Privacy guard | Required ValidateInput() call before API |
| Output validation | All suggestions validated against schema |
| MaxSuggestions | Clamped to 1-5 in config and provider |

## Guardrails

Script: `scripts/guardrails/shadow_real_chat_enforced.sh`

35+ checks including:
1. Chat provider exists
2. Stdlib net/http only
3. No auto-retry patterns
4. Privacy guard validation
5. Output validator exports
6. Prompt template v1.1.0
7. MaxSuggestions config
8. No time.Now() in internal/shadowllm/
9. No goroutines in internal/shadowllm/
10. ChatProvider interface methods
11. Shared types exist
12. Provider selection in main.go

## Demo Tests

Package: `internal/demo_phase19_3c_real_chat_shadow/demo_test.go`

15 tests covering:
- Prompt template version
- Prompt render abstract input
- Validator exported functions
- Validator why_generic
- Validator parse single output
- MaxSuggestions default
- MaxSuggestions clamping
- ChatProvider config validation
- ChatProvider MaxSuggestions clamping
- Privacy guard valid input
- Privacy safe input builder
- Shared chat types
- Array output schema

## Consequences

### Positive
- Clear separation between chat and embeddings providers
- Strict JSON schema validation prevents malformed output
- MaxSuggestions clamping prevents runaway suggestions
- Shared types reduce duplication
- Feature flags allow gradual rollout

### Negative
- Two provider wrappers in main.go (acceptable for clarity)
- Array format breaks backward compatibility with v1.0.0 clients
- More complex validation logic

### Neutral
- Latency measurement remains in cmd/ layer
- Provider kind tracking unchanged

## References

- ADR-0043: Phase 19.2 Shadow Mode Contract
- ADR-0044: Phase 19.3 Azure OpenAI Shadow Provider
- ADR-0049: Phase 19.3b Go Real Azure + Embeddings
