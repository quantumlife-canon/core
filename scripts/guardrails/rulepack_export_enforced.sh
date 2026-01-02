#!/bin/bash
# Phase 19.6: Rule Pack Export Guardrails
#
# Validates that Rule Pack Export implementation follows all invariants.
#
# CRITICAL: Rule Pack Export must:
#   - NOT apply itself - no policy mutation
#   - NOT affect behavior - no execution path changes
#   - Contain NO raw identifiers (emails, URLs, vendor names, currency)
#   - Be deterministic - same inputs + clock => same outputs
#   - Use pipe-delimited format - no JSON for canonical strings
#   - Use clock injection - no time.Now()
#   - No goroutines - synchronous only
#   - stdlib only - no external dependencies
#
# Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md

set -e

ERRORS=0

error() {
    echo "[ERROR] $1"
    ERRORS=$((ERRORS + 1))
}

check() {
    echo "[CHECK] $1"
}

pass() {
    echo "[PASS] $1"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Check 1: No goroutines in rulepack packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No goroutines in pkg/domain/rulepack"
if grep -r 'go func' pkg/domain/rulepack/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in pkg/domain/rulepack"
else
    pass "No goroutines in pkg/domain/rulepack"
fi

check "No goroutines in internal/rulepack"
if grep -r 'go func' internal/rulepack/ 2>/dev/null | grep -v '_test.go'; then
    error "Found goroutine in internal/rulepack"
else
    pass "No goroutines in internal/rulepack"
fi

check "No goroutines in rulepack_store.go"
if grep 'go func' internal/persist/rulepack_store.go 2>/dev/null; then
    error "Found goroutine in rulepack_store.go"
else
    pass "No goroutines in rulepack_store.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 2: No time.Now() in rulepack packages (must use clock injection)
# ═══════════════════════════════════════════════════════════════════════════════
check "No time.Now() in pkg/domain/rulepack"
if grep -rn 'time\.Now()' pkg/domain/rulepack/ 2>/dev/null | grep -v '_test.go' | grep -v '//'; then
    error "Found time.Now() in pkg/domain/rulepack - must use clock injection"
else
    pass "No time.Now() in pkg/domain/rulepack"
fi

check "No time.Now() in internal/rulepack"
if grep -rn 'time\.Now()' internal/rulepack/ 2>/dev/null | grep -v '_test.go' | grep -v '//'; then
    error "Found time.Now() in internal/rulepack - must use clock injection"
else
    pass "No time.Now() in internal/rulepack"
fi

check "No time.Now() in rulepack_store.go (nowFunc must be used)"
if grep -n 'time\.Now()' internal/persist/rulepack_store.go 2>/dev/null | grep -v '//' | grep -v 'nowFunc'; then
    error "Found time.Now() in rulepack_store.go - must use nowFunc"
else
    pass "No time.Now() in rulepack_store.go"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 3: No network calls in rulepack packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No net/http imports in pkg/domain/rulepack"
if grep -r '"net/http"' pkg/domain/rulepack/ 2>/dev/null | grep -v '_test.go'; then
    error "Found net/http import in pkg/domain/rulepack"
else
    pass "No net/http in pkg/domain/rulepack"
fi

check "No net/http imports in internal/rulepack"
if grep -r '"net/http"' internal/rulepack/ 2>/dev/null | grep -v '_test.go'; then
    error "Found net/http import in internal/rulepack"
else
    pass "No net/http in internal/rulepack"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 4: Pipe-delimited canonical format (no JSON)
# ═══════════════════════════════════════════════════════════════════════════════
check "CanonicalString uses pipe delimiter in types.go"
if ! grep -q 'RULE_PACK|' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "RulePack.CanonicalString must use pipe-delimited format"
else
    pass "RulePack.CanonicalString uses pipe delimiter"
fi

check "CanonicalString uses pipe delimiter for RuleChange"
if ! grep -q 'RULE_CHANGE|' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "RuleChange.CanonicalString must use pipe-delimited format"
else
    pass "RuleChange.CanonicalString uses pipe delimiter"
fi

check "CanonicalString uses pipe delimiter for PackAck"
if ! grep -q 'PACK_ACK|' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "PackAck.CanonicalString must use pipe-delimited format"
else
    pass "PackAck.CanonicalString uses pipe delimiter"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 5: Privacy - forbidden patterns blocked
# ═══════════════════════════════════════════════════════════════════════════════
check "ForbiddenPatterns includes @ (email addresses)"
if ! grep -q '"@"' pkg/domain/rulepack/export.go 2>/dev/null; then
    error "ForbiddenPatterns must include @ to block email addresses"
else
    pass "ForbiddenPatterns blocks email addresses"
fi

check "ForbiddenPatterns includes http:// (URLs)"
if ! grep -q '"http://"' pkg/domain/rulepack/export.go 2>/dev/null; then
    error "ForbiddenPatterns must include http:// to block URLs"
else
    pass "ForbiddenPatterns blocks URLs"
fi

check "ForbiddenPatterns includes $ (currency)"
if ! grep -q '"\$"' pkg/domain/rulepack/export.go 2>/dev/null; then
    error "ForbiddenPatterns must include $ to block currency"
else
    pass "ForbiddenPatterns blocks currency"
fi

check "ForbiddenPatterns includes vendor names"
if ! grep -q '"amazon"' pkg/domain/rulepack/export.go 2>/dev/null; then
    error "ForbiddenPatterns must include common vendor names"
else
    pass "ForbiddenPatterns blocks vendor names"
fi

check "ValidateExportPrivacy function exists"
if ! grep -q 'func ValidateExportPrivacy' pkg/domain/rulepack/export.go 2>/dev/null; then
    error "ValidateExportPrivacy function must exist"
else
    pass "ValidateExportPrivacy function exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 6: No policy mutation
# ═══════════════════════════════════════════════════════════════════════════════
check "No imports from rulepack into policy package"
if grep -r 'quantumlife/pkg/domain/rulepack' pkg/domain/policy/ 2>/dev/null | grep -v '_test.go'; then
    error "Found rulepack import in policy package - RulePack must NOT apply itself"
else
    pass "No rulepack imports in policy package"
fi

check "No imports from rulepack into obligations package"
if grep -r 'quantumlife/pkg/domain/rulepack' internal/obligations/ 2>/dev/null | grep -v '_test.go'; then
    error "Found rulepack import in obligations package - RulePack must NOT affect behavior"
else
    pass "No rulepack imports in obligations package"
fi

check "No imports from rulepack into interrupts package"
if grep -r 'quantumlife/pkg/domain/rulepack' internal/interrupts/ 2>/dev/null | grep -v '_test.go'; then
    error "Found rulepack import in interrupts package - RulePack must NOT affect behavior"
else
    pass "No rulepack imports in interrupts package"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 7: Gating thresholds documented
# ═══════════════════════════════════════════════════════════════════════════════
check "MinUsefulnessBucket constant exists"
if ! grep -q 'MinUsefulnessBucket' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "MinUsefulnessBucket constant must exist for gating threshold"
else
    pass "MinUsefulnessBucket constant exists"
fi

check "MinVoteCount constant exists"
if ! grep -q 'MinVoteCount' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "MinVoteCount constant must exist for gating threshold"
else
    pass "MinVoteCount constant exists"
fi

check "MinVoteConfidenceBucket constant exists"
if ! grep -q 'MinVoteConfidenceBucket' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "MinVoteConfidenceBucket constant must exist for gating threshold"
else
    pass "MinVoteConfidenceBucket constant exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 8: Export format version
# ═══════════════════════════════════════════════════════════════════════════════
check "ExportFormatVersion constant exists"
if ! grep -q 'ExportFormatVersion = "v1"' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "ExportFormatVersion must be defined as v1"
else
    pass "ExportFormatVersion is v1"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 9: Engine uses clock injection
# ═══════════════════════════════════════════════════════════════════════════════
check "Engine uses clock.Clock"
if ! grep -q 'clock\.Clock' internal/rulepack/engine.go 2>/dev/null; then
    error "Engine must use clock.Clock for determinism"
else
    pass "Engine uses clock.Clock"
fi

check "Engine has clk field"
if ! grep -q 'clk clock\.Clock' internal/rulepack/engine.go 2>/dev/null; then
    error "Engine must have clk field for clock injection"
else
    pass "Engine has clk field"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 10: Deterministic sorting
# ═══════════════════════════════════════════════════════════════════════════════
check "SortRuleChanges function exists"
if ! grep -q 'func SortRuleChanges' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "SortRuleChanges function must exist for deterministic output"
else
    pass "SortRuleChanges function exists"
fi

check "SortRulePacks function exists"
if ! grep -q 'func SortRulePacks' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "SortRulePacks function must exist for deterministic output"
else
    pass "SortRulePacks function exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 11: Storelog integration
# ═══════════════════════════════════════════════════════════════════════════════
check "RulePackStore uses storelog record types"
if ! grep -q 'RecordTypeRulePackExported' internal/persist/rulepack_store.go 2>/dev/null; then
    error "RulePackStore must define RecordTypeRulePackExported"
else
    pass "RecordTypeRulePackExported exists"
fi

check "Replay support exists"
if ! grep -q 'ReplayPackRecord' internal/persist/rulepack_store.go 2>/dev/null; then
    error "ReplayPackRecord function must exist for persistence"
else
    pass "ReplayPackRecord function exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 12: Phase 19.6 events defined
# ═══════════════════════════════════════════════════════════════════════════════
check "Phase 19.6 events defined in events.go"
if ! grep -q 'Phase19_6' pkg/events/events.go 2>/dev/null; then
    error "Phase 19.6 events must be defined in events.go"
else
    pass "Phase 19.6 events defined"
fi

check "Pack build event exists"
if ! grep -q 'phase19_6.pack.built' pkg/events/events.go 2>/dev/null; then
    error "phase19_6.pack.built event must be defined"
else
    pass "Pack build event exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 13: Ack kinds defined
# ═══════════════════════════════════════════════════════════════════════════════
check "AckKind type exists"
if ! grep -q 'type AckKind string' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "AckKind type must be defined"
else
    pass "AckKind type exists"
fi

check "AckViewed constant exists"
if ! grep -q 'AckViewed' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "AckViewed constant must be defined"
else
    pass "AckViewed constant exists"
fi

check "AckExported constant exists"
if ! grep -q 'AckExported' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "AckExported constant must be defined"
else
    pass "AckExported constant exists"
fi

check "AckDismissed constant exists"
if ! grep -q 'AckDismissed' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "AckDismissed constant must be defined"
else
    pass "AckDismissed constant exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 14: Change kinds defined
# ═══════════════════════════════════════════════════════════════════════════════
check "ChangeBiasAdjust constant exists"
if ! grep -q 'ChangeBiasAdjust' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "ChangeBiasAdjust constant must be defined"
else
    pass "ChangeBiasAdjust constant exists"
fi

check "ChangeThresholdAdjust constant exists"
if ! grep -q 'ChangeThresholdAdjust' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "ChangeThresholdAdjust constant must be defined"
else
    pass "ChangeThresholdAdjust constant exists"
fi

check "ChangeSuppressSuggest constant exists"
if ! grep -q 'ChangeSuppressSuggest' pkg/domain/rulepack/types.go 2>/dev/null; then
    error "ChangeSuppressSuggest constant must be defined"
else
    pass "ChangeSuppressSuggest constant exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 15: No external dependencies in rulepack packages
# ═══════════════════════════════════════════════════════════════════════════════
check "No external imports in pkg/domain/rulepack"
if grep -r 'github.com' pkg/domain/rulepack/ 2>/dev/null | grep -v '_test.go' | grep -v 'quantumlife'; then
    error "Found external imports in pkg/domain/rulepack - stdlib only"
else
    pass "No external imports in pkg/domain/rulepack"
fi

check "No external imports in internal/rulepack"
if grep -r 'github.com' internal/rulepack/ 2>/dev/null | grep -v '_test.go' | grep -v 'quantumlife'; then
    error "Found external imports in internal/rulepack - stdlib only"
else
    pass "No external imports in internal/rulepack"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Check 16: Demo tests exist
# ═══════════════════════════════════════════════════════════════════════════════
check "Demo tests exist for Phase 19.6"
if [ ! -f "internal/demo_phase19_6_rulepack_export/demo_test.go" ]; then
    error "Demo tests must exist at internal/demo_phase19_6_rulepack_export/demo_test.go"
else
    pass "Demo tests exist"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"
if [ $ERRORS -eq 0 ]; then
    echo "[SUCCESS] All Phase 19.6 guardrails passed!"
    echo "═══════════════════════════════════════════════════════════════════════════════"
    exit 0
else
    echo "[FAILURE] $ERRORS guardrail check(s) failed"
    echo "═══════════════════════════════════════════════════════════════════════════════"
    exit 1
fi
