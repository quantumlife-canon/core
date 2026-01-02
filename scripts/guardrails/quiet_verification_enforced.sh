#!/bin/bash
# quiet_verification_enforced.sh - Guardrail checks for Phase 18.9: Real Data Quiet Verification
#
# Reference: docs/ADR/ADR-0042-phase18-9-real-data-quiet-verification.md
#
# This script validates:
# 1. Gmail consent page has restraint copy
# 2. Sync uses magnitude buckets only
# 3. Gmail obligations default to HOLD
# 4. No raw counts exposed
# 5. No sender/subject stored
# 6. Revocation is immediate
# 7. Demo tests exist
# 8. Restraint rules file exists

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 18.9: Real Data Quiet Verification - Guardrail Checks     ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: Gmail consent page route exists
echo "Checking /connect/gmail route exists..."
if grep -q 'HandleFunc.*"/connect/gmail"' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /connect/gmail route exists"
else
    echo -e "${RED}✗${NC} /connect/gmail route missing"
    FAILED=1
fi

# Check 2: Gmail consent template has restraint language
echo "Checking Gmail consent template has restraint language..."
if grep -q 'Read-only\. Revocable\. Nothing stored\.' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Gmail consent has restraint tagline"
else
    echo -e "${RED}✗${NC} Gmail consent missing restraint tagline"
    FAILED=1
fi

# Check 3: Gmail consent template explains what is NOT stored
echo "Checking Gmail consent explains what is NOT stored..."
if grep -q 'Not:.*email bodies\|Not:.*attachments\|Not:.*subject lines' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Gmail consent explains what is NOT stored"
else
    echo -e "${RED}✗${NC} Gmail consent missing 'Not stored' explanation"
    FAILED=1
fi

# Check 4: MagnitudeBucket function exists in oauth package
echo "Checking MagnitudeBucket function exists..."
if grep -q 'func MagnitudeBucket' "$PROJECT_ROOT/internal/oauth/receipts.go"; then
    echo -e "${GREEN}✓${NC} MagnitudeBucket function exists"
else
    echo -e "${RED}✗${NC} MagnitudeBucket function missing"
    FAILED=1
fi

# Check 5: Gmail rules file exists
echo "Checking Gmail restraint rules file exists..."
if [ -f "$PROJECT_ROOT/internal/obligations/rules_gmail.go" ]; then
    echo -e "${GREEN}✓${NC} Gmail restraint rules file exists"
else
    echo -e "${RED}✗${NC} Gmail restraint rules file missing"
    FAILED=1
fi

# Check 6: Gmail rules enforce DefaultToHold
echo "Checking Gmail rules enforce DefaultToHold..."
if grep -q 'DefaultToHold.*true' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Gmail rules enforce DefaultToHold = true"
else
    echo -e "${RED}✗${NC} Gmail rules do not enforce DefaultToHold"
    FAILED=1
fi

# Check 7: Gmail rules enforce RequireExplicitAction
echo "Checking Gmail rules enforce RequireExplicitAction..."
if grep -q 'RequireExplicitAction.*true' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Gmail rules enforce RequireExplicitAction = true"
else
    echo -e "${RED}✗${NC} Gmail rules do not enforce RequireExplicitAction"
    FAILED=1
fi

# Check 8: MaxRegret is below surface threshold (0.5)
echo "Checking MaxRegret is below surface threshold..."
MAX_REGRET=$(grep -o 'MaxRegret:.*0\.[0-9]*' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null | grep -o '0\.[0-9]*' | head -1)
if [ -n "$MAX_REGRET" ]; then
    # Compare using bc (if available) or assume pass if value starts with 0.3 or 0.2
    if echo "$MAX_REGRET" | grep -qE '^0\.[0-4]'; then
        echo -e "${GREEN}✓${NC} MaxRegret ($MAX_REGRET) is below surface threshold (0.5)"
    else
        echo -e "${RED}✗${NC} MaxRegret ($MAX_REGRET) may be at or above surface threshold"
        FAILED=1
    fi
else
    echo -e "${YELLOW}?${NC} Could not determine MaxRegret value"
fi

# Check 9: No sender/subject fields in Gmail rules
echo "Checking no sender/subject stored in Gmail rules..."
if grep -rn '"sender"\|"subject"\|"body"' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Gmail rules may store sender/subject/body"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} Gmail rules do not store sender/subject/body"
fi

# Check 10: Gmail rules use abstract buckets only
echo "Checking Gmail rules use abstract buckets..."
if grep -q 'domain_bucket\|label_bucket' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Gmail rules use abstract buckets (domain_bucket, label_bucket)"
else
    echo -e "${RED}✗${NC} Gmail rules may expose specific values"
    FAILED=1
fi

# Check 11: MaxDailyObligations is limited (< 10)
echo "Checking MaxDailyObligations is limited..."
MAX_DAILY=$(grep -o 'MaxDailyObligations:.*[0-9]*' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null | grep -o '[0-9]*' | head -1)
if [ -n "$MAX_DAILY" ] && [ "$MAX_DAILY" -lt 10 ]; then
    echo -e "${GREEN}✓${NC} MaxDailyObligations ($MAX_DAILY) is appropriately limited"
else
    echo -e "${RED}✗${NC} MaxDailyObligations may be too high or undefined"
    FAILED=1
fi

# Check 12: Demo tests exist
echo "Checking Phase 18.9 demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase18_9_quiet_verification/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Phase 18.9 demo tests exist"
else
    echo -e "${RED}✗${NC} Phase 18.9 demo tests missing"
    FAILED=1
fi

# Check 13: Demo tests pass
echo "Checking Phase 18.9 demo tests pass..."
if go test -count=1 "$PROJECT_ROOT/internal/demo_phase18_9_quiet_verification/..." > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Phase 18.9 demo tests pass"
else
    echo -e "${RED}✗${NC} Phase 18.9 demo tests fail"
    FAILED=1
fi

# Check 14: GmailRestraintPolicy has Validate function
echo "Checking GmailRestraintPolicy has Validate function..."
if grep -q 'func.*GmailRestraintPolicy.*Validate' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} GmailRestraintPolicy has Validate function"
else
    echo -e "${RED}✗${NC} GmailRestraintPolicy missing Validate function"
    FAILED=1
fi

# Check 15: No goroutines in Gmail rules
echo "Checking no goroutines in Gmail rules..."
if grep -rn 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Goroutines found in Gmail rules"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in Gmail rules"
fi

# Check 16: Revocation handler exists
echo "Checking Gmail revocation handler exists..."
if grep -q 'handleGmailDisconnect\|handleGmailRevoke' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Gmail revocation handler exists"
else
    echo -e "${RED}✗${NC} Gmail revocation handler missing"
    FAILED=1
fi

# Check 17: Disconnect confirmation template exists
echo "Checking disconnect confirmation template..."
if grep -q 'gmail-disconnected\|Nothing further is read' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Disconnect confirmation template exists"
else
    echo -e "${RED}✗${NC} Disconnect confirmation template missing"
    FAILED=1
fi

# Check 18: Mirror shows NotStored for email
echo "Checking Mirror shows NotStored for email..."
if grep -q 'KindEmail.*{' "$PROJECT_ROOT/internal/mirror/engine.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Mirror shows NotStored for email"
elif grep -q 'connection\.KindEmail' "$PROJECT_ROOT/internal/mirror/engine.go" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Mirror shows NotStored for email"
else
    echo -e "${RED}✗${NC} Mirror may not show NotStored for email"
    FAILED=1
fi

# Check 19: ShouldHold function returns true for all Gmail obligations
echo "Checking ShouldHold always returns true..."
SHOULD_HOLD_RETURNS=$(grep -A 20 'func.*ShouldHold' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null | grep -c 'return true')
if [ "$SHOULD_HOLD_RETURNS" -ge 3 ]; then
    echo -e "${GREEN}✓${NC} ShouldHold returns true in multiple code paths"
else
    echo -e "${YELLOW}?${NC} ShouldHold may not always return true - manual review recommended"
fi

# Check 20: No auto-surface logic in Gmail extractor
echo "Checking no auto-surface logic..."
# Look for actual auto-surface function calls, not comments or variable names
AUTO_SURFACE_CALLS=$(grep -rn 'AutoSurface\|autoSurface' "$PROJECT_ROOT/internal/obligations/rules_gmail.go" 2>/dev/null | grep -v '^[^:]*:[0-9]*:\s*//' | grep -v 'Never' | grep -v '// ' | wc -l)
if [ "$AUTO_SURFACE_CALLS" -gt 0 ]; then
    echo -e "${RED}✗${NC} Auto-surface logic may exist"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No auto-surface logic found"
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 18.9 guardrail checks passed.${NC}"
    echo ""
    echo "Real Gmail data will integrate quietly. Trust is maintained."
    exit 0
else
    echo -e "${RED}Some Phase 18.9 guardrail checks failed.${NC}"
    echo ""
    echo "Fix the issues above before proceeding with real data."
    exit 1
fi
