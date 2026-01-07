#!/bin/bash
# Phase 31: Commerce Observer Guardrails
#
# Enforces critical invariants for commerce observation:
# - NO amounts, NO merchant names, NO timestamps
# - NO spend analysis, NO recommendations, NO execution
# - NO goroutines, NO time.Now()
# - stdlib only
#
# Usage:
#   ./commerce_observer_enforced.sh --check      # Check for violations (default)
#   ./commerce_observer_enforced.sh --self-test  # Run self-test to verify detection
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found or self-test failed
#
# Reference: docs/ADR/ADR-0062-phase31-commerce-observers.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCANNED DIRECTORIES
# ============================================================================
SCAN_DIRS=(
    "pkg/domain/commerceobserver"
    "internal/commerceobserver"
    "internal/persist/commerceobserver_store.go"
)

# ============================================================================
# FORBIDDEN PATTERNS
# ============================================================================

# Financial values - NO amounts
FORBIDDEN_AMOUNT_PATTERNS=(
    '\bamount\b'
    '\bprice\b'
    '\bcost\b'
    '\btotal\b'
    '\bspend\b'
    '\bbalance\b'
    '\bcurrency\b'
    '\bpence\b'
    '\bcents\b'
)

# Merchant identity - NO merchant names
FORBIDDEN_MERCHANT_PATTERNS=(
    '\bmerchant\b'
    '\bvendor\b'
    '\bstore\b'
    '\bshop\b'
    '\bretailer\b'
    '\bseller\b'
)

# Spend analysis - NO insights
FORBIDDEN_ANALYSIS_PATTERNS=(
    '\bbudget\b'
    '\bsavings\b'
    '\boptimize\b'
    '\breduce\b'
    '\bcut\b'
    '\binsight\b'
    '\btrend\b.*analysis'
    '\bspending\b.*analysis'
)

# Recommendations - NO advice
FORBIDDEN_ADVICE_PATTERNS=(
    '\bsuggest\b'
    '\brecommend\b'
    '\bshould\b'
    '\bconsider\b'
    '\badvice\b'
    '\btip\b'
)

# Execution paths - NO actions
FORBIDDEN_EXECUTION_PATTERNS=(
    'func.*Execute\('
    'func.*Send\('
    'func.*Pay\('
    'func.*Transfer\('
)

# Goroutines - NO async
FORBIDDEN_GOROUTINE_PATTERNS=(
    'go[[:space:]]+func[[:space:]]*\('
    'go[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*\('
)

# time.Now() - NO nondeterminism
FORBIDDEN_TIME_PATTERNS=(
    'time\.Now\('
)

# ============================================================================
# ALLOWLIST
# ============================================================================
# These files are ALLOWED certain patterns with justification.
ALLOWLIST=(
    # Test files may use time.Now() for test setup
    "_test.go"
    # Demo files
    "internal/demo_phase31"
)

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo "========================================"
    echo "Phase 31: Commerce Observer Guardrails"
    echo "========================================"
    echo
}

is_allowlisted() {
    local file="$1"
    for pattern in "${ALLOWLIST[@]}"; do
        if [[ "$file" == *"$pattern"* ]]; then
            return 0
        fi
    done
    return 1
}

check_pattern() {
    local pattern="$1"
    local category="$2"
    local violations=0

    for scan_dir in "${SCAN_DIRS[@]}"; do
        local full_path="$REPO_ROOT/$scan_dir"
        if [[ ! -e "$full_path" ]]; then
            continue
        fi

        # Find Go files
        if [[ -f "$full_path" ]]; then
            files=("$full_path")
        else
            mapfile -t files < <(find "$full_path" -name "*.go" 2>/dev/null || true)
        fi

        for file in "${files[@]}"; do
            if [[ ! -f "$file" ]]; then
                continue
            fi

            # Skip allowlisted files
            if is_allowlisted "$file"; then
                continue
            fi

            # Check for pattern (case-insensitive for text patterns)
            if grep -iqE "$pattern" "$file" 2>/dev/null; then
                # Filter out comments
                if grep -v "^[[:space:]]*//" "$file" | grep -iqE "$pattern" 2>/dev/null; then
                    echo "[$category] Violation in: ${file#$REPO_ROOT/}"
                    grep -nE "$pattern" "$file" 2>/dev/null | head -3
                    echo
                    violations=$((violations + 1))
                fi
            fi
        done
    done

    return $violations
}

run_checks() {
    print_header
    local total_violations=0

    echo "Checking for forbidden patterns..."
    echo

    # Check amount patterns
    for pattern in "${FORBIDDEN_AMOUNT_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "AMOUNT"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check merchant patterns
    for pattern in "${FORBIDDEN_MERCHANT_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "MERCHANT"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check analysis patterns
    for pattern in "${FORBIDDEN_ANALYSIS_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "ANALYSIS"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check advice patterns
    for pattern in "${FORBIDDEN_ADVICE_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "ADVICE"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check execution patterns
    for pattern in "${FORBIDDEN_EXECUTION_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "EXECUTION"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check goroutine patterns
    for pattern in "${FORBIDDEN_GOROUTINE_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "GOROUTINE"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check time.Now() patterns
    for pattern in "${FORBIDDEN_TIME_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "TIME_NOW"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check for external imports (stdlib only)
    echo "Checking for external imports (stdlib only)..."
    for scan_dir in "${SCAN_DIRS[@]}"; do
        local full_path="$REPO_ROOT/$scan_dir"
        if [[ ! -e "$full_path" ]]; then
            continue
        fi

        if [[ -f "$full_path" ]]; then
            files=("$full_path")
        else
            mapfile -t files < <(find "$full_path" -name "*.go" 2>/dev/null || true)
        fi

        for file in "${files[@]}"; do
            if [[ ! -f "$file" ]]; then
                continue
            fi

            # Check for external imports (not stdlib, not quantumlife)
            if grep -E '^\s*"[a-z]+\.[a-z]+/' "$file" 2>/dev/null | grep -v 'quantumlife/' >/dev/null 2>&1; then
                echo "[EXTERNAL_IMPORT] Violation in: ${file#$REPO_ROOT/}"
                grep -nE '^\s*"[a-z]+\.[a-z]+/' "$file" | grep -v 'quantumlife/'
                echo
                total_violations=$((total_violations + 1))
            fi
        done
    done

    echo "========================================"
    if [[ $total_violations -eq 0 ]]; then
        echo "OK: No violations found"
        return 0
    else
        echo "FAIL: $total_violations violation(s) found"
        return 1
    fi
}

run_self_test() {
    echo "Running self-test..."
    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf $tmpdir" EXIT

    # Create test file with violations
    cat > "$tmpdir/test_violations.go" << 'EOF'
package test

import "time"

func badFunc() {
    amount := 100
    merchant := "Deliveroo"
    budget := 500
    suggest := true
    go func() {}()
    now := time.Now()
}
EOF

    # Check that violations are detected
    local found=0
    if grep -qE '\bamount\b' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE '\bmerchant\b' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE '\bbudget\b' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE '\bsuggest\b' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'go[[:space:]]+func' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'time\.Now\(' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi

    if [[ $found -ge 6 ]]; then
        echo "Self-test PASSED: All patterns detected"
        return 0
    else
        echo "Self-test FAILED: Only $found/6 patterns detected"
        return 1
    fi
}

# ============================================================================
# MAIN
# ============================================================================

main() {
    case "${1:-}" in
        --self-test)
            run_self_test
            ;;
        --check|"")
            run_checks
            ;;
        *)
            echo "Usage: $0 [--check|--self-test]"
            exit 1
            ;;
    esac
}

main "$@"
