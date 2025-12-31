# Level 0 Architecture Block Diagram

System-level view of QuantumLife Canon showing all major components and guardrail boundaries.

## System Block Diagram

```mermaid
graph TB
    subgraph "External World"
        Gmail["Gmail API"]
        GCal["Google Calendar API"]
        Plaid["Plaid API"]
        Banks["Bank APIs"]
    end

    subgraph "Worker Boundary [WORKER]"
        Scheduler["Scheduler<br/>(cron/systemd)"]
        IngestCmd["quantumlife-ingest<br/>(runs once, exits)"]
    end

    subgraph "Canon Core Boundary [CORE]"
        subgraph "Sense Layer"
            Adapters["Read-Only Adapters<br/>gmail_read | gcal_read | finance_read"]
            Normalizer["Event Normalizer"]
        end

        subgraph "Identity Layer"
            IdResolver["Identity Resolver"]
            CircleMgr["Circle Manager"]
        end

        subgraph "Store Layer"
            EventStore["Event Store<br/>(append-only)"]
            ViewStore["View Store<br/>(snapshots)"]
            AuditLog["Audit Log<br/>(immutable)"]
        end

        subgraph "Model Layer"
            ViewMat["View Materializer"]
            MemoryRetrieval["Memory Retrieval<br/>(future: RAG)"]
        end

        subgraph "Decide Layer"
            RuleEngine["Rule Engine<br/>(deterministic)"]
            Observations["Observation Generator"]
        end

        subgraph "Propose Layer"
            ProposalGen["Proposal Generator"]
            ActionQueue["Action Queue"]
        end

        subgraph "Approve Layer [GATE]"
            MultiParty["Multi-Party Gate"]
            ApprovalStore["Approval Store"]
        end

        subgraph "Execute Layer"
            Executor["Executor<br/>[IDEMPOTENT]"]
            ProviderReg["Provider Registry<br/>[REGISTRY]"]
            PayeeReg["Payee Registry<br/>[REGISTRY]"]
            Caps["Caps & Rate Limits"]
        end
    end

    subgraph "Credential Vault"
        TokenBroker["Token Broker"]
        EncryptedTokens["Encrypted Tokens<br/>(AES-256-GCM)"]
    end

    subgraph "User Interface"
        CLI["quantumlife-cli"]
        TUI["TUI Dashboard<br/>(future)"]
    end

    %% External connections
    Gmail --> Adapters
    GCal --> Adapters
    Plaid --> Adapters
    Banks --> Adapters

    %% Worker flow
    Scheduler -->|"triggers"| IngestCmd
    IngestCmd -->|"calls"| Adapters

    %% Sense flow
    Adapters -->|"raw data"| Normalizer
    Normalizer -->|"canonical events"| IdResolver

    %% Identity flow
    IdResolver --> CircleMgr
    CircleMgr -->|"circle-tagged events"| EventStore

    %% Store flow
    EventStore -->|"events"| ViewMat
    ViewMat -->|"snapshots"| ViewStore

    %% Model flow
    ViewStore -->|"views"| RuleEngine
    MemoryRetrieval -.->|"context"| RuleEngine

    %% Decide flow
    RuleEngine -->|"decisions"| Observations
    Observations -->|"observations"| ProposalGen

    %% Propose flow
    ProposalGen -->|"proposals"| ActionQueue

    %% Approve flow
    ActionQueue -->|"pending actions"| MultiParty
    MultiParty -->|"approvals"| ApprovalStore
    ApprovalStore -->|"approved actions"| Executor

    %% Execute flow
    Executor -->|"validate"| ProviderReg
    Executor -->|"validate"| PayeeReg
    Executor -->|"check"| Caps
    Executor -->|"audit"| AuditLog

    %% Credential flow
    Adapters -->|"mint token"| TokenBroker
    TokenBroker --> EncryptedTokens

    %% User interface
    CLI -->|"commands"| CircleMgr
    CLI -->|"approve"| MultiParty
    CLI -->|"view"| ViewStore
    TUI -.->|"display"| ViewStore

    %% Styling
    classDef core fill:#e1f5fe,stroke:#01579b
    classDef worker fill:#fff3e0,stroke:#e65100
    classDef vault fill:#fce4ec,stroke:#880e4f
    classDef external fill:#f5f5f5,stroke:#616161
    classDef gate fill:#ffecb3,stroke:#ff6f00

    class Adapters,Normalizer,IdResolver,CircleMgr,EventStore,ViewStore,AuditLog,ViewMat,MemoryRetrieval,RuleEngine,Observations,ProposalGen,ActionQueue,Executor,ProviderReg,PayeeReg,Caps core
    class Scheduler,IngestCmd worker
    class TokenBroker,EncryptedTokens vault
    class Gmail,GCal,Plaid,Banks external
    class MultiParty,ApprovalStore gate
```

## Component Descriptions

### Canon Core Boundary [CORE]

All components within this boundary follow Canon v9+ guardrails:

| Guardrail | Enforcement |
|-----------|-------------|
| No background execution | Single-run commands only, no goroutines in core |
| Deterministic clock | `clock.Clock` interface injected at entry points |
| Single trace finalization | One trace ID per request, finalized before return |
| No auto-retry | Failures surface immediately, caller decides retry |

### Worker Boundary [WORKER]

External schedulers (cron, systemd timers) trigger ingestion:

```bash
# Example: Run ingestion every 15 minutes
*/15 * * * * /usr/local/bin/quantumlife-ingest --mode=real --circle=family
```

### Multi-Party Gate [GATE]

Financial execution requires multi-party approval:

- Threshold-based (e.g., 2-of-3 approvers)
- Action hash binding prevents tampering
- Approval artifacts are signed and stored

### Registry Locks [REGISTRY]

- **Provider Registry**: Only registered write providers can execute
- **Payee Registry**: Only registered payees can receive funds

### Idempotency [IDEMPOTENT]

Every execution carries an idempotency key:

- SHA-256 hash of (action_type, parameters, timestamp window)
- Duplicate detection before provider call
- Replay defense across restarts

## Data Flow Summary

```
External APIs → Adapters → Normalizer → Identity → EventStore → ViewMaterializer → ViewStore
                                                                       ↓
                                                                  RuleEngine
                                                                       ↓
                                                               ProposalGenerator
                                                                       ↓
                                                                MultiPartyGate
                                                                       ↓
                                                                  Executor → AuditLog
```

## Related

- [CLOSED_LOOP_LIFECYCLE.md](CLOSED_LOOP_LIFECYCLE.md) - Detailed lifecycle stages
- [TRUST_BOUNDARIES.md](TRUST_BOUNDARIES.md) - Security trust zones
- [CONTROL_DATA_PLANE.md](CONTROL_DATA_PLANE.md) - Plane separation
