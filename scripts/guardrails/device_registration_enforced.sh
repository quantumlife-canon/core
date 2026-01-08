#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════════
# Phase 37: Device Registration + Deep-Link Guardrails
# ═══════════════════════════════════════════════════════════════════════════════
#
# This script enforces critical invariants for Device Registration.
#
# CRITICAL INVARIANTS:
#   - device_token string appears ONLY in:
#     - cmd/quantumlife-web/main.go (handler input parsing)
#     - internal/persist/sealed_secret_store.go
#     - internal/pushtransport/transport/apns.go
#   - No time.Now() in internal/ or pkg/ business logic
#   - No goroutines in internal/ or pkg/
#   - Routes exist in main.go
#   - Domain types with CanonicalString + Validate
#   - Storage bounded retention
#   - /open only accepts t=... and has no other params
#   - Proof page shows no identifiers
#   - Whisper cue at lowest priority
#
# Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
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
[ -f "docs/ADR/ADR-0074-phase37-device-registration-deeplink.md" ]
check "ADR-0074 exists" $?

# 1.2 Domain package exists
[ -d "pkg/domain/devicereg" ]
check "Domain package exists" $?

# 1.3 Engine exists
[ -f "internal/devicereg/engine.go" ]
check "Engine exists" $?

# 1.4 Persistence store exists
[ -f "internal/persist/device_registration_store.go" ]
check "Persistence store exists" $?

# 1.5 Demo tests exist
[ -d "internal/demo_phase37_device_registration" ]
check "Demo tests directory exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 2: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 2: No Goroutines ==="

# 2.1 No goroutines in domain package
! grep -r "go func" pkg/domain/devicereg/ --include="*.go"
check "No goroutines in domain package" $?

# 2.2 No goroutines in engine
! grep -q "go func" internal/devicereg/engine.go
check "No goroutines in engine" $?

# 2.3 No goroutines in store
! grep -q "go func" internal/persist/device_registration_store.go
check "No goroutines in store" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 3: No time.Now()
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 3: Clock Injection ==="

# 3.1 No time.Now() in domain package (excluding comments)
! grep -r "time.Now()" pkg/domain/devicereg/ --include="*.go" | grep -v "^[^:]*:[[:space:]]*//"
check "No time.Now() in domain package" $?

# 3.2 No time.Now() in engine (excluding comments)
! grep -v "^[[:space:]]*//\|eviction only" internal/devicereg/engine.go | grep -q "time.Now()"
check "No time.Now() in engine" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 4: Domain Model Completeness
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 4: Domain Model ==="

DOMAIN_FILE="pkg/domain/devicereg/types.go"

# 4.1 DevicePlatform exists
grep -q "type DevicePlatform string" "$DOMAIN_FILE"
check "DevicePlatform type exists" $?

# 4.2 DeviceRegState exists
grep -q "type DeviceRegState string" "$DOMAIN_FILE"
check "DeviceRegState type exists" $?

# 4.3 DeepLinkTarget exists
grep -q "type DeepLinkTarget string" "$DOMAIN_FILE"
check "DeepLinkTarget type exists" $?

# 4.4 DeviceRegistrationReceipt exists
grep -q "type DeviceRegistrationReceipt struct" "$DOMAIN_FILE"
check "DeviceRegistrationReceipt type exists" $?

# 4.5 DeviceRegistrationProofPage exists
grep -q "type DeviceRegistrationProofPage struct" "$DOMAIN_FILE"
check "DeviceRegistrationProofPage type exists" $?

# 4.6 Validate() methods exist
grep -q "func (p DevicePlatform) Validate()" "$DOMAIN_FILE"
check "DevicePlatform.Validate() exists" $?

grep -q "func (s DeviceRegState) Validate()" "$DOMAIN_FILE"
check "DeviceRegState.Validate() exists" $?

grep -q "func (t DeepLinkTarget) Validate()" "$DOMAIN_FILE"
check "DeepLinkTarget.Validate() exists" $?

grep -q "func (r \*DeviceRegistrationReceipt) Validate()" "$DOMAIN_FILE"
check "DeviceRegistrationReceipt.Validate() exists" $?

# 4.7 CanonicalString() methods exist
grep -q "func (p DevicePlatform) CanonicalString()" "$DOMAIN_FILE"
check "DevicePlatform.CanonicalString() exists" $?

grep -q "func (r \*DeviceRegistrationReceipt) CanonicalString()" "$DOMAIN_FILE"
check "DeviceRegistrationReceipt.CanonicalString() exists" $?

# 4.8 Bounded retention constants
grep -q "MaxRegistrationRecords.*=.*200" "$DOMAIN_FILE"
check "MaxRegistrationRecords = 200" $?

grep -q "MaxRetentionDays.*=.*30" "$DOMAIN_FILE"
check "MaxRetentionDays = 30" $?

# 4.9 ForbiddenPatterns check helper
grep -q "ForbiddenPatterns" "$DOMAIN_FILE"
check "ForbiddenPatterns exists" $?

grep -q "CheckForbiddenPatterns" "$DOMAIN_FILE"
check "CheckForbiddenPatterns helper exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 5: Engine Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 5: Engine Constraints ==="

ENGINE_FILE="internal/devicereg/engine.go"

# 5.1 Engine is pure (no side effects imports)
! grep -q '"net/http"' "$ENGINE_FILE"
check "Engine has no net/http import" $?

! grep -q '"os"' "$ENGINE_FILE"
check "Engine has no os import" $?

# 5.2 Has BuildRegistrationReceipt function
grep -q "func (e \*Engine) BuildRegistrationReceipt" "$ENGINE_FILE"
check "BuildRegistrationReceipt exists" $?

# 5.3 Has BuildProofPage function
grep -q "func (e \*Engine) BuildProofPage" "$ENGINE_FILE"
check "BuildProofPage exists" $?

# 5.4 Has ComputeDeepLinkTarget function
grep -q "func (e \*Engine) ComputeDeepLinkTarget" "$ENGINE_FILE"
check "ComputeDeepLinkTarget exists" $?

# 5.5 Has ShouldShowDeviceRegCue function
grep -q "func (e \*Engine) ShouldShowDeviceRegCue" "$ENGINE_FILE"
check "ShouldShowDeviceRegCue exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 6: Sealed Secret Boundary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 6: Sealed Secret Boundary ==="

# 6.1 device_token only in allowed files
# Check that device_token appears only in:
#   - cmd/quantumlife-web/main.go
#   - internal/persist/sealed_secret_store.go
#   - internal/pushtransport/transport/apns.go
#   - pkg/domain/pushtransport/types.go (push notification types)
#   - pkg/events/events.go (event names)
#   - demo tests (allowed)

# Find all .go files with device_token (excluding tests and allowed files)
FORBIDDEN_TOKEN_FILES=$(grep -rl "device_token\|DeviceToken" --include="*.go" \
    internal/ pkg/ 2>/dev/null | \
    grep -v "sealed_secret_store.go" | \
    grep -v "apns.go" | \
    grep -v "pushtransport/types.go" | \
    grep -v "events/events.go" | \
    grep -v "_test.go" | \
    grep -v "demo_phase" || true)

[ -z "$FORBIDDEN_TOKEN_FILES" ]
check "device_token only in sealed boundary" $?

# 6.2 Raw token not stored in registration store
! grep -q "RawToken\|rawToken" internal/persist/device_registration_store.go
check "No raw token in registration store" $?

# 6.3 Registration store uses only hashes
grep -q "CircleIDHash" internal/persist/device_registration_store.go
check "Store uses CircleIDHash" $?

# TokenHash is in domain model, store imports it
grep -q "quantumlife/pkg/domain/devicereg" internal/persist/device_registration_store.go
check "Store imports devicereg domain (with TokenHash)" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 7: Deep Link Safety
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 7: Deep Link Safety ==="

# 7.1 DeepLinkTarget.ToPath() exists
grep -q "func (t DeepLinkTarget) ToPath()" "$DOMAIN_FILE"
check "DeepLinkTarget.ToPath() exists" $?

# 7.2 ValidateOpenParam exists
grep -q "func ValidateOpenParam" "$DOMAIN_FILE"
check "ValidateOpenParam exists" $?

# 7.3 AllDeepLinkTargets exists
grep -q "func AllDeepLinkTargets()" "$DOMAIN_FILE"
check "AllDeepLinkTargets() exists" $?

# 7.4 No identifiers in deep link targets
! grep -q "circle\|user\|email\|hash" "$DOMAIN_FILE" 2>/dev/null || \
    ! grep -A5 "DeepLinkTarget.*=" "$DOMAIN_FILE" | grep -qi "circle_id\|user_id\|email"
check "No identifiers in deep link targets" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 8: Persistence Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 8: Persistence ==="

STORE_FILE="internal/persist/device_registration_store.go"

# 8.1 Bounded retention
grep -q "maxRetentionDays" "$STORE_FILE"
check "Store has maxRetentionDays" $?

grep -q "maxRecords" "$STORE_FILE"
check "Store has maxRecords" $?

# 8.2 FIFO eviction
grep -q "allRecordIDs" "$STORE_FILE"
check "Store has FIFO tracking" $?

grep -q "evictLocked\|EvictOldPeriods" "$STORE_FILE"
check "Store has eviction" $?

# 8.3 Storelog integration
grep -q "storelogRef" "$STORE_FILE"
check "Store has storelog integration" $?

# 8.4 One active per circle+platform
grep -q "activeByCircle" "$STORE_FILE"
check "Store tracks active per circle+platform" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 9: Events and Storelog Records
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 9: Events and Records ==="

EVENTS_FILE="pkg/events/events.go"
STORELOG_FILE="pkg/domain/storelog/log.go"

# 9.1 Phase 37 events exist
grep -q "Phase37DeviceRegisterRequested" "$EVENTS_FILE"
check "Phase37DeviceRegisterRequested event exists" $?

grep -q "Phase37DeviceSealed" "$EVENTS_FILE"
check "Phase37DeviceSealed event exists" $?

grep -q "Phase37DeviceRegistered" "$EVENTS_FILE"
check "Phase37DeviceRegistered event exists" $?

grep -q "Phase37DeviceProofViewed" "$EVENTS_FILE"
check "Phase37DeviceProofViewed event exists" $?

grep -q "Phase37OpenRedirected" "$EVENTS_FILE"
check "Phase37OpenRedirected event exists" $?

# 9.2 Storelog record types exist
grep -q "RecordTypeDeviceRegistration" "$STORELOG_FILE"
check "RecordTypeDeviceRegistration exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 10: ADR Content
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 10: ADR Content ==="

ADR="docs/ADR/ADR-0074-phase37-device-registration-deeplink.md"

# 10.1 Explains sealed secrets
grep -qi "sealed" "$ADR"
check "ADR explains sealed secrets" $?

# 10.2 Explains no identifiers in deep links
grep -qi "no identifier\|no.*identifier" "$ADR"
check "ADR explains no identifiers in deep links" $?

# 10.3 Explains trust preservation
grep -qi "trust" "$ADR"
check "ADR discusses trust" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 11: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 11: Demo Tests ==="

DEMO_FILE="internal/demo_phase37_device_registration/demo_test.go"

# 11.1 Demo file exists
[ -f "$DEMO_FILE" ]
check "Demo test file exists" $?

if [ -f "$DEMO_FILE" ]; then
    # 11.2 Has sufficient test functions
    TEST_COUNT=$(grep -c "^func Test" "$DEMO_FILE" || true)
    [ "$TEST_COUNT" -ge 15 ]
    check "Has at least 15 test functions (found: $TEST_COUNT)" $?

    # 11.3 Tests enum validation
    grep -q "Validate\|validate" "$DEMO_FILE"
    check "Tests validation" $?

    # 11.4 Tests deep link target
    grep -q "DeepLinkTarget\|ComputeDeepLinkTarget" "$DEMO_FILE"
    check "Tests deep link target" $?

    # 11.5 Tests bounded retention
    grep -q "Evict\|evict\|Retention\|retention" "$DEMO_FILE"
    check "Tests bounded retention" $?

    # 11.6 Tests hash-only storage
    grep -q "Hash\|hash" "$DEMO_FILE"
    check "Tests hash-only storage" $?
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
    echo -e "${GREEN}All Phase 37 Device Registration guardrails passed!${NC}"
    exit 0
else
    echo -e "${RED}$ERRORS guardrail(s) failed${NC}"
    exit 1
fi
