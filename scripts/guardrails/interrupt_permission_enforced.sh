#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════
# Phase 33: Interrupt Permission Contract Guardrails
# ═══════════════════════════════════════════════════════════════════════════
#
# CRITICAL INVARIANTS:
#   - NO interrupt delivery (no notifications, no emails, no SMS, no push)
#   - Policy evaluation only. No side effects.
#   - Deterministic: same inputs => same outputs.
#   - No goroutines in internal/ or pkg/
#   - No time.Now() — clock injection required
#   - stdlib-only in internal/ and pkg/
#   - Hash-only storage (no raw identifiers)
#   - Default stance: NO interrupts allowed
#
# Reference: docs/ADR/ADR-0069-phase33-interrupt-permission-contract.md
# ═══════════════════════════════════════════════════════════════════════════

set -e

REPO_ROOT="${REPO_ROOT:-$(git rev-parse --show-toplevel)}"
PASSED=0
FAILED=0
TOTAL=0

pass() {
    echo "✓ $1"
    PASSED=$((PASSED+1))
    TOTAL=$((TOTAL+1))
}

fail() {
    echo "✗ $1"
    FAILED=$((FAILED+1))
    TOTAL=$((TOTAL+1))
}

check_exists() {
    if [[ -e "$REPO_ROOT/$2" ]]; then
        pass "$1"
    else
        fail "$1: $2 not found"
    fi
}

check_file_contains() {
    if grep -q "$3" "$REPO_ROOT/$2" 2>/dev/null; then
        pass "$1"
    else
        fail "$1: pattern not found in $2"
    fi
}

check_file_not_contains() {
    if ! grep -q "$3" "$REPO_ROOT/$2" 2>/dev/null; then
        pass "$1"
    else
        fail "$1: forbidden pattern found in $2"
    fi
}

check_no_pattern_in_dir() {
    local desc="$1"
    local dir="$2"
    local pattern="$3"
    local exclude="${4:-}"

    if [[ -n "$exclude" ]]; then
        if grep -r "$pattern" "$REPO_ROOT/$dir" --include="*.go" 2>/dev/null | grep -v "$exclude" | grep -v "_test.go" | head -1 | grep -q .; then
            fail "$desc"
        else
            pass "$desc"
        fi
    else
        if grep -r "$pattern" "$REPO_ROOT/$dir" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
            fail "$desc"
        else
            pass "$desc"
        fi
    fi
}

echo "═══════════════════════════════════════════════════════════════════════════"
echo "Phase 33: Interrupt Permission Contract Guardrails"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""

# ═══════════════════════════════════════════════════════════════════════════
# Section 1: Package Structure
# ═══════════════════════════════════════════════════════════════════════════
echo "--- Package Structure ---"

check_exists "ADR-0069 exists" "docs/ADR/ADR-0069-phase33-interrupt-permission-contract.md"
check_exists "Domain types exist" "pkg/domain/interruptpolicy/types.go"
check_exists "Engine exists" "internal/interruptpolicy/engine.go"
check_exists "Policy store exists" "internal/persist/interrupt_policy_store.go"
check_exists "Proof ack store exists" "internal/persist/interrupt_proof_ack_store.go"
check_exists "Demo tests exist" "internal/demo_phase33_interrupt_permission/demo_test.go"

# ═══════════════════════════════════════════════════════════════════════════
# Section 2: NO Interrupt Delivery
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- NO Interrupt Delivery ---"

# Check for forbidden delivery patterns
check_no_pattern_in_dir "No 'notify' in interruptpolicy" "internal/interruptpolicy" "notify"
check_no_pattern_in_dir "No 'notification' in interruptpolicy" "internal/interruptpolicy" "notification"
check_no_pattern_in_dir "No 'push' in interruptpolicy" "internal/interruptpolicy" "push"
check_no_pattern_in_dir "No 'sms' in interruptpolicy" "internal/interruptpolicy" "[sS][mM][sS]"
check_no_pattern_in_dir "No 'email' send in interruptpolicy" "internal/interruptpolicy" "sendEmail\\|SendEmail"
check_no_pattern_in_dir "No 'webhook' in interruptpolicy" "internal/interruptpolicy" "webhook"

check_no_pattern_in_dir "No 'notify' in domain" "pkg/domain/interruptpolicy" "notify"
check_no_pattern_in_dir "No 'notification' in domain" "pkg/domain/interruptpolicy" "notification"
check_no_pattern_in_dir "No 'push' in domain" "pkg/domain/interruptpolicy" "push"
check_no_pattern_in_dir "No 'sms' in domain" "pkg/domain/interruptpolicy" "[sS][mM][sS]"
check_no_pattern_in_dir "No 'webhook' in domain" "pkg/domain/interruptpolicy" "webhook"

# ═══════════════════════════════════════════════════════════════════════════
# Section 3: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- No Goroutines ---"

check_no_pattern_in_dir "No goroutines in interruptpolicy" "internal/interruptpolicy" "go func"
check_no_pattern_in_dir "No goroutines in domain" "pkg/domain/interruptpolicy" "go func"
check_no_pattern_in_dir "No goroutines in persist (interrupt)" "internal/persist/interrupt" "go func"

# ═══════════════════════════════════════════════════════════════════════════
# Section 4: Clock Injection (No time.Now())
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Clock Injection ---"

# Check for time.Now() in business logic (eviction is OK)
if grep -r "time\.Now()" "$REPO_ROOT/internal/interruptpolicy" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No time.Now() in interruptpolicy engine"
else
    pass "No time.Now() in interruptpolicy engine"
fi

if grep -r "time\.Now()" "$REPO_ROOT/pkg/domain/interruptpolicy" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No time.Now() in domain types"
else
    pass "No time.Now() in domain types"
fi

# ═══════════════════════════════════════════════════════════════════════════
# Section 5: stdlib-only
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- stdlib-only ---"

# Check for forbidden imports in internal/interruptpolicy
if grep -r "github.com/" "$REPO_ROOT/internal/interruptpolicy" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No external imports in interruptpolicy"
else
    pass "No external imports in interruptpolicy"
fi

if grep -r "github.com/" "$REPO_ROOT/pkg/domain/interruptpolicy" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No external imports in domain"
else
    pass "No external imports in domain"
fi

# ═══════════════════════════════════════════════════════════════════════════
# Section 6: Domain Model Completeness
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Domain Model Completeness ---"

check_file_contains "InterruptAllowance type exists" "pkg/domain/interruptpolicy/types.go" "type InterruptAllowance string"
check_file_contains "AllowNone constant exists" "pkg/domain/interruptpolicy/types.go" "AllowNone"
check_file_contains "AllowHumansNow constant exists" "pkg/domain/interruptpolicy/types.go" "AllowHumansNow"
check_file_contains "AllowInstitutionsSoon constant exists" "pkg/domain/interruptpolicy/types.go" "AllowInstitutionsSoon"
check_file_contains "AllowTwoPerDay constant exists" "pkg/domain/interruptpolicy/types.go" "AllowTwoPerDay"

check_file_contains "InterruptPolicy type exists" "pkg/domain/interruptpolicy/types.go" "type InterruptPolicy struct"
check_file_contains "InterruptPermissionDecision type exists" "pkg/domain/interruptpolicy/types.go" "type InterruptPermissionDecision struct"
check_file_contains "InterruptProofPage type exists" "pkg/domain/interruptpolicy/types.go" "type InterruptProofPage struct"
check_file_contains "InterruptCandidate type exists" "pkg/domain/interruptpolicy/types.go" "type InterruptCandidate struct"

check_file_contains "ReasonBucket type exists" "pkg/domain/interruptpolicy/types.go" "type ReasonBucket string"
check_file_contains "MagnitudeBucket type exists" "pkg/domain/interruptpolicy/types.go" "type MagnitudeBucket string"

check_file_contains "CanonicalString method on policy" "pkg/domain/interruptpolicy/types.go" "func (p \\*InterruptPolicy) CanonicalString"
check_file_contains "Validate method on policy" "pkg/domain/interruptpolicy/types.go" "func (p \\*InterruptPolicy) Validate"

# ═══════════════════════════════════════════════════════════════════════════
# Section 7: Engine Requirements
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Engine Requirements ---"

check_file_contains "Engine type exists" "internal/interruptpolicy/engine.go" "type Engine struct"
check_file_contains "NewEngine function exists" "internal/interruptpolicy/engine.go" "func NewEngine"
check_file_contains "Evaluate method exists" "internal/interruptpolicy/engine.go" "func (e \\*Engine) Evaluate"
check_file_contains "BuildProofPage method exists" "internal/interruptpolicy/engine.go" "func (e \\*Engine) BuildProofPage"
check_file_contains "ShouldShowWhisperCue method exists" "internal/interruptpolicy/engine.go" "func (e \\*Engine) ShouldShowWhisperCue"

# ═══════════════════════════════════════════════════════════════════════════
# Section 8: Persistence Requirements
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Persistence Requirements ---"

check_file_contains "InterruptPolicyStore type exists" "internal/persist/interrupt_policy_store.go" "type InterruptPolicyStore struct"
check_file_contains "Append method on policy store" "internal/persist/interrupt_policy_store.go" "func (s \\*InterruptPolicyStore) Append"
check_file_contains "GetEffectivePolicy method" "internal/persist/interrupt_policy_store.go" "func (s \\*InterruptPolicyStore) GetEffectivePolicy"

check_file_contains "InterruptProofAckStore type exists" "internal/persist/interrupt_proof_ack_store.go" "type InterruptProofAckStore struct"
check_file_contains "IsDismissed method" "internal/persist/interrupt_proof_ack_store.go" "func (s \\*InterruptProofAckStore) IsDismissed"

# ═══════════════════════════════════════════════════════════════════════════
# Section 9: Events
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Events ---"

check_file_contains "Phase33InterruptPolicySaved event" "pkg/events/events.go" "Phase33InterruptPolicySaved"
check_file_contains "Phase33InterruptPolicyRendered event" "pkg/events/events.go" "Phase33InterruptPolicyRendered"
check_file_contains "Phase33InterruptProofRequested event" "pkg/events/events.go" "Phase33InterruptProofRequested"
check_file_contains "Phase33InterruptProofRendered event" "pkg/events/events.go" "Phase33InterruptProofRendered"
check_file_contains "Phase33InterruptProofDismissed event" "pkg/events/events.go" "Phase33InterruptProofDismissed"
check_file_contains "Phase33InterruptPermissionComputed event" "pkg/events/events.go" "Phase33InterruptPermissionComputed"

# ═══════════════════════════════════════════════════════════════════════════
# Section 10: Storelog Record Types
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Storelog Record Types ---"

check_file_contains "RecordTypeInterruptPolicy exists" "pkg/domain/storelog/log.go" "RecordTypeInterruptPolicy"
check_file_contains "RecordTypeInterruptProofAck exists" "pkg/domain/storelog/log.go" "RecordTypeInterruptProofAck"

# ═══════════════════════════════════════════════════════════════════════════
# Section 11: No Forbidden UI Tokens
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- No Forbidden UI Tokens ---"

# Check domain types for forbidden tokens
check_no_pattern_in_dir "No @ symbol in domain" "pkg/domain/interruptpolicy" "@"
check_no_pattern_in_dir "No http:// in domain" "pkg/domain/interruptpolicy" "http://"
check_no_pattern_in_dir "No amounts (£) in domain" "pkg/domain/interruptpolicy" "£"
check_no_pattern_in_dir "No amounts ($) in domain logic" "pkg/domain/interruptpolicy" '\\$[0-9]'

# ═══════════════════════════════════════════════════════════════════════════
# Section 12: Default Stance Checks
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Default Stance ---"

check_file_contains "Default is AllowNone" "pkg/domain/interruptpolicy/types.go" "AllowNone.*default"
check_file_contains "DefaultInterruptPolicy function" "pkg/domain/interruptpolicy/types.go" "func DefaultInterruptPolicy"

# ═══════════════════════════════════════════════════════════════════════════
# Section 13: Commerce Always Blocked
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Commerce Always Blocked ---"

check_file_contains "Commerce type constant" "pkg/domain/interruptpolicy/types.go" "CircleTypeCommerce"
check_file_contains "ReasonCategoryBlocked constant" "pkg/domain/interruptpolicy/types.go" "ReasonCategoryBlocked"

# ═══════════════════════════════════════════════════════════════════════════
# Section 14: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Demo Tests ---"

# Count test functions
TEST_COUNT=$(grep -c "^func Test" "$REPO_ROOT/internal/demo_phase33_interrupt_permission/demo_test.go" 2>/dev/null || echo "0")
if [[ "$TEST_COUNT" -ge 18 ]]; then
    pass "At least 18 test functions ($TEST_COUNT found)"
else
    fail "At least 18 test functions (only $TEST_COUNT found)"
fi

check_file_contains "Determinism test exists" "internal/demo_phase33_interrupt_permission/demo_test.go" "TestDeterminism"
check_file_contains "Default policy test exists" "internal/demo_phase33_interrupt_permission/demo_test.go" "TestDefaultPolicy"
check_file_contains "Commerce blocked test exists" "internal/demo_phase33_interrupt_permission/demo_test.go" "TestCommerce"
check_file_contains "MaxPerDay test exists" "internal/demo_phase33_interrupt_permission/demo_test.go" "TestMaxPerDay"
check_file_contains "Trust fragile test exists" "internal/demo_phase33_interrupt_permission/demo_test.go" "TestTrustFragile"

# ═══════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "Summary: $PASSED passed, $FAILED failed, $TOTAL total"
echo "═══════════════════════════════════════════════════════════════════════════"

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi

exit 0
