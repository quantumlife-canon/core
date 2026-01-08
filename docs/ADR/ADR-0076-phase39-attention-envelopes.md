# ADR-0076: Phase 39 - Temporal Attention Envelopes

## Status
Accepted

## Context

Phase 32 introduced the Pressure Decision Gate, which classifies pressure signals
into HOLD, SURFACE, or INTERRUPT_CANDIDATE based on magnitude, horizon, circle
type, and trust status.

However, real life creates legitimate temporary attention windows:
- On-call duty requiring faster response
- Travel transit with time-sensitive connections
- Deadlines requiring focused awareness
- Family emergencies requiring heightened attention

Traditional systems handle this through:
- "Do Not Disturb" modes (suppress everything)
- Priority contacts (static, not time-bounded)
- Manual notification settings (complex, permanent)

None of these address the core need: "For the next few hours, I want the
system to be *slightly more responsive* to genuine pressure."

## Decision

Introduce Attention Envelopes: time-boxed, explicit, revocable windows that
modify pressure input BEFORE Phase 32 processing.

### Core Principles

1. **Explicit, not automatic**
   - User starts envelope via POST only
   - Never auto-enabled based on calendar, location, or patterns
   - No inference, no ML, no background detection

2. **Bounded, not open-ended**
   - Fixed duration buckets: 15m, 1h, 4h, day
   - Auto-expires deterministically based on 15-minute period arithmetic
   - No extension without explicit restart

3. **Revocable, not sticky**
   - User can stop envelope early via POST
   - No confirmation dialogs, no friction
   - Immediate effect

4. **Minimal, not transformative**
   - Horizon shift: max 1 step earlier (later→soon, soon→now)
   - Magnitude bias: max +1 bucket (nothing→a_few, a_few→several)
   - Cannot force interrupts (Phase 33 permission still applies)

5. **Commerce-excluded**
   - Envelope NEVER escalates commerce-origin pressure
   - Commerce always stays at baseline (HOLD path)
   - No dark patterns, no purchase urgency

### Why Not "Modes" or "Preferences"?

Modes are persistent states that require explicit exit.
Preferences are permanent settings that affect all future behavior.

Envelopes are temporal windows:
- Start time is explicit (POST)
- End time is deterministic (start + duration)
- No state persists after expiry
- No "I forgot to turn it off" scenarios

### Effect Table by Kind

| Kind | Horizon Shift | Magnitude Bias | Cap Delta | Use Case |
|------|---------------|----------------|-----------|----------|
| none | 0 | 0 | 0 | Baseline (default) |
| on_call | 1 step earlier | +1 | +1 | Duty/rotation periods |
| working | 0 | +1 | 0 | Focused work sessions |
| travel | 1 step earlier | 0 | 0 | Transit connections |
| emergency | 1 step earlier | +1 | +1 | Family/health situations |

### Duration Buckets

| Bucket | Duration | Use Case |
|--------|----------|----------|
| 15m | 15 minutes | Quick tasks, arriving soon |
| 1h | 1 hour | Meetings, short windows |
| 4h | 4 hours | Half-day focus |
| day | 24 hours | On-call shifts, travel days |

### Integration Point

Envelope modifies ONLY the PressureDecisionInput BEFORE Phase 32:

```
Pressure Sources (31.4, 38, etc.)
         ↓
   [Build PressureInput]
         ↓
   [Phase 39: ApplyEnvelope()] ← Modifies Horizon, Magnitude
         ↓
   [Phase 32: Classify()]
         ↓
   Phase 33/34/36 (unchanged)
```

Envelope does NOT:
- Bypass Phase 33 permission checks
- Skip Phase 34 preview
- Trigger Phase 36 delivery
- Change execution eligibility (Phase 25/28)
- Store raw timestamps or identifiers

### Why Commerce is Excluded

Commerce pressure (deliveries, subscriptions, payments) should never be
escalated by user attention state because:

1. Commerce entities would exploit "attention mode" to create urgency
2. "Your package is arriving" becomes "YOUR PACKAGE IS ARRIVING"
3. Purchase-related pressure should stay calm regardless of user state
4. External commercial interests don't get attention amplification

Implementation: `if CircleType == "commerce" { return input unchanged }`

## Critical Invariants

1. **No envelope = no change** - Default behavior unchanged
2. **Envelope explicit** - POST only, never auto-started
3. **Envelope bounded** - Fixed durations, auto-expires
4. **Envelope revocable** - Stop anytime, immediate effect
5. **Effects bounded** - Max 1 step horizon, +1 magnitude, +1 cap
6. **Commerce excluded** - Never escalated
7. **No forced interrupts** - Phase 33/34 still apply
8. **Deterministic** - Same inputs + clock = same outputs
9. **No time.Now()** - Clock injection only
10. **No goroutines** - Synchronous processing
11. **stdlib only** - No external dependencies
12. **Hash-only storage** - No raw timestamps, no identifiers

## Proof and Audit

Envelope state is visible through:

1. **GET /envelope** - Current envelope state (active/inactive, duration bucket)
2. **GET /proof/envelope** - Receipt history (hashes only, no timestamps)

UI shows ONLY:
- "Active" / "Inactive"
- Duration bucket ("4h", "day", etc.)
- Kind bucket ("on_call", "travel", etc.)

UI does NOT show:
- Start time
- End time
- Remaining time
- Countdown

This prevents clock-watching and time anxiety.

## Consequences

### Positive
- Users can temporarily adjust responsiveness for legitimate needs
- Time-bounded by design prevents permanent state drift
- Commerce exclusion prevents dark pattern exploitation
- Bounded effects prevent system gaming
- Deterministic expiry prevents forgotten states

### Negative
- Cannot fine-tune effects per-circle or per-source
- No ML-based auto-suggestion of envelopes
- No location-based auto-activation
- Requires explicit user action to start

### Neutral
- One active envelope per circle at a time
- 30-day receipt retention for proof pages
- Max 200 records with FIFO eviction

## Related

- ADR-0068: Phase 32 Pressure Decision Gate (modified input target)
- ADR-0069: Phase 33 Interrupt Permission Contract (unchanged)
- ADR-0070: Phase 34 Permitted Interrupt Preview (unchanged)
- ADR-0067: Phase 31.4 External Pressure Circles (pressure source)
- ADR-0075: Phase 38 Mobile Notification Metadata Observer (pressure source)
