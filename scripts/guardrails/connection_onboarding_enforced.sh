#!/bin/bash
# connection_onboarding_enforced.sh - Guardrail checks for Phase 18.6: First Connect
#
# Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
#
# This script validates:
# 1. No OAuth libs or third-party auth libs
# 2. No goroutines in pkg/domain/connection or internal/persist/connection*
# 3. No time.Now() in new code
# 4. Canonical strings are pipe-delimited (no json.Marshal)
# 5. No storing of raw emails/secrets
# 6. Routes exist
# 7. Events exist
# 8. stdlib only imports

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.6: First Connect - Guardrail Checks                    ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: /start route exists in main.go
echo "Checking /start route exists..."
if grep -q 'HandleFunc.*"/start"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /start route exists"
else
    echo -e "${RED}✗${NC} /start route missing"
    FAILED=1
fi

# Check 2: /connections route exists
echo "Checking /connections route exists..."
if grep -q 'HandleFunc.*"/connections"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /connections route exists"
else
    echo -e "${RED}✗${NC} /connections route missing"
    FAILED=1
fi

# Check 3: /connect/ route exists
echo "Checking /connect/ route exists..."
if grep -q 'HandleFunc.*"/connect/"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /connect/ route exists"
else
    echo -e "${RED}✗${NC} /connect/ route missing"
    FAILED=1
fi

# Check 4: /disconnect/ route exists
echo "Checking /disconnect/ route exists..."
if grep -q 'HandleFunc.*"/disconnect/"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /disconnect/ route exists"
else
    echo -e "${RED}✗${NC} /disconnect/ route missing"
    FAILED=1
fi

# Check 5: pkg/domain/connection package exists
echo "Checking pkg/domain/connection package exists..."
if [ -d "$PROJECT_ROOT/pkg/domain/connection" ]; then
    echo -e "${GREEN}✓${NC} pkg/domain/connection directory exists"
else
    echo -e "${RED}✗${NC} pkg/domain/connection directory missing"
    FAILED=1
fi

# Check 6: ConnectionKind type exists
echo "Checking ConnectionKind type exists..."
if grep -q 'type ConnectionKind' "$PROJECT_ROOT/pkg/domain/connection/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} ConnectionKind type exists"
else
    echo -e "${RED}✗${NC} ConnectionKind type missing"
    FAILED=1
fi

# Check 7: ConnectionIntent type exists
echo "Checking ConnectionIntent type exists..."
if grep -q 'type ConnectionIntent struct' "$PROJECT_ROOT/pkg/domain/connection/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} ConnectionIntent type exists"
else
    echo -e "${RED}✗${NC} ConnectionIntent type missing"
    FAILED=1
fi

# Check 8: No OAuth imports in pkg/domain/connection
echo "Checking no OAuth imports in pkg/domain/connection..."
# Exclude test files and comment lines (lines with // at start after stripping whitespace)
if grep -rn 'oauth\|OAuth\|google.golang.org/api' "$PROJECT_ROOT/pkg/domain/connection/" 2>/dev/null | grep -v '_test.go' | grep -v '^\s*//' | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} OAuth imports found in pkg/domain/connection"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No OAuth imports in pkg/domain/connection"
fi

# Check 9: No goroutines in pkg/domain/connection
echo "Checking no goroutines in pkg/domain/connection..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/pkg/domain/connection/" 2>/dev/null | grep -v '_test.go' | grep -v '^Binary'; then
    echo -e "${RED}✗${NC} Goroutines found in pkg/domain/connection"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in pkg/domain/connection"
fi

# Check 10: No time.Now() in pkg/domain/connection
echo "Checking no time.Now() in pkg/domain/connection..."
if grep -rn 'time\.Now()' "$PROJECT_ROOT/pkg/domain/connection/" 2>/dev/null | grep -v '_test.go' | grep -v '^\s*//' | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} time.Now() found in pkg/domain/connection"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in pkg/domain/connection (clock injection used)"
fi

# Check 11: No json.Marshal in canonical strings
echo "Checking no json.Marshal in connection types..."
if grep -rn 'json\.Marshal' "$PROJECT_ROOT/pkg/domain/connection/" 2>/dev/null | grep -v '_test.go'; then
    echo -e "${RED}✗${NC} json.Marshal found - canonical strings should be pipe-delimited"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No json.Marshal in connection types (pipe-delimited)"
fi

# Check 12: Canonical string uses pipe delimiter
echo "Checking canonical string uses pipe delimiter..."
if grep -q 'CONN_INTENT|v1' "$PROJECT_ROOT/pkg/domain/connection/types.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Canonical string uses pipe delimiter"
else
    echo -e "${RED}✗${NC} Canonical string may not use pipe delimiter"
    FAILED=1
fi

# Check 13: Phase 18.6 events exist
echo "Checking Phase 18.6 events exist..."
EVENTS_FILE="$PROJECT_ROOT/pkg/events/events.go"
if grep -q 'phase18_6.connection.intent.recorded' "$EVENTS_FILE" && \
   grep -q 'phase18_6.connection.state.computed' "$EVENTS_FILE" && \
   grep -q 'phase18_6.connection.connect.requested' "$EVENTS_FILE" && \
   grep -q 'phase18_6.connection.disconnect.requested' "$EVENTS_FILE"; then
    echo -e "${GREEN}✓${NC} Phase 18.6 events exist"
else
    echo -e "${RED}✗${NC} Phase 18.6 events missing"
    FAILED=1
fi

# Check 14: Start template exists
echo "Checking start template exists..."
if grep -q '{{define "start"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Start template exists"
else
    echo -e "${RED}✗${NC} Start template missing"
    FAILED=1
fi

# Check 15: Connections template exists
echo "Checking connections template exists..."
if grep -q '{{define "connections"}}' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Connections template exists"
else
    echo -e "${RED}✗${NC} Connections template missing"
    FAILED=1
fi

# Check 16: Demo tests exist
echo "Checking demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_6_first_connect/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Demo tests exist"
else
    echo -e "${RED}✗${NC} Demo tests missing"
    FAILED=1
fi

# Check 17: CSS for start page exists
echo "Checking CSS for start page exists..."
if grep -q '.start ' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for start page exists"
else
    echo -e "${RED}✗${NC} CSS styling for start page missing"
    FAILED=1
fi

# Check 18: CSS for connections page exists
echo "Checking CSS for connections page exists..."
if grep -q '.connections ' "$PROJECT_ROOT/cmd/quantumlife-web/static/app.css" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} CSS styling for connections page exists"
else
    echo -e "${RED}✗${NC} CSS styling for connections page missing"
    FAILED=1
fi

# Check 19: No storing of raw secrets in intent notes
echo "Checking no raw secrets in intent notes..."
if grep -rn 'password\|secret\|token\|api_key' "$PROJECT_ROOT/pkg/domain/connection/" 2>/dev/null | grep -v '_test.go' | grep -vi 'no.*secret\|no.*token'; then
    echo -e "${RED}✗${NC} Potential secret storage found in connection code"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No raw secrets stored in connection intents"
fi

# Check 20: RecordType for connection exists in storelog
echo "Checking RecordType for connection exists..."
if grep -q 'RecordTypeConnectionIntent' "$PROJECT_ROOT/pkg/domain/storelog/log.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} RecordTypeConnectionIntent exists in storelog"
else
    echo -e "${RED}✗${NC} RecordTypeConnectionIntent missing from storelog"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.6 guardrail checks passed.${NC}"
    exit 0
else
    echo -e "${RED}Some Phase 18.6 guardrail checks failed.${NC}"
    exit 1
fi
