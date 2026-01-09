# ADR-0083: Phase 45 - Circle Semantics & Necessity Declaration

## Status

Accepted

## Context

As the marketplace pressure system grows more sophisticated, there is a need to understand what kind of entity each circle represents. Different sources have different urgency models:

- A human friend waiting for a reply has a different urgency model than a marketing email
- An institution with hard deadlines (bank, government) differs from an optional subscription service
- Commerce sources should never escalate to interrupts regardless of any urgency signal

Without explicit semantics, the system cannot explain why it treats different sources differently. Users cannot declare "this is a person I care about" vs "this is an optional newsletter."

### Key Questions Answered

1. **How do we know what kind of thing a circle is?**
   - Explicit semantic declaration (human/institution/service)
   - Derived defaults based on source type
   - Provenance tracking (who declared it)

2. **How do we model urgency appropriately per source?**
   - UrgencyModel enum (never_interrupt/hard_deadline/human_waiting/etc.)
   - Meaning-only - does NOT grant permission to interrupt
   - Does NOT change any decision logic in Phase 45

3. **How do we ensure this is meaning-only?**
   - SemanticsEffect enum with ONLY `effect_no_power` allowed
   - Guardrails preventing import into decision packages
   - Explicit non-goals in this ADR

## Decision

### Architecture

```
User/Derived → CircleSemantics → SemanticsRecord
                    ↑
           (effect_no_power ALWAYS)
```

### Components

1. **Domain Types (`pkg/domain/circlesemantics/types.go`)**
   - CircleSemanticKind: human, institution, service_essential, service_transactional, service_optional, unknown
   - UrgencyModel: never_interrupt, hard_deadline, human_waiting, time_window, soft_reminder, unknown
   - NecessityLevel: low, medium, high, unknown
   - SemanticsProvenance: user_declared, derived_rules, imported_connector
   - SemanticsEffect: ONLY effect_no_power in Phase 45
   - CircleSemantics struct with Validate() and CanonicalStringV1()
   - SemanticsRecord, SemanticsChange, SemanticsProofAck

2. **Engine (`internal/circlesemantics/engine.go`)**
   - DeriveDefaultSemantics(circleIDHash, circleType) → defaults based on type
   - BuildSettingsPage(inputs, existingRecords) → UI model
   - ApplyUserDeclaration(circleIDHash, desired, previous) → record + change
   - BuildProofPage(records) → proof UI model
   - ComputeCue(inputs, records, ackStore) → whisper cue (lowest priority)

   Default derivation rules:
   - commerce → semantic_service_optional, urgency_never_interrupt, necessity_low
   - human → semantic_human, urgency_human_waiting, necessity_medium
   - institution → semantic_institution, urgency_hard_deadline, necessity_high
   - unknown → all unknown values

3. **Persistence (`internal/persist/circle_semantics_store.go`)**
   - CircleSemanticsStore: hash-only, append-only record log
   - CircleSemanticsAckStore: proof acknowledgments
   - Bounded retention: max 200 records OR 30 days (FIFO)

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/settings/semantics` | GET | Show settings page |
| `/settings/semantics/save` | POST | Save user declaration |
| `/proof/semantics` | GET | Show proof page |
| `/proof/semantics/dismiss` | POST | Dismiss proof cue |

### Events

- `phase45.semantics.settings.viewed` - Settings page viewed
- `phase45.semantics.saved` - Semantics saved (changed: yes/no, provenance)
- `phase45.semantics.proof.rendered` - Proof page viewed
- `phase45.semantics.proof.dismissed` - Proof dismissed
- `phase45.semantics.cue.computed` - Cue computed

### Invariants

1. **Effect Always No-Power**: SemanticsEffect MUST be effect_no_power
2. **No Behavior Change**: Semantics do NOT affect decision/delivery/execution
3. **Hash-Only Storage**: Only circle_id_hash stored, no raw identifiers
4. **Pipe-Delimited Canonical**: CanonicalStringV1 uses pipe delimiters
5. **Bounded Retention**: 30 days OR 200 records max
6. **No Goroutines**: In pkg/ and internal/ Phase 45 code
7. **Clock Injection**: No time.Now() in domain/engine
8. **No Forbidden Imports**: Cannot import/be imported by decision packages

### Cue Rules

Semantics cue shows at LOWEST priority (below Phase 27) only when:
- At least one circle has semantic_unknown
- AND user has connected something (gmail or truelayer)
- AND proof not dismissed this period

Cue text: "You can name what kind of thing this is."

## Consequences

### Positive

- **Legibility**: System can explain what kind of source each circle is
- **User agency**: Users can declare semantics explicitly
- **Future foundation**: Enables Phase 46 registry and preview refinements
- **No risk**: Cannot affect behavior in Phase 45 (effect_no_power enforced)

### Negative

- **Additional complexity**: New domain types, engine, stores
- **No immediate benefit**: Semantics don't change behavior yet
- **Maintenance burden**: Must keep guardrails preventing misuse

### Neutral

- No new actions enabled
- No new delivery logic
- No urgency changes
- Pure meaning layer

## Non-Goals

These are explicitly NOT goals of Phase 45:

1. **Does NOT change decision gate** - Semantics do not affect PressureDecision
2. **Does NOT enable interrupts** - Cannot escalate to INTERRUPT_CANDIDATE
3. **Does NOT add urgency delivery** - Cannot trigger any delivery based on urgency
4. **Does NOT import into decision packages** - Guardrailed to prevent wiring
5. **Does NOT store identifiers** - Only circle_id_hash (already hashed)

## Security Considerations

- Hash-only storage (no raw identifiers)
- POST-only for mutations
- No sensitive data in semantics
- Bounded retention prevents unbounded growth
- Effect enum prevents future misuse until Phase 46+

## Implementation Notes

### Guardrails (120+ checks)

Key sections:
- File existence and package headers
- No goroutines, clock injection
- Forbidden patterns (emails, URLs, amounts)
- Domain enums and structs
- Engine methods
- Store methods
- Bounded retention
- Web routes and templates
- No forbidden imports (pressuredecision, interrupt*, pushtransport, etc.)

### Demo Tests (25+ tests)

Key tests:
- Determinism: same inputs → same hashes
- Validate() rejects invalid enums
- Default derivation matches rules
- ApplyUserDeclaration enforces provenance and effect_no_power
- Store FIFO eviction
- Proof ack dismissal
- Cue logic (unknown + connected + not dismissed)
- "No Behavior Change" test (engine produces no Decision types)

## References

- Phase 31.4: External Pressure Circles
- Phase 32: Pressure Decision
- Phase 33: Interrupt Permission Contract
- Phase 44.2: Enforcement Wiring Audit
