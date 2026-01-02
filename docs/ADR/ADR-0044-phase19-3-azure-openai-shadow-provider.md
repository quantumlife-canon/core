# ADR-0044: Phase 19.3 - Azure OpenAI Shadow Provider

## Status

Accepted

## Context

Phase 19.2 introduced the LLM Shadow Mode Contract with a deterministic stub provider.
Phase 19.3 extends this to support real LLM providers (Azure OpenAI) while maintaining
all safety invariants:

- Shadow mode is OBSERVATION ONLY - never affects behavior
- Privacy-safe inputs only (no raw content)
- Explicit opt-in required for real providers
- Non-determinism is acceptable but auditable
- Stub provider remains deterministic baseline

## Decision

### Core Constraints Maintained

1. **Observation Only**: Shadow mode produces metadata and suggestions but never
   modifies obligations, drafts, interruptions, or execution state.

2. **Privacy-Safe Input**: Provider receives ONLY abstract data:
   - Category buckets (money, time, work, etc.)
   - Magnitude buckets (nothing, a_few, several)
   - Horizon buckets (now, soon, later, someday)
   - Hashes of snapshots (never raw content)
   - No email addresses, subjects, bodies, vendor names, amounts

3. **Privacy-Safe Output**: Model output is strictly validated:
   - JSON schema with enum values only
   - WhyGeneric limited to 140 chars, checked for forbidden patterns
   - Invalid output results in safe defaults

4. **Explicit Opt-In**: Real providers require:
   - Config flag: `real_allowed = true`
   - Consent acknowledgment stored
   - Environment variables for credentials

5. **Non-Determinism Handling**:
   - Stub provider: Fully deterministic (same inputs + clock = same hash)
   - Real providers: Non-deterministic but auditable via provenance
   - Provenance includes: provider_kind, model/deployment, policy hash, template version

### New Components

#### Provider Kind Enum (`pkg/domain/shadowllm/types.go`)

```go
type ProviderKind string

const (
    ProviderKindNone        ProviderKind = "none"
    ProviderKindStub        ProviderKind = "stub"
    ProviderKindAzureOpenAI ProviderKind = "azure_openai"
    ProviderKindLocalSLM    ProviderKind = "local_slm"  // Placeholder
)
```

#### Provenance (`pkg/domain/shadowllm/types.go`)

```go
type Provenance struct {
    ProviderKind          ProviderKind
    ModelOrDeployment     string
    RequestPolicyHash     string
    PromptTemplateVersion string
    LatencyBucket         LatencyBucket
    Status                ReceiptStatus
    ErrorBucket           string
}
```

#### Privacy Guard (`internal/shadowllm/privacy/guard.go`)

- Validates inputs for forbidden patterns (emails, URLs, amounts, etc.)
- Builds privacy-safe input from abstract sources
- Policy version for audit trail

#### Prompt Template (`internal/shadowllm/prompt/template.go`)

- Versioned prompt template (v1.0.0)
- Instructs model to output JSON with enum values
- Explicitly forbids identifiable information in output

#### Output Validator (`internal/shadowllm/validate/validator.go`)

- Parses and validates JSON output
- Rejects output with forbidden patterns
- Returns safe defaults on invalid output

#### Azure OpenAI Provider (`internal/shadowllm/providers/azureopenai/provider.go`)

- Uses stdlib `net/http` only (no cloud SDKs)
- Single request, NO retries
- Honors context deadline
- Limits response size
- Never logs secrets or response content

### Config Extension

```
[shadow]
mode = observe
provider_kind = azure_openai
real_allowed = true
azure_endpoint = https://your-resource.openai.azure.com
azure_deployment = gpt-4o-mini
```

Environment variables (preferred for credentials):
- `AZURE_OPENAI_ENDPOINT`
- `AZURE_OPENAI_DEPLOYMENT`
- `AZURE_OPENAI_API_KEY`
- `AZURE_OPENAI_API_VERSION`

### Events

Phase 19.3 events:
- `phase19_3.azure.shadow.requested`
- `phase19_3.azure.shadow.completed`
- `phase19_3.azure.shadow.failed`
- `phase19_3.azure.shadow.timeout`
- `phase19_3.azure.shadow.not_permitted`
- `phase19_3.privacy.guard.passed`
- `phase19_3.privacy.guard.blocked`
- `phase19_3.output.validation.passed`
- `phase19_3.output.validation.failed`
- `phase19_3.shadow.consent.granted`
- `phase19_3.provider.selected`

### What Azure Provider CANNOT Do

- Modify obligations, drafts, or interruptions
- Affect routing or policy decisions
- Access raw email/calendar/finance content
- Retry failed requests
- Run in background
- Store identifiable information

### Error Handling

On failure, provider returns:
- Safe default values (ConfidenceLow, HorizonSomeday, SuggestHold)
- Abstract error bucket (network_error, timeout, parse_error, etc.)
- Receipt with ReceiptStatusFailed

No retry loops. Single request per shadow run.

### Guardrails

20+ checks in `scripts/guardrails/shadow_azure_enforced.sh`:
- No cloud SDK imports
- Stdlib net/http only
- No auto-retry patterns
- No raw email fields in prompt
- No time.Now() in shadow packages
- No goroutines
- No secret logging
- Privacy guard exists with validation
- Output validator exists
- Provenance types exist
- Default config has RealAllowed=false

### Testing

Demo tests in `internal/demo_phase19_3_azure_shadow/demo_test.go`:
- Privacy guard blocks forbidden patterns
- Output validator rejects identifiers
- Stub provider remains deterministic
- Provenance correctly populated
- Config gates real providers
- Prompt versioning works

Integration tests (skipped unless env vars present):
- Real Azure request with privacy-safe input
- Validate output shape (not determinism)

## Consequences

### Positive

- Enables real LLM observation for shadow mode
- Maintains strict privacy guarantees
- Auditable via provenance
- Safe defaults on failure
- No behavior impact

### Negative

- Additional complexity
- Non-determinism for real providers
- Requires Azure credentials for full testing

### Neutral

- Stub remains default and deterministic
- Future providers (OpenAI, Anthropic, local SLM) follow same pattern

## Future Roadmap

Phase 19.3+ may introduce:
- Additional providers (OpenAI, Anthropic Claude)
- Local SLM on-device (ProviderKindLocalSLM placeholder ready)
- Rate limiting and cost controls
- Enhanced prompt templates
- Web UI for shadow provider status

All future providers must maintain the same constraints:
- Observation only
- Privacy-safe input/output
- No influence on behavior
- Auditable provenance
