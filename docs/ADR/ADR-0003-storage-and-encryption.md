# ADR-0003: Storage Architecture and Encryption Model

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife stores sovereign circle data, intersection contracts, authority grants, actions, and audit logs. The storage architecture must:

- Enforce ownership boundaries (circle-owned vs intersection-owned vs audit)
- Support versioned contracts and grants
- Enable per-circle encryption with key rotation
- Provide append-only, immutable audit with hash chaining
- Support export for exit rights

The Canon defines memory as "private by default" and requires all sharing via intersections. The Human Guarantees promise exportable circle state with audit chain.

---

## Decision

### Authoritative State: PostgreSQL

**Azure Database for PostgreSQL (Flexible Server)** is the authoritative store for:

| Schema | Contents | Ownership |
|--------|----------|-----------|
| `circles` | Identity, policy, memory, encrypted keys | Circle-owned |
| `intersections` | Contracts, versions, grants, proposals, commitments, actions, settlements | Intersection-owned |
| `audit` | Append-only log, hash chain, explanations | System (immutable) |

### Assistive Memory: Qdrant
Qdrant provides semantic search over circle memory for intelligent retrieval. It is **not authoritative** — regenerable from PostgreSQL.

### Ephemeral Cache: Redis
Azure Redis caches query results and session state. **Non-authoritative** — evictable without data loss.

---

## Encryption Model

### Envelope Encryption

```
Azure Key Vault (KEK per tenant)
         │
         ▼
   Data Encryption Key (DEK per circle)
         │
         ▼
   Encrypted circle data in PostgreSQL
```

| Component | Key Type | Storage | Rotation |
|-----------|----------|---------|----------|
| Tenant KEK | RSA-4096 or PQ-hybrid | Azure Key Vault | Annual or on-demand |
| Circle DEK | AES-256-GCM | Encrypted in PostgreSQL (wrapped by KEK) | Quarterly or on-demand |

### Post-Quantum Readiness
- Key format includes algorithm version field
- Hybrid scheme: classical + PQ algorithm (e.g., Kyber) for key exchange
- Algorithm agility: can rotate to new PQ algorithms without data migration

---

## Row-Level Security

PostgreSQL RLS enforces tenant and circle boundaries:

```sql
-- Tenant isolation
CREATE POLICY tenant_isolation ON circles.memory
  USING (tenant_id = current_setting('app.tenant_id')::uuid);

-- Circle isolation within tenant
CREATE POLICY circle_isolation ON circles.memory
  USING (circle_id = current_setting('app.circle_id')::uuid);
```

All queries set `app.tenant_id` and `app.circle_id` via connection context.

---

## Audit Hash Chain

Audit log entries are hash-chained for tamper evidence:

```
entry_n.hash = SHA-256(entry_n.data || entry_(n-1).hash)
```

- Append-only: no UPDATE or DELETE on audit tables
- Periodic hash anchors to external witness (optional)
- Export includes full chain for verification

---

## Alternatives Considered

### Document Store (MongoDB, CosmosDB)
- **Pros:** Flexible schema
- **Cons:** Weaker transaction guarantees, harder RLS, less mature audit patterns
- **Verdict:** Rejected; PostgreSQL's relational model fits contract versioning

### Separate Database per Tenant
- **Pros:** Strongest isolation
- **Cons:** Operational complexity at scale, connection overhead
- **Verdict:** Reserved for enterprise cells; shared DB with RLS for default

### Client-Side Encryption Only
- **Pros:** Zero-knowledge at server
- **Cons:** Breaks semantic search, complicates backup/restore
- **Verdict:** Hybrid: server-side encryption with per-circle DEK; client holds nothing server can't reconstruct with user's key

---

## Consequences

### Positive
- Strong isolation via RLS + encryption
- Versioned contracts natively supported by PostgreSQL
- Hash-chained audit is tamper-evident
- Export includes cryptographically verifiable audit chain

### Negative
- RLS adds query complexity
- Key rotation requires DEK re-wrap (not data re-encryption)
- Qdrant sync must stay consistent with PostgreSQL

### Mitigations
- RLS policies tested in CI
- Key rotation is atomic wrap/unwrap operation
- Qdrant refresh triggered on PostgreSQL writes

---

## Canon & Technical Split Alignment

| Requirement | How Storage Satisfies |
|-------------|----------------------|
| Memory private by default (Canon §Ontology) | Circle schema with RLS + per-circle DEK |
| Versioned contracts (Canon §Intersections) | PostgreSQL versioning in intersections schema |
| Audit immutable (Split §3.7) | Append-only table, hash chain, no UPDATE/DELETE |
| Exit rights (Guarantees §2) | Export API returns full circle + audit chain |
| No backdoors (Guarantees §6) | Key Vault access audited; no platform key override |

---

## References

- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (Memory), §Intersections
- `docs/TECHNICAL_SPLIT_V1.md` — §3.6 Memory Layer, §3.7 Audit & Governance
- `docs/HUMAN_GUARANTEES_V1.md` — §2 Sovereignty, §10 Verification
