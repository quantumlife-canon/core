# ADR-0057: Phase 26C - Connected Reality Check

## Status
Accepted

## Context

After implementing Phase 26A (Guided Journey) and Phase 26B (First Five Minutes Proof), we need a way to prove "this is real" - that real data connections are working, syncing, and staying quiet.

The challenge: how to prove realness without exposing content, identifiers, timestamps, vendors, people, or secrets?

Traditional approaches would show:
- Raw connection details
- API endpoint URLs
- Last sync timestamps
- Message counts
- Configuration file paths
- Environment variable values

These approaches create:
- Security exposure risk
- Privacy violations
- Surveillance anxiety
- Complexity for non-technical observers

QuantumLife's philosophy is different: we prove reality through abstract buckets and deterministic hashes.

## Decision

Introduce a **Connected Reality Check** page (`/reality`) - a single calm trust proof page that answers:

1. Is Gmail connected? (yes/no)
2. Did sync happen? (never/recent/stale)
3. Were messages noticed? (magnitude bucket)
4. Are obligations held? (yes/no)
5. Is auto-surface off? (yes/no)
6. What is shadow mode status? (off/stub/azure)
7. Is chat/embed configured? (yes/no)

### Why This Is Not Analytics

| Analytics | Reality Check |
|-----------|---------------|
| Tracks behavior | Proves connection |
| Raw counts | Magnitude buckets |
| Timestamps | Recency buckets |
| Dashboards | Single page |
| Optimization | Trust |
| Engagement | Proof |

### Why Abstract Buckets

Instead of showing "synced 3 minutes ago", we show "recent".
Instead of showing "47 messages noticed", we show "several".
Instead of showing "https://azure-endpoint.openai.com", we show "endpoint: yes".

This protects:
- Privacy (no raw data exposure)
- Security (no endpoint URLs)
- Trust (no surveillance feeling)

### Why A Single StatusHash

The page includes a deterministic SHA256 hash (32 hex chars) that:
- Proves the page content is deterministic
- Allows acknowledgement tracking
- Enables audit without storing content

## Implementation

### Domain Model

```go
// pkg/domain/reality/types.go

type RealityLineKind string
const (
    LineKindBool   = "bool"   // yes/no
    LineKindBucket = "bucket" // nothing/a_few/several
    LineKindEnum   = "enum"   // never/recent/stale
    LineKindNote   = "note"   // informational
)

type RealityLine struct {
    Label string
    Value string
    Kind  RealityLineKind
}

type RealityPage struct {
    Title      string
    Subtitle   string
    Lines      []RealityLine
    CalmLine   string
    StatusHash string  // 128-bit deterministic hash
    BackPath   string
}

type RealityInputs struct {
    CircleID           string
    NowBucket          string           // Period key, not timestamp
    GmailConnected     bool
    SyncBucket         SyncBucket       // never/recent/stale/unknown
    SyncMagnitude      MagnitudeBucket  // nothing/a_few/several/na
    ObligationsHeld    bool
    AutoSurface        bool
    ShadowProviderKind ShadowProviderKind
    ShadowRealAllowed  bool
    ShadowMagnitude    MagnitudeBucket
    ChatConfigured     bool
    EmbedConfigured    bool
    EndpointConfigured bool
    Region             string  // If explicitly configured
}

type RealityAck struct {
    Period     string
    StatusHash string
}
```

### Engine

```go
// internal/reality/engine.go

type Engine struct {
    clock Clock
}

func (e *Engine) BuildPage(inputs *RealityInputs) *RealityPage
func (e *Engine) ComputeCue(inputs *RealityInputs, acked bool) *RealityCue
func (e *Engine) ShouldShowRealityCue(...) bool
```

**CalmLine selection** (deterministic):
- Not connected: "Nothing is connected yet. Quiet is still the baseline."
- Connected but never synced: "Connected. Waiting for your explicit sync."
- Synced: "Quiet baseline verified."

### Persistence

```go
// internal/persist/reality_ack_store.go

type RealityAckStore struct {
    acks       map[string]*RealityAck  // period -> ack
    maxPeriods int  // 30 days bounded retention
    clock      func() time.Time
}
```

Constraints:
- Hash-only storage (no raw content)
- Bounded retention (30 days)
- Append-only with storelog integration

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/reality` | GET | Show reality check page |
| `/reality/ack` | POST | Acknowledge page, redirect to /today |

### UI Design

```
┌─────────────────────────────────────────┐
│  Connected, quietly.                    │
│  Proof that real data stays quiet.      │
│                                         │
│  Gmail connected         yes            │
│  Last sync              recent          │
│  Messages noticed       a few           │
│  Obligations held       yes             │
│  Auto-surface           no              │
│  Shadow mode            stub            │
│  Real providers allowed no              │
│  Chat configured        yes             │
│  Embeddings configured  no              │
│  Endpoint configured    yes             │
│                                         │
│  Quiet baseline verified.               │
│                                         │
│  StatusHash: a1b2c3d4...                │
│                                         │
│                    [Back to today]      │
└─────────────────────────────────────────┘
```

**No:** raw counts, timestamps, URLs, API keys, email addresses

### Whisper Integration

Priority order (single whisper rule):
1. Surface cue
2. Proof cue
3. First-minutes cue (26B)
4. **Reality cue (26C)** - lowest priority

Cue text: "if you ever wondered—connected is real."
Link text: "proof"

Cue only appears when:
- Gmail is connected
- At least one sync has happened
- Not already acknowledged for current status hash
- No higher priority cues are active

### Events

```go
Phase26CRealityRequested  = "phase26c.reality.requested"
Phase26CRealityComputed   = "phase26c.reality.computed"
Phase26CRealityViewed     = "phase26c.reality.viewed"
Phase26CRealityAckRecorded = "phase26c.reality.ack.recorded"
```

All payloads contain hashes only - never identifiers.

## Guardrails

47 guardrails enforce:

1. **Package structure** - domain, engine, store exist
2. **Stdlib only** - no external dependencies
3. **No time.Now()** - clock injection only
4. **No goroutines** - synchronous execution
5. **Domain model** - all types and enums
6. **Engine** - BuildPage, ComputeCue, deterministic
7. **Persistence** - bounded retention, hash-only
8. **Events** - all 4 events defined
9. **Web routes** - /reality endpoints
10. **Privacy** - no forbidden tokens

## Data Sources (Read-Only)

Phase 26C reads from existing stores - no new data collection:

| Store | Method | Signal |
|-------|--------|--------|
| ConnectionStore | `State()` | GmailConnected |
| SyncReceiptStore | `GetLatestByCircle()` | SyncBucket, SyncMagnitude |
| ShadowReceiptStore | `ListForCircle()` | ShadowMagnitude |
| Config | `Shadow` | ProviderKind, RealAllowed, ChatConfigured, etc. |

## Consequences

### Positive
1. Proves realness without exposing content
2. Single page vs complex dashboards
3. Deterministic and auditable
4. Privacy-preserving by design
5. Dismissable and non-intrusive
6. Calm language throughout

### Negative
1. Cannot show exact timestamps (by design)
2. Cannot show raw counts (by design)
3. Cannot expose configuration details (by design)

### Neutral
1. Abstract buckets may confuse technical observers expecting raw data
2. Page requires sync to have happened at least once to be meaningful

## Absolute Constraints

- stdlib only
- No new concepts introduced
- No config flags
- No secrets exposed (API keys, tokens, env values)
- No endpoint URLs shown
- No timestamps shown
- No raw counts shown
- No goroutines
- No time.Now()
- Deterministic: same inputs + clock => identical output + hash

**This is NOT analytics. This is a trust proof page.**

## Files Created

| File | Purpose |
|------|---------|
| `pkg/domain/reality/types.go` | Domain model |
| `internal/reality/engine.go` | Engine |
| `internal/persist/reality_ack_store.go` | Persistence |
| `scripts/guardrails/reality_check_enforced.sh` | Guardrails (47 checks) |
| `internal/demo_phase26C_reality_check/demo_test.go` | Demo tests (16 tests) |

## Files Modified

| File | Changes |
|------|---------|
| `pkg/domain/storelog/log.go` | Added RecordTypeRealityAck |
| `pkg/events/events.go` | Added Phase 26C events |
| `cmd/quantumlife-web/main.go` | Added routes, handlers, whisper integration |

## Related ADRs

- ADR-0056: Phase 26B - First Five Minutes Proof
- ADR-0055: Phase 26A - Guided Journey
- ADR-0043: Phase 19.2 - Shadow Mode Contract
- ADR-0038: Phase 18.6 - First Connect

## References

- `docs/QUANTUMLIFE_CANON_V1.md` - Core philosophy
- `docs/HUMAN_GUARANTEES_V1.md` - Trust contracts
