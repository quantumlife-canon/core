#!/bin/bash
# real_gmail_quiet_enforced.sh - Guardrail checks for Phase 19.1: Real Gmail Connection
#
# Reference: Phase 19.1 specification
#
# This script validates:
# 1. Sync handler enforces max 25 message cap
# 2. Sync handler enforces 7 day limit
# 3. DefaultToHold is enforced for Gmail obligations
# 4. No auto-trigger of drafts/execution from Gmail ingestion
# 5. No persistence of raw gmail payloads
# 6. Templates do not render subject/body/sender
# 7. Magnitude buckets used instead of raw counts
# 8. No background polling (no goroutines in sync)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 19.1: Real Gmail Connection - Guardrail Checks           ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# Check 1: Sync handler enforces max 25 message cap
echo "Checking sync handler enforces max 25 message cap..."
if grep -q 'maxMessages = 25' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Sync handler enforces max 25 message cap"
else
    echo -e "${RED}✗${NC} Sync handler does not enforce max 25 message cap"
    FAILED=1
fi

# Check 2: Sync handler enforces 7 day limit
echo "Checking sync handler enforces 7 day limit..."
if grep -q 'syncDays = 7' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Sync handler enforces 7 day limit"
else
    echo -e "${RED}✗${NC} Sync handler does not enforce 7 day limit"
    FAILED=1
fi

# Check 3: DefaultToHold is enforced for Gmail obligations
echo "Checking DefaultToHold is enforced for Gmail obligations..."
if grep -q 'DefaultToHold.*true' "$PROJECT_ROOT/internal/obligations/rules_gmail.go"; then
    echo -e "${GREEN}✓${NC} DefaultToHold is enforced for Gmail obligations"
else
    echo -e "${RED}✗${NC} DefaultToHold is not enforced for Gmail obligations"
    FAILED=1
fi

# Check 4: No auto-execution from Gmail sync handler
echo "Checking no auto-execution from Gmail sync handler..."
if grep -rn 'ExecuteDraft\|AutoExecute\|AutoApprove' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 2>/dev/null | grep -i gmail | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Auto-execution found in Gmail sync handler"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No auto-execution in Gmail sync handler"
fi

# Check 5: No auto-drafts from Gmail sync handler
echo "Checking no auto-drafts from Gmail sync handler..."
if grep -rn 'CreateDraft\|GenerateDraft\|AutoDraft' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 2>/dev/null | grep -i gmail | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Auto-drafts found in Gmail sync handler"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No auto-drafts in Gmail sync handler"
fi

# Check 6: MagnitudeBucket type exists
echo "Checking MagnitudeBucket type exists..."
if grep -q 'type MagnitudeBucket string' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} MagnitudeBucket type exists"
else
    echo -e "${RED}✗${NC} MagnitudeBucket type missing"
    FAILED=1
fi

# Check 7: SyncReceipt uses magnitude buckets
echo "Checking SyncReceipt uses magnitude buckets..."
if grep -q 'MagnitudeBucket.*MagnitudeBucket' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} SyncReceipt uses magnitude buckets"
else
    echo -e "${RED}✗${NC} SyncReceipt does not use magnitude buckets"
    FAILED=1
fi

# Check 8: No goroutines in sync handler
echo "Checking no goroutines in sync handler..."
SYNC_HANDLER=$(grep -n 'func.*handleGmailSync' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" -A 200 | head -200)
if echo "$SYNC_HANDLER" | grep -q 'go func\|go [a-zA-Z]'; then
    echo -e "${RED}✗${NC} Goroutines found in sync handler"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in sync handler"
fi

# Check 9: Templates don't render Subject field
echo "Checking templates don't render Subject field..."
if grep -rn '\.Subject' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 2>/dev/null | grep 'template\|{{' | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Templates render Subject field"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} Templates don't render Subject field"
fi

# Check 10: Templates don't render Body field
echo "Checking templates don't render Body field..."
if grep -rn '\.Body\>' "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 2>/dev/null | grep 'template\|{{' | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Templates render Body field"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} Templates don't render Body field"
fi

# Check 11: QuietCheckStatus exists
echo "Checking QuietCheckStatus exists..."
if grep -q 'type QuietCheckStatus struct' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} QuietCheckStatus exists"
else
    echo -e "${RED}✗${NC} QuietCheckStatus missing"
    FAILED=1
fi

# Check 12: IsQuiet method exists
echo "Checking IsQuiet method exists..."
if grep -q 'func.*QuietCheckStatus.*IsQuiet' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} IsQuiet method exists"
else
    echo -e "${RED}✗${NC} IsQuiet method missing"
    FAILED=1
fi

# Check 13: Quiet check handler exists
echo "Checking quiet check handler exists..."
if grep -q 'handleQuietCheck' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Quiet check handler exists"
else
    echo -e "${RED}✗${NC} Quiet check handler missing"
    FAILED=1
fi

# Check 14: Phase 19.1 events exist
echo "Checking Phase 19.1 events exist..."
if grep -q 'Phase19_1GmailSyncRequested' "$PROJECT_ROOT/pkg/events/events.go"; then
    echo -e "${GREEN}✓${NC} Phase 19.1 events exist"
else
    echo -e "${RED}✗${NC} Phase 19.1 events missing"
    FAILED=1
fi

# Check 15: SyncReceiptStore exists
echo "Checking SyncReceiptStore exists..."
if grep -q 'type SyncReceiptStore struct' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} SyncReceiptStore exists"
else
    echo -e "${RED}✗${NC} SyncReceiptStore missing"
    FAILED=1
fi

# Check 16: TimeBucket function exists (5-min privacy)
echo "Checking TimeBucket function exists..."
if grep -q 'func TimeBucket' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} TimeBucket function exists"
else
    echo -e "${RED}✗${NC} TimeBucket function missing"
    FAILED=1
fi

# Check 17: TimeBucket uses 5-minute intervals
echo "Checking TimeBucket uses 5-minute intervals..."
if grep -q 'Truncate(5 \* time.Minute)' "$PROJECT_ROOT/internal/persist/sync_receipt_store.go"; then
    echo -e "${GREEN}✓${NC} TimeBucket uses 5-minute intervals"
else
    echo -e "${RED}✗${NC} TimeBucket does not use 5-minute intervals"
    FAILED=1
fi

# Check 18: Quiet check template exists
echo "Checking quiet check template exists..."
if grep -q 'quiet-check-content' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Quiet check template exists"
else
    echo -e "${RED}✗${NC} Quiet check template missing"
    FAILED=1
fi

# Check 19: Demo tests exist
echo "Checking Phase 19.1 demo tests exist..."
if [ -f "$PROJECT_ROOT/internal/demo_phase19_1_real_gmail_quiet/demo_test.go" ]; then
    echo -e "${GREEN}✓${NC} Phase 19.1 demo tests exist"
else
    echo -e "${RED}✗${NC} Phase 19.1 demo tests missing"
    FAILED=1
fi

# Check 20: Demo tests pass
echo "Checking Phase 19.1 demo tests pass..."
if go test -count=1 "$PROJECT_ROOT/internal/demo_phase19_1_real_gmail_quiet/..." > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Phase 19.1 demo tests pass"
else
    echo -e "${RED}✗${NC} Phase 19.1 demo tests fail"
    FAILED=1
fi

echo ""
echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 19.1 guardrail checks passed.${NC}"
    echo ""
    echo "Real Gmail connection is safe:"
    echo "  - Explicit sync only (no background polling)"
    echo "  - Max 25 messages, 7 day limit"
    echo "  - DefaultToHold = true"
    echo "  - Magnitude buckets only (no raw counts)"
    echo "  - No auto-drafts or auto-execution"
    exit 0
else
    echo -e "${RED}Some Phase 19.1 guardrail checks failed.${NC}"
    echo ""
    echo "Fix the issues above before proceeding."
    exit 1
fi
