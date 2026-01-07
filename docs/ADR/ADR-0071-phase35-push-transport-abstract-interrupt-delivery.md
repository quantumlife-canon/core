# ADR-0071: Phase 35 — Push Transport (Abstract Interrupt Delivery)

## Status
Accepted

## Date
2026-01-07

## Context

Phase 33 introduced interrupt permission policies (user controls who/what can interrupt). Phase 34 added web-only preview of permitted interrupt candidates. However, web-only preview requires the user to actively check /today — it cannot reach them when they're away from the browser.

We now need a real-world "delivery" path for permitted interrupts. However, push notifications are inherently invasive and can break trust if misused. Traditional notifications contain identifying information (sender names, subject lines, amounts) that violate privacy and create anxiety.

## Decision

Implement Phase 35: Push Transport as a **transport-only** layer that delivers permitted interrupt candidates in an **abstract, non-identifying way**.

### Why Abstract Payload Only

1. **Privacy by design** — No names, merchants, amounts, subjects, or timestamps in push body
2. **Anti-anxiety** — Generic message prevents catastrophizing ("Something needs you" vs "URGENT: $5,000 overdue!")
3. **Trust preservation** — User knows the push contains no identifying data that could be intercepted
4. **Uniform appearance** — All pushes look identical; cannot be distinguished by observers

### Push Payload Specification

```
Title: "QuantumLife"
Body: "Something needs you. Open QuantumLife."
Data: { status_hash: "<opaque_sha256>" }
```

No additional fields. No customization. This is intentional.

### Why Transport-Only Separation

1. **Single responsibility** — Transport layer only moves data; it does not decide what to send
2. **Testability** — Transport can be stubbed/mocked without affecting decision logic
3. **Auditability** — Decision made by Phase 33/34 engines; transport only executes
4. **No engagement** — Transport cannot add urgency, personalization, or nudges

### Delivery Eligibility (from Phase 33/34, NOT new logic)

Delivery is allowed ONLY if:
1. User has explicitly enabled interrupt policy (Phase 33)
2. Candidate is eligible per Phase 34 preview rules
3. Daily cap not exceeded (max 2/day)
4. Push transport channel is explicitly enabled
5. Device is registered

If any condition fails, attempt is **skipped** (not failed) with a reason bucket.

## Implementation

### Domain Model

```go
// PushProviderKind identifies the transport mechanism
type PushProviderKind string
const (
    ProviderAPNs    PushProviderKind = "apns"
    ProviderWebhook PushProviderKind = "webhook"
    ProviderStub    PushProviderKind = "stub"
)

// PushRegistration stores device registration (hash-only)
type PushRegistration struct {
    CircleIDHash          string
    DeviceFingerprintHash string
    ProviderKind          PushProviderKind
    TokenHash             string  // NEVER raw token
    CreatedPeriodKey      string
}

// PushDeliveryAttempt records a delivery attempt
type PushDeliveryAttempt struct {
    AttemptID       string
    CircleIDHash    string
    CandidateHash   string
    ProviderKind    PushProviderKind
    Status          AttemptStatus  // sent|skipped|failed
    FailureBucket   FailureBucket  // why skipped/failed
    StatusHash      string
    PeriodKey       string
}
```

### Transport Interface

```go
type Transport interface {
    ProviderKind() PushProviderKind
    Send(ctx context.Context, req TransportRequest) (TransportResult, error)
}
```

Implementations:
- **StubTransport** — Deterministic, no network (for testing)
- **WebhookTransport** — Posts to configured endpoint (for dev/testing)
- **APNsTransport** — (Optional) Token-based auth with stdlib net/http

### Storage

- Hash-only, append-only, 30-day bounded retention
- One active registration per circle per provider kind
- Attempt deduplication: same (circle+candidate+period) = same attempt ID
- Storelog integration for replay

### Web Routes

- `POST /push/register` — Register device token (hashed before storage)
- `POST /push/send` — Trigger single delivery attempt
- `GET /proof/push` — View delivery proof (abstract only)

## Invariants

### CRITICAL — Transport-Only
- NO new decision logic (uses Phase 33/34 output)
- NO changes to obligations, drafts, execution, or trust engines
- NO engagement mechanics (no badges, no escalation)

### CRITICAL — Privacy
- Notification body is constant string literal
- No identifying information in push payload
- Token is hashed before storage
- Proof page shows only abstract buckets and hashes

### Standard Canon Invariants
- No goroutines in internal/ or pkg/
- No time.Now() — clock injection required
- stdlib-only in internal/ and pkg/
- Commerce never interrupts

## Future Work

Phase 36+ may implement:
1. APNs real integration with iOS token registration
2. Scheduled delivery (not user-triggered)
3. FCM support for Android
4. Delivery confirmation from device

But ONLY after this transport-only layer proves the pattern works.

## Consequences

### Positive
- Real-world urgency without engagement mechanics
- Privacy preserved through abstract payload
- Clean separation between decision and transport
- Testable with stub/webhook transports

### Negative
- User must actively register device (by design)
- Generic message may feel impersonal (acceptable — trust over UX)
- No delivery confirmation in this phase

### Risks
- User may expect traditional notifications (mitigated by clear settings UI)
- Push may be blocked by OS (acceptable — graceful degradation)

## References

- ADR-0069: Phase 33 Interrupt Permission Contract
- ADR-0070: Phase 34 Permitted Interrupt Preview
- ADR-0061: Phase 30A Identity + Replay (DeviceFingerprintHash)
