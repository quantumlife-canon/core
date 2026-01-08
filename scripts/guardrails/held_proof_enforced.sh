#!/bin/bash
# Phase 43: Held Under Agreement Proof Ledger Guardrails
# Verifies all Phase 43 invariants are enforced.
#
# Reference: docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md

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

if [ -f "pkg/domain/heldproof/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/heldproof/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/held_proof_store.go" ]; then
    pass "Store file exists"
else
    fail "Store file missing"
fi

if [ -f "docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md" ]; then
    pass "ADR file exists"
else
    fail "ADR file missing"
fi

# ============================================================================
section "Section 2: Package Headers"
# ============================================================================

if grep -q "Package heldproof" pkg/domain/heldproof/types.go; then
    pass "Domain package header present"
else
    fail "Domain package header missing"
fi

if grep -q "Package heldproof" internal/heldproof/engine.go; then
    pass "Engine package header present"
else
    fail "Engine package header missing"
fi

if grep -q "Package persist" internal/persist/held_proof_store.go; then
    pass "Store package header present"
else
    fail "Store package header missing"
fi

# ============================================================================
section "Section 3: No Goroutines"
# ============================================================================

if ! grep -rn "go func" pkg/domain/heldproof/ internal/heldproof/ internal/persist/held_proof_store.go 2>/dev/null; then
    pass "No 'go func' in Phase 43 packages"
else
    fail "'go func' found in Phase 43 packages"
fi

if ! grep -rn "^\s*go " pkg/domain/heldproof/ internal/heldproof/ internal/persist/held_proof_store.go 2>/dev/null; then
    pass "No 'go ' goroutine spawn in Phase 43 packages"
else
    fail "'go ' goroutine spawn found in Phase 43 packages"
fi

# ============================================================================
section "Section 4: Clock Injection"
# ============================================================================

if ! grep -rn "time\.Now()" pkg/domain/heldproof/ internal/heldproof/ 2>/dev/null | grep -v "^[^:]*:[0-9]*:\s*//" | grep -v "// " >/dev/null; then
    pass "No time.Now() in domain/engine packages"
else
    fail "time.Now() found in domain/engine packages"
fi

# ============================================================================
section "Section 5: Forbidden Patterns"
# ============================================================================

PHASE43_FILES="pkg/domain/heldproof/types.go internal/heldproof/engine.go internal/persist/held_proof_store.go"

for pattern in "@" "http://" "https://" "£" "\\$[0-9]" "gmail" "truelayer" "merchant" "amount" "user" "users"; do
    if ! grep -rn "$pattern" $PHASE43_FILES 2>/dev/null | grep -v "// " | grep -v "http.Method" | grep -v "StatusSeeOther" >/dev/null; then
        pass "No forbidden pattern: $pattern"
    else
        fail "Forbidden pattern found: $pattern"
    fi
done

# ============================================================================
section "Section 6: Domain Enums"
# ============================================================================

for enum in "HeldProofKind" "HeldProofMagnitudeBucket" "HeldProofHorizonBucket" "HeldProofCircleType" "HeldProofAckKind"; do
    if grep -q "type $enum string" pkg/domain/heldproof/types.go; then
        pass "$enum type defined"
    else
        fail "$enum type missing"
    fi
done

# ============================================================================
section "Section 7: Enum Values"
# ============================================================================

for value in "KindDelegatedHolding" "MagnitudeNothing" "MagnitudeAFew" "MagnitudeSeveral" "HorizonNow" "HorizonSoon" "HorizonLater" "CircleTypeHuman" "CircleTypeInstitution" "CircleTypeCommerce" "AckViewed" "AckDismissed"; do
    if grep -q "$value" pkg/domain/heldproof/types.go; then
        pass "$value enum value defined"
    else
        fail "$value enum value missing"
    fi
done

# ============================================================================
section "Section 8: Validate Methods"
# ============================================================================

for type in "HeldProofKind" "HeldProofMagnitudeBucket" "HeldProofHorizonBucket" "HeldProofCircleType" "HeldProofAckKind"; do
    if grep -q "func (. $type) Validate()" pkg/domain/heldproof/types.go; then
        pass "$type has Validate method"
    else
        fail "$type missing Validate method"
    fi
done

# ============================================================================
section "Section 9: CanonicalString Methods"
# ============================================================================

for type in "HeldProofKind" "HeldProofMagnitudeBucket" "HeldProofHorizonBucket" "HeldProofCircleType" "HeldProofAckKind"; do
    if grep -q "func (. $type) CanonicalString()" pkg/domain/heldproof/types.go; then
        pass "$type has CanonicalString method"
    else
        fail "$type missing CanonicalString method"
    fi
done

# ============================================================================
section "Section 10: Core Types"
# ============================================================================

for type in "HeldProofPeriod" "HeldProofSignal" "HeldProofPage" "HeldProofAck" "HeldProofCue"; do
    if grep -q "type $type struct" pkg/domain/heldproof/types.go; then
        pass "$type struct defined"
    else
        fail "$type struct missing"
    fi
done

# ============================================================================
section "Section 11: Commerce Exclusion"
# ============================================================================

if grep -q "IsCommerce()" pkg/domain/heldproof/types.go; then
    pass "IsCommerce method exists"
else
    fail "IsCommerce method missing"
fi

if grep -q "CircleType.IsCommerce()" internal/heldproof/engine.go; then
    pass "Engine checks IsCommerce"
else
    fail "Engine does not check IsCommerce"
fi

# ============================================================================
section "Section 12: Engine Interface Methods"
# ============================================================================

for iface in "SignalStore" "AckStore" "Clock"; do
    if grep -q "type $iface interface" internal/heldproof/engine.go; then
        pass "$iface interface defined"
    else
        fail "$iface interface missing"
    fi
done

# ============================================================================
section "Section 13: Engine Methods"
# ============================================================================

for method in "BuildSignals" "BuildSignalFromDecision" "BuildPage" "BuildCue" "PersistSignal" "LoadSignals" "RecordViewed" "RecordDismissed"; do
    if grep -q "func (e \*Engine) $method" internal/heldproof/engine.go; then
        pass "Engine.$method exists"
    else
        fail "Engine.$method missing"
    fi
done

# ============================================================================
section "Section 14: No Forbidden Imports in Engine"
# ============================================================================

for pkg in "pushtransport" "interruptdelivery" "execution" "oauth"; do
    if ! grep -q "\"quantumlife/internal/$pkg\"" internal/heldproof/engine.go 2>/dev/null; then
        pass "Engine does not import $pkg"
    else
        fail "Engine imports forbidden package: $pkg"
    fi
done

# ============================================================================
section "Section 15: Store Methods"
# ============================================================================

if grep -q "func (s \*HeldProofSignalStore) AppendSignal" internal/persist/held_proof_store.go; then
    pass "Store.AppendSignal exists"
else
    fail "Store.AppendSignal missing"
fi

if grep -q "func (s \*HeldProofSignalStore) ListSignals" internal/persist/held_proof_store.go; then
    pass "Store.ListSignals exists"
else
    fail "Store.ListSignals missing"
fi

if grep -q "func (s \*HeldProofAckStore) RecordViewed" internal/persist/held_proof_store.go; then
    pass "Store.RecordViewed exists"
else
    fail "Store.RecordViewed missing"
fi

if grep -q "func (s \*HeldProofAckStore) RecordDismissed" internal/persist/held_proof_store.go; then
    pass "Store.RecordDismissed exists"
else
    fail "Store.RecordDismissed missing"
fi

# ============================================================================
section "Section 16: Bounded Retention"
# ============================================================================

if grep -q "MaxRetentionDays.*=.*30" pkg/domain/heldproof/types.go; then
    pass "MaxRetentionDays = 30"
else
    fail "MaxRetentionDays != 30"
fi

if grep -q "MaxSignalRecords.*=.*500" pkg/domain/heldproof/types.go; then
    pass "MaxSignalRecords = 500"
else
    fail "MaxSignalRecords != 500"
fi

if grep -q "MaxAckRecords.*=.*200" pkg/domain/heldproof/types.go; then
    pass "MaxAckRecords = 200"
else
    fail "MaxAckRecords != 200"
fi

if grep -q "MaxSignalsPerPage.*=.*3" pkg/domain/heldproof/types.go; then
    pass "MaxSignalsPerPage = 3"
else
    fail "MaxSignalsPerPage != 3"
fi

# ============================================================================
section "Section 17: Storelog Record Types"
# ============================================================================

if grep -q "RecordTypeHeldProofSignal" pkg/domain/storelog/log.go; then
    pass "RecordTypeHeldProofSignal defined"
else
    fail "RecordTypeHeldProofSignal missing"
fi

if grep -q "RecordTypeHeldProofAck" pkg/domain/storelog/log.go; then
    pass "RecordTypeHeldProofAck defined"
else
    fail "RecordTypeHeldProofAck missing"
fi

# ============================================================================
section "Section 18: Events"
# ============================================================================

for event in "Phase43HeldProofSignalPersisted" "Phase43HeldProofPageRendered" "Phase43HeldProofCueComputed" "Phase43HeldProofAckViewed" "Phase43HeldProofAckDismissed"; do
    if grep -q "$event" pkg/events/events.go; then
        pass "$event event defined"
    else
        fail "$event event missing"
    fi
done

# ============================================================================
section "Section 19: Web Routes"
# ============================================================================

if grep -q 'HandleFunc("/proof/held"' cmd/quantumlife-web/main.go; then
    pass "/proof/held route exists"
else
    fail "/proof/held route missing"
fi

if grep -q 'HandleFunc("/proof/held/dismiss"' cmd/quantumlife-web/main.go; then
    pass "/proof/held/dismiss route exists"
else
    fail "/proof/held/dismiss route missing"
fi

# ============================================================================
section "Section 20: Handler POST Enforcement"
# ============================================================================

if grep -A2 "handleHeldProofDismiss" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleHeldProofDismiss enforces POST"
else
    fail "handleHeldProofDismiss does not enforce POST"
fi

# ============================================================================
section "Section 21: Hash Computation"
# ============================================================================

if grep -q "ComputeHash()" pkg/domain/heldproof/types.go; then
    pass "ComputeHash method exists in domain"
else
    fail "ComputeHash method missing"
fi

if grep -q "ComputeEvidenceHash" pkg/domain/heldproof/types.go; then
    pass "ComputeEvidenceHash function exists"
else
    fail "ComputeEvidenceHash function missing"
fi

# ============================================================================
section "Section 22: Phase 42 Integration"
# ============================================================================

if grep -q "Phase42QueueProofOutcome" internal/heldproof/engine.go; then
    pass "Phase42QueueProofOutcome type exists"
else
    fail "Phase42QueueProofOutcome type missing"
fi

if grep -q "HandleQueueProofOutcome" internal/heldproof/engine.go; then
    pass "HandleQueueProofOutcome method exists"
else
    fail "HandleQueueProofOutcome method missing"
fi

# ============================================================================
section "Section 23: ADR Content"
# ============================================================================

if grep -q "## Status" docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md 2>/dev/null; then
    pass "ADR has Status section"
else
    fail "ADR missing Status section"
fi

if grep -q "## Context" docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md 2>/dev/null; then
    pass "ADR has Context section"
else
    fail "ADR missing Context section"
fi

if grep -q "## Decision" docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md 2>/dev/null; then
    pass "ADR has Decision section"
else
    fail "ADR missing Decision section"
fi

if grep -q "## Consequences" docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md 2>/dev/null; then
    pass "ADR has Consequences section"
else
    fail "ADR missing Consequences section"
fi

# ============================================================================
section "Section 24: Template Exists"
# ============================================================================

if grep -q 'define "held-proof"' cmd/quantumlife-web/main.go; then
    pass "held-proof template defined"
else
    fail "held-proof template missing"
fi

if grep -q 'define "held-proof-content"' cmd/quantumlife-web/main.go; then
    pass "held-proof-content template defined"
else
    fail "held-proof-content template missing"
fi

# ============================================================================
section "Section 25: Dedup by Evidence Hash"
# ============================================================================

if grep -q "dedupIndex" internal/persist/held_proof_store.go; then
    pass "Dedup index in store"
else
    fail "Dedup index missing"
fi

if grep -q "dedupKey" internal/persist/held_proof_store.go; then
    pass "Dedup key logic exists"
else
    fail "Dedup key logic missing"
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
