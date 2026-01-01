# ADR-0037: Phase 18.5 - Quiet Proof (Restraint Ledger)

## Status
ACCEPTED

## Context
Users in quiet mode need proof that QuantumLife is actively protecting them without surfacing details, counts, or anything actionable. The challenge: how do we demonstrate restraint without turning it into analytics, guilt, or a dashboard?

### Core Concept
> "We chose not to interrupt you — and here's the gentle evidence."

The Quiet Proof is a restraint ledger - abstract proof of what was held back. It's not a weekly digest, not analytics, not a notification count. It's trust.

### What This Must NOT Do
- Show raw counts, dates, vendors, names, or IDs
- Create a timeline or chart
- Feel like a "weekly digest"
- Create guilt or FOMO
- Become a dashboard

### What This Must Do
- Prove restraint abstractly
- Use magnitude buckets only: nothing / a_few / several
- Show category chips (money/time/work/people/home)
- Provide optional "why this matters" disclosure
- Be dismissable with no nagging
- Be deterministic and auditable

## Decision

### Routes and Flow

```
/today → (subtle cue) → /proof → (dismiss) → /today
```

New routes:
- `GET /proof` - View abstract restraint proof
- `POST /proof/dismiss` - Dismiss proof, return to /today

### Proof Cue (on /today)

The cue is shown only when:
1. Preference is quiet
2. AND proof magnitude is not nothing
3. AND user hasn't dismissed/acknowledged proof recently

Cue appearance:
- Extremely small font (--text-xs)
- Low contrast (--color-text-quaternary)
- No button styling
- Dotted underline link

```html
<section class="quiet-proof-cue">
  <p class="quiet-proof-cue-text">If you ever wondered—silence is intentional.</p>
  <a href="/proof" class="quiet-proof-cue-link">Proof, if you want it.</a>
</section>
```

### Proof Input Model

```go
type ProofInput struct {
    // SuppressedByCategory maps category to suppressed count.
    // Counts are used internally for bucketing only - never exposed.
    SuppressedByCategory map[Category]int

    // PreferenceQuiet indicates if user prefers quiet mode.
    PreferenceQuiet bool

    // Period is the time window (e.g., "week").
    // No dates are ever shown - just the abstract period.
    Period string
}
```

### Magnitude Buckets

Counts are used internally only - never exposed:
| Total Count | Magnitude |
|-------------|-----------|
| 0           | `nothing` |
| 1-3         | `a_few`   |
| 4+          | `several` |

### Statement Variants

Deterministic selection based on magnitude:
```go
var statements = map[Magnitude]string{
    MagnitudeAFew:    "We chose not to interrupt you a few times.",
    MagnitudeSeveral: "We chose not to interrupt you often.",
    MagnitudeNothing: "Nothing needed holding.",
}
```

### Why Line

Reassurance for non-nothing magnitudes:
```
"Quiet is a feature. Not a gap."
```

### Proof Summary

```go
type ProofSummary struct {
    Magnitude  Magnitude  // nothing / a_few / several
    Categories []Category // sorted lexicographically
    Statement  string     // calm, abstract copy
    WhyLine    string     // optional short reassurance
    Hash       string     // SHA256 of canonical string
}
```

### Ack Store

Records ONLY hashes, never raw content:

```go
type AckRecord struct {
    Action    AckAction // viewed / dismissed
    ProofHash string
    TSHash    string    // Hash of timestamp, not raw
}
```

Bounded growth with configurable maxRecords (default 128).

### Phase 18.5 Events

| Event | Payload |
|-------|---------|
| `phase18_5.proof.computed` | `{proof_hash, magnitude}` |
| `phase18_5.proof.viewed` | `{proof_hash, magnitude}` |
| `phase18_5.proof.dismissed` | `{proof_hash}` |

### Guardrails

The guardrail script validates:
1. `/proof` route exists
2. `/proof/dismiss` route exists
3. `internal/proof` package exists
4. Engine type with deterministic projection
5. Model uses SHA256
6. No goroutines in internal/proof
7. No time.Now() (clock injection only)
8. No forbidden imports (stdlib only)
9. All Phase 18.5 events exist
10. Proof templates exist
11. No identifiers in templates
12. CSS classes exist
13. Demo tests exist
14. Magnitude enum exists (no raw counts)
15. Store uses record hash
16. Store has bounded growth
17. Proof cue CSS exists

## Consequences

### Positive
- Proves restraint without creating a dashboard
- Builds trust through transparency
- Magnitude buckets prevent count obsession
- Dismissable with no nagging
- Deterministic and reproducible
- Auditable via hash chain
- Even more subtle than /surface

### Negative
- Users might miss the cue entirely (by design)
- Abstract descriptions may seem vague
- No way to see specific items (by design)

### Constraints
- NEVER expose raw counts
- NEVER expose specific identifiers
- NEVER create a timeline or chart
- NEVER auto-surface or auto-notify
- ALWAYS use clock injection
- ALWAYS use stdlib only
- ALWAYS maintain determinism

## Files Changed
```
internal/proof/model.go                             (NEW)
internal/proof/engine.go                            (NEW)
internal/proof/store.go                             (NEW)
pkg/events/events.go                                (MODIFIED)
cmd/quantumlife-web/main.go                         (MODIFIED)
cmd/quantumlife-web/static/app.css                  (MODIFIED)
internal/demo_phase18_5_proof/demo_test.go          (NEW)
scripts/guardrails/proof_enforced.sh                (NEW)
Makefile                                            (MODIFIED)
docs/ADR/ADR-0037-phase18-5-quiet-proof.md          (NEW)
```

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Phase 18: docs/ADR/ADR-0034-phase18-product-language-and-web-shell.md
- Phase 18.3: docs/ADR/ADR-0035-phase18-3-proof-of-care.md
- Phase 18.4: docs/ADR/ADR-0036-phase18-4-quiet-shift.md
