# Package: crypto

## Canon Reference
- `docs/QUANTUMLIFE_CANON_V1.md` — §Invitations (signed invite tokens)

## Technology Selection Reference
- `docs/TECHNOLOGY_SELECTION_V1.md` — §6 Identity & Crypto Posture

## Responsibility
Define interfaces for cryptographic operations including signing, verification, and key management.
Supports post-quantum readiness via algorithm agility.

## What This Package OWNS
- Signer interface definition
- Verifier interface definition
- KeyManager interface definition
- Key metadata types

## What This Package MUST NOT DO
- Implement specific algorithms (implementation is in internal/)
- Store keys directly
- Make policy decisions
- Access circles or intersections

## Invariants
- All keys are versioned
- Algorithm field supports rotation
- PQ-hybrid extension slot available
- No key material in exported types
