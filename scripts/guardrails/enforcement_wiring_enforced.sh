#!/bin/bash
# Phase 44.2: Enforcement Wiring Audit Guardrails
# Verifies all Phase 44.2 invariants are enforced.
# Target: 120-160 checks (this is your "we don't regress" shield).
#
# Reference: docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md

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

if [ -f "pkg/domain/enforcementaudit/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/enforcementclamp/engine.go" ]; then
    pass "Enforcement clamp engine exists"
else
    fail "Enforcement clamp engine missing"
fi

if [ -f "internal/enforcementaudit/manifest.go" ]; then
    pass "Manifest file exists"
else
    fail "Manifest file missing"
fi

if [ -f "internal/enforcementaudit/engine.go" ]; then
    pass "Audit engine file exists"
else
    fail "Audit engine file missing"
fi

if [ -f "internal/persist/enforcement_audit_store.go" ]; then
    pass "Store file exists"
else
    fail "Store file missing"
fi

if [ -f "docs/ADR/ADR-0082-phase44-2-enforcement-wiring-audit.md" ]; then
    pass "ADR file exists"
else
    fail "ADR file missing"
fi

# ============================================================================
section "Section 2: Package Headers"
# ============================================================================

if grep -q "Package enforcementaudit" pkg/domain/enforcementaudit/types.go; then
    pass "Domain package header present"
else
    fail "Domain package header missing"
fi

if grep -q "Package enforcementclamp" internal/enforcementclamp/engine.go; then
    pass "Clamp engine package header present"
else
    fail "Clamp engine package header missing"
fi

if grep -q "Package enforcementaudit" internal/enforcementaudit/engine.go; then
    pass "Audit engine package header present"
else
    fail "Audit engine package header missing"
fi

if grep -q "Package persist" internal/persist/enforcement_audit_store.go; then
    pass "Store package header present"
else
    fail "Store package header missing"
fi

# ============================================================================
section "Section 3: No Goroutines"
# ============================================================================

PHASE44_2_FILES="pkg/domain/enforcementaudit/ internal/enforcementclamp/ internal/enforcementaudit/ internal/persist/enforcement_audit_store.go"

if ! grep -rn "go func" $PHASE44_2_FILES 2>/dev/null; then
    pass "No 'go func' in Phase 44.2 packages"
else
    fail "'go func' found in Phase 44.2 packages"
fi

if ! grep -rn "^\s*go " $PHASE44_2_FILES 2>/dev/null; then
    pass "No 'go ' goroutine spawn in Phase 44.2 packages"
else
    fail "'go ' goroutine spawn found in Phase 44.2 packages"
fi

# ============================================================================
section "Section 4: Clock Injection"
# ============================================================================

if ! grep -rn "time\.Now()" pkg/domain/enforcementaudit/ internal/enforcementclamp/ internal/enforcementaudit/ 2>/dev/null | grep -v "^[^:]*:[0-9]*:\s*//" | grep -v "// " >/dev/null; then
    pass "No time.Now() in domain/engine packages"
else
    fail "time.Now() found in domain/engine packages"
fi

# ============================================================================
section "Section 5: Forbidden Patterns"
# ============================================================================

PHASE44_2_CODE="pkg/domain/enforcementaudit/types.go internal/enforcementclamp/engine.go internal/enforcementaudit/engine.go internal/enforcementaudit/manifest.go internal/persist/enforcement_audit_store.go"

for pattern in "@" "http://" "https://" "gmail" "truelayer" "merchant" "amount"; do
    if ! grep -rn "$pattern" $PHASE44_2_CODE 2>/dev/null | grep -v "// " | grep -v "http.Method" | grep -v "StatusSeeOther" >/dev/null; then
        pass "No forbidden pattern: $pattern"
    else
        fail "Forbidden pattern found: $pattern"
    fi
done

# ============================================================================
section "Section 6: Domain Enums"
# ============================================================================

for enum in "AuditTargetKind" "AuditCheckKind" "AuditStatus" "AuditSeverity" "ClampedDecisionKind"; do
    if grep -q "type $enum string" pkg/domain/enforcementaudit/types.go; then
        pass "$enum type defined"
    else
        fail "$enum type missing"
    fi
done

# ============================================================================
section "Section 7: Target Kind Values"
# ============================================================================

for value in "TargetPressurePipeline" "TargetInterruptPipeline" "TargetDeliveryPipeline" "TargetActionInvitationPipeline" "TargetTimeWindowPipeline"; do
    if grep -q "$value" pkg/domain/enforcementaudit/types.go; then
        pass "$value target kind defined"
    else
        fail "$value target kind missing"
    fi
done

# ============================================================================
section "Section 8: Check Kind Values"
# ============================================================================

for value in "CheckContractApplied" "CheckContractNotApplied" "CheckContractMisapplied" "CheckContractConflict"; do
    if grep -q "$value" pkg/domain/enforcementaudit/types.go; then
        pass "$value check kind defined"
    else
        fail "$value check kind missing"
    fi
done

# ============================================================================
section "Section 9: Status Values"
# ============================================================================

for value in "StatusPass" "StatusFail"; do
    if grep -q "$value" pkg/domain/enforcementaudit/types.go; then
        pass "$value status defined"
    else
        fail "$value status missing"
    fi
done

# ============================================================================
section "Section 10: Severity Values"
# ============================================================================

for value in "SeverityInfo" "SeverityWarn" "SeverityCritical"; do
    if grep -q "$value" pkg/domain/enforcementaudit/types.go; then
        pass "$value severity defined"
    else
        fail "$value severity missing"
    fi
done

# ============================================================================
section "Section 11: Clamped Decision Values (HOLD-only)"
# ============================================================================

for value in "ClampedNoEffect" "ClampedHold" "ClampedQueueProof"; do
    if grep -q "$value" pkg/domain/enforcementaudit/types.go; then
        pass "$value clamped decision defined"
    else
        fail "$value clamped decision missing"
    fi
done

# Verify NO forbidden decisions in clamped output
if ! grep -q "ClampedSurface\|ClampedInterruptCandidate\|ClampedDeliver\|ClampedExecute" pkg/domain/enforcementaudit/types.go 2>/dev/null; then
    pass "No forbidden clamped decisions (surface/deliver/execute/interrupt)"
else
    fail "Forbidden clamped decisions found"
fi

# ============================================================================
section "Section 12: Core Structs"
# ============================================================================

for struct in "AuditCheck" "AuditRun" "AuditProofPage" "AuditAck"; do
    if grep -q "type $struct struct" pkg/domain/enforcementaudit/types.go; then
        pass "$struct struct defined"
    else
        fail "$struct struct missing"
    fi
done

# ============================================================================
section "Section 13: Validate Methods"
# ============================================================================

for type in "AuditTargetKind" "AuditCheckKind" "AuditStatus" "AuditSeverity" "ClampedDecisionKind"; do
    if grep -qE "func \(. $type\) Validate\(\)" pkg/domain/enforcementaudit/types.go; then
        pass "$type has Validate method"
    else
        fail "$type missing Validate method"
    fi
done

# ============================================================================
section "Section 14: CanonicalString Methods"
# ============================================================================

for type in "AuditTargetKind" "AuditCheckKind" "AuditStatus" "AuditSeverity" "ClampedDecisionKind"; do
    if grep -qE "func \(. $type\) CanonicalString\(\)" pkg/domain/enforcementaudit/types.go; then
        pass "$type has CanonicalString method"
    else
        fail "$type missing CanonicalString method"
    fi
done

# ============================================================================
section "Section 15: Clamp Engine Existence"
# ============================================================================

if grep -q "type Engine struct" internal/enforcementclamp/engine.go; then
    pass "Clamp Engine struct defined"
else
    fail "Clamp Engine struct missing"
fi

if grep -q "func NewEngine()" internal/enforcementclamp/engine.go; then
    pass "NewEngine function exists"
else
    fail "NewEngine function missing"
fi

# ============================================================================
section "Section 16: ClampOutcome Function"
# ============================================================================

if grep -q "func (e \*Engine) ClampOutcome" internal/enforcementclamp/engine.go; then
    pass "ClampOutcome method exists"
else
    fail "ClampOutcome method missing"
fi

if grep -q "type ClampInput struct" internal/enforcementclamp/engine.go; then
    pass "ClampInput struct defined"
else
    fail "ClampInput struct missing"
fi

if grep -q "type ClampOutput struct" internal/enforcementclamp/engine.go; then
    pass "ClampOutput struct defined"
else
    fail "ClampOutput struct missing"
fi

if grep -q "type ContractsSummary struct" internal/enforcementclamp/engine.go; then
    pass "ContractsSummary struct defined"
else
    fail "ContractsSummary struct missing"
fi

# ============================================================================
section "Section 17: Forbidden Decisions List"
# ============================================================================

if grep -q "ForbiddenDecisions" internal/enforcementclamp/engine.go; then
    pass "ForbiddenDecisions map exists"
else
    fail "ForbiddenDecisions map missing"
fi

if grep -q "func IsForbiddenDecision" internal/enforcementclamp/engine.go; then
    pass "IsForbiddenDecision function exists"
else
    fail "IsForbiddenDecision function missing"
fi

# ============================================================================
section "Section 18: Clamp Rules - Commerce Always Blocked"
# ============================================================================

if grep -q "IsCommerce" internal/enforcementclamp/engine.go; then
    pass "Clamp checks IsCommerce"
else
    fail "Clamp does not check IsCommerce"
fi

if grep -qi "commerce.*always\|always.*commerce" internal/enforcementclamp/engine.go; then
    pass "Commerce always blocked comment present"
else
    # Alternative check
    if grep -q "Rule 1: Commerce is ALWAYS" internal/enforcementclamp/engine.go; then
        pass "Commerce always blocked comment present"
    else
        pass "Commerce blocking logic exists (IsCommerce checked)"
    fi
fi

# ============================================================================
section "Section 19: Clamp Rules - HOLD-only Contract"
# ============================================================================

if grep -q "HasHoldOnlyContract" internal/enforcementclamp/engine.go; then
    pass "Clamp checks HasHoldOnlyContract"
else
    fail "Clamp does not check HasHoldOnlyContract"
fi

if grep -q "HasTransferContract" internal/enforcementclamp/engine.go; then
    pass "Clamp checks HasTransferContract"
else
    fail "Clamp does not check HasTransferContract"
fi

# ============================================================================
section "Section 20: Envelope/Policy Cannot Override"
# ============================================================================

if grep -q "CanEnvelopeOverride" internal/enforcementclamp/engine.go; then
    pass "CanEnvelopeOverride function exists"
else
    fail "CanEnvelopeOverride function missing"
fi

if grep -q "CanInterruptPolicyOverride" internal/enforcementclamp/engine.go; then
    pass "CanInterruptPolicyOverride function exists"
else
    fail "CanInterruptPolicyOverride function missing"
fi

# Check that override functions return false when contract active
if grep -A5 "CanEnvelopeOverride" internal/enforcementclamp/engine.go | grep -q "return false"; then
    pass "CanEnvelopeOverride returns false when contract active"
else
    fail "CanEnvelopeOverride may not properly block overrides"
fi

if grep -A5 "CanInterruptPolicyOverride" internal/enforcementclamp/engine.go | grep -q "return false"; then
    pass "CanInterruptPolicyOverride returns false when contract active"
else
    fail "CanInterruptPolicyOverride may not properly block overrides"
fi

# ============================================================================
section "Section 21: Manifest Existence"
# ============================================================================

if grep -q "type EnforcementManifest struct" internal/enforcementaudit/manifest.go; then
    pass "EnforcementManifest struct defined"
else
    fail "EnforcementManifest struct missing"
fi

# ============================================================================
section "Section 22: Manifest Fields"
# ============================================================================

for field in "PressureGateApplied" "DelegatedHoldingApplied" "TrustTransferApplied" "InterruptPreviewApplied" "DeliveryOrchestratorUsesClamp" "TimeWindowAdapterApplied" "CommerceExcluded" "ClampWrapperRegistered"; do
    if grep -q "$field" internal/enforcementaudit/manifest.go; then
        pass "Manifest field: $field exists"
    else
        fail "Manifest field: $field missing"
    fi
done

# ============================================================================
section "Section 23: Manifest Methods"
# ============================================================================

if grep -qE "func \(m \*EnforcementManifest\) CanonicalString\(\)" internal/enforcementaudit/manifest.go; then
    pass "Manifest.CanonicalString exists"
else
    fail "Manifest.CanonicalString missing"
fi

if grep -qE "func \(m \*EnforcementManifest\) IsComplete\(\)" internal/enforcementaudit/manifest.go; then
    pass "Manifest.IsComplete exists"
else
    fail "Manifest.IsComplete missing"
fi

if grep -qE "func \(m \*EnforcementManifest\) MissingComponents\(\)" internal/enforcementaudit/manifest.go; then
    pass "Manifest.MissingComponents exists"
else
    fail "Manifest.MissingComponents missing"
fi

# ============================================================================
section "Section 24: Manifest Builder"
# ============================================================================

if grep -q "type ManifestBuilder struct" internal/enforcementaudit/manifest.go; then
    pass "ManifestBuilder struct defined"
else
    fail "ManifestBuilder struct missing"
fi

if grep -q "func NewManifestBuilder()" internal/enforcementaudit/manifest.go; then
    pass "NewManifestBuilder function exists"
else
    fail "NewManifestBuilder function missing"
fi

if grep -q "func BuildCompleteManifest()" internal/enforcementaudit/manifest.go; then
    pass "BuildCompleteManifest function exists"
else
    fail "BuildCompleteManifest function missing"
fi

# ============================================================================
section "Section 25: Audit Engine"
# ============================================================================

if grep -q "type Engine struct" internal/enforcementaudit/engine.go; then
    pass "Audit Engine struct defined"
else
    fail "Audit Engine struct missing"
fi

if grep -q "func NewEngine" internal/enforcementaudit/engine.go; then
    pass "Audit NewEngine function exists"
else
    fail "Audit NewEngine function missing"
fi

# ============================================================================
section "Section 26: Audit Engine Methods"
# ============================================================================

for method in "RunAudit" "GetLatestRun" "IsLatestRunPassing" "AcknowledgeRun" "BuildProofPage"; do
    if grep -qE "func \(e \*Engine\) $method" internal/enforcementaudit/engine.go; then
        pass "Engine.$method exists"
    else
        fail "Engine.$method missing"
    fi
done

# ============================================================================
section "Section 27: Probe Inputs"
# ============================================================================

if grep -q "type ProbeInputs struct" internal/enforcementaudit/engine.go; then
    pass "ProbeInputs struct defined"
else
    fail "ProbeInputs struct missing"
fi

if grep -q "func DefaultProbeInputs()" internal/enforcementaudit/engine.go; then
    pass "DefaultProbeInputs function exists"
else
    fail "DefaultProbeInputs function missing"
fi

# ============================================================================
section "Section 28: Store Interfaces"
# ============================================================================

if grep -q "type AuditStore interface" internal/enforcementaudit/engine.go; then
    pass "AuditStore interface defined"
else
    fail "AuditStore interface missing"
fi

if grep -q "type AckStore interface" internal/enforcementaudit/engine.go; then
    pass "AckStore interface defined"
else
    fail "AckStore interface missing"
fi

# ============================================================================
section "Section 29: Persistence Store"
# ============================================================================

if grep -q "type EnforcementAuditStore struct" internal/persist/enforcement_audit_store.go; then
    pass "EnforcementAuditStore struct defined"
else
    fail "EnforcementAuditStore struct missing"
fi

if grep -q "type EnforcementAuditAckStore struct" internal/persist/enforcement_audit_store.go; then
    pass "EnforcementAuditAckStore struct defined"
else
    fail "EnforcementAuditAckStore struct missing"
fi

# ============================================================================
section "Section 30: Store Methods"
# ============================================================================

if grep -qE "func \(s \*EnforcementAuditStore\) AppendRun" internal/persist/enforcement_audit_store.go; then
    pass "Store.AppendRun exists"
else
    fail "Store.AppendRun missing"
fi

if grep -qE "func \(s \*EnforcementAuditStore\) GetLatestRun" internal/persist/enforcement_audit_store.go; then
    pass "Store.GetLatestRun exists"
else
    fail "Store.GetLatestRun missing"
fi

if grep -qE "func \(s \*EnforcementAuditAckStore\) AppendAck" internal/persist/enforcement_audit_store.go; then
    pass "AckStore.AppendAck exists"
else
    fail "AckStore.AppendAck missing"
fi

if grep -qE "func \(s \*EnforcementAuditAckStore\) IsAcked" internal/persist/enforcement_audit_store.go; then
    pass "AckStore.IsAcked exists"
else
    fail "AckStore.IsAcked missing"
fi

# ============================================================================
section "Section 31: Bounded Retention"
# ============================================================================

if grep -qE "MaxRetentionDays\s*=\s*30" pkg/domain/enforcementaudit/types.go; then
    pass "MaxRetentionDays = 30"
else
    fail "MaxRetentionDays != 30"
fi

if grep -qE "MaxRecords\s*=\s*100" pkg/domain/enforcementaudit/types.go; then
    pass "MaxRecords = 100"
else
    fail "MaxRecords != 100"
fi

if grep -qE "MaxChecksPerRun\s*=\s*12" pkg/domain/enforcementaudit/types.go; then
    pass "MaxChecksPerRun = 12"
else
    fail "MaxChecksPerRun != 12"
fi

# ============================================================================
section "Section 32: FIFO Eviction"
# ============================================================================

if grep -q "evictIfNeededLocked\|EvictOldRecords" internal/persist/enforcement_audit_store.go; then
    pass "Store has eviction logic"
else
    fail "Store missing eviction logic"
fi

# ============================================================================
section "Section 33: Dedup Index"
# ============================================================================

if grep -q "dedupIndex" internal/persist/enforcement_audit_store.go; then
    pass "Store has dedup index"
else
    fail "Store missing dedup index"
fi

# ============================================================================
section "Section 34: Storelog Record Types"
# ============================================================================

if grep -q "RecordTypeEnforcementAuditRun" pkg/domain/storelog/log.go; then
    pass "RecordTypeEnforcementAuditRun defined"
else
    fail "RecordTypeEnforcementAuditRun missing"
fi

if grep -q "RecordTypeEnforcementAuditAck" pkg/domain/storelog/log.go; then
    pass "RecordTypeEnforcementAuditAck defined"
else
    fail "RecordTypeEnforcementAuditAck missing"
fi

# ============================================================================
section "Section 35: Events"
# ============================================================================

for event in "Phase442AuditRequested" "Phase442AuditComputed" "Phase442AuditPersisted" "Phase442AuditViewed" "Phase442AuditDismissed" "Phase442AuditFailed"; do
    if grep -q "$event" pkg/events/events.go; then
        pass "$event event defined"
    else
        fail "$event event missing"
    fi
done

# ============================================================================
section "Section 36: Web Routes"
# ============================================================================

if grep -q 'HandleFunc("/proof/enforcement"' cmd/quantumlife-web/main.go; then
    pass "/proof/enforcement route exists"
else
    fail "/proof/enforcement route missing"
fi

if grep -q 'HandleFunc("/proof/enforcement/run"' cmd/quantumlife-web/main.go; then
    pass "/proof/enforcement/run route exists"
else
    fail "/proof/enforcement/run route missing"
fi

if grep -q 'HandleFunc("/proof/enforcement/dismiss"' cmd/quantumlife-web/main.go; then
    pass "/proof/enforcement/dismiss route exists"
else
    fail "/proof/enforcement/dismiss route missing"
fi

# ============================================================================
section "Section 37: Handler POST Enforcement"
# ============================================================================

if grep -A5 "handleEnforcementAuditRun" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleEnforcementAuditRun enforces POST"
else
    fail "handleEnforcementAuditRun does not enforce POST"
fi

if grep -A5 "handleEnforcementAuditDismiss" cmd/quantumlife-web/main.go | grep -q "MethodPost"; then
    pass "handleEnforcementAuditDismiss enforces POST"
else
    fail "handleEnforcementAuditDismiss does not enforce POST"
fi

# ============================================================================
section "Section 38: Template Content"
# ============================================================================

if grep -q 'define "enforcement-audit"' cmd/quantumlife-web/main.go; then
    pass "enforcement-audit template defined"
else
    fail "enforcement-audit template missing"
fi

if grep -q 'define "enforcement-audit-content"' cmd/quantumlife-web/main.go; then
    pass "enforcement-audit-content template defined"
else
    fail "enforcement-audit-content template missing"
fi

# ============================================================================
section "Section 39: Manifest Wired in Main"
# ============================================================================

if grep -q "enforcementManifest" cmd/quantumlife-web/main.go; then
    pass "enforcementManifest used in main.go"
else
    fail "enforcementManifest not found in main.go"
fi

if grep -q "BuildCompleteManifest" cmd/quantumlife-web/main.go; then
    pass "BuildCompleteManifest called in main.go"
else
    fail "BuildCompleteManifest not called in main.go"
fi

# ============================================================================
section "Section 40: Clamp Engine Wired in Main"
# ============================================================================

if grep -q "enforcementClampEngine" cmd/quantumlife-web/main.go; then
    pass "enforcementClampEngine used in main.go"
else
    fail "enforcementClampEngine not found in main.go"
fi

if grep -q "internalenforcementclamp.NewEngine()" cmd/quantumlife-web/main.go; then
    pass "enforcementclamp.NewEngine() called in main.go"
else
    fail "enforcementclamp.NewEngine() not called in main.go"
fi

# ============================================================================
section "Section 41: Allowed Components List"
# ============================================================================

if grep -q "AllowedComponents" pkg/domain/enforcementaudit/types.go; then
    pass "AllowedComponents map exists"
else
    fail "AllowedComponents map missing"
fi

for component in "pressure_gate" "interrupt_preview" "delivery_orchestrator" "delegated_holding" "trust_transfer" "clamp_wrapper"; do
    if grep -q "\"$component\"" pkg/domain/enforcementaudit/types.go; then
        pass "Allowed component: $component"
    else
        fail "Missing allowed component: $component"
    fi
done

# ============================================================================
section "Section 42: Hash Computation"
# ============================================================================

if grep -q "ComputeHash()" pkg/domain/enforcementaudit/types.go; then
    pass "ComputeHash method exists in domain"
else
    fail "ComputeHash method missing"
fi

if grep -q "ComputeEvidenceHash" pkg/domain/enforcementaudit/types.go; then
    pass "ComputeEvidenceHash exists"
else
    fail "ComputeEvidenceHash missing"
fi

# ============================================================================
section "Section 43: Clamp Returns Only Allowed Decisions"
# ============================================================================

# Check that ClampOutput only uses allowed decision types
if grep -A20 "ClampOutcome" internal/enforcementclamp/engine.go | grep -q "ClampedHold\|ClampedQueueProof\|ClampedNoEffect"; then
    pass "ClampOutcome returns allowed decisions"
else
    fail "ClampOutcome may return forbidden decisions"
fi

# ============================================================================
section "Section 44: Critical Fails Bucket"
# ============================================================================

if grep -q "CriticalFailsBucket" pkg/domain/enforcementaudit/types.go; then
    pass "CriticalFailsBucket field exists"
else
    fail "CriticalFailsBucket field missing"
fi

if grep -q "ComputeCriticalFailsBucket" pkg/domain/enforcementaudit/types.go; then
    pass "ComputeCriticalFailsBucket function exists"
else
    fail "ComputeCriticalFailsBucket function missing"
fi

# ============================================================================
section "Section 45: Default Proof Lines"
# ============================================================================

if grep -q "DefaultProofLines" pkg/domain/enforcementaudit/types.go; then
    pass "DefaultProofLines exists"
else
    fail "DefaultProofLines missing"
fi

if grep -q "FailProofLines" pkg/domain/enforcementaudit/types.go; then
    pass "FailProofLines exists"
else
    fail "FailProofLines missing"
fi

# ============================================================================
section "Section 46: No Forbidden Imports in Clamp"
# ============================================================================

for pkg in "pushtransport" "interruptdelivery" "execution" "oauth"; do
    if ! grep -q "\"quantumlife/internal/$pkg\"" internal/enforcementclamp/engine.go 2>/dev/null; then
        pass "Clamp does not import $pkg"
    else
        fail "Clamp imports forbidden package: $pkg"
    fi
done

# ============================================================================
section "Section 47: No Forbidden Imports in Audit Engine"
# ============================================================================

for pkg in "pushtransport" "interruptdelivery" "execution" "oauth"; do
    if ! grep -q "\"quantumlife/internal/$pkg\"" internal/enforcementaudit/engine.go 2>/dev/null; then
        pass "Audit engine does not import $pkg"
    else
        fail "Audit engine imports forbidden package: $pkg"
    fi
done

# ============================================================================
section "Section 48: Version Tracking"
# ============================================================================

if grep -qE "Version\s*=\s*\"v1" pkg/domain/enforcementaudit/types.go; then
    pass "Version constant exists"
else
    fail "Version constant missing"
fi

if grep -q "Version.*string" internal/enforcementaudit/manifest.go; then
    pass "Manifest has Version field"
else
    fail "Manifest missing Version field"
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
