# Package: authority

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (Authority Grant)

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.3 Authority & Policy Engine

## Responsibility
Validate all authority grants and enforce policy boundaries.

## What This Package OWNS
- Authority validation logic
- Policy enforcement rules
- Ceiling checks
- Scope boundary enforcement
- Expiry tracking

## What This Package MUST NOT DO
- Create authority (circles create via circle package)
- Define policy (circles define via circle package)
- Execute actions
- Own memory state
- Perform negotiation logic

## Invariants
- Authority MUST be explicitly granted (no implicit authority)
- Authority MUST be scoped and bounded
- Authority MUST be revocable
- Policy MUST be enforceable at any point in the loop
- All operations MUST be deterministic (no LLM/SLM)

## Package Imports
This package MUST NOT import other internal packages.
