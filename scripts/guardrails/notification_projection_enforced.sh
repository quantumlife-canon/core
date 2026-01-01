#!/usr/bin/env bash
# notification_projection_enforced.sh
#
# Guardrail: Notification projection MUST be deterministic and policy-driven.
#
# CRITICAL: No goroutines in notification packages.
# CRITICAL: No time.Now() - use clock injection.
# CRITICAL: Web badges are primary; email is first outbound channel.
# CRITICAL: Push/SMS are mock-only (ErrNotSupported in real mode).
# CRITICAL: No auto-retry patterns.
# CRITICAL: All notifications are policy-gated.
#
# Reference: Phase 16 Notification Projection

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Guardrail: Notification Projection Enforced ==="
echo ""

VIOLATIONS=0

# Check 1: Notify domain uses deterministic hashing
echo "Check 1: Notify domain uses deterministic hashing..."

HASH_USE=$(grep -c "ComputeHash\|CanonicalString\|sha256" pkg/domain/notify/types.go 2>/dev/null || echo "0")

if [ "$HASH_USE" -lt 5 ]; then
  echo -e "${RED}VIOLATION: Notify domain missing deterministic hashing (found: $HASH_USE)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Notify domain uses deterministic hashing (count: $HASH_USE)${NC}"
fi

# Check 2: Notification policy has quiet hours
echo ""
echo "Check 2: Notification policy has quiet hours..."

QUIET_HOURS=$(grep -c "QuietHours\|IsQuietTime" pkg/domain/policy/types.go 2>/dev/null || echo "0")

if [ "$QUIET_HOURS" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Policy missing quiet hours support (found: $QUIET_HOURS)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Policy has quiet hours support (count: $QUIET_HOURS)${NC}"
fi

# Check 3: No goroutines in notify packages
echo ""
echo "Check 3: No goroutines in notify packages..."

GOROUTINES=$(grep -rn "go func\|go [a-zA-Z]" \
  --include="*.go" \
  pkg/domain/notify/ internal/notifyplan/ internal/notifyexec/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$GOROUTINES" ]; then
  echo -e "${RED}VIOLATION: Goroutines found in notify packages:${NC}"
  echo "$GOROUTINES"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No goroutines in notify packages${NC}"
fi

# Check 4: No time.Now() in notify packages
echo ""
echo "Check 4: No time.Now() in notify packages..."

TIME_NOW=$(grep -rn "time\.Now()" \
  --include="*.go" \
  pkg/domain/notify/ internal/notifyplan/ internal/notifyexec/ 2>/dev/null | \
  grep -v "_test.go" | \
  grep -v "^[^:]*:[0-9]*:\s*//" || true)

if [ -n "$TIME_NOW" ]; then
  echo -e "${RED}VIOLATION: time.Now() found (should use clock injection):${NC}"
  echo "$TIME_NOW"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No time.Now() in notify packages${NC}"
fi

# Check 5: Planner exists with deterministic output
echo ""
echo "Check 5: Notification planner exists..."

if [ -f "internal/notifyplan/planner.go" ]; then
  PLANNER_HASH=$(grep -c "PolicyHash\|ComputeHash\|deterministic" internal/notifyplan/planner.go 2>/dev/null || echo "0")
  if [ "$PLANNER_HASH" -lt 2 ]; then
    echo -e "${RED}VIOLATION: Planner missing deterministic patterns (found: $PLANNER_HASH)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Notification planner exists with deterministic patterns${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Notification planner missing (internal/notifyplan/planner.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 6: Executor exists with envelope pattern
echo ""
echo "Check 6: Notification executor exists..."

if [ -f "internal/notifyexec/executor.go" ]; then
  EXEC_PATTERN=$(grep -c "Envelope\|Execute\|PolicySnapshot" internal/notifyexec/executor.go 2>/dev/null || echo "0")
  if [ "$EXEC_PATTERN" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Executor missing envelope pattern (found: $EXEC_PATTERN)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Notification executor exists with envelope pattern${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Notification executor missing (internal/notifyexec/executor.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 7: Push/SMS blocked in real mode
echo ""
echo "Check 7: Push/SMS blocked in real mode..."

REAL_MODE_BLOCK=$(grep -c "realMode\|ErrChannelBlocked\|ErrNotSupported" internal/notifyexec/executor.go 2>/dev/null || echo "0")

if [ "$REAL_MODE_BLOCK" -lt 3 ]; then
  echo -e "${RED}VIOLATION: Push/SMS blocking not implemented (found: $REAL_MODE_BLOCK)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Push/SMS blocked in real mode${NC}"
fi

# Check 8: Phase 16 events are defined
echo ""
echo "Check 8: Phase 16 events are defined..."

PHASE16_EVENTS=$(grep -c "Phase16\|phase16" pkg/events/events.go 2>/dev/null || echo "0")

if [ "$PHASE16_EVENTS" -lt 20 ]; then
  echo -e "${RED}VIOLATION: Missing Phase 16 events (found: $PHASE16_EVENTS)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Phase 16 events defined (count: $PHASE16_EVENTS)${NC}"
fi

# Check 9: Phase 16 storelog record types are defined
echo ""
echo "Check 9: Phase 16 storelog record types are defined..."

RECORD_TYPES=$(grep -c "RecordTypeNotification\|RecordTypeNotify" pkg/domain/storelog/log.go 2>/dev/null || echo "0")

if [ "$RECORD_TYPES" -lt 5 ]; then
  echo -e "${RED}VIOLATION: Missing Phase 16 storelog record types (found: $RECORD_TYPES)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Phase 16 storelog record types defined (count: $RECORD_TYPES)${NC}"
fi

# Check 10: Demo tests exist
echo ""
echo "Check 10: Phase 16 demo tests exist..."

if [ -f "internal/demo_phase16_notifications/demo_test.go" ]; then
  DEMO_TESTS=$(grep -c "func Test" internal/demo_phase16_notifications/demo_test.go 2>/dev/null || echo "0")
  if [ "$DEMO_TESTS" -lt 5 ]; then
    echo -e "${RED}VIOLATION: Insufficient demo tests (found: $DEMO_TESTS)${NC}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    echo -e "${GREEN}PASS: Demo tests exist (count: $DEMO_TESTS)${NC}"
  fi
else
  echo -e "${RED}VIOLATION: Demo tests missing (internal/demo_phase16_notifications/demo_test.go)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 11: No auto-retry patterns
echo ""
echo "Check 11: No auto-retry patterns in notify packages..."

AUTO_RETRY=$(grep -rn "retry\|Retry\|backoff\|Backoff" \
  --include="*.go" \
  internal/notifyplan/ internal/notifyexec/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$AUTO_RETRY" ]; then
  echo -e "${RED}VIOLATION: Auto-retry patterns found:${NC}"
  echo "$AUTO_RETRY"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No auto-retry patterns${NC}"
fi

# Check 12: Web badge is primary channel
echo ""
echo "Check 12: Web badge is primary channel..."

WEB_BADGE=$(grep -c "ChannelWebBadge\|web_badge" pkg/domain/notify/types.go internal/notifyexec/executor.go 2>/dev/null || echo "0")

if [ "$WEB_BADGE" -lt 5 ]; then
  echo -e "${RED}VIOLATION: Web badge not primary (found: $WEB_BADGE)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Web badge is primary channel (count: $WEB_BADGE)${NC}"
fi

# Summary
echo ""
echo "=== Summary ==="
if [ $VIOLATIONS -eq 0 ]; then
  echo -e "${GREEN}All notification projection guardrails passed.${NC}"
  exit 0
else
  echo -e "${RED}Found $VIOLATIONS guardrail violation(s).${NC}"
  exit 1
fi
