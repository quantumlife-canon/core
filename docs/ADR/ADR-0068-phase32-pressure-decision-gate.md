# ADR-0068: Phase 32 — Pressure Decision Gate (Classification Only)

## Status

Accepted

## Context

Phase 31.4 established external pressure circles and pressure maps that aggregate abstract
pressure signals from commerce observations. We now need a decision boundary that classifies
whether pressure is even allowed to compete for the user's attention.

This is NOT an interruption system. It is a gate that answers one question:

**"Is this pressure allowed to compete for the user's attention?"**

The answer is one of three states: HOLD, SURFACE, or INTERRUPT_CANDIDATE.

## Decision

### Core Principle: Classification Only

This phase:
- Classifies pressure into three states
- Does NOT notify
- Does NOT execute
- Does NOT change UI
- Does NOT add buttons

The decision gate is invisible to users. Future phases may consume this output.

### Decision Kinds

```go
type PressureDecisionKind string

const (
    DecisionHold               PressureDecisionKind = "hold"
    DecisionSurface            PressureDecisionKind = "surface"
    DecisionInterruptCandidate PressureDecisionKind = "interrupt_candidate"
)
```

**HOLD** (default): Pressure is acknowledged but not surfaced. The system remains silent.

**SURFACE**: Pressure may appear in calm mirror views but does not interrupt.

**INTERRUPT_CANDIDATE**: Pressure is eligible to compete for interruption in future phases.
Rate-limited to max 2 per day.

### Classification Rules (Authoritative)

Apply in order. First match wins.

#### Rule 0 — Default
```
Decision = HOLD
```
This is the default. Silence is success.

#### Rule 1 — Commerce Never Interrupts Alone
```
IF CircleType == commerce
THEN Decision = HOLD
```
Commerce pressure (delivery, retail, subscriptions) never interrupts on its own.
It can only contribute to decisions when combined with human or institution pressure.

#### Rule 2 — Human + Horizon NOW
```
IF CircleType == human
AND Horizon == now
AND Magnitude != nothing
THEN Decision = INTERRUPT_CANDIDATE
```
A human circle with immediate pressure and non-trivial magnitude may interrupt.

#### Rule 3 — Institution + Deadline Pressure
```
IF CircleType == institution
AND Horizon == soon
AND Magnitude == several
THEN Decision = SURFACE
```
Institutional pressure (HMRC, bank, etc.) with deadline and significant volume surfaces.

#### Rule 4 — Trust Degradation Protection
```
IF TrustBaseline == fragile
THEN max Decision = SURFACE
```
When trust is fragile (from Phase 20), no interruption candidates are allowed.
This protects the user from pressure when the relationship is stressed.

#### Rule 5 — Rate Limit
```
IF interrupt_candidates_today >= 2
THEN downgrade to SURFACE
```
Maximum 2 interrupt candidates per day. Excess downgrades to surface.

### Inputs (Read-Only)

The decision engine consumes only existing artifacts:
- `PressureMapSnapshot` (Phase 31.4) — pressure by category/magnitude/horizon
- `CircleKind` — sovereign (human/institution) vs external_derived (commerce)
- `PressureMagnitude` — nothing | a_few | several
- `PressureHorizon` — now | soon | later | unknown
- `TrustBaselineStatus` — normal | fragile (from Phase 20, if available)

No new data sources are introduced.

### Output

```go
type PressureDecision struct {
    CircleIDHash     string               // Identifies the circle
    Decision         PressureDecisionKind // hold | surface | interrupt_candidate
    ReasonBucket     ReasonBucket         // Abstract reason enum
    PeriodKey        string               // Day bucket (YYYY-MM-DD)
    StatusHash       string               // Deterministic hash
}
```

### Reason Buckets

```go
type ReasonBucket string

const (
    ReasonDefault                   ReasonBucket = "default"
    ReasonCommerceNeverInterrupts   ReasonBucket = "commerce_never_interrupts"
    ReasonHumanNow                  ReasonBucket = "human_now"
    ReasonInstitutionDeadline       ReasonBucket = "institution_deadline"
    ReasonTrustFragileDowngrade     ReasonBucket = "trust_fragile_downgrade"
    ReasonRateLimitDowngrade        ReasonBucket = "rate_limit_downgrade"
)
```

### Persistence

Hash-only, append-only records:
- `PressureDecisionRecord` — persists decisions
- Period-keyed (daily)
- 30-day bounded retention
- Storelog integration required

No overwrites. No deletes.

### Events

```go
// Phase 32 events
Phase32PressureDecisionComputed       EventType = "phase32.pressure_decision.computed"
Phase32PressureDecisionPersisted      EventType = "phase32.pressure_decision.persisted"
Phase32InterruptCandidateRateLimited  EventType = "phase32.interrupt_candidate.rate_limited"
```

### Constraints (Non-Negotiable)

1. **Classification Only**
   - No notifications
   - No execution
   - No UI action buttons
   - No background jobs

2. **No LLM Authority**
   - No shadow model calls
   - No probabilistic reasoning
   - Deterministic rules only

3. **Deterministic**
   - Same inputs + same clock → same output
   - Canonical strings everywhere

4. **Privacy-Preserving**
   - NO merchant names
   - NO people names
   - NO timestamps
   - NO raw urgency text
   - Buckets only

5. **Bounded**
   - Max 2 INTERRUPT_CANDIDATEs per period (day)
   - Excess must downgrade to SURFACE

6. **Silence-First**
   - HOLD is the default outcome
   - INTERRUPT_CANDIDATE must be rare

7. **No Goroutines**
   - All operations synchronous
   - No background processing

8. **Clock Injection**
   - No `time.Now()` calls
   - All timestamps passed explicitly

9. **stdlib Only**
   - No external dependencies in decision logic

## Why This Design

### Why Classification ≠ Interruption

Classification decides IF something CAN interrupt. A separate phase (future) will decide
IF it SHOULD interrupt. This separation allows:
- Rate limiting at the gate
- Trust protection at the gate
- Future UI without changing the gate

### Why Commerce is Excluded from Interrupts

Commerce pressure (Uber, Deliveroo, Amazon) represents habitual patterns, not urgent signals.
Allowing commerce to interrupt would:
- Create notification fatigue
- Prioritize spending over wellbeing
- Undermine the "Nothing Needs You" philosophy

Commerce can only contribute to decisions when combined with human relationships.

### Why LLMs are Forbidden Here

The decision gate must be:
- Auditable: Every decision can be explained by a rule
- Reproducible: Same inputs produce same outputs
- Fast: No network calls, no inference time
- Trustworthy: No hallucinated urgency

LLMs may be used elsewhere (shadow mode) but never at this decision boundary.

### Why 2 Interrupt Candidates Per Day

Research shows notification fatigue begins at ~3 interruptions per hour. By limiting
to 2 per day, we ensure:
- Each interrupt candidate is valuable
- Users don't learn to ignore the system
- The system earns trust through restraint

## Consequences

### Positive

- Clear decision boundary with auditable rules
- Rate limiting prevents notification fatigue
- Trust protection during fragile periods
- Commerce isolation prevents spending-driven interrupts
- Deterministic and reproducible

### Negative

- May miss genuinely urgent commerce signals (e.g., fraud alerts)
- Rule-based system may not adapt to edge cases
- Additional complexity in the decision pipeline

### Mitigations

- Commerce fraud alerts would come through financial institution (CircleType=institution)
- Edge cases can be addressed in future phases with human-in-the-loop rules
- Decision records enable analysis and rule refinement

## References

- Canon v1 (Meaning)
- Technical Split v8 (Boundaries)
- ADR-0067: Phase 31.4 External Pressure Circles
- Phase 20: Trust Baseline (for TrustBaselineStatus)
