#!/bin/bash
# Guardrails for Phase 25: First Undoable Execution (Opt-In, Single-Shot)
#
# CRITICAL INVARIANTS:
#   - ONLY calendar_respond is allowed (no email, no finance)
#   - Single-shot per period (max one execution per day)
#   - Undo window is bucketed time (15-minute buckets)
#   - Undo is first-class flow, not "best effort"
#   - No goroutines
#   - No time.Now() - clock injection only
#   - Hash-only storage (no identifiers)
#   - Reuses existing Phase 5 calendar execution boundary

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

pass_count=0
fail_count=0

check() {
    local name="$1"
    local condition="$2"

    if eval "$condition"; then
        echo -e "${GREEN}PASS${NC}: $name"
        pass_count=$((pass_count + 1))
    else
        echo -e "${RED}FAIL${NC}: $name"
        fail_count=$((fail_count + 1))
    fi
}

check_not() {
    local name="$1"
    local condition="$2"

    if ! eval "$condition"; then
        echo -e "${GREEN}PASS${NC}: $name"
        pass_count=$((pass_count + 1))
    else
        echo -e "${RED}FAIL${NC}: $name"
        fail_count=$((fail_count + 1))
    fi
}

cd "$PROJECT_ROOT"

echo "=== Phase 25: Undoable Execution Guardrails ==="
echo ""

# =============================================================================
# Domain Model Checks
# =============================================================================

echo "--- Domain Model Checks ---"

check "UndoableActionKind type exists" \
    "grep -q 'type UndoableActionKind string' pkg/domain/undoableexec/types.go"

check "Only calendar_respond action kind defined" \
    "grep -q 'ActionKindCalendarRespond.*calendar_respond' pkg/domain/undoableexec/types.go"

check_not "No email action kind defined" \
    "grep -q 'ActionKindEmail' pkg/domain/undoableexec/types.go"

check_not "No finance action kind defined" \
    "grep -q 'ActionKindFinance' pkg/domain/undoableexec/types.go"

check "UndoState type exists" \
    "grep -q 'type UndoState string' pkg/domain/undoableexec/types.go"

check "StateUndoAvailable exists" \
    "grep -q 'StateUndoAvailable.*undo_available' pkg/domain/undoableexec/types.go"

check "UndoWindow type exists" \
    "grep -q 'type UndoWindow struct' pkg/domain/undoableexec/types.go"

check "UndoWindow uses bucketed time" \
    "grep -q 'BucketStartRFC3339' pkg/domain/undoableexec/types.go"

check "UndoWindow has 15-minute buckets" \
    "grep -q 'BucketDurationMinutes: 15' pkg/domain/undoableexec/types.go"

check "UndoRecord type exists" \
    "grep -q 'type UndoRecord struct' pkg/domain/undoableexec/types.go"

check "UndoRecord has hash-only ID" \
    "grep -q 'ID.*string' pkg/domain/undoableexec/types.go"

check "UndoAck type exists" \
    "grep -q 'type UndoAck struct' pkg/domain/undoableexec/types.go"

# =============================================================================
# Engine Checks
# =============================================================================

echo ""
echo "--- Engine Checks ---"

check "Engine type exists" \
    "grep -q 'type Engine struct' internal/undoableexec/engine.go"

check "Engine uses clock injection" \
    "grep -q 'clock.*func().*time.Time' internal/undoableexec/engine.go"

check_not "No time.Now() in engine (excluding comments)" \
    "grep -v '^[[:space:]]*//' internal/undoableexec/engine.go | grep -E 'time\.Now\(\)'"

check_not "No goroutines in engine" \
    "grep -E '\bgo\s+func' internal/undoableexec/engine.go"

check "EligibleAction method exists" \
    "grep -q 'func.*Engine.*EligibleAction' internal/undoableexec/engine.go"

check "RunOnce method exists" \
    "grep -q 'func.*Engine.*RunOnce' internal/undoableexec/engine.go"

check "Undo method exists" \
    "grep -q 'func.*Engine.*Undo' internal/undoableexec/engine.go"

check "Single-shot enforcement: HasExecutedThisPeriod" \
    "grep -q 'HasExecutedThisPeriod' internal/undoableexec/engine.go"

check "Uses calendar execution boundary" \
    "grep -q 'calendarExecutor' internal/undoableexec/engine.go"

check "Uses calendar/execution package" \
    "grep -q 'calendar/execution' internal/undoableexec/engine.go"

# =============================================================================
# Persistence Checks
# =============================================================================

echo ""
echo "--- Persistence Checks ---"

check "UndoableExecStore type exists" \
    "grep -q 'type UndoableExecStore struct' internal/persist/undoable_exec_store.go"

check "Store uses clock injection" \
    "grep -q 'clock.*func().*time.Time' internal/persist/undoable_exec_store.go"

check_not "No time.Now() in store (excluding comments)" \
    "grep -v '^[[:space:]]*//' internal/persist/undoable_exec_store.go | grep -E 'time\.Now\(\)'"

check_not "No goroutines in store" \
    "grep -E '\bgo\s+func' internal/persist/undoable_exec_store.go"

check "AppendRecord method exists" \
    "grep -q 'func.*UndoableExecStore.*AppendRecord' internal/persist/undoable_exec_store.go"

check "AppendAck method exists" \
    "grep -q 'func.*UndoableExecStore.*AppendAck' internal/persist/undoable_exec_store.go"

check "Storelog record type for undo records" \
    "grep -q 'RecordTypeUndoExecRecord' pkg/domain/storelog/log.go"

check "Storelog record type for undo acks" \
    "grep -q 'RecordTypeUndoExecAck' pkg/domain/storelog/log.go"

check "Replay support exists" \
    "grep -q 'ReplayRecordFromStorelog' internal/persist/undoable_exec_store.go"

# =============================================================================
# Draft Integration Checks
# =============================================================================

echo ""
echo "--- Draft Integration Checks ---"

check "PreviousResponseStatus field exists in CalendarDraftContent" \
    "grep -q 'PreviousResponseStatus' pkg/domain/draft/types.go"

check "GetPreviousResponseStatus method exists" \
    "grep -q 'func.*CalendarDraftContent.*GetPreviousResponseStatus' pkg/domain/draft/types.go"

# =============================================================================
# Event Checks
# =============================================================================

echo ""
echo "--- Event Checks ---"

check "Phase 25 undoable viewed event" \
    "grep -q 'Phase25UndoableViewed' pkg/events/events.go"

check "Phase 25 run executed event" \
    "grep -q 'Phase25RunExecuted' pkg/events/events.go"

check "Phase 25 undo executed event" \
    "grep -q 'Phase25UndoExecuted' pkg/events/events.go"

check "Phase 25 record persisted event" \
    "grep -q 'Phase25RecordPersisted' pkg/events/events.go"

# =============================================================================
# Web Route Checks
# =============================================================================

echo ""
echo "--- Web Route Checks ---"

check "Undoable page route exists" \
    "grep -q '/action/undoable' cmd/quantumlife-web/main.go"

check "Undoable run route exists" \
    "grep -q '/action/undoable/run' cmd/quantumlife-web/main.go"

check "Undoable done route exists" \
    "grep -q '/action/undoable/done' cmd/quantumlife-web/main.go"

check "Undoable undo route exists" \
    "grep -q '/action/undoable/undo' cmd/quantumlife-web/main.go"

# =============================================================================
# Safety Checks - No Auto-Execution
# =============================================================================

echo ""
echo "--- Safety Checks ---"

check_not "No auto-run in undoable engine" \
    "grep -E 'AutoRun|autoRun|auto_run' internal/undoableexec/engine.go"

check_not "No background execution in undoable engine (excluding comments)" \
    "grep -v '^[[:space:]]*//' internal/undoableexec/engine.go | grep -E 'background|Background|BACKGROUND'"

check_not "No retry logic in undoable engine" \
    "grep -E 'retry|Retry|RETRY' internal/undoableexec/engine.go"

# =============================================================================
# Summary
# =============================================================================

echo ""
echo "=== Summary ==="
echo -e "Passed: ${GREEN}${pass_count}${NC}"
echo -e "Failed: ${RED}${fail_count}${NC}"

if [ "$fail_count" -gt 0 ]; then
    echo ""
    echo -e "${RED}Phase 25 guardrails FAILED${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}All Phase 25 guardrails passed!${NC}"
exit 0
