# ADR-0051: Phase 21 - Unified Onboarding + Shadow Receipt Viewer

## Status

Accepted

## Context

After implementing real Gmail OAuth (Phase 18.8), quiet verification (Phase 18.9), and the shadow mode pipeline (Phase 19.x), users need:

1. A unified entry point that explains what the system does and doesn't do
2. Visual feedback about their current state (demo, connected, shadow)
3. A way to view shadow observation proofs without exposing raw content

The system has accumulated multiple connection states and operating modes that need to be surfaced clearly while maintaining the "quiet" philosophy.

## Decision

### 1. Unified Onboarding Page (`/onboarding`)

Create a single entry point with calm, minimal, truthful copy:

```
What we do:
- We read your email headers (never content)
- We notice patterns
- We suggest, never act
- All proofs are recorded

What we don't do:
- We never send emails
- We never make purchases
- We never share data
- We never nag
```

### 2. Mode Indicator (Derived, Not Stored)

Mode is **derived** from existing state, not a configuration setting:

| Mode | Condition |
|------|-----------|
| Demo | No Gmail connection OR shadow provider is stub-only |
| Connected | Gmail connected but no shadow receipt for current period |
| Shadow | Shadow receipt exists for current period |

```go
type Mode string

const (
    ModeDemo      Mode = "demo"
    ModeConnected Mode = "connected"
    ModeShadow    Mode = "shadow"
)
```

### 3. Shadow Receipt Viewer (`/shadow/receipt`)

Displays shadow observation proofs with ONLY abstract buckets and hashes:

**Sections:**
- Source: "Connected: email (read-only)" or "No sources connected"
- Observation: magnitude bucket, horizon bucket, categories
- Confidence: bucket only (low/medium/high)
- Restraint: always "No actions taken", "No drafts created", etc.
- Calibration: agreement bucket and vote status (if available)
- Trust Anchor: period label and receipt hash

### 4. Whisper Rule Integration

Receipt cue follows single whisper rule:
- At most ONE cue shown on `/today`
- Priority: surface cue > proof cue > receipt cue
- Cue dismissed for current period stays dismissed
- Acknowledgement stored as hash only

### 5. Ack Store

Stores acknowledgements using hash-only pattern:

```go
type AckRecord struct {
    Action       AckAction  // viewed or dismissed
    ReceiptHash  string     // SHA256 hash only
    TSHash       string     // Timestamp hash, never raw
    PeriodBucket string     // YYYY-MM-DD
}
```

Bounded size (default 128 records) with LRU eviction.

## File Structure

```
internal/mode/
├── model.go          # Mode enum, ModeIndicator
└── engine.go         # DeriveMode logic with clock injection

internal/shadowview/
├── model.go          # ShadowReceiptPage, section types
├── engine.go         # BuildPage, BuildCue logic
└── ack_store.go      # Hash-only acknowledgement storage
```

## Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/onboarding` | GET | Unified entry page |
| `/shadow/receipt` | GET | Shadow receipt viewer |
| `/shadow/receipt/dismiss` | POST | Dismiss receipt cue for period |

## Events

```go
Phase21OnboardingViewed       = "phase21.onboarding.viewed"
Phase21ModeComputed           = "phase21.mode.computed"
Phase21ShadowReceiptViewed    = "phase21.shadow.receipt.viewed"
Phase21ShadowReceiptDismissed = "phase21.shadow.receipt.dismissed"
Phase21ShadowReceiptCueShown  = "phase21.shadow.receipt.cue.shown"
```

## Critical Invariants

1. **Mode is DERIVED, not stored** - purely read-only computation from existing state
2. **UI shows ONLY abstract buckets and hashes** - never raw content
3. **No goroutines** in mode/ or shadowview/ packages
4. **No time.Now()** - clock injection pattern enforced
5. **Ack store stores ONLY hashes** - never raw timestamps
6. **Single whisper rule** - at most one cue per page

## Guardrails

30 checks in `scripts/guardrails/phase21_onboarding_shadow_receipt_enforced.sh`:

- No goroutines in mode/ or shadowview/
- No time.Now() usage
- Clock injection patterns present
- Mode enum with correct values
- ShadowReceiptPage with section structs
- AckStore with bounded size and hash-only storage
- Routes registered
- Events defined
- Demo tests exist and compile

## Test Coverage

Demo tests in `internal/demo_phase21_onboarding_shadow_receipt/`:

- Mode derivation is deterministic
- Shadow receipt page shows only abstract data
- Ack store persists hash-only records
- Receipt cue follows single whisper rule
- Bounded size enforcement

## Consequences

### Positive
- Users understand system state at a glance
- Mode indicator is always accurate (derived, not cached)
- Shadow observations are verifiable without exposing content
- Whisper integration maintains calm aesthetic

### Negative
- Mode derivation requires checking multiple sources each request
- Receipt viewer adds another page to maintain

### Neutral
- Mode indicator style matches existing UI patterns
- Ack store follows same pattern as proof/mirror ack stores

## References

- ADR-0041: Phase 18.8 Real OAuth Gmail
- ADR-0042: Phase 18.9 Quiet Verification
- ADR-0045: Phase 19.2 Shadow Mode
- ADR-0048: Phase 19.4 Shadow Diff Calibration
