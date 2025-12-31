#!/bin/bash
# v9.13 View Snapshot Hash Enforcement Guardrail
#
# Enforces the Canon Addendum v9.13 invariant: ViewSnapshotHash is REQUIRED.
# All execution envelopes MUST have a non-empty ViewSnapshotHash bound at
# creation time. Empty hash results in hard block at execution time.
#
# This ensures:
# 1. Read-before-write: View must be fetched before execution
# 2. View freshness: View must be within MaxStaleness at execution
# 3. Hash binding: Envelope is bound to specific view state
# 4. Drift detection: View change between approval and execution blocks
#
# Usage:
#   ./view_snapshot_enforced.sh --check      # Check for violations (default)
#   ./view_snapshot_enforced.sh --self-test  # Run self-test
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found
#   2 = Script usage error
#
# Reference:
#   - docs/QUANTUMLIFE_CANON_V1.md
#   - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
#   - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# REQUIRED PATTERNS
# ============================================================================
# These patterns MUST be present in the executor for v9.13 compliance.

# Pattern 1: Hard-block when ViewSnapshotHash is empty
# The executor MUST block when ViewSnapshotHash == ""
REQUIRED_MISSING_HASH_BLOCK_PATTERN='ViewSnapshotHash == ""'

# Pattern 2: The block reason MUST be clear about missing hash
REQUIRED_MISSING_HASH_REASON='view snapshot hash missing'

# Pattern 3: There must be a validation check recorded
REQUIRED_VALIDATION_CHECK='view_snapshot_hash_present'

# Pattern 4: Event for missing hash must be emitted
REQUIRED_MISSING_HASH_EVENT='EventV913ExecutionBlockedViewHashMissing'

# ============================================================================
# VIEW FRESHNESS PATTERNS (v9.13 core)
# ============================================================================
# These patterns ensure v9.13 view freshness verification is intact.

REQUIRED_VIEW_SNAPSHOT_FETCH='GetViewSnapshot'
REQUIRED_FRESHNESS_CHECK='CheckViewFreshness'
REQUIRED_HASH_MISMATCH_EVENT='EventV913ViewHashMismatch'
REQUIRED_STALE_BLOCKED_EVENT='EventV913ExecutionBlockedViewStale'
REQUIRED_HASH_MISMATCH_BLOCKED_EVENT='EventV913ExecutionBlockedViewHashMismatch'

# ============================================================================
# MULTI-PARTY SYMMETRY PATTERNS (v9.13)
# ============================================================================
# These patterns ensure ViewSnapshotHash is verified in multi-party gate.

REQUIRED_MULTIPARTY_HASH_CHECK='ViewSnapshotHash'
REQUIRED_MULTIPARTY_FILE='multiparty_gate.go'

# ============================================================================
# CHECK FUNCTIONS
# ============================================================================

print_header() {
    echo "========================================"
    echo "v9.13 View Snapshot Hash Enforcement"
    echo "========================================"
}

check_executor() {
    local executor_file="$REPO_ROOT/internal/finance/execution/executor_v96.go"
    local violations=0

    if [[ ! -f "$executor_file" ]]; then
        echo ""
        echo "VIOLATION: Executor file not found"
        echo "  Expected: internal/finance/execution/executor_v96.go"
        return 1
    fi

    echo "Checking executor for v9.13 view snapshot enforcement..."
    echo ""

    # Check 1: Missing hash block pattern
    if grep -q "$REQUIRED_MISSING_HASH_BLOCK_PATTERN" "$executor_file"; then
        echo "  [PASS] Missing hash check: ViewSnapshotHash == \"\""
    else
        echo "  [FAIL] Missing hash check pattern not found"
        echo "         Required: $REQUIRED_MISSING_HASH_BLOCK_PATTERN"
        ((violations++)) || true
    fi

    # Check 2: Missing hash reason
    if grep -q "$REQUIRED_MISSING_HASH_REASON" "$executor_file"; then
        echo "  [PASS] Missing hash reason message present"
    else
        echo "  [FAIL] Missing hash reason not found"
        echo "         Required: $REQUIRED_MISSING_HASH_REASON"
        ((violations++)) || true
    fi

    # Check 3: Validation check name
    if grep -q "$REQUIRED_VALIDATION_CHECK" "$executor_file"; then
        echo "  [PASS] Validation check 'view_snapshot_hash_present' recorded"
    else
        echo "  [FAIL] Validation check name not found"
        echo "         Required: $REQUIRED_VALIDATION_CHECK"
        ((violations++)) || true
    fi

    # Check 4: Missing hash event
    if grep -q "$REQUIRED_MISSING_HASH_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV913ExecutionBlockedViewHashMissing emitted"
    else
        echo "  [FAIL] Missing hash event not emitted"
        echo "         Required: $REQUIRED_MISSING_HASH_EVENT"
        ((violations++)) || true
    fi

    echo ""
    echo "Checking v9.13 view freshness verification..."
    echo ""

    # Check 5: View snapshot fetch
    if grep -q "$REQUIRED_VIEW_SNAPSHOT_FETCH" "$executor_file"; then
        echo "  [PASS] View snapshot fetch (GetViewSnapshot) present"
    else
        echo "  [FAIL] View snapshot fetch not found"
        echo "         Required: $REQUIRED_VIEW_SNAPSHOT_FETCH"
        ((violations++)) || true
    fi

    # Check 6: Freshness check
    if grep -q "$REQUIRED_FRESHNESS_CHECK" "$executor_file"; then
        echo "  [PASS] View freshness check (CheckViewFreshness) present"
    else
        echo "  [FAIL] Freshness check not found"
        echo "         Required: $REQUIRED_FRESHNESS_CHECK"
        ((violations++)) || true
    fi

    # Check 7: Hash mismatch event
    if grep -q "$REQUIRED_HASH_MISMATCH_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV913ViewHashMismatch present"
    else
        echo "  [FAIL] Hash mismatch event not found"
        echo "         Required: $REQUIRED_HASH_MISMATCH_EVENT"
        ((violations++)) || true
    fi

    # Check 8: Stale blocked event
    if grep -q "$REQUIRED_STALE_BLOCKED_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV913ExecutionBlockedViewStale present"
    else
        echo "  [FAIL] Stale blocked event not found"
        echo "         Required: $REQUIRED_STALE_BLOCKED_EVENT"
        ((violations++)) || true
    fi

    # Check 9: Hash mismatch blocked event
    if grep -q "$REQUIRED_HASH_MISMATCH_BLOCKED_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV913ExecutionBlockedViewHashMismatch present"
    else
        echo "  [FAIL] Hash mismatch blocked event not found"
        echo "         Required: $REQUIRED_HASH_MISMATCH_BLOCKED_EVENT"
        ((violations++)) || true
    fi

    return $violations
}

check_multiparty_gate() {
    local gate_file="$REPO_ROOT/internal/finance/execution/$REQUIRED_MULTIPARTY_FILE"
    local violations=0

    if [[ ! -f "$gate_file" ]]; then
        echo ""
        echo "VIOLATION: Multi-party gate file not found"
        echo "  Expected: internal/finance/execution/$REQUIRED_MULTIPARTY_FILE"
        return 1
    fi

    echo ""
    echo "Checking multi-party gate for v9.13 ViewSnapshotHash symmetry..."
    echo ""

    # Check ViewSnapshotHash is verified in multi-party gate
    if grep -q "$REQUIRED_MULTIPARTY_HASH_CHECK" "$gate_file"; then
        echo "  [PASS] ViewSnapshotHash verified in multi-party gate"
    else
        echo "  [FAIL] ViewSnapshotHash not verified in multi-party gate"
        echo "         Required: $REQUIRED_MULTIPARTY_HASH_CHECK"
        ((violations++)) || true
    fi

    # Check for v9.13 events in multi-party gate
    if grep -q "EventV913ViewHashVerified\|EventV913ViewHashMismatch" "$gate_file"; then
        echo "  [PASS] v9.13 events referenced in multi-party gate"
    else
        echo "  [FAIL] v9.13 events not referenced in multi-party gate"
        ((violations++)) || true
    fi

    return $violations
}

check_events() {
    local events_file="$REPO_ROOT/pkg/events/events.go"
    local violations=0

    if [[ ! -f "$events_file" ]]; then
        echo ""
        echo "VIOLATION: Events file not found"
        echo "  Expected: pkg/events/events.go"
        return 1
    fi

    echo ""
    echo "Checking events for v9.13 event definitions..."
    echo ""

    # Check v9.13 events are defined
    if grep -q "EventV913ViewSnapshotRequested" "$events_file"; then
        echo "  [PASS] EventV913ViewSnapshotRequested defined"
    else
        echo "  [FAIL] EventV913ViewSnapshotRequested not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ViewSnapshotReceived" "$events_file"; then
        echo "  [PASS] EventV913ViewSnapshotReceived defined"
    else
        echo "  [FAIL] EventV913ViewSnapshotReceived not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ViewFreshnessChecked" "$events_file"; then
        echo "  [PASS] EventV913ViewFreshnessChecked defined"
    else
        echo "  [FAIL] EventV913ViewFreshnessChecked not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ViewHashVerified" "$events_file"; then
        echo "  [PASS] EventV913ViewHashVerified defined"
    else
        echo "  [FAIL] EventV913ViewHashVerified not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ViewHashMismatch" "$events_file"; then
        echo "  [PASS] EventV913ViewHashMismatch defined"
    else
        echo "  [FAIL] EventV913ViewHashMismatch not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ExecutionBlockedViewStale" "$events_file"; then
        echo "  [PASS] EventV913ExecutionBlockedViewStale defined"
    else
        echo "  [FAIL] EventV913ExecutionBlockedViewStale not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ExecutionBlockedViewHashMismatch" "$events_file"; then
        echo "  [PASS] EventV913ExecutionBlockedViewHashMismatch defined"
    else
        echo "  [FAIL] EventV913ExecutionBlockedViewHashMismatch not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ExecutionBlockedViewHashMissing" "$events_file"; then
        echo "  [PASS] EventV913ExecutionBlockedViewHashMissing defined"
    else
        echo "  [FAIL] EventV913ExecutionBlockedViewHashMissing not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV913ViewSnapshotBound" "$events_file"; then
        echo "  [PASS] EventV913ViewSnapshotBound defined"
    else
        echo "  [FAIL] EventV913ViewSnapshotBound not defined"
        ((violations++)) || true
    fi

    return $violations
}

check_violations() {
    local total_violations=0

    print_header
    echo ""
    echo "Reference: Canon Addendum v9.13 - View Freshness Binding"
    echo "           ViewSnapshotHash is REQUIRED (not optional)"
    echo "           View must be fresh (within MaxStaleness)"
    echo ""

    # Check executor
    local executor_violations=0
    check_executor || executor_violations=$?
    ((total_violations += executor_violations)) || true

    # Check multi-party gate
    local gate_violations=0
    check_multiparty_gate || gate_violations=$?
    ((total_violations += gate_violations)) || true

    # Check events
    local events_violations=0
    check_events || events_violations=$?
    ((total_violations += events_violations)) || true

    echo ""
    echo "========================================"

    if [[ $total_violations -gt 0 ]]; then
        echo ""
        echo -e "\033[0;31mFAILED: Found $total_violations violation(s)\033[0m"
        echo ""
        echo "v9.13 requires ViewSnapshotHash to be MANDATORY."
        echo "Empty hash must result in hard block at execution time."
        echo "View must be fetched and verified fresh before execution."
        echo ""
        echo "To fix:"
        echo "  1. Ensure executor blocks when ViewSnapshotHash == \"\""
        echo "  2. Emit EventV913ExecutionBlockedViewHashMissing when hash is missing"
        echo "  3. Fetch view via GetViewSnapshot and check freshness"
        echo "  4. Verify ViewSnapshotHash symmetry in multi-party gate"
        echo "  5. Record 'view_snapshot_hash_present' validation check"
        echo ""
        echo "Reference: docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md"
        return 1
    else
        echo ""
        echo -e "\033[0;32mPASSED: v9.13 view snapshot enforcement verified\033[0m"
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
    # Test 1: Detect missing hash check pattern
    # -------------------------------------------------------------------------
    echo "Test 1: Detecting missing hash check pattern..."

    mkdir -p "$temp_dir/internal/finance/execution"
    cat > "$temp_dir/internal/finance/execution/executor_v96.go" << 'GOFILE'
package execution

func (e *V96Executor) Execute() {
    // v9.13: Check for missing view hash
    if req.Envelope.ViewSnapshotHash == "" {
        // Block execution - view snapshot hash missing
        result.BlockedReason = "view snapshot hash missing: envelope must be bound"
        e.emitEvent(events.EventV913ExecutionBlockedViewHashMissing)
        result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
            Check: "view_snapshot_hash_present",
        })
        return
    }

    // v9.13: Fetch and verify view freshness
    currentView, _ := e.viewProvider.GetViewSnapshot(ctx, req)
    freshnessResult := CheckViewFreshness(currentView, now, maxStaleness)
    if !freshnessResult.Fresh {
        e.emitEvent(events.EventV913ExecutionBlockedViewStale)
        return
    }

    // v9.13: Verify hash matches
    if currentHash != expectedHash {
        e.emitEvent(events.EventV913ViewHashMismatch)
        e.emitEvent(events.EventV913ExecutionBlockedViewHashMismatch)
        return
    }
}
GOFILE

    if grep -q 'ViewSnapshotHash == ""' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        echo "  [PASS] Missing hash check pattern detected"
        ((tests_passed++))
    else
        echo "  [FAIL] Missing hash check pattern not detected"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 2: Detect all required patterns in good file
    # -------------------------------------------------------------------------
    echo "Test 2: Detecting all v9.13 patterns..."

    local patterns_found=0

    if grep -q 'view snapshot hash missing' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'view_snapshot_hash_present' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'EventV913ExecutionBlockedViewHashMissing' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'GetViewSnapshot' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'CheckViewFreshness' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi

    if [[ $patterns_found -eq 5 ]]; then
        echo "  [PASS] All 5 key v9.13 patterns found"
        ((tests_passed++))
    else
        echo "  [FAIL] Only $patterns_found/5 patterns found"
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
            echo "  --check      Check for view snapshot enforcement (default)"
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
