# ADR-0087: Phase 49 — Vendor Reality Contracts

## Status

Accepted

## Context

Phase 48 introduced Market Signal Binding (Non-Extractive Marketplace v1) with `effect_no_power` semantics.
Marketplace vendors (external circles) now need accountability and urgency realism without "growthy" pressure.

Problem: Vendors want to participate in the marketplace without being able to:
- Apply excessive pressure
- Bypass user interrupt policy
- Override attention envelopes
- Create urgency where none exists

Phase 49 introduces **Vendor Reality Contracts** — vendor-declared caps that can only REDUCE pressure,
integrated via a single choke-point clamp function.

## Decision

### Invariant 1: HOLD-First, Clamp-Only

Vendor contracts can only REDUCE pressure, never increase it.
Default for any invalid or missing contract is `allow_hold_only`.

### Invariant 2: Commerce Cap at SURFACE_ONLY

Commerce vendors are ALWAYS capped to at most `allow_surface_only`, even if vendor declares higher.
This is a hard boundary that cannot be overridden.

### Invariant 3: Cannot Override User Policy

Vendor contracts CANNOT override:
- User interrupt policy (Phase 33)
- Attention envelope (Phase 39)
- Existing HOLD-only contracts (Phase 42, 44)

### Invariant 4: Single Choke-Point Integration

All vendor contract caps are applied via the enforcement clamp pattern.
There is ONE integration point in the pressure pipeline, not many.

### Invariant 5: Hash-Only Storage

Vendor contracts store ONLY:
- VendorCircleHash (SHA256 of circle ID)
- Scope, allowance, frequency, emergency buckets (enums)
- Period key
- Contract hash

Vendor contracts do NOT store:
- Vendor names, emails, URLs
- Merchant strings or tokens
- Raw timestamps
- Amounts or currency

### Invariant 6: Bounded Retention

- Maximum 200 records OR 30 days
- FIFO eviction
- Period-key based age calculation (no wallclock)

### Invariant 7: No Execution Power

Vendor contracts are declarative caps only. They CANNOT:
- Deliver notifications
- Execute actions
- Trigger interrupts
- Grant permissions

### Invariant 8: Deterministic Proof

All contract outcomes are deterministic and proofable.
Same inputs produce same outputs (hash-based verification).

## Non-Goals

Phase 49 does NOT:
- Grant vendors execution power
- Enable notification delivery
- Store vendor identity strings
- Process payments or billing
- Rank or search vendors
- Create vendor analytics

## Architecture

```
┌─────────────────────────┐
│  Vendor declares        │
│  contract constraints   │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  ValidateContract()     │
│  - Validate enums       │
│  - Check required fields│
│  - Reject invalid       │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  ComputeEffectiveCap()  │
│  - Apply commerce cap   │
│  - Compute min(req,cap) │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│  Pressure Pipeline      │────▶│  ClampPressureAllowance │
│  (Phase 32 decision)    │     │  (single choke-point)   │
└─────────────────────────┘     └───────────┬─────────────┘
                                            │
                                            ▼
                                ┌─────────────────────────┐
                                │  Clamped outcome        │
                                │  (never higher than cap)│
                                └─────────────────────────┘
```

## Enums

| Enum                | Values                                                           |
|---------------------|------------------------------------------------------------------|
| ContractScope       | scope_commerce, scope_institution, scope_health, scope_transport, scope_unknown |
| PressureAllowance   | allow_hold_only, allow_surface_only, allow_interrupt_candidate   |
| FrequencyBucket     | freq_per_day, freq_per_week, freq_per_event                      |
| EmergencyBucket     | emergency_none, emergency_human_only, emergency_institution_only |
| DeclaredByKind      | declared_vendor_self, declared_regulator, declared_marketplace   |
| ContractStatus      | status_active, status_revoked                                    |
| ContractReasonBucket| reason_ok, reason_invalid, reason_commerce_capped, reason_no_power, reason_rejected |

## Pressure Allowance Ordering

```
allow_hold_only < allow_surface_only < allow_interrupt_candidate
```

The clamp function returns `min(current, cap)` based on this ordering.

## Routes

| Method | Path                     | Purpose                          |
|--------|--------------------------|----------------------------------|
| GET    | /vendor/contract         | View vendor contract status      |
| POST   | /vendor/contract/declare | Declare/update vendor contract   |
| POST   | /vendor/contract/revoke  | Revoke vendor contract           |
| GET    | /proof/vendor            | View vendor contract proof       |
| POST   | /proof/vendor/dismiss    | Dismiss vendor proof cue         |

## Events

| Event                              | When                                |
|------------------------------------|-------------------------------------|
| phase49.vendor_contract.declared   | Contract declared/updated           |
| phase49.vendor_contract.revoked    | Contract revoked                    |
| phase49.vendor_contract.applied    | Contract applied during decision    |
| phase49.vendor_contract.clamped    | Contract caused outcome to be clamped|
| phase49.vendor_proof.rendered      | Vendor proof page rendered          |

## Storelog Records

| Record Type                    | Purpose                         |
|--------------------------------|---------------------------------|
| VENDOR_CONTRACT                | Contract declared or updated    |
| VENDOR_CONTRACT_REVOCATION     | Contract revoked                |

## Consequences

### Positive
- Vendors have accountability for declared pressure limits
- Commerce is always bounded regardless of vendor declaration
- User policy remains authoritative
- Single integration point reduces complexity

### Negative
- Vendors cannot escalate beyond declared limits
- No "urgent sale" or "limited time" pressure patterns
- No vendor identity in proof pages

### Neutral
- Implementation is intentionally restrictive
- No vendor-side analytics or metrics
- Silence is correct output for invalid contracts

## Threat Model

| Threat                          | Mitigation                                    |
|---------------------------------|-----------------------------------------------|
| Vendor declares high allowance  | Commerce cap at SURFACE_ONLY regardless       |
| Vendor bypasses contract        | Single choke-point clamp enforces at runtime  |
| Contract data reveals vendor ID | Hash-only storage, no names/emails/URLs       |
| Time-based gaming               | Period-key based, no raw timestamps           |

## References

- Phase 33: Interrupt Permission Contract
- Phase 39: Attention Envelopes
- Phase 42: Delegated Holding Contracts
- Phase 44: Cross-Circle Trust Transfer
- Phase 44.2: Enforcement Wiring Audit
- Phase 48: Market Signal Binding
- ADR-0082: Phase 44.2 Enforcement Wiring Audit
- ADR-0086: Phase 48 Market Signal Binding
