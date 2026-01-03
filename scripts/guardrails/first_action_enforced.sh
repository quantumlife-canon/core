#!/bin/bash
# Phase 24: First Reversible Real Action - Guardrails
#
# Enforces:
#   - Preview only, never execution
#   - One action per period maximum
#   - No identifiable data persisted
#   - Silence resumes after action
#   - No goroutines
#   - No time.Now() - clock injection only
#   - Stdlib only
#   - Pipe-delimited canonical strings (no JSON)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

ERRORS=0

echo "=== Phase 24: First Reversible Real Action Guardrails ==="
echo ""

# 1. Check that ActionKind only has preview_only
echo "Check 1: ActionKind has only preview_only..."
if ! grep -q 'KindPreviewOnly.*ActionKind.*=.*"preview_only"' "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
    echo "  FAIL: ActionKind must have KindPreviewOnly = \"preview_only\""
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: ActionKind has preview_only"
fi

# 2. Check that no execution methods exist
echo "Check 2: No execution methods in firstaction packages..."
if grep -rn "Execute\|SendEmail\|SendDraft\|SubmitPayment" "$PROJECT_ROOT/pkg/domain/firstaction/" "$PROJECT_ROOT/internal/firstaction/" 2>/dev/null | grep -v "\.sh:" | grep -v "# "; then
    echo "  FAIL: Execution methods found in firstaction packages"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No execution methods"
fi

# 3. Check that no goroutines exist
echo "Check 3: No goroutines in firstaction packages..."
if grep -rn "go func\|go \w\+(" "$PROJECT_ROOT/pkg/domain/firstaction/" "$PROJECT_ROOT/internal/firstaction/" 2>/dev/null | grep -v "\.sh:" | grep -v "\.md:"; then
    echo "  FAIL: Goroutines found in firstaction packages"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No goroutines"
fi

# 4. Check that no time.Now() exists (excluding comments)
echo "Check 4: No time.Now() in firstaction packages..."
if grep -rn "time\.Now()" "$PROJECT_ROOT/pkg/domain/firstaction/" "$PROJECT_ROOT/internal/firstaction/" 2>/dev/null | grep -v "\.sh:" | grep -v "//.*time\.Now"; then
    echo "  FAIL: time.Now() found in firstaction packages"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No time.Now()"
fi

# 5. Check that clock injection is used
echo "Check 5: Clock injection pattern used..."
if ! grep -q "clock func() time.Time" "$PROJECT_ROOT/internal/firstaction/engine.go"; then
    echo "  FAIL: Clock injection not found in engine"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Clock injection used"
fi

# 6. Check for pipe-delimited canonical strings
echo "Check 6: Pipe-delimited canonical strings used..."
if ! grep -q 'strings.Join(parts, "|")' "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
    echo "  FAIL: Pipe-delimited canonical strings not found"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Pipe-delimited canonical strings"
fi

# 7. Check that no JSON marshaling exists in domain
echo "Check 7: No JSON marshaling in domain..."
if grep -rn "json\.Marshal\|json\.Unmarshal" "$PROJECT_ROOT/pkg/domain/firstaction/" 2>/dev/null | grep -v "\.sh:"; then
    echo "  FAIL: JSON marshaling found in domain"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No JSON marshaling in domain"
fi

# 8. Check that persistence is hash-only
echo "Check 8: Persistence is hash-only..."
if ! grep -q "hash -> record\|Hash-only\|NO raw content" "$PROJECT_ROOT/internal/persist/firstaction_store.go"; then
    echo "  FAIL: Persistence must be hash-only"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Persistence is hash-only"
fi

# 9. Check that one-per-period enforcement exists
echo "Check 9: One-per-period enforcement exists..."
if ! grep -q "HasActionThisPeriod\|one-per-period" "$PROJECT_ROOT/internal/persist/firstaction_store.go"; then
    echo "  FAIL: One-per-period enforcement not found"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: One-per-period enforcement exists"
fi

# 10. Check that ActionState has correct values
echo "Check 10: ActionState has correct values..."
MISSING_STATES=0
for state in "offered" "viewed" "dismissed" "acknowledged"; do
    if ! grep -q "State.*=.*\"$state\"" "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
        echo "  FAIL: Missing state: $state"
        MISSING_STATES=$((MISSING_STATES + 1))
    fi
done
if [ $MISSING_STATES -eq 0 ]; then
    echo "  PASS: All ActionState values present"
else
    ERRORS=$((ERRORS + MISSING_STATES))
fi

# 11. Check that AbstractCategory enum exists
echo "Check 11: AbstractCategory enum exists..."
MISSING_CATEGORIES=0
for cat in "money" "time" "work" "people" "home"; do
    if ! grep -q "Category.*=.*\"$cat\"" "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
        echo "  FAIL: Missing category: $cat"
        MISSING_CATEGORIES=$((MISSING_CATEGORIES + 1))
    fi
done
if [ $MISSING_CATEGORIES -eq 0 ]; then
    echo "  PASS: All AbstractCategory values present"
else
    ERRORS=$((ERRORS + MISSING_CATEGORIES))
fi

# 12. Check that HorizonBucket enum exists
echo "Check 12: HorizonBucket enum exists..."
MISSING_HORIZONS=0
for h in "soon" "later" "someday"; do
    if ! grep -q "Horizon.*=.*\"$h\"" "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
        echo "  FAIL: Missing horizon: $h"
        MISSING_HORIZONS=$((MISSING_HORIZONS + 1))
    fi
done
if [ $MISSING_HORIZONS -eq 0 ]; then
    echo "  PASS: All HorizonBucket values present"
else
    ERRORS=$((ERRORS + MISSING_HORIZONS))
fi

# 13. Check that MagnitudeBucket enum exists
echo "Check 13: MagnitudeBucket enum exists..."
MISSING_MAGS=0
for m in "small" "medium" "large"; do
    if ! grep -q "Magnitude.*=.*\"$m\"" "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
        echo "  FAIL: Missing magnitude: $m"
        MISSING_MAGS=$((MISSING_MAGS + 1))
    fi
done
if [ $MISSING_MAGS -eq 0 ]; then
    echo "  PASS: All MagnitudeBucket values present"
else
    ERRORS=$((ERRORS + MISSING_MAGS))
fi

# 14. Check that ActionPreview has no identifiers
echo "Check 14: ActionPreview has no identifiers..."
if grep -A20 "type ActionPreview struct" "$PROJECT_ROOT/pkg/domain/firstaction/types.go" | grep -q "Email\|Subject\|Body\|Sender\|Recipient\|Amount\|VendorName"; then
    echo "  FAIL: ActionPreview contains identifiers"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: ActionPreview has no identifiers"
fi

# 15. Check that handlers emit correct events
echo "Check 15: Handlers emit Phase 24 events..."
if ! grep -q "Phase24InvitationOffered\|Phase24ActionViewed\|Phase24ActionDismissed\|Phase24PreviewRendered\|Phase24PeriodClosed" "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo "  FAIL: Phase 24 events not emitted in handlers"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Phase 24 events emitted"
fi

# 16. Check that routes exist
echo "Check 16: Phase 24 routes exist..."
MISSING_ROUTES=0
for route in "/action/once" "/action/once/run" "/action/once/dismiss"; do
    if ! grep -q "\"$route\"" "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
        echo "  FAIL: Missing route: $route"
        MISSING_ROUTES=$((MISSING_ROUTES + 1))
    fi
done
if [ $MISSING_ROUTES -eq 0 ]; then
    echo "  PASS: All routes exist"
else
    ERRORS=$((ERRORS + MISSING_ROUTES))
fi

# 17. Check that no external HTTP calls exist
echo "Check 17: No external HTTP calls in firstaction packages..."
if grep -rn "http\.Get\|http\.Post\|http\.Client" "$PROJECT_ROOT/pkg/domain/firstaction/" "$PROJECT_ROOT/internal/firstaction/" 2>/dev/null | grep -v "\.sh:"; then
    echo "  FAIL: External HTTP calls found"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No external HTTP calls"
fi

# 18. Check that no retries exist
echo "Check 18: No retry patterns in firstaction packages..."
if grep -rn "retry\|Retry\|backoff\|Backoff" "$PROJECT_ROOT/pkg/domain/firstaction/" "$PROJECT_ROOT/internal/firstaction/" 2>/dev/null | grep -v "\.sh:"; then
    echo "  FAIL: Retry patterns found"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No retry patterns"
fi

# 19. Check that SelectHeldItem is deterministic
echo "Check 19: SelectHeldItem is deterministic..."
if ! grep -q "deterministic\|Deterministic\|lowest hash" "$PROJECT_ROOT/internal/firstaction/engine.go"; then
    echo "  FAIL: SelectHeldItem must be deterministic"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: SelectHeldItem is deterministic"
fi

# 20. Check that preview page shows disclaimer
echo "Check 20: Preview page has disclaimer..."
if ! grep -q "This is a preview\|We did not act\|preview\. We did not act" "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
    echo "  FAIL: Preview page must have disclaimer"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Preview page has disclaimer"
fi

# 21. Check that silence resumes after action
echo "Check 21: Silence resumes after action..."
if ! grep -q "Quiet resumes\|silence resumes" "$PROJECT_ROOT/pkg/domain/firstaction/types.go" "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 2>/dev/null; then
    echo "  FAIL: Silence must resume after action"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Silence resumes after action"
fi

# 22. Check that bounded retention exists
echo "Check 22: Bounded retention in store..."
if ! grep -q "maxEntries\|Bounded\|evict" "$PROJECT_ROOT/internal/persist/firstaction_store.go"; then
    echo "  FAIL: Bounded retention not found"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Bounded retention exists"
fi

# 23. Check that ActionPage has calm text
echo "Check 23: ActionPage has calm text..."
CALM_PHRASES=0
for phrase in "Nothing to look at" "quietly" "wait" "No rush"; do
    if grep -q "$phrase" "$PROJECT_ROOT/pkg/domain/firstaction/types.go"; then
        CALM_PHRASES=$((CALM_PHRASES + 1))
    fi
done
if [ $CALM_PHRASES -lt 3 ]; then
    echo "  FAIL: ActionPage must have calm text"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: ActionPage has calm text"
fi

# 24. Check that whisper cue exists
echo "Check 24: Whisper cue exists for /today..."
if ! grep -q "WhisperCue\|whisper" "$PROJECT_ROOT/internal/firstaction/engine.go"; then
    echo "  FAIL: Whisper cue not found"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: Whisper cue exists"
fi

# 25. Check that no forbidden terms exist in domain
echo "Check 25: No forbidden terms in domain..."
if grep -rn "urgency\|urgent\|fear\|shame\|blame\|escalate" "$PROJECT_ROOT/pkg/domain/firstaction/" 2>/dev/null | grep -v "\.sh:" | grep -iv "// "; then
    echo "  FAIL: Forbidden terms found in domain"
    ERRORS=$((ERRORS + 1))
else
    echo "  PASS: No forbidden terms"
fi

echo ""
echo "=== Summary ==="
if [ $ERRORS -eq 0 ]; then
    echo "All 25 checks passed!"
    exit 0
else
    echo "FAILED: $ERRORS check(s) failed"
    exit 1
fi
