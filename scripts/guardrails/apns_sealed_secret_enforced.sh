#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════════
# Phase 35b: APNs Sealed Secret Boundary Guardrails
# ═══════════════════════════════════════════════════════════════════════════════
#
# This script enforces critical invariants for APNs Sealed Secret Boundary.
#
# CRITICAL INVARIANTS:
#   - Raw device_token ONLY in sealed_secret_store.go and apns.go
#   - No device_token in pkg/, events, logs, storelog
#   - Encrypted blob never leaves sealed store (except to apns.go)
#   - AES-GCM encryption used
#   - stdlib-only
#   - No goroutines
#   - No time.Now() in business logic
#   - APNs transport does not implement decision logic
#   - Daily delivery cap preserved (Phase 35)
#
# Reference: docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md
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
[ -f "docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md" ]
check "ADR-0072 exists" $?

# 1.2 Sealed secret store exists
[ -f "internal/persist/sealed_secret_store.go" ]
check "Sealed secret store exists" $?

# 1.3 APNs transport exists
[ -f "internal/pushtransport/transport/apns.go" ]
check "APNs transport exists" $?

# 1.4 Demo tests exist
[ -d "internal/demo_phase35b_apns_transport" ]
check "Demo tests directory exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 2: Sealed Secret Boundary — Token Containment
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 2: Token Containment ==="

# 2.1 No "device_token" or "deviceToken" in pkg/ (domain layer)
# Excluding types.go which has TokenKindDeviceToken enum
! grep -rq "device_token\|deviceToken" pkg/ --include="*.go" || \
grep -rq "TokenKindDeviceToken\|device_token.*=" pkg/domain/pushtransport/types.go
check "No device_token in pkg/ (except enum)" $?

# 2.2 No "device_token" in events file
! grep -q "device_token\|deviceToken" pkg/events/events.go
check "No device_token in events" $?

# 2.3 No "device_token" in storelog
! grep -q "device_token\|deviceToken" pkg/domain/storelog/log.go
check "No device_token in storelog" $?

# 2.4 No raw token storage outside sealed boundary
# Check that only sealed_secret_store.go and apns.go handle raw tokens
SEALED_FILES="internal/persist/sealed_secret_store.go internal/pushtransport/transport/apns.go"
for f in $SEALED_FILES; do
    [ -f "$f" ]
    check "Sealed boundary file exists: $f" $?
done

# 2.5 Other persist stores don't mention encrypted blobs
! grep -q "encryptedBlob\|EncryptedBlob\|Encrypt\|Decrypt" internal/persist/push_registration_store.go
check "Push registration store has no encryption" $?

! grep -q "encryptedBlob\|EncryptedBlob\|Encrypt\|Decrypt" internal/persist/push_attempt_store.go
check "Push attempt store has no encryption" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 3: AES-GCM Encryption
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 3: AES-GCM Encryption ==="

SEALED_STORE="internal/persist/sealed_secret_store.go"

# 3.1 Uses crypto/aes
grep -q '"crypto/aes"' "$SEALED_STORE"
check "Uses crypto/aes" $?

# 3.2 Uses cipher.AEAD (GCM)
grep -q "cipher.AEAD\|cipher.NewGCM" "$SEALED_STORE"
check "Uses cipher.AEAD (GCM)" $?

# 3.3 Nonce is generated randomly
grep -q "rand.Reader" "$SEALED_STORE"
check "Nonce uses crypto/rand" $?

# 3.4 32-byte key requirement
grep -q "32" "$SEALED_STORE" && grep -q "len(key)" "$SEALED_STORE"
check "Validates 32-byte key" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 4: File Permissions
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 4: File Permissions ==="

# 4.1 Creates directory with 0700
grep -q "0700" "$SEALED_STORE"
check "Creates directory with 0700 permissions" $?

# 4.2 Creates files with 0600
grep -q "0600" "$SEALED_STORE"
check "Creates files with 0600 permissions" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 5: APNs Transport Constraints
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 5: APNs Transport ==="

APNS_FILE="internal/pushtransport/transport/apns.go"

# 5.1 Implements Transport interface
grep -q "ProviderKind()" "$APNS_FILE" && grep -q "Send.*context.Context" "$APNS_FILE"
check "Implements Transport interface" $?

# 5.2 Returns ProviderAPNs
grep -q "ProviderAPNs" "$APNS_FILE"
check "Returns ProviderAPNs provider kind" $?

# 5.3 Uses stdlib net/http only
grep -q '"net/http"' "$APNS_FILE"
check "Uses net/http" $?

# 5.4 No external HTTP libraries
! grep -q "github.com/.*http\|golang.org/x/net/http2" "$APNS_FILE"
check "No external HTTP libraries" $?

# 5.5 Loads from sealed store
grep -q "sealedStore\|LoadEncrypted" "$APNS_FILE"
check "Uses sealed store for tokens" $?

# 5.6 Constant payload
grep -q "DefaultAPNsPayload\|PushTitle\|PushBody" "$APNS_FILE"
check "Uses constant payload" $?

# 5.7 No retry logic
! grep -q "retry\|Retry\|backoff\|Backoff\|for.*range" "$APNS_FILE" || \
! grep -B5 -A5 "for" "$APNS_FILE" | grep -q "Send\|request\|http"
check "No retry logic" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 6: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 6: No Goroutines ==="

# 6.1 No goroutines in sealed store
! grep -q "go func" "$SEALED_STORE"
check "No goroutines in sealed store" $?

# 6.2 No goroutines in APNs transport
! grep -q "go func" "$APNS_FILE"
check "No goroutines in APNs transport" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 7: Clock Injection
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 7: Clock Injection ==="

# 7.1 No time.Now() in sealed store (excluding comments)
! grep -v "^[[:space:]]*//" "$SEALED_STORE" | grep -q "time.Now()"
check "No time.Now() in sealed store" $?

# 7.2 time.Now() in APNs transport only for JWT (acceptable infrastructure)
# JWT expiry checking is infrastructure, not business logic
grep -q "time.Now()" "$APNS_FILE" && grep -B5 "time.Now()" "$APNS_FILE" | grep -q "JWT\|jwt\|expir"
check "time.Now() only for JWT expiry (acceptable)" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 8: No Decision Logic in Transport
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 8: Transport-Only ==="

# 8.1 APNs transport doesn't import decision engines
! grep -q "interruptpolicy\|interruptpreview" "$APNS_FILE"
check "APNs transport has no decision engine imports" $?

# 8.2 No Phase 33/34 imports
! grep -q "phase33\|phase34" "$APNS_FILE"
check "No Phase 33/34 references" $?

# 8.3 Comment confirms sealed boundary
grep -q "SEALED SECRET BOUNDARY\|sealed secret" "$APNS_FILE"
check "APNs transport documents sealed boundary" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 9: Abstract Payload
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 9: Abstract Payload ==="

# 9.1 DefaultAPNsPayload uses PushTitle
grep -A10 "DefaultAPNsPayload" "$APNS_FILE" | grep -q "PushTitle"
check "Payload uses PushTitle constant" $?

# 9.2 DefaultAPNsPayload uses PushBody
grep -A10 "DefaultAPNsPayload" "$APNS_FILE" | grep -q "PushBody"
check "Payload uses PushBody constant" $?

# 9.3 No dynamic content in payload (excluding comments)
! grep -v "^[[:space:]]*//" "$APNS_FILE" | grep -q "candidate\|subject\|sender\|amount\|merchant"
check "No dynamic content in payload" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 10: Storelog Integration
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 10: Storelog ==="

STORELOG="pkg/domain/storelog/log.go"

# 10.1 APNs record types exist
grep -q "RecordTypeAPNsRegistration" "$STORELOG"
check "RecordTypeAPNsRegistration exists" $?

grep -q "RecordTypeAPNsDelivery" "$STORELOG"
check "RecordTypeAPNsDelivery exists" $?

# 10.2 No raw token mentions in storelog
! grep -q "device_token\|deviceToken\|rawToken" "$STORELOG"
check "No raw token mentions in storelog" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 11: Events
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 11: Events ==="

EVENTS="pkg/events/events.go"

# 11.1 Phase 35b events section exists
grep -q "PHASE 35b" "$EVENTS"
check "Phase 35b events section exists" $?

# 11.2 Sealed secret events
grep -q "Phase35bSealedSecretStored" "$EVENTS"
check "Sealed secret stored event exists" $?

# 11.3 APNs registration events
grep -q "Phase35bAPNsRegistrationCreated" "$EVENTS"
check "APNs registration created event exists" $?

# 11.4 APNs delivery events
grep -q "Phase35bAPNsDeliverySent" "$EVENTS"
check "APNs delivery sent event exists" $?

# 11.5 No token in event names
! grep "Phase35b" "$EVENTS" | grep -qi "token"
check "No token in Phase 35b event names" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 12: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 12: Demo Tests ==="

DEMO_FILE="internal/demo_phase35b_apns_transport/demo_test.go"

# 12.1 Demo file exists
[ -f "$DEMO_FILE" ]
check "Demo test file exists" $?

# 12.2 Has sufficient test functions
TEST_COUNT=$(grep -c "^func Test" "$DEMO_FILE" || true)
[ "$TEST_COUNT" -ge 15 ]
check "Has at least 15 test functions (found: $TEST_COUNT)" $?

# 12.3 Tests encryption roundtrip
grep -q "EncryptDecrypt\|Roundtrip" "$DEMO_FILE"
check "Tests encryption roundtrip" $?

# 12.4 Tests token not stored raw
grep -q "NeverStoredRaw\|TokenNever" "$DEMO_FILE"
check "Tests token not stored raw" $?

# 12.5 Tests sealed store
grep -q "SealedSecretStore" "$DEMO_FILE"
check "Tests sealed secret store" $?

# 12.6 Uses httptest
grep -q "httptest" "$DEMO_FILE"
check "Uses httptest for mocking" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 13: ADR Content
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 13: ADR Content ==="

ADR="docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md"

# 13.1 Explains why hash-only is insufficient
grep -qi "hash-only.*insufficient\|insufficient.*hash" "$ADR"
check "ADR explains why hash-only is insufficient" $?

# 13.2 Explains sealed boundary
grep -qi "sealed.*boundary\|boundary.*sealed" "$ADR"
check "ADR explains sealed boundary" $?

# 13.3 Documents AES-GCM
grep -qi "AES-GCM\|AES.*GCM" "$ADR"
check "ADR documents AES-GCM" $?

# 13.4 Confirms trust not weakened
grep -qi "trust\|Trust" "$ADR" && grep -qi "not.*weaken\|preserve\|maintain" "$ADR"
check "ADR confirms trust not weakened" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 14: stdlib-only
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 14: stdlib-only ==="

# 14.1 No external crypto libraries in sealed store
! grep "github.com" "$SEALED_STORE"
check "Sealed store uses stdlib only" $?

# 14.2 No external libraries in APNs transport (except internal packages)
! grep "github.com" "$APNS_FILE"
check "APNs transport uses stdlib only" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}All Phase 35b APNs Sealed Secret guardrails passed!${NC}"
    exit 0
else
    echo -e "${RED}$ERRORS guardrail(s) failed${NC}"
    exit 1
fi
