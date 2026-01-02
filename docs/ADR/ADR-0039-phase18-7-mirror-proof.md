# ADR-0039: Phase 18.7 - Mirror Proof (Trust Through Evidence of Reading)

## Status
ACCEPTED

## Context
QuantumLife has established connections in Phase 18.6 but users may wonder: "What did you actually see — and what didn't you keep?" This phase answers that question through abstract, calm evidence.

### Core Concept
> "Seen, quietly."

The Mirror Proof provides verifiable evidence that QuantumLife has read connected sources — without showing content, identities, timestamps, vendors, or actions. It is a read receipt for reality, designed to increase trust without increasing cognitive load.

### What This Must NOT Do
- Show names, dates, vendors, senders, subjects, amounts
- Display timestamps (use horizon buckets only)
- Expose raw counts (use magnitude buckets only)
- Create notifications or badges
- Add urgency
- Show CTAs or upsells
- Push information uninvited

### What This Must Do
- Provide abstract evidence of reading
- Show what was NOT stored explicitly
- Use magnitude buckets (none, a_few, several)
- Use horizon buckets (recent, ongoing, earlier)
- Appear only when user seeks it
- Maintain full determinism

## Decision

### Route and Navigation

New route:
- `GET /mirror` - Shows abstract evidence of reading

Navigation:
- Linked subtly from `/connections` → "What we noticed"
- NEVER auto-linked from `/today`

### Page Copy

**Title:** "Seen, quietly."

**Subtitle:** "A record of what we noticed — and what we didn't keep."

**Sources Section (per connected source):**
```
Email
• Read successfully
• Not stored: messages, senders, subjects
• Observed: a few time commitments, some receipts
```

**Outcome Section:**
```
As a result:
• One item is being held quietly
• Nothing requires your attention
```

**Restraint Section:**
```
"We chose not to interrupt you."
"Quiet is a feature, not a gap."
```

### Domain Model

#### Package: pkg/domain/mirror/

Types:
- `MagnitudeBucket` - none, a_few, several
- `HorizonBucket` - recent, ongoing, earlier
- `ObservedCategory` - time_commitments, receipts, messages, patterns
- `ObservedItem` - single abstract observation
- `MirrorSourceSummary` - per-source abstract summary
- `MirrorOutcome` - what changed (held/surfaced)
- `MirrorPage` - complete mirror proof page
- `MirrorInput` - input for building mirror
- `MirrorAck` - acknowledgement record

All implement:
- `CanonicalString()` - pipe-delimited, not JSON
- `Hash()` - SHA256 of canonical string

### Engine

#### Package: internal/mirror/

Responsibilities:
- Build mirror view from connection state
- Abstract raw counts to magnitude buckets
- Select horizon buckets deterministically
- Generate reassuring "not stored" statements
- NEVER read raw events
- NEVER expose identifiers
- Sort everything deterministically

### Persistence

#### Package: internal/mirror/

`AckStore` - append-only bounded store:
- Records hash of mirror page when viewed
- Prevents resurfacing same proof repeatedly
- Hash-only storage (no raw content)
- Bounded capacity with FIFO eviction

### Events

| Event | Payload |
|-------|---------|
| `phase18_7.mirror.computed` | `{mirror_hash, source_count, held_quietly}` |
| `phase18_7.mirror.viewed` | `{mirror_hash}` |
| `phase18_7.mirror.acknowledged` | `{mirror_hash}` |

All events:
- Include canonical hash
- NEVER include source identifiers

### Guardrails

The guardrail script validates:
1. `/mirror` route exists
2. `pkg/domain/mirror` package exists
3. `internal/mirror` package exists
4. Core types exist (MirrorPage, MagnitudeBucket, HorizonBucket)
5. No goroutines in mirror code
6. No time.Now() (clock injection only)
7. Canonical strings use pipe delimiter
8. No json.Marshal
9. Phase 18.7 events exist
10. Mirror template exists
11. Demo tests exist
12. CSS styling exists
13. No raw counts exposed
14. No vendor names in statements
15. stdlib only imports
16. Link from connections to mirror exists

### Styling Rules

- Same palette as Phase 18
- Uses `--text-sm` and `--color-text-secondary`
- NO cards
- NO borders (except subtle dividers)
- Whitespace + headings only

### Explicitly Forbidden

- Dashboards
- Logs
- "Last synced at"
- "You received"
- "We processed X items"
- OAuth scopes display
- Percentages
- Graphs
- Timelines
- Specific dates
- Vendor names
- Sender names
- Subject lines
- Dollar amounts

## Consequences

### Positive
- Users can verify QuantumLife is reading connected sources
- Trust increases through transparency
- Abstract-only prevents information overload
- Restraint messaging reinforces calm philosophy
- Deterministic and replayable

### Negative
- No actionable information (by design)
- May seem vague to users wanting specifics
- Requires connected sources to show content

### Constraints
- NEVER expose identifiers
- NEVER show raw counts
- NEVER show timestamps
- ALWAYS use magnitude buckets
- ALWAYS use horizon buckets
- ALWAYS maintain determinism
- NEVER push information to user

## Files Changed
```
pkg/domain/mirror/types.go                           (NEW)
internal/mirror/engine.go                            (NEW)
internal/mirror/store.go                             (NEW)
pkg/events/events.go                                 (MODIFIED)
cmd/quantumlife-web/main.go                          (MODIFIED)
cmd/quantumlife-web/static/app.css                   (MODIFIED)
internal/demo_phase18_7_mirror/demo_test.go          (NEW)
scripts/guardrails/mirror_proof_enforced.sh          (NEW)
Makefile                                             (MODIFIED)
docs/ADR/ADR-0039-phase18-7-mirror-proof.md          (NEW)
```

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Phase 18.6: docs/ADR/ADR-0038-phase18-6-first-connect.md
- Phase 18.5: docs/ADR/ADR-0037-phase18-5-quiet-proof.md
