# Package: intersection

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Intersections: Versioned Contracts, §Geometry

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.2 Intersection Runtime

## Responsibility
Manage the shared contract space between two or more circles.

## What This Package OWNS
- Contract state (versioned)
- Party references
- Scope definitions
- Audit trail for intersection
- Governance rules

## What This Package MUST NOT DO
- Own circle-private memory
- Hold authority beyond what parties granted
- Execute actions (only coordinates them)
- Maintain a global intersection registry
- Allow unilateral changes

## Invariants
- An intersection MUST be explicitly created (no implicit intersections)
- An intersection MUST NOT expand scope without all-party consent
- An intersection MUST version all changes
- An intersection MUST allow any party to dissolve their participation

## Package Imports
This package MUST NOT import other internal packages.
