#!/bin/bash
# Phase 28: Trust Kept — First Real Act, Then Silence
# Guardrails enforcing critical safety invariants.
#
# CRITICAL INVARIANTS:
#   - Only calendar_respond action allowed
#   - Single execution per period (day)
#   - 15-minute undo window (bucketed)
#   - After execution: silence forever
#   - No growth mechanics, engagement loops, or escalation paths
#   - stdlib only
#   - No goroutines in engine/store/handlers
#   - No time.Now() - clock injection only
#   - Hash-only storage (no raw identifiers)
#
# Reference: docs/ADR/ADR-0059-phase28-trust-kept.md

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Phase 28: Trust Kept guardrails ==="
echo

fail_count=0

# Helper function
check() {
    local description="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "[PASS] $description"
    else
        echo "[FAIL] $description"
        fail_count=$((fail_count + 1))
    fi
}

check_not() {
    local description="$1"
    shift
    if ! "$@" >/dev/null 2>&1; then
        echo "[PASS] $description"
    else
        echo "[FAIL] $description"
        fail_count=$((fail_count + 1))
    fi
}

echo "--- 1. Package structure ---"
check "Domain model exists" test -f "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "Engine exists" test -f "$ROOT_DIR/internal/trustaction/engine.go"
check "Store exists" test -f "$ROOT_DIR/internal/persist/trustaction_store.go"

echo
echo "--- 2. Only calendar_respond allowed ---"
check "ActionKindCalendarRespond defined" grep -q 'ActionKindCalendarRespond.*calendar_respond' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check_not "No email_send action kind" grep -q 'ActionKindEmailSend\|email_send' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check_not "No finance action kind" grep -q 'ActionKindFinance\|finance' "$ROOT_DIR/pkg/domain/trustaction/types.go"
# Only calendar_respond is allowed - verified by explicit check above

echo
echo "--- 3. State machine ---"
check "StateEligible defined" grep -q 'StateEligible.*eligible' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "StateExecuted defined" grep -q 'StateExecuted.*executed' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "StateUndone defined" grep -q 'StateUndone.*undone' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "StateExpired defined" grep -q 'StateExpired.*expired' "$ROOT_DIR/pkg/domain/trustaction/types.go"

echo
echo "--- 4. Single execution per period ---"
check "HasExecutedThisPeriod method exists in store" grep -q 'func.*HasExecutedThisPeriod' "$ROOT_DIR/internal/persist/trustaction_store.go"
check "Period key format check exists" grep -q '2006-01-02' "$ROOT_DIR/internal/trustaction/engine.go"

echo
echo "--- 5. Undo window (15 minutes) ---"
check "UndoBucket type exists" grep -q 'type UndoBucket struct' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "BucketDurationMinutes field" grep -q 'BucketDurationMinutes.*int' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "15 minute default" grep -q 'BucketDurationMinutes:.*15\|BucketDurationMinutes = 15' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "IsExpired method" grep -q 'func.*IsExpired' "$ROOT_DIR/pkg/domain/trustaction/types.go"

echo
echo "--- 6. No new execution paths ---"
check "Delegates to calendar executor" grep -q 'calendarExecutor\|ExecuteFromDraft\|calexec' "$ROOT_DIR/internal/trustaction/engine.go"
check_not "No direct writer calls in engine" grep -q 'WriteEmail\|SendEmail\|WriteCalendar' "$ROOT_DIR/internal/trustaction/engine.go"

echo
echo "--- 7. No goroutines ---"
check_not "No goroutines in domain" grep -rq 'go func\|go .*\(' "$ROOT_DIR/pkg/domain/trustaction/"
check_not "No goroutines in engine" grep -q 'go func\|go .*\(' "$ROOT_DIR/internal/trustaction/engine.go"
check_not "No goroutines in store" grep -q 'go func\|go .*\(' "$ROOT_DIR/internal/persist/trustaction_store.go"

echo
echo "--- 8. No time.Now() in code (comments allowed) ---"
check_not "No time.Now in domain code" grep -v '^[[:space:]]*//' "$ROOT_DIR/pkg/domain/trustaction/types.go" | grep -q 'time\.Now\(\)'
check_not "No time.Now in engine code" grep -v '^[[:space:]]*//' "$ROOT_DIR/internal/trustaction/engine.go" | grep -q 'time\.Now\(\)'
check_not "No time.Now in store code" grep -v '^[[:space:]]*//' "$ROOT_DIR/internal/persist/trustaction_store.go" | grep -q 'time\.Now\(\)'

echo
echo "--- 9. Clock injection ---"
check "Engine has clock field" grep -q 'clock.*func.*time\.Time' "$ROOT_DIR/internal/trustaction/engine.go"
check "Store has clock field" grep -q 'clock.*func.*time\.Time' "$ROOT_DIR/internal/persist/trustaction_store.go"

echo
echo "--- 10. Hash-only storage ---"
check "DraftIDHash field (not raw)" grep -q 'DraftIDHash.*string' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "EnvelopeHash field (not raw)" grep -q 'EnvelopeHash.*string' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "ComputeStatusHash method" grep -q 'func.*ComputeStatusHash' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "ComputeReceiptID method" grep -q 'func.*ComputeReceiptID' "$ROOT_DIR/pkg/domain/trustaction/types.go"

echo
echo "--- 11. Silence enforcement ---"
check "ShouldShowCue method exists" grep -q 'func.*ShouldShowCue' "$ROOT_DIR/internal/trustaction/engine.go"
check_not "No push notification patterns" grep -rq 'push\|notification\|notify\|remind' "$ROOT_DIR/internal/trustaction/"
check_not "No re-invitation patterns" grep -rq 'reinvite\|re-invite\|invite_again' "$ROOT_DIR/internal/trustaction/"

echo
echo "--- 12. Domain model completeness ---"
check "TrustActionKind type" grep -q 'type TrustActionKind string' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "TrustActionState type" grep -q 'type TrustActionState string' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "TrustActionPreview type" grep -q 'type TrustActionPreview struct' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "TrustActionReceipt type" grep -q 'type TrustActionReceipt struct' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "TrustActionCue type" grep -q 'type TrustActionCue struct' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "HorizonBucket type" grep -q 'type HorizonBucket string' "$ROOT_DIR/pkg/domain/trustaction/types.go"

echo
echo "--- 13. Engine methods ---"
check "CheckEligibility method" grep -q 'func.*CheckEligibility' "$ROOT_DIR/internal/trustaction/engine.go"
check "Execute method" grep -q 'func.*Execute' "$ROOT_DIR/internal/trustaction/engine.go"
check "Undo method" grep -q 'func.*Undo' "$ROOT_DIR/internal/trustaction/engine.go"
check "GetReceipt method" grep -q 'func.*GetReceipt' "$ROOT_DIR/internal/trustaction/engine.go"
check "GetLatestReceipt method" grep -q 'func.*GetLatestReceipt' "$ROOT_DIR/internal/trustaction/engine.go"

echo
echo "--- 14. Store methods ---"
check "AppendReceipt method" grep -q 'func.*AppendReceipt' "$ROOT_DIR/internal/persist/trustaction_store.go"
check "GetByID method" grep -q 'func.*GetByID' "$ROOT_DIR/internal/persist/trustaction_store.go"
check "UpdateState method" grep -q 'func.*UpdateState' "$ROOT_DIR/internal/persist/trustaction_store.go"

echo
echo "--- 15. Storelog integration ---"
check "RecordTypeTrustActionReceipt defined" grep -q 'RecordTypeTrustActionReceipt.*TRUST_ACTION_RECEIPT' "$ROOT_DIR/pkg/domain/storelog/log.go"
check "RecordTypeTrustActionUpdate defined" grep -q 'RecordTypeTrustActionUpdate.*TRUST_ACTION_UPDATE' "$ROOT_DIR/pkg/domain/storelog/log.go"

echo
echo "--- 16. Events defined ---"
check "Phase28TrustActionEligible event" grep -q 'Phase28TrustActionEligible.*phase28.trust_action.eligible' "$ROOT_DIR/pkg/events/events.go"
check "Phase28TrustActionExecuted event" grep -q 'Phase28TrustActionExecuted.*phase28.trust_action.executed' "$ROOT_DIR/pkg/events/events.go"
check "Phase28TrustActionUndone event" grep -q 'Phase28TrustActionUndone.*phase28.trust_action.undone' "$ROOT_DIR/pkg/events/events.go"
check "Phase28TrustActionExpired event" grep -q 'Phase28TrustActionExpired.*phase28.trust_action.expired' "$ROOT_DIR/pkg/events/events.go"

echo
echo "--- 17. Web routes ---"
check "/trust/action route" grep -q '"/trust/action"' "$ROOT_DIR/cmd/quantumlife-web/main.go"
check "/trust/action/execute route" grep -q '"/trust/action/execute"' "$ROOT_DIR/cmd/quantumlife-web/main.go"
check "/trust/action/undo route" grep -q '"/trust/action/undo"' "$ROOT_DIR/cmd/quantumlife-web/main.go"
check "/trust/action/receipt route" grep -q '"/trust/action/receipt"' "$ROOT_DIR/cmd/quantumlife-web/main.go"

echo
echo "--- 18. Canonical string format ---"
check "CanonicalString on Receipt" grep -q 'func.*TrustActionReceipt.*CanonicalString' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "CanonicalString on Preview" grep -q 'func.*TrustActionPreview.*CanonicalString' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "CanonicalString on UndoBucket" grep -q 'func.*UndoBucket.*CanonicalString' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "Pipe-delimited format" grep -q 'v1|' "$ROOT_DIR/pkg/domain/trustaction/types.go"

echo
echo "--- 19. Reversible always true ---"
check "Reversible field exists" grep -q 'Reversible.*bool' "$ROOT_DIR/pkg/domain/trustaction/types.go"
check "Reversible set to true in preview" grep -q 'Reversible:.*true' "$ROOT_DIR/internal/trustaction/engine.go"

echo
echo "--- 20. No forbidden UI patterns ---"
check_not "No countdown timer patterns" grep -rq 'countdown\|timer.*expire\|time.*remaining' "$ROOT_DIR/internal/trustaction/"
check_not "No urgency language in engine" grep -rq 'urgent\|hurry\|quickly\|now\!' "$ROOT_DIR/internal/trustaction/"

echo
echo "=== Results ==="
if [ $fail_count -eq 0 ]; then
    echo "All Phase 28 guardrails passed!"
    exit 0
else
    echo "FAILED: $fail_count guardrail(s) failed"
    exit 1
fi
