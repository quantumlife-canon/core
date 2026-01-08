# ADR-0080: Phase 43 - Held Under Agreement Proof Ledger

## Status
Accepted

## Context

Phase 42 introduced Delegated Holding Contracts, allowing the circle to pre-consent to HOLD for bounded periods. When a contract has `action: queue_proof`, the system suppresses surfaces but should record proof that something was held.

Users deserve proof that:
- Items were held under their explicit agreement
- The system honored their delegation contract
- Nothing was silently dropped without consent

This is a **proof-only ledger** — it shows what was held under agreement, not what happened. It does NOT change behavior; Phase 42 contracts continue to function exactly as before. This page simply surfaces evidence.

### Why a Separate Proof Ledger?

The existing proof pages (Phase 18.5, 28, 29) show restraint or trust evidence. This ledger specifically shows:
- Items held because of a Phase 42 QUEUE_PROOF outcome
- Abstract signals (circle_type, horizon, magnitude)
- No identifiers, no amounts, no names

### Commerce Exclusion

Commerce pressure is NEVER included in held proof. Commerce is observed but never surfaces, never interrupts, and never appears in proof ledgers. This is canon.

### Bounded Retention

Like all proof systems in QuantumLife:
- Signals retained for 30 days OR max 500 records
- Acks retained for 30 days OR max 200 records
- FIFO eviction when limits reached
- Dedup by (dayKey + evidenceHash) prevents duplicates

## Decision

Implement Phase 43 as **Held Under Agreement Proof Ledger**:

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│           Phase 43: Held Under Agreement Proof Ledger               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Phase 42 produces QUEUE_PROOF outcome                              │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  HandleQueueProofOutcome                 │                       │
│  │    - Extract abstract buckets only       │                       │
│  │    - Compute evidenceHash                │                       │
│  │    - Reject commerce                     │                       │
│  └─────────────────────────────────────────┘                       │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  HeldProofSignalStore                    │                       │
│  │    - Dedup by dayKey + evidenceHash      │                       │
│  │    - FIFO eviction (30 days / 500 max)   │                       │
│  │    - Write to storelog                   │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
│  ═══════════════════════════════════════════════════════════════   │
│                                                                     │
│  Circle visits /proof/held                                          │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  Engine.BuildPageForDay                  │                       │
│  │    - Load signals for today              │                       │
│  │    - Filter commerce (reject)            │                       │
│  │    - Cap at 3 signals, 1 per circle type │                       │
│  │    - Sort by evidenceHash                │                       │
│  │    - Compute statusHash                  │                       │
│  └─────────────────────────────────────────┘                       │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  Render Page                             │                       │
│  │    - Title: "Held, by agreement."        │                       │
│  │    - Line: based on magnitude            │                       │
│  │    - Chips: circle types                 │                       │
│  │    - StatusHash prefix (16 chars)        │                       │
│  │    - Dismiss button                      │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
│  ═══════════════════════════════════════════════════════════════   │
│                                                                     │
│  /today cue chain integration                                       │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │  buildHeldProofCueForToday               │                       │
│  │    - If signals exist AND not dismissed  │                       │
│  │      AND not viewed → show cue           │                       │
│  │    - Cue: "We held some things —         │                       │
│  │           by agreement."                 │                       │
│  │    - Path: /proof/held                   │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Domain Types

```go
// HeldProofSignal represents a single held proof signal.
type HeldProofSignal struct {
    Kind         HeldProofKind            // Source: delegated_holding
    CircleType   HeldProofCircleType      // human | institution (commerce forbidden)
    Horizon      HeldProofHorizonBucket   // now | soon | later
    Magnitude    HeldProofMagnitudeBucket // nothing | a_few | several
    EvidenceHash string                   // sha256 hex
}

// HeldProofPage is the rendered proof page.
type HeldProofPage struct {
    Title      string
    Line       string
    Chips      []string
    Magnitude  HeldProofMagnitudeBucket
    StatusHash string
}

// HeldProofCue is the whisper cue for /today.
type HeldProofCue struct {
    Available  bool
    CueText    string
    Path       string
    StatusHash string
}
```

### Routes

| Route                | Method | Handler                | Description |
|---------------------|--------|------------------------|-------------|
| `/proof/held`       | GET    | `handleHeldProof`      | Proof page  |
| `/proof/held/dismiss` | POST | `handleHeldProofDismiss` | Dismiss cue |

### Events

- `Phase43HeldProofSignalPersisted`: Signal written to store
- `Phase43HeldProofPageRendered`: Page viewed
- `Phase43HeldProofCueComputed`: Cue computed for /today
- `Phase43HeldProofAckViewed`: Page viewed acknowledgment
- `Phase43HeldProofAckDismissed`: Page dismissed

### Storelog Records

- `HELD_PROOF_SIGNAL`: Signal record (hash-only)
- `HELD_PROOF_ACK`: Acknowledgment record (hash-only)

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| MaxRetentionDays | 30 | Days before eviction |
| MaxSignalRecords | 500 | Max signals before FIFO |
| MaxAckRecords | 200 | Max acks before FIFO |
| MaxSignalsPerPage | 3 | Cap per page |
| MaxSignalsPerCircle | 1 | Cap per circle type |

### UX Copy

| Magnitude | Line |
|-----------|------|
| a_few | "A few things were held so you didn't have to decide yet." |
| several | "Several pressures were held under your agreement." |

### Guardrails

The implementation must pass 95+ guardrails:
- No goroutines in pkg/domain/heldproof or internal/heldproof
- No time.Now() in pkg/domain/heldproof or internal/heldproof
- Commerce exclusion enforced (IsCommerce check)
- Bounded retention (30 days / max records)
- Dedup by (dayKey + evidenceHash)
- Hash-only storage (no identifiers)
- POST enforcement for dismiss handler

## Consequences

### Positive

- Users can verify their delegation contracts were honored
- Proof of restraint under explicit agreement
- No behavior change — purely informational
- Commerce completely excluded from proof
- Bounded retention prevents growth

### Negative

- Additional storage overhead (500 signals max)
- Additional page and route to maintain
- Must be wired to Phase 42 QUEUE_PROOF

### Neutral

- Follows established proof page pattern
- Uses same whisper chain for /today integration
- Same hash-only, abstract-only approach as other proofs

## Related ADRs

- ADR-0079: Phase 42 - Delegated Holding Contracts (source of QUEUE_PROOF)
- ADR-0032: Phase 18.5 - Quiet Proof (established proof page pattern)
- ADR-0050: Phase 28 - Trust Action Receipt (proof of execution)
- ADR-0054: Phase 29 - Finance Mirror Proof (hash-only proof pattern)
