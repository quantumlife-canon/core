# ADR-0004: Qdrant for Vector Memory

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife agents need semantic memory retrieval — finding relevant past actions, preferences, and context based on meaning rather than exact keywords. This enables:

- "What did I decide about weekend plans last month?"
- "Find commitments similar to this proposal"
- "What's my preference pattern for restaurant recommendations?"

The Canon defines memory as circle-owned and private by default. Vector search must respect sovereignty.

---

## Decision

**Qdrant** is selected as the vector database for semantic memory retrieval.

### Deployment
- Self-hosted on AKS for v1 (control over data locality)
- Managed Qdrant Cloud as future option

### Role: Assistive Only
Qdrant is **not authoritative**. It is a derived index:

| Property | Value |
|----------|-------|
| Source of truth | PostgreSQL (circles.memory) |
| Qdrant role | Semantic index for retrieval |
| Regenerability | Full rebuild from PostgreSQL if needed |
| Staleness tolerance | Acceptable for seconds; not real-time critical |

### Collection Structure
```
Collection: circle_{circle_id}_memory
  - vector: embedding (dimension based on model)
  - payload: { entry_id, timestamp, summary, category }
  - filter: intersection_id (for intersection-scoped queries)
```

### Isolation
- One logical collection per circle
- Tenant ID validated before query
- No cross-circle vector search

---

## Alternatives Considered

### Pinecone
- **Pros:** Fully managed, scalable
- **Cons:** External SaaS, data leaves Azure, cost at scale
- **Verdict:** Rejected for v1; data sovereignty concerns

### pgvector (PostgreSQL extension)
- **Pros:** Single database, simpler ops
- **Cons:** Performance at scale, less mature ANN algorithms
- **Verdict:** Viable for small scale; Qdrant preferred for performance

### Weaviate
- **Pros:** Strong feature set, good integrations
- **Cons:** Heavier footprint, less focused than Qdrant
- **Verdict:** Viable alternative; Qdrant chosen for simplicity

### Milvus
- **Pros:** Highly scalable
- **Cons:** Complex deployment, overkill for v1
- **Verdict:** Consider for future scale; Qdrant for v1

---

## Consequences

### Positive
- Fast semantic retrieval enhances agent intelligence
- Self-hosted on AKS keeps data in Azure tenant
- Not authoritative = simpler consistency model
- Rebuild from PostgreSQL if corrupted

### Negative
- Additional infrastructure to operate
- Sync lag between PostgreSQL and Qdrant
- Embedding model choice affects retrieval quality

### Mitigations
- Qdrant deployed with persistent volumes on AKS
- Async indexing on PostgreSQL write with retry
- Embedding model versioned; re-index on model upgrade

---

## Embedding Model

### V1 Approach
- Use Azure OpenAI embedding model (text-embedding-3-small or similar)
- Store embedding version with each vector
- Re-index on model change

### Future
- On-device embedding for privacy-sensitive content
- Multiple embedding spaces for different memory types

---

## Canon & Technical Split Alignment

| Requirement | How Qdrant Satisfies |
|-------------|---------------------|
| Memory owned by circle (Canon §Ontology) | Per-circle collection, no cross-circle search |
| Assistive not authoritative (Split §3.6) | PostgreSQL is source of truth; Qdrant is derived |
| Private by default (Guarantees §7) | No cross-tenant vectors; tenant ID validated |
| Audit logs ≠ operational memory (Split §3.6) | Qdrant indexes circle memory, not audit |

---

## References

- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (Memory)
- `docs/TECHNICAL_SPLIT_V1.md` — §3.6 Memory Layer
- `docs/HUMAN_GUARANTEES_V1.md` — §7 Privacy & Taste
