#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════
# Phase 34: Permitted Interrupt Preview Guardrails
# ═══════════════════════════════════════════════════════════════════════════
#
# CRITICAL INVARIANTS:
#   - NO external signals (no push, no email, no SMS, no OS notifications)
#   - Web-only preview. User-initiated.
#   - Hash-only, bucket-only. No raw identifiers.
#   - Deterministic: same inputs => same outputs.
#   - No goroutines in internal/ or pkg/
#   - No time.Now() — clock injection required
#   - stdlib-only in internal/ and pkg/
#   - Commerce always excluded
#   - Single-whisper rule respected
#
# Reference: docs/ADR/ADR-0070-phase34-interrupt-preview-web-only.md
# ═══════════════════════════════════════════════════════════════════════════

set -e

REPO_ROOT="${REPO_ROOT:-$(git rev-parse --show-toplevel)}"
PASSED=0
FAILED=0
TOTAL=0

pass() {
    echo "✓ $1"
    PASSED=$((PASSED+1))
    TOTAL=$((TOTAL+1))
}

fail() {
    echo "✗ $1"
    FAILED=$((FAILED+1))
    TOTAL=$((TOTAL+1))
}

check_exists() {
    if [[ -e "$REPO_ROOT/$2" ]]; then
        pass "$1"
    else
        fail "$1: $2 not found"
    fi
}

check_file_contains() {
    if grep -q "$3" "$REPO_ROOT/$2" 2>/dev/null; then
        pass "$1"
    else
        fail "$1: pattern not found in $2"
    fi
}

check_file_not_contains() {
    if ! grep -q "$3" "$REPO_ROOT/$2" 2>/dev/null; then
        pass "$1"
    else
        fail "$1: forbidden pattern found in $2"
    fi
}

check_no_pattern_in_dir() {
    local desc="$1"
    local dir="$2"
    local pattern="$3"
    local exclude="${4:-}"

    if [[ -n "$exclude" ]]; then
        if grep -r "$pattern" "$REPO_ROOT/$dir" --include="*.go" 2>/dev/null | grep -v "$exclude" | grep -v "_test.go" | head -1 | grep -q .; then
            fail "$desc"
        else
            pass "$desc"
        fi
    else
        if grep -r "$pattern" "$REPO_ROOT/$dir" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
            fail "$desc"
        else
            pass "$desc"
        fi
    fi
}

echo "═══════════════════════════════════════════════════════════════════════════"
echo "Phase 34: Permitted Interrupt Preview Guardrails"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""

# ═══════════════════════════════════════════════════════════════════════════
# Section 1: Package Structure
# ═══════════════════════════════════════════════════════════════════════════
echo "--- Package Structure ---"

check_exists "ADR-0070 exists" "docs/ADR/ADR-0070-phase34-interrupt-preview-web-only.md"
check_exists "Domain types exist" "pkg/domain/interruptpreview/types.go"
check_exists "Engine exists" "internal/interruptpreview/engine.go"
check_exists "Preview ack store exists" "internal/persist/interrupt_preview_ack_store.go"
check_exists "Demo tests exist" "internal/demo_phase34_interrupt_preview/demo_test.go"

# ═══════════════════════════════════════════════════════════════════════════
# Section 2: NO External Signals (Web-only)
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- NO External Signals (Web-only) ---"

# Check for forbidden external signal patterns in internal package
check_no_pattern_in_dir "No 'notify' in interruptpreview" "internal/interruptpreview" "notify"
check_no_pattern_in_dir "No 'push' in interruptpreview" "internal/interruptpreview" "[Pp]ush[Nn]otif"
check_no_pattern_in_dir "No 'apns' in interruptpreview" "internal/interruptpreview" "[aA][pP][nN][sS]"
check_no_pattern_in_dir "No 'fcm' in interruptpreview" "internal/interruptpreview" "[fF][cC][mM]"
check_no_pattern_in_dir "No 'sendEmail' in interruptpreview" "internal/interruptpreview" "sendEmail\\|SendEmail"
check_no_pattern_in_dir "No 'webhook' in interruptpreview" "internal/interruptpreview" "webhook"
check_no_pattern_in_dir "No 'twilio' in interruptpreview" "internal/interruptpreview" "[tT]wilio"

# Check for forbidden external signal patterns in domain package
check_no_pattern_in_dir "No 'notify' in domain" "pkg/domain/interruptpreview" "notify"
check_no_pattern_in_dir "No 'push' in domain" "pkg/domain/interruptpreview" "[Pp]ush[Nn]otif"
check_no_pattern_in_dir "No 'apns' in domain" "pkg/domain/interruptpreview" "[aA][pP][nN][sS]"
check_no_pattern_in_dir "No 'fcm' in domain" "pkg/domain/interruptpreview" "[fF][cC][mM]"
check_no_pattern_in_dir "No 'webhook' in domain" "pkg/domain/interruptpreview" "webhook"

# ═══════════════════════════════════════════════════════════════════════════
# Section 3: No Goroutines
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- No Goroutines ---"

check_no_pattern_in_dir "No goroutines in interruptpreview" "internal/interruptpreview" "go func"
check_no_pattern_in_dir "No goroutines in domain" "pkg/domain/interruptpreview" "go func"
check_no_pattern_in_dir "No goroutines in persist (preview)" "internal/persist/interrupt_preview" "go func"

# ═══════════════════════════════════════════════════════════════════════════
# Section 4: Clock Injection (No time.Now())
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Clock Injection ---"

# Check for time.Now() in business logic
if grep -r "time\.Now()" "$REPO_ROOT/internal/interruptpreview" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No time.Now() in interruptpreview engine"
else
    pass "No time.Now() in interruptpreview engine"
fi

if grep -r "time\.Now()" "$REPO_ROOT/pkg/domain/interruptpreview" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No time.Now() in domain types"
else
    pass "No time.Now() in domain types"
fi

# ═══════════════════════════════════════════════════════════════════════════
# Section 5: stdlib-only
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- stdlib-only ---"

# Check for forbidden imports in internal/interruptpreview
if grep -r "github.com/" "$REPO_ROOT/internal/interruptpreview" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No external imports in interruptpreview"
else
    pass "No external imports in interruptpreview"
fi

if grep -r "github.com/" "$REPO_ROOT/pkg/domain/interruptpreview" --include="*.go" 2>/dev/null | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No external imports in domain"
else
    pass "No external imports in domain"
fi

# ═══════════════════════════════════════════════════════════════════════════
# Section 6: Domain Model Completeness
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Domain Model Completeness ---"

# Bucket types
check_file_contains "CircleTypeBucket type exists" "pkg/domain/interruptpreview/types.go" "type CircleTypeBucket string"
check_file_contains "HorizonBucket type exists" "pkg/domain/interruptpreview/types.go" "type HorizonBucket string"
check_file_contains "MagnitudeBucket type exists" "pkg/domain/interruptpreview/types.go" "type MagnitudeBucket string"
check_file_contains "ReasonBucket type exists" "pkg/domain/interruptpreview/types.go" "type ReasonBucket string"
check_file_contains "AllowanceBucket type exists" "pkg/domain/interruptpreview/types.go" "type AllowanceBucket string"

# Bucket constants
check_file_contains "CircleTypeHuman constant" "pkg/domain/interruptpreview/types.go" "CircleTypeHuman"
check_file_contains "CircleTypeInstitution constant" "pkg/domain/interruptpreview/types.go" "CircleTypeInstitution"
check_file_contains "HorizonNow constant" "pkg/domain/interruptpreview/types.go" "HorizonNow"
check_file_contains "HorizonSoon constant" "pkg/domain/interruptpreview/types.go" "HorizonSoon"
check_file_contains "HorizonLater constant" "pkg/domain/interruptpreview/types.go" "HorizonLater"
check_file_contains "MagnitudeNothing constant" "pkg/domain/interruptpreview/types.go" "MagnitudeNothing"
check_file_contains "MagnitudeAFew constant" "pkg/domain/interruptpreview/types.go" "MagnitudeAFew"
check_file_contains "MagnitudeSeveral constant" "pkg/domain/interruptpreview/types.go" "MagnitudeSeveral"

# Core types
check_file_contains "PreviewCandidate type exists" "pkg/domain/interruptpreview/types.go" "type PreviewCandidate struct"
check_file_contains "PreviewCue type exists" "pkg/domain/interruptpreview/types.go" "type PreviewCue struct"
check_file_contains "PreviewPage type exists" "pkg/domain/interruptpreview/types.go" "type PreviewPage struct"
check_file_contains "PreviewProofPage type exists" "pkg/domain/interruptpreview/types.go" "type PreviewProofPage struct"
check_file_contains "PreviewAck type exists" "pkg/domain/interruptpreview/types.go" "type PreviewAck struct"
check_file_contains "PreviewInput type exists" "pkg/domain/interruptpreview/types.go" "type PreviewInput struct"

# Ack kinds
check_file_contains "PreviewAckKind type exists" "pkg/domain/interruptpreview/types.go" "type PreviewAckKind string"
check_file_contains "AckViewed constant" "pkg/domain/interruptpreview/types.go" "AckViewed"
check_file_contains "AckDismissed constant" "pkg/domain/interruptpreview/types.go" "AckDismissed"
check_file_contains "AckHeld constant" "pkg/domain/interruptpreview/types.go" "AckHeld"

# Methods
check_file_contains "CanonicalString method on PreviewAck" "pkg/domain/interruptpreview/types.go" "func (a \\*PreviewAck) CanonicalString"
check_file_contains "ComputeAckID method on PreviewAck" "pkg/domain/interruptpreview/types.go" "func (a \\*PreviewAck) ComputeAckID"
check_file_contains "Validate method on PreviewCandidate" "pkg/domain/interruptpreview/types.go" "func (c \\*PreviewCandidate) Validate"
check_file_contains "DisplayLabel on CircleTypeBucket" "pkg/domain/interruptpreview/types.go" "func (c CircleTypeBucket) DisplayLabel"
check_file_contains "DisplayLabel on HorizonBucket" "pkg/domain/interruptpreview/types.go" "func (h HorizonBucket) DisplayLabel"

# ═══════════════════════════════════════════════════════════════════════════
# Section 7: Engine Requirements
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Engine Requirements ---"

check_file_contains "Engine type exists" "internal/interruptpreview/engine.go" "type Engine struct"
check_file_contains "NewEngine function exists" "internal/interruptpreview/engine.go" "func NewEngine"
check_file_contains "SelectCandidate method exists" "internal/interruptpreview/engine.go" "func (e \\*Engine) SelectCandidate"
check_file_contains "BuildCue method exists" "internal/interruptpreview/engine.go" "func (e \\*Engine) BuildCue"
check_file_contains "BuildPage method exists" "internal/interruptpreview/engine.go" "func (e \\*Engine) BuildPage"
check_file_contains "BuildProofPage method exists" "internal/interruptpreview/engine.go" "func (e \\*Engine) BuildProofPage"
check_file_contains "ShouldShowCue method exists" "internal/interruptpreview/engine.go" "func (e \\*Engine) ShouldShowCue"
check_file_contains "FilterCommerce method exists" "internal/interruptpreview/engine.go" "func (e \\*Engine) FilterCommerce"

# ═══════════════════════════════════════════════════════════════════════════
# Section 8: Persistence Requirements
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Persistence Requirements ---"

check_file_contains "InterruptPreviewAckStore type exists" "internal/persist/interrupt_preview_ack_store.go" "type InterruptPreviewAckStore struct"
check_file_contains "Append method on store" "internal/persist/interrupt_preview_ack_store.go" "func (s \\*InterruptPreviewAckStore) Append"
check_file_contains "IsDismissed method" "internal/persist/interrupt_preview_ack_store.go" "func (s \\*InterruptPreviewAckStore) IsDismissed"
check_file_contains "IsHeld method" "internal/persist/interrupt_preview_ack_store.go" "func (s \\*InterruptPreviewAckStore) IsHeld"
check_file_contains "GetAck method" "internal/persist/interrupt_preview_ack_store.go" "func (s \\*InterruptPreviewAckStore) GetAck"
check_file_contains "EvictOldPeriods method" "internal/persist/interrupt_preview_ack_store.go" "func (s \\*InterruptPreviewAckStore) EvictOldPeriods"

# ═══════════════════════════════════════════════════════════════════════════
# Section 9: Events
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Events ---"

check_file_contains "Phase34InterruptPreviewCueComputed event" "pkg/events/events.go" "Phase34InterruptPreviewCueComputed"
check_file_contains "Phase34InterruptPreviewCueShown event" "pkg/events/events.go" "Phase34InterruptPreviewCueShown"
check_file_contains "Phase34InterruptPreviewPageRequested event" "pkg/events/events.go" "Phase34InterruptPreviewPageRequested"
check_file_contains "Phase34InterruptPreviewPageRendered event" "pkg/events/events.go" "Phase34InterruptPreviewPageRendered"
check_file_contains "Phase34InterruptPreviewViewed event" "pkg/events/events.go" "Phase34InterruptPreviewViewed"
check_file_contains "Phase34InterruptPreviewDismissed event" "pkg/events/events.go" "Phase34InterruptPreviewDismissed"
check_file_contains "Phase34InterruptPreviewHeld event" "pkg/events/events.go" "Phase34InterruptPreviewHeld"
check_file_contains "Phase34InterruptPreviewProofRequested event" "pkg/events/events.go" "Phase34InterruptPreviewProofRequested"
check_file_contains "Phase34InterruptPreviewProofRendered event" "pkg/events/events.go" "Phase34InterruptPreviewProofRendered"

# ═══════════════════════════════════════════════════════════════════════════
# Section 10: Storelog Record Types
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Storelog Record Types ---"

check_file_contains "RecordTypeInterruptPreviewAck exists" "pkg/domain/storelog/log.go" "RecordTypeInterruptPreviewAck"

# ═══════════════════════════════════════════════════════════════════════════
# Section 11: No Forbidden UI Tokens (Privacy)
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- No Forbidden UI Tokens ---"

# Check domain types for forbidden tokens (excluding comments and ForbiddenPatterns validation)
check_no_pattern_in_dir "No email patterns in domain" "pkg/domain/interruptpreview" "@[a-zA-Z0-9]"
check_no_pattern_in_dir "No http:// in domain" "pkg/domain/interruptpreview" "http://"

# Currency symbols appear in ForbiddenPatterns regex definitions - that's correct usage
# These checks verify no raw currency amounts appear in code logic (excluding regex patterns)
if grep -r "£" "$REPO_ROOT/pkg/domain/interruptpreview" --include="*.go" 2>/dev/null | grep -v "regexp.MustCompile" | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No amounts (£) in domain logic"
else
    pass "No amounts (£) in domain logic"
fi

if grep -r "€" "$REPO_ROOT/pkg/domain/interruptpreview" --include="*.go" 2>/dev/null | grep -v "regexp.MustCompile" | grep -v "_test.go" | head -1 | grep -q .; then
    fail "No amounts (€) in domain logic"
else
    pass "No amounts (€) in domain logic"
fi

check_no_pattern_in_dir "No amounts ($) in domain logic" "pkg/domain/interruptpreview" '\\$[0-9]'

# ═══════════════════════════════════════════════════════════════════════════
# Section 12: Hash-only Storage
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Hash-only Storage ---"

check_file_contains "CircleIDHash field in PreviewAck" "pkg/domain/interruptpreview/types.go" "CircleIDHash"
check_file_contains "CandidateHash field in PreviewAck" "pkg/domain/interruptpreview/types.go" "CandidateHash"
check_file_contains "StatusHash field in PreviewCue" "pkg/domain/interruptpreview/types.go" "StatusHash"
check_file_contains "ComputeStatusHash method" "pkg/domain/interruptpreview/types.go" "ComputeStatusHash"

# ═══════════════════════════════════════════════════════════════════════════
# Section 13: Commerce Exclusion
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Commerce Exclusion ---"

# Commerce should never be in valid circle types
check_file_not_contains "No commerce in ValidCircleTypes" "pkg/domain/interruptpreview/types.go" "CircleTypeCommerce.*:.*true"
check_file_contains "FilterCommerce filters commerce" "internal/interruptpreview/engine.go" "commerce"

# ═══════════════════════════════════════════════════════════════════════════
# Section 14: Demo Tests
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Demo Tests ---"

# Count test functions
TEST_COUNT=$(grep -c "^func Test" "$REPO_ROOT/internal/demo_phase34_interrupt_preview/demo_test.go" 2>/dev/null || echo "0")
if [[ "$TEST_COUNT" -ge 18 ]]; then
    pass "At least 18 test functions ($TEST_COUNT found)"
else
    fail "At least 18 test functions (only $TEST_COUNT found)"
fi

check_file_contains "Determinism test exists" "internal/demo_phase34_interrupt_preview/demo_test.go" "TestDeterminism"
check_file_contains "No candidates test exists" "internal/demo_phase34_interrupt_preview/demo_test.go" "TestNoCandidates"
check_file_contains "Dismissed test exists" "internal/demo_phase34_interrupt_preview/demo_test.go" "TestDismissed"
check_file_contains "Held test exists" "internal/demo_phase34_interrupt_preview/demo_test.go" "TestHeld"
check_file_contains "Commerce filter test exists" "internal/demo_phase34_interrupt_preview/demo_test.go" "TestFilterCommerce"
check_file_contains "Ack store test exists" "internal/demo_phase34_interrupt_preview/demo_test.go" "TestAckStore"

# ═══════════════════════════════════════════════════════════════════════════
# Section 15: Web Routes (cmd/quantumlife-web/main.go)
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "--- Web Routes ---"

check_file_contains "/interrupts/preview route" "cmd/quantumlife-web/main.go" "/interrupts/preview"
check_file_contains "/interrupts/preview/dismiss route" "cmd/quantumlife-web/main.go" "/interrupts/preview/dismiss"
check_file_contains "/interrupts/preview/hold route" "cmd/quantumlife-web/main.go" "/interrupts/preview/hold"
check_file_contains "/proof/interrupts/preview route" "cmd/quantumlife-web/main.go" "/proof/interrupts/preview"

# ═══════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "Summary: $PASSED passed, $FAILED failed, $TOTAL total"
echo "═══════════════════════════════════════════════════════════════════════════"

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
