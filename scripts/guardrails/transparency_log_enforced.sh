#!/bin/bash
# Phase 51: Transparency Log / Claim Ledger Guardrails
# ===========================================================
# This script enforces Phase 51 invariants:
# - No power - observation/proof only
# - Hash-only - never store raw identifiers
# - Append-only - no mutation/deletion
# - Deterministic - same inputs = same outputs
# - No forbidden imports
# - No goroutines
# - No time.Now() in pkg/internal
#
# Target: 120+ checks
# ===========================================================

set -e

PASS_COUNT=0
FAIL_COUNT=0
TOTAL_CHECKS=0

pass() {
    echo "  ✓ $1"
    PASS_COUNT=$((PASS_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
}

fail() {
    echo "  ✗ $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
}

section() {
    echo ""
    echo "=== $1 ==="
}

# ===========================================================
# Section 1: File Existence
# ===========================================================
section "File Existence"

if [ -f "pkg/domain/transparencylog/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/transparencylog/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/transparency_log_store.go" ]; then
    pass "Persistence store file exists"
else
    fail "Persistence store file missing"
fi

if [ -d "internal/demo_phase51_transparency_log" ]; then
    pass "Demo test directory exists"
else
    fail "Demo test directory missing"
fi

# ===========================================================
# Section 2: Domain Types - Enum Validation
# ===========================================================
section "Domain Types - Enum Validation"

DOMAIN_FILE="pkg/domain/transparencylog/types.go"

if grep -q 'LogSignedVendorClaim.*LogKind.*=.*"log_signed_vendor_claim"' "$DOMAIN_FILE"; then
    pass "LogSignedVendorClaim enum defined"
else
    fail "LogSignedVendorClaim enum missing"
fi

if grep -q 'LogSignedPackManifest.*LogKind.*=.*"log_signed_pack_manifest"' "$DOMAIN_FILE"; then
    pass "LogSignedPackManifest enum defined"
else
    fail "LogSignedPackManifest enum missing"
fi

if grep -q 'ProvUserSupplied.*LogProvenanceBucket.*=.*"prov_user_supplied"' "$DOMAIN_FILE"; then
    pass "ProvUserSupplied enum defined"
else
    fail "ProvUserSupplied enum missing"
fi

if grep -q 'ProvMarketplace.*LogProvenanceBucket.*=.*"prov_marketplace"' "$DOMAIN_FILE"; then
    pass "ProvMarketplace enum defined"
else
    fail "ProvMarketplace enum missing"
fi

if grep -q 'ProvUnknown.*LogProvenanceBucket.*=.*"prov_unknown"' "$DOMAIN_FILE"; then
    pass "ProvUnknown enum defined"
else
    fail "ProvUnknown enum missing"
fi

if grep -q 'MagnitudeNothing.*MagnitudeBucket.*=.*"nothing"' "$DOMAIN_FILE"; then
    pass "MagnitudeNothing enum defined"
else
    fail "MagnitudeNothing enum missing"
fi

if grep -q 'MagnitudeAFew.*MagnitudeBucket.*=.*"a_few"' "$DOMAIN_FILE"; then
    pass "MagnitudeAFew enum defined"
else
    fail "MagnitudeAFew enum missing"
fi

if grep -q 'MagnitudeSeveral.*MagnitudeBucket.*=.*"several"' "$DOMAIN_FILE"; then
    pass "MagnitudeSeveral enum defined"
else
    fail "MagnitudeSeveral enum missing"
fi

# ===========================================================
# Section 3: Domain Types - Struct Fields
# ===========================================================
section "Domain Types - Struct Fields"

if grep -q 'type KeyFingerprint string' "$DOMAIN_FILE"; then
    pass "KeyFingerprint type defined"
else
    fail "KeyFingerprint type missing"
fi

if grep -q 'type PeriodKey string' "$DOMAIN_FILE"; then
    pass "PeriodKey type defined"
else
    fail "PeriodKey type missing"
fi

if grep -q 'type LogLineHash string' "$DOMAIN_FILE"; then
    pass "LogLineHash type defined"
else
    fail "LogLineHash type missing"
fi

if grep -q 'type LogEntryID string' "$DOMAIN_FILE"; then
    pass "LogEntryID type defined"
else
    fail "LogEntryID type missing"
fi

if grep -q 'type RefHash string' "$DOMAIN_FILE"; then
    pass "RefHash type defined"
else
    fail "RefHash type missing"
fi

if grep -q 'type TransparencyLogEntry struct' "$DOMAIN_FILE"; then
    pass "TransparencyLogEntry struct defined"
else
    fail "TransparencyLogEntry struct missing"
fi

if grep -q 'type TransparencyLogPage struct' "$DOMAIN_FILE"; then
    pass "TransparencyLogPage struct defined"
else
    fail "TransparencyLogPage struct missing"
fi

if grep -q 'type TransparencyLogLineView struct' "$DOMAIN_FILE"; then
    pass "TransparencyLogLineView struct defined"
else
    fail "TransparencyLogLineView struct missing"
fi

if grep -q 'type TransparencyLogSummary struct' "$DOMAIN_FILE"; then
    pass "TransparencyLogSummary struct defined"
else
    fail "TransparencyLogSummary struct missing"
fi

if grep -q 'type TransparencyLogExportBundle struct' "$DOMAIN_FILE"; then
    pass "TransparencyLogExportBundle struct defined"
else
    fail "TransparencyLogExportBundle struct missing"
fi

# ===========================================================
# Section 4: Domain Types - Required Methods
# ===========================================================
section "Domain Types - Required Methods"

if grep -q 'func (k LogKind) Validate()' "$DOMAIN_FILE"; then
    pass "LogKind.Validate() method exists"
else
    fail "LogKind.Validate() method missing"
fi

if grep -q 'func (p LogProvenanceBucket) Validate()' "$DOMAIN_FILE"; then
    pass "LogProvenanceBucket.Validate() method exists"
else
    fail "LogProvenanceBucket.Validate() method missing"
fi

if grep -q 'func (e TransparencyLogEntry) CanonicalLine()' "$DOMAIN_FILE"; then
    pass "TransparencyLogEntry.CanonicalLine() method exists"
else
    fail "TransparencyLogEntry.CanonicalLine() method missing"
fi

if grep -q 'func (e TransparencyLogEntry) ComputeLineHash()' "$DOMAIN_FILE"; then
    pass "TransparencyLogEntry.ComputeLineHash() method exists"
else
    fail "TransparencyLogEntry.ComputeLineHash() method missing"
fi

if grep -q 'func (e TransparencyLogEntry) Validate()' "$DOMAIN_FILE"; then
    pass "TransparencyLogEntry.Validate() method exists"
else
    fail "TransparencyLogEntry.Validate() method missing"
fi

if grep -q 'func (p TransparencyLogPage) ComputeStatusHash()' "$DOMAIN_FILE"; then
    pass "TransparencyLogPage.ComputeStatusHash() method exists"
else
    fail "TransparencyLogPage.ComputeStatusHash() method missing"
fi

if grep -q 'func (b TransparencyLogExportBundle) Validate()' "$DOMAIN_FILE"; then
    pass "TransparencyLogExportBundle.Validate() method exists"
else
    fail "TransparencyLogExportBundle.Validate() method missing"
fi

if grep -q 'func (b TransparencyLogExportBundle) ComputeBundleHash()' "$DOMAIN_FILE"; then
    pass "TransparencyLogExportBundle.ComputeBundleHash() method exists"
else
    fail "TransparencyLogExportBundle.ComputeBundleHash() method missing"
fi

# ===========================================================
# Section 5: Pipe-Delimited Canonical Strings
# ===========================================================
section "Pipe-Delimited Canonical Strings"

if grep -q 'strings.Join.*"|"' "$DOMAIN_FILE"; then
    pass "Canonical strings use pipe delimiter"
else
    fail "Canonical strings do not use pipe delimiter"
fi

if grep -q 'v1|period=' "$DOMAIN_FILE"; then
    pass "Canonical line format correct"
else
    fail "Canonical line format missing"
fi

if grep -q 'bundle|v1|period=' "$DOMAIN_FILE"; then
    pass "Bundle format correct"
else
    fail "Bundle format missing"
fi

# No JSON in domain package
if grep -ri "json.Marshal\|json.Unmarshal" pkg/domain/transparencylog/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "JSON marshaling found in domain package"
else
    pass "No JSON marshaling in domain package"
fi

# ===========================================================
# Section 6: No time.Now() in Domain/Engine
# ===========================================================
section "No time.Now() in Domain/Engine/Store"

if grep -r "time\.Now()" pkg/domain/transparencylog/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in domain package"
else
    pass "No time.Now() in domain package"
fi

if grep -r "time\.Now()" internal/transparencylog/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in engine package"
else
    pass "No time.Now() in engine package"
fi

if grep -r "time\.Now()" internal/persist/transparency_log_store.go 2>/dev/null | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in store"
else
    pass "No time.Now() in store"
fi

# ===========================================================
# Section 7: No Goroutines
# ===========================================================
section "No Goroutines"

if grep -r "go func" pkg/domain/transparencylog/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in domain package"
else
    pass "No goroutines in domain package"
fi

if grep -r "go func" internal/transparencylog/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in engine package"
else
    pass "No goroutines in engine package"
fi

if grep -r "go func" internal/persist/transparency_log_store.go 2>/dev/null > /dev/null 2>&1; then
    fail "Goroutine found in store"
else
    pass "No goroutines in store"
fi

# ===========================================================
# Section 8: Forbidden Imports (No Power)
# ===========================================================
section "Forbidden Imports (No Power)"

FORBIDDEN_IMPORTS=(
    "pressuredecision"
    "interruptpolicy"
    "interruptpreview"
    "pushtransport"
    "interruptdelivery"
    "enforcementclamp"
    "vendorcontract"
)

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" pkg/domain/transparencylog/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in domain package"
    else
        pass "No forbidden import $import in domain"
    fi
done

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" internal/transparencylog/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in engine package"
    else
        pass "No forbidden import $import in engine"
    fi
done

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" internal/persist/transparency_log_store.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in store"
    else
        pass "No forbidden import $import in store"
    fi
done

# ===========================================================
# Section 9: Stdlib Only
# ===========================================================
section "Stdlib Only (No External Dependencies)"

if grep -E "github.com|gopkg.in" pkg/domain/transparencylog/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "External dependency in domain package"
else
    pass "No external dependencies in domain package"
fi

if grep -E "github.com|gopkg.in" internal/transparencylog/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "External dependency in engine package"
else
    pass "No external dependencies in engine package"
fi

if grep -E "github.com|gopkg.in" internal/persist/transparency_log_store.go 2>/dev/null > /dev/null 2>&1; then
    fail "External dependency in store"
else
    pass "No external dependencies in store"
fi

# ===========================================================
# Section 10: Engine Functions
# ===========================================================
section "Engine Functions"

ENGINE_FILE="internal/transparencylog/engine.go"

if grep -q 'func (e \*Engine) BuildEntries' "$ENGINE_FILE"; then
    pass "Engine.BuildEntries() method exists"
else
    fail "Engine.BuildEntries() method missing"
fi

if grep -q 'func (e \*Engine) BuildPage' "$ENGINE_FILE"; then
    pass "Engine.BuildPage() method exists"
else
    fail "Engine.BuildPage() method missing"
fi

if grep -q 'func (e \*Engine) BuildExportBundle' "$ENGINE_FILE"; then
    pass "Engine.BuildExportBundle() method exists"
else
    fail "Engine.BuildExportBundle() method missing"
fi

if grep -q 'func (e \*Engine) ValidateExportBundle' "$ENGINE_FILE"; then
    pass "Engine.ValidateExportBundle() method exists"
else
    fail "Engine.ValidateExportBundle() method missing"
fi

if grep -q 'func (e \*Engine) ImportBundle' "$ENGINE_FILE"; then
    pass "Engine.ImportBundle() method exists"
else
    fail "Engine.ImportBundle() method missing"
fi

if grep -q 'sortEntries' "$ENGINE_FILE"; then
    pass "Deterministic sorting exists"
else
    fail "Deterministic sorting missing"
fi

if grep -q 'MaxDisplayLines.*=.*12' "$ENGINE_FILE"; then
    pass "MaxDisplayLines constant defined as 12"
else
    fail "MaxDisplayLines constant missing or wrong value"
fi

# ===========================================================
# Section 11: Append-Only Store
# ===========================================================
section "Append-Only Store"

STORE_FILE="internal/persist/transparency_log_store.go"

if grep -q 'func (s \*TransparencyLogStore) Append' "$STORE_FILE"; then
    pass "Store.Append() method exists"
else
    fail "Store.Append() method missing"
fi

if grep -q 'func (s \*TransparencyLogStore) ListByPeriod' "$STORE_FILE"; then
    pass "Store.ListByPeriod() method exists"
else
    fail "Store.ListByPeriod() method missing"
fi

if grep -q 'func (s \*TransparencyLogStore) ReplayAll' "$STORE_FILE"; then
    pass "Store.ReplayAll() method exists"
else
    fail "Store.ReplayAll() method missing"
fi

if grep -q 'dedupIndex' "$STORE_FILE"; then
    pass "Store has dedup index"
else
    fail "Store missing dedup index"
fi

if grep -q 'TransparencyLogMaxEntries.*=.*5000' "$STORE_FILE"; then
    pass "Store has max entries constant (5000)"
else
    fail "Store missing max entries constant"
fi

if grep -q 'TransparencyLogMaxRetentionDays.*=.*30' "$STORE_FILE"; then
    pass "Store has max retention days constant (30)"
else
    fail "Store missing max retention days constant"
fi

if grep -q 'evictOldEntriesLocked' "$STORE_FILE"; then
    pass "Store has eviction method"
else
    fail "Store missing eviction method"
fi

# ===========================================================
# Section 12: Events
# ===========================================================
section "Events"

EVENTS_FILE="pkg/events/events.go"

if grep -q "Phase51TransparencyRequested" "$EVENTS_FILE"; then
    pass "Phase51TransparencyRequested event defined"
else
    fail "Phase51TransparencyRequested event missing"
fi

if grep -q "Phase51TransparencyRendered" "$EVENTS_FILE"; then
    pass "Phase51TransparencyRendered event defined"
else
    fail "Phase51TransparencyRendered event missing"
fi

if grep -q "Phase51TransparencyExported" "$EVENTS_FILE"; then
    pass "Phase51TransparencyExported event defined"
else
    fail "Phase51TransparencyExported event missing"
fi

if grep -q "Phase51TransparencyImported" "$EVENTS_FILE"; then
    pass "Phase51TransparencyImported event defined"
else
    fail "Phase51TransparencyImported event missing"
fi

if grep -q "Phase51TransparencyAppended" "$EVENTS_FILE"; then
    pass "Phase51TransparencyAppended event defined"
else
    fail "Phase51TransparencyAppended event missing"
fi

if grep -q "Phase51TransparencyDeduped" "$EVENTS_FILE"; then
    pass "Phase51TransparencyDeduped event defined"
else
    fail "Phase51TransparencyDeduped event missing"
fi

# ===========================================================
# Section 13: Storelog Record Types
# ===========================================================
section "Storelog Record Types"

STORELOG_FILE="pkg/domain/storelog/log.go"

if grep -q "RecordTypeTransparencyLogEntry" "$STORELOG_FILE"; then
    pass "RecordTypeTransparencyLogEntry defined"
else
    fail "RecordTypeTransparencyLogEntry missing"
fi

if grep -q "RecordTypeTransparencyLogImport" "$STORELOG_FILE"; then
    pass "RecordTypeTransparencyLogImport defined"
else
    fail "RecordTypeTransparencyLogImport missing"
fi

# ===========================================================
# Section 14: Web Routes
# ===========================================================
section "Web Routes"

MAIN_FILE="cmd/quantumlife-web/main.go"

if grep -q '"/proof/transparency"' "$MAIN_FILE"; then
    pass "GET /proof/transparency route defined"
else
    fail "GET /proof/transparency route missing"
fi

if grep -q '"/proof/transparency/export"' "$MAIN_FILE"; then
    pass "GET /proof/transparency/export route defined"
else
    fail "GET /proof/transparency/export route missing"
fi

if grep -q '"/proof/transparency/import"' "$MAIN_FILE"; then
    pass "POST /proof/transparency/import route defined"
else
    fail "POST /proof/transparency/import route missing"
fi

# ===========================================================
# Section 15: Forbidden Patterns in Domain/Engine
# ===========================================================
section "Forbidden Patterns"

FORBIDDEN_PATTERNS=("@" "http://" "https://" "vendorID" "vendor_id" "packID" "pack_id" "merchant" "periodKey" "period_key")

# Check domain file doesn't have forbidden patterns in string literals (except in validation code)
for pattern in "vendorID" "packID" "merchant" "@" "http://"; do
    # Skip if it's in the ForbiddenPatterns list definition
    if grep -v "ForbiddenPatterns" "$DOMAIN_FILE" | grep -v "//" | grep -q "\"$pattern\""; then
        # Allow if it's part of validation
        if grep -B5 -A5 "\"$pattern\"" "$DOMAIN_FILE" | grep -q "ValidateNoForbiddenPatterns\|ForbiddenPatterns"; then
            pass "Pattern '$pattern' only in validation code"
        else
            fail "Forbidden pattern '$pattern' found in domain types"
        fi
    else
        pass "No forbidden pattern '$pattern' in domain"
    fi
done

# ===========================================================
# Section 16: Export is text/plain
# ===========================================================
section "Export Format"

if grep -A30 "handleTransparencyLogExport" "$MAIN_FILE" | grep -q 'text/plain'; then
    pass "Export handler returns text/plain"
else
    fail "Export handler may not return text/plain"
fi

if grep -A30 "handleTransparencyLogExport" "$MAIN_FILE" | grep -q 'json'; then
    fail "Export handler may return JSON"
else
    pass "Export handler does not return JSON"
fi

# ===========================================================
# Section 17: Web Handlers Reject Forbidden Params
# ===========================================================
section "Web Handlers Reject Forbidden Params"

if grep -A30 "handleTransparencyLog" "$MAIN_FILE" | grep -q 'forbiddenParams'; then
    pass "View handler checks forbidden params"
else
    fail "View handler may not check forbidden params"
fi

if grep -A30 "handleTransparencyLogImport" "$MAIN_FILE" | grep -q 'forbiddenParams'; then
    pass "Import handler checks forbidden params"
else
    fail "Import handler may not check forbidden params"
fi

# ===========================================================
# Section 18: Status Hash Rendered
# ===========================================================
section "Status Hash Rendered"

if grep -q 'StatusHash' "$DOMAIN_FILE"; then
    pass "StatusHash field exists in domain"
else
    fail "StatusHash field missing"
fi

if grep -A100 "renderTransparencyLogPage" "$MAIN_FILE" | grep -q 'StatusHash\|status-hash'; then
    pass "Status hash is rendered in page"
else
    fail "Status hash may not be rendered"
fi

# ===========================================================
# Section 19: Deterministic Ordering
# ===========================================================
section "Deterministic Ordering"

if grep -q 'sort.Slice' "$ENGINE_FILE"; then
    pass "Engine uses sort.Slice for determinism"
else
    fail "Engine may not use deterministic sorting"
fi

if grep -q 'sort.Strings' "$DOMAIN_FILE"; then
    pass "Domain uses sort.Strings for determinism"
else
    fail "Domain may not use deterministic sorting"
fi

# ===========================================================
# Section 20: Demo Tests Exist
# ===========================================================
section "Demo Tests"

if [ -f "internal/demo_phase51_transparency_log/demo_test.go" ]; then
    pass "Demo test file exists"
else
    fail "Demo test file missing"
fi

# Count test functions
if [ -f "internal/demo_phase51_transparency_log/demo_test.go" ]; then
    TEST_COUNT=$(grep -c "func Test" internal/demo_phase51_transparency_log/demo_test.go 2>/dev/null || echo "0")
    if [ "$TEST_COUNT" -ge 30 ]; then
        pass "Demo tests have >= 30 test functions ($TEST_COUNT found)"
    else
        fail "Demo tests have < 30 test functions ($TEST_COUNT found)"
    fi
fi

# ===========================================================
# Section 21: No Decision Logic Imports
# ===========================================================
section "No Decision Logic Imports in Phase 51"

DECISION_PACKAGES=("pressuredecision" "interruptpolicy" "interruptpreview" "pushtransport" "interruptdelivery" "enforcementclamp")

for pkg in "${DECISION_PACKAGES[@]}"; do
    if grep -rq "\"quantumlife.*/$pkg\"" pkg/domain/transparencylog/ internal/transparencylog/ internal/persist/transparency_log_store.go 2>/dev/null; then
        fail "Phase 51 imports forbidden package: $pkg"
    else
        pass "Phase 51 does not import: $pkg"
    fi
done

# ===========================================================
# Summary
# ===========================================================
echo ""
echo "==========================================="
echo "Phase 51 Guardrails Summary"
echo "==========================================="
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -eq 0 ]; then
    echo ""
    echo "All Phase 51 guardrails passed!"
    exit 0
else
    echo ""
    echo "Phase 51 guardrails FAILED. Fix issues above."
    exit 1
fi
