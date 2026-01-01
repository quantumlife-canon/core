#!/usr/bin/env bash
# policy_learning_enforced.sh
#
# Guardrail: Preference learning MUST be deterministic and rule-based.
#
# CRITICAL: No ML, neural networks, or stochastic algorithms.
# CRITICAL: Policy updates go through PolicyStore with version control.
# CRITICAL: Suppression rules have deterministic IDs (SHA256).
# CRITICAL: All decisions are explainable with canonical audit trail.
# CRITICAL: No time.Now() - use clock injection.
#
# Reference: Phase 14 Circle Policies + Preference Learning

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Guardrail: Policy Learning Enforced (Deterministic) ==="
echo ""

VIOLATIONS=0

# Check 1: No ML/AI patterns in preference learning
echo "Check 1: No ML/AI patterns in preference learning..."

ML_PATTERNS=$(grep -rn "neural\|tensorflow\|pytorch\|sklearn\|machine.learning\|gradient\|backprop" \
  --include="*.go" \
  internal/preflearn/ pkg/domain/policy/ pkg/domain/suppress/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$ML_PATTERNS" ]; then
  echo -e "${RED}VIOLATION: ML/AI patterns found in preference learning:${NC}"
  echo "$ML_PATTERNS"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No ML/AI patterns in preference learning${NC}"
fi

# Check 2: No random number generators in preference learning
echo ""
echo "Check 2: No random number generators in preference learning..."

RANDOM_PATTERNS=$(grep -rn "math/rand\|crypto/rand\|rand\.\|Random" \
  --include="*.go" \
  internal/preflearn/ pkg/domain/policy/ pkg/domain/suppress/ 2>/dev/null | \
  grep -v "_test.go" | \
  grep -v "// rand" || true)

if [ -n "$RANDOM_PATTERNS" ]; then
  echo -e "${RED}VIOLATION: Random patterns found in preference learning:${NC}"
  echo "$RANDOM_PATTERNS"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No random patterns in preference learning${NC}"
fi

# Check 3: Policy domain uses deterministic hashing
echo ""
echo "Check 3: Policy domain uses deterministic hashing..."

HASH_USE=$(grep -c "ComputeHash\|CanonicalString\|sha256" pkg/domain/policy/types.go 2>/dev/null || echo "0")

if [ "$HASH_USE" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Policy domain missing deterministic hashing (found: $HASH_USE)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Policy domain uses deterministic hashing (count: $HASH_USE)${NC}"
fi

# Check 4: Suppression rules have deterministic IDs
echo ""
echo "Check 4: Suppression rules have deterministic IDs..."

SUPP_HASH=$(grep -c "computeRuleID\|CanonicalString\|sha256" pkg/domain/suppress/types.go 2>/dev/null || echo "0")

if [ "$SUPP_HASH" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Suppression rules missing deterministic IDs (found: $SUPP_HASH)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Suppression rules have deterministic IDs (count: $SUPP_HASH)${NC}"
fi

# Check 5: Explainability module exists
echo ""
echo "Check 5: Explainability module exists..."

if [ -f "pkg/domain/interrupt/explain.go" ]; then
  EXPLAIN_METHODS=$(grep -c "ExplainRecord\|ExplainBuilder\|FormatForUI" pkg/domain/interrupt/explain.go 2>/dev/null || echo "0")
  if [ "$EXPLAIN_METHODS" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Explainability module incomplete (found: $EXPLAIN_METHODS methods)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Explainability module complete (methods: $EXPLAIN_METHODS)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Explainability module missing (pkg/domain/interrupt/explain.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 6: No time.Now() in policy/suppress packages
echo ""
echo "Check 6: No time.Now() in policy/suppress packages..."

# Exclude comments (lines starting with // after whitespace) and test files
TIME_NOW=$(grep -rn "time\.Now()" \
  --include="*.go" \
  internal/preflearn/ pkg/domain/policy/ pkg/domain/suppress/ internal/persist/ 2>/dev/null | \
  grep -v "_test.go" | \
  grep -v "^[^:]*:[0-9]*:\s*//" || true)

if [ -n "$TIME_NOW" ]; then
  echo -e "${RED}VIOLATION: time.Now() found (should use clock injection):${NC}"
  echo "$TIME_NOW"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No time.Now() in policy/suppress packages${NC}"
fi

# Check 7: No goroutines in preference learning
echo ""
echo "Check 7: No goroutines in preference learning..."

GOROUTINES=$(grep -rn "go func\|go [a-zA-Z]" \
  --include="*.go" \
  internal/preflearn/ pkg/domain/policy/ pkg/domain/suppress/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$GOROUTINES" ]; then
  echo -e "${RED}VIOLATION: Goroutines found in preference learning:${NC}"
  echo "$GOROUTINES"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No goroutines in preference learning${NC}"
fi

# Check 8: Phase 14 events are defined
echo ""
echo "Check 8: Phase 14 events are defined..."

PHASE14_EVENTS=$(grep -c "Phase14\|phase14" pkg/events/events.go 2>/dev/null || echo "0")

if [ "$PHASE14_EVENTS" -lt 10 ]; then
  echo -e "${RED}VIOLATION: Missing Phase 14 events (found: $PHASE14_EVENTS)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Phase 14 events defined (count: $PHASE14_EVENTS)${NC}"
fi

# Check 9: PolicyStore uses storelog for persistence
echo ""
echo "Check 9: PolicyStore uses storelog for persistence..."

if [ -f "internal/persist/policy_store.go" ]; then
  STORELOG_USE=$(grep -c "storelog\|RecordTypePolicySet" internal/persist/policy_store.go 2>/dev/null || echo "0")
  if [ "$STORELOG_USE" -lt 2 ]; then
    echo -e "${RED}VIOLATION: PolicyStore not using storelog (found: $STORELOG_USE)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: PolicyStore uses storelog (count: $STORELOG_USE)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: PolicyStore missing (internal/persist/policy_store.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 10: Preference learning engine has deterministic decisions
echo ""
echo "Check 10: Preference learning engine has decision records..."

if [ -f "internal/preflearn/engine.go" ]; then
  DECISION_RECORDS=$(grep -c "DecisionRecord\|ApplyResult\|CanonicalString" internal/preflearn/engine.go 2>/dev/null || echo "0")
  if [ "$DECISION_RECORDS" -lt 3 ]; then
    echo -e "${RED}VIOLATION: Preference learning missing decision records (found: $DECISION_RECORDS)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Preference learning has decision records (count: $DECISION_RECORDS)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Preference learning engine missing (internal/preflearn/engine.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Summary
echo ""
echo "=== Summary ==="
if [ $VIOLATIONS -eq 0 ]; then
  echo -e "${GREEN}All policy learning guardrails passed.${NC}"
  exit 0
else
  echo -e "${RED}Found $VIOLATIONS guardrail violation(s).${NC}"
  exit 1
fi
