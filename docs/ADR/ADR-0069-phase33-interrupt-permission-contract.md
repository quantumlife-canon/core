# ADR-0069: Phase 33 — Interrupt Permission Contract (Policy + Proof + Rate-limits)

## Status
Accepted

## Date
2026-01-07

## Context

Phase 32 introduced the Pressure Decision Gate, which classifies external pressure into:
- HOLD (default): Pressure acknowledged but not surfaced
- SURFACE: Pressure may appear in calm mirror views
- INTERRUPT_CANDIDATE: Pressure may compete for interruption (rare, max 2/day)

However, being an INTERRUPT_CANDIDATE does not mean the system will interrupt the user. We need a separate contract layer that answers: "Even if something is an INTERRUPT_CANDIDATE, is it permitted to interrupt (in principle)?"

This separation is critical because:
1. Classification (Phase 32) is about what pressure exists
2. Permission (Phase 33) is about what the user consents to
3. Delivery (future Phase 34+) is about how/when to actually interrupt

## Decision

Implement Phase 33: Interrupt Permission Contract as a policy + proof layer with NO delivery capability.

### Key Design Choices

1. **Separate "candidate" from "permission" from "delivery"**
   - Phase 32: Classifies pressure → produces INTERRUPT_CANDIDATE
   - Phase 33: Evaluates permission → produces ALLOWED/DENIED decision
   - Phase 34+: Implements actual delivery channels (future)

2. **Default stance is NO interrupts**
   - InterruptAllowance defaults to `allow_none`
   - User must explicitly enable any interrupts
   - Commerce never interrupts regardless of policy

3. **Policy is per-circle, per-period**
   - Each pressure circle can have its own policy
   - Policies are daily-bucketed for bounded retention
   - Hash-only storage preserves privacy

4. **Rate limiting is deterministic**
   - MaxPerDay clamped to 0..2
   - When candidates exceed cap, selection is by CandidateHash ascending
   - Same inputs always produce same allowed/denied set

5. **Proof page shows abstract buckets only**
   - "Permitted today: nothing / a_few / several"
   - No raw counts, no identifiers, no timestamps
   - User can dismiss proof cue for period

## Rationale

Real-world pressure exists, but trust must not be broken. By separating permission policy from delivery:

1. Users can define what COULD interrupt them without triggering any notifications
2. The system proves it respects boundaries before any delivery is implemented
3. Future delivery channels (iOS push, etc.) will require explicit Phase 34+ work
4. The contract creates accountability: policy + proof exist before action

## Implementation

### Domain Model

```go
// InterruptAllowance defines what types of interrupts are permitted
type InterruptAllowance string
const (
    AllowNone             InterruptAllowance = "allow_none"           // Default
    AllowHumansNow        InterruptAllowance = "allow_humans_now"     // Human + NOW only
    AllowInstitutionsSoon InterruptAllowance = "allow_institutions_soon" // Institution + SOON/NOW
    AllowTwoPerDay        InterruptAllowance = "allow_two_per_day"    // Cap only, any eligible
)

// InterruptPolicy defines user's interrupt permission settings
type InterruptPolicy struct {
    CircleIDHash  string             // Hash of circle ID
    PeriodKey     string             // Daily bucket (YYYY-MM-DD)
    Allowance     InterruptAllowance // What's permitted
    MaxPerDay     int                // Clamped 0..2
    CreatedBucket string             // Time bucket (no raw timestamp)
    PolicyHash    string             // Deterministic hash
}

// InterruptPermissionDecision is the result of permission evaluation
type InterruptPermissionDecision struct {
    CandidateHash     string       // Hash of the candidate
    Allowed           bool         // Permission granted?
    ReasonBucket      ReasonBucket // Why allowed/denied
    DeterministicHash string       // Verifiable hash
}
```

### Permission Rules

1. If `Allowance == allow_none` → deny all (reason_policy_denies)
2. If `CircleType == commerce` → deny always (reason_category_blocked)
3. If `allow_humans_now` → allow only (CircleType==human AND Horizon==now)
4. If `allow_institutions_soon` → allow only (CircleType==institution AND Horizon in {soon, now})
5. Apply MaxPerDay cap deterministically by CandidateHash ascending order

### Web Routes

- `GET /settings/interrupts` — View/edit policy (calm, minimal UI)
- `POST /settings/interrupts/save` — Save policy (hash-only persistence)
- `GET /proof/interrupts` — Proof page showing permitted/denied magnitudes
- `POST /proof/interrupts/dismiss` — Dismiss proof cue for period

### Whisper Cue Integration

Phase 33 adds a whisper cue to /today with LOWEST priority (below all existing cues):
- Text: "If you ever need it — interruptions are still being held."
- Link: /proof/interrupts
- Shown only if: INTERRUPT_CANDIDATE decisions exist AND cue not dismissed

## Invariants

### CRITICAL — NO DELIVERY
- NO OS notifications
- NO web push
- NO email send
- NO SMS
- NO webhooks
- NO "notify" side effects whatsoever

This phase is policy evaluation and proof ONLY.

### Standard Canon Invariants
- No goroutines in internal/ or pkg/
- No time.Now() — clock injection required
- stdlib-only in internal/ and pkg/
- Hash-only storage (no raw identifiers)
- No identifiers in UI
- Deterministic: same inputs + clock bucket → same outputs + same hashes
- Single-whisper rule preserved

## Future Work

Phase 34 will implement the first delivery channel (likely iOS push notification) but ONLY after this contract layer proves the system respects user boundaries.

The delivery implementation will:
1. Require Phase 33 policy to allow the interrupt
2. Require explicit user opt-in for the specific channel
3. Respect all rate limits and category blocks
4. Remain auditable via proof pages

## Consequences

### Positive
- Clear separation of concerns: classify → permit → deliver
- User has full control over what COULD interrupt them
- System proves boundaries before any delivery
- Deterministic, auditable permission decisions

### Negative
- Additional complexity in the interrupt pipeline
- Users may wonder why interrupts don't work yet (they're not implemented)
- Must clearly communicate that "allowing" doesn't mean "will receive"

### Risks
- Users might enable interrupts expecting notifications (mitigated by clear UI copy)
- Future delivery phase must honor all Phase 33 contracts (mitigated by deterministic hashing)

## References

- ADR-0068: Phase 32 Pressure Decision Gate
- ADR-0063: Phase 31.4 External Pressure Circles
- Canon invariants documentation
