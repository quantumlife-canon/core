# ADR-0001: Runtime Language Selection

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife requires a runtime language for all core services, daemons, and CLI tooling. The language must support:

- High-performance, low-latency execution for the data plane
- Strong concurrency primitives for agent coordination
- Static typing for safety-critical code paths (authority enforcement, audit)
- Cross-platform compilation for client-side components
- Long-term maintainability for a decades-spanning product

The Canon defines sovereignty, explicit authority, and deterministic execution. The Technical Split mandates that the data plane be deterministic and non-decisional. The language choice must enable these properties.

---

## Decision

**Go** is selected as the runtime language for all core services.

### Rationale

| Requirement | Go Capability |
|-------------|---------------|
| Deterministic execution | Compiled, predictable performance, no GC pauses in hot paths with careful design |
| Concurrency | Goroutines + channels map well to agent coordination and intersection messaging |
| Static typing | Compile-time safety for authority checks, contract versioning |
| Cross-platform | Single binary compilation for Linux (server), macOS, Windows, mobile (via gomobile) |
| Operational simplicity | No runtime dependencies, small container images, fast startup |
| Ecosystem | Strong Azure SDK, PostgreSQL drivers, gRPC support |

---

## Alternatives Considered

### Rust
- **Pros:** Memory safety, zero-cost abstractions, excellent for data plane
- **Cons:** Steeper learning curve, slower iteration for control plane logic, smaller talent pool
- **Verdict:** Considered for future data plane hot paths; not v1

### TypeScript/Node.js
- **Pros:** Fast iteration, large ecosystem
- **Cons:** Single-threaded, GC unpredictability, weaker type safety at runtime
- **Verdict:** Acceptable for UI layer only; rejected for core services

### Python
- **Pros:** ML/AI ecosystem
- **Cons:** Performance, GIL, runtime type errors
- **Verdict:** Rejected for core; acceptable for offline ML training pipelines

### Java/Kotlin
- **Pros:** Mature, strong typing, good Azure support
- **Cons:** JVM overhead, container size, startup time
- **Verdict:** Viable but Go preferred for operational simplicity

---

## Consequences

### Positive
- Single language for all backend services reduces cognitive overhead
- Fast compilation enables rapid iteration
- Small binaries simplify deployment and reduce attack surface
- Strong concurrency model aligns with agent coordination patterns

### Negative
- Generics are limited (improved in Go 1.18+, but less expressive than Rust/TypeScript)
- Error handling is verbose (mitigated by consistent patterns)
- UI requires separate language (TypeScript/React planned for later)

### Mitigations
- Establish idiomatic Go patterns in style guide
- Use code generation for repetitive boilerplate
- Reserve Rust for future performance-critical paths if needed

---

## Canon & Technical Split Alignment

| Requirement | How Go Satisfies |
|-------------|------------------|
| Deterministic data plane (Split §3.5) | Compiled execution, no runtime interpretation |
| Auditable actions (Canon §Intersections) | Static types enable compile-time audit hook enforcement |
| Revocation halts execution (Guarantees §5) | Goroutine cancellation via context propagation |
| Explainability (Guarantees §2) | Structured logging with strong typing |

---

## References

- `docs/QUANTUMLIFE_CANON_V1.md` — §Execution Rule
- `docs/TECHNICAL_SPLIT_V1.md` — §3.5 Action Execution Layer, §4 Control vs Data Plane
- `docs/HUMAN_GUARANTEES_V1.md` — §5 Safety & Reversibility
