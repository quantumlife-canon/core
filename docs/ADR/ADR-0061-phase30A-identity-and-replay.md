# ADR-0061: Phase 30A — Identity + Replay

## Status

Accepted

## Context

QuantumLife needs device-rooted identity to enable secure replay bundle export/import between devices. This allows users to:

1. Restore state on a new device
2. Share state between devices in a circle (family/household)
3. Prove ownership of data without exposing raw identifiers

The identity system must:
- Use stdlib-only cryptography (Ed25519)
- Store only hashes, never raw identifiers
- Bound the number of devices per circle
- Require explicit user action for binding and export/import
- Prevent replay attacks using time-bucketed signatures

## Decision

### 1. Ed25519 Device Identity

Each device generates a unique Ed25519 keypair on first use:
- Private key stored locally with 0600 permissions
- Public key used to derive a fingerprint (SHA256, 64 hex chars)
- Fingerprint is the only identifier that leaves the device

**Why Ed25519:**
- Part of Go stdlib (crypto/ed25519)
- Fast, secure, deterministic signatures
- No cloud SDK dependencies

### 2. Device Fingerprint

```
Fingerprint = SHA256(PublicKey)
```

- 64 hex characters (256 bits)
- Deterministic: same key always produces same fingerprint
- One-way: cannot derive key from fingerprint

### 3. PeriodKey for Replay Protection

Each signed request includes a PeriodKey:
- Format: `YYYY-MM-DDTHH:MM` (floored to 15-minute buckets)
- Valid for current period and previous period (30-minute window)
- Prevents replay attacks outside this window

### 4. Circle Binding

Devices bind to circles by fingerprint:
- Maximum 5 devices per circle
- Binding is idempotent (same fingerprint can rebind)
- Only fingerprints stored, never raw public keys
- Stored with hash of circle ID (not raw ID)

### 5. Signed Requests

Export/import requires signed requests:
```
SignedRequest {
    Method:    "POST"
    Path:      "/replay/export"
    BodyHash:  SHA256(body)
    PeriodKey: "2025-01-15T10:30"
    PublicKey: hex(ed25519.PublicKey)
    Signature: hex(ed25519.Sign(canonical_string))
}
```

### 6. Replay Bundle Format

Bundles use pipe-delimited canonical format (NOT JSON):
```
v1|CIRCLE_ID_HASH|2025-01-15T10:30|RECORD_COUNT|EARLIEST|LATEST
RECORD_TYPE|RECORD_HASH|PERIOD_BUCKET|PAYLOAD_HASH
RECORD_TYPE|RECORD_HASH|PERIOD_BUCKET|PAYLOAD_HASH
...
BUNDLE_HASH
```

**Why not JSON:**
- Deterministic ordering (no object key ordering issues)
- Simpler parsing
- Hash stability guaranteed

### 7. Safe Record Types

Only hash-only record types are exported:
- SHADOWLLM_RECEIPT
- SHADOW_DIFF
- SHADOW_CALIBRATION
- REALITY_ACK
- JOURNEY_DISMISSAL
- FIRST_MINUTES_SUMMARY
- FIRST_MINUTES_DISMISSAL
- SHADOW_RECEIPT_ACK
- SHADOW_RECEIPT_VOTE
- TRUST_ACTION_RECEIPT
- TRUST_ACTION_UPDATE
- UNDO_EXEC_RECORD
- UNDO_EXEC_ACK
- FINANCE_SYNC_RECEIPT
- FINANCE_MIRROR_ACK
- CIRCLE_BINDING

Record types containing raw data (EVENT, DRAFT, APPROVAL) are never exported.

### 8. Forbidden Patterns

Bundles are scanned for forbidden patterns:
- Email addresses (@)
- URLs (http://, https://)
- Currency symbols and amounts
- Merchant names

Any match rejects the bundle.

### 9. Bounded Retention

- Default: 30 days
- Configurable per export
- Records older than cutoff are excluded

## Package Structure

```
pkg/domain/deviceidentity/types.go  # Domain model
pkg/domain/replay/types.go          # Replay bundle model
internal/persist/device_key_store.go    # Key generation/storage
internal/persist/circle_binding_store.go # Circle bindings
internal/deviceidentity/engine.go   # Identity operations
internal/replay/engine.go           # Bundle build/validate/import
```

## Web Routes

- `GET /identity` - View device identity and binding status
- `POST /identity/bind` - Bind device to circle
- `GET /replay/export` - Export form
- `POST /replay/export` - Download bundle
- `GET /replay/import` - Import form
- `POST /replay/import` - Validate and import bundle

## Events

- `phase30A.identity.created` - Keypair generated
- `phase30A.identity.viewed` - Identity page viewed
- `phase30A.identity.bound` - Device bound to circle
- `phase30A.replay.exported` - Bundle exported
- `phase30A.replay.imported` - Bundle imported
- `phase30A.replay.rejected` - Bundle rejected

## Invariants

1. **stdlib only** - No cloud crypto SDKs
2. **No time.Now()** - Clock injection everywhere
3. **No goroutines** - All operations synchronous
4. **Hash-only storage** - Never raw identifiers
5. **Max 5 devices** - Per circle
6. **30-day default** - Bounded retention
7. **POST only** - For sensitive operations
8. **Pipe-delimited** - Not JSON
9. **Forbidden patterns** - Block raw data
10. **0600 permissions** - Private key security

## Guardrails

`scripts/guardrails/identity_replay_enforced.sh` verifies:
- Ed25519 cryptography usage
- Domain model types
- Max devices constant
- Retention constant
- Pipe-delimited format
- Safe record types whitelist
- Forbidden patterns check
- No goroutines
- No time.Now()
- Clock injection
- Hash-only storage
- Signature verification
- Private key security
- Deterministic bundle hash

## Consequences

### Positive
- Secure device identity without cloud dependencies
- Deterministic, auditable bundles
- Privacy preserved (hash-only)
- Bounded complexity (max 5 devices)
- Replay attack protection

### Negative
- Keypair must be backed up by user
- Lost key = lost device identity
- 30-minute signature window limits offline use

### Neutral
- Devices must bind before export/import
- Empty bundles are valid

## Related ADRs

- ADR-0027: Persistence + Replay (Phase 12) - Storelog foundation
- ADR-0044: Phase 19.3 - Azure OpenAI Shadow Provider
- ADR-0059: Phase 28 - Trust Kept
