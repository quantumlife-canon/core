#!/usr/bin/env bash
# commerce_drafts_enforced.sh
#
# Guardrail: Commerce drafts MUST be proposals only (no execution).
#
# CRITICAL: Commerce drafts are NEVER executed automatically.
# CRITICAL: VendorContactRef MUST NOT be free-text.
# CRITICAL: Deterministic generation required.
# CRITICAL: No external writes from commerce draft code.
#
# Reference: Phase 9 Commerce Action Drafts (ADR-0025)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Guardrail: Commerce Drafts Enforced ==="
echo ""

VIOLATIONS=0

# Check 1: VendorContactRef must use factory functions
echo "Check 1: VendorContactRef uses factory functions..."

# Look for direct VendorContactRef string construction
DIRECT_CONTACT=$(grep -rn 'VendorContactRef("' \
  --include="*.go" \
  . 2>/dev/null | \
  grep -v "commerce_content.go" | \
  grep -v "_test.go" || true)

if [ -n "$DIRECT_CONTACT" ]; then
  echo -e "${RED}VIOLATION: Direct VendorContactRef construction (should use KnownVendorContact/UnknownVendorContact):${NC}"
  echo "$DIRECT_CONTACT"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: VendorContactRef uses factory functions${NC}"
fi

# Check 2: No external API calls in commerce drafts
echo ""
echo "Check 2: No external API calls in commerce draft code..."

API_CALLS=$(grep -rn "http\.Get\|http\.Post\|http\.Do\|net/http" \
  --include="*.go" \
  internal/drafts/commerce/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$API_CALLS" ]; then
  echo -e "${RED}VIOLATION: HTTP calls found in commerce drafts:${NC}"
  echo "$API_CALLS"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No HTTP calls in commerce drafts${NC}"
fi

# Check 3: No email sending from commerce drafts
echo ""
echo "Check 3: No email sending from commerce draft code..."

EMAIL_SEND=$(grep -rn "SendEmail\|SendReply\|smtp\|SMTP" \
  --include="*.go" \
  internal/drafts/commerce/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$EMAIL_SEND" ]; then
  echo -e "${RED}VIOLATION: Email sending found in commerce drafts:${NC}"
  echo "$EMAIL_SEND"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No email sending in commerce drafts${NC}"
fi

# Check 4: No goroutines in commerce drafts
echo ""
echo "Check 4: No goroutines in commerce draft code..."

GOROUTINES=$(grep -rn "go func\|go [a-zA-Z]" \
  --include="*.go" \
  internal/drafts/commerce/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$GOROUTINES" ]; then
  echo -e "${RED}VIOLATION: Goroutines found in commerce drafts:${NC}"
  echo "$GOROUTINES"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No goroutines in commerce drafts${NC}"
fi

# Check 5: No time.Now() in commerce drafts (must use injected clock)
echo ""
echo "Check 5: No time.Now() in commerce draft code..."

TIME_NOW=$(grep -rn "time\.Now()" \
  --include="*.go" \
  internal/drafts/commerce/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$TIME_NOW" ]; then
  echo -e "${RED}VIOLATION: time.Now() found (should use injected clock):${NC}"
  echo "$TIME_NOW"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No time.Now() in commerce drafts${NC}"
fi

# Check 6: Commerce content types implement CanonicalString
echo ""
echo "Check 6: Commerce content types implement CanonicalString..."

CANONICAL_COUNT=$(grep -c "func.*CanonicalString.*string" \
  pkg/domain/draft/commerce_content.go 2>/dev/null || echo "0")

if [ "$CANONICAL_COUNT" -lt 4 ]; then
  echo -e "${RED}VIOLATION: Missing CanonicalString implementations (found: $CANONICAL_COUNT, expected: 4):${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: All commerce content types implement CanonicalString (count: $CANONICAL_COUNT)${NC}"
fi

# Check 7: Commerce draft types are defined
echo ""
echo "Check 7: Commerce draft types are defined..."

DRAFT_TYPES=$(grep -c "DraftType.*=" \
  pkg/domain/draft/commerce_content.go 2>/dev/null || echo "0")

if [ "$DRAFT_TYPES" -lt 4 ]; then
  echo -e "${RED}VIOLATION: Missing commerce draft types (found: $DRAFT_TYPES, expected: 4):${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Commerce draft types defined (count: $DRAFT_TYPES)${NC}"
fi

# Check 8: IsCommerceDraft function exists
echo ""
echo "Check 8: IsCommerceDraft helper exists..."

IS_COMMERCE=$(grep -c "func IsCommerceDraft" \
  pkg/domain/draft/commerce_content.go 2>/dev/null || echo "0")

if [ "$IS_COMMERCE" -eq 0 ]; then
  echo -e "${RED}VIOLATION: IsCommerceDraft function not found${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: IsCommerceDraft function exists${NC}"
fi

# Check 9: Commerce generator implements DraftGenerator interface
echo ""
echo "Check 9: Commerce generator implements DraftGenerator..."

CAN_HANDLE=$(grep -c "func.*CanHandle" \
  internal/drafts/commerce/engine.go 2>/dev/null || echo "0")

GENERATE=$(grep -c "func.*Generate.*GenerationResult" \
  internal/drafts/commerce/engine.go 2>/dev/null || echo "0")

if [ "$CAN_HANDLE" -eq 0 ] || [ "$GENERATE" -eq 0 ]; then
  echo -e "${RED}VIOLATION: Commerce generator doesn't implement DraftGenerator interface${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: Commerce generator implements DraftGenerator${NC}"
fi

# Check 10: No payment SDK usage in commerce drafts
echo ""
echo "Check 10: No payment SDK usage in commerce draft code..."

# Look for actual payment SDK imports/usage, not the word "payment" in text
PAYMENT_SDK=$(grep -rn "stripe\.\|Stripe\.\|paypal\.\|Paypal\.\|braintree\.\|square\." \
  --include="*.go" \
  internal/drafts/commerce/ 2>/dev/null | \
  grep -v "_test.go" || true)

if [ -n "$PAYMENT_SDK" ]; then
  echo -e "${RED}VIOLATION: Payment SDK usage found in commerce drafts:${NC}"
  echo "$PAYMENT_SDK"
  VIOLATIONS=$((VIOLATIONS + 1))
else
  echo -e "${GREEN}PASS: No payment SDK usage in commerce drafts${NC}"
fi

# Summary
echo ""
echo "=== Summary ==="
if [ $VIOLATIONS -eq 0 ]; then
  echo -e "${GREEN}All commerce draft guardrails passed.${NC}"
  exit 0
else
  echo -e "${RED}Found $VIOLATIONS guardrail violation(s).${NC}"
  exit 1
fi
