# ADR-0020: Phase 3 Interruption Engine + Weekly Digest

**Status:** Accepted
**Date:** 2025-01-01
**Version:** Phase 3

## Context

Phase 2 established obligation extraction from email, calendar, and finance data. Obligations represent things that might need attention. Phase 3 transforms these obligations into a prioritized interruption stream with quota limits to prevent notification fatigue, plus weekly digest generation for reflection.

The core challenge is balancing urgency against overwhelm:
- Surface truly urgent items immediately
- Batch less-urgent items for digest review
- Prevent "spammy repeats" through deduplication
- Enforce per-circle quotas to avoid notification fatigue
- Generate weekly patterns for user reflection

## Decision

Implement a deterministic interruption engine with level-based prioritization, dedup, quota enforcement, and weekly digest generation.

### Core Design Principles

1. **Level-Based Priority**: Five distinct interruption levels from Silent to Urgent, with different delivery semantics.

2. **Regret-to-Level Mapping**: Regret scores (0-100) combined with due date proximity determine interruption level.

3. **Dedup with Time Buckets**: Hour buckets for Urgent/Notify, day buckets for lower levels. Same obligation in same bucket = deduplicated.

4. **Quota Enforcement**: Per-circle limits on Notify+Urgent per day. Urgent is NEVER downgraded. Notify is downgraded to Queued when quota exceeded.

5. **Weekly Digest**: Aggregates interruptions over 7 days with pattern observations.

### Interruption Levels

```go
type Level string

const (
    LevelSilent  Level = "silent"   // Never interrupt, view on demand
    LevelAmbient Level = "ambient"  // Background awareness, daily digest
    LevelQueued  Level = "queued"   // Batched delivery, next digest slot
    LevelNotify  Level = "notify"   // Active notification, quota-limited
    LevelUrgent  Level = "urgent"   // Immediate attention, NEVER downgraded
)
```

### Level Ordering

Higher level = more urgent. Used for sorting and filtering.

```
Urgent (4) > Notify (3) > Queued (2) > Ambient (1) > Silent (0)
```

### Triggers

Trigger types classify the source of the interruption:

```go
type Trigger string

const (
    TriggerEmailActionNeeded     = "email_action_needed"
    TriggerCalendarUpcoming      = "calendar_upcoming"
    TriggerCalendarConflict      = "calendar_conflict"
    TriggerCalendarInvitePending = "calendar_invite_pending"
    TriggerFinanceLowBalance     = "finance_low_balance"
    TriggerFinanceLargeTxn       = "finance_large_txn"
    TriggerFinancePending        = "finance_pending"
    TriggerObligationDueSoon     = "obligation_due_soon"
    TriggerUnknown               = "unknown"
)
```

### Interruption Model

```go
type Interruption struct {
    InterruptionID string           // Deterministic: sha256(canonical)[:16]
    CircleID       identity.EntityID
    Level          Level
    Trigger        Trigger
    SourceEventID  string
    ObligationID   string
    RegretScore    int              // 0-100 (clamped)
    Confidence     int              // 0-100 (clamped)
    ExpiresAt      time.Time
    CreatedAt      time.Time
    Summary        string
    DedupKey       string           // For dedup: circle|trigger|source|bucket
}
```

### Regret Score Computation

```
score = CircleBase[circleType]   // finance=30, family=25, work=15, health=20, home=10
      + DueBoost                  // 24h=+30, 7d=+15, else=0
      + ActionNeededBoost         // +15 if reply/pay/decide or high/critical severity
      + ObligationRegret * 30     // Scale 0.0-1.0 to 0-30
      + SeverityBoost             // critical=+20, high=+10

score = min(score, 100)          // Cap at 100
```

### Level Determination

```
Urgent:  regret >= 90 AND due within 24h
Notify:  regret >= 75 AND due within 48h
Queued:  regret >= 50
Ambient: regret >= 25
Silent:  else
```

### Dedup Strategy

Dedup keys include time bucket to allow re-surfacing over time:

- **Urgent/Notify**: Hour bucket (2025-01-15T10)
- **Queued/Ambient/Silent**: Day bucket (2025-01-15)

Key format: `{circleID}|{trigger}|{sourceEventID}|{timeBucket}`

Same key in dedup store = drop duplicate.

### Quota Enforcement

Per-circle daily limits on Notify+Urgent interruptions:

```go
DefaultQuotaConfig = {
    MaxNotifyUrgentPerDay: {
        "finance": 3,
        "family":  3,
        "work":    2,
        "health":  1,
        "home":    1,
    }
}
```

Enforcement rules:
- Urgent is **NEVER** downgraded (safety-critical)
- Notify is downgraded to Queued when quota exceeded
- Quota resets at UTC midnight

### Engine Processing Pipeline

```
1. Transform obligations → interruptions (compute regret, level, trigger)
2. Apply dedup (filter by DedupKey)
3. Apply quota (downgrade Notify → Queued if over limit)
4. Sort by level (desc) then regret (desc)
5. Build circle summaries
6. Compute result hash
```

### Weekly Digest

Aggregates 7 days of interruptions with:

- Per-circle summaries (counts by level)
- Top items (3 highest regret per circle)
- Pattern observations:
  - "High urgent count this week"
  - "X circle dominated this week"
  - "No urgent items - well managed!"
  - "Light week with few interruptions"

```go
type WeeklyDigest struct {
    WeekStart       time.Time
    WeekEnd         time.Time
    GeneratedAt     time.Time
    TotalInterruptions int
    ByLevel         map[Level]int
    CircleSummaries map[EntityID]*CircleSummary
    Observations    []string
    Hash            string
}
```

## Package Structure

```
pkg/domain/interrupt/
├── types.go          # Level, Trigger, Interruption, sorting
├── hash.go           # Deterministic hashing utilities
└── *_test.go

internal/interruptions/
├── engine.go         # Main engine (transform + pipeline)
├── dedup.go          # DedupStore interface + InMemoryDeduper
├── quota.go          # QuotaEnforcer + per-circle limits
└── *_test.go

internal/digest/
├── weekly.go         # Weekly digest generator
└── weekly_test.go

internal/demo_phase3_interruptions/
└── demo.go           # Demo runner
```

## Consequences

### Positive

- Deterministic: Same inputs + clock = same interruptions
- Quota prevents notification fatigue
- Urgent never downgraded (safety-critical items always surface)
- Dedup prevents spammy repeats
- Weekly digest enables reflection without daily overwhelm
- Read-only: Never modifies source data

### Negative

- Fixed quotas may need per-user tuning (future work)
- Simple bucket-based dedup may miss some semantic duplicates
- No learning from user dismissals (intentional for Phase 3)

## Testing Requirements

1. **Determinism Tests**: Run engine twice, verify identical hashes
2. **Regret Scoring Tests**: Verify formula produces expected scores
3. **Level Tests**: Verify regret+due thresholds map to correct levels
4. **Dedup Tests**: Same key in store = dropped, different bucket = kept
5. **Quota Tests**: Notify downgraded when over limit, Urgent never downgraded
6. **Sorting Tests**: Urgent first, then by regret descending
7. **Digest Tests**: Circle summaries, observations, deterministic hash

## Canon Compliance

- NO goroutines (synchronous processing)
- NO time.Now() (injected clock)
- NO auto-retry (single-pass processing)
- NO background execution
- NO writes to source systems
- stdlib only (no third-party dependencies)

## References

- QUANTUMLIFE_CANON_V1.md
- ARCHITECTURE_LIFE_OS_V1.md
- ADR-0019: Phase 2 Obligation Extraction
- ADR-0010: No Background Execution Guardrail
- ADR-0011: No Auto-Retry and Single Trace Finalization
