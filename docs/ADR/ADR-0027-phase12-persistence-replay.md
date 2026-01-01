# ADR-0027: Phase 12 - Persistence + Deterministic Replay

## Status

Accepted

## Context

QuantumLife requires durable storage for all system state to ensure:
1. Data survives restarts
2. Runs can be replayed for verification
3. All state changes are auditable
4. Hashes allow integrity verification

This ADR defines the persistence layer that enables deterministic replay.

## Decision

### 1. Append-Only Log

All persistent state is stored in append-only logs:

```
pkg/domain/storelog/
  log.go          # AppendOnlyLog interface, LogRecord type
  file_log.go     # FileLog implementation with atomic writes
```

Record format:
```
TYPE|VERSION|TS|HASH|CIRCLE_ID|PAYLOAD
```

Example:
```
EVENT|v1|2025-01-15T10:30:00Z|abc123...|work|email|google|msg-1|test@example.com
```

### 2. Record Types

```go
const (
    RecordTypeEvent    = "EVENT"
    RecordTypeDraft    = "DRAFT"
    RecordTypeApproval = "APPROVAL"
    RecordTypeFeedback = "FEEDBACK"
    RecordTypeRun      = "RUN"
)
```

### 3. Persistent Stores

File-backed implementations in `internal/persist/`:

| Store | Purpose | Key Fields |
|-------|---------|------------|
| DraftStore | Draft persistence and status tracking | DraftID, Status, CircleID |
| FeedbackStore | User feedback records | FeedbackID, Signal, TargetID |
| DedupStore | Interruption deduplication | DedupKey, SeenAt |
| ApprovalStore | Multi-party approval artifacts | ApprovalID, ActionHash |

Each store:
- Implements replay from log on startup
- Uses the same log for all writes
- Maintains in-memory indexes for fast lookup
- Supports listing by type and circle

### 4. Run Snapshots

Run snapshots capture complete run state:

```go
type RunSnapshot struct {
    RunID                     string
    StartTime, EndTime        time.Time
    CircleID                  identity.EntityID
    EventsIngested            int
    InterruptionsCreated      int
    InterruptionsDeduplicated int
    DraftsCreated             int
    NeedsYouItems             int
    EventHashes               []string  // Sorted
    InterruptionHashes        []string  // Sorted
    DraftHashes               []string  // Sorted
    NeedsYouHash              string
    ResultHash                string    // Computed
    ConfigHash                string
}
```

### 5. Replay Verification

```go
func VerifyReplay(original, replay *RunSnapshot) *ReplayResult {
    // Compare hashes
    // Report any differences
}
```

### 6. Guardrails Enforced

The `persistence_replay_enforced.sh` script verifies:
1. storelog uses stdlib only
2. Log records include hash verification
3. Persistent stores implement replay
4. Run snapshots compute deterministic hashes
5. No goroutines in Phase 12 packages
6. No time.Now() in Phase 12 packages
7. RunStore interface exists
8. AppendOnlyLog interface exists
9. Demo tests exist
10. Replay verification exists

## Consequences

### Positive

- All state is durable and survives restarts
- Append-only logs provide full audit trail
- Hash verification ensures data integrity
- Replay enables debugging and verification
- File-based storage is simple and portable

### Negative

- Log files grow unbounded (compaction needed later)
- Replay requires re-reading entire log
- More disk I/O than in-memory storage

### Neutral

- Log format is extensible via VERSION field
- Multiple stores share one log (simpler, but coupled)

## Implementation

### Package Structure

```
pkg/domain/
  storelog/         # Append-only log
    log.go          # LogRecord, AppendOnlyLog interface
    file_log.go     # FileLog, InMemoryLog
    log_test.go
  runlog/           # Run snapshots
    snapshot.go     # RunSnapshot, VerifyReplay
    store.go        # RunStore, FileRunStore
    runlog_test.go

internal/
  persist/          # Persistent store implementations
    draft_store.go
    feedback_store.go
    dedup_store.go
    approval_store.go
    persist_test.go
  demo_phase12_persistence_replay/
    demo_test.go
```

### Log File Location

Default: `~/.quantumlife/quantumlife.log`

### Atomic Writes

FileLog uses atomic writes:
1. Write to temp file
2. Sync to disk
3. Rename to target

This ensures log integrity even during crashes.

### Schema Evolution

Records include VERSION field for forward compatibility:
- v1: Initial format
- Future versions can add fields while reading older records

## References

- Canon v1: "QuantumLife never executes without explicit human approval"
- v9.6.2: "Controlled Clock Injection"
- Phase 11: Multi-circle ingestion
- Phase 6: The Quiet Loop

## Checklist

- [x] Append-only log interface and implementation
- [x] Hash verification on all records
- [x] Persistent stores with replay
- [x] Run snapshot and verification
- [x] No goroutines (synchronous only)
- [x] No time.Now() (injected clock)
- [x] Demo tests passing
- [x] Guardrail script
- [x] Makefile targets
