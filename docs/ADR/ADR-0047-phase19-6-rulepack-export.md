# ADR-0047: Phase 19.6 - Rule Pack Export (Promotion Pipeline)

## Status

Accepted

## Context

Phase 19.5 established Shadow Gating + Promotion Candidates, which:
- Computes candidates from shadow diffs (patterns shadow detected but canon didn't)
- Allows users to record promotion intents (intent to promote to rules)
- Maintains privacy-safe descriptions with forbidden pattern validation

We now need a way to bundle these promotion intents into exportable, deterministic "Rule Pack" artifacts that can be reviewed, audited, and (in future phases) applied to rules. This phase creates the export pipeline WITHOUT applying any changes.

## Decision

### What This Phase Does

Phase 19.6 introduces Rule Pack Export (Promotion Pipeline):

1. **RulePack**: Exportable collection of proposed rule changes derived from PromotionIntents
2. **RuleChange**: Individual change proposal with change kind, target scope, and evidence
3. **Gating Thresholds**: Only qualified candidates (usefulness >= medium, votes >= 3, confidence >= medium) are included
4. **Export Format**: Stable, pipe-delimited text format for human/machine review

### Critical Invariants

| Invariant | Enforcement |
|-----------|-------------|
| RulePack does NOT apply itself | No imports into policy/obligations/interruptions |
| No policy mutation | Intent recording only |
| No behavior change | Isolated from execution paths |
| No raw identifiers | Only hashes and buckets in exports |
| Deterministic | Same inputs + clock => same hashes |
| Pipe-delimited format | No JSON for canonical strings |
| Clock injection | No time.Now() allowed |
| No goroutines | Synchronous only |
| Stdlib only | No external dependencies |

### Domain Model

```go
// pkg/domain/rulepack/types.go

// Gating thresholds
const (
    MinUsefulnessBucket     = shadowgate.UsefulnessMedium
    MinVoteCount            = 3
    MinVoteConfidenceBucket = shadowgate.VoteConfidenceMedium
)

type ChangeKind string // bias_adjust | threshold_adjust | suppress_suggest

type TargetScope string // circle | trigger | category | itemkey | unknown

type SuggestedDelta string // delta_none | delta_small | delta_medium | delta_large

type NoveltyBucket string // none | shadow_only | canon_only

type AgreementBucket string // match | softer | earlier | later | conflict

type AckKind string // viewed | exported | dismissed

type RuleChange struct {
    ChangeID             string
    CandidateHash        string
    IntentHash           string
    CircleID             identity.EntityID
    ChangeKind           ChangeKind
    TargetScope          TargetScope
    TargetHash           string // Privacy-safe hash
    Category             shadowllm.AbstractCategory
    SuggestedDelta       SuggestedDelta
    UsefulnessBucket     shadowgate.UsefulnessBucket
    VoteConfidenceBucket shadowgate.VoteConfidenceBucket
    NoveltyBucket        NoveltyBucket
    AgreementBucket      AgreementBucket
}

type RulePack struct {
    PackID              string
    PackHash            string
    PeriodKey           string
    CircleID            identity.EntityID
    CreatedAtBucket     string
    ExportFormatVersion string
    Changes             []RuleChange
    CreatedAt           time.Time
}

type PackAck struct {
    AckID         string
    AckHash       string
    PackID        string
    PackHash      string
    AckKind       AckKind
    CreatedBucket string
    CreatedAt     time.Time
}
```

### Canonical Hashing

All hashing uses pipe-delimited format (not JSON):

```
RULE_CHANGE|v1|change_id|candidate_hash|intent_hash|circle|kind|scope|...
RULE_PACK|v1|pack_id|period|circle|created_bucket|change_count|changes_hash
PACK_ACK|v1|ack_id|pack_id|pack_hash|ack_kind|created_bucket
```

### Export Format

```
# RULEPACK EXPORT FORMAT v1
# PACK_ID|period|circle|created_bucket|change_count|pack_hash
PACK|<pack_id>|<period>|<circle>|<created_bucket>|<change_count>|<pack_hash>
# CHANGES (sorted deterministically)
# change_id|candidate_hash|intent_hash|circle|kind|scope|target_hash|category|delta|usefulness|confidence|novelty|agreement
CHANGE|<change_id>|<candidate_hash>|<intent_hash>|<circle>|<kind>|<scope>|<target_hash>|<category>|<delta>|<usefulness>|<confidence>|<novelty>|<agreement>
...
# END
```

### Gating Logic

The engine includes only candidates that meet ALL thresholds:

1. **Usefulness >= Medium**: At least 25% of votes were "useful"
2. **Vote Count >= 3**: At least 3 total votes
3. **Confidence >= Medium**: Vote count is at least 3

```go
func meetsGatingCriteria(c *shadowgate.Candidate) bool {
    if c.UsefulnessBucket < MinUsefulnessBucket {
        return false
    }
    totalVotes := c.VotesUseful + c.VotesUnnecessary
    if totalVotes < MinVoteCount {
        return false
    }
    if c.VoteConfidenceBucket < MinVoteConfidenceBucket {
        return false
    }
    return true
}
```

### ChangeKind Inference

Change kind is derived from candidate origin:

| Origin | ChangeKind | Meaning |
|--------|------------|---------|
| shadow_only | bias_adjust | Shadow saw something canon didn't |
| canon_only | threshold_adjust | Canon surfaced something shadow didn't |
| conflict | suppress_suggest | Canon and shadow disagreed |

### Privacy Validation

Exports are validated against forbidden patterns:

```go
var ForbiddenPatterns = []string{
    "@",        // Email addresses
    "http://",  // URLs
    "https://",
    "$", "£", "€", "¥", "₹", // Currency
    "amazon", "google", "apple", // Vendor names
    // ... more patterns
}

func ValidateExportPrivacy(text string) error {
    for _, pattern := range ForbiddenPatterns {
        if strings.Contains(lower, pattern) {
            return ErrPrivacyViolation
        }
    }
    return nil
}
```

### Persistence

```go
// internal/persist/rulepack_store.go

const (
    RecordTypeRulePackExported = "RULEPACK_EXPORTED"
    RecordTypeRulePackAck      = "RULEPACK_ACK"
)

type RulePackStore struct {
    // Append-only storage
    // Replay support for storelog
}

func (s *RulePackStore) AppendPack(pack *RulePack) error
func (s *RulePackStore) GetPack(packID string) (*RulePack, bool)
func (s *RulePackStore) ListPacks(periodKey string) []RulePack
func (s *RulePackStore) AckPack(packID string, kind AckKind) error
func (s *RulePackStore) ReplayPackRecord(record *storelog.LogRecord) error
```

### Events

```go
// pkg/events/events.go

Phase19_6PackBuildRequested EventType = "phase19_6.pack.build_requested"
Phase19_6PackBuilt          EventType = "phase19_6.pack.built"
Phase19_6PackPersisted      EventType = "phase19_6.pack.persisted"
Phase19_6PackViewed         EventType = "phase19_6.pack.viewed"
Phase19_6PackExported       EventType = "phase19_6.pack.exported"
Phase19_6PackDismissed      EventType = "phase19_6.pack.dismissed"
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/shadow/packs` | GET | List all packs |
| `/shadow/packs/:id` | GET | View pack details |
| `/shadow/packs/:id/export` | POST | Download text export |
| `/shadow/packs/:id/dismiss` | POST | Record dismissal |
| `/shadow/packs/build` | POST | Build new pack from intents |

### Whisper-Style UI

The UI follows whisper-style design:
- No dashboards or metrics
- Magnitude buckets instead of raw counts
- Calm, quiet copy
- No urgency or fear language

## Consequences

### Benefits

1. **Auditable Pipeline**: All rule changes flow through exportable, versioned packs
2. **Human Review**: Packs can be reviewed before any rules change
3. **Privacy Safe**: No raw identifiers ever leave the system
4. **Deterministic**: Same inputs always produce same outputs
5. **Future Ready**: Packs can be applied in future phases

### Limitations

1. **No Auto-Application**: Packs do NOT apply themselves (by design)
2. **Manual Export**: Users must explicitly request exports
3. **Gating Strictness**: Only well-voted candidates are included

## Files Created/Modified

| File | Purpose |
|------|---------|
| `pkg/domain/rulepack/types.go` | Domain types |
| `pkg/domain/rulepack/export.go` | Export format and privacy |
| `pkg/domain/rulepack/types_test.go` | Domain tests |
| `internal/rulepack/engine.go` | Build engine |
| `internal/persist/rulepack_store.go` | Persistence |
| `pkg/events/events.go` | Phase 19.6 events |
| `cmd/quantumlife-web/main.go` | Web routes |
| `internal/demo_phase19_6_rulepack_export/demo_test.go` | 19 demo tests |
| `scripts/guardrails/rulepack_export_enforced.sh` | 39 guardrail checks |

## Verification

```bash
go build ./...                              # Compiles
go test ./...                               # All tests pass
make demo-phase19-6                         # Demo tests pass
make check-rulepack-export                  # Guardrails pass
```

## References

- Phase 19.5: Shadow Gating + Promotion Candidates (ADR-0046)
- Phase 19.4: Shadow Diff + Calibration (ADR-0045)
- Phase 19.2: Shadow Mode Contract (ADR-0043)
