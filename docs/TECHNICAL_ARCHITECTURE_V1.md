# Technical Architecture v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Supersedes | None |
| Related | ARCHITECTURE_LIFE_OS_V1.md, CANONICAL_CAPABILITIES_V1.md, TECH_SPEC_V1.md |

---

## 1. Overview

This document defines the technical architecture of QuantumLife, with explicit focus on:

1. **v9.7 Compliance**: Reconciling background ingestion with no-background-execution in core
2. **Vendor Explosion**: Capability-based connector architecture that abstracts vendor specifics
3. **Process Topology**: Clear separation of concerns across runtime boundaries

**Core Invariant**: The core engine is deterministic, synchronous, and auditable. All asynchronous/background work happens in separate processes that communicate via defined interfaces.

---

## 2. Process Architecture

### 2.1 Runtime Topology

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           QUANTUMLIFE SYSTEM                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    INGESTION LAYER (Background OK)                   │   │
│  │                                                                       │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│  │  │   Email      │  │  Calendar    │  │   Finance    │               │   │
│  │  │   Poller     │  │   Syncer     │  │   Poller     │   ...         │   │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘               │   │
│  │         │                 │                 │                        │   │
│  │         └────────────────┬┴─────────────────┘                        │   │
│  │                          │                                           │   │
│  │                          ▼                                           │   │
│  │              ┌───────────────────────┐                               │   │
│  │              │    Canonical Event    │                               │   │
│  │              │        Queue          │                               │   │
│  │              └───────────┬───────────┘                               │   │
│  │                          │                                           │   │
│  └──────────────────────────┼───────────────────────────────────────────┘   │
│                             │                                               │
│  ═══════════════════════════╪═══════════════════════════════════════════   │
│       PROCESS BOUNDARY      │  (IPC: gRPC / Unix Socket / Message Queue)   │
│  ═══════════════════════════╪═══════════════════════════════════════════   │
│                             │                                               │
│  ┌──────────────────────────┼───────────────────────────────────────────┐   │
│  │                          ▼                                           │   │
│  │              ┌───────────────────────┐                               │   │
│  │              │    Event Receiver     │  (Synchronous read from queue)│   │
│  │              └───────────┬───────────┘                               │   │
│  │                          │                                           │   │
│  │                          ▼                                           │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │                 CORE ENGINE (v9.7 Compliant)                   │   │   │
│  │  │                                                                 │   │   │
│  │  │   ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐       │   │   │
│  │  │   │ Classify│──▶│  Model  │──▶│ Decide  │──▶│ Propose │       │   │   │
│  │  │   └─────────┘   └─────────┘   └─────────┘   └─────────┘       │   │   │
│  │  │                                                                 │   │   │
│  │  │   - No goroutines in core packages                             │   │   │
│  │  │   - Deterministic execution                                    │   │   │
│  │  │   - All state changes are synchronous                          │   │   │
│  │  │   - Full audit trail                                           │   │   │
│  │  │                                                                 │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  │                          │                                           │   │
│  │                          ▼                                           │   │
│  │              ┌───────────────────────┐                               │   │
│  │              │   Proposal Store      │                               │   │
│  │              │   (Pending Actions)   │                               │   │
│  │              └───────────────────────┘                               │   │
│  │                                                                       │   │
│  │                    CORE PROCESS (No Background Execution)             │   │
│  └───────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│       PROCESS BOUNDARY                                                      │
│  ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                    EXECUTION LAYER (User-Triggered)                   │   │
│  │                                                                       │   │
│  │   User Approval (tap)                                                 │   │
│  │          │                                                            │   │
│  │          ▼                                                            │   │
│  │   ┌─────────────────┐                                                │   │
│  │   │ Execution       │  - v9.7: Triggered by user action              │   │
│  │   │ Coordinator     │  - v9.8: No auto-retry                         │   │
│  │   │                 │  - v9.12: Policy snapshot binding              │   │
│  │   └────────┬────────┘  - v9.13: View freshness binding               │   │
│  │            │                                                          │   │
│  │            ▼                                                          │   │
│  │   ┌─────────────────┐     ┌─────────────────┐                        │   │
│  │   │ Write Provider  │────▶│ External API    │                        │   │
│  │   │ (TrueLayer etc) │     │ (Bank, Email)   │                        │   │
│  │   └─────────────────┘     └─────────────────┘                        │   │
│  │                                                                       │   │
│  └───────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Process Definitions

| Process | Background Allowed | v9.7 Status | Responsibilities |
|---------|-------------------|-------------|------------------|
| **Ingestion Workers** | YES | N/A (outside core) | Poll vendors, transform to canonical events, enqueue |
| **Core Engine** | NO | Compliant | Classify, model, decide, propose - all synchronous |
| **Execution Coordinator** | NO | Compliant | Execute approved actions, called synchronously by user action |
| **API Gateway** | YES (for HTTP) | N/A (infrastructure) | Serve mobile/web clients, route requests |

### 2.3 v9.7 Reconciliation

The v9.7 guardrail states: "Core packages never spawn goroutines for execution."

**Resolution**:

```
┌─────────────────────────────────────────────────────────────────┐
│                        v9.7 COMPLIANCE MODEL                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ALLOWED (Outside Core):                                        │
│  ├── cmd/ingestion-worker/     → Background polling OK          │
│  ├── cmd/scheduler/            → Cron-style scheduling OK       │
│  ├── cmd/api-gateway/          → HTTP server goroutines OK      │
│  └── infrastructure/           → Database pools, caches OK      │
│                                                                 │
│  FORBIDDEN (Inside Core):                                       │
│  ├── internal/finance/execution/   → No goroutines              │
│  ├── internal/interruption/        → No goroutines              │
│  ├── internal/obligation/          → No goroutines              │
│  ├── pkg/primitives/               → No goroutines              │
│  └── pkg/events/                   → No goroutines              │
│                                                                 │
│  BOUNDARY ENFORCEMENT:                                          │
│  ├── Core receives events via synchronous channel read          │
│  ├── Core processes one event at a time                         │
│  ├── Core writes proposals to store (synchronous)               │
│  └── Execution only on user-triggered API call                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Capability-Based Connector Architecture

### 3.1 The Vendor Explosion Problem

Satish has accounts across:
- **UK**: Barclays, Monzo, NatWest, Amex (UK), British Gas, Thames Water, ...
- **India**: HDFC, ICICI, Airtel, Jio, ...
- **US**: (potential future) Chase, Citi, ...

**Anti-Pattern**: Writing vendor-specific code in core
```go
// BAD: Vendor logic in core
switch vendor {
case "barclays":
    // Barclays-specific parsing
case "hdfc":
    // HDFC-specific parsing
// ... 100 more vendors
}
```

**Solution**: Capability-based abstraction with canonical events

### 3.2 Capability Abstraction

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        CAPABILITY ARCHITECTURE                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  CORE ENGINE                              CONNECTOR LAYER               │
│  ───────────                              ───────────────               │
│                                                                         │
│  ┌─────────────────┐                     ┌─────────────────────┐       │
│  │                 │                     │  Email Capability   │       │
│  │   Canonical     │◀────────────────────│  ├─ Gmail          │       │
│  │   Event         │   EmailMessageEvent │  ├─ Outlook        │       │
│  │   Processor     │                     │  ├─ Yahoo          │       │
│  │                 │                     │  └─ ProtonMail     │       │
│  │                 │                     └─────────────────────┘       │
│  │                 │                                                    │
│  │                 │                     ┌─────────────────────┐       │
│  │                 │◀────────────────────│  Finance Capability │       │
│  │                 │   TransactionEvent  │  ├─ Plaid (US)     │       │
│  │                 │   BalanceEvent      │  ├─ TrueLayer (UK) │       │
│  │                 │                     │  ├─ Setu (India)   │       │
│  │                 │                     │  └─ Direct Banks   │       │
│  │                 │                     └─────────────────────┘       │
│  │                 │                                                    │
│  │                 │                     ┌─────────────────────┐       │
│  │                 │◀────────────────────│  Calendar Capability│       │
│  │                 │   CalendarEvent     │  ├─ Google         │       │
│  │                 │                     │  ├─ Apple          │       │
│  │                 │                     │  ├─ Outlook        │       │
│  │                 │                     │  └─ CalDAV         │       │
│  └─────────────────┘                     └─────────────────────┘       │
│                                                                         │
│  Core knows ONLY canonical events.       Connectors transform vendor   │
│  Zero vendor-specific code.              formats to canonical events.  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.3 Capability Interface

```go
// Capability defines what a connector can do, not which vendor it connects to.
type Capability interface {
    // Identity
    CapabilityID() CapabilityID    // e.g., "email.read", "finance.balance"

    // What this capability can read
    CanRead() []CanonicalEventType

    // What this capability can write (if any)
    CanWrite() []CanonicalActionType

    // Health check
    HealthCheck(ctx context.Context) error
}

// Connector implements one or more capabilities for a specific vendor.
type Connector interface {
    // Identity
    ConnectorID() ConnectorID      // e.g., "gmail", "truelayer-uk"
    VendorID() VendorID            // e.g., "google", "truelayer"
    Region() Region                // e.g., "UK", "IN", "US"

    // Capabilities this connector provides
    Capabilities() []CapabilityID

    // Lifecycle
    Connect(ctx context.Context, creds Credentials) error
    Disconnect(ctx context.Context) error
}

// ReadCapability can ingest data and produce canonical events.
type ReadCapability interface {
    Capability

    // Poll for new data, emit canonical events
    Poll(ctx context.Context, since time.Time) ([]CanonicalEvent, error)

    // Subscribe to real-time updates (if supported)
    Subscribe(ctx context.Context, handler EventHandler) error
}

// WriteCapability can execute actions.
type WriteCapability interface {
    Capability

    // Execute a canonical action
    Execute(ctx context.Context, action CanonicalAction) (ExecutionResult, error)
}
```

### 3.4 Capability Registry

```yaml
capabilities:
  # Email capabilities
  email.read:
    description: "Read email messages"
    canonical_events: [EmailMessageEvent]
    implementations:
      - connector: gmail
        vendor: google
        regions: [GLOBAL]
      - connector: outlook
        vendor: microsoft
        regions: [GLOBAL]
      - connector: protonmail
        vendor: proton
        regions: [GLOBAL]

  email.send:
    description: "Send email messages"
    canonical_actions: [SendEmailAction]
    requires_approval: true
    implementations:
      - connector: gmail
        vendor: google
      - connector: outlook
        vendor: microsoft

  # Finance capabilities
  finance.balance:
    description: "Read account balances"
    canonical_events: [BalanceEvent]
    implementations:
      - connector: truelayer-uk
        vendor: truelayer
        regions: [UK]
      - connector: plaid-us
        vendor: plaid
        regions: [US]
      - connector: setu-in
        vendor: setu
        regions: [IN]

  finance.transactions:
    description: "Read transaction history"
    canonical_events: [TransactionEvent]
    implementations:
      - connector: truelayer-uk
        vendor: truelayer
        regions: [UK]
      - connector: plaid-us
        vendor: plaid
        regions: [US]

  finance.payment:
    description: "Initiate payments"
    canonical_actions: [PaymentAction]
    requires_approval: true
    v9_enforcement: FULL  # Policy binding, view binding, forced pause
    implementations:
      - connector: truelayer-uk
        vendor: truelayer
        regions: [UK]

  # Calendar capabilities
  calendar.read:
    description: "Read calendar events"
    canonical_events: [CalendarEventEvent]
    implementations:
      - connector: google-calendar
        vendor: google
        regions: [GLOBAL]
      - connector: outlook-calendar
        vendor: microsoft
        regions: [GLOBAL]
      - connector: apple-calendar
        vendor: apple
        regions: [GLOBAL]

  calendar.write:
    description: "Create/modify calendar events"
    canonical_actions: [CreateEventAction, UpdateEventAction, DeleteEventAction]
    requires_approval: true
    implementations:
      - connector: google-calendar
        vendor: google
      - connector: outlook-calendar
        vendor: microsoft

  # Messaging capabilities
  messaging.read:
    description: "Read messages"
    canonical_events: [MessageEvent]
    implementations:
      - connector: whatsapp-business
        vendor: meta
        regions: [GLOBAL]
      - connector: slack
        vendor: slack
        regions: [GLOBAL]

  # Health capabilities
  health.activity:
    description: "Read activity data"
    canonical_events: [ActivityEvent, WorkoutEvent]
    implementations:
      - connector: apple-health
        vendor: apple
        regions: [GLOBAL]
      - connector: google-fit
        vendor: google
        regions: [GLOBAL]

  health.sleep:
    description: "Read sleep data"
    canonical_events: [SleepEvent]
    implementations:
      - connector: apple-health
        vendor: apple
      - connector: oura
        vendor: oura
```

---

## 4. Data Flow Architecture

### 4.1 Ingestion Flow (Background - Outside Core)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         INGESTION FLOW                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. POLL/WEBHOOK                                                        │
│     ┌──────────────┐                                                   │
│     │  Scheduler   │  "Poll Gmail every 5 minutes"                     │
│     │  (cron)      │  "Poll TrueLayer every 15 minutes"                │
│     └──────┬───────┘                                                   │
│            │                                                            │
│            ▼                                                            │
│  2. CONNECTOR INVOCATION                                                │
│     ┌──────────────┐     ┌──────────────┐                              │
│     │ Gmail        │     │ TrueLayer    │                              │
│     │ Connector    │     │ Connector    │   (Vendor-specific code)     │
│     └──────┬───────┘     └──────┬───────┘                              │
│            │                    │                                       │
│            │  Raw email         │  Raw transactions                     │
│            │  (Gmail format)    │  (TrueLayer format)                   │
│            │                    │                                       │
│            ▼                    ▼                                       │
│  3. TRANSFORMATION                                                      │
│     ┌──────────────┐     ┌──────────────┐                              │
│     │ Gmail        │     │ TrueLayer    │                              │
│     │ Transformer  │     │ Transformer  │   (Vendor → Canonical)       │
│     └──────┬───────┘     └──────┬───────┘                              │
│            │                    │                                       │
│            │  EmailMessageEvent │  TransactionEvent                     │
│            │  (canonical)       │  (canonical)                          │
│            │                    │                                       │
│            └────────┬───────────┘                                       │
│                     │                                                   │
│                     ▼                                                   │
│  4. ENQUEUE                                                             │
│     ┌──────────────────────────────────────┐                           │
│     │         Canonical Event Queue        │                           │
│     │  (Redis Stream / Postgres Queue)     │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Core Processing Flow (Synchronous - v9.7 Compliant)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    CORE PROCESSING FLOW (Synchronous)                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. DEQUEUE (Synchronous read, blocks until event available)            │
│     ┌──────────────────────────────────────┐                           │
│     │         Canonical Event Queue        │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  CanonicalEvent                                │
│                        ▼                                                │
│  2. CLASSIFY                                                            │
│     ┌──────────────────────────────────────┐                           │
│     │           Event Classifier           │                           │
│     │  - Assign to Circle(s)               │                           │
│     │  - Extract entities                  │                           │
│     │  - Detect obligations                │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  ClassifiedEvent                               │
│                        ▼                                                │
│  3. MODEL                                                               │
│     ┌──────────────────────────────────────┐                           │
│     │           World Model Update         │                           │
│     │  - Update entity graph               │                           │
│     │  - Update obligation registry        │                           │
│     │  - Update circle states              │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  WorldStateΔ                                   │
│                        ▼                                                │
│  4. DECIDE                                                              │
│     ┌──────────────────────────────────────┐                           │
│     │         Interruption Decider         │                           │
│     │  - Calculate regret scores           │                           │
│     │  - Apply thresholds                  │                           │
│     │  - Respect rate limits               │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  InterruptionDecision                          │
│                        ▼                                                │
│  5. PROPOSE (if action needed)                                          │
│     ┌──────────────────────────────────────┐                           │
│     │         Proposal Generator           │                           │
│     │  - Create ExecutionEnvelope          │                           │
│     │  - Bind PolicySnapshotHash (v9.12)   │                           │
│     │  - Bind ViewSnapshotHash (v9.13)     │                           │
│     │  - Store in Proposal Store           │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
│  6. AUDIT (always)                                                      │
│     ┌──────────────────────────────────────┐                           │
│     │           Audit Logger               │                           │
│     │  - Log classification decision       │                           │
│     │  - Log interruption decision         │                           │
│     │  - Log proposal (if created)         │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
│  Loop back to step 1 (synchronous event loop)                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Execution Flow (User-Triggered - v9.7 Compliant)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    EXECUTION FLOW (User-Triggered)                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. USER APPROVAL                                                       │
│     ┌──────────────────────────────────────┐                           │
│     │         Mobile/Web UI                │                           │
│     │   User taps "Approve" on proposal    │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  HTTP POST /execute/{proposal_id}              │
│                        ▼                                                │
│  2. VALIDATION                                                          │
│     ┌──────────────────────────────────────┐                           │
│     │       Execution Coordinator          │                           │
│     │  - Verify PolicySnapshotHash (v9.12) │                           │
│     │  - Verify ViewSnapshotHash (v9.13)   │                           │
│     │  - Verify approvals meet threshold   │                           │
│     │  - Check idempotency key (v9.6)      │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  If validation fails → REJECT (no retry)       │
│                        ▼                                                │
│  3. FORCED PAUSE (v9+)                                                  │
│     ┌──────────────────────────────────────┐                           │
│     │         Forced Pause                 │                           │
│     │  - Wait configured duration          │                           │
│     │  - Cannot be bypassed                │                           │
│     │  - Audit: pause_started              │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        ▼                                                │
│  4. EXECUTE                                                             │
│     ┌──────────────────────────────────────┐                           │
│     │         Write Provider               │                           │
│     │  - Call external API                 │                           │
│     │  - Single attempt (no retry v9.8)    │                           │
│     │  - Capture response                  │                           │
│     └──────────────────┬───────────────────┘                           │
│                        │                                                │
│                        │  Success OR Failure (terminal)                 │
│                        ▼                                                │
│  5. FINALIZE                                                            │
│     ┌──────────────────────────────────────┐                           │
│     │         Single Trace Finalization    │                           │
│     │  - Mark execution complete           │                           │
│     │  - Record terminal state             │                           │
│     │  - Audit: execution_completed        │                           │
│     │  - v9.8: Exactly one terminal state  │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
│  Response returned synchronously to user                                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Storage Architecture

### 5.1 Data Stores

| Store | Technology | Purpose | Retention |
|-------|------------|---------|-----------|
| **Event Store** | Postgres + TimescaleDB | Canonical events, immutable | 7 years |
| **World Model** | Postgres | Entity graph, obligations, circle state | Current + 90 days history |
| **Proposal Store** | Postgres | Pending proposals awaiting approval | Until executed/expired |
| **Audit Log** | Postgres + S3 | All decisions, executions | 7 years |
| **Event Queue** | Redis Streams | Ingestion → Core communication | 24 hours |
| **Cache** | Redis | View snapshots, session data | TTL-based |
| **Secrets** | Vault / AWS Secrets Manager | Credentials, API keys | N/A |

### 5.2 Data Partitioning

```yaml
partitioning:
  # By user (Satish is user 1, but designed for multi-tenant)
  user_partitioned:
    - canonical_events
    - world_model
    - proposals
    - audit_log

  # By time (for efficient querying)
  time_partitioned:
    - canonical_events (by month)
    - audit_log (by month)

  # By region (for data residency)
  region_partitioned:
    - credentials (UK data in UK, India data in India)
```

---

## 6. Security Architecture

### 6.1 Authentication & Authorization

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      SECURITY ARCHITECTURE                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  USER AUTHENTICATION                                                    │
│  ├── Mobile app: Biometric + device key                                │
│  ├── Web app: Passkey (WebAuthn)                                       │
│  └── API: Short-lived JWT + refresh token                              │
│                                                                         │
│  CONNECTOR AUTHENTICATION                                               │
│  ├── OAuth 2.0 tokens stored encrypted                                 │
│  ├── Token refresh handled by ingestion layer                          │
│  └── Credential rotation supported                                      │
│                                                                         │
│  AUTHORIZATION                                                          │
│  ├── User can only access own data                                     │
│  ├── Multi-party approvals enforced (intersection policies)            │
│  └── Execution requires fresh approval (not cached)                    │
│                                                                         │
│  DATA ENCRYPTION                                                        │
│  ├── At rest: AES-256 (database-level)                                 │
│  ├── In transit: TLS 1.3                                               │
│  ├── Sensitive fields: Application-level encryption                    │
│  └── Secrets: Vault with HSM backend                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 6.2 v9+ Security Enforcement Points

| Checkpoint | Location | Enforcement |
|------------|----------|-------------|
| Policy Binding | Proposal creation | PolicySnapshotHash computed and stored |
| View Binding | Proposal creation | ViewSnapshotHash computed and stored |
| Policy Verification | Execution start | Hash match required |
| View Freshness | Execution start | Staleness check required |
| Approval Verification | Execution start | Required approvers signed |
| Idempotency | Execution start | Duplicate request rejected |
| Forced Pause | Pre-execution | Mandatory wait period |
| Single Finalization | Execution end | Only one terminal state |
| Audit | Every step | Immutable log entry |

---

## 7. Deployment Architecture

### 7.1 Container Topology

```yaml
services:
  # Ingestion layer (background OK)
  ingestion-email:
    image: quantumlife/ingestion-email
    replicas: 2
    resources:
      cpu: "500m"
      memory: "512Mi"

  ingestion-finance:
    image: quantumlife/ingestion-finance
    replicas: 1
    resources:
      cpu: "250m"
      memory: "256Mi"

  ingestion-calendar:
    image: quantumlife/ingestion-calendar
    replicas: 1
    resources:
      cpu: "250m"
      memory: "256Mi"

  # Core engine (no background)
  core-engine:
    image: quantumlife/core-engine
    replicas: 1  # Single instance for determinism
    resources:
      cpu: "1000m"
      memory: "1Gi"
    environment:
      - GOMAXPROCS=1  # Single-threaded for determinism

  # API gateway
  api-gateway:
    image: quantumlife/api-gateway
    replicas: 2
    resources:
      cpu: "500m"
      memory: "512Mi"

  # Execution coordinator
  execution-coordinator:
    image: quantumlife/execution-coordinator
    replicas: 1  # Single instance for safety
    resources:
      cpu: "500m"
      memory: "512Mi"
```

### 7.2 Infrastructure

```yaml
infrastructure:
  cloud: AWS (primary), with regional considerations

  regions:
    uk:
      primary: eu-west-2 (London)
      services: [all]
    india:
      primary: ap-south-1 (Mumbai)
      services: [ingestion-india, credentials-india]  # Data residency

  databases:
    postgres:
      instance: db.r6g.large
      storage: 100GB gp3
      multi_az: true
    redis:
      instance: cache.r6g.large
      cluster_mode: false

  observability:
    metrics: Prometheus + Grafana
    logs: Loki
    traces: Jaeger
    alerts: PagerDuty
```

---

## 8. Operational Architecture

### 8.1 Monitoring & Alerting

```yaml
monitoring:
  # Core engine health
  core_engine:
    - metric: event_processing_latency_p99
      threshold: 100ms
      alert: warning
    - metric: event_queue_depth
      threshold: 1000
      alert: critical
    - metric: proposal_creation_rate
      baseline: true
      deviation_alert: 2x

  # Execution health
  execution:
    - metric: execution_success_rate
      threshold: 99%
      alert: critical
    - metric: forced_pause_skipped
      threshold: 0
      alert: critical  # v9 violation!
    - metric: retry_attempted
      threshold: 0
      alert: critical  # v9.8 violation!

  # Ingestion health
  ingestion:
    - metric: connector_error_rate
      threshold: 5%
      alert: warning
    - metric: oauth_token_expiry
      threshold: 24h
      alert: warning
```

### 8.2 Disaster Recovery

| Scenario | RTO | RPO | Strategy |
|----------|-----|-----|----------|
| Database failure | 5 min | 0 | Multi-AZ failover |
| Region failure | 1 hour | 5 min | Cross-region replica |
| Data corruption | 4 hours | 1 hour | Point-in-time recovery |
| Credential leak | Immediate | N/A | Rotation + revocation |

---

## 9. Package Structure

```
quantumlife/
├── cmd/
│   ├── core-engine/           # Main core process
│   ├── ingestion-email/       # Email polling workers
│   ├── ingestion-finance/     # Finance polling workers
│   ├── ingestion-calendar/    # Calendar sync workers
│   ├── api-gateway/           # HTTP API server
│   └── execution-coordinator/ # Execution handler
│
├── internal/
│   ├── core/                  # v9.7 COMPLIANT (no goroutines)
│   │   ├── classifier/        # Event classification
│   │   ├── modeler/           # World model updates
│   │   ├── decider/           # Interruption decisions
│   │   └── proposer/          # Proposal generation
│   │
│   ├── finance/
│   │   ├── execution/         # v9.7 COMPLIANT
│   │   ├── visibility/        # View generation
│   │   └── caps/              # Spending caps
│   │
│   ├── interruption/          # v9.7 COMPLIANT
│   │   ├── regret/            # Regret scoring
│   │   ├── threshold/         # Threshold checking
│   │   └── ratelimit/         # Rate limiting
│   │
│   └── obligation/            # v9.7 COMPLIANT
│       ├── registry/          # Obligation tracking
│       └── deadline/          # Deadline monitoring
│
├── pkg/
│   ├── primitives/            # v9.7 COMPLIANT
│   │   ├── finance/           # Financial primitives
│   │   └── identity/          # Identity primitives
│   │
│   ├── events/                # v9.7 COMPLIANT
│   │   └── canonical/         # Canonical event definitions
│   │
│   └── capabilities/          # Capability interfaces
│       ├── email/
│       ├── finance/
│       ├── calendar/
│       └── messaging/
│
├── connectors/                # OUTSIDE CORE (background OK)
│   ├── gmail/
│   ├── outlook/
│   ├── truelayer/
│   ├── plaid/
│   ├── google-calendar/
│   └── apple-health/
│
└── infrastructure/            # OUTSIDE CORE (background OK)
    ├── database/
    ├── queue/
    ├── cache/
    └── secrets/
```

---

## 10. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
