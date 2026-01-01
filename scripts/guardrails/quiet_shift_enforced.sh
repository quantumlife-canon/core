#!/bin/bash
# quiet_shift_enforced.sh - Guardrail checks for Phase 18.4: Quiet Shift
#
# Reference: docs/ADR/ADR-0036-phase18-4-quiet-shift.md
#
# This script validates:
# 1. No goroutines in internal/surface
# 2. No identifiers in surface templates
# 3. Store writes hash only
# 4. stdlib only imports
# 5. Routes exist
# 6. Events exist
# 7. CSS class exists

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.4: Quiet Shift - Guardrail Checks                      ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: /surface route exists in main.go
echo "Checking /surface route exists..."
if grep -q 'HandleFunc.*"/surface"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /surface route exists"
else
    echo -e "${RED}✗${NC} /surface route missing"
    FAILED=1
fi

# Check 2: internal/surface package exists
echo "Checking internal/surface package exists..."
if [ -d "$PROJECT_ROOT/internal/surface" ]; then
    echo -e "${GREEN}✓${NC} internal/surface directory exists"
else
    echo -e "${RED}✗${NC} internal/surface directory missing"
    FAILED=1
fi

# Check 3: Engine type exists
echo "Checking Engine type exists..."
if grep -q 'type Engine struct' "$PROJECT_ROOT/internal/surface/engine.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Engine type exists"
else
    echo -e "${RED}✗${NC} Engine type missing"
    FAILED=1
fi

# Check 4: Model uses SHA256
echo "Checking model uses SHA256..."
if grep -q 'crypto/sha256' "$PROJECT_ROOT/internal/surface/model.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Model uses SHA256"
else
    echo -e "${RED}✗${NC} Model does not use SHA256"
    FAILED=1
fi

# Check 5: No goroutines in internal/surface
echo "Checking no goroutines in internal/surface..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/surface/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in internal/surface"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in internal/surface"
fi

# Check 6: No time.Now() in internal/surface
echo "Checking no time.Now() in internal/surface..."
if grep -rn 'time\.Now()' "$PROJECT_ROOT/internal/surface/" 2>/dev/null | grep -v '_test.go' | grep -v 'default:.*time.Now'; then
    echo -e "${RED}✗${NC} time.Now() found in internal/surface"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in internal/surface (clock injection used)"
fi

# Check 7: No forbidden imports in internal/surface
echo "Checking no forbidden imports..."
FORBIDDEN_IMPORTS="net/http|database/sql|github.com"
if grep -rn "import.*($FORBIDDEN_IMPORTS)" "$PROJECT_ROOT/internal/surface/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Forbidden imports found in internal/surface"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No forbidden imports in internal/surface (stdlib only)"
fi

# Check 8: Phase 18.4 events exist
echo "Checking Phase 18.4 events exist..."
EVENTS_FILE="$PROJECT_ROOT/pkg/events/events.go"
if grep -q 'phase18_4.surface.cue.computed' "$EVENTS_FILE" && \
   grep -q 'phase18_4.surface.page.rendered' "$EVENTS_FILE" && \
   grep -q 'phase18_4.surface.action.viewed' "$EVENTS_FILE" && \
   grep -q 'phase18_4.surface.action.held' "$EVENTS_FILE" && \
   grep -q 'phase18_4.surface.action.why' "$EVENTS_FILE" && \
   grep -q 'phase18_4.surface.action.prefer_show_all' "$EVENTS_FILE"; then
    echo -e "${GREEN}✓${NC} Phase 18.4 events exist"
else
    echo -e "${RED}✗${NC} Phase 18.4 events missing"
    FAILED=1
fi

# Check 9: Surface template exists
echo "Checking surface template exists..."
if grep -q '{{define "surface"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Surface template exists"
else
    echo -e "${RED}✗${NC} Surface template missing"
    FAILED=1
fi

# Check 10: Surface-content template exists
echo "Checking surface-content template exists..."
if grep -q '{{define "surface-content"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Surface-content template exists"
else
    echo -e "${RED}✗${NC} Surface-content template missing"
    FAILED=1
fi

# Check 11: No identifiers in surface template
echo "Checking no identifiers in surface template..."
SURFACE_TEMPLATE=$(sed -n '/{{define "surface-content"}}/,/{{end}}/p' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" | head -100)
FORBIDDEN_IDENTIFIERS="@|\\\$|http://|https://|amazon|uber|netflix|spotify"
if echo "$SURFACE_TEMPLATE" | grep -iE "$FORBIDDEN_IDENTIFIERS" 2>/dev/null | grep -v 'SurfacePage'; then
    echo -e "${RED}✗${NC} Potential identifiers found in surface template"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No identifiers in surface template"
fi

# Check 12: Quiet-shift CSS class exists
echo "Checking quiet-shift CSS class exists..."
if grep -q '.quiet-shift' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for quiet-shift exists"
else
    echo -e "${RED}✗${NC} CSS styling for quiet-shift missing"
    FAILED=1
fi

# Check 13: Surface CSS class exists
echo "Checking surface CSS class exists..."
if grep -q '.surface ' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for surface page exists"
else
    echo -e "${RED}✗${NC} CSS styling for surface page missing"
    FAILED=1
fi

# Check 14: Demo tests exist
echo "Checking demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_4_quiet_shift/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Demo tests exist"
else
    echo -e "${RED}✗${NC} Demo tests missing"
    FAILED=1
fi

# Check 15: Store only records hash (no raw content fields)
echo "Checking store only stores hash..."
if grep -q 'RecordHash' "$PROJECT_ROOT/internal/surface/store.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Store uses RecordHash field"
else
    echo -e "${RED}✗${NC} Store may not use hash"
    FAILED=1
fi

# Check 16: Action store bounded
echo "Checking action store is bounded..."
if grep -q 'maxRecords' "$PROJECT_ROOT/internal/surface/store.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Action store has bounded growth"
else
    echo -e "${RED}✗${NC} Action store may grow unbounded"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.4 guardrail checks passed.${NC}"
    exit 0
else
    echo -e "${RED}Some Phase 18.4 guardrail checks failed.${NC}"
    exit 1
fi
