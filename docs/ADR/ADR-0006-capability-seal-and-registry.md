# ADR-0006: Capability Seal and Registry

**Status:** Accepted
**Date:** 2025-12-29
**Deciders:** QuantumLayer Platform Ltd

---

## Context

QuantumLife includes a marketplace for capabilities — extensions that agents can use to interact with external services (calendars, email, finance, etc.). The Canon defines this as a certification system, not an app store.

Capabilities must:
- Respect circle sovereignty
- Operate only within granted authority
- Follow the irreducible loop
- Be auditable and revocable

The marketplace exists from day one with the **QuantumLife Seal** as the trust marker.

---

## Decision

### Capability Manifest Format

Every capability is defined by a signed manifest (YAML):

```yaml
id: "com.quantumlife.calendar-sync"
version: "1.2.0"
name: "Calendar Sync"
description: "Syncs calendar events with Google and Microsoft"

scopes_requested:
  - "read:calendar"
  - "write:calendar.events"
  - "execute:booking.confirm"

risk_class: "standard"  # standard | elevated | high

audit_hooks:
  - on_action: "log_event_created"
  - on_failure: "log_sync_failure"

constraints:
  max_frequency: "10/minute"
  requires_intersection: true

signature:
  algorithm: "Ed25519"  # or PQ-hybrid
  signer: "QuantumLayer Platform Ltd"
  timestamp: "2025-12-29T10:00:00Z"
  value: "base64-encoded-signature"

provenance:
  source_repo: "https://github.com/quantumlayer/calendar-sync"
  commit_hash: "abc123..."
  build_id: "build-456"
```

### Required Fields

| Field | Purpose |
|-------|---------|
| `id` | Unique identifier (reverse-domain) |
| `version` | Semantic version |
| `scopes_requested` | What authority the capability needs |
| `risk_class` | Determines approval flow |
| `audit_hooks` | What events are logged |
| `signature` | Cryptographic attestation |
| `provenance` | Where it came from |

---

### Seal Certification

The **QuantumLife Seal** certifies that:

1. Capability respects circle sovereignty
2. Operates only within granted authority (no self-authorization)
3. Follows the irreducible loop
4. All actions are auditable
5. Is revocable

### Certification Process

```
Capability submitted
    │
    ▼
Automated validation
  - Manifest schema check
  - Scope analysis
  - Risk class validation
    │
    ▼
Manual review (for elevated/high risk)
    │
    ▼
Signing by QuantumLayer
    │
    ▼
Published to registry
```

---

### Registry Layout

OCI-like registry structure:

```
registry.quantumlife.io/
  └── capabilities/
      └── com.quantumlife.calendar-sync/
          ├── manifest.yaml
          ├── manifest.yaml.sig
          └── versions/
              ├── 1.0.0/
              ├── 1.1.0/
              └── 1.2.0/
```

- HTTPS access
- Manifest signature verified on pull
- Version immutability (no overwriting published versions)

---

### Runtime Gating

| Seal Status | Behavior |
|-------------|----------|
| **Certified** | Agent can propose use; authority still required from circle |
| **Uncertified** | Explicit human approval required before first use |
| **Revoked** | Cannot be used; existing grants invalidated |

### Validator Behavior

```go
func ValidateCapability(manifest Manifest) error {
    // 1. Verify signature
    if !VerifySignature(manifest) {
        return ErrInvalidSignature
    }

    // 2. Check seal status in registry
    status := registry.GetSealStatus(manifest.ID, manifest.Version)
    if status == Revoked {
        return ErrCapabilityRevoked
    }

    // 3. Validate scopes against circle's authority grants
    if !circle.HasGrantedScopes(manifest.ScopesRequested) {
        return ErrInsufficientAuthority
    }

    // 4. Check risk class approval
    if manifest.RiskClass == "high" && !circle.HasHighRiskApproval() {
        return ErrHighRiskNotApproved
    }

    return nil
}
```

---

## Alternatives Considered

### App Store Model
- **Pros:** Familiar to users
- **Cons:** Implies platform control, silent updates, permission drift
- **Verdict:** Rejected; certification model preserves sovereignty

### No Marketplace (BYO Integrations)
- **Pros:** Maximum flexibility
- **Cons:** No trust signals, fragmented ecosystem, security risk
- **Verdict:** Rejected; seal provides trust baseline

### Blockchain-Based Registry
- **Pros:** Decentralized, tamper-proof
- **Cons:** Complexity, latency, overkill for this use case
- **Verdict:** Rejected; signed manifests sufficient

### Plugin System (Runtime Injection)
- **Pros:** Dynamic loading
- **Cons:** Security risk, harder to audit
- **Verdict:** Rejected; capabilities are external services, not injected code

---

## Consequences

### Positive
- Trust signal from day one (Seal)
- Capability ecosystem can grow without compromising sovereignty
- All capability actions auditable
- Revocation is instant and effective

### Negative
- Certification process adds friction for capability developers
- QuantumLayer becomes trust anchor (responsibility)
- Registry infrastructure to maintain

### Mitigations
- Automated validation for most submissions
- Clear certification criteria published
- Registry backed by Azure Blob + CDN for availability

---

## Canon & Technical Split Alignment

| Requirement | How Seal Satisfies |
|-------------|-------------------|
| Capabilities respect sovereignty (Canon §Marketplace) | Seal certifies compliance |
| Only circles grant authority (Canon §Ontology) | Capabilities request; circles approve |
| Auditable (Guarantees §8) | All capability actions logged |
| Revocable (Guarantees §5) | Revoked seal invalidates capability |
| No self-authorization (Split §3.8) | Validator enforces scope check |

---

## References

- `docs/QUANTUMLIFE_CANON_V1.md` — §Marketplace: The QuantumLife Seal
- `docs/TECHNICAL_SPLIT_V1.md` — §3.8 Marketplace & Seal Validation Layer
- `docs/HUMAN_GUARANTEES_V1.md` — §8 Marketplace & Seal Guarantees
