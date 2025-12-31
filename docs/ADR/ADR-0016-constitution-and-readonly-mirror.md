# ADR-0016: Constitution and Read-Only Mirror

## Status

Accepted

## Context

QuantumLife lacks a formal declaration of its core principles and constraints. While individual guardrails and policies exist across multiple specifications, there is no single document that establishes:

1. **North Star Definition**: What "Nothing Needs You" means in practice
2. **User Rights**: Sovereignty, transparency, consent, and safe defaults
3. **Interruption Rights**: When and how the system may demand attention
4. **Execution Constraints**: Non-negotiable rules for financial actions
5. **Non-Goals**: What the system explicitly refuses to become

Additionally, the read-only ingestion pathway lacks formalization. External data sources (Gmail, Google Calendar, financial aggregators) need to be ingested into canonical event formats without any writes to those external systems.

## Decision

### Part A: Establish the QuantumLife Constitution

Create `docs/QUANTUMLIFE_CONSTITUTION_V1.md` as the foundational law of the system with the following structure:

#### Article I: North Star
- "Nothing Needs You" means: open QuantumLife, feel calm, close with earned trust
- Success metric: Users should frequently see "Nothing Needs You"
- Absence is the goal, not engagement

#### Article II: Core Rights
1. **Sovereignty**: User owns all data, exports freely, deletes completely
2. **Transparency**: Every decision has audit trail, no black boxes
3. **Consent for Writes**: Financial actions require explicit approval (v9+)
4. **Safe Defaults**: Opt-in to everything, off by default

#### Article III: Interruption Rights
- Only earned interruptions (calendar deadline, pending bill, waiting approval)
- Levels: Badge, Push, Urgent Push (with rate limits)
- Regret-driven threshold: Would I regret NOT seeing this?
- Rate limits: Max 3 pushes/day, max 1 urgent/week

#### Article IV: Execution Constitution
- No background execution in core (v9.7)
- No auto-retry, single trace finalization (v9.8)
- Provider registry lock (v9.9)
- Payee registry lock (v9.10)
- Caps and rate limits (v9.11)
- Policy snapshot binding (v9.12)
- View freshness binding (v9.13)

#### Article V: Non-Goals
- No engagement loops, no gamification
- No spam notifications, no urgency manufacturing
- No "AI as boss", no autonomous value judgments
- No dark patterns, no attention-capture

#### Article VI: Amendment Process
- Core principles require unanimous agreement
- Guardrails can be strengthened, never weakened
- All changes documented in ADRs

#### Article VII: Enforcement
- Guardrail scripts block CI on violations
- Audit events track all decisions
- Constitution violations trigger system halt

### Part B: Read-Only Mirror Architecture

Implement read-only ingestion with the following components:

#### 1. Identity Graph (`pkg/domain/identity`)

```go
type EntityType string // person, organization, email_account, etc.
type EntityID string   // Deterministic: {type}_{sha256_prefix}

type Entity interface {
    ID() EntityID
    Type() EntityType
    CanonicalString() string
    CreatedAt() time.Time
}
```

**Key entities**: Person, EmailAccount, CalendarAccount, FinanceAccount, Organization, Circle, Payee

**Deterministic ID generation**:
- Use SHA256 hash of canonical string
- Prefix with entity type
- 16-character hash suffix for readability

**Email normalization**:
- Gmail dot-insensitivity (s.a.t.i.s.h → satish)
- Plus-addressing ignored (satish+spam → satish)
- googlemail.com → gmail.com

#### 2. Canonical Events (`pkg/domain/events`)

```go
type CanonicalEvent interface {
    EventID() string
    EventType() EventType
    SourceVendor() string
    CapturedAt() time.Time
    CircleID() identity.EntityID
    EntityRefs() []identity.EntityRef
    CanonicalString() string
}
```

**Event types**:
- `EmailMessageEvent`: Ingested email message
- `CalendarEventEvent`: Ingested calendar event
- `TransactionEvent`: Ingested financial transaction
- `BalanceEvent`: Account balance snapshot

**Design principles**:
- Vendor-agnostic (canonical format, not Gmail/Outlook specific)
- Deterministic event IDs via SHA256
- No full body storage (preview only)
- EntityRef for identity graph links

#### 3. View Snapshots (`pkg/domain/view`)

```go
type CircleViewSnapshot struct {
    CircleID     identity.EntityID
    CapturedAt   time.Time
    Counts       ViewCounts
    Hash         string // Deterministic
}

func (s *CircleViewSnapshot) NothingNeedsYou() bool {
    return s.Counts.UnreadEmails == 0 &&
           s.Counts.TodayEvents == 0 &&
           s.Counts.PendingTransactions == 0
}
```

#### 4. Read-Only Adapters (`internal/integrations/*_read`)

```go
// gmail_read.Adapter
type Adapter interface {
    FetchMessages(accountEmail string, since time.Time, limit int) ([]*events.EmailMessageEvent, error)
    FetchUnreadCount(accountEmail string) (int, error)
    Name() string
}
```

**Guardrails**:
- No write methods exist in interface
- Package name includes `_read` suffix
- CRITICAL comments in package docs

#### 5. Ingestion Runner (`internal/ingestion`)

```go
type Runner struct {
    clock        clock.Clock
    eventStore   events.EventStore
    viewStore    view.ViewStore
}

// GUARDRAIL: This method does NOT spawn goroutines
func (r *Runner) Run(config *Config) (*RunResult, error)
```

**Design constraints**:
- Synchronous execution (no goroutines)
- Clock injection (no time.Now in core)
- Stdlib only (no external dependencies)

#### 6. Ingest Command (`cmd/quantumlife-ingest`)

```go
// Command quantumlife-ingest performs a single synchronous ingestion run.
// CRITICAL: This command runs once and exits. It does NOT spawn background
// processes or polling loops. For continuous ingestion, use a scheduler.
```

## Consequences

### Positive

- **Single source of truth**: Constitution establishes core principles
- **Vendor-agnostic ingestion**: Canonical events work across providers
- **Deterministic IDs**: Same input always produces same ID
- **Entity unification**: One person across 20 email accounts
- **Clean separation**: Read-only adapters cannot accidentally write
- **Testable**: Clock injection enables deterministic tests

### Negative

- **More packages**: Identity, events, view, ingestion, adapters
- **Normalization complexity**: Email and merchant normalization rules
- **No background polling**: External scheduler required for continuous ingestion

### Tradeoffs

1. **Determinism over simplicity**: SHA256 hashing adds complexity but ensures reproducibility.

2. **Canonical events over vendor-specific**: Transformation layer required but enables multi-vendor support.

3. **Synchronous over background**: Caller controls scheduling but requires external orchestration.

## Compatibility

- Integrates with v9.x execution constraints
- Clock injection matches existing pkg/clock pattern
- In-memory stores match existing test patterns
- Guardrail scripts verify compliance

## Implementation Notes

### Files Created

```
docs/QUANTUMLIFE_CONSTITUTION_V1.md       # Constitution document
pkg/domain/identity/types.go              # Entity types and ID generation
pkg/domain/identity/generator.go          # Factory functions with normalization
pkg/domain/identity/repository.go         # Storage interfaces
pkg/domain/identity/identity_test.go      # Determinism and collision tests
pkg/domain/events/canonical.go            # Canonical event types
pkg/domain/events/store.go                # In-memory event store
pkg/domain/view/snapshot.go               # View snapshot with hash
internal/integrations/gmail_read/         # Read-only Gmail adapter
internal/integrations/gcal_read/          # Read-only Calendar adapter
internal/integrations/finance_read/       # Read-only Finance adapter
internal/ingestion/runner.go              # Synchronous ingestion runner
cmd/quantumlife-ingest/main.go            # Entry point command
internal/demo_readonly_mirror/            # Demo and tests
```

### Makefile Target

```makefile
ingest-once:
	go run ./cmd/quantumlife-ingest
```

### Demo Test

`TestReadOnlyMirror_SatishConfig` creates:
- 20 emails (15 work, 5 personal)
- 5 calendar events (3 work, 2 family)
- 3 transactions + 1 balance

Verifies:
- Event counts per type
- View snapshot counts per circle
- Deterministic hashes across runs
- "Nothing Needs You" status

## References

- docs/QUANTUMLIFE_CONSTITUTION_V1.md
- docs/TECH_SPEC_V1.md
- docs/IDENTITY_GRAPH_V1.md (referenced)
- ADR-0010-no-background-execution-guardrail.md
- ADR-0011-no-auto-retry-and-single-trace-finalization.md
