# ADR-0021: Phase 4 - Drafts-Only Assistance

## Status
Accepted

## Context

QuantumLife progresses through phases of increasing capability:
- Phase 1: Read-Only Mirror (observability)
- Phase 2: Obligation Extraction (what needs attention)
- Phase 3: Interruption Engine (when to surface)
- **Phase 4: Drafts-Only Assistance (actionable proposals)**

Phase 4 introduces the ability to generate actionable drafts (email replies, calendar responses) from obligations. These drafts are **internal proposals only** - they require explicit user approval before any external action occurs.

### Problem Statement

Users receive obligations (emails needing reply, calendar invites needing response) but currently must compose responses manually. We want to:

1. Generate draft responses that users can review and approve
2. Maintain full user control - no autonomous external writes
3. Ensure determinism - same inputs produce same drafts
4. Provide deduplication to prevent proposal spam
5. Support TTL-based expiration for stale drafts
6. Enable rate limiting per circle per day

### Constraints

- **stdlib only** - no external dependencies
- **No goroutines/timers in core** - deterministic, testable code
- **Injected clock** - never call `time.Now()` directly
- **Canonical hashing (NOT JSON)** - deterministic ID generation
- **NO external writes** - drafts are internal artifacts only

## Decision

### 1. Draft Domain Model (`pkg/domain/draft/`)

**types.go:**
- `DraftID`: deterministic identifier (sha256 of canonical string, 16 hex chars)
- `DraftType`: `email_reply` | `calendar_response`
- `DraftStatus`: `proposed` | `approved` | `rejected` | `expired`
- `Draft`: contains all draft metadata and content
- `EmailDraftContent`: email-specific content (to, cc, subject, body, thread)
- `CalendarDraftContent`: calendar-specific content (event, response, message)

**Determinism guarantees:**
- `CanonicalString()` produces stable string representation
- `Hash()` produces deterministic SHA256 hash
- `DedupKey()` enables deduplication within same obligation/thread
- `SortDrafts()` provides stable ordering

**policy.go:**
- `DraftPolicy`: configures TTLs and rate limits
- `EmailTTLHours`: 48 (default)
- `CalendarTTLHours`: 72 (default)
- `MaxDraftsPerCirclePerDay`: 20 (rate limit)

**store.go:**
- `Store` interface for draft persistence
- `InMemoryStore` implementation with deterministic operations
- Thread-safe with proper mutex locking

### 2. Draft Generation Engines (`internal/drafts/`)

**interfaces.go:**
- `DraftGenerator` interface for type-specific engines
- `GenerationContext` provides input for generation
- `GenerationResult` contains output or skip reason

**email/engine.go:**
- Handles `ObligationReply` + `SourceType=email`
- Extracts context from obligation evidence
- Generates rule-based reply body (LLM hook available)
- Builds safety notes for external recipients, urgency

**calendar/engine.go:**
- Handles `ObligationDecide` or `ObligationAttend` + `SourceType=calendar`
- Extracts context from obligation evidence
- Determines response using configurable rules
- Builds safety notes for external organizers, all-day events

**engine.go (orchestrator):**
- Routes obligations to appropriate generator
- Handles deduplication via `DedupKey`
- Enforces rate limits via `DraftQuotaTracker`
- Provides batch processing with deterministic ordering

### 3. Review/Approval Flow (`internal/drafts/review/`)

**service.go:**
- `GetForReview()`: retrieves draft with safety warnings
- `Approve()`: marks draft as approved (runs safety checks)
- `Reject()`: marks draft as rejected (requires reason)
- `ListPending()`: returns pending drafts for circle
- `ExpireStale()`: marks expired drafts

**Safety checks:**
- Run before approval (blocking)
- Collected as warnings during review (non-blocking)
- Extensible via `SafetyCheck` interface

### 4. Audit Events (`pkg/events/events.go`)

New event types for Phase 4:
- `draft.generated`: draft was created
- `draft.stored`: draft was persisted
- `draft.dedupe`: duplicate draft detected
- `draft.expired`: draft TTL expired
- `draft.review.started/completed`: review lifecycle
- `draft.approved/rejected`: approval lifecycle
- `draft.safety.*`: safety check results
- `draft.ratelimit.*`: rate limit enforcement
- `draft.rule.*`: generation rule matching

## Draft Lifecycle

```
Obligation → [Generate] → Draft (proposed)
                              ↓
                         [Review]
                              ↓
                    ┌─────────┴─────────┐
                    ↓                   ↓
              [Approve]            [Reject]
                    ↓                   ↓
             Draft (approved)    Draft (rejected)
                    ↓
            [Future: Execute]
```

TTL-based expiration:
```
Draft (proposed) → [time passes] → Draft (expired)
```

## Status Transitions

| From | To | Allowed |
|------|-----|---------|
| proposed | approved | ✓ |
| proposed | rejected | ✓ |
| proposed | expired | ✓ |
| approved | * | ✗ (terminal) |
| rejected | * | ✗ (terminal) |
| expired | * | ✗ (terminal) |

## Consequences

### Positive
- Users get actionable draft proposals
- Full control via explicit approval
- Deterministic, testable implementation
- Clear audit trail via events
- Rate limiting prevents proposal spam
- TTL prevents stale drafts

### Negative
- Drafts add storage overhead
- Review adds user friction (intentional)
- No LLM integration yet (rule-based only)

### Neutral
- Execution is a future concern (Phase 5+)
- Approved drafts are ready for execution when enabled

## References

- ADR-0019: Phase 2 Obligation Extraction
- ADR-0020: Phase 3 Interruption Engine
- TECHNICAL_SPLIT_V1.md §3.7 Audit & Governance Layer
