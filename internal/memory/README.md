# Package: memory

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (Memory)

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.6 Memory Layer

## Technology Selection Reference
- `docs/TECHNOLOGY_SELECTION_V1.md` — §4 Data Stores & Ownership Model

## Responsibility
Persist and retrieve state owned by circles and intersections.

## What This Package OWNS
- Storage operations
- Versioned writes
- Query interface
- Private-by-default semantics
- Memory isolation

## What This Package MUST NOT DO
- Make ownership decisions (circles own, intersections share)
- Enforce access control logic (authority package does that)
- Execute actions
- Handle negotiation state
- Grant authority

## Data Store Mapping
- Authoritative: PostgreSQL (circles.memory, intersections.*)
- Assistive: Qdrant (semantic index, regenerable)
- Cache: Redis (ephemeral, non-authoritative)

## Invariants
- Memory MUST be owned by a circle or intersection
- Memory MUST be private by default
- Memory MUST NOT be shared without explicit intersection
- Memory writes MUST be versioned
- Audit logs MUST NOT be used as operational memory

## Package Imports
This package MUST NOT import other internal packages.
