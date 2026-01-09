# ADR-0086: Phase 48 — Market Signal Binding (Non-Extractive Marketplace v1)

## Status

Accepted

## Context

Phase 46 introduced Circle Registry + Packs (Marketplace v0) with `effect_no_power` semantics.
Phase 47 connected Marketplace Packs to actual observer capabilities (Coverage Realization).

Phase 48 binds **unmet necessities** to **available marketplace packs** without recommendations,
nudges, ranking, persuasion, or execution. This is **signal exposure only**, not a funnel.

The user has declared:
- Which circles matter via **Necessity Declarations** (Phase 45)
- What observer capabilities they have via **Coverage Plans** (Phase 47)
- What packs are available via **Marketplace** (Phase 46)

Phase 48 observes these three inputs and emits **MarketSignals** that say:
> "A pack exists that could cover this unmet necessity."

Phase 48 introduces market observability without influence.
It does not create a marketplace funnel.

## Decision

### Invariant 1: Signal Exposure Only

Market signals OBSERVE the gap between necessity and coverage.
They do NOT recommend, rank, score, or prompt installation.

### Invariant 2: Effect No Power

All market signals have `effect_no_power`. Signals cannot:
- Grant permission
- Enable actions
- Trigger interrupts
- Affect delivery or execution

### Invariant 3: Proof-Only Visibility

Market signals are surfaced ONLY as proof pages.
They are never pushed, never prioritized, never urgent.

### Invariant 4: No Recommendation Language

Signal copy MUST be neutral:
- ALLOWED: "Some needs you marked as important are not yet covered."
- FORBIDDEN: "We recommend...", "You should...", "Don't miss out...", "Limited time..."

### Invariant 5: No Ranking or Scoring

When multiple packs match a gap, they are emitted in deterministic (hash-sorted) order.
No pack is "better", "featured", or "recommended".

### Invariant 6: No Pricing or Urgency

Market signals contain:
- Necessity kind (abstract)
- Coverage gap kind (abstract)
- Pack ID hash (abstract)

Market signals do NOT contain:
- Prices
- Urgency indicators
- Time limits
- Conversion copy

### Invariant 7: Bounded Signal Emission

- Maximum 3 signals per circle per period
- Deterministic ordering by hash
- FIFO eviction for storage bounds

### Invariant 8: Silence as Default

If there are no coverage gaps, no signals are emitted.
Silence is the correct output for a complete coverage plan.

## Non-Goals

Phase 48 does NOT:
- Create a marketplace funnel
- Optimize for conversion
- A/B test signal copy
- Track click-through rates
- Personalize recommendations
- Influence interrupt decisions
- Affect pressure calculations

## Architecture

```
┌─────────────────────┐     ┌─────────────────────┐     ┌─────────────────────┐
│  Phase 45           │     │  Phase 47           │     │  Phase 46           │
│  NecessityDecl      │     │  CoveragePlan       │     │  AvailablePacks     │
└─────────┬───────────┘     └─────────┬───────────┘     └─────────┬───────────┘
          │                           │                           │
          └───────────────────────────┼───────────────────────────┘
                                      │
                                      ▼
                         ┌────────────────────────┐
                         │  MarketSignalEngine    │
                         │  (pure, deterministic) │
                         └────────────┬───────────┘
                                      │
                                      ▼
                         ┌────────────────────────┐
                         │  MarketSignal[]        │
                         │  - SignalID (hash)     │
                         │  - CircleHash          │
                         │  - NecessityKind       │
                         │  - CoverageGapKind     │
                         │  - PackIDHash          │
                         │  - effect_no_power     │
                         │  - proof_only          │
                         └────────────────────────┘
```

## Routes

| Method | Path                    | Purpose                    |
|--------|-------------------------|----------------------------|
| GET    | /proof/market           | View market signals proof  |
| POST   | /proof/market/dismiss   | Dismiss market proof cue   |

## Events

| Event                          | When                              |
|--------------------------------|-----------------------------------|
| Phase48MarketSignalGenerated   | Signal computed from inputs       |
| Phase48MarketProofViewed       | User viewed /proof/market         |
| Phase48MarketProofDismissed    | User dismissed market proof cue   |

## Consequences

### Positive
- Users can discover coverage gaps without manipulation
- Non-extractive marketplace respects user agency
- Silence remains the default outcome
- No growth mechanics or conversion pressure

### Negative
- Lower "engagement" by design
- No "featured" or "promoted" packs
- No analytics on pack discovery

### Neutral
- Implementation is intentionally boring
- No recommendation engine needed
- No ML scoring required

## References

- Phase 45: Circle Semantics & Necessity Declaration
- Phase 46: Circle Registry + Packs (Marketplace v0)
- Phase 47: Pack Coverage Realization
- ADR-0083: Phase 45 Circle Semantics
- ADR-0084: Phase 46 Circle Registry
- ADR-0085: Phase 47 Coverage Realization
