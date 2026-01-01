#!/usr/bin/env bash
# household_approvals_enforced.sh
#
# Guardrail: Household approvals MUST be deterministic and signed.
#
# CRITICAL: No goroutines in approval domain packages.
# CRITICAL: No time.Now() - use clock injection.
# CRITICAL: All approvals require Ed25519 signatures.
# CRITICAL: Approval states have deterministic IDs (SHA256).
# CRITICAL: Tokens are signed and verified before use.
# CRITICAL: Ledger uses storelog for append-only persistence.
#
# Reference: Phase 15 Household Approvals + Intersections

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Guardrail: Household Approvals Enforced (Deterministic) ==="
echo ""

VIOLATIONS=0

# Check 1: Intersection domain uses deterministic hashing
echo "Check 1: Intersection domain uses deterministic hashing..."

HASH_USE=$(grep -c "ComputeHash\|CanonicalString\|sha256" pkg/domain/intersection/types.go 2>/dev/null || echo "0")

if [ "$HASH_USE" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Intersection domain missing deterministic hashing (found: $HASH_USE)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Intersection domain uses deterministic hashing (count: $HASH_USE)${NC}"
fi

# Check 2: Approvalflow domain uses deterministic hashing
echo ""
echo "Check 2: Approvalflow domain uses deterministic hashing..."

FLOW_HASH=$(grep -c "ComputeHash\|CanonicalString\|sha256" pkg/domain/approvalflow/types.go 2>/dev/null || echo "0")

if [ "$FLOW_HASH" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Approvalflow domain missing deterministic hashing (found: $FLOW_HASH)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Approvalflow domain uses deterministic hashing (count: $FLOW_HASH)${NC}"
fi

# Check 3: Approval tokens are signable
echo ""
echo "Check 3: Approval tokens are signable..."

TOKEN_SIGN=$(grep -c "SignableString\|SignableBytes\|Signature" pkg/domain/approvaltoken/types.go 2>/dev/null || echo "0")

if [ "$TOKEN_SIGN" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Approval tokens missing signature support (found: $TOKEN_SIGN)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Approval tokens are signable (count: $TOKEN_SIGN)${NC}"
fi

# Check 4: No goroutines in approval domain
echo ""
echo "Check 4: No goroutines in approval domain..."

GOROUTINES=$(grep -rn "go func\|go [a-zA-Z]" \
  --include="*.go" \
  pkg/domain/intersection/ pkg/domain/approvalflow/ pkg/domain/approvaltoken/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$GOROUTINES" ]; then
  echo -e "${RED}VIOLATION: Goroutines found in approval domain:${NC}"
  echo "$GOROUTINES"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No goroutines in approval domain${NC}"
fi

# Check 5: No time.Now() in approval domain
echo ""
echo "Check 5: No time.Now() in approval domain..."

TIME_NOW=$(grep -rn "time\.Now()" \
  --include="*.go" \
  pkg/domain/intersection/ pkg/domain/approvalflow/ pkg/domain/approvaltoken/ 2>/dev/null | \
  grep -v "_test.go" | \
  grep -v "^[^:]*:[0-9]*:\s*//" || true)

if [ -n "$TIME_NOW" ]; then
  echo -e "${RED}VIOLATION: time.Now() found (should use clock injection):${NC}"
  echo "$TIME_NOW"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No time.Now() in approval domain${NC}"
fi

# Check 6: Approval ledger uses storelog
echo ""
echo "Check 6: Approval ledger uses storelog..."

if [ -f "internal/persist/approval_ledger.go" ]; then
  STORELOG_USE=$(grep -c "storelog\|RecordType" internal/persist/approval_ledger.go 2>/dev/null || echo "0")
  if [ "$STORELOG_USE" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Approval ledger not using storelog properly (found: $STORELOG_USE)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Approval ledger uses storelog (count: $STORELOG_USE)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Approval ledger missing (internal/persist/approval_ledger.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 7: Phase 15 events are defined
echo ""
echo "Check 7: Phase 15 events are defined..."

PHASE15_EVENTS=$(grep -c "Phase15\|phase15" pkg/events/events.go 2>/dev/null || echo "0")

if [ "$PHASE15_EVENTS" -lt 20 ]; then
  echo -e "${RED}VIOLATION: Missing Phase 15 events (found: $PHASE15_EVENTS)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Phase 15 events defined (count: $PHASE15_EVENTS)${NC}"
fi

# Check 8: Phase 15 storelog record types are defined
echo ""
echo "Check 8: Phase 15 storelog record types are defined..."

RECORD_TYPES=$(grep -c "RecordTypeIntersectionPolicy\|RecordTypeApprovalState\|RecordTypeApprovalToken" pkg/domain/storelog/log.go 2>/dev/null || echo "0")

if [ "$RECORD_TYPES" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Missing Phase 15 storelog record types (found: $RECORD_TYPES)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Phase 15 storelog record types defined (count: $RECORD_TYPES)${NC}"
fi

# Check 9: Approval state has status computation
echo ""
echo "Check 9: Approval state has status computation..."

STATUS_COMPUTE=$(grep -c "ComputeStatus\|StatusPending\|StatusApproved\|StatusRejected\|StatusExpired" pkg/domain/approvalflow/types.go 2>/dev/null || echo "0")

if [ "$STATUS_COMPUTE" -lt 5 ]; then
  echo -e "${RED}VIOLATION: Approval state missing status computation (found: $STATUS_COMPUTE)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Approval state has status computation (count: $STATUS_COMPUTE)${NC}"
fi

# Check 10: Demo tests exist
echo ""
echo "Check 10: Phase 15 demo tests exist..."

if [ -f "internal/demo_phase15_household_approvals/demo_test.go" ]; then
  DEMO_TESTS=$(grep -c "func Test" internal/demo_phase15_household_approvals/demo_test.go 2>/dev/null || echo "0")
  if [ "$DEMO_TESTS" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Insufficient demo tests (found: $DEMO_TESTS)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Demo tests exist (count: $DEMO_TESTS)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Demo tests missing (internal/demo_phase15_household_approvals/demo_test.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Summary
echo ""
echo "=== Summary ==="
if [ $VIOLATIONS -eq 0 ]; then
  echo -e "${GREEN}All household approvals guardrails passed.${NC}"
  exit 0
else
  echo -e "${RED}Found $VIOLATIONS guardrail violation(s).${NC}"
  exit 1
fi
