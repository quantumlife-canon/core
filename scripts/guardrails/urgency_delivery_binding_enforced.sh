#!/bin/bash
# Phase 54: Urgency → Delivery Binding Guardrails
#
# CRITICAL INVARIANTS:
# - NO BACKGROUND EXECUTION: No goroutines, no timers, no polling.
# - POST-TRIGGERED ONLY: Any delivery attempt via explicit POST.
# - NO NEW DECISION LOGIC: Reuses existing pipelines.
# - COMMERCE EXCLUDED: Never escalated, never delivered.
# - ABSTRACT PAYLOAD ONLY: Two-line payload, no identifiers.
# - DETERMINISTIC: Same inputs + same clock → same output hash/ID.
# - HASH-ONLY STORAGE: Never stores raw identifiers.
# - No time.Now() in pkg/ or internal/. Use injected clock.
# - BOUNDED RETENTION: 30 days and max 200 records.
#
# Reference: docs/ADR/ADR-0092-phase54-urgency-delivery-binding.md

set -e

DOMAIN_FILE="pkg/domain/urgencydelivery/types.go"
ENGINE_FILE="internal/urgencydelivery/engine.go"
STORE_FILE="internal/persist/urgency_delivery_store.go"
DEMO_DIR="internal/demo_phase54_urgency_delivery_binding"
WEB_FILE="cmd/quantumlife-web/main.go"
EVENTS_FILE="pkg/events/events.go"
STORELOG_FILE="pkg/domain/storelog/log.go"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo "  ✓ $1"
    PASS_COUNT=$((PASS_COUNT+1))
}

fail() {
    echo "  ✗ $1"
    FAIL_COUNT=$((FAIL_COUNT+1))
}

check() {
    if eval "$2" > /dev/null 2>&1; then
        pass "$1"
    else
        fail "$1"
    fi
}

check_not() {
    if eval "$2" > /dev/null 2>&1; then
        fail "$1"
    else
        pass "$1"
    fi
}

echo "=== File Existence ==="
check "Domain types file exists" "test -f $DOMAIN_FILE"
check "Engine file exists" "test -f $ENGINE_FILE"
check "Persistence store file exists" "test -f $STORE_FILE"
check "Demo test directory exists" "test -d $DEMO_DIR"

echo ""
echo "=== Domain Types - BindingRunKind Enum ==="
check "BindingRunKind RunManual defined" "grep -q 'RunManual.*BindingRunKind.*run_manual' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - BindingOutcomeKind Enum ==="
check "BindingOutcomeKind OutcomeDelivered defined" "grep -q 'OutcomeDelivered.*BindingOutcomeKind.*outcome_delivered' $DOMAIN_FILE"
check "BindingOutcomeKind OutcomeNotDelivered defined" "grep -q 'OutcomeNotDelivered.*BindingOutcomeKind.*outcome_not_delivered' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - BindingRejectionReason Enum ==="
check "RejectNoCandidate defined" "grep -q 'RejectNoCandidate.*BindingRejectionReason.*reject_no_candidate' $DOMAIN_FILE"
check "RejectCommerceExcluded defined" "grep -q 'RejectCommerceExcluded.*BindingRejectionReason.*reject_commerce_excluded' $DOMAIN_FILE"
check "RejectPolicyDisallows defined" "grep -q 'RejectPolicyDisallows.*BindingRejectionReason.*reject_policy_disallows' $DOMAIN_FILE"
check "RejectNotPermittedByUrgency defined" "grep -q 'RejectNotPermittedByUrgency.*BindingRejectionReason.*reject_not_permitted_by_urgency' $DOMAIN_FILE"
check "RejectRateLimited defined" "grep -q 'RejectRateLimited.*BindingRejectionReason.*reject_rate_limited' $DOMAIN_FILE"
check "RejectNoDevice defined" "grep -q 'RejectNoDevice.*BindingRejectionReason.*reject_no_device' $DOMAIN_FILE"
check "RejectTransportUnavailable defined" "grep -q 'RejectTransportUnavailable.*BindingRejectionReason.*reject_transport_unavailable' $DOMAIN_FILE"
check "RejectSealedKeyMissing defined" "grep -q 'RejectSealedKeyMissing.*BindingRejectionReason.*reject_sealed_key_missing' $DOMAIN_FILE"
check "RejectEnforcementClamped defined" "grep -q 'RejectEnforcementClamped.*BindingRejectionReason.*reject_enforcement_clamped' $DOMAIN_FILE"
check "RejectInternalError defined" "grep -q 'RejectInternalError.*BindingRejectionReason.*reject_internal_error' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - UrgencyBucket Enum ==="
check "UrgencyBucket UrgencyNone defined" "grep -q 'UrgencyNone.*UrgencyBucket.*urgency_none' $DOMAIN_FILE"
check "UrgencyBucket UrgencyLow defined" "grep -q 'UrgencyLow.*UrgencyBucket.*urgency_low' $DOMAIN_FILE"
check "UrgencyBucket UrgencyMedium defined" "grep -q 'UrgencyMedium.*UrgencyBucket.*urgency_medium' $DOMAIN_FILE"
check "UrgencyBucket UrgencyHigh defined" "grep -q 'UrgencyHigh.*UrgencyBucket.*urgency_high' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - DeliveryIntentKind Enum ==="
check "DeliveryIntentKind IntentHold defined" "grep -q 'IntentHold.*DeliveryIntentKind.*intent_hold' $DOMAIN_FILE"
check "DeliveryIntentKind IntentSurfaceOnly defined" "grep -q 'IntentSurfaceOnly.*DeliveryIntentKind.*intent_surface_only' $DOMAIN_FILE"
check "DeliveryIntentKind IntentInterruptCandidate defined" "grep -q 'IntentInterruptCandidate.*DeliveryIntentKind.*intent_interrupt_candidate' $DOMAIN_FILE"
check "DeliveryIntentKind IntentDeliver defined" "grep -q 'IntentDeliver.*DeliveryIntentKind.*intent_deliver' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - CircleTypeBucket Enum ==="
check "CircleTypeBucket CircleTypeHuman defined" "grep -q 'CircleTypeHuman.*CircleTypeBucket.*bucket_human' $DOMAIN_FILE"
check "CircleTypeBucket CircleTypeInstitution defined" "grep -q 'CircleTypeInstitution.*CircleTypeBucket.*bucket_institution' $DOMAIN_FILE"
check "CircleTypeBucket CircleTypeCommerce defined" "grep -q 'CircleTypeCommerce.*CircleTypeBucket.*bucket_commerce' $DOMAIN_FILE"
check "CircleTypeBucket CircleTypeUnknown defined" "grep -q 'CircleTypeUnknown.*CircleTypeBucket.*bucket_unknown' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - PolicyAllowanceBucket Enum ==="
check "PolicyAllowanceBucket PolicyAllowed defined" "grep -q 'PolicyAllowed.*PolicyAllowanceBucket.*policy_allowed' $DOMAIN_FILE"
check "PolicyAllowanceBucket PolicyDenied defined" "grep -q 'PolicyDenied.*PolicyAllowanceBucket.*policy_denied' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - EnforcementClampBucket Enum ==="
check "EnforcementClampBucket EnforcementNotClamped defined" "grep -q 'EnforcementNotClamped.*EnforcementClampBucket.*enforcement_not_clamped' $DOMAIN_FILE"
check "EnforcementClampBucket EnforcementClamped defined" "grep -q 'EnforcementClamped.*EnforcementClampBucket.*enforcement_clamped' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - Struct Definitions ==="
check "BindingInputs struct defined" "grep -q 'type BindingInputs struct' $DOMAIN_FILE"
check "BindingDecision struct defined" "grep -q 'type BindingDecision struct' $DOMAIN_FILE"
check "UrgencyDeliveryReceipt struct defined" "grep -q 'type UrgencyDeliveryReceipt struct' $DOMAIN_FILE"
check "ReceiptLine struct defined" "grep -q 'type ReceiptLine struct' $DOMAIN_FILE"
check "ProofPage struct defined" "grep -q 'type ProofPage struct' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - Required Methods ==="
check "BindingRunKind.Validate() method exists" "grep -q 'func (k BindingRunKind) Validate()' $DOMAIN_FILE"
check "BindingOutcomeKind.Validate() method exists" "grep -q 'func (k BindingOutcomeKind) Validate()' $DOMAIN_FILE"
check "BindingRejectionReason.Validate() method exists" "grep -q 'func (r BindingRejectionReason) Validate()' $DOMAIN_FILE"
check "UrgencyBucket.Validate() method exists" "grep -q 'func (u UrgencyBucket) Validate()' $DOMAIN_FILE"
check "DeliveryIntentKind.Validate() method exists" "grep -q 'func (i DeliveryIntentKind) Validate()' $DOMAIN_FILE"
check "BindingInputs.CanonicalString() method exists" "grep -q 'func (i BindingInputs) CanonicalString()' $DOMAIN_FILE"
check "BindingInputs.Validate() method exists" "grep -q 'func (i BindingInputs) Validate()' $DOMAIN_FILE"
check "BindingInputs.Hash() method exists" "grep -q 'func (i BindingInputs) Hash()' $DOMAIN_FILE"
check "BindingDecision.CanonicalString() method exists" "grep -q 'func (d BindingDecision) CanonicalString()' $DOMAIN_FILE"
check "BindingDecision.ComputeHash() method exists" "grep -q 'func (d BindingDecision) ComputeHash()' $DOMAIN_FILE"
check "UrgencyDeliveryReceipt.CanonicalString() method exists" "grep -q 'func (r UrgencyDeliveryReceipt) CanonicalString()' $DOMAIN_FILE"
check "UrgencyDeliveryReceipt.ComputeReceiptHash() method exists" "grep -q 'func (r UrgencyDeliveryReceipt) ComputeReceiptHash()' $DOMAIN_FILE"
check "UrgencyDeliveryReceipt.DedupKey() method exists" "grep -q 'func (r UrgencyDeliveryReceipt) DedupKey()' $DOMAIN_FILE"
check "ProofPage.Validate() method exists" "grep -q 'func (p ProofPage) Validate()' $DOMAIN_FILE"

echo ""
echo "=== Commerce Excluded Check ==="
check "IsCommerce() method exists" "grep -q 'func (c CircleTypeBucket) IsCommerce()' $DOMAIN_FILE"
check "AllowsDelivery() method exists" "grep -q 'func (u UrgencyBucket) AllowsDelivery()' $DOMAIN_FILE"

echo ""
echo "=== Engine - Required Functions ==="
check "Engine struct defined" "grep -q 'type Engine struct' $ENGINE_FILE"
check "NewEngine function exists" "grep -q 'func NewEngine()' $ENGINE_FILE"
check "ComputeDecision function exists" "grep -q 'func (e \*Engine) ComputeDecision' $ENGINE_FILE"
check "BuildReceipt function exists" "grep -q 'func (e \*Engine) BuildReceipt' $ENGINE_FILE"
check "BuildReceiptWithAttempt function exists" "grep -q 'func (e \*Engine) BuildReceiptWithAttempt' $ENGINE_FILE"
check "BuildProofPage function exists" "grep -q 'func (e \*Engine) BuildProofPage' $ENGINE_FILE"
check "MapUrgencyLevel function exists" "grep -q 'func MapUrgencyLevel' $ENGINE_FILE"
check "MapCircleType function exists" "grep -q 'func MapCircleType' $ENGINE_FILE"

echo ""
echo "=== Engine - Decision Rules ==="
check "Engine checks for no candidate" "grep -q 'RejectNoCandidate' $ENGINE_FILE"
check "Engine checks for commerce excluded" "grep -q 'RejectCommerceExcluded' $ENGINE_FILE"
check "Engine checks for enforcement clamped" "grep -q 'RejectEnforcementClamped' $ENGINE_FILE"
check "Engine checks for policy disallows" "grep -q 'RejectPolicyDisallows' $ENGINE_FILE"
check "Engine checks for urgency not permitted" "grep -q 'RejectNotPermittedByUrgency' $ENGINE_FILE"
check "Engine checks for no device" "grep -q 'RejectNoDevice' $ENGINE_FILE"
check "Engine checks for transport unavailable" "grep -q 'RejectTransportUnavailable' $ENGINE_FILE"
check "Engine checks for sealed key missing" "grep -q 'RejectSealedKeyMissing' $ENGINE_FILE"
check "Engine checks for rate limited" "grep -q 'RejectRateLimited' $ENGINE_FILE"

echo ""
echo "=== Store - Required Functions ==="
check "UrgencyDeliveryStore struct defined" "grep -q 'type UrgencyDeliveryStore struct' $STORE_FILE"
check "NewUrgencyDeliveryStore function exists" "grep -q 'func NewUrgencyDeliveryStore' $STORE_FILE"
check "AppendReceipt function exists" "grep -q 'func (s \*UrgencyDeliveryStore) AppendReceipt' $STORE_FILE"
check "ListRecentByCircle function exists" "grep -q 'func (s \*UrgencyDeliveryStore) ListRecentByCircle' $STORE_FILE"
check "HasReceiptForCandidatePeriod function exists" "grep -q 'func (s \*UrgencyDeliveryStore) HasReceiptForCandidatePeriod' $STORE_FILE"
check "CountDeliveredForPeriod function exists" "grep -q 'func (s \*UrgencyDeliveryStore) CountDeliveredForPeriod' $STORE_FILE"

echo ""
echo "=== Bounded Retention ==="
check "MaxEntries constant defined" "grep -q 'UrgencyDeliveryMaxEntries.*=.*200' $STORE_FILE"
check "MaxRetentionDays constant defined" "grep -q 'UrgencyDeliveryMaxRetentionDays.*=.*30' $STORE_FILE"
check "MaxDeliveriesPerDay constant defined" "grep -q 'MaxDeliveriesPerDay.*=.*2' $DOMAIN_FILE"
check "MaxProofPageReceipts constant defined" "grep -q 'MaxProofPageReceipts.*=.*6' $DOMAIN_FILE"

echo ""
echo "=== Pipe-Delimited Canonical Strings ==="
check "Canonical strings use pipe delimiter" "grep -q 'strings.Join.*\"|\"' $DOMAIN_FILE"
check "No JSON marshaling in domain package" "! grep -q 'json.Marshal' $DOMAIN_FILE"

echo ""
echo "=== No time.Now() in Domain/Engine/Store ==="
# Exclude comments (lines starting with //) when checking for time.Now()
check_not "No time.Now() in domain package" "grep -v '^[[:space:]]*//' $DOMAIN_FILE | grep -q 'time\\.Now()'"
check_not "No time.Now() in engine package" "grep -v '^[[:space:]]*//' $ENGINE_FILE | grep -q 'time\\.Now()'"
check_not "No time.Now() in store" "grep -v '^[[:space:]]*//' $STORE_FILE | grep -q 'time\\.Now()'"

echo ""
echo "=== No Goroutines (No Background Execution) ==="
check_not "No goroutines in domain package" "grep -q 'go func' $DOMAIN_FILE"
check_not "No goroutines in engine package" "grep -q 'go func' $ENGINE_FILE"
check_not "No goroutines in store" "grep -q 'go func' $STORE_FILE"

echo ""
echo "=== No Timers/Polling ==="
check_not "No time.Ticker in domain" "grep -q 'time\\.Ticker' $DOMAIN_FILE"
check_not "No time.Timer in domain" "grep -q 'time\\.Timer' $DOMAIN_FILE"
check_not "No time.AfterFunc in domain" "grep -q 'time\\.AfterFunc' $DOMAIN_FILE"
check_not "No time.Ticker in engine" "grep -q 'time\\.Ticker' $ENGINE_FILE"
check_not "No time.Timer in engine" "grep -q 'time\\.Timer' $ENGINE_FILE"
check_not "No time.AfterFunc in engine" "grep -q 'time\\.AfterFunc' $ENGINE_FILE"
check_not "No time.Ticker in store" "grep -q 'time\\.Ticker' $STORE_FILE"
check_not "No time.Timer in store" "grep -q 'time\\.Timer' $STORE_FILE"
check_not "No time.AfterFunc in store" "grep -q 'time\\.AfterFunc' $STORE_FILE"

echo ""
echo "=== Forbidden Patterns (No Raw Identifiers) ==="
FORBIDDEN_PATTERNS="vendor_id pack_id merchant amount currency sender subject recipient device_token"
for pattern in $FORBIDDEN_PATTERNS; do
    check_not "No '$pattern' in domain (as struct field)" "grep -E '${pattern}\\s+string' $DOMAIN_FILE"
done

echo ""
echo "=== Events ==="
check "Phase54UrgencyDeliveryRequested event defined" "grep -q 'Phase54UrgencyDeliveryRequested' $EVENTS_FILE"
check "Phase54UrgencyDeliveryComputed event defined" "grep -q 'Phase54UrgencyDeliveryComputed' $EVENTS_FILE"
check "Phase54UrgencyDeliveryRejected event defined" "grep -q 'Phase54UrgencyDeliveryRejected' $EVENTS_FILE"
check "Phase54UrgencyDeliveryAttempted event defined" "grep -q 'Phase54UrgencyDeliveryAttempted' $EVENTS_FILE"
check "Phase54UrgencyDeliveryDelivered event defined" "grep -q 'Phase54UrgencyDeliveryDelivered' $EVENTS_FILE"
check "Phase54UrgencyDeliveryPersisted event defined" "grep -q 'Phase54UrgencyDeliveryPersisted' $EVENTS_FILE"

echo ""
echo "=== Storelog Record Types ==="
check "RecordTypeUrgencyDeliveryReceipt defined" "grep -q 'RecordTypeUrgencyDeliveryReceipt' $STORELOG_FILE"

echo ""
echo "=== Web Routes ==="
check "/proof/urgency-delivery route defined" "grep -q '/proof/urgency-delivery' $WEB_FILE"
check "/run/urgency-delivery route defined" "grep -q '/run/urgency-delivery' $WEB_FILE"
check "handleUrgencyDeliveryProof handler exists" "grep -q 'func (s \*Server) handleUrgencyDeliveryProof' $WEB_FILE"
check "handleUrgencyDeliveryRun handler exists" "grep -q 'func (s \*Server) handleUrgencyDeliveryRun' $WEB_FILE"

echo ""
echo "=== POST-Only for Run Endpoint ==="
check "Run handler checks for POST method" "grep -A5 'handleUrgencyDeliveryRun' $WEB_FILE | grep -q 'MethodPost'"

echo ""
echo "=== Demo Tests ==="
check "Demo test file exists" "test -f $DEMO_DIR/demo_test.go"
if [ -f "$DEMO_DIR/demo_test.go" ]; then
    check "Test for determinism exists" "grep -q 'Determinism' $DEMO_DIR/demo_test.go"
    check "Test for commerce excluded exists" "grep -q 'Commerce' $DEMO_DIR/demo_test.go"
    check "Test for urgency level check exists" "grep -q 'Urgency' $DEMO_DIR/demo_test.go"
    check "Test for rate limit exists" "grep -q 'RateLimit' $DEMO_DIR/demo_test.go"
    check "Test for enforcement clamp exists" "grep -q 'Enforcement' $DEMO_DIR/demo_test.go"
fi

echo ""
echo "=== Forbidden Patterns in Handlers ==="
HANDLER_FORBIDDEN="vendorID packID merchant amount £ \\$ @"
for pattern in $HANDLER_FORBIDDEN; do
    check_not "No '$pattern' in handlers" "grep -E '^[^/]*$pattern' $WEB_FILE | grep -q 'handleUrgencyDelivery'"
done

echo ""
echo "==========================================="
echo "Phase 54 Guardrails Summary"
echo "==========================================="
echo "Total checks: $((PASS_COUNT + FAIL_COUNT))"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -gt 0 ]; then
    echo ""
    echo "GUARDRAILS FAILED"
    exit 1
else
    echo ""
    echo "All Phase 54 guardrails passed!"
    exit 0
fi
