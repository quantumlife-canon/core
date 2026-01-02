# ADR-0038: Phase 18.6 - First Connect (Consent-first Onboarding)

## Status
ACCEPTED

## Context
QuantumLife needs a bridge between the calm landing page, the quiet loop (/today), and real-world integrations. This must happen without adding real OAuth keys yet, while establishing the consent-first pattern that will carry through to real integrations.

### Core Concept
> "First, consent."

The First Connect phase creates the consent and connection infrastructure that will eventually power real integrations. It establishes that:
1. The user explicitly chooses what to connect
2. The system explains what it can read, do, and never do
3. Connection state is deterministic and replayable
4. No real data flows without explicit configuration

### What This Must NOT Do
- Add real OAuth keys or flows
- Store credentials or secrets
- Create automatic connections
- Make decisions without consent
- Bypass the "nothing needs you" pattern

### What This Must Do
- Provide calm consent page explaining read/do/never
- Show connection status deterministically
- Record connection intents as append-only log
- Support mock mode for development
- Respect the single whisper rule (no new whispers on /today)

## Decision

### Routes and Flow

```
/ → /start → /connect/:kind → /connections → /today
```

New routes:
- `GET /start` - Consent page with connect options
- `GET /connections` - Shows connected sources status
- `POST /connect/:kind` - Create connect intent (kind ∈ {email, calendar, finance})
- `POST /disconnect/:kind` - Create disconnect intent
- `GET /connect/:kind` - Stub connector page (optional)

### Consent Page (/start)

Content structure:
- Title: "First, consent."
- Sub: "QuantumLife stays quiet by default."
- Three sections:
  1. **What we can read** - Email headers, calendar events, commerce receipts
  2. **What we can do** - Draft replies, draft responses, suggest actions
  3. **What we never do** - No auto-send, no auto-pay, no background actions
- Action area: "Connect one circle." with 3 buttons (Email, Calendar, Finance)
- Secondary link: "See connected sources" → /connections

### Domain Model

#### ConnectionKind
```go
type ConnectionKind string
const (
    KindEmail    ConnectionKind = "email"
    KindCalendar ConnectionKind = "calendar"
    KindFinance  ConnectionKind = "finance"
)
```

#### ConnectionStatus
```go
type ConnectionStatus string
const (
    StatusNotConnected  ConnectionStatus = "not_connected"
    StatusConnectedMock ConnectionStatus = "connected_mock"
    StatusConnectedReal ConnectionStatus = "connected_real"
    StatusNeedsConfig   ConnectionStatus = "needs_config"
)
```

#### ConnectionIntent
```go
type ConnectionIntent struct {
    ID     string           // SHA256 of canonical string
    Kind   ConnectionKind
    Action IntentAction     // connect | disconnect
    Mode   IntentMode       // mock | real
    At     time.Time        // from injected clock
    Note   IntentNote       // bounded values only
}
```

### Canonical String Format

Pipe-delimited, not JSON:
```
CONN_INTENT|v1|kind|action|mode|atRFC3339|note
```

### State Computation

```go
func ComputeState(intents IntentList, configPresent map[ConnectionKind]bool) *ConnectionStateSet
```

Rules:
- Last-write-wins by timestamp
- Tie-break by hash (lexical)
- disconnect → NotConnected
- connect + mock → ConnectedMock
- connect + real + no config → NeedsConfig
- connect + real + config → ConnectedReal

### Phase 18.6 Events

| Event | Payload |
|-------|---------|
| `phase18_6.connection.intent.recorded` | `{intent_id, kind, action, mode}` |
| `phase18_6.connection.state.computed` | `{state_hash}` |
| `phase18_6.connection.connect.requested` | `{kind, mode}` |
| `phase18_6.connection.disconnect.requested` | `{kind, mode}` |

### Guardrails

The guardrail script validates:
1. `/start` route exists
2. `/connections` route exists
3. `/connect/` route exists
4. `/disconnect/` route exists
5. `pkg/domain/connection` package exists
6. ConnectionKind and ConnectionIntent types exist
7. No OAuth imports
8. No goroutines in connection code
9. No time.Now() (clock injection only)
10. No json.Marshal (pipe-delimited only)
11. Canonical string uses pipe delimiter
12. Phase 18.6 events exist
13. Templates exist
14. Demo tests exist
15. CSS styling exists
16. No raw secrets stored
17. RecordType exists in storelog

### Single Whisper Rule Compliance

Phase 18.6 does NOT add a new whisper to /today.
- Link to /start is on landing page only (subtle)
- No changes to the /today whisper priority (surface > proof)

## Consequences

### Positive
- Clear consent pattern established
- Connection infrastructure ready for real integrations
- Deterministic and replayable state
- Mock mode for development/testing
- No real data flows until explicitly configured

### Negative
- Users cannot connect real accounts yet
- "Needs configuration" may seem incomplete
- No actual sync functionality

### Constraints
- NEVER store real credentials or secrets
- NEVER add OAuth or third-party auth libs
- NEVER create automatic connections
- ALWAYS use clock injection
- ALWAYS use pipe-delimited canonical strings
- ALWAYS maintain determinism

## Files Changed
```
pkg/domain/connection/types.go                           (NEW)
pkg/domain/connection/state.go                           (NEW)
pkg/domain/storelog/log.go                               (MODIFIED)
internal/persist/connection_store.go                     (NEW)
pkg/events/events.go                                     (MODIFIED)
cmd/quantumlife-web/main.go                              (MODIFIED)
cmd/quantumlife-web/static/app.css                       (MODIFIED)
internal/demo_phase18_6_first_connect/demo_test.go       (NEW)
scripts/guardrails/connection_onboarding_enforced.sh     (NEW)
Makefile                                                 (MODIFIED)
docs/ADR/ADR-0038-phase18-6-first-connect.md             (NEW)
```

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Phase 18: docs/ADR/ADR-0034-phase18-product-language-and-web-shell.md
- Phase 18.5: docs/ADR/ADR-0037-phase18-5-quiet-proof.md
