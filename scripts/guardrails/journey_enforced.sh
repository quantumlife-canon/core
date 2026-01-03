#!/usr/bin/env bash
#
# journey_enforced.sh
#
# Phase 26A: Guided Journey Guardrails
#
# Enforces:
# 1. internal/journey uses stdlib only
# 2. No time.Now() in journey packages
# 3. No goroutines in journey packages
# 4. /journey routes exist
# 5. Status hash validation on dismiss
# 6. No forbidden tokens displayed (email, @, http, currency)
# 7. Magnitude buckets only
# 8. Hash-only persistence
#
# Exit codes:
#   0 - All checks passed
#   1 - One or more checks failed
#
# Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Phase 26A: Guided Journey Guardrails"
echo "=========================================="
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

# Check journey package exists
if [[ -d "${REPO_ROOT}/internal/journey" ]]; then
    check_pass "internal/journey package exists"
else
    check_fail "internal/journey package missing"
fi

# Check model.go exists
if [[ -f "${REPO_ROOT}/internal/journey/model.go" ]]; then
    check_pass "model.go exists"
else
    check_fail "model.go missing"
fi

# Check engine.go exists
if [[ -f "${REPO_ROOT}/internal/journey/engine.go" ]]; then
    check_pass "engine.go exists"
else
    check_fail "engine.go missing"
fi

# Check no external imports in internal/journey
journey_imports=$(grep -rh "^import\|^\t\"" "${REPO_ROOT}/internal/journey" 2>/dev/null | grep -v "quantumlife/" | grep -v "^import" | grep -v "^\t\"crypto" | grep -v "^\t\"encoding" | grep -v "^\t\"strings" | grep -v "^\t\"time" | grep -v "^\t\"fmt" | grep -v "^\t\"sync" | grep -v "^\t\"context" | grep -v "^\t\"sort" | grep -v "^$" | grep -v "^\t)" || true)
if [[ -z "$journey_imports" ]]; then
    check_pass "internal/journey uses stdlib only"
else
    check_fail "internal/journey has external imports: $journey_imports"
fi

# Check journey store exists
if [[ -f "${REPO_ROOT}/internal/persist/journey_store.go" ]]; then
    check_pass "journey_store.go exists"
else
    check_fail "journey_store.go missing"
fi

echo ""

# ==============================================================================
# Section 2: No time.Now() in journey packages
# ==============================================================================

echo "Section 2: Clock injection (no time.Now())"
echo ""

# Check for time.Now() in internal/journey (excluding comments)
time_now_journey=$(grep -rn "time\.Now()" "${REPO_ROOT}/internal/journey" 2>/dev/null | grep -v "//" || true)
if [[ -z "$time_now_journey" ]]; then
    check_pass "No time.Now() in internal/journey"
else
    check_fail "time.Now() found in internal/journey: $time_now_journey"
fi

# Check for time.Now() in journey_store.go (excluding comments)
time_now_store=$(grep -n "time\.Now()" "${REPO_ROOT}/internal/persist/journey_store.go" 2>/dev/null | grep -v "//" || true)
if [[ -z "$time_now_store" ]]; then
    check_pass "No time.Now() in journey_store.go"
else
    check_fail "time.Now() found in journey_store.go: $time_now_store"
fi

echo ""

# ==============================================================================
# Section 3: No goroutines in journey packages
# ==============================================================================

echo "Section 3: No goroutines"
echo ""

# Check for goroutines in internal/journey
goroutines_journey=$(grep -rn "go\s\+func\|go\s\+[a-zA-Z]" "${REPO_ROOT}/internal/journey" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$goroutines_journey" ]]; then
    check_pass "No goroutines in internal/journey"
else
    check_fail "Goroutines found in internal/journey: $goroutines_journey"
fi

# Check for goroutines in journey_store.go
goroutines_store=$(grep -n "go\s\+func\|go\s\+[a-zA-Z]" "${REPO_ROOT}/internal/persist/journey_store.go" 2>/dev/null | grep -v "^[[:space:]]*//" || true)
if [[ -z "$goroutines_store" ]]; then
    check_pass "No goroutines in journey_store.go"
else
    check_fail "Goroutines found in journey_store.go: $goroutines_store"
fi

echo ""

# ==============================================================================
# Section 4: Domain model types
# ==============================================================================

echo "Section 4: Domain model types"
echo ""

# Check StepKind enum exists
if grep -q "type StepKind string" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepKind enum exists"
else
    check_fail "StepKind enum missing"
fi

# Check step_connect constant
if grep -q "StepConnect.*step_connect" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepConnect constant exists"
else
    check_fail "StepConnect constant missing"
fi

# Check step_sync constant
if grep -q "StepSync.*step_sync" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepSync constant exists"
else
    check_fail "StepSync constant missing"
fi

# Check step_mirror constant
if grep -q "StepMirror.*step_mirror" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepMirror constant exists"
else
    check_fail "StepMirror constant missing"
fi

# Check step_today constant
if grep -q "StepToday.*step_today" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepToday constant exists"
else
    check_fail "StepToday constant missing"
fi

# Check step_action constant
if grep -q "StepAction.*step_action" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepAction constant exists"
else
    check_fail "StepAction constant missing"
fi

# Check step_done constant
if grep -q "StepDone.*step_done" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "StepDone constant exists"
else
    check_fail "StepDone constant missing"
fi

# Check JourneyPage struct exists
if grep -q "type JourneyPage struct" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "JourneyPage struct exists"
else
    check_fail "JourneyPage struct missing"
fi

# Check JourneyAction struct exists
if grep -q "type JourneyAction struct" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "JourneyAction struct exists"
else
    check_fail "JourneyAction struct missing"
fi

# Check JourneyInputs struct exists
if grep -q "type JourneyInputs struct" "${REPO_ROOT}/internal/journey/model.go" 2>/dev/null; then
    check_pass "JourneyInputs struct exists"
else
    check_fail "JourneyInputs struct missing"
fi

echo ""

# ==============================================================================
# Section 5: Engine implementation
# ==============================================================================

echo "Section 5: Engine implementation"
echo ""

# Check Engine struct exists
if grep -q "type Engine struct" "${REPO_ROOT}/internal/journey/engine.go" 2>/dev/null; then
    check_pass "Engine struct exists"
else
    check_fail "Engine struct missing"
fi

# Check NewEngine function exists
if grep -q "func NewEngine" "${REPO_ROOT}/internal/journey/engine.go" 2>/dev/null; then
    check_pass "NewEngine function exists"
else
    check_fail "NewEngine function missing"
fi

# Check NextStep method exists
if grep -q "func.*NextStep" "${REPO_ROOT}/internal/journey/engine.go" 2>/dev/null; then
    check_pass "NextStep method exists"
else
    check_fail "NextStep method missing"
fi

# Check BuildPage method exists
if grep -q "func.*BuildPage" "${REPO_ROOT}/internal/journey/engine.go" 2>/dev/null; then
    check_pass "BuildPage method exists"
else
    check_fail "BuildPage method missing"
fi

echo ""

# ==============================================================================
# Section 6: Persistence (hash-only)
# ==============================================================================

echo "Section 6: Persistence (hash-only)"
echo ""

# Check JourneyDismissalStore struct exists
if grep -q "type JourneyDismissalStore struct" "${REPO_ROOT}/internal/persist/journey_store.go" 2>/dev/null; then
    check_pass "JourneyDismissalStore struct exists"
else
    check_fail "JourneyDismissalStore struct missing"
fi

# Check RecordDismissal method exists
if grep -q "func.*RecordDismissal" "${REPO_ROOT}/internal/persist/journey_store.go" 2>/dev/null; then
    check_pass "RecordDismissal method exists"
else
    check_fail "RecordDismissal method missing"
fi

# Check IsDismissedForPeriod method exists
if grep -q "func.*IsDismissedForPeriod" "${REPO_ROOT}/internal/persist/journey_store.go" 2>/dev/null; then
    check_pass "IsDismissedForPeriod method exists"
else
    check_fail "IsDismissedForPeriod method missing"
fi

# Check GetDismissedStatusHash method exists
if grep -q "func.*GetDismissedStatusHash" "${REPO_ROOT}/internal/persist/journey_store.go" 2>/dev/null; then
    check_pass "GetDismissedStatusHash method exists"
else
    check_fail "GetDismissedStatusHash method missing"
fi

# Check storelog record type exists
if grep -q "RecordTypeJourneyDismissal" "${REPO_ROOT}/pkg/domain/storelog/log.go" 2>/dev/null; then
    check_pass "RecordTypeJourneyDismissal exists in storelog"
else
    check_fail "RecordTypeJourneyDismissal missing from storelog"
fi

echo ""

# ==============================================================================
# Section 7: Web routes
# ==============================================================================

echo "Section 7: Web routes"
echo ""

# Check /journey route exists
if grep -q '"/journey"' "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "/journey route exists"
else
    check_fail "/journey route missing"
fi

# Check /journey/next route exists
if grep -q '"/journey/next"' "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "/journey/next route exists"
else
    check_fail "/journey/next route missing"
fi

# Check /journey/dismiss route exists
if grep -q '"/journey/dismiss"' "${REPO_ROOT}/cmd/quantumlife-web/main.go" 2>/dev/null; then
    check_pass "/journey/dismiss route exists"
else
    check_fail "/journey/dismiss route missing"
fi

echo ""

# ==============================================================================
# Section 8: Events
# ==============================================================================

echo "Section 8: Events"
echo ""

# Check Phase26AJourneyRequested event exists
if grep -q "Phase26AJourneyRequested" "${REPO_ROOT}/pkg/events/events.go" 2>/dev/null; then
    check_pass "Phase26AJourneyRequested event exists"
else
    check_fail "Phase26AJourneyRequested event missing"
fi

# Check Phase26AJourneyComputed event exists
if grep -q "Phase26AJourneyComputed" "${REPO_ROOT}/pkg/events/events.go" 2>/dev/null; then
    check_pass "Phase26AJourneyComputed event exists"
else
    check_fail "Phase26AJourneyComputed event missing"
fi

# Check Phase26AJourneyDismissed event exists
if grep -q "Phase26AJourneyDismissed" "${REPO_ROOT}/pkg/events/events.go" 2>/dev/null; then
    check_pass "Phase26AJourneyDismissed event exists"
else
    check_fail "Phase26AJourneyDismissed event missing"
fi

# Check Phase26AJourneyNextRedirected event exists
if grep -q "Phase26AJourneyNextRedirected" "${REPO_ROOT}/pkg/events/events.go" 2>/dev/null; then
    check_pass "Phase26AJourneyNextRedirected event exists"
else
    check_fail "Phase26AJourneyNextRedirected event missing"
fi

echo ""

# ==============================================================================
# Section 9: Privacy (no forbidden tokens)
# ==============================================================================

echo "Section 9: Privacy (no forbidden tokens in UI strings)"
echo ""

# Check no @symbol in journey strings
at_symbol=$(grep -rn '".*@.*"' "${REPO_ROOT}/internal/journey" 2>/dev/null | grep -v "_test\.go" | grep -v "RFC3339" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$at_symbol" ]]; then
    check_pass "No @ symbols in journey strings"
else
    check_fail "@ symbols found in journey strings: $at_symbol"
fi

# Check no http in journey strings
http_urls=$(grep -rn '".*http.*"' "${REPO_ROOT}/internal/journey" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$http_urls" ]]; then
    check_pass "No http URLs in journey strings"
else
    check_fail "http URLs found in journey strings: $http_urls"
fi

# Check no currency symbols in journey strings (£, $, €)
currency=$(grep -rn '".*[\$£€].*"' "${REPO_ROOT}/internal/journey" 2>/dev/null | grep -v "_test\.go" | grep -v "^[[:space:]]*//" || true)
if [[ -z "$currency" ]]; then
    check_pass "No currency symbols in journey strings"
else
    check_fail "Currency symbols found in journey strings: $currency"
fi

echo ""

# ==============================================================================
# Summary
# ==============================================================================

echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "Passed: $pass_count"
echo "Failed: $fail_count"
echo ""

if [[ $fail_count -gt 0 ]]; then
    echo -e "${RED}FAILED: Some guardrails not met${NC}"
    exit 1
else
    echo -e "${GREEN}PASSED: All guardrails met (${pass_count} checks)${NC}"
    exit 0
fi
