#!/bin/bash
# held_projection_enforced.sh - Guardrail checks for Phase 18.3: Held, not shown
#
# Reference: docs/ADR/ADR-0035-phase18-3-proof-of-care.md
#
# This script validates:
# 1. No raw events referenced in internal/held
# 2. No identifiers rendered in held template
# 3. No goroutines
# 4. No time.Now()
# 5. stdlib only
# 6. No counts tied to specific items

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.3: Held, not shown - Guardrail Checks                  ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: /held route exists in main.go
echo "Checking /held route exists..."
if grep -q 'HandleFunc.*"/held"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /held route exists"
else
    echo -e "${RED}✗${NC} /held route missing"
    FAILED=1
fi

# Check 2: internal/held package exists
echo "Checking internal/held package exists..."
if [ -d "$PROJECT_ROOT/internal/held" ]; then
    echo -e "${GREEN}✓${NC} internal/held directory exists"
else
    echo -e "${RED}✗${NC} internal/held directory missing"
    FAILED=1
fi

# Check 3: Engine type exists
echo "Checking Engine type exists..."
if grep -q 'type Engine struct' "$PROJECT_ROOT/internal/held/engine.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Engine type exists"
else
    echo -e "${RED}✗${NC} Engine type missing"
    FAILED=1
fi

# Check 4: Store uses SHA256
echo "Checking model uses SHA256..."
if grep -q 'crypto/sha256' "$PROJECT_ROOT/internal/held/model.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Model uses SHA256"
else
    echo -e "${RED}✗${NC} Model does not use SHA256"
    FAILED=1
fi

# Check 5: No raw event imports in internal/held
echo "Checking no raw event imports in internal/held..."
FORBIDDEN_EVENT_IMPORTS="domainevents|pkg/domain/events|NewEmailMessageEvent|NewCalendarEventEvent"
if grep -rn "$FORBIDDEN_EVENT_IMPORTS" "$PROJECT_ROOT/internal/held/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Raw event imports found in internal/held"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No raw event imports in internal/held"
fi

# Check 6: No goroutines in internal/held
echo "Checking no goroutines in internal/held..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/held/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in internal/held"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in internal/held"
fi

# Check 7: No time.Now() in internal/held
echo "Checking no time.Now() in internal/held..."
if grep -rn 'time\.Now()' "$PROJECT_ROOT/internal/held/" 2>/dev/null | grep -v '_test.go' | grep -v 'default:.*time.Now'; then
    echo -e "${RED}✗${NC} time.Now() found in internal/held"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in internal/held (clock injection used)"
fi

# Check 8: No forbidden imports in internal/held
echo "Checking no forbidden imports..."
FORBIDDEN_IMPORTS="net/http|database/sql|github.com"
if grep -rn "import.*($FORBIDDEN_IMPORTS)" "$PROJECT_ROOT/internal/held/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Forbidden imports found in internal/held"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No forbidden imports in internal/held (stdlib only)"
fi

# Check 9: Phase 18.3 events exist
echo "Checking Phase 18.3 events exist..."
EVENTS_FILE="$PROJECT_ROOT/pkg/events/events.go"
if grep -q 'phase18_3.held.computed' "$EVENTS_FILE" && \
   grep -q 'phase18_3.held.presented' "$EVENTS_FILE"; then
    echo -e "${GREEN}✓${NC} Phase 18.3 events exist"
else
    echo -e "${RED}✗${NC} Phase 18.3 events missing"
    FAILED=1
fi

# Check 10: Held template exists
echo "Checking held template exists..."
if grep -q '{{define "held"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Held template exists"
else
    echo -e "${RED}✗${NC} Held template missing"
    FAILED=1
fi

# Check 11: Held-content template exists
echo "Checking held-content template exists..."
if grep -q '{{define "held-content"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Held-content template exists"
else
    echo -e "${RED}✗${NC} Held-content template missing"
    FAILED=1
fi

# Check 12: No identifiers in held template
echo "Checking no identifiers in held template..."
# Extract the held-content template and check for forbidden patterns
HELD_TEMPLATE=$(sed -n '/{{define "held-content"}}/,/{{end}}/p' "$PROJECT_ROOT/cmd/quantumlife-web/main.go")
FORBIDDEN_IDENTIFIERS="@|\\$|http://|https://|meeting|bill|appointment|invoice"
if echo "$HELD_TEMPLATE" | grep -E "$FORBIDDEN_IDENTIFIERS" 2>/dev/null | grep -v 'HeldSummary'; then
    echo -e "${RED}✗${NC} Potential identifiers found in held template"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No identifiers in held template"
fi

# Check 13: No action buttons in held template
echo "Checking no action buttons in held template..."
if echo "$HELD_TEMPLATE" | grep -iE 'button.*review|button.*manage|button.*see|button.*expand|type="submit"' 2>/dev/null; then
    echo -e "${RED}✗${NC} Action buttons found in held template"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No action buttons in held template"
fi

# Check 14: Demo tests exist
echo "Checking demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_3_held/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Demo tests exist"
else
    echo -e "${RED}✗${NC} Demo tests missing"
    FAILED=1
fi

# Check 15: CSS styling exists for held page
echo "Checking CSS styling exists..."
if grep -q '.held' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for held page exists"
else
    echo -e "${RED}✗${NC} CSS styling for held page missing"
    FAILED=1
fi

# Check 16: HeldSummary only stores hash, not raw data
echo "Checking store only stores hash..."
if grep -q 'func.*Record.*HeldSummary' "$PROJECT_ROOT/internal/held/store.go" 2>/dev/null; then
    # Verify the Record function only uses summary.Hash
    RECORD_FUNC=$(sed -n '/func.*Record.*HeldSummary/,/^func\|^}/p' "$PROJECT_ROOT/internal/held/store.go")
    if echo "$RECORD_FUNC" | grep -q 'summary.Hash'; then
        echo -e "${GREEN}✓${NC} Store records hash only"
    else
        echo -e "${RED}✗${NC} Store may record more than hash"
        FAILED=1
    fi
else
    echo -e "${RED}✗${NC} Record function not found"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.3 guardrail checks passed.${NC}"
    exit 0
else
    echo -e "${RED}Some Phase 18.3 guardrail checks failed.${NC}"
    exit 1
fi
