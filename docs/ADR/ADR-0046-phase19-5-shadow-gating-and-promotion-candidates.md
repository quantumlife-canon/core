# ADR-0046: Phase 19.5 - Shadow Gating + Promotion Candidates

## Status

Accepted

## Context

Phase 19.4 established the Shadow Diff + Calibration system, which:
- Computes differences between canon rules and shadow observations
- Allows human voting on diff usefulness (useful vs unnecessary)
- Produces calibration statistics

We now need a way to surface patterns that consistently appear in shadow but not in canon, allowing users to propose promotion of these patterns to rules. This must be done WITHOUT affecting any runtime behavior.

## Decision

### What This Phase Does

Phase 19.5 introduces Shadow Gating + Promotion Candidates:

1. **Candidates**: Patterns that shadow detected but canon didn't surface (or vice versa)
2. **Promotion Intents**: User-recorded intentions to eventually promote candidates to rules
3. **Privacy Guard**: Ensures all candidate descriptions are privacy-safe

### Critical Invariants

| Invariant | Enforcement |
|-----------|-------------|
| Shadow does NOT affect behavior | No imports into execution/drafts/interruptions |
| No canon thresholds changed | Intent only - no policy mutation |
| No obligation rules changed | Candidates are read-only artifacts |
| No interruption logic changed | Engine isolated from loop |
| No drafts generated | No draft package imports |
| No execution boundaries touched | No execution package imports |
| Deterministic | Clock injection, no time.Now() |
| Privacy-safe | Privacy guard with forbidden patterns |
| No background work | No goroutines |
| Stdlib only | No external dependencies |

### Domain Model

```go
// pkg/domain/shadowgate/types.go

type CandidateOrigin string // shadow_only | canon_only | conflict

type UsefulnessBucket string // unknown | low | medium | high
// Computed from vote percentage: <25% = low, 25-75% = medium, >75% = high

type VoteConfidenceBucket string // unknown | low | medium | high
// Computed from vote count: 0 = unknown, 1-2 = low, 3-5 = medium, 6+ = high

type NoteCode string // promote_rule | needs_more_votes | ignore_for_now

type Candidate struct {
    ID                   string
    Hash                 string
    PeriodKey            string // YYYY-MM-DD
    CircleID             identity.EntityID
    Origin               CandidateOrigin
    Category             shadowllm.AbstractCategory
    HorizonBucket        shadowllm.Horizon
    MagnitudeBucket      shadowllm.MagnitudeBucket
    WhyGeneric           string // Privacy-safe reason
    UsefulnessPct        int    // 0-100
    UsefulnessBucket     UsefulnessBucket
    VoteConfidenceBucket VoteConfidenceBucket
    VotesUseful          int
    VotesUnnecessary     int
    FirstSeenBucket      string
    LastSeenBucket       string
    CreatedAt            time.Time
}

type PromotionIntent struct {
    IntentID      string
    IntentHash    string
    CandidateID   string
    CandidateHash string
    PeriodKey     string
    NoteCode      NoteCode
    CreatedBucket string
    CreatedAt     time.Time
}
```

### Canonical Hashing

All hashing uses pipe-delimited format (not JSON):

```
SHADOW_CANDIDATE|v1|period|circle|origin|category|horizon|magnitude|why|...
PROMOTION_INTENT|v1|candidate_id|candidate_hash|period|note|...
```

### Privacy Guard

The privacy guard ensures WhyGeneric contains no identifiable information:

**Forbidden Patterns:**
- Email addresses (`user@domain.com`)
- URLs and domains (`http://`, `www.`, `.com`)
- Currency amounts (`$100`, `EUR 50`)
- Phone numbers
- Credit card patterns
- Vendor names (Amazon, Google, Netflix, etc.)

**Allowed Reason Phrases:**
```go
var AllowedReasonPhrases = []string{
    "A pattern we've seen before.",
    "Something that recurs in this category.",
    "A timing pattern you might want to address.",
    "Items that tend to need attention together.",
    "A spending pattern worth reviewing.",
    "Calendar items that often align.",
    "Messages that cluster by topic.",
    "Work items with similar urgency.",
    "Home tasks that appear together.",
    "People-related items requiring attention.",
}
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/shadow/candidates` | GET | View candidates (whisper-style) |
| `/shadow/candidates/refresh` | POST | Recompute candidates from diffs |
| `/shadow/candidates/propose` | POST | Record promotion intent |

### Events

```go
Phase19_5CandidatesRefreshRequested EventType = "phase19_5.candidates.refresh_requested"
Phase19_5CandidatesComputed         EventType = "phase19_5.candidates.computed"
Phase19_5CandidatesPersisted        EventType = "phase19_5.candidates.persisted"
Phase19_5CandidatesViewed           EventType = "phase19_5.candidates.viewed"
Phase19_5PromotionProposed          EventType = "phase19_5.promotion.proposed"
Phase19_5PromotionPersisted         EventType = "phase19_5.promotion.persisted"
```

### Sorting

Candidates are sorted for deterministic display:
1. UsefulnessBucket desc (high > medium > low > unknown)
2. Origin priority (shadow_only > canon_only > conflict)
3. Hash asc (deterministic tiebreaker)

### Architecture

```
Phase 19.4 Diffs + Votes
        |
        v
+------------------+
| DiffSource       | <- Adapter to ShadowCalibrationStore
+------------------+
        |
        v
+------------------+
| Candidate Engine | <- Groups by signature, computes usefulness
| (shadowgate)     |
+------------------+
        |
        v
+------------------+
| Privacy Guard    | <- Validates WhyGeneric
+------------------+
        |
        v
+------------------+
| ShadowGateStore  | <- Append-only persistence
+------------------+
        |
        v
+------------------+
| Web UI           | <- Whisper-style candidates page
+------------------+
```

## Consequences

### Positive

1. Users can see patterns that shadow suggests but canon missed
2. Human voting determines usefulness of each pattern
3. Promotion intents provide clear signal for future rule development
4. Privacy guard ensures no PII leaks into candidate descriptions
5. All data is deterministic and replayable

### Negative

1. Candidates do NOT actually affect behavior (by design)
2. Manual process required to turn intents into rules
3. Additional storage for candidates and intents

### Neutral

1. Requires Phase 19.4 (Shadow Diff) to be complete
2. Web UI follows existing whisper-style patterns
3. Guardrails ensure isolation from runtime behavior

## Files Changed

| File | Change |
|------|--------|
| `pkg/domain/shadowgate/types.go` | New domain types |
| `pkg/domain/shadowgate/types_test.go` | Domain tests |
| `internal/shadowgate/engine.go` | Candidate engine |
| `internal/shadowgate/privacy.go` | Privacy guard |
| `internal/persist/shadow_gate_store.go` | Persistence |
| `pkg/events/events.go` | Phase 19.5 events |
| `cmd/quantumlife-web/main.go` | Web routes and handlers |
| `internal/demo_phase19_5_shadow_gating/demo_test.go` | Demo tests |
| `scripts/guardrails/shadow_gating_enforced.sh` | Guardrails |
| `Makefile` | New targets |

## References

- ADR-0045: Phase 19.4 - Shadow Diff + Calibration
- ADR-0044: Phase 19.3 - Azure OpenAI Shadow Provider
- ADR-0043: Phase 19.2 - Shadow Mode Contract
