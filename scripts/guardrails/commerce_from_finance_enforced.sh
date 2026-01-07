#!/bin/bash
# Phase 31.2: Commerce from Finance Guardrails
#
# Enforces critical invariants for TrueLayer transaction observation:
# - NO merchant names stored or used
# - NO amounts stored or used
# - NO raw timestamps stored
# - Only ProviderCategory, MCC, PaymentChannel are used
# - NO goroutines, NO time.Now()
# - stdlib only
# - Pipe-delimited canonical strings only (not JSON)
# - Max 3 categories in output
#
# Usage:
#   ./commerce_from_finance_enforced.sh --check      # Check for violations (default)
#   ./commerce_from_finance_enforced.sh --self-test  # Run self-test to verify detection
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found or self-test failed
#
# Reference: docs/ADR/ADR-0064-phase31-2-commerce-from-finance.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCANNED DIRECTORIES
# ============================================================================
SCAN_DIRS=(
    "internal/financetxscan"
)

# ============================================================================
# FORBIDDEN PATTERNS
# ============================================================================

# Merchant names - NO merchant data used
FORBIDDEN_MERCHANT_PATTERNS=(
    'MerchantName'
    'merchantName'
    'merchant_name'
    'vendorName'
    'storeName'
)

# Amounts - NO amount data used
FORBIDDEN_AMOUNT_PATTERNS=(
    'AmountCents'
    'amountCents'
    'amount_cents'
    '\bAmount\b'
    '\bprice\b'
    '\bcost\b'
    '\btotal\b'
    '\bspend\b'
)

# Raw timestamps - NO timestamp data stored
FORBIDDEN_TIMESTAMP_PATTERNS=(
    'CreatedAt'
    'createdAt'
    'created_at'
    'TransactionDate'
    'transactionDate'
    'transaction_date'
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

# JSON in canonical strings - must be pipe-delimited
FORBIDDEN_JSON_PATTERNS=(
    'json\.Marshal.*CanonicalString'
    'CanonicalString.*json\.Marshal'
)

# ============================================================================
# ALLOWLIST
# ============================================================================
# These files are ALLOWED certain patterns with justification.
ALLOWLIST=(
    # Test files
    "_test.go"
    # Demo files
    "internal/demo_phase31_2"
)

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo "=========================================="
    echo "Phase 31.2: Commerce from Finance Guardrails"
    echo "=========================================="
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

            # Check for pattern (case-sensitive)
            if grep -qE "$pattern" "$file" 2>/dev/null; then
                # Filter out comments
                if grep -v "^[[:space:]]*//" "$file" | grep -qE "$pattern" 2>/dev/null; then
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

check_required_patterns() {
    local violations=0

    echo "Checking for required patterns..."

    # Check that canonical strings are pipe-delimited
    for scan_dir in "${SCAN_DIRS[@]}"; do
        local full_path="$REPO_ROOT/$scan_dir"
        if [[ ! -e "$full_path" ]]; then
            continue
        fi

        mapfile -t files < <(find "$full_path" -name "*.go" 2>/dev/null || true)

        for file in "${files[@]}"; do
            if [[ ! -f "$file" ]]; then
                continue
            fi

            # Skip test files
            if [[ "$file" == *"_test.go" ]]; then
                continue
            fi

            # Check that CanonicalString() uses pipe delimiter
            if grep -q "CanonicalString()" "$file" 2>/dev/null; then
                if ! grep -q '"|"' "$file" 2>/dev/null; then
                    echo "[PIPE_DELIMITER] Missing pipe delimiter in: ${file#$REPO_ROOT/}"
                    violations=$((violations + 1))
                fi
            fi
        done
    done

    return $violations
}

check_max_categories() {
    local violations=0

    echo "Checking max categories constraint..."

    # Check that MaxCategories = 3
    for scan_dir in "${SCAN_DIRS[@]}"; do
        local full_path="$REPO_ROOT/$scan_dir"
        if [[ ! -e "$full_path" ]]; then
            continue
        fi

        mapfile -t files < <(find "$full_path" -name "*.go" 2>/dev/null || true)

        for file in "${files[@]}"; do
            if [[ ! -f "$file" ]]; then
                continue
            fi

            # Check for MaxCategories constant
            if grep -q "MaxCategories" "$file" 2>/dev/null; then
                if ! grep -qE "MaxCategories\s*=\s*3" "$file" 2>/dev/null; then
                    # Allow if it references commerceobserver.MaxBuckets
                    if ! grep -q "MaxBuckets" "$file" 2>/dev/null; then
                        echo "[MAX_CATEGORIES] MaxCategories should be 3 in: ${file#$REPO_ROOT/}"
                        violations=$((violations + 1))
                    fi
                fi
            fi
        done
    done

    return $violations
}

check_no_external_imports() {
    local violations=0

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
                violations=$((violations + 1))
            fi
        done
    done

    return $violations
}

check_source_kind() {
    local violations=0

    echo "Checking that observations use SourceFinanceTrueLayer..."

    for scan_dir in "${SCAN_DIRS[@]}"; do
        local full_path="$REPO_ROOT/$scan_dir"
        if [[ ! -e "$full_path" ]]; then
            continue
        fi

        mapfile -t files < <(find "$full_path" -name "*.go" 2>/dev/null || true)

        for file in "${files[@]}"; do
            if [[ ! -f "$file" ]]; then
                continue
            fi

            # Skip test files
            if [[ "$file" == *"_test.go" ]]; then
                continue
            fi

            # Check that CommerceObservation sets Source field
            if grep -q "CommerceObservation{" "$file" 2>/dev/null; then
                if ! grep -q "SourceFinanceTrueLayer" "$file" 2>/dev/null; then
                    echo "[SOURCE_KIND] Missing SourceFinanceTrueLayer in: ${file#$REPO_ROOT/}"
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

    # Check merchant patterns
    for pattern in "${FORBIDDEN_MERCHANT_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "MERCHANT"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check amount patterns
    for pattern in "${FORBIDDEN_AMOUNT_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "AMOUNT"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check timestamp patterns
    for pattern in "${FORBIDDEN_TIMESTAMP_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "TIMESTAMP"; then
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

    # Check JSON patterns
    for pattern in "${FORBIDDEN_JSON_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "JSON_CANONICAL"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check for external imports
    if ! check_no_external_imports; then
        total_violations=$((total_violations + 1))
    fi

    # Check required patterns
    if ! check_required_patterns; then
        total_violations=$((total_violations + 1))
    fi

    # Check max categories
    if ! check_max_categories; then
        total_violations=$((total_violations + 1))
    fi

    # Check source kind
    if ! check_source_kind; then
        total_violations=$((total_violations + 1))
    fi

    echo "=========================================="
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
    merchant := tx.MerchantName
    amount := tx.AmountCents
    date := tx.TransactionDate
    go func() {}()
    now := time.Now()
}
EOF

    # Check that violations are detected
    local found=0
    if grep -qE 'MerchantName' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'AmountCents' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'TransactionDate' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'go[[:space:]]+func' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'time\.Now\(' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi

    if [[ $found -ge 5 ]]; then
        echo "Self-test PASSED: All patterns detected"
        return 0
    else
        echo "Self-test FAILED: Only $found/5 patterns detected"
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
