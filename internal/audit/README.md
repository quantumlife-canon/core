# Package: audit

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Intersections (Audit Trail)
- `docs/QUANTUMLIFE_CANON_V1.md` — §Agent Persona Contract (auditable)

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.7 Audit & Governance Layer

## Human Guarantees Reference
- `docs/HUMAN_GUARANTEES_V1.md` — §2 Sovereignty (Auditability)
- `docs/HUMAN_GUARANTEES_V1.md` — §10 Verification

## Responsibility
Log all actions, enforce governance, and provide explainability.

## What This Package OWNS
- Immutable action logs
- Version histories
- Compliance checks
- Explainability interface
- Governance rule enforcement
- Hash chain integrity

## What This Package MUST NOT DO
- Execute actions
- Make decisions
- Own memory state
- Grant authority
- Handle negotiation

## Critical Invariant
Audit logs MUST NOT be used as operational memory or decision input.

## Invariants
- Every action MUST be logged
- Logs MUST be immutable (append-only)
- Any action MUST be explainable
- Governance violations MUST be flagged
- Hash chain MUST be verifiable
- Export MUST include full chain

## Package Imports
This package MUST NOT import other internal packages.
