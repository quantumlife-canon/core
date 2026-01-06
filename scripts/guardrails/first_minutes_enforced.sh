#!/usr/bin/env bash
#
# first_minutes_enforced.sh
#
# Phase 26B: First Five Minutes Proof Guardrails
#
# This is NOT analytics. This is NOT telemetry. This is narrative proof.
#
# Enforces:
# 1. Package structure and stdlib-only
# 2. No time.Now() in first minutes packages
# 3. No goroutines in first minutes packages
# 4. Domain model types exist
# 5. Engine implementation
# 6. Persistence constraints
# 7. Events defined
# 8. Web routes exist
# 9. Privacy (no forbidden tokens)
# 10. No analytics patterns
#
# Exit codes:
#   0 - All checks passed
#   1 - One or more checks failed
#
# Reference: docs/ADR/ADR-0056-phase26B-first-five-minutes-proof.md

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Phase 26B: First Five Minutes Proof Guardrails"
echo "=========================================="
echo "This is NOT analytics. This is narrative proof."
echo ""

pass_count=0
fail_count=0

check_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    pass_count=$((pass_count + 1))
}

check_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    fail_count=$((fail_count + 1))
}

# ==============================================================================
# Section 1: Package structure and stdlib-only
# ==============================================================================

echo "Section 1: Package structure and stdlib-only"
echo ""

# Check domain package exists
if [[ -d "${REPO_ROOT}/pkg/domain/firstminutes" ]]; then
    check_pass "pkg/domain/firstminutes package exists"
else
    check_fail "pkg/domain/firstminutes package missing"
fi

# Check domain types.go exists
if [[ -f "${REPO_ROOT}/pkg/domain/firstminutes/types.go" ]]; then
    check_pass "types.go exists in domain"
else
    check_fail "types.go missing in domain"
fi

# Check internal engine package exists
if [[ -d "${REPO_ROOT}/internal/firstminutes" ]]; then
    check_pass "internal/firstminutes package exists"
else
    check_fail "internal/firstminutes package missing"
fi

# Check engine.go exists
if [[ -f "${REPO_ROOT}/internal/firstminutes/engine.go" ]]; then
    check_pass "engine.go exists"
else
    check_fail "engine.go missing"
fi

# Check store exists
if [[ -f "${REPO_ROOT}/internal/persist/firstminutes_store.go" ]]; then
    check_pass "firstminutes_store.go exists"
else
    check_fail "firstminutes_store.go missing"
fi

# Check no external imports in domain
domain_imports=$(grep -rh "^import\|^\t\"" "${REPO_ROOT}/pkg/domain/firstminutes" 2>/dev/null | grep -v "quantumlife/" | grep -v "^import" | grep -v "^\t\"crypto" | grep -v "^\t\"encoding" | grep -v "^\t\"strings" | grep -v "^\t\"sort" | grep -v "^$" | grep -v "^\t)" || true)
if [[ -z "$domain_imports" ]]; then
    check_pass "pkg/domain/firstminutes uses stdlib only"
else
    check_fail "pkg/domain/firstminutes has external imports: $domain_imports"
fi

# Check no external imports in engine
engine_imports=$(grep -rh "^import\|^\t\"" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "quantumlife/" | grep -v "^import" | grep -v "^\t\"crypto" | grep -v "^\t\"encoding" | grep -v "^\t\"strings" | grep -v "^\t\"time" | grep -v "^\t\"sort" | grep -v "^$" | grep -v "^\t)" || true)
if [[ -z "$engine_imports" ]]; then
    check_pass "internal/firstminutes uses stdlib only"
else
    check_fail "internal/firstminutes has external imports: $engine_imports"
fi

echo ""

# ==============================================================================
# Section 2: No time.Now() (clock injection only)
# ==============================================================================

echo "Section 2: Clock injection (no time.Now())"
echo ""

# Check for time.Now() in domain
time_now_domain=$(grep -rn "time\.Now()" "${REPO_ROOT}/pkg/domain/firstminutes" 2>/dev/null | grep -v "//" || true)
if [[ -z "$time_now_domain" ]]; then
    check_pass "No time.Now() in pkg/domain/firstminutes"
else
    check_fail "time.Now() found in pkg/domain/firstminutes: $time_now_domain"
fi

# Check for time.Now() in engine
time_now_engine=$(grep -rn "time\.Now()" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "//" || true)
if [[ -z "$time_now_engine" ]]; then
    check_pass "No time.Now() in internal/firstminutes"
else
    check_fail "time.Now() found in internal/firstminutes: $time_now_engine"
fi

# Check for time.Now() in store
time_now_store=$(grep -n "time\.Now()" "${REPO_ROOT}/internal/persist/firstminutes_store.go" 2>/dev/null | grep -v "//" || true)
if [[ -z "$time_now_store" ]]; then
    check_pass "No time.Now() in firstminutes_store.go"
else
    check_fail "time.Now() found in firstminutes_store.go: $time_now_store"
fi

echo ""

# ==============================================================================
# Section 3: No goroutines
# ==============================================================================

echo "Section 3: No goroutines"
echo ""

# Check for goroutines in domain
goroutines_domain=$(grep -rn "go\s\+func\|go\s\+[a-zA-Z]" "${REPO_ROOT}/pkg/domain/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$goroutines_domain" ]]; then
    check_pass "No goroutines in pkg/domain/firstminutes"
else
    check_fail "Goroutines found in pkg/domain/firstminutes: $goroutines_domain"
fi

# Check for goroutines in engine
goroutines_engine=$(grep -rn "go\s\+func\|go\s\+[a-zA-Z]" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$goroutines_engine" ]]; then
    check_pass "No goroutines in internal/firstminutes"
else
    check_fail "Goroutines found in internal/firstminutes: $goroutines_engine"
fi

# Check for goroutines in store
goroutines_store=$(grep -n "go\s\+func\|go\s\+[a-zA-Z]" "${REPO_ROOT}/internal/persist/firstminutes_store.go" 2>/dev/null | grep -v "^[[:space:]]*//" || true)
if [[ -z "$goroutines_store" ]]; then
    check_pass "No goroutines in firstminutes_store.go"
else
    check_fail "Goroutines found in firstminutes_store.go: $goroutines_store"
fi

echo ""

# ==============================================================================
# Section 4: Domain model types
# ==============================================================================

echo "Section 4: Domain model types"
echo ""

# Check FirstMinutesPeriod type
if grep -q "type FirstMinutesPeriod string" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
    check_pass "FirstMinutesPeriod type exists"
else
    check_fail "FirstMinutesPeriod type missing"
fi

# Check FirstMinutesSignalKind type
if grep -q "type FirstMinutesSignalKind string" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
    check_pass "FirstMinutesSignalKind type exists"
else
    check_fail "FirstMinutesSignalKind type missing"
fi

# Check signal kinds
for signal in "connected" "synced" "mirrored" "held" "action_previewed" "action_executed" "silence_preserved"; do
    if grep -q "\"$signal\"" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
        check_pass "Signal kind '$signal' exists"
    else
        check_fail "Signal kind '$signal' missing"
    fi
done

# Check MagnitudeBucket type
if grep -q "type MagnitudeBucket string" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
    check_pass "MagnitudeBucket type exists"
else
    check_fail "MagnitudeBucket type missing"
fi

# Check magnitude buckets
for bucket in "nothing" "a_few" "several"; do
    if grep -q "\"$bucket\"" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
        check_pass "Magnitude bucket '$bucket' exists"
    else
        check_fail "Magnitude bucket '$bucket' missing"
    fi
done

# Check FirstMinutesSummary struct
if grep -q "type FirstMinutesSummary struct" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
    check_pass "FirstMinutesSummary struct exists"
else
    check_fail "FirstMinutesSummary struct missing"
fi

# Check FirstMinutesDismissal struct
if grep -q "type FirstMinutesDismissal struct" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
    check_pass "FirstMinutesDismissal struct exists"
else
    check_fail "FirstMinutesDismissal struct missing"
fi

# Check CanonicalString methods
if grep -q "func.*CanonicalString" "${REPO_ROOT}/pkg/domain/firstminutes/types.go" 2>/dev/null; then
    check_pass "CanonicalString methods exist"
else
    check_fail "CanonicalString methods missing"
fi

echo ""

# ==============================================================================
# Section 5: Engine implementation
# ==============================================================================

echo "Section 5: Engine implementation"
echo ""

# Check Engine struct
if grep -q "type Engine struct" "${REPO_ROOT}/internal/firstminutes/engine.go" 2>/dev/null; then
    check_pass "Engine struct exists"
else
    check_fail "Engine struct missing"
fi

# Check clock injection
if grep -q "clock.*func().*time.Time" "${REPO_ROOT}/internal/firstminutes/engine.go" 2>/dev/null; then
    check_pass "Clock injection pattern exists"
else
    check_fail "Clock injection pattern missing"
fi

# Check NewEngine function
if grep -q "func NewEngine" "${REPO_ROOT}/internal/firstminutes/engine.go" 2>/dev/null; then
    check_pass "NewEngine function exists"
else
    check_fail "NewEngine function missing"
fi

# Check ComputeSummary method
if grep -q "func.*ComputeSummary" "${REPO_ROOT}/internal/firstminutes/engine.go" 2>/dev/null; then
    check_pass "ComputeSummary method exists"
else
    check_fail "ComputeSummary method missing"
fi

# Check ComputeCue method
if grep -q "func.*ComputeCue" "${REPO_ROOT}/internal/firstminutes/engine.go" 2>/dev/null; then
    check_pass "ComputeCue method exists"
else
    check_fail "ComputeCue method missing"
fi

# Check no Persist calls in engine (engine should not persist)
persist_in_engine=$(grep -n "\.Persist\|\.Record" "${REPO_ROOT}/internal/firstminutes/engine.go" 2>/dev/null | grep -v "//" || true)
if [[ -z "$persist_in_engine" ]]; then
    check_pass "No persistence calls in engine (no side effects)"
else
    check_fail "Persistence calls found in engine: $persist_in_engine"
fi

echo ""

# ==============================================================================
# Section 6: Persistence constraints
# ==============================================================================

echo "Section 6: Persistence constraints"
echo ""

# Check FirstMinutesStore struct
if grep -q "type FirstMinutesStore struct" "${REPO_ROOT}/internal/persist/firstminutes_store.go" 2>/dev/null; then
    check_pass "FirstMinutesStore struct exists"
else
    check_fail "FirstMinutesStore struct missing"
fi

# Check bounded retention (maxPeriods)
if grep -q "maxPeriods" "${REPO_ROOT}/internal/persist/firstminutes_store.go" 2>/dev/null; then
    check_pass "Bounded retention (maxPeriods) exists"
else
    check_fail "Bounded retention (maxPeriods) missing"
fi

# Check storelog integration
if grep -q "storelog" "${REPO_ROOT}/internal/persist/firstminutes_store.go" 2>/dev/null; then
    check_pass "Storelog integration exists"
else
    check_fail "Storelog integration missing"
fi

# Check RecordTypeFirstMinutes in storelog
if grep -q "RecordTypeFirstMinutes" "${REPO_ROOT}/pkg/domain/storelog/log.go" 2>/dev/null; then
    check_pass "FirstMinutes record types in storelog"
else
    check_fail "FirstMinutes record types missing from storelog"
fi

echo ""

# ==============================================================================
# Section 7: Events defined
# ==============================================================================

echo "Section 7: Events defined"
echo ""

# Check Phase 26B events exist
for event in "phase26b.first_minutes.computed" "phase26b.first_minutes.persisted" "phase26b.first_minutes.viewed" "phase26b.first_minutes.dismissed"; do
    if grep -q "$event" "${REPO_ROOT}/pkg/events/events.go" 2>/dev/null; then
        check_pass "Event '$event' defined"
    else
        check_fail "Event '$event' missing"
    fi
done

echo ""

# ==============================================================================
# Section 8: Web routes exist
# ==============================================================================

echo "Section 8: Web routes exist"
echo ""

# Check /first-minutes route
if grep -q '"/first-minutes"' "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "/first-minutes route exists"
else
    check_fail "/first-minutes route missing"
fi

# Check /first-minutes/dismiss route
if grep -q '"/first-minutes/dismiss"' "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "/first-minutes/dismiss route exists"
else
    check_fail "/first-minutes/dismiss route missing"
fi

# Check handleFirstMinutes handler
if grep -q "handleFirstMinutes" "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "handleFirstMinutes handler exists"
else
    check_fail "handleFirstMinutes handler missing"
fi

# Check handleFirstMinutesDismiss handler
if grep -q "handleFirstMinutesDismiss" "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "handleFirstMinutesDismiss handler exists"
else
    check_fail "handleFirstMinutesDismiss handler missing"
fi

echo ""

# ==============================================================================
# Section 9: Privacy (no forbidden tokens)
# ==============================================================================

echo "Section 9: Privacy (no forbidden tokens in UI strings)"
echo ""

# Check for @ symbols in strings (email addresses)
at_symbol=$(grep -rn '".*@.*"' "${REPO_ROOT}/pkg/domain/firstminutes" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -v "RFC3339" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$at_symbol" ]]; then
    check_pass "No @ symbols in firstminutes strings"
else
    check_fail "@ symbols found in firstminutes strings: $at_symbol"
fi

# Check for http URLs in strings
http_url=$(grep -rn '"http' "${REPO_ROOT}/pkg/domain/firstminutes" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$http_url" ]]; then
    check_pass "No http URLs in firstminutes strings"
else
    check_fail "http URLs found in firstminutes strings: $http_url"
fi

# Check for currency symbols in strings
currency=$(grep -rn '"\$\|"£\|"€' "${REPO_ROOT}/pkg/domain/firstminutes" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$currency" ]]; then
    check_pass "No currency symbols in firstminutes strings"
else
    check_fail "Currency symbols found in firstminutes strings: $currency"
fi

echo ""

# ==============================================================================
# Section 10: No analytics patterns
# ==============================================================================

echo "Section 10: No analytics patterns"
echo ""

# Check for analytics-like terms (should not exist)
# Note: We allow comments that say "NOT analytics" or "not analytics" etc.
analytics_terms=$(grep -rni "analytics\|telemetry\|tracking\|metrics\|dashboard" "${REPO_ROOT}/pkg/domain/firstminutes" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -vi "NOT analytics\|not analytics\|NOT telemetry\|not telemetry\|NOT tracking\|not tracking\|is not analytics\|is not telemetry" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$analytics_terms" ]]; then
    check_pass "No analytics-like terms in code"
else
    check_fail "Analytics-like terms found: $analytics_terms"
fi

# Check for count/number exposure (only magnitude buckets allowed)
raw_counts=$(grep -rn "Count\s*int\|count\s*int\|\.Count()" "${REPO_ROOT}/pkg/domain/firstminutes" "${REPO_ROOT}/internal/firstminutes" 2>/dev/null | grep -v "_test\.go" | grep -v "signal_count" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$raw_counts" ]]; then
    check_pass "No raw count exposure (magnitude buckets only)"
else
    check_fail "Raw count exposure found: $raw_counts"
fi

echo ""

# ==============================================================================
# Summary
# ==============================================================================

echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo -e "Passed: ${GREEN}${pass_count}${NC}"
echo -e "Failed: ${RED}${fail_count}${NC}"
echo ""

if [[ "$fail_count" -gt 0 ]]; then
    echo -e "${RED}Phase 26B guardrails FAILED${NC}"
    echo ""
    echo "Fix the above failures before proceeding."
    echo "This is NOT analytics. This is narrative proof."
    exit 1
else
    echo -e "${GREEN}Phase 26B guardrails PASSED${NC}"
    echo ""
    echo "Silence remains the success state."
    exit 0
fi
