#!/bin/bash
# Phase 19.3: Azure OpenAI Shadow Provider Guardrails
#
# Validates that Azure OpenAI shadow provider follows all invariants.
#
# CRITICAL: Azure provider must:
#   - Only be used in shadow packages
#   - Use stdlib net/http only (no cloud SDKs)
#   - Not reference raw email fields in prompt builder
#   - Have no auto-retry keywords
#   - Require consent gate for /run/shadow/real
#   - Store only abstract data
#   - Not log secrets
#
# Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md

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
# Check 1: Azure provider exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Azure OpenAI provider exists"
if [ ! -f "internal/shadowllm/providers/azureopenai/provider.go" ]; then
    error "Azure OpenAI provider not found at internal/shadowllm/providers/azureopenai/provider.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 2: No cloud SDK imports
# ═══════════════════════════════════════════════════════════════════════════════
check "No Azure SDK imports in shadow packages"
if grep -r 'github.com/Azure' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found Azure SDK import in shadow packages"
fi

check "No OpenAI SDK imports in shadow packages"
if grep -r 'github.com/openai' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found OpenAI SDK import in shadow packages"
fi

check "No sashat/go-openai imports"
if grep -r 'sashat/go-openai\|go-openai' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found go-openai SDK import in shadow packages"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 3: Stdlib net/http only
# ═══════════════════════════════════════════════════════════════════════════════
check "Azure provider uses stdlib net/http"
if ! grep -q '"net/http"' internal/shadowllm/providers/azureopenai/provider.go; then
    error "Azure provider should use stdlib net/http"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 4: No auto-retry patterns
# ═══════════════════════════════════════════════════════════════════════════════
check "No auto-retry in Azure provider"
if grep -i 'retry\|backoff\|exponential' internal/shadowllm/providers/azureopenai/provider.go 2>/dev/null | grep -v 'NO\|not\|single\|comment\|//'; then
    error "Found retry pattern in Azure provider"
fi

check "No for-loop retries in Azure provider"
if grep -E 'for\s+(i|j|attempt)\s*:?=' internal/shadowllm/providers/azureopenai/provider.go 2>/dev/null | grep -v '//' | grep -v 'range'; then
    error "Found potential retry loop in Azure provider"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 5: No raw email fields in prompt builder
# ═══════════════════════════════════════════════════════════════════════════════
check "No raw email fields in prompt template"
FORBIDDEN_FIELDS="Subject|Body|Sender|Recipient|From:|To:|EmailAddress|VendorName|Amount|AccountNumber"
if grep -E "($FORBIDDEN_FIELDS)" internal/shadowllm/prompt/template.go 2>/dev/null | grep -v '//' | grep -v 'FORBIDDEN' | grep -v 'CRITICAL' | grep -v 'MUST NOT'; then
    error "Prompt template references forbidden fields"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 6: Privacy guard exists and validates
# ═══════════════════════════════════════════════════════════════════════════════
check "Privacy guard exists"
if [ ! -f "internal/shadowllm/privacy/guard.go" ]; then
    error "Privacy guard not found at internal/shadowllm/privacy/guard.go"
fi

check "Privacy guard has validation function"
if ! grep -q 'func.*ValidateInput' internal/shadowllm/privacy/guard.go; then
    error "Privacy guard missing ValidateInput function"
fi

check "Privacy guard has forbidden patterns"
if ! grep -q 'forbiddenPatterns' internal/shadowllm/privacy/guard.go; then
    error "Privacy guard missing forbiddenPatterns"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 7: Output validator exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Output validator exists"
if [ ! -f "internal/shadowllm/validate/validator.go" ]; then
    error "Output validator not found at internal/shadowllm/validate/validator.go"
fi

check "Output validator has ParseAndValidate function"
if ! grep -q 'func.*ParseAndValidate' internal/shadowllm/validate/validator.go; then
    error "Output validator missing ParseAndValidate function"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 8: Provenance types exist
# ═══════════════════════════════════════════════════════════════════════════════
check "ProviderKind type exists"
if ! grep -q 'type ProviderKind string' pkg/domain/shadowllm/types.go; then
    error "ProviderKind type not found"
fi

check "Provenance type exists"
if ! grep -q 'type Provenance struct' pkg/domain/shadowllm/types.go; then
    error "Provenance type not found"
fi

check "LatencyBucket type exists"
if ! grep -q 'type LatencyBucket string' pkg/domain/shadowllm/types.go; then
    error "LatencyBucket type not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 9: Default config has RealAllowed=false
# ═══════════════════════════════════════════════════════════════════════════════
check "DefaultShadowConfig has RealAllowed=false"
if ! grep -A10 'func DefaultShadowConfig' pkg/domain/config/types.go | grep -q 'RealAllowed:\s*false'; then
    error "DefaultShadowConfig should have RealAllowed: false"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 10: No goroutines in shadow packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No goroutines in internal/shadowllm"
if grep -r 'go func' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in internal/shadowllm"
fi

check "No goroutines in pkg/domain/shadowllm"
if grep -r 'go func' pkg/domain/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in pkg/domain/shadowllm"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 11: No time.Now() in shadow packages (except comments)
# ═══════════════════════════════════════════════════════════════════════════════
check "No time.Now() in internal/shadowllm"
if grep -rn 'time\.Now()' internal/shadowllm/ 2>/dev/null | grep -v '_test.go' | grep -v '//' > /dev/null; then
    error "Found time.Now() in internal/shadowllm - must use clock injection"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 12: Phase 19.3 events exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Phase19_3AzureShadowRequested event exists"
if ! grep -q 'Phase19_3AzureShadowRequested' pkg/events/events.go; then
    error "Phase19_3AzureShadowRequested event not found"
fi

check "Phase19_3PrivacyGuardBlocked event exists"
if ! grep -q 'Phase19_3PrivacyGuardBlocked' pkg/events/events.go; then
    error "Phase19_3PrivacyGuardBlocked event not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 13: No secret logging in Azure provider
# ═══════════════════════════════════════════════════════════════════════════════
check "No API key logging in Azure provider"
if grep -i 'log.*apikey\|log.*api_key\|print.*apikey\|print.*api_key' internal/shadowllm/providers/azureopenai/provider.go 2>/dev/null; then
    error "Found potential API key logging in Azure provider"
fi

check "No response body logging in Azure provider"
if grep -i 'log.*respBytes\|log.*body\|print.*respBytes\|print.*body' internal/shadowllm/providers/azureopenai/provider.go 2>/dev/null | grep -v 'bodyBytes\|bodyReader'; then
    error "Found potential response body logging in Azure provider"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 14: Demo tests exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Demo test file exists"
if [ ! -f "internal/demo_phase19_3_azure_shadow/demo_test.go" ]; then
    error "Demo test file not found"
fi

check "Demo tests have 10+ test functions"
TEST_COUNT=$(grep -c 'func Test' internal/demo_phase19_3_azure_shadow/demo_test.go 2>/dev/null || echo 0)
if [ "$TEST_COUNT" -lt 10 ]; then
    error "Demo tests have only $TEST_COUNT test functions (need 10+)"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 15: Prompt template version exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Prompt template version constant exists"
if ! grep -q 'TemplateVersion.*=.*"v' internal/shadowllm/prompt/template.go; then
    error "TemplateVersion constant not found in prompt template"
fi

check "Privacy policy version constant exists"
if ! grep -q 'PolicyVersion.*=.*"v' internal/shadowllm/privacy/guard.go; then
    error "PolicyVersion constant not found in privacy guard"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 16: Azure provider handles context deadline
# ═══════════════════════════════════════════════════════════════════════════════
check "Azure provider handles context deadline"
if ! grep -q 'context.DeadlineExceeded\|ctx.Err()' internal/shadowllm/providers/azureopenai/provider.go; then
    error "Azure provider should handle context deadline"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 17: Azure provider uses NewRequestWithContext
# ═══════════════════════════════════════════════════════════════════════════════
check "Azure provider uses NewRequestWithContext"
if ! grep -q 'NewRequestWithContext' internal/shadowllm/providers/azureopenai/provider.go; then
    error "Azure provider should use NewRequestWithContext for proper context handling"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 18: No imports from shadowllm into execution packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No shadowllm imports in execution packages"
if grep -r 'shadowllm' internal/email/execution/ internal/calendar/execution/ 2>/dev/null; then
    error "Found shadowllm import in execution package"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 19: Config supports new shadow fields
# ═══════════════════════════════════════════════════════════════════════════════
check "Config has ProviderKind field"
if ! grep -q 'ProviderKind.*string' pkg/domain/config/types.go; then
    error "ShadowConfig missing ProviderKind field"
fi

check "Config has RealAllowed field"
if ! grep -q 'RealAllowed.*bool' pkg/domain/config/types.go; then
    error "ShadowConfig missing RealAllowed field"
fi

check "Config has AzureOpenAI field"
if ! grep -q 'AzureOpenAI.*AzureOpenAIConfig' pkg/domain/config/types.go; then
    error "ShadowConfig missing AzureOpenAI field"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 20: Azure provider limits response size
# ═══════════════════════════════════════════════════════════════════════════════
check "Azure provider limits response size"
if ! grep -q 'LimitReader' internal/shadowllm/providers/azureopenai/provider.go; then
    error "Azure provider should use LimitReader to limit response size"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "=========================================="
    echo "  Phase 19.3 Azure Shadow Guardrails: PASS"
    echo "=========================================="
    exit 0
else
    echo "=========================================="
    echo "  Phase 19.3 Azure Shadow Guardrails: FAIL"
    echo "  Errors: $ERRORS"
    echo "=========================================="
    exit 1
fi
