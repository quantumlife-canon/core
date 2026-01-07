# ADR-0059: Phase 28 — Trust Kept: First Real Act, Then Silence

## Status

Accepted

## Context

QuantumLife has built a comprehensive trust foundation through Phases 1-27:
- Read-only connections to Gmail and Calendar
- Shadow mode for LLM observation without execution
- Multi-party approval for sensitive actions
- Reality verification requiring explicit acknowledgment
- Single whisper rule for minimal intrusion

The next step is to prove that the system can be trusted with a real action - and then stop.

The core question is: **Can I trust this system to act on my behalf and not abuse that trust?**

## Decision

Phase 28 implements the first and only trust-confirming real action:

> "I trusted it, and it didn't break that trust."

After execution: **silence forever**. No growth mechanics, engagement loops, or escalation paths.

### What This Phase Does

1. **Single Action Type**: Only `calendar_respond` (responding to calendar invitations)
   - Lowest risk (can be undone)
   - Most reversible (decline can become accept and vice versa)
   - Clear bounded scope

2. **Explicit Opt-In**: User must:
   - Have a trust baseline (Phase 20)
   - Have verified reality (Phase 26C)
   - Have exactly one approved calendar draft
   - Choose "Let it happen" from the preview

3. **Undoable**: 15-minute window to reverse via "Undo" button
   - Time bucketed (floored to :00, :15, :30, :45)
   - No countdown timers or urgency
   - Silent expiry

4. **Receipt**: Single card showing:
   - "Trust kept."
   - "We did what you allowed — and stopped."
   - Hash for verification

5. **Silence**: After receipt:
   - No re-invitation
   - No push notifications
   - No reminders
   - No engagement loops
   - System is comfortable doing nothing forever

### What This Phase Does NOT Do

- Add more action types
- Enable automatic execution
- Create growth mechanics
- Introduce engagement loops
- Add notifications or reminders
- Encourage more actions

## Domain Model

### TrustActionKind

```go
type TrustActionKind string

const (
    ActionKindCalendarRespond TrustActionKind = "calendar_respond"
)
```

Only one kind. By design.

### TrustActionState

```go
type TrustActionState string

const (
    StateEligible TrustActionState = "eligible"
    StateExecuted TrustActionState = "executed"
    StateUndone   TrustActionState = "undone"
    StateExpired  TrustActionState = "expired"
)
```

### UndoBucket

```go
type UndoBucket struct {
    BucketStartRFC3339    string // Floored to 15-minute boundary
    BucketDurationMinutes int    // Always 15
}
```

### TrustActionReceipt

```go
type TrustActionReceipt struct {
    ReceiptID    string           // Deterministic hash
    ActionKind   TrustActionKind
    State        TrustActionState
    UndoBucket   UndoBucket
    Period       string           // "2025-01-15" format
    CircleID     string
    StatusHash   string           // 32 hex chars
    DraftIDHash  string           // Hash of draft ID, NOT raw
    EnvelopeHash string           // Hash of envelope ID, NOT raw
}
```

Hash-only. Never raw identifiers.

## Engine

The engine orchestrates eligibility, execution, and undo:

```go
type Engine struct {
    clock            func() time.Time    // Clock injection
    calendarExecutor *calexec.Executor   // Phase 5 boundary
    draftStore       draft.Store
    trustStore       *persist.TrustStore
    realityAckStore  *persist.RealityAckStore
    trustActionStore *persist.TrustActionStore
    realityEngine    *reality.Engine
}
```

### Eligibility Check

Prerequisites for showing the invitation:

1. Trust baseline exists (Phase 20 `GetRecentMeaningfulSummary() != nil`)
2. Reality verified (Phase 26C `IsAcked(period, statusHash) == true`)
3. Exactly one approved calendar draft exists
4. No prior execution this period (`HasExecutedThisPeriod() == false`)

### Execution

Via Phase 5 calendar boundary:

```go
func (e *Engine) Execute(ctx context.Context, circleID, draftID string) *ExecuteResult
```

1. Re-verify eligibility
2. Get draft from store
3. Execute via `calendarExecutor.ExecuteFromDraft()` (Phase 5)
4. Create receipt with hashes
5. Store receipt (single-shot enforcement)
6. Return receipt

### Undo

Within 15-minute window:

```go
func (e *Engine) Undo(ctx context.Context, receiptID string) *UndoResult
```

1. Get receipt
2. Verify state is `executed`
3. Verify undo window not expired
4. Build reversal draft
5. Execute reversal via Phase 5
6. Update state to `undone`

## Persistence

### TrustActionStore

```go
type TrustActionStore struct {
    clock            func() time.Time
    receipts         map[string]*trustaction.TrustActionReceipt
    receiptsByCircle map[string][]string
    receiptsByPeriod map[string]string  // "circleID:period" -> receiptID
    maxPeriods       int                // 30 days
}
```

Constraints:
- One receipt per circle per period (single-shot enforcement)
- Hash-only storage
- Bounded retention (30 days)
- Append-only with state transitions
- Storelog integration for replay

## Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/trust/action` | GET | Preview page |
| `/trust/action/execute` | POST | Execute action |
| `/trust/action/undo` | POST | Undo action |
| `/trust/action/receipt` | GET | View receipt |
| `/trust/action/dismiss` | POST | Keep holding |

## Whisper Integration

Priority order (single whisper rule):
1. Surface cue (highest)
2. Proof cue
3. First-minutes cue
4. Reality cue
5. Shadow receipt cue
6. **Trust action cue** (lowest)

Cue text: "One thing could happen — if you let it."
Link text: "preview"

Disappears after:
- Execution
- Dismissal ("Keep holding")
- Expiry (end of period)

## Events

```go
Phase28TrustActionEligible      // Eligibility computed
Phase28TrustActionPreviewViewed // Preview page viewed
Phase28TrustActionExecuted      // Action executed
Phase28TrustActionUndone        // Action undone
Phase28TrustActionExpired       // Undo window expired
Phase28TrustActionReceiptViewed // Receipt viewed
Phase28TrustActionReceiptDismissed // Receipt dismissed
Phase28TrustActionDismissed     // Kept holding
```

All events contain hashes only, never identifiers.

## Guardrails

40+ checks enforcing:
1. Only `calendar_respond` allowed
2. Single execution per period
3. 15-minute undo window
4. No new execution paths (delegates to Phase 5)
5. No goroutines
6. No `time.Now()` (clock injection)
7. Hash-only storage
8. Silence after completion
9. No push/remind/encourage patterns

## Consequences

### Positive

- Proves trust can be earned through action
- Provides concrete evidence for "it didn't break trust"
- Establishes pattern for future action types (if ever needed)
- Demonstrates silence as the success state

### Negative

- Only one action type (by design)
- Requires significant prerequisites (by design)
- No automation (by design)

### Neutral

- Future phases could add more action types (but probably shouldn't)
- Pattern is established for trust-preserving execution

## Related ADRs

- ADR-0001: Canon v1 (foundation)
- ADR-0020: Phase 5 Calendar Execution Boundary
- ADR-0044: Phase 20 Trust Baseline
- ADR-0054: Phase 26C Reality Verification
- ADR-0058: Phase 27 Shadow Receipt

## Final Constraint

After Phase 28 completes, the system must be comfortable doing nothing forever.

Any code path that pushes, reminds, encourages, or re-invites violates this phase.

The user should be able to say truthfully:

> "I trusted it, and it didn't break that trust."

And the system should not respond.
