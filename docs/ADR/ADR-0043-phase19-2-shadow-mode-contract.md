# ADR-0043: Phase 19.2 - LLM Shadow Mode Contract

## Status

Accepted

## Context

QuantumLife needs the capability to run LLM-based analysis alongside the quiet loop,
but with strict constraints to ensure it:
- Never affects actual behavior (observation only)
- Never exposes raw content or PII
- Remains deterministic at the system boundary
- Requires explicit user action (no background polling)

This phase introduces a "shadow mode" that can observe what the quiet loop produces
and generate abstract suggestions, but these suggestions are **logged only** and
never influence drafts, interruptions, or execution.

## Decision

### Core Constraints

1. **Observation Only**: Shadow mode produces metadata and suggestions but never
   modifies obligations, drafts, interruptions, or execution state.

2. **Privacy-Safe Output**: All output is abstract:
   - Categories: money, time, work, people, home, health, family, school, unknown
   - Horizons: now, soon, later, someday
   - Magnitudes: nothing, a_few, several
   - Confidence: low, med, high
   - Suggestion types: hold, surface_candidate, draft_candidate

3. **Deterministic**: Same inputs + same clock = same receipt hash.
   Uses canonical pipe-delimited strings (NOT JSON) for all hashing.

4. **Explicit Action Only**: Shadow mode runs only when user clicks a specific
   button. No background polling. No goroutines. No auto-run.

5. **Stub Provider Only**: Phase 19.2 uses a deterministic stub provider.
   No real LLM API calls. No API keys. No network calls.

6. **Clock Injection**: No `time.Now()` in shadow packages. All time comes
   from injected clock.

### Components

#### Domain Model (`pkg/domain/shadowllm/`)

- `ShadowMode`: off | observe
- `ShadowSuggestion`: Category, Horizon, Magnitude, Confidence, SuggestionType
- `ShadowReceipt`: ReceiptID, CircleID, WindowBucket, InputDigestHash, Suggestions, Hash
- `ShadowInputDigest`: Abstract inputs (bucketed counts, category maps)

#### Engine (`internal/shadowllm/engine.go`)

Orchestrates shadow runs:
- Takes abstract inputs from quiet loop state
- Runs stub provider
- Creates and returns receipt
- Never modifies any state

#### Stub Provider (`internal/shadowllm/stub/stub.go`)

Deterministic implementation:
- Maps input digest hash to suggestions
- No network calls
- Same inputs = same outputs

#### Persistence (`internal/persist/shadow_receipt_store.go`)

Append-only storage:
- Stores receipts by ID
- Supports replay verification
- Uses storelog record type

#### Events (`pkg/events/events.go`)

Phase 19.2 events:
- `phase19_2.shadow.requested`
- `phase19_2.shadow.computed`
- `phase19_2.shadow.persisted`
- `phase19_2.shadow.blocked`
- `phase19_2.shadow.failed`

#### Web Integration (`cmd/quantumlife-web/main.go`)

- POST `/run/shadow` route (explicit user action)
- Whisper link on /today: "If you wanted to, we could sanity-check this day."
- Single whisper rule respected (only shows when no other whisper active)

### What Shadow Mode CANNOT Do

- Create or modify obligations
- Create or modify drafts
- Affect interruption levels
- Trigger any execution
- Surface any UI text
- Store raw content
- Make network calls (in Phase 19.2)
- Run in the background

### Privacy Guarantees

The `ShadowReceipt` contains ONLY:
- Abstract category buckets
- Horizon indicators (now/soon/later/someday)
- Magnitude buckets (nothing/a_few/several)
- Confidence buckets (low/med/high)
- Hashes of inputs (never the inputs themselves)

It NEVER contains:
- Email addresses, subjects, or bodies
- Sender or recipient names
- Amounts or financial details
- Specific dates or timestamps (except technical metadata)
- Vendor or company names
- Any raw content

## Guardrails

20+ checks in `scripts/guardrails/shadow_mode_enforced.sh`:
- No `net/http` imports in shadow packages
- No real LLM provider strings (OpenAI, Anthropic, Gemini)
- No goroutines
- No `time.Now()` (must use clock injection)
- Only stub provider exists
- Canonical strings use pipe delimiter (not JSON)
- No shadowllm imports in execution/drafts packages

## Testing

14 demo tests in `internal/demo_phase19_2_shadow_mode/demo_test.go`:
- Determinism verification
- Privacy pattern checks
- Suggestion sorting stability
- Replay hash verification
- State isolation tests
- Receipt and suggestion validation

## Future Roadmap

Phase 19.2+ may introduce:
- Real LLM provider adapters (Claude, GPT, etc.)
- Explicit permission system for LLM providers
- Signed policy requirements
- Rate limiting and cost controls

All future providers must maintain the same constraints:
- Observation only
- Privacy-safe output
- No influence on behavior

## Consequences

### Positive
- Lays groundwork for LLM-assisted observation
- Maintains strict privacy guarantees
- Deterministic and auditable
- No risk of accidental execution

### Negative
- Stub provider provides limited value (placeholder only)
- Additional code complexity

### Neutral
- Future phases required for real LLM integration
- Policy framework needed for LLM provider approval
