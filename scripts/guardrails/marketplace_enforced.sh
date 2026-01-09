#!/usr/bin/env bash
# =============================================================================
# Phase 46: Circle Registry + Packs (Marketplace v0) Guardrails
# =============================================================================
#
# CRITICAL INVARIANTS:
# 1. effect_no_power ALWAYS - packs provide meaning only, no permission
# 2. Observer bindings are INTENT-ONLY - no real wiring
# 3. Hash-only storage - no raw identifiers or secrets
# 4. No goroutines in pkg/ or internal/marketplace
# 5. No imports from decision packages
# 6. POST-only for mutations
# 7. Clock injection (no time.Now in domain/engine)
# 8. Bounded retention (30 days OR 200 records)
#
# Reference: docs/ADR/ADR-0084-phase46-circle-registry-packs.md
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PASS_COUNT=0
FAIL_COUNT=0
TOTAL_CHECKS=0

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    echo "  [PASS] $1"
}

fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    echo "  [FAIL] $1"
}

check_file_exists() {
    if [[ -f "$1" ]]; then
        pass "File exists: $1"
    else
        fail "File missing: $1"
    fi
}

check_file_contains() {
    local file="$1"
    local pattern="$2"
    local desc="$3"
    if grep -q "$pattern" "$file" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

check_file_not_contains() {
    local file="$1"
    local pattern="$2"
    local desc="$3"
    if ! grep -q "$pattern" "$file" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

check_dir_not_contains() {
    local dir="$1"
    local pattern="$2"
    local desc="$3"
    if ! grep -rq "$pattern" "$dir" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

echo "============================================================"
echo "Phase 46: Circle Registry + Packs (Marketplace v0) Guardrails"
echo "============================================================"
echo ""

# =============================================================================
# Section 1: File Structure (15 checks)
# =============================================================================
echo "Section 1: File Structure"
echo "-----------------------------------------------------------"

check_file_exists "$PROJECT_ROOT/pkg/domain/marketplace/types.go"
check_file_exists "$PROJECT_ROOT/internal/marketplace/registry.go"
check_file_exists "$PROJECT_ROOT/internal/marketplace/engine.go"
check_file_exists "$PROJECT_ROOT/internal/persist/marketplace_store.go"
check_file_exists "$PROJECT_ROOT/docs/ADR/ADR-0084-phase46-circle-registry-packs.md"

# Check package declarations
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "package marketplace" "Domain types package declaration"
check_file_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "package marketplace" "Registry package declaration"
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "package marketplace" "Engine package declaration"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "package persist" "Persist package declaration"

# Check critical comments
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "effect_no_power" "Domain types effect_no_power reference"
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "CRITICAL" "Engine CRITICAL comments"
check_file_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "MUST NOT" "Registry MUST NOT comments"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "MUST NOT grant permission" "Domain types permission warning"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "FIFO" "Store FIFO eviction reference"
check_file_contains "$PROJECT_ROOT/docs/ADR/ADR-0084-phase46-circle-registry-packs.md" "effect_no_power" "ADR effect_no_power reference"

echo ""

# =============================================================================
# Section 2: effect_no_power Enforcement (20 checks)
# =============================================================================
echo "Section 2: effect_no_power Enforcement"
echo "-----------------------------------------------------------"

# Domain types must define EffectNoPower
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" 'EffectNoPower PackEffect = "effect_no_power"' "EffectNoPower constant defined"

# PackEffect Validate must only allow effect_no_power
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "only effect_no_power allowed" "PackEffect validation message"

# PackTemplate must have Effect field
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "Effect.*PackEffect" "PackTemplate has Effect field"

# ObserverBinding must have Effect field
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "ObserverBinding struct" "ObserverBinding struct exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "Effect.*PackEffect.*EffectNoPower" "ObserverBinding Effect field"

# PackInstallRecord must have Effect field
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "PackInstallRecord struct" "PackInstallRecord struct exists"

# PackCard must have Effect field
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "PackCard struct" "PackCard struct exists"

# InstalledProofLine must have Effect field
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "InstalledProofLine struct" "InstalledProofLine struct exists"

# Engine must enforce EffectNoPower on install
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "EffectNoPower" "Engine uses EffectNoPower"
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "CRITICAL.*Always enforced" "Engine CRITICAL enforcement comment"

# Registry default packs must use EffectNoPower
check_file_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "Effect:.*EffectNoPower" "Registry packs use EffectNoPower"

# No other effect values should exist
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" 'effect_surface' "No effect_surface in domain types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" 'effect_interrupt' "No effect_interrupt in domain types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" 'effect_deliver' "No effect_deliver in domain types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" 'effect_execute' "No effect_execute in domain types"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" 'effect_surface' "No effect_surface in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" 'effect_interrupt' "No effect_interrupt in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" 'effect_deliver' "No effect_deliver in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" 'effect_execute' "No effect_execute in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" 'effect_surface' "No effect_surface in registry"

echo ""

# =============================================================================
# Section 3: Observer Bindings Intent-Only (15 checks)
# =============================================================================
echo "Section 3: Observer Bindings Intent-Only"
echo "-----------------------------------------------------------"

# ObserverBinding should be intent-only
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "intent" "Domain types mention intent"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "no real wiring" "Domain types no real wiring"

# BindingKind should exist
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "BindingKind" "BindingKind type exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "binding_kind_observe_only" "observe_only binding kind"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "binding_kind_annotate" "annotate binding kind"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "binding_kind_enrich" "enrich binding kind"

# No actual observer wiring imports
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "observer.Wire" "No observer wiring in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "observer.Bind" "No observer binding in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "observer.Wire" "No observer wiring in registry"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "observer.Bind" "No observer binding in registry"

# ObserverBindingDisplay for proof pages
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "ObserverBindingDisplay" "ObserverBindingDisplay exists"

# Template mentions intent-only
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "Intent Only" "Template mentions Intent Only"

# Engine BuildDetailPage uses binding displays
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "ObserverBindingDisplay" "Engine uses ObserverBindingDisplay"

# No execute or deliver logic
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "Execute\(" "No Execute in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "Deliver\(" "No Deliver in engine"

echo ""

# =============================================================================
# Section 4: Hash-Only Storage (20 checks)
# =============================================================================
echo "Section 4: Hash-Only Storage"
echo "-----------------------------------------------------------"

# PackSlugHash usage
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "PackSlugHash" "PackSlugHash field exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "VersionHash" "VersionHash field exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "StatusHash" "StatusHash field exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "CirclePatternHash" "CirclePatternHash field exists"

# Hash functions
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "HashString" "HashString function exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "ComputeStatusHash" "ComputeStatusHash function exists"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "ComputeRemovalStatusHash" "ComputeRemovalStatusHash function exists"

# Store uses hashes
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "PackSlugHash" "Store uses PackSlugHash"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "StatusHash" "Store uses StatusHash"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "dedupIndex" "Store has dedup index"

# Engine uses hash functions
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "HashString" "Engine uses HashString"

# Registry hashes patterns
check_file_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "HashString" "Registry uses HashString"

# No raw identifiers in storage
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "RawSlug" "No RawSlug in store"
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "RawName" "No RawName in store"
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "Email" "No Email in store"
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "PhoneNumber" "No PhoneNumber in store"

# Canonical strings for hashing
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "CanonicalStringV1" "CanonicalStringV1 methods exist"
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "pipe-delimited" "Pipe-delimited format mentioned"

# SlugHash in UI
check_file_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "SlugHash.*string" "PackCard has SlugHash"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "SlugHash" "Template uses SlugHash"

echo ""

# =============================================================================
# Section 5: No Goroutines (10 checks)
# =============================================================================
echo "Section 5: No Goroutines"
echo "-----------------------------------------------------------"

check_dir_not_contains "$PROJECT_ROOT/pkg/domain/marketplace" "go func" "No goroutines in domain types"
check_dir_not_contains "$PROJECT_ROOT/internal/marketplace" "go func" "No goroutines in internal marketplace"
check_dir_not_contains "$PROJECT_ROOT/pkg/domain/marketplace" "go func(" "No goroutine spawns in domain"
check_dir_not_contains "$PROJECT_ROOT/internal/marketplace" "go func(" "No goroutine spawns in internal"
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "go func" "No goroutines in marketplace store"

# Sync package usage should only be for mutex
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "sync.RWMutex" "Store uses RWMutex"
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "sync.WaitGroup" "No WaitGroup in store"
check_file_not_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "sync.Cond" "No Cond in store"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "chan " "No channels in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "chan " "No channels in registry"

echo ""

# =============================================================================
# Section 6: No Decision Package Imports (15 checks)
# =============================================================================
echo "Section 6: No Decision Package Imports"
echo "-----------------------------------------------------------"

# Domain types should not import decision packages
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "pressuredecision" "No pressuredecision import in types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "interruptpolicy" "No interruptpolicy import in types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "interruptpreview" "No interruptpreview import in types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "execrouter" "No execrouter import in types"
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "execexecutor" "No execexecutor import in types"

# Engine should not import decision packages
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "pressuredecision" "No pressuredecision import in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "interruptpolicy" "No interruptpolicy import in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "interruptpreview" "No interruptpreview import in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "execrouter" "No execrouter import in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "execexecutor" "No execexecutor import in engine"

# Registry should not import decision packages
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "pressuredecision" "No pressuredecision import in registry"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "interruptpolicy" "No interruptpolicy import in registry"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "interruptpreview" "No interruptpreview import in registry"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "execrouter" "No execrouter import in registry"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "execexecutor" "No execexecutor import in registry"

echo ""

# =============================================================================
# Section 7: POST-Only Mutations (10 checks)
# =============================================================================
echo "Section 7: POST-Only Mutations"
echo "-----------------------------------------------------------"

# Check handlers exist for mutations
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'handleMarketplaceInstall' "Install handler exists"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'handleMarketplaceRemove' "Remove handler exists"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'handleMarketplaceProofDismiss' "Dismiss handler exists"

# Routes use POST for mutations (check route registration comments)
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/marketplace/install.*POST' "Install route comment shows POST"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/marketplace/remove.*POST' "Remove route comment shows POST"

# Templates use POST forms
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'method="POST" action="/marketplace/install"' "Install form uses POST"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'method="POST" action="/marketplace/remove"' "Remove form uses POST"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'method="POST" action="/proof/marketplace/dismiss"' "Dismiss form uses POST"

# GET routes for read-only (check route registration)
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/marketplace.*handleMarketplaceHome' "Home handler registered"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/proof/marketplace.*handleMarketplaceProof' "Proof handler registered"

echo ""

# =============================================================================
# Section 8: Clock Injection (10 checks)
# =============================================================================
echo "Section 8: Clock Injection"
echo "-----------------------------------------------------------"

# Engine uses injected clock
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "clock func()" "Engine has clock function"
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "NewEngine.*clock" "NewEngine takes clock"
check_file_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "e.clock()" "Engine uses clock field"

# Stores use injected clock
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "clock.*func().*time.Time" "Store has clock function"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "s.clock()" "Store uses clock field"

# No time.Now() calls in domain or engine (comments OK)
check_file_not_contains "$PROJECT_ROOT/pkg/domain/marketplace/types.go" "= time.Now" "No time.Now in domain types"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/engine.go" "= time.Now" "No time.Now call in engine"
check_file_not_contains "$PROJECT_ROOT/internal/marketplace/registry.go" "= time.Now" "No time.Now in registry"

# Main.go injects clock
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "NewMarketplaceInstallStore" "Install store created in main"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "internalmarketplace.NewEngine" "Engine created in main"

echo ""

# =============================================================================
# Section 9: Bounded Retention (10 checks)
# =============================================================================
echo "Section 9: Bounded Retention"
echo "-----------------------------------------------------------"

# Constants defined
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "MarketplaceInstallMaxRecords.*200" "Install max records = 200"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "MarketplaceInstallMaxRetentionDays.*30" "Install retention = 30 days"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "MarketplaceRemovalMaxRecords.*200" "Removal max records = 200"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "MarketplaceRemovalMaxRetentionDays.*30" "Removal retention = 30 days"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "MarketplaceAckMaxRecords.*200" "Ack max records = 200"

# Eviction functions exist
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "evictOldRecordsLocked" "Install eviction function exists"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "evictOldAcksLocked" "Ack eviction function exists"

# FIFO eviction logic
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "cutoffKey" "Cutoff date calculated"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "keepRecords" "Keep records logic exists"
check_file_contains "$PROJECT_ROOT/internal/persist/marketplace_store.go" "evictCount" "Evict count calculated"

echo ""

# =============================================================================
# Section 10: Web Integration (10 checks)
# =============================================================================
echo "Section 10: Web Integration"
echo "-----------------------------------------------------------"

# Routes registered
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/marketplace.*handleMarketplaceHome' "Home route registered"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/marketplace/pack/.*handleMarketplacePackDetail' "Detail route registered"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" '/proof/marketplace.*handleMarketplaceProof' "Proof route registered"

# Server struct fields
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "marketplaceRegistry" "Server has registry field"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "marketplaceEngine" "Server has engine field"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" "marketplaceInstallStore" "Server has install store field"

# Templates defined
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'define "marketplace-home"' "Home template defined"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'define "marketplace-pack-detail"' "Detail template defined"
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'define "marketplace-proof"' "Proof template defined"

# Imports
check_file_contains "$PROJECT_ROOT/cmd/quantumlife-web/main.go" 'internalmarketplace "quantumlife/internal/marketplace"' "Internal marketplace import"

echo ""

# =============================================================================
# Section 11: Events and Storelog (10 checks)
# =============================================================================
echo "Section 11: Events and Storelog"
echo "-----------------------------------------------------------"

# Events defined
check_file_contains "$PROJECT_ROOT/pkg/events/events.go" "Phase46MarketplaceHomeViewed" "Home viewed event"
check_file_contains "$PROJECT_ROOT/pkg/events/events.go" "Phase46PackInstalled" "Pack installed event"
check_file_contains "$PROJECT_ROOT/pkg/events/events.go" "Phase46PackRemoved" "Pack removed event"
check_file_contains "$PROJECT_ROOT/pkg/events/events.go" "Phase46MarketplaceProofRendered" "Proof rendered event"
check_file_contains "$PROJECT_ROOT/pkg/events/events.go" "Phase46MarketplaceProofDismissed" "Proof dismissed event"

# Storelog record types
check_file_contains "$PROJECT_ROOT/pkg/domain/storelog/log.go" "RecordTypePackInstall" "Pack install record type"
check_file_contains "$PROJECT_ROOT/pkg/domain/storelog/log.go" "RecordTypePackRemoval" "Pack removal record type"
check_file_contains "$PROJECT_ROOT/pkg/domain/storelog/log.go" "RecordTypeMarketplaceAck" "Marketplace ack record type"

# Critical comments in storelog
check_file_contains "$PROJECT_ROOT/pkg/domain/storelog/log.go" "Phase 46" "Storelog Phase 46 section"
check_file_contains "$PROJECT_ROOT/pkg/domain/storelog/log.go" "effect_no_power" "Storelog effect_no_power reference"

echo ""

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "============================================================"
echo "Summary"
echo "============================================================"
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"
echo ""

if [[ $FAIL_COUNT -eq 0 ]]; then
    echo "All Phase 46 guardrails PASSED!"
    exit 0
else
    echo "Phase 46 guardrails FAILED with $FAIL_COUNT failures."
    exit 1
fi
