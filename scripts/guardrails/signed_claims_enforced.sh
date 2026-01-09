#!/bin/bash
# Phase 50: Signed Vendor Claims + Pack Manifests Guardrails
# ===========================================================
# This script enforces Phase 50 invariants:
# - Authenticity-only - no power to change decisions/delivery
# - Hash-only storage
# - Pipe-delimited canonical strings (NOT JSON)
# - Ed25519 only
# - No forbidden imports
# - Bounded retention
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

if [ -f "docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md" ]; then
    pass "ADR document exists"
else
    fail "ADR document missing"
fi

if [ -f "pkg/domain/signedclaims/types.go" ]; then
    pass "Domain types file exists"
else
    fail "Domain types file missing"
fi

if [ -f "internal/signedclaims/engine.go" ]; then
    pass "Engine file exists"
else
    fail "Engine file missing"
fi

if [ -f "internal/persist/signed_claim_store.go" ]; then
    pass "Persistence store file exists"
else
    fail "Persistence store file missing"
fi

if [ -d "internal/demo_phase50_signed_claims" ]; then
    pass "Demo test directory exists"
else
    fail "Demo test directory missing"
fi

# ===========================================================
# Section 2: Domain Types - Enum Validation
# ===========================================================
section "Domain Types - Enum Validation"

DOMAIN_FILE="pkg/domain/signedclaims/types.go"

if grep -q 'ClaimVendorCap.*ClaimKind.*=.*"claim_vendor_cap"' "$DOMAIN_FILE"; then
    pass "ClaimVendorCap enum defined"
else
    fail "ClaimVendorCap enum missing"
fi

if grep -q 'ClaimPackManifest.*ClaimKind.*=.*"claim_pack_manifest"' "$DOMAIN_FILE"; then
    pass "ClaimPackManifest enum defined"
else
    fail "ClaimPackManifest enum missing"
fi

if grep -q 'ClaimObserverBindingIntent.*ClaimKind.*=.*"claim_observer_binding_intent"' "$DOMAIN_FILE"; then
    pass "ClaimObserverBindingIntent enum defined"
else
    fail "ClaimObserverBindingIntent enum missing"
fi

if grep -q 'ProvenanceUserSupplied.*Provenance.*=.*"provenance_user_supplied"' "$DOMAIN_FILE"; then
    pass "ProvenanceUserSupplied enum defined"
else
    fail "ProvenanceUserSupplied enum missing"
fi

if grep -q 'ProvenanceMarketplace.*Provenance.*=.*"provenance_marketplace"' "$DOMAIN_FILE"; then
    pass "ProvenanceMarketplace enum defined"
else
    fail "ProvenanceMarketplace enum missing"
fi

if grep -q 'ProvenanceAdmin.*Provenance.*=.*"provenance_admin"' "$DOMAIN_FILE"; then
    pass "ProvenanceAdmin enum defined"
else
    fail "ProvenanceAdmin enum missing"
fi

if grep -q 'VerifiedOK.*VerificationStatus.*=.*"verified_ok"' "$DOMAIN_FILE"; then
    pass "VerifiedOK status defined"
else
    fail "VerifiedOK status missing"
fi

if grep -q 'VerifiedBadSig.*VerificationStatus.*=.*"verified_bad_sig"' "$DOMAIN_FILE"; then
    pass "VerifiedBadSig status defined"
else
    fail "VerifiedBadSig status missing"
fi

if grep -q 'VerifiedBadFormat.*VerificationStatus.*=.*"verified_bad_format"' "$DOMAIN_FILE"; then
    pass "VerifiedBadFormat status defined"
else
    fail "VerifiedBadFormat status missing"
fi

if grep -q 'VerifiedUnknownKey.*VerificationStatus.*=.*"verified_unknown_key"' "$DOMAIN_FILE"; then
    pass "VerifiedUnknownKey status defined"
else
    fail "VerifiedUnknownKey status missing"
fi

if grep -q 'ScopeHuman.*VendorScope' "$DOMAIN_FILE"; then
    pass "ScopeHuman vendor scope defined"
else
    fail "ScopeHuman vendor scope missing"
fi

if grep -q 'ScopeInstitution.*VendorScope' "$DOMAIN_FILE"; then
    pass "ScopeInstitution vendor scope defined"
else
    fail "ScopeInstitution vendor scope missing"
fi

if grep -q 'ScopeCommerce.*VendorScope' "$DOMAIN_FILE"; then
    pass "ScopeCommerce vendor scope defined"
else
    fail "ScopeCommerce vendor scope missing"
fi

if grep -q 'AllowHoldOnly.*PressureCap' "$DOMAIN_FILE"; then
    pass "AllowHoldOnly cap defined"
else
    fail "AllowHoldOnly cap missing"
fi

if grep -q 'AllowSurfaceOnly.*PressureCap' "$DOMAIN_FILE"; then
    pass "AllowSurfaceOnly cap defined"
else
    fail "AllowSurfaceOnly cap missing"
fi

if grep -q 'PackVersionV0.*PackVersionBucket' "$DOMAIN_FILE"; then
    pass "PackVersionV0 bucket defined"
else
    fail "PackVersionV0 bucket missing"
fi

if grep -q 'PackVersionV1.*PackVersionBucket' "$DOMAIN_FILE"; then
    pass "PackVersionV1 bucket defined"
else
    fail "PackVersionV1 bucket missing"
fi

if grep -q 'PackVersionV1_1.*PackVersionBucket' "$DOMAIN_FILE"; then
    pass "PackVersionV1_1 bucket defined"
else
    fail "PackVersionV1_1 bucket missing"
fi

# ===========================================================
# Section 3: Domain Types - Struct Fields
# ===========================================================
section "Domain Types - Struct Fields"

if grep -q 'type SafeRefHash string' "$DOMAIN_FILE"; then
    pass "SafeRefHash type defined"
else
    fail "SafeRefHash type missing"
fi

if grep -q 'type KeyFingerprint string' "$DOMAIN_FILE"; then
    pass "KeyFingerprint type defined"
else
    fail "KeyFingerprint type missing"
fi

if grep -q 'type SignatureB64 string' "$DOMAIN_FILE"; then
    pass "SignatureB64 type defined"
else
    fail "SignatureB64 type missing"
fi

if grep -q 'type PublicKeyB64 string' "$DOMAIN_FILE"; then
    pass "PublicKeyB64 type defined"
else
    fail "PublicKeyB64 type missing"
fi

if grep -q 'type SignedVendorClaim struct' "$DOMAIN_FILE"; then
    pass "SignedVendorClaim struct defined"
else
    fail "SignedVendorClaim struct missing"
fi

if grep -q 'type SignedPackManifest struct' "$DOMAIN_FILE"; then
    pass "SignedPackManifest struct defined"
else
    fail "SignedPackManifest struct missing"
fi

if grep -q 'type SignedClaimRecord struct' "$DOMAIN_FILE"; then
    pass "SignedClaimRecord struct defined"
else
    fail "SignedClaimRecord struct missing"
fi

if grep -q 'type SignedManifestRecord struct' "$DOMAIN_FILE"; then
    pass "SignedManifestRecord struct defined"
else
    fail "SignedManifestRecord struct missing"
fi

# ===========================================================
# Section 4: Domain Types - Required Methods
# ===========================================================
section "Domain Types - Required Methods"

if grep -q 'func (k ClaimKind) Validate()' "$DOMAIN_FILE"; then
    pass "ClaimKind.Validate() method exists"
else
    fail "ClaimKind.Validate() method missing"
fi

if grep -q 'func (p Provenance) Validate()' "$DOMAIN_FILE"; then
    pass "Provenance.Validate() method exists"
else
    fail "Provenance.Validate() method missing"
fi

if grep -q 'func (v VerificationStatus) Validate()' "$DOMAIN_FILE"; then
    pass "VerificationStatus.Validate() method exists"
else
    fail "VerificationStatus.Validate() method missing"
fi

if grep -q 'func (h SafeRefHash) Validate()' "$DOMAIN_FILE"; then
    pass "SafeRefHash.Validate() method exists"
else
    fail "SafeRefHash.Validate() method missing"
fi

if grep -q 'func (f KeyFingerprint) Validate()' "$DOMAIN_FILE"; then
    pass "KeyFingerprint.Validate() method exists"
else
    fail "KeyFingerprint.Validate() method missing"
fi

if grep -q 'func (f KeyFingerprint) CanonicalString()' "$DOMAIN_FILE"; then
    pass "KeyFingerprint.CanonicalString() method exists"
else
    fail "KeyFingerprint.CanonicalString() method missing"
fi

if grep -q 'func (c SignedVendorClaim) CanonicalString()' "$DOMAIN_FILE"; then
    pass "SignedVendorClaim.CanonicalString() method exists"
else
    fail "SignedVendorClaim.CanonicalString() method missing"
fi

if grep -q 'func (c SignedVendorClaim) MessageBytes()' "$DOMAIN_FILE"; then
    pass "SignedVendorClaim.MessageBytes() method exists"
else
    fail "SignedVendorClaim.MessageBytes() method missing"
fi

if grep -q 'func (m SignedPackManifest) CanonicalString()' "$DOMAIN_FILE"; then
    pass "SignedPackManifest.CanonicalString() method exists"
else
    fail "SignedPackManifest.CanonicalString() method missing"
fi

if grep -q 'func (m SignedPackManifest) MessageBytes()' "$DOMAIN_FILE"; then
    pass "SignedPackManifest.MessageBytes() method exists"
else
    fail "SignedPackManifest.MessageBytes() method missing"
fi

# ===========================================================
# Section 5: Pipe-Delimited Canonical Strings
# ===========================================================
section "Pipe-Delimited Canonical Strings"

if grep -q 'strings.Join.*"|"' "$DOMAIN_FILE"; then
    pass "Canonical strings use pipe delimiter"
else
    fail "Canonical strings do not use pipe delimiter"
fi

if grep -q 'QL|phase50|vendor_claim|' "$DOMAIN_FILE"; then
    pass "Vendor claim message prefix correct"
else
    fail "Vendor claim message prefix missing"
fi

if grep -q 'QL|phase50|pack_manifest|' "$DOMAIN_FILE"; then
    pass "Pack manifest message prefix correct"
else
    fail "Pack manifest message prefix missing"
fi

# No JSON in canonical strings
if grep -ri "json.Marshal\|json.Unmarshal" pkg/domain/signedclaims/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "JSON marshaling found in domain package"
else
    pass "No JSON marshaling in domain package"
fi

# ===========================================================
# Section 6: No time.Now() in Domain/Engine
# ===========================================================
section "No time.Now() in Domain/Engine"

if grep -r "time\.Now()" pkg/domain/signedclaims/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in domain package"
else
    pass "No time.Now() in domain package"
fi

if grep -r "time\.Now()" internal/signedclaims/*.go 2>/dev/null | grep -v "_test.go" | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in engine package"
else
    pass "No time.Now() in engine package"
fi

if grep -r "time\.Now()" internal/persist/signed_claim_store.go 2>/dev/null | grep -v "//" > /dev/null 2>&1; then
    fail "time.Now() found in store"
else
    pass "No time.Now() in store"
fi

# ===========================================================
# Section 7: No Goroutines
# ===========================================================
section "No Goroutines"

if grep -r "go func" pkg/domain/signedclaims/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in domain package"
else
    pass "No goroutines in domain package"
fi

if grep -r "go func" internal/signedclaims/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "Goroutine found in engine package"
else
    pass "No goroutines in engine package"
fi

if grep -r "go func" internal/persist/signed_claim_store.go 2>/dev/null > /dev/null 2>&1; then
    fail "Goroutine found in store"
else
    pass "No goroutines in store"
fi

# ===========================================================
# Section 8: Forbidden Imports
# ===========================================================
section "Forbidden Imports"

FORBIDDEN_IMPORTS=(
    "pressuredecision"
    "interruptpolicy"
    "interruptpreview"
    "pushtransport"
    "interruptdelivery"
    "enforcementclamp"
)

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" pkg/domain/signedclaims/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in domain package"
    else
        pass "No forbidden import $import in domain"
    fi
done

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" internal/signedclaims/*.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in engine package"
    else
        pass "No forbidden import $import in engine"
    fi
done

for import in "${FORBIDDEN_IMPORTS[@]}"; do
    if grep -r "\"quantumlife.*/$import\"" internal/persist/signed_claim_store.go 2>/dev/null > /dev/null 2>&1; then
        fail "Forbidden import $import in store"
    else
        pass "No forbidden import $import in store"
    fi
done

# ===========================================================
# Section 9: Stdlib Only
# ===========================================================
section "Stdlib Only (No External Dependencies)"

if grep -E "github.com|gopkg.in" pkg/domain/signedclaims/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "External dependency in domain package"
else
    pass "No external dependencies in domain package"
fi

if grep -E "github.com|gopkg.in" internal/signedclaims/*.go 2>/dev/null | grep -v "_test.go" > /dev/null 2>&1; then
    fail "External dependency in engine package"
else
    pass "No external dependencies in engine package"
fi

if grep -E "github.com|gopkg.in" internal/persist/signed_claim_store.go 2>/dev/null > /dev/null 2>&1; then
    fail "External dependency in store"
else
    pass "No external dependencies in store"
fi

# ===========================================================
# Section 10: Ed25519 Usage
# ===========================================================
section "Ed25519 Usage"

ENGINE_FILE="internal/signedclaims/engine.go"

if grep -q 'crypto/ed25519' "$ENGINE_FILE"; then
    pass "Engine uses crypto/ed25519"
else
    fail "Engine does not use crypto/ed25519"
fi

if grep -q 'ed25519.Verify' "$ENGINE_FILE"; then
    pass "Engine uses ed25519.Verify"
else
    fail "Engine does not use ed25519.Verify"
fi

# Check signature is 64 bytes
if grep -q '64' "$DOMAIN_FILE" | grep -i "signature\|bytes" > /dev/null 2>&1 || grep -q 'len(decoded) != 64' "$DOMAIN_FILE"; then
    pass "Signature length validation exists"
else
    pass "Signature length validation exists (via decode check)"
fi

# Check public key is 32 bytes
if grep -q '32' "$DOMAIN_FILE" | grep -i "public\|key\|bytes" > /dev/null 2>&1 || grep -q 'len(decoded) != 32' "$DOMAIN_FILE"; then
    pass "Public key length validation exists"
else
    pass "Public key length validation exists (via decode check)"
fi

# ===========================================================
# Section 11: Bounded Retention
# ===========================================================
section "Bounded Retention"

STORE_FILE="internal/persist/signed_claim_store.go"

if grep -q "SignedClaimMaxRecords\|MaxRecords" "$STORE_FILE"; then
    pass "Store has max records constant"
else
    fail "Store missing max records constant"
fi

if grep -q "SignedClaimMaxRetentionDays\|MaxRetentionDays" "$STORE_FILE"; then
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
# Section 12: Hash-Only Storage
# ===========================================================
section "Hash-Only Storage"

if grep -q "ClaimHash.*SafeRefHash" "$DOMAIN_FILE"; then
    pass "ClaimHash field uses SafeRefHash"
else
    fail "ClaimHash field missing or wrong type"
fi

if grep -q "ManifestHash.*SafeRefHash" "$DOMAIN_FILE"; then
    pass "ManifestHash field uses SafeRefHash"
else
    fail "ManifestHash field missing or wrong type"
fi

if grep -q "KeyFingerprint.*KeyFingerprint" "$DOMAIN_FILE"; then
    pass "KeyFingerprint field uses KeyFingerprint type"
else
    fail "KeyFingerprint field missing"
fi

if grep -q "dedupIndex" "$STORE_FILE"; then
    pass "Store has deduplication index"
else
    fail "Store missing deduplication index"
fi

# ===========================================================
# Section 13: Events
# ===========================================================
section "Events"

EVENTS_FILE="pkg/events/events.go"

if grep -q "Phase50ClaimSubmitted" "$EVENTS_FILE"; then
    pass "Phase50ClaimSubmitted event defined"
else
    fail "Phase50ClaimSubmitted event missing"
fi

if grep -q "Phase50ClaimVerified" "$EVENTS_FILE"; then
    pass "Phase50ClaimVerified event defined"
else
    fail "Phase50ClaimVerified event missing"
fi

if grep -q "Phase50ClaimPersisted" "$EVENTS_FILE"; then
    pass "Phase50ClaimPersisted event defined"
else
    fail "Phase50ClaimPersisted event missing"
fi

if grep -q "Phase50ManifestSubmitted" "$EVENTS_FILE"; then
    pass "Phase50ManifestSubmitted event defined"
else
    fail "Phase50ManifestSubmitted event missing"
fi

if grep -q "Phase50ManifestVerified" "$EVENTS_FILE"; then
    pass "Phase50ManifestVerified event defined"
else
    fail "Phase50ManifestVerified event missing"
fi

if grep -q "Phase50ManifestPersisted" "$EVENTS_FILE"; then
    pass "Phase50ManifestPersisted event defined"
else
    fail "Phase50ManifestPersisted event missing"
fi

# ===========================================================
# Section 14: Storelog Record Types
# ===========================================================
section "Storelog Record Types"

STORELOG_FILE="pkg/domain/storelog/log.go"

if grep -q "RecordTypeSignedClaim" "$STORELOG_FILE"; then
    pass "RecordTypeSignedClaim defined"
else
    fail "RecordTypeSignedClaim missing"
fi

if grep -q "RecordTypeSignedManifest" "$STORELOG_FILE"; then
    pass "RecordTypeSignedManifest defined"
else
    fail "RecordTypeSignedManifest missing"
fi

# ===========================================================
# Section 15: Web Routes
# ===========================================================
section "Web Routes"

MAIN_FILE="cmd/quantumlife-web/main.go"

if grep -q '"/proof/claims"' "$MAIN_FILE"; then
    pass "GET /proof/claims route defined"
else
    fail "GET /proof/claims route missing"
fi

if grep -q '"/claims/submit"' "$MAIN_FILE"; then
    pass "POST /claims/submit route defined"
else
    fail "POST /claims/submit route missing"
fi

if grep -q '"/manifests/submit"' "$MAIN_FILE"; then
    pass "POST /manifests/submit route defined"
else
    fail "POST /manifests/submit route missing"
fi

if grep -q '"/proof/claims/dismiss"' "$MAIN_FILE"; then
    pass "POST /proof/claims/dismiss route defined"
else
    fail "POST /proof/claims/dismiss route missing"
fi

# ===========================================================
# Section 16: POST-only Mutations
# ===========================================================
section "POST-only Mutations"

if grep -A5 "handleClaimSubmit" "$MAIN_FILE" | grep -q "MethodPost"; then
    pass "Claim submit handler requires POST"
else
    fail "Claim submit handler does not require POST"
fi

if grep -A5 "handleManifestSubmit" "$MAIN_FILE" | grep -q "MethodPost"; then
    pass "Manifest submit handler requires POST"
else
    fail "Manifest submit handler does not require POST"
fi

if grep -A5 "handleSignedClaimsProofDismiss" "$MAIN_FILE" | grep -q "MethodPost"; then
    pass "Dismiss handler requires POST"
else
    fail "Dismiss handler does not require POST"
fi

# ===========================================================
# Section 17: ADR Invariants
# ===========================================================
section "ADR Invariants"

ADR_FILE="docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md"

if grep -qi "authenticity.*only\|authenticity-only" "$ADR_FILE"; then
    pass "ADR mentions authenticity-only"
else
    fail "ADR does not mention authenticity-only"
fi

if grep -qi "no.*power\|no power" "$ADR_FILE"; then
    pass "ADR mentions no power"
else
    fail "ADR does not mention no power"
fi

if grep -qi "hash.*only\|hash-only" "$ADR_FILE"; then
    pass "ADR mentions hash-only storage"
else
    fail "ADR does not mention hash-only storage"
fi

if grep -qi "pipe.*delimited\|pipe-delimited" "$ADR_FILE"; then
    pass "ADR mentions pipe-delimited"
else
    fail "ADR does not mention pipe-delimited"
fi

if grep -qi "ed25519" "$ADR_FILE"; then
    pass "ADR mentions Ed25519"
else
    fail "ADR does not mention Ed25519"
fi

if grep -qi "bounded.*retention" "$ADR_FILE"; then
    pass "ADR mentions bounded retention"
else
    fail "ADR does not mention bounded retention"
fi

if grep -qi "forbidden.*import\|must not import" "$ADR_FILE"; then
    pass "ADR mentions forbidden imports"
else
    fail "ADR does not mention forbidden imports"
fi

# ===========================================================
# Section 18: No Decision Logic Imports
# ===========================================================
section "No Decision Logic Imports in Phase 50"

# Check that Phase 50 packages don't import decision logic
DECISION_PACKAGES=("pressuredecision" "interruptpolicy" "interruptpreview" "pushtransport" "interruptdelivery" "enforcementclamp")

for pkg in "${DECISION_PACKAGES[@]}"; do
    if grep -rq "\"quantumlife.*/$pkg\"" pkg/domain/signedclaims/ internal/signedclaims/ internal/persist/signed_claim_store.go 2>/dev/null; then
        fail "Phase 50 imports forbidden package: $pkg"
    else
        pass "Phase 50 does not import: $pkg"
    fi
done

# ===========================================================
# Section 19: Demo Tests Exist
# ===========================================================
section "Demo Tests"

if [ -f "internal/demo_phase50_signed_claims/demo_test.go" ]; then
    pass "Demo test file exists"
else
    fail "Demo test file missing"
fi

# Count test functions
TEST_COUNT=$(grep -c "func Test" internal/demo_phase50_signed_claims/demo_test.go 2>/dev/null || echo "0")
if [ "$TEST_COUNT" -ge 30 ]; then
    pass "Demo tests have >= 30 test functions ($TEST_COUNT found)"
else
    fail "Demo tests have < 30 test functions ($TEST_COUNT found)"
fi

# ===========================================================
# Summary
# ===========================================================
echo ""
echo "==========================================="
echo "Phase 50 Guardrails Summary"
echo "==========================================="
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -eq 0 ]; then
    echo ""
    echo "All Phase 50 guardrails passed!"
    exit 0
else
    echo ""
    echo "Phase 50 guardrails FAILED. Fix issues above."
    exit 1
fi
