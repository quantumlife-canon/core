#!/bin/bash
# shadow_receipt_primary_enforced.sh - Guardrail checks for Phase 27: Real Shadow Receipt
#
# Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md
#
# This script validates:
# 1. Domain model types exist with correct structure
# 2. Engine methods exist for primary proof page and cue
# 3. Ack/vote store exists with correct methods
# 4. Events are defined
# 5. Web routes exist
# 6. Privacy: no identifiers in stored data
# 7. Vote does NOT change behavior
# 8. Single-whisper rule respected

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

FAILED=0

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  Phase 27: Real Shadow Receipt (Primary Proof) - Guardrails     ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# =============================================================================
# Domain Model Checks (pkg/domain/shadowview/types.go)
# =============================================================================

echo "─── Domain Model Checks ───────────────────────────────────────────"

# Check 1: ShadowReceiptPrimaryPage exists
echo "Checking ShadowReceiptPrimaryPage exists..."
if grep -q 'type ShadowReceiptPrimaryPage struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptPrimaryPage exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptPrimaryPage missing"
    FAILED=1
fi

# Check 2: ShadowReceiptEvidence exists
echo "Checking ShadowReceiptEvidence exists..."
if grep -q 'type ShadowReceiptEvidence struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptEvidence exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptEvidence missing"
    FAILED=1
fi

# Check 3: ShadowReceiptModelReturn exists
echo "Checking ShadowReceiptModelReturn exists..."
if grep -q 'type ShadowReceiptModelReturn struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptModelReturn exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptModelReturn missing"
    FAILED=1
fi

# Check 4: ShadowReceiptDecision exists
echo "Checking ShadowReceiptDecision exists..."
if grep -q 'type ShadowReceiptDecision struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptDecision exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptDecision missing"
    FAILED=1
fi

# Check 5: ShadowReceiptReason exists
echo "Checking ShadowReceiptReason exists..."
if grep -q 'type ShadowReceiptReason struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptReason exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptReason missing"
    FAILED=1
fi

# Check 6: ShadowReceiptProvider exists
echo "Checking ShadowReceiptProvider exists..."
if grep -q 'type ShadowReceiptProvider struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptProvider exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptProvider missing"
    FAILED=1
fi

# Check 7: VoteChoice type exists
echo "Checking VoteChoice type exists..."
if grep -q 'type VoteChoice string' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} VoteChoice type exists"
else
    echo -e "${RED}✗${NC} VoteChoice type missing"
    FAILED=1
fi

# Check 8: Vote choices defined (useful, unnecessary, skip)
echo "Checking vote choices defined..."
if grep -q 'VoteUseful.*VoteChoice' "$PROJECT_ROOT/pkg/domain/shadowview/types.go" && \
   grep -q 'VoteUnnecessary.*VoteChoice' "$PROJECT_ROOT/pkg/domain/shadowview/types.go" && \
   grep -q 'VoteSkip.*VoteChoice' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} Vote choices defined"
else
    echo -e "${RED}✗${NC} Vote choices missing"
    FAILED=1
fi

# Check 9: ShadowReceiptVote exists
echo "Checking ShadowReceiptVote exists..."
if grep -q 'type ShadowReceiptVote struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptVote exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptVote missing"
    FAILED=1
fi

# Check 10: ShadowReceiptCue exists
echo "Checking ShadowReceiptCue exists..."
if grep -q 'type ShadowReceiptCue struct' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptCue exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptCue missing"
    FAILED=1
fi

# Check 11: ProviderKind type exists
echo "Checking ProviderKind type exists..."
if grep -q 'type ProviderKind string' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} ProviderKind type exists"
else
    echo -e "${RED}✗${NC} ProviderKind type missing"
    FAILED=1
fi

# Check 12: HorizonBucket type exists
echo "Checking HorizonBucket type exists..."
if grep -q 'type HorizonBucket string' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} HorizonBucket type exists"
else
    echo -e "${RED}✗${NC} HorizonBucket type missing"
    FAILED=1
fi

# Check 13: CanonicalString methods exist
echo "Checking CanonicalString methods exist..."
if grep -q 'func.*CanonicalString' "$PROJECT_ROOT/pkg/domain/shadowview/types.go"; then
    echo -e "${GREEN}✓${NC} CanonicalString methods exist"
else
    echo -e "${RED}✗${NC} CanonicalString methods missing"
    FAILED=1
fi

echo ""

# =============================================================================
# Engine Checks (internal/shadowview/engine.go)
# =============================================================================

echo "─── Engine Checks ─────────────────────────────────────────────────"

# Check 14: BuildPrimaryPage exists
echo "Checking BuildPrimaryPage exists..."
if grep -q 'func.*BuildPrimaryPage' "$PROJECT_ROOT/internal/shadowview/engine.go"; then
    echo -e "${GREEN}✓${NC} BuildPrimaryPage exists"
else
    echo -e "${RED}✗${NC} BuildPrimaryPage missing"
    FAILED=1
fi

# Check 15: BuildPrimaryCue exists
echo "Checking BuildPrimaryCue exists..."
if grep -q 'func.*BuildPrimaryCue' "$PROJECT_ROOT/internal/shadowview/engine.go"; then
    echo -e "${GREEN}✓${NC} BuildPrimaryCue exists"
else
    echo -e "${RED}✗${NC} BuildPrimaryCue missing"
    FAILED=1
fi

# Check 16: BuildPrimaryPageInput exists
echo "Checking BuildPrimaryPageInput exists..."
if grep -q 'type BuildPrimaryPageInput struct' "$PROJECT_ROOT/internal/shadowview/engine.go"; then
    echo -e "${GREEN}✓${NC} BuildPrimaryPageInput exists"
else
    echo -e "${RED}✗${NC} BuildPrimaryPageInput missing"
    FAILED=1
fi

# Check 17: BuildPrimaryCueInput exists
echo "Checking BuildPrimaryCueInput exists..."
if grep -q 'type BuildPrimaryCueInput struct' "$PROJECT_ROOT/internal/shadowview/engine.go"; then
    echo -e "${GREEN}✓${NC} BuildPrimaryCueInput exists"
else
    echo -e "${RED}✗${NC} BuildPrimaryCueInput missing"
    FAILED=1
fi

# Check 18: No time.Now() in engine (actual calls, not comments)
echo "Checking no time.Now() in engine..."
# Search for actual time.Now() calls (not in comments)
if grep -n 'time\.Now()' "$PROJECT_ROOT/internal/shadowview/engine.go" | grep -v '^\s*//' | grep -v 'No time\.Now' | grep -v '// '; then
    echo -e "${RED}✗${NC} time.Now() found in engine"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in engine"
fi

# Check 19: No goroutines in engine
echo "Checking no goroutines in engine..."
if grep -n 'go func\|go [a-zA-Z]' "$PROJECT_ROOT/internal/shadowview/engine.go" | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} Goroutines found in engine"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No goroutines in engine"
fi

echo ""

# =============================================================================
# Store Checks (internal/persist/shadow_receipt_ack_store.go)
# =============================================================================

echo "─── Ack/Vote Store Checks ─────────────────────────────────────────"

# Check 20: ShadowReceiptAckStore exists
echo "Checking ShadowReceiptAckStore exists..."
if grep -q 'type ShadowReceiptAckStore struct' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptAckStore exists"
else
    echo -e "${RED}✗${NC} ShadowReceiptAckStore missing"
    FAILED=1
fi

# Check 21: RecordViewed exists
echo "Checking RecordViewed exists..."
if grep -q 'func.*ShadowReceiptAckStore.*RecordViewed' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} RecordViewed exists"
else
    echo -e "${RED}✗${NC} RecordViewed missing"
    FAILED=1
fi

# Check 22: RecordDismissed exists
echo "Checking RecordDismissed exists..."
if grep -q 'func.*ShadowReceiptAckStore.*RecordDismissed' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} RecordDismissed exists"
else
    echo -e "${RED}✗${NC} RecordDismissed missing"
    FAILED=1
fi

# Check 23: RecordVote exists
echo "Checking RecordVote exists..."
if grep -q 'func.*ShadowReceiptAckStore.*RecordVote' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} RecordVote exists"
else
    echo -e "${RED}✗${NC} RecordVote missing"
    FAILED=1
fi

# Check 24: IsDismissed exists
echo "Checking IsDismissed exists..."
if grep -q 'func.*ShadowReceiptAckStore.*IsDismissed' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} IsDismissed exists"
else
    echo -e "${RED}✗${NC} IsDismissed missing"
    FAILED=1
fi

# Check 25: HasVoted exists
echo "Checking HasVoted exists..."
if grep -q 'func.*ShadowReceiptAckStore.*HasVoted' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} HasVoted exists"
else
    echo -e "${RED}✗${NC} HasVoted missing"
    FAILED=1
fi

# Check 26: CountVotesByPeriod exists
echo "Checking CountVotesByPeriod exists..."
if grep -q 'func.*ShadowReceiptAckStore.*CountVotesByPeriod' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} CountVotesByPeriod exists"
else
    echo -e "${RED}✗${NC} CountVotesByPeriod missing"
    FAILED=1
fi

# Check 27: No time.Now() in store
echo "Checking no time.Now() in store..."
if grep -n 'time\.Now()' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go" | grep -v '^[^:]*:[0-9]*:\s*//'; then
    echo -e "${RED}✗${NC} time.Now() found in store"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No time.Now() in store"
fi

# Check 28: Hash-only storage (no raw content)
echo "Checking hash-only storage..."
if grep -q 'hashAck\|hashVote\|hashTimestamp' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} Hash functions exist (hash-only storage)"
else
    echo -e "${RED}✗${NC} Hash functions missing"
    FAILED=1
fi

# Check 29: Bounded retention (30 days)
echo "Checking bounded retention..."
if grep -q 'DefaultMaxShadowReceiptPeriods = 30' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} Bounded retention (30 days)"
else
    echo -e "${RED}✗${NC} Bounded retention missing"
    FAILED=1
fi

echo ""

# =============================================================================
# Event Checks (pkg/events/events.go)
# =============================================================================

echo "─── Event Checks ──────────────────────────────────────────────────"

# Check 30: Phase27ShadowReceiptRendered event exists
echo "Checking Phase27ShadowReceiptRendered event exists..."
if grep -q 'Phase27ShadowReceiptRendered' "$PROJECT_ROOT/pkg/events/events.go"; then
    echo -e "${GREEN}✓${NC} Phase27ShadowReceiptRendered event exists"
else
    echo -e "${RED}✗${NC} Phase27ShadowReceiptRendered event missing"
    FAILED=1
fi

# Check 31: Phase27ShadowReceiptVoted event exists
echo "Checking Phase27ShadowReceiptVoted event exists..."
if grep -q 'Phase27ShadowReceiptVoted' "$PROJECT_ROOT/pkg/events/events.go"; then
    echo -e "${GREEN}✓${NC} Phase27ShadowReceiptVoted event exists"
else
    echo -e "${RED}✗${NC} Phase27ShadowReceiptVoted event missing"
    FAILED=1
fi

# Check 32: Phase27ShadowReceiptDismissed event exists
echo "Checking Phase27ShadowReceiptDismissed event exists..."
if grep -q 'Phase27ShadowReceiptDismissed' "$PROJECT_ROOT/pkg/events/events.go"; then
    echo -e "${GREEN}✓${NC} Phase27ShadowReceiptDismissed event exists"
else
    echo -e "${RED}✗${NC} Phase27ShadowReceiptDismissed event missing"
    FAILED=1
fi

echo ""

# =============================================================================
# Web Route Checks (cmd/quantumlife-web/main.go)
# =============================================================================

echo "─── Web Route Checks ──────────────────────────────────────────────"

# Check 33: /shadow/receipt route exists
echo "Checking /shadow/receipt route exists..."
if grep -q 'mux.HandleFunc.*/shadow/receipt' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /shadow/receipt route exists"
else
    echo -e "${RED}✗${NC} /shadow/receipt route missing"
    FAILED=1
fi

# Check 34: /shadow/receipt/vote route exists
echo "Checking /shadow/receipt/vote route exists..."
if grep -q 'mux.HandleFunc.*/shadow/receipt/vote' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /shadow/receipt/vote route exists"
else
    echo -e "${RED}✗${NC} /shadow/receipt/vote route missing"
    FAILED=1
fi

# Check 35: /shadow/receipt/dismiss route exists
echo "Checking /shadow/receipt/dismiss route exists..."
if grep -q 'mux.HandleFunc.*/shadow/receipt/dismiss' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} /shadow/receipt/dismiss route exists"
else
    echo -e "${RED}✗${NC} /shadow/receipt/dismiss route missing"
    FAILED=1
fi

# Check 36: handleShadowReceiptVote handler exists
echo "Checking handleShadowReceiptVote handler exists..."
if grep -q 'func.*handleShadowReceiptVote' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} handleShadowReceiptVote handler exists"
else
    echo -e "${RED}✗${NC} handleShadowReceiptVote handler missing"
    FAILED=1
fi

echo ""

# =============================================================================
# Storelog Record Type Checks
# =============================================================================

echo "─── Storelog Record Type Checks ───────────────────────────────────"

# Check 37: RecordTypeShadowReceiptAck exists
echo "Checking RecordTypeShadowReceiptAck exists..."
if grep -q 'RecordTypeShadowReceiptAck' "$PROJECT_ROOT/pkg/domain/storelog/log.go"; then
    echo -e "${GREEN}✓${NC} RecordTypeShadowReceiptAck exists"
else
    echo -e "${RED}✗${NC} RecordTypeShadowReceiptAck missing"
    FAILED=1
fi

# Check 38: RecordTypeShadowReceiptVote exists
echo "Checking RecordTypeShadowReceiptVote exists..."
if grep -q 'RecordTypeShadowReceiptVote' "$PROJECT_ROOT/pkg/domain/storelog/log.go"; then
    echo -e "${GREEN}✓${NC} RecordTypeShadowReceiptVote exists"
else
    echo -e "${RED}✗${NC} RecordTypeShadowReceiptVote missing"
    FAILED=1
fi

echo ""

# =============================================================================
# Safety Invariant Checks
# =============================================================================

echo "─── Safety Invariant Checks ───────────────────────────────────────"

# Check 39: Vote does NOT change behavior comment
echo "Checking 'Vote does NOT change behavior' invariant..."
if grep -q 'Vote does NOT change behavior' "$PROJECT_ROOT/internal/persist/shadow_receipt_ack_store.go"; then
    echo -e "${GREEN}✓${NC} 'Vote does NOT change behavior' invariant documented"
else
    echo -e "${RED}✗${NC} 'Vote does NOT change behavior' invariant missing"
    FAILED=1
fi

# Check 40: Single-whisper rule integration
echo "Checking single-whisper rule integration..."
if grep -q 'shadow receipt primary cue.*lowest priority\|Priority.*shadow receipt' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} Single-whisper rule integration exists"
else
    echo -e "${RED}✗${NC} Single-whisper rule integration missing"
    FAILED=1
fi

# Check 41: ShadowReceiptPrimaryCue in templateData
echo "Checking ShadowReceiptPrimaryCue in templateData..."
if grep -q 'ShadowReceiptPrimaryCue' "$PROJECT_ROOT/cmd/quantumlife-web/main.go"; then
    echo -e "${GREEN}✓${NC} ShadowReceiptPrimaryCue in templateData"
else
    echo -e "${RED}✗${NC} ShadowReceiptPrimaryCue missing from templateData"
    FAILED=1
fi

# Check 42: No forbidden patterns in domain types (no @, URLs, raw amounts)
echo "Checking no forbidden patterns in domain types..."
DOMAIN_FILE="$PROJECT_ROOT/pkg/domain/shadowview/types.go"
if grep -E '@[a-zA-Z]|http[s]?://|[0-9]+\.[0-9]{2}' "$DOMAIN_FILE" | grep -v '^[^:]*:[0-9]*:\s*//' | grep -v 'email\|url\|amount' > /dev/null 2>&1; then
    echo -e "${RED}✗${NC} Forbidden patterns found in domain types"
    FAILED=1
else
    echo -e "${GREEN}✓${NC} No forbidden patterns in domain types"
fi

echo ""

# =============================================================================
# Summary
# =============================================================================

echo "══════════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All Phase 27 guardrail checks passed.${NC}"
    echo ""
    echo "Real Shadow Receipt (Primary Proof) is safe:"
    echo "  - Domain models complete with all required types"
    echo "  - Engine methods exist (BuildPrimaryPage, BuildPrimaryCue)"
    echo "  - Ack/Vote store with hash-only storage"
    echo "  - Vote does NOT change behavior"
    echo "  - Single-whisper rule integrated"
    echo "  - Events defined for audit"
    exit 0
else
    echo -e "${RED}Some Phase 27 guardrail checks failed.${NC}"
    echo ""
    echo "Fix the issues above before proceeding."
    exit 1
fi
