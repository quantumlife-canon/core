# ADR-0070: Phase 34 — Permitted Interrupt Preview (Web-only, No Notifications)

## Status
Accepted

## Date
2026-01-07

## Context

Phase 32 introduced the Pressure Decision Gate, classifying pressure into HOLD/SURFACE/INTERRUPT_CANDIDATE. Phase 33 added the Interrupt Permission Contract, determining whether candidates are permitted based on user policy.

We now need the first "delivery" primitive — a way to surface permitted interrupts to the user. However, we must preserve the canon's core principles:
- No push notifications, no email, no SMS, no OS notifications
- No background work, no goroutines
- No raw identifiers (names, merchants, amounts, timestamps)
- No execution, no auto-actions
- Deterministic: same inputs + same clock => same hashes, same UI text

The purpose is to make "urgent reality" possible while preserving Calm.

## Decision

Implement Phase 34: Permitted Interrupt Preview as a web-only, user-initiated preview mechanism.

### Why Web-only Preview is the First "Delivery"

1. **No external dependencies** — Web preview requires no push notification services, no APNs certificates, no SMS gateways
2. **User-initiated** — The user must explicitly click to see the preview; no interruption of attention
3. **Respects boundaries** — A subtle cue on /today that the user can ignore
4. **Preserves trust** — No phone buzzes, no inbox pollution, no notification fatigue
5. **Proves the pattern** — Validates the permission contract before adding real delivery channels

### Why No Notifications Yet

1. **Trust must be earned** — The system must prove it respects boundaries before interrupting
2. **Calm comes first** — A notification is an interruption; a web cue is an invitation
3. **Infrastructure independence** — No cloud services, no certificates, no third-party dependencies
4. **Reversibility** — A web cue can be dismissed; a notification cannot be unsent

### Why Abstract Buckets Only

1. **Privacy by design** — No raw content means no data exposure
2. **Determinism** — Buckets are enumerable, hashable, verifiable
3. **Calm language** — "A few things" vs "3 urgent messages from John"
4. **Trust boundary** — User chooses when to see details (in a future phase)

## Implementation

### UX Flow

1. **On /today**: If a permitted interrupt candidate exists and is not dismissed:
   - Show ONE subtle cue: "If you want to look now — there's something time-sensitive."
   - Link to /interrupts/preview

2. **On /interrupts/preview**: Single calm card showing:
   - Title: "Available, if you want it."
   - Subtitle: "This is time-sensitive, but still your choice."
   - Abstract fields only: circle type, horizon, magnitude, reason bucket
   - Two actions: "Hold quietly" or "Show me later" (both dismiss for period)

3. **On /proof/interrupts/preview**: Proof page showing:
   - Whether any preview was available (yes/no)
   - Whether user dismissed/held (yes/no)
   - Status hash for verification

### Candidate Selection

Deterministic selection of ONE candidate from permitted set:
1. Filter interrupt candidates through Phase 33 permission decisions
2. Sort by SHA256(CanonicalString(candidate) + periodKey)
3. Select the lowest hash
4. Display only abstract buckets, never raw identifiers

### Data Model

```go
// PreviewCandidate contains only abstract fields
type PreviewCandidate struct {
    CandidateHash string           // SHA256 hash
    CircleType    CircleTypeBucket // human, institution (never commerce)
    Horizon       HorizonBucket    // now, soon, later
    Magnitude     MagnitudeBucket  // nothing, a_few, several
    ReasonBucket  ReasonBucket     // why permitted
    Allowance     AllowanceBucket  // policy that permitted it
}

// PreviewAckKind tracks user acknowledgment
type PreviewAckKind string
const (
    AckViewed    PreviewAckKind = "viewed"
    AckDismissed PreviewAckKind = "dismissed"
    AckHeld      PreviewAckKind = "held"
)
```

### Storage

- Hash-only, append-only ack store
- Records: preview_viewed, preview_dismissed, preview_held
- Keyed by (circle_id_hash, period_key, candidate_hash)
- 30-day bounded retention
- Storelog integration for replay

## How It Supports "Urgent Reality" Without Breaking Trust

1. **Urgent**: Time-sensitive matters CAN surface if permitted
2. **Reality**: Real pressure is acknowledged, not hidden
3. **Without Breaking Trust**:
   - User controls the policy (Phase 33)
   - User chooses when to look (click to preview)
   - User can dismiss (respected for period)
   - No forced attention interruption
   - No notification anxiety

## Invariants

### CRITICAL — Web-only
- NO push notifications
- NO email
- NO SMS
- NO OS notifications
- NO background processing

### Standard Canon Invariants
- No goroutines in internal/ or pkg/
- No time.Now() — clock injection required
- stdlib-only
- Hash-only storage (no raw identifiers)
- No identifiers in UI
- Deterministic output
- Single-whisper rule respected

## Future Work

Phase 35+ may implement actual delivery channels (iOS push, etc.) but ONLY after:
1. This web-only preview proves the pattern works
2. User explicitly opts into the specific channel
3. All permission contracts are honored

## Consequences

### Positive
- First "delivery" without breaking calm
- User retains full control
- Proves permission contract before real notifications
- No external dependencies

### Negative
- User must actively check /today to see cue
- Not suitable for truly urgent matters (but that's by design)
- May feel underwhelming compared to traditional notifications

### Risks
- Users may expect push notifications (mitigated by clear UI copy)
- Cue may be overlooked (acceptable — calm over urgent)

## References

- ADR-0068: Phase 32 Pressure Decision Gate
- ADR-0069: Phase 33 Interrupt Permission Contract
- Canon invariants documentation
