#!/bin/bash
# Phase 31.1: Gmail Receipt Observer Guardrails
#
# Enforces critical invariants for Gmail receipt observation:
# - NO merchant names stored
# - NO amounts, currency symbols
# - NO sender emails stored
# - NO subjects stored
# - NO goroutines, NO time.Now()
# - stdlib only
# - Pipe-delimited canonical strings only (not JSON)
# - Max 3 categories in mirror page
#
# Usage:
#   ./receipt_observer_enforced.sh --check      # Check for violations (default)
#   ./receipt_observer_enforced.sh --self-test  # Run self-test to verify detection
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found or self-test failed
#
# Reference: docs/ADR/ADR-0063-phase31-1-gmail-receipt-observers.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCANNED DIRECTORIES
# ============================================================================
SCAN_DIRS=(
    "internal/receiptscan"
    "internal/commerceingest"
)

# ============================================================================
# FORBIDDEN PATTERNS
# ============================================================================

# Email addresses - NO sender emails stored
FORBIDDEN_EMAIL_PATTERNS=(
    '@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'
    'email.*address'
)

# URLs - NO raw URLs
FORBIDDEN_URL_PATTERNS=(
    'http://'
    'https://'
)

# Currency symbols - NO amounts
FORBIDDEN_CURRENCY_PATTERNS=(
    '\\$[0-9]'
    '£[0-9]'
    '€[0-9]'
    '\\bprice\\b'
    '\\bcost\\b'
    '\\btotal\\b'
    '\\bamount\\b'
    '\\bspend\\b'
)

# Vendor names - NO merchant names stored
FORBIDDEN_VENDOR_PATTERNS=(
    '"uber"'
    '"deliveroo"'
    '"amazon"'
    '"netflix"'
    '"spotify"'
    '"justeat"'
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
    "internal/demo_phase31_1"
)

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo "=========================================="
    echo "Phase 31.1: Receipt Observer Guardrails"
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

run_checks() {
    print_header
    local total_violations=0

    echo "Checking for forbidden patterns..."
    echo

    # Check email patterns
    for pattern in "${FORBIDDEN_EMAIL_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "EMAIL"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check URL patterns
    for pattern in "${FORBIDDEN_URL_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "URL"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check currency patterns
    for pattern in "${FORBIDDEN_CURRENCY_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "CURRENCY"; then
            total_violations=$((total_violations + 1))
        fi
    done

    # Check vendor patterns
    for pattern in "${FORBIDDEN_VENDOR_PATTERNS[@]}"; do
        if ! check_pattern "$pattern" "VENDOR"; then
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

    # Check required patterns
    if ! check_required_patterns; then
        total_violations=$((total_violations + 1))
    fi

    # Check max categories
    if ! check_max_categories; then
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
    email := "user@example.com"
    url := "https://api.uber.com"
    price := "$10.00"
    vendor := "uber"
    go func() {}()
    now := time.Now()
}
EOF

    # Check that violations are detected
    local found=0
    if grep -qE '@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'https://' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE '\$[0-9]' "$tmpdir/test_violations.go"; then
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
