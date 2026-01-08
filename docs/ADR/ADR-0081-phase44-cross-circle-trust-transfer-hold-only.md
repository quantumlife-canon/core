# ADR-0081: Phase 44 - Cross-Circle Trust Transfer (HOLD-only)

## Status
Accepted

## Context

QuantumLife's Phase 42 established Delegated Holding Contracts where a user can delegate HOLD bias for their own circle. However, users sometimes want to share restraint across circles:

- "I'm traveling with my partner — let their circle hold things for mine."
- "During my sabbatical, my assistant's circle should hold work pressure for me."
- "When I'm in a meeting, hold family communications using my work circle's restraint."

This is NOT about granting action permission. It's about allowing one circle (To) to apply HOLD restraint on behalf of another circle (From). The key insight is:

**"Delegation of silence, not action."**

### Cross-Circle vs. Same-Circle

Phase 42 (Delegated Holding): Single circle delegates to the system
Phase 44 (Trust Transfer): One circle delegates HOLD to another circle

The critical distinction is that Phase 44 involves TWO circles in a trust relationship, where the To circle can restrain (but NEVER escalate) the From circle's pressure.

### Trust Direction

```
From Circle ──(delegates HOLD-only restraint to)──> To Circle
```

The From circle is the one being protected (held). The To circle is the one providing restraint. The To circle CANNOT:
- Surface items to the From circle
- Trigger interrupts for the From circle
- Execute any actions
- Deliver any messages

The To circle CAN only:
- Apply additional HOLD bias to the From circle's pressure
- Queue proof signals

### Why HOLD-only?

The HOLD-only constraint is CRITICAL for trust safety:

1. **No escalation risk**: Trust can only make things quieter, never noisier
2. **No action risk**: No circle can execute on another's behalf
3. **Reversible**: HOLD can be undone; EXECUTE cannot
4. **Bounded scope**: Commerce is ALWAYS excluded

## Decision

Implement Phase 44 as **Cross-Circle Trust Transfer (HOLD-only)**:

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│       Phase 44: Cross-Circle Trust Transfer (HOLD-only)             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  From Circle visits /delegate/transfer                              │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  Build Proposal                          │                       │
│  │    - FromCircleHash (proposer)           │                       │
│  │    - ToCircleHash (receiver of trust)    │                       │
│  │    - Scope: human | institution | all    │                       │
│  │    - Duration: hour | day | trip         │                       │
│  │    - Reason: travel | work | health | ...│                       │
│  └─────────────────────────────────────────┘                       │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  Accept Proposal → Create Contract       │                       │
│  │    - Mode: HOLD_ONLY (always)            │                       │
│  │    - State: ACTIVE                       │                       │
│  │    - One active per FromCircle           │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
│  ═══════════════════════════════════════════════════════════════   │
│                                                                     │
│  When Phase 32 pressure pipeline evaluates:                         │
│                                                                     │
│  ┌─────────────────────────────────────────┐                       │
│  │  ApplyTransfer(contract, decision, now)  │                       │
│  │    - Is contract active?                 │                       │
│  │    - Is this commerce? → NO_EFFECT       │                       │
│  │    - Does scope match circle type?       │                       │
│  │    - Is decision forbidden?              │                       │
│  │      (SURFACE, INTERRUPT_CANDIDATE,      │                       │
│  │       DELIVER, EXECUTE)                  │                       │
│  └─────────────────────────────────────────┘                       │
│           │                                                         │
│    ┌──────┴───────────────┐                                        │
│    │                      │                                        │
│  No Match/Commerce    Forbidden Decision                           │
│    │                      │                                        │
│    ▼                      ▼                                        │
│  NO_EFFECT            CLAMP TO HOLD                                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### HOLD-only Clamping

The core invariant of Phase 44 is that ApplyTransfer can ONLY return:
- `NO_EFFECT`: Contract doesn't apply (scope mismatch, commerce, inactive)
- `HOLD`: Forbidden decision was clamped to HOLD
- `QUEUE_PROOF`: Allow proof but no surface/interrupt

It NEVER returns:
- `SURFACE`
- `INTERRUPT_CANDIDATE`
- `DELIVER`
- `EXECUTE`

### Commerce Exclusion

Commerce pressure is ALWAYS excluded from trust transfer, even with `scope: all`:
- Commerce circles involve financial decisions
- Cross-circle trust should never affect commerce
- Commerce visibility remains unchanged regardless of transfer status

### Web Routes

1. **GET /delegate/transfer**
   - Title: "Shared Holding"
   - Shows active transfer status (if any)
   - Shows propose form (if eligible)
   - Shows revoke button (if active)

2. **POST /delegate/transfer/propose**
   - Creates a new transfer proposal
   - Validates inputs
   - Returns proposal with hash

3. **POST /delegate/transfer/accept**
   - Accepts a proposal, creates active contract
   - Validates one-per-FromCircle rule
   - Redirects to /delegate/transfer

4. **POST /delegate/transfer/revoke**
   - Revokes active contract
   - Records revocation with reason
   - Redirects to /delegate/transfer

5. **GET /proof/transfer**
   - Title: "Shared Holding Proof"
   - Shows abstract contract status
   - Hash prefixes for verification

### Transfer Scope

- `human`: Matches pressure from HUMAN circles only
- `institution`: Matches pressure from INSTITUTION circles only
- `all`: Matches HUMAN and INSTITUTION (never commerce)

### Transfer Duration

- `hour`: Expires after 1 hour bucket
- `day`: Expires after 1 day bucket
- `trip`: User must revoke; auto-expires after 7 days max

### Proposal Reasons (Allowlist)

- `travel`: Traveling with someone
- `work`: Work focus/collaboration
- `health`: Health-related sharing
- `overload`: Overwhelmed, need help
- `family`: Family time sharing

No free-text reasons. All reasons are from allowlist.

### Revoke Reasons (Allowlist)

- `done`: Transfer complete
- `too_much`: Trust was too broad
- `changed_mind`: Changed preference
- `trust_reset`: Resetting trust relationship

### One Active Per FromCircle

Only one transfer contract can be active per FromCircle at a time. Creating a new contract while one is active is rejected.

### Bounded Retention

- Maximum 30 days retention
- Maximum 200 records
- FIFO eviction when limits exceeded

## Consequences

### Positive

1. **Cross-circle trust**: Enables trusted restraint sharing between circles
2. **HOLD-only safety**: Cannot escalate, only restrain
3. **Commerce excluded**: Financial decisions remain protected
4. **Bounded effects**: Time limits prevent indefinite transfer
5. **Revocable**: Either circle can end transfer

### Negative

1. **Complexity**: Adds cross-circle dynamics to pressure pipeline
2. **Trust verification**: Must validate To circle exists and is trusted

### Neutral

1. **Manual only**: No automatic transfer creation
2. **Single direction**: From delegates to To, not bidirectional

## Non-Goals

Phase 44 explicitly does NOT provide:
- Permission to execute any action
- Permission to deliver interrupts
- Permission to surface items
- Override of Phase 33/34 policies
- Background automation
- Bidirectional transfer
- Commerce pressure handling

## Implementation

### Package Structure

```
pkg/domain/trusttransfer/
├── types.go           # Enums, structs, validation, hash computation

internal/trusttransfer/
├── engine.go          # Pure deterministic engine with HOLD-only clamping

internal/persist/
├── trust_transfer_store.go  # Hash-only persistence with FIFO eviction
```

### Critical Files

1. `pkg/domain/trusttransfer/types.go`
   - TransferScope, TransferMode, TransferState, TransferDuration
   - TransferDecision (NO_EFFECT, HOLD, QUEUE_PROOF only)
   - ProposalReason, RevokeReason
   - TrustTransferProposal, TrustTransferContract, TrustTransferRevocation
   - TrustTransferEffect with WasClamped flag
   - Phase32DecisionInput for decision evaluation

2. `internal/trusttransfer/engine.go`
   - BuildProposal, AcceptProposal, Revoke
   - ApplyTransfer with HOLD-only clamping
   - ClampDecision, IsForbiddenDecision helpers

3. `internal/persist/trust_transfer_store.go`
   - TrustTransferContractStore with GetActiveForFromCircle
   - TrustTransferRevocationStore
   - Bounded retention (30 days, 200 records)

### Storelog Record Types

- `TRUST_TRANSFER_CONTRACT`: Contract creation/acceptance
- `TRUST_TRANSFER_REVOCATION`: Contract revocation

### Events

- `phase44.transfer.proposed`: Proposal created
- `phase44.transfer.accepted`: Contract accepted
- `phase44.transfer.revoked`: Contract revoked
- `phase44.transfer.effect_applied`: Transfer effect computed
- `phase44.transfer.proof.rendered`: Proof page viewed

## Security Considerations

1. **Hash-only storage**: No raw identifiers stored
2. **HOLD-only enforcement**: Clamping is enforced in engine, not UI
3. **Commerce isolation**: Commerce check happens before scope check
4. **One-per-FromCircle**: Prevents conflicting transfers
5. **Bounded retention**: Old data auto-evicted

## Testing

Demo tests in `internal/demo_phase44_trust_transfer/demo_test.go` verify:
- Determinism (same inputs → same hashes)
- HOLD-only clamping (SURFACE, INTERRUPT, DELIVER, EXECUTE → HOLD)
- Commerce exclusion (always NO_EFFECT)
- One active per FromCircle
- Scope matching (human vs institution vs all)
- Revocation and state changes
- Proof and status page building

## References

- Phase 42: ADR-0079 (Delegated Holding Contracts)
- Phase 43: ADR-0080 (Held Proof Ledger)
- Phase 32: Pressure Decision Gate
- Phase 33: Interrupt Permission Contract
- Phase 34: Permitted Interrupt Preview
