# Control Plane vs Data Plane

Separation of policy/configuration flow (Control Plane) from operational data flow (Data Plane).

## Plane Separation Diagram

```mermaid
flowchart TB
    subgraph "Control Plane (Policy & Configuration)"
        direction TB

        subgraph "Policy Definition"
            RuleDefs["Rule Definitions"]
            Thresholds["Threshold Configs"]
            CirclePolicies["Circle Policies"]
        end

        subgraph "Registry Management"
            ProviderReg["Provider Registry<br/>(v9.9)"]
            PayeeReg["Payee Registry<br/>(v9.10)"]
            ScopeAllowlist["Scope Allowlist"]
        end

        subgraph "Approval Configuration"
            ApprovalPolicy["Approval Policies"]
            ThresholdConfig["Threshold Configs<br/>(e.g., 2-of-3)"]
            ApproverList["Approver Lists"]
        end

        subgraph "Limit Configuration"
            CapsConfig["Caps Configuration<br/>(v9.11)"]
            RateLimits["Rate Limit Rules"]
            CooldownPeriods["Cooldown Periods"]
        end

        subgraph "Crypto Configuration"
            KeyRotation["Key Rotation Policy"]
            AlgorithmConfig["Algorithm Selection"]
            PQCConfig["PQC Migration Config"]
        end
    end

    subgraph "Data Plane (Operational Flow)"
        direction TB

        subgraph "Ingestion"
            RawData["Raw Provider Data"]
            CanonicalEvents["Canonical Events"]
            StoredEvents["Stored Events"]
        end

        subgraph "View Computation"
            EventStream["Event Stream"]
            ViewSnapshots["View Snapshots"]
            ViewHashes["View Hashes<br/>(v9.12 binding)"]
        end

        subgraph "Decision Execution"
            RuleEval["Rule Evaluation"]
            ObsGen["Observation Generation"]
            ProposalGen["Proposal Generation"]
        end

        subgraph "Action Execution"
            ApprovalArtifacts["Approval Artifacts"]
            IdempotencyKeys["Idempotency Keys<br/>(v9.6)"]
            ExecutionTraces["Execution Traces"]
        end

        subgraph "Audit Trail"
            AuditEvents["Audit Events"]
            TraceRecords["Trace Records"]
            ImmutableLog["Immutable Log"]
        end
    end

    %% Control plane → Data plane influence
    RuleDefs -->|"defines"| RuleEval
    Thresholds -->|"configures"| RuleEval
    CirclePolicies -->|"scopes"| CanonicalEvents

    ProviderReg -->|"validates"| ExecutionTraces
    PayeeReg -->|"validates"| ExecutionTraces
    ScopeAllowlist -->|"restricts"| RawData

    ApprovalPolicy -->|"gates"| ApprovalArtifacts
    ThresholdConfig -->|"requires"| ApprovalArtifacts
    ApproverList -->|"authorizes"| ApprovalArtifacts

    CapsConfig -->|"limits"| ExecutionTraces
    RateLimits -->|"throttles"| ExecutionTraces
    CooldownPeriods -->|"delays"| ProposalGen

    KeyRotation -->|"rotates"| AuditEvents
    AlgorithmConfig -->|"signs"| ApprovalArtifacts

    %% Data plane flow
    RawData --> CanonicalEvents
    CanonicalEvents --> StoredEvents
    StoredEvents --> EventStream
    EventStream --> ViewSnapshots
    ViewSnapshots --> ViewHashes
    ViewHashes --> RuleEval
    RuleEval --> ObsGen
    ObsGen --> ProposalGen
    ProposalGen --> ApprovalArtifacts
    ApprovalArtifacts --> IdempotencyKeys
    IdempotencyKeys --> ExecutionTraces
    ExecutionTraces --> AuditEvents
    AuditEvents --> TraceRecords
    TraceRecords --> ImmutableLog

    %% Styling
    classDef control fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
    classDef data fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px
    classDef binding fill:#fff3e0,stroke:#ef6c00,stroke-width:2px

    class RuleDefs,Thresholds,CirclePolicies,ProviderReg,PayeeReg,ScopeAllowlist,ApprovalPolicy,ThresholdConfig,ApproverList,CapsConfig,RateLimits,CooldownPeriods,KeyRotation,AlgorithmConfig,PQCConfig control
    class RawData,CanonicalEvents,StoredEvents,EventStream,ViewSnapshots,RuleEval,ObsGen,ProposalGen,ApprovalArtifacts,IdempotencyKeys,ExecutionTraces,AuditEvents,TraceRecords,ImmutableLog data
    class ViewHashes binding
```

## Plane Characteristics

### Control Plane

**Purpose**: Define policies, configurations, and constraints

**Properties**:
| Property | Description |
|----------|-------------|
| Infrequent changes | Configuration changes are rare and deliberate |
| High privilege | Changes require elevated permissions |
| Audited | All changes logged to audit trail |
| Versioned | Configurations have version history |

**Components**:

#### Policy Definition
```go
// Rule definitions loaded at startup
type RuleConfig struct {
    ID          string
    CircleID    string
    Condition   string  // Deterministic predicate
    Observation string  // Template for observation text
    Priority    int
}
```

#### Registry Management (v9.9/v9.10)
```go
// Provider registry - control plane configuration
type ProviderEntry struct {
    ID           string
    Name         string
    Capabilities []string
    RegisteredAt time.Time
    RegisteredBy string  // Approver who registered
}

// Payee registry - control plane configuration
type PayeeEntry struct {
    ID           string
    Name         string
    AccountHash  string  // Hashed account details
    RegisteredAt time.Time
    RegisteredBy string
}
```

#### Caps Configuration (v9.11)
```go
// Spending caps - control plane configuration
type CapsConfig struct {
    DailyLimitCents   int64
    WeeklyLimitCents  int64
    MonthlyLimitCents int64
    PerTxLimitCents   int64
    CooldownMinutes   int
}
```

### Data Plane

**Purpose**: Process operational data through the system

**Properties**:
| Property | Description |
|----------|-------------|
| High volume | Continuous stream of events |
| Constrained by control | Policies limit what data can do |
| Audited | All operations logged |
| Ephemeral (mostly) | Views recomputable from events |

**Components**:

#### Ingestion
```go
// Canonical event - data plane artifact
type EmailEvent struct {
    EventID     string
    CircleID    string
    Provider    string
    AccountID   string
    MessageID   string
    Subject     string
    From        EmailAddress
    SentAt      time.Time
    CapturedAt  time.Time  // Deterministic clock
}
```

#### View Computation (v9.12/v9.13)
```go
// View snapshot with hash binding
type ViewSnapshot struct {
    CircleID    string
    ComputedAt  time.Time
    Hash        string  // SHA-256 of canonical content
    Content     ViewContent
}

// Proposal binds to view hash
type Proposal struct {
    ID           string
    ViewHash     string  // v9.12 binding
    ViewFreshness time.Time  // v9.13 binding
    Action       ActionSpec
}
```

#### Execution Trace
```go
// Execution trace - data plane artifact
type ExecutionTrace struct {
    TraceID        string
    ActionID       string
    IdempotencyKey string  // v9.6
    ProviderID     string  // Validated against registry
    PayeeID        string  // Validated against registry
    ApprovalIDs    []string
    Result         ExecutionResult
    AuditEvents    []AuditEvent
}
```

## Control Plane → Data Plane Binding

### Policy Snapshot Hash Binding (v9.12)

```mermaid
sequenceDiagram
    participant Policy as Control Plane
    participant View as View Materializer
    participant Proposal as Proposal Generator
    participant Exec as Executor

    Policy->>View: CapsConfig{DailyLimit: 10000}
    View->>View: Compute snapshot
    View->>View: Hash = SHA256(canonicalize(snapshot))
    View-->>Proposal: ViewSnapshot{Hash: "abc123"}

    Proposal->>Proposal: Create proposal
    Proposal->>Proposal: Bind to ViewHash: "abc123"

    Note over Exec: At execution time
    Exec->>Exec: Verify ViewHash matches current
    Exec->>Exec: If mismatch: reject (stale view)
```

### Scope Allowlist Enforcement

```mermaid
flowchart LR
    subgraph "Control Plane"
        Allowlist["Scope Allowlist<br/>calendar:read ✓<br/>gmail:read ✓<br/>payment:write ✗"]
    end

    subgraph "Data Plane"
        TokenReq["Token Request<br/>scopes: [gmail:read]"]
        TokenMint["Token Mint"]
        DataFetch["Data Fetch"]
    end

    Allowlist -->|"validates"| TokenReq
    TokenReq -->|"allowed"| TokenMint
    TokenMint --> DataFetch

    style Allowlist fill:#e3f2fd,stroke:#1565c0
    style TokenReq fill:#e8f5e9,stroke:#2e7d32
```

## Separation Benefits

| Benefit | Description |
|---------|-------------|
| **Auditability** | Policy changes are infrequent and logged separately from high-volume data |
| **Security** | Elevated privileges for control plane, standard privileges for data plane |
| **Rollback** | Policy configurations can be versioned and rolled back |
| **Testing** | Data plane can be tested with different control plane configs |
| **Compliance** | Clear separation for regulatory requirements |

## Anti-Patterns (What NOT to Do)

### 1. Policy in Data
```go
// BAD: Embedding policy decisions in data processing
func ProcessTransaction(tx Transaction) {
    if tx.Amount > 10000 {  // Magic number in code
        // ...
    }
}

// GOOD: Policy from control plane
func ProcessTransaction(tx Transaction, caps CapsConfig) {
    if tx.Amount > caps.PerTxLimitCents {
        // ...
    }
}
```

### 2. Data Affecting Policy
```go
// BAD: Data modifying policy at runtime
func ProcessTransaction(tx Transaction) {
    if tx.IsLarge() {
        caps.PerTxLimitCents = tx.Amount + 1  // Modifying policy!
    }
}

// GOOD: Policy is immutable during data processing
func ProcessTransaction(tx Transaction, caps CapsConfig) {
    // caps is read-only, cannot be modified
    if tx.Amount > caps.PerTxLimitCents {
        return ErrExceedsCap
    }
}
```

### 3. Missing Binding
```go
// BAD: Proposal without view binding
type Proposal struct {
    Action ActionSpec
    // No ViewHash - can execute against stale view!
}

// GOOD: Proposal bound to view hash (v9.12)
type Proposal struct {
    Action    ActionSpec
    ViewHash  string     // Must match current view
    ViewTime  time.Time  // v9.13 freshness
}
```

## Configuration Flow Example

```mermaid
sequenceDiagram
    participant Admin as Administrator
    participant CLI as quantumlife-cli
    participant Control as Control Plane
    participant Data as Data Plane
    participant Audit as Audit Log

    Admin->>CLI: register-provider plaid
    CLI->>Control: AddProvider(plaid)
    Control->>Audit: LogConfigChange(add_provider, plaid)
    Control-->>CLI: OK

    Note over Data: Later, during execution
    Data->>Control: IsRegistered(plaid)?
    Control-->>Data: true
    Data->>Data: Proceed with execution
```

## Related

- [ARCH_BLOCK_L0.md](ARCH_BLOCK_L0.md) - Component overview
- [TRUST_BOUNDARIES.md](TRUST_BOUNDARIES.md) - Security zones
- [CLOSED_LOOP_LIFECYCLE.md](CLOSED_LOOP_LIFECYCLE.md) - Lifecycle stages
