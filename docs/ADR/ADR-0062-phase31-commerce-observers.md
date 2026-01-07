# ADR-0062: Phase 31 - Commerce Observers (Silent by Default)

**Status:** Accepted
**Date:** 2025-01-07
**Version:** Phase 31

## Context

QuantumLife has established a pattern of silent observation across multiple domains:
- Phase 18.7: Mirror proof (what we saw but didn't store)
- Phase 22: Quiet inbox mirror (Gmail activity without content)
- Phase 29: Finance mirror (TrueLayer data as abstract buckets)

Commerce data (food delivery, transport, retail, subscriptions) represents another class of behavioral signals. Unlike finance data which has immediate obligations (bills, payments), commerce data represents long-horizon patterns that MAY matter someday but usually do not.

The key insight: **Commerce is NOT finance. Commerce is NOT budgeting. Commerce is NOT insights.**

Previous approaches to commerce data in consumer apps have created problems:
- Spend analysis creates anxiety ("You spent £X on food delivery this month")
- Merchant categorization enables surveillance capitalism
- Optimization suggestions imply judgment ("Consider reducing...")
- Push notifications create urgency where none exists

## Decision

We implement Commerce Observers with a fundamentally different philosophy:

**Default outcome: NOTHING SHOWN.**

Commerce is observed. Nothing else.

### Core Invariants

1. **NO amounts** - No prices, totals, costs, or spending figures
2. **NO merchant names** - No vendor identification, store names, or retailers
3. **NO timestamps** - No purchase times, only abstract period buckets
4. **NO spend analysis** - No budgets, savings, optimization, or reduction advice
5. **NO recommendations** - No suggestions, tips, or "consider" language
6. **NO execution paths** - No actions, drafts, or invitations triggered
7. **NO trust impact** - Does not affect trust score or execution eligibility

### Domain Model

```go
// Bucket enums (validated)
type CategoryBucket string   // food_delivery, transport, retail, subscriptions, utilities, other
type FrequencyBucket string  // rare, occasional, frequent
type StabilityBucket string  // stable, drifting, volatile

// Core observation
type CommerceObservation struct {
    Category     CategoryBucket
    Frequency    FrequencyBucket
    Stability    StabilityBucket
    Period       string  // "2024-W03" format
    EvidenceHash string  // Never raw data
}

// Mirror page (max 3 categories, max 2 lines)
type CommerceMirrorPage struct {
    Title      string           // "Seen, quietly."
    Lines      []string         // Calm sentences only
    Buckets    []CategoryBucket // Max 3
    StatusHash string
}
```

### Silence as Success

The mirror page shows:
- "Some routines appear to be holding steady."
- "A few patterns were noticed — nothing urgent."
- "Quiet observation continues."

The page does NOT show:
- Amounts or totals
- Merchant names
- Timestamps
- Comparisons
- Recommendations
- Actions

### Single Whisper Rule

The commerce cue has **lowest priority** in the whisper chain:
1. Surface cue (highest)
2. Proof cue
3. First-minutes cue
4. Reality cue
5. Shadow receipt cue
6. Trust action cue
7. **Commerce cue (lowest)**

If any higher-priority cue is active, commerce cue is suppressed.

## Why Commerce is NOT Finance

| Aspect | Finance | Commerce |
|--------|---------|----------|
| Urgency | Bills are due | Patterns are not urgent |
| Action | Payments required | No action needed |
| Visibility | Must see | May see |
| Default | Show obligations | Show nothing |
| Trust impact | Yes | No |

## Why This Phase Comes After Phase 30A

Commerce observation requires:
1. Identity foundation (Phase 30A) for multi-device consistency
2. Trust baseline (Phase 20) to prove restraint works
3. Mirror patterns (Phase 18.7, 22, 29) as established precedent

Commerce is the final observation domain before any execution expansion.

## Consequences

### Positive
- Demonstrates ultimate restraint (observing without acting)
- Establishes pattern for future observation domains
- Proves system can hold long-horizon signals silently
- No anxiety created by surfacing commerce data

### Negative
- Limited utility in short term (by design)
- May seem "useless" to users expecting insights (this is correct)

### Mitigations
- Clear documentation that silence is success
- No marketing of "commerce insights" or similar
- Commerce cue only shown when explicitly relevant

## Implementation

### Files Created
- `pkg/domain/commerceobserver/types.go` - Domain types
- `internal/commerceobserver/engine.go` - Engine logic
- `internal/persist/commerceobserver_store.go` - Storage (30-day bounded)
- `scripts/guardrails/commerce_observer_enforced.sh` - CI enforcement
- `internal/demo_phase31_commerce_observer/demo_test.go` - Tests

### Web Route
- `GET /mirror/commerce` - Commerce mirror page (no actions, back link only)

### Events
- `phase31.commerce.observed` - Observation recorded
- `phase31.commerce.mirror.rendered` - Page viewed
- `phase31.commerce.cue.computed` - Cue availability checked

## References

- ADR-0060: Phase 29 - TrueLayer Read-Only Finance Mirror
- ADR-0043: Phase 18.7 - Mirror Proof
- ADR-0044: Phase 22 - Quiet Inbox Mirror
- Phase 31 specification in canon
