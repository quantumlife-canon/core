# ADR-0058: Phase 27 - Real Shadow Receipt (Primary Proof of Intelligence, Zero Pressure)

## Status

Accepted

## Context

Phase 19 introduced shadow mode - a deterministic observation layer that consults real LLMs but never alters runtime behavior. The shadow receipt viewer (Phase 21) provided basic visibility into shadow observations.

However, users still lack a definitive "proof" page that demonstrates:
1. A real model was actually consulted
2. Its output was received and validated
3. Restraint was applied (nothing was surfaced)
4. The system chose NOT to interrupt

This phase makes the Shadow Receipt the **primary proof** of intelligence working quietly in the background - the first undeniable "this is real" moment for skeptical users.

## Decision

Implement a **Primary Proof Page** with four structured sections that prove intelligence was applied without interruption:

### Receipt Content Structure

1. **"What We Asked"** (Evidence Section)
   - Generic phrasing only (e.g., "We asked whether anything should reach you today")
   - Never shows specific queries or prompts

2. **"What the Model Returned"** (Model Return Section)
   - Abstract buckets only: horizon (soon/later/someday), magnitude (nothing/a_few/several), confidence (low/medium/high)
   - Human-readable summary statement
   - Never raw counts, categories beyond canon, or identifiers

3. **"What We Did"** (Decision Section)
   - Always restraint-forward phrasing
   - Examples: "We chose not to surface anything", "No model was consulted"

4. **"Why This Didn't Interrupt You"** (Reason Section)
   - Explains the restraint mechanism that protected the user
   - Examples: "Signal was below surfacing threshold", "Default hold policy was in effect"

### Provider Disclosure

The page shows **abstract provider disclosure only**:
- `none` - No provider configured/used
- `stub` - Deterministic stub provider
- `azure_openai_chat` - Azure OpenAI chat

**Never** shows model names, deployment names, regions, endpoints, or keys.

### Single Vote System

An optional, single-shot, one-time vote on the restraint:
- **Useful** - The restraint was appropriate
- **Unnecessary** - The restraint was unnecessary
- **Skip** - No opinion

**CRITICAL INVARIANTS:**
- Vote does NOT change behavior
- Vote feeds Phase 19 calibration only (via `CountVotesByPeriod()`)
- One vote per receipt hash
- Skipping is permanent - no nagging or re-prompting

### Whisper Integration

The shadow receipt cue follows the single-whisper rule:
- **Priority order**: Journey > Surface > Proof > First-minutes > Reality > Shadow receipt (lowest)
- **Cue text**: "We checked something — quietly."
- **Link text**: "proof"

## Implementation

### Domain Types (`pkg/domain/shadowview/types.go`)

```go
// Evidence types
type EvidenceAskedKind string // surface_check, none
type ShadowReceiptEvidence struct { Kind, Statement }

// Model return types
type HorizonBucket string    // soon, later, someday, none
type MagnitudeBucket string  // nothing, a_few, several
type ConfidenceBucket string // low, medium, high, na
type ShadowReceiptModelReturn struct { Horizon, Magnitude, Confidence, Statement }

// Decision types
type DecisionKind string // no_surface, no_model
type ShadowReceiptDecision struct { Kind, Statement }

// Reason types
type ReasonKind string // below_threshold, default_hold, explicit_action_required, no_model_consulted, shadow_only
type ShadowReceiptReason struct { Kind, Statement }

// Provider disclosure
type ProviderKind string // none, stub, azure_openai_chat
type ShadowReceiptProvider struct { Kind, WasConsulted, Statement }

// Vote types
type VoteChoice string // useful, unnecessary, skip
type ShadowReceiptVote struct { ReceiptHash, Choice, PeriodBucket }

// Primary page
type ShadowReceiptPrimaryPage struct {
    HasReceipt bool
    Evidence, ModelReturn, Decision, Reason, Provider
    VoteEligibility, StatusHash, BackPath
}

// Whisper cue
type ShadowReceiptCue struct { Available, CueText, LinkText, ReceiptHash }
```

### Engine Methods (`internal/shadowview/engine.go`)

```go
// BuildPrimaryPageInput contains inputs for the primary proof page.
type BuildPrimaryPageInput struct {
    Receipt      *shadowllm.ShadowReceipt
    HasVoted     bool
    VoteChoice   domainshadowview.VoteChoice
    ProviderKind string
}

// BuildPrimaryPage builds the Phase 27 primary proof page.
func (e *Engine) BuildPrimaryPage(input BuildPrimaryPageInput) domainshadowview.ShadowReceiptPrimaryPage

// BuildPrimaryCueInput contains inputs for the Phase 27 whisper cue.
type BuildPrimaryCueInput struct {
    Receipt        *shadowllm.ShadowReceipt
    IsDismissed    bool
    OtherCueActive bool
    ProviderKind   string
}

// BuildPrimaryCue builds the Phase 27 whisper cue.
func (e *Engine) BuildPrimaryCue(input BuildPrimaryCueInput) domainshadowview.ShadowReceiptCue
```

### Ack/Vote Store (`internal/persist/shadow_receipt_ack_store.go`)

```go
type ShadowReceiptAckStore struct {
    acks        map[string]*shadowReceiptAck  // period+receiptHash -> ack
    votes       map[string]*ShadowReceiptVote // receiptHash -> vote
    maxPeriods  int                           // 30 days
    storelogRef storelog.AppendOnlyLog
}

func (s *ShadowReceiptAckStore) RecordViewed(receiptHash, periodBucket string) error
func (s *ShadowReceiptAckStore) RecordDismissed(receiptHash, periodBucket string) error
func (s *ShadowReceiptAckStore) RecordVote(vote *ShadowReceiptVote) error
func (s *ShadowReceiptAckStore) IsDismissed(receiptHash, periodBucket string) bool
func (s *ShadowReceiptAckStore) HasVoted(receiptHash string) bool
func (s *ShadowReceiptAckStore) CountVotesByPeriod(periodBucket string) (useful, unnecessary int)
```

### Events (`pkg/events/events.go`)

```go
Phase27ShadowReceiptRendered  EventType = "phase27.shadow_receipt.rendered"
Phase27ShadowReceiptVoted     EventType = "phase27.shadow_receipt.voted"
Phase27ShadowReceiptDismissed EventType = "phase27.shadow_receipt.dismissed"
```

### Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/shadow/receipt` | GET | Display primary proof page |
| `/shadow/receipt/vote` | POST | Record vote |
| `/shadow/receipt/dismiss` | POST | Dismiss cue |

## Guardrails (42 checks)

The guardrail script validates:

1. **Domain model checks** (13 checks)
   - All required types exist
   - CanonicalString methods exist
   - Vote choices defined

2. **Engine checks** (6 checks)
   - BuildPrimaryPage exists
   - BuildPrimaryCue exists
   - No time.Now(), no goroutines

3. **Store checks** (10 checks)
   - All required methods exist
   - Hash-only storage
   - Bounded retention (30 days)

4. **Event checks** (3 checks)
   - All Phase 27 events defined

5. **Web route checks** (4 checks)
   - All routes exist
   - Handler exists

6. **Safety invariant checks** (6 checks)
   - "Vote does NOT change behavior" documented
   - Single-whisper rule integrated
   - No forbidden patterns

## Consequences

### Positive

- Users can see definitive proof that intelligence is working
- Abstract buckets protect privacy while providing transparency
- Single vote provides calibration data without nagging
- Lowest-priority whisper cue ensures non-intrusive discovery

### Negative

- Additional complexity in shadowview engine
- One more whisper cue in priority chain

### Neutral

- Extends existing shadow receipt viewer without breaking it
- Vote data feeds Phase 19 calibration system

## Related ADRs

- ADR-0041: Phase 19.2 - Shadow Mode Determinism
- ADR-0044: Phase 19.3 - Azure OpenAI Shadow Provider
- ADR-0047: Phase 19.4 - Shadow Diff + Calibration
- ADR-0050: Phase 21 - Unified Onboarding + Shadow Receipt Viewer
