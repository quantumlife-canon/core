#!/bin/bash
# Phase 52: Proof Hub + Connected Status Guardrails
#
# CRITICAL INVARIANTS:
# - NO POWER: Observation/proof only, no execution, no delivery.
# - HASH-ONLY: Only hashes, buckets, status flags stored/rendered.
# - NO TIMESTAMPS: Only recency buckets (never, recent, stale).
# - NO COUNTS: Only magnitude buckets (nothing, a_few, several).
# - DETERMINISTIC: Same inputs + same clock = same status hash.
# - PIPE-DELIMITED: Canonical strings use pipe format, not JSON.
# - NO FORBIDDEN PATTERNS: No merchant, amount, email, url, etc.
#
# Reference: docs/ADR/ADR-0090-phase52-proof-hub-connected-status.md

set -e

DOMAIN_FILE="pkg/domain/proofhub/types.go"
ENGINE_FILE="internal/proofhub/engine.go"
STORE_FILE="internal/persist/proofhub_ack_store.go"
DEMO_DIR="internal/demo_phase52_proof_hub"
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
echo "=== Domain Types - Enum Validation ==="
check "ProviderStatus StatusUnknown defined" "grep -q 'StatusUnknown.*ProviderStatus.*status_unknown' $DOMAIN_FILE"
check "ProviderStatus StatusOK defined" "grep -q 'StatusOK.*ProviderStatus.*status_ok' $DOMAIN_FILE"
check "ProviderStatus StatusMissing defined" "grep -q 'StatusMissing.*ProviderStatus.*status_missing' $DOMAIN_FILE"
check "ProviderStatus StatusError defined" "grep -q 'StatusError.*ProviderStatus.*status_error' $DOMAIN_FILE"
check "ConnectStatus ConnectNo defined" "grep -q 'ConnectNo.*ConnectStatus.*connect_no' $DOMAIN_FILE"
check "ConnectStatus ConnectYes defined" "grep -q 'ConnectYes.*ConnectStatus.*connect_yes' $DOMAIN_FILE"
check "SyncRecencyBucket SyncNever defined" "grep -q 'SyncNever.*SyncRecencyBucket.*sync_never' $DOMAIN_FILE"
check "SyncRecencyBucket SyncRecent defined" "grep -q 'SyncRecent.*SyncRecencyBucket.*sync_recent' $DOMAIN_FILE"
check "SyncRecencyBucket SyncStale defined" "grep -q 'SyncStale.*SyncRecencyBucket.*sync_stale' $DOMAIN_FILE"
check "MagnitudeBucket MagNothing defined" "grep -q 'MagNothing.*MagnitudeBucket.*mag_nothing' $DOMAIN_FILE"
check "MagnitudeBucket MagAFew defined" "grep -q 'MagAFew.*MagnitudeBucket.*mag_a_few' $DOMAIN_FILE"
check "MagnitudeBucket MagSeveral defined" "grep -q 'MagSeveral.*MagnitudeBucket.*mag_several' $DOMAIN_FILE"
check "ProofHubSectionKind SectionIdentity defined" "grep -q 'SectionIdentity.*ProofHubSectionKind.*section_identity' $DOMAIN_FILE"
check "ProofHubSectionKind SectionConnections defined" "grep -q 'SectionConnections.*ProofHubSectionKind.*section_connections' $DOMAIN_FILE"
check "ProofHubSectionKind SectionSync defined" "grep -q 'SectionSync.*ProofHubSectionKind.*section_sync' $DOMAIN_FILE"
check "ProofHubSectionKind SectionShadow defined" "grep -q 'SectionShadow.*ProofHubSectionKind.*section_shadow' $DOMAIN_FILE"
check "ProofHubSectionKind SectionLedger defined" "grep -q 'SectionLedger.*ProofHubSectionKind.*section_ledger' $DOMAIN_FILE"
check "ProofHubSectionKind SectionInvariants defined" "grep -q 'SectionInvariants.*ProofHubSectionKind.*section_invariants' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - Struct Fields ==="
check "ProofHubInputs struct defined" "grep -q 'type ProofHubInputs struct' $DOMAIN_FILE"
check "ProofHubBadge struct defined" "grep -q 'type ProofHubBadge struct' $DOMAIN_FILE"
check "ProofHubSection struct defined" "grep -q 'type ProofHubSection struct' $DOMAIN_FILE"
check "ProofHubPage struct defined" "grep -q 'type ProofHubPage struct' $DOMAIN_FILE"
check "ProofHubCue struct defined" "grep -q 'type ProofHubCue struct' $DOMAIN_FILE"
check "ProofHubAck struct defined" "grep -q 'type ProofHubAck struct' $DOMAIN_FILE"
check "ProofHubAction struct defined" "grep -q 'type ProofHubAction struct' $DOMAIN_FILE"

echo ""
echo "=== Domain Types - Required Methods ==="
check "ProviderStatus.Validate() method exists" "grep -q 'func (s ProviderStatus) Validate()' $DOMAIN_FILE"
check "ConnectStatus.Validate() method exists" "grep -q 'func (c ConnectStatus) Validate()' $DOMAIN_FILE"
check "SyncRecencyBucket.Validate() method exists" "grep -q 'func (s SyncRecencyBucket) Validate()' $DOMAIN_FILE"
check "MagnitudeBucket.Validate() method exists" "grep -q 'func (m MagnitudeBucket) Validate()' $DOMAIN_FILE"
check "ProofHubInputs.CanonicalString() method exists" "grep -q 'func (i ProofHubInputs) CanonicalString()' $DOMAIN_FILE"
check "ProofHubInputs.Validate() method exists" "grep -q 'func (i ProofHubInputs) Validate()' $DOMAIN_FILE"
check "ProofHubPage.CanonicalString() method exists" "grep -q 'func (p ProofHubPage) CanonicalString()' $DOMAIN_FILE"
check "ProofHubAck.CanonicalString() method exists" "grep -q 'func (a ProofHubAck) CanonicalString()' $DOMAIN_FILE"
check "HashProofHubStatus function exists" "grep -q 'func HashProofHubStatus' $DOMAIN_FILE"

echo ""
echo "=== Pipe-Delimited Canonical Strings ==="
check "Canonical strings use pipe delimiter" "grep -q 'strings.Join.*\"|\"' $DOMAIN_FILE"
check "Inputs canonical format correct" "grep -q 'v1.*circle=.*period=' $DOMAIN_FILE"
check "No JSON marshaling in domain package" "! grep -q 'json.Marshal' $DOMAIN_FILE"

echo ""
echo "=== No time.Now() in Domain/Engine/Store ==="
check_not "No time.Now() in domain package" "grep -q 'time\.Now()' $DOMAIN_FILE"
check_not "No time.Now() in engine package" "grep -q 'time\.Now()' $ENGINE_FILE"
check_not "No time.Now() in store" "grep -q 'time\.Now()' $STORE_FILE"

echo ""
echo "=== No Goroutines ==="
check_not "No goroutines in domain package" "grep -q 'go func' $DOMAIN_FILE"
check_not "No goroutines in engine package" "grep -q 'go func' $ENGINE_FILE"
check_not "No goroutines in store" "grep -q 'go func' $STORE_FILE"

echo ""
echo "=== Forbidden Imports (No Power) ==="
FORBIDDEN_IMPORTS="pressuredecision interruptpolicy interruptpreview pushtransport interruptdelivery enforcementclamp vendorcontract"
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
check "Engine.BuildInputs() method exists" "grep -q 'func (e \*Engine) BuildInputs' $ENGINE_FILE"
check "Engine.BuildPage() method exists" "grep -q 'func (e \*Engine) BuildPage' $ENGINE_FILE"
check "Engine.BuildCue() method exists" "grep -q 'func (e \*Engine) BuildCue' $ENGINE_FILE"
check "Engine.ShouldShowCue() method exists" "grep -q 'func (e \*Engine) ShouldShowCue' $ENGINE_FILE"
check "ComputePeriodKey function exists" "grep -q 'func ComputePeriodKey' $ENGINE_FILE"

echo ""
echo "=== Engine Sections ==="
check "Identity section builder exists" "grep -q 'buildIdentitySection' $ENGINE_FILE"
check "Connections section builder exists" "grep -q 'buildConnectionsSection' $ENGINE_FILE"
check "Sync section builder exists" "grep -q 'buildSyncSection' $ENGINE_FILE"
check "Shadow section builder exists" "grep -q 'buildShadowSection' $ENGINE_FILE"
check "Ledger section builder exists" "grep -q 'buildLedgerSection' $ENGINE_FILE"
check "Invariants section builder exists" "grep -q 'buildInvariantsSection' $ENGINE_FILE"

echo ""
echo "=== Append-Only Store ==="
check "Store.RecordDismissed() method exists" "grep -q 'func (s \*ProofHubAckStore) RecordDismissed' $STORE_FILE"
check "Store.IsDismissed() method exists" "grep -q 'func (s \*ProofHubAckStore) IsDismissed' $STORE_FILE"
check "Store.LastAckedStatusHash() method exists" "grep -q 'func (s \*ProofHubAckStore) LastAckedStatusHash' $STORE_FILE"
check "Store has dedup index" "grep -q 'dedupIndex' $STORE_FILE"
check "Store has max entries constant (200)" "grep -q 'ProofHubAckMaxEntries.*=.*200' $STORE_FILE"
check "Store has max retention days constant (30)" "grep -q 'ProofHubAckMaxRetentionDays.*=.*30' $STORE_FILE"
check "Store has eviction method" "grep -q 'evictOldEntriesLocked' $STORE_FILE"

echo ""
echo "=== Events ==="
check "Phase52ProofHubRequested event defined" "grep -q 'Phase52ProofHubRequested.*EventType.*phase52.proofhub.requested' $EVENTS_FILE"
check "Phase52ProofHubRendered event defined" "grep -q 'Phase52ProofHubRendered.*EventType.*phase52.proofhub.rendered' $EVENTS_FILE"
check "Phase52ProofHubAcknowledged event defined" "grep -q 'Phase52ProofHubAcknowledged.*EventType.*phase52.proofhub.acknowledged' $EVENTS_FILE"
check "Phase52ProofHubCueComputed event defined" "grep -q 'Phase52ProofHubCueComputed.*EventType.*phase52.proofhub.cue.computed' $EVENTS_FILE"
check "Phase52ProofHubInputsBuilt event defined" "grep -q 'Phase52ProofHubInputsBuilt.*EventType.*phase52.proofhub.inputs.built' $EVENTS_FILE"
check "Phase52ProofHubStatusHashed event defined" "grep -q 'Phase52ProofHubStatusHashed.*EventType.*phase52.proofhub.status.hashed' $EVENTS_FILE"

echo ""
echo "=== Storelog Record Types ==="
check "RecordTypeProofHubAck defined" "grep -q 'RecordTypeProofHubAck.*=.*PROOF_HUB_ACK' $STORELOG_FILE"

echo ""
echo "=== Web Routes ==="
check "GET /proof/hub route defined" "grep -q '/proof/hub.*handleProofHub' $WEB_FILE"
check "POST /proof/hub/dismiss route defined" "grep -q '/proof/hub/dismiss.*handleProofHubDismiss' $WEB_FILE"

echo ""
echo "=== Web Handlers ==="
check "handleProofHub function exists" "grep -q 'func (s \*Server) handleProofHub' $WEB_FILE"
check "handleProofHubDismiss function exists" "grep -q 'func (s \*Server) handleProofHubDismiss' $WEB_FILE"
check "renderProofHubPage function exists" "grep -q 'func (s \*Server) renderProofHubPage' $WEB_FILE"

echo ""
echo "=== Forbidden Patterns in Domain ==="
# Check that domain types don't contain forbidden patterns in struct fields
check_not "No forbidden pattern 'vendorID' in domain" "grep -i 'vendorID' $DOMAIN_FILE"
check_not "No forbidden pattern 'packID' in domain" "grep -i 'packID' $DOMAIN_FILE"
check_not "No forbidden pattern 'merchant' in domain structs" "grep -E 'Merchant\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'sender' in domain structs" "grep -E 'Sender\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'subject' in domain structs" "grep -E 'Subject\s+string' $DOMAIN_FILE"
check_not "No forbidden pattern 'amount' in domain structs" "grep -E 'Amount\s+(int|float|string)' $DOMAIN_FILE"
check_not "No forbidden pattern 'currency' in domain structs" "grep -E 'Currency\s+string' $DOMAIN_FILE"

echo ""
echo "=== Web Handlers Reject Forbidden Params ==="
check "View handler checks forbidden params" "grep -A20 'handleProofHub' $WEB_FILE | grep -q 'forbiddenParams'"
check "Dismiss handler checks forbidden params" "grep -A20 'handleProofHubDismiss' $WEB_FILE | grep -q 'forbiddenParams'"

echo ""
echo "=== Status Hash Rendered ==="
check "StatusHash field exists in domain" "grep -q 'StatusHash.*string' $DOMAIN_FILE"
check "Status hash is rendered in page" "grep -A50 'renderProofHubPage' $WEB_FILE | grep -q 'StatusHash'"

echo ""
echo "=== Deterministic Ordering ==="
check "Domain uses SortSections for determinism" "grep -q 'func SortSections' $DOMAIN_FILE"
check "Domain uses SortBadges for determinism" "grep -q 'func SortBadges' $DOMAIN_FILE"
check "Engine calls SortSections" "grep -q 'domain.SortSections' $ENGINE_FILE"

echo ""
echo "=== SHA256 Hashing ==="
check "Domain uses SHA256 for status hash" "grep -q 'crypto/sha256' $DOMAIN_FILE"
check "HashProofHubStatus uses SHA256" "grep -A10 'func HashProofHubStatus' $DOMAIN_FILE | grep -q 'sha256.Sum256'"

echo ""
echo "=== Demo Tests ==="
if [ -f "$DEMO_DIR/demo_test.go" ]; then
    TEST_COUNT=$(grep -c 'func Test' "$DEMO_DIR/demo_test.go" || echo "0")
    if [ "$TEST_COUNT" -ge 24 ]; then
        pass "Demo tests have >= 24 test functions ($TEST_COUNT found)"
    else
        fail "Demo tests should have >= 24 test functions ($TEST_COUNT found)"
    fi
else
    fail "Demo test file exists"
fi

echo ""
echo "=== No Decision Logic Imports in Phase 52 ==="
DECISION_IMPORTS="pressuredecision interruptpolicy interruptpreview pushtransport interruptdelivery enforcementclamp"
for imp in $DECISION_IMPORTS; do
    check_not "Phase 52 does not import: $imp" "grep -rq '\"quantumlife/internal/$imp\"' $DOMAIN_FILE $ENGINE_FILE $STORE_FILE"
done

echo ""
echo "==========================================="
echo "Phase 52 Guardrails Summary"
echo "==========================================="
echo "Total checks: $((PASS_COUNT + FAIL_COUNT))"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -eq 0 ]; then
    echo ""
    echo "All Phase 52 guardrails passed!"
    exit 0
else
    echo ""
    echo "ERROR: $FAIL_COUNT guardrail(s) failed"
    exit 1
fi
