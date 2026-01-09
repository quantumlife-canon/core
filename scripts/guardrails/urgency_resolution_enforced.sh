#!/bin/bash
# Phase 53: Urgency Resolution Layer Guardrails
#
# CRITICAL INVARIANTS:
# - NO POWER: Cap-only, clamp-only, no execution, no delivery.
# - HASH-ONLY: Only hashes, buckets, status flags stored/rendered.
# - NO TIMESTAMPS: Only recency/horizon buckets.
# - NO COUNTS: Only magnitude buckets.
# - DETERMINISTIC: Same inputs + same clock = same resolution hash.
# - PIPE-DELIMITED: Canonical strings use pipe format, not JSON.
# - COMMERCE NEVER ESCALATES: Always cap_hold_only.
# - CAPS ONLY REDUCE: Never increase power.
# - REASONS MAX 3: Sorted, capped at 3.
# - POST-ONLY MUTATIONS: Run and dismiss are POST-only.
#
# Reference: docs/ADR/ADR-0091-phase53-urgency-resolution-layer.md

set -e

DOMAIN_FILE="pkg/domain/urgencyresolve/types.go"
ENGINE_FILE="internal/urgencyresolve/engine.go"
STORE_FILE="internal/persist/urgency_resolution_store.go"
DEMO_DIR="internal/demo_phase53_urgency_resolution"
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
echo "=== Domain Types - UrgencyLevel Enum ==="
check "UrgencyLevel UrgNone defined" "grep -q 'UrgNone.*UrgencyLevel.*urg_none' $DOMAIN_FILE"
check "UrgencyLevel UrgLow defined" "grep -q 'UrgLow.*UrgencyLevel.*urg_low' $DOMAIN_FILE"
check "UrgencyLevel UrgMedium defined" "grep -q 'UrgMedium.*UrgencyLevel.*urg_medium' $DOMAIN_FILE"
check "UrgencyLevel UrgHigh defined" "grep -q 'UrgHigh.*UrgencyLevel.*urg_high' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - EscalationCap Enum ==="
check "EscalationCap CapHoldOnly defined" "grep -q 'CapHoldOnly.*EscalationCap.*cap_hold_only' $DOMAIN_FILE"
check "EscalationCap CapSurfaceOnly defined" "grep -q 'CapSurfaceOnly.*EscalationCap.*cap_surface_only' $DOMAIN_FILE"
check "EscalationCap CapInterruptCandidateOnly defined" "grep -q 'CapInterruptCandidateOnly.*EscalationCap.*cap_interrupt_candidate_only' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - UrgencyReasonBucket Enum ==="
check "ReasonTimeWindow defined" "grep -q 'ReasonTimeWindow.*UrgencyReasonBucket.*reason_time_window' $DOMAIN_FILE"
check "ReasonInstitutionDeadline defined" "grep -q 'ReasonInstitutionDeadline.*UrgencyReasonBucket.*reason_institution_deadline' $DOMAIN_FILE"
check "ReasonHumanNow defined" "grep -q 'ReasonHumanNow.*UrgencyReasonBucket.*reason_human_now' $DOMAIN_FILE"
check "ReasonTrustProtection defined" "grep -q 'ReasonTrustProtection.*UrgencyReasonBucket.*reason_trust_protection' $DOMAIN_FILE"
check "ReasonVendorContractCap defined" "grep -q 'ReasonVendorContractCap.*UrgencyReasonBucket.*reason_vendor_contract_cap' $DOMAIN_FILE"
check "ReasonSemanticsNecessity defined" "grep -q 'ReasonSemanticsNecessity.*UrgencyReasonBucket.*reason_semantics_necessity' $DOMAIN_FILE"
check "ReasonEnvelopeActive defined" "grep -q 'ReasonEnvelopeActive.*UrgencyReasonBucket.*reason_envelope_active' $DOMAIN_FILE"
check "ReasonDefaultHold defined" "grep -q 'ReasonDefaultHold.*UrgencyReasonBucket.*reason_default_hold' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - ResolutionStatus Enum ==="
check "ResolutionStatus StatusOK defined" "grep -q 'StatusOK.*ResolutionStatus.*status_ok' $DOMAIN_FILE"
check "ResolutionStatus StatusClamped defined" "grep -q 'StatusClamped.*ResolutionStatus.*status_clamped' $DOMAIN_FILE"
check "ResolutionStatus StatusRejected defined" "grep -q 'StatusRejected.*ResolutionStatus.*status_rejected' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - CircleTypeBucket Enum ==="
check "CircleTypeBucket BucketHuman defined" "grep -q 'BucketHuman.*CircleTypeBucket.*bucket_human' $DOMAIN_FILE"
check "CircleTypeBucket BucketInstitution defined" "grep -q 'BucketInstitution.*CircleTypeBucket.*bucket_institution' $DOMAIN_FILE"
check "CircleTypeBucket BucketCommerce defined" "grep -q 'BucketCommerce.*CircleTypeBucket.*bucket_commerce' $DOMAIN_FILE"
check "CircleTypeBucket BucketUnknown defined" "grep -q 'BucketUnknown.*CircleTypeBucket.*bucket_unknown' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - RecencyBucket Enum ==="
check "RecencyBucket RecNever defined" "grep -q 'RecNever.*RecencyBucket.*rec_never' $DOMAIN_FILE"
check "RecencyBucket RecRecent defined" "grep -q 'RecRecent.*RecencyBucket.*rec_recent' $DOMAIN_FILE"
check "RecencyBucket RecStale defined" "grep -q 'RecStale.*RecencyBucket.*rec_stale' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - Struct Definitions ==="
check "UrgencyInputs struct defined" "grep -q 'type UrgencyInputs struct' $DOMAIN_FILE"
check "UrgencyResolution struct defined" "grep -q 'type UrgencyResolution struct' $DOMAIN_FILE"
check "UrgencyProofPage struct defined" "grep -q 'type UrgencyProofPage struct' $DOMAIN_FILE"
check "UrgencyCue struct defined" "grep -q 'type UrgencyCue struct' $DOMAIN_FILE"
check "UrgencyAck struct defined" "grep -q 'type UrgencyAck struct' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - Required Methods ==="
check "UrgencyLevel.Validate() method exists" "grep -q 'func (u UrgencyLevel) Validate()' $DOMAIN_FILE"
check "EscalationCap.Validate() method exists" "grep -q 'func (c EscalationCap) Validate()' $DOMAIN_FILE"
check "UrgencyReasonBucket.Validate() method exists" "grep -q 'func (r UrgencyReasonBucket) Validate()' $DOMAIN_FILE"
check "ResolutionStatus.Validate() method exists" "grep -q 'func (s ResolutionStatus) Validate()' $DOMAIN_FILE"
check "CircleTypeBucket.Validate() method exists" "grep -q 'func (c CircleTypeBucket) Validate()' $DOMAIN_FILE"
check "UrgencyInputs.CanonicalString() method exists" "grep -q 'func (i UrgencyInputs) CanonicalString()' $DOMAIN_FILE"
check "UrgencyInputs.Validate() method exists" "grep -q 'func (i UrgencyInputs) Validate()' $DOMAIN_FILE"
check "UrgencyResolution.CanonicalString() method exists" "grep -q 'func (r UrgencyResolution) CanonicalString()' $DOMAIN_FILE"
check "UrgencyResolution.ComputeHash() method exists" "grep -q 'func (r UrgencyResolution) ComputeHash()' $DOMAIN_FILE"
check "UrgencyAck.CanonicalString() method exists" "grep -q 'func (a UrgencyAck) CanonicalString()' $DOMAIN_FILE"
check "SortReasons function exists" "grep -q 'func SortReasons' $DOMAIN_FILE"
check "MinCap function exists" "grep -q 'func MinCap' $DOMAIN_FILE"

echo ""
echo "=== Pipe-Delimited Canonical Strings ==="
check "Canonical strings use pipe delimiter" "grep -q 'strings.Join.*\"|\"' $DOMAIN_FILE"
check "Inputs canonical format v1|circle=|period=" "grep -q 'v1.*circle=.*period=' $DOMAIN_FILE"
check "No JSON marshaling in domain package" "! grep -q 'json.Marshal' $DOMAIN_FILE"

echo ""
echo "=== No time.Now() in Domain/Engine/Store ==="
check_not "No time.Now() in domain package" "grep -q 'time\\.Now()' $DOMAIN_FILE"
check_not "No time.Now() in engine package" "grep -q 'time\\.Now()' $ENGINE_FILE"
check_not "No time.Now() in store" "grep -q 'time\\.Now()' $STORE_FILE"

echo ""
echo "=== No Goroutines ==="
check_not "No goroutines in domain package" "grep -q 'go func' $DOMAIN_FILE"
check_not "No goroutines in engine package" "grep -q 'go func' $ENGINE_FILE"
check_not "No goroutines in store" "grep -q 'go func' $STORE_FILE"

echo ""
echo "=== Forbidden Imports (No Power) ==="
FORBIDDEN_IMPORTS="pressuredecision interruptpolicy interruptpreview pushtransport interruptdelivery enforcementclamp"
for imp in $FORBIDDEN_IMPORTS; do
    check_not "No forbidden import $imp in domain" "grep -q '\"quantumlife/internal/$imp\"' $DOMAIN_FILE"
    check_not "No forbidden import $imp in engine" "grep -q '\"quantumlife/internal/$imp\"' $ENGINE_FILE"
    check_not "No forbidden import $imp in store" "grep -q '\"quantumlife/internal/$imp\"' $STORE_FILE"
done

echo ""
echo "=== Stdlib Only (No External Dependencies) ==="
check_not "No external dependencies in domain package" "grep -E '^[[:space:]]+\"github.com|^[[:space:]]+\"golang.org' $DOMAIN_FILE"
check_not "No external dependencies in engine package" "grep -E '^[[:space:]]+\"github.com|^[[:space:]]+\"golang.org' $ENGINE_FILE"
check_not "No external dependencies in store" "grep -E '^[[:space:]]+\"github.com|^[[:space:]]+\"golang.org' $STORE_FILE"

echo ""
echo "=== Engine Functions ==="
check "Engine.BuildInputs() method exists" "grep -q 'func (e \\*Engine) BuildInputs' $ENGINE_FILE"
check "Engine.ComputeResolution() method exists" "grep -q 'func (e \\*Engine) ComputeResolution' $ENGINE_FILE"
check "Engine.BuildProofPage() method exists" "grep -q 'func (e \\*Engine) BuildProofPage' $ENGINE_FILE"
check "Engine.BuildCue() method exists" "grep -q 'func (e \\*Engine) BuildCue' $ENGINE_FILE"
check "Engine.ShouldShowCue() method exists" "grep -q 'func (e \\*Engine) ShouldShowCue' $ENGINE_FILE"
check "ComputePeriodKey function exists" "grep -q 'func ComputePeriodKey' $ENGINE_FILE"

echo ""
echo "=== Engine Source Interfaces ==="
check "PressureSource interface exists" "grep -q 'type PressureSource interface' $ENGINE_FILE"
check "EnvelopeSource interface exists" "grep -q 'type EnvelopeSource interface' $ENGINE_FILE"
check "TimeWindowSource interface exists" "grep -q 'type TimeWindowSource interface' $ENGINE_FILE"
check "VendorCapSource interface exists" "grep -q 'type VendorCapSource interface' $ENGINE_FILE"
check "SemanticsSource interface exists" "grep -q 'type SemanticsSource interface' $ENGINE_FILE"
check "TrustSource interface exists" "grep -q 'type TrustSource interface' $ENGINE_FILE"
check "PolicySource interface exists" "grep -q 'type PolicySource interface' $ENGINE_FILE"
check "CircleTypeSource interface exists" "grep -q 'type CircleTypeSource interface' $ENGINE_FILE"
check "AckSource interface exists" "grep -q 'type AckSource interface' $ENGINE_FILE"

echo ""
echo "=== Engine Resolution Rules ==="
check "Rule0 default HOLD" "grep -q 'reason_default_hold\\|ReasonDefaultHold' $ENGINE_FILE"
check "Rule1 commerce cap_hold_only" "grep -q 'BucketCommerce' $ENGINE_FILE"
check "Rule2 VendorCap min clamp" "grep -q 'VendorCap' $ENGINE_FILE"
check "Rule3 TrustFragile clamp" "grep -q 'TrustFragile' $ENGINE_FILE"
check "Rule4 WindowSignal level shift" "grep -q 'WindowSignal\\|WindowActive' $ENGINE_FILE"
check "Rule7 EnvelopeActive level shift" "grep -q 'EnvelopeActive' $ENGINE_FILE"
check "Rule8 NecessityDeclared reduce only" "grep -q 'NecessityDeclared' $ENGINE_FILE"
check "Cap-to-level mapping exists" "grep -q 'capToMaxLevel' $ENGINE_FILE"

echo ""
echo "=== Store Constants ==="
check "Store has max entries constant (500)" "grep -q 'UrgencyResolutionMaxEntries.*=.*500' $STORE_FILE"
check "Store has max retention days constant (30)" "grep -q 'UrgencyResolutionMaxRetentionDays.*=.*30' $STORE_FILE"

echo ""
echo "=== Append-Only Store ==="
check "Store.RecordResolution() method exists" "grep -q 'func (s \\*UrgencyResolutionStore) RecordResolution' $STORE_FILE"
check "Store.GetLatestResolution() method exists" "grep -q 'func (s \\*UrgencyResolutionStore) GetLatestResolution' $STORE_FILE"
check "Store has dedup index" "grep -q 'dedupIndex' $STORE_FILE"
check "Store has eviction method" "grep -q 'evictOldEntriesLocked' $STORE_FILE"

echo ""
echo "=== Ack Store ==="
check "AckStore.RecordDismissed() method exists" "grep -q 'func (s \\*UrgencyAckStore) RecordDismissed' $STORE_FILE"
check "AckStore.IsDismissed() method exists" "grep -q 'func (s \\*UrgencyAckStore) IsDismissed' $STORE_FILE"
check "AckStore.LastAckedResolutionHash() method exists" "grep -q 'func (s \\*UrgencyAckStore) LastAckedResolutionHash' $STORE_FILE"

echo ""
echo "=== Events ==="
check "Phase53UrgencyRequested event defined" "grep -q 'Phase53UrgencyRequested.*EventType.*phase53.urgency.requested' $EVENTS_FILE"
check "Phase53UrgencyComputed event defined" "grep -q 'Phase53UrgencyComputed.*EventType.*phase53.urgency.computed' $EVENTS_FILE"
check "Phase53UrgencyPersisted event defined" "grep -q 'Phase53UrgencyPersisted.*EventType.*phase53.urgency.persisted' $EVENTS_FILE"
check "Phase53UrgencyProofRendered event defined" "grep -q 'Phase53UrgencyProofRendered.*EventType.*phase53.urgency.proof.rendered' $EVENTS_FILE"
check "Phase53UrgencyDismissed event defined" "grep -q 'Phase53UrgencyDismissed.*EventType.*phase53.urgency.dismissed' $EVENTS_FILE"
check "Phase53UrgencyRejected event defined" "grep -q 'Phase53UrgencyRejected.*EventType.*phase53.urgency.rejected' $EVENTS_FILE"

echo ""
echo "=== Storelog Record Types ==="
check "RecordTypeUrgencyResolution defined" "grep -q 'RecordTypeUrgencyResolution.*=.*URGENCY_RESOLUTION' $STORELOG_FILE"
check "RecordTypeUrgencyAck defined" "grep -q 'RecordTypeUrgencyAck.*=.*URGENCY_ACK' $STORELOG_FILE"

echo ""
echo "=== Web Routes ==="
check "GET /proof/urgency route defined" "grep -q '/proof/urgency.*handleUrgencyProof' $WEB_FILE"
check "POST /proof/urgency/run route defined" "grep -q '/proof/urgency/run.*handleUrgencyRun' $WEB_FILE"
check "POST /proof/urgency/dismiss route defined" "grep -q '/proof/urgency/dismiss.*handleUrgencyDismiss' $WEB_FILE"

echo ""
echo "=== Web Handlers ==="
check "handleUrgencyProof function exists" "grep -q 'func (s \\*Server) handleUrgencyProof' $WEB_FILE"
check "handleUrgencyRun function exists" "grep -q 'func (s \\*Server) handleUrgencyRun' $WEB_FILE"
check "handleUrgencyDismiss function exists" "grep -q 'func (s \\*Server) handleUrgencyDismiss' $WEB_FILE"
check "renderUrgencyProofPage function exists" "grep -q 'func (s \\*Server) renderUrgencyProofPage' $WEB_FILE"

echo ""
echo "=== POST-Only Mutations ==="
check "handleUrgencyRun is POST-only" "grep -A5 'handleUrgencyRun' $WEB_FILE | grep -q 'MethodPost'"
check "handleUrgencyDismiss is POST-only" "grep -A5 'handleUrgencyDismiss' $WEB_FILE | grep -q 'MethodPost'"

echo ""
echo "=== Forbidden Patterns in Domain Structs ==="
# Check that domain structs don't have vendorID/packID fields (exclude ForbiddenPatterns list)
check_not "No vendorID struct field in domain" "grep -E 'VendorID\\s+string' $DOMAIN_FILE"
check_not "No packID struct field in domain" "grep -E 'PackID\\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'merchant' in domain structs" "grep -E 'Merchant\\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'sender' in domain structs" "grep -E 'Sender\\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'subject' in domain structs" "grep -E 'Subject\\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'amount' in domain structs" "grep -E 'Amount\\s+(int|float|string)' $DOMAIN_FILE"
check_not "No forbidden pattern 'currency' in domain structs" "grep -E 'Currency\\s+string' $DOMAIN_FILE"

echo ""
echo "=== Web Handlers Reject Forbidden Params ==="
check "Proof handler checks forbidden params" "grep -A30 'handleUrgencyProof' $WEB_FILE | grep -q 'forbiddenParams'"
check "Run handler checks forbidden params" "grep -A30 'handleUrgencyRun' $WEB_FILE | grep -q 'forbiddenParams'"
check "Dismiss handler checks forbidden params" "grep -A30 'handleUrgencyDismiss' $WEB_FILE | grep -q 'forbiddenParams'"

echo ""
echo "=== Web Handlers Reject Period Injection ==="
check "Proof handler rejects period param" "grep -A30 'handleUrgencyProof' $WEB_FILE | grep -q '\"period\"'"
check "Run handler rejects period param" "grep -A30 'handleUrgencyRun' $WEB_FILE | grep -q '\"period\"'"
check "Dismiss handler rejects period param" "grep -A30 'handleUrgencyDismiss' $WEB_FILE | grep -q '\"period\"'"

echo ""
echo "=== Commerce Always Hold-Only ==="
check "Commerce clamp to hold-only in engine" "grep -A10 'BucketCommerce' $ENGINE_FILE | grep -q 'CapHoldOnly'"

echo ""
echo "=== Reasons Max 3 + Sorted ==="
check "SortReasons caps at 3" "grep -A15 'func SortReasons' $DOMAIN_FILE | grep -q 'len(sorted) > 3'"
check "SortReasons uses sort" "grep -A15 'func SortReasons' $DOMAIN_FILE | grep -q 'sort.Slice'"
check "Resolution validates reasons max 3" "grep -A20 'func (r UrgencyResolution) Validate' $DOMAIN_FILE | grep -q 'len(r.Reasons) > 3'"
check "ProofPage validates reason chips max 3" "grep -A20 'func (p UrgencyProofPage) Validate' $DOMAIN_FILE | grep -q 'len(p.ReasonChips) > 3'"

echo ""
echo "=== Hash-Only Storage ==="
check "Resolution has ResolutionHash field" "grep -q 'ResolutionHash.*string' $DOMAIN_FILE"
check "Ack has ResolutionHash field" "grep -A10 'type UrgencyAck struct' $DOMAIN_FILE | grep -q 'ResolutionHash'"
check "Store dedup uses hashes" "grep -q 'dedupKey.*resolutionHash' $STORE_FILE"

echo ""
echo "=== SHA256 Hashing ==="
check "Domain uses SHA256 for resolution hash" "grep -q 'crypto/sha256' $DOMAIN_FILE"
check "ComputeHash uses SHA256" "grep -A10 'func (r UrgencyResolution) ComputeHash' $DOMAIN_FILE | grep -q 'sha256.Sum256'"

echo ""
echo "=== Status Hash Rendered ==="
check "StatusHash field exists in ProofPage" "grep -A15 'type UrgencyProofPage struct' $DOMAIN_FILE | grep -q 'StatusHash'"
check "Status hash is rendered in page" "grep -A100 'renderUrgencyProofPage' $WEB_FILE | grep -q 'StatusHash'"

echo ""
echo "=== Lines Max 8 ==="
check "ProofPage validates lines max 8" "grep -A20 'func (p UrgencyProofPage) Validate' $DOMAIN_FILE | grep -q 'len(p.Lines) > 8'"

echo ""
echo "=== Cue Properties ==="
check "UrgencyCue has Available field" "grep -A10 'type UrgencyCue struct' $DOMAIN_FILE | grep -q 'Available.*bool'"
check "UrgencyCue has Line field" "grep -A10 'type UrgencyCue struct' $DOMAIN_FILE | grep -q 'Line.*string'"
check "UrgencyCue has Priority field" "grep -A10 'type UrgencyCue struct' $DOMAIN_FILE | grep -q 'Priority.*int'"
check "BuildCue respects single-whisper rule" "grep -A15 'func (e \\*Engine) BuildCue' $ENGINE_FILE | grep -q 'alreadyHasHigherCue'"

echo ""
echo "=== Demo Tests ==="
if [ -f "$DEMO_DIR/demo_test.go" ]; then
    TEST_COUNT=$(grep -c 'func Test' "$DEMO_DIR/demo_test.go" || echo "0")
    if [ "$TEST_COUNT" -ge 30 ]; then
        pass "Demo tests have >= 30 test functions ($TEST_COUNT found)"
    else
        fail "Demo tests should have >= 30 test functions ($TEST_COUNT found)"
    fi
else
    fail "Demo test file exists"
fi

echo ""
echo "=== No Decision Logic Imports in Phase 53 ==="
DECISION_IMPORTS="pressuredecision interruptpolicy interruptpreview pushtransport interruptdelivery enforcementclamp"
for imp in $DECISION_IMPORTS; do
    check_not "Phase 53 does not import: $imp" "grep -rq '\"quantumlife/internal/$imp\"' $DOMAIN_FILE $ENGINE_FILE $STORE_FILE"
done

echo ""
echo "=== No Delivery/Execution in Phase 53 ==="
# Check for actual deliver/execute/push function calls (exclude comments)
check_not "No 'Deliver' function call in engine" "grep -E '^[^/]*Deliver' $ENGINE_FILE"
check_not "No 'Execute' function call in engine" "grep -E '^[^/]*Execute\\(' $ENGINE_FILE"
check_not "No 'Push' function call in engine" "grep -E '^[^/]*Push\\(' $ENGINE_FILE"
check_not "No 'Notify' function call in engine" "grep -E '^[^/]*Notify\\(' $ENGINE_FILE"

echo ""
echo "=== Clock Injection ==="
check "Engine has Clock interface" "grep -q 'type Clock interface' $ENGINE_FILE"
check "Engine receives clock in constructor" "grep -q 'clk.*Clock' $ENGINE_FILE"
check "Store receives clock function" "grep -q 'clock.*func().*time.Time' $STORE_FILE"

echo ""
echo "=== Forbidden Patterns Validation ==="
check "ContainsForbiddenPattern function exists" "grep -q 'func ContainsForbiddenPattern' $DOMAIN_FILE"
check "ForbiddenPatterns list exists" "grep -q 'ForbiddenPatterns.*=.*\\[\\]string' $DOMAIN_FILE"
check "Forbidden patterns include @" "grep -A20 'ForbiddenPatterns' $DOMAIN_FILE | grep -q '@'"
check "Forbidden patterns include http" "grep -A20 'ForbiddenPatterns' $DOMAIN_FILE | grep -q 'http'"
check "Forbidden patterns include vendor_id" "grep -A20 'ForbiddenPatterns' $DOMAIN_FILE | grep -q 'vendor_id'"
check "Forbidden patterns include merchant" "grep -A20 'ForbiddenPatterns' $DOMAIN_FILE | grep -q 'merchant'"

echo ""
echo "==========================================="
echo "Phase 53 Guardrails Summary"
echo "==========================================="
echo "Total checks: $((PASS_COUNT + FAIL_COUNT))"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -eq 0 ]; then
    echo ""
    echo "All Phase 53 guardrails passed!"
    exit 0
else
    echo ""
    echo "ERROR: $FAIL_COUNT guardrail(s) failed"
    exit 1
fi
