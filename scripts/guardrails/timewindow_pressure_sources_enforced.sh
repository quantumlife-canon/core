#!/bin/bash
# Phase 40: Time-Window Pressure Sources Guardrails
# CRITICAL: Enforces canon invariants for Phase 40.
# Reference: docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md

set -e

PASS=0
FAIL=0

pass() {
    echo "  ✓ $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "  ✗ $1"
    FAIL=$((FAIL + 1))
}

check() {
    if eval "$2"; then
        pass "$1"
    else
        fail "$1"
    fi
}

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "Phase 40: Time-Window Pressure Sources Guardrails"
echo "═══════════════════════════════════════════════════════════════════════════════"
echo ""

# ============================================================================
# Section 1: Package Structure
# ============================================================================
echo "--- Section 1: Package Structure ---"

check "ADR exists" "[ -f docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md ]"
check "Domain types exist" "[ -f pkg/domain/timewindow/types.go ]"
check "Engine exists" "[ -f internal/timewindow/engine.go ]"
check "Persist store exists" "[ -f internal/persist/timewindow_store.go ]"
check "Demo tests exist" "[ -f internal/demo_phase40_timewindow_sources/demo_test.go ]"
check "Package declaration in domain" "grep -q 'package timewindow' pkg/domain/timewindow/types.go"
check "Package declaration in engine" "grep -q 'package timewindow' internal/timewindow/engine.go"
check "ADR reference in domain types" "grep -q 'ADR-0077' pkg/domain/timewindow/types.go"
check "ADR reference in engine" "grep -q 'ADR-0077' internal/timewindow/engine.go"
check "ADR reference in store" "grep -q 'ADR-0077' internal/persist/timewindow_store.go"
echo ""

# ============================================================================
# Section 2: No Goroutines
# ============================================================================
echo "--- Section 2: No Goroutines ---"

check "No goroutines in domain types" "! grep -q 'go func' pkg/domain/timewindow/types.go"
check "No goroutines in engine" "! grep -q 'go func' internal/timewindow/engine.go"
check "No goroutines in store" "! grep -q 'go func' internal/persist/timewindow_store.go"
check "No channel creation in engine" "! grep -q 'make(chan' internal/timewindow/engine.go"
check "No channel creation in store" "! grep -q 'make(chan' internal/persist/timewindow_store.go"
echo ""

# ============================================================================
# Section 3: Clock Injection
# ============================================================================
echo "--- Section 3: Clock Injection ---"

check "No time.Now() in domain types" "! grep -v '^[[:space:]]*//' pkg/domain/timewindow/types.go | grep -q 'time\.Now()'"
check "No time.Now() in engine" "! grep -q 'time\.Now()' internal/timewindow/engine.go"
check "Clock parameter in BuildSignals" "grep -q 'BuildSignals.*clock time.Time' internal/timewindow/engine.go"
check "Clock parameter in store EvictOldPeriods" "grep -q 'EvictOldPeriods.*now time.Time' internal/persist/timewindow_store.go"
check "Clock parameter in store GetOverallMagnitude" "grep -q 'GetOverallMagnitude.*now time.Time' internal/persist/timewindow_store.go"
echo ""

# ============================================================================
# Section 4: stdlib only
# ============================================================================
echo "--- Section 4: stdlib only ---"

check "No external imports in domain" "! grep -E 'github\.com|gopkg\.in|golang\.org/x' pkg/domain/timewindow/types.go"
check "No external imports in engine" "! grep -E 'github\.com|gopkg\.in|golang\.org/x' internal/timewindow/engine.go"
check "No external imports in store" "! grep -E 'github\.com|gopkg\.in|golang\.org/x' internal/persist/timewindow_store.go"
check "No cloud SDK in engine" "! grep -q 'cloud.google.com\|amazonaws' internal/timewindow/engine.go"
check "No cloud SDK in store" "! grep -q 'cloud.google.com\|amazonaws' internal/persist/timewindow_store.go"
echo ""

# ============================================================================
# Section 5: No Execution/Transport/Network Imports
# ============================================================================
echo "--- Section 5: No Execution/Transport/Network Imports ---"

check "No net/http in engine" "! grep -q '\"net/http\"' internal/timewindow/engine.go"
check "No notification imports in engine" "! grep -q 'notification' internal/timewindow/engine.go"
check "No push imports in engine" "! grep -q '/push' internal/timewindow/engine.go"
check "No execution imports in engine" "! grep -q '/execution' internal/timewindow/engine.go"
check "No delivery imports in engine" "! grep -q '/delivery' internal/timewindow/engine.go"
check "No transport imports in engine" "! grep -q '/transport' internal/timewindow/engine.go"
echo ""

# ============================================================================
# Section 6: Enum-Only Types
# ============================================================================
echo "--- Section 6: Enum-Only Types ---"

check "WindowSourceKind enum exists" "grep -q 'type WindowSourceKind string' pkg/domain/timewindow/types.go"
check "WindowKind enum exists" "grep -q 'type WindowKind string' pkg/domain/timewindow/types.go"
check "WindowReasonBucket enum exists" "grep -q 'type WindowReasonBucket string' pkg/domain/timewindow/types.go"
check "WindowCircleType enum exists" "grep -q 'type WindowCircleType string' pkg/domain/timewindow/types.go"
check "WindowMagnitudeBucket enum exists" "grep -q 'type WindowMagnitudeBucket string' pkg/domain/timewindow/types.go"
check "WindowEvidenceKind enum exists" "grep -q 'type WindowEvidenceKind string' pkg/domain/timewindow/types.go"
check "Validate method on WindowSourceKind" "grep -q 'func (k WindowSourceKind) Validate()' pkg/domain/timewindow/types.go"
check "Validate method on WindowKind" "grep -q 'func (k WindowKind) Validate()' pkg/domain/timewindow/types.go"
check "Validate method on WindowReasonBucket" "grep -q 'func (r WindowReasonBucket) Validate()' pkg/domain/timewindow/types.go"
check "CanonicalString on WindowSourceKind" "grep -q 'func (k WindowSourceKind) CanonicalString()' pkg/domain/timewindow/types.go"
echo ""

# ============================================================================
# Section 7: Domain Model Completeness
# ============================================================================
echo "--- Section 7: Domain Model Completeness ---"

check "SourceCalendar defined" "grep -q 'SourceCalendar.*source_calendar' pkg/domain/timewindow/types.go"
check "SourceInboxInstitution defined" "grep -q 'SourceInboxInstitution.*source_inbox_institution' pkg/domain/timewindow/types.go"
check "SourceInboxHuman defined" "grep -q 'SourceInboxHuman.*source_inbox_human' pkg/domain/timewindow/types.go"
check "SourceDeviceHint defined" "grep -q 'SourceDeviceHint.*source_device_hint' pkg/domain/timewindow/types.go"
check "WindowNow defined" "grep -q 'WindowNow.*window_now' pkg/domain/timewindow/types.go"
check "WindowSoon defined" "grep -q 'WindowSoon.*window_soon' pkg/domain/timewindow/types.go"
check "WindowToday defined" "grep -q 'WindowToday.*window_today' pkg/domain/timewindow/types.go"
check "WindowLater defined" "grep -q 'WindowLater.*window_later' pkg/domain/timewindow/types.go"
check "TimeWindowSignal struct exists" "grep -q 'type TimeWindowSignal struct' pkg/domain/timewindow/types.go"
check "TimeWindowBuildResult struct exists" "grep -q 'type TimeWindowBuildResult struct' pkg/domain/timewindow/types.go"
echo ""

# ============================================================================
# Section 8: Bounded Effects
# ============================================================================
echo "--- Section 8: Bounded Effects ---"

check "ShiftEarlier function exists" "grep -q 'func (k WindowKind) ShiftEarlier()' pkg/domain/timewindow/types.go"
check "IncrementMagnitude function exists" "grep -q 'func (m WindowMagnitudeBucket) IncrementMagnitude()' pkg/domain/timewindow/types.go"
check "applyEnvelopeShift in engine" "grep -q 'applyEnvelopeShift' internal/timewindow/engine.go"
check "WindowNow stays WindowNow on shift" "grep -q 'return k.*WindowNow stays' pkg/domain/timewindow/types.go"
check "MagnitudeSeveral stays several" "grep -q 'return MagnitudeSeveral' pkg/domain/timewindow/types.go"
check "Max 1 step shift comment" "grep -q 'Max 1 step' pkg/domain/timewindow/types.go"
check "Envelope shift bounded comment in engine" "grep -q 'Max 1 step' internal/timewindow/engine.go"
check "Max signals constant" "grep -q 'MaxSignals.*=.*3' pkg/domain/timewindow/types.go"
check "Max evidence hashes constant" "grep -q 'MaxEvidenceHashes.*=.*3' pkg/domain/timewindow/types.go"
check "capEvidenceHashes in engine" "grep -q 'capEvidenceHashes' internal/timewindow/engine.go"
echo ""

# ============================================================================
# Section 9: Commerce Exclusion
# ============================================================================
echo "--- Section 9: Commerce Exclusion ---"

check "Commerce comment in engine" "grep -q 'Commerce.*excluded\|Commerce MUST NOT' internal/timewindow/engine.go"
check "No commerce source type" "! grep -q 'source_commerce' pkg/domain/timewindow/types.go"
check "ADR mentions commerce exclusion" "grep -q 'commerce\|Commerce' docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md"
check "Engine collectCandidates excludes commerce" "grep -q 'Commerce is excluded' internal/timewindow/engine.go"
check "No CircleTypeCommerce in engine" "! grep -q 'CircleCommerce' internal/timewindow/engine.go"
echo ""

# ============================================================================
# Section 10: Deterministic Sorting
# ============================================================================
echo "--- Section 10: Deterministic Sorting ---"

check "Sort import in types" "grep -q '\"sort\"' pkg/domain/timewindow/types.go"
check "Sort import in engine" "grep -q '\"sort\"' internal/timewindow/engine.go"
check "Sort.Strings in types" "grep -q 'sort.Strings' pkg/domain/timewindow/types.go"
check "Sort.SliceStable in engine" "grep -q 'sort.SliceStable' internal/timewindow/engine.go"
check "Precedence function for sorting" "grep -q 'func (k WindowSourceKind) Precedence()' pkg/domain/timewindow/types.go"
check "StatusHash deterministic selection" "grep -q 'StatusHash.*deterministic\|lowest.*StatusHash' internal/timewindow/engine.go"
echo ""

# ============================================================================
# Section 11: Storage Constraints
# ============================================================================
echo "--- Section 11: Storage Constraints ---"

check "MaxRecords constant" "grep -q 'MaxRecords.*=.*500' pkg/domain/timewindow/types.go"
check "MaxRetentionDays constant" "grep -q 'MaxRetentionDays.*=.*30' pkg/domain/timewindow/types.go"
check "FIFO eviction in store" "grep -q 'evictExcessRecordsLocked\|FIFO' internal/persist/timewindow_store.go"
check "Dedupe by composite key" "grep -q 'makeKey\|circle_id_hash.*period_key.*result_hash' internal/persist/timewindow_store.go"
check "Hash-only storage comment" "grep -q 'Hash-only' internal/persist/timewindow_store.go"
echo ""

# ============================================================================
# Section 12: Storelog Integration
# ============================================================================
echo "--- Section 12: Storelog Integration ---"

check "RecordTypeTimeWindowSignal in storelog" "grep -q 'RecordTypeTimeWindowSignal' pkg/domain/storelog/log.go"
check "RecordTypeTimeWindowResult in storelog" "grep -q 'RecordTypeTimeWindowResult' pkg/domain/storelog/log.go"
check "Storelog integration in store" "grep -q 'storelog\|storelogRef' internal/persist/timewindow_store.go"
check "Store writes to storelog" "grep -q 'storelogRef.Append' internal/persist/timewindow_store.go"
check "Store imports storelog" "grep -q 'quantumlife/pkg/domain/storelog' internal/persist/timewindow_store.go"
echo ""

# ============================================================================
# Section 13: Events
# ============================================================================
echo "--- Section 13: Events ---"

check "Phase40WindowsBuildRequested event" "grep -q 'Phase40WindowsBuildRequested' pkg/events/events.go"
check "Phase40WindowsBuilt event" "grep -q 'Phase40WindowsBuilt' pkg/events/events.go"
check "Phase40WindowsPersisted event" "grep -q 'Phase40WindowsPersisted' pkg/events/events.go"
check "Phase40WindowsViewed event" "grep -q 'Phase40WindowsViewed' pkg/events/events.go"
check "Phase40WindowsCueDismissed event" "grep -q 'Phase40WindowsCueDismissed' pkg/events/events.go"
echo ""

# ============================================================================
# Section 14: Engine Requirements
# ============================================================================
echo "--- Section 14: Engine Requirements ---"

check "NewEngine function" "grep -q 'func NewEngine()' internal/timewindow/engine.go"
check "BuildSignals function" "grep -q 'func.*BuildSignals' internal/timewindow/engine.go"
check "SignalToPressureInput function" "grep -q 'func.*SignalToPressureInput' internal/timewindow/engine.go"
check "BuildProofPage function" "grep -q 'func.*BuildProofPage' internal/timewindow/engine.go"
check "GetCalmWhisperCue function" "grep -q 'func.*GetCalmWhisperCue' internal/timewindow/engine.go"
check "selectFinalSignals function" "grep -q 'selectFinalSignals' internal/timewindow/engine.go"
check "collectCandidates function" "grep -q 'collectCandidates' internal/timewindow/engine.go"
echo ""

# ============================================================================
# Section 15: Canonical Strings
# ============================================================================
echo "--- Section 15: Canonical Strings ---"

check "CanonicalString for TimeWindowSignal" "grep -q 'func (s \*TimeWindowSignal) CanonicalString()' pkg/domain/timewindow/types.go"
check "CanonicalString for TimeWindowBuildResult" "grep -q 'func (r \*TimeWindowBuildResult) CanonicalString()' pkg/domain/timewindow/types.go"
check "ComputeStatusHash for signal" "grep -q 'func (s \*TimeWindowSignal) ComputeStatusHash()' pkg/domain/timewindow/types.go"
check "ComputeResultHash for result" "grep -q 'func (r \*TimeWindowBuildResult) ComputeResultHash()' pkg/domain/timewindow/types.go"
check "Pipe-delimited format" "grep -q 'WINDOW_SIGNAL|v1' pkg/domain/timewindow/types.go"
check "SHA256 hashing" "grep -q 'sha256.Sum256' pkg/domain/timewindow/types.go"
echo ""

# ============================================================================
# Section 16: No Raw Data
# ============================================================================
echo "--- Section 16: No Raw Data (Privacy) ---"

check "No timestamps comment in types" "grep -q 'NO raw timestamps\|No raw timestamps' pkg/domain/timewindow/types.go"
check "No email addresses comment" "grep -q 'NO email addresses\|No email' pkg/domain/timewindow/types.go"
check "No merchant strings comment" "grep -q 'NO merchant\|No merchant' pkg/domain/timewindow/types.go"
check "Observation only comment" "grep -q 'OBSERVATION ONLY\|Observation ONLY' pkg/domain/timewindow/types.go"
check "Cannot deliver comment" "grep -q 'cannot deliver' pkg/domain/timewindow/types.go"
echo ""

# ============================================================================
# Section 17: Web Routes
# ============================================================================
echo "--- Section 17: Web Routes ---"

check "GET /reality/windows route" "grep -q '/reality/windows.*handleTimeWindows' cmd/quantumlife-web/main.go"
check "POST /reality/windows/run route" "grep -q '/reality/windows/run.*handleTimeWindowsRun' cmd/quantumlife-web/main.go"
check "handleTimeWindows handler exists" "grep -q 'func.*handleTimeWindows' cmd/quantumlife-web/main.go"
check "handleTimeWindowsRun handler exists" "grep -q 'func.*handleTimeWindowsRun' cmd/quantumlife-web/main.go"
check "Calm UI comment in handler" "grep -qE 'calm|Calm|OBSERVATION ONLY' cmd/quantumlife-web/main.go && grep -q 'Phase 40' cmd/quantumlife-web/main.go"
echo ""

# ============================================================================
# Section 18: Proof Page
# ============================================================================
echo "--- Section 18: Proof Page ---"

check "WindowsProofPage struct exists" "grep -q 'type WindowsProofPage struct' pkg/domain/timewindow/types.go"
check "BuildWindowsProofPage function" "grep -q 'func BuildWindowsProofPage' pkg/domain/timewindow/types.go"
check "GetOverallMagnitude function" "grep -q 'func (r \*TimeWindowBuildResult) GetOverallMagnitude()' pkg/domain/timewindow/types.go"
check "GetSourceChips function" "grep -q 'func (r \*TimeWindowBuildResult) GetSourceChips()' pkg/domain/timewindow/types.go"
check "No details stored text" "grep -q 'No details.*stored' cmd/quantumlife-web/main.go"
echo ""

echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"
echo "Summary: $PASS passed, $FAIL failed, $((PASS + FAIL)) total"
echo "═══════════════════════════════════════════════════════════════════════════════"

if [ $FAIL -gt 0 ]; then
    exit 1
fi
