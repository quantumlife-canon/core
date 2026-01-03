# ADR-0053: Phase 23 - Gentle Action Invitation (Trust-Preserving)

## Status

Accepted

## Context

QuantumLife has established trust through:
- Phase 19.1: Real Gmail connection with sync receipts
- Phase 22: Quiet Inbox Mirror (first real value moment)
- Phases 18.x: Product language emphasizing silence and restraint

Now we introduce the first optional, reversible user choice WITHOUT breaking silence.

This phase must:
- Invite agency only after trust is proven
- Never create urgency
- Never auto-execute
- Never surface identifiers
- Never increase engagement pressure

**Silence remains the success state.**

## Decision

We implement a **Gentle Action Invitation** that:

### 1. Appears Only After Trust Is Proven

Prerequisites:
- Gmail connected
- At least one real sync
- Quiet Inbox Mirror viewed
- Trust baseline exists

### 2. Shows One Whisper, Once Per Period

User sees one calm invitation per day (period):

> "If you ever want, you can decide what should happen next."

Clicking leads to one calm screen with one invitation.

### 3. Offers Three Calm Choices

| Kind | Display Text |
|------|--------------|
| `hold_continue` | "We can keep holding this." |
| `review_once` | "You can look once, if you want." |
| `notify_next_time` | "Tell us how you'd like this to reach you next time." |

### 4. Records Decision Without Execution

- **Accept**: Records acceptance, hides future invitations for period
- **Dismiss**: Suppresses for current period only

Neither action triggers any execution. This is **invitation only**.

## Architecture

```
Trust Baseline → Eligibility Check → Invitation Engine → Page Render
       │                                    │
       │                              (max 1/period)
       │                              (deterministic)
       │                                    │
       └──────────────────────────────────> Store (hash-only)
```

### Domain Model

```go
// pkg/domain/invitation/types.go

type InvitationKind string
const (
    KindHoldContinue   InvitationKind = "hold_continue"
    KindReviewOnce     InvitationKind = "review_once"
    KindNotifyNextTime InvitationKind = "notify_next_time"
)

type InvitationDecision string
const (
    DecisionAccepted  InvitationDecision = "accepted"
    DecisionDismissed InvitationDecision = "dismissed"
)

type InvitationSummary struct {
    CircleID   string
    Period     InvitationPeriod
    Kind       InvitationKind
    Text       string           // from allowed phrases only
    WhisperCue string
    SourceHash string
}
```

### Engine

```go
// internal/invitation/engine.go

func (e *Engine) ComputeEligibility(...) *InvitationEligibility
func (e *Engine) Compute(eligibility) *InvitationSummary
func (e *Engine) BuildPage(summary) *InvitationPage
func (e *Engine) BuildWhisperCue(summary) *WhisperCue
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/invite` | GET | Show invitation page |
| `/invite/accept` | POST | Record acceptance |
| `/invite/dismiss` | POST | Record dismissal |

### Events (6 total)

```go
Phase23InvitationEligible  = "phase23.invitation.eligible"
Phase23InvitationRendered  = "phase23.invitation.rendered"
Phase23InvitationAccepted  = "phase23.invitation.accepted"
Phase23InvitationDismissed = "phase23.invitation.dismissed"
Phase23InvitationPersisted = "phase23.invitation.persisted"
Phase23InvitationSkipped   = "phase23.invitation.skipped"
```

Payloads contain **hashes only**, **enums only**, **no text**.

## Copy Rules (Strict)

### Allowed Phrases Only

- "We can keep holding this."
- "You can look once, if you want."
- "Tell us how you'd like this to reach you next time."
- "If you ever want, you can decide what should happen next."

### Forbidden

- Urgency language ("urgent", "immediately", "now", "hurry", "asap", "important")
- Numbers
- Dates
- Vendor names
- Calls to action ("review now", "important")

## Consequences

### Benefits

1. **Trust Through Agency**
   - User controls when (if ever) to engage
   - No pressure, no consequences

2. **Silence Preserved**
   - Invitation is optional
   - Dismissal is respected
   - No escalation

3. **Foundation for Future**
   - Phase 24 can build on accepted invitations
   - Reversible decisions only

### Tradeoffs

1. **Limited Engagement**
   - Most users may never click
   - This is intentional

2. **No Immediate Value**
   - Invitation doesn't do anything yet
   - This is intentional

## Non-Goals (Explicit)

Phase 23 must NOT:
- Create tasks
- Execute drafts
- Change canon behavior
- Surface shadow output
- Introduce workflows

**This phase is invitation only.**

## Implementation

### Files Created

| File | Purpose |
|------|---------|
| `pkg/domain/invitation/types.go` | Domain model |
| `internal/invitation/engine.go` | Invitation engine |
| `internal/persist/invitation_store.go` | Hash-only persistence |
| `cmd/quantumlife-web/main.go` | Web handlers |
| `internal/demo_phase23_gentle_invitation/demo_test.go` | Demo tests |
| `scripts/guardrails/gentle_invitation_enforced.sh` | Guardrails |

### Guardrails (35+ checks)

- No goroutines in invitation packages
- No time.Now() (clock injection only)
- No execution imports
- No draft creation
- No obligation creation
- No retry patterns
- Stdlib only
- Phase 23 events defined
- Hash-keyed storage
- Bounded retention
- Routes registered
- No urgency language

### Demo Tests (13 tests)

1. Invitation appears only after quiet proof
2. Invitation requires Gmail sync
3. Invitation requires mirror viewed
4. Eligible when all conditions met
5. Only one invitation per period
6. Dismiss suppresses for period
7. Deterministic output
8. Canonical string format
9. Page when no invitation
10. Page when invitation exists
11. Whisper cue
12. Store persistence
13. Kind selection based on context

## Success Criteria

Phase 23 is complete when:
- A real Gmail user sees one calm invitation
- They accept or dismiss
- Nothing urgent happens
- Silence resumes
- CI + guardrails pass

## Invariants

**NON-NEGOTIABLE:**

1. Silence is success
2. Agency is optional
3. Decisions are reversible
4. No execution ever
5. No urgency ever
6. Determinism everywhere

## References

- Phase 22: Quiet Inbox Mirror
- Phase 21: Unified Onboarding
- Phase 19.1: Real Gmail Connection
- Canon v1: Forbidden at Core
