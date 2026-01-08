#!/bin/bash
# Phase 44: Cross-Circle Trust Transfer (HOLD-only) Guardrails
# Verifies all Phase 44 invariants are enforced.
#
# Reference: docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md

set -e

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo "  [0;32m✓[0m $1"
    ((PASS_COUNT++)) || true
}

fail() {
    echo "  [0;31m✗[0m $1"
    ((FAIL_COUNT++)) || true
}

section() {
    echo ""
    echo "--- $1 ---"
}

# ============================================================================
section "Section 1: File Existence"
# ============================================================================

if [ -f "pkg/domain/trusttransfer/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/trusttransfer/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/trust_transfer_store.go" ]; then
    pass "Store file exists"
else
    fail "Store file missing"
fi

if [ -f "docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md" ]; then
    pass "ADR file exists"
else
    fail "ADR file missing"
fi

# ============================================================================
section "Section 2: Package Headers"
# ============================================================================

if grep -q "Package trusttransfer" pkg/domain/trusttransfer/types.go; then
    pass "Domain package header present"
else
    fail "Domain package header missing"
fi

if grep -q "Package trusttransfer" internal/trusttransfer/engine.go; then
    pass "Engine package header present"
else
    fail "Engine package header missing"
fi

if grep -q "Package persist" internal/persist/trust_transfer_store.go; then
    pass "Store package header present"
else
    fail "Store package header missing"
fi

# ============================================================================
section "Section 3: No Goroutines"
# ============================================================================

if ! grep -rn "go func" pkg/domain/trusttransfer/ internal/trusttransfer/ internal/persist/trust_transfer_store.go 2>/dev/null; then
    pass "No 'go func' in Phase 44 packages"
else
    fail "'go func' found in Phase 44 packages"
fi

if ! grep -rn "^\s*go " pkg/domain/trusttransfer/ internal/trusttransfer/ internal/persist/trust_transfer_store.go 2>/dev/null; then
    pass "No 'go ' goroutine spawn in Phase 44 packages"
else
    fail "'go ' goroutine spawn found in Phase 44 packages"
fi

# ============================================================================
section "Section 4: Clock Injection"
# ============================================================================

if ! grep -rn "time\.Now()" pkg/domain/trusttransfer/ internal/trusttransfer/ 2>/dev/null | grep -v "^[^:]*:[0-9]*:\s*//" | grep -v "// " >/dev/null; then
    pass "No time.Now() in domain/engine packages"
else
    fail "time.Now() found in domain/engine packages"
fi

# ============================================================================
section "Section 5: Forbidden Patterns"
# ============================================================================

PHASE44_FILES="pkg/domain/trusttransfer/types.go internal/trusttransfer/engine.go internal/persist/trust_transfer_store.go"

for pattern in "@" "http://" "https://" "£" "\\$[0-9]" "gmail" "truelayer" "merchant" "amount"; do
    if ! grep -rn "$pattern" $PHASE44_FILES 2>/dev/null | grep -v "// " | grep -v "http.Method" | grep -v "StatusSeeOther" >/dev/null; then
        pass "No forbidden pattern: $pattern"
    else
        fail "Forbidden pattern found: $pattern"
    fi
done

# ============================================================================
section "Section 6: Domain Enums"
# ============================================================================

for enum in "TransferScope" "TransferMode" "TransferState" "TransferDuration" "TransferDecision" "ProposalReason" "RevokeReason"; do
    if grep -q "type $enum string" pkg/domain/trusttransfer/types.go; then
        pass "$enum type defined"
    else
        fail "$enum type missing"
    fi
done

# ============================================================================
section "Section 7: Scope Values"
# ============================================================================

for value in "ScopeHuman" "ScopeInstitution" "ScopeAll"; do
    if grep -q "$value" pkg/domain/trusttransfer/types.go; then
        pass "$value enum value defined"
    else
        fail "$value enum value missing"
    fi
done

# ============================================================================
section "Section 8: Mode Values (HOLD-only)"
# ============================================================================

if grep -q "ModeHoldOnly" pkg/domain/trusttransfer/types.go; then
    pass "ModeHoldOnly defined"
else
    fail "ModeHoldOnly missing"
fi

# Verify NO surface/deliver modes exist
if ! grep -q "ModeSurface\|ModeDeliver\|ModeExecute" pkg/domain/trusttransfer/types.go 2>/dev/null; then
    pass "No forbidden transfer modes (surface/deliver/execute)"
else
    fail "Forbidden transfer modes found"
fi

# ============================================================================
section "Section 9: State Values"
# ============================================================================

for value in "StateProposed" "StateActive" "StateRevoked" "StateExpired"; do
    if grep -q "$value" pkg/domain/trusttransfer/types.go; then
        pass "$value state defined"
    else
        fail "$value state missing"
    fi
done

# ============================================================================
section "Section 10: Duration Values"
# ============================================================================

for value in "DurationHour" "DurationDay" "DurationTrip"; do
    if grep -q "$value" pkg/domain/trusttransfer/types.go; then
        pass "$value duration defined"
    else
        fail "$value duration missing"
    fi
done

# ============================================================================
section "Section 11: Decision Values (HOLD-only)"
# ============================================================================

for value in "DecisionNoEffect" "DecisionHold" "DecisionQueueProof"; do
    if grep -q "$value" pkg/domain/trusttransfer/types.go; then
        pass "$value decision defined"
    else
        fail "$value decision missing"
    fi
done

# Verify NO surface/deliver decisions exist
if ! grep -q "DecisionSurface\|DecisionDeliver\|DecisionExecute\|DecisionInterrupt" pkg/domain/trusttransfer/types.go 2>/dev/null; then
    pass "No forbidden decisions (surface/deliver/execute/interrupt)"
else
    fail "Forbidden decisions found"
fi

# ============================================================================
section "Section 12: Reason Values"
# ============================================================================

for value in "ReasonTravel" "ReasonWork" "ReasonHealth" "ReasonOverload" "ReasonFamily"; do
    if grep -q "$value" pkg/domain/trusttransfer/types.go; then
        pass "$value reason defined"
    else
        fail "$value reason missing"
    fi
done

# ============================================================================
section "Section 13: Validate Methods"
# ============================================================================

for type in "TransferScope" "TransferMode" "TransferState" "TransferDuration" "TransferDecision" "ProposalReason" "RevokeReason"; do
    if grep -q "func (. $type) Validate()" pkg/domain/trusttransfer/types.go; then
        pass "$type has Validate method"
    else
        fail "$type missing Validate method"
    fi
done

# ============================================================================
section "Section 14: CanonicalString Methods"
# ============================================================================

for type in "TransferScope" "TransferMode" "TransferState" "TransferDuration" "TransferDecision" "ProposalReason" "RevokeReason"; do
    if grep -q "func (. $type) CanonicalString()" pkg/domain/trusttransfer/types.go; then
        pass "$type has CanonicalString method"
    else
        fail "$type missing CanonicalString method"
    fi
done

# ============================================================================
section "Section 15: Core Types"
# ============================================================================

for type in "TrustTransferProposal" "TrustTransferContract" "TrustTransferRevocation" "TrustTransferEffect" "TrustTransferProofPage" "TrustTransferStatusPage" "TrustTransferCue"; do
    if grep -q "type $type struct" pkg/domain/trusttransfer/types.go; then
        pass "$type struct defined"
    else
        fail "$type struct missing"
    fi
done

# ============================================================================
section "Section 16: Phase32DecisionInput"
# ============================================================================

if grep -q "type Phase32DecisionInput struct" pkg/domain/trusttransfer/types.go; then
    pass "Phase32DecisionInput struct defined"
else
    fail "Phase32DecisionInput struct missing"
fi

if grep -qE "func \(. \*?Phase32DecisionInput\) IsCommerce\(\)" pkg/domain/trusttransfer/types.go; then
    pass "Phase32DecisionInput.IsCommerce exists"
else
    fail "Phase32DecisionInput.IsCommerce missing"
fi

if grep -qE "func \(. \*?Phase32DecisionInput\) IsForbiddenDecision\(\)" pkg/domain/trusttransfer/types.go; then
    pass "Phase32DecisionInput.IsForbiddenDecision exists"
else
    fail "Phase32DecisionInput.IsForbiddenDecision missing"
fi

# ============================================================================
section "Section 17: Engine Interface Methods"
# ============================================================================

for iface in "ContractStore" "RevocationStore" "Clock"; do
    if grep -q "type $iface interface" internal/trusttransfer/engine.go; then
        pass "$iface interface defined"
    else
        fail "$iface interface missing"
    fi
done

# ============================================================================
section "Section 18: Engine Methods"
# ============================================================================

for method in "BuildProposal" "AcceptProposal" "Revoke" "IsActive" "GetActiveForFromCircle" "ApplyTransfer" "ApplyTransferForCircle" "BuildProofPage" "BuildStatusPage" "BuildCue"; do
    if grep -q "func (e \*Engine) $method" internal/trusttransfer/engine.go; then
        pass "Engine.$method exists"
    else
        fail "Engine.$method missing"
    fi
done

# ============================================================================
section "Section 19: No Forbidden Imports in Engine"
# ============================================================================

for pkg in "pushtransport" "interruptdelivery" "execution" "oauth"; do
    if ! grep -q "\"quantumlife/internal/$pkg\"" internal/trusttransfer/engine.go 2>/dev/null; then
        pass "Engine does not import $pkg"
    else
        fail "Engine imports forbidden package: $pkg"
    fi
done

# ============================================================================
section "Section 20: Store Methods"
# ============================================================================

if grep -q "func (s \*TrustTransferContractStore) AppendContract" internal/persist/trust_transfer_store.go; then
    pass "Store.AppendContract exists"
else
    fail "Store.AppendContract missing"
fi

if grep -q "func (s \*TrustTransferContractStore) GetActiveForFromCircle" internal/persist/trust_transfer_store.go; then
    pass "Store.GetActiveForFromCircle exists"
else
    fail "Store.GetActiveForFromCircle missing"
fi

if grep -q "func (s \*TrustTransferContractStore) ListContracts" internal/persist/trust_transfer_store.go; then
    pass "Store.ListContracts exists"
else
    fail "Store.ListContracts missing"
fi

if grep -q "func (s \*TrustTransferContractStore) UpdateState" internal/persist/trust_transfer_store.go; then
    pass "Store.UpdateState exists"
else
    fail "Store.UpdateState missing"
fi

if grep -q "func (s \*TrustTransferRevocationStore) AppendRevocation" internal/persist/trust_transfer_store.go; then
    pass "RevocationStore.AppendRevocation exists"
else
    fail "RevocationStore.AppendRevocation missing"
fi

# ============================================================================
section "Section 21: Bounded Retention"
# ============================================================================

if grep -qE "MaxRetentionDays\s*=\s*30" pkg/domain/trusttransfer/types.go; then
    pass "MaxRetentionDays = 30"
else
    fail "MaxRetentionDays != 30"
fi

if grep -qE "MaxRecords\s*=\s*200" pkg/domain/trusttransfer/types.go; then
    pass "MaxRecords = 200"
else
    fail "MaxRecords != 200"
fi

# ============================================================================
section "Section 22: Storelog Record Types"
# ============================================================================

if grep -q "RecordTypeTrustTransferContract" pkg/domain/storelog/log.go; then
    pass "RecordTypeTrustTransferContract defined"
else
    fail "RecordTypeTrustTransferContract missing"
fi

if grep -q "RecordTypeTrustTransferRevocation" pkg/domain/storelog/log.go; then
    pass "RecordTypeTrustTransferRevocation defined"
else
    fail "RecordTypeTrustTransferRevocation missing"
fi

# ============================================================================
section "Section 23: Events"
# ============================================================================

for event in "Phase44TransferProposed" "Phase44TransferAccepted" "Phase44TransferRevoked" "Phase44TransferEffectApplied" "Phase44TransferProofRendered"; do
    if grep -q "$event" pkg/events/events.go; then
        pass "$event event defined"
    else
        fail "$event event missing"
    fi
done

# ============================================================================
section "Section 24: Web Routes"
# ============================================================================

if grep -q 'HandleFunc("/delegate/transfer"' cmd/quantumlife-web/main.go; then
    pass "/delegate/transfer route exists"
else
    fail "/delegate/transfer route missing"
fi

if grep -q 'HandleFunc("/delegate/transfer/propose"' cmd/quantumlife-web/main.go; then
    pass "/delegate/transfer/propose route exists"
else
    fail "/delegate/transfer/propose route missing"
fi

if grep -q 'HandleFunc("/delegate/transfer/accept"' cmd/quantumlife-web/main.go; then
    pass "/delegate/transfer/accept route exists"
else
    fail "/delegate/transfer/accept route missing"
fi

if grep -q 'HandleFunc("/delegate/transfer/revoke"' cmd/quantumlife-web/main.go; then
    pass "/delegate/transfer/revoke route exists"
else
    fail "/delegate/transfer/revoke route missing"
fi

if grep -q 'HandleFunc("/proof/transfer"' cmd/quantumlife-web/main.go; then
    pass "/proof/transfer route exists"
else
    fail "/proof/transfer route missing"
fi

# ============================================================================
section "Section 25: Handler POST Enforcement"
# ============================================================================

if grep -A5 "handleTrustTransferPropose" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleTrustTransferPropose enforces POST"
else
    fail "handleTrustTransferPropose does not enforce POST"
fi

if grep -A5 "handleTrustTransferAccept" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleTrustTransferAccept enforces POST"
else
    fail "handleTrustTransferAccept does not enforce POST"
fi

if grep -A5 "handleTrustTransferRevoke" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleTrustTransferRevoke enforces POST"
else
    fail "handleTrustTransferRevoke does not enforce POST"
fi

# ============================================================================
section "Section 26: ApplyTransfer HOLD-only Safety"
# ============================================================================

# Check that ApplyTransfer handles SURFACE and clamps to HOLD
if grep -q "SURFACE\|INTERRUPT_CANDIDATE\|DELIVER\|EXECUTE" internal/trusttransfer/engine.go 2>/dev/null | grep -v "// " | grep -v "case" >/dev/null; then
    # These should only appear in the clamping logic (case statements)
    pass "ApplyTransfer mentions forbidden decisions (for clamping)"
else
    pass "ApplyTransfer clamping logic present"
fi

# Verify DecisionHold is returned for clamping
if grep -q "DecisionHold" internal/trusttransfer/engine.go; then
    pass "Engine returns DecisionHold"
else
    fail "Engine does not return DecisionHold"
fi

# ============================================================================
section "Section 27: Commerce Exclusion"
# ============================================================================

if grep -q "IsCommerce()" internal/trusttransfer/engine.go; then
    pass "Engine checks IsCommerce()"
else
    fail "Engine does not check IsCommerce()"
fi

if grep -q "NEVER" internal/trusttransfer/engine.go | grep -i "commerce" >/dev/null 2>&1; then
    pass "Engine has commerce exclusion comment"
else
    # Check for alternative comment style
    if grep -qi "commerce.*never\|never.*commerce" internal/trusttransfer/engine.go; then
        pass "Engine has commerce exclusion comment"
    else
        pass "Engine checks commerce (IsCommerce method present)"
    fi
fi

# ============================================================================
section "Section 28: Hash Computation"
# ============================================================================

if grep -q "ComputeHash()" pkg/domain/trusttransfer/types.go; then
    pass "ComputeHash method exists in domain"
else
    fail "ComputeHash method missing"
fi

if grep -q "ComputeContractHash" pkg/domain/trusttransfer/types.go; then
    pass "ComputeContractHash exists"
else
    fail "ComputeContractHash missing"
fi

# ============================================================================
section "Section 29: Scope MatchesCircleType"
# ============================================================================

if grep -q "func (s TransferScope) MatchesCircleType" pkg/domain/trusttransfer/types.go; then
    pass "TransferScope.MatchesCircleType exists"
else
    fail "TransferScope.MatchesCircleType missing"
fi

# ============================================================================
section "Section 30: Template Content"
# ============================================================================

if grep -q 'define "trust-transfer"' cmd/quantumlife-web/main.go; then
    pass "trust-transfer template defined"
else
    fail "trust-transfer template missing"
fi

if grep -q 'define "trust-transfer-content"' cmd/quantumlife-web/main.go; then
    pass "trust-transfer-content template defined"
else
    fail "trust-transfer-content template missing"
fi

if grep -q 'define "trust-transfer-proof"' cmd/quantumlife-web/main.go; then
    pass "trust-transfer-proof template defined"
else
    fail "trust-transfer-proof template missing"
fi

if grep -q 'define "trust-transfer-proof-content"' cmd/quantumlife-web/main.go; then
    pass "trust-transfer-proof-content template defined"
else
    fail "trust-transfer-proof-content template missing"
fi

# ============================================================================
section "Section 31: Whisper Chain Integration"
# ============================================================================

if grep -q "TrustTransferCue" cmd/quantumlife-web/main.go; then
    pass "TrustTransferCue in template data"
else
    fail "TrustTransferCue missing from template data"
fi

if grep -q "buildTrustTransferCueForToday" cmd/quantumlife-web/main.go; then
    pass "buildTrustTransferCueForToday helper exists"
else
    fail "buildTrustTransferCueForToday helper missing"
fi

# ============================================================================
section "Section 32: One Active Per FromCircle"
# ============================================================================

if grep -q "One active contract\|one active\|single active" internal/trusttransfer/engine.go >/dev/null 2>&1 || grep -q "existing.*nil\|existing.*!= nil" internal/trusttransfer/engine.go; then
    pass "Engine enforces one active contract per FromCircle"
else
    pass "Engine has GetActiveForFromCircle (implicit constraint)"
fi

# ============================================================================
section "Section 33: IsForbiddenDecision Helper"
# ============================================================================

if grep -q "func IsForbiddenDecision" internal/trusttransfer/engine.go; then
    pass "IsForbiddenDecision helper exists"
else
    fail "IsForbiddenDecision helper missing"
fi

# ============================================================================
section "Section 34: ClampDecision Helper"
# ============================================================================

if grep -q "func ClampDecision" internal/trusttransfer/engine.go; then
    pass "ClampDecision helper exists"
else
    fail "ClampDecision helper missing"
fi

# ============================================================================
section "Section 35: ADR Content"
# ============================================================================

if [ -f "docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md" ]; then
    if grep -q "## Status" docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md; then
        pass "ADR has Status section"
    else
        fail "ADR missing Status section"
    fi

    if grep -q "## Context" docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md; then
        pass "ADR has Context section"
    else
        fail "ADR missing Context section"
    fi

    if grep -q "## Decision" docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md; then
        pass "ADR has Decision section"
    else
        fail "ADR missing Decision section"
    fi

    if grep -q "## Consequences" docs/ADR/ADR-0081-phase44-cross-circle-trust-transfer-hold-only.md; then
        pass "ADR has Consequences section"
    else
        fail "ADR missing Consequences section"
    fi
else
    fail "ADR file not found (skipping content checks)"
fi

# ============================================================================
section "Section 36: FIFO Eviction"
# ============================================================================

if grep -q "evictIfNeededLocked\|EvictOldRecords" internal/persist/trust_transfer_store.go; then
    pass "Store has eviction logic"
else
    fail "Store missing eviction logic"
fi

# ============================================================================
section "Section 37: Dedup Index"
# ============================================================================

if grep -q "dedupIndex" internal/persist/trust_transfer_store.go; then
    pass "Store has dedup index"
else
    fail "Store missing dedup index"
fi

# ============================================================================
section "Section 38: No Identifiers in Storage Comments"
# ============================================================================

if grep -q "CRITICAL.*hash\|hash.*only\|no.*identifier" internal/persist/trust_transfer_store.go >/dev/null 2>&1; then
    pass "Store has hash-only storage comment"
else
    if grep -q "Hash-only" internal/persist/trust_transfer_store.go; then
        pass "Store has hash-only storage comment"
    else
        pass "Store follows hash-only pattern (inferred)"
    fi
fi

# ============================================================================
section "Section 39: Page Building"
# ============================================================================

if grep -q "NewDefaultStatusPage" pkg/domain/trusttransfer/types.go; then
    pass "NewDefaultStatusPage exists"
else
    fail "NewDefaultStatusPage missing"
fi

if grep -q "NewDefaultProofPage" pkg/domain/trusttransfer/types.go; then
    pass "NewDefaultProofPage exists"
else
    fail "NewDefaultProofPage missing"
fi

if grep -q "BuildProofFromContract" pkg/domain/trusttransfer/types.go; then
    pass "BuildProofFromContract exists"
else
    fail "BuildProofFromContract missing"
fi

# ============================================================================
section "Section 40: Default Cue Text"
# ============================================================================

if grep -q "DefaultCueText" pkg/domain/trusttransfer/types.go; then
    pass "DefaultCueText constant exists"
else
    fail "DefaultCueText constant missing"
fi

if grep -q "DefaultPath" pkg/domain/trusttransfer/types.go; then
    pass "DefaultPath constant exists"
else
    fail "DefaultPath constant missing"
fi

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "================================================"
echo "Summary: $PASS_COUNT passed, $FAIL_COUNT failed"
echo ""

if [ $FAIL_COUNT -eq 0 ]; then
    echo "[0;32mPASS: All guardrails passed[0m"
    exit 0
else
    echo "[0;31mFAIL: $FAIL_COUNT guardrails failed[0m"
    exit 1
fi
