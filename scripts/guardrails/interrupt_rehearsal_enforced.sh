#!/usr/bin/env bash
# Phase 41: Live Interrupt Loop (APNs) - Guardrail Enforcement Script
#
# CRITICAL INVARIANTS:
#   - NO goroutines in internal/ or pkg/
#   - NO time.Now() in internal/ or pkg/
#   - Abstract payload only. No identifiers.
#   - Device token secrecy: raw token only in sealed boundary.
#   - Delivery cap: max 2/day per circle.
#   - POST-triggered only.
#   - Single whisper rule: no new whispers on /today.
#
# Reference: docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

PASS=0
FAIL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

check() {
    local name="$1"
    local cmd="$2"

    if eval "$cmd" >/dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $name"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} $name"
        FAIL=$((FAIL + 1))
    fi
}

echo "Phase 41: Live Interrupt Loop (APNs) Guardrails"
echo "================================================"
echo ""

# ============================================================================
# Section 1: File Existence
# ============================================================================
echo "--- Section 1: File Existence ---"

check "Domain types file exists" "test -f pkg/domain/interruptrehearsal/types.go"
check "Engine file exists" "test -f internal/interruptrehearsal/engine.go"
check "Store file exists" "test -f internal/persist/interrupt_rehearsal_store.go"
check "ADR exists" "test -f docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md"
check "Demo tests exist" "test -f internal/demo_phase41_interrupt_rehearsal/demo_test.go"
check "Guardrails script exists" "test -f scripts/guardrails/interrupt_rehearsal_enforced.sh"

# ============================================================================
# Section 2: Package Headers
# ============================================================================
echo ""
echo "--- Section 2: Package Headers ---"

check "Domain types has package declaration" "grep -q 'package interruptrehearsal' pkg/domain/interruptrehearsal/types.go"
check "Engine has package declaration" "grep -q 'package interruptrehearsal' internal/interruptrehearsal/engine.go"
check "Store has package declaration" "grep -q 'package persist' internal/persist/interrupt_rehearsal_store.go"
check "Demo tests have package declaration" "grep -q 'package demo_phase41_interrupt_rehearsal' internal/demo_phase41_interrupt_rehearsal/demo_test.go"

# ============================================================================
# Section 3: No Goroutines
# ============================================================================
echo ""
echo "--- Section 3: No Goroutines ---"

check "No goroutines in domain types" "! grep -qE 'go func|go [a-zA-Z]+\(' pkg/domain/interruptrehearsal/types.go"
check "No goroutines in engine" "! grep -qE 'go func|go [a-zA-Z]+\(' internal/interruptrehearsal/engine.go"
check "No goroutines in store" "! grep -qE 'go func|go [a-zA-Z]+\(' internal/persist/interrupt_rehearsal_store.go"

# ============================================================================
# Section 4: Clock Injection
# ============================================================================
echo ""
echo "--- Section 4: Clock Injection ---"

check "No time.Now() in domain types (excluding comments)" "! grep -v '^[[:space:]]*//' pkg/domain/interruptrehearsal/types.go | grep -q 'time\.Now()'"
check "No time.Now() in engine (excluding comments)" "! grep -v '^[[:space:]]*//' internal/interruptrehearsal/engine.go | grep -q 'time\.Now()'"
check "No time.Now() in store (excluding comments)" "! grep -v '^[[:space:]]*//' internal/persist/interrupt_rehearsal_store.go | grep -q 'time\.Now()'"

# ============================================================================
# Section 5: Forbidden Patterns (No Identifiers)
# ============================================================================
echo ""
echo "--- Section 5: Forbidden Patterns ---"

# No email patterns
check "No email patterns in domain types" "! grep -qE '@[a-zA-Z0-9]+\.[a-z]{2,}' pkg/domain/interruptrehearsal/types.go"
check "No email patterns in engine" "! grep -qE '@[a-zA-Z0-9]+\.[a-z]{2,}' internal/interruptrehearsal/engine.go"
check "No email patterns in store" "! grep -qE '@[a-zA-Z0-9]+\.[a-z]{2,}' internal/persist/interrupt_rehearsal_store.go"

# No URL patterns (except in comments/docs)
check "No http:// URLs in domain types" "! grep -v '//' pkg/domain/interruptrehearsal/types.go | grep -qE 'http://[a-zA-Z]'"
check "No http:// URLs in engine" "! grep -v '//' internal/interruptrehearsal/engine.go | grep -qE 'http://[a-zA-Z]'"

# No currency symbols with amounts (actual $ followed by digits, or literal £/€)
check "No currency amounts in domain types" "! grep -qE '\\\$[0-9]+|£[0-9]+|€[0-9]+' pkg/domain/interruptrehearsal/types.go"
check "No currency amounts in engine" "! grep -qE '\\\$[0-9]+|£[0-9]+|€[0-9]+' internal/interruptrehearsal/engine.go"

# No merchant keywords
check "No merchant keywords in domain types" "! grep -qiE '(uber|deliveroo|amazon|paypal|invoice|receipt|merchant)' pkg/domain/interruptrehearsal/types.go || grep -q 'forbidden' pkg/domain/interruptrehearsal/types.go"
check "No subject/sender keywords" "! grep -qiE '(subject_line|sender_name|sender_email|email_body)' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 6: Abstract Payload Constants
# ============================================================================
echo ""
echo "--- Section 6: Abstract Payload Constants ---"

check "PushTitle constant exists" "grep -q 'PushTitle.*=.*\"QuantumLife\"' pkg/domain/interruptrehearsal/types.go"
check "PushBody constant exists" "grep -q 'PushBody.*=.*\"Something needs you' pkg/domain/interruptrehearsal/types.go"
check "DeepLinkTarget constant exists" "grep -q 'DeepLinkTarget.*=.*\"interrupts\"' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 7: Rehearsal Status Enum
# ============================================================================
echo ""
echo "--- Section 7: Rehearsal Status Enum ---"

check "StatusRequested exists" "grep -q 'StatusRequested.*RehearsalStatus.*=.*\"status_requested\"' pkg/domain/interruptrehearsal/types.go"
check "StatusRejected exists" "grep -q 'StatusRejected.*RehearsalStatus.*=.*\"status_rejected\"' pkg/domain/interruptrehearsal/types.go"
check "StatusAttempted exists" "grep -q 'StatusAttempted.*RehearsalStatus.*=.*\"status_attempted\"' pkg/domain/interruptrehearsal/types.go"
check "StatusDelivered exists" "grep -q 'StatusDelivered.*RehearsalStatus.*=.*\"status_delivered\"' pkg/domain/interruptrehearsal/types.go"
check "StatusFailed exists" "grep -q 'StatusFailed.*RehearsalStatus.*=.*\"status_failed\"' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 8: Reject Reason Enum
# ============================================================================
echo ""
echo "--- Section 8: Reject Reason Enum ---"

check "RejectNoDevice exists" "grep -q 'RejectNoDevice' pkg/domain/interruptrehearsal/types.go"
check "RejectPolicyDisallows exists" "grep -q 'RejectPolicyDisallows' pkg/domain/interruptrehearsal/types.go"
check "RejectNoCandidate exists" "grep -q 'RejectNoCandidate' pkg/domain/interruptrehearsal/types.go"
check "RejectRateLimited exists" "grep -q 'RejectRateLimited' pkg/domain/interruptrehearsal/types.go"
check "RejectTransportUnavailable exists" "grep -q 'RejectTransportUnavailable' pkg/domain/interruptrehearsal/types.go"
check "RejectSealedKeyMissing exists" "grep -q 'RejectSealedKeyMissing' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 9: Transport Kind Enum
# ============================================================================
echo ""
echo "--- Section 9: Transport Kind Enum ---"

check "TransportAPNs exists" "grep -q 'TransportAPNs' pkg/domain/interruptrehearsal/types.go"
check "TransportWebhook exists" "grep -q 'TransportWebhook' pkg/domain/interruptrehearsal/types.go"
check "TransportStub exists" "grep -q 'TransportStub' pkg/domain/interruptrehearsal/types.go"
check "TransportNone exists" "grep -q 'TransportNone' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 10: Delivery Cap
# ============================================================================
echo ""
echo "--- Section 10: Delivery Cap ---"

check "MaxDeliveriesPerDay constant exists" "grep -q 'MaxDeliveriesPerDay.*=.*2' pkg/domain/interruptrehearsal/types.go"
check "Cap enforcement in store" "grep -q 'MaxDeliveriesPerDay' internal/persist/interrupt_rehearsal_store.go"

# ============================================================================
# Section 11: Retention Bounds
# ============================================================================
echo ""
echo "--- Section 11: Retention Bounds ---"

check "MaxRetentionDays constant exists" "grep -q 'MaxRetentionDays.*=.*30' pkg/domain/interruptrehearsal/types.go"
check "MaxRecords constant exists" "grep -q 'MaxRecords.*=.*500' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 12: Canonical String Methods
# ============================================================================
echo ""
echo "--- Section 12: Canonical String Methods ---"

check "RehearsalReceipt has CanonicalString" "grep -q 'func.*RehearsalReceipt.*CanonicalString' pkg/domain/interruptrehearsal/types.go"
check "RehearsalPlan has CanonicalString" "grep -q 'func.*RehearsalPlan.*CanonicalString' pkg/domain/interruptrehearsal/types.go"
check "RehearsalInputs has CanonicalString" "grep -q 'func.*RehearsalInputs.*CanonicalString' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 13: Hash Computation
# ============================================================================
echo ""
echo "--- Section 13: Hash Computation ---"

check "ComputeStatusHash exists" "grep -q 'ComputeStatusHash' pkg/domain/interruptrehearsal/types.go"
check "ComputeAttemptIDHash exists" "grep -q 'ComputeAttemptIDHash' pkg/domain/interruptrehearsal/types.go"
check "SHA256 used for hashing" "grep -q 'sha256' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 14: Engine Interface Methods
# ============================================================================
echo ""
echo "--- Section 14: Engine Interface Methods ---"

check "CandidateSource interface exists" "grep -q 'type CandidateSource interface' internal/interruptrehearsal/engine.go"
check "PolicySource interface exists" "grep -q 'type PolicySource interface' internal/interruptrehearsal/engine.go"
check "DeviceSource interface exists" "grep -q 'type DeviceSource interface' internal/interruptrehearsal/engine.go"
check "RateLimitSource interface exists" "grep -q 'type RateLimitSource interface' internal/interruptrehearsal/engine.go"
check "SealedStatusSource interface exists" "grep -q 'type SealedStatusSource interface' internal/interruptrehearsal/engine.go"
check "EnvelopeSource interface exists" "grep -q 'type EnvelopeSource interface' internal/interruptrehearsal/engine.go"

# ============================================================================
# Section 15: Engine Methods
# ============================================================================
echo ""
echo "--- Section 15: Engine Methods ---"

check "EvaluateEligibility exists" "grep -q 'func.*Engine.*EvaluateEligibility' internal/interruptrehearsal/engine.go"
check "BuildPlan exists" "grep -q 'func.*Engine.*BuildPlan' internal/interruptrehearsal/engine.go"
check "FinalizeAfterAttempt exists" "grep -q 'func.*Engine.*FinalizeAfterAttempt' internal/interruptrehearsal/engine.go"
check "BuildProofPage exists" "grep -q 'func.*Engine.*BuildProofPage' internal/interruptrehearsal/engine.go"
check "BuildRehearsePage exists" "grep -q 'func.*Engine.*BuildRehearsePage' internal/interruptrehearsal/engine.go"

# ============================================================================
# Section 16: Store Methods
# ============================================================================
echo ""
echo "--- Section 16: Store Methods ---"

check "AppendReceipt exists" "grep -q 'func.*InterruptRehearsalStore.*AppendReceipt' internal/persist/interrupt_rehearsal_store.go"
check "GetLatestByCircleAndPeriod exists" "grep -q 'func.*InterruptRehearsalStore.*GetLatestByCircleAndPeriod' internal/persist/interrupt_rehearsal_store.go"
check "CanDeliver exists" "grep -q 'func.*InterruptRehearsalStore.*CanDeliver' internal/persist/interrupt_rehearsal_store.go"
check "GetDailyDeliveryCount exists" "grep -q 'func.*InterruptRehearsalStore.*GetDailyDeliveryCount' internal/persist/interrupt_rehearsal_store.go"

# ============================================================================
# Section 17: Web Routes
# ============================================================================
echo ""
echo "--- Section 17: Web Routes ---"

check "Rehearse GET route exists" "grep -q '/interrupts/rehearse.*handleRehearse' cmd/quantumlife-web/main.go"
check "Rehearse send POST route exists" "grep -q '/interrupts/rehearse/send.*handleRehearseSend' cmd/quantumlife-web/main.go"
check "Rehearse proof GET route exists" "grep -q '/proof/interrupts/rehearse.*handleRehearseProof' cmd/quantumlife-web/main.go"
check "Rehearse dismiss POST route exists" "grep -q '/proof/interrupts/rehearse/dismiss.*handleRehearseProofDismiss' cmd/quantumlife-web/main.go"

# ============================================================================
# Section 18: Handler POST Enforcement
# ============================================================================
echo ""
echo "--- Section 18: Handler POST Enforcement ---"

check "handleRehearseSend checks POST method" "grep -A5 'func.*handleRehearseSend' cmd/quantumlife-web/main.go | grep -q 'MethodPost'"
check "handleRehearseProofDismiss checks POST method" "grep -A5 'func.*handleRehearseProofDismiss' cmd/quantumlife-web/main.go | grep -q 'MethodPost'"

# ============================================================================
# Section 19: Events
# ============================================================================
echo ""
echo "--- Section 19: Events ---"

check "Phase41RehearsalRequested event exists" "grep -q 'Phase41RehearsalRequested' pkg/events/events.go"
check "Phase41RehearsalEligibilityComputed event exists" "grep -q 'Phase41RehearsalEligibilityComputed' pkg/events/events.go"
check "Phase41RehearsalRejected event exists" "grep -q 'Phase41RehearsalRejected' pkg/events/events.go"
check "Phase41RehearsalPlanBuilt event exists" "grep -q 'Phase41RehearsalPlanBuilt' pkg/events/events.go"
check "Phase41RehearsalDeliveryAttempted event exists" "grep -q 'Phase41RehearsalDeliveryAttempted' pkg/events/events.go"
check "Phase41RehearsalDeliveryCompleted event exists" "grep -q 'Phase41RehearsalDeliveryCompleted' pkg/events/events.go"
check "Phase41RehearsalReceiptPersisted event exists" "grep -q 'Phase41RehearsalReceiptPersisted' pkg/events/events.go"
check "Phase41RehearsalProofViewed event exists" "grep -q 'Phase41RehearsalProofViewed' pkg/events/events.go"

# ============================================================================
# Section 20: Storelog Record Types
# ============================================================================
echo ""
echo "--- Section 20: Storelog Record Types ---"

check "RecordTypeInterruptRehearsalReceipt exists" "grep -q 'RecordTypeInterruptRehearsalReceipt' pkg/domain/storelog/log.go"
check "RecordTypeInterruptRehearsalAck exists" "grep -q 'RecordTypeInterruptRehearsalAck' pkg/domain/storelog/log.go"

# ============================================================================
# Section 21: No Retry Patterns
# ============================================================================
echo ""
echo "--- Section 21: No Retry Patterns ---"

check "No retry loops in engine" "! grep -qiE 'for.*retry|retry.*for|backoff|attempts.*:=.*0' internal/interruptrehearsal/engine.go"
check "No retry loops in store" "! grep -qiE 'for.*retry|retry.*for|backoff|attempts.*:=.*0' internal/persist/interrupt_rehearsal_store.go"

# ============================================================================
# Section 22: No New /today Whispers
# ============================================================================
echo ""
echo "--- Section 22: No New /today Whispers ---"

check "No whisper added to /today by Phase 41" "! grep -q 'Phase41.*whisper\|rehearsal.*whisper' cmd/quantumlife-web/main.go || ! grep -A20 'handleToday' cmd/quantumlife-web/main.go | grep -qi 'rehearsal'"

# ============================================================================
# Section 23: Validate Methods
# ============================================================================
echo ""
echo "--- Section 23: Validate Methods ---"

check "RehearsalKind has Validate" "grep -q 'func.*RehearsalKind.*Validate' pkg/domain/interruptrehearsal/types.go"
check "RehearsalStatus has Validate" "grep -q 'func.*RehearsalStatus.*Validate' pkg/domain/interruptrehearsal/types.go"
check "RehearsalRejectReason has Validate" "grep -q 'func.*RehearsalRejectReason.*Validate' pkg/domain/interruptrehearsal/types.go"
check "TransportKind has Validate" "grep -q 'func.*TransportKind.*Validate' pkg/domain/interruptrehearsal/types.go"
check "DeliveryBucket has Validate" "grep -q 'func.*DeliveryBucket.*Validate' pkg/domain/interruptrehearsal/types.go"
check "LatencyBucket has Validate" "grep -q 'func.*LatencyBucket.*Validate' pkg/domain/interruptrehearsal/types.go"
check "ErrorClassBucket has Validate" "grep -q 'func.*ErrorClassBucket.*Validate' pkg/domain/interruptrehearsal/types.go"

# ============================================================================
# Section 24: ADR Content
# ============================================================================
echo ""
echo "--- Section 24: ADR Content ---"

check "ADR has Status section" "grep -q '## Status' docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md"
check "ADR has Context section" "grep -q '## Context' docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md"
check "ADR has Decision section" "grep -q '## Decision' docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md"
check "ADR has Consequences section" "grep -q '## Consequences' docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "================================================"
echo "Summary: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo -e "${RED}FAIL: Some guardrails not met${NC}"
    exit 1
else
    echo -e "${GREEN}PASS: All guardrails passed${NC}"
    exit 0
fi
