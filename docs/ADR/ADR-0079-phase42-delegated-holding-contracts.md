# ADR-0079: Phase 42 - Delegated Holding Contracts

## Status
Accepted

## Context

QuantumLife's core principle is "silence is success" — by default, everything is HELD quietly. Phases 32-34 established a pressure-to-interrupt pipeline where HUMAN and INSTITUTION pressure can potentially surface items or trigger interrupt candidates.

However, users sometimes want to explicitly delegate holding for a bounded period:
- "I'm traveling for 3 days — hold all delivery-related pressure silently."
- "I'm in a meeting for an hour — don't interrupt me for institutional matters."
- "I'm on vacation for a week — queue proofs but don't surface anything."

This is NOT about granting execution permission. It's about reinforcing HOLD behavior with explicit, time-bounded, revocable consent.

### Why "Delegated" Holding?

The user explicitly "delegates" the decision to HOLD to the system for a defined scope and duration. This is:
- **Pre-consent to HOLD**: The user agrees ahead of time that certain pressure should be held.
- **Not permission to act**: Holding contracts cannot escalate, execute, or interrupt.
- **Bounded**: Duration is explicit (hour/day/trip) with automatic expiry.
- **Revocable**: User can revoke at any time.

### Trust Baseline Required

A holding contract requires an established trust baseline (Phase 20). This ensures:
- User has engaged with the system
- System has demonstrated restraint
- Contract is not a first-time interaction

### Integration with Pressure Pipeline

Phase 42 integrates ONLY as a bias toward HOLD:
- If contract active AND pressure matches scope/magnitude/horizon → HOLD (or QUEUE_PROOF)
- Otherwise → NO_EFFECT (existing pipeline unchanged)

Phase 42 CANNOT:
- Override Phase 33 permission policy
- Create SURFACE or INTERRUPT outcomes
- Bypass Phase 34 preview requirements
- Execute any action

## Decision

Implement Phase 42 as **Delegated Holding Contracts**:

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│              Phase 42: Delegated Holding Contracts                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User visits /delegate                                              │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  Check Eligibility (Engine)              │                       │
│  │    - Trust baseline exists?              │                       │
│  │    - No active interrupt preview?        │                       │
│  │    - No existing active contract?        │                       │
│  └─────────────────────────────────────────┘                       │
│           │                                                         │
│    ┌──────┴───────┐                                                │
│    │              │                                                │
│  Ineligible    Eligible                                            │
│    │              │                                                │
│    ▼              ▼                                                │
│  [Show why]   [Show create form]                                   │
│                   │                                                │
│                   ▼                                                │
│  ┌─────────────────────────────────────────┐                       │
│  │  POST /delegate/create                   │                       │
│  │    - Scope: human | institution          │                       │
│  │    - Action: hold_silently | queue_proof │                       │
│  │    - Duration: hour | day | trip         │                       │
│  │    - MaxHorizon: soon | later | unknown  │                       │
│  │    - MaxMagnitude: nothing | a_few | several │                  │
│  └─────────────────────────────────────────┘                       │
│                   │                                                │
│                   ▼                                                │
│  ┌─────────────────────────────────────────┐                       │
│  │  Store Contract + Redirect to /delegate  │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
│  ═══════════════════════════════════════════════════════════════   │
│                                                                     │
│  When pressure pipeline evaluates:                                  │
│                                                                     │
│  ┌─────────────────────────────────────────┐                       │
│  │  ApplyContract(contract, pressure, now)  │                       │
│  │    - Is contract active?                 │                       │
│  │    - Does pressure scope match?          │                       │
│  │    - Is horizon <= MaxHorizon?           │                       │
│  │    - Is magnitude <= MaxMagnitude?       │                       │
│  └─────────────────────────────────────────┘                       │
│           │                                                         │
│    ┌──────┴───────┐                                                │
│    │              │                                                │
│  No Match    Match                                                 │
│    │              │                                                │
│    ▼              ▼                                                │
│  NO_EFFECT    HOLD or QUEUE_PROOF                                  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Web Routes

1. **GET /delegate**
   - Title: "Held, by agreement."
   - Shows active contract status (if any)
   - Shows create form (if eligible)
   - Shows revoke button (if active)

2. **POST /delegate/create**
   - Creates a new holding contract
   - Validates eligibility
   - Redirects to /delegate

3. **POST /delegate/revoke**
   - Revokes active contract
   - Redirects to /delegate

4. **GET /proof/delegate**
   - Title: "Agreement, kept."
   - Shows abstract contract status
   - Hash prefixes for verification

### Delegation Scope

- `human`: Matches pressure from HUMAN circles (individuals, communications)
- `institution`: Matches pressure from INSTITUTION circles (companies, services)

### Delegation Action

- `hold_silently`: Suppress from all surfaces, no proof queued
- `queue_proof`: Suppress from surfaces, but queue proof for later viewing

### Delegation Duration

- `hour`: Expires after 1 hour bucket
- `day`: Expires after 1 day bucket
- `trip`: User must revoke; auto-expires after 7 days max

### Duration Expiry

Expiry is computed deterministically from duration buckets:
- `hour`: Created bucket + 1 hour
- `day`: Created bucket + 1 day
- `trip`: Created bucket + 7 days (hard cap)

### One Active Contract Per Circle

Only one contract can be active per circle at a time. Creating a new contract while one is active requires revoking the existing one first.

## Consequences

### Positive

1. **Explicit consent**: Users explicitly choose to delegate holding
2. **Bounded effects**: Time limits prevent indefinite suppression
3. **Revocable**: Users can cancel at any time
4. **No escalation**: Cannot create SURFACE or INTERRUPT outcomes
5. **Trust-gated**: Requires established trust baseline

### Negative

1. **Complexity**: Adds a new layer to pressure pipeline
2. **Edge cases**: Duration expiry must handle clock edge cases

### Neutral

1. **Manual only**: No automatic contract creation
2. **Single scope**: One contract handles one scope at a time

## Non-Goals

Phase 42 explicitly does NOT provide:
- Permission to execute any action
- Permission to deliver interrupts
- Override of Phase 33/34 policies
- Background automation
- Multi-scope contracts
- Unlimited duration

## Implementation

### Package Structure

```
pkg/domain/delegatedholding/
├── types.go           # Enums, structs, validation

internal/delegatedholding/
├── engine.go          # Pure deterministic engine

internal/persist/
├── delegated_holding_store.go  # Hash-only persistence
```

### Events

- `phase42.delegation.created`
- `phase42.delegation.revoked`
- `phase42.delegation.expired`
- `phase42.delegation.applied`
- `phase42.delegation.proof.viewed`

### Storelog Records

- `DELEGATED_HOLDING_CONTRACT`
- `DELEGATED_HOLDING_REVOCATION`

### Retention

- Max 30 days
- Max 200 records
- FIFO eviction

## References

- ADR-0048: Phase 20 - Trust Accrual Layer
- ADR-0067: Phase 31.4 - External Pressure Circles
- ADR-0068: Phase 32 - Pressure Decision Pipeline
- ADR-0069: Phase 33 - Interrupt Permission Contract
- ADR-0070: Phase 34 - Permitted Interrupt Preview
