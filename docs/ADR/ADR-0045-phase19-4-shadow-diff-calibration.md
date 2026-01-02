# ADR-0045: Phase 19.4 Shadow Diff + Calibration (Truth Harness)

## Status
Accepted

## Context

Phase 19.3 introduced the Azure OpenAI Shadow Provider, allowing real LLM calls in shadow mode. However, we need a way to evaluate whether Shadow's observations are useful - do they align with canon rules, and when they differ, is that insight or noise?

The core question: **Is Shadow intelligence aligned with the canon — and when it isn't, is it useful or noise?**

This requires a deterministic comparison harness that:
- Compares rule-based canon decisions vs Shadow LLM observations
- Does NOT allow Shadow to influence behavior
- Measures alignment, disagreement, and usefulness
- Works with stub provider (no real LLM required for testing)

## Decision

### Architecture

```
Canon (rules) ─┐
               ├─► DiffEngine ─► DiffResult ─► CalibrationStore ─► Stats
Shadow (LLM) ──┘                                    │
                                                    ▼
                                            Human votes (useful/unnecessary)
                                                    │
                                                    ▼
                                            UsefulnessScore
```

### Core Types

```go
// pkg/domain/shadowdiff/types.go

type AgreementKind string
const (
    AgreementMatch    AgreementKind = "match"     // Same conclusion
    AgreementSofter   AgreementKind = "softer"    // Same but lower confidence
    AgreementEarlier  AgreementKind = "earlier"   // Shadow thinks more urgent
    AgreementLater    AgreementKind = "later"     // Shadow thinks less urgent
    AgreementConflict AgreementKind = "conflict"  // Opposite conclusions
)

type Novelty string
const (
    NoveltyNone       Novelty = "none"        // Both saw it
    NoveltyShadowOnly Novelty = "shadow_only" // Only Shadow noticed
    NoveltyCanonOnly  Novelty = "canon_only"  // Only Canon noticed
)

type CalibrationVote string
const (
    VoteUseful      CalibrationVote = "useful"      // Human agrees with Shadow
    VoteUnnecessary CalibrationVote = "unnecessary" // Shadow was noise
)

type DiffResult struct {
    Key         ComparisonKey
    Canon       *CanonSignal
    Shadow      *ShadowSignal
    Agreement   AgreementKind
    Novelty     Novelty
    DiffHash    string
    ComputedAt  time.Time
}
```

### Diff Rules

| Canon Horizon | Shadow Horizon | Canon Magnitude | Shadow Magnitude | Shadow Confidence | Result |
|---------------|----------------|-----------------|------------------|-------------------|--------|
| X | X | Y | Y | high | match |
| X | X | Y | Y | low/med | softer |
| X | < X | * | * | * | earlier |
| X | > X | * | * | * | later |
| * | * | nothing | several | * | conflict |
| * | * | several | nothing | * | conflict |
| * | * | Y | Y±1 | * | conflict |

### Calibration Stats

```go
type CalibrationStats struct {
    Period            string
    TotalDiffs        int
    ByAgreement       map[AgreementKind]int
    ByNovelty         map[Novelty]int
    TotalVotes        int
    UsefulVotes       int
    UnnecessaryVotes  int
    AgreementRate     float64  // matches / total
    NoveltyRate       float64  // novel / total
    ConflictRate      float64  // conflicts / total
    UsefulnessScore   float64  // useful / (useful + unnecessary)
}
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/shadow/report` | GET | View calibration stats with plain-language summary |
| `/shadow/vote` | POST | Record human vote on a diff |

### Plain Language Summaries

The system generates human-readable summaries:

- **Agreement Summary**: "Shadow strongly agrees with the system." / "Shadow mostly agrees with the system." / "Shadow has mixed observations."
- **Novelty Summary**: "Shadow sometimes notices things the system missed." / "Shadow rarely adds new observations." / "Both systems see the same things."
- **Overall Summary**: "Shadow is highly aligned and occasionally useful." / "Shadow partially agrees with the system." / etc.

## Implementation

### Package Structure

```
pkg/domain/shadowdiff/
├── types.go      # Core domain types
└── hashing.go    # Canonical string + SHA256 hashing

internal/shadowdiff/
├── engine.go     # Diff computation engine
└── rules.go      # Agreement/Novelty rules

internal/shadowcalibration/
├── engine.go     # Calibration aggregation
└── stats.go      # Stats computation + summaries

internal/persist/
└── shadow_calibration_store.go  # Append-only storage
```

### Events

```go
Phase19_4DiffComputed     = "phase19_4.diff.computed"
Phase19_4DiffPersisted    = "phase19_4.diff.persisted"
Phase19_4VoteRecorded     = "phase19_4.vote.recorded"
Phase19_4VotePersisted    = "phase19_4.vote.persisted"
Phase19_4StatsComputed    = "phase19_4.stats.computed"
Phase19_4StatsViewed      = "phase19_4.stats.viewed"
Phase19_4ReportRequested  = "phase19_4.report.requested"
Phase19_4ReportRendered   = "phase19_4.report.rendered"
```

### Storelog Record Types

```go
RecordTypeShadowDiff        = "SHADOW_DIFF"
RecordTypeShadowCalibration = "SHADOW_CALIBRATION"
```

## Constraints

| Constraint | Enforcement |
|------------|-------------|
| No execution influence | No shadowdiff imports in execution/drafts/routing |
| No policy mutation | No SetPolicy/UpdatePolicy/DeletePolicy in shadow packages |
| No goroutines | Guardrail scan: `go func` |
| No time.Now() | Clock injection required |
| stdlib only | No external imports (except quantumlife) |
| Hash-only persistence | No raw content fields (Subject, Body, etc.) |
| Pipe-delimited strings | No json.Marshal in hashing |

## Guardrails

`scripts/guardrails/shadow_diff_enforced.sh` validates:

1. Domain types exist (types.go, hashing.go)
2. Engine exists (engine.go, rules.go)
3. Calibration store exists
4. Calibration stats engine exists
5. No execution path influence
6. No policy mutation
7. No goroutines in shadow packages
8. No time.Now() in shadow packages
9. stdlib only
10. Hash-only persistence
11. Phase 19.4 events exist
12. Storelog record types exist
13. Demo tests exist (8+ functions)
14. Web routes exist
15. Agreement types complete (match, conflict, earlier, later, softer)
16. Novelty types complete (none, shadow_only, canon_only)
17. Vote types complete (useful, unnecessary)
18. Pipe-delimited canonical strings

## Consequences

### Positive
- Quantifiable measurement of Shadow usefulness
- Human feedback loop for calibration
- No risk of Shadow affecting production behavior
- Works with stub provider for deterministic testing
- Plain language summaries for non-technical understanding

### Negative
- Adds complexity to shadow mode infrastructure
- Requires human votes for usefulness scoring
- Stats may not be meaningful with small sample sizes

### Neutral
- Does not change any existing behavior
- Does not require real LLM for basic testing
- Stats reset with each period (rolling window)

## References

- ADR-0044: Phase 19.3 Azure OpenAI Shadow Provider
- ADR-0043: Phase 19.2 Shadow Mode Architecture
- ADR-0042: Phase 19.1 Real Gmail Connection
