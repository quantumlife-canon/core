#!/bin/bash
# shadow_mode_enforced.sh - Guardrail checks for Phase 19: LLM Shadow-Mode Contract
#
# Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
#
# This script validates:
# 1. No net/http imports in shadow packages
# 2. No OpenAI/Anthropic/Claude/Gemini strings
# 3. ShadowContext has no forbidden fields
# 4. No imports from shadowllm into drafts/interruptions/execution/templates
# 5. ShadowMode default is "off"
# 6. No goroutines in shadow packages
# 7. No time.Now() in shadow packages
# 8. All canonical strings are pipe-delimited

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 19: LLM Shadow-Mode Contract - Guardrail Checks          ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: No net/http imports in shadow packages
echo "Checking no net/http imports in shadow packages..."
if grep -rn 'net/http' "$PROJECT_ROOT/pkg/domain/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} net/http import found in pkg/domain/shadowllm/"
    FAILED=1
elif grep -rn 'net/http' "$PROJECT_ROOT/internal/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} net/http import found in internal/shadowllm/"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No net/http imports in shadow packages"
fi

# Check 2: No real LLM provider strings
echo "Checking no real LLM provider strings..."
LLM_PROVIDERS="OpenAI\|Anthropic\|Claude\|Gemini\|GPT-4\|gpt-4"
if grep -rni "$LLM_PROVIDERS" "$PROJECT_ROOT/pkg/domain/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} LLM provider strings found in pkg/domain/shadowllm/"
    FAILED=1
elif grep -rni "$LLM_PROVIDERS" "$PROJECT_ROOT/internal/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} LLM provider strings found in internal/shadowllm/"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No real LLM provider strings in shadow packages"
fi

# Check 3: ShadowContext has no forbidden fields (subject, body, vendor, amount)
echo "Checking ShadowContext has no forbidden fields..."
# Look at the struct definition itself, excluding comments
# The struct definition ends at the first "}"
SHADOW_CONTEXT_STRUCT=$(grep -n "type ShadowContext struct" "$PROJECT_ROOT/pkg/domain/shadowllm/interfaces.go" -A 30 | grep -v '//' | grep -v '^\s*$' | head -20)
FORBIDDEN_FIELDS="Subject\|Body\|Vendor\|Amount\|Sender\|Recipient"
if echo "$SHADOW_CONTEXT_STRUCT" | grep -i "$FORBIDDEN_FIELDS" | grep -v 'FORBIDDEN'; then
    echo -e "${RED}✗${NC} Forbidden fields found in ShadowContext struct"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} ShadowContext has no forbidden fields"
fi

# Check 4: No imports from shadowllm into drafts/interruptions/execution/templates
echo "Checking no shadowllm imports in restricted packages..."
RESTRICTED_PATHS="internal/draft\|internal/interrupt\|internal/execution\|cmd/quantumlife-web/templates"
if grep -rn 'quantumlife/pkg/domain/shadowllm\|quantumlife/internal/shadowllm' "$PROJECT_ROOT/internal/draft/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} shadowllm imported in internal/draft/"
    FAILED=1
elif grep -rn 'quantumlife/pkg/domain/shadowllm\|quantumlife/internal/shadowllm' "$PROJECT_ROOT/internal/interruptions/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} shadowllm imported in internal/interruptions/"
    FAILED=1
elif grep -rn 'quantumlife/pkg/domain/shadowllm\|quantumlife/internal/shadowllm' "$PROJECT_ROOT/internal/execution/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} shadowllm imported in internal/execution/"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No shadowllm imports in restricted packages"
fi

# Check 5: ShadowMode default is "off"
echo "Checking ShadowMode default is 'off'..."
if grep -q 'Mode:.*"off"' "$PROJECT_ROOT/pkg/domain/config/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowMode default is 'off'"
else
    echo -e "${RED}✗${NC} ShadowMode default is not 'off'"
    FAILED=1
fi

# Check 6: No goroutines in shadow packages
echo "Checking no goroutines in shadow packages..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/pkg/domain/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Goroutines found in pkg/domain/shadowllm/"
    FAILED=1
elif grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Goroutines found in internal/shadowllm/"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in shadow packages"
fi

# Check 7: No time.Now() in shadow packages
echo "Checking no time.Now() in shadow packages..."
if grep -rn 'time\.Now()' "$PROJECT_ROOT/pkg/domain/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} time.Now() found in pkg/domain/shadowllm/"
    FAILED=1
elif grep -rn 'time\.Now()' "$PROJECT_ROOT/internal/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} time.Now() found in internal/shadowllm/"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in shadow packages"
fi

# Check 8: Canonical strings use pipe delimiter (not JSON)
echo "Checking canonical strings use pipe delimiter..."
if grep -q 'SHADOW_RUN|v1|' "$PROJECT_ROOT/pkg/domain/shadowllm/hashing.go"; then
    echo -e "${GREEN}✓${NC} Canonical strings use pipe delimiter"
else
    echo -e "${RED}✗${NC} Canonical strings may not use pipe delimiter"
    FAILED=1
fi

# Check 9: ShadowSignal uses pipe delimiter
echo "Checking ShadowSignal uses pipe delimiter..."
if grep -q 'SHADOW_SIGNAL|v1|' "$PROJECT_ROOT/pkg/domain/shadowllm/hashing.go"; then
    echo -e "${GREEN}✓${NC} ShadowSignal uses pipe delimiter"
else
    echo -e "${RED}✗${NC} ShadowSignal may not use pipe delimiter"
    FAILED=1
fi

# Check 10: AbstractInputs uses pipe delimiter
echo "Checking AbstractInputs uses pipe delimiter..."
if grep -q 'ABSTRACT_INPUTS|v1' "$PROJECT_ROOT/pkg/domain/shadowllm/interfaces.go"; then
    echo -e "${GREEN}✓${NC} AbstractInputs uses pipe delimiter"
else
    echo -e "${RED}✗${NC} AbstractInputs may not use pipe delimiter"
    FAILED=1
fi

# Check 11: ShadowModel interface exists
echo "Checking ShadowModel interface exists..."
if grep -q 'type ShadowModel interface' "$PROJECT_ROOT/pkg/domain/shadowllm/interfaces.go"; then
    echo -e "${GREEN}✓${NC} ShadowModel interface exists"
else
    echo -e "${RED}✗${NC} ShadowModel interface missing"
    FAILED=1
fi

# Check 12: StubModel implements ShadowModel
echo "Checking StubModel implements ShadowModel..."
if grep -q 'var _ shadowllm.ShadowModel = ' "$PROJECT_ROOT/internal/shadowllm/stub/stub.go"; then
    echo -e "${GREEN}✓${NC} StubModel implements ShadowModel"
else
    echo -e "${RED}✗${NC} StubModel may not implement ShadowModel"
    FAILED=1
fi

# Check 13: ShadowConfig exists in config types
echo "Checking ShadowConfig exists in config types..."
if grep -q 'type ShadowConfig struct' "$PROJECT_ROOT/pkg/domain/config/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowConfig exists in config types"
else
    echo -e "${RED}✗${NC} ShadowConfig missing from config types"
    FAILED=1
fi

# Check 14: Phase 19 events exist
echo "Checking Phase 19 events exist..."
if grep -q 'Phase19ShadowRunStarted' "$PROJECT_ROOT/pkg/events/events.go"; then
    echo -e "${GREEN}✓${NC} Phase 19 shadow events exist"
else
    echo -e "${RED}✗${NC} Phase 19 shadow events missing"
    FAILED=1
fi

# Check 15: Shadow storelog record types exist
echo "Checking shadow storelog record types exist..."
if grep -q 'RecordTypeShadowLLMRun' "$PROJECT_ROOT/pkg/domain/storelog/log.go"; then
    echo -e "${GREEN}✓${NC} Shadow storelog record types exist"
else
    echo -e "${RED}✗${NC} Shadow storelog record types missing"
    FAILED=1
fi

# Check 16: ShadowLLMStore exists
echo "Checking ShadowLLMStore exists..."
if [ -f "$PROJECT_ROOT/internal/persist/shadowllm_store.go" ]; then
    echo -e "${GREEN}✓${NC} ShadowLLMStore exists"
else
    echo -e "${RED}✗${NC} ShadowLLMStore missing"
    FAILED=1
fi

# Check 17: Demo tests exist
echo "Checking Phase 19 demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase19_shadow_contract/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Phase 19 demo tests exist"
else
    echo -e "${RED}✗${NC} Phase 19 demo tests missing"
    FAILED=1
fi

# Check 18: Demo tests pass (if they exist)
echo "Checking Phase 19 demo tests pass..."
if [ -f "$PROJECT_ROOT/internal/demo_phase19_shadow_contract/demo_test.go" ]; then
    if go test -count=1 "$PROJECT_ROOT/internal/demo_phase19_shadow_contract/..." > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Phase 19 demo tests pass"
    else
        echo -e "${RED}✗${NC} Phase 19 demo tests fail"
        FAILED=1
    fi
else
    echo -e "${YELLOW}?${NC} Phase 19 demo tests not found - skipping"
fi

# Check 19: No JSON marshaling in shadow types
echo "Checking no JSON marshaling in shadow types..."
if grep -rn 'json.Marshal\|json.Unmarshal\|encoding/json' "$PROJECT_ROOT/pkg/domain/shadowllm/" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} JSON marshaling found in pkg/domain/shadowllm/"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No JSON marshaling in shadow types"
fi

# Check 20: MaxSignalsPerRun is 5
echo "Checking MaxSignalsPerRun is 5..."
if grep -q 'MaxSignalsPerRun = 5' "$PROJECT_ROOT/pkg/domain/shadowllm/types.go"; then
    echo -e "${GREEN}✓${NC} MaxSignalsPerRun is 5"
else
    echo -e "${RED}✗${NC} MaxSignalsPerRun is not 5"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 19 guardrail checks passed.${NC}"
    echo ""
    echo "Shadow mode is safe: metadata only, OFF by default, no network calls."
    exit 0
else
    echo -e "${RED}Some Phase 19 guardrail checks failed.${NC}"
    echo ""
    echo "Fix the issues above before proceeding with shadow mode."
    exit 1
fi
