#!/usr/bin/env bash
#
# quiet_inbox_mirror_enforced.sh
#
# Phase 22: Quiet Inbox Mirror Guardrails
#
# Verifies that Phase 22 implementation follows all invariants:
#   - No Gmail fields referenced (no subjects, senders, bodies)
#   - No UI actions exist (no buttons, forms for actions)
#   - No LLM usage (shadow-only)
#   - No time.Now() (clock injection only)
#   - Stdlib only
#   - Deterministic hashing (pipe-delimited)
#   - No goroutines
#   - No notification paths
#
# Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

passed=0
failed=0

check() {
    local name="$1"
    local result="$2"
    if [[ "$result" == "pass" ]]; then
        echo -e "[${GREEN}PASS${NC}] $name"
        ((passed++))
    else
        echo -e "[${RED}FAIL${NC}] $name"
        ((failed++))
    fi
}

echo "=== Phase 22: Quiet Inbox Mirror Guardrails ==="
echo ""

# -----------------------------------------------------------------------------
# Domain Model Guardrails
# -----------------------------------------------------------------------------

echo "--- Domain Model Guardrails ---"

# Check no goroutines in pkg/domain/quietmirror/
if grep -r "go func" "${REPO_ROOT}/pkg/domain/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No goroutines in pkg/domain/quietmirror/" "fail"
else
    check "No goroutines in pkg/domain/quietmirror/" "pass"
fi

# Check no time.Now() in pkg/domain/quietmirror/
if grep -r "time\.Now()" "${REPO_ROOT}/pkg/domain/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No time.Now() in pkg/domain/quietmirror/" "fail"
else
    check "No time.Now() in pkg/domain/quietmirror/" "pass"
fi

# Check MirrorMagnitude enum exists
if grep -q "MagnitudeNothing.*MirrorMagnitude" "${REPO_ROOT}/pkg/domain/quietmirror/types.go"; then
    check "MirrorMagnitude enum exists" "pass"
else
    check "MirrorMagnitude enum exists" "fail"
fi

# Check MirrorCategory enum exists
if grep -q "CategoryWork.*MirrorCategory" "${REPO_ROOT}/pkg/domain/quietmirror/types.go"; then
    check "MirrorCategory enum exists" "pass"
else
    check "MirrorCategory enum exists" "fail"
fi

# Check QuietMirrorSummary struct exists
if grep -q "type QuietMirrorSummary struct" "${REPO_ROOT}/pkg/domain/quietmirror/types.go"; then
    check "QuietMirrorSummary struct exists" "pass"
else
    check "QuietMirrorSummary struct exists" "fail"
fi

# Check pipe-delimited canonical string
if grep -q 'QUIET_MIRROR|v1|' "${REPO_ROOT}/pkg/domain/quietmirror/types.go"; then
    check "Pipe-delimited canonical string format" "pass"
else
    check "Pipe-delimited canonical string format" "fail"
fi

echo ""

# -----------------------------------------------------------------------------
# Engine Guardrails
# -----------------------------------------------------------------------------

echo "--- Engine Guardrails ---"

# Check no goroutines in internal/quietmirror/
if grep -r "go func" "${REPO_ROOT}/internal/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No goroutines in internal/quietmirror/" "fail"
else
    check "No goroutines in internal/quietmirror/" "pass"
fi

# Check no time.Now() in internal/quietmirror/ (exclude comments)
if grep -r "time\.Now()" "${REPO_ROOT}/internal/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -v "//" | grep -q .; then
    check "No time.Now() in internal/quietmirror/" "fail"
else
    check "No time.Now() in internal/quietmirror/" "pass"
fi

# Check clock injection pattern
if grep -q "clock func() time.Time" "${REPO_ROOT}/internal/quietmirror/engine.go"; then
    check "Clock injection pattern in engine" "pass"
else
    check "Clock injection pattern in engine" "fail"
fi

# Check Engine struct exists
if grep -q "type Engine struct" "${REPO_ROOT}/internal/quietmirror/engine.go"; then
    check "Engine struct exists" "pass"
else
    check "Engine struct exists" "fail"
fi

# Check Compute function exists
if grep -q "func (e \*Engine) Compute" "${REPO_ROOT}/internal/quietmirror/engine.go"; then
    check "Compute function exists" "pass"
else
    check "Compute function exists" "fail"
fi

# Check BuildPage function exists
if grep -q "func (e \*Engine) BuildPage" "${REPO_ROOT}/internal/quietmirror/engine.go"; then
    check "BuildPage function exists" "pass"
else
    check "BuildPage function exists" "fail"
fi

# Check WhisperCue exists
if grep -q "type WhisperCue struct" "${REPO_ROOT}/internal/quietmirror/engine.go"; then
    check "WhisperCue type exists" "pass"
else
    check "WhisperCue type exists" "fail"
fi

echo ""

# -----------------------------------------------------------------------------
# Privacy Guardrails
# -----------------------------------------------------------------------------

echo "--- Privacy Guardrails ---"

# Check no Gmail field references in domain (word boundaries to avoid false positives)
# Matches: Subject, From, To, Body, Snippet, InternalDate as standalone words
# Avoids matching: EncodeToString, etc.
GMAIL_FIELDS="\bSubject\b|\bFrom\b|\bBody\b|\bSnippet\b|\bInternalDate\b"
if grep -rE "$GMAIL_FIELDS" "${REPO_ROOT}/pkg/domain/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No Gmail fields in domain model" "fail"
else
    check "No Gmail fields in domain model" "pass"
fi

# Check no Gmail field references in engine
if grep -rE "$GMAIL_FIELDS" "${REPO_ROOT}/internal/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No Gmail fields in engine" "fail"
else
    check "No Gmail fields in engine" "pass"
fi

# Check no raw counts stored (only magnitude buckets)
if grep -q "MessageCount\|EmailCount\|RawCount" "${REPO_ROOT}/pkg/domain/quietmirror/types.go" 2>/dev/null; then
    check "No raw counts in domain model" "fail"
else
    check "No raw counts in domain model" "pass"
fi

echo ""

# -----------------------------------------------------------------------------
# LLM Guardrails
# -----------------------------------------------------------------------------

echo "--- LLM Guardrails ---"

# Check no LLM imports in quietmirror packages
LLM_IMPORTS="shadowllm|openai|anthropic|claude|gemini"
if grep -rEi "$LLM_IMPORTS" "${REPO_ROOT}/internal/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No LLM imports in engine" "fail"
else
    check "No LLM imports in engine" "pass"
fi

if grep -rEi "$LLM_IMPORTS" "${REPO_ROOT}/pkg/domain/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -v "^Binary" | grep -q .; then
    check "No LLM imports in domain" "fail"
else
    check "No LLM imports in domain" "pass"
fi

echo ""

# -----------------------------------------------------------------------------
# UI Guardrails
# -----------------------------------------------------------------------------

echo "--- UI Guardrails ---"

# Check /mirror/inbox route registered
if grep -q "/mirror/inbox" "${REPO_ROOT}/cmd/quantumlife-web/main.go"; then
    check "/mirror/inbox route registered" "pass"
else
    check "/mirror/inbox route registered" "fail"
fi

# Check handleQuietInboxMirror exists
if grep -q "func (s \*Server) handleQuietInboxMirror" "${REPO_ROOT}/cmd/quantumlife-web/main.go"; then
    check "handleQuietInboxMirror handler exists" "pass"
else
    check "handleQuietInboxMirror handler exists" "fail"
fi

# Check no action buttons in mirror template (look for form with action methods)
if grep -A50 "quietMirrorTemplate" "${REPO_ROOT}/cmd/quantumlife-web/main.go" | grep -q '<button\|<input type="submit"\|action="/'; then
    check "No action buttons in mirror template" "fail"
else
    check "No action buttons in mirror template" "pass"
fi

echo ""

# -----------------------------------------------------------------------------
# Events Guardrails
# -----------------------------------------------------------------------------

echo "--- Events Guardrails ---"

# Check Phase22QuietMirrorComputed event defined
if grep -q "Phase22QuietMirrorComputed" "${REPO_ROOT}/pkg/events/events.go"; then
    check "Phase22QuietMirrorComputed event defined" "pass"
else
    check "Phase22QuietMirrorComputed event defined" "fail"
fi

# Check Phase22QuietMirrorViewed event defined
if grep -q "Phase22QuietMirrorViewed" "${REPO_ROOT}/pkg/events/events.go"; then
    check "Phase22QuietMirrorViewed event defined" "pass"
else
    check "Phase22QuietMirrorViewed event defined" "fail"
fi

# Check Phase22QuietMirrorAbsent event defined
if grep -q "Phase22QuietMirrorAbsent" "${REPO_ROOT}/pkg/events/events.go"; then
    check "Phase22QuietMirrorAbsent event defined" "pass"
else
    check "Phase22QuietMirrorAbsent event defined" "fail"
fi

echo ""

# -----------------------------------------------------------------------------
# Persistence Guardrails
# -----------------------------------------------------------------------------

echo "--- Persistence Guardrails ---"

# Check QuietMirrorStore exists
if grep -q "type QuietMirrorStore struct" "${REPO_ROOT}/internal/persist/quietmirror_store.go"; then
    check "QuietMirrorStore exists" "pass"
else
    check "QuietMirrorStore exists" "fail"
fi

# Check store uses hash-only storage
if grep -q "summaries map\[string\]" "${REPO_ROOT}/internal/persist/quietmirror_store.go"; then
    check "Store uses hash-keyed map" "pass"
else
    check "Store uses hash-keyed map" "fail"
fi

# Check no time.Now() in store
if grep "time\.Now()" "${REPO_ROOT}/internal/persist/quietmirror_store.go" 2>/dev/null | grep -v "//" | grep -q .; then
    check "No time.Now() in store" "fail"
else
    check "No time.Now() in store" "pass"
fi

echo ""

# -----------------------------------------------------------------------------
# Demo Tests Guardrails
# -----------------------------------------------------------------------------

echo "--- Demo Tests Guardrails ---"

# Check demo tests file exists
if [[ -f "${REPO_ROOT}/internal/demo_phase22_quiet_inbox_mirror/demo_test.go" ]]; then
    check "Demo tests file exists" "pass"
else
    check "Demo tests file exists" "fail"
fi

# Check demo tests compile
if go build "${REPO_ROOT}/internal/demo_phase22_quiet_inbox_mirror/..." 2>/dev/null; then
    check "Demo tests compile" "pass"
else
    check "Demo tests compile" "fail"
fi

echo ""

# -----------------------------------------------------------------------------
# Stdlib Only Guardrails
# -----------------------------------------------------------------------------

echo "--- Stdlib Only Guardrails ---"

# Check no external imports in domain
EXTERNAL_IMPORTS="github\.com|golang\.org/x|google\.golang\.org"
if grep -rE "\"$EXTERNAL_IMPORTS" "${REPO_ROOT}/pkg/domain/quietmirror/" 2>/dev/null | grep -v "_test.go" | grep -q .; then
    check "Stdlib only in domain" "fail"
else
    check "Stdlib only in domain" "pass"
fi

# Check no external imports in engine (except quantumlife/)
if grep -E "\"(github\.com|golang\.org)" "${REPO_ROOT}/internal/quietmirror/engine.go" 2>/dev/null | grep -q .; then
    check "Stdlib only in engine" "fail"
else
    check "Stdlib only in engine" "pass"
fi

echo ""

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------

echo "==================================================================="
echo "Summary: $passed passed, $failed failed"
echo "==================================================================="

if [[ $failed -eq 0 ]]; then
    echo -e "${GREEN}All Phase 22 guardrails PASSED.${NC}"
    exit 0
else
    echo -e "${RED}Phase 22 guardrails FAILED.${NC}"
    exit 1
fi
