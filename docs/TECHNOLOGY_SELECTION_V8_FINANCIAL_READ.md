# QuantumLife Technology Selection v8 — Financial Read

**Status:** LOCKED
**Version:** 1.0
**Subordinate To:** QuantumLife Canon v1, QuantumLife Technical Split v8 — Financial Read
**Scope:** Technology selection for Financial Read & Propose capabilities only

---

## 1. Document Purpose

This document specifies concrete technology selections for implementing Financial Read capabilities within the QuantumLife system. All selections are constrained by and subordinate to the Canon v1 and Technical Split v8 documents.

This document makes binding technology choices. It does not define architecture, product requirements, or system behavior—those are defined in superior documents.

**This document is a selection guide, not an extension of Canon.**

---

## 2. Scope Clarification

### 2.1 What This Document Covers

- Technology choices for reading financial data from external providers
- Technology choices for normalizing and storing financial observations
- Technology choices for generating deterministic proposals
- Technology choices for audit and security compliance

### 2.2 Explicit Boundaries

**PERMITTED:**
- Read financial data from connected providers
- Normalize data into canonical representations
- Store observations in circle/intersection memory
- Generate proposals for human review
- Audit all read operations

**FORBIDDEN — No Technology Selection Exists For:**
- Payment execution or transfers
- Scheduled or automated operations
- Machine learning inference
- Behavioral optimization or nudging
- Write operations to financial providers
- Cross-circle data aggregation
- Background job processing

The absence of technology selections for forbidden capabilities is intentional and constitutive. There is no "future selection" for execution—execution is architecturally impossible.

---

## 3. Runtime Placement

### 3.1 Control Plane Positioning

Financial Read operates exclusively within the **Control Plane** as defined in Technical Split v8.

| Layer | Financial Read Role |
|-------|---------------------|
| Decision Layer | Proposals generated here; no execution authority |
| Connector Layer | Read-only adapters to financial providers |
| Memory Layer | Snapshot and observation storage |
| Audit Layer | All operations logged with redaction |

### 3.2 Why Control Plane Only

Financial Read MUST NOT interact with any execution layer because:

1. **No write primitives exist** — The Action type for financial operations only supports `propose` mode
2. **No execution connectors exist** — Technology selection explicitly excludes payment SDKs with write capability
3. **Memory is append-only for observations** — Historical financial data cannot be mutated
4. **Audit prevents silent operations** — Every read operation leaves a trace

The Control Plane boundary is enforced at the type level, not by runtime checks.

---

## 4. Connector Technology Selection

### 4.1 Financial Read Connector Interface

**Technology:** Go interface with read-only method signatures

The connector interface MUST be defined with only read operations. Write methods are not merely unimplemented—they do not exist in the interface definition.

```go
// Illustrative only — actual interface in internal/connectors/finance
type FinanceReadConnector interface {
    GetAccounts(ctx, params) ([]FinancialAccount, error)
    GetTransactions(ctx, params) ([]Transaction, error)
    GetBalances(ctx, params) ([]Balance, error)
    ProviderInfo() ProviderInfo
}
```

**Rationale:** By defining an interface without write methods, no implementation can provide write capability. This is compile-time enforcement, not runtime policy.

### 4.2 Provider SDK Strategy

**Technology:** Plaid Go SDK (read-only import path)

**Selection Criteria:**
- Well-maintained official SDK
- Clear separation of read vs. write endpoints
- OAuth 2.0 support with scope restrictions
- No payment initiation modules imported

**Import Restrictions:**

| PERMITTED Imports | FORBIDDEN Imports |
|-------------------|-------------------|
| `plaid-go/plaid` (core) | `plaid-go/plaid/payment` |
| Account read endpoints | Transfer initiation |
| Transaction read endpoints | Payment execution |
| Balance read endpoints | Link token with write scopes |

**Enforcement:** Go build constraints and code review. The `payment` subpackage MUST NOT appear in any import statement within the Financial Read connector.

**Rationale:** Importing only read-capable modules makes write operations impossible at the dependency level. Even if code attempted to call a write endpoint, the types would not be available.

### 4.3 Token Handling

**Technology:** Existing Token Broker (as defined in Technical Split v8)

Financial provider tokens MUST be:
- Stored via the existing Token Broker infrastructure
- Encrypted at rest using existing crypto primitives
- Scoped to the circle that authorized the connection
- Never shared across intersections without explicit grant

**No new token storage technology is introduced.** Financial Read reuses the same Token Broker that handles calendar OAuth tokens.

### 4.4 OAuth Scope Restrictions

**Technology:** Scope allowlist enforced at Token Broker level

**Permitted Scopes (Plaid example):**
- `transactions:read`
- `accounts:read`
- `balance:read`
- `identity:read` (for account holder name matching)

**Forbidden Scopes (hard rejection):**
- `payment_initiation:*`
- `transfer:*`
- `auth:*` (bank auth for payments)
- Any scope containing `write`, `execute`, `initiate`, or `transfer`

**Enforcement Mechanism:**

The Token Broker MUST reject any OAuth flow that requests forbidden scopes. This is not a policy check—it is a code path that does not exist for forbidden scopes.

**Rationale:** OAuth scope restriction at the broker level means that even if a connector implementation attempted to use a write scope, the token would never be issued.

---

## 5. Data Ingestion & Normalization

### 5.1 Canonical Financial Model

**Technology:** Go structs with deterministic field mappings

The canonical model provides a provider-agnostic representation of financial data:

| Canonical Type | Purpose |
|----------------|---------|
| `FinancialSnapshot` | Point-in-time view of accounts and balances |
| `NormalizedTransaction` | Single transaction with category and merchant |
| `FinancialObservation` | Derived insight from snapshot comparison |
| `FinancialProposal` | Human-reviewable suggestion |

**Field Normalization Rules:**
- Currency amounts stored as integer cents (no floating point)
- Timestamps normalized to UTC
- Merchant names normalized via deterministic string rules (lowercase, trim, collapse whitespace)
- Categories mapped via explicit lookup table

**Rationale:** Deterministic normalization ensures that the same input always produces the same output. There is no probabilistic inference, no ML-based entity resolution, no "smart" matching that could produce surprising results.

### 5.2 Normalization Implementation

**Technology:** Pure Go functions with no external dependencies

Normalization MUST be implemented as:
- Pure functions (output depends only on input)
- No network calls during normalization
- No database lookups during normalization
- No randomness or time-dependent behavior

**Rationale:** Pure normalization functions are testable, auditable, and predictable. A circle can understand exactly how their data was transformed.

### 5.3 Versioning Strategy

**Technology:** Explicit version field on all canonical types

Every canonical type MUST include:
- `SchemaVersion string` — The version of the canonical schema
- `NormalizerVersion string` — The version of the normalization rules applied

When normalization rules change, the version increments. Historical data is never re-normalized silently.

**Rationale:** Version fields make transformations explicit. A human reviewing financial data can see exactly which rules were applied.

---

## 6. Categorization Strategy

### 6.1 Rule-Based Categorization Only

**Technology:** Deterministic rule engine (Go switch/case + lookup tables)

Transaction categorization MUST use:
- Explicit merchant-to-category mapping tables
- Pattern-based rules with deterministic priority ordering
- Fallback to "Uncategorized" rather than probabilistic guessing

**Categorization is NOT:**
- Machine learning classification
- Probabilistic inference
- Collaborative filtering based on other circles' data
- "Smart" categorization that learns over time

### 6.2 Category Taxonomy

**Technology:** Fixed enumeration of categories

Categories MUST be:
- Defined in a single canonical location
- Enumerated (not free-form strings)
- Stable across versions (categories may be added but never removed or renamed)

**Rationale:** A fixed taxonomy ensures that financial observations are comparable over time and that proposals reference stable categories.

### 6.3 Categorization Confidence

**Technology:** Boolean certainty flag, not probability score

Each categorization MUST include:
- `Category string` — The assigned category
- `MatchedRule string` — The rule that produced this categorization
- `Certain bool` — Whether the match was exact (true) or fallback (false)

**Why not probability scores?**

Probability scores invite optimization. A 73% confidence categorization suggests that a better algorithm could reach 85%. This creates pressure for ML enhancement. Binary certainty (matched or fallback) removes this pressure.

**Rationale:** Binary certainty is honest. Either we know, or we don't. There is no illusion of precision.

---

## 7. Proposal Generation

### 7.1 Deterministic Insight Generation

**Technology:** Rule-based observation engine (Go)

Proposals MUST be generated by:
- Comparing current snapshot to previous snapshots
- Applying threshold-based rules to detect notable changes
- Generating human-readable observations with explicit reasoning

**Proposal generation is NOT:**
- Predictive modeling
- Trend extrapolation
- Goal optimization
- Behavioral nudging

### 7.2 Threshold-Based Observations

**Technology:** Configurable thresholds stored in intersection contract

Observable patterns and their thresholds:

| Pattern | Threshold | Proposal Type |
|---------|-----------|---------------|
| Balance decrease | > 20% of prior balance | `observation.balance_change` |
| New recurring charge | 2+ occurrences same merchant/amount | `observation.recurring_detected` |
| Large transaction | > configured amount | `observation.large_transaction` |
| Category shift | > 30% change in category spend | `observation.category_shift` |

Thresholds MUST be:
- Explicitly configured in the intersection contract
- Visible to all parties
- Modifiable only through contract negotiation

**Rationale:** Explicit thresholds make proposals predictable. A circle knows exactly what will trigger an observation.

### 7.3 Confidence Annotations

**Technology:** Structured confidence metadata on proposals

Every proposal MUST include:
- `Basis []string` — The data points that produced this proposal
- `Assumptions []string` — Any assumptions made (e.g., "assuming monthly recurrence")
- `Limitations []string` — What the proposal does not account for

**Rationale:** Confidence annotations prevent false certainty. A proposal about "unusual spending" explicitly states that it only sees connected accounts.

### 7.4 No Urgency Framing

**Technology:** Neutral language templates

Proposal text MUST NOT use:
- Urgency language ("Act now", "Don't miss", "Before it's too late")
- Fear language ("Warning", "Alert", "Risk")
- Comparative judgment ("You're spending more than...")
- Goal-oriented framing ("To save more...")

Proposal text MUST use:
- Observational language ("This month's grocery spending was...")
- Factual comparisons ("This is higher than the previous month by...")
- Optional framing ("You may want to review...")

**Rationale:** Neutral language respects autonomy. Observations inform; they do not manipulate.

---

## 8. Memory Storage

### 8.1 Memory Keys for Financial Data

**Technology:** Existing Memory Store with financial key prefixes

| Key Pattern | Data Type | Ownership |
|-------------|-----------|-----------|
| `fin:snapshot:{circle}:{date}` | FinancialSnapshot | Circle |
| `fin:tx:{circle}:{id}` | NormalizedTransaction | Circle |
| `fin:obs:{intersection}:{id}` | FinancialObservation | Intersection |
| `fin:proposal:{intersection}:{id}` | FinancialProposal | Intersection |

### 8.2 Ownership Semantics

**Circle-Owned Data:**
- Raw financial snapshots belong to the circle that authorized the connection
- Transaction details are circle-scoped
- A circle may revoke access, removing intersection visibility

**Intersection-Owned Data:**
- Observations derived from comparing circles' data
- Proposals generated for intersection review
- Audit logs of what was shared and when

**Rationale:** Clear ownership ensures that financial data respects circle boundaries. An intersection sees only what circles have explicitly shared.

### 8.3 Versioning Rules

**Technology:** Append-only storage with version chains

Financial memory MUST be:
- Append-only for snapshots (new snapshot does not delete old)
- Immutable for historical transactions (corrections create new records)
- Version-chained for observations (each observation references prior version)

**Mutation is forbidden.** If a transaction is miscategorized, the correction creates a new record with a reference to the original. The original remains unchanged.

**Rationale:** Immutability ensures auditability. Historical financial data cannot be silently altered.

### 8.4 Retention Policy

**Technology:** Configurable retention in intersection contract

Retention MUST be:
- Explicitly specified in the intersection contract
- Enforced by the Memory Store
- Audited when data is aged out

Default retention periods:
- Snapshots: 13 months (covers year-over-year comparison)
- Transactions: 13 months
- Observations: 25 months
- Proposals: Until explicitly dismissed or 25 months

**Rationale:** Explicit retention prevents indefinite data accumulation while supporting reasonable lookback periods.

---

## 9. Audit & Explainability

### 9.1 Required Audit Events

**Technology:** Existing Audit Store with financial event types

| Event Type | Trigger | Required Fields |
|------------|---------|-----------------|
| `finance.provider_connected` | OAuth flow completed | provider, scopes_granted, circle |
| `finance.snapshot_ingested` | Snapshot stored | snapshot_id, record_count, circle |
| `finance.observation_generated` | Observation created | observation_id, basis_snapshot_ids, intersection |
| `finance.proposal_created` | Proposal generated | proposal_id, observation_id, intersection |
| `finance.proposal_dismissed` | Human dismissed proposal | proposal_id, dismissing_circle |
| `finance.data_shared` | Circle shared data with intersection | snapshot_id, intersection, scopes |
| `finance.access_revoked` | Circle revoked intersection access | intersection, circle, reason |

### 9.2 Redaction Rules

**Technology:** Field-level redaction in audit serialization

Sensitive fields MUST be redacted in audit logs:
- Account numbers: Show last 4 digits only
- Balances: Redact unless explicitly configured to log
- Transaction amounts: Redact in audit, available in memory
- Merchant names: Full name in audit (not considered sensitive)

**Rationale:** Audit logs must be reviewable without exposing full financial details. Redaction balances auditability with privacy.

### 9.3 Traceability Guarantees

**Technology:** Trace ID propagation through all operations

Every financial operation MUST:
- Carry a trace ID from ingestion to proposal
- Reference source snapshot IDs in derived data
- Link observations to the proposals they generated

**A human MUST be able to answer:** "Why did I see this proposal?" by following trace IDs back to source data.

**Rationale:** Traceability makes the system explainable. There are no mysterious recommendations.

---

## 10. Security & Privacy

### 10.1 Encryption at Rest

**Technology:** Azure Storage Service Encryption with customer-managed keys

Financial data MUST be encrypted using:
- Azure SSE for blob/table storage
- Customer-managed keys (CMK) in Azure Key Vault
- Key rotation every 90 days

**No additional encryption layer is introduced.** Financial data uses the same encryption infrastructure as other sensitive data.

### 10.2 Encryption in Transit

**Technology:** TLS 1.3 for all provider communication

All connections to financial providers MUST use:
- TLS 1.3 (minimum TLS 1.2 for legacy providers)
- Certificate validation
- No certificate pinning (providers rotate certificates)

### 10.3 Least-Privilege Access

**Technology:** Azure RBAC with dedicated Financial Read role

The Financial Read service principal MUST have:
- Read access to financial memory keys only
- No write access to execution stores
- No access to other circles' data
- Audit log write access only

**Rationale:** Least-privilege ensures that even if the Financial Read component is compromised, it cannot execute transactions or access unauthorized data.

### 10.4 No Cross-Tenant Analytics

**Technology:** Tenant isolation at storage layer

Financial data MUST be:
- Stored in tenant-specific containers
- Never aggregated across tenants
- Never used for cross-tenant insights or benchmarking

**There is no "anonymized aggregate" path.** Each tenant's financial data is isolated completely.

**Rationale:** Cross-tenant analytics, even "anonymized," creates pressure for data retention and secondary use. Complete isolation removes this pressure.

### 10.5 No Secondary Data Use

**Technology:** No technology selected (intentional absence)

Financial data MUST NOT be used for:
- Model training (no ML selected)
- Product analytics
- Marketing segmentation
- Partner data sharing

The absence of technology for secondary use is the enforcement mechanism.

---

## 11. Failure Semantics

### 11.1 Provider Failure Handling

**Technology:** Circuit breaker pattern with explicit degradation

When a financial provider is unavailable:
1. Return last successful snapshot with `stale: true` flag
2. Log provider failure to audit
3. Display clear "Data as of {timestamp}" in proposals
4. Retry with exponential backoff (max 3 attempts)
5. After sustained failure, notify circle of degraded state

**Failure does NOT:**
- Trigger automated remediation
- Escalate to alerts or notifications
- Invoke fallback providers
- Cache indefinitely

### 11.2 Partial Data Handling

**Technology:** Explicit partial flags on snapshots

When only some accounts are accessible:
- Store partial snapshot with `partial: true` flag
- List accessible and inaccessible accounts
- Generate observations only from accessible data
- Note partiality in any derived proposals

**Rationale:** Partial data is clearly marked. Proposals based on incomplete information are explicit about their limitations.

### 11.3 Calm Degradation Principles

**Technology:** Degradation states as first-class types

Degradation is not an error state—it is a normal operating condition:

| State | Meaning | System Behavior |
|-------|---------|-----------------|
| `current` | Data is fresh | Normal operation |
| `stale` | Data older than refresh interval | Show staleness, continue operation |
| `partial` | Some accounts unavailable | Note gaps, continue with available |
| `unavailable` | Provider unreachable | Show last known state, pause new observations |

**Rationale:** Calm degradation means the system continues to be useful even when providers fail. There is no panic state.

---

## 12. Explicit Anti-Patterns (Hard Rejections)

### 12.1 Rejected Technologies

| Technology | Reason for Rejection |
|------------|---------------------|
| **Plaid Transfer API** | Enables payment execution — violates Canon |
| **Any payments SDK** | Write capability exists — violates read-only constraint |
| **Scheduled job frameworks** | Enables automation — violates human-in-loop requirement |
| **ML inference engines** | Probabilistic output — violates determinism requirement |
| **Message queues for financial ops** | Enables async execution — violates synchronous proposal model |
| **Third-party categorization APIs** | External inference — violates explainability requirement |
| **Financial planning optimizers** | Goal optimization — violates neutral observation requirement |
| **Push notification services** | Urgency mechanism — violates calm communication requirement |
| **A/B testing frameworks** | Behavioral experimentation — violates trust requirement |
| **Analytics pipelines** | Secondary data use — violates privacy requirement |

### 12.2 Rejected Approaches

| Approach | Reason for Rejection |
|----------|---------------------|
| **"Read now, execute later" patterns** | Creates execution path — violates Canon |
| **Storing payment credentials** | Enables future execution — violates read-only constraint |
| **Background sync jobs** | Hidden automation — violates transparency requirement |
| **Smart categorization** | Unexplainable inference — violates explainability requirement |
| **Spending goals** | Behavioral manipulation — violates autonomy requirement |
| **Alerts and notifications** | Urgency pressure — violates calm requirement |
| **Comparative benchmarks** | Social pressure — violates privacy requirement |
| **Predictive budgeting** | Future projection — violates observational scope |

### 12.3 Why These Rejections Matter

Each rejection removes a path to authority creep. The question is not "could this be used safely?" but "does this create a path that must be governed?"

By rejecting technologies that could enable execution, we eliminate the governance burden entirely. There is no policy needed for payment execution because no payment execution capability exists.

---

## 13. Readiness Checklist

### 13.1 Technology Selection Complete When

- [ ] Financial Read Connector interface defined (read-only methods only)
- [ ] Plaid SDK imported with payment modules excluded
- [ ] Token Broker extended with financial scope allowlist
- [ ] OAuth scope rejection implemented for write scopes
- [ ] Canonical financial model defined with version fields
- [ ] Deterministic normalization functions implemented
- [ ] Rule-based categorization engine implemented
- [ ] Category taxonomy enumerated
- [ ] Threshold-based observation rules defined
- [ ] Proposal templates with neutral language created
- [ ] Memory keys defined with ownership semantics
- [ ] Audit events defined for all financial operations
- [ ] Redaction rules implemented for sensitive fields
- [ ] Azure encryption configured with CMK
- [ ] Tenant isolation verified at storage layer
- [ ] Circuit breaker implemented for provider failures
- [ ] Degradation states implemented as types
- [ ] All rejected technologies verified absent from codebase

### 13.2 Execution Remains Impossible

Before v8 Financial Read is considered complete, the following MUST be verified:

1. **No write methods exist** in any financial connector interface
2. **No payment SDK modules** are imported anywhere in the codebase
3. **No write OAuth scopes** can be requested through Token Broker
4. **No execution Action types** exist for financial operations
5. **No background jobs** process financial data
6. **No queues** exist for financial operations
7. **No stored credentials** enable payment execution

**Verification method:** Code audit + CI guardrails that fail on forbidden imports

### 13.3 Final Attestation

This technology selection document is complete when:

1. All checklist items are satisfied
2. Execution impossibility is verified
3. Document is reviewed against Canon v1 and Technical Split v8
4. No technology selection creates a path to execution
5. All selections reinforce calm, clarity, and trust

---

## Appendix A: Relationship to Superior Documents

| Document | Relationship |
|----------|--------------|
| QuantumLife Canon v1 | Supreme authority — this document cannot contradict |
| Technical Split v8 | Architectural authority — this document implements within constraints |
| Human Guarantees v1 | Trust authority — all selections must preserve guarantees |
| This Document | Technology selection only — subordinate to all above |

This document makes no architectural decisions. It selects technologies that implement the architecture defined in Technical Split v8, within the boundaries defined by Canon v1, preserving the guarantees defined in Human Guarantees v1.

---

## Appendix B: Glossary

| Term | Definition in This Document |
|------|----------------------------|
| **Read** | Retrieve data from external provider without modification |
| **Propose** | Generate human-reviewable observation or suggestion |
| **Execute** | Perform action that modifies external state (FORBIDDEN) |
| **Deterministic** | Same input always produces same output |
| **Observation** | Factual statement derived from financial data |
| **Proposal** | Suggestion presented for human review, never auto-executed |
| **Calm** | Without urgency, pressure, or manipulation |

---

*This document is LOCKED. Changes require explicit versioning and review against Canon constraints.*
