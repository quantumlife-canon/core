#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════════
# Phase 35: Push Transport Guardrails
# ═══════════════════════════════════════════════════════════════════════════════
#
# This script enforces critical invariants for Push Transport (Phase 35).
#
# CRITICAL INVARIANTS:
#   - Transport-only. No new decision logic.
#   - Abstract payload only. No identifiers in push body.
#   - TokenHash only. Raw token NEVER stored.
#   - No goroutines. Synchronous delivery only.
#   - Commerce never interrupts.
#   - Daily cap: max 2 deliveries.
#   - stdlib-only for transport implementations.
#
# Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
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

# 1.1 Domain types exist
[ -f "pkg/domain/pushtransport/types.go" ]
check "Domain types file exists" $?

# 1.2 Engine exists
[ -f "internal/pushtransport/engine.go" ]
check "Engine file exists" $?

# 1.3 Transport interface exists
[ -f "internal/pushtransport/transport/interface.go" ]
check "Transport interface exists" $?

# 1.4 Stub transport exists
[ -f "internal/pushtransport/transport/stub.go" ]
check "Stub transport exists" $?

# 1.5 Webhook transport exists
[ -f "internal/pushtransport/transport/webhook.go" ]
check "Webhook transport exists" $?

# 1.6 Registration store exists
[ -f "internal/persist/push_registration_store.go" ]
check "Registration store exists" $?

# 1.7 Attempt store exists
[ -f "internal/persist/push_attempt_store.go" ]
check "Attempt store exists" $?

# 1.8 Demo tests exist
[ -d "internal/demo_phase35_push_transport" ]
check "Demo tests directory exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 2: Domain Model Completeness
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 2: Domain Model Completeness ==="

DOMAIN_FILE="pkg/domain/pushtransport/types.go"

# 2.1 PushProviderKind enum exists
grep -q "type PushProviderKind string" "$DOMAIN_FILE"
check "PushProviderKind type exists" $?

# 2.2 Provider constants exist
grep -q 'ProviderAPNs.*=.*"apns"' "$DOMAIN_FILE"
check "ProviderAPNs constant exists" $?

grep -q 'ProviderWebhook.*=.*"webhook"' "$DOMAIN_FILE"
check "ProviderWebhook constant exists" $?

grep -q 'ProviderStub.*=.*"stub"' "$DOMAIN_FILE"
check "ProviderStub constant exists" $?

# 2.3 PushTokenKind enum exists
grep -q "type PushTokenKind string" "$DOMAIN_FILE"
check "PushTokenKind type exists" $?

# 2.4 AttemptStatus enum exists
grep -q "type AttemptStatus string" "$DOMAIN_FILE"
check "AttemptStatus type exists" $?

# 2.5 FailureBucket enum exists
grep -q "type FailureBucket string" "$DOMAIN_FILE"
check "FailureBucket type exists" $?

# 2.6 PushRegistration struct exists
grep -q "type PushRegistration struct" "$DOMAIN_FILE"
check "PushRegistration struct exists" $?

# 2.7 PushDeliveryAttempt struct exists
grep -q "type PushDeliveryAttempt struct" "$DOMAIN_FILE"
check "PushDeliveryAttempt struct exists" $?

# 2.8 TransportRequest struct exists
grep -q "type TransportRequest struct" "$DOMAIN_FILE"
check "TransportRequest struct exists" $?

# 2.9 TransportPayload struct exists
grep -q "type TransportPayload struct" "$DOMAIN_FILE"
check "TransportPayload struct exists" $?

# 2.10 TransportResult struct exists
grep -q "type TransportResult struct" "$DOMAIN_FILE"
check "TransportResult struct exists" $?

# 2.11 DeliveryEligibilityInput struct exists
grep -q "type DeliveryEligibilityInput struct" "$DOMAIN_FILE"
check "DeliveryEligibilityInput struct exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 3: Abstract Payload Only
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 3: Abstract Payload Only ==="

# 3.1 PushTitle constant exists
grep -q 'PushTitle.*=.*"QuantumLife"' "$DOMAIN_FILE"
check "PushTitle constant is 'QuantumLife'" $?

# 3.2 PushBody constant exists
grep -q 'PushBody.*=.*"Something needs you. Open QuantumLife."' "$DOMAIN_FILE"
check "PushBody constant is abstract" $?

# 3.3 DefaultTransportPayload uses constants
grep -q "DefaultTransportPayload" "$DOMAIN_FILE" && \
grep -A5 "DefaultTransportPayload" "$DOMAIN_FILE" | grep -q "PushTitle"
check "DefaultTransportPayload uses PushTitle constant" $?

grep -q "DefaultTransportPayload" "$DOMAIN_FILE" && \
grep -A5 "DefaultTransportPayload" "$DOMAIN_FILE" | grep -q "PushBody"
check "DefaultTransportPayload uses PushBody constant" $?

# 3.4 No dynamic title/body in transport
! grep -rq "payload.Title.*=" internal/pushtransport/transport/*.go || \
grep -rq "payload.Title.*=.*PushTitle" internal/pushtransport/transport/*.go || \
grep -rq 'Title:.*req\.Payload\.Title' internal/pushtransport/transport/*.go
check "Transport uses constant title" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 4: Token Security
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 4: Token Security ==="

# 4.1 HashToken function exists
grep -q "func HashToken" "$DOMAIN_FILE"
check "HashToken function exists" $?

# 4.2 TokenHash field in PushRegistration (not RawToken)
grep -A20 "type PushRegistration struct" "$DOMAIN_FILE" | grep -q "TokenHash"
check "PushRegistration has TokenHash field" $?

# 4.3 RawToken in TransportRequest but NOT persisted
grep -A20 "type TransportRequest struct" "$DOMAIN_FILE" | grep -q "RawToken"
check "TransportRequest has RawToken for delivery" $?

# 4.4 Stores don't store RawToken
! grep -q "RawToken" internal/persist/push_registration_store.go
check "Registration store does not mention RawToken" $?

! grep -q "RawToken" internal/persist/push_attempt_store.go
check "Attempt store does not mention RawToken" $?

# 4.5 Canonical strings don't include raw token
grep -A10 "func.*PushRegistration.*CanonicalString" "$DOMAIN_FILE" | grep -q "TokenHash" && \
! grep -A10 "func.*PushRegistration.*CanonicalString" "$DOMAIN_FILE" | grep -q "RawToken"
check "CanonicalString uses TokenHash not RawToken" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 5: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 5: No Goroutines ==="

# 5.1 No goroutines in domain types
! grep -q "go func" "$DOMAIN_FILE"
check "No goroutines in domain types" $?

# 5.2 No goroutines in engine
! grep -q "go func" internal/pushtransport/engine.go
check "No goroutines in engine" $?

# 5.3 No goroutines in transport
! grep -rq "go func" internal/pushtransport/transport/
check "No goroutines in transport" $?

# 5.4 No goroutines in stores
! grep -q "go func" internal/persist/push_registration_store.go
check "No goroutines in registration store" $?

! grep -q "go func" internal/persist/push_attempt_store.go
check "No goroutines in attempt store" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 6: No time.Now() in Business Logic
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 6: Clock Injection ==="

# 6.1 No time.Now() in domain types (excluding comments)
! grep -v "^[[:space:]]*//" "$DOMAIN_FILE" | grep -q "time.Now()"
check "No time.Now() in domain types" $?

# 6.2 No time.Now() in engine (excluding comments)
! grep -v "^[[:space:]]*//" internal/pushtransport/engine.go | grep -q "time.Now()"
check "No time.Now() in engine" $?

# 6.3 No time.Now() in transport (except timeout which is acceptable)
# Allow time.Duration and http.Client timeout
! grep -v "Timeout" internal/pushtransport/transport/*.go | grep -q "time.Now()"
check "No time.Now() in transport (except timeout)" $?

# 6.4 time.Now() only in eviction (acceptable)
grep -q "time.Now()" internal/persist/push_registration_store.go && \
grep -B5 "time.Now()" internal/persist/push_registration_store.go | grep -q "evict\|Evict"
check "time.Now() only in eviction (registration store)" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 7: stdlib-only in Transport
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 7: stdlib-only in Transport ==="

# 7.1 Stub transport uses only stdlib
! grep "github.com" internal/pushtransport/transport/stub.go
check "Stub transport uses only stdlib" $?

# 7.2 Webhook transport uses only stdlib
! grep "github.com" internal/pushtransport/transport/webhook.go
check "Webhook transport uses only stdlib" $?

# 7.3 Webhook uses net/http
grep -q '"net/http"' internal/pushtransport/transport/webhook.go
check "Webhook transport uses net/http" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 8: Daily Cap Enforcement
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 8: Daily Cap Enforcement ==="

# 8.1 DefaultMaxPushPerDay constant exists
grep -q "DefaultMaxPushPerDay" "$DOMAIN_FILE"
check "DefaultMaxPushPerDay constant exists" $?

# 8.2 DefaultMaxPushPerDay is 2
grep -q "DefaultMaxPushPerDay.*=.*2" "$DOMAIN_FILE"
check "DefaultMaxPushPerDay is 2" $?

# 8.3 Engine checks daily cap
grep -q "DailyAttemptCount" internal/pushtransport/engine.go && \
grep -q "MaxPerDay" internal/pushtransport/engine.go
check "Engine checks daily cap" $?

# 8.4 FailureCapReached exists
grep -q "FailureCapReached" "$DOMAIN_FILE"
check "FailureCapReached failure bucket exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 9: Deduplication
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 9: Deduplication ==="

# 9.1 ComputeAttemptID exists
grep -q "func.*ComputeAttemptID" "$DOMAIN_FILE"
check "ComputeAttemptID function exists" $?

# 9.2 Dedup key includes circle+candidate+period
grep -A10 "ComputeAttemptID" "$DOMAIN_FILE" | grep -q "CircleIDHash" && \
grep -A10 "ComputeAttemptID" "$DOMAIN_FILE" | grep -q "CandidateHash" && \
grep -A10 "ComputeAttemptID" "$DOMAIN_FILE" | grep -q "PeriodKey"
check "Dedup key includes circle+candidate+period" $?

# 9.3 Store rejects duplicate attempt ID
grep -q "duplicate attempt" internal/persist/push_attempt_store.go
check "Attempt store rejects duplicates" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 10: Bounded Retention
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 10: Bounded Retention ==="

# 10.1 maxRetentionDays in registration store
grep -q "maxRetentionDays" internal/persist/push_registration_store.go
check "Registration store has maxRetentionDays" $?

# 10.2 maxRetentionDays in attempt store
grep -q "maxRetentionDays" internal/persist/push_attempt_store.go
check "Attempt store has maxRetentionDays" $?

# 10.3 Default is 30 days
grep -q "MaxRetentionDays:.*30" internal/persist/push_registration_store.go
check "Registration store default is 30 days" $?

grep -q "MaxRetentionDays:.*30" internal/persist/push_attempt_store.go
check "Attempt store default is 30 days" $?

# 10.4 Eviction functions exist
grep -q "evictOldPeriodsLocked" internal/persist/push_registration_store.go
check "Registration store has eviction function" $?

grep -q "evictOldPeriodsLocked" internal/persist/push_attempt_store.go
check "Attempt store has eviction function" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 11: Storelog Integration
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 11: Storelog Integration ==="

# 11.1 RecordTypePushRegistration exists
grep -q "RecordTypePushRegistration" pkg/domain/storelog/log.go
check "RecordTypePushRegistration exists in storelog" $?

# 11.2 RecordTypePushAttempt exists
grep -q "RecordTypePushAttempt" pkg/domain/storelog/log.go
check "RecordTypePushAttempt exists in storelog" $?

# 11.3 Registration store uses storelog
grep -q "storelogRef" internal/persist/push_registration_store.go
check "Registration store has storelog reference" $?

# 11.4 Attempt store uses storelog
grep -q "storelogRef" internal/persist/push_attempt_store.go
check "Attempt store has storelog reference" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 12: Events
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 12: Events ==="

EVENTS_FILE="pkg/events/events.go"

# 12.1 Phase 35 events section exists
grep -q "PHASE 35" "$EVENTS_FILE"
check "Phase 35 events section exists" $?

# 12.2 Registration events exist
grep -q "Phase35PushRegistrationCreated" "$EVENTS_FILE"
check "Phase35PushRegistrationCreated event exists" $?

# 12.3 Delivery events exist
grep -q "Phase35PushDeliveryEligible" "$EVENTS_FILE"
check "Phase35PushDeliveryEligible event exists" $?

grep -q "Phase35PushDeliverySkipped" "$EVENTS_FILE"
check "Phase35PushDeliverySkipped event exists" $?

# 12.4 Transport events exist
grep -q "Phase35PushTransportSent" "$EVENTS_FILE"
check "Phase35PushTransportSent event exists" $?

grep -q "Phase35PushTransportFailed" "$EVENTS_FILE"
check "Phase35PushTransportFailed event exists" $?

# 12.5 Proof events exist
grep -q "Phase35PushProofRequested" "$EVENTS_FILE"
check "Phase35PushProofRequested event exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 13: Transport Interface
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 13: Transport Interface ==="

INTERFACE_FILE="internal/pushtransport/transport/interface.go"

# 13.1 Transport interface exists
grep -q "type Transport interface" "$INTERFACE_FILE"
check "Transport interface exists" $?

# 13.2 ProviderKind method exists
grep -q "ProviderKind()" "$INTERFACE_FILE"
check "ProviderKind() method exists" $?

# 13.3 Send method exists
grep -q "Send.*context.Context" "$INTERFACE_FILE"
check "Send() method exists" $?

# 13.4 Registry exists
grep -q "type Registry struct" "$INTERFACE_FILE"
check "Registry struct exists" $?

# 13.5 Register and Get methods exist
grep -q "func.*Register" "$INTERFACE_FILE"
check "Register() method exists" $?

grep -q "func.*Get" "$INTERFACE_FILE"
check "Get() method exists" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 14: Canonical Strings
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 14: Canonical Strings ==="

# 14.1 PushRegistration has CanonicalString
grep -q "func.*PushRegistration.*CanonicalString" "$DOMAIN_FILE"
check "PushRegistration has CanonicalString" $?

# 14.2 PushDeliveryAttempt has CanonicalString
grep -q "func.*PushDeliveryAttempt.*CanonicalString" "$DOMAIN_FILE"
check "PushDeliveryAttempt has CanonicalString" $?

# 14.3 Pipe-delimited format with version
grep -A5 "func.*PushRegistration.*CanonicalString" "$DOMAIN_FILE" | grep -q "PUSH_REG|v1"
check "Registration canonical string is versioned" $?

grep -A5 "func.*PushDeliveryAttempt.*CanonicalString" "$DOMAIN_FILE" | grep -q "PUSH_ATTEMPT|v1"
check "Attempt canonical string is versioned" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 15: No Decision Logic (Transport-Only)
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 15: Transport-Only ==="

ENGINE_FILE="internal/pushtransport/engine.go"

# 15.1 Engine does NOT perform network calls
! grep -q "http.Do\|http.Get\|http.Post" "$ENGINE_FILE"
check "Engine does not perform network calls" $?

# 15.2 Engine returns TransportRequest (for cmd/ to execute)
grep -q "TransportRequest" "$ENGINE_FILE"
check "Engine returns TransportRequest" $?

# 15.3 Engine comment says transport-only
grep -q "Does NOT perform network calls" "$ENGINE_FILE"
check "Engine comment confirms no network calls" $?

# 15.4 No imports of external decision engines
! grep -q "interruptpolicy\|interruptpreview" "$ENGINE_FILE"
check "Engine does not import decision engines" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 16: ADR Exists
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 16: ADR Exists ==="

ADR_FILE="docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md"

# 16.1 ADR file exists
[ -f "$ADR_FILE" ]
check "ADR-0071 exists" $?

# 16.2 ADR mentions abstract payload
grep -qi "abstract.*payload\|payload.*abstract" "$ADR_FILE"
check "ADR mentions abstract payload" $?

# 16.3 ADR mentions transport-only
grep -qi "transport-only\|transport only" "$ADR_FILE"
check "ADR mentions transport-only" $?

# 16.4 ADR mentions token hashing
grep -qi "TokenHash\|token.*hash" "$ADR_FILE"
check "ADR mentions token hashing" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Section 17: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "=== Section 17: Demo Tests ==="

DEMO_FILE="internal/demo_phase35_push_transport/demo_test.go"

# 17.1 Demo test file exists
[ -f "$DEMO_FILE" ]
check "Demo test file exists" $?

# 17.2 Has at least 10 test functions
TEST_COUNT=$(grep -c "^func Test" "$DEMO_FILE" || true)
[ "$TEST_COUNT" -ge 10 ]
check "Has at least 10 test functions (found: $TEST_COUNT)" $?

# 17.3 Tests token hashing
grep -q "TestTokenHashing\|TokenHash" "$DEMO_FILE"
check "Demo tests cover token hashing" $?

# 17.4 Tests deduplication
grep -q "Dedup\|dedup" "$DEMO_FILE"
check "Demo tests cover deduplication" $?

# 17.5 Tests abstract payload
grep -q "Abstract\|abstract" "$DEMO_FILE"
check "Demo tests cover abstract payload" $?

# 17.6 Tests eligibility
grep -q "Eligible\|Skipped" "$DEMO_FILE"
check "Demo tests cover eligibility" $?

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}All Phase 35 Push Transport guardrails passed!${NC}"
    exit 0
else
    echo -e "${RED}$ERRORS guardrail(s) failed${NC}"
    exit 1
fi
