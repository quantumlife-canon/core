# Package: circle

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (Circle), §Genesis

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.1 Circle Runtime

## Responsibility
Provide the sovereign execution boundary for a single human's digital presence.

## What This Package OWNS
- Circle identity (self-sovereign)
- Root memory references
- Policy declarations
- Authority grant records
- Agent lifecycle

## What This Package MUST NOT DO
- Access other circles directly
- Own intersection contracts (only references)
- Maintain global state
- Execute actions outside authority
- Have direct access to other agents

## Invariants
- A circle MUST be self-contained
- A circle MUST NOT access another circle except through an intersection
- A circle MUST own all its memory
- A circle MUST be able to terminate at any time

## Package Imports
This package MUST NOT import other internal packages.
Cross-cutting concerns flow through interfaces defined here.
