# ADR-0085: Phase 47 - Pack Coverage Realization

## Status
Accepted

## Context

Phase 46 introduced Marketplace Packs that bundle semantics presets and observer binding intents.
However, these packs are meaning-only - they declare what observers SHOULD run but don't actually
enable them. This phase bridges the gap by **realizing** pack intents into actual observer coverage.

**Core principle**: Coverage realization expands OBSERVERS and SCANNERS only. It NEVER grants
permission, NEVER changes interrupt policy, NEVER changes delivery, NEVER changes execution.

This is Track B: Expand observers, not actions.

## Problem

Packs are semantics unless they expand observers. A user can install a "commerce-observer" pack,
but without realization, no commerce observations actually occur. We need to:

1. Derive a CoveragePlan from installed packs
2. Wire that plan into existing ingestion flows (Gmail sync, TrueLayer sync, notification observer)
3. Provide calm proof of what got broader

## Decision

### Invariant: effect_no_power Always

Every coverage plan, delta, and record MUST reflect `effect_no_power`. Coverage realization
enables additional observer modules but does NOT grant any permission to:
- SURFACE
- INTERRUPT_CANDIDATE
- DELIVER
- EXECUTE

Commerce capability NEVER implies interrupts - just observation.

### Invariant: Capabilities Are Fixed Vocabulary

Coverage capabilities are a small, fixed set of strings:
- `cap_receipt_observer` - Receipt scanning from email
- `cap_commerce_observer` - Commerce pattern detection
- `cap_finance_commerce_observer` - Finance transaction observation
- `cap_pressure_map` - Pressure map updates
- `cap_timewindow_sources` - Time-window source analysis
- `cap_notification_metadata` - Device notification metadata

No free text. No custom capabilities.

### Invariant: Sources Are Fixed Vocabulary

Coverage sources are a small, fixed set:
- `source_gmail` - Gmail sync
- `source_finance_truelayer` - TrueLayer finance sync
- `source_device_notification` - Device notification observer

### Invariant: Hash-Only Storage

All stored records use hash-only identifiers:
- `CircleIDHash` - SHA256 hash of circle ID
- `PlanHash` - SHA256 hash of plan canonical string
- `DeltaHash` - SHA256 hash of delta canonical string
- `StatusHash` - SHA256 hash of record state

No raw identifiers, no secrets, no personal data in storage.

### Invariant: No Decision Package Imports

The coverageplan package MUST NOT import from decision-making packages:
- `pressuredecision`
- `interruptpolicy`
- `interruptpreview`
- `interruptdelivery`
- `pushtransport`
- `trustaction`
- `firstaction`
- `execrouter`
- `execexecutor`

Coverage is observation-only. It does not participate in decision-making.

### Invariant: No Goroutines

No goroutines in pkg/domain/coverageplan or internal/coverageplan. All operations are synchronous.
Clock injection is used instead of time.Now().

### Invariant: Bounded Retention

Stores use FIFO eviction with bounds:
- Maximum 200 records per store
- Maximum 30 days retention
- Oldest records evicted first when bounds exceeded

### Invariant: Wiring In cmd/ Only

Coverage realization wiring (enabling/disabling observers based on coverage plan) MUST occur
only in cmd/quantumlife-web/main.go. The internal/ and pkg/ packages remain pure.

## Non-Goals

1. **No permissions** - Coverage realization does not grant permission to surface, interrupt, deliver, or execute
2. **No delivery changes** - Does not modify how or when interrupts are delivered
3. **No execution changes** - Does not enable or modify any actions
4. **No noisy behavior** - All changes happen quietly; proof page is calm and abstract
5. **No pack IDs in proof** - Proof page shows capabilities, not pack identifiers

## Architecture

### Domain Types (pkg/domain/coverageplan/)

Enums:
1. **CoverageSourceKind** - source_gmail, source_finance_truelayer, source_device_notification
2. **CoverageCapability** - cap_receipt_observer, cap_commerce_observer, etc.
3. **CoverageChangeKind** - change_added, change_removed, change_unchanged
4. **CoverageProofAckKind** - ack_viewed, ack_dismissed

Structs:
- **CoveragePlan** - Full plan for a circle
- **CoverageSourcePlan** - Per-source enabled capabilities
- **CoverageDelta** - Diff between plans
- **CoverageProofPage** - UI model for proof
- **CoverageProofCue** - Whisper cue for coverage changes
- **CoverageProofAck** - Acknowledgment record

### Engine (internal/coverageplan/)

Pure deterministic logic for:
- Building coverage plans from installed packs
- Computing deltas between plans
- Building proof pages and cues
- Determining cue visibility

Pack-to-capability mapping is hardcoded:
- `core-gmail-receipts` -> source_gmail: cap_receipt_observer, cap_commerce_observer
- `core-finance-commerce` -> source_finance_truelayer: cap_finance_commerce_observer, cap_commerce_observer
- `core-pressure` -> cap_pressure_map, cap_timewindow_sources
- `core-device-hints` -> source_device_notification: cap_notification_metadata

Unknown pack IDs are ignored (not errors).

### Persistence (internal/persist/)

Two stores with bounded retention:
1. **CoveragePlanStore** - Coverage plan records
2. **CoverageProofAckStore** - Proof acknowledgments

### Web Routes

| Route | Method | Description |
|-------|--------|-------------|
| /proof/coverage | GET | Coverage proof page |
| /proof/coverage/dismiss | POST | Dismiss coverage proof |

### Events

- `phase47.coverage.plan_built`
- `phase47.coverage.plan_persisted`
- `phase47.coverage.delta_computed`
- `phase47.coverage.proof.rendered`
- `phase47.coverage.ack.recorded`
- `phase47.coverage.cue.computed`

### Storelog Records

- `COVERAGE_PLAN` - Coverage plan created/updated
- `COVERAGE_PROOF_ACK` - Proof acknowledgment

### Coverage Wiring (cmd/quantumlife-web/main.go)

After Gmail sync:
- Check current CoveragePlan capabilities
- If cap_receipt_observer enabled -> run receiptscan pipeline
- If cap_commerce_observer enabled -> update commerce observer store

After TrueLayer sync:
- If cap_finance_commerce_observer enabled -> run financeTxScan pipeline

After POST /observe/notification:
- If cap_notification_metadata enabled -> accept signal
- Else -> reject with "coverage_disabled"

## Consequences

### Positive

1. Installed packs now have effect on observer behavior
2. Users see calm proof of expanded coverage
3. No permission escalation possible
4. Hash-only storage maintains privacy
5. Clean separation: internal/ is pure, cmd/ does wiring

### Negative

1. Some pack types don't map to capabilities (semantics-only packs)
2. Unknown pack IDs are silently ignored
3. No dynamic capability discovery

### Neutral

1. Coverage wiring is compile-time fixed
2. No pack versioning affects coverage
3. Proof page is abstract (capabilities, not packs)

## Implementation Notes

1. Pack-to-capability mapping is a static map in engine
2. Capabilities are sorted lexicographically for deterministic hashing
3. Proof page shows "Coverage widened quietly." / "Nothing changed."
4. Cue text: "A new lens was added - quietly."
5. Cue priority: LOWEST (below shadow receipt, reality, first-minutes, etc.)

## Security Considerations

1. No raw identifiers in storage
2. No secrets in coverage plans
3. POST-only for mutations
4. Hash-only URLs
5. effect_no_power prevents permission escalation
6. Wiring cannot enable actions, only observers

## Forbidden Patterns

These patterns MUST NOT appear in coverageplan code:
- Email addresses or email-like patterns
- URLs or HTTP patterns
- Currency symbols (£, $, €)
- Merchant strings (uber, deliveroo, amazon)
- Imports from decision/delivery/execution packages

## References

- Phase 46: ADR-0084-phase46-circle-registry-packs.md
- Phase 31: ADR-0065-phase31-commerce-observers.md
- Phase 38: ADR-0072-phase38-notification-metadata-observer.md
- Phase 40: ADR-0074-phase40-timewindow-pressure.md
