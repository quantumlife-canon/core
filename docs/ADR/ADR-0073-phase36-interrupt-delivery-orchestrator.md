# ADR-0073: Phase 36 — Interrupt Delivery Orchestrator (Explicit, Deterministic, Proof-First)

## Status

Accepted

## Context

Previous phases created the components of the interrupt pipeline:
- **Phase 31.4**: External Pressure Circles — identifies sources of pressure
- **Phase 32**: Pressure Decision Gate — classifies pressure into HOLD/SURFACE/INTERRUPT_CANDIDATE
- **Phase 33**: Interrupt Permission Contract — evaluates if candidates are permitted
- **Phase 34**: Permitted Interrupt Preview — web-only preview of permitted candidates
- **Phase 35/35b**: Push Transport — abstract delivery mechanism (stub/webhook/APNs)

These phases are isolated by design. Each does one thing:
- Phase 32 classifies, but does not notify
- Phase 33 permits, but does not deliver
- Phase 34 previews, but does not push
- Phase 35 transports, but does not decide

**Phase 36 orchestrates** these phases into a unified delivery action.

## Decision

### Delivery is Explicit, Not Automatic

Delivery occurs ONLY when:
1. A human triggers `POST /run/deliver`
2. Phase 32 produced an `INTERRUPT_CANDIDATE`
3. Phase 33 policy allows it
4. Phase 34 preview was shown or dismissed
5. Phase 35 transport is available

There is NO background delivery. NO scheduled jobs. NO polling loops.
The human must explicitly request delivery.

### Why Explicit?

1. **Trust preservation**: The system never interrupts without explicit permission.
2. **Determinism**: Same POST at same time produces same outcome.
3. **Auditability**: Every delivery is traceable to a human action.
4. **Restraint**: The system does less, not more.

### Why No Background Execution?

1. **Goroutines are forbidden**: Canon prohibits background concurrency.
2. **Human agency**: The human controls when delivery happens.
3. **Rate limiting**: Daily cap (2/day) is enforced at delivery time.
4. **Proof-first**: Every delivery produces a receipt that can be verified.

### Transport is Abstract

The orchestrator does NOT know what APNs is. It:
1. Asks Phase 35 engine for a transport request
2. Passes the request to the transport registry
3. Records the outcome

Transport-specific logic (JWT, HTTP/2, sealed secrets) lives in Phase 35b.
The orchestrator is transport-agnostic.

### Trust is Preserved

| Invariant | How Preserved |
|-----------|---------------|
| No background execution | POST-only delivery |
| Daily cap (2/day) | Enforced at orchestration time |
| Hash-only storage | Attempts stored by hash only |
| No identifiers in push | Payload is constant literal |
| Deterministic ordering | Candidates sorted by hash |
| Commerce never interrupts | Filtered by Phase 32 before reaching orchestrator |

## Consequences

### Positive

1. **Single delivery endpoint**: `POST /run/deliver` unifies the pipeline
2. **Proof page**: `GET /proof/delivery` shows what happened
3. **Explicit control**: Human decides when delivery is attempted
4. **No new decisions**: Orchestrator only calls existing engines

### Negative

1. **Manual trigger required**: Delivery won't happen automatically
2. **Delayed notification possible**: If human doesn't trigger, candidates age out

### Neutral

1. **Whisper cue**: Lowest priority cue after delivery
2. **Deduplication**: Same candidate won't be delivered twice in same period

## Domain Model

```go
// DeliveryCandidate represents an eligible interrupt candidate
type DeliveryCandidate struct {
    CandidateHash string           // From Phase 32
    CircleIDHash  string           // Circle that generated the pressure
    DecisionHash  string           // Phase 32 decision hash
    PeriodKey     string           // Day bucket (YYYY-MM-DD)
}

// DeliveryAttempt records a delivery attempt
type DeliveryAttempt struct {
    AttemptID      string           // SHA256 of canonical
    CandidateHash  string           // Which candidate
    CircleIDHash   string           // Which circle
    TransportKind  TransportKind    // stub | apns | webhook
    ResultBucket   ResultBucket     // sent | skipped | rejected | deduped
    ReasonBucket   ReasonBucket     // Why this result
    PeriodKey      string           // Day bucket
    AttemptBucket  string           // Time bucket (15-min)
    StatusHash     string           // Deterministic hash
}

// DeliveryReceipt is the proof of a delivery run
type DeliveryReceipt struct {
    ReceiptID      string           // SHA256 of canonical
    Attempts       []AttemptSummary // Bucketed attempt summaries
    SentCount      int              // Abstract: how many sent
    SkippedCount   int              // Abstract: how many skipped
    PeriodKey      string           // Day bucket
    StatusHash     string           // Deterministic hash
}
```

## Engine Flow

```
POST /run/deliver
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Load INTERRUPT_CANDIDATEs from Phase 32 store            │
│    (this period only, sorted by hash)                       │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Filter via Phase 33 policy                               │
│    - Check InterruptPolicy allowance                        │
│    - Respect MaxPerDay                                      │
│    - Block if TrustFragile                                  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Deduplicate                                              │
│    - Check delivery store for prior attempts                │
│    - Same circle+candidate+period = skip                    │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Check daily cap                                          │
│    - Count sent attempts today                              │
│    - Stop if >= 2                                           │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Build DeliveryAttempt list                               │
│    - Sort candidates by hash (deterministic)                │
│    - Take up to (2 - sent_today) candidates                 │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. For each attempt:                                        │
│    - Call Phase 35 engine.ComputeDeliveryAttempt()          │
│    - If request != nil, call transport.Send()               │
│    - Record attempt in delivery store                       │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. Build and return DeliveryReceipt                         │
└─────────────────────────────────────────────────────────────┘
```

## Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/run/deliver` | POST | Trigger delivery orchestration |
| `/proof/delivery` | GET | View delivery proof page |
| `/proof/delivery/dismiss` | POST | Dismiss delivery cue for period |

## Events

```
phase36.delivery.requested   - POST /run/deliver called
phase36.delivery.attempted   - Single delivery attempted
phase36.delivery.completed   - Delivery run completed
phase36.delivery.skipped     - Candidate skipped (policy/cap/dedup)
phase36.delivery.proved      - Proof page viewed
```

## Guardrails

The guardrail script MUST verify:
1. No goroutines in `internal/interruptdelivery/`
2. No `time.Now()` in `internal/interruptdelivery/`
3. No direct APNs/HTTP calls (uses transport interface)
4. No decision logic duplication (calls existing engines)
5. No merchant/person/institution strings in domain types
6. POST-only delivery endpoint
7. Max 2 deliveries/day enforced in engine
8. Transport interface used (not reimplemented)
9. Hash-only persistence
10. Deduplication by (candidate_hash, period)

## References

- ADR-0068: Phase 32 — Pressure Decision Gate
- ADR-0069: Phase 33 — Interrupt Permission Contract
- ADR-0070: Phase 34 — Permitted Interrupt Preview
- ADR-0071: Phase 35 — Push Transport
- ADR-0072: Phase 35b — APNs Sealed Secret Boundary
