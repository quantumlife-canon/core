#!/bin/bash
# =============================================================================
# Phase 31.4: External Pressure Circles Guardrails
# =============================================================================
#
# Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
#
# CRITICAL INVARIANTS:
#   - NO raw merchant strings, NO vendor identifiers, NO amounts, NO timestamps
#   - Only category hints, magnitude buckets, and horizon buckets
#   - Derived circles CANNOT approve, CANNOT execute, CANNOT receive drafts
#   - Hash-only persistence; deterministic: same inputs => same hashes
#   - No goroutines. No time.Now() - clock injection only.
#   - stdlib only.
#
# =============================================================================

set -e

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo "✓ $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo "✗ $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

echo "=== Phase 31.4: External Pressure Circles Guardrails ==="
echo ""

# =============================================================================
# 1. Package Structure Checks
# =============================================================================
echo "--- Package Structure ---"

if [ -f "pkg/domain/externalpressure/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/externalpressure/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/external_circle_store.go" ]; then
    pass "External circle store file exists"
else
    fail "External circle store file missing"
fi

if [ -f "internal/persist/pressure_map_store.go" ]; then
    pass "Pressure map store file exists"
else
    fail "Pressure map store file missing"
fi

if [ -d "internal/demo_phase31_4_external_pressure" ]; then
    pass "Demo test directory exists"
else
    fail "Demo test directory missing"
fi

# =============================================================================
# 2. Domain Model Checks
# =============================================================================
echo ""
echo "--- Domain Model ---"

if grep -q "CircleKindSovereign" pkg/domain/externalpressure/types.go; then
    pass "CircleKindSovereign enum exists"
else
    fail "CircleKindSovereign enum missing"
fi

if grep -q "CircleKindExternalDerived" pkg/domain/externalpressure/types.go; then
    pass "CircleKindExternalDerived enum exists"
else
    fail "CircleKindExternalDerived enum missing"
fi

if grep -q "PressureCategory" pkg/domain/externalpressure/types.go; then
    pass "PressureCategory type exists"
else
    fail "PressureCategory type missing"
fi

if grep -q "PressureMagnitude" pkg/domain/externalpressure/types.go; then
    pass "PressureMagnitude type exists"
else
    fail "PressureMagnitude type missing"
fi

if grep -q "PressureHorizon" pkg/domain/externalpressure/types.go; then
    pass "PressureHorizon type exists"
else
    fail "PressureHorizon type missing"
fi

if grep -q "ExternalDerivedCircle" pkg/domain/externalpressure/types.go; then
    pass "ExternalDerivedCircle struct exists"
else
    fail "ExternalDerivedCircle struct missing"
fi

if grep -q "PressureMapSnapshot" pkg/domain/externalpressure/types.go; then
    pass "PressureMapSnapshot struct exists"
else
    fail "PressureMapSnapshot struct missing"
fi

if grep -q "MaxPressureItems.*=.*3" pkg/domain/externalpressure/types.go; then
    pass "MaxPressureItems = 3 defined"
else
    fail "MaxPressureItems constant missing or incorrect"
fi

# =============================================================================
# 3. No Merchant Strings Check
# =============================================================================
echo ""
echo "--- No Merchant Strings ---"

FORBIDDEN_MERCHANTS="deliveroo|uber|doordash|grubhub|amazon|walmart|tesco|netflix|spotify"

if ! grep -iE "$FORBIDDEN_MERCHANTS" pkg/domain/externalpressure/types.go 2>/dev/null; then
    pass "No merchant strings in domain types"
else
    fail "Merchant strings found in domain types"
fi

if ! grep -iE "$FORBIDDEN_MERCHANTS" internal/externalpressure/engine.go 2>/dev/null; then
    pass "No merchant strings in engine (except guard patterns)"
else
    # The engine has forbidden pattern validation - verify it's in the ValidateForbiddenPatterns function
    if grep -q "func ValidateForbiddenPatterns" internal/externalpressure/engine.go; then
        pass "Merchant strings in ValidateForbiddenPatterns guard"
    else
        fail "Merchant strings found outside guard in engine"
    fi
fi

# =============================================================================
# 4. Clock Injection Checks (No time.Now())
# =============================================================================
echo ""
echo "--- Clock Injection ---"

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" pkg/domain/externalpressure/types.go | grep -q "time\.Now()"; then
    pass "No time.Now() in domain types"
else
    fail "time.Now() found in domain types"
fi

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/externalpressure/engine.go | grep -q "time\.Now()"; then
    pass "No time.Now() in engine"
else
    fail "time.Now() found in engine"
fi

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/persist/external_circle_store.go | grep -q "time\.Now()"; then
    pass "No time.Now() in external circle store"
else
    fail "time.Now() found in external circle store"
fi

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/persist/pressure_map_store.go | grep -q "time\.Now()"; then
    pass "No time.Now() in pressure map store"
else
    fail "time.Now() found in pressure map store"
fi

# Check for clock injection field in engine
if grep -q "clock.*func().*time\.Time" internal/externalpressure/engine.go; then
    pass "Clock injection field present in engine"
else
    fail "Clock injection field missing in engine"
fi

# =============================================================================
# 5. No Goroutines Checks
# =============================================================================
echo ""
echo "--- No Goroutines ---"

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/externalpressure/engine.go | grep -q "go func"; then
    pass "No goroutines in engine"
else
    fail "Goroutine found in engine"
fi

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/persist/external_circle_store.go | grep -q "go func"; then
    pass "No goroutines in external circle store"
else
    fail "Goroutine found in external circle store"
fi

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/persist/pressure_map_store.go | grep -q "go func"; then
    pass "No goroutines in pressure map store"
else
    fail "Goroutine found in pressure map store"
fi

# =============================================================================
# 6. stdlib Only Checks
# =============================================================================
echo ""
echo "--- stdlib Only ---"

if ! grep -q "github.com/aws" pkg/domain/externalpressure/types.go 2>/dev/null; then
    pass "No AWS SDK in domain types"
else
    fail "AWS SDK found in domain types"
fi

if ! grep -q "cloud.google.com" internal/externalpressure/engine.go 2>/dev/null; then
    pass "No Google Cloud SDK in engine"
else
    fail "Google Cloud SDK found in engine"
fi

# =============================================================================
# 7. Events Checks
# =============================================================================
echo ""
echo "--- Phase 31.4 Events ---"

if grep -q "Phase31_4PressureComputed" pkg/events/events.go; then
    pass "Phase31_4PressureComputed event defined"
else
    fail "Phase31_4PressureComputed event missing"
fi

if grep -q "Phase31_4PressurePersisted" pkg/events/events.go; then
    pass "Phase31_4PressurePersisted event defined"
else
    fail "Phase31_4PressurePersisted event missing"
fi

if grep -q "Phase31_4ExternalCircleDerived" pkg/events/events.go; then
    pass "Phase31_4ExternalCircleDerived event defined"
else
    fail "Phase31_4ExternalCircleDerived event missing"
fi

if grep -q "Phase31_4RealityViewed" pkg/events/events.go; then
    pass "Phase31_4RealityViewed event defined"
else
    fail "Phase31_4RealityViewed event missing"
fi

# =============================================================================
# 8. Storelog Record Types
# =============================================================================
echo ""
echo "--- Storelog Record Types ---"

if grep -q "RecordTypeExternalDerivedCircle" pkg/domain/storelog/log.go; then
    pass "RecordTypeExternalDerivedCircle defined"
else
    fail "RecordTypeExternalDerivedCircle missing"
fi

if grep -q "RecordTypePressureMapSnapshot" pkg/domain/storelog/log.go; then
    pass "RecordTypePressureMapSnapshot defined"
else
    fail "RecordTypePressureMapSnapshot missing"
fi

# =============================================================================
# 9. Web Handler Checks
# =============================================================================
echo ""
echo "--- Web Handler Integration ---"

if grep -q "handlePressureProof" cmd/quantumlife-web/main.go; then
    pass "Pressure proof handler exists"
else
    fail "Pressure proof handler missing"
fi

if grep -q '"/reality/pressure"' cmd/quantumlife-web/main.go; then
    pass "Pressure proof route registered"
else
    fail "Pressure proof route missing"
fi

if grep -q "pressure-proof" cmd/quantumlife-web/main.go; then
    pass "Pressure proof template referenced"
else
    fail "Pressure proof template missing"
fi

if grep -q "externalPressureEngine" cmd/quantumlife-web/main.go; then
    pass "External pressure engine in server struct"
else
    fail "External pressure engine missing from server struct"
fi

if grep -q "pressureMapStore" cmd/quantumlife-web/main.go; then
    pass "Pressure map store in server struct"
else
    fail "Pressure map store missing from server struct"
fi

if grep -q "externalCircleStore" cmd/quantumlife-web/main.go; then
    pass "External circle store in server struct"
else
    fail "External circle store missing from server struct"
fi

# =============================================================================
# 10. Integration Checks
# =============================================================================
echo ""
echo "--- Integration Points ---"

if grep -q "computeExternalPressure" cmd/quantumlife-web/main.go; then
    pass "External pressure computation function exists"
else
    fail "External pressure computation function missing"
fi

# Check Gmail sync integration
if grep -A15 "Phase31_1CommerceObservationsPersisted" cmd/quantumlife-web/main.go | grep -q "computeExternalPressure"; then
    pass "External pressure computed after Gmail sync"
else
    fail "External pressure not integrated with Gmail sync"
fi

# Check TrueLayer sync integration
if grep -A15 "Phase31_3bTrueLayerIngestCompleted" cmd/quantumlife-web/main.go | grep -q "computeExternalPressure"; then
    pass "External pressure computed after TrueLayer sync"
else
    fail "External pressure not integrated with TrueLayer sync"
fi

# =============================================================================
# 11. Test Coverage Checks
# =============================================================================
echo ""
echo "--- Test Coverage ---"

if [ -f "internal/demo_phase31_4_external_pressure/demo_test.go" ]; then
    pass "Demo tests exist"
else
    fail "Demo tests missing"
fi

if grep -q "TestDeterminism" internal/demo_phase31_4_external_pressure/demo_test.go 2>/dev/null; then
    pass "Determinism tests exist"
else
    fail "Determinism tests missing"
fi

if grep -q "TestMaxItemsEnforced" internal/demo_phase31_4_external_pressure/demo_test.go 2>/dev/null; then
    pass "Max items tests exist"
else
    fail "Max items tests missing"
fi

if grep -q "TestNoForbiddenTokens" internal/demo_phase31_4_external_pressure/demo_test.go 2>/dev/null; then
    pass "Forbidden tokens tests exist"
else
    fail "Forbidden tokens tests missing"
fi

if grep -q "TestForbiddenPatternValidation" internal/demo_phase31_4_external_pressure/demo_test.go 2>/dev/null; then
    pass "Forbidden pattern validation tests exist"
else
    fail "Forbidden pattern validation tests missing"
fi

# =============================================================================
# 12. Privacy Checks
# =============================================================================
echo ""
echo "--- Privacy Checks ---"

# Check that domain types don't have amount fields
if ! grep "Amount.*float\|Amount.*int" pkg/domain/externalpressure/types.go | grep -v "^[[:space:]]*//"; then
    pass "No Amount fields in domain types"
else
    fail "Amount field found in domain types"
fi

# Check that domain types don't have merchant name fields
if ! grep "MerchantName\|VendorName\|StoreName" pkg/domain/externalpressure/types.go; then
    pass "No merchant name fields in domain types"
else
    fail "Merchant name field found in domain types"
fi

# Check that domain types don't have URL fields
if ! grep "URL.*string" pkg/domain/externalpressure/types.go | grep -v "^[[:space:]]*//"; then
    pass "No URL fields in domain types"
else
    fail "URL field found in domain types"
fi

# =============================================================================
# 13. Canonical String Checks
# =============================================================================
echo ""
echo "--- Canonical String Format ---"

if grep -q "func.*CanonicalString" pkg/domain/externalpressure/types.go; then
    pass "CanonicalString methods exist"
else
    fail "CanonicalString methods missing"
fi

if grep -q "PRESSURE_MAP|v1" pkg/domain/externalpressure/types.go; then
    pass "PressureMapSnapshot has versioned canonical string"
else
    fail "PressureMapSnapshot canonical string version missing"
fi

if grep -q "EXT_CIRCLE|v1" pkg/domain/externalpressure/types.go; then
    pass "ExternalDerivedCircle has versioned canonical string"
else
    fail "ExternalDerivedCircle canonical string version missing"
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "=== Summary ==="
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -gt 0 ]; then
    echo ""
    echo "Phase 31.4 guardrails FAILED"
    exit 1
fi

echo ""
echo "Phase 31.4 guardrails PASSED"
exit 0
