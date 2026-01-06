#!/bin/bash
# Phase 26C: Connected Reality Check Guardrails
#
# This is NOT analytics. This is a trust proof page.
# These guardrails verify that the reality check implementation:
# - Uses stdlib only
# - No time.Now() (clock injection only)
# - No goroutines in new packages
# - No forbidden tokens (secrets, identifiers)
# - Proper domain model and engine
# - Hash-only persistence
#
# Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

echo "=========================================="
echo "Phase 26C: Connected Reality Check Guardrails"
echo "=========================================="
echo "This is NOT analytics. This is a trust proof page."
echo ""

# =============================================================================
# Section 1: Package structure and stdlib-only
# =============================================================================

echo "Section 1: Package structure and stdlib-only"
echo ""

# Check pkg/domain/reality exists
if [ -d "pkg/domain/reality" ]; then
    pass "pkg/domain/reality package exists"
else
    fail "pkg/domain/reality package missing"
fi

# Check types.go exists
if [ -f "pkg/domain/reality/types.go" ]; then
    pass "types.go exists in domain"
else
    fail "types.go missing in domain"
fi

# Check internal/reality exists
if [ -d "internal/reality" ]; then
    pass "internal/reality package exists"
else
    fail "internal/reality package missing"
fi

# Check engine.go exists
if [ -f "internal/reality/engine.go" ]; then
    pass "engine.go exists"
else
    fail "engine.go missing"
fi

# Check reality_ack_store.go exists
if [ -f "internal/persist/reality_ack_store.go" ]; then
    pass "reality_ack_store.go exists"
else
    fail "reality_ack_store.go missing"
fi

# Check pkg/domain/reality uses stdlib only
if grep -rE '^import.*"github\.com|^import.*"golang\.org/x' pkg/domain/reality/ 2>/dev/null | grep -v '_test.go'; then
    fail "pkg/domain/reality uses non-stdlib imports"
else
    pass "pkg/domain/reality uses stdlib only"
fi

# Check internal/reality uses stdlib only
if grep -rE '^import.*"github\.com|^import.*"golang\.org/x' internal/reality/ 2>/dev/null | grep -v '_test.go'; then
    fail "internal/reality uses non-stdlib imports"
else
    pass "internal/reality uses stdlib only"
fi

echo ""

# =============================================================================
# Section 2: Clock injection (no time.Now())
# =============================================================================

echo "Section 2: Clock injection (no time.Now())"
echo ""

# Check no time.Now() in pkg/domain/reality (exclude comments)
if grep -r "time\.Now()" pkg/domain/reality/ 2>/dev/null | grep -v '_test.go' | grep -v '// '; then
    fail "time.Now() found in pkg/domain/reality"
else
    pass "No time.Now() in pkg/domain/reality"
fi

# Check no time.Now() in internal/reality (exclude comments)
if grep -r "time\.Now()" internal/reality/ 2>/dev/null | grep -v '_test.go' | grep -v '// '; then
    fail "time.Now() found in internal/reality"
else
    pass "No time.Now() in internal/reality"
fi

# Check no time.Now() in reality_ack_store.go (exclude comments)
if grep "time\.Now()" internal/persist/reality_ack_store.go 2>/dev/null | grep -v '// '; then
    fail "time.Now() found in reality_ack_store.go"
else
    pass "No time.Now() in reality_ack_store.go"
fi

echo ""

# =============================================================================
# Section 3: No goroutines
# =============================================================================

echo "Section 3: No goroutines"
echo ""

# Check no goroutines in pkg/domain/reality
if grep -rE '\bgo\s+\w+\(|go\s+func' pkg/domain/reality/ 2>/dev/null | grep -v '_test.go'; then
    fail "Goroutines found in pkg/domain/reality"
else
    pass "No goroutines in pkg/domain/reality"
fi

# Check no goroutines in internal/reality
if grep -rE '\bgo\s+\w+\(|go\s+func' internal/reality/ 2>/dev/null | grep -v '_test.go'; then
    fail "Goroutines found in internal/reality"
else
    pass "No goroutines in internal/reality"
fi

# Check no goroutines in reality_ack_store.go
if grep -E '\bgo\s+\w+\(|go\s+func' internal/persist/reality_ack_store.go 2>/dev/null; then
    fail "Goroutines found in reality_ack_store.go"
else
    pass "No goroutines in reality_ack_store.go"
fi

echo ""

# =============================================================================
# Section 4: Domain model types
# =============================================================================

echo "Section 4: Domain model types"
echo ""

# Check RealityLineKind type exists
if grep -q "type RealityLineKind" pkg/domain/reality/types.go; then
    pass "RealityLineKind type exists"
else
    fail "RealityLineKind type missing"
fi

# Check RealityLine struct exists
if grep -q "type RealityLine struct" pkg/domain/reality/types.go; then
    pass "RealityLine struct exists"
else
    fail "RealityLine struct missing"
fi

# Check RealityPage struct exists
if grep -q "type RealityPage struct" pkg/domain/reality/types.go; then
    pass "RealityPage struct exists"
else
    fail "RealityPage struct missing"
fi

# Check RealityInputs struct exists
if grep -q "type RealityInputs struct" pkg/domain/reality/types.go; then
    pass "RealityInputs struct exists"
else
    fail "RealityInputs struct missing"
fi

# Check RealityAck struct exists
if grep -q "type RealityAck struct" pkg/domain/reality/types.go; then
    pass "RealityAck struct exists"
else
    fail "RealityAck struct missing"
fi

# Check RealityCue struct exists
if grep -q "type RealityCue struct" pkg/domain/reality/types.go; then
    pass "RealityCue struct exists"
else
    fail "RealityCue struct missing"
fi

# Check SyncBucket type exists
if grep -q "type SyncBucket" pkg/domain/reality/types.go; then
    pass "SyncBucket type exists"
else
    fail "SyncBucket type missing"
fi

# Check MagnitudeBucket type exists
if grep -q "type MagnitudeBucket" pkg/domain/reality/types.go; then
    pass "MagnitudeBucket type exists"
else
    fail "MagnitudeBucket type missing"
fi

# Check ShadowProviderKind type exists
if grep -q "type ShadowProviderKind" pkg/domain/reality/types.go; then
    pass "ShadowProviderKind type exists"
else
    fail "ShadowProviderKind type missing"
fi

# Check CanonicalString methods exist
if grep -q "CanonicalString()" pkg/domain/reality/types.go; then
    pass "CanonicalString methods exist"
else
    fail "CanonicalString methods missing"
fi

echo ""

# =============================================================================
# Section 5: Engine implementation
# =============================================================================

echo "Section 5: Engine implementation"
echo ""

# Check Engine struct exists
if grep -q "type Engine struct" internal/reality/engine.go; then
    pass "Engine struct exists"
else
    fail "Engine struct missing"
fi

# Check Clock interface exists
if grep -q "type Clock interface" internal/reality/engine.go; then
    pass "Clock interface exists"
else
    fail "Clock interface missing"
fi

# Check NewEngine function exists
if grep -q "func NewEngine" internal/reality/engine.go; then
    pass "NewEngine function exists"
else
    fail "NewEngine function missing"
fi

# Check BuildPage method exists
if grep -q "func.*BuildPage" internal/reality/engine.go; then
    pass "BuildPage method exists"
else
    fail "BuildPage method missing"
fi

# Check ComputeCue method exists
if grep -q "func.*ComputeCue" internal/reality/engine.go; then
    pass "ComputeCue method exists"
else
    fail "ComputeCue method missing"
fi

# Check ShouldShowRealityCue method exists
if grep -q "func.*ShouldShowRealityCue" internal/reality/engine.go; then
    pass "ShouldShowRealityCue method exists"
else
    fail "ShouldShowRealityCue method missing"
fi

echo ""

# =============================================================================
# Section 6: Persistence constraints
# =============================================================================

echo "Section 6: Persistence constraints"
echo ""

# Check RealityAckStore struct exists
if grep -q "type RealityAckStore struct" internal/persist/reality_ack_store.go; then
    pass "RealityAckStore struct exists"
else
    fail "RealityAckStore struct missing"
fi

# Check bounded retention (maxPeriods) exists
if grep -q "maxPeriods" internal/persist/reality_ack_store.go; then
    pass "Bounded retention (maxPeriods) exists"
else
    fail "Bounded retention (maxPeriods) missing"
fi

# Check storelog integration exists
if grep -q "storelog" internal/persist/reality_ack_store.go; then
    pass "Storelog integration exists"
else
    fail "Storelog integration missing"
fi

# Check Reality Ack record type in storelog
if grep -q "RecordTypeRealityAck" pkg/domain/storelog/log.go; then
    pass "RecordTypeRealityAck exists in storelog"
else
    fail "RecordTypeRealityAck missing in storelog"
fi

echo ""

# =============================================================================
# Section 7: Events defined
# =============================================================================

echo "Section 7: Events defined"
echo ""

# Check phase26c.reality.requested event
if grep -q '"phase26c.reality.requested"' pkg/events/events.go; then
    pass "Event 'phase26c.reality.requested' defined"
else
    fail "Event 'phase26c.reality.requested' missing"
fi

# Check phase26c.reality.computed event
if grep -q '"phase26c.reality.computed"' pkg/events/events.go; then
    pass "Event 'phase26c.reality.computed' defined"
else
    fail "Event 'phase26c.reality.computed' missing"
fi

# Check phase26c.reality.viewed event
if grep -q '"phase26c.reality.viewed"' pkg/events/events.go; then
    pass "Event 'phase26c.reality.viewed' defined"
else
    fail "Event 'phase26c.reality.viewed' missing"
fi

# Check phase26c.reality.ack.recorded event
if grep -q '"phase26c.reality.ack.recorded"' pkg/events/events.go; then
    pass "Event 'phase26c.reality.ack.recorded' defined"
else
    fail "Event 'phase26c.reality.ack.recorded' missing"
fi

echo ""

# =============================================================================
# Section 8: Web routes exist
# =============================================================================

echo "Section 8: Web routes exist"
echo ""

# Check /reality route exists
if grep -q '"/reality"' cmd/quantumlife-web/main.go; then
    pass "/reality route exists"
else
    fail "/reality route missing"
fi

# Check /reality/ack route exists
if grep -q '"/reality/ack"' cmd/quantumlife-web/main.go; then
    pass "/reality/ack route exists"
else
    fail "/reality/ack route missing"
fi

# Check handleReality handler exists
if grep -q "handleReality" cmd/quantumlife-web/main.go; then
    pass "handleReality handler exists"
else
    fail "handleReality handler missing"
fi

# Check handleRealityAck handler exists
if grep -q "handleRealityAck" cmd/quantumlife-web/main.go; then
    pass "handleRealityAck handler exists"
else
    fail "handleRealityAck handler missing"
fi

echo ""

# =============================================================================
# Section 9: Privacy - no forbidden tokens
# =============================================================================

echo "Section 9: Privacy - no forbidden tokens"
echo ""

# Check no @ symbols in reality strings (excluding comments and test files)
if grep -rE '"[^"]*@[^"]*"' pkg/domain/reality/ internal/reality/ 2>/dev/null | grep -v '_test.go' | grep -v '// '; then
    fail "@ symbols found in reality strings"
else
    pass "No @ symbols in reality strings"
fi

# Check no http URLs in reality strings
if grep -rE '"https?://' pkg/domain/reality/ internal/reality/ 2>/dev/null | grep -v '_test.go'; then
    fail "http URLs found in reality strings"
else
    pass "No http URLs in reality strings"
fi

# Check no Bearer/Authorization in reality code
if grep -rE 'Bearer|Authorization|api-key|client_secret|refresh_token' pkg/domain/reality/ internal/reality/ internal/persist/reality_ack_store.go 2>/dev/null | grep -v '_test.go' | grep -v '// '; then
    fail "Secret tokens found in reality code"
else
    pass "No secret tokens in reality code"
fi

echo ""

# =============================================================================
# Section 10: Deterministic hashing
# =============================================================================

echo "Section 10: Deterministic hashing"
echo ""

# Check ComputeStatusHash exists
if grep -q "ComputeStatusHash" pkg/domain/reality/types.go; then
    pass "ComputeStatusHash exists"
else
    fail "ComputeStatusHash missing"
fi

# Check Hash method exists on RealityInputs
if grep -q "func.*RealityInputs.*Hash" pkg/domain/reality/types.go; then
    pass "Hash method exists on RealityInputs"
else
    fail "Hash method missing on RealityInputs"
fi

# Check pipe-delimited format (|) in canonical strings
if grep -q '|v1|' pkg/domain/reality/types.go; then
    pass "Pipe-delimited format in canonical strings"
else
    fail "Pipe-delimited format missing in canonical strings"
fi

echo ""

# =============================================================================
# Summary
# =============================================================================

echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""

echo "Passed: ${GREEN}${PASS_COUNT}${NC}"
echo "Failed: ${RED}${FAIL_COUNT}${NC}"

if [ $FAIL_COUNT -gt 0 ]; then
    echo ""
    echo -e "${RED}Phase 26C guardrails FAILED${NC}"
    exit 1
else
    echo ""
    echo -e "${GREEN}Phase 26C guardrails PASSED${NC}"
    echo ""
    echo "This is NOT analytics. This is a trust proof page."
fi
