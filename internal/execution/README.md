# Package: execution

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §The Irreducible Loop (Action, Settlement)
- `docs/QUANTUMLIFE_CANON_V1.md` — §Agent Persona Contract (What the Agent MUST NEVER Do)

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.5 Action Execution Layer
- `docs/TECHNICAL_SPLIT_V1.md` — §4.2 Data Plane

## Human Guarantees Reference
- `docs/HUMAN_GUARANTEES_V1.md` — §5 Safety & Reversibility

## Responsibility
Execute committed actions within granted authority.
This is a DATA PLANE component — DETERMINISTIC ONLY.

## What This Package OWNS
- Action execution
- Outcome reporting
- Settlement triggering
- Execution state
- Pause/abort mechanics

## What This Package MUST NOT DO
- Make decisions
- Expand scope
- Grant authority
- Interpret policy
- Own memory
- Use LLM/SLM (CRITICAL: data plane is model-free)

## Invariants
- This layer MUST NOT make decisions
- This layer MUST NOT expand scope
- This layer MUST report all outcomes
- This layer MUST be pausable and abortable
- Revocation MUST halt execution immediately
- There is NO "finish what you started" exception

## Package Imports
This package MUST NOT import negotiation or authority packages.
