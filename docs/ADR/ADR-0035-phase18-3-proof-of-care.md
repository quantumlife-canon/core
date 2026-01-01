# ADR-0035: Phase 18.3 - The Proof of Care (Held, not shown)

## Status
ACCEPTED

## Context
Users who ask "What did QuantumLife do?" deserve an answer that proves stewardship without compromising restraint. The problem: surfacing details violates our philosophy of silence. The solution: show PROOF OF CARE without showing CONTENTS.

### Core Concept
> "Held, not shown."

The /held page demonstrates that QuantumLife is actively working on behalf of the user - holding things quietly, respecting policies, waiting for the right moment. It provides reassurance without creating mental load.

### What This Page Must NOT Do
- Show lists of any kind
- Show counts tied to specific items
- Show timestamps, names, vendors, dates, or any identifier
- Create work, urgency, or anxiety
- Offer actions (no review, manage, or expand buttons)
- Grow unbounded (store must be bounded)

### What This Page Must Do
- Prove QuantumLife is actively holding things
- Show ONE calm statement
- Show 1-3 abstracted categories (time, money, people, work, home)
- Show reassurance text
- Be more restrained than /today

## Decision

### Route and Flow
New route: `GET /held`

Navigation flow:
```
/ → /today → /held
```

Each step should feel CALMER than the previous.

### Abstract Projection Model
All data is projected through an abstract lens that discards identifiers:

```go
// Categories - abstract, never specific
type Category string
const (
    CategoryTime   Category = "time"   // Calendar, scheduling
    CategoryMoney  Category = "money"  // Financial
    CategoryPeople Category = "people" // Contacts, relationships
    CategoryWork   Category = "work"   // Professional
    CategoryHome   Category = "home"   // Household
)

// ReasonHeld - why something is being held
type ReasonHeld string
const (
    ReasonNotUrgent         ReasonHeld = "not_urgent"
    ReasonAwaitingContext   ReasonHeld = "awaiting_context"
    ReasonProtectedByPolicy ReasonHeld = "protected_by_policy"
    ReasonNoRegretRisk      ReasonHeld = "no_regret_risk"
    ReasonQuietHours        ReasonHeld = "quiet_hours"
)

// HeldSummary - the only output, contains NO identifiers
type HeldSummary struct {
    Statement   string            // Calm statement
    Categories  []CategorySummary // Present categories (presence only)
    Magnitude   string            // "nothing", "a_few", "several"
    Hash        string            // SHA256 for audit
    GeneratedAt time.Time         // For replay
}
```

### Magnitude Bucketing
Exact counts are NEVER exposed. All counts are bucketed:

| Count | Magnitude |
|-------|-----------|
| 0 | "nothing" |
| 1-3 | "a_few" |
| 4+ | "several" |

### Statements by Magnitude

```go
var statements = map[string]string{
    "nothing": "Everything that could need you has been considered. Nothing does.",
    "a_few":   "There are a few things we're holding quietly for you. None of them need you today.",
    "several": "We're holding several things quietly. None of them are urgent.",
}
```

### Deterministic Engine
Same inputs + same clock = same hash, always:

```go
type Engine struct {
    clock func() time.Time
}

func NewEngine(clock func() time.Time) *Engine {
    return &Engine{clock: clock}
}

func (e *Engine) Generate(input HeldInput) HeldSummary {
    // Deterministic projection
    magnitude := computeMagnitude(input.TotalCount())
    statement := statements[magnitude]
    categories := projectCategories(input)
    hash := computeHash(input, e.clock())

    return HeldSummary{
        Statement:   statement,
        Categories:  categories,
        Magnitude:   magnitude,
        Hash:        hash,
        GeneratedAt: e.clock(),
    }
}
```

### Append-Only Store (Hash Only)
The store records ONLY hashes, never raw data:

```go
type SummaryRecord struct {
    Hash       string
    CircleID   string // Optional context
    RecordedAt time.Time
}

type SummaryStore struct {
    records    []SummaryRecord
    maxRecords int // Bounded growth
}

func (s *SummaryStore) Record(summary HeldSummary) error {
    record := SummaryRecord{
        Hash:       summary.Hash,
        RecordedAt: s.clock(),
    }
    s.records = append(s.records, record)

    // Enforce bounded growth
    if len(s.records) > s.maxRecords {
        s.records = s.records[len(s.records)-s.maxRecords:]
    }
    return nil
}
```

### Phase 18.3 Events
Events contain ONLY hashes:

| Event | Payload |
|-------|---------|
| `phase18_3.held.computed` | `{hash, magnitude, category_count, generated_at}` |
| `phase18_3.held.presented` | `{hash, presented_at}` |

### Template Design
Minimal, centered, more restrained than /today:

```html
<div class="held">
  <header class="held-header">
    <h1 class="held-title">Held, quietly.</h1>
  </header>

  <section class="held-statement">
    <p class="held-statement-text">{{.HeldSummary.Statement}}</p>
  </section>

  {{if .HeldSummary.Categories}}
  <section class="held-categories">
    <ul class="held-categories-list">
      {{range .HeldSummary.Categories}}
      <li class="held-category">{{.Category}}</li>
      {{end}}
    </ul>
  </section>
  {{end}}

  <section class="held-reassurance">
    <p class="held-reassurance-text">We're watching, so you don't have to.</p>
  </section>
</div>
```

### Guardrails

The guardrail script (`scripts/guardrails/held_projection_enforced.sh`) validates:

1. `/held` route exists
2. `internal/held` package exists
3. Engine type exists with clock injection
4. Model uses SHA256 for hashing
5. No raw event imports in internal/held
6. No goroutines in internal/held
7. No `time.Now()` (clock injection only)
8. No forbidden imports (stdlib only)
9. Phase 18.3 events exist
10. Held template exists with no identifiers
11. No action buttons in held template
12. Store only records hash, not raw data
13. Demo tests exist and pass

## Consequences

### Positive
- Proves stewardship without disclosure
- Maintains philosophy of silence
- Bounded storage (no unbounded growth)
- Deterministic and reproducible
- Auditable via hash chain
- Even more restrained than /today

### Negative
- Users cannot drill down to specifics
- No actionable information on this page
- Requires discipline to maintain abstraction

### Constraints
- NEVER expose raw counts (bucketing only)
- NEVER store raw data (hashes only)
- NEVER reference specific events (categories only)
- NEVER allow time.Now() (clock injection)
- ALWAYS use stdlib only (no external deps)
- ALWAYS maintain determinism

## Files Changed
```
internal/held/model.go                           (NEW)
internal/held/engine.go                          (NEW)
internal/held/store.go                           (NEW)
pkg/events/events.go                             (MODIFIED)
cmd/quantumlife-web/main.go                      (MODIFIED)
cmd/quantumlife-web/static/app.css               (MODIFIED)
internal/demo_phase18_3_held/demo_test.go        (NEW)
scripts/guardrails/held_projection_enforced.sh   (NEW)
Makefile                                          (MODIFIED)
docs/ADR/ADR-0035-phase18-3-proof-of-care.md     (NEW)
```

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Phase 18: docs/ADR/ADR-0034-phase18-product-language-and-web-shell.md
- Technical Split v9: docs/TECHNICAL_SPLIT_V9_EXECUTION.md
