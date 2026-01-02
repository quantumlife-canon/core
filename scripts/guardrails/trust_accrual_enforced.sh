#!/usr/bin/env bash
# =============================================================================
# Phase 20: Trust Accrual Layer Guardrails
# =============================================================================
#
# CRITICAL INVARIANTS:
#   - Silence is the default outcome
#   - Trust signals are NEVER pushed
#   - Trust signals are NEVER frequent
#   - Trust signals are NEVER actionable
#   - Only abstract buckets (nothing / a_few / several)
#   - NO timestamps, counts, vendors, people, or content
#   - Append-only, hash-only storage
#   - Deterministic: same inputs + clock => same hashes
#   - No goroutines, no time.Now()
#   - stdlib only
#
# Reference: docs/ADR/ADR-0048-phase20-trust-accrual-layer.md
# =============================================================================

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FAILED=0

red() { echo -e "\033[0;31m$1\033[0m"; }
green() { echo -e "\033[0;32m$1\033[0m"; }
check() {
    if eval "$2"; then
        green "✓ $1"
    else
        red "✗ $1"
        FAILED=$((FAILED + 1))
    fi
}

echo "Phase 20: Trust Accrual Layer Guardrails"
echo "========================================="

# -----------------------------------------------------------------------------
# 1. Domain Model Exists
# -----------------------------------------------------------------------------
check "pkg/domain/trust/types.go exists" \
    "[ -f '$REPO_ROOT/pkg/domain/trust/types.go' ]"

# -----------------------------------------------------------------------------
# 2. Engine Exists
# -----------------------------------------------------------------------------
check "internal/trust/engine.go exists" \
    "[ -f '$REPO_ROOT/internal/trust/engine.go' ]"

# -----------------------------------------------------------------------------
# 3. Persistence Exists
# -----------------------------------------------------------------------------
check "internal/persist/trust_store.go exists" \
    "[ -f '$REPO_ROOT/internal/persist/trust_store.go' ]"

# -----------------------------------------------------------------------------
# 4. No time.Now() in trust packages (exclude comments and test files)
# -----------------------------------------------------------------------------
check "No time.Now() in pkg/domain/trust/" \
    "! grep -rn 'time\.Now()' '$REPO_ROOT/pkg/domain/trust/' 2>/dev/null | grep -v '_test.go' | grep -v '^\s*//' | grep -v 'no time.Now' | grep -q ."

check "No time.Now() in internal/trust/" \
    "! grep -rn 'time\.Now()' '$REPO_ROOT/internal/trust/' 2>/dev/null | grep -v '_test.go' | grep -v '^\s*//' | grep -v 'no time.Now' | grep -q ."

# -----------------------------------------------------------------------------
# 5. No goroutines in trust packages
# -----------------------------------------------------------------------------
check "No goroutines in pkg/domain/trust/" \
    "! grep -rn 'go func\|go .*(' '$REPO_ROOT/pkg/domain/trust/' 2>/dev/null | grep -v '_test.go' | grep -q ."

check "No goroutines in internal/trust/" \
    "! grep -rn 'go func\|go .*(' '$REPO_ROOT/internal/trust/' 2>/dev/null | grep -v '_test.go' | grep -q ."

# -----------------------------------------------------------------------------
# 6. Only stdlib in trust packages (no external imports)
# -----------------------------------------------------------------------------
check "No external imports in pkg/domain/trust/" \
    "! grep -rn '^import' -A 20 '$REPO_ROOT/pkg/domain/trust/'*.go 2>/dev/null | grep -E 'github\.com|gopkg\.in|golang\.org/x' | grep -v '_test.go' | grep -q ."

check "No external imports in internal/trust/" \
    "! grep -rn '^import' -A 20 '$REPO_ROOT/internal/trust/'*.go 2>/dev/null | grep -E 'github\.com|gopkg\.in|golang\.org/x' | grep -v '_test.go' | grep -q ."

# -----------------------------------------------------------------------------
# 7. TrustPeriod enum exists with week/month only
# -----------------------------------------------------------------------------
check "TrustPeriod has week value" \
    "grep -q 'PeriodWeek.*TrustPeriod.*=.*\"week\"' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "TrustPeriod has month value" \
    "grep -q 'PeriodMonth.*TrustPeriod.*=.*\"month\"' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 8. TrustSignalKind enum has required values
# -----------------------------------------------------------------------------
check "SignalQuietHeld exists" \
    "grep -q 'SignalQuietHeld' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "SignalInterruptionPrevented exists" \
    "grep -q 'SignalInterruptionPrevented' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "SignalNothingRequired exists" \
    "grep -q 'SignalNothingRequired' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 9. Canonical strings use pipes (not JSON)
# -----------------------------------------------------------------------------
check "TrustSummary.CanonicalString uses pipes" \
    "grep -q 'TRUST_SUMMARY|v1|' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "TrustDismissal.CanonicalString uses pipes" \
    "grep -q 'TRUST_DISMISSAL|v1|' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 10. Uses MagnitudeBucket from shadowllm (abstract buckets)
# -----------------------------------------------------------------------------
check "Uses shadowllm.MagnitudeBucket" \
    "grep -q 'shadowllm.MagnitudeBucket' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "Engine uses MagnitudeNothing" \
    "grep -q 'MagnitudeNothing' '$REPO_ROOT/internal/trust/engine.go'"

check "Engine uses MagnitudeAFew" \
    "grep -q 'MagnitudeAFew' '$REPO_ROOT/internal/trust/engine.go'"

check "Engine uses MagnitudeSeveral" \
    "grep -q 'MagnitudeSeveral' '$REPO_ROOT/internal/trust/engine.go'"

# -----------------------------------------------------------------------------
# 11. Clock injection (uses pkg/clock)
# -----------------------------------------------------------------------------
check "Engine uses clock.Clock interface" \
    "grep -q 'clock.Clock' '$REPO_ROOT/internal/trust/engine.go'"

check "Engine gets time from clock" \
    "grep -q 'e.clk.Now()' '$REPO_ROOT/internal/trust/engine.go'"

# -----------------------------------------------------------------------------
# 12. Append-only storage pattern
# -----------------------------------------------------------------------------
check "TrustStore has AppendSummary" \
    "grep -q 'func.*AppendSummary' '$REPO_ROOT/internal/persist/trust_store.go'"

check "TrustStore has no Update method" \
    "! grep -q 'func.*UpdateSummary' '$REPO_ROOT/internal/persist/trust_store.go'"

check "TrustStore has no Delete method" \
    "! grep -q 'func.*DeleteSummary' '$REPO_ROOT/internal/persist/trust_store.go'"

# -----------------------------------------------------------------------------
# 13. Dismissal is permanent
# -----------------------------------------------------------------------------
check "TrustStore has DismissSummary" \
    "grep -q 'func.*DismissSummary' '$REPO_ROOT/internal/persist/trust_store.go'"

check "TrustStore has IsDismissed" \
    "grep -q 'func.*IsDismissed' '$REPO_ROOT/internal/persist/trust_store.go'"

# -----------------------------------------------------------------------------
# 14. Replay support exists
# -----------------------------------------------------------------------------
check "TrustStore has ReplaySummaryRecord" \
    "grep -q 'func.*ReplaySummaryRecord' '$REPO_ROOT/internal/persist/trust_store.go'"

check "TrustStore has ReplayDismissalRecord" \
    "grep -q 'func.*ReplayDismissalRecord' '$REPO_ROOT/internal/persist/trust_store.go'"

# -----------------------------------------------------------------------------
# 15. Phase 20 events exist
# -----------------------------------------------------------------------------
check "Phase20TrustComputed event exists" \
    "grep -q 'Phase20TrustComputed' '$REPO_ROOT/pkg/events/events.go'"

check "Phase20TrustPersisted event exists" \
    "grep -q 'Phase20TrustPersisted' '$REPO_ROOT/pkg/events/events.go'"

check "Phase20TrustViewed event exists" \
    "grep -q 'Phase20TrustViewed' '$REPO_ROOT/pkg/events/events.go'"

check "Phase20TrustDismissed event exists" \
    "grep -q 'Phase20TrustDismissed' '$REPO_ROOT/pkg/events/events.go'"

# -----------------------------------------------------------------------------
# 16. No performative language in human-readable strings
# -----------------------------------------------------------------------------
check "No 'saved' in HumanReadable" \
    "! grep -q '\".*saved.*\"' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "No 'protected' in HumanReadable" \
    "! grep -q '\".*protected.*\"' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "No 'amazing' in HumanReadable" \
    "! grep -q '\".*amazing.*\"' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "No 'value' in HumanReadable" \
    "! grep -q '\".*value.*\"' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 17. No raw counts exposed (only magnitude buckets)
# -----------------------------------------------------------------------------
check "countToMagnitude is internal only" \
    "grep -q 'func countToMagnitude' '$REPO_ROOT/internal/trust/engine.go'"

check "No Count fields in TrustSummary" \
    "! grep -q 'Count.*int' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 18. FiveMinuteBucket for determinism
# -----------------------------------------------------------------------------
check "FiveMinuteBucket function exists" \
    "grep -q 'func FiveMinuteBucket' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "TrustSummary has CreatedBucket" \
    "grep -q 'CreatedBucket.*string' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 19. Period keys are abstract
# -----------------------------------------------------------------------------
check "WeekKey function exists" \
    "grep -q 'func WeekKey' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "MonthKey function exists" \
    "grep -q 'func MonthKey' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 20. Validation exists
# -----------------------------------------------------------------------------
check "TrustSummary.Validate exists" \
    "grep -q 'func.*TrustSummary.*Validate' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "TrustDismissal.Validate exists" \
    "grep -q 'func.*TrustDismissal.*Validate' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 21. Silence default (returns nil when nothing happened)
# -----------------------------------------------------------------------------
check "Engine returns nil Summary when not meaningful" \
    "grep -q 'if !meaningful' '$REPO_ROOT/internal/trust/engine.go'"

# -----------------------------------------------------------------------------
# 22. SHA256 hashing
# -----------------------------------------------------------------------------
check "Uses crypto/sha256" \
    "grep -q 'crypto/sha256' '$REPO_ROOT/pkg/domain/trust/types.go'"

check "ComputeHash exists" \
    "grep -q 'func.*ComputeHash' '$REPO_ROOT/pkg/domain/trust/types.go'"

# -----------------------------------------------------------------------------
# 23. No net/http in trust packages (not a notification system)
# -----------------------------------------------------------------------------
check "No net/http in pkg/domain/trust/" \
    "! grep -q 'net/http' '$REPO_ROOT/pkg/domain/trust/'*.go 2>/dev/null"

check "No net/http in internal/trust/" \
    "! grep -q 'net/http' '$REPO_ROOT/internal/trust/'*.go 2>/dev/null"

# -----------------------------------------------------------------------------
# 24. Demo tests exist
# -----------------------------------------------------------------------------
check "Demo tests exist" \
    "[ -f '$REPO_ROOT/internal/demo_phase20_trust_accrual/demo_test.go' ]"

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "========================================="
if [ $FAILED -eq 0 ]; then
    green "All Phase 20 guardrails passed!"
else
    red "$FAILED guardrail(s) failed"
    exit 1
fi
