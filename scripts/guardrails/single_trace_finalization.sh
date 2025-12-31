#!/bin/bash
# v9.8 Single Trace Finalization Guardrail
#
# Enforces the Canon Addendum v9 invariant: EXACTLY-ONCE trace finalization.
# Each execution attempt must finalize its audit trace exactly once.
#
# Usage:
#   ./single_trace_finalization.sh --check      # Check for violations (default)
#   ./single_trace_finalization.sh --self-test  # Run self-test
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
# Core packages that must have exactly-once trace finalization.
SCANNED_DIRS=(
    "internal/finance/execution"
    "internal/audit"
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
# FINALIZATION CALL PATTERNS (for actual calls, not definitions)
# ============================================================================
# These patterns detect CALLS to audit trace finalization functions.
# We specifically look for method calls and function calls, not definitions.
#
# NOTE: FinalizeAttempt is LEDGER state update, not audit trace finalization.
# The audit trace finalization is the emit*Finalized calls.
# Calling FinalizeAttempt + emitAttemptFinalized together is CORRECT (complementary).
FINALIZE_CALL_PATTERNS=(
    '\.emitAttemptFinalized\('
    '\.EmitTraceFinalized\('
    'emitAttemptFinalized\('
    'EmitTraceFinalized\('
)

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

# Check if a function name indicates it's a finalization helper
is_finalization_helper() {
    local func_name="$1"
    # These are helper functions that implement finalization - they're allowed to contain the pattern
    if [[ "$func_name" == "emitAttemptFinalized" ]] || \
       [[ "$func_name" == "EmitTraceFinalized" ]] || \
       [[ "$func_name" == "FinalizeAttempt" ]] || \
       [[ "$func_name" == "finalizeBlocked" ]] || \
       [[ "$func_name" == "emitBlocked" ]]; then
        return 0  # Is a helper
    fi
    return 1  # Not a helper
}

print_header() {
    echo "========================================"
    echo "v9.8 Single Trace Finalization Guardrail"
    echo "========================================"
}

# Check for defer + explicit finalization pattern
check_defer_and_explicit() {
    local file="$1"
    local violations=0
    local relative_path="${file#$REPO_ROOT/}"

    # Build combined pattern for finalization calls
    local call_regex=""
    for pattern in "${FINALIZE_CALL_PATTERNS[@]}"; do
        if [[ -n "$call_regex" ]]; then
            call_regex="$call_regex|$pattern"
        else
            call_regex="$pattern"
        fi
    done

    # Skip files without finalization calls
    if ! grep -qE "$call_regex" "$file" 2>/dev/null; then
        return 0
    fi

    # Parse function by function looking for defer + explicit
    local current_func=""
    local func_start=0
    local brace_depth=0
    local in_func=0
    local defer_finalize_lines=""
    local explicit_finalize_lines=""
    local line_num=0

    while IFS= read -r line; do
        ((line_num++)) || true

        # Detect function start
        if [[ "$line" =~ ^func[[:space:]] ]]; then
            # Check previous function
            if [[ $in_func -eq 1 ]] && [[ -n "$defer_finalize_lines" ]] && [[ -n "$explicit_finalize_lines" ]]; then
                # Skip if it's a finalization helper function
                if ! is_finalization_helper "$current_func"; then
                    echo ""
                    echo "VIOLATION: Both defer AND explicit finalization in function"
                    echo "  File:     $relative_path"
                    echo "  Function: $current_func (line $func_start)"
                    echo "  Defer:    lines $defer_finalize_lines"
                    echo "  Explicit: lines $explicit_finalize_lines"
                    echo "  Reason:   Choose either defer OR explicit, not both"
                    ((violations++)) || true
                fi
            fi

            # Extract function name (handle both regular and method syntax)
            current_func=$(echo "$line" | sed -E 's/^func[[:space:]]+(\([^)]*\)[[:space:]]+)?([a-zA-Z_][a-zA-Z0-9_]*).*/\2/')
            func_start=$line_num
            in_func=1
            brace_depth=0
            defer_finalize_lines=""
            explicit_finalize_lines=""
        fi

        # Track braces
        if [[ $in_func -eq 1 ]]; then
            local open_count=$(echo "$line" | tr -cd '{' | wc -c)
            local close_count=$(echo "$line" | tr -cd '}' | wc -c)
            ((brace_depth += open_count - close_count)) || true

            # Check for finalization calls
            for pattern in "${FINALIZE_CALL_PATTERNS[@]}"; do
                if [[ "$line" =~ $pattern ]]; then
                    if [[ "$line" =~ ^[[:space:]]*defer ]]; then
                        if [[ -n "$defer_finalize_lines" ]]; then
                            defer_finalize_lines="$defer_finalize_lines, $line_num"
                        else
                            defer_finalize_lines="$line_num"
                        fi
                    else
                        if [[ -n "$explicit_finalize_lines" ]]; then
                            explicit_finalize_lines="$explicit_finalize_lines, $line_num"
                        else
                            explicit_finalize_lines="$line_num"
                        fi
                    fi
                    break
                fi
            done

            # Function end
            if [[ $brace_depth -le 0 ]] && [[ $line_num -gt $func_start ]]; then
                # Check this function
                if [[ -n "$defer_finalize_lines" ]] && [[ -n "$explicit_finalize_lines" ]]; then
                    if ! is_finalization_helper "$current_func"; then
                        echo ""
                        echo "VIOLATION: Both defer AND explicit finalization in function"
                        echo "  File:     $relative_path"
                        echo "  Function: $current_func (line $func_start)"
                        echo "  Defer:    lines $defer_finalize_lines"
                        echo "  Explicit: lines $explicit_finalize_lines"
                        echo "  Reason:   Choose either defer OR explicit, not both"
                        ((violations++)) || true
                    fi
                fi
                in_func=0
            fi
        fi
    done < "$file"

    # Handle last function
    if [[ $in_func -eq 1 ]] && [[ -n "$defer_finalize_lines" ]] && [[ -n "$explicit_finalize_lines" ]]; then
        if ! is_finalization_helper "$current_func"; then
            echo ""
            echo "VIOLATION: Both defer AND explicit finalization in function"
            echo "  File:     $relative_path"
            echo "  Function: $current_func (line $func_start)"
            echo "  Defer:    lines $defer_finalize_lines"
            echo "  Explicit: lines $explicit_finalize_lines"
            echo "  Reason:   Choose either defer OR explicit, not both"
            ((violations++)) || true
        fi
    fi

    return $violations
}

# Check for consecutive non-branching double finalization
# This catches code like: finalize(); finalize(); (without if/else between)
check_consecutive_finalize() {
    local file="$1"
    local violations=0
    local relative_path="${file#$REPO_ROOT/}"

    # Build combined pattern
    local call_regex=""
    for pattern in "${FINALIZE_CALL_PATTERNS[@]}"; do
        if [[ -n "$call_regex" ]]; then
            call_regex="$call_regex|$pattern"
        else
            call_regex="$pattern"
        fi
    done

    # Skip files without finalization calls
    if ! grep -qE "$call_regex" "$file" 2>/dev/null; then
        return 0
    fi

    local current_func=""
    local func_start=0
    local brace_depth=0
    local in_func=0
    local last_finalize_line=0
    local had_branch_since_finalize=0
    local line_num=0

    while IFS= read -r line; do
        ((line_num++)) || true

        # Detect function start
        if [[ "$line" =~ ^func[[:space:]] ]]; then
            current_func=$(echo "$line" | sed -E 's/^func[[:space:]]+(\([^)]*\)[[:space:]]+)?([a-zA-Z_][a-zA-Z0-9_]*).*/\2/')
            func_start=$line_num
            in_func=1
            brace_depth=0
            last_finalize_line=0
            had_branch_since_finalize=0
        fi

        if [[ $in_func -eq 1 ]]; then
            local open_count=$(echo "$line" | tr -cd '{' | wc -c)
            local close_count=$(echo "$line" | tr -cd '}' | wc -c)
            ((brace_depth += open_count - close_count)) || true

            # Detect branching constructs
            if [[ "$line" =~ (^[[:space:]]*(if|else|switch|case|for|select)[[:space:]]|return[[:space:]]) ]]; then
                had_branch_since_finalize=1
            fi

            # Check for finalization calls
            for pattern in "${FINALIZE_CALL_PATTERNS[@]}"; do
                if [[ "$line" =~ $pattern ]]; then
                    # Skip defer lines (handled separately)
                    if [[ "$line" =~ ^[[:space:]]*defer ]]; then
                        break
                    fi

                    if [[ $last_finalize_line -gt 0 ]] && [[ $had_branch_since_finalize -eq 0 ]]; then
                        if ! is_finalization_helper "$current_func"; then
                            echo ""
                            echo "VIOLATION: Consecutive finalization without branching"
                            echo "  File:     $relative_path"
                            echo "  Function: $current_func (line $func_start)"
                            echo "  First:    line $last_finalize_line"
                            echo "  Second:   line $line_num"
                            echo "  Reason:   Double finalization in same code path"
                            ((violations++)) || true
                        fi
                    fi
                    last_finalize_line=$line_num
                    had_branch_since_finalize=0
                    break
                fi
            done

            # Function end
            if [[ $brace_depth -le 0 ]] && [[ $line_num -gt $func_start ]]; then
                in_func=0
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

    check_defer_and_explicit "$file" || v=$?
    ((violations += v)) || true

    v=0
    check_consecutive_finalize "$file" || v=$?
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
    echo "Reference: Canon Addendum v9 - Exactly-Once Trace Finalization"
    echo ""
    echo "Scanning for multiple finalization patterns..."
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
        echo "Trace finalization must happen EXACTLY ONCE per attempt."
        echo ""
        echo "To fix:"
        echo "  1. Choose either defer OR explicit finalization, not both"
        echo "  2. Ensure each code path finalizes exactly once"
        echo "  3. Use helper functions to centralize finalization"
        echo ""
        echo "Reference: docs/ADR/ADR-0011-no-auto-retry-and-single-trace-finalization.md"
        return 1
    else
        echo ""
        echo -e "\033[0;32mPASSED: No trace finalization violations found\033[0m"
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
    # Test 1: Detect defer + explicit finalize (violation)
    # -------------------------------------------------------------------------
    echo "Test 1: Detecting defer + explicit finalization..."
    mkdir -p "$temp_dir/internal/finance/execution"
    cat > "$temp_dir/internal/finance/execution/bad_defer_explicit.go" << 'GOFILE'
package execution

func badDeferAndExplicit() {
    defer e.emitAttemptFinalized()

    if someCondition {
        e.emitAttemptFinalized()
        return
    }
}
GOFILE

    local old_repo_root="$REPO_ROOT"
    REPO_ROOT="$temp_dir"

    local output
    output=$(check_file "$temp_dir/internal/finance/execution/bad_defer_explicit.go" 2>&1)
    local result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -gt 0 ]]; then
        echo "  ✓ Defer + explicit finalization detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect defer + explicit finalization"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 2: Detect consecutive finalization (violation)
    # -------------------------------------------------------------------------
    echo "Test 2: Detecting consecutive finalizations..."
    cat > "$temp_dir/internal/finance/execution/bad_consecutive.go" << 'GOFILE'
package execution

func badConsecutive() {
    e.emitAttemptFinalized()
    e.emitAttemptFinalized()
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/bad_consecutive.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -gt 0 ]]; then
        echo "  ✓ Consecutive finalizations detected"
        ((tests_passed++))
    else
        echo "  ✗ Failed to detect consecutive finalizations"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 3: Allow single defer finalize (pass)
    # -------------------------------------------------------------------------
    echo "Test 3: Allowing single defer finalization..."
    cat > "$temp_dir/internal/finance/execution/good_single_defer.go" << 'GOFILE'
package execution

func goodSingleDefer() {
    defer e.emitAttemptFinalized()

    if someCondition {
        return
    }
    doSomething()
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/good_single_defer.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -eq 0 ]]; then
        echo "  ✓ Single defer finalization correctly allowed"
        ((tests_passed++))
    else
        echo "  ✗ Single defer finalization should be allowed"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 4: Allow multiple explicit on different branches (pass)
    # -------------------------------------------------------------------------
    echo "Test 4: Allowing explicit finalization on different branches..."
    cat > "$temp_dir/internal/finance/execution/good_branched.go" << 'GOFILE'
package execution

func goodBranched() {
    if success {
        e.emitAttemptFinalized()
        return
    }

    if failure {
        e.emitAttemptFinalized()
        return
    }
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/good_branched.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -eq 0 ]]; then
        echo "  ✓ Branched finalization correctly allowed"
        ((tests_passed++))
    else
        echo "  ✗ Branched finalization should be allowed"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 5: Allow finalization helper function (pass)
    # -------------------------------------------------------------------------
    echo "Test 5: Allowing finalization helper function..."
    cat > "$temp_dir/internal/finance/execution/good_helper.go" << 'GOFILE'
package execution

func emitAttemptFinalized() {
    emitter.Emit(EventV96AttemptFinalized)
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/good_helper.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -eq 0 ]]; then
        echo "  ✓ Finalization helper correctly allowed"
        ((tests_passed++))
    else
        echo "  ✗ Finalization helper should be allowed"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 6: Allow function without finalization (pass)
    # -------------------------------------------------------------------------
    echo "Test 6: Allowing function without finalization..."
    cat > "$temp_dir/internal/finance/execution/good_no_finalize.go" << 'GOFILE'
package execution

func helperFunction() {
    doSomething()
    return result
}
GOFILE

    REPO_ROOT="$temp_dir"
    output=$(check_file "$temp_dir/internal/finance/execution/good_no_finalize.go" 2>&1)
    result=$?
    REPO_ROOT="$old_repo_root"

    if [[ $result -eq 0 ]]; then
        echo "  ✓ Function without finalization correctly allowed"
        ((tests_passed++))
    else
        echo "  ✗ Function without finalization should be allowed"
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
            echo "  --check      Check for multiple finalization violations (default)"
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
