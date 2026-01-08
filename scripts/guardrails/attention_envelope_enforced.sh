#!/bin/bash
# Phase 39: Attention Envelope Guardrails
#
# CRITICAL INVARIANTS:
#   - No envelope = no change (calm is default)
#   - Explicit start (POST only)
#   - Bounded duration (15m, 1h, 4h, day)
#   - Effects bounded: max 1 step horizon, +1 magnitude, +1 cap
#   - Commerce excluded: never escalated
#   - Does NOT bypass Phase 33/34 permission/preview
#   - No time.Now() (clock injection only)
#   - No goroutines
#   - stdlib only
#   - Hash-only storage
#
# Reference: docs/ADR/ADR-0076-phase39-attention-envelopes.md

set -e

REPO_ROOT="${REPO_ROOT:-$(git rev-parse --show-toplevel)}"
PASSED=0
FAILED=0
TOTAL=0

pass() {
    PASSED=$((PASSED + 1))
    TOTAL=$((TOTAL + 1))
    echo "  ✓ $1"
}

fail() {
    FAILED=$((FAILED + 1))
    TOTAL=$((TOTAL + 1))
    echo "  ✗ $1"
}

check_exists() {
    local desc="$1"
    local path="$2"
    if [ -e "$REPO_ROOT/$path" ]; then
        pass "$desc"
    else
        fail "$desc: $path not found"
    fi
}

check_file_contains() {
    local desc="$1"
    local path="$2"
    local pattern="$3"
    if grep -q "$pattern" "$REPO_ROOT/$path" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc: pattern not found in $path"
    fi
}

check_file_not_contains() {
    local desc="$1"
    local path="$2"
    local pattern="$3"
    if ! grep -q "$pattern" "$REPO_ROOT/$path" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc: forbidden pattern found in $path"
    fi
}

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "Phase 39: Attention Envelope Guardrails"
echo "═══════════════════════════════════════════════════════════════════════════════"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 1: Package Structure (10 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 1: Package Structure ---"

check_exists "ADR exists" "docs/ADR/ADR-0076-phase39-attention-envelopes.md"
check_exists "Domain types exist" "pkg/domain/attentionenvelope/types.go"
check_exists "Engine exists" "internal/attentionenvelope/engine.go"
check_exists "Persist store exists" "internal/persist/attention_envelope_store.go"
check_exists "Demo tests exist" "internal/demo_phase39_attention_envelope/demo_test.go"

check_file_contains "Package declaration in domain" \
    "pkg/domain/attentionenvelope/types.go" "package attentionenvelope"
check_file_contains "Package declaration in engine" \
    "internal/attentionenvelope/engine.go" "package attentionenvelope"
check_file_contains "ADR reference in domain types" \
    "pkg/domain/attentionenvelope/types.go" "ADR-0076"
check_file_contains "ADR reference in engine" \
    "internal/attentionenvelope/engine.go" "ADR-0076"
check_file_contains "ADR reference in store" \
    "internal/persist/attention_envelope_store.go" "ADR-0076"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 2: No Goroutines (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 2: No Goroutines ---"

check_file_not_contains "No goroutines in domain types" \
    "pkg/domain/attentionenvelope/types.go" "go func"
check_file_not_contains "No goroutines in engine" \
    "internal/attentionenvelope/engine.go" "go func"
check_file_not_contains "No goroutines in store" \
    "internal/persist/attention_envelope_store.go" "go func"
check_file_not_contains "No channel creation in engine" \
    "internal/attentionenvelope/engine.go" "make(chan"
check_file_not_contains "No channel creation in store" \
    "internal/persist/attention_envelope_store.go" "make(chan"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 3: Clock Injection - No time.Now() (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 3: Clock Injection ---"

# Check that time.Now() is not used in business logic
# Note: Storelog timestamps are allowed in persist stores
if ! grep -v "^[[:space:]]*//\|Storelog\|storelog" "$REPO_ROOT/pkg/domain/attentionenvelope/types.go" | grep -q "time.Now()"; then
    pass "No time.Now() in domain types"
else
    fail "time.Now() found in domain types"
fi

if ! grep -v "^[[:space:]]*//\|Storelog\|storelog" "$REPO_ROOT/internal/attentionenvelope/engine.go" | grep -q "time.Now()"; then
    pass "No time.Now() in engine"
else
    fail "time.Now() found in engine"
fi

check_file_contains "Clock parameter in BuildEnvelope" \
    "internal/attentionenvelope/engine.go" "clock time.Time"
check_file_contains "Clock parameter in IsActive" \
    "internal/attentionenvelope/engine.go" "clock time.Time"
check_file_contains "Clock parameter in store GetActiveEnvelope" \
    "internal/persist/attention_envelope_store.go" "clock time.Time"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 4: stdlib only (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 4: stdlib only ---"

check_file_not_contains "No external imports in domain" \
    "pkg/domain/attentionenvelope/types.go" "github.com"
check_file_not_contains "No external imports in engine" \
    "internal/attentionenvelope/engine.go" "github.com"
check_file_not_contains "No external imports in store" \
    "internal/persist/attention_envelope_store.go" "github.com"
check_file_not_contains "No cloud SDK in engine" \
    "internal/attentionenvelope/engine.go" "cloud.google.com"
check_file_not_contains "No cloud SDK in store" \
    "internal/persist/attention_envelope_store.go" "cloud.google.com"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 5: No Execution/Transport Imports (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 5: No Execution/Transport Imports ---"

check_file_not_contains "No notification imports in engine" \
    "internal/attentionenvelope/engine.go" "notification"
check_file_not_contains "No push imports in engine" \
    "internal/attentionenvelope/engine.go" "apns\|fcm\|push"
check_file_not_contains "No execution imports in engine" \
    "internal/attentionenvelope/engine.go" "calendar/execution\|email/execution\|finance/execution"
check_file_not_contains "No delivery imports in engine" \
    "internal/attentionenvelope/engine.go" "interruptdelivery"
check_file_not_contains "No transport imports in engine" \
    "internal/attentionenvelope/engine.go" "pushtransport"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 6: Enum-Only Reasons (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 6: Enum-Only Reasons ---"

check_file_contains "EnvelopeKind enum exists" \
    "pkg/domain/attentionenvelope/types.go" "type EnvelopeKind string"
check_file_contains "DurationBucket enum exists" \
    "pkg/domain/attentionenvelope/types.go" "type DurationBucket string"
check_file_contains "EnvelopeReason enum exists" \
    "pkg/domain/attentionenvelope/types.go" "type EnvelopeReason string"
check_file_contains "EnvelopeState enum exists" \
    "pkg/domain/attentionenvelope/types.go" "type EnvelopeState string"
check_file_contains "Validate method on EnvelopeReason" \
    "pkg/domain/attentionenvelope/types.go" "func (r EnvelopeReason) Validate()"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 7: Domain Model Completeness (10 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 7: Domain Model Completeness ---"

check_file_contains "EnvelopeKindNone defined" \
    "pkg/domain/attentionenvelope/types.go" "EnvelopeKindNone"
check_file_contains "EnvelopeKindOnCall defined" \
    "pkg/domain/attentionenvelope/types.go" "EnvelopeKindOnCall"
check_file_contains "EnvelopeKindWorking defined" \
    "pkg/domain/attentionenvelope/types.go" "EnvelopeKindWorking"
check_file_contains "EnvelopeKindTravel defined" \
    "pkg/domain/attentionenvelope/types.go" "EnvelopeKindTravel"
check_file_contains "EnvelopeKindEmergency defined" \
    "pkg/domain/attentionenvelope/types.go" "EnvelopeKindEmergency"
check_file_contains "Duration15m defined" \
    "pkg/domain/attentionenvelope/types.go" "Duration15m"
check_file_contains "Duration1h defined" \
    "pkg/domain/attentionenvelope/types.go" "Duration1h"
check_file_contains "Duration4h defined" \
    "pkg/domain/attentionenvelope/types.go" "Duration4h"
check_file_contains "DurationDay defined" \
    "pkg/domain/attentionenvelope/types.go" "DurationDay"
check_file_contains "AttentionEnvelope struct exists" \
    "pkg/domain/attentionenvelope/types.go" "type AttentionEnvelope struct"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 8: Bounded Effects (10 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 8: Bounded Effects ---"

check_file_contains "computeHorizonShift function exists" \
    "internal/attentionenvelope/engine.go" "computeHorizonShift"
check_file_contains "computeMagnitudeBias function exists" \
    "internal/attentionenvelope/engine.go" "computeMagnitudeBias"
check_file_contains "ComputeCapDelta function exists" \
    "internal/attentionenvelope/engine.go" "ComputeCapDelta"

# Check bounded shift comments
check_file_contains "Horizon shift bounded to 1 step" \
    "internal/attentionenvelope/engine.go" "max 1 step\|exactly 1 step"
check_file_contains "Magnitude bias bounded to +1" \
    "internal/attentionenvelope/engine.go" "max +1\|exactly 1 bucket"
check_file_contains "Cap delta returns 0 or 1" \
    "internal/attentionenvelope/engine.go" "return 0\|return 1"

# Check horizon shift logic
check_file_contains "Later to Soon horizon shift" \
    "internal/attentionenvelope/engine.go" "HorizonLater"
check_file_contains "Soon to Now horizon shift" \
    "internal/attentionenvelope/engine.go" "HorizonSoon"
check_file_contains "Nothing to AFew magnitude shift" \
    "internal/attentionenvelope/engine.go" "MagnitudeNothing"
check_file_contains "AFew to Several magnitude shift" \
    "internal/attentionenvelope/engine.go" "MagnitudeAFew"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 9: Commerce Exclusion (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 9: Commerce Exclusion ---"

check_file_contains "Commerce exclusion in ApplyEnvelope" \
    "internal/attentionenvelope/engine.go" "CircleTypeCommerce"
check_file_contains "Commerce check returns input unchanged" \
    "internal/attentionenvelope/engine.go" "return input"
check_file_contains "ADR mentions commerce exclusion" \
    "docs/ADR/ADR-0076-phase39-attention-envelopes.md" "Commerce.*excluded\|commerce.*never\|Commerce NEVER"
check_file_contains "Commerce comment in engine" \
    "internal/attentionenvelope/engine.go" "[Cc]ommerce"
check_file_contains "Commerce handling in ApplyEnvelope" \
    "internal/attentionenvelope/engine.go" "Commerce exclusion\|NEVER escalate commerce"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 10: Storage Constraints (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 10: Storage Constraints ---"

check_file_contains "MaxEnvelopeRecords constant" \
    "pkg/domain/attentionenvelope/types.go" "MaxEnvelopeRecords"
check_file_contains "MaxRetentionDays constant" \
    "pkg/domain/attentionenvelope/types.go" "MaxRetentionDays"
check_file_contains "FIFO eviction in store" \
    "internal/persist/attention_envelope_store.go" "evict\|FIFO"
check_file_contains "One active per circle constraint" \
    "internal/persist/attention_envelope_store.go" "One active\|replaces any existing"
check_file_contains "Hash-only storage comment" \
    "internal/persist/attention_envelope_store.go" "Hash-only\|hash-only"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 11: Storelog Integration (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 11: Storelog Integration ---"

check_file_contains "RecordTypeEnvelopeStart in storelog" \
    "pkg/domain/storelog/log.go" "RecordTypeEnvelopeStart"
check_file_contains "RecordTypeEnvelopeStop in storelog" \
    "pkg/domain/storelog/log.go" "RecordTypeEnvelopeStop"
check_file_contains "RecordTypeEnvelopeExpire in storelog" \
    "pkg/domain/storelog/log.go" "RecordTypeEnvelopeExpire"
check_file_contains "RecordTypeEnvelopeApply in storelog" \
    "pkg/domain/storelog/log.go" "RecordTypeEnvelopeApply"
check_file_contains "Storelog integration in store" \
    "internal/persist/attention_envelope_store.go" "storelogRef"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 12: Events (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 12: Events ---"

check_file_contains "Phase39EnvelopeStarted event" \
    "pkg/events/events.go" "Phase39EnvelopeStarted"
check_file_contains "Phase39EnvelopeStopped event" \
    "pkg/events/events.go" "Phase39EnvelopeStopped"
check_file_contains "Phase39EnvelopeApplied event" \
    "pkg/events/events.go" "Phase39EnvelopeApplied"
check_file_contains "Phase39EnvelopeExpired event" \
    "pkg/events/events.go" "Phase39EnvelopeExpired"
check_file_contains "Phase39EnvelopeProofViewed event" \
    "pkg/events/events.go" "Phase39EnvelopeProofViewed"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 13: Engine Requirements (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 13: Engine Requirements ---"

check_file_contains "BuildEnvelope function" \
    "internal/attentionenvelope/engine.go" "func.*BuildEnvelope"
check_file_contains "IsActive function" \
    "internal/attentionenvelope/engine.go" "func.*IsActive"
check_file_contains "HasExpired function" \
    "internal/attentionenvelope/engine.go" "func.*HasExpired"
check_file_contains "ApplyEnvelope function" \
    "internal/attentionenvelope/engine.go" "func.*ApplyEnvelope"
check_file_contains "BuildReceipt function" \
    "internal/attentionenvelope/engine.go" "func.*BuildReceipt"

# ═══════════════════════════════════════════════════════════════════════════════
# Section 14: Canonical Strings (5 checks)
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Section 14: Canonical Strings ---"

check_file_contains "CanonicalString for AttentionEnvelope" \
    "pkg/domain/attentionenvelope/types.go" "func.*AttentionEnvelope.*CanonicalString"
check_file_contains "ComputeEnvelopeID" \
    "pkg/domain/attentionenvelope/types.go" "ComputeEnvelopeID"
check_file_contains "ComputeStatusHash" \
    "pkg/domain/attentionenvelope/types.go" "ComputeStatusHash"
check_file_contains "Pipe-delimited format" \
    "pkg/domain/attentionenvelope/types.go" "ENVELOPE|v1"
check_file_contains "SHA256 hashing" \
    "pkg/domain/attentionenvelope/types.go" "sha256.Sum256"

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"
echo "Summary: $PASSED passed, $FAILED failed, $TOTAL total"
echo "═══════════════════════════════════════════════════════════════════════════════"

if [ $FAILED -gt 0 ]; then
    exit 1
fi

exit 0
