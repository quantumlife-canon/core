# Package: negotiation

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §The Irreducible Loop (Authority Negotiation)

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.4 Negotiation Engine

## Responsibility
Handle proposals, counterproposals, and commitment formation.
This is a control-plane component — may use LLM/SLM.

## What This Package OWNS
- Proposal lifecycle
- Counterproposal generation
- Consensus detection
- Commitment formation
- Negotiation history

## What This Package MUST NOT DO
- Make final authority decisions (authority package does that)
- Execute actions (execution package does that)
- Write to memory directly (memory package does that)
- Enforce policy (authority package does that)
- Own circle identity

## Invariants
- Negotiation MUST be between parties in an intersection
- Negotiation MUST respect authority boundaries
- Commitment MUST require explicit acceptance
- Negotiation history MUST be auditable
- Negotiation MUST NOT assume cooperation; refusal is valid

## LLM/SLM Usage
This package may invoke LLM/SLM for:
- Intent classification
- Proposal analysis
- Counterproposal generation
- Disambiguation

All model calls MUST capture explainability data.

## Package Imports
This package MUST NOT import other internal packages.
