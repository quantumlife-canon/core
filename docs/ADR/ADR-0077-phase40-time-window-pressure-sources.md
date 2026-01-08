# ADR-0077: Phase 40 - Time-Window Pressure Sources

## Status
Accepted

## Context

Phase 32 introduced the Pressure Decision Gate, which classifies pressure signals
into HOLD, SURFACE, or INTERRUPT_CANDIDATE based on magnitude, horizon, circle
type, and trust status.

Phase 31.4 introduced External Pressure Circles derived from commerce observations.

However, real life creates "time windows" that deserve attention:
- Calendar appointments approaching
- Deadlines in inbox messages
- People waiting for responses
- Travel connections coming up
- Health-related time windows

These time windows represent legitimate pressure sources that should feed into
the existing pipeline, but they come from fundamentally different sources than
commerce observations.

### Why Windows ≠ Merchants

Commerce-derived pressure (Phase 31.4) captures external commercial forces:
- Delivery tracking
- Subscription renewals
- Purchase-related messages

Time windows capture human-centric pressure:
- Someone is waiting for your response
- An appointment is approaching
- A deadline is near

The key difference:
- **Commerce** = external entity wants your attention
- **Time Windows** = reality creates temporal pressure

Commerce should NEVER interrupt. Time windows may inform interrupt decisions
(through the existing Phase 32-36 pipeline).

### Why Calendar is Authoritative

Calendar events are explicitly scheduled by the user. They represent:
- Known commitments
- Agreed appointments
- Self-imposed deadlines

This makes calendar the most authoritative source for time windows:
1. User explicitly created the event
2. Event has defined time bounds
3. Event represents a commitment

Source precedence: calendar > inbox_institution > inbox_human > device_hint

### Modeling "Waiting Humans" Abstractly

When a human sends a message and awaits response, this creates temporal pressure.
We model this WITHOUT exposing:
- Who is waiting (no identity)
- What they said (no content)
- When they sent it (no timestamp)

Instead we capture:
- Circle type: human vs institution
- Magnitude bucket: nothing / a_few / several
- Window horizon: now / soon / today / later
- Evidence hashes (for audit, not identity)

This allows the system to recognize "someone is waiting" without knowing who.

## Decision

Implement Phase 40 as **observation only**:

1. **No delivery**: Phase 40 does not deliver interrupts
2. **No execution**: Phase 40 does not trigger actions
3. **No notifications**: Phase 40 does not send signals externally

Phase 40 produces TimeWindowSignal values that feed into the existing pipeline:
```
Phase 40 (Time Windows) → Phase 31.4 (External Pressure) → Phase 32 (Decision Gate) → Phase 33 (Permission) → Phase 34 (Preview) → Phase 36 (Delivery)
```

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Phase 40: Time Windows                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │  Calendar   │  │   Inbox     │  │   Device    │                 │
│  │   Inputs    │  │   Inputs    │  │   Hints     │                 │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                 │
│         │                │                │                         │
│         └────────────────┼────────────────┘                         │
│                          │                                          │
│                          ▼                                          │
│              ┌───────────────────────┐                              │
│              │   TimeWindow Engine   │                              │
│              │   (Pure, Deterministic)│                              │
│              └───────────┬───────────┘                              │
│                          │                                          │
│                          ▼                                          │
│              ┌───────────────────────┐                              │
│              │  0-3 TimeWindowSignals │                              │
│              │  (max 1 per CircleType)│                              │
│              └───────────┬───────────┘                              │
│                          │                                          │
└──────────────────────────┼──────────────────────────────────────────┘
                           │
                           ▼
              ┌───────────────────────┐
              │  Adapter → PressureInput │
              └───────────┬───────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  Phase 32: Decision Gate │
              └───────────────────────┘
```

### Bounded Effects

Time windows only modify the **input** to Phase 32. They cannot:
- Bypass Phase 33 permission contracts
- Bypass Phase 34 preview requirements
- Force Phase 36 delivery
- Change execution eligibility

Effects are strictly bounded:
- **Max 3 signals** per build
- **Max 1 signal** per CircleType
- **Max 3 evidence hashes** per signal
- **Envelope shift**: max 1 step (later→today, today→soon, soon→now)
- **Commerce exclusion**: commerce sources NEVER appear in time windows

### Commerce Exclusion

Commerce MUST NOT appear as a time window source. Rationale:
1. Commerce pressure is handled by Phase 31.4 External Pressure Circles
2. Commerce should never interrupt (fundamental canon principle)
3. Mixing commerce into time windows would enable commerce escalation

The engine explicitly filters out any commerce-related signals.

### Integration with Phase 39 (Attention Envelopes)

When an attention envelope is active:
- Window kind may shift by **at most 1 step** earlier
- Envelope may NOT create new windows
- Only certain envelope kinds trigger shifts (on_call, travel, emergency)
- Working envelope does NOT shift windows

This preserves the "calm by default" principle while allowing temporary heightened awareness.

## Consequences

### Positive

1. **Real-life awareness**: System can recognize temporal pressure without content access
2. **Privacy preserved**: No identifiers, no content, no timestamps stored
3. **Pipeline integration**: Uses existing Phase 32-36 infrastructure
4. **Bounded scope**: Observation only, cannot force delivery
5. **Deterministic**: Same inputs + clock = same outputs

### Negative

1. **Requires calendar/inbox integration**: Full value needs data sources
2. **Abstract signals**: Cannot show "who" or "what", only abstract pressure

### Neutral

1. **Web routes for proof**: /reality/windows shows abstract observation state
2. **Storage bounded**: 30-day retention, max 500 records, FIFO eviction

## Implementation

### Package Structure

```
pkg/domain/timewindow/
├── types.go           # Enums, structs, validation

internal/timewindow/
├── engine.go          # Pure engine functions

internal/persist/
├── timewindow_store.go  # Hash-only persistence
```

### Web Routes

- `GET /reality/windows` - View current window state
- `POST /reality/windows/run` - Run observation (builds signals)

### Events

- `phase40.windows.build_requested`
- `phase40.windows.built`
- `phase40.windows.persisted`
- `phase40.windows.viewed`
- `phase40.windows.cue_dismissed`

### Storelog Records

- `TIME_WINDOW_SIGNAL`
- `TIME_WINDOW_RESULT`

## Why Phase 40 Never Delivers Interrupts

Phase 40 is explicitly **observation only**. Reasons:

1. **Separation of concerns**: Observation is separate from action
2. **Pipeline integrity**: Phase 33-36 handle delivery decisions
3. **Trust preservation**: No new delivery paths that bypass permission contracts
4. **Calm by default**: Observation doesn't mean interruption

The pipeline remains:
```
Observe (Phase 40) → Classify (Phase 32) → Permission (Phase 33) → Preview (Phase 34) → Delivery (Phase 36)
```

Phase 40 only participates in the first step. It produces inputs for the classifier.

## References

- ADR-0068: Phase 32 - Pressure Decision Gate
- ADR-0069: Phase 33 - Interrupt Permission Contract
- ADR-0070: Phase 34 - Permitted Interrupt Preview
- ADR-0073: Phase 36 - Interrupt Delivery Orchestrator
- ADR-0076: Phase 39 - Temporal Attention Envelopes
- ADR-0067: Phase 31.4 - External Pressure Circles
