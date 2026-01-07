# ADR-0067: Phase 31.4 — External Pressure Circles + Intersection Pressure Map

## Status

Accepted

## Context

Phase 31 established commerce observation with abstract buckets (category, frequency, stability).
Phase 31.1 added Gmail receipt parsing. Phase 31.2 added TrueLayer transaction mapping.
Phase 31.3 proved real finance connections.

We now need to model external entities that exert pressure on sovereign circles without
becoming full circle members. Examples: recurring delivery services, subscription patterns,
transport habits. These are NOT people — they are abstract pressure sources.

## Decision

### Domain Model

Introduce `CircleKind` enum:
- `sovereign`: Human-owned circles (existing)
- `external_derived`: System-derived from commerce patterns

### External Derived Circle

An external circle is:
- Derived from commerce observations (not created by humans)
- Identified by hash (not raw merchant strings)
- Attached to a sovereign circle via `AttachedToCircle`
- CANNOT approve actions
- CANNOT execute anything
- CANNOT receive drafts
- CANNOT become sovereign

External Circle ID computation:
```
sha256("external|" + source_kind + "|" + category + "|" + sovereign_circle_id_hash)
```

### Pressure Map

A `PressureMapSnapshot` aggregates pressure from multiple sources:
- Max 3 items (bounded)
- Each item has: Category, Magnitude, Horizon
- Deterministic ordering: by category name, then by magnitude
- Hash-only storage

### Types

```go
type CircleKind string
const (
    CircleKindSovereign       CircleKind = "sovereign"
    CircleKindExternalDerived CircleKind = "external_derived"
)

type SourceKind string
const (
    SourceKindGmailReceipt   SourceKind = "gmail_receipt"
    SourceKindFinanceConnect SourceKind = "finance_connect"
)

type PressureCategory string
const (
    PressureCategoryDelivery     PressureCategory = "delivery"
    PressureCategoryTransport    PressureCategory = "transport"
    PressureCategoryRetail       PressureCategory = "retail"
    PressureCategorySubscription PressureCategory = "subscription"
    PressureCategoryOther        PressureCategory = "other"
)

type PressureMagnitude string
const (
    PressureMagnitudeNothing PressureMagnitude = "nothing"
    PressureMagnitudeAFew    PressureMagnitude = "a_few"
    PressureMagnitudeSeveral PressureMagnitude = "several"
)

type PressureHorizon string
const (
    PressureHorizonSoon    PressureHorizon = "soon"
    PressureHorizonLater   PressureHorizon = "later"
    PressureHorizonUnknown PressureHorizon = "unknown"
)
```

### Constraints

1. **No Merchant Strings**: Domain model contains NO raw merchant names, vendor identifiers,
   amounts, or timestamps
2. **Abstract Only**: Only category hints, magnitude buckets, and horizon buckets
3. **Read-Only Pressure**: External circles observe. They do NOT act.
4. **Bounded**: Max 3 pressure items per snapshot
5. **Deterministic**: Same inputs produce same hashes
6. **stdlib Only**: No cloud SDKs, no external dependencies
7. **Clock Injection**: No `time.Now()` — all timestamps passed explicitly
8. **No Goroutines**: All operations synchronous
9. **Hash-Only Persistence**: No raw content stored
10. **30-Day Retention**: Bounded storage with FIFO eviction

### Integration Points

Pressure is computed after:
1. Gmail sync (Phase 31.1) — when `CommerceObservationsPersisted` event fires
2. TrueLayer sync (Phase 31.3b) — when `TrueLayerIngestCompleted` event fires

### Web Route

`GET /reality/pressure` — Shows calm pressure proof page with:
- Period bucket (YYYY-MM-DD)
- Up to 3 pressure items (category + magnitude + horizon)
- No buttons, no actions, no recommendations

### Storelog Record Types

- `EXTERNAL_DERIVED_CIRCLE` — Persists external circle fingerprints
- `PRESSURE_MAP_SNAPSHOT` — Persists pressure map snapshots

### Events

- `phase31_4.pressure.computed` — Pressure map computed
- `phase31_4.pressure.persisted` — Pressure map persisted
- `phase31_4.external_circle.derived` — External circle derived
- `phase31_4.reality.viewed` — Pressure proof page viewed

## Consequences

### Positive

- Models external pressure without creating false relationships
- Maintains privacy (no merchant strings, only abstract categories)
- Bounded and deterministic
- Integrates cleanly with existing commerce observation pipeline
- Provides visible proof of pressure patterns

### Negative

- Additional complexity in circle model
- More storage for pressure snapshots
- External circles may confuse users expecting "people"

### Mitigations

- Clear UI labeling ("Pressure Patterns" not "External People")
- Single calm card design (no dashboards, no analytics)
- External circles are hidden from normal circle views

## References

- Canon v1 (Meaning)
- Technical Split v8 (Boundaries)
- ADR-0065: Phase 31.2 Commerce from Finance
- ADR-0066: Phase 31.3 Real Finance Only
