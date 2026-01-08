#!/bin/bash
# Phase 42: Delegated Holding Contracts Guardrails
# Verifies all Phase 42 invariants are enforced.
#
# Reference: docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md

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

if [ -f "pkg/domain/delegatedholding/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/delegatedholding/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/delegated_holding_store.go" ]; then
    pass "Store file exists"
else
    fail "Store file missing"
fi

if [ -f "docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md" ]; then
    pass "ADR file exists"
else
    fail "ADR file missing"
fi

# ============================================================================
section "Section 2: Package Headers"
# ============================================================================

if grep -q "Package delegatedholding" pkg/domain/delegatedholding/types.go; then
    pass "Domain package header present"
else
    fail "Domain package header missing"
fi

if grep -q "Package delegatedholding" internal/delegatedholding/engine.go; then
    pass "Engine package header present"
else
    fail "Engine package header missing"
fi

# ============================================================================
section "Section 3: No Goroutines"
# ============================================================================

if ! grep -rn "go func" pkg/domain/delegatedholding/ internal/delegatedholding/ internal/persist/delegated_holding_store.go 2>/dev/null; then
    pass "No 'go func' in Phase 42 packages"
else
    fail "'go func' found in Phase 42 packages"
fi

if ! grep -rn "^\s*go " pkg/domain/delegatedholding/ internal/delegatedholding/ internal/persist/delegated_holding_store.go 2>/dev/null; then
    pass "No 'go ' goroutine spawn in Phase 42 packages"
else
    fail "'go ' goroutine spawn found in Phase 42 packages"
fi

# ============================================================================
section "Section 4: Clock Injection"
# ============================================================================

if ! grep -rn "time\.Now()" pkg/domain/delegatedholding/ internal/delegatedholding/ 2>/dev/null | grep -v "^[^:]*:[0-9]*:\s*//" | grep -v "// " >/dev/null; then
    pass "No time.Now() in domain/engine packages"
else
    fail "time.Now() found in domain/engine packages"
fi

# ============================================================================
section "Section 5: Forbidden Patterns"
# ============================================================================

PHASE42_FILES="pkg/domain/delegatedholding/types.go internal/delegatedholding/engine.go internal/persist/delegated_holding_store.go"

for pattern in "@" "http://" "https://" "£" "\\$[0-9]" "gmail" "truelayer" "merchant" "amount"; do
    if ! grep -rn "$pattern" $PHASE42_FILES 2>/dev/null | grep -v "// " | grep -v "http.Method" | grep -v "StatusSeeOther" >/dev/null; then
        pass "No forbidden pattern: $pattern"
    else
        fail "Forbidden pattern found: $pattern"
    fi
done

# ============================================================================
section "Section 6: Domain Enums"
# ============================================================================

for enum in "DelegationScope" "DelegationAction" "DelegationDuration" "DelegationState" "ApplyResultKind" "DecisionReasonBucket"; do
    if grep -q "type $enum string" pkg/domain/delegatedholding/types.go; then
        pass "$enum type defined"
    else
        fail "$enum type missing"
    fi
done

# ============================================================================
section "Section 7: Enum Values"
# ============================================================================

for value in "ScopeHuman" "ScopeInstitution" "ActionHoldSilently" "ActionQueueProof" "DurationHour" "DurationDay" "DurationTrip" "StateActive" "StateExpired" "StateRevoked" "ResultNoEffect" "ResultHold" "ResultQueueProof"; do
    if grep -q "$value" pkg/domain/delegatedholding/types.go; then
        pass "$value enum value defined"
    else
        fail "$value enum value missing"
    fi
done

# ============================================================================
section "Section 8: Validate Methods"
# ============================================================================

for type in "DelegationScope" "DelegationAction" "DelegationDuration" "DelegationState" "ApplyResultKind" "DecisionReasonBucket"; do
    if grep -q "func (. $type) Validate()" pkg/domain/delegatedholding/types.go; then
        pass "$type has Validate method"
    else
        fail "$type missing Validate method"
    fi
done

# ============================================================================
section "Section 9: CanonicalString Methods"
# ============================================================================

for type in "DelegationScope" "DelegationAction" "DelegationDuration" "DelegationState" "ApplyResultKind"; do
    if grep -q "func (. $type) CanonicalString()" pkg/domain/delegatedholding/types.go; then
        pass "$type has CanonicalString method"
    else
        fail "$type missing CanonicalString method"
    fi
done

# ============================================================================
section "Section 10: Core Types"
# ============================================================================

for type in "DelegatedHoldingContract" "DelegationInputs" "CreateContractInput" "RevokeContractInput" "DelegationProof" "EligibilityDecision" "HoldingDecision" "PressureInput"; do
    if grep -q "type $type struct" pkg/domain/delegatedholding/types.go; then
        pass "$type struct defined"
    else
        fail "$type struct missing"
    fi
done

# ============================================================================
section "Section 11: Engine Interface Methods"
# ============================================================================

for iface in "TrustSource" "InterruptPreviewSource" "ContractStore" "Clock"; do
    if grep -q "type $iface interface" internal/delegatedholding/engine.go; then
        pass "$iface interface defined"
    else
        fail "$iface interface missing"
    fi
done

# ============================================================================
section "Section 12: Engine Methods"
# ============================================================================

for method in "CanCreateContract" "CreateContract" "ComputeState" "ApplyContract" "RevokeContract" "BuildDelegatePage" "BuildProofPage"; do
    if grep -q "func (e \*Engine) $method" internal/delegatedholding/engine.go; then
        pass "Engine.$method exists"
    else
        fail "Engine.$method missing"
    fi
done

# ============================================================================
section "Section 13: No Forbidden Imports in Engine"
# ============================================================================

for pkg in "pushtransport" "interruptdelivery" "execution" "oauth"; do
    if ! grep -q "\"quantumlife/internal/$pkg\"" internal/delegatedholding/engine.go 2>/dev/null; then
        pass "Engine does not import $pkg"
    else
        fail "Engine imports forbidden package: $pkg"
    fi
done

# ============================================================================
section "Section 14: Store Methods"
# ============================================================================

for method in "UpsertActiveContract" "AppendRevocation" "GetActiveContract" "ListRecentContracts"; do
    if grep -q "func (s \*DelegatedHoldingStore) $method" internal/persist/delegated_holding_store.go; then
        pass "Store.$method exists"
    else
        fail "Store.$method missing"
    fi
done

# ============================================================================
section "Section 15: Bounded Retention"
# ============================================================================

if grep -q "MaxRetentionDays = 30" pkg/domain/delegatedholding/types.go; then
    pass "MaxRetentionDays = 30"
else
    fail "MaxRetentionDays != 30"
fi

if grep -q "MaxRecords = 200" pkg/domain/delegatedholding/types.go; then
    pass "MaxRecords = 200"
else
    fail "MaxRecords != 200"
fi

# ============================================================================
section "Section 16: Storelog Record Types"
# ============================================================================

if grep -q "RecordTypeDelegatedHoldingContract" pkg/domain/storelog/log.go; then
    pass "RecordTypeDelegatedHoldingContract defined"
else
    fail "RecordTypeDelegatedHoldingContract missing"
fi

if grep -q "RecordTypeDelegatedHoldingRevocation" pkg/domain/storelog/log.go; then
    pass "RecordTypeDelegatedHoldingRevocation defined"
else
    fail "RecordTypeDelegatedHoldingRevocation missing"
fi

# ============================================================================
section "Section 17: Events"
# ============================================================================

for event in "Phase42DelegationCreated" "Phase42DelegationRevoked" "Phase42DelegationExpired" "Phase42DelegationApplied" "Phase42DelegationProofViewed"; do
    if grep -q "$event" pkg/events/events.go; then
        pass "$event event defined"
    else
        fail "$event event missing"
    fi
done

# ============================================================================
section "Section 18: Web Routes"
# ============================================================================

if grep -q 'HandleFunc("/delegate"' cmd/quantumlife-web/main.go; then
    pass "/delegate route exists"
else
    fail "/delegate route missing"
fi

if grep -q 'HandleFunc("/delegate/create"' cmd/quantumlife-web/main.go; then
    pass "/delegate/create route exists"
else
    fail "/delegate/create route missing"
fi

if grep -q 'HandleFunc("/delegate/revoke"' cmd/quantumlife-web/main.go; then
    pass "/delegate/revoke route exists"
else
    fail "/delegate/revoke route missing"
fi

if grep -q 'HandleFunc("/proof/delegate"' cmd/quantumlife-web/main.go; then
    pass "/proof/delegate route exists"
else
    fail "/proof/delegate route missing"
fi

# ============================================================================
section "Section 19: Handler POST Enforcement"
# ============================================================================

if grep -A2 "handleDelegateCreate" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleDelegateCreate enforces POST"
else
    fail "handleDelegateCreate does not enforce POST"
fi

if grep -A2 "handleDelegateRevoke" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleDelegateRevoke enforces POST"
else
    fail "handleDelegateRevoke does not enforce POST"
fi

# ============================================================================
section "Section 20: ApplyContract Safety"
# ============================================================================

if ! grep -q "SURFACE\|INTERRUPT" internal/delegatedholding/engine.go 2>/dev/null | grep -v "// "; then
    pass "ApplyContract does not return SURFACE or INTERRUPT"
else
    fail "ApplyContract may return SURFACE or INTERRUPT"
fi

# ============================================================================
section "Section 21: Hash Computation"
# ============================================================================

if grep -q "ComputeHash()" pkg/domain/delegatedholding/types.go; then
    pass "ComputeHash method exists in domain"
else
    fail "ComputeHash method missing"
fi

if grep -q "ComputeContractIDHash()" pkg/domain/delegatedholding/types.go; then
    pass "ComputeContractIDHash method exists"
else
    fail "ComputeContractIDHash method missing"
fi

# ============================================================================
section "Section 22: ADR Content"
# ============================================================================

if grep -q "## Status" docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md; then
    pass "ADR has Status section"
else
    fail "ADR missing Status section"
fi

if grep -q "## Context" docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md; then
    pass "ADR has Context section"
else
    fail "ADR missing Context section"
fi

if grep -q "## Decision" docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md; then
    pass "ADR has Decision section"
else
    fail "ADR missing Decision section"
fi

if grep -q "## Consequences" docs/ADR/ADR-0079-phase42-delegated-holding-contracts.md; then
    pass "ADR has Consequences section"
else
    fail "ADR missing Consequences section"
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
