# ADR-0052: Phase 22 - Quiet Inbox Mirror (First Real Value Moment)

## Status

Accepted

## Context

QuantumLife has reached a critical milestone: real Gmail data is now flowing through the system (Phase 19.1). The challenge is delivering undeniable value to the circle while maintaining the core promise: **Nothing surfaces unless it truly needs the user.**

Previous phases established:
- Phase 19.1: Real Gmail connection with sync receipts
- Phase 18.9: Quiet baseline verification
- Phase 18.7: Mirror proof of what was read

Now we need to show circles that their data is being processed meaningfully, without:
- Showing any email content
- Creating urgency or anxiety
- Requiring any action
- Building engagement loops

This is the **first real value moment** - proving the system works without asking for anything in return.

## Decision

We implement a **Quiet Inbox Mirror** that provides:

1. **Abstract Reflection Only**
   - Magnitude buckets: `nothing` | `a_few` | `several`
   - Category buckets: `work` | `time` | `money` | `people` | `home`
   - One calm, ignorable statement

2. **No Identifiable Information**
   - No email subjects
   - No senders
   - No timestamps
   - No counts
   - No raw data of any kind

3. **Deterministic Output**
   - Same inputs always produce same output
   - Pipe-delimited canonical strings
   - SHA256 hashing for integrity

4. **No LLM Usage**
   - LLM remains shadow-only
   - All categorization is rule-based
   - All statements are predetermined

### Architecture

```
Gmail SyncReceipt → QuietMirrorInput → Engine.Compute() → QuietMirrorSummary → QuietMirrorPage
                         │                                        │
                    (abstract only)                         (single statement,
                    - magnitude bucket                        max 3 categories)
                    - category presence
```

### Domain Model

```go
// Magnitude buckets (never counts)
type MirrorMagnitude string
const (
    MagnitudeNothing MirrorMagnitude = "nothing"
    MagnitudeAFew    MirrorMagnitude = "a_few"
    MagnitudeSeveral MirrorMagnitude = "several"
)

// Abstract categories
type MirrorCategory string
const (
    CategoryWork   MirrorCategory = "work"
    CategoryTime   MirrorCategory = "time"
    CategoryMoney  MirrorCategory = "money"
    CategoryPeople MirrorCategory = "people"
    CategoryHome   MirrorCategory = "home"
)

// Summary contains ONLY abstract data
type QuietMirrorSummary struct {
    CircleID   string
    Period     string              // "2024-01-15" (bucket)
    Magnitude  MirrorMagnitude
    Categories []MirrorCategory    // max 3, sorted
    Statement  MirrorStatement     // one calm statement
    HasMirror  bool
    SourceHash string              // for replay
}
```

### UI Design

**Route:** `GET /mirror/inbox`

**Title:** "Seen, quietly."

**Content:**
- Single statement (deterministically chosen based on magnitude)
- Optional category chips (max 3)
- Footer reassurance

**Examples:**
- "Nothing here needs you today."
- "A few patterns are being kept an eye on."
- "Some things are being watched quietly."

**Whisper Cue (optional, from /today):**
- "If you were curious, we noticed something — quietly."
- Dismissable
- Never pushed

### Events

```go
Phase22QuietMirrorComputed EventType = "phase22.quiet_mirror.computed"
Phase22QuietMirrorViewed   EventType = "phase22.quiet_mirror.viewed"
Phase22QuietMirrorAbsent   EventType = "phase22.quiet_mirror.absent"
Phase22WhisperCueShown     EventType = "phase22.whisper_cue.shown"
Phase22WhisperCueDismissed EventType = "phase22.whisper_cue.dismissed"
```

All events contain **hashes only**, never content or identifiers.

## Consequences

### Benefits

1. **Trust Through Restraint**
   - The system proves it's working without demanding attention
   - No engagement loops, no urgency manufacturing

2. **Privacy Preservation**
   - No identifiable information ever displayed
   - Abstract buckets are the maximum resolution

3. **Determinism**
   - Same inputs produce same outputs
   - Replay and verification possible

4. **Foundation for Future**
   - This pattern establishes how QuantumLife shows value
   - Other mirrors (calendar, finance) can follow same approach

### Tradeoffs

1. **Limited Specificity**
   - Cannot tell circles exactly what was noticed
   - This is intentional - specificity creates anxiety

2. **Category Limitations**
   - Max 3 categories means some context is lost
   - This is intentional - less is more

3. **No Actions**
   - Circle cannot act from mirror page
   - This is intentional - actions belong elsewhere

## Implementation

### Files Created

| File | Purpose |
|------|---------|
| `pkg/domain/quietmirror/types.go` | Domain model |
| `internal/quietmirror/engine.go` | Projection engine |
| `internal/persist/quietmirror_store.go` | Persistence |
| `cmd/quantumlife-web/main.go` | Web handlers |
| `internal/demo_phase22_quiet_inbox_mirror/demo_test.go` | Demo tests |
| `scripts/guardrails/quiet_inbox_mirror_enforced.sh` | Guardrails |

### Guardrails (31 checks)

- No goroutines in pkg/domain/quietmirror/
- No time.Now() (clock injection only)
- Magnitude/Category enums exist
- Pipe-delimited canonical strings
- No Gmail fields (Subject, From, Body, etc.)
- No LLM imports
- No action buttons in template
- Phase 22 events defined
- Hash-keyed storage
- Stdlib only

### Demo Tests (13 tests)

1. No connection → no mirror
2. Nothing notable → "Nothing needs you"
3. Patterns exist → abstract mirror shown
4. Determinism: same inputs = same output
5. Privacy: no identifiers
6. No auto-surface
7. Mirror doesn't affect obligations
8. Categories capped at 3
9. Magnitude buckets only
10. Page display properties
11. Store persistence
12. Empty page when no receipt
13. Canonical string format

## Invariants

**NON-NEGOTIABLE:**

1. Silence is success
2. Abstraction over explanation
3. Circle effort must be optional
4. No engagement loops
5. Proof > persuasion
6. Determinism everywhere

## Success Criteria

This phase succeeds if the circle thinks:

> "Oh... it's already working.
> And it didn't ask me to do anything."

## References

- Phase 19.1: Real Gmail Connection
- Phase 18.9: Quiet Baseline Verification
- Phase 18.7: Mirror Proof
- Canon v1: Forbidden at Core
