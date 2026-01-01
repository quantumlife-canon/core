#!/bin/bash
# proof_enforced.sh - Guardrail checks for Phase 18.5: Quiet Proof
#
# Reference: docs/ADR/ADR-0037-phase18-5-quiet-proof.md
#
# This script validates:
# 1. No goroutines in internal/proof
# 2. No identifiers in proof templates
# 3. Store writes hash only
# 4. stdlib only imports
# 5. Routes exist
# 6. Events exist
# 7. CSS class exists
# 8. Magnitude buckets only (no raw counts)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.5: Quiet Proof - Guardrail Checks                      ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: /proof route exists in main.go
echo "Checking /proof route exists..."
if grep -q 'HandleFunc.*"/proof"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /proof route exists"
else
    echo -e "${RED}✗${NC} /proof route missing"
    FAILED=1
fi

# Check 2: /proof/dismiss route exists
echo "Checking /proof/dismiss route exists..."
if grep -q 'HandleFunc.*"/proof/dismiss"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /proof/dismiss route exists"
else
    echo -e "${RED}✗${NC} /proof/dismiss route missing"
    FAILED=1
fi

# Check 3: internal/proof package exists
echo "Checking internal/proof package exists..."
if [ -d "$PROJECT_ROOT/internal/proof" ]; then
    echo -e "${GREEN}✓${NC} internal/proof directory exists"
else
    echo -e "${RED}✗${NC} internal/proof directory missing"
    FAILED=1
fi

# Check 4: Engine type exists
echo "Checking Engine type exists..."
if grep -q 'type Engine struct' "$PROJECT_ROOT/internal/proof/engine.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Engine type exists"
else
    echo -e "${RED}✗${NC} Engine type missing"
    FAILED=1
fi

# Check 5: Model uses SHA256
echo "Checking model uses SHA256..."
if grep -q 'crypto/sha256' "$PROJECT_ROOT/internal/proof/model.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Model uses SHA256"
else
    echo -e "${RED}✗${NC} Model does not use SHA256"
    FAILED=1
fi

# Check 6: No goroutines in internal/proof
echo "Checking no goroutines in internal/proof..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/proof/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in internal/proof"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in internal/proof"
fi

# Check 7: No time.Now() in internal/proof
echo "Checking no time.Now() in internal/proof..."
# Exclude test files and comments (lines starting with // after stripping)
if grep -rn 'time\.Now()' "$PROJECT_ROOT/internal/proof/" 2>/dev/null | grep -v '_test.go' | grep -v '^\s*//' | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} time.Now() found in internal/proof"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in internal/proof (clock injection used)"
fi

# Check 8: No forbidden imports in internal/proof
echo "Checking no forbidden imports..."
FORBIDDEN_IMPORTS="net/http|database/sql|github.com"
if grep -rn "import.*($FORBIDDEN_IMPORTS)" "$PROJECT_ROOT/internal/proof/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} Forbidden imports found in internal/proof"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No forbidden imports in internal/proof (stdlib only)"
fi

# Check 9: Phase 18.5 events exist
echo "Checking Phase 18.5 events exist..."
EVENTS_FILE="$PROJECT_ROOT/pkg/events/events.go"
if grep -q 'phase18_5.proof.computed' "$EVENTS_FILE" && \
   grep -q 'phase18_5.proof.viewed' "$EVENTS_FILE" && \
   grep -q 'phase18_5.proof.dismissed' "$EVENTS_FILE"; then
    echo -e "${GREEN}✓${NC} Phase 18.5 events exist"
else
    echo -e "${RED}✗${NC} Phase 18.5 events missing"
    FAILED=1
fi

# Check 10: Proof template exists
echo "Checking proof template exists..."
if grep -q '{{define "proof"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Proof template exists"
else
    echo -e "${RED}✗${NC} Proof template missing"
    FAILED=1
fi

# Check 11: Proof-content template exists
echo "Checking proof-content template exists..."
if grep -q '{{define "proof-content"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Proof-content template exists"
else
    echo -e "${RED}✗${NC} Proof-content template missing"
    FAILED=1
fi

# Check 12: No identifiers in proof template
echo "Checking no identifiers in proof template..."
PROOF_TEMPLATE=$(sed -n '/{{define "proof-content"}}/,/{{end}}/p' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" | head -100)
FORBIDDEN_IDENTIFIERS="@|\\$|£|€|http://|https://|amazon|uber|netflix|spotify"
if echo "$PROOF_TEMPLATE" | grep -iE "$FORBIDDEN_IDENTIFIERS" 2>/dev/null | grep -v 'ProofSummary'; then
    echo -e "${RED}✗${NC} Potential identifiers found in proof template"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No identifiers in proof template"
fi

# Check 13: Proof CSS class exists
echo "Checking proof CSS class exists..."
if grep -q '.proof ' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for proof page exists"
else
    echo -e "${RED}✗${NC} CSS styling for proof page missing"
    FAILED=1
fi

# Check 14: Demo tests exist
echo "Checking demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_5_proof/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Demo tests exist"
else
    echo -e "${RED}✗${NC} Demo tests missing"
    FAILED=1
fi

# Check 15: Magnitude enum exists (no raw counts)
echo "Checking magnitude enum exists..."
if grep -q 'MagnitudeNothing' "$PROJECT_ROOT/internal/proof/model.go" 2>/dev/null && \
   grep -q 'MagnitudeAFew' "$PROJECT_ROOT/internal/proof/model.go" 2>/dev/null && \
   grep -q 'MagnitudeSeveral' "$PROJECT_ROOT/internal/proof/model.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Magnitude buckets exist (no raw counts)"
else
    echo -e "${RED}✗${NC} Magnitude buckets missing"
    FAILED=1
fi

# Check 16: Store uses RecordHash
echo "Checking store uses hash..."
if grep -q 'RecordHash\|ComputeRecordHash' "$PROJECT_ROOT/internal/proof/store.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Store uses record hash"
else
    echo -e "${RED}✗${NC} Store may not use hash"
    FAILED=1
fi

# Check 17: Ack store bounded
echo "Checking ack store is bounded..."
if grep -q 'maxRecords' "$PROJECT_ROOT/internal/proof/store.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Ack store has bounded growth"
else
    echo -e "${RED}✗${NC} Ack store may grow unbounded"
    FAILED=1
fi

# Check 18: Proof cue CSS exists
echo "Checking proof cue CSS exists..."
if grep -q '.quiet-proof-cue' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for proof cue exists"
else
    echo -e "${RED}✗${NC} CSS styling for proof cue missing"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.5 guardrail checks passed.${NC}"
    exit 0
else
    echo -e "${RED}Some Phase 18.5 guardrail checks failed.${NC}"
    exit 1
fi
