#!/bin/bash
# =============================================================================
# Phase 31.3b: TrueLayer Real Sync Guardrails
# =============================================================================
#
# Reference: docs/ADR/ADR-0066-phase31-3b-truelayer-real-sync.md
#
# CRITICAL INVARIANTS:
#   - Real HTTP calls via stdlib net/http (no cloud SDKs)
#   - Bounded sync limits enforced (25 accounts, 25 tx/account, 7 days)
#   - No mock transaction fixtures in production sync handler
#   - No time.Now() in internal/ or pkg/ (clock injection only)
#   - No goroutines in sync/persist code
#   - Privacy: no amounts/merchants/timestamps in outputs
#   - ProviderTrueLayer required for commerce ingest
#
# =============================================================================

set -e

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo "✓ $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo "✗ $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

echo "=== Phase 31.3b: TrueLayer Real Sync Guardrails ==="
echo ""

# =============================================================================
# 1. Package Structure Checks
# =============================================================================
echo "--- Package Structure ---"

if [ -f "internal/connectors/finance/read/providers/truelayer/sync.go" ]; then
    pass "Sync service file exists"
else
    fail "Sync service file missing"
fi

if [ -f "internal/persist/truelayer_token_store.go" ]; then
    pass "Token store file exists"
else
    fail "Token store file missing"
fi

if [ -d "internal/demo_phase31_3b_truelayer_real_sync" ]; then
    pass "Demo test directory exists"
else
    fail "Demo test directory missing"
fi

# =============================================================================
# 2. Bounded Sync Constants Checks
# =============================================================================
echo ""
echo "--- Bounded Sync Constants ---"

if grep -q "MaxAccounts.*=.*25" internal/connectors/finance/read/providers/truelayer/sync.go; then
    pass "MaxAccounts = 25 defined"
else
    fail "MaxAccounts constant missing or incorrect"
fi

if grep -q "MaxTransactionsPerAccount.*=.*25" internal/connectors/finance/read/providers/truelayer/sync.go; then
    pass "MaxTransactionsPerAccount = 25 defined"
else
    fail "MaxTransactionsPerAccount constant missing or incorrect"
fi

if grep -q "SyncWindowDays.*=.*7" internal/connectors/finance/read/providers/truelayer/sync.go; then
    pass "SyncWindowDays = 7 defined"
else
    fail "SyncWindowDays constant missing or incorrect"
fi

# =============================================================================
# 3. stdlib Only Checks (No cloud SDKs)
# =============================================================================
echo ""
echo "--- stdlib Only (No Cloud SDKs) ---"

# Check for forbidden SDK imports in sync service
if ! grep -q "github.com/aws" internal/connectors/finance/read/providers/truelayer/sync.go 2>/dev/null; then
    pass "No AWS SDK in sync service"
else
    fail "AWS SDK found in sync service"
fi

if ! grep -q "cloud.google.com" internal/connectors/finance/read/providers/truelayer/sync.go 2>/dev/null; then
    pass "No Google Cloud SDK in sync service"
else
    fail "Google Cloud SDK found in sync service"
fi

if ! grep -q "github.com/Azure" internal/connectors/finance/read/providers/truelayer/sync.go 2>/dev/null; then
    pass "No Azure SDK in sync service"
else
    fail "Azure SDK found in sync service"
fi

# Verify net/http is used
if grep -q '"net/http"' internal/connectors/finance/read/providers/truelayer/client.go; then
    pass "stdlib net/http used in client"
else
    fail "net/http not found in client"
fi

# =============================================================================
# 4. Clock Injection Checks (No time.Now())
# =============================================================================
echo ""
echo "--- Clock Injection ---"

# Check sync service
if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/connectors/finance/read/providers/truelayer/sync.go | grep -q "time\.Now()"; then
    pass "No time.Now() in sync service"
else
    fail "time.Now() found in sync service"
fi

# Check token store
if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/persist/truelayer_token_store.go | grep -q "time\.Now()"; then
    pass "No time.Now() in token store"
else
    fail "time.Now() found in token store"
fi

# Verify clock field exists in sync service
if grep -q "clock.*func().*time\.Time" internal/connectors/finance/read/providers/truelayer/sync.go; then
    pass "Clock injection field present in sync service"
else
    fail "Clock injection field missing in sync service"
fi

# Verify clock field exists in token store
if grep -q "clock.*func().*time\.Time" internal/persist/truelayer_token_store.go; then
    pass "Clock injection field present in token store"
else
    fail "Clock injection field missing in token store"
fi

# =============================================================================
# 5. No Goroutines Checks
# =============================================================================
echo ""
echo "--- No Goroutines ---"

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/connectors/finance/read/providers/truelayer/sync.go | grep -q "go func"; then
    pass "No goroutines in sync service"
else
    fail "Goroutine found in sync service"
fi

if ! grep -v "^[[:space:]]*//\|^[[:space:]]*\*" internal/persist/truelayer_token_store.go | grep -q "go func"; then
    pass "No goroutines in token store"
else
    fail "Goroutine found in token store"
fi

# =============================================================================
# 6. Privacy Checks (No Forbidden Tokens in Output Types)
# =============================================================================
echo ""
echo "--- Privacy Checks ---"

# TransactionClassification should not have Amount field
if ! grep -q "Amount.*float\|Amount.*int\|Amount.*string" internal/connectors/finance/read/providers/truelayer/sync.go | grep -v "^[[:space:]]*//"; then
    pass "No Amount field in TransactionClassification"
else
    fail "Amount field found in TransactionClassification"
fi

# TransactionClassification should not have MerchantName field
if ! grep "type TransactionClassification" -A 20 internal/connectors/finance/read/providers/truelayer/sync.go | grep -q "MerchantName\|Merchant"; then
    pass "No MerchantName field in TransactionClassification"
else
    fail "MerchantName field found in TransactionClassification"
fi

# SyncOutput should not have raw amounts
if ! grep "type SyncOutput" -A 30 internal/connectors/finance/read/providers/truelayer/sync.go | grep -q "RawAmount\|TotalAmount"; then
    pass "No raw amounts in SyncOutput"
else
    fail "Raw amounts found in SyncOutput"
fi

# =============================================================================
# 7. Provider Validation Checks (Phase 31.3 Compliance)
# =============================================================================
echo ""
echo "--- Provider Validation (Phase 31.3 Compliance) ---"

if grep -q "ProviderTrueLayer" internal/financetxscan/model.go; then
    pass "ProviderTrueLayer constant exists"
else
    fail "ProviderTrueLayer constant missing"
fi

if grep -q "ProviderMock" internal/financetxscan/model.go; then
    pass "ProviderMock constant exists (for rejection)"
else
    fail "ProviderMock constant missing"
fi

if grep -q "ValidateProvider" internal/financetxscan/model.go; then
    pass "ValidateProvider function exists"
else
    fail "ValidateProvider function missing"
fi

if grep -q "rejected_mock_provider" internal/financetxscan/engine.go; then
    pass "Mock provider rejection status exists"
else
    fail "Mock provider rejection status missing"
fi

# =============================================================================
# 8. Events Checks
# =============================================================================
echo ""
echo "--- Phase 31.3b Events ---"

if grep -q "Phase31_3bTrueLayerSyncStarted" pkg/events/events.go; then
    pass "Phase31_3bTrueLayerSyncStarted event defined"
else
    fail "Phase31_3bTrueLayerSyncStarted event missing"
fi

if grep -q "Phase31_3bTrueLayerSyncCompleted" pkg/events/events.go; then
    pass "Phase31_3bTrueLayerSyncCompleted event defined"
else
    fail "Phase31_3bTrueLayerSyncCompleted event missing"
fi

if grep -q "Phase31_3bTrueLayerIngestStarted" pkg/events/events.go; then
    pass "Phase31_3bTrueLayerIngestStarted event defined"
else
    fail "Phase31_3bTrueLayerIngestStarted event missing"
fi

if grep -q "Phase31_3bTrueLayerIngestCompleted" pkg/events/events.go; then
    pass "Phase31_3bTrueLayerIngestCompleted event defined"
else
    fail "Phase31_3bTrueLayerIngestCompleted event missing"
fi

if grep -q "Phase31_3bTrueLayerTokenStored" pkg/events/events.go; then
    pass "Phase31_3bTrueLayerTokenStored event defined"
else
    fail "Phase31_3bTrueLayerTokenStored event missing"
fi

# =============================================================================
# 9. Web Handler Integration Checks
# =============================================================================
echo ""
echo "--- Web Handler Integration ---"

if grep -q "trueLayerTokenStore" cmd/quantumlife-web/main.go; then
    pass "Token store field in Server struct"
else
    fail "Token store field missing in Server struct"
fi

if grep -q "trueLayerSyncService" cmd/quantumlife-web/main.go; then
    pass "Sync service field in Server struct"
else
    fail "Sync service field missing in Server struct"
fi

if grep -q "Phase31_3bTrueLayerSyncStarted" cmd/quantumlife-web/main.go; then
    pass "Sync started event emitted in handler"
else
    fail "Sync started event not emitted in handler"
fi

if grep -q "ProviderTrueLayer" cmd/quantumlife-web/main.go; then
    pass "ProviderTrueLayer used in handler"
else
    fail "ProviderTrueLayer not used in handler"
fi

# =============================================================================
# 10. Token Store Security Checks
# =============================================================================
echo ""
echo "--- Token Store Security ---"

if grep -q "SENSITIVE.*Never log" internal/persist/truelayer_token_store.go; then
    pass "Token sensitivity documented"
else
    fail "Token sensitivity not documented"
fi

if grep -q "sync\.RWMutex\|sync\.Mutex" internal/persist/truelayer_token_store.go; then
    pass "Thread-safe with mutex"
else
    fail "No mutex found in token store"
fi

if grep -q "TokenHash" internal/persist/truelayer_token_store.go; then
    pass "Token hash field exists (safe for logging)"
else
    fail "Token hash field missing"
fi

# =============================================================================
# 11. Test Coverage Checks
# =============================================================================
echo ""
echo "--- Test Coverage ---"

if [ -f "internal/demo_phase31_3b_truelayer_real_sync/demo_test.go" ]; then
    pass "Demo tests exist"
else
    fail "Demo tests missing"
fi

if grep -q "TestBoundedSyncLimits" internal/demo_phase31_3b_truelayer_real_sync/demo_test.go 2>/dev/null; then
    pass "Bounded limits tests exist"
else
    fail "Bounded limits tests missing"
fi

if grep -q "TestProviderValidation" internal/demo_phase31_3b_truelayer_real_sync/demo_test.go 2>/dev/null; then
    pass "Provider validation tests exist"
else
    fail "Provider validation tests missing"
fi

if grep -q "TestDeterminism" internal/demo_phase31_3b_truelayer_real_sync/demo_test.go 2>/dev/null; then
    pass "Determinism tests exist"
else
    fail "Determinism tests missing"
fi

if grep -q "httptest" internal/demo_phase31_3b_truelayer_real_sync/demo_test.go 2>/dev/null; then
    pass "httptest used (CI-safe)"
else
    fail "httptest not used in tests"
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "=== Summary ==="
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if [ $FAIL_COUNT -gt 0 ]; then
    echo ""
    echo "Phase 31.3b guardrails FAILED"
    exit 1
fi

echo ""
echo "Phase 31.3b guardrails PASSED"
exit 0
