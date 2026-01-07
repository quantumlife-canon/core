#!/bin/bash
# Phase 30A: Identity + Replay
# Guardrails enforcing critical safety invariants.
#
# CRITICAL INVARIANTS:
#   - Ed25519 device-rooted identity (stdlib only)
#   - Device fingerprint bound to Circle (hash-only, max 5 devices)
#   - Signed requests for replay export/import (POST only)
#   - Deterministic replay bundle (pipe-delimited, NOT JSON)
#   - No time.Now() in internal/pkg (clock injection only)
#   - No goroutines in engine/store
#   - No raw identifiers in bundles
#   - Bounded retention (30 days)
#
# Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Phase 30A: Identity + Replay guardrails ==="
echo

fail_count=0

# Helper function
check() {
    local description="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "[PASS] $description"
    else
        echo "[FAIL] $description"
        fail_count=$((fail_count + 1))
    fi
}

check_not() {
    local description="$1"
    shift
    if ! "$@" >/dev/null 2>&1; then
        echo "[PASS] $description"
    else
        echo "[FAIL] $description"
        fail_count=$((fail_count + 1))
    fi
}

echo "--- 1. Package structure ---"
check "Device identity domain exists" test -f "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "Replay domain exists" test -f "$ROOT_DIR/pkg/domain/replay/types.go"
check "Device key store exists" test -f "$ROOT_DIR/internal/persist/device_key_store.go"
check "Circle binding store exists" test -f "$ROOT_DIR/internal/persist/circle_binding_store.go"
check "Identity engine exists" test -f "$ROOT_DIR/internal/deviceidentity/engine.go"
check "Replay engine exists" test -f "$ROOT_DIR/internal/replay/engine.go"

echo
echo "--- 2. Ed25519 cryptography (stdlib only) ---"
check "crypto/ed25519 used in key store" grep -q 'crypto/ed25519' "$ROOT_DIR/internal/persist/device_key_store.go"
check "ed25519.GenerateKey used" grep -q 'ed25519\.GenerateKey' "$ROOT_DIR/internal/persist/device_key_store.go"
check "ed25519.Sign used" grep -q 'ed25519\.Sign' "$ROOT_DIR/internal/persist/device_key_store.go"
check "ed25519.Verify used" grep -q 'ed25519\.Verify' "$ROOT_DIR/internal/persist/device_key_store.go"
check_not "No cloud crypto SDKs in key store" grep -q 'github.com/aws\|cloud.google.com\|github.com/Azure' "$ROOT_DIR/internal/persist/device_key_store.go"

echo
echo "--- 3. Device identity types ---"
check "DevicePublicKey type exists" grep -q 'type DevicePublicKey string' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "Fingerprint type exists" grep -q 'type Fingerprint string' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "PeriodKey type exists" grep -q 'type PeriodKey string' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "SignedRequest type exists" grep -q 'type SignedRequest struct' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "CircleBinding type exists" grep -q 'type CircleBinding struct' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"

echo
echo "--- 4. Max devices per circle (bounded) ---"
check "MaxDevicesPerCircle constant" grep -q 'MaxDevicesPerCircle.*=.*5' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "Binding store enforces limit" grep -q 'MaxDevicesPerCircle' "$ROOT_DIR/internal/persist/circle_binding_store.go"

echo
echo "--- 5. Default retention (bounded) ---"
check "DefaultRetentionDays constant" grep -q 'DefaultRetentionDays.*=.*30' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"

echo
echo "--- 6. Replay bundle format (pipe-delimited, NOT JSON) ---"
check "Pipe delimiter in CanonicalString" grep -q 'Sprintf.*|.*|' "$ROOT_DIR/pkg/domain/replay/types.go"
check "CanonicalRecordLine type exists" grep -q 'type CanonicalRecordLine struct' "$ROOT_DIR/pkg/domain/replay/types.go"
check_not "No json.Marshal in replay types" grep -q 'json\.Marshal' "$ROOT_DIR/pkg/domain/replay/types.go"
check_not "No json.Unmarshal in replay types" grep -q 'json\.Unmarshal' "$ROOT_DIR/pkg/domain/replay/types.go"

echo
echo "--- 7. Safe record types whitelist ---"
check "SafeRecordTypes map exists" grep -q 'SafeRecordTypes' "$ROOT_DIR/pkg/domain/replay/types.go"
check "IsSafeForExport function exists" grep -q 'func IsSafeForExport' "$ROOT_DIR/pkg/domain/replay/types.go"

echo
echo "--- 8. Forbidden patterns check ---"
check "ForbiddenPatterns list exists" grep -q 'ForbiddenPatterns' "$ROOT_DIR/pkg/domain/replay/types.go"
check "ContainsForbiddenPattern function exists" grep -q 'func ContainsForbiddenPattern' "$ROOT_DIR/pkg/domain/replay/types.go"

echo
echo "--- 9. No goroutines ---"
check_not "No goroutines in deviceidentity domain" grep -rq 'go func\|go .*(' "$ROOT_DIR/pkg/domain/deviceidentity/"
check_not "No goroutines in replay domain" grep -rq 'go func\|go .*(' "$ROOT_DIR/pkg/domain/replay/"
check_not "No goroutines in device key store" grep -q 'go func\|go .*(' "$ROOT_DIR/internal/persist/device_key_store.go"
check_not "No goroutines in circle binding store" grep -q 'go func\|go .*(' "$ROOT_DIR/internal/persist/circle_binding_store.go"
check_not "No goroutines in identity engine" grep -q 'go func\|go .*(' "$ROOT_DIR/internal/deviceidentity/engine.go"
check_not "No goroutines in replay engine" grep -q 'go func\|go .*(' "$ROOT_DIR/internal/replay/engine.go"

echo
echo "--- 10. No time.Now() in code (comments allowed) ---"
# Use bash -c for proper pipeline handling
check_not "No time.Now in deviceidentity domain code" bash -c "grep -v '^[[:space:]]*//' '$ROOT_DIR/pkg/domain/deviceidentity/types.go' | grep -q 'time\.Now()'"
check_not "No time.Now in replay domain code" bash -c "grep -v '^[[:space:]]*//' '$ROOT_DIR/pkg/domain/replay/types.go' | grep -q 'time\.Now()'"
check_not "No time.Now in device key store code" bash -c "grep -v '^[[:space:]]*//' '$ROOT_DIR/internal/persist/device_key_store.go' | grep -q 'time\.Now()'"
check_not "No time.Now in circle binding store code" bash -c "grep -v '^[[:space:]]*//' '$ROOT_DIR/internal/persist/circle_binding_store.go' | grep -q 'time\.Now()'"
check_not "No time.Now in identity engine code" bash -c "grep -v '^[[:space:]]*//' '$ROOT_DIR/internal/deviceidentity/engine.go' | grep -q 'time\.Now()'"
check_not "No time.Now in replay engine code" bash -c "grep -v '^[[:space:]]*//' '$ROOT_DIR/internal/replay/engine.go' | grep -q 'time\.Now()'"

echo
echo "--- 11. Clock injection ---"
check "Clock param in circle binding store" grep -q 'clock.*func().*time\.Time' "$ROOT_DIR/internal/persist/circle_binding_store.go"
check "Clock param in identity engine" grep -q 'clock.*func().*time\.Time' "$ROOT_DIR/internal/deviceidentity/engine.go"
check "Clock param in replay engine" grep -q 'clock.*func().*time\.Time' "$ROOT_DIR/internal/replay/engine.go"

echo
echo "--- 12. Hash-only storage ---"
check "CircleIDHash in binding" grep -q 'CircleIDHash' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "Fingerprint in binding (hash of pubkey)" grep -q 'Fingerprint.*Fingerprint' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"
check "BindingHash field" grep -q 'BindingHash.*string' "$ROOT_DIR/pkg/domain/deviceidentity/types.go"

echo
echo "--- 13. Signature verification ---"
check "VerifySignedRequest exists in engine" grep -q 'func.*VerifySignedRequest' "$ROOT_DIR/internal/deviceidentity/engine.go"
check "RequireBoundDevice exists in engine" grep -q 'func.*RequireBoundDevice' "$ROOT_DIR/internal/deviceidentity/engine.go"
check "VerifyRequest exists in key store" grep -q 'func.*VerifyRequest\|func Verify' "$ROOT_DIR/internal/persist/device_key_store.go"

echo
echo "--- 14. Storelog record type ---"
check "RecordTypeCircleBinding defined" grep -q 'RecordTypeCircleBinding.*CIRCLE_BINDING' "$ROOT_DIR/pkg/domain/storelog/log.go"

echo
echo "--- 15. Events ---"
check "Phase30AIdentityCreated event" grep -q 'Phase30AIdentityCreated' "$ROOT_DIR/pkg/events/events.go"
check "Phase30AIdentityBound event" grep -q 'Phase30AIdentityBound' "$ROOT_DIR/pkg/events/events.go"
check "Phase30AReplayExported event" grep -q 'Phase30AReplayExported' "$ROOT_DIR/pkg/events/events.go"
check "Phase30AReplayImported event" grep -q 'Phase30AReplayImported' "$ROOT_DIR/pkg/events/events.go"
check "Phase30AReplayRejected event" grep -q 'Phase30AReplayRejected' "$ROOT_DIR/pkg/events/events.go"

echo
echo "--- 16. Private key security ---"
check "Key stored with 0600 permissions" grep -q '0600\|os.FileMode(0600)\|0o600' "$ROOT_DIR/internal/persist/device_key_store.go"
check_not "No private key in errors" grep -q 'privateKey.*error\|Error.*privateKey' "$ROOT_DIR/internal/persist/device_key_store.go"
check "Never log warning in comments" grep -q 'NEVER.*log.*private\|never.*log.*private' "$ROOT_DIR/internal/persist/device_key_store.go"

echo
echo "--- 17. Deterministic bundle hash ---"
check "ComputeBundleHash function exists" grep -q 'func.*ComputeBundleHash' "$ROOT_DIR/pkg/domain/replay/types.go"
check "Bundle uses deterministic ordering" grep -q 'sort\.' "$ROOT_DIR/internal/replay/engine.go"

echo
echo "=== Summary ==="
if [ $fail_count -eq 0 ]; then
    echo "All Phase 30A guardrails passed!"
    exit 0
else
    echo "FAILED: $fail_count guardrail(s) failed"
    exit 1
fi
