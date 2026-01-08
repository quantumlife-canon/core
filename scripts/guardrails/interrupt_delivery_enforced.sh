#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════════
# Phase 36: Interrupt Delivery Orchestrator Guardrails
# ═══════════════════════════════════════════════════════════════════════════════
#
# This script enforces critical invariants for Interrupt Delivery Orchestrator.
#
# CRITICAL INVARIANTS:
#   - Delivery is EXPLICIT. POST-only. No background execution.
#   - NO goroutines. NO time.Now() in business logic.
#   - Max 2 deliveries per day. Hard cap enforced.
#   - Deterministic ordering. Candidates sorted by hash.
#   - Transport-agnostic. Uses Phase 35 transport interface.
#   - Does NOT implement new decision logic.
#   - Hash-only storage.
#   - No merchant/person/institution strings.
#
# Reference: docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md
# ═══════════════════════════════════════════════════════════════════════════════

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

ERRORS=0

check() {
    local desc="$1"
    local result="$2"

    if [ "$result" -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $desc"
    else
        echo -e "${RED}✗${NC} $desc"
        ERRORS=$((ERRORS + 1))
    fi
}

# ═══════════════════════════════════════════════════════════════════════════════
# Section 1: Package Structure
# ═══════════════════════════════════════════════════════════════════════════════

echo "=== Section 1: Package Structure ==="

# 1.1 ADR exists
[ -f "docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md" ]
check "ADR-0073 exists" $?

# 1.2 Domain package exists
[ -d "pkg/domain/interruptdelivery" ]
check "Domain package exists" $?

# 1.3 Engine exists
[ -f "internal/interruptdelivery/engine.go" ]
check "Engine exists" $?

# 1.4 Persistence store exists
[ -f "internal/persist/interrupt_delivery_store.go" ]
check "Persistence store exists" $?

# 1.5 Demo tests exist
[ -d "internal/demo_phase36_interrupt_delivery" ]
check "Demo tests directory exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 2: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 2: No Goroutines ==="

# 2.1 No goroutines in domain package
! grep -r "go func" pkg/domain/interruptdelivery/ --include="*.go"
check "No goroutines in domain package" $?

# 2.2 No goroutines in engine
! grep -q "go func" internal/interruptdelivery/engine.go
check "No goroutines in engine" $?

# 2.3 No goroutines in store
! grep -q "go func" internal/persist/interrupt_delivery_store.go
check "No goroutines in store" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 3: No time.Now()
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 3: Clock Injection ==="

# 3.1 No time.Now() in domain package
! grep -r "time.Now()" pkg/domain/interruptdelivery/ --include="*.go"
check "No time.Now() in domain package" $?

# 3.2 No time.Now() in engine (excluding comments)
! grep -v "^[[:space:]]*//" internal/interruptdelivery/engine.go | grep -q "time.Now()"
check "No time.Now() in engine" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 4: Domain Model Completeness
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 4: Domain Model ==="

DOMAIN_FILE="pkg/domain/interruptdelivery/types.go"

# 4.1 DeliveryCandidate exists
grep -q "type DeliveryCandidate struct" "$DOMAIN_FILE"
check "DeliveryCandidate type exists" $?

# 4.2 DeliveryAttempt exists
grep -q "type DeliveryAttempt struct" "$DOMAIN_FILE"
check "DeliveryAttempt type exists" $?

# 4.3 DeliveryReceipt exists
grep -q "type DeliveryReceipt struct" "$DOMAIN_FILE"
check "DeliveryReceipt type exists" $?

# 4.4 TransportKind enum exists
grep -q "type TransportKind string" "$DOMAIN_FILE"
check "TransportKind enum exists" $?

# 4.5 ResultBucket enum exists
grep -q "type ResultBucket string" "$DOMAIN_FILE"
check "ResultBucket enum exists" $?

# 4.6 ReasonBucket enum exists
grep -q "type ReasonBucket string" "$DOMAIN_FILE"
check "ReasonBucket enum exists" $?

# 4.7 Validate() methods exist
grep -q "func (c \*DeliveryCandidate) Validate()" "$DOMAIN_FILE"
check "DeliveryCandidate.Validate() exists" $?

grep -q "func (a \*DeliveryAttempt) Validate()" "$DOMAIN_FILE"
check "DeliveryAttempt.Validate() exists" $?

grep -q "func (r \*DeliveryReceipt) Validate()" "$DOMAIN_FILE"
check "DeliveryReceipt.Validate() exists" $?

# 4.8 CanonicalString() methods exist
grep -q "func (c \*DeliveryCandidate) CanonicalString()" "$DOMAIN_FILE"
check "DeliveryCandidate.CanonicalString() exists" $?

grep -q "func (a \*DeliveryAttempt) CanonicalString()" "$DOMAIN_FILE"
check "DeliveryAttempt.CanonicalString() exists" $?

grep -q "func (r \*DeliveryReceipt) CanonicalString()" "$DOMAIN_FILE"
check "DeliveryReceipt.CanonicalString() exists" $?

# 4.9 MaxDeliveriesPerDay constant
grep -q "MaxDeliveriesPerDay.*=.*2" "$DOMAIN_FILE"
check "MaxDeliveriesPerDay = 2" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 5: Engine Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 5: Engine Constraints ==="

ENGINE_FILE="internal/interruptdelivery/engine.go"

# 5.1 Engine is pure (no side effects imports)
! grep -q '"net/http"' "$ENGINE_FILE"
check "Engine has no net/http import" $?

! grep -q '"os"' "$ENGINE_FILE"
check "Engine has no os import" $?

# 5.2 Uses sort for deterministic ordering
grep -q '"sort"' "$ENGINE_FILE"
check "Engine uses sort for determinism" $?

# 5.3 Has ComputeDeliveryRun function
grep -q "func (e \*Engine) ComputeDeliveryRun" "$ENGINE_FILE"
check "ComputeDeliveryRun exists" $?

# 5.4 Has BuildProofPage function
grep -q "func (e \*Engine) BuildProofPage" "$ENGINE_FILE"
check "BuildProofPage exists" $?

# 5.5 Checks policy
grep -q "PolicyAllowed" "$ENGINE_FILE"
check "Engine checks policy" $?

# 5.6 Checks daily cap
grep -q "remainingSlots" "$ENGINE_FILE" || grep -q "MaxPerDay" "$ENGINE_FILE"
check "Engine checks daily cap" $?

# 5.7 Deduplication check
grep -q "PriorAttempts" "$ENGINE_FILE" || grep -q "dedup" "$ENGINE_FILE"
check "Engine has deduplication" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 6: No New Decision Logic
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 6: No New Decision Logic ==="

# 6.1 No pressure decision logic (uses existing Phase 32)
! grep -q "DecisionInterruptCandidate" "$ENGINE_FILE"
check "Engine doesn't implement decision logic" $?

# 6.2 No policy evaluation logic (uses existing Phase 33)
! grep -q "AllowHumansNow\|AllowInstitutionsSoon" "$ENGINE_FILE"
check "Engine doesn't implement policy logic" $?

# 6.3 No transport implementation (uses Phase 35)
! grep -q "http.Client\|http.Post\|http.Get" "$ENGINE_FILE"
check "Engine doesn't implement transport" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 7: No Forbidden Strings
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 7: No Forbidden Strings ==="

# 7.1 No merchant strings in domain
! grep -qi "uber\|deliveroo\|amazon\|paypal" "$DOMAIN_FILE"
check "No merchant strings in domain" $?

# 7.2 No person identifiers in domain
! grep -qi "john\|jane\|smith\|jones" "$DOMAIN_FILE"
check "No person identifiers in domain" $?

# 7.3 No institution strings in domain
! grep -qi "hmrc\|barclays\|lloyds\|hsbc" "$DOMAIN_FILE"
check "No institution strings in domain" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 8: Persistence Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 8: Persistence ==="

STORE_FILE="internal/persist/interrupt_delivery_store.go"

# 8.1 Hash-only storage
grep -q "AttemptID.*ComputeAttemptID\|StatusHash.*ComputeStatusHash" "$STORE_FILE"
check "Store uses computed hashes" $?

# 8.2 Bounded retention
grep -q "maxRetentionDays" "$STORE_FILE"
check "Store has bounded retention" $?

# 8.3 Dedup index
grep -q "dedupIndex" "$STORE_FILE"
check "Store has dedup index" $?

# 8.4 Storelog integration
grep -q "storelogRef" "$STORE_FILE"
check "Store has storelog integration" $?

# 8.5 No raw content storage
! grep -q "rawContent\|RawContent" "$STORE_FILE"
check "No raw content in store" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 9: Events and Storelog Records
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 9: Events and Records ==="

EVENTS_FILE="pkg/events/events.go"
STORELOG_FILE="pkg/domain/storelog/log.go"

# 9.1 Phase 36 events exist
grep -q "Phase36DeliveryRequested" "$EVENTS_FILE"
check "Phase36DeliveryRequested event exists" $?

grep -q "Phase36DeliveryCompleted" "$EVENTS_FILE"
check "Phase36DeliveryCompleted event exists" $?

grep -q "Phase36ProofRendered" "$EVENTS_FILE"
check "Phase36ProofRendered event exists" $?

# 9.2 Storelog record types exist
grep -q "RecordTypeDeliveryAttempt" "$STORELOG_FILE"
check "RecordTypeDeliveryAttempt exists" $?

grep -q "RecordTypeDeliveryReceipt" "$STORELOG_FILE"
check "RecordTypeDeliveryReceipt exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 10: ADR Content
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 10: ADR Content ==="

ADR="docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md"

# 10.1 Explains explicit delivery
grep -qi "explicit" "$ADR" && grep -qi "POST" "$ADR"
check "ADR explains explicit delivery" $?

# 10.2 Explains no background execution
grep -qi "no background\|no goroutine\|POST-only" "$ADR"
check "ADR explains no background execution" $?

# 10.3 Explains transport abstraction
grep -qi "transport.*abstract\|transport-agnostic\|Phase 35" "$ADR"
check "ADR explains transport abstraction" $?

# 10.4 Explains trust preservation
grep -qi "trust\|Trust" "$ADR"
check "ADR discusses trust" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 11: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 11: Demo Tests ==="

DEMO_FILE="internal/demo_phase36_interrupt_delivery/demo_test.go"

# 11.1 Demo file exists
[ -f "$DEMO_FILE" ]
check "Demo test file exists" $?

if [ -f "$DEMO_FILE" ]; then
    # 11.2 Has sufficient test functions
    TEST_COUNT=$(grep -c "^func Test" "$DEMO_FILE" || true)
    [ "$TEST_COUNT" -ge 10 ]
    check "Has at least 10 test functions (found: $TEST_COUNT)" $?

    # 11.3 Tests deterministic ordering
    grep -q "Deterministic\|sort\|ordering" "$DEMO_FILE"
    check "Tests deterministic ordering" $?

    # 11.4 Tests policy disallow
    grep -q "Policy\|policy.*denied\|PolicyAllowed.*false" "$DEMO_FILE"
    check "Tests policy disallow" $?

    # 11.5 Tests deduplication
    grep -q "Dedup\|dedup\|already.*sent" "$DEMO_FILE"
    check "Tests deduplication" $?

    # 11.6 Tests daily cap
    grep -q "Cap\|cap\|max.*day\|MaxPerDay" "$DEMO_FILE"
    check "Tests daily cap" $?

    # 11.7 Tests hash-only persistence
    grep -q "Hash\|hash.*only\|StatusHash" "$DEMO_FILE"
    check "Tests hash-only persistence" $?
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Section 12: stdlib-only
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 12: stdlib-only ==="

# 12.1 No external libraries in domain
! grep "github.com" "$DOMAIN_FILE"
check "Domain uses stdlib only" $?

# 12.2 No external libraries in engine
! grep "github.com" "$ENGINE_FILE"
check "Engine uses stdlib only" $?

# 12.3 No external libraries in store (except internal packages)
! grep "github.com" "$STORE_FILE"
check "Store uses stdlib only" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}All Phase 36 Interrupt Delivery guardrails passed!${NC}"
    exit 0
else
    echo -e "${RED}$ERRORS guardrail(s) failed${NC}"
    exit 1
fi
