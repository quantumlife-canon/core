#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════════
# Phase 38: Mobile Notification Metadata Observer Guardrails
# ═══════════════════════════════════════════════════════════════════════════════
#
# This script enforces critical invariants for Notification Metadata Observer.
#
# CRITICAL INVARIANTS:
#   - NO notification content - only OS-provided category metadata
#   - NO app names - only abstract class buckets
#   - NO device identifiers - hash-only storage
#   - NO time.Now() in internal/ or pkg/ business logic
#   - NO goroutines in internal/ or pkg/
#   - NO network calls - pure local observation
#   - NO decision logic - observation ONLY
#   - NO delivery triggers - cannot send notifications
#   - stdlib only
#   - Max 1 signal per app class per period
#   - Bounded retention: 200 records OR 30 days
#
# Reference: docs/ADR/ADR-0075-phase38-notification-metadata-observer.md
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
[ -f "docs/ADR/ADR-0075-phase38-notification-metadata-observer.md" ]
check "ADR-0075 exists" $?

# 1.2 Domain package exists
[ -d "pkg/domain/notificationobserver" ]
check "Domain package exists" $?

# 1.3 Engine exists
[ -f "internal/notificationobserver/engine.go" ]
check "Engine exists" $?

# 1.4 Persistence store exists
[ -f "internal/persist/notificationobserver_store.go" ]
check "Persistence store exists" $?

# 1.5 Demo tests exist
[ -d "internal/demo_phase38_notification_observer" ]
check "Demo tests directory exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 2: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 2: No Goroutines ==="

# 2.1 No goroutines in domain package
! grep -r "go func" pkg/domain/notificationobserver/ --include="*.go"
check "No goroutines in domain package" $?

# 2.2 No goroutines in engine
! grep -q "go func" internal/notificationobserver/engine.go
check "No goroutines in engine" $?

# 2.3 No goroutines in store
! grep -q "go func" internal/persist/notificationobserver_store.go
check "No goroutines in store" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 3: No time.Now()
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 3: Clock Injection ==="

# 3.1 No time.Now() in domain package (excluding comments)
! grep -r "time.Now()" pkg/domain/notificationobserver/ --include="*.go" | grep -v "^[^:]*:[[:space:]]*//\|eviction only"
check "No time.Now() in domain package" $?

# 3.2 No time.Now() in engine (excluding comments)
! grep -v "^[[:space:]]*//\|eviction only" internal/notificationobserver/engine.go | grep -q "time.Now()"
check "No time.Now() in engine" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 4: Domain Model Completeness
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 4: Domain Model ==="

DOMAIN_FILE="pkg/domain/notificationobserver/types.go"

# 4.1 NotificationSourceKind exists
grep -q "type NotificationSourceKind string" "$DOMAIN_FILE"
check "NotificationSourceKind type exists" $?

# 4.2 NotificationAppClass exists
grep -q "type NotificationAppClass string" "$DOMAIN_FILE"
check "NotificationAppClass type exists" $?

# 4.3 MagnitudeBucket exists
grep -q "type MagnitudeBucket string" "$DOMAIN_FILE"
check "MagnitudeBucket type exists" $?

# 4.4 HorizonBucket exists
grep -q "type HorizonBucket string" "$DOMAIN_FILE"
check "HorizonBucket type exists" $?

# 4.5 NotificationPressureSignal exists
grep -q "type NotificationPressureSignal struct" "$DOMAIN_FILE"
check "NotificationPressureSignal type exists" $?

# 4.6 NotificationObserverInput exists
grep -q "type NotificationObserverInput struct" "$DOMAIN_FILE"
check "NotificationObserverInput type exists" $?

# 4.7 App class values exist
grep -q "AppClassTransport.*=.*\"transport\"" "$DOMAIN_FILE"
check "AppClassTransport exists" $?

grep -q "AppClassHealth.*=.*\"health\"" "$DOMAIN_FILE"
check "AppClassHealth exists" $?

grep -q "AppClassInstitution.*=.*\"institution\"" "$DOMAIN_FILE"
check "AppClassInstitution exists" $?

grep -q "AppClassCommerce.*=.*\"commerce\"" "$DOMAIN_FILE"
check "AppClassCommerce exists" $?

grep -q "AppClassUnknown.*=.*\"unknown\"" "$DOMAIN_FILE"
check "AppClassUnknown exists" $?

# 4.8 Magnitude values exist
grep -q "MagnitudeNothing.*=.*\"nothing\"" "$DOMAIN_FILE"
check "MagnitudeNothing exists" $?

grep -q "MagnitudeAFew.*=.*\"a_few\"" "$DOMAIN_FILE"
check "MagnitudeAFew exists" $?

grep -q "MagnitudeSeveral.*=.*\"several\"" "$DOMAIN_FILE"
check "MagnitudeSeveral exists" $?

# 4.9 Horizon values exist
grep -q "HorizonNow.*=.*\"now\"" "$DOMAIN_FILE"
check "HorizonNow exists" $?

grep -q "HorizonSoon.*=.*\"soon\"" "$DOMAIN_FILE"
check "HorizonSoon exists" $?

grep -q "HorizonLater.*=.*\"later\"" "$DOMAIN_FILE"
check "HorizonLater exists" $?

# 4.10 Validate() methods exist
grep -q "func (s NotificationSourceKind) Validate()" "$DOMAIN_FILE"
check "NotificationSourceKind.Validate() exists" $?

grep -q "func (c NotificationAppClass) Validate()" "$DOMAIN_FILE"
check "NotificationAppClass.Validate() exists" $?

grep -q "func (m MagnitudeBucket) Validate()" "$DOMAIN_FILE"
check "MagnitudeBucket.Validate() exists" $?

grep -q "func (h HorizonBucket) Validate()" "$DOMAIN_FILE"
check "HorizonBucket.Validate() exists" $?

# 4.11 CanonicalString() methods exist
grep -q "func (s NotificationSourceKind) CanonicalString()" "$DOMAIN_FILE"
check "NotificationSourceKind.CanonicalString() exists" $?

grep -q "func (s \*NotificationPressureSignal) CanonicalString()" "$DOMAIN_FILE"
check "NotificationPressureSignal.CanonicalString() exists" $?

# 4.12 Bounded retention constants
grep -q "MaxSignalRecords.*=.*200" "$DOMAIN_FILE"
check "MaxSignalRecords = 200" $?

grep -q "MaxRetentionDays.*=.*30" "$DOMAIN_FILE"
check "MaxRetentionDays = 30" $?

# 4.13 Max 1 per app class per period
grep -q "MaxSignalsPerAppClassPerPeriod.*=.*1" "$DOMAIN_FILE"
check "MaxSignalsPerAppClassPerPeriod = 1" $?

# 4.14 Forbidden content check helper
grep -q "CheckForbiddenContent" "$DOMAIN_FILE"
check "CheckForbiddenContent helper exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 5: Engine Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 5: Engine Constraints ==="

ENGINE_FILE="internal/notificationobserver/engine.go"

# 5.1 Engine is pure (no side effects imports)
! grep -q '"net/http"' "$ENGINE_FILE"
check "Engine has no net/http import" $?

! grep -q '"os"' "$ENGINE_FILE"
check "Engine has no os import" $?

! grep -q '"io"' "$ENGINE_FILE"
check "Engine has no io import" $?

# 5.2 Has ObserveNotificationMetadata function
grep -q "func (e \*Engine) ObserveNotificationMetadata" "$ENGINE_FILE"
check "ObserveNotificationMetadata exists" $?

# 5.3 Has ShouldHold function
grep -q "func (e \*Engine) ShouldHold" "$ENGINE_FILE"
check "ShouldHold exists" $?

# 5.4 Has ConvertToPressureInput function
grep -q "func (e \*Engine) ConvertToPressureInput" "$ENGINE_FILE"
check "ConvertToPressureInput exists" $?

# 5.5 Has BuildInputFromParams function
grep -q "func (e \*Engine) BuildInputFromParams" "$ENGINE_FILE"
check "BuildInputFromParams exists" $?

# 5.6 Has MergeSignals function
grep -q "func (e \*Engine) MergeSignals" "$ENGINE_FILE"
check "MergeSignals exists" $?

# 5.7 Integrates with externalpressure (Phase 31.4)
grep -q "externalpressure" "$ENGINE_FILE"
check "Engine integrates with externalpressure" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 6: No Notification Content
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 6: No Notification Content ==="

# 6.1 No title/body/content fields in domain
! grep -qE "Title|Body|Content|Message" "$DOMAIN_FILE" 2>/dev/null || \
    ! grep -qE "title\s+string|body\s+string|content\s+string|message\s+string" "$DOMAIN_FILE"
check "No notification content fields in domain" $?

# 6.2 No app name field
! grep -qE "AppName|app_name|appName" "$DOMAIN_FILE" 2>/dev/null
check "No app name fields in domain" $?

# 6.3 No sender/recipient fields
! grep -qE "Sender|Recipient|sender|recipient" "$DOMAIN_FILE" 2>/dev/null
check "No sender/recipient fields in domain" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 7: Persistence Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 7: Persistence ==="

STORE_FILE="internal/persist/notificationobserver_store.go"

# 7.1 Bounded retention
grep -q "maxRetentionDays" "$STORE_FILE"
check "Store has maxRetentionDays" $?

grep -q "maxRecords" "$STORE_FILE"
check "Store has maxRecords" $?

# 7.2 FIFO eviction
grep -q "allRecordIDs" "$STORE_FILE"
check "Store has FIFO tracking" $?

grep -q "evictLocked\|EvictOldPeriods" "$STORE_FILE"
check "Store has eviction" $?

# 7.3 Storelog integration
grep -q "storelogRef" "$STORE_FILE"
check "Store has storelog integration" $?

# 7.4 Signal deduplication (max 1 per app class per period)
grep -q "SignalID" "$STORE_FILE"
check "Store uses SignalID for deduplication" $?

# 7.5 Hash-only storage
grep -q "StatusHash\|EvidenceHash" "$STORE_FILE"
check "Store uses hash-only storage" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 8: Events and Storelog Records
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 8: Events and Records ==="

EVENTS_FILE="pkg/events/events.go"
STORELOG_FILE="pkg/domain/storelog/log.go"

# 8.1 Phase 38 events exist
grep -q "Phase38NotificationObserved" "$EVENTS_FILE"
check "Phase38NotificationObserved event exists" $?

grep -q "Phase38NotificationIgnored" "$EVENTS_FILE"
check "Phase38NotificationIgnored event exists" $?

grep -q "Phase38NotificationPersisted" "$EVENTS_FILE"
check "Phase38NotificationPersisted event exists" $?

grep -q "Phase38PressureInputCreated" "$EVENTS_FILE"
check "Phase38PressureInputCreated event exists" $?

# 8.2 Storelog record types exist
grep -q "RecordTypeNotificationSignal" "$STORELOG_FILE"
check "RecordTypeNotificationSignal exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 9: Web Route
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 9: Web Route ==="

MAIN_FILE="cmd/quantumlife-web/main.go"

# 9.1 Route exists
grep -q '"/observe/notification"' "$MAIN_FILE"
check "POST /observe/notification route exists" $?

# 9.2 Handler exists
grep -q "handleObserveNotification" "$MAIN_FILE"
check "handleObserveNotification handler exists" $?

# 9.3 Handler uses engine
grep -A50 "handleObserveNotification" "$MAIN_FILE" | grep -q "notifObserverEngine"
check "Handler uses notification observer engine" $?

# 9.4 Handler uses store
grep -A120 "handleObserveNotification" "$MAIN_FILE" | grep -q "notifObserverStore"
check "Handler uses notification observer store" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 10: ADR Content
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 10: ADR Content ==="

ADR="docs/ADR/ADR-0075-phase38-notification-metadata-observer.md"

# 10.1 Explains no notification content
grep -qi "no notification content\|notification content.*forbidden" "$ADR"
check "ADR explains no notification content" $?

# 10.2 Explains abstraction
grep -qi "abstract\|bucket" "$ADR"
check "ADR explains abstraction" $?

# 10.3 Explains observation only
grep -qi "observation only\|cannot interrupt\|cannot deliver" "$ADR"
check "ADR explains observation-only" $?

# 10.4 Explains Phase 31.4 integration
grep -qi "phase 31\.4\|pressure pipeline\|external.*pressure" "$ADR"
check "ADR explains Phase 31.4 integration" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 11: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 11: Demo Tests ==="

DEMO_FILE="internal/demo_phase38_notification_observer/demo_test.go"

# 11.1 Demo file exists
[ -f "$DEMO_FILE" ]
check "Demo test file exists" $?

if [ -f "$DEMO_FILE" ]; then
    # 11.2 Has sufficient test functions
    TEST_COUNT=$(grep -c "^func Test" "$DEMO_FILE" || true)
    [ "$TEST_COUNT" -ge 18 ]
    check "Has at least 18 test functions (found: $TEST_COUNT)" $?

    # 11.3 Tests enum validation
    grep -q "Validate\|validate" "$DEMO_FILE"
    check "Tests validation" $?

    # 11.4 Tests observation
    grep -q "ObserveNotificationMetadata\|Observe" "$DEMO_FILE"
    check "Tests observation" $?

    # 11.5 Tests bounded retention
    grep -q "Evict\|evict\|Retention\|retention\|MaxRecords\|maxRecords" "$DEMO_FILE"
    check "Tests bounded retention" $?

    # 11.6 Tests hash-only storage
    grep -q "Hash\|hash" "$DEMO_FILE"
    check "Tests hash-only storage" $?

    # 11.7 Tests deduplication (max 1 per app class per period)
    grep -q "Dedup\|dedup\|Merge\|merge\|SignalID" "$DEMO_FILE"
    check "Tests deduplication" $?
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

# 12.3 No external libraries in store
! grep "github.com" "$STORE_FILE"
check "Store uses stdlib only" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}All Phase 38 Notification Observer guardrails passed!${NC}"
    exit 0
else
    echo -e "${RED}$ERRORS guardrail(s) failed${NC}"
    exit 1
fi
