# ADR-0090: Phase 52 — Proof Hub + Connected Status

**Status:** Accepted
**Date:** 2025-01-09
**Phase:** 52

## Context

Users of QuantumLife experience a "mock vibe" problem — the inability to tell whether the system is actually connected to real services, syncing data, or operating in a meaningful way. This creates a trust gap where users cannot verify that the system is behaving as expected.

Previous phases have built:
- Gmail and TrueLayer connection infrastructure (Phase 18.6, 29)
- Shadow provider health monitoring (Phase 19)
- Device registration (Phase 37)
- Transparency log for signed claims/manifests (Phase 51)
- Trust accrual and reality check mechanisms (Phase 20, 26C)

However, there is no single place that answers: "Is this real, connected, and behaving quietly?"

## Decision

Create a **Proof Hub** — a single page that shows connected status without exposing identifiers, content, counts, timestamps, vendors, merchants, or people.

### Key Design Principles

1. **Abstract-Only Display**
   - Connection status as yes/no only
   - Sync recency as buckets only (never/recent/stale)
   - Activity as magnitude buckets only (nothing/a_few/several)
   - No exact timestamps, counts, or identifiers

2. **Deterministic Status Hash**
   - Same inputs + same clock period = same status hash
   - Hash computed over pipe-delimited canonical string (not JSON)
   - Enables verification that nothing changed

3. **Observation/Proof Only**
   - NO execution triggered from proof hub
   - NO delivery, polling, or goroutines
   - Pure read-only presentation of existing state

4. **Hash-Only Storage**
   - Acks store only: circleIDHash, periodKey, statusHash
   - Never stores raw identifiers
   - Bounded retention: 30 days OR 200 entries max

### Sections

1. **Identity** — Circle hash (short prefix only)
2. **Connections** — Gmail/TrueLayer/Device status
3. **Sync** — Recency buckets + magnitude buckets
4. **Shadow** — Provider kind, real allowed, health status
5. **Ledger** — Transparency log magnitude, ledger recency
6. **Invariants** — Hard-coded: "No background execution", "Hash-only storage", "Silence is default"

### Cue Integration

The proof hub cue integrates into the `/today` whisper chain at low priority:
- Text: "Proof is available — quietly."
- Dismissed per period+statusHash
- Reappears if status hash changes

## Non-Goals

- **No dashboards** — This is not analytics
- **No tips or recommendations** — Observation only
- **No notifications** — Never initiates contact
- **No background refresh** — On-demand only

## Consequences

### Positive
- Users can verify real connections exist
- Deterministic hash enables external verification
- No privacy leakage (hash-only, bucket-only)
- Composes with existing cue/whisper infrastructure

### Negative
- Adds another page to maintain
- Requires adapter interfaces for stores that don't exist yet

### Neutral
- Status hash changes when underlying state changes
- Cue visibility follows single-whisper-rule priority

## Invariants

1. **NO POWER**: This package is observation/proof only
2. **HASH-ONLY**: Never store or render raw identifiers
3. **NO TIMESTAMPS**: Only recency buckets (never, recent, stale)
4. **NO COUNTS**: Only magnitude buckets (nothing, a_few, several)
5. **DETERMINISTIC**: Same inputs = same status hash
6. **PIPE-DELIMITED**: Canonical strings use | delimiter, never JSON
7. **NO FORBIDDEN PATTERNS**: Never include @, http, merchant, amount, etc.

## References

- Phase 18.6: Connection infrastructure
- Phase 19: Shadow provider health
- Phase 20: Trust accrual
- Phase 26C: Reality check
- Phase 29: TrueLayer finance mirror
- Phase 37: Device registration
- Phase 51: Transparency log
