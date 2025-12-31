#!/bin/bash
# v9.8 No Auto-Retry Guardrail
#
# Enforces the Canon Addendum v9 invariant: NO RETRIES in financial execution.
# All failures require new approvals. Retry loops are forbidden.
#
# Usage:
#   ./forbidden_auto_retry.sh --check      # Check for violations (default)
#   ./forbidden_auto_retry.sh --self-test  # Run self-test
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found
#   2 = Script usage error
#
# Reference:
#   - docs/QUANTUMLIFE_CANON_V1.md
#   - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
#   - docs/ADR/ADR-0011-no-auto-retry-and-single-trace-finalization.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCOPE DEFINITION
# ============================================================================
# Core packages that MUST NOT contain retry patterns.
SCANNED_DIRS=(
    "internal/finance/execution"
    "internal/connectors/finance/write"
    "internal/action"
)

# Directories/patterns to EXCLUDE from scanning.
EXCLUDED_PATTERNS=(
    "_test.go"
    "scripts/guardrails/"
    "cmd/"
    "pkg/clock/"
    "internal/demo"
)

# ============================================================================
# FORBIDDEN PATTERNS
# ============================================================================

# Pattern A: Retry-related identifiers
RETRY_IDENTIFIER_PATTERNS=(
    'Retry[A-Z]'
    'Retrier'
    'Backoff[A-Z]'
    'BackoffPolicy'
    'Exponential[A-Z].*Backoff'
    'ExponentialBackoff'
    'Jitter[A-Z]'
    'WithRetry'
    'RetryCount'
    'MaxRetries'
    'RetryDelay'
    'RetryPolicy'
    'RetryStrategy'
    'AutoRetry'
)

# Pattern B: time.Sleep used as backoff (near Execute/Prepare calls)
SLEEP_BACKOFF_PATTERN='time\.Sleep'

# Pattern C: Loop patterns that may indicate retries
# We look for loops containing Execute or Prepare calls

# ============================================================================
# HELPER FUNCTIONS
# ============================================================================

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

print_header() {
    echo "========================================"
    echo "v9.8 No Auto-Retry Guardrail"
    echo "========================================"
}

# Check a file for retry-related identifiers
check_retry_identifiers() {
    local file="$1"
    local violations=0
    local relative_path="${file#$REPO_ROOT/}"

    for pattern in "${RETRY_IDENTIFIER_PATTERNS[@]}"; do
        while IFS=: read -r line_num line_content; do
            if [[ -n "$line_num" ]]; then
                echo ""
                echo "VIOLATION: Retry-related identifier"
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

# Check for time.Sleep near provider calls (potential backoff)
check_sleep_backoff() {
    local file="$1"
    local violations=0
    local relative_path="${file#$REPO_ROOT/}"

    # Check if file contains both time.Sleep and Execute/Prepare
    if grep -qE 'time\.Sleep' "$file" 2>/dev/null; then
        if grep -qE '\.(Execute|Prepare)\(' "$file" 2>/dev/null; then
            # Look for sleep patterns that might be retry backoff
            # Skip if it's clearly a forced pause (ForcedPauseDuration context)
            if ! grep -qE 'ForcedPauseDuration' "$file" 2>/dev/null; then
                while IFS=: read -r line_num line_content; do
                    if [[ -n "$line_num" ]]; then
                        # Skip if this is clearly not retry-related
                        if [[ "$line_content" == *"ForcedPause"* ]] || \
                           [[ "$line_content" == *"// test"* ]] || \
                           [[ "$line_content" == *"//test"* ]]; then
                            continue
                        fi
                        echo ""
                        echo "VIOLATION: time.Sleep in file with provider calls (potential retry backoff)"
                        echo "  File:    $relative_path"
                        echo "  Line:    $line_num"
                        echo "  Content: $line_content"
                        echo "  Reason:  time.Sleep should not be used for retry backoff"
                        ((violations++)) || true
                    fi
                done < <(grep -nE 'time\.Sleep' "$file" 2>/dev/null || true)
            fi
        fi
    fi

    return $violations
}

# Check for Execute calls inside for loops (retry pattern)
check_execute_in_loop() {
    local file="$1"
    local violations=0
    local relative_path="${file#$REPO_ROOT/}"

    # Use awk to find Execute/Prepare inside for loops
    # This is a simplified check - looks for 'for' followed by 'Execute' or 'Prepare'
    # within a reasonable distance (indicating they're in the same block)

    local in_for_loop=0
    local for_line=0
    local brace_depth=0
    local line_num=0

    while IFS= read -r line; do
        ((line_num++)) || true

        # Track for loop entry
        if [[ "$line" =~ ^[[:space:]]*for[[:space:]] ]]; then
            in_for_loop=1
            for_line=$line_num
            brace_depth=0
        fi

        # Track braces if we're in a for loop
        if [[ $in_for_loop -eq 1 ]]; then
            # Count opening braces
            local open_count=$(echo "$line" | tr -cd '{' | wc -c)
            local close_count=$(echo "$line" | tr -cd '}' | wc -c)
            ((brace_depth += open_count - close_count)) || true

            # Check for Execute/Prepare calls
            if [[ "$line" =~ \.(Execute|Prepare)\( ]]; then
                # Skip if this is iterating approvals (common legitimate pattern)
                if [[ "$line" != *"approval"* ]] && [[ "$line" != *"Approval"* ]]; then
                    echo ""
                    echo "VIOLATION: Execute/Prepare call inside for loop (retry pattern)"
                    echo "  File:    $relative_path"
                    echo "  Line:    $line_num"
                    echo "  Content: $line"
                    echo "  For loop started at line: $for_line"
                    echo "  Reason:  Execute/Prepare must not be called in loops (no retries allowed)"
                    ((violations++)) || true
                fi
            fi

            # Exit for loop tracking when braces balance
            if [[ $brace_depth -le 0 ]] && [[ $line_num -gt $for_line ]]; then
                in_for_loop=0
            fi
        fi
    done < "$file"

    return $violations
}

# Check for error handling that re-invokes Execute/Prepare
check_error_retry_pattern() {
    local file="$1"
    local violations=0
    local relative_path="${file#$REPO_ROOT/}"

    # Look for patterns like: if err != nil { ... Execute ... }
    # This is a heuristic - we look for error handling blocks containing Execute/Prepare

    local in_error_block=0
    local error_line=0
    local brace_depth=0
    local line_num=0

    while IFS= read -r line; do
        ((line_num++)) || true

        # Track error handling entry (if err != nil)
        if [[ "$line" =~ if[[:space:]]+(err|e)[[:space:]]*!=[[:space:]]*nil ]]; then
            in_error_block=1
            error_line=$line_num
            brace_depth=0
        fi

        # Track braces if we're in an error block
        if [[ $in_error_block -eq 1 ]]; then
            local open_count=$(echo "$line" | tr -cd '{' | wc -c)
            local close_count=$(echo "$line" | tr -cd '}' | wc -c)
            ((brace_depth += open_count - close_count)) || true

            # Check for Execute/Prepare calls in error handling
            if [[ "$line" =~ \.(Execute|Prepare)\( ]]; then
                echo ""
                echo "VIOLATION: Execute/Prepare call in error handling block (retry pattern)"
                echo "  File:    $relative_path"
                echo "  Line:    $line_num"
                echo "  Content: $line"
                echo "  Error block started at line: $error_line"
                echo "  Reason:  Failures must not retry - new approval required"
                ((violations++)) || true
            fi

            # Exit error block tracking when braces balance
            if [[ $brace_depth -le 0 ]] && [[ $line_num -gt $error_line ]]; then
                in_error_block=0
            fi
        fi
    done < "$file"

    return $violations
}

# Check a single file
check_file() {
    local file="$1"
    local violations=0

    # Skip excluded files
    if is_excluded "$file"; then
        return 0
    fi

    local v=0

    check_retry_identifiers "$file" || v=$?
    ((violations += v)) || true

    v=0
    check_sleep_backoff "$file" || v=$?
    ((violations += v)) || true

    v=0
    check_execute_in_loop "$file" || v=$?
    ((violations += v)) || true

    v=0
    check_error_retry_pattern "$file" || v=$?
    ((violations += v)) || true

    return $violations
}

# Main check function
check_violations() {
    local root_dir="${1:-$REPO_ROOT}"
    local total_violations=0
    local files_checked=0

    print_header
    echo ""
    echo "Reference: Canon Addendum v9 - No Retries"
    echo ""
    echo "Scanning for forbidden retry patterns..."
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
        echo "Retries are FORBIDDEN in financial execution."
        echo "Failures require new approvals - no automatic retry."
        echo ""
        echo "To fix:"
        echo "  1. Remove retry loops around Execute/Prepare calls"
        echo "  2. Remove retry-related identifiers (Retry, Backoff, etc.)"
        echo "  3. Remove time.Sleep used for retry backoff"
        echo "  4. Error handling must NOT re-invoke Execute/Prepare"
        echo ""
        echo "Reference: docs/ADR/ADR-0011-no-auto-retry-and-single-trace-finalization.md"
        return 1
    else
        echo ""
        echo -e "\033[0;32mPASSED: No auto-retry violations found\033[0m"
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

    # Disable errexit for self-test
    set +e

    local tests_passed=0
    local tests_failed=0

    # -------------------------------------------------------------------------
    # Test 1: Detect retry loop calling Execute twice
    # -------------------------------------------------------------------------
    echo "Test 1: Detecting retry loop with Execute..."
    mkdir -p "$temp_dir/internal/finance/execution"
    cat > "$temp_dir/internal/finance/execution/bad_retry.go" << 'GOFILE'
package execution

func badRetryLoop() {
    for i := 0; i < 3; i++ {
        result, err := provider.Execute(ctx, req)
        if err == nil {
            return result, nil
        }
    }
}
GOFILE

    local old_repo_root="$REPO_ROOT"
    REPO_ROOT="$temp_dir"

    local output
    output=$(check_file "$temp_dir/internal/finance/execution/bad_retry.go" 2>&1)
    local result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -gt 0 ]]; then
        echo "  ✓ Retry loop detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect retry loop"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 2: Detect backoff sleep before retry
    # -------------------------------------------------------------------------
    echo "Test 2: Detecting backoff sleep..."
    cat > "$temp_dir/internal/finance/execution/bad_sleep.go" << 'GOFILE'
package execution

import "time"

func badBackoff() {
    result, err := provider.Execute(ctx, req)
    if err != nil {
        time.Sleep(1 * time.Second)
    }
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/bad_sleep.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -gt 0 ]]; then
        echo "  ✓ Backoff sleep detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect backoff sleep"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 3: Allow loop iterating approvals (no Execute)
    # -------------------------------------------------------------------------
    echo "Test 3: Allowing approval iteration loop..."
    cat > "$temp_dir/internal/finance/execution/good_approval_loop.go" << 'GOFILE'
package execution

func goodApprovalLoop(approvals []Approval) {
    for _, approval := range approvals {
        if approval.IsValid() {
            validateApproval(approval)
        }
    }
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/good_approval_loop.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -eq 0 ]]; then
        echo "  ✓ Approval loop correctly allowed"
        ((tests_passed++))
    else
        echo "  ✗ Approval loop should be allowed"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 4: Allow single Execute call with normal error return
    # -------------------------------------------------------------------------
    echo "Test 4: Allowing single Execute with error return..."
    cat > "$temp_dir/internal/finance/execution/good_single_execute.go" << 'GOFILE'
package execution

func goodSingleExecute() (*Result, error) {
    result, err := provider.Execute(ctx, req)
    if err != nil {
        return nil, err
    }
    return result, nil
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/good_single_execute.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -eq 0 ]]; then
        echo "  ✓ Single Execute correctly allowed"
        ((tests_passed++))
    else
        echo "  ✗ Single Execute should be allowed"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 5: Detect RetryPolicy identifier
    # -------------------------------------------------------------------------
    echo "Test 5: Detecting retry identifier..."
    cat > "$temp_dir/internal/finance/execution/bad_retry_policy.go" << 'GOFILE'
package execution

type RetryPolicy struct {
    MaxRetries int
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/bad_retry_policy.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -gt 0 ]]; then
        echo "  ✓ RetryPolicy identifier detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect RetryPolicy identifier"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 6: Detect error handling that re-invokes Execute
    # -------------------------------------------------------------------------
    echo "Test 6: Detecting Execute in error handler..."
    cat > "$temp_dir/internal/finance/execution/bad_error_retry.go" << 'GOFILE'
package execution

func badErrorRetry() {
    result, err := provider.Prepare(ctx, req)
    if err != nil {
        result, err = provider.Execute(ctx, req)
    }
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/bad_error_retry.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -gt 0 ]]; then
        echo "  ✓ Execute in error handler detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect Execute in error handler"
        ((tests_failed++))
    fi

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
            echo "  --check      Check for auto-retry violations (default)"
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
