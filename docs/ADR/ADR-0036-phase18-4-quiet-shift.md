# ADR-0036: Phase 18.4 - Quiet Shift (Subtle Availability)

## Status
ACCEPTED

## Context
Users on the /today page may have held items that could benefit from their attention, but we cannot create urgency or mental load. The challenge: how do we inform without interrupting?

### Core Concept
> "If you wanted to, there's one thing you could look at."

The Quiet Shift is an ambient availability layer - a barely-visible cue that can be completely ignored with zero cost. It's not a notification, not an alert, not a badge. It's a whisper.

### What This Must NOT Do
- Create urgency or anxiety
- Show specific items, vendors, dates, or amounts
- Compete for attention with the main content
- Require acknowledgment
- Store identifiable information

### What This Must Do
- Be extremely subtle (low contrast, small font)
- Link to a single abstract surfaced item
- Provide explainability on demand
- Allow user to set preference to show_all
- Be deterministic and auditable

## Decision

### Routes and Flow

```
/today → (subtle cue) → /surface → (actions) → /today
```

New routes:
- `GET /surface` - View abstract surfaced item
- `POST /surface/hold` - Mark as held, return to /today
- `POST /surface/why` - Show explainability
- `POST /surface/prefer` - Set preference to show_all

### Surface Cue (on /today)

The cue is shown only when:
1. There exists at least one held category with magnitude `a_few` or `several`
2. AND user preference is `quiet` (if `show_all`, they'll see it elsewhere)

Cue appearance:
- Extremely small font (--text-xs)
- Low contrast (--color-text-quaternary)
- No button styling
- Dotted underline link

```html
<section class="quiet-shift">
  <p class="quiet-shift-cue">If you wanted to, there's one thing you could look at.</p>
  <a href="/surface" class="quiet-shift-link">View, if you like</a>
</section>
```

### Surface Input Model

```go
type SurfaceInput struct {
    // HeldCategories maps category to magnitude (from Phase 18.3)
    HeldCategories map[Category]MagnitudeBucket

    // UserPreference is quiet vs show_all
    UserPreference string

    // Suppression signals for horizon assignment
    SuppressedFinance bool
    SuppressedWork    bool

    // Now is injected for determinism
    Now time.Time
}
```

### Category Priority

Deterministic priority order for surfacing:
```
money > time > work > people > home
```

### Horizon Buckets

Vague time references (no specific dates):
| Signal | Horizon |
|--------|---------|
| Money + SuppressedFinance | `soon` |
| Work + SuppressedWork | `this_week` |
| Time | `this_week` |
| Other | `later` |

### Surface Page

Shows ONE abstract item:

```go
type SurfaceItem struct {
    Category      Category        // money/time/work/people/home
    Magnitude     MagnitudeBucket // a_few/several
    Horizon       HorizonBucket   // soon/this_week/later
    ReasonSummary string          // Abstract, no identifiers
    Explain       []ExplainLine   // On-demand explainability
    ItemKeyHash   string          // SHA256 for audit
}
```

### Reason Summaries (Abstract)

```go
var reasonSummaries = map[Category]string{
    CategoryMoney:  "We noticed a pattern that tends to become urgent if ignored.",
    CategoryTime:   "Something time-related is being held that you might want to know about.",
    CategoryWork:   "There's a work-related item we're watching quietly.",
    CategoryPeople: "We're holding something related to people in your life.",
    CategoryHome:   "A household matter is being held for you.",
}
```

### Action Store

Records ONLY hashes, never raw content:

```go
type ActionRecord struct {
    CircleID    string
    ItemKeyHash string
    Action      Action    // viewed/held/why/prefer_show_all
    RecordedAt  time.Time
    RecordHash  string    // SHA256 of canonical record
}
```

Bounded growth with configurable maxRecords.

### Phase 18.4 Events

| Event | Payload |
|-------|---------|
| `phase18_4.surface.cue.computed` | `{cue_hash, available}` |
| `phase18_4.surface.page.rendered` | `{page_hash, item_category, show_explain}` |
| `phase18_4.surface.action.viewed` | `{item_key_hash}` |
| `phase18_4.surface.action.held` | `{item_key_hash}` |
| `phase18_4.surface.action.why` | `{item_key_hash}` |
| `phase18_4.surface.action.prefer_show_all` | `{item_key_hash}` |

### Guardrails

The guardrail script validates:
1. `/surface` route exists
2. `internal/surface` package exists
3. Engine type with clock injection
4. Model uses SHA256
5. No goroutines in internal/surface
6. No time.Now() (clock injection only)
7. No forbidden imports (stdlib only)
8. All Phase 18.4 events exist
9. Surface templates exist
10. No identifiers in templates
11. CSS classes exist
12. Demo tests exist
13. Store records hash only
14. Store has bounded growth

## Consequences

### Positive
- Ambient awareness without interruption
- Explainability on demand
- User can opt for show_all preference
- Deterministic and reproducible
- Auditable via hash chain
- Even more subtle than /today

### Negative
- Users might miss the cue entirely (by design)
- No way to surface multiple items at once
- Abstract descriptions may seem vague

### Constraints
- NEVER expose specific identifiers
- NEVER create urgency or anxiety
- NEVER auto-surface or auto-notify
- ALWAYS use clock injection
- ALWAYS use stdlib only
- ALWAYS maintain determinism

## Files Changed
```
internal/surface/model.go                           (NEW)
internal/surface/engine.go                          (NEW)
internal/surface/store.go                           (NEW)
pkg/events/events.go                                (MODIFIED)
cmd/quantumlife-web/main.go                         (MODIFIED)
cmd/quantumlife-web/static/app.css                  (MODIFIED)
internal/demo_phase18_4_quiet_shift/demo_test.go    (NEW)
scripts/guardrails/quiet_shift_enforced.sh          (NEW)
Makefile                                             (MODIFIED)
docs/ADR/ADR-0036-phase18-4-quiet-shift.md          (NEW)
```

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Phase 18: docs/ADR/ADR-0034-phase18-product-language-and-web-shell.md
- Phase 18.3: docs/ADR/ADR-0035-phase18-3-proof-of-care.md
