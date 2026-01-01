#!/bin/bash
# today_quietly_enforced.sh - Guardrail checks for Phase 18.2: Today, quietly
#
# Reference: Phase 18.2 specification
#
# This script validates:
# 1. /today route exists
# 2. internal/todayquietly package exists with Engine
# 3. Store uses canonical strings + SHA256
# 4. Events include phase18_2.*
# 5. Templates include today template
# 6. No forbidden imports / no goroutines in internal/pkg

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.2: Today, quietly - Guardrail Checks                   ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: /today route exists in main.go
echo "Checking /today route exists..."
if grep -q 'HandleFunc.*"/today"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /today route exists"
else
    echo -e "${RED}✗${NC} /today route missing"
    FAILED=1
fi

# Check 2: /today/preference route exists
echo "Checking /today/preference route exists..."
if grep -q 'HandleFunc.*"/today/preference"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /today/preference route exists"
else
    echo -e "${RED}✗${NC} /today/preference route missing"
    FAILED=1
fi

# Check 3: internal/todayquietly package exists
echo "Checking internal/todayquietly package exists..."
if [ -d "$PROJECT_ROOT/internal/todayquietly" ]; then
    echo -e "${GREEN}✓${NC} internal/todayquietly directory exists"
else
    echo -e "${RED}✗${NC} internal/todayquietly directory missing"
    FAILED=1
fi

# Check 4: Engine type exists
echo "Checking Engine type exists..."
if grep -q 'type Engine struct' "$PROJECT_ROOT/internal/todayquietly/engine.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Engine type exists"
else
    echo -e "${RED}✗${NC} Engine type missing"
    FAILED=1
fi

# Check 5: Store uses SHA256
echo "Checking store uses SHA256..."
if grep -q 'crypto/sha256' "$PROJECT_ROOT/internal/todayquietly/store.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Store uses SHA256"
else
    echo -e "${RED}✗${NC} Store does not use SHA256"
    FAILED=1
fi

# Check 6: Store uses canonical strings (pipe-delimited)
echo "Checking store uses canonical strings..."
if grep -q 'fmt.Sprintf.*|' "$PROJECT_ROOT/internal/todayquietly/store.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Store uses canonical pipe-delimited strings"
else
    echo -e "${RED}✗${NC} Store does not use canonical strings"
    FAILED=1
fi

# Check 7: Phase 18.2 events exist
echo "Checking Phase 18.2 events exist..."
EVENTS_FILE="$PROJECT_ROOT/pkg/events/events.go"
if grep -q 'phase18_2.today.rendered' "$EVENTS_FILE" && \
   grep -q 'phase18_2.preference.recorded' "$EVENTS_FILE" && \
   grep -q 'phase18_2.suppression.demonstrated' "$EVENTS_FILE"; then
    echo -e "${GREEN}✓${NC} Phase 18.2 events exist"
else
    echo -e "${RED}✗${NC} Phase 18.2 events missing"
    FAILED=1
fi

# Check 8: Today template exists
echo "Checking today template exists..."
if grep -q '{{define "today"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Today template exists"
else
    echo -e "${RED}✗${NC} Today template missing"
    FAILED=1
fi

# Check 9: Today-content template exists
echo "Checking today-content template exists..."
if grep -q '{{define "today-content"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Today-content template exists"
else
    echo -e "${RED}✗${NC} Today-content template missing"
    FAILED=1
fi

# Check 10: No goroutines in internal/todayquietly
echo "Checking no goroutines in internal/todayquietly..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/todayquietly/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in internal/todayquietly"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in internal/todayquietly"
fi

# Check 11: No forbidden imports in internal/todayquietly
echo "Checking no forbidden imports..."
FORBIDDEN_IMPORTS="net/http|database/sql|github.com"
if grep -rn "import.*($FORBIDDEN_IMPORTS)" "$PROJECT_ROOT/internal/todayquietly/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Forbidden imports found in internal/todayquietly"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No forbidden imports in internal/todayquietly"
fi

# Check 12: Model has deterministic hash function
echo "Checking model has deterministic hash function..."
if grep -q 'func.*Hash()' "$PROJECT_ROOT/internal/todayquietly/model.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Model has hash function"
else
    echo -e "${RED}✗${NC} Model missing hash function"
    FAILED=1
fi

# Check 13: Demo tests exist
echo "Checking demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_2_today_quietly/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Demo tests exist"
else
    echo -e "${RED}✗${NC} Demo tests missing"
    FAILED=1
fi

# Check 14: CSS styling exists for today page
echo "Checking CSS styling exists..."
if grep -q '.today-quietly' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for today page exists"
else
    echo -e "${RED}✗${NC} CSS styling for today page missing"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.2 guardrail checks passed.${NC}"
    exit 0
else
    echo -e "${RED}Some Phase 18.2 guardrail checks failed.${NC}"
    exit 1
fi
