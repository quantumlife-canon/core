# ADR-0072: Phase 35b — APNs Push Transport with Sealed Secret Boundary

## Status
Accepted

## Date
2026-01-07

## Context

Phase 35 introduced abstract push transport with the invariant that raw tokens are NEVER stored — only token hashes. This works for webhook transport where the endpoint URL can be reconstructed, but APNs device tokens present a unique challenge:

1. **APNs requires the raw device token** to deliver a push notification
2. **Device tokens are opaque binary blobs** — they cannot be reconstructed from a hash
3. **Tokens are device-specific secrets** — if leaked, attackers could target specific devices

The hash-only storage model that protects privacy in other contexts is insufficient for APNs because we genuinely need the raw token to send a push.

## Decision

Introduce a **Sealed Secret Boundary** — a formally documented exception to hash-only storage that allows encrypted storage of APNs device tokens under strict constraints.

### Why This Does NOT Weaken Trust Guarantees

1. **Narrower than execution boundaries** — This boundary only handles device tokens, not transaction amounts, recipient identities, or decision logic. Execution boundaries (Phase 17, 28) handle far more sensitive data.

2. **Encryption at rest** — Raw tokens are AES-GCM encrypted using a symmetric key from environment. Even if the store file is compromised, tokens remain protected.

3. **Minimal access surface** — Only the APNs transport implementation (`apns.go`) may decrypt tokens, and only at the moment of delivery. No other code path has access.

4. **No metadata leakage** — The sealed store is indexed ONLY by token_hash. There is no way to query "all tokens for circle X" or "all tokens for device Y". The relationship between token and identity exists only in the registration store (which stores only hashes).

5. **Audit trail preserved** — All delivery attempts are recorded with token_hash, not raw token. Proof pages show delivery status without exposing device identifiers.

6. **Reversible at UI level** — Users can disable push at any time. The sealed store supports deletion by token_hash.

### Why Hash-Only Storage Is Insufficient for APNs

Unlike other transport mechanisms:
- **Webhook URLs** can be stored as hashes because the full URL is provided in the request
- **Email addresses** can be derived from identity resolution
- **APNs tokens** are opaque, device-generated, and non-derivable

Without the raw token, push delivery is impossible. This is a fundamental constraint of the APNs protocol.

### Boundary Definition

The Sealed Secret Boundary is defined as:

```
┌─────────────────────────────────────────────────────────────────┐
│                    SEALED SECRET BOUNDARY                        │
├─────────────────────────────────────────────────────────────────┤
│  internal/persist/sealed_secret_store.go                        │
│  internal/pushtransport/transport/apns.go                       │
│                                                                  │
│  ALLOWED:                                                        │
│    - Store/load encrypted blobs                                  │
│    - Decrypt token ONLY in apns.go Send()                        │
│    - Delete by token_hash                                        │
│                                                                  │
│  FORBIDDEN:                                                      │
│    - Query by circle, user, or device                            │
│    - Log raw tokens                                              │
│    - Include tokens in events                                    │
│    - Pass tokens outside boundary                                │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation

### Sealed Secret Store

```go
// internal/persist/sealed_secret_store.go

type SealedSecretStore struct {
    encryptionKey []byte  // 32 bytes, from QL_SEALED_SECRET_KEY
    dataDir       string  // File storage directory
}

// StoreEncrypted stores an encrypted blob indexed by token_hash.
// The blob is AES-GCM encrypted before this call.
func (s *SealedSecretStore) StoreEncrypted(tokenHash string, encryptedBlob []byte) error

// LoadEncrypted retrieves an encrypted blob by token_hash.
func (s *SealedSecretStore) LoadEncrypted(tokenHash string) ([]byte, error)

// DeleteEncrypted removes an encrypted blob by token_hash.
func (s *SealedSecretStore) DeleteEncrypted(tokenHash string) error
```

File storage:
- Directory: `$QL_DATA_DIR/sealed/`
- File naming: `{token_hash}.sealed`
- Permissions: 0600 (owner read/write only)

### APNs Transport

```go
// internal/pushtransport/transport/apns.go

type APNsTransport struct {
    sealedStore *persist.SealedSecretStore
    keyID       string  // From QL_APNS_KEY_ID
    teamID      string  // From QL_APNS_TEAM_ID
    bundleID    string  // From QL_APNS_BUNDLE_ID
    // Private key for JWT signing (P-256)
}

func (t *APNsTransport) Send(ctx context.Context, req *TransportRequest) (*TransportResult, error) {
    // 1. Load encrypted blob from sealed store using req.TokenHash
    // 2. Decrypt to get raw device token
    // 3. Build JWT for APNs authentication
    // 4. Send single HTTP/2 request to APNs
    // 5. Return result (never expose token in result)
}
```

### Payload Specification

The APNs payload is a constant literal:

```json
{
  "aps": {
    "alert": {
      "title": "QuantumLife",
      "body": "Something needs you. Open QuantumLife."
    },
    "sound": "default"
  }
}
```

No dynamic fields. No identifiers. No candidate details.

### Device Registration Flow

1. iOS app generates device token via `registerForRemoteNotifications()`
2. App signs registration request with Ed25519 key (Phase 30A)
3. Server verifies signature
4. Server computes `token_hash = SHA256(device_token)`
5. Server encrypts `device_token` with AES-GCM
6. Server stores encrypted blob in sealed store (indexed by token_hash)
7. Server stores registration in push_registration_store (token_hash only)

### Encryption Details

- Algorithm: AES-256-GCM
- Key: 32 bytes from `QL_SEALED_SECRET_KEY` (base64 encoded)
- Nonce: 12 bytes, randomly generated per encryption
- Format: `nonce || ciphertext || tag`

## Invariants

### CRITICAL — Sealed Boundary Constraints
- Raw device_token ONLY in sealed_secret_store.go and apns.go
- No device_token in pkg/, events, logs, storelog, receipts
- Encrypted blob never leaves sealed store (except to apns.go)
- AES-GCM encryption mandatory
- File permissions 0600

### Standard Canon Invariants
- No goroutines in internal/ or pkg/
- No time.Now() — clock injection required
- stdlib-only (no Apple SDK)
- No new decision logic (uses Phase 33/34 output)
- Daily delivery cap preserved (max 2/day from Phase 35)
- Commerce never interrupts

## Consequences

### Positive
- Real iOS push delivery enabled
- Privacy preserved through encryption and minimal access
- Audit trail maintained with hash-only logging
- Formally documented exception with clear boundaries
- No weakening of trust guarantees for other data

### Negative
- Introduces encrypted storage (first exception to hash-only)
- Requires key management (QL_SEALED_SECRET_KEY)
- APNs authentication complexity (JWT, HTTP/2)

### Risks
- Key compromise would expose device tokens (mitigated by file permissions, env-only key)
- APNs certificate rotation required annually (operational concern)

## References

- ADR-0071: Phase 35 Push Transport (Abstract Interrupt Delivery)
- ADR-0069: Phase 33 Interrupt Permission Contract
- ADR-0061: Phase 30A Identity + Replay (Ed25519 signing)
- Apple Push Notification service (APNs) documentation
