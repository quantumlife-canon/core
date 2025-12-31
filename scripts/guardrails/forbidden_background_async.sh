#!/bin/bash
# v9.7 No Background Execution Guardrail
#
# Enforces the Canon Addendum v9 invariant: NO BACKGROUND EXECUTION
# in core runtime packages. All execution must be synchronous, explicit,
# and auditable.
#
# Usage:
#   ./forbidden_background_async.sh --check      # Check for violations (default)
#   ./forbidden_background_async.sh --self-test  # Run self-test
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found
#   2 = Script usage error
#
# Reference:
#   - docs/QUANTUMLIFE_CANON_V1.md
#   - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
#   - docs/ADR/ADR-0010-no-background-execution-guardrail.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCOPE DEFINITION
# ============================================================================
# Core packages that MUST NOT contain background/async patterns.
SCANNED_DIRS=(
    "internal/authority"
    "internal/action"
    "internal/execution"
    "internal/finance"
    "internal/intersection"
    "internal/negotiation"
    "internal/revocation"
    "internal/memory"
    "internal/audit"
    "internal/approval"
    "internal/connectors"
)

# Directories/patterns to EXCLUDE from scanning.
# These are not core runtime packages.
EXCLUDED_PATTERNS=(
    "cmd/"
    "internal/demo"
    "_test.go"
    "pkg/clock/"
    "scripts/"
    "docs/"
)

# ============================================================================
# FORBIDDEN PATTERNS
# ============================================================================
# Pattern A: Goroutines in core paths
GOROUTINE_PATTERNS=(
    'go[[:space:]]+func[[:space:]]*\('
    'go[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*\('
)

# Pattern B: Timers / tickers / delayed callbacks
TIMER_PATTERNS=(
    'time\.NewTicker[[:space:]]*\('
    'time\.NewTimer[[:space:]]*\('
    'time\.AfterFunc[[:space:]]*\('
    'time\.After[[:space:]]*\('
)

# Pattern D: Async/Background function names
ASYNC_FUNC_PATTERNS=(
    'func[[:space:]]+[a-zA-Z_]*Async[[:space:]]*\('
    'func[[:space:]]+[a-zA-Z_]*Background[[:space:]]*\('
    'func[[:space:]]+\([^)]*\)[[:space:]]+[a-zA-Z_]*Async[[:space:]]*\('
    'func[[:space:]]+\([^)]*\)[[:space:]]+[a-zA-Z_]*Background[[:space:]]*\('
)

# ============================================================================
# ALLOWLIST
# ============================================================================
# Files that may use specific patterns under controlled conditions.
# Format: "pattern:filepath_glob:justification"

is_allowlisted() {
    local file="$1"
    local pattern_type="$2"
    local relative_path="${file#$REPO_ROOT/}"

    if [[ "$pattern_type" == "timer" ]]; then
        # Allowlist 1: Connector client.go files may use time.After/time.NewTimer
        # for HTTP request timeouts (not background execution)
        if [[ "$relative_path" == internal/connectors/*/client.go ]] || \
           [[ "$relative_path" == internal/connectors/*/*/client.go ]] || \
           [[ "$relative_path" == internal/connectors/*/*/*/client.go ]]; then
            # Check if this file contains http.Client or context.WithTimeout
            # (indicates legitimate timeout usage, not background execution)
            if grep -qE '(http\.Client|context\.WithTimeout|context\.WithDeadline)' "$file" 2>/dev/null; then
                return 0  # Allowed
            fi
        fi

        # Allowlist 2: Finance execution forced pause implementation
        # v9.3 requires a mandatory synchronous pause before external writes.
        # time.After in a select is used for cancellation support while blocking.
        # This is NOT background execution - it's synchronous blocking with
        # context cancellation support.
        # Reference: TECHNICAL_SPLIT_V9_EXECUTION.md §Forced Pause
        if [[ "$relative_path" == internal/finance/execution/executor_v9*.go ]] || \
           [[ "$relative_path" == internal/connectors/finance/write/providers/*/connector.go ]]; then
            # Check if this is forced pause pattern (ForcedPauseDuration in same file)
            if grep -qE 'ForcedPauseDuration' "$file" 2>/dev/null; then
                return 0  # Allowed - forced pause implementation
            fi
        fi
    fi

    return 1  # Not allowlisted
}

is_excluded() {
    local file="$1"
    local relative_path="${file#$REPO_ROOT/}"

    for pattern in "${EXCLUDED_PATTERNS[@]}"; do
        if [[ "$relative_path" == *"$pattern"* ]]; then
            return 0  # Excluded
        fi
    done

    return 1  # Not excluded
}

# ============================================================================
# CHECK FUNCTIONS
# ============================================================================

print_header() {
    echo "========================================"
    echo "v9.7 No Background Execution Guardrail"
    echo "========================================"
}

check_file() {
    local file="$1"
    local violations=0

    # Skip excluded files
    if is_excluded "$file"; then
        return 0
    fi

    local relative_path="${file#$REPO_ROOT/}"

    # Check goroutine patterns
    for pattern in "${GOROUTINE_PATTERNS[@]}"; do
        while IFS=: read -r line_num line_content; do
            if [[ -n "$line_num" ]]; then
                echo ""
                echo "VIOLATION: Goroutine in core package"
                echo "  File:    $relative_path"
                echo "  Line:    $line_num"
                echo "  Content: $line_content"
                echo "  Pattern: $pattern"
                ((violations++)) || true
            fi
        done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)
    done

    # Check timer patterns (with allowlist)
    for pattern in "${TIMER_PATTERNS[@]}"; do
        while IFS=: read -r line_num line_content; do
            if [[ -n "$line_num" ]]; then
                if ! is_allowlisted "$file" "timer"; then
                    echo ""
                    echo "VIOLATION: Timer/Ticker in core package"
                    echo "  File:    $relative_path"
                    echo "  Line:    $line_num"
                    echo "  Content: $line_content"
                    echo "  Pattern: $pattern"
                    ((violations++)) || true
                fi
            fi
        done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)
    done

    # Check async function name patterns
    for pattern in "${ASYNC_FUNC_PATTERNS[@]}"; do
        while IFS=: read -r line_num line_content; do
            if [[ -n "$line_num" ]]; then
                echo ""
                echo "VIOLATION: Async/Background function name"
                echo "  File:    $relative_path"
                echo "  Line:    $line_num"
                echo "  Content: $line_content"
                echo "  Pattern: $pattern"
                ((violations++)) || true
            fi
        done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)
    done

    return $violations
}

check_violations() {
    local root_dir="${1:-$REPO_ROOT}"
    local total_violations=0
    local files_checked=0

    print_header
    echo ""
    echo "Reference: Canon Addendum v9 - No Background Execution"
    echo ""
    echo "Scanning core packages for forbidden async patterns..."
    echo ""

    for dir in "${SCANNED_DIRS[@]}"; do
        local search_path="$root_dir/$dir"

        # Skip if directory doesn't exist
        if [[ ! -d "$search_path" ]]; then
            continue
        fi

        # Find all .go files (excluding test files)
        while IFS= read -r -d '' file; do
            local violations_in_file=0
            check_file "$file" || violations_in_file=$?
            ((total_violations += violations_in_file)) || true
            ((files_checked++)) || true
        done < <(find "$search_path" -name "*.go" ! -name "*_test.go" -print0 2>/dev/null)
    done

    echo ""
    echo "========================================"
    echo "Files checked: $files_checked"

    if [[ $total_violations -gt 0 ]]; then
        echo ""
        echo -e "\033[0;31mFAILED: Found $total_violations violation(s)\033[0m"
        echo ""
        echo "Background execution is FORBIDDEN in core runtime packages."
        echo "All execution must be synchronous and explicit."
        echo ""
        echo "To fix:"
        echo "  1. Remove goroutines - use synchronous calls"
        echo "  2. Remove timers/tickers - use explicit time parameters"
        echo "  3. Rename Async/Background functions - execution is always sync"
        echo ""
        echo "Reference: docs/ADR/ADR-0010-no-background-execution-guardrail.md"
        return 1
    else
        echo ""
        echo -e "\033[0;32mPASSED: No background execution violations found\033[0m"
        return 0
    fi
}

# ============================================================================
# SELF-TEST
# ============================================================================

run_self_test() {
    print_header
    echo ""
    echo "Running self-test..."
    echo ""

    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    # Disable errexit for self-test (we check return values explicitly)
    set +e

    local tests_passed=0
    local tests_failed=0

    # -------------------------------------------------------------------------
    # Test 1: Detect goroutine in core package
    # -------------------------------------------------------------------------
    echo "Test 1: Detecting goroutine in core package..."
    mkdir -p "$temp_dir/internal/finance"
    cat > "$temp_dir/internal/finance/bad_goroutine.go" << 'GOFILE'
package finance

func badFunc() {
    go func() {
        // This is forbidden
    }()
}
GOFILE

    if grep -qE 'go[[:space:]]+func[[:space:]]*\(' "$temp_dir/internal/finance/bad_goroutine.go"; then
        echo "  ✓ Goroutine pattern detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect goroutine"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 2: Detect timer in core package
    # -------------------------------------------------------------------------
    echo "Test 2: Detecting timer in core package..."
    mkdir -p "$temp_dir/internal/execution"
    cat > "$temp_dir/internal/execution/bad_timer.go" << 'GOFILE'
package execution

import "time"

func badFunc() {
    timer := time.NewTimer(5 * time.Second)
    <-timer.C
}
GOFILE

    if grep -qE 'time\.NewTimer[[:space:]]*\(' "$temp_dir/internal/execution/bad_timer.go"; then
        echo "  ✓ Timer pattern detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect timer"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 3: Detect Async function name
    # -------------------------------------------------------------------------
    echo "Test 3: Detecting Async function name..."
    mkdir -p "$temp_dir/internal/action"
    cat > "$temp_dir/internal/action/bad_async.go" << 'GOFILE'
package action

func ExecuteAsync() {
    // Async function names are forbidden
}
GOFILE

    if grep -qE 'func[[:space:]]+[a-zA-Z_]*Async[[:space:]]*\(' "$temp_dir/internal/action/bad_async.go"; then
        echo "  ✓ Async function name detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect Async function name"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 4: Allow test files (excluded)
    # -------------------------------------------------------------------------
    echo "Test 4: Allowing test files..."
    cat > "$temp_dir/internal/finance/something_test.go" << 'GOFILE'
package finance

func TestWithGoroutine() {
    go func() {
        // Allowed in test files
    }()
}
GOFILE

    local test_file="$temp_dir/internal/finance/something_test.go"
    if is_excluded "$test_file"; then
        echo "  ✓ Test file correctly excluded"
        ((tests_passed++))
    else
        echo "  ✗ Test file should be excluded"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 5: Allow demo packages (excluded)
    # -------------------------------------------------------------------------
    echo "Test 5: Allowing demo packages..."
    mkdir -p "$temp_dir/internal/demo_test"
    cat > "$temp_dir/internal/demo_test/runner.go" << 'GOFILE'
package demo_test

func RunDemo() {
    go func() {
        // Allowed in demo packages
    }()
}
GOFILE

    local demo_file="$temp_dir/internal/demo_test/runner.go"
    if is_excluded "$demo_file"; then
        echo "  ✓ Demo package correctly excluded"
        ((tests_passed++))
    else
        echo "  ✗ Demo package should be excluded"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 6: Allow connector client.go with HTTP timeout
    # -------------------------------------------------------------------------
    echo "Test 6: Allowing connector client.go with HTTP timeout..."
    mkdir -p "$temp_dir/internal/connectors/finance/read/providers/test"
    cat > "$temp_dir/internal/connectors/finance/read/providers/test/client.go" << 'GOFILE'
package test

import (
    "context"
    "net/http"
    "time"
)

type Client struct {
    httpClient *http.Client
}

func (c *Client) doRequest(ctx context.Context) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // This timer usage is allowed for HTTP timeouts
    select {
    case <-time.After(5 * time.Second):
        return
    case <-ctx.Done():
        return
    }
}
GOFILE

    # Manually set REPO_ROOT for this test
    local old_repo_root="$REPO_ROOT"
    REPO_ROOT="$temp_dir"

    local connector_file="$temp_dir/internal/connectors/finance/read/providers/test/client.go"
    if is_allowlisted "$connector_file" "timer"; then
        echo "  ✓ Connector client.go with HTTP timeout correctly allowlisted"
        ((tests_passed++))
    else
        echo "  ✗ Connector client.go with HTTP timeout should be allowlisted"
        ((tests_failed++))
    fi

    REPO_ROOT="$old_repo_root"

    # -------------------------------------------------------------------------
    # Test 7: Full check should find violations
    # -------------------------------------------------------------------------
    echo "Test 7: Full check finds violations in temp tree..."

    # Run check_violations against temp dir (should fail due to bad files)
    local old_repo_root="$REPO_ROOT"
    REPO_ROOT="$temp_dir"

    if check_violations "$temp_dir" > /dev/null 2>&1; then
        echo "  ✗ Check should have found violations"
        ((tests_failed++))
    else
        echo "  ✓ Check correctly found violations"
        ((tests_passed++))
    fi

    REPO_ROOT="$old_repo_root"

    # -------------------------------------------------------------------------
    # Summary
    # -------------------------------------------------------------------------
    echo ""
    echo "========================================"
    echo "Self-test results: $tests_passed passed, $tests_failed failed"

    if [[ $tests_failed -gt 0 ]]; then
        echo -e "\033[0;31mSELF-TEST FAILED\033[0m"
        return 1
    else
        echo -e "\033[0;32mSELF-TEST PASSED\033[0m"
        return 0
    fi
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
            echo "  --check      Check for background execution violations (default)"
            echo "  --self-test  Run self-test to verify detection"
            echo ""
            echo "Exit codes:"
            echo "  0 = No violations"
            echo "  1 = Violations found"
            echo "  2 = Usage error"
            ;;
        *)
            echo "Unknown option: $mode" >&2
            echo "Use --help for usage" >&2
            exit 2
            ;;
    esac
}

main "$@"
