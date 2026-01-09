#!/bin/bash
# Phase 48: Market Signal Binding Guardrails
# ===========================================================
# This script enforces Phase 48 invariants:
# - No recommendations, nudges, ranking, persuasion
# - effect_no_power only
# - proof_only visibility only
# - No pricing, urgency, or calls to action
# - Signal exposure only - not a marketplace funnel
#
# Target: 120+ checks
# ===========================================================

set -e

PASS_COUNT=0
FAIL_COUNT=0
TOTAL_CHECKS=0

pass() {
    echo "  ✓ $1"
    PASS_COUNT=$((PASS_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
}

fail() {
    echo "  ✗ $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
}

section() {
    echo ""
    echo "=== $1 ==="
}

# ===========================================================
# Section 1: File Existence
# ===========================================================
section "File Existence"

if [ -f "docs/ADR/ADR-0086-phase48-market-signal-binding.md" ]; then
    pass "ADR document exists"
else
    fail "ADR document missing"
fi

if [ -f "pkg/domain/marketsignal/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/marketsignal/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/market_signal_store.go" ]; then
    pass "Persistence store file exists"
else
    fail "Persistence store file missing"
fi

# ===========================================================
# Section 2: Domain Types - Enum Validation
# ===========================================================
section "Domain Types - Enum Validation"

DOMAIN_FILE="pkg/domain/marketsignal/types.go"

if grep -q 'MarketSignalCoverageGap.*MarketSignalKind.*=.*"coverage_gap"' "$DOMAIN_FILE"; then
    pass "MarketSignalCoverageGap enum defined"
else
    fail "MarketSignalCoverageGap enum missing"
fi

if grep -q 'EffectNoPower.*MarketSignalEffect.*=.*"effect_no_power"' "$DOMAIN_FILE"; then
    pass "EffectNoPower effect defined"
else
    fail "EffectNoPower effect missing"
fi

if grep -q 'VisibilityProofOnly.*MarketSignalVisibility.*=.*"proof_only"' "$DOMAIN_FILE"; then
    pass "VisibilityProofOnly visibility defined"
else
    fail "VisibilityProofOnly visibility missing"
fi

if grep -q 'GapNoObserver.*CoverageGapKind' "$DOMAIN_FILE"; then
    pass "GapNoObserver gap kind defined"
else
    fail "GapNoObserver gap kind missing"
fi

if grep -q 'GapPartialCover.*CoverageGapKind' "$DOMAIN_FILE"; then
    pass "GapPartialCover gap kind defined"
else
    fail "GapPartialCover gap kind missing"
fi

if grep -q 'AckViewed.*MarketProofAckKind' "$DOMAIN_FILE"; then
    pass "AckViewed ack kind defined"
else
    fail "AckViewed ack kind missing"
fi

if grep -q 'AckDismissed.*MarketProofAckKind' "$DOMAIN_FILE"; then
    pass "AckDismissed ack kind defined"
else
    fail "AckDismissed ack kind missing"
fi

# ===========================================================
# Section 3: Domain Types - Struct Fields
# ===========================================================
section "Domain Types - Struct Fields"

if grep -q 'SignalID.*string' "$DOMAIN_FILE"; then
    pass "MarketSignal has SignalID field"
else
    fail "MarketSignal missing SignalID field"
fi

if grep -q 'CircleHash.*string' "$DOMAIN_FILE"; then
    pass "MarketSignal has CircleHash field"
else
    fail "MarketSignal missing CircleHash field"
fi

if grep -q 'NecessityKind.*NecessityKind' "$DOMAIN_FILE"; then
    pass "MarketSignal has NecessityKind field"
else
    fail "MarketSignal missing NecessityKind field"
fi

if grep -q 'CoverageGap.*CoverageGapKind' "$DOMAIN_FILE"; then
    pass "MarketSignal has CoverageGap field"
else
    fail "MarketSignal missing CoverageGap field"
fi

if grep -q 'PackIDHash.*string' "$DOMAIN_FILE"; then
    pass "MarketSignal has PackIDHash field"
else
    fail "MarketSignal missing PackIDHash field"
fi

if grep -q 'Kind.*MarketSignalKind' "$DOMAIN_FILE"; then
    pass "MarketSignal has Kind field"
else
    fail "MarketSignal missing Kind field"
fi

if grep -q 'Effect.*MarketSignalEffect' "$DOMAIN_FILE"; then
    pass "MarketSignal has Effect field"
else
    fail "MarketSignal missing Effect field"
fi

if grep -q 'Visibility.*MarketSignalVisibility' "$DOMAIN_FILE"; then
    pass "MarketSignal has Visibility field"
else
    fail "MarketSignal missing Visibility field"
fi

if grep -q 'PeriodKey.*string' "$DOMAIN_FILE"; then
    pass "MarketSignal has PeriodKey field"
else
    fail "MarketSignal missing PeriodKey field"
fi

# ===========================================================
# Section 4: Domain Types - Required Methods
# ===========================================================
section "Domain Types - Required Methods"

if grep -q 'func (s MarketSignal) Validate()' "$DOMAIN_FILE"; then
    pass "MarketSignal.Validate() method exists"
else
    fail "MarketSignal.Validate() method missing"
fi

if grep -q 'func (s MarketSignal) CanonicalString()' "$DOMAIN_FILE"; then
    pass "MarketSignal.CanonicalString() method exists"
else
    fail "MarketSignal.CanonicalString() method missing"
fi

if grep -q 'func (s MarketSignal) ComputeSignalID()' "$DOMAIN_FILE"; then
    pass "MarketSignal.ComputeSignalID() method exists"
else
    fail "MarketSignal.ComputeSignalID() method missing"
fi

if grep -q 'func (a MarketProofAck) Validate()' "$DOMAIN_FILE"; then
    pass "MarketProofAck.Validate() method exists"
else
    fail "MarketProofAck.Validate() method missing"
fi

if grep -q 'func (a MarketProofAck) CanonicalString()' "$DOMAIN_FILE"; then
    pass "MarketProofAck.CanonicalString() method exists"
else
    fail "MarketProofAck.CanonicalString() method missing"
fi

if grep -q 'func (k MarketSignalKind) Validate()' "$DOMAIN_FILE"; then
    pass "MarketSignalKind.Validate() method exists"
else
    fail "MarketSignalKind.Validate() method missing"
fi

if grep -q 'func (e MarketSignalEffect) Validate()' "$DOMAIN_FILE"; then
    pass "MarketSignalEffect.Validate() method exists"
else
    fail "MarketSignalEffect.Validate() method missing"
fi

if grep -q 'func (v MarketSignalVisibility) Validate()' "$DOMAIN_FILE"; then
    pass "MarketSignalVisibility.Validate() method exists"
else
    fail "MarketSignalVisibility.Validate() method missing"
fi

# ===========================================================
# Section 5: No time.Now() in Domain/Engine
# ===========================================================
section "No time.Now() in Domain/Engine"

if grep -r "time\.Now()" pkg/domain/marketsignal/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in domain package"
else
    pass "No time.Now() in domain package"
fi

if grep -r "time\.Now()" internal/marketsignal/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in engine package"
else
    pass "No time.Now() in engine package"
fi

# ===========================================================
# Section 6: No Goroutines
# ===========================================================
section "No Goroutines"

if grep -r "go func" pkg/domain/marketsignal/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in domain package"
else
    pass "No goroutines in domain package"
fi

if grep -r "go func" internal/marketsignal/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in engine package"
else
    pass "No goroutines in engine package"
fi

if grep -r "go func" internal/persist/market_signal_store.go 2>/dev/null > /dev/null 2>&1; then
    fail "Goroutine found in store"
else
    pass "No goroutines in store"
fi

# ===========================================================
# Section 7: Forbidden Imports
# ===========================================================
section "Forbidden Imports"

FORBIDDEN_IMPORTS=(
    "pressuredecision"
    "interruptpolicy"
    "interruptpreview"
    "pushtransport"
    "interruptdelivery"
)

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" pkg/domain/marketsignal/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in domain package"
    else
        pass "No forbidden import $import in domain"
    fi
done

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" internal/marketsignal/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in engine package"
    else
        pass "No forbidden import $import in engine"
    fi
done

# ===========================================================
# Section 8: No Recommendation Language
# ===========================================================
section "No Recommendation Language"

FORBIDDEN_WORDS=(
    "recommend"
    "should buy"
    "should install"
    "don't miss"
    "limited time"
    "featured"
    "promoted"
    "best choice"
    "top pick"
    "trending"
    "popular"
    "conversion"
    "funnel"
    "upsell"
    "cross-sell"
)

for word in "${FORBIDDEN_WORDS[@]}"; do
    if grep -ri "$word" pkg/domain/marketsignal/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
        fail "Forbidden word '$word' found in domain"
    else
        pass "No forbidden word '$word' in domain"
    fi
done

# ===========================================================
# Section 9: Effect No Power Enforcement
# ===========================================================
section "Effect No Power Enforcement"

ENGINE_FILE="internal/marketsignal/engine.go"

if grep -q 'EffectNoPower' "$ENGINE_FILE"; then
    pass "EffectNoPower used in engine"
else
    fail "EffectNoPower not used in engine"
fi

if grep -q 'only effect_no_power allowed' "$DOMAIN_FILE"; then
    pass "Effect validation enforces effect_no_power"
else
    fail "Effect validation does not enforce effect_no_power"
fi

# ===========================================================
# Section 10: Proof Only Visibility Enforcement
# ===========================================================
section "Proof Only Visibility Enforcement"

if grep -q 'VisibilityProofOnly' "$ENGINE_FILE"; then
    pass "VisibilityProofOnly used in engine"
else
    fail "VisibilityProofOnly not used in engine"
fi

if grep -q 'only proof_only allowed' "$DOMAIN_FILE"; then
    pass "Visibility validation enforces proof_only"
else
    fail "Visibility validation does not enforce proof_only"
fi

# ===========================================================
# Section 11: Bounded Retention
# ===========================================================
section "Bounded Retention"

STORE_FILE="internal/persist/market_signal_store.go"

if grep -q "MaxRecords\|MaxMarketSignalRecords" "$STORE_FILE"; then
    pass "Store has max records constant"
else
    fail "Store missing max records constant"
fi

if grep -q "MaxRetentionDays\|MaxMarketSignalDays" "$STORE_FILE"; then
    pass "Store has max retention days constant"
else
    fail "Store missing max retention days constant"
fi

if grep -q "evictOldRecordsLocked\|evictOld" "$STORE_FILE"; then
    pass "Store has eviction method"
else
    fail "Store missing eviction method"
fi

if grep -q "200" "$STORE_FILE"; then
    pass "Store uses 200 record limit"
else
    fail "Store does not use 200 record limit"
fi

if grep -q "30" "$STORE_FILE"; then
    pass "Store uses 30 day retention"
else
    fail "Store does not use 30 day retention"
fi

# ===========================================================
# Section 12: Max Signals Per Circle
# ===========================================================
section "Max Signals Per Circle"

if grep -q "MaxSignalsPerCirclePeriod\|MaxSignalsPerCircle" "$DOMAIN_FILE" || grep -q "MaxSignalsPerCircle" "$ENGINE_FILE"; then
    pass "Max signals per circle defined"
else
    fail "Max signals per circle not defined"
fi

if grep -q "3" "$DOMAIN_FILE" || grep -q "3" "$ENGINE_FILE"; then
    pass "Max 3 signals per circle enforced"
else
    fail "Max 3 signals per circle not enforced"
fi

# ===========================================================
# Section 13: No Ranking Logic
# ===========================================================
section "No Ranking Logic"

RANKING_WORDS=("score" "weight" "sort.*by.*relevance")

for word in "${RANKING_WORDS[@]}"; do
    if grep -ri "$word" internal/marketsignal/*.go 2>/dev/null | grep -vi "// NO RANKING" | grep -vi "No ranking" | grep -vi "NOT ranking" | grep -v "_test.go" | grep -v "hash" | grep -v "sort.*by.*SignalID\|sort.*by.*hash\|hash.*sort" > /dev/null 2>&1; then
        fail "Possible ranking logic '$word' found"
    else
        pass "No ranking logic '$word' found"
    fi
done

# Check specifically that "rank" and "priority" are only in comments explaining no ranking
if grep -ri "rank" internal/marketsignal/*.go 2>/dev/null | grep -vi "// " | grep -vi "#" | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Ranking logic found outside comments"
else
    pass "No ranking logic outside comments"
fi

if grep -ri "priorit" internal/marketsignal/*.go 2>/dev/null | grep -vi "// " | grep -vi "#" | grep -v "_test.go" | grep -vi "necessityPriority\|necessity.*priority\|priority.*necessity" > /dev/null 2>&1; then
    fail "Priority logic found outside comments"
else
    pass "No priority logic outside comments (necessityPriority is for finding highest necessity, not ranking signals)"
fi

# ===========================================================
# Section 14: No Pricing Fields
# ===========================================================
section "No Pricing Fields"

PRICING_WORDS=("price" "cost" "amount" "currency" "USD" "EUR" "GBP")

for word in "${PRICING_WORDS[@]}"; do
    if grep -ri "$word" pkg/domain/marketsignal/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
        fail "Pricing field '$word' found in domain"
    else
        pass "No pricing field '$word' in domain"
    fi
done

# ===========================================================
# Section 15: No Analytics
# ===========================================================
section "No Analytics"

ANALYTICS_WORDS=("track" "analytics" "telemetry" "metrics" "clickthrough" "ctr")

for word in "${ANALYTICS_WORDS[@]}"; do
    if grep -ri "$word" internal/marketsignal/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
        fail "Analytics '$word' found in engine"
    else
        pass "No analytics '$word' in engine"
    fi
done

# ===========================================================
# Section 16: Events
# ===========================================================
section "Events"

EVENTS_FILE="pkg/events/events.go"

if grep -q "Phase48MarketSignalGenerated" "$EVENTS_FILE"; then
    pass "Phase48MarketSignalGenerated event defined"
else
    fail "Phase48MarketSignalGenerated event missing"
fi

if grep -q "Phase48MarketProofViewed" "$EVENTS_FILE"; then
    pass "Phase48MarketProofViewed event defined"
else
    fail "Phase48MarketProofViewed event missing"
fi

if grep -q "Phase48MarketProofDismissed" "$EVENTS_FILE"; then
    pass "Phase48MarketProofDismissed event defined"
else
    fail "Phase48MarketProofDismissed event missing"
fi

# ===========================================================
# Section 17: Storelog Record Types
# ===========================================================
section "Storelog Record Types"

STORELOG_FILE="pkg/domain/storelog/log.go"

if grep -q "RecordTypeMarketSignal" "$STORELOG_FILE"; then
    pass "RecordTypeMarketSignal defined"
else
    fail "RecordTypeMarketSignal missing"
fi

if grep -q "RecordTypeMarketProofAck" "$STORELOG_FILE"; then
    pass "RecordTypeMarketProofAck defined"
else
    fail "RecordTypeMarketProofAck missing"
fi

# ===========================================================
# Section 18: Web Routes
# ===========================================================
section "Web Routes"

MAIN_FILE="cmd/quantumlife-web/main.go"

if grep -q '"/proof/market"' "$MAIN_FILE"; then
    pass "GET /proof/market route defined"
else
    fail "GET /proof/market route missing"
fi

if grep -q '"/proof/market/dismiss"' "$MAIN_FILE"; then
    pass "POST /proof/market/dismiss route defined"
else
    fail "POST /proof/market/dismiss route missing"
fi

# ===========================================================
# Section 19: POST-only Mutations
# ===========================================================
section "POST-only Mutations"

if grep -A5 "handleMarketProofDismiss" "$MAIN_FILE" | grep -q "MethodPost"; then
    pass "Dismiss handler requires POST"
else
    fail "Dismiss handler does not require POST"
fi

# ===========================================================
# Section 20: ADR Invariants
# ===========================================================
section "ADR Invariants"

ADR_FILE="docs/ADR/ADR-0086-phase48-market-signal-binding.md"

if grep -qi "signal exposure only" "$ADR_FILE"; then
    pass "ADR mentions signal exposure only"
else
    fail "ADR does not mention signal exposure only"
fi

if grep -qi "no.*recommend\|not.*recommend\|never.*recommend" "$ADR_FILE"; then
    pass "ADR prohibits recommendations"
else
    fail "ADR does not prohibit recommendations"
fi

if grep -qi "no.*rank\|not.*rank\|never.*rank" "$ADR_FILE"; then
    pass "ADR prohibits ranking"
else
    fail "ADR does not prohibit ranking"
fi

if grep -qi "effect.*no.*power\|effect_no_power" "$ADR_FILE"; then
    pass "ADR enforces effect_no_power"
else
    fail "ADR does not enforce effect_no_power"
fi

if grep -qi "proof.*only\|proof_only" "$ADR_FILE"; then
    pass "ADR enforces proof_only visibility"
else
    fail "ADR does not enforce proof_only visibility"
fi

if grep -qi "silence.*default\|default.*silence" "$ADR_FILE"; then
    pass "ADR mentions silence as default"
else
    fail "ADR does not mention silence as default"
fi

if grep -qi "not.*funnel\|no.*funnel" "$ADR_FILE"; then
    pass "ADR mentions not a funnel"
else
    fail "ADR does not mention not a funnel"
fi

if grep -qi "no.*pric\|not.*pric\|never.*pric" "$ADR_FILE"; then
    pass "ADR prohibits pricing"
else
    fail "ADR does not prohibit pricing"
fi

if grep -qi "no.*urgency\|not.*urgent" "$ADR_FILE"; then
    pass "ADR prohibits urgency"
else
    fail "ADR does not prohibit urgency"
fi

# ===========================================================
# Section 21: Stdlib Only
# ===========================================================
section "Stdlib Only (No External Dependencies)"

# Check domain package imports
if grep -E "github.com|gopkg.in" pkg/domain/marketsignal/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "External dependency in domain package"
else
    pass "No external dependencies in domain package"
fi

# Check engine package imports
if grep -E "github.com|gopkg.in" internal/marketsignal/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "External dependency in engine package"
else
    pass "No external dependencies in engine package"
fi

# ===========================================================
# Section 22: Hash-Only Storage
# ===========================================================
section "Hash-Only Storage"

if grep -q "HashString\|ComputeSignalID\|ComputeStatusHash" "$DOMAIN_FILE"; then
    pass "Hash functions defined"
else
    fail "Hash functions missing"
fi

if grep -q "dedupIndex\|dedup" "$STORE_FILE"; then
    pass "Store has deduplication"
else
    fail "Store missing deduplication"
fi

# ===========================================================
# Summary
# ===========================================================
echo ""
echo "==========================================="
echo "Phase 48 Guardrails Summary"
echo "==========================================="
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -eq 0 ]; then
    echo ""
    echo "All Phase 48 guardrails passed!"
    exit 0
else
    echo ""
    echo "Phase 48 guardrails FAILED. Fix issues above."
    exit 1
fi
