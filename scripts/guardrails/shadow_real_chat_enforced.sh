#!/bin/bash
#
# Phase 19.3c: Real Azure Chat Shadow Guardrails
#
# This script enforces the safety invariants for the chat provider:
#   - Stdlib net/http only for providers
#   - No auto-retry patterns
#   - Privacy guard exists and is used
#   - Output validator exists and validates
#   - Prompt template is v1.1.0+
#   - MaxSuggestions clamping (1-5)
#   - No time.Now() in internal/shadowllm/
#   - No goroutines in internal/shadowllm/
#
# Reference: docs/ADR/ADR-0050-phase19-3c-real-azure-chat-shadow.md

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
CHAT_FILE="$ROOT_DIR/internal/shadowllm/providers/azureopenai/chat.go"
PROMPT_FILE="$ROOT_DIR/internal/shadowllm/prompt/template.go"
VALIDATOR_FILE="$ROOT_DIR/internal/shadowllm/validate/validator.go"
CONFIG_FILE="$ROOT_DIR/pkg/domain/config/types.go"

echo "Phase 19.3c: Real Azure Chat Shadow Guardrails"
echo "=============================================="

FAIL=0

check() {
    if [ $? -ne 0 ]; then
        echo "  [FAIL] $1"
        FAIL=1
    else
        echo "  [PASS] $1"
    fi
}

# -----------------------------------------------------------------------------
# 1. Chat provider exists
# -----------------------------------------------------------------------------
echo ""
echo "1. Chat Provider Existence"
test -f "$CHAT_FILE"
check "chat.go exists"

# -----------------------------------------------------------------------------
# 2. Stdlib net/http only in providers
# -----------------------------------------------------------------------------
echo ""
echo "2. Stdlib Only (no cloud SDKs)"

# No azure SDK imports
! grep -q 'github.com/Azure' "$CHAT_FILE" 2>/dev/null
check "No Azure SDK imports in chat.go"

# No openai SDK imports
! grep -q 'github.com/openai' "$CHAT_FILE" 2>/dev/null
check "No OpenAI SDK imports in chat.go"

# Uses stdlib net/http
grep -q '"net/http"' "$CHAT_FILE"
check "Uses stdlib net/http"

# -----------------------------------------------------------------------------
# 3. No auto-retry patterns
# -----------------------------------------------------------------------------
echo ""
echo "3. No Auto-Retry Patterns"

# No retry loops
! grep -q 'for.*attempt' "$CHAT_FILE" 2>/dev/null
check "No retry loops in chat.go"

! grep -q 'for.*retry' "$CHAT_FILE" 2>/dev/null
check "No retry loops (retry keyword)"

# No backoff packages
! grep -q 'backoff' "$CHAT_FILE" 2>/dev/null
check "No backoff packages"

# -----------------------------------------------------------------------------
# 4. Privacy guard validation
# -----------------------------------------------------------------------------
echo ""
echo "4. Privacy Guard"

# Complete() calls privacy guard
grep -q 'privacy.NewGuard' "$CHAT_FILE"
check "Uses privacy.NewGuard()"

grep -q 'guard.ValidateInput' "$CHAT_FILE"
check "Calls guard.ValidateInput()"

# -----------------------------------------------------------------------------
# 5. Output validator
# -----------------------------------------------------------------------------
echo ""
echo "5. Output Validator"

# Validator is used
grep -q 'validate.NewValidator' "$CHAT_FILE"
check "Uses validate.NewValidator()"

# Exported validation functions exist
grep -q 'func ValidateCategory' "$VALIDATOR_FILE"
check "ValidateCategory is exported"

grep -q 'func ValidateHorizon' "$VALIDATOR_FILE"
check "ValidateHorizon is exported"

grep -q 'func ValidateMagnitude' "$VALIDATOR_FILE"
check "ValidateMagnitude is exported"

grep -q 'func ValidateConfidence' "$VALIDATOR_FILE"
check "ValidateConfidence is exported"

grep -q 'func (v \*Validator) ValidateWhyGeneric' "$VALIDATOR_FILE"
check "ValidateWhyGeneric method is exported"

# -----------------------------------------------------------------------------
# 6. Prompt template version
# -----------------------------------------------------------------------------
echo ""
echo "6. Prompt Template"

grep -q 'TemplateVersion = "v1.1.0"' "$PROMPT_FILE"
check "Template version is v1.1.0"

# Array output schema exists
grep -q 'ModelOutputArraySchema' "$PROMPT_FILE"
check "ModelOutputArraySchema type exists"

grep -q 'SuggestionSchema' "$PROMPT_FILE"
check "SuggestionSchema type exists"

# -----------------------------------------------------------------------------
# 7. MaxSuggestions config
# -----------------------------------------------------------------------------
echo ""
echo "7. MaxSuggestions Config"

grep -q 'MaxSuggestions' "$CONFIG_FILE"
check "MaxSuggestions field in config"

grep -q 'GetMaxSuggestions' "$CONFIG_FILE"
check "GetMaxSuggestions method exists"

grep -q 'DefaultMaxSuggestions' "$CONFIG_FILE"
check "DefaultMaxSuggestions constant exists"

# -----------------------------------------------------------------------------
# 8. No time.Now() in internal/shadowllm/
# -----------------------------------------------------------------------------
echo ""
echo "8. No time.Now() in internal/shadowllm/"

# Exclude comments (lines starting with //)
! grep -r 'time.Now()' "$ROOT_DIR/internal/shadowllm/" 2>/dev/null | grep -v '^\s*//' | grep -v ':.*//.*time.Now()' | grep -q 'time.Now()'
check "No time.Now() in internal/shadowllm/"

# -----------------------------------------------------------------------------
# 9. No goroutines in internal/shadowllm/
# -----------------------------------------------------------------------------
echo ""
echo "9. No goroutines in internal/shadowllm/"

! grep -rq 'go func' "$ROOT_DIR/internal/shadowllm/" 2>/dev/null
check "No 'go func' in internal/shadowllm/"

! grep -rq 'go handler' "$ROOT_DIR/internal/shadowllm/" 2>/dev/null
check "No 'go handler' in internal/shadowllm/"

# -----------------------------------------------------------------------------
# 10. ChatProvider interface compliance
# -----------------------------------------------------------------------------
echo ""
echo "10. ChatProvider Interface"

grep -q 'func (p \*ChatProvider) Name()' "$CHAT_FILE"
check "Name() method exists"

grep -q 'func (p \*ChatProvider) Deployment()' "$CHAT_FILE"
check "Deployment() method exists"

grep -q 'func (p \*ChatProvider) ProviderKind()' "$CHAT_FILE"
check "ProviderKind() method exists"

grep -q 'func (p \*ChatProvider) Complete(' "$CHAT_FILE"
check "Complete() method exists"

# -----------------------------------------------------------------------------
# 11. Shared types
# -----------------------------------------------------------------------------
echo ""
echo "11. Shared Types"

TYPES_FILE="$ROOT_DIR/internal/shadowllm/providers/azureopenai/types.go"
test -f "$TYPES_FILE"
check "types.go exists"

grep -q 'ChatRequest' "$TYPES_FILE"
check "ChatRequest type in types.go"

grep -q 'ChatMessage' "$TYPES_FILE"
check "ChatMessage type in types.go"

grep -q 'ChatResponse' "$TYPES_FILE"
check "ChatResponse type in types.go"

# -----------------------------------------------------------------------------
# 12. Provider selection in main.go
# -----------------------------------------------------------------------------
echo ""
echo "12. Provider Selection"

MAIN_FILE="$ROOT_DIR/cmd/quantumlife-web/main.go"

grep -q 'azure_openai_chat' "$MAIN_FILE"
check "azure_openai_chat provider kind supported"

grep -q 'IsChatConfigured' "$MAIN_FILE"
check "IsChatConfigured() check in main.go"

grep -q 'NewChatProviderFromEnv' "$MAIN_FILE"
check "NewChatProviderFromEnv() used in main.go"

grep -q 'wrapAzureChatProvider' "$MAIN_FILE"
check "wrapAzureChatProvider() wrapper exists"

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "=============================================="
if [ $FAIL -eq 0 ]; then
    echo "All Phase 19.3c guardrails PASSED!"
    exit 0
else
    echo "Some Phase 19.3c guardrails FAILED!"
    exit 1
fi
