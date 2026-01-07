#!/bin/bash
# Phase 31.3: Real Finance Only Guardrails
#
# Enforces critical invariants for real-only finance commerce observation:
# - Provider field is REQUIRED on TransactionData
# - ValidateProvider function exists and rejects mock/empty
# - NO mock transaction data in production paths
# - ProviderTrueLayer is the only valid provider
# - Mock rejection returns "rejected_mock_provider" status
#
# Usage:
#   ./commerce_real_finance_only_enforced.sh --check      # Check for violations (default)
#   ./commerce_real_finance_only_enforced.sh --self-test  # Run self-test to verify detection
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found or self-test failed
#
# Reference: docs/ADR/ADR-0065-phase31-3-real-finance-only.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCANNED DIRECTORIES
# ============================================================================
SCAN_DIRS=(
    "internal/financetxscan"
    "cmd/quantumlife-web/main.go"
)

# ============================================================================
# REQUIRED PATTERNS (must exist)
# ============================================================================

# These patterns MUST exist in financetxscan
REQUIRED_PATTERNS=(
    'ProviderKind'
    'ProviderTrueLayer'
    'ValidateProvider'
    'IsValidProvider'
    'rejected_mock_provider'
)

# ============================================================================
# FORBIDDEN PATTERNS (must NOT exist in production paths)
# ============================================================================

# Mock data patterns - NO mock data in production
FORBIDDEN_MOCK_PATTERNS=(
    'mockTransactions'
    'mock_transactions'
    'MockTransaction'
    'MOCK_PROVIDER'
    '"mock".*ProviderKind'
)

# ============================================================================
# ALLOWLIST
# ============================================================================
# These files are ALLOWED certain patterns with justification.
ALLOWLIST=(
    # Test files
    "_test.go"
    # Demo files
    "internal/demo_"
)

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo "=========================================="
    echo "Phase 31.3: Real Finance Only Guardrails"
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

check_required_patterns() {
    local violations=0

    echo "Checking for required patterns in financetxscan..."

    local model_file="$REPO_ROOT/internal/financetxscan/model.go"
    local engine_file="$REPO_ROOT/internal/financetxscan/engine.go"

    for pattern in "${REQUIRED_PATTERNS[@]}"; do
        local found=0

        if [[ -f "$model_file" ]] && grep -q "$pattern" "$model_file" 2>/dev/null; then
            found=1
        fi

        if [[ -f "$engine_file" ]] && grep -q "$pattern" "$engine_file" 2>/dev/null; then
            found=1
        fi

        if [[ $found -eq 0 ]]; then
            echo "[REQUIRED] Missing pattern: $pattern"
            violations=$((violations + 1))
        fi
    done

    return $violations
}

check_provider_field() {
    local violations=0

    echo "Checking Provider field in TransactionData..."

    local engine_file="$REPO_ROOT/internal/financetxscan/engine.go"
    local model_file="$REPO_ROOT/internal/financetxscan/model.go"

    # Check TransactionData has Provider field
    if [[ -f "$engine_file" ]]; then
        if grep -q "TransactionData struct" "$engine_file" 2>/dev/null; then
            if ! grep -A20 "TransactionData struct" "$engine_file" | grep -q "Provider.*ProviderKind" 2>/dev/null; then
                echo "[PROVIDER_FIELD] TransactionData must have Provider field in engine.go"
                violations=$((violations + 1))
            fi
        fi
    fi

    # Check TransactionInput has Provider field
    if [[ -f "$model_file" ]]; then
        if grep -q "TransactionInput struct" "$model_file" 2>/dev/null; then
            if ! grep -A20 "TransactionInput struct" "$model_file" | grep -q "Provider.*ProviderKind" 2>/dev/null; then
                echo "[PROVIDER_FIELD] TransactionInput must have Provider field in model.go"
                violations=$((violations + 1))
            fi
        fi
    fi

    return $violations
}

check_mock_rejection() {
    local violations=0

    echo "Checking mock rejection logic..."

    local engine_file="$REPO_ROOT/internal/financetxscan/engine.go"
    local model_file="$REPO_ROOT/internal/financetxscan/model.go"

    # Check ValidateProvider exists and rejects mock
    if [[ -f "$model_file" ]]; then
        if ! grep -q "ProviderMock.*rejected" "$model_file" 2>/dev/null; then
            if ! grep -q 'mock provider rejected' "$model_file" 2>/dev/null; then
                echo "[MOCK_REJECTION] ValidateProvider must reject mock provider"
                violations=$((violations + 1))
            fi
        fi
    fi

    # Check BuildFromTransactions validates provider
    if [[ -f "$engine_file" ]]; then
        if grep -q "BuildFromTransactions" "$engine_file" 2>/dev/null; then
            if ! grep -A30 "BuildFromTransactions" "$engine_file" | grep -q "ValidateProvider" 2>/dev/null; then
                echo "[MOCK_REJECTION] BuildFromTransactions must call ValidateProvider"
                violations=$((violations + 1))
            fi
        fi
    fi

    return $violations
}

check_forbidden_mock_patterns() {
    local violations=0

    echo "Checking for forbidden mock patterns in production paths..."

    for scan_dir in "${SCAN_DIRS[@]}"; do
        local full_path="$REPO_ROOT/$scan_dir"
        if [[ ! -e "$full_path" ]]; then
            continue
        fi

        # Get files
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

            for pattern in "${FORBIDDEN_MOCK_PATTERNS[@]}"; do
                # Check for pattern (case-sensitive), excluding comments
                if grep -v "^[[:space:]]*//" "$file" | grep -qE "$pattern" 2>/dev/null; then
                    echo "[MOCK_DATA] Violation in: ${file#$REPO_ROOT/}"
                    grep -nE "$pattern" "$file" 2>/dev/null | head -3
                    echo
                    violations=$((violations + 1))
                fi
            done
        done
    done

    return $violations
}

check_events_defined() {
    local violations=0

    echo "Checking Phase 31.3 events defined..."

    local events_file="$REPO_ROOT/pkg/events/events.go"

    if [[ -f "$events_file" ]]; then
        if ! grep -q "Phase31_3RealFinance" "$events_file" 2>/dev/null; then
            echo "[EVENTS] Phase 31.3 events not defined in events.go"
            violations=$((violations + 1))
        fi
    else
        echo "[EVENTS] events.go not found"
        violations=$((violations + 1))
    fi

    return $violations
}

check_main_uses_real_only() {
    local violations=0

    echo "Checking main.go uses real-only finance path..."

    local main_file="$REPO_ROOT/cmd/quantumlife-web/main.go"

    if [[ -f "$main_file" ]]; then
        # Check that handleTrueLayerSync doesn't contain mock transactions
        if grep -A100 "handleTrueLayerSync" "$main_file" | grep -q "mockTransactions\|ExtractTransactionData.*\"tx-" 2>/dev/null; then
            echo "[REAL_ONLY] main.go still contains mock transactions"
            violations=$((violations + 1))
        fi

        # Check that Phase 31.3 event is emitted
        if ! grep -q "Phase31_3RealFinance" "$main_file" 2>/dev/null; then
            echo "[REAL_ONLY] main.go should emit Phase 31.3 event"
            violations=$((violations + 1))
        fi
    fi

    return $violations
}

run_checks() {
    print_header
    local total_violations=0

    # Check required patterns
    if ! check_required_patterns; then
        total_violations=$((total_violations + 1))
    fi
    echo

    # Check Provider field
    if ! check_provider_field; then
        total_violations=$((total_violations + 1))
    fi
    echo

    # Check mock rejection
    if ! check_mock_rejection; then
        total_violations=$((total_violations + 1))
    fi
    echo

    # Check forbidden mock patterns
    if ! check_forbidden_mock_patterns; then
        total_violations=$((total_violations + 1))
    fi
    echo

    # Check events defined
    if ! check_events_defined; then
        total_violations=$((total_violations + 1))
    fi
    echo

    # Check main uses real-only
    if ! check_main_uses_real_only; then
        total_violations=$((total_violations + 1))
    fi
    echo

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

    # Create test file with required patterns
    cat > "$tmpdir/test_required.go" << 'EOF'
package test

type ProviderKind string

const ProviderTrueLayer ProviderKind = "truelayer"

func ValidateProvider(p ProviderKind) error { return nil }
func IsValidProvider(p ProviderKind) bool { return true }

func test() {
    result := "rejected_mock_provider"
    _ = result
}
EOF

    # Check that required patterns are detected
    local found=0
    for pattern in "${REQUIRED_PATTERNS[@]}"; do
        if grep -q "$pattern" "$tmpdir/test_required.go"; then
            found=$((found + 1))
        fi
    done

    if [[ $found -ge ${#REQUIRED_PATTERNS[@]} ]]; then
        echo "Self-test PASSED: All patterns detected"
        return 0
    else
        echo "Self-test FAILED: Only $found/${#REQUIRED_PATTERNS[@]} patterns detected"
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
