# QuantumLife Technology Selection v1

**Product:** QuantumLife
**Company:** QuantumLayer Platform Ltd
**Version:** 1.0
**Status:** Technology Decision Document — LOCKED

---

## 1. Document Hierarchy & Scope

### Subordination

> **This document is subordinate to:**
> 1. QuantumLife Canon v1 (meaning)
> 2. Technical Split v1 (boundaries)
> 3. Human Guarantees v1 (trust contract)
>
> **Conflict resolution:** Canon wins. If any technology choice conflicts with canon invariants, technical boundaries, or human guarantees, the technology must adapt.

### What This Document Decides

- Runtime language and core frameworks
- Cloud platform and multi-tenancy model
- Data stores and encryption architecture
- LLM/SLM placement and routing policy
- Cryptographic posture and key management
- Capability packaging and certification
- Integration strategy and connector boundaries
- Deployment topology and observability

### What This Document Does NOT Decide

- UI framework (deferred; likely TypeScript/React)
- Specific mobile frameworks (deferred)
- Marketing/pricing tiers
- Detailed API contracts (separate spec)
- CI/CD tooling specifics

---

## 2. One-Page Decision Summary

| Runtime Layer | Technology | Responsibilities | Key Risks | Mitigations |
|---------------|------------|------------------|-----------|-------------|
| **Circle Runtime** | Go service, Azure Key Vault | Identity, policy, per-circle vault | Key compromise | HSM-backed KEK, rotation |
| **Intersection Runtime** | Go service, PostgreSQL | Contracts, versioning, multi-party | RLS bypass | CI RLS tests, audit |
| **Authority & Policy Engine** | Go service, PostgreSQL | Grant validation, ceiling checks | Logic errors | Property-based tests |
| **Negotiation Engine** | Go service, SLM + Azure OpenAI | Proposals, counterproposals, commitment | Hallucination | Confidence thresholds, explain capture |
| **Action Execution Layer** | Go service, Connectors | Deterministic execution | Connector bugs | Retry policies, idempotency |
| **Memory Layer** | PostgreSQL (auth) + Qdrant (assist) + Redis (cache) | State storage, semantic retrieval | Sync drift | Async index, rebuild capability |
| **Audit & Governance Layer** | Go service, PostgreSQL (append-only) | Hash chain, explainability | Tampering | Immutable tables, periodic anchoring |
| **Marketplace & Seal Layer** | Go service, OCI-like registry | Manifest validation, gating | Malicious capability | Certification process, revocation |

---

## 3. Layer-by-Layer Mapping

*Reference: TECHNICAL_SPLIT_V1.md §3*

### 3.1 Circle Runtime

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`circle-runtime`) |
| **Identity storage** | PostgreSQL `circles.identity` |
| **Key management** | Azure Key Vault (KEK per tenant), DEK per circle |
| **Per-circle vault concept** | Encrypted key material stored in PostgreSQL, wrapped by tenant KEK |
| **Agent activation** | Circle runtime spawns agent goroutine with context |

**Invariant enforcement:**
- Circle ID in context for all operations
- RLS on all circle tables
- No cross-circle access without intersection

### 3.2 Intersection Runtime

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`intersection-runtime`) |
| **Contract storage** | PostgreSQL `intersections.contracts` with version history |
| **Versioning** | Semantic versioning; every change creates new version row |
| **Multi-party coordination** | Server-mediated messaging; intersection-scoped channels |

**Invariant enforcement:**
- All parties must approve changes (consensus check)
- Version bump on any modification
- Dissolution preserves audit history

### 3.3 Authority & Policy Engine

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`authority-engine`) |
| **Grant storage** | PostgreSQL `intersections.authority_grants` |
| **Policy storage** | PostgreSQL `circles.policy` |
| **Enforcement** | Deterministic Go code; no model involvement |

**Invariant enforcement:**
- Grants checked before any action execution
- Expiry enforced at query time
- Revocation immediate (context cancellation)

### 3.4 Negotiation Engine

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`negotiation-engine`) |
| **SLM** | Server-side small model (Phi-3/Mistral class) for classification |
| **LLM** | Azure OpenAI (GPT-4 class) for complex reasoning |
| **Routing** | SLM-first with confidence thresholds (see §5) |
| **Explainability** | All model calls logged with reasoning trace |

**Invariant enforcement:**
- Models only in control plane
- Refusal is valid outcome (no forced cooperation)
- Explain capture for every decision

### 3.5 Action Execution Layer

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`action-executor`) |
| **Execution model** | Deterministic; no LLM/SLM |
| **Connector framework** | Pluggable connectors (Go interfaces) |
| **Retry policy** | Configurable per action type; idempotency keys |

**Invariant enforcement:**
- No decisions in executor
- Pause/abort via context cancellation
- All outcomes reported to audit

### 3.6 Memory Layer

| Aspect | Decision |
|--------|----------|
| **Authoritative store** | PostgreSQL (circles.memory, intersections.*) |
| **Assistive store** | Qdrant (semantic index, regenerable) |
| **Cache** | Redis (ephemeral, non-authoritative) |

**Invariant enforcement:**
- Qdrant never source of truth
- Redis evictable without data loss
- Audit logs NOT used as operational memory

### 3.7 Audit & Governance Layer

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`audit-service`) |
| **Storage** | PostgreSQL `audit.log` (append-only) |
| **Hash chain** | SHA-256 chaining; each entry hashes previous |
| **Export** | API returns full chain with verification |

**Invariant enforcement:**
- No UPDATE/DELETE on audit tables
- Periodic hash anchoring (optional external witness)
- Explanation stored with every action

### 3.8 Marketplace & Seal Validation Layer

| Aspect | Decision |
|--------|----------|
| **Service** | Go service (`seal-validator`) |
| **Registry** | OCI-like HTTPS registry (Azure Blob + CDN) |
| **Manifest format** | YAML with required fields (see ADR-0006) |
| **Signing** | Ed25519 (v1), PQ-hybrid ready |

**Invariant enforcement:**
- Signature verified on every capability load
- Uncertified requires explicit human approval
- Capabilities cannot self-authorize

---

## 4. Data Stores & Ownership Model

*Reference: diagrams/11_data_stores_and_ownership.mmd*

### 4.1 PostgreSQL Schema Organization

| Schema | Ownership | Contents |
|--------|-----------|----------|
| `circles` | Circle-owned | identity, policy, memory, keys |
| `intersections` | Intersection-owned | contracts, versions, grants, proposals, commitments, actions, settlements |
| `audit` | System (immutable) | log, hash_chain, explanations |
| `system` | Platform | tenant metadata, capability registry cache |

### 4.2 Row-Level Security Strategy

```sql
-- All tables have tenant_id column
-- RLS policy on every table:
CREATE POLICY tenant_rls ON <table>
  USING (tenant_id = current_setting('app.tenant_id')::uuid);

-- Circle tables additionally have circle_id RLS
CREATE POLICY circle_rls ON circles.<table>
  USING (circle_id = current_setting('app.circle_id')::uuid);
```

- Connection context set by service layer
- RLS enforced by PostgreSQL (defense in depth)
- All RLS policies tested in CI

### 4.3 Encryption Model

```
┌─────────────────────────────────────┐
│ Azure Key Vault                     │
│   └─ KEK per tenant (RSA-4096)      │
│        └─ PQ-hybrid version ready   │
└─────────────────────────────────────┘
              │
              ▼ wraps
┌─────────────────────────────────────┐
│ Per-Circle DEK (AES-256-GCM)        │
│   └─ Stored encrypted in PostgreSQL │
└─────────────────────────────────────┘
              │
              ▼ encrypts
┌─────────────────────────────────────┐
│ Circle data (memory, keys, etc.)    │
└─────────────────────────────────────┘
```

### 4.4 Backup, Restore & Export

| Operation | Mechanism |
|-----------|-----------|
| Backup | Azure PostgreSQL automated backups + PITR |
| Restore | Point-in-time restore for DR |
| Export (exit rights) | API extracts circle state + audit chain as signed archive |

### 4.5 Migration & Versioning

- Contract schema changes via versioned migrations (golang-migrate)
- Memory schema backward-compatible; old versions readable
- Qdrant re-indexed on embedding model change

---

## 5. LLM/SLM Placement & Routing Policy

*Reference: diagrams/12_llm_slm_placement.mmd, ADR-0005*

### 5.1 Control Plane Model Usage

| Irreducible Loop Step | Model Allowed | Model Role |
|-----------------------|---------------|------------|
| Intent | SLM | Classification, extraction |
| Intersection Discovery | SLM | Matching, similarity |
| Authority Negotiation | SLM → LLM | Proposal analysis, counterproposal |
| Commitment | None | Deterministic formation |
| Action | None | Deterministic execution |
| Settlement | None | Deterministic confirmation |
| Memory Update | SLM (embedding) | Vectorization for Qdrant |

### 5.2 Confidence Scoring

| SLM Confidence | Action |
|----------------|--------|
| ≥ 0.85 | Proceed (unless high-risk) |
| 0.70–0.85 | Proceed if low-risk; escalate if high-risk |
| < 0.70 | Escalate to LLM |

### 5.3 High-Risk Classification

| Category | Examples | Required |
|----------|----------|----------|
| Financial | Payment > $100, recurring commitment | LLM + human confirm |
| Legal | Authority grant, contract amendment | LLM + human confirm |
| Irreversible | Data deletion, intersection dissolution | LLM + human confirm |
| Ambiguous | Multiple valid interpretations | LLM |

### 5.4 Fallback Modes

| Scenario | Behavior |
|----------|----------|
| LLM unavailable | Suggest-only mode; no execution without human approval |
| SLM unavailable (server) | Route to LLM (higher cost, acceptable) |
| Both unavailable | Suggest-only with on-device SLM if available; else queue |

### 5.5 Prompt & Version Management

- Prompts stored as versioned templates in git
- Prompt changes require evaluation gate (test set accuracy)
- Model version pinned; upgrade is explicit migration

---

## 6. Identity & Crypto Posture

### 6.1 Versioned Keys

| Key Type | Algorithm (v1) | PQ-Ready |
|----------|----------------|----------|
| Tenant KEK | RSA-4096 | Hybrid slot for Kyber |
| Circle DEK | AES-256-GCM | Symmetric, algorithm-agile |
| Signing | Ed25519 | Hybrid slot for Dilithium |
| Invite tokens | Ed25519 signature | Hybrid slot for Dilithium |

### 6.2 Key Format

```json
{
  "key_id": "uuid",
  "algorithm": "Ed25519",
  "algorithm_version": 1,
  "public_key": "base64...",
  "created_at": "timestamp",
  "expires_at": "timestamp",
  "pq_extension": null  // future: Dilithium public key
}
```

### 6.3 Signed Invite Tokens

| Field | Purpose |
|-------|---------|
| `issuer_circle_id` | Who created the invite |
| `intersection_template` | Proposed terms |
| `scopes_offered` | What authority is offered |
| `expiry` | When token expires |
| `signature` | Ed25519 (or hybrid) signature |

Verification:
1. Check expiry
2. Verify signature against issuer's public key
3. Validate intersection template against canon

### 6.4 Key Rotation

- KEK: Annual rotation or on-demand
- DEK: Quarterly rotation; re-wrap, not re-encrypt
- Signing keys: On compromise or schedule

### 6.5 Recovery

- Recovery is intersection-governed (Human Guarantees §6)
- Quorum-based recovery (e.g., 2-of-3 trusted circles)
- Time-delay recovery (e.g., 72-hour wait with notification)
- No platform backdoors; QuantumLayer cannot recover without user-configured intersection

---

## 7. Capability Packaging: QuantumLife Seal

*Reference: ADR-0006*

### 7.1 Manifest Required Fields

```yaml
id: "reverse.domain.capability-name"
version: "semver"
name: "Human-readable name"
description: "What it does"
scopes_requested: ["list", "of", "scopes"]
risk_class: "standard|elevated|high"
audit_hooks:
  - on_action: "hook_name"
signature:
  algorithm: "Ed25519"
  signer: "QuantumLayer Platform Ltd"
  value: "base64..."
provenance:
  source_repo: "url"
  commit_hash: "hash"
```

### 7.2 Certification Flow

1. Developer submits manifest
2. Automated validation (schema, scope analysis)
3. Manual review for elevated/high risk
4. QuantumLayer signs manifest
5. Published to registry

### 7.3 Registry Structure

```
registry.quantumlife.io/
  capabilities/
    <id>/
      manifest.yaml
      manifest.yaml.sig
      versions/
        <version>/
```

### 7.4 Runtime Gating

| Seal Status | Behavior |
|-------------|----------|
| Certified | Agent can propose; circle must still grant authority |
| Uncertified | Explicit human approval required |
| Revoked | Cannot be used; existing grants void |

---

## 8. Integrations Strategy

### 8.1 Connector Abstraction

Connectors are data-plane components (deterministic, no models):

```go
type Connector interface {
    Execute(ctx context.Context, action Action) (Outcome, error)
    Capabilities() []string
    RequiredScopes() []string
}
```

### 8.2 V1 Must-Have Integrations

| Integration | Scope | Approach |
|-------------|-------|----------|
| **Google Calendar** | Read + Write events | OAuth2, calendar API |
| **Microsoft Calendar** | Read + Write events | MS Graph API |
| **Email (Gmail/Outlook)** | Read-only summary | OAuth2, read-only scope; summarize via SLM |
| **Finance (Plaid)** | Read-only transactions | Plaid Link; transaction insights, no transfers v1 |

### 8.3 Token Storage

- OAuth tokens encrypted with circle DEK
- Stored in PostgreSQL `circles.tokens`
- Scoped to intersections (token only usable for granted scopes)
- Automatic refresh; rotation logged

### 8.4 Finance Approach

- **V1:** Read-only transaction insights
- Transfers deferred until deterministic action framework proven
- High-risk classification for any payment capability

---

## 9. Deployment Topology

*Reference: diagrams/13_deployment_topology.mmd*

### 9.1 Azure Services

| Component | Azure Service | Notes |
|-----------|---------------|-------|
| Compute | AKS (Kubernetes) | Multi-tenant with namespace isolation |
| PostgreSQL | Azure Database for PostgreSQL Flexible | Managed, HA |
| Qdrant | Self-hosted on AKS | Persistent volumes; v1 simplicity |
| Redis | Azure Cache for Redis | Managed |
| Key Vault | Azure Key Vault | HSM-backed option for production |
| LLM | Azure OpenAI | GPT-4 class models |
| CDN | Azure CDN | Registry and static assets |
| Blob | Azure Blob Storage | Registry storage, backups |

### 9.2 Qdrant Deployment Justification

Self-hosted on AKS for v1:
- Data stays in Azure tenant (no external SaaS)
- Full control over version and config
- Managed Qdrant Cloud as future option

### 9.3 On-Device SLM

- Mobile app includes SLM (e.g., Phi-3 via llama.cpp)
- Local classification, summarization
- Sync via secure API to server

### 9.4 Agent-to-Agent Messaging

- **V1:** Server-mediated messaging
- Messages flow through intersection-scoped channels
- No direct agent-to-agent network connections
- Future: Consider libp2p or similar for enterprise

### 9.5 Enterprise Cell Upgrade

For enterprise customers requiring dedicated isolation:
- Dedicated AKS cluster
- Dedicated PostgreSQL instance
- Dedicated Key Vault
- Network-isolated VNet

---

## 10. Observability & Auditability

### 10.1 Structured Logging

```json
{
  "timestamp": "RFC3339",
  "level": "info|warn|error",
  "service": "circle-runtime",
  "tenant_id": "uuid",
  "circle_id": "uuid",
  "trace_id": "uuid",
  "message": "description",
  "attributes": {}
}
```

### 10.2 Metrics

- Prometheus-compatible metrics
- Key metrics: request latency, model invocation count, action success rate
- Per-tenant cardinality for isolation

### 10.3 Tracing

- OpenTelemetry for distributed tracing
- Trace ID propagated through all services
- Linked to audit log entries

### 10.4 Audit Hash Verification

```
entry_n.hash = SHA-256(entry_n.data || entry_(n-1).hash)
```

- Periodic integrity check (cron job)
- Optional external anchoring (timestamping service)
- Export includes full chain for independent verification

### 10.5 Security Monitoring

- Rate limiting per tenant/circle
- Anomaly detection on action patterns (without inspecting content)
- Alert on unusual grant patterns
- No surveillance of content (sovereignty preserved)

---

## 11. Non-Choices (Explicit Rejections)

| Rejected Technology | Why |
|---------------------|-----|
| **Kafka / Event Bus (v1)** | Unnecessary complexity; PostgreSQL + direct calls sufficient |
| **Blockchain for identity** | Overkill; signed manifests and Key Vault sufficient |
| **Global social graph** | Violates Canon (no global state) |
| **Implicit permission systems** | Violates Canon (no silent expansion) |
| **GraphQL** | REST simpler for v1; GraphQL for future if needed |
| **Microservices per feature** | Over-decomposition; layer-aligned services sufficient |
| **Self-hosted LLM (v1)** | Ops complexity; Azure OpenAI for v1 |

---

## 12. Risks & Mitigations

### 12.1 Security Risks

| Risk | Mitigation |
|------|------------|
| Compromised tenant | RLS isolation, per-tenant KEK, audit trail |
| Compromised device | On-device encryption, token revocation, session limits |
| Data exfiltration | No bulk export without auth, rate limits, audit |
| Key compromise | HSM-backed keys, rotation, monitoring |

### 12.2 AI/Model Risks

| Risk | Mitigation |
|------|------------|
| Hallucination | Confidence thresholds, high-risk escalation, human confirmation |
| Prompt injection | Input sanitization, structured prompts, output validation |
| Model unavailability | Suggest-only fallback, guarantee preservation |

### 12.3 Operational Risks

| Risk | Mitigation |
|------|------------|
| Cost overrun (LLM) | SLM-first routing, caching, usage monitoring |
| Outage | Suggest-only fallback, multi-AZ deployment |
| Audit storage growth | Retention policy (keep summary after N years), archival |
| Qdrant sync drift | Async indexing with retry, full rebuild capability |

---

## 13. Immediate Implementation Checklist

Build order aligned with runtime layers (bottom-up):

### Phase 1: Foundation
- [ ] PostgreSQL schema for circles, intersections, audit
- [ ] RLS policies and CI tests
- [ ] Azure Key Vault integration (KEK/DEK)
- [ ] Circle Runtime service (identity, policy)
- [ ] Basic auth service

### Phase 2: Core Loop
- [ ] Intersection Runtime service (contracts, versioning)
- [ ] Authority & Policy Engine (grants, enforcement)
- [ ] Audit service (append-only, hash chain)
- [ ] Invite token signing and verification

### Phase 3: Intelligence
- [ ] Server-side SLM deployment
- [ ] Azure OpenAI integration
- [ ] Negotiation Engine with routing
- [ ] Explainability capture

### Phase 4: Execution
- [ ] Action Executor service
- [ ] Connector framework
- [ ] Calendar connectors (Google + Microsoft)
- [ ] Pause/abort mechanics

### Phase 5: Memory
- [ ] Qdrant deployment on AKS
- [ ] Embedding pipeline
- [ ] Redis cache integration
- [ ] Memory layer service

### Phase 6: Marketplace
- [ ] Capability registry (Blob + CDN)
- [ ] Manifest validation
- [ ] Seal Validator service
- [ ] First certified capability

### Phase 7: Client
- [ ] Mobile app with on-device SLM
- [ ] Secure sync protocol
- [ ] Web client (basic)

---

## Document Control

| Field | Value |
|-------|-------|
| **Document** | QuantumLife Technology Selection v1 |
| **Status** | LOCKED — Technology Decisions |
| **Owner** | QuantumLayer Platform Ltd |
| **Subordinate To** | Canon v1, Technical Split v1, Human Guarantees v1 |
| **Changes** | Require ADR, version bump, and compliance review |

---

*Technologies are selected. Build order is defined. Canon compliance is mandatory.*
