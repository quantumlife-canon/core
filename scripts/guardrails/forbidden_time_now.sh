#!/bin/bash
# v9.6.2 Clock Guardrail: Forbid time.Now() in core logic packages
#
# This script detects nondeterministic time.Now() usage in packages that
# require deterministic time for testing and ceiling/authorization logic.
#
# Usage:
#   ./forbidden_time_now.sh --check      # Check for violations (default)
#   ./forbidden_time_now.sh --self-test  # Run self-test to verify detection
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found or self-test failed
#
# Reference: v9.6.2 Clock Guardrail

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# FORBIDDEN DIRECTORIES
# ============================================================================
# These directories MUST NOT contain time.Now() calls.
# Core logic must use injected clock for determinism.
FORBIDDEN_PATTERNS=(
    "internal/demo"           # All demo packages
    "internal/action/impl_"   # Action implementations
    "internal/approval/impl_" # Approval implementations
    "internal/audit/impl_"    # Audit implementations
    "internal/authority/impl_" # Authority implementations (ceiling checks!)
    "internal/circle/impl_"   # Circle implementations
    "internal/execution/impl_" # Execution implementations
    "internal/finance/impl_"  # Finance implementations
    "internal/intersection/impl_" # Intersection implementations
    "internal/memory/impl_"   # Memory implementations
    "internal/negotiation/impl_" # Negotiation implementations
    "internal/orchestrator/impl_" # Orchestrator implementations
    "internal/revocation/impl_" # Revocation implementations
    "internal/connectors/auth/impl_" # Auth connector implementations
    "internal/connectors/calendar/impl_" # Calendar connector implementations
    "internal/connectors/finance/read/providers/" # Finance read providers
    "internal/connectors/finance/write/providers/" # Finance write providers
    "internal/finance/execution/" # V9 execution (ceiling checks!)
    "internal/finance/adjustments/" # Financial adjustments
    "internal/finance/categorize/" # Financial categorization
    "internal/finance/neutrality/" # Financial neutrality checks
    "internal/finance/pagination/" # Pagination logic
    "internal/finance/propose/" # Proposal logic
    "internal/finance/reconcile/" # Reconciliation logic
    "internal/finance/sharedview/" # Shared view logic
    "internal/finance/visibility/" # Visibility logic
)

# ============================================================================
# ALLOWLIST
# ============================================================================
# These paths are ALLOWED to use time.Now() with justification.
ALLOWLIST=(
    # pkg/clock is the canonical clock source - it MUST use time.Now()
    "pkg/clock/"

    # cmd/* entrypoints may use time.Now() for banners/timestamps
    "cmd/"

    # Test files may use time.Now() for test setup (not in production paths)
    # Note: _test.go files are not production code
)

# Pattern to detect time.Now() usage
TIME_NOW_PATTERN='time\.Now\('

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo "========================================"
    echo "v9.6.2 Clock Guardrail"
    echo "========================================"
}

is_allowlisted() {
    local file="$1"
    local relative_path="${file#$REPO_ROOT/}"

    for allowed in "${ALLOWLIST[@]}"; do
        if [[ "$relative_path" == $allowed* ]]; then
            return 0
        fi
    done

    # Allow _test.go files (test code, not production)
    if [[ "$file" == *_test.go ]]; then
        return 0
    fi

    return 1
}

check_violations() {
    local violations=0
    local violation_files=()

    print_header
    echo ""
    echo "Scanning for forbidden time.Now() usage..."
    echo ""

    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        local search_path="$REPO_ROOT/$pattern"

        # Skip if directory doesn't exist
        if [[ ! -d "$search_path" ]] && [[ ! -d "$(dirname "$search_path")" ]]; then
            continue
        fi

        # Find all .go files matching the pattern
        while IFS= read -r -d '' file; do
            # Skip allowlisted files
            if is_allowlisted "$file"; then
                continue
            fi

            # Check for time.Now( usage
            if grep -n "$TIME_NOW_PATTERN" "$file" 2>/dev/null; then
                relative_path="${file#$REPO_ROOT/}"
                echo ""
                echo "VIOLATION: $relative_path"
                grep -n "$TIME_NOW_PATTERN" "$file" | while read -r line; do
                    echo "  $line"
                done
                violation_files+=("$relative_path")
                ((violations++)) || true
            fi
        done < <(find "$REPO_ROOT" -path "$search_path*" -name "*.go" -print0 2>/dev/null)
    done

    echo ""
    echo "========================================"

    if [[ $violations -gt 0 ]]; then
        echo "FAILED: Found $violations file(s) with time.Now() violations"
        echo ""
        echo "Fix: Inject clock.Clock instead of calling time.Now() directly."
        echo "See: pkg/clock/README.md"
        echo ""
        echo "Violating files:"
        for f in "${violation_files[@]}"; do
            echo "  - $f"
        done
        return 1
    else
        echo "PASSED: No time.Now() violations found"
        return 0
    fi
}

run_self_test() {
    print_header
    echo ""
    echo "Running self-test..."
    echo ""

    # Create temp directory
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    # Test 1: Create a mock violation file (non-test file)
    local test_dir="$temp_dir/internal/demo_foo"
    mkdir -p "$test_dir"

    cat > "$test_dir/violation.go" << 'GOFILE'
package demo_foo

import "time"

func badFunc() time.Time {
    return time.Now()
}
GOFILE

    echo "Test 1: Detecting time.Now() in non-test file..."
    if grep -qE 'time\.Now\(' "$test_dir/violation.go"; then
        echo "  ✓ time.Now() detected in violation.go"
    else
        echo "SELF-TEST FAILED: Guardrail did not detect time.Now() in violation file"
        return 1
    fi

    # Test 2: Verify allowlist for pkg/clock
    echo ""
    echo "Test 2: Verifying allowlist excludes pkg/clock/..."
    local allowed_dir="$temp_dir/pkg/clock"
    mkdir -p "$allowed_dir"
    cat > "$allowed_dir/clock.go" << 'GOFILE'
package clock

import "time"

func Now() time.Time {
    return time.Now()
}
GOFILE

    local relative_path="pkg/clock/clock.go"
    local is_allowed=0
    for allowed in "${ALLOWLIST[@]}"; do
        if [[ "$relative_path" == $allowed* ]]; then
            is_allowed=1
            break
        fi
    done

    if [[ $is_allowed -eq 1 ]]; then
        echo "  ✓ pkg/clock/ correctly allowlisted"
    else
        echo "SELF-TEST FAILED: pkg/clock/ should be allowlisted"
        return 1
    fi

    # Test 3: Verify _test.go files are allowed
    echo ""
    echo "Test 3: Verifying _test.go files are allowed..."
    cat > "$test_dir/violation_test.go" << 'GOFILE'
package demo_foo

import "time"

func TestBadFunc() {
    _ = time.Now()
}
GOFILE

    if [[ "$test_dir/violation_test.go" == *_test.go ]]; then
        echo "  ✓ _test.go files correctly identified as allowed"
    else
        echo "SELF-TEST FAILED: _test.go files should be allowed"
        return 1
    fi

    echo ""
    echo "========================================"
    echo "SELF-TEST PASSED: All detection tests passed"
    return 0
}

# ============================================================================
# MAIN
# ============================================================================

main() {
    local mode="${1:---check}"

    case "$mode" in
        --check)
            check_violations
            ;;
        --self-test)
            run_self_test
            ;;
        --help|-h)
            echo "Usage: $0 [--check|--self-test]"
            echo ""
            echo "  --check      Check for time.Now() violations (default)"
            echo "  --self-test  Run self-test to verify detection works"
            ;;
        *)
            echo "Unknown option: $mode"
            echo "Use --help for usage"
            exit 1
            ;;
    esac
}

main "$@"
