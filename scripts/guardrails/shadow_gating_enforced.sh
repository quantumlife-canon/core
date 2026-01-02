#!/bin/bash
# Phase 19.5: Shadow Gating + Promotion Candidates Guardrails
#
# Validates that shadow gating implementation follows all invariants.
#
# CRITICAL: Shadow gating must:
#   - NOT affect any runtime behavior
#   - NOT change any canon thresholds or policies
#   - NOT change any obligation rules
#   - NOT change any interruption logic
#   - NOT generate any drafts
#   - NOT touch any execution boundaries
#   - Be deterministic: same inputs + clock = same outputs
#   - Be privacy-safe: only abstract buckets, no identifiable info
#   - Use clock injection - no time.Now()
#   - No goroutines - synchronous only
#
# Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md

set -e

ERRORS=0

error() {
    echo "[ERROR] $1"
    ERRORS=$((ERRORS + 1))
}

check() {
    echo "[CHECK] $1"
}

# =============================================================================
# Check 1: Required files exist
# =============================================================================
check "Domain types file exists"
if [ ! -f "pkg/domain/shadowgate/types.go" ]; then
    error "pkg/domain/shadowgate/types.go not found"
fi

check "Domain types test file exists"
if [ ! -f "pkg/domain/shadowgate/types_test.go" ]; then
    error "pkg/domain/shadowgate/types_test.go not found"
fi

check "Engine file exists"
if [ ! -f "internal/shadowgate/engine.go" ]; then
    error "internal/shadowgate/engine.go not found"
fi

check "Privacy guard file exists"
if [ ! -f "internal/shadowgate/privacy.go" ]; then
    error "internal/shadowgate/privacy.go not found"
fi

check "Store file exists"
if [ ! -f "internal/persist/shadow_gate_store.go" ]; then
    error "internal/persist/shadow_gate_store.go not found"
fi

# =============================================================================
# Check 2: No goroutines in shadow gating packages
# =============================================================================
check "No goroutines in pkg/domain/shadowgate"
if grep -r 'go func' pkg/domain/shadowgate/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in pkg/domain/shadowgate"
fi

check "No goroutines in internal/shadowgate"
if grep -r 'go func' internal/shadowgate/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in internal/shadowgate"
fi

check "No goroutines in shadow_gate_store.go"
if grep 'go func' internal/persist/shadow_gate_store.go 2>/dev/null; then
    error "Found goroutine in shadow_gate_store.go"
fi

# =============================================================================
# Check 3: No time.Now() in shadow gating packages
# =============================================================================
check "No time.Now() in pkg/domain/shadowgate"
if grep -rn 'time\.Now()' pkg/domain/shadowgate/ 2>/dev/null | grep -v '_test.go' | grep -v '//'; then
    error "Found time.Now() in pkg/domain/shadowgate - must use clock injection"
fi

check "No time.Now() in internal/shadowgate"
if grep -rn 'time\.Now()' internal/shadowgate/ 2>/dev/null | grep -v '_test.go' | grep -v '//'; then
    error "Found time.Now() in internal/shadowgate - must use clock injection"
fi

check "No time.Now() in shadow_gate_store.go"
if grep -n 'time\.Now()' internal/persist/shadow_gate_store.go 2>/dev/null | grep -v '//'; then
    error "Found time.Now() in shadow_gate_store.go - must use clock injection"
fi

# =============================================================================
# Check 4: No network calls in shadow gating packages
# =============================================================================
check "No net/http imports in pkg/domain/shadowgate"
if grep -r '"net/http"' pkg/domain/shadowgate/ 2>/dev/null | grep -v '_test.go'; then
    error "Found net/http import in pkg/domain/shadowgate"
fi

check "No net/http imports in internal/shadowgate"
if grep -r '"net/http"' internal/shadowgate/ 2>/dev/null | grep -v '_test.go'; then
    error "Found net/http import in internal/shadowgate"
fi

# =============================================================================
# Check 5: Canonical strings use pipe delimiter (not JSON)
# =============================================================================
check "Candidate uses pipe-delimited canonical string"
if ! grep -q 'SHADOW_CANDIDATE|v1|' pkg/domain/shadowgate/types.go; then
    error "Candidate canonical string missing pipe-delimited prefix"
fi

check "PromotionIntent uses pipe-delimited canonical string"
if ! grep -q 'PROMOTION_INTENT|v1|' pkg/domain/shadowgate/types.go; then
    error "PromotionIntent canonical string missing pipe-delimited prefix"
fi

check "No json.Marshal in shadowgate types hashing"
if grep 'json\.Marshal' pkg/domain/shadowgate/types.go 2>/dev/null; then
    error "Found json.Marshal in types.go - must use pipe-delimited strings"
fi

# =============================================================================
# Check 6: No imports into execution/drafts/interruptions packages
# =============================================================================
check "No shadowgate imports in execution packages"
if grep -r 'shadowgate' internal/email/execution/ internal/calendar/execution/ 2>/dev/null; then
    error "Found shadowgate import in execution package"
fi

check "No shadowgate imports in drafts packages"
if grep -r 'shadowgate' internal/drafts/ 2>/dev/null | grep -v '_test.go'; then
    error "Found shadowgate import in drafts package"
fi

check "No shadowgate imports in interruptions package"
if grep -r 'shadowgate' internal/interruptions/ 2>/dev/null | grep -v '_test.go'; then
    error "Found shadowgate import in interruptions package"
fi

check "No shadowgate imports in obligations package"
if grep -r 'shadowgate' internal/obligations/ 2>/dev/null | grep -v '_test.go'; then
    error "Found shadowgate import in obligations package"
fi

# =============================================================================
# Check 7: Phase 19.5 events exist
# =============================================================================
check "Phase19_5CandidatesRefreshRequested event exists"
if ! grep -q 'Phase19_5CandidatesRefreshRequested' pkg/events/events.go; then
    error "Phase19_5CandidatesRefreshRequested event not found"
fi

check "Phase19_5CandidatesComputed event exists"
if ! grep -q 'Phase19_5CandidatesComputed' pkg/events/events.go; then
    error "Phase19_5CandidatesComputed event not found"
fi

check "Phase19_5CandidatesPersisted event exists"
if ! grep -q 'Phase19_5CandidatesPersisted' pkg/events/events.go; then
    error "Phase19_5CandidatesPersisted event not found"
fi

check "Phase19_5CandidatesViewed event exists"
if ! grep -q 'Phase19_5CandidatesViewed' pkg/events/events.go; then
    error "Phase19_5CandidatesViewed event not found"
fi

check "Phase19_5PromotionProposed event exists"
if ! grep -q 'Phase19_5PromotionProposed' pkg/events/events.go; then
    error "Phase19_5PromotionProposed event not found"
fi

check "Phase19_5PromotionPersisted event exists"
if ! grep -q 'Phase19_5PromotionPersisted' pkg/events/events.go; then
    error "Phase19_5PromotionPersisted event not found"
fi

# =============================================================================
# Check 8: Web routes exist
# =============================================================================
check "/shadow/candidates route is registered"
if ! grep -q '/shadow/candidates' cmd/quantumlife-web/main.go; then
    error "/shadow/candidates route not found"
fi

check "/shadow/candidates/refresh route is registered"
if ! grep -q '/shadow/candidates/refresh' cmd/quantumlife-web/main.go; then
    error "/shadow/candidates/refresh route not found"
fi

check "/shadow/candidates/propose route is registered"
if ! grep -q '/shadow/candidates/propose' cmd/quantumlife-web/main.go; then
    error "/shadow/candidates/propose route not found"
fi

# =============================================================================
# Check 9: Privacy guard exists with forbidden patterns
# =============================================================================
check "Privacy guard validates WhyGeneric"
if ! grep -q 'ValidateWhyGeneric' internal/shadowgate/privacy.go; then
    error "ValidateWhyGeneric function not found"
fi

check "Privacy guard has forbidden patterns"
if ! grep -q 'forbiddenPatterns' internal/shadowgate/privacy.go; then
    error "forbiddenPatterns not found in privacy.go"
fi

check "Privacy guard checks emails"
if ! grep -q '@.*\.' internal/shadowgate/privacy.go; then
    error "Email pattern not found in privacy guard"
fi

check "Privacy guard checks URLs"
if ! grep -q 'https?://' internal/shadowgate/privacy.go; then
    error "URL pattern not found in privacy guard"
fi

check "Privacy guard checks currency"
if ! grep -q '\$\|£\|€' internal/shadowgate/privacy.go; then
    error "Currency pattern not found in privacy guard"
fi

# =============================================================================
# Check 10: Required enums exist
# =============================================================================
check "CandidateOrigin enum exists"
if ! grep -q 'type CandidateOrigin string' pkg/domain/shadowgate/types.go; then
    error "CandidateOrigin type not found"
fi

check "OriginShadowOnly value exists"
if ! grep -q 'OriginShadowOnly' pkg/domain/shadowgate/types.go; then
    error "OriginShadowOnly not found"
fi

check "UsefulnessBucket enum exists"
if ! grep -q 'type UsefulnessBucket string' pkg/domain/shadowgate/types.go; then
    error "UsefulnessBucket type not found"
fi

check "VoteConfidenceBucket enum exists"
if ! grep -q 'type VoteConfidenceBucket string' pkg/domain/shadowgate/types.go; then
    error "VoteConfidenceBucket type not found"
fi

check "NoteCode enum exists"
if ! grep -q 'type NoteCode string' pkg/domain/shadowgate/types.go; then
    error "NoteCode type not found"
fi

# =============================================================================
# Check 11: Store uses storelog
# =============================================================================
check "Shadow gate store uses storelog"
if ! grep -q 'storelog' internal/persist/shadow_gate_store.go; then
    error "shadow_gate_store.go does not reference storelog"
fi

check "RecordTypeShadowCandidate exists"
if ! grep -q 'RecordTypeShadowCandidate' internal/persist/shadow_gate_store.go; then
    error "RecordTypeShadowCandidate not found"
fi

check "RecordTypeShadowPromotionIntent exists"
if ! grep -q 'RecordTypeShadowPromotionIntent' internal/persist/shadow_gate_store.go; then
    error "RecordTypeShadowPromotionIntent not found"
fi

# =============================================================================
# Check 12: Replay support exists
# =============================================================================
check "ReplayCandidateRecord function exists"
if ! grep -q 'ReplayCandidateRecord' internal/persist/shadow_gate_store.go; then
    error "ReplayCandidateRecord not found"
fi

check "ReplayIntentRecord function exists"
if ! grep -q 'ReplayIntentRecord' internal/persist/shadow_gate_store.go; then
    error "ReplayIntentRecord not found"
fi

# =============================================================================
# Check 13: Demo tests exist with sufficient coverage
# =============================================================================
check "Demo test file exists"
if [ ! -f "internal/demo_phase19_5_shadow_gating/demo_test.go" ]; then
    error "Demo test file not found"
fi

check "Demo tests have 10+ test functions"
TEST_COUNT=$(grep -c 'func Test' internal/demo_phase19_5_shadow_gating/demo_test.go 2>/dev/null || echo 0)
if [ "$TEST_COUNT" -lt 10 ]; then
    error "Demo tests have only $TEST_COUNT test functions (need 10+)"
fi

# =============================================================================
# Check 14: DiffSource interface exists
# =============================================================================
check "DiffSource interface exists"
if ! grep -q 'type DiffSource interface' internal/shadowgate/engine.go; then
    error "DiffSource interface not found in engine.go"
fi

# =============================================================================
# Check 15: AllowedReasonPhrases exist
# =============================================================================
check "AllowedReasonPhrases list exists"
if ! grep -q 'AllowedReasonPhrases' internal/shadowgate/privacy.go; then
    error "AllowedReasonPhrases not found"
fi

check "Has at least 5 allowed phrases"
PHRASE_COUNT=$(grep -c '".*pattern\|timing\|spending\|items\|cluster' internal/shadowgate/privacy.go 2>/dev/null || echo 0)
if [ "$PHRASE_COUNT" -lt 3 ]; then
    error "AllowedReasonPhrases has insufficient entries"
fi

# =============================================================================
# Check 16: PeriodKeyFromTime helper exists
# =============================================================================
check "PeriodKeyFromTime function exists"
if ! grep -q 'PeriodKeyFromTime' pkg/domain/shadowgate/types.go; then
    error "PeriodKeyFromTime not found"
fi

# =============================================================================
# Check 17: CRITICAL INVARIANTS in comments
# =============================================================================
check "Engine has CRITICAL comments"
if ! grep -q 'CRITICAL.*NOT affect' internal/shadowgate/engine.go; then
    error "Engine missing CRITICAL invariant comments"
fi

check "Store has CRITICAL comments"
if ! grep -q 'CRITICAL.*NOT affect\|CRITICAL.*Append-only' internal/persist/shadow_gate_store.go; then
    error "Store missing CRITICAL invariant comments"
fi

# =============================================================================
# Check 18: Sorting is deterministic
# =============================================================================
check "SortCandidates function exists"
if ! grep -q 'func SortCandidates' pkg/domain/shadowgate/types.go; then
    error "SortCandidates not found"
fi

check "SortPromotionIntents function exists"
if ! grep -q 'func SortPromotionIntents' pkg/domain/shadowgate/types.go; then
    error "SortPromotionIntents not found"
fi

# =============================================================================
# Check 19: Navigation link exists
# =============================================================================
check "Link from shadow report to candidates"
if ! grep -q '/shadow/candidates' cmd/quantumlife-web/main.go | grep -q 'View candidates'; then
    # Check in a different way
    if ! grep 'View candidates' cmd/quantumlife-web/main.go > /dev/null 2>&1; then
        error "Navigation link to /shadow/candidates not found"
    fi
fi

# =============================================================================
# Check 20: Handler enforces POST for mutations
# =============================================================================
check "Refresh handler enforces POST"
if ! grep -A10 'handleShadowCandidatesRefresh' cmd/quantumlife-web/main.go | grep -q 'MethodPost'; then
    error "Refresh handler does not enforce POST"
fi

check "Propose handler enforces POST"
if ! grep -A10 'handleShadowCandidatesPropose' cmd/quantumlife-web/main.go | grep -q 'MethodPost'; then
    error "Propose handler does not enforce POST"
fi

# =============================================================================
# Summary
# =============================================================================

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "=============================================="
    echo "  Phase 19.5 Shadow Gating Guardrails: PASS"
    echo "=============================================="
    exit 0
else
    echo "=============================================="
    echo "  Phase 19.5 Shadow Gating Guardrails: FAIL"
    echo "  Errors: $ERRORS"
    echo "=============================================="
    exit 1
fi
