# ADR-0005: LLM/SLM Routing Architecture

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife agents require language model capabilities for:
- Intent classification
- Proposal generation and counterproposal reasoning
- Summarization and extraction
- Complex negotiation and disambiguation

The Canon requires explainability and auditability. The Technical Split mandates that the data plane be deterministic and model-free. Model usage must be constrained to the control plane.

Cost, latency, and availability considerations require intelligent routing between:
- Small Language Models (SLM) for fast, cheap first-pass
- Large Language Models (LLM) for complex reasoning

---

## Decision

### Model Placement

| Plane | Model Usage | Rationale |
|-------|-------------|-----------|
| **Control Plane** | SLM + LLM allowed | Decisions are explainable and auditable |
| **Data Plane** | NO models | Execution is deterministic |

### Routing Policy: SLM-First

```
Intent arrives
    │
    ▼
SLM processes (classification, simple extraction)
    │
    ▼
Confidence score generated
    │
    ├─ High confidence (≥ 0.85) ──► Proceed
    │
    ├─ Medium (0.70–0.85) ──► Context check
    │                              │
    │                              ├─ Low risk ──► Proceed
    │                              └─ High risk ──► Escalate to LLM
    │
    └─ Low confidence (< 0.70) ──► Escalate to LLM
```

### High-Risk Classification

Actions requiring LLM escalation regardless of SLM confidence:

| Risk Class | Examples |
|------------|----------|
| Financial | Payments > threshold, recurring commitments |
| Legal | Contract modifications, authority grants to new parties |
| Irreversible | Data deletion, intersection dissolution |
| Ambiguous | Multiple valid interpretations detected |

### LLM Provider
- **Azure OpenAI** (GPT-4 class) for high-quality reasoning
- Model version pinned and tested before promotion

### SLM Deployment
- **Server-side:** Small model (e.g., Phi-3, Mistral-7B quantized) on AKS for cheap first-pass
- **On-device:** SLM in mobile app for offline classification and local-first UX

---

## Fallback Modes

### LLM Unavailable
When Azure OpenAI is unreachable:

1. **Suggest-Only Mode** activates
2. Agent proposes actions but DOES NOT execute
3. Human must explicitly approve
4. All guarantees preserved
5. Audit records fallback state

### SLM Unavailable (server-side)
- Route directly to LLM (higher cost, acceptable for resilience)

### Both Unavailable
- Suggest-Only with SLM on-device if available
- Otherwise, queue intents for later processing

---

## Explainability Capture

Every model invocation records:

| Field | Purpose |
|-------|---------|
| `model_id` | Which model (version) was used |
| `input_hash` | Hash of prompt (for reproducibility) |
| `output` | Model response |
| `confidence` | Score if applicable |
| `reasoning_trace` | Chain-of-thought or decision rationale |
| `timestamp` | When invoked |
| `escalation_reason` | Why LLM was called (if escalated) |

Stored in audit log, queryable for explainability.

---

## Alternatives Considered

### LLM-Only
- **Pros:** Simplest architecture
- **Cons:** Cost prohibitive at scale, latency, single point of failure
- **Verdict:** Rejected; SLM-first is economically necessary

### SLM-Only
- **Pros:** Lowest cost, fastest
- **Cons:** Insufficient for complex reasoning, negotiations
- **Verdict:** Rejected; LLM escalation required for quality

### Multiple LLM Providers
- **Pros:** Resilience, cost optimization
- **Cons:** Prompt/behavior inconsistency, operational complexity
- **Verdict:** Azure OpenAI only for v1; multi-provider later

### Self-Hosted LLM
- **Pros:** Data stays local, no API costs
- **Cons:** Infra complexity, GPU costs, model updates
- **Verdict:** Consider for enterprise cells; Azure OpenAI for v1

---

## Consequences

### Positive
- Cost controlled via SLM-first routing
- Latency optimized for common cases
- Fallback preserves guarantees (suggest-only)
- Explainability captured for all model calls

### Negative
- Two model tiers to maintain and test
- Confidence thresholds require tuning
- On-device SLM requires mobile expertise

### Mitigations
- Evaluation harness for confidence calibration
- A/B testing framework for routing thresholds
- Mobile SLM integration via standard framework (e.g., llama.cpp)

---

## Canon & Technical Split Alignment

| Requirement | How Routing Satisfies |
|-------------|----------------------|
| Data plane deterministic (Split §4.2) | No models in data plane |
| Explainability (Canon §Agent Persona) | Every model call recorded with reasoning |
| Suggest-only fallback (Guarantees §3) | LLM unavailable triggers suggest-only mode |
| Audit completeness (Guarantees §10) | Model invocations in audit log |

---

## References

- `docs/QUANTUMLIFE_CANON_V1.md` — §Agent Persona Contract
- `docs/TECHNICAL_SPLIT_V1.md` — §4 Control Plane vs Data Plane, §3.4 Negotiation Engine
- `docs/HUMAN_GUARANTEES_V1.md` — §3 Authority & Autonomy, §10 Verification
