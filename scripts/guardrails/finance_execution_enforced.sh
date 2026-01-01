#!/usr/bin/env bash
# finance_execution_enforced.sh
#
# Guardrail: Finance execution MUST be deterministic, constrained, and auditable.
#
# CRITICAL: Mock providers NEVER move real money (Simulated=true).
# CRITICAL: No goroutines in finance write packages.
# CRITICAL: No time.Now() - use clock injection.
# CRITICAL: All executions require explicit approval.
# CRITICAL: Payees must be pre-defined (v9.10 - no free-text recipients).
# CRITICAL: Providers must be registered (v9.9 - allowlist enforcement).
# CRITICAL: Policy snapshots required (v9.12).
# CRITICAL: View snapshots required (v9.13).
# CRITICAL: Idempotency enforced (v9.6).
#
# Reference: Phase 17 Finance Execution Boundary

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Guardrail: Finance Execution Enforced (Sandbox->Live) ==="
echo ""

VIOLATIONS=0

# Check 1: Mock connector reports Simulated=true
echo "Check 1: Mock connector reports Simulated=true..."

if [ -f "internal/connectors/finance/write/providers/mock/mock.go" ]; then
  SIMULATED_TRUE=$(grep -c "Simulated: true\|Simulated:.*true" internal/connectors/finance/write/providers/mock/mock.go 2>/dev/null || echo "0")
  if [ "$SIMULATED_TRUE" -lt 1 ]; then
    echo -e "${RED}VIOLATION: Mock connector missing Simulated=true${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Mock connector sets Simulated=true (count: $SIMULATED_TRUE)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Mock connector missing (internal/connectors/finance/write/providers/mock/mock.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 2: No goroutines in finance write packages
echo ""
echo "Check 2: No goroutines in finance write packages..."

GOROUTINES=$(grep -rn "go func\|go [a-zA-Z]" \
  --include="*.go" \
  internal/connectors/finance/write/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$GOROUTINES" ]; then
  echo -e "${RED}VIOLATION: Goroutines found in finance write packages:${NC}"
  echo "$GOROUTINES"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No goroutines in finance write packages${NC}"
fi

# Check 3: No time.Now() in mock connector (TrueLayer excluded - uses OAuth)
echo ""
echo "Check 3: No time.Now() in mock connector..."

TIME_NOW=$(grep -rn "time\.Now()" \
  --include="*.go" \
  internal/connectors/finance/write/providers/mock/ \
  internal/connectors/finance/write/payees/ \
  internal/connectors/finance/write/registry/ 2>/dev/null | \
  grep -v "_test.go" | \
  grep -v "^[^:]*:[0-9]*:\s*//" || true)

if [ -n "$TIME_NOW" ]; then
  echo -e "${RED}VIOLATION: time.Now() found (should use clock injection):${NC}"
  echo "$TIME_NOW"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No time.Now() in mock connector/registries${NC}"
fi

# Check 4: Payee registry enforces pre-defined payees
echo ""
echo "Check 4: Payee registry enforces pre-defined payees..."

if [ -f "internal/connectors/finance/write/payees/registry.go" ]; then
  PAYEE_REGISTRY=$(grep -c "RequireAllowed\|PayeeID\|BlockedPayeeIDs\|AllowedPayeeIDs" internal/connectors/finance/write/payees/registry.go 2>/dev/null || echo "0")
  if [ "$PAYEE_REGISTRY" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Payee registry missing enforcement (found: $PAYEE_REGISTRY)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Payee registry has enforcement (count: $PAYEE_REGISTRY)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Payee registry missing (internal/connectors/finance/write/payees/registry.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 5: Provider registry enforces allowlist
echo ""
echo "Check 5: Provider registry enforces allowlist..."

if [ -f "internal/connectors/finance/write/registry/registry.go" ]; then
  PROVIDER_REGISTRY=$(grep -c "RequireAllowed\|ProviderID\|AllowedProviderIDs" internal/connectors/finance/write/registry/registry.go 2>/dev/null || echo "0")
  if [ "$PROVIDER_REGISTRY" -lt 3 ]; then
    echo -e "${RED}VIOLATION: Provider registry missing enforcement (found: $PROVIDER_REGISTRY)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Provider registry has enforcement (count: $PROVIDER_REGISTRY)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Provider registry missing (internal/connectors/finance/write/registry/registry.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 6: Phase 17 events are defined
echo ""
echo "Check 6: Phase 17 events are defined..."

PHASE17_EVENTS=$(grep -c "Phase17" pkg/events/events.go 2>/dev/null || echo "0")

if [ "$PHASE17_EVENTS" -lt 30 ]; then
  echo -e "${RED}VIOLATION: Missing Phase 17 events (found: $PHASE17_EVENTS)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Phase 17 events defined (count: $PHASE17_EVENTS)${NC}"
fi

# Check 7: Payment draft content exists
echo ""
echo "Check 7: Payment draft content exists..."

if [ -f "pkg/domain/draft/payment_content.go" ]; then
  PAYMENT_CONTENT=$(grep -c "PaymentDraftContent\|PayeeID\|AmountCents" pkg/domain/draft/payment_content.go 2>/dev/null || echo "0")
  if [ "$PAYMENT_CONTENT" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Payment draft content incomplete (found: $PAYMENT_CONTENT)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Payment draft content exists (count: $PAYMENT_CONTENT)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Payment draft content missing (pkg/domain/draft/payment_content.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 8: Finance execution has idempotency
echo ""
echo "Check 8: Finance execution has idempotency..."

IDEMPOTENCY=$(grep -c "IdempotencyKey\|idempotencyKey\|idempotent" internal/connectors/finance/write/providers/mock/mock.go 2>/dev/null || echo "0")

if [ "$IDEMPOTENCY" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Mock connector missing idempotency (found: $IDEMPOTENCY)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Mock connector has idempotency (count: $IDEMPOTENCY)${NC}"
fi

# Check 9: Write connector implements abort
echo ""
echo "Check 9: Write connector implements abort..."

ABORT=$(grep -c "func.*Abort\|abortedEnvelopes\|ErrExecutionAborted" internal/connectors/finance/write/providers/mock/mock.go 2>/dev/null || echo "0")

if [ "$ABORT" -lt 2 ]; then
  echo -e "${RED}VIOLATION: Mock connector missing abort support (found: $ABORT)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Mock connector has abort support (count: $ABORT)${NC}"
fi

# Check 10: Demo tests exist
echo ""
echo "Check 10: Phase 17 demo tests exist..."

if [ -f "internal/demo_phase17_finance_execution/demo_test.go" ]; then
  DEMO_TESTS=$(grep -c "func Test" internal/demo_phase17_finance_execution/demo_test.go 2>/dev/null || echo "0")
  if [ "$DEMO_TESTS" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Insufficient demo tests (found: $DEMO_TESTS)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Demo tests exist (count: $DEMO_TESTS)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Demo tests missing (internal/demo_phase17_finance_execution/demo_test.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 11: ActionFinancePayment is defined for intersection approvals
echo ""
echo "Check 11: ActionFinancePayment is defined for intersection approvals..."

FINANCE_ACTION=$(grep -c "ActionFinancePayment\|finance_payment" pkg/domain/intersection/types.go 2>/dev/null || echo "0")

if [ "$FINANCE_ACTION" -lt 1 ]; then
  echo -e "${RED}VIOLATION: ActionFinancePayment not defined in intersection (found: $FINANCE_ACTION)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: ActionFinancePayment defined (count: $FINANCE_ACTION)${NC}"
fi

# Summary
echo ""
echo "=== Summary ==="
if [ $VIOLATIONS -eq 0 ]; then
  echo -e "${GREEN}All finance execution guardrails passed.${NC}"
  exit 0
else
  echo -e "${RED}Found $VIOLATIONS guardrail violation(s).${NC}"
  exit 1
fi
