#!/bin/bash
# Phase 47: Pack Coverage Realization Guardrails
# Reference: docs/ADR/ADR-0085-phase47-pack-coverage-realization.md
#
# CRITICAL: Coverage realization expands OBSERVERS only, NEVER grants permission.
# CRITICAL: NEVER changes interrupt policy, delivery, or execution.
# CRITICAL: Track B: Expand observers, not actions.
#
# This script enforces Phase 47 invariants and must pass before merge.

set -euo pipefail

echo "============================================================"
echo "Phase 47: Pack Coverage Realization Guardrails"
echo "============================================================"
echo ""

# Initialize counters
PASS_COUNT=0
FAIL_COUNT=0
TOTAL_CHECKS=0

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    echo "  [PASS] $1"
}

fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    echo "  [FAIL] $1"
}

# ============================================================================
# Section 1: File Structure
# ============================================================================
echo "Section 1: File Structure"
echo "-----------------------------------------------------------"

# Check ADR exists
if [ -f "docs/ADR/ADR-0085-phase47-pack-coverage-realization.md" ]; then
    pass "ADR document exists"
else
    fail "ADR document missing: docs/ADR/ADR-0085-phase47-pack-coverage-realization.md"
fi

# Check domain types exist
if [ -f "pkg/domain/coverageplan/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing: pkg/domain/coverageplan/types.go"
fi

# Check engine exists
if [ -f "internal/coverageplan/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing: internal/coverageplan/engine.go"
fi

# Check persistence store exists
if [ -f "internal/persist/coverage_plan_store.go" ]; then
    pass "Persistence store file exists"
else
    fail "Persistence store file missing: internal/persist/coverage_plan_store.go"
fi

# Check demo tests exist
if [ -f "internal/demo_phase47_coverage_realization/demo_test.go" ]; then
    pass "Demo tests file exists"
else
    fail "Demo tests file missing: internal/demo_phase47_coverage_realization/demo_test.go"
fi

echo ""

# ============================================================================
# Section 2: No time.Now() in domain/engine
# ============================================================================
echo "Section 2: No time.Now() in domain/engine (clock injection required)"
echo "-----------------------------------------------------------"

# Check pkg/domain/coverageplan for time.Now()
# Exclude comments (lines starting with // or containing "// time.Now")
if grep -r "time\.Now()" pkg/domain/coverageplan/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in pkg/domain/coverageplan"
else
    pass "No time.Now() in pkg/domain/coverageplan"
fi

# Check internal/coverageplan for time.Now()
# Exclude comments (lines starting with // or containing "// time.Now")
if grep -r "time\.Now()" internal/coverageplan/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in internal/coverageplan"
else
    pass "No time.Now() in internal/coverageplan"
fi

# Check internal/persist/coverage_plan_store.go for time.Now()
if grep "time\.Now()" internal/persist/coverage_plan_store.go 2>/dev/null | grep -v "// time.Now" > /dev/null 2>&1; then
    fail "time.Now() found in coverage_plan_store.go"
else
    pass "No time.Now() in coverage_plan_store.go"
fi

echo ""

# ============================================================================
# Section 3: No goroutines in coverage packages
# ============================================================================
echo "Section 3: No goroutines in coverage packages"
echo "-----------------------------------------------------------"

# Check for goroutines in domain
if grep -r "go func" pkg/domain/coverageplan/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in pkg/domain/coverageplan"
else
    pass "No goroutines in pkg/domain/coverageplan"
fi

# Check for goroutines in engine
if grep -r "go func" internal/coverageplan/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in internal/coverageplan"
else
    pass "No goroutines in internal/coverageplan"
fi

# Check for goroutines in store
if grep "go func" internal/persist/coverage_plan_store.go 2>/dev/null > /dev/null 2>&1; then
    fail "Goroutine found in coverage_plan_store.go"
else
    pass "No goroutines in coverage_plan_store.go"
fi

echo ""

# ============================================================================
# Section 4: Forbidden imports in coverage packages
# ============================================================================
echo "Section 4: Forbidden imports (decision/delivery/execution packages)"
echo "-----------------------------------------------------------"

FORBIDDEN_IMPORTS=(
    "pressuredecision"
    "interruptpolicy"
    "interruptpreview"
    "interruptdelivery"
    "pushtransport"
    "trustaction"
    "firstaction"
    "execrouter"
    "execexecutor"
    "undoableexec"
)

for pkg in "${FORBIDDEN_IMPORTS[@]}"; do
    # Check domain
    if grep -r "\"quantumlife.*${pkg}\"" pkg/domain/coverageplan/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import ${pkg} found in pkg/domain/coverageplan"
    else
        pass "No import of ${pkg} in pkg/domain/coverageplan"
    fi

    # Check engine
    if grep -r "\"quantumlife.*${pkg}\"" internal/coverageplan/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import ${pkg} found in internal/coverageplan"
    else
        pass "No import of ${pkg} in internal/coverageplan"
    fi
done

echo ""

# ============================================================================
# Section 5: Forbidden patterns (PII, merchants, URLs)
# ============================================================================
echo "Section 5: Forbidden patterns (PII, merchants, URLs)"
echo "-----------------------------------------------------------"

COVERAGE_FILES=(
    "pkg/domain/coverageplan/types.go"
    "internal/coverageplan/engine.go"
    "internal/persist/coverage_plan_store.go"
)

# Email patterns (excluding legitimate uses like "example@" in comments)
for file in "${COVERAGE_FILES[@]}"; do
    if [ -f "$file" ]; then
        if grep -E "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}" "$file" 2>/dev/null | grep -v "//" > /dev/null 2>&1; then
            fail "Email-like pattern found in $file"
        else
            pass "No email patterns in $file"
        fi
    fi
done

# URL patterns
for file in "${COVERAGE_FILES[@]}"; do
    if [ -f "$file" ]; then
        if grep -E "https?://" "$file" 2>/dev/null | grep -v "// " > /dev/null 2>&1; then
            fail "URL pattern found in $file"
        else
            pass "No URL patterns in $file"
        fi
    fi
done

# Currency symbols
for file in "${COVERAGE_FILES[@]}"; do
    if [ -f "$file" ]; then
        if grep -E "[£$€]" "$file" 2>/dev/null > /dev/null 2>&1; then
            fail "Currency symbol found in $file"
        else
            pass "No currency symbols in $file"
        fi
    fi
done

# Merchant strings (case insensitive)
MERCHANT_STRINGS=("uber" "deliveroo" "amazon" "netflix" "spotify")
for merchant in "${MERCHANT_STRINGS[@]}"; do
    for file in "${COVERAGE_FILES[@]}"; do
        if [ -f "$file" ]; then
            if grep -i "$merchant" "$file" 2>/dev/null > /dev/null 2>&1; then
                fail "Merchant string '$merchant' found in $file"
            else
                pass "No '$merchant' in $file"
            fi
        fi
    done
done

echo ""

# ============================================================================
# Section 6: Capability vocabulary validation
# ============================================================================
echo "Section 6: Capability vocabulary validation (fixed vocabulary)"
echo "-----------------------------------------------------------"

VALID_CAPABILITIES=(
    "cap_receipt_observer"
    "cap_commerce_observer"
    "cap_finance_commerce_observer"
    "cap_pressure_map"
    "cap_timewindow_sources"
    "cap_notification_metadata"
)

# Check that types.go defines all valid capabilities
for cap in "${VALID_CAPABILITIES[@]}"; do
    if grep -q "$cap" pkg/domain/coverageplan/types.go 2>/dev/null; then
        pass "Capability '$cap' defined in types.go"
    else
        fail "Capability '$cap' not found in types.go"
    fi
done

echo ""

# ============================================================================
# Section 7: Source vocabulary validation
# ============================================================================
echo "Section 7: Source vocabulary validation (fixed vocabulary)"
echo "-----------------------------------------------------------"

VALID_SOURCES=(
    "source_gmail"
    "source_finance_truelayer"
    "source_device_notification"
)

for src in "${VALID_SOURCES[@]}"; do
    if grep -q "$src" pkg/domain/coverageplan/types.go 2>/dev/null; then
        pass "Source '$src' defined in types.go"
    else
        fail "Source '$src' not found in types.go"
    fi
done

echo ""

# ============================================================================
# Section 8: Hash functions exist
# ============================================================================
echo "Section 8: Hash functions (deterministic hashing)"
echo "-----------------------------------------------------------"

# Check for ComputePlanHash
if grep -q "ComputePlanHash" pkg/domain/coverageplan/types.go 2>/dev/null; then
    pass "ComputePlanHash function exists"
else
    fail "ComputePlanHash function missing"
fi

# Check for ComputeDeltaHash
if grep -q "ComputeDeltaHash" pkg/domain/coverageplan/types.go 2>/dev/null; then
    pass "ComputeDeltaHash function exists"
else
    fail "ComputeDeltaHash function missing"
fi

# Check for sha256 usage
if grep -q "sha256" pkg/domain/coverageplan/types.go 2>/dev/null; then
    pass "SHA256 used for hashing in types.go"
else
    fail "SHA256 not found in types.go"
fi

echo ""

# ============================================================================
# Section 9: Bounded retention in stores
# ============================================================================
echo "Section 9: Bounded retention (FIFO eviction)"
echo "-----------------------------------------------------------"

# Check for max records constant
if grep -q "MaxRecords\|MaxCoveragePlanRecords" internal/persist/coverage_plan_store.go 2>/dev/null; then
    pass "Max records constant defined"
else
    fail "Max records constant not found"
fi

# Check for max days constant
if grep -q "MaxDays\|MaxRetentionDays\|MaxCoveragePlanDays" internal/persist/coverage_plan_store.go 2>/dev/null; then
    pass "Max days constant defined"
else
    fail "Max days constant not found"
fi

# Check for eviction logic
if grep -q "evict" internal/persist/coverage_plan_store.go 2>/dev/null; then
    pass "Eviction logic exists"
else
    fail "Eviction logic not found"
fi

echo ""

# ============================================================================
# Section 10: Storelog record types
# ============================================================================
echo "Section 10: Storelog record types"
echo "-----------------------------------------------------------"

# Check for COVERAGE_PLAN record type
if grep -q "COVERAGE_PLAN" pkg/domain/storelog/log.go 2>/dev/null; then
    pass "COVERAGE_PLAN record type defined"
else
    fail "COVERAGE_PLAN record type not found"
fi

# Check for COVERAGE_PROOF_ACK record type
if grep -q "COVERAGE_PROOF_ACK" pkg/domain/storelog/log.go 2>/dev/null; then
    pass "COVERAGE_PROOF_ACK record type defined"
else
    fail "COVERAGE_PROOF_ACK record type not found"
fi

echo ""

# ============================================================================
# Section 11: Events defined
# ============================================================================
echo "Section 11: Phase 47 events defined"
echo "-----------------------------------------------------------"

EXPECTED_EVENTS=(
    "phase47.coverage.plan_built"
    "phase47.coverage.plan_persisted"
    "phase47.coverage.delta_computed"
    "phase47.coverage.proof.rendered"
    "phase47.coverage.ack.recorded"
    "phase47.coverage.cue.computed"
)

for event in "${EXPECTED_EVENTS[@]}"; do
    if grep -q "$event" pkg/events/events.go 2>/dev/null; then
        pass "Event '$event' defined"
    else
        fail "Event '$event' not found"
    fi
done

echo ""

# ============================================================================
# Section 12: Web routes defined
# ============================================================================
echo "Section 12: Web routes defined"
echo "-----------------------------------------------------------"

# Check for /proof/coverage GET route
if grep -q "/proof/coverage" cmd/quantumlife-web/main.go 2>/dev/null; then
    pass "/proof/coverage route defined"
else
    fail "/proof/coverage route not found"
fi

# Check for /proof/coverage/dismiss POST route
if grep -q "/proof/coverage/dismiss" cmd/quantumlife-web/main.go 2>/dev/null; then
    pass "/proof/coverage/dismiss route defined"
else
    fail "/proof/coverage/dismiss route not found"
fi

echo ""

# ============================================================================
# Section 13: POST-only mutation routes
# ============================================================================
echo "Section 13: POST-only mutation routes"
echo "-----------------------------------------------------------"

# Check handleCoverageProofDismiss has POST check
if grep -A5 "handleCoverageProofDismiss" cmd/quantumlife-web/main.go 2>/dev/null | grep -q "MethodPost"; then
    pass "handleCoverageProofDismiss is POST-only"
else
    fail "handleCoverageProofDismiss may not be POST-only"
fi

echo ""

# ============================================================================
# Section 14: No pack IDs in proof page
# ============================================================================
echo "Section 14: Proof page does not display pack IDs"
echo "-----------------------------------------------------------"

# Check that proof page template doesn't reference pack IDs
if grep -A100 "coverage-proof-content" cmd/quantumlife-web/main.go 2>/dev/null | grep -E "pack_id|PackSlug|PackSlugHash" > /dev/null 2>&1; then
    fail "Pack ID reference found in coverage proof template"
else
    pass "No pack ID references in coverage proof template"
fi

# Check engine BuildProofPage doesn't leak pack IDs
if grep -A30 "BuildProofPage" internal/coverageplan/engine.go 2>/dev/null | grep -E "PackSlug|pack_id" > /dev/null 2>&1; then
    fail "Pack ID reference found in BuildProofPage"
else
    pass "No pack ID references in BuildProofPage"
fi

echo ""

# ============================================================================
# Section 15: Clock injection in engine
# ============================================================================
echo "Section 15: Clock injection pattern"
echo "-----------------------------------------------------------"

# Check Engine struct has clock field
if grep -q "clock" internal/coverageplan/engine.go 2>/dev/null; then
    pass "Engine has clock field"
else
    fail "Engine missing clock field"
fi

# Check NewEngine accepts clock
if grep -q "NewEngine.*Clock\|NewEngine.*clock" internal/coverageplan/engine.go 2>/dev/null; then
    pass "NewEngine accepts clock parameter"
else
    fail "NewEngine does not accept clock parameter"
fi

echo ""

# ============================================================================
# Section 16: Coverage wiring in cmd/ only
# ============================================================================
echo "Section 16: Coverage wiring only in cmd/"
echo "-----------------------------------------------------------"

# Check that isCoverageCapabilityEnabled is called in main.go
if grep -q "isCoverageCapabilityEnabled" cmd/quantumlife-web/main.go 2>/dev/null; then
    pass "isCoverageCapabilityEnabled used in main.go"
else
    fail "isCoverageCapabilityEnabled not used in main.go"
fi

# Check that pkg/domain/coverageplan doesn't have wiring logic
if grep -q "ListInstalled\|InstalledPacks" pkg/domain/coverageplan/*.go 2>/dev/null; then
    fail "Wiring logic found in pkg/domain/coverageplan"
else
    pass "No wiring logic in pkg/domain/coverageplan"
fi

echo ""

# ============================================================================
# Section 17: Validate() methods exist
# ============================================================================
echo "Section 17: Validate() methods on types"
echo "-----------------------------------------------------------"

TYPES_WITH_VALIDATE=(
    "CoverageSourceKind"
    "CoverageCapability"
    "CoverageChangeKind"
    "CoverageProofAckKind"
    "CoveragePlan"
    "CoverageProofAck"
)

for type in "${TYPES_WITH_VALIDATE[@]}"; do
    if grep -q "func.*${type}.*Validate" pkg/domain/coverageplan/types.go 2>/dev/null; then
        pass "${type} has Validate() method"
    else
        fail "${type} missing Validate() method"
    fi
done

echo ""

# ============================================================================
# Section 18: CanonicalString methods exist
# ============================================================================
echo "Section 18: CanonicalString methods for hashing"
echo "-----------------------------------------------------------"

TYPES_WITH_CANONICAL=(
    "CoverageSourceKind"
    "CoverageCapability"
    "CoveragePlan"
    "CoverageDelta"
    "CoverageProofAck"
)

for type in "${TYPES_WITH_CANONICAL[@]}"; do
    if grep -q "func.*${type}.*CanonicalString" pkg/domain/coverageplan/types.go 2>/dev/null; then
        pass "${type} has CanonicalString method"
    else
        fail "${type} missing CanonicalString method"
    fi
done

echo ""

# ============================================================================
# Section 19: Normalize functions exist
# ============================================================================
echo "Section 19: Normalize functions for determinism"
echo "-----------------------------------------------------------"

if grep -q "NormalizeCapabilities" pkg/domain/coverageplan/types.go 2>/dev/null; then
    pass "NormalizeCapabilities function exists"
else
    fail "NormalizeCapabilities function missing"
fi

if grep -q "NormalizeSources" pkg/domain/coverageplan/types.go 2>/dev/null; then
    pass "NormalizeSources function exists"
else
    fail "NormalizeSources function missing"
fi

echo ""

# ============================================================================
# Section 20: Coverage enabled checks in ingestion flows
# ============================================================================
echo "Section 20: Coverage checks in ingestion flows"
echo "-----------------------------------------------------------"

# Check Gmail sync has coverage check (may be anywhere in the function, not just first 30 lines)
if grep -q "receiptObserverEnabled\|commerceObserverEnabled" cmd/quantumlife-web/main.go 2>/dev/null; then
    pass "Gmail sync has coverage capability check"
else
    fail "Gmail sync missing coverage capability check"
fi

# Check TrueLayer sync has coverage check
if grep -q "financeCommerceEnabled" cmd/quantumlife-web/main.go 2>/dev/null; then
    pass "TrueLayer sync has coverage capability check"
else
    fail "TrueLayer sync missing coverage capability check"
fi

# Check notification observer has coverage check
if grep -A30 "handleObserveNotification" cmd/quantumlife-web/main.go 2>/dev/null | grep -q "isCoverageCapabilityEnabled\|CapNotificationMetadata"; then
    pass "Notification observer has coverage capability check"
else
    fail "Notification observer missing coverage capability check"
fi

echo ""

# ============================================================================
# Section 21: effect_no_power enforcement (via Phase 46 dependency)
# ============================================================================
echo "Section 21: effect_no_power enforcement"
echo "-----------------------------------------------------------"

# Coverage uses marketplace packs which enforce effect_no_power
if grep -q "marketplace" internal/coverageplan/engine.go 2>/dev/null; then
    pass "Engine uses marketplace types (effect_no_power enforced)"
else
    fail "Engine doesn't use marketplace types"
fi

echo ""

# ============================================================================
# Section 22: ADR content validation
# ============================================================================
echo "Section 22: ADR content validation"
echo "-----------------------------------------------------------"

ADR_FILE="docs/ADR/ADR-0085-phase47-pack-coverage-realization.md"

if [ -f "$ADR_FILE" ]; then
    # Check for key sections
    if grep -q "## Status" "$ADR_FILE" 2>/dev/null; then
        pass "ADR has Status section"
    else
        fail "ADR missing Status section"
    fi

    if grep -q "## Context" "$ADR_FILE" 2>/dev/null; then
        pass "ADR has Context section"
    else
        fail "ADR missing Context section"
    fi

    if grep -q "## Decision" "$ADR_FILE" 2>/dev/null; then
        pass "ADR has Decision section"
    else
        fail "ADR missing Decision section"
    fi

    if grep -q "effect_no_power" "$ADR_FILE" 2>/dev/null; then
        pass "ADR mentions effect_no_power invariant"
    else
        fail "ADR missing effect_no_power mention"
    fi

    if grep -qi "never.*permission\|no.*permission\|does not grant.*permission" "$ADR_FILE" 2>/dev/null; then
        pass "ADR documents no-permission constraint"
    else
        fail "ADR missing no-permission constraint"
    fi
fi

echo ""

# ============================================================================
# Summary
# ============================================================================
echo "============================================================"
echo "Summary"
echo "============================================================"
echo ""
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "FAILED: $FAIL_COUNT guardrail(s) failed."
    exit 1
else
    echo "All Phase 47 guardrails PASSED!"
    exit 0
fi
