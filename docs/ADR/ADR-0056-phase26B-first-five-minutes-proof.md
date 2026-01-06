# ADR-0056: Phase 26B - First Five Minutes Proof

## Status
Accepted

## Context

After implementing Phase 26A (Guided Journey), we need a way to prove that the first-time user journey worked. Traditional approaches would use analytics, telemetry, or engagement dashboards. These approaches:

- Create surveillance anxiety
- Encourage optimization for metrics rather than user wellbeing
- Expose raw counts, timestamps, and identifiable patterns
- Generate urgency through "engagement" framing
- Build dashboards that become ends in themselves

QuantumLife's philosophy is different: we prove value through silence, not surveillance.

## Decision

Introduce a **First Five Minutes Receipt** - a single abstract, deterministic summary that answers:

1. Did the user connect?
2. Did data sync?
3. Did we hold things quietly?
4. Did one safe action occur (or not)?
5. Did silence remain intact?

### Why This Is Not Analytics

| Analytics | First Minutes Proof |
|-----------|---------------------|
| Tracks behavior | Summarizes outcomes |
| Raw counts | Magnitude buckets |
| Timestamps | Period buckets |
| Funnels | Single receipt |
| Optimization | Narrative |
| Dashboards | One card |
| Engagement | Restraint |

### Why Silence Is The Metric

The receipt proves we *didn't* interrupt. If no signals exist, the summary returns `nil` - and that's success. The absence of a receipt means everything worked so quietly it didn't need acknowledgment.

### Why A Single Receipt Beats Dashboards

- Dashboards invite staring
- Receipts invite glancing
- Dashboards suggest "improve these numbers"
- Receipts say "this is what happened"
- Dashboards persist
- Receipts fade

### Why This Is Safe To Show Investors

The receipt contains:
- Abstract signal kinds (connected, synced, held, etc.)
- Magnitude buckets (nothing, a_few, several)
- One calm sentence
- A deterministic hash

It does NOT contain:
- Email addresses, subjects, or senders
- Raw counts or timestamps
- User identifiers
- Behavioral sequences
- Conversion metrics

## Implementation

### Domain Model

```go
// pkg/domain/firstminutes/types.go

type FirstMinutesPeriod string  // Day bucket (YYYY-MM-DD)

type FirstMinutesSignalKind string
const (
    SignalConnected        = "connected"
    SignalSynced           = "synced"
    SignalMirrored         = "mirrored"
    SignalHeld             = "held"
    SignalActionPreviewed  = "action_previewed"
    SignalActionExecuted   = "action_executed"
    SignalSilencePreserved = "silence_preserved"
)

type MagnitudeBucket string
const (
    MagnitudeNothing = "nothing"
    MagnitudeAFew    = "a_few"
    MagnitudeSeveral = "several"
)

type FirstMinutesSummary struct {
    Period     FirstMinutesPeriod
    Signals    []FirstMinutesSignal
    CalmLine   string  // Single sentence
    StatusHash string  // 128-bit deterministic
}

type FirstMinutesDismissal struct {
    Period      FirstMinutesPeriod
    SummaryHash string
}
```

### Engine

```go
// internal/firstminutes/engine.go

type Engine struct {
    clock func() time.Time  // Clock injection
}

func NewEngine(clock func() time.Time) *Engine

// Reads from existing stores, no new data collection
func (e *Engine) ComputeSummary(inputs *FirstMinutesInputs) *FirstMinutesSummary

// Returns nil if no meaningful signals (silence is success)
```

### CalmLine Selection

Deterministic selection based on signal pattern:

| Pattern | CalmLine |
|---------|----------|
| Action executed | "One action happened. Silence resumed." |
| Action previewed | "One action was previewed. Nothing else needed you." |
| Held items | "A few things were seen and held without interruption." |
| Connected + synced + mirrored | "Your data arrived. We watched quietly." |
| Connected only | "You connected once, and we stayed quiet." |
| Silence preserved | "Nothing happened - and that was the point." |

### Persistence

```go
// internal/persist/firstminutes_store.go

type FirstMinutesStore struct {
    summaries   map[string]*FirstMinutesSummary  // period -> summary
    dismissals  map[string]*FirstMinutesDismissal
    maxPeriods  int  // 30 days bounded retention
    clock       func() time.Time
}
```

Constraints:
- One summary per period
- Hash-only storage (no raw content)
- Bounded retention (30 days)
- Append-only with storelog integration

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/first-minutes` | GET | Show receipt (single calm card) |
| `/first-minutes/dismiss` | POST | Dismiss receipt for current period |

### UI Template

```
+------------------------------------------+
|  Your first minutes, quietly.            |
|                                          |
|  "{CalmLine}"                            |
|                                          |
|  [connected] [synced] [held]  <- chips   |
|                                          |
|  Quiet is the outcome.                   |
|                                          |
|                          [Dismiss]       |
+------------------------------------------+
```

**No:** numbers, timelines, charts, "next steps", improvement language

### Whisper Integration

Priority order (single whisper rule):
1. Journey cue (26A)
2. Surface cue
3. Proof cue
4. **First-minutes cue (26B)** - lowest priority

Cue text: "If you ever wondered how the beginning went."

### Events

```go
Phase26BFirstMinutesComputed  = "phase26b.first_minutes.computed"
Phase26BFirstMinutesPersisted = "phase26b.first_minutes.persisted"
Phase26BFirstMinutesViewed    = "phase26b.first_minutes.viewed"
Phase26BFirstMinutesDismissed = "phase26b.first_minutes.dismissed"
```

All payloads contain hashes only - never identifiers.

## Guardrails

52 guardrails enforce:

1. **Package structure** - domain, engine, store exist
2. **Stdlib only** - no external dependencies
3. **No time.Now()** - clock injection only
4. **No goroutines** - synchronous execution
5. **Domain model** - all types and constants
6. **Engine** - no side effects, deterministic
7. **Persistence** - bounded retention, hash-only
8. **Events** - all 4 events defined
9. **Web routes** - /first-minutes endpoints
10. **Privacy** - no forbidden tokens
11. **No analytics patterns** - no metrics, dashboards

## Store Dependencies (Read-Only)

Phase 26B reads from existing stores - no new data collection:

| Store | Method | Signal |
|-------|--------|--------|
| ConnectionStore | `State()` | connected |
| SyncReceiptStore | `GetLatestByCircle()` | synced |
| QuietMirrorStore | `GetLatestForPeriod()` | mirrored |
| TrustStore | `GetRecentMeaningfulSummary()` | held |
| FirstActionStore | `GetForPeriod()` | action_previewed |
| UndoableExecStore | `GetForPeriod()` | action_executed |

## Consequences

### Positive
1. Proves value without surveillance
2. Single artifact vs dashboard complexity
3. Deterministic and auditable
4. Privacy-preserving by design
5. Dismissable and non-intrusive

### Negative
1. Cannot optimize funnels (by design)
2. Cannot track individual users (by design)
3. Cannot A/B test (by design)

### Neutral
1. Silence as success may confuse traditional product teams
2. Receipt availability depends on having any signals

## Absolute Constraints

- stdlib only
- No new concepts introduced
- No config flags
- No metrics
- No logging beyond events
- No user education copy
- No "improvement" language
- No calls to LLMs
- No background processing

**Silence remains the success state.**

## Files Created

| File | Purpose |
|------|---------|
| `pkg/domain/firstminutes/types.go` | Domain model |
| `internal/firstminutes/engine.go` | Engine |
| `internal/persist/firstminutes_store.go` | Persistence |
| `scripts/guardrails/first_minutes_enforced.sh` | Guardrails (52 checks) |
| `internal/demo_phase26B_first_minutes/demo_test.go` | Demo tests (17 tests) |

## Files Modified

| File | Changes |
|------|---------|
| `pkg/domain/storelog/log.go` | Added FirstMinutes record types |
| `pkg/events/events.go` | Added Phase 26B events |
| `cmd/quantumlife-web/main.go` | Added routes, handlers, whisper integration |

## Related ADRs

- ADR-0055: Phase 26A - Guided Journey
- ADR-0054: Phase 25 - First Undoable Execution
- ADR-0053: Phase 24 - First Reversible Real Action
- ADR-0052: Phase 23 - Gentle Action Invitation

## References

- `docs/QUANTUMLIFE_CANON_V1.md` - Core philosophy
- `docs/HUMAN_GUARANTEES_V1.md` - Trust contracts
