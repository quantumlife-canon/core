# ADR-0049: Phase 19.3b - Go Real Azure Shadow + Embeddings Healthcheck

## Status

Accepted

## Context

Phase 19.3 established the Azure OpenAI shadow provider architecture with:
- Privacy guard preventing identifiable information in inputs
- Prompt templates producing abstract-only outputs
- Output validator ensuring no content leakage
- Provenance tracking for auditability

However, the system remains "mock-heavy" with no clear path to verify real Azure
connectivity without manual testing. We need:

1. **Real Azure verification** - Prove the shadow pipeline works with actual Azure OpenAI
2. **Embeddings readiness** - Wire up embeddings for future RAG without enabling RAG yet
3. **Health visibility** - A single page showing provider status and last receipt
4. **CI-capable testing** - Tests that prove real-capability without requiring actual keys

## Decision

### A) Configuration Model

Extend shadow configuration with explicit Azure settings:

```go
// ShadowAzureConfig contains Azure OpenAI provider configuration.
type ShadowAzureConfig struct {
    Endpoint        string // Azure OpenAI endpoint URL
    APIKeyEnvName   string // Env var name containing API key (default: AZURE_OPENAI_API_KEY)
    APIVersion      string // API version (default: 2024-02-15-preview)
    ChatDeployment  string // Chat model deployment name
    EmbedDeployment string // Embeddings model deployment name (optional)
}
```

Key principles:
- **No secrets in config files** - Only env var names, never actual keys
- **Explicit deployments** - Separate chat and embeddings deployment names
- **Fail-fast** - Missing required env vars produce clear errors at startup

### B) Embeddings Provider

Add minimal embeddings support for healthcheck only:

```go
// EmbedHealthcheck performs a single embeddings call with a safe constant input.
// Returns only a hash of the result vector, never the raw embeddings.
func (p *EmbedProvider) Healthcheck(ctx context.Context) (*EmbedHealthResult, error)
```

Privacy constraints:
- Input is ALWAYS the constant `"quantumlife-shadow-healthcheck"` - never user data
- Output is SHA256 hash of vector bytes - never raw embeddings
- Single request, no retries
- Never logs secrets or response content

### C) Receipt Extension

Extend ShadowReceipt with embeddings healthcheck fields:

```go
type EmbedHealth struct {
    Status        EmbedStatus   // ok | fail | skipped | not_configured
    LatencyBucket LatencyBucket // fast | medium | slow | timeout | na
    VectorHash    string        // SHA256 of vector bytes (deterministic)
}
```

Receipt hashing bumped to v3 to incorporate embed health fields.

### D) Health Endpoint

Add `/shadow/health` endpoint showing:
- Provider kind (stub/azure_openai)
- RealAllowed status
- Chat deployment configured
- Embed deployment configured
- Last receipt summary
- Privacy reassurance

Add `/shadow/health/run` POST to trigger a safe demo shadow run.

### E) Testing Strategy

Tests use `httptest` to mock Azure endpoints:
- Chat completion returns valid JSON with abstract suggestions
- Embeddings returns mock vector for hash verification
- No actual Azure calls in CI

## Consequences

### Positive

- **Verifiable real mode** - Clear proof that Azure integration works
- **Embeddings ready** - Infrastructure in place for future RAG
- **No behavior change** - Shadow mode remains observation-only
- **CI-safe** - All tests pass without real credentials
- **Privacy preserved** - Embeddings only ever see safe constant input

### Negative

- **Added complexity** - More config surface area
- **Not full RAG** - Embeddings are healthcheck-only, not used for retrieval

### Neutral

- **Key management** - Local env vars now, Key Vault integration later

## Key/Environment Management

Current (Phase 19.3b):
```bash
export AZURE_OPENAI_API_KEY="your-key"
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
```

Future (Phase 20+):
- Azure Key Vault integration for production
- Managed identity for Azure-hosted deployments

## Privacy Proof

The embeddings healthcheck is privacy-safe because:

1. **Constant input** - Always `"quantumlife-shadow-healthcheck"`, never derived from user data
2. **Hash-only output** - Only SHA256 of vector stored, never raw embeddings
3. **No RAG yet** - Embeddings not used for retrieval or similarity search
4. **Audit trail** - EmbedHealth in receipt proves what was sent/received

## Determinism

For stub provider, embeddings healthcheck:
- Returns deterministic hash based on deployment name + API version
- Allows replay verification without network calls

For azure provider:
- Actual Azure response may vary slightly
- Hash captures the specific response for audit
- Provenance tracks which deployment was called

## References

- ADR-0043: Phase 19 Shadow Mode Contract
- ADR-0044: Phase 19.3 Azure OpenAI Shadow Provider
- docs/REAL_KEYS_LOCAL_RUNBOOK_V1.md
