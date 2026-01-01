# ADR-0030: Phase 14 - Circle Policies + Preference Learning (Deterministic)

## Status
Accepted

## Context

QuantumLife's Quiet Loop surfaces interruptions based on computed regret scores.
Users naturally develop preferences: "show me fewer newsletters" or "always
surface payment reminders from Alice". The system needs to learn these preferences
WITHOUT:

1. Losing determinism (same inputs must produce same outputs)
2. Using ML/AI (no neural networks, no stochastic algorithms)
3. Violating Canon constraints (no execution, no auto-retry)
4. Making decisions opaque (users must understand "why am I seeing this?")

## Decision

We implement **rule-based preference learning** with:

### A) Policy Domain Model (`pkg/domain/policy/`)

Per-circle policies with threshold adjustments:

```go
type CirclePolicy struct {
    CircleID         string
    RegretThreshold  int  // 0-100, floor for queue
    NotifyThreshold  int  // 0-100, floor for notify level
    UrgentThreshold  int  // 0-100, floor for urgent level
    DailyNotifyQuota int  // max notify interruptions per day
    DailyQueuedQuota int  // max queued interruptions per day
    Hours            *HoursPolicy  // optional time-of-day restrictions
}
```

Per-trigger policies with bias adjustments:

```go
type TriggerPolicy struct {
    Trigger           string
    MinLevel          string  // enforce minimum level
    SuppressByDefault bool    // suppress unless explicitly enabled
    RegretBias        int     // -50 to +50 adjustment
}
```

### B) Suppression Domain Model (`pkg/domain/suppress/`)

Explicit suppression rules with deterministic IDs:

```go
type SuppressionRule struct {
    RuleID    string   // SHA256-based deterministic ID
    CircleID  string
    Scope     Scope    // circle, person, vendor, trigger, itemkey
    Key       string
    ExpiresAt *time.Time
    Source    Source   // manual, feedback
}
```

Scopes (from broadest to narrowest):
- `scope_circle`: suppress everything in a circle
- `scope_person`: suppress from a specific person
- `scope_vendor`: suppress from a specific vendor
- `scope_trigger`: suppress a trigger type
- `scope_itemkey`: suppress a specific dedup key

### C) Preference Learning Engine (`internal/preflearn/`)

Rule-based (NOT ML) learning from feedback:

**Unnecessary Feedback Rules:**
1. First unnecessary → increase circle threshold by 5
2. Second unnecessary for same trigger → add suppression rule
3. If PersonID present → prefer person-scoped suppression

**Helpful Feedback Rules:**
1. Helpful → decrease circle threshold by 3 (floor: 5)
2. If trigger context → increase trigger bias by 5

All decisions produce `DecisionRecord` with:
- FeedbackID (input)
- Action (threshold_increase, threshold_decrease, suppression_add, etc.)
- Reason (deterministic explanation)
- Before/After hashes (audit trail)

### D) Explainability (`pkg/domain/interrupt/explain.go`)

Every interruption can be explained:

```go
type ExplainRecord struct {
    InterruptionID string
    CircleID       string
    Trigger        string
    RegretScore    int
    Level          Level
    Reasons        []string  // stable-ordered
    Scoring        *ScoringBreakdown
    QuotaState     *QuotaState
    SuppressionHit *string
    Hash           string  // deterministic
}
```

`FormatForUI()` produces human-readable explanation:

```
Interruption: int-001
Circle: work
Trigger: obligation.due_soon
Regret Score: 75/100
Level: notify

Why this interruption:
  1. Score 75 >= notify threshold 60
  2. Due within 12 hours

Score breakdown:
  Circle base: 30
  Due date boost: +20
  Action required boost: +15
  Severity boost: +10
  Final score: 75

Quota status:
  Notify: 2/5 used
  Queued: 8/20 used
```

### E) Persistence (`internal/persist/`)

PolicyStore and SuppressStore use storelog append-only persistence:

```
Record Types:
  POLICY_SET       - full policy set snapshot
  SUPPRESSION_ADD  - add suppression rule
  SUPPRESSION_REM  - remove suppression rule
```

Replay on startup reconstructs current state.

### F) Web UI

- `/policies` - list all circle policies
- `/policies/:circleID` - view/edit single circle policy
- `/interruptions/:id/why` - explain why an interruption appeared
- Feedback controls on interruption cards

## Consequences

### Positive

1. **Deterministic**: Same feedback always produces same policy changes
2. **Auditable**: Full decision record with before/after hashes
3. **Explainable**: Users understand why they see (or don't see) items
4. **Bounded**: No runaway learning, thresholds capped at floor/ceiling
5. **Reversible**: Suppression rules can expire or be removed

### Negative

1. **Simple Rules**: May not capture complex preferences as well as ML
2. **Manual Tuning**: Default thresholds need careful calibration
3. **Version Tracking**: Policy version must increment on every change

### Neutral

1. Learning is intentionally conservative (small adjustments)
2. Suppression rules default to temporary (can be made permanent)

## Guardrails

`scripts/guardrails/policy_learning_enforced.sh` verifies:

1. No ML/AI patterns in preference learning
2. No random number generators
3. Deterministic hashing in policy and suppress domains
4. Explainability module exists and is complete
5. No time.Now() (clock injection required)
6. No goroutines in preference learning
7. Phase 14 events defined
8. PolicyStore uses storelog
9. Decision records exist

## Implementation Checklist

- [x] A) Policy Domain Model (`pkg/domain/policy/`)
- [x] B) Suppression Domain Model (`pkg/domain/suppress/`)
- [x] C) PolicyStore + SuppressStore (`internal/persist/`)
- [x] D) Preference Learning Engine (`internal/preflearn/`)
- [x] E) Interruption Explainability (`pkg/domain/interrupt/explain.go`)
- [x] F) Phase 14 Events (`pkg/events/events.go`)
- [x] G) Web UI routes
- [x] H) Demo package (`internal/demo_phase14_policy_learning/`)
- [x] I) Guardrail script
- [x] J) ADR

## References

- Phase 12: Persistence + Replay Guarantees (ADR-0028)
- Phase 13.1: Identity-Driven Routing (ADR-0029)
- Canon v1: Core Principles
