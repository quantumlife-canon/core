# ADR-0024: Phase 8 - Commerce Mirror (Email-Derived)

**Status:** Accepted
**Date:** 2025-01-15
**Authors:** QuantumLife Team
**Supersedes:** None
**Related:** ADR-0019 (Obligations), ADR-0020 (Interruptions), ADR-0023 (Quiet Loop)

## Context

QuantumLife serves users across UK, US, and India. These markets have vastly different vendor ecosystems:

- **UK:** Deliveroo, Just Eat, Tesco, Sainsburys, DPD, Royal Mail, Evri, etc.
- **US:** Uber, DoorDash, Amazon, FedEx, UPS, Netflix, etc.
- **India:** Swiggy, Zomato, Flipkart, Delhivery, Ola, etc.

Building direct integrations with 100+ vendors is impractical. However, commerce activity leaves a consistent signal: **transactional emails**.

Order confirmations, shipping updates, invoices, receipts, and subscription renewals all arrive in the user's inbox with relatively predictable patterns.

## Decision

**Phase 8 implements the "Commerce Mirror" - extracting canonical commerce events from email signals.**

### Core Principles

1. **Email-First Coverage**
   - Extract commerce events from EmailMessageEvent (already ingested in Phase 1)
   - No vendor APIs required
   - Immediate coverage across all markets

2. **Canonical Event Types**
   - Vendor-agnostic: `order_placed`, `shipment_update`, `invoice_issued`, `payment_receipt`, etc.
   - Vendor detection maps to canonical names (e.g., "deliveroo.co.uk" → "Deliveroo")
   - Category classification (food_delivery, grocery, courier, retail, etc.)

3. **Rule-Based Extraction**
   - Deterministic pattern matching
   - No ML/LLM for extraction (determinism required)
   - Subject + body snippets + sender domain as signals

4. **Obligation Integration**
   - Commerce events generate obligations:
     - Pending shipment → "Track shipment" followup
     - Invoice → "Pay invoice" obligation
     - Subscription renewal → "Review renewal" obligation
   - Obligations flow into interruption engine (Phase 3)

### What This Phase Is NOT

- No vendor APIs (Deliveroo API, Uber API, etc.)
- No payment execution (commerce is mirror-only)
- No LLM-based extraction (determinism first)

## Architecture

```
EmailMessageEvent
       │
       ▼
┌──────────────────────┐
│ Commerce Extractor   │
│  - Vendor detection  │
│  - Event type class  │
│  - Amount parsing    │
└──────────────────────┘
       │
       ▼
  CommerceEvent[]
       │
       ▼
┌──────────────────────┐
│ Commerce Obligation  │
│    Extractor         │
└──────────────────────┘
       │
       ▼
   Obligation[]
       │
       ▼
  Interruption Engine (Phase 3)
       │
       ▼
  NeedsYou Summary
```

### Commerce Event Types

| Type | Description | Generated From |
|------|-------------|----------------|
| `order_placed` | Order confirmed | "order confirmed", "thank you for your order" |
| `order_updated` | Order modified | "order updated", "changes to your order" |
| `shipment_update` | Tracking update | "shipped", "in transit", "out for delivery" |
| `invoice_issued` | Invoice/bill | "invoice", "payment due" |
| `payment_receipt` | Payment confirmation | "receipt", "payment confirmed" |
| `subscription_created` | New subscription | "welcome to", "subscription confirmed" |
| `subscription_renewed` | Renewal | "renewed", "auto-renewal" |
| `ride_receipt` | Ride completion | "trip receipt", "thanks for riding" |
| `refund_issued` | Refund processed | "refund", "money back" |

### Commerce Categories

| Category | Examples |
|----------|----------|
| `food_delivery` | Deliveroo, Uber Eats, Swiggy, Zomato, DoorDash |
| `grocery` | Tesco, Sainsburys, Ocado, Amazon Fresh |
| `courier` | DPD, Royal Mail, FedEx, UPS, Delhivery |
| `ride_hailing` | Uber, Lyft, Ola, Bolt |
| `retail` | Amazon, eBay, Flipkart |
| `utilities` | British Gas, EDF, Octopus Energy |
| `subscriptions` | Netflix, Spotify, Apple |

### Amount Parsing

Multi-currency support:
- **GBP:** `£24.99`, `GBP 24.99`
- **USD:** `$15.00`, `USD 15.00`
- **EUR:** `€10.50`, `EUR 10.50`
- **INR:** `₹450`, `Rs. 450`, `INR 450`

Amounts are stored in cents (minor units) for precision.

### Commerce Triggers (Interruptions)

| Trigger | Level | Description |
|---------|-------|-------------|
| `commerce_invoice_due` | Queued | Invoice needs payment |
| `commerce_shipment_pending` | Ambient | Shipment in transit |
| `commerce_refund_pending` | Queued | Refund not yet settled |
| `commerce_subscription_renewed` | Ambient | Renewal for review |

## File Structure

```
pkg/domain/commerce/
  types.go              # CommerceEvent, types, canonical hashing
  types_test.go         # Determinism tests

internal/commerce/extract/
  engine.go             # Main extraction engine
  rules_vendor_signals.go  # Vendor detection patterns
  rules_event_types.go     # Event type classification
  parse_amount.go          # Currency amount parsing
  extract_test.go          # Extraction tests

internal/obligations/
  rules_commerce.go     # Commerce → Obligation mapping

internal/demo_phase8_commerce_mirror/
  demo.go               # Demo runner
  demo_test.go          # Demo tests
```

## Canon Compliance

| Invariant | Implementation |
|-----------|----------------|
| stdlib only | Yes - regexp, crypto/sha256, strings |
| No goroutines in pkg/internal | Yes - synchronous extraction |
| No time.Now() | Yes - clock injection |
| Deterministic | Yes - same emails → same events → same hashes |
| No auto-retry | Yes - single-pass extraction |
| Canonical hashing | Yes - pipe-delimited strings, not JSON |

## Future Path

Phase 8 lays foundation for:

1. **Connector Marketplace**
   - Vendor-specific connectors can augment email signals
   - E.g., Deliveroo connector provides real-time order tracking
   - Email signals become fallback/validation

2. **Receipt OCR**
   - PDF/image receipt parsing
   - Augments email-derived amounts

3. **Expense Categorization**
   - Commerce events feed into finance (Phase 8.5+)
   - Cross-reference with bank transactions

## Verification

```bash
# Run Phase 8 demo
make demo-phase8

# Run all tests
go test ./...

# Run guardrails
make guardrails
```

## Consequences

### Positive

- Immediate commerce coverage without vendor integrations
- Deterministic extraction suitable for Canon
- Cross-market support (UK/US/India)
- Foundation for connector marketplace

### Negative

- Email-dependent (no email = no commerce mirror)
- Pattern-based extraction may miss edge cases
- No real-time updates (depends on email polling)

### Neutral

- Vendor detection must be maintained as new vendors emerge
- Amount parsing must handle currency variations

## References

- [Phase 2: Obligation Extraction](ADR-0019-phase2-obligation-extraction.md)
- [Phase 3: Interruptions](ADR-0020-phase3-interruptions-and-digest.md)
- [Phase 6: Quiet Loop](ADR-0023-phase6-quiet-loop-web.md)
- [QuantumLife Canon V1](../QUANTUMLIFE_CANON_V1.md)
