#!/bin/bash
# v9.10 Free-Text Recipient Elimination Guardrail
#
# CRITICAL: This guardrail enforces that:
# 1) No free-text recipient fields exist in write execution paths
# 2) All execution code uses PayeeID instead of Recipient
# 3) No runtime-supplied payment destinations allowed
#
# Reference:
# - docs/ADR/ADR-0013-payee-registry-lock.md
# - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
#
# Usage:
#   ./forbidden_free_text_recipient.sh --check      Run CI check
#   ./forbidden_free_text_recipient.sh --self-test  Run self-tests
#   ./forbidden_free_text_recipient.sh --help       Show help

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Forbidden patterns in write execution paths
FORBIDDEN_PATTERNS=(
    "Recipient string"
    "recipient string"
    "DestinationName"
    "PayeeName string"
)

# Directories to scan for write execution code
SCAN_DIRS=(
    "$REPO_ROOT/internal/finance/execution"
    "$REPO_ROOT/internal/connectors/finance/write"
)

# Allowlisted paths (documentation, tests with read-only context)
ALLOWLIST_PATTERNS=(
    "docs/"
    "_test.go"
    "registry.go"
    "payees/"
)

show_help() {
    echo "v9.10 Free-Text Recipient Elimination Guardrail"
    echo ""
    echo "Usage:"
    echo "  $0 --check      Run CI check (default)"
    echo "  $0 --self-test  Run internal self-tests"
    echo "  $0 --help       Show this help"
    echo ""
    echo "This guardrail enforces that:"
    echo "  1) No free-text recipient fields in write execution paths"
    echo "  2) All execution code uses PayeeID instead of Recipient"
    echo "  3) No runtime-supplied payment destinations"
}

# Check if a path should be allowlisted
is_allowlisted() {
    local path="$1"
    for pattern in "${ALLOWLIST_PATTERNS[@]}"; do
        if [[ "$path" == *"$pattern"* ]]; then
            return 0
        fi
    done
    return 1
}

# Check for forbidden patterns in a file
check_file() {
    local file="$1"
    local violations=0

    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        if grep -q "$pattern" "$file" 2>/dev/null; then
            # Check if it's in a struct definition (write execution context)
            if grep -n "$pattern" "$file" 2>/dev/null | grep -v "^[[:space:]]*//"; then
                echo -e "${RED}VIOLATION: Found '$pattern' in $file${NC}"
                grep -n "$pattern" "$file" 2>/dev/null | head -3
                violations=$((violations + 1))
            fi
        fi
    done

    return $violations
}

# Check that execution types use PayeeID
check_execution_types() {
    local violations=0

    echo "Checking execution types for PayeeID usage..."

    # Check that ExecutionIntent has PayeeID, not Recipient
    local types_file="$REPO_ROOT/internal/finance/execution/types.go"
    if [[ -f "$types_file" ]]; then
        if grep -q "Recipient string" "$types_file" 2>/dev/null; then
            echo -e "${RED}VIOLATION: ExecutionIntent still has 'Recipient string' field${NC}"
            violations=$((violations + 1))
        fi

        if ! grep -q "PayeeID string" "$types_file" 2>/dev/null; then
            echo -e "${RED}VIOLATION: ExecutionIntent missing 'PayeeID string' field${NC}"
            violations=$((violations + 1))
        fi
    fi

    # Check that ActionSpec has PayeeID, not Recipient
    if [[ -f "$types_file" ]]; then
        # Check ActionSpec struct
        if grep -A10 "type ActionSpec struct" "$types_file" 2>/dev/null | grep -q "Recipient string"; then
            echo -e "${RED}VIOLATION: ActionSpec still has 'Recipient string' field${NC}"
            violations=$((violations + 1))
        fi
    fi

    return $violations
}

# Check that write interface uses PayeeID
check_write_interface() {
    local violations=0

    echo "Checking write interface for PayeeID usage..."

    local interface_file="$REPO_ROOT/internal/connectors/finance/write/interface.go"
    if [[ -f "$interface_file" ]]; then
        # Check ActionSpec in interface
        if grep -A10 "type ActionSpec struct" "$interface_file" 2>/dev/null | grep -q "Recipient string"; then
            echo -e "${RED}VIOLATION: write.ActionSpec still has 'Recipient string' field${NC}"
            violations=$((violations + 1))
        fi
    fi

    return $violations
}

# Check that payee registry exists
check_payee_registry() {
    local violations=0

    echo "Checking payee registry exists..."

    local registry_file="$REPO_ROOT/internal/connectors/finance/write/payees/registry.go"
    if [[ ! -f "$registry_file" ]]; then
        echo -e "${RED}VIOLATION: Payee registry not found at $registry_file${NC}"
        violations=$((violations + 1))
    else
        echo -e "${GREEN}Payee registry exists: $registry_file${NC}"
    fi

    return $violations
}

# Check executor uses payee registry
check_executor_payee_usage() {
    local violations=0

    echo "Checking executor payee registry usage..."

    local executor_file="$REPO_ROOT/internal/finance/execution/executor_v96.go"
    if [[ -f "$executor_file" ]]; then
        # Check for payee registry import
        if ! grep -q 'connectors/finance/write/payees' "$executor_file" 2>/dev/null; then
            echo -e "${RED}VIOLATION: executor_v96.go does not import payees registry${NC}"
            violations=$((violations + 1))
        fi

        # Check for RequireAllowed usage with payee
        if ! grep -q 'payeeRegistry.RequireAllowed' "$executor_file" 2>/dev/null; then
            echo -e "${RED}VIOLATION: executor_v96.go does not use payeeRegistry.RequireAllowed${NC}"
            violations=$((violations + 1))
        fi
    fi

    return $violations
}

# Run CI check
run_check() {
    echo "========================================"
    echo "v9.10 Free-Text Recipient Elimination"
    echo "========================================"
    echo ""
    echo "Reference: Canon Addendum v9 - Payee Registry Lock"
    echo ""

    local total_violations=0

    # Check payee registry exists
    check_payee_registry
    total_violations=$((total_violations + $?))

    echo ""

    # Check execution types
    check_execution_types
    total_violations=$((total_violations + $?))

    echo ""

    # Check write interface
    check_write_interface
    total_violations=$((total_violations + $?))

    echo ""

    # Check executor usage
    check_executor_payee_usage
    total_violations=$((total_violations + $?))

    echo ""

    # Scan for forbidden patterns in write paths
    echo "Scanning for forbidden free-text recipient patterns..."
    for dir in "${SCAN_DIRS[@]}"; do
        if [[ ! -d "$dir" ]]; then
            continue
        fi

        while IFS= read -r -d '' file; do
            # Skip allowlisted files
            if is_allowlisted "$file"; then
                continue
            fi

            check_file "$file"
            total_violations=$((total_violations + $?))
        done < <(find "$dir" -name "*.go" -type f -print0 2>/dev/null)
    done

    echo ""
    echo "========================================"

    if [[ $total_violations -gt 0 ]]; then
        echo -e "${RED}FAILED: $total_violations violation(s) found${NC}"
        exit 1
    fi

    echo -e "${GREEN}PASSED: No free-text recipient violations found${NC}"
    exit 0
}

# Run self-tests
run_self_test() {
    echo "========================================"
    echo "v9.10 Guardrail Self-Tests"
    echo "========================================"
    echo ""

    local tests_passed=0
    local tests_failed=0

    # Test 1: is_allowlisted function
    echo "Test 1: is_allowlisted function"
    if is_allowlisted "docs/README.md"; then
        echo -e "  ${GREEN}PASS: docs/ path is allowlisted${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: docs/ path should be allowlisted${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    if is_allowlisted "internal/foo_test.go"; then
        echo -e "  ${GREEN}PASS: _test.go is allowlisted${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: _test.go should be allowlisted${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    if ! is_allowlisted "internal/finance/execution/types.go"; then
        echo -e "  ${GREEN}PASS: types.go is not allowlisted${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: types.go should not be allowlisted${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    echo ""

    # Test 2: Payee registry exists
    echo "Test 2: Payee registry file exists"
    if [[ -f "$REPO_ROOT/internal/connectors/finance/write/payees/registry.go" ]]; then
        echo -e "  ${GREEN}PASS: Payee registry found${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: Payee registry not found${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    echo ""

    # Test 3: ExecutionIntent uses PayeeID
    echo "Test 3: ExecutionIntent uses PayeeID"
    if grep -q "PayeeID string" "$REPO_ROOT/internal/finance/execution/types.go" 2>/dev/null; then
        echo -e "  ${GREEN}PASS: ExecutionIntent has PayeeID field${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: ExecutionIntent missing PayeeID field${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    echo ""
    echo "========================================"
    echo "Self-test results: $tests_passed passed, $tests_failed failed"

    if [[ $tests_failed -gt 0 ]]; then
        exit 1
    fi
    exit 0
}

# Main
case "${1:-}" in
    --help|-h)
        show_help
        exit 0
        ;;
    --self-test)
        run_self_test
        ;;
    --check|"")
        run_check
        ;;
    *)
        echo "Unknown option: $1"
        echo "Use --help for usage"
        exit 2
        ;;
esac
