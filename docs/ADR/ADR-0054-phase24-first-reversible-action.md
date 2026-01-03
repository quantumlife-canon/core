# ADR-0054: Phase 24 — First Reversible Real Action (Trust-Preserving)

## Status

Accepted

## Context

After Phase 23's gentle action invitations, we need to enable the first real action
that circles can take through QuantumLife. This action must be:

1. **Explicitly requested** — never auto-triggered
2. **Reversible** — preview only, no execution
3. **Singular** — operates on exactly one held item
4. **Bounded** — one action per period maximum
5. **Trust-preserving** — requires proven trust before offering

The system has earned trust through:
- Gmail connected and synchronized
- Quiet baseline verified
- Mirror viewed (transparency proven)
- Trust accrual exists

Now the system may offer ONE action per day, but only as a preview. No execution.
This establishes the pattern for future real actions while maintaining safety.

## Decision

### Action Model

```
ActionKind: preview_only (only value in Phase 24)
ActionState: offered → viewed → dismissed | acknowledged
```

### Eligibility Requirements

ALL must be true for an action to be offered:
1. HasGmailConnection — Gmail is connected
2. HasQuietBaseline — Quiet baseline verified
3. HasMirrorViewed — Circle has viewed the mirror
4. HasTrustAccrual — Trust score > 0
5. !HasPriorActionThisPeriod — No action taken today
6. HasHeldItems — At least one held item exists

### Selection Algorithm

When eligible, exactly ONE held item is selected:
- Deterministic selection by lowest hash value
- Same inputs always produce same selection
- No randomness, no priority queues

### Action Flow

```
Circle visits /today
  ↓
Whisper cue appears (if eligible): "If you'd like, we can look at one thing together."
  ↓
Circle clicks whisper → /action/once
  ↓
Action page: "Once, together. We'll look at one thing. Then we'll stop."
  ↓
Circle clicks "Show me" → /action/once/run (POST)
  ↓
Preview page: Shows abstract category, horizon, magnitude
  Disclaimer: "This is a preview. We did not act."
  ↓
Circle clicks "Hold this" or "Dismiss"
  ↓
Silence resumes. No lingering prompts.
```

### Abstract Data Only

The preview shows ONLY abstract information:
- **Category**: money | time | work | people | home
- **Horizon**: soon | later | someday
- **Magnitude**: small | medium | large
- **SourceHash**: Hash of the held item (not the item itself)

No identifiable data (subjects, bodies, vendors, amounts) is shown.

### Period Enforcement

```go
type ActionPeriod struct {
    DateBucket string  // "2024-01-15"
    PeriodHash string  // SHA256("ACTION_PERIOD|v1|2024-01-15")
}
```

One action per period (daily bucket). After any action state beyond "offered",
no more actions can be offered that period.

### Persistence

```go
type ActionRecord struct {
    ActionHash string      // Hash of the preview
    State      ActionState // viewed | dismissed | acknowledged
    PeriodHash string      // Hash of the period
    CircleID   string      // Circle identifier
}
```

Records are:
- Hash-only (no raw content)
- Append-only (never modified)
- Bounded retention (max 1000 entries)
- Period-scoped

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/action/once` | GET | Show action invitation page |
| `/action/once/run` | POST | Execute preview (NOT action) |
| `/action/once/dismiss` | POST | Dismiss and resume silence |

### Events

```go
Phase24InvitationOffered  // Action page loaded
Phase24ActionViewed       // Preview was viewed
Phase24ActionDismissed    // Circle dismissed
Phase24PreviewRendered    // Preview page shown
Phase24PeriodClosed       // Period ended (dismissed or acknowledged)
```

## Consequences

### Positive

1. **Trust-preserving**: Action only offered after trust proven
2. **Safe by design**: Preview only, no execution possible
3. **Bounded exposure**: One per day maximum
4. **Deterministic**: Same inputs produce same selection
5. **Reversible**: Dismissal has zero cost, silence resumes

### Negative

1. **Limited utility**: Preview-only is not very actionable
2. **Single item**: Cannot batch multiple held items
3. **Daily limit**: Circles wanting more must wait

### Neutral

1. Sets pattern for future phases with real execution
2. Proves the action model before adding risk
3. Establishes period-based rate limiting

## Constraints

| Constraint | Enforcement |
|------------|-------------|
| No execution | ActionKind only has preview_only |
| No goroutines | Guardrail scan |
| No time.Now() | Clock injection pattern |
| Hash-only persistence | No raw content stored |
| One per period | HasActionThisPeriod check |
| Deterministic selection | Lowest hash wins |
| Stdlib only | No external imports |
| Pipe-delimited canonical | No JSON marshaling |

## Calm Language

All UI text uses calm, non-urgent language:

```
"Once, together."
"We'll look at one thing. Then we'll stop."
"Nothing will be sent. Nothing will change."
"This will wait. No rush."
"This is a preview. We did not act."
"Quiet resumes."
"Nothing to look at."
"Everything is being held quietly."
```

No urgency, no fear, no shame, no blame.

## Verification

```bash
# Build
go build ./...

# Tests
go test ./internal/demo_phase24_first_action/...

# Guardrails
./scripts/guardrails/first_action_enforced.sh
```

## Future Phases

Phase 24 establishes the foundation for:
- Phase 25+: First real execution (with undo window)
- Phase 26+: Multi-item batch actions
- Phase 27+: Cross-circle coordination

Each future phase will extend this model while preserving its safety properties.

## References

- ADR-0053: Phase 23 — Gentle Action Invitation
- ADR-0052: Phase 22 — Quiet Mirror (Dual-Proof)
- ADR-0051: Phase 21 — Unified Onboarding + Shadow Receipt Viewer
