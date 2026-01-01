# ADR-0025: Phase 9 - Commerce & Life Action Drafts (Still Zero Auto-Execution)

**Status:** Accepted
**Date:** 2025-01-15
**Authors:** QuantumLife Team
**Supersedes:** None
**Related:** ADR-0024 (Commerce Mirror), ADR-0021 (Phase 4 Drafts)

## Context

Phase 8 introduced the Commerce Mirror - extracting canonical commerce events from email signals. These events generate obligations:

- Pending shipment → "Track shipment" followup
- Invoice → "Pay invoice" obligation
- Subscription renewal → "Review renewal" obligation
- Refund pending → "Check refund status" followup

Phase 4 established the drafts-only pattern for email replies and calendar responses. Users review and approve drafts before any external action occurs.

Phase 9 extends this pattern to commerce-derived actions.

## Decision

**Phase 9 implements Commerce Action Drafts - proposals for commerce-related actions that require human approval.**

### Core Principles

1. **Drafts Only, No Execution**
   - Commerce drafts are proposals, not actions
   - NO external writes (emails, payments, API calls)
   - User must explicitly approve before any action

2. **Deterministic Generation**
   - Same inputs + clock = identical drafts
   - No randomness, no LLM generation
   - Canonical string serialization (pipe-delimited, NOT JSON)
   - SHA256-based deterministic IDs

3. **Vendor-Agnostic in Core**
   - Uses canonical CommerceEvent fields only
   - VendorContactRef is NOT free-text (derived or placeholder)
   - pkg/domain/draft/* is vendor-agnostic
   - Vendor-specific logic lives in internal/drafts/commerce/

### What This Phase Is NOT

- No payment execution (drafts only)
- No email sending (drafts only)
- No API calls to vendors
- No LLM-based content generation
- No automatic approval workflows

## Architecture

```
CommerceObligation (from Phase 8)
       │
       ▼
┌──────────────────────┐
│ Commerce Draft       │
│   Generator          │
│  (internal/drafts/   │
│       commerce/)     │
└──────────────────────┘
       │
       ▼
  CommerceDraft
       │
       ▼
┌──────────────────────┐
│ Draft Store          │
│  (deduplication,     │
│   TTL, status)       │
└──────────────────────┘
       │
       ▼
  NeedsYou Summary
  (proposed drafts)
       │
       ▼
  User Review & Approval
```

### Commerce Draft Types

| Draft Type | Source Obligation | Generated Content |
|------------|-------------------|-------------------|
| `shipment_followup` | `followup` (with tracking_id) | "Where is my order?" email draft |
| `refund_followup` | `followup` (with refund context) | "Refund status enquiry" email draft |
| `invoice_reminder` | `pay` | Payment reminder or vendor contact draft |
| `subscription_review` | `review` | Subscription review/cancel request draft |

### Content Types

Each draft type has a specific content structure:

```go
// ShipmentFollowUpContent
type ShipmentFollowUpContent struct {
    Vendor          string
    VendorContact   VendorContactRef  // NOT free-text
    OrderID         string
    TrackingID      string
    ShipmentStatus  string
    Subject         string
    Body            string
    OrderDate       string
    AmountFormatted string
}

// RefundFollowUpContent
type RefundFollowUpContent struct {
    Vendor          string
    VendorContact   VendorContactRef
    OrderID         string
    Subject         string
    Body            string
    RefundDate      string
    AmountFormatted string
}

// InvoiceReminderContent
type InvoiceReminderContent struct {
    Vendor          string
    VendorContact   VendorContactRef
    InvoiceID       string
    OrderID         string
    Subject         string
    Body            string
    InvoiceDate     string
    DueDate         string
    AmountFormatted string
    IsOverdue       bool
}

// SubscriptionReviewContent
type SubscriptionReviewContent struct {
    Vendor          string
    VendorContact   VendorContactRef
    SubscriptionID  string
    Action          string  // "review", "cancel", "keep"
    Subject         string
    Body            string
    RenewalDate     string
    NextRenewalDate string
    AmountFormatted string
}
```

### VendorContactRef

VendorContactRef ensures deterministic vendor contact references:

```go
// Known contact (email derived from vendor domain)
KnownVendorContact("support@amazon.co.uk")
// → "vendor-contact:email:support@amazon.co.uk"

// Unknown contact (deterministic placeholder)
UnknownVendorContact(vendorHash)
// → "vendor-contact:unknown:{hash}"
```

This prevents free-text recipient fields while maintaining determinism.

### Determinism Requirements

1. **CanonicalString()** method on all content types
2. Pipe-delimited format (NOT JSON)
3. Normalized values (lowercase, trimmed, newlines replaced)
4. Same inputs = same canonical string = same hash = same DraftID

Example canonical string:
```
shipment_followup|vendor:amazon|contact:vendor-contact:email:support@amazon.co.uk|order:123-456|tracking:trk789|status:in_transit|subject:where is my order|body:hello i placed an order...|date:2025-01-10|amount:£24.99
```

## Constraints

### Hard Constraints

1. **stdlib only** - No new dependencies
2. **Deterministic** - Same inputs + clock = identical outputs
3. **No time.Now()** in pkg/ or internal/ - injected clock only
4. **No goroutines** in internal/ or pkg/
5. **No auto-retry** - single attempt, fail cleanly
6. **Drafts are proposals** - NO external writes

### Vendor Contact Safety

VendorContactRef MUST be:
- Derived from known vendor domain (KnownVendorContact)
- OR a deterministic placeholder (UnknownVendorContact)

VendorContactRef MUST NOT be:
- Free-text user input
- LLM-generated
- Guessed or inferred

## Implementation

### Package Structure

```
pkg/domain/draft/
├── commerce_content.go      # Content types + VendorContactRef
├── commerce_content_test.go # Content tests
└── types.go                 # DedupKey() updated

internal/drafts/commerce/
├── engine.go               # Commerce draft generator
└── engine_test.go          # Generator tests

internal/demo_phase9_commerce_drafts/
├── demo.go                 # Phase 9 demo
└── demo_test.go            # Demo tests
```

### Generator Interface

```go
// DraftGenerator interface (from Phase 4)
type DraftGenerator interface {
    CanHandle(obl *obligation.Obligation) bool
    Generate(ctx GenerationContext) GenerationResult
}

// Commerce generator
func (e *Engine) CanHandle(obl *obligation.Obligation) bool {
    return obl != nil && obl.SourceType == "commerce"
}

func (e *Engine) Generate(ctx GenerationContext) GenerationResult {
    // Route to specific generator based on obligation type
    switch ctx.Obligation.Type {
    case obligation.ObligationFollowup:
        return e.generateFollowUp(ctx)
    case obligation.ObligationPay:
        return e.generateInvoiceReminder(ctx)
    case obligation.ObligationReview:
        return e.generateSubscriptionReview(ctx)
    default:
        return GenerationResult{Skipped: true, SkipReason: "unsupported type"}
    }
}
```

## Acceptance Criteria

### Functional

- [ ] Shipment followup drafts generated from commerce obligations
- [ ] Refund followup drafts generated from commerce obligations
- [ ] Invoice reminder drafts generated from commerce obligations
- [ ] Subscription review drafts generated from commerce obligations
- [ ] Drafts appear in "Needs You" summary
- [ ] Drafts can be approved/rejected via web UI

### Determinism

- [ ] Same inputs + clock produces identical DraftID
- [ ] Same inputs + clock produces identical DeterministicHash
- [ ] CanonicalString() is stable across runs

### Safety

- [ ] No external writes occur
- [ ] VendorContactRef is never free-text
- [ ] Drafts require explicit user approval

### Demo

- [ ] `make demo-phase9` runs successfully
- [ ] Demo output shows all four draft types
- [ ] Demo verifies determinism
- [ ] Demo verifies deduplication

## Future Work

Phase 10 and beyond may add:
- Approved draft execution (with additional safety layers)
- Vendor API integrations for specific high-value vendors
- Draft templates customization per circle
- Multi-party approval for high-value actions

These are NOT in scope for Phase 9.

## Testing

```bash
# Run commerce generator tests
go test ./internal/drafts/commerce/... -v

# Run commerce content tests
go test ./pkg/domain/draft/... -v -run Commerce

# Run Phase 9 demo tests
go test ./internal/demo_phase9_commerce_drafts/... -v

# Run full test suite
make ci
```
