#!/bin/bash
# Phase 19.3b: Go Real Azure + Embeddings Guardrails
#
# Validates that Phase 19.3b follows all invariants for real Azure usage.
#
# CRITICAL: These checks ensure:
#   - Embeddings use ONLY safe constant input
#   - No secrets in logs or storage
#   - Stub implementations work without credentials
#   - Config correctly separates Chat vs Embed deployments
#   - ShadowRuntimeFlags are correctly populated
#   - All new events are defined
#
# Reference: docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md

set -e

ERRORS=0

error() {
    echo "[ERROR] $1"
    ERRORS=$((ERRORS + 1))
}

check() {
    echo "[CHECK] $1"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Check 1: ADR exists
# ═══════════════════════════════════════════════════════════════════════════════
check "ADR-0049 exists"
if [ ! -f "docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md" ]; then
    error "ADR-0049 not found at docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 2: Embeddings provider exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Embeddings provider exists"
if [ ! -f "internal/shadowllm/providers/azureopenai/embed.go" ]; then
    error "Embeddings provider not found at internal/shadowllm/providers/azureopenai/embed.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 3: Embeddings use safe constant input ONLY
# ═══════════════════════════════════════════════════════════════════════════════
check "Embeddings healthcheck input is safe constant"
if ! grep -q 'EmbedHealthcheckInput.*=.*"quantumlife-shadow-healthcheck"' internal/shadowllm/providers/azureopenai/embed.go; then
    error "EmbedHealthcheckInput should be 'quantumlife-shadow-healthcheck'"
fi

check "Embeddings input is ALWAYS the safe constant"
if ! grep -q 'Input:.*EmbedHealthcheckInput' internal/shadowllm/providers/azureopenai/embed.go; then
    error "Embeddings request should ALWAYS use EmbedHealthcheckInput"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 4: Embeddings output is hash only
# ═══════════════════════════════════════════════════════════════════════════════
check "Embeddings hash function exists"
if ! grep -q 'func hashEmbedding' internal/shadowllm/providers/azureopenai/embed.go; then
    error "hashEmbedding function not found"
fi

check "Embeddings return VectorHash not raw vectors"
if ! grep -q 'VectorHash.*string' internal/shadowllm/providers/azureopenai/embed.go; then
    error "EmbedHealthResult should have VectorHash field"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 5: No cloud SDKs in embeddings provider
# ═══════════════════════════════════════════════════════════════════════════════
check "No Azure SDK in embeddings provider"
if grep 'github.com/Azure' internal/shadowllm/providers/azureopenai/embed.go 2>/dev/null; then
    error "Embeddings provider should not use Azure SDK"
fi

check "Embeddings provider uses stdlib net/http"
if ! grep -q '"net/http"' internal/shadowllm/providers/azureopenai/embed.go; then
    error "Embeddings provider should use stdlib net/http"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 6: Config extensions exist
# ═══════════════════════════════════════════════════════════════════════════════
check "AzureOpenAIConfig has ChatDeployment field"
if ! grep -q 'ChatDeployment.*string' pkg/domain/config/types.go; then
    error "AzureOpenAIConfig missing ChatDeployment field"
fi

check "AzureOpenAIConfig has EmbedDeployment field"
if ! grep -q 'EmbedDeployment.*string' pkg/domain/config/types.go; then
    error "AzureOpenAIConfig missing EmbedDeployment field"
fi

check "AzureOpenAIConfig has APIKeyEnvName field"
if ! grep -q 'APIKeyEnvName.*string' pkg/domain/config/types.go; then
    error "AzureOpenAIConfig missing APIKeyEnvName field"
fi

check "GetChatDeployment method exists"
if ! grep -q 'func.*AzureOpenAIConfig.*GetChatDeployment' pkg/domain/config/types.go; then
    error "GetChatDeployment method not found"
fi

check "GetAPIKeyEnvName method exists"
if ! grep -q 'func.*AzureOpenAIConfig.*GetAPIKeyEnvName' pkg/domain/config/types.go; then
    error "GetAPIKeyEnvName method not found"
fi

check "HasEmbeddings method exists"
if ! grep -q 'func.*AzureOpenAIConfig.*HasEmbeddings' pkg/domain/config/types.go; then
    error "HasEmbeddings method not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 7: ShadowRuntimeFlags type exists
# ═══════════════════════════════════════════════════════════════════════════════
check "ShadowRuntimeFlags type exists"
if ! grep -q 'type ShadowRuntimeFlags struct' pkg/domain/config/types.go; then
    error "ShadowRuntimeFlags type not found"
fi

check "ShadowRuntimeFlags has Enabled field"
if ! grep -A15 'type ShadowRuntimeFlags struct' pkg/domain/config/types.go | grep -q 'Enabled.*bool'; then
    error "ShadowRuntimeFlags missing Enabled field"
fi

check "ShadowRuntimeFlags has ChatConfigured field"
if ! grep -A15 'type ShadowRuntimeFlags struct' pkg/domain/config/types.go | grep -q 'ChatConfigured.*bool'; then
    error "ShadowRuntimeFlags missing ChatConfigured field"
fi

check "ShadowRuntimeFlags has EmbedConfigured field"
if ! grep -A15 'type ShadowRuntimeFlags struct' pkg/domain/config/types.go | grep -q 'EmbedConfigured.*bool'; then
    error "ShadowRuntimeFlags missing EmbedConfigured field"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 8: EmbedStatus type exists
# ═══════════════════════════════════════════════════════════════════════════════
check "EmbedStatus type exists"
if ! grep -q 'type EmbedStatus string' pkg/domain/config/types.go; then
    error "EmbedStatus type not found"
fi

check "EmbedStatusOK constant exists"
if ! grep -q 'EmbedStatusOK.*EmbedStatus.*=.*"ok"' pkg/domain/config/types.go; then
    error "EmbedStatusOK constant not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 9: Stub embed healthchecker exists
# ═══════════════════════════════════════════════════════════════════════════════
check "StubEmbedHealthchecker exists"
if ! grep -q 'type StubEmbedHealthchecker struct' internal/shadowllm/engine.go; then
    error "StubEmbedHealthchecker not found in engine.go"
fi

check "Stub healthchecker implements Healthcheck"
if ! grep -q 'func.*StubEmbedHealthchecker.*Healthcheck' internal/shadowllm/engine.go; then
    error "StubEmbedHealthchecker missing Healthcheck method"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 10: Phase 19.3b events exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Phase19_3bHealthViewed event exists"
if ! grep -q 'Phase19_3bHealthViewed' pkg/events/events.go; then
    error "Phase19_3bHealthViewed event not found"
fi

check "Phase19_3bHealthRunCompleted event exists"
if ! grep -q 'Phase19_3bHealthRunCompleted' pkg/events/events.go; then
    error "Phase19_3bHealthRunCompleted event not found"
fi

check "Phase19_3bEmbedHealthCompleted event exists"
if ! grep -q 'Phase19_3bEmbedHealthCompleted' pkg/events/events.go; then
    error "Phase19_3bEmbedHealthCompleted event not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 11: No secrets in logs
# ═══════════════════════════════════════════════════════════════════════════════
check "No API key logging in embeddings provider"
if grep -iE 'log.*apikey|log.*api_key|print.*apikey|print.*api_key' internal/shadowllm/providers/azureopenai/embed.go 2>/dev/null; then
    error "Found potential API key logging in embeddings provider"
fi

check "No raw embedding logging"
if grep -iE 'log.*embedding\[|log.*Embedding\[|print.*embedding\[' internal/shadowllm/providers/azureopenai/embed.go 2>/dev/null; then
    error "Found potential raw embedding logging"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 12: Embeddings handles context deadline
# ═══════════════════════════════════════════════════════════════════════════════
check "Embeddings handles context deadline"
if ! grep -q 'context.DeadlineExceeded\|ctx.Err()' internal/shadowllm/providers/azureopenai/embed.go; then
    error "Embeddings provider should handle context deadline"
fi

check "Embeddings uses NewRequestWithContext"
if ! grep -q 'NewRequestWithContext' internal/shadowllm/providers/azureopenai/embed.go; then
    error "Embeddings provider should use NewRequestWithContext"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 13: Embeddings limits response size
# ═══════════════════════════════════════════════════════════════════════════════
check "Embeddings limits response size"
if ! grep -q 'LimitReader' internal/shadowllm/providers/azureopenai/embed.go; then
    error "Embeddings provider should use LimitReader"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 14: No goroutines in embeddings provider
# ═══════════════════════════════════════════════════════════════════════════════
check "No goroutines in embeddings provider"
if grep 'go func' internal/shadowllm/providers/azureopenai/embed.go 2>/dev/null; then
    error "Found goroutine in embeddings provider"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 15: No time.Now() in embeddings provider
# ═══════════════════════════════════════════════════════════════════════════════
check "Embeddings uses time.Now() only for latency measurement"
# Note: time.Now() is OK for latency measurement, but should not be used for receipts
if grep -c 'time\.Now()' internal/shadowllm/providers/azureopenai/embed.go 2>/dev/null | grep -v '^[01]$' > /dev/null; then
    error "Too many time.Now() calls in embeddings provider"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 16: Config loader parses new fields
# ═══════════════════════════════════════════════════════════════════════════════
check "Config loader parses azure_chat_deployment"
if ! grep -q 'azure_chat_deployment' internal/config/loader.go; then
    error "Config loader missing azure_chat_deployment parsing"
fi

check "Config loader parses azure_embed_deployment"
if ! grep -q 'azure_embed_deployment' internal/config/loader.go; then
    error "Config loader missing azure_embed_deployment parsing"
fi

check "Config loader parses azure_key_env_name"
if ! grep -q 'azure_key_env_name' internal/config/loader.go; then
    error "Config loader missing azure_key_env_name parsing"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 17: Demo tests exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Demo test file exists"
if [ ! -f "internal/demo_phase19_3b_go_real/demo_test.go" ]; then
    error "Demo test file not found"
fi

check "Demo tests have 10+ test functions"
TEST_COUNT=$(grep -c 'func Test' internal/demo_phase19_3b_go_real/demo_test.go 2>/dev/null || echo 0)
if [ "$TEST_COUNT" -lt 10 ]; then
    error "Demo tests have only $TEST_COUNT test functions (need 10+)"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 18: Web handlers exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Shadow health handler exists"
if ! grep -q 'handleShadowHealth' cmd/quantumlife-web/main.go; then
    error "handleShadowHealth function not found"
fi

check "Shadow health run handler exists"
if ! grep -q 'handleShadowHealthRun' cmd/quantumlife-web/main.go; then
    error "handleShadowHealthRun function not found"
fi

check "getShadowRuntimeFlags helper exists"
if ! grep -q 'getShadowRuntimeFlags' cmd/quantumlife-web/main.go; then
    error "getShadowRuntimeFlags function not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 19: IsEmbedConfigured function exists
# ═══════════════════════════════════════════════════════════════════════════════
check "IsEmbedConfigured function exists"
if ! grep -q 'func IsEmbedConfigured' internal/shadowllm/providers/azureopenai/embed.go; then
    error "IsEmbedConfigured function not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 20: DefaultAzureAPIKeyEnvName constant exists
# ═══════════════════════════════════════════════════════════════════════════════
check "DefaultAzureAPIKeyEnvName constant exists"
if ! grep -q 'DefaultAzureAPIKeyEnvName' pkg/domain/config/types.go; then
    error "DefaultAzureAPIKeyEnvName constant not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "=========================================="
    echo "  Phase 19.3b Go Real Azure Guardrails: PASS"
    echo "=========================================="
    exit 0
else
    echo "=========================================="
    echo "  Phase 19.3b Go Real Azure Guardrails: FAIL"
    echo "  Errors: $ERRORS"
    echo "=========================================="
    exit 1
fi
