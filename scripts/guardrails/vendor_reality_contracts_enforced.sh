#!/bin/bash
# Phase 49: Vendor Reality Contracts Guardrails
# Ensures all Phase 49 invariants are enforced.
#
# CRITICAL: Contracts can only REDUCE pressure, never increase it.
# CRITICAL: Commerce vendors capped at SURFACE_ONLY regardless of declaration.
# CRITICAL: Hash-only storage - no vendor names, emails, URLs.
# CRITICAL: Single choke-point clamp integration.
# CRITICAL: No time.Now() in pkg/ or internal/.
# CRITICAL: No goroutines in pkg/ or internal/.
#
# Reference: docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md

set -e

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo "  ✓ $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo "  ✗ $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

check() {
    local desc="$1"
    shift
    if "$@" > /dev/null 2>&1; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

check_not() {
    local desc="$1"
    shift
    if ! "$@" > /dev/null 2>&1; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

echo "=============================================="
echo "Phase 49: Vendor Reality Contracts Guardrails"
echo "=============================================="
echo ""

# ============================================================================
# Section 1: Required Files Exist
# ============================================================================
echo "Section 1: Required Files Exist"

check "ADR-0087 exists" test -f docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "Domain types exist" test -f pkg/domain/vendorcontract/types.go
check "Engine exists" test -f internal/vendorcontract/engine.go
check "Contract store exists" test -f internal/persist/vendor_contract_store.go
check "Demo tests exist" test -f internal/demo_phase49_vendor_contracts/demo_test.go

echo ""

# ============================================================================
# Section 2: Domain Enums - ContractScope
# ============================================================================
echo "Section 2: Domain Enums - ContractScope"

check "ContractScope type defined" grep -q "type ContractScope string" pkg/domain/vendorcontract/types.go
check "scope_commerce defined" grep -q 'ScopeCommerce.*scope_commerce' pkg/domain/vendorcontract/types.go
check "scope_institution defined" grep -q 'ScopeInstitution.*scope_institution' pkg/domain/vendorcontract/types.go
check "scope_health defined" grep -q 'ScopeHealth.*scope_health' pkg/domain/vendorcontract/types.go
check "scope_transport defined" grep -q 'ScopeTransport.*scope_transport' pkg/domain/vendorcontract/types.go
check "scope_unknown defined" grep -q 'ScopeUnknown.*scope_unknown' pkg/domain/vendorcontract/types.go
check "ContractScope.Validate exists" grep -q "func (s ContractScope) Validate()" pkg/domain/vendorcontract/types.go
check "ContractScope.CanonicalString exists" grep -q "func (s ContractScope) CanonicalString()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 3: Domain Enums - PressureAllowance
# ============================================================================
echo "Section 3: Domain Enums - PressureAllowance"

check "PressureAllowance type defined" grep -q "type PressureAllowance string" pkg/domain/vendorcontract/types.go
check "allow_hold_only defined" grep -q 'AllowHoldOnly.*allow_hold_only' pkg/domain/vendorcontract/types.go
check "allow_surface_only defined" grep -q 'AllowSurfaceOnly.*allow_surface_only' pkg/domain/vendorcontract/types.go
check "allow_interrupt_candidate defined" grep -q 'AllowInterruptCandidate.*allow_interrupt_candidate' pkg/domain/vendorcontract/types.go
check "PressureAllowance.Validate exists" grep -q "func (p PressureAllowance) Validate()" pkg/domain/vendorcontract/types.go
check "PressureAllowance.CanonicalString exists" grep -q "func (p PressureAllowance) CanonicalString()" pkg/domain/vendorcontract/types.go
check "PressureAllowance.Level exists" grep -q "func (p PressureAllowance) Level()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 4: Domain Enums - FrequencyBucket
# ============================================================================
echo "Section 4: Domain Enums - FrequencyBucket"

check "FrequencyBucket type defined" grep -q "type FrequencyBucket string" pkg/domain/vendorcontract/types.go
check "freq_per_day defined" grep -q 'FreqPerDay.*freq_per_day' pkg/domain/vendorcontract/types.go
check "freq_per_week defined" grep -q 'FreqPerWeek.*freq_per_week' pkg/domain/vendorcontract/types.go
check "freq_per_event defined" grep -q 'FreqPerEvent.*freq_per_event' pkg/domain/vendorcontract/types.go
check "FrequencyBucket.Validate exists" grep -q "func (f FrequencyBucket) Validate()" pkg/domain/vendorcontract/types.go
check "FrequencyBucket.CanonicalString exists" grep -q "func (f FrequencyBucket) CanonicalString()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 5: Domain Enums - EmergencyBucket
# ============================================================================
echo "Section 5: Domain Enums - EmergencyBucket"

check "EmergencyBucket type defined" grep -q "type EmergencyBucket string" pkg/domain/vendorcontract/types.go
check "emergency_none defined" grep -q 'EmergencyNone.*emergency_none' pkg/domain/vendorcontract/types.go
check "emergency_human_only defined" grep -q 'EmergencyHumanOnly.*emergency_human_only' pkg/domain/vendorcontract/types.go
check "emergency_institution_only defined" grep -q 'EmergencyInstitutionOnly.*emergency_institution_only' pkg/domain/vendorcontract/types.go
check "EmergencyBucket.Validate exists" grep -q "func (e EmergencyBucket) Validate()" pkg/domain/vendorcontract/types.go
check "EmergencyBucket.CanonicalString exists" grep -q "func (e EmergencyBucket) CanonicalString()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 6: Domain Enums - DeclaredByKind
# ============================================================================
echo "Section 6: Domain Enums - DeclaredByKind"

check "DeclaredByKind type defined" grep -q "type DeclaredByKind string" pkg/domain/vendorcontract/types.go
check "declared_vendor_self defined" grep -q 'DeclaredVendorSelf.*declared_vendor_self' pkg/domain/vendorcontract/types.go
check "declared_regulator defined" grep -q 'DeclaredRegulator.*declared_regulator' pkg/domain/vendorcontract/types.go
check "declared_marketplace defined" grep -q 'DeclaredMarketplace.*declared_marketplace' pkg/domain/vendorcontract/types.go
check "DeclaredByKind.Validate exists" grep -q "func (d DeclaredByKind) Validate()" pkg/domain/vendorcontract/types.go
check "DeclaredByKind.CanonicalString exists" grep -q "func (d DeclaredByKind) CanonicalString()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 7: Domain Enums - ContractStatus
# ============================================================================
echo "Section 7: Domain Enums - ContractStatus"

check "ContractStatus type defined" grep -q "type ContractStatus string" pkg/domain/vendorcontract/types.go
check "status_active defined" grep -q 'StatusActive.*status_active' pkg/domain/vendorcontract/types.go
check "status_revoked defined" grep -q 'StatusRevoked.*status_revoked' pkg/domain/vendorcontract/types.go
check "ContractStatus.Validate exists" grep -q "func (c ContractStatus) Validate()" pkg/domain/vendorcontract/types.go
check "ContractStatus.CanonicalString exists" grep -q "func (c ContractStatus) CanonicalString()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 8: Domain Enums - ContractReasonBucket
# ============================================================================
echo "Section 8: Domain Enums - ContractReasonBucket"

check "ContractReasonBucket type defined" grep -q "type ContractReasonBucket string" pkg/domain/vendorcontract/types.go
check "reason_ok defined" grep -q 'ReasonOK.*reason_ok' pkg/domain/vendorcontract/types.go
check "reason_invalid defined" grep -q 'ReasonInvalid.*reason_invalid' pkg/domain/vendorcontract/types.go
check "reason_commerce_capped defined" grep -q 'ReasonCommerceCapped.*reason_commerce_capped' pkg/domain/vendorcontract/types.go
check "reason_no_power defined" grep -q 'ReasonNoPower.*reason_no_power' pkg/domain/vendorcontract/types.go
check "reason_rejected defined" grep -q 'ReasonRejected.*reason_rejected' pkg/domain/vendorcontract/types.go
check "ContractReasonBucket.Validate exists" grep -q "func (r ContractReasonBucket) Validate()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 9: Domain Structs
# ============================================================================
echo "Section 9: Domain Structs"

check "VendorContract struct exists" grep -q "type VendorContract struct" pkg/domain/vendorcontract/types.go
check "VendorContract.CanonicalString exists" grep -q "func (c VendorContract) CanonicalString()" pkg/domain/vendorcontract/types.go
check "VendorContract.Validate exists" grep -q "func (c VendorContract) Validate()" pkg/domain/vendorcontract/types.go
check "VendorContractRecord struct exists" grep -q "type VendorContractRecord struct" pkg/domain/vendorcontract/types.go
check "VendorContractRecord.CanonicalString exists" grep -q "func (r VendorContractRecord) CanonicalString()" pkg/domain/vendorcontract/types.go
check "VendorContractOutcome struct exists" grep -q "type VendorContractOutcome struct" pkg/domain/vendorcontract/types.go
check "VendorContractOutcome.CanonicalString exists" grep -q "func (o VendorContractOutcome) CanonicalString()" pkg/domain/vendorcontract/types.go
check "VendorContractProofLine struct exists" grep -q "type VendorContractProofLine struct" pkg/domain/vendorcontract/types.go
check "VendorContractProofLine.CanonicalString exists" grep -q "func (p VendorContractProofLine) CanonicalString()" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 10: Domain Hash Helpers
# ============================================================================
echo "Section 10: Domain Hash Helpers"

check "HashContractString exists" grep -q "func HashContractString" pkg/domain/vendorcontract/types.go
check "Uses sha256" grep -q "sha256" pkg/domain/vendorcontract/types.go
check "Uses hex encoding" grep -q "hex.EncodeToString" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 11: Engine Methods
# ============================================================================
echo "Section 11: Engine Methods"

check "Engine struct exists" grep -q "type Engine struct" internal/vendorcontract/engine.go
check "NewEngine exists" grep -q "func NewEngine" internal/vendorcontract/engine.go
check "ValidateContract exists" grep -q "func (e \*Engine) ValidateContract" internal/vendorcontract/engine.go
check "ComputeEffectiveCap exists" grep -q "func (e \*Engine) ComputeEffectiveCap" internal/vendorcontract/engine.go
check "DecideOutcome exists" grep -q "func (e \*Engine) DecideOutcome" internal/vendorcontract/engine.go
check "ClampPressureAllowance exists" grep -q "func (e \*Engine) ClampPressureAllowance" internal/vendorcontract/engine.go
check "BuildProofLine exists" grep -q "func (e \*Engine) BuildProofLine" internal/vendorcontract/engine.go
check "BuildProofPage exists" grep -q "func (e \*Engine) BuildProofPage" internal/vendorcontract/engine.go
check "BuildCue exists" grep -q "func (e \*Engine) BuildCue" internal/vendorcontract/engine.go

echo ""

# ============================================================================
# Section 12: Commerce Cap Rule
# ============================================================================
echo "Section 12: Commerce Cap Rule"

check "Commerce cap logic in engine" grep -q "ScopeCommerce" internal/vendorcontract/engine.go
check "Commerce returns surface_only max" grep -q "AllowSurfaceOnly" internal/vendorcontract/engine.go
check "Commerce capped reason" grep -q "ReasonCommerceCapped" internal/vendorcontract/engine.go

echo ""

# ============================================================================
# Section 13: Persistence Store
# ============================================================================
echo "Section 13: Persistence Store"

check "VendorContractStore exists" grep -q "type VendorContractStore struct" internal/persist/vendor_contract_store.go
check "NewVendorContractStore exists" grep -q "func NewVendorContractStore" internal/persist/vendor_contract_store.go
check "UpsertActiveContract exists" grep -q "func (s \*VendorContractStore) UpsertActiveContract" internal/persist/vendor_contract_store.go
check "RevokeContract exists" grep -q "func (s \*VendorContractStore) RevokeContract" internal/persist/vendor_contract_store.go
check "GetActiveContract exists" grep -q "func (s \*VendorContractStore) GetActiveContract" internal/persist/vendor_contract_store.go
check "ListByPeriod exists" grep -q "func (s \*VendorContractStore) ListByPeriod" internal/persist/vendor_contract_store.go
check "VendorProofAckStore exists" grep -q "type VendorProofAckStore struct" internal/persist/vendor_contract_store.go
check "NewVendorProofAckStore exists" grep -q "func NewVendorProofAckStore" internal/persist/vendor_contract_store.go

echo ""

# ============================================================================
# Section 14: Bounded Retention
# ============================================================================
echo "Section 14: Bounded Retention"

check "MaxVendorContractRecords constant" grep -q "MaxVendorContractRecords" pkg/domain/vendorcontract/types.go
check "Max records is 200" grep -q "MaxVendorContractRecords.*=.*200" pkg/domain/vendorcontract/types.go
check "MaxVendorContractDays constant" grep -q "MaxVendorContractDays" pkg/domain/vendorcontract/types.go
check "Max days is 30" grep -q "MaxVendorContractDays.*=.*30" pkg/domain/vendorcontract/types.go
check "Eviction logic in store" grep -q "evictOldRecordsLocked" internal/persist/vendor_contract_store.go

echo ""

# ============================================================================
# Section 15: Events
# ============================================================================
echo "Section 15: Events"

check "Phase49VendorContractDeclared event" grep -q 'Phase49VendorContractDeclared.*=.*"phase49.vendor_contract.declared"' pkg/events/events.go
check "Phase49VendorContractRevoked event" grep -q 'Phase49VendorContractRevoked.*=.*"phase49.vendor_contract.revoked"' pkg/events/events.go
check "Phase49VendorContractApplied event" grep -q 'Phase49VendorContractApplied.*=.*"phase49.vendor_contract.applied"' pkg/events/events.go
check "Phase49VendorContractClamped event" grep -q 'Phase49VendorContractClamped.*=.*"phase49.vendor_contract.clamped"' pkg/events/events.go
check "Phase49VendorProofRendered event" grep -q 'Phase49VendorProofRendered.*=.*"phase49.vendor_proof.rendered"' pkg/events/events.go

echo ""

# ============================================================================
# Section 16: Storelog Record Types
# ============================================================================
echo "Section 16: Storelog Record Types"

check "RecordTypeVendorContract exists" grep -q 'RecordTypeVendorContract.*=.*"VENDOR_CONTRACT"' pkg/domain/storelog/log.go
check "RecordTypeVendorContractRevocation exists" grep -q 'RecordTypeVendorContractRevocation.*=.*"VENDOR_CONTRACT_REVOCATION"' pkg/domain/storelog/log.go

echo ""

# ============================================================================
# Section 17: Web Routes
# ============================================================================
echo "Section 17: Web Routes"

check "/vendor/contract route" grep -q '"/vendor/contract"' cmd/quantumlife-web/main.go
check "/vendor/contract/declare route" grep -q '"/vendor/contract/declare"' cmd/quantumlife-web/main.go
check "/vendor/contract/revoke route" grep -q '"/vendor/contract/revoke"' cmd/quantumlife-web/main.go
check "/proof/vendor route" grep -q '"/proof/vendor"' cmd/quantumlife-web/main.go
check "/proof/vendor/dismiss route" grep -q '"/proof/vendor/dismiss"' cmd/quantumlife-web/main.go

echo ""

# ============================================================================
# Section 18: No time.Now() in pkg/ or internal/
# ============================================================================
echo "Section 18: No time.Now() in pkg/ or internal/"

# Exclude comments when checking for time.Now()
check_not "No time.Now() in domain types" grep -v "//" pkg/domain/vendorcontract/types.go | grep "time\.Now()"
check_not "No time.Now() in engine" grep -v "//" internal/vendorcontract/engine.go | grep "time\.Now()"
check_not "No time.Now() in stores" grep -v "//" internal/persist/vendor_contract_store.go | grep "time\.Now()"

echo ""

# ============================================================================
# Section 19: No Goroutines in pkg/ or internal/
# ============================================================================
echo "Section 19: No Goroutines in pkg/ or internal/"

check_not "No goroutines in domain types" grep -r "go func" pkg/domain/vendorcontract/
check_not "No goroutines in engine" grep -r "go func" internal/vendorcontract/
check_not "No goroutines in stores" grep "go func" internal/persist/vendor_contract_store.go

echo ""

# ============================================================================
# Section 20: Stdlib-only in new packages
# ============================================================================
echo "Section 20: Stdlib-only in new packages"

check_not "No external deps in domain (only crypto, encoding, errors)" grep -E "github\.com|golang\.org" pkg/domain/vendorcontract/types.go
check "Domain uses crypto/sha256" grep -q "crypto/sha256" pkg/domain/vendorcontract/types.go
check "Domain uses encoding/hex" grep -q "encoding/hex" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 21: No Forbidden Imports in Domain
# ============================================================================
echo "Section 21: No Forbidden Imports in Domain"

check_not "No pressuredecision import" grep -r "pressuredecision" pkg/domain/vendorcontract/
check_not "No interruptpolicy import" grep -r "interruptpolicy" pkg/domain/vendorcontract/
check_not "No delivery import" grep -r "delivery" pkg/domain/vendorcontract/
check_not "No execution import in domain" grep -r "execution" pkg/domain/vendorcontract/

echo ""

# ============================================================================
# Section 22: No Forbidden Tokens in Files
# ============================================================================
echo "Section 22: No Forbidden Tokens in Files"

check_not "No @ symbol in domain" grep "@" pkg/domain/vendorcontract/types.go
check_not "No http:// in domain" grep "http://" pkg/domain/vendorcontract/types.go
check_not "No https:// in domain" grep "https://" pkg/domain/vendorcontract/types.go
check_not "No @ symbol in engine" grep "@" internal/vendorcontract/engine.go
check_not "No http:// in engine" grep "http://" internal/vendorcontract/engine.go
check_not "No https:// in engine" grep "https://" internal/vendorcontract/engine.go
check_not "No @ symbol in stores" grep "@" internal/persist/vendor_contract_store.go
check_not "No http:// in stores" grep "http://" internal/persist/vendor_contract_store.go
check_not "No https:// in stores" grep "https://" internal/persist/vendor_contract_store.go

echo ""

# ============================================================================
# Section 23: Clamp Logic Correctness
# ============================================================================
echo "Section 23: Clamp Logic Correctness"

check "ClampPressureAllowance compares levels" grep -q "Level()" internal/vendorcontract/engine.go
check "Clamp returns min of current and cap" grep -q "return current" internal/vendorcontract/engine.go
check "Clamp returns cap when clamped" grep -q "return contractCap" internal/vendorcontract/engine.go

echo ""

# ============================================================================
# Section 24: Contract Can Only Reduce Pressure
# ============================================================================
echo "Section 24: Contract Can Only Reduce Pressure"

check "CRITICAL comment about reduce only" grep -q "can only REDUCE pressure" pkg/domain/vendorcontract/types.go
check "CRITICAL comment in engine" grep -q "can only REDUCE pressure" internal/vendorcontract/engine.go
check "Level ordering documented" grep -q "allow_hold_only < allow_surface_only < allow_interrupt_candidate" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 25: Hash-Only Storage
# ============================================================================
echo "Section 25: Hash-Only Storage"

check "VendorCircleHash field exists" grep -q "VendorCircleHash" pkg/domain/vendorcontract/types.go
check "ContractHash field exists" grep -q "ContractHash" pkg/domain/vendorcontract/types.go
check "ProofHash field exists" grep -q "ProofHash" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 26: Deterministic Hashing
# ============================================================================
echo "Section 26: Deterministic Hashing"

check "ComputeContractHash method" grep -q "ComputeContractHash" pkg/domain/vendorcontract/types.go
check "ComputeProofHash method" grep -q "ComputeProofHash" pkg/domain/vendorcontract/types.go
check "ComputeStatusHash method" grep -q "ComputeStatusHash" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 27: Proof Page Types
# ============================================================================
echo "Section 27: Proof Page Types"

check "VendorProofPage struct exists" grep -q "type VendorProofPage struct" pkg/domain/vendorcontract/types.go
check "VendorProofCue struct exists" grep -q "type VendorProofCue struct" pkg/domain/vendorcontract/types.go
check "VendorProofAck struct exists" grep -q "type VendorProofAck struct" pkg/domain/vendorcontract/types.go
check "VendorProofAckKind type exists" grep -q "type VendorProofAckKind string" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Section 28: ADR Content
# ============================================================================
echo "Section 28: ADR Content"

check "ADR has Status section" grep -q "## Status" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "ADR has Context section" grep -q "## Context" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "ADR has Decision section" grep -q "## Decision" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "ADR mentions HOLD-first" grep -q "HOLD" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "ADR mentions clamp-only" grep -q "clamp" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "ADR mentions commerce cap" grep -q "Commerce" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md
check "ADR mentions bounded retention" grep -q "Bounded" docs/ADR/ADR-0087-phase49-vendor-reality-contracts.md

echo ""

# ============================================================================
# Section 29: Period Key Usage
# ============================================================================
echo "Section 29: Period Key Usage"

check "PeriodKey in VendorContract" grep -q "PeriodKey.*string" pkg/domain/vendorcontract/types.go
check "Period key format in engine" grep -q '2006-01-02' internal/vendorcontract/engine.go
check "Period key in stores" grep -q "PeriodKey" internal/persist/vendor_contract_store.go

echo ""

# ============================================================================
# Section 30: No Notification or Execution
# ============================================================================
echo "Section 30: No Notification or Execution"

check_not "No notification in domain" grep -i "notification" pkg/domain/vendorcontract/types.go
check_not "No notification in engine" grep -i "notification" internal/vendorcontract/engine.go
check_not "No deliver in domain" grep -qi "deliver" pkg/domain/vendorcontract/types.go
check_not "No execute in domain" grep -qi "execute" pkg/domain/vendorcontract/types.go

echo ""

# ============================================================================
# Summary
# ============================================================================
echo "=============================================="
echo "Summary"
echo "=============================================="
echo ""
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"
echo ""

if [ $FAIL_COUNT -gt 0 ]; then
    echo "Some guardrails failed!"
    exit 1
else
    echo "All Phase 49 guardrails passed!"
    exit 0
fi
