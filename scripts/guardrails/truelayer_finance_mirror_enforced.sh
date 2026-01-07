#!/usr/bin/env bash
#
# truelayer_finance_mirror_enforced.sh
#
# Phase 29: TrueLayer Read-Only Connect (UK Sandbox) + Finance Mirror Proof
#
# Enforces Phase 29 invariants:
# 1. Read-only scopes only (no payment scopes)
# 2. Bounded sync (25 items, 7 days)
# 3. Privacy guard (no raw amounts, merchants, identifiers)
# 4. No goroutines in internal packages
# 5. No time.Now() in internal packages
# 6. Hash-only storage
# 7. Web routes exist
#
# Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
#
# Exit codes:
#   0 - All checks passed
#   1 - Violations found
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Phase 29: TrueLayer Finance Mirror Enforced"
echo "=========================================="
echo "Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md"
echo ""

violations_found=0
checks_passed=0

# Helper function to count grep matches (handles empty results without pipefail issues)
# Runs in a subshell with pipefail disabled to handle grep returning 1 when no matches
count_grep() {
    local pattern="$1"
    local file="$2"
    local exclude_pattern="${3:-}"
    local result
    if [[ -n "$exclude_pattern" ]]; then
        result=$(bash -c "grep -E '$pattern' '$file' 2>/dev/null | grep -v '$exclude_pattern' | wc -l | tr -d ' '" 2>/dev/null) || result="0"
    else
        result=$(bash -c "grep -E '$pattern' '$file' 2>/dev/null | wc -l | tr -d ' '" 2>/dev/null) || result="0"
    fi
    # Ensure result is a valid number
    if [[ ! "$result" =~ ^[0-9]+$ ]]; then
        result="0"
    fi
    echo "$result"
}

# Helper function to check for pattern
check_pattern() {
    local description="$1"
    local pattern="$2"
    local path="$3"
    local should_exist="$4"  # "yes" or "no"

    count=$(grep -r "$pattern" "$path" 2>/dev/null | grep -v "_test.go" | wc -l || echo "0")

    if [[ "$should_exist" == "yes" ]]; then
        if [[ $count -gt 0 ]]; then
            echo -e "${GREEN}PASS: $description (count: $count)${NC}"
            ((checks_passed++)) || true || true
        else
            echo -e "${RED}FAIL: $description (expected to find, count: 0)${NC}"
            ((violations_found++)) || true
        fi
    else
        if [[ $count -eq 0 ]]; then
            echo -e "${GREEN}PASS: $description (not found, as expected)${NC}"
            ((checks_passed++)) || true || true
        else
            echo -e "${RED}FAIL: $description (found $count occurrences, expected 0)${NC}"
            grep -r "$pattern" "$path" 2>/dev/null | grep -v "_test.go" | head -5 || true
            ((violations_found++)) || true
        fi
    fi
}

echo ""
echo "=== Check 1: Read-Only Scopes Only ==="
echo ""

# Check TrueLayerScopes contains only read scopes
if grep -q 'TrueLayerScopes.*=.*\[\]string{' "$REPO_ROOT/internal/oauth/truelayer.go" 2>/dev/null; then
    if grep -A5 'TrueLayerScopes.*=.*\[\]string{' "$REPO_ROOT/internal/oauth/truelayer.go" | grep -qE 'payment|transfer|write|initiate'; then
        echo -e "${RED}FAIL: TrueLayerScopes contains forbidden scope patterns${NC}"
        ((violations_found++)) || true
    else
        echo -e "${GREEN}PASS: TrueLayerScopes contains only read scopes${NC}"
        ((checks_passed++)) || true
    fi
else
    echo -e "${RED}FAIL: TrueLayerScopes not found${NC}"
    ((violations_found++)) || true
fi

# Check forbidden scope patterns are defined
check_pattern "TrueLayerForbiddenScopePatterns exists" "TrueLayerForbiddenScopePatterns" "$REPO_ROOT/internal/oauth/truelayer.go" "yes"

# Check isForbiddenTrueLayerScope function exists
check_pattern "isForbiddenTrueLayerScope function exists" "func isForbiddenTrueLayerScope" "$REPO_ROOT/internal/oauth/truelayer.go" "yes"

# Check scope validation in callback
check_pattern "validateTrueLayerScopes called in callback" "validateTrueLayerScopes" "$REPO_ROOT/internal/oauth/truelayer.go" "yes"

echo ""
echo "=== Check 2: Bounded Sync Limits ==="
echo ""

# Check MaxAccountsToFetch
if grep -q 'MaxAccountsToFetch.*=.*25' "$REPO_ROOT/internal/financemirror/engine.go" 2>/dev/null; then
    echo -e "${GREEN}PASS: MaxAccountsToFetch = 25${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: MaxAccountsToFetch should be 25${NC}"
    ((violations_found++)) || true
fi

# Check MaxTransactionsToFetch
if grep -q 'MaxTransactionsToFetch.*=.*25' "$REPO_ROOT/internal/financemirror/engine.go" 2>/dev/null; then
    echo -e "${GREEN}PASS: MaxTransactionsToFetch = 25${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: MaxTransactionsToFetch should be 25${NC}"
    ((violations_found++)) || true
fi

# Check MaxSyncDays
if grep -q 'MaxSyncDays.*=.*7' "$REPO_ROOT/internal/financemirror/engine.go" 2>/dev/null; then
    echo -e "${GREEN}PASS: MaxSyncDays = 7${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: MaxSyncDays should be 7${NC}"
    ((violations_found++)) || true
fi

echo ""
echo "=== Check 3: Privacy Guard ==="
echo ""

# Check privacy guard exists
check_pattern "containsPII function exists" "func containsPII" "$REPO_ROOT/internal/financemirror/engine.go" "yes"
check_pattern "validatePrivacy function exists" "func validatePrivacy" "$REPO_ROOT/internal/financemirror/engine.go" "yes"
check_pattern "PrivacyGuard type exists" "type PrivacyGuard struct" "$REPO_ROOT/internal/financemirror/engine.go" "yes"

# Check for forbidden patterns in privacy guard
check_pattern "Email pattern check exists" "emailPattern" "$REPO_ROOT/internal/financemirror/engine.go" "yes"
check_pattern "IBAN pattern check exists" "ibanPattern" "$REPO_ROOT/internal/financemirror/engine.go" "yes"
check_pattern "Currency pattern check exists" "currencyPattern" "$REPO_ROOT/internal/financemirror/engine.go" "yes"

echo ""
echo "=== Check 4: Domain Model ==="
echo ""

# Check MagnitudeBucket exists
check_pattern "MagnitudeBucket type exists" "type MagnitudeBucket string" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

# Check CategoryBucket exists
check_pattern "CategoryBucket type exists" "type CategoryBucket string" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

# Check FinanceSyncReceipt exists
check_pattern "FinanceSyncReceipt type exists" "type FinanceSyncReceipt struct" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

# Check FinanceMirrorPage exists
check_pattern "FinanceMirrorPage type exists" "type FinanceMirrorPage struct" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

# Check CanonicalString methods exist
check_pattern "CanonicalString methods exist" "func.*CanonicalString" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

echo ""
echo "=== Check 5: No Goroutines in Internal Packages ==="
echo ""

# Check no goroutines in financemirror engine
goroutine_count=$(count_grep '\bgo\s+func|go\s+\w+\(' "$REPO_ROOT/internal/financemirror/engine.go" '//')
if [[ "$goroutine_count" -eq 0 ]]; then
    echo -e "${GREEN}PASS: No goroutines in financemirror engine${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: Found goroutines in financemirror engine${NC}"
    grep -nE '\bgo\s+func|go\s+\w+\(' "$REPO_ROOT/internal/financemirror/engine.go" 2>/dev/null | grep -v '//' || true
    ((violations_found++)) || true
fi

# Check no goroutines in persist store
goroutine_count=$(count_grep '\bgo\s+func|go\s+\w+\(' "$REPO_ROOT/internal/persist/financemirror_store.go" '//')
if [[ "$goroutine_count" -eq 0 ]]; then
    echo -e "${GREEN}PASS: No goroutines in financemirror store${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: Found goroutines in financemirror store${NC}"
    grep -nE '\bgo\s+func|go\s+\w+\(' "$REPO_ROOT/internal/persist/financemirror_store.go" 2>/dev/null | grep -v '//' || true
    ((violations_found++)) || true
fi

# Check no goroutines in oauth truelayer
goroutine_count=$(count_grep '\bgo\s+func|go\s+\w+\(' "$REPO_ROOT/internal/oauth/truelayer.go" '//')
if [[ "$goroutine_count" -eq 0 ]]; then
    echo -e "${GREEN}PASS: No goroutines in oauth truelayer${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: Found goroutines in oauth truelayer${NC}"
    grep -nE '\bgo\s+func|go\s+\w+\(' "$REPO_ROOT/internal/oauth/truelayer.go" 2>/dev/null | grep -v '//' || true
    ((violations_found++)) || true
fi

echo ""
echo "=== Check 6: No time.Now() in Internal Packages ==="
echo ""

# Check no time.Now() in financemirror (excluding comments)
timenow_count=$(count_grep 'time\.Now\(\)' "$REPO_ROOT/internal/financemirror/engine.go" '//')
if [[ "$timenow_count" -eq 0 ]]; then
    echo -e "${GREEN}PASS: No time.Now() in financemirror engine${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: Found time.Now() in financemirror engine${NC}"
    grep -n 'time\.Now()' "$REPO_ROOT/internal/financemirror/engine.go" 2>/dev/null | grep -v '//' || true
    ((violations_found++)) || true
fi

# Check no time.Now() in persist store (excluding comments)
timenow_count=$(count_grep 'time\.Now\(\)' "$REPO_ROOT/internal/persist/financemirror_store.go" '//')
if [[ "$timenow_count" -eq 0 ]]; then
    echo -e "${GREEN}PASS: No time.Now() in financemirror store${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: Found time.Now() in financemirror store${NC}"
    grep -n 'time\.Now()' "$REPO_ROOT/internal/persist/financemirror_store.go" 2>/dev/null | grep -v '//' || true
    ((violations_found++)) || true
fi

# Check no time.Now() in domain types (excluding comments)
timenow_count=$(count_grep 'time\.Now\(\)' "$REPO_ROOT/pkg/domain/financemirror/types.go" '//')
if [[ "$timenow_count" -eq 0 ]]; then
    echo -e "${GREEN}PASS: No time.Now() in domain types${NC}"
    ((checks_passed++)) || true
else
    echo -e "${RED}FAIL: Found time.Now() in domain types${NC}"
    grep -n 'time\.Now()' "$REPO_ROOT/pkg/domain/financemirror/types.go" 2>/dev/null | grep -v '//' || true
    ((violations_found++)) || true
fi

echo ""
echo "=== Check 7: Hash-Only Storage ==="
echo ""

# Check EvidenceHash field exists
check_pattern "EvidenceHash field exists" "EvidenceHash" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

# Check StatusHash field exists
check_pattern "StatusHash field exists" "StatusHash" "$REPO_ROOT/pkg/domain/financemirror/types.go" "yes"

# Check connectionHashes in store (not raw tokens)
check_pattern "connectionHashes map exists" "connectionHashes.*map\[string\]string" "$REPO_ROOT/internal/persist/financemirror_store.go" "yes"

echo ""
echo "=== Check 8: Events Defined ==="
echo ""

check_pattern "Phase29TrueLayerOAuthStart event" "Phase29TrueLayerOAuthStart" "$REPO_ROOT/pkg/events/events.go" "yes"
check_pattern "Phase29TrueLayerSyncCompleted event" "Phase29TrueLayerSyncCompleted" "$REPO_ROOT/pkg/events/events.go" "yes"
check_pattern "Phase29FinanceMirrorRendered event" "Phase29FinanceMirrorRendered" "$REPO_ROOT/pkg/events/events.go" "yes"

echo ""
echo "=== Check 9: Storelog Record Types ==="
echo ""

check_pattern "RecordTypeTrueLayerConnection exists" "RecordTypeTrueLayerConnection" "$REPO_ROOT/pkg/domain/storelog/log.go" "yes"
check_pattern "RecordTypeFinanceSyncReceipt exists" "RecordTypeFinanceSyncReceipt" "$REPO_ROOT/pkg/domain/storelog/log.go" "yes"
check_pattern "RecordTypeFinanceMirrorAck exists" "RecordTypeFinanceMirrorAck" "$REPO_ROOT/pkg/domain/storelog/log.go" "yes"

echo ""
echo "=== Check 10: Demo Tests Exist ==="
echo ""

if [[ -f "$REPO_ROOT/internal/demo_phase29_truelayer_finance_mirror/demo_test.go" ]]; then
    echo -e "${GREEN}PASS: Demo tests file exists${NC}"
    ((checks_passed++)) || true

    # Count test functions
    test_count=$(grep -c "^func Test" "$REPO_ROOT/internal/demo_phase29_truelayer_finance_mirror/demo_test.go" 2>/dev/null || echo "0")
    if [[ $test_count -ge 15 ]]; then
        echo -e "${GREEN}PASS: Demo tests have $test_count tests (required: 15+)${NC}"
        ((checks_passed++)) || true
    else
        echo -e "${RED}FAIL: Demo tests have $test_count tests (required: 15+)${NC}"
        ((violations_found++)) || true
    fi
else
    echo -e "${RED}FAIL: Demo tests file does not exist${NC}"
    ((violations_found++)) || true
fi

echo ""
echo "=== Check 11: No Forbidden Tokens in Templates ==="
echo ""

# These patterns should NOT appear in UI-facing code
forbidden_patterns=(
    "£[0-9]"
    '\$[0-9]'
    "€[0-9]"
    "iban"
    "sort.code"
    "account.number"
)

# Check domain types for forbidden patterns (excluding comments)
for pattern in "${forbidden_patterns[@]}"; do
    pattern_count=$(count_grep "$pattern" "$REPO_ROOT/pkg/domain/financemirror/types.go" '//')
    if [[ "$pattern_count" -eq 0 ]]; then
        echo -e "${GREEN}PASS: No '$pattern' in domain types${NC}"
        ((checks_passed++)) || true
    else
        echo -e "${RED}FAIL: Found '$pattern' in domain types${NC}"
        ((violations_found++)) || true
    fi
done

echo ""
echo "=== Summary ==="
echo ""
echo "Checks passed: $checks_passed"
echo "Violations found: $violations_found"
echo ""

if [[ $violations_found -gt 0 ]]; then
    echo -e "${RED}FAILED: Phase 29 guardrails violations found${NC}"
    echo ""
    echo "Phase 29 invariants enforce:"
    echo "  - Read-only TrueLayer scopes only (no payments)"
    echo "  - Bounded sync (25 items, 7 days)"
    echo "  - Privacy guard (no raw data)"
    echo "  - Hash-only storage"
    echo "  - No goroutines, no time.Now()"
    echo ""
    echo "See: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md"
    exit 1
else
    echo -e "${GREEN}PASSED: All Phase 29 guardrails passed${NC}"
    exit 0
fi
