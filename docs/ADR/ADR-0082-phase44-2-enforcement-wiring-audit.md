# ADR-0082: Phase 44.2 - Enforcement Wiring Audit

## Status

Accepted

## Context

Phase 42 introduced Delegated Holding Contracts and Phase 44 introduced Cross-Circle Trust Transfer. Both enforce HOLD-only outcomes - they can never escalate to SURFACE, INTERRUPT_CANDIDATE, DELIVER, or EXECUTE.

However, there is a class of bugs where a new engine or pipeline "forgets" to consult these contracts, accidentally bypassing the HOLD-only constraint. This is the "cofounder mode" problem: ensuring that constraints actually bind the runtime, not just exist in code.

Phase 44.2 solves this by:
1. Creating a single choke-point wrapper (`ClampOutcome`) that all pipelines must pass through
2. Creating an explicit audit route that proves wiring is intact
3. Using guardrails to prevent regressions by enforcing call-site counts and "no bypass" patterns

### Key Questions Answered

1. **How do we prove HOLD-only is enforced everywhere?**
   - Single enforcement clamp wrapper at all decision points
   - Manifest tracking which wrappers are wired
   - Behavioral probes verifying clamping works

2. **How do we prevent regressions?**
   - 150+ guardrails checking invariants
   - Explicit call-site verification
   - Demo tests proving clamp behavior

3. **How do we make this visible?**
   - `/proof/enforcement` page showing audit status
   - Explicit POST to run audit
   - Calm lines confirming enforcement

## Decision

### Architecture

```
Pipelines → ClampOutcome → Clamped Decision
              ↑
     ContractsSummary
     (HasHoldOnlyContract,
      HasTransferContract,
      IsCommerce, etc.)
```

### Components

1. **Enforcement Clamp (`internal/enforcementclamp/engine.go`)**
   - Single choke-point for all decision clamping
   - `ClampOutcome(input) → ClampOutput`
   - Rules:
     - Commerce: ALWAYS clamp to HOLD
     - HOLD-only contract: Clamp forbidden decisions to HOLD/QUEUE_PROOF
     - Envelope: CANNOT override HOLD-only clamp
     - Interrupt policy: CANNOT override HOLD-only clamp
   - NEVER returns SURFACE, INTERRUPT_CANDIDATE, DELIVER, EXECUTE when contract present

2. **Enforcement Manifest (`internal/enforcementaudit/manifest.go`)**
   - Tracks which enforcement wrappers are wired
   - Fields:
     - `PressureGateApplied`
     - `DelegatedHoldingApplied`
     - `TrustTransferApplied`
     - `InterruptPreviewApplied`
     - `DeliveryOrchestratorUsesClamp`
     - `TimeWindowAdapterApplied`
     - `CommerceExcluded`
     - `ClampWrapperRegistered`
   - `IsComplete()` verifies all components wired
   - `MissingComponents()` lists gaps

3. **Audit Engine (`internal/enforcementaudit/engine.go`)**
   - Runs wiring audit (checks manifest)
   - Runs behavioral probes (verifies clamping)
   - Produces `AuditRun` with status and checks
   - Supports acknowledgment flow

4. **Domain Types (`pkg/domain/enforcementaudit/types.go`)**
   - `AuditTargetKind`: pressure_pipeline, interrupt_pipeline, etc.
   - `AuditCheckKind`: contract_applied, contract_not_applied, etc.
   - `AuditStatus`: pass, fail
   - `AuditSeverity`: info, warn, critical
   - `ClampedDecisionKind`: no_effect, hold, queue_proof (NO surface/deliver/execute)
   - `AuditCheck`, `AuditRun`, `AuditProofPage`, `AuditAck`

5. **Persistence (`internal/persist/enforcement_audit_store.go`)**
   - `EnforcementAuditStore`: Stores audit runs
   - `EnforcementAuditAckStore`: Stores acknowledgments
   - Hash-only storage, FIFO eviction
   - Bounded: 30 days OR 100 records

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/proof/enforcement` | GET | Show audit proof page |
| `/proof/enforcement/run` | POST | Run enforcement audit |
| `/proof/enforcement/dismiss` | POST | Acknowledge audit |

### Events

- `phase44_2.audit.requested` - Audit run requested
- `phase44_2.audit.computed` - Audit completed
- `phase44_2.audit.persisted` - Audit saved
- `phase44_2.audit.viewed` - Proof page viewed
- `phase44_2.audit.dismissed` - Audit acknowledged
- `phase44_2.audit.failed` - Audit failed (status=fail)

### Invariants

1. **Single Choke-Point**: All pipelines use `ClampOutcome`
2. **HOLD-only Enforcement**: When contract present, ONLY returns NO_EFFECT, HOLD, QUEUE_PROOF
3. **Commerce Always Blocked**: IsCommerce → HOLD (redundant safety)
4. **No Override**: Envelope and interrupt policy cannot override HOLD-only clamp
5. **Manifest Completeness**: All 10 fields must be true for pass
6. **Hash-Only Storage**: No raw identifiers in stores
7. **Bounded Retention**: 30 days OR 100 records, FIFO
8. **Max 12 Checks**: Per audit run
9. **No Goroutines**: In pkg/ and internal/ Phase 44.2 code
10. **Clock Injection**: No time.Now() in domain/engine

### Clamp Rules (in precedence order)

1. Commerce → HOLD (always)
2. HOLD-only contract + forbidden decision → HOLD or QUEUE_PROOF
3. Transfer contract + forbidden decision → HOLD or QUEUE_PROOF
4. Envelope active → CANNOT override #2 or #3
5. Interrupt policy active → CANNOT override #2 or #3
6. No contract → pass through normalized

### Forbidden Decisions (under HOLD-only)

- `SURFACE`
- `INTERRUPT_CANDIDATE`
- `DELIVER`
- `EXECUTE`

## Consequences

### Positive

- **Provable enforcement**: Can verify HOLD-only is actually enforced
- **Regression prevention**: 150+ guardrails catch violations
- **Single choke-point**: All decisions go through one wrapper
- **Visible audit**: Users can see enforcement status
- **Calm UI**: No identifiers, no urgency, just facts

### Negative

- **Additional complexity**: New engine, manifest, stores
- **Performance overhead**: Clamp wrapper on every decision (minimal)
- **Maintenance burden**: Must keep manifest in sync

### Neutral

- No new actions enabled
- No new delivery logic
- Pure enforcement and proof

## Non-Goals

- No notifications from audit
- No new actions enabled
- No automatic remediation
- No blocking deployments on audit failure

## Security Considerations

- Hash-only storage (no identifiers)
- POST-only for mutations
- No sensitive data in audit checks
- Evidence hashes, not raw evidence

## Implementation Notes

### Guardrails (150+ checks)

Key sections:
- File existence and package headers
- No goroutines, clock injection
- Forbidden patterns
- Domain enums and structs
- Clamp engine methods
- Manifest fields and methods
- Store methods
- Bounded retention
- Web routes and templates
- No forbidden imports

### Demo Tests (30+ tests)

Key tests:
- `TestManifestCanonicalDeterministic`
- `TestAuditRunHashDeterministic`
- `TestClampHoldOnlyOverridesSurface`
- `TestClampHoldOnlyOverridesInterruptCandidate`
- `TestClampQueueProofAllowed`
- `TestClampNeverEnablesDeliver`
- `TestEnvelopeCannotOverrideHoldOnly`
- `TestInterruptPolicyCannotOverrideHoldOnly`
- `TestAuditFailsWhenManifestSaysMissingClamp`
- `TestAuditPassesWhenManifestComplete`

## References

- Phase 42: Delegated Holding Contracts (ADR-0079)
- Phase 43: Held Under Agreement Proof Ledger (ADR-0080)
- Phase 44: Cross-Circle Trust Transfer (ADR-0081)
