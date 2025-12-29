# ADR-0002: Azure Multi-Tenancy Architecture

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife is a premium SaaS product serving sovereign circles. The hosting model must:

- Enforce strong tenant isolation (circles must not leak data to other tenants)
- Support per-circle and per-tenant encryption
- Enable geographic compliance (data residency)
- Provide enterprise upgrade path to dedicated deployments
- Remain operationally efficient at scale

The Canon forbids global state and requires intersection-only sharing. The Human Guarantees promise that no data leaves a circle except through explicit intersections.

---

## Decision

**Azure** is selected as the cloud platform with a **multi-tenant hosted SaaS** architecture.

### Isolation Model

| Layer | Isolation Mechanism |
|-------|---------------------|
| Compute | Shared AKS cluster with namespace separation; tenant ID in all requests |
| Database | Shared PostgreSQL with row-level security (RLS) enforcing tenant boundaries |
| Encryption | Per-tenant KEK in Azure Key Vault; per-circle DEK for data encryption |
| Network | Network policies restricting cross-tenant traffic |
| Secrets | Per-tenant secret namespaces in Key Vault |

### Enterprise Option
Dedicated "cell" deployment available for enterprise customers:
- Isolated AKS cluster
- Dedicated PostgreSQL instance
- Separate Key Vault
- Own network boundaries

---

## Alternatives Considered

### AWS
- **Pros:** Mature, broad service catalog
- **Cons:** Azure has tighter LLM integration (Azure OpenAI), better enterprise identity (Entra ID)
- **Verdict:** Azure preferred for AI integration and enterprise sales

### GCP
- **Pros:** Strong Kubernetes, good AI/ML
- **Cons:** Smaller enterprise footprint, less familiar to target market
- **Verdict:** Viable but Azure preferred for go-to-market

### Single-Tenant from Day One
- **Pros:** Maximum isolation
- **Cons:** Operational complexity, cost prohibitive for consumer tier
- **Verdict:** Reserved for enterprise option; multi-tenant is default

### On-Premises / Self-Hosted
- **Pros:** Maximum control
- **Cons:** Incompatible with SaaS model; support burden
- **Verdict:** Not v1; may consider for regulated industries later

---

## Consequences

### Positive
- Azure OpenAI integration simplifies LLM deployment
- Enterprise customers familiar with Azure/Entra ID
- Managed services (PostgreSQL, Redis, Key Vault) reduce ops burden
- Cell-based upgrade path satisfies enterprise isolation requirements

### Negative
- Vendor lock-in to Azure services
- Multi-tenant RLS requires careful implementation and testing
- Per-tenant encryption adds key management complexity

### Mitigations
- Abstract cloud-specific APIs behind interfaces for future portability
- Comprehensive RLS testing in CI/CD
- Automated key rotation via Key Vault policies

---

## Canon & Technical Split Alignment

| Requirement | How Azure Multi-Tenancy Satisfies |
|-------------|-----------------------------------|
| No global state (Canon §Forbidden) | RLS enforces tenant boundaries; no cross-tenant queries |
| Circle sovereignty (Canon §Ontology) | Per-circle DEK ensures data encrypted at rest |
| Intersection-only sharing (Canon §Geometry) | Cross-tenant communication only via intersection contracts |
| Exit rights (Guarantees §2) | Export API extracts circle data from tenant-scoped storage |
| No backdoors (Guarantees §6) | Key Vault access audited; no platform-level override |

---

## References

- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (Forbidden at Core), §Geometry
- `docs/TECHNICAL_SPLIT_V1.md` — §3.1 Circle Runtime, §3.2 Intersection Runtime
- `docs/HUMAN_GUARANTEES_V1.md` — §2 Sovereignty, §6 Recovery & Succession
