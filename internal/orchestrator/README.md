# Orchestrator Package

This package implements the **Irreducible Loop** orchestration as defined in the Canon.

## Canon Reference

- `docs/QUANTUMLIFE_CANON_V1.md` §The Irreducible Loop
- `docs/TECHNICAL_SPLIT_V1.md` §3.2 Orchestration Layer

## The Irreducible Loop

Every operation follows these seven steps in order:

1. **Intent** — A desire or goal is expressed
2. **Intersection Discovery** — Find or create the relevant intersection
3. **Authority Negotiation** — Confirm or acquire necessary authority
4. **Commitment** — Bind to an action under stated conditions
5. **Action** — Execute within granted authority
6. **Settlement** — Confirm completion, exchange value if needed
7. **Memory Update** — Record outcome for future reference

## Responsibilities

- Orchestrate the flow through all loop steps
- Delegate to appropriate layer services at each step
- Maintain loop context and trace information
- Handle step failures and transitions
- Emit events at each step transition

## What This Package MUST Do

- Thread `LoopContext` through all steps
- Ensure each step completes before the next begins
- Call the appropriate layer service for each step
- Emit audit events at every step transition
- Enforce step ordering invariants

## What This Package MUST NOT Do

- Execute actions directly (delegate to execution layer)
- Make authority decisions (delegate to authority layer)
- Process intents semantically (delegate to negotiation layer)
- Store state persistently (delegate to memory layer)

## Boundary Enforcement

This package coordinates between layers but does not implement layer logic.
Each layer receives the `LoopContext` and returns control to the orchestrator.
