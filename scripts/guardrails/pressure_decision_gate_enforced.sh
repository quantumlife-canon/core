#!/bin/bash
# Phase 32: Pressure Decision Gate Guardrails
#
# Enforces critical invariants for pressure decision classification:
# - Classification ONLY. NO notifications. NO execution.
# - NO LLM authority. Deterministic rules only.
# - NO time.Now() - clock injection only.
# - NO goroutines.
# - Max 2 INTERRUPT_CANDIDATEs per day enforced.
# - HOLD is the default.
# - NO merchant/person strings.
# - stdlib only.
#
# Usage:
#   ./pressure_decision_gate_enforced.sh --check      # Check for violations (default)
#   ./pressure_decision_gate_enforced.sh --self-test  # Run self-test
#
# Exit codes:
#   0 = No violations found
#   1 = Violations found
#
# Reference: docs/ADR/ADR-0068-phase32-pressure-decision-gate.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ============================================================================
# SCANNED DIRECTORIES
# ============================================================================
SCAN_DIRS=(
    "pkg/domain/pressuredecision"
    "internal/pressuredecision"
    "internal/persist/pressuredecision_store.go"
)

# ============================================================================
# CHECK COUNTERS
# ============================================================================
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo "========================================"
    echo "Phase 32: Pressure Decision Gate Guardrails"
    echo "========================================"
    echo
}

check_pass() {
    local msg="$1"
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
    echo "[PASS] $msg"
}

check_fail() {
    local msg="$1"
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
    echo "[FAIL] $msg"
}

file_exists() {
    local path="$1"
    [[ -f "$REPO_ROOT/$path" ]]
}

dir_exists() {
    local path="$1"
    [[ -d "$REPO_ROOT/$path" ]]
}

grep_in_dir() {
    local pattern="$1"
    local dir="$2"
    local full_path="$REPO_ROOT/$dir"

    if [[ ! -e "$full_path" ]]; then
        return 1
    fi

    if [[ -f "$full_path" ]]; then
        grep -qE "$pattern" "$full_path" 2>/dev/null
    else
        grep -rqE "$pattern" "$full_path" --include="*.go" 2>/dev/null
    fi
}

grep_not_in_dir() {
    local pattern="$1"
    local dir="$2"
    ! grep_in_dir "$pattern" "$dir"
}

count_pattern_in_dir() {
    local pattern="$1"
    local dir="$2"
    local full_path="$REPO_ROOT/$dir"

    if [[ ! -e "$full_path" ]]; then
        echo "0"
        return
    fi

    if [[ -f "$full_path" ]]; then
        grep -cE "$pattern" "$full_path" 2>/dev/null || echo "0"
    else
        grep -rE "$pattern" "$full_path" --include="*.go" 2>/dev/null | wc -l || echo "0"
    fi
}

# ============================================================================
# STRUCTURE CHECKS (10 checks)
# ============================================================================

run_structure_checks() {
    echo "--- Structure Checks ---"

    # Check 1: Domain package exists
    if dir_exists "pkg/domain/pressuredecision"; then
        check_pass "Domain package exists: pkg/domain/pressuredecision"
    else
        check_fail "Domain package missing: pkg/domain/pressuredecision"
    fi

    # Check 2: Engine package exists
    if dir_exists "internal/pressuredecision"; then
        check_pass "Engine package exists: internal/pressuredecision"
    else
        check_fail "Engine package missing: internal/pressuredecision"
    fi

    # Check 3: Types file exists
    if file_exists "pkg/domain/pressuredecision/types.go"; then
        check_pass "Types file exists: pkg/domain/pressuredecision/types.go"
    else
        check_fail "Types file missing: pkg/domain/pressuredecision/types.go"
    fi

    # Check 4: Engine file exists
    if file_exists "internal/pressuredecision/engine.go"; then
        check_pass "Engine file exists: internal/pressuredecision/engine.go"
    else
        check_fail "Engine file missing: internal/pressuredecision/engine.go"
    fi

    # Check 5: Persistence store exists
    if file_exists "internal/persist/pressuredecision_store.go"; then
        check_pass "Persistence store exists"
    else
        check_fail "Persistence store missing"
    fi

    # Check 6: ADR exists
    if file_exists "docs/ADR/ADR-0068-phase32-pressure-decision-gate.md"; then
        check_pass "ADR-0068 exists"
    else
        check_fail "ADR-0068 missing"
    fi

    # Check 7: Demo tests exist
    if dir_exists "internal/demo_phase32_pressure_decision"; then
        check_pass "Demo tests directory exists"
    else
        check_fail "Demo tests directory missing"
    fi

    # Check 8: Demo test file exists
    if file_exists "internal/demo_phase32_pressure_decision/demo_test.go"; then
        check_pass "Demo test file exists"
    else
        check_fail "Demo test file missing"
    fi

    # Check 9: Events defined
    if grep_in_dir "Phase32PressureDecisionComputed" "pkg/events/events.go"; then
        check_pass "Phase 32 events defined in events.go"
    else
        check_fail "Phase 32 events missing from events.go"
    fi

    # Check 10: Guardrails script exists (meta check)
    if file_exists "scripts/guardrails/pressure_decision_gate_enforced.sh"; then
        check_pass "Guardrails script exists"
    else
        check_fail "Guardrails script missing"
    fi

    echo
}

# ============================================================================
# DOMAIN MODEL CHECKS (10 checks)
# ============================================================================

run_domain_checks() {
    echo "--- Domain Model Checks ---"

    # Check 11: PressureDecisionKind enum exists
    if grep_in_dir "type PressureDecisionKind string" "pkg/domain/pressuredecision"; then
        check_pass "PressureDecisionKind type defined"
    else
        check_fail "PressureDecisionKind type missing"
    fi

    # Check 12: DecisionHold constant exists
    if grep_in_dir 'DecisionHold.*=.*"hold"' "pkg/domain/pressuredecision"; then
        check_pass "DecisionHold constant defined"
    else
        check_fail "DecisionHold constant missing"
    fi

    # Check 13: DecisionSurface constant exists
    if grep_in_dir 'DecisionSurface.*=.*"surface"' "pkg/domain/pressuredecision"; then
        check_pass "DecisionSurface constant defined"
    else
        check_fail "DecisionSurface constant missing"
    fi

    # Check 14: DecisionInterruptCandidate constant exists
    if grep_in_dir 'DecisionInterruptCandidate.*=.*"interrupt_candidate"' "pkg/domain/pressuredecision"; then
        check_pass "DecisionInterruptCandidate constant defined"
    else
        check_fail "DecisionInterruptCandidate constant missing"
    fi

    # Check 15: ReasonBucket type exists
    if grep_in_dir "type ReasonBucket string" "pkg/domain/pressuredecision"; then
        check_pass "ReasonBucket type defined"
    else
        check_fail "ReasonBucket type missing"
    fi

    # Check 16: CircleType enum exists
    if grep_in_dir "type CircleType string" "pkg/domain/pressuredecision"; then
        check_pass "CircleType enum defined"
    else
        check_fail "CircleType enum missing"
    fi

    # Check 17: CircleTypeCommerce defined
    if grep_in_dir 'CircleTypeCommerce.*=.*"commerce"' "pkg/domain/pressuredecision"; then
        check_pass "CircleTypeCommerce constant defined"
    else
        check_fail "CircleTypeCommerce constant missing"
    fi

    # Check 18: MaxInterruptCandidatesPerDay constant exists
    if grep_in_dir "MaxInterruptCandidatesPerDay.*=.*2" "pkg/domain/pressuredecision"; then
        check_pass "MaxInterruptCandidatesPerDay = 2"
    else
        check_fail "MaxInterruptCandidatesPerDay missing or wrong value"
    fi

    # Check 19: CanonicalString method exists
    if grep_in_dir "func.*CanonicalString.*string" "pkg/domain/pressuredecision"; then
        check_pass "CanonicalString methods defined"
    else
        check_fail "CanonicalString methods missing"
    fi

    # Check 20: ComputeHash method exists
    if grep_in_dir "func.*ComputeHash.*string" "pkg/domain/pressuredecision"; then
        check_pass "ComputeHash methods defined"
    else
        check_fail "ComputeHash methods missing"
    fi

    echo
}

# ============================================================================
# FORBIDDEN PATTERN CHECKS (15 checks)
# ============================================================================

run_forbidden_checks() {
    echo "--- Forbidden Pattern Checks ---"

    # Check 21: No notifications
    if grep_not_in_dir '\bnotify\b|\bNotify\b|\bnotification\b|\bNotification\b' "pkg/domain/pressuredecision"; then
        check_pass "No notification patterns in domain"
    else
        check_fail "Notification patterns found in domain"
    fi

    # Check 22: No execution calls
    if grep_not_in_dir 'func.*Execute\(|\.Execute\(' "internal/pressuredecision"; then
        check_pass "No Execute methods in engine"
    else
        check_fail "Execute methods found in engine"
    fi

    # Check 23: No LLM imports
    if grep_not_in_dir 'shadowllm|openai|anthropic|azure.*openai' "pkg/domain/pressuredecision"; then
        check_pass "No LLM imports in domain"
    else
        check_fail "LLM imports found in domain"
    fi

    # Check 24: No LLM imports in engine
    if grep_not_in_dir 'shadowllm|openai|anthropic|azure.*openai' "internal/pressuredecision"; then
        check_pass "No LLM imports in engine"
    else
        check_fail "LLM imports found in engine"
    fi

    # Check 25: No time.Now() in domain
    if grep_not_in_dir 'time\.Now\(' "pkg/domain/pressuredecision"; then
        check_pass "No time.Now() in domain"
    else
        check_fail "time.Now() found in domain"
    fi

    # Check 26: No time.Now() in engine
    if grep_not_in_dir 'time\.Now\(' "internal/pressuredecision"; then
        check_pass "No time.Now() in engine"
    else
        check_fail "time.Now() found in engine"
    fi

    # Check 27: No goroutines in domain
    if grep_not_in_dir 'go[[:space:]]+func[[:space:]]*\(' "pkg/domain/pressuredecision"; then
        check_pass "No goroutines in domain"
    else
        check_fail "Goroutines found in domain"
    fi

    # Check 28: No goroutines in engine
    if grep_not_in_dir 'go[[:space:]]+func[[:space:]]*\(' "internal/pressuredecision"; then
        check_pass "No goroutines in engine"
    else
        check_fail "Goroutines found in engine"
    fi

    # Check 29: No merchant strings
    if grep_not_in_dir '\bmerchant\b|\bvendor\b|\bstore\b' "pkg/domain/pressuredecision"; then
        check_pass "No merchant strings in domain"
    else
        check_fail "Merchant strings found in domain"
    fi

    # Check 30: No person names patterns
    if grep_not_in_dir '\bfirstName\b|\blastName\b|\bfull_name\b' "pkg/domain/pressuredecision"; then
        check_pass "No person name patterns in domain"
    else
        check_fail "Person name patterns found in domain"
    fi

    # Check 31: No urgency free-text
    if grep_not_in_dir '\burgent\b.*string|\bemergency\b.*string' "pkg/domain/pressuredecision"; then
        check_pass "No urgency free-text in domain"
    else
        check_fail "Urgency free-text found in domain"
    fi

    # Check 32: No external imports in domain
    if grep_not_in_dir '^\s*"[a-z]+\.[a-z]+/' "pkg/domain/pressuredecision/types.go"; then
        check_pass "No external imports in domain types"
    else
        check_fail "External imports found in domain types"
    fi

    # Check 33: No send/push patterns
    if grep_not_in_dir 'func.*Send\(|func.*Push\(' "internal/pressuredecision"; then
        check_pass "No Send/Push methods in engine"
    else
        check_fail "Send/Push methods found in engine"
    fi

    # Check 34: No UI button patterns
    if grep_not_in_dir '\bbutton\b|\bButton\b|\baction.*button' "pkg/domain/pressuredecision"; then
        check_pass "No UI button patterns in domain"
    else
        check_fail "UI button patterns found in domain"
    fi

    # Check 35: No background job patterns
    if grep_not_in_dir '\bworker\b|\bqueue\b|\bjob\b|\bcron\b' "internal/pressuredecision"; then
        check_pass "No background job patterns in engine"
    else
        check_fail "Background job patterns found in engine"
    fi

    echo
}

# ============================================================================
# REQUIRED PATTERN CHECKS (10 checks)
# ============================================================================

run_required_checks() {
    echo "--- Required Pattern Checks ---"

    # Check 36: Default HOLD path exists
    if grep_in_dir 'DecisionHold' "internal/pressuredecision/engine.go"; then
        check_pass "Default HOLD path exists in engine"
    else
        check_fail "Default HOLD path missing in engine"
    fi

    # Check 37: Commerce never interrupts rule
    if grep_in_dir 'CircleTypeCommerce' "internal/pressuredecision/engine.go"; then
        check_pass "Commerce type check exists in engine"
    else
        check_fail "Commerce type check missing in engine"
    fi

    # Check 38: Rate limit check exists
    if grep_in_dir 'MaxInterruptCandidatesPerDay|InterruptCandidatesToday' "internal/pressuredecision/engine.go"; then
        check_pass "Rate limit check exists in engine"
    else
        check_fail "Rate limit check missing in engine"
    fi

    # Check 39: Trust fragile check exists
    if grep_in_dir 'TrustStatusFragile|TrustFragile' "internal/pressuredecision/engine.go"; then
        check_pass "Trust fragile check exists in engine"
    else
        check_fail "Trust fragile check missing in engine"
    fi

    # Check 40: Deterministic hash computation
    if grep_in_dir 'sha256' "pkg/domain/pressuredecision/types.go"; then
        check_pass "SHA256 hash computation in domain"
    else
        check_fail "SHA256 hash computation missing in domain"
    fi

    # Check 41: Pipe-delimited canonical string
    if grep_in_dir 'DECISION.*|.*v1' "pkg/domain/pressuredecision/types.go"; then
        check_pass "Pipe-delimited canonical strings"
    else
        check_fail "Pipe-delimited canonical strings missing"
    fi

    # Check 42: Validate methods exist
    if grep_in_dir 'func.*Validate.*error' "pkg/domain/pressuredecision/types.go"; then
        check_pass "Validate methods defined"
    else
        check_fail "Validate methods missing"
    fi

    # Check 43: Persistence append-only
    if grep_in_dir 'Append' "internal/persist/pressuredecision_store.go"; then
        check_pass "Append method in persistence store"
    else
        check_fail "Append method missing in persistence store"
    fi

    # Check 44: No delete in persistence
    if grep_not_in_dir 'func.*Delete\(' "internal/persist/pressuredecision_store.go"; then
        check_pass "No Delete method in persistence store"
    else
        check_fail "Delete method found in persistence store"
    fi

    # Check 45: 30-day retention
    if grep_in_dir '30|MaxRetentionDays' "internal/persist/pressuredecision_store.go"; then
        check_pass "30-day retention configured"
    else
        check_fail "30-day retention not configured"
    fi

    echo
}

# ============================================================================
# ADDITIONAL SAFETY CHECKS (5+ checks)
# ============================================================================

run_safety_checks() {
    echo "--- Additional Safety Checks ---"

    # Check 46: No JSON marshaling for canonical strings
    if grep_not_in_dir 'json\.Marshal.*Canonical' "pkg/domain/pressuredecision/types.go"; then
        check_pass "No JSON marshaling for canonical strings"
    else
        check_fail "JSON marshaling used for canonical strings"
    fi

    # Check 47: Priority method exists
    if grep_in_dir 'func.*Priority.*int' "pkg/domain/pressuredecision/types.go"; then
        check_pass "Priority method defined"
    else
        check_fail "Priority method missing"
    fi

    # Check 48: All decision kinds have validators
    if grep_in_dir 'AllDecisionKinds' "pkg/domain/pressuredecision/types.go"; then
        check_pass "AllDecisionKinds helper exists"
    else
        check_fail "AllDecisionKinds helper missing"
    fi

    # Check 49: Storelog integration
    if grep_in_dir 'storelog' "internal/persist/pressuredecision_store.go"; then
        check_pass "Storelog integration in persistence"
    else
        check_fail "Storelog integration missing"
    fi

    # Check 50: Record type constant
    if grep_in_dir 'StorelogRecordType.*PRESSURE_DECISION' "internal/persist/pressuredecision_store.go"; then
        check_pass "Storelog record type defined"
    else
        check_fail "Storelog record type missing"
    fi

    echo
}

# ============================================================================
# MAIN
# ============================================================================

run_checks() {
    print_header

    run_structure_checks
    run_domain_checks
    run_forbidden_checks
    run_required_checks
    run_safety_checks

    echo "========================================"
    echo "Results: $PASSED_CHECKS/$TOTAL_CHECKS checks passed"
    echo "========================================"

    if [[ $FAILED_CHECKS -gt 0 ]]; then
        echo "FAIL: $FAILED_CHECKS violation(s) found"
        return 1
    else
        echo "OK: All checks passed"
        return 0
    fi
}

run_self_test() {
    echo "Running self-test..."

    # Test that the script can detect patterns
    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf $tmpdir" EXIT

    # Create test file with violations
    cat > "$tmpdir/test_violations.go" << 'EOF'
package test

import "time"

func badFunc() {
    notify := true
    now := time.Now()
    go func() {}()
}
EOF

    # Check detection
    local found=0
    if grep -qE '\bnotify\b' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'time\.Now\(' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi
    if grep -qE 'go[[:space:]]+func' "$tmpdir/test_violations.go"; then
        found=$((found + 1))
    fi

    if [[ $found -ge 3 ]]; then
        echo "Self-test PASSED: All patterns detected"
        return 0
    else
        echo "Self-test FAILED: Only $found/3 patterns detected"
        return 1
    fi
}

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
