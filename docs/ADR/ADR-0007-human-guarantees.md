# ADR-0007: Human Guarantees Document

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife positions itself as a personal sovereign agent — a delegate, a digital twin, an "other self" that acts on the human's behalf across decades of life. This is fundamentally different from a SaaS tool or app subscription.

For this promise to be credible, users need more than feature documentation. They need a **trust contract** — explicit, testable guarantees about what QuantumLife will and will not do, regardless of business pressures, technology changes, or market evolution.

The Canon (v1) defines *what QuantumLife is*. The Technical Split (v1) defines *what must exist* technically. Neither speaks directly to the human in plain terms about their rights and protections.

A guarantees document bridges this gap. It translates canon invariants into user-facing commitments that can be tested, audited, and enforced.

---

## Decision

Create and lock `HUMAN_GUARANTEES_V1.md` as the permanent user trust contract.

This document:
- Is subordinate to the Canon (conflicts defer to Canon)
- Expresses guarantees as MUST/MUST NOT commitments
- Is testable — each guarantee maps to auditable system behavior
- Covers sovereignty, authority, time-bounds, safety, recovery, privacy, and marketplace
- Includes explicit "what we will never do" commitments

---

## Alternatives Considered

### Alternative 1: Embed guarantees in PRD
**Rejected.** PRDs change with product evolution. Guarantees must be stable across decades. Mixing them conflates mutable features with immutable principles.

### Alternative 2: Leave guarantees to UX/legal
**Rejected.** UX copy and legal terms are often vague or defensive. Technical guarantees must be precise enough to drive implementation and testing. Relegating them to legal creates drift between promise and product.

### Alternative 3: Define trust policies in code only
**Rejected.** Code without a governing document becomes implicit. Future developers may change behavior without realizing they've violated a guarantee. The document provides the authoritative reference.

### Alternative 4: Use industry-standard privacy policy
**Rejected.** Standard privacy policies are defensive documents designed for compliance. QuantumLife's guarantees are offensive — they define what the product *commits to*, not what it *might do*.

---

## Consequences

### Positive
- Users have a clear, permanent trust contract
- Implementation teams have testable requirements for trust-critical behavior
- Anti-drift is reinforced — guarantees make violations visible
- Recovery and succession are addressed before they become urgent
- Marketplace certification has explicit constraints

### Negative
- Guarantees constrain future flexibility (intentionally)
- Some guarantees may be expensive to implement (cost of trust)
- Requires ongoing verification that system behavior matches guarantees

### Neutral
- Guarantees document must be versioned if ever changed
- Changes require explicit rationale and canon compliance review

---

## Canon & Technical Split Alignment

| Guarantee Area | Canon Reference | Technical Split Reference |
|----------------|-----------------|---------------------------|
| Circle sovereignty | Canon §Ontology, §Genesis | Split §3.1 Circle Runtime |
| Intersection-only sharing | Canon §Geometry, §Intersections | Split §3.2 Intersection Runtime |
| Authority grants | Canon §Ontology (Authority Grant) | Split §3.3 Authority & Policy Engine |
| Revocation semantics | Canon §Agent Persona Contract | Split §6.1 Authority Revocation Mid-Action |
| Audit/explainability | Canon §Intersections (Audit Trail) | Split §3.7 Audit & Governance Layer |
| Safe failure | Canon §Production Definition | Split §6.4 Safe Failure Definition |
| Marketplace seal | Canon §Marketplace | Split §3.8 Marketplace & Seal Validation |
| Time-bounded authority | Canon §Intersections (Ceilings) | Split §5.3 Authority Grant Lifecycle |
| No silent expansion | Canon §Forbidden at Core | Split §7 Non-Goals |

---

## Related Documents

- `docs/QUANTUMLIFE_CANON_V1.md` — Constitutional document (superior)
- `docs/TECHNICAL_SPLIT_V1.md` — Technical boundaries (peer)
- `docs/HUMAN_GUARANTEES_V1.md` — This decision's output
- `docs/diagrams/14_trust_and_recovery_overview.mmd` — Visual summary

---

## Verification

This ADR is satisfied when:
1. `HUMAN_GUARANTEES_V1.md` exists and is marked LOCKED
2. Each guarantee is traceable to a canon invariant
3. Each guarantee is testable via audit, logging, or system behavior
4. No guarantee introduces new primitives or forbidden concepts
