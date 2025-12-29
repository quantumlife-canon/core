# Package: seal

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Marketplace: The QuantumLife Seal

## Technical Split Reference
- `docs/TECHNICAL_SPLIT_V1.md` — §3.8 Marketplace & Seal Validation Layer

## Human Guarantees Reference
- `docs/HUMAN_GUARANTEES_V1.md` — §8 Marketplace & Seal Guarantees

## ADR Reference
- `docs/ADR/ADR-0006-capability-seal-and-registry.md`

## Responsibility
Validate certified capabilities and gate external capability access.

## What This Package OWNS
- Seal validation logic
- Certification registry (read)
- Capability gating
- Trust attestation

## What This Package MUST NOT DO
- Execute capabilities
- Grant authority (only circles can do that)
- Own circle state
- Handle negotiation

## Invariants
- Uncertified capabilities MUST require explicit human approval
- Certified capabilities MUST conform to canon principles
- Seal validation MUST be auditable
- Certification MUST NOT imply automatic trust
- Capabilities MUST NOT self-authorize

## Manifest Requirements
Per ADR-0006, capability manifests must include:
- id, version, name, description
- scopes_requested
- risk_class
- audit_hooks
- signature (with algorithm)
- provenance

## Package Imports
This package MUST NOT import other internal packages.
