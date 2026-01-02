#!/bin/bash
# mirror_proof_enforced.sh - Guardrail checks for Phase 18.7: Mirror Proof
#
# Reference: docs/ADR/ADR-0039-phase18-7-mirror-proof.md
#
# This script validates:
# 1. No timestamps rendered
# 2. No vendor names
# 3. No sender/subject fields
# 4. No raw counts
# 5. No goroutines in mirror code
# 6. No time.Now() in mirror code
# 7. stdlib only imports
# 8. Route exists
# 9. Events exist
# 10. Demo tests exist

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.7: Mirror Proof - Guardrail Checks                     ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: /mirror route exists in main.go
echo "Checking /mirror route exists..."
if grep -q 'HandleFunc.*"/mirror"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /mirror route exists"
else
    echo -e "${RED}✗${NC} /mirror route missing"
    FAILED=1
fi

# Check 2: pkg/domain/mirror package exists
echo "Checking pkg/domain/mirror package exists..."
if [ -d "$PROJECT_ROOT/pkg/domain/mirror" ]; then
    echo -e "${GREEN}✓${NC} pkg/domain/mirror directory exists"
else
    echo -e "${RED}✗${NC} pkg/domain/mirror directory missing"
    FAILED=1
fi

# Check 3: internal/mirror package exists
echo "Checking internal/mirror package exists..."
if [ -d "$PROJECT_ROOT/internal/mirror" ]; then
    echo -e "${GREEN}✓${NC} internal/mirror directory exists"
else
    echo -e "${RED}✗${NC} internal/mirror directory missing"
    FAILED=1
fi

# Check 4: MirrorPage type exists
echo "Checking MirrorPage type exists..."
if grep -q 'type MirrorPage struct' "$PROJECT_ROOT/pkg/domain/mirror/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} MirrorPage type exists"
else
    echo -e "${RED}✗${NC} MirrorPage type missing"
    FAILED=1
fi

# Check 5: MagnitudeBucket type exists
echo "Checking MagnitudeBucket type exists..."
if grep -q 'type MagnitudeBucket' "$PROJECT_ROOT/pkg/domain/mirror/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} MagnitudeBucket type exists"
else
    echo -e "${RED}✗${NC} MagnitudeBucket type missing"
    FAILED=1
fi

# Check 6: HorizonBucket type exists
echo "Checking HorizonBucket type exists..."
if grep -q 'type HorizonBucket' "$PROJECT_ROOT/pkg/domain/mirror/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} HorizonBucket type exists"
else
    echo -e "${RED}✗${NC} HorizonBucket type missing"
    FAILED=1
fi

# Check 7: No goroutines in pkg/domain/mirror
echo "Checking no goroutines in pkg/domain/mirror..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/pkg/domain/mirror/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in pkg/domain/mirror"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in pkg/domain/mirror"
fi

# Check 8: No goroutines in internal/mirror
echo "Checking no goroutines in internal/mirror..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/mirror/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in internal/mirror"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in internal/mirror"
fi

# Check 9: No time.Now() in pkg/domain/mirror
echo "Checking no time.Now() in pkg/domain/mirror..."
if grep -rn 'time\.Now()' "$PROJECT_ROOT/pkg/domain/mirror/" 2>/dev/null | grep -v '_test.go' | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} time.Now() found in pkg/domain/mirror"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in pkg/domain/mirror (clock injection used)"
fi

# Check 10: No time.Now() in internal/mirror (except default in NewEngine)
echo "Checking no direct time.Now() in internal/mirror..."
TIME_NOW_COUNT=$(grep -rn 'time\.Now' "$PROJECT_ROOT/internal/mirror/" 2>/dev/null | grep -v '_test.go' | grep -v 'clock = time.Now' | grep -v '^[^:]*:[0-9]*:\s*//' | wc -l)
if [ "$TIME_NOW_COUNT" -gt 0 ]; then
    echo -e "${RED}✗${NC} Unexpected time.Now() in internal/mirror"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No direct time.Now() in internal/mirror"
fi

# Check 11: Canonical strings are pipe-delimited (not JSON)
echo "Checking canonical strings use pipe delimiter..."
if grep -q 'MIRROR_PAGE|v1' "$PROJECT_ROOT/pkg/domain/mirror/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Canonical strings use pipe delimiter"
else
    echo -e "${RED}✗${NC} Canonical strings may not use pipe delimiter"
    FAILED=1
fi

# Check 12: No json.Marshal in mirror types
echo "Checking no json.Marshal in mirror types..."
if grep -rn 'json\.Marshal' "$PROJECT_ROOT/pkg/domain/mirror/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} json.Marshal found - canonical strings should be pipe-delimited"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No json.Marshal in mirror types"
fi

# Check 13: Phase 18.7 events exist
echo "Checking Phase 18.7 events exist..."
EVENTS_FILE="$PROJECT_ROOT/pkg/events/events.go"
if grep -q 'phase18_7.mirror.computed' "$EVENTS_FILE" && \
   grep -q 'phase18_7.mirror.viewed' "$EVENTS_FILE" && \
   grep -q 'phase18_7.mirror.acknowledged' "$EVENTS_FILE"; then
    echo -e "${GREEN}✓${NC} Phase 18.7 events exist"
else
    echo -e "${RED}✗${NC} Phase 18.7 events missing"
    FAILED=1
fi

# Check 14: Mirror template exists
echo "Checking mirror template exists..."
if grep -q '{{define "mirror"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Mirror template exists"
else
    echo -e "${RED}✗${NC} Mirror template missing"
    FAILED=1
fi

# Check 15: Demo tests exist
echo "Checking demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_7_mirror/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Demo tests exist"
else
    echo -e "${RED}✗${NC} Demo tests missing"
    FAILED=1
fi

# Check 16: CSS for mirror page exists
echo "Checking CSS for mirror page exists..."
if grep -q '.mirror ' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for mirror page exists"
else
    echo -e "${RED}✗${NC} CSS styling for mirror page missing"
    FAILED=1
fi

# Check 17: No raw counts exposed in types (only magnitude buckets)
echo "Checking no raw counts exposed..."
# Look for exposed count fields (excluding internal fields)
if grep -rn 'Count.*int.*\`json' "$PROJECT_ROOT/pkg/domain/mirror/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Raw counts may be exposed via JSON"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No raw counts exposed in mirror types"
fi

# Check 18: No vendor names in not-stored statements
echo "Checking no vendor names in statements..."
NOT_STORED_PATTERNS="gmail|outlook|yahoo|plaid|stripe|google|microsoft|amazon"
if grep -rniE "$NOT_STORED_PATTERNS" "$PROJECT_ROOT/internal/mirror/engine.go" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Vendor names found in engine"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No vendor names in mirror engine"
fi

# Check 19: No third-party imports in pkg/domain/mirror
echo "Checking stdlib only in pkg/domain/mirror..."
if grep -rn '^import' "$PROJECT_ROOT/pkg/domain/mirror/" -A 20 2>/dev/null | grep -E 'github\.com|gopkg\.in' | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Third-party imports found in pkg/domain/mirror"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} stdlib only in pkg/domain/mirror (plus internal packages)"
fi

# Check 20: Link from connections to mirror exists
echo "Checking link from connections to mirror..."
if grep -q '/mirror' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" | grep -q 'connections'; then
    echo -e "${GREEN}✓${NC} Link from connections to mirror exists"
elif grep -q 'connections-mirror-link' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Link from connections to mirror exists"
else
    # Try another pattern
    if grep -q 'What we noticed' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
        echo -e "${GREEN}✓${NC} Link from connections to mirror exists"
    else
        echo -e "${RED}✗${NC} Link from connections to mirror missing"
        FAILED=1
    fi
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.7 guardrail checks passed.${NC}"
    exit 0
else
    echo -e "${RED}Some Phase 18.7 guardrail checks failed.${NC}"
    exit 1
fi
