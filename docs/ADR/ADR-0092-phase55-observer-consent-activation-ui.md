# ADR-0092: Phase 55 - Observer Consent Activation UI

## Status
Accepted

## Context

Users need explicit control over which observer capabilities are active. The current implicit
consent model (via OAuth or pack installation) doesn't give users clear visibility into what
is being observed, leading to confusion about whether the system is working.

Phase 55 provides a proof-first, explicit consent UI that lets circles enable/disable
specific observer capabilities. This integrates with the existing Coverage Plan mechanism
(Phase 47) without adding any new "power" to the system.

## Problem

1. **Implicit consent is confusing** - Users don't know what observers are active
2. **No explicit toggle UI** - No way to enable/disable individual observer capabilities
3. **Observer vs Action confusion** - Users conflate observation with interruption/execution
4. **No proof of consent state** - Users can't verify what they've consented to

## Decision

### Core Principle: No New Power

Phase 55 adds ZERO new power to the system:
- No new delivery paths
- No new execution paths
- No new interrupts
- No new policy escalation
- No notifications
- No changes to decision logic in any phase

**Consent controls only what the system MAY observe. It never changes what the system MAY do.**

### Observer Capability Allowlist

The following capabilities from the Coverage Plan can be toggled via Phase 55:

| Capability | Description | Source |
|------------|-------------|--------|
| `cap_receipt_observer` | Observe receipt patterns in email | Gmail API |
| `cap_commerce_observer` | Observe commerce patterns | Composite |
| `cap_finance_commerce_observer` | Observe commerce from finance data | TrueLayer API |
| `cap_notification_metadata` | Observe notification metadata | Device push |

Non-observer capabilities (e.g., `cap_pressure_map`, `cap_timewindow_sources`) are NOT
toggleable via this UI - they are derived capabilities, not direct observation consent.

### Consent States

| State | Meaning |
|-------|---------|
| `applied` | Consent request processed; capability state changed |
| `no_change` | Already in requested state; no change needed |
| `rejected` | Request invalid (bad capability, missing circle, etc.) |

### Reject Reasons

| Reason | When |
|--------|------|
| `reject_invalid` | Invalid capability or action |
| `reject_not_allowlisted` | Capability not in Phase 55 allowlist |
| `reject_missing_circle` | Circle ID not provided or invalid |
| `reject_period_invalid` | Client tried to supply period (forbidden) |

### Hash-Only Storage

All stored records use hash-only identifiers:
- `CircleIDHash` - SHA256 hash of circle ID
- `ReceiptHash` - SHA256 hash of canonical receipt string
- `PeriodKey` - Server-derived period (YYYY-MM-DD)

No raw identifiers, emails, tokens, or timestamps in storage.

### Bounded Retention

- Maximum 200 records per store
- Maximum 30 days retention
- FIFO eviction when limits exceeded

### Period Key Derivation

Period key is ALWAYS derived server-side from the injected clock:
```go
periodKey := s.clk().Format("2006-01-02")
```

Clients MUST NOT supply period. If any period-related field is provided in the request,
the request is rejected with `reject_period_invalid`.

### Forbidden Client Fields

The following fields are rejected if provided by the client:
- `period`, `periodKey`, `period_key`
- `periodKeyHash`
- `email`, `url`, `name`, `vendor`, `token`, `device`

## Non-Goals

1. **No automatic sync on consent** - Enabling an observer doesn't trigger sync
2. **No consent-based decision changes** - Consent is observation-only
3. **No new notification types** - No consent-related interrupts
4. **No blanket consent** - Each capability is toggled individually

## Architecture

### Domain Types (pkg/domain/observerconsent/)

Enums:
- `ObserverKind` - receipt, calendar, finance_commerce, commerce, notification, device_hint, unknown
- `ConsentAction` - enable, disable
- `ConsentResult` - applied, no_change, rejected
- `RejectReason` - reject_invalid, reject_not_allowlisted, reject_missing_circle, reject_period_invalid

Structs:
- `ObserverConsentRequest` - Request to enable/disable a capability
- `ObserverConsentReceipt` - Immutable record of consent action
- `ObserverConsentAck` - Proof dismissal record

### Engine (internal/observerconsent/)

Pure deterministic methods:
- `ApplyConsent(periodKey, currentCaps, req)` -> (newCaps, receipt)
- `IsAllowlisted(cap)` -> bool
- `KindFromCapability(cap)` -> ObserverKind

Rules:
1. Validate request
2. Check capability is allowlisted
3. Derive observer kind from capability
4. Apply enable/disable logic
5. Return receipt (always)

### Persistence (internal/persist/)

Stores:
- `ObserverConsentStore` - Append-only receipt store
- `ObserverConsentAckStore` - Proof dismissal store

Both use:
- Dedup keys for idempotency
- FIFO eviction for bounded retention
- Hash-only fields

### Web Routes

| Route | Method | Description |
|-------|--------|-------------|
| `/settings/observers` | GET | Observer consent settings page |
| `/settings/observers/enable` | POST | Enable observer capability |
| `/settings/observers/disable` | POST | Disable observer capability |
| `/proof/observers` | GET | Observer consent proof page |
| `/proof/observers/dismiss` | POST | Dismiss proof page |

### Events

| Event | When Emitted |
|-------|--------------|
| `phase55.observer_consent.page.rendered` | Settings page viewed |
| `phase55.observer_consent.requested` | Consent request received |
| `phase55.observer_consent.applied` | Consent applied successfully |
| `phase55.observer_consent.rejected` | Consent request rejected |
| `phase55.observer_consent.persisted` | Receipt persisted |
| `phase55.observer_consent.proof.rendered` | Proof page viewed |
| `phase55.observer_consent.ack.dismissed` | Proof dismissed |

### Storelog Records

| Record Type | When Written |
|-------------|--------------|
| `OBSERVER_CONSENT_RECEIPT` | After consent toggle |
| `OBSERVER_CONSENT_ACK` | After proof dismissal |

## Consequences

### Positive
- Users have explicit control over observation
- Clear proof of consent state
- Integrates with existing Coverage Plan mechanism
- No new power added to system

### Negative
- Additional UI surface to maintain
- Two-layer model (Coverage Plan + Consent) may confuse users
  - Mitigation: UI clearly explains "what can be observed"

### Risks
- Users might disable all observers and wonder why features don't work
  - Mitigation: Calm messaging explains consequences

## References

- [ADR-0085](./ADR-0085-phase47-pack-coverage-realization.md) - Phase 47 Coverage Plan
- [ADR-0091](./ADR-0091-phase53-urgency-resolution-layer.md) - Phase 53 Urgency Resolution
