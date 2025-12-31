# QuantumLife Documentation

**Company:** QuantumLayer Platform Ltd
**Product:** QuantumLife

---

## Overview

This folder contains the complete QuantumLife specification — from foundational canon to technical architecture to implementation specs.

---

## Document Index

### Core Canon

The foundational "meaning lock" documents that define what QuantumLife is.

| Document | Description |
|----------|-------------|
| [QUANTUMLIFE_CANON_V1.md](QUANTUMLIFE_CANON_V1.md) | The constitutional definition of QuantumLife |
| [HUMAN_GUARANTEES_V1.md](HUMAN_GUARANTEES_V1.md) | User safety guarantees and commitments |

---

### End-State Vision

High-level vision and product definition.

| Document | Description |
|----------|-------------|
| [QUANTUMLIFE_END_STATE_V1.md](QUANTUMLIFE_END_STATE_V1.md) | End-state vision with 10 day-in-the-life scenarios |
| [SATISH_CIRCLES_TAXONOMY_V1.md](SATISH_CIRCLES_TAXONOMY_V1.md) | Satish's circle hierarchy: Work, Family, Finance, Health, Home, Kids |

---

### System Architecture

How the system is structured and operates.

| Document | Description |
|----------|-------------|
| [ARCHITECTURE_LIFE_OS_V1.md](ARCHITECTURE_LIFE_OS_V1.md) | Closed-loop architecture: Sense → Model → Decide → Propose → Approve → Execute → Audit → Learn |
| [TECHNICAL_ARCHITECTURE_V1.md](TECHNICAL_ARCHITECTURE_V1.md) | Process topology, v9.7 compliance, capability-based connectors |
| [INTERRUPTION_CONTRACT_V1.md](INTERRUPTION_CONTRACT_V1.md) | 5 interruption levels, regret scoring, rate limits |
| [IDENTITY_GRAPH_V1.md](IDENTITY_GRAPH_V1.md) | Entity definitions, deterministic IDs, multi-account unification |

---

### Technical Specifications

Detailed technical contracts and data models.

| Document | Description |
|----------|-------------|
| [TECH_SPEC_V1.md](TECH_SPEC_V1.md) | Canonical event models (Transaction, Order, Shipment, Ride, Subscription, Invoice, Message) |
| [CANONICAL_CAPABILITIES_V1.md](CANONICAL_CAPABILITIES_V1.md) | Capability taxonomy: email.*, calendar.*, finance.*, health.*, commerce.*, school.* |
| [MARKETPLACE_V1.md](MARKETPLACE_V1.md) | Connector ecosystem, SDK, 50+ planned connectors, regional routing |
| [INTEGRATIONS_MATRIX_V1.md](INTEGRATIONS_MATRIX_V1.md) | All integrations: Gmail, Outlook, Plaid, TrueLayer, WhatsApp, etc. |

---

### v9 Execution Canon

The execution safety layer — ensuring actions are safe, auditable, and reversible.

| Document | Description |
|----------|-------------|
| [CANON_ADDENDUM_V9_EXECUTION.md](CANON_ADDENDUM_V9_EXECUTION.md) | v9 execution principles and invariants |
| [HUMAN_GUARANTEES_V9_EXECUTION.md](HUMAN_GUARANTEES_V9_EXECUTION.md) | Execution-specific safety guarantees |
| [TECHNICAL_SPLIT_V9_EXECUTION.md](TECHNICAL_SPLIT_V9_EXECUTION.md) | v9 technical implementation details |
| [ACCEPTANCE_TESTS_V9_EXECUTION.md](ACCEPTANCE_TESTS_V9_EXECUTION.md) | v9 behavioral acceptance tests |

---

### v8 Financial Read Canon

Read-only financial capabilities — observation without execution.

| Document | Description |
|----------|-------------|
| [TECHNICAL_SPLIT_V1.md](TECHNICAL_SPLIT_V1.md) | Core technical architecture split |
| [TECHNOLOGY_SELECTION_V1.md](TECHNOLOGY_SELECTION_V1.md) | Technology choices and rationale |
| [TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md](TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md) | v8 financial read technology choices |
| [ACCEPTANCE_TESTS_V8_FINANCIAL_READ.md](ACCEPTANCE_TESTS_V8_FINANCIAL_READ.md) | v8 financial read behavioral tests |

---

### Architecture Decision Records (ADRs)

Documented decisions with context and rationale.

| ADR | Title |
|-----|-------|
| [ADR-0001](ADR/ADR-0001-runtime-language.md) | Runtime Language (Go) |
| [ADR-0002](ADR/ADR-0002-azure-multitenancy.md) | Azure Multitenancy |
| [ADR-0003](ADR/ADR-0003-storage-and-encryption.md) | Storage and Encryption |
| [ADR-0004](ADR/ADR-0004-qdrant-vector-memory.md) | Qdrant Vector Memory |
| [ADR-0005](ADR/ADR-0005-llm-slm-routing.md) | LLM/SLM Routing |
| [ADR-0006](ADR/ADR-0006-capability-seal-and-registry.md) | Capability Seal and Registry |
| [ADR-0007](ADR/ADR-0007-human-guarantees.md) | Human Guarantees |
| [ADR-0009](ADR/ADR-0009-v9-execution-technical-split.md) | v9 Execution Technical Split |
| [ADR-0010](ADR/ADR-0010-no-background-execution-guardrail.md) | v9.7 No Background Execution |
| [ADR-0011](ADR/ADR-0011-no-auto-retry-and-single-trace-finalization.md) | v9.8 No Auto-Retry + Single Trace |
| [ADR-0012](ADR/ADR-0012-write-provider-registry-lock.md) | v9.9 Write Provider Registry Lock |
| [ADR-0013](ADR/ADR-0013-payee-registry-lock.md) | v9.10 Payee Registry Lock |
| [ADR-0014](ADR/ADR-0014-v9.11-caps-and-rate-limits.md) | v9.11 Caps and Rate Limits |
| [ADR-0015](ADR/ADR-0015-v9.13-view-freshness-binding.md) | v9.13 View Freshness Binding |

---

### Diagrams

Visual representations of core concepts.

| File | Description |
|------|-------------|
| [diagrams/01_circles_intersections.mmd](diagrams/01_circles_intersections.mmd) | Overlapping sovereign circles and intersections |
| [diagrams/02_root_circle_genesis.mmd](diagrams/02_root_circle_genesis.mmd) | Root circle creation at onboarding |
| [diagrams/03_invite_flow.mmd](diagrams/03_invite_flow.mmd) | Invite token acceptance sequence |
| [diagrams/04_intersection_contract.mmd](diagrams/04_intersection_contract.mmd) | Intersection contract structure |
| [diagrams/05_irreducible_loop.mmd](diagrams/05_irreducible_loop.mmd) | The irreducible loop |
| [diagrams/06_concentric_product_levels.mmd](diagrams/06_concentric_product_levels.mmd) | Single → Family → Enterprise → Market |

---

## Quick Reference

### The Primitives

| Primitive | Definition |
|-----------|------------|
| **Circle** | Sovereign agent with identity, memory, policy |
| **Intersection** | Versioned contract between circles |
| **Authority Grant** | Scoped delegation of capability |
| **Proposal** | Request to change intersection terms |
| **Commitment** | Binding agreement to act |
| **Action** | Executed operation within authority |
| **Settlement** | Confirmed completion |
| **Memory** | Private state owned by circle |

### The Irreducible Loop

```
Intent → Intersection Discovery → Authority Negotiation → Commitment → Action → Settlement → Memory Update
```

### v9 Execution Invariants

| Version | Invariant |
|---------|-----------|
| v9.6 | Idempotency + Replay Defense |
| v9.7 | No Background Execution |
| v9.8 | No Auto-Retry + Single Trace Finalization |
| v9.9 | Write Provider Registry Lock |
| v9.10 | Payee Registry Lock |
| v9.11 | Caps and Rate Limits |
| v9.12 | Policy Snapshot Binding |
| v9.13 | View Freshness Binding |

### Capability Taxonomy

```
capabilities/
├── email/       (read, send, labels)
├── calendar/    (read, write, availability)
├── finance/     (balance, transactions, payment)
├── messaging/   (read, send)
├── health/      (activity, sleep, vitals, workouts)
├── commerce/    (orders, shipments, subscriptions)
├── transport/   (rides, trains, flights)
├── school/      (notifications, calendar, forms)
└── identity/    (contacts, profile)
```

---

## How to Use This Documentation

### For Designers

1. Start with [QUANTUMLIFE_END_STATE_V1.md](QUANTUMLIFE_END_STATE_V1.md) for vision
2. Review [SATISH_CIRCLES_TAXONOMY_V1.md](SATISH_CIRCLES_TAXONOMY_V1.md) for domain model
3. Check [INTERRUPTION_CONTRACT_V1.md](INTERRUPTION_CONTRACT_V1.md) for UX constraints
4. Verify against [QUANTUMLIFE_CANON_V1.md](QUANTUMLIFE_CANON_V1.md) anti-drift rules

### For Engineers

1. Start with [TECHNICAL_ARCHITECTURE_V1.md](TECHNICAL_ARCHITECTURE_V1.md) for system design
2. Review [TECH_SPEC_V1.md](TECH_SPEC_V1.md) for data models
3. Check [CANONICAL_CAPABILITIES_V1.md](CANONICAL_CAPABILITIES_V1.md) for interfaces
4. Follow ADRs for implementation decisions
5. Verify v9 compliance for any execution code

### For AI Tools

The **Execution Rule** applies:

> Claude Code and all AI tools implementing QuantumLife may build only what exists in this canon.

Before generating code:
1. Identify which canon primitive applies
2. Confirm the action stays within the irreducible loop
3. Verify no forbidden concepts are introduced
4. Check v9 invariants for execution code

---

## Anti-Drift Rules

### Forbidden Concepts at Core

- **Users** — Only circles
- **Accounts** — Circles have identity
- **Roles** — Authority is explicit per intersection
- **Workspaces** — Intersections are the shared domains
- **Global State** — All state is owned by circles or intersections

### QuantumLife MUST NOT Become

- Chatbot wrapper
- General app platform
- Social network
- Workflow engine
- Generic AI OS

---

## Modifying Documentation

### Changes Require

- Founder approval
- Clear justification
- Version bump
- Documented rationale

### Change Process

1. Propose change with written rationale
2. Review against existing canon
3. Identify all downstream impacts
4. Obtain founder approval
5. Update version number
6. Document change in changelog

---

*This documentation locks meaning. Drift is forbidden.*
