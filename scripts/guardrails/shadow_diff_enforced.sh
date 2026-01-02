#!/bin/bash
# Phase 19.4: Shadow Diff + Calibration Guardrails
#
# Validates that shadow diff implementation follows all invariants.
#
# CRITICAL: Shadow diff must:
#   - NOT affect any execution path
#   - NOT mutate policies or routing
#   - Work with stub provider (no real LLM required)
#   - Use stdlib only
#   - Have no goroutines
#   - Have no time.Now()
#   - Use hash-only persistence
#
# Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md

set -e

ERRORS=0

error() {
    echo "[ERROR] $1"
    ERRORS=$((ERRORS + 1))
}

check() {
    echo "[CHECK] $1"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Check 1: Shadow diff domain types exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Shadow diff domain types exist"
if [ ! -f "pkg/domain/shadowdiff/types.go" ]; then
    error "types.go not found at pkg/domain/shadowdiff/types.go"
fi

check "Shadow diff hashing exists"
if [ ! -f "pkg/domain/shadowdiff/hashing.go" ]; then
    error "hashing.go not found at pkg/domain/shadowdiff/hashing.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 2: Diff engine exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Diff engine exists"
if [ ! -f "internal/shadowdiff/engine.go" ]; then
    error "engine.go not found at internal/shadowdiff/engine.go"
fi

check "Diff rules exist"
if [ ! -f "internal/shadowdiff/rules.go" ]; then
    error "rules.go not found at internal/shadowdiff/rules.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 3: Calibration store exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Calibration store exists"
if [ ! -f "internal/persist/shadow_calibration_store.go" ]; then
    error "shadow_calibration_store.go not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 4: Calibration aggregator exists
# ═══════════════════════════════════════════════════════════════════════════════
check "Calibration stats engine exists"
if [ ! -f "internal/shadowcalibration/engine.go" ]; then
    error "engine.go not found at internal/shadowcalibration/engine.go"
fi

check "Calibration stats module exists"
if [ ! -f "internal/shadowcalibration/stats.go" ]; then
    error "stats.go not found at internal/shadowcalibration/stats.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 5: No execution path influence
# ═══════════════════════════════════════════════════════════════════════════════
check "No shadowdiff imports in execution packages"
if grep -r 'shadowdiff\|shadowcalibration' internal/email/execution/ internal/calendar/execution/ 2>/dev/null; then
    error "Found shadow diff import in execution package"
fi

check "No shadowdiff imports in drafts packages"
if grep -r 'shadowdiff\|shadowcalibration' internal/drafts/ 2>/dev/null | grep -v '_test.go'; then
    error "Found shadow diff import in drafts package"
fi

check "No shadowdiff imports in routing packages"
if grep -r 'shadowdiff\|shadowcalibration' internal/routing/ 2>/dev/null | grep -v '_test.go'; then
    error "Found shadow diff import in routing package"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 6: No policy mutation
# ═══════════════════════════════════════════════════════════════════════════════
check "No policy references in shadowdiff"
if grep -r 'SetPolicy\|UpdatePolicy\|DeletePolicy' internal/shadowdiff/ internal/shadowcalibration/ 2>/dev/null | grep -v '_test.go'; then
    error "Found policy mutation in shadow diff packages"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 7: No goroutines in shadow diff packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No goroutines in pkg/domain/shadowdiff"
if grep -r 'go func' pkg/domain/shadowdiff/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in pkg/domain/shadowdiff"
fi

check "No goroutines in internal/shadowdiff"
if grep -r 'go func' internal/shadowdiff/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in internal/shadowdiff"
fi

check "No goroutines in internal/shadowcalibration"
if grep -r 'go func' internal/shadowcalibration/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in internal/shadowcalibration"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 8: No time.Now() in shadow diff packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No time.Now() in pkg/domain/shadowdiff"
if grep -rn 'time\.Now()' pkg/domain/shadowdiff/ 2>/dev/null | grep -v '_test.go' | grep -v '//' > /dev/null; then
    error "Found time.Now() in pkg/domain/shadowdiff - must use clock injection"
fi

check "No time.Now() in internal/shadowdiff"
if grep -rn 'time\.Now()' internal/shadowdiff/ 2>/dev/null | grep -v '_test.go' | grep -v '//' > /dev/null; then
    error "Found time.Now() in internal/shadowdiff - must use clock injection"
fi

check "No time.Now() in internal/shadowcalibration"
if grep -rn 'time\.Now()' internal/shadowcalibration/ 2>/dev/null | grep -v '_test.go' | grep -v '//' > /dev/null; then
    error "Found time.Now() in internal/shadowcalibration - must use clock injection"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 9: stdlib only (no external imports)
# ═══════════════════════════════════════════════════════════════════════════════
check "No external imports in pkg/domain/shadowdiff"
if grep -r 'github.com' pkg/domain/shadowdiff/ 2>/dev/null | grep -v 'quantumlife' | grep -v '_test.go'; then
    error "Found external import in pkg/domain/shadowdiff"
fi

check "No external imports in internal/shadowdiff"
if grep -r 'github.com' internal/shadowdiff/ 2>/dev/null | grep -v 'quantumlife' | grep -v '_test.go'; then
    error "Found external import in internal/shadowdiff"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 10: Hash-only persistence (no raw content)
# ═══════════════════════════════════════════════════════════════════════════════
check "Calibration store uses hash-only storage"
if ! grep -q 'DiffHash' internal/persist/shadow_calibration_store.go; then
    error "Calibration store should reference DiffHash"
fi

check "No raw content fields in shadowdiff types"
FORBIDDEN_FIELDS="Subject|Body|Sender|Recipient|RawContent|EmailAddress|VendorName"
if grep -E "($FORBIDDEN_FIELDS)" pkg/domain/shadowdiff/types.go 2>/dev/null | grep -v '//'; then
    error "Found forbidden raw content fields in shadowdiff types"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 11: Phase 19.4 events exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Phase19_4DiffComputed event exists"
if ! grep -q 'Phase19_4DiffComputed' pkg/events/events.go; then
    error "Phase19_4DiffComputed event not found"
fi

check "Phase19_4VoteRecorded event exists"
if ! grep -q 'Phase19_4VoteRecorded' pkg/events/events.go; then
    error "Phase19_4VoteRecorded event not found"
fi

check "Phase19_4StatsComputed event exists"
if ! grep -q 'Phase19_4StatsComputed' pkg/events/events.go; then
    error "Phase19_4StatsComputed event not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 12: Storelog record types exist
# ═══════════════════════════════════════════════════════════════════════════════
check "RecordTypeShadowDiff exists"
if ! grep -q 'RecordTypeShadowDiff' pkg/domain/storelog/log.go; then
    error "RecordTypeShadowDiff not found in storelog"
fi

check "RecordTypeShadowCalibration exists"
if ! grep -q 'RecordTypeShadowCalibration' pkg/domain/storelog/log.go; then
    error "RecordTypeShadowCalibration not found in storelog"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 13: Demo tests exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Demo test file exists"
if [ ! -f "internal/demo_phase19_4_shadow_diff/demo_test.go" ]; then
    error "Demo test file not found"
fi

check "Demo tests have 8+ test functions"
TEST_COUNT=$(grep -c 'func Test' internal/demo_phase19_4_shadow_diff/demo_test.go 2>/dev/null || echo 0)
if [ "$TEST_COUNT" -lt 8 ]; then
    error "Demo tests have only $TEST_COUNT test functions (need 8+)"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 14: Web routes exist
# ═══════════════════════════════════════════════════════════════════════════════
check "/shadow/report route exists"
if ! grep -q '/shadow/report' cmd/quantumlife-web/main.go; then
    error "/shadow/report route not found"
fi

check "/shadow/vote route exists"
if ! grep -q '/shadow/vote' cmd/quantumlife-web/main.go; then
    error "/shadow/vote route not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 15: Agreement types are complete
# ═══════════════════════════════════════════════════════════════════════════════
check "AgreementMatch type exists"
if ! grep -q 'AgreementMatch' pkg/domain/shadowdiff/types.go; then
    error "AgreementMatch type not found"
fi

check "AgreementConflict type exists"
if ! grep -q 'AgreementConflict' pkg/domain/shadowdiff/types.go; then
    error "AgreementConflict type not found"
fi

check "AgreementEarlier type exists"
if ! grep -q 'AgreementEarlier' pkg/domain/shadowdiff/types.go; then
    error "AgreementEarlier type not found"
fi

check "AgreementLater type exists"
if ! grep -q 'AgreementLater' pkg/domain/shadowdiff/types.go; then
    error "AgreementLater type not found"
fi

check "AgreementSofter type exists"
if ! grep -q 'AgreementSofter' pkg/domain/shadowdiff/types.go; then
    error "AgreementSofter type not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 16: Novelty types are complete
# ═══════════════════════════════════════════════════════════════════════════════
check "NoveltyNone type exists"
if ! grep -q 'NoveltyNone' pkg/domain/shadowdiff/types.go; then
    error "NoveltyNone type not found"
fi

check "NoveltyShadowOnly type exists"
if ! grep -q 'NoveltyShadowOnly' pkg/domain/shadowdiff/types.go; then
    error "NoveltyShadowOnly type not found"
fi

check "NoveltyCanonOnly type exists"
if ! grep -q 'NoveltyCanonOnly' pkg/domain/shadowdiff/types.go; then
    error "NoveltyCanonOnly type not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 17: Vote types are complete
# ═══════════════════════════════════════════════════════════════════════════════
check "VoteUseful type exists"
if ! grep -q 'VoteUseful' pkg/domain/shadowdiff/types.go; then
    error "VoteUseful type not found"
fi

check "VoteUnnecessary type exists"
if ! grep -q 'VoteUnnecessary' pkg/domain/shadowdiff/types.go; then
    error "VoteUnnecessary type not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 18: Canonical strings use pipe delimiter
# ═══════════════════════════════════════════════════════════════════════════════
check "DiffResult uses pipe-delimited canonical string"
if ! grep -q 'DIFF_RESULT|v1|' pkg/domain/shadowdiff/hashing.go; then
    error "DiffResult canonical string missing pipe-delimited prefix"
fi

check "No json.Marshal in shadowdiff hashing"
if grep -r 'json\.Marshal' pkg/domain/shadowdiff/hashing.go 2>/dev/null; then
    error "Found json.Marshal in hashing.go - must use pipe-delimited strings"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "=========================================="
    echo "  Phase 19.4 Shadow Diff Guardrails: PASS"
    echo "=========================================="
    exit 0
else
    echo "=========================================="
    echo "  Phase 19.4 Shadow Diff Guardrails: FAIL"
    echo "  Errors: $ERRORS"
    echo "=========================================="
    exit 1
fi
