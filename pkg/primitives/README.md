# Package: primitives

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Ontology (The Only Primitives Allowed at Core)

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §2 System Overview (Canon Primitives)

## Responsibility
Define the immutable data structures for all canon primitives that flow through the irreducible loop.

## What This Package OWNS
- Intent struct definition
- Proposal struct definition
- Commitment struct definition
- Action struct definition
- Settlement struct definition
- Basic validation stubs for each primitive

## What This Package MUST NOT DO
- Contain business logic
- Make decisions
- Persist data
- Call external services
- Import internal packages
- Define behavior beyond validation

## Invariants
- All primitives are immutable after creation
- All primitives have ID, Version, CreatedAt, Issuer
- No primitive may reference implementation details
