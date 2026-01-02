#!/usr/bin/env bash
# Guardrail: Phase 18.8 OAuth Gmail Read-Only Enforcement
#
# This script ensures:
# 1. Gmail OAuth scopes are read-only only (gmail.readonly)
# 2. No write scopes are allowed in Gmail-related code
# 3. OAuth flows use deterministic state management
# 4. No goroutines in OAuth handlers
#
# Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

FAILED=0

echo "=== Phase 18.8: OAuth Gmail Read-Only Enforcement ==="
echo ""

# Check 1: Ensure gmail.readonly is the only Gmail scope
echo "Check 1: Gmail scopes must be read-only..."
GMAIL_WRITE_SCOPES=$(grep -rn 'gmail\.\(send\|modify\|compose\|insert\|labels\|settings\)' \
    internal/oauth/ internal/integrations/gmail_read/ pkg/events/events.go 2>/dev/null || true)
if [ -n "$GMAIL_WRITE_SCOPES" ]; then
    echo "FAIL: Found Gmail write scopes:"
    echo "$GMAIL_WRITE_SCOPES"
    FAILED=1
else
    echo "PASS: No Gmail write scopes found"
fi

# Check 2: Ensure GmailScopes only contains gmail.readonly
echo ""
echo "Check 2: GmailScopes variable must only contain gmail.readonly..."
GMAIL_SCOPES_DEF=$(grep -n 'GmailScopes.*=' internal/oauth/gmail.go 2>/dev/null || true)
if echo "$GMAIL_SCOPES_DEF" | grep -q 'gmail\.readonly'; then
    if echo "$GMAIL_SCOPES_DEF" | grep -v 'gmail\.readonly' | grep -qE 'gmail\.[a-z]+'; then
        echo "FAIL: GmailScopes contains non-readonly scopes"
        echo "$GMAIL_SCOPES_DEF"
        FAILED=1
    else
        echo "PASS: GmailScopes only contains gmail.readonly"
    fi
else
    echo "FAIL: GmailScopes not found or doesn't contain gmail.readonly"
    FAILED=1
fi

# Check 3: No write methods in Gmail handlers
echo ""
echo "Check 3: Gmail handlers must not have write methods..."
WRITE_METHODS=$(grep -rn 'Send\|Compose\|Draft\|Delete\|Modify\|Update' \
    internal/oauth/gmail.go 2>/dev/null | grep -v '// ' || true)
if [ -n "$WRITE_METHODS" ]; then
    echo "FAIL: Found potential write methods in Gmail handlers:"
    echo "$WRITE_METHODS"
    FAILED=1
else
    echo "PASS: No write methods found in Gmail handlers"
fi

# Check 4: OAuth state uses HMAC for CSRF protection
echo ""
echo "Check 4: OAuth state must use HMAC for CSRF protection..."
if grep -q 'hmac\.New\|crypto/hmac' internal/oauth/state.go 2>/dev/null; then
    echo "PASS: OAuth state uses HMAC"
else
    echo "FAIL: OAuth state does not use HMAC for CSRF protection"
    FAILED=1
fi

# Check 5: No goroutines in OAuth code
echo ""
echo "Check 5: No goroutines in OAuth code..."
GOROUTINES=$(grep -rn 'go func\|go [a-z]' internal/oauth/ 2>/dev/null | grep -v '// ' | grep -v '_test\.go' || true)
if [ -n "$GOROUTINES" ]; then
    echo "FAIL: Found goroutines in OAuth code:"
    echo "$GOROUTINES"
    FAILED=1
else
    echo "PASS: No goroutines in OAuth code"
fi

# Check 6: OAuth receipts don't contain tokens
echo ""
echo "Check 6: OAuth receipts must not contain tokens..."
TOKEN_IN_RECEIPTS=$(grep -rn 'Token.*string' internal/oauth/receipts.go 2>/dev/null | grep -v 'TokenHandle' || true)
if [ -n "$TOKEN_IN_RECEIPTS" ]; then
    echo "FAIL: Found token fields in OAuth receipts:"
    echo "$TOKEN_IN_RECEIPTS"
    FAILED=1
else
    echo "PASS: OAuth receipts don't contain tokens"
fi

# Check 7: Revocation is idempotent
echo ""
echo "Check 7: Revocation must be idempotent..."
if grep -q 'idempotent' internal/oauth/gmail.go 2>/dev/null; then
    echo "PASS: Revocation is documented as idempotent"
else
    echo "WARN: Revocation idempotency not explicitly documented"
fi

# Check 8: All OAuth events use Phase18_8 prefix
echo ""
echo "Check 8: OAuth events must use Phase18_8 prefix..."
OAUTH_EVENTS=$(grep -n 'Phase18_8' pkg/events/events.go 2>/dev/null | wc -l)
if [ "$OAUTH_EVENTS" -ge 8 ]; then
    echo "PASS: Found $OAUTH_EVENTS Phase18_8 events"
else
    echo "FAIL: Expected at least 8 Phase18_8 events, found $OAUTH_EVENTS"
    FAILED=1
fi

# Check 9: Gmail adapter is read-only
echo ""
echo "Check 9: Gmail adapter must be read-only..."
if grep -q 'READ-ONLY' internal/integrations/gmail_read/adapter.go 2>/dev/null; then
    echo "PASS: Gmail adapter documented as read-only"
else
    echo "FAIL: Gmail adapter not documented as read-only"
    FAILED=1
fi

echo ""
echo "=== Summary ==="
if [ $FAILED -eq 0 ]; then
    echo "All OAuth Gmail read-only guardrails passed!"
    exit 0
else
    echo "Some guardrails failed. Please fix the issues above."
    exit 1
fi
