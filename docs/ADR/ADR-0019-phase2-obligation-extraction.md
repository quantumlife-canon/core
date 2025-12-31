# ADR-0019: Phase 2 Obligation Extraction + Daily View Generation

**Status:** Accepted
**Date:** 2025-01-01
**Version:** Phase 2

## Context

QuantumLife's core promise is making "Nothing Needs You" emotionally true. Phase 1 established read-only adapters for email, calendar, and finance data. Phase 2 transforms this raw data into actionable obligations and a daily "home truth" that tells users whether they need to act.

The challenge is determining when to surface obligations without:
- Creating anxiety through false positives
- Missing important items through false negatives
- Being paternalistic or presumptuous
- Violating read-only guarantees

## Decision

Implement a deterministic obligation extraction engine with rule-based scoring that produces a daily NeedsYou computation.

### Core Design Principles

1. **Ephemeral Obligations**: Obligations are computed on demand, never persisted. Same events + same clock = same obligations. This ensures determinism and prevents stale state.

2. **Rule-First Extraction**: All extraction uses explicit pattern matching and threshold-based rules. No ML/AI inference in the extraction path.

3. **Canonical String Hashing**: All deterministic IDs and hashes use pipe-delimited canonical strings, NOT JSON serialization.

4. **Injected Clock**: All time computations use an injected `clock.Clock` interface. Never call `time.Now()` directly.

5. **Synchronous Processing**: Single-pass extraction with no goroutines or background processing.

### Obligation Model

```go
type Obligation struct {
    ID            string           // Deterministic: sha256(canonical)[:16]
    CircleID      identity.EntityID
    SourceEventID string
    Type          ObligationType   // reply, attend, pay, review, decide, followup
    Horizon       AttentionHorizon // today, 24h, 7d, someday
    Severity      Severity
    DueBy         *time.Time
    RegretScore   float64          // 0.0-1.0: probability of regret if ignored
    Confidence    float64          // 0.0-1.0: confidence in extraction
    Reason        string           // Human-readable explanation
    Evidence      map[string]string
}
```

### Attention Horizons

- **today**: Overdue or due now (until <= 0)
- **24h**: Due within 24 hours
- **7d**: Due within 7 days
- **someday**: No specific deadline or far future

### NeedsYou Computation

NeedsYou = true when:
- At least one obligation exists
- Within attention horizon (today OR 24h by default)
- With regret score >= threshold (0.5 by default)

```go
type NeedsYouConfig struct {
    RegretThreshold   float64                  // Default: 0.5
    AttentionHorizons []AttentionHorizon       // Default: [today, 24h]
    MaxReasons        int                      // Default: 3
}
```

### Extraction Rules

**Email Rules:**
1. Unread + action cue keywords → Review obligation (regret: 0.7)
2. Unread + important/starred → Review obligation (regret: 0.6)
3. Unread + transactional (invoice) → Pay obligation (regret: 0.65)
4. Stale unread (>7 days) from high-priority sender → Followup (regret: 0.35)

**Calendar Rules:**
1. Upcoming event (within 24h) + unresponded → Decide obligation (regret: 0.6-0.8)
2. Upcoming event (within 24h) + accepted/tentative → Attend obligation (regret: 0.5-0.8)
3. Overlapping events → Decide obligation (regret: 0.8, severity: critical)

**Finance Rules:**
1. Balance below threshold → Review obligation (regret: 0.7)
2. Large transaction (within 48h) → Review obligation (regret: 0.45)
3. Pending transaction → Review obligation (regret: 0.25)

### DailyView Structure

```go
type DailyView struct {
    DateKey         string                    // YYYY-MM-DD UTC
    ComputedAt      time.Time
    NeedsYou        bool
    NeedsYouReasons []string                  // Max 3
    Obligations     []*Obligation             // Sorted by priority
    Circles         map[EntityID]*CircleSummary
    Hash            string                    // Deterministic view hash
    ObligationsHash string
}
```

### Due Date Parsing

Deterministic parsing using stdlib regexp only. Supported patterns:
- ISO format: `2025-01-20`
- US format: `01/20/2025`
- Month-day: `January 20`, `Jan 20`
- Day-month: `20 January`, `20th Jan`
- Weekday: `by Friday`, `by next Monday`
- Relative: `EOD`, `EOW`, `EOM`, `within 3 days`
- Action cues: `action required by ...`

If date without year is in the past, assume next year.

## Package Structure

```
pkg/domain/obligation/
├── types.go          # Core types, sorting, hashing
├── dueparse.go       # Deterministic due date parsing
└── *_test.go

internal/obligations/
├── engine.go         # Extraction engine
└── engine_test.go

pkg/domain/view/
└── daily.go          # DailyView and NeedsYou logic

internal/demo_phase2_obligations/
├── runner.go         # Demo with mock scenarios
└── demo_test.go
```

## Consequences

### Positive

- Deterministic: Same inputs always produce same outputs
- Testable: Fixed clock enables reliable testing
- Auditable: Canonical strings make hashing transparent
- Simple: No ML dependencies, stdlib only
- Read-only: Never modifies source data

### Negative

- Rule-based may miss nuanced cases that ML could detect
- Thresholds require tuning for individual users (future work)
- No learning from user feedback (intentional for Phase 2)

## Testing Requirements

1. **Determinism Tests**: Run extraction twice, verify identical hashes
2. **Scenario Tests**:
   - ScenarioNeedsYou → NeedsYou=true with reasons
   - ScenarioNothingNeedsYou → NeedsYou=false
   - ScenarioMixed → Demonstrates threshold filtering
3. **Ordering Tests**: Obligations sorted by horizon, then regret
4. **Hash Stability**: Multiple runs produce stable hashes
5. **Different Clock = Different Hash**: Verify time sensitivity

## Canon Compliance

- NO goroutines (synchronous processing)
- NO time.Now() (injected clock)
- NO auto-retry (single-pass extraction)
- NO background execution
- NO writes to source systems
- stdlib only (no third-party dependencies)

## References

- QUANTUMLIFE_CANON_V1.md
- ARCHITECTURE_LIFE_OS_V1.md
- ADR-0010: No Background Execution Guardrail
- ADR-0011: No Auto-Retry and Single Trace Finalization
