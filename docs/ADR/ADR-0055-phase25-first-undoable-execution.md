# ADR-0055: Phase 25 - First Undoable Execution (Opt-In, Single-Shot)

## Status

Accepted

## Context

Previous phases established execution boundaries for calendar responses (Phase 5), email sends (Phase 7), and finance (Phase 17 - sandbox only). All of these execute external actions based on approved drafts. However, none of them provide a meaningful undo capability.

Real-world AI assistants need to allow users to reverse mistakes. But not all actions are undoable:
- **Email send**: Not truly undoable - once sent, the recipient has the email
- **Finance**: Not undoable - payments cannot be reversed unilaterally
- **Calendar RSVP**: Undoable - can be reversed by sending the opposite response

Phase 25 introduces the first truly undoable execution capability, limited to calendar responses.

## Decision

### 1. Only Calendar Respond is Undoable

Calendar RSVP responses are the only action kind in Phase 25:
- `ActionKindCalendarRespond` is the only supported `UndoableActionKind`
- Email and finance action kinds are explicitly NOT defined
- The guardrails enforce this constraint

### 2. Single-Shot Per Period

Only one undoable execution per circle per day:
- Period key uses daily buckets (YYYY-MM-DD)
- `HasExecutedThisPeriod` check prevents multiple executions
- Prevents runaway execution and gives users time to review

### 3. Bounded Undo Window

Undo availability uses 15-minute buckets:
- `UndoWindow` stores `BucketStartRFC3339` and `BucketDurationMinutes`
- Execution timestamp is floored to nearest 15-minute bucket
- Undo deadline is one bucket after execution (15 minutes)
- Bucketed time provides privacy (no exact timestamps)

### 4. Undo is First-Class Flow

Undo is not "best effort" - it's a fully supported operation:
- `UndoRecord` captures before/after status for reversal
- `UndoAck` records state transitions
- Dedicated UI pages for undo confirmation
- Engine has explicit `Undo()` method

### 5. Reuse Existing Calendar Execution Boundary

Phase 25 does NOT create a new execution path:
- Uses the existing `calendar/execution.Executor` from Phase 5
- `RunOnce()` calls `ExecuteFromDraft()` on the existing executor
- `Undo()` creates a reversal draft and executes it the same way

### 6. State Machine

UndoRecord follows this state flow:
```
pending → executed → undo_available → undone
                                   → expired
```

- `pending`: Execution not yet completed
- `executed`: Execution completed, undo not yet available (unused in Phase 25)
- `undo_available`: Undo window is open
- `undone`: Undo was performed
- `expired`: Undo window expired without undo

### 7. Hash-Only Storage

No identifiers are stored in UndoRecord:
- `ID` is a hash of the canonical record
- `DraftID` and `EnvelopeID` are hashes
- `BeforeStatus` and `AfterStatus` are enums
- Bucketed timestamps only

## Consequences

### Positive

1. **Real Undo**: Users can reverse calendar responses within the window
2. **Single-Shot Safety**: Prevents runaway execution
3. **Privacy**: Bucketed time prevents exact timestamp inference
4. **Auditability**: Full state machine with ack tracking
5. **Reuse**: No new execution boundary code

### Negative

1. **Limited Scope**: Only calendar responses - not email or finance
2. **Short Window**: 15-minute undo window may be too short for some users
3. **Daily Limit**: Single-shot per day may be restrictive

### Neutral

1. **Determinism**: Same inputs + clock = same outputs
2. **No Background**: All operations are synchronous

## Implementation

### Domain Model

```go
// pkg/domain/undoableexec/types.go
type UndoableActionKind string
const ActionKindCalendarRespond UndoableActionKind = "calendar_respond"

type UndoState string
const (
    StatePending       UndoState = "pending"
    StateUndoAvailable UndoState = "undo_available"
    StateUndone        UndoState = "undone"
    StateExpired       UndoState = "expired"
)

type UndoWindow struct {
    BucketStartRFC3339    string
    BucketDurationMinutes int // Always 15
}

type UndoRecord struct {
    ID                       string
    PeriodKey                string
    CircleID                 string
    ActionKind               UndoableActionKind
    DraftID                  string
    EnvelopeID               string
    BeforeStatus             ResponseStatus
    AfterStatus              ResponseStatus
    UndoAvailableUntilBucket UndoWindow
    State                    UndoState
    ExecutedAtBucket         UndoWindow
}
```

### Engine

```go
// internal/undoableexec/engine.go
type Engine struct {
    clock            func() time.Time
    calendarExecutor *calexec.Executor
    draftStore       draft.Store
    undoStore        *persist.UndoableExecStore
}

func (e *Engine) EligibleAction(ctx context.Context, circleID identity.EntityID) *ActionEligibility
func (e *Engine) RunOnce(ctx context.Context, circleID identity.EntityID, draftID string) *RunOnceResult
func (e *Engine) Undo(ctx context.Context, undoRecordID string) *UndoResult
```

### Persistence

```go
// internal/persist/undoable_exec_store.go
type UndoableExecStore struct {
    records  map[string]*UndoRecord
    acks     []*UndoAck
    byCircle map[identity.EntityID][]*UndoRecord
    byPeriod map[string][]*UndoRecord
    clock    func() time.Time
}

func (s *UndoableExecStore) AppendRecord(record *UndoRecord) error
func (s *UndoableExecStore) AppendAck(ack *UndoAck) error
func (s *UndoableExecStore) HasExecutedThisPeriod(circleID identity.EntityID, periodKey string) bool
func (s *UndoableExecStore) GetLatestUndoable(circleID identity.EntityID) *UndoRecord
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/action/undoable` | GET | Show undoable action page |
| `/action/undoable/run` | POST | Execute the undoable action |
| `/action/undoable/done` | GET | Show done confirmation |
| `/action/undoable/undo` | GET | Show undo confirmation page |
| `/action/undoable/undo/run` | POST | Execute undo |
| `/action/undoable/dismiss` | POST | Dismiss without action |

### Events

```go
Phase25UndoableViewed   = "phase25.undoable.viewed"
Phase25EligibleComputed = "phase25.eligible.computed"
Phase25RunRequested     = "phase25.run.requested"
Phase25RunExecuted      = "phase25.run.executed"
Phase25RecordPersisted  = "phase25.record.persisted"
Phase25UndoViewed       = "phase25.undo.viewed"
Phase25UndoRequested    = "phase25.undo.requested"
Phase25UndoExecuted     = "phase25.undo.executed"
Phase25AckPersisted     = "phase25.ack.persisted"
Phase25Dismissed        = "phase25.dismissed"
```

## Guardrails

44 guardrails enforce:
1. Only `calendar_respond` action kind defined
2. No email or finance action kinds
3. 15-minute undo window buckets
4. Clock injection (no `time.Now()`)
5. No goroutines
6. Uses existing calendar execution boundary
7. Single-shot enforcement via `HasExecutedThisPeriod`

## Related ADRs

- ADR-0005: Phase 5 - Calendar Execution Boundary
- ADR-0007: Phase 7 - Email Execution Boundary
- ADR-0017: Phase 17 - Finance Execution Boundary
- ADR-0021: Phase 4 - Drafts-Only Assistance

## References

- `pkg/domain/undoableexec/types.go`
- `internal/undoableexec/engine.go`
- `internal/persist/undoable_exec_store.go`
- `scripts/guardrails/undoable_execution_enforced.sh`
- `internal/demo_phase25_first_undoable_execution/demo_test.go`
