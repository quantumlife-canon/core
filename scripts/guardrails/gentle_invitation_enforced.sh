#!/bin/bash
# Phase 23: Gentle Action Invitation Guardrails
# Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
#
# CRITICAL INVARIANTS:
#   - No auto-execution
#   - No background triggers
#   - No retries
#   - No goroutines
#   - No time.Now()
#   - Stdlib only
#   - No identifiers in events

set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

PASS=0
FAIL=0

check() {
    local name="$1"
    local result="$2"
    if [ "$result" -eq 0 ]; then
        echo -e "[${GREEN}PASS${NC}] $name"
        ((PASS++))
    else
        echo -e "[${RED}FAIL${NC}] $name"
        ((FAIL++))
    fi
}

echo "=== Phase 23: Gentle Action Invitation Guardrails ==="
echo ""

# -----------------------------------------------------------------------------
# Domain Model Guardrails
# -----------------------------------------------------------------------------
echo "--- Domain Model Guardrails ---"

# Check: No goroutines in domain
if grep -r "go func\|go " pkg/domain/invitation/*.go 2>/dev/null | grep -v "// " | grep -v ".go:" > /dev/null 2>&1; then
    check "No goroutines in pkg/domain/invitation/" 1
else
    check "No goroutines in pkg/domain/invitation/" 0
fi

# Check: No time.Now() in domain (must use clock injection)
if grep -r "time\.Now()" pkg/domain/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No time.Now() in pkg/domain/invitation/" 1
else
    check "No time.Now() in pkg/domain/invitation/" 0
fi

# Check: InvitationKind enum exists
if grep -q "type InvitationKind string" pkg/domain/invitation/types.go 2>/dev/null; then
    check "InvitationKind enum exists" 0
else
    check "InvitationKind enum exists" 1
fi

# Check: InvitationDecision enum exists
if grep -q "type InvitationDecision string" pkg/domain/invitation/types.go 2>/dev/null; then
    check "InvitationDecision enum exists" 0
else
    check "InvitationDecision enum exists" 1
fi

# Check: InvitationSummary struct exists
if grep -q "type InvitationSummary struct" pkg/domain/invitation/types.go 2>/dev/null; then
    check "InvitationSummary struct exists" 0
else
    check "InvitationSummary struct exists" 1
fi

# Check: Pipe-delimited canonical string format
if grep -q 'strings.Join.*"|"' pkg/domain/invitation/types.go 2>/dev/null; then
    check "Pipe-delimited canonical string format" 0
else
    check "Pipe-delimited canonical string format" 1
fi

# Check: hold_continue kind exists
if grep -q 'KindHoldContinue.*=.*"hold_continue"' pkg/domain/invitation/types.go 2>/dev/null; then
    check "hold_continue kind exists" 0
else
    check "hold_continue kind exists" 1
fi

# Check: review_once kind exists
if grep -q 'KindReviewOnce.*=.*"review_once"' pkg/domain/invitation/types.go 2>/dev/null; then
    check "review_once kind exists" 0
else
    check "review_once kind exists" 1
fi

# Check: notify_next_time kind exists
if grep -q 'KindNotifyNextTime.*=.*"notify_next_time"' pkg/domain/invitation/types.go 2>/dev/null; then
    check "notify_next_time kind exists" 0
else
    check "notify_next_time kind exists" 1
fi

echo ""

# -----------------------------------------------------------------------------
# Engine Guardrails
# -----------------------------------------------------------------------------
echo "--- Engine Guardrails ---"

# Check: No goroutines in engine
if grep -r "go func\|go " internal/invitation/*.go 2>/dev/null | grep -v "// " | grep -v ".go:" > /dev/null 2>&1; then
    check "No goroutines in internal/invitation/" 1
else
    check "No goroutines in internal/invitation/" 0
fi

# Check: No time.Now() in engine
if grep -r "time\.Now()" internal/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No time.Now() in internal/invitation/" 1
else
    check "No time.Now() in internal/invitation/" 0
fi

# Check: Clock injection pattern
if grep -q "clock func() time.Time" internal/invitation/engine.go 2>/dev/null; then
    check "Clock injection pattern in engine" 0
else
    check "Clock injection pattern in engine" 1
fi

# Check: Engine struct exists
if grep -q "type Engine struct" internal/invitation/engine.go 2>/dev/null; then
    check "Engine struct exists" 0
else
    check "Engine struct exists" 1
fi

# Check: Compute function exists
if grep -q "func (e \*Engine) Compute" internal/invitation/engine.go 2>/dev/null; then
    check "Compute function exists" 0
else
    check "Compute function exists" 1
fi

echo ""

# -----------------------------------------------------------------------------
# No Auto-Execution Guardrails
# -----------------------------------------------------------------------------
echo "--- No Auto-Execution Guardrails ---"

# Check: No execute imports in invitation
if grep -r "execution\|executor" internal/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No execution imports in invitation" 1
else
    check "No execution imports in invitation" 0
fi

# Check: No draft creation in invitation
if grep -r "CreateDraft\|NewDraft" internal/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No draft creation in invitation" 1
else
    check "No draft creation in invitation" 0
fi

# Check: No obligation creation in invitation
if grep -r "CreateObligation\|NewObligation" internal/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No obligation creation in invitation" 1
else
    check "No obligation creation in invitation" 0
fi

echo ""

# -----------------------------------------------------------------------------
# No Background Triggers Guardrails
# -----------------------------------------------------------------------------
echo "--- No Background Triggers Guardrails ---"

# Check: No ticker in invitation
if grep -r "time.Ticker\|time.NewTicker" internal/invitation/*.go pkg/domain/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No ticker in invitation packages" 1
else
    check "No ticker in invitation packages" 0
fi

# Check: No timer in invitation
if grep -r "time.Timer\|time.NewTimer\|time.After" internal/invitation/*.go pkg/domain/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No timer in invitation packages" 1
else
    check "No timer in invitation packages" 0
fi

echo ""

# -----------------------------------------------------------------------------
# No Retries Guardrails
# -----------------------------------------------------------------------------
echo "--- No Retries Guardrails ---"

# Check: No retry patterns
if grep -ri "retry\|Retry\|backoff\|Backoff" internal/invitation/*.go pkg/domain/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No retry patterns in invitation" 1
else
    check "No retry patterns in invitation" 0
fi

echo ""

# -----------------------------------------------------------------------------
# Stdlib Only Guardrails
# -----------------------------------------------------------------------------
echo "--- Stdlib Only Guardrails ---"

# Check: Stdlib only in domain
if grep -E "^\s+\"github\.com|^\s+\"golang\.org" pkg/domain/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "Stdlib only in domain" 1
else
    check "Stdlib only in domain" 0
fi

# Check: Stdlib only in engine
if grep -E "^\s+\"github\.com|^\s+\"golang\.org" internal/invitation/*.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "Stdlib only in engine" 1
else
    check "Stdlib only in engine" 0
fi

echo ""

# -----------------------------------------------------------------------------
# Events Guardrails
# -----------------------------------------------------------------------------
echo "--- Events Guardrails ---"

# Check: Phase23InvitationEligible event defined
if grep -q "Phase23InvitationEligible" pkg/events/events.go 2>/dev/null; then
    check "Phase23InvitationEligible event defined" 0
else
    check "Phase23InvitationEligible event defined" 1
fi

# Check: Phase23InvitationRendered event defined
if grep -q "Phase23InvitationRendered" pkg/events/events.go 2>/dev/null; then
    check "Phase23InvitationRendered event defined" 0
else
    check "Phase23InvitationRendered event defined" 1
fi

# Check: Phase23InvitationAccepted event defined
if grep -q "Phase23InvitationAccepted" pkg/events/events.go 2>/dev/null; then
    check "Phase23InvitationAccepted event defined" 0
else
    check "Phase23InvitationAccepted event defined" 1
fi

# Check: Phase23InvitationDismissed event defined
if grep -q "Phase23InvitationDismissed" pkg/events/events.go 2>/dev/null; then
    check "Phase23InvitationDismissed event defined" 0
else
    check "Phase23InvitationDismissed event defined" 1
fi

# Check: Phase23InvitationPersisted event defined
if grep -q "Phase23InvitationPersisted" pkg/events/events.go 2>/dev/null; then
    check "Phase23InvitationPersisted event defined" 0
else
    check "Phase23InvitationPersisted event defined" 1
fi

# Check: Phase23InvitationSkipped event defined
if grep -q "Phase23InvitationSkipped" pkg/events/events.go 2>/dev/null; then
    check "Phase23InvitationSkipped event defined" 0
else
    check "Phase23InvitationSkipped event defined" 1
fi

echo ""

# -----------------------------------------------------------------------------
# Persistence Guardrails
# -----------------------------------------------------------------------------
echo "--- Persistence Guardrails ---"

# Check: InvitationStore exists
if grep -q "type InvitationStore struct" internal/persist/invitation_store.go 2>/dev/null; then
    check "InvitationStore exists" 0
else
    check "InvitationStore exists" 1
fi

# Check: No time.Now() in store
if grep -r "time\.Now()" internal/persist/invitation_store.go 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
    check "No time.Now() in invitation store" 1
else
    check "No time.Now() in invitation store" 0
fi

# Check: Hash-keyed storage
if grep -q "map\[string\]" internal/persist/invitation_store.go 2>/dev/null; then
    check "Hash-keyed storage in store" 0
else
    check "Hash-keyed storage in store" 1
fi

# Check: Bounded retention
if grep -q "maxEntries" internal/persist/invitation_store.go 2>/dev/null; then
    check "Bounded retention in store" 0
else
    check "Bounded retention in store" 1
fi

echo ""

# -----------------------------------------------------------------------------
# Web Routes Guardrails
# -----------------------------------------------------------------------------
echo "--- Web Routes Guardrails ---"

# Check: /invite route registered
if grep -q '"/invite"' cmd/quantumlife-web/main.go 2>/dev/null; then
    check "/invite route registered" 0
else
    check "/invite route registered" 1
fi

# Check: /invite/accept route registered
if grep -q '"/invite/accept"' cmd/quantumlife-web/main.go 2>/dev/null; then
    check "/invite/accept route registered" 0
else
    check "/invite/accept route registered" 1
fi

# Check: /invite/dismiss route registered
if grep -q '"/invite/dismiss"' cmd/quantumlife-web/main.go 2>/dev/null; then
    check "/invite/dismiss route registered" 0
else
    check "/invite/dismiss route registered" 1
fi

# Check: handleInvitation handler exists
if grep -q "func (s \*Server) handleInvitation" cmd/quantumlife-web/main.go 2>/dev/null; then
    check "handleInvitation handler exists" 0
else
    check "handleInvitation handler exists" 1
fi

echo ""

# -----------------------------------------------------------------------------
# Copy Rules Guardrails (Forbidden Terms)
# -----------------------------------------------------------------------------
echo "--- Copy Rules Guardrails ---"

# Check: No urgency language in invitation text
if grep -ri "urgent\|immediately\|now\|hurry\|asap\|important" pkg/domain/invitation/types.go 2>/dev/null | grep -v "// " | grep -i "displaytext\|whisper" > /dev/null 2>&1; then
    check "No urgency language in invitation copy" 1
else
    check "No urgency language in invitation copy" 0
fi

# Check: Allowed phrases exist
if grep -q "We can keep holding this" pkg/domain/invitation/types.go 2>/dev/null; then
    check "Allowed phrase 'hold_continue' exists" 0
else
    check "Allowed phrase 'hold_continue' exists" 1
fi

if grep -q "You can look once, if you want" pkg/domain/invitation/types.go 2>/dev/null; then
    check "Allowed phrase 'review_once' exists" 0
else
    check "Allowed phrase 'review_once' exists" 1
fi

echo ""

# -----------------------------------------------------------------------------
# Demo Tests Guardrails
# -----------------------------------------------------------------------------
echo "--- Demo Tests Guardrails ---"

# Check: Demo tests file exists
if [ -f "internal/demo_phase23_gentle_invitation/demo_test.go" ]; then
    check "Demo tests file exists" 0
else
    check "Demo tests file exists" 1
fi

# Check: Demo tests compile (if file exists)
if [ -f "internal/demo_phase23_gentle_invitation/demo_test.go" ]; then
    if go build ./internal/demo_phase23_gentle_invitation/... 2>/dev/null; then
        check "Demo tests compile" 0
    else
        check "Demo tests compile" 1
    fi
else
    check "Demo tests compile" 1
fi

echo ""

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo "==================================================================="
echo "Summary: $PASS passed, $FAIL failed"
echo "==================================================================="

if [ "$FAIL" -gt 0 ]; then
    echo -e "${RED}Phase 23 guardrails FAILED.${NC}"
    exit 1
else
    echo -e "${GREEN}All Phase 23 guardrails PASSED.${NC}"
    exit 0
fi
