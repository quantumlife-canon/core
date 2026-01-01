# ADR-0026: Phase 11 - Real Data Quiet Loop (Multi-account)

**Status:** Accepted
**Date:** 2025-01-15
**Authors:** QuantumLife Team
**Supersedes:** None
**Related:** ADR-0023 (Phase 6 Quiet Loop), ADR-0019 (Phase 2 Obligations)

## Context

Previous phases established the Quiet Loop pattern with a single-account model. Real usage requires:

- Multiple circles (work, personal, family, finance)
- Multiple email/calendar integrations per circle
- Routing of events to appropriate circles
- Combined ingestion + loop execution per request

Phase 11 extends the architecture for multi-account, multi-circle operation.

## Decision

**Phase 11 implements Multi-Circle Real Data Quiet Loop with the following components:**

### 1. Multi-Account Configuration

A runtime-loaded configuration file (`.qlconf` format) defines:

- Circle definitions with email, calendar, and finance integrations
- Routing rules for event-to-circle assignment
- VIP sender lists, family members, work/personal domains

### 2. Multi-Account Ingestion Runner

A synchronous ingestion runner that:

- Processes circles in deterministic sorted order
- Fetches from all registered adapters per circle
- Tags events with CircleID before storage
- Returns structured receipts per circle/integration

### 3. Circle Routing Rules

Deterministic routing based on:

1. Receiver email address (highest priority)
2. Sender domain matching (work/personal)
3. Family member matching
4. Default circle fallback

### 4. Extended Loop Engine

MultiCircleRunner combines:

- Multi-account ingestion
- Per-circle loop execution
- Aggregated NeedsYou summary
- Circle sync state tracking

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Multi-Circle Config                       │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │  Work   │  │Personal │  │ Family  │  │Finance  │        │
│  │  email  │  │  email  │  │calendar │  │  plaid  │        │
│  │calendar │  │calendar │  │         │  │         │        │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                 Multi-Account Ingestion Runner               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ For each circle (sorted order):                      │    │
│  │   For each integration:                              │    │
│  │     Fetch via adapter → Tag with CircleID → Store   │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Circle Router                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Priority: receiver → domain → family → default      │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Multi-Circle Loop Runner                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Run ingestion (optional) → Run loop → Aggregate     │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
                    NeedsYou Summary
                    (per-circle breakdown)
```

## Configuration Format

The `.qlconf` format uses line-based parsing (stdlib only):

```ini
# Quantumlife Circle Configuration

[circle:personal]
name = Personal
email = google:me@gmail.com:email:read

[circle:work]
name = Work
email = google:work@company.com:email:read
calendar = google:primary:calendar:read

[circle:family]
name = Family
calendar = google:family-shared:calendar:read

[routing]
work_domains = company.com,corp.company.com
personal_domains = gmail.com,yahoo.com
family_members = spouse@gmail.com,kid@gmail.com
vip_senders = ceo@company.com
```

### Integration Format

```
provider:account_id:type:scopes
```

Examples:
- `google:me@gmail.com:email:read`
- `google:primary:calendar:read,write`
- `plaid:chase-checking:finance:read`

## Core Types

### MultiCircleConfig

```go
type MultiCircleConfig struct {
    Circles      map[identity.EntityID]*CircleConfig
    Routing      RoutingConfig
    LoadedAt     time.Time
    SourcePath   string
}

func (c *MultiCircleConfig) Hash() string          // Deterministic hash
func (c *MultiCircleConfig) CircleIDs() []EntityID // Sorted order
```

### CircleConfig

```go
type CircleConfig struct {
    ID                   identity.EntityID
    Name                 string
    EmailIntegrations    []EmailIntegration
    CalendarIntegrations []CalendarIntegration
    FinanceIntegrations  []FinanceIntegration
}
```

### RoutingConfig

```go
type RoutingConfig struct {
    WorkDomains     []string
    PersonalDomains []string
    FamilyMembers   []string
    VIPSenders      []string
}
```

### MultiCircleRunner

```go
type MultiCircleRunner struct {
    Engine       *Engine
    Clock        clock.Clock
    Config       *config.MultiCircleConfig
    Router       *routing.Router
    MultiRunner  *ingestion.MultiRunner
    SyncReceipts map[identity.EntityID]*CircleSyncState
    EventEmitter events.Emitter
}

func (r *MultiCircleRunner) Run(opts MultiCircleRunOptions) MultiCircleRunResult
```

## Routing Rules

### Email Routing Priority

1. **Receiver email match** - If email is sent TO a configured address, route to that circle
2. **Sender domain match** - If sender domain is in work_domains, route to work circle
3. **Family member match** - If sender is in family_members, route to family circle
4. **Personal domain match** - If sender domain is in personal_domains, route to personal
5. **Default** - Route to "personal" circle if exists, else first alphabetically

### Calendar Routing Priority

1. **Calendar ID match** - If calendar ID matches a configured integration
2. **Family attendee match** - If any attendee is in family_members, route to family
3. **Organizer domain match** - If organizer domain is in work_domains, route to work
4. **Default** - Route to "personal" circle if exists, else first alphabetically

### VIP Senders

VIP senders are flagged but do not affect routing. The Router exposes:

```go
func (r *Router) IsVIPSender(email string) bool
```

## Determinism Requirements

1. **Config hash** - Same file content = same hash (regardless of load time)
2. **Circle ordering** - Circles processed in sorted ID order
3. **Integration ordering** - Integrations processed in sorted order
4. **Routing** - Same event = same circle assignment (no randomness)
5. **Run hash** - Same inputs + clock = identical result hash

## Constraints

### Hard Constraints (inherited)

1. **stdlib only** - No YAML/JSON/TOML parsing libraries
2. **No goroutines** - All processing synchronous
3. **No time.Now()** - Injected clock only
4. **No auto-retry** - Single attempt, fail cleanly
5. **Deterministic** - Same inputs = same outputs

### Phase 11 Specific

1. **Config is read-only** - Loaded at startup, never modified at runtime
2. **Ingestion is per-request** - No background polling
3. **Receipts are immutable** - Created once per sync operation
4. **Circle IDs are stable** - Must not change across restarts

## Implementation

### Package Structure

```
internal/config/
├── types.go         # MultiCircleConfig, CircleConfig, RoutingConfig
├── loader.go        # LoadFromFile, LoadFromString
└── loader_test.go   # Config loading tests

internal/routing/
├── router.go        # Router with deterministic routing
└── router_test.go   # Routing tests

internal/ingestion/
├── multi_runner.go  # MultiRunner for multi-account ingestion
└── adapters.go      # Adapter interfaces

internal/loop/
└── multi_circle.go  # MultiCircleRunner

internal/demo_phase11_multicircle/
└── demo_test.go     # Phase 11 demo tests

configs/circles/
└── default.qlconf   # Sample configuration

scripts/guardrails/
└── multicircle_enforced.sh  # Phase 11 guardrail
```

### Web UI Updates

- `--config` flag for config file path
- `/circles` endpoint shows configured circles and sync status
- `/run/daily?circle=<id>` for circle-specific runs
- Template data includes CircleConfigs, ConfigHash

## Events

Phase 11 emits the following audit events:

| Event | Description |
|-------|-------------|
| `phase11.multicircle.run.started` | Multi-circle run started |
| `phase11.multicircle.run.completed` | Multi-circle run completed |
| `phase11.ingestion.started` | Ingestion started |
| `phase11.ingestion.completed` | Ingestion completed |
| `phase11.circle.synced` | Circle sync completed |
| `phase11.config.loaded` | Configuration loaded |
| `phase11.config.error` | Configuration error |
| `phase11.adapter.registered` | Adapter registered |
| `phase11.adapter.missing` | Required adapter missing |

## Acceptance Criteria

### Configuration

- [ ] `.qlconf` files load successfully with stdlib-only parsing
- [ ] Config hash is deterministic
- [ ] Circles are enumerated in sorted order
- [ ] Validation errors are clear and specific

### Routing

- [ ] Router assigns events to correct circles deterministically
- [ ] Receiver email has highest priority
- [ ] Work/personal domain routing works correctly
- [ ] Family member routing works correctly
- [ ] VIP sender detection works correctly

### Ingestion

- [ ] MultiRunner processes circles in sorted order
- [ ] Events are tagged with CircleID before storage
- [ ] Sync receipts are generated per circle/integration
- [ ] Missing adapters are handled gracefully

### Loop

- [ ] MultiCircleRunner combines ingestion + loop
- [ ] Circle-specific runs work correctly
- [ ] Result hash is deterministic
- [ ] NeedsYou is aggregated across circles

### Demo

- [ ] `make demo-phase11` runs successfully
- [ ] `make check-multicircle` passes all checks
- [ ] Demo verifies determinism
- [ ] Demo verifies routing

## Testing

```bash
# Run config tests
go test ./internal/config/... -v

# Run routing tests
go test ./internal/routing/... -v

# Run Phase 11 demo tests
go test ./internal/demo_phase11_multicircle/... -v

# Run guardrail
make check-multicircle

# Run full CI
make ci
```

## Future Work

Phase 12 and beyond may add:

- Real OAuth adapter implementations
- Token refresh handling
- Multi-party approval for multi-circle writes
- Cross-circle event correlation
- Circle-specific privacy settings

These are NOT in scope for Phase 11.
