# ADR-0074: Phase 37 - iOS Device Registration + Deep-Link Receipt Landing

**Status:** Accepted
**Date:** 2026-01-08
**Phase:** 37

## Context

QuantumLife has established a complete interrupt pipeline (Phases 31-36) with sealed APNs secret
handling (Phase 35b). To complete the real-device experience, we need:

1. iOS app to register its APNs device token with the server
2. Server to store raw tokens ONLY in the sealed secret boundary
3. Deep links to land on correct proof/preview screens
4. Web UI proof page showing registration happened (hash-only)

## Decision

### Why Sealed Secrets

Raw device tokens are sensitive credentials that can be used to push arbitrary content to a device.
They MUST be encrypted at rest and accessible only during actual push delivery.

From Phase 35b, we already have:
- `internal/persist/sealed_secret_store.go`: AES-GCM encrypted storage
- `internal/pushtransport/transport/apns.go`: Consumes raw tokens at delivery time

This phase extends that pattern:
- Registration handler seals tokens immediately after receipt
- Only token_hash and sealed_ref_hash stored in registration records
- Raw token NEVER persists outside sealed boundary

### Why No Identifiers in Deep Links

Deep links could be intercepted or logged. Including circle IDs, hashes, or user identifiers would:
- Leak information about which circles receive interrupts
- Enable correlation attacks across push notifications
- Violate our hash-only-in-transport principle

Instead, deep links use abstract target types:
- `quantumlife://open?t=interrupts` → /interrupts/preview
- `quantumlife://open?t=trust` → /trust/action/receipt
- `quantumlife://open?t=shadow` → /shadow/receipt
- `quantumlife://open?t=reality` → /reality
- `quantumlife://open?t=today` → /today

The app determines which screen to show based on LOCAL state, not URL parameters.

### How This Preserves Trust While Enabling Real Delivery

1. **Registration is explicit**: User presses a button, not auto-registered
2. **Registration ≠ interrupts enabled**: Device token sealed ≠ permission to interrupt
3. **Push content is abstract**: "QuantumLife" / "Something needs you. Open QuantumLife."
4. **Landing is deterministic**: Same local state → same screen, no server-controlled routing
5. **Proof is hash-only**: Web UI shows token_hash prefix, not actual token

## Specification

### Domain Model (pkg/domain/devicereg/types.go)

```go
// DevicePlatform identifies the device operating system
type DevicePlatform string
const DevicePlatformIOS DevicePlatform = "ios"

// DeviceRegState indicates registration state
type DeviceRegState string
const (
    DeviceRegStateRegistered DeviceRegState = "registered"
    DeviceRegStateRevoked    DeviceRegState = "revoked"
)

// DeviceRegistrationReceipt is the hash-only record of a registration
type DeviceRegistrationReceipt struct {
    PeriodKey      string
    Platform       DevicePlatform
    CircleIDHash   string
    TokenHash      string
    SealedRefHash  string
    State          DeviceRegState
    StatusHash     string
}

// DeepLinkTarget enumerates valid landing screens
type DeepLinkTarget string
const (
    DeepLinkTargetInterrupts DeepLinkTarget = "interrupts"
    DeepLinkTargetTrust      DeepLinkTarget = "trust"
    DeepLinkTargetShadow     DeepLinkTarget = "shadow"
    DeepLinkTargetReality    DeepLinkTarget = "reality"
    DeepLinkTargetToday      DeepLinkTarget = "today"
)
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/devices` | GET | Calm registration page |
| `/devices/register` | POST | Seal token, store hashes |
| `/proof/device` | GET | Show registration proof (hash-only) |
| `/open` | GET | Deep link landing redirect |

### Events

- `phase37.device.register.requested`
- `phase37.device.sealed`
- `phase37.device.registered`
- `phase37.device.proof.viewed`
- `phase37.open.redirected`

### iOS Changes

- Register button on Connections screen
- APNs token retrieval (UNUserNotificationCenter)
- URL scheme `quantumlife://open?t=...`
- Deep link routing to existing screens

## Consequences

### Positive
- Real iOS devices can receive pushes
- Token security maintained via sealed secrets
- No identifiers leak through deep links
- Proof pages provide transparency

### Negative
- Additional complexity in iOS app
- Deep links limited to predefined targets
- Registration is manual (user must press button)

### Risks Mitigated
- Token theft: Encrypted at rest, never logged
- Correlation attacks: No identifiers in URLs
- Unauthorized delivery: Registration ≠ permission

## References

- Phase 35b: docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md
- Phase 36: docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md
- Canon v9.6: Hash-only storage invariant
