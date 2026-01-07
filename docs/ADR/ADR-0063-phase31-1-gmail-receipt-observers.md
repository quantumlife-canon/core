# ADR-0063: Phase 31.1 - Gmail Receipt Observers (Email -> CommerceSignals)

**Status:** Accepted
**Date:** 2025-01-07
**Version:** Phase 31.1

## Context

Phase 31 established Commerce Observers with the philosophy: "Commerce is observed. Nothing else." However, Phase 31 lacked a real data source - it only processed mock inputs.

Phase 31.1 addresses this by connecting real Gmail sync data (from Phase 19.1) to the Commerce Observer pipeline. This creates a complete end-to-end flow:

```
Gmail Sync -> Receipt Classification -> Commerce Observations -> /mirror/commerce
```

### Why Email Receipts Beat Vendor APIs

1. **Privacy**: Email receipts are already on the user's device. No additional data sharing agreements needed.

2. **Coverage**: A single Gmail integration covers all merchants that send receipts via email - no need to integrate with each vendor individually.

3. **User Control**: Users explicitly trigger syncs. No background data collection.

4. **Simplicity**: Rule-based classification requires no vendor API keys, OAuth flows, or rate limits beyond Gmail.

5. **Portability**: The same approach works with any email provider (future phases: Outlook, ProtonMail).

### Why Rule-Based First (No LLM)

1. **Determinism**: Same inputs always produce same outputs. Essential for auditability.

2. **Privacy**: No data sent to external services for classification.

3. **Speed**: Classification happens instantly in-memory.

4. **Transparency**: Rules are explicit and auditable. No black-box model decisions.

5. **Resource Efficiency**: No GPU, no API costs, no network latency.

LLMs may be added later (Phase 32+) for edge cases, but only as an optional enhancement, never as a requirement.

## Decision

Implement Gmail Receipt Observers with these core properties:

### 1. Privacy Model (Hash-Only, Abstract Buckets)

**What IS stored:**
- Category bucket: `delivery` | `transport` | `retail` | `subscription` | `bills` | `travel` | `other`
- Magnitude bucket: `nothing` | `a_few` | `several`
- Horizon bucket: `now` | `soon` | `later`
- Evidence hash: SHA256 of abstract classification tokens
- Period: ISO week format (e.g., "2025-W03")

**What is NOT stored:**
- Merchant names
- Sender email addresses
- Email subjects
- Email snippets/body
- Amounts or currency symbols
- Raw message IDs
- Timestamps (beyond period bucket)

### 2. Determinism Model (Canonical Strings + Sorting)

All classification is deterministic:

```go
// Same inputs always produce same hash
result1 := engine.BuildFromGmailMessages(circleID, period, syncHash, messages)
result2 := engine.BuildFromGmailMessages(circleID, period, syncHash, messages)
// result1.StatusHash == result2.StatusHash (always)
```

Achieved through:
- Sorting inputs by MessageIDHash before processing
- Using pipe-delimited canonical strings (not JSON)
- Fixed category priority order
- Deterministic hash computation

### 3. Classification Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│  Gmail Message (in memory only, NOT stored)                     │
│  - MessageID (hashed immediately)                               │
│  - SenderDomain (used for classification, then discarded)       │
│  - Subject (used for classification, then discarded)            │
│  - Snippet (used for classification, then discarded)            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  Receipt Classification (internal/receiptscan/)                 │
│  - Check for receipt-indicating keywords                        │
│  - Classify category from domain patterns                       │
│  - Classify horizon from content patterns                       │
│  - Build evidence hash from ABSTRACT tokens only                │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  Commerce Ingest (internal/commerceingest/)                     │
│  - Aggregate receipt signals by category                        │
│  - Select top 3 categories (deterministic priority)             │
│  - Convert counts to magnitude buckets                          │
│  - Build CommerceObservation records                            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  Persistence (existing Phase 31 store)                          │
│  - Append-only storage                                          │
│  - Hash-only (no raw data)                                      │
│  - 30-day bounded retention                                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  /mirror/commerce (existing Phase 31 page)                      │
│  - "Seen, quietly." title                                       │
│  - Max 3 category chips                                         │
│  - 1-2 calm lines                                               │
│  - NO counts, merchants, or amounts                             │
└─────────────────────────────────────────────────────────────────┘
```

### 4. Integration Point

Phase 31.1 hooks into the existing Gmail sync handler (`/run/gmail-sync`):

```go
// After sync completes and SyncReceipt is created:
if len(messages) > 0 {
    // Extract message data for classification (discarded after use)
    messageData := extractMessageData(messages)

    // Build observations
    result := engine.BuildFromGmailMessages(circleID, period, syncHash, messageData)

    // Persist to existing Phase 31 store
    for _, obs := range result.Observations {
        store.PersistObservation(circleID, &obs)
    }

    // Emit Phase 31.1 events
}
```

## What This Enables Later

Phase 31.1 establishes the foundation for future enhancements:

1. **Vendor APIs (Optional)**: Direct integrations with Deliveroo, Uber, etc. can supplement email classification but are never required.

2. **Bank Rail**: TrueLayer transaction data can be classified using similar patterns, with merchant names abstracted to categories.

3. **SLM On-Device**: Small language models running locally could handle edge cases that rule-based classification misses.

4. **Cross-Provider**: Outlook, ProtonMail, and other email providers use the same classification pipeline.

## Consequences

### Positive

- Real data flows through the Commerce Observer pipeline
- Privacy is preserved through abstract buckets and hashing
- Classification is deterministic and auditable
- No external dependencies for classification
- Proves the system works end-to-end with real Gmail

### Negative

- Rule-based classification has false positives/negatives
- Some receipts may not match known patterns
- Manual rule updates needed for new merchant patterns

### Mitigations

- Classification errors are acceptable (this is observation, not execution)
- Rules cover common patterns; edge cases can wait for LLM enhancement
- False negatives are fine (better to miss than to overclassify)

## Implementation

### Files Created

- `internal/receiptscan/model.go` - Receipt scan types
- `internal/receiptscan/rules.go` - Classification rules
- `internal/commerceingest/model.go` - Ingest types
- `internal/commerceingest/engine.go` - Ingest engine
- `scripts/guardrails/receipt_observer_enforced.sh` - CI enforcement
- `internal/demo_phase31_1_gmail_receipt_observer/demo_test.go` - Tests

### Files Modified

- `cmd/quantumlife-web/main.go` - Gmail sync integration
- `pkg/events/events.go` - Phase 31.1 events

### Events

- `phase31_1.receipt_scan.started` - Scan began
- `phase31_1.receipt_scan.completed` - Scan finished (with abstract buckets)
- `phase31_1.commerce_observations.persisted` - Observations stored

## Hard Constraints (CI-Enforced)

- stdlib only (no external deps)
- NO goroutines
- NO time.Now() anywhere in internal/ or pkg/ (clock injection only)
- Deterministic output: same inputs + same clock => same hashes
- NO merchant names, amounts, sender emails, or subjects stored
- Max 3 categories per mirror page
- Pipe-delimited canonical strings (not JSON)

## References

- ADR-0062: Phase 31 - Commerce Observers (Silent by Default)
- ADR-0046: Phase 19.1 - Real Gmail Connection (You-only)
- Phase 31.1 specification in canon
