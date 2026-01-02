# ADR-0048: Phase 20 - Trust Accrual Layer (Proof Over Time)

## Status

Accepted

## Date

2024-01-15

## Context

Users cannot observe that QuantumLife is working unless something is actively
surfaced. This creates a paradox: the best outcome (silence) appears identical
to system failure or inactivity.

Traditional solutions involve:
- Activity dashboards (creates engagement pressure)
- Notification counts (creates anxiety)
- "Saved you X hours" metrics (performative, manipulative)
- Streak mechanics (addictive, punitive)

All of these solutions **worsen the user experience** by trading silence for
engagement metrics.

We need a way to make silence observable **without creating engagement pressure**.

## Decision

Implement a **Trust Accrual Layer** that:

1. **Accrues evidence of restraint over time**
   - Counts held obligations, suppressions, and shadow rejections
   - Aggregates into week/month periods
   - Uses only abstract magnitude buckets (never raw counts)

2. **Makes silence legible without noise**
   - Available on-demand via whisper-style `/trust` page
   - Never pushed, notified, or highlighted
   - Dismissable without consequence

3. **Builds user trust longitudinally**
   - Evidence accumulates over weeks/months
   - User can observe pattern: "It was quiet, but it was watching"
   - No gamification, no streaks, no achievements

## Absolute Constraints

| Constraint | Enforcement |
|------------|-------------|
| Silence is the default outcome | Engine returns `nil` when nothing happened |
| Trust signals are NEVER pushed | No notification integration |
| Trust signals are NEVER frequent | Week/month granularity only |
| Trust signals are NEVER actionable | No buttons, only plain text links |
| Only abstract buckets | `nothing` / `a_few` / `several` only |
| NO timestamps, counts, vendors, people, or content | Types exclude these fields |
| Append-only, hash-only storage | No update/delete methods |
| Deterministic | Same inputs + clock → same hash |
| stdlib only | No external dependencies |
| No goroutines | Synchronous only |
| No time.Now() | Clock injection required |

## Architecture

### Domain Model (`pkg/domain/trust/types.go`)

```go
// TrustPeriod represents the time granularity
type TrustPeriod string
const (
    PeriodWeek  TrustPeriod = "week"
    PeriodMonth TrustPeriod = "month"
)

// TrustSignalKind represents what kind of restraint was demonstrated
type TrustSignalKind string
const (
    SignalQuietHeld             TrustSignalKind = "quiet_held"
    SignalInterruptionPrevented TrustSignalKind = "interruption_prevented"
    SignalNothingRequired       TrustSignalKind = "nothing_required"
)

// TrustSummary represents aggregated evidence of restraint
type TrustSummary struct {
    SummaryID       string
    SummaryHash     string
    Period          TrustPeriod
    PeriodKey       string                    // e.g., "2024-W03"
    SignalKind      TrustSignalKind
    MagnitudeBucket shadowllm.MagnitudeBucket // nothing | a_few | several
    DismissedBucket string
    CreatedBucket   string
    CreatedAt       time.Time
}

// TrustDismissal records that a user dismissed a summary
type TrustDismissal struct {
    DismissalID   string
    DismissalHash string
    SummaryID     string
    SummaryHash   string
    CreatedBucket string
    CreatedAt     time.Time
}
```

### Engine (`internal/trust/engine.go`)

```go
// RestraintSource provides access to evidence of restraint
type RestraintSource interface {
    GetHeldCount(periodKey string, period TrustPeriod) int
    GetSuppressionCount(periodKey string, period TrustPeriod) int
    GetShadowRejectionCount(periodKey string, period TrustPeriod) int
}

// Engine computes TrustSummaries from restraint evidence
type Engine struct {
    clk clock.Clock
}

// Compute calculates a TrustSummary for the given period
// Returns nil Summary if nothing meaningful occurred
func (e *Engine) Compute(input ComputeInput) (*ComputeOutput, error)
```

### Storage (`internal/persist/trust_store.go`)

```go
type TrustStore struct {
    summaries         map[string]*TrustSummary
    summariesByPeriod map[string]string
    dismissals        map[string]*TrustDismissal
}

// Operations:
// - AppendSummary (deduplicates by period)
// - DismissSummary (permanent)
// - ListUndismissedSummaries
// - GetRecentMeaningfulSummary
// - ReplaySummaryRecord / ReplayDismissalRecord
```

### Web Surface (`/trust`)

Whisper-style page showing 1-3 recent undismissed summaries:
- Plain text descriptions (e.g., "Things were held quietly.")
- Abstract chips (period key, magnitude bucket)
- Plain link to dismiss (not a button)

## Magnitude Bucketing

Raw counts are **never exposed**. The engine converts internally:

| Count | Bucket |
|-------|--------|
| 0 | nothing |
| 1-3 | a_few |
| 4+ | several |

## Signal Priority

When multiple types of evidence exist, priority order:
1. Suppressions (most active form of restraint)
2. Held obligations
3. Shadow rejections

## Human-Readable Language

```go
func (s TrustSignalKind) HumanReadable() string {
    switch s {
    case SignalQuietHeld:
        return "Things were held quietly."
    case SignalInterruptionPrevented:
        return "Interruptions were prevented."
    case SignalNothingRequired:
        return "Nothing required attention."
    }
}
```

**Forbidden language:**
- "Saved you X" (performative)
- "Protected you from" (fear-based)
- "Thanks to us" (self-congratulatory)
- Any urgency, superlatives, or calls to action

## Period Key Format

```go
// Week: "2024-W03" (ISO week number)
func WeekKey(t time.Time) string

// Month: "2024-01"
func MonthKey(t time.Time) string
```

Period keys are **abstract identifiers**, not timestamps.

## Canonical String Format

Pipe-delimited for deterministic hashing:

```
TRUST_SUMMARY|v1|week|2024-W03|quiet_held|a_few|2024-01-15T10:30
```

**NOT JSON** - this ensures byte-for-byte reproducibility.

## Events

```go
Phase20TrustComputed  = "phase20.trust.computed"
Phase20TrustPersisted = "phase20.trust.persisted"
Phase20TrustViewed    = "phase20.trust.viewed"
Phase20TrustDismissed = "phase20.trust.dismissed"
```

## What This Phase Does NOT Do

- Does NOT push notifications
- Does NOT count anything in the UI
- Does NOT create engagement pressure
- Does NOT affect any existing behavior
- Does NOT gamify or reward usage
- Does NOT surface unless explicitly visited

## Alternatives Considered

### Activity Dashboard
Rejected: Creates engagement pressure and anxiety.

### "We saved you X hours" Metrics
Rejected: Performative, manipulative, and often inaccurate.

### Weekly Email Summaries
Rejected: Pushes information, violates silence principle.

### Achievement/Badge System
Rejected: Gamification creates addictive engagement patterns.

## Consequences

### Positive
- Users can observe restraint without being interrupted
- Trust builds longitudinally through accumulated evidence
- Silence remains the primary experience
- No engagement pressure or manipulation

### Negative
- Users who never visit `/trust` may not see the value
- Requires patience - value emerges over weeks/months
- Less "impressive" than flashy dashboards

### Neutral
- Adds storage overhead for trust summaries
- Requires integration with existing stores for source data

## Guardrails (50 checks)

See `scripts/guardrails/trust_accrual_enforced.sh`:
- Domain model structure
- No time.Now(), no goroutines
- Clock injection
- Canonical string format
- Append-only storage
- Magnitude buckets only
- Event definitions
- No performative language

## Files Changed

| File | Change |
|------|--------|
| `pkg/domain/trust/types.go` | New domain types |
| `internal/trust/engine.go` | Trust accrual engine |
| `internal/persist/trust_store.go` | Append-only storage |
| `pkg/events/events.go` | Phase 20 events |
| `cmd/quantumlife-web/main.go` | /trust route and handlers |
| `scripts/guardrails/trust_accrual_enforced.sh` | Guardrails |
| `internal/demo_phase20_trust_accrual/demo_test.go` | Demo tests |

## References

- Phase 19 Shadow Mode contracts
- Existing magnitude bucket implementation in shadowllm
- Clock injection pattern used throughout codebase
