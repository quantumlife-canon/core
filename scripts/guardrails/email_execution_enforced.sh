#!/usr/bin/env bash
# email_execution_enforced.sh
#
# Guardrail: Email writes MUST go through the execution boundary.
#
# CRITICAL: This is the ONLY path to external email writes.
# CRITICAL: Direct provider calls are FORBIDDEN outside the executor.
# CRITICAL: No auto-retries. No background execution.
# CRITICAL: Reply-only - no new thread creation.
#
# Reference: Phase 7 Email Execution Boundary

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Guardrail: Email Execution Boundary Enforced ==="
echo ""

VIOLATIONS=0

# Check 1: Email provider SendReply calls ONLY in executor
echo "Check 1: Email provider SendReply calls only in executor..."

# Find SendReply calls outside the allowed paths
SENDREPLY_CALLS=$(grep -rn "\.SendReply(" \
  --include="*.go" \
  . 2>/dev/null | \
  grep -v "internal/email/execution/executor.go" | \
  grep -v "_test.go" | \
  grep -v "interface.go" | \
  grep -v "mock.go" | \
  grep -v "google.go" | \
  grep -v "demo.go" || true)

if [ -n "$SENDREPLY_CALLS" ]; then
  echo -e "${RED}VIOLATION: SendReply called outside executor:${NC}"
  echo "$SENDREPLY_CALLS"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: SendReply calls only in executor${NC}"
fi

# Check 2: Envelope creation requires both snapshot hashes
echo ""
echo "Check 2: Envelope creation requires both snapshot hashes..."

# Check that NewEnvelopeFromDraft is the only way to create envelopes
DIRECT_ENVELOPE=$(grep -rn "execution\.Envelope{" \
  --include="*.go" \
  . 2>/dev/null | \
  grep -v "_test.go" | \
  grep -v "demo.go" | \
  grep -v "envelope.go" || true)

if [ -n "$DIRECT_ENVELOPE" ]; then
  echo -e "${RED}VIOLATION: Direct Envelope construction found (should use NewEnvelopeFromDraft):${NC}"
  echo "$DIRECT_ENVELOPE"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Envelope creation goes through factory${NC}"
fi

# Check 3: No raw HTTP calls to Gmail API outside provider (writes only)
echo ""
echo "Check 3: No raw Gmail WRITE calls outside provider..."

# Allow: providers/google/google.go (write provider)
# Allow: gmail_read/ (read integration is separate from write boundary)
GMAIL_CALLS=$(grep -rn "gmail.googleapis.com" \
  --include="*.go" \
  . 2>/dev/null | \
  grep -v "providers/google/google.go" | \
  grep -v "gmail_read/" || true)

if [ -n "$GMAIL_CALLS" ]; then
  echo -e "${RED}VIOLATION: Gmail WRITE API calls outside provider:${NC}"
  echo "$GMAIL_CALLS"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Gmail write calls only in provider${NC}"
fi

# Check 4: No goroutines in email execution packages
echo ""
echo "Check 4: No goroutines in email execution packages..."

# Look for "go func" or "go " followed by function call in internal/email
GOROUTINES=$(grep -rn "go func\|go [a-zA-Z]" \
  --include="*.go" \
  internal/email/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$GOROUTINES" ]; then
  echo -e "${RED}VIOLATION: Goroutines found in email execution:${NC}"
  echo "$GOROUTINES"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No goroutines in email execution${NC}"
fi

# Check 5: No auto-retry patterns
echo ""
echo "Check 5: No auto-retry patterns in email execution..."

RETRY_PATTERNS=$(grep -rn "retry\|Retry\|backoff\|Backoff" \
  --include="*.go" \
  internal/email/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$RETRY_PATTERNS" ]; then
  echo -e "${RED}VIOLATION: Retry patterns found in email execution:${NC}"
  echo "$RETRY_PATTERNS"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No retry patterns in email execution${NC}"
fi

# Check 6: Email execution events are defined
echo ""
echo "Check 6: Email execution events are defined..."

EMAIL_EVENTS=$(grep -c "EmailExecution" pkg/events/events.go || echo "0")

if [ "$EMAIL_EVENTS" -lt 5 ]; then
  echo -e "${RED}VIOLATION: Missing email execution events (found: $EMAIL_EVENTS)${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Email execution events defined (count: $EMAIL_EVENTS)${NC}"
fi

# Summary
echo ""
echo "=== Summary ==="
if [ $VIOLATIONS -eq 0 ]; then
  echo -e "${GREEN}All email execution boundary guardrails passed.${NC}"
  exit 0
else
  echo -e "${RED}Found $VIOLATIONS guardrail violation(s).${NC}"
  exit 1
fi
