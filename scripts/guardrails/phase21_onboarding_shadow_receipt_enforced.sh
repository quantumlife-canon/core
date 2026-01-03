#!/bin/bash
# Phase 21: Unified Onboarding + Shadow Receipt Viewer Guardrails
#
# CRITICAL INVARIANTS:
#   - Mode is DERIVED, not stored
#   - UI shows ONLY abstract buckets and hashes
#   - No goroutines in mode/ or shadowview/
#   - No time.Now() in mode/ or shadowview/
#   - Clock injection pattern enforced
#   - Ack store stores ONLY hashes
#
# Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md

set -e

echo "=== Phase 21: Unified Onboarding + Shadow Receipt Viewer Guardrails ==="
echo ""

FAILED=0
PASSED=0

check() {
    local name="$1"
    local result="$2"
    if [ "$result" = "PASS" ]; then
        echo "[PASS] $name"
        PASSED=$((PASSED + 1))
    else
        echo "[FAIL] $name"
        FAILED=$((FAILED + 1))
    fi
}

# ============================================================================
# Mode Package Guardrails (internal/mode/)
# ============================================================================

echo ""
echo "--- Mode Package Guardrails ---"

# 1. No goroutines in mode package
if ! grep -r "go func\|go \w\+(" internal/mode/*.go 2>/dev/null; then
    check "No goroutines in internal/mode/" "PASS"
else
    check "No goroutines in internal/mode/" "FAIL"
fi

# 2. No time.Now() in mode package (excluding comments)
if ! grep -r "time\.Now()" internal/mode/*.go 2>/dev/null | grep -v "//"; then
    check "No time.Now() in internal/mode/" "PASS"
else
    check "No time.Now() in internal/mode/" "FAIL"
fi

# 3. Mode package uses clock injection
if grep -r "func.*clock.*func.*time\.Time" internal/mode/*.go 2>/dev/null | grep -q .; then
    check "Mode engine uses clock injection" "PASS"
else
    check "Mode engine uses clock injection" "FAIL"
fi

# 4. Mode enum exists with correct values
if grep -q "ModeDemo.*Mode.*=" internal/mode/model.go && \
   grep -q "ModeConnected.*Mode.*=" internal/mode/model.go && \
   grep -q "ModeShadow.*Mode.*=" internal/mode/model.go; then
    check "Mode enum has Demo/Connected/Shadow values" "PASS"
else
    check "Mode enum has Demo/Connected/Shadow values" "FAIL"
fi

# 5. ModeIndicator struct exists
if grep -q "type ModeIndicator struct" internal/mode/model.go; then
    check "ModeIndicator struct exists" "PASS"
else
    check "ModeIndicator struct exists" "FAIL"
fi

# 6. DeriveMode function exists
if grep -q "func.*DeriveMode" internal/mode/engine.go; then
    check "DeriveMode function exists" "PASS"
else
    check "DeriveMode function exists" "FAIL"
fi

# ============================================================================
# Shadowview Package Guardrails (internal/shadowview/)
# ============================================================================

echo ""
echo "--- Shadowview Package Guardrails ---"

# 7. No goroutines in shadowview package
if ! grep -r "go func\|go \w\+(" internal/shadowview/*.go 2>/dev/null; then
    check "No goroutines in internal/shadowview/" "PASS"
else
    check "No goroutines in internal/shadowview/" "FAIL"
fi

# 8. No time.Now() in shadowview package (excluding comments)
if ! grep -r "time\.Now()" internal/shadowview/*.go 2>/dev/null | grep -v "//"; then
    check "No time.Now() in internal/shadowview/" "PASS"
else
    check "No time.Now() in internal/shadowview/" "FAIL"
fi

# 9. Shadowview engine uses clock injection
if grep -r "func.*clock.*func.*time\.Time" internal/shadowview/*.go 2>/dev/null | grep -q .; then
    check "Shadowview engine uses clock injection" "PASS"
else
    check "Shadowview engine uses clock injection" "FAIL"
fi

# 10. ShadowReceiptPage struct exists
if grep -q "type ShadowReceiptPage struct" internal/shadowview/model.go; then
    check "ShadowReceiptPage struct exists" "PASS"
else
    check "ShadowReceiptPage struct exists" "FAIL"
fi

# 11. AckStore struct exists
if grep -q "type AckStore struct" internal/shadowview/ack_store.go; then
    check "AckStore struct exists" "PASS"
else
    check "AckStore struct exists" "FAIL"
fi

# 12. Ack store has bounded size
if grep -q "maxRecords" internal/shadowview/ack_store.go; then
    check "AckStore has bounded size" "PASS"
else
    check "AckStore has bounded size" "FAIL"
fi

# 13. Ack store only stores hashes
if grep -q "ReceiptHash.*string" internal/shadowview/ack_store.go && \
   grep -q "TSHash.*string" internal/shadowview/ack_store.go; then
    check "AckStore stores only hashes (ReceiptHash, TSHash)" "PASS"
else
    check "AckStore stores only hashes (ReceiptHash, TSHash)" "FAIL"
fi

# 14. ReceiptCue struct exists
if grep -q "type ReceiptCue struct" internal/shadowview/engine.go; then
    check "ReceiptCue struct exists" "PASS"
else
    check "ReceiptCue struct exists" "FAIL"
fi

# 15. BuildCue function exists
if grep -q "func.*BuildCue" internal/shadowview/engine.go; then
    check "BuildCue function exists" "PASS"
else
    check "BuildCue function exists" "FAIL"
fi

# 16. Single whisper rule enforced in BuildCue
if grep -q "OtherCueActive" internal/shadowview/engine.go; then
    check "Single whisper rule check in BuildCue" "PASS"
else
    check "Single whisper rule check in BuildCue" "FAIL"
fi

# ============================================================================
# UI/Template Guardrails
# ============================================================================

echo ""
echo "--- UI/Template Guardrails ---"

# 17. Onboarding route exists
if grep -q '"/onboarding"' cmd/quantumlife-web/main.go; then
    check "/onboarding route registered" "PASS"
else
    check "/onboarding route registered" "FAIL"
fi

# 18. Shadow receipt route exists
if grep -q '"/shadow/receipt"' cmd/quantumlife-web/main.go; then
    check "/shadow/receipt route registered" "PASS"
else
    check "/shadow/receipt route registered" "FAIL"
fi

# 19. Shadow receipt dismiss route exists
if grep -q '"/shadow/receipt/dismiss"' cmd/quantumlife-web/main.go; then
    check "/shadow/receipt/dismiss route registered" "PASS"
else
    check "/shadow/receipt/dismiss route registered" "FAIL"
fi

# 20. Handler for onboarding exists
if grep -q "handleOnboarding" cmd/quantumlife-web/main.go; then
    check "handleOnboarding handler exists" "PASS"
else
    check "handleOnboarding handler exists" "FAIL"
fi

# 21. Handler for shadow receipt exists
if grep -q "handleShadowReceipt" cmd/quantumlife-web/main.go; then
    check "handleShadowReceipt handler exists" "PASS"
else
    check "handleShadowReceipt handler exists" "FAIL"
fi

# ============================================================================
# Events Guardrails
# ============================================================================

echo ""
echo "--- Events Guardrails ---"

# 22. Phase 21 onboarding viewed event exists
if grep -q "Phase21OnboardingViewed" pkg/events/events.go; then
    check "Phase21OnboardingViewed event defined" "PASS"
else
    check "Phase21OnboardingViewed event defined" "FAIL"
fi

# 23. Phase 21 shadow receipt viewed event exists
if grep -q "Phase21ShadowReceiptViewed" pkg/events/events.go; then
    check "Phase21ShadowReceiptViewed event defined" "PASS"
else
    check "Phase21ShadowReceiptViewed event defined" "FAIL"
fi

# 24. Phase 21 shadow receipt dismissed event exists
if grep -q "Phase21ShadowReceiptDismissed" pkg/events/events.go; then
    check "Phase21ShadowReceiptDismissed event defined" "PASS"
else
    check "Phase21ShadowReceiptDismissed event defined" "FAIL"
fi

# ============================================================================
# Demo Tests Guardrails
# ============================================================================

echo ""
echo "--- Demo Tests Guardrails ---"

# 25. Demo tests exist
if [ -f "internal/demo_phase21_onboarding_shadow_receipt/demo_test.go" ]; then
    check "Demo tests file exists" "PASS"
else
    check "Demo tests file exists" "FAIL"
fi

# 26. Demo tests compile
if go build ./internal/demo_phase21_onboarding_shadow_receipt/... 2>/dev/null; then
    check "Demo tests compile" "PASS"
else
    check "Demo tests compile" "FAIL"
fi

# ============================================================================
# Abstract-Only Display Guardrails
# ============================================================================

echo ""
echo "--- Abstract-Only Display Guardrails ---"

# 27. Page shows magnitude bucket, not raw count
if grep -q "Magnitude.*string" internal/shadowview/model.go; then
    check "Page uses magnitude bucket string, not count" "PASS"
else
    check "Page uses magnitude bucket string, not count" "FAIL"
fi

# 28. Page shows horizon bucket
if grep -q "Horizon.*string" internal/shadowview/model.go; then
    check "Page uses horizon bucket string" "PASS"
else
    check "Page uses horizon bucket string" "FAIL"
fi

# 29. Page shows confidence bucket
if grep -q "Bucket.*string" internal/shadowview/model.go; then
    check "Page uses confidence bucket string" "PASS"
else
    check "Page uses confidence bucket string" "FAIL"
fi

# 30. Restraint section always true
if grep -q "NoActionsTaken.*true" internal/shadowview/engine.go && \
   grep -q "NoDraftsCreated.*true" internal/shadowview/engine.go; then
    check "Restraint section always shows true values" "PASS"
else
    check "Restraint section always shows true values" "FAIL"
fi

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "==================================================================="
echo "Summary: $PASSED passed, $FAILED failed"
echo "==================================================================="

if [ $FAILED -gt 0 ]; then
    echo "Phase 21 guardrails FAILED. Fix issues above."
    exit 1
fi

echo "All Phase 21 guardrails PASSED."
exit 0
