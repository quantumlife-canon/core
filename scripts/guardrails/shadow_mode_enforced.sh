#!/bin/bash
# Phase 19.2: Shadow Mode Guardrails
#
# Validates that shadow mode implementation follows all invariants.
#
# CRITICAL: Shadow mode must:
#   - Be observation ONLY - no state modification
#   - Use stub provider ONLY - no real LLM API calls
#   - Be OFF by default - explicit user action required
#   - Store ONLY abstract data - no raw content
#   - Use clock injection - no time.Now()
#   - No goroutines - synchronous only
#   - No HTTP calls in shadow packages
#
# Reference: docs/ADR/ADR-0043-phase19-2-shadow-mode-contract.md

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
# Check 1: No network calls in shadowllm packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No net/http imports in internal/shadowllm"
if grep -r '"net/http"' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found net/http import in internal/shadowllm (non-test)"
fi

check "No http.Client in internal/shadowllm"
if grep -r 'http\.Client' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found http.Client in internal/shadowllm (non-test)"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 2: No real LLM provider strings
# ═══════════════════════════════════════════════════════════════════════════════
check "No OpenAI strings in shadowllm packages"
if grep -ri 'openai' internal/shadowllm/ pkg/domain/shadowllm/ 2>/dev/null | grep -v '_test.go' | grep -v 'TODO'; then
    error "Found 'openai' string in shadow packages"
fi

check "No Anthropic strings in shadowllm packages"
if grep -ri 'anthropic\|claude' internal/shadowllm/ pkg/domain/shadowllm/ 2>/dev/null | grep -v '_test.go' | grep -v 'TODO'; then
    error "Found 'anthropic/claude' string in shadow packages"
fi

check "No Gemini strings in shadowllm packages"
if grep -ri 'gemini' internal/shadowllm/ pkg/domain/shadowllm/ 2>/dev/null | grep -v '_test.go' | grep -v 'TODO'; then
    error "Found 'gemini' string in shadow packages"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 3: No goroutines in shadow packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No goroutines in pkg/domain/shadowllm"
if grep -r 'go func' pkg/domain/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in pkg/domain/shadowllm"
fi

check "No goroutines in internal/shadowllm"
if grep -r 'go func' internal/shadowllm/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in internal/shadowllm"
fi

check "No goroutines in internal/persist/shadow_receipt_store.go"
if grep 'go func' internal/persist/shadow_receipt_store.go 2>/dev/null; then
    error "Found goroutine in shadow_receipt_store.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 4: No time.Now() in shadow packages (excluding comments)
# We're looking for actual code using time.Now(), not comments mentioning it
# ═══════════════════════════════════════════════════════════════════════════════
check "No time.Now() in pkg/domain/shadowllm"
# Look for lines with time.Now() that are NOT comments (no // before time.Now)
if grep -rn 'time\.Now()' pkg/domain/shadowllm/ 2>/dev/null | grep -v '_test.go' | grep -v '//' > /dev/null; then
    error "Found time.Now() in pkg/domain/shadowllm - must use clock injection"
fi

check "No time.Now() in internal/shadowllm"
if grep -rn 'time\.Now()' internal/shadowllm/ 2>/dev/null | grep -v '_test.go' | grep -v '//' > /dev/null; then
    error "Found time.Now() in internal/shadowllm - must use clock injection"
fi

check "No time.Now() in shadow_receipt_store.go"
if grep -n 'time\.Now()' internal/persist/shadow_receipt_store.go 2>/dev/null | grep -v '//' > /dev/null; then
    error "Found time.Now() in shadow_receipt_store.go - must use clock injection"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 5: /run/shadow exists and is POST-only
# ═══════════════════════════════════════════════════════════════════════════════
check "/run/shadow route is registered"
if ! grep -q '/run/shadow' cmd/quantumlife-web/main.go; then
    error "/run/shadow route not found"
fi

check "/run/shadow handler enforces POST"
if ! grep -A5 'handleShadowRun' cmd/quantumlife-web/main.go | grep -q 'Method.*POST'; then
    error "/run/shadow does not enforce POST method"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 6: Shadow store uses storelog
# ═══════════════════════════════════════════════════════════════════════════════
check "Shadow receipt store uses storelog"
if ! grep -q 'storelog' internal/persist/shadow_receipt_store.go; then
    error "shadow_receipt_store.go does not reference storelog"
fi

check "RecordTypeShadowLLMReceipt exists"
if ! grep -q 'RecordTypeShadowLLMReceipt' pkg/domain/storelog/log.go; then
    error "RecordTypeShadowLLMReceipt not found in storelog"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 7: Only stub provider exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Stub provider exists"
if [ ! -f "internal/shadowllm/stub/stub.go" ]; then
    error "Stub provider not found at internal/shadowllm/stub/stub.go"
fi

check "No real LLM providers exist yet"
for dir in internal/shadowllm/providers/*/; do
    if [ -d "$dir" ] && [ "$(basename "$dir")" != "stub" ]; then
        error "Found non-stub provider: $dir"
    fi
done

# ═══════════════════════════════════════════════════════════════════════════════
# Check 8: Canonical strings use pipe delimiter (not JSON)
# ═══════════════════════════════════════════════════════════════════════════════
check "ShadowReceipt uses pipe-delimited canonical string"
if ! grep -q 'SHADOW_RECEIPT|v1|' pkg/domain/shadowllm/hashing.go; then
    error "ShadowReceipt canonical string missing pipe-delimited prefix"
fi

check "ShadowSuggestion uses pipe-delimited canonical string"
if ! grep -q 'SHADOW_SUGGESTION|v1|' pkg/domain/shadowllm/hashing.go; then
    error "ShadowSuggestion canonical string missing pipe-delimited prefix"
fi

check "No json.Marshal in shadowllm hashing"
if grep -r 'json\.Marshal' pkg/domain/shadowllm/hashing.go 2>/dev/null; then
    error "Found json.Marshal in hashing.go - must use pipe-delimited strings"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 9: No forbidden fields in ShadowContext/ShadowInputDigest
# ═══════════════════════════════════════════════════════════════════════════════
check "ShadowInputDigest has no forbidden fields"
FORBIDDEN_FIELDS="Subject|Body|Sender|Recipient|Amount|VendorName|RawContent|MessageID"
if grep -E "($FORBIDDEN_FIELDS)" pkg/domain/shadowllm/types.go | grep -v '//' | grep -v 'FORBIDDEN' | grep -v 'CRITICAL'; then
    error "ShadowInputDigest may contain forbidden fields"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 10: No imports from shadowllm into execution/drafts packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No shadowllm imports in execution packages"
if grep -r 'shadowllm' internal/email/execution/ internal/calendar/execution/ 2>/dev/null; then
    error "Found shadowllm import in execution package"
fi

check "No shadowllm imports in drafts packages"
if grep -r 'shadowllm' internal/drafts/ 2>/dev/null | grep -v '_test.go'; then
    error "Found shadowllm import in drafts package"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 11: Phase 19.2 events exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Phase19_2ShadowRequested event exists"
if ! grep -q 'Phase19_2ShadowRequested' pkg/events/events.go; then
    error "Phase19_2ShadowRequested event not found"
fi

check "Phase19_2ShadowComputed event exists"
if ! grep -q 'Phase19_2ShadowComputed' pkg/events/events.go; then
    error "Phase19_2ShadowComputed event not found"
fi

check "Phase19_2ShadowPersisted event exists"
if ! grep -q 'Phase19_2ShadowPersisted' pkg/events/events.go; then
    error "Phase19_2ShadowPersisted event not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 12: ShadowReceipt validation enforces limits
# ═══════════════════════════════════════════════════════════════════════════════
check "MaxSuggestionsPerReceipt constant exists"
if ! grep -q 'MaxSuggestionsPerReceipt.*=.*5' pkg/domain/shadowllm/types.go; then
    error "MaxSuggestionsPerReceipt not set to 5"
fi

check "ShadowReceipt validates suggestion count"
if ! grep -A30 'func (r \*ShadowReceipt) Validate()' pkg/domain/shadowllm/types.go | grep -q 'MaxSuggestionsPerReceipt'; then
    error "ShadowReceipt.Validate() does not check MaxSuggestionsPerReceipt"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 13: Demo tests exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Demo test file exists"
if [ ! -f "internal/demo_phase19_2_shadow_mode/demo_test.go" ]; then
    error "Demo test file not found"
fi

check "Demo tests have 10+ test functions"
TEST_COUNT=$(grep -c 'func Test' internal/demo_phase19_2_shadow_mode/demo_test.go 2>/dev/null || echo 0)
if [ "$TEST_COUNT" -lt 10 ]; then
    error "Demo tests have only $TEST_COUNT test functions (need 10+)"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 14: Whisper link on /today template
# ═══════════════════════════════════════════════════════════════════════════════
check "Shadow whisper link exists in today template"
if ! grep -q 'shadow-whisper' cmd/quantumlife-web/main.go; then
    error "Shadow whisper section not found in today template"
fi

check "Whisper link uses POST form"
if ! grep -q 'action="/run/shadow" method="POST"' cmd/quantumlife-web/main.go; then
    error "Shadow whisper link does not use POST form"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 15: Shadow mode categories include required values
# ═══════════════════════════════════════════════════════════════════════════════
check "AbstractCategory includes all required values"
for cat in money time work people home health family school unknown; do
    if ! grep -qi "Category.*=.*\"$cat\"" pkg/domain/shadowllm/types.go 2>/dev/null; then
        error "AbstractCategory missing: $cat"
    fi
done

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "=========================================="
    echo "  Phase 19.2 Shadow Mode Guardrails: PASS"
    echo "=========================================="
    exit 0
else
    echo "=========================================="
    echo "  Phase 19.2 Shadow Mode Guardrails: FAIL"
    echo "  Errors: $ERRORS"
    echo "=========================================="
    exit 1
fi
