#!/bin/bash
# v9.12.1 Policy Snapshot Hash Enforcement Guardrail
#
# Enforces the Canon Addendum v9.12 invariant: PolicySnapshotHash is REQUIRED.
# All execution envelopes MUST have a non-empty PolicySnapshotHash bound at
# creation time. Empty hash results in hard block at execution time.
#
# Usage:
#   ./policy_snapshot_enforced.sh --check      # Check for violations (default)
#   ./policy_snapshot_enforced.sh --self-test  # Run self-test
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
# These patterns MUST be present in the executor for v9.12.1 compliance.

# Pattern 1: Hard-block when PolicySnapshotHash is empty
# The executor MUST block when PolicySnapshotHash == ""
REQUIRED_MISSING_HASH_BLOCK_PATTERN='PolicySnapshotHash == ""'

# Pattern 2: The block reason MUST be clear about missing hash
REQUIRED_MISSING_HASH_REASON='policy snapshot hash missing'

# Pattern 3: There must be a validation check recorded
REQUIRED_VALIDATION_CHECK='policy_snapshot_hash_present'

# Pattern 4: Event for missing hash must be emitted
REQUIRED_MISSING_HASH_EVENT='EventV912PolicySnapshotMissing'

# Pattern 5: Event for blocked due to missing hash
REQUIRED_BLOCKED_EVENT='EventV912ExecutionBlockedMissingHash'

# ============================================================================
# POLICY DRIFT PATTERNS (v9.12 baseline)
# ============================================================================
# These patterns ensure v9.12 policy drift detection is intact.

REQUIRED_DRIFT_VERIFICATION='computeCurrentPolicySnapshot'
REQUIRED_HASH_MISMATCH_CHECK='PolicySnapshotHash != ""'
REQUIRED_DRIFT_EVENT='EventV912PolicySnapshotMismatch'
REQUIRED_DRIFT_BLOCKED_EVENT='EventV912ExecutionBlockedPolicyDrift'

# ============================================================================
# CHECK FUNCTIONS
# ============================================================================

print_header() {
    echo "========================================"
    echo "v9.12.1 Policy Snapshot Hash Enforcement"
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

    echo "Checking executor for v9.12.1 policy snapshot enforcement..."
    echo ""

    # Check 1: Missing hash block pattern
    if grep -q "$REQUIRED_MISSING_HASH_BLOCK_PATTERN" "$executor_file"; then
        echo "  [PASS] Missing hash check: PolicySnapshotHash == \"\""
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
        echo "  [PASS] Validation check 'policy_snapshot_hash_present' recorded"
    else
        echo "  [FAIL] Validation check name not found"
        echo "         Required: $REQUIRED_VALIDATION_CHECK"
        ((violations++)) || true
    fi

    # Check 4: Missing hash event
    if grep -q "$REQUIRED_MISSING_HASH_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV912PolicySnapshotMissing emitted"
    else
        echo "  [FAIL] Missing hash event not emitted"
        echo "         Required: $REQUIRED_MISSING_HASH_EVENT"
        ((violations++)) || true
    fi

    # Check 5: Blocked event
    if grep -q "$REQUIRED_BLOCKED_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV912ExecutionBlockedMissingHash emitted"
    else
        echo "  [FAIL] Blocked event not emitted"
        echo "         Required: $REQUIRED_BLOCKED_EVENT"
        ((violations++)) || true
    fi

    echo ""
    echo "Checking v9.12 baseline policy drift enforcement..."
    echo ""

    # Check 6: Current policy snapshot computation
    if grep -q "$REQUIRED_DRIFT_VERIFICATION" "$executor_file"; then
        echo "  [PASS] Policy snapshot computation present"
    else
        echo "  [FAIL] Policy snapshot computation not found"
        echo "         Required: $REQUIRED_DRIFT_VERIFICATION"
        ((violations++)) || true
    fi

    # Check 7: Hash mismatch verification (when hash is present)
    if grep -q "$REQUIRED_HASH_MISMATCH_CHECK" "$executor_file"; then
        echo "  [PASS] Hash mismatch verification present"
    else
        echo "  [FAIL] Hash mismatch verification not found"
        echo "         Required: $REQUIRED_HASH_MISMATCH_CHECK"
        ((violations++)) || true
    fi

    # Check 8: Drift event
    if grep -q "$REQUIRED_DRIFT_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV912PolicySnapshotMismatch present"
    else
        echo "  [FAIL] Policy drift mismatch event not found"
        echo "         Required: $REQUIRED_DRIFT_EVENT"
        ((violations++)) || true
    fi

    # Check 9: Drift blocked event
    if grep -q "$REQUIRED_DRIFT_BLOCKED_EVENT" "$executor_file"; then
        echo "  [PASS] Event EventV912ExecutionBlockedPolicyDrift present"
    else
        echo "  [FAIL] Policy drift blocked event not found"
        echo "         Required: $REQUIRED_DRIFT_BLOCKED_EVENT"
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
    echo "Checking events for v9.12.1 event definitions..."
    echo ""

    # Check v9.12.1 events are defined
    if grep -q "EventV912PolicySnapshotMissing" "$events_file"; then
        echo "  [PASS] EventV912PolicySnapshotMissing defined"
    else
        echo "  [FAIL] EventV912PolicySnapshotMissing not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV912ExecutionBlockedMissingHash" "$events_file"; then
        echo "  [PASS] EventV912ExecutionBlockedMissingHash defined"
    else
        echo "  [FAIL] EventV912ExecutionBlockedMissingHash not defined"
        ((violations++)) || true
    fi

    # Check v9.12 events are defined (baseline)
    if grep -q "EventV912PolicySnapshotComputed" "$events_file"; then
        echo "  [PASS] EventV912PolicySnapshotComputed defined"
    else
        echo "  [FAIL] EventV912PolicySnapshotComputed not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV912PolicySnapshotVerified" "$events_file"; then
        echo "  [PASS] EventV912PolicySnapshotVerified defined"
    else
        echo "  [FAIL] EventV912PolicySnapshotVerified not defined"
        ((violations++)) || true
    fi

    if grep -q "EventV912PolicySnapshotMismatch" "$events_file"; then
        echo "  [PASS] EventV912PolicySnapshotMismatch defined"
    else
        echo "  [FAIL] EventV912PolicySnapshotMismatch not defined"
        ((violations++)) || true
    fi

    return $violations
}

check_violations() {
    local total_violations=0

    print_header
    echo ""
    echo "Reference: Canon Addendum v9.12 - Policy Snapshot Hash Binding"
    echo "           v9.12.1 - PolicySnapshotHash REQUIRED (not optional)"
    echo ""

    # Check executor
    local executor_violations=0
    check_executor || executor_violations=$?
    ((total_violations += executor_violations)) || true

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
        echo "v9.12.1 requires PolicySnapshotHash to be MANDATORY."
        echo "Empty hash must result in hard block at execution time."
        echo ""
        echo "To fix:"
        echo "  1. Ensure executor blocks when PolicySnapshotHash == \"\""
        echo "  2. Emit EventV912PolicySnapshotMissing when hash is missing"
        echo "  3. Emit EventV912ExecutionBlockedMissingHash when blocking"
        echo "  4. Record 'policy_snapshot_hash_present' validation check"
        echo ""
        echo "Reference: docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md"
        return 1
    else
        echo ""
        echo -e "\033[0;32mPASSED: v9.12.1 policy snapshot enforcement verified\033[0m"
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
    // v9.12.1: Check for missing hash
    if req.Envelope.PolicySnapshotHash == "" {
        // Block execution - policy snapshot hash missing
        result.BlockedReason = "policy snapshot hash missing: envelope must be bound"
        e.emitEvent(events.EventV912PolicySnapshotMissing)
        e.emitEvent(events.EventV912ExecutionBlockedMissingHash)
        result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
            Check: "policy_snapshot_hash_present",
        })
        return
    }

    // v9.12: Verify hash matches current policy
    if req.Envelope.PolicySnapshotHash != "" {
        currentSnapshot, currentHash := e.computeCurrentPolicySnapshot()
        if string(currentHash) != req.Envelope.PolicySnapshotHash {
            e.emitEvent(events.EventV912PolicySnapshotMismatch)
            e.emitEvent(events.EventV912ExecutionBlockedPolicyDrift)
        }
    }
}
GOFILE

    if grep -q 'PolicySnapshotHash == ""' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        echo "  [PASS] Missing hash check pattern detected"
        ((tests_passed++))
    else
        echo "  [FAIL] Missing hash check pattern not detected"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 2: Detect all required patterns in good file
    # -------------------------------------------------------------------------
    echo "Test 2: Detecting all v9.12.1 patterns..."

    local patterns_found=0

    if grep -q 'policy snapshot hash missing' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'policy_snapshot_hash_present' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'EventV912PolicySnapshotMissing' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi
    if grep -q 'EventV912ExecutionBlockedMissingHash' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        ((patterns_found++))
    fi

    if [[ $patterns_found -eq 4 ]]; then
        echo "  [PASS] All 4 v9.12.1 patterns found"
        ((tests_passed++))
    else
        echo "  [FAIL] Only $patterns_found/4 patterns found"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 3: Detect missing patterns in bad file
    # -------------------------------------------------------------------------
    echo "Test 3: Detecting missing patterns in incomplete file..."

    cat > "$temp_dir/internal/finance/execution/executor_v96.go" << 'GOFILE'
package execution

func (e *V96Executor) Execute() {
    // Missing v9.12.1 hard-block!
    // Only has v9.12 drift verification
    if req.Envelope.PolicySnapshotHash != "" {
        currentSnapshot, currentHash := e.computeCurrentPolicySnapshot()
        if string(currentHash) != req.Envelope.PolicySnapshotHash {
            e.emitEvent(events.EventV912PolicySnapshotMismatch)
            e.emitEvent(events.EventV912ExecutionBlockedPolicyDrift)
        }
    }
}
GOFILE

    if ! grep -q 'PolicySnapshotHash == ""' "$temp_dir/internal/finance/execution/executor_v96.go"; then
        echo "  [PASS] Correctly detected missing empty-hash check"
        ((tests_passed++))
    else
        echo "  [FAIL] Should not find empty-hash check in bad file"
        ((tests_failed++))
    fi

    # -------------------------------------------------------------------------
    # Test 4: Verify full check passes on good file
    # -------------------------------------------------------------------------
    echo "Test 4: Full check passes on compliant file..."

    # Create compliant executor
    cat > "$temp_dir/internal/finance/execution/executor_v96.go" << 'GOFILE'
package execution

func (e *V96Executor) Execute() {
    // v9.12.1: Check for missing hash
    if req.Envelope.PolicySnapshotHash == "" {
        // Block execution - policy snapshot hash missing
        result.BlockedReason = "policy snapshot hash missing: envelope must be bound"
        e.emitEvent(events.EventV912PolicySnapshotMissing)
        e.emitEvent(events.EventV912ExecutionBlockedMissingHash)
        result.ValidationDetails = append(result.ValidationDetails, ValidationCheckResult{
            Check: "policy_snapshot_hash_present",
        })
        return
    }

    // v9.12: Verify hash matches current policy
    if req.Envelope.PolicySnapshotHash != "" {
        currentSnapshot, currentHash := e.computeCurrentPolicySnapshot()
        if string(currentHash) != req.Envelope.PolicySnapshotHash {
            e.emitEvent(events.EventV912PolicySnapshotMismatch)
            e.emitEvent(events.EventV912ExecutionBlockedPolicyDrift)
        }
    }
}
GOFILE

    # Create compliant events file
    mkdir -p "$temp_dir/pkg/events"
    cat > "$temp_dir/pkg/events/events.go" << 'GOFILE'
package events

type EventType string

const (
    EventV912PolicySnapshotComputed EventType = "v9.policy.snapshot.computed"
    EventV912PolicySnapshotVerified EventType = "v9.policy.snapshot.verified"
    EventV912PolicySnapshotMismatch EventType = "v9.policy.snapshot.mismatch"
    EventV912PolicySnapshotMissing EventType = "v9.policy.snapshot.missing"
    EventV912ExecutionBlockedMissingHash EventType = "v9.execution.blocked.missing_hash"
)
GOFILE

    # Run check against temp dir
    local old_repo_root="$REPO_ROOT"
    REPO_ROOT="$temp_dir"

    if check_violations > /dev/null 2>&1; then
        echo "  [PASS] Full check passes on compliant files"
        ((tests_passed++))
    else
        echo "  [FAIL] Full check should pass on compliant files"
        ((tests_failed++))
    fi

    REPO_ROOT="$old_repo_root"

    # -------------------------------------------------------------------------
    # Test 5: Verify full check fails on non-compliant file
    # -------------------------------------------------------------------------
    echo "Test 5: Full check fails on non-compliant file..."

    # Create non-compliant executor (missing v9.12.1)
    cat > "$temp_dir/internal/finance/execution/executor_v96.go" << 'GOFILE'
package execution

func (e *V96Executor) Execute() {
    // MISSING v9.12.1 hard-block!
    // Only has v9.12 drift verification (not enough)
    if req.Envelope.PolicySnapshotHash != "" {
        currentSnapshot, currentHash := e.computeCurrentPolicySnapshot()
        if string(currentHash) != req.Envelope.PolicySnapshotHash {
            e.emitEvent(events.EventV912PolicySnapshotMismatch)
            e.emitEvent(events.EventV912ExecutionBlockedPolicyDrift)
        }
    }
}
GOFILE

    REPO_ROOT="$temp_dir"

    if check_violations > /dev/null 2>&1; then
        echo "  [FAIL] Full check should fail on non-compliant files"
        ((tests_failed++))
    else
        echo "  [PASS] Full check correctly fails on non-compliant files"
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
            echo "  --check      Check for policy snapshot enforcement (default)"
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
