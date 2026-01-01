# ADR-0022: Phase 5 — Calendar Execution Boundary

## Status

Accepted

## Context

Phase 4 established the draft-generation pattern where QuantumLife proposes actions but never executes without explicit human approval. With drafts working correctly, we now need to implement the **first real external write** in the system.

Calendar response actions (accept/decline/tentative) are chosen as the first execution target because:
1. They are low-risk, reversible operations
2. They have clear success/failure semantics
3. They demonstrate the complete execution pattern
4. They provide immediate user value

This is a critical boundary crossing that establishes patterns for all future external writes.

## Decision

### Core Invariants

**CRITICAL: These invariants are NON-NEGOTIABLE:**

1. **Execution ONLY from approved drafts** — No execution without a prior draft that was explicitly approved
2. **No auto-retries** — If execution fails, it fails. User must explicitly retry
3. **No background execution** — All execution is synchronous, in the foreground
4. **Idempotency** — Same envelope executed twice returns same result
5. **Policy snapshot binding** — Execution fails if policy changed since approval
6. **View snapshot freshness** — Execution fails if calendar state is stale
7. **Full audit trail** — Every execution attempt is logged with trace

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Execution Flow                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Approved Draft                                               │
│       │                                                          │
│       ▼                                                          │
│  2. Create Envelope                                              │
│       │   - Bind PolicySnapshotHash                              │
│       │   - Bind ViewSnapshotHash                                │
│       │   - Compute IdempotencyKey                               │
│       │                                                          │
│       ▼                                                          │
│  3. Verify Policy                                                │
│       │   - Check PolicySnapshotHash matches current             │
│       │   - BLOCK if mismatch                                    │
│       │                                                          │
│       ▼                                                          │
│  4. Verify View Freshness                                        │
│       │   - Check ViewSnapshotHash is fresh                      │
│       │   - Check ViewSnapshotHash matches current               │
│       │   - BLOCK if stale or changed                            │
│       │                                                          │
│       ▼                                                          │
│  5. Execute (via Writer)                                         │
│       │   - Call provider API                                    │
│       │   - NO retries on failure                                │
│       │                                                          │
│       ▼                                                          │
│  6. Record Result                                                │
│       - Store ExecutionResult in envelope                        │
│       - Emit audit events                                        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Components

#### 1. Calendar Write Connector (`internal/connectors/calendar/write/`)

The Writer interface defines the contract for calendar write operations:

```go
type Writer interface {
    // RespondToEvent responds to a calendar event invitation.
    // CRITICAL: This performs a REAL external write.
    // CRITICAL: Must be idempotent - same IdempotencyKey returns same result.
    // CRITICAL: No auto-retries on failure.
    RespondToEvent(ctx context.Context, input RespondInput) (RespondReceipt, error)

    ProviderID() string
    IsSandbox() bool
}
```

Providers:
- `providers/mock/` — Mock implementation for testing
- `providers/google/` — Real Google Calendar API (HTTP stdlib only)

#### 2. Execution Envelope (`internal/calendar/execution/envelope.go`)

The Envelope binds together all components needed for safe execution:

```go
type Envelope struct {
    EnvelopeID         string
    DraftID            draft.DraftID
    CircleID           identity.EntityID

    // Calendar action
    Provider           string
    CalendarID         string
    EventID            string
    Response           draft.CalendarResponse

    // Snapshot bindings
    PolicySnapshotHash string  // REQUIRED
    ViewSnapshotHash   string  // REQUIRED
    ViewSnapshotAt     time.Time

    // Idempotency
    IdempotencyKey     string  // REQUIRED
    TraceID            string  // REQUIRED

    Status             EnvelopeStatus
    ExecutionResult    *ExecutionResult
}
```

#### 3. Policy Snapshot (`internal/calendar/execution/policy_snapshot.go`)

Captures policy state at envelope creation time:

```go
type PolicySnapshot struct {
    PolicyHash           string
    CalendarWriteEnabled bool
    AllowedCalendarIDs   []string
    AllowedProviders     []string
    MaxStalenessMinutes  int
    DryRunMode           bool
}
```

#### 4. View Snapshot (`internal/calendar/execution/view_snapshot.go`)

Captures calendar state for freshness checking:

```go
type ViewSnapshot struct {
    ViewHash               string
    EventETag              string
    AttendeeResponseStatus string
    CapturedAt             time.Time
}
```

Default max staleness: 15 minutes.

#### 5. Executor (`internal/calendar/execution/executor.go`)

The Executor is the ONLY path to external calendar writes:

```go
func (e *Executor) Execute(ctx context.Context, envelope *Envelope) ExecuteResult {
    // 1. Validate envelope
    // 2. Check idempotency (return prior result if exists)
    // 3. Verify policy snapshot
    // 4. Verify view snapshot freshness
    // 5. Get writer and execute
    // 6. Store result and emit events
}
```

### Blocking Conditions

Execution is BLOCKED (not failed) when:

1. **Policy mismatch** — PolicySnapshotHash differs from current policy
2. **View stale** — ViewSnapshotAt is older than MaxStaleness
3. **View changed** — ViewSnapshotHash differs from current view
4. **Missing hashes** — PolicySnapshotHash or ViewSnapshotHash is empty
5. **No writer** — No writer registered for the provider

### Events

Phase 5 events (defined in `pkg/events/events.go`):

```go
// Envelope lifecycle
Phase5CalendarEnvelopeCreated
Phase5CalendarEnvelopeValidated

// Policy snapshot
Phase5CalendarPolicySnapshotTaken
Phase5CalendarPolicySnapshotVerified
Phase5CalendarPolicySnapshotMismatch

// View snapshot
Phase5CalendarViewSnapshotTaken
Phase5CalendarViewSnapshotFresh
Phase5CalendarViewSnapshotStale
Phase5CalendarViewSnapshotChanged

// Execution lifecycle
Phase5CalendarExecutionStarted
Phase5CalendarExecutionSuccess
Phase5CalendarExecutionFailed
Phase5CalendarExecutionBlocked
Phase5CalendarExecutionIdempotent
```

### Idempotency

Idempotency is enforced at two levels:

1. **Envelope level** — IdempotencyKey is computed from envelope contents
2. **Provider level** — IdempotencyKey is passed to the provider API

Same IdempotencyKey always returns same result, even if called multiple times.

### Propose New Time

When `ProposeNewTime=true`:
- Response is set to tentative
- A note is added to the event description
- The event times are NOT modified
- This is safe and reversible

## Consequences

### Positive

1. **Boringly safe** — Every execution is audited, verified, and idempotent
2. **No surprises** — Policy/view changes block execution rather than proceeding
3. **Full control** — Users must approve every draft before execution
4. **Reversible** — Calendar responses can be changed later

### Negative

1. **More friction** — Users must approve each action individually
2. **Stale views** — Fresh views may require re-fetching before execution
3. **Complexity** — Multiple verification steps add latency

### Neutral

1. **Patterns established** — This sets the template for all future writes
2. **Testing strategy** — Mock provider enables thorough testing

## Alternatives Considered

### 1. Direct execution without envelopes

Rejected: No audit trail, no idempotency, no policy binding.

### 2. Auto-retry on transient failures

Rejected: Violates core guardrail. Retries must be explicit.

### 3. Background execution queue

Rejected: Violates core guardrail. All execution is synchronous.

### 4. Blanket approvals (e.g., "approve all accepts")

Rejected: Violates per-action approval requirement.

## References

- Phase 4: ADR-0021-phase4-drafts-only-assistance.md
- No Background Execution: ADR-0010-no-background-execution-guardrail.md
- No Auto-Retry: ADR-0011-no-auto-retry-and-single-trace-finalization.md
- View Freshness: ADR-0015-v9.13-view-freshness-binding.md
