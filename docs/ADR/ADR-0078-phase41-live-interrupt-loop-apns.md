# ADR-0078: Phase 41 - Live Interrupt Loop (APNs)

## Status
Accepted

## Context

Phase 33-37 established:
- Phase 33: Interrupt Permission Contract (policy evaluation)
- Phase 34: Permitted Interrupt Preview (web-only candidates)
- Phase 35: Push Transport (abstract delivery mechanism)
- Phase 35b: APNs Sealed Secret Boundary (token secrecy)
- Phase 36: Interrupt Delivery Orchestrator (deterministic selection)
- Phase 37: Device Registration + Deep-Link (iOS registration flow)

However, there was no end-to-end rehearsal capability. Users could:
- Register a device (Phase 37)
- Configure interrupt policy (Phase 33)
- View preview candidates (Phase 34)

But they could not:
- Test that the entire delivery chain works
- Verify APNs credentials are correctly configured
- Validate that pushes actually arrive on their device
- See proof of delivery attempts

### Why Rehearsal?

A "rehearsal" is a manual, user-triggered delivery that:
1. Uses the full delivery pipeline (preview → policy → transport)
2. Sends a real push notification (if APNs configured)
3. Records proof of the attempt
4. Allows users to validate configuration before depending on it

This is NOT automatic background delivery. It is:
- POST-triggered only (user clicks "Send rehearsal")
- Limited to 2/day (same cap as real interrupts)
- Abstract payload only (no content, no identifiers)
- Fully audited (hash-only receipts)

### Why "Live" Matters

The rehearsal must use the real APNs transport when configured. This:
- Validates APNs credentials work
- Proves the sealed boundary is correctly implemented
- Gives users confidence the system can deliver when needed
- Surfaces any transport issues before they matter

## Decision

Implement Phase 41 as a **manual rehearsal delivery loop**:

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                  Phase 41: Live Interrupt Loop                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User clicks "Send rehearsal push"                                  │
│                      │                                              │
│                      ▼                                              │
│  ┌─────────────────────────────────────────┐                       │
│  │  1. Check Eligibility (Engine)           │                       │
│  │     - Device registered? (Phase 37)      │                       │
│  │     - Policy allows? (Phase 33)          │                       │
│  │     - Candidate available? (Phase 34)    │                       │
│  │     - Rate limit OK? (2/day cap)         │                       │
│  │     - Transport available?               │                       │
│  │     - Sealed key ready? (Phase 35b)      │                       │
│  └─────────────────────────────────────────┘                       │
│                      │                                              │
│           ┌─────────┴─────────┐                                    │
│           │                   │                                    │
│     Rejected            Eligible                                   │
│           │                   │                                    │
│           ▼                   ▼                                    │
│  ┌───────────────┐   ┌───────────────────────────────┐            │
│  │ Store receipt │   │  2. Build Plan (Engine)        │            │
│  │ (rejected)    │   │     - Deterministic attempt ID │            │
│  └───────────────┘   │     - Abstract payload only    │            │
│                      │     - Deep link: t=interrupts  │            │
│                      └───────────────────────────────┘            │
│                               │                                    │
│                               ▼                                    │
│                      ┌───────────────────────────────┐            │
│                      │  3. Execute Delivery (cmd/)    │            │
│                      │     - Call Phase 35 transport  │            │
│                      │     - Measure latency          │            │
│                      │     - Handle response          │            │
│                      └───────────────────────────────┘            │
│                               │                                    │
│                               ▼                                    │
│                      ┌───────────────────────────────┐            │
│                      │  4. Finalize (Engine)          │            │
│                      │     - Map latency to bucket    │            │
│                      │     - Determine final status   │            │
│                      │     - Compute status hash      │            │
│                      └───────────────────────────────┘            │
│                               │                                    │
│                               ▼                                    │
│                      ┌───────────────────────────────┐            │
│                      │  5. Store Receipt + Redirect   │            │
│                      │     → /proof/interrupts/rehearse│            │
│                      └───────────────────────────────┘            │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Web Routes

1. **GET /interrupts/rehearse**
   - Shows current eligibility status
   - Primary button: "Send one rehearsal push"
   - Secondary: "View proof"
   - No automatic actions

2. **POST /interrupts/rehearse/send**
   - Only path that triggers real APNs call
   - Evaluates eligibility → builds plan → executes → finalizes
   - Redirects to proof page

3. **GET /proof/interrupts/rehearse**
   - Shows latest receipt as proof
   - Abstract status only (no raw data)
   - Hash prefixes for verification

4. **POST /proof/interrupts/rehearse/dismiss**
   - Records acknowledgment
   - Redirects to /today

### Reject Reasons

Rehearsal can be rejected for:
- `reject_no_device`: No device registered
- `reject_policy_disallows`: Interrupt policy is allow_none
- `reject_no_candidate`: No interrupt preview candidate
- `reject_rate_limited`: 2/day cap reached
- `reject_transport_unavailable`: No transport configured
- `reject_sealed_key_missing`: APNs selected but sealed key not ready

### Abstract Payload

The push notification uses ONLY constant literals:
- Title: "QuantumLife"
- Body: "Something needs you. Open QuantumLife."
- Deep link: `t=interrupts` (opens /interrupts/preview)

No merchant names. No email addresses. No amounts. No subject lines.

### Single Whisper Rule

Phase 41 adds NO new whispers to /today. The rehearsal is:
- Accessible via /interrupts/rehearse
- Linkable from interrupt preview page
- Not surfaced as a daily cue

This preserves the "calm by default" principle.

### Bounded Effects

Phase 41 only orchestrates existing pipeline. It cannot:
- Bypass Phase 33 permission policy
- Bypass Phase 34 preview requirements
- Change the 2/day delivery cap
- Store raw device tokens (Phase 35b boundary)
- Send background notifications

## Consequences

### Positive

1. **End-to-end validation**: Users can verify the full delivery chain works
2. **Configuration confidence**: APNs credentials can be tested safely
3. **Audit trail**: Every rehearsal is recorded with hash-only receipt
4. **No new risks**: Uses existing pipeline, no new delivery paths
5. **Privacy preserved**: Abstract payload only, no identifiers

### Negative

1. **Consumes cap**: Rehearsals count toward 2/day limit
2. **Requires setup**: Device must be registered, policy must allow

### Neutral

1. **Manual only**: No automatic rehearsals (by design)
2. **One transport**: Uses registered device's transport kind

## Non-Goals

Phase 41 explicitly does NOT provide:
- Background delivery capabilities
- Real-time urgency guarantees
- Merchant/app-specific messaging
- Automatic retry on failure
- Notification content customization

## Implementation

### Package Structure

```
pkg/domain/interruptrehearsal/
├── types.go           # Enums, structs, validation

internal/interruptrehearsal/
├── engine.go          # Pure deterministic engine

internal/persist/
├── interrupt_rehearsal_store.go  # Hash-only persistence
```

### Events

- `phase41.rehearsal.requested`
- `phase41.rehearsal.eligibility_computed`
- `phase41.rehearsal.rejected`
- `phase41.rehearsal.plan_built`
- `phase41.rehearsal.delivery_attempted`
- `phase41.rehearsal.delivery_completed`
- `phase41.rehearsal.receipt_persisted`
- `phase41.rehearsal.proof_viewed`

### Storelog Records

- `INTERRUPT_REHEARSAL_RECEIPT`
- `INTERRUPT_REHEARSAL_ACK`

### Retention

- Max 30 days
- Max 500 records
- FIFO eviction

## References

- ADR-0069: Phase 33 - Interrupt Permission Contract
- ADR-0070: Phase 34 - Permitted Interrupt Preview
- ADR-0071: Phase 35 - Push Transport
- ADR-0072: Phase 35b - APNs Sealed Secret Boundary
- ADR-0073: Phase 36 - Interrupt Delivery Orchestrator
- ADR-0074: Phase 37 - Device Registration + Deep-Link
