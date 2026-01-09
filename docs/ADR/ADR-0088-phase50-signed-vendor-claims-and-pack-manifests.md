# ADR-0088: Phase 50 — Signed Vendor Claims + Pack Manifests (Authenticity-Only)

## Status

Accepted

## Context

Phase 46 introduced Circle Registry + Packs (Marketplace v0).
Phase 48 introduced Market Signal Binding (Non-Extractive Marketplace v1).
Phase 49 introduced Vendor Reality Contracts (HOLD-first, clamp-only).

There is now a need to verify that claims about vendors and packs are authentic —
that they actually come from the entity that controls a specific cryptographic key.

This phase introduces **cryptographic authenticity primitives**:
- Vendors can publish **Signed Vendor Claims** (e.g., "we cap ourselves at allow_surface_only")
- Pack authors can publish **Signed Pack Manifests** (hashes of pack contents + intended bindings)
- The system can verify Ed25519 signatures and persist only hashes/fingerprints

## Decision

### Invariant 1: Authenticity-Only, No Power

Phase 50 MUST NOT change decisions, outcomes, interrupts, delivery, or execution.
It only adds verifiable authenticity metadata and proof pages.

"Verified" means the signature checks out against the provided public key.
It does NOT mean the vendor is reputable or trustworthy.

### Invariant 2: Hash-Only Storage

We store ONLY:
- Claim/manifest hashes (SHA256 of canonical string)
- Key fingerprints (SHA256 of public key bytes)
- Verification status (enum)
- Provenance (enum)
- Period key
- Circle ID hash

We NEVER store:
- Raw public key bytes
- Signature bytes
- Vendor names, emails, URLs
- Free-form text fields

### Invariant 3: Pipe-Delimited Canonical Strings

All canonical strings use pipe-delimited format, NOT JSON.
This ensures deterministic serialization across implementations.

Message bytes for signature verification:
- Vendor claim: `QL|phase50|vendor_claim|` + claim.CanonicalString()
- Pack manifest: `QL|phase50|pack_manifest|` + manifest.CanonicalString()

### Invariant 4: No Forbidden Imports

Phase 50 packages MUST NOT import:
- pressuredecision
- interruptpolicy
- interruptpreview
- pushtransport
- interruptdelivery
- enforcementclamp
- Any package that controls decisions/delivery/execution

### Invariant 5: Ed25519 Only

We use Ed25519 for signatures:
- 32-byte public keys
- 64-byte signatures
- Standard Go crypto/ed25519 library
- No external cryptographic dependencies

### Invariant 6: Bounded Retention

- Maximum 200 records OR 30 days
- FIFO eviction
- Per-circle deduplication by (circleIDHash, periodKey, claimHash/manifestHash)

## What Is Signed

### Signed Vendor Claim

A vendor claim asserts something about the vendor's posture:
- Claim kind (enum): claim_vendor_cap, claim_pack_manifest, claim_observer_binding_intent
- Vendor scope (enum): scope_human, scope_institution, scope_commerce
- Pressure cap (enum): allow_hold_only, allow_surface_only
- Reference hash: SHA256 hex of related contract/signal record
- Provenance (enum): where the claim originated

The canonical string includes all fields in stable order, pipe-delimited.
The signature is over: `QL|phase50|vendor_claim|` + canonicalString

### Signed Pack Manifest

A pack manifest asserts the contents and bindings of a pack:
- Pack hash: SHA256 hex of pack content
- Pack version (enum bucket): v0, v1, v1_1
- Bindings hash: SHA256 hex of intended observer bindings
- Provenance (enum): where the manifest originated

The canonical string includes all fields in stable order, pipe-delimited.
The signature is over: `QL|phase50|pack_manifest|` + canonicalString

## Threat Model

| Threat | Mitigation |
|--------|------------|
| Fake claim from non-owner | Signature verification rejects |
| Key fingerprint collision | SHA256 has 256-bit security |
| Replay of old claim | Period key + dedup prevents storage duplicates |
| Vendor identity confusion | No identity mapping; fingerprint only |
| Data leakage | Hash-only storage; no raw keys/signatures |
| Decision manipulation | "No power" invariant; no imports from decision packages |

## Non-Goals

Phase 50 does NOT:
- Establish vendor identity or reputation
- Map keys to real-world entities
- Grant any permissions or capabilities
- Affect interrupt decisions or delivery
- Create allowlists or blocklists
- Store any free-form text

## Architecture

```
┌─────────────────────────┐
│  Vendor/Pack Author     │
│  (has Ed25519 keypair)  │
└───────────┬─────────────┘
            │
            │ POST /claims/submit
            │ or POST /manifests/submit
            │ (pubkey_b64, signature_b64, fields)
            ▼
┌─────────────────────────┐
│  SignedClaimsEngine     │
│  - Verify signature     │
│  - Hash canonical       │
│  - Fingerprint key      │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  SignedClaimStore       │
│  - Append record        │
│  - Hash-only storage    │
│  - Bounded retention    │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  /proof/claims          │
│  - Show verified status │
│  - Fingerprints only    │
│  - No raw keys          │
└─────────────────────────┘
```

## Routes

| Method | Path                | Purpose                          |
|--------|---------------------|----------------------------------|
| GET    | /proof/claims       | View verified claim/manifest proofs |
| POST   | /claims/submit      | Submit signed vendor claim       |
| POST   | /manifests/submit   | Submit signed pack manifest      |
| POST   | /proof/claims/dismiss | Dismiss proof cue for period   |

## Events

| Event                          | When                              |
|--------------------------------|-----------------------------------|
| phase50.claim.submitted        | Claim submitted for verification  |
| phase50.claim.verified         | Claim signature verified          |
| phase50.claim.persisted        | Claim record persisted to store   |
| phase50.manifest.submitted     | Manifest submitted for verification |
| phase50.manifest.verified      | Manifest signature verified       |
| phase50.manifest.persisted     | Manifest record persisted to store |

## Storelog Records

| Record Type              | Purpose                    |
|--------------------------|----------------------------|
| SIGNED_CLAIM             | Vendor claim verified      |
| SIGNED_MANIFEST          | Pack manifest verified     |

## Consequences

### Positive
- Vendors can prove authenticity of their claims
- Pack authors can prove authenticity of manifests
- No trust required in transport layer
- Cryptographically verifiable proofs

### Negative
- Requires key management by vendors/authors
- No identity recovery if key lost
- No reputation system (by design)

### Neutral
- Hash-only storage limits what can be displayed
- Fingerprints are opaque to users
- "Verified" is a technical term, not a trust endorsement

## References

- Phase 46: Circle Registry + Packs (Marketplace v0)
- Phase 48: Market Signal Binding (Non-Extractive Marketplace v1)
- Phase 49: Vendor Reality Contracts
- ADR-0084: Phase 46 Circle Registry
- ADR-0086: Phase 48 Market Signal Binding
- ADR-0087: Phase 49 Vendor Reality Contracts
