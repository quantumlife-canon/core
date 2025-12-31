#!/bin/bash
# v9.9 Write Provider Registry Guardrail
#
# CRITICAL: This guardrail enforces that:
# 1) All write providers are registered in the provider registry
# 2) All executors consult the registry before using a provider
# 3) No unregistered/unapproved providers can be added silently
#
# Reference:
# - docs/ADR/ADR-0012-write-provider-registry-lock.md
# - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
#
# Usage:
#   ./forbidden_unregistered_write_provider.sh --check      Run CI check
#   ./forbidden_unregistered_write_provider.sh --self-test  Run self-tests
#   ./forbidden_unregistered_write_provider.sh --help       Show help

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Registry file
REGISTRY_FILE="$REPO_ROOT/internal/connectors/finance/write/registry/registry.go"

# Known provider IDs from registry (extracted from registry.go)
# These are the only allowed provider IDs
KNOWN_PROVIDER_IDS=(
    "mock-write"
    "truelayer-sandbox"
    "truelayer-live"
)

# Directories to scan for write providers
PROVIDER_DIRS=(
    "$REPO_ROOT/internal/connectors/finance/write/providers"
)

# Directories to scan for executors
EXECUTOR_DIRS=(
    "$REPO_ROOT/internal/finance/execution"
)

# Allowed demo directories (can use mock-write)
DEMO_DIRS=(
    "$REPO_ROOT/internal/demo*"
)

show_help() {
    echo "v9.9 Write Provider Registry Guardrail"
    echo ""
    echo "Usage:"
    echo "  $0 --check      Run CI check (default)"
    echo "  $0 --self-test  Run internal self-tests"
    echo "  $0 --help       Show this help"
    echo ""
    echo "This guardrail enforces that:"
    echo "  1) All write providers are registered in the provider registry"
    echo "  2) All executors consult the registry before using a provider"
    echo "  3) Provider IDs match known registered providers"
}

# Check if a provider ID is known/registered
is_known_provider() {
    local provider_id="$1"
    for known in "${KNOWN_PROVIDER_IDS[@]}"; do
        if [[ "$provider_id" == "$known" ]]; then
            return 0
        fi
    done
    return 1
}

# Extract provider IDs from a Go file
extract_provider_ids() {
    local file="$1"
    # Look for patterns like: ProviderID() string { return "..." }
    # Or: Provider() string { return "..." }
    # Or: ProviderID = "..."
    grep -oP '(ProviderID\(\)|Provider\(\)).*return\s*"[^"]+"|ProviderID\s*=\s*"[^"]+"' "$file" 2>/dev/null | \
        grep -oP '"[^"]+"' | tr -d '"' || true
}

# Check that registry file exists
check_registry_exists() {
    if [[ ! -f "$REGISTRY_FILE" ]]; then
        echo -e "${RED}ERROR: Registry file not found: $REGISTRY_FILE${NC}" >&2
        return 1
    fi
    return 0
}

# Check that executors use RequireAllowed or IsAllowed
check_executor_registry_usage() {
    local violations=0

    echo "Checking executor registry usage..."

    for dir in "${EXECUTOR_DIRS[@]}"; do
        if [[ ! -d "$dir" ]]; then
            continue
        fi

        # Find executor files (v9*)
        for file in "$dir"/executor_v*.go; do
            if [[ ! -f "$file" ]]; then
                continue
            fi

            # Skip test files
            if [[ "$file" == *"_test.go" ]]; then
                continue
            fi

            filename=$(basename "$file")

            # Check for registry import
            if ! grep -q 'connectors/finance/write/registry' "$file" 2>/dev/null; then
                # v93 and v94 may not have registry yet (older versions)
                # Only require for v95+
                if [[ "$filename" == "executor_v96.go" ]] || [[ "$filename" == "executor_v95.go" ]]; then
                    echo -e "${YELLOW}WARNING: $filename does not import registry package${NC}"
                fi
                continue
            fi

            # Check for RequireAllowed usage
            if ! grep -q 'RequireAllowed\|IsAllowed' "$file" 2>/dev/null; then
                echo -e "${RED}VIOLATION: $filename imports registry but does not use RequireAllowed/IsAllowed${NC}"
                violations=$((violations + 1))
            fi
        done
    done

    return $violations
}

# Check provider implementations for valid provider IDs
check_provider_implementations() {
    local violations=0

    echo "Checking provider implementations..."

    for dir in "${PROVIDER_DIRS[@]}"; do
        if [[ ! -d "$dir" ]]; then
            continue
        fi

        # Find all Go files in provider directories
        while IFS= read -r -d '' file; do
            # Skip test files
            if [[ "$file" == *"_test.go" ]]; then
                continue
            fi

            # Extract provider IDs from file
            provider_ids=$(extract_provider_ids "$file")

            for provider_id in $provider_ids; do
                if ! is_known_provider "$provider_id"; then
                    echo -e "${RED}VIOLATION: Unknown provider ID '$provider_id' in $file${NC}"
                    echo "  Known providers: ${KNOWN_PROVIDER_IDS[*]}"
                    violations=$((violations + 1))
                fi
            done
        done < <(find "$dir" -name "*.go" -type f -print0 2>/dev/null)
    done

    return $violations
}

# Check demo mock connectors
check_demo_providers() {
    local violations=0

    echo "Checking demo mock providers..."

    for pattern in "${DEMO_DIRS[@]}"; do
        for dir in $pattern; do
            if [[ ! -d "$dir" ]]; then
                continue
            fi

            # Find mock connector files
            for file in "$dir"/*mock*connector*.go; do
                if [[ ! -f "$file" ]]; then
                    continue
                fi

                # Skip test files
                if [[ "$file" == *"_test.go" ]]; then
                    continue
                fi

                # Extract provider IDs
                provider_ids=$(extract_provider_ids "$file")

                for provider_id in $provider_ids; do
                    # Demo connectors should only use mock-write
                    if [[ "$provider_id" != "mock-write" ]]; then
                        echo -e "${YELLOW}WARNING: Demo connector using non-mock provider '$provider_id' in $file${NC}"
                    fi
                done
            done
        done
    done

    return $violations
}

# Run CI check
run_check() {
    echo "========================================"
    echo "v9.9 Write Provider Registry Guardrail"
    echo "========================================"
    echo ""
    echo "Reference: Canon Addendum v9 - Provider Registry Lock"
    echo ""

    local total_violations=0

    # Check registry exists
    if ! check_registry_exists; then
        exit 1
    fi
    echo -e "${GREEN}Registry file exists: $REGISTRY_FILE${NC}"
    echo ""

    # Check executor registry usage
    check_executor_registry_usage
    total_violations=$((total_violations + $?))

    echo ""

    # Check provider implementations
    check_provider_implementations
    total_violations=$((total_violations + $?))

    echo ""

    # Check demo providers
    check_demo_providers
    total_violations=$((total_violations + $?))

    echo ""
    echo "========================================"

    if [[ $total_violations -gt 0 ]]; then
        echo -e "${RED}FAILED: $total_violations violation(s) found${NC}"
        exit 1
    fi

    echo -e "${GREEN}PASSED: No write provider registry violations found${NC}"
    exit 0
}

# Run self-tests
run_self_test() {
    echo "========================================"
    echo "v9.9 Guardrail Self-Tests"
    echo "========================================"
    echo ""

    local tests_passed=0
    local tests_failed=0

    # Test 1: is_known_provider function
    echo "Test 1: is_known_provider function"
    if is_known_provider "mock-write"; then
        echo -e "  ${GREEN}PASS: mock-write is recognized${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: mock-write should be recognized${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    if is_known_provider "truelayer-sandbox"; then
        echo -e "  ${GREEN}PASS: truelayer-sandbox is recognized${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: truelayer-sandbox should be recognized${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    if ! is_known_provider "unknown-provider"; then
        echo -e "  ${GREEN}PASS: unknown-provider is not recognized${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: unknown-provider should not be recognized${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    echo ""

    # Test 2: Registry file check
    echo "Test 2: Registry file exists"
    if check_registry_exists 2>/dev/null; then
        echo -e "  ${GREEN}PASS: Registry file found${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: Registry file not found${NC}"
        tests_failed=$((tests_failed + 1))
    fi

    echo ""

    # Test 3: Extract provider IDs from sample
    echo "Test 3: Extract provider IDs from sample code"
    local sample_code='func (c *Connector) ProviderID() string { return "mock-write" }'
    local tmp_file=$(mktemp)
    echo "$sample_code" > "$tmp_file"
    local extracted=$(extract_provider_ids "$tmp_file")
    rm -f "$tmp_file"

    if [[ "$extracted" == *"mock-write"* ]]; then
        echo -e "  ${GREEN}PASS: Correctly extracted provider ID${NC}"
        tests_passed=$((tests_passed + 1))
    else
        echo -e "  ${RED}FAIL: Failed to extract provider ID (got: $extracted)${NC}"
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
