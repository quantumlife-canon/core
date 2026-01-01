# ADR-0023: Phase 6 — The Quiet Loop (Web)

## Status

Accepted

## Context

Phases 1-5 established the complete pipeline from event ingestion through obligation extraction, interruption computation, draft generation, and calendar execution. All these components exist but were accessed only through demos and tests.

Users need a unified interface to:
1. See the current state of all their circles
2. Review items that need their attention (drafts, interruptions)
3. Approve or reject drafts
4. View execution history
5. Provide feedback on system behavior

The "quiet loop" concept means: when nothing needs attention, the system should say "Nothing Needs You" and get out of the way. The goal is **earned silence** — silence that the user can trust because the system has genuinely processed everything.

## Decision

### Core Invariants

**CRITICAL: These invariants are NON-NEGOTIABLE:**

1. **Synchronous loop execution** — Loop runs per request, not in background
2. **No background workers** — No goroutines polling or processing
3. **No auto-retries** — If something fails, user must explicitly retry
4. **Deterministic given same inputs + clock** — Same state produces same output
5. **Injected clock** — All time comes from `clock.Clock`, never `time.Now()`
6. **Full audit trail** — Every run, feedback, and action is logged via events

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Daily Loop Flow                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  HTTP Request (GET /, POST /approve, etc.)                       │
│       │                                                          │
│       ▼                                                          │
│  Loop Engine.Run()                                               │
│       │                                                          │
│       ├──▶ Get Circles from IdentityRepo                         │
│       │                                                          │
│       ▼                                                          │
│  For each Circle:                                                │
│       │                                                          │
│       ├──▶ 1. Extract Obligations (from EventStore)              │
│       │                                                          │
│       ├──▶ 2. Build DailyView                                    │
│       │                                                          │
│       ├──▶ 3. Compute Interruptions                              │
│       │                                                          │
│       ├──▶ 4. Generate Drafts                                    │
│       │                                                          │
│       ├──▶ 5. Get Pending Drafts                                 │
│       │                                                          │
│       └──▶ 6. Execute Approved Drafts (if requested)             │
│                                                                  │
│       ▼                                                          │
│  Compute NeedsYou Summary                                        │
│       │   - Aggregate pending drafts                             │
│       │   - Aggregate active interruptions                       │
│       │   - Compute deterministic hash                           │
│       │   - Set IsQuiet = (TotalItems == 0)                      │
│       │                                                          │
│       ▼                                                          │
│  Return RunResult                                                │
│       │                                                          │
│       ▼                                                          │
│  Render HTML (or return JSON)                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Components

#### 1. Loop Engine (`internal/loop/engine.go`)

The Engine orchestrates the complete daily loop:

```go
type Engine struct {
    Clock              clock.Clock
    IdentityRepo       identity.Repository
    EventStore         domainevents.EventStore
    ObligationEngine   *obligations.Engine
    InterruptionEngine *interruptions.Engine
    DraftEngine        *drafts.Engine
    DraftStore         draft.Store
    ReviewService      *review.Service
    CalendarExecutor   *execution.Executor
    FeedbackStore      feedback.Store
    EventEmitter       events.Emitter
}

func (e *Engine) Run(ctx context.Context, opts RunOptions) RunResult
func (e *Engine) RecordFeedback(...) (feedback.FeedbackRecord, error)
func (e *Engine) ApproveDraft(draftID, circleID, reason) error
func (e *Engine) RejectDraft(draftID, circleID, reason) error
```

#### 2. NeedsYou Summary (`internal/loop/engine.go`)

The NeedsYou summary captures what needs user attention:

```go
type NeedsYouSummary struct {
    TotalItems          int
    PendingDrafts       []draft.Draft
    ActiveInterruptions []*interrupt.Interruption
    Hash                string  // Deterministic hash of state
    IsQuiet             bool    // True when nothing needs attention
}
```

The hash is computed deterministically:
- All draft IDs and interruption IDs are collected
- IDs are sorted alphabetically
- SHA256 hash is computed from the sorted list

#### 3. Feedback Domain (`pkg/domain/feedback/`)

Feedback allows users to signal whether items were helpful:

```go
type FeedbackTargetType string
const (
    TargetInterruption FeedbackTargetType = "interruption"
    TargetDraft        FeedbackTargetType = "draft"
)

type FeedbackSignal string
const (
    SignalHelpful     FeedbackSignal = "helpful"
    SignalUnnecessary FeedbackSignal = "unnecessary"
)

type FeedbackRecord struct {
    FeedbackID string  // Deterministic from inputs
    TargetType FeedbackTargetType
    TargetID   string
    CircleID   identity.EntityID
    Timestamp  time.Time
    Signal     FeedbackSignal
    Reason     string
}
```

#### 4. Web Server (`cmd/quantumlife-web/`)

The web server provides:

| Route | Method | Description |
|-------|--------|-------------|
| `/` | GET | Home page with NeedsYou summary |
| `/circles` | GET | List all circles |
| `/circle/{id}` | GET | Circle detail view |
| `/drafts` | GET | All pending drafts |
| `/draft/{id}` | GET | Draft detail |
| `/draft/{id}/approve` | POST | Approve a draft |
| `/draft/{id}/reject` | POST | Reject a draft |
| `/feedback` | POST | Record feedback |
| `/history` | GET | Execution history |
| `/run` | POST | Manually trigger loop run |

Technology: stdlib `net/http` + `html/template`. No external dependencies.

#### 5. Run Options (`internal/loop/engine.go`)

```go
type RunOptions struct {
    CircleID              identity.EntityID  // Filter to specific circle
    IncludeMockData       bool               // Use mock data for demo
    ExecuteApprovedDrafts bool               // Auto-execute approved drafts
}
```

### Determinism Requirements

#### Run ID

Every loop run gets a deterministic ID:

```go
func computeRunID(now time.Time, opts RunOptions) string {
    canonical := fmt.Sprintf("run|%s|%s|%t",
        now.UTC().Format(time.RFC3339Nano),
        opts.CircleID,
        opts.IncludeMockData,
    )
    hash := sha256.Sum256([]byte(canonical))
    return hex.EncodeToString(hash[:])[:16]
}
```

Same clock + same options = same RunID.

#### NeedsYou Hash

The NeedsYou hash captures the complete state:

```go
func computeNeedsYouHash(drafts []draft.Draft, interrupts []*interrupt.Interruption) string {
    var ids []string
    for _, d := range drafts { ids = append(ids, string(d.DraftID)) }
    for _, i := range interrupts { ids = append(ids, i.InterruptionID) }
    sort.Strings(ids)  // CRITICAL: Sort for determinism

    canonical := fmt.Sprintf("needsyou|%v", ids)
    hash := sha256.Sum256([]byte(canonical))
    return hex.EncodeToString(hash[:])[:16]
}
```

#### Output Determinism

All collections must be sorted before output:
- Circles sorted by ID
- Drafts sorted by DraftID
- Interruptions sorted by InterruptionID
- Event counts sorted by EventType

This ensures identical output given identical inputs.

### Events

Phase 6 events (defined in `pkg/events/events.go`):

```go
// Daily loop lifecycle
Phase6DailyRunStarted
Phase6DailyRunCompleted

// View and state
Phase6ViewComputed
Phase6NeedsYouComputed

// Feedback
Phase6FeedbackRecorded
```

### The "Quiet" State

When `NeedsYou.IsQuiet == true`:

1. No pending drafts requiring approval
2. No active interruptions requiring attention
3. All obligations have been processed
4. User can trust the silence

The UI displays: **"Nothing Needs You"** with a message like "All caught up. Enjoy the quiet."

This is the goal state. The system should strive to reach quiet as quickly as possible after the user has taken all necessary actions.

### Graceful Shutdown (Phase 6.1)

The web server supports graceful shutdown via SIGINT/SIGTERM signals:

```
┌─────────────────────────────────────────────────────────────────┐
│                 Graceful Shutdown Architecture                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  SIGNAL (SIGINT/SIGTERM)                                         │
│       │                                                          │
│       ▼                                                          │
│  Signal Handler Goroutine (cmd/quantumlife-web ONLY)             │
│       │   - Prints: "quantumlife-web: shutting down"             │
│       │   - Creates 3-second timeout context                     │
│       │                                                          │
│       ▼                                                          │
│  http.Server.Shutdown(ctx)                                       │
│       │   - Stops accepting new connections                      │
│       │   - Waits for in-flight requests to complete             │
│       │   - Times out after 3 seconds                            │
│       │                                                          │
│       ▼                                                          │
│  Exit with code 0                                                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**CRITICAL INVARIANT**: The signal-handling goroutine is ONLY in the command layer (`cmd/quantumlife-web/main.go`). Core packages (`internal/`, `pkg/`) remain synchronous with no goroutines. This preserves:

1. **Determinism** — Core computation is single-threaded and reproducible
2. **Testability** — No race conditions in business logic
3. **Simplicity** — Concurrency is isolated to infrastructure concerns

Makefile convenience targets:
- `make web-mock` — Run with mock data
- `make web` — Run without mock data
- `make web-stop` — Stop the server on :8080
- `make web-status` — Check if :8080 is bound

## Consequences

### Positive

1. **Unified interface** — All Phase 1-5 capabilities accessible through one loop
2. **Earned silence** — "Nothing Needs You" is trustworthy because everything is processed
3. **Feedback loop** — System can learn from helpful/unnecessary signals
4. **Fully auditable** — Every run produces deterministic, logged results
5. **Web-first** — Accessible from any device with a browser

### Negative

1. **Latency** — Full loop runs on every request (no caching yet)
2. **Single-threaded** — No parallel processing of circles (by design)
3. **No real-time** — User must refresh to see updates

### Neutral

1. **Template rendering** — Go html/template is verbose but safe and stdlib
2. **No JavaScript** — All interactions are form POSTs (progressive enhancement possible later)

## Alternatives Considered

### 1. Background polling loop

Rejected: Violates no-background-workers guardrail. All processing is request-driven.

### 2. WebSocket real-time updates

Rejected: Adds complexity. Start with request/response, add later if needed.

### 3. SPA with API backend

Rejected: stdlib-only requirement. Server-rendered HTML is simpler and sufficient.

### 4. Caching NeedsYou state

Rejected for now: Deterministic hash enables cache invalidation, but adds complexity. Start without caching.

## Demo

The demo at `internal/demo_phase6_quiet_loop/` demonstrates:

1. Empty state → "Nothing Needs You"
2. Adding events → Loop processes → Items need attention
3. Feedback capture → Helpful/unnecessary signals stored
4. Determinism check → Same inputs produce identical RunIDs and hashes

## References

- Phase 2: ADR-0019-phase2-obligation-extraction.md
- Phase 3: ADR-0020-phase3-interruptions-and-digest.md
- Phase 4: ADR-0021-phase4-drafts-only-assistance.md
- Phase 5: ADR-0022-phase5-calendar-execution-boundary.md
- No Background Execution: ADR-0010-no-background-execution-guardrail.md
- No Auto-Retry: ADR-0011-no-auto-retry-and-single-trace-finalization.md
