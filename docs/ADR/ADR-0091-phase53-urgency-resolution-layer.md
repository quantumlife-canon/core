# ADR-0091: Phase 53 — Urgency Resolution Layer

**Status:** Accepted
**Date:** 2025-01-09
**Phase:** 53

## Context

QuantumLife has accumulated multiple sources of pressure and urgency signals across phases:
- Pressure outcomes (Phase 20)
- Time windows (Phase 38)
- Envelope activity (Phase 39)
- Vendor contracts (Phase 49)
- Circle semantics and necessity (Phase 45)
- Trust fragility (Phase 26)

However, there is no unified layer that:
1. Combines these signals into a single resolution
2. Applies deterministic caps and clamps
3. Produces a privacy-safe, hash-only proof

This creates fragmented decision-making and makes it hard to verify that the system respects its invariants.

## Decision

Create an **Urgency Resolution Layer** — a deterministic, privacy-safe resolution engine that:
1. Converts existing pressure signals + contracts into an abstract `UrgencyResolution`
2. Applies cap-only, clamp-only rules (never escalates, only reduces)
3. Persists hash-only records
4. Renders a proof page

### Key Design Principles

1. **NO NEW POWER**
   - This phase produces NO delivery and NO execution
   - It is observation/proof only
   - No push, no notify, no observers

2. **CAP-ONLY, CLAMP-ONLY**
   - Caps can only reduce escalation; never increase power
   - Commerce always gets `cap_hold_only`
   - Vendor contract caps are applied as min() clamp
   - Trust fragile clamps max to `cap_surface_only`

3. **DETERMINISTIC**
   - Same inputs + same clock period = same resolution hash
   - Reasons are deterministically sorted
   - All state derivation is pure

4. **PRIVACY-SAFE**
   - Hash-only storage (no raw identifiers)
   - Bucket-only values (no timestamps, no counts)
   - No vendor names, merchant strings, emails, URLs, amounts

### Resolution Rules

1. **Rule 0 (Default HOLD)**: Level=urg_none, Cap=cap_hold_only, reason=default_hold
2. **Rule 1 (Commerce)**: Commerce circle type always gets cap_hold_only
3. **Rule 2 (Vendor Cap)**: Apply VendorCap as min() clamp
4. **Rule 3 (Trust Fragile)**: Clamps max to cap_surface_only
5. **Rule 4 (Window Signal)**: May raise Level by +1 step max (none->low->medium)
6. **Rule 5 (Institution Deadline)**: Institution + soon + several => propose cap_surface_only
7. **Rule 6 (Human Now)**: Human + now => propose cap_interrupt_candidate_only
8. **Rule 7 (Envelope Active)**: Allows one-step Level shift up, but never exceed cap
9. **Rule 8 (Necessity)**: Can only reduce (never increase); if false for institution, clamp max surface
10. **Rule 9 (Reasons)**: Deterministic sort by enum string; keep first 3
11. **Rule 10 (Status)**: status_ok if no clamp; status_clamped if any clamp reduced

### Enums

- `UrgencyLevel`: urg_none, urg_low, urg_medium, urg_high
- `EscalationCap`: cap_hold_only, cap_surface_only, cap_interrupt_candidate_only
- `UrgencyReasonBucket`: reason_time_window, reason_institution_deadline, reason_human_now, reason_trust_protection, reason_vendor_contract_cap, reason_semantics_necessity, reason_envelope_active, reason_default_hold
- `ResolutionStatus`: status_ok, status_clamped, status_rejected
- `CircleTypeBucket`: bucket_human, bucket_institution, bucket_commerce, bucket_unknown
- `RecencyBucket`: rec_never, rec_recent, rec_stale

### Routes

- `GET /proof/urgency` — View urgency resolution proof
- `POST /proof/urgency/run` — Run and persist resolution
- `POST /proof/urgency/dismiss` — Dismiss urgency cue

### Storage

- Append-only, hash-only store
- Dedup on period|circle|resolutionHash
- Retention: 30 days OR 500 records FIFO
- Never stores raw identifiers

## Non-Goals

- **No delivery** — This phase does not send push notifications
- **No execution** — This phase does not run any actions
- **No observers** — This phase does not add any observers
- **No escalation** — Caps can only reduce, never increase

## Consequences

### Positive
- Unified urgency resolution from multiple signals
- Deterministic, verifiable hash-based proof
- Privacy-safe (hash-only, bucket-only)
- Composes with existing cue/whisper infrastructure
- Clear invariants prevent scope creep

### Negative
- Adds another layer of abstraction
- Requires adapter interfaces for sources that may not exist yet
- Complexity in understanding the rule interactions

### Neutral
- Resolution hash changes when underlying signals change
- Cue visibility follows single-whisper-rule priority

## Invariants

1. **NO POWER**: This package is cap-only, clamp-only. It MUST NOT deliver push, execute anything, or add any observers. Proof only.
2. **HASH-ONLY**: Never store or render raw identifiers
3. **NO TIMESTAMPS**: Only recency/horizon buckets
4. **NO COUNTS**: Only magnitude buckets
5. **DETERMINISTIC**: Same inputs = same resolution hash
6. **PIPE-DELIMITED**: Canonical strings use | delimiter, never JSON
7. **COMMERCE NEVER ESCALATES**: Always cap_hold_only
8. **CAPS ONLY REDUCE**: Never increase power
9. **REASONS MAX 3**: Sorted, capped at 3
10. **POST-ONLY MUTATIONS**: Run and dismiss endpoints are POST-only

## References

- Phase 20: Trust accrual
- Phase 26: Reality check / trust fragility
- Phase 38: Time windows
- Phase 39: Envelope activity
- Phase 45: Circle semantics and necessity
- Phase 49: Vendor contracts
- Phase 52: Proof hub (pattern reference)
